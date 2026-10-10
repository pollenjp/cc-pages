# cc-pages

Claude Code の長い応答を HTML としてローカルに残し、web で読むための Go 製ツール。
ターミナルの stdout には結論 1 行とリンクだけを返す。

## 使い方

    go build -o cc-pages .
    ln -s "$PWD/cc-pages" ~/bin/cc-pages

    cc-pages serve            # 127.0.0.1:7777 で待ち受ける
    cc-pages new --session "$CLAUDE_CODE_SESSION_ID" --title "調べたこと"

`new` が返す `dir` に `index.html`（`<body>` の中身に相当するフラグメント）を書き、
`url` をブラウザで開く。

    cc-pages status --session "$CLAUDE_CODE_SESSION_ID" \
      --now "調査が終わり案 A で合意" --next "spec を commit する"

`status` はセッション直下の `status.json` を丸ごと書き換え（全置換）、セッションの
ページ一覧 `/p/<session>/` の上に「現在の状況」「次やること」として出す。`--now` と
`--next` は繰り返し指定でき、どちらか 1 つは要る。ページより先に打ってもよい。
`status.json` が無いセッションは「状況はまだ書かれていません。」と出る。

`--mode standalone` を付けると `index.html` を完全な HTML 文書として扱い、
ページの URL でそのまま返す（共通 CSS もナビも被せない）。その文書にだけ
インライン `<script>` を許す CSP を返すので、JS が要るページはこちらへ逃がす。
外部オリジンは fragment と同じく止まったまま。戻りはブラウザの戻る、または
文書側から `../`（そのセッションのページ一覧）へのリンクで。

ヘッダの「全幅にする」で本文枠（既定 52rem）を画面いっぱいに広げられる。横に広い図を
原寸で見たいとき用。状態はブラウザに残るので全ページ共通、再読み込みしても保つ。
standalone は元から幅自由なので対象外。

本文の画像はクリックで拡大できる。`assets/` の `<img>` も、フラグメントに直接書いた
inline SVG も対象。最初は画面に収まる大きさで開き、もう一度クリックすると原寸になり、
はみ出す分はドラッグで動かせる。overlay の「別タブで開く」で画像だけを単体で開ける
（inline SVG はその場で組み立てた `blob:`）。Ctrl / ⌘ + クリックと中クリックは overlay を
挟まず直接別タブへ。既に `<a>` で包んである画像は、書き手の意図を優先して対象外。

フラグメントの `<pre class="diff">` は、行頭の `+` / `-` / `@@` を見て 1 行ずつ
色を付けて返す。diff は生のまま貼ればよく、`<span>` を自分で巻く必要はない。

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

## e2e

`tests/e2e/` に Playwright のテストがある。diff の着色を実ブラウザで検査し、
ライト / ダークのスクリーンショットを撮る。

    cd tests/e2e && npm ci
    nix develop ../.. --command npx playwright test

ブラウザと日本語フォントは `flake.nix` の devShell が供給する。フォントが無いと
検査は通るのにスクリーンショットだけが豆腐になるので、devShell の外では止まる。

CI は公式の playwright image を使うので flake を必要としない
(`.github/workflows/e2e.yml`)。撮った画像は PR に sticky コメントで貼られ、
原寸と trace は artifact から取れる。

## 設定

| | 既定 | 上書き |
| --- | --- | --- |
| bind | `127.0.0.1:7777` | `CC_PAGES_ADDR` / `~/.config/cc-pages/config.toml` の `addr` |
| データ root | `~/.local/share/cc-pages` | `CC_PAGES_ROOT` / 同 `root` |

## まだ無いもの

mermaid の描画、引用のクリップボードコピー、systemd socket activation、
本文全文検索、`cc-pages open` / `ls`。

設計は `claude-skills`（private）の
`docs/superpowers/specs/2026-08-29-cc-pages-design.md` にある。

状況板 (`cc-pages status`) の設計はこの repo の
`docs/superpowers/specs/2026-09-22-session-status-design.md` にある。

plugin と skill (`plugins/cc-pages/`) の設計はこの repo の
`docs/superpowers/specs/2026-10-10-sample-skill-plugin-design.md` にある。
