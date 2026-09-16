---
type: memory
description: The Go port's config layer and setup commands — internal/config (path resolution, StateDir/Tildefy/ExpandHome, the embedded tu.default.conf with a drift guard, Load's five-layer cascade and its legacy/version warnings, InitConf with FieldBlocks/ensureUserConf, InitMetrics with the three-branch metrics_repo write and the CloneStep handoff, Status/Lines/RelativeTime, CloneFailedMarker/RemoveCloneMarker), internal/sync's Exec git driver, and cmd/tu's setup-command dispatch — 95 harness cases green
---
# Config and Setup Commands (Go port)

**Domain**: go-port

## Overview

`internal/config` owns the full config cascade — `tu.conf`/`org.conf`/legacy selection, the embedded shipped defaults, `Load`, and the setup commands `init-conf`, `init-metrics`, and `status` — reproducing the frozen TypeScript behavior documented in [config-system](/configuration/config-system.md) byte for byte. `internal/sync` holds the git driver the setup flow needs; `cmd/tu` dispatches the setup commands and executes the clone at the edge (the full write order lives in [command-edge](/go-port/command-edge.md)). Nothing below `cmd/tu` prints or exits; `config` execs nothing.

## Requirements

### Requirement: Path resolution and path helpers
`ResolvePaths(home)` returns `Paths{Home, ConfigDir, UserConf, OrgConf, LegacyConf}` — `$HOME/.config/tu`, `$HOME/.config/tu/tu.conf`, `$HOME/.config/tu/org.conf`, `$HOME/.tu.conf` — built from `$HOME` and nothing else (no XDG, no `TU_*` path override); an empty home returns `ErrNoHome` whose message is byte-exact: `tu: $HOME is not set; cannot locate config`. `StateDir(home)` is the runtime-state root `$HOME/.tu` (cache, metrics-repo clone, `.last-sync`, `.clone-failed`) — intentionally separate from the config root. `Tildefy(p, home)` abbreviates a path under home to `~/…` (a prefix match on the home string, no trailing-slash handling); other paths pass through. `ExpandHome(p, home)` resolves a leading `~/` or a bare `~` against home. `ParseConf(raw)` trims lines, skips blanks and `#` comments, splits at the FIRST `=`, trims key and value; later keys overwrite earlier ones.

### Requirement: Embedded shipped defaults with a drift guard
`defaults.go` carries `//go:embed tu.default.conf` into `DefaultConf []byte`; the sibling `src/go/internal/config/tu.default.conf` is a byte-identical copy of the repo-root `tu.default.conf` (the file `scripts/build.sh` copies beside `tu.mjs`), and `defaults_test.go` walks up from the package directory to the directory containing `package.json` and asserts the two files are byte-equal, so the copy cannot drift silently. `DefaultConfName = "tu.default.conf"` is the string the newer-version warning prints when the out-of-range `version` came from the defaults layer. `Load` uses `DefaultConf` as its defaults layer; `InitConf` writes `DefaultConf` when scaffolding from defaults.

### Requirement: Load — the five-layer cascade
`Load(p Paths, env Env, ov Overrides) (Config, []string)` (the TS `readConfig`) reads files and returns values — the `Config`, the stderr warning lines in emission order, and never an error: an unreadable file is an empty layer. The cascade merges `defaults ∪ org.conf ∪ user conf` (later wins per key, no per-key inversion), then overrides `metrics_repo` alone with `TU_METRICS_REPO` when non-empty, then with `ov.MetricsRepo` when non-nil — the CLI argument beats the env var even when the argument is the empty string (the TS nullish test). `TU_METRICS_REPO` is the only config-bearing environment variable. `Mode` is `Multi` iff the final `MetricsRepo != ""`; a `mode` key in any file is ignored. `Env` injects `Getenv`/`Hostname`/`Username` so tests pin the process view; `Overrides{MetricsRepo *string}` is the CLI-argument layer.

