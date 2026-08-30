package store

import (
	"testing"
	"time"
)

func TestSlug(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Notion callout の調査", "notion-callout-の調査"},
		{"  余分な   空白  ", "余分な-空白"},
		{"A/B テスト", "a-b-テスト"},
		{"!!!", ""},
		{"", ""},
		{"---already---kebab---", "already-kebab"},
		{"CamelCase Title", "camelcase-title"},
	}
	for _, c := range cases {
		if got := Slug(c.in); got != c.want {
			t.Errorf("Slug(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSlugTruncatesByRune(t *testing.T) {
	in := ""
	for i := 0; i < 60; i++ {
		in += "あ"
	}
	got := Slug(in)
	if n := len([]rune(got)); n != 40 {
		t.Errorf("len(runes) = %d, want 40", n)
	}
}

func TestFormatID(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{{1, "0001"}, {7, "0007"}, {123, "0123"}, {12345, "12345"}}
	for _, c := range cases {
		if got := FormatID(c.in); got != c.want {
			t.Errorf("FormatID(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSessionDirName(t *testing.T) {
	at := time.Date(2026, 8, 29, 22, 30, 0, 0, time.UTC)
	got := SessionDirName(at, "409bfd08-ade3-44d0-a3f4-56369f828d21")
	if want := "20260829-409bfd08"; got != want {
		t.Errorf("SessionDirName() = %q, want %q", got, want)
	}
}

func TestSessionDirNameShortID(t *testing.T) {
	at := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	got := SessionDirName(at, "abc")
	if want := "20260829-abc"; got != want {
		t.Errorf("SessionDirName() = %q, want %q", got, want)
	}
}

func TestPageDirName(t *testing.T) {
	if got := PageDirName("0007", "notion-callout"); got != "0007-notion-callout" {
		t.Errorf("PageDirName() = %q", got)
	}
	if got := PageDirName("0007", ""); got != "0007" {
		t.Errorf("PageDirName() with empty slug = %q, want 0007", got)
	}
}
