package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
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

const statusBase = "http://localhost:7777"

func TestUpdateStatusWritesStatusAndSkeleton(t *testing.T) {
	root := t.TempDir()
	got, err := UpdateStatus(root, statusBase, fixedTime(), StatusInput{
		SessionID: "409bfd08-aaaa",
		Now:       []string{"設計を提示", " 承認待ち "},
		Next:      []string{"spec を commit", "", "   "},
	})
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	if want := statusBase + "/p/20260829-409bfd08/"; got.URL != want {
		t.Errorf("URL = %q, want %q", got.URL, want)
	}

	sessionPath := filepath.Join(root, "sessions", "20260829-409bfd08")
	s, err := ReadStatus(sessionPath)
	if err != nil {
		t.Fatalf("ReadStatus() error = %v", err)
	}
	if s.Schema != SchemaVersion {
		t.Errorf("Schema = %d, want %d", s.Schema, SchemaVersion)
	}
	// trim され、空になった項目は落ちる
	if len(s.Now) != 2 || s.Now[0] != "設計を提示" || s.Now[1] != "承認待ち" {
		t.Errorf("Now = %q", s.Now)
	}
	if len(s.Next) != 1 || s.Next[0] != "spec を commit" {
		t.Errorf("Next = %q", s.Next)
	}
	if !s.UpdatedAt.Equal(fixedTime()) {
		t.Errorf("UpdatedAt = %v, want %v", s.UpdatedAt, fixedTime())
	}

	// ページより先に打ったので、session.json の骨はここで書かれる
	sess, err := ReadSession(sessionPath)
	if err != nil {
		t.Fatalf("ReadSession() error = %v", err)
	}
	if sess.SessionID != "409bfd08-aaaa" || sess.Dir != "20260829-409bfd08" {
		t.Errorf("Session = %+v", sess)
	}
	if !sess.LastSeen.Equal(fixedTime()) {
		t.Errorf("LastSeen = %v, want %v", sess.LastSeen, fixedTime())
	}

	// stdout に出す JSON は {"url": ...} の 1 キー
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var back map[string]string
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if len(back) != 1 || back["url"] == "" {
		t.Errorf("JSON = %s, want url だけ", b)
	}
}

func TestUpdateStatusReplacesWhole(t *testing.T) {
	root := t.TempDir()
	first := StatusInput{SessionID: "409bfd08-aaaa", Now: []string{"古い"}, Next: []string{"古い次"}}
	if _, err := UpdateStatus(root, statusBase, fixedTime(), first); err != nil {
		t.Fatal(err)
	}
	later := fixedTime().Add(time.Hour)
	second := StatusInput{SessionID: "409bfd08-aaaa", Now: []string{"新しい"}}
	if _, err := UpdateStatus(root, statusBase, later, second); err != nil {
		t.Fatal(err)
	}

	sessionPath := filepath.Join(root, "sessions", "20260829-409bfd08")
	s, err := ReadStatus(sessionPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Now) != 1 || s.Now[0] != "新しい" {
		t.Errorf("Now = %q, want [新しい]", s.Now)
	}
	// 差分ではなく全置換。前回の next は残らない
	if len(s.Next) != 0 {
		t.Errorf("前回の next が残っている: %q", s.Next)
	}
	if !s.UpdatedAt.Equal(later) {
		t.Errorf("UpdatedAt = %v, want %v", s.UpdatedAt, later)
	}
	sess, err := ReadSession(sessionPath)
	if err != nil {
		t.Fatal(err)
	}
	if !sess.LastSeen.Equal(later) {
		t.Errorf("LastSeen = %v, want %v (status でも進める)", sess.LastSeen, later)
	}
}

func TestUpdateStatusReusesSessionDirOfPages(t *testing.T) {
	root := t.TempDir()
	res, err := CreatePage(root, statusBase, fixedTime(), NewPageInput{SessionID: "409bfd08-aaaa", Title: "T"})
	if err != nil {
		t.Fatal(err)
	}
	// 日付をまたいでも、同じ session id なら同じディレクトリに書く
	nextDay := fixedTime().Add(24 * time.Hour)
	got, err := UpdateStatus(root, statusBase, nextDay, StatusInput{SessionID: "409bfd08-aaaa", Now: []string{"x"}})
	if err != nil {
		t.Fatal(err)
	}
	sessionPath := filepath.Dir(res.Dir)
	if _, err := os.Stat(filepath.Join(sessionPath, StatusFileName)); err != nil {
		t.Errorf("ページと同じセッションディレクトリに status.json が無い: %v", err)
	}
	ents, err := os.ReadDir(filepath.Join(root, "sessions"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 1 {
		t.Errorf("セッションディレクトリが割れた: %d 個", len(ents))
	}
	if want := statusBase + "/p/" + filepath.Base(sessionPath) + "/"; got.URL != want {
		t.Errorf("URL = %q, want %q", got.URL, want)
	}
}

func TestUpdateStatusRejectsNoItems(t *testing.T) {
	root := t.TempDir()
	// 空白だけの項目は落ちるので、実質 0 件
	_, err := UpdateStatus(root, statusBase, fixedTime(), StatusInput{SessionID: "409bfd08-aaaa", Now: []string{"  "}})
	if err == nil {
		t.Fatal("now も next も無いのに通った")
	}
	// 拒否したら何も作らない
	if _, err := os.Stat(filepath.Join(root, "sessions")); !os.IsNotExist(err) {
		t.Errorf("拒否したのに sessions/ が作られた")
	}
}

func TestUpdateStatusRejectsBadSession(t *testing.T) {
	for _, sid := range []string{"", "../../../x", "a/b", "a b"} {
		root := t.TempDir()
		_, err := UpdateStatus(root, statusBase, fixedTime(), StatusInput{SessionID: sid, Now: []string{"x"}})
		if err == nil {
			t.Errorf("session id %q が通ってしまった", sid)
		}
		ents, err := os.ReadDir(root)
		if err != nil {
			t.Fatal(err)
		}
		if len(ents) != 0 {
			t.Errorf("session id %q は拒否されるべきなのに root に何か作られた: %v", sid, ents)
		}
	}
}
