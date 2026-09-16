# Plan: Config Cascade and Setup Commands (Go port row B1)

**Change**: 260916-4fs0-config-and-setup-commands
**Intake**: `intake.md`

> Read `intake.md` first — it carries every byte-exact message, the node-side reference captures for the 20 harness cases (§2), the cascade rules (§3), the setup-command algorithms (§4–§6), and the package design (§7–§10). This plan restates them as requirements and tasks; where the two disagree, the intake's quoted TypeScript behavior wins, because the external surface is frozen (plan Goal, constitution § Go Transition).

## Requirements

### config: the full cascade

#### R1: Load reproduces the TypeScript readConfig cascade
`config.Load(p Paths, env Env, ov Overrides) (Config, []string)` MUST merge `defaults ∪ org.conf ∪ user conf` (later wins per key), then override `metrics_repo` alone with `TU_METRICS_REPO` when non-empty, then with `ov.MetricsRepo` when non-nil (even when it points at an empty string). It MUST never read `XDG_CONFIG_HOME`, `TU_CONFIG*`, or `TU_HOME`. `Mode` MUST be `Multi` iff the final `MetricsRepo != ""`; a `mode` key in any file MUST be ignored. Unreadable files MUST read as empty layers and `Load` MUST never return an error. `DetectMode` MUST be removed.

- **GIVEN** `org.conf` with `metrics_repo = A` and `tu.conf` with `metrics_repo = B`, env `TU_METRICS_REPO=C`, and `ov.MetricsRepo` pointing at `D`
- **WHEN** `Load` runs
- **THEN** `MetricsRepo == "D"` and `Mode == Multi`; without the override it is `C`; without the env it is `B`; without `tu.conf` it is `A`
- **GIVEN** `TU_METRICS_REPO=""` and no `metrics_repo` in any file
- **WHEN** `Load` runs
- **THEN** `Mode == Single`

#### R2: User-conf selection and the deprecation warning
`Load` MUST read `p.UserConf` when `os.ReadFile` succeeds (legacy ignored silently); else read `p.LegacyConf` when readable AND append the warning `tu: ~/.tu.conf is deprecated; move it to ~/.config/tu/tu.conf` (literal text); else neither. `Config.UserConfRead` MUST be the path actually read or `""`. `Config.OrgRead` MUST be true iff `org.conf` was readable. Warnings MUST be returned in emission order: deprecation before the version warning.

- **GIVEN** only `$HOME/.tu.conf` exists (the harness `legacy` variant)
- **WHEN** `Load` runs
- **THEN** its keys are merged, `UserConfRead == p.LegacyConf`, and warnings is exactly `["tu: ~/.tu.conf is deprecated; move it to ~/.config/tu/tu.conf"]`
- **GIVEN** both `tu.conf` and `.tu.conf` exist
- **WHEN** `Load` runs
- **THEN** only `tu.conf` is read and warnings is empty

#### R3: Version, fields, sentinels, and expansion
`Version` MUST be parsed from `merged["version"]` with JavaScript `parseInt(s, 10)` semantics (optional leading whitespace and sign, longest ASCII-digit run; no digits → 1; missing → 1). When `Version > 2`, `Load` MUST append `Warning: {source} version {N} is newer than tu supports (2). Please update tu.` where `{source}` is `UserConfRead` when non-empty, else `p.OrgConf` when `OrgRead`, else `DefaultConfName` — an absolute path, never tildefied. `MetricsDir` MUST be `ExpandHome(expandSentinels(merged["metrics_dir"] or "~/.tu/metrics_repo"), p.Home)`; `Machine` MUST be `expandSentinels(merged["machine"] or "$HOSTNAME")`; `User` MUST be `expandSentinels(merged["user"] or "$USER")`; `AutoSync` MUST be false only when `auto_sync` is exactly `false` or `0`. `expandSentinels` MUST replace a value that is exactly `$HOSTNAME` with `env.Hostname()` (error → `""`) and exactly `$USER` with `env.Username()` (error → `unknown`), and nothing else. `StateDir(home)` MUST be `$HOME/.tu`; `Tildefy(p, home)` MUST replace a leading `home` prefix with `~`; `ExpandHome` MUST resolve a leading `~/` or bare `~` against `home`.

- **GIVEN** `tu.conf` with `version = 9` and `machine = $HOSTNAME`, `env.Hostname` returning `box`
- **WHEN** `Load` runs
- **THEN** `Version == 9`, `Machine == "box"`, and warnings contains `Warning: {p.UserConf} version 9 is newer than tu supports (2). Please update tu.`
- **GIVEN** `version = 2abc`, `auto_sync = FALSE`, `metrics_dir = ~/x`
- **WHEN** `Load` runs
- **THEN** `Version == 2`, `AutoSync == true`, `MetricsDir == filepath.Join(home, "x")`

