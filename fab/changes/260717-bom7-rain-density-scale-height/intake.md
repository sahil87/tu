# Intake: Rain Density Scales with Rain-Zone Height

**Change**: 260717-bom7-rain-density-scale-height
**Created**: 2026-07-18

## Origin

> Send a follow on fix - right now on tall screens the density of the rain is very low - I think the density currently isn't linked to window height

Follow-on to `260717-h9h7-watch-rain-polish` (PR #46, merged as `916e511`, released in v0.8.1). The user observed the sparsity on tall screens after the polish change shipped. The diagnosis was verified in code before this intake was created: drop population is sized from width alone. Interaction mode: one-shot; the fix approach below was designed by the agent from the verified root cause (no further user discussion).

## Why

1. **Problem**: In `RainState.initDrops()` (`src/node/tui/rain.ts:60`), the active drop count is `Math.round(this.cols * DENSITY)` with `DENSITY = 0.3` — width-only, at most one drop per column (the shuffled-columns pick guarantees uniqueness). Trail lengths are fixed at 3-8 (`MIN_LENGTH`/`MAX_LENGTH`). So the fraction of rain-zone cells occupied is roughly `DENSITY × avgTrailLen / rows ≈ 1.65 / rows` — **inversely proportional to zone height**. At a 15-row zone that is ~11% coverage; at a 60-row zone (a tall terminal with modest content — `availableRainRows = termRows - contentHeight - footerRow` in `Compositor.setupRainZone`, compositor.ts:299) it collapses to ~2.75%. Tall screens show a nearly empty rain zone with a handful of short drops.
2. **If not fixed**: the taller the terminal, the worse watch mode looks — precisely the screens with the most room for the ambient effect show the least of it.
3. **Approach**: scale the *number of drops* with zone height (allowing multiple drops per column), rather than lengthening trails or speeding drops up — this preserves the calibrated look (trail lengths, speeds, per-drop behavior all unchanged) and is a minimal, contained change to `initDrops()`. The existing render/clear logic already supports multiple drops per column: cell positions are tracked in a `"row,col"`-keyed Set, overlapping writes are last-wins, and clears fire only for vacated cells — no renderer changes needed.

## What Changes

### Height-scaled drop count (`src/node/tui/rain.ts`)

- Add two module constants next to `DENSITY`:
  ```ts
  const DENSITY_REF_ROWS = 20;   // zone height the current DENSITY was calibrated at
  const MAX_DENSITY_SCALE = 3;   // cap so very tall zones don't produce unbounded per-frame output
  ```
- Add an exported pure helper (testability seam — `RainState`'s drop array is private):
  ```ts
  export function computeActiveDropCount(cols: number, rows: number): number {
    const heightScale = Math.min(MAX_DENSITY_SCALE, Math.max(1, rows / DENSITY_REF_ROWS));
    return Math.round(cols * DENSITY * heightScale);
  }
  ```
- `initDrops()` uses it: `const activeCount = computeActiveDropCount(this.cols, this.rows);`
- **Column assignment for counts above `cols`**: keep the existing Fisher-Yates shuffle of column indices, but assign drops round-robin over it — `this.drops.push(this.createDrop(columns[i % this.cols], true))` for `i < activeCount` — so every column is used once before any column gets a second drop, and extra drops distribute evenly. The current `i < columns.length` loop guard is removed (it exists only because activeCount could never exceed `cols` before).
- **Behavior at or below the reference height is unchanged**: `Math.max(1, rows / 20)` clamps the scale to 1 for zones ≤ 20 rows, so the common laptop-terminal case renders byte-identically to today (same count, same one-drop-per-column outcome for DENSITY = 0.3).
- Scaling summary: 40-col zone → 12 drops at ≤20 rows (today's behavior everywhere), 24 at 40 rows, 36 at 60+ rows (capped at 3×). Coverage at 60 rows goes from ~2.75% back to ~8% — the calibrated look.
- Everything else is untouched: `tick()`, `render()`, `resize()` (which re-runs `initDrops()` and picks up the new count automatically), respawn (same column), scatter initialization (already scatters over the full height), speeds, trail lengths, shimmer, right-margin mode (its zone height is the content height, typically under the reference — unaffected in practice).

### Testing (`src/node/tui/__tests__/rain.test.ts`)

Extend the existing suite (which already stubs `Math.random` with the seeded `mulberry32` pattern):

- `computeActiveDropCount`: at/below reference (`rows ≤ 20`) equals `Math.round(cols * 0.3)` — regression guard for today's behavior; linear region (`cols=40, rows=40` → 24); cap (`cols=40, rows=60` → 36, and `rows=200` → still 36).
- Multi-drop columns: construct a tall `RainState` (e.g. 10×60), run several `tick()`/`render(1)` cycles, and assert output stays within zone bounds (existing bounds-assertion pattern) and renders without error; optionally assert occupied-cell count exceeds the old single-drop-per-column ceiling.

## Affected Memory

- `watch-mode/tui`: (modify) Rain requirements/design decisions: drop count now scales with rain-zone height (reference 20 rows, capped at 3×; `computeActiveDropCount`), multiple drops may share a column on tall zones; at or below 20 rows behavior is unchanged.

## Impact

- **Code**: `src/node/tui/rain.ts` only (~15 lines). `compositor.ts`, `watch.ts` untouched.
- **Tests**: additions to `src/node/tui/__tests__/rain.test.ts`; no existing test changes expected (current tests use zones ≤ 20 rows or exercise behavior independent of drop count).
- **Behavior/output**: watch-mode-only, cosmetic; zones ≤ 20 rows render identically. No table/JSON/non-watch output changes (Output Stability constraint untouched).
- **Docs**: `docs/memory/watch-mode/tui.md` via hydrate.

## Open Questions

- None — root cause verified in code; the approach is a contained, tunable change.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Root cause is width-only drop sizing: `activeCount = Math.round(cols * DENSITY)` at rain.ts:60, one drop per column max; coverage ≈ 1.65/rows | Verified by reading rain.ts in this session; matches the user's hypothesis | S:90 R:95 A:95 D:90 |
| 2 | Confident | Fix by scaling drop count with rows (multi-drop columns, round-robin over the shuffled column list), not by lengthening trails or changing speeds | Preserves the calibrated per-drop look; renderer already handles shared columns; minimal contained diff | S:75 R:90 A:85 D:70 |
| 3 | Confident | `DENSITY_REF_ROWS = 20` as the calibration baseline, with `Math.max(1, …)` so zones ≤ 20 rows are byte-identical to today | 20 approximates the typical below-content zone on a standard terminal; clamp guarantees no regression for the common case; one-line retune | S:60 R:95 A:75 D:65 |
| 4 | Confident | Cap the height scale at 3× (`MAX_DENSITY_SCALE = 3`) | Bounds per-frame ANSI output on very tall terminals — stdout backpressure work was explicitly deferred from the prior change, so unbounded growth would reopen that risk; at the cap, coverage matches the calibrated baseline for ~60-row zones | S:60 R:95 A:80 D:70 |
| 5 | Confident | Expose the count formula as an exported pure `computeActiveDropCount(cols, rows)` | `drops` is private; a pure helper is the cheapest test seam and keeps `initDrops()` thin; internal API, reshapeable at apply | S:55 R:90 A:80 D:70 |

5 assumptions (1 certain, 4 confident, 0 tentative, 0 unresolved).
