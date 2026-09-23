# セッション状況板 (cc-pages status / status.json) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `cc-pages status` がセッション直下の `status.json` を全置換で書き、`/p/<session>/` に「現在の状況」「次やること」を出す。skill はページを書き終えるたびにそれを打つ。

**Architecture:** ディスク形式は `internal/store` に閉じたまま `status.json` を 1 つ足す。`store.Scan` が拾い、`index` が `SessionView.Status` に写し、`server` が `session.html` で描く。CLI は `new` と同じ形の `status` サブコマンドを足すだけで、skill は CLI の引数と戻りの URL しか知らない。

**Tech Stack:** Go 1.24 (`go.mod`; 手元は mise の go 1.27)、`html/template`、標準ライブラリのみ。e2e は Playwright 1.62.1 (`tests/e2e`)、ブラウザは `nix develop` の devShell が供給する。

**Spec:** `docs/superpowers/specs/2026-09-22-session-status-design.md`

## Global Constraints

- 新しい依存は足さない (`go.mod` の require は `github.com/BurntSushi/toml` のみのまま)
- ディスク上の形式 (ファイル名・スキーマ・採番) を触るコードは `internal/store` にだけ書く
- `page.json` / `session.json` のスキーマは変えない。`status.json` は `schema: 1`
- ディレクトリは `0o700`、ファイルは `0o600` で作る (既存と同じ)
- 壊れた `status.json` は黙って飛ばす。now と next が両方とも空の `status.json` は「無い」ものとして扱う
- status の項目は `html/template` の自動エスケープに任せる。`template.HTML` に包まない
- CI (`.github/workflows/go.yml`) は `gofmt -l .` が空、`go vet ./...`、`go test ./...` を要求する。各 task の最後にこの 3 つを通す。`gofmt -l .` にファイルが出たら `gofmt -w <file>` で整形してから commit する (plan 中のコード片は複合リテラルの揃えまでは保証しない)
- コメント・エラー文・UI 文言は日本語。コメントは「何をするか」ではなく「なぜそうするか」を書く (既存の流儀)
- コミットメッセージは `type(scope): 日本語の要約` + 空行 + 本文、末尾に `Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>`
- 作業ブランチはこの worktree の `worktree/green-meadow-776b`。`main` には直接 commit しない
- skill 側 (`~/ghq/github.com/pollenjp/claude-skills`) は今 `main` で clean。必ず branch を切ってから触る
- 反映の順序: app のバイナリを差し替えてから skill を変える (逆だと skill の手順 3 が毎回失敗する)

## File Structure

| ファイル | 役割 |
| --- | --- |
| `internal/store/status.go` (新規) | `Status` 型、`ReadStatus` / `WriteStatus`、`StatusInput` / `StatusResult`、`UpdateStatus`、`cleanItems`、`statusModTime` |
| `internal/store/status_test.go` (新規) | 上記のテスト |
| `internal/store/scan.go` | `SessionEntry` に `Status` / `StatusModTime`。`readSession` が status.json を読み、`Scan` の差分判定に mtime を足す |
| `internal/store/scan_test.go` | status を拾う / 壊れたものを飛ばす / 上書きで読み直す |
| `internal/index/index.go` | `StatusView`、`SessionView.Status`、`buildView` / `buildHaystack` |
| `internal/index/index_test.go` | view への写しと検索 |
| `internal/server/render.go` | `statusRow`、`sessionRow.Status`、`toRow` |
| `internal/server/status_test.go` (新規) | session ページの 3 状態・エスケープ・一覧が変わらないこと |
| `internal/web/session.html` | 状況板の 2 枠と空状態 |
| `internal/web/static/style.css` | `.status` の見た目 |
| `main.go` / `main_test.go` | `status` サブコマンド (`cmdStatus`)。`tagList` を `multiFlag` に改名 |
| `tests/e2e/fixtures/root/sessions/20260101-e2efixture/status.json` (新規) | e2e の fixture |
| `tests/e2e/session.spec.ts` (新規) | 状況板の検査とライト / ダークのスクショ (08, 09) |
| `README.md` | `status` の使い方と状況板の説明 |
| `claude-skills/skills/pjp-cc-page/SKILL.md` | 手順 3「セッションの状況を更新する」、`--summary` の必須化 |

既存のテストヘルパで使うもの:

- `internal/store`: `fixedTime()` (`create_test.go`)、`seed()` / `only()` (`scan_test.go`)
- `internal/index`: `at()` / `entry()` (`index_test.go`)
- `internal/server`: `at()` / `mkEntry()` / `get()` (`server_test.go`)

---

### Task 1: store — status.json の読み書き

**Files:**
- Create: `internal/store/status.go`
- Create: `internal/store/status_test.go`

**Interfaces:**
- Consumes: `SchemaVersion` (`page.go`)、`fixedTime()` (`create_test.go`)
- Produces: `const StatusFileName = "status.json"`、`type Status struct{ Schema int; Now, Next []string; UpdatedAt time.Time }`、`func (s Status) Empty() bool`、`func ReadStatus(dir string) (Status, error)`、`func WriteStatus(dir string, s Status) error`

- [ ] **Step 1: 失敗するテストを書く**

`internal/store/status_test.go`:

```go
package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStatusRoundTrip(t *testing.T) {
	dir := t.TempDir()
	in := Status{
		Schema: SchemaVersion,
		Now:    []string{"設計を提示", "承認待ち"},
		Next:   []string{"spec を commit"},
		UpdatedAt: fixedTime(),
	}
	if err := WriteStatus(dir, in); err != nil {
		t.Fatalf("WriteStatus() error = %v", err)
	}
	got, err := ReadStatus(dir)
	if err != nil {
		t.Fatalf("ReadStatus() error = %v", err)
	}
	if got.Schema != SchemaVersion {
		t.Errorf("Schema = %d, want %d", got.Schema, SchemaVersion)
	}
	if len(got.Now) != 2 || got.Now[0] != "設計を提示" || got.Now[1] != "承認待ち" {
		t.Errorf("Now = %v", got.Now)
	}
	if len(got.Next) != 1 || got.Next[0] != "spec を commit" {
		t.Errorf("Next = %v", got.Next)
	}
	if !got.UpdatedAt.Equal(fixedTime()) {
		t.Errorf("UpdatedAt = %v, want %v", got.UpdatedAt, fixedTime())
	}
	fi, err := os.Stat(filepath.Join(dir, StatusFileName))
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("status.json のパーミッション = %o, want 600", perm)
	}
}

func TestReadStatusMissing(t *testing.T) {
	if _, err := ReadStatus(t.TempDir()); !os.IsNotExist(err) {
		t.Errorf("無いときのエラー = %v, want IsNotExist", err)
	}
}

func TestReadStatusBrokenJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, StatusFileName), []byte("{ではない"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadStatus(dir); err == nil {
		t.Error("壊れた JSON がエラーにならなかった")
	}
}

func TestStatusEmpty(t *testing.T) {
	cases := []struct {
		name string
		s    Status
		want bool
	}{
		{"両方無し", Status{}, true},
		{"now だけ", Status{Now: []string{"x"}}, false},
		{"next だけ", Status{Next: []string{"x"}}, false},
	}
	for _, c := range cases {
		if got := c.s.Empty(); got != c.want {
			t.Errorf("%s: Empty() = %v, want %v", c.name, got, c.want)
		}
	}
}
```

- [ ] **Step 2: 落ちることを確認する**

Run: `go test ./internal/store/ -run 'TestStatus|TestReadStatus' 2>&1 | head -20`
Expected: コンパイルエラー (`undefined: Status`, `undefined: WriteStatus` など)

- [ ] **Step 3: 最小の実装を書く**

`internal/store/status.go`:

```go
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
```

- [ ] **Step 4: 通ることを確認する**

