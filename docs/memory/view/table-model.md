---
type: memory
description: The render-agnostic table model — view.Table/Row/Cell/Column types, the CompactTable narrow-terminal model, the Metric cost-vs-tokens seam, data-sized metric columns with a 9-rune floor, dimmed exact-zero cells, and the watch compact threshold.
---

# Table Model

**Domain**: view

## Overview

`src/go/internal/view` shapes a [query](/query/aggregation.md) result into a render-agnostic `Table` or `CompactTable` — columns, rows, cells, bars and legends with no ANSI and no I/O; the encoders ([render/ansi](/render/ansi.md), [render/json](/render/json.md), [render/csv-and-markdown](/render/csv-and-markdown.md)) consume the model. The package imports only `fact`, `query`, `render` and the standard library.

## Requirements

### Requirement: The Table model carries no ANSI
`view.Table{Title, Columns, Rows, Empty, Footer, Legend, Note, Scale, DeltaSpaced, DeltaInCell}` MUST describe layout only: `Column{Title, Width, Align, BarAfter}`, `Row{Kind, Cells, Bar, Delta}` with `RowKind` ∈ `Header`/`Divider`/`Data`/`Total`/`Separator` (Separator is the dim month-boundary divider), and `Cell{Text, Dim, Style, Leader, Delta}` with `LabelStyle` ∈ `Plain`/`Current`/`Weekend` used only by a Data row's first (Date) cell. No escape sequence, `os`, `fmt.Print*`, io writer or ccusage import exists anywhere in the package (`internal/view/table.go`). `Empty` is the no-data line — `"  No usage"` on the snapshot, `"  No data"` on the history tables and the leaderboard — and `Rows` is nil when `Empty != ""`. `Note` is a trailing dim line (the `--by-machine` legend) rendered after `Footer`.

### Requirement: Metric selects the displayed unit
`view.Metric` is `Cost` or `Tokens` (`internal/view/metric.go`). `metricValue(t, m)` MUST pick `TotalCost` or `float64(TotalTokens)`; `fmtMetric(v, m)` MUST render `render.FormatCost(v)` under Cost and `render.FormatInt(int64(math.Round(v)))` under Tokens; `metricHeader(m)` is `"Cost"` or `"Tokens"`. Cells, bars and footer stats render in the selected metric.

### Requirement: Data-sized metric columns
Every data-sized right-aligned metric column MUST be sized by `metricColumnWidth(values, m)` = `max(metricFloor, longest fmtMetric over every value the column will hold, including its Total-row value)`; `metricFloor` is 9 in `internal/view/metric.go`. Machine columns share one data-sized width — see [breakdown](/view/breakdown.md). (7x4i)

### Requirement: Dimmed exact zeros
`metricCell(v, m)` MUST set `Cell.Dim` iff the value is exactly 0 — a sub-cent value that formats as `$0.00` is NOT dimmed; the test is `v == 0`, never the formatted string. Header and Total cells never go through `metricCell`. Padding and color are the encoder's (pad first, then Dim), so the visible width is unchanged. (7x4i)

#### Scenario: Sub-cent spend is not dimmed
- **GIVEN** a pivot cell whose value is $0.004 and one whose value is exactly 0
- **WHEN** `metricCell` builds both cells
- **THEN** the first carries `Text: "$0.00", Dim: false` and the second `Dim: true`

### Requirement: The compact table model
`view.CompactTable{Title, Rows []CompactRow, Total *CompactRow, Empty}` (`internal/view/compact.go`) is the narrow-terminal model built by `CompactSnapshot`, `CompactHistory` and `CompactTotalHistory`: two space-joined columns, no header row, no gutters, no bars, no machine columns and no legend. The layout constants are `CompactNameWidth = 14`, `CompactValueWidth = 12` and `CompactDivWidth = CompactNameWidth + 1 + CompactValueWidth` (27). `Total` MUST be nil unless more than one row is visible; the snapshot's Total sums the metric over ALL input rows, the histories' over the visible (windowed) entries or labels.

### Requirement: The compact threshold lives in watch
Watch mode MUST select the compact model when the terminal is narrower than `watch.CompactThreshold` (60 columns, `internal/watch/compositor.go`); one-shot output always renders the full table regardless of width. Leaderboards have no compact form — below the threshold they render the full table with no stats grid. See [watch/compositor](/watch/compositor.md).

## Design Decisions

### View emits a pure model; encoders own the bytes
**Decision**: `view` returns `Table`/`CompactTable` values carrying raw glyphs and geometry; `render/ansi` colors, pads and slices.
**Why**: The strip-ANSI-equals-no-color invariant becomes structural, and the width budget stays in a pure, golden-testable layer.
**Rejected**: Computing glyphs and colors inside the encoder — duplicates the scale math and demotes parity to a test assertion instead of a construction.
*Introduced by*: 260916-9ax5-history-and-periods

### Zero-cell dimming tests exact zero
**Decision**: `metricCell` dims a metric data cell only when its value is exactly 0.
**Why**: Dimming targets absent data in either unit, not tiny values; an exact-zero test is unambiguous and needs no epsilon.
**Rejected**: Dimming anything that formats as the unit's zero string — hides a real sub-cent spend.
*Introduced by*: 260828-7x4i-cost-column-autosize-dim-zeros

### Metric columns are data-sized with a fixed floor
**Decision**: `metricColumnWidth` sizes each right-aligned metric column from its data, floored at 9, computed from the pre-collected row data before the bar budget.
**Why**: Output whose cells fit 9 chars is byte-identical to a fixed 9-wide column; columns grow only when needed and never overflow at any magnitude, so decimal points and bar starts sit at one column index on every row including the Total.
**Rejected**: A larger fixed constant (e.g. 11) — steals bar characters from every small render and breaks again past `$9,999.99`.
*Introduced by*: 260828-7x4i-cost-column-autosize-dim-zeros
