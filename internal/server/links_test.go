package server

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/pollenjp/cc-pages/internal/config"
	"github.com/pollenjp/cc-pages/internal/index"
	"github.com/pollenjp/cc-pages/internal/store"
)

// lt は 2026-08-<day> hh:mm のローカル時刻。表示はローカルに揃うので、期待する
// 文字列を機械の時刻帯に左右されずに書ける。
func lt(day, h, m int) time.Time { return time.Date(2026, 8, day, h, m, 0, 0, time.Local) }

func mkLink(u, label, group string, sources []string, first, last time.Time) store.Link {
	return store.Link{URL: u, Label: label, Group: group, Sources: sources, FirstSeen: first, LastSeen: last}
}

var (
	asst     = []string{store.SourceAssistant}
	both     = []string{store.SourceAssistant, store.SourceTool}
	toolOnly = []string{store.SourceTool}
)

// linksFixture は 20260829-aaaa (session id sa、ページ 0001-notion と 0002-調べたこと)
// に links を付け、20260829-bbbb (Slack の調査、リンク無し) を並べたハンドラを返す。
// aaaa の会話は 2026-08-29 09:00 (ローカル) に始まった。
func linksFixture(t *testing.T, links ...store.Link) http.Handler {
	t.Helper()
	a := mkEntry("20260829-aaaa", "sa", "Notion の調査", "0001-notion", 9)
	a.Session.LastSeen = lt(29, 18, 0)
	a.Pages[0].Page.CreatedAt = lt(29, 9, 0)
	a.Pages = append(a.Pages, store.PageEntry{
		DirName: "0002-調べたこと", DirPath: "/root/sessions/20260829-aaaa/0002-調べたこと",
		Page: store.Page{ID: "0002", Title: "調べたこと", CreatedAt: lt(29, 10, 0)},
	})
	a.Links = links
	b := mkEntry("20260829-bbbb", "sb", "Slack の調査", "0001-slack", 18)
	ix := index.New()
	ix.Replace(map[string]store.SessionEntry{"20260829-aaaa": a, "20260829-bbbb": b}, nil)
	return New(config.Config{Addr: "127.0.0.1:7777", Root: "/root"}, ix, func() {}, nil).Handler()
}

// standardLinks は組が 3 つ (チケット 15:06・PR 16:00・cc-page 16:10 が組の一番早い
// 最初の時刻) と、tool だけの 1 件。
func standardLinks() []store.Link {
	return []store.Link{
		mkLink("https://app.notion.com/p/abc", "TKT-12", "チケット", asst, lt(29, 15, 6), lt(29, 16, 38)),
		mkLink("https://github.com/o/r/pull/11", "r#11", "PR", both, lt(29, 16, 20), lt(29, 16, 35)),
		mkLink("https://github.com/o/r/pull/12", "r#12", "PR", asst, lt(29, 16, 0), lt(29, 17, 0)),
		mkLink("http://localhost:7777/p/20260829-aaaa/0001-notion/", "📄0001", "cc-page", asst, lt(29, 16, 10), lt(29, 16, 10)),
		mkLink("https://github.com/o/r/actions/runs/1", "run", "CI", toolOnly, lt(29, 15, 0), lt(29, 15, 0)),
	}
}

// indexOf は s の中の sub の位置。無ければテストを止める。
func indexOf(t *testing.T, s, sub string) int {
	t.Helper()
	i := strings.Index(s, sub)
	if i < 0 {
		t.Fatalf("出力に %q が無い", sub)
	}
	return i
}

func TestSessionPageShowsLinks(t *testing.T) {
	body := get(t, linksFixture(t, standardLinks()...), "/p/20260829-aaaa/").Body.String()

	indexOf(t, body, "<h2>リンク (5)</h2>")
	// 組は、組の中で一番早い最初の時刻の順
	ticket, pr, page := indexOf(t, body, "<h3>チケット</h3>"), indexOf(t, body, "<h3>PR</h3>"), indexOf(t, body, "<h3>cc-page</h3>")
	if !(ticket < pr && pr < page) {
		t.Errorf("組の並び: チケット=%d PR=%d cc-page=%d, want チケット → PR → cc-page", ticket, pr, page)
	}
	// 組の中は、最後の時刻の新しいものを上に (r#12 は 17:00、r#11 は 16:35)
	if r12, r11 := indexOf(t, body, ">r#12</a>"), indexOf(t, body, ">r#11</a>"); r12 > r11 {
		t.Errorf("PR の中の並びが最後の時刻の新しい順でない")
	}
	// 置き場所は「ページ」の節の後ろ
	if indexOf(t, body, "<h2>ページ</h2>") > indexOf(t, body, `<section class="links">`) {
		t.Errorf("リンクの節がページの節より前にある")
	}
}

