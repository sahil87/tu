# Intake: Stacked Tool Bars in Pivot History

**Change**: 260816-3tah-stacked-tool-bars-pivot
**Created**: 2026-08-16

## Origin

Conversational — a follow-up `/fab-discuss` output-DX review of `tu dh` on v0.10.1. The agent's finding 2:

> The bar shows the total but hides the mix. Now that three tools have real cost in the same window, the row bar could be a stacked bar — per-tool colored segments within the same length — so the August Claude/Codex/Kimi composition is readable without scanning cells.

The agent presented a mock and a color-budget tension (green = "bar", yellow = "overflow" were both taken). The user settled the design explicitly:

> We can do (a) — reassign green. Keep only yellow for overflow. Then you are free to use cyan / magenta / blue / green for whichever the top 4 columns are supposed to be.

## Why

1. **Pain point**: The pivot's inline bar renders only the row total. With multiple tools now carrying real cost in the same window, composition ("which tool drove Friday's spike?") requires scanning 3–4 numeric cells per row; the bar answers "how much" but not "of what".
2. **Consequence of not fixing**: The single most glanceable element of the table under-uses the data directly to its left; composition questions send users back to cell-by-cell reading.
3. **Why this approach**: Segmenting the existing bar keeps total length — and therefore day-to-day comparability and the two-zone p95 scale — fully intact; only the fill gains meaning. The palette decision (reassign green to a tool; yellow stays overflow-only) was made by the user.

## What Changes

All in `src/node/tui/formatter.ts`, `renderTotalHistory` (the ANSI pivot) unless noted.

### 1. Per-tool segmented bar fill

The row bar's length is computed exactly as today (row total on the two-zone p95 scale). The **main-zone** portion is then split into contiguous per-tool segments, left to right in pivot column order, each segment's length proportional to that tool's share of the row cost:

- Segment lengths use largest-remainder rounding over the main-zone character count, so segments always sum exactly to the total bar length (no drift against today's rendering). The fractional-eighths final character applies only to the last (rightmost) segment.
- Tools whose share rounds to zero characters get no segment (honest for sub-dollar costs like a $0.04 day).
- Rows in a single-zone window (no p95 trigger) stack the whole bar the same way.

### 2. Color assignment

- Tool segment palette, assigned in pivot **column order (left → right)** to the visible (post-zero-column-filter) tools: **cyan, magenta, blue, green** — per the user's decision. A 5th+ visible tool renders its segments in default/white (no color); with the zero-column filter, >4 active tools in one window is rare.
- **Yellow is reserved exclusively for the overflow zone**: the portion of a clipped row's bar past the `┊` rule renders solid yellow with no tool segmentation (the compressed scale makes proportional segments misleading there; yellow keeps its single meaning of "beyond scale").
- **Green is no longer the generic bar color in the pivot.** The single-tool history (`tu cc h`) bar is out of scope and keeps green unchanged — it has nothing to stack and no legend.
- `colors.ts` already exports `cyan`, `green`, `red`, `yellow`; add `magenta` and `blue` helpers in the same style if absent.

### 3. Legend

When the pivot renders stacked bars (bars visible and ≥2 tools), append a legend group to the existing summary footer, one colored `█` swatch per visible tool in column order:

```
avg $716.97/day · peak $4,031.61 (2026-06-12) · ┊ = $1,736.43 (p95) · █ Claude Code █ Codex █ Kimi
```

(each `█` in the tool's assigned color). When color is disabled (`--no-color`/`NO_COLOR`), omit the legend group — uncolored swatches carry no information. When bars are suppressed (narrow terminal), omit it too.

### 4. Degradation

- `--no-color`: segments are indistinguishable solid blocks — the bar reads exactly as today's total bar; byte output differs only by the absent legend. No information regression.
- Watch mode: the pivot is re-rendered per poll with the same code path; delta arrows (green ↑ / red ↓) are unaffected — arrow green is a shared status color, not the bar channel.
- Compact mode, CSV, Markdown emitters: untouched (no bars there).

### 5. Versioning

The footer line gains a legend group and bar color semantics change → minor version bump per the constitution's Output Stability rule.

## Affected Memory

- `display/formatting`: (modify) stacked-bar fill rule, tool palette assignment, yellow-overflow reservation, footer legend

## Impact

- `src/node/tui/formatter.ts` — segmented bar builder, palette table, legend in the footer builder; `renderBar` stays the single-segment primitive
- `src/node/tui/colors.ts` — add `magenta`/`blue` if absent
- Formatter tests — segment lengths sum to the unstacked bar length (largest-remainder property), column-order palette assignment, zero-share tools get no segment, overflow stays yellow/unsegmented, legend presence/omission (no-color, narrow, single-tool), 5th-tool fallback
- `docs/specs/layouts.md` — §4 pivot mockup + Color Reference table (green's row re-scoped, magenta/blue added)
- Version: minor bump required

## Open Questions

*(none)*

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Palette cyan/magenta/blue/green by column order; yellow overflow-only; green freed from generic pivot bar | Explicit user decision in discussion | S:85 R:85 A:90 D:90 |
| 2 | Confident | Overflow zone renders solid yellow, unsegmented | Compressed scale makes proportional segments misleading; keeps yellow's single meaning — consistent with "keep only yellow for overflow" | S:65 R:85 A:80 D:70 |
| 3 | Confident | Largest-remainder rounding; eighths only on the last segment | Preserves exact total bar length vs today; simplest correct apportionment | S:55 R:85 A:85 D:75 |
| 4 | Confident | Legend rides the summary footer, omitted under no-color/narrow/single-tool | Footer is the established metadata line (p95 legend precedent); uncolored swatches are noise | S:60 R:85 A:80 D:75 |
| 5 | Confident | Single-tool history bar keeps green, unchanged | Nothing to stack; user's palette decision scoped to pivot columns | S:60 R:85 A:80 D:75 |
| 6 | Tentative | 5th+ visible tool falls back to default/white segments | Palette has 4 slots by user decision; fallback choice (white vs reusing colors) is a judgment call, rare in practice post-zero-column-filter | S:50 R:85 A:65 D:55 |
| 7 | Certain | Minor version bump | Footer content and color semantics change parseable/stable output | S:85 R:90 A:95 D:95 |

7 assumptions (2 certain, 4 confident, 1 tentative, 0 unresolved).
<!-- assumed: 5th+ visible tool renders default/white segments — palette deliberately capped at 4 per user decision; fallback color is a tunable judgment call -->
