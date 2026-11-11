package store

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// LinksFileName は links.json のファイル名。
const LinksFileName = "links.json"

const (
	// MaxLinks は 1 つの会話に残すリンクの数の上限。呼ぶ側の不具合で links.json が
	// 膨らみ続けないようにする。超える新しい URL は捨てる。
	MaxLinks = 5000
	// MaxLinkURLRunes は URL の長さの上限 (文字数)。
	MaxLinkURLRunes = 2048
)

// リンクの出どころ。呼ぶ側 (Claude Code の Stop hook) が、そのリンクを Claude の
// 応答の文で見たのか、tool の出力で見たのかを渡す。
const (
	SourceAssistant = "assistant"
	SourceTool      = "tool"
)

// DefaultLinkGroup は組の名前が無いリンクを入れる組。ファイルには空で持ち、描くときに
// この名前を出す。
const DefaultLinkGroup = "その他"

const (
	maxLabelRunes = 200
	maxGroupRunes = 50
	// maxLinkLineBytes と maxLinkInputBytes は --stdin の 1 行と入力全体の上限。
	// ふつうの 1 行は数百バイトなので、上限は呼ぶ側の不具合 (巨大な行・終わらない
	// 入力) に引きずられないためだけにある。
	maxLinkLineBytes  = 64 << 10
	maxLinkInputBytes = 16 << 20
	// quoteRunes は理由の文に引く値の長さ。stderr を値で埋めない。
	quoteRunes = 100
)

// Link は links.json の 1 件。同じ URL は 1 件にまとめる (まとめ方は AddLinks)。
type Link struct {
	URL       string    `json:"url"`
	Label     string    `json:"label,omitempty"` // 空なら URL を出す
	Group     string    `json:"group,omitempty"` // 空なら DefaultLinkGroup に入れる
	Sources   []string  `json:"sources"`         // SourceAssistant・SourceTool の和
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
}

// Links は会話ごとの links.json。
//
// 書くのは AddLinks (cc-pages link) だけ。session.json・status.json と分けて
// あるのは、あちらを書く new・status と書き込みがぶつからないようにするため
// (status.json を分けたのと同じ理由)。
type Links struct {
	Schema    int       `json:"schema"`
	UpdatedAt time.Time `json:"updated_at"`
	Links     []Link    `json:"links"`
}

// ReadLinks は dir/links.json を読む。
func ReadLinks(dir string) (Links, error) {
	b, err := os.ReadFile(filepath.Join(dir, LinksFileName))
	if err != nil {
		return Links{}, err
	}
	var l Links
	if err := json.Unmarshal(b, &l); err != nil {
		return Links{}, err
	}
	return l, nil
}

// writeLinks は dir/links.json を丸ごと書く。serve が書きかけを読まないよう、
// 一時ファイルに書いて rename する。
func writeLinks(dir string, l Links) error {
	if l.Links == nil {
		l.Links = []Link{} // "links": null ではなく [] にする
	}
	b, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(dir, LinksFileName), append(b, '\n'), 0o600)
}

// LinkInput は cc-pages link に渡された 1 件を、検証して整えたもの。
type LinkInput struct {
	URL    string
	Label  string // 空なら URL を出す
	Group  string // 空なら DefaultLinkGroup
	Source string // SourceAssistant か SourceTool
	At     time.Time
}

// NewLinkInput は 1 件を検証して整える。--url などの flag と --stdin の 1 行の
// どちらもここを通る。
//
// URL は前後の空白を落とすだけで、ほかは書き換えない (同じ URL かどうかを文字列で
// 比べるので、呼ぶ側が渡した形のまま持つ)。label・group は描くときに崩れないよう
// 制御文字を落とし、長さを切る。既定と同じ値 (URL と同じ label・「その他」の group)
// は空に寄せる。空なら後から来た件の値で埋める、というまとめ方の規則に乗せるため。
func NewLinkInput(rawURL, label, group, source, at string, now time.Time) (LinkInput, error) {
	u, err := checkLinkURL(rawURL)
	if err != nil {
		return LinkInput{}, err
	}
	in := LinkInput{
		URL:    u,
		Label:  cleanText(label, maxLabelRunes),
		Group:  cleanText(group, maxGroupRunes),
		Source: source,
		At:     now,
	}
	if in.Label == u {
		in.Label = ""
	}
	if in.Group == DefaultLinkGroup {
		in.Group = ""
	}
	switch in.Source {
	case "":
		in.Source = SourceAssistant
	case SourceAssistant, SourceTool:
	default:
		return LinkInput{}, fmt.Errorf("source が読めない (assistant か tool): %q", truncRunes(source, quoteRunes))
	}
	if at != "" {
		t, err := time.Parse(time.RFC3339, at)
		if err != nil {
			return LinkInput{}, fmt.Errorf("at が RFC 3339 の時刻でない: %q", truncRunes(at, quoteRunes))
		}
		in.At = t
	}
	return in, nil
}

