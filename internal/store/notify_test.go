package store

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
