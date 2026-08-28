# Plan: Cost Column Autosize, Dim Zeros, Negligible-Column Omission

**Change**: 260828-7x4i-cost-column-autosize-dim-zeros
**Intake**: `intake.md`

## Requirements

### Display: Data-sized cost columns

#### R1: Cost column width follows the data, floored at 9
Every right-aligned cost column in the ANSI renderers (`renderTotalHistory` Cost column, `renderHistory` Cost column, `renderHistory`/`renderTotal` machine columns) MUST be sized to `max(COST_WIDTH, longest fmtCost() among every value the column will hold, including its Total-row value)` via one shared module-private helper `costColumnWidth(values: number[]): number`. `COST_WIDTH = 9` and `MACHINE_COL_WIDTH` remain as the floor so any render whose cells all fit 9 chars is byte-identical to today.

- **GIVEN** a monthly pivot with rows `$0.27`, `$15,429.88`, `$90,831.65` and grand total `$270,191.65`
- **WHEN** `renderTotalHistory` renders it
- **THEN** the Cost column is 11 wide; after `stripAnsi`, every data row and the Total row have the cost cell's `.` at the same string index and the bar (or `┊` rule) starting at the same index
- **AND** a pivot whose every cost cell is ≤ `$9,999.99` renders byte-identical to the current 9-wide output

#### R2: Width is computed before the bar budget
In `renderTotalHistory` the per-row cost data (`rowData`, `grandTotal`, `toolSums`) MUST be collected before `toolWidths`/`tableWidth`/`barWidth` are derived, and `barWidth` MUST subtract the computed `costWidth` (not `COST_WIDTH`). In `renderHistory` the cost and machine sums MUST likewise be pre-computed so `costWidth`/`machineColWidth` feed the `barWidth` budget. The header (`Cost`), divider (`costDiv`), each row cell and the Total cell all consume the same width variable.

- **GIVEN** a 100-col terminal and a pivot with an 11-wide Cost column
- **WHEN** bars are rendered
- **THEN** no rendered line (after `stripAnsi`) exceeds 100 visible chars, and the bar area is 2 chars narrower than the same window would get with a 9-wide column

#### R3: Per-tool pivot columns are also data-sized
Each pivot tool column MUST be `max(toolName.length, MIN_TOOL_COL_WIDTH, longest fmtCost() in that column including its Total-row sum)`.

- **GIVEN** a `Codex` column containing `$10,000.00`
- **WHEN** the pivot renders
- **THEN** that column is 10 wide, and the Cost cell and bar of every row start at one common index

#### R4: Machine columns share one data-sized width
In `renderHistory` and `renderTotal`, all machine columns MUST share a single width `machineColWidth = costColumnWidth(every machine cell ∪ every machine Total)` floored at `MACHINE_COL_WIDTH`; `machineDiv`, `machineHeader`, row cells and Total cells consume it. The snapshot's own 12-wide Cost cell (`fmtCostDelta`) is unchanged.

- **GIVEN** `--by-machine` data with one machine at `$12,345.67` and another at `$5.00`
- **WHEN** `renderHistory` or `renderTotal` renders
- **THEN** both machine columns are 10 wide and the Total row's machine cells align with the rows above

### Display: Dim zero data cells

#### R5: Exact-zero cost data cells render dim
A cost **data cell** whose value is exactly `0` MUST render its already-padded text through `dim()`: pivot per-tool cells and the pivot row Cost cell, `renderHistory` row Cost cell and machine cells, `renderTotal` machine cells. Padding happens before coloring so `row()`/`colorRow()` `padStart` is a no-op. The Total row, headers and dividers are never dimmed. A nonzero sub-cent value that formats as `$0.00` is not dimmed.

- **GIVEN** a pivot row where Codex is `$0.00` and the row total is `$0.00`
- **WHEN** rendered with color enabled
- **THEN** both cells contain the dim escape and `stripAnsi(line)` has the same length and content as an undimmed render
- **AND** under `setNoColor(true)` the output is byte-identical to the pre-change render
- **AND** the Total row's `$0.00` cells (if any) remain `boldWhite`, not dim