// checkLinkURL は URL が http(s) の絶対 URL かを確かめ、前後の空白を落とした形を返す。
//
// 中の空白と制御文字も拒む。url.Parse はパスの空白を通すが、それは呼ぶ側が URL の
// 切り出しを誤ったときに起きる。日本語などの非 ASCII は通す (cc-pages new の URL は
// パスの日本語を %XX に潰さない)。
func checkLinkURL(raw string) (string, error) {
	u := strings.TrimSpace(raw)
	if u == "" {
		return "", errors.New("url が無い")
	}
	if !utf8.ValidString(u) {
		return "", errors.New("url が UTF-8 として読めない")
	}
	if utf8.RuneCountInString(u) > MaxLinkURLRunes {
		return "", fmt.Errorf("url が長すぎる (%d 字まで)", MaxLinkURLRunes)
	}
	for _, r := range u {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return "", fmt.Errorf("url に空白か制御文字が入っている: %q", truncRunes(u, quoteRunes))
		}
	}
	p, err := url.Parse(u)
	if err != nil || (p.Scheme != "http" && p.Scheme != "https") || p.Host == "" {
		return "", fmt.Errorf("url が http(s) の絶対 URL でない: %q", truncRunes(u, quoteRunes))
	}
	return u, nil
}

// cleanText は制御文字を空白に替え、空白の並びを 1 つにして前後を落とし、max 字で切る。
func cleanText(s string, max int) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > max {
		s = string(r[:max-1]) + "…"
	}
	return s
}

// truncRunes は s を n 字までに切る (切ったら末尾に …)。
func truncRunes(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// linkLine は --stdin の 1 行 (JSON Lines の 1 件)。知らないキーは読み飛ばす。
type linkLine struct {
	URL    string `json:"url"`
	Label  string `json:"label"`
	Group  string `json:"group"`
	Source string `json:"source"`
	At     string `json:"at"`
}

// ParseLinkLines は r から JSON Lines を読み、1 行 1 件の LinkInput にする。
//
// 読めない行は捨て、行番号と理由を warn に 1 行ずつ渡す。1 件のために全部を
// 捨てないのは、呼ぶ側が turn の終わりごとに打つから (全部を落とすと取り逃しが
// 続く)。空行は件に数えない。
func ParseLinkLines(r io.Reader, now time.Time, warn func(string)) (items []LinkInput, skipped int) {
	return parseLinkLines(r, now, warn, maxLinkLineBytes, maxLinkInputBytes)
}

// parseLinkLines は ParseLinkLines の本体。上限を引数に取るのはテストのため。
//
// maxLine を超える行は行末まで読み捨てて次の行へ進む。入力全体は maxTotal バイトで
// 読むのをやめる。終わらない入力でも、cc-pages link が止まらないようにする。
func parseLinkLines(r io.Reader, now time.Time, warn func(string), maxLine, maxTotal int) (items []LinkInput, skipped int) {
	lr := &io.LimitedReader{R: r, N: int64(maxTotal) + 1}
	// 行の中身と "\r\n" が収まる大きさにしておけば、超えた行は ErrBufferFull で分かる。
	br := bufio.NewReaderSize(lr, maxLine+2)
	tooLong := fmt.Sprintf("行が長すぎる (%d バイトまで)", maxLine)
	drop := func(n int, reason string) {
		skipped++
		warn(fmt.Sprintf("%d 行目を捨てた: %s", n, reason))
	}
	take := func(n int, line []byte) {
		in, reason, ok := parseLinkLine(line, now, maxLine, tooLong)
		switch {
		case ok:
			items = append(items, in)
		case reason != "":
			drop(n, reason)
		}
	}

	for n := 1; ; n++ {
		line, err := br.ReadSlice('\n')
		if errors.Is(err, bufio.ErrBufferFull) {
			for errors.Is(err, bufio.ErrBufferFull) {
				_, err = br.ReadSlice('\n')
			}
			// 上限で切れた行は、壊れた 1 行ではなく読むのをやめた所として扱う (下の warn)
			if err == nil || !cutByLimit(lr) {
				drop(n, tooLong)
			}
			if err != nil {
				break
			}
			continue
		}
		if err != nil {
			// 改行で終わらない最後の行
			if len(line) > 0 && !cutByLimit(lr) {
				take(n, line)
			}
			break
		}
		take(n, line)
	}
	if cutByLimit(lr) {
		warn(fmt.Sprintf("入力が上限 (%d バイト) を超えたので、そこから先は読んでいない", maxTotal))
	}
	return items, skipped
}

// cutByLimit は入力が上限で切られたかを返す。上限より 1 バイト多く読めたら切ったことになる。
func cutByLimit(lr *io.LimitedReader) bool { return lr.N <= 0 }

// parseLinkLine は 1 行を読む。通れば ok。空行は ok も reason も空 (件に数えない)。
func parseLinkLine(line []byte, now time.Time, maxLine int, tooLong string) (in LinkInput, reason string, ok bool) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return LinkInput{}, "", false
	}
	if len(line) > maxLine {
		return LinkInput{}, tooLong, false
	}
	var l linkLine
	if err := json.Unmarshal(line, &l); err != nil {
		return LinkInput{}, "JSON として読めない", false
	}
	in, err := NewLinkInput(l.URL, l.Label, l.Group, l.Source, l.At, now)
	if err != nil {
		return LinkInput{}, err.Error(), false
	}
	return in, "", true
}

