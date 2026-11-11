package store

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
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
