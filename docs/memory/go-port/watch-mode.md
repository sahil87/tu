---
type: memory
description: "The Go port's watch mode — internal/watch: the x/term + os/signal TUI loop (D12; Run's single select over keys/SIGWINCH/SIGINT/poll results/countdown/rain, re-entrancy guard, cleanup returning the last lines), the Terminal seam (80×24 fallback, TTY-gated raw mode keeping OPOST|ONLCR), the pure compositor (Lay/Frame, StatsGrid/BurnRate, footer, skeleton, RainState on an injected rand/v2), golden-frame tests under a fake clock, and the Frame/Stats/Poll contract command.Run fills per poll"
---
# Watch Mode (Go port)

**Domain**: go-port

## Overview

`internal/watch` is the Go port's live-polling watch TUI — `-w`/`--watch` on every display type (snapshot, history, pivot, `lb`, `lbh`) — hand-rolled on `golang.org/x/term` + `os/signal` + `time` with no TUI framework (plan decision D12). One goroutine (`Run`'s) owns every terminal write; everything else — layout, the stats grid, the footer, the skeleton, the rain — is a pure function golden-testable without a terminal. The package imports no `source`, `query`, `view` or render-encoder package: the per-poll table lines arrive through the `Poll` callback that `cmd/tu` composes over `command.Run` ([command-edge](/go-port/command-edge.md)); the render inputs the polls feed (cell deltas, compact tables, the row budget) live in [query-view-render](/go-port/query-view-render.md); the TS behavior being reproduced byte-for-byte is [watch-mode/tui](/watch-mode/tui.md). `render/ansi` supplies `Colors`/`StripANSI`/the pad helpers and `render` the JS rounding/formatting twins.

## Requirements

### Requirement: The Terminal seam
`Terminal` is the loop's I/O seam: `Size() (cols, rows int)`, `Write`, `Keys() <-chan []byte`, `Resize() <-chan struct{}`, `Interrupt() <-chan struct{}`, `Close() error`. `NewTerminal(stdout, stdin *os.File)` is the real implementation over x/term + os/signal: `Size` probes stdout and falls back per axis to 80×24 when stdout is not a TTY or the probe fails (the TS `columns ?? 80` / `rows ?? 24`); stdin enters raw mode (`term.MakeRaw`) and the key reader goroutine starts only when stdin is a TTY (the TS `process.stdin.isTTY` guard — otherwise `Keys()` is nil and SIGINT is the only exit); each stdin `Read` is delivered as one key chunk (the TS per-chunk comparison); SIGWINCH/SIGINT arrive via `signal.Notify` through forwarders that coalesce to one pending event each. `Close` is idempotent — it restores the pre-`MakeRaw` state and stops signal delivery; a key read blocked on stdin is left to the process exit (as the TS just pauses stdin and exits). The release targets are linux and darwin only (D8), so `syscall.SIGWINCH` needs no build tags. Tests use `fakeTerminal` — a settable size, a scripted key channel, manual resize/interrupt channels and a `bytes.Buffer` capturing every write, with a per-write `notify` channel as the synchronization seam (`waitFor` — no wall-clock sleeps).

Raw mode keeps output post-processing: after `MakeRaw`, `keepOutputPostProcessing` re-arms `OPOST|ONLCR` on the tty through `x/sys/unix` termios ioctls (`termios_linux.go` TCGETS/TCSETS, `termios_darwin.go` TIOCGETA/TIOCSETA), because x/term's `MakeRaw` clears `OPOST` (and with it `ONLCR`) while libuv's `uv_tty_set_mode(UV_TTY_MODE_RAW)` keeps `c_oflag |= ONLCR` — the session's bytes assume `\n` becomes `\r\n` at the driver, the mid-session `Warning: fetch failed` stderr lines included. A probe failure is silent: the session still works, only line endings degrade. `Close` restores the pre-`MakeRaw` state, which already had `OPOST` on.

#### Scenario: Non-TTY fallback
- **GIVEN** stdout piped
- **WHEN** `Size()` is called
- **THEN** it returns `(80, 24)`

