## 2026-08-15

- **Update** [formatting](/display/formatting.md) — History views: month separators, current-period boldWhite marker, dim avg/this-month/peak footer, p95 two-zone bar scale with ┊ rule (oojd)

## 2026-04-23

- **Update** [formatting](/display/formatting.md) — Added `emitCsv` and `emitMarkdown` renderers for snapshot/history/total-history kinds, selected via a single `outputFormat` dispatch; CSV uses RFC 4180 (raw numerics, no `$`, LF, no BOM, `machine_{name}_cost` columns); Markdown uses GFM tables (commas, `$` prefix, `## {title}` heading, bolded Total row, machine names in headers) (lx0g)

## 2026-03-07

- **Update** [formatting](/display/formatting.md) — Added per-machine cost columns to `renderHistory` and `renderTotal` via `FormatOptions.machineCosts`; letter-coded headers (A/B/C), dim legend line, omitted in compact mode

## 2026-03-06

- **Update** [formatting](/display/formatting.md) — Generated from code analysis
- **Update** [formatting](/display/formatting.md) — Updated file paths from `src/` to `src/node/tui/` for formatter and colors
- **Update** [formatting](/display/formatting.md) — Added requirement: Total row guarded by visible tool count > 1 in renderTotal and renderCompactSnapshot
- **Update** [formatting](/display/formatting.md) — Added: watch mode uses same render functions without side-by-side merge (redesign removed mergeSideBySide)
