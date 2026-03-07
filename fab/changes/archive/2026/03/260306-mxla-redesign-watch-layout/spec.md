# Spec: Redesign Watch Layout

**Change**: 260306-mxla-redesign-watch-layout
**Created**: 2026-03-06
**Affected memory**: `docs/memory/watch-mode/tui.md`, `docs/memory/display/formatting.md`

## Non-Goals

- Changing non-watch output (snapshot, history, pivot tables in normal mode) — this is watch-mode-only
- Modifying the data fetching or caching pipeline — only layout and rendering changes
- Changing the rain character set, density, or trail behavior — only the tick interval changes

## Watch Mode: Stats Grid

### Requirement: Stats Grid Layout

The watch mode stats area SHALL render as a 2-column, 3-row grid positioned above the table. The left column SHALL contain session-related stats (Elapsed, Session cost delta). The right column SHALL contain cost-related stats (Tok/min, Rate, Proj. day). Row 3 of the left column SHALL be blank (2 session stats vs 3 cost stats).

#### Scenario: Full stats grid with all data available
- **GIVEN** watch mode is running with 2+ completed poll cycles
- **WHEN** the compositor renders the stats grid
- **THEN** the grid SHALL display 3 rows with left and right columns:
  - Row 1: `Elapsed {time}` | `Tok/min ~{value}`
  - Row 2: `Session +${delta}` | `Rate ~${rate}/hr`
  - Row 3: (blank) | `Proj. day ~${value}`
- **AND** labels SHALL be styled `dim`, values `boldWhite`, Rate value in `yellow`

#### Scenario: Stats before 2 polls (unavailable stats show dashes)
- **GIVEN** watch mode has completed 0 or 1 poll cycles
- **WHEN** the stats grid renders
- **THEN** unavailable stats (Tok/min, Rate, Proj. day, Session delta) SHALL show `--` as placeholder
- **AND** the grid SHALL remain fixed at 3 rows to avoid layout shift

#### Scenario: Loading skeleton stats
- **GIVEN** watch mode has just entered the alternate screen buffer and no data has been fetched
- **WHEN** the skeleton renders
- **THEN** Elapsed SHALL show `0s`, Session SHALL show `$0.00`, and right-column stats SHALL show `--`
- **AND** the grid SHALL be 3 rows with the dim horizontal rule below

### Requirement: Stats Grid Separator

A dim horizontal rule (`───`) SHALL separate the stats grid from the table title below. The rule SHALL span the visible content width. This separator SHALL appear in both the loading skeleton and live rendering.

#### Scenario: Separator between stats and table
- **GIVEN** the stats grid has been rendered
- **WHEN** the table title follows
- **THEN** a dim `───` rule SHALL appear between the last stats row and the `📊` title line

## Watch Mode: Sparkline Removal

### Requirement: Remove Sparkline

The sparkline component (`src/node/tui/sparkline.ts`) SHALL be deleted entirely. The `SparkPanel` class in the compositor SHALL be removed. The `buildPanel()` function SHALL no longer render sparkline charts. The sparkline history cache (5-minute TTL in watch.ts) SHALL be removed.

#### Scenario: No sparkline in watch output
- **GIVEN** watch mode is running on a wide terminal
- **WHEN** the compositor renders
- **THEN** no braille sparkline chart SHALL appear anywhere in the output

### Requirement: Remove Side-by-Side Merge

The `mergeSideBySide()` function SHALL be removed from the compositor. The table SHALL render at full width without a side panel. The `MERGE_GUTTER` constant and related panel width calculations SHALL be removed.

#### Scenario: Table renders without side panel
- **GIVEN** watch mode is running on a terminal >= 60 columns
- **WHEN** the table renders
- **THEN** the table SHALL use the same render functions as non-watch mode (no side-by-side merge)
- **AND** no gutter or panel content SHALL appear to the right of the table

## Watch Mode: Table Rendering

### Requirement: Unified Table Rendering

The table rendered in watch mode SHALL be identical to non-watch mode output — same `renderTotal`, `renderHistory`, `renderTotalHistory` calls. The compositor SHALL render the stats grid above, then the table below the separator, without any side-by-side merge.

#### Scenario: Watch table matches non-watch table
- **GIVEN** the same data set in both watch and non-watch mode
- **WHEN** the table portion renders
- **THEN** the table content SHALL be identical (same columns, same formatting, same bar charts)

## Watch Mode: Rain Tick Interval

### Requirement: Slower Rain Animation

The matrix rain tick interval SHALL change from 80ms to 107ms (75% of the original refresh rate: 80 / 0.75 ≈ 107). All other rain behavior (character set, density, trail, shimmer, respawn) SHALL remain unchanged.

#### Scenario: Rain tick timing
- **GIVEN** watch mode with rain enabled
- **WHEN** the rain animation runs
- **THEN** rain drops SHALL advance at ~107ms intervals instead of 80ms

## Watch Mode: Loading Skeleton

### Requirement: Skeleton on Alt-Screen Entry

When entering watch mode, a loading skeleton SHALL render immediately after switching to the alternate screen buffer — before the first fetch completes. The skeleton SHALL use the real layout structure (stats grid with placeholders + dim separator rule + table header + centered "Loading..." in `dim`). Rain SHALL start immediately alongside the skeleton.

