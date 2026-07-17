# Plan: Watch-Mode Rain Polish — Cursor, Skeleton Rain, Flush Blink, Tick Removal, Gutter

**Change**: 260717-h9h7-watch-rain-polish
**Intake**: `intake.md`

## Requirements

### Watch Mode: Cursor Visibility

#### R1: Hide the terminal cursor for the watch session
The watch session SHALL hide the terminal cursor on alt-screen entry and MUST restore it on exit, so the visible cursor never hops around the rain zone or the footer during a session.

- **GIVEN** the user starts watch mode
- **WHEN** the alternate screen is entered (`enterAltScreen`)
- **THEN** a hide-cursor sequence (`\x1b[?25l`) is written after `\x1b[?1049h`
- **AND** on exit (`exitAltScreen`, reached via both `q` and SIGINT cleanup) a show-cursor sequence (`\x1b[?25h`) is written before `\x1b[?1049l`

### Watch Mode: Skeleton Rain

#### R2: Rain animates alongside the loading skeleton before the first poll
Rain SHALL animate in the zone below the loading skeleton from the first rain tick, including while the first fetch is slow or repeatedly failing — satisfying the documented requirement that "Rain starts immediately alongside" the skeleton.

- **GIVEN** watch mode has entered the alt screen and rendered the loading skeleton, but no poll has yet succeeded
- **WHEN** the rain timer fires
- **THEN** the rain layer is already set up against the skeleton's content geometry (content height + max content width) and produces visible animated output
- **AND** the first successful `updateAfterPoll()` re-lays-out with the real table dimensions (a no-op resize when dimensions are unchanged)

### Watch Mode: Poll-Flush Blink

#### R3: Rain redraws immediately after every flush
Rain MUST be redrawn immediately after each compositor flush so the `\x1b[K`/`\x1b[J` clears performed by `flush()` do not leave the rain zone blank until the next rain tick (eliminating the per-poll blink).

- **GIVEN** a poll completes and `Compositor.flush()` writes content, clears trailing lines with `\x1b[J`, and writes the footer
- **WHEN** the flush finishes
- **THEN** the current rain frame is re-emitted (via `rainLayer.renderDirect()`) so every occupied rain cell is restored in the same flush, with no visible gap

### Watch Mode: Push-Driven Footer (Tick Removal)

#### R4: The 16ms compositor tick is removed; the footer is push-driven
The compositor SHALL NOT run a periodic 16ms interval. The footer status line MUST instead be re-rendered synchronously whenever the countdown state changes, reducing idle event-loop wakeups while preserving footer behavior.

- **GIVEN** the compositor is running
- **WHEN** the countdown value changes (initial set, per-second tick-down) or the refreshing state is set
- **THEN** the footer status line is written immediately (push-driven), with no polling interval
- **AND** no `setInterval` for a compositor tick exists; `start()`/`stop()` manage only the rain and countdown timers

### Watch Mode: Right-Margin Rain Gutter

#### R5: Right-margin rain keeps a 2-column gutter from the content
In right-margin rain mode, rain columns MUST start `RAIN_GUTTER` (2) columns to the right of the widest content line, and the minimum-columns check MUST apply to the post-gutter width so rain disables itself when fewer than the minimum columns remain.

- **GIVEN** watch mode is in right-margin rain mode (no rows below content, columns available to the right)
- **WHEN** the rain zone is computed
- **THEN** the rain start column is `maxContentWidth + RAIN_GUTTER` and the rain width is `termWidth - maxContentWidth - RAIN_GUTTER`
- **AND** rain is disabled when the post-gutter width is below `MIN_RAIN_COLS` (10)

### Design Decisions

1. **Cursor hide/show inside `enterAltScreen`/`exitAltScreen`**: place the sequences at the single alt-screen seam — *Why*: every existing call site (including SIGINT cleanup) is already routed through `exitAltScreen`, so no new call sites are needed — *Rejected*: hiding/showing at each write site (error-prone, scattered).
2. **Skeleton rain via a shared `setupRainZone` helper + `layoutForSkeleton(lines)`**: extract the rain-zone computation from `layoutAndUpdate` into a private helper, add a public `layoutForSkeleton` that derives geometry from the skeleton lines; `renderSkeleton` returns its `string[]` — *Why*: minimal-diff reuse of the existing layout logic, no duplication — *Rejected*: duplicating the zone math in a second code path.
3. **Poll blink fixed by appending `rainLayer.renderDirect()` at end of `flush()`**: relies on `RainState.render()` rewriting every occupied cell each frame — *Why*: one-line restoration of the full rain frame after `\x1b[J`, no renderer change — *Rejected*: suppressing `\x1b[J`/`\x1b[K` (would leave stale content lines).
4. **Footer push-driven; `PanelBuffer`/`dirty` interface left intact**: delete the interval and call a new `renderStatus()` after each `updateCountdown` — *Why*: deletes code and idle wakeups while keeping the diff minimal; the `status.dirty` flag becomes vestigial but harmless — *Rejected*: lengthening the interval (still wakes the loop for no benefit).

