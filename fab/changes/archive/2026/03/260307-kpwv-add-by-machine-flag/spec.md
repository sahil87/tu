# Spec: Add --by-machine Flag

**Change**: 260307-kpwv-add-by-machine-flag
**Created**: 2026-03-07
**Affected memory**: `docs/memory/cli/data-pipeline.md`, `docs/memory/display/formatting.md`, `docs/memory/sync/multi-machine.md`

## Non-Goals

- All-tools history pivot (`tu h --by-machine`, `tu mh --by-machine`) — the pivot already uses tool names as dynamic columns; adding machine columns per tool creates a matrix, not a table
- Per-machine token breakdown — only cost ($) is shown per machine; full token columns per machine would overflow the terminal

## CLI: Flag Parsing

### Requirement: --by-machine Flag Registration

The CLI SHALL accept `--by-machine` as a global boolean flag. It SHALL be added to the `GlobalFlags` interface as `byMachineFlag: boolean`. The flag SHALL be filtered from positional args during parsing (same pattern as `--no-color`, `--no-rain`).

#### Scenario: Flag parsed and filtered
- **GIVEN** the user runs `tu cc h --by-machine`
- **WHEN** `parseGlobalFlags` processes the args
- **THEN** `byMachineFlag` is `true`
- **AND** `filteredArgs` is `["cc", "h"]` (flag removed)

#### Scenario: Flag absent
- **GIVEN** the user runs `tu cc h`
- **WHEN** `parseGlobalFlags` processes the args
- **THEN** `byMachineFlag` is `false`

### Requirement: --by-machine in Help Text

`FULL_HELP` SHALL include a line for `--by-machine` in the Flags section:
```
  --by-machine           Show per-machine cost breakdown (data commands only)
```

### Requirement: --by-machine Incompatible with All-Tools History Pivot

When `--by-machine` is active AND the resolved display mode is all-tools history (source=`all`, display=`history`), the CLI SHALL warn on stderr (`Warning: --by-machine is not supported with all-tools history — ignoring.`) and proceed without machine columns.

#### Scenario: Incompatible combination warned
- **GIVEN** the user runs `tu h --by-machine`
- **WHEN** dispatch resolves source=`all`, display=`history`
- **THEN** stderr contains `Warning: --by-machine is not supported with all-tools history — ignoring.`
- **AND** the output renders the normal all-tools history pivot without machine columns

#### Scenario: Compatible combination proceeds
- **GIVEN** the user runs `tu cc h --by-machine`
- **WHEN** dispatch resolves source=`cc`, display=`history`
- **THEN** no warning is emitted
- **AND** the output includes machine columns

## Sync: readRemoteEntriesByMachine

### Requirement: Grouped Machine Reads

`src/node/sync/sync.ts` SHALL export a new function `readRemoteEntriesByMachine(metricsDir, targetUser, excludeMachine, toolKey)` returning `Map<string, UsageEntry[]>` where keys are machine directory names and values are that machine's entries for the given tool.

The function SHALL walk the same `{user}/{year}/{machine}/` directory structure as `readRemoteEntries`. When `excludeMachine` is non-null, that machine SHALL be excluded from the map. Error handling SHALL match `readRemoteEntries` (skip unreadable dirs/files silently).

#### Scenario: Two machines in metrics repo
- **GIVEN** a metrics repo with entries under `sahil/2026/macbook/` and `sahil/2026/studio/`
- **WHEN** `readRemoteEntriesByMachine(dir, "sahil", null, "cc")` is called
- **THEN** the returned map has keys `"macbook"` and `"studio"`
- **AND** each value contains the `UsageEntry[]` for that machine

#### Scenario: Exclude local machine
- **GIVEN** a metrics repo with entries under `sahil/2026/macbook/` and `sahil/2026/studio/`
- **WHEN** `readRemoteEntriesByMachine(dir, "sahil", "macbook", "cc")` is called
- **THEN** the returned map has only key `"studio"`

#### Scenario: Empty metrics directory
- **GIVEN** the user path does not exist
- **WHEN** `readRemoteEntriesByMachine` is called
- **THEN** an empty map is returned

### Requirement: readRemoteEntries Delegates

The existing `readRemoteEntries` SHALL be refactored to delegate to `readRemoteEntriesByMachine` and flatten the result. Its public signature and behavior SHALL remain unchanged.

#### Scenario: Existing behavior preserved
- **GIVEN** existing callers of `readRemoteEntries`
- **WHEN** they call `readRemoteEntries(dir, "sahil", "macbook", "cc")`
- **THEN** the result is identical to the pre-refactor implementation (flat `UsageEntry[]`)

## Data Pipeline: fetchToolMerged with Machine Grouping

### Requirement: Machine-Aware Fetch

`fetchToolMerged` SHALL accept an optional `byMachine` boolean parameter (default `false`). When `byMachine` is `true`, the function SHALL return an object `{ entries: UsageEntry[], machineMap: Map<string, UsageEntry[]> }` instead of a plain `UsageEntry[]`.

To support both return types without breaking existing callers, a new function `fetchToolMergedWithMachines` SHALL be introduced with the enriched return type. `fetchToolMerged` SHALL remain unchanged.