### Display: Negligible-column omission (ANSI pivot)

#### R6: Significance filter replaces exact-zero filter in the ANSI pivot
`renderTotalHistory` MUST select visible tool columns with a new `significantCostTools(toolNames, costMap, labels)` helper: keep a tool iff its visible-window total `≥ NEGLIGIBLE_COST_ABS` (`1.0`) AND `≥ NEGLIGIBLE_COST_SHARE` (`0.001`) × the window grand total (boundaries kept). When nothing survives, fall back to `nonzeroCostTools` (which itself falls back to the full list). `emitMarkdownTotalHistory` keeps calling `nonzeroCostTools`; CSV and JSON are untouched.

- **GIVEN** a window with Claude Code `$10,000`, Codex `$5.00`, Gemini `$0.99`, Kimi `$1.00` (and Kimi ≥ 0.1% of grand)
- **WHEN** the pivot renders
- **THEN** Codex (0.05%) and Gemini (< $1) columns are omitted; Claude Code and Kimi columns render
- **GIVEN** a window whose grand total is `$0.40` split across two tools
- **WHEN** the pivot renders
- **THEN** both nonzero tools render (first fallback), and an all-zero window renders the full registry (second fallback)

#### R7: Omitted tools still count everywhere except their own column
`rowCost`, `grandTotal`, `barTotal` (bar length) and the footer MUST sum over **all** registry tools; `cells`, `toolBars` (segments), `toolSums`, the legend and `stackedBarPalette(n)` use the filtered set. `apportionSegments` needs no change (it normalises over the shares it receives).

- **GIVEN** the R6 first fixture
- **WHEN** rendered
- **THEN** each row's Cost cell and the Total include the omitted `$5.00` and `$0.99`, the footer `avg`/`peak` include them, and `stripAnsi` of each row's stacked bar equals the unstacked bar for the full row total

### Docs: Comments and existing tests

#### R8: Constants comments and fixture tests reflect the floor semantics
The `COST_WIDTH` and `MIN_TOOL_COL_WIDTH` comments MUST describe 9 as a floor that grows with data (five-figure monthly cells are routine under `-u all`), with 96 as the **minimum** full 6-tool row. The two `formatter.test.ts` 6-tool fixtures (~483 "96 chars", ~550 "no line exceeds terminal width") MUST use per-tool costs `≥ $1.00`, `≥ 0.1%` of the row, and `≤ $9,999.99` so their width assertions remain valid under R6.

- **GIVEN** the updated fixtures
- **WHEN** `npx tsx --test src/node/tui/__tests__/formatter*.test.ts` runs
- **THEN** all tests pass

### Non-Goals
- Kimi swatch colour, footer avg computation, tokens table mode (`-t`) — follow-up change B.
- `package.json` version bump — happens at release (`just release minor`); PR body notes "requires minor release".
- Markdown pivot adopting the negligible rule — stays on exact-zero omission.
- `watch.ts` skeleton, `compositor.ts`, emitters, `colors.ts`, `STACK_PALETTE`.

### Design Decisions

#### Data-sized width with a fixed floor, not a larger constant
**Decision**: `costColumnWidth(values)` = `max(COST_WIDTH, longest fmtCost)`; 9 stays as the floor.
**Why**: Byte-identical output for every render that fits today; grows only when needed; never overflows at any magnitude. The pivot's `toolWidths` already establishes data-sizing.
**Rejected**: Raising `COST_WIDTH` to 11 — steals two bar chars from every small-cost user and breaks again at `$1,000,000.00`.
*Introduced by*: 260828-7x4i-cost-column-autosize-dim-zeros

#### One shared width for all machine columns
**Decision**: Machine columns share `machineColWidth` rather than sizing each independently.
**Why**: Letter-coded A/B/C columns read as a uniform block; one variable keeps the `mcols.length * (w + 3)` budget arithmetic.
**Rejected**: Per-column sizing — saves at most a char or two of bar width for extra bookkeeping.
*Introduced by*: 260828-7x4i-cost-column-autosize-dim-zeros

