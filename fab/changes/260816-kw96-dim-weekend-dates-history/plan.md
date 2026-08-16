# Plan: Dim Weekend Dates in History

**Change**: 260816-kw96-dim-weekend-dates-history
**Intake**: `intake.md`

## Requirements

### Display: Weekend date-cell dimming

#### R1: Weekend date cells render dim in daily history views
In `src/node/tui/formatter.ts`, `renderHistory` and `renderTotalHistory` MUST render the **date cell only** with `dim` when the period is `daily` and the row label falls on a Saturday or Sunday. Cost/token cells and the row bar stay full-intensity — the annotation marks calendar position, not data importance. Non-compact ANSI renderers only.

- **GIVEN** a daily history window containing `2026-06-06` (Saturday) and `2026-06-08` (Monday)
- **WHEN** `renderHistory` or `renderTotalHistory` renders the rows with color enabled
- **THEN** the `2026-06-06` date cell is wrapped in `dim` and the `2026-06-08` date cell carries no styling
- **AND** every other cell in the weekend row is byte-identical to what it would be without this change

#### R2: Today marker takes precedence over weekend dim
When a weekend row's label equals `currentLabel(period)`, the date cell MUST render `boldWhite` (the existing today marker), not `dim`. One cell, one style.

- **GIVEN** today is a Saturday and appears in the daily window
- **WHEN** the row is rendered
- **THEN** the date cell renders `boldWhite` with no `dim` wrapping

#### R3: Weekday derived timezone-independently from the ISO label
The weekday check MUST parse the label with `new Date(label)` (ISO date-only strings parse as UTC midnight) and test `getUTCDay() === 0 || getUTCDay() === 6`, via one small shared helper in `src/node/tui/formatter.ts`. UTC accessors on a UTC-parsed date make the weekday a pure calendar fact, immune to the local timezone.

- **GIVEN** the label `2026-06-07` (Sunday)
- **WHEN** the helper evaluates it in any host timezone
- **THEN** it returns true; `2026-06-08` returns false

#### R4: Scope exclusions — everything else is unaffected
The dimming MUST NOT apply to: monthly period (labels have no weekday), compact mode (`renderCompactHistory` / `renderCompactTotalHistory` — the label is the row's only identifier there), CSV/Markdown emitters (carry no color), month separators, Total row, or footer. With `--no-color`/`NO_COLOR`, output MUST be byte-identical to pre-change output.

- **GIVEN** a monthly window, a compact-mode render, and a `--no-color` daily render
- **WHEN** each is rendered
- **THEN** all three outputs are byte-identical to pre-change output

#### R5: layouts.md documents the weekend-dim rule
`docs/specs/layouts.md` §3 and §4 color notes MUST gain the weekend-dim rule (dim date cell on Sat/Sun, daily only, today-marker precedence), and the Color Reference `dim` row MUST gain "weekend dates".

- **GIVEN** the shipped change
- **WHEN** a reader consults layouts.md §3/§4 or the Color Reference
- **THEN** the weekend-dim behavior is documented in both places

### Non-Goals

- Week separator lines — considered in 260815-oojd and deliberately skipped as too heavy; cell dimming is the quiet version
- Weekly period handling — weekly labels aggregate whole weeks; no per-day weekday exists
- A version bump in this diff — Output-Stability bumps happen at release time (existing Design Decision, 260816-3tah); this is color-only, patch-acceptable

## Tasks

### Phase 1: Core Implementation

- [x] T001 Add `isWeekendLabel(label: string): boolean` helper in `src/node/tui/formatter.ts` (UTC parse + `getUTCDay()`), and extend the date-cell ternary in `renderHistory` (~line 391): today → `boldWhite`, else weekend ∧ `period === "daily"` → `dim(label.padEnd(D))`, else plain <!-- R1, R2, R3 -->
- [x] T002 Apply the same date-cell extension in `renderTotalHistory` (~line 666, `D = PIVOT_DATE_WIDTH`) <!-- R1, R2 -->

### Phase 2: Tests & Docs

- [x] T003 Add tests in `src/node/tui/__tests__/formatter-history.test.ts`: Saturday and Sunday date cells dim in both renderers; weekday cells not dim; weekend-today renders boldWhite (precedence); monthly period rows carry no dim date cell; NO_COLOR daily output byte-identical to pre-change; run `npx tsx --test src/node/tui/__tests__/formatter-history.test.ts` <!-- R1, R2, R3, R4 -->
- [x] T004 [P] Update `docs/specs/layouts.md`: add the weekend-dim bullet to §3 and §4 (alongside the current-period-marker bullets at lines ~83 and ~128) and append "weekend dates" to the Color Reference `dim` row (~line 351) <!-- R5 -->

## Acceptance

### Functional Completeness

- [x] A-001 R1: Saturday/Sunday date cells render wrapped in `dim` in both `renderHistory` and `renderTotalHistory` for daily period, and only the date cell is styled
- [x] A-002 R3: The weekday helper uses `new Date(label)` + `getUTCDay()` and is shared by both renderers (no duplicated inline checks)
- [x] A-003 R5: layouts.md §3, §4, and the Color Reference document the weekend-dim rule

### Behavioral Correctness

- [x] A-004 R2: A weekend row whose label equals `currentLabel("daily")` renders its date cell `boldWhite`, never `dim`

### Scenario Coverage

- [x] A-005 R1: Tests cover weekend-dim and weekday-not-dim for both renderers and pass under `npx tsx --test`

### Edge Cases & Error Handling

- [x] A-006 R4: Monthly period and compact mode render byte-identical to pre-change; CSV/Markdown emitters untouched
- [x] A-007 R4: With color disabled (`NO_COLOR`/`--no-color`), daily history output is byte-identical to pre-change output

### Code Quality

- [x] A-008 Pattern consistency: The dim cell composes through the existing `labelCell` seam (same construction as the boldWhite today marker), following surrounding style
- [x] A-009 No unnecessary duplication: One weekday helper, reused by both renderers; no new code paths beyond the ternary extension

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Deletion Candidates

- None — this change adds new functionality (weekend date-cell dimming) without making any existing code redundant or unused

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Extend the existing `labelCell` ternary rather than restructure row construction | The today marker already styles the date cell independently at exactly this seam; the intake names it as the extension point | S:80 R:95 A:95 D:90 |
| 2 | Certain | `dim` wraps the pre-padded label (`dim(label.padEnd(D))`), mirroring the boldWhite marker | Identical mechanics to the existing marker; `row()`'s re-pad is a no-op on the longer ANSI string, keeping cell width unchanged | S:75 R:90 A:95 D:90 |
| 3 | Confident | Weekly period gets no dimming (gate is `period === "daily"` exactly) | Intake scopes to daily; weekly labels aggregate whole weeks so a weekday test is meaningless there | S:65 R:90 A:85 D:85 |
| 4 | Confident | Tests live in the existing `formatter-history.test.ts` | The file already covers history-renderer styling (month separators, marker, footer); co-locating follows test-alongside strategy | S:60 R:90 A:85 D:80 |

4 assumptions (2 certain, 2 confident, 0 tentative).
