package main

// plugin で配る skill (plugins/cc-pages/skills/) と、この repo の CLI・CSS との
// 食い違いを落とす。
//
// skill は GitHub の main から配られ、利用者の手元の binary とは別に更新される。
// skill が CLI に無い flag を書くと、利用者の手元で cc-pages new が失敗する。
// 共通 CSS に無いクラスを勧めると、ページの見た目が黙って崩れる。

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/pollenjp/cc-pages/internal/store"
)

// skillGlobs は契約テストが読む skill の Markdown。
var skillGlobs = []string{
	"plugins/cc-pages/skills/*/SKILL.md",
	"plugins/cc-pages/skills/*/references/*.md",
}

// fragmentStyle は「使えるクラス」の表を持つ文書。
const fragmentStyle = "plugins/cc-pages/skills/cc-page/references/fragment-style.md"

// skillCommand は skill のコードブロックから拾った cc-pages のコマンド 1 つ。
type skillCommand struct {
	Line  int      // コマンドが始まる行 (1 始まり)
	Sub   string   // サブコマンド。無ければ空
	Flags []string // 先頭の - を落とした flag の名前
}

var (
	// quotedValue は "…" と '…' の値。値の中の -- を flag と見なさないために潰す。
	quotedValue = regexp.MustCompile(`"[^"]*"|'[^']*'`)
	// flagToken は -x / --x / --x=v の x を取る。
	flagToken = regexp.MustCompile(`^--?([A-Za-z][A-Za-z0-9_-]*)(=.*)?$`)
)

