# Plan: Green-Dominant Stack Palette

**Change**: 260816-w61k-green-dominant-stack-palette
**Intake**: `intake.md`

## Requirements

### Display: Stacked pivot palette order

#### R1: STACK_PALETTE renders the first visible column dark green
`STACK_PALETTE` in `src/node/tui/formatter.ts` MUST be `[green, magenta, blue, cyan]` (was `[cyan, magenta, blue, green]`), assigned in visible pivot column order as before. The first visible column (Claude Code in the shipped registry order) MUST render dark green (ANSI `\x1b[32m`), restoring the pivot history's visual continuity with the single-tool history bar. Only the array literal (and its adjacent order-describing comment, if it names colors) changes — `stackedBarPalette()`, `apportionSegments()`, `renderStackedScaledBar()`, legend rendering, and all bar geometry MUST be untouched.

- **GIVEN** a pivot history window with Claude Code, Codex, and Kimi visible (3 columns)
- **WHEN** `renderTotalHistory` renders a data row's stacked bar with color enabled
- **THEN** the segments run green → magenta → blue left to right, and the footer legend swatches carry the same colors
- **AND** stripping ANSI yields bytes identical to the unstacked bar (the existing invariant)

- **GIVEN** a 4th visible tool column
- **WHEN** its share renders
- **THEN** its segment and legend swatch are cyan (`\x1b[36m`); a 5th+ tool stays uncolored

- **GIVEN** `--no-color` or `NO_COLOR`
- **WHEN** any pivot history renders
- **THEN** output is byte-identical to before this change

#### R2: Test expectations follow the new palette
`src/node/tui/__tests__/formatter-stacked.test.ts` MUST assert the new order (Constitution Test Integrity: tests conform to spec; the spec is updated by R3). Known touch points:

- Line 28: local mirror `const PALETTE = [cyan, magenta, blue, green]` → `[green, magenta, blue, cyan]`
- "colors segments in palette order": first-slot index moves `\x1b[36m` → `\x1b[32m`; ordering assertion/message becomes green → magenta → blue
- "gives a sub-dollar share no segment": `includes("\x1b[36m")` → `includes("\x1b[32m")` (slot-3 blue exclusion unchanged)
- "lets the fractional-eighths character ride the last nonzero segment": expected `cyan("██") + magenta("▎")` → `green("██") + magenta("▎")`
- "leaves a 5th tool's segment uncolored": palette literal `[cyan, magenta, blue, green, identity]` → `[green, magenta, blue, cyan, identity]` (the 4-colored-runs regex `\x1b\[3[2-6]m` still counts 4)
- "assigns cyan/magenta/blue in column order…": first-slot index and title/message become green-first; the zero-rounded-share blue check is positionally unchanged
- "stacks the whole bar in a two-zone window…": the overflow no-segment-color check MUST track the new slot-1 color (`!overflow.includes("\x1b[32m")` instead of `\x1b[36m`) so it keeps testing what it tested
- Footer legend tests: Claude Code swatch `\x1b[36m█` → `\x1b[32m█`; the dim-continuity check `\x1b[0m\x1b[36m` → `\x1b[0m\x1b[32m`; Codex magenta and Kimi blue swatches unchanged
- "keeps green bars with no stacked palette colors" (single-tool history): assertions happen to remain valid (the exclusion set `{magenta, blue, cyan█}` is still exactly the non-green stacked colors); re-check comments/messages for accuracy since green is now both the single-tool bar color and stack slot 0

- **GIVEN** the updated palette
- **WHEN** `npx tsx --test src/node/tui/__tests__/formatter-stacked.test.ts` runs
- **THEN** all tests pass, and the full suite (`npx tsx --test src/node/**/__tests__/*.test.ts` per the justfile) stays green

#### R3: layouts.md documents the new order
`docs/specs/layouts.md` MUST be updated in all three locations that state the palette, and nowhere else:

- Layout 4 intro prose (~line 107): "the Claude Code share of each row renders cyan" → "renders green"
- Layout 4 stacking bullet (~line 127): fixed palette "**cyan, magenta, blue, green**" → "**green, magenta, blue, cyan**"
- Color Reference table (~lines 353–357): `green` row gains "pivot bar segment (1st tool)" (replacing "(4th tool)"); `cyan` row becomes "pivot bar segment (4th tool)"; `magenta` (2nd) and `blue` (3rd) rows unchanged; the `yellow` overflow-reservation row MUST remain unchanged

- **GIVEN** the updated spec
- **WHEN** the palette statements are compared against `STACK_PALETTE`
- **THEN** every documented order matches `[green, magenta, blue, cyan]` and the yellow overflow reservation is intact

### Non-Goals

- No cost-ranked (dynamic) color assignment — rejected in the intake for color stability
- No change to the yellow overflow zone, bar geometry, apportionment, or legend logic
- No `package.json` version bump in this diff — the Output-Stability bump happens at release time per the established convention (memory: display/formatting Design Decisions)