#### R4: Embedded shipped defaults with a drift guard
`internal/config/defaults.go` MUST `//go:embed tu.default.conf` into `DefaultConf []byte`, where `src/go/internal/config/tu.default.conf` is a byte-identical copy of the repo-root `tu.default.conf`; `DefaultConfName` MUST be `"tu.default.conf"`. A test MUST locate the repo-root file (walk up from the package directory to the directory containing `package.json`) and assert byte equality. `Load` MUST use `DefaultConf` as its defaults layer; `InitConf` MUST write `DefaultConf` when scaffolding from defaults.

- **GIVEN** the repo checkout
- **WHEN** `go test ./internal/config/` runs
- **THEN** the drift-guard test passes; editing one byte of either copy fails it

### config: setup commands

#### R5: InitConf and the shared ensureUserConf
`InitConf(p Paths) ([]string, error)` MUST implement the intake §4 algorithm: ensureUserConf (create `p.ConfigDir` 0o755; when `p.UserConf` is absent, write it (0o644) from `p.LegacyConf` when that exists with the line `Copied ~/.tu.conf → ~/.config/tu/tu.conf`, else from `DefaultConf` with `Created ~/.config/tu/tu.conf — edit it to configure multi-machine sync.`, both paths via `Tildefy`, and stop); otherwise classify each `FieldBlocks` key in order as present / mentioned / missing using the intake's two regexes, then emit `{dp} is already complete.`, or `Updated {dp} — added missing fields: {keys}.` after appending the missing blocks, and/or `{dp} has commented-out fields that need uncommenting: {keys}.` `FieldBlocks` MUST be the six blocks from the intake, byte-exact, in order `version, metrics_repo, metrics_dir, machine, user, auto_sync`. A filesystem failure MUST be returned as `*config.Error` carrying the OS error text.

- **GIVEN** an empty `$HOME` (harness `single`)
- **WHEN** `InitConf` runs
- **THEN** `$HOME/.config/tu/tu.conf` equals `DefaultConf` and the lines are exactly `["Created ~/.config/tu/tu.conf — edit it to configure multi-machine sync."]`
- **GIVEN** `tu.conf` containing only `version = 2`
- **WHEN** `InitConf` runs
- **THEN** the five other blocks are appended in order and the lines are `["Updated ~/.config/tu/tu.conf — added missing fields: metrics_repo, metrics_dir, machine, user, auto_sync."]`
- **GIVEN** the harness `multi` conf (all six keys active)
- **WHEN** `InitConf` runs
- **THEN** the lines are `["~/.config/tu/tu.conf is already complete."]`

#### R6: InitMetrics up to the clone step
`InitMetrics(p Paths, env Env, url *string, git Git) (InitMetricsResult, error)` MUST implement intake §5 steps 1–5 in order: the `\r`/`\n` URL check (`*Error{"Error: repo-url must be a single line (no newline or carriage-return characters)."}`); ensureUserConf; setMetricsRepoInConf with its three branches (active `metrics_repo` line replaced in place; else commented `#\s*metrics_repo\s*=` line replaced; else the `metrics_repo` block appended with its sample line replaced by `metrics_repo = {url}`), writing `strings.Join(lines, "\n")` for the replace branches; the line `Set metrics_repo = {url} in {Tildefy(p.UserConf)}`; `Load` with the override; `*Error{"Error: metrics_repo is not set. Add it to {dp}, run 'tu init-metrics <repo-url>', or set TU_METRICS_REPO."}` when unset; when `MetricsDir` exists, `git.IsRepo(dir)` → `Already initialized: {dir}` (absolute) with `Clone == nil`, else `*Error{"Error: {dir} exists but is not a git repo. Remove it or set a different metrics_dir in {dp}."}`; otherwise `Clone = &CloneStep{URL: MetricsRepo, Dir: MetricsDir}`. The result MUST also carry the `Load` warnings (`Warnings []string`) so the edge prints them. `ClonedLine(url, dir)` MUST return `Cloned {url} → {dir}`. `CloneFailedMarker` MUST be `.clone-failed`; `RemoveCloneMarker(stateDir)` MUST delete `stateDir/.clone-failed` when present and never fail. `Git` MUST be `interface{ IsRepo(dir string) bool }`.