#### Negligible = `< $1.00 OR < 0.1%`, ANSI pivot only, two-level fallback
**Decision**: `significantCostTools` keeps a column iff `≥ $1.00 AND ≥ 0.1% of grand`; falls back to `nonzeroCostTools`, then to the full list. Markdown stays on exact-zero omission.
**Why**: A `$0.04` column costs ~12 chars of bar area; the absolute floor handles tiny windows, the share handles org-scale windows where `$50` is noise. Markdown is paste-ready output where hiding a real nonzero column is more surprising than an extra column.
**Rejected**: Share-only (a `$0.04` column survives a `$10` window); applying to Markdown (silently drops nonzero data from a shareable table).
*Introduced by*: 260828-7x4i-cost-column-autosize-dim-zeros

#### Dim only exact `=== 0`
**Decision**: The dim test is `cost === 0`.
**Why**: "Zeros are noise" targets absent data, not tiny spend; no epsilon needed.
**Rejected**: Dimming anything that formats as `$0.00` — hides a real sub-cent spend.
*Introduced by*: 260828-7x4i-cost-column-autosize-dim-zeros

## Tasks

### Phase 1: Setup

- [x] T001 In `src/node/tui/formatter.ts`: add module-private helpers `costColumnWidth(values: number[]): number` and `costCell(cost: number, width: number): string` near `fmtCost`; add `NEGLIGIBLE_COST_ABS = 1.0` / `NEGLIGIBLE_COST_SHARE = 0.001` constants and `significantCostTools(toolNames, costMap, labels)` beside `nonzeroCostTools`; rewrite the `COST_WIDTH` and `MIN_TOOL_COL_WIDTH` comments per R8 <!-- R1, R5, R6, R8 -->

### Phase 2: Core Implementation

- [x] T002 `renderTotalHistory` (`src/node/tui/formatter.ts`): move the `rowData`/`toolSums`/`grandTotal` loop ahead of the width budget; sum `rowCost`/`barTotal`/`grandTotal` over `allToolNames` and `cells`/`toolBars`/`toolSums` over the filtered `toolNames`; switch the filter to `significantCostTools`; compute `costWidth` and data-sized `toolWidths`; use `costWidth` in `barWidth`, `costDiv`, `costHeader`, row `costBase`, Total cell; pre-pad per-tool cells and the row Cost cell through `costCell` (Total row stays `boldWhite`) <!-- R1, R2, R3, R5, R6, R7 -->
- [x] T003 `renderHistory` (`src/node/tui/formatter.ts`): pre-pass to collect `sumCost` and `machineSums`; `costWidth = costColumnWidth([...costs, sumCost])`, shared `machineColWidth`; feed both into `barWidth`; replace `COST_WIDTH`/`MACHINE_COL_WIDTH` in `costDiv`, `costHeader`, `machineDiv`, `machineHeader`, row cells and Total cells; row Cost cell and machine cells via `costCell` <!-- R1, R2, R4, R5 -->
- [x] T004 `renderTotal` (`src/node/tui/formatter.ts`): pre-pass over `toolTotals` × `mcols` to collect machine cells and `machineSums`; shared `machineColWidth`; replace `MACHINE_COL_WIDTH` in `machineDiv`, `machineHeader`, row machine cells (via `costCell`) and Total machine cells; leave the tool row's `fmtCostDelta` Cost cell unchanged <!-- R4, R5 -->

### Phase 3: Integration & Edge Cases

