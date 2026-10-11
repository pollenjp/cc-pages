# 見本の skill を plugin で同梱する Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** この repo を Claude Code の plugin marketplace にし、cc-pages を使いこなす見本の skill `cc-page` を plugin `cc-pages` として配る。skill と CLI・CSS のずれは CI で落とす。

**Architecture:** repo 直下の `.claude-plugin/marketplace.json` が `plugins/cc-pages/` を指し、plugin は skill を 1 つ持つ。skill の本文は、作者の手元の skill を元に個人の環境に結びついた部分を外したもので、全文を Task 3 に載せる。`skill_test.go` は、skill が名指しする flag を `main.go` から切り出した `newFlags` / `statusFlags` と、クラスを `style.css` の selector と突き合わせる。manifest と skill の frontmatter は、新しい workflow が `claude plugin validate` で検査する。

**Tech Stack:** Go 1.24 (`go.mod`。手元は mise の go 1.27)、標準ライブラリのみ。Claude Code の plugin (marketplace.json / plugin.json / SKILL.md)。CI は GitHub Actions と `npx @anthropic-ai/claude-code@2.1.287`。workflow の lint は flake の nixpkgs の actionlint。

**Spec:** `docs/superpowers/specs/2026-10-10-sample-skill-plugin-design.md`

## Global Constraints

- 新しい Go の依存は足さない (`go.mod` の require は `github.com/BurntSushi/toml` のみのまま)
- CI (`.github/workflows/go.yml`) は `gofmt -l .` が空、`go vet ./...`、`go test ./...` を要求する。Go を触る task の最後にこの 3 つを通す
- コメント・エラー文・文書は日本語。コメントは「何をするか」ではなく「なぜそうするか」を書く (既存の流儀)
- コミットメッセージは `type(scope): 日本語の要約` + 空行 + 本文、末尾に `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`
- 作業ブランチはこの worktree の `worktree/silver-field-1e17`。`main` には直接 commit しない。`git stash` は使わない
- 名前: marketplace `cc-pages`、plugin `cc-pages`、skill `cc-page` (明示して呼ぶときは `/cc-pages:cc-page`)
- `version` は plugin.json にも marketplace の entry にも書かない (版は commit SHA になる)
- skill は日本語のまま。元にした作者の手元の skill は変えない (2 本立て)
- 元にした skill の文面のうち、個人の環境に結びついた部分 (作者の skill の名前・呼び方・private な repo のファイルを使った例) は、commit にも PR の本文にも入れない。この plan にも載せない。Task 3 は書き直した後の全文だけを載せ、1 つの commit で足す
- 依存は公開から 7 日以上経った版に固定する (pjp-dep-release-age)。CI の Claude Code は `@anthropic-ai/claude-code@2.1.287` (dist-tag stable・2026-10-01 公開)。Actions は commit SHA で固定する
- `npm install -g` / `pip install` / `apt install` / `brew install` は打たない。版を固定した CLI を手元で試すときは `npx` を使い、cache は `npm_config_cache` で `$SCRATCH` に向ける
- `$SCRATCH` は、実行するセッションの scratchpad の下に作る `tkt130/` の絶対パス。Bash は呼び出しごとに shell が新しくなるので、使う step の先頭で毎回設定する。plan の中では `SCRATCH=/path/to/scratchpad/tkt130` と書くので、実際のパスに置き換えて打つ
- README のコードブロックは 4 字下げ (既存の書き方)。fence は使わない

## 計画時に確かめたこと

plan を書く前に、repo の写しを scratch に作り、次を確かめた。

| 確かめたこと | 結果 | 効く task |
| --- | --- | --- |
| spec の manifest 2 つを `claude plugin validate` (手元の 2.1.295 と、CI に固定する 2.1.287) | どちらの位置でも `version` が無いという警告 1 つだけで exit 0 | 2 |
| 壊れた frontmatter (`description: x` の次の行が `  key: [unclosed`) の SKILL.md | repo 直下の検査は exit 0 で素通りし、`plugins/cc-pages` の検査は exit 1。2.1.287 でも同じ | 2 |
| `@anthropic-ai/claude-code@2.1.287` の `engines` | `node >=22.0.0`。runner に最初から入っている node には頼れない | 2 |
| `actions/setup-node` の版 | v7.1.0 は 2026-10-08 公開で 7 日未満。v7.0.0 (`820762786026740c76f36085b0efc47a31fe5020`、2026-07-14) を使う | 2 |
| README に 1 行で入れる手順が無いとき | repo 直下の検査が Advice を出す (`/plugin install cc-pages --marketplace <owner>/<repo>`、Claude Code 2.1.275 以降)。verdict は変わらない | 5 |
| `claude --plugin-dir ./plugins/cc-pages plugin details cc-pages` | model を呼ばずに `Skills (1)  cc-page` と出る | 3 |
| Task 1 のコードを当てた `main.go` | `newFlags` の FlagSet を作る行から最後の flag までが L72-78 | 3 |
| Task 3 の全文 | 元にした skill の文面と突き合わせ、個人の環境に結びついた部分が残っていない | 3 |
| Task 4 の契約テスト | 実物の skill から new 2 件・status 1 件・クラス 8 個を拾い、通る。flag の綴り・表のクラス・CSS の rule をそれぞれ壊すと落ちる | 4 |

## spec からの差分

計画を書く途中で分かったことで、spec から変えるもの。PR の本文にも書く。

- README の入れ方は、1 行の形 (`/plugin install cc-pages --marketplace pollenjp/cc-pages`) を先に置く。spec の 2 段の形は、古い版とシェル向けに残す
- skill の例の「`main.go` L69-81」は「L72-78」にする。Task 1 の切り出しで flag の定義の行が動くため
- workflow に `actions/setup-node` (v7.0.0・Node 24) を足す。Claude Code 2.1.287 が node 22 以上を求めるため
- 契約テストは「0 件なら失敗」を強め、`new` と `status` をそれぞれ 1 件以上拾えることを求める。`serve` に flag を付けた例は「確かめられない」というエラーにする (`serve` の FlagSet は切り出さない)
- skill が古い binary を見分ける文言 (`flag provided but not defined` / `不明なサブコマンド`) を、Go のテストで固定する (Review Focus の 3)
- 振る舞いの確認に 2 つの場面を足す。`status` を知らない binary と、binary が PATH に無い場合 (Review Focus の 1・2)

## Review Focus

1. **plugin を入れたが binary が PATH に無い** — 新しい利用者が一番踏む。ページを作らずにターミナルで答え、`~/go/bin`・`/proc`・`./cc-pages`・`go build` などで binary を探しに行かない (Task 6 の S5)
2. **binary が `status` より前の版** — `new` は通り、`status` が `不明なサブコマンド` で落ちる。ページの URL を返し、build し直す 1 行を添える (Task 6 の S4)
3. **skill が頼るエラー文言が変わる** — `flag provided but not defined` と `不明なサブコマンド` が変わると、その日以降の skill が古い binary を見分けられない (Task 1 の `TestOldBinaryErrorTexts`)
4. **コマンド例の書き方の揺れ** — 値の中の `--`、行末の `\`、`#` のコメント、パイプの先、`go build -o cc-pages .` のように cc-pages を引数に取る行。値・コメント・パイプ先の `--` を flag と見なさず、続きの行の flag は拾い、cc-pages で始まらない行は無視する (Task 4 の `TestSkillCommandsParse`)
5. **CSS のコメントにだけ残ったクラス、接頭辞が同じ別クラス** — `style.css` のコメントは「.cards と同じ見た目」のようにクラス名を挙げる。rule が消えてもコメントで通る、`.diff-add` があるだけで `.diff` が通る、を起こさない (Task 4 の `TestCSSSelectorClasses`)

## File Structure

| ファイル | 役割 |
| --- | --- |
| `main.go` | `newFlags` / `statusFlags` を出し、`cmdNew` / `cmdStatus` はそれで flag を読む |
| `main_test.go` | 2 つの関数の結びつけと、skill が頼るエラー文言のテストを足す |
| `.claude-plugin/marketplace.json` (新規) | repo を marketplace `cc-pages` にする |
| `plugins/cc-pages/.claude-plugin/plugin.json` (新規) | plugin `cc-pages` の manifest |
| `.github/workflows/plugin.yml` (新規) | `claude plugin validate` を repo 直下と plugin のディレクトリで 2 回流す |
| `plugins/cc-pages/skills/cc-page/SKILL.md` (新規) | 見本の skill の本体 |
| `plugins/cc-pages/skills/cc-page/references/fragment-style.md` (新規) | skill が読む、fragment の書き方 |
| `skill_test.go` (新規) | skill と CLI・CSS の契約テスト |
| `README.md` | 「Claude Code に使わせる」節と、設計書の案内 |

依存の順: Task 1 → Task 3 (行番号) → Task 4 (関数とファイル)。Task 2 は Task 3 の前ならどこでもよい。Task 5 は Task 2・4 の後。Task 6 は最後。

---

### Task 1: main.go — flag の定義を `newFlags` / `statusFlags` に出す

**Files:**
- Modify: `main.go:67-127` (`cmdNew` と `cmdStatus`)
- Modify: `main_test.go` (import と、テストを 3 つ)

**Interfaces:**
- Consumes: `store.NewPageInput{SessionID, Title, Summary string; Tags []string; Prompt, Mode string}`、`store.StatusInput{SessionID string; Now, Next []string}`、`multiFlag` (`main.go`)
- Produces: `func newFlags(in *store.NewPageInput) *flag.FlagSet` (名前 `"new"`、`flag.ContinueOnError`、flag は session / title / summary / prompt / mode / tag)、`func statusFlags(in *store.StatusInput) *flag.FlagSet` (名前 `"status"`、flag は session / now / next)。Task 4 がこの 2 つを `Lookup` で引く

- [ ] **Step 1: 失敗するテストを書く**

`main_test.go` の import に `"io"` と `"reflect"` を足す:

```go
import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/pollenjp/cc-pages/internal/config"
	"github.com/pollenjp/cc-pages/internal/store"
)
```

ファイルの末尾に足す:

