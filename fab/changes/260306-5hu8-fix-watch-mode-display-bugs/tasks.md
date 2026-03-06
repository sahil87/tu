# Tasks: Fix Watch Mode Display Bugs

**Change**: 260306-5hu8-fix-watch-mode-display-bugs
**Spec**: `spec.md`
**Intake**: `intake.md`

## Phase 1: Core Implementation

- [x] T001 [P] Add `pollHistory.length > 1` guard to Tokens/min stat in `src/node/tui/panel.ts` — change the condition at line 81 from `if (elapsedMin > 0 && session.totalTokens > 0)` to `if (session.pollHistory.length > 1 && session.totalTokens > 0)`
- [x] T002 [P] Guard Total row in `renderTotal()` in `src/node/tui/formatter.ts` — count visible rows (tools with `totalTokens > 0`) before lines 190-191 and only render divider + Total row when count > 1
- [x] T003 [P] Guard Total row in `renderCompactSnapshot()` in `src/node/tui/formatter.ts` — count visible rows and only render divider + Total row when count > 1 (lines 324-326)

## Phase 2: Tests

- [x] T004 [P] Add/update tests in `src/node/tui/__tests__/panel.test.ts` — verify Tokens/min is suppressed with `pollHistory.length <= 1` and shown with `pollHistory.length >= 2`
- [x] T005 [P] Add/update tests in `src/node/tui/__tests__/formatter.test.ts` — verify `renderTotal()` omits Total row with single visible tool, shows it with multiple
- [x] T006 [P] Add/update tests in `src/node/tui/__tests__/formatter.test.ts` — verify `renderCompactSnapshot()` omits Total row with single visible tool, shows it with multiple

---

## Execution Order

- T001, T002, T003 are independent (different functions/files), can run in parallel
- T004, T005, T006 are independent, can run in parallel after Phase 1
