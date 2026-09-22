# セッション top ページを状況板にする — `cc-pages status` と `status.json`

- 日付: 2026-09-22
- 状態: 設計確定（実装計画はこれから）
- 前提の設計書: `claude-skills/docs/superpowers/specs/2026-08-29-cc-pages-design.md`
  （ディスク形式・境界・退行の原則はそちらに従う。この文書はその差分）

## 背景

セッションの top ページ `/p/<session>/` は、今はタイトルとページ一覧（タイトル・summary・時刻）を
出すだけで、「このセッションが今どこまで進んでいて、次に何をするか」はどこにも無い。
知るには各ページを開いて読むしかない。

skill（`pjp-cc-page`）がページを書くたびに、その時点の状況を短く残し、top ページを
「現在の状況・次やること・各ページの一覧と説明」を持つ状況板にしたい。

## 目的

1. `/p/<session>/` に「現在の状況」「次やること」を出す
2. その中身は skill が **ページを書き終えるたびに** 更新する
3. 「各ページの説明」は既存の `summary` をそのまま使う（新しいデータは足さない）
4. skill はディレクトリレイアウトも HTML も知らないまま（境界 1 を保つ）

## 決定事項

| 論点 | 決定 |
| --- | --- |
| 渡し方 | 新サブコマンド `cc-pages status --session ID --now S... --next S...`。構造化した文字列の配列 |
| 置き場 | セッション直下の `status.json`。`session.json` とは分ける |
| 意味 | 全置換。前回の now / next は捨て、最新の 1 つだけを持つ（履歴は持たない） |
| 書く側 | `internal/store` だけ。skill は CLI を叩くだけ |
| 描画 | `session.html` の h1 / meta の直下に「現在の状況」「次やること」の 2 枠、その下に既存のページ一覧 |
| 空状態 | `status.json` が無いセッションは「状況はまだ書かれていません。」を muted で 1 行出す |
| `/` の一覧 | 変えない |
| 各ページの説明 | `page.json` の `summary`。skill 側で必須に格上げする |
| 通知 | `new` と同じ `POST /_/touch`（fire-and-forget） |

### 却下した案

| 案 | 却下理由 |
| --- | --- |
| B: `cc-pages new` に `--now` / `--next` を足す | 状況はセッション全体の属性で、ページ 1 枚の属性ではない。本文を書く前に状況を書くことになり、ページを作らずに状況だけ直すこともできない |
| C: `page.json` に now / next を持ち、最新ページの分を出す | B と同じ抱き合わせの問題に加え、`page.json` の schema 移行が要る。履歴はページ一覧の時系列で既に読めるので利点が薄い |
| 自由な HTML の `overview.html` を skill が書く | skill がセッション dir を知ることになり、毎回全文を書き直すのでトークンも増える。図が要るなら通常のページに書けばよい |
| `session.json` に混ぜる | `session.json` は `new` が read-modify-write する骨で、そこに混ぜると書き込みが競合する。`status.json` は丸ごと上書きだけなので競合の面が無い |

## ディスク形式（差分）

```
~/.local/share/cc-pages/sessions/20260922-a18223c0/
  session.json         # app が書く骨。変更なし
  status.json          # 新規。cc-pages status だけが書く。最新の 1 つだけ
  0001-…/page.json     # cc-pages new が書く。変更なし
  0001-…/index.html    # skill が書く。変更なし
```

### `status.json`

```json
{
  "schema": 1,
  "now": ["設計案を提示し、承認待ち", "案 A を推奨"],
  "next": ["承認後に spec を commit", "writing-plans で計画を書く"],
  "updated_at": "2026-09-22T15:04:05+09:00"
}
```

| キー | 出所 |
| --- | --- |
| `schema` | 固定値 `1`（`store.SchemaVersion`） |
| `now` | `--now` の配列。trim 済み、空は落とす。0 件なら `[]` ではなく省略（`omitempty`） |
| `next` | `--next` の配列。同上 |
| `updated_at` | バイナリが `time.Now()` で書く |

`session_id` は持たない（同じ dir の `session.json` が正）。

読む側は壊れた `status.json` を **黙って飛ばす**（`page.json` と同じ）。`now` と `next` が
両方とも空の `status.json` は「無い」ものとして扱う（手で編集した場合にだけ起きる）。

## CLI

