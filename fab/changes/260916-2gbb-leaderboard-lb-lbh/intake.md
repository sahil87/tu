# Intake: Leaderboard (Go port row B5)

**Change**: 260916-2gbb-leaderboard-lb-lbh
**Created**: 2026-09-17

## Origin

> Context: fab/plans/sahil/26-09-15-go-port.md, row B5. Read the Decisions and Target architecture sections before writing the intake. Do not change any external surface listed under Goal. Build lb/lbh, user ranking, previous-period deltas, leaderboard JSON/CSV/Markdown. Harness gate: leaderboard cases.

One-shot `/fab-new` invocation from the Go-port operator queue (Phase 2, after B4 merged as PR #91). The row text in the plan reads: *"`lb`/`lbh`, user ranking, previous-period deltas, leaderboard JSON/CSV/Markdown. Harness: leaderboard cases."* — size M, depends on B3 (merged as PR #90). The plan's status column for B1–B4 and B8 still says "not started"; `git log -- src/go` shows V1, V2, B1, B2, G1-rework, B8, B3 and B4 all merged — the row map in `docs/memory/go-port/command-edge.md` is the current truth and names `lb`/`lbh`/`--top` as B5's.

Key readings that shaped this intake (2026-09-17, worktree at `93e7add`):

- **What is red**: `command.inScope` rejects the `Leaderboard`/`LeaderboardHistory` displays and `Flags.Top`, so every `lb*`/`lbh*` harness case prints the placeholder (exit 1) — 44 of the 56 remaining red cases (the other 12 are B6's `sync` groups). The grammar is already parsed: `Parse` sets `Display` for `lb`/`lbh`, validates `--top` (`Error: --top requires a positive integer`, exit 2, incl. `--top 0`), and `Normalize` already carries the `lbh --by-machine` warn-and-clear (B4, emits no bytes yet).
- **The seams are in place**: `Repo.Users()`/`Repo.Read(user, tool)` stamp `User`/`Machine` from the metrics walk (users ascending; year, machine, file ascending within a user); `query.GroupBy` accepts `User` and `Machine`; `view.TotalHistory` keeps the visible-column list and row values as ordinary slices so B5 can reorder without reshaping (`query-view-render` memory, line "the `lbh` hooks … are B5's"); `config.lastSync` already formats `.last-sync` exactly as the TS `formatLastSync` (unexported); `render/csv.Cost` already implements the exact-binary half-up rounding `toFixed` uses, at two decimals.
- **The TS reference** is `src/node/core/leaderboard.ts` (`currentWindow`, `previousWindow`, `sumByKey`, `buildLeaderboard`), the `fetchLeaderboardRows`/`fetchLeaderboardHistory`/`foldLeaderboardColumns`/`sumLeaderboardToolMaps`/`readAllUsersByUserMachine` block in `src/node/core/cli.ts` (lines 747–1027) plus the `main()` guard order (lines 1914–1994), and `renderLeaderboard`/`leaderboardRowsToJson`/`emitCsvLeaderboard`/`emitMarkdownLeaderboard` and the four `lbh` `FormatOptions` hooks in `src/node/tui/formatter.ts`. Contract text: `docs/specs/usage.md` § Displays, § Flags (`--top`, `-u`, `--since`, `--full`, `--by-machine`), § Exit Codes, § Data flow (Leaderboard, Leaderboard history), § Leaderboard Table, § Leaderboard History Table, § JSON/CSV/Markdown tables, DC-07/08/11/13/14/16; `docs/specs/layouts.md` § 5, § 6, § 12, § 15, § 16, § 17 (the `lb --by-machine` rows), § 18, § 19.

## Why

**Problem.** The Go binary answers every snapshot and history request but none of the leaderboard grammar: `tu lb`, `tu m lb`, `tu cc m lb`, `tu lbh`, `tu w lbh`, any `--top`, and the `-u` pin semantics all return `ErrUnported` and print `tu: not implemented (Go port in progress)`, exit 1. The leaderboard is the one display an org reads together (who spent what this month), and it is the last multi-mode display type in Phase 2 — B7 (watch) depends on B5 (`lb -w`, `lbh -w` are supported combinations).

**Consequence of not doing it.** 44 harness cases stay red for a single reason, B7 cannot start (its `-w` matrix covers every display type), and a Goal-listed surface (the CLI grammar, the table/JSON/CSV/Markdown output, the exit-1 `lb requires multi mode` guard) is unreproduced at cutover.

**Why this shape.** The plan's target architecture says the leaderboards pivot over `Repo.Users()` with the **one** `GroupBy` (§ Target architecture, `query`: "`GroupBy(dims...)` — one group-by for tool, machine and user pivots"; `multi-mode` memory § Seams: "The leaderboards (B5) pivot over `Repo.Users()` with `GroupBy(User)`"). The TS builds the leaderboard from three ad-hoc shapes (`Map<user, UsageEntry[]>` per tool, the `sumLeaderboardToolMaps` fold, the `LeaderboardRow` tuple) and a dedicated 140-line ANSI renderer; the Go port reads the same repo records `gather` already returns, ranks them with one `GroupBy` per window, hands `view` a render-agnostic row set, and lets the existing table encoder render it through two small, zero-valued model extensions (a mid-row bar column, a per-cell leader highlight) instead of a second encoder. `lbh` is `view.TotalHistory` with three option hooks — the same reuse the TS chose (its Design Decision "`lbh` is the tool pivot with users as columns") — so the pivot's month separators, current-period marker, weekend dimming, stacked bars, legend, p95 scale and footer are inherited, not re-implemented.

**Constraint from the Goal.** Every external surface stays byte-identical to the shipped TS: the heading without `📊` (DC-07), the `others` column sorted by its own total (DC-08), the CSV `share`/`delta` rounding (DC-11), the `→ {until}` heading and all-`new` deltas of an `--until`-only window (DC-13), the guard message naming `lb` for `lbh` (DC-14), alphabetical CSV/Markdown user columns beside the ranked ANSI columns (DC-16), the raw JSON doubles and their float association (DC-10). The `[DECIDE]` markers are gate G0's; this row reproduces, never resolves.

## What Changes

### 1. `command.Run` — the multi-mode gate and the leaderboard record path

**Gate (before `Normalize`, in TS `main()` order — the `-u` single-mode guard exempts the leaderboards and the `lb` guard fires next, before every later guard):**

```go
// ErrLeaderboardMode is the TS exit-1 guard: the leaderboard is an all-users
// view of the metrics repo; single mode has no repo to rank. The message
// names lb even for lbh (DC-14). cmd/tu prints err.Error(), exit 1 — no
// notices, no fetch, no lines (the TS exits before the later guards run).
var ErrLeaderboardMode = errors.New("Error: lb requires multi mode — run tu init-metrics <repo-url> to set up a metrics repo")
```

`Run`: `if (req.Display == Leaderboard || req.Display == LeaderboardHistory) && cfg.Mode == config.Single { return Result{}, ErrLeaderboardMode }` — evaluated on the **post-guard** config, so a demoted multi config (missing metrics dir + fresh `.clone-failed` marker) lands on this message after the guard's own stderr lines, exactly as the TS `checkMetricsDirGuard` → `main()` sequence. `cmd/tu` already prints any non-`ErrUnported` error's text with `ExitOperational`; no edge change is needed for the gate.

**Record path.** Both displays are repo-only in the TS (`fetchToolMergedWithMachines(…, ALL_USERS)` → `readAllUsersByUser`; `readAllUsersByUserMachine` under `--by-machine`): no live fetch, no ccusage call, no source warnings, no writes. Reuse `gatherAllUsers(deps.Repo, tools)` — users ascending, tools in registry order per user, walk order within a tool — the same un-collapsed records the `-u all` path returns. The harness compares the call multiset informationally; the node side records zero ccusage calls on every `lb*`/`lbh*` case and the Go side must too. `Result.Warnings` is nil on these paths.

### 2. `command.Normalize` — the leaderboard exemptions and the `--top` guard

The TS `main()` guard sequence, with the leaderboard displays threaded in (current Go behaviour in brackets where it changes):

0. `-u` set ∧ single mode ∧ display ∉ {`lb`, `lbh`} → `userNotice`, cleared. [today: every display — the leaderboards must be exempt so the exit-1 gate in §1 fires without a `-u` line.]
0'. `-u all` ∧ display ∈ {`lb`, `lbh`} → `Flags.User = ""` silently (no notice; the leaderboard is inherently all-users). `-u <name>` is kept: it **pins** that user (§4), never filters.
0a. `--by-machine` on the all-tools history → notice, cleared. [unchanged]
0b. `--by-machine` on `lbh` → `byMachineLbhNotice`, cleared. [unchanged — it now emits bytes, because `lbh` renders after it.] `--by-machine` on `lb` is **kept** and keys rows by `user/machine`.
1. `--since`/`--until` set ∧ display ∉ {`h`, `lb`, `lbh`} → `sinceUntilNotice`, both cleared. [today: `!= History` — the leaderboards accept the window; on `lb` it **replaces** the period window (§3).]
2. `--full` ∧ display ∉ {`h`, `lbh`} → `fullNotice` (flag left set). [today: `!= History` — `lb` warns like a snapshot, `lbh` does not.]
2b. **New**: `--top` set ∧ display ∉ {`lb`, `lbh`} → `topNotice = "Warning: --top applies to leaderboard display — ignoring."`, `Flags.Top = 0`. Position: after the `--full` notice, before the cap (TS order). A bare `tu --top 3` therefore warns and renders the snapshot, exit 0 (DC-03).
3. Cap: display ∈ {`h`, `lbh`} ∧ period ≠ monthly ∧ no bound ∧ `!Full` → `Since = ThreeMonthFloor(now)`, `capActive`. [today: `== History` — `lbh` daily/weekly is capped like `h`; `lb` is never capped.]

`guards_test.go` gains rows for each changed step, the `-u`/`lb` exemption, the `--top` notice text and position (`-u other --top 2 --full` on a single-mode snapshot → `-u` line, `--full` line, `--top` line), and the `lbh` cap.

**`inScope`**: admit `Leaderboard` and `LeaderboardHistory`; admit `Top` (Normalize clears it off-display; the leaderboards consume it). `Watch`, `Sync`, `DryRun`, `NoRain`, `SkipBrewUpdate` stay out (B6/B7), so `tu lb -w`, `tu lb --sync` keep the placeholder.

### 3. Leaderboard data — windows, ranking, deltas (pure, in `command` + `query`)

**Windows** (the TS `currentWindow`/`previousWindow`, `src/node/core/leaderboard.ts`), computed from `req.Period`, `Flags.Since`/`Until` and `deps.Now()`:

| Case | Current window (start, end, heading label) | Previous window (start, end, Δ label) |
|------|--------------------------------------------|---------------------------------------|
| daily, no bounds | today, today, today (`query.CurrentLabel(Daily, now)`) | yesterday, yesterday, yesterday (local calendar day) |
| weekly, no bounds | this Sunday (`CurrentLabel(Weekly)`), today, the Sunday | Sunday−7, Sunday−1, label = Sunday−7 |
| monthly, no bounds | `YYYY-MM-01`, today, `CurrentLabel(Monthly)` | first of previous month, last day of previous month (both **local**: `time.Date(y, m-1, 1)` / `time.Date(y, m, 0)`), label = English 3-letter month of that last day (`Jan`…`Dec`) |
| `--since S --until U` | S, U, `S → U` | equal-length range ending the day before S: length = days(S..end)+1 where end = U (or today when U absent); start = S−length, end = S−1; label `prev`; **nil when length < 1** |
| `--since S` only | S, "" (open), `S →` | as above with end = today |
| `--until U` only | "" (open), U, `→ U` | **nil** → every row `new` (DC-13) |

Day arithmetic on the ISO strings is UTC-parsed (`time.Parse("2006-01-02")` + `AddDate`) — timezone-independent, like the TS `addDays`/`diffDays`; the "today"/"this month" anchors are `deps.Now()` in local time (the harness `tz` axis). Bounds are inclusive, an empty bound is open — `query.Window` as-is.

**Ranking** (the TS `sumByKey` + `buildLeaderboard`), one `GroupBy` per window over the same records:

```go
dims := []query.Dim{query.User}
if req.Flags.ByMachine { dims = append(dims, query.Machine) }
daily := query.SortByDate(query.Collapse(raw, append(dims, query.Date)...))   // per key per day, summed in record input order
cur    := query.GroupBy(query.Window(daily, curStart, curEnd), dims...)       // per key over the window, days ascending
prev   := query.GroupBy(query.Window(daily, prevStart, prevEnd), dims...)     // nil window ⇒ no prev map
```

- The per-day collapse in **record input order** (tool registry order, walk order within a tool) reproduces the TS association: `sumLeaderboardToolMaps` merges each tool's flattened per-machine entries into the running per-label sum (`mergeEntries`: first entry copied, then `+=` in order), tools in registry order — the left fold over the same sequence `((cc_m1 + cc_m2) + codex_m1)`. The window sum then folds the per-day values **ascending by label** (`mergeEntries` sorts; `sumByKey` iterates in that order) — hence `SortByDate` before `Window`/`GroupBy`. This matters for the raw JSON `cost`/`share`/`delta` doubles (DC-10).
- Key = `User`, or `User + "/" + Machine` under `--by-machine`. The `LeaderboardRow` keeps them split (`User`, `Machine`) for the JSON/CSV/Markdown `machine` field; the ANSI name cell joins them.
- **Drop** keys whose window totals have `TotalTokens == 0 && TotalCost == 0`. A key with data in the previous window only is dropped too (it is not in `cur`).
- **Sort** descending by `metricValue(totals, metric)` (cost, or tokens under `-t`/`--metric tokens`), ties by key **ascending byte order** (the TS `<`/`>` string compare on ASCII keys). Deterministic — no stability dependence.
- `grand` = left fold of `metricValue` over the **ranked** kept rows starting at 0; `Share = value / grand` (0 when `grand == 0`). Summing in ranked order is byte-visible in the JSON `share`.
- `Delta`: `prevValue = metricValue(prev[key])` (0 when the key is absent from `prev` or `prev` is nil); `Delta = (value − prevValue) / prevValue` when `prevValue != 0`, else **nil** (rendered `new`, JSON `null`, CSV empty).
- `Rank = i + 1` after sorting.

`Result`: `TotalCost` = fold of `TotalCost` over the ranked rows (all rows, not the `--top` slice); `TotalTokens` likewise; `CostByItem[key] = metricValue(totals, metric)` keyed by `user` or `user/machine` (the TS `leaderboardPrevMap` — B7's watch seam). `Notices` from Normalize; `Warnings` nil.

**`--top n` on `lb`**: the ranked list is computed in full; `n` slices what is **rendered** (`n ≥ len(rows)` ⇒ nothing collapsed). Shares, the Total row and `Result` totals cover the full set.

### 4. `view.Leaderboard` — the table model for the ranked rows

New file `internal/view/leaderboard.go`:

```go
// LeaderboardRow is one ranked key: the user (and machine under --by-machine),
// its window totals, its share of the grand total in the display metric, and
// its fractional change vs the previous window (nil = "new").
type LeaderboardRow struct {
    Rank    int
    User    string
    Machine string   // "" unless --by-machine
    fact.Totals
    Share   float64
    Delta   *float64
}

type LeaderboardOptions struct {
    Period      query.Period
    WindowLabel string   // heading: the current window's label (§3)
    DeltaLabel  string   // Δ header: the previous window's label, or "prev"
    Metric      Metric
    PinnedUser  string   // rows whose User equals it carry " ◂"
    Top         int      // 0 = all; else rows past Top collapse into "… +k others"
    Width       int      // terminal budget (80 when piped)
    LastSync    string   // config.LastSync text: "never" or "{relative} ({ISO})"
    Prev        map[string]float64 // watch deltas keyed by user or user/machine; nil until B7
}

func Leaderboard(rows []LeaderboardRow, o LeaderboardOptions) Table
```

Rules (the TS `renderLeaderboard`, byte for byte):

- **Title**: `Leaderboard ({period}) · {WindowLabel} · by {cost|tokens}` — **no `📊`** (DC-07); `{period}` is `Period.String()` (never the cap hint — `lb` is never capped).
- **Footer** (the staleness line, dim): `synced {LastSync} · tu sync to refresh` when `LastSync != "never"`, else `never synced · tu sync to refresh`. Always present, including the empty state.
- **Empty** (`len(rows) == 0`): `Empty = "  No data"`, no columns — the encoder emits title, blank, `  No data`, blank, `Dim(Footer)`, blank (§5 extends the empty branch to render `Footer`).
- **Visible rows** = `rows[:Top]` when `Top > 0 && Top < len(rows)`, else all; `collapsed = len(rows) − len(visible)`; `collapsedLabel = "… +{k} others"` when `k > 0`.
- **Name cell** = key (`User` or `User/Machine`) + `" ◂"` when `row.User == PinnedUser` (every machine row of the pinned user under `--by-machine`).
- **Widths** (over **visible** rows unless stated): `rank = max(1, len(strconv.Itoa(r.Rank)))` (1 up to 9 visible rows, 2 from 10); `name = max(len("User"), runes(collapsedLabel), runes(name cells))` — the collapsed label counts; `cost = metricColumnWidth(visible costs ∪ grandCost, Cost)` where `grandCost` folds over **all** rows; `tokens = metricColumnWidth(visible token counts ∪ grandTokens, Tokens)`; `share = max(len("Share"), share cells)`; `delta = max(runes("Δ vs {DeltaLabel}"), delta cells)`. All rune-counted (`Δ`, `◂`, `…` are single runes — `ansi.PadLeft`/`PadRight` already count runes).
- **Columns**: `#` Right, `User` Left, `Cost` Right **with `BarAfter: true`** (§5), `Tokens` Right, `Share` Right, `Δ vs {DeltaLabel}` Right.
- **Cells**: rank `strconv.Itoa`; name; cost `render.FormatCost(TotalCost)` with `Dim: TotalCost == 0`; tokens `render.FormatInt(TotalTokens)` with `Dim: TotalTokens == 0`; share `FixedHalfUp(share×100, 1) + "%"` (§7 — the `toFixed(1)` rule: `69.0%`, `100.0%`); delta `fmtDeltaCell` = `new` when nil, else `{sign}{pct}%` with `pct = floor(delta×100 + 0.5)` (the JS `Math.round` — half toward +∞, **not** Go's `math.Round`) and `+` for `pct ≥ 0` (`+0%`, `-55%`, `+4757%`).
- **Bars**: `Scale = ComputeScale(values of visible rows in the metric, barWidth)` through the shared `barBudget(Width, bodyWidth, costWidth, reserve)` with `bodyWidth = rank + 3 + name + 3 + tokens + 3 + share + 3 + delta` (every non-metric column and its gutters) and `reserve = 1` when `Prev != nil` — i.e. `barWidth = min(Width − tableWidth − 1 − reserve, 30)`, bars when `≥ 10` (19 at 80 columns for the layouts § 5 data; usually none under `--by-machine`). `Bar{Main}` per data row (solid, `Segments` nil); the two-zone p95 rule applies exactly as on the histories.
- **Collapsed row** (when `k > 0`): a `Data` row with cells `["", {Text: collapsedLabel, Dim: true}, "", "", "", ""]`, `Bar` nil — the encoder's pad-then-Dim yields `dim(label.padEnd(name))` and blank cells become spaces.
- **Total row** when `len(rows) ≥ 2` (the full set — also under `--top`): Divider, then `Total` cells `["", "Total", FormatCost(grandCost), FormatInt(grandTokens), "", ""]` (each padded then BoldWhite-wrapped by the encoder, blanks included — `boldWhite("   ")` in the TS).
- `Legend` nil, `Note` "", `DeltaSpaced` false; the watch indicator (B7) rides the Cost cell under cost and the Tokens cell under tokens, inside the dim wrap — recorded here as the seam, not built.

### 5. `view.Table` + `render/ansi` — two zero-valued model extensions, one encoder branch

```go
// Column gains:
BarAfter bool // the bar renders after this column's cell (the leaderboard's
              // mid-row bar); false everywhere ⇒ the bar trails the last cell
// Cell gains:
Leader bool   // BoldWhite-wrapped after padding and any Dim wrap (the lbh
              // per-row leader); false ⇒ unchanged
```

Encoder (`ansi.Table`/`dataRow`/`joinStyled`/`divider`):

- When a column has `BarAfter` and `Scale.Width > 0`: every non-divider row inserts the **bar area** — exactly `1 + Scale.Width` visible characters — immediately after that column's padded cell and before the next ` | `. Data rows: `" " + Green(Main) + spaces(Width − runes(Main))` single-zone (the TS `renderScaledBar` output padded to `barWidth + 1`; an empty `Main` is `barWidth + 1` spaces), the two-zone form unchanged (already full-width). Header, Total and bar-less Data rows (the collapsed line): `" " + spaces(Width)`, **unstyled** (outside the BoldCyan/BoldWhite cell wraps). Dividers insert `barDiv` (`"─" + "─"×Width`) after that column's dashes instead of at the end. When no column has `BarAfter`, everything is byte-identical to today (the bar trails, single-zone unpadded).
- `Leader` cells: `padded → Dim (if Dim) → BoldWhite (if Leader)` — a zero leader cell is `boldWhite(dim(text))`, the TS double wrap.
- Empty branch: `"", Title, "", Empty, ""` then, when `Footer != ""`, `Dim(Footer), ""`. Existing tables have no footer on empty — unchanged.

Goldens (`render/ansi/testdata`): `leaderboard_color`, `leaderboard_nocolor`, `leaderboard_tokens` (`by tokens`, re-ranked), `leaderboard_top` (collapsed line, widened User column), `leaderboard_by_machine` (`user/machine ◂` rows, no bar at 80), `leaderboard_wide` (Width 120: 30-char bars), `leaderboard_two_zone` (p95 rule mid-row), `leaderboard_empty` (footer after `  No data`), `leaderboard_never_synced`, `leaderboard_single_row` (no Total), `pivot_lbh_ranked` (columns by total, leader cells), `pivot_lbh_top_others` (`others` mid-table, DC-08).

### 6. `view.TotalHistory` — the three `lbh` hooks

`HistoryOptions` gains:

```go
Title           string // "" = "📊 Combined {Cost,Token} History ({PeriodLabel})"; lbh passes "📊 Leaderboard History (…)" / "📊 Leaderboard Token History (…)"
RankColumns     bool   // order visible columns by descending window total in the metric; ties keep first-seen order (stable)
HighlightLeader bool   // each Data row's max cell (strict >, first wins; index 0 when all equal) gets Cell.Leader
KeepAllColumns  bool   // skip the significance/nonzero omission: every series is a column, all-zero ones included
```

- `RankColumns` reorders `visible` (and each row's `values`) **after** `pivotData` computes `toolSums` over the input order — the TS sorts `toolNames` by `toolSums` desc with `a.i − b.i` as tie-break. Widths, header, cells, `Segments`, `Legend` swatches and the Total row follow the reordered `visible`. Row values and `grandTotal` are order-independent sums.
- `KeepAllColumns` makes `visible` every series index: with the seed, `tu m lbh` shows both users; `tu w lbh` on a window where a user has nothing shows that user as a `$0.00` column (dim cells, width `max(len(name), 9)`) — the TS `allToolNames` path.
- `HighlightLeader` marks one cell per Data row; with all-zero values the first (post-reorder) column is the leader — `boldWhite(dim("$0.00"…))`.
- Defaults leave `tu h` byte-identical (goldens unchanged).

### 7. `render` helpers — `FixedHalfUp`

Lift the exact-binary half-up rounding out of `csv.Cost` into `render.FixedHalfUp(x float64, digits int) string` (big.Rat on the exact value, magnitude rounded half-up, sign re-added, zero-padded fraction, `-0` → `0`) — the JS `toFixed(d)` rule for `d ∈ {1, 2}`. `csv.Cost(x)` becomes `FixedHalfUp(x, 2)` (its Node-verified table stays as the test); the share cells use `FixedHalfUp(share×100, 1)`. Not `strconv.FormatFloat(x, 'f', 1, 64)` — half-even on exact ties (`12.25` → `12.2`; `toFixed(1)` gives `12.3`).

`jsRound(x) = math.Floor(x + 0.5)` (half toward +∞; negative zero normalized) is the `Math.round` twin for the delta percent and the CSV fractions.

### 8. `render/json` — the `lb` array; `lbh` reuses `TotalHistory`

`json.Leaderboard(rows []view.LeaderboardRow) []string`: `[]` inline when empty; else `JSON.stringify(v, null, 2)` layout — one object per row with keys in order `rank` (int), `user` (string), `machine` (only under `--by-machine`, i.e. when `Machine != ""` on the row — the TS conditional spread), `cost` (`encodeFloat(TotalCost)`, raw double), `totalTokens` (int64), `share` (`encodeFloat`), `delta` (`encodeFloat` or `null`). `--top` **truncates** the array (the sliced rows). Goldens: `leaderboard`, `leaderboard_by_machine`, `leaderboard_new_delta` (`null`), `leaderboard_empty`.

`lbh --json` = `renderjson.TotalHistory(series)` unchanged: an object keyed by user in **`Repo.Users()` order** (ascending — the TS map insertion order from the sorted `listUsers`, which the spec calls "alphabetical"), every user present (`[]` inline for one with no entries in the window), and under `--top` the kept users in that order followed by `others` **last** (the TS `foldLeaderboardColumns` appends it after the kept keys). Entry objects carry the seven history keys — the day-file key order the metrics writer emits. Golden: `total_history_lbh_top` (`others` last).

### 9. `render/csv` — the `lb` kind; `lbh` reuses `TotalHistory`

`csv.Leaderboard(rows, all []view.LeaderboardRow, byMachine bool) []string`: header `rank,user,cost,total_tokens,share,delta` — `machine` after `user` when `byMachine` (the explicit flag, so an empty result keeps the schema); one row per **sliced** row: `rank`, `user`, [`machine`], `Cost(TotalCost)` (2 decimals), `TotalTokens`, `csvShare(Share)`, `csvShare(*Delta)` or empty when nil, where `csvShare(n) = FormatFloat(jsRound(n×1000)/1000, 'f', -1, 64)` (the TS `String(Math.round(n*1000)/1000)`: `0.69`, `-0.3`, `17.309`, `0` — shortest round-trip, trailing zeros dropped, DC-11); a `Total,,{cost},{tokens},,` row (one more empty field under `byMachine`) when `len(all) > 1` — the **full** set, so `--top 1` on two users still emits it. Header alone when empty. Goldens: `leaderboard`, `leaderboard_by_machine`, `leaderboard_top_total`.

`lbh --csv` = `csv.TotalHistory(series)`: header `date,{users…},total` with users in `Repo.Users()` order (+ `others` last when folded), every user column kept, never a Total row (DC-16 in the alphabetical direction).

### 10. `render/markdown` — the `lb` table; `TotalHistory` takes a title

`markdown.Leaderboard(rows, all []view.LeaderboardRow, period query.Period, deltaLabel string, byMachine bool) []string`: `## Leaderboard ({period})` (period word only — no window, no metric), blank, header `#`, `User`, [`Machine`], `Cost`, `Tokens`, `Share`, `Δ vs {deltaLabel}` with alignment `---:`, `:---`, [`:---`], `---:`, `---:`, `---:`, `---:`; data rows: rank, user, [machine], `render.FormatCost`, `render.FormatInt`, `FixedHalfUp(share×100, 1)%`, `fmtDeltaCell` (`+12%`, `-55%`, `new`); a `**Total**` row (`**Total**`, ``, [``], `**$cost**`, `**tokens**`, ``, ``) when `len(all) > 1`; trailing blank line. Goldens: `leaderboard`, `leaderboard_by_machine`, `leaderboard_top_total`, `leaderboard_empty` (heading + two header lines).

`markdown.TotalHistory(series, period, capActive, title string)` — the heading text becomes a parameter (`"Combined Cost History"` from the existing callers; `"Leaderboard History"` / `"Leaderboard Token History"` from `lbh`, the TS `lbhTitle(metric, …, markdown=true)` without the emoji); the parenthetical stays `view.PeriodLabel(period, capActive)`. The all-`$0.00` column drop and the Total rule are inherited (layouts § 16 shows `| Date | otheruser | sbuser | Cost |`).

### 11. `command` — the two run paths

**`runLeaderboard(req, cfg, raw, notices, deps)`**: windows (§3) → ranking (§3) → `view.LeaderboardRow` slice → by format: `JSON` → `json.Leaderboard(sliced)`; `CSV` → `csv.Leaderboard(sliced, all, req.Flags.ByMachine)`; `Markdown` → `markdown.Leaderboard(sliced, all, req.Period, deltaLabel, req.Flags.ByMachine)`; default → `ansi.Table(view.Leaderboard(all, opts), deps.Colors)` with `opts.Top = req.Flags.Top`, `opts.PinnedUser = req.Flags.User` when set else `cfg.User`, `opts.LastSync = deps.LastSync()`, `opts.Width = deps.Width`.

**`runLeaderboardHistory(req, cfg, raw, notices, capActive, deps)`**: one `view.Series` per user in `Repo.Users()` order:

```go
for _, u := range deps.Repo.Users() {
    recs := query.ByUser(raw, u)                                          // gather order preserved: tools in registry order, walk order within
    groups := query.GroupBy(query.Relabel(query.Window(recs, since, until), req.Period), query.Date)
    // one series per user: one Entry per group, then a stable sort ascending by label
}
```

The value per (user, period label) is the **left fold in record input order** over the windowed, relabelled records — tool-major, walk order within a tool — because the TS `aggregateMachineMap` runs `aggregateForPeriod` on each tool's **flattened, unmerged** machine entries (machine-major within the tool: `(m1_d5 + m1_d6) + m2_d6`) and `sumLeaderboardToolMaps` then adds the tools in registry order: the same single left fold. This is **not** the main history's collapse-then-roll-up association (`multi-mode` memory) and not `RollUp` (which keys on Machine); it is the B4 breakdown's pattern (`GroupBy(Relabel(Window(raw)), …)`). Entries sort ascending by label after grouping (the TS `mergeEntries`/`aggregateMonthly` sort). A user with no records in the window yields an empty series (kept — `KeepAllColumns`).

`--top n`: `foldColumns(series, n, metric)` — total per series = left fold of `metricValue` over its entries (ascending) starting at 0; stable sort descending (`sort.SliceStable`); keep the top `n` names **in their original order**; when anything was folded, append one series named `others` whose entries are `GroupBy(Date)` over the folded series' entries concatenated in original order (per label left fold in that order), sorted by label. `n ≥ len(series)` ⇒ unchanged, no `others`.

By format: `JSON` → `renderjson.TotalHistory(series)`; `CSV` → `csv.TotalHistory(series)`; `Markdown` → `markdown.TotalHistory(series, period, capActive, "Leaderboard History"|"Leaderboard Token History")`; default → `ansi.Table(view.TotalHistory(series, opts), deps.Colors)` with `Title = "📊 Leaderboard History (" + PeriodLabel + ")"` (or `Token`), `RankColumns`, `HighlightLeader`, `KeepAllColumns` true. `Result.TotalCost`/`TotalTokens`/`CostByItem` as `runHistory` computes them (`{user}:{label}`, `total:{label}` — the TS `buildCostMap`).

### 12. `cmd/tu` and `config`

- Export `config.LastSync(stateDir string, now time.Time) string` (rename of the unexported `lastSync`; `Status` keeps calling it). The edge sets `Deps.LastSync: func() string { return config.LastSync(config.StateDir(paths.Home), time.Now()) }` — a closure like `Now`, evaluated only by the leaderboard path so no other command reads the file; `command` stays I/O-free. A nil `LastSync` in tests reads as `"never"`.
- `main.go`'s package comment and the row map sentence list the leaderboards as answered for real.
- `e2e_test.go`: flip `TestRunUnported`'s `leaderboard`/`lbh`/`lb by-machine`/`top`/`multi lb`/`multi lbh`/`multi top` rows into real assertions: single home `lb`, `lbh`, `lb -u other-user` → the exit-1 message on stderr, stdout empty, **no** `-u` line; multi home `lb` (today: heading `Leaderboard (daily) · {today} · by cost`, `  No data`, `never synced · tu sync to refresh`, exit 0), `m lbh` (populated `2026-01` row: `harness-user` `$1.70`, `other-user` `$1.30`, ranked columns, Total row — Width 80 so no bars), `lb --since 2026-01-01 --until 2026-01-31` (rows `1 | harness-user ◂ | $1.70 | … | 56.7% | new`, `2 | other-user | $1.30 | … | 43.3% | new`; Total `$3.00`), the same with `--json` (raw `1.7`, `1.3`, shares as doubles, `"delta": null`), `--csv` (`0.567`/`0.433`, empty delta, `Total,,3.00,…,,`), `--md`, `--by-machine` (`other-user/laptop`, `harness-user/harness-machine ◂`, `harness-user/other-box ◂`), `--top 1` (`… +1 others`, Total still `$3.00`), `-t` (`by tokens`, re-ranked: `harness-user` 97,600 tokens vs `other-user` 48,800), `--by-machine -t` (a three-way tie on 48,800 tokens broken by key name: `harness-user/harness-machine`, `harness-user/other-box`, `other-user/laptop`), `cc lb --since … --until …` (cc only: `$1.40` / `$1.10`), `lb --until 2026-01-31` (heading `· → 2026-01-31 ·`), `lbh --since … --until …` (daily rows with month separator rules), `m lbh --top 1` (`others` column, JSON `others` last), `lbh --by-machine` (the warning line, then the capped `📊 Leaderboard History (daily, last 3 months)` + `  No data`), `--top 3` on a snapshot (the `--top` warning + the empty table), `-u all` on `lb` (no notice), a staged `.last-sync` file (footer `synced 15m ago (2026-…) · tu sync to refresh` with a fixed `Now`).
- `run_test.go`: fake-`Repo` tests for the window table (all six rows of § 3, including the `length < 1` nil), the ranking tie-break, the drop rule, the `share` fold order with an association-sensitive triple, the delta nil cases (absent key, zero prev), `--top` slicing vs full-set totals, the `lbh` fold (`others` placement, `n ≥ users`), and the `-u all`/`-u name` pin semantics.

### 13. Harness matrix — populated leaderboard cases

The 44 red `lb*`/`lbh*` cases go green but exercise only the exit-1 guard (single/legacy/org-without-repo) and the empty state (today ≠ the seed's January) — `m lbh` is the only populated one and it is not in the matrix. Add (additive; the denominator grows from 386):

```json
{ "id": "m-lbh", "args": ["m", "lbh"], "conf": ["single", "multi"] },
{ "id": "m-lbh-json", "args": ["m", "lbh", "--json"], "conf": ["multi"] },
{ "id": "m-lbh-csv", "args": ["m", "lbh", "--csv"], "conf": ["multi"] },
{ "id": "m-lbh-md", "args": ["m", "lbh", "--md"], "conf": ["multi"] },
{ "id": "m-lbh-top-1", "args": ["m", "lbh", "--top", "1"], "conf": ["multi"] },
{ "id": "m-lbh-top-1-json", "args": ["m", "lbh", "--top", "1", "--json"], "conf": ["multi"] },
{ "id": "m-lbh-tokens", "args": ["m", "lbh", "-t"], "conf": ["multi"] },
{ "id": "lbh-full", "args": ["lbh", "--full"], "conf": ["multi"], "tz": ["fixed", "alt"] },
{ "id": "lbh-window", "args": ["lbh", "--since", "2026-01-01", "--until", "2026-01-31"], "conf": ["multi"] },
{ "id": "lbh-by-machine", "args": ["lbh", "--by-machine"], "conf": ["multi"] },
{ "id": "lb-window", "args": ["lb", "--since", "2026-01-01", "--until", "2026-01-31"], "conf": ["single", "multi"], "env": ["default", "nocolor"] },
{ "id": "lb-window-json", "args": ["lb", "--since", "2026-01-01", "--until", "2026-01-31", "--json"], "conf": ["multi"] },
{ "id": "lb-window-csv", "args": ["lb", "--since", "2026-01-01", "--until", "2026-01-31", "--csv"], "conf": ["multi"] },
{ "id": "lb-window-md", "args": ["lb", "--since", "2026-01-01", "--until", "2026-01-31", "--md"], "conf": ["multi"] },
{ "id": "lb-window-by-machine", "args": ["lb", "--since", "2026-01-01", "--until", "2026-01-31", "--by-machine"], "conf": ["multi"] },
{ "id": "lb-window-by-machine-json", "args": ["lb", "--since", "2026-01-01", "--until", "2026-01-31", "--by-machine", "--json"], "conf": ["multi"] },
{ "id": "lb-window-by-machine-csv", "args": ["lb", "--since", "2026-01-01", "--until", "2026-01-31", "--by-machine", "--csv"], "conf": ["multi"] },
{ "id": "lb-window-top-1", "args": ["lb", "--since", "2026-01-01", "--until", "2026-01-31", "--top", "1"], "conf": ["multi"] },
{ "id": "lb-window-top-1-csv", "args": ["lb", "--since", "2026-01-01", "--until", "2026-01-31", "--top", "1", "--csv"], "conf": ["multi"] },
{ "id": "lb-window-tokens", "args": ["lb", "--since", "2026-01-01", "--until", "2026-01-31", "-t"], "conf": ["multi"] },
{ "id": "lb-window-by-machine-tokens", "args": ["lb", "--since", "2026-01-01", "--until", "2026-01-31", "--by-machine", "-t"], "conf": ["multi"] },
{ "id": "lb-window-user-other", "args": ["lb", "--since", "2026-01-01", "--until", "2026-01-31", "-u", "other-user"], "conf": ["multi"] },
{ "id": "cc-lb-window", "args": ["cc", "lb", "--since", "2026-01-01", "--until", "2026-01-31"], "conf": ["multi"] },
{ "id": "lb-until-only", "args": ["lb", "--until", "2026-01-31"], "conf": ["multi"] },
{ "id": "lb-since-only", "args": ["lb", "--since", "2026-01-01"], "conf": ["multi"] },
{ "id": "lb-full", "args": ["lb", "--full"], "conf": ["multi"] },
{ "id": "top-snapshot", "args": ["--top", "3"], "conf": ["single", "multi"] },
{ "id": "lb-tty", "args": ["lb", "--since", "2026-01-01", "--until", "2026-01-31"], "conf": ["multi"], "io": ["tty"] }
```

The seed makes these populated on the multi confs: `harness-user` cc `0.25` + `0.75` + `0.40` and codex `0.30` (`$1.70`, two machines), `other-user` cc `1.10` and gemini `0.20` (`$1.30`); every day-file carries the same `24400` tokens, so `-t` re-ranks by file count (97,600 vs 48,800), `--by-machine -t` exercises the key-name tie-break (three keys at 48,800), and `--top 1` folds one user. The `lb-tty` case gives the bar its one real-terminal check (`script` allocates a pseudo-terminal; the TS reads `process.stdout.columns`). `just go-diff --placeholder` is the gate: every `lb*`/`*-lb*`/`lbh*` case green, no previously green case regresses, the only remaining red are B6's 12 `sync` cases.

### Out of scope (owned elsewhere)

- `--watch` on either leaderboard, the live `Prev` maps, `indicatorReserve` in effect, compact layouts — B7 (the seams — `Prev`, `CostByItem`, `Column.BarAfter`'s reserve — are wired).
- `--sync` / `tu sync` and the metrics writer — B6.
- Any spec edit. Divergences the harness cannot observe with the seed (none identified for this row) would be recorded in memory for G0, not applied to `docs/specs/`.
- Constitution: Principles I, II, V hold (pure `query`/`view`/`render`, the guard message on stderr at the edge, one fact type); Go Transition article unchanged; TypeScript untouched (D4).

## Affected Memory

- `go-port/command-edge`: (modify) `Run`'s `ErrLeaderboardMode` gate (position, post-guard config, DC-14); `Normalize` steps 0/0'/1/2/2b/3 with the leaderboard exemptions and the `--top` notice; `inScope` admits the two displays and `Top`; `runLeaderboard`/`runLeaderboardHistory` (windows, one-`GroupBy`-per-window ranking, the left-fold association, `--top` slice vs fold, `-u` pin/no-op); `Deps.LastSync`; the row map drops B5 (only B6/B7 unported); the e2e inventory and the harness gate status (green count, remaining red = B6's 12).
- `go-port/query-view-render`: (modify) `view.Leaderboard`/`LeaderboardRow`/`LeaderboardOptions` and their column rules; `Column.BarAfter`, `Cell.Leader` and the encoder's mid-row bar area, empty-branch footer and leader wrap; `HistoryOptions.Title`/`RankColumns`/`HighlightLeader`/`KeepAllColumns`; `render.FixedHalfUp`/`jsRound` and `csv.Cost` as its two-digit form; `json.Leaderboard`, `csv.Leaderboard`, `markdown.Leaderboard`, `markdown.TotalHistory`'s title parameter; the golden inventory; the "lbh hooks are B5's" sentence becomes present truth.
- `go-port/multi-mode`: (modify) the leaderboards as the fifth consumer of `gatherAllUsers` (repo-only, no ccusage call, no warnings); § Seams: B5 done, B6 remains; the `lbh` association rule (tool-major left fold, distinct from the main table's collapse-then-roll-up).
- `go-port/config-and-setup`: (modify) `config.LastSync` exported and shared by `Status` and the leaderboard footer.
- `harness/differential-harness`: (modify) the added leaderboard matrix groups, the `io: tty` leaderboard case, and the new case count.

TS-facing memory (`display/formatting`, `cli/data-pipeline`, `sync/multi-machine`) and `docs/specs/` are unchanged — the shipped TypeScript is frozen (D4) and the Goal surfaces are the contract this row reproduces.

## Impact

- **Code** (`src/go/`): `internal/command/{run.go,guards.go,leaderboard.go (new),run_test.go,guards_test.go,leaderboard_test.go (new)}`; `internal/view/{table.go,leaderboard.go (new),leaderboard_test.go (new),pivot.go,history.go,pivot_test.go}`; `internal/render/{format.go,format_test.go}`; `internal/render/ansi/{table.go,table_test.go,testdata/*}`; `internal/render/json/{leaderboard.go (new),*_test.go,testdata/*}`; `internal/render/csv/{csv.go,csv_test.go,testdata/*}`; `internal/render/markdown/{markdown.go,markdown_test.go,testdata/*}`; `internal/config/status.go`; `cmd/tu/{main.go,e2e_test.go}`.
- **Harness**: `harness/matrix.json` (additive cases). Gate: `just go-diff --placeholder` (builds `dist/tu.mjs`, `bin/tu`, `bin/harness/*`; run `npm ci` first in a fresh worktree — see the worktree memory note; `go test ./...` is green at `93e7add`).
- **Not touched**: `src/node/`, `docs/specs/`, the formula, CI. No new dependency. `--help`, `help-dump`, exit codes, `tu.conf` unchanged.
- **Risk**: the JSON float association of `cost`/`share`/`delta` (the seed's two-machine `harness-user` and two-tool users observe it; the `-u all` history cases already pin the same walk order); the `toFixed(1)` share and `Math.round` delta twins (`FixedHalfUp`, `jsRound` — unit-tested against Node-verified tables); the mid-row bar encoder change (only the `lb-tty` case and the goldens see bars); the `lbh` monthly association differing from `mh`'s (pinned by an association-sensitive `Run` test).
- **Size**: M, as planned — roughly 600 lines of Go plus goldens and tests; one PR.

## Open Questions

- None blocking.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Scope is `lb`/`lbh` in all four formats, `--top` on both plus its off-display guard, the `-u` pin/no-op semantics, the exit-1 multi-mode gate, `lb --by-machine`; watch and sync stay B7/B6 | Plan row B5 and the command-edge row map name exactly this split; the Goal freezes the surfaces | S:85 R:70 A:90 D:90 |
| 2 | Certain | Both displays read the repo only through `gatherAllUsers` — no live fetch, no ccusage call, no writes, no warnings | The TS `fetchLeaderboardRows`/`fetchLeaderboardHistory` pass `ALL_USERS`, which is the repo-only branch; the harness call log records zero ccusage calls on the node side | S:85 R:85 A:95 D:90 |
| 3 | Confident | The `lb` ranking is one `GroupBy` per window over `SortByDate(Collapse(raw, User[, Machine], Date))`, sharing the grand total in ranked order | Reproduces the TS `mergeEntries` per-label left fold, the label-sorted `sumByKey` fold and the `grand` fold over the sorted `kept`; honours the one-group-by rule (G1 item 2) | S:70 R:70 A:80 D:65 |
| 4 | Confident | The `lbh` value per user and period is a left fold in record input order over the windowed, relabelled records (`GroupBy(Relabel(Window(recs_u)), Date)`), not the main table's collapse-then-roll-up | The TS `aggregateMachineMap` aggregates each tool's unmerged, machine-major entries, then `sumLeaderboardToolMaps` adds tools in registry order — one left fold; the JSON doubles are byte surfaces (DC-10) | S:65 R:70 A:75 D:60 |
| 5 | Confident | The leaderboard renders through the shared `ansi.Table` via two zero-valued model extensions (`Column.BarAfter`, `Cell.Leader`) and an empty-branch footer, not a dedicated encoder | Fewer pathways (code-quality); every other rule (pad-then-Dim, BoldWhite Total cells, BoldCyan headers, dividers) already matches the TS renderer; existing goldens stay byte-identical | S:65 R:75 A:80 D:60 |
| 6 | Certain | `lbh` is `view.TotalHistory` with `Title`, `RankColumns`, `HighlightLeader`, `KeepAllColumns` options | The TS made the same reuse decision (formatting memory DD) and the Go pivot was shaped for it (query-view-render memory: "so B5 can reorder without reshaping") | S:80 R:80 A:90 D:80 |
| 7 | Certain | The exit-1 gate lives in `Run` as `ErrLeaderboardMode` evaluated on the post-guard config before `Normalize`; `cmd/tu` prints it through the existing non-`ErrUnported` error path | Keeps `cmd/tu` the only writer without a new edge branch; the TS order (`-u` guard exempts the leaderboards, the `lb` guard fires next) means no notice precedes it | S:70 R:85 A:85 D:75 |
| 8 | Confident | `config.LastSync` is exported and reaches `command` as `Deps.LastSync func() string`, evaluated only on the `lb` path | `command` must stay I/O-free (target architecture); a closure mirrors `Deps.Now`; `Status` already has the exact TS formatting | S:60 R:85 A:85 D:70 |
| 9 | Certain | `toFixed(1)` and `Math.round` get exact twins: `render.FixedHalfUp` (generalising `csv.Cost`) and `jsRound = floor(x + 0.5)` | Go's `FormatFloat` is half-even on exact ties and `math.Round` is half-away-from-zero; both differ from JS on representable ties and negative halves, which are byte-visible in Share, Δ and CSV fractions | S:60 R:90 A:85 D:80 |
| 10 | Confident | `lbh` JSON/CSV user order is `Repo.Users()` ascending with `others` last; the ANSI columns rank by total (DC-16, DC-08 reproduced, not resolved) | The TS map insertion order comes from the sorted `listUsers` and `foldLeaderboardColumns` appends `others` after the kept keys; the harness byte-diff is the bar (D6) | S:65 R:80 A:85 D:75 |
| 11 | Confident | Add 28 populated leaderboard matrix groups (windowed `lb` in every format, `--by-machine`, `--top`, `-t`, `-u`, `--until`-only, `m lbh` family, `lbh --full`, the `--top` snapshot guard, one `io: tty` case) | The 44 existing cases only hit the gate and the empty state with the placeholder corpus; D6 makes the harness the release gate; additive, denominator grows (B4 precedent) | S:55 R:85 A:70 D:60 |
| 12 | Certain | Change type is `feat`, pinned explicitly | Sibling rows B3/B4/B8 shipped as `feat`; the quoted Goal's "redesigned" would re-infer `refactor` | S:80 R:95 A:95 D:95 |
| 13 | Certain | The slug is `leaderboard-lb-lbh` (the plan's row name alone fails the 2-word slug rule); the change ID `2gbb` is used for every fab command | `leaderboard-lb-lbh` is also a substring of the TS change `260828-4xwg-leaderboard-lb-lbh-display`, so name-based resolution would be ambiguous | S:70 R:90 A:90 D:85 |
| 14 | Confident | `KeepAllColumns` renders every `Repo.Users()` profile as an `lbh` column, all-zero ones included, and a zero-only row highlights its first column | The TS `omitNegligibleColumns: false` path uses `allToolNames` (every map key) and `highlightRowLeader` picks index 0 under strict `>` | S:60 R:85 A:80 D:70 |
| 15 | Certain | Memory updates touch the four go-port files and the harness memory only; TS-facing memory and specs are untouched | D4 freezes `src/node/`; the Goal fixes the surfaces; hydrate owns the go-port domain | S:70 R:90 A:85 D:85 |

15 assumptions (8 certain, 7 confident, 0 tentative, 0 unresolved).
