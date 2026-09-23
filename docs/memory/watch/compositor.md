---
type: memory
description: The pure watch compositor — Lay/LaySkeleton rain-zone selection and Layout.Frame bytes, the CompactThreshold/minRainCols/rainGutter/MaxRows constants, the loading Skeleton and truncating Footer, and the golden-frame byte gate under an injected clock and seeded RNG
---

# Watch Compositor

**Domain**: watch

## Overview

The compositor (`internal/watch/compositor.go`, `skeleton.go`, `status.go`) is pure: content lines + terminal size + colors go in, frame bytes come out — no timers, no terminal. `Lay` picks the rain zone, `Layout.Frame` assembles the full-frame write, `Skeleton` is the loading frame, `Footer` the push-driven status line, and golden frames under an injected clock and seeded RNG pin every byte. The loop that drives it is [loop-and-terminal](/watch/loop-and-terminal.md); the stats/rain content is [panel-and-rain](/watch/panel-and-rain.md).

## Requirements

### Requirement: Layout constants
`internal/watch/compositor.go` MUST hold the layout constants: `CompactThreshold = 60`, `RainTick = 107ms`, `CountdownTick = 1s`, `minRainCols = 10`, `rainGutter = 2`, `footerRows = 1`, `MaxRows = 15`. The watch row budget MUST be the constant `MaxRows = 15`, not "rows that fit the terminal height" (the spec sentence is flagged for gate G0) (4pze). The budget reaches the table render through `Frame.MaxRows` ([history](/view/history.md)) via the `Poll` contract in [loop-and-terminal](/watch/loop-and-terminal.md).

### Requirement: Lay zone selection
`Lay(statsLines, tableLines, cols, rows, noRain) Layout` (`internal/watch/compositor.go`) MUST: drop the stats lines when `cols < CompactThreshold`; compute `contentHeight = len(stats) + len(table)`; take `maxContentWidth` as the max `ansi.StripANSI` rune width over the content lines; and set `available = rows − contentHeight − footerRows`, `wantRain = !noRain && !compact`. It MUST choose a below-content zone `{Cols cols, Rows available, StartRow contentHeight + 1, StartCol 0}` when `wantRain && available > 0`; else a right-margin zone `{cols − maxContentWidth − rainGutter, rows − footerRows, StartRow 1, StartCol maxContentWidth + rainGutter}` when that width is ≥ `minRainCols`; else the zone is disabled. `RainZone` MUST carry `Enabled`, `Cols`, `Rows`, the 1-based `StartRow` and the 0-based `StartCol`. `LaySkeleton(lines, …)` MUST equal `Lay(nil, lines, …)` — the skeleton's own lines are the whole content.

#### Scenario: Zone selection
- **GIVEN** 100×30 with 4 stats lines and 12 table lines, rain on
- **WHEN** `Lay` runs
- **THEN** the zone is `{Cols 100, Rows 13, StartRow 17, StartCol 0}`; at 100×12 with a widest line of 87 the zone is `{11, 11, StartRow 1, StartCol 89}`, and at a widest line of 89 the rain is disabled (`marginCols` 9 < `minRainCols`)

### Requirement: Frame byte shape
`Layout.Frame(footer, rain) []byte` (`internal/watch/compositor.go`) MUST emit `\x1b[H`; each content line (stats then table) + `\x1b[K\n`; `\x1b[J`; the footer as `\x1b[{rows};1H\x1b[K{footer}`; then the current rain frame re-emitted — the flush's clears erase every drawn rain cell and `RainState.Render` rewrites every occupied cell, so the re-emit restores the rain in the same write and polls do not blink it. `FooterLine(footer, rows)` MUST be the footer-only write `\x1b[{rows};1H\x1b[K{footer}` — push-driven on countdown changes, never on a periodic compositor tick.

### Requirement: The skeleton
`Skeleton(cols, now, colors)` (`internal/watch/skeleton.go`) MUST be the loading frame written on alt-screen entry before the first fetch. Full mode: the stats grid for a zeroed session (`Elapsed 0s`, `Session $0.00`, the rest `--`); `""`; `BoldWhite("📊 Combined Usage (daily)")` — the daily snapshot header on EVERY display type (4pze); `""`; the `BoldCyan` snapshot header (`Tool` padded right to 12, the rest padded left to 12, joined ` | `); the `Dim` 87-char divider; then 38 spaces + `Dim("Loading...")` (`floor((87 − 10) / 2)`). Compact (`cols < CompactThreshold`): `""` then `Dim("Loading...")`. `SkeletonFrame(lines)` MUST be the write form `\x1b[H` + each line + `\n` (no `\x1b[K`).

