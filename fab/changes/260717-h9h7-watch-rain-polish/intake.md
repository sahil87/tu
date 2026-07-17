# Intake: Watch-Mode Rain Polish — Cursor, Skeleton Rain, Flush Blink, Tick Removal, Gutter

**Change**: 260717-h9h7-watch-rain-polish
**Created**: 2026-07-17

## Origin

> Add 1-4 + 9 as a change — use /fab-draft — create it in the main worktree, and commit.

Conversational origin: a `/fab-discuss` session reviewed the matrix rain animation in `-w` watch mode (`src/node/tui/rain.ts`, `compositor.ts`, `watch.ts`, `colors.ts`) for performance and quality. The review produced 9 numbered findings; the user explicitly selected items **1, 2, 3, 4, and 9** for this change and explicitly deferred the rest (5: stdout backpressure guard, 6: per-run color-code batching, 7: micro allocation churn, 8: per-cell glyph ownership / frame diffing). The five selected items were assessed as "what a user would actually notice" — three visible artifacts, one CPU/battery win, one cosmetic fix.

## Why

1. **The terminal cursor is never hidden** — nothing in watch mode writes `\x1b[?25l`, so the visible cursor hops around the rain zone ~9×/sec (it lands wherever the last cursor-positioned rain write left it) and jumps to the footer on every countdown update. This is the single most user-visible flaw in watch mode.
2. **Rain does not actually start with the loading skeleton**, contradicting the documented requirement in `docs/memory/watch-mode/tui.md` ("A loading skeleton MUST render on alt-screen entry … Rain starts immediately alongside"). `RainLayer.setup()` is only reached via `Compositor.layoutAndUpdate()`, which first runs after the first successful poll (`updateAfterPoll`). During "Loading…" — and indefinitely while the first fetch keeps failing — the rain timer fires against a disabled layer and nothing animates.
3. **Rain blinks on every poll.** `Compositor.flush()` ends with `\x1b[K` per content line and `\x1b[J` below content, erasing every drawn rain cell; the rain reappears only on the next rain tick, up to 107ms later. With the default 10s interval this is a visible flash every 10 seconds.
4. **The 16ms compositor tick is ~62 wasted event-loop wakeups/sec.** The interval exists solely to poll `StatusPanel.dirty`, which changes at most once per second (the countdown). Making the footer push-driven removes the interval entirely — a CPU/battery win that also deletes code.
5. **Right-margin rain touches the content.** In right-margin mode rain columns start at the column immediately after the widest content line — glyphs visually collide with the table edge.

If not fixed: the cursor artifact and poll blink degrade every watch session; the skeleton-rain gap is a standing implementation/memory contradiction; the idle tick burns battery on laptops for zero benefit.

Approach rationale: all five are small, independent, low-risk fixes to the existing architecture (decoupled rain timer + overlay writes are kept — they are sound). The larger redesigns discussed (per-cell glyph ownership, frame diffing, backpressure) were deliberately deferred as a separate change.

## What Changes

### 1. Hide the terminal cursor for the watch session (`src/node/tui/watch.ts`)

- `enterAltScreen()` (watch.ts:30) additionally writes `\x1b[?25l` (hide cursor) after `\x1b[?1049h`.
- `exitAltScreen()` (watch.ts:34) writes `\x1b[?25h` (show cursor) before `\x1b[?1049l`.
- Because `cleanup()` already calls `exitAltScreen()` on both `q` and SIGINT paths, restoring visibility needs no new call sites.

### 2. Rain starts with the loading skeleton (`src/node/tui/watch.ts` + `src/node/tui/compositor.ts`)

- Extract the rain-zone computation in `Compositor.layoutAndUpdate()` (the block from `// Compute content height for rain zone calculation`, compositor.ts:274-300) into a private helper, e.g. `setupRainZone(contentHeight: number, maxContentWidth: number)`. `layoutAndUpdate` calls it with its existing values.
- Add a public method, e.g. `layoutForSkeleton(lines: string[])`, that derives `contentHeight = lines.length` and `maxContentWidth` (via `stripAnsi`, same as `computeMaxContentWidth`) from the skeleton lines and calls the same helper.
- `renderSkeleton()` in watch.ts builds its output as a `string[]` (same content it writes today) and returns it; `runWatch()` passes the returned lines to `compositor.layoutForSkeleton(...)` before `compositor.start()`.
- Result: rain animates in the zone below the skeleton from the first tick, including while the first fetch is slow or failing. The first `updateAfterPoll()` re-lays-out as today (RainState.resize handles dimension changes; no-op when identical).

### 3. Rain redraws immediately after every flush (`src/node/tui/compositor.ts`)

- At the end of `Compositor.flush()` (compositor.ts:313), after the footer write, append:
  `const rainOut = this.rainLayer.renderDirect(); if (rainOut) process.stdout.write(rainOut);`
- This works with the current renderer because `RainState.render()` rewrites every occupied cell every frame (`prevPositions` only drives clears), so the post-`\x1b[J` redraw fully restores the rain without waiting for the next 107ms tick.