Run: `gofmt -l . && go vet ./... && go test ./internal/store/`
Expected: gofmt の出力なし、PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/status.go internal/store/status_test.go
git commit -m "$(cat <<'EOF'
feat(store): status.json の読み書きを足す

セッション直下に「現在の状況」を持つ status.json を足す。session.json と
分けるのは、あちらが new の read-modify-write する骨で、混ぜると書き込みが
競合するため。こちらは丸ごと上書きしかしない。

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: store — UpdateStatus (全置換・dir の用意・URL)

**Files:**
- Modify: `internal/store/status.go`
- Modify: `internal/store/status_test.go`

**Interfaces:**
- Consumes: `validateSessionID`、`findOrMakeSessionDir`、`touchSession` (`create.go`)、`WriteStatus` (Task 1)、`CreatePage` / `ReadSession` (テスト)
- Produces: `type StatusInput struct{ SessionID string; Now, Next []string }`、`type StatusResult struct{ URL string \`json:"url"\` }`、`func UpdateStatus(root, baseURL string, now time.Time, in StatusInput) (StatusResult, error)`、`func cleanItems(items []string) []string`

- [ ] **Step 1: 失敗するテストを書く**

`internal/store/status_test.go` に追記 (import に `"encoding/json"`、`"time"` を足す):

```go
const statusBase = "http://localhost:7777"

func TestUpdateStatusWritesStatusAndSkeleton(t *testing.T) {
	root := t.TempDir()
	got, err := UpdateStatus(root, statusBase, fixedTime(), StatusInput{
		SessionID: "409bfd08-aaaa",
		Now:       []string{"設計を提示", " 承認待ち "},
		Next:      []string{"spec を commit", "", "   "},
	})
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	if want := statusBase + "/p/20260829-409bfd08/"; got.URL != want {
		t.Errorf("URL = %q, want %q", got.URL, want)
	}

	sessionPath := filepath.Join(root, "sessions", "20260829-409bfd08")
	s, err := ReadStatus(sessionPath)
	if err != nil {
		t.Fatalf("ReadStatus() error = %v", err)
	}
	if s.Schema != SchemaVersion {
		t.Errorf("Schema = %d, want %d", s.Schema, SchemaVersion)
	}
	// trim され、空になった項目は落ちる
	if len(s.Now) != 2 || s.Now[0] != "設計を提示" || s.Now[1] != "承認待ち" {
		t.Errorf("Now = %q", s.Now)
	}
	if len(s.Next) != 1 || s.Next[0] != "spec を commit" {
		t.Errorf("Next = %q", s.Next)
	}
	if !s.UpdatedAt.Equal(fixedTime()) {
		t.Errorf("UpdatedAt = %v, want %v", s.UpdatedAt, fixedTime())
	}

	// ページより先に打ったので、session.json の骨はここで書かれる
	sess, err := ReadSession(sessionPath)
	if err != nil {
		t.Fatalf("ReadSession() error = %v", err)
	}
	if sess.SessionID != "409bfd08-aaaa" || sess.Dir != "20260829-409bfd08" {
		t.Errorf("Session = %+v", sess)
	}
	if !sess.LastSeen.Equal(fixedTime()) {
		t.Errorf("LastSeen = %v, want %v", sess.LastSeen, fixedTime())
	}

	// stdout に出す JSON は {"url": ...} の 1 キー
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var back map[string]string
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if len(back) != 1 || back["url"] == "" {
		t.Errorf("JSON = %s, want url だけ", b)
	}
}

func TestUpdateStatusReplacesWhole(t *testing.T) {
	root := t.TempDir()
	first := StatusInput{SessionID: "409bfd08-aaaa", Now: []string{"古い"}, Next: []string{"古い次"}}
	if _, err := UpdateStatus(root, statusBase, fixedTime(), first); err != nil {
		t.Fatal(err)
	}
	later := fixedTime().Add(time.Hour)
	second := StatusInput{SessionID: "409bfd08-aaaa", Now: []string{"新しい"}}
	if _, err := UpdateStatus(root, statusBase, later, second); err != nil {
		t.Fatal(err)
	}

	sessionPath := filepath.Join(root, "sessions", "20260829-409bfd08")
	s, err := ReadStatus(sessionPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Now) != 1 || s.Now[0] != "新しい" {
		t.Errorf("Now = %q, want [新しい]", s.Now)
	}
	// 差分ではなく全置換。前回の next は残らない
	if len(s.Next) != 0 {
		t.Errorf("前回の next が残っている: %q", s.Next)
	}
	if !s.UpdatedAt.Equal(later) {
		t.Errorf("UpdatedAt = %v, want %v", s.UpdatedAt, later)
	}
	sess, err := ReadSession(sessionPath)
	if err != nil {
		t.Fatal(err)
	}
	if !sess.LastSeen.Equal(later) {
		t.Errorf("LastSeen = %v, want %v (status でも進める)", sess.LastSeen, later)
	}
}

func TestUpdateStatusReusesSessionDirOfPages(t *testing.T) {
	root := t.TempDir()
	res, err := CreatePage(root, statusBase, fixedTime(), NewPageInput{SessionID: "409bfd08-aaaa", Title: "T"})
	if err != nil {
		t.Fatal(err)
	}
	// 日付をまたいでも、同じ session id なら同じディレクトリに書く
	nextDay := fixedTime().Add(24 * time.Hour)
	got, err := UpdateStatus(root, statusBase, nextDay, StatusInput{SessionID: "409bfd08-aaaa", Now: []string{"x"}})
	if err != nil {
		t.Fatal(err)
	}
	sessionPath := filepath.Dir(res.Dir)
	if _, err := os.Stat(filepath.Join(sessionPath, StatusFileName)); err != nil {
		t.Errorf("ページと同じセッションディレクトリに status.json が無い: %v", err)
	}
	ents, err := os.ReadDir(filepath.Join(root, "sessions"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 1 {
		t.Errorf("セッションディレクトリが割れた: %d 個", len(ents))
	}
	if want := statusBase + "/p/" + filepath.Base(sessionPath) + "/"; got.URL != want {
		t.Errorf("URL = %q, want %q", got.URL, want)
	}
}

func TestUpdateStatusRejectsNoItems(t *testing.T) {
	root := t.TempDir()
	// 空白だけの項目は落ちるので、実質 0 件
	_, err := UpdateStatus(root, statusBase, fixedTime(), StatusInput{SessionID: "409bfd08-aaaa", Now: []string{"  "}})
	if err == nil {
		t.Fatal("now も next も無いのに通った")
	}
	// 拒否したら何も作らない
	if _, err := os.Stat(filepath.Join(root, "sessions")); !os.IsNotExist(err) {
		t.Errorf("拒否したのに sessions/ が作られた")
	}
}

func TestUpdateStatusRejectsBadSession(t *testing.T) {
	for _, sid := range []string{"", "../../../x", "a/b", "a b"} {
		root := t.TempDir()
		_, err := UpdateStatus(root, statusBase, fixedTime(), StatusInput{SessionID: sid, Now: []string{"x"}})
		if err == nil {
			t.Errorf("session id %q が通ってしまった", sid)
		}
		ents, err := os.ReadDir(root)
		if err != nil {
			t.Fatal(err)
		}
		if len(ents) != 0 {
			t.Errorf("session id %q は拒否されるべきなのに root に何か作られた: %v", sid, ents)
		}
	}
}
```

- [ ] **Step 2: 落ちることを確認する**

Run: `go test ./internal/store/ -run TestUpdateStatus 2>&1 | head -20`
Expected: コンパイルエラー (`undefined: UpdateStatus`, `undefined: StatusInput`)

- [ ] **Step 3: 実装を書く**

`internal/store/status.go` の import を次にする:

```go
import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)
```

ファイル末尾に追記:

```go
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
```

- [ ] **Step 4: 通ることを確認する**

