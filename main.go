// cc-pages は Claude Code の応答を HTML ページとしてローカルに残し、
// web app で読むためのツール。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/pollenjp/cc-pages/internal/config"
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
	enc := json.NewEncoder(os.Stdout)
	return enc.Encode(res)
}
