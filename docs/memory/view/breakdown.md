---
type: memory
description: The --by-machine / -u all breakdown column set — view.Breakdown over machine/user slices, letter-coded columns with one shared data-sized width, the legend line — plus the history summary footer (avg / this month / peak / p95 term).
---

# Breakdown and Footer

**Domain**: view

## Overview

`view.Breakdown` (`internal/view/breakdown.go`) is the `--by-machine` (or `-u all`, by user) column set appended to the [snapshot](/view/snapshot.md) and single-tool [history](/view/history.md) tables, and `footerText` (`internal/view/footer.go`) builds the history tables' dim summary line.

## Requirements

### Requirement: Breakdown carries full Totals
`view.Breakdown{Noun string; Rows map[string][]Slice}` (`internal/view/breakdown.go`) MUST key slices by row key — the tool display name on the snapshot, the ISO label on the single-tool history — with slices in first-seen order; `Noun` is `"Machines"` or `"Users"`. `Slice{Name string; fact.Totals}` carries full Totals, so the ANSI table picks the display metric via `metricValue` while JSON/CSV/Markdown read `TotalCost` through the nil-safe `Slices(key)` (first-seen order) and `CostOf(key, name)` (0 when absent) accessors — one structure, no second build. `Names()` is the sorted union of every slice name across all rows (byte order), nil on a nil breakdown or one with no slices. (pmsd)

### Requirement: Letter-coded columns
`machineColumns` MUST append one right-aligned column per name after the metric column, titled by `letter(i) = string(rune('A' + i))` — A…Z and beyond, no cap. All machine columns share one data-sized width: `machineWidth = metricColumnWidth(every cell value ∪ the per-name sums, m)` floored at `metricFloor` (9). Cells are `metricCell(sliceValue, m)` per name in name order — dim on exact zero, 0 when the row has no slice — in the DISPLAYED metric; the Total row carries the per-name sums via `machineTotalCells` (never dim). The names union spans every row (a hidden snapshot tool can still carry slices); the width pre-pass and sums cover the rows each table defines (snapshot: visible rows only; history: every entry). (svlv, pmsd)

### Requirement: The legend line
`Table.Note` MUST carry `"{Noun}: A = name, B = name, …"` (`Breakdown.note`), rendered dim after a blank line following the Footer; a nil breakdown or one with no names leaves `Note` empty and the output byte-identical to the no-breakdown render. Machine columns come out of the single-tool history's bar budget — `machineColsWidth = len(names) × (machineWidth + 3)` as the `barBudget` reserve — and are dropped in the compact tables. (svlv)

### Requirement: Footer stats
`footerText(labels, values, p, scale, now, m)` (`internal/view/footer.go`) MUST build: `avg {fmtMetric(sum/len(values))}{/day|/week|/month by period}` (`periodUnitSuffix`); daily only, `this month {fmtMetric(sum of rows whose label has the query.CurrentLabel(Monthly, now) prefix)}` — omitted when no row matches; `peak {fmtMetric(max)} ({label})` where the max is the FIRST strict maximum starting from 0 — an all-zero window prints `peak $0.00` with no parentheses; two-zone scale only, `┊ = {fmtMetric(scale.P95)} (p95)`. Parts join with ` · `. The footer appears only when the table has a Total row (more than one entry/label); the encoder wraps the whole text dim and appends the legend swatches after the dim span. (oojd)

#### Scenario: All-zero window
- **GIVEN** a daily history window where every row value is 0
- **WHEN** `footerText` runs under a single-zone scale
- **THEN** the peak part reads `peak $0.00` with no label parentheses, and no `┊ = … (p95)` term appears

## Design Decisions

### One breakdown structure carrying Totals
**Decision**: Slices carry `fact.Totals`; the ANSI table picks the display metric via `metricValue`, and the machine formats read `TotalCost` through the `Slices`/`CostOf` accessors.
**Why**: One structure with the consumer choosing the field removes a second aggregation build per render and cannot drift between the table and the machine formats.
**Rejected**: Building a display-metric map and a cost map per render — a second aggregation over the same records that can drift.
*Introduced by*: 260916-pmsd-machine-columns

### All machine columns share one width
**Decision**: One `machineWidth` serves every machine column rather than per-column sizing.
**Why**: The letter-coded A/B/C columns read as a uniform block; one width keeps the `len(names) × (width + 3)` bar-budget arithmetic simple.
**Rejected**: Per-column sizing — saves at most a character or two of bar width for extra bookkeeping.
*Introduced by*: 260828-7x4i-cost-column-autosize-dim-zeros

### Machine columns follow the displayed metric; machine formats keep cost
**Decision**: ANSI machine cells use `metricValue(t, m)` — token machine cells beside a $ Cost column under `-t --by-machine` — while JSON/CSV/Markdown stay cost-denominated via `CostOf`.
**Why**: A cost block inside a token table is the inconsistency the metric seam exists to remove, while the machine formats are machine contracts that stay cost-denominated.
**Rejected**: Cost-only machine columns (a token table with dollar columns).
*Introduced by*: 260828-018g-tokens-table-mode-t-flag
