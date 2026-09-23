---
type: memory
description: The leaderboard models — command.rankLeaderboard window math and ranking over user or user/machine keys, view.Leaderboard's ranked table with Share/Δ/mid-row bar, the --top collapse line and lbh "others" fold, the ◂ pinned-user marker, and the staleness footer.
---

# Leaderboard

**Domain**: view

## Overview

`command.rankLeaderboard` (`internal/command/leaderboard.go`) ranks users — or user/machine pairs under `--by-machine` — over a current window with a delta against the previous window, and `view.Leaderboard` (`internal/view/leaderboard.go`) shapes the ranked rows into the lb `Table`; lbh reuses the history pivot through `HistoryOptions` hooks. [command/run-and-result](/command/run-and-result.md) wires the run; encoders live under [render/ansi](/render/ansi.md) and [render/json](/render/json.md).

## Requirements

### Requirement: Windows
`leaderboardWindows(period, since, until, now)` (`internal/command/leaderboard.go`) MUST resolve the current and previous ranking windows: an explicit `--since`/`--until` REPLACES the period window (label `S → U` / `S →` / `→ U`); its previous window is the equal-length range ending the day before `--since` (label `"prev"`), nil without `--since` or when the length is under one day. Otherwise daily windows are today vs yesterday, weekly the Sunday-anchored current week vs the previous week, monthly the calendar month vs the previous month (labelled with the English 3-letter month of its last day, `shortMonths`). Day arithmetic (`addDays`, `diffDays`) parses labels as UTC midnight, so it is timezone-independent.

### Requirement: Ranking, share and delta
`rankLeaderboard(raw, byMachine, cur, prev, metric)` (`internal/command/leaderboard.go`) MUST: collapse per key per day (`query.Collapse` over User[, Machine], Date in record input order, then `query.SortByDate`); window and `query.GroupBy` per window; drop keys with `TotalTokens == 0 && TotalCost == 0`; sort descending by the metric value (`leaderboardMetricValue`), ties by key ascending byte order; fold `grand` over the ranked rows from 0 with `Share = value/grand` (0 when grand is 0); set `Delta = (value − prevValue)/prevValue` when the previous-window value is nonzero, else nil (rendered `new`, JSON null, CSV empty). The key is the user, or `"user/machine"` under `--by-machine` (`leaderboardGroupKey`).

### Requirement: The lb table
`view.Leaderboard(rows, LeaderboardOptions)` (`internal/view/leaderboard.go`) MUST render: title `Leaderboard ({period}) · {WindowLabel} · by {cost|tokens}` — no `📊`, the period word only, never the cap hint; columns `#` Right, `User` Left, `Cost` Right with `BarAfter` (the mid-row bar), `Tokens` Right, `Share` Right, `Δ vs {DeltaLabel}` Right; `Empty = "  No data"` on zero rows with no columns; share cells `render.FixedHalfUp(share×100, 1) + "%"`; delta cells `fmtDeltaCell` — `"new"` when nil, else `{sign}{pct}%` with `pct = render.JSRound(delta×100)` and `+` for pct ≥ 0; Cost/Tokens cells dim on exact zero; a solid bar per Data row scaled over the VISIBLE rows' metric values through `ComputeScale` (the two-zone p95 rule applies); a Divider + Total row (blank rank/share/delta cells) when `len(rows) ≥ 2`, over the FULL row set. Widths are rune-counted over the VISIBLE rows, with the Cost/Tokens widths also sized by the grand total folded over ALL rows. (4xwg, 2gbb)

### Requirement: The --top collapse line
With `0 < Top < len(rows)` the visible rows MUST be `rows[:Top]` and the rest MUST collapse into one dim Data row whose name cell reads `… +{k} others` with blank cells and no bar; collapsed rows still count toward the Total row and every share denominator, and the collapsed label counts toward the User column width. `Top ≥ len(rows)` collapses nothing. (4xwg)

#### Scenario: --top 3 of 5
- **GIVEN** five ranked rows and `--top 3`
- **WHEN** `Leaderboard` builds the table
- **THEN** three ranked Data rows render, a fourth dim row reads `… +2 others` with blank cells and no bar, and the Total row sums all five rows

