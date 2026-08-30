package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// seed は sessions ディレクトリに 1 ページ作って sessionsDir を返す。
func seed(t *testing.T, root, sessionID, title string, now time.Time) NewPageResult {
	t.Helper()
	res, err := CreatePage(root, "http://localhost:7777", now, NewPageInput{
		SessionID: sessionID, Title: title,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(res.Dir, "index.html"), []byte("<p>x</p>"), 0o600); err != nil {
		t.Fatal(err)
	}
	return res
}

func TestScanEmpty(t *testing.T) {
	got, err := Scan(filepath.Join(t.TempDir(), "sessions"), nil)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

func TestScanFindsPagesInOrder(t *testing.T) {
	root := t.TempDir()
	seed(t, root, "sess-aaaa", "1 つ目", fixedTime())
	seed(t, root, "sess-aaaa", "2 つ目", fixedTime())
	got, err := Scan(filepath.Join(root, "sessions"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("セッション数 = %d, want 1", len(got))
	}
	for _, e := range got {
		if len(e.Pages) != 2 {
			t.Fatalf("ページ数 = %d, want 2", len(e.Pages))
		}
		if e.Pages[0].Page.ID != "0001" || e.Pages[1].Page.ID != "0002" {
			t.Errorf("連番順に並んでいない: %q %q", e.Pages[0].Page.ID, e.Pages[1].Page.ID)
		}
		if e.Session.SessionID != "sess-aaaa" {
			t.Errorf("SessionID = %q", e.Session.SessionID)
		}
		if e.Bytes <= 0 {
			t.Errorf("Bytes = %d, want > 0", e.Bytes)
		}
	}
}

func TestScanSkipsBrokenPage(t *testing.T) {
	root := t.TempDir()
	res := seed(t, root, "sess-aaaa", "良いページ", fixedTime())
	broken := filepath.Join(filepath.Dir(res.Dir), "0002-broken")
	if err := os.MkdirAll(broken, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(broken, "page.json"), []byte("{ではない"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Scan(filepath.Join(root, "sessions"), nil)
	if err != nil {
		t.Fatalf("壊れた page.json で Scan がエラーになった: %v", err)
	}
	for _, e := range got {
		if len(e.Pages) != 1 {
			t.Errorf("ページ数 = %d, want 1 (壊れた方は飛ばす)", len(e.Pages))
		}
	}
}

func TestScanReusesUnchangedSession(t *testing.T) {
	root := t.TempDir()
	seed(t, root, "sess-aaaa", "T", fixedTime())
	sessionsDir := filepath.Join(root, "sessions")

	first, err := Scan(sessionsDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	// prev のページを差し替えておく。再利用されたなら差し替えたものが返る。
	for k, e := range first {
		e.Pages[0].Page.Title = "差し替え済み"
		first[k] = e
	}
	second, err := Scan(sessionsDir, first)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range second {
		if e.Pages[0].Page.Title != "差し替え済み" {
			t.Errorf("mtime が変わっていないのに再読み込みした")
		}
	}
}

func TestScanRereadsChangedSession(t *testing.T) {
	root := t.TempDir()
	seed(t, root, "sess-aaaa", "T", fixedTime())
	sessionsDir := filepath.Join(root, "sessions")

	first, err := Scan(sessionsDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	for k, e := range first {
		e.Pages[0].Page.Title = "差し替え済み"
		first[k] = e
	}
	// ページを足すとセッションディレクトリの mtime が動く。
	seed(t, root, "sess-aaaa", "2 つ目", fixedTime())

	second, err := Scan(sessionsDir, first)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range second {
		if len(e.Pages) != 2 {
			t.Fatalf("ページ数 = %d, want 2", len(e.Pages))
		}
		if e.Pages[0].Page.Title == "差し替え済み" {
			t.Errorf("mtime が変わったのに再読み込みしていない")
		}
	}
}

func TestScanDropsRemovedSession(t *testing.T) {
	root := t.TempDir()
	res := seed(t, root, "sess-aaaa", "T", fixedTime())
	sessionsDir := filepath.Join(root, "sessions")
	first, err := Scan(sessionsDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Dir(res.Dir)); err != nil {
		t.Fatal(err)
	}
	second, err := Scan(sessionsDir, first)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 0 {
		t.Errorf("消したセッションが残っている: %d", len(second))
	}
}