#### Scenario: Layer precedence
- **GIVEN** `org.conf` with `metrics_repo = A`, `tu.conf` with `metrics_repo = B`, env `TU_METRICS_REPO=C`, and an override pointing at `D`
- **WHEN** `Load` runs
- **THEN** `MetricsRepo == "D"` and `Mode == Multi`; without the override it is `C`; without the env it is `B`; without `tu.conf` it is `A`

### Requirement: User-conf selection and the deprecation warning
`UserConf` readable → its fields, `UserConfRead = UserConf`, and the legacy file silently ignored; else `LegacyConf` readable → its fields, `UserConfRead = LegacyConf`, and the warning `tu: ~/.tu.conf is deprecated; move it to ~/.config/tu/tu.conf` (literal text, not built from the path); else neither file feeds the merge and `UserConfRead == ""`. "Readable" means `os.ReadFile` succeeds — an existing-but-unreadable file falls back. `OrgRead` is true iff `org.conf` was readable. Warnings return in emission order: deprecation first, the version warning second. `selectUserConfPath` reports the path the selection rule would read, for `status` (an exists-based check would misreport an existing-but-unreadable file as selected while `Load` falls back).

### Requirement: Version, fields, sentinels, and expansion
`Version` is parsed from the merged `version` with JavaScript `parseInt(s, 10)` semantics — optional leading whitespace and sign, then the longest run of ASCII digits (`"2abc"` → 2); no digits or a missing key → 1. When `Version > CurrentConfigVersion` (2), the warning is `Warning: {source} version {N} is newer than tu supports (2). Please update tu.` where `{source}` is `UserConfRead` when non-empty, else `p.OrgConf` when `OrgRead`, else `DefaultConfName` — an absolute path, never tildefied. `MetricsDir` is `ExpandHome(expandSentinels(merged["metrics_dir"] or "~/.tu/metrics_repo"), Home)`; `Machine` and `User` come from `machine`/`$HOSTNAME` and `user`/`$USER` through the same sentinel expansion. `expandSentinels` replaces a value that is exactly `$HOSTNAME` with the hostname (error → `""`) and exactly `$USER` with the username (error → `unknown`); no substring expansion. `AutoSync` is false only when `auto_sync` is exactly `false` or `0`.

### Requirement: InitConf scaffolds and repairs the user conf
`InitConf(p Paths) ([]string, error)` is `tu init-conf`; a filesystem failure is `*Error` carrying the OS error text (exit 1 at the edge). The shared `ensureUserConf` runs first: `UserConf` exists → no-op; else `MkdirAll(ConfigDir, 0o755)` and write `UserConf` (0o644) from the legacy file's bytes when that exists (emitting `Copied ~/.tu.conf → ~/.config/tu/tu.conf`, both paths tildefied; the legacy file is never moved or deleted), else from `DefaultConf` (emitting `Created ~/.config/tu/tu.conf — edit it to configure multi-machine sync.`), and stop with that one line. Otherwise the conf is classified per `FieldBlocks` key — the six byte-exact scaffold blocks, in the order `version, metrics_repo, metrics_dir, machine, user, auto_sync`: **present** when some whitespace-trimmed line is not a comment and matches `^{key}\s*=`; else **mentioned** when the content matches a possibly-commented `(?m)^\s*#?\s*{key}\s*=`; else **missing**. All present → `{dp} is already complete.`; missing keys → their blocks are appended in `FieldBlocks` order with the line `Updated {dp} — added missing fields: {keys}.`; commented keys → `{dp} has commented-out fields that need uncommenting: {keys}.` (after the Updated line when both apply), where `dp = Tildefy(UserConf, Home)`.

#### Scenario: Missing-field repair
- **GIVEN** `tu.conf` containing only `version = 2`
- **WHEN** `InitConf` runs
- **THEN** the five other blocks are appended in order and the line is `Updated ~/.config/tu/tu.conf — added missing fields: metrics_repo, metrics_dir, machine, user, auto_sync.`

