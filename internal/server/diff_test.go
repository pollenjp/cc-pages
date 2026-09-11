package server

import (
	"net/http"
	"strings"
	"testing"
)

func TestColorizeDiffsWrapsEachLine(t *testing.T) {
	in := "<pre class=\"diff\"><code>@@ -1,3 +1,3 @@\n ctx\n-old\n+new\n</code></pre>"
	want := `<pre class="diff"><code>` +
		`<span class="h">@@ -1,3 +1,3 @@</span>` +
		`<span class="c"> ctx</span>` +
		`<span class="d">-old</span>` +
		`<span class="a">+new</span>` +
		`</code></pre>`
	if got := string(colorizeDiffs([]byte(in))); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// 改行は span (display:block) に置き換わって消える。残すと行間にもう 1 行
// 描かれて全体が倍の高さになる。
func TestColorizeDiffsDropsNewlines(t *testing.T) {
	in := "<pre class=\"diff\">-a\n+b</pre>"
	got := string(colorizeDiffs([]byte(in)))
	if strings.Contains(got, "\n") {
		t.Errorf("改行が残っている: %q", got)
	}
	if want := `<pre class="diff"><span class="d">-a</span><span class="a">+b</span></pre>`; got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// タグの直後で改行して書くのが自然な書き方。それが空行として出ないこと。
func TestColorizeDiffsTrimsSurroundingNewline(t *testing.T) {
	in := "<pre class=\"diff\"><code>\n+a\n</code></pre>"
	want := `<pre class="diff"><code><span class="a">+a</span></code></pre>`
	if got := string(colorizeDiffs([]byte(in))); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// --- と +++ が対で並んだときだけファイルヘッダ。単独の "--- foo" は
// "-" + "-- foo" の削除行 (SQL や Lua のコメントを消した diff) のことがある。
func TestColorizeDiffsFileHeaderNeedsPair(t *testing.T) {
	pair := string(colorizeDiffs([]byte("<pre class=\"diff\">--- a/x\n+++ b/x</pre>")))
	if !strings.Contains(pair, `<span class="m">--- a/x</span>`) ||
		!strings.Contains(pair, `<span class="m">+++ b/x</span>`) {
		t.Errorf("対の --- / +++ がメタになっていない: %s", pair)
	}

	lone := string(colorizeDiffs([]byte("<pre class=\"diff\">--- SQL のコメント\n ctx</pre>")))
	if !strings.Contains(lone, `<span class="d">--- SQL のコメント</span>`) {
		t.Errorf("単独の --- が削除行になっていない: %s", lone)
	}
}

func TestColorizeDiffsMetaLines(t *testing.T) {
	in := "<pre class=\"diff\">diff --git a/x b/x\nindex 1..2 100644\n+a\n\\ No newline at end of file</pre>"
	got := string(colorizeDiffs([]byte(in)))
	for _, want := range []string{
		`<span class="m">diff --git a/x b/x</span>`,
		`<span class="m">index 1..2 100644</span>`,
		`<span class="a">+a</span>`,
		`<span class="m">\ No newline at end of file</span>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("%s が無い: %s", want, got)
		}
	}
}

// 文脈行の先頭の空白を省いた手書きの diff でも、地の文が muted に落ちないこと。
// 「+ でも - でもない行はメタ」と判定してしまうと、この形が全滅する。
func TestColorizeDiffsHandWrittenContextStaysDefault(t *testing.T) {
	in := "<pre class=\"diff\">func f() {\n-\treturn 1\n+\treturn 2\n}</pre>"
	got := string(colorizeDiffs([]byte(in)))
	for _, want := range []string{`<span class="c">func f() {</span>`, `<span class="c">}</span>`} {
		if !strings.Contains(got, want) {
			t.Errorf("%s が無い: %s", want, got)
		}
	}
}

func TestColorizeDiffsLeavesOtherContentAlone(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"class の無い pre", "<pre><code>$ git status\n-のように見える行</code></pre>"},
		{"別 class の pre", `<pre class="sh">-a</pre>`},
		{"pre の外", "<p>+ と - の話</p>"},
		{"閉じない pre", `<pre class="diff">-a`},
		{"タグ名が pre で終わらない", `<prefix class="diff">-a</prefix>`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := string(colorizeDiffs([]byte(c.in))); got != c.in {
				t.Errorf("書き換わっている\ngot  %s\nwant %s", got, c.in)
			}
		})
	}
}

func TestColorizeDiffsClassTokenList(t *testing.T) {
	got := string(colorizeDiffs([]byte(`<pre class="scroll diff">+a</pre>`)))
	if !strings.Contains(got, `<span class="a">+a</span>`) {
		t.Errorf("class を併記した pre が対象外になっている: %s", got)
	}
}

// 対象の pre が複数あっても、その間にある地の文は素通しすること。
func TestColorizeDiffsMultipleBlocks(t *testing.T) {
	in := `<pre class="diff">+a</pre><p>あいだ</p><pre><code>+b</code></pre><pre class="diff">-c</pre>`
	want := `<pre class="diff"><span class="a">+a</span></pre>` +
		`<p>あいだ</p><pre><code>+b</code></pre>` +
		`<pre class="diff"><span class="d">-c</span></pre>`
	if got := string(colorizeDiffs([]byte(in))); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// 中身は書き手がエスケープ済みの HTML。二重エスケープしないこと。
func TestColorizeDiffsDoesNotReescape(t *testing.T) {
	in := "<pre class=\"diff\">+if a &lt; b {</pre>"
	got := string(colorizeDiffs([]byte(in)))
	if !strings.Contains(got, `<span class="a">+if a &lt; b {</span>`) {
		t.Errorf("中身が書き換わっている: %s", got)
	}
}

// fragment ページを実際に引いて、着色済みの span が chrome の中に出ること。
func TestPageColorizesDiffFragment(t *testing.T) {
	h := realFixture(t, "<pre class=\"diff\"><code>-old\n+new</code></pre>", "0001-alpha")
	rec := get(t, h, "/p/20260829-aaaa/0001-alpha/")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{`<span class="d">-old</span>`, `<span class="a">+new</span>`} {
		if !strings.Contains(body, want) {
			t.Errorf("出力に %q が無い", want)
		}
	}
}
