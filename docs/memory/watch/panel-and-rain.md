---
type: memory
description: Watch stats panel and rain layer — Session/PollPoint state, FormatElapsed, BurnRate over the last 5 polls, the fixed 2×3 StatsGrid with its spacing constants, and RainState on an injected math/rand/v2 RNG with height-scaled density, shimmer and deterministic clears; --no-rain removes the ticker entirely
---

# Watch Panel and Rain

**Domain**: watch

## Overview

`internal/watch/panel.go` renders the 2×3 stats grid above the table from the accumulated `Session`, and `internal/watch/rain.go` is the matrix-rain overlay — a state machine on an injected `math/rand/v2` RNG whose frames are cursor-positioned writes. Both are pure given the clock and RNG; the loop that feeds them is [loop-and-terminal](/watch/loop-and-terminal.md), the zones and frame bytes are [compositor](/watch/compositor.md), and the number/color helpers are [number-formatting](/render/number-formatting.md) and [ansi](/render/ansi.md).

## Requirements

### Requirement: Session state
`Session{StartTime, StartCost, StartTokens, Polls []PollPoint, TotalTokens}` (`internal/watch/panel.go`) MUST accumulate one `PollPoint{At, Cost}` per successful poll; the first successful poll MUST set `StartTime`/`StartCost`/`StartTokens`, and every success MUST refresh `TotalTokens`.

### Requirement: FormatElapsed
`FormatElapsed(d)` (`internal/watch/panel.go`) MUST render floor seconds as `Xh Xm Xs` when hours > 0, else `Xm Xs` when minutes > 0, else `Xs`; negative durations MUST clamp to 0.

### Requirement: BurnRate
`BurnRate(polls) (perHour float64, ok bool)` (`internal/watch/panel.go`) MUST compute $/hour over the last `rollingWindow = 5` polls: `ok = false` below two polls, and `(0, true)` when the window's Δt is 0.

### Requirement: The stats grid
`StatsGrid(session, todayCost, now, colors)` (`internal/watch/panel.go`) MUST return 3 rows plus a `Dim` `─` separator sized to the widest row's visible width: row 1 `Elapsed` / `Tok/min`, row 2 `Session` / `Rate`, row 3 blank / `Proj. day`. Values: **Elapsed** = `FormatElapsed(now − StartTime)`; **Session** = `$0.00` before two polls, else the sign (`+` when the delta is ≥ 0) + `$` + `render.FixedHalfUp(|latest.Cost − StartCost|, 2)`; **Tok/min** = `--` until two polls, then `~` + `render.FormatInt(render.JSRound(sessionTokens / elapsedMin))` when the session tokens and elapsed minutes are positive; **Rate** = `~$` + `render.FixedHalfUp(rate, 2)` + `/hr` in Yellow when `BurnRate` is positive, else `--`; **Proj. day** MUST show only when Rate does: `~$` + `render.FixedHalfUp(todayCost + rate × hoursRemaining, 2)` with `hoursRemaining = 24 − now.Hour() − now.Minute()/60` in LOCAL time; `todayCost` is the rendered rows' total, whatever the display. The number helpers are the [number-formatting](/render/number-formatting.md) twins `render.FixedHalfUp`, `render.JSRound` and `render.FormatInt`; colors come from [ansi](/render/ansi.md).

#### Scenario: First poll shows placeholders
- **GIVEN** a `Session` with one poll
- **WHEN** `StatsGrid` runs
- **THEN** Session shows `$0.00`, Tok/min / Rate / Proj. day show `--`, and the grid stays 3 rows plus separator

### Requirement: Grid row spacing
`gridRow` (`internal/watch/panel.go`) MUST build `Dim(" " + PadRight(label, 9)) + BoldWhite(value)` on the left (12 spaces for the blank row-3 left), then `max(1, 23 − leftVisible)` spaces, then `Dim(PadRight(rightLabel, 10)) + value` — Yellow on the Rate row when a value is shown, BoldWhite otherwise. The layout constants MUST be `leftLabelW = 9`, `maxLeftValueW = 8`, `gridGap = 5`, `rightLabelW = 10`, `placeholder = "--"`.

#### Scenario: Placeholders hold the layout
- **GIVEN** fewer than two polls
- **WHEN** `StatsGrid` runs
- **THEN** the grid stays 3 rows plus the separator, with `--` placeholders and the blank row-3 left, so the rain zone below does not shift as stats arrive

### Requirement: RainState on an injected RNG
`RainState` (`internal/watch/rain.go`) MUST take an injected `*rand.Rand` (`math/rand/v2`) and `ansi.Colors`. The character pool MUST be half-width katakana + digits + `A–Za–z` (every character is BMP, so rune indexing matches the calibrated draw order). The tuning constants MUST be `rainDensity = 0.3` calibrated at `rainDensityRefRows = 20` with the `rainMaxDensityScale = 3` cap, speed 0.3–1.0 rows/tick (`rainMinSpeed`/`rainMaxSpeed`), trail length 3–8 (`rainMinLength`/`rainMaxLength`), respawn delay 0–5 ticks (`rainMaxRespawnDelay`), shimmer 0.05 (`rainShimmerRate`). `ActiveDropCount(cols, rows)` MUST equal `render.JSRound(cols × 0.3 × min(3, max(1, rows/20)))` — the count scales with the zone HEIGHT: 1× at ≤ 20 rows, 3× at 60+. `NewRainState(cols, rows, startCol, rng, colors)` MUST scatter the initial drops (`initDrops`): a Fisher–Yates shuffle of the column indices, then drop `i` on `columns[i % cols]` with `row ∈ (−length, rows)` and no delay. The RNG draw order per drop MUST be length, the chars (length draws), the scattered row (init only), the speed, and the respawn delay (respawn only) — the `rain_seeded` golden depends on it.

