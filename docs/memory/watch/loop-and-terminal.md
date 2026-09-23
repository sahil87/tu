---
type: memory
description: Watch loop and terminal seam — Run's single select over keys/SIGWINCH/SIGINT/poll results/countdown/rain with a re-entrancy guard and cleanup returning the last lines, the Terminal seam with the 80×24 fallback, and TTY-gated raw mode that keeps OPOST|ONLCR via per-OS termios ioctls
---

# Watch Loop and Terminal

**Domain**: watch

## Overview

`watch.Run` (`internal/watch/watch.go`) is the live-polling driver behind `-w`/`--watch`: one goroutine owns every terminal write and blocks on a single `select` over keys, SIGWINCH, SIGINT, poll results, the countdown and the rain ticker until `q`, Ctrl-C or SIGINT. `Terminal` (`internal/watch/terminal.go`) is the I/O seam — real over `golang.org/x/term` + `os/signal`, faked in tests — with raw mode that keeps output post-processing through the per-OS termios shims `termios_linux.go`/`termios_darwin.go`. The `-w` branch and the print-on-exit of the returned lines live in [entry-point](/command/entry-point.md); the `Poll` callback composes [run-and-result](/command/run-and-result.md).

## Requirements

### Requirement: Startup order
`watch.Run(ctx, Options) (last []string)` MUST write `\x1b[?1049h` then `\x1b[?25l`, then write the skeleton via `SkeletonFrame`, lay out the rain zone against the skeleton lines with `LaySkeleton`, start the `RainTick` ticker whenever rain is on (the zone may enable only on a later layout), and only then run the first poll — the key, resize and interrupt inputs are already being selected on (`internal/watch/watch.go`). `Options{Interval, NoRain, Poll, Term, Stderr, Colors, Now, Rand, NewTicker, NewTimer}`: `Interval` is seconds, already validated by `command.Parse` (default 10, range 5–3600, `internal/command/parse.go`); `NewTicker`/`NewTimer` are the timer seams, nil installing the real-time constructors.

### Requirement: The Terminal seam
`Terminal` (`internal/watch/terminal.go`) MUST expose `Size() (cols, rows int)`, `Write`, `Keys() <-chan []byte`, `Resize() <-chan struct{}`, `Interrupt() <-chan struct{}`, and an idempotent `Close() error`. `NewTerminal(stdout, stdin)` MUST probe stdout for the geometry, enter raw mode and start the key reader only when stdin is a TTY (otherwise `Keys()` is nil and SIGINT is the only exit), deliver each stdin `Read` as one key chunk, and forward SIGWINCH/SIGINT via `signal.Notify`, coalescing to one pending event per channel. `Close` MUST be idempotent, restore the pre-`MakeRaw` state, and stop signal delivery; a key read blocked on stdin is left to the process exit. Tests use `fakeTerminal` (`internal/watch/faketerm_test.go`) — settable size, scripted keys, manual resize/interrupt channels, a `bytes.Buffer` per write with a `notify` seam (`waitFor`, no wall-clock sleeps).

#### Scenario: Non-TTY fallback
- **GIVEN** stdout is piped (not a TTY)
- **WHEN** `Size()` is called
- **THEN** it returns `(80, 24)` — `fallbackCols`/`fallbackRows` in `internal/watch/terminal.go`, each axis falling back independently of the other

### Requirement: Raw mode keeps output post-processing
After `term.MakeRaw`, `keepOutputPostProcessing` MUST re-arm `OPOST|ONLCR` on the tty through `x/sys/unix` termios ioctls — `TCGETS`/`TCSETS` on linux (`internal/watch/termios_linux.go`), `TIOCGETA`/`TIOCSETA` on darwin (`internal/watch/termios_darwin.go`). A probe failure MUST be silent: the session still works, only line endings degrade. `Close` restores the pre-`MakeRaw` state, which already has `OPOST` on. The release targets are linux and darwin only, so `syscall.SIGWINCH` needs no build tags.

