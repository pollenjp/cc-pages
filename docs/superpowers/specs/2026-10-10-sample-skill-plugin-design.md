# 見本の skill を Claude Code の plugin として同梱する — `plugins/cc-pages` と marketplace

- 日付: 2026-10-10
- 状態: 設計確定（実装計画はこれから）
- チケット: TKT-130 <https://app.notion.com/p/cc-pages-skill-repo-3f579149a66f81238c2fd0f3fbc278b9>
- 前提の設計書: `claude-skills/docs/superpowers/specs/2026-08-29-cc-pages-design.md`
  （skill とアプリをつなぐのは CLI 一本・退行の原則はそちらに従う。この文書はその差分）

## 背景

cc-pages は public な repo だが、Claude Code にページを書かせる skill は作者の手元（private）に
しかない。README を読んでも、Claude Code に cc-pages を使わせる方法が分からない。

## 目的

1. cc-pages を使いこなすための見本の skill を、この repo から Claude Code の plugin として配る
2. 利用者は 2 コマンドで入れられ、入れれば長い応答がページになる
3. skill に書いた CLI の使い方と、実際の CLI とのずれを CI で落とす

## 決定事項

| 論点 | 決定 |
| --- | --- |
| 作者の手元の skill との関係 | 独立した 2 本立て。手元の skill は変えない。見本は、個人に結びついた部分を外した版 |
| 中身 | cc-pages の機能に直結する規則と、4 つの作法（図を多めに描く・md より先にページ・参照の原文を添える・箇条書き主体の刻み方） |
| 配り方 | Claude Code の plugin。repo 直下に marketplace、`plugins/cc-pages/` に plugin |
| 名前 | marketplace `cc-pages`、plugin `cc-pages`、skill `cc-page`（明示して呼ぶときは `/cc-pages:cc-page`） |
| 版 | `version` を書かない。版は plugin のディレクトリの commit SHA になる |
| binary とのずれ | `new` / `status` が知らない flag やサブコマンドで失敗したら、skill は 1 行で binary の更新を促し、ターミナルで答える |
| 確かめ方 | Go の契約テストと `claude plugin validate` を CI で回す。振る舞いは実装時に subagent で確かめる |
| `claude plugin eval` | 後回し（別チケット） |
| README | 「Claude Code に使わせる」節を足す。図を描く skill は、種類だけを書いて併用を勧める |

### 却下した案

| 案 | 却下理由 |
| --- | --- |
| 見本に一本化し、手元の skill を畳む | ユーザーが 2 本立てを選んだ。個人の好みは private 側に残す |
| CLI の叩き方だけを書いた最小限の雛形 | 「使いこなす」に届かない |
| `skills/cc-page/` を置き、README で `~/.claude/skills/` への symlink を案内する | binary と同じ checkout から読めて版がそろう利点はあった。ユーザーが plugin を選んだ。名前空間が付かず、利用者の手元の skill とぶつかりうる |
| plugin の構成にし、symlink の入れ方も案内する | 入れ方の説明が 2 通りになり、保守が増える |
| plugin の `version` を書く | 文字列を上げたときだけ利用者に届く。上げ忘れると、直した skill が届かない。binary も版を持たず main を追っている |
| repo 直下を plugin にする（`"source": "./"`） | install で Go のソースや e2e の fixture まで cache に写る |
| plugin の `bin/` に binary を同梱する | OS・arch ごとの build と、binary の commit が要る |
| `claude plugin eval` の suite を今回入れる | eval の Bash は sandbox で動き、home 配下を読めず、書けるのは run の作業場所だけになる。`~/bin/cc-pages` を呼べず、データの置き場所も向け直す必要があり、下調べが要る |
| README で図の skill を名指しする | ユーザーが種類だけを書くことを選んだ |

## 構成

```
.claude-plugin/
  marketplace.json                 # 新規。repo を marketplace にする
plugins/cc-pages/
  .claude-plugin/
    plugin.json                    # 新規
  skills/cc-page/
    SKILL.md                       # 新規。作者の手元の skill を元に書き直す
    references/
      fragment-style.md            # 新規。同上
main.go                            # FlagSet を作る部分を関数に出す
skill_test.go                      # 新規。skill と CLI・CSS の契約テスト
.github/workflows/plugin.yml       # 新規。claude plugin validate
README.md                          # 「Claude Code に使わせる」節を足す
```

### `.claude-plugin/marketplace.json`

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