#### Scenario: Density scales with zone height
- **GIVEN** rain zones of 20 rows and of 60 rows at the same width
- **WHEN** `ActiveDropCount(cols, rows)` runs
- **THEN** the 20-row zone gets `JSRound(cols × 0.3)` drops and the 60-row zone 3× that; at or below 20 rows the scale is clamped to 1×

### Requirement: Tick and Render
`Tick` (`internal/watch/rain.go`) MUST: count down a delayed drop; otherwise advance by speed, re-roll each trail char with probability `rainShimmerRate` (0.05), and respawn in place when fully off-screen (`floor(row) − length > rows`: back to `−length` with new speed, length, delay and chars). `Render(startRow)` MUST emit, per active drop and trail index, `\x1b[{startRow+row};{startCol+col+1}H` + the colored char (`BrightGreen` head, `Green` body, `DimGreen` the last two), then a `\x1b[{r};{c}H ` clear for every cell occupied by the prior frame but not the current one; the prior frame's cells MUST be kept in insertion order so the clears are deterministic. `Render` MUST return `""` when `rows <= 0`. `Resize` MUST keep the drops when `(cols, rows, startCol)` are unchanged and re-initialise otherwise (`startRow` is a render-time input, not part of the geometry).

#### Scenario: Seeded rain is deterministic
- **GIVEN** a seeded `*rand.Rand`
- **WHEN** one tick, one render and one clear pass run
- **THEN** the bytes match `internal/watch/testdata/rain_seeded.golden` — the draw order (length, chars, scattered row, speed, respawn delay) is pinned by the golden

### Requirement: --no-rain
With `Options.NoRain` set, `watch.Run` MUST NOT start the rain ticker, `Lay` MUST disable the zone (`wantRain = !noRain && !compact`), and the stream MUST contain no rain bytes (`internal/watch/testdata/frame_norain_100x30.golden`). Compact mode (`cols < CompactThreshold`) MUST also disable the zone.

## Design Decisions

### Height-scaled drop count
**Decision**: the active drop count scales with the rain-zone height — `ActiveDropCount = render.JSRound(cols × rainDensity × min(rainMaxDensityScale, max(1, rows/rainDensityRefRows)))` — not with width alone.
**Why**: the 0.3 density is calibrated at the 20-row reference height, so a fixed count lets coverage decay as the zone grows taller; scaling the count (not trail length or speed) preserves the calibrated per-drop look while restoring roughly constant coverage, clamped to 1× at/below the 20-row reference and capped at 3× to bound per-frame ANSI output on very tall terminals.
**Rejected**: scaling trail length or speed — changes the per-drop look the density is calibrated against. (bom7)
*Introduced by*: 260717-bom7-rain-density-scale-height

### Insertion-order occupancy for deterministic clears
**Decision**: `RainState` keeps the previous frame's occupied cells in insertion order so the vacated-cell clears emit in a deterministic sequence.
**Why**: the `rain_seeded` golden pins one tick, one render and one clear pass byte-for-byte; a map iteration order would make the clear sequence nondeterministic.
**Rejected**: sorting the key set — same bytes, more code. (4pze)
*Introduced by*: 260917-4pze-watch-mode-tui

### Fixed RNG draw order per drop
**Decision**: the RNG draw order per drop is fixed as length, the chars (length draws), the scattered row (init only), the speed, and the respawn delay (respawn only).
**Why**: `internal/watch/testdata/rain_seeded.golden` pins one tick, one render and one clear pass byte-for-byte against a seeded `math/rand/v2` RNG; a reordered draw would change every downstream byte.
**Rejected**: drawing lazily at render time — the golden would be unable to separate tick behavior from render behavior. (4pze)
*Introduced by*: 260917-4pze-watch-mode-tui

### Rain opt-out removes the ticker entirely
**Decision**: `--no-rain` means no ticker, no zone and no state — not a hidden ticker that still ticks; compact mode likewise disables the zone.
**Why**: the flag exists for users who find the animation distracting or costly; an idle ticker would still wake the loop every `RainTick` (107 ms) for zero output.
**Rejected**: ticking and discarding the frame — the same wakeups for zero bytes. (4pze)
*Introduced by*: 260917-4pze-watch-mode-tui

### Burn rate over a rolling 5-poll window
**Decision**: `BurnRate` averages $/hour over the last `rollingWindow = 5` polls.
**Why**: a two-poll instantaneous rate swings wildly with fetch jitter; five polls smooth the jitter while still reacting within a minute at the default 10 s interval.
**Rejected**: a cumulative since-session-start rate — an early burst would dominate the displayed rate for the whole session. (4pze)
*Introduced by*: 260917-4pze-watch-mode-tui

### Fixed 3-row grid with placeholders
**Decision**: the stats grid stays at 3 rows plus separator even when stats are unavailable, showing `--` placeholders (`$0.00` for Session).
**Why**: the rain zone below the content keys off a stable content height; collapsing unavailable rows would re-lay out the frame on every early poll.
**Rejected**: collapsing unavailable rows — every early poll would re-lay out the frame and shift the rain zone. (4pze)
*Introduced by*: 260917-4pze-watch-mode-tui
