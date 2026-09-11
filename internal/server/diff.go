package server

import (
	"bytes"
	"strings"
)

// diff の着色。
//
// フラグメントに貼られた <pre class="diff"> の中身を 1 行ずつ span で包み、
// 行頭の記号 (+ / - / @@) から class を決める。色そのものは style.css が持つ。
//
// なぜサーバ側でやるか: 書き手 (Claude) が 1 行ずつ span を巻くことにすると、
// 行数に比例して markup が膨らむ上に巻き忘れる。diff は「生のまま貼れる」のが
// 唯一まともな入力形式で、行頭 1 文字を見るだけの機械的な写像ならここで済む。
//
// 対象は fragment だけ。standalone はアプリの CSS が当たらないので、包んでも
// class の当たらない span が増えるだけになる (readFragment を通らない)。

// 行の class。style.css の pre.diff 配下と対応する。
//
//	a … 追加 (+)     緑
//	d … 削除 (-)     赤
//	h … hunk (@@)    accent 色
//	m … メタ情報     muted
//	c … 文脈行       既定色
const (
	classAdd     = "a"
	classDel     = "d"
	classHunk    = "h"
	classMeta    = "m"
	classContext = "c"
)

// diffMetaPrefixes は unified diff のうち、本文ではなくメタ情報の行の先頭。
//
// 「+ でも - でも空白でもない行はすべてメタ」とは判定しないこと。説明のために
// 手で書く diff は文脈行の先頭の空白を省きがちで、その規則だと地の文が軒並み
// muted に落ちる。拾えるものだけを明示して、残りは文脈行として扱う。
var diffMetaPrefixes = []string{
	"diff ", "index ",
	"new file mode", "deleted file mode", "old mode", "new mode",
	"similarity index", "dissimilarity index",
	"rename from", "rename to", "copy from", "copy to",
	"Binary files ", `\ No newline`,
}

// colorizeDiffs は fragment 内の <pre class="diff"> だけを書き換えて返す。
// 対象が 1 つも無ければ入力をそのまま返す。
func colorizeDiffs(frag []byte) []byte {
	if !bytes.Contains(frag, []byte("<pre")) {
		return frag
	}
	var out bytes.Buffer
	rest := frag
	for {
		i := indexPreTag(rest)
		if i < 0 {
			break
		}
		gt := bytes.IndexByte(rest[i:], '>')
		if gt < 0 {
			break // 閉じない開始タグ。壊れた入力には触らない
		}
		gt += i
		end := bytes.Index(rest[gt+1:], []byte("</pre>"))
		if !hasClassToken(rest[i:gt+1], "diff") || end < 0 {
			out.Write(rest[:gt+1]) // 対象外の <pre> はそのまま通し、次を探す
			rest = rest[gt+1:]
			continue
		}
		out.Write(rest[:gt+1])
		writeDiffBody(&out, rest[gt+1:gt+1+end])
		rest = rest[gt+1+end:]
	}
	out.Write(rest)
	return out.Bytes()
}

// indexPreTag は次の <pre> 開始タグの位置を返す。<presentation> のような
// 別のタグ名に食いつかないよう、タグ名がそこで終わることを確かめる。
func indexPreTag(b []byte) int {
	for off := 0; ; {
		i := bytes.Index(b[off:], []byte("<pre"))
		if i < 0 {
			return -1
		}
		i += off
		if i+len("<pre") >= len(b) {
			return -1
		}
		switch c := b[i+len("<pre")]; c {
		case '>', ' ', '\t', '\r', '\n', '/':
			return i
		}
		off = i + len("<pre")
	}
}

