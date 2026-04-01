# Quality Checklist: Add --by-machine Flag

**Change**: 260307-kpwv-add-by-machine-flag
**Generated**: 2026-03-07
**Spec**: `spec.md`

## Functional Completeness

- [x] CHK-001 Flag Registration: `--by-machine` parsed by `parseGlobalFlags`, `byMachineFlag` in `GlobalFlags`, filtered from positional args
- [x] CHK-002 Help Text: `FULL_HELP` includes `--by-machine` line in Flags section
- [x] CHK-003 readRemoteEntriesByMachine: New function exported from sync.ts, returns `Map<string, UsageEntry[]>`
- [x] CHK-004 readRemoteEntries Delegation: Existing function delegates to `readRemoteEntriesByMachine` and flattens, behavior unchanged
- [x] CHK-005 fetchToolMergedWithMachines: New function returns `{ entries, machineMap }` with correct machine grouping
- [x] CHK-006 Machine Columns in renderHistory: Letter-coded columns after Cost, sorted alphabetically, 8-char width, legend line
- [x] CHK-007 Machine Columns in renderTotal: Same letter scheme and legend as renderHistory, per-tool machine costs
- [x] CHK-008 Dispatch Threading: `byMachineFlag` threaded through snapshot and single-tool dispatch functions
- [x] CHK-009 JSON Machine Breakdown: `machines` key with per-machine costs when `--by-machine` + `--json`
- [x] CHK-010 Watch Mode Threading: Machine columns refresh on each poll cycle

## Behavioral Correctness

- [x] CHK-011 Incompatible Pivot: `tu h --by-machine` warns on stderr and renders normal pivot without machine columns
- [x] CHK-012 FormatOptions Extension: `machineCosts` field added to FormatOptions (renamed from spec's `machineMap` for clarity; nested `Map<string, Map<string, number>>` instead of flat — justified deviation), formatter uses it when present
- [x] CHK-013 Compact Mode: Machine columns omitted when compact mode is active

## Scenario Coverage

- [x] CHK-014 Two-machine history: History table shows columns A, B with correct per-machine costs and legend
- [x] CHK-015 Two-machine snapshot: Snapshot table shows columns A, B with per-tool machine costs
- [x] CHK-016 Single mode: One machine column (A) with all costs attributed to hostname
- [x] CHK-017 Exclude local machine: readRemoteEntriesByMachine excludes machine when excludeMachine is set
- [x] CHK-018 Empty metrics dir: readRemoteEntriesByMachine returns empty map
- [x] CHK-019 Monthly aggregation: Both entries and machineMap values aggregated when period is monthly
- [x] CHK-020 JSON snapshot with machines: Each tool object in JSON includes `machines` mapping

## Edge Cases & Error Handling

- [x] CHK-021 No machines in map: When machineMap is empty/undefined, tables render identically to current behavior
- [x] CHK-022 Flag absent: `byMachineFlag` defaults to false, no machine columns anywhere

## Code Quality

- [x] CHK-023 Pattern consistency: New code follows existing patterns (function style, error handling, naming)
- [x] CHK-024 No unnecessary duplication: readRemoteEntries reuses readRemoteEntriesByMachine instead of duplicating walk logic
- [x] CHK-025 Functional style: No classes introduced, functions and plain objects only
- [x] CHK-026 Node-prefixed imports: Any new `node:*` imports use the prefix
- [x] CHK-027 No magic numbers: Column widths use named constants

## Notes

- Check items as you review: `- [x]`
- All items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] CHK-008 **N/A**: {reason}`