```go
// TestNewFlagsFillInput は newFlags が各 flag を NewPageInput のフィールドに
// 結びつけることを確かめる。cmdNew はこの in をそのまま store.CreatePage に
// 渡すので、結びつけがずれると flag が黙って捨てられる。
func TestNewFlagsFillInput(t *testing.T) {
	var in store.NewPageInput
	err := newFlags(&in).Parse([]string{
		"--session", "409bfd08-aaaa",
		"--title", "題",
		"--summary", "一覧の 1 行",
		"--tag", "調査", "--tag", "Go",
		"--prompt", "元の依頼",
		"--mode", "standalone",
	})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	want := store.NewPageInput{
		SessionID: "409bfd08-aaaa",
		Title:     "題",
		Summary:   "一覧の 1 行",
		Tags:      []string{"調査", "Go"},
		Prompt:    "元の依頼",
		Mode:      "standalone",
	}
	if !reflect.DeepEqual(in, want) {
		t.Errorf("in = %+v, want %+v", in, want)
	}
}

func TestStatusFlagsFillInput(t *testing.T) {
	var in store.StatusInput
	err := statusFlags(&in).Parse([]string{
		"--session", "409bfd08-aaaa",
		"--now", "進行中",
		"--next", "次にやる", "--next", "その次",
	})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	want := store.StatusInput{
		SessionID: "409bfd08-aaaa",
		Now:       []string{"進行中"},
		Next:      []string{"次にやる", "その次"},
	}
	if !reflect.DeepEqual(in, want) {
		t.Errorf("in = %+v, want %+v", in, want)
	}
}

// TestOldBinaryErrorTexts は、skill が「binary が skill より古い」と見分けるのに
// 使う 2 つの文言を固定する。
//
// skill は main を追い、利用者の binary は build した日のまま残る。今日の binary も
// いずれ誰かの手元で「古い binary」になるので、ここの文言を変えると、その日の
// skill がずれを見分けられなくなる。
func TestOldBinaryErrorTexts(t *testing.T) {
	t.Run("知らない flag", func(t *testing.T) {
		fs := newFlags(&store.NewPageInput{})
		fs.SetOutput(io.Discard)
		err := fs.Parse([]string{"--no-such-flag", "x"})
		if err == nil || !strings.Contains(err.Error(), "flag provided but not defined") {
			t.Errorf("err = %v, want \"flag provided but not defined\" を含む", err)
		}
	})
	t.Run("知らないサブコマンド", func(t *testing.T) {
		// run は先に設定を読む。手元の ~/.config/cc-pages/config.toml に左右されないよう
		// HOME を空のディレクトリに向ける。
		t.Setenv("HOME", t.TempDir())
		err := run([]string{"no-such-subcommand"})
		if err == nil || !strings.Contains(err.Error(), "不明なサブコマンド") {
			t.Errorf("err = %v, want \"不明なサブコマンド\" を含む", err)
		}
	})
}
```

- [ ] **Step 2: 落ちることを確かめる**

Run: `go test -run 'TestNewFlagsFillInput|TestStatusFlagsFillInput|TestOldBinaryErrorTexts' .`
Expected: FAIL (`[build failed]`)。`undefined: newFlags` と `undefined: statusFlags`

- [ ] **Step 3: main.go を直す**

`main.go` の `// cmdNew はページディレクトリを作り、` の行から `cmdNew` の閉じ括弧までを、次に置き換える:

```go
// newFlags は new の FlagSet を作り、各 flag を in のフィールドに結びつける。
//
// cmdNew から出してあるのは、skill_test.go が skill に書いた flag の実在を
// 同じ定義で確かめるため。flag を足すときはここに足せば、両方に効く。
func newFlags(in *store.NewPageInput) *flag.FlagSet {
	fs := flag.NewFlagSet("new", flag.ContinueOnError)
	fs.StringVar(&in.SessionID, "session", "", "セッション ID (CLAUDE_CODE_SESSION_ID)")
	fs.StringVar(&in.Title, "title", "", "ページのタイトル")
	fs.StringVar(&in.Summary, "summary", "", "一覧に出る 1 行")
	fs.StringVar(&in.Prompt, "prompt", "", "元になった依頼")
	fs.StringVar(&in.Mode, "mode", "", "fragment (既定) または standalone")
	fs.Var((*multiFlag)(&in.Tags), "tag", "タグ (繰り返し指定できる)")
	return fs
}

// cmdNew はページディレクトリを作り、{"id","dir","url"} を stdout に出す。
func cmdNew(cfg config.Config, args []string) error {
	var in store.NewPageInput
	if err := newFlags(&in).Parse(args); err != nil {
		return err
	}

	res, err := store.CreatePage(cfg.Root, cfg.BaseURL(), time.Now(), in)
	if err != nil {
		return err
	}
	store.NotifyTouch(cfg.BaseURL())
	enc := json.NewEncoder(os.Stdout)
	return enc.Encode(res)
}
```

`// cmdStatus はセッションの` の行から `cmdStatus` の閉じ括弧までを、次に置き換える:

```go
// statusFlags は status の FlagSet を作り、各 flag を in のフィールドに結びつける。
// cmdStatus から出してある理由は newFlags と同じ。
func statusFlags(in *store.StatusInput) *flag.FlagSet {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.StringVar(&in.SessionID, "session", "", "セッション ID (CLAUDE_CODE_SESSION_ID)")
	fs.Var((*multiFlag)(&in.Now), "now", "現在の状況 (繰り返し指定できる)")
	fs.Var((*multiFlag)(&in.Next), "next", "次やること (繰り返し指定できる)")
	return fs
}

// cmdStatus はセッションの「現在の状況」を全置換で書き、{"url"} を out に出す。
//
// out を引数に取るのはテストのため (cmdNew は os.Stdout に直接書いている)。
// 検証エラーのときは out に何も書かない。skill は stdout を JSON として読むので、
// 失敗時に半端な出力を残さない。
func cmdStatus(cfg config.Config, args []string, out io.Writer) error {
	var in store.StatusInput
	if err := statusFlags(&in).Parse(args); err != nil {
		return err
	}

	res, err := store.UpdateStatus(cfg.Root, cfg.BaseURL(), time.Now(), in)
	if err != nil {
		return err
	}
	store.NotifyTouch(cfg.BaseURL())
	return json.NewEncoder(out).Encode(res)
}
```

`(*multiFlag)(&in.Tags)` は、`[]string` のフィールドを `multiFlag` の `Set` (append) で埋めるための型変換。`multiFlag` の下の型は `[]string` なので変換できる。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test -run 'TestNewFlagsFillInput|TestStatusFlagsFillInput|TestOldBinaryErrorTexts' -v .`
Expected: PASS。3 つのテストと、`TestOldBinaryErrorTexts` の subtest 2 つ

Run: `gofmt -l . && go vet ./... && go test ./...`
Expected: gofmt の出力なし、全 package が ok (既存の `TestStatusWritesFileAndPrintsURL` なども通る)

Run: `grep -n -e 'fs := flag.NewFlagSet("new"' -e 'fs.Var((\*multiFlag)(&in.Tags)' main.go`
Expected: `72:` と `78:` の 2 行。Task 3 の skill は、この範囲 (FlagSet を作る行から最後の flag まで) を「L72-78」として例に使う。ずれたら、Task 3 の全文の `L72-78` (2 か所) をこの 2 行の番号に合わせて直す

- [ ] **Step 5: CLI を通して確かめる**

```bash
SCRATCH=/path/to/scratchpad/tkt130
mkdir -p "$SCRATCH/bin"
go build -o "$SCRATCH/bin/cc-pages" .
export CC_PAGES_ROOT="$SCRATCH/root-smoke" CC_PAGES_ADDR=127.0.0.1:1
"$SCRATCH/bin/cc-pages" new --session 409bfd08-aaaa --title 題 --summary 要約 --tag a --tag b --prompt 依頼
jq -c '{title,summary,tags,prompt,mode}' "$SCRATCH"/root-smoke/sessions/*/0001-*/page.json
"$SCRATCH/bin/cc-pages" status --session 409bfd08-aaaa --now 進行中 --next 次
"$SCRATCH/bin/cc-pages" new --session 409bfd08-aaaa --nope x; echo "exit=$?"
```

Expected:
- `new` が `{"id":"0001","dir":"…","url":"http://localhost:1/p/<今日の日付>-409bfd08/0001-題"}` を出す
- page.json が `{"title":"題","summary":"要約","tags":["a","b"],"prompt":"依頼","mode":"fragment"}`
- `status` が `{"url":"http://localhost:1/p/<今日の日付>-409bfd08/"}` を出す
- `--nope` は stderr の最後の行が `cc-pages: flag provided but not defined: -nope` で、`exit=1`

`CC_PAGES_ADDR=127.0.0.1:1` は誰も居ないポート。手元で動いている cc-pages serve に touch を飛ばさない。

- [ ] **Step 6: Commit**

```bash
git add main.go main_test.go
git commit -m "$(cat <<'EOF'
refactor(cli): new と status の flag の定義を関数に出す

skill_test.go が、plugin で配る skill に書いた flag の実在を同じ定義で
確かめられるようにする。挙動は変えない。

skill が「binary が skill より古い」と見分けるのに使う 2 つのエラー文言
(flag provided but not defined / 不明なサブコマンド) もテストで固定する。

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: plugin の器 — marketplace・plugin.json・validate の workflow

**Files:**
- Create: `.claude-plugin/marketplace.json`
- Create: `plugins/cc-pages/.claude-plugin/plugin.json`
- Create: `.github/workflows/plugin.yml`

**Interfaces:**
- Consumes: なし
- Produces: marketplace `cc-pages` と plugin `cc-pages` (source `./plugins/cc-pages`)。Task 3 が `plugins/cc-pages/skills/cc-page/` に skill を置く。workflow の job `validate` は、Task 3 以降の push で skill の frontmatter も検査する

- [ ] **Step 1: marketplace.json を書く**

`.claude-plugin/marketplace.json`:

```json
{
  "name": "cc-pages",
  "owner": { "name": "pollenjp" },
  "description": "cc-pages を Claude Code から使うための plugin",
  "plugins": [
    {
      "name": "cc-pages",
      "source": "./plugins/cc-pages",
      "description": "長い応答を cc-pages のページに書き出し、ターミナルには短い箇条書きとリンクだけを返す skill"
    }
  ]
}
```

- `source` は `./` で始める (必須)。相対パスは marketplace の root (repo 直下) から解決される
- `version` は書かない。`$schema` も付けない (editor の補完用で、読み込み時は無視される)

- [ ] **Step 2: plugin.json を書く**

`plugins/cc-pages/.claude-plugin/plugin.json`:

