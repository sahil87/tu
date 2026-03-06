# Watch Mode & TUI

## Overview

Watch mode (`--watch`/`-w`) provides a persistent live-polling terminal UI using the alternate screen buffer. The architecture is built around a `Compositor` class that manages independent panel buffers with dirty-flag rendering. Components: `src/node/tui/watch.ts` (orchestration), `src/node/tui/compositor.ts` (layout engine), `src/node/tui/panel.ts` (stats grid), `src/node/tui/rain.ts` (matrix rain animation).

## Requirements

- Watch mode MUST use the alternate screen buffer (`\x1b[?1049h`/`\x1b[?1049l`)
- On exit (q or Ctrl-C), MUST restore normal screen and print last rendered output
- Poll interval MUST be configurable via `--interval`/`-i` (default 10s, range 5-3600s)
- Enter/Space MUST trigger immediate refresh, canceling the countdown
- The Compositor MUST manage three panels:
  1. **StatsPanel**: 2x3 stats grid rendered above the table
  2. **TablePanel**: main data table (from formatter)
  3. **StatusPanel**: footer with countdown timer and controls hint
  4. **RainLayer**: matrix rain animation overlay
- Rain animation MUST run on its own 107ms setInterval (75% of original 80ms rate), independent of compositor tick (16ms) and API polling
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
- A loading skeleton MUST render on alt-screen entry before the first fetch: stats grid with zeros/dashes, dim separator, table header, centered dim "Loading..." placeholder. Rain starts immediately alongside
- Terminal resize MUST trigger immediate re-layout via `compositor.rerender()`
- Raw mode stdin MUST be enabled for keypress handling

## Design Decisions

- **Compositor architecture**: Each panel is an independent buffer with a `dirty` flag. The compositor tick (16ms) only redraws dirty regions, avoiding full-screen repaints on every frame. Rain bypasses this entirely with direct cursor writes.
- **Decoupled rain animation**: Rain runs on a separate 107ms timer so it never freezes during API fetches. The 107ms interval (75% of original 80ms) makes the rain calmer and more ambient.
- **Stats grid above table**: Session stats render as a horizontal 2x3 grid above the table instead of a side panel. This places useful information where the eye lands first and eliminates the disconnected side panel layout.
- **Fixed grid layout**: The stats grid stays at 3 rows even when some stats are unavailable (showing `--` placeholders). This prevents layout shift as stats become available over successive polls.
- **No sparkline**: The braille sparkline was removed — 3 rows of braille over a wide cost range produced a nearly flat line, and the history table already shows trend data.
- **No side-by-side merge**: Without a side panel, `mergeSideBySide()` was removed. The table renders at full width directly.
- **Exit behavior**: On quit, the last rendered lines are printed to the normal screen so the user retains the final data without re-fetching.

## Changelog

| Date | Change |
|------|--------|
| 2026-03-06 | Generated from code analysis |
| 2026-03-06 | Updated file paths from `src/` to `src/node/tui/` for watch, compositor, panel, sparkline, rain |
| 2026-03-06 | Added requirement: Tokens/min suppressed until 2+ polls (fix divide-by-near-zero on first render) |
| 2026-03-06 | Redesign: removed sparkline and side panel, added 2x3 stats grid above table, simplified to 2-tier breakpoints, rain tick 80ms→107ms, added loading skeleton |
