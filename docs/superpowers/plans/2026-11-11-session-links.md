# 会話のトップページにリンクを残す (cc-pages link / links.json / /s/) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `cc-pages link` が会話の `links.json` にリンクをまとめて残し、会話のトップページ `/p/<session>/` に「リンク」の節を出す。`/s/<session_id>/` で session id から会話のトップページへ飛べ、一覧にリンクの数が出て、絞り込みでリンクも引ける。

**Architecture:** ディスクの形は今までどおり `internal/store` に閉じ、`links.json` を 1 つ足す (書くのは `AddLinks` だけ。lock を取り、一時ファイルに書いて rename する)。`store.Scan` が拾い、`index` が `SessionView.Links` に写し、`server` が `session.html` で組ごとに描く。`link` の知らせは新しい口 `POST /_/touch/{session}` に応答を待たずに送り、serve はその会話のディレクトリだけを読み直す (transcript の走査は走らせない)。

**Tech Stack:** Go 1.24 (`go.mod`。手元は mise の go 1.27)、`html/template`、標準ライブラリのみ (lock は `syscall.Flock`)。e2e は Playwright (`tests/e2e`)。

**Spec:** `docs/superpowers/specs/2026-11-11-session-links-design.md`

## Global Constraints

- 新しい依存は足さない (`go.mod` の require は `github.com/BurntSushi/toml` のみのまま)
- ディスクの形 (ファイル名・スキーマ・採番) を触るコードは `internal/store` にだけ書く。`links.json` は `schema: 1`。`page.json`・`session.json`・`status.json` のスキーマは変えない
- ディレクトリは `0o700`、ファイルは `0o600`
- 契約の値 (spec の「契約」の節。変えない): `url` は http か https の絶対 URL で 2048 字まで / `label` が無い・空なら URL を出す / `group` が無い・空なら `その他` / `source` は `assistant` か `tool`、無ければ `assistant` / `at` は RFC 3339、無ければ今 / stdout は `{"url","added","updated","skipped"}` の 1 行 / 終了コード 1 は session id の検証エラー・ファイルの読み書きの失敗・flag の誤り・flag で渡した 1 件を捨てたとき / 1 会話 5000 件まで / `/s/` は 302 か 404 で、404 の文言は「この会話のページ・リンクはまだありません」
- `new`・`status`・`POST /_/touch` (全体の走査) と、今のページの振る舞いは変えない
- リンクの label・URL は `html/template` の自動エスケープに任せる。`template.HTML` に包まない。`<a>` にするのは描くときにもう一度確かめた http(s) だけ
- CI (`.github/workflows/go.yml`) は `gofmt -l .` が空・`go vet ./...`・`go test ./...` を要求する。各 task の最後にこの 3 つを通す
- コメント・エラー文・UI の文言は日本語。コメントは「なぜそうするか」を書く (既存の流儀)
- commit は `type(scope): 日本語の要約` + 空行 + 本文、末尾に `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`。public な repo なので、チケットの ID・URL と、ほかの repo の issue・PR・commit への参照は書かない
- 作業 branch は `feat/session-links`。実物で確かめるときは自分で build した `./cc-pages` を、一時ディレクトリの `CC_PAGES_ROOT` と空いている別の `CC_PAGES_ADDR` で動かす (ユーザーの `~/.local/share/cc-pages`・動いている serve・systemd の unit には触らない)

## Review Focus

1. 呼ぶ側が turn ごとに同じリンクの組を渡し直す — 2 回目からは何も書かず (links.json と session.json の mtime が動かない)、知らせも送らず、`added`・`updated` は 0 (Task 3 の `TestAddLinksIsIdempotent`)
2. リンクの無い会話でも呼ぶ側が空の入力で打つ — 会話のディレクトリを作らない (一覧に空の会話が増えない)。`url` は `/s/<session_id>/` (Task 3 の `TestAddLinksWithoutItemsDoesNotTouchDisk`・Task 9 の `TestLinkEmptyStdinCreatesNothing`)
3. cc-page の URL のパスは日本語のまま (`/p/…/0001-diff-の配色/`) も、%XX に潰した形も来る — どちらも受け付け、添え書きはページの題になる (Task 1 の `TestNewLinkInputKeepsJapaneseURL`・Task 8 の `TestSessionPageLinkNotes`)
4. 呼ぶ側の不具合で巨大な 1 行・終わらない入力が来る — 長すぎる行は捨てて続きを読み、入力全体は上限で読むのをやめる (Task 1 の `TestParseLinkLinesBounds`)
5. links.json を手で壊した・`javascript:` を書いた — serve はほかの会話ごと落ちず、そのリンクは `<a>` にならない。次の `cc-pages link` は壊れたファイルを `links.json.broken` に退けて書き直す (Task 3 の `TestAddLinksSetsBrokenFileAside`・Task 4 の `TestScanSkipsBrokenLinks`・Task 8 の `TestSessionPageDoesNotLinkUnsafeURL`)

## File Structure

