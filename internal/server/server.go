// Package server は HTTP のルーティングと描画を担う。
package server

import (
	"bytes"
	"html/template"
	"net/http"
	"net/url"
	"path/filepath"

	"github.com/pollenjp/cc-pages/internal/config"
	"github.com/pollenjp/cc-pages/internal/index"
	"github.com/pollenjp/cc-pages/internal/web"
)

// CSP は全ページに付ける Content-Security-Policy。
//
// フラグメントの <style> のために style-src だけインラインを開ける。
// script-src は開けないので、フラグメントに <script> は書けない。
const CSP = "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:"

// Server は cc-pages serve の HTTP 側。
type Server struct {
	cfg     config.Config
	ix      *index.Index
	refresh func()

	listTmpl    *template.Template
	sessionTmpl *template.Template
	pageTmpl    *template.Template
}

// New はサーバを組み立てる。
//
// refresh は POST /_/touch から呼ぶ、索引を強制的に全部読み直す関数
// (Refresher.RefreshAll)。差分走査を渡すと「再読み込み」ボタンが
// 定期走査と同じ取りこぼしを共有してしまい、押しても直らなくなる。
func New(cfg config.Config, ix *index.Index, refresh func()) *Server {
	return &Server{
		cfg:         cfg,
		ix:          ix,
		refresh:     refresh,
		listTmpl:    parseTemplate("list.html"),
		sessionTmpl: parseTemplate("session.html"),
		pageTmpl:    parseTemplate("page.html"),
	}
}

// Handler はルーティング済みの http.Handler を返す。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /_/", http.StripPrefix("/_/", http.FileServerFS(web.Static)))
	mux.HandleFunc("POST /_/touch", s.handleTouch)
	mux.HandleFunc("GET /{$}", s.handleList)
	mux.HandleFunc("GET /p/{session}/{$}", s.handleSession)
	mux.HandleFunc("GET /p/{session}/{page}/assets/{path...}", s.handleAsset)
	mux.HandleFunc("GET /p/{session}/{page}/{$}", s.handlePage)
	mux.HandleFunc("GET /p/{session}/{page}", s.handlePageLegacy)
	return s.withHeaders(mux)
}

// withHeaders は全レスポンスに CSP を付ける。
func (s *Server) withHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", CSP)
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

// handleList はセッション一覧を出す。
func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	views := s.ix.Search(q)
	rows := make([]sessionRow, 0, len(views))
	for _, v := range views {
		rows = append(rows, toRow(v))
	}
	s.render(w, s.listTmpl, pageData{
		Title: "cc-pages", Query: q, ShowSearch: true, Sessions: rows,
	})
}

// handleTouch は索引を作り直す。cc-pages new と「再読み込み」ボタンから呼ばれる。
//
// Accept ヘッダが無い呼び出し (cc-pages new) には 204 を返し、ブラウザからの
// フォーム送信には一覧へのリダイレクトを返す。
func (s *Server) handleTouch(w http.ResponseWriter, r *http.Request) {
	s.refresh()
	if r.Header.Get("Accept") == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// handleSession はセッション内のページ一覧を出す。
func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	v, ok := s.ix.Session(r.PathValue("session"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	row := toRow(v)
	s.render(w, s.sessionTmpl, pageData{
		Title: row.Title, Crumb: row.Title, Session: &row,
	})
}

// handlePage は 1 ページを chrome で包んで出す。
func (s *Server) handlePage(w http.ResponseWriter, r *http.Request) {
	sessionDir, pageDir := r.PathValue("session"), r.PathValue("page")
	v, ok := s.ix.Session(sessionDir)
	if !ok {
		http.NotFound(w, r)
		return
	}
	p, path, ok := s.ix.Page(sessionDir, pageDir)
	if !ok {
		http.NotFound(w, r)
		return
	}
	row := toRow(v)
	d := pageData{
		Title: p.Title,
		Crumb: p.Title,
		Body:  readFragment(path),
		// パンくずのセッション部分はページ一覧へのリンクにする。ページからは
		// ここを通らないと同じセッションの他のページへ戻れない。
		SessionTitle: row.Title,
		SessionURL:   sessionURL(sessionDir),
	}
	// 前後ページ。Pages は連番の昇順に並んでいる。
	for i, q := range row.Pages {
		if q.DirName != pageDir {
			continue
		}
		if i > 0 {
			d.PrevURL = pageURL(sessionDir, row.Pages[i-1].DirName)
		}
		if i+1 < len(row.Pages) {
			d.NextURL = pageURL(sessionDir, row.Pages[i+1].DirName)
		}
	}
	s.render(w, s.pageTmpl, d)
}

// handlePageLegacy は末尾スラッシュの無い旧形式の URL を正規形へ送る。
//
// cc-pages new は既にこの形の URL を出力しており、設計書も例示で固定している。
// ServeMux 自身も「末尾スラッシュ付きだけが登録されている」場合に 307 を返す
// 機能を持つが、それは存在しないページにも無条件で掛かり、「知らない URL は
// 404」という保証を崩す。だからここで明示的に登録し、引けるページに限って
// 恒久リダイレクト (301) する。
func (s *Server) handlePageLegacy(w http.ResponseWriter, r *http.Request) {
	sessionDir, pageDir := r.PathValue("session"), r.PathValue("page")
	if _, _, ok := s.ix.Page(sessionDir, pageDir); !ok {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, pageURL(sessionDir, pageDir), http.StatusMovedPermanently)
}

// pageURL はページへのパスを組み立てる。日本語のディレクトリ名も通る。
//
// 末尾のスラッシュは飾りではない。これがないと、フラグメントが書く相対参照
// (assets/x.png) が 1 階層上に解決されて 404 になり、同一文書内のアンカー
// (#toc) も「別の文書の断片」として扱われてページ内移動にならない。
// <base> で誤魔化すと後者が直らないので、URL 自体を正規形にする。
func pageURL(sessionDir, pageDir string) string {
	return sessionURL(sessionDir) + url.PathEscape(pageDir) + "/"
}

// sessionURL はセッション内ページ一覧へのパスを組み立てる。
func sessionURL(sessionDir string) string {
	return "/p/" + url.PathEscape(sessionDir) + "/"
}

// handleAsset はページディレクトリの assets/ を配信する。
//
// http.Dir 越しに配信するので、assets/ の外には出られない。
func (s *Server) handleAsset(w http.ResponseWriter, r *http.Request) {
	_, path, ok := s.ix.Page(r.PathValue("session"), r.PathValue("page"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	http.StripPrefix(
		"/p/"+r.PathValue("session")+"/"+r.PathValue("page")+"/assets/",
		http.FileServer(http.Dir(filepath.Join(path, "assets"))),
	).ServeHTTP(w, r)
}

// render はテンプレートをいったんバッファに組み立ててから w に書く。
//
// w に直接 ExecuteTemplate すると、Content-Type を立てた後で描画が途中失敗した
// 場合に 200 と部分的な HTML が既に書き出されてしまい、続く http.Error が効かず
// 壊れた出力に追記されるだけになる (Task 7 レビュー指摘)。バッファに通すことで、
// 失敗時は w に何も書かずに 500 を返せる。
func (s *Server) render(w http.ResponseWriter, t *template.Template, d pageData) {
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", d); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w) // クライアント切断など書き込み側のエラーはもう手当てしようがない
}