- `source` は `./` で始める（必須）。相対パスは marketplace の root（repo 直下）から解決される
- entry に `version` を書かない（plugin.json にも書かない）
- top-level の `description` が無いと validate が警告するので書く
- `$schema` は付けない（editor の補完用で、読み込み時は無視される）

### `plugins/cc-pages/.claude-plugin/plugin.json`

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

- `name` は marketplace の entry と同じにする
- `version` が無いので、validate は「version が無い」警告を出す（CI で `--strict` を付けない理由）
- plugin.json 自体は省けるが、置いておく。後で `claude plugin eval` を足すときに要る

### 利用者の入れ方と更新

```
/plugin marketplace add pollenjp/cc-pages
/plugin install cc-pages@cc-pages
```

- シェルからは `claude plugin marketplace add pollenjp/cc-pages` と `claude plugin install cc-pages@cc-pages`
- install で cache に写るのは `plugins/cc-pages/` だけ（`~/.claude/plugins/cache/cc-pages/cc-pages/<sha>/`）。
  repo 全体の clone は `~/.claude/plugins/marketplaces/cc-pages/` に置かれる
- 自前の marketplace は自動更新が既定で off。更新は `claude plugin marketplace update cc-pages` →
  `claude plugin update cc-pages@cc-pages` → `/reload-plugins` か再起動
- skill は main を追うので、binary も pull して build し直す

## skill の中身

作者の手元の skill（private）を元にし、個人の環境に結びついた部分（作者の skill の名前・呼び方・
private な repo のファイルを使った例）を外して書き直す。言語は日本語のまま（この repo の README と同じ）。

### `SKILL.md`

| 箇所 | 中身 |
| --- | --- |
| frontmatter の `name` | `cc-page`（ディレクトリ名とそろえる） |
| `description` | 名指しの例は「cc-page で」「cc-page に出して」「ページにして」。道具の名前を「cc-pages（ローカル web ビューア）」として出す |
| 見出し | `# cc-page` |
| 手順 1 | `cc-pages new` でページを作る。binary が PATH に無ければ使わず、ほかの場所も探さない。binary が古いときの段落を置く（下記） |
| 手順 2 の原文アコーディオン | 例は cc-pages の中から取る。指し方の例は「`main.go` L69-81 で〜」、`<details>` の例は `README.md`「使い方」の写し |
| 手順 2 の図の段落 | 図を描く skill は「手元にあれば」使うものとして書き、名指ししない |
| 使わない場面 | 名指しは「cc-page で」と頼まれたときと、`/cc-pages:cc-page` での呼び出し |
| 他 | 元の skill と同じ（手順 3 の status、手順 4 の stdout、md ファイルの節、粒度） |

手順 1 の、binary が古いときの段落（文言は実装時に整える）:

```markdown
**`new` が `flag provided but not defined` や `不明なサブコマンド` で失敗したら、cc-pages の
binary がこの skill より古い。** ページは作らずにターミナルで答え、最後に「cc-pages の binary が
この skill より古いので、pull して build し直すと cc-page が使える」と 1 行だけ添える。
手順 3 の `status` が同じ理由で失敗したときは、ページの URL は返したうえで同じ 1 行を添える。
```

### `references/fragment-style.md`

| 箇所 | 中身 |
| --- | --- |
| 「参照の原文」の「付けるかどうか」の表 | 例は cc-pages の中の題材（`main.go`・`README.md`・`docs/`）で書く |
| 「描き方は他の skill を引く」 | skill は名指しせず、図の種類の表（inline SVG の実装・数値のグラフ・PlantUML / draw.io で SVG に焼く・1 手ずつ送れる図・before-after のアニメーション）にする。「手元にその種類の skill があれば読み、無ければ inline SVG で描く」とする。Artifact 向けの図の skill を読むときも、色と `<style>` の置き場所はこの文書の「inline SVG の書き方」を優先する、と注意する |
| 他 | 元の skill と同じ（文章の刻み方・使えるクラス・diff・画像・inline SVG の書き方・文字のはみ出しの確認・standalone モード） |

## 確かめ方

### Go の契約テスト（`skill_test.go`、既存の `go.yml` の `go test ./...` で回る）

skill は GitHub の main から配られるので、CLI と食い違うと利用者の手元で `new` が失敗する。
skill が名指ししている CLI の flag と CSS のクラスが、実在することを確かめる。