### 4. Remove the 16ms compositor tick — push-driven footer (`src/node/tui/compositor.ts`)

- Delete `COMPOSITOR_TICK_MS`, the `compositorTimer` field, its `setInterval` in `start()`, its `clearInterval` in `stop()`, and the private `tick()` method (compositor.ts:23, 160, 180, 193-196, 336-346).
- Add a private `renderStatus()` that does what `tick()` did unconditionally: `const statusLines = this.status.render(); if (statusLines.length > 0) writeFooterLine(statusLines[0], this.opts.getTermRows());`
- Call `renderStatus()` immediately after every `status.updateCountdown(...)` call inside the compositor: in `startCountdown()` (both the initial set and the per-second `tickDown`) and in `setRefreshing()`.
- `flush()` keeps rendering the footer itself (unchanged); the `PanelBuffer` interface and the `dirty` flags stay as-is (the status `dirty` flag becomes vestigial but harmless — minimal-diff choice).
- Behavior note: footer updates become synchronous with countdown changes (previously up to 16ms deferred) — no observable regression; event-loop wakeups drop from ~62/sec to ~10/sec (rain 9.35/sec + countdown 1/sec).

### 5. Gutter in right-margin rain mode (`src/node/tui/compositor.ts`)

- In the right-margin branch of the rain-zone computation (compositor.ts:288-297): introduce `const RAIN_GUTTER = 2;` (module-level constant next to `MIN_RAIN_COLS`), compute `marginCols = tw - maxContentWidth - RAIN_GUTTER`, and set the rain zone up with `startCol = maxContentWidth + RAIN_GUTTER`.
- The existing `MIN_RAIN_COLS = 10` check applies to the post-gutter width, so rain disables itself when the gutter leaves fewer than 10 columns.

### Testing

- Existing suites are unaffected: `src/node/tui/__tests__/rain.test.ts` exercises `RainState` directly; `src/node/tui/__tests__/watch.test.ts` does not reference the compositor timer.
- Per project review policy (CLI output changes SHOULD include coverage): add coverage where cheaply testable — e.g. gutter arithmetic / rain-zone setup via the compositor's public surface, and cursor hide/show sequences if the enter/exit helpers are exported or their output captured. Test runner: `npx tsx --test` (Node built-in), co-located `__tests__/` convention.

## Affected Memory

- `watch-mode/tui`: (modify) Update requirements/design decisions: cursor hidden for the watch session (`\x1b[?25l`/`\x1b[?25h`); rain-starts-with-skeleton is now actually implemented (layout from skeleton lines before first poll); rain redraw appended to every flush (no poll blink); the 16ms compositor tick is removed — footer is push-driven on countdown changes (all "compositor tick (16ms)" mentions need rewording); right-margin rain keeps a 2-column gutter.

## Impact

- **Code**: `src/node/tui/watch.ts` (~10 lines), `src/node/tui/compositor.ts` (~30 lines net, mostly deletions). `src/node/tui/rain.ts` untouched.
- **Tests**: possible small additions under `src/node/tui/__tests__/`; no existing test changes expected.
- **Behavior/output**: watch-mode-only, cosmetic/efficiency; no table layout, JSON, or non-watch output changes (Output Stability constraint untouched).
- **Docs**: `docs/memory/watch-mode/tui.md` via hydrate (see Affected Memory).

## Open Questions

- None — scope and per-item approach were settled in the originating discussion.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Scope is exactly discussion items 1-4 + 9; backpressure guard, color-code batching, allocation micro-churn, and per-cell glyphs/frame diffing (items 5-8) are excluded | User explicitly selected "1-4 + 9"; deferred items named in discussion | S:95 R:90 A:95 D:95 |
| 2 | Certain | Cursor hide/show implemented inside `enterAltScreen`/`exitAltScreen` so all existing call sites (including SIGINT cleanup) are covered | Single obvious seam; trivially reversible | S:85 R:95 A:95 D:90 |
| 3 | Confident | Right-margin gutter is 2 columns (`RAIN_GUTTER = 2`), applied to both width check and start column | Discussion said "1-2 column gap"; 2 gives clearer separation; one-line change to retune | S:70 R:95 A:80 D:70 |
| 4 | Confident | Skeleton rain via extracting the rain-zone computation into a shared helper plus a `layoutForSkeleton(lines)` public method; `renderSkeleton` returns its lines | Minimal-diff way to reuse the existing layout logic; internal API, easily reshaped at apply | S:70 R:90 A:85 D:65 |
| 5 | Confident | Footer becomes push-driven and the 16ms interval is deleted outright (not lengthened); `PanelBuffer`/`dirty` interface left intact | Discussed ("render directly from updateCountdown… deletes code"); dirty flag kept vestigial for minimal diff | S:75 R:85 A:85 D:75 |
| 6 | Confident | Poll blink fixed by writing `rainLayer.renderDirect()` at the end of `flush()`, relying on full-cell redraw semantics of `RainState.render()` | Verified in code that render rewrites all occupied cells each frame; one-line addition | S:75 R:95 A:90 D:80 |

6 assumptions (2 certain, 4 confident, 0 tentative, 0 unresolved).
