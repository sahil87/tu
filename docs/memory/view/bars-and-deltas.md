---
type: memory
description: Bar geometry and watch deltas — eighth-block glyphs (view.Glyphs/Percentile/ComputeScale/Apportion), the p95 two-zone scale with the ┊ rule and trigger 1.5×, BarAfter and leader cells, and the in-cell versus trailing delta placements.
---

# Bars and Deltas

**Domain**: view

## Overview

`internal/view/bar.go` reduces every row's bar to raw block glyphs plus shared `Scale` geometry, and `internal/view/table.go` defines the two delta shapes (in-cell and trailing); [render/ansi](/render/ansi.md) colors, pads and slices the result.

## Requirements

### Requirement: Eighth-block glyphs
`Glyphs(value, max, width)` (`internal/view/bar.go`) MUST render raw block glyphs: `""` when `value == 0` or `max == 0`; `scaled = value/max × width`, `full = floor(scaled)`, `e = round((scaled − full) × 8)` (half up); `e == 8` carries to `"█" × (full+1)`; else `"█" × full + eighths[e]` with `eighths = ["", "▏", "▎", "▍", "▌", "▋", "▊", "▉"]` (U+258F..U+2589); an empty result becomes `"▏"` (`minBar`, U+258F) — the floor for a nonzero sliver. `fullBlock` is `█` (U+2588); the two-zone scale-break rule is `┊` (`scaleRule`, U+250A).

#### Scenario: Sub-cell sliver
- **GIVEN** a value scaling to 0.3 of one cell, and one scaling below 1/16 of a cell
- **WHEN** `Glyphs` renders both
- **THEN** the first renders `▎` (e rounds to 2) and the second the floor `▏`

### Requirement: The p95 two-zone scale
`ComputeScale(values, barWidth)` MUST compute `Max` over all values and `P95 = Percentile(sorted nonzero values, p95Percentile)` — `p95Percentile = 95.0`, linear interpolation `idx = p/100 × (n−1)` over the sorted-ascending sample. Two-zone engages iff `max > p95TriggerFactor × p95` (`p95TriggerFactor = 1.5`), with `OverflowZone = max(overflowZoneMin, round(barWidth / overflowZoneDivisor))` (`overflowZoneMin = 4`, `overflowZoneDivisor = 4`) and `MainZone = barWidth − OverflowZone − 1`; otherwise single-zone. Bars are suppressed below `minBarArea = 10` and capped at `maxBarWidth = 30`. `rowBar` MUST render single-zone `Main = Glyphs(v, Max, Width)`; two-zone `Main = Glyphs(min(v, P95), P95, MainZone)` and `Overflow = Glyphs(v − P95, Max − P95, OverflowZone)` only when `v > P95` — a row at exactly p95 ends at the rule. (oojd)

#### Scenario: Exactly at p95
- **GIVEN** a two-zone scale and a row whose value equals P95 exactly
- **WHEN** `rowBar` renders it
- **THEN** `Main` fills the main zone and `Overflow` is empty — the bar ends at the rule

### Requirement: Segment apportionment sums exactly
`Apportion(shares, total)` MUST distribute `total` glyphs over shares by largest remainder — floor each quota, hand the remainder to the largest fractional parts, ties to the earlier index; a zero share gets zero; the result sums exactly to `total`. The pivot passes its per-visible-tool values (`Bar.Segments`); the single-tool history and the leaderboard pass nil segments (solid fill). (3tah)

### Requirement: Bar placement
`Column.BarAfter` MUST render the bar area — exactly `1 + Scale.Width` visible characters — immediately after that column's cell (the leaderboard's mid-row bar); false everywhere else means the bar trails the last cell. `Scale.Width = 0` suppresses bars and no row carries a `Bar`. (2gbb)

### Requirement: Watch deltas have two shapes
`rowDelta(prev, key, value)` MUST resolve `DeltaUp` when the value grew, `DeltaDown` when it shrank, `DeltaNone` without a map entry or a nil map. The history tables carry the trailing `Row.Delta`; `Table.DeltaSpaced` selects the `" ↑"` form (single-tool history, true) or the space-less `"↑"` form abutting the cell (the pivot, false). The snapshot, the compact tables and the leaderboard carry the in-cell `Cell.Delta`, composed per `Table.DeltaInCell`: `DeltaPadsArrow` pads text + arrow together (the raw-length padding quirk — with color on, the cell renders effectively unpadded); `DeltaAfterPad` pads the text first, then appends the arrow, with the exact-zero Dim wrap covering the composite. (4pze)

### Requirement: Leader cells
`Cell.Leader` wraps the padded cell (after any Dim wrap) in BoldWhite — the lbh per-row leader set on each Data row's first strict maximum over the visible columns (index 0 when all values are equal); Header and Total cells are never Leader or Dim. (2gbb)

## Design Decisions

### Two-zone geometry: overflow = max(4, round(barWidth/4)), rule 1 char, main the remainder
**Decision**: The split derives from `barWidth` — the main zone scales 0→p95, one rule char separates, the overflow zone scales p95→max.
**Why**: At the minimum bars width (10) this leaves main 5 / rule 1 / overflow 4 — both zones stay legible; at the maximum (30) it gives main 21 / overflow 8.
**Rejected**: A percentage-only split with no floor (the overflow zone collapses to 1–2 chars at narrow widths, making large values indistinguishable again — the failure the feature fixes).
*Introduced by*: 260815-oojd-history-month-anchors-p95-bars

### Stacked bars color the already-rendered glyph sequence
**Decision**: The pivot renders the row's bar exactly as the unstacked path, then apportions its runes among the visible tools via `Apportion` (largest remainder) and colors each contiguous run.
**Why**: The segments-sum-to-bar-length invariant becomes structural — stripping ANSI provably yields the unstacked bytes — and the single-glyph primitive stays one code path.
**Rejected**: Rendering each segment independently — per-segment eighths rounding drifts the total length by ±1 char against the unstacked bar.
*Introduced by*: 260816-3tah-stacked-tool-bars-pivot

### Stack palette leads green, assigned positionally
**Decision**: Palette slots are 0 green, 1 magenta, 2 blue, 3 cyan, ≥4 uncolored (`Swatch.Palette` in `internal/view/table.go`), assigned in visible column order — never by cost rank; yellow stays reserved for the overflow zone.
**Why**: A tool's color never changes across windows, days or cost shifts, preserving day-to-day comparability, and the dominant first column matches the single-tool history bar's green identity.
**Rejected**: Assigning green dynamically to the highest-cost tool — colors would swap mid-history whenever another tool takes the lead, and the legend would disagree across renders.
*Introduced by*: 260816-w61k-green-dominant-stack-palette