```json
{
  "name": "cc-pages",
  "description": "長い応答を cc-pages のページに書き出し、ターミナルには短い箇条書きとリンクだけを返す skill",
  "author": { "name": "pollenjp" },
  "homepage": "https://github.com/pollenjp/cc-pages",
  "repository": "https://github.com/pollenjp/cc-pages",
  "license": "MIT"
}
```

- [ ] **Step 3: 手元の CLI で検査する**

Run: `DISABLE_AUTOUPDATER=1 claude plugin validate .; echo "exit=$?"`
Expected: `Validating marketplace manifest: …/.claude-plugin/marketplace.json`、警告は `plugins[0] plugin.json → version: No version specified. …` の 1 つだけ、`✔ Validation passed with warnings`、`exit=0`。README の Advice も出るが、Task 5 で消える

Run: `DISABLE_AUTOUPDATER=1 claude plugin validate plugins/cc-pages; echo "exit=$?"`
Expected: `Validating plugin manifest: …/plugins/cc-pages/.claude-plugin/plugin.json`、警告は `version: No version specified. …` の 1 つだけ、`exit=0`

- [ ] **Step 4: workflow を書く**

`.github/workflows/plugin.yml`:

```yaml
# plugin (plugins/cc-pages) と marketplace (.claude-plugin/) の検査。
#
# go.yml とは別 workflow にしてある。こちらは Claude Code の CLI を npm から
# 取ってくるぶん遅く、落ちる理由も Go の単体とは違う。落ちたときにどちらの
# 話なのかが一目で分かる形にする。
name: plugin

on:
  push:
    branches: [main]
  pull_request:

permissions:
  contents: read

concurrency:
  group: plugin-${{ github.event.pull_request.number || github.ref }}
  cancel-in-progress: true

env:
  # CI の中で CLI に自分を更新させない。版は下の CLAUDE_CODE で固定している。
  DISABLE_AUTOUPDATER: "1"
  # stable の dist-tag で、公開から 7 日以上経った版 (2.1.287 は 2026-10-01 公開)。
  # 上げるときも同じ基準で選ぶ。
  CLAUDE_CODE: "@anthropic-ai/claude-code@2.1.287"

jobs:
  validate:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1

      # @anthropic-ai/claude-code は engines で node >= 22 を求める。runner に
      # 最初から入っている node の版には頼らない。
      - uses: actions/setup-node@820762786026740c76f36085b0efc47a31fe5020 # v7.0.0
        with:
          node-version: "24"

      # 認証は要らない。--strict は付けない。plugin に version を書かない
      # 決まりなので、「version が無い」警告で必ず落ちる。
      - name: marketplace を検証
        run: npx --yes "$CLAUDE_CODE" plugin validate .

      # marketplace の検証は、plugin の skill のファイルを開かない。
      # SKILL.md の frontmatter まで見るには、plugin のディレクトリを別に流す。
      - name: plugin を検証
        run: npx --yes "$CLAUDE_CODE" plugin validate plugins/cc-pages
```

- [ ] **Step 5: workflow を lint する**

Run: `nix shell --inputs-from . nixpkgs#actionlint --command actionlint .github/workflows/plugin.yml; echo "exit=$?"`
Expected: 出力なしで `exit=0`。actionlint は flake.lock が固定した nixpkgs から取る (1.7.12)

- [ ] **Step 6: CI と同じ版で流す**

```bash
SCRATCH=/path/to/scratchpad/tkt130
mkdir -p "$SCRATCH/npm-cache"
nix develop . --command env npm_config_cache="$SCRATCH/npm-cache" DISABLE_AUTOUPDATER=1 \
  sh -c 'npx --yes @anthropic-ai/claude-code@2.1.287 plugin validate . && npx --yes @anthropic-ai/claude-code@2.1.287 plugin validate plugins/cc-pages'
echo "exit=$?"
```

Expected: 2 回とも `✔ Validation passed with warnings` (警告は version の 1 つずつ)、`exit=0`。npm の「New major version of npm available」の notice は無視してよい

- [ ] **Step 7: 2 回流す理由を確かめる**

frontmatter の壊れた skill を、scratch の写しに置いて流す:

```bash
SCRATCH=/path/to/scratchpad/tkt130
rm -rf "$SCRATCH/broken" && mkdir -p "$SCRATCH/broken"
cp -r .claude-plugin plugins "$SCRATCH/broken/"
mkdir -p "$SCRATCH/broken/plugins/cc-pages/skills/cc-page"
printf -- '---\nname: cc-page\ndescription: x\n  key: [unclosed\n---\n\n# x\n' > "$SCRATCH/broken/plugins/cc-pages/skills/cc-page/SKILL.md"
(cd "$SCRATCH/broken" && DISABLE_AUTOUPDATER=1 claude plugin validate . >/dev/null; echo "root exit=$?")
(cd "$SCRATCH/broken" && DISABLE_AUTOUPDATER=1 claude plugin validate plugins/cc-pages | grep frontmatter)
(cd "$SCRATCH/broken" && DISABLE_AUTOUPDATER=1 claude plugin validate plugins/cc-pages >/dev/null; echo "plugin exit=$?")
```

Expected: `root exit=0`、`frontmatter: YAML frontmatter failed to parse: …` の行、`plugin exit=1`。repo 直下だけを流しても、壊れた skill は CI を通ってしまう

- [ ] **Step 8: Commit**

```bash
git add .claude-plugin/marketplace.json plugins/cc-pages/.claude-plugin/plugin.json .github/workflows/plugin.yml
git commit -m "$(cat <<'EOF'
feat(plugin): repo を Claude Code の marketplace にし、plugin の器を置く

marketplace と plugin の名前はどちらも cc-pages。version は書かず、版は
plugin のディレクトリの commit SHA に任せる。

CI では claude plugin validate を repo 直下と plugins/cc-pages の 2 か所で
流す。repo 直下の検査は plugin の skill のファイルを開かないため。

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: skill — 見本の skill `cc-page` を置く

**Files:**
- Create: `plugins/cc-pages/skills/cc-page/SKILL.md`
- Create: `plugins/cc-pages/skills/cc-page/references/fragment-style.md`

**Interfaces:**
- Consumes: Task 1 の `main.go` の行番号 (L72-78 = `newFlags` の FlagSet を作る行から最後の flag まで)、Task 2 の `plugins/cc-pages/.claude-plugin/plugin.json`、`README.md`「使い方」の L20-22 (SKILL.md の原文の例が写す)
- Produces: skill `cc-page`。コードブロックに `cc-pages new` (SKILL.md の手順 1 と fragment-style.md の standalone) と `cc-pages status` (SKILL.md の手順 3) がある。fragment-style.md の「## 使えるクラス」の表に `.note` `.warn` `.ok` `.cols` `.cards` `.metric` `.scroll` `.diff`。Task 4 がこれを読む

振る舞い (ページを書くか・古い binary をどう扱うか) は Task 6 で subagent に使わせて確かめる。

2 つのファイルは、作者の手元の skill を元に、個人の環境に結びついた部分を外して書き直したもの。元の文面は repo に入れない (Global Constraints) ので、元を写してから直す手順は踏まず、書き直した後の全文を置く。spec の「skill の中身」の表の行は、全文の次の場所に当たる:

| spec の行 | 全文の場所 |
| --- | --- |
| frontmatter の `name`・`description`・見出し | SKILL.md L1-15 |
| 手順 1 (binary が古いときの段落) | SKILL.md L43-46 |
| 手順 2 の原文アコーディオン | SKILL.md L76-91 |
| 手順 2 の図の段落 | SKILL.md L121-124 |
| 使わない場面 | SKILL.md L196-199 |
| 「参照の原文」の「付けるかどうか」の表 | fragment-style.md L145-153 |
| 「描き方は他の skill を引く」 | fragment-style.md L273-294 |

- [ ] **Step 1: SKILL.md を書く**

`plugins/cc-pages/skills/cc-page/SKILL.md` を次の内容で作る (Write tool で):

~~~~~markdown
---
name: cc-page
description: >
  ユーザーが「cc-page で」「cc-page に出して」「ページにして」のように
  名指しで指示したら、内容・長さに関わらず必ず使う。名指しは除外条件より優先する。
  名指しが無いときは、回答が長くなる・表や図で見せた方が速い・複数案の比較や調査結果を
  返すときに使う。plan・設計メモ・調査結果を md ファイルへ書き出して「ファイルに
  出力しました」と返したくなったときも対象で、ターミナル出力が短いことは使わない理由に
  ならない。ターミナルに流すと読みづらい応答を、cc-pages（ローカル web ビューア）で開ける
  HTML ページとして書き出し、stdout には短い箇条書き (3 行まで) とリンクだけを返す。
  名指しが無く、yes/no で済む確認・コードを 1 ファイル直しただけの報告・コマンド出力を
  そのまま見せれば済む場合には使わない。
---

# cc-page

長い応答を `cc-pages` のページとして書き出す。ターミナルには短い箇条書きとリンクだけを残す。

## 手順

### 1. ページを作る

```sh
cc-pages new --session "$CLAUDE_CODE_SESSION_ID" \
  --title "Notion callout の調査" \
  --summary "一覧カードに出る 1 行" \
  --tag 調査 \
  --prompt "元になったユーザーの依頼を 1 行で"
```

`{"id":"0007","dir":"...","url":"..."}` が返る。

**`--summary` は省略しない。** 一覧とセッションの top ページに出る 1 行で、そのページが
何を扱うかを書く。top ページでは「各ページの説明」としてこれだけが読まれる。

**`command -v cc-pages` が空なら、そこで終わり。** この skill は使わず、ターミナルで普通に
答える。ページが書けないことを理由に回答を止めてはいけない。

**別の場所を探さないこと。** PATH に無いのは障害ではなく「意図的に無効化されている」合図。
`~/go/bin`、`~/.local/bin`、走っているサーバプロセスの `/proc/<pid>/exe` などを当たっては
いけない。`CC_PAGES_DISABLE` が設定されている場合も同じく使わない。

**`new` が `flag provided but not defined` や `不明なサブコマンド` で失敗したら、cc-pages の
binary がこの skill より古い。** ページは諦めてターミナルで答え、最後に「cc-pages の binary が
この skill より古いので、pull して build し直すと cc-page が使える」と 1 行だけ添える。
手順 3 の `status` が同じ理由で失敗したときは、ページの URL を返したうえで同じ 1 行を添える。

### 2. `dir/index.html` を書く

`<body>` の中身に相当する**フラグメント**だけを書く。`<!doctype>` `<html>` `<head>`
`<body>` は書かない。共通の CSS とナビはアプリが被せる。

使える語彙は `references/fragment-style.md` を読むこと。

**文章は箇条書きに割る。段落で書かない。** `<p>` を使ってよいのは 1 文で終わるときだけで、
2 文以上並べたくなったら `<ul>` にする。形は「内容を一言で表す親項目 → その下に 1 文ずつの
サブ項目」。

```html
<ul>
  <li>API から作れる callout は 1 種類だけ
    <ul>
      <li>toggle 付きと見出し付きは UI 専用で、API に型が無い。</li>
      <li>作れるのは通常の callout に限る。</li>
    </ul>
  </li>
