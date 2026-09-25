---
type: memory
description: MetricsDirGuard in internal/config/guard.go — the multi-mode auto-clone fallback behind the Cloner interface, the .clone-failed cooldown marker (3 h retry window, RFC 3339 content), Node-shaped clone-failure detail, and mode demotion with ordered stderr lines
---

# Metrics-Dir Guard

**Domain**: config

## Overview

`internal/config/guard.go` implements the multi-mode auto-clone guard: when a `Multi` config's metrics dir is missing, `MetricsDirGuard` attempts a quiet clone and demotes the config to `Single` — with a warning — when the clone fails or a recent failure is on cooldown. The edge places the guard between config load and the data path (see [command/entry-point](/command/entry-point.md)).

## Requirements

### Requirement: Guard placement and shape
`MetricsDirGuard(cfg Config, stateDir string, now time.Time, git Cloner) (Config, []string)` in `internal/config/guard.go` returns the config the data path runs with — `Mode` possibly demoted to `Single` — plus the stderr lines the edge prints, in order. It prints nothing itself and is pure apart from the marker file and the clone it delegates to `git`. It fires only when `cfg.Mode == Multi` and `cfg.MetricsDir` does not exist — an existence check via `os.Stat`, not an is-repo check, because the harness seed directory has no `.git/`.

#### Scenario: Metrics dir present
- **GIVEN** a `Multi` config whose `MetricsDir` exists
- **WHEN** `MetricsDirGuard` runs
- **THEN** it returns the config unchanged with no lines and never touches `git`

### Requirement: The .clone-failed cooldown
`cloneMarkerFresh(stateDir, now)` in `internal/config/guard.go` reports fresh when `stateDir/.clone-failed` (`CloneFailedMarker` in `internal/config/setup.go`) exists, its trimmed content parses as an RFC 3339 timestamp, and the marker is younger than `cloneRetryWindow` — 3 hours (`3 * time.Hour`). Missing, unreadable, or unparseable content counts as stale; a future timestamp counts as fresh. When the marker is fresh, the guard MUST skip the clone, demote to `Single`, and emit exactly `Warning: metrics repo not available — falling back to single mode.` On a failed clone, `writeCloneMarker` creates the state dir if needed (`0o755`) and writes the marker (`0o644`) with content `now.UTC().Format("2006-01-02T15:04:05.000Z")` — the JavaScript `toISOString()` shape.

#### Scenario: Fresh marker skips the clone
- **GIVEN** a `Multi` config with a missing `MetricsDir` and a marker written 1 hour ago
- **WHEN** `MetricsDirGuard` runs
- **THEN** the `Cloner` is never called, the returned `Mode` is `Single`, and the only line is the not-available warning

#### Scenario: Garbage marker counts as stale
- **GIVEN** a `.clone-failed` file whose content does not parse as RFC 3339
- **WHEN** `MetricsDirGuard` runs
- **THEN** the marker is treated as stale and a clone is attempted

### Requirement: The Cloner interface and the quiet clone
`Cloner` in `internal/config/guard.go` is `interface{ CloneQuiet(ctx, url, dir) (stderr string, err error) }` — the only git question the guard asks, satisfied by the sync driver's `Exec` (see [sync/git-flow](/sync/git-flow.md)). `CloneQuiet` runs `git clone <url> <dir>` with stdout/stderr captured (never inherited), `GIT_TERMINAL_PROMPT=0` in the child environment, and a 30 s deadline. On success (`err == nil`) the guard removes the marker via `RemoveCloneMarker`, keeps `Mode` as `Multi` — even when the clone left no directory behind, in which case the readers simply find nothing — and emits `Cloned metrics repo → {cfg.MetricsDir}` with the dir absolute.

#### Scenario: Clone succeeds, then marker cooldown lifts
- **GIVEN** a `Multi` config with a missing `MetricsDir`, a stale or absent marker, and a succeeding `Cloner`
- **WHEN** `MetricsDirGuard` runs
- **THEN** the `Cloner` is called once with `(cfg.MetricsRepo, cfg.MetricsDir)`, the config stays `Multi`, the marker is removed, and the lines are exactly `[Cloned metrics repo → <dir>]`

