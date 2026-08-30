package store

import (
	"os"
	"path/filepath"
	"sort"
	"time"
)

// PageEntry は走査で見つかった 1 ページ。
type PageEntry struct {
	Page    Page
	DirName string // "0007-notion-callout"
	DirPath string // 絶対パス
}

// SessionEntry は走査で見つかった 1 セッション。
type SessionEntry struct {
	Session Session
	DirName string
	DirPath string
	Pages   []PageEntry // 連番の昇順
	ModTime time.Time   // セッションディレクトリの mtime。差分判定に使う

	// PageModTime は配下のページディレクトリの mtime の最大値。差分判定に使う。
	//
	// セッションディレクトリの mtime はページの追加・削除でしか動かない。
	// skill は cc-pages new が戻った後にページディレクトリへ index.html を書くので、
	// こちらも見ないとその本文が永久に読み直されない。
	PageModTime time.Time

	// Bytes は配下のページディレクトリの合計サイズ。session.json は含まない。
	Bytes int64
}

// Scan は sessionsDir を走査する。
//
// prev を渡すと、mtime が変わっていないセッションは読み直さず prev の結果を
// 使い回す。ファイル監視 (inotify) を持たずに更新を拾うための仕組み。
// 見るのはセッションディレクトリと、その直下のページディレクトリの mtime の
// 両方。ページディレクトリを stat するだけならページ 1 枚の page.json を読み直す
// より安いので、「変わっていないセッションは読み直さない」という狙いは保てる。
// sessionsDir が存在しない場合は空の結果を返し、エラーにしない。
// 壊れた page.json は黙って飛ばす。ページ 1 枚のために一覧全体を落とさない。
func Scan(sessionsDir string, prev map[string]SessionEntry) (map[string]SessionEntry, error) {
	ents, err := os.ReadDir(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]SessionEntry{}, nil
		}
		return nil, err
	}

	out := make(map[string]SessionEntry, len(ents))
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		dirPath := filepath.Join(sessionsDir, e.Name())
		fi, err := os.Stat(dirPath)
		if err != nil {
			continue
		}
		if old, ok := prev[e.Name()]; ok && old.ModTime.Equal(fi.ModTime()) &&
			!maxPageModTime(dirPath).After(old.PageModTime) {
			out[e.Name()] = old
			continue
		}
		out[e.Name()] = readSession(dirPath, e.Name(), fi.ModTime())
	}
	return out, nil
}

// readSession は 1 セッション分を読み直す。
func readSession(dirPath, dirName string, modTime time.Time) SessionEntry {
	entry := SessionEntry{DirName: dirName, DirPath: dirPath, ModTime: modTime}
	if s, err := ReadSession(dirPath); err == nil {
		entry.Session = s
	}
	if entry.Session.Dir == "" {
		entry.Session.Dir = dirName
	}

	ents, err := os.ReadDir(dirPath)
	if err != nil {
		return entry
	}
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		// mtime は中身を読む前に採る。読んだ後に採ると、読んでいる最中に入った
		// 変更まで「見たことにして」次回の読み直しを取りこぼす。
		if fi, err := e.Info(); err == nil && fi.ModTime().After(entry.PageModTime) {
			entry.PageModTime = fi.ModTime()
		}
		pagePath := filepath.Join(dirPath, e.Name())
		p, err := ReadPage(pagePath)
		if err != nil {
			continue // 壊れたページは飛ばす
		}
		entry.Pages = append(entry.Pages, PageEntry{Page: p, DirName: e.Name(), DirPath: pagePath})
		entry.Bytes += dirBytes(pagePath)
	}
	sort.Slice(entry.Pages, func(i, j int) bool {
		return entry.Pages[i].Page.ID < entry.Pages[j].Page.ID
	})

	// session.json が無い / 古い場合の補完。ページから最終更新を作る。
	if len(entry.Pages) > 0 {
		last := entry.Pages[len(entry.Pages)-1].Page
		if entry.Session.SessionID == "" {
			entry.Session.SessionID = last.SessionID
		}
		if entry.Session.LastSeen.IsZero() {
			entry.Session.LastSeen = last.CreatedAt
		}
		if entry.Session.FirstSeen.IsZero() {
			entry.Session.FirstSeen = entry.Pages[0].Page.CreatedAt
		}
		if entry.Session.Cwd == "" {
			entry.Session.Cwd = last.Cwd
		}
		if entry.Session.GitBranch == "" {
			entry.Session.GitBranch = last.GitBranch
		}
	}
	return entry
}

// maxPageModTime はセッション直下のページディレクトリの mtime の最大値を返す。
// 読めなければゼロ値を返し、使い回しを妨げない。
func maxPageModTime(dir string) time.Time {
	var newest time.Time
	ents, err := os.ReadDir(dir)
	if err != nil {
		return newest
	}
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		if fi, err := e.Info(); err == nil && fi.ModTime().After(newest) {
			newest = fi.ModTime()
		}
	}
	return newest
}

// dirBytes はディレクトリ配下の合計サイズを返す。読めないものは 0 扱い。
func dirBytes(dir string) int64 {
	var total int64
	_ = filepath.WalkDir(dir, func(_ string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if fi, err := d.Info(); err == nil {
			total += fi.Size()
		}
		return nil
	})
	return total
}
