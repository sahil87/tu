# Plan: Pivot Zero-Column Omission + Cost Thousands Separators

**Change**: 260815-q6fx-pivot-zero-columns-cost-separators
**Intake**: `intake.md`

## Requirements

### Display: Pivot zero-column omission

#### R1: ANSI pivot omits all-zero tool columns
`renderTotalHistory` (`src/node/tui/formatter.ts`) MUST omit any tool column whose total cost is zero across the **visible labels** (the labels remaining after `opts.maxRows` truncation). If the filter would leave zero tools, the renderer MUST fall back to the unfiltered tool list. When exactly one tool remains, the pivot shape (Date | Tool | Cost) MUST be preserved — no special-casing to the single-tool layout; the row Cost column stays (it is the delta-indicator and bar anchor in watch mode).

- **GIVEN** six registered tools of which only Claude Code has nonzero cost in the visible window
- **WHEN** `renderTotalHistory` renders
- **THEN** the header is `Date | Claude Code | Cost` and no `$0.00`-only column appears
- **AND** the Total row and per-row Cost/bars render against the same filtered column set

- **GIVEN** a tool whose only nonzero-cost entries fall outside the post-`maxRows` window (watch mode)
- **WHEN** the pivot renders with `maxRows` truncation active
- **THEN** that tool's column is omitted (the filter operates on displayed labels, not all fetched labels)

- **GIVEN** a pathological input where every tool sums to zero over nonempty visible labels
- **WHEN** the filter yields an empty set
- **THEN** the renderer falls back to the unfiltered tool list (defensive guard; cannot normally occur since a label exists only if some entry produced it)

#### R2: Markdown emitter applies the same omission
`emitMarkdownTotalHistory` (`src/node/tui/formatter.ts`) MUST apply the same zero-column filter (over all its labels — the emit path has no `maxRows`), with the same empty-set fallback. Markdown targets human paste contexts (PRs/Slack).

- **GIVEN** the same six-tool data with one active tool
- **WHEN** `emitMarkdown(data, "total-history", opts)` runs
- **THEN** the GFM header row contains only `Date`, the active tool(s), `Cost` (and machine columns when present)

#### R3: CSV emitter is unchanged
`emitCsvTotalHistory` MUST NOT apply the omission filter — CSV is a machine contract; scripts may index columns positionally. Every registry tool keeps its column with raw `0.00` cells.

- **GIVEN** the same six-tool data with one active tool
- **WHEN** `emitCsv(data, "total-history", opts)` runs
- **THEN** the header row is `date,{tool1},...,{tool6},total[,machine_*]` with all six tools present, cost cells raw-numeric with no separators

### Display: Cost thousands separators

#### R4: fmtCost renders thousands separators
`fmtCost` (`src/node/tui/formatter.ts`) MUST render `$${n.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}` (e.g. `$4,031.61`), mirroring the existing `fmtNum` implementation. This propagates to every ANSI renderer and to `mdCost` (which delegates to `fmtCost`). `csvCost` MUST stay `toFixed(2)` (raw-numeric, RFC 4180 contract).

- **GIVEN** a cost of 4031.61
- **WHEN** rendered in any ANSI table or Markdown table
- **THEN** the cell shows `$4,031.61`
- **AND** the CSV cell shows `4031.61`

#### R5: Width constants absorb the extra separator char
`COST_WIDTH` MUST be bumped 8 → 9 and `MIN_TOOL_COL_WIDTH` 8 → 9 in `src/node/tui/formatter.ts` so `$9,999.99` (9 chars) does not overflow its cell via `padStart` (overflow widens the row and can wrap the watch compositor). `COMPACT_COST_W` (12) already fits `$99,999.99` and MUST NOT change. `MACHINE_COL_WIDTH` (defined as `= COST_WIDTH`) follows to 9 by aliasing. The row-width comments in the formatter (the 90-char full-row math at `MIN_TOOL_COL_WIDTH`, the 79/80/81-char note in `deltaIndicator`) MUST be recomputed for the new constants: full 6-tool row = `10 + (11+9+9+9+9+9) + 6×3 + 3 + 9 = 96` (watch mode 97) — noting that with zero-column omission the typically rendered width is far below 80.

- **GIVEN** a single-tool daily cost ≥ $1,000
- **WHEN** the pivot or history table renders
- **THEN** the cell is exactly its column width (no padStart overflow) and the row does not widen

### Docs: layouts spec update

#### R6: layouts.md reflects the new widths and omission rule
`docs/specs/layouts.md` MUST be updated: §4 pivot mockup shows only tools with data (omission rule replaces the "all registry tools get a column" bullet), width math recomputed for the 9-char floors, §1–3 cost-column width notes updated (8 → 9), §5 watch-mode pivot mockup consistent with §4. The Color Reference is unaffected.

- **GIVEN** the updated spec
- **WHEN** §4's width math is checked against the formatter constants
- **THEN** the numbers match the implementation (96/97 full-row math, 9-char floors, omission rule stated)

### Release: version bump

