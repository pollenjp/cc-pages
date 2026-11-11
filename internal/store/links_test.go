package store

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// linkNow は links のテストの「今」。fixedTime と別にしておくと、at を省いた件に
// どちらの時刻が入ったのかを取り違えない。
func linkNow() time.Time {
	return time.Date(2026, 11, 11, 16, 38, 0, 0, time.FixedZone("JST", 9*3600))
}

func TestLinksRoundTrip(t *testing.T) {
	dir := t.TempDir()
	first := time.Date(2026, 11, 11, 15, 6, 0, 0, time.FixedZone("JST", 9*3600))
	in := Links{
		Schema:    SchemaVersion,
		UpdatedAt: linkNow(),
		Links: []Link{
			{
				URL: "https://app.notion.com/p/abc", Label: "TKT-12", Group: "チケット",
				Sources: []string{SourceAssistant, SourceTool}, FirstSeen: first, LastSeen: linkNow(),
			},
			{URL: "https://github.com/o/r/pull/11", Sources: []string{SourceTool}, FirstSeen: first, LastSeen: first},
		},
	}
	if err := writeLinks(dir, in); err != nil {
		t.Fatalf("writeLinks() error = %v", err)
	}
	got, err := ReadLinks(dir)
	if err != nil {
		t.Fatalf("ReadLinks() error = %v", err)
	}
	if got.Schema != SchemaVersion || !got.UpdatedAt.Equal(linkNow()) {
		t.Errorf("Schema = %d, UpdatedAt = %v", got.Schema, got.UpdatedAt)
	}
	if len(got.Links) != 2 {
		t.Fatalf("len(Links) = %d, want 2", len(got.Links))
	}
	a := got.Links[0]
	if a.URL != "https://app.notion.com/p/abc" || a.Label != "TKT-12" || a.Group != "チケット" ||
		!reflect.DeepEqual(a.Sources, []string{"assistant", "tool"}) ||
		!a.FirstSeen.Equal(first) || !a.LastSeen.Equal(linkNow()) {
		t.Errorf("Links[0] = %+v", a)
	}
	b := got.Links[1]
	if b.Label != "" || b.Group != "" || !reflect.DeepEqual(b.Sources, []string{"tool"}) {
		t.Errorf("Links[1] = %+v", b)
	}

	fi, err := os.Stat(filepath.Join(dir, LinksFileName))
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("links.json のパーミッション = %o, want 600", perm)
	}
	// 一時ファイルに書いて rename するので、書き終えたら links.json だけが残る
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 1 || ents[0].Name() != LinksFileName {
		names := []string{}
		for _, e := range ents {
			names = append(names, e.Name())
		}
		t.Errorf("dir に links.json 以外が残っている: %q", names)
	}
}

func TestReadLinksMissing(t *testing.T) {
	if _, err := ReadLinks(t.TempDir()); !os.IsNotExist(err) {
		t.Errorf("無いときのエラー = %v, want IsNotExist", err)
	}
}

func TestNewLinkInputDefaults(t *testing.T) {
	got, err := NewLinkInput("  https://github.com/o/r/pull/12 ", "", "", "", "", linkNow())
	if err != nil {
		t.Fatalf("NewLinkInput() error = %v", err)
	}
	want := LinkInput{URL: "https://github.com/o/r/pull/12", Source: "assistant"}
	if got.URL != want.URL || got.Label != "" || got.Group != "" || got.Source != want.Source {
		t.Errorf("got = %+v, want %+v", got, want)
	}
	if !got.At.Equal(linkNow()) {
		t.Errorf("At = %v, want 今 (%v)", got.At, linkNow())
	}

	// 「その他」は組の既定の名前、URL と同じ label は URL を出すのと同じ。どちらも
	// 「空」として持つと、後から来た件の値で埋められる (まとめ方の規則)。
	got, err = NewLinkInput("https://ex.com/a", "https://ex.com/a", "その他", "tool", "", linkNow())
	if err != nil {
		t.Fatal(err)
	}
	if got.Label != "" || got.Group != "" || got.Source != "tool" {
		t.Errorf("既定と同じ値が空に寄っていない: %+v", got)
	}
}

