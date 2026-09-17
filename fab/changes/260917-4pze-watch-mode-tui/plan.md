# Plan: Watch Mode (Go port row B7)

**Change**: 260917-4pze-watch-mode-tui
**Intake**: `intake.md`

> The intake (`intake.md` § What Changes 1–12) carries every exact byte sequence, constant, formula and placement rule; this plan states the requirements and points at the intake section that owns each detail. Where the intake and the TS source disagree, the TS source (`src/node/tui/watch.ts`, `compositor.ts`, `panel.ts`, `rain.ts`, `formatter.ts`, `core/cli.ts`) is the contract — the port reproduces bytes, not prose.

## Requirements

### Watch: loop and terminal seam

#### R1: Startup order and alt-screen entry
`watch.Run` SHALL, in order: write `\x1b[?1049h` then `\x1b[?25l`; write the skeleton (R11) as `\x1b[H` + each line + `\n`; lay out the rain zone against the skeleton lines (R7); start the 107 ms rain ticker (unless `NoRain`); register the key, resize and interrupt inputs; run the first poll. All terminal writes happen from the single loop goroutine.

- **GIVEN** `tu -w` on a 100×30 TTY
- **WHEN** `Run` starts
- **THEN** the byte stream begins `\x1b[?1049h\x1b[?25l\x1b[H` followed by the full-mode skeleton lines each terminated by `\n`, and rain cells appear before the first poll result

#### R2: Poll sequence and session state
Each poll SHALL: set the footer to `Refreshing...` (R10); build `Frame{Prev, Compact: width < 60, MaxRows: 15, Width}` where `Prev` is the previous poll's `CostByItem` (nil when empty); call `Poll`. On error it SHALL write `Warning: fetch failed, retrying next cycle\n` to stderr and restart the countdown without changing the frame. On success it SHALL record `cost = Stats.TotalCost`, set `StartTime/StartCost/StartTokens` on the first success only, append `{now, cost}` to the poll history, store the lines as `last`, lay out + flush (R7, R8), copy `Stats.CostByItem` into `Prev` for the next poll, and start the countdown at `Interval`. A poll request while one is in flight SHALL be dropped.

- **GIVEN** a `Poll` that returns lines with `TotalCost 31.79` then `31.86`
- **WHEN** two polls complete 11 s apart
- **THEN** the second frame's stats grid shows `Session  +$0.07` and the second poll's `Frame.Prev` equals the first poll's `CostByItem`
- **GIVEN** `Poll` returns an error
- **WHEN** the poll completes
- **THEN** stderr receives exactly `Warning: fetch failed, retrying next cycle\n`, the last frame is not rewritten, and the footer restarts at `Next refresh: {Interval}s`

#### R3: Countdown and keys
The countdown SHALL start at `Interval`, render the footer immediately, decrement every 1000 ms re-rendering only the footer line, and trigger a poll when the value reaches 0. A key chunk equal to `\r`, `\n` or ` ` SHALL cancel the countdown and poll immediately; a chunk equal to `q` or `\x03` SHALL exit (R4); any other chunk SHALL be ignored. Keys are read only when stdin is a TTY (raw mode via `term.MakeRaw`).

- **GIVEN** interval 10 and a completed poll
- **WHEN** 3 s elapse
- **THEN** exactly three footer rewrites occurred (`9s`, `8s`, `7s`) and no full frame
- **GIVEN** the countdown at `6s`
- **WHEN** the key reader delivers `\r`
- **THEN** the countdown timer is cancelled, the footer reads `Refreshing...`, and `Poll` is called

#### R4: Exit path
On `q`, `\x03` or SIGINT, `Run` SHALL (once): stop the rain ticker and countdown timer, restore the terminal (raw mode off, signals released), write `\x1b[?25h` then `\x1b[?1049l`, and return the last successful poll's lines. `cmd/tu` SHALL print each returned line + `\n` to stdout and return exit 0.

- **GIVEN** a session with one successful poll
- **WHEN** `q` arrives
- **THEN** the stream ends with `\x1b[?25h\x1b[?1049l` and the process stdout afterwards equals the one-shot `command.Run` lines for the same request (each + `\n`)

#### R5: Resize
A SIGWINCH before the first successful poll SHALL do nothing. After it, resize SHALL re-lay-out from the cached table lines, session and cost against the new size and flush a full frame (R8).

- **GIVEN** the skeleton on screen and no poll yet
- **WHEN** SIGWINCH fires
- **THEN** no bytes are written
- **GIVEN** a completed poll at 100×30
- **WHEN** the size becomes 59×20 and SIGWINCH fires
- **THEN** a full frame is written with no stats grid and no rain (compact)

