package server

import (
	"fmt"
	"html/template"
	"time"

	"github.com/pollenjp/cc-pages/internal/index"
	"github.com/pollenjp/cc-pages/internal/web"
)

// sessionRow は一覧に出す 1 セッション。テンプレートに渡す形。
type sessionRow struct {
	DirName   string
	Title     string
	Cwd       string
	GitBranch string
	PageCount int
	Bytes     int64
	LastSeen  time.Time
	Pages     []pageRow
}

// pageRow はセッション内の 1 ページ。テンプレートに渡す形。
type pageRow struct {
	DirName   string
	ID        string
	Title     string
	Summary   string
	Tags      []string
	CreatedAt time.Time
	Mode      string
	DirPath   string
}

// pageData は全テンプレートに渡す共通の形。
type pageData struct {
	Title      string
	Crumb      string
	Query      string
	ShowSearch bool
	Sessions   []sessionRow
	Session    *sessionRow
	Body       template.HTML
	PrevURL    string
	NextURL    string
}

// toRow は索引のビューをテンプレート用の形に写す。
func toRow(v index.SessionView) sessionRow {
	r := sessionRow{
		DirName: v.DirName, Title: v.Title, Cwd: v.Cwd, GitBranch: v.GitBranch,
		PageCount: v.PageCount, Bytes: v.Bytes, LastSeen: v.LastSeen,
	}
	for _, p := range v.Pages {
		r.Pages = append(r.Pages, pageRow{
			DirName: p.DirName, ID: p.ID, Title: p.Title, Summary: p.Summary,
			Tags: p.Tags, CreatedAt: p.CreatedAt, Mode: p.Mode, DirPath: p.DirPath,
		})
	}
	return r
}

// humanBytes は 2048 を "2.0 KB" のような形にする。
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGT"[exp])
}

// templateFuncs はテンプレートから使える関数。
var templateFuncs = template.FuncMap{"humanBytes": humanBytes}

// parseTemplate は layout と 1 つの body テンプレートを組み合わせて返す。
func parseTemplate(bodyFile string) *template.Template {
	return template.Must(template.New("layout").Funcs(templateFuncs).
		ParseFS(web.Templates, "layout.html", bodyFile))
}
