# Quality Checklist: Redesign Watch Layout

**Change**: 260306-mxla-redesign-watch-layout
**Generated**: 2026-03-06
**Spec**: `spec.md`

## Functional Completeness

- [ ] CHK-001 Stats grid layout: 2x3 grid renders above table with correct column grouping (session left, cost right)
- [ ] CHK-002 Stats grid separator: dim horizontal rule appears between stats grid and table title
- [ ] CHK-003 Sparkline removal: `sparkline.ts` deleted, no braille chart rendering in output
- [ ] CHK-004 Side-by-side merge removal: `mergeSideBySide()` deleted, table renders at full width
- [ ] CHK-005 Rain tick interval: `RAIN_TICK_MS` is 107 (not 80)
- [ ] CHK-006 Loading skeleton: skeleton renders on alt-screen entry before first fetch
- [ ] CHK-007 Two-tier breakpoints: full mode >= 60 cols, compact < 60 cols, no 113-col threshold

## Behavioral Correctness

- [ ] CHK-008 Unavailable stats show `--` placeholder: grid stays fixed 3 rows when Rate/Tok/min/Proj.day unavailable
- [ ] CHK-009 Stats populate progressively: after 2+ polls, all stats show real values
- [ ] CHK-010 Table output identical to non-watch mode: same render functions, no merge transformation
- [ ] CHK-011 Rain animation still works: fills space below content, right-margin fallback operational

## Removal Verification

- [ ] CHK-012 `src/node/tui/sparkline.ts` does not exist
- [ ] CHK-013 `src/node/tui/__tests__/sparkline.test.ts` does not exist
- [ ] CHK-014 No remaining imports of `sparkline` or `renderSparkline` in the codebase
- [ ] CHK-015 No remaining references to `mergeSideBySide` in the codebase (except test file removal)
- [ ] CHK-016 No remaining references to `HISTORY_CACHE_TTL` or `refreshHistoryCache` or `fetchHistory` option
- [ ] CHK-017 `SparkPanel` class removed from compositor
- [ ] CHK-018 `MIN_TABLE_WIDTH`, `MIN_PANEL`, `MERGE_GUTTER` constants removed from compositor

## Scenario Coverage

- [ ] CHK-019 Full stats grid with all data available (2+ polls)
- [ ] CHK-020 Stats before 2 polls (dashes for unavailable)
- [ ] CHK-021 Loading skeleton appearance
- [ ] CHK-022 Full mode on 80-col terminal (formerly "medium" — now shows stats grid + rain)
- [ ] CHK-023 Compact mode on narrow terminal (< 60 cols, no stats, no rain)

## Edge Cases & Error Handling

- [ ] CHK-024 Zero elapsed time: Elapsed shows `0s`, no division errors in Tok/min
- [ ] CHK-025 First poll only: Session delta shows `--`, Rate shows `--`, Proj. day shows `--`
- [ ] CHK-026 Terminal resize triggers re-layout with new breakpoint system

## Code Quality

- [ ] CHK-027 Pattern consistency: new code follows naming and structural patterns of surrounding code
- [ ] CHK-028 No unnecessary duplication: existing utilities (`formatElapsed`, `computeBurnRate`, color helpers) reused
- [ ] CHK-029 Readability: stats grid rendering logic is clear and maintainable
- [ ] CHK-030 No god functions: `buildStatsGrid()` stays focused, compositor changes don't bloat `layoutAndUpdate()`
- [ ] CHK-031 No magic strings/numbers: grid column widths, separator characters use named constants or are self-documenting

## Notes

- Check items as you review: `- [x]`
- All items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] CHK-008 **N/A**: {reason}`
