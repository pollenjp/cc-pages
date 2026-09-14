#!/usr/bin/env bash
# CI が撮ったスクリーンショットを PR の sticky コメント 1 つに集約する。
# workflow の最後の step から呼ぶ (assets/e2e.yml を参照)。
#
# **Playwright 専用ではない。** 前提は「あるディレクトリに *.png が置かれている」ことだけで、
# Storybook / VRT / 自前の撮影スクリプトでもそのまま使える。
#
# コメントの形:
#   <details><summary><code>abc1234</code> …</summary> 最大 4 列の画像グリッド </details>
#   <details><summary>archived</summary> 過去 commit 分の同型 <details> (新しい順) </details>
#
# 仕組み (なぜこの形かは references/mechanism.md):
# - コメントは marker で upsert (1 PR に 1 つ)。本文末尾の HTML コメントに JSON state を
#   埋め、次回 run が読んで「今回分を先頭に、前回分を archived へ」送る。
# - 画像実体は PR 専用ブランチ e2e-screenshots/pr-<番号> へ <test-commit-sha>/<file>.png で
#   置く (PR ごとに分離 = 複数 PR の run が互いの画像を潰さない。同一 PR 内の並行 run は
#   workflow の concurrency で直列化/cancel する)。PR close 時に cleanup workflow が
#   ブランチごと削除する。
# - コミットは **append-only** (force-push しない)。tip の tree は常に「今回 run の分だけ」に
#   剪定するので checkout は軽いまま、過去 run の画像は履歴側の commit に残る。
# - **画像 URL は branch 名ではなく「その run が画像を積んだ commit の SHA」で書く**
#   (= GitHub の permalink)。branch 名は毎 run 進むため URL の指す中身が変わり得るのに対し、
#   commit SHA 固定なら archived の entry も撮影当時のツリーを指し続ける。append-only なので
#   参照先 commit は常に branch tip の ancestor = reachable (unreachable object の retention に
#   依存しない)。
# - private repo でも描画されるよう <img src> は github.com/<repo>/raw/... を使う (ログイン
#   cookie → token 付き redirect)。raw.githubusercontent.com 直は匿名 fetch になり
#   描画されない。<a href> は blob URL (人間が開く用)。
# - archived の上限 (MAX_ARCHIVED) を超えた古い entry は画像ごと落ちる。
#
# 使い方:  pr-screenshots.sh <pr_number> <head_sha> <job_status>
# 必要 env: GITHUB_REPOSITORY / GITHUB_TOKEN (gh と push が使う)
# 任意 env: SHOTS_DIR (既定 screenshots) / MAX_ARCHIVED (既定 15)
#           GRID_COLS (既定 4) / IMG_WIDTH (既定 200) / SHOTS_BRANCH (既定 e2e-screenshots/pr-<PR番号>)
#           SHOTS_MARKER (既定 <!-- e2e-screenshots -->) / SHOTS_TITLE / SHOTS_SOURCE
#
# markup だけ確認したいとき (API も push もしない。移植したら最初にこれを叩く):
#   RENDER_TEST=1 .github/scripts/pr-screenshots.sh
set -eu -o pipefail

# 1 repo で 2 系統のスクショ (例: PC 版 / モバイル版) を貼りたい場合は、系統ごとに
# SHOTS_MARKER と SHOTS_BRANCH を変える。marker が違えば別のコメントとして upsert される。
MARKER=${SHOTS_MARKER:-"<!-- e2e-screenshots -->"}
STATE_PREFIX="e2e-shots-state:"
SHOTS_DIR=${SHOTS_DIR:-screenshots}
SHOTS_TITLE=${SHOTS_TITLE:-"e2e スクリーンショット"}
SHOTS_SOURCE=${SHOTS_SOURCE:-}
MAX_ARCHIVED=${MAX_ARCHIVED:-15}
GRID_COLS=${GRID_COLS:-4}
IMG_WIDTH=${IMG_WIDTH:-200}

# --- 描画 (state JSON → コメント本文) -----------------------------------------

# $1: entries JSON (新しい順)。stdout に本文を出す。
render_body() {
  local entries="$1"

  echo "${MARKER}"
  echo "### ${SHOTS_TITLE}"
  echo
  echo "最新 run の画像。過去の commit 分は archived に畳んで残す (最大 ${MAX_ARCHIVED} 件)。"
  if [ -n "${SHOTS_SOURCE}" ]; then
    echo "生成元: \`${SHOTS_SOURCE}\` / 実体は \`${SHOTS_BRANCH}\` ブランチ。"
  else
    echo "画像の実体は \`${SHOTS_BRANCH}\` ブランチ。"
  fi
  echo

  local count
  count=$(jq 'length' <<<"$entries")
  if [ "$count" -eq 0 ]; then
    echo "(まだ画像がありません)"
  else
    render_entry "$(jq -c '.[0]' <<<"$entries")"
    if [ "$count" -gt 1 ]; then
      echo "<details>"
      echo "<summary>archived (過去 $((count - 1)) 件)</summary>"
      echo
      local i
      for ((i = 1; i < count; i++)); do
        render_entry "$(jq -c ".[$i]" <<<"$entries")"
      done
      echo "</details>"
    fi
  fi

  echo
  echo "<!-- ${STATE_PREFIX} $(jq -c . <<<"$entries") -->"
}

