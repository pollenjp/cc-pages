package index

import (
	"reflect"
	"testing"
	"time"

	"github.com/pollenjp/cc-pages/internal/store"
	"github.com/pollenjp/cc-pages/internal/transcript"
)

func at(h int) time.Time {
	return time.Date(2026, 8, 29, h, 0, 0, 0, time.UTC)
}

// entry はテスト用の SessionEntry を組み立てる。
func entry(dirName, sessionID, title string, last time.Time) store.SessionEntry {
	return store.SessionEntry{
		DirName: dirName,
		DirPath: "/root/sessions/" + dirName,
		Session: store.Session{SessionID: sessionID, Dir: dirName, LastSeen: last},
		Pages: []store.PageEntry{{
			DirName: "0001-" + title,
			DirPath: "/root/sessions/" + dirName + "/0001-" + title,
			Page: store.Page{
				ID: "0001", Title: title, Summary: "要約:" + title,
				Tags: []string{"tag-" + title}, Prompt: "依頼:" + title,
				CreatedAt: last, SessionID: sessionID, Mode: store.ModeFragment,
			},
		}},
		Bytes: 100,
	}
}

func TestSessionsSortedByLastSeenDesc(t *testing.T) {
	ix := New()
	ix.Replace(map[string]store.SessionEntry{
		"a": entry("a", "sa", "古い", at(9)),
		"b": entry("b", "sb", "新しい", at(18)),
	}, nil)
	got := ix.Sessions()
	if len(got) != 2 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].DirName != "b" {
		t.Errorf("先頭 = %q, want b", got[0].DirName)
	}
}

func TestTitleFallsBackToFirstPage(t *testing.T) {
	ix := New()
	ix.Replace(map[string]store.SessionEntry{"a": entry("a", "sa", "ページ名", at(9))}, nil)
	if got := ix.Sessions()[0].Title; got != "ページ名" {
		t.Errorf("Title = %q, want ページ名", got)
	}
}

func TestAITitleWins(t *testing.T) {
	ix := New()
	ix.Replace(
		map[string]store.SessionEntry{"a": entry("a", "sa", "ページ名", at(9))},
		map[string]transcript.Meta{"sa": {SessionID: "sa", AITitle: "CC がつけた名前", Cwd: "/w", GitBranch: "br"}},
	)
	s := ix.Sessions()[0]
	if s.Title != "CC がつけた名前" {
		t.Errorf("Title = %q", s.Title)
	}
	if s.Cwd != "/w" || s.GitBranch != "br" {
		t.Errorf("Cwd/GitBranch = %q/%q", s.Cwd, s.GitBranch)
	}
}

func TestSearchMatchesMetaFields(t *testing.T) {
	ix := New()
	ix.Replace(map[string]store.SessionEntry{
		"a": entry("a", "sa", "Notion", at(9)),
		"b": entry("b", "sb", "Slack", at(10)),
	}, nil)
	cases := []struct {
		q    string
		want string
	}{
		{"notion", "a"},     // タイトル (大文字小文字を無視)
		{"要約:Slack", "b"},   // 要約
		{"tag-Notion", "a"}, // タグ
		{"依頼:Slack", "b"},   // 元プロンプト
	}
	for _, c := range cases {
		got := ix.Search(c.q)
		if len(got) != 1 || got[0].DirName != c.want {
			t.Errorf("Search(%q) = %v, want 1 件で %q", c.q, got, c.want)
		}
	}
}

func TestSearchEmptyReturnsAll(t *testing.T) {
	ix := New()
	ix.Replace(map[string]store.SessionEntry{
		"a": entry("a", "sa", "x", at(9)),
		"b": entry("b", "sb", "y", at(10)),
	}, nil)
	if got := ix.Search("   "); len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
}

func TestSearchMatchesCwdAndBranch(t *testing.T) {
	ix := New()
	ix.Replace(
		map[string]store.SessionEntry{"a": entry("a", "sa", "x", at(9))},
		map[string]transcript.Meta{"sa": {SessionID: "sa", Cwd: "/home/u/dotfiles", GitBranch: "feat/abc"}},
	)
	if len(ix.Search("dotfiles")) != 1 {
		t.Error("cwd で引けない")
	}
	if len(ix.Search("feat/abc")) != 1 {
		t.Error("ブランチで引けない")
	}
}

func TestPageLookup(t *testing.T) {
	ix := New()
	ix.Replace(map[string]store.SessionEntry{"a": entry("a", "sa", "T", at(9))}, nil)
	p, path, ok := ix.Page("a", "0001-T")
	if !ok {
		t.Fatal("ページが引けない")
	}
	if p.Title != "T" {
		t.Errorf("Title = %q", p.Title)
	}
	if p.Mode != store.ModeFragment {
		t.Errorf("Mode = %q", p.Mode)
	}
	if path != "/root/sessions/a/0001-T" {
		t.Errorf("path = %q", path)
	}
	if _, _, ok := ix.Page("a", "9999-none"); ok {
		t.Error("無いページが引けてしまった")
	}
}

func TestSessionLookup(t *testing.T) {
	ix := New()
	ix.Replace(map[string]store.SessionEntry{"a": entry("a", "sa", "T", at(9))}, nil)
	if _, ok := ix.Session("a"); !ok {
		t.Error("セッションが引けない")
	}
	if _, ok := ix.Session("none"); ok {
		t.Error("無いセッションが引けてしまった")
	}
}

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