Run: `gofmt -l . && go vet ./... && go test ./internal/store/`
Expected: gofmt の出力なし、PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/status.go internal/store/status_test.go
git commit -m "$(cat <<'EOF'
feat(store): cc-pages status のために UpdateStatus を足す

status.json を全置換で書く。差分ではなく全置換にするのは、「今の状況を
全部言い直す」方が skill の指示が単純で、更新し忘れた項目が残り続ける
事故が起きないため。セッションディレクトリが無ければ new と同じ規則で
作り、ページより先に状況だけ書けるようにする。

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: store — Scan が status.json を拾い、上書きでも読み直す

**Files:**
- Modify: `internal/store/scan.go`
- Modify: `internal/store/status.go` (`statusModTime` を足す)
- Modify: `internal/store/scan_test.go`

**Interfaces:**
- Consumes: `ReadStatus` / `Status.Empty` / `StatusFileName` (Task 1)、`UpdateStatus` (Task 2)、`seed()` / `only()` (`scan_test.go`)
- Produces: `SessionEntry.Status *Status`、`SessionEntry.StatusModTime time.Time`、`func statusModTime(dir string) time.Time`

- [ ] **Step 1: 失敗するテストを書く**

`internal/store/scan_test.go` の末尾に追記 (import は既に `os` / `filepath` / `strings` / `testing` / `time` がある):

```go
func TestScanReadsStatus(t *testing.T) {
	root := t.TempDir()
	seed(t, root, "sess-aaaa", "T", fixedTime())
	if _, err := UpdateStatus(root, "http://localhost:7777", fixedTime(), StatusInput{
		SessionID: "sess-aaaa", Now: []string{"進行中"}, Next: []string{"次"},
	}); err != nil {
		t.Fatal(err)
	}
	got, err := Scan(filepath.Join(root, "sessions"), nil)
	if err != nil {
		t.Fatal(err)
	}
	e := only(t, got)
	if e.Status == nil {
		t.Fatal("status.json が拾われていない")
	}
	if len(e.Status.Now) != 1 || e.Status.Now[0] != "進行中" || len(e.Status.Next) != 1 || e.Status.Next[0] != "次" {
		t.Errorf("Status = %+v", *e.Status)
	}
	if e.StatusModTime.IsZero() {
		t.Error("StatusModTime が採られていない")
	}
	if len(e.Pages) != 1 {
		t.Errorf("ページ数 = %d, want 1", len(e.Pages))
	}
}

func TestScanSkipsBrokenStatus(t *testing.T) {
	root := t.TempDir()
	res := seed(t, root, "sess-aaaa", "T", fixedTime())
	sessionPath := filepath.Dir(res.Dir)
	if err := os.WriteFile(filepath.Join(sessionPath, StatusFileName), []byte("{ではない"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Scan(filepath.Join(root, "sessions"), nil)
	if err != nil {
		t.Fatalf("壊れた status.json で Scan がエラーになった: %v", err)
	}
	e := only(t, got)
	if e.Status != nil {
		t.Errorf("壊れた status.json が拾われた: %+v", *e.Status)
	}
	if len(e.Pages) != 1 {
		t.Errorf("ページまで飛んだ: %d", len(e.Pages))
	}
}

func TestScanTreatsEmptyStatusAsAbsent(t *testing.T) {
	root := t.TempDir()
	res := seed(t, root, "sess-aaaa", "T", fixedTime())
	// 手で編集して now も next も消えた status.json
	if err := WriteStatus(filepath.Dir(res.Dir), Status{Schema: SchemaVersion, UpdatedAt: fixedTime()}); err != nil {
		t.Fatal(err)
	}
	got, err := Scan(filepath.Join(root, "sessions"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if e := only(t, got); e.Status != nil {
		t.Errorf("空の status.json が「有る」扱いになった: %+v", *e.Status)
	}
}

func TestScanReusesSessionWithUnchangedStatus(t *testing.T) {
	root := t.TempDir()
	seed(t, root, "sess-aaaa", "T", fixedTime())
	sessionsDir := filepath.Join(root, "sessions")
	if _, err := UpdateStatus(root, "http://localhost:7777", fixedTime(), StatusInput{
		SessionID: "sess-aaaa", Now: []string{"進行中"},
	}); err != nil {
		t.Fatal(err)
	}
	first, err := Scan(sessionsDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	// prev の status を差し替えておく。再利用されたなら差し替えたものが返る。
	for k, e := range first {
		e.Status.Now[0] = "差し替え済み"
		first[k] = e
	}
	second, err := Scan(sessionsDir, first)
	if err != nil {
		t.Fatal(err)
	}
	if e := only(t, second); e.Status == nil || e.Status.Now[0] != "差し替え済み" {
		t.Errorf("何も変わっていないのに読み直した: %+v", e.Status)
	}
}

// TestScanRereadsWhenStatusOverwritten は、status.json の上書きだけで読み直される
// ことを確認する。
//
// 上書きはセッションディレクトリの mtime を動かさない (動くのは作成・削除のとき
// だけ)。cc-pages status は毎回 touch を送るので普段は全走査で拾えるが、touch が
// 届かなかったときに定期走査が永久に取りこぼしてはいけない。
func TestScanRereadsWhenStatusOverwritten(t *testing.T) {
	root := t.TempDir()
	res := seed(t, root, "sess-aaaa", "T", fixedTime())
	sessionsDir := filepath.Join(root, "sessions")
	sessionPath := filepath.Dir(res.Dir)
	if _, err := UpdateStatus(root, "http://localhost:7777", fixedTime(), StatusInput{
		SessionID: "sess-aaaa", Now: []string{"古い"},
	}); err != nil {
		t.Fatal(err)
	}
	first, err := Scan(sessionsDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	dirBefore, err := os.Stat(sessionPath)
	if err != nil {
		t.Fatal(err)
	}

	// 上書き。ディレクトリの mtime は動かない。
	if _, err := UpdateStatus(root, "http://localhost:7777", fixedTime(), StatusInput{
		SessionID: "sess-aaaa", Now: []string{"新しい"},
	}); err != nil {
		t.Fatal(err)
	}
	// mtime の分解能が粗い FS でも差が出るよう、status.json の mtime を明示的に
	// 進め、セッションディレクトリの mtime は元に戻して「上書きだけ」を再現する。
	newer := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(filepath.Join(sessionPath, StatusFileName), newer, newer); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(sessionPath, dirBefore.ModTime(), dirBefore.ModTime()); err != nil {
		t.Fatal(err)
	}

	second, err := Scan(sessionsDir, first)
	if err != nil {
		t.Fatal(err)
	}
	if e := only(t, second); e.Status == nil || e.Status.Now[0] != "新しい" {
		t.Errorf("status.json を上書きしたのに読み直していない: %+v", e.Status)
	}
}
```

- [ ] **Step 2: 落ちることを確認する**

Run: `go test ./internal/store/ -run 'TestScanReadsStatus|TestScanSkipsBrokenStatus|TestScanTreatsEmptyStatusAsAbsent|TestScanReusesSessionWithUnchangedStatus|TestScanRereadsWhenStatusOverwritten' 2>&1 | head -20`
Expected: コンパイルエラー (`e.Status undefined`)

- [ ] **Step 3: 実装を書く**

`internal/store/status.go` の末尾に追記:

```go
// statusModTime は dir/status.json の mtime を返す。無ければゼロ値。
//
// Scan の差分判定用。status.json の上書きはセッションディレクトリの mtime を
// 動かさない (動くのは作成・削除のときだけ) ので、これも見ないと上書きが
// 定期走査から永久に見えない。削除されたときはゼロ値になり、前回の値と
// 食い違うのでやはり読み直す。
func statusModTime(dir string) time.Time {
	fi, err := os.Stat(filepath.Join(dir, StatusFileName))
	if err != nil {
		return time.Time{}
	}
	return fi.ModTime()
}
```

