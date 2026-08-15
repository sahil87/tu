# Plan: Snapshot Cache Token Visibility

**Change**: 260815-nda3-snapshot-cache-token-visibility
**Intake**: `intake.md`

## Requirements

### Display: Snapshot Cache Column

#### R1: Combined Cache column in the ANSI snapshot
`renderTotal` (`src/node/tui/formatter.ts`) MUST render a combined `Cache` column (`cacheCreationTokens + cacheReadTokens`) so the visible numeric columns close arithmetically: `Input + Output + Cache = Tokens`. Column order MUST be `Tool | Tokens | Input | Output | Cache | Cost`. The Total row MUST sum the Cache column like the other numerics. A tool with zero cache MUST render `0` in the Cache cell.

- **GIVEN** a `toolTotals` map with Claude Code at input 3,734 / output 1,121,329 / cache write+read 486,557,984 / total 487,683,047
- **WHEN** `renderTotal("daily", totals)` renders
- **THEN** the row shows `Cache` = `486,557,984` between `Output` and `Cost`, and Input + Output + Cache equals the Tokens cell
- **AND** the Total row's Cache cell is the sum of per-tool cache values

#### R2: Width budget ≤ 90
The snapshot table MUST fit all five numeric columns within 90 visible columns. Achieve this by narrowing the snapshot's fixed widths from 14 to 12 for both the Tool column (`W`) and the numeric columns (`N`): `12 + 5×12 + 5×3 = 87`. A 12-wide cell holds `999,999,999,999`. The machine-column variant (`machineCosts`) MUST keep appending letter-coded 9-wide columns after `Cost` exactly as today; compact mode (`renderCompactSnapshot`) is unchanged.

- **GIVEN** the full snapshot with 5 numeric columns
- **WHEN** the header, divider, data rows, and Total row render
- **THEN** every line measures 87 visible chars (≤ 90) before any machine columns
- **AND** with `machineCosts` present, machine columns append after Cost with 9-char width and the dim legend, as before

#### R3: Markdown snapshot gains the same Cache column
`emitMarkdownSnapshot` MUST add a right-aligned `Cache` column between `Output` and `Cost` (value `mdNum(cacheCreationTokens + cacheReadTokens)`), including a bolded Total cell. Markdown is width-unconstrained; machine columns still append after Cost.

- **GIVEN** snapshot data with nonzero cache
- **WHEN** `emitMarkdown(data, "snapshot", opts)` runs
- **THEN** the header row is `| Tool | Tokens | Input | Output | Cache | Cost |` (plus machine names when present) and the `**Total**` row bolds the Cache sum

#### R4: CSV snapshot gains a `cache` column after `output`
`emitCsvSnapshot` MUST add a `cache` column appended after `output`, before `cost`: header `tool,tokens,input,output,cache,cost[,machine_{name}_cost...]`. Values are raw numbers (`csvNum`), the Total row includes the cache sum, and machine columns keep their position after `cost`. This is a CSV shape change and MUST be flagged in the PR body.

- **GIVEN** snapshot CSV output via `tu --csv`
- **WHEN** the header row emits
- **THEN** it reads exactly `tool,tokens,input,output,cache,cost` (machine columns appended after `cost` when present)

#### R5: Watch loading skeleton stays in sync
The watch-mode loading skeleton (`src/node/tui/watch.ts`, `renderSkeleton` header block) MUST mirror `renderTotal`'s new column set and widths: columns `Tool, Tokens, Input, Output, Cache, Cost`, `W = 12`, `N = 12`, divider built over six widths — so the skeleton's header/divider align with the first real frame.

- **GIVEN** watch mode before the first fetch completes
- **WHEN** the loading skeleton renders
- **THEN** its header and divider are identical in columns and widths to `renderTotal`'s

#### R6: Docs and version
`docs/specs/layouts.md` §1 and §2 mockups and column notes MUST be updated to the new six-column, 12-wide layout, and the §6 loading-skeleton mockup MUST match R5. The constitution's Output Stability rule requires a minor version bump; `package.json` is already at the unreleased `0.10.0` (bumped with q6fx/oojd), which this change MAY share — verify, do not bump again.

- **GIVEN** the updated snapshot layout
- **WHEN** a reader consults layouts.md §1/§2/§6
- **THEN** the mockups show `Tool | Tokens | Input | Output | Cache | Cost` at the new widths and the arithmetic in the mockup rows closes

### Non-Goals

- JSON output — already exposes both cache fields; no change
- Compact mode (watch, <60 cols) — name + cost only; no change
- Single-tool history — already shows separate Cache Write / Cache Read columns; no change
- The compact `487.7M (99% cache)` treatment — rejected at intake in favor of a column

### Design Decisions