| ファイル | 役割 |
| --- | --- |
| `internal/store/links.go` (新規) | `Link`・`Links`・`ReadLinks`・`writeLinks`・`LinkInput`・`NewLinkInput`・`ParseLinkLines`・`LinkResult`・`AddLinks`・`linksModTime` (Task 1・3・4 で足していく) |
| `internal/store/atomic.go` (新規) | `writeFileAtomic` (一時ファイル → fsync → rename) |
| `internal/store/lock_flock.go` / `lock_other.go` (新規) | `lockFile` (linux・darwin は `flock`、ほかは lock しない) |
| `internal/store/create.go` | `ValidateSessionID` (export)・`FindSessionDir`・`sessionDirMatches`・名前で絞る `findOrMakeSessionDir`・`NotifySession` |
| `internal/store/session.go` | `WriteSession` を `writeFileAtomic` に (link が turn ごとに書くので、読み手が書きかけを読まないように) |
| `internal/store/scan.go` | `SessionEntry.Links`・`LinksModTime`、`readSession` が links.json を読む、`Scan` の差分判定、`ReadSessionEntry` |
| `internal/index/index.go` | `LinkView`、`SessionView.SessionID`・`Started`・`Links`・`LinkCount`、`SessionByID`、検索対象 |
| `internal/server/refresh.go` | `Refresher.meta` (最後の transcript のメタ) と `RefreshSession` |
| `internal/server/server.go` | `New` の第 4 引数 `refreshSession`、`POST /_/touch/{session}`、`GET /s/{session}`、`renderStatus` |
| `internal/server/links.go` (新規) | 「リンク」の節の組み立て (`linkSection`・`buildLinks`・`safeHref`・`shortURL`・`timeRange`・`selfTitle`) |
| `internal/server/render.go` | `pageData.Links`、`sessionRow.LinkCount` |
| `internal/web/session.html`・`list.html`・`layout.html`・`notfound.html` (新規) | 「リンク」の節・一覧の数・検索の placeholder・`/s/` の 404 |
| `internal/web/static/style.css` | `.links` の見た目 |
| `main.go` / `main_test.go` | `link` サブコマンド (`linkArgs`・`linkFlags`・`cmdLink`)、serve の配線 |
| `tests/e2e/fixtures/root/sessions/20260101-e2efixture/links.json` (新規)・`tests/e2e/links.spec.ts` (新規) | e2e とスクショ (10・11) |
| `README.md` | `cc-pages link` と `/s/<session_id>/` |

既存のテストヘルパで使うもの: `internal/store` の `fixedTime()`・`seed()`・`only()`、`internal/index` の `at()`・`entry()`、`internal/server` の `at()`・`mkEntry()`・`get()`、`main_test.go` の `statusCfg()`。

---

### Task 1: store — links.json の型と読み書き、1 件の検証と JSON Lines の読み取り

**Files:**
- Create: `internal/store/links.go`, `internal/store/atomic.go`, `internal/store/links_test.go`

**Interfaces:**
- Produces:
  - `const LinksFileName = "links.json"`、`const MaxLinks = 5000`、`const MaxLinkURLRunes = 2048`、`const SourceAssistant = "assistant"`、`const SourceTool = "tool"`、`const DefaultLinkGroup = "その他"`
  - `type Link struct { URL string "url"; Label string "label,omitempty"; Group string "group,omitempty"; Sources []string "sources"; FirstSeen time.Time "first_seen"; LastSeen time.Time "last_seen" }` (引用は json タグ)
  - `type Links struct { Schema int "schema"; UpdatedAt time.Time "updated_at"; Links []Link "links" }`
  - `func ReadLinks(dir string) (Links, error)`、`func writeLinks(dir string, l Links) error`
  - `type LinkInput struct { URL, Label, Group, Source string; At time.Time }`
  - `func NewLinkInput(rawURL, label, group, source, at string, now time.Time) (LinkInput, error)`
  - `func ParseLinkLines(r io.Reader, now time.Time, warn func(string)) (items []LinkInput, skipped int)`
  - `func writeFileAtomic(path string, data []byte, perm os.FileMode) error`

- [ ] **Step 1: Write the failing tests** (`links_test.go`)
  - `TestLinksRoundTrip`: `writeLinks` → `ReadLinks` で Schema・UpdatedAt・2 件の全フィールドが戻る。`links.json` は `0o600`。dir に `links.json` 以外のファイル (一時ファイル) が残らない
  - `TestReadLinksMissing`: 無いときは `os.IsNotExist(err)`
  - `TestNewLinkInputDefaults`: `NewLinkInput("  https://github.com/o/r/pull/12 ", "", "", "", "", now)` → URL は前後の空白を落とした形、Label `""`、Group `""`、Source `"assistant"`、`At.Equal(now)`。group `"その他"` は `""` に、URL と同じ label は `""` に寄せる
  - `TestNewLinkInputParsesAt`: `"2026-11-11T15:06:01.234Z"` が読め、`At.Equal(time.Date(2026,11,11,15,6,1,234e6,time.UTC))`
  - `TestNewLinkInputRejects` (表で、どれも err): `""`、`"ftp://x/y"`、`"javascript:alert(1)"`、`"/p/x/"`、`"https://"`、`"http:foo"`、`"https://a b.com/"`、`"https://ex.com/a\tb"`、`"https://ex.com/\x00"`、2049 字の URL (`"https://ex.com/" + strings.Repeat("a", 2049-15)`)、source `"web"`、at `"昨日"`。2048 字ちょうどは通る
  - `TestNewLinkInputKeepsJapaneseURL`: `http://localhost:7777/p/20261111-0a1b2c3d/0001-調べたこと/` も `%E8%AA%BF…` の形も、文字列を変えずに通る
  - `TestNewLinkInputCleansText`: label `"a\nb\tc"` → `"a b c"`、201 字の label は 200 字 (末尾 `…`)、51 字の group は 50 字
  - `TestParseLinkLines`: 入力 5 行 (正しい行・空行・`{壊れた`・`[1,2]`・`{"label":"x"}`) と `{"url":"https://a/","source":"tool","at":"2026-11-11T15:06:00+09:00"}` → items 2 件、skipped 3、warn は 3 回で、それぞれ行番号 (`3 行目` など) と理由を含む。空行は数えない
  - `TestParseLinkLinesBounds`: 内部の `parseLinkLines(r, now, warn, maxLine=64, maxTotal=256)` で、65 バイトの行は捨てて (skipped に数え) 次の行を読む。256 バイトを超えた所で読むのをやめ、warn を 1 回出す

