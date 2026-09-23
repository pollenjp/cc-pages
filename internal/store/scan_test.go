package store

import (
	"os"
	"path/filepath"
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
