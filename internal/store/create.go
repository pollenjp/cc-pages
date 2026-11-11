package store

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// NewPageInput は cc-pages new のフラグに対応する。
type NewPageInput struct {
	SessionID string
	Title     string
	Summary   string
	Tags      []string
	Prompt    string
	Mode      string // 空なら ModeFragment
}

// NewPageResult は cc-pages new が stdout に出す JSON。
type NewPageResult struct {
	ID  string `json:"id"`
	Dir string `json:"dir"`
	URL string `json:"url"`
}

// CreatePage はセッションディレクトリを用意し、連番を採番し、page.json を書く。
//
// index.html は書かない。それは skill の仕事。
func CreatePage(root, baseURL string, now time.Time, in NewPageInput) (NewPageResult, error) {
	if in.SessionID == "" {
		return NewPageResult{}, errors.New("session id が空")
	}
	if err := ValidateSessionID(in.SessionID); err != nil {
		return NewPageResult{}, err
	}
	if in.Title == "" {
		return NewPageResult{}, errors.New("title が空")
	}
	mode := in.Mode
	if mode == "" {
		mode = ModeFragment
	}
	if mode != ModeFragment && mode != ModeStandalone {
		return NewPageResult{}, fmt.Errorf("不明な mode: %q", mode)
	}

	sessionsDir := filepath.Join(root, "sessions")
	if err := os.MkdirAll(sessionsDir, 0o700); err != nil {
		return NewPageResult{}, err
	}

	sessionDirName, err := findOrMakeSessionDir(sessionsDir, in.SessionID, now)
	if err != nil {
		return NewPageResult{}, err
	}
	sessionPath := filepath.Join(sessionsDir, sessionDirName)

	id := FormatID(nextPageNumber(sessionPath))
	pageDirName := PageDirName(id, Slug(in.Title))
	pagePath := filepath.Join(sessionPath, pageDirName)
	if err := os.MkdirAll(pagePath, 0o700); err != nil {
		return NewPageResult{}, err
	}

	cwd, branch := DetectEnv()
	page := Page{
		Schema:    SchemaVersion,
		ID:        id,
		Title:     in.Title,
		Summary:   in.Summary,
		Tags:      in.Tags,
		Prompt:    in.Prompt,
		Mode:      mode,
		CreatedAt: now,
		SessionID: in.SessionID,
		Cwd:       cwd,
		GitBranch: branch,
	}
	if err := WritePage(pagePath, page); err != nil {
		return NewPageResult{}, err
	}

	if err := touchSession(sessionPath, sessionDirName, in.SessionID, now); err != nil {
		return NewPageResult{}, err
	}

	return NewPageResult{
		ID:  id,
		Dir: pagePath,
		// URL のパス片はどちらも安全な文字種しか含まない:
		//   - sessionDirName = 日付 (数字) + "-" + SessionID の先頭部分。SessionID 自体は
		//     ValidateSessionID が CreatePage の入口で検証済み (文字・数字・"-" のみ)
		//   - pageDirName    = 連番 (数字) + "-" + Slug(タイトル)。Slug が文字・数字・"-"
		//     だけに絞っている
		// だからそのまま連結してよい。url.PathEscape は使わない — 非 ASCII を %XX に
		// 潰してしまい、日本語タイトルの URL が読めなくなるため。
		URL: strings.TrimRight(baseURL, "/") + "/p/" + sessionDirName + "/" + pageDirName,
	}, nil
}

// ValidateSessionID は session id の文字種を検証する。
//
// 許可するのは文字・数字・"-" だけ (Slug と同じ規則)。ここを絞らないと、
// "../../../x" のような値が SessionDirName 経由でそのままディレクトリ名や URL に
// 流れ込み、パストラバーサルや URL の破壊につながる。
//
// serve の /s/<session_id>/ も同じ規則で弾くので export してある。空文字は通る
// (呼ぶ側が先に「空」を別のエラーにしている)。
func ValidateSessionID(sessionID string) error {
	for _, r := range sessionID {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || r == '-' {
			continue
		}
		return fmt.Errorf("session id に使えない文字が含まれる (文字・数字・\"-\" のみ許可): %q", sessionID)
	}
	return nil
}

