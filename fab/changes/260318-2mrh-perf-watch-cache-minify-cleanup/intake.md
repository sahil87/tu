# Intake: Performance — Watch Cache, Monthly Aggregation, Minify, EMPTY Cleanup

**Change**: 260318-2mrh-perf-watch-cache-minify-cleanup
**Created**: 2026-03-18
**Status**: Draft

## Origin

> Performance exploration via `/fab-discuss` with a team of four parallel agents analyzing the codebase for optimization opportunities. The user reviewed the findings and selected four items for implementation: watch-mode metrics caching, single-pass monthly aggregation, esbuild minification, and EMPTY object cloning removal.

Conversational mode — user reviewed a ranked list of ~10 findings, accepted 4, explicitly rejected one (write-then-read round-trip) citing the "keep minimum pathways" principle.

## Why

1. **Watch mode I/O overhead**: In multi-machine mode, every 10-second poll cycle calls `readRemoteEntriesByMachine()` which traverses the entire metrics directory tree with `readdirSync()` + `readFileSync()` for every JSONL file across all years and machines. The existing 60-second cache in `fetcher.ts` only covers external tool fetches (ccusage binaries) — metrics directory reads have no caching at all. For users with months of history across multiple machines, this is unnecessary repeated filesystem work when files haven't changed.

2. **Double monthly aggregation**: When `--by-machine` is combined with monthly period, `fetchToolMergedWithMachines()` calls `aggregateMonthly(merged)` on the combined entries AND `aggregateMonthly(mEntries)` for each machine separately. The per-machine aggregation iterates over the same daily entries that were already processed in the merged aggregation. This happens at cli.ts:424-427 and cli.ts:450-453.

3. **Bundle size**: The esbuild command has no `--minify` flag. The production bundle includes all whitespace, full variable names, and comments. Adding minification is a one-line change for a meaningful size reduction.

4. **Unnecessary object allocations**: The `EMPTY` constant in `fetcher.ts:61-68` is spread (`{ ...EMPTY }`) at ~13 call sites across `fetcher.ts` and `cli.ts` as a null-coalescing fallback. Since `EMPTY` is never mutated anywhere in the codebase, these spreads create unnecessary object allocations on every call. The pattern appears in hot paths like `pickCurrentEntry()`, `fetchTotals()`, and every `dispatchAll*`/`dispatchSingleTool` snapshot branch.

## What Changes

### 1. Metrics directory read caching for watch mode

Add an mtime-based cache layer for `readRemoteEntriesByMachine()` and `readRemoteEntries()` in `src/node/sync/sync.ts`. The cache should:

- Store the last result of `readRemoteEntriesByMachine()` keyed by `(metricsDir, targetUser, excludeMachine, toolKey)`
- On subsequent calls, check the mtime of the metrics user directory (`join(metricsDir, targetUser)`)
- If the directory mtime hasn't changed since the last read, return the cached result
- If mtime has changed, perform the full directory scan and update the cache
- Expose a `clearMetricsCache()` function so callers can invalidate after `writeMetrics()` or `syncMetrics()` (these are the only two operations that modify the metrics directory)

Call `clearMetricsCache()` from:
- `writeMetrics()` after writing files (sync.ts:28-35)
- `syncMetrics()` after the git pull/push cycle completes (sync.ts:67)

This primarily benefits watch mode where `doPoll()` triggers full directory rescans every 10 seconds even when no new data has been written.

### 2. Single-pass monthly aggregation for `--by-machine`

In `fetchToolMergedWithMachines()` (cli.ts:409-457), the monthly aggregation block at lines 423-427 and 449-453 currently:

```typescript
const monthlyEntries = aggregateMonthly(merged);
const monthlyMap = new Map<string, UsageEntry[]>();
for (const [machine, mEntries] of machineMap) monthlyMap.set(machine, aggregateMonthly(mEntries));
```

Replace with a single-pass approach that aggregates per-machine monthly entries and derives the merged monthly entries by summing across machines, avoiding the redundant `aggregateMonthly(merged)` call. The merged monthly result can be computed by iterating the per-machine monthly maps and summing entries by label — which is what `mergeEntries()` already does. So the pattern becomes:

```typescript
const monthlyMap = new Map<string, UsageEntry[]>();
for (const [machine, mEntries] of machineMap) monthlyMap.set(machine, aggregateMonthly(mEntries));
const allMonthly: UsageEntry[] = [];
for (const mEntries of monthlyMap.values()) allMonthly.push(...mEntries);
const monthlyEntries = mergeEntries(allMonthly, []);
```