- [ ] **Step 2: Run** `go test ./internal/store -run 'Links|LinkInput|ParseLink'` — Expected: FAIL (undefined)

- [ ] **Step 3: Implement**
  - `writeFileAtomic`: 同じ dir に `os.CreateTemp(dir, "."+base+".tmp-*")`、書いて `Sync` して閉じ、`os.Rename`。失敗したら一時ファイルを消す。`perm` を `Chmod` で当てる
  - `NewLinkInput` の検証の順: TrimSpace → 空なら「url が無い」→ `utf8.ValidString` → 2048 字 (rune) を超えたら「長すぎる」→ 空白・制御文字 (`unicode.IsSpace`・`unicode.IsControl`) を含めば不可 → `url.Parse` → scheme が http/https で Host が空でないこと。label・group は制御文字を空白に替え、空白の並びを 1 つにし、200 字・50 字で切る (末尾 `…`)。理由の文に値を引くときは `%q` で 100 字まで
  - `parseLinkLines`: `bufio.Reader` の `ReadSlice('\n')` で 1 行ずつ。`maxLine` を超えた行は行末まで読み捨てる。`io.LimitReader(r, maxTotal+1)` で読み、超えたら止める。行ごとに `json.Unmarshal` で `{url,label,group,source,at}` (どれも string) に読み、読めなければ「JSON として読めない」。`ParseLinkLines` は `maxLine = 64 KiB`・`maxTotal = 16 MiB` で呼ぶ
  - warn の文は `"<n> 行目を捨てた: <理由>"` の形

- [ ] **Step 4: Run** `go test ./internal/store` — Expected: PASS

- [ ] **Step 5: gofmt・vet・test を通して commit** — `feat(store): links.json の形と、リンク 1 件の検証・JSON Lines の読み取りを足す`

---

### Task 2: store — 会話のディレクトリを名前で絞って引き、session.json を書きかけのまま見せない

**Files:**
- Modify: `internal/store/create.go`, `internal/store/session.go`, `internal/store/status.go` (呼び名の変更だけ)
- Test: `internal/store/create_test.go`

**Interfaces:**
- Produces: `func ValidateSessionID(sessionID string) error` (今の `validateSessionID` の改名)、`func FindSessionDir(sessionsDir, sessionID string) (string, bool)`、`func sessionDirMatches(name, sessionID string) bool`

- [ ] **Step 1: Write the failing tests**
  - `TestFindOrMakeSessionDirReadsOnlyMatchingNames`: `sessions/20260101-zzzzzzzz/session.json` に session_id `409bfd08-aaaa` を書いておく。`CreatePage` (session `409bfd08-aaaa`) は `20260829-409bfd08` を新しく作る (名前の合わない dir の session.json は読まない)
  - `TestFindSessionDir`: 12 字に伸ばした名前 `20260829-409bfd08-aaa` を見つける / 先頭が同じ別の会話 (`409bfd08-bbbb`) は見つけない / sessions が無ければ `("", false)`
  - `TestSessionDirMatches` (表): (`20261111-0a1b2c3d`, `0a1b2c3d-e5f6-…`) → true、(`20261111-0a1b2c3d-e5f`, 同) → true、(`20261111-zzzz`, 同) → false、(`nodash`, 同) → false、(`20261111-`, 同) → false
  - `TestWriteSessionLeavesNoTempFile`: `WriteSession` の後、dir には `session.json` だけ。パーミッションは `0o600`

- [ ] **Step 2: Run** — Expected: FAIL

- [ ] **Step 3: Implement**
  - `sessionDirMatches`: 最初の `-` で切った残りが空でなく、session id の先頭と一致する (日付 + `SessionDirName` が作る先頭部分の形)
  - `findOrMakeSessionDir`: `ReadDir` の名前で `sessionDirMatches` に当たるものだけ `ReadSession` し、`session_id` の一致で決める (最後の決め手は今までどおり)。衝突を避ける `taken` は全部の dir の名前から作る (今と同じ)。`FindSessionDir` は同じ照合を、作らずに行う
  - `WriteSession` は `writeFileAtomic(…, 0o600)` で書く
  - `validateSessionID` の呼び出しを `ValidateSessionID` に替える

