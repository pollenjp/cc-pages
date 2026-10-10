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
