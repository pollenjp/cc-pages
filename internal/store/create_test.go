package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func fixedTime() time.Time {
	return time.Date(2026, 8, 29, 22, 30, 0, 0, time.FixedZone("JST", 9*3600))
}

func TestCreatePageFirstPage(t *testing.T) {
	root := t.TempDir()
	in := NewPageInput{
		SessionID: "409bfd08-ade3-44d0-a3f4-56369f828d21",
		Title:     "Notion callout の調査",
		Summary:   "一覧に出る 1 行",
		Tags:      []string{"調査"},
		Prompt:    "callout について調べて",
	}
	got, err := CreatePage(root, "http://localhost:7777", fixedTime(), in)
	if err != nil {
		t.Fatalf("CreatePage() error = %v", err)
	}
	if got.ID != "0001" {
		t.Errorf("ID = %q, want 0001", got.ID)
	}
	wantDir := filepath.Join(root, "sessions", "20260829-409bfd08", "0001-notion-callout-の調査")
	if got.Dir != wantDir {
		t.Errorf("Dir = %q, want %q", got.Dir, wantDir)
	}
	wantURL := "http://localhost:7777/p/20260829-409bfd08/0001-notion-callout-の調査"
	if got.URL != wantURL {
		t.Errorf("URL = %q, want %q", got.URL, wantURL)
	}
	if _, err := os.Stat(got.Dir); err != nil {
		t.Errorf("ページディレクトリが無い: %v", err)
	}
	// index.html は skill が書く。new は書かない。
	if _, err := os.Stat(filepath.Join(got.Dir, "index.html")); !os.IsNotExist(err) {
		t.Errorf("new が index.html を作ってしまっている")
	}
}

func TestCreatePageWritesPageJSON(t *testing.T) {
	root := t.TempDir()
	in := NewPageInput{SessionID: "409bfd08-aaaa", Title: "T", Tags: []string{"a", "b"}}
	got, err := CreatePage(root, "http://localhost:7777", fixedTime(), in)
	if err != nil {
		t.Fatalf("CreatePage() error = %v", err)
	}
	p, err := ReadPage(got.Dir)
	if err != nil {
		t.Fatalf("ReadPage() error = %v", err)
	}
	if p.Schema != SchemaVersion {
		t.Errorf("Schema = %d, want %d", p.Schema, SchemaVersion)
	}
	if p.ID != "0001" || p.Title != "T" || p.Mode != ModeFragment {
		t.Errorf("Page = %+v", p)
	}
	if len(p.Tags) != 2 || p.Tags[0] != "a" {
		t.Errorf("Tags = %v", p.Tags)
	}
	if !p.CreatedAt.Equal(fixedTime()) {
		t.Errorf("CreatedAt = %v, want %v", p.CreatedAt, fixedTime())
	}
	if p.SessionID != "409bfd08-aaaa" {
		t.Errorf("SessionID = %q", p.SessionID)
	}
}

func TestCreatePageIncrementsID(t *testing.T) {
	root := t.TempDir()
	in := NewPageInput{SessionID: "409bfd08-aaaa", Title: "T"}
	for want := 1; want <= 3; want++ {
		got, err := CreatePage(root, "http://localhost:7777", fixedTime(), in)
		if err != nil {
			t.Fatalf("CreatePage() error = %v", err)
		}
		if got.ID != FormatID(want) {
			t.Errorf("ID = %q, want %q", got.ID, FormatID(want))
		}
	}
}

func TestCreatePageReusesSessionDirAcrossDays(t *testing.T) {
	root := t.TempDir()
	in := NewPageInput{SessionID: "409bfd08-aaaa", Title: "T"}
	first, err := CreatePage(root, "http://localhost:7777", fixedTime(), in)
	if err != nil {
		t.Fatal(err)
	}
	nextDay := fixedTime().Add(24 * time.Hour)
	second, err := CreatePage(root, "http://localhost:7777", nextDay, in)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(first.Dir) != filepath.Dir(second.Dir) {
		t.Errorf("日付をまたいだらセッションディレクトリが割れた: %q vs %q",
			filepath.Dir(first.Dir), filepath.Dir(second.Dir))
	}
	if second.ID != "0002" {
		t.Errorf("ID = %q, want 0002", second.ID)
	}
}

func TestCreatePageDistinguishesCollidingPrefixes(t *testing.T) {
	root := t.TempDir()
	a := NewPageInput{SessionID: "409bfd08-aaaa", Title: "A"}
	b := NewPageInput{SessionID: "409bfd08-bbbb", Title: "B"}
	ra, err := CreatePage(root, "http://localhost:7777", fixedTime(), a)
	if err != nil {
		t.Fatal(err)
	}
	rb, err := CreatePage(root, "http://localhost:7777", fixedTime(), b)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(ra.Dir) == filepath.Dir(rb.Dir) {
		t.Fatalf("先頭 8 桁が同じ別セッションが同じディレクトリに入った: %q", ra.Dir)
	}
	if rb.ID != "0001" {
		t.Errorf("別セッションなのに採番が継続した: ID = %q", rb.ID)
	}
}

