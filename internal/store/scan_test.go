package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// seed は sessions ディレクトリに 1 ページ作って sessionsDir を返す。
func seed(t *testing.T, root, sessionID, title string, now time.Time) NewPageResult {
	t.Helper()
	res, err := CreatePage(root, "http://localhost:7777", now, NewPageInput{
		SessionID: sessionID, Title: title,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(res.Dir, "index.html"), []byte("<p>x</p>"), 0o600); err != nil {
		t.Fatal(err)
	}
	return res
}

func TestScanEmpty(t *testing.T) {
	got, err := Scan(filepath.Join(t.TempDir(), "sessions"), nil)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

func TestScanFindsPagesInOrder(t *testing.T) {
	root := t.TempDir()
	seed(t, root, "sess-aaaa", "1 つ目", fixedTime())
	seed(t, root, "sess-aaaa", "2 つ目", fixedTime())
	got, err := Scan(filepath.Join(root, "sessions"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("セッション数 = %d, want 1", len(got))
	}
	for _, e := range got {
		if len(e.Pages) != 2 {
			t.Fatalf("ページ数 = %d, want 2", len(e.Pages))
		}
		if e.Pages[0].Page.ID != "0001" || e.Pages[1].Page.ID != "0002" {
			t.Errorf("連番順に並んでいない: %q %q", e.Pages[0].Page.ID, e.Pages[1].Page.ID)
		}
		if e.Session.SessionID != "sess-aaaa" {
			t.Errorf("SessionID = %q", e.Session.SessionID)
		}
		if e.Bytes <= 0 {
			t.Errorf("Bytes = %d, want > 0", e.Bytes)
		}
	}
}

func TestScanSkipsBrokenPage(t *testing.T) {
	root := t.TempDir()
	res := seed(t, root, "sess-aaaa", "良いページ", fixedTime())
	broken := filepath.Join(filepath.Dir(res.Dir), "0002-broken")
	if err := os.MkdirAll(broken, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(broken, "page.json"), []byte("{ではない"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Scan(filepath.Join(root, "sessions"), nil)
	if err != nil {
		t.Fatalf("壊れた page.json で Scan がエラーになった: %v", err)
	}
	for _, e := range got {
		if len(e.Pages) != 1 {
			t.Errorf("ページ数 = %d, want 1 (壊れた方は飛ばす)", len(e.Pages))
		}
	}
}

func TestScanReusesUnchangedSession(t *testing.T) {
	root := t.TempDir()
	seed(t, root, "sess-aaaa", "T", fixedTime())
	sessionsDir := filepath.Join(root, "sessions")

	first, err := Scan(sessionsDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	// prev のページを差し替えておく。再利用されたなら差し替えたものが返る。
	for k, e := range first {
		e.Pages[0].Page.Title = "差し替え済み"
		first[k] = e
	}
	second, err := Scan(sessionsDir, first)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range second {
		if e.Pages[0].Page.Title != "差し替え済み" {
			t.Errorf("mtime が変わっていないのに再読み込みした")
		}
	}
}

func TestScanRereadsChangedSession(t *testing.T) {
	root := t.TempDir()
	seed(t, root, "sess-aaaa", "T", fixedTime())
	sessionsDir := filepath.Join(root, "sessions")

	first, err := Scan(sessionsDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	for k, e := range first {
		e.Pages[0].Page.Title = "差し替え済み"
		first[k] = e
	}
	// ページを足すとセッションディレクトリの mtime が動く。
	seed(t, root, "sess-aaaa", "2 つ目", fixedTime())

	second, err := Scan(sessionsDir, first)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range second {
		if len(e.Pages) != 2 {
			t.Fatalf("ページ数 = %d, want 2", len(e.Pages))
		}
		if e.Pages[0].Page.Title == "差し替え済み" {
			t.Errorf("mtime が変わったのに再読み込みしていない")
		}
	}
}

func TestScanDropsRemovedSession(t *testing.T) {
	root := t.TempDir()
	res := seed(t, root, "sess-aaaa", "T", fixedTime())
	sessionsDir := filepath.Join(root, "sessions")
	first, err := Scan(sessionsDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Dir(res.Dir)); err != nil {
		t.Fatal(err)
	}
	second, err := Scan(sessionsDir, first)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 0 {
		t.Errorf("消したセッションが残っている: %d", len(second))
	}
}

// only は 1 件だけのはずの走査結果を取り出す。
func only(t *testing.T, m map[string]SessionEntry) SessionEntry {
	t.Helper()
	if len(m) != 1 {
		t.Fatalf("セッション数 = %d, want 1", len(m))
	}
	for _, e := range m {
		return e
	}
	return SessionEntry{}
}

// TestScanPicksUpIndexHTMLWrittenAfterScan は、セッションディレクトリの mtime が
// 動かない変更が拾われることを確認する。
//
// skill は cc-pages new が戻った後にページディレクトリへ index.html を書く。
// これが動かすのはページディレクトリの mtime だけで、セッションディレクトリの
// mtime は変わらない。セッションディレクトリだけを見て使い回していると、
// 一番新しいページの本文は永久に集計されず、1 ページのセッション (このツールで
// 想定される形) のサイズが数百バイトのまま止まる。
func TestScanPicksUpIndexHTMLWrittenAfterScan(t *testing.T) {
	root := t.TempDir()
	res, err := CreatePage(root, "http://localhost:7777", fixedTime(), NewPageInput{
		SessionID: "sess-aaaa", Title: "T",
	})
	if err != nil {
		t.Fatal(err)
	}
	sessionsDir := filepath.Join(root, "sessions")

	// cc-pages new が戻った直後の走査。本文はまだ無い。
	first, err := Scan(sessionsDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	before := only(t, first).Bytes

	// skill が本文を書く。セッションディレクトリの mtime は動かない。
	body := strings.Repeat("x", 4096)
	if err := os.WriteFile(filepath.Join(res.Dir, "index.html"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	second, err := Scan(sessionsDir, first)
	if err != nil {
		t.Fatal(err)
	}
	after := only(t, second).Bytes
	if after <= before {
		t.Errorf("Bytes = %d, want > %d (後から書かれた index.html が数えられていない)", after, before)
	}
	if after-before < int64(len(body)) {
		t.Errorf("Bytes の増分 = %d, want >= %d", after-before, len(body))
	}
}

func TestScanReadsStatus(t *testing.T) {
	root := t.TempDir()
	seed(t, root, "sess-aaaa", "T", fixedTime())
	if _, err := UpdateStatus(root, "http://localhost:7777", fixedTime(), StatusInput{
		SessionID: "sess-aaaa", Now: []string{"進行中"}, Next: []string{"次"},
	}); err != nil {
		t.Fatal(err)
	}
	got, err := Scan(filepath.Join(root, "sessions"), nil)
	if err != nil {
		t.Fatal(err)
	}
	e := only(t, got)
	if e.Status == nil {
		t.Fatal("status.json が拾われていない")
	}
	if len(e.Status.Now) != 1 || e.Status.Now[0] != "進行中" || len(e.Status.Next) != 1 || e.Status.Next[0] != "次" {
		t.Errorf("Status = %+v", *e.Status)
	}
	if e.StatusModTime.IsZero() {
		t.Error("StatusModTime が採られていない")
	}
	if len(e.Pages) != 1 {
		t.Errorf("ページ数 = %d, want 1", len(e.Pages))
	}
}

func TestScanSkipsBrokenStatus(t *testing.T) {
	root := t.TempDir()
	res := seed(t, root, "sess-aaaa", "T", fixedTime())
	sessionPath := filepath.Dir(res.Dir)
	if err := os.WriteFile(filepath.Join(sessionPath, StatusFileName), []byte("{ではない"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Scan(filepath.Join(root, "sessions"), nil)
	if err != nil {
		t.Fatalf("壊れた status.json で Scan がエラーになった: %v", err)
	}
	e := only(t, got)
	if e.Status != nil {
		t.Errorf("壊れた status.json が拾われた: %+v", *e.Status)
	}
	if len(e.Pages) != 1 {
		t.Errorf("ページまで飛んだ: %d", len(e.Pages))
	}
}

func TestScanTreatsEmptyStatusAsAbsent(t *testing.T) {
	root := t.TempDir()
	res := seed(t, root, "sess-aaaa", "T", fixedTime())
	// 手で編集して now も next も消えた status.json
	if err := WriteStatus(filepath.Dir(res.Dir), Status{Schema: SchemaVersion, UpdatedAt: fixedTime()}); err != nil {
		t.Fatal(err)
	}
	got, err := Scan(filepath.Join(root, "sessions"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if e := only(t, got); e.Status != nil {
		t.Errorf("空の status.json が「有る」扱いになった: %+v", *e.Status)
	}
}

func TestScanReusesSessionWithUnchangedStatus(t *testing.T) {
	root := t.TempDir()
	seed(t, root, "sess-aaaa", "T", fixedTime())
	sessionsDir := filepath.Join(root, "sessions")
	if _, err := UpdateStatus(root, "http://localhost:7777", fixedTime(), StatusInput{
		SessionID: "sess-aaaa", Now: []string{"進行中"},
	}); err != nil {
		t.Fatal(err)
	}
	first, err := Scan(sessionsDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	// prev の status を差し替えておく。再利用されたなら差し替えたものが返る。
	for k, e := range first {
		e.Status.Now[0] = "差し替え済み"
		first[k] = e
	}
	second, err := Scan(sessionsDir, first)
	if err != nil {
		t.Fatal(err)
	}
	if e := only(t, second); e.Status == nil || e.Status.Now[0] != "差し替え済み" {
		t.Errorf("何も変わっていないのに読み直した: %+v", e.Status)
	}
}

// TestScanRereadsWhenStatusOverwritten は、status.json の上書きだけで読み直される
// ことを確認する。
//
// 上書きはセッションディレクトリの mtime を動かさない (動くのは作成・削除のとき
// だけ)。cc-pages status は毎回 touch を送るので普段は全走査で拾えるが、touch が
// 届かなかったときに定期走査が永久に取りこぼしてはいけない。
func TestScanRereadsWhenStatusOverwritten(t *testing.T) {
	root := t.TempDir()
	res := seed(t, root, "sess-aaaa", "T", fixedTime())
	sessionsDir := filepath.Join(root, "sessions")
	sessionPath := filepath.Dir(res.Dir)
	if _, err := UpdateStatus(root, "http://localhost:7777", fixedTime(), StatusInput{
		SessionID: "sess-aaaa", Now: []string{"古い"},
	}); err != nil {
		t.Fatal(err)
	}
	first, err := Scan(sessionsDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	dirBefore, err := os.Stat(sessionPath)
	if err != nil {
		t.Fatal(err)
	}

	// 上書き。ディレクトリの mtime は動かない。
	if _, err := UpdateStatus(root, "http://localhost:7777", fixedTime(), StatusInput{
		SessionID: "sess-aaaa", Now: []string{"新しい"},
	}); err != nil {
		t.Fatal(err)
	}
	// mtime の分解能が粗い FS でも差が出るよう、status.json の mtime を明示的に
	// 進め、セッションディレクトリの mtime は元に戻して「上書きだけ」を再現する。
	newer := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(filepath.Join(sessionPath, StatusFileName), newer, newer); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(sessionPath, dirBefore.ModTime(), dirBefore.ModTime()); err != nil {
		t.Fatal(err)
	}

	second, err := Scan(sessionsDir, first)
	if err != nil {
		t.Fatal(err)
	}
	if e := only(t, second); e.Status == nil || e.Status.Now[0] != "新しい" {
		t.Errorf("status.json を上書きしたのに読み直していない: %+v", e.Status)
	}
}

// addTwoLinks は sess-aaaa の会話にリンクを 2 件足し、会話のディレクトリを返す。
func addTwoLinks(t *testing.T, root string) string {
	t.Helper()
	res, err := AddLinks(root, "http://localhost:7777", fixedTime(), "sess-aaaa", []LinkInput{
		li("https://app.notion.com/p/abc", "TKT-12", "チケット", SourceAssistant, jst(15, 6)),
		li("https://github.com/o/r/pull/11", "r#11", "PR", SourceTool, jst(16, 20)),
	}, func(string) {})
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(root, "sessions", res.Dir)
}

// TestScanReadsLinks は links.json を拾うことと、ページの無いリンクだけの会話も
// 1 つの会話として出ることを確かめる (status だけの会話と同じ扱い)。
func TestScanReadsLinks(t *testing.T) {
	root := t.TempDir()
	addTwoLinks(t, root)
	got, err := Scan(filepath.Join(root, "sessions"), nil)
	if err != nil {
		t.Fatal(err)
	}
	e := only(t, got)
	if len(e.Links) != 2 || e.Links[0].Label != "TKT-12" || e.Links[1].URL != "https://github.com/o/r/pull/11" {
		t.Errorf("Links = %+v", e.Links)
	}
	if e.LinksModTime.IsZero() {
		t.Error("LinksModTime が採られていない")
	}
	if len(e.Pages) != 0 || e.Session.SessionID != "sess-aaaa" {
		t.Errorf("Pages = %d, SessionID = %q", len(e.Pages), e.Session.SessionID)
	}
}

func TestScanSkipsBrokenLinks(t *testing.T) {
	root := t.TempDir()
	res := seed(t, root, "sess-aaaa", "T", fixedTime())
	if err := os.WriteFile(filepath.Join(filepath.Dir(res.Dir), LinksFileName), []byte("{壊れた"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Scan(filepath.Join(root, "sessions"), nil)
	if err != nil {
		t.Fatalf("壊れた links.json で Scan がエラーになった: %v", err)
	}
	e := only(t, got)
	if e.Links != nil {
		t.Errorf("壊れた links.json が拾われた: %+v", e.Links)
	}
	if len(e.Pages) != 1 {
		t.Errorf("ページまで飛んだ: %d", len(e.Pages))
	}
}

// TestScanRereadsWhenLinksOverwritten は、links.json の書き換えだけで読み直される
// ことを確かめる。cc-pages link は rename で書くのでディレクトリの mtime も動くが、
// 手で直したとき (その場で上書き) や、知らせが届かなかったときに定期走査が永久に
// 取りこぼさないよう、links.json の mtime も見る。
func TestScanRereadsWhenLinksOverwritten(t *testing.T) {
	root := t.TempDir()
	sessionPath := addTwoLinks(t, root)
	sessionsDir := filepath.Join(root, "sessions")
	first, err := Scan(sessionsDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	dirBefore, err := os.Stat(sessionPath)
	if err != nil {
		t.Fatal(err)
	}

	l, err := ReadLinks(sessionPath)
	if err != nil {
		t.Fatal(err)
	}
	l.Links = l.Links[:1]
	l.Links[0].Label = "書き換えた"
	b, err := json.Marshal(l)
	if err != nil {
		t.Fatal(err)
	}
	// その場で上書きし、links.json の mtime だけを進めてディレクトリの mtime は戻す
	if err := os.WriteFile(filepath.Join(sessionPath, LinksFileName), b, 0o600); err != nil {
		t.Fatal(err)
	}
	newer := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(filepath.Join(sessionPath, LinksFileName), newer, newer); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(sessionPath, dirBefore.ModTime(), dirBefore.ModTime()); err != nil {
		t.Fatal(err)
	}

	second, err := Scan(sessionsDir, first)
	if err != nil {
		t.Fatal(err)
	}
	if e := only(t, second); len(e.Links) != 1 || e.Links[0].Label != "書き換えた" {
		t.Errorf("links.json を書き換えたのに読み直していない: %+v", e.Links)
	}
}

func TestReadSessionEntry(t *testing.T) {
	root := t.TempDir()
	seed(t, root, "sess-aaaa", "T", fixedTime())
	sessionPath := addTwoLinks(t, root)
	sessionsDir := filepath.Join(root, "sessions")
	all, err := Scan(sessionsDir, nil)
	if err != nil {
		t.Fatal(err)
	}

	got, ok := ReadSessionEntry(sessionsDir, filepath.Base(sessionPath))
	if !ok {
		t.Fatal("ある会話を読めなかった")
	}
	if want := only(t, all); !reflect.DeepEqual(got, want) {
		t.Errorf("ReadSessionEntry = %+v\nwant (Scan の 1 件) %+v", got, want)
	}
	if _, ok := ReadSessionEntry(sessionsDir, "20260829-nothere"); ok {
		t.Error("無い会話を読めたことになった")
	}
}