#### Scenario: Skeleton appears before first fetch
- **GIVEN** the user starts watch mode
- **WHEN** the alternate screen buffer is activated
- **THEN** the skeleton SHALL appear with: stats grid (zeros/dashes), dim separator, table header, and dim "Loading..." centered in the data area
- **AND** matrix rain SHALL begin animating below the skeleton

#### Scenario: Skeleton replaced by real data
- **GIVEN** the loading skeleton is displayed
- **WHEN** the first fetch completes successfully
- **THEN** the skeleton SHALL be replaced by the real stats grid and table data

## Watch Mode: Terminal Breakpoints

### Requirement: Two-Tier Breakpoint System

The three-tier breakpoint system (wide >= 113 / medium 60-112 / narrow < 60) SHALL be replaced with two tiers:
- **Full** (>= 60 cols): stats grid + dim separator + full table + rain
- **Compact** (< 60 cols): compact table only, no stats grid, no rain

The 113-column threshold and all side panel logic SHALL be removed.

#### Scenario: Full mode on 80-column terminal
- **GIVEN** terminal width is 80 columns (previously "medium" tier)
- **WHEN** watch mode renders
- **THEN** the stats grid, separator, full table, and rain SHALL all be displayed

#### Scenario: Full mode on 120-column terminal
- **GIVEN** terminal width is 120 columns (previously "wide" tier with side panel)
- **WHEN** watch mode renders
- **THEN** the stats grid, separator, and full table SHALL be displayed without a side panel
- **AND** rain SHALL fill available space below content

#### Scenario: Compact mode on narrow terminal
- **GIVEN** terminal width is less than 60 columns
- **WHEN** watch mode renders
- **THEN** only the compact table SHALL be displayed (date/name + cost, no stats grid, no rain)

## Deprecated Requirements

### Side Panel Layout (wide terminal >= 113 cols)

**Reason**: The side panel (sparkline + session stats) is replaced by the stats grid above the table. The 113-col threshold no longer has a purpose.
**Migration**: Session stats are rendered in the 2x3 grid above the table. Sparkline is removed entirely.

### Sparkline Chart

**Reason**: 3 rows of braille over a wide cost range produces a nearly flat line. The history table already shows trend data.
**Migration**: N/A — removed with no replacement.

### `mergeSideBySide()` Function

**Reason**: With no side panel, the side-by-side merge is unnecessary.
**Migration**: Table renders directly without merge. The function and its export are deleted.

### History Cache (5-minute TTL)

**Reason**: The history cache existed solely to feed the sparkline. Without the sparkline, the cache is dead code.
**Migration**: N/A — removed with no replacement.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Remove sparkline entirely | Confirmed from intake #1 — user decided it adds little value | S:95 R:85 A:90 D:95 |
| 2 | Certain | Stats grid is 2x3: session-related left, cost-related right | Confirmed from intake #2 — user specified this exact grouping | S:95 R:90 A:90 D:95 |
| 3 | Certain | Stats grid renders above the table, not beside it | Confirmed from intake #3 — user said "show on top" | S:95 R:90 A:90 D:95 |
| 4 | Certain | Two terminal breakpoints: >= 60 full, < 60 compact | Confirmed from intake #4 — user confirmed no wide/medium distinction | S:95 R:85 A:90 D:95 |
| 5 | Certain | Rain tick changes from 80ms to 107ms | Confirmed from intake #5 — user specified 75% of current rate | S:90 R:95 A:90 D:95 |
| 6 | Certain | Delete sparkline.ts file entirely | Upgraded from intake #6 Confident — codebase confirms no other consumer | S:95 R:80 A:90 D:90 |
| 7 | Certain | Remove `mergeSideBySide()` from compositor | Upgraded from intake #7 Confident — spec confirms no side panel anywhere | S:95 R:80 A:90 D:85 |
| 8 | Certain | Loading skeleton on alt-screen entry with zeros/dashes and "Loading..." | Confirmed from intake #8 — user chose structure skeleton, watch mode only | S:95 R:90 A:90 D:95 |
| 9 | Certain | Dim horizontal rule separates stats grid from table title | Confirmed from intake #9 — user clarified, chose dim rule | S:95 R:95 A:90 D:95 |
| 10 | Certain | Unavailable stats show `--` placeholder; grid stays fixed 3 rows | Confirmed from intake #10 — user chose stable layout over contracting | S:95 R:85 A:85 D:95 |
| 11 | Certain | `buildPanel()` is rewritten as stats grid renderer, not deleted | Codebase analysis — panel.ts contains `formatElapsed`/`computeBurnRate` that remain needed | S:90 R:85 A:95 D:90 |
| 12 | Certain | `fetchHistory` callback and `refreshHistoryCache` removed from watch.ts | Only consumer was sparkline; history data for table comes from normal data flow | S:90 R:85 A:95 D:90 |
| 13 | Confident | Rain right-margin fallback mode still operational | Intake says "rain fills vertical space below content as before"; right-margin is existing fallback, not explicitly discussed | S:75 R:90 A:85 D:80 |

13 assumptions (12 certain, 1 confident, 0 tentative, 0 unresolved).