// skillCommands は Markdown の fenced code block から cc-pages のコマンドを拾う。
//
// 行頭 (字下げと "$ " は飛ばす) が cc-pages の行だけをコマンドと見なす。
// "go build -o cc-pages ." のように cc-pages を引数に取るだけの行は拾わない。
// 行末の \ で続く行は 1 つにつなぐ。
func skillCommands(md string) []skillCommand {
	var (
		cmds    []skillCommand
		inFence bool
		buf     strings.Builder
		start   int
	)
	for i, line := range strings.Split(md, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			buf.Reset()
			continue
		}
		if !inFence {
			continue
		}
		if buf.Len() == 0 {
			start = i + 1
		}
		body, cont := strings.CutSuffix(trimmed, `\`)
		buf.WriteString(body)
		buf.WriteString(" ")
		if cont {
			continue
		}
		if c, ok := parseCommand(buf.String()); ok {
			c.Line = start
			cmds = append(cmds, c)
		}
		buf.Reset()
	}
	return cmds
}

// parseCommand は 1 行につないだコマンドを読む。cc-pages で始まらなければ ok は false。
// パイプや # のコメントから先は cc-pages の引数ではないので読まない。
func parseCommand(s string) (skillCommand, bool) {
	fields := strings.Fields(quotedValue.ReplaceAllString(s, `""`))
	if len(fields) > 0 && fields[0] == "$" {
		fields = fields[1:]
	}
	if len(fields) == 0 || fields[0] != "cc-pages" {
		return skillCommand{}, false
	}
	var c skillCommand
	for i, f := range fields[1:] {
		if f == "|" || f == "||" || f == "&&" || f == ";" || strings.HasPrefix(f, "#") {
			break
		}
		if i == 0 {
			c.Sub = f
			continue
		}
		if m := flagToken.FindStringSubmatch(f); m != nil {
			c.Flags = append(c.Flags, m[1])
		}
	}
	return c, true
}

// knownFlagSets は skill が呼んでよいサブコマンドと、その flag の定義。
//
// serve は FlagSet を関数に出していないので nil にしてある。skill で serve に
// flag を付けたくなったら、newFlags と同じ形で serveFlags を出してここに足す。
func knownFlagSets() map[string]*flag.FlagSet {
	return map[string]*flag.FlagSet{
		"new":    newFlags(&store.NewPageInput{}),
		"status": statusFlags(&store.StatusInput{}),
		"serve":  nil,
	}
}

// commandErrors は c が CLI に無いサブコマンドや flag を使っていれば、その説明を返す。
func commandErrors(c skillCommand, sets map[string]*flag.FlagSet) []string {
	fs, ok := sets[c.Sub]
	if !ok {
		return []string{fmt.Sprintf("L%d: cc-pages に %q というサブコマンドは無い", c.Line, c.Sub)}
	}
	var errs []string
	for _, name := range c.Flags {
		switch {
		case fs == nil:
			errs = append(errs, fmt.Sprintf("L%d: %s の flag (--%s) は確かめられない。main.go に %sFlags を出し、knownFlagSets に足す", c.Line, c.Sub, name, c.Sub))
		case fs.Lookup(name) == nil:
			errs = append(errs, fmt.Sprintf("L%d: cc-pages %s に --%s は無い", c.Line, c.Sub, name))
		}
	}
	return errs
}

// classCell は「使えるクラス」の表の 1 列目 (| `.note` | …) からクラス名を取る。
var classCell = regexp.MustCompile("^\\|\\s*`\\.([A-Za-z][A-Za-z0-9_-]*)`")

// tableClasses は「## 使えるクラス」の節にある表から、1 列目のクラス名を拾う。
func tableClasses(md string) []string {
	var (
		classes []string
		inTable bool
	)
	for _, line := range strings.Split(md, "\n") {
		if strings.HasPrefix(line, "## ") {
			inTable = strings.TrimSpace(line) == "## 使えるクラス"
			continue
		}
		if !inTable {
			continue
		}
		if m := classCell.FindStringSubmatch(line); m != nil {
			classes = append(classes, m[1])
		}
	}
	return classes
}

var (
	cssComment = regexp.MustCompile(`(?s)/\*.*?\*/`)
	cssClass   = regexp.MustCompile(`\.([A-Za-z_-][A-Za-z0-9_-]*)`)
)

// cssSelectorClasses は CSS の rule で、selector の主語に現れるクラス名の集合を返す。
//
// コメントと宣言の中は見ない。style.css のコメントは「.cards と同じ見た目」のように
// クラス名を挙げるので、全文を検索すると、消えたクラスを見逃す。
//
// 数えるのは、, で区切った各 selector の一番右の複合 selector (主語) だけ。
// .status .cols > div の .cols のように祖先の位置にだけ出るクラスは、その要素
// 自身には何も当てていないので、.cols { } が消えたことを見逃さないよう数えない。
// @media などの @ で始まる prelude も selector ではないので飛ばす。
func cssSelectorClasses(css string) map[string]bool {
	css = cssComment.ReplaceAllString(css, "")
	classes := map[string]bool{}
	start := 0
	for i, r := range css {
		switch r {
		case '{':
			prelude := strings.TrimSpace(css[start:i])
			start = i + 1
			if strings.HasPrefix(prelude, "@") {
				continue
			}
			for _, sel := range strings.Split(prelude, ",") {
				for _, m := range cssClass.FindAllStringSubmatch(selectorSubject(sel), -1) {
					classes[m[1]] = true
				}
			}
		case '}', ';':
			start = i + 1
		}
	}
	return classes
}

// selectorSubject は selector の一番右の複合 selector を返す。結合子 (空白・> + ~)
// より後ろの部分で、その rule が実際に当たる要素を表す。
func selectorSubject(sel string) string {
	sel = strings.TrimSpace(sel)
	if i := strings.LastIndexAny(sel, " \t\n>+~"); i >= 0 {
		return sel[i+1:]
	}
	return sel
}

// skillFiles は skillGlobs に当たるファイルを返す。1 つも無ければ止める。
func skillFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	for _, g := range skillGlobs {
		m, err := filepath.Glob(g)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, m...)
	}
	if len(files) == 0 {
		t.Fatalf("skill のファイルが見つからない: %v", skillGlobs)
	}
	return files
}

func TestSkillCommandsParse(t *testing.T) {
	md := strings.Join([]string{
		"本文の `cc-pages new --nope` は拾わない",
		"```sh",
		`cc-pages new --session "$CLAUDE_CODE_SESSION_ID" \`,
		`  --title "--fake を含む題" \`,
		"  --tag a",
		"go build -o cc-pages .",
		`ln -s "$PWD/cc-pages" ~/bin/cc-pages`,
		"$ cc-pages status --now x  # --comment は拾わない",
		"cc-pages serve | grep --color x",
		"```",
		"cc-pages new --outside",
	}, "\n")
	want := []skillCommand{
		{Line: 3, Sub: "new", Flags: []string{"session", "title", "tag"}},
		{Line: 8, Sub: "status", Flags: []string{"now"}},
		{Line: 9, Sub: "serve"},
	}
	if got := skillCommands(md); !reflect.DeepEqual(got, want) {
		t.Errorf("skillCommands() = %+v, want %+v", got, want)
	}
}

func TestCommandErrors(t *testing.T) {
	sets := knownFlagSets()
	cases := []struct {
		name string
		cmd  skillCommand
		want int // エラーの数
	}{
		{"実在する flag", skillCommand{Sub: "new", Flags: []string{"session", "summary", "tag"}}, 0},
		{"綴りを誤った flag", skillCommand{Sub: "new", Flags: []string{"sumary"}}, 1},
		{"status の flag を new に付けた", skillCommand{Sub: "new", Flags: []string{"now"}}, 1},
		{"知らないサブコマンド", skillCommand{Sub: "open"}, 1},
		{"サブコマンドが無い", skillCommand{}, 1},
		{"flag の無い serve", skillCommand{Sub: "serve"}, 0},
		{"flag の付いた serve", skillCommand{Sub: "serve", Flags: []string{"rescan"}}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := commandErrors(tc.cmd, sets); len(got) != tc.want {
				t.Errorf("commandErrors() = %q, want %d 件", got, tc.want)
			}
		})
	}
}

