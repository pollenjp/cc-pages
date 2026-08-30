// cc-pages は Claude Code の応答を HTML ページとしてローカルに残し、
// web app で読むためのツール。
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
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
		return fmt.Errorf("サブコマンドが要る (new / serve)")
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

// tagList は繰り返し指定できる --tag を受ける。
type tagList []string

func (t *tagList) String() string     { return fmt.Sprint([]string(*t)) }
func (t *tagList) Set(v string) error { *t = append(*t, v); return nil }

// cmdNew はページディレクトリを作り、{"id","dir","url"} を stdout に出す。
func cmdNew(cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("new", flag.ContinueOnError)
	var (
		session = fs.String("session", "", "セッション ID (CLAUDE_CODE_SESSION_ID)")
		title   = fs.String("title", "", "ページのタイトル")
		summary = fs.String("summary", "", "一覧に出る 1 行")
		prompt  = fs.String("prompt", "", "元になった依頼")
		mode    = fs.String("mode", "", "fragment (既定) または standalone")
		tags    tagList
	)
	fs.Var(&tags, "tag", "タグ (繰り返し指定できる)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	res, err := store.CreatePage(cfg.Root, cfg.BaseURL(), time.Now(), store.NewPageInput{
		SessionID: *session,
		Title:     *title,
		Summary:   *summary,
		Tags:      tags,
		Prompt:    *prompt,
		Mode:      *mode,
	})
	if err != nil {
		return err
	}
	store.NotifyTouch(cfg.BaseURL())
	enc := json.NewEncoder(os.Stdout)
	return enc.Encode(res)
}

// cmdServe は索引を作り、HTTP サーバを起動する。
func cmdServe(cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	every := fs.Duration("rescan", 60*time.Second, "定期走査の間隔")
	if err := fs.Parse(args); err != nil {
		return err
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
		Addr:              cfg.Addr,
		Handler:           server.New(cfg, ix, r.Refresh).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(sctx)
	}()

	fmt.Fprintf(os.Stderr, "cc-pages: %s で待ち受けます (root=%s)\n", cfg.BaseURL(), cfg.Root)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
