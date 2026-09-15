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
