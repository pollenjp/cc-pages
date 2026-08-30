// Package config は cc-pages の設定解決を担う。
//
// 解決順は 環境変数 → TOML ファイル → 既定値。serve と new が同じ関数を通ることで、
// 「片方だけが systemd の環境変数を見る」という食い違いを作らない。
package config

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	// DefaultAddr は bind するアドレス。LAN には出さない。
	DefaultAddr = "127.0.0.1:7777"
	// EnvAddr / EnvRoot は環境変数名。
	EnvAddr = "CC_PAGES_ADDR"
	EnvRoot = "CC_PAGES_ROOT"
)

// Config は解決済みの設定。
type Config struct {
	Addr string // bind するアドレス
	Root string // データ root の絶対パス
}

// DefaultConfigPath は既定の設定ファイルパスを返す。
func DefaultConfigPath(home string) string {
	return filepath.Join(home, ".config", "cc-pages", "config.toml")
}

// DefaultRoot は既定のデータ root を返す。
func DefaultRoot(home string) string {
	return filepath.Join(home, ".local", "share", "cc-pages")
}

// fileConfig は TOML の形。
type fileConfig struct {
	Addr string `toml:"addr"`
	Root string `toml:"root"`
}

// Load は設定を解決する。configPath が無い場合はエラーにしない。
func Load(env func(string) string, configPath, home string) (Config, error) {
	c := Config{Addr: DefaultAddr, Root: DefaultRoot(home)}

	var fc fileConfig
	if _, err := toml.DecodeFile(configPath, &fc); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return Config{}, err
		}
	}
	if fc.Addr != "" {
		c.Addr = fc.Addr
	}
	if fc.Root != "" {
		c.Root = fc.Root
	}

	if v := env(EnvAddr); v != "" {
		c.Addr = v
	}
	if v := env(EnvRoot); v != "" {
		c.Root = v
	}
	return c, nil
}

// BaseURL は人に見せるリンクの接頭辞を返す。
//
// bind は 127.0.0.1 だが、リンクは localhost で出す。localhost は secure context
// として扱われ、ブラウザの clipboard API がそのまま使えるため。
func (c Config) BaseURL() string {
	host, port, ok := strings.Cut(c.Addr, ":")
	if !ok {
		return "http://" + c.Addr
	}
	switch host {
	case "", "127.0.0.1", "0.0.0.0":
		host = "localhost"
	}
	return "http://" + host + ":" + port
}

// SessionsDir は root/sessions を返す。
func (c Config) SessionsDir() string { return filepath.Join(c.Root, "sessions") }
