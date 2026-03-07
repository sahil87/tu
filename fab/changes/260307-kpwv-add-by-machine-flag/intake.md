# Intake: Add --by-machine Flag

**Change**: 260307-kpwv-add-by-machine-flag
**Created**: 2026-03-07
**Status**: Draft

## Origin

> Conversational — user asked during `/fab-discuss` session: "How about a flag that shows the distribution of token usage across machines for a user?" After exploring the existing data model, metrics repo structure, and display layouts, the design was refined through several rounds of discussion covering data types, column format, scope, and data path unification.

Key decisions from the conversation:
1. Show dollar amounts only (not token breakdown) per machine
2. Use `--by-machine` as the flag name
3. Letter-coded columns (A, B, C...) with a legend line mapping letters to machine names
4. Works with snapshot and single-tool history; incompatible with all-tools history pivot (warn and ignore)
5. Unified data path: local (this machine) → merge remote (other machines, 0 in single mode) → render
6. Single mode is orthogonal — just happens to have one machine, no special-casing
7. `Map<string, UsageEntry[]>` (machine → entries) as the grouped return type

## Why

In multi-machine mode, `tu` aggregates usage across machines but only shows the combined total. Users with multiple machines (e.g., a MacBook and a workstation) cannot see which machine is driving costs. This makes it hard to understand usage patterns — e.g., "am I burning more on the office desktop or the laptop?"

The metrics repo already stores data per-machine (`{user}/{year}/{machine}/`), so the information is available — it's just flattened during merge. Without this feature, users must manually inspect the metrics repo directory structure to understand per-machine distribution.

## What Changes

### 1. New CLI Flag: `--by-machine`

Add `--by-machine` to the global flags parser in `src/node/core/cli.ts`:

- Parsed alongside existing flags (`--json`, `--sync`, `--fresh`, etc.)
- Boolean flag, no value argument
- Added to `GlobalFlags` interface as `byMachineFlag: boolean`
- Added to `FULL_HELP` output
- Added to `parseGlobalFlags` filter list