const (
	// linksLockName は links.json を読み・まとめ・書く間だけ握る lock のファイル名。
	linksLockName = "links.lock"
	// lockTimeout は lock を待つ上限。ふつうは数 ms で取れる。
	lockTimeout = 3 * time.Second
)

// LinkResult は cc-pages link が stdout に出す JSON。
type LinkResult struct {
	URL     string `json:"url"`     // 会話のトップページ
	Added   int    `json:"added"`   // 新しく足した URL の数
	Updated int    `json:"updated"` // 前からあった URL のうち、時刻か出どころが変わった数
	Skipped int    `json:"skipped"` // 捨てた件数

	// Dir と Changed は stdout に出さない。cmdLink が serve に知らせるかどうかと、
	// どの会話を読み直させるかに使う。
	Dir     string `json:"-"`
	Changed bool   `json:"-"`
}

// AddLinks は会話の links.json に items をまとめて書く。
//
// 会話のディレクトリが無ければ new・status と同じ規則で作り、session.json の骨も
// 書く。ただし items が 0 件ならディスクに触らない。呼ぶ側 (Stop hook) はリンクの
// 無い会話でも打つので、ここで作ると一覧が空の会話で埋まる。そのときの URL は、
// 会話のディレクトリがあればそのトップページ、無ければ /s/<session_id>/ (後で
// 何かが書かれればトップページへ飛ぶ入口) を返す。
//
// 読み・まとめ・書きは会話のディレクトリの lock の下で行う。同じ会話に 2 つの
// cc-pages link が同時に来ても、片方の書き込みが消えないようにするため。
// 何も変わらなければ書かない (呼ぶ側は turn ごとに同じ組を渡し直しうる)。
//
// items は NewLinkInput を通ったものを渡す。
func AddLinks(root, baseURL string, now time.Time, sessionID string, items []LinkInput, warn func(string)) (LinkResult, error) {
	if sessionID == "" {
		return LinkResult{}, errors.New("session id が空")
	}
	if err := ValidateSessionID(sessionID); err != nil {
		return LinkResult{}, err
	}
	base := strings.TrimRight(baseURL, "/")
	sessionsDir := filepath.Join(root, "sessions")

	// sessionDirName は日付 + 検証済みの session id の先頭部分なので、そのまま
	// 連結してよい (CreatePage の URL と同じ理屈)。
	if len(items) == 0 {
		if name, ok := FindSessionDir(sessionsDir, sessionID); ok {
			return LinkResult{URL: base + "/p/" + name + "/", Dir: name}, nil
		}
		return LinkResult{URL: base + "/s/" + url.PathEscape(sessionID) + "/"}, nil
	}

	if err := os.MkdirAll(sessionsDir, 0o700); err != nil {
		return LinkResult{}, err
	}
	name, err := findOrMakeSessionDir(sessionsDir, sessionID, now)
	if err != nil {
		return LinkResult{}, err
	}
	sessionPath := filepath.Join(sessionsDir, name)
	res := LinkResult{URL: base + "/p/" + name + "/", Dir: name}

	unlock, err := lockFile(filepath.Join(sessionPath, linksLockName), lockTimeout)
	if err != nil {
		return LinkResult{}, err
	}
	defer unlock()

	cur, err := loadLinksForUpdate(sessionPath, warn)
	if err != nil {
		return LinkResult{}, err
	}
	var changed bool
	res.Added, res.Updated, res.Skipped, changed = mergeLinks(&cur, items, warn)
	if !changed {
		return res, nil
	}
	cur.Schema = SchemaVersion
	cur.UpdatedAt = now
	if err := writeLinks(sessionPath, cur); err != nil {
		return LinkResult{}, err
	}
	// LastSeen を進める。リンクが増えたのも会話の活動なので、一覧で上に来てよい。
	if err := touchSession(sessionPath, name, sessionID, now); err != nil {
		return LinkResult{}, err
	}
	res.Changed = true
	return res, nil
}

