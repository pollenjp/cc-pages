package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// SessionFileName は session.json のファイル名。
const SessionFileName = "session.json"

// Session はセッション単位のメタ。
//
// 骨 (Schema / SessionID / Dir / FirstSeen / LastSeen) は CreatePage が書く。
// AITitle / Cwd / GitBranch / Transcript はインデクサが後から埋める。
// skill はこのファイルに触らない。
type Session struct {
	Schema     int       `json:"schema"`
	SessionID  string    `json:"session_id"`
	Dir        string    `json:"dir"` // ディレクトリ名のみ (絶対パスではない)
	FirstSeen  time.Time `json:"first_seen"`
	LastSeen   time.Time `json:"last_seen"`
	Cwd        string    `json:"cwd,omitempty"`
	GitBranch  string    `json:"git_branch,omitempty"`
	AITitle    string    `json:"ai_title,omitempty"`
	Transcript string    `json:"transcript,omitempty"`
}

// ReadSession は dir/session.json を読む。
func ReadSession(dir string) (Session, error) {
	b, err := os.ReadFile(filepath.Join(dir, SessionFileName))
	if err != nil {
		return Session{}, err
	}
	var s Session
	if err := json.Unmarshal(b, &s); err != nil {
		return Session{}, err
	}
	return s, nil
}

// WriteSession は dir/session.json を書く。
func WriteSession(dir string, s Session) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, SessionFileName), append(b, '\n'), 0o600)
}