func TestNewLinkInputParsesAt(t *testing.T) {
	got, err := NewLinkInput("https://ex.com/", "", "", "", "2026-11-11T15:06:01.234Z", linkNow())
	if err != nil {
		t.Fatalf("NewLinkInput() error = %v", err)
	}
	if want := time.Date(2026, 11, 11, 15, 6, 1, 234e6, time.UTC); !got.At.Equal(want) {
		t.Errorf("At = %v, want %v", got.At, want)
	}
}

func TestNewLinkInputRejects(t *testing.T) {
	long := "https://ex.com/" + strings.Repeat("a", 2049-15)
	cases := []struct {
		name, url, source, at string
	}{
		{"空", "", "", ""},
		{"空白だけ", "   ", "", ""},
		{"ftp", "ftp://x/y", "", ""},
		{"javascript", "javascript:alert(1)", "", ""},
		{"相対", "/p/x/", "", ""},
		{"host が無い", "https://", "", ""},
		{"opaque", "http:foo", "", ""},
		{"host に空白", "https://a b.com/", "", ""},
		{"タブ", "https://ex.com/a\tb", "", ""},
		{"NUL", "https://ex.com/\x00", "", ""},
		{"壊れた UTF-8", "https://ex.com/\xff", "", ""},
		{"2049 字", long, "", ""},
		{"知らない source", "https://ex.com/", "web", ""},
		{"読めない at", "https://ex.com/", "", "昨日"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := NewLinkInput(c.url, "", "", c.source, c.at, linkNow()); err == nil {
				t.Errorf("NewLinkInput(%q, source=%q, at=%q) が通ってしまった", c.url, c.source, c.at)
			}
		})
	}

	exact := "https://ex.com/" + strings.Repeat("a", 2048-15)
	if _, err := NewLinkInput(exact, "", "", "", "", linkNow()); err != nil {
		t.Errorf("2048 字ちょうどの URL が拒まれた: %v", err)
	}
}

// TestNewLinkInputKeepsJapaneseURL は cc-pages 自身のページの URL を受け付けることを
// 確かめる。cc-pages new はパスの日本語を %XX に潰さずに出すので、制御文字を弾く検査が
// 非 ASCII まで弾くと、cc-page へのリンクがすべて捨てられる。
func TestNewLinkInputKeepsJapaneseURL(t *testing.T) {
	for _, u := range []string{
		"http://localhost:7777/p/20261111-0a1b2c3d/0001-調べたこと/",
		"http://localhost:7777/p/20261111-0a1b2c3d/0001-%E8%AA%BF%E3%81%B9%E3%81%9F%E3%81%93%E3%81%A8/",
	} {
		got, err := NewLinkInput(u, "📄0001", "cc-page", "", "", linkNow())
		if err != nil {
			t.Errorf("NewLinkInput(%q) error = %v", u, err)
			continue
		}
		if got.URL != u {
			t.Errorf("URL が書き換わった: %q, want %q", got.URL, u)
		}
	}
}

func TestNewLinkInputCleansText(t *testing.T) {
	got, err := NewLinkInput("https://ex.com/", "a\nb\t c\x00", " PR\r\n", "", "", linkNow())
	if err != nil {
		t.Fatal(err)
	}
	if got.Label != "a b c" {
		t.Errorf("Label = %q, want %q", got.Label, "a b c")
	}
	if got.Group != "PR" {
		t.Errorf("Group = %q, want %q", got.Group, "PR")
	}

	got, err = NewLinkInput("https://ex.com/", strings.Repeat("あ", 201), strings.Repeat("い", 51), "", "", linkNow())
	if err != nil {
		t.Fatal(err)
	}
	if want := strings.Repeat("あ", 199) + "…"; got.Label != want {
		t.Errorf("201 字の label = %q (%d 字), want 200 字で末尾が …", got.Label, len([]rune(got.Label)))
	}
	if want := strings.Repeat("い", 49) + "…"; got.Group != want {
		t.Errorf("51 字の group = %q (%d 字), want 50 字で末尾が …", got.Group, len([]rune(got.Group)))
	}
}