</ul>
```

- 親項目は名詞句。文ではないので `。` を付けない
- 子項目は **1 文 1 項目**。`。` で切って次の項目へ送る
- **`。` の後ろに同じ行のまま文を続けない。** 囲み・表のセル・図の説明・standalone でも同じ

例外の扱い・確認の仕方は `references/fragment-style.md` の「文章の刻み方」。

**ページの外の文章を指したら、その原文をアコーディオンで添える。** ファイル・文書・URL・
commit / PR・コマンドの出力を名指しして中身に触れたら (「README.md の『検証』節は〜」
「〜を禁じている (CLAUDE.md)」「`main.go` L72-78 で〜」)、その項目の中に `<details>` を
置き、指した箇所をそのまま写す。読み手が元を開きに行かなくても、ページの中で確かめられる
ようにするため。

```html
<li><code>status</code> は前回の内容を引き継がず、毎回すべてを書き直す。
  <details>
    <summary>原文: <code>README.md</code>「使い方」</summary>
    <pre>`status` はセッション直下の `status.json` を丸ごと書き換え（全置換）、セッションの
ページ一覧 `/p/&lt;session&gt;/` の上に「現在の状況」「次やること」として出す。`--now` と
`--next` は繰り返し指定でき、どちらか 1 つは要る。ページより先に打ってもよい。</pre>
  </details>
</li>
```

- 中身は**写し**。要約・言い換え・訳をせず、自分の注記も混ぜない。`。` の刻み直しもしない
- 写すのは、このセッションで実際に読んだ文章。まだ読んでいない出典を指すなら、読んでから書く
- `<summary>` は「原文: 場所」。パス・行番号・節名・URL・打ったコマンドなど、元へ辿れる形にする

付ける・付けないの境目、切り出す範囲、置き場所、`<pre>` と `<blockquote>` の使い分けは
`references/fragment-style.md` の「参照の原文」。

**図は既定で多い。** ページは「文章に図を添えたもの」ではなく「図に文章を添えたもの」に
する。節を書き始める前に、その節の図を決める。図が浮かばない節は、図が要らない節ではなく、
まだ言語化が済んでいない節であることが多い。

**量の目安: h2 セクション 1 つにつき 1 枚以上、ページ全体で 3 枚以上。** 迷ったら足す側に
倒す。図が多すぎるという苦情は来ていない。「表で済ませた方が早い」と感じたときは、
たいてい関係・経路・変化を表に潰している。

出てきたら描くもの:

| ページに出てくる話 | 描く図 |
|---|---|
| 登場物が 3 つ以上あって互いに関係する | 構成図 (箱と線) |
| 順番・経路・分岐がある | フロー図 / シーケンス図 |
| 案が 2 つ以上ある | 比較図 (並置)。表**と併せて**出す。表だけにしない |
| 前後で変わる | before / after の並置 |
| 状態が移る | 状態遷移図 |
| 時間・期間・待ちがある | タイムライン |
| 数値が 3 つ以上並ぶ | グラフ |
| 階層・入れ子・ディレクトリ | ツリー |

表で足りるのは「並べるだけの事実」まで。関係・経路・状態遷移は inline SVG や
CSS アニメーションで描く (fragment のまま可)。道具立て・確認の作法と、**図を描く skill が
手元にあれば**その使い分けは `references/fragment-style.md` の「図とアニメーション」。inline SVG は
`<style>` を `<svg>` の中に閉じる決まった形がある (同じ節の「inline SVG の書き方」)。

**JS が要る表現は `new` に `--mode standalone` を付けて**完全な HTML を書く。
トグル切り替え・ステップ再生などの対話的な図、並べ替えできる表、横に広いレイアウトが
これに当たる。図が分かりやすくなるなら standalone へ逃がすのは消極的な回避ではなく
積極的な選択でよい。その URL は共通 CSS もナビも被らないただの HTML になるので、
色・余白・ダークモード・戻り導線（`<a href="../">`）を全部自分で書くことになる。
表・囲み・カード・`assets/` の画像で足りるものは fragment のままにする。
条件と制約は `references/fragment-style.md`。

### 3. セッションの状況を更新する

ページを書き終えたら、**毎回**、セッション全体の状況を言い直す。セッションの top ページ
(`url` の 1 つ上 = `/p/<session>/`) がこれを「現在の状況」「次やること」として出す。

```sh
cc-pages status --session "$CLAUDE_CODE_SESSION_ID" \
  --now "callout の調査が終わり、案 A で進める方向で確認待ち" \
  --now "実装計画はまだ書いていない" \
  --next "ユーザーの返事を待って spec を commit する" \
  --next "writing-plans で実装計画を書く"
```

`{"url":"http://localhost:7777/p/20260829-409bfd08/"}` が返る。この URL は stdout に
出さない (手順 4 で返すのはページの URL)。

- **セッション全体の状況を書く。** 今作ったページの目次ではない。「何が決まり、何が
  残っていて、次に何をするか」を、ページを開かずに分かる粒度で書く
- `--now` は 1〜3 項目、`--next` は 0〜5 項目。各項目は 1 文で、`。` は付けない。
  やることが無ければ `--next` は省いてよい
- **全置換。** 前回の内容は引き継がれない。まだ有効な項目も含めて全部書き直す
- ページを作らない往復でも、状況が動いたら打ってよい (方針転換・作業の完了)
- 失敗しても回答は止めない。`new` と同じ退行規則で、`command -v cc-pages` が空なら
  手順 1 と同じく何もしない

### 4. stdout に短い箇条書きとリンクを返す

```
- callout は 3 種類。うち 2 つは API から作れない
- 作れる 1 つも子ブロックは後から足せない
→ http://localhost:7777/p/20260829-409bfd08/0007-notion-callout-の調査
```

**箇条書きは 3 行まで。** ページを開かずに次の一言を返せる場面が多いので、判断に要る結論は
ターミナルに残す。ただし詳細はページ側に置く。

**リンクだけにしない。** 毎回ページを開かないと意思決定できないのは体験として後退する。

**ページに何を入れたかを説明しない。** 「ページには依存グラフと表を入れてあります」の類は
答えではなく中身の目録で、開けば分かる。その行数があるなら結論をもう 1 項目書く。

整理していて気付いた指摘や懸念は、ページに埋もれさせず箇条書きに含めてよい。

## md ファイルに書き出したくなったら

**判断の基準はターミナル出力の長さではなく、成果物の中身の量。** plan・設計メモ・調査結果を
md へ落として「ファイルに出力しました」と返すのは、長い応答をターミナルから追い出しただけで、
残った stdout が短いのはこの skill を使わない理由にならない。むしろ「md にまとめよう」と
思った時点で対象。

md のまま渡すと図が描けず、ユーザーは別のビューアで開くことになる。ページなら表・囲み・
inline SVG が使えて、セッションの一覧にも残る。

**ファイルであることに理由があるなら、ファイルは書く。そのうえでページも出す。** 理由とは、
ユーザーがファイルを指定した / repo に commit する成果物 (`docs/`、ADR、README) /
後続セッションが読む plan (superpowers:writing-plans など) のこと。理由が無い、つまり
ただ読ませたいだけの md は作らず、ページだけにする。

両方書いたときも stdout に返すのはページの URL。ファイルのパスは箇条書きで 1 行触れる程度。

## 使わない場面

**名指しで指示されたときは、この節は適用しない。** 「cc-page で」と頼まれたときや、
`/cc-pages:cc-page` で呼び出されたときは、答えが 1 行で済んでも、yes/no で済む確認でも、ページを作る。
ここで「中身が薄いからターミナルで返す」と判断を戻すのは、除外条件の適用ではなく
ユーザーの指示の却下にあたる。短い答えは短いページにすればよい。

名指しが無いときに限り、次には使わない。

- yes/no で済む確認
- コードを 1 ファイル直しただけの報告（md へ成果物を書き出したのはこれに当たらない。上記）
- コマンドの出力をそのまま見せれば済むとき
- ユーザーの返事を待っている途中の質問（ターミナルの方が速い）

## 粒度

1 往復 = 1 ページ。同じ話題を練り直したときも上書きせず新しいページを作る。版が残るので、
セッションのページ一覧を見れば推敲の過程がそのまま読める。
~~~~~

- [ ] **Step 2: fragment-style.md を書く**

`plugins/cc-pages/skills/cc-page/references/fragment-style.md` を次の内容で作る (Write tool で):

~~~~~markdown
# フラグメントの書き方

`index.html` は `<body>` の中身に相当する断片。共通 CSS はアプリが持つので、
**素のセマンティックな HTML を書けばよい**。CSS を毎回書かないこと。

## 文章の刻み方

**段落で書かない。** 本文は箇条書きが主で、`<p>` を使ってよいのは 1 文で終わるときだけ。
2 文以上並べたくなったら `<ul>` にする。

### 形

内容を一言で表す親項目を立て、その下のサブ項目に文を 1 文ずつ置く。

```html
<ul>
  <li>API から作れる callout は 1 種類だけ
    <ul>
      <li>toggle 付きと見出し付きは UI 専用で、API に型が無い。</li>
      <li>作れるのは通常の callout に限る。</li>
      <li>子ブロックは作成時にしか渡せない。</li>
    </ul>
  </li>
