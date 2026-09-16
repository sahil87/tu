---
type: memory
description: The Go port's middle and output layers — internal/query (Period/CurrentLabel/WeekLabel, Window/ByTool/ByUser filters, RollUp, the one GroupBy with first-seen order), internal/view (the ANSI-free Table model and the cross-tool Snapshot rules), internal/render (FormatInt/FormatCost with the ICU rounding rule), render/ansi (Colors value, palette, table line layout), and render/json (ordered snapshot writer, conditional label, ES6 floats) — pure packages pinned by golden files
---
# Query, View and Render (Go port)

**Domain**: go-port

## Overview

The Go port's pipeline stages between the input layer ([fact-and-sources](/go-port/fact-and-sources.md)) and the command edge ([command-edge](/go-port/command-edge.md)): `query` filters, rolls up, and groups `[]fact.Record`; `view` shapes a query result into a render-agnostic `Table` model; `render` and its `ansi`/`json` subpackages encode the model into output lines. None of these packages imports I/O, exec, or the ccusage registry; encoders return `[]string`, and nothing below `cmd/tu` writes to a stream.

## Requirements

### Requirement: Period and label arithmetic
`package query` defines `Period` (`Daily`, `Weekly`, `Monthly`) with `String()` returning `daily`/`weekly`/`monthly` — the table heading text. `CurrentLabel(p, now)` computes the period's label in `now.Location()` (local time, mirroring the TS `currentLabel`): daily `2006-01-02`; weekly the ISO date of the current week's Sunday, `now.AddDate(0, 0, -int(now.Weekday()))` (month/year underflow normalizes); monthly `2006-01`. The edge passes `time.Now()`; `time.Local` honors `$TZ` (the harness `tz: alt` axis). `WeekLabel(daily)` maps a daily ISO label to its week's Sunday using UTC date arithmetic on the date-only string (DST-immune); a label that does not parse as `2006-01-02` is returned unchanged and becomes its own bucket.

#### Scenario: Labels for a fixed clock
- **GIVEN** `now` = 2026-09-16 12:00 in `Asia/Kolkata`
- **WHEN** `CurrentLabel` runs for the three periods
- **THEN** the results are `2026-09-16`, `2026-09-13`, `2026-09`; `WeekLabel("2026-01-01")` (a Thursday) is `2025-12-28` and `WeekLabel("garbage")` is `garbage`

### Requirement: Pure filters
`Window(recs, since, until)` keeps records with `since <= Date <= until` by lexicographic comparison on the ISO labels, inclusive on both ends; an empty bound is open on that side. `ByTool(recs, key)` / `ByUser(recs, user)` keep records whose `Tool`/`User` equals the argument. None mutates its input or returns the input slice.

### Requirement: RollUp re-labels and sums
`RollUp(recs, p)` re-labels each record to its period bucket (daily identity; weekly `WeekLabel(Date)`; monthly `Date[:7]`) and sums `Totals` via `Totals.Add` over records sharing `(Date', Tool, User, Machine)`. `TotalTokens` is summed, never recomputed. Output is ascending by `Date` via `sort.SliceStable` (byte order equals the TS `localeCompare` on ISO labels), first-seen order within a label. Daily returns a copy, never the input slice.

### Requirement: GroupBy is the one group-by
`Dim` is `Date`, `Tool`, `User`, `Machine`. `GroupBy(recs, dims ...Dim) []Group` sums `Totals` per distinct tuple of the requested dims; `Group.Key` carries only the grouped dims (its other string fields empty). Groups are emitted in first-seen input order, so a registry-ordered input yields registry-ordered groups; `GroupBy` never sorts and never mutates its input. Every pivot shares it: the snapshot is `GroupBy(Window(RollUp(recs, p), cur, cur), Tool)`; history groups on `Date`; machine columns on `Tool, Machine`; leaderboards on `User`. No per-dimension aggregation code exists anywhere.

