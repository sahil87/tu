# Plan: History Month Anchors + p95 Bar Scaling

**Change**: 260815-oojd-history-month-anchors-p95-bars
**Intake**: `intake.md`

## Requirements

### Display: Month-boundary separators

#### R1: Month-boundary separator lines in daily history views
`renderHistory` and `renderTotalHistory` MUST emit a dim divider line (identical construction to the existing header divider: `divStr + costDiv + [machineDiv] + barDiv` for `renderHistory`; `divStr + costDiv + barDiv` for `renderTotalHistory`) immediately before each data row whose label's `YYYY-MM` prefix differs from the previous row's label prefix. The separator MUST render only when `period === "daily"` and only in the non-compact ANSI renderers (compact mode, CSV, and Markdown emitters are untouched). Separators reflect the visible window (labels after `maxRows` truncation).

- **GIVEN** a daily history window spanning 2026-06-28 … 2026-07-02
- **WHEN** `renderHistory` (or `renderTotalHistory`) renders the table
- **THEN** exactly one dim divider line appears, between the 2026-06-30 row and the 2026-07-01 row
- **AND** no separator appears before the first data row (there is no previous label)

- **GIVEN** a monthly (or weekly) period
- **WHEN** the same window renders
- **THEN** no month-boundary separators appear

### Display: Current-period row marker

#### R2: Current-period label cell rendered in boldWhite
In `renderHistory` and `renderTotalHistory` (non-compact ANSI only), the row whose label equals `currentLabel(period)` (imported from `src/node/core/fetcher.ts` — local-time semantics, already covers daily/weekly/monthly) MUST render its date/label cell in `boldWhite` (the existing Total-row emphasis color). No extra glyph or column — the cell's padded width is unchanged, and the color is stripped automatically under `--no-color`/`NO_COLOR`.

- **GIVEN** a daily history containing a row labeled with today's local ISO date
- **WHEN** the table renders
- **THEN** that row's date cell is wrapped in the `boldWhite` ANSI sequence and every other row's date cell is not
- **AND** `stripAnsi` of the marked row equals the unmarked rendering (width unchanged)

- **GIVEN** a monthly history containing the current month's row
- **WHEN** the table renders
- **THEN** the current month's label cell is `boldWhite`

### Display: Summary footer

#### R3: Summary footer line after the Total row in history views
`renderHistory` and `renderTotalHistory` MUST append one dim footer line after the Total row (same visual weight as the machine legend), rendered only in non-compact ANSI output and only when the window has 2 or more data rows (matching the Total row's own condition):

```
avg $XX.XX/day · this month $X,XXX.XX · peak $X,XXX.XX (2026-06-12)
```

- `avg` = window total cost / number of data rows (days with data, not calendar days), formatted with `fmtCost`; the unit suffix derives from `period` (`/day`, `/week`, `/month`).
- `this month` = sum of row costs whose label falls in the current calendar month — included only for the daily period, and omitted when the window contains no current-month rows.
- `peak` = max row cost with its label in parentheses.
- CSV and Markdown emitters MUST NOT carry the footer (CSV is a machine contract; Markdown tables are consumed standalone).

- **GIVEN** a daily window with rows totaling $1,025.70 over 5 data rows, containing current-month rows summing $1,204.50, peak $4,031.61 on 2026-06-12
- **WHEN** the table renders
- **THEN** the footer reads `avg $205.14/day · this month $1,204.50 · peak $4,031.61 (2026-06-12)` (dim)

- **GIVEN** a monthly window
- **WHEN** the table renders
- **THEN** the footer reads `avg $X,XXX.XX/month · peak $X,XXX.XX (2026-06)` with no `this month` segment

### Display: p95 two-zone bar scale

#### R4: p95-capped two-zone bar rendering with a scale-break rule
Bar scaling in `renderHistory` and `renderTotalHistory` MUST switch from single-zone linear (`barScale = maxCost`) to a two-zone piecewise scale when `maxCost > 1.5 × p95`, where `p95` is the 95th percentile (linear interpolation over the sorted ascending **nonzero** visible row costs). When the trigger does not fire (including windows with no nonzero costs), rendering MUST be byte-identical to today's single-zone output. When active, the bar area (`barWidth` chars) splits into:

- **Main zone** — linear `0 → p95`, width `barWidth − overflowZone − 1`; rows with `cost ≥ p95` fill it completely.
- **Scale-break rule** — a single dim `┊` (U+250A) at the cap column, rendered in **every** row including zero/short-bar rows (short bars are space-padded up to the rule column so the rule aligns vertically).
- **Overflow zone** — width `max(4, round(barWidth / 4))`, linear `p95 → maxCost`; only rows with `cost > p95` render into it, using the same fractional-eighths blocks, colored `yellow`. The main zone stays `green`. Rows at exactly `p95` end at the rule with no overflow segment.

When the two-zone mode is active, the R3 footer MUST append ` · ┊ = $XXX (p95)` (p95 formatted with `fmtCost`).

- **GIVEN** a window where p95 ≈ $846 and rows of $1,091.67 and $4,031.61 exist (trigger fires)
- **WHEN** the bars render
- **THEN** every row carries the `┊` rule at the same visible column, the $1,091.67 row shows a short yellow overflow segment, the $4,031.61 row shows a proportionally longer one (they render differently), and rows below p95 show green bars space-padded to the rule

- **GIVEN** a well-behaved window (`maxCost ≤ 1.5 × p95`)
- **WHEN** the bars render
- **THEN** output is unchanged from the current single-zone rendering — no rule, no footer legend

#### R5: Spec mockups updated
`docs/specs/layouts.md` §3/§4 mockups MUST gain the month separators, summary footer, and clipped-bar (two-zone) examples so the spec matches the shipped layouts.

- **GIVEN** the layouts spec
- **WHEN** the change ships
- **THEN** the single-tool history and cross-tool pivot mockups show a month separator, the dim footer line, and a `┊` scale-break example

### Release: Version bump

#### R6: Minor version bump
The footer and separator lines change parseable output shape → `package.json` version MUST receive a minor bump (0.9.4 → 0.10.0) per the constitution's Output Stability rule. (q6fx's changes ride the same bump if released together.)