#### R7: Minor version bump is coordinated with the release flow
Constitution "Output Stability": both changes alter parseable table output → a **minor** version bump is required. Releases are cut separately via `just release minor` (`scripts/release.sh` bumps, tags, pushes). This change MUST NOT bump `package.json` itself; the PR body MUST state the minor-bump requirement so the next release is cut as minor.

- **GIVEN** this change is merged
- **WHEN** the next release is cut
- **THEN** it is a minor bump (the PR body carries the instruction)

### Non-Goals

- Dimming zero columns (omission was the user's stated preference; dimming keeps the width problem)
- Any change to compact mode, snapshot layouts, or single-tool history column sets (no per-tool columns or already row-omitting)
- Locale-configurable separators (en-US fixed, matching `fmtNum`)
- A `--all-columns` escape hatch for the pivot (CSV already serves the stable-columns use case)

### Design Decisions

#### Zero-cost pivot columns are omitted, superseding 260703-gmcp's always-render rule
**Decision**: `renderTotalHistory` and `emitMarkdownTotalHistory` filter out tool columns with zero total cost across the visible labels; CSV keeps all columns.
**Why**: With 6 registered tools and typically 1–2 active, ~80% of the table was `$0.00` noise; the dead columns pushed the full row to 90 chars, breaking the 80-col fit and suppressing the inline bar chart (`MIN_BAR_AREA`) at 90–100-col terminals. The snapshot renderer already omits zero-token tool *rows* — the same precedent applied to columns. Output Stability is preserved where it matters: CSV (the machine contract) keeps positional columns; the ANSI/Markdown change ships with a minor version bump per the constitution.
**Rejected**: Dimming the zero columns (keeps the width problem); keeping 260703-gmcp's always-render rule (machine-dependent column presence was the concern, but the human-facing formats prioritize signal density and the bump covers the contract change).
*Introduced by*: 260815-q6fx-pivot-zero-columns-cost-separators

#### Filter on the post-maxRows visible window
**Decision**: The ANSI filter sums costs over the labels actually displayed (after `maxRows` truncation), not all fetched labels.
**Why**: Matches what the user sees; avoids ghost columns for tools active only outside the truncated watch window. The compositor re-measures every frame, so a column legitimately appearing mid-watch (tool crosses $0 → nonzero) is a safe layout change; delta keys (`total:{label}`) are label-based and unaffected.
**Rejected**: Filtering on all fetched labels (ghost all-`$0.00` columns in the watch window).
*Introduced by*: 260815-q6fx-pivot-zero-columns-cost-separators

### Deprecated Requirements

#### Zero-data tools always render a pivot column (260703-gmcp)
**Reason**: Reversed by this change — the all-`$0.00` columns bury the signal, break the 80-col fit, and crowd out the bar chart. The original rationale (machine-independent output stability) is now served by the CSV contract plus the minor version bump.
**Migration**: ANSI + Markdown pivots omit zero-cost columns (R1/R2); CSV retains all registry columns (R3). `docs/memory/display/formatting.md`'s zero-data-column requirement and the gmcp Design Decision's "Rejected: hiding zero-data columns" note must be updated at hydrate.

## Tasks

### Phase 2: Core Implementation

- [x] T001 In `src/node/tui/formatter.ts`: change `fmtCost` to `toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 })`; bump `COST_WIDTH` 8 → 9 and `MIN_TOOL_COL_WIDTH` 8 → 9; recompute the width-math comments at `MIN_TOOL_COL_WIDTH` (90 → 96 full-row math) and in `deltaIndicator` (the 79/81-char note) for the new constants and the omission rule <!-- R4, R5 -->
- [x] T002 In `src/node/tui/formatter.ts` `renderTotalHistory`: after the `maxRows` truncation and `costMap` build, filter `toolNames` to tools with nonzero total cost across the visible `labels`; fall back to the unfiltered list when the filter yields an empty set; all downstream consumers (`toolWidths`, `row`/`colorRow`, `toolSums`, Total row) use the filtered list <!-- R1 -->
- [x] T003 In `src/node/tui/formatter.ts` `emitMarkdownTotalHistory`: apply the same nonzero-cost filter over its labels with the same empty-set fallback; `emitCsvTotalHistory` stays untouched <!-- R2, R3 -->

### Phase 3: Integration & Edge Cases (tests)

- [x] T004 In `src/node/tui/__tests__/formatter.test.ts` (and `formatter-options.test.ts` where affected): update existing expected strings for the new cost format and 9-char widths <!-- R4, R5 -->
- [x] T005 Add tests in `src/node/tui/__tests__/formatter.test.ts`: (a) pivot omits all-zero tool columns (header + rows + Total), (b) single-remaining-tool pivot keeps the Date | Tool | Cost shape, (c) filter respects the post-`maxRows` window (tool with cost only outside the window is omitted), (d) empty-filter fallback renders the unfiltered list, (e) ≥$1,000 cost cell renders `$4,031.61` at exactly 9 chars (no overflow), (f) Markdown total-history omits zero columns, (g) CSV total-history retains all tool columns with raw `0.00`/separator-free cells <!-- R1, R2, R3, R4, R5 -->

### Phase 4: Polish

- [x] T006 Update `docs/specs/layouts.md`: §4 mockup (only tools with data, new widths), §4 width math (96/97, 9-char floors) and the "all registry tools get a column" bullet replaced by the omission rule (CSV exception noted), §1–3 cost-column width notes 8 → 9, §5 watch-mode pivot mockup consistent with §4 <!-- R6 -->
- [x] T007 Record the minor-version-bump requirement for the PR body (no `package.json` change; releases cut separately via `just release minor`) — add a note under `## Notes` in this plan for `/git-pr` to pick up <!-- R7 -->

## Execution Order

- T001 before T002/T003 (constants and `fmtCost` land first so the filter work tests against final widths)
- T004/T005 after T001–T003; T006/T007 independent, may run last

## Acceptance

### Functional Completeness

- [x] A-001 R1: `renderTotalHistory` with one active tool renders only `Date | {tool} | Cost` — no all-`$0.00` columns in header, data rows, or Total row
- [x] A-002 R2: `emitMarkdownTotalHistory` omits zero-cost tool columns from the GFM table
- [x] A-003 R3: `emitCsvTotalHistory` retains every registry tool column with raw-numeric cost cells (no separators, no `$`)
- [x] A-004 R4: `fmtCost(4031.61)` returns `$4,031.61`; ANSI and Markdown cost cells carry separators; `csvCost` output unchanged
- [x] A-005 R5: `COST_WIDTH` and `MIN_TOOL_COL_WIDTH` are 9; a ≥$1,000 cell does not overflow its column via `padStart`
- [x] A-006 R6: `docs/specs/layouts.md` §4 mockup, width math, and omission bullet match the implementation; §1–3 cost widths say 9; §5 consistent
- [x] A-007 R7: the minor-bump requirement is recorded for the PR body; `package.json` version untouched

### Behavioral Correctness

- [x] A-008 R1: the filter sums over post-`maxRows` visible labels — a tool with cost only outside the truncated window gets no column
- [x] A-009 R1: with a single remaining tool the pivot shape is preserved (no collapse to the single-tool history layout); watch-mode delta keys (`total:{label}`) unaffected

### Removal Verification

- [x] A-010 R1: no code path forces zero-data tools to render a pivot column in ANSI/Markdown (260703-gmcp's always-render rule is gone from the renderer)

### Scenario Coverage

- [x] A-011 R1: test exists for zero-column omission incl. Total row alignment
- [x] A-012 R5: test exists for the ≥$1,000 width case
- [x] A-013 R3: test exists asserting CSV column stability

### Edge Cases & Error Handling

- [x] A-014 R1: empty-filter fallback covered by test — all-zero visible window renders the unfiltered tool list rather than a degenerate `Date | Cost` table

### Code Quality

- [x] A-015 Pattern consistency: new code follows the formatter's existing naming/structure (render/print split untouched, pure functions, no classes)
- [x] A-016 No unnecessary duplication: the filter logic is not copy-pasted divergently between ANSI and Markdown paths beyond what their differing label windows require
- [x] A-017 No magic numbers: width changes live in the named constants (`COST_WIDTH`, `MIN_TOOL_COL_WIDTH`); no inline `9`s
- [x] A-018 Tests conform to the spec (constitution Test Integrity): expected strings updated to match the spec'd format, not vice versa

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`
- **For the PR body (R7)**: this change alters parseable ANSI/Markdown table output — per the constitution's Output Stability rule, the next release MUST be a **minor** bump (`just release minor`). `package.json` is deliberately untouched here; the release flow owns the bump.

## Deletion Candidates

- None — this change adds new functionality without making existing code redundant (the zero-column rendering path was modified in place; `nonzeroCostTools` is shared by both filtered renderers and every touched symbol retains call sites)

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | No `package.json` bump in this change; minor-bump requirement recorded for the PR body | `justfile`/`scripts/release.sh` show releases are cut separately (bump+tag+push); intake explicitly allows this path | S:70 R:90 A:90 D:85 |
| 2 | Confident | `MACHINE_COL_WIDTH` follows `COST_WIDTH` to 9 via its existing `= COST_WIDTH` alias | Per-machine cost cells face the same ≥$1,000 overflow; keeping the alias is the existing pattern | S:55 R:85 A:85 D:80 |
| 3 | Confident | Markdown filter operates on all its labels (emit path has no `maxRows`) | `EmitOptions` carries no row truncation; "visible window" degenerates to all labels there | S:60 R:85 A:85 D:80 |
| 4 | Confident | change_type corrected `docs` → `feat` by the orchestrator | The change modifies `src/` behavior and output format; `docs` would have skipped review's parsimony pass | S:60 R:90 A:85 D:80 |
| 5 | Confident | layouts.md §5 watch mockups updated alongside §4 | §5 embeds the same pivot; leaving it stale would fail the spec-consistency check the change itself introduces | S:55 R:85 A:80 D:85 |

5 assumptions (1 certain, 4 confident, 0 tentative).
