# Tasks: Filter Usage by User

**Change**: 260307-v2bu-filter-usage-by-user
**Spec**: `spec.md`
**Intake**: `intake.md`

## Phase 1: Foundation

- [x] T001 [P] Refactor `readRemoteEntries()` in `src/node/sync/sync.ts` — change signature to `(metricsDir, targetUser, excludeMachine: string | null, toolKey)`, replace all-users iteration with single-user directory read, skip `excludeMachine` when non-null
- [x] T002 [P] Add `-u`/`--user` flag parsing to `parseGlobalFlags` in `src/node/core/cli.ts` — add `userFlag: string | undefined` to `GlobalFlags` interface, extract value from `-u`/`--user`, error and exit if flag present with no value

## Phase 2: Core Implementation

- [x] T003 Modify `fetchToolMerged()` in `src/node/core/cli.ts` — add optional `targetUser?: string` parameter; when set, skip `fetchHistory`/`writeMetrics` and call `readRemoteEntries(metricsDir, targetUser, null, toolKey)` directly; apply `aggregateMonthly` if period is monthly
- [x] T004 Thread `userFlag` through dispatch functions in `src/node/core/cli.ts` — add `targetUser` parameter to `dispatchAllHistory`, `dispatchAllSnapshot`, `dispatchSingleTool` and their `*Lines` watch-mode variants; pass through to `fetchToolMerged`
- [x] T005 Wire `-u` in `main()` in `src/node/core/cli.ts` — after config load, if `userFlag` set and mode is not multi, warn on stderr and clear; pass `userFlag` to dispatch functions

## Phase 3: Integration & Polish

- [x] T006 Update `FULL_HELP` in `src/node/core/cli.ts` — add `--user / -u <user>` line to Flags section with description for multi-mode user filtering
- [x] T007 Update tests in `src/node/sync/__tests__/sync.test.ts` — change "reads entries from other users" to verify other users are NOT returned, add test for `excludeMachine=null` (reads all machines), update "reads from multiple users and machines" to test single-user multi-machine

---

## Execution Order

- T001 and T002 are independent, can run in parallel
- T003 depends on T001 (uses new `readRemoteEntries` signature)
- T004 depends on T003 (passes `targetUser` to modified `fetchToolMerged`)
- T005 depends on T002 and T004 (uses `userFlag` from parsing + threaded dispatch)
- T006 is independent, can run alongside any task
- T007 depends on T001 (tests the new `readRemoteEntries` behavior)
