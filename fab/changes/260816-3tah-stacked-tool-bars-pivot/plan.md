# Plan: Stacked Tool Bars in Pivot History

**Change**: 260816-3tah-stacked-tool-bars-pivot
**Intake**: `intake.md`

## Requirements

### Display: Segmented Bar Fill

#### R1: Per-tool segmented main-zone fill
The pivot's row bar (`renderTotalHistory` in `src/node/tui/formatter.ts`) MUST keep its total length computed exactly as today (row total on the single-zone or two-zone p95 scale), and MUST split the **main-zone** portion of that bar into contiguous per-tool colored segments, left to right in pivot column order, each segment's character count proportional to that tool's share of the row cost.

- Segment character counts MUST be apportioned by **largest-remainder rounding** over the bar's visible character count, so segment lengths always sum exactly to the unstacked bar's length (no drift against today's rendering). Ties in fractional remainder break by column order (earlier column wins) for determinism.
- The fractional-eighths final character (when present) MUST belong to the last (rightmost) segment that received ≥1 character.
- A tool whose share rounds to zero characters MUST get no segment.
- Rows in a single-zone window (no p95 trigger) MUST stack the whole bar under the same apportionment rule.
- `renderBar` MUST stay the single-segment primitive — stacking is layered on top of it, not into it.

- **GIVEN** a daily pivot row with Claude $300, Codex $100, Kimi $0.04 (row total $400.04) whose unstacked bar renders 20 characters
- **WHEN** the stacked bar renders
- **THEN** the bar is still exactly 20 visible characters, split ~15 cyan / ~5 magenta by largest remainder, with Kimi receiving no segment
- **AND** stripping ANSI yields the identical character sequence today's unstacked bar produces

#### R2: Palette assignment in column order
Tool segments MUST be colored from the fixed palette **cyan, magenta, blue, green**, assigned in pivot column order (left → right) to the visible (post-zero-column-filter) tools. A 5th+ visible tool MUST render its segments uncolored (default/white). `src/node/tui/colors.ts` MUST export `magenta` and `blue` helpers in the same style as the existing color functions (they are currently absent).

- **GIVEN** a pivot window where Claude Code, Codex, and Kimi are the visible columns in that order
- **WHEN** stacked bars render
- **THEN** Claude segments are cyan, Codex magenta, Kimi blue
- **GIVEN** five visible tools
- **WHEN** the 5th tool's segment renders
- **THEN** it carries no color code

#### R3: Yellow reserved for overflow; green freed from the generic pivot bar
The overflow zone of a clipped row (the portion past the `┊` rule) MUST render solid yellow with **no** tool segmentation, exactly as today. Green MUST no longer be the generic bar color in the pivot (it is now just palette slot 4). The single-tool history (`renderHistory`) bar MUST keep its green single-segment rendering byte-identical to today.

- **GIVEN** a two-zone window with a clipped row
- **WHEN** its bar renders
- **THEN** the main zone is per-tool segmented and the overflow portion is solid yellow, unsegmented
- **GIVEN** `tu cc h` (single-tool history)
- **WHEN** bars render
- **THEN** output is byte-identical to v0.10.1

### Display: Legend

#### R4: Footer legend for stacked bars
When the pivot renders stacked bars (bars visible AND ≥2 visible tools AND color enabled), the summary footer (`renderHistoryFooter`) MUST append a legend group — one `█` swatch per visible tool in column order, each swatch in the tool's assigned color, followed by the tool name:

```
avg $716.97/day · peak $4,031.61 (2026-06-12) · ┊ = $1,736.43 (p95) · █ Claude Code █ Codex █ Kimi
```

The legend MUST be omitted when color is disabled (`--no-color`/`NO_COLOR` — uncolored swatches carry no information), when bars are suppressed (narrow terminal), or when only one tool column is visible. Non-swatch legend text keeps the footer's dim styling; the per-swatch color resets MUST NOT bleed into or strip the dim styling of the surrounding footer text.

- **GIVEN** a pivot with bars shown and 3 visible tools
- **WHEN** the footer renders
- **THEN** it ends with `· █ Claude Code █ Codex █ Kimi` with each swatch in that tool's palette color
- **GIVEN** the same pivot under `NO_COLOR=1`
- **WHEN** the footer renders
- **THEN** no legend group appears and the footer is byte-identical to today's

### Display: Degradation Invariants

#### R5: No-color, watch, and non-ANSI paths regress nothing
Under `--no-color`/`NO_COLOR`, stacked segments MUST collapse to indistinguishable solid blocks such that byte output differs from v0.10.1 **only** by the absent legend (i.e. not at all, since the legend is omitted). Watch mode MUST reuse the same render path with delta arrows unaffected (arrow green/red is the status channel, not the bar channel). Compact mode, CSV, and Markdown emitters MUST be untouched (no bars there).

- **GIVEN** `tu dh --no-color`
- **WHEN** the pivot renders
- **THEN** output is byte-identical to v0.10.1's `--no-color` output
- **GIVEN** watch mode with `prevCosts` set
- **WHEN** a row's cost changes between polls
- **THEN** the space-less delta arrow renders exactly as today, and `indicatorReserve` bar-width math is unchanged

### Docs: Spec update

#### R6: layouts.md reflects the new bar semantics
`docs/specs/layouts.md` §4 (History — All Tools Pivot) MUST describe the stacked-bar fill and show the legend in its footer mockups, and the `## Color Reference` table MUST re-scope green's row (no longer the generic pivot bar; still single-tool history bars + up-arrow), add `magenta` and `blue` rows, and note yellow's overflow-zone reservation.

- **GIVEN** a reader of layouts.md §4
- **WHEN** they compare the doc against `tu dh` output
- **THEN** the mockup, stacking rules, legend conditions, and color table match the implementation

### Non-Goals

- Stacking the single-tool history bar (`tu cc h`) — nothing to stack, keeps green unchanged
- A legend under `--no-color` (uncolored swatches carry no information)
- Configurable palette or per-tool color pinning — fixed 4-slot palette by user decision
- CSV/Markdown/JSON changes — no bars exist in those formats

### Design Decisions

#### Stack by re-coloring today's exact bar character sequence
**Decision**: The stacked renderer first computes the row's raw bar string exactly as today (`renderBar` against the same scale/zone widths), then apportions that string's visible characters among tools by largest remainder and colors each contiguous run.
**Why**: Reusing the already-rendered character sequence makes the "segments sum exactly to the unstacked bar length" invariant structural — stripping ANSI provably yields today's bytes — and keeps `renderBar` the single-segment primitive.
**Rejected**: Rendering each segment independently via per-tool `renderBar` calls — per-segment eighths rounding drifts the total length by ±1 char against today's bar.
*Introduced by*: 260816-3tah-stacked-tool-bars-pivot

#### Minor version bump happens at release time, not in this change
**Decision**: This change does not edit `package.json`; the Output Stability minor bump is satisfied by cutting the next release as `just release minor` (release commits like `release: v0.10.1` are separate, made by `scripts/release.sh`). The PR body MUST note "requires minor release".
**Why**: The repo's release flow owns the version field; bumping it in a feature PR would collide with `release.sh`.
**Rejected**: Bumping `package.json` to 0.11.0 in this PR — fights the established release automation.
*Introduced by*: 260816-3tah-stacked-tool-bars-pivot

## Tasks

### Phase 1: Setup

- [x] T001 Add `magenta` and `blue` color helpers to `src/node/tui/colors.ts` in the existing `wrap(...)` style (`\x1b[35m`, `\x1b[34m`), and export a color-disabled accessor (e.g. `colorDisabled(): boolean`) so the formatter can gate the legend <!-- R2, R4 -->

### Phase 2: Core Implementation

- [x] T002 Add an exported largest-remainder apportionment helper in `src/node/tui/formatter.ts` (e.g. `apportionSegments(shares: number[], total: number): number[]` — floors quotas, distributes remaining chars by descending fractional remainder, column-order tie-break) next to `percentile`/`computeBarScale` <!-- R1 -->
- [x] T003 Add a stacked bar renderer in `src/node/tui/formatter.ts` (e.g. `renderStackedScaledBar(rowTotal, toolCosts, colorFns, scale, barWidth)`) that reproduces today's raw bar chars per zone, colors main-zone/single-zone runs per T002's apportionment (eighths char rides the last nonzero segment; zero-share tools get no run), and keeps the overflow zone solid yellow unsegmented; `renderBar` and `renderScaledBar` stay untouched as primitives <!-- R1, R3 -->
- [x] T004 Wire `renderTotalHistory` to the stacked renderer: build the per-tool palette `[cyan, magenta, blue, green]` by visible column order with identity fallback for 5th+, collect per-tool row costs alongside the existing `cells` loop, and pass them through; single-tool history path (`renderHistory`) untouched <!-- R1, R2, R3 -->
- [x] T005 Append the legend group in `renderHistoryFooter` (new optional legend parameter passed only by `renderTotalHistory` when bars shown ∧ ≥2 visible tools ∧ color enabled), with the dim/reset interplay handled so swatch colors don't strip dim from surrounding footer text <!-- R4 -->

### Phase 3: Integration & Edge Cases

- [x] T006 Tests in `src/node/tui/__tests__/formatter-stacked.test.ts`: apportionment sums/ties/zero-share properties; stacked bar stripped-ANSI bytes equal today's unstacked bar (single-zone and two-zone, clipped and unclipped rows); column-order palette assignment; 5th-tool uncolored fallback; overflow stays yellow/unsegmented; legend presence and each omission (no-color, narrow, single-tool); single-tool history byte-identical; no-color pivot byte-identical <!-- R1, R2, R3, R4, R5 -->


### Phase 4: Polish

- [x] T007 Update `docs/specs/layouts.md`: §4 pivot bullets + footer mockups gain the stacking rules and legend; `## Color Reference` re-scopes green, adds magenta/blue rows, notes yellow's overflow reservation <!-- R6 -->

## Acceptance

### Functional Completeness

- [x] A-001 R1: Stacked main-zone segments render in column order, apportioned by largest remainder, summing exactly to the unstacked bar length; zero-share tools get no segment; single-zone windows stack the whole bar
- [x] A-002 R2: Palette is cyan/magenta/blue/green by visible column order; 5th+ tool uncolored; `magenta`/`blue` exported from colors.ts
- [x] A-003 R4: Legend renders in the summary footer with correctly colored swatches when bars shown ∧ ≥2 tools ∧ color enabled
- [x] A-004 R6: layouts.md §4 and Color Reference updated to match the implementation

### Behavioral Correctness

- [x] A-005 R1: Stripping ANSI from a stacked bar yields the identical character sequence v0.10.1 renders for the same row (verified by test)
- [x] A-006 R3: Overflow zone renders solid yellow with no segmentation; green no longer wraps the whole pivot bar

### Scenario Coverage

- [x] A-007 R1: Tests cover clipped and unclipped rows in a two-zone window, and a single-zone window, all with the length-preservation property
- [x] A-008 R4: Tests cover legend presence plus all three omission conditions (no-color, bars suppressed, single visible tool)

### Edge Cases & Error Handling

- [x] A-009 R1: Sub-dollar shares (e.g. a $0.04 tool) receive no segment; a 1-char bar (MIN_BAR) goes entirely to the largest-share tool; rows with zero total render no bar exactly as today
- [x] A-010 R5: `--no-color` pivot output is byte-identical to v0.10.1; single-tool history output byte-identical with color on

### Code Quality

- [x] A-011 Pattern consistency: New helpers follow formatter.ts's exported-pure-helper style (percentile/computeBarScale precedent); colors.ts additions match the wrap() idiom; `node:` imports and type imports where applicable
- [x] A-012 No unnecessary duplication: Stacking reuses `renderBar`/`computeBarScale` primitives rather than reimplementing bar math; no duplicated color plumbing
- [x] A-013 No magic numbers: Palette table and any new geometry constants are named constants
- [x] A-014 Minimum pathways: One stacked render path used by both single-zone and two-zone pivot rows, not two parallel implementations

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`
- The PR body must note: **requires minor release** (`just release minor`) per the Output Stability rule — the version bump itself is release-time, not part of this diff.

## Deletion Candidates

None — this change adds new functionality without making existing code redundant. `renderScaledBar` remains the single-tool history primitive (src/node/tui/formatter.ts:406), `renderBar` stays the shared char-sequence primitive, and `green` is still used by the single-tool bar and the up-arrow delta. The pivot's old solid-green fill was re-colored in place, not superseded by a parallel renderer that strands the old one.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | Stack by re-coloring today's exact bar char sequence, not per-segment renderBar calls | Makes the length invariant structural; avoids eighths rounding drift | S:70 R:85 A:85 D:75 |
| 2 | Confident | No package.json edit; minor bump satisfied at release time via `just release minor`, noted in PR body | `scripts/release.sh` owns the version field (release commits are separate in git history) | S:55 R:90 A:80 D:70 |
| 3 | Confident | Legend gated on a new exported color-disabled accessor from colors.ts | `isColorDisabled` is currently private; exporting an accessor beats duplicating the NO_COLOR check | S:60 R:90 A:85 D:75 |
| 4 | Confident | Largest-remainder ties break by column order (earlier wins) | Intake doesn't specify; deterministic and matches left-to-right reading order | S:45 R:90 A:80 D:70 |
| 5 | Certain | New helpers exported from formatter.ts beside percentile/computeBarScale | Existing pattern for testable bar-math helpers | S:80 R:90 A:95 D:90 |
| 6 | Confident | The footer legend lists every visible tool, including a 5th+ tool with an uncolored swatch | R4 says one swatch per visible tool; an uncolored swatch matches that tool's uncolored segments | S:55 R:85 A:80 D:70 |

6 assumptions (1 certain, 5 confident, 0 tentative).