**Compatibility rules:**
- Works with snapshot (`tu --by-machine`, `tu cc --by-machine`, `tu m --by-machine`)
- Works with single-tool history (`tu cc h --by-machine`, `tu cc mh --by-machine`)
- Incompatible with all-tools history pivot (`tu h --by-machine`, `tu mh --by-machine`) — warn on stderr and ignore the flag
- Compatible with `--watch` mode (machine columns refresh with each poll)
- Compatible with `--json` (machine breakdown included in JSON output)
- Compatible with `-u` flag (shows target user's per-machine breakdown)

### 2. Data Layer: `readRemoteEntriesByMachine()`

New function in `src/node/sync/sync.ts`:

```typescript
export function readRemoteEntriesByMachine(
  metricsDir: string,
  targetUser: string,
  excludeMachine: string | null,
  toolKey: string,
): Map<string, UsageEntry[]> {
  // Same directory walking as readRemoteEntries, but groups by machine name
  // Returns Map where key = machine name, value = that machine's entries
  // When excludeMachine is non-null, that machine is excluded from the map
}
```

The existing `readRemoteEntries()` can be refactored to delegate to this function and flatten the result.

### 3. Data Path: Unified Local + Remote Merge

Current `fetchToolMerged` has separate branches for `targetUser` and self-user. The by-machine feature needs machine tagging in both paths.

**Proposed flow** (same for multi and single mode):

1. Fetch local entries via `fetchHistory()` — these belong to `config.machine` (or hostname in single mode)
2. Read remote entries grouped by machine via `readRemoteEntriesByMachine()` — excludes local machine to avoid double-counting (returns empty map in single mode since no metrics dir)
3. Build `machineMap: Map<string, UsageEntry[]>` — local machine's entries keyed by its name, plus all remote machine maps
4. Merge all machine entries into a flat `UsageEntry[]` for the existing display path
5. When `--by-machine` is active, pass the `machineMap` alongside merged entries to the formatter

For single mode without a metrics dir: step 2 returns an empty map, so the final `machineMap` has one entry (this machine). No special-casing needed.

### 4. Formatter: Letter-Coded Machine Columns

When `byMachine` data is present, the formatter adds extra columns after the Cost column.

**Column layout for single-tool history** (`renderHistory`):

```
Date         |  Input | Output | ... | Total |     Cost |    A |    B |    C
──────────────────────────────────────────────────────────────────────────────
2026-03-06   |   ...  |   ...  | ... |  ...  |    $2.85 | $1.90| $0.95| $0.00
2026-03-05   |   ...  |   ...  | ... |  ...  |    $2.10 | $2.10| $0.00| $0.00

Machines: A = macbook, B = studio, C = tower
```

**Column layout for snapshot** (`renderTotal`):

```
Tool           |  Tokens |  Input | Output |     Cost |    A |    B
────────────────────────────────────────────────────────────────────
Claude Code    |   ...   |   ...  |  ...   |   $12.34 | $8.20| $4.14
Codex          |   ...   |   ...  |  ...   |   $23.45 |$23.45| $0.00
────────────────────────────────────────────────────────────────────
Total          |   ...   |   ...  |  ...   |   $35.79 |$31.65| $4.14

Machines: A = macbook, B = studio
```

**Letter assignment**: Machines sorted alphabetically by name. A = first, B = second, etc.

**Machine column width**: 8 characters right-aligned (fits `$999.99`), same as the Cost column width constant.

**Legend line**: `Machines: A = {name}, B = {name}, ...` — rendered after the table's trailing blank line, in `dim` color.

**Compact mode** (watch mode, narrow terminal): Machine columns omitted in compact mode to preserve the minimal layout.

### 5. JSON Output

When `--by-machine` and `--json` are combined, include machine breakdown in the output:

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
      "macbook": { "totalCost": 8.20 },
      "studio": { "totalCost": 4.14 }
    }
  }
}
```

### 6. FormatOptions Extension

Extend the `FormatOptions` interface in `src/node/tui/formatter.ts`:

```typescript
export interface FormatOptions {
  prevCosts?: Map<string, number>;
  compact?: boolean;
  maxRows?: number;
  byMachine?: Map<string, Map<string, UsageEntry[]>>;
  // key: tool name (for snapshot) or "self" (for single-tool)
  // value: machine name → entries for that machine
}
```

Or simpler: pass a flat `machineMap: Map<string, UsageEntry[]>` (machine → entries) directly to each render function, since the tool-level grouping is already handled by the dispatch layer.

## Affected Memory

- `cli/data-pipeline`: (modify) Add `--by-machine` flag, `byMachineFlag` in `GlobalFlags`, updated data flow
- `display/formatting`: (modify) Letter-coded machine columns, legend line rendering, `FormatOptions.byMachine`
- `sync/multi-machine`: (modify) New `readRemoteEntriesByMachine()` function, refactored `readRemoteEntries()`

## Impact

- **`src/node/core/cli.ts`**: Flag parsing, dispatch functions gain `byMachine` parameter threading
- **`src/node/core/types.ts`**: Possibly unchanged if we keep `Map<string, UsageEntry[]>` as the grouped type without a new interface
- **`src/node/sync/sync.ts`**: New `readRemoteEntriesByMachine()`, possibly refactor `readRemoteEntries()` to delegate
- **`src/node/tui/formatter.ts`**: `renderHistory` and `renderTotal` gain machine column rendering; `FormatOptions` extended
- **`src/node/tui/watch.ts`**: Pass-through of `byMachine` data (no structural change, just threading)
- **Tests**: New tests for `readRemoteEntriesByMachine`, machine column rendering, flag parsing

## Open Questions

- Should the Total row in the table also show per-machine totals? (Likely yes for consistency.)

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Flag name is `--by-machine` | Discussed — user explicitly chose this name | S:95 R:90 A:95 D:95 |
| 2 | Certain | Letter-coded columns (A, B, C) with legend line | Discussed — user proposed this format to save column width | S:95 R:85 A:90 D:95 |
| 3 | Certain | Works with snapshot and single-tool history | Discussed — user confirmed scope | S:95 R:90 A:90 D:95 |
| 4 | Certain | Incompatible with all-tools history pivot — warn and ignore | Discussed — user agreed to leave pivot out | S:90 R:90 A:90 D:95 |
| 5 | Certain | Unified data path: local → merge remote (0 in single) → render | Discussed — user proposed this unification | S:95 R:80 A:85 D:90 |
| 6 | Certain | Single mode is orthogonal — shows one machine, no special-casing | Discussed — user corrected initial assumption to skip single mode | S:95 R:90 A:90 D:95 |
| 7 | Certain | `Map<string, UsageEntry[]>` as grouped return type | Discussed — user agreed to this approach | S:90 R:85 A:90 D:90 |
| 8 | Confident | Machine column width is 8 chars (same as Cost column) | Matches existing `COST_WIDTH` constant, fits `$999.99` | S:70 R:90 A:85 D:80 |
| 9 | Confident | Machines sorted alphabetically for letter assignment | Standard ordering, predictable for user | S:65 R:95 A:80 D:75 |
| 10 | Confident | Total row includes per-machine totals | Consistency with existing table patterns that always show totals | S:60 R:90 A:85 D:80 |
| 11 | Confident | JSON output includes `machines` object with cost-only breakdown | Mirrors table display; cost-only matches the "just $" decision | S:70 R:85 A:80 D:75 |

11 assumptions (7 certain, 4 confident, 0 tentative, 0 unresolved).
