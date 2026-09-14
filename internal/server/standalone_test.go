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

// sampleDoc は standalone ページが書く「完全な HTML」。chrome の出力と
// 見分けられるよう、中の文書にしか出ない文字列を含める。
const sampleDoc = `<!doctype html><html lang="ja"><head><title>中の文書</title></head>` +
	`<body><h1>完全な HTML</h1><script>document.title = "js が動いた"</script></body></html>`

// standaloneFixture は 1 ページ (0001-alpha) だけを持つ索引とハンドラを返す。
// doc が空文字なら index.html を書かない (cc-pages new した直後の状態)。
func standaloneFixture(t *testing.T, mode, doc string) http.Handler {
	t.Helper()
	return standaloneFixtureNamed(t, mode, doc, "0001-alpha")
}

// standaloneFixtureNamed はページディレクトリ名を指定できる standaloneFixture。
func standaloneFixtureNamed(t *testing.T, mode, doc, dirName string) http.Handler {
	t.Helper()
	root := t.TempDir()
	sessionPath := filepath.Join(root, "sessions", "20260829-aaaa")
	pageDir := filepath.Join(sessionPath, dirName)
	if err := os.MkdirAll(filepath.Join(pageDir, "assets"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pageDir, "assets", "a.txt"), []byte("あさっと"), 0o600); err != nil {
		t.Fatal(err)
	}
	if doc != "" {
		if err := os.WriteFile(filepath.Join(pageDir, "index.html"), []byte(doc), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ix := index.New()
	ix.Replace(map[string]store.SessionEntry{
		"20260829-aaaa": {
			DirName: "20260829-aaaa", DirPath: sessionPath,
			Session: store.Session{SessionID: "sa", Dir: "20260829-aaaa", LastSeen: at(9)},
			Pages: []store.PageEntry{{
				DirName: dirName, DirPath: pageDir,
				Page: store.Page{ID: "0001", Title: "page.json のタイトル", Mode: mode, CreatedAt: at(9)},
			}},
		},
	}, nil)
	return New(config.Config{Addr: "127.0.0.1:7777", Root: root}, ix, func() {}).Handler()
}

// TestStandalonePageServesDocumentVerbatim は、standalone ページの URL が
// index.html をそのまま返すことを確認する。
//
// iframe に入れず、ページ本体の URL で直接開く。完全な HTML を書けるように
// した目的 (表現力) に対して、chrome の中の箱に押し込むのは逆行するため。
func TestStandalonePageServesDocumentVerbatim(t *testing.T) {
	h := standaloneFixture(t, store.ModeStandalone, sampleDoc)
	rec := get(t, h, "/p/20260829-aaaa/0001-alpha/")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Body.String(); got != sampleDoc {
		t.Errorf("body = %q\nwant 元のバイト列そのまま", got)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}

// TestStandalonePageHasNoChrome は、standalone に外枠が一切被さらないことを見る。
func TestStandalonePageHasNoChrome(t *testing.T) {
	h := standaloneFixture(t, store.ModeStandalone, sampleDoc)
	body := get(t, h, "/p/20260829-aaaa/0001-alpha/").Body.String()
	for _, unwanted := range []string{
		"<iframe",      // 箱に入れない
		"<article>",    // フラグメント用の包みも被せない
		"再読み込み",        // ナビ
		"/_/style.css", // アプリの CSS は当たらない
		"/_/width.js",  // 幅トグルも chrome の一部。standalone は元から幅自由
		"widthtoggle",
		"page.json のタイトル", // <title> は中の文書のものを使う
	} {
		if strings.Contains(body, unwanted) {
			t.Errorf("standalone に %q が混ざっている", unwanted)
		}
	}
}

// TestStandalonePageCSPAllowsInlineScript は standalone にだけ付く CSP を
// 字面で拘束する。
//
// TestCSPHeader と同じ理由で server 側の定数ではなく独立したリテラルと比べる。
// script-src にインラインを開けるのが standalone モードの存在理由そのもので、
// 逆に外部オリジンを一切足さないのがこのビューアの前提 (ローカル完結)。
func TestStandalonePageCSPAllowsInlineScript(t *testing.T) {
	h := standaloneFixture(t, store.ModeStandalone, sampleDoc)
	const wantCSP = "default-src 'self'; style-src 'self' 'unsafe-inline'; " +
		"script-src 'self' 'unsafe-inline'; img-src 'self' data:"
	got := get(t, h, "/p/20260829-aaaa/0001-alpha/").Header().Get("Content-Security-Policy")
	if got != wantCSP {
		t.Errorf("CSP = %q, want %q", got, wantCSP)
	}
}

// TestFragmentPageKeepsChromeAndStrictCSP は、standalone を足したことで
// fragment 側が何も変わっていないことを見る。
func TestFragmentPageKeepsChromeAndStrictCSP(t *testing.T) {
	h := standaloneFixture(t, store.ModeFragment, "<h1>断片</h1>")
	rec := get(t, h, "/p/20260829-aaaa/0001-alpha/")
	body := rec.Body.String()
	for _, want := range []string{"<article>", "<h1>断片</h1>", "/_/style.css", "再読み込み"} {
		if !strings.Contains(body, want) {
			t.Errorf("fragment ページに %q が無い", want)
		}
	}
	const wantCSP = "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:"
	if got := rec.Header().Get("Content-Security-Policy"); got != wantCSP {
		t.Errorf("CSP = %q, want %q (fragment は据え置き)", got, wantCSP)
	}
}

// TestStandalonePageMissingIndexHTMLShowsPlaceholder は、cc-pages new した
// 直後 (index.html がまだ無い) に開いても 404 ではなく、fragment と同じ
// プレースホルダを chrome 付きで見せることを確認する。
func TestStandalonePageMissingIndexHTMLShowsPlaceholder(t *testing.T) {
	h := standaloneFixture(t, store.ModeStandalone, "")
	rec := get(t, h, "/p/20260829-aaaa/0001-alpha/")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (書きかけでも 500/404 にしない)", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "まだ書かれていません") {
		t.Errorf("プレースホルダが出ていない")
	}
}

// TestStandalonePageMissingIndexHTMLKeepsStrictCSP は、中身が無くて chrome へ
// 落ちたときに CSP まで緩めたままにしないことを見る。standalone というだけで
// アプリ自身のページに script-src が開くと、緩和の範囲が「その文書 1 枚」から
// はみ出す。
func TestStandalonePageMissingIndexHTMLKeepsStrictCSP(t *testing.T) {
	h := standaloneFixture(t, store.ModeStandalone, "")
	got := get(t, h, "/p/20260829-aaaa/0001-alpha/").Header().Get("Content-Security-Policy")
	if strings.Contains(got, "script-src") {
		t.Errorf("chrome に落ちたのに CSP が緩んでいる: %q", got)
	}
}

func TestStandaloneJapaneseDirNameServesDocument(t *testing.T) {
	h := standaloneFixtureNamed(t, store.ModeStandalone, sampleDoc, "0001-テスト")
	rec := get(t, h, "/p/20260829-aaaa/"+url.PathEscape("0001-テスト")+"/")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Body.String() != sampleDoc {
		t.Errorf("日本語ディレクトリで中身が返っていない")
	}
}

// TestStandaloneAssetsResolveRelativeToPageURL は、中の文書が書く相対参照
// (assets/a.txt) の解決先が実際に引けることを確認する。
//
// ページ本体の URL が末尾スラッシュで終わるので、相対参照は
// /p/{session}/{page}/assets/... に解決される。ここが崩れると、standalone に
// した瞬間に画像だけ全部落ちる。
func TestStandaloneAssetsResolveRelativeToPageURL(t *testing.T) {
	h := standaloneFixture(t, store.ModeStandalone, sampleDoc)
	rec := get(t, h, "/p/20260829-aaaa/0001-alpha/assets/a.txt")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Body.String() != "あさっと" {
		t.Errorf("body = %q", rec.Body.String())
	}
}
