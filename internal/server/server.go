// Package server は HTTP のルーティングと描画を担う。
package server

import (
	"html/template"
	"net/http"

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

	listTmpl *template.Template
}

// New はサーバを組み立てる。refresh は索引を作り直す関数。
func New(cfg config.Config, ix *index.Index, refresh func()) *Server {
	return &Server{
		cfg:      cfg,
		ix:       ix,
		refresh:  refresh,
		listTmpl: parseTemplate("list.html"),
	}
}

// Handler はルーティング済みの http.Handler を返す。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /_/", http.StripPrefix("/_/", http.FileServerFS(web.Static)))
	mux.HandleFunc("POST /_/touch", s.handleTouch)
	mux.HandleFunc("GET /{$}", s.handleList)
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

// render はテンプレートを流す。
func (s *Server) render(w http.ResponseWriter, t *template.Template, d pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "layout", d); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