// writeDiffBody は <pre> と </pre> の間を 1 行ずつ span で包んで書く。
func writeDiffBody(out *bytes.Buffer, body []byte) {
	// <code> で包まれていればそれは外側に残す。span は display:block なので、
	// <code> の内と外にまたがらせると入れ子が崩れる。
	var head, tail []byte
	if n := codeOpenLen(body); n > 0 && bytes.HasSuffix(body[n:], []byte("</code>")) {
		head, tail = body[:n], []byte("</code>")
		body = body[n : len(body)-len(tail)]
	}
	out.Write(head)

	// 行を span (display:block) に変えると、行間に残った改行そのものが
	// もう 1 行として描かれて全体が倍の高さになる。改行は捨て、行の区切りは
	// block に任せる。前後の余分な 1 改行もここで落とす (タグの直後で改行して
	// 書くのが自然な書き方で、それを空行として出さない)。
	lines := strings.Split(string(trimOneNewline(body)), "\n")
	classes := classifyDiffLines(lines)
	for i, line := range lines {
		out.WriteString(`<span class="` + classes[i] + `">`)
		out.WriteString(strings.TrimSuffix(line, "\r"))
		out.WriteString(`</span>`)
	}
	out.Write(tail)
}

// classifyDiffLines は各行の class を決める。
func classifyDiffLines(lines []string) []string {
	classes := make([]string, len(lines))
	for i, line := range lines {
		classes[i] = diffLineClass(line)
	}
	// --- / +++ が隣り合っているときだけファイルヘッダと見なし、メタへ落とす。
	// 単独の "--- foo" は削除行 ("-" + "-- foo") のことがある (SQL や Lua の
	// コメントを消した diff がまさにこれ) ので、対で現れたときに限る。
	for i := 0; i+1 < len(lines); i++ {
		if isFileHeader(lines[i], "---") && isFileHeader(lines[i+1], "+++") {
			classes[i], classes[i+1] = classMeta, classMeta
		}
	}
	return classes
}

// diffLineClass は行頭から class を 1 つ決める。
func diffLineClass(line string) string {
	switch {
	case strings.HasPrefix(line, "@@"):
		return classHunk
	case strings.HasPrefix(line, "+"):
		return classAdd
	case strings.HasPrefix(line, "-"):
		return classDel
	}
	for _, p := range diffMetaPrefixes {
		if strings.HasPrefix(line, p) {
			return classMeta
		}
	}
	return classContext
}

// isFileHeader は "--- a/x" や "+++ /dev/null" の形か (marker は "---" / "+++")。
func isFileHeader(line, marker string) bool {
	return line == marker ||
		strings.HasPrefix(line, marker+" ") ||
		strings.HasPrefix(line, marker+"\t")
}

// trimOneNewline は先頭と末尾の改行を 1 つずつだけ落とす。
func trimOneNewline(b []byte) []byte {
	b = bytes.TrimPrefix(b, []byte("\r\n"))
	b = bytes.TrimPrefix(b, []byte("\n"))
	b = bytes.TrimSuffix(b, []byte("\n"))
	b = bytes.TrimSuffix(b, []byte("\r"))
	return b
}

// codeOpenLen は b が <code> 開始タグで始まるならその長さを、でなければ 0 を返す。
func codeOpenLen(b []byte) int {
	if !bytes.HasPrefix(b, []byte("<code")) || len(b) == len("<code") {
		return 0
	}
	switch c := b[len("<code")]; c {
	case '>', ' ', '\t', '\r', '\n', '/':
	default:
		return 0
	}
	j := bytes.IndexByte(b, '>')
	if j < 0 {
		return 0
	}
	return j + 1
}

// hasClassToken は開始タグの class 属性に token が含まれるかを返す。
// class="diff scroll" のように併記されていても拾えるようにする。
func hasClassToken(tag []byte, token string) bool {
	i := bytes.Index(tag, []byte("class="))
	if i < 0 {
		return false
	}
	v := tag[i+len("class="):]
	if len(v) == 0 {
		return false
	}
	var val []byte
	switch q := v[0]; q {
	case '"', '\'':
		j := bytes.IndexByte(v[1:], q)
		if j < 0 {
			return false
		}
		val = v[1 : 1+j]
	default: // 引用符なしの属性値
		j := bytes.IndexAny(v, " \t\r\n>")
		if j < 0 {
			j = len(v)
		}
		val = v[:j]
	}
	for _, f := range bytes.Fields(val) {
		if string(f) == token {
			return true
		}
	}
	return false
}
