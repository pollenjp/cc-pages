package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
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

// StatusInput は cc-pages status のフラグに対応する。
type StatusInput struct {
	SessionID string
	Now       []string
	Next      []string
}

// StatusResult は cc-pages status が stdout に出す JSON。
type StatusResult struct {
	URL string `json:"url"` // セッションのページ一覧 (= 状況板)
}

// UpdateStatus はセッションディレクトリを用意し、status.json を全置換で書く。
//
// 差分ではなく全置換にするのは、「今の状況を全部言い直す」方が skill の指示が
// 単純で、更新し忘れた項目が残り続ける事故が起きないため。
//
// セッションディレクトリが無ければ new と同じ規則で作り、session.json の骨も
// 書く。ページより先に状況だけ書いてよい (セッションの最初に計画だけ残したい
// 場面がある)。dir が無いからと失敗させると、skill 側に順序の約束が増える。
func UpdateStatus(root, baseURL string, now time.Time, in StatusInput) (StatusResult, error) {
	if in.SessionID == "" {
		return StatusResult{}, errors.New("session id が空")
	}
	if err := validateSessionID(in.SessionID); err != nil {
		return StatusResult{}, err
	}
	nowItems, nextItems := cleanItems(in.Now), cleanItems(in.Next)
	// 検証はディスクに触る前に済ませる。拒否したときに空のディレクトリを残さない。
	if len(nowItems) == 0 && len(nextItems) == 0 {
		return StatusResult{}, errors.New("now か next のどちらかは要る")
	}

	sessionsDir := filepath.Join(root, "sessions")
	if err := os.MkdirAll(sessionsDir, 0o700); err != nil {
		return StatusResult{}, err
	}
	sessionDirName, err := findOrMakeSessionDir(sessionsDir, in.SessionID, now)
	if err != nil {
		return StatusResult{}, err
	}
	sessionPath := filepath.Join(sessionsDir, sessionDirName)

	if err := WriteStatus(sessionPath, Status{
		Schema: SchemaVersion, Now: nowItems, Next: nextItems, UpdatedAt: now,
	}); err != nil {
		return StatusResult{}, err
	}
	// LastSeen を進める。状況を書くのも活動なので、一覧で上に来てよい。
	if err := touchSession(sessionPath, sessionDirName, in.SessionID, now); err != nil {
		return StatusResult{}, err
	}

	// sessionDirName は日付 + 検証済みの session id の先頭部分なので、そのまま
	// 連結してよい (CreatePage の URL と同じ理屈)。末尾のスラッシュは
	// GET /p/{session}/{$} の正規形。
	return StatusResult{URL: strings.TrimRight(baseURL, "/") + "/p/" + sessionDirName + "/"}, nil
}

// cleanItems は前後の空白を落とし、空になった項目を捨てる。
//
// 0 件なら nil を返す。json の omitempty で "now": [] ではなくキーごと省かれる。
func cleanItems(items []string) []string {
	var out []string
	for _, s := range items {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}