### Requirement: The pinned-user marker
Rows whose `User` equals `LeaderboardOptions.PinnedUser` (`-u <name>`, else the configured user) MUST carry ` ◂` appended to the name cell — every machine row of the pinned user under `--by-machine` — and the glyph counts toward the User column width. (4xwg)

### Requirement: The staleness footer
`Footer` MUST be `synced {LastSync} · tu sync to refresh` when `LastSync` is set and not `"never"`, else `never synced · tu sync to refresh` — always present, including the empty state; dim at encode. (4xwg)

### Requirement: The watch delta rides the metric cell
Under watch the arrow comes from `Prev[user]` or `Prev[user/machine]`, valued in the display metric, riding the Cost cell under cost and the Tokens cell under tokens, appended AFTER the padded text (`Table.DeltaInCell = DeltaAfterPad`) with the exact-zero Dim wrap around the composite; the bar budget reserves one column when `Prev != nil`. (4pze)

### Requirement: lbh is the pivot with user columns
`runLeaderboardHistory` (`internal/command/leaderboard.go`) MUST build one `view.Series` per repo user in repo order — a user with no window records yields an empty series, kept — and render through `view.TotalHistory` with `RankColumns` (columns by descending window total, stable ties), `HighlightLeader` (each Data row's first strict maximum gets `Cell.Leader`, index 0 when all equal), `KeepAllColumns` (no omission — every user is a column), and the title override `📊 Leaderboard History (…)` / `📊 Leaderboard Token History (…)`. `--top <n>` folds series past the top N into one `"others"` series appended LAST (`foldColumns`), its entries re-grouped per label; `n ≥ len` folds nothing. lbh carries `Prev` but no `MaxRows` and has no compact form. (4xwg, 2gbb)

## Design Decisions

### --top folds rows, never their totals
**Decision**: `--top` slices what renders — shares, the Total row and the Result totals are computed over the full ranked set, and the collapsed tail is one dim `… +k others` line whose label counts toward the User width.
**Why**: A ranking that hides rows must not silently change the denominators a reader sees; `--top` is an explicit display control, so collapsed rows still count toward the Total and every share denominator.
**Rejected**: Truncating the ranked set before computing shares (the visible rows would sum past their true share).
*Introduced by*: 260828-4xwg-leaderboard-lb-lbh-display

### lbh reuses the pivot through hooks
**Decision**: The leaderboard history renders through `TotalHistory` via the `Title`/`RankColumns`/`HighlightLeader`/`KeepAllColumns` hooks on `HistoryOptions`; their zero values reproduce the tool pivot's output exactly.
**Why**: Month separators, the cap hint, bars, segments and the footer are inherited; one pathway cannot drift, and the zero values keep the tool pivot byte-identical.
**Rejected**: A dedicated leaderboard-history builder (duplicate pathway with divergence risk); applying the significance omission to lbh (silently hiding a low-spend user from a ranking is wrong — `--top` is the explicit control).
*Introduced by*: 260828-4xwg-leaderboard-lb-lbh-display

### Ranking is collapse-sort-window-group
**Decision**: The ranking composes `GroupBy(Window(SortByDate(Collapse(raw, User[, Machine], Date))), dims…)` per window, folding the grand total over the ranked rows from 0.
**Why**: Reproduces the reference aggregation order exactly — per-day folds in record input order, window sums ascending by label, grand folded over the ranked rows — so the JSON cost/share/delta doubles are byte-stable.
**Rejected**: `GroupBy(Window(raw), dims…)` directly — sums in walk order across days, a different float association that can differ in the last bit.
*Introduced by*: 260916-2gbb-leaderboard-lb-lbh

### The lb table renders through the shared encoder via zero-valued flags
**Decision**: The leaderboard renders through `ansi.Table` via `Column.BarAfter` (the mid-row bar) and `Cell.Leader`, plus a footer on the Empty branch.
**Why**: Every other leaderboard rule (pad-then-Dim, BoldWhite Total cells, BoldCyan headers, `─|─` dividers) already matches the shared encoder; zero-valued flags keep every existing table byte-identical.
**Rejected**: A dedicated leaderboard encoder — a second pathway repeating the same rules.
*Introduced by*: 260916-2gbb-leaderboard-lb-lbh
