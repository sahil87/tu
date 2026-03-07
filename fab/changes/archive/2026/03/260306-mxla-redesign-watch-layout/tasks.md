# Tasks: Redesign Watch Layout

**Change**: 260306-mxla-redesign-watch-layout
**Spec**: `spec.md`
**Intake**: `intake.md`

## Phase 1: Removal

<!-- Remove sparkline, side panel, and merge infrastructure. Order: sparkline first (leaf), then panel refs, then compositor merge. -->

- [x] T001 Delete `src/node/tui/sparkline.ts` and `src/node/tui/__tests__/sparkline.test.ts`
- [x] T002 Remove sparkline import and rendering from `src/node/tui/panel.ts` — delete `import { renderSparkline }` and the sparkline block in `buildPanel()`
- [x] T003 Remove `SparkPanel` class from `src/node/tui/compositor.ts`. Remove `mergeSideBySide()` export and function. Remove `MERGE_GUTTER`, `MIN_TABLE_WIDTH`, `MIN_PANEL` constants. Remove `lastShowPanel` field and all showPanel logic in `layoutAndUpdate()` and `flush()`
- [x] T004 Remove sparkline history cache from `src/node/tui/watch.ts` — delete `historyCache`, `historyCacheTime`, `refreshHistoryCache()`, `HISTORY_CACHE_TTL`, the `fetchHistory` option from `WatchOptions`, and both `refreshHistoryCache()` calls. Remove `mergeSideBySide` re-export. Remove `history` parameter from `compositor.updateAfterPoll()` call

## Phase 2: Core Implementation

<!-- Build the new stats grid and integrate it into the compositor. -->

- [x] T005 Rewrite `buildPanel()` in `src/node/tui/panel.ts` as `buildStatsGrid()`: render a fixed 3-row, 2-column grid. Left column: Elapsed, Session delta. Right column: Tok/min, Rate, Proj. day. Unavailable stats show `--`. Append dim `───` separator as final line. Remove `PanelSession` interface's dependency on `UsageEntry[]` history parameter. Keep `formatElapsed`, `computeBurnRate`, `PanelSession`, and `fmtDollar` exports unchanged
- [x] T006 Add `StatsPanel` class to `src/node/tui/compositor.ts` replacing `SparkPanel` — wraps `buildStatsGrid()`, implements `PanelBuffer` with dirty flag. Update `Compositor` class: replace `spark` field with `stats` field. Update `updateAfterPoll()` signature: remove `history` parameter, add stats panel update. Update `flush()`: render stats lines above table lines (concatenate stats + table), no side-by-side merge
- [x] T007 Change `RAIN_TICK_MS` from `80` to `107` in `src/node/tui/compositor.ts`
- [x] T008 Add loading skeleton to `src/node/tui/watch.ts`: after `enterAltScreen()` and before `compositor.start()`, render a skeleton frame — stats grid with zeros/dashes + dim separator + table header + centered dim "Loading..." line. Use `compositor.flush()` or direct stdout write

## Phase 3: Integration & Breakpoints

<!-- Wire up the simplified breakpoint system and update the compositor layout logic. -->

- [x] T009 Simplify breakpoint logic in `src/node/tui/compositor.ts` `layoutAndUpdate()`: remove the 113-col wide/medium distinction. Full mode (>= 60): render stats grid + table + rain. Compact mode (< 60): table only, no stats, no rain. Remove `computeMaxContentWidth()` method if no longer needed (rain zone now just uses full terminal width below content)
- [x] T010 Update `src/node/tui/watch.ts` `doPoll()`: remove `historyCache` argument from `compositor.updateAfterPoll()`. Ensure `PanelSession` is still built and passed for stats grid

## Phase 4: Tests & Cleanup

- [x] T011 [P] Rewrite `src/node/tui/__tests__/panel.test.ts`: test `buildStatsGrid()` — full stats, partial stats (dashes), first poll, burn rate, elapsed formatting. Remove sparkline-related assertions
- [x] T012 [P] Update `src/node/tui/__tests__/watch.test.ts`: remove `mergeSideBySide` tests. Remove `HISTORY_CACHE_TTL` import if no longer exported. Keep `formatElapsed`, `computeBurnRate`, `buildFooter` tests
- [x] T013 [P] Remove or update any remaining imports/references to deleted symbols across the codebase (sparkline, mergeSideBySide, HISTORY_CACHE_TTL, fetchHistory)

---

## Execution Order

- T001 → T002 → T003 → T004 (sequential: remove leaf first, then references)
- T005 blocks T006 (stats grid function needed before compositor integration)
- T006 blocks T008 (skeleton uses compositor)
- T006 blocks T009 (breakpoint logic depends on new compositor structure)
- T007 is independent, can run alongside T005-T006
- T010 depends on T004 + T006
- T011, T012, T013 are parallel, depend on all Phase 2-3 tasks
