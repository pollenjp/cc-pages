package transcript

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// projects は testdata の jsonl を projects/<slug>/ の形に置いた一時ディレクトリを作る。
func projects(t *testing.T, files ...string) string {
	t.Helper()
	root := t.TempDir()
	proj := filepath.Join(root, "-home-u-proj")
	if err := os.MkdirAll(proj, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		b, err := os.ReadFile(filepath.Join("testdata", f))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(proj, f), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestScanReadsMeta(t *testing.T) {
	got := NewCache().Scan(projects(t, "sample-ok.jsonl"))
	m, ok := got["11111111-2222-3333-4444-555555555555"]
	if !ok {
		t.Fatalf("セッションが見つからない: %v", got)
	}
	if m.AITitle != "挨拶のやりとり" {
		t.Errorf("AITitle = %q", m.AITitle)
	}
	if m.Cwd != "/home/u/proj" {
		t.Errorf("Cwd = %q", m.Cwd)
	}
	if m.GitBranch != "feat/x" {
		t.Errorf("GitBranch = %q, want feat/x (最後の値を採る)", m.GitBranch)
	}
	want := time.Date(2026, 8, 29, 11, 30, 0, 0, time.UTC)
	if !m.LastSeen.Equal(want) {
		t.Errorf("LastSeen = %v, want %v", m.LastSeen, want)
	}
	if filepath.Base(m.Path) != "sample-ok.jsonl" {
		t.Errorf("Path = %q", m.Path)
	}
}

func TestScanSurvivesBrokenLines(t *testing.T) {
	got := NewCache().Scan(projects(t, "sample-broken.jsonl"))
	m, ok := got["99999999-aaaa"]
	if !ok {
		t.Fatalf("壊れた行に挟まれた正常な行が拾えていない: %v", got)
	}
	if m.Cwd != "/home/u/other" {
		t.Errorf("Cwd = %q", m.Cwd)
	}
}

func TestScanMissingDirIsEmpty(t *testing.T) {
	got := NewCache().Scan("/nonexistent/projects")
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

func TestScanCachesByModTimeAndSize(t *testing.T) {
	root := projects(t, "sample-ok.jsonl")
	c := NewCache()
	first := c.Scan(root)
	if len(first) != 1 {
		t.Fatalf("len = %d", len(first))
	}
	// ファイルを空にしても、mtime と size を戻せばキャッシュが効いて同じ結果になる。
	path := filepath.Join(root, "-home-u-proj", "sample-ok.jsonl")
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	before := fi.ModTime()
	if err := os.WriteFile(path, make([]byte, fi.Size()), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, before, before); err != nil {
		t.Fatal(err)
	}
	second := c.Scan(root)
	if _, ok := second["11111111-2222-3333-4444-555555555555"]; !ok {
		t.Errorf("mtime と size が同じなのにキャッシュが効いていない")
	}
}
