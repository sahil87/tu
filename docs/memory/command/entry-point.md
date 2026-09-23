---
type: memory
description: cmd/tu/main.go — the only writer of the process streams; routing order (Parse → version → non-data dispatch → HOME → config.Load → MetricsDirGuard → reserved-user check → deps → --sync block → watch branch → Run), the setup commands, the sync and watch branches, and the exit-code mapping
---
# Entry Point

**Domain**: command

## Overview

`src/go/cmd/tu/main.go` is the process edge: `run(args, stdout, stderr) int` dispatches on the parsed [Request](/command/request-and-parse.md), wires the adapters into `command.Deps` ([run-and-result](/command/run-and-result.md)), and writes every byte of stdout/stderr — nothing below `cmd/tu` touches the process streams or calls `os.Exit`.

## Requirements

### Requirement: The routing order
`run` dispatches in a fixed order (`main.go`):

1. `command.Parse` — a usage error prints the message, then `ShortUsage` when `ShowUsage`, exit `ExitUsage` (2). Parse precedes the `$HOME` check, so `tu bogus` with `$HOME` unset still exits 2.
2. `Version` → the version line on stdout, exit 0 ([version-and-help-dump](/toolkit/version-and-help-dump.md)); `var version` is stamped via `-ldflags -X main.version=…` at build time ([toolchain](/build/toolchain.md)).
3. `Command != ""` → `runCommand` (below).
4. `config.ResolvePaths(os.Getenv("HOME"))` — an error prints its message on stderr, exit 1 ([cascade](/config/cascade.md)).
5. `config.Load(paths, env, Overrides{})` — the cascade warnings print on stderr before anything else.
6. `config.MetricsDirGuard(cfg, config.StateDir(paths.Home), time.Now(), metricsync.Exec{})` — the auto-clone guard runs HERE, between `Load` and the reserved-user check, on the data path only; its lines go to stderr in order ([metrics-dir-guard](/config/metrics-dir-guard.md)).
7. The reserved-user check — `cfg.User == "all"` → `Error: config user "all" is reserved (used by -u all)` on stderr, exit 2 ([guards](/command/guards.md)).
8. Deps wiring: `&ccusage.Source{Cache, User, Machine}`, `metrics.Source{Dir: cfg.MetricsDir}`, `metricsync.Writer{Dir: cfg.MetricsDir}` — the compile-time `command.Fetcher`/`Repo`/`Writer` assertions sit beside the assignment — plus `Now: time.Now`, `ansi.Colors{Enabled: !NoColor && os.Getenv("NO_COLOR") == ""}`, `Width: terminalWidth(stdout)`, and `LastSync` as a closure over `config.LastSync` evaluated only on the lb path.
9. The `--sync` block, then the `-w` watch branch, then one-shot `command.Run`.
10. Output: `Result.Notices` to stderr (before any fetch warning), then `source.WriteWarnings(stderr, res.Warnings)`, then the lines to stdout, exit 0. `ErrUnported` → the placeholder line on stderr, exit 1, empty stdout; any other error → its text on stderr, exit 1 ([errors-and-warnings](/source/errors-and-warnings.md)).

(4fs0) (xivf) (lsml) (2gbb)

#### Scenario: Usage errors outrank a missing HOME
- **GIVEN** `tu bogus` with `$HOME` unset
- **WHEN** `run` executes
- **THEN** the unknown-argument message and `ShortUsage` print on stderr, exit 2 — the `$HOME` error never surfaces

