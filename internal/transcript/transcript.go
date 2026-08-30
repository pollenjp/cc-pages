// Package transcript は ~/.claude/projects の jsonl を best-effort で読む。
//
// jsonl は Claude Code の非公開フォーマットで、キーはバージョンで変わり得る。
// このパッケージの関数はエラーを返さない。読めなければそのセッションのメタが
// 欠けるだけで、cc-pages 本体は動き続ける。
package transcript

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// Meta は 1 セッション分の、jsonl から拾えたメタ。
type Meta struct {
	SessionID string
	AITitle   string
	Cwd       string
	GitBranch string
	LastSeen  time.Time
	Path      string // 元の jsonl のパス
}

// Cache は (パス, mtime, size) で解析結果を使い回す。
type Cache struct {
	entries map[string]cacheEntry
}

type cacheEntry struct {
	modTime time.Time
	size    int64
	meta    []Meta
}

// NewCache は空のキャッシュを作る。
func NewCache() *Cache { return &Cache{entries: map[string]cacheEntry{}} }

// Scan は projectsDir 配下の *.jsonl をすべて読み、session id をキーに返す。
//
// projectsDir が無い場合も空の map を返す。ディレクトリ名のスラッグ規則は
// 実装しない。スラッグは非公開の規則で変わり得るため、ファイルの中身の
// sessionId だけを信じる。
func (c *Cache) Scan(projectsDir string) map[string]Meta {
	out := map[string]Meta{}
	seen := map[string]bool{}

	_ = filepath.WalkDir(projectsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".jsonl" {
			return nil //nolint:nilerr // 読めないものは飛ばす
		}
		fi, err := d.Info()
		if err != nil {
			return nil
		}
		seen[path] = true

		metas := c.cached(path, fi)
		for _, m := range metas {
			if m.SessionID == "" {
				continue
			}
			if prev, ok := out[m.SessionID]; !ok || m.LastSeen.After(prev.LastSeen) {
				out[m.SessionID] = m
			}
		}
		return nil
	})

	for path := range c.entries {
		if !seen[path] {
			delete(c.entries, path)
		}
	}
	return out
}

// cached はキャッシュが有効ならそれを、そうでなければ読み直した結果を返す。
func (c *Cache) cached(path string, fi fs.FileInfo) []Meta {
	if e, ok := c.entries[path]; ok && e.modTime.Equal(fi.ModTime()) && e.size == fi.Size() {
		return e.meta
	}
	metas := parseFile(path)
	c.entries[path] = cacheEntry{modTime: fi.ModTime(), size: fi.Size(), meta: metas}
	return metas
}

// 行を JSON デコードする前に、この語を含むかバイト列で判定して絞り込む。
// 全行をデコードすると数百 MB を舐めることになるため。
var wanted = [][]byte{
	[]byte(`"sessionId"`),
}

// line は jsonl の 1 行のうち、必要なキーだけを受ける形。
type line struct {
	SessionID string `json:"sessionId"`
	AITitle   string `json:"aiTitle"`
	Cwd       string `json:"cwd"`
	GitBranch string `json:"gitBranch"`
	Timestamp string `json:"timestamp"`
}

// parseFile は 1 ファイルを読み、含まれるセッションごとの Meta を返す。
//
// 実際には 1 ファイル 1 セッションだが、そう決め打ちしない。
func parseFile(path string) []Meta {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	acc := map[string]*Meta{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024) // 長い行があるので上限を上げる
	for sc.Scan() {
		b := sc.Bytes()
		if !containsAny(b, wanted) {
			continue
		}
		var l line
		if err := json.Unmarshal(b, &l); err != nil || l.SessionID == "" {
			continue
		}
		m, ok := acc[l.SessionID]
		if !ok {
			m = &Meta{SessionID: l.SessionID, Path: path}
			acc[l.SessionID] = m
		}
		if l.AITitle != "" {
			m.AITitle = l.AITitle
		}
		if l.Cwd != "" {
			m.Cwd = l.Cwd
		}
		if l.GitBranch != "" {
			m.GitBranch = l.GitBranch
		}
		if l.Timestamp != "" {
			if ts, err := time.Parse(time.RFC3339, l.Timestamp); err == nil && ts.After(m.LastSeen) {
				m.LastSeen = ts
			}
		}
	}

	out := make([]Meta, 0, len(acc))
	for _, m := range acc {
		out = append(out, *m)
	}
	return out
}

// containsAny は b がいずれかの語を含むかを返す。
func containsAny(b []byte, needles [][]byte) bool {
	for _, n := range needles {
		if bytes.Contains(b, n) {
			return true
		}
	}
	return false
}