- **GIVEN** the harness `single` home, `url = "git@example.invalid:harness/tu-metrics.git"`, a `Git` whose `IsRepo` is never called
- **WHEN** `InitMetrics` runs
- **THEN** `tu.conf` exists with the commented sample line replaced by `metrics_repo = git@example.invalid:harness/tu-metrics.git`, `Lines` is `["Created ~/.config/tu/tu.conf — edit it to configure multi-machine sync.", "Set metrics_repo = git@example.invalid:harness/tu-metrics.git in ~/.config/tu/tu.conf"]`, and `Clone` is `{URL: the url, Dir: $HOME/.tu/metrics_repo}`
- **GIVEN** the harness `legacy` home and no url, a `Git` returning true
- **WHEN** `InitMetrics` runs
- **THEN** `Lines` is `["Already initialized: $HOME/.tu/metrics_repo"]`, `Warnings` is the deprecation line, `Clone == nil`, and no file was written
- **GIVEN** the harness `single` home and no url
- **WHEN** `InitMetrics` runs
- **THEN** the error is `Error: metrics_repo is not set. Add it to ~/.config/tu/tu.conf, run 'tu init-metrics <repo-url>', or set TU_METRICS_REPO.`

#### R7: Status data and layout
`Status(p Paths, env Env, now time.Time) (StatusData, []string)` and `(StatusData).Lines()` MUST implement intake §6: the one-line `Mode:        single (no ~/.config/tu/tu.conf)` when neither a readable user/legacy conf nor `org.conf` exists (no `Load`, no warnings); otherwise `Load`, then the single layout (`Mode:        single`, optional `Config:      {Tildefy(selected)} (v{Version})`, optional `Org config:  {Tildefy(OrgConf)}`) or the multi layout (`Mode:        multi`, `User:        {User}`, `Machine:     {Machine}`, optional Config line, optional Org line, `Metrics:     {Tildefy(MetricsDir)}` with the suffix ` (NOT FOUND — run 'tu init-metrics')` when the dir does not exist, `Last sync:   {LastSync}`, `Auto-sync:   on` or `off`). `LastSync` MUST be `never` when `StateDir/.last-sync` is absent, unreadable, or does not parse as RFC3339Nano after trimming, else `{RelativeTime(now − ts)} ({trimmed raw})`. `RelativeTime(d)` MUST floor to whole seconds with negatives clamped to 0 and return `<1m ago`, `{m}m ago`, `{h}h ago`, or `{d}d ago`. Labels MUST be the literal 13-column strings. `Status` MUST write nothing.

- **GIVEN** the harness `org` home
- **WHEN** `Status` runs and `Lines()` is rendered
- **THEN** the lines are exactly `Mode:        multi`, `User:        harness-user`, `Machine:     harness-machine`, `Org config:  ~/.config/tu/org.conf`, `Metrics:     ~/.tu/metrics_repo`, `Last sync:   never`, `Auto-sync:   on`
- **GIVEN** `.last-sync` containing `2026-09-15T18:54:44.502Z\n` and `now = 2026-09-15T19:09:44.502Z`
- **WHEN** `Status` runs
- **THEN** `LastSync == "15m ago (2026-09-15T18:54:44.502Z)"`
- **GIVEN** the harness `legacy` home
- **WHEN** `Status` runs
- **THEN** the Config line is `Config:      ~/.tu.conf (v2)` and the warnings are the deprecation line

### sync: the git driver

#### R8: sync.Exec drives the real git
Package `internal/sync` MUST exist with `type Exec struct{}` satisfying `config.Git`: `IsRepo(dir)` MUST run `git -C <dir> rev-parse --git-dir` with stdout and stderr discarded and return exit-0; `Clone(ctx, url, dir string, stdout, stderr io.Writer) error` MUST run `git clone <url> <dir>` with the writers attached to the child's stdout and stderr and return the exec error on non-zero exit. No timeout and no `GIT_TERMINAL_PROMPT` (those are B3's auto-clone guard). `git` MUST be resolved through `PATH`.

- **GIVEN** a fake `git` first on PATH that records argv and exits 0
- **WHEN** `IsRepo("/x")` and `Clone(ctx, "u", "/y", out, err)` run
- **THEN** the recorded argv are `["-C", "/x", "rev-parse", "--git-dir"]` and `["clone", "u", "/y"]`, `IsRepo` returned true, and `Clone` returned nil
- **GIVEN** a fake `git` exiting 128 and writing `boom` to stderr
- **WHEN** `Clone` runs
- **THEN** `boom` appears on the passed stderr writer and the returned error is an `*exec.ExitError` with code 128

