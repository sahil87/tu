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
- Watch mode uses the same `render*` functions as non-watch mode — no side-by-side merge or layout transformation is applied by the formatter
- When `FormatOptions.machineCosts` is provided (`Map<string, Map<string, number>>`, keyed by label/toolName → machine → cost), `renderHistory` and `renderTotal` MUST append letter-coded machine cost columns (A, B, C...) after the Cost column, with 8-char right-aligned width (`MACHINE_COL_WIDTH`), alphabetically sorted machine names, and a `dim` legend line (`Machines: A = name, B = name`)
- Machine columns MUST include per-machine totals in the Total row
- Machine columns MUST be omitted in compact mode regardless of `machineCosts` presence
- `emitCsv(data, kind, opts)` (`src/node/tui/formatter.ts`) MUST render three kinds — `"snapshot"`, `"history"`, `"total-history"` — as RFC 4180-compliant CSV on stdout. Rules: first line is a header row, comma separator, LF (`\n`) terminator (no CRLF), no BOM, no ANSI/no bars/no delta arrows. String fields containing `,`, `"`, or newlines MUST be quoted with `"` and internal `"` doubled. Numeric fields render raw (no thousands separators). Cost fields use two decimals with no `$` prefix. A final `Total,...` row follows when more than one tool has visible data (`visibleCount > 1`). Snapshot header: `tool,tokens,input,output,cost`. History header: `date,input,output,cache_write,cache_read,total,cost`. Total-history header: `date,{tool1},{tool2},...,total` (rows sorted by date ascending). Date labels use ISO format (`YYYY-MM-DD` or `YYYY-MM`)
- `emitMarkdown(data, kind, opts)` MUST render the same three kinds as GitHub-flavoured Markdown tables on stdout. Rules: output begins with a `## {title}` heading line (titles match ANSI renderer titles — `Combined Usage ({period})`, `{toolName} ({period})`, `Combined Cost History ({period})`), followed by a blank line, then a GFM table (header row, alignment separator, data rows). String columns left-aligned (`:---`); numeric columns right-aligned (`---:`). Numeric values keep comma thousands separators (human readability). Cost values use `$` prefix with two decimals. A `**Total**` row with bolded numeric values is appended when `visibleCount > 1`. No ANSI, no inline bars, no delta arrows. Trailing newline at end of output
- Machine columns in CSV MUST append after base columns as `machine_{name}_cost` per machine, sorted alphabetically by machine name (via `collectMachineNames`)
- Machine columns in Markdown MUST append after base columns using the machine name directly as the header (no A/B/C letter codes, no separate `Machines:` legend line — GFM output is paste-ready and doesn't need the width-conservation letter coding used by the ANSI renderer)
- Output-format dispatch MUST happen once per dispatch function: `fetchAllHistory` / `fetchToolMerged` / `fetchToolSnapshot` runs exactly once, then a single `switch (outputFormat)` selects `emitJson`, `emitCsv`, `emitMarkdown`, or the existing `print*` renderer. Duplicating fetch logic across format branches is prohibited

## Design Decisions

- **Render/print split**: `render*` functions return `string[]` for compositor consumption in watch mode. `print*` wrappers exist for direct one-shot output. This avoids duplicating rendering logic.
- **Fractional bar precision**: Using 8 Unicode block widths per character cell gives smooth visual resolution without requiring half-line tricks.
- **Delta via callback**: Watch mode passes `prevCosts` map from the previous poll cycle. Keys use `"{toolName}:{label}"` for history entries and plain `"{toolName}"` for snapshots. This keeps the formatter stateless.
- **Progressive column layout**: Tables measure column widths (D=12 for dates, N=14 for numbers) and compute remaining space for bars, enabling graceful degradation on narrow terminals.
- **Unified rendering**: Watch mode and non-watch mode produce identical table output. The compositor stacks the stats grid above the table output without modifying it.
- **CSV vs. Markdown numeric conventions** (260423-lx0g): CSV targets machine consumers (spreadsheets, `awk`/`cut` pipelines) — raw numbers with no thousands separators, costs without `$`, snake_case `machine_{name}_cost` column headers sorted alphabetically. Markdown targets humans (PRs, Slack, docs) — commas in numbers, `$` prefix on costs, machine names used directly in headers, `**Total**` row bolded to match the ANSI renderer's `boldWhite` convention. Each format optimises for its paste target rather than sharing a single numeric rendering path.
- **CSV/MD output always strips ANSI, inline bars, and delta arrows** (260423-lx0g): Bars are terminal-visual affordances, and delta arrows only have meaning in watch context. Both are useless (or actively harmful) in CSV/MD paste targets.
- **Markdown output always includes a `## {title}` heading** (260423-lx0g): The dominant paste targets (GH PRs, GH issues, internal docs) benefit from the heading; removing it post-hoc is trivial (`tail -n +2`), adding one is more work. A `--no-heading` flag is listed as a non-goal for this change but is an easy follow-up if requested.

## Changelog

| Date | Change |
|------|--------|
| 2026-03-06 | Generated from code analysis |
| 2026-03-06 | Updated file paths from `src/` to `src/node/tui/` for formatter and colors |
| 2026-03-06 | Added requirement: Total row guarded by visible tool count > 1 in renderTotal and renderCompactSnapshot |
| 2026-03-06 | Added: watch mode uses same render functions without side-by-side merge (redesign removed mergeSideBySide) |
| 2026-03-07 | Added per-machine cost columns to `renderHistory` and `renderTotal` via `FormatOptions.machineCosts`; letter-coded headers (A/B/C), dim legend line, omitted in compact mode |
| 2026-04-23 | Added `emitCsv` and `emitMarkdown` renderers for snapshot/history/total-history kinds, selected via a single `outputFormat` dispatch; CSV uses RFC 4180 (raw numerics, no `$`, LF, no BOM, `machine_{name}_cost` columns); Markdown uses GFM tables (commas, `$` prefix, `## {title}` heading, bolded Total row, machine names in headers) (260423-lx0g) |
