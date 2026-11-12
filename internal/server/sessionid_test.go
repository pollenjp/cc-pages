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

// /s/<session_id>/ は、Claude Code の status line が session id しか知らないまま
// 会話のトップページを開くための入口。

func TestSessionByIDRedirects(t *testing.T) {
	_, h, _ := fixture(t) // 20260829-aaaa の session id は sa
	for _, path := range []string{"/s/sa/", "/s/sa"} {
		rec := get(t, h, path)
		if rec.Code != http.StatusFound {
			t.Errorf("%s: status = %d, want 302", path, rec.Code)
			continue
		}
		if got := rec.Header().Get("Location"); got != "/p/20260829-aaaa/" {
			t.Errorf("%s: Location = %q, want /p/20260829-aaaa/", path, got)
		}
	}
}

func TestSessionByIDNotFound(t *testing.T) {
	_, h, _ := fixture(t)
	rec := get(t, h, "/s/no-such-session/")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	body := rec.Body.String()
	for _, want := range []string{"この会話のページ・リンクはまだありません", `<a href="/">一覧へ</a>`} {
		if !strings.Contains(body, want) {
			t.Errorf("本文に %q が無い", want)
		}
	}
	// 渡された session id は画面に出さない
	if strings.Contains(body, "no-such-session") {
		t.Errorf("session id が本文に出ている")
	}
}

func TestSessionByIDRejectsBadChars(t *testing.T) {
	_, h, _ := fixture(t)
	if rec := get(t, h, "/s/a.b/"); rec.Code != http.StatusNotFound {
		t.Errorf("/s/a.b/: status = %d, want 404", rec.Code)
	}
	// ".." はディレクトリの外へ出る形。会話のページへは送らない
	for _, path := range []string{"/s/%2e%2e/", "/s/../"} {
		rec := get(t, h, path)
		if loc := rec.Header().Get("Location"); strings.HasPrefix(loc, "/p/") {
			t.Errorf("%s: %d で %s に送った", path, rec.Code, loc)
		}
	}
}

// TestSessionByIDFallsBackToDisk は、索引にまだ無い会話でも、ディスクにあれば
// 送ることを確かめる。cc-pages link の知らせが serve に届かなかったとき (serve の
// 再起動中など) にも、入口を定期走査まで切らさない。
func TestSessionByIDFallsBackToDisk(t *testing.T) {
	root := t.TempDir()
	res, err := store.AddLinks(root, "http://localhost:7777", time.Now(), "sess-disk", []store.LinkInput{
		{URL: "https://a/", Source: store.SourceAssistant, At: time.Now()},
	}, func(string) {})
	if err != nil {
		t.Fatal(err)
	}
	var dirs []string
	h := New(config.Config{Addr: "127.0.0.1:7777", Root: root}, index.New(),
		func() {}, func(d string) { dirs = append(dirs, d) }).Handler()

	rec := get(t, h, "/s/sess-disk/")
	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}
	if want := "/p/" + res.Dir + "/"; rec.Header().Get("Location") != want {
		t.Errorf("Location = %q, want %q", rec.Header().Get("Location"), want)
	}
	if len(dirs) != 1 || dirs[0] != res.Dir {
		t.Errorf("その会話を読み直させていない: %q", dirs)
	}
}
