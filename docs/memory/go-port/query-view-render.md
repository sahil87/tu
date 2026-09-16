---
type: memory
description: The Go port's middle and output layers — query (Period/labels, ThreeMonthFloor, Window/ByTool/ByUser, RollUp, MaxMerge and Collapse over the one GroupBy), view (the ANSI-free Table model, Snapshot, the History/TotalHistory tables with bars/deltas/footer/legend, bar geometry), render (FormatInt/FormatCost ICU rule), render/ansi (Colors, styled cells, bars), render/csv (exact-half-up Cost), render/markdown, render/json (ordered writers, history shapes) — pure packages pinned by golden files
---
# Query, View and Render (Go port)

**Domain**: go-port

## Overview

The Go port's pipeline stages between the input layer ([fact-and-sources](/go-port/fact-and-sources.md)) and the command edge ([command-edge](/go-port/command-edge.md)): `query` filters, merges, rolls up, and groups `[]fact.Record`; `view` shapes a query result into a render-agnostic `Table` model (snapshot plus the two history tables, including bar geometry); `render` and its `ansi`/`csv`/`markdown`/`json` subpackages encode the model into output lines. None of these packages imports I/O, exec, or the ccusage registry; encoders return `[]string`, and nothing below `cmd/tu` writes to a stream.

## Requirements

### Requirement: Period and label arithmetic
`package query` defines `Period` (`Daily`, `Weekly`, `Monthly`) with `String()` returning `daily`/`weekly`/`monthly` — the table heading text. `CurrentLabel(p, now)` computes the period's label in `now.Location()` (local time, mirroring the TS `currentLabel`): daily `2006-01-02`; weekly the ISO date of the current week's Sunday, `now.AddDate(0, 0, -int(now.Weekday()))` (month/year underflow normalizes); monthly `2006-01`. The edge passes `time.Now()`; `time.Local` honors `$TZ` (the harness `tz: alt` axis). `WeekLabel(daily)` maps a daily ISO label to its week's Sunday using UTC date arithmetic on the date-only string (DST-immune); a label that does not parse as `2006-01-02` is returned unchanged and becomes its own bucket. `ThreeMonthFloor(now)` returns the first day of the local month two calendar months back, formatted `2006-01-02` (`time.Date` normalizes the month underflow across the year boundary) — the implicit history cap's floor, so the default window spans three calendar months including the current one; the caller passes `deps.Now()` so the harness tz axis holds.

#### Scenario: Labels for a fixed clock
- **GIVEN** `now` = 2026-09-16 12:00 in `Asia/Kolkata`
- **WHEN** `CurrentLabel` runs for the three periods and `ThreeMonthFloor(now)` runs
- **THEN** the results are `2026-09-16`, `2026-09-13`, `2026-09` and `2026-07-01`; `ThreeMonthFloor(2026-01-15)` is `2025-11-01`; `WeekLabel("2026-01-01")` (a Thursday) is `2025-12-28` and `WeekLabel("garbage")` is `garbage`

### Requirement: Pure filters
`Window(recs, since, until)` keeps records with `since <= Date <= until` by lexicographic comparison on the ISO labels, inclusive on both ends; an empty bound is open on that side. `ByTool(recs, key)` / `ByUser(recs, user)` keep records whose `Tool`/`User` equals the argument. None mutates its input or returns the input slice.

### Requirement: RollUp re-labels and sums
`RollUp(recs, p)` re-labels each record to its period bucket (daily identity; weekly `WeekLabel(Date)`; monthly `Date[:7]`) and sums `Totals` via `Totals.Add` over records sharing `(Date', Tool, User, Machine)`. `TotalTokens` is summed, never recomputed. Output is ascending by `Date` via `sort.SliceStable` (byte order equals the TS `localeCompare` on ISO labels), first-seen order within a label. Daily returns a copy, never the input slice.

### Requirement: History windows daily records before rolling up
The history pipeline composes `RollUp(Window(recs, since, until), period)` — the window applies to the **daily** records first and the roll-up second — so a partial month sums only in-window days and a window that starts mid-week yields a leading partial week labeled by its Sunday (which may precede `since`). The snapshot composes the other way round, `Window(RollUp(recs, p), cur, cur)`, because its window is a single period label; the two orders coexist deliberately.

