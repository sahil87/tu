# Watch Mode & TUI

## Overview

Watch mode (`--watch`/`-w`) provides a persistent live-polling terminal UI using the alternate screen buffer. The architecture is built around a `Compositor` class that manages independent panel buffers with dirty-flag rendering. Components: `src/node/tui/watch.ts` (orchestration), `src/node/tui/compositor.ts` (layout engine), `src/node/tui/panel.ts` (side panel with session stats), `src/node/tui/sparkline.ts` (braille chart), `src/node/tui/rain.ts` (matrix rain animation).

## Requirements

- Watch mode MUST use the alternate screen buffer (`\x1b[?1049h`/`\x1b[?1049l`)
- On exit (q or Ctrl-C), MUST restore normal screen and print last rendered output
- Poll interval MUST be configurable via `--interval`/`-i` (default 10s, range 5-3600s)
- Enter/Space MUST trigger immediate refresh, canceling the countdown
- The Compositor MUST manage four panels:
  1. **TablePanel**: main data table (from formatter)
  2. **SparkPanel**: side panel with sparkline chart and session stats
  3. **StatusPanel**: footer with countdown timer and controls hint
  4. **RainLayer**: matrix rain animation overlay
- Rain animation MUST run on its own 80ms setInterval, independent of compositor tick (16ms) and API polling
- Rain MUST use cursor-positioned writes as an overlay, never triggering full recomposite
- Rain renders in two modes: below-content (when rows available) or right-margin (when no rows below but columns to the right)
- Side panel MUST show when terminal width >= 113 (90 min table + 3 gutter + 20 min panel) and not compact
- Sparkline MUST use braille characters (U+2800-U+28FF) with 2 data points per character and 3 rows of chart height
- Session stats MUST include: session cost delta, elapsed time, tokens/min, burn rate ($/hr), projected daily cost
- Burn rate MUST use a rolling window of last 5 polls
- History cache for sparkline MUST have 5-minute TTL
- Compact mode MUST activate below 60 columns terminal width
- Terminal resize MUST trigger immediate re-layout via `compositor.rerender()`
- Raw mode stdin MUST be enabled for keypress handling

## Design Decisions

- **Compositor architecture**: Each panel is an independent buffer with a `dirty` flag. The compositor tick (16ms) only redraws dirty regions, avoiding full-screen repaints on every frame. Rain bypasses this entirely with direct cursor writes.
- **Decoupled rain animation**: Rain runs on a separate 80ms timer so it never freezes during API fetches. This was a deliberate choice to keep visual feedback smooth even when network calls stall.
- **Braille sparklines**: Using Unicode braille patterns gives 8x resolution per character cell (2 cols x 4 rows of dots), enabling readable mini-charts in as few as 4 character columns.
- **Side-by-side merge**: `mergeSideBySide()` pads table lines to a fixed width and appends panel lines with a 3-char gutter. This is simpler than a terminal multiplexer approach and works with ANSI strings.
- **Exit behavior**: On quit, the last rendered lines are printed to the normal screen so the user retains the final data without re-fetching.

## Changelog

| Date | Change |
|------|--------|
| 2026-03-06 | Generated from code analysis |
| 2026-03-06 | Updated file paths from `src/` to `src/node/tui/` for watch, compositor, panel, sparkline, rain |
