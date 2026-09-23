package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pollenjp/cc-pages/internal/config"
	"github.com/pollenjp/cc-pages/internal/store"
)

// TestServeRejectsNonPositiveRescan は --rescan に 0 や負値を渡したときに
// cmdServe がきれいなエラーを返すことを確認する。
//
// time.NewTicker は d <= 0 で panic する契約になっており、Run は goroutine の
// 中で呼ばれるので、検証を怠るとプロセス全体が生のスタックトレースで落ちる。
// この検証は fs.Parse の直後、bind より前に走る必要があるので、ここでは
// cmdServe が (ポート 0 の bind を試みず) 即座に戻ることも確認する。
func TestServeRejectsNonPositiveRescan(t *testing.T) {
	for _, v := range []string{"0", "-1s"} {
		t.Run(v, func(t *testing.T) {
			cfg := config.Config{Addr: "127.0.0.1:0", Root: t.TempDir()}
			done := make(chan error, 1)
			go func() { done <- cmdServe(cfg, []string{"--rescan", v}) }()

			select {
			case err := <-done:
				if err == nil {
					t.Fatalf("--rescan %s がエラーにならなかった", v)
				}
				if !strings.Contains(err.Error(), "--rescan") {
					t.Errorf("エラーメッセージに --rescan が含まれない: %v", err)
				}
			case <-time.After(time.Second):
				// bind まで進んでしまうと ListenAndServe がブロックし続け、
				// ここに来る。検証が bind より前に無いことの徴候。
				t.Fatal("cmdServe が戻らない (bind まで進んでしまった可能性がある)")
			}
		})
	}
}

// statusCfg は status のテスト用の設定。Addr は誰も居ないポートにして、
// NotifyTouch が即座に諦めるようにする (100ms タイムアウト)。
func statusCfg(t *testing.T) config.Config {
	t.Helper()
	return config.Config{Addr: "127.0.0.1:1", Root: t.TempDir()}
}

func TestStatusWritesFileAndPrintsURL(t *testing.T) {
	cfg := statusCfg(t)
	var out bytes.Buffer
	err := cmdStatus(cfg, []string{
		"--session", "409bfd08-aaaa",
		"--now", "進行中",
		"--next", "次にやる", "--next", "その次",
	}, &out)
	if err != nil {
		t.Fatalf("cmdStatus() error = %v", err)
	}

	var res struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("stdout が JSON ではない: %q", out.String())
	}

	ents, err := os.ReadDir(filepath.Join(cfg.Root, "sessions"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 1 {
		t.Fatalf("セッションディレクトリ数 = %d, want 1", len(ents))
	}
	dir := ents[0].Name()
	if want := "http://localhost:1/p/" + dir + "/"; res.URL != want {
		t.Errorf("url = %q, want %q", res.URL, want)
	}
	s, err := store.ReadStatus(filepath.Join(cfg.Root, "sessions", dir))
	if err != nil {
		t.Fatalf("status.json が無い: %v", err)
	}
	if len(s.Now) != 1 || s.Now[0] != "進行中" {
		t.Errorf("Now = %q", s.Now)
	}
	if len(s.Next) != 2 || s.Next[0] != "次にやる" || s.Next[1] != "その次" {
		t.Errorf("Next = %q", s.Next)
	}
}

func TestStatusRejectsNoItems(t *testing.T) {
	cfg := statusCfg(t)
	var out bytes.Buffer
	if err := cmdStatus(cfg, []string{"--session", "409bfd08-aaaa"}, &out); err == nil {
		t.Fatal("now も next も無いのに通った")
	}
	if out.Len() != 0 {
		t.Errorf("失敗したのに stdout に出力がある: %q", out.String())
	}
}

func TestStatusRejectsBadSession(t *testing.T) {
	cfg := statusCfg(t)
	var out bytes.Buffer
	if err := cmdStatus(cfg, []string{"--session", "../x", "--now", "x"}, &out); err == nil {
		t.Fatal("不正な session id が通った")
	}
	if out.Len() != 0 {
		t.Errorf("失敗したのに stdout に出力がある: %q", out.String())
	}
}

func TestRunListsStatusSubcommand(t *testing.T) {
	err := run(nil)
	if err == nil || !strings.Contains(err.Error(), "status") {
		t.Errorf("サブコマンド無しのエラーに status が載っていない: %v", err)
	}
}
