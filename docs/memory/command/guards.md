---
type: memory
description: internal/command guards — Normalize's warn-and-clear flag guards (since/until off history, -u in single mode, --top off-leaderboard, --by-machine on the pivot/lbh, --full off history), the 3-month cap and --full, the reserved `all` user, and the leaderboard multi-mode gate
---
# Guards

**Domain**: command

## Overview

`command.Normalize` (`guards.go`) is the pure warn-and-clear guard step between [parse](/command/request-and-parse.md) and [Run](/command/run-and-result.md): it takes the post-guard `config.Mode` and returns the request the pipeline runs plus the stderr notice lines the edge prints before any fetch warning. Two fail-fast guards flank it — the reserved-user check in `cmd/tu` ([entry-point](/command/entry-point.md)) and `command.ErrLeaderboardMode` in `Run`.

## Requirements

### Requirement: Normalize is pure and ordered
`Normalize(req, mode config.Mode, now time.Time) (Request, []string, bool)` (`guards.go`) applies the guards in a fixed order and returns the adjusted request, the notice lines, and the `capActive` marker. It writes nothing: the notices ride the `Result` and the edge prints them BEFORE any fetch warning ([errors-and-warnings](/source/errors-and-warnings.md)). `leaderboard(d)` reports whether the display is `lb`/`lbh`; the guard steps that exempt them test it. (9ax5)

### Requirement: -u in single mode warns and clears
`Flags.User` set in single mode on a display outside {lb, lbh} → notice `Warning: -u flag requires multi mode — ignoring.`; `Flags.User` cleared. The leaderboards are exempt so the exit-1 `ErrLeaderboardMode` gate — which runs in `Run` BEFORE `Normalize` — fires without a notice line; in multi mode this step never runs. On lb/lbh, `User == "all"` is cleared SILENTLY (the leaderboard is inherently all-users), while `-u <name>` is kept: it pins that user's row (the `◂` marker), never filters ([leaderboard](/view/leaderboard.md)). (4xwg) (2gbb)

#### Scenario: -u on lb in single mode fails without a notice
- **GIVEN** `tu lb -u bob` in single mode
- **WHEN** `Run` executes
- **THEN** stderr carries only `Error: lb requires multi mode — run tu init-metrics <repo-url> to set up a metrics repo`, exit 1, with no `-u` notice line

### Requirement: --by-machine guards
`Flags.ByMachine` on the all-tools history pivot (`Source == "" && Display == History`) → notice `Warning: --by-machine is not supported with all-tools history — ignoring.`; cleared. On `LeaderboardHistory` → notice `Warning: --by-machine is not supported with leaderboard history — ignoring.`; cleared. The single-tool history, both snapshots, and `lb` keep the flag — on `lb` it keys rows by `user/machine` ([breakdown](/view/breakdown.md), [multi-mode](/command/multi-mode.md)). (pmsd) (2gbb)

### Requirement: since/until off the history displays warns and clears
`Since`/`Until` set on a display outside {h, lb, lbh} → notice `Warning: --since/--until apply to history display — ignoring.`; both cleared, so the snapshot is in scope after the clear. The leaderboards keep the window — on `lb` it REPLACES the period window ([periods-and-windows](/query/periods-and-windows.md)). The `-u` notice precedes this one when both apply. (sntl) (4xwg)

#### Scenario: A well-shaped impossible date warns and renders
- **GIVEN** `tu h --since 2026-13-01` (shape-valid, impossible)
- **WHEN** parsed and normalized
- **THEN** `Parse` accepts the shape, the window is empty, the display renders empty with exit 0; on a snapshot display the same flag would warn and clear instead, exit 0

### Requirement: --full and --top guards
`Flags.Full` on a display outside {h, lbh} → notice `Warning: --full applies to daily/weekly history — ignoring.`; the flag is left set (nothing reads it downstream). `lb --full` warns like any snapshot; `lbh` and `h` keep it. `Flags.Top` set on a display outside {lb, lbh} → notice `Warning: --top applies to leaderboard display — ignoring.`; cleared — positioned after the `--full` notice and before the cap. (yuuj) (4xwg) (2gbb)

#### Scenario: --top off the leaderboards warns and renders
- **GIVEN** `tu --top 3` (a snapshot)
- **WHEN** normalized
- **THEN** stderr carries `Warning: --top applies to leaderboard display — ignoring.`, `Top` is cleared, and the snapshot renders with exit 0