### Requirement: The view.Table model carries no ANSI
`package view` defines `Align` (`Left`/`Right`), `RowKind` (`Header`/`Divider`/`Data`/`Total`), `Column{Title, Width, Align}`, `Cell{Text, Dim}`, `Row{Kind, Cells, Bar, Delta}`, and `Table{Title, Columns, Rows, Empty, Legend, Footer}`. `Cell.Dim` marks exact-zero cell styling (pivot/machine columns) and is false everywhere in the snapshot; `Row.Bar`/`Row.Delta`, `Table.Legend`, and `Table.Footer` are reserved slots (history bars and summary footer, machine legend, watch deltas) and empty in the snapshot. The package imports only `fact`, `query`, `render`, and stdlib.

### Requirement: Snapshot builds the cross-tool table
`ToolTotals{Name, Label string; fact.Totals}` is one snapshot row candidate: `Name` is the display name resolved by `command` from the registry (view knows no registry); `Label` is the current label when the tool had a matching record (consumed by render/json). `Snapshot(rows, p)` reproduces the TS `renderTotal` rules:

- `Title` is `📊 Combined Usage ({p})` — also for a single-source snapshot, never the tool name.
- Columns are `Tool` (12, left) then `Tokens`, `Input`, `Output`, `Cache`, `Cost` (12 each, right) — fixed, never data-sized; a wider value overflows its cell.
- One `Data` row per input row with `TotalTokens > 0`, in input order: `Name`, `FormatInt(TotalTokens)`, `FormatInt(InputTokens)`, `FormatInt(OutputTokens)`, `FormatInt(CacheCreationTokens+CacheReadTokens)`, `FormatCost(TotalCost)`.
- `Empty = "  No usage"` (two leading spaces) with no header/divider rows when every input row has `TotalTokens == 0`.
- A `Divider` + `Total` row appears only when more than one row is visible; the Total sums every input row, visible or not — the TS accumulates the grand totals before checking visibility, so a tool with cost but zero tokens is hidden yet counted.
- Token mode (`-t`, `--metric tokens`) changes nothing in the one-shot table (the delta indicator is watch-only).

### Requirement: FormatInt and FormatCost reproduce en-US toLocaleString
`render.FormatInt(int64)` groups thousands with commas (`24400` → `24,400`; negative values group the magnitude behind the sign). `render.FormatCost(float64)` returns `$` + the value with exactly two decimals and grouping, reproducing `toLocaleString("en-US", {minimumFractionDigits: 2, maximumFractionDigits: 2})`. The rounding runs on the **shortest round-trip decimal representation** of the double (`strconv.FormatFloat(x, 'f', -1, 64)` — Go's and V8's shortest-digit algorithms agree), rounded to two fraction digits half away from zero with carry into the integer part, then grouped. It is NOT `strconv.FormatFloat(x, 'f', 2, 64)`, which rounds the exact binary value half-even. Negative zero renders `$0.00` (costs are non-negative sums).

Verified divergences (node v24 vs Go, 2026-09-16): `1.005 → $1.01` (Go `'f',2`: `1.00`); `0.125 → $0.13` (Go `0.12`); `0.015 → $0.02` (Go `0.01`); `2.675 → $2.68` (Go `2.67`); `999999.995 → $1,000,000.00` (Go `999999.99`); `4.936068800000001 → $4.94`; `1234567.891 → $1,234,567.89`.

### Requirement: ansi primitives with Colors as a value
`render/ansi` defines `Colors{Enabled bool}`, computed once at the command edge as `!--no-color && NO_COLOR == ""` and passed down; no global color state exists. The methods `Bold`, `Dim`, `Green`, `Red`, `Cyan`, `Yellow`, `Magenta`, `Blue`, `BoldWhite`, `BoldCyan`, `BrightGreen`, `DimGreen` wrap with the layouts.md Color Reference codes (`\x1b[1m`, `\x1b[2m`, `\x1b[32m`, `\x1b[31m`, `\x1b[36m`, `\x1b[33m`, `\x1b[35m`, `\x1b[34m`, `\x1b[1;37m`, `\x1b[1;36m`, `\x1b[92m`, `\x1b[2;32m`) and close with `\x1b[0m`, returning the input unchanged when `!Enabled`. Color is emitted whether or not stdout is a TTY (no isatty probing); `--no-color` and a non-empty `NO_COLOR` produce byte-identical output. `StripANSI` removes the SGR pattern `\x1b\[[0-9;]*m`; `PadLeft`/`PadRight` pad by rune count (snapshot text is ASCII; the 📊 title is never padded).

