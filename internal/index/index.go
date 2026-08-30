// Package index はディスクの走査結果をメモリ上の索引にして、一覧と検索を提供する。
//
// データベースは持たない。タイトル・要約・タグ・元プロンプトだけなら数千ページでも
// メモリで足りる。本文全文検索が要るときは、その時にファイルを舐める二段目を足す。
package index

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pollenjp/cc-pages/internal/store"
	"github.com/pollenjp/cc-pages/internal/transcript"
)

// PageView は一覧・ページ表示に必要な 1 ページ分。
type PageView struct {
	DirName   string
	ID        string
	Title     string
	Summary   string
	Prompt    string
	Tags      []string
	CreatedAt time.Time
	Mode      string
	DirPath   string
}

// SessionView は一覧に出す 1 セッション分。
type SessionView struct {
	DirName   string
	Title     string
	Cwd       string
	GitBranch string
	PageCount int
	Bytes     int64
	LastSeen  time.Time
	Pages     []PageView
}

// Index はメモリ上の索引。Replace と読み出しは並行に呼ばれる。
type Index struct {
	mu       sync.RWMutex
	sessions []SessionView          // LastSeen の降順
	byDir    map[string]SessionView // DirName で引く
	haystack map[string]string      // DirName → 検索対象を連結した小文字の文字列
}

// New は空の索引を作る。
func New() *Index {
	return &Index{byDir: map[string]SessionView{}, haystack: map[string]string{}}
}

// Replace は索引を丸ごと入れ替える。
//
// meta は session id をキーにした jsonl 由来のメタ。nil でもよい。
func (ix *Index) Replace(entries map[string]store.SessionEntry, meta map[string]transcript.Meta) {
	views := make([]SessionView, 0, len(entries))
	byDir := make(map[string]SessionView, len(entries))
	hay := make(map[string]string, len(entries))

	for _, e := range entries {
		v := buildView(e, meta[e.Session.SessionID])
		views = append(views, v)
		byDir[v.DirName] = v
		hay[v.DirName] = buildHaystack(v)
	}
	sort.Slice(views, func(i, j int) bool { return views[i].LastSeen.After(views[j].LastSeen) })

	ix.mu.Lock()
	defer ix.mu.Unlock()
	ix.sessions, ix.byDir, ix.haystack = views, byDir, hay
}

// buildView は 1 セッション分のビューを組み立てる。
func buildView(e store.SessionEntry, m transcript.Meta) SessionView {
	v := SessionView{
		DirName:   e.DirName,
		Cwd:       e.Session.Cwd,
		GitBranch: e.Session.GitBranch,
		PageCount: len(e.Pages),
		Bytes:     e.Bytes,
		LastSeen:  e.Session.LastSeen,
	}
	for _, p := range e.Pages {
		v.Pages = append(v.Pages, PageView{
			DirName: p.DirName, ID: p.Page.ID, Title: p.Page.Title,
			Summary: p.Page.Summary, Prompt: p.Page.Prompt, Tags: p.Page.Tags,
			CreatedAt: p.Page.CreatedAt, Mode: p.Page.Mode, DirPath: p.DirPath,
		})
	}
	// jsonl 由来のメタが取れていればそちらを優先する。
	if m.AITitle != "" {
		v.Title = m.AITitle
	}
	if m.Cwd != "" {
		v.Cwd = m.Cwd
	}
	if m.GitBranch != "" {
		v.GitBranch = m.GitBranch
	}
	if m.LastSeen.After(v.LastSeen) {
		v.LastSeen = m.LastSeen
	}
	if v.Title == "" && len(v.Pages) > 0 {
		v.Title = v.Pages[0].Title
	}
	if v.Title == "" {
		v.Title = v.DirName
	}
	return v
}

// buildHaystack は検索対象を 1 本の小文字文字列に潰す。
//
// 対象はタイトル・cwd・ブランチと、各ページのタイトル・要約・タグ・元プロンプト。
// 本文 (index.html) は含めない。常時の索引を軽く保つため。
func buildHaystack(v SessionView) string {
	var b strings.Builder
	b.WriteString(v.Title)
	b.WriteByte('\n')
	b.WriteString(v.Cwd)
	b.WriteByte('\n')
	b.WriteString(v.GitBranch)
	for _, p := range v.Pages {
		b.WriteByte('\n')
		b.WriteString(p.Title)
		b.WriteByte('\n')
		b.WriteString(p.Summary)
		b.WriteByte('\n')
		b.WriteString(p.Prompt)
		for _, t := range p.Tags {
			b.WriteByte('\n')
			b.WriteString(t)
		}
	}
	return strings.ToLower(b.String())
}

// Sessions は LastSeen の降順で全セッションを返す。
func (ix *Index) Sessions() []SessionView {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	out := make([]SessionView, len(ix.sessions))
	copy(out, ix.sessions)
	return out
}

// Search は q を含むセッションを LastSeen の降順で返す。q が空なら全件。
func (ix *Index) Search(q string) []SessionView {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return ix.Sessions()
	}
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	var out []SessionView
	for _, v := range ix.sessions {
		if strings.Contains(ix.haystack[v.DirName], q) {
			out = append(out, v)
		}
	}
	return out
}

// Session は DirName でセッションを引く。
func (ix *Index) Session(dirName string) (SessionView, bool) {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	v, ok := ix.byDir[dirName]
	return v, ok
}

// Page はセッションとページのディレクトリ名でページを引く。
// 3 つ目の戻り値はページディレクトリの絶対パス。
func (ix *Index) Page(dirName, pageDirName string) (PageView, string, bool) {
	v, ok := ix.Session(dirName)
	if !ok {
		return PageView{}, "", false
	}
	for _, p := range v.Pages {
		if p.DirName == pageDirName {
			return p, p.DirPath, true
		}
	}
	return PageView{}, "", false
}