func TestParseLinkLines(t *testing.T) {
	in := strings.Join([]string{
		`{"url": "https://github.com/o/r/pull/12", "label": "r#12", "group": "PR", "source": "assistant", "at": "2026-11-11T15:06:01.234Z"}`,
		``,
		`{壊れた`,
		`[1,2]`,
		`{"label":"x"}`,
		`{"url":"https://a/","source":"tool","at":"2026-11-11T15:06:00+09:00"}`,
	}, "\n") + "\n"
	var warns []string
	items, skipped := ParseLinkLines(strings.NewReader(in), linkNow(), func(s string) { warns = append(warns, s) })

	if len(items) != 2 {
		t.Fatalf("items = %d 件, want 2: %+v", len(items), items)
	}
	if items[0].URL != "https://github.com/o/r/pull/12" || items[0].Label != "r#12" || items[0].Group != "PR" ||
		items[0].Source != "assistant" || !items[0].At.Equal(time.Date(2026, 11, 11, 15, 6, 1, 234e6, time.UTC)) {
		t.Errorf("items[0] = %+v", items[0])
	}
	if items[1].URL != "https://a/" || items[1].Source != "tool" ||
		!items[1].At.Equal(time.Date(2026, 11, 11, 6, 6, 0, 0, time.UTC)) {
		t.Errorf("items[1] = %+v", items[1])
	}
	// 空行は捨てた件に数えない (JSON Lines の末尾の改行や空行はふつうにある)
	if skipped != 3 {
		t.Errorf("skipped = %d, want 3", skipped)
	}
	if len(warns) != 3 {
		t.Fatalf("warn = %d 回, want 3: %q", len(warns), warns)
	}
	for i, n := range []string{"3 行目", "4 行目", "5 行目"} {
		if !strings.Contains(warns[i], n) {
			t.Errorf("warn[%d] = %q, want %q を含む", i, warns[i], n)
		}
	}
	if !strings.Contains(warns[2], "url") {
		t.Errorf("url の無い行の理由に url が出ていない: %q", warns[2])
	}
}

// TestParseLinkLinesBounds は、呼ぶ側の不具合で来る巨大な 1 行や終わらない入力に
// 引きずられないことを確かめる。上限は小さく渡して試す。
func TestParseLinkLinesBounds(t *testing.T) {
	ok := `{"url":"https://a/"}` // 20 バイト
	// 65 バイトの行は読み取りの枠 (64+2) に収まり、67 バイトの行は枠からあふれる。
	// どちらの道でも「長すぎる」で捨てる。
	long65 := `{"url":"https://a/` + strings.Repeat("x", 45) + `"}`
	long67 := `{"url":"https://a/` + strings.Repeat("x", 47) + `"}`
	if len(long65) != 65 || len(long67) != 67 {
		t.Fatalf("前提が崩れた: %d, %d", len(long65), len(long67))
	}

	t.Run("長すぎる行は捨てて続きを読む", func(t *testing.T) {
		var warns []string
		items, skipped := parseLinkLines(strings.NewReader(long65+"\n"+long67+"\n"+ok+"\n"), linkNow(),
			func(s string) { warns = append(warns, s) }, 64, 256)
		if len(items) != 1 || skipped != 2 {
			t.Errorf("items = %d, skipped = %d, want 1, 2", len(items), skipped)
		}
		if len(warns) != 2 || !strings.Contains(warns[0], "1 行目") || !strings.Contains(warns[1], "2 行目") {
			t.Errorf("warn = %q, want 1 行目と 2 行目", warns)
		}
	})

	t.Run("入力全体の上限で読むのをやめる", func(t *testing.T) {
		in := strings.Repeat(ok+"\n", 20) // 420 バイト
		var warns []string
		items, _ := parseLinkLines(strings.NewReader(in), linkNow(),
			func(s string) { warns = append(warns, s) }, 64, 256)
		// 256 バイトに収まるのは 12 行 (252 バイト)。13 行目は途中で切れる
		if len(items) != 12 {
			t.Errorf("items = %d, want 12", len(items))
		}
		if len(warns) != 1 {
			t.Errorf("warn = %q, want 上限の 1 回", warns)
		}
	})
}

const linkBase = "http://localhost:7777"