func TestTableClasses(t *testing.T) {
	md := strings.Join([]string{
		"## 使えるクラス",
		"",
		"| クラス | 用途 |",
		"| --- | --- |",
		"| `.note` | 補足の囲み |",
		"| `.diff` | diff |",
		"",
		"## 書いてはいけないもの",
		"",
		"| `.other` | 別の節の表は拾わない |",
	}, "\n")
	want := []string{"note", "diff"}
	if got := tableClasses(md); !reflect.DeepEqual(got, want) {
		t.Errorf("tableClasses() = %q, want %q", got, want)
	}
}

func TestCSSSelectorClasses(t *testing.T) {
	css := strings.Join([]string{
		"/* 枠は .gone と同じ見た目にする */",
		"pre.diff { padding: 0; }",
		"pre.diff .a { background: var(--diff-add-bg); }",
		".note, .warn { border-left: 4px solid; }",
		".status .cols > div { margin: 0; }",
		".cards > * { padding: 1rem; }",
		"@media (prefers-color-scheme: dark) {",
		"  .ok { color: green; }",
		"}",
	}, "\n")
	got := cssSelectorClasses(css)
	for _, c := range []string{"diff", "a", "note", "warn", "ok"} {
		if !got[c] {
			t.Errorf(".%s を拾えていない (%v)", c, got)
		}
	}
	if got["gone"] {
		t.Error("コメントの中の .gone を selector と見なした")
	}
	// 祖先の位置にだけ出るクラスは、その要素自身には何も当てていない。
	// .cols { } が消えても .status .cols > div が残っていれば通る、を起こさない。
	for _, c := range []string{"status", "cols", "cards"} {
		if got[c] {
			t.Errorf("祖先の位置にだけ出る .%s を数えた (%v)", c, got)
		}
	}
	if cssSelectorClasses(".diff-add { color: red; }")["diff"] {
		t.Error(".diff-add を .diff と見なした")
	}
}

// TestSkillCommandsMatchCLI は skill のコマンド例が、CLI に実在する
// サブコマンドと flag だけを使うことを確かめる。
func TestSkillCommandsMatchCLI(t *testing.T) {
	sets := knownFlagSets()
	seen := map[string]int{}
	files := skillFiles(t)
	for _, path := range files {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range skillCommands(string(b)) {
			seen[c.Sub]++
			for _, e := range commandErrors(c, sets) {
				t.Errorf("%s %s", path, e)
			}
		}
	}
	// skill の手順は new と status で回る。どちらかを 1 つも拾えないなら、
	// コードブロックの書き方が変わり、このテストが空回りしている。
	for _, sub := range []string{"new", "status"} {
		if seen[sub] == 0 {
			t.Errorf("skill から cc-pages %s のコマンドを 1 つも拾えなかった: %v", sub, files)
		}
	}
}

// TestSkillClassesExistInCSS は fragment-style.md の「使えるクラス」の表に並べた
// クラスが、共通 CSS に selector として実在することを確かめる。
func TestSkillClassesExistInCSS(t *testing.T) {
	md, err := os.ReadFile(fragmentStyle)
	if err != nil {
		t.Fatal(err)
	}
	css, err := os.ReadFile("internal/web/static/style.css")
	if err != nil {
		t.Fatal(err)
	}
	classes := tableClasses(string(md))
	if len(classes) == 0 {
		t.Fatalf("%s の「使えるクラス」の表からクラスを 1 つも拾えなかった", fragmentStyle)
	}
	have := cssSelectorClasses(string(css))
	for _, c := range classes {
		if !have[c] {
			t.Errorf("%s の .%s が style.css の selector に無い", fragmentStyle, c)
		}
	}
}

// ccPagesEnv は skill が名指しする cc-pages の環境変数 (CC_PAGES_ROOT など)。
var ccPagesEnv = regexp.MustCompile(`\bCC_PAGES_[A-Z][A-Z_]*\b`)

// TestSkillEnvVarsDocumented は skill が名指しする CC_PAGES_* の環境変数が、
// README.md にも書いてあることを確かめる。skill だけが知っている切り替えは、
// 利用者からは見つけようがない。
func TestSkillEnvVarsDocumented(t *testing.T) {
	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range skillFiles(t) {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		seen := map[string]bool{}
		for _, v := range ccPagesEnv.FindAllString(string(b), -1) {
			if seen[v] {
				continue
			}
			seen[v] = true
			if !strings.Contains(string(readme), v) {
				t.Errorf("%s の %s が README.md に無い", path, v)
			}
		}
	}
}