#### Scenario: Leading partial week
- **GIVEN** daily records 2026-01-05..07 and the window `--since 2026-01-01 --until 2026-01-31` with `Period == Weekly`
- **WHEN** the history path runs
- **THEN** one entry labeled `2026-01-04` (the Sunday preceding `since`) sums the three days

### Requirement: GroupBy is the one group-by
`Dim` is `Date`, `Tool`, `User`, `Machine`. `GroupBy(recs, dims ...Dim) []Group` sums `Totals` per distinct tuple of the requested dims; `Group.Key` carries only the grouped dims (its other string fields empty). Groups are emitted in first-seen input order, so a registry-ordered input yields registry-ordered groups; `GroupBy` never sorts and never mutates its input. Every pivot shares it: the snapshot is `GroupBy(Window(RollUp(recs, p), cur, cur), Tool)`; history builds its per-tool series from one `GroupBy(recs, Tool, Date)` pass over the rolled-up records (`RollUp` already sorted, so each group's entries are ascending); machine columns group on `Tool, Machine`; leaderboards on `User`; the multi-mode daily collapse (`Collapse`) is a wrapper over it. No per-dimension aggregation code exists anywhere.

### Requirement: MaxMerge and Collapse — the merge arithmetic
`MaxMerge(a, b []fact.Record) []fact.Record` is the self-view high-water merge (the TS `maxMergeEntries`): per key `(Date, Tool, User, Machine)` it keeps whichever WHOLE record — from `a` or from `b` — has the greater `TotalCost`; on a tie the record from `a` wins; never field-wise, never summed. Output order is `a`'s records in `a`'s order (replaced in place when `b` wins), then `b`'s unmatched records in `b`'s order; inputs are never mutated or returned. `Collapse(recs, dims ...Dim) []fact.Record` sums `Totals` per distinct tuple of `dims` and returns one record per group carrying only the grouped dims, in first-seen order with additions performed in input order — the TS `mergeEntries`, the daily cross-machine/cross-user sum that precedes the window filter and the period roll-up. `Collapse` is a thin wrapper over the one `GroupBy` (its groups turned back into records) — no second aggregation loop. Both serve the multi-mode composition ([multi-mode](/go-port/multi-mode.md)); `Collapse` runs on the single-mode path too, where it is the identity on unique keys.

#### Scenario: Collapse sums across machines per day
- **GIVEN** records cc `2026-01-06` from machine A (`$0.75`) then machine B (`$0.40`), and codex `2026-01-07` from B (`$0.30`)
- **WHEN** `Collapse(recs, Tool, Date)` runs
- **THEN** it returns cc `2026-01-06` (`$1.15`, User/Machine empty) then codex `2026-01-07` (`$0.30`), and on records already unique per key the output equals the input's Totals one-for-one

### Requirement: The view.Table model carries no ANSI
`package view` defines `Align` (`Left`/`Right`), `RowKind` (`Header`/`Divider`/`Data`/`Total`/`Separator` — Separator is the dim month-boundary divider), `Column{Title, Width, Align}`, `LabelStyle` (`Plain`/`Current`/`Weekend`, used only by a Data row's first Date cell), `Cell{Text, Dim, Style}`, `Delta` (`DeltaNone`/`DeltaUp`/`DeltaDown` — the watch-mode seam; `command` passes a nil `Prev` map until B7 fills it), `Bar{Main, Overflow string; Segments []int}` (one row's bar reduced to raw block glyphs; `Segments` apportions `Main` over visible tools for the pivot's stacked bars, nil = solid fill), `Scale{TwoZone, Max, P95, Width, MainZone, OverflowZone}` (bar geometry shared by every row; `Width` 0 suppresses bars), `Swatch{Name, Palette}` (legend entry; palette slot 0 green, 1 magenta, 2 blue, 3 cyan, ≥4 uncolored), `Row{Kind, Cells, Bar, Delta}`, and `Table{Title, Columns, Rows, Empty, Footer, Legend, Scale, DeltaSpaced}`. `Cell.Dim` marks exact-zero cell styling (pivot columns) and is false everywhere in the snapshot; `DeltaSpaced` selects the delta form (`" ↑"` on the single-tool history, `"↑"` abutting the cell on the pivot). The snapshot sets `Style` Plain, no `Bar`, `Scale.Width` 0. The package imports only `fact`, `query`, `render`, and stdlib.

### Requirement: Snapshot builds the cross-tool table
`ToolTotals{Name, Label string; fact.Totals}` is one snapshot row candidate: `Name` is the display name resolved by `command` from the registry (view knows no registry); `Label` is the current label when the tool had a matching record (consumed by render/json). `Snapshot(rows, p)` reproduces the TS `renderTotal` rules:

- `Title` is `📊 Combined Usage ({p})` — also for a single-source snapshot, never the tool name.
- Columns are `Tool` (12, left) then `Tokens`, `Input`, `Output`, `Cache`, `Cost` (12 each, right) — fixed, never data-sized; a wider value overflows its cell.
- One `Data` row per input row with `TotalTokens > 0`, in input order: `Name`, `FormatInt(TotalTokens)`, `FormatInt(InputTokens)`, `FormatInt(OutputTokens)`, `FormatInt(CacheCreationTokens+CacheReadTokens)`, `FormatCost(TotalCost)`.
- `Empty = "  No usage"` (two leading spaces) with no header/divider rows when every input row has `TotalTokens == 0`.
- A `Divider` + `Total` row appears only when more than one row is visible; the Total sums every input row, visible or not — the TS accumulates the grand totals before checking visibility, so a tool with cost but zero tokens is hidden yet counted.
- Token mode (`-t`, `--metric tokens`) changes nothing in the one-shot table (the delta indicator is watch-only).

### Requirement: Shared history inputs and helpers
`Entry{Label string; fact.Totals}` is one history row candidate; `Series{Name string; Entries []Entry}` is one tool's history with a display name (resolved by `command`) and entries ascending by label. `Metric` is `Cost` or `Tokens`. `HistoryOptions{Period, Now, Width, CapActive, Metric, Prev}` carries the period, the clock (for `CurrentLabel` and the footer's "this month" prefix), the terminal width budget (80 when stdout is not a TTY), the implicit-cap hint flag, the metric, and the watch delta map (keyed `{Name}:{label}` / `total:{label}`; nil until B7). `PeriodLabel(p, capActive)` is `"{p}, last 3 months"` when the cap is active. Helpers: `fmtMetric(v, m)` = `render.FormatCost(v)` under cost, `render.FormatInt(int64(round(v)))` under tokens; `metricValue(t, m)` = `TotalCost` or `float64(TotalTokens)`; `metricColumnWidth(values, m)` = `max(9, longest fmtMetric)`; `metricCell(v, m)` = `Cell{Text: fmtMetric(v, m), Dim: v == 0}` (exact zero only — a sub-cent value that formats as `$0.00` is not dimmed); `labelStyle(label, p, now)` = `Current` when the label equals `query.CurrentLabel(p, now)`, else `Weekend` when `p == Daily` and the UTC-parsed date is Saturday/Sunday (the marker wins on a weekend today; a malformed label is never a weekend), else `Plain`.

### Requirement: History builds the single-tool table
`History(s Series, o HistoryOptions) Table` reproduces the TS `renderHistory` (layouts §3):

- `Title` `📊 {Name} ({PeriodLabel})` — metric-independent (only the pivot's title changes under tokens).
- No entries → `Empty = "  No data"`, no rows.
- Columns: `Date` (12, left); `Input`, `Output`, `Cache Write`, `Cache Read`, `Total` (14, right); last column `Cost`/`Tokens` (right), data-sized `metricColumnWidth` over every row's metric value plus their sum, floor 9. The fixed body measures 97 visible chars (`historyBodyWidth = 12 + 5×14 + 5×3`).
- Bar budget through the shared `barBudget(Width, historyBodyWidth, costWidth, 0)`: `barWidth = min(Width − 97 − 3 − costWidth − 1, 30)`; bars show when `barWidth ≥ 10`, else `Scale.Width = 0` and no row carries a `Bar`.
- Data rows in input order: `[label, FormatInt(Input), FormatInt(Output), FormatInt(CacheCreation), FormatInt(CacheRead), FormatInt(TotalTokens), metricCell]`; `Cells[0].Style` per `labelStyle`; `Delta` from `Prev["{Name}:{label}"]`; solid bars (`Segments` nil). A `Separator` row precedes any Data row whose `label[:7]` differs from the previous row's — daily period only, never before the first row.
- When `len(entries) > 1`: a `Divider`, a `Total` row `["Total", the five FormatInt sums, fmtMetric(sum)]`, and `Footer` per the footer requirement. A one-row window has none of the three.
- `DeltaSpaced = true`.

### Requirement: TotalHistory builds the cross-tool pivot
`TotalHistory(series []Series, o HistoryOptions) Table` reproduces the TS `renderTotalHistory` (layouts §4):

- `Title` `📊 Combined Cost History ({PeriodLabel})`, or `📊 Combined Token History (…)` under tokens.
- Labels = `LabelUnion(series)`: the sorted union of every series' labels (ascending byte order). None → `Empty = "  No data"`.
- `valueMap`: tool → label → `metricValue`; missing = 0.
- Visible tools (`significant`): keep a series iff its total over the labels is `≥ negligibleAbs` ($1.00 under cost, 1,000 tokens under tokens) **and** `≥ 0.001 × grand`, where `grand` sums **every** series; boundary values are kept. If nothing survives, fall back to `nonzero` (series with any nonzero cell); if that is empty too, every series. Visible order = input (registry) order.
- Per label, `rowValue` sums **all** series (omitted tools still count in the row, the Total, and the footer); `values[i]` covers visible tools; `toolSums` accumulates over visible tools.
- Widths: Date 10; per visible tool `max(len(Name), 9, longest fmtMetric among its cells, fmtMetric(toolSum))`; last column `metricColumnWidth(rowValues + grandTotal)`, floor 9.
- Bar budget through the shared `barBudget(Width, pivotBodyWidth, costWidth, indicatorReserve)`: `barWidth = min(Width − tableWidth − 3 − costWidth − 1 − indicatorReserve, 30)` with `indicatorReserve = 1` when `Prev != nil`; bars show when `barWidth ≥ 10`. At 80 columns the six-tool placeholder pivot needs 84 + 3 + 9 + 1 = 97, so no bars.
- Rows: header `["Date", visible names…, "Cost" or "Tokens"]`; Data rows `[label, metricCell per visible tool…, metricCell(rowValue)]` with `Cells[0].Style`, `Delta` from `Prev["total:{label}"]`, and `Bar.Segments = Apportion(values, runes(Main))`. `Separator` rows as in the single-tool table (daily only).
- When `len(labels) > 1`: `Divider`, `Total` `["Total", fmtMetric(toolSums)…, fmtMetric(grandTotal)]`, and `Footer`. `Legend` = one `Swatch{Name, i}` per visible tool iff bars are shown and ≥ 2 tools are visible (the encoder additionally requires color).
- `DeltaSpaced = false` (the space-less `$128.13↑` form — the pivot's width contract).
- The `lbh` hooks (column reorder, leader highlight, title override) are B5's; the pivot keeps the visible-tool list and row values as ordinary slices so B5 can reorder without reshaping.

### Requirement: Footer text
`footerText(labels, values, p, scale, now, m)` reproduces the TS `renderHistoryFooter` minus the legend: `avg {fmtMetric(sum/len)}{/day|/week|/month by period}`; daily only, `this month {fmtMetric(sum of rows whose label has the CurrentLabel(Monthly, now) prefix)}` — omitted when no row matches; `peak {fmtMetric(max)} ({label})` where the max is the **first strict maximum** (`v > peak` starting from `peak = 0`) — an all-zero window prints `peak $0.00` with no parentheses; two-zone only, `┊ = {fmtMetric(p95)} (p95)`. Parts join with ` · ` (U+00B7 with spaces). The encoder wraps the whole text in `Dim` and appends the legend after it (the legend swatches carry their own resets, which is why the legend is not part of the dim span).

### Requirement: Bar primitives and scale
`bar.go` lifts the TS `renderBar`/`percentile`/`computeBarScale`/`apportionSegments` verbatim:

- `Glyphs(value, max, width)` renders raw block glyphs: `""` when `value == 0` or `max == 0`; `scaled = value/max × width`, `full = floor(scaled)`, `eighths = round((scaled − full) × 8)` (half up, matching JS `Math.round` for non-negatives); `eighths == 8` carries to `"█" × (full+1)`; else `"█" × full + eighths[eighths]`; an empty result becomes `"▏"` (the floor). The eighths table is U+258F..U+2589 (`▏▎▍▌▋▊▉`), `"█"` is U+2588, the two-zone rule is `"┊"` U+250A.
- `Percentile(sortedAsc, p)`: linear interpolation over a sorted-ascending sample, `idx = p/100 × (n−1)`.
- `ComputeScale(values, barWidth)`: `Max` over all values; `P95 = Percentile(nonzero, 95)`; two-zone iff `max > 1.5 × p95`, with `OverflowZone = max(4, round(barWidth/4))` and `MainZone = barWidth − OverflowZone − 1`; otherwise single-zone.
- `Apportion(shares, total)` distributes `total` glyphs over shares by largest remainder — floor each quota, hand the remainder to the largest fractional parts, ties to the earlier index; a zero share gets zero; the result sums exactly to `total`.
- Per-row bars (`rowBar`): single-zone `Main = Glyphs(v, Max, Width)`; two-zone `Main = Glyphs(min(v, P95), P95, MainZone)` and `Overflow = Glyphs(v − P95, Max − P95, OverflowZone)` only when `v > P95` — a row at exactly p95 ends at the rule.

### Requirement: FormatInt and FormatCost reproduce en-US toLocaleString
`render.FormatInt(int64)` groups thousands with commas (`24400` → `24,400`; negative values group the magnitude behind the sign). `render.FormatCost(float64)` returns `$` + the value with exactly two decimals and grouping, reproducing `toLocaleString("en-US", {minimumFractionDigits: 2, maximumFractionDigits: 2})`. The rounding runs on the **shortest round-trip decimal representation** of the double (`strconv.FormatFloat(x, 'f', -1, 64)` — Go's and V8's shortest-digit algorithms agree), rounded to two fraction digits half away from zero with carry into the integer part, then grouped. It is NOT `strconv.FormatFloat(x, 'f', 2, 64)`, which rounds the exact binary value half-even. Negative zero renders `$0.00` (costs are non-negative sums).

Verified divergences (node v24 vs Go, 2026-09-16): `1.005 → $1.01` (Go `'f',2`: `1.00`); `0.125 → $0.13` (Go `0.12`); `0.015 → $0.02` (Go `0.01`); `2.675 → $2.68` (Go `2.67`); `999999.995 → $1,000,000.00` (Go `999999.99`); `4.936068800000001 → $4.94`; `1234567.891 → $1,234,567.89`.

### Requirement: ansi primitives with Colors as a value
`render/ansi` defines `Colors{Enabled bool}`, computed once at the command edge as `!--no-color && NO_COLOR == ""` and passed down; no global color state exists. The methods `Bold`, `Dim`, `Green`, `Red`, `Cyan`, `Yellow`, `Magenta`, `Blue`, `BoldWhite`, `BoldCyan`, `BrightGreen`, `DimGreen` wrap with the layouts.md Color Reference codes (`\x1b[1m`, `\x1b[2m`, `\x1b[32m`, `\x1b[31m`, `\x1b[36m`, `\x1b[33m`, `\x1b[35m`, `\x1b[34m`, `\x1b[1;37m`, `\x1b[1;36m`, `\x1b[92m`, `\x1b[2;32m`) and close with `\x1b[0m`, returning the input unchanged when `!Enabled`. `palette(slot)` maps a legend/segment slot to `Green`/`Magenta`/`Blue`/`Cyan` (slot ≥ 4 uncolored). Color is emitted whether or not stdout is a TTY (no isatty probing); `--no-color` and a non-empty `NO_COLOR` produce byte-identical output. `StripANSI` removes the SGR pattern `\x1b\[[0-9;]*m`; `PadLeft`/`PadRight` pad by rune count (snapshot text is ASCII; the 📊 title is never padded). No width probing, no `os`, no `NO_COLOR` read — `Colors` and `Width` arrive as values.

### Requirement: ansi.Table emits the table line layout
`ansi.Table(t view.Table, c Colors) []string` returns: `""`, `BoldWhite(title)`, `""`; then, if `Empty != ""`, the empty text and a trailing `""`; else the Header row (each cell padded per column, then BoldCyan-wrapped **individually**, joined by unstyled ` | `), Divider and Separator rows as `Dim` of `─`×width per column joined by `─|─` plus `barDiv` (`"─" + "─"×Scale.Width` when `Scale.Width > 0`, appended to header and Total dividers too — the TS `divStr + costDiv + barDiv`), Data rows, the Total row as padded cells each BoldWhite-wrapped individually, then Footer/Legend, then `""`. Per-cell encoding on Data rows: a first cell with `Style` is padded first, then wrapped — `BoldWhite` for `Current`, `Dim` for `Weekend`; a `Dim` cell is padded first, then `Dim`-wrapped; Header/Total cells are never dimmed. A `Delta` follows the last cell — `Green("↑")`/`Red("↓")`, prefixed with `" "` when `DeltaSpaced`. The bar follows the delta: single-zone `" " + fill(Main)` only when `Main != ""`; two-zone **always** `" " + fill(Main) + spaces(MainZone − runes(Main)) + Dim("┊") + Yellow(Overflow) + spaces(OverflowZone − runes(Overflow))`, including a zero row, with **no empty-string guard** — `Green("")`/`Yellow("")` emit `ESC[32mESC[0m`/`ESC[33mESC[0m` exactly as the TS `wrap()` does. `fill` is solid `Green(Main)` when `Segments == nil`, else contiguous rune slices of `Main` colored by palette slot (zero-count segments skipped) — stripping ANSI from the result yields `Main` unchanged. The footer line is `Dim(Footer)` plus, when `Legend != nil` and color is enabled, `Dim(" · ")` + swatches joined by `" "`, each `palette(i)("█") + " " + Dim(Name)`; with color disabled the legend is omitted entirely. The snapshot divider is 87 visible characters.

### Requirement: csv encodes the machine contract
`render/csv` provides `Snapshot(rows)`, `History(s)`, and `TotalHistory(series)` returning RFC 4180 lines. Headers: `tool,tokens,input,output,cache,cost`; `date,input,output,cache_write,cache_read,total,cost`; `date,{Name1},…,total`. Snapshot emits one row per tool with `TotalTokens > 0` (`cache` = cacheCreation + cacheRead) and a `Total` row — summing **every** input row, hidden included — only when more than one row is visible. The two history kinds NEVER carry a Total row; `History` emits one row per entry in input order; `TotalHistory` keeps **every** series column (no omission — the positional contract) over the sorted label union, with `Cost(rowTotal)` last. Integers are raw `strconv.FormatInt`; every field passes `quote` (wrap in `"` and double inner `"` when it contains `,`, `"`, `\n` or `\r`). An empty window prints the header line alone. `Cost(x)` is the TS `csvCost` = JS `toFixed(2)`: the **exact binary value** rounded half-up to two decimals via `big.Rat.SetFloat64` (scale by 100, floor, round up when the remainder ≥ 1/2), formatted `int.frac2`, with `-0`/`0` → `0.00` — a third rounding rule, distinct from `render.FormatCost` and from `strconv.FormatFloat(x, 'f', 2, 64)` (half-even). Node-verified (node v24, 2026-09-16): `1.005 → 1.00` (ICU `1.01`), `0.125 → 0.13`, `0.375 → 0.38`, `2.675 → 2.67` (ICU `2.68`), `0.015 → 0.01` (ICU `0.02`), `1.045 → 1.04` (ICU `1.05`), `8.345 → 8.35`, `0.005 → 0.01`, `0.045 → 0.04` (ICU `0.05`), `999999.995 → 999999.99` (ICU `1,000,000.00`), `1234567.891 → 1234567.89`, `0.5 → 0.50`, `0 → 0.00`.

### Requirement: markdown encodes the human contract
`render/markdown` provides `Snapshot(rows, period)`, `History(s, period, capActive)`, and `TotalHistory(series, period, capActive)` returning GFM lines: `## {title}`, blank, the header row, the alignment row (`:---` for the text column, `---:` for numerics), data rows, a `**Total**` row with bold cells when more than one data row is visible (snapshot: more than one visible tool), then a blank line (the caller's per-line `Fprintln` reproduces the TS trailing blank). Titles: `Combined Usage ({period})`; `{Name} ({period}[, last 3 months])`; `Combined Cost History ({period}[, last 3 months])` — the pivot title and cells are **always cost**, ignoring `--metric` (the TS `emitMarkdownTotalHistory` reads `totalCost`). No `📊`. Numbers via `render.FormatInt`, costs via `render.FormatCost`. Snapshot rows filter `TotalTokens > 0`. TotalHistory columns drop exact-zero tools over the cost map across **all** labels (the `nonzero` rule, not the pivot table's significance rule); when none is nonzero all six stay. An empty window emits heading, blank, header, alignment row, blank — no data rows.

### Requirement: json.Snapshot writes the ordered snapshot object
`render/json.Snapshot(rows []view.ToolTotals) []string` produces the lines of `JSON.stringify(obj, null, 2)`: an object keyed by display name in input order; per row `"label"` first when `Label != ""`, then the six pinned totals keys (`totalCost`, `inputTokens`, `outputTokens`, `cacheCreationTokens`, `cacheReadTokens`, `totalTokens`); two-space indent. The writer is hand-ordered — Go maps are unordered and struct marshalling cannot emit the conditional `label` key first — and uses `encoding/json` only for scalars: floats follow the ES6 rules `encoding/json` implements (shortest repr, `1e+21`/`1e-7` exponent thresholds) with `-0` normalized to `0`; strings encode without HTML escaping. The trailing newline of `console.log` is the caller's per-line `Fprintln`.

### Requirement: json history shapes
`render/json.History(s)` emits a bare array of entry objects (`[]` inline when empty); `render/json.TotalHistory(series)` emits an object keyed by display name in input (registry) order, each series an array of entries, `[]` inline for an empty series — every registry tool appears even when empty. Entry keys in order: `label`, `totalCost`, `inputTokens`, `outputTokens`, `cacheCreationTokens`, `cacheReadTokens`, `totalTokens` (`label` is always present on history entries — the snapshot's conditional-label rule does not apply). Layout follows `JSON.stringify(v, null, 2)`: a populated nested array puts each object on its own indented block; scalars go through the same helpers as the snapshot. Costs are the raw summed doubles — the Go sum order (per-tool ascending by date within `RollUp`) matches the TS accumulation order, so values like `1.5` and `4.936068800000001` agree.

### Requirement: Golden-file encoder tests
Every encoder is pinned by golden files under each package's `testdata/` directory, regenerated with a package-level `-update` flag (`go test ./internal/render/ansi/ -update`); every colored ansi golden is asserted equal to its no-color twin under `StripANSI`, and every bar row's stripped bar equals `Main` + padding + `┊` + `Overflow` + padding. Inventory: `ansi` — `snapshot_color`, `snapshot_nocolor`, `snapshot_single`, `snapshot_empty`, `history_80col_color`, `history_80col_nocolor`, `history_wide_bars` (Width 140, single-zone), `history_two_zone` (outlier window: rule, yellow overflow, the empty-wrap escapes on a zero row, the p95 footer term), `history_single_row`, `history_empty`, `history_tokens`, `history_separators_weekend_today`, `pivot_80col_color`, `pivot_80col_nocolor`, `pivot_wide_stacked` (Width 120, segments + legend), `pivot_wide_nocolor` (legend omitted), `pivot_omission_dim_zero`, `pivot_single_row`, `pivot_empty`; `csv` — `snapshot_populated`/`_empty`, `history_populated`/`_empty`, `total_history_populated`/`_empty`; `markdown` — `snapshot_populated`/`_empty`, `history_populated`, `history_with_total`, `history_empty_cap`, `total_history_populated`, `total_history_empty_cap`, `total_history_omission`; `json` — `all_tools_labeled`, `all_tools_unlabeled`, `all_zero`, `single_tool`, `history_populated`, `history_empty`, `total_history_mixed`, `total_history_all_empty`. Bars are unobservable in the differential harness (piped output is 80 columns and neither history table fits a bar there), so injected-width goldens are the only CI check for bars; the placeholder corpus likewise never triggers column omission, dim zero cells, month separators, the current-period marker, weekend dimming, token mode on history, the two-zone scale, or the stacked legend — the local-capture harness run is the real-bytes check for those (see [differential-harness](/harness/differential-harness.md)).

## Design Decisions

### Colors is a value, not a global
**Decision**: `ansi.Colors{Enabled}` is computed once in `cmd/tu` and passed to the renderers.
**Why**: The TS `setNoColor()` module global is the output-channel leak the plan removes; a value keeps `render` pure and testable.
**Rejected**: A package-level `SetNoColor` mirroring the TS.
*Introduced by*: 260916-3am6-query-view-render-snapshot

### Number formatting lives in the parent render package
**Decision**: `render.FormatInt`/`FormatCost` sit in `internal/render`, shared by `render/ansi`, `render/markdown`, and `render/json`.
**Why**: One implementation of the ICU rounding rule serves the encoders that share it.
**Rejected**: Formatting inside `view` (the rule belongs to the encoder family); reusing `FormatCost` for CSV — CSV's cost rule is `toFixed(2)`, which rounds the exact binary value half-up (`1.005 → 1.00`, `0.125 → 0.13`), a DIFFERENT rule from FormatCost's ICU rule, so `render/csv` implements its own (`csv.Cost`).
*Introduced by*: 260916-3am6-query-view-render-snapshot

### view receives display names, not registry keys
**Decision**: `command` resolves `fact.Lookup(key).Name` and hands `view.ToolTotals{Name}` / `view.Series{Name}` in registry order.
**Why**: Keeps `query`/`view`/`render` free of the registry and the exec adapters (the layering the plan's G1 gate checks).
**Rejected**: Letting `view` import the registry or resolve names itself (pulls the registry dependency into the pure layer).
*Introduced by*: 260916-3am6-query-view-render-snapshot

### One bar budget and concern-per-helper structure for the history tables
**Decision**: `barBudget(width, bodyWidth, costWidth, reserve)` is the single bar-width formula both history tables call — the two differ only in `bodyWidth` (the pivot's computed table width vs the fixed `historyBodyWidth`) and `reserve` (the pivot's `Prev != nil` indicator vs 0). Each table body is a short composition over concern-named helpers: `pivotValues`/`pivotData`/`pivotWidths`/`pivotRows`/`pivotTotals` for `TotalHistory`, `historyValues`/`historyColumns`/`historyRows` for `History`, with parallel `*Values`/`*Rows` verbs.
**Why**: B4's machine columns add a column source to the pivot and B5's `lbh` hooks add a row source — both extend the tables by adding a helper, not lines; one bar formula cannot drift into two.
**Rejected**: Keeping each table as one body with its `min(...)` bar expression inline — two copies of one rule and no named joints for the later rows.
*Introduced by*: 260916-m9of-g1-rework-1

### GroupBy emits groups in first-seen order
**Decision**: No sorting inside `GroupBy`; callers order their input.
**Why**: Registry-ordered input yields registry-ordered columns with no extra pass; history callers sort by label before grouping.
**Rejected**: Sorting by key inside `GroupBy` (would reorder tool columns alphabetically).
*Introduced by*: 260916-3am6-query-view-render-snapshot

### View emits raw glyphs plus geometry; ansi only colors and pads
**Decision**: `view.Bar` carries the raw block-glyph strings (`Main`, `Overflow`) and per-tool `Segments`; `render/ansi` slices and colors them and adds padding and the `┊` rule.
**Why**: The strip-ANSI-equals-no-color invariant becomes structural — the colored bar is the raw bar with escapes inserted at slice boundaries — and the width budget stays in the pure layer the plan assigns bar scales to.
**Rejected**: Computing glyphs inside `render/ansi` from numeric values (duplicates the scale math in the encoder and makes the no-color twin a test assertion instead of a construction).
*Introduced by*: 260916-9ax5-history-and-periods

### csv.Cost is a third rounding rule on big.Rat
**Decision**: `render/csv.Cost` rounds the exact binary value half-up to two decimals via `math/big`.
**Why**: JS `toFixed(2)` is defined on the exact value with ties to the larger n; `FormatFloat('f', 2)` is half-even on the exact value and `render.FormatCost` is ICU's shortest-repr half-away-from-zero — both diverge on node-verified inputs.
**Rejected**: Reusing `render.FormatCost` and stripping the `$` (wrong on `1.005`, `0.015`, `1.045`); `FormatFloat('f', 2)` (wrong on `0.125`, `0.375`).
*Introduced by*: 260916-9ax5-history-and-periods
