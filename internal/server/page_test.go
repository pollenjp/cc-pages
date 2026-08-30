package server

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pollenjp/cc-pages/internal/config"
	"github.com/pollenjp/cc-pages/internal/index"
	"github.com/pollenjp/cc-pages/internal/store"
)

// realFixture は実ファイルを伴う索引とハンドラを返す。
// pageDirs の順に 0001, 0002... のページを作り、それぞれに fragment を書く。
func realFixture(t *testing.T, fragment string, pageDirs ...string) http.Handler {
	t.Helper()
	root := t.TempDir()
	sessionPath := filepath.Join(root, "sessions", "20260829-aaaa")

	var pages []store.PageEntry
	for i, name := range pageDirs {
		dir := filepath.Join(sessionPath, name)
		if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(fragment), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "assets", "a.txt"), []byte("あさっと"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "page.json"), []byte("{\"schema\":1}"), 0o600); err != nil {
			t.Fatal(err)
		}
		pages = append(pages, store.PageEntry{
			DirName: name, DirPath: dir,
			Page: store.Page{
				ID: store.FormatID(i + 1), Title: name,
				Mode: store.ModeFragment, CreatedAt: at(9),
			},
		})
	}

	ix := index.New()
	ix.Replace(map[string]store.SessionEntry{
		"20260829-aaaa": {
			DirName: "20260829-aaaa", DirPath: sessionPath,
			Session: store.Session{SessionID: "sa", Dir: "20260829-aaaa", LastSeen: at(9)},
			Pages:   pages,
		},
	}, nil)
	return New(config.Config{Addr: "127.0.0.1:7777", Root: root}, ix, func() {}).Handler()
}

func TestPageRendersFragmentInsideChrome(t *testing.T) {
	h := realFixture(t, "<h1>見出し</h1><div class=\"note\">囲み</div>", "0001-alpha")
	rec := get(t, h, "/p/20260829-aaaa/0001-alpha")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"<!doctype html>",                            // chrome が付いている
		"<link rel=\"stylesheet\" href=\"/_/style.css\">",
		"<h1>見出し</h1>",                              // フラグメントがエスケープされていない
		"<div class=\"note\">囲み</div>",
		"cc-pages",                                   // ナビ
	} {
		if !strings.Contains(body, want) {
			t.Errorf("出力に %q が無い", want)
		}
	}
}

func TestPageJapaneseDirNameRoundTrips(t *testing.T) {
	h := realFixture(t, "<p>日本語パス</p>", "0001-テスト")
	rec := get(t, h, "/p/20260829-aaaa/"+url.PathEscape("0001-テスト"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "日本語パス") {
		t.Errorf("本文が出ていない")
	}
}

func TestPageMissingIndexHTMLShowsPlaceholder(t *testing.T) {
	root := t.TempDir()
	pageDir := filepath.Join(root, "sessions", "20260829-aaaa", "0001-mada")
	if err := os.MkdirAll(pageDir, 0o700); err != nil {
		t.Fatal(err)
	}
	ix := index.New()
	ix.Replace(map[string]store.SessionEntry{
		"20260829-aaaa": {
			DirName: "20260829-aaaa", DirPath: filepath.Dir(pageDir),
			Session: store.Session{SessionID: "sa", Dir: "20260829-aaaa", LastSeen: at(9)},
			Pages: []store.PageEntry{{
				DirName: "0001-mada", DirPath: pageDir,
				Page: store.Page{ID: "0001", Title: "まだ", Mode: store.ModeFragment},
			}},
		},
	}, nil)
	h := New(config.Config{Root: root}, ix, func() {}).Handler()
	rec := get(t, h, "/p/20260829-aaaa/0001-mada")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (書きかけでも 500 にしない)", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "まだ書かれていません") {
		t.Errorf("プレースホルダが出ていない")
	}
}

func TestPagePrevNextLinks(t *testing.T) {
	h := realFixture(t, "<p>x</p>", "0001-alpha", "0002-beta", "0003-gamma")

	first := get(t, h, "/p/20260829-aaaa/0001-alpha").Body.String()
	if strings.Contains(first, "前のページ") {
		t.Errorf("先頭に前のページが出ている")
	}
	if !strings.Contains(first, "/p/20260829-aaaa/0002-beta") {
		t.Errorf("先頭に次のページのリンクが無い")
	}

	mid := get(t, h, "/p/20260829-aaaa/0002-beta").Body.String()
	if !strings.Contains(mid, "/p/20260829-aaaa/0001-alpha") {
		t.Errorf("中間に前のページのリンクが無い")
	}
	if !strings.Contains(mid, "/p/20260829-aaaa/0003-gamma") {
		t.Errorf("中間に次のページのリンクが無い")
	}

	last := get(t, h, "/p/20260829-aaaa/0003-gamma").Body.String()
	if strings.Contains(last, "次のページ") {
		t.Errorf("末尾に次のページが出ている")
	}
}

func TestSessionIndexListsPages(t *testing.T) {
	h := realFixture(t, "<p>x</p>", "0001-alpha", "0002-beta")
	rec := get(t, h, "/p/20260829-aaaa/")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"/p/20260829-aaaa/0001-alpha", "/p/20260829-aaaa/0002-beta"} {
		if !strings.Contains(body, want) {
			t.Errorf("ページ一覧に %q が無い", want)
		}
	}
}

func TestAssetsServed(t *testing.T) {
	h := realFixture(t, "<p>x</p>", "0001-alpha")
	rec := get(t, h, "/p/20260829-aaaa/0001-alpha/assets/a.txt")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Body.String() != "あさっと" {
		t.Errorf("body = %q", rec.Body.String())
	}
}

func TestAssetsCannotEscapeDirectory(t *testing.T) {
	h := realFixture(t, "<p>x</p>", "0001-alpha")
	rec := get(t, h, "/p/20260829-aaaa/0001-alpha/assets/../page.json")
	if rec.Code == http.StatusOK && strings.Contains(rec.Body.String(), "schema") {
		t.Errorf("assets の外に出られてしまった: %d %q", rec.Code, rec.Body.String())
	}
}

func TestPageNotFound(t *testing.T) {
	h := realFixture(t, "<p>x</p>", "0001-alpha")
	if rec := get(t, h, "/p/20260829-aaaa/9999-none"); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	if rec := get(t, h, "/p/none/0001-alpha"); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	if rec := get(t, h, "/p/none/"); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}
