# Intake: Fix Watch Mode Display Bugs

**Change**: 260306-5hu8-fix-watch-mode-display-bugs
**Created**: 2026-03-06
**Status**: Draft

## Origin

> Fix watch mode bugs: (1) Tokens/min shows absurd value (~3 trillion) at 0s elapsed due to divide-by-near-zero — rate stats should be suppressed until at least one full poll interval has elapsed. (2) Total row displays when only one tool has data, violating the spec that says "Total row shown only when >1 tool has data."

Identified during a `/fab-discuss` session reviewing a live screenshot of watch mode. Both bugs were visually confirmed against the layouts spec.

## Why

Two visual bugs undermine trust in the watch mode display:

1. **Tokens/min divide-by-near-zero**: On the first render (0s elapsed), `elapsedMin` is a tiny fraction (e.g., 0.00005 minutes), producing absurd rates like `~3,065,198,700,000 tokens/min`. This makes the session stats panel look broken on every watch mode launch. The value stabilizes after a few seconds but the first impression is jarring.

2. **Total row with single tool**: The snapshot layout (`renderTotal` in `src/node/tui/formatter.ts`) unconditionally renders a Total row + divider, even when only one tool has data. The layouts spec explicitly states "Total row shown only when >1 tool has data" (layouts.md line 27). The history (`renderHistory`) and compact (`renderCompactSnapshot`) layouts already guard with `entries.length > 1`, but the snapshot layout was missed.

If unfixed, every user sees garbage stats on watch mode launch, and single-tool users see a redundant Total row that just duplicates the data row above it.

## What Changes

### 1. Suppress rate-based stats until meaningful elapsed time

In `src/node/tui/panel.ts`, the guard at line 81 (`if (elapsedMin > 0 && session.totalTokens > 0)`) is insufficient — any non-zero millisecond produces a truthy `elapsedMin`. The fix: require at least one full poll interval to have elapsed before showing Tokens/min. This aligns with Session cost delta which already guards on `pollHistory.length > 1`.

The simplest approach: change the Tokens/min guard to also require `session.pollHistory.length > 1` (i.e., at least two polls have completed). This is consistent with the Session cost delta guard on line 72 and ensures the rate is computed from actual observed change, not a near-zero time delta.

### 2. Guard Total row in snapshot layout

In `src/node/tui/formatter.ts`, `renderTotal()` (lines 190-191) unconditionally renders the divider and Total row. Add a guard: only render when more than one tool has non-zero data. Count visible rows (tools with `totalTokens > 0`) and only show Total when count > 1.

This matches the existing pattern in:
- `renderHistory` line 127: `if (entries.length > 1)`
- `renderTotalHistory` line 294: `if (labels.length > 1)`
- `renderCompactSnapshot` line 340: `if (entries.length > 1)`
- `renderCompactTotalHistory` line 357: `if (labels.length > 1)`

## Affected Memory

- `watch-mode/tui`: (modify) Update session stats requirements to clarify minimum elapsed time before showing rate stats
- `display/formatting`: (modify) Clarify Total row guard applies to all layout variants including snapshot

## Impact

- **`src/node/tui/panel.ts`**: Session stats rendering logic (Tokens/min guard)
- **`src/node/tui/formatter.ts`**: `renderTotal()` function (Total row guard)
- **Tests**: Existing tests in `src/node/tui/__tests__/panel.test.ts` and `src/node/tui/__tests__/formatter.test.ts` may need updates for the new guards
- **No API/config/CLI changes** — purely display logic fixes

## Open Questions

None — both bugs have clear fixes grounded in existing patterns and spec language.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Use `pollHistory.length > 1` guard for Tokens/min | Matches existing Session cost delta guard on line 72; consistent pattern | S:90 R:95 A:95 D:95 |
| 2 | Certain | Count visible rows (non-zero tools) for Total guard | Matches existing guards in renderHistory, renderTotalHistory, renderCompactSnapshot | S:95 R:95 A:95 D:95 |
| 3 | Certain | Test runner is `npx tsx --test` | Constitution specifies this | S:95 R:90 A:95 D:95 |
| 4 | Certain | No changes to CLI interface or config | Pure display bug fixes, no user-facing behavior changes beyond correctness | S:90 R:95 A:90 D:95 |
| 5 | Confident | No need to suppress Rate/Proj. day separately — `computeBurnRate` already returns null with < 2 polls | Discussed — confirmed Rate guard at line 87 depends on `computeBurnRate` which needs poll history; but should verify | S:80 R:85 A:70 D:85 |

5 assumptions (4 certain, 1 confident, 0 tentative, 0 unresolved).