// twoLinks は TKT-12 (チケット) と pull/11 (PR) の 2 件。時刻はわざと UTC と
// 固定オフセットを混ぜる (索引でローカルに揃うことを見るため)。最も早いのは
// pull/11 の 7 時 (UTC)。TKT-12 の 18:06 (JST) は 9:06 (UTC)。
func twoLinks() []store.Link {
	jst := time.FixedZone("JST", 9*3600)
	return []store.Link{
		{
			URL: "https://app.notion.com/p/abc", Label: "TKT-12", Group: "チケット",
			Sources:   []string{store.SourceAssistant},
			FirstSeen: time.Date(2026, 8, 29, 18, 6, 0, 0, jst), LastSeen: time.Date(2026, 8, 29, 19, 38, 0, 0, jst),
		},
		{
			URL: "https://github.com/o/r/pull/11", Sources: []string{store.SourceTool},
			FirstSeen: at(7), LastSeen: at(8),
		},
	}
}

func TestBuildViewCopiesLinks(t *testing.T) {
	e := entry("a", "sa", "ページ名", at(9))
	e.Links = twoLinks()
	ix := New()
	ix.Replace(map[string]store.SessionEntry{"a": e}, nil)
	v := ix.Sessions()[0]

	if v.SessionID != "sa" {
		t.Errorf("SessionID = %q, want sa", v.SessionID)
	}
	if v.LinkCount != 2 || len(v.Links) != 2 {
		t.Fatalf("LinkCount = %d, len(Links) = %d, want 2", v.LinkCount, len(v.Links))
	}
	l := v.Links[0]
	if l.URL != "https://app.notion.com/p/abc" || l.Label != "TKT-12" || l.Group != "チケット" ||
		!reflect.DeepEqual(l.Sources, []string{"assistant"}) {
		t.Errorf("Links[0] = %+v", l)
	}
	if !v.Links[1].FirstSeen.Equal(at(7)) || !v.Links[1].LastSeen.Equal(at(8)) {
		t.Errorf("Links[1] の時刻 = %v 〜 %v", v.Links[1].FirstSeen, v.Links[1].LastSeen)
	}
	// 表示の時刻は索引でローカルに揃える (LastSeen・ページの CreatedAt と同じ理由)
	for i, l := range v.Links {
		if l.FirstSeen.Location() != time.Local || l.LastSeen.Location() != time.Local {
			t.Errorf("Links[%d] の時刻がローカルでない: %v / %v", i, l.FirstSeen.Location(), l.LastSeen.Location())
		}
	}
}

// TestStartedIsEarliest は「会話の始まり」が、手元にある時刻のうち最も早いものになる
// ことを確かめる。session.json の FirstSeen は会話のディレクトリを作った時刻で、
// 呼ぶ側が渡すリンクの時刻 (transcript の時刻) はそれより前のことがある。
func TestStartedIsEarliest(t *testing.T) {
	e := entry("a", "sa", "ページ名", at(12)) // ページの CreatedAt は 12 時
	e.Session.FirstSeen = at(10)
	e.Links = twoLinks() // pull/11 の FirstSeen は 7 時
	ix := New()
	ix.Replace(map[string]store.SessionEntry{"a": e}, nil)
	if got := ix.Sessions()[0].Started; !got.Equal(at(7)) {
		t.Errorf("Started = %v, want リンクの 7 時", got)
	}

	// session.json に FirstSeen が無く、リンクも無ければページの時刻
	e = entry("b", "sb", "ページ名", at(12))
	ix.Replace(map[string]store.SessionEntry{"b": e}, nil)
	if got := ix.Sessions()[0].Started; !got.Equal(at(12)) {
		t.Errorf("Started = %v, want ページの 12 時", got)
	}
}

func TestSearchMatchesLinkLabelAndURL(t *testing.T) {
	a := entry("a", "sa", "リンクのある会話", at(9))
	a.Links = twoLinks()
	ix := New()
	ix.Replace(map[string]store.SessionEntry{
		"a": a,
		"b": entry("b", "sb", "無関係", at(18)),
	}, nil)
	for _, q := range []string{"TKT-12", "tkt-12", "pull/11", "app.notion.com"} {
		got := ix.Search(q)
		if len(got) != 1 || got[0].DirName != "a" {
			t.Errorf("Search(%q) = %d 件, want a の 1 件", q, len(got))
		}
	}
}

func TestSessionByID(t *testing.T) {
	ix := New()
	ix.Replace(map[string]store.SessionEntry{
		"a": entry("a", "sa", "古い方", at(9)),
		"b": entry("b", "sb", "別の会話", at(18)),
		// 同じ session id のディレクトリが割れていたら、新しい方へ送る
		"c": entry("c", "sa", "新しい方", at(20)),
	}, nil)
	if v, ok := ix.SessionByID("sa"); !ok || v.DirName != "c" {
		t.Errorf("SessionByID(sa) = %q, %v, want c", v.DirName, ok)
	}
	if v, ok := ix.SessionByID("sb"); !ok || v.DirName != "b" {
		t.Errorf("SessionByID(sb) = %q, %v, want b", v.DirName, ok)
	}
	for _, id := range []string{"zz", ""} {
		if v, ok := ix.SessionByID(id); ok {
			t.Errorf("SessionByID(%q) が %q を返した", id, v.DirName)
		}
	}
}