#### Combined Cache column, widths 12/12
**Decision**: One combined `Cache` column (write + read) between `Output` and `Cost`; snapshot fixed widths `W`/`N` both narrow 14 → 12, giving an 87-char full row.
**Why**: Closes the row arithmetic with data already on `UsageTotals`; 12-wide cells hold any realistic token count; 87 ≤ 90 keeps the intake's hard requirement without per-column special cases.
**Rejected**: Separate Cache Write / Cache Read columns (too wide for an at-a-glance snapshot — that granularity lives in single-tool history); keeping 14-wide columns (99-char rows, needlessly past the ≤ 90 budget).
*Introduced by*: 260815-nda3-snapshot-cache-token-visibility

## Tasks

### Phase 2: Core Implementation

- [x] T001 In `src/node/tui/formatter.ts` `renderTotal`: narrow `W`/`N` 14 → 12, add `Cache` header/divider/data/Total cells (`cacheCreationTokens + cacheReadTokens`), keeping machine columns after Cost and compact mode untouched <!-- R1, R2 -->
- [x] T002 [P] In `src/node/tui/formatter.ts`: add the Cache column to `emitMarkdownSnapshot` (between Output and Cost, bold Total cell) and the `cache` column to `emitCsvSnapshot` (header + rows + Total, after `output`) <!-- R3, R4 -->

### Phase 3: Integration & Edge Cases

- [x] T003 In `src/node/tui/watch.ts` loading skeleton: update `cols` to six columns and `W`/`N` to 12, divider over six widths, matching `renderTotal` <!-- R5 -->
- [x] T004 Update `src/node/tui/__tests__/formatter.test.ts` (renderTotal, emitCsv snapshot header assertions incl. machine-column variant, emitMarkdown snapshot) and any watch skeleton assertions; add cases: cache arithmetic closes (Input+Output+Cache = Tokens cells), zero-cache tool renders `0`, machine-column variant renders after Cost; run the formatter + watch test files <!-- R1, R2, R3, R4, R5 -->

### Phase 4: Polish

- [x] T005 Update `docs/specs/layouts.md` §1/§2 mockups + column notes and §6 loading-skeleton mockup to the six-column 12-wide layout; verify `package.json` 0.10.0 minor bump is unreleased and shared (no further bump) <!-- R6 -->

## Acceptance

### Functional Completeness

- [x] A-001 R1: `renderTotal` shows a `Cache` column between `Output` and `Cost` whose per-row value is `cacheCreationTokens + cacheReadTokens`, with a summed Total cell
- [x] A-002 R3: `emitMarkdown(..., "snapshot")` output carries the `Cache` column with right alignment and a bolded Total cell
- [x] A-003 R4: `emitCsv(..., "snapshot")` header is `tool,tokens,input,output,cache,cost` with machine columns (when present) after `cost`
- [x] A-004 R5: the watch loading skeleton header/divider match `renderTotal`'s new columns and widths

### Behavioral Correctness

- [x] A-005 R1: for every rendered snapshot row (including Total), Input + Output + Cache = Tokens
- [x] A-006 R2: full snapshot rows measure 87 visible chars (≤ 90) without machine columns; machine variant appends 9-wide columns after Cost unchanged

### Scenario Coverage

- [x] A-007 R1: test covers the cache-arithmetic scenario and a zero-cache tool rendering `0`
- [x] A-008 R2: test covers the machine-column variant width/position with the new layout

### Edge Cases & Error Handling

- [x] A-009 R1: all-zero totals still short-circuit to `No usage`; compact mode output unchanged

### Code Quality

- [x] A-010 Pattern consistency: new cells reuse `fmtNum`/`mdNum`/`csvNum` and the existing row/colorRow builders; no new rendering pathway introduced
- [x] A-011 No unnecessary duplication: cache sum computed via the existing per-loop accumulation pattern, not a parallel helper

### Docs & Version

- [x] A-012 R6: layouts.md §1/§2/§6 mockups match the implemented output (columns, widths, closing arithmetic); version 0.10.0 minor bump confirmed shared and unreleased

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Deletion Candidates

- None — this change adds new functionality without making existing code redundant

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | Snapshot widths land at W=12, N=12 (87-char row) rather than chasing ≤ 80 | Intake fixes the requirement at ≤ 90 with clean columns preferred over distortion; 12/12 is the intake's own worked example | S:70 R:85 A:80 D:70 |
| 2 | Confident | Watch loading skeleton is updated in the same change | It hardcodes the snapshot header/widths; leaving it would misalign the skeleton with the first real frame — sync is implied by the intake's renderTotal scope | S:60 R:85 A:85 D:75 |
| 3 | Certain | No further version bump — the unreleased 0.10.0 shared with q6fx/oojd satisfies Output Stability | Intake assumption 5 explicitly allows a shared bump; package.json already reads 0.10.0 above last release v0.9.4 | S:80 R:90 A:95 D:90 |
| 4 | Confident | layouts.md §6 loading-skeleton mockup updated alongside §1/§2 | §6 shows the same snapshot header; updating only §1/§2 would leave the spec internally inconsistent | S:60 R:90 A:85 D:75 |

4 assumptions (1 certain, 3 confident, 0 tentative).