- [x] T005 Update `src/node/tui/__tests__/formatter.test.ts` 6-tool fixtures (~483 "96 chars", ~550 "no line exceeds terminal width") to costs like `123.45 / 12.34 / 4.56 / 1.23 / 2.34 / 3.45` (row `$147.37`) and fix the `$128.19`-class literals and the `COST_WIDTH` comments (~189, ~235, ~707); run `npx tsx --test src/node/tui/__tests__/formatter*.test.ts` <!-- R8 -->
- [x] T006 Add `src/node/tui/__tests__/formatter-widths.test.ts`: alignment invariants for the pivot (5-/6-figure rows + Total: identical `.` index and bar-start index; floor byte-identity vs a ≤9-char fixture; no line exceeds `termWidth`), for a `$10,000.00` tool column, for `renderHistory` Cost + machine columns and `renderTotal` machine columns; dim tests (escape present on `$0.00` data cells in all three renderers, absent on Total row, `setNoColor(true)` byte-identical) <!-- R1, R2, R3, R4, R5 -->
- [x] T007 Add negligible-omission tests to `formatter-widths.test.ts` (or `formatter.test.ts` `printTotalHistory` block): `$0.99` omitted, `$1.00` kept, `$5` vs `$10,000` grand omitted (0.05%), omitted cost still present in row Cost / Total / footer, legend and palette follow the filtered set, two-level fallback (`$0.40` window → nonzero tools; all-zero → full list), Markdown pivot still exact-zero, stacked bar `stripAnsi` equals unstacked bar with an omitted tool <!-- R6, R7 -->

## Acceptance

### Functional Completeness

- [x] A-001 R1: A shared `costColumnWidth` helper exists and every ANSI cost/machine column width derives from it with `COST_WIDTH`/`MACHINE_COL_WIDTH` as floor (formatter.ts:46-48; pivot `costWidth` :774, `renderHistory` :413, shared `machineColWidth` :416/:565)
- [x] A-002 R2: In both history renderers the width variables are computed before `barWidth` and `barWidth` subtracts them (`renderHistory` pre-pass :399-419, `barWidth` :421; `renderTotalHistory` `rowData` pass :735-757 before `costWidth`/`toolWidths`, `barWidth` :783)
- [x] A-003 R3: Pivot `toolWidths` include the longest cell and Total sum per column (:765-770)
- [x] A-004 R4: `renderHistory` and `renderTotal` machine columns share one data-sized width; the snapshot Cost cell is unchanged (`renderTotal` pre-pass :556-568; `fmtCostDelta` Cost cell untouched :583)
- [x] A-005 R5: `costCell` dims exact-zero data cells in all three renderers; Total row, header, dividers untouched (`costCell` :53-56; pivot cells :820-821, `renderHistory` :470/:478, `renderTotal` :589; Total rows keep `boldWhite` :494/:607/:830-831)
- [x] A-006 R6: `significantCostTools` with the two named constants gates the ANSI pivot; Markdown still uses `nonzeroCostTools`; CSV/JSON untouched (:637-639, :651-660, pivot call :724; `emitMarkdownTotalHistory` :1242; CSV/JSON not in diff)
- [x] A-007 R7: `rowCost`/`grandTotal`/`barTotal`/footer sum over all tools; cells/segments/legend/palette over visible tools (all-tools loop :741-745; filtered loop :747-754; footer over `barTotal` :838; legend/palette over `toolNames` :729/:835-837)
- [x] A-008 R8: Constant comments describe floor semantics; the two 6-tool fixtures use `≥ $1.00` costs and still assert 96/97 (formatter.ts:111-125; formatter.test.ts:488-489 `123.45/12.34/4.56/1.23/2.34/3.45`, 96-char :520, 97-char watch :547)

### Behavioral Correctness

- [x] A-009 R1: A pivot with `$15,429.88`-class rows and a `$270,191.65`-class Total has all cost `.` characters and all bar starts at one index (test asserts) (formatter-widths.test.ts:41-53, grand `$106,261.80` → 11-wide column)
- [x] A-010 R1: A pivot whose cells all fit 9 chars renders byte-identical to the pre-change output (test asserts against a fixed expected string or the old width) (formatter-widths.test.ts:67-79 — exact header string and 48-char row length at the 9-wide floor)
- [x] A-011 R5: `setNoColor(true)` output contains no escape codes and equals the undimmed render byte-for-byte (formatter-widths.test.ts:235-249; `dim()` returns its input when color is disabled, colors.ts:20-23)
- [x] A-012 R6: `$0.99` omitted, `$1.00` kept, `$5` against `$10,000` omitted (formatter-widths.test.ts:256-276)

