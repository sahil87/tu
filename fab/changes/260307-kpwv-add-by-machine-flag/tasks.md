# Tasks: Add --by-machine Flag

**Change**: 260307-kpwv-add-by-machine-flag
**Spec**: `spec.md`
**Intake**: `intake.md`

## Phase 1: Setup

- [x] T001 [P] Add `byMachineFlag` to `GlobalFlags` interface and `parseGlobalFlags()` in `src/node/core/cli.ts` — parse `--by-machine`, filter from positional args, add to help text
- [x] T002 [P] Add `readRemoteEntriesByMachine()` to `src/node/sync/sync.ts` — returns `Map<string, UsageEntry[]>` grouped by machine directory name. Refactor `readRemoteEntries()` to delegate to it and flatten

## Phase 2: Core Implementation

- [x] T003 Add `fetchToolMergedWithMachines()` to `src/node/core/cli.ts` — returns `{ entries: UsageEntry[], machineMap: Map<string, UsageEntry[]> }`. Self-user path: local entries keyed by `config.machine`, remote via `readRemoteEntriesByMachine` excluding local. Different-user path: all machines from `readRemoteEntriesByMachine` with null exclude. Single mode: local entries only, one-key map. Apply `aggregateMonthly` to both `entries` and each machineMap value when period is monthly
- [x] T004 [P] Add `machineMap` field to `FormatOptions` in `src/node/tui/formatter.ts` as `machineMap?: Map<string, number>` (machine name → cost, pre-computed by dispatch)
- [x] T005 Add machine column rendering to `renderHistory()` in `src/node/tui/formatter.ts` — sort machine names alphabetically, assign A/B/C letters, add 8-char right-aligned columns after Cost, add per-machine sums in Total row, append legend line in `dim`. Skip in compact mode
- [x] T006 Add machine column rendering to `renderTotal()` in `src/node/tui/formatter.ts` — same letter assignment and column width as renderHistory, per-tool machine costs from machineMap, Total row with per-machine sums, legend line. Skip in compact mode

## Phase 3: Integration & Edge Cases

- [x] T007 Thread `byMachineFlag` through dispatch functions in `src/node/core/cli.ts` — `dispatchAllSnapshot`, `dispatchSingleTool`, and their `*Lines` variants. When flag is active and mode is compatible: call `fetchToolMergedWithMachines`, build per-row/per-tool `machineMap`, pass via `FormatOptions`. For `dispatchAllHistory`: warn on stderr and ignore flag
- [x] T008 Thread `byMachineFlag` into JSON output in `src/node/core/cli.ts` — when `--by-machine` and `--json` are both active, attach `machines` key (machine name → cost number) to each tool/entry in the JSON output
- [x] T009 Thread `byMachineFlag` through watch mode in `src/node/core/cli.ts` — pass flag into the watch action closure so machine columns refresh on each poll

## Phase 4: Polish

- [x] T010 Add tests in `src/node/sync/__tests__/sync-by-machine.test.ts` — test `readRemoteEntriesByMachine` with multi-machine fixture dirs, exclude logic, empty dir
- [x] T011 Add tests in `src/node/core/__tests__/cli-by-machine.test.ts` — test `parseGlobalFlags` recognizes `--by-machine`, test incompatible pivot warning

---

## Execution Order

- T001 and T002 are independent (parallel)
- T003 depends on T002 (needs `readRemoteEntriesByMachine`)
- T004 is independent of T003 (parallel with T003)
- T005 and T006 depend on T004 (need `machineMap` in FormatOptions)
- T007 depends on T003, T005, T006 (wires fetch + format together)
- T008 and T009 depend on T007 (extend dispatch with JSON and watch threading)
- T010 and T011 are independent (parallel), can start after T002 and T001 respectively