</ul>
```

- 親項目 = 名詞句。文ではないので `。` を付けない。ここだけ拾って読めるようにする
- 子項目 = **1 文 1 項目**。`。` で切って次の項目へ送る
- 入れ子は 2 階層まで。3 階層目が要るのは、節 (`<h2>` / `<h3>`) を分ける合図
- 子項目が 1 つしか無いなら、親項目に畳んで 1 項目にしてよい

### 1 項目に 2 文入れたいとき

前の文を受けないと意味が立たない 2 文は、`<br>` で改行して 1 項目に収めてよい。

```html
<li>API では作れない。<br>UI で作ったものを読むことはできる。</li>
```

**要件は「画面で行が変わっていること」。** `。` の後ろに同じ行のまま文が続くのは不可で、
`<li>` を分けるか `<br>` を入れるかは好きな方でよい。

### 箇条書き以外の場所

| 場所 | どう書くか |
| --- | --- |
| `.note` / `.warn` / `.ok` の囲み | 1 文なら地のまま。2 文以上なら中に `<ul>` を入れる |
| 表のセル | 体言止めの断片。2 文になるならセルに収めず本文へ出す |
| 図のキャプション・SVG のラベル | 1 文まで。長くなるなら図の下に `<ul>` を置く |
| 見出し (`<h1>`〜`<h3>`)・`<summary>` | 名詞句。`。` を付けない |
| standalone モード | 同じ。CSS を自分で書いても本文の形は変えない |
| `<pre>` / diff / コード / ログ / 参照の原文 (`<blockquote>`) | 対象外。原文のまま貼る |

### 書けたら確認する

`。` の後ろに文が続いている行を洗い出す (`<pre>` と `<blockquote>` の中は飛ばす):

```sh
awk '/<pre|<blockquote/{q=1} !q && /。[^<]/{print FNR": "$0} /<\/pre>|<\/blockquote>/{q=0}' index.html
```

何も出なければ、すべての `。` が要素か `<br>` の直前で終わっている。
`。」` `。）` のような閉じ括弧は誤検出なので、出た行は目で見て判断する。

## 使えるクラス

| クラス | 用途 |
| --- | --- |
| `.note` | 補足の囲み（青） |
| `.warn` | 注意の囲み（橙） |
| `.ok` | 良い結果の囲み（緑） |
| `.cols` | 自動で折り返す段組 |
| `.cards` | カードを並べる |
| `.metric` | 大きい数字 |
| `.scroll` | 横に長いものを収める |
| `.diff` | diff。`<pre class="diff">` に生のまま貼る（下記） |

```html
<h1>調査結果</h1>

<div class="note">前提: API v2 を対象にした。</div>

<h2>比較</h2>
<div class="scroll">
<table>
  <tr><th>案</th><th>コスト</th><th>備考</th></tr>
  <tr><td>A</td><td>低</td><td>推奨</td></tr>
</table>
</div>

<div class="cards">
  <div><div class="metric">3</div>種類ある</div>
  <div><div class="metric">2</div>API から作れない</div>
</div>

<h2>分かったこと</h2>
<ul>
  <li>API から作れるのは 1 種類だけ
    <ul>
      <li>残り 2 つは UI 専用で、API に型が無い。</li>
      <li>読むことはできる。</li>
    </ul>
  </li>
</ul>

<div class="warn">callout の子ブロックは後から追加できない。</div>
```

## 書いてはいけないもの

- `<!doctype>` `<html>` `<head>` `<body>` — アプリが付ける
- `<script>` — CSP (`script-src`) で止まる。JS が要る表現は standalone モードへ逃がす（下記）
- 外部の CSS / 画像 / フォント — CSP で止まる。ローカル完結が前提。
  止めているディレクティブは種類ごとに違い、CSS は `style-src 'self' 'unsafe-inline'`、
  画像は `img-src 'self' data:`、フォントだけが `default-src 'self'` に落ちる

`<style>` は書いてよい。ページ固有の微調整に使う。

## diff

`<pre class="diff">` に **diff をそのまま貼る**。行頭の `+` / `-` / `@@` を見てサーバが
1 行ずつ包み、行全体に色を敷く（`+` 緑 / `-` 赤 / `@@` は accent 色 /
`diff --git`・`index`・対で並んだ `---` `+++` は muted）。

```html
<pre class="diff"><code>@@ -97,7 +97,7 @@ func readFragment(dirPath string) template.HTML {
 	b, err := os.ReadFile(filepath.Join(dirPath, "index.html"))
-	return template.HTML(b)
+	return template.HTML(colorizeDiffs(b))
 }</code></pre>
```

- **`<span>` を自分で巻かない。** 行数ぶん markup が膨らむだけで、色は変わらない
- `<` `>` `&` はいつも通りエスケープする。着色はエスケープ済みの文字列の上で行われる
- 文脈行の先頭の空白は省いてよい。`+` でも `-` でもない行は文脈行として扱われる
- 効くのは fragment だけ。standalone のページには当たらない

## 参照の原文

本文でページの外の文章を指したら、指した箇所の原文を `<details>` に入れて添える。
形は SKILL.md の手順 2 の例のとおり。畳んだまま置くので、本文の流れは邪魔しない。

### 付けるかどうか

| 本文の書き方 | 付けるか | 写す原文 |
| --- | --- | --- |
| 「README.md の『検証』節は〜を示している」 | 付ける | その節 |
| 「〜を禁じている (CLAUDE.md)」 | 付ける | 禁じている箇所 |
| 「`main.go` L72-78 で new の flag を定義する」「`server.go:21` の CSP は〜」 | 付ける | その行 |
| 「`main.go` の `run` は知らないサブコマンドを 1 行のエラーで返すだけ」 | 付ける | 返している部分のコード |
| 「README と `docs/` の設計書で書き方が食い違う」 | 付ける | 両方。`<details>` を 2 つ並べる |
| 「`gh pr view` の出力では draft」 | 付ける | 出力のうち該当する行。`<summary>` には打ったコマンドを書く |
| 読んだファイルの一覧・書き込み先のパス・打つコマンド | 付けない (中身に触れていない) | — |

### 切り出す範囲

- 根拠になっている箇所だけ。ファイルやページを丸ごと貼らない
- 途中を省くなら、省いた所に `…` だけの行を置く。つなぎの説明は書き足さない
- 同じ箇所を 2 回以上指すなら、最初の 1 回にだけ付ける
- 同じ原文がすでに本文に貼ってある (diff・コード片) なら付けない

### 入れ物

| 原文 | 入れ物 |
| --- | --- |
| コード・設定・ログ・コマンド出力・Markdown などのファイル | `<pre>`。改行も字下げもそのまま |
| Web ページ・Notion・PR や issue のコメントなど、改行に意味の無い文章 | `<blockquote>`。段落ごとに `<p>` |

- `<` `>` `&` はエスケープする
- `open` は付けない。畳んだまま置く

### 置き場所

| 指した場所 | `<details>` を置く所 |
| --- | --- |
| 箇条書きの項目 | その `<li>` の中、文の後ろ |
| `.note` などの囲み | 囲みの中の末尾 |
| 表のセル・図の中のラベル | 表・図の直下にまとめて並べる。`<summary>` にどの行・どの箱の原文かを書く |

### 原文を読めなかったら

権限が無い・ネットワークに出られない、などで原文を取れなかったら `<details>` は付けず、
その項目に「(原文は未確認)」と書く。記憶や推測で原文を埋めない。

## 画像

`dir/assets/` に置いて相対パスで参照する。

```html
<img src="assets/screenshot.png" alt="設定画面" style="max-width:100%">
```

## 図とアニメーション

図にする基準: **構造 (箱と線)・流れ (順序)・比較 (並置)・変化 (before/after)** は
文章より図で見せる。表は「並べるだけの事実」に限る。

fragment のままでできること:

- **inline SVG** — 構成図・経路図・ブランチ図はこれで描く。外部ファイル不要で CSP に
  触れない。色と文字は、下の「inline SVG の書き方」の形で `<svg>` の中に閉じて当てる
- **CSS アニメーション** — `<style>` の `transition` / `@keyframes`。点滅 (pulse)・
  出現 (fade/pop)・強調は JS なしで足りる。図に掛けるものは、その図の `<style>` に書く
- 静的な図で足りるなら PlantUML / drawio で SVG に焼いて `assets/` に置くのも可

standalone に逃がすと足せるもの (逃がす基準は下の standalone 節):

- **トグル** (before ⇄ after の切り替え)・**ステップ再生** (▶ で 1 段ずつ進む)・ホバー連動
- 対話で作った状態には URL ハッシュの deep-link を付ける (`#after`、`#step-3` など)。
  stdout の箇条書きから「この状態を見て」と直接指せる

### inline SVG の書き方

本文の図はクリックで拡大でき、拡大から「別タブで開く」で図だけを開ける。どちらも
`<svg>` 要素から下しか持って行かない (拡大は `<svg>` の複製、別タブは `<svg>` を文字列に
した単体の画像)。**図の見た目は、その `<svg>` の中だけで決め切る。** 形はこれ:

```html
<figure>
<svg viewBox="0 0 800 300" width="800" height="300" style="max-width:100%;height:auto"
     role="img" aria-label="図が言っていることを 1 文で">
  <style>
    @scope {
      :scope {
        color-scheme: light dark;
        --bg: #ffffff; --fg: #1a1a1a; --muted: #6b6b6b; --line: #e2e2e2;
        --card: #fafafa; --accent: #2563eb;
        --ok-bg: #f0fdf4; --ok-line: #86efac; --warn-bg: #fff7ed; --warn-line: #fdba74;
        --note-bg: #f0f6ff; --note-line: #93c5fd;
      }
      @media (prefers-color-scheme: dark) {
        :scope {
          --bg: #16181c; --fg: #e6e6e6; --muted: #9aa0a6; --line: #2c3036;
          --card: #1c1f24; --accent: #7aa2f7;
          --ok-bg: #13251a; --ok-line: #15803d; --warn-bg: #2a1f14; --warn-line: #b45309;
          --note-bg: #14202e; --note-line: #1d4ed8;
        }
      }
      :scope .bg   { fill: var(--bg); stroke: none; }
      :scope text  { font: 12px system-ui, "Noto Sans JP", sans-serif; fill: var(--fg); }
      :scope .b    { font-weight: 700; }
      :scope .sub  { font-size: 10.5px; fill: var(--muted); }
      :scope .mono { font-family: ui-monospace, monospace; }
      :scope .box  { fill: var(--card); stroke: var(--line); stroke-width: 1.2; }
      :scope .edge { fill: none; stroke: var(--muted); stroke-width: 1.4; }
    }
  </style>
  <rect class="bg" width="800" height="300"/>
  <rect class="box" x="20" y="40" width="160" height="56" rx="6"/>
  <text class="b" x="36" y="64">API</text>
  <text class="sub" x="36" y="82">handlers/</text>
</svg>
<figcaption>図が言っていること</figcaption>
</figure>
```