### Requirement: InitMetrics up to the clone step
`InitMetrics(p Paths, env Env, url *string, git Git) (InitMetricsResult, error)` performs everything up to (not including) the clone; the arity check happens in `command.Parse` before `$HOME` is consulted. With `url != nil`: a URL containing `\r` or `\n` is `*Error{"Error: repo-url must be a single line (no newline or carriage-return characters)."}` before any file is written; then `ensureUserConf` (its Created/Copied line is the first stdout line when it fires); then `setMetricsRepoInConf` — an active `metrics_repo` line is replaced in place with `metrics_repo = {url}`, else a commented `#\s*metrics_repo\s*=` line is replaced the same way, else the `metrics_repo` FieldBlocks block is appended with its sample line replaced — and the line `Set metrics_repo = {url} in {Tildefy(UserConf)}` is emitted, with the URL also set as the `Overrides` layer. Then `Load` runs and its warnings ride home in `InitMetricsResult.Warnings` for the edge to print. `MetricsRepo == ""` → `*Error{"Error: metrics_repo is not set. Add it to {dp}, run 'tu init-metrics <repo-url>', or set TU_METRICS_REPO."}`. `MetricsDir` exists: `git.IsRepo(dir)` true → `Already initialized: {dir}` (absolute) with `Clone == nil`; false → `*Error{"Error: {dir} exists but is not a git repo. Remove it or set a different metrics_dir in {dp}."}`. Otherwise the result carries `Clone = &CloneStep{URL: MetricsRepo, Dir: MetricsDir}`. `Git` is `interface{ IsRepo(dir string) bool }` — the one git question `config` asks. `ClonedLine(url, dir)` returns `Cloned {url} → {dir}` with the dir absolute (DC-19). `CloneFailedMarker` is `.clone-failed` under `StateDir`; `RemoveCloneMarker(stateDir)` deletes it when present, best-effort and never failing (the marker's write/read is B3's auto-clone guard).

#### Scenario: init-metrics with a URL on an empty home
- **GIVEN** an empty `$HOME`, `url = git@example.invalid:harness/tu-metrics.git`, and a `Git` whose `IsRepo` is never called
- **WHEN** `InitMetrics` runs
- **THEN** `tu.conf` exists with the commented sample line replaced by `metrics_repo = git@example.invalid:harness/tu-metrics.git`, the lines are `Created …` then `Set metrics_repo = git@example.invalid:harness/tu-metrics.git in ~/.config/tu/tu.conf`, and `Clone` is `{URL: the url, Dir: $HOME/.tu/metrics_repo}`

### Requirement: Status data and layout
`Status(p Paths, env Env, now time.Time) (StatusData, []string)` reads config (via `Load`) and state and returns the data plus the cascade warnings the edge prints; it never clones, syncs, or writes. When neither a readable user/legacy conf nor `org.conf` exists the result is the one-line form `Mode:        single (no ~/.config/tu/tu.conf)` — no `Load`, no warnings; an org-only setup falls through to the full layout with the `Config:` line omitted. `Lines()` renders the layouts §13 block with the literal 13-column label strings (not `%-13s` — the em dash and the apostrophe in the NOT FOUND suffix are copied verbatim). Single mode: `Mode:        single`, the optional `Config:      {Tildefy(selected)} (v{Version})`, the optional `Org config:  {Tildefy(OrgConf)}`. Multi mode: `Mode:        multi`, `User:        {User}`, `Machine:     {Machine}`, the optional Config and Org lines, `Metrics:     {Tildefy(MetricsDir)}` (suffixed ` (NOT FOUND — run 'tu init-metrics')` when the dir does not exist), `Last sync:   {LastSync}`, `Auto-sync:   on|off`. `LastSync` is `never` when `StateDir/.last-sync` is absent, unreadable, or does not parse as RFC3339Nano after trimming (the file is written by `tu sync` as JavaScript `toISOString()`), else `{RelativeTime(now − ts)} ({trimmed raw})`. `RelativeTime(d)` floors to whole seconds (negatives clamp to 0): under 60 s → `<1m ago`; under 60 min → `{m}m ago`; under 24 h → `{h}h ago`; else `{d}d ago`.

#### Scenario: Last-sync rendering
- **GIVEN** `.last-sync` containing `2026-09-15T18:54:44.502Z` and `now` fifteen minutes later
- **WHEN** `Status` runs
- **THEN** `LastSync == "15m ago (2026-09-15T18:54:44.502Z)"`

