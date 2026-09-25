---
type: memory
description: tu status in internal/config/status.go — Status/StatusData read config and state and return data plus warnings, Lines renders the layouts §13 block with literal 13-column labels, LastSync formats .last-sync as relative + ISO (shared with the lb footer), RelativeTime buckets
---

# Status Command

**Domain**: config

## Overview

`internal/config/status.go` implements `tu status`: `Status` reads the config cascade and runtime state and returns `StatusData` plus the cascade warnings; `Lines` renders the fixed-column layout block. `Status` never clones, syncs, or writes — [command/entry-point](/command/entry-point.md) prints the returned lines and warnings.

## Requirements

### Requirement: Status reads config and state, returns data
`Status(p Paths, env Env, now time.Time) (StatusData, []string)` in `internal/config/status.go` runs the [Load cascade](/config/cascade.md) and reads state; it returns `StatusData` plus the cascade warnings the edge prints on stderr. `Status` MUST NOT clone, sync, or write. The user-conf line uses `selectUserConfPath` — the same readability-based selection `Load` uses, so an existing-but-unreadable user conf falls back identically. When neither a readable user/legacy conf nor `org.conf` exists, `Status` returns `StatusData{Mode: Single, NoConfig: true}` with no `Load` and no warnings. An org-only setup falls through to the full layout with the `Config:` line omitted.

#### Scenario: No config at all
- **GIVEN** no readable user or legacy conf and no `org.conf`
- **WHEN** `Status` runs
- **THEN** `StatusData.NoConfig` is true, `Mode` is `Single`, and no `Load` warnings are produced

### Requirement: Lines renders the layout block
`StatusData.Lines()` in `internal/config/status.go` MUST render with literal 13-character label columns as literal strings — not `%-13s` — so the em dash and the apostrophe in the NOT FOUND suffix are byte-exact. The no-config form is the single line `Mode:        single (no ~/.config/tu/tu.conf)`. Single mode renders `Mode:        single`, then the optional `Config:      {Tildefy(UserConfRead)} (v{Version})` (omitted when no user conf was read), then the optional `Org config:  {Tildefy(OrgConf)}` (only when `org.conf` exists). Multi mode renders `Mode:        multi`, `User:        {User}`, `Machine:     {Machine}`, the optional Config and Org lines, `Metrics:     {Tildefy(MetricsDir)}` suffixed ` (NOT FOUND — run 'tu init-metrics')` when the metrics dir does not exist, `Last sync:   {LastSync}`, and `Auto-sync:   on|off`. An org-only setup omits the `Config:` line but still prints `Org config:  …`.

#### Scenario: No config
- **GIVEN** `StatusData.NoConfig` is true
- **WHEN** `Lines()` renders
- **THEN** the output is exactly `Mode:        single (no ~/.config/tu/tu.conf)`

#### Scenario: Missing metrics dir in multi mode
- **GIVEN** multi mode with a metrics dir that does not exist
- **WHEN** `Lines()` renders
- **THEN** the Metrics line reads `Metrics:     {tildefied dir} (NOT FOUND — run 'tu init-metrics')`

### Requirement: LastSync formats the sync timestamp
`LastSync(stateDir, now)` in `internal/config/status.go` reads `stateDir/.last-sync` (the state dir — see [source/cache](/source/cache.md); the file is written by the sync flow, [sync/git-flow](/sync/git-flow.md)). Absent, unreadable, or unparseable content (after trimming, parsed as `time.RFC3339Nano` — the JavaScript `toISOString()` shape) yields `never`; otherwise the result is `{RelativeTime(now − ts)} ({trimmed raw})`. `LastSync` is exported and shared: `Status` calls it for the `Last sync:` line and the leaderboard's staleness footer reaches the same text through `Deps.LastSync` (see [view/leaderboard](/view/leaderboard.md)).

#### Scenario: Last-sync rendering
- **GIVEN** `.last-sync` containing `2026-09-15T18:54:44.502Z` and `now` fifteen minutes later
- **WHEN** `LastSync` runs
- **THEN** the result is `15m ago (2026-09-15T18:54:44.502Z)`

#### Scenario: Unparseable timestamp
- **GIVEN** `.last-sync` containing `not-a-date`
- **WHEN** `LastSync` runs
- **THEN** the result is `never`

### Requirement: RelativeTime buckets
`RelativeTime(d time.Duration)` in `internal/config/status.go` floors to whole seconds with negatives clamped to 0, then buckets: under 60 s → `<1m ago`; under 60 min → `{m}m ago`; under 24 h → `{h}h ago`; otherwise `{d}d ago`.

#### Scenario: Boundary values
- **GIVEN** durations of 59 s, 60 s, 60 min, and 25 h
- **WHEN** `RelativeTime` formats them
- **THEN** they render `<1m ago`, `1m ago`, `1h ago`, and `1d ago` respectively

## Design Decisions

### Status mirrors the Load selection rule
**Decision**: `Status` reports the user conf through `selectUserConfPath` — the same readability-based test `Load` applies — rather than an existence check.
**Why**: An existing-but-unreadable user conf falls back to the legacy file in `Load`; an exists-based status line would misreport which file fed the merge.
**Rejected**: An `os.Stat` existence check — reports a file `Load` actually skipped.
*Introduced by*: 260916-4fs0-config-and-setup-commands (4fs0)

### Literal 13-column labels over printf widths
**Decision**: `Lines` renders the layout block with the label columns as literal strings (`Mode:        `, `Metrics:     `, …), not `fmt` width verbs.
**Why**: Parity with the frozen golden corpus (`/harness/golden-corpus.md`, the retired TypeScript implementation's bytes): the em dash in `NOT FOUND —` and the apostrophe in `run 'tu init-metrics'` survive byte-exact when they are copied verbatim instead of reconstructed by a format verb.
**Rejected**: `fmt.Sprintf("%-13s", label)` — column widths drift from the pinned bytes and invite reflow of the multibyte suffixes.
*Introduced by*: 260916-4fs0-config-and-setup-commands (4fs0)

### .last-sync reaches command as a closure
**Decision**: `command.Deps.LastSync` is a `func() string` built at the edge over `config.LastSync(StateDir(home), time.Now())`, called only on the `lb` path; a nil `LastSync` in tests reads as `"never"`.
**Why**: The command layer stays I/O-free; the closure mirrors `Deps.Now`; no other command pays the file read; `Status` and the leaderboard's staleness footer share one formatting implementation.
**Rejected**: Passing `StateDir` into the command layer and reading the file there — I/O below the edge.
*Introduced by*: 260916-2gbb-leaderboard-lb-lbh (2gbb)
