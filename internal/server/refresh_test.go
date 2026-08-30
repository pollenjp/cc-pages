package server

import (
	"context"
	"os"
	"path/filepath"
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