### Requirement: One select loop owns the terminal
`watch.Run` MUST block until `q`, Ctrl-C or SIGINT and never call `os.Exit`; one goroutine owns every terminal write. In the `select` (`internal/watch/watch.go`): a key chunk equal to `q` or `\x03` exits; `\r`, `\n` or ` ` cancels the countdown and polls immediately; any other chunk (arrow keys, multi-byte reads) is ignored. The `polling` flag is the re-entrancy guard — a second poll request while one is in flight MUST be dropped. SIGWINCH MUST be a no-op until the first successful poll (the skeleton's rain zone is not re-laid-out on an early resize); afterwards resize re-reads `Size`, re-lays out from the cached lines/session/cost with `Lay`, and flushes a full frame.

### Requirement: The poll sequence and the Poll contract
`Poll func(ctx, f Frame) (Lines []string, Stats Stats, err error)` is the only thing watch knows of `command` ([run-and-result](/command/run-and-result.md) — the edge fills `Deps.Live` from `Frame`). `Frame{Prev, Compact, MaxRows, Width}` MUST carry the previous poll's `CostByItem` (nil on the first poll and whenever the previous map was empty), `Compact = width < CompactThreshold`, `MaxRows = MaxRows`, and the live width. Each poll sets the footer to `Refreshing...` and runs `Poll` in a worker goroutine delivering on a buffered channel. On error the loop MUST write exactly `Warning: fetch failed, retrying next cycle\n` to `Stderr` and restart the countdown with no frame change. On success it MUST update the `Session` (first success sets `StartTime`/`StartCost`/`StartTokens`; `{now, cost}` appends to `Polls`), cache the lines as `last`, re-read the geometry live, flush a full frame carrying the current `Refreshing...` footer, clone `Stats.CostByItem` into `Prev` (nil when empty), and start the countdown at `Interval`.

#### Scenario: Poll error leaves the frame
- **GIVEN** `Poll` returns an error
- **WHEN** the poll completes
- **THEN** stderr receives exactly `Warning: fetch failed, retrying next cycle\n`, the last frame is not rewritten, and the footer restarts at `Next refresh: {Interval}s`

### Requirement: Push-driven countdown
The countdown MUST be push-driven: set to `Interval` with the footer rendered, then a fresh 1 s `Timer` (`CountdownTick`) per step decrements and re-renders only the footer; at 0 a poll fires and the footer never renders `0s` (`startCountdown` in `internal/watch/watch.go`). The footer is rewritten only on countdown state changes and is built against the live `Size`.

### Requirement: Cleanup returns the last lines
Cleanup MUST run once on exit: stop the countdown timer and rain ticker, `Term.Close()`, write `\x1b[?25h` then `\x1b[?1049l`. `Run` MUST return the last successful poll's lines and never call `os.Exit` — [entry-point](/command/entry-point.md) prints the returned lines on the normal screen.

#### Scenario: Exit restores and returns
- **GIVEN** a session with one successful poll
- **WHEN** `q` arrives
- **THEN** the stream ends with `\x1b[?25h\x1b[?1049l` and `Run` returns that poll's table lines

## Design Decisions

### Raw mode keeps output post-processing
**Decision**: after `term.MakeRaw`, `keepOutputPostProcessing` re-arms `OPOST|ONLCR` through `x/sys/unix` termios ioctls (`TCGETS`/`TCSETS` on linux, `TIOCGETA`/`TIOCSETA` on darwin); `Close` restores the pre-`MakeRaw` state.
**Why**: x/term's `MakeRaw` clears `OPOST` (and with it `ONLCR`), so the compositor's `\n` stops becoming `\r\n` at the tty driver and frames stair-step; the session's bytes assume `\n` becomes `\r\n` at the driver, the mid-session stderr warning lines included — parity with the frozen `src/node/` oracle (until plan row Z1). A probe failure is silent: the session still works, only line endings degrade.
**Rejected**: emitting `\r\n` from the compositor (diverges from the byte producer the goldens pin); stdlib syscalls instead of `x/sys/unix` (the ioctl request constants differ per OS and `x/sys` already wraps both). (4pze)
*Introduced by*: 260917-4pze-watch-mode-tui

### Per-axis 80×24 geometry fallback
**Decision**: `Terminal.Size` falls back per axis to `fallbackCols = 80` / `fallbackRows = 24` when stdout is not a TTY, the probe fails, or an axis is non-positive.
**Why**: layout, footer positioning and the rain zone always need a geometry; independent per-axis fallback keeps a partially valid probe (one bad axis) usable.
**Rejected**: failing the session on a probe error — watch still works at a fixed geometry. (4pze)
*Introduced by*: 260917-4pze-watch-mode-tui

### Per-OS termios shims for linux and darwin only
**Decision**: `getTermios`/`setTermios` live behind `//go:build linux` (`TCGETS`/`TCSETS`) and `//go:build darwin` (`TIOCGETA`/`TIOCSETA`); `syscall.SIGWINCH` needs no build tags.
**Why**: the release targets are linux and darwin only, so two small per-OS files beat an abstraction over ioctl request constants.
**Rejected**: stdlib syscalls — the ioctl request constants differ per OS and `x/sys/unix` already wraps both. (4pze)
*Introduced by*: 260917-4pze-watch-mode-tui

### One goroutine owns the terminal; timers and RNG are seams
**Decision**: one goroutine (`watch.Run`) owns every terminal write; `Options.NewTicker`/`NewTimer`/`Now`/`Rand` are injected seams, nil installing the real-time constructors.
**Why**: with every write on one goroutine and `Ticker`/`Timer`/`Now`/`Rand` injected, loop tests run against `fakeTerminal` with manual timers and a `notify` write seam — no wall-clock sleeps ([compositor](/watch/compositor.md) covers the golden gate).
**Rejected**: a TUI framework — an extra dependency whose byte stream the goldens could not pin. (4pze)
*Introduced by*: 260917-4pze-watch-mode-tui