`internal/store/scan.go` の `SessionEntry` を次にする (`Bytes` の後ろに 2 フィールド足す):

```go
// SessionEntry は走査で見つかった 1 セッション。
type SessionEntry struct {
	Session Session
	DirName string
	DirPath string
	Pages   []PageEntry // 連番の昇順
	ModTime time.Time   // セッションディレクトリの mtime。差分判定に使う

	// PageModTime は配下のページディレクトリの mtime の最大値。差分判定に使う。
	//
	// セッションディレクトリの mtime はページの追加・削除でしか動かない。
	// skill は cc-pages new が戻った後にページディレクトリへ index.html を書くので、
	// こちらも見ないとその本文が永久に読み直されない。
	PageModTime time.Time

	// Bytes は配下のページディレクトリの合計サイズ。session.json は含まない。
	Bytes int64

	// Status はセッション直下の status.json。無い / 壊れている / 空なら nil。
	Status *Status

	// StatusModTime は status.json の mtime。無ければゼロ値。差分判定に使う。
	//
	// status.json の上書きもセッションディレクトリの mtime を動かさない。
	// cc-pages status は毎回 touch を送るが、届かなかったときに定期走査が
	// 永久に取りこぼさないよう、これも見る。
	StatusModTime time.Time
}
```

`Scan` の使い回し条件に `statusModTime` を足す:

```go
		if old, ok := prev[e.Name()]; ok && old.ModTime.Equal(fi.ModTime()) &&
			!maxPageModTime(dirPath).After(old.PageModTime) &&
			statusModTime(dirPath).Equal(old.StatusModTime) {
			out[e.Name()] = old
			continue
		}
```

`readSession` の、`entry.Session.Dir` を補う `if` の直後 (ページの `os.ReadDir` より前) に追記:

```go
	// status.json。mtime は中身を読む前に採る (ページと同じ理由)。
	// 壊れていれば黙って飛ばし、now も next も無いものは「無い」扱いにする。
	entry.StatusModTime = statusModTime(dirPath)
	if s, err := ReadStatus(dirPath); err == nil && !s.Empty() {
		entry.Status = &s
	}
```

- [ ] **Step 4: 通ることを確認する**

Run: `gofmt -l . && go vet ./... && go test ./internal/store/`
Expected: gofmt の出力なし、PASS (既存の `TestScan*` も含めて全部)

- [ ] **Step 5: Commit**

```bash
git add internal/store/scan.go internal/store/status.go internal/store/scan_test.go
git commit -m "$(cat <<'EOF'
feat(store): 走査が status.json を拾い、上書きだけでも読み直す

上書きはセッションディレクトリの mtime を動かさないので、差分判定に
status.json の mtime を足す。cc-pages status は毎回 touch を送るが、
届かなかったときに定期走査が永久に取りこぼさないようにするため。

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>
EOF
)"
```

---

### Task 4: index — 状況を索引に載せ、検索対象にする

**Files:**
- Modify: `internal/index/index.go`
- Modify: `internal/index/index_test.go`

**Interfaces:**
- Consumes: `store.SessionEntry.Status` (Task 3)、`at()` / `entry()` (`index_test.go`)
- Produces: `type StatusView struct{ Now, Next []string; UpdatedAt time.Time }`、`SessionView.Status *StatusView` (無ければ nil、UpdatedAt はローカルタイム)

- [ ] **Step 1: 失敗するテストを書く**

`internal/index/index_test.go` の末尾に追記:

```go
func TestStatusCopiedToView(t *testing.T) {
	e := entry("a", "sa", "T", at(9))
	e.Status = &store.Status{Schema: 1, Now: []string{"進行中"}, Next: []string{"次"}, UpdatedAt: at(10)}
	ix := New()
	ix.Replace(map[string]store.SessionEntry{"a": e}, nil)
	v := ix.Sessions()[0]
	if v.Status == nil {
		t.Fatal("Status が写っていない")
	}
	if len(v.Status.Now) != 1 || v.Status.Now[0] != "進行中" || len(v.Status.Next) != 1 || v.Status.Next[0] != "次" {
		t.Errorf("Status = %+v", *v.Status)
	}
	if !v.Status.UpdatedAt.Equal(at(10)) {
		t.Errorf("UpdatedAt = %v, want %v", v.Status.UpdatedAt, at(10))
	}
	// 表示時刻は索引を作るここでローカルタイムに揃える (buildView の流儀)
	if v.Status.UpdatedAt.Location() != time.Local {
		t.Errorf("UpdatedAt がローカルタイムになっていない: %v", v.Status.UpdatedAt.Location())
	}
}

func TestStatusAbsentIsNil(t *testing.T) {
	ix := New()
	ix.Replace(map[string]store.SessionEntry{"a": entry("a", "sa", "T", at(9))}, nil)
	if v := ix.Sessions()[0]; v.Status != nil {
		t.Errorf("status が無いのに nil ではない: %+v", *v.Status)
	}
}

func TestSearchMatchesStatusText(t *testing.T) {
	a := entry("a", "sa", "x", at(9))
	a.Status = &store.Status{Schema: 1, Now: []string{"認証まわりを調査中"}, Next: []string{"トークン更新を直す"}, UpdatedAt: at(9)}
	ix := New()
	ix.Replace(map[string]store.SessionEntry{
		"a": a,
		"b": entry("b", "sb", "y", at(10)),
	}, nil)
	for _, q := range []string{"認証", "トークン更新"} {
		got := ix.Search(q)
		if len(got) != 1 || got[0].DirName != "a" {
			t.Errorf("Search(%q) = %v, want 1 件で a", q, got)
		}
	}
}
```

- [ ] **Step 2: 落ちることを確認する**

Run: `go test ./internal/index/ 2>&1 | head -20`
Expected: コンパイルエラー (`v.Status undefined`)

- [ ] **Step 3: 実装を書く**

`internal/index/index.go` の `SessionView` の直前に型を足し、`SessionView` に `Status` を足す:

```go
// StatusView はセッションの「現在の状況」(status.json 由来)。
type StatusView struct {
	Now       []string
	Next      []string
	UpdatedAt time.Time
}

// SessionView は一覧に出す 1 セッション分。
type SessionView struct {
	DirName   string
	Title     string
	Cwd       string
	GitBranch string
	PageCount int
	Bytes     int64
	LastSeen  time.Time
	Pages     []PageView
	Status    *StatusView // 無ければ nil
}
```

`buildView` の `for _, p := range e.Pages { ... }` の直後に追記:

```go
	// 状況。Scan が空のものを nil にしているが、テストなどで直接組んだ entry の
	// ために Empty も見る。時刻はここでローカルに揃える (LastSeen と同じ理由)。
	if s := e.Status; s != nil && !s.Empty() {
		v.Status = &StatusView{Now: s.Now, Next: s.Next, UpdatedAt: s.UpdatedAt.Local()}
	}
```

`buildHaystack` の `for _, p := range v.Pages { ... }` の直後 (`return` の前) に追記し、コメントも直す:

```go
	// 状況の文言でも引けるようにする。
	if v.Status != nil {
		for _, s := range v.Status.Now {
			b.WriteByte('\n')
			b.WriteString(s)
		}
		for _, s := range v.Status.Next {
			b.WriteByte('\n')
			b.WriteString(s)
		}
	}
```

`buildHaystack` のドキュメントコメントの「対象は…」の行を次にする:

```go
// 対象はタイトル・cwd・ブランチ・状況 (now / next) と、各ページのタイトル・要約・
// タグ・元プロンプト。
```

- [ ] **Step 4: 通ることを確認する**

Run: `gofmt -l . && go vet ./... && go test ./internal/index/ ./internal/store/`
Expected: gofmt の出力なし、PASS

- [ ] **Step 5: Commit**