### Requirement: sync.Exec drives the real git
`internal/sync/git.go` holds `type Exec struct{}`, which satisfies `config.Git`: `IsRepo(dir)` runs `git -C <dir> rev-parse --git-dir` with stdout/stderr discarded and reports exit 0; `Clone(ctx, url, dir, stdout, stderr)` runs `git clone <url> <dir>` with the writers attached to the child's stdout and stderr (the TS `stdio: "inherit"`), returning the exec error (`*exec.ExitError`) on a non-zero exit. `git` resolves through `PATH` — how both sides reach the fake git in the harness, so the recorded argv shapes match the TS after home normalization. No timeout and no `GIT_TERMINAL_PROMPT` — those belong to B3's auto-clone guard, not the interactive init-metrics clone.

### Requirement: The edge executes the clone
For `init-metrics`, `cmd/tu` prints `res.Warnings` on stderr and `res.Lines` on stdout (on error: Lines first, then the message on stderr, exit 1); when `res.Clone != nil` it runs `sync.Exec{}.Clone` with its own stdout/stderr passed through — git's `Cloning into '…'…` chatter reaches the user's stderr — and on success calls `RemoveCloneMarker(StateDir(paths.Home))` then prints `ClonedLine`. A non-zero clone exit prints `Error: git clone failed (exit {N}).` on stderr, exit 1; the TypeScript side prints Node's uncaught-exception stack trace on this path — inherently unmatchable and recorded as an expected diff, never a harness case (the fake git always exits 0).

### Requirement: Harness gate status
`just go-diff --placeholder` reports 95 of 368 cases green; the newest 22 are the 20 setup-command cases — `status`, `init-conf`, `init-metrics`, `init-metrics-url`, `init-metrics-extra` × the four conf variants — and the two `--dry-run` misuse cases (`dry-run/single/default/pipe/fixed`, `cc-dry-run/single/default/pipe/fixed`). The `[calls differ]` marker is absent from the `init-metrics` groups: both sides issue the same git argv after home normalization. See [differential-harness](/harness/differential-harness.md).

## Design Decisions

### Embedded shipped defaults
**Decision**: `tu.default.conf` is embedded into the Go binary from a byte-identical copy under `internal/config`, drift-guarded by a test.
**Why**: No runtime file lookup, no silent empty-defaults failure mode when the file is missing, and tests need no path injection; the install layout is not a Goal-listed surface.
**Rejected**: Reproducing the TS beside-the-binary then walk-up lookup — a runtime dependency on a file the Go tarball would otherwise not need, and a missing file degrades `init-conf` to an empty scaffold.
*Introduced by*: 260916-4fs0-config-and-setup-commands

### Setup commands return values; the edge executes the clone
**Decision**: `InitConf`, `InitMetrics`, `Status` live in `internal/config` and return lines plus a typed `config.Error`; `InitMetrics` returns a `CloneStep` that `cmd/tu` runs through `sync.Exec`, then prints `ClonedLine`.
**Why**: Keeps "nothing prints below `cmd/tu`" while preserving the TS ordering of the `Set metrics_repo` line before git's clone chatter; `config` execs nothing.
**Rejected**: Passing `io.Writer`s into `config` (makes `config` a writer); running the clone inside `config` via an injected interface that takes writers (same problem, one level down).
*Introduced by*: 260916-4fs0-config-and-setup-commands

### Git driver lands in sync
**Decision**: `internal/sync/git.go` holds the real git driver (`Exec`) that satisfies `config.Git`; the metrics-repo writer, never-shrink guard, dry-run report, and sync flow grow the same package in B6.
**Why**: The port plan's Target architecture assigns the git driver to `sync`; `config` stays exec-free.
**Rejected**: `Exec` in `config` (blurs the package boundary the G1 gate checks); exec in `cmd/tu` (main.go stops being thin and B6 would need the driver anyway).
*Introduced by*: 260916-4fs0-config-and-setup-commands