#### R6: Terminal seam
`watch.Terminal` SHALL expose `Size()`, `Write`, `Keys()`, `Resize()`, `Interrupt()`, `Close()` (intake § 2). The `x/term` implementation SHALL report 80×24 when stdout is not a TTY or the probe fails, return a nil `Keys()` channel when stdin is not a TTY, use `syscall.SIGWINCH`/`SIGINT` via `os/signal`, and restore raw mode in `Close`. Tests SHALL use a fake terminal with a scripted key sequence, settable size and a capturing buffer.

- **GIVEN** stdout piped
- **WHEN** `Size()` is called
- **THEN** it returns `(80, 24)`

### Watch: compositor and rendering

#### R7: Layout and rain zone
`Lay(stats, table, cols, rows, noRain)` SHALL reproduce `layoutAndUpdate` + `setupRainZone` (intake § 3): `compact = cols < 60` drops the stats lines; `available = rows − contentHeight − 1`; below-content zone `{cols, available, startRow contentHeight+1, startCol 0}` when `wantRain && available > 0`; else right-margin zone `{cols − maxContentWidth − 2, rows − 1, startRow 1, startCol maxContentWidth + 2}` when that width is ≥ 10; else disabled. `maxContentWidth` is the max `StripANSI` rune width over the content lines. A rain state whose `(cols, rows, startCol)` are unchanged SHALL keep its drops.

- **GIVEN** 100×30, 4 stats lines and 12 table lines, rain on
- **WHEN** `Lay` runs
- **THEN** the zone is `{100, 13, 17, 0}`
- **GIVEN** 100×12, content of 16 lines whose widest line is 87
- **WHEN** `Lay` runs
- **THEN** the zone is `{11, 11, 1, 89}`; at widest 89 it is disabled (`marginCols` 9)

#### R8: Frame bytes
`Layout.Frame(footer, rain)` SHALL emit `\x1b[H`, each content line + `\x1b[K\n`, `\x1b[J`, `\x1b[{rows};1H\x1b[K{footer}`, then the rain frame. The footer-only write SHALL be `\x1b[{rows};1H\x1b[K{footer}`.

- **GIVEN** a two-line content, rows 24, footer `F`, rain `R`
- **WHEN** `Frame` runs
- **THEN** the bytes are `\x1b[H` + `L1\x1b[K\n` + `L2\x1b[K\n` + `\x1b[J` + `\x1b[24;1H\x1b[KF` + `R`

#### R9: Stats grid
`StatsGrid(session, todayCost, now, colors)` SHALL reproduce `buildStatsGrid` byte for byte (intake § 4): the Elapsed/Session/Tok-min/Rate/Proj-day values and placeholder rules, the `formatGridRow` widths (`leftLabelW 9`, `maxLeftValueW 8`, `gap 5`, `rightLabelW 10`), Dim labels, BoldWhite values, Yellow Rate, and the Dim `─` separator sized to the widest row. `BurnRate` SHALL use the last 5 polls and report `!ok` below two polls, `0` when `Δt == 0`.

- **GIVEN** polls at t=0 (`$31.79`) and t=11 s (`$31.86`), tokens 64,946,471 → 73,409,000, now = t+11 s, todayCost 31.86 at 09:00 local
- **WHEN** the grid renders
- **THEN** row 1 is `Elapsed  11s` / `Tok/min   ~46,159,249`, row 2 is `Session  +$0.07` / `Rate      ~$22.91/hr` (yellow), row 3 is `Proj. day ~$375.50`, and the separator width equals the widest row's visible width
- **GIVEN** a single poll
- **WHEN** the grid renders
- **THEN** Session is `$0.00` and Tok/min, Rate, Proj. day are `--`; the separator is 35 `─`

#### R10: Footer
The footer SHALL be `Dim("Next refresh: {n}s")` or `Dim("Refreshing...")`, then `Dim("↵ refresh · q quit")`, joined by `Dim(" · ")`, dropping trailing parts while the visible width exceeds the terminal width and more than one part remains.

- **GIVEN** width 20
- **WHEN** the countdown is 45
- **THEN** the footer is `Dim("Next refresh: 45s")` only

#### R11: Skeleton
The skeleton SHALL reproduce `renderSkeleton` (intake § 6): full mode = stats grid for `Session{StartTime: now}` + `""` + `BoldWhite("📊 Combined Usage (daily)")` + `""` + the BoldCyan snapshot header + `Dim(divider)` + `38 spaces + Dim("Loading...")`; compact = `""` + `Dim("Loading...")`. The header is the daily snapshot header for every display type.