func TestSessionPageFoldsToolOnlyLinks(t *testing.T) {
	body := get(t, linksFixture(t, standardLinks()...), "/p/20260829-aaaa/").Body.String()

	details := indexOf(t, body, "<details>")
	if strings.Contains(body, "<details open") {
		t.Errorf("tool だけのリンクが畳まれていない")
	}
	indexOf(t, body, "<summary>tool の出力だけに出たリンク (1)</summary>")
	if run := indexOf(t, body, ">run</a>"); run < details {
		t.Errorf("tool だけの件が畳んだ所より上に出ている")
	}
	if strings.Count(body, ">run</a>") != 1 {
		t.Errorf("tool だけの件が 2 回出ている")
	}
	// assistant と tool の両方に出た件は上に出る
	if r11 := indexOf(t, body, ">r#11</a>"); r11 > details {
		t.Errorf("assistant を含む件が畳んだ所に入った")
	}
	// 畳んだ中も同じ組分け
	if ci := indexOf(t, body, "<h3>CI</h3>"); ci < details {
		t.Errorf("CI の組が畳んだ所の外にある")
	}
}

func TestSessionPageHidesLinksWhenNone(t *testing.T) {
	body := get(t, linksFixture(t), "/p/20260829-aaaa/").Body.String()
	for _, bad := range []string{`class="links"`, "リンク (", "<details>"} {
		if strings.Contains(body, bad) {
			t.Errorf("リンクの無い会話に %q が出ている", bad)
		}
	}
}

func TestSessionPageLinkNotes(t *testing.T) {
	long := "https://example.com/" + strings.Repeat("a", 60)
	body := get(t, linksFixture(t,
		mkLink("http://localhost:7777/p/20260829-aaaa/0001-notion/", "📄0001", "cc-page", asst, lt(29, 15, 0), lt(29, 15, 0)),
		mkLink("http://localhost:7777/p/20260829-aaaa/0002-%E8%AA%BF%E3%81%B9%E3%81%9F%E3%81%93%E3%81%A8/", "📄0002", "cc-page", asst, lt(29, 15, 1), lt(29, 15, 1)),
		mkLink("http://localhost:7777/p/20260829-aaaa/0002-調べたこと/", "📄0002 (日本語のまま)", "cc-page", asst, lt(29, 15, 1), lt(29, 15, 1)),
		mkLink("http://localhost:7777/p/20260829-bbbb/", "前の会話", "cc-page", asst, lt(29, 15, 2), lt(29, 15, 2)),
		mkLink("https://github.com/o/r/pull/11", "r#11", "PR", asst, lt(29, 15, 3), lt(29, 15, 3)),
		mkLink(long, "長い", "PR", asst, lt(29, 15, 4), lt(29, 15, 4)),
		mkLink("https://ex.com/nolabel", "", "", asst, lt(29, 15, 5), lt(29, 15, 5)),
	), "/p/20260829-aaaa/").Body.String()

	for _, want := range []string{
		// cc-pages 自身のページなら、索引にあるそのページの題。cc-pages new が出す
		// 日本語のままの形でも、%XX に潰した形でも引ける
		`>📄0001</a><span class="desc">Notion の調査</span>`,
		`>📄0002</a><span class="desc">調べたこと</span>`,
		`>📄0002 (日本語のまま)</a><span class="desc">調べたこと</span>`,
		// 会話のトップページなら、その会話の題
		`>前の会話</a><span class="desc">Slack の調査</span>`,
		// ほかは scheme を除いた URL
		`>r#11</a><span class="desc">github.com/o/r/pull/11</span>`,
		// 長い URL は 60 字で切る
		`>長い</a><span class="desc">example.com/` + strings.Repeat("a", 47) + `…</span>`,
		// label の無い件は scheme を除いた URL を label にし、添え書きは空。組は「その他」
		`>ex.com/nolabel</a><span class="desc"></span>`,
		"<h3>その他</h3>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("出力に %q が無い", want)
		}
	}
}

