package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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

// runLink は cmdLink を stdin を渡して打ち、stdout と stderr を返す。
func runLink(t *testing.T, cfg config.Config, stdin string, args ...string) (string, string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	err := cmdLink(cfg, args, strings.NewReader(stdin), &out, &errOut)
	return out.String(), errOut.String(), err
}

// linkOut は cc-pages link の stdout の JSON。
type linkOut struct {
	URL     string `json:"url"`
	Added   int    `json:"added"`
	Updated int    `json:"updated"`
	Skipped int    `json:"skipped"`
}

func decodeLinkOut(t *testing.T, out string) linkOut {
	t.Helper()
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("stdout が 1 行ではない: %q", out)
	}
	var keys map[string]any
	if err := json.Unmarshal([]byte(out), &keys); err != nil {
		t.Fatalf("stdout が JSON ではない: %q", out)
	}
	if len(keys) != 4 {
		t.Errorf("stdout のキー = %v, want url・added・updated・skipped の 4 つだけ", keys)
	}
	var res linkOut
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatal(err)
	}
	return res
}

// onlySessionDir は root/sessions にある 1 つだけの会話のディレクトリを返す。
func onlySessionDir(t *testing.T, root string) string {
	t.Helper()
	ents, err := os.ReadDir(filepath.Join(root, "sessions"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 1 {
		t.Fatalf("会話のディレクトリ数 = %d, want 1", len(ents))
	}
	return ents[0].Name()
}

func TestLinkStdinWritesAndPrintsJSON(t *testing.T) {
	cfg := statusCfg(t)
	in := strings.Join([]string{
		`{"url":"https://github.com/o/r/pull/12","label":"r#12","group":"PR"}`,
		`{"url":"javascript:x"}`,
		`{"url":"https://app.notion.com/p/abc","label":"TKT-12","group":"チケット","source":"tool","at":"2026-11-11T15:06:01Z"}`,
	}, "\n") + "\n"
	out, errOut, err := runLink(t, cfg, in, "--session", "409bfd08-aaaa", "--stdin")
	if err != nil {
		t.Fatalf("cmdLink() error = %v (stderr: %s)", err, errOut)
	}
	res := decodeLinkOut(t, out)
	dir := onlySessionDir(t, cfg.Root)
	want := linkOut{URL: "http://localhost:1/p/" + dir + "/", Added: 2, Skipped: 1}
	if res != want {
		t.Errorf("stdout = %+v, want %+v", res, want)
	}
	// 捨てた行は、行番号と理由を stderr に 1 行ずつ
	if !strings.Contains(errOut, "2 行目") || !strings.Contains(errOut, "http(s)") {
		t.Errorf("stderr = %q, want 2 行目とその理由", errOut)
	}
	l, err := store.ReadLinks(filepath.Join(cfg.Root, "sessions", dir))
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Links) != 2 || l.Links[1].Label != "TKT-12" || !reflect.DeepEqual(l.Links[1].Sources, []string{"tool"}) {
		t.Errorf("links.json = %+v", l.Links)
	}
}

func TestLinkFlagsSingleItem(t *testing.T) {
	cfg := statusCfg(t)
	out, errOut, err := runLink(t, cfg, "", "--session", "409bfd08-aaaa",
		"--url", "https://github.com/o/r/pull/12", "--label", "r#12", "--group", "PR",
		"--source", "tool", "--at", "2026-11-11T15:06:01Z")
	if err != nil {
		t.Fatalf("cmdLink() error = %v (stderr: %s)", err, errOut)
	}
	if res := decodeLinkOut(t, out); res.Added != 1 {
		t.Errorf("stdout = %+v, want added 1", res)
	}
	l, err := store.ReadLinks(filepath.Join(cfg.Root, "sessions", onlySessionDir(t, cfg.Root)))
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Links) != 1 {
		t.Fatalf("links.json = %+v", l.Links)
	}
	got := l.Links[0]
	if got.URL != "https://github.com/o/r/pull/12" || got.Label != "r#12" || got.Group != "PR" ||
		!reflect.DeepEqual(got.Sources, []string{"tool"}) ||
		!got.FirstSeen.Equal(time.Date(2026, 11, 11, 15, 6, 1, 0, time.UTC)) {
		t.Errorf("Links[0] = %+v", got)
	}
}

// TestLinkFlagBadItemFails は、flag で渡した 1 件を捨てるときは失敗にすることを
// 確かめる (status の検証エラーと同じ扱い)。stdout は JSON として読まれるので何も書かない。
func TestLinkFlagBadItemFails(t *testing.T) {
	cfg := statusCfg(t)
	out, _, err := runLink(t, cfg, "", "--session", "409bfd08-aaaa", "--url", "javascript:x")
	if err == nil {
		t.Fatal("http(s) でない URL が通った")
	}
	if out != "" {
		t.Errorf("失敗したのに stdout に出力がある: %q", out)
	}
	if _, err := os.Stat(filepath.Join(cfg.Root, "sessions")); !os.IsNotExist(err) {
		t.Errorf("失敗したのに sessions を作った")
	}
}