```
cc-pages status --session ID [--now S]... [--next S]...
→ {"url":"http://localhost:7777/p/20260922-a18223c0/"}
```

| 事項 | 決め |
| --- | --- |
| `--session` | 必須。`new` と同じ検証（文字・数字・`-` のみ）。空はエラー |
| `--now` / `--next` | 繰り返し指定。各値は trim し、空になったものは落とす |
| 両方とも 0 件 | エラー（"now か next のどちらかは要る"）。片方だけ空は可（next が無い = やることが無い） |
| セッション dir が無い | `new` と同じ規則（`findOrMakeSessionDir`）で作り、`session.json` の骨も書く。ページより先に status を打ってよい |
| `session.json` | `touchSession` で `LastSeen` を進める（`/` の一覧で上に来る） |
| 書き込み | `status.json` を丸ごと上書き（全置換） |
| 通知 | `store.NotifyTouch`（`new` と同じ。届かなくても失敗にしない） |
| stdout | `{"url": "<base>/p/<session dir>/"}` を 1 行 |
| 終了コード | 検証エラー・書き込み失敗は 1（`new` と同じ） |

全置換にするのは、「今の状況を全部言い直す」方が差分で足し引きするより skill の指示が単純で、
更新し忘れた項目が残り続ける事故が起きないため。

### `store` の API

```go
const StatusFileName = "status.json"

type Status struct {
    Schema    int       `json:"schema"`
    Now       []string  `json:"now,omitempty"`
    Next      []string  `json:"next,omitempty"`
    UpdatedAt time.Time `json:"updated_at"`
}

func ReadStatus(dir string) (Status, error)
func WriteStatus(dir string, s Status) error

type StatusInput struct {
    SessionID string
    Now, Next []string
}
type StatusResult struct {
    URL string `json:"url"`
}

// UpdateStatus はセッション dir を用意し、status.json を全置換で書く。
func UpdateStatus(root, baseURL string, now time.Time, in StatusInput) (StatusResult, error)
```

`main.go` の `cmdStatus` は `new` と同じ形（flag.NewFlagSet → `store.UpdateStatus` →
`store.NotifyTouch` → JSON を stdout）。`tagList` と同じ繰り返しフラグ型を `--now` / `--next` に使う。

## 走査と索引

`status.json` の **上書き** はセッション dir の mtime を動かさない（動くのは作成・削除のとき
だけ）。`status` は毎回 `POST /_/touch` を送るので通常は全走査で拾えるが、touch が届かなかった
ときに定期走査が永久に取りこぼす。差分判定に `status.json` の mtime を 1 つ足す。

- `store.SessionEntry` に `Status *Status` と `StatusModTime time.Time` を足す
  - `readSession` が `ReadStatus` を試み、読めなければ nil
  - `StatusModTime` は `status.json` の mtime。無ければゼロ値
- `store.Scan` の使い回し条件に `statusModTime(dirPath).Equal(old.StatusModTime)` を加える
  （削除されたときはゼロ値になり、やはり読み直す）
- `index.SessionView` に `Status *StatusView` を足す
  - `StatusView{ Now, Next []string; UpdatedAt time.Time }`
  - `buildView` で `UpdatedAt` を `.Local()` に揃える（既存の流儀。表示時刻はここで正規化する）
  - now と next が両方とも空なら nil
- `buildHaystack` に now / next の各項目を足す（状況の文言で検索できる）

## session ページの描画

`session.html`:

```html
<h1>{{.Session.Title}}</h1>
<div class="meta">…既存…</div>
{{with .Session.Status}}
<section class="status">
  <div class="cols">
    {{if .Now}}<div><h2>現在の状況</h2><ul>{{range .Now}}<li>{{.}}</li>{{end}}</ul></div>{{end}}
    {{if .Next}}<div><h2>次やること</h2><ol>{{range .Next}}<li>{{.}}</li>{{end}}</ol></div>{{end}}
  </div>
  <div class="meta"><span>更新 {{.UpdatedAt.Format "2006-01-02 15:04"}}</span></div>
</section>
{{else}}
<p class="meta status-empty">状況はまだ書かれていません。</p>
{{end}}
<h2>ページ</h2>
<ol>…既存の一覧…</ol>
```

- now は `<ul>`、next は順序に意味があるので `<ol>`
- 片方が空ならその枠は出さない。`.cols` なので 1 枠だけなら横いっぱいに広がる
- 項目は `html/template` が escape する。フラグメント（`index.html`）とは違い、status の項目は
  信頼された HTML として **扱わない**。`<script>` を含む文字列も文字として出る