This eliminates one full `aggregateMonthly()` pass over all merged entries. Both blocks (lines 423-427 and 449-453) need the same fix.

### 3. Add `--minify` to esbuild build config

Add the `--minify` flag to both build commands:
- `package.json` scripts.build (line 10)
- `justfile` build recipe (line 10)

Both should become:
```
esbuild src/node/core/cli.ts --bundle --platform=node --format=esm --minify --outfile=dist/tu.mjs --banner:js='#!/usr/bin/env node'
```

### 4. Remove unnecessary `{ ...EMPTY }` cloning

Replace `{ ...EMPTY }` with `EMPTY` at all call sites where the result is used as a read-only fallback value. The affected locations:

In `src/node/core/fetcher.ts`:
- `pickCurrentEntry()` line 149: `return match ? toUsageTotals(match) : { ...EMPTY };`
- `fetchTotals()` line 176: `if (!parsed) return { ...EMPTY };`
- `fetchTotals()` line 179: `if (!dailyRaw || dailyRaw.length === 0) return { ...EMPTY };`

In `src/node/core/cli.ts` (all in dispatch functions):
- `dispatchAllSnapshot()` line 631: `result.set(TOOLS[toolKeys[i]].name, current ?? { ...EMPTY });`
- `dispatchAllSnapshot()` line 647: `result.set(TOOLS[toolKeys[i]].name, current ?? { ...EMPTY });`
- `dispatchAllSnapshot()` line 662: `result.set(TOOLS[toolKeys[i]].name, match ?? { ...EMPTY });`
- `dispatchSingleTool()` line 704: `result.set(toolCfg.name, current ?? { ...EMPTY });`
- `dispatchSingleTool()` line 739: `result.set(toolCfg.name, current ?? { ...EMPTY });`
- Plus any watch-mode dispatch line variants

Safety: `EMPTY` is defined as a `const` at module scope and is typed `UsageTotals`. No code path mutates the returned value — it flows into `printTotal()`, `emitJson()`, or cost-sum functions which only read properties.

## Affected Memory

- `watch-mode/tui`: (modify) Add note about metrics caching for poll cycles
- `build/toolchain`: (modify) Update esbuild command to include `--minify`

## Impact

- **src/node/sync/sync.ts**: New cache layer for `readRemoteEntriesByMachine()`, `clearMetricsCache()` export
- **src/node/core/cli.ts**: Refactored monthly aggregation in `fetchToolMergedWithMachines()`, EMPTY spread removal across dispatch functions
- **src/node/core/fetcher.ts**: EMPTY spread removal in `pickCurrentEntry()`, `fetchTotals()`
- **package.json**: `--minify` added to build script
- **justfile**: `--minify` added to build recipe
- **dist/tu.mjs**: Minified output (smaller, less readable — debug via `TU_DEBUG=1` unaffected since stderr writes are runtime)

## Open Questions

- None — all four changes are well-scoped with clear implementation paths.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Use directory mtime for metrics cache invalidation | Discussed — simplest signal; `writeMetrics()` and `syncMetrics()` are the only writers, both can call `clearMetricsCache()` | S:90 R:85 A:90 D:90 |
| 2 | Certain | Derive merged monthly from per-machine monthly via `mergeEntries()` | Discussed — mathematically equivalent to aggregating merged daily, reuses existing function | S:85 R:90 A:85 D:95 |
| 3 | Certain | Add `--minify` without `--sourcemap` | esbuild minification is standard; no sourcemaps needed since debug mode uses stderr writes, not stack traces | S:90 R:95 A:90 D:90 |
| 4 | Certain | Replace `{ ...EMPTY }` with `EMPTY` reference | Discussed — EMPTY is a module-level const, never mutated; all consumers are read-only | S:90 R:90 A:95 D:95 |
| 5 | Confident | Cache keyed by `(metricsDir, targetUser, excludeMachine, toolKey)` tuple | Full parameter set ensures no cross-contamination; could over-cache if same user called with different excludeMachine values, but in practice the same excludeMachine is used consistently per session | S:75 R:80 A:80 D:75 |
| 6 | Confident | `clearMetricsCache()` clears entire cache, not per-key | Simpler implementation; watch mode always writes+reads for the same user, so selective invalidation adds complexity without benefit | S:70 R:85 A:80 D:80 |
| 7 | Certain | Change type is `refactor` — no new features, no bug fixes, pure optimization | All four items improve performance of existing behavior without changing observable output | S:95 R:95 A:95 D:95 |

7 assumptions (5 certain, 2 confident, 0 tentative, 0 unresolved).