// jst は時刻を hh:mm で書くための helper。日付は 2026-11-11 に固定する。
func jst(h, m int) time.Time {
	return time.Date(2026, 11, 11, h, m, 0, 0, time.FixedZone("JST", 9*3600))
}

// li は検証済みの 1 件を組み立てる (AddLinks は NewLinkInput を通った件を受け取る)。
func li(u, label, group, source string, at time.Time) LinkInput {
	return LinkInput{URL: u, Label: label, Group: group, Source: source, At: at}
}

// collect は warn の文を貯める。
func collect(warns *[]string) func(string) {
	return func(s string) { *warns = append(*warns, s) }
}

func readLinksAt(t *testing.T, root, dir string) Links {
	t.Helper()
	l, err := ReadLinks(filepath.Join(root, "sessions", dir))
	if err != nil {
		t.Fatalf("ReadLinks(%s) error = %v", dir, err)
	}
	return l
}

func TestAddLinksCreatesSession(t *testing.T) {
	root := t.TempDir()
	var warns []string
	res, err := AddLinks(root, linkBase, fixedTime(), "409bfd08-aaaa", []LinkInput{
		li("https://app.notion.com/p/abc", "TKT-12", "チケット", SourceAssistant, jst(15, 6)),
		li("https://github.com/o/r/pull/11", "r#11", "PR", SourceTool, jst(16, 20)),
	}, collect(&warns))
	if err != nil {
		t.Fatalf("AddLinks() error = %v", err)
	}
	want := LinkResult{URL: linkBase + "/p/20260829-409bfd08/", Added: 2, Dir: "20260829-409bfd08", Changed: true}
	if res != want {
		t.Errorf("res = %+v, want %+v", res, want)
	}
	if len(warns) != 0 {
		t.Errorf("warn = %q", warns)
	}
	l := readLinksAt(t, root, "20260829-409bfd08")
	if l.Schema != SchemaVersion || !l.UpdatedAt.Equal(fixedTime()) || len(l.Links) != 2 {
		t.Fatalf("links.json = %+v", l)
	}
	if got := l.Links[1]; got.URL != "https://github.com/o/r/pull/11" || got.Label != "r#11" || got.Group != "PR" ||
		!reflect.DeepEqual(got.Sources, []string{"tool"}) || !got.FirstSeen.Equal(jst(16, 20)) || !got.LastSeen.Equal(jst(16, 20)) {
		t.Errorf("Links[1] = %+v", got)
	}
	// ページより先に打ったので、session.json の骨もここで書かれる (status と同じ規則)
	sess, err := ReadSession(filepath.Join(root, "sessions", "20260829-409bfd08"))
	if err != nil {
		t.Fatalf("session.json が無い: %v", err)
	}
	if sess.SessionID != "409bfd08-aaaa" || sess.Dir != "20260829-409bfd08" {
		t.Errorf("Session = %+v", sess)
	}
}