- **GIVEN** 100 columns and `tu m lbh -w`
- **WHEN** the skeleton renders
- **THEN** its title line is `📊 Combined Usage (daily)` and the placeholder line is 38 spaces + `Loading...`

#### R12: Rain
`RainState` SHALL reproduce `rain.ts` with an injected `*rand.Rand` (intake § 7): `ActiveDropCount`, shuffled round-robin column assignment, scatter on init, `tick` (delay, speed, shimmer 0.05, respawn), `render(startRow)` cell emission order and colors (`BrightGreen` head, `Green` body, `DimGreen` last two), vacated-cell clears with a space, `resize` no-op on identical geometry. The ticker SHALL run every 107 ms and write `render()` when non-empty.

- **GIVEN** cols 100, rows 20
- **WHEN** `ActiveDropCount`
- **THEN** 30; at rows 60 it is 90; at rows 10 it is 30
- **GIVEN** a seeded state after one tick
- **WHEN** `render(17)` runs twice with a tick between
- **THEN** every cell vacated between the frames is followed by a `\x1b[{r};{c}H ` clear in the second output

### View and render: deltas, row budget, compact tables

#### R13: Cell-level delta with two placements
`view.Cell` SHALL gain `Delta`; `view.Table` SHALL gain `DeltaInCell` (`DeltaPadsArrow` | `DeltaAfterPad`). `ansi.dataRow` SHALL render a cell delta as `PadLeft(text + arrow, width)` under `DeltaPadsArrow` (rune count over the colored string — the JS raw-length quirk) and `PadLeft(text, width) + arrow` under `DeltaAfterPad`, with the exact-zero Dim wrapping the composite. `view.Snapshot(rows, p, bd, SnapshotOptions{Metric, Prev})` SHALL set the Cost cell's delta (Tokens cell under `Tokens`) from `Prev[Name]` with `DeltaPadsArrow`; `view.Leaderboard` SHALL set the Cost (or Tokens) cell's delta from `Prev[key]` with `DeltaAfterPad`. The history tables keep the trailing `Row.Delta`.

- **GIVEN** a snapshot row `Kimi` with cost 26.86, `Prev{"Kimi": 26.80}`, colors on
- **WHEN** rendered
- **THEN** the Cost cell is ` | $26.86 \x1b[32m↑\x1b[0m` with no leading pad (raw length 17 ≥ 12)
- **GIVEN** the same with `NO_COLOR`
- **THEN** the Cost cell is ` |     $26.86 ↑` (padded to 12)
- **GIVEN** a leaderboard row with cost 0 and `Prev` lower
- **THEN** the Cost cell is `Dim(PadLeft("$0.00", w) + Green("↑"))`

#### R14: Row budget
`HistoryOptions.MaxRows` (0 = unlimited) SHALL keep the last `MaxRows` entries in `History` (after the empty check) and the last `MaxRows` labels in `TotalHistory` (before the empty check); significance, separators, the p95 scale, the footer and the legend SHALL be computed on the truncated window.

- **GIVEN** 40 daily entries and `MaxRows 15`
- **WHEN** `History` renders
- **THEN** 15 data rows appear, the footer `avg` divides by 15, and a month separator appears only if a boundary falls inside those 15

#### R15: Compact tables
`view.CompactSnapshot`, `CompactHistory`, `CompactTotalHistory` and `ansi.CompactTable` SHALL reproduce the TS compact renderers (intake § 8): title lines as the full table, rows `PadRight(name, 14) + " " + PadLeft(value + arrow, 12)`, Dim 27-char divider and BoldWhite Total only when more than one row, trailing `""`; the empty check precedes compact; snapshot Total sums all input rows; no machine columns, legend, bars or separators; token mode puts total tokens in the value cell. Leaderboards have no compact form.

- **GIVEN** `tu -w` at 50 columns with Claude Code `$5.38` and Kimi `$27.25`
- **WHEN** the compact snapshot renders with `NO_COLOR`
- **THEN** the lines are `""`, `📊 Combined Usage (daily)`, `""`, `Claude Code           $5.38`, `Kimi                 $27.25`, `───────────────────────────`, `Total                $32.63`, `""`

### Command and edge

#### R16: `command` admits watch and carries live options
`inScope` SHALL no longer reject `Watch` or `NoRain`. `Deps` SHALL gain `Live *LiveOptions{Prev, Compact, MaxRows}`; `runSnapshot`/`runHistory`/`runLeaderboard`/`runLeaderboardHistory` SHALL pass `Prev`, the history paths SHALL set `MaxRows`, and the snapshot/history table paths SHALL render the compact tables when `Live.Compact`. `Result.TotalCost/TotalTokens/CostByItem` stay computed over untruncated data. `tu --no-rain` and `tu --interval 30` without `-w` SHALL render the ordinary one-shot output.