### Requirement: ansi.Table emits the renderTotal line layout
`ansi.Table(t view.Table, c Colors) []string` returns: `""`, `BoldWhite(title)`, `""`; then, if `Empty != ""`, the empty text and a trailing `""`; else the Header row (each cell padded per column, then BoldCyan-wrapped **individually**, joined by unstyled ` | `), Divider rows as `Dim` of `─`×width per column joined by `─|─`, Data rows as padded cells joined ` | ` with no escapes, the Total row as padded cells each BoldWhite-wrapped individually, then `Legend`/`Footer` when non-empty, then `""`. The snapshot divider is 87 visible characters.

### Requirement: json.Snapshot writes the ordered snapshot object
`render/json.Snapshot(rows []view.ToolTotals) []string` produces the lines of `JSON.stringify(obj, null, 2)`: an object keyed by display name in input order; per row `"label"` first when `Label != ""`, then the six pinned totals keys (`totalCost`, `inputTokens`, `outputTokens`, `cacheCreationTokens`, `cacheReadTokens`, `totalTokens`); two-space indent. The writer is hand-ordered — Go maps are unordered and struct marshalling cannot emit the conditional `label` key first — and uses `encoding/json` only for scalars: floats follow the ES6 rules `encoding/json` implements (shortest repr, `1e+21`/`1e-7` exponent thresholds) with `-0` normalized to `0`; strings encode without HTML escaping. The trailing newline of `console.log` is the caller's per-line `Fprintln`.

### Requirement: Golden-file encoder tests
The `render/ansi` and `render/json` encoders are pinned by golden files under each package's `testdata/` directory (`ansi`: `snapshot_color`, `snapshot_nocolor`, `snapshot_single`, `snapshot_empty`; `json`: `all_tools_labeled`, `all_tools_unlabeled`, `all_zero`, `single_tool`), regenerated with a package-level `-update` flag (`go test ./internal/render/ansi/ -update`). Every colored golden is asserted equal to its no-color twin under `StripANSI`. The populated snapshot path is covered only by these goldens (with injected clocks upstream) and by the local-capture harness run — the placeholder corpus's dates never match "today", so every `--placeholder` harness snapshot case renders the empty state (see [differential-harness](/harness/differential-harness.md)).

## Design Decisions

### Colors is a value, not a global
**Decision**: `ansi.Colors{Enabled}` is computed once in `cmd/tu` and passed to the renderers.
**Why**: The TS `setNoColor()` module global is the output-channel leak the plan removes; a value keeps `render` pure and testable.
**Rejected**: A package-level `SetNoColor` mirroring the TS.
*Introduced by*: 260916-3am6-query-view-render-snapshot

### Number formatting lives in the parent render package
**Decision**: `render.FormatInt`/`FormatCost` sit in `internal/render`, shared by `render/ansi` and the later `render/markdown`.
**Why**: One implementation of the ICU rounding rule serves the encoders that share it.
**Rejected**: Formatting inside `view` (the rule belongs to the encoder family); reusing `FormatCost` for CSV — CSV's cost rule is `toFixed(2)`, which rounds the exact binary value half-up (`1.005 → 1.00`, `0.125 → 0.13`), a DIFFERENT rule from FormatCost's ICU rule, so the CSV encoder must implement its own.
*Introduced by*: 260916-3am6-query-view-render-snapshot

### view receives display names, not registry keys
**Decision**: `command` resolves `ccusage.Lookup(key).Name` and hands `view.ToolTotals{Name}` in registry order.
**Why**: Keeps `query`/`view`/`render` free of the exec package (the layering the plan's G1 gate checks).
**Rejected**: Moving the registry into `fact` (churns the input layer for no gain).
*Introduced by*: 260916-3am6-query-view-render-snapshot

### GroupBy emits groups in first-seen order
**Decision**: No sorting inside `GroupBy`; callers order their input.
**Why**: Registry-ordered input yields registry-ordered columns with no extra pass; history callers sort by label before grouping.
**Rejected**: Sorting by key inside `GroupBy` (would reorder tool columns alphabetically).
*Introduced by*: 260916-3am6-query-view-render-snapshot