### Scenario Coverage

- [x] A-013 R7: Omitted tool's cost appears in the row Cost, Total and footer `avg`/`peak` (formatter-widths.test.ts:278-292 — `$10,005.99` row, `$20,005.99` Total, footer avg/peak)
- [x] A-014 R6: Fallback chain — `$0.40` window shows nonzero tools; all-zero window shows the full registry (formatter-widths.test.ts:324-345)
- [x] A-015 R3: `$10,000.00` in a 9-floor tool column widens that column and keeps Cost/bar aligned (formatter-widths.test.ts:81-93 — `$10,100.00` Codex Total → 10-wide column, aligned `.`/bar)

### Edge Cases & Error Handling

- [x] A-016 R2: No rendered line exceeds `termWidth` at 100 cols with an 11-wide Cost column and bars shown; watch-mode `indicatorReserve` still holds (formatter-widths.test.ts:95-110; `barWidth` subtracts `costWidth` + `indicatorReserve`, formatter.ts:783)
- [x] A-017 R5: A sub-cent nonzero (`0.004`) renders `$0.00` undimmed (formatter-widths.test.ts:224-233; `costCell` tests `cost === 0` only)
- [x] A-018 R7: With an omitted tool, `stripAnsi(stacked bar) === unstacked bar` for the full row total (formatter-widths.test.ts:294-309 — omitted-Gemini row fills the full 30-char bar; `apportionSegments` normalisation unchanged)

### Code Quality

- [x] A-019 Pattern consistency: new helpers are plain functions, `type` imports, `node:` imports untouched; pre-pad-then-color follows the existing `labelCell` idiom (helpers :46/:53/:651 are plain functions mirroring `nonzeroCostTools`; no import changes in the diff; `costCell` comment :49-52 names the `labelCell` trick)
- [x] A-020 No unnecessary duplication: one `costColumnWidth`, one `costCell`, one `significantCostTools`; no per-renderer copies (single definitions at :46, :53, :651, consumed by all three renderers)
- [x] A-021 Named constants: thresholds are `NEGLIGIBLE_COST_ABS`/`NEGLIGIBLE_COST_SHARE`, no magic numbers (formatter.ts:637-639)
- [x] A-022 Minimum pathways: the significance filter delegates its fallback to `nonzeroCostTools` rather than re-implementing it (formatter.ts:659)
- [x] A-023 Tests co-located in `src/node/tui/__tests__/`, Node built-in runner, all formatter tests green (`formatter-widths.test.ts` sibling; `npx tsx --test src/node/tui/__tests__/formatter*.test.ts` → 206 pass / 0 fail; `npx tsc --noEmit` clean)

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`
- PR body must note "requires minor release" (Constitution § Output Stability); do not edit `package.json`.

## Deletion Candidates

None — this change adds new functionality (data-sized widths, dim-zero cells, significance filter) without making existing code redundant. `nonzeroCostTools` (formatter.ts:631) keeps a live consumer in `emitMarkdownTotalHistory` (:1242) plus the `significantCostTools` fallback (:659); `COST_WIDTH`/`MIN_TOOL_COL_WIDTH` survive as floor constants consumed by the new helpers; no existing symbol, branch, or test lost its last call site.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | New tests go in a new sibling `formatter-widths.test.ts` rather than growing the 1372-line `formatter.test.ts` | Intake allows either; existing `formatter-history/options/stacked` siblings establish the split-by-concern naming | S:70 R:95 A:90 D:80 |
| 2 | Confident | `renderTotal` machine-column pre-pass iterates only tools with `totalTokens > 0` (the rows actually rendered) when sizing | Sizing over hidden rows would widen a column for a value nobody sees; Total sums already exclude nothing (they sum every tool) so the Total cell is included in the width set separately | S:60 R:90 A:85 D:75 |
| 3 | Certain | `costCell` is used for data cells only; Total-row cells keep `boldWhite(fmtCost(v).padStart(w))` | Intake: Total row never dimmed | S:95 R:95 A:95 D:95 |

3 assumptions (1 certain, 2 confident, 0 tentative).