- **GIVEN** the current version 0.9.4
- **WHEN** this change ships
- **THEN** `package.json` reads 0.10.0

### Non-Goals

- No footer/separators/marker in compact mode, CSV, or Markdown output — CSV is a machine contract; Markdown tables are standalone; compact is the narrow-terminal fallback.
- No configurability of the percentile, trigger factor, or zone split — hardcoded judgment calls per the intake.
- No weekly-specific grouping separators (month boundaries only, daily only).

### Design Decisions

#### Current-period marker keys on `currentLabel(period)` uniformly
**Decision**: The marker highlights the row whose label equals `currentLabel(period)` from `src/node/core/fetcher.ts`, for every period (daily → today, weekly → this week, monthly → this month).
**Why**: One code path covers the intake's daily + monthly cases and weekly falls out for free; `currentLabel` already encodes the local-time label logic and importing it introduces no import cycle (fetcher does not import the TUI layer).
**Rejected**: A local `todayLabel` helper in formatter.ts — duplicates existing logic (code-quality anti-pattern); a `todayLabel` FormatOptions field — extra pathway with no consumer needing an override.
*Introduced by*: 260815-oojd-history-month-anchors-p95-bars

#### Two-zone geometry: overflow = max(4, round(barWidth/4)), rule 1 char, main = remainder
**Decision**: Fixed split derived from `barWidth`; main zone scales `0→p95`, overflow scales `p95→maxCost`.
**Why**: At the minimum bars-shown width (10) this leaves main 5 / rule 1 / overflow 4 — both zones stay legible; at MAX_BAR_WIDTH 30 it gives main 21 / overflow 8, matching the intake mock's proportions.
**Rejected**: Percentage-only split with no floor (overflow collapses to 1–2 chars at narrow widths, making $1k vs $4k indistinguishable again — the exact failure the feature fixes).
*Introduced by*: 260815-oojd-history-month-anchors-p95-bars

## Tasks

### Phase 2: Core Implementation