### Requirement: Non-data command dispatch
`runCommand` answers the toolkit commands — `help`/`-h`/`--help`, `help-dump`, `skill`, `shell-init`, `update` — for real BEFORE path resolution: none of them needs `$HOME` ([version-and-help-dump](/toolkit/version-and-help-dump.md), [skill-bundle](/toolkit/skill-bundle.md), [shell-init-and-completions](/toolkit/shell-init-and-completions.md), [update](/toolkit/update.md)). `init-conf`, `init-metrics`, `status`, and `sync` resolve paths first ([setup-commands](/config/setup-commands.md), [status](/config/status.md)). Data flags on a non-data command are ignored — `Parse` sets `Command` regardless of flags and the handlers never look at `Format`/`Flags` (except `sync`, which reads only `--dry-run`). The `init-metrics` clone runs at the edge with the process streams passed through to git (its chatter reaches the user's stderr); a non-exit failure reports exit code 1. Every other command name keeps the placeholder as defense, exit 1. (4fs0) (vcur) (gzrn)

### Requirement: The sync and --dry-run branches
`tu sync` (`runSync`) runs in order: `config.Load` (warnings to stderr) → the reserved-user check (exit 2, BEFORE the mode check) → the single-mode gate (the two-line `tu sync requires metrics_repo to be set.` message, exit 1) → `MetricsDirGuard` (a demotion exits 1 with just the guard's lines, so `tu sync --dry-run` with a missing dir still clones first) → `--dry-run` prints the byte-exact report on stdout, exit 0 ([dry-run-report](/sync/dry-run-report.md)); the live sync prints its lines to stderr and `Synced to {tilde-path}` on stdout, exit 0, with `!OK` → `Error: sync failed — check network and remote config.`, exit 1 ([git-flow](/sync/git-flow.md)). A filesystem failure prints `err.Error()`, exit 1. The `--sync` block on a data command sits after the reserved-user check and before the watch branch and `command.Run`, so its stderr lines precede Run's notices: `syncing metrics... ` (no trailing newline), the sync under `source.DefaultTimeout` with the SAME `ccusage.Source` the data path uses (the sync's fetch warms the shared cache), then `synced.` or `sync failed — using local data.`; single mode stays silent — no line, no git call ([cache](/source/cache.md)). (lsml)

### Requirement: The watch branch
When `Flags.Watch`, after the reserved-user check and the `--sync` block, `runWatchBranch` runs the loop ([loop-and-terminal](/watch/loop-and-terminal.md)): `command.Normalize` runs ONCE at the edge and its notices print to stderr BEFORE the alt screen (per-poll `Result.Notices` are discarded — `Normalize` does not clear `--full`, so its notice would otherwise repeat every poll); the leaderboard single-mode gate fires before the alt screen exactly as on the one-shot path (the `ErrLeaderboardMode` line, exit 1, no alt screen); the `Poll` closure copies `deps`, sets `Width` and `LiveOptions` from the frame (a nil `Prev` becomes an EMPTY non-nil map on the leaderboards so the indicator column is reserved from the first frame), forces `Flags.Fresh = true` (every poll bypasses the fetch cache), writes the source warnings to stderr every poll (alt screen or not), and returns the lines plus `watch.Stats`. After the loop returns, the last rendered lines print to stdout, exit 0. The terminal constructor and the loop itself are the two test seams (`newWatchTerminal`, `runWatchLoop`). (4pze)

#### Scenario: Notice once, before the alt screen
- **GIVEN** `tu -u bob -w` in single mode
- **WHEN** the binary starts
- **THEN** stderr receives the `-u` notice once, before the alt-screen enter sequence, and no notice repeats on later polls

### Requirement: cmd/tu is the only writer
Nothing below `cmd/tu` writes to stdout/stderr or calls `os.Exit`; `run` returns the process exit code and `main` maps it through `os.Exit`. Every error surfaces as text on stderr plus one of the three exit codes ([request-and-parse](/command/request-and-parse.md)): 0 success (including benign no-ops and warn-and-continue guards), 1 operational failure (including the placeholder), 2 usage error. `shell-init` keeps stdout EMPTY on its usage errors — stdout may be eval'd — and writes its script with no added newline ([shell-init-and-completions](/toolkit/shell-init-and-completions.md)). (4fs0) (8h6g)

### Requirement: Width probe and timezone data
`terminalWidth` probes stdout with `golang.org/x/term` (`IsTerminal` + `GetSize` on the descriptor) and returns the column count, or the `defaultWidth` constant `80` when stdout is not a TTY (a pipe, a test buffer, a probe error); `COLUMNS` is never consulted, and only `cmd/tu` probes — renderers receive the width as a value ([ansi](/render/ansi.md)). `main.go` imports `_ "time/tzdata"` so `TZ` names resolve on hosts without a system zoneinfo database; Go consults the embed only when the system database is missing. (9ax5) (4fs0)

## Design Decisions

### Toolkit commands answer before $HOME resolution
**Decision**: `help`, `help-dump`, `skill`, `shell-init`, and `update` are dispatched BEFORE `config.ResolvePaths`; config path resolution is lazy, at dispatch/read time, so only the config-reading commands (data commands, `init-conf`, `init-metrics`, `sync`, `status`) fail on a missing `$HOME`.
**Why**: none of the toolkit commands needs config; a user with `$HOME` unset (or a broken config) can still get help, completions, the skill bundle, and the version.
**Rejected**: resolving paths once at startup — every command would fail on a missing `$HOME`, including `--help`.
*Introduced by*: gzrn

### Terminal width via golang.org/x/term at the edge
**Decision**: `cmd/tu` probes stdout with `x/term` (`IsTerminal` + `GetSize` on the descriptor) and hands `Deps.Width` (80 when not a TTY) down; render never probes, and `COLUMNS` is never read.
**Why**: the 80-column pipe default is a fixed byte contract; the watch mode and the sibling toolkit tools build on `x/term`; a stdlib ioctl needs per-OS build tags for five lines.
**Rejected**: hardcoding 80 (bars invisible in a real terminal); a stdlib `TIOCGWINSZ` probe (build-tag surface for no gain).
*Introduced by*: 260916-9ax5-history-and-periods
