// cc-pages は Claude Code の応答を HTML ページとしてローカルに残し、
// web app で読むためのツール。
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/pollenjp/cc-pages/internal/config"
	"github.com/pollenjp/cc-pages/internal/index"
	"github.com/pollenjp/cc-pages/internal/server"
	"github.com/pollenjp/cc-pages/internal/store"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "cc-pages:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("サブコマンドが要る (new / serve / status / link)")
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	switch args[0] {
	case "new":
		return cmdNew(cfg, args[1:])
	case "serve":
		return cmdServe(cfg, args[1:])
	case "status":
		return cmdStatus(cfg, args[1:], os.Stdout)
	case "link":
		return cmdLink(cfg, args[1:], os.Stdin, os.Stdout, os.Stderr)
	default:
		return fmt.Errorf("不明なサブコマンド: %s", args[0])
	}
}

// loadConfig は環境変数とホームディレクトリから設定を解決する。
func loadConfig() (config.Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return config.Config{}, err
	}
	return config.Load(os.Getenv, config.DefaultConfigPath(home), home)
}

// multiFlag は繰り返し指定できる文字列のフラグ (--tag / --now / --next)。
type multiFlag []string

func (m *multiFlag) String() string     { return fmt.Sprint([]string(*m)) }
func (m *multiFlag) Set(v string) error { *m = append(*m, v); return nil }

// newFlags は new の FlagSet を作り、各 flag を in のフィールドに結びつける。
//
// cmdNew から出してあるのは、skill_test.go が skill に書いた flag の実在を
// 同じ定義で確かめるため。flag を足すときはここに足せば、両方に効く。
func newFlags(in *store.NewPageInput) *flag.FlagSet {
	fs := flag.NewFlagSet("new", flag.ContinueOnError)
	fs.StringVar(&in.SessionID, "session", "", "セッション ID (CLAUDE_CODE_SESSION_ID)")
	fs.StringVar(&in.Title, "title", "", "ページのタイトル")
	fs.StringVar(&in.Summary, "summary", "", "一覧に出る 1 行")
	fs.StringVar(&in.Prompt, "prompt", "", "元になった依頼")
	fs.StringVar(&in.Mode, "mode", "", "fragment (既定) または standalone")
	fs.Var((*multiFlag)(&in.Tags), "tag", "タグ (繰り返し指定できる)")
	return fs
}

// cmdNew はページディレクトリを作り、{"id","dir","url"} を stdout に出す。
func cmdNew(cfg config.Config, args []string) error {
	var in store.NewPageInput
	if err := newFlags(&in).Parse(args); err != nil {
		return err
	}

	res, err := store.CreatePage(cfg.Root, cfg.BaseURL(), time.Now(), in)
	if err != nil {
		return err
	}
	store.NotifyTouch(cfg.BaseURL())
	enc := json.NewEncoder(os.Stdout)
	return enc.Encode(res)
}

// statusFlags は status の FlagSet を作り、各 flag を in のフィールドに結びつける。
// cmdStatus から出してある理由は newFlags と同じ。
func statusFlags(in *store.StatusInput) *flag.FlagSet {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.StringVar(&in.SessionID, "session", "", "セッション ID (CLAUDE_CODE_SESSION_ID)")
	fs.Var((*multiFlag)(&in.Now), "now", "現在の状況 (繰り返し指定できる)")
	fs.Var((*multiFlag)(&in.Next), "next", "次やること (繰り返し指定できる)")
	return fs
}

// cmdStatus はセッションの「現在の状況」を全置換で書き、{"url"} を out に出す。
//
// out を引数に取るのはテストのため (cmdNew は os.Stdout に直接書いている)。
// 検証エラーのときは out に何も書かない。skill は stdout を JSON として読むので、
// 失敗時に半端な出力を残さない。
func cmdStatus(cfg config.Config, args []string, out io.Writer) error {
	var in store.StatusInput
	if err := statusFlags(&in).Parse(args); err != nil {
		return err
	}

	res, err := store.UpdateStatus(cfg.Root, cfg.BaseURL(), time.Now(), in)
	if err != nil {
		return err
	}
	store.NotifyTouch(cfg.BaseURL())
	return json.NewEncoder(out).Encode(res)
}

// linkArgs は cc-pages link の flag。
type linkArgs struct {
	SessionID string
	Stdin     bool
	URL       string
	Label     string
	Group     string
	Source    string
	At        string
}

// linkFlags は link の FlagSet を作り、各 flag を a のフィールドに結びつける。
// cmdLink から出してある理由は newFlags と同じ (flag の定義を 1 か所にする)。
func linkFlags(a *linkArgs) *flag.FlagSet {
	fs := flag.NewFlagSet("link", flag.ContinueOnError)
	fs.StringVar(&a.SessionID, "session", "", "セッション ID (CLAUDE_CODE_SESSION_ID)")
	fs.BoolVar(&a.Stdin, "stdin", false, "標準入力から JSON Lines (1 行 1 件) を読む")
	fs.StringVar(&a.URL, "url", "", "足すリンクの URL (http か https の絶対 URL)")
	fs.StringVar(&a.Label, "label", "", "表示の文字 (無ければ URL を出す)")
	fs.StringVar(&a.Group, "group", "", "組の名前 (無ければ その他)")
	fs.StringVar(&a.Source, "source", "", "assistant (既定) か tool")
	fs.StringVar(&a.At, "at", "", "リンクが出た時刻 (RFC 3339。無ければ今)")
	return fs
}