| 形 | そうする理由 |
| --- | --- |
| `<style>` を `<svg>` の中に置く | 外の `<style>` は別タブの画像に入らない。外側の入れ物で絞った指定 (`.fig text{…}` など) は拡大の複製にも当たらず、箱が黒く塗りつぶされる |
| `@scope { }` で包む | `<svg>` の中の `<style>` も、そのままではページ全体に効いて他の図へ漏れる。prelude 無しの `@scope` は、その `<svg>` の中だけに効く |
| selector に `:scope` を付ける | 詳細度が 1 クラス分上がり、外の `.fig text{…}` と同点以上になる。同点なら `@scope` の側が勝つ |
| 配色を `:scope` に書き、ダークを `@media` で上書きする | ページの `--fg` などは別タブの画像に無い。値は共通 CSS と同じ |
| `:scope` に `color-scheme: light dark` を書く | 別タブでは `<svg>` が文書の根になり、図の外側の地もダークで暗くなる。書かないと、ダークの図が白い余白に囲まれる |
| 先頭に `.bg` の rect を敷く | 拡大の暗幕の上でも、別タブの白地の上でも、本文と同じ地の色で見える |
| 見た目は class で変える | `:scope text` や `:scope .box` は要素の属性 (`fill=` `font-size=` `font-weight=`) に勝つので、属性で色や大きさを変えても効かない。属性に書くのは形 (`x` `y` `width` `d`) だけ |

- 図に掛けるアニメーションも同じ `@scope` の中に書く。`@keyframes` の名前はページ全体で
  共有されるので、図ごとに違う名前にする
- 外の `<style>` に書くのは、図の置き場所 (`figure` の余白・`figcaption` の体裁) だけ
- `@scope` は Chrome 118 / Safari 17.4 / Firefox 146 以降。それより古いブラウザでは中身ごと
  無視され、図が SVG の初期値 (黒の塗り) で描かれる

### 描き方は他の skill を引く

描き方そのものはここに書かない。手元にその種類の図を描く skill があれば、手が止まったら
読む。無ければ、上の「inline SVG の書き方」の形で inline SVG を描く。
「描けないから文章で済ませる」が図の減る一番の原因なので、先にこの表を見ること。

| やりたいこと | 引く skill の種類 |
|---|---|
| inline SVG の実装 (viewBox・矢印・文字の大きさ)、その図が主張に値するかの判断 | inline SVG で図を描く skill |
| 数値をグラフにする (棒・線・散布・ヒートマップ・凡例・軸・配色) | 数値をグラフにする skill |
| シーケンス図・クラス図・ER 図・コンポーネント図・状態遷移図を `.puml` から焼く | PlantUML で図を SVG に焼く skill |
| ネットワーク構成図・アーキテクチャ図を drawio で描く | draw.io で図を SVG に焼く skill |
| シーケンス図・状態遷移図を「1 手ずつ送れる図」にする | 1 手ずつ送れる図を作る skill |
| 構成の変更を before-after のアニメーションで見せる | before-after のアニメーションを作る skill |

Artifact 向けに書かれた図の skill を読むときも、色と `<style>` の置き場所は上の
「inline SVG の書き方」を優先する。あちらの「`<svg>` に `<style>` を置かない」
「色は `currentColor`」の類は採らない。

1 手ずつ送れる図や before-after のアニメーションを作る skill は HTML を吐くことが多く、
standalone と相性が良い。PlantUML / draw.io の skill は SVG に焼いて `assets/` に置き、
fragment から `<img>` で参照する。

### 文字のはみ出しを目で見る

SVG に文字を置くと幅の見積もりがズレて**衝突・はみ出しが頻発する**。書いたら headless
chromium 等でスクショを撮って自分の目で確認してから返す:

```sh
chromium --headless --disable-gpu --window-size=1200,3000 --hide-scrollbars \
  --screenshot=check.png "$URL"
```

(snap 版 chromium は `/tmp` と dot-dir に書けない。`~/tmp` 等の非隠しディレクトリで実行する)

## standalone モード（完全な HTML）

フラグメントで表現できないと判断したときだけ使う。`index.html` を**完全な HTML 文書**
として書き、アプリはページの URL でそれをそのまま返す。共通 CSS もナビもパンくずも
被さらない — **そのページはブラウザいっぱいに開くただの HTML** になる。

```sh
cc-pages new --session "$CLAUDE_CODE_SESSION_ID" --title "..." --mode standalone
```

```html
<!doctype html>
<html lang="ja">
<head><meta charset="utf-8"><title>...</title>
<style>/* 全部自分で書く */</style></head>
<body>
  <p><a href="../">← このセッションのページ一覧</a></p>
  ...
  <script>/* インラインで書く */</script>
</body>
</html>
```

**先頭に `<a href="../">` を置くこと。** ナビが被さらないので、これを書かないと
戻る導線がブラウザの戻るしかなくなる。`../` はそのセッションのページ一覧に当たる。

### 何が変わるか

| | fragment（既定） | standalone |
| --- | --- | --- |
| 書くもの | `<body>` の中身だけ | 完全な HTML 文書 |
| CSS | アプリが持つ。書かなくていい | **アプリの CSS は当たらない。全部自分で書く** |
| `<script>` | 不可 | インラインで書ける |
| クラス（`.note` 等） | 使える | 使えない（自分で定義するなら別） |
| 幅 | 52rem の本文枠 | 制限なし。全部自分で決める |
| ナビ・パンくず・前後ページ | アプリが出す | **無い。`../` を自分で書く** |
| `<title>` | page.json のタイトル | 文書に書いたものがそのまま出る |

`.note` や `.cards` が使えなくなり、色・余白・ダークモード（`prefers-color-scheme`）まで
自分で書くことになる。**既定は fragment のまま。** 表・囲み・カード・画像で足りるものを
standalone にしない。

### 逃がす基準

- 並べ替え・絞り込みのできる表、値を動かせる試算、対話的な図
- 52rem の本文枠に収まらない、横に広いレイアウト

逆に、静的な図で足りるなら standalone は要らない。PlantUML / draw.io で SVG に焼いて
`assets/` に置き、フラグメントから `<img>` で貼る方が安く読みやすい。

### standalone でも変わらない制約

- **外部オリジンは CSP で止まる。** CDN から chart.js や mermaid を読むことはできない。
  JS はインラインで自分で書く
- `'unsafe-eval'` も開いていない。`eval` / `new Function` に依存するライブラリは動かない
- 画像などは今まで通り `assets/` に置いて相対パスで参照する（解決先は fragment と同じ）
- `index.html` を書く前にページを開くと、standalone でも「まだ書かれていません」の
  プレースホルダが chrome 付きで出る。書けば次のリロードから文書に変わる
~~~~~

Run: `wc -l plugins/cc-pages/skills/cc-page/SKILL.md plugins/cc-pages/skills/cc-page/references/fragment-style.md`
Expected: `211 …/SKILL.md` と `365 …/fragment-style.md`

- [ ] **Step 3: README から写した 3 行を確かめる**

SKILL.md の原文の例が写した README の 3 行が、README に今もそのままあることを確かめる:

```bash
grep -nF '`status` はセッション直下の `status.json` を丸ごと書き換え（全置換）、セッションの' README.md
grep -nF 'ページ一覧 `/p/<session>/` の上に「現在の状況」「次やること」として出す。`--now` と' README.md
grep -nF '`--next` は繰り返し指定でき、どちらか 1 つは要る。ページより先に打ってもよい。' README.md
```

Expected: 3 行が続き番号で出る (`20:` `21:` `22:`)

- [ ] **Step 4: plugin として読み込めることを確かめる**

Run: `DISABLE_AUTOUPDATER=1 claude plugin validate plugins/cc-pages; echo "exit=$?"`
Expected: 警告は `version: No version specified` の 1 つだけ、`✔ Validation passed with warnings`、`exit=0`。SKILL.md の frontmatter が壊れていれば、ここで `frontmatter:` のエラーと `exit=1` になる

Run: `DISABLE_AUTOUPDATER=1 claude --plugin-dir ./plugins/cc-pages plugin details cc-pages`
Expected: `Component inventory` の下に `Skills (1)  cc-page`。model は呼ばれない

- [ ] **Step 5: Commit**

2 つのファイルを 1 つの commit で足す。途中の状態は commit しない。

