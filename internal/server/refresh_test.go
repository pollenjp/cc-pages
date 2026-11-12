package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pollenjp/cc-pages/internal/index"
	"github.com/pollenjp/cc-pages/internal/store"
)

func TestRefreshPicksUpNewPages(t *testing.T) {
	root := t.TempDir()
	ix := index.New()
	r := NewRefresher(filepath.Join(root, "sessions"), "/nonexistent/projects", ix)

	r.Refresh()
	if len(ix.Sessions()) != 0 {
		t.Fatalf("最初は 0 件のはず: %d", len(ix.Sessions()))
	}

	res, err := store.CreatePage(root, "http://localhost:7777", time.Now(), store.NewPageInput{
		SessionID: "sess-aaaa", Title: "新しいページ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(res.Dir, "index.html"), []byte("<p>x</p>"), 0o600); err != nil {
		t.Fatal(err)
	}

	r.Refresh()
	got := ix.Sessions()
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Pages[0].Title != "新しいページ" {
		t.Errorf("Title = %q", got[0].Pages[0].Title)
	}
}

func TestRefreshMissingRootIsNotFatal(t *testing.T) {
	ix := index.New()
	r := NewRefresher("/nonexistent/sessions", "/nonexistent/projects", ix)
	r.Refresh() // panic しないこと
	if len(ix.Sessions()) != 0 {
		t.Errorf("len = %d, want 0", len(ix.Sessions()))
	}
}

func TestRunStopsOnContextCancel(t *testing.T) {
	ix := index.New()
	r := NewRefresher("/nonexistent/sessions", "/nonexistent/projects", ix)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { r.Run(ctx, 10*time.Millisecond); close(done) }()
	time.Sleep(30 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run が止まらない")
	}
}

// TestRefreshAllSeesInPlaceRewrite は「再読み込み」ボタンの経路が、差分走査には
// 見えない変更を拾えることを確認する。
//
// 差分判定は mtime を見るだけで、既存ファイルの上書きはどのディレクトリの mtime も
// 動かさないので Refresh からは見えない。ここで RefreshAll まで差分走査を通して
// しまうと、ボタンが定期走査と同じ取りこぼしを共有し、押しても直らなくなる。
// 仕様が約束している「定期 stat を待たずに走査できる」の実体がこれ。
func TestRefreshAllSeesInPlaceRewrite(t *testing.T) {
	root := t.TempDir()
	ix := index.New()
	r := NewRefresher(filepath.Join(root, "sessions"), "/nonexistent/projects", ix)

	res, err := store.CreatePage(root, "http://localhost:7777", time.Now(), store.NewPageInput{
		SessionID: "sess-aaaa", Title: "T",
	})
	if err != nil {
		t.Fatal(err)
	}
	page := filepath.Join(res.Dir, "index.html")
	if err := os.WriteFile(page, []byte("<p>x</p>"), 0o600); err != nil {
		t.Fatal(err)
	}

	r.Refresh()
	got := ix.Sessions()
	if len(got) != 1 {
		t.Fatalf("セッション数 = %d, want 1", len(got))
	}
	before := got[0].Bytes

	// 同じ名前のファイルを上書きする。ページディレクトリもセッションディレクトリも
	// mtime は動かない。
	body := strings.Repeat("y", 8192)
	if err := os.WriteFile(page, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	r.RefreshAll()
	after := ix.Sessions()[0].Bytes
	if after-before < int64(len(body))-64 {
		t.Errorf("Bytes = %d (前 %d), 上書きした %d バイトが反映されていない", after, before, len(body))
	}
}

// projectsWithTitle は projects/p/s.jsonl に、session id sid の aiTitle を 1 行書く。
func projectsWithTitle(t *testing.T, projects, sid, title string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(projects, "p"), 0o700); err != nil {
		t.Fatal(err)
	}
	line := `{"sessionId":"` + sid + `","aiTitle":"` + title + `","timestamp":"2026-08-29T10:00:00Z"}` + "\n"
	if err := os.WriteFile(filepath.Join(projects, "p", "s.jsonl"), []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
}

// addLink は会話 sid に URL u を 1 件足し、会話のディレクトリ名を返す。
func addLink(t *testing.T, root, sid, u string) string {
	t.Helper()
	res, err := store.AddLinks(root, "http://localhost:7777", time.Now(), sid, []store.LinkInput{
		{URL: u, Source: store.SourceAssistant, At: time.Now()},
	}, func(string) {})
	if err != nil {
		t.Fatal(err)
	}
	return res.Dir
}

// TestRefreshSessionDoesNotRescanTranscripts は、cc-pages link の知らせでその会話だけを
// 読み直し、transcript (~/.claude/projects の jsonl) は読み直さないことを確かめる。
//
// 全体の走査は、変わった transcript を頭から読み直す。このマシンには 50 MB を超える
// transcript があり、turn ごとの知らせのたびにそれを読むと重い。題 (aiTitle) は
// 定期走査が拾えば足りる。
func TestRefreshSessionDoesNotRescanTranscripts(t *testing.T) {
	root := t.TempDir()
	projects := filepath.Join(t.TempDir(), "projects")
	projectsWithTitle(t, projects, "sess-aaaa", "古い題")
	ix := index.New()
	r := NewRefresher(filepath.Join(root, "sessions"), projects, ix)

	dir := addLink(t, root, "sess-aaaa", "https://a/")
	r.Refresh()
	if v := ix.Sessions(); len(v) != 1 || v[0].Title != "古い題" || v[0].LinkCount != 1 {
		t.Fatalf("前提: 索引 = %+v", v)
	}

	projectsWithTitle(t, projects, "sess-aaaa", "新しい題に書き換えた")
	addLink(t, root, "sess-aaaa", "https://b/")
	r.RefreshSession(dir)

	v := ix.Sessions()
	if len(v) != 1 || v[0].LinkCount != 2 {
		t.Fatalf("足したリンクが索引に出ていない: %+v", v)
	}
	if v[0].Title != "古い題" {
		t.Errorf("Title = %q: 知らせで transcript まで読み直した", v[0].Title)
	}
}

func TestRefreshSessionDropsRemovedSession(t *testing.T) {
	root := t.TempDir()
	ix := index.New()
	r := NewRefresher(filepath.Join(root, "sessions"), "/nonexistent/projects", ix)
	gone := addLink(t, root, "sess-aaaa", "https://a/")
	addLink(t, root, "sess-bbbb", "https://b/")
	r.Refresh()
	if len(ix.Sessions()) != 2 {
		t.Fatalf("前提: %d 件", len(ix.Sessions()))
	}

	if err := os.RemoveAll(filepath.Join(root, "sessions", gone)); err != nil {
		t.Fatal(err)
	}
	r.RefreshSession(gone)
	v := ix.Sessions()
	if len(v) != 1 || v[0].SessionID != "sess-bbbb" {
		t.Errorf("索引 = %+v, want sess-bbbb だけ", v)
	}
}