- [x] T001 Add exported `percentile(sortedAscending: number[], p: number): number` (linear interpolation) and a `computeBarScale(costs: number[], barWidth: number)` helper in `src/node/tui/formatter.ts` returning a scale descriptor: `{ mode: "single", max }` or `{ mode: "two-zone", p95, max, mainZone, overflowZone }` per the R4 trigger (`maxCost > 1.5 × p95` over nonzero costs; no nonzero costs → single) <!-- R4 -->
- [x] T002 Add a two-zone bar builder in `src/node/tui/formatter.ts` (e.g. `renderScaledBar(value, scale)`) reusing `renderBar`/`BLOCK_EIGHTHS`: green main zone (space-padded to the rule for short bars), dim `┊` U+250A rule in every row, yellow overflow segment for `value > p95`; single-zone mode delegates to today's `renderBar` unchanged <!-- R4 -->
- [x] T003 Wire `computeBarScale`/`renderScaledBar` into `renderHistory` (replacing the `maxCost`-scaled `renderBar` call) so single-zone windows render byte-identically to today <!-- R4 -->
- [x] T004 Wire the same into `renderTotalHistory` (row totals as costs; `indicatorReserve` math unchanged — the two-zone bar never exceeds `barWidth`) <!-- R4 -->
- [x] T005 [P] Month-boundary separators in `renderHistory` (divider `divStr + costDiv + machineDiv + barDiv`) and `renderTotalHistory` (`divStr + costDiv + barDiv`), emitted before rows whose `YYYY-MM` label prefix differs from the previous row's, `period === "daily"` only, non-compact only <!-- R1 -->
- [x] T006 [P] Current-period marker: import `currentLabel` from `../core/fetcher.js` into `src/node/tui/formatter.ts`; render the matching row's label cell `boldWhite` in `renderHistory` and `renderTotalHistory` (non-compact only), preserving padded width <!-- R2 -->
- [x] T007 Summary footer builder (e.g. `renderHistoryFooter(labels/costs, period, scale)`) in `src/node/tui/formatter.ts`: dim `avg … · [this month …] · peak … [(· ┊ = $X (p95))]` line per R3/R4-legend rules; append after the Total row in `renderHistory` and `renderTotalHistory` when ≥2 data rows, non-compact ANSI only (before the machine-legend blank line in `renderHistory`) <!-- R3 -->

### Phase 3: Integration & Edge Cases

- [x] T008 Tests in new co-located file `src/node/tui/__tests__/formatter-history.test.ts`: percentile math; separator placement (boundary count/position, none for monthly, none before first row); today-marker color on today's row only, width unchanged via `stripAnsi`; footer math (avg/this-month/peak, monthly variant drops `this month`, unit suffix per period); p95 trigger on/off (identical output when off); rule column alignment across rows; overflow proportions ($1,091 vs $4,031 render differently); footer p95 legend; compact/CSV/Markdown untouched <!-- R1 R2 R3 R4 -->
- [x] T009 Run the full test suite (`npm test`) and the bundle build (`just build` or `npm run build` per justfile) to verify no regressions, including existing formatter/compositor/watch tests <!-- R4 -->

### Phase 4: Polish

- [x] T010 [P] Update `docs/specs/layouts.md` §3 (single-tool history) and §4 (cross-tool pivot) mockups: month separator line, dim summary footer, two-zone clipped-bar example with `┊` rule and p95 legend <!-- R5 -->
- [x] T011 [P] Bump `package.json` version 0.9.4 → 0.10.0 <!-- R6 -->

## Execution Order

- T001 → T002 → T003 → T004 (bar-scale chain); T005/T006 independent of the chain
- T007 depends on T001 (needs the scale descriptor for the p95 legend)
- T008–T009 after all implementation tasks; T010/T011 anytime

## Acceptance

### Functional Completeness

- [x] A-001 R1: Daily `renderHistory`/`renderTotalHistory` output contains a dim divider before each month-crossing row and nowhere else; monthly/weekly output contains none
- [x] A-002 R2: The `currentLabel(period)` row's label cell renders in `boldWhite` in both history renderers; all other rows unmarked
- [x] A-003 R3: A dim footer with avg/this-month/peak follows the Total row in both history renderers (≥2 rows, non-compact ANSI only)
- [x] A-004 R4: Two-zone bars activate exactly when `maxCost > 1.5 × p95` (nonzero costs), with aligned `┊` rule, green main zone, yellow overflow zone, and footer p95 legend
- [x] A-005 R5: `docs/specs/layouts.md` §3/§4 mockups show separators, footer, and a clipped-bar example
- [x] A-006 R6: `package.json` version is 0.10.0

### Behavioral Correctness