```typescript
interface MergedResult {
  entries: UsageEntry[];
  machineMap: Map<string, UsageEntry[]>;
}

async function fetchToolMergedWithMachines(
  config: TuConfig,
  toolKey: string,
  period: string,
  extra: string[],
  skipCache?: boolean,
  targetUser?: string,
): Promise<MergedResult>
```

#### Scenario: Multi mode, self-user, by-machine
- **GIVEN** config.mode is `multi`, config.user is `sahil`, config.machine is `macbook`
- **WHEN** `fetchToolMergedWithMachines(config, "cc", "daily", [])` is called
- **THEN** `entries` contains the merged flat entries (same as `fetchToolMerged`)
- **AND** `machineMap` has key `"macbook"` with local entries and key `"studio"` with remote entries (if studio has data)

#### Scenario: Multi mode, different user, by-machine
- **GIVEN** config.mode is `multi`, targetUser is `"alice"` (different from config.user)
- **WHEN** `fetchToolMergedWithMachines(config, "cc", "daily", [], false, "alice")` is called
- **THEN** `machineMap` contains all of alice's machines (excludeMachine is null)
- **AND** `entries` is the flat merge of all machine entries

#### Scenario: Single mode, by-machine
- **GIVEN** config.mode is `single`, config.machine is `macbook`
- **WHEN** `fetchToolMergedWithMachines(config, "cc", "daily", [])` is called
- **THEN** `machineMap` has one key `"macbook"` with the local entries
- **AND** `entries` is identical to the local entries

#### Scenario: Monthly aggregation with machine map
- **GIVEN** period is `monthly`
- **WHEN** `fetchToolMergedWithMachines` returns
- **THEN** both `entries` and each value in `machineMap` are aggregated monthly via `aggregateMonthly`

## Display: Machine Columns in Tables

### Requirement: Letter-Coded Machine Columns in renderHistory

When `FormatOptions.machineMap` is provided and non-empty, `renderHistory` SHALL append machine cost columns after the Cost column. Machine names SHALL be sorted alphabetically and assigned uppercase letters starting from A. Column width SHALL be 8 characters, right-aligned (matching `COST_WIDTH`).

A legend line SHALL be rendered after the table: `Machines: A = {name}, B = {name}, ...` in `dim` color.

The Total row (when present) SHALL include per-machine cost totals.

#### Scenario: Two-machine history table
- **GIVEN** entries for dates 2026-03-05 and 2026-03-06
- **AND** machineMap has `macbook` ($1.90, $2.10) and `studio` ($0.95, $0.00)
- **WHEN** `renderHistory` is called with the machineMap
- **THEN** headers include `... | Cost | A | B`
- **AND** row 2026-03-06 shows `... | $2.85 | $1.90 | $0.95`
- **AND** the Total row shows per-machine sums
- **AND** a legend line reads `Machines: A = macbook, B = studio`

#### Scenario: Machine map empty (no by-machine flag)
- **GIVEN** machineMap is undefined or empty
- **WHEN** `renderHistory` is called
- **THEN** output is identical to current behavior (no machine columns, no legend)

### Requirement: Letter-Coded Machine Columns in renderTotal

When `FormatOptions.machineMap` is provided and non-empty, `renderTotal` SHALL append machine cost columns after the Cost column, using the same letter assignment and column width as `renderHistory`.

For snapshot mode, each tool row's machine cost SHALL be the current-period entry's `totalCost` from that machine's entries for that tool. The dispatch layer SHALL pre-compute per-tool machine costs and pass them via `FormatOptions`.

#### Scenario: Two-machine snapshot table
- **GIVEN** toolTotals for Claude Code ($12.34) and Codex ($23.45)
- **AND** machineMap shows macbook: CC=$8.20, Codex=$23.45; studio: CC=$4.14, Codex=$0.00
- **WHEN** `renderTotal` is called with the machineMap
- **THEN** headers include `... | Cost | A | B`
- **AND** Claude Code row shows `... | $12.34 | $8.20 | $4.14`
- **AND** Total row shows per-machine sums

### Requirement: Machine Columns Omitted in Compact Mode

When `opts.compact` is `true` (narrow terminal in watch mode), machine columns SHALL NOT be rendered regardless of machineMap presence. Compact mode preserves its minimal name+cost layout.

#### Scenario: Compact mode ignores machine data
- **GIVEN** compact mode is active (terminal width < 60)
- **AND** machineMap is provided
- **WHEN** `renderHistory` or `renderTotal` is called
- **THEN** output uses the compact layout without machine columns

### Requirement: FormatOptions Extension

`FormatOptions` in `src/node/tui/formatter.ts` SHALL add an optional `machineMap` field:

```typescript
export interface FormatOptions {
  prevCosts?: Map<string, number>;
  compact?: boolean;
  maxRows?: number;
  machineMap?: Map<string, number>;
  // key: machine name, value: cost for this row/tool (pre-computed by dispatch)
}
```

The dispatch layer SHALL pre-compute per-machine costs into a simple `Map<string, number>` per render call, keeping the formatter stateless.