## Tasks

### Phase 2: Core Implementation

- [x] T001 Hide/show the terminal cursor in `src/node/tui/watch.ts`: in `enterAltScreen()` write `\x1b[?25l` after `\x1b[?1049h`; in `exitAltScreen()` write `\x1b[?25h` before `\x1b[?1049l`. <!-- R1 -->
- [x] T002 Extract rain-zone computation in `src/node/tui/compositor.ts`: move the `layoutAndUpdate` rain-zone block into a private `setupRainZone(contentHeight, maxContentWidth)` helper and have `layoutAndUpdate` call it with its computed values (behavior-preserving refactor). <!-- R2 -->
- [x] T003 Add `RAIN_GUTTER = 2` module-level constant (next to the existing `MIN_RAIN_COLS` usage) in `src/node/tui/compositor.ts` and apply the gutter in the right-margin branch of `setupRainZone`: `startCol = maxContentWidth + RAIN_GUTTER`, `marginCols = tw - maxContentWidth - RAIN_GUTTER`, `MIN_RAIN_COLS` check on the post-gutter width. <!-- R5 -->
- [x] T004 Add public `layoutForSkeleton(lines: string[])` to `Compositor` in `src/node/tui/compositor.ts`: derive `contentHeight = lines.length` and `maxContentWidth` (via `stripAnsi`, same as `computeMaxContentWidth`) and call `setupRainZone(...)`. <!-- R2 -->
- [x] T005 Make `renderSkeleton` return `string[]` in `src/node/tui/watch.ts`: build the same content it writes today into an array, write it, and return it; in `runWatch` capture the returned lines and call `compositor.layoutForSkeleton(lines)` before `compositor.start()`. <!-- R2 -->
- [x] T006 Redraw rain after every flush in `src/node/tui/compositor.ts`: at the end of `Compositor.flush()`, after the footer write, append `const rainOut = this.rainLayer.renderDirect(); if (rainOut) process.stdout.write(rainOut);`. <!-- R3 -->
- [x] T007 Remove the 16ms compositor tick in `src/node/tui/compositor.ts`: delete `COMPOSITOR_TICK_MS`, the `compositorTimer` field, its `setInterval` in `start()`, its `clearInterval` in `stop()`, and the private `tick()` method; add a private `renderStatus()` that renders the footer unconditionally and call it after every `status.updateCountdown(...)` (both sites in `startCountdown` and in `setRefreshing`). <!-- R4 -->

### Phase 3: Tests

- [x] T008 [P] Add compositor test coverage in `src/node/tui/__tests__/compositor.test.ts`: right-margin gutter arithmetic (start column and post-gutter min-cols disable), skeleton rain enabled via `layoutForSkeleton`, and rain redraw appended by `flush()`, driving the compositor's public surface with stubbed term dimensions. <!-- R2 R3 R5 -->
- [x] T009 [P] Add cursor hide/show coverage in `src/node/tui/__tests__/watch.test.ts` (or capture within compositor test): assert the hide/show sequences are emitted by the exported enter/exit helpers, capturing stdout writes. <!-- R1 -->

## Execution Order

- T002 blocks T003 and T004 (both operate on the extracted `setupRainZone` helper).
- T004 blocks T005 (`layoutForSkeleton` must exist before `runWatch` calls it).
- T008/T009 depend on the implementation tasks (T001-T007) being complete.

## Acceptance

### Functional Completeness

- [x] A-001 R1: `enterAltScreen` emits `\x1b[?25l` (after `\x1b[?1049h`) and `exitAltScreen` emits `\x1b[?25h` (before `\x1b[?1049l`); cursor is restored on both `q` and SIGINT paths (both route through `exitAltScreen`).
- [x] A-002 R2: A shared `setupRainZone` helper exists and both `layoutAndUpdate` and the new `layoutForSkeleton(lines)` use it; `runWatch` lays out from skeleton lines before `compositor.start()`, so rain is enabled before the first poll.
- [x] A-003 R3: `Compositor.flush()` re-emits the current rain frame (`rainLayer.renderDirect()`) after the footer write, so no rain-zone blank persists past a flush.
- [x] A-004 R4: No compositor `setInterval` remains; footer is written synchronously on every countdown/refresh state change via a `renderStatus()` helper; `start()`/`stop()` manage only rain + countdown timers.
- [x] A-005 R5: Right-margin rain starts at `maxContentWidth + 2` with width `termWidth - maxContentWidth - 2`, and disables when the post-gutter width `< MIN_RAIN_COLS`.