### Requirement: Run — one select loop owns the terminal
`Run(ctx, o Options) (last []string)` blocks until `q`, Ctrl-C or SIGINT and never calls `os.Exit`. `Options{Interval, NoRain, Poll, Term, Stderr, Colors, Now, Rand, NewTicker, NewTimer}`: `Interval` is in seconds, already validated by `Parse` (5–3600, default 10); `Rand` is `math/rand/v2` (PCG, time-seeded at the edge, seeded in tests); `NewTicker`/`NewTimer` are the timer seams (`Ticker`/`Timer` = a fire channel plus `Stop`; nil installs the real-time constructors) — no test sleeps on wall-clock time. Startup order (the TS `runWatch`): write `\x1b[?1049h` then `\x1b[?25l`; write the skeleton via `SkeletonFrame`; lay out the rain zone against the skeleton lines (`LaySkeleton`); start the 107 ms rain ticker whenever rain is on (the zone may enable on a later layout); then run the first poll — the key, resize and interrupt inputs are already being selected on. In the select loop: a key chunk equal to `q` or `\x03` exits; `\r`, `\n` or ` ` cancels the countdown and polls immediately; any other chunk (arrow keys, multi-byte reads) is ignored. The `polling` flag is the re-entrancy guard — a second poll request while one is in flight is dropped, as in the TS. SIGWINCH is a no-op until the first successful poll (the TS `rerender()` guard — the skeleton's rain zone is NOT re-laid-out on an early resize); afterwards resize re-reads `Size`, re-lays out from the cached table lines, session and cost, and flushes a full frame.

Cleanup runs once on exit: stop the countdown timer and rain ticker, `Term.Close()`, write `\x1b[?25h` then `\x1b[?1049l`; `Run` returns the last successful poll's lines and `cmd/tu` prints each + `\n` to stdout with exit 0.

#### Scenario: Exit restores and returns
- **GIVEN** a session with one successful poll
- **WHEN** `q` arrives
- **THEN** the stream ends with `\x1b[?25h\x1b[?1049l` and `Run` returns that poll's table lines

### Requirement: The poll sequence and the Poll contract
`Poll func(ctx context.Context, f Frame) (Lines []string, Stats Stats, err error)` is the only thing watch knows about `command`. `Frame{Prev map[string]float64, Compact bool, MaxRows int, Width int}` is one poll's render inputs (the TS `FormatOptions` the loop builds): `Prev` is the previous poll's `CostByItem` (nil on the first poll and whenever the previous map was empty — the TS `size > 0` guard), `Compact` is `width < CompactThreshold`, `MaxRows` is the constant 15, `Width` is the live terminal width. `Stats{TotalCost float64, TotalTokens int64, CostByItem map[string]float64}` carries one poll's watch numbers. Each poll (the TS `doPoll`): the footer goes to `Refreshing...`; the `Poll` runs in a worker goroutine delivering on a buffered channel. On error the loop writes `Warning: fetch failed, retrying next cycle` + `\n` to `Stderr` and restarts the countdown with no frame change. On success: `todayCost = Stats.TotalCost`; the first success sets `Session.StartTime`/`StartCost`/`StartTokens`; `{now, cost}` appends to `Session.Polls`; `TotalTokens` is stored; the lines are cached as `last`; the geometry is re-read live, `Lay` + flush run (the flush carries the current `Refreshing...` footer; the countdown's initial footer write follows — the TS flush → `startCountdown` order); `Prev` becomes a clone of `Stats.CostByItem`; the countdown starts at `Interval`.

The countdown (the TS `startCountdown`) is push-driven: the value is set to `Interval` and the footer rendered; a fresh one-second `Timer` per step (the TS `setTimeout` chain) decrements it, re-rendering only the footer line; at 0 a poll fires — the footer never renders `0s`. The footer is rewritten only on these events, never on a periodic compositor tick, and is built against the live `Size`.

#### Scenario: Poll error leaves the frame
- **GIVEN** `Poll` returns an error
- **WHEN** the poll completes
- **THEN** stderr receives exactly `Warning: fetch failed, retrying next cycle\n`, the last frame is not rewritten, and the footer restarts at `Next refresh: {Interval}s`

