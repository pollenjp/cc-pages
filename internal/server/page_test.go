package server

import (
	"html/template"
	"net/http"
	"net/http/httptest"
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
	rec := get(t, h, "/p/20260829-aaaa/0001-alpha/")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"<!doctype html>", // chrome が付いている
		"<link rel=\"stylesheet\" href=\"/_/style.css\">",
		"<h1>見出し</h1>", // フラグメントがエスケープされていない
		"<div class=\"note\">囲み</div>",
		"cc-pages", // ナビ
	} {
		if !strings.Contains(body, want) {
			t.Errorf("出力に %q が無い", want)
		}
	}
}

func TestPageJapaneseDirNameRoundTrips(t *testing.T) {
	h := realFixture(t, "<p>日本語パス</p>", "0001-テスト")
	rec := get(t, h, "/p/20260829-aaaa/"+url.PathEscape("0001-テスト")+"/")
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
	rec := get(t, h, "/p/20260829-aaaa/0001-mada/")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (書きかけでも 500 にしない)", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "まだ書かれていません") {
		t.Errorf("プレースホルダが出ていない")
	}
}

func TestPagePrevNextLinks(t *testing.T) {
	h := realFixture(t, "<p>x</p>", "0001-alpha", "0002-beta", "0003-gamma")

	first := get(t, h, "/p/20260829-aaaa/0001-alpha/").Body.String()
	if strings.Contains(first, "前のページ") {
		t.Errorf("先頭に前のページが出ている")
	}
	if !strings.Contains(first, "/p/20260829-aaaa/0002-beta") {
		t.Errorf("先頭に次のページのリンクが無い")
	}

	mid := get(t, h, "/p/20260829-aaaa/0002-beta/").Body.String()
	if !strings.Contains(mid, "/p/20260829-aaaa/0001-alpha") {
		t.Errorf("中間に前のページのリンクが無い")
	}
	if !strings.Contains(mid, "/p/20260829-aaaa/0003-gamma") {
		t.Errorf("中間に次のページのリンクが無い")
	}

	last := get(t, h, "/p/20260829-aaaa/0003-gamma/").Body.String()
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

// TestPageRendersAtTrailingSlashURL はページの正規 URL (末尾スラッシュ付き) で
// 200 が返ることを確認する。
func TestPageRendersAtTrailingSlashURL(t *testing.T) {
	h := realFixture(t, "<p>x</p>", "0001-alpha")
	if rec := get(t, h, "/p/20260829-aaaa/0001-alpha/"); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

// TestPageSlashlessURLRedirectsToCanonical は末尾スラッシュの無い旧形式が
// 引き続きページに届くことを確認する。cc-pages new は既にこの形の URL を
// 出力しており、設計書も例示で固定しているので、ここを壊すと配った
// リンクが死ぬ。
//
// 実装は明示ルート + 301。ServeMux は「末尾スラッシュ付きだけを登録した」
// 場合に 307 を自動で返すが、それは存在しないページにも無条件で掛かり、
// 「知らない URL は 404」(TestPageNotFound) を崩すので使っていない。
func TestPageSlashlessURLRedirectsToCanonical(t *testing.T) {
	h := realFixture(t, "<p>x</p>", "0001-alpha")
	rec := get(t, h, "/p/20260829-aaaa/0001-alpha")
	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("status = %d, want 301", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if want := "/p/20260829-aaaa/0001-alpha/"; loc != want {
		t.Fatalf("Location = %q, want %q", loc, want)
	}
	if rec2 := get(t, h, loc); rec2.Code != http.StatusOK {
		t.Errorf("リダイレクト先の status = %d, want 200", rec2.Code)
	}
}

// TestNoBaseTagAnywhere は描画されるどのページにも <base> が出ないことを確認する。
//
// ページ URL を末尾スラッシュの正規形にしたことで、相対参照 (assets/x.png) は
// <base> 無しで正しく解決するようになった。逆に <base> を戻すと、HTML 仕様
// どおり <a href="#toc"> が「別文書の断片」に解決されてページ内移動ではなく
// 遷移になり、しかもその URL にはルートが無いので 404 になる。
func TestNoBaseTagAnywhere(t *testing.T) {
	h := realFixture(t, "<p>x</p>", "0001-alpha")
	for _, path := range []string{"/p/20260829-aaaa/0001-alpha/", "/p/20260829-aaaa/"} {
		if body := get(t, h, path).Body.String(); strings.Contains(body, "<base") {
			t.Errorf("%s に base タグが出ている: %s", path, body)
		}
	}
	_, lh, _ := fixture(t)
	if body := get(t, lh, "/").Body.String(); strings.Contains(body, "<base") {
		t.Errorf("一覧に base タグが出ている: %s", body)
	}
}

// TestGeneratedLinksUseTrailingSlash は前後ページのリンクとセッション内一覧の
// リンクが正規形 (末尾スラッシュ付き) であることを確認する。前方一致では
// スラッシュを落とした実装を素通しさせてしまうので、完全な形で照合する。
func TestGeneratedLinksUseTrailingSlash(t *testing.T) {
	h := realFixture(t, "<p>x</p>", "0001-alpha", "0002-beta", "0003-gamma")

	mid := get(t, h, "/p/20260829-aaaa/0002-beta/").Body.String()
	for _, want := range []string{
		`href="/p/20260829-aaaa/0001-alpha/"`,
		`href="/p/20260829-aaaa/0003-gamma/"`,
	} {
		if !strings.Contains(mid, want) {
			t.Errorf("前後リンクに %s が無い: %s", want, mid)
		}
	}

	list := get(t, h, "/p/20260829-aaaa/").Body.String()
	for _, want := range []string{
		`href="/p/20260829-aaaa/0001-alpha/"`,
		`href="/p/20260829-aaaa/0002-beta/"`,
	} {
		if !strings.Contains(list, want) {
			t.Errorf("セッション内一覧に %s が無い: %s", want, list)
		}
	}
}

// TestPageLinksEscapeJapaneseDirName は日本語のページディレクトリ名でも、
// 生成されるリンクがパーセントエスケープされ末尾スラッシュで終わることを
// 確認する。期待値はハードコードしたバイト列ではなく url.PathEscape から導き、
// エスケープの規則そのものを検証する。
func TestPageLinksEscapeJapaneseDirName(t *testing.T) {
	h := realFixture(t, "<p>x</p>", "0001-テスト", "0002-つぎ")
	canonical := "/p/20260829-aaaa/" + url.PathEscape("0001-テスト") + "/"

	rec := get(t, h, canonical)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	want := `href="/p/20260829-aaaa/` + url.PathEscape("0002-つぎ") + `/"`
	if !strings.Contains(rec.Body.String(), want) {
		t.Errorf("次のページへのリンク %s が無い: %s", want, rec.Body.String())
	}

	// 旧形式もリダイレクト経由で正規形へ届く。
	old := strings.TrimSuffix(canonical, "/")
	r := get(t, h, old)
	if r.Code != http.StatusMovedPermanently || r.Header().Get("Location") != canonical {
		t.Errorf("%q -> %d %q, want 301 %q", old, r.Code, r.Header().Get("Location"), canonical)
	}
}

// TestBreadcrumbLinksToSession はページのパンくずのセッション部分が、そのセッションの
// ページ一覧へのリンクになっていることを確認する。ここが素のテキストだと、ページから
// 同じセッションの他のページへ戻る動線が無くなる。
func TestBreadcrumbLinksToSession(t *testing.T) {
	h := realFixture(t, "<p>x</p>", "0001-alpha")
	body := get(t, h, "/p/20260829-aaaa/0001-alpha/").Body.String()
	if !strings.Contains(body, `<span class="crumb"><a href="/p/20260829-aaaa/">`) {
		t.Errorf("パンくずのセッションがリンクになっていない: %s", body)
	}
	// セッション内一覧では自分自身へのパンくずリンクは出さない。
	sess := get(t, h, "/p/20260829-aaaa/").Body.String()
	if strings.Contains(sess, `<span class="crumb"><a`) {
		t.Errorf("セッション内一覧でパンくずがリンクになっている: %s", sess)
	}
}

// TestRenderWritesNothingOnTemplateFailure は、描画が途中で失敗したときに w へ
// 部分的な HTML が漏れないことを確認する。
//
// w へ直接 ExecuteTemplate すると、Content-Type と 200 を立てた後で失敗した場合に
// 部分出力が既に書き出されており、続く http.Error はヘッダを差し替えられず壊れた
// HTML に平文を追記するだけになる。render がいったんバッファに組み立ててから
// コピーしているのはこのため。unexported なので同じパッケージから直接呼ぶ。
func TestRenderWritesNothingOnTemplateFailure(t *testing.T) {
	const partial = "PARTIAL-OUTPUT-MUST-NOT-LEAK"
	// .Boom は pageData に無いフィールドなので、partial を書き出した後に
	// 実行時エラーになる。
	tmpl := template.Must(template.New("layout").Parse(partial + "{{.Boom}}"))

	s := New(config.Config{}, index.New(), func() {})
	rec := httptest.NewRecorder()
	s.render(rec, tmpl, pageData{})

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if body := rec.Body.String(); strings.Contains(body, partial) {
		t.Errorf("部分出力が漏れている: %q", body)
	}
	if ct := rec.Header().Get("Content-Type"); strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, 失敗経路で text/html を立ててはいけない", ct)
	}
}