- **flag**: `SKILL.md` と `references/*.md` の fenced code block から `cc-pages <sub>` で始まる
  コマンドを拾い（行末の `\` で続く行は連結する）、その中の各 `--<flag>` が `<sub>` の FlagSet に
  定義されていることを確かめる。`"…"` で囲んだ値の中は見ない。`<sub>` が `new` / `status` /
  `serve` 以外ならエラー
- 本文の inline code の flag（`` `--summary` `` など）は見ない。`fragment-style.md` は CSS の
  変数（`` `--fg` `` など）も inline code で書くので、区別できない。本文で触れる flag は
  どれもコマンドの例にも出てくるので、そちらで捕まる
- **class**: `fragment-style.md` の「使えるクラス」の表の 1 列目（`` `.note` `` など）が、
  `internal/web/static/style.css` に selector として現れることを確かめる
- それぞれ、拾えた件数が 0 なら失敗にする（パスや書式が変わって、テストが空回りするのを防ぐ）
- `main.go` は FlagSet を作る部分を `newFlags(in *store.NewPageInput) *flag.FlagSet` と
  `statusFlags(in *store.StatusInput) *flag.FlagSet` に出し、`cmdNew` / `cmdStatus` はそれを使う。
  テストはこの 2 つで FlagSet を作り、`Lookup` で引く

### `claude plugin validate`（新しい workflow `.github/workflows/plugin.yml`）

- `npx --yes @anthropic-ai/claude-code@2.1.287 plugin validate .` と、同じく
  `plugin validate plugins/cc-pages` の 2 回を流す
  - marketplace の検証は plugin の skill ファイルを開かないので、plugin のディレクトリも別に流す
- `--strict` は付けない（version が無いという警告で落ちるため）
- 版は stable の dist-tag で、公開から 7 日以上経ったもの（2026-10-01 公開の 2.1.287）に固定する
- 認証は要らない。`DISABLE_AUTOUPDATER=1` を渡す
- 起動は `go.yml` と同じく、main への push と pull_request

### 振る舞い（実装時に手元で 1 回）

subagent に見本の `SKILL.md` を読ませ、それに従って答えさせる（作者の手元の skill は使わせない）。
`CC_PAGES_ROOT` は scratchpad に向け、普段のページに混ぜない。

| 依頼 | 期待すること |
| --- | --- |
| 比較の依頼（例: Go の CLI の flag の扱い方を 3 通り比べる） | ページを書く。`page.json` に summary がある。`index.html` に図が 3 枚以上あり、inline SVG が `@scope` の形。`。` の刻み（awk）に引っかからない。`status.json` がある。stdout は 3 行までの箇条書きと URL |
| yes/no で済む質問 | ページを書かない |
| 古い binary（`new` が `flag provided but not defined` で失敗する偽の `cc-pages` を PATH の先頭に置く） | ターミナルで答え、binary の更新を促す 1 行を添える |

## README

「使い方」の直後に「Claude Code に使わせる」節を足す。

- 入れ方（`/plugin` の 2 コマンドと、シェルの 2 コマンド）と、明示して呼ぶときの `/cc-pages:cc-page`
- 更新の仕方（marketplace の update → plugin の update）と、binary も build し直すこと
- 図を描く skill を一緒に入れる勧め。種類だけを書く（PlantUML や draw.io で図を SVG に焼く skill、
  数値をグラフにする skill、1 手ずつ送れる図を作る skill など）
- 自分用に直したいとき: `plugins/cc-pages/skills/cc-page/` を `~/.claude/skills/<名前>/` へ写し、
  `SKILL.md` の `name` もその名前に変え、plugin は外す（両方が同じ場面で起動するため）
- skill を直す人向け: `claude --plugin-dir ./plugins/cc-pages` で 1 セッションだけ試せること、
  `claude plugin validate` を repo 直下と `plugins/cc-pages` の 2 か所で流すこと

末尾の設計書の案内に、この文書を足す。

## 保守の決まり

- CLI の flag・サブコマンド・fragment のクラスを変える PR では、同じ PR で skill も直す。
  契約テストがずれを落とす
- 作者の手元の skill に入れた改良を見本へ持ってくるかは、その都度決める（自動では同期しない）

## v1 の範囲

**入れる**: marketplace / plugin / skill（`SKILL.md` と `references/fragment-style.md`）/
`main.go` の FlagSet の切り出し / 契約テスト / validate の workflow / README。

**後で（必要になったら）**: `claude plugin eval` の suite（別チケット。sandbox の中から cc-pages を
呼ぶ方法の下調べから）、plugin の `version` と `claude plugin tag`、plugin の `bin/` への binary の同梱。
