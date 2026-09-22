package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStatusRoundTrip(t *testing.T) {
	dir := t.TempDir()
	in := Status{
		Schema:    SchemaVersion,
		Now:       []string{"設計を提示", "承認待ち"},
		Next:      []string{"spec を commit"},
		UpdatedAt: fixedTime(),
	}
	if err := WriteStatus(dir, in); err != nil {
		t.Fatalf("WriteStatus() error = %v", err)
	}
	got, err := ReadStatus(dir)
	if err != nil {
		t.Fatalf("ReadStatus() error = %v", err)
	}
	if got.Schema != SchemaVersion {
		t.Errorf("Schema = %d, want %d", got.Schema, SchemaVersion)
	}
	if len(got.Now) != 2 || got.Now[0] != "設計を提示" || got.Now[1] != "承認待ち" {
		t.Errorf("Now = %v", got.Now)
	}
	if len(got.Next) != 1 || got.Next[0] != "spec を commit" {
		t.Errorf("Next = %v", got.Next)
	}
	if !got.UpdatedAt.Equal(fixedTime()) {
		t.Errorf("UpdatedAt = %v, want %v", got.UpdatedAt, fixedTime())
	}
	fi, err := os.Stat(filepath.Join(dir, StatusFileName))
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("status.json のパーミッション = %o, want 600", perm)
	}
}

func TestReadStatusMissing(t *testing.T) {
	if _, err := ReadStatus(t.TempDir()); !os.IsNotExist(err) {
		t.Errorf("無いときのエラー = %v, want IsNotExist", err)
	}
}

func TestReadStatusBrokenJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, StatusFileName), []byte("{ではない"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadStatus(dir); err == nil {
		t.Error("壊れた JSON がエラーにならなかった")
	}
}

func TestStatusEmpty(t *testing.T) {
	cases := []struct {
		name string
		s    Status
		want bool
	}{
		{"両方無し", Status{}, true},
		{"now だけ", Status{Now: []string{"x"}}, false},
		{"next だけ", Status{Next: []string{"x"}}, false},
	}
	for _, c := range cases {
		if got := c.s.Empty(); got != c.want {
			t.Errorf("%s: Empty() = %v, want %v", c.name, got, c.want)
		}
	}
}