### command: Args, the exit table, and two usage errors

#### R9: Exit-code constants and Request.Args
`command` MUST export `ExitOK = 0`, `ExitOperational = 1`, `ExitUsage = 2`. `Request` MUST gain `Args []string` — the positionals after `Command` (nil for data commands). `UsageError` MUST keep its shape. `cmd/tu` MUST use the constants instead of literals.

- **GIVEN** argv `init-metrics https://x`
- **WHEN** parsed
- **THEN** `Command == "init-metrics"` and `Args == ["https://x"]`

#### R10: The --dry-run misuse guard and the init-metrics arity error, in the TS main() order
`Parse` MUST, after flag validation: (1) return `Version` for any `--version`/`-V`/`-v`; (2) return `Command` for a first positional in {`help`, `-h`, `--help`}; (3) when `Flags.DryRun` and the first positional is not `sync` (including no positional), return `UsageError{Message: "Error: --dry-run is supported only with 'tu sync' — run 'tu sync --dry-run' to preview a sync.", ShowUsage: false}`; (4) set `Command`/`Args` for the other non-data tokens and, for `init-metrics` with `len(Args) > 1`, return `UsageError{Message: "Error: init-metrics takes at most one argument (repo-url)", ShowUsage: true}`; (5) parse the data grammar as before. Data flags on setup commands MUST NOT error (`status --json` sets `Command` and `Format` without a usage error).

- **GIVEN** argv `--dry-run`, `cc --dry-run`, `status --dry-run`, `--dry-run --version`, `help --dry-run`, `sync --dry-run`
- **WHEN** parsed
- **THEN** the first three are the dry-run usage error with `ShowUsage == false`; the fourth is `Version`; the fifth is `Command == "help"`; the sixth is `Command == "sync"` with `DryRun` set and no error
- **GIVEN** argv `init-metrics a b`
- **WHEN** parsed
- **THEN** the usage error message is `Error: init-metrics takes at most one argument (repo-url)` with `ShowUsage == true`

### cmd/tu: dispatch and the data path

#### R11: Setup-command dispatch and write order
`run` MUST, when `Command` is `init-conf`, `init-metrics`, or `status`: resolve paths (`ErrNoHome` → message, `ExitOperational`); build `env` from `os.Getenv`, `os.Hostname`, and `user.Current`; then for `init-conf` print the lines and return `ExitOK` (a `*config.Error` → message on stderr, `ExitOperational`); for `status` print the warnings on stderr, then `Lines()` on stdout, `ExitOK`; for `init-metrics` call `InitMetrics` with `sync.Exec{}` and `url = &Args[0]` when present, print `Warnings` on stderr and `Lines` on stdout (on error: `Lines` first, then the message on stderr, `ExitOperational`), and when `Clone != nil` run `sync.Exec{}.Clone` with the edge's own stdout/stderr — failure → `Error: git clone failed (exit {N}).` on stderr, `ExitOperational`; success → `RemoveCloneMarker(StateDir(paths.Home))` then `ClonedLine` on stdout, `ExitOK`. Every other `Command` MUST stay on the placeholder. `--json`, `--fresh`, `--watch`, `-t` on a setup command MUST be ignored (the handler still runs).

- **GIVEN** the harness `single` home, a fake git on PATH, argv `init-metrics git@example.invalid:harness/tu-metrics.git`
- **WHEN** `run` executes
- **THEN** stdout is the three lines `Created …`, `Set metrics_repo = … in ~/.config/tu/tu.conf`, `Cloned git@example.invalid:harness/tu-metrics.git → $HOME/.tu/metrics_repo`, stderr is empty, exit 0, and the fake git recorded exactly one `clone` call
- **GIVEN** `HOME` unset and argv `status`
- **WHEN** `run` executes
- **THEN** stderr is `tu: $HOME is not set; cannot locate config`, exit 1

#### R12: Data path through Load and the reserved-user guard
For data commands `run` MUST call `config.Load(paths, env, Overrides{})`, print its warnings on stderr before anything else, then apply the reserved-user guard — `cfg.User == "all"` → `Error: config user "all" is reserved (used by -u all)` on stderr, `ExitUsage` — then pass `cfg.Mode` to `command.Run` (the signature is unchanged), then write source warnings and lines as V2 does. The 52 single-mode snapshot harness cases MUST stay green.