func TestCreatePageWritesSessionSkeleton(t *testing.T) {
	root := t.TempDir()
	in := NewPageInput{SessionID: "409bfd08-aaaa", Title: "T"}
	got, err := CreatePage(root, "http://localhost:7777", fixedTime(), in)
	if err != nil {
		t.Fatal(err)
	}
	s, err := ReadSession(filepath.Dir(got.Dir))
	if err != nil {
		t.Fatalf("ReadSession() error = %v", err)
	}
	if s.SessionID != "409bfd08-aaaa" {
		t.Errorf("SessionID = %q", s.SessionID)
	}
	if s.Dir != "20260829-409bfd08" {
		t.Errorf("Dir = %q", s.Dir)
	}
	if !s.LastSeen.Equal(fixedTime()) {
		t.Errorf("LastSeen = %v", s.LastSeen)
	}
}

func TestCreatePageRejectsEmptySession(t *testing.T) {
	root := t.TempDir()
	_, err := CreatePage(root, "http://localhost:7777", fixedTime(), NewPageInput{Title: "T"})
	if err == nil {
		t.Fatal("session が空でもエラーにならなかった")
	}
}

func TestCreatePageRejectsBadMode(t *testing.T) {
	root := t.TempDir()
	in := NewPageInput{SessionID: "s", Title: "T", Mode: "bogus"}
	if _, err := CreatePage(root, "http://localhost:7777", fixedTime(), in); err == nil {
		t.Fatal("不正な mode が通ってしまった")
	}
}