### Requirement: The 3-month cap and the --full escape
When display ∈ {h, lbh} ∧ period ≠ monthly ∧ `Since == "" && Until == ""` ∧ `!Full`, `Normalize` sets `Since = query.ThreeMonthFloor(now)` and returns `capActive = true`. `query.ThreeMonthFloor` (`internal/query/period.go`) returns the first day of the local month two months back — a window covering three calendar months including the current one. An explicit bound on either side disables the cap entirely (no intersection); `--full` disables it (`mh --full` is a silent no-op, monthly being exempt); `lb` is never capped. The defaulted floor windows repo-sourced records identically to live ones — the window runs after the merge ([multi-mode](/command/multi-mode.md), [periods-and-windows](/query/periods-and-windows.md)). (yuuj) (4xwg)

#### Scenario: An explicit bound disables the cap entirely
- **GIVEN** `tu h --until 2026-01-31` with no `--since` and no `--full`
- **WHEN** `Normalize` runs
- **THEN** no floor is injected, `capActive` is false, and the explicit `--until` alone bounds the window (an intersection would silently empty the output)

### Requirement: The reserved user `all`
`all` is rejected as a configured profile name: after `config.Load` and the metrics-dir guard, `cmd/tu` (and the `tu sync` path) checks `cfg.User == "all"` and prints `Error: config user "all" is reserved (used by -u all)` to stderr, exiting with the usage code 2 — a bad config value is invocation-fixable ([entry-point](/command/entry-point.md), [metrics-dir-guard](/config/metrics-dir-guard.md)). (svlv)

### Requirement: The leaderboards require multi mode
`command.ErrLeaderboardMode` fires in `Run` when the display is lb/lbh and the post-guard `cfg.Mode == config.Single`, evaluated BEFORE `Normalize` — no notices, no fetch, no lines, exit 1, message `Error: lb requires multi mode — run tu init-metrics <repo-url> to set up a metrics repo` (it names `lb` even for `lbh`). A multi config demoted by the clone guard lands on the same message after the guard's own stderr lines ([metrics-dir-guard](/config/metrics-dir-guard.md)). (4xwg) (2gbb)

#### Scenario: A demoted multi config still fails fast
- **GIVEN** a multi config whose metrics dir is missing and whose clone failed (demoted to single)
- **WHEN** `tu lb` runs
- **THEN** the guard's stderr lines print first, then `Error: lb requires multi mode — …`, exit 1

## Design Decisions

### Flag guards are a pure Normalize step whose notices ride the Result
**Decision**: `Normalize` clears/defaults flags and returns notice lines; `cmd/tu` prints them before the source warnings.
**Why**: nothing below `cmd/tu` may write; the guards print before any fetch, so ordering notices ahead of the fetch warnings reproduces the stderr byte order; a guarded snapshot (e.g. one carrying `--since`) stays in scope after the clear and renders with exit 0.
**Rejected**: printing from `command` (breaks the only-writer rule); treating guarded flags as out of scope (turns a warn-and-render exit 0 into the placeholder's exit 1).
*Introduced by*: 260916-9ax5-history-and-periods

### Implicit 3-month cap as a defaulted Since at the guard seam
**Decision**: the default 3-month window on daily/weekly history defaults `Since = ThreeMonthFloor(now)` inside `Normalize` — the defaulted floor rides the same `query.Window` machinery as an explicit `--since`.
**Why**: one code path, zero new filtering logic; the fetch cache, the multi-mode merge, every output format, and watch mode inherit the cap uniformly through the single defaulted flag. A month-aligned floor makes the window a clean N-calendar-month figure; monthly is exempt because one compact row per month is already the long-term view; an explicit bound disables the cap entirely because intersecting a past `--until` with the floor would silently empty the output.
**Rejected**: passing `--since` through to the fetch layer (cache bypass plus a multi-mode blind spot — repo records never pass through the fetch).
*Introduced by*: 260717-yuuj

### The reserved `all` user is a usage error
**Decision**: a config whose user is `all` exits 2 on every data command and on `tu sync`, right after the config load and the metrics-dir guard.
**Why**: `all` is the `-u all` aggregate token; a profile with that name would be unaddressable and would corrupt every multi-mode view. The bad config value is invocation-fixable, so the usage code (2) applies.
**Rejected**: warn-and-continue (a misconfigured user silently corrupts every multi-mode view).
*Introduced by*: svlv

### Leaderboards fail fast in single mode
**Decision**: lb/lbh in single mode exit 1 with the `ErrLeaderboardMode` line before any fetch; the `-u`-in-single-mode guard exempts the leaderboards so this failure fires first, with no notice line.
**Why**: the leaderboard is an all-users view of the metrics repo; single mode has no repo to rank. Exit 1 (operational) rather than 2 because the environment or config must be fixed, not the command line; a demoted multi config lands on the same actionable message.
**Rejected**: warn-and-empty (hides a misconfigured setup behind a plausible-looking empty table).
*Introduced by*: 260828-4xwg-leaderboard-lb-lbh-display