### Requirement: The footer
`Footer(count, refreshing, width, colors)` (`internal/watch/status.go`) MUST join `Dim("Next refresh: {n}s")` — or `Dim("Refreshing...")` when refreshing — with `Dim("↵ refresh · q quit")` via `Dim(" · ")`; while the visible (`ansi.StripANSI`) width exceeds the terminal width and more than one part remains, the last part MUST be dropped and the parts re-joined.

#### Scenario: Footer truncation
- **GIVEN** a footer whose visible width exceeds the terminal width
- **WHEN** `Footer` runs
- **THEN** the controls hint `↵ refresh · q quit` drops first; at narrower widths only the status text remains

### Requirement: Golden frames are the byte gate
Golden frames under an injected clock and seeded RNG MUST pin the byte stream (4pze): `internal/watch/testdata/` holds `skeleton_100x30`, `skeleton_50x20`, `frame_first_poll_100x30`, `frame_second_poll_100x30` (clock advanced 11 s, cost +0.07 — populated Rate/Proj. day), `frame_compact_59x20`, `frame_rightmargin_100x12`, `frame_norain_100x30`, and `rain_seeded` (one tick, one render, one clear pass), regenerated with `go test ./internal/watch/ -update` (4pze). The loop test against `fakeTerminal` (`internal/watch/faketerm_test.go`, `watch_test.go`) MUST cover skeleton → first poll → `q` (returning the poll's lines), `\r` cancelling the countdown, a mid-poll `\r` dropped by the re-entrancy guard, SIGWINCH before the first poll (writes nothing) and after (re-flush), a `Poll` error (stderr warning, frame unchanged), and the `\x03`/SIGINT exits ending `\x1b[?25h\x1b[?1049l` — no wall-clock sleeps. The deterministic watch-family matrix cases in the [differential-harness matrix](/harness/matrix-and-staging.md) exercise the same parse and dispatch path.

## Design Decisions

### Pure compositor under an injected clock and RNG
**Decision**: `Lay`/`Frame`, `StatsGrid`, the footer, the skeleton and `RainState` are pure functions of (state, size, `now`, `*rand.Rand`); only `watch.Run` touches timers and the `Terminal`.
**Why**: a session frame depends on wall-clock (Elapsed, countdown) and on randomness (rain), so a byte gate needs both injected; goldens under a fake clock and seeded RNG are the gate the no-framework plan decision names. A live byte-diff capture would need a new external surface with nothing to diff against.
**Rejected**: a one-shot frame-capture flag — a new external surface with nothing to diff against. (4pze)
*Introduced by*: 260917-4pze-watch-mode-tui

### MaxRows is a constant, not a height fit
**Decision**: the watch row budget is the constant `MaxRows = 15` (`internal/watch/compositor.go`), not "rows that fit the terminal height".
**Why**: the pinned bytes use 15; the spec's "rows that fit the terminal height" sentence is flagged for gate G0 rather than silently followed.
**Rejected**: sizing the budget from the live terminal height — the pinned bytes would vary per geometry. (4pze)
*Introduced by*: 260917-4pze-watch-mode-tui

### Rain re-emit on every flush
**Decision**: `Layout.Frame` re-emits the current rain frame after the footer write.
**Why**: the flush's `\x1b[J`/`\x1b[K` clears wipe the rain zone, and `RainState.Render` rewrites every occupied cell each frame, so the re-emit fully restores the rain in the same write — polls do not blink the rain.
**Rejected**: suppressing the clears — stale content lines would survive reflows. (h9h7)
*Introduced by*: 260717-h9h7-watch-rain-polish

### Skeleton rain reuses Lay
**Decision**: `LaySkeleton` is `Lay` with the skeleton's lines as the whole content, so the rain zone is laid out before the first poll and rain animates during `Loading...`.
**Why**: minimal-diff reuse of the zone math; the first real poll's re-layout is a no-op `Resize` when the geometry matches the skeleton's, so the drops survive.
**Rejected**: a separate skeleton-zone computation — duplicated zone math that can drift. (h9h7)
*Introduced by*: 260717-h9h7-watch-rain-polish

### Push-driven footer, no compositor tick
**Decision**: `FooterLine` is written only on countdown state changes and `Refreshing...` transitions; there is no periodic compositor tick.
**Why**: the countdown changes at most once per second, so a periodic tick would be dozens of idle wakeups per second for identical bytes.
**Rejected**: a periodic dirty-flag tick — dozens of idle wakeups per second for identical bytes. (h9h7)
*Introduced by*: 260717-h9h7-watch-rain-polish
