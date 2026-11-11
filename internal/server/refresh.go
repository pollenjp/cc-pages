package server

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/pollenjp/cc-pages/internal/index"
	"github.com/pollenjp/cc-pages/internal/store"
	"github.com/pollenjp/cc-pages/internal/transcript"
)

// Refresher はディスクを走査して索引を差し替える。
//
// ファイル監視 (inotify) は使わない。cc-pages new からの POST /_/touch と、
// 定期的な mtime 比較の 2 経路で更新する。watch の寿命管理をしなくて済む。
type Refresher struct {
	sessionsDir string
	projectsDir string
	ix          *index.Index

	mu    sync.Mutex
	prev  map[string]store.SessionEntry
	cache *transcript.Cache
	// meta は最後の走査で読んだ transcript のメタ。RefreshSession は transcript を
	// 読まずにこれを使い回す。
	meta map[string]transcript.Meta
}

// NewRefresher を作る。projectsDir が無くても動く。
func NewRefresher(sessionsDir, projectsDir string, ix *index.Index) *Refresher {
	return &Refresher{
		sessionsDir: sessionsDir,
		projectsDir: projectsDir,
		ix:          ix,
		cache:       transcript.NewCache(),
	}
}

// Refresh は前回の結果を使い回しつつ 1 回走査して索引を差し替える。
// 定期走査 (Run) 用。
func (r *Refresher) Refresh() { r.scan(false) }

// RefreshAll は前回の結果を捨てて全部読み直す。
//
// POST /_/touch — cc-pages new からの通知と、一覧の「再読み込み」ボタン — 用。
// 差分判定は mtime を見るだけなので、mtime を動かさない変更 (既存ファイルの
// 上書きなど) は原理的に取りこぼす。「定期 stat を待たずに走査できる」という
// 再読み込みボタンの約束を守るには、その経路だけは無条件に読み直す必要がある。
func (r *Refresher) RefreshAll() { r.scan(true) }

// scan は 1 回走査して索引を差し替える。
//
// 走査に失敗しても panic せず、前回の索引をそのまま残す。
func (r *Refresher) scan(full bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	prev := r.prev
	if full {
		prev = nil
	}
	entries, err := store.Scan(r.sessionsDir, prev)
	if err != nil {
		// 黙って戻ると「一覧が更新されない」以外に何の手がかりも残らない。
		log.Printf("cc-pages: 走査に失敗した (前回の索引を残す): %v", err)
		return
	}
	r.prev = entries
	r.meta = r.cache.Scan(r.projectsDir)
	r.ix.Replace(entries, r.meta)
}

// RefreshSession は 1 つの会話のディレクトリだけを読み直して索引を差し替える。
//
// POST /_/touch/{session} — cc-pages link からの知らせ — 用。全体の走査はしない。
// とくに transcript (~/.claude/projects の jsonl) は読み直さない。変わった transcript を
// 頭から読み直すのが走査で一番重く、link は turn の終わりごとに知らせてくるため。
// 題 (aiTitle) などは最後の走査のものを使い、次の定期走査で追いつく。
//
// ディレクトリが無くなっていれば、その会話を索引から外す。
func (r *Refresher) RefreshSession(dirName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.prev == nil {
		r.prev = map[string]store.SessionEntry{}
	}
	if e, ok := store.ReadSessionEntry(r.sessionsDir, dirName); ok {
		r.prev[dirName] = e
	} else {
		delete(r.prev, dirName)
	}
	r.ix.Replace(r.prev, r.meta)
}

// Run は every ごとに Refresh する。ctx が終わったら戻る。
func (r *Refresher) Run(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.Refresh()
		}
	}
}
