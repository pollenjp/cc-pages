package server

import (
	"net"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/pollenjp/cc-pages/internal/index"
	"github.com/pollenjp/cc-pages/internal/store"
)

// linkSection は会話のトップページの「リンク」の節。テンプレートに渡す形。
type linkSection struct {
	Total     int
	Groups    []linkGroup // 出どころに assistant を含む件
	Tool      []linkGroup // tool の出力だけに出た件。畳んで出す
	ToolCount int
}

// linkGroup は組 (呼ぶ側が渡した group) ごとのリンク。
type linkGroup struct {
	Name string
	Rows []linkRow
}

// linkRow はリンク 1 行。文字はどれも html/template にエスケープさせる。
type linkRow struct {
	Label    string // 表示の文字。label が無ければ URL を短くしたもの
	Href     string // http(s) と確かめた URL。空なら <a> にしない
	Title    string // URL 全体 (title 属性。省略した URL を指で確かめられるように)
	Note     string // 添え書き
	Time     string // 最初と最後の時刻
	External bool   // 外へのリンク (rel="noopener noreferrer" を付ける)
}

// shortURLRunes は添え書きに出す URL の長さ。
const shortURLRunes = 60

// buildLinks は会話のリンクを「リンク」の節の形に組む。リンクが無ければ nil で、
// 節ごと出さない (cc-pages link を使わない人の会話に毎回「まだありません」が出ないように)。
//
// 出どころに assistant を含む件 (Claude が応答の文で触れたリンク) を上に出し、
// tool の出力だけに出た件 (コマンドの出力に紛れていたリンク) は節の最後に畳む。
func (s *Server) buildLinks(v index.SessionView) *linkSection {
	if len(v.Links) == 0 {
		return nil
	}
	var main, tool []index.LinkView
	for _, l := range v.Links {
		if isToolOnly(l.Sources) {
			tool = append(tool, l)
		} else {
			main = append(main, l)
		}
	}
	return &linkSection{
		Total:     len(v.Links),
		Groups:    s.groupLinks(main, v.Started),
		Tool:      s.groupLinks(tool, v.Started),
		ToolCount: len(tool),
	}
}

// isToolOnly は tool の出力だけに出たリンクかを返す。assistant を含むものと、
// 出どころが分からないもの (手で書いた links.json) は上に出す。
func isToolOnly(srcs []string) bool {
	tool := false
	for _, s := range srcs {
		switch s {
		case store.SourceAssistant:
			return false
		case store.SourceTool:
			tool = true
		}
	}
	return tool
}

// groupLinks は組ごとにまとめる。組の並びは、組の中で一番早い最初の時刻の順
// (会話の中で話題に出た順)。組の中は、最後の時刻の新しいものを上にする。
func (s *Server) groupLinks(links []index.LinkView, started time.Time) []linkGroup {
	type acc struct {
		name  string
		first time.Time
		links []index.LinkView
	}
	byName := map[string]*acc{}
	var order []*acc
	for _, l := range links {
		name := l.Group
		if name == "" {
			name = store.DefaultLinkGroup
		}
		a, ok := byName[name]
		if !ok {
			a = &acc{name: name, first: l.FirstSeen}
			byName[name] = a
			order = append(order, a)
		}
		if l.FirstSeen.Before(a.first) {
			a.first = l.FirstSeen
		}
		a.links = append(a.links, l)
	}
	sort.SliceStable(order, func(i, j int) bool {
		if !order[i].first.Equal(order[j].first) {
			return order[i].first.Before(order[j].first)
		}
		return order[i].name < order[j].name
	})

	groups := make([]linkGroup, 0, len(order))
	for _, a := range order {
		sort.SliceStable(a.links, func(i, j int) bool {
			if !a.links[i].LastSeen.Equal(a.links[j].LastSeen) {
				return a.links[i].LastSeen.After(a.links[j].LastSeen)
			}
			return a.links[i].URL < a.links[j].URL
		})
		g := linkGroup{Name: a.name}
		for _, l := range a.links {
			g.Rows = append(g.Rows, s.linkRow(l, started))
		}
		groups = append(groups, g)
	}
	return groups
}

