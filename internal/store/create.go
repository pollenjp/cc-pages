package store

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
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
		// sessionDirName / pageDirName は Slug/SessionDirName/PageDirName が
		// 数字・ハイフン・Unicode の文字/数字だけに絞っているので、そのまま繋げる。
		// url.PathEscape は使わない — 非 ASCII を %XX に潰してしまい、日本語タイトルの
		// URL が読めなくなる。
		URL: strings.TrimRight(baseURL, "/") + "/p/" + sessionDirName + "/" + pageDirName,
	}, nil
}

// findOrMakeSessionDir は session id に対応するディレクトリ名を返す。
//
// 既存ディレクトリの session.json を読んで一致を判定する。ディレクトリ名の
// 先頭 8 桁で照合しないのは、衝突したときに別セッションのページを取り違えるため。
func findOrMakeSessionDir(sessionsDir, sessionID string, now time.Time) (string, error) {
	ents, err := os.ReadDir(sessionsDir)
	if err != nil {
		return "", err
	}
	taken := map[string]bool{}
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		taken[e.Name()] = true
		s, err := ReadSession(filepath.Join(sessionsDir, e.Name()))
		if err == nil && s.SessionID == sessionID {
			return e.Name(), nil
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
	return name, os.MkdirAll(filepath.Join(sessionsDir, name), 0o700)
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