### Design Decisions

#### Green takes palette slot 0 positionally, not by cost rank
**Decision**: `STACK_PALETTE[0]` is `green`, assigned to the first visible pivot column like every other slot — the "dominant tool gets green" outcome falls out of registry order (Claude Code first), not a cost computation.
**Why**: Restores the pre-stacking dark-green-dominant chart identity while keeping colors deterministic — a tool's color never changes across windows, days, or cost shifts, preserving day-to-day comparability.
**Rejected**: Assigning green dynamically to the highest-cost tool — colors would swap mid-history whenever another tool overtook Claude Code, and the legend would disagree across renders.
*Introduced by*: 260816-w61k-green-dominant-stack-palette

#### Cyan moves to slot 4; blue stays on slot 3
**Decision**: The vacated slot for green's old position is filled as `[green, magenta, blue, cyan]` — Gemini (slot 3) keeps blue, Kimi (slot 4) gets cyan.
**Why**: ANSI blue is the muddiest color on dark backgrounds so it goes to the rarest tool; in the user's real data Kimi appears more often than Gemini, so Kimi gets the more legible cyan. Yellow is excluded — reserved for the overflow zone.
**Rejected**: `[green, magenta, cyan, blue]` (Gemini cyan, Kimi blue) — viable, called "minor either way", but gives the more legible color to the less-used tool.
*Introduced by*: 260816-w61k-green-dominant-stack-palette

## Tasks

### Phase 2: Core Implementation

- [x] T001 Reorder `STACK_PALETTE` in `src/node/tui/formatter.ts` (line ~199) to `[green, magenta, blue, cyan]` and update the adjacent comment if it names color order; no other code changes <!-- R1 -->
- [x] T002 Update color expectations in `src/node/tui/__tests__/formatter-stacked.test.ts` per R2's touch-point list (PALETTE mirror, slot-1 cyan→green assertions, fractional-eighths literal, 5-tool palette literal, overflow no-segment-color check to `\x1b[32m`, legend swatch/dim-continuity checks, title/message text), then run `npx tsx --test src/node/tui/__tests__/formatter-stacked.test.ts` and the full test suite <!-- R2 -->
- [x] T003 [P] Update `docs/specs/layouts.md` in exactly three locations (intro prose ~107, stacking bullet ~127, Color Reference rows ~353–357) to the new order, leaving the yellow reservation untouched <!-- R3 -->

## Acceptance

### Functional Completeness

- [x] A-001 R1: `STACK_PALETTE` in `src/node/tui/formatter.ts` is exactly `[green, magenta, blue, cyan]`; no other symbol in the file changed behavior
- [x] A-002 R1: With color enabled, the first visible pivot column's bar segments and footer legend swatch render `\x1b[32m` (verified by test assertions)

### Behavioral Correctness

- [x] A-003 R1: Stripped-ANSI pivot output is byte-identical to pre-change output (existing invariant tests still pass unmodified in their stripped-bytes assertions)
- [x] A-004 R2: The two-zone overflow test now excludes `\x1b[32m` from the overflow zone (the check tracks the new slot-1 color rather than passing vacuously)

### Scenario Coverage

- [x] A-005 R2: `npx tsx --test src/node/tui/__tests__/formatter-stacked.test.ts` passes; the full suite passes
- [x] A-006 R3: All three `docs/specs/layouts.md` palette statements read `[green, magenta, blue, cyan]`-consistent text; `grep -n "cyan, magenta, blue, green" docs/specs/` returns nothing

### Edge Cases & Error Handling

- [x] A-007 R1: `--no-color`/`NO_COLOR` pivot output is byte-identical to before (existing no-color tests unmodified and passing)
- [x] A-008 R1: Single-tool history (`renderHistory`) still renders solid green with no stacked palette colors (existing test passing, comments accurate)

### Code Quality

- [x] A-009 Pattern consistency: Edits follow surrounding comment style and test idioms; no new code paths introduced
- [x] A-010 No unnecessary duplication: No new helpers or utilities added — the change is data-only (array order) plus expectation updates

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Deletion Candidates

None — this change reorders an existing palette literal and updates test expectations and spec prose; no existing file, symbol, or branch was made redundant or unused.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | Retarget the overflow no-segment-color check from `\x1b[36m` to `\x1b[32m` | The check's intent is "the slot-1 color never leaks into the overflow zone"; keeping cyan would make it vacuous under the new palette | S:70 R:90 A:85 D:75 |
| 2 | Certain | Rename test titles/messages that name colors (e.g. "cyan → magenta → blue") to match the new order | Cosmetic accuracy; titles are not a parser contract | S:85 R:95 A:95 D:90 |
| 3 | Certain | No `package.json` version bump in this diff | Established release-time-bump convention (memory: display/formatting Design Decisions, 260816-3tah) | S:90 R:95 A:95 D:95 |

3 assumptions (2 certain, 1 confident, 0 tentative).
