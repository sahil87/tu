# Intake: Redesign Watch Layout

**Change**: 260306-mxla-redesign-watch-layout
**Created**: 2026-03-06
**Status**: Draft

## Origin

> Redesign watch mode layout: remove sparkline, move session stats to top as 2x3 grid, regular table below, rain in background at 75% speed, remove wide/medium breakpoint distinction.

Arose from a `/fab-discuss` session reviewing a live screenshot. The side panel (sparkline + stats) felt visually disconnected from the main table, and the vertical space allocation was ~80% rain / 20% content. The redesign simplifies the layout to: stats on top → table → rain fills remaining space.

## Why

The current watch mode layout has structural problems:

1. **Disconnected side panel**: The sparkline and session stats float to the right with a 3-char gutter, no visual container. They feel like two separate things that happen to be on the right rather than a cohesive unit.
2. **Sparkline adds little value**: 3 rows of braille (12 vertical dots) over a wide cost range produces a nearly flat line. The history table already shows trend data. The sparkline is decorative overhead.
3. **Wasted vertical space**: The table takes ~6 rows, the side panel ~8, and rain fills the remaining ~35 rows. That's 80% rain — the content is dwarfed by the animation.
4. **Unnecessary complexity**: The wide (≥113) / medium (60-112) / narrow (<60) breakpoint system exists solely for the side panel. Without it, two breakpoints suffice.

The redesign places useful information (stats) at the top where the eye lands first, uses the standard table below, and lets rain fill remaining space as ambient background at a calmer pace.

## What Changes

### 1. Remove sparkline

Delete `src/node/tui/sparkline.ts` and its test file. Remove sparkline rendering from the side panel in `src/node/tui/panel.ts`. Remove the SparkPanel from the compositor. Remove the history cache that feeds the sparkline (5-minute TTL cache in watch.ts).

### 2. Session stats as 2x3 grid on top

Replace the vertical stats list with a horizontal 2-column, 3-row grid rendered above the table:

```
 Elapsed  5m 32s     Tok/min   ~12,345
 Session  +$0.50     Rate      ~$1.25/hr
                     Proj. day ~$15.00
```

- **Left column** (session-related): Elapsed, Session cost delta
- **Right column** (cost-related): Tok/min, Rate, Proj. day
- Left column has a blank cell at row 3 (only 2 session stats vs 3 cost stats)
- Same styling: labels `dim`, values `boldWhite`, Rate in `yellow`
- Stats that aren't available yet (e.g., Rate before 2 polls) are omitted; the grid contracts vertically

### 3. Regular table below stats

The table rendered in watch mode is identical to non-watch mode output — same `renderTotal`, `renderHistory`, `renderTotalHistory` calls. No side-by-side merge. The compositor no longer needs `mergeSideBySide()`.

### 4. Matrix rain at 75% refresh rate

Change rain tick interval from 80ms to ~107ms (80 / 0.75 ≈ 107). This makes the rain calmer and less "screensaver," more ambient. Rain still fills vertical space below content as before.

### 5. Simplify terminal breakpoints

Remove the three-tier system (wide ≥113 / medium 60-112 / narrow <60). Replace with two tiers:
- **Full** (≥60 cols): stats grid + full table + rain
- **Compact** (<60 cols): compact table only, no stats grid, no rain

The 113-col threshold and side panel logic are removed entirely.

## Affected Memory

- `watch-mode/tui`: (modify) Remove sparkline and side panel requirements, add stats grid layout, update breakpoints, update rain tick interval
- `display/formatting`: (modify) Note that watch mode now uses the same render functions as non-watch mode without side-by-side merge

## Impact

- **`src/node/tui/sparkline.ts`**: Deleted entirely
- **`src/node/tui/panel.ts`**: Rewritten — stats grid rendering replaces vertical list + sparkline
- **`src/node/tui/compositor.ts`**: Simplified — remove SparkPanel, remove `mergeSideBySide()`, stats grid becomes a new panel rendered above table
- **`src/node/tui/watch.ts`**: Remove sparkline history cache, update panel composition order, update rain tick constant
- **`src/node/tui/rain.ts`**: Tick interval constant change (80 → 107)
- **Tests**: Panel tests, compositor tests need updates; sparkline tests deleted
- **Layouts spec** (`docs/specs/layouts.md`): Sections 5, 6 need rewriting to reflect new layout

## Open Questions

None — all design decisions were resolved in the discussion session.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Remove sparkline entirely | Discussed — user decided it adds little value at braille resolution | S:95 R:85 A:90 D:95 |
| 2 | Certain | Stats grid is 2x3: session-related left, cost-related right | Discussed — user specified this exact grouping | S:95 R:90 A:90 D:95 |
| 3 | Certain | Stats grid renders above the table, not beside it | Discussed — user said "show on top" | S:95 R:90 A:90 D:95 |
| 4 | Certain | Two terminal breakpoints: ≥60 full, <60 compact | Discussed — user confirmed "no need to differentiate" wide vs medium | S:95 R:85 A:90 D:95 |
| 5 | Certain | Rain tick changes from 80ms to ~107ms | Discussed — user specified "75% of current refresh rate" | S:90 R:95 A:90 D:95 |
| 6 | Confident | Delete sparkline.ts file entirely (not just stop using it) | No other consumer exists; dead code should be removed | S:80 R:80 A:85 D:90 |
| 7 | Confident | Remove `mergeSideBySide()` from compositor | No side panel means no side-by-side merge needed | S:80 R:80 A:85 D:85 |
| 8 | Tentative | No visual separator (dim rule or blank line) between stats grid and table | Not explicitly discussed — a blank line may be sufficient | S:50 R:95 A:60 D:55 |
<!-- assumed: No separator between stats grid and table — not discussed, defaulting to blank line which is easily changed -->

8 assumptions (5 certain, 2 confident, 1 tentative, 0 unresolved).