```bash
git add internal/index/index.go internal/index/index_test.go
git commit -m "$(cat <<'EOF'
feat(index): セッションの状況を索引に載せ、検索対象にする

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>
EOF
)"
```

---

### Task 5: server — session ページに「現在の状況」「次やること」を出す

**Files:**
- Modify: `internal/server/render.go`
- Modify: `internal/web/session.html`
- Modify: `internal/web/static/style.css`
- Create: `internal/server/status_test.go`

**Interfaces:**
- Consumes: `index.SessionView.Status` (Task 4)、`at()` / `mkEntry()` / `get()` (`server_test.go`)
- Produces: `type statusRow struct{ Now, Next []string; UpdatedAt time.Time }`、`sessionRow.Status *statusRow`、テンプレートの文言「現在の状況」「次やること」「ページ」「状況はまだ書かれていません。」、`section.status` / `p.status-empty`

- [ ] **Step 1: 失敗するテストを書く**

`internal/server/status_test.go`:

```go
package server

import (
	"net/http"
	"strings"
	"testing"

	"github.com/pollenjp/cc-pages/internal/config"
	"github.com/pollenjp/cc-pages/internal/index"
	"github.com/pollenjp/cc-pages/internal/store"
)

// statusFixture は 1 セッション (0001-notion) に st を付けた索引のハンドラを返す。
// st が nil なら status 無し。
func statusFixture(t *testing.T, st *store.Status) http.Handler {
	t.Helper()
	e := mkEntry("20260829-aaaa", "sa", "Notion の調査", "0001-notion", 9)
	e.Status = st
	ix := index.New()
	ix.Replace(map[string]store.SessionEntry{"20260829-aaaa": e}, nil)
	return New(config.Config{Addr: "127.0.0.1:7777", Root: "/root"}, ix, func() {}).Handler()
}

func TestSessionPageShowsStatus(t *testing.T) {
	h := statusFixture(t, &store.Status{
		Schema: 1,
		Now:    []string{"設計を提示し承認待ち", "案 A を推奨"},
		Next:   []string{"spec を commit", "計画を書く"},
		UpdatedAt: at(10),
	})
	rec := get(t, h, "/p/20260829-aaaa/")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`<section class="status">`,
		"現在の状況", "次やること",
		"<ul><li>設計を提示し承認待ち</li><li>案 A を推奨</li></ul>",
		"<ol><li>spec を commit</li><li>計画を書く</li></ol>",
		"更新 " + at(10).Local().Format("2006-01-02 15:04"),
		"<h2>ページ</h2>",
		"/p/20260829-aaaa/0001-notion/",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("出力に %q が無い", want)
		}
	}
	// 並び: 現在の状況 → 次やること → ページ一覧
	i, j, k := strings.Index(body, "現在の状況"), strings.Index(body, "次やること"), strings.Index(body, "<h2>ページ</h2>")
	if !(i < j && j < k) {
		t.Errorf("並びが違う: 現在の状況=%d 次やること=%d ページ=%d", i, j, k)
	}
	if strings.Contains(body, "状況はまだ書かれていません") {
		t.Errorf("status があるのに空状態の文言が出ている")
	}
}

func TestSessionPageHidesEmptyNext(t *testing.T) {
	h := statusFixture(t, &store.Status{Schema: 1, Now: []string{"実装が終わり PR を出した"}, UpdatedAt: at(10)})
	body := get(t, h, "/p/20260829-aaaa/").Body.String()
	if !strings.Contains(body, "現在の状況") || !strings.Contains(body, "実装が終わり PR を出した") {
		t.Errorf("now が出ていない")
	}
	if strings.Contains(body, "次やること") {
		t.Errorf("next が空なのに見出しが出ている")
	}
}

func TestSessionPageWithoutStatusShowsPlaceholder(t *testing.T) {
	h := statusFixture(t, nil)
	body := get(t, h, "/p/20260829-aaaa/").Body.String()
	if !strings.Contains(body, `<p class="meta status-empty">状況はまだ書かれていません。</p>`) {
		t.Errorf("空状態の文言が無い")
	}
	for _, bad := range []string{`<section class="status">`, "現在の状況", "次やること"} {
		if strings.Contains(body, bad) {
			t.Errorf("status が無いのに %q が出ている", bad)
		}
	}
	// ページ一覧は今まで通り出る
	if !strings.Contains(body, "/p/20260829-aaaa/0001-notion/") {
		t.Errorf("ページ一覧が消えた")
	}
}

// TestSessionPageEscapesStatusItems は status の項目がフラグメントと違って
// 信頼された HTML として扱われないことを担保する。
func TestSessionPageEscapesStatusItems(t *testing.T) {
	h := statusFixture(t, &store.Status{Schema: 1, Now: []string{`<script>alert(1)</script>`}, UpdatedAt: at(10)})
	body := get(t, h, "/p/20260829-aaaa/").Body.String()
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Errorf("status の項目が生の HTML として出ている")
	}
	if !strings.Contains(body, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Errorf("エスケープされた項目が出ていない")
	}
}

func TestListPageDoesNotShowStatus(t *testing.T) {
	h := statusFixture(t, &store.Status{Schema: 1, Now: []string{"一覧には出ない文言"}, UpdatedAt: at(10)})
	body := get(t, h, "/").Body.String()
	for _, bad := range []string{"一覧には出ない文言", "現在の状況", "状況はまだ書かれていません"} {
		if strings.Contains(body, bad) {
			t.Errorf("/ に %q が出ている", bad)
		}
	}
}
```

- [ ] **Step 2: 落ちることを確認する**

Run: `go test ./internal/server/ -run 'TestSessionPage|TestListPageDoesNotShowStatus' 2>&1 | head -20`
Expected: FAIL (`出力に "<section class=\"status\">" が無い` など。`e.Status` は Task 3 で足したのでコンパイルは通る)

- [ ] **Step 3: render.go に statusRow を足す**

`internal/server/render.go` の `sessionRow` を次にし、直後に `statusRow` を足す:

```go
// sessionRow は一覧に出す 1 セッション。テンプレートに渡す形。
type sessionRow struct {
	DirName   string
	Title     string
	Cwd       string
	GitBranch string
	PageCount int
	Bytes     int64
	LastSeen  time.Time
	Pages     []pageRow
	Status    *statusRow // 無ければ nil。session.html が {{with}} で分岐する
}

// statusRow はセッションの「現在の状況」。テンプレートに渡す形。
//
// 項目は素の string のまま渡す。フラグメント (index.html) は template.HTML で
// 信頼するが、こちらは html/template に普通にエスケープさせる。
type statusRow struct {
	Now       []string
	Next      []string
	UpdatedAt time.Time
}
```

`toRow` の `for _, p := range v.Pages { ... }` の直後 (`return r` の前) に追記:

```go
	if v.Status != nil {
		r.Status = &statusRow{Now: v.Status.Now, Next: v.Status.Next, UpdatedAt: v.Status.UpdatedAt}
	}
```

- [ ] **Step 4: session.html を書き換える**

`internal/web/session.html` を丸ごと次にする:

```html
{{define "body"}}
<h1>{{.Session.Title}}</h1>
<div class="meta">
  <span>{{.Session.LastSeen.Format "2006-01-02 15:04"}}</span>
  <span>{{.Session.PageCount}} ページ</span>
  {{if .Session.GitBranch}}<span>{{.Session.GitBranch}}</span>{{end}}
  {{if .Session.Cwd}}<span>{{.Session.Cwd}}</span>{{end}}
</div>
{{/* 状況板。cc-pages status が書いた now / next。片方が空ならその枠は出さない。
     status 自体が無いときは 1 行だけ出す。何も出さないと「無い」のか skill が
     更新を忘れたのか分からない。 */}}
{{with .Session.Status}}
<section class="status">
  <div class="cols">
    {{if .Now}}<div><h2>現在の状況</h2><ul>{{range .Now}}<li>{{.}}</li>{{end}}</ul></div>{{end}}
    {{if .Next}}<div><h2>次やること</h2><ol>{{range .Next}}<li>{{.}}</li>{{end}}</ol></div>{{end}}
  </div>
  <div class="meta"><span>更新 {{.UpdatedAt.Format "2006-01-02 15:04"}}</span></div>
</section>
{{else}}
<p class="meta status-empty">状況はまだ書かれていません。</p>
{{end}}
<h2>ページ</h2>
<ol>
{{range .Session.Pages}}
  <li><a href="/p/{{$.Session.DirName}}/{{.DirName}}/">{{.Title}}</a>
    {{if .Summary}}<span class="meta">— {{.Summary}}</span>{{end}}
    <span class="meta">{{.CreatedAt.Format "15:04"}}</span></li>
{{end}}
</ol>
{{end}}
```

