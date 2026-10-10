package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
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

// TestNewFlagsFillInput は newFlags が各 flag を NewPageInput のフィールドに
// 結びつけることを確かめる。cmdNew はこの in をそのまま store.CreatePage に
// 渡すので、結びつけがずれると flag が黙って捨てられる。
func TestNewFlagsFillInput(t *testing.T) {
	var in store.NewPageInput
	err := newFlags(&in).Parse([]string{
		"--session", "409bfd08-aaaa",
		"--title", "題",
		"--summary", "一覧の 1 行",
		"--tag", "調査", "--tag", "Go",
		"--prompt", "元の依頼",
		"--mode", "standalone",
	})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	want := store.NewPageInput{
		SessionID: "409bfd08-aaaa",
		Title:     "題",
		Summary:   "一覧の 1 行",
		Tags:      []string{"調査", "Go"},
		Prompt:    "元の依頼",
		Mode:      "standalone",
	}
	if !reflect.DeepEqual(in, want) {
		t.Errorf("in = %+v, want %+v", in, want)
	}
}

func TestStatusFlagsFillInput(t *testing.T) {
	var in store.StatusInput
	err := statusFlags(&in).Parse([]string{
		"--session", "409bfd08-aaaa",
		"--now", "進行中",
		"--next", "次にやる", "--next", "その次",
	})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	want := store.StatusInput{
		SessionID: "409bfd08-aaaa",
		Now:       []string{"進行中"},
		Next:      []string{"次にやる", "その次"},
	}
	if !reflect.DeepEqual(in, want) {
		t.Errorf("in = %+v, want %+v", in, want)
	}
}

// TestOldBinaryErrorTexts は、skill が「binary が skill より古い」と見分けるのに
// 使う 2 つの文言を固定する。
//
// skill は main を追い、利用者の binary は build した日のまま残る。今日の binary も
// いずれ誰かの手元で「古い binary」になるので、ここの文言を変えると、その日の
// skill がずれを見分けられなくなる。
func TestOldBinaryErrorTexts(t *testing.T) {
	t.Run("知らない flag", func(t *testing.T) {
		fs := newFlags(&store.NewPageInput{})
		fs.SetOutput(io.Discard)
		err := fs.Parse([]string{"--no-such-flag", "x"})
		if err == nil || !strings.Contains(err.Error(), "flag provided but not defined") {
			t.Errorf("err = %v, want \"flag provided but not defined\" を含む", err)
		}
	})
	t.Run("知らないサブコマンド", func(t *testing.T) {
		// run は先に設定を読む。手元の ~/.config/cc-pages/config.toml に左右されないよう
		// HOME を空のディレクトリに向ける。
		t.Setenv("HOME", t.TempDir())
		err := run([]string{"no-such-subcommand"})
		if err == nil || !strings.Contains(err.Error(), "不明なサブコマンド") {
			t.Errorf("err = %v, want \"不明なサブコマンド\" を含む", err)
		}
	})
}