// loadLinksForUpdate は書き足す前の links.json を読む。無ければ空。
//
// JSON として壊れていれば (手で直そうとして崩したときにしか起きない)、
// links.json.broken に退けて空から書き直す。呼ぶ側は失敗を黙って捨てるので、
// 失敗を返し続けると、誰も気づかないままリンクを取り逃す。壊れた中身は消さない。
func loadLinksForUpdate(dir string, warn func(string)) (Links, error) {
	l, err := ReadLinks(dir)
	if err == nil {
		return l, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return Links{}, nil
	}
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	if !errors.As(err, &syntaxErr) && !errors.As(err, &typeErr) {
		return Links{}, err // 読めない (権限など)。終了コード 1 にする
	}
	path := filepath.Join(dir, LinksFileName)
	if err := os.Rename(path, path+".broken"); err != nil {
		return Links{}, err
	}
	warn(fmt.Sprintf("links.json が壊れていたので %s に退けて、空から書き直す", path+".broken"))
	return Links{}, nil
}

// mergeLinks は items を cur にまとめる。
//
// 同じ URL (文字列で比べる) には、最初の時刻は小さい方、最後の時刻は大きい方、
// 出どころは和を残す。label・group は最初に入ったものを残し、前が空のときだけ
// 埋める。updated は前からあった URL のうち時刻か出どころが変わった数で、label・
// group を埋めただけでは数えない (changed は立てて書き直す)。新しい URL は MaxLinks に
// 達していれば捨てる。保存する時刻はローカルに揃える (status.json と同じく、人が
// 読んだときに分かる形)。
func mergeLinks(cur *Links, items []LinkInput, warn func(string)) (added, updated, skipped int, changed bool) {
	pos := make(map[string]int, len(cur.Links))
	for i, l := range cur.Links {
		if _, dup := pos[l.URL]; !dup {
			pos[l.URL] = i
		}
	}
	preexisting := len(cur.Links) // これより前の添字が「前からあった」URL
	updatedURL := map[string]bool{}

	for _, in := range items {
		at := in.At.Local()
		i, ok := pos[in.URL]
		if !ok {
			if len(cur.Links) >= MaxLinks {
				skipped++
				warn(fmt.Sprintf("上限 (%d 件) に達しているので足さない: %q", MaxLinks, truncRunes(in.URL, quoteRunes)))
				continue
			}
			cur.Links = append(cur.Links, Link{
				URL: in.URL, Label: in.Label, Group: in.Group,
				Sources: []string{in.Source}, FirstSeen: at, LastSeen: at,
			})
			pos[in.URL] = len(cur.Links) - 1
			added++
			changed = true
			continue
		}

		l := &cur.Links[i]
		moved := false
		if l.FirstSeen.IsZero() || at.Before(l.FirstSeen) {
			l.FirstSeen, moved = at, true
		}
		if at.After(l.LastSeen) {
			l.LastSeen, moved = at, true
		}
		if srcs, grew := addSource(l.Sources, in.Source); grew {
			l.Sources, moved = srcs, true
		}
		if l.Label == "" && in.Label != "" {
			l.Label, changed = in.Label, true
		}
		if l.Group == "" && in.Group != "" {
			l.Group, changed = in.Group, true
		}
		if moved {
			changed = true
			if i < preexisting && !updatedURL[in.URL] {
				updatedURL[in.URL] = true
				updated++
			}
		}
	}
	return added, updated, skipped, changed
}

// addSource は srcs に s を足した列を返す。並びは assistant, tool の順 (知らない値は
// 後ろ) に揃え、ファイルの差分が出どころの来た順で揺れないようにする。
func addSource(srcs []string, s string) ([]string, bool) {
	for _, x := range srcs {
		if x == s {
			return srcs, false
		}
	}
	out := append(append([]string(nil), srcs...), s)
	sort.SliceStable(out, func(i, j int) bool { return sourceRank(out[i]) < sourceRank(out[j]) })
	return out, true
}

func sourceRank(s string) int {
	switch s {
	case SourceAssistant:
		return 0
	case SourceTool:
		return 1
	}
	return 2
}
