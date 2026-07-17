# Plan: Rain Density Scales with Rain-Zone Height

**Change**: 260717-bom7-rain-density-scale-height
**Intake**: `intake.md`

## Requirements

### Rain Animation: Height-Scaled Drop Count

#### R1: Drop count scales with rain-zone height
The active drop count in `RainState.initDrops()` SHALL scale with the rain-zone height (`rows`) in addition to width (`cols`), so that taller zones render proportionally more drops and maintain the calibrated coverage of the ambient rain effect.

- **GIVEN** a rain zone of `cols` columns and `rows` rows taller than the reference height
- **WHEN** `RainState` initializes (or re-initializes on resize) its drops
- **THEN** the active drop count is `Math.round(cols * DENSITY * heightScale)` where `heightScale = Math.min(MAX_DENSITY_SCALE, Math.max(1, rows / DENSITY_REF_ROWS))`
- **AND** a 40-column zone yields 12 drops at ≤20 rows, 24 at 40 rows, and 36 at 60+ rows

#### R2: Behavior at or below the reference height is unchanged
The scaling SHALL be clamped to a minimum of 1× for zones at or below `DENSITY_REF_ROWS` (20 rows), so that the common laptop-terminal case renders byte-identically to the prior width-only behavior.

- **GIVEN** a rain zone with `rows ≤ 20` (`DENSITY_REF_ROWS`)
- **WHEN** the drop count is computed
- **THEN** it equals `Math.round(cols * DENSITY)` — the pre-change count
- **AND** each column receives at most one drop (existing one-drop-per-column outcome)

#### R3: Scale is capped for very tall zones
The height scale SHALL be capped at `MAX_DENSITY_SCALE` (3×) so per-frame ANSI output stays bounded on very tall terminals (stdout backpressure was explicitly deferred from the prior change).

- **GIVEN** a rain zone with `rows` far above `3 × DENSITY_REF_ROWS` (e.g. 200 rows)
- **WHEN** the drop count is computed
- **THEN** the height scale saturates at 3 and the count equals `Math.round(cols * DENSITY * 3)` — identical to the count at 60 rows

#### R4: Extra drops distribute evenly across columns (multi-drop columns)
When the active drop count exceeds the column count, `initDrops()` SHALL assign drops round-robin over the shuffled column list, so every column is used once before any column receives a second drop, and extra drops spread evenly.

- **GIVEN** a tall zone where `activeCount > cols`
- **WHEN** `initDrops()` populates the drop array
- **THEN** drops are assigned to `columns[i % cols]` for `i` in `0..activeCount-1`, and the loop is no longer guarded by `i < columns.length`
- **AND** the existing render/clear logic (positions keyed by `"row,col"`, last-wins overlaps, clears only for vacated cells) handles shared columns without renderer changes

#### R5: Drop-count formula is an exported pure helper
The count formula SHALL be exposed as an exported pure function `computeActiveDropCount(cols, rows)` in `rain.ts`, providing a test seam (the `drops` array is private) and keeping `initDrops()` thin.

- **GIVEN** the module `src/node/tui/rain.ts`
- **WHEN** a test imports `computeActiveDropCount`
- **THEN** it is an exported pure function returning the active drop count for given `cols`/`rows`, with no side effects
- **AND** `initDrops()` calls it via `const activeCount = computeActiveDropCount(this.cols, this.rows);`

### Non-Goals

- Lengthening trails, changing drop speeds, or otherwise altering per-drop behavior — the fix scales *count* only, preserving the calibrated look.
- Any change to `tick()`, `render()`, `resize()` logic, respawn, scatter, shimmer, or right-margin mode.
- Any change to `compositor.ts` or `watch.ts`.
- stdout backpressure handling (deferred; bounded here via the 3× cap).

### Design Decisions

1. **Scale count, not trails/speeds**: multiply the drop count by a height-derived factor — *Why*: preserves the calibrated per-drop look; the renderer already handles shared columns; minimal contained diff — *Rejected*: lengthening trails or speeding drops (changes the visual character).
2. **Reference height of 20 rows with `Math.max(1, …)` clamp**: `DENSITY_REF_ROWS = 20` — *Why*: approximates the typical below-content zone on a standard terminal; the clamp guarantees byte-identical output for the common case — *Rejected*: no clamp (would regress the common small-zone case).
3. **Cap at 3×**: `MAX_DENSITY_SCALE = 3` — *Why*: bounds per-frame ANSI output on very tall terminals; at the cap coverage matches the calibrated baseline for ~60-row zones — *Rejected*: unbounded scale (reopens the deferred backpressure risk).
4. **Exported pure `computeActiveDropCount`**: — *Why*: `drops` is private, so a pure helper is the cheapest test seam and keeps `initDrops()` thin — *Rejected*: testing via render-output inspection only (indirect, brittle for count assertions).

## Tasks

### Phase 2: Core Implementation

- [x] T001 Add `DENSITY_REF_ROWS = 20` and `MAX_DENSITY_SCALE = 3` module constants next to `DENSITY` in `src/node/tui/rain.ts`, and add the exported pure `computeActiveDropCount(cols, rows)` helper returning `Math.round(cols * DENSITY * Math.min(MAX_DENSITY_SCALE, Math.max(1, rows / DENSITY_REF_ROWS)))` <!-- R1 R2 R3 R5 -->
- [x] T002 Update `RainState.initDrops()` in `src/node/tui/rain.ts` to compute `const activeCount = computeActiveDropCount(this.cols, this.rows);`, and change the drop-assignment loop to round-robin over the shuffled columns — `this.drops.push(this.createDrop(columns[i % this.cols], true))` for `i < activeCount`, removing the `i < columns.length` guard <!-- R4 -->

