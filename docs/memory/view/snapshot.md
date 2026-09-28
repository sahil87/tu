---
type: memory
description: The cross-tool snapshot table model — view.Snapshot over []ToolTotals with the 12-floored data-sized numeric columns (87 chars at typical values), the source-keyed title (single-source names the tool), the visibility and Total rules, the metric-following watch delta, appended machine columns, and the compact form.
---

# Snapshot Table

**Domain**: view

## Overview

`view.Snapshot` builds the cross-tool snapshot `Table` from `[]view.ToolTotals` — one row per tool with usage in the current period — and `view.CompactSnapshot` is its narrow-terminal watch form; [command/run-and-result](/command/run-and-result.md) assembles the inputs and [render/ansi](/render/ansi.md) and [render/json](/render/json.md) encode the model.

## Requirements

### Requirement: Row candidates and visibility
`view.ToolTotals{Name, Label string; fact.Totals}` (`internal/view/snapshot.go`) is one snapshot row candidate: command resolves the display `Name` from the registry (view knows no registry) and stamps `Label` with the current period's label on every row, data or not (consumed by [render/json](/render/json.md)). `Snapshot(rows, p, bd, SnapshotOptions{Metric, Prev, Single})` MUST emit one Data row per input row with `TotalTokens > 0`, in input order, with cells Name, `render.FormatInt(TotalTokens)`, `FormatInt(InputTokens)`, `FormatInt(OutputTokens)`, `FormatInt(CacheCreationTokens + CacheReadTokens)`, `render.FormatCost(TotalCost)`. The title (`snapshotTitle`) MUST be `📊 {Tool} Usage ({p})` when `Single` is set and rows is non-empty, else `📊 Combined Usage ({p})` — the source, not the row count, decides, so a one-tool all-tools snapshot stays `Combined`. When every input row has `TotalTokens == 0`, the table MUST be `Empty = "  No usage"` (two leading spaces) with no rows.

### Requirement: Data-sized numeric columns
The `Tool` column is fixed at 12-wide left; the numeric columns `Tokens`, `Input`, `Output`, `Cache`, `Cost` are right-aligned and data-sized above a 12 floor (`snapshotNameWidth`/`snapshotNumWidth`, `snapshotColumnsSized` in `internal/view/snapshot.go`): each is `max(12, widest rendered cell across the data and Total rows)` — the header titles are all shorter than the floor. With every value ≤ 12 chars the full row measures 87 visible characters (`12 + 5×12 + 5×3`); a wider value widens only its own column, keeping the header, dividers and rows aligned. The combined `Cache` column (write + read) sits between `Output` and `Cost` so the visible numeric columns close arithmetically (`Input + Output + Cache = Tokens`).

### Requirement: The Total row counts hidden tools
A Divider + Total row MUST appear only when more than one row is visible, and the Total MUST sum every input row, visible or not — `Snapshot` accumulates the grand totals over all input rows before checking visibility, so a tool with cost but zero tokens is hidden yet counted.

#### Scenario: A cost-only tool is hidden but counted
- **GIVEN** one tool with `TotalTokens == 0, TotalCost > 0` and two visible tools
- **WHEN** `Snapshot` builds the table
- **THEN** the hidden tool has no Data row, and the Total row sums its cost into the grand totals; with only one visible tool there is no Total row at all

### Requirement: Only the watch delta follows the metric
The one-shot columns are metric-neutral — the table stays token/dollar-mixed under `-t`. Only the watch delta indicator follows `SnapshotOptions.Metric`: under Tokens the arrow rides the Tokens cell (the Cost cell stays plain), under Cost it rides the Cost cell, resolved from `Prev[Name]` by `rowDelta`. The arrow sits IN the cell text before padding (`Table.DeltaInCell = DeltaPadsArrow` — the raw-length padding quirk: with color on, the cell renders effectively unpadded). (4pze, 018g)