func TestPermissions(t *testing.T) {
	root := t.TempDir()
	in := NewPageInput{SessionID: "409bfd08-aaaa", Title: "T"}
	got, err := CreatePage(root, "http://localhost:7777", fixedTime(), in)
	if err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(filepath.Join(root, "sessions"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o700 {
		t.Errorf("sessions のパーミッション = %o, want 700", perm)
	}
	fj, err := os.Stat(filepath.Join(got.Dir, "page.json"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := fj.Mode().Perm(); perm != 0o600 {
		t.Errorf("page.json のパーミッション = %o, want 600", perm)
	}
}

func TestNewPageResultIsValidJSON(t *testing.T) {
	root := t.TempDir()
	in := NewPageInput{SessionID: "409bfd08-aaaa", Title: "T"}
	got, err := CreatePage(root, "http://localhost:7777", fixedTime(), in)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var back map[string]string
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"id", "dir", "url"} {
		if back[k] == "" {
			t.Errorf("キー %q が空", k)
		}
	}
}

func TestCreatePageRejectsUnsafeSession(t *testing.T) {
	bad := []string{"../../../x", "a/b", "a#b", "a?b", "a b"}
	for _, sid := range bad {
		root := t.TempDir()
		in := NewPageInput{SessionID: sid, Title: "T"}
		if _, err := CreatePage(root, "http://localhost:7777", fixedTime(), in); err == nil {
			t.Errorf("session id %q が通ってしまった", sid)
		}
		ents, err := os.ReadDir(root)
		if err != nil {
			t.Fatalf("ReadDir(root) error = %v", err)
		}
		if len(ents) != 0 {
			t.Errorf("session id %q は拒否されるべきなのに root にディレクトリが作られた: %v", sid, ents)
		}
	}

	// 正常系の対照: 実際の UUID 形状 (文字・数字・"-" のみ) は拒否されない。
	root := t.TempDir()
	in := NewPageInput{SessionID: "409bfd08-ade3-44d0-a3f4-56369f828d21", Title: "T"}
	if _, err := CreatePage(root, "http://localhost:7777", fixedTime(), in); err != nil {
		t.Errorf("UUID 形状の session id が拒否された: %v", err)
	}
}

// mkSessionDir は sessions/<name>/session.json を session id sid で書く。
func mkSessionDir(t *testing.T, sessionsDir, name, sid string) {
	t.Helper()
	dir := filepath.Join(sessionsDir, name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := WriteSession(dir, Session{Schema: SchemaVersion, SessionID: sid, Dir: name}); err != nil {
		t.Fatal(err)
	}
}

func TestSessionDirMatches(t *testing.T) {
	const sid = "0a1b2c3d-e5f6-4711-8000-000000000000"
	cases := []struct {
		name string
		want bool
	}{
		{"20261111-0a1b2c3d", true},
		{"20261111-0a1b2c3d-e5f", true}, // 衝突して 12 字に伸ばした名前
		{"20261111-zzzz", false},
		{"nodash", false},
		{"20261111-", false},
	}
	for _, c := range cases {
		if got := sessionDirMatches(c.name, sid); got != c.want {
			t.Errorf("sessionDirMatches(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestFindOrMakeSessionDirReadsOnlyMatchingNames は、会話のディレクトリを引くときに
// 名前の合わないディレクトリの session.json を読まないことを確かめる。
//
// cc-pages link は turn の終わりごとに打たれる。全部の会話の session.json を読むと、
// 会話が増えるほど遅くなる。名前は「日付-session id の先頭」なので、名前で絞れる。
// 名前の合わないディレクトリが同じ session id を名乗っていても使わない (手で
// 名前を変えたときにしか起きない)。
func TestFindOrMakeSessionDirReadsOnlyMatchingNames(t *testing.T) {
	root := t.TempDir()
	sessionsDir := filepath.Join(root, "sessions")
	mkSessionDir(t, sessionsDir, "20260101-zzzzzzzz", "409bfd08-aaaa")

	res, err := CreatePage(root, "http://localhost:7777", fixedTime(), NewPageInput{SessionID: "409bfd08-aaaa", Title: "T"})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(sessionsDir, "20260829-409bfd08", "0001-t"); res.Dir != want {
		t.Errorf("Dir = %q, want %q (名前の合わない dir を使った)", res.Dir, want)
	}
}

func TestFindSessionDir(t *testing.T) {
	sessionsDir := filepath.Join(t.TempDir(), "sessions")
	if name, ok := FindSessionDir(sessionsDir, "409bfd08-aaaa"); ok {
		t.Errorf("sessions が無いのに見つかった: %q", name)
	}

	// 先頭 8 字が同じ別の会話が先に居たので、12 字に伸ばした名前で作られた会話
	mkSessionDir(t, sessionsDir, "20260829-409bfd08", "409bfd08-bbbb")
	mkSessionDir(t, sessionsDir, "20260829-409bfd08-aaa", "409bfd08-aaaa")

	if name, ok := FindSessionDir(sessionsDir, "409bfd08-aaaa"); !ok || name != "20260829-409bfd08-aaa" {
		t.Errorf("FindSessionDir(aaaa) = %q, %v, want 20260829-409bfd08-aaa", name, ok)
	}
	if name, ok := FindSessionDir(sessionsDir, "409bfd08-bbbb"); !ok || name != "20260829-409bfd08" {
		t.Errorf("FindSessionDir(bbbb) = %q, %v, want 20260829-409bfd08", name, ok)
	}
	// 名前の先頭は合うが session.json の session id が違う
	if name, ok := FindSessionDir(sessionsDir, "409bfd08-cccc"); ok {
		t.Errorf("別の会話のディレクトリを返した: %q", name)
	}
	// 探すだけで、作らない
	ents, err := os.ReadDir(sessionsDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 2 {
		t.Errorf("FindSessionDir がディレクトリを作った: %d 個", len(ents))
	}
}

// TestWriteSessionKeepsOldContentForOpenReaders は、session.json を書き直している間に
// 読んだ側が、前の中身を丸ごと読めることを確かめる。
//
// cc-pages link は turn ごとに session.json を書く。切り詰めてから書くと、その隙に
// 読んだ findOrMakeSessionDir が空の JSON を見て「知らない会話」と取り違え、
// 会話のディレクトリを 2 つに割ってしまう。一時ファイルから rename で入れ替えれば、
// 開いていた読み手は前のファイルを読み続ける。
func TestWriteSessionKeepsOldContentForOpenReaders(t *testing.T) {
	dir := t.TempDir()
	if err := WriteSession(dir, Session{Schema: SchemaVersion, SessionID: "old", Dir: "d"}); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filepath.Join(dir, SessionFileName))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if err := WriteSession(dir, Session{Schema: SchemaVersion, SessionID: "new", Dir: "d"}); err != nil {
		t.Fatal(err)
	}

	var s Session
	if err := json.NewDecoder(f).Decode(&s); err != nil {
		t.Fatalf("開いていた session.json が読めない (切り詰められた): %v", err)
	}
	if s.SessionID != "old" {
		t.Errorf("開いていた読み手が見た session_id = %q, want old", s.SessionID)
	}
	if got, err := ReadSession(dir); err != nil || got.SessionID != "new" {
		t.Errorf("書いた後の ReadSession = %+v, %v, want new", got, err)
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 1 {
		t.Errorf("session.json 以外のファイルが残った: %d 個", len(ents))
	}
	fi, err := os.Stat(filepath.Join(dir, SessionFileName))
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("session.json のパーミッション = %o, want 600", perm)
	}
}
