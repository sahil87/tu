# Plan: Leaderboard (Go port row B5)

**Change**: 260916-2gbb-leaderboard-lb-lbh
**Intake**: `intake.md`

## Requirements

> Every surface below is Goal-frozen: the Go binary reproduces the shipped TypeScript bytes. Oracle sources: `src/node/core/leaderboard.ts` (`currentWindow`, `previousWindow`, `sumByKey`, `buildLeaderboard`), `src/node/core/cli.ts` (`main()` guards at lines ~1914–1994; `fetchLeaderboardRows`, `fetchLeaderboardHistory`, `foldLeaderboardColumns`, `sumLeaderboardToolMaps`, `readAllUsersByUserMachine`, `lbhFormatOptions`, `lbhTitle`, `renderLeaderboardByFormat`, `formatLastSync`), `src/node/tui/formatter.ts` (`renderLeaderboard`, `fmtDeltaCell`, `leaderboardRowsToJson`, `emitCsvLeaderboard`/`csvShare`, `emitMarkdownLeaderboard`/`mdShare`/`mdDelta`, the `historyTitle`/`columnOrder`/`highlightRowLeader`/`omitNegligibleColumns` hooks in `renderTotalHistory`). Contract text: `docs/specs/usage.md` § Displays, § Flags, § Exit Codes, § Data flow, § Leaderboard Table, § Leaderboard History Table, § JSON/CSV/Markdown, DC-07/08/11/13/14/16; `docs/specs/layouts.md` § 5, § 6, § 12, § 15, § 16, § 17, § 18, § 19. **Read `intake.md` § What Changes before starting — its design values (§ 1–§ 13) are normative and more detailed than the summaries here.** Byte references appear under `bin/harness/report/cases/<id>/…/node.*` after `env -u TU_METRICS_REPO -u NO_COLOR just go-diff --placeholder`; the node side is the oracle.

### Command: gate, guards, scope

#### R1: The multi-mode gate
`command.Run` SHALL return `ErrLeaderboardMode` — `errors.New("Error: lb requires multi mode — run tu init-metrics <repo-url> to set up a metrics repo")` — when `Display ∈ {Leaderboard, LeaderboardHistory}` and `cfg.Mode == config.Single`, evaluated on the post-guard config **before** `Normalize` runs, so no notice is produced. `cmd/tu` prints the error text on stderr with `ExitOperational` through its existing non-`ErrUnported` error path (no edge change). The message names `lb` for `lbh` too (DC-14).

- **GIVEN** a single-mode config and `Request{Display: LeaderboardHistory, Flags: {ByMachine: true, User: "x"}}`
- **WHEN** `Run` executes
- **THEN** it returns `ErrLeaderboardMode` with an empty `Result` (no notices, no lines), and the e2e binary prints exactly the one stderr line, empty stdout, exit 1