- **GIVEN** only `$HOME/.tu.conf` exists and it has no `metrics_repo`, argv empty, the fake ccusage on PATH
- **WHEN** `run` executes
- **THEN** stderr starts with the deprecation line, stdout is the empty snapshot table, exit 0
- **GIVEN** `tu.conf` with `user = all`, argv `cc`
- **WHEN** `run` executes
- **THEN** stderr is `Error: config user "all" is reserved (used by -u all)`, stdout empty, exit 2

### harness: home normalization

#### R13: NormalizeHome in Compare, CompareCallLogs, and the report files
`harness.NormalizeHome(b []byte, home string) []byte` MUST replace every occurrence of `home` in `b` with the literal `$HOME` (byte replacement; empty `home` is a no-op). `SideCapture` MUST gain `Home string`, set by `runCase`. `Compare` MUST normalize each side's `Stdout`/`Stderr`/`TTY` with that side's `Home` before comparing, and compute offset/line/excerpts on the normalized bytes. The per-side report files `{node,go}.{stdout,stderr,tty}` MUST be written normalized. `CompareCallLogs` MUST normalize each side's `argv` entries with that side's home before forming the compared set (it needs both homes — extend its signature). Green/red/timeout semantics, the `harness` channel, and the unconfirmed marker MUST be unchanged.

- **GIVEN** node stdout `Already initialized: /tmp/c/node/home/.tu/metrics_repo\n` with home `/tmp/c/node/home` and go stdout `Already initialized: /tmp/c/go/home/.tu/metrics_repo\n` with home `/tmp/c/go/home`, equal exit and empty stderr
- **WHEN** `Compare` runs
- **THEN** the verdict is green
- **GIVEN** call logs whose argv differ only by the two homes
- **WHEN** `CompareCallLogs` runs
- **THEN** `differ == false`