- [ ] **Step 4: Run** `go test ./internal/store` — Expected: PASS (既存の `TestCreatePageDistinguishesCollidingPrefixes` なども)

- [ ] **Step 5: commit** — `perf(store): 会話のディレクトリを名前で絞って引き、session.json を一時ファイルから書く`

---

### Task 3: store — AddLinks (まとめ方・上限・lock・0 件)

**Files:**
- Create: `internal/store/lock_flock.go` (`//go:build linux || darwin`), `internal/store/lock_other.go` (`//go:build !linux && !darwin`)
- Modify: `internal/store/links.go`
- Test: `internal/store/links_test.go`

**Interfaces:**
- Consumes: Task 1 の `Link`・`Links`・`LinkInput`・`ReadLinks`・`writeLinks`、Task 2 の `ValidateSessionID`・`FindSessionDir`・`findOrMakeSessionDir`、既存の `touchSession`
- Produces:
  - `type LinkResult struct { URL string "url"; Added int "added"; Updated int "updated"; Skipped int "skipped"; Dir string "-"; Changed bool "-" }`
  - `func AddLinks(root, baseURL string, now time.Time, sessionID string, items []LinkInput, warn func(string)) (LinkResult, error)`
  - `func lockFile(path string, timeout time.Duration) (unlock func(), err error)`

- [ ] **Step 1: Write the failing tests**
  - `TestAddLinksCreatesSession`: 新しい session に 2 件 → `sessions/20260829-409bfd08/links.json` に 2 件、`session.json` の骨 (session_id・dir)、`URL == base+"/p/20260829-409bfd08/"`、`Added == 2`、`Changed`、`Dir == "20260829-409bfd08"`
  - `TestAddLinksMerges`: 1 回目 `{u, label "", group "", assistant, 10:00}`、2 回目 `{u, label "r#12", group "PR", tool, 09:00}` と `{u, …, 11:00}` → 1 件で、FirstSeen 09:00・LastSeen 11:00・Sources `["assistant","tool"]`・Label `"r#12"`・Group `"PR"`、`Updated == 1`・`Added == 0`。3 回目に label `"別"` を渡しても `"r#12"` のまま
  - `TestAddLinksSameURLInOneCall`: 1 回の呼び出しに同じ URL が 3 回 → `Added == 1`・`Updated == 0`、時刻は最小と最大
  - `TestAddLinksIsIdempotent`: 同じ入力を 2 回 → 2 回目は `Added == 0`・`Updated == 0`・`!Changed`。links.json と session.json の中身と mtime が 1 回目の後から変わらない
  - `TestAddLinksCap`: 5000 件の links.json を置いておき、新しい URL 1 件と既にある URL 1 件 (時刻を進めたもの) → `Skipped == 1`・`Updated == 1`、件数は 5000 のまま、warn に上限の旨
  - `TestAddLinksWithoutItemsDoesNotTouchDisk`: items 0 件 → root に何も作らない、`URL == base+"/s/409bfd08-aaaa/"`。会話のディレクトリが既にあれば `URL` はその `/p/<dir>/` で、ファイルは書かない
  - `TestAddLinksRejectsBadSession`: `""`・`"../x"`・`"a b"` → err、root に何も作らない
  - `TestAddLinksReusesSessionDirOfPages`: `CreatePage` の後の `AddLinks` は同じ dir に書く
  - `TestAddLinksConcurrent`: 20 個の goroutine がそれぞれ別の URL を 5 件ずつ → links.json に 100 件 (lock が無いと読み・まとめ・書きが重なって消える)
  - `TestAddLinksSetsBrokenFileAside`: links.json が `{壊れた` → 成功し、元の中身は `links.json.broken` に移り、warn が出て、新しい links.json に渡した件が入る

- [ ] **Step 2: Run** — Expected: FAIL

- [ ] **Step 3: Implement**
  - 順: session id を検証 → items が 0 なら `FindSessionDir` で URL だけ決めて返す (ディスクに触らない) → `findOrMakeSessionDir` → `lockFile(<dir>/links.lock, 3*time.Second)` → `ReadLinks` (無ければ空。JSON が壊れていれば `links.json.broken` へ rename して warn し、空から) → まとめる → 変わったときだけ `writeLinks` (Schema 1・UpdatedAt now) と `touchSession`
  - まとめ方: 前からある URL の集合を先に取る。同じ URL には FirstSeen は小さい方・LastSeen は大きい方・Sources は和 (`assistant`, `tool` の順に並べる)。時刻か出どころが変わった「前からある URL」の数が `Updated`。label・group は空のときだけ埋める (埋めても `Updated` には数えないが、書き直す)。新しい URL は `MaxLinks` に達していれば捨てて `Skipped` に数え、warn する。保存する時刻は `.Local()`
  - `lockFile` (flock): `O_RDWR|O_CREATE` (0o600) で開き、`LOCK_EX|LOCK_NB` を 5ms ごとに timeout まで試す。取れなければ「lock が取れない」の err。`lock_other.go` は何もしない unlock を返す (cc-pages は linux・darwin で使う。コメントで書く)