// cmdLink は会話にリンクを足し、{"url","added","updated","skipped"} を stdout に出す。
//
// 呼ぶ側 (Claude Code の Stop hook) は --stdin で、会話に出たリンクを JSON Lines で
// まとめて渡す。読めない行は理由を stderr に出して捨て、ほかの行は残す (1 件のために
// 全部を落とすと、turn ごとの取り逃しが続く)。手で 1 件を足すときは --url などで渡し、
// それを捨てるなら失敗にする (status の検証エラーと同じく stdout に何も書かない)。
//
// 足したら serve に知らせ、その会話だけを読み直させる。知らせの応答は待たない。
func cmdLink(cfg config.Config, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	var a linkArgs
	fs := linkFlags(&a)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("余分な引数がある: %q", fs.Args())
	}
	// stdin を読む前に確かめる。拒むなら入力を待たせない。
	if a.SessionID == "" {
		return errors.New("--session が要る")
	}
	if err := store.ValidateSessionID(a.SessionID); err != nil {
		return err
	}
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	switch {
	case a.Stdin && set["url"]:
		return errors.New("--stdin と --url は一緒に使えない")
	case !a.Stdin && !set["url"]:
		return errors.New("--stdin か --url のどちらかが要る")
	case a.Stdin:
		for _, name := range []string{"label", "group", "source", "at"} {
			if set[name] {
				return fmt.Errorf("--%s は --url と一緒に使う (--stdin では JSON Lines のキーで渡す)", name)
			}
		}
	}

	now := time.Now()
	warn := func(s string) { fmt.Fprintln(stderr, "cc-pages link:", s) }
	var items []store.LinkInput
	skipped := 0
	if a.Stdin {
		items, skipped = store.ParseLinkLines(stdin, now, warn)
	} else {
		in, err := store.NewLinkInput(a.URL, a.Label, a.Group, a.Source, a.At, now)
		if err != nil {
			return err
		}
		items = []store.LinkInput{in}
	}

	res, err := store.AddLinks(cfg.Root, cfg.BaseURL(), now, a.SessionID, items, warn)
	if err != nil {
		return err
	}
	if !a.Stdin && res.Skipped > 0 {
		return fmt.Errorf("リンクを足せなかった (1 つの会話に残すのは %d 件まで)", store.MaxLinks)
	}
	res.Skipped += skipped
	if res.Changed {
		store.NotifySession(cfg.BaseURL(), res.Dir)
	}
	return json.NewEncoder(stdout).Encode(res)
}

// cmdServe は索引を作り、HTTP サーバを起動する。
func cmdServe(cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	every := fs.Duration("rescan", 60*time.Second, "定期走査の間隔")
	if err := fs.Parse(args); err != nil {
		return err
	}
	// time.NewTicker は d <= 0 で panic する契約になっている。Run はこの後
	// goroutine の中で呼ぶので、ここで弾かないと --rescan 0 のような
	// ありふれた誤指定でプロセス全体が生のスタックトレースで落ちる。
	// os.UserHomeDir や bind より前に確認することで、ネットワークにも
	// ディスクにも触らずに検証できる。
	if *every <= 0 {
		return fmt.Errorf("--rescan は正の値でなければならない: %v", *every)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	ix := index.New()
	r := server.NewRefresher(
		cfg.SessionsDir(),
		filepath.Join(home, ".claude", "projects"),
		ix,
	)
	r.Refresh()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go r.Run(ctx, *every)

	srv := &http.Server{
		Addr: cfg.Addr,
		// touch (cc-pages new / 再読み込みボタン) は差分ではなく全走査。
		// cc-pages link の知らせはその会話だけを読み直す。
		Handler:           server.New(cfg, ix, r.RefreshAll, r.RefreshSession).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	// done は shutdown goroutine が Shutdown まで完了したら閉じる。
	//
	// Shutdown を呼ぶと ListenAndServe は即座に ErrServerClosed を返す
	// (net/http.Server.Shutdown のドキュメントが明示的に警告している罠:
	// "Make sure the program doesn't exit and waits instead for Shutdown
	// to return")。ここで <-done を待たずに return すると、最大 5 秒の
	// ドレインが終わる前にプロセスが終了し、実行中の Shutdown goroutine を
	// 道連れにしてしまう。
	done := make(chan struct{})
	go func() {
		defer close(done)
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(sctx)
	}()

	fmt.Fprintf(os.Stderr, "cc-pages: %s で待ち受けます (root=%s)\n", cfg.BaseURL(), cfg.Root)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		// ポート衝突などの実エラー。ctx はまだキャンセルされておらず、shutdown
		// goroutine は <-ctx.Done() で止まったままなので、ここで <-done を
		// 待つと永久にハングする。待たずに return する。
		return err
	}
	// 不変条件: ここに来るのは ErrServerClosed が返ったとき、つまり
	// Shutdown が既に呼ばれたときだけ。だから shutdown goroutine は必ず
	// 進行しており <-done は必ず閉じる。この行を上の早期 return より前に
	// 動かすと、ポート衝突のたびにハングするので注意。
	<-done
	return nil
}