func TestLinkFlagErrors(t *testing.T) {
	const line = `{"url":"https://a/"}` + "\n"
	cases := []struct {
		name string
		args []string
	}{
		{"--stdin も --url も無い", []string{"--session", "409bfd08-aaaa"}},
		{"両方ある", []string{"--session", "409bfd08-aaaa", "--stdin", "--url", "https://a/"}},
		{"--stdin と --label", []string{"--session", "409bfd08-aaaa", "--stdin", "--label", "x"}},
		{"--session が無い", []string{"--url", "https://a/"}},
		{"使えない session id", []string{"--session", "../x", "--url", "https://a/"}},
		{"余分な引数", []string{"--session", "409bfd08-aaaa", "--stdin", "extra"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := statusCfg(t)
			out, _, err := runLink(t, cfg, line, c.args...)
			if err == nil {
				t.Errorf("%v が通った", c.args)
			}
			if out != "" {
				t.Errorf("失敗したのに stdout に出力がある: %q", out)
			}
		})
	}
}

// TestLinkEmptyStdinCreatesNothing は、渡すリンクが無い turn にも呼ぶ側が打つことを
// 想定する。会話のディレクトリを作らず、一覧に空の会話を増やさない。
func TestLinkEmptyStdinCreatesNothing(t *testing.T) {
	cfg := statusCfg(t)
	out, errOut, err := runLink(t, cfg, "", "--session", "409bfd08-aaaa", "--stdin")
	if err != nil {
		t.Fatalf("cmdLink() error = %v (stderr: %s)", err, errOut)
	}
	if res, want := decodeLinkOut(t, out), (linkOut{URL: "http://localhost:1/s/409bfd08-aaaa/"}); res != want {
		t.Errorf("stdout = %+v, want %+v", res, want)
	}
	if _, err := os.Stat(filepath.Join(cfg.Root, "sessions")); !os.IsNotExist(err) {
		t.Errorf("空の入力で sessions を作った")
	}
}

// TestLinkNotifiesServe は、足したら動いている serve にその会話だけを読み直させる
// こと、何も変わらなければ知らせないことを確かめる。
func TestLinkNotifiesServe(t *testing.T) {
	got := make(chan string, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r.Method + " " + r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	cfg := config.Config{Addr: srv.Listener.Addr().String(), Root: t.TempDir()}
	// at を渡す。無ければ「今」が入り、2 回目は最後の時刻が進んで変わったことになる
	const in = `{"url":"https://github.com/o/r/pull/12","at":"2026-11-11T15:06:01Z"}` + "\n"

	if _, errOut, err := runLink(t, cfg, in, "--session", "409bfd08-aaaa", "--stdin"); err != nil {
		t.Fatalf("cmdLink() error = %v (stderr: %s)", err, errOut)
	}
	select {
	case v := <-got:
		if want := "POST /_/touch/" + onlySessionDir(t, cfg.Root); v != want {
			t.Errorf("知らせ = %q, want %q", v, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("serve に知らせていない")
	}

	if _, errOut, err := runLink(t, cfg, in, "--session", "409bfd08-aaaa", "--stdin"); err != nil {
		t.Fatalf("2 回目の cmdLink() error = %v (stderr: %s)", err, errOut)
	}
	select {
	case v := <-got:
		t.Errorf("何も変わっていないのに知らせた: %q", v)
	case <-time.After(300 * time.Millisecond):
	}
}

func TestLinkFlagsFillInput(t *testing.T) {
	var a linkArgs
	err := linkFlags(&a).Parse([]string{
		"--session", "409bfd08-aaaa", "--stdin",
		"--url", "https://a/", "--label", "r#12", "--group", "PR",
		"--source", "tool", "--at", "2026-11-11T15:06:01Z",
	})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	want := linkArgs{
		SessionID: "409bfd08-aaaa", Stdin: true, URL: "https://a/", Label: "r#12",
		Group: "PR", Source: "tool", At: "2026-11-11T15:06:01Z",
	}
	if a != want {
		t.Errorf("a = %+v, want %+v", a, want)
	}
}

func TestRunListsLinkSubcommand(t *testing.T) {
	err := run(nil)
	if err == nil || !strings.Contains(err.Error(), "link") {
		t.Errorf("サブコマンド無しのエラーに link が載っていない: %v", err)
	}
}