- 空状態の 1 行は、skill の更新漏れに気付くためのもの。何も出さないと「無い」のか「忘れた」のか
  分からない
- `server.sessionRow` に `Status *statusRow` を写す（`toRow`）。`pageData` は変えない
- `style.css` に `.status` の枠（`.cols > div` を `.cards` と同じ見た目のカードに）と
  `.status h2` の余白を足す。`.status-empty` は `.meta` の色で十分なので追加の色は要らない
- `/` の一覧（`list.html`）と各ページ（`page.html`）は変えない

## skill 側（`pjp-cc-page`）の変更

`claude-skills/skills/pjp-cc-page/SKILL.md` の「手順」に **3. 状況を更新する** を足し、
今の 3（stdout）を 4 に繰り下げる。`description` と `references/fragment-style.md` は変えない。

追加する節の骨子（文言は実装時に整える）:

```markdown
### 3. セッションの状況を更新する

ページを書き終えたら、毎回、セッション全体の状況を言い直す。

    cc-pages status --session "$CLAUDE_CODE_SESSION_ID" \
      --now "設計案を提示し、承認待ち" \
      --now "案 A を推奨" \
      --next "承認後に spec を commit" \
      --next "writing-plans で計画を書く"

- **セッション全体の状況を書く。** 今作ったページの目次ではない。
  「何が決まり、何が残っていて、次に何をするか」を、ページを開かずに分かる粒度で
- `--now` は 1〜3 項目、`--next` は 0〜5 項目。各項目は 1 文。`。` は付けない
- **全置換。** 前回の内容は引き継がれないので、まだ有効な項目も含めて全部書き直す
- 失敗しても回答は止めない（`new` と同じ）。`command -v cc-pages` が空なら 1 と同じく何もしない
- ページを作らない往復でも、状況が動いたら打ってよい（方針転換・作業の完了）
```

あわせて手順 1 の `--summary` を「**省略しない**。一覧と top ページに出る 1 行で、
そのページが何を扱うかを書く」に格上げする。top ページの各ページ行がそのまま
「各ページの説明」になるため。

## 反映の順序

skill の新しい手順は、`~/bin/cc-pages` が `status` を持ってから効く。逆順にすると毎回
コマンドが失敗する（退行規則で回答は止まらないが、状況が一度も書かれない）。

1. cc-pages: 実装・テスト・README → PR
2. `go build` して `~/bin/cc-pages` を差し替え、`cc-pages serve` を再起動
3. claude-skills: branch を切って SKILL.md を変更 → commit → PR

## テスト

| 層 | 見るもの |
| --- | --- |
| `store` | `WriteStatus` / `ReadStatus` の往復。`UpdateStatus` が dir を作り `session.json` の骨と `status.json` を書く。両方空でエラー。trim と空項目の除去。`Scan` が `Status` を拾う。**上書きだけ**（dir の mtime を戻して）で読み直される。壊れた JSON を飛ばす |
| CLI (`main_test.go`) | `status` が `status.json` を書いて `{"url":…}` を出す。両方空でエラー。不正な session id でエラー |
| `server` (httptest) | status あり / next 空 / status 無し の 3 状態の描画。項目の `<script>` が escape される。`/` の一覧は変わらない。既存の `TestDisplayedTimesShareOneZone`（時刻が 2 つ）は status 無しの fixture なので壊れない |
| e2e (Playwright) | fixture `tests/e2e/fixtures/root/sessions/20260101-e2efixture/status.json` を足し、`/p/20260101-e2efixture/` で見出しと項目を検査してライト / ダークのスクショを撮る（既存の連番に続ける） |
| skill | 自動テスト不可。実装後にこのセッションで数回使い、更新の粒度が「適度」かを見る |

## v1 の範囲

**入れる**: `cc-pages status` / `status.json` / 走査の差分判定 / 索引と検索対象 /
`session.html` の 2 枠と空状態 / README / skill の手順 3 と summary の必須化。

**後で（必要になったら）**: 状況の履歴（追記ログ）、`/` の一覧カードに now の 1 行目、
`cc-pages status --show`（今の状況を読む）、各ページの表示に status を出す、自由な HTML の
overview。どれも v1 の形を壊さずに足せる。