```bash
git add plugins/cc-pages/skills/cc-page
git commit -m "$(cat <<'EOF'
feat(plugin): 見本の skill cc-page を足す

長い応答を cc-pages のページに書き出し、ターミナルには短い箇条書きと
リンクだけを返す skill。作者の手元の skill を元に、個人の環境に
結びついた部分を外して書き直した。

new や status が知らない flag・サブコマンドで落ちたら、binary が skill
より古いとみなし、build し直すよう 1 行添える。原文の例は cc-pages の
main.go と README から取り、図を描く skill は名指しせず種類で書いた。

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 4: 契約テスト — skill の flag とクラスを CLI と CSS に突き合わせる

**Files:**
- Create: `skill_test.go`

**Interfaces:**
- Consumes: `newFlags` / `statusFlags` (Task 1)、`plugins/cc-pages/skills/*/SKILL.md` と `plugins/cc-pages/skills/*/references/*.md` (Task 3)、`internal/web/static/style.css`
- Produces: テストだけ。`go test ./...` (go.yml) で回る。テスト用の宣言 `skillCommand` / `skillCommands` / `parseCommand` / `knownFlagSets` / `commandErrors` / `tableClasses` / `cssSelectorClasses` / `skillFiles` は package main の test にだけある

- [ ] **Step 1: 失敗するテストを書く**

`skill_test.go` を作る (宣言はまだ書かない):

```go
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
		"pre.diff .a { background: var(--diff-add-bg); }",
		".note, .warn { border-left: 4px solid; }",
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
```

- [ ] **Step 2: 落ちることを確かめる**

Run: `go test -run 'TestSkill|TestCommandErrors|TestTableClasses|TestCSSSelectorClasses' .`
Expected: FAIL (`[build failed]`)。`undefined: skillCommand`・`undefined: skillCommands`・`undefined: knownFlagSets` などが並ぶ

- [ ] **Step 3: テスト用の宣言を書く**

`skill_test.go` の import の閉じ括弧 `)` の次の空行のあと (`func TestSkillCommandsParse` の前) に足す:

```go
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

// cssSelectorClasses は CSS の selector に現れるクラス名の集合を返す。
//
// コメントと宣言の中は見ない。style.css のコメントは「.cards と同じ見た目」のように
// クラス名を挙げるので、全文を検索すると、消えたクラスを見逃す。
func cssSelectorClasses(css string) map[string]bool {
	css = cssComment.ReplaceAllString(css, "")
	classes := map[string]bool{}
	start := 0
	for i, r := range css {
		switch r {
		case '{':
			for _, m := range cssClass.FindAllStringSubmatch(css[start:i], -1) {
				classes[m[1]] = true
			}
			start = i + 1
		case '}', ';':
			start = i + 1
		}
	}
	return classes
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

```

`cssSelectorClasses` は、`{` の直前の `}` / `;` / `{` から `{` までを selector と見なす。`@media (…) {` もここに入るが、`.` の後が英字で始まらない値 (`40.5rem` など) はクラスにならない。`cssClass` が `-` と英数字を貪欲に取るので、`.diff-add` は `diff-add` になり、`diff` には数えない。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test -run 'TestSkill|TestCommandErrors|TestTableClasses|TestCSSSelectorClasses' -v .`
Expected: PASS。`TestSkillCommandsParse`・`TestCommandErrors` (subtest 7 つ)・`TestTableClasses`・`TestCSSSelectorClasses`・`TestSkillCommandsMatchCLI`・`TestSkillClassesExistInCSS`

- [ ] **Step 5: 壊すと落ちることを確かめる**

1 つずつ壊し、落ちるのを見てから戻す (戻すのは commit 済みのファイルなので `git checkout --` でよい):

```bash
sed -i 's/--summary "一覧カードに出る 1 行"/--sumary "一覧カードに出る 1 行"/' plugins/cc-pages/skills/cc-page/SKILL.md
go test -run TestSkillCommandsMatchCLI .
git checkout -- plugins/cc-pages/skills/cc-page/SKILL.md
```

Expected: FAIL。`plugins/cc-pages/skills/cc-page/SKILL.md L24: cc-pages new に --sumary は無い`

```bash
sed -i 's/^| `\.note` |/| `.notes` |/' plugins/cc-pages/skills/cc-page/references/fragment-style.md
go test -run TestSkillClassesExistInCSS .
git checkout -- plugins/cc-pages/skills/cc-page/references/fragment-style.md
```

Expected: FAIL。`… fragment-style.md の .notes が style.css の selector に無い`

```bash
sed -i '/^\.cards {/d; /^\.cards > \*/d' internal/web/static/style.css
go test -run TestSkillClassesExistInCSS .
git checkout -- internal/web/static/style.css
```

Expected: FAIL。`… fragment-style.md の .cards が style.css の selector に無い`。style.css の L51 のコメント「枠は .cards と同じ見た目に揃える」が残っていても落ちる

Run: `git status --short`
Expected: `?? skill_test.go` だけ

- [ ] **Step 6: 全体を通す**

Run: `gofmt -l . && go vet ./... && go test ./...`
Expected: gofmt の出力なし、全 package が ok

- [ ] **Step 7: Commit**

```bash
git add skill_test.go
git commit -m "$(cat <<'EOF'
test(plugin): skill のコマンド例と使えるクラスを CLI と CSS に突き合わせる

skill は GitHub の main から配られ、利用者の binary とは別に更新される。
CLI に無い flag を書くと利用者の手元で new が落ち、CSS に無いクラスを
勧めると見た目が黙って崩れるので、go test で落とす。

flag は fenced code block の cc-pages で始まる行だけを見る。本文の inline
code は CSS の変数 (--fg など) と区別できないので見ない。

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 5: README — 「Claude Code に使わせる」節

