# Formatting & Display

## Overview

The formatting layer (`src/node/tui/formatter.ts`) renders token usage data into terminal tables with ANSI colors. It supports three table layouts plus compact variants for narrow terminals. The color system (`src/node/tui/colors.ts`) provides NO_COLOR-compliant ANSI wrappers.

## Requirements

- Three table layouts MUST be supported:
  1. **Single-tool history** (`renderHistory`): date rows with token breakdown (input, output, cache write, cache read, total) plus cost column and inline bar chart
  2. **Cross-tool snapshot** (`renderTotal`): tool rows with tokens, input, output, cost
  3. **Cross-tool history pivot** (`renderTotalHistory`): date rows x tool columns showing costs, with total cost column and bar chart
- Each layout MUST have both `render*` (returns `string[]`) and `print*` (console.log) variants
- Inline bar charts MUST use Unicode block characters at eighths precision (U+2588-U+258F)
- Bar width MUST auto-scale based on terminal width with 10-char minimum and 30-char maximum
- Delta indicators (up/down arrows) MUST show when `prevCosts` map is provided (for watch mode)
- Compact mode (date + cost only) MUST activate when terminal width < 60
- Colors MUST respect `NO_COLOR` env var and `--no-color` flag via `setNoColor()`
- Available color functions: `bold`, `dim`, `green`, `red`, `cyan`, `yellow`, `boldWhite`, `boldCyan`, `brightGreen`, `dimGreen`
- `stripAnsi()` MUST be available for measuring visible string length
- Total row MUST only render when more than one tool has visible data (`totalTokens > 0`) — applies to all snapshot layouts: `renderTotal`, `renderCompactSnapshot`

## Design Decisions

- **Render/print split**: `render*` functions return `string[]` for compositor consumption in watch mode. `print*` wrappers exist for direct one-shot output. This avoids duplicating rendering logic.
- **Fractional bar precision**: Using 8 Unicode block widths per character cell gives smooth visual resolution without requiring half-line tricks.
- **Delta via callback**: Watch mode passes `prevCosts` map from the previous poll cycle. Keys use `"{toolName}:{label}"` for history entries and plain `"{toolName}"` for snapshots. This keeps the formatter stateless.
- **Progressive column layout**: Tables measure column widths (D=12 for dates, N=14 for numbers) and compute remaining space for bars, enabling graceful degradation on narrow terminals.

## Changelog

| Date | Change |
|------|--------|
| 2026-03-06 | Generated from code analysis |
| 2026-03-06 | Updated file paths from `src/` to `src/node/tui/` for formatter and colors |
| 2026-03-06 | Added requirement: Total row guarded by visible tool count > 1 in renderTotal and renderCompactSnapshot |