### Requirement: Clone failure detail and demotion
On clone failure the guard writes the marker, demotes to `Single`, and emits `Warning: could not clone metrics repo ({detail}) — falling back to single mode.`, where `cloneFailureDetail` in `internal/config/guard.go` composes `{detail}` to reproduce Node's `execFileSync` message: `Command failed: git clone {url} {dir}` plus `\n` + the captured stderr when that is non-empty; `spawnSync git ETIMEDOUT` when the error wraps `context.DeadlineExceeded` (the 30 s deadline); `spawnSync git ENOENT` when git never started (`exec.ErrNotFound` / `fs.ErrNotExist`); `spawnSync git EACCES` on `fs.ErrPermission`.

#### Scenario: Clone deadline fires
- **GIVEN** a `Cloner` whose error wraps `context.DeadlineExceeded`
- **WHEN** the guard handles the failure
- **THEN** the warning's detail is `spawnSync git ETIMEDOUT`

#### Scenario: Non-zero exit with stderr
- **GIVEN** a `Cloner` returning a non-zero exit with stderr text `fatal: repository not found`
- **WHEN** the guard handles the failure
- **THEN** the detail is `Command failed: git clone {url} {dir}\nfatal: repository not found`, the marker is written, and the config demotes to `Single`

## Design Decisions

### The guard in config, git behind Cloner, the edge prints
**Decision**: `MetricsDirGuard` lives in `internal/config` — which already owns `CloneFailedMarker`, `RemoveCloneMarker`, `StateDir`, and `Load` — sees git only through the `Cloner` interface, and returns its stderr lines for `cmd/tu` to print.
**Why**: The only-writer rule; a fake `Cloner` makes every branch (marker fresh/stale/garbage, clone ok/fail/timeout) unit-testable without git or a network.
**Rejected**: Running the clone inline in `cmd/tu` (untestable, bloats the edge); putting the guard in the sync package (the guard decides config mode, not sync behavior).
*Introduced by*: 260916-xivf-metrics-source-and-multi-mode (xivf)

### Existence check, not is-repo check
**Decision**: The guard tests `os.Stat(MetricsDir)`, not whether the dir is a git repo.
**Why**: The harness seeds the metrics dir as a plain directory with no `.git/`; an is-repo check would attempt a clone on top of a valid seed.
**Rejected**: `git -C <dir> rev-parse --git-dir` — an extra exec on every multi-mode invocation and a harness mismatch.
*Introduced by*: 260916-xivf-metrics-source-and-multi-mode (xivf)

### Node-shaped clone-failure detail
**Decision**: `cloneFailureDetail` reproduces `execFileSync` error text — `Command failed: git clone …` with captured stderr, or `spawnSync git {ETIMEDOUT|ENOENT|EACCES}` — instead of Go's native `*exec.ExitError` strings.
**Why**: Parity with the frozen golden corpus (`/harness/golden-corpus.md`, the retired TypeScript implementation's bytes): the differential harness pins the full warning line byte-for-byte, including the deadline and process-start shapes.
**Rejected**: Go-native error text (`exit status 128`, `signal: killed`) — readable but a different wire shape.
*Introduced by*: 260916-xivf-metrics-source-and-multi-mode (xivf)

### Cooldown marker instead of repeated clone attempts
**Decision**: A failed clone writes `stateDir/.clone-failed` with the current UTC timestamp; the guard skips further clones for 3 hours and degrades to single mode with a warning.
**Why**: An offline or unauthorized repo would otherwise stall every invocation for the full 30 s clone deadline; the marker bounds the retry cost to once per window while keeping the failure visible on stderr.
**Rejected**: No marker (a 30 s stall on every command while the repo is unreachable); an infinite blacklist (the repo never recovers without manual intervention).
*Introduced by*: 260916-xivf-metrics-source-and-multi-mode (xivf)
