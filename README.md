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

フラグメントの `<pre class="diff">` は、行頭の `+` / `-` / `@@` を見て 1 行ずつ
色を付けて返す。diff は生のまま貼ればよく、`<span>` を自分で巻く必要はない。

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