func TestSessionPageLinkTimes(t *testing.T) {
	body := get(t, linksFixture(t,
		mkLink("https://ex.com/1", "同じ分", "", asst, lt(29, 15, 6).Add(10*time.Second), lt(29, 15, 6).Add(40*time.Second)),
		mkLink("https://ex.com/2", "範囲", "", asst, lt(29, 15, 6), lt(29, 16, 38)),
		mkLink("https://ex.com/3", "翌日", "", asst, lt(30, 9, 0), lt(30, 9, 0)),
		mkLink("https://ex.com/4", "日をまたぐ", "", asst, lt(29, 23, 0), lt(30, 1, 0)),
	), "/p/20260829-aaaa/").Body.String()

	for _, want := range []string{
		`<span class="time">15:06</span>`,
		`<span class="time">15:06〜16:38</span>`,
		// 会話の始まり (2026-08-29) と違う日なら日付も付ける
		`<span class="time">2026-08-30 09:00</span>`,
		`<span class="time">23:00〜2026-08-30 01:00</span>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("出力に %q が無い", want)
		}
	}
}

// TestSessionPageDoesNotLinkUnsafeURL は、links.json を手で書いて http(s) でない URL が
// 入っていても、それを <a> にしないことを確かめる (書くときに検証していても、描く
// ときにもう一度確かめる)。
func TestSessionPageDoesNotLinkUnsafeURL(t *testing.T) {
	body := get(t, linksFixture(t,
		mkLink("javascript:alert(1)", "危ない", "", asst, lt(29, 15, 0), lt(29, 15, 0)),
		mkLink("data:text/html,<script>alert(1)</script>", "データ", "", asst, lt(29, 15, 1), lt(29, 15, 1)),
	), "/p/20260829-aaaa/").Body.String()

	for _, bad := range []string{`href="javascript`, `href="data:`, "ZgotmplZ", "<script>alert(1)</script>"} {
		if strings.Contains(body, bad) {
			t.Errorf("出力に %q が出ている", bad)
		}
	}
	for _, want := range []string{"危ない", "データ"} {
		if !strings.Contains(body, want) {
			t.Errorf("label %q が文字としても出ていない", want)
		}
	}
}

func TestExternalLinksHaveRel(t *testing.T) {
	body := get(t, linksFixture(t, standardLinks()...), "/p/20260829-aaaa/").Body.String()
	indexOf(t, body, `<a href="https://github.com/o/r/pull/11" rel="noopener noreferrer"`)
	// 自分の cc-page は外へのリンクではない
	indexOf(t, body, `<a href="http://localhost:7777/p/20260829-aaaa/0001-notion/" title=`)
}

func TestListShowsLinkCount(t *testing.T) {
	a := mkEntry("20260829-aaaa", "sa", "Notion の調査", "0001-notion", 9)
	a.Links = standardLinks()[:2]
	c := store.SessionEntry{ // ページの無い、リンクだけの会話
		DirName: "20260829-cccc", DirPath: "/root/sessions/20260829-cccc",
		Session: store.Session{SessionID: "sc", Dir: "20260829-cccc", LastSeen: at(12)},
		Links:   standardLinks()[4:],
	}
	ix := index.New()
	ix.Replace(map[string]store.SessionEntry{
		"20260829-aaaa": a,
		"20260829-bbbb": mkEntry("20260829-bbbb", "sb", "Slack の調査", "0001-slack", 18),
		"20260829-cccc": c,
	}, nil)
	h := New(config.Config{Addr: "127.0.0.1:7777", Root: "/root"}, ix, func() {}, nil).Handler()

	body := get(t, h, "/").Body.String()
	for _, want := range []string{"<span>2 リンク</span>", "<span>1 リンク</span>", `href="/p/20260829-cccc/"`} {
		if !strings.Contains(body, want) {
			t.Errorf("一覧に %q が無い", want)
		}
	}
	if n := strings.Count(body, " リンク</span>"); n != 2 {
		t.Errorf("リンクの数が %d 回出ている, want 2 (リンクの無い会話には出さない)", n)
	}

	body = get(t, h, "/?q=TKT-12").Body.String()
	if !strings.Contains(body, "Notion の調査") || strings.Contains(body, "Slack の調査") {
		t.Errorf("リンクの label で絞り込めていない")
	}
}

func TestSearchPlaceholderMentionsLinks(t *testing.T) {
	_, h, _ := fixture(t)
	body := get(t, h, "/").Body.String()
	indexOf(t, body, `placeholder="タイトル・要約・タグ・依頼・状況・リンク・cwd・ブランチ"`)
}
