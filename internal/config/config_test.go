package config

import "testing"

// envFunc はテスト用に環境変数を差し替える。
func envFunc(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadDefaults(t *testing.T) {
	got, err := Load(envFunc(nil), "/nonexistent/config.toml", "/home/u")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Addr != "127.0.0.1:7777" {
		t.Errorf("Addr = %q, want 127.0.0.1:7777", got.Addr)
	}
	if got.Root != "/home/u/.local/share/cc-pages" {
		t.Errorf("Root = %q, want /home/u/.local/share/cc-pages", got.Root)
	}
}

func TestLoadEnvWins(t *testing.T) {
	env := envFunc(map[string]string{
		"CC_PAGES_ADDR": "127.0.0.1:9999",
		"CC_PAGES_ROOT": "/tmp/pages",
	})
	got, err := Load(env, "/nonexistent/config.toml", "/home/u")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Addr != "127.0.0.1:9999" || got.Root != "/tmp/pages" {
		t.Errorf("Load() = %+v, want addr 127.0.0.1:9999 root /tmp/pages", got)
	}
}

func TestLoadTOMLUsedWhenEnvAbsent(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/config.toml"
	if err := writeFile(path, "addr = \"127.0.0.1:8888\"\nroot = \"/srv/pages\"\n"); err != nil {
		t.Fatal(err)
	}
	got, err := Load(envFunc(nil), path, "/home/u")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Addr != "127.0.0.1:8888" || got.Root != "/srv/pages" {
		t.Errorf("Load() = %+v, want addr 127.0.0.1:8888 root /srv/pages", got)
	}
}

func TestBaseURLUsesLocalhost(t *testing.T) {
	cases := []struct{ addr, want string }{
		{"127.0.0.1:7777", "http://localhost:7777"},
		{"0.0.0.0:7777", "http://localhost:7777"},
		{":7777", "http://localhost:7777"},
		{"192.168.1.5:7777", "http://192.168.1.5:7777"},
	}
	for _, c := range cases {
		if got := (Config{Addr: c.addr}).BaseURL(); got != c.want {
			t.Errorf("BaseURL(%q) = %q, want %q", c.addr, got, c.want)
		}
	}
}
