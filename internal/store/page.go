package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// SchemaVersion は page.json / session.json のスキーマ版。
const SchemaVersion = 1

// ページの描画モード。
const (
	// ModeFragment は index.html を <body> の中身として扱う (既定)。
	ModeFragment = "fragment"
	// ModeStandalone は index.html を完全な HTML として扱い、iframe に入れる。
	ModeStandalone = "standalone"
)

// PageFileName は page.json のファイル名。
const PageFileName = "page.json"

// Page は 1 ページ分のメタ。skill はこのスキーマを知らない。
type Page struct {
	Schema    int       `json:"schema"`
	ID        string    `json:"id"` // "0007"
	Title     string    `json:"title"`
	Summary   string    `json:"summary,omitempty"`
	Tags      []string  `json:"tags,omitempty"`
	Prompt    string    `json:"prompt,omitempty"`
	Mode      string    `json:"mode"`
	CreatedAt time.Time `json:"created_at"`
	SessionID string    `json:"session_id"`
	Cwd       string    `json:"cwd,omitempty"`
	GitBranch string    `json:"git_branch,omitempty"`
}

// ReadPage は dir/page.json を読む。
func ReadPage(dir string) (Page, error) {
	b, err := os.ReadFile(filepath.Join(dir, PageFileName))
	if err != nil {
		return Page{}, err
	}
	var p Page
	if err := json.Unmarshal(b, &p); err != nil {
		return Page{}, err
	}
	return p, nil
}

// WritePage は dir/page.json を書く。
func WritePage(dir string, p Page) error {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, PageFileName), append(b, '\n'), 0o600)
}