#### R14: Harness gate
After all tasks, `just go-diff --placeholder` MUST report green for the 20 setup-command case IDs listed in intake §2 plus `dry-run/single/default/pipe/fixed` and `cc-dry-run/single/default/pipe/fixed`, with no regression among the cases green after V2 (the summary's first line moves from 52 to at least 74 green), and the `[calls differ]` marker MUST be absent from the `init-metrics` groups.

- **GIVEN** `just build`, `just go-build`, `just harness-build` succeed
- **WHEN** `bin/harness/tudiff run --placeholder --filter init-metrics` runs
- **THEN** all 12 cases are GREEN with no `[calls differ]` suffix

### Non-Goals

- Multi-mode data path, metrics reader, auto-clone guard, `-u` (B3); sync, `.last-sync` writing, `sync --dry-run` (B6); watch (B7); help/help-dump/shell-init/skill/update (B8).
- No change under `src/node/`, `docs/specs/`, `harness/matrix.json`, `harness/metrics-repo/`, `justfile`, or CI.

### Design Decisions

#### Embedded shipped defaults
**Decision**: `tu.default.conf` is embedded into the Go binary from a byte-identical copy under `internal/config`, drift-guarded by a test.
**Why**: No runtime file lookup, no silent empty-defaults failure mode when the file is missing, and tests need no path injection; the install layout is not a Goal-listed surface.
**Rejected**: Reproducing the TS beside-the-binary then walk-up lookup — a runtime dependency on a file the Go tarball would otherwise not need, and a missing file degrades `init-conf` to an empty scaffold.
*Introduced by*: 260916-4fs0-config-and-setup-commands

#### Setup commands return values; the edge executes the clone
**Decision**: `InitConf`, `InitMetrics`, `Status` live in `internal/config` and return lines plus a typed `config.Error`; `InitMetrics` returns a `CloneStep` that `cmd/tu` runs through `sync.Exec`, then prints `ClonedLine`.
**Why**: Keeps "nothing prints below `cmd/tu`" while preserving the TS ordering of the `Set metrics_repo` line before git's clone chatter; `config` execs nothing.
**Rejected**: Passing `io.Writer`s into `config` (makes `config` a writer); running the clone inside `config` via an injected interface that takes writers (same problem, one level down).
*Introduced by*: 260916-4fs0-config-and-setup-commands

#### Git driver lands in sync
**Decision**: `internal/sync/git.go` holds the real git driver (`Exec`) that satisfies `config.Git`; B6 grows the package.
**Why**: The Target architecture table assigns the git driver to `sync`; `config` stays exec-free and G1 item 1's "only `source` execs" reads as "plus `sync`'s git driver" — flagged in the intake's Open Questions.
**Rejected**: Exec in `config` (blurs the boundary G1 checks); exec in `cmd/tu` (main.go stops being thin and B6 would need the driver anyway).
*Introduced by*: 260916-4fs0-config-and-setup-commands

#### Compare-side home normalization
**Decision**: The harness replaces each side's staged home with `$HOME` in captures and call-log argv before comparing, and writes normalized report files.
**Why**: Two fresh homes per case (P4's decision) is still right; the absolute-path messages of `init-metrics` are Goal-frozen surfaces that legitimately embed `$HOME`.
**Rejected**: A shared home (masks divergences through shared cache and day-files); staging both sides at the same absolute path via a symlink (fragile across `script`/tty and still leaks the side's real dir through `cwd`).
*Introduced by*: 260916-4fs0-config-and-setup-commands

## Tasks

### Phase 1: Setup

- [x] T001 Add `src/go/internal/config/tu.default.conf` as a byte-identical copy of the repo-root `tu.default.conf`; add `src/go/internal/config/defaults.go` with `//go:embed tu.default.conf` into `DefaultConf []byte` and `const DefaultConfName = "tu.default.conf"`; add `defaults_test.go` walking up to the `package.json` directory and asserting byte equality with the root file <!-- R4 -->
- [x] T002 [P] Add `ExitOK`/`ExitOperational`/`ExitUsage` constants and `Args []string` on `Request` in `src/go/internal/command/request.go` <!-- R9 -->
- [x] T003 [P] Add `src/go/internal/harness/diff.go` `NormalizeHome(b []byte, home string) []byte` and `Home string` on `SideCapture`, with table tests in `diff_test.go` (multiple occurrences, prefix inside a longer path, empty home no-op) <!-- R13 -->

### Phase 2: Core Implementation

- [x] T004 In `src/go/internal/config/config.go`: add `StateDir`, `Tildefy`, `ExpandHome`, `Env`, `Overrides`, `Config`, `CurrentConfigVersion = 2`, `parseIntJS`, `expandSentinels`, and `Load` per R1–R3; remove `DetectMode` (keep `userConfPath`-style selection as an internal helper that also reports the path read). Migrate the `DetectMode` rows in `config_test.go` into table-driven `Load` tests covering every R1–R3 scenario (temp HOMEs, injected `Env`) <!-- R1 -->
- [x] T005 Add `src/go/internal/config/setup.go`: `Error`, `FieldBlocks` (six byte-exact blocks), `ensureUserConf`, `fieldPresent`, `fieldMentioned`, `InitConf` per R5, with `setup_test.go` covering create-from-defaults, copy-from-legacy, already-complete, missing-fields append (file bytes asserted), commented-fields report, and both reports together <!-- R5 -->
- [x] T006 In `src/go/internal/config/setup.go`: add `Git`, `CloneStep`, `InitMetricsResult{Lines, Warnings, Clone}`, `setMetricsRepoInConf` (three branches), `InitMetrics` per R6, `ClonedLine`, `CloneFailedMarker`, `RemoveCloneMarker`; tests for the newline check, each replace/append branch with resulting file bytes, unset repo error, dir-not-a-repo error, already-initialized (with the legacy warning case), clone-needed, override-beats-env, and `RemoveCloneMarker` present/absent <!-- R6 -->
- [x] T007 Add `src/go/internal/config/status.go`: `StatusData`, `Status`, `Lines`, `RelativeTime`, `lastSync` per R7, with `status_test.go` covering every layouts §13 block (no-config, single with config, single with legacy + warning, single with org, org-only multi, multi with config, NOT FOUND suffix, `off`), `.last-sync` present/absent/garbage with a fixed `now`, and `RelativeTime` boundaries (0, 59 s, 60 s, 59 min, 60 min, 23 h, 24 h, negative) <!-- R7 -->
- [x] T008 [P] Add `src/go/internal/sync/git.go` (package doc naming B6 as the owner of the rest) with `Exec` implementing `IsRepo` and `Clone` per R8, and `git_test.go` using a fake `git` script written into a temp dir placed first on PATH that records argv to a file and honors an exit code and stderr text from env <!-- R8 -->
- [x] T009 In `src/go/internal/command/parse.go`: capture `Args` after a non-data command; reorder to version → help → `--dry-run` guard → non-data command (with the `init-metrics` arity error, `ShowUsage: true`) → data grammar per R10; add `parse_test.go` rows for every R9/R10 scenario plus `status --json` and `init-metrics a` <!-- R10 -->

### Phase 3: Integration & Edge Cases

- [x] T010 In `src/go/cmd/tu/main.go`: replace exit literals with the constants; add the setup-command dispatch per R11 (`init-conf`, `status`, `init-metrics` with `sync.Exec{}`, clone execution with passthrough writers, marker removal, `ClonedLine`, the `Error: git clone failed (exit N).` path); build `config.Env` from `os.Getenv`/`os.Hostname`/`user.Current`; keep every other `Command` on the placeholder <!-- R11 -->
- [x] T011 In `src/go/cmd/tu/main.go`: switch the data path from `DetectMode` to `config.Load`, print its warnings on stderr first, add the reserved-user guard (exit `ExitUsage`), pass `cfg.Mode` to `command.Run` per R12; update `main_test.go` so `init-conf`/`init-metrics`/`status` leave the placeholder list and the `--dry-run` / `init-metrics a b` usage errors are asserted byte-exact <!-- R12 -->
- [x] T012 In `src/go/cmd/tu/e2e_test.go`: build `fakegit` as `git` into the PATH dir in `TestMain`; add byte-exact cases (using `harness.StageHome` for the four variants and a fixture-free `TUDIFF_GIT_SCRIPT`-less fake git) for `status` ×4 variants, `init-conf` create/copy/complete, `init-metrics` unset/already-initialized/clone (asserting the recorded git argv via `TUDIFF_CALL_LOG`), `init-metrics a b`, `tu --dry-run`, `tu` with a legacy conf and no `metrics_repo` (deprecation line then the empty table), `tu cc` with `user = all` (exit 2), and `tu status` with `HOME` unset (exit 1) <!-- R11 -->
- [x] T013 In `src/go/internal/harness/diff.go` and `src/go/cmd/tudiff/run.go`: set `SideCapture.Home` in `runCase`; make `Compare` normalize each side's byte channels with its own home before comparing and before `firstDivergence`; write normalized bytes to the per-side report files; extend `CompareCallLogs` to take both homes and normalize argv; add tests for a home-only stdout difference → green and home-only argv difference → not differ <!-- R13 -->

### Phase 4: Polish

- [x] T014 Run `just go-lint`, `just go-test`, then `just build && just go-build && just harness-build && bin/harness/tudiff run --placeholder` and confirm R14: the 22 target cases green, no regression among the previously green cases, no `[calls differ]` on the `init-metrics` groups; record the summary line in `## Notes` <!-- R14 -->

## Execution Order

- T001, T002, T003 are independent and precede everything.
- T004 blocks T005, T006, T007 (they call `Load` and the helpers); T005 blocks T006 (shared `ensureUserConf`).
- T008 is independent of the config tasks; T009 depends on T002.
- T010 depends on T004–T009; T011 depends on T004 and T010; T012 depends on T010–T011; T013 depends on T003.
- T014 last.

## Acceptance

### Functional Completeness

- [x] A-001 R1: `Load` merges defaults, org, user, env, and override in the specified order with no per-key inversion; `DetectMode` no longer exists and nothing references it
- [x] A-002 R2: Legacy fallback fires only when `tu.conf` is unreadable and returns the exact deprecation line; `UserConfRead` and `OrgRead` are set as specified
- [x] A-003 R3: Version parsing, the version warning with the correct source attribution, sentinel expansion, `~` expansion, and `auto_sync` parsing match the TypeScript
- [x] A-004 R4: `DefaultConf` is embedded and the drift-guard test asserts byte equality with the repo-root file
- [x] A-005 R5: `InitConf` produces the exact `Created`, `Copied`, `already complete`, `Updated … added missing fields`, and `commented-out fields` lines and writes the exact file bytes
- [x] A-006 R6: `InitMetrics` performs the newline check, conf scaffolding, the three-branch `metrics_repo` write, the `Set …` line, the cascade read with override, and the four outcomes (unset error, not-a-repo error, already initialized, clone step), carrying the `Load` warnings
- [x] A-007 R7: `Status` renders every layouts §13 block byte-exact with 13-column labels and the `never` / `{relative} ({ISO})` last-sync forms
- [x] A-008 R8: `sync.Exec` issues exactly `-C <dir> rev-parse --git-dir` and `clone <url> <dir>` and passes the child's output through the given writers
- [x] A-009 R9: The exit constants exist and are the only exit values `cmd/tu` returns; `Request.Args` carries post-command positionals
- [x] A-010 R10: The dry-run guard and the init-metrics arity error fire with the exact messages in the TS order (version and help win over the guard)
- [x] A-011 R11: `cmd/tu` dispatches the three setup commands with the specified write order and exit codes, executes the clone at the edge, removes the marker, and ignores data flags on setup commands
- [x] A-012 R12: The data path prints `Load` warnings first and enforces the reserved-user guard with exit 2
- [x] A-013 R13: `Compare`, the report files, and `CompareCallLogs` operate on home-normalized bytes and argv
- [x] A-014 R14: `tudiff run --placeholder` reports the 22 target cases green with no regression and no `[calls differ]` on the `init-metrics` groups

### Behavioral Correctness

- [x] A-015 R12: A single-mode snapshot with a legacy-only conf prints the deprecation line on stderr and the table on stdout with exit 0 (the table bytes unchanged from V2)
- [x] A-016 R11: `tu status --json` and `tu init-conf --fresh` behave exactly like the flag-less commands

### Scenario Coverage

- [x] A-017 R6: Unit tests cover the `init-metrics-url` legacy scenario: `Copied` line, `Set` line, no deprecation warning (the new file exists before `Load` runs)
- [x] A-018 R7: A unit test pins `15m ago (2026-09-15T18:54:44.502Z)` from a fixed `now`
- [x] A-019 R11: The e2e test records exactly one `clone` git call for the single-home `init-metrics <url>` case and exactly one `rev-parse` call for the multi-home no-url case

### Edge Cases & Error Handling

- [x] A-020 R6: A URL containing `\n` or `\r` is rejected with exit 1 and the exact message before any file is written
- [x] A-021 R3: `version = 2abc` parses as 2 and `version = x` as 1 without a warning; `version = 9` warns naming the absolute path of the file actually read
- [x] A-022 R11: `HOME` unset on `status`, `init-conf`, and `init-metrics` prints `tu: $HOME is not set; cannot locate config` with exit 1, while `init-metrics a b` with `HOME` unset still exits 2 (arity precedes the HOME check)
- [x] A-023 R7: A `.last-sync` file with unparseable content renders `never`

### Code Quality

- [x] A-024 Pattern consistency: New Go code follows the V1/V2 conventions (doc comments naming the TS function mirrored, table-driven tests, `_test.go` siblings, no `__tests__/`)
- [x] A-025 No unnecessary duplication: `ensureUserConf` is shared by `InitConf` and `InitMetrics`; the user-conf selection helper is shared by `Load` and `Status`; `Tildefy`/`ExpandHome`/`StateDir` are the only path helpers
- [x] A-026 Minimum pathways: setup commands and the data path both read config through `Load` (one cascade implementation)
- [x] A-027 No magic strings: the exit codes, `CloneFailedMarker`, `DefaultConfName`, `CurrentConfigVersion`, and the 13-column labels are named constants or one literal table
- [x] A-028 Errors are never swallowed silently: `RemoveCloneMarker` is the one documented best-effort call; every other failure returns an error or a warning line
- [x] A-029 Nothing below `cmd/tu` writes to stdout/stderr or calls `os.Exit`; `config` execs nothing; `gofmt -l` and `go vet` are clean (`just go-lint`)

## Notes

- Harness gate (T014, 2026-09-16): `tudiff: 368 cases — 95 green, 273 red, 0 timeout   (fixtures: _placeholder; 148 cases replayed unconfirmed fixtures)`. Green-set diff against a HEAD baseline binary (73 green) shows exactly the 22 target cases newly green, zero regressions, and no `[calls differ]` marker on the init-metrics groups.
- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`
- Do NOT run `git checkout`, `git stash`, or `git restore` on tracked files during apply or review; the working tree is the deliverable.

## Deletion Candidates

None — the one planned removal (`DetectMode`) is executed in the diff itself (function and its tests deleted in place, callers migrated to `Load`); the change adds new functionality without making any other existing code redundant or unused.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | `InitMetricsResult` carries `Warnings []string` alongside `Lines` and `Clone`, so one call returns everything the edge prints | The intake left the shape open; one result value keeps the edge a straight-line writer | S:60 R:90 A:85 D:70 |
| 2 | Confident | `CompareCallLogs` gains two home parameters rather than reading homes from the log's `cwd` field | Explicit inputs match how `Compare` receives `Home`; the `cwd` field is per-side metadata the comparison already ignores | S:55 R:90 A:85 D:70 |
| 3 | Confident | The fake git for the `sync` unit test is a shell script written by the test, not the built `fakegit` binary | The package test must not depend on a build step in another command; the e2e test uses the real `fakegit` | S:55 R:90 A:85 D:65 |
| 4 | Certain | `command.Run` keeps its `Mode` parameter; B3 widens it to `Config` | The intake states this explicitly; nothing in B1's scope needs more than `Mode` | S:85 R:90 A:90 D:90 |

4 assumptions (1 certain, 3 confident, 0 tentative).