- [ ] **Step 4: Run** `go test -race ./internal/store` — Expected: PASS

- [ ] **Step 5: commit** — `feat(store): AddLinks で会話のリンクを lock の下でまとめて links.json に書く`

---

### Task 4: store — 走査が links.json を拾い、1 会話だけを読み直せる

**Files:**
- Modify: `internal/store/scan.go`
- Test: `internal/store/scan_test.go`

**Interfaces:**
- Consumes: Task 1 の `ReadLinks`
- Produces: `func linksModTime(dir string) time.Time` (`statusModTime` と同じ形。無ければゼロ値)、`SessionEntry.Links []Link` (無い・壊れている・0 件なら nil)、`SessionEntry.LinksModTime time.Time`、`func ReadSessionEntry(sessionsDir, dirName string) (SessionEntry, bool)` (dir が無ければ false)

- [ ] **Step 1: Write the failing tests**
  - `TestScanReadsLinks`: `AddLinks` で 2 件 → `only(Scan(…)).Links` が 2 件、`LinksModTime` が非ゼロ。ページの無い (リンクだけの) 会話も 1 件の会話として出て、`Session.SessionID` が入る
  - `TestScanSkipsBrokenLinks`: 壊れた links.json → `Links == nil`、ページは残る
  - `TestScanRereadsWhenLinksOverwritten`: `TestScanRereadsWhenStatusOverwritten` と同じ手で、links.json の mtime だけ進めて dir の mtime を戻す → 読み直す
  - `TestReadSessionEntry`: ある dir は `Scan` の 1 件と同じ中身、無い dir は false

- [ ] **Step 2: Run** — Expected: FAIL

- [ ] **Step 3: Implement** — `readSession` で `LinksModTime` を中身より先に採り、`ReadLinks` が通って 0 件でなければ `Links` に入れる。`Scan` の使い回しの条件に `linksModTime(dirPath).Equal(old.LinksModTime)` を足す。`ReadSessionEntry` は `os.Stat` の dir の mtime で `readSession` を呼ぶ

- [ ] **Step 4: Run** `go test ./internal/store` — Expected: PASS

- [ ] **Step 5: commit** — `feat(store): 走査が links.json を拾い、1 つの会話だけを読み直せるようにする`

---

### Task 5: index — リンクを索引に載せ、session id で引き、検索の対象にする

**Files:**
- Modify: `internal/index/index.go`
- Test: `internal/index/index_test.go`

**Interfaces:**
- Consumes: Task 4 の `SessionEntry.Links`
- Produces: `type LinkView struct { URL, Label, Group string; Sources []string; FirstSeen, LastSeen time.Time }`、`SessionView.SessionID string`・`Started time.Time`・`Links []LinkView`・`LinkCount int`、`func (ix *Index) SessionByID(sessionID string) (SessionView, bool)`

- [ ] **Step 1: Write the failing tests**
  - `TestBuildViewCopiesLinks`: links の時刻は `.Local()` に揃う、`LinkCount == 2`、`SessionID` は session.json の値
  - `TestStartedIsEarliest`: `Started` は session の FirstSeen・ページの CreatedAt・リンクの FirstSeen のうち最も早い非ゼロの時刻
  - `TestSearchMatchesLinkLabelAndURL`: label `TKT-12` と URL の一部 `pull/11` のどちらでも引け、無関係の会話は出ない
  - `TestSessionByID`: 引ける / 無い / 同じ session id の dir が 2 つあれば LastSeen の新しい方

- [ ] **Step 2: Run** — Expected: FAIL

- [ ] **Step 3: Implement** — `Replace` で並べ替えた後の `views` を新しい順に回し、空でない session id ごとに最初の dir を `bySession` に入れる。`buildHaystack` は各リンクの label と URL を足す

- [ ] **Step 4: Run** `go test ./internal/index` — Expected: PASS

- [ ] **Step 5: commit** — `feat(index): リンクを索引に載せ、session id で会話を引き、絞り込みの対象にする`

---

### Task 6: 知らせの口 — NotifySession と POST /_/touch/{session} と RefreshSession

**Files:**
- Modify: `internal/store/create.go` (`NotifySession`), `internal/server/refresh.go`, `internal/server/server.go`, `main.go` (serve の配線), `internal/server/*_test.go` (`New` の呼び出し 9 か所に第 4 引数 `nil`)
- Test: `internal/store/notify_test.go`, `internal/server/refresh_test.go`, `internal/server/server_test.go`

**Interfaces:**
- Consumes: Task 4 の `ReadSessionEntry`
- Produces: `func NotifySession(baseURL, dirName string)`、`func (r *Refresher) RefreshSession(dirName string)`、`func New(cfg config.Config, ix *index.Index, refresh func(), refreshSession func(dirName string)) *Server` (nil は何もしない関数に替える)

