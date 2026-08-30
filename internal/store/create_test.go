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