### Behavioral Correctness

- [x] A-006 R4: Footer content is unchanged from the pre-change output (countdown text, `Refreshing...`, key hints, progressive truncation); only its render trigger changed from interval-polled to push-driven.
- [x] A-007 R2: The first `updateAfterPoll()` re-layout is a no-op resize when skeleton and table geometry match, and correctly resizes when they differ (no crash, rain continues).

### Scenario Coverage

- [x] A-008 R2 R3 R5: Compositor tests exercise skeleton-rain enablement, post-flush rain redraw, and right-margin gutter arithmetic (including the min-cols disable boundary) via the public surface.
- [x] A-009 R1: A test asserts the cursor hide/show sequences are emitted by the enter/exit helpers.

### Code Quality

- [x] A-010 Pattern consistency: New code follows the surrounding TUI style — functions and plain objects (no new classes beyond the existing panel structure), `node:` built-in imports where applicable, `type` imports for type-only values, named constants (no magic strings/numbers — `RAIN_GUTTER`).
- [x] A-011 No unnecessary duplication: The rain-zone computation is shared via `setupRainZone` rather than duplicated across `layoutAndUpdate` and `layoutForSkeleton`; `maxContentWidth` derivation reuses the existing `stripAnsi`/`computeMaxContentWidth` approach.
- [x] A-012 No silent error-swallowing: No new error paths introduced; watch-mode graceful-degradation behavior (fetch-failure retry) is untouched.
- [x] A-013 Output stability: No changes to table layout, JSON, or non-watch output; changes are watch-mode-only and cosmetic/efficiency (Output Stability constraint honored).

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Deletion Candidates

- `PanelBuffer.dirty` + the three panel `dirty` fields (`src/node/tui/compositor.ts:13`, `:34/:40/:44` TablePanel, `:51/:57/:61` StatsPanel, `:72/:81` StatusPanel) — the removed 16ms `tick()` was the only reader; the dirty-flag machinery is now write-only (kept vestigial per Design Decision 4 — follow-up cleanup candidate).
- `StatusPanel.getContent()` (`src/node/tui/compositor.ts:107-109`) — zero call sites repo-wide; pre-existing dead code discovered in this change's area.
- `buildFooter()` legacy export (`src/node/tui/watch.ts:271-293`) — duplicates `StatusPanel.render()` truncation logic and exists only for its own backward-compat tests; deletable together with its `watch.test.ts` describe block.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Scope is exactly the 5 "What Changes" items (cursor, skeleton rain, flush redraw, tick removal, gutter); deferred discussion items 5-8 (backpressure, color batching, allocation churn, per-cell glyphs/frame diffing) are excluded | Intake states user explicitly selected these and deferred the rest | S:95 R:90 A:95 D:95 |
| 2 | Certain | Cursor hide/show implemented inside `enterAltScreen`/`exitAltScreen` so all call sites incl. SIGINT cleanup are covered | Single obvious seam; trivially reversible; intake specifies exact sequences and line anchors | S:90 R:95 A:95 D:95 |
| 3 | Confident | `renderStatus()` renders the footer unconditionally (mirrors what `tick()` did but without the `dirty` guard), called after each `updateCountdown`; the `status.dirty` flag is left vestigial for a minimal diff | Intake prescribes this exact approach; keeping `PanelBuffer`/`dirty` intact minimizes blast radius | S:85 R:85 A:85 D:80 |
| 4 | Confident | New tests go in a new `src/node/tui/__tests__/compositor.test.ts` (compositor had no prior test file) plus additions to `watch.test.ts`, driving public methods with stubbed `getTermWidth`/`getTermRows` and captured stdout writes | Co-located `__tests__/` convention; compositor's public surface (`layoutForSkeleton`, `flush`, `rainLayer.isEnabled`) is the cheap test seam the intake suggests | S:70 R:90 A:80 D:75 |
| 5 | Confident | `layoutForSkeleton` derives `maxContentWidth` with a local `stripAnsi` pass over the skeleton lines (same logic as `computeMaxContentWidth`), reusing `computeMaxContentWidth` by passing the lines as one array | Intake says "via `stripAnsi`, same as `computeMaxContentWidth`"; reuse avoids duplication | S:75 R:90 A:85 D:80 |

5 assumptions (2 certain, 3 confident, 0 tentative).