- [ ] **Step 1: Write the failing tests**
  - `TestNotifySessionDoesNotWaitForResponse`: 受けたら channel に `r.Method+" "+r.URL.Path` を送り、2 秒眠ってから返す httptest のサーバ → `NotifySession(srv.URL, "20261111-0a1b2c3d")` は 500ms より早く戻り、サーバは `POST /_/touch/20261111-0a1b2c3d` を受ける
  - `TestNotifySessionIgnoresDeadServer`: `http://127.0.0.1:1` で panic せず 500ms より早く戻る
  - `TestRefreshSessionDoesNotRescanTranscripts`: projects に `aiTitle` 入りの jsonl を置いて `Refresh` → jsonl の `aiTitle` を書き換え、`AddLinks` で 1 件足してから `RefreshSession(dir)` → 索引のリンクは 1 件に増え、題は古い `aiTitle` のまま (全体の走査が走っていない)
  - `TestRefreshSessionDropsRemovedSession`: dir を消して `RefreshSession` → 索引から消える。ほかの会話は残る
  - `TestTouchSessionCallsOnlySessionRefresh`: `POST /_/touch/20260829-aaaa` → 204、refreshSession が `"20260829-aaaa"` で 1 回、全体の refresh は 0 回
  - `TestTouchSessionRejectsBadName`: `/_/touch/..`・`/_/touch/a.b` → 404、refreshSession は呼ばれない

- [ ] **Step 2: Run** — Expected: FAIL

- [ ] **Step 3: Implement**
  - `NotifySession`: `net.Dialer{Timeout: 100ms}` で baseURL の host に繋ぎ、書き込みの期限 100ms で `http.NewRequest(POST, …/_/touch/<PathEscape(dir)>)` を `req.Write(conn)` で書いて閉じる。応答は読まない (transport の WroteRequest で cancel する形は、本文の無い request だと flush の前に切れうるので使わない)
  - `Refresher`: `scan` で `r.meta` に transcript のメタを控える。`RefreshSession` は `r.mu` の下で `ReadSessionEntry` の結果で `r.prev[dir]` を差し替え (無ければ消し)、`r.ix.Replace(r.prev, r.meta)`。transcript は読まない
  - `handleTouchSession`: `{session}` が空でなく `store.ValidateSessionID` を通るときだけ `refreshSession` を呼んで 204、ほかは 404
  - `main.go`: `server.New(cfg, ix, r.RefreshAll, r.RefreshSession)`

- [ ] **Step 4: Run** `go test ./...` — Expected: PASS

- [ ] **Step 5: commit** — `feat(server): 1 つの会話だけを読み直す知らせの口を足し、知らせは応答を待たずに送る`

---

### Task 7: server — GET /s/<session_id>/

**Files:**
- Create: `internal/web/notfound.html`
- Modify: `internal/server/server.go`
- Test: `internal/server/sessionid_test.go` (新規)

**Interfaces:**
- Consumes: Task 5 の `SessionByID`、Task 2 の `FindSessionDir`・`ValidateSessionID`、Task 6 の `refreshSession`
- Produces: `func (s *Server) renderStatus(w http.ResponseWriter, t *template.Template, d pageData, code int)` (`render` は 200 で呼ぶ)

- [ ] **Step 1: Write the failing tests**
  - `TestSessionByIDRedirects`: `/s/sa/` と `/s/sa` → 302、`Location == "/p/20260829-aaaa/"`
  - `TestSessionByIDNotFound`: `/s/no-such/` → 404、本文に「この会話のページ・リンクはまだありません」と `href="/"`、`no-such` は本文に出ない、Content-Type は text/html
  - `TestSessionByIDRejectsBadChars`: `/s/a.b/`・`/s/%2e%2e/` → 404
  - `TestSessionByIDFallsBackToDisk`: 索引は空、`Root` の一時ディレクトリに `AddLinks` で会話を作る → `/s/<id>/` は refreshSession を `<dir>` で呼んで 302 `/p/<dir>/`

- [ ] **Step 2: Run** — Expected: FAIL

- [ ] **Step 3: Implement** — `GET /s/{session}/{$}` と `GET /s/{session}` を同じ handler に。索引に無ければ `FindSessionDir(s.cfg.SessionsDir(), id)` で探し、あれば `refreshSession(dir)` してから送る (知らせが serve に届かなかった・走査の前でも入口が切れないように)。404 は `notfound.html` の body (`<p class="empty">この会話のページ・リンクはまだありません。</p>` と `<a href="/">一覧へ</a>`) を 404 で描く

- [ ] **Step 4: Run** `go test ./internal/server` — Expected: PASS

- [ ] **Step 5: commit** — `feat(server): session id から会話のトップページへ送る /s/<session_id>/ を足す`

---

### Task 8: server — トップページの「リンク」の節と、一覧のリンクの数

**Files:**
- Create: `internal/server/links.go`, `internal/server/links_test.go`
- Modify: `internal/server/render.go`, `internal/server/server.go` (`handleSession`), `internal/web/session.html`, `internal/web/list.html`, `internal/web/layout.html`, `internal/web/static/style.css`

**Interfaces:**
- Consumes: Task 5 の `LinkView`・`Started`・`LinkCount`
- Produces: `type linkSection struct { Total int; Groups []linkGroup; Tool []linkGroup; ToolCount int }`、`type linkGroup struct { Name string; Rows []linkRow }`、`type linkRow struct { Label, Href, Title, Note, Time string; External bool }`、`func (s *Server) buildLinks(v index.SessionView) *linkSection` (0 件なら nil)、`pageData.Links *linkSection`、`sessionRow.LinkCount int`

