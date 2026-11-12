package store

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNotifyTouchPostsToTouch(t *testing.T) {
	got := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r.Method + " " + r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	NotifyTouch(srv.URL)
	select {
	case v := <-got:
		if v != "POST /_/touch" {
			t.Errorf("受け取ったのは %q", v)
		}
	default:
		t.Fatal("リクエストが届いていない")
	}
}

func TestNotifyTouchIgnoresDeadServer(t *testing.T) {
	// 誰も居ないポート。panic せず黙って戻ること。
	NotifyTouch("http://127.0.0.1:1")
}

// TestNotifySessionDoesNotWaitForResponse は、cc-pages link の知らせが serve の応答を
// 待たずに戻ることを確かめる。link は turn の終わりごとに打たれるので、serve が
// 忙しい (全体の走査で大きな transcript を読んでいる) 間も待たされてはいけない。
func TestNotifySessionDoesNotWaitForResponse(t *testing.T) {
	got := make(chan string, 1)
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r.Method + " " + r.URL.Path
		select {
		case <-release:
		case <-time.After(2 * time.Second):
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	defer close(release) // srv.Close は処理中の request を待つので、先に返させる

	start := time.Now()
	NotifySession(srv.URL, "20261111-0a1b2c3d")
	if d := time.Since(start); d > 500*time.Millisecond {
		t.Errorf("NotifySession が %v 待った (応答を待っている)", d)
	}
	select {
	case v := <-got:
		if v != "POST /_/touch/20261111-0a1b2c3d" {
			t.Errorf("受け取ったのは %q", v)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("知らせが届いていない")
	}
}

func TestNotifySessionIgnoresDeadServer(t *testing.T) {
	start := time.Now()
	NotifySession("http://127.0.0.1:1", "20261111-0a1b2c3d") // 誰も居ないポート
	if d := time.Since(start); d > 500*time.Millisecond {
		t.Errorf("居ない serve に %v かかった", d)
	}
}
