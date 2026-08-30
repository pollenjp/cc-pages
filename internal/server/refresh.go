package server

import (
	"context"
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

// Refresh は 1 回走査して索引を差し替える。
//
// 走査に失敗しても panic せず、前回の索引をそのまま残す。
func (r *Refresher) Refresh() {
	r.mu.Lock()
	defer r.mu.Unlock()

	entries, err := store.Scan(r.sessionsDir, r.prev)
	if err != nil {
		return
	}
	r.prev = entries
	r.ix.Replace(entries, r.cache.Scan(r.projectsDir))
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
