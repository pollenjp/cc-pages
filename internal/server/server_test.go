package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pollenjp/cc-pages/internal/config"
	"github.com/pollenjp/cc-pages/internal/index"
	"github.com/pollenjp/cc-pages/internal/store"
)

func at(h int) time.Time { return time.Date(2026, 8, 29, h, 0, 0, 0, time.UTC) }

// mkEntry はテスト用の SessionEntry を組み立てる。pageDir は ASCII にする。
func mkEntry(dir, sid, title, pageDir string, h int) store.SessionEntry {
	return store.SessionEntry{
		DirName: dir, DirPath: "/root/sessions/" + dir,
		Session: store.Session{SessionID: sid, Dir: dir, LastSeen: at(h), GitBranch: "main"},
		Pages: []store.PageEntry{{
			DirName: pageDir, DirPath: "/root/sessions/" + dir + "/" + pageDir,
			Page: store.Page{ID: "0001", Title: title, Summary: "ようやく", CreatedAt: at(h)},
		}},
		Bytes: 2048,
	}
}

// fixture は 2 セッション入りの索引と、それを載せたハンドラを返す。
// 3 つ目は refresh が呼ばれた回数。
func fixture(t *testing.T) (*index.Index, http.Handler, *int) {
	t.Helper()
	ix := index.New()
	ix.Replace(map[string]store.SessionEntry{
		"20260829-aaaa": mkEntry("20260829-aaaa", "sa", "Notion の調査", "0001-notion", 9),
		"20260829-bbbb": mkEntry("20260829-bbbb", "sb", "Slack の調査", "0001-slack", 18),
	}, nil)

	calls := 0
	s := New(config.Config{Addr: "127.0.0.1:7777", Root: "/root"}, ix, func() { calls++ })
	return ix, s.Handler(), &calls
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestListShowsSessionsNewestFirst(t *testing.T) {
	_, h, _ := fixture(t)
	rec := get(t, h, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	slack := strings.Index(body, "Slack の調査")
	notion := strings.Index(body, "Notion の調査")
	if slack < 0 || notion < 0 {
		t.Fatalf("両方のセッションが出ていない")
	}
	if slack > notion {
		t.Errorf("新しい順になっていない")
	}
}

func TestListPageLinksUseOwnSessionDir(t *testing.T) {
	_, h, _ := fixture(t)
	body := get(t, h, "/").Body.String()
	for _, want := range []string{
		"/p/20260829-aaaa/0001-notion",
		"/p/20260829-bbbb/0001-slack",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("リンクが無い: %s", want)
		}
	}
	// セッションを取り違えたリンクが出ていないこと。
	for _, bad := range []string{
		"/p/20260829-aaaa/0001-slack",
		"/p/20260829-bbbb/0001-notion",
	} {
		if strings.Contains(body, bad) {
			t.Errorf("別セッションのページにリンクしている: %s", bad)
		}
	}
}

func TestListFiltersByQuery(t *testing.T) {
	_, h, _ := fixture(t)
	body := get(t, h, "/?q=slack").Body.String()
	if strings.Contains(body, "Notion の調査") {
		t.Errorf("絞り込まれていない")
	}
	if !strings.Contains(body, "Slack の調査") {
		t.Errorf("該当が消えている")
	}
}

func TestListEmptyState(t *testing.T) {
	ix := index.New()
	ix.Replace(map[string]store.SessionEntry{}, nil)
	s := New(config.Config{Addr: "127.0.0.1:7777"}, ix, func() {})
	body := get(t, s.Handler(), "/").Body.String()
	if !strings.Contains(body, "まだページがありません") {
		t.Errorf("空状態が出ていない")
	}
}

func TestCSPHeader(t *testing.T) {
	_, h, _ := fixture(t)
	// CSP はこのプロジェクトが正確な値を拘束している数少ない制約なので、
	// server.CSP 定数ではなく独立したリテラルと比較する。定数同士の比較だと
	// server.go 側で CSP を緩めてもテストが追従してしまい、検出できない。
	const wantCSP = "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:"
	got := get(t, h, "/").Header().Get("Content-Security-Policy")
	if got != wantCSP {
		t.Errorf("CSP = %q, want %q", got, wantCSP)
	}
	// フラグメントに <script> を書けない設計の担保。実際に返ったヘッダで検証する。
	if strings.Contains(got, "script-src") {
		t.Errorf("script-src を開けてはいけない: %q", got)
	}
}

func TestStyleCSSServed(t *testing.T) {
	_, h, _ := fixture(t)
	rec := get(t, h, "/_/style.css")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "--bg") {
		t.Errorf("style.css の中身が違う")
	}
}

func TestTemplatesNotServedAsAssets(t *testing.T) {
	_, h, _ := fixture(t)
	if rec := get(t, h, "/_/layout.html"); rec.Code == http.StatusOK {
		t.Errorf("テンプレートが配信されている")
	}
}

// TestTouchReturnsNoContentWithoutAccept は cc-pages new からの呼び出しを想定する。
// Accept ヘッダが無いリクエストには 204 を返す。
func TestTouchReturnsNoContentWithoutAccept(t *testing.T) {
	_, h, calls := fixture(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/_/touch", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if *calls != 1 {
		t.Errorf("refresh が呼ばれた回数 = %d, want 1", *calls)
	}
}

// TestTouchRedirectsForBrowser はブラウザの「再読み込み」フォーム送信を想定する。
// Accept ヘッダがあるリクエストには一覧へのリダイレクトを返す。
func TestTouchRedirectsForBrowser(t *testing.T) {
	_, h, calls := fixture(t)
	req := httptest.NewRequest(http.MethodPost, "/_/touch", nil)
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != "/" {
		t.Errorf("Location = %q, want %q", got, "/")
	}
	if *calls != 1 {
		t.Errorf("refresh が呼ばれた回数 = %d, want 1", *calls)
	}
}

func TestUnknownPathIs404(t *testing.T) {
	_, h, _ := fixture(t)
	if rec := get(t, h, "/p/none/none"); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestHumanBytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{{512, "512 B"}, {2048, "2.0 KB"}, {1572864, "1.5 MB"}}
	for _, c := range cases {
		if got := humanBytes(c.in); got != c.want {
			t.Errorf("humanBytes(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}