### Requirement: Lay and Frame — pure layout and frame bytes
`compositor.go` holds the constants (the TS `compositor.ts`/`watch.ts`): `CompactThreshold = 60`, `RainTick = 107ms`, `CountdownTick = 1s`, `minRainCols = 10`, `rainGutter = 2`, `footerRows = 1`, and `MaxRows = 15` — the watch row budget is the TS `maxRows: 15`, a constant, not the spec's "rows that fit the terminal height" (usage § Table semantics, layouts § 7 — the TS bytes are the oracle; flagged for gate G0) (4pze). `Lay(statsLines, tableLines, cols, rows, noRain) Layout` reproduces `layoutAndUpdate` + `setupRainZone`: compact (`cols < 60`) drops the stats lines; `contentHeight = len(stats) + len(table)`; `maxContentWidth` is the max `StripANSI` rune width over the content lines; `available = rows − contentHeight − 1`; `wantRain = !noRain && !compact`. A below-content zone `{cols, available, startRow contentHeight+1, startCol 0}` when `wantRain && available > 0`; else a right-margin zone `{cols − maxContentWidth − 2, rows − 1, startRow 1, startCol maxContentWidth + 2}` when that width is ≥ 10; else disabled. `Layout{Compact, Stats, Table, Rain, Rows}` is the layout record (`Rain` carries `Enabled`, `Cols`, `Rows`, 1-based `StartRow`, 0-based `StartCol`). `LaySkeleton(lines, …)` is `Lay` with the skeleton's own lines as the whole content (the TS `layoutForSkeleton`).

`Layout.Frame(footer, rain) []byte` reproduces `flush`: `\x1b[H`; each content line (stats then table) + `\x1b[K\n`; `\x1b[J`; the footer as `\x1b[{rows};1H\x1b[K{footer}`; then the current rain frame re-emitted — the flush's clears erased every drawn rain cell, and the rain renderer rewrites every occupied cell, so the re-emit restores them in the same write and polls do not blink the rain. `FooterLine(footer, rows)` is the footer-only write (the TS `renderStatus`). `setupRain` applies a zone to the rain state: a disabled or degenerate zone drops the state; an existing state `Resize`s (a no-op when the geometry is unchanged, so the first real poll after the skeleton keeps the drops when they match); otherwise a new state is created.

#### Scenario: Zone selection
- **GIVEN** 100×30 with 4 stats lines and 12 table lines, rain on
- **WHEN** `Lay` runs
- **THEN** the zone is `{100, 13, 17, 0}`; at 100×12 with a widest line of 87 the zone is `{11, 11, 1, 89}`, and at a widest line of 89 the rain is disabled (`marginCols` 9)

### Requirement: The stats grid
`panel.go` reproduces the TS `panel.ts` with `now` injected. `Session{StartTime, StartCost, StartTokens, Polls []PollPoint, TotalTokens}` is the session state; `PollPoint{At, Cost}` is one successful poll. `FormatElapsed(d)` renders floor seconds as `Xh Xm Xs` / `Xm Xs` / `Xs` (negative durations clamp to 0). `BurnRate(polls) (perHour float64, ok bool)` is $/hour over the last `rollingWindow = 5` polls: `!ok` below two polls, `0` when the window's Δt is 0. `StatsGrid(session, todayCost, now, colors)` returns 3 rows plus a `Dim` `─` separator sized to the widest row's visible width: row 1 `Elapsed` / `Tok/min`, row 2 `Session` / `Rate`, row 3 blank / `Proj. day`. Values: **Elapsed** = `FormatElapsed(now − StartTime)`; **Session** = `$0.00` before two polls, else the sign (`+` when the delta is ≥ 0) + `$` + `FixedHalfUp(|latest.Cost − StartCost|, 2)`; **Tok/min** = `--` until two polls, then `~` + `FormatInt(JSRound(sessionTokens / elapsedMin))` when the session tokens and elapsed minutes are positive; **Rate** = `~$` + `FixedHalfUp(rate, 2)` + `/hr` in Yellow when the burn rate is positive, else `--`; **Proj. day** shows only when Rate does: `~$` + `FixedHalfUp(todayCost + rate × hoursRemaining, 2)` with `hoursRemaining = 24 − now.Hour() − now.Minute()/60` in LOCAL time; `todayCost` is the rendered rows' total, whatever the display (the TS passes `getCost()` unchanged). `gridRow` (the TS `formatGridRow`): `Dim(" " + PadRight(label, 9)) + BoldWhite(value)` on the left (12 spaces for the blank row-3 left), then `max(1, 23 − leftVisible)` spaces, then `Dim(PadRight(rightLabel, 10))` + the value (Yellow on the Rate row when shown, BoldWhite otherwise). The numbers are the `render` twins `FixedHalfUp` (`toFixed`), `JSRound` (`Math.round`) and `FormatInt` (`toLocaleString`).

