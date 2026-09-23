---
type: memory
description: Setup commands in internal/config/setup.go — InitConf scaffolding/repair via FieldBlocks and ensureUserConf legacy seeding, InitMetrics URL validation, setMetricsRepoInConf editing, the CloneStep handoff the edge executes, ClonedLine, and the .clone-failed marker constant
---

# Setup Commands

**Domain**: config

## Overview

`internal/config/setup.go` implements the data side of `tu init-conf` and `tu init-metrics`: scaffolding and repairing the user conf, writing `metrics_repo`, and deciding whether a metrics-repo clone is needed. The commands return lines and a `CloneStep`; [command/entry-point](/command/entry-point.md) prints the lines and executes the clone.

## Requirements

### Requirement: Operational errors as values
A setup command's operational failure MUST be `*config.Error` (`type Error struct{ Message string }` in `internal/config/setup.go`) whose message the edge prints on stderr verbatim with exit 1. Usage errors (exit 2) never originate in this package — they come from `command.Parse`. The package writes nothing to any stream.

### Requirement: FieldBlocks — the six scaffold blocks
`FieldBlocks` in `internal/config/setup.go` holds the six scaffold blocks in order — `version, metrics_repo, metrics_dir, machine, user, auto_sync` — each a byte-exact `\n`-prefixed comment block plus assignment (e.g. `version` → `\n# Config schema version\nversion = 2\n`). The scaffold contains no `mode` field.

### Requirement: ensureUserConf seeds the user conf
`ensureUserConf(p Paths)` in `internal/config/setup.go` is shared by `InitConf` and `InitMetrics`. When `UserConf` exists it is a no-op. Otherwise it MUST `MkdirAll(ConfigDir, 0o755)` and write `UserConf` with mode `0o644`, seeded from the legacy file's bytes when `~/.tu.conf` exists — emitting `Copied ~/.tu.conf → ~/.config/tu/tu.conf` with both paths tildefied; the legacy file is never moved or deleted — else from `DefaultConf` (the embedded [defaults](/config/cascade.md)) emitting `Created ~/.config/tu/tu.conf — edit it to configure multi-machine sync.` A filesystem failure returns `*Error` carrying the OS error text.

### Requirement: InitConf scaffolds and repairs
`InitConf(p Paths) ([]string, error)` in `internal/config/setup.go` runs `ensureUserConf` first and, when it created the file, stops with that one stdout line. Otherwise each `FieldBlocks` key is classified: **present** when some line, after trimming leading whitespace, is not a comment and matches `^{key}\s*=` (`fieldPresent`); else **mentioned** when the content matches a possibly-commented `(?m)^\s*#?\s*{key}\s*=` line (`fieldMentioned`); else missing. All present → `{dp} is already complete.` Missing keys → their `FieldBlocks` blocks are appended in `FieldBlocks` order with the line `Updated {dp} — added missing fields: {keys}.` Commented keys → `{dp} has commented-out fields that need uncommenting: {keys}.` printed after the Updated line when both apply. `dp` is `Tildefy(UserConf, Home)`. Filesystem failures return `*Error` with the OS error text.

#### Scenario: Missing-field repair
- **GIVEN** a `tu.conf` containing only `version = 2`
- **WHEN** `InitConf` runs
- **THEN** the five other blocks are appended in `FieldBlocks` order and the line is `Updated ~/.config/tu/tu.conf — added missing fields: metrics_repo, metrics_dir, machine, user, auto_sync.`

#### Scenario: Complete conf
- **GIVEN** a `tu.conf` where every `FieldBlocks` key has an active assignment
- **WHEN** `InitConf` runs
- **THEN** the single line is `~/.config/tu/tu.conf is already complete.` and the file is untouched