#### R2: Normalize — leaderboard exemptions and the `--top` guard
`Normalize` SHALL follow the TS `main()` order (intake § 2): (0) the `-u` single-mode notice skips `lb`/`lbh`; (0') `-u all` on `lb`/`lbh` clears `User` silently; (0a/0b) unchanged; (1) since/until are kept on `History`, `Leaderboard`, `LeaderboardHistory` and warn-and-clear elsewhere; (2) `--full` warns on every display except `History` and `LeaderboardHistory` (so `lb --full` warns); (2b) `Top != 0` on a display other than `lb`/`lbh` appends `Warning: --top applies to leaderboard display — ignoring.` and sets `Top = 0`, positioned after the `--full` notice and before the cap; (3) the cap applies to `History` and `LeaderboardHistory` (period ≠ monthly, no bound, `!Full`).

- **GIVEN** single mode, `Request{Flags: {User: "other", Top: 2, Full: true}}` (snapshot)
- **WHEN** `Normalize` runs
- **THEN** notices are exactly `[-u line, --full line, --top line]` in that order and `Top == 0`
- **AND** multi mode `Request{Display: LeaderboardHistory, Period: Daily}` gets `Since = ThreeMonthFloor(now)` and `capActive == true`; `Request{Display: Leaderboard, Period: Daily}` gets no cap
- **AND** `Request{Display: Leaderboard, Flags: {Since: "2026-01-01", Until: "2026-01-31", User: "all"}}` in multi mode keeps both bounds, clears `User`, emits no notice

#### R3: Scope
`inScope` SHALL admit `Display ∈ {Leaderboard, LeaderboardHistory}` and `Flags.Top != 0`. `Watch`, `Sync`, `DryRun`, `NoRain`, `SkipBrewUpdate` MUST still return `ErrUnported`.

- **GIVEN** `Request{Display: Leaderboard, Flags: {Top: 3}}` and `Request{Display: Leaderboard, Flags: {Watch: true}}` in multi mode
- **WHEN** `Run` executes
- **THEN** the first renders and the second returns `ErrUnported`

### Leaderboard data

#### R4: Windows
`command` SHALL compute the current and previous windows exactly per intake § 3's table: period windows anchored on `deps.Now()` in local time (`query.CurrentLabel`), the monthly previous window's bounds via `time.Date(y, m-1, 1)` / `time.Date(y, m, 0)` (local) labelled with the English 3-letter month; explicit `--since S [--until U]` replaces the period window with heading label `S → U` / `S →` / `→ U`; the previous window under an explicit bound is the equal-length range ending the day before `S` (label `prev`, `nil` when length < 1 or when only `--until` is given). Day arithmetic is UTC-parsed on the ISO strings.

- **GIVEN** `now = 2026-09-17` local, no bounds
- **WHEN** windows are computed for daily, weekly, monthly
- **THEN** current = (`2026-09-17`,`2026-09-17`,`2026-09-17`), (`2026-09-13`,`2026-09-17`,`2026-09-13`), (`2026-09-01`,`2026-09-17`,`2026-09`); previous = (`2026-09-16`,`2026-09-16`,`2026-09-16`), (`2026-09-06`,`2026-09-12`,`2026-09-06`), (`2026-08-01`,`2026-08-31`,`Aug`)
- **AND** `--since 2026-01-01 --until 2026-01-31` gives current label `2026-01-01 → 2026-01-31` and previous (`2025-12-01`,`2025-12-31`,`prev`); `--until 2026-01-31` gives label `→ 2026-01-31` and a nil previous window

#### R5: Ranking, shares, deltas
`command` SHALL rank per intake § 3: `daily = SortByDate(Collapse(raw, User[, Machine], Date))`; `cur = GroupBy(Window(daily, curStart, curEnd), dims…)`; `prev` likewise (nil when no previous window); drop keys with `TotalTokens == 0 && TotalCost == 0`; sort descending by `metricValue(totals, metric)` with ties by key ascending byte order; `grand` = left fold over the ranked rows from 0; `Share = value/grand` (0 when grand is 0); `Delta = (value − prevValue)/prevValue` when `prevValue != 0`, else nil; `Rank = i+1`. `Result.TotalCost`/`TotalTokens` fold over all ranked rows; `Result.CostByItem[key]` = `metricValue` keyed `user` or `user/machine`; `Warnings` nil.

- **GIVEN** the committed seed, multi mode, `lb --since 2026-01-01 --until 2026-01-31`
- **WHEN** `Run` executes
- **THEN** rows are `1 harness-user $1.70 97,600 tokens`, `2 other-user $1.30 48,800 tokens`; shares `56.7%` / `43.3%`; both deltas nil (`new`); Total `$3.00` / `146,400`
- **AND** with `--by-machine -t` the three keys tie at 48,800 tokens and rank `harness-user/harness-machine`, `harness-user/other-box`, `other-user/laptop`
- **AND** a `Run` test with an association-sensitive triple pins that the per-day collapse sums in record input order and the window sum folds ascending by date

### View and encoder

#### R6: `view.Leaderboard`
`view` SHALL define `LeaderboardRow{Rank int; User, Machine string; fact.Totals; Share float64; Delta *float64}`, `LeaderboardOptions{Period, WindowLabel, DeltaLabel, Metric, PinnedUser, Top, Width, LastSync, Prev}` and `Leaderboard(rows, o) Table` implementing intake § 4 byte for byte: title without `📊`, the staleness `Footer`, `Empty = "  No data"` on zero rows, visible-row slicing under `Top`, the `… +k others` collapsed Data row (dim name cell, blank cells), the ` ◂` pin, widths over visible rows with grand totals over all rows, columns `#`/`User`/`Cost`(`BarAfter`)/`Tokens`/`Share`/`Δ vs {label}`, dim exact-zero Cost/Tokens cells, `FixedHalfUp(share×100, 1)%` share cells, `fmtDeltaCell` (`new` / signed `jsRound(delta×100)%`), the bar through `barBudget` with `bodyWidth` = every non-metric column plus gutters and `reserve = 1` when `Prev != nil`, and the Total row (blank rank/share/delta cells) when `len(rows) ≥ 2`.

- **GIVEN** the layouts § 5 six-row data at Width 80 under cost
- **WHEN** `view.Leaderboard` builds and `ansi.Table` encodes
- **THEN** the golden matches the mockup's structure: rank width 1, `sahil ◂`, a 19-glyph bar budget between Cost and Tokens, `Δ vs Aug`, Total `$18,761.10`, dim footer
- **AND** zero rows yield `"", title, "", "  No data", "", Dim(footer), ""`
- **AND** `Top = 2` on six rows yields two Data rows, a dim `… +4 others` row whose label widens the User column, and a Total over all six

#### R7: Table model extensions and the ANSI encoder
`view.Column` SHALL gain `BarAfter bool` and `view.Cell` SHALL gain `Leader bool` (both zero-valued ⇒ today's bytes). `ansi.Table` SHALL: when a column has `BarAfter` and `Scale.Width > 0`, insert a bar area of exactly `1 + Scale.Width` visible characters after that column's cell on every non-divider row — Data rows `" " + Green(Main) + spaces(Width − runes(Main))` single-zone (two-zone unchanged), Header/Total/bar-less Data rows `" " + spaces(Width)` unstyled — and place `barDiv` after that column in dividers; wrap `Leader` cells `BoldWhite` after padding and any `Dim` wrap; on the Empty branch append `Dim(Footer), ""` after the blank when `Footer != ""`. Without `BarAfter`/`Leader`/empty-footer inputs every existing golden MUST stay byte-identical.

- **GIVEN** the existing `render/ansi` goldens
- **WHEN** the suite runs after the encoder change
- **THEN** every existing golden passes unchanged
- **AND** a pivot row whose leader cell is exactly zero encodes `BoldWhite(Dim(padded))` (the TS double wrap)

#### R8: `view.TotalHistory` hooks
`HistoryOptions` SHALL gain `Title string` (override; `""` = today's title), `RankColumns bool` (reorder `visible` and each row's values by descending window total in the metric, stable on ties, after `pivotData`), `HighlightLeader bool` (per Data row the first strict maximum gets `Cell.Leader`; index 0 when all values are equal), `KeepAllColumns bool` (skip `significant`: every series is a column, all-zero ones included). Widths, header, cells, `Segments`, `Legend` swatches and the Total row follow the reordered `visible`.

- **GIVEN** series `sahil`, `alice`, `bob`, `eunice` with window totals 767.50, 615.70, 389.05, 0 and `RankColumns`, `HighlightLeader`, `KeepAllColumns`
- **WHEN** `TotalHistory` builds
- **THEN** columns are `Date, sahil, alice, bob, eunice, Cost`, `eunice` renders dim `$0.00` cells, and each row's max cell has `Leader` set
- **AND** with all four options unset the table equals today's `pivot_*` goldens

### Render helpers and machine formats

#### R9: `render.FixedHalfUp` and `jsRound`
`render` SHALL export `FixedHalfUp(x float64, digits int) string` — the exact-binary half-up rounding `csv.Cost` implements today, generalised to `digits ∈ {1, 2}`, magnitude rounded, sign re-added, `-0` → `0` — and `csv.Cost(x)` SHALL become `FixedHalfUp(x, 2)` (its Node-verified table stays as the test). A `jsRound(x) = math.Floor(x + 0.5)` helper (negative zero normalised) SHALL back every `Math.round` twin.

- **GIVEN** `12.25`, `0.05`, `1.005`, `-0.3`
- **WHEN** `FixedHalfUp(x, 1)` / `FixedHalfUp(x, 2)` run
- **THEN** `12.3`, `0.1` (0.05 is above the exact tie), `1.00` (the Node-verified `1.005 → 1.00`), `-0.3`/`-0.30`
- **AND** `jsRound(-553.5) == -553`, `jsRound(2.5) == 3`, `jsRound(-0.4)` formats as `0`

#### R10: JSON leaderboard
`json.Leaderboard(rows []view.LeaderboardRow) []string` SHALL emit `[]` inline when empty, else the `JSON.stringify(v, null, 2)` layout with keys `rank`, `user`, [`machine` when `Machine != ""`], `cost` (`encodeFloat`), `totalTokens`, `share` (`encodeFloat`), `delta` (`encodeFloat` or `null`). `lbh --json` SHALL reuse `renderjson.TotalHistory(series)` unchanged with users in `Repo.Users()` order and `others` last.

- **GIVEN** rows `{1, sahil, "", cost 12945.642098040002, 15962442751, share 0.6900258986630633, delta -0.5532817482507824}` and `{2, bob, "", …, delta nil}`
- **WHEN** encoded
- **THEN** the output equals layouts § 12's array shape with `"delta": null` on the second object

#### R11: CSV leaderboard
`csv.Leaderboard(rows, all []view.LeaderboardRow, byMachine bool) []string` SHALL emit header `rank,user[,machine],cost,total_tokens,share,delta`, one row per sliced row with `Cost(TotalCost)`, raw tokens, `csvShare(Share)` and `csvShare(*Delta)` or empty, where `csvShare(n) = FormatFloat(jsRound(n×1000)/1000, 'f', -1, 64)`; a `Total,,{cost},{tokens},,` row (one more empty field under `byMachine`) when `len(all) > 1`; header alone when empty. `lbh --csv` SHALL reuse `csv.TotalHistory(series)`.

- **GIVEN** the seed window rows and `byMachine == false`
- **WHEN** encoded
- **THEN** lines are `rank,user,cost,total_tokens,share,delta`, `1,harness-user,1.70,97600,0.567,`, `2,other-user,1.30,48800,0.433,`, `Total,,3.00,146400,,`
- **AND** `--top 1` still emits the same Total row after one data row

#### R12: Markdown leaderboard and the pivot title parameter
`markdown.Leaderboard(rows, all, period, deltaLabel, byMachine) []string` SHALL emit `## Leaderboard ({period})`, blank, header `#, User, [Machine,] Cost, Tokens, Share, Δ vs {deltaLabel}` with alignment `---:, :---, [:---,] ---:, ---:, ---:, ---:`, data rows (`render.FormatCost`, `render.FormatInt`, `FixedHalfUp(share×100, 1)%`, `fmtDeltaCell`), a `**Total**` row when `len(all) > 1`, trailing blank. `markdown.TotalHistory` SHALL take the heading text as a parameter (`"Combined Cost History"` from existing callers; `"Leaderboard History"` / `"Leaderboard Token History"` from `lbh`).

- **GIVEN** the layouts § 16 leaderboard data
- **WHEN** encoded
- **THEN** the output equals the mockup, with the Total row `| **Total** |  | **$18,761.10** | **23,730,948,614** |  |  |`
- **AND** `tu m lbh --md` against the seed prints `## Leaderboard History (monthly)` and both users as columns

### Composition

#### R13: `runLeaderboard` and `runLeaderboardHistory`
`command` SHALL add both run paths per intake § 11. `runLeaderboard` threads windows → ranking → the sliced/all row sets into `json`/`csv`/`markdown`/`ansi.Table(view.Leaderboard(...))` with `PinnedUser = Flags.User` when set else `cfg.User`, `LastSync = deps.LastSync()`. `runLeaderboardHistory` builds one `view.Series` per `Repo.Users()` profile as the left fold in record input order over `Relabel(Window(recs_u, since, until), period)` grouped by `Date` (entries sorted ascending by label; an empty series kept), applies `foldColumns(series, Top, metric)` (stable descending sort by series total, keep the top N in original order, append `others` last when anything was folded), and renders through `renderjson.TotalHistory` / `csv.TotalHistory` / `markdown.TotalHistory(…, title)` / `ansi.Table(view.TotalHistory(series, opts))` with `Title = "📊 Leaderboard History (" + PeriodLabel + ")"` (`Token` under tokens) and the three hooks on. `Result` fields as `runHistory` computes them.

- **GIVEN** the seed, multi mode, `m lbh`
- **WHEN** `Run` executes
- **THEN** one `2026-01` row with columns ranked `harness-user` (`$1.70`), `other-user` (`$1.30`), the leader cell bold, row total `$3.00`, no Total row (one label), title `📊 Leaderboard History (monthly)`
- **AND** `m lbh --top 1` folds `other-user` into `others` (JSON keys `harness-user`, `others` in that order; ANSI columns ranked by total)
- **AND** a `Run` test with two machines on one day for one tool and a second tool pins the tool-major left fold (`((m1_d5 + m1_d6) + m2_d6) + codex`), distinct from `Collapse`-then-`RollUp`

#### R14: `config.LastSync` and `Deps.LastSync`
`config` SHALL export `LastSync(stateDir string, now time.Time) string` (the current unexported `lastSync`, used by `Status`); `command.Deps` SHALL gain `LastSync func() string`, evaluated only on the `lb` path, `nil` reading as `"never"`; `cmd/tu` SHALL set it to a closure over `config.StateDir(paths.Home)` and `time.Now`.

- **GIVEN** a staged home with `.tu/.last-sync` = `2026-09-15T18:54:44.502Z` and a fixed `Now` 15 minutes later
- **WHEN** `lb` renders
- **THEN** the footer is `synced 15m ago (2026-09-15T18:54:44.502Z) · tu sync to refresh`; without the file it is `never synced · tu sync to refresh`

### Edge and harness

#### R15: e2e and harness gate
`cmd/tu/e2e_test.go` SHALL replace the leaderboard placeholder expectations with the assertions listed in intake § 12; `run_test.go` SHALL move the seven leaderboard/`top` rows out of `TestRunUnported`; `harness/matrix.json` SHALL gain the 28 groups listed in intake § 13. `env -u TU_METRICS_REPO -u NO_COLOR just go-diff --placeholder` MUST report every case whose id contains `lb` green, no previously green case red, and only B6's `sync*`/`*-sync*` cases remaining red.

- **GIVEN** the rebuilt `bin/tu` and `dist/tu.mjs`
- **WHEN** the harness runs
- **THEN** the summary's red count equals B6's 12 and the green count equals the pre-change baseline plus the 44 flipped cases plus every new case

### Non-Goals
- `--watch` on either leaderboard, the live `Prev` maps and the indicator reserve in effect — B7 (the seams are wired).
- `--sync` / `tu sync` — B6.
- Any edit to `docs/specs/` or `src/node/`; DC-07/08/11/13/14/16 are reproduced, not resolved.

### Design Decisions

#### One ANSI encoder, extended by two zero-valued flags
**Decision**: The leaderboard renders through `ansi.Table` via `Column.BarAfter` (mid-row bar area, always `1 + Width` visible chars) and `Cell.Leader` (BoldWhite after pad and Dim), plus a footer on the Empty branch.
**Why**: Every other leaderboard rule (pad-then-Dim, BoldWhite Total cells, BoldCyan headers, `─|─` dividers) already matches the TS; a dedicated encoder would duplicate them. Zero values keep every existing golden byte-identical.
**Rejected**: A `render/ansi.Leaderboard` encoder mirroring the TS `renderLeaderboard` — a second 100-line pathway with the same rules.
*Introduced by*: 260916-2gbb-leaderboard-lb-lbh

#### Leaderboard ranking as collapse-sort-window-group
**Decision**: `GroupBy(Window(SortByDate(Collapse(raw, User[, Machine], Date))), dims…)` per window, sharing the grand total in ranked order.
**Why**: Reproduces the TS float association exactly — `mergeEntries` folds tools and machines per label in input order, `sumByKey` folds labels ascending, `grand` folds the sorted `kept` — and the JSON `cost`/`share`/`delta` are raw doubles (DC-10). Both windows come from one collapsed record set, like the TS second filter pass.
**Rejected**: `GroupBy(Window(raw), dims…)` directly — sums in walk order across days, a different association.
*Introduced by*: 260916-2gbb-leaderboard-lb-lbh

#### `lbh` values as a tool-major left fold
**Decision**: Per user, `GroupBy(Relabel(Window(recs_u)), Date)` over the gather-ordered records, not `Collapse(Tool, Date)` then `RollUp`.
**Why**: The TS `aggregateMachineMap` aggregates each tool's unmerged machine-major entries into the period and `sumLeaderboardToolMaps` then adds tools in registry order — one left fold over the same ordered sequence. The main history's collapse-then-roll-up association is a different fold and would differ in the last bit.
**Rejected**: Reusing `runHistory`'s tail — byte drift in `lbh --json`.
*Introduced by*: 260916-2gbb-leaderboard-lb-lbh

#### Exact JS rounding twins in `render`
**Decision**: `render.FixedHalfUp` (big.Rat exact half-up, generalising `csv.Cost`) for `toFixed(d)` and `jsRound = floor(x + 0.5)` for `Math.round`.
**Why**: Go's `FormatFloat` is half-even on exact ties (`12.25 → 12.2`) and `math.Round` is half-away-from-zero (`-553.5 → -554`); JS gives `12.3` and `-553`. Share, Δ and CSV fractions are byte surfaces.
**Rejected**: `strconv.FormatFloat(x, 'f', 1, 64)` / `math.Round` — correct on almost every input, wrong on the ties the seed can hit.
*Introduced by*: 260916-2gbb-leaderboard-lb-lbh

#### `.last-sync` reaches `command` as a closure
**Decision**: `Deps.LastSync func() string`, built at the edge over `config.LastSync(StateDir(home), time.Now())`, called only by the `lb` path.
**Why**: `command` stays I/O-free (target architecture); the closure mirrors `Deps.Now`; no other command pays the file read.
**Rejected**: Passing `StateDir` into `command` and reading the file there — I/O below the edge.
*Introduced by*: 260916-2gbb-leaderboard-lb-lbh

## Tasks

### Phase 1: Helpers, config, guards, scope

- [x] T001 In `src/go/internal/render/format.go` add `FixedHalfUp(x, digits)` (lift the big.Rat body out of `csv.Cost`) and `JSRound(x)`; make `src/go/internal/render/csv/csv.go` `Cost` call `FixedHalfUp(x, 2)`; tests in `format_test.go` (the Node-verified 2-digit table, the 1-digit ties `12.25→12.3`, `-0→0`, `JSRound(-553.5) == -553`) <!-- R9 -->
- [x] T002 [P] In `src/go/internal/config/status.go` export `LastSync(stateDir, now)` (rename `lastSync`; `Status` keeps calling it); add `LastSync func() string` to `Deps` in `src/go/internal/command/run.go` with a nil-safe accessor returning `"never"` <!-- R14 -->
- [x] T003 [P] In `src/go/internal/command/guards.go` add `topNotice`, thread the leaderboard exemptions through steps 0/0'/1/2/2b/3 per R2, update the doc comment; `guards_test.go`: the ordering case, the `lb`/`lbh` exemptions, the `--top` clear, the `lbh` cap and the `lb` no-cap <!-- R2 -->
- [x] T004 In `src/go/internal/command/run.go` add `ErrLeaderboardMode` and the pre-`Normalize` gate in `Run`; let `inScope` admit the two displays and `Top`; route `Display == Leaderboard` / `LeaderboardHistory` to the (stub-then-real) `runLeaderboard`/`runLeaderboardHistory`; `run_test.go`: move the seven leaderboard/`top` rows out of `TestRunUnported`, add the gate test (single mode, notices absent) <!-- R1, R3 -->

### Phase 2: Windows and view model

- [x] T005 Add `src/go/internal/command/leaderboard.go` with the window computation (`leaderboardWindows(period, since, until, now) (cur, prev)` returning start/end/label triples, prev nil per R4) using `query.CurrentLabel`, UTC day arithmetic and local month bounds; table-driven `leaderboard_test.go` covering the six R4 rows <!-- R4 -->
- [x] T006 In `src/go/internal/view/table.go` add `Column.BarAfter` and `Cell.Leader`; add `src/go/internal/view/leaderboard.go` with `LeaderboardRow`, `LeaderboardOptions`, `Leaderboard(rows, o)` and helpers (`fmtDeltaCell`, share cell, widths, collapsed row, Total row, bar budget); `leaderboard_test.go`: widths over visible rows vs grand over all, rank width flip at 10, pin marker, collapsed label width, empty state with footer, `Top ≥ len` no collapse <!-- R6, R7 -->
- [x] T007 In `src/go/internal/view/history.go` add the four `HistoryOptions` hooks and in `pivot.go` implement `Title`, `RankColumns` (stable reorder after `pivotData`), `HighlightLeader` (first strict max, index 0 on ties), `KeepAllColumns`; `pivot_test.go`: the R8 scenario, defaults unchanged <!-- R8 -->

### Phase 3: Encoders

- [x] T008 In `src/go/internal/render/ansi/table.go` implement the `BarAfter` bar area (Data padded single-zone, unstyled gap on Header/Total/bar-less rows, `barDiv` placement in dividers), the `Leader` wrap order, and the Empty-branch footer; goldens `leaderboard_color`, `leaderboard_nocolor`, `leaderboard_tokens`, `leaderboard_top`, `leaderboard_by_machine`, `leaderboard_wide`, `leaderboard_two_zone`, `leaderboard_empty`, `leaderboard_never_synced`, `leaderboard_single_row`, `pivot_lbh_ranked`, `pivot_lbh_top_others`; existing goldens unchanged <!-- R7 -->
- [x] T009 [P] Add `src/go/internal/render/json/leaderboard.go` (`Leaderboard(rows)`) per R10 with goldens `leaderboard`, `leaderboard_by_machine`, `leaderboard_new_delta`, `leaderboard_empty`, plus `total_history_lbh_top` for the `others`-last pivot shape <!-- R10 -->
- [x] T010 [P] In `src/go/internal/render/csv/csv.go` add `Leaderboard(rows, all, byMachine)` and `csvShare` per R11; goldens `leaderboard`, `leaderboard_by_machine`, `leaderboard_top_total`, `leaderboard_empty` <!-- R11 -->
- [x] T011 [P] In `src/go/internal/render/markdown/markdown.go` add `Leaderboard(rows, all, period, deltaLabel, byMachine)` per R12 and give `TotalHistory` a `title string` parameter (update `command` callers); goldens `leaderboard`, `leaderboard_by_machine`, `leaderboard_top_total`, `leaderboard_empty`, `total_history_lbh_title` <!-- R12 -->

### Phase 4: Composition, edge, harness

- [x] T012 In `src/go/internal/command/leaderboard.go` implement ranking per R5 (`rankLeaderboard(raw, dims, cur, prev, metric)` → `[]view.LeaderboardRow`) and `runLeaderboard` (formats, `PinnedUser`, `LastSync`, `Result` fields); `leaderboard_test.go`/`run_test.go`: the seed-derived R5 scenario with a fake `Repo`, the tie-break, the drop rule, the `share` fold order, delta nil cases, `--top` slice vs full-set totals, `-u all`/`-u name` pin semantics, no ccusage call on the fake `Fetcher` <!-- R5, R13 -->
- [x] T013 In `src/go/internal/command/leaderboard.go` implement `runLeaderboardHistory` and `foldColumns` per R13 (per-user left fold, `others` last, title/hook options, `Result` fields); tests: the `m lbh` seed scenario, `--top 1` fold and `Top ≥ users`, the tool-major association triple <!-- R13 -->
- [x] T014 In `src/go/cmd/tu/main.go` set `Deps.LastSync` and update the package comment; in `e2e_test.go` add the intake § 12 assertions (single-home `lb`/`lbh`/`lb -u other-user` exit 1; multi-home `lb` today empty + footer, `m lbh`, the January window in table/json/csv/md, `--by-machine`, `--by-machine -t` tie, `--top 1`, `-t`, `cc lb`, `--until`-only heading, `lbh --since/--until`, `m lbh --top 1`, `lbh --by-machine` warning, `--top 3` snapshot warning, `-u all` no notice, a staged `.last-sync` footer with fixed `Now`) <!-- R14, R15 -->
- [x] T015 Add the 28 matrix groups from intake § 13 to `harness/matrix.json`; run `just go-lint`, `just go-test`, and `env -u TU_METRICS_REPO -u NO_COLOR just go-diff --placeholder`; fix until every case whose id contains `lb` is green, no previously green case regressed, and only `sync*`/`*-sync*` cases remain red <!-- R15 -->

## Execution Order

- T001–T004 first (T001 before T010/T011 goldens; T002/T003 are independent of each other; T004 stubs the run paths so the tree compiles).
- T005 and T006 are independent; T006 blocks T007 (shared `Cell.Leader`) and T008.
- T009–T011 are independent of each other; T011's `TotalHistory` signature change must land before T013 compiles.
- T012 needs T005, T006, T008–T011; T013 needs T007, T011; T014–T015 last.

## Acceptance

### Functional Completeness

- [x] A-001 R1: `ErrLeaderboardMode` exists with the byte-exact message, is returned before `Normalize` on the post-guard single-mode config, and the e2e binary prints one stderr line, empty stdout, exit 1 for `lb`, `lbh`, `lb -u other-user`
- [x] A-002 R2: `Normalize` follows the TS order — `-u` exemption for `lb`/`lbh`, silent `-u all` clear, since/until kept on the leaderboards, `--full` warns on `lb` only, `--top` notice byte-exact and positioned after `--full`, cap on `lbh`
- [x] A-003 R3: `inScope` admits both displays and `Top`; `Watch`/`Sync`/`DryRun`/`NoRain`/`SkipBrewUpdate` still `ErrUnported`
- [x] A-004 R4: All six window rows (daily/weekly/monthly/both bounds/since-only/until-only) match, including the monthly short-month label and the nil previous window
- [x] A-005 R5: Ranking, drop rule, tie-break, share and delta rules implemented exactly; `Result.CostByItem` keyed by `user` or `user/machine`
- [x] A-006 R6: `view.Leaderboard` reproduces every intake § 4 rule (title, footer, empty state, visible slicing, collapsed row, pin, widths, columns, dim cells, share/delta cells, bar budget, Total row)
- [x] A-007 R7: `Column.BarAfter`, `Cell.Leader` and the encoder's bar area / leader wrap / empty footer behave as specified
- [x] A-008 R8: The four `HistoryOptions` hooks behave as specified and default to today's output
- [x] A-009 R9: `render.FixedHalfUp` and `JSRound` exist, `csv.Cost` delegates, tests pass
- [x] A-010 R10: `json.Leaderboard` key order, `null` delta, `[]` empty, `--top` truncation; `lbh --json` users in `Repo.Users()` order with `others` last
- [x] A-011 R11: `csv.Leaderboard` header/rows/Total per R11; `csvShare` drops trailing zeros
- [x] A-012 R12: `markdown.Leaderboard` per R12; `markdown.TotalHistory` takes the title and `lbh --md` prints `Leaderboard History (…)`
- [x] A-013 R13: Both run paths compose the pipeline as specified; `foldColumns` appends `others` last and is a no-op when `Top ≥ len(series)`
- [x] A-014 R14: `config.LastSync` exported; `Deps.LastSync` nil-safe; the edge closure wired
- [x] A-015 R15: The 28 matrix groups exist; the harness shows every `lb` case green, no regression, only B6's `sync` cases red

### Behavioral Correctness

- [x] A-016 R7: Every pre-existing `render/ansi`, `json`, `csv`, `markdown` golden and every `view` test passes unchanged
- [x] A-017 R1: No notice line precedes the gate message (a `lbh --by-machine` in single mode prints only the error)
- [x] A-018 R2: `tu --top 3` on a snapshot prints the `--top` warning and the empty table, exit 0

### Scenario Coverage

- [x] A-019 R5: A `Run` test pins the per-day input-order collapse and the ascending window fold with an association-sensitive triple
- [x] A-020 R13: A `Run` test pins the `lbh` tool-major left fold against the collapse-then-roll-up alternative
- [x] A-021 R15: e2e asserts the multi-home January window bytes for `lb` (`harness-user ◂` `$1.70` `56.7%` `new`; `other-user` `$1.30` `43.3%`; Total `$3.00`) and `m lbh` (ranked columns, bold leader, `$3.00` row total)
- [x] A-022 R15: e2e asserts the `--by-machine -t` three-way tie order and the `--top 1` collapsed line

### Edge Cases & Error Handling

- [x] A-023 R6: Zero rows render title, `  No data`, blank, dim footer; `Top ≥ len(rows)` collapses nothing; a single row has no Total
- [x] A-024 R8: An all-zero pivot row highlights its first (post-reorder) column as `BoldWhite(Dim(…))`
- [x] A-025 R4: `--since` after today (length < 1) yields a nil previous window and every row `new`
- [x] A-026 R11: An empty leaderboard CSV prints the header alone, with `machine` when `--by-machine` was passed

### Code Quality

- [x] A-027 Pattern consistency: new code follows the `view`/`render`/`command` helper style (small pure functions, doc comments naming the TS oracle), no ANSI or I/O below `render/ansi` and `cmd/tu`
- [x] A-028 No unnecessary duplication: reuses `metricCell`, `metricColumnWidth`, `fmtMetric`, `barBudget`, `ComputeScale`, `query.GroupBy`/`Collapse`/`Window`/`Relabel`/`SortByDate`, `render.FormatCost`/`FormatInt`, `csv.Cost`; one `GroupBy` per window, no per-dimension aggregation loop
- [x] A-029 Readability: no god functions — `view.Leaderboard`, `runLeaderboard`, `runLeaderboardHistory` decomposed into concern-named helpers like the pivot
- [x] A-030 Minimum pathways: one ANSI encoder, one repo record path (`gatherAllUsers`), one rounding helper for every `toFixed` twin
- [x] A-031 Error paths: no new `os.Exit`, no stderr writes below `cmd/tu`; `gofmt -l` and `go vet` clean (`just go-lint`)

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`
- Environment: run tests with `env -u TU_METRICS_REPO -u NO_COLOR …`; the shell exports both and they tilt config tests and manual probes. `node_modules`, `dist/tu.mjs`, `bin/tu` and `bin/harness/*` are already built in this worktree.

## Deletion Candidates

None — this change adds new functionality without making existing code redundant. (`csv.Cost`'s big.Rat body was *moved* into `render.FixedHalfUp`, which `csv.Cost` now delegates to — no dead copy remains; `markdown.alignRow` stays in use by the snapshot/history encoders beside the new `alignRowOf`.)

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | `LeaderboardRow.Delta` is a `*float64` (nil = `new`) rather than a sentinel or a `(value, ok)` pair | Mirrors the TS `undefined`; JSON `null` and CSV empty fall out of a nil check | S:65 R:90 A:85 D:75 |
| 2 | Confident | `FixedHalfUp`/`JSRound` live in `render/format.go` beside `FormatCost`/`FormatInt`, and `csv.Cost` becomes a thin wrapper | `render` is the shared formatting package every encoder and `view` already import; `csv` importing `render` keeps the dependency direction | S:60 R:85 A:85 D:75 |
| 3 | Confident | The window computation and ranking live in `command/leaderboard.go` (not `query`) | They compose `query` primitives with request flags and the clock — `command`'s job; `query` stays record-only | S:60 R:80 A:80 D:70 |
| 4 | Confident | `markdown.TotalHistory` gains a positional `title` parameter and the two existing callers pass the current text | One extra argument at two call sites beats a second function; `view.PeriodLabel` still owns the parenthetical | S:60 R:90 A:85 D:75 |
| 5 | Confident | The `lb-tty` matrix case reuses the existing `io: tty` axis (`script` is already required by the toolkit `help` tty case) | The bar is the one leaderboard surface a pipe never shows; the axis exists and CI already satisfies its preflight | S:55 R:85 A:80 D:70 |

5 assumptions (0 certain, 5 confident, 0 tentative).