func TestAddLinksMerges(t *testing.T) {
	root := t.TempDir()
	const u = "https://github.com/o/r/pull/12"
	add := func(items ...LinkInput) LinkResult {
		t.Helper()
		res, err := AddLinks(root, linkBase, fixedTime(), "409bfd08-aaaa", items, func(string) {})
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	add(li(u, "", "", SourceAssistant, jst(10, 0)))
	res := add(
		li(u, "r#12", "PR", SourceTool, jst(9, 0)),
		li(u, "r#12", "PR", SourceTool, jst(11, 0)),
	)
	if res.Added != 0 || res.Updated != 1 || !res.Changed {
		t.Errorf("2 回目の res = %+v, want added 0 / updated 1", res)
	}
	l := readLinksAt(t, root, res.Dir)
	if len(l.Links) != 1 {
		t.Fatalf("同じ URL が 1 件にまとまっていない: %d 件", len(l.Links))
	}
	got := l.Links[0]
	if !got.FirstSeen.Equal(jst(9, 0)) || !got.LastSeen.Equal(jst(11, 0)) {
		t.Errorf("時刻 = %v 〜 %v, want 09:00 〜 11:00 (最初は小さい方・最後は大きい方)", got.FirstSeen, got.LastSeen)
	}
	if !reflect.DeepEqual(got.Sources, []string{"assistant", "tool"}) {
		t.Errorf("Sources = %q, want 和", got.Sources)
	}
	// 前は空 (URL を出していた・「その他」だった) なので、後から来た値で埋まる
	if got.Label != "r#12" || got.Group != "PR" {
		t.Errorf("Label/Group = %q/%q, want 埋まる", got.Label, got.Group)
	}

	// 最初に入った label・group を残す (turn ごとに表示が揺れないように)
	add(li(u, "別", "別の組", SourceTool, jst(11, 0)))
	got = readLinksAt(t, root, res.Dir).Links[0]
	if got.Label != "r#12" || got.Group != "PR" {
		t.Errorf("Label/Group = %q/%q, want 最初の r#12/PR のまま", got.Label, got.Group)
	}
}

func TestAddLinksSameURLInOneCall(t *testing.T) {
	root := t.TempDir()
	const u = "https://github.com/o/r/pull/12"
	res, err := AddLinks(root, linkBase, fixedTime(), "409bfd08-aaaa", []LinkInput{
		li(u, "r#12", "PR", SourceAssistant, jst(10, 0)),
		li(u, "r#12", "PR", SourceAssistant, jst(9, 0)),
		li(u, "r#12", "PR", SourceAssistant, jst(12, 0)),
	}, func(string) {})
	if err != nil {
		t.Fatal(err)
	}
	if res.Added != 1 || res.Updated != 0 {
		t.Errorf("res = %+v, want added 1 / updated 0 (この呼び出しで足した URL は「前からあった」に数えない)", res)
	}
	got := readLinksAt(t, root, res.Dir).Links
	if len(got) != 1 || !got[0].FirstSeen.Equal(jst(9, 0)) || !got[0].LastSeen.Equal(jst(12, 0)) {
		t.Errorf("Links = %+v", got)
	}
}

// TestAddLinksIsIdempotent は、呼ぶ側が turn ごとに同じリンクの組を渡し直しても、
// 2 回目からは何も書かないことを確かめる。書くと links.json と session.json の mtime が
// 動き、serve の走査が毎回その会話を読み直す。
func TestAddLinksIsIdempotent(t *testing.T) {
	root := t.TempDir()
	items := []LinkInput{
		li("https://app.notion.com/p/abc", "TKT-12", "チケット", SourceAssistant, jst(15, 6)),
		li("https://github.com/o/r/pull/11", "r#11", "PR", SourceTool, jst(16, 20)),
	}
	first, err := AddLinks(root, linkBase, fixedTime(), "409bfd08-aaaa", items, func(string) {})
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "sessions", first.Dir)
	old := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	before := map[string][]byte{}
	for _, name := range []string{LinksFileName, SessionFileName} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		before[name] = b
		if err := os.Chtimes(filepath.Join(dir, name), old, old); err != nil {
			t.Fatal(err)
		}
	}

	second, err := AddLinks(root, linkBase, fixedTime().Add(time.Hour), "409bfd08-aaaa", items, func(string) {})
	if err != nil {
		t.Fatal(err)
	}
	if second.Added != 0 || second.Updated != 0 || second.Changed {
		t.Errorf("2 回目の res = %+v, want 何も変わらない", second)
	}
	if second.URL != first.URL {
		t.Errorf("URL = %q, want %q", second.URL, first.URL)
	}
	for _, name := range []string{LinksFileName, SessionFileName} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != string(before[name]) {
			t.Errorf("%s の中身が変わった", name)
		}
		fi, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if !fi.ModTime().Equal(old) {
			t.Errorf("%s を書き直した (mtime = %v)", name, fi.ModTime())
		}
	}
}