- **GIVEN** `Deps.Live = &LiveOptions{Compact: true, MaxRows: 15}` and a history request
- **WHEN** `Run` completes
- **THEN** `Lines` are the compact history and `CostByItem` has one key per untruncated entry

#### R17: `cmd/tu` watch branch
When `req.Flags.Watch`, after the reserved-user guard and the `--sync` block, `cmd/tu` SHALL run `command.Normalize` once and print its notices to stderr before the alt screen, build `watch.Options` (interval, no-rain, colors, `time.Now`, a time-seeded `rand.Rand`, the real terminal, stderr) with a `Poll` that copies `deps` with `Width: f.Width` and `Live` from the frame, sets `Fresh = true`, calls `command.Run`, writes `source.WriteWarnings(stderr, res.Warnings)` per poll, and returns lines + stats. `ErrLeaderboardMode` SHALL be reported before the alt screen (exit 1). After `Run` returns, the last lines are printed and the exit code is 0.

- **GIVEN** `tu -u bob -w` in single mode
- **WHEN** the binary starts
- **THEN** stderr receives the `-u` notice once, before `\x1b[?1049h`, and no notice repeats on later polls
- **GIVEN** `tu lb -w` in single mode
- **THEN** stderr is the `lb requires multi mode` line, exit 1, no alt screen

#### R18: Harness matrix
`harness/matrix.json` SHALL gain `watch-csv`, `watch-md`, `watch-interval-min` (`-w -i 3`), `watch-interval-max` (`-w -i 4000`), `watch-interval-nan` (`-w -i abc`), `no-rain-without-watch` (`--no-rain`, conf single/multi, io pipe/tty) and `interval-without-watch-long` (`--interval 30`); `TestCommittedMatrix`'s size rail SHALL still pass (raise its ceiling only if the count crosses it); `just go-diff --placeholder` SHALL be all green.

- **GIVEN** the new cases
- **WHEN** `just go-diff --placeholder` runs
- **THEN** every case is green and the summary count grew by the expanded new cases

### Tests

#### R19: Golden frames and loop tests
`internal/watch` SHALL carry golden-file tests (`-update` flag, `testdata/*.golden`) for the skeleton (100×30, 50×20), first- and second-poll frames under a fixed clock, compact frames at 59×20, right-margin rain at 100×12, `--no-rain`, footer truncation, and seeded rain; table-driven tests for `FormatElapsed`, `BurnRate`, `StatsGrid`, `ActiveDropCount`, `Lay`; and a loop test with the fake terminal covering first poll → `q`, `\r` immediate poll with the re-entrancy guard, SIGWINCH before/after the first poll, a `Poll` error, `\x03` and SIGINT exits. `view`/`ansi` SHALL carry goldens for the snapshot delta under cost/tokens with and without color, the leaderboard delta, `MaxRows`, and the three compact tables. `cmd/tu` e2e SHALL cover `--no-rain` and `--interval 30` one-shot rendering.

- **GIVEN** `go test ./internal/watch/`
- **WHEN** run without `-update`
- **THEN** every golden matches and no test sleeps on wall-clock time

