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
		Schema:    1,
		Now:       []string{"設計を提示し承認待ち", "案 A を推奨"},
		Next:      []string{"spec を commit", "計画を書く"},
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
