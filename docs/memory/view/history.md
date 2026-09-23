---
type: memory
description: The two history table models — view.History (single tool) and view.TotalHistory (the all-tools pivot) over shared HistoryOptions — with the MaxRows watch budget, month separators, weekend dimming and current-period marker, negligible-column omission, and stacked tool bars.
---

# History Tables

**Domain**: view

## Overview

`view.History` builds the single-tool history table and `view.TotalHistory` the all-tools pivot, both from `[]view.Series` and the shared `view.HistoryOptions`; bar geometry lives in [bars-and-deltas](/view/bars-and-deltas.md), the summary footer and breakdown columns in [breakdown](/view/breakdown.md), and the input series come from [query/aggregation](/query/aggregation.md).

## Requirements

### Requirement: Shared history inputs
`view.Entry{Label, fact.Totals}` is one row candidate and `view.Series{Name, Entries []Entry}` one tool's history, entries ascending by label; [command/run-and-result](/command/run-and-result.md) resolves the display name (view knows no registry). `HistoryOptions{Period, Now, Width, CapActive, Metric, Prev, MaxRows, Title, RankColumns, HighlightLeader, KeepAllColumns}` (`internal/view/history.go`) carries the period, the clock (for `query.CurrentLabel` and the footer's month prefix), the terminal width budget (80 when stdout is not a TTY), the implicit-cap hint flag, the metric, the watch `Prev` delta map (keyed `{Name}:{label}` / `total:{label}`; nil one-shot), and the watch row budget — `MaxRows` 0 renders every entry byte-identical to the one-shot table; the watch loop passes `watch.MaxRows` (15, `internal/watch/compositor.go`). The last four fields are the lbh hooks — see [leaderboard](/view/leaderboard.md); their zero values reproduce the tool pivot's output. (4pze) (2gbb)

### Requirement: Headings and the cap hint
`History`'s title MUST be `📊 {Name} ({PeriodLabel})` — metric-independent. `TotalHistory`'s title MUST be `📊 Combined Cost History ({PeriodLabel})`, `📊 Combined Token History (…)` under tokens, or `HistoryOptions.Title` verbatim when set (the lbh override). `PeriodLabel(p, capActive)` MUST append `, last 3 months` inside the parenthetical when the implicit 3-month cap is active; the hint is absent when the cap is inactive. An empty series or label union yields `Empty = "  No data"` (two leading spaces) with no rows. (yuuj)

### Requirement: MaxRows truncates before layout
With `HistoryOptions.MaxRows > 0` (the watch budget — `watch.MaxRows` is 15, `internal/watch/compositor.go`), `History` MUST keep the last MaxRows entries AFTER its empty check, and `TotalHistory` MUST keep the last MaxRows labels of `LabelUnion(series)` BEFORE its empty check. Separators, the bar scale, significance, the Total row, the footer and the legend MUST be computed on the truncated window. `MaxRows` 0 renders every entry, byte-identical to the one-shot table. (4pze)

### Requirement: Single-tool layout
`History`'s columns MUST be `Date` 12-wide left; `Input`, `Output`, `Cache Write`, `Cache Read`, `Total` each 14-wide right; then the metric column (`Cost`/`Tokens`) right, data-sized with floor 9 (`historyDateWidth`/`historyNumWidth`, `metricColumnWidth` in `internal/view/history.go`). The fixed body measures 97 visible characters (`historyBodyWidth = 12 + 5×14 + 5×3`). Bars follow `barBudget(Width, historyBodyWidth, costWidth, machineColsWidth)`: `barWidth = min(Width − 97 − 3 − costWidth − machineColsWidth − 1, maxBarWidth)`, shown when ≥ `minBarArea` (10). Data rows carry FormatInt token cells, the `metricCell`, the label style, the `Prev` delta (`DeltaSpaced = true` — the `" ↑"` form) and a solid bar (`Bar.Segments` nil). Divider + Total + Footer appear only when the window has more than one entry.

### Requirement: Month separators
A `Separator` row MUST precede a Data row whose `label[:7]` month prefix (`monthPrefixOf`) differs from the previous row's — daily period only, never before the first row, on both history tables, computed on the truncated (post-MaxRows) window. (oojd)

### Requirement: Label styles
`labelStyle(label, p, now)` MUST mark a Data row's first cell `Current` when the label equals `query.CurrentLabel(p, now)`, else `Weekend` when the period is daily and the UTC-parsed label is a Saturday or Sunday (`isWeekend` — a malformed label is never a weekend), else `Plain`. The Current marker wins on a weekend today — one cell, one style. (oojd) (kw96)

#### Scenario: Weekend today
- **GIVEN** a daily row whose label falls on a Saturday and equals `query.CurrentLabel(Daily, now)`
- **WHEN** `labelStyle` runs
- **THEN** the cell style is `Current`, never `Weekend` — one cell, one style

