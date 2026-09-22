package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// StatusFileName は status.json のファイル名。
const StatusFileName = "status.json"

// Status はセッション単位の「現在の状況」。
//
// 書くのは UpdateStatus (cc-pages status) だけ。skill はこのスキーマを知らず、
// CLI に文字列の配列を渡すだけ。最新の 1 つだけを持ち、履歴は持たない。
//
// session.json と分けているのは、あちらが new の read-modify-write する骨で、
// 混ぜると書き込みが競合するため。こちらは丸ごと上書きしかしない。
// session_id も持たない。同じディレクトリの session.json が正で、二重に持つと
// 食い違いの余地ができる。
type Status struct {
	Schema    int       `json:"schema"`
	Now       []string  `json:"now,omitempty"`
	Next      []string  `json:"next,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Empty は now も next も無いかを返す。
//
// 読む側は Empty な status を「無い」ものとして扱う。手で編集した場合にしか
// 起きない (UpdateStatus は両方空を拒む)。
func (s Status) Empty() bool { return len(s.Now) == 0 && len(s.Next) == 0 }

// ReadStatus は dir/status.json を読む。
func ReadStatus(dir string) (Status, error) {
	b, err := os.ReadFile(filepath.Join(dir, StatusFileName))
	if err != nil {
		return Status{}, err
	}
	var s Status
	if err := json.Unmarshal(b, &s); err != nil {
		return Status{}, err
	}
	return s, nil
}

// WriteStatus は dir/status.json を丸ごと書く。
func WriteStatus(dir string, s Status) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, StatusFileName), append(b, '\n'), 0o600)
}