- [ ] **Step 1: Write the failing tests** (`links_test.go`。fixture は `mkEntry` に `Links` を足した entry)
  - `TestSessionPageShowsLinks`: 組の見出しの並びが「チケット → PR → cc-page」(組の中で一番早い FirstSeen の順)、PR の中は LastSeen の新しい順、見出しが `リンク (N)` (N は全件)、節は「ページ」の後ろ
  - `TestSessionPageFoldsToolOnlyLinks`: sources `["tool"]` の件は `<details>` (open 属性なし) の中にだけ出て、summary は `tool の出力だけに出たリンク (1)`。`["assistant","tool"]` の件は上に出る
  - `TestSessionPageHidesLinksWhenNone`: リンクの無い会話に `class="links"` も「リンク」の見出しも出ない
  - `TestSessionPageLinkNotes`: 自分の cc-page の URL (`http://localhost:7777/p/20260829-aaaa/0001-notion/`、%XX の形も) の添え書きはページの題「Notion の調査」、GitHub の URL は `github.com/o/r/pull/11` (scheme なし)、61 字を超える URL は 60 字で `…`、label の無い件は scheme を除いた URL を label にして添え書きは空、group の無い件は「その他」の組
  - `TestSessionPageLinkTimes`: 最初と最後が同じ分なら `15:06` だけ、違えば `15:06〜16:38`、会話の始まりと違う日なら `2026-08-30 09:00` のように日付が付く (時刻はテストの中で `.Local()` から作る)
  - `TestSessionPageDoesNotLinkUnsafeURL`: links に `javascript:alert(1)` (label `危ない`) → `href="javascript` も `#ZgotmplZ` も出ず、`危ない` は文字で出る
  - `TestExternalLinksHaveRel`: 外への `<a>` に `rel="noopener noreferrer"`、自分の cc-page には付けない
  - `TestListShowsLinkCount`: 一覧に `2 リンク`、リンクの無い会話には出ない。ページが 0 でリンクだけの会話も一覧に出る。`/?q=TKT-12` で絞り込める
  - `TestSearchPlaceholderMentionsLinks`: placeholder に `リンク` を含む

- [ ] **Step 2: Run** — Expected: FAIL

- [ ] **Step 3: Implement**
  - `buildLinks`: sources に `assistant` を含む件は `Groups`、`tool` だけの件は `Tool`。組の名前は `Group` (空なら `その他`)。組の並びは組の中の最小の FirstSeen (同じなら名前)、組の中は LastSeen の新しい順 (同じなら URL)
  - 行: `Href` は `safeHref` (http/https・Host あり・空白と制御文字なし) を通ったときだけ。`External` は `selfTitle` の対象 (自分の host:port) でないもの。`Label` は label か `shortURL`、`Note` は自分の cc-page ならその題 (`ix.Page`・`/p/<session>/` なら会話の題)、label があれば `shortURL`、label が無ければ空。`Title` は URL 全体
  - 自分の host: `s.cfg.Addr` の port と同じで、hostname が `localhost`・`127.0.0.1`・`::1` か Addr の host
  - `timeRange(first, last, started)`: 上のテストの形。`started` がゼロなら first の日を基準に
  - `session.html`: ページの `<ol>` の後ろに `{{with .Links}}<section class="links">…{{end}}`、組は `{{define "linkgroup"}}` (`<h3>` + `<ul>`、行は label・`<span class="desc">`・`<span class="time">`)
  - `list.html`: `{{if $sess.LinkCount}}<span>{{$sess.LinkCount}} リンク</span>{{end}}` をページ数の隣に
  - `layout.html`: placeholder を `タイトル・要約・タグ・依頼・状況・リンク・cwd・ブランチ` に
  - `style.css`: `.links ul` を `grid-template-columns: fit-content(40%) minmax(0, 1fr) max-content` の grid にし、`li` を subgrid にして列を行どうしで揃える。`.desc` は 1 行で省略、`.time` は折り返さない

- [ ] **Step 4: Run** `go test ./...` — Expected: PASS

- [ ] **Step 5: commit** — `feat(web): 会話のトップページにリンクの節を出し、一覧にリンクの数を出す`

---

### Task 9: CLI — cc-pages link

**Files:**
- Modify: `main.go`, `main_test.go`

**Interfaces:**
- Consumes: Task 1 の `NewLinkInput`・`ParseLinkLines`、Task 3 の `AddLinks`・`LinkResult`、Task 6 の `NotifySession`
- Produces: `type linkArgs struct { SessionID string; Stdin bool; URL, Label, Group, Source, At string }`、`func linkFlags(a *linkArgs) *flag.FlagSet`、`func cmdLink(cfg config.Config, args []string, stdin io.Reader, stdout, stderr io.Writer) error`