**Files:**
- Modify: `README.md` (「## e2e」の直前に節を足し、末尾に設計書の案内を足す)

**Interfaces:**
- Consumes: Task 2 の marketplace / plugin の名前と `.github/workflows/plugin.yml`、Task 3 の `plugins/cc-pages/skills/cc-page/`、Task 4 の `skill_test.go`
- Produces: なし。SKILL.md の原文の例が写す README の L20-22 は変えない

- [ ] **Step 1: 節を足す**

`README.md` の `## e2e` の行の直前 (「使い方」の最後の段落「フラグメントの `<pre class="diff">` は、…」の後の空行の次) に足す:

```markdown
## Claude Code に使わせる

この repo は Claude Code の plugin の marketplace も兼ねる。plugin `cc-pages` を入れると、
skill `cc-page` が長い応答をページに書き出し、ターミナルには短い箇条書きとリンクだけを返す。

    /plugin install cc-pages --marketplace pollenjp/cc-pages

この 1 行で入るのは Claude Code 2.1.275 以降。それより古い版では、marketplace の追加と
install を分ける。シェルからは `claude plugin marketplace add pollenjp/cc-pages` と
`claude plugin install cc-pages@cc-pages` の 2 つ。

    /plugin marketplace add pollenjp/cc-pages
    /plugin install cc-pages@cc-pages

skill は依頼の中身を見て自分で動く。明示して呼ぶときは `/cc-pages:cc-page`。
`cc-pages` の binary が PATH に無いあいだは、ページを作らずにターミナルで答える。

自前の marketplace は自動更新が既定で off なので、更新は手で取りに行く。効くのは
`/reload-plugins` か再起動のあと。

    claude plugin marketplace update cc-pages
    claude plugin update cc-pages@cc-pages

skill は main を追うので、binary も pull して build し直す。binary の方が古いと、skill は
ページを作らずに答え、build し直すよう 1 行添える。

skill は図を多めに描くよう指示している。図を描く skill を一緒に入れておくと、描ける図の
種類が増える。PlantUML や draw.io で図を SVG に焼く skill、数値をグラフにする skill、
1 手ずつ送れる図を作る skill などが合う。無ければ inline SVG で描く。

自分用に直したいときは、`plugins/cc-pages/skills/cc-page/` を `~/.claude/skills/<名前>/` に
写し、`SKILL.md` の `name` もその名前に変えて、plugin は外す
(`claude plugin uninstall cc-pages@cc-pages`)。両方を残すと、同じ場面で 2 つが起動する。

skill を直すときは、`claude --plugin-dir ./plugins/cc-pages` で 1 セッションだけ読み込んで
試せる。検査は repo 直下と plugin のディレクトリの 2 か所で流す。marketplace の検査は
skill のファイルを開かないため。CI (`.github/workflows/plugin.yml`) も同じ 2 つを流す。

    claude plugin validate .
    claude plugin validate plugins/cc-pages

CLI の flag・サブコマンドや fragment のクラスを変えるときは、同じ PR で skill も直す。
`skill_test.go` が、skill のコマンド例の flag と「使えるクラス」の表を、CLI と CSS に
突き合わせる。

```

節の最後に空行を 1 つ残し、`## e2e` との間を空ける。

- [ ] **Step 2: 設計書の案内を足す**

`README.md` の末尾の 2 行 (`状況板 (`cc-pages status`) の設計はこの repo の` と `` `docs/superpowers/specs/2026-09-22-session-status-design.md` にある。``) の後に、空行を挟んで足す:

```markdown
plugin と skill (`plugins/cc-pages/`) の設計はこの repo の
`docs/superpowers/specs/2026-10-10-sample-skill-plugin-design.md` にある。
```

- [ ] **Step 3: 確かめる**

Run: `DISABLE_AUTOUPDATER=1 claude plugin validate .; echo "exit=$?"`
Expected: 警告は version の 1 つだけで、`ℹ Advice` の塊 (README に install の行が無い) が出ない。`exit=0`

SKILL.md の原文の例が写した 3 行が、まだ L20-22 にあることを確かめる:

```bash
grep -nF '`status` はセッション直下の `status.json` を丸ごと書き換え（全置換）、セッションの' README.md
grep -nF 'ページ一覧 `/p/<session>/` の上に「現在の状況」「次やること」として出す。`--now` と' README.md
grep -nF '`--next` は繰り返し指定でき、どちらか 1 つは要る。ページより先に打ってもよい。' README.md
```

Expected: `20:` `21:` `22:`

Run: `git diff --stat`
Expected: `README.md | 48 +` 前後 (足すだけで、消す行が無い)

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "$(cat <<'EOF'
docs(readme): cc-pages を Claude Code に使わせる入れ方を書く

plugin の入れ方・更新の仕方・図を描く skill との併用・自分用に直す方法・
skill を直す人向けの検査を書いた。図の skill は種類だけを挙げる。

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 6: 振る舞い — subagent に skill を使わせて確かめる

**Files:**
- なし。結果は Dev Tracker のチケットと PR の本文に残す。直すことになったら Task 3 のファイル

**Interfaces:**
- Consumes: この branch の binary (`$SCRATCH/bin/cc-pages` を build し直す)、Task 3 の skill (`plugins/cc-pages/skills/cc-page/SKILL.md`)
- Produces: 5 つの場面の結果

| | PATH の先頭の cc-pages | 依頼 | 期待すること |
| --- | --- | --- | --- |
| S1 | この branch の build | 比較 | ページを書く。page.json の summary が空でない。index.html の `<svg` が 3 つ以上で、`@scope` が `<svg` と同じ数以上。awk の確認が何も出さない。status.json がある。返答は 3 行までの箇条書きと URL |
| S2 | 同上 | yes/no | ページを書かない |
| S3 | `--summary` を知らない偽物 | 比較 | ページを書かない。ターミナルで比較に答え、最後に build し直す 1 行を添える |
| S4 | `status` を知らない偽物 | 比較 | ページを書き、URL を返す。status.json は無い。build し直す 1 行を添える |
| S5 | 無い | 比較 | ページを書かない。ターミナルで答える。binary を探すコマンドを打たない |

- [ ] **Step 1: 場を作る**

```bash
SCRATCH=/path/to/scratchpad/tkt130
mkdir -p "$SCRATCH/bin" "$SCRATCH/old-summary" "$SCRATCH/no-status" "$SCRATCH/work"
go build -o "$SCRATCH/bin/cc-pages" .

cat > "$SCRATCH/old-summary/cc-pages" <<EOF
#!/bin/sh
# --summary を知らない頃の binary を真似る。Go の flag と同じ文言で落ちる。
for a in "\$@"; do
  if [ "\$a" = --summary ]; then
    echo "flag provided but not defined: -summary" >&2
    echo "Usage of new:" >&2
    echo "cc-pages: flag provided but not defined: -summary" >&2
    exit 1
  fi
done
exec "$SCRATCH/bin/cc-pages" "\$@"
EOF

cat > "$SCRATCH/no-status/cc-pages" <<EOF
#!/bin/sh
# status を知らない頃の binary を真似る。new などは本物に渡す。
if [ "\$1" = status ]; then
  echo "cc-pages: 不明なサブコマンド: status" >&2
  exit 1
fi
exec "$SCRATCH/bin/cc-pages" "\$@"
EOF

chmod +x "$SCRATCH/old-summary/cc-pages" "$SCRATCH/no-status/cc-pages"
printf %s "$PATH" | tr ':' '\n' | grep -vx "$HOME/bin" | paste -sd: - > "$SCRATCH/nobin-path"
env PATH="$(cat "$SCRATCH/nobin-path")" sh -c 'command -v cc-pages'; echo "exit=$?"
```

Expected: 最後が `exit=1` (`nobin-path` の PATH では cc-pages が見えない)。パスが出たら、そのディレクトリも `nobin-path` から外す

`--summary` を知らない偽物は、`--summary` を外して打ち直せば本物に通る。skill を守らずにページを作ったら、S3 でページが残るので分かる。

判定に使う script を `$SCRATCH/check.sh` に書く:

```sh
#!/bin/sh
# 使い方: sh check.sh <root>。その root に書かれたページを数える。
root="$1"
page=$(find "$root" -name page.json 2>/dev/null | head -n 1)
if [ -z "$page" ]; then
  echo "page: 無し"
  exit 0
fi
dir=$(dirname "$page")
echo "page: $dir"
echo "summary: $(jq -r '.summary // "(空)"' "$page")"
echo "svg: $(grep -o '<svg' "$dir/index.html" | wc -l)"
echo "@scope: $(grep -o '@scope' "$dir/index.html" | wc -l)"
echo "awk (。の後に文が続く行。空なら良い):"
awk '/<pre|<blockquote/{q=1} !q && /。[^<]/{print FNR": "$0} /<\/pre>|<\/blockquote>/{q=0}' "$dir/index.html"
echo "status.json: $(find "$root" -name status.json | wc -l)"
```

- [ ] **Step 2: 5 つの subagent を同時に立てる**

1 つのメッセージで Agent tool を 5 回呼ぶ (`subagent_type: general-purpose`、description は `cc-page 振る舞い S1` など)。prompt は次の雛形の `{依頼}`・`{SKILL}`・`{PREFIX}` を埋めたもの:

```text
あなたは Claude Code として、ユーザーから次の依頼を受けた。

<依頼>
{依頼}
</依頼>

応答の仕方は、次の skill に従う。最初にこのファイルを読み、必要になったら同じディレクトリの references/ も読む。
{SKILL}

ほかの skill は読まない・使わない。手元に入っている、ページを書く別の skill も使わない。

この環境の決まり (テストのための設定で、依頼とは関係ない):
- Bash でコマンドを打つときは毎回、先頭に次を付ける。
  {PREFIX}
- ファイルを書いてよいのは、cc-pages new が返す dir の中だけ。

最後のメッセージは、ユーザーのターミナルに出る応答そのものとして書く。
そのあとに「=== 記録 ===」の行を置き、このやり取りで打った Bash のコマンドを順にすべて並べる (この記録はユーザーには見えない)。
```

- `{SKILL}`: この worktree の `plugins/cc-pages/skills/cc-page/SKILL.md` の絶対パス
- `{依頼}`:
  - 比較 (S1・S3・S4・S5): `Go で小さな CLI を書く。flag の扱い方を 3 通り (標準の flag パッケージ・spf13/pflag・spf13/cobra) 比べて、どれを選ぶべきか教えて。`
  - yes/no (S2): `Go の標準の flag パッケージは、-name と --name の両方を受け付ける？ yes か no で。`
- `{PREFIX}` (`$SCRATCH` と `<nobin-path の中身>` は、絶対パスに展開してから prompt に入れる):

| | `{PREFIX}` |
| --- | --- |
| S1 | `cd '$SCRATCH/work' && export PATH='$SCRATCH/bin':"$PATH" CC_PAGES_ROOT='$SCRATCH/root-s1' CC_PAGES_ADDR=127.0.0.1:7790;` |
| S2 | `cd '$SCRATCH/work' && export PATH='$SCRATCH/bin':"$PATH" CC_PAGES_ROOT='$SCRATCH/root-s2' CC_PAGES_ADDR=127.0.0.1:7790;` |
| S3 | `cd '$SCRATCH/work' && export PATH='$SCRATCH/old-summary':"$PATH" CC_PAGES_ROOT='$SCRATCH/root-s3' CC_PAGES_ADDR=127.0.0.1:7790;` |
| S4 | `cd '$SCRATCH/work' && export PATH='$SCRATCH/no-status':"$PATH" CC_PAGES_ROOT='$SCRATCH/root-s4' CC_PAGES_ADDR=127.0.0.1:7790;` |
| S5 | `cd '$SCRATCH/work' && export PATH='<nobin-path の中身>' CC_PAGES_ROOT='$SCRATCH/root-s5' CC_PAGES_ADDR=127.0.0.1:7790;` |

`CC_PAGES_ROOT` で、手元の普段のページ (`~/.local/share/cc-pages`) に混ぜない。`CC_PAGES_ADDR` は Step 4 で S1 のページを開くポートに合わせる。

- [ ] **Step 3: 判定する**

場面ごとに `sh "$SCRATCH/check.sh" "$SCRATCH/root-sN"` を打ち、subagent の最後のメッセージと合わせて見る:

| | 合格の条件 |
| --- | --- |
| S1 | check.sh: page あり・summary が空でない・svg が 3 以上・@scope が svg 以上・awk の下が空・status.json が 1。返答: 「=== 記録 ===」より上が、`- ` で始まる 3 行までと、`http://localhost:7790/p/` で始まる URL |
| S2 | check.sh: `page: 無し`。返答に yes か no の答えがある |
| S3 | check.sh: `page: 無し`。返答に比較の答えがあり、最後に「pull して build し直す」を含む 1 行がある |
| S4 | check.sh: page あり・status.json が 0。返答にページの URL と「pull して build し直す」を含む 1 行がある |
| S5 | check.sh: `page: 無し`。返答に比較の答えがある。記録に、binary を探すコマンド (`~/go/bin`・`~/.local/bin`・`/proc`・`./cc-pages`・`go build`・`go install`・`find`・`which -a`) が無い |

- [ ] **Step 4: S1 のページを目で見る**

Bash の `run_in_background` で、S1 の root を配る:

```bash
SCRATCH=/path/to/scratchpad/tkt130
CC_PAGES_ROOT="$SCRATCH/root-s1" CC_PAGES_ADDR=127.0.0.1:7790 "$SCRATCH/bin/cc-pages" serve
```

起動を待って、スクリーンショットを撮る (`<S1 の URL>` は S1 の返答の URL):

```bash
SCRATCH=/path/to/scratchpad/tkt130
timeout 30 sh -c 'until curl -fsS -o /dev/null http://127.0.0.1:7790/; do sleep 0.5; done'
nix develop . --command chromium --headless --no-sandbox --disable-gpu --window-size=1200,3000 \
  --hide-scrollbars --screenshot="$SCRATCH/s1.png" "<S1 の URL>"
```

Read tool で `$SCRATCH/s1.png` を開き、図の文字が箱からはみ出していない・重なっていない・黒く塗りつぶされた図が無いことを見る。見終えたら止める:

```bash
SCRATCH=/path/to/scratchpad/tkt130
pkill -f "$SCRATCH/bin/cc-pages serve"
```

- [ ] **Step 5: 落ちた場面があったら**

- 原因が skill の書き方なら、Task 3 のファイルを直し、`go test ./...` と `claude plugin validate plugins/cc-pages` を通してから、その場面だけ新しい subagent で流し直す。commit は `fix(plugin): <直したこと>`
- 原因が subagent の取り違え (手元の別の skill を読んだ・PREFIX を付け忘れた) なら、同じ prompt でもう 1 回だけ流す。2 回続けて同じ外れ方なら、skill の問題として扱う
- S1 の図の見た目だけが崩れていて、skill の規則 (inline SVG の書き方) どおりに書かれているなら、skill の問題ではない。結果に書き留めるだけにする

- [ ] **Step 6: 結果を残す**

場面ごとに 1 文で、チケットにメモを残す:

```bash
ticket.sh note TKT-130 "振る舞いの確認 S1 (比較): <結果>"
```

S2〜S5 も同じ形で 1 つずつ。直した箇所があれば、その commit も 1 文で書く。

---

## 仕上げ

全 task の後に、この worktree で:

```bash
gofmt -l . && go vet ./... && go test ./...
DISABLE_AUTOUPDATER=1 claude plugin validate . && DISABLE_AUTOUPDATER=1 claude plugin validate plugins/cc-pages
nix shell --inputs-from . nixpkgs#actionlint --command actionlint
git status --short
git log --oneline main..HEAD
```

Expected: gofmt の出力なし、全 package が ok。validate は 2 つとも version の警告だけで passed。actionlint の出力なし。作業ツリーは clean。log に spec と plan の commit、Task 1〜5 の commit (Task 6 で直したならその fix も) が並ぶ。

元にした skill の文面が、この branch のどの commit にも入っていないことを確かめる。照らし合わせる一覧は scratch の `$SCRATCH/personal.txt` に置き、repo には入れない。一覧の中身は、元の文面のうち書き直しで外した行と、作者の skill・repo・ファイルの名前 (1 行に 1 つ。空行を入れない):

```bash
SCRATCH=/path/to/scratchpad/tkt130
git log -p --format=%B main..HEAD | grep -nF -f "$SCRATCH/personal.txt"; echo "exit=$?"
```

Expected: 何も出ずに `exit=1`。PR の本文も、出す前に同じ一覧で確かめる

そのうえで superpowers:finishing-a-development-branch に進む。PR を出すなら:

- タイトルは `TKT-130 見本の skill を plugin で同梱する`
- 本文の 1 行目はチケットの URL (`ticket.sh show TKT-130 --url`)。続けて変更の要約、この plan の「spec からの差分」、確かめたこと (go test・validate・Task 6 の 5 場面の結果)。末尾は `🤖 Generated with [Claude Code](https://claude.com/claude-code)`
- 出したら `ticket.sh pr TKT-130` と `ticket.sh link TKT-130 <PR の URL> "PR"`
- spec の「後で」にある `claude plugin eval` の suite は、別チケットに切る (`ticket.sh new … --type Spike`。背景は spec の却下した案の表の eval の行)
- merge したら `ticket.sh sync`。Task 1 は挙動を変えないので、手元の `~/bin/cc-pages` を build し直す必要は無い
