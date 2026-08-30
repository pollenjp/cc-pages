package index

import (
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