- [ ] **Step 1: Write the failing tests**
  - `TestLinkStdinWritesAndPrintsJSON`: 3 行 (うち 1 行は `{"url":"javascript:x"}`) → err nil、stdout は 1 行の JSON で `url == "http://localhost:1/p/<dir>/"`・`added == 2`・`updated == 0`・`skipped == 1`、stderr に `2 行目` (など捨てた行の番号) と理由
  - `TestLinkFlagsSingleItem`: `--url https://github.com/o/r/pull/12 --label r#12 --group PR --source tool --at 2026-11-11T15:06:01Z` → links.json に 1 件で各フィールドが入る
  - `TestLinkFlagBadItemFails`: `--url javascript:x` → err、stdout は空
  - `TestLinkFlagErrors` (どれも err・stdout 空): `--stdin` も `--url` も無い / 両方ある / `--stdin` と `--label` / `--session` が無い / `--session ../x`
  - `TestLinkEmptyStdinCreatesNothing`: 空の stdin → err nil、`added/updated/skipped` が 0、`url` は `/s/<id>/`、root に `sessions` を作らない
  - `TestLinkNotifiesServe`: httptest のサーバの addr を `cfg.Addr` にして足す → サーバが `POST /_/touch/<dir>` を受ける。同じ入力をもう一度渡すと知らせは来ない
  - `TestLinkFlagsFillInput`: `linkFlags` が全部の flag を `linkArgs` に結ぶ (`TestStatusFlagsFillInput` と同じ形)
  - `TestRunListsLinkSubcommand`: `run(nil)` のエラーに `link` が載る

- [ ] **Step 2: Run** `go test . -run Link` — Expected: FAIL

- [ ] **Step 3: Implement** — `run` に `case "link": return cmdLink(cfg, args[1:], os.Stdin, os.Stdout, os.Stderr)`、使い方の文を `(new / serve / status / link)`。`cmdLink` は flag を読み、session id を stdin より先に確かめ、flag の組み合わせを `fs.Visit` で確かめる。stdin なら `ParseLinkLines`、flag なら `NewLinkInput` 1 件 (err なら stdout に何も書かずに返す)。flag の 1 件が `AddLinks` で捨てられた (上限) ときも err。`res.Changed` のときだけ `NotifySession(cfg.BaseURL(), res.Dir)`、最後に `json.NewEncoder(stdout).Encode(res)`。warn は stderr に `cc-pages link: ` を付けて 1 行

- [ ] **Step 4: Run** `go test ./...` — Expected: PASS

- [ ] **Step 5: commit** — `feat(cli): cc-pages link で会話にリンクを足す`

---

### Task 10: e2e — リンクの節を実ブラウザで見て、スクショを撮る

**Files:**
- Create: `tests/e2e/fixtures/root/sessions/20260101-e2efixture/links.json`, `tests/e2e/links.spec.ts`

- [ ] **Step 1: fixture** — 組が 4 つ (チケット・PR・cc-page・その他) と tool だけの 1 件、PR の 1 件は翌日 (2026-01-02) の時刻、cc-page は `http://localhost:7788/p/20260101-e2efixture/0001-diff-の配色/` (label `📄0001`)
- [ ] **Step 2: spec** (light と dark の 2 つの describe)
  - `section.links` の h2 が `リンク (6)`、h3 の並びが fixture の FirstSeen の順
  - cc-page の行の添え書きが `diff の配色`
  - `details` は閉じていて、tool だけの件は見えない。summary を押すと見える
  - details を開いた状態で `10-session-links-light.png` / `11-session-links-dark.png` (fullPage)
  - `/s/e2e-fixture/` を開くと `/p/20260101-e2efixture/` に着く。`/s/no-such/` は「この会話のページ・リンクはまだありません」
  - 一覧の会話の行に `6 リンク`
- [ ] **Step 3: Run** CI に任せる (手元で回すなら `cd tests/e2e && npm ci && nix develop ../.. --command npx playwright test`)。既存の `session.spec.ts` (状況板) の検査が、リンクの節を足しても通ること
- [ ] **Step 4: commit** — `test(e2e): 会話のトップページのリンクの節を検査し、スクショを撮る`

---

### Task 11: README・spec の細部・実物で確かめる

**Files:**
- Modify: `README.md`, `docs/superpowers/specs/2026-11-11-session-links-design.md` (契約の節は触らず、「作りながら決めたこと」の節を足す)

- [ ] **Step 1: README** — 使い方に `cc-pages link` の例 (flag の形と `--stdin`)、JSON Lines のキー・stdout・終了コード・まとめ方・5000 件の要点、`/s/<session_id>/`、設計の文書の場所
- [ ] **Step 2: spec** — 作りながら決めた細部 (0 件では作らない・`/s/` を返す、空行は数えない、label 200 字・group 50 字、行 64 KiB・入力 16 MiB、壊れた links.json は退ける、知らせの口 `POST /_/touch/{session}`、`/s/` は索引に無ければディスクも見る、session.json も一時ファイルから書く、lock は linux・darwin の flock)
- [ ] **Step 3: 実物** — `go build -o cc-pages .`、一時ディレクトリを `CC_PAGES_ROOT`、空いている port を `CC_PAGES_ADDR` にして `./cc-pages serve` を起こし、`--stdin` と flag で足す → `curl` で `/p/<dir>/` にリンクの節、`/s/<id>/` の 302、`/s/x/` の 404、一覧の数。知らせで再読み込みボタンを押さずに出ること。ブラウザ (Playwright MCP) で見た目を light・dark で確かめる
- [ ] **Step 4: commit** — `docs(readme): cc-pages link と /s/<session_id>/ を書く`・`docs(spec): 作りながら決めた細部を書き足す`