# $1: entry JSON {sha, result, time, files[]}
render_entry() {
  local entry="$1" sha result time short emoji
  sha=$(jq -r '.sha' <<<"$entry")
  result=$(jq -r '.result' <<<"$entry")
  time=$(jq -r '.time' <<<"$entry")
  short=${sha:0:7}
  emoji="✅"
  [ "$result" = "success" ] || emoji="❌"

  echo "<details>"
  echo "<summary><code>${short}</code> ${emoji} ${time}</summary>"
  echo
  render_grid "$sha" "$entry"
  echo
  echo "</details>"
}

# $1: sha / $2: entry JSON。files を GRID_COLS 列の <table> に並べる。
render_grid() {
  local sha="$1" entry="$2"
  local repo="${GITHUB_REPOSITORY:-owner/repo}"
  # 参照先は「その run が画像を積んだ commit」の SHA = permalink (branch 名は毎 run 進むので
  # 使わない)。shots_commit を持たない古い entry だけ branch にフォールバックする
  # (この機能より前に作られた既存コメントを壊さないため)。
  local ref
  ref=$(jq -r '.shots_commit // ""' <<<"$entry")
  [ -n "$ref" ] || ref="$SHOTS_BRANCH"
  # img は github.com/raw (private でもログイン済みブラウザなら token redirect で描画される)。
  # リンクは blob (GitHub のファイルビューア)。
  local raw_base="https://github.com/${repo}/raw/${ref}"
  local blob_base="https://github.com/${repo}/blob/${ref}"
  local files n i url blob_url
  mapfile -t files < <(jq -r '.files[]' <<<"$entry")
  n=${#files[@]}
  if [ "$n" -eq 0 ]; then
    echo "(画像なし: テスト前に落ちた run)"
    return
  fi
  echo "<table><tr>"
  for ((i = 0; i < n; i++)); do
    if [ "$i" -gt 0 ] && [ $((i % GRID_COLS)) -eq 0 ]; then
      echo "</tr><tr>"
    fi
    url="${raw_base}/${sha}/${files[$i]}"
    blob_url="${blob_base}/${sha}/${files[$i]}"
    echo "<td><a href=\"${blob_url}\"><img src=\"${url}\" width=\"${IMG_WIDTH}\" alt=\"${files[$i]}\"></a><br><sub>${files[$i]}</sub></td>"
  done
  echo "</tr></table>"
}

# --- RENDER_TEST: fake データで markup を確認 ----------------------------------

if [ "${RENDER_TEST:-}" = "1" ]; then
  SHOTS_BRANCH=${SHOTS_BRANCH:-e2e-screenshots/pr-0}
  entries=$(jq -n '[
    {sha: "aaaaaaa1111111111111111111111111111111111", result: "success",
     time: "2026-09-09 10:00 UTC",
     shots_commit: "cccccccc33333333333333333333333333333333",
     files: ["01-a.png","02-b.png","03-c.png","04-d.png","05-e.png"]},
    {sha: "bbbbbbb2222222222222222222222222222222222", result: "failure",
     time: "2026-09-09 09:00 UTC", files: ["01-a.png"]}
  ]')
  render_body "$entries"
  exit 0
fi

# --- 本体 ----------------------------------------------------------------------

PR_NUMBER=${1:?usage: pr-screenshots.sh <pr_number> <head_sha> <job_status>}
HEAD_SHA=${2:?head_sha required}
JOB_STATUS=${3:?job_status required}
REPO=${GITHUB_REPOSITORY:?GITHUB_REPOSITORY required}
# PR ごとにブランチを分ける (複数 PR の run が互いの画像を潰さないように)
SHOTS_BRANCH=${SHOTS_BRANCH:-e2e-screenshots/pr-${PR_NUMBER}}

if [ ! -d "$SHOTS_DIR" ] || ! ls "$SHOTS_DIR"/*.png >/dev/null 2>&1; then
  echo "スクリーンショットが無い (テスト前に落ちた run?) のでコメントしない: $SHOTS_DIR"
  exit 0
fi

# PR が既に close / merge 済みなら何もしない。close 間際に走り出した run が
# cleanup workflow の削除後にブランチを再 push して孤児化させる race を塞ぐ。
pr_state=$(gh api "repos/${REPO}/pulls/${PR_NUMBER}" --jq '.state')
if [ "$pr_state" != "open" ]; then
  echo "PR #${PR_NUMBER} は ${pr_state} のためスキップ (cleanup 済みブランチを復活させない)"
  exit 0
fi

# 1. 既存の sticky コメントを探す (marker 一致)
comments=$(gh api "repos/${REPO}/issues/${PR_NUMBER}/comments" --paginate)
comment_id=$(jq -r --arg m "$MARKER" \
  '[.[] | select(.body | startswith($m))][0].id // empty' <<<"$comments")

# 2. 既存 state を取り出す (無ければ空配列)
prev_entries="[]"
if [ -n "$comment_id" ]; then
  prev_body=$(jq -r --arg m "$MARKER" \
    '[.[] | select(.body | startswith($m))][0].body' <<<"$comments")
  extracted=$(sed -n "s/.*<!-- ${STATE_PREFIX} \(.*\) -->.*/\1/p" <<<"$prev_body" | head -1)
  if [ -n "$extracted" ] && jq -e . >/dev/null 2>&1 <<<"$extracted"; then
    prev_entries="$extracted"
  fi
fi

# 3. 今回分を先頭に積む (同一 sha の再 run は置き換え)。current 1 + archived MAX まで
files_json=$(find "$SHOTS_DIR" -maxdepth 1 -name '*.png' -printf '%f\n' | sort | jq -R . | jq -sc .)
now=$(date -u '+%Y-%m-%d %H:%M UTC')
entries=$(jq -c \
  --arg sha "$HEAD_SHA" --arg result "$JOB_STATUS" --arg time "$now" \
  --argjson files "$files_json" --argjson max "$MAX_ARCHIVED" \
  '[{sha: $sha, result: $result, time: $time, files: $files}]
   + [.[] | select(.sha != $sha)] | .[0:($max + 1)]' <<<"$prev_entries")

# 4. スクショを SHOTS_BRANCH に積む (append-only。force-push しない)
#    tip の tree は今回 run の分だけに剪定する = checkout は軽いまま、過去 run の画像は
#    履歴側の commit に残り、コメントは各 entry の shots_commit SHA で参照する
#    (ancestor なので必ず reachable = permalink が腐らない)。
remote="https://x-access-token:${GITHUB_TOKEN}@github.com/${REPO}.git"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
repo_dir="$work/shots"

push_shots() {
  rm -rf "$repo_dir"
  if git ls-remote --exit-code "$remote" "refs/heads/${SHOTS_BRANCH}" >/dev/null 2>&1; then
    # 履歴ごと clone する (shallow clone からの push は拒否され得るため深さ制限しない)。
    # 画像は 1 run 1MB 未満で、ブランチは PR close 時に消えるので実用上問題ない。
    git clone --quiet --single-branch --branch "$SHOTS_BRANCH" "$remote" "$repo_dir"
  else
    mkdir -p "$repo_dir"
    git -C "$repo_dir" init --quiet -b "$SHOTS_BRANCH"
  fi
  git -C "$repo_dir" config user.name "github-actions[bot]"
  git -C "$repo_dir" config user.email "41898282+github-actions[bot]@users.noreply.github.com"
  # tip の tree は今回分のみ (過去分は履歴の commit から SHA で引く)
  find "$repo_dir" -mindepth 1 -maxdepth 1 -not -name .git -exec rm -rf {} +
  mkdir -p "$repo_dir/$HEAD_SHA"
  cp "$SHOTS_DIR"/*.png "$repo_dir/$HEAD_SHA/"
  git -C "$repo_dir" add -A
  git -C "$repo_dir" commit --quiet -m "screenshots for ${HEAD_SHA} (自動生成)"
  git -C "$repo_dir" push --quiet "$remote" "HEAD:refs/heads/${SHOTS_BRANCH}"
}

# concurrency で同一 PR の run は直列化されるが、取りこぼしの non-fast-forward に備えて
# 1 度だけ clone からやり直す
if ! push_shots; then
  echo "スクショの push が拒否された (並行 run?)。5 秒後に 1 度だけ再試行する" >&2
  sleep 5
  push_shots
fi
shots_commit=$(git -C "$repo_dir" rev-parse HEAD)
echo "pushed screenshots to ${SHOTS_BRANCH} @ ${shots_commit} (for ${HEAD_SHA})"

# 今回 entry に画像の格納先 commit を記録する (コメントの URL がこれを指す)
entries=$(jq -c --arg c "$shots_commit" '.[0].shots_commit = $c' <<<"$entries")

# 5. コメント本文を作って upsert
body_file="$work/body.md"
render_body "$entries" >"$body_file"
payload="$work/payload.json"
jq -n --rawfile b "$body_file" '{body: $b}' >"$payload"
if [ -n "$comment_id" ]; then
  gh api --method PATCH "repos/${REPO}/issues/comments/${comment_id}" --input "$payload" >/dev/null
  echo "updated comment ${comment_id}"
else
  gh api --method POST "repos/${REPO}/issues/${PR_NUMBER}/comments" --input "$payload" >/dev/null
  echo "created new comment"
fi