### Requirement: The pivot omits negligible columns
`significant` MUST keep a tool column iff its total over the visible labels is ≥ `negligibleAbs(metric)` (`negligibleCostAbs` = 1.0 under cost, `negligibleTokensAbs` = 1,000 under tokens) AND ≥ `negligibleShare` (0.001) × the window grand total over ALL series — boundary values kept; an emptied set falls back to series with any nonzero cell (`nonzero`), then to every series. `rowValue` and the grand total MUST sum over ALL series — an omitted column still counts in the row total, the Total row and the footer. With `KeepAllColumns` every series is a column, all-zero ones included. (q6fx) (7x4i)

### Requirement: The pivot layout and stacked bars
`TotalHistory`'s columns MUST be `Date` 10-wide left (`pivotDateWidth`), one column per visible tool sized `max(len(Name), metricFloor, longest cell, its Total)`, then the row-total column data-sized with floor 9 (`metricColumnWidth` over every row value plus the grand total). `LabelUnion(series)` is the sorted label union (ascending byte order). Bars follow `barBudget(Width, pivotBodyWidth, costWidth, indicatorReserve)` with `indicatorReserve = 1` when `Prev != nil`; each row's `Bar.Segments = Apportion(per-visible-tool values, runes(Main))` stacks the bar over the visible tools, and `DeltaSpaced = false` gives the space-less `$128.13↑` form. Divider + Total + Footer appear only when the window has more than one label; `Legend` carries one `Swatch` per visible tool iff bars are shown and at least 2 tools are visible. (gmcp) (3tah)

### Requirement: Compact histories
`CompactHistory` MUST run the empty check first, then the `MaxRows` window, then emit one row per entry keyed `{Name}:{label}` with the Total summing the visible (windowed) entries. `CompactTotalHistory` MUST window the sorted label union BEFORE the empty check and emit one row per label carrying the sum over ALL series (an omitted tool still counts), keyed `total:{label}`, with the Total the grand over the windowed labels. (4pze)

## Design Decisions

### Current-period marker keys on CurrentLabel uniformly
**Decision**: The marker highlights the row whose label equals `query.CurrentLabel(p, now)` for every period.
**Why**: One code path covers daily, weekly and monthly; `query.CurrentLabel` already encodes the local-time label logic, and reusing it introduces no import cycle.
**Rejected**: A view-local today helper (duplicates existing logic); a Today field on the options (a pathway with no override consumer).
*Introduced by*: 260815-oojd-history-month-anchors-p95-bars

### Weekend annotation dims the date cell only
**Decision**: Saturday/Sunday rows in daily history views dim just the date cell through the same `labelStyle` seam the current-period marker uses.
**Why**: Weekend usage data is not less important — the annotation marks calendar position, explaining the weekly sawtooth at zero width cost; reusing the date-cell style seam adds no code path.
**Rejected**: Whole-row dimming (visually demotes real data); week separator lines (too heavy — cell dimming is the quiet version).
*Introduced by*: 260816-kw96-dim-weekend-dates-history

### Negligible and zero-value pivot columns are omitted
**Decision**: The pivot keeps a column only when it passes the absolute floor ($1.00 / 1,000 tokens) AND the 0.1% share rule, boundary values kept, with the nonzero-then-all fallback chain; row totals, the Total row and the footer still sum over every series.
**Why**: With six registered tools and typically one or two active, dead columns pushed the full row past 80 chars and suppressed the inline bars; the absolute floor handles tiny windows while the share handles org-scale windows.
**Rejected**: Dimming the zero columns (keeps the width problem); share-only (a tiny absolute column survives a small window); always rendering every column (the width problem returns).
*Introduced by*: 260828-7x4i-cost-column-autosize-dim-zeros

### The significance filter keys on the visible window
**Decision**: The filter sums over the labels actually displayed (after MaxRows truncation), not all fetched labels.
**Why**: Matches what the user sees; a tool active only outside the truncated watch window leaves no ghost column, and the watch compositor re-measures each frame so a column appearing mid-watch is a safe layout change.
**Rejected**: Filtering on all fetched labels (ghost all-zero columns in the watch window).
*Introduced by*: 260815-q6fx-pivot-zero-columns-cost-separators

### Space-less delta in the pivot
**Decision**: `TotalHistory` sets `DeltaSpaced = false` so the watch arrow abuts the cell (`$128.13↑`, +1 char) instead of the spaced form, and the bar budget reserves one column (`indicatorReserve`) when `Prev != nil`.
**Why**: The full pivot row's minimum width is 96 chars; a 2-char indicator would wrap rows on 96–97-column terminals and corrupt the watch compositor's line counting — the +1 form keeps the row at 97, and the reserve keeps every bars-band row within the terminal width.
**Rejected**: Fixed-width pivot columns (overflow 80 at five tools).
*Introduced by*: 260703-gmcp