### Requirement: Machine columns append after Cost
A non-nil `*Breakdown` with at least one name MUST append the letter-coded machine columns after `Cost` (`internal/view/snapshot.go`): cells in the DISPLAYED metric (dim on exact zero), per-name sums over the VISIBLE rows on the Total row, the names union spanning every input row (a hidden tool can still carry slices), and the legend as `Table.Note`. A nil breakdown yields byte-identical no-breakdown output. See [breakdown](/view/breakdown.md). (pmsd)

### Requirement: The compact snapshot drops machine columns
`CompactSnapshot(rows, p, o SnapshotOptions)` MUST follow the full table's title rule (`📊 {Tool} Usage ({p})` single-source via `o.Single`, else `📊 Combined Usage ({p})`), emit one row per tool with `TotalTokens > 0` carrying the `o.Metric` value and the `o.Prev` delta keyed by name, and emit a Total summing the metric over ALL input rows only when more than one row is visible. Machine columns and the legend are dropped — the compact branch runs before them. The empty check runs BEFORE the compact branch.

## Design Decisions

### Snapshot shows one combined Cache column
**Decision**: A single `Cache` column (write + read) sits between `Output` and `Cost` on the 12-floored layout, landing the full row at 87 characters for typical values.
**Why**: `Tokens` includes cache, so without the column the visible breakdown appeared off by orders of magnitude on cache-heavy data and read as a bug; the combined column closes the row arithmetic (`Input + Output + Cache = Tokens`) with data already on `fact.Totals`.
**Rejected**: Separate Cache Write / Cache Read columns (single-tool granularity, too wide for an at-a-glance snapshot); a compact `487.7M (99% cache)` treatment (departs from the tabular idiom); 14-wide numerics (99-char rows past the width budget).
*Introduced by*: 260815-nda3-snapshot-cache-token-visibility

### Snapshot table is metric-neutral; only the delta follows the metric
**Decision**: The columns stay token/dollar-mixed under `-t`; only the watch delta arrow follows the metric, riding the Tokens cell under tokens and the Cost cell under cost.
**Why**: The snapshot is already token-denominated, so it needs no layout; comparing a dollar cell against a token-valued prev map would be wrong, and two prev maps per poll is a second pathway.
**Rejected**: Hiding the Cost column in token mode (loses the dollar reading the snapshot exists for); a prev map per metric (a second pathway per poll).
*Introduced by*: 260828-018g-tokens-table-mode-t-flag

### Snapshot numeric columns are data-sized above a 12 floor
**Decision**: Each numeric column's width is `max(12, widest rendered cell across the data and Total rows)`, following the machine-column sizing pattern; the `Tool` column stays fixed at 12.
**Why**: On a fixed 12-wide column a wider value (e.g. a 14-char Tokens total) overflows its cell and shifts that row out of alignment with the header and dividers; the floor keeps every typical snapshot byte-identical to the 87-char layout.
**Rejected**: Widening every numeric column to a fixed 14 (99-char rows past the width budget); leaving the overflow (a misaligned row on large values).
*Introduced by*: 260928-lfj9-output-drop-at-cutover-fixes

### A single-source snapshot is titled after the tool
**Decision**: `SnapshotOptions.Single` (the caller's `req.Source != ""`) selects the title `📊 {Tool} Usage ({p})`; the all-tools snapshot keeps `📊 Combined Usage ({p})`. The compact snapshot and the Markdown encoder follow the same rule (Markdown without the emoji).
**Why**: A single-source snapshot titled "Combined" misdescribes a one-tool table; the source, not the row count, is the real signal — a one-tool all-tools snapshot is still "Combined".
**Rejected**: Inferring single-source from the visible row count inside `view` (a one-tool all-tools snapshot would be mistitled).
*Introduced by*: 260928-lfj9-output-drop-at-cutover-fixes

### Command resolves display names; view knows no registry
**Decision**: `command` resolves `fact.Tool` display names and hands `view.ToolTotals{Name}` in registry order; `view` never imports the registry.
**Why**: Keeps `view` and the encoders free of the registry and the exec adapters — the layering the pure-model architecture requires.
**Rejected**: Letting `view` import the registry (pulls the registry into the pure layer).
*Introduced by*: 260916-3am6-query-view-render-snapshot