### Non-Goals
- No change under `src/node/` (D4) or `docs/specs/`; the two G0 flags (row budget constant, skeleton header) are recorded in memory at hydrate.
- No live `-w` byte-diff in the harness and no capture switch (intake § Why).
- DC-17 (over-wide tables break the frame) is reproduced, not guarded.
- `--skip-brew-update` on a data command stays `ErrUnported` (B8's flag family).

### Design Decisions

#### Pure compositor under an injected clock and RNG
**Decision**: `Lay`/`Frame`, `StatsGrid`, the footer, the skeleton and `RainState` are pure functions of (state, size, `now`, `*rand.Rand`); only `Run` touches timers and the `Terminal`.
**Why**: the harness cannot byte-diff a time- and RNG-dependent session without a new external surface; goldens under a fake clock are the gate D12 names.
**Rejected**: a `--once` flag or `TU_WATCH_ONCE` env on both binaries — a new external surface the plan's Goal forbids.
*Introduced by*: 260917-4pze-watch-mode-tui

#### Cell-level delta with two padding placements
**Decision**: `Cell.Delta` + `Table.DeltaInCell` (`DeltaPadsArrow` for snapshot and compact, `DeltaAfterPad` for the leaderboard); the history tables keep the trailing `Row.Delta`.
**Why**: the TS places the arrow three different ways and pads by JS raw string length; `PadLeft`'s rune count over the colored string reproduces that quirk with no special casing.
**Rejected**: normalising every table to the trailing form — it changes bytes on the snapshot and leaderboard.
*Introduced by*: 260917-4pze-watch-mode-tui

#### Compact tables as their own model
**Decision**: `view.CompactTable` + `ansi.CompactTable`, separate from `view.Table`.
**Why**: two space-joined columns, no header row, no gutters — a different shape; the full `Table` encoder stays untouched.
**Rejected**: a `Compact bool` on `Table` threaded through `ansi.Table` — every encoder branch would grow a compact arm.
*Introduced by*: 260917-4pze-watch-mode-tui

#### Live options on `Deps`, not on `Request`
**Decision**: `Deps.Live *LiveOptions{Prev, Compact, MaxRows}` and per-poll `Deps.Width`.
**Why**: they are render state of the live session, not CLI grammar; `Deps` is already the per-call value the edge composes.
**Rejected**: `Request.Flags` fields — they would leak into `Parse`/`Normalize` and the help text seam.
*Introduced by*: 260917-4pze-watch-mode-tui

#### Guard notices once, at the edge
**Decision**: `cmd/tu` calls `command.Normalize` once before entering the alt screen, prints those notices, and discards per-poll `Result.Notices`.
**Why**: the TS prints the guards in `main()` ahead of `runWatch`; `Normalize` does not clear `--full`, so its notice would repeat every poll if taken from `Run`.
**Rejected**: a `Deps.Quiet` flag suppressing notices inside `Run` — more surface for the same bytes.
*Introduced by*: 260917-4pze-watch-mode-tui

## Tasks

### Phase 1: Setup

- [x] T001 `src/go/internal/view/table.go`, `snapshot.go`, `src/go/internal/render/ansi/table.go` (+ `_test.go`, goldens): add `Cell.Delta`, `Table.DeltaInCell` (`DeltaPadsArrow`/`DeltaAfterPad`), `SnapshotOptions{Metric, Prev}` (update every `view.Snapshot` caller), `HistoryOptions.MaxRows`; render the cell delta per placement in `ansi.dataRow`; goldens for the snapshot delta under cost and tokens, colored and `NO_COLOR` <!-- R13 -->
- [x] T002 [P] `src/go/internal/view/compact.go`, `src/go/internal/render/ansi/compact.go` (+ tests, goldens): `CompactTable`/`CompactRow`, `CompactSnapshot`, `CompactHistory`, `CompactTotalHistory`, `ansi.CompactTable`; goldens for the three tables with and without deltas, single-row (no Total), token mode <!-- R15 -->

### Phase 2: Core Implementation

- [x] T003 `src/go/internal/view/history.go`, `pivot.go`, `leaderboard.go` (+ tests, goldens): `MaxRows` truncation in `History`/`TotalHistory` with window-scoped footer/significance; `Leaderboard` fills the metric cell's `Delta` from `Prev` with `DeltaAfterPad` and the zero-Dim composite <!-- R14 -->
- [x] T004 `src/go/internal/command/run.go`, `leaderboard.go` (+ tests): `inScope` admits `Watch`/`NoRain`; `LiveOptions` on `Deps`; `Prev`/`MaxRows`/compact wiring in the four run paths; table-driven tests for compact selection, `MaxRows`, `Prev` pass-through, untruncated `CostByItem`, and `--no-rain`/`--interval` one-shot in scope <!-- R16 -->
- [x] T005 `src/go/internal/watch/terminal.go` (+ `terminal_test.go`, `faketerm_test.go` helper): `Terminal` interface, `NewTerminal(stdout, stdin *os.File)` on `x/term` + `os/signal` (80×24 fallback, TTY-gated raw mode and key reader, SIGWINCH/SIGINT, `Close` restore), fake terminal for tests <!-- R6 --> <!-- rework: raw mode must keep OPOST|ONLCR (libuv parity) -->
- [x] T006 `src/go/internal/watch/panel.go` (+ tests): `Session`, `PollPoint`, `FormatElapsed`, `BurnRate`, `StatsGrid` with `render.FixedHalfUp`/`JSRound`/`FormatInt`; table-driven cases incl. the R9 scenarios, negative/zero rate, `Δt == 0` <!-- R9 -->
- [x] T007 [P] `src/go/internal/watch/status.go` (+ tests): footer composition and progressive truncation at 30/20/10 columns <!-- R10 -->
- [x] T008 [P] `src/go/internal/watch/rain.go` (+ tests): constants, `ActiveDropCount`, `NewRainState(cols, rows, startCol, rng)`, `Tick`, `Render(startRow)`, `Resize`; seeded goldens for one tick/render/clear cycle and the `ActiveDropCount` table <!-- R12 -->
- [x] T009 `src/go/internal/watch/compositor.go`, `skeleton.go` (+ tests, `testdata/*.golden`, `-update` flag): `Layout`, `Lay`, `Frame`, `FooterLine`, `Skeleton(cols, now, colors)`; goldens for skeleton 100×30 and 50×20, `Lay` zone table (below-content, right-margin at 10 and 9 cols, disabled, compact), `Frame` bytes <!-- R7, R8, R11 -->
- [x] T010 `src/go/internal/watch/watch.go` (+ `watch_test.go`): `Options`, `Frame`, `Stats`, `Poll`, `Run` — the select loop, session state, poll worker, countdown timer, rain ticker, keys, resize, interrupt, cleanup; loop tests with the fake terminal and a manual clock (first poll → `q`, `\r` + re-entrancy guard, SIGWINCH before/after, `Poll` error, `\x03`, SIGINT); frame goldens for first/second poll at 100×30, compact 59×20, `--no-rain`, right-margin 100×12 <!-- R1, R2, R3, R4, R5, R19 -->

### Phase 3: Integration & Edge Cases

- [x] T011 `src/go/cmd/tu/main.go` (+ `e2e_test.go`): the watch branch (Normalize once → notices → `watch.Run` → last lines, exit 0; `ErrLeaderboardMode` before the alt screen; per-poll `WriteWarnings`; `Fresh = true`); rewrite the package doc comment and the `ErrUnported` comment in `command/run.go`; e2e for `--no-rain` and `--interval 30` one-shot and the notice-once ordering with a fake terminal seam or an injected `Run` hook <!-- R17 -->
- [x] T012 `harness/matrix.json` (+ `src/go/internal/harness/matrix_test.go` if the rail trips): add the seven deterministic watch-family cases; run `just go-diff --placeholder` and record the new count <!-- R18 -->

### Phase 4: Polish

- [x] T013 Gates and notes: `cd src/go && gofmt -l . && go vet ./... && go test ./... -count=1`, `just go-diff --placeholder`, `just go-live`, `env -u TU_METRICS_REPO -u NO_COLOR npm test` (unchanged); paste the real tails into `## Notes`; confirm `go.mod` gained no dependency <!-- R19 -->

## Execution Order

- T001 blocks T002, T003, T004 (shared `Cell.Delta`/options types)
- T005 blocks T010; T006, T007, T008, T009 block T010
- T004 and T010 block T011; T011 blocks T012; T012 blocks T013

## Acceptance

### Functional Completeness

- [x] A-001 R1: `Run` writes `\x1b[?1049h\x1b[?25l\x1b[H` then the skeleton lines with `\n` before any poll result
- [x] A-002 R2: the poll sequence sets footer `Refreshing...`, builds `Frame{Prev, Compact, MaxRows 15, Width}`, updates session state on success, and copies `CostByItem` into `Prev`
- [x] A-003 R3: countdown starts at `Interval`, rewrites only the footer each second, polls at 0; `\r`/`\n`/` ` cancel and poll; `q`/`\x03` exit; other chunks ignored
- [x] A-004 R4: exit writes `\x1b[?25h\x1b[?1049l` and `cmd/tu` prints the last lines, exit 0
- [x] A-005 R5: SIGWINCH before the first poll writes nothing; after it re-lays-out and flushes
- [x] A-006 R6: `Terminal` interface per intake § 2; 80×24 fallback; raw mode and keys only on a TTY stdin
- [x] A-007 R7: `Lay` selects below-content / right-margin (gutter 2, min 10) / disabled / compact exactly as `setupRainZone`
- [x] A-008 R8: `Frame` and footer-only bytes match intake § 3
- [x] A-009 R9: `StatsGrid` values, widths, colors and separator match `buildStatsGrid`; `BurnRate` 5-poll window
- [x] A-010 R10: footer parts, Dim separator and truncation order
- [x] A-011 R11: skeleton full and compact forms; daily snapshot header for every display
- [x] A-012 R12: `RainState` constants, init, tick, render (colors, order, clears) and resize match `rain.ts`
- [x] A-013 R13: `Cell.Delta` + `DeltaInCell`; snapshot uses `DeltaPadsArrow`, leaderboard `DeltaAfterPad`; history tables unchanged
- [x] A-014 R14: `MaxRows` truncates `History` after and `TotalHistory` before the empty check; footer/significance on the window
- [x] A-015 R15: the three compact tables and encoder match the TS compact renderers; no compact for lb/lbh
- [x] A-016 R16: `inScope` admits `Watch`/`NoRain`; `Deps.Live` wired in all four run paths; stats untruncated
- [x] A-017 R17: `cmd/tu` watch branch order (guard → sync → Normalize once → alt screen), per-poll warnings, `Fresh`, exit 0, `ErrLeaderboardMode` before the alt screen
- [x] A-018 R18: seven matrix cases added; `TestCommittedMatrix` passes; `just go-diff --placeholder` all green

### Behavioral Correctness

- [x] A-019 R13: with colors on, a snapshot cost cell with an arrow is unpadded (`$26.86 ` + colored arrow); with `NO_COLOR` it pads to 12 — both pinned by goldens
- [x] A-020 R16: `tu --no-rain` and `tu --interval 30` render the ordinary one-shot table (no placeholder)
- [x] A-021 R2: a `Poll` error writes exactly `Warning: fetch failed, retrying next cycle\n` and leaves the frame unchanged

### Scenario Coverage

- [x] A-022 R19: golden frames exist and pass for skeleton (100×30, 50×20), first/second poll, compact 59×20, right-margin 100×12, `--no-rain`, footer truncation, seeded rain
- [x] A-023 R19: the loop test covers first poll → `q`, `\r` with re-entrancy guard, SIGWINCH before/after, `Poll` error, `\x03`, SIGINT — with no wall-clock sleeps
- [x] A-024 R9: the R9 two-poll scenario values are asserted in a table-driven test

### Edge Cases & Error Handling

- [x] A-025 R6: non-TTY stdin → no key reader, SIGINT still exits cleanly
- [x] A-026 R7: `marginCols` 9 disables rain; identical geometry keeps drops across polls
- [x] A-027 R14: `MaxRows 0` leaves the tables byte-identical to today (all existing goldens unchanged)

### Code Quality

- [x] A-028 Pattern consistency: new packages follow the sibling shapes (`_test.go` siblings, `-update` goldens, `Now func() time.Time` seams, no I/O below `cmd/tu` except `watch`'s `Terminal`)
- [x] A-029 No unnecessary duplication: `ansi.Colors`, `StripANSI`, `PadLeft/PadRight`, `render.FixedHalfUp/JSRound/FormatInt` reused; no second ANSI stripper or padder
- [x] A-030 Readability: no function over ~50 lines without reason; named constants for every TS constant (no magic numbers)
- [x] A-031 Errors: no swallowed errors — poll failures reach stderr; `Terminal` probe failures degrade to 80×24 explicitly
- [x] A-032 Architecture: `watch` imports no `source`, `query`, `view` or `render/{json,csv,markdown}` package; `command` imports no `watch`; nothing calls `os.Exit` below `cmd/tu`
- [x] A-033 Dependencies: no new module in src/go/go.mod (x/sys moves from indirect to direct; no other change)

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`
- Byte references come from running `node dist/tu.mjs` on a staged home (`just build`) or from the TS source read literally — never from spec prose.

### Gate tails (apply, 2026-09-17)

`cd src/go && gofmt -l . && go vet ./... && go test ./... -count=1` — gofmt/vet silent; all 24 packages `ok` (incl. `internal/watch 0.055s`):

```
ok  	github.com/sahil87/tu/cmd/tu	0.417s
ok  	github.com/sahil87/tu/internal/command	0.008s
ok  	github.com/sahil87/tu/internal/render/ansi	0.008s
ok  	github.com/sahil87/tu/internal/view	0.004s
ok  	github.com/sahil87/tu/internal/watch	0.055s
```

`env -u TU_METRICS_REPO -u NO_COLOR just go-diff --placeholder` (baseline was 442/442 before the matrix edits; 7 groups / +10 cases added, all green):

```
tudiff: 452 cases — 452 green, 0 red, 0 timeout   (fixtures: _placeholder; 172 cases replayed unconfirmed fixtures)
  by conf:  single 182/182  multi 176/176  org 47/47  legacy 47/47
  by env:   default 370/370  nocolor 34/34  envrepo 32/32  pullfail 8/8  pushfail 4/4  dirty 4/4
  by io:    pipe 376/376  tty 76/76
  by tz:    fixed 385/385  alt 67/67
```

`just go-live`:

```
tudiff: 9 cases — 9 green, 0 red, 0 timeout   (fixtures: _placeholder, live-alias; 0 cases replayed unconfirmed fixtures)
```

`env -u TU_METRICS_REPO -u NO_COLOR npm test` (TS side untouched): `pass 1091, fail 0`.

`go.mod`: unchanged (`git status` clean for `src/go/go.mod`/`go.sum`); no dependency added (`math/rand/v2`, `os/signal`, `syscall` are stdlib; `golang.org/x/term` was already required).

R9's Proj. day expectation reads `~$375.51`; the `FixedHalfUp` twin (the plan's own assumption 5 defers to it) computes `~$375.50` for the scenario inputs (31.86 + (0.07/11×3600) × 15 = 375.49636…) — the golden pins the twin's value.

### Rework (pre-review, raw-mode OPOST|ONLCR)

The orchestrator's live smoke at 100×30 showed stair-stepped frames: `term.MakeRaw` clears `OPOST`/`ONLCR`, so `\n` no longer becomes `\r\n` at the tty driver. libuv's `uv_tty_set_mode(UV_TTY_MODE_RAW)` keeps output post-processing on, so the Go session must too. Fix: `terminal.go` re-arms `Oflag |= OPOST | ONLCR` after `MakeRaw` via `x/sys/unix` termios ioctls (`termios_linux.go` TCGETS/TCSETS, `termios_darwin.go` TIOCGETA/TIOCSETA); `Close` still restores the pre-MakeRaw state. `go mod tidy` moved `golang.org/x/sys v0.48.0` from `// indirect` to direct — the only `go.mod` change (A-033 reworded accordingly); no new module.

Gates after the fix: `gofmt -l .`/`go vet ./...` silent; `go test ./... -count=1` all 24 packages ok; `just go-diff --placeholder` **452/452**; `just go-live` **9/9**.

Smoke (100×30 tmux window, `bin/tu -w -i 5 --no-rain`): no stair-stepping, every line at column 1, footer on the last row:

```
 Elapsed  5s           Tok/min   --
 Session  +$0.00       Rate      --
                       Proj. day --
───────────────────────────────────

📊 Combined Usage (daily)

Tool         |       Tokens |        Input |       Output |        Cache |         Cost
─────────────|──────────────|──────────────|──────────────|──────────────|─────────────
Claude Code  |  156,283,017 |          636 |       45,473 |  156,236,908 |       $33.51
Kimi         |  341,042,778 |    6,001,977 |    1,174,858 |  333,865,943 |      $203.68
─────────────|──────────────|──────────────|──────────────|──────────────|─────────────
Total        |  497,325,795 |    6,002,613 |    1,220,331 |  490,102,851 |      $237.19

Next refresh: 1s · ↵ refresh · q quit
```

After `q`, the last frame prints on the normal screen followed by `EXIT=0`:

```
Claude Code  |  156,283,017 |          636 |       45,473 |  156,236,908 |       $33.51
Kimi         |  341,042,778 |    6,001,977 |    1,174,858 |  333,865,943 |      $203.68
Total        |  497,325,795 |    6,002,613 |    1,220,331 |  490,102,851 |      $237.19

EXIT=0
```

## Deletion Candidates

- None — this change adds new functionality without making existing code redundant. The scaffold placeholder (`notImplementedMsg`, `ErrUnported`) stays in use for `--skip-brew-update` on a data command; the pre-existing `Row.Delta` trailing form remains the history-table placement; no symbol, branch or config lost its last caller.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | The RNG is `math/rand/v2` `*rand.Rand` with a PCG source, time-seeded at the edge and fixed in tests | stdlib, injectable, no dependency; the TS `Math.random` sequence is not reproducible anyway | S:70 R:90 A:90 D:80 |
| 2 | Confident | Keys are compared as whole read chunks (single bytes for `q`, `\r`, `\n`, ` `, `\x03`); escape sequences are ignored | Mirrors the TS `data` chunk comparison; arrow keys arrive as multi-byte chunks and are dropped | S:65 R:90 A:85 D:80 |
| 3 | Confident | The loop test drives time through an injected clock and channels rather than real timers (`Options.Now` plus timer/ticker constructor hooks) | Golden frames must not depend on wall-clock; the sibling `Now func()` seam is the precedent | S:60 R:85 A:85 D:75 |
| 4 | Certain | `Fresh = true` on every watch poll and `Interval` validated by `Parse` only | `action(true, …)` in `watch.ts`; Parse already validates 5–3600 | S:85 R:90 A:95 D:95 |
| 5 | Confident | The R9 scenario numbers are computed from the formulas with `FixedHalfUp`; the golden pins whatever the twins produce for those inputs | Hand arithmetic can be off by a cent; the twins are Node-verified | S:60 R:90 A:85 D:80 |

5 assumptions (1 certain, 4 confident, 0 tentative).
