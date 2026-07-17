---
type: memory
description: Live polling TUI, compositor architecture, sparkline, rain animation, session stats
---

# Watch Mode & TUI

## Overview

Watch mode (`--watch`/`-w`) provides a persistent live-polling terminal UI using the alternate screen buffer. The architecture is built around a `Compositor` class that composites independent panel buffers and renders event-driven (on poll, resize, and countdown change) — there is no periodic compositor tick. Components: `src/node/tui/watch.ts` (orchestration), `src/node/tui/compositor.ts` (layout engine), `src/node/tui/panel.ts` (stats grid), `src/node/tui/rain.ts` (matrix rain animation).

## Requirements

- Watch mode MUST use the alternate screen buffer (`\x1b[?1049h`/`\x1b[?1049l`)
- The terminal cursor MUST be hidden for the watch session: `enterAltScreen` writes `\x1b[?25l` (after `\x1b[?1049h`) and `exitAltScreen` writes `\x1b[?25h` (before `\x1b[?1049l`). Because `cleanup()` routes both `q` and SIGINT through `exitAltScreen`, visibility is always restored on exit
- On exit (q or Ctrl-C), MUST restore normal screen and print last rendered output
- Poll interval MUST be configurable via `--interval`/`-i` (default 10s, range 5-3600s)
- Enter/Space MUST trigger immediate refresh, canceling the countdown
- The Compositor MUST manage three panels:
  1. **StatsPanel**: 2x3 stats grid rendered above the table
  2. **TablePanel**: main data table (from formatter)
  3. **StatusPanel**: footer with countdown timer and controls hint
  4. **RainLayer**: matrix rain animation overlay
- Rain animation MUST run on its own 107ms setInterval (75% of original 80ms rate), independent of the countdown timer and API polling
- Rain MUST use cursor-positioned writes as an overlay, never triggering full recomposite
- Rain renders in two modes: below-content (when rows available) or right-margin (when no rows below but columns to the right)
- Stats grid MUST render as a 2-column, 3-row grid above the table:
  - Left column (session): Elapsed, Session cost delta
  - Right column (cost): Tok/min, Rate ($/hr), Proj. day
  - Row 3 left is blank (2 session stats vs 3 cost stats)
  - Labels styled `dim`, values `boldWhite`, Rate in `yellow`
- A dim horizontal rule (`───`) MUST separate the stats grid from the table title
- Unavailable stats (before 2 polls) MUST show `--` as placeholder; grid stays fixed at 3 rows
- Session delta MUST show `$0.00` before 2 polls (not `--`)
- Rate-based stats (Tokens/min) MUST NOT display until at least two poll cycles have completed (`pollHistory.length > 1`); this prevents divide-by-near-zero producing absurd values on first render
- Burn rate MUST use a rolling window of last 5 polls
- Two terminal breakpoints:
  - **Full** (>= 60 cols): stats grid + dim separator + full table + rain
  - **Compact** (< 60 cols): compact table only, no stats grid, no rain
- A loading skeleton MUST render on alt-screen entry before the first fetch: stats grid with zeros/dashes, dim separator, table header, centered dim "Loading..." placeholder. Rain MUST animate alongside it from the first tick: `renderSkeleton()` returns its content `string[]`, and `runWatch` passes them to `Compositor.layoutForSkeleton(lines)` to lay out the rain zone before `compositor.start()`. Rain therefore animates during "Loading…" and continues while the first fetch keeps failing (previously the rain layer was only set up on the first successful poll, so nothing animated until then)
- The right-margin rain mode MUST keep a `RAIN_GUTTER` (2)-column gap from the widest content line: rain starts at column `maxContentWidth + RAIN_GUTTER` with width `termWidth - maxContentWidth - RAIN_GUTTER`, and the `MIN_RAIN_COLS` (10) check applies to that post-gutter width (rain disables itself when fewer than 10 columns remain after the gutter)
- Terminal resize MUST trigger immediate re-layout via `compositor.rerender()`
- The footer status line MUST be push-driven, not polled: `Compositor` runs NO periodic compositor tick. `renderStatus()` is called synchronously on every countdown state change (`startCountdown`'s initial set and per-second tick-down) and on `setRefreshing()`, so the footer updates the instant the countdown changes
- `flush()` MUST re-emit the current rain frame (`rainLayer.renderDirect()`) after writing the footer, because the flush's `\x1b[K`/`\x1b[J` clears erase every drawn rain cell; the re-emit restores them in the same flush so polls no longer blink the rain (`RainState.render()` rewrites every occupied cell each frame, so no per-cell diff is needed)
- Raw mode stdin MUST be enabled for keypress handling

## Design Decisions

- **Compositor architecture (push-driven, no periodic tick)**: Each panel is an independent buffer exposing `render()`. There is NO periodic compositor tick — rendering is event-driven: `flush()` composites the stats/table/footer on a poll (or resize), and the footer is re-rendered on demand via `renderStatus()` whenever the countdown state changes. Rain bypasses the panel pipeline entirely with direct cursor writes on its own timer. *(Superseded the earlier 16ms `setInterval` that polled the panels' `dirty` flags — removed in `watch-rain-polish`. The `PanelBuffer.dirty` flag and its per-panel fields remain in the interface but are now vestigial: the tick was their only reader, so they are write-only and slated for follow-up cleanup — see `plan.md` Deletion Candidates. Removing the tick cut idle event-loop wakeups from ~62/sec to ~10/sec, since the countdown only changes once per second.)*
- **Decoupled rain animation**: Rain runs on a separate 107ms timer so it never freezes during API fetches. The 107ms interval (75% of original 80ms) makes the rain calmer and more ambient. The rain timer is the compositor's only remaining periodic interval.
- **Rain redraw on every flush (no poll blink)** (`watch-rain-polish`): `flush()` re-emits the current rain frame after the footer write, because its `\x1b[J`/`\x1b[K` clears wipe the rain zone. This relies on `RainState.render()` rewriting every occupied cell each frame, so the re-emit fully restores the rain in the same flush — chosen over suppressing the clears (which would leave stale content lines).
- **Skeleton rain via a shared `setupRainZone` helper** (`watch-rain-polish`): the rain-zone computation was extracted from `layoutAndUpdate` into a private `setupRainZone(contentHeight, maxContentWidth)`, and a public `layoutForSkeleton(lines)` derives geometry from the skeleton lines and reuses it. `renderSkeleton` returns its `string[]` so `runWatch` can lay out the rain zone before the first poll — minimal-diff reuse of the layout logic rather than duplicating the zone math. The first `updateAfterPoll()` re-lays-out with the real table dimensions (a no-op resize when they match the skeleton's geometry).
- **Stats grid above table**: Session stats render as a horizontal 2x3 grid above the table instead of a side panel. This places useful information where the eye lands first and eliminates the disconnected side panel layout.
- **Fixed grid layout**: The stats grid stays at 3 rows even when some stats are unavailable (showing `--` placeholders). This prevents layout shift as stats become available over successive polls.
- **No sparkline**: The braille sparkline was removed — 3 rows of braille over a wide cost range produced a nearly flat line, and the history table already shows trend data.
- **No side-by-side merge**: Without a side panel, `mergeSideBySide()` was removed. The table renders at full width directly.
- **Exit behavior**: On quit, the last rendered lines are printed to the normal screen so the user retains the final data without re-fetching.