// findOrMakeSessionDir は session id に対応するディレクトリ名を返す。無ければ作る。
//
// 既存ディレクトリの session.json を読んで一致を判定する。ディレクトリ名の
// 先頭 8 桁で照合しないのは、衝突したときに別セッションのページを取り違えるため。
// ただし読むのは、名前がこの session id の形 (sessionDirMatches) のディレクトリの
// session.json だけ。cc-pages link は turn の終わりごとに打たれるので、全部の会話の
// session.json を毎回読むと、会話が増えるほど遅くなる。
//
// 作るときは sessions の lock を握って探し直し、作ったらすぐ session.json の骨を書く。
// 同じ会話への最初の書き込みが 2 つ重なると (cc-pages link が同時に 2 つ来たときなど)、
// 片方が「作られたばかりで session.json がまだ無いディレクトリ」を別の会話のものと見て
// 名前を伸ばし、会話のディレクトリが 2 つに割れるため。あれば lock は取らない。
func findOrMakeSessionDir(sessionsDir, sessionID string, now time.Time) (string, error) {
	if name, ok := FindSessionDir(sessionsDir, sessionID); ok {
		return name, nil
	}
	unlock, err := lockFile(sessionsLockPath(sessionsDir), lockTimeout)
	if err != nil {
		return "", err
	}
	defer unlock()

	ents, err := os.ReadDir(sessionsDir)
	if err != nil {
		return "", err
	}
	// lock を待つ間に、別の cc-pages が作り終えているかもしれない
	if name := matchSessionDir(sessionsDir, ents, sessionID); name != "" {
		return name, nil
	}
	taken := map[string]bool{}
	for _, e := range ents {
		if e.IsDir() {
			taken[e.Name()] = true
		}
	}
	// 未知のセッション。名前が空くまで先頭を伸ばす。
	name := SessionDirName(now, sessionID)
	for n := 12; taken[name] && n <= len(sessionID); n += 4 {
		short := sessionID
		if len(short) > n {
			short = short[:n]
		}
		name = now.Format("20060102") + "-" + short
	}
	if taken[name] {
		return "", fmt.Errorf("セッションディレクトリ名が衝突した: %s", name)
	}
	path := filepath.Join(sessionsDir, name)
	if err := os.MkdirAll(path, 0o700); err != nil {
		return "", err
	}
	// 骨は lock を外す前に書く (touchSession が後で LastSeen を進める)
	return name, WriteSession(path, Session{
		Schema: SchemaVersion, SessionID: sessionID, Dir: name, FirstSeen: now, LastSeen: now,
	})
}

// sessionsLockPath は会話のディレクトリを作る間だけ握る lock のファイル
// (<root>/sessions.lock)。sessions の中に置かないのは、sessions の直下を会話の
// ディレクトリだけにしておくため。
func sessionsLockPath(sessionsDir string) string {
	return filepath.Clean(sessionsDir) + ".lock"
}

// FindSessionDir は session id に対応する会話のディレクトリ名を探す。作らない。
//
// cc-pages link に 1 件も渡されなかったとき (会話のディレクトリを作らずに URL だけ
// 返す) と、serve の /s/<session_id>/ が索引に無い会話をディスクで探すときに使う。
func FindSessionDir(sessionsDir, sessionID string) (string, bool) {
	ents, err := os.ReadDir(sessionsDir)
	if err != nil {
		return "", false
	}
	name := matchSessionDir(sessionsDir, ents, sessionID)
	return name, name != ""
}

// matchSessionDir は ents のうち、名前がこの session id の形で、session.json の
// session_id が一致するディレクトリの名前を返す。無ければ空。
func matchSessionDir(sessionsDir string, ents []os.DirEntry, sessionID string) string {
	for _, e := range ents {
		if !e.IsDir() || !sessionDirMatches(e.Name(), sessionID) {
			continue
		}
		s, err := ReadSession(filepath.Join(sessionsDir, e.Name()))
		if err == nil && s.SessionID == sessionID {
			return e.Name()
		}
	}
	return ""
}