// linkRow は 1 件を 1 行の形にする。
//
// 添え書きは、cc-pages 自身のページなら索引にあるそのページ (会話のトップページなら
// その会話) の題。ほかは scheme を除いた URL を短くしたもの。label が無い件は、その
// URL を label の位置に出し、添え書きは空にする (同じ文字を 2 度並べない)。
func (s *Server) linkRow(l index.LinkView, started time.Time) linkRow {
	row := linkRow{Label: l.Label, Title: l.URL, Time: timeRange(l.FirstSeen, l.LastSeen, started)}
	u, ok := safeHref(l.URL)
	title, self := s.selfTitle(u)
	if ok {
		row.Href = u.String()
		row.External = !self
	}
	switch {
	case title != "":
		row.Note = title
	case l.Label != "":
		row.Note = shortURL(l.URL)
	}
	if row.Label == "" {
		row.Label = shortURL(l.URL)
	}
	return row
}

// safeHref は URL を <a> にしてよいか (http(s) の絶対 URL か) を確かめる。
//
// links.json は cc-pages link が検証してから書くが、手で書き換えられることもあるので、
// 描くときにもう一度、書くときと同じ規則で確かめる。javascript: などは html/template も
// 潰すが、潰した "#ZgotmplZ" のリンクを出すより、リンクにしない方が分かりやすい。
func safeHref(raw string) (*url.URL, bool) {
	clean, err := store.CheckLinkURL(raw)
	if err != nil {
		return nil, false
	}
	u, err := url.Parse(clean)
	if err != nil {
		return nil, false
	}
	return u, true
}

// selfTitle は u が cc-pages 自身のページ (/p/<session>/<page>/) か会話のトップページ
// (/p/<session>/) なら、その題を返す。2 つ目は u がこの serve の URL かどうか。
func (s *Server) selfTitle(u *url.URL) (string, bool) {
	if u == nil || !s.isSelfHost(u) {
		return "", false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 || parts[0] != "p" {
		return "", true
	}
	switch len(parts) {
	case 2:
		if v, ok := s.ix.Session(parts[1]); ok {
			return v.Title, true
		}
	case 3:
		if p, _, ok := s.ix.Page(parts[1], parts[2]); ok {
			return p.Title, true
		}
	}
	return "", true
}

// isSelfHost は u がこの serve を指すかを返す。cc-pages new が出す URL の host は
// localhost (BaseURL) なので、port が同じで host が loopback か bind の host ならそう見なす。
func (s *Server) isSelfHost(u *url.URL) bool {
	host, port, err := net.SplitHostPort(s.cfg.Addr)
	if err != nil {
		return false
	}
	uport := u.Port()
	if uport == "" {
		uport = "80"
		if u.Scheme == "https" {
			uport = "443"
		}
	}
	if uport != port {
		return false
	}
	switch h := u.Hostname(); h {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return host != "" && h == host
	}
}

// shortURL は scheme を除き、%XX を戻して、shortURLRunes 字で切った URL を返す。
// 表示のためだけの形 (href には使わない)。
func shortURL(raw string) string {
	s := raw
	for _, p := range []string{"https://", "http://"} {
		if len(s) >= len(p) && strings.EqualFold(s[:len(p)], p) {
			s = s[len(p):]
			break
		}
	}
	if dec, err := url.PathUnescape(s); err == nil {
		s = dec
	}
	if r := []rune(s); len(r) > shortURLRunes {
		s = string(r[:shortURLRunes-1]) + "…"
	}
	return s
}

// timeRange は最初と最後の時刻を「15:06〜16:38」の形にする。同じ分なら 1 つだけ。
// 会話の始まり (started) と違う日の時刻には日付も付け、日をまたいだ会話でも
// どの日のことか分かるようにする。started がゼロなら first の日を基準にする。
func timeRange(first, last, started time.Time) string {
	ref := started
	if ref.IsZero() {
		ref = first
	}
	f := clockOn(first, ref)
	if last.IsZero() || first.Format("2006-01-02 15:04") == last.Format("2006-01-02 15:04") {
		return f
	}
	if sameDay(first, last) {
		return f + "〜" + last.Format("15:04")
	}
	return f + "〜" + clockOn(last, ref)
}

// clockOn は ref と同じ日なら時刻だけ、違う日なら日付と時刻を返す。
func clockOn(t, ref time.Time) string {
	if sameDay(t, ref) {
		return t.Format("15:04")
	}
	return t.Format("2006-01-02 15:04")
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}