- [ ] **Step 5: style.css に状況板の見た目を足す**

`internal/web/static/style.css` の `.empty { ... }` の行の直後に追記:

```css
/* セッションの状況板 (session.html)。cc-pages status が書いた now / next を
   2 枠で出す。枠は .cards と同じ見た目に揃える。.cols なので片方だけなら
   横いっぱいに、狭い画面では縦に落ちる。 */
.status { margin: 1rem 0 1.5rem; }
.status .cols > div { border: 1px solid var(--line); border-radius: .5rem; padding: .7rem .9rem; background: var(--card); }
.status h2 { margin: 0 0 .4rem; font-size: 1rem; }
.status ul, .status ol { margin: 0; padding-left: 1.3rem; }
.status .meta { margin-top: .5rem; }
```

- [ ] **Step 6: 通ることを確認する**

Run: `gofmt -l . && go vet ./... && go test ./...`
Expected: gofmt の出力なし、全 package PASS。特に `TestDisplayedTimesShareOneZone` (status 無しで時刻が 2 つ) が通ること

- [ ] **Step 7: Commit**

```bash
git add internal/server/render.go internal/server/status_test.go internal/web/session.html internal/web/static/style.css
git commit -m "$(cat <<'EOF'
feat(web): session ページに「現在の状況」「次やること」を出す

h1 の直下に status.json 由来の 2 枠を置き、その下に既存のページ一覧を
続ける。status が無いセッションは 1 行の文言を出す。何も出さないと
「無い」のか skill が更新を忘れたのか分からないため。項目は
html/template に普通にエスケープさせ、フラグメントとは違って信頼しない。

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>
EOF
)"
```

---

### Task 6: CLI — `cc-pages status` サブコマンド

**Files:**
- Modify: `main.go`
- Modify: `main_test.go`

**Interfaces:**
- Consumes: `store.UpdateStatus` / `store.StatusInput` (Task 2)、`store.NotifyTouch`、`config.Config`
- Produces: `func cmdStatus(cfg config.Config, args []string, out io.Writer) error`、`type multiFlag []string` (旧 `tagList`)、`run` の `case "status"`

- [ ] **Step 1: 失敗するテストを書く**

`main_test.go` の import を次にし、末尾にテストを足す:

```go
import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pollenjp/cc-pages/internal/config"
	"github.com/pollenjp/cc-pages/internal/store"
)
```

```go
// statusCfg は status のテスト用の設定。Addr は誰も居ないポートにして、
// NotifyTouch が即座に諦めるようにする (100ms タイムアウト)。
func statusCfg(t *testing.T) config.Config {
	t.Helper()
	return config.Config{Addr: "127.0.0.1:1", Root: t.TempDir()}
}

func TestStatusWritesFileAndPrintsURL(t *testing.T) {
	cfg := statusCfg(t)
	var out bytes.Buffer
	err := cmdStatus(cfg, []string{
		"--session", "409bfd08-aaaa",
		"--now", "進行中",
		"--next", "次にやる", "--next", "その次",
	}, &out)
	if err != nil {
		t.Fatalf("cmdStatus() error = %v", err)
	}

	var res struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("stdout が JSON ではない: %q", out.String())
	}

	ents, err := os.ReadDir(filepath.Join(cfg.Root, "sessions"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 1 {
		t.Fatalf("セッションディレクトリ数 = %d, want 1", len(ents))
	}
	dir := ents[0].Name()
	if want := "http://localhost:1/p/" + dir + "/"; res.URL != want {
		t.Errorf("url = %q, want %q", res.URL, want)
	}
	s, err := store.ReadStatus(filepath.Join(cfg.Root, "sessions", dir))
	if err != nil {
		t.Fatalf("status.json が無い: %v", err)
	}
	if len(s.Now) != 1 || s.Now[0] != "進行中" {
		t.Errorf("Now = %q", s.Now)
	}
	if len(s.Next) != 2 || s.Next[0] != "次にやる" || s.Next[1] != "その次" {
		t.Errorf("Next = %q", s.Next)
	}
}

func TestStatusRejectsNoItems(t *testing.T) {
	cfg := statusCfg(t)
	var out bytes.Buffer
	if err := cmdStatus(cfg, []string{"--session", "409bfd08-aaaa"}, &out); err == nil {
		t.Fatal("now も next も無いのに通った")
	}
	if out.Len() != 0 {
		t.Errorf("失敗したのに stdout に出力がある: %q", out.String())
	}
}

func TestStatusRejectsBadSession(t *testing.T) {
	cfg := statusCfg(t)
	var out bytes.Buffer
	if err := cmdStatus(cfg, []string{"--session", "../x", "--now", "x"}, &out); err == nil {
		t.Fatal("不正な session id が通った")
	}
}

func TestRunListsStatusSubcommand(t *testing.T) {
	err := run(nil)
	if err == nil || !strings.Contains(err.Error(), "status") {
		t.Errorf("サブコマンド無しのエラーに status が載っていない: %v", err)
	}
}
```

- [ ] **Step 2: 落ちることを確認する**

Run: `go test . 2>&1 | head -20`
Expected: コンパイルエラー (`undefined: cmdStatus`)

- [ ] **Step 3: main.go を直す**

import に `"io"` を足す。

`run` のサブコマンド無しのエラーと `switch` を次にする:

```go
	if len(args) == 0 {
		return fmt.Errorf("サブコマンドが要る (new / serve / status)")
	}
```

```go
	switch args[0] {
	case "new":
		return cmdNew(cfg, args[1:])
	case "serve":
		return cmdServe(cfg, args[1:])
	case "status":
		return cmdStatus(cfg, args[1:], os.Stdout)
	default:
		return fmt.Errorf("不明なサブコマンド: %s", args[0])
	}
```

`tagList` を `multiFlag` に改名する (`--now` / `--next` にも使うので、名前を用途から外す):

```go
// multiFlag は繰り返し指定できる文字列のフラグ (--tag / --now / --next)。
type multiFlag []string

func (m *multiFlag) String() string     { return fmt.Sprint([]string(*m)) }
func (m *multiFlag) Set(v string) error { *m = append(*m, v); return nil }
```

`cmdNew` の `tags tagList` を `tags multiFlag` にする。

`cmdNew` の直後に `cmdStatus` を足す:

```go
// cmdStatus はセッションの「現在の状況」を全置換で書き、{"url"} を out に出す。
//
// out を引数に取るのはテストのため (cmdNew は os.Stdout に直接書いている)。
// 検証エラーのときは out に何も書かない。skill は stdout を JSON として読むので、
// 失敗時に半端な出力を残さない。
func cmdStatus(cfg config.Config, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	var (
		session = fs.String("session", "", "セッション ID (CLAUDE_CODE_SESSION_ID)")
		now     multiFlag
		next    multiFlag
	)
	fs.Var(&now, "now", "現在の状況 (繰り返し指定できる)")
	fs.Var(&next, "next", "次やること (繰り返し指定できる)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	res, err := store.UpdateStatus(cfg.Root, cfg.BaseURL(), time.Now(), store.StatusInput{
		SessionID: *session,
		Now:       now,
		Next:      next,
	})
	if err != nil {
		return err
	}
	store.NotifyTouch(cfg.BaseURL())
	return json.NewEncoder(out).Encode(res)
}
```

