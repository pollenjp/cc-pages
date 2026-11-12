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
// 書くのは CreatePage だけで、内容は骨 (Schema / SessionID / Dir /
// FirstSeen / LastSeen) に限る。Cwd / GitBranch は走査時にページから補われる
// ことがあるが、それもメモリ上の話でファイルには書き戻さない。
// jsonl 由来のメタ (AI がつけたタイトルなど) は索引を組み立てるときに
// メモリ上で合流させるだけで、ここには一切永続化しない。jsonl は非公開
// フォーマットで壊れ得るものなので、それを自分のファイルに焼き付けない。
// skill はこのファイルに触らない。
type Session struct {
	Schema    int       `json:"schema"`
	SessionID string    `json:"session_id"`
	Dir       string    `json:"dir"` // ディレクトリ名のみ (絶対パスではない)
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
	Cwd       string    `json:"cwd,omitempty"`
	GitBranch string    `json:"git_branch,omitempty"`
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
//
// 一時ファイルに書いて rename する。cc-pages link が turn の終わりごとに書くので、
// 切り詰めてから書くと、その隙に読んだ側 (別の cc-pages の findOrMakeSessionDir や
// serve の走査) が空の JSON を見る。findOrMakeSessionDir がそれを「知らない会話」と
// 取り違えると、会話のディレクトリが 2 つに割れる。
func WriteSession(dir string, s Session) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(dir, SessionFileName), append(b, '\n'), 0o600)
}