### Requirement: InitMetrics up to the clone step
`InitMetrics(p Paths, env Env, url *string, git Git) (InitMetricsResult, error)` in `internal/config/setup.go` performs everything up to (not including) the clone. With a non-nil `url`: a URL containing `\r` or `\n` MUST fail with `*Error{"Error: repo-url must be a single line (no newline or carriage-return characters)."}` before any file is written; then `ensureUserConf` runs (its Created/Copied line is first when it fires); then `setMetricsRepoInConf` writes the URL — replacing an active `metrics_repo\s*=` assignment in place, else replacing a commented `#\s*metrics_repo\s*=` line, else appending the `metrics_repo` `FieldBlocks` block with the sample line swapped for the real URL — followed by the stdout line `Set metrics_repo = {url} in {Tildefy(UserConf)}`; the URL also becomes the `Overrides` layer so it beats `TU_METRICS_REPO` for this invocation. `Load` then runs and its warnings ride home in `InitMetricsResult.Warnings`. An empty resolved `MetricsRepo` fails with `Error: metrics_repo is not set. Add it to {dp}, run 'tu init-metrics <repo-url>', or set TU_METRICS_REPO.` When `MetricsDir` exists: `git.IsRepo(dir)` true → the line `Already initialized: {dir}` with `Clone == nil`; false → `*Error{"Error: {dir} exists but is not a git repo. Remove it or set a different metrics_dir in {dp}."}`. Otherwise the result carries `Clone = &CloneStep{URL, Dir}` and the edge executes the clone (see [command/entry-point](/command/entry-point.md)). `Git` is `interface{ IsRepo(dir string) bool }` — the only git question this package asks.

#### Scenario: Newline injection rejected
- **GIVEN** `url` containing a `\n`
- **WHEN** `InitMetrics` runs
- **THEN** it returns `*Error{"Error: repo-url must be a single line (no newline or carriage-return characters)."}` and no file is written

#### Scenario: init-metrics with a URL on an empty home
- **GIVEN** an empty home and `url = git@github.com:me/tu-metrics.git`
- **WHEN** `InitMetrics` runs
- **THEN** `tu.conf` is scaffolded with the commented sample line replaced by `metrics_repo = <url>`, the lines are `Created …` then `Set metrics_repo = <url> in ~/.config/tu/tu.conf`, and `Clone` is `{URL: <url>, Dir: ~/.tu/metrics_repo expanded}`

#### Scenario: Existing non-repo metrics dir
- **GIVEN** `MetricsDir` exists and `git.IsRepo(dir)` is false
- **WHEN** `InitMetrics` runs
- **THEN** it fails with `Error: {dir} exists but is not a git repo. Remove it or set a different metrics_dir in {dp}.`

### Requirement: The CloneStep handoff
`CloneStep{URL, Dir}` in `internal/config/setup.go` is what the edge runs as `git clone <URL> <Dir>` with stdout/stderr passed through; `InitMetricsResult.Clone == nil` means nothing is left to do. `ClonedLine(url, dir)` returns `Cloned {url} → {dir}` with `dir` absolute — the edge prints it after a successful clone. `CloneFailedMarker = ".clone-failed"` names the cooldown file under the state dir and `RemoveCloneMarker(stateDir)` deletes it best-effort, never erroring; the marker's write/read lives in [MetricsDirGuard](/config/metrics-dir-guard.md).

## Design Decisions

### Setup commands return values; the edge executes the clone
**Decision**: `InitConf`, `InitMetrics`, and `Status` live in `internal/config` and return lines plus a typed `*config.Error`; `InitMetrics` returns a `CloneStep` that `cmd/tu` runs through the git driver before printing `ClonedLine`.
**Why**: Keeps nothing printing below `cmd/tu` while preserving the ordering of the `Set metrics_repo` line before git's clone chatter; `config` execs nothing.
**Rejected**: Passing `io.Writer`s into `config` (makes `config` a writer); running the clone inside `config` via an injected interface (same problem, one level down).
*Introduced by*: 260916-4fs0-config-and-setup-commands (4fs0)

### Write-time legacy seeding
**Decision**: When tu creates `~/.config/tu/tu.conf` (via `init-conf` or `init-metrics <url>`) and a legacy `~/.tu.conf` exists, the new file is seeded from the legacy file's bytes, with the stdout `Copied …` note, rather than from the shipped defaults.
**Why**: Seeding avoids silently orphaning the user's `machine`/`user`/`metrics_repo` overrides the moment they run the one-liner; it is a write tu is already making, so it does not reintroduce the rejected read-time auto-migration.
**Rejected**: Always scaffolding from the shipped defaults — simpler, but loses existing overrides.
*Introduced by*: 260827-gzrn-config-home-conformance-org-layer (gzrn)

### Newline/CR rejection on the repo URL
**Decision**: `InitMetrics` rejects a `url` argument containing `\r` or `\n` before any file is written.
**Why**: The URL is written verbatim into `tu.conf`; a crafted argument containing a newline would inject extra config lines into the user's conf.
**Rejected**: Sanitizing/escaping the URL before writing — mutates a value that must reach `git clone` byte-for-byte.
*Introduced by*: 260916-4fs0-config-and-setup-commands (4fs0)