- [ ] **Step 4: 通ることを確認する**

Run: `gofmt -l . && go vet ./... && go test ./...`
Expected: gofmt の出力なし、全 package PASS

- [ ] **Step 5: 実物で 1 往復見る**

```bash
go build -o cc-pages . && \
export CC_PAGES_ROOT=/tmp/cc-pages-task6 CC_PAGES_ADDR=127.0.0.1:7799 && \
./cc-pages new --session demo-aaaa --title "確認用" --summary "1 行" && \
./cc-pages status --session demo-aaaa --now "実装中" --next "e2e を書く" && \
cat /tmp/cc-pages-task6/sessions/*/status.json && \
(./cc-pages serve & echo $! > /tmp/cc-pages-task6.pid; sleep 1; \
 URL=$(./cc-pages status --session demo-aaaa --now "実装中" | sed 's/.*"url":"\([^"]*\)".*/\1/'); \
 curl -s "$URL" | grep -c '現在の状況'; \
 curl -s "$URL" | grep -o '<ol><li>[^<]*</li></ol>'); \
kill "$(cat /tmp/cc-pages-task6.pid)"; unset CC_PAGES_ROOT CC_PAGES_ADDR; rm -rf /tmp/cc-pages-task6 /tmp/cc-pages-task6.pid
```

Expected: `status` の出力は `{"url":"http://localhost:7799/p/<日付>-demo-aaa/"}`、`status.json` に now / next / updated_at、`grep -c` が `1`。2 回目の `status` は `--next` 無しなので `<ol>` の grep は何も出ない (次やることの枠が消えている)

- [ ] **Step 6: Commit**

```bash
git add main.go main_test.go
git commit -m "$(cat <<'EOF'
feat(cli): cc-pages status サブコマンドを足す

--now / --next を繰り返し受け、store.UpdateStatus で status.json を
全置換する。new と同じく POST /_/touch を fire-and-forget で送り、
stdout にはセッションのページ一覧 (状況板) の URL を JSON で出す。

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>
EOF
)"
```

---

### Task 7: e2e — 状況板を実ブラウザで検査し、スクショを撮る

**Files:**
- Create: `tests/e2e/fixtures/root/sessions/20260101-e2efixture/status.json`
- Create: `tests/e2e/session.spec.ts`

**Interfaces:**
- Consumes: `section.status` / `h2` 見出し「現在の状況」「次やること」「ページ」(Task 5)、fixture のセッション `20260101-e2efixture`
- Produces: スクショ `08-session-status-light.png` / `09-session-status-dark.png` (CI が PR の sticky コメントに貼る)

- [ ] **Step 1: fixture の status.json を置く**

`tests/e2e/fixtures/root/sessions/20260101-e2efixture/status.json`:

```json
{
  "schema": 1,
  "now": [
    "diff の着色・本文幅トグル・画像の拡大を e2e で検査済み",
    "セッションの状況板を実装中"
  ],
  "next": [
    "status.json の fixture でスクショを撮る",
    "PR の sticky コメントで見た目を確認する"
  ],
  "updated_at": "2026-01-01T09:30:00+09:00"
}
```

- [ ] **Step 2: 失敗する spec を書く**

`tests/e2e/session.spec.ts`:

```ts
import { test, expect, type Page } from '@playwright/test';
import * as path from 'node:path';

// セッションの状況板 (/p/<session>/ の「現在の状況」「次やること」) の e2e。
//
// 撮っていないもの (撮れないものではなく、意図的に範囲外):
// - status 無しのセッション。fixture は 1 セッションしか持たず、空状態の文言は
//   Go 側の TestSessionPageWithoutStatusShowsPlaceholder が字面で見ている
// - / の一覧。状況は出さない設計で、Go 側の TestListPageDoesNotShowStatus が見ている

const SESSION_URL = '/p/20260101-e2efixture/';
const SHOTS = path.join(__dirname, 'screenshots');

const status = (page: Page) => page.locator('section.status');

async function openSession(page: Page) {
  await page.goto(SESSION_URL);
  // 真っ白でも通る「撮るだけの test」にしない。状況板そのものを見る
  await expect(status(page)).toBeVisible();
}

/** 状況板の中身が fixture の status.json と一致することを見る。 */
async function expectStatus(page: Page) {
  await expect(status(page).getByRole('heading', { name: '現在の状況' })).toBeVisible();
  await expect(status(page).getByRole('heading', { name: '次やること' })).toBeVisible();
  // now は ul、next は ol (順序に意味がある)
  await expect(status(page).locator('ul li')).toHaveText([
    'diff の着色・本文幅トグル・画像の拡大を e2e で検査済み',
    'セッションの状況板を実装中',
  ]);
  await expect(status(page).locator('ol li')).toHaveText([
    'status.json の fixture でスクショを撮る',
    'PR の sticky コメントで見た目を確認する',
  ]);

  // 状況板はページ一覧より上にある
  const statusTop = await status(page).evaluate((el) => el.getBoundingClientRect().top);
  const listTop = await page
    .getByRole('heading', { name: 'ページ', exact: true })
    .evaluate((el) => el.getBoundingClientRect().top);
  expect(statusTop).toBeLessThan(listTop);
}

test.describe('light', () => {
  test.use({ colorScheme: 'light' });

  test('セッションの状況板 (light)', async ({ page }) => {
    await openSession(page);
    await expectStatus(page);
    await page.screenshot({ path: path.join(SHOTS, '08-session-status-light.png'), fullPage: true });
  });
});

test.describe('dark', () => {
  test.use({ colorScheme: 'dark' });

  test('セッションの状況板 (dark)', async ({ page }) => {
    await openSession(page);
    await expectStatus(page);
    await page.screenshot({ path: path.join(SHOTS, '09-session-status-dark.png'), fullPage: true });
  });
});
```

- [ ] **Step 3: 依存を入れて走らせる**

`tests/e2e/node_modules` が無いので lockfile 通りに入れる (依存の追加・更新ではない):

```bash
cd tests/e2e && nix develop ../.. --command npm ci
```

Run: `cd tests/e2e && nix develop ../.. --command npx playwright test session.spec.ts`
Expected: 2 tests passed。`tests/e2e/screenshots/08-session-status-light.png` と `09-session-status-dark.png` ができる。
webServer が `go run . serve` を fixture root で起こすので、`cc-pages serve` を手で起こす必要は無い (7788 が空いていること)。

もし `CHROMIUM_BIN が空です` で止まったら `nix develop` の外で叩いている。

- [ ] **Step 4: スクショを目で見る**

`08-session-status-light.png` と `09-session-status-dark.png` を Read で開き、次を確認する:

- 2 枠が h1 の下・ページ一覧の上に横並びで出ている
- 枠の色が `.cards` と同じ (ダークでは暗いカード色)
- 日本語が豆腐 (□) になっていない (なっていれば devShell の外)

- [ ] **Step 5: 既存の e2e も全部通す**

Run: `cd tests/e2e && nix develop ../.. --command npx playwright test`
Expected: 全 spec passed (fixture に status.json を足しても既存のページの spec は影響を受けない)

- [ ] **Step 6: Commit**

```bash
git add tests/e2e/fixtures/root/sessions/20260101-e2efixture/status.json tests/e2e/session.spec.ts
git commit -m "$(cat <<'EOF'
test(e2e): セッションの状況板を実ブラウザで検査し、スクショを撮る

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>
EOF
)"
```

---

### Task 8: README と手元のバイナリの差し替え

**Files:**
- Modify: `README.md`
- (repo 外) `~/ghq/github.com/pollenjp/cc-pages/cc-pages` = `~/bin/cc-pages` の symlink 先、`cc-pages.service` (systemd user)