func TestAddLinksCap(t *testing.T) {
	root := t.TempDir()
	sessionsDir := filepath.Join(root, "sessions")
	mkSessionDir(t, sessionsDir, "20260829-409bfd08", "409bfd08-aaaa")
	full := Links{Schema: SchemaVersion, UpdatedAt: jst(9, 0)}
	for i := 0; i < MaxLinks; i++ {
		full.Links = append(full.Links, Link{
			URL: fmt.Sprintf("https://ex.com/%d", i), Sources: []string{SourceAssistant},
			FirstSeen: jst(9, 0), LastSeen: jst(9, 0),
		})
	}
	if err := writeLinks(filepath.Join(sessionsDir, "20260829-409bfd08"), full); err != nil {
		t.Fatal(err)
	}

	var warns []string
	res, err := AddLinks(root, linkBase, fixedTime(), "409bfd08-aaaa", []LinkInput{
		li("https://ex.com/new", "", "", SourceAssistant, jst(10, 0)),
		li("https://ex.com/0", "", "", SourceAssistant, jst(10, 0)),
	}, collect(&warns))
	if err != nil {
		t.Fatal(err)
	}
	if res.Skipped != 1 || res.Updated != 1 || res.Added != 0 {
		t.Errorf("res = %+v, want skipped 1 / updated 1 (上限でも前からある URL はまとめる)", res)
	}
	l := readLinksAt(t, root, "20260829-409bfd08")
	if len(l.Links) != MaxLinks {
		t.Errorf("件数 = %d, want %d", len(l.Links), MaxLinks)
	}
	if !l.Links[0].LastSeen.Equal(jst(10, 0)) {
		t.Errorf("前からある URL がまとまっていない: %+v", l.Links[0])
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "5000") || !strings.Contains(warns[0], "https://ex.com/new") {
		t.Errorf("warn = %q, want 上限の旨と捨てた URL", warns)
	}
}

// TestAddLinksWithoutItemsDoesNotTouchDisk は、渡された件が 0 のとき (空の入力・全部
// 捨てた) に会話のディレクトリを作らないことを確かめる。呼ぶ側はリンクの無い会話でも
// 打つので、ここで作ると一覧が空の会話で埋まる。
func TestAddLinksWithoutItemsDoesNotTouchDisk(t *testing.T) {
	root := t.TempDir()
	res, err := AddLinks(root, linkBase, fixedTime(), "409bfd08-aaaa", nil, func(string) {})
	if err != nil {
		t.Fatal(err)
	}
	if want := (LinkResult{URL: linkBase + "/s/409bfd08-aaaa/"}); res != want {
		t.Errorf("res = %+v, want %+v", res, want)
	}
	if ents, _ := os.ReadDir(root); len(ents) != 0 {
		t.Errorf("0 件なのに root に作った: %v", ents)
	}

	// 会話のディレクトリが既にあれば、そのトップページを返す。何も書かない。
	page, err := CreatePage(root, linkBase, fixedTime(), NewPageInput{SessionID: "409bfd08-aaaa", Title: "T"})
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(page.Dir)
	res, err = AddLinks(root, linkBase, fixedTime(), "409bfd08-aaaa", nil, func(string) {})
	if err != nil {
		t.Fatal(err)
	}
	if want := linkBase + "/p/" + filepath.Base(dir) + "/"; res.URL != want || res.Changed {
		t.Errorf("res = %+v, want url %q", res, want)
	}
	if _, err := os.Stat(filepath.Join(dir, LinksFileName)); !os.IsNotExist(err) {
		t.Errorf("0 件なのに links.json を書いた: %v", err)
	}
}

func TestAddLinksRejectsBadSession(t *testing.T) {
	for _, sid := range []string{"", "../x", "a b"} {
		root := t.TempDir()
		_, err := AddLinks(root, linkBase, fixedTime(), sid, []LinkInput{
			li("https://ex.com/", "", "", SourceAssistant, jst(10, 0)),
		}, func(string) {})
		if err == nil {
			t.Errorf("session id %q が通ってしまった", sid)
		}
		if ents, _ := os.ReadDir(root); len(ents) != 0 {
			t.Errorf("session id %q を拒んだのに root に作った: %v", sid, ents)
		}
	}
}

func TestAddLinksReusesSessionDirOfPages(t *testing.T) {
	root := t.TempDir()
	page, err := CreatePage(root, linkBase, fixedTime(), NewPageInput{SessionID: "409bfd08-aaaa", Title: "T"})
	if err != nil {
		t.Fatal(err)
	}
	// 日付をまたいでも、同じ session id なら同じディレクトリに書く
	res, err := AddLinks(root, linkBase, fixedTime().Add(24*time.Hour), "409bfd08-aaaa", []LinkInput{
		li("https://ex.com/", "", "", SourceAssistant, jst(10, 0)),
	}, func(string) {})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Base(filepath.Dir(page.Dir)); res.Dir != want {
		t.Errorf("Dir = %q, want ページと同じ %q", res.Dir, want)
	}
	if ents, _ := os.ReadDir(filepath.Join(root, "sessions")); len(ents) != 1 {
		t.Errorf("会話のディレクトリが割れた: %d 個", len(ents))
	}
}