## Display: Dispatch Layer Threading

### Requirement: Machine Data Threading in Dispatch

The dispatch functions (`dispatchAllSnapshot`, `dispatchSingleTool`, `dispatchAllHistory`, and their `*Lines` variants) SHALL accept a `byMachineFlag` parameter. When `true` and the display mode is compatible, they SHALL call `fetchToolMergedWithMachines` instead of `fetchToolMerged` and construct the `machineMap` for `FormatOptions`.

For **snapshot** mode: the dispatch layer SHALL compute per-machine costs for the current period label by iterating each machine's entries in the `MergedResult.machineMap`.

For **single-tool history** mode: the dispatch layer SHALL compute per-machine costs per date label.

For **all-tools history** (pivot): `byMachineFlag` SHALL be ignored (no machine columns).

#### Scenario: Snapshot dispatch with by-machine
- **GIVEN** `byMachineFlag` is `true` and source is `all`
- **WHEN** `dispatchAllSnapshot` processes the data
- **THEN** `renderTotal` receives a `machineMap` with per-machine costs for the current period

#### Scenario: Single-tool history dispatch with by-machine
- **GIVEN** `byMachineFlag` is `true` and source is `cc`, display is `history`
- **WHEN** `dispatchSingleTool` processes the data
- **THEN** `renderHistory` receives per-row machineMap data

## Display: JSON Output with Machine Breakdown

### Requirement: JSON Machine Breakdown

When `--by-machine` and `--json` are both active, the JSON output SHALL include a `machines` key in each tool/entry object containing per-machine costs.

For **snapshot** mode:
```json
{
  "Claude Code": {
    "totalCost": 12.34,
    "inputTokens": 567890,
    "outputTokens": 666677,
    "cacheCreationTokens": 23456,
    "cacheReadTokens": 12345,
    "totalTokens": 1234567,
    "machines": {
      "macbook": 8.20,
      "studio": 4.14
    }
  }
}
```

For **single-tool history** mode:
```json
[
  {
    "label": "2026-03-06",
    "totalCost": 2.85,
    "inputTokens": 567890,
    "outputTokens": 666677,
    "cacheCreationTokens": 23456,
    "cacheReadTokens": 12345,
    "totalTokens": 1234567,
    "machines": {
      "macbook": 1.90,
      "studio": 0.95
    }
  }
]
```

#### Scenario: JSON snapshot with machines
- **GIVEN** `--json` and `--by-machine` are both active, source is `all`
- **WHEN** the snapshot JSON is emitted
- **THEN** each tool object includes a `machines` key mapping machine names to cost numbers

#### Scenario: JSON without by-machine
- **GIVEN** `--json` is active but `--by-machine` is not
- **WHEN** JSON is emitted
- **THEN** no `machines` key appears in the output (backwards-compatible)

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Flag name is `--by-machine` | Confirmed from intake #1 — user explicitly chose | S:95 R:90 A:95 D:95 |
| 2 | Certain | Letter-coded columns (A, B, C) with legend line | Confirmed from intake #2 — user proposed this format | S:95 R:85 A:90 D:95 |
| 3 | Certain | Works with snapshot and single-tool history only | Confirmed from intake #3 — user confirmed scope | S:95 R:90 A:90 D:95 |
| 4 | Certain | Incompatible with all-tools history pivot — warn and ignore | Confirmed from intake #4 — user agreed | S:90 R:90 A:90 D:95 |
| 5 | Certain | Single mode is orthogonal — one machine, no special-casing | Confirmed from intake #6 — user corrected initial assumption | S:95 R:90 A:90 D:95 |
| 6 | Certain | `Map<string, UsageEntry[]>` as grouped return type from sync layer | Confirmed from intake #7 | S:90 R:85 A:90 D:90 |
| 7 | Certain | New `fetchToolMergedWithMachines` function preserves existing `fetchToolMerged` | Codebase pattern — existing callers untouched, new function for enriched return | S:85 R:95 A:90 D:90 |
| 8 | Confident | Machine column width is 8 chars (same as COST_WIDTH) | Confirmed from intake #8 — matches existing constant | S:70 R:90 A:85 D:80 |
| 9 | Confident | Machines sorted alphabetically for letter assignment | Confirmed from intake #9 — standard ordering | S:65 R:95 A:80 D:75 |
| 10 | Confident | Total row includes per-machine cost totals | Confirmed from intake #10 — consistency with existing patterns | S:60 R:90 A:85 D:80 |
| 11 | Confident | JSON `machines` maps machine names to cost numbers (not objects) | Simpler than `{ totalCost: N }` wrapper; only cost is shown per machine | S:70 R:90 A:80 D:75 |
| 12 | Certain | Pre-compute per-machine costs in dispatch, pass simple Map to formatter | Keeps formatter stateless; dispatch already knows the tool/period context | S:80 R:90 A:90 D:85 |
| 13 | Confident | refactor readRemoteEntries to delegate to readRemoteEntriesByMachine | Avoids code duplication; intake explicitly suggested this | S:75 R:85 A:85 D:80 |

13 assumptions (8 certain, 5 confident, 0 tentative, 0 unresolved).