// sessionDirMatches は、ディレクトリ名がこの session id のために作られた形かを返す。
//
// 名前は "<日付>-<session id の先頭>" (SessionDirName)。先頭は 8 字で、衝突すると
// 4 字ずつ伸ばす。だから最初の "-" より後ろが session id の先頭と一致すればよい。
// 最後の決め手は session.json の session_id (先頭が同じ別の会話がありうる)。
func sessionDirMatches(name, sessionID string) bool {
	_, short, ok := strings.Cut(name, "-")
	return ok && short != "" && strings.HasPrefix(sessionID, short)
}

// nextPageNumber は既存のページディレクトリを見て次の連番を返す。
func nextPageNumber(sessionPath string) int {
	ents, err := os.ReadDir(sessionPath)
	if err != nil {
		return 1
	}
	max := 0
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		head, _, _ := strings.Cut(e.Name(), "-")
		n, err := strconv.Atoi(head)
		if err == nil && n > max {
			max = n
		}
	}
	return max + 1
}

// touchSession は session.json の骨を書く / LastSeen を進める。
func touchSession(sessionPath, dirName, sessionID string, now time.Time) error {
	s, err := ReadSession(sessionPath)
	if err != nil {
		s = Session{
			Schema:    SchemaVersion,
			SessionID: sessionID,
			Dir:       dirName,
			FirstSeen: now,
		}
	}
	s.LastSeen = now
	return WriteSession(sessionPath, s)
}

// DetectEnv は cwd と git ブランチを取る。
//
// git が無い、リポジトリ外、detached HEAD のときはブランチを空で返す。
// cc-pages new が自分で取るので、skill 側の仕事にはならない。
func DetectEnv() (cwd, gitBranch string) {
	cwd, _ = os.Getwd()
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return cwd, ""
	}
	b := strings.TrimSpace(string(out))
	if b == "HEAD" {
		return cwd, ""
	}
	return cwd, b
}

// NotifyTouch は動いている cc-pages serve に再インデックスを促す。
//
// fire-and-forget。サーバが居なければ何もしない。届かなくても、最悪 60 秒後の
// 定期走査で拾われる。ここでエラーを返さないのは、new の成功を「サーバが
// 動いていること」に依存させないため。
func NotifyTouch(baseURL string) {
	c := &http.Client{Timeout: 100 * time.Millisecond}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(baseURL, "/")+"/_/touch", nil)
	if err != nil {
		return
	}
	resp, err := c.Do(req)
	if err != nil {
		return
	}
	_ = resp.Body.Close()
}

// NotifySession は動いている cc-pages serve に、1 つの会話のディレクトリだけを
// 読み直させる (cc-pages link の知らせ)。
//
// NotifyTouch と違い、応答を待たない。NotifyTouch の受け手は全体の走査を終えてから
// 返り、全体の走査は変わった transcript (数十〜百数十 MB のものがある) を頭から
// 読み直す。link は turn の終わりごとに打たれるので、それを待てない。
// request を書いたら接続を閉じる。閉じても、書いた request は serve に届いて処理される。
// http.Client で送って書き終えた所で取り消す形にしないのは、本文の無い request だと、
// 書いたことになった時点ではまだ送り出す前の buffer にあり、取り消すと送られずに
// 終わりうるため。
//
// 届かなくても何もしない。serve が居なければ定期走査で拾われる。
func NotifySession(baseURL, dirName string) {
	u, err := url.Parse(strings.TrimRight(baseURL, "/") + "/_/touch/" + url.PathEscape(dirName))
	if err != nil || u.Host == "" {
		return
	}
	req, err := http.NewRequest(http.MethodPost, u.String(), nil)
	if err != nil {
		return
	}
	req.Close = true
	d := net.Dialer{Timeout: 100 * time.Millisecond}
	conn, err := d.Dial("tcp", hostPort(u))
	if err != nil {
		return
	}
	defer conn.Close()
	_ = conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
	_ = req.Write(conn)
}

// hostPort は URL の host:port を返す。port が無ければ scheme の既定。
func hostPort(u *url.URL) string {
	if u.Port() != "" {
		return u.Host
	}
	port := "80"
	if u.Scheme == "https" {
		port = "443"
	}
	return net.JoinHostPort(u.Hostname(), port)
}