**Interfaces:**
- Consumes: `cc-pages status` (Task 6)
- Produces: `~/bin/cc-pages` が `status` を持つ状態 (Task 9 の skill が前提にする)

- [ ] **Step 1: README の使い方に status を足す**

`README.md` の「`new` が返す `dir` に `index.html`（…）を書き、`url` をブラウザで開く。」の段落の直後に追記:

```markdown
    cc-pages status --session "$CLAUDE_CODE_SESSION_ID" \
      --now "調査が終わり案 A で合意" --next "spec を commit する"

`status` はセッション直下の `status.json` を丸ごと書き換え（全置換）、セッションの
ページ一覧 `/p/<session>/` の上に「現在の状況」「次やること」として出す。`--now` と
`--next` は繰り返し指定でき、どちらか 1 つは要る。ページより先に打ってもよい。
`status.json` が無いセッションは「状況はまだ書かれていません。」と出る。
```

- [ ] **Step 2: 設計書の所在を README に足す**

README 末尾の「設計は `claude-skills`（private）の …にある。」の段落の直後に追記:

```markdown
状況板 (`cc-pages status`) の設計はこの repo の
`docs/superpowers/specs/2026-09-22-session-status-design.md` にある。
```

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "$(cat <<'EOF'
docs(readme): cc-pages status と状況板を書く

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>
EOF
)"
```

- [ ] **Step 4: 手元のバイナリを差し替えて serve を再起動する**

`~/bin/cc-pages` は `~/ghq/github.com/pollenjp/cc-pages/cc-pages` への symlink で、`cc-pages.service` (systemd user) がそれを動かしている。symlink 先に build して unit を再起動する:

```bash
go build -o /home/pollenjp/ghq/github.com/pollenjp/cc-pages/cc-pages . && \
systemctl --user restart cc-pages.service && sleep 1 && \
systemctl --user is-active cc-pages.service && \
cc-pages status --session "$CLAUDE_CODE_SESSION_ID" \
  --now "cc-pages 側の実装が終わり、手元のバイナリを差し替えた" \
  --next "skill (pjp-cc-page) に手順 3 を足す" \
  --next "cc-pages と claude-skills の PR を出す"
```

Expected: `active`、`{"url":"http://localhost:7777/p/20260922-a18223c0/"}`。その URL を `curl -s` して `現在の状況` が出ること。

- [ ] **Step 5: 実データで目で見る**

`nix develop --command chromium --headless=new --disable-gpu --no-sandbox --hide-scrollbars --window-size=1100,1400 --screenshot=/tmp/claude-1000/-home-pollenjp--herdr-worktrees-cc-pages-worktree-green-meadow-776b/a18223c0-8869-444d-99e0-3b159dbf52cc/scratchpad/session-real.png "http://localhost:7777/p/20260922-a18223c0/"` で撮って Read し、2 枠とページ一覧 (0001, 0002) が出ていることを見る。

---

### Task 9: skill — pjp-cc-page に手順 3「セッションの状況を更新する」を足す

**Files:**
- Modify: `~/ghq/github.com/pollenjp/claude-skills/skills/pjp-cc-page/SKILL.md` (symlink `~/.claude/skills/pjp-cc-page` の実体。Nix 管理ではないので直接編集してよいが commit が要る)

**Interfaces:**
- Consumes: `cc-pages status` (Task 6、Task 8 で `~/bin` に入っていること)
- Produces: 手順 3 の文言 (skill の実行時に CC が読む)

- [ ] **Step 1: branch を切る**

```bash
cd ~/ghq/github.com/pollenjp/claude-skills && git status --short && git switch -c feat/cc-page-session-status
```

Expected: `git status --short` は空 (clean)。作業後は元の branch に戻さなくてよい (symlink は branch に関係なく同じ path を指す) が、他の作業を始める前に `main` へ戻すこと。

- [ ] **Step 2: 手順 1 の summary を必須にする**

`SKILL.md` の「### 1. ページを作る」の `{"id":"0007","dir":"...","url":"..."}` が返る。` の行の直後に追記:

```markdown
**`--summary` は省略しない。** 一覧とセッションの top ページに出る 1 行で、そのページが
何を扱うかを書く。top ページでは「各ページの説明」としてこれだけが読まれる。
```

- [ ] **Step 3: 手順 3 を挿し、旧 3 を 4 に繰り下げる**

`### 3. stdout に短い箇条書きとリンクを返す` を `### 4. stdout に短い箇条書きとリンクを返す` にし、その見出しの直前に次を挿す:

````markdown
### 3. セッションの状況を更新する

ページを書き終えたら、**毎回**、セッション全体の状況を言い直す。セッションの top ページ
(`url` の 1 つ上 = `/p/<session>/`) がこれを「現在の状況」「次やること」として出す。

```sh
cc-pages status --session "$CLAUDE_CODE_SESSION_ID" \
  --now "callout の調査が終わり、案 A で進める方向で確認待ち" \
  --now "実装計画はまだ書いていない" \
  --next "ユーザーの返事を待って spec を commit する" \
  --next "writing-plans で実装計画を書く"
```

`{"url":"http://localhost:7777/p/20260829-409bfd08/"}` が返る。この URL は stdout に
出さない (手順 4 で返すのはページの URL)。

- **セッション全体の状況を書く。** 今作ったページの目次ではない。「何が決まり、何が
  残っていて、次に何をするか」を、ページを開かずに分かる粒度で書く
- `--now` は 1〜3 項目、`--next` は 0〜5 項目。各項目は 1 文で、`。` は付けない。
  やることが無ければ `--next` は省いてよい
- **全置換。** 前回の内容は引き継がれない。まだ有効な項目も含めて全部書き直す
- ページを作らない往復でも、状況が動いたら打ってよい (方針転換・作業の完了)
- 失敗しても回答は止めない。`new` と同じ退行規則で、`command -v cc-pages` が空なら
  手順 1 と同じく何もしない

````

- [ ] **Step 4: 手順 4 の中の「3」への参照を確認する**

`grep -n '手順 3\|### 3\|### 4' SKILL.md` で、手順番号の参照が新しい番号と合っていることを見る。旧「3」を指す文は無いはずだが、あれば「4」に直す。

- [ ] **Step 5: lint と実機確認**

```bash
cd ~/ghq/github.com/pollenjp/claude-skills && ./scripts/lint.sh
```

Expected: lint が通る (`scripts/lint.sh` は flake の devShell に自分で入り直す。初回は nix の取得で時間が掛かる)。

次に、このセッションで pjp-cc-page を 1 回使い (短いページでよい)、手順 3 が実行されて `http://localhost:7777/p/20260922-a18223c0/` の状況が更新されることを見る。

- [ ] **Step 6: Commit**

```bash
cd ~/ghq/github.com/pollenjp/claude-skills && git add skills/pjp-cc-page/SKILL.md && git commit -m "$(cat <<'EOF'
feat(pjp-cc-page): ページを書き終えるたびにセッションの状況を更新する

cc-pages status で「現在の状況」「次やること」を全置換で書く手順 3 を足し、
stdout の手順を 4 に繰り下げる。--summary は top ページの「各ページの説明」
になるので省略させない。

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>
EOF
)"
```

---

## 仕上げ

全 task の後に、この worktree で:

```bash
gofmt -l . && go vet ./... && go test ./... && \
cd tests/e2e && nix develop ../.. --command npx playwright test; cd ../..
git log --oneline main..HEAD
```

Expected: gofmt の出力なし、Go の全テストと e2e が通り、log に Task 1〜8 の commit (spec の commit も含む) が並ぶ。

そのうえで superpowers:finishing-a-development-branch に進む。PR は cc-pages (この branch) と claude-skills (`feat/cc-page-session-status`) の 2 本。