- [x] A-007 R4: A window with `maxCost ≤ 1.5 × p95` renders byte-identically to the pre-change output (no rule, no legend, no width change)
- [x] A-008 R3: Footer math verified: avg = total/data-row-count; `this month` omitted when no current-month rows and for non-daily periods; peak carries the correct label
- [x] A-009 R2: Marked row's visible width (via `stripAnsi`) equals the unmarked rendering — no layout shift

### Scenario Coverage

- [x] A-010 R1: Test exercises a window spanning a month boundary and asserts separator count and position
- [x] A-011 R4: Test asserts $1,091.67 and $4,031.61 overflow segments differ in length under p95 ≈ $846
- [x] A-012 R4: Rule column index is identical across all rows in a two-zone rendering (via `stripAnsi` column check)

### Edge Cases & Error Handling

- [x] A-013 R4: Window with all-zero costs, or fewer than ~10 nonzero rows where p95 ≈ max, renders single-zone (trigger does not fire)
- [x] A-014 R4: A row at exactly p95 ends at the rule with no overflow segment
- [x] A-015 R1: No separator before the first visible row; separators computed on the post-`maxRows` window
- [x] A-016 R3: CSV and Markdown emitters carry no footer, separators, or marker (existing emitter tests still pass unchanged)

### Code Quality

- [x] A-017 Pattern consistency: New helpers follow the existing functional style, `node:`/`type` import conventions, and the render/print split
- [x] A-018 No unnecessary duplication: `currentLabel`, `renderBar`, `fmtCost`, and the divider construction are reused, not re-implemented
- [x] A-019 No magic numbers: the 1.5 trigger factor, overflow floor (4), and zone ratio are named constants
- [x] A-020 Minimum pathways: single-zone path delegates to the existing `renderBar` code path rather than a parallel implementation

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Deletion Candidates

- None — this change adds new functionality (separators, marker, footer, two-zone scale) without making existing code redundant; the old `maxCost`-scaled `renderBar` call sites were replaced by `renderScaledBar`, which still delegates to `renderBar` on the single-zone path.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | Marker keys on `currentLabel(period)` uniformly (weekly gets a this-week marker for free) | One code path per code-quality "minimum pathways"; intake named daily+monthly, weekly is the same mechanism | S:65 R:85 A:80 D:75 |
| 2 | Confident | Footer unit suffix derives from period (`/day`, `/week`, `/month`); `this month` segment daily-only | Intake specifies daily and monthly; weekly follows the same drop-`this month` rule as monthly | S:60 R:85 A:75 D:70 |
| 3 | Confident | Footer renders only when ≥2 data rows (matches the Total row's own condition) | Intake places the footer "after the Total row", which only renders at ≥2 rows; a 1-row footer restates the row | S:60 R:85 A:80 D:75 |
| 4 | Confident | Zone split: overflow `max(4, round(barWidth/4))`, rule 1 char, main = remainder | Intake marks the exact split "a plan-level decision" with ≈1/4 and min 4 given; formula preserves both zones at the 10-char minimum bar area | S:70 R:85 A:75 D:70 |
| 5 | Confident | New tests live in a new co-located file `formatter-history.test.ts` | `formatter.test.ts` is 1,318 lines; a sibling file follows the existing `formatter-options.test.ts` precedent | S:55 R:90 A:85 D:80 |
| 6 | Certain | Version bump is 0.9.4 → 0.10.0 in this change (q6fx shares it) | Constitution Output Stability mandates minor bump; current tree reads 0.9.4 with q6fx already merged | S:85 R:90 A:95 D:90 |
| 7 | Tentative | `currentLabel` imported from `core/fetcher.ts` into the TUI formatter | No import cycle exists and it avoids duplication, but it couples the display layer to the fetcher module; a future extraction into a shared date-label module is defensible | S:55 R:85 A:70 D:55 |
| 8 | Confident | Scale descriptor computes only when bars render (`showBars`); the p95 legend is suppressed with the bars on narrow terminals | A `┊ = $X (p95)` legend is meaningless without a visible rule; single-zone fallback keeps the footer width-stable | S:55 R:85 A:75 D:65 |
| 9 | Confident | Two-zone bars space-pad to full `barWidth` (main zone pads up to the rule, overflow zone pads to the end) | R4 requires the rule column aligned across rows and total row width unchanged; trailing pad spaces are part of the bar area | S:65 R:85 A:80 D:70 |

9 assumptions (1 certain, 7 confident, 1 tentative).