### Requirement: The footer
`Footer(countdown, refreshing, width, colors)` (the TS `StatusPanel.render`): `Dim("Next refresh: {n}s")` — or `Dim("Refreshing...")` — and `Dim("↵ refresh · q quit")`, joined with `Dim(" · ")`; while the visible (`StripANSI`) width exceeds the terminal width and more than one part remains, the last part is dropped and the parts re-joined (the controls hint goes first, then only the status text remains).

### Requirement: The skeleton
`Skeleton(cols, now, colors)` reproduces `renderSkeleton(termWidth)` — the loading frame written on alt-screen entry before the first poll. Full mode: the stats grid for a zeroed session (`Elapsed 0s`, `Session $0.00`, the rest `--`); `""`; `BoldWhite("📊 Combined Usage (daily)")` — the daily snapshot header on EVERY display type (the TS builds it unconditionally; layouts § 8 shows it for `tu -w` only — reproduced, not "improved", and flagged for gate G0) (4pze); `""`; the `BoldCyan` snapshot header (`Tool` `PadRight` 12, the rest `PadLeft` 12, joined ` | `); the `Dim` 87-char divider; then 38 spaces + `Dim("Loading...")` (`floor((87 − 10) / 2)`). Compact (`cols < 60`): `""` then `Dim("Loading...")`. `SkeletonFrame(lines)` is the write form: `\x1b[H` + each line + `\n` (no `\x1b[K`).

### Requirement: RainState
`rain.go` reproduces `rain.ts` with an injected `*rand.Rand` and `Colors`. The character pool is half-width katakana + digits + `A–Za–z` (every character is BMP, so rune indexing matches the TS code-unit indexing). Constants: density 0.3 calibrated at `rainDensityRefRows = 20` with the 3× scale cap, speed 0.3–1.0 rows/tick, trail length 3–8, respawn delay 0–5 ticks, shimmer 0.05. `ActiveDropCount(cols, rows)` = `JSRound(cols × 0.3 × min(3, max(1, rows/20)))` — the count scales with the zone HEIGHT (1× at ≤ 20 rows, 3× at 60+). `NewRainState(cols, rows, startCol, rng, colors)` scatters the initial drops (`initDrops`): a Fisher–Yates shuffle of the column indices, then drop *i* on `columns[i % cols]` with `row ∈ (−length, rows)` and no delay. The RNG draw order per drop is length, the chars (length draws), the scattered row (init only), the speed, and the respawn delay (respawn only) — the seeded golden depends on it. `Tick`: a delayed drop counts down; others move by their speed, each trail char is re-rolled with probability 0.05, and a drop fully off-screen (`floor(row) − length > rows`) respawns in place (`row = −length`, new speed, length, delay and chars). `Render(startRow)` emits, for each active drop and trail index, `\x1b[{startRow+r};{startCol+col+1}H` + the colored char (`BrightGreen` head, `Green` body, `DimGreen` the last two), then a `\x1b[{r};{c}H ` clear (a space) for every previously occupied cell not occupied now — the previous frame's cells are kept in insertion order (the JS `Set`'s iteration order) so the clears are deterministic; `""` when `rows <= 0`. `Resize` keeps the drops when `(cols, rows, startCol)` are unchanged and re-initialises otherwise (`startRow` is a render-time input, not part of the geometry).

### Requirement: Golden frames and the loop test are the harness gate
A `--once`-style frame capture against the TS binary is not possible under the port plan's Goal: a session frame depends on wall-clock (Elapsed, the countdown) and on `Math.random` (the rain), so a byte-diff between the two binaries would need a capture switch on BOTH sides — a new external surface the Goal forbids — and a Go-only switch has nothing to diff against. The gate is therefore (4pze): **(a)** golden frames from the pure compositor under an injected clock and a seeded RNG — `internal/watch/testdata/`: `skeleton_100x30`, `skeleton_50x20`, `frame_first_poll_100x30`, `frame_second_poll_100x30` (the clock advanced 11 s, cost +0.07 — populated Rate/Proj. day), `frame_compact_59x20`, `frame_rightmargin_100x12` (zone at `maxContentWidth + 2`), `frame_norain_100x30`, `rain_seeded` (one tick, one render, one clear pass) — behind the package `-update` flag as in `render/ansi`; **(b)** a loop test against the fake terminal with manual timers covering skeleton → first poll → `q` (returns the poll's lines), `\r` cancelling the countdown (a second `\r` mid-poll dropped by the re-entrancy guard), SIGWINCH before the first poll (writes nothing) and after (re-flushes), a `Poll` error (stderr warning, frame unchanged), and the `\x03` and SIGINT exits, with the stream ending `\x1b[?25h\x1b[?1049l` — no wall-clock sleeps; **(c)** the seven deterministic watch-family matrix cases ([differential-harness](/harness/differential-harness.md)) — the exit-2 incompatibilities and interval bounds and the DC-03 silent-acceptance cases, byte-diffable today and exercising the same parse and dispatch path. Table-driven tests pin `FormatElapsed`, `BurnRate` (the window, Δt = 0, a negative rate → `--`), the `StatsGrid` values (the `$0.00` before two polls, the sign rule), `ActiveDropCount` (1× at ≤ 20 rows, 3× at 60+), the footer truncation at 30/20/10 columns, and `Lay`'s zone table (below-content vs right-margin at 10/9 margin columns vs disabled). The `cmd/tu` e2e covers `--no-rain` and `--interval 30` rendering the ordinary one-shot table and the notice-once ordering through the `newWatchTerminal`/`runWatchLoop` seams ([command-edge](/go-port/command-edge.md)).

