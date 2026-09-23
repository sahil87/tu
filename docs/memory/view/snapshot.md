---
type: memory
description: The cross-tool snapshot table model — view.Snapshot over []ToolTotals with the fixed 87-char column layout, the visibility and Total rules, the metric-following watch delta, appended machine columns, and the compact form.
---

# Snapshot Table

**Domain**: view

## Overview

`view.Snapshot` builds the cross-tool snapshot `Table` from `[]view.ToolTotals` — one row per tool with usage in the current period — and `view.CompactSnapshot` is its narrow-terminal watch form; [command/run-and-result](/command/run-and-result.md) assembles the inputs and [render/ansi](/render/ansi.md) and [render/json](/render/json.md) encode the model.

## Requirements

### Requirement: Row candidates and visibility
`view.ToolTotals{Name, Label string; fact.Totals}` (`internal/view/snapshot.go`) is one snapshot row candidate: command resolves the display `Name` from the registry (view knows no registry) and `Label` is the current period's label when the tool had a matching record (consumed by [render/json](/render/json.md)). `Snapshot(rows, p, bd, SnapshotOptions{Metric, Prev})` MUST emit one Data row per input row with `TotalTokens > 0`, in input order, with cells Name, `render.FormatInt(TotalTokens)`, `FormatInt(InputTokens)`, `FormatInt(OutputTokens)`, `FormatInt(CacheCreationTokens + CacheReadTokens)`, `render.FormatCost(TotalCost)`. When every input row has `TotalTokens == 0`, the table MUST be `Empty = "  No usage"` (two leading spaces) with no rows.

### Requirement: Fixed column layout
The snapshot columns are fixed, never data-sized: `Tool` 12-wide left, then `Tokens`, `Input`, `Output`, `Cache`, `Cost` each 12-wide right (`snapshotNameWidth`/`snapshotNumWidth` in `internal/view/snapshot.go`). The full row measures 87 visible characters (`12 + 5×12 + 5×3`); a wider value overflows its cell while the header and dividers keep their width. The combined `Cache` column (write + read) sits between `Output` and `Cost` so the visible numeric columns close arithmetically (`Input + Output + Cache = Tokens`).

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
`CompactSnapshot(rows, p, m, prev)` MUST keep the full table's title (`📊 Combined Usage ({p})`), emit one row per tool with `TotalTokens > 0` carrying the metric value and the `Prev` delta keyed by name, and emit a Total summing the metric over ALL input rows only when more than one row is visible. Machine columns and the legend are dropped — the compact branch runs before them. The empty check runs BEFORE the compact branch.

## Design Decisions

### Snapshot shows one combined Cache column
**Decision**: A single `Cache` column (write + read) sits between `Output` and `Cost` on the fixed 12-wide layout, landing the full row at 87 characters.
**Why**: `Tokens` includes cache, so without the column the visible breakdown appeared off by orders of magnitude on cache-heavy data and read as a bug; the combined column closes the row arithmetic (`Input + Output + Cache = Tokens`) with data already on `fact.Totals`.
**Rejected**: Separate Cache Write / Cache Read columns (single-tool granularity, too wide for an at-a-glance snapshot); a compact `487.7M (99% cache)` treatment (departs from the tabular idiom); 14-wide numerics (99-char rows past the width budget).
*Introduced by*: 260815-nda3-snapshot-cache-token-visibility

### Snapshot table is metric-neutral; only the delta follows the metric
**Decision**: The columns stay token/dollar-mixed under `-t`; only the watch delta arrow follows the metric, riding the Tokens cell under tokens and the Cost cell under cost.
**Why**: The snapshot is already token-denominated, so it needs no layout; comparing a dollar cell against a token-valued prev map would be wrong, and two prev maps per poll is a second pathway.
**Rejected**: Hiding the Cost column in token mode (loses the dollar reading the snapshot exists for); a prev map per metric (a second pathway per poll).
*Introduced by*: 260828-018g-tokens-table-mode-t-flag

### Command resolves display names; view knows no registry
**Decision**: `command` resolves `fact.Tool` display names and hands `view.ToolTotals{Name}` in registry order; `view` never imports the registry.
**Why**: Keeps `view` and the encoders free of the registry and the exec adapters — the layering the pure-model architecture requires.
**Rejected**: Letting `view` import the registry (pulls the registry into the pure layer).
*Introduced by*: 260916-3am6-query-view-render-snapshot
