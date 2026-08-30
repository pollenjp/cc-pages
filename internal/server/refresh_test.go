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