## Design Decisions

### Pure compositor under an injected clock and RNG
**Decision**: `Lay`/`Frame`, `StatsGrid`, the footer, the skeleton and `RainState` are pure functions of (state, size, `now`, `*rand.Rand`); only `Run` touches timers and the `Terminal`.
**Why**: the harness cannot byte-diff a time- and RNG-dependent session without a new external surface; goldens under a fake clock are the gate D12 names.
**Rejected**: a `--once` flag or `TU_WATCH_ONCE` env on both binaries — a new external surface the plan's Goal forbids.
*Introduced by*: 260917-4pze-watch-mode-tui

### Cell-level delta with two padding placements
**Decision**: `Cell.Delta` + `Table.DeltaInCell` (`DeltaPadsArrow` for snapshot and compact, `DeltaAfterPad` for the leaderboard); the history tables keep the trailing `Row.Delta`.
**Why**: the TS places the arrow three different ways and pads by JS raw string length; `PadLeft`'s rune count over the colored string reproduces that quirk with no special casing.
**Rejected**: normalising every table to the trailing form — it changes bytes on the snapshot and leaderboard.
*Introduced by*: 260917-4pze-watch-mode-tui

### Compact tables as their own model
**Decision**: `view.CompactTable` + `ansi.CompactTable`, separate from `view.Table`.
**Why**: two space-joined columns, no header row, no gutters — a different shape; the full `Table` encoder stays untouched.
**Rejected**: a `Compact bool` on `Table` threaded through `ansi.Table` — every encoder branch would grow a compact arm.
*Introduced by*: 260917-4pze-watch-mode-tui

### Live options on Deps, not on Request
**Decision**: `Deps.Live *LiveOptions{Prev, Compact, MaxRows}` and per-poll `Deps.Width`.
**Why**: they are render state of the live session, not CLI grammar; `Deps` is already the per-call value the edge composes.
**Rejected**: `Request.Flags` fields — they would leak into `Parse`/`Normalize` and the help text seam.
*Introduced by*: 260917-4pze-watch-mode-tui

### Guard notices once, at the edge
**Decision**: `cmd/tu` calls `command.Normalize` once before entering the alt screen, prints those notices, and discards per-poll `Result.Notices`.
**Why**: the TS prints the guards in `main()` ahead of `runWatch`; `Normalize` does not clear `--full`, so its notice would repeat every poll if taken from `Run`.
**Rejected**: a `Deps.Quiet` flag suppressing notices inside `Run` — more surface for the same bytes.
*Introduced by*: 260917-4pze-watch-mode-tui

### Raw mode keeps output post-processing
**Decision**: after `term.MakeRaw`, the terminal re-arms `OPOST|ONLCR` through `x/sys/unix` termios ioctls (`TCGETS`/`TCSETS` on linux, `TIOCGETA`/`TIOCSETA` on darwin); `Close` restores the pre-`MakeRaw` state.
**Why**: x/term's `MakeRaw` clears `OPOST` (and with it `ONLCR`), so the compositor's `\n` stops becoming `\r\n` at the tty driver and frames stair-step; libuv's raw mode keeps output post-processing on and the TS session's bytes assume it, the mid-session stderr warning lines included.
**Rejected**: emitting `\r\n` from the compositor (diverges from the TS byte producer the goldens pin); stdlib syscalls in place of `x/sys/unix` (the ioctl constants differ per OS and `x/sys` already wraps both).
*Introduced by*: 260917-4pze-watch-mode-tui