// TestAddLinksConcurrent は、同じ会話に cc-pages link が同時に来ても書き込みが
// 消えないことを確かめる。まだディレクトリの無い会話への最初の書き込みが重なる
// 場合も含める (重なると、会話のディレクトリを 2 つ作りうる)。
func TestAddLinksConcurrent(t *testing.T) {
	root := t.TempDir()
	const workers, each = 20, 5
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			var items []LinkInput
			for i := 0; i < each; i++ {
				items = append(items, li(fmt.Sprintf("https://ex.com/%d/%d", w, i), "", "", SourceAssistant, jst(10, 0)))
			}
			if _, err := AddLinks(root, linkBase, fixedTime(), "409bfd08-aaaa", items, func(string) {}); err != nil {
				errs <- err
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("AddLinks() error = %v", err)
	}
	ents, err := os.ReadDir(filepath.Join(root, "sessions"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 1 {
		names := []string{}
		for _, e := range ents {
			names = append(names, e.Name())
		}
		t.Fatalf("会話のディレクトリが割れた: %q", names)
	}
	if got := len(readLinksAt(t, root, ents[0].Name()).Links); got != workers*each {
		t.Errorf("links.json の件数 = %d, want %d (重なった書き込みが消えた)", got, workers*each)
	}
}

// TestAddLinksSetsBrokenFileAside は、手で壊した links.json があっても cc-pages link が
// 止まらないことを確かめる。呼ぶ側は失敗を黙って捨てるので、ここで失敗し続けると
// 誰も気づかないままリンクを取り逃す。壊れた中身は消さずに links.json.broken に残す。
// JSON の文法の崩れだけでなく、時刻の文字列の崩れ (time の解析の誤り) も同じに扱う。
func TestAddLinksSetsBrokenFileAside(t *testing.T) {
	cases := []struct{ name, body string }{
		{"JSON の文法が崩れた", "{壊れた"},
		{"時刻が空", `{"schema":1,"links":[{"url":"https://a/","sources":["assistant"],"first_seen":"","last_seen":""}]}`},
		{"時刻の書式が違う", `{"schema":1,"links":[{"url":"https://a/","sources":["assistant"],"first_seen":"2026-11-11 15:06","last_seen":"2026-11-11 15:06"}]}`},
		{"型が違う", `{"schema":1,"links":"一覧ではない"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			sessionsDir := filepath.Join(root, "sessions")
			mkSessionDir(t, sessionsDir, "20260829-409bfd08", "409bfd08-aaaa")
			dir := filepath.Join(sessionsDir, "20260829-409bfd08")
			if err := os.WriteFile(filepath.Join(dir, LinksFileName), []byte(c.body), 0o600); err != nil {
				t.Fatal(err)
			}

			var warns []string
			res, err := AddLinks(root, linkBase, fixedTime(), "409bfd08-aaaa", []LinkInput{
				li("https://ex.com/", "", "", SourceAssistant, jst(10, 0)),
			}, collect(&warns))
			if err != nil {
				t.Fatalf("壊れた links.json で止まった: %v", err)
			}
			if res.Added != 1 {
				t.Errorf("res = %+v", res)
			}
			b, err := os.ReadFile(filepath.Join(dir, LinksFileName+".broken"))
			if err != nil || string(b) != c.body {
				t.Errorf("links.json.broken = %q, %v, want 元の中身", b, err)
			}
			if got := readLinksAt(t, root, "20260829-409bfd08").Links; len(got) != 1 || got[0].URL != "https://ex.com/" {
				t.Errorf("Links = %+v", got)
			}
			if len(warns) != 1 || !strings.Contains(warns[0], ".broken") {
				t.Errorf("warn = %q, want 退けた先", warns)
			}
		})
	}
}