### Phase 3: Integration & Edge Cases

- [x] T003 [P] Add `computeActiveDropCount` unit tests to `src/node/tui/__tests__/rain.test.ts`: at/below reference (`rows ≤ 20` equals `Math.round(cols * 0.3)` — regression guard), linear region (`cols=40, rows=40` → 24), and cap (`cols=40, rows=60` → 36; `cols=40, rows=200` → still 36) <!-- R1 R2 R3 R5 -->
- [x] T004 [P] Add a height-scaling render test to `src/node/tui/__tests__/rain.test.ts`: construct a tall `RainState` (e.g. 40×60), run several `tick()`/`render(1)` cycles, assert output stays within zone bounds (existing bounds-assertion pattern) and renders without error, and assert the tall zone occupies more distinct cells than a same-width reference-height zone (40×20) — the observable effect of the scaled-up drop count feeding the round-robin assignment <!-- R4 -->

## Execution Order

- T001 blocks T002 (initDrops calls the new helper) and T003 (tests import the helper).
- T003 and T004 are independent test additions; both depend on T001/T002 being in place.

## Acceptance

### Functional Completeness

- [x] A-001 R1: `initDrops()` sizes the active drop count from both `cols` and `rows` via `computeActiveDropCount`, with the documented 12/24/36 counts for a 40-col zone at 20/40/60 rows
- [x] A-002 R5: `computeActiveDropCount(cols, rows)` is exported from `rain.ts` as a pure function and is called by `initDrops()`

### Behavioral Correctness

- [x] A-003 R2: For `rows ≤ 20`, `computeActiveDropCount` returns `Math.round(cols * 0.3)` — the pre-change count — and at-or-below-reference output is unchanged (one drop per column)
- [x] A-004 R4: When `activeCount > cols`, drops are assigned round-robin (`columns[i % cols]`) with the `i < columns.length` guard removed, so columns fill once before doubling up *(verified by code inspection, rain.ts:81-83 — shipped constants never produce `activeCount > cols` (max is `round(0.9·cols)`), per Assumption 4)*

### Scenario Coverage

- [x] A-005 R1 R2 R3: `computeActiveDropCount` unit tests cover the at/below-reference regression guard, the linear region (40×40 → 24), and the cap (40×60 → 36, 40×200 → 36)
- [x] A-006 R4: A height-scaling render test exercises a tall zone (40×60) over several tick/render cycles, asserting in-bounds output and more distinct occupied cells than a same-width reference-height zone (40×20) *(review should-fix resolved post-review: a distinct-occupied-columns assertion was added — pre-fix sizing can never exceed 12 distinct columns in a 40-col zone; verified to fail against neutralized height scaling and pass against the fix)*

### Edge Cases & Error Handling

- [x] A-007 R3: Very tall zones saturate the height scale at 3× rather than growing per-frame output unbounded

### Code Quality

- [x] A-008 Pattern consistency: New code follows `rain.ts` conventions — module-level `const` named constants, standalone pure functions, `export function` matching the existing style; tests follow the seeded `mulberry32` pattern already in `rain.test.ts`
- [x] A-009 No unnecessary duplication: The count formula lives in one exported helper reused by `initDrops()` and the tests, not duplicated
- [x] A-010 No magic numbers: The reference height and cap are named constants (`DENSITY_REF_ROWS`, `MAX_DENSITY_SCALE`), not inline literals
- [x] A-011 Functional style: No classes introduced; the helper is a standalone pure function consistent with the codebase's functions-and-objects convention

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Deletion Candidates

None — this change adds new functionality without making existing code redundant. (The one thing it obsoleted — the `i < columns.length` loop guard in `initDrops()` — was removed within this same diff; `DENSITY` remains in use by the new helper, and no other code computes drop counts.)

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Root cause is width-only drop sizing (`activeCount = Math.round(cols * DENSITY)`, one drop/column max); fix scales count with rows | Verified in `rain.ts:60`; matches user hypothesis and the intake's verified diagnosis | S:90 R:95 A:95 D:90 |
| 2 | Certain | Constants, helper signature, and `initDrops()` loop change taken verbatim from the intake's "What Changes" section (`DENSITY_REF_ROWS=20`, `MAX_DENSITY_SCALE=3`, round-robin `columns[i % this.cols]`) | Intake specifies exact code; apply reproduces it — no reinterpretation | S:95 R:90 A:95 D:95 |
| 3 | Confident | Tests use the existing seeded `mulberry32` PRNG pattern and bounds-assertion helpers already in `rain.test.ts` | Constitution/code-quality mandate following existing project patterns; the file already establishes this seam | S:85 R:95 A:90 D:85 |
| 4 | Confident | R4 render test asserts a 40×60 zone occupies more distinct cells than a same-width 40×20 zone (rather than "drops-per-column > 1"), because with DENSITY=0.3 and the 3× cap `activeCount` (≤ 0.9·cols) never exceeds `cols` — multi-drop-per-column is a capability the guard removal enables, not one the shipped constants trigger | Verified the count math; the observable, non-flaky effect of scaling is higher total cell occupancy, which the render test measures directly | S:70 R:90 A:85 D:70 |

4 assumptions (2 certain, 2 confident, 0 tentative).
