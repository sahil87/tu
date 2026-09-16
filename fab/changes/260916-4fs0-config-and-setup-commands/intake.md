# Intake: Config Cascade and Setup Commands (Go port row B1)

**Change**: 260916-4fs0-config-and-setup-commands
**Created**: 2026-09-16

## Origin

One-shot `/fab-new` invocation, handed over from the Go-port plan's queue (plan row B1, the third Phase 1 queue row, after V2 merged as PR #85):

> Context: fab/plans/sahil/26-09-15-go-port.md, row B1. Read the Decisions and Target architecture sections before writing the intake. Do not change any external surface listed under Goal. Build the config cascade (tu.conf/org.conf/legacy), init-conf, status, init-metrics (git clone, clone marker), and the exit-code table. Harness gate: conf variants and setup commands.

No prior discussion in this conversation. Sources read to ground every value below: the plan's Decisions (D1–D13), Target architecture table, G1 review protocol and checklist; the V2 memory `docs/memory/go-port/command-edge.md` and the V2 code (`src/go/internal/config/config.go`, `internal/command/{request,parse,run}.go`, `cmd/tu/main.go`, `cmd/tu/e2e_test.go`); the harness memory `docs/memory/harness/differential-harness.md`, `harness/matrix.json`, `src/go/internal/harness/{homes,matrix,diff}.go`, `src/go/cmd/fakegit/main.go`, `src/go/cmd/tudiff/run.go`; the TypeScript memory `docs/memory/configuration/config-system.md`; the specs `docs/specs/usage.md` (§ Global Flags, § Setup Commands, § Toolkit Contracts › config-home, § Exit Codes, § Multi-Machine Mode › Configuration, › Auto-Clone Guard, › Staleness, § Drop at cutover DC-02/DC-03/DC-19/DC-20) and `docs/specs/layouts.md` (§13 Status, §20 Setup, Sync, and Diagnostic Messages); the shipped TypeScript — `src/node/core/config.ts` (all of it: `resolveConfigPaths`, `findDefaultConf`, `parseConf`, `expandSentinels`, `resolveHome`, `selectUserConf`, `selectUserConfPath`, `readConfig`), `src/node/core/cli.ts` (`EXIT_USAGE`, `tildefy`, `assertUserNotReserved`, `FIELD_BLOCKS`, `fieldPresent`, `fieldMentioned`, `normalizePaths`, `ensureUserConf`, `setMetricsRepoInConf`, `runInitConf`, `runInitMetrics`, `relativeTime`, `formatLastSync`, `printOrgLine`, `runStatus`, `removeCloneMarker`, `CLONE_FAILED_MARKER`, the `main()` dispatch order), and `tu.default.conf`. A live `just go-diff --placeholder` run was made for the three setup-command filters (`status`, `init-conf`, `init-metrics`): all 20 cases are red today (Go exit 1 placeholder against node exit 0 or 2), and the node-side captures under `bin/harness/report/cases/` are the reference bytes quoted verbatim in §2 and §10.

Plan context that shapes this change:

- **D2** — Go lands dark in `main`; nothing ships. B1 makes the Go binary answer `init-conf`, `init-metrics`, `status`, and the config-dependent parts of the data path for real. Every other recognized-but-unported request keeps printing `tu: not implemented (Go port in progress)` (stderr, exit 1).
- **D4** — feature freeze on `src/node/`. Nothing under `src/node/` changes. Two stale TypeScript comments were noticed (`cli.ts` says the `--dry-run` guard "fails fast, exit 1" while the code exits 2; the spec and code agree on 2) and are left alone.
- **D6 / D11** — the harness is the gate; the metrics-repo protocol is B6's and is not touched. B1 clones and probes the metrics dir, it never reads or writes day-files.
- **Target architecture** — `config` owns "`tu.conf` / `org.conf` / legacy cascade, config-home standard, `init-conf`, `status` data"; `sync` owns the "git driver"; `command` parses and composes; `cmd/tu` is the only writer. B1 is the row that grows the V2 stub `internal/config` (Paths, ParseConf, DetectMode) into the full cascade and lands the first file of `internal/sync` (the git driver only).
- **Goal** — every external surface is frozen. B1 reproduces byte for byte: the cascade semantics, the `init-conf` / `init-metrics` / `status` stdout and stderr lines, the legacy-deprecation and version warnings, the `$HOME`-unset error, the reserved-user guard, the `--dry-run` misuse guard, the `init-metrics` arity error, and the per-subcommand exit codes.
- **G1** — the agent gate reviews V1+V2+B1+B2 as a unit: package boundaries, typed errors surfaced at the edge, no `os.Exit` or stderr writes below `cmd/tu`, harness diff count fell for the claimed case classes (conf variants). This intake records every layering choice so G1 can see it, and flags one wording tension in the G1 checklist (Open Questions).

## Why

V2 wired the pipeline end to end for one display in single mode, and to do so it stubbed `internal/config` down to the minimum that decides single vs. multi (`DetectMode`). Everything the shipped tool does with configuration beyond that decision is still missing on the Go side: the shipped defaults layer, the `~`/sentinel expansion that produces the real `metrics_dir`/`machine`/`user`, the legacy-deprecation and version warnings, the reserved-user guard, and the three setup commands users run first (`init-conf`, `init-metrics`, `status`). Those are the entry points of the multi-machine story, and B3 (metrics source and multi mode), B5 (leaderboard), and B6 (sync) all consume the `Config` value B1 defines — `MetricsRepo`, `MetricsDir`, `Machine`, `User`, `AutoSync`, `Mode`. Landing the cascade before them means each later row reads one settled type rather than growing its own view of the conf files.

The TypeScript side this replaces is small but scattered: `config.ts` holds the cascade with a module-level `Set` guarding the once-per-process deprecation warning and `console.error` calls inside the reader; `cli.ts` holds `runInitConf`, `runInitMetrics`, `runStatus` with direct `console.log`/`process.exit` sites (11 of the 107 the plan counts), the exit-code convention as a comment plus a single constant, and the clone marker helpers. The Go shape returns values: `config.Load` returns a `Config` plus its warnings as strings; the setup commands return lines and a typed error; `cmd/tu` writes and maps errors to the exit table. Nothing below `cmd/tu` prints or exits, which is G1 checklist items 3 and 4 made concrete for the config path.

The harness makes the row checkable and also exposed a harness gap. Today all 20 setup-command cases (`status`, `init-conf`, `init-metrics`, `init-metrics-url`, `init-metrics-extra` × the four conf variants) are red because the Go side prints the placeholder. Three of those groups print an absolute path that embeds `$HOME` (`Already initialized: /tmp/tudiff-…/cases/…/node/home/.tu/metrics_repo`), and the harness deliberately stages two different `$HOME`s per case (`…/node/home` vs `…/go/home`), so those cases can never go green without normalizing each side's home path before comparison. That normalization is in scope: the plan row names "Harness: conf variants + setup commands" as the gate, and a gate that cannot be reached is not a gate.

## What Changes

### 1. Package layout

```
src/go/
  cmd/tu/
    main.go                 run(): parse → version → command dispatch → config.Load → data path; still the ONLY writer
    main_test.go            updated: init-conf/init-metrics/status leave the placeholder list; dry-run/arity usage errors added
    e2e_test.go             TestMain also builds fakegit onto PATH; setup-command and config-warning cases added
  internal/
    config/                 GROWN from the V2 stub
      config.go             Paths, ResolvePaths (unchanged), StateDir, Tildefy, ParseConf (unchanged), Mode, Config, Env, Overrides, Load
      defaults.go           go:embed of tu.default.conf (the shipped defaults, byte-identical copy) + DefaultConfName
      tu.default.conf       embedded copy of the repo-root file (drift-guarded by a test)
      setup.go              FieldBlocks, InitConf, InitMetrics, InitMetricsResult, CloneStep, ClonedLine, RemoveCloneMarker, Error
      status.go             StatusData, Status, (StatusData).Lines, RelativeTime
      *_test.go             table-driven; temp HOMEs; fixed clock and fixed hostname/username
    sync/                   NEW (git driver only — B6 grows the package)
      git.go                Git interface {IsRepo}, Exec (the real driver: IsRepo, Clone with passthrough writers)
      git_test.go           against a fake git on PATH
    command/
      request.go            Request gains Args []string; exit-code constants ExitOK/ExitOperational/ExitUsage
      parse.go              Args capture; the --dry-run misuse guard; the init-metrics arity usage error; TS main() order
      parse_test.go         new rows
      run.go                inScope unchanged in spirit; no change to Run's composition
    harness/
      diff.go               NormalizeHome; Compare and CompareCallLogs compare normalized bytes
      diff_test.go          new rows
  cmd/tudiff/
    run.go                  passes each side's staged HOME into normalization; report files hold the normalized bytes
```

No new module dependencies; `go.mod` stays `require`-free (`os/user`, `os/exec`, `embed` are stdlib). No `justfile`, workflow, `src/node/`, spec, `harness/matrix.json`, or `harness/metrics-repo/` edit. `docs/memory/` changes are hydrate's (Affected Memory).

### 2. Scope, the row map after B1, and the harness gate

**In scope (produces real output)**:

- The non-data commands `init-conf`, `init-metrics [repo-url]`, `status`, in every conf variant and mode.
- The full config cascade on the **data path**: `cmd/tu` calls `config.Load` instead of `DetectMode`, prints its warnings on stderr (legacy deprecation, newer-version), applies the reserved-user guard (exit 2), and passes `Config` to `command.Run`. The in-scope data grammar itself does not widen: single-mode snapshot, table or JSON, as V2 left it. Multi mode stays on the placeholder (B3).
- Two usage errors owned by the exit-code table: the `--dry-run` misuse guard and the `init-metrics` arity check.
- The `$HOME`-unset error for the three setup commands (the data path already had it).

**Recognized but unported (placeholder, unchanged)**: multi mode and `-u` (B3); `h`/`dh`/`wh`/`mh`, `--csv`/`--md`, `--since`/`--until`/`--full` (B2); `--by-machine` (B4); `lb`/`lbh`/`--top` (B5); the `sync` command, `--sync`, `sync --dry-run` (B6); `--watch`/`--interval`/`--no-rain` (B7); `help`/`-h`/`--help` first-arg, `help-dump`, `skill`, `shell-init`, `update`, `--skip-brew-update` (B8). The V2 row map in `command-edge.md` changes in exactly two places: `init-conf`/`init-metrics`/`status` leave it (done here), and the `--dry-run` **misuse guard** moves from B6 to B1 (B6 keeps `tu sync --dry-run` itself). The auto-clone guard on multi-mode data commands (`checkMetricsDirGuard`: clone attempt, 30 s timeout, `GIT_TERMINAL_PROMPT=0`, marker write, `Warning: … falling back to single mode.`) is **B3's**; B1 provides only the marker constant and `RemoveCloneMarker` (§5).

**The gate.** `just go-diff --placeholder` must report these **20 case IDs green** (all `default/pipe/fixed` — the setup groups carry only the `conf` axis):

```
status/{single,multi,org,legacy}/default/pipe/fixed               (4)
init-conf/{single,multi,org,legacy}/default/pipe/fixed            (4)
init-metrics/{single,multi,org,legacy}/default/pipe/fixed         (4)
init-metrics-url/{single,multi,org,legacy}/default/pipe/fixed     (4)
init-metrics-extra/{single,multi,org,legacy}/default/pipe/fixed   (4)
```

Two more cases go green as a bonus from the `--dry-run` guard: `dry-run/single/default/pipe/fixed` and `cc-dry-run/single/default/pipe/fixed`. Nothing that was green after V2 (the 52 single-mode snapshot cases plus the usage-error groups) may regress; the summary's first line moves from 52 green to at least 74 green. The `[calls differ]` marker on the `init-metrics` groups (node issues one `git` call, Go issued none) must disappear once Go probes and clones through the fake git with the same argv (§7) and the call-log comparison normalizes home paths (§10).

**Reference bytes** (node side, captured 2026-09-16 under `--placeholder`; `$HOME` stands for the staged home). Every line ends in `\n`.

| Case (conf) | exit | stdout | stderr |
|-------------|------|--------|--------|
| `status` single | 0 | `Mode:        single (no ~/.config/tu/tu.conf)` | — |
| `status` multi | 0 | `Mode:        multi` / `User:        harness-user` / `Machine:     harness-machine` / `Config:      ~/.config/tu/tu.conf (v2)` / `Metrics:     ~/.tu/metrics_repo` / `Last sync:   never` / `Auto-sync:   on` | — |
| `status` org | 0 | as multi but the `Config:` line is replaced by `Org config:  ~/.config/tu/org.conf` in the same position | — |
| `status` legacy | 0 | as multi but `Config:      ~/.tu.conf (v2)` | `tu: ~/.tu.conf is deprecated; move it to ~/.config/tu/tu.conf` |
| `init-conf` single, org | 0 | `Created ~/.config/tu/tu.conf — edit it to configure multi-machine sync.` | — |
| `init-conf` multi | 0 | `~/.config/tu/tu.conf is already complete.` | — |
| `init-conf` legacy | 0 | `Copied ~/.tu.conf → ~/.config/tu/tu.conf` | — |
| `init-metrics` single | 1 | — | `Error: metrics_repo is not set. Add it to ~/.config/tu/tu.conf, run 'tu init-metrics <repo-url>', or set TU_METRICS_REPO.` |
| `init-metrics` multi, org | 0 | `Already initialized: $HOME/.tu/metrics_repo` | — |
| `init-metrics` legacy | 0 | `Already initialized: $HOME/.tu/metrics_repo` | the deprecation line |
| `init-metrics-url` single | 0 | `Created …` / `Set metrics_repo = git@example.invalid:harness/tu-metrics.git in ~/.config/tu/tu.conf` / `Cloned git@example.invalid:harness/tu-metrics.git → $HOME/.tu/metrics_repo` | — |
| `init-metrics-url` multi | 0 | `Set metrics_repo = … in ~/.config/tu/tu.conf` / `Already initialized: $HOME/.tu/metrics_repo` | — |
| `init-metrics-url` org | 0 | `Created …` / `Set …` / `Already initialized: …` | — |
| `init-metrics-url` legacy | 0 | `Copied ~/.tu.conf → ~/.config/tu/tu.conf` / `Set …` / `Already initialized: …` | — (no deprecation: the new file now exists when the cascade reads) |
| `init-metrics-extra` all four | 2 | — | `Error: init-metrics takes at most one argument (repo-url)` then `ShortUsage` |

Git calls recorded by the fake: `init-metrics`/`init-metrics-url` in multi/org/legacy issue exactly `["-C", "$HOME/.tu/metrics_repo", "rev-parse", "--git-dir"]`; `init-metrics-url` in single issues exactly `["clone", "git@example.invalid:harness/tu-metrics.git", "$HOME/.tu/metrics_repo"]`; the other cases issue no git call.

### 3. `config` — the full cascade (`config.go`, `defaults.go`)

The V2 API that stays: `Paths{Home, ConfigDir, UserConf, OrgConf, LegacyConf}`, `ResolvePaths(home) (Paths, error)` with `ErrNoHome` (`tu: $HOME is not set; cannot locate config`), `ParseConf(raw) map[string]string`, `Mode` with `Single`/`Multi`. `DetectMode` is **removed** — `Load(...).Mode` replaces its one caller; its table-driven tests migrate to `Load`.

New helpers:

```go
// StateDir is the runtime-state root, the TS TU_HOME: $HOME/.tu (cache, metrics
// repo clone, .last-sync, .clone-failed). Config lives under $HOME/.config/tu —
// the two roots are intentionally separate (config-system memory).
func StateDir(home string) string            // filepath.Join(home, ".tu")

// Tildefy abbreviates a path under home to "~/…" (TS tildefy: prefix match on
// the home string, no trailing-slash handling); other paths are returned as-is.
func Tildefy(p, home string) string

// ExpandHome resolves a leading "~/" or a bare "~" against home (TS resolveHome).
func ExpandHome(p, home string) string
```

The defaults layer is **embedded**: `defaults.go` carries `//go:embed tu.default.conf` into `var DefaultConf []byte`, where `src/go/internal/config/tu.default.conf` is a byte-identical copy of the repo-root `tu.default.conf` (the file `scripts/build.sh` copies beside `tu.mjs`). A test in `defaults_test.go` walks up from the package dir to the repo root and asserts the two files are byte-equal, so the copy cannot drift silently. `DefaultConfName = "tu.default.conf"` is the string the version warning prints when the newer-than-supported `version` came from the defaults (unreachable with the shipped file, which says `version = 2`; kept for shape parity). Rationale in Design Decisions; the TS beside-the-bundle/walk-up lookup is not reproduced.

The reader:

```go
// Env is Load's view of the process environment, injected so tests pin
// hostname/username/env without touching the real process.
type Env struct {
    Getenv   func(string) string   // os.Getenv at the edge
    Hostname func() (string, error) // os.Hostname
    Username func() (string, error) // user.Current().Username
}

// Overrides is the CLI-argument layer (the fifth layer of the cascade);
// only metrics_repo can be overridden, exactly as in the TS.
type Overrides struct{ MetricsRepo *string }

type Config struct {
    Version     int
    Mode        Mode
    MetricsRepo string
    MetricsDir  string  // sentinels expanded, then ~ expanded against Home
    Machine     string  // sentinel expanded
    User        string  // sentinel expanded
    AutoSync    bool
    UserConfRead string // the user-conf path actually read: UserConf, LegacyConf, or "" (neither readable)
    OrgRead      bool   // org.conf was readable and fed the merge
}

// Load is the TS readConfig. It reads files and returns values: the Config,
// the stderr warning lines the edge prints (in order), and never an error —
// unreadable files are empty layers, exactly as the TS `readConfFile` returns null.
func Load(p Paths, env Env, ov Overrides) (Config, []string)
```

Semantics, all byte-exact where a string is involved:

1. **Cascade order, no per-key inversion**: `merged = defaults ∪ org ∪ user` (later wins per key), then `metrics_repo` alone is overridden by `TU_METRICS_REPO` when non-empty (an empty value is unset), then by `Overrides.MetricsRepo` when non-nil (a CLI argument beats the env var even when the argument is the empty string — the TS `overrides.metrics_repo ?? …` nullish test). `TU_METRICS_REPO` is the **only** config-bearing environment variable; `XDG_CONFIG_HOME`, `TU_CONFIG*`, `TU_HOME` are never read.
2. **User-conf selection**: `UserConf` readable → use it, ignore legacy silently; else `LegacyConf` readable → use it AND append the warning `tu: ~/.tu.conf is deprecated; move it to ~/.config/tu/tu.conf` (literal text, not built from the path); else neither. "Readable" means `os.ReadFile` succeeds — an existing-but-unreadable file falls back, as the TS does (`selectUserConfPath` exists in the TS precisely so `status` mirrors this). The TS once-per-process `Set` guard is unnecessary in Go: `Load` is called once per invocation on every path B1 wires (watch mode, which re-reads, is B7's and will call `Load` per tick and de-duplicate at the edge if needed).
3. **Mode**: `Multi` iff the final `MetricsRepo != ""`. A `mode` key in any file is ignored.
4. **Version**: `merged["version"]` parsed with JavaScript `parseInt(s, 10)` semantics — optional leading whitespace and sign, then the longest run of ASCII digits (`"2abc"` → 2), no digits → NaN → 1; missing → 1. When `Version > 2` (`CurrentConfigVersion = 2`), append `Warning: {source} version {N} is newer than tu supports (2). Please update tu.` where `{source}` is `UserConfRead` when non-empty, else `p.OrgConf` when `OrgRead`, else `DefaultConfName` — an **absolute** path, not tildefied (spec: "`Warning: {absolute path} version {N} …`"; layouts §20 shows `/home/user/.config/tu/tu.conf version 9`).
5. **Fields**: `MetricsDir = ExpandHome(expandSentinels(merged["metrics_dir"] or "~/.tu/metrics_repo"), p.Home)`; `Machine = expandSentinels(merged["machine"] or "$HOSTNAME")`; `User = expandSentinels(merged["user"] or "$USER")`; `AutoSync = !(merged["auto_sync"] == "false" || merged["auto_sync"] == "0")` (missing → true). `expandSentinels` replaces a value that is **exactly** `$HOSTNAME` with `env.Hostname()` and exactly `$USER` with `env.Username()` (no substring expansion); a hostname error yields `""` (the TS `os.hostname()` does not throw in practice), a username error yields `unknown` (the TS `safeUsername`).
6. Warnings are returned in the order the TS emits them: deprecation first (during selection), version second (after the merge).

The shipped `tu.default.conf`, which is also what `init-conf` copies, is reproduced here because §4 depends on its exact commented lines:

```
# Default tu configuration — fallback for all properties.
# User overrides go in ~/.config/tu/tu.conf (created by 'tu init-conf'). Org-wide defaults may be dropped in ~/.config/tu/org.conf.
#
# Sentinel values:
#   $HOSTNAME  — resolved to os.hostname() at runtime
#   $USER      — resolved to os.userInfo().username at runtime

version = 2

# Git repo URL for metrics storage
# metrics_repo = git@github.com:you/tu-metrics.git

# Local path where the metrics repo is cloned
metrics_dir = ~/.tu/metrics_repo

# Label for this machine in the metrics repo
machine = $HOSTNAME

# Profile name — groups your machines in the metrics repo
user = $USER

# Auto-sync: use 'tu <cmd> --sync' to sync before fetch
auto_sync = true
```

### 4. `config` — `init-conf` (`setup.go`)

```go
// Error is a setup command's operational failure: the message is printed on
// stderr verbatim, exit 1 (command.ExitOperational). Usage errors (exit 2)
// never originate here — they are command.UsageError from Parse.
type Error struct{ Message string }
func (e *Error) Error() string

// FieldBlocks are the six scaffold blocks, in this order, byte-exact (TS FIELD_BLOCKS).
var FieldBlocks = []struct{ Key, Block string }{
    {"version",      "\n# Config schema version\nversion = 2\n"},
    {"metrics_repo", "\n# Git repo URL for metrics storage (enables multi-machine sync)\n# Set here or via TU_METRICS_REPO env var\n# metrics_repo = git@github.com:you/tu-metrics.git\n"},
    {"metrics_dir",  "\n# Optional: local path where the metrics repo is cloned (default: ~/.tu/metrics_repo)\n# metrics_dir = ~/.tu/metrics_repo\n"},
    {"machine",      "\n# Optional: label for this machine in the metrics repo (default: system hostname)\n# machine = my-macbook\n"},
    {"user",         "\n# Optional: profile name — groups your machines in the metrics repo (default: system username)\n# user = your-name\n"},
    {"auto_sync",    "\n# Auto-sync: no longer auto-triggers; use 'tu <cmd> --sync' to sync before fetch\nauto_sync = true\n"},
}

// InitConf is `tu init-conf`: returns the stdout lines; it cannot fail
// operationally except on a filesystem error, which is returned as *Error
// with the OS error text (the TS throws — an uncaught exception, exit 1).
func InitConf(p Paths) ([]string, error)
```

Behavior (TS `runInitConf` + `ensureUserConf`):

1. **ensureUserConf** (shared with `InitMetrics`): if `p.UserConf` exists → return false, no output. Else `MkdirAll(p.ConfigDir, 0o755)`; if `p.LegacyConf` exists → write `UserConf` with the legacy file's bytes and emit `Copied ~/.tu.conf → ~/.config/tu/tu.conf` (both paths via `Tildefy`); else write `UserConf` with `DefaultConf` and emit `Created ~/.config/tu/tu.conf — edit it to configure multi-machine sync.`; return true. File mode 0o644. The legacy file is never moved or deleted.
2. If the file was just created, stop (one line of output).
3. Otherwise read `UserConf`; for each `FieldBlocks` key in order classify: **present** when some line, after `TrimLeft` of whitespace, does not start with `#` and matches `^{key}\s*=`; else **mentioned** when the whole content matches `(?m)^\s*#?\s*{key}\s*=`; else **missing**.
4. No missing and no commented → emit `{dp} is already complete.` (`dp = Tildefy(p.UserConf, p.Home)`).
5. Missing non-empty → append the missing keys' `Block`s (in `FieldBlocks` order) to the file and emit `Updated {dp} — added missing fields: {keys joined by ", "}.`
6. Commented non-empty → emit `{dp} has commented-out fields that need uncommenting: {keys joined by ", "}.` (after the Updated line when both apply).

Under the harness: `single`/`org` create (defaults have every key present or commented: `metrics_repo` is commented → but the file was just created, so step 2 stops before the report); `multi` is `already complete` (the staged conf has all six keys active); `legacy` copies.

### 5. `config` — `init-metrics [repo-url]` (`setup.go`)

The arity check happens in `command.Parse` (§8), before `$HOME` is consulted. The rest:

```go
// CloneStep is what the edge must run when InitMetrics decides a clone is
// needed: `git clone <URL> <Dir>` with stdout/stderr passed through.
type CloneStep struct{ URL, Dir string }

// InitMetricsResult is the config side of `tu init-metrics [url]`.
type InitMetricsResult struct {
    Lines []string   // stdout lines produced so far, in order
    Clone *CloneStep // nil when nothing is left to do (Already initialized)
}

// InitMetrics performs everything up to (not including) the clone; git is
// consulted only to answer "is Dir a git repo" (git -C Dir rev-parse --git-dir).
func InitMetrics(p Paths, env Env, url *string, git Git) (InitMetricsResult, error)

// ClonedLine is the stdout line the edge prints after a successful clone.
func ClonedLine(url, dir string) string   // "Cloned {url} → {dir}"  — dir ABSOLUTE (DC-19)

// CloneFailedMarker is the clone-failure cooldown file under StateDir (B3 writes and reads it).
const CloneFailedMarker = ".clone-failed"

// RemoveCloneMarker deletes StateDir/.clone-failed if present; best-effort, never errors.
func RemoveCloneMarker(stateDir string)

// Git is the one git question config asks; internal/sync.Exec answers it for real.
type Git interface{ IsRepo(dir string) bool }
```

Behavior (TS `runInitMetrics`), in this order:

1. With `url != nil`: if the URL contains `\r` or `\n` → `*Error{"Error: repo-url must be a single line (no newline or carriage-return characters)."}` (exit 1). Then ensureUserConf (§4 step 1 — its `Created`/`Copied` line is the first stdout line when it fires). Then **setMetricsRepoInConf**: read `UserConf`, split on `\n`; if some line whose `TrimLeft` does not start with `#` matches `^metrics_repo\s*=` → replace that whole line with `metrics_repo = {url}` and write `strings.Join(lines, "\n")`; else if some line matches `^\s*#\s*metrics_repo\s*=` → replace it the same way; else append `FieldBlocks.metrics_repo` with its `# metrics_repo = git@github.com:you/tu-metrics.git` line replaced by `metrics_repo = {url}`. Emit `Set metrics_repo = {url} in {Tildefy(UserConf)}`. Set `ov.MetricsRepo = &url`.
2. `cfg, warnings := Load(p, env, ov)` — the edge prints `warnings` on stderr (the legacy deprecation appears here for `init-metrics` in the legacy variant, and does NOT appear for `init-metrics-url` in the legacy variant because step 1 created the new file first).
3. `cfg.MetricsRepo == ""` → `*Error{"Error: metrics_repo is not set. Add it to {dp}, run 'tu init-metrics <repo-url>', or set TU_METRICS_REPO."}` with `dp = Tildefy(p.UserConf, p.Home)`.
4. `cfg.MetricsDir` exists (any file type): `git.IsRepo(dir)` true → emit `Already initialized: {dir}` (absolute) and return with `Clone == nil`; false → `*Error{"Error: {dir} exists but is not a git repo. Remove it or set a different metrics_dir in {dp}."}`.
5. Otherwise return `Clone = &CloneStep{cfg.MetricsRepo, cfg.MetricsDir}`.

The edge (§9) prints `Lines`, then runs the clone through `sync.Exec.Clone` with its own stdout/stderr as the passthrough writers (git's `Cloning into '…'…` reaches the user's stderr exactly as the TS `stdio: "inherit"` does), then on success calls `RemoveCloneMarker(StateDir(p.Home))` and prints `ClonedLine(url, dir)`. On a non-zero clone exit the edge prints `Error: git clone failed (exit {N}).` on stderr and exits 1 — the TS lets `execSync` throw and Node prints an uncaught-exception stack trace with exit 1; that stderr is unmatchable by any port and is recorded in Open Questions as an inherent expected diff (never a harness case: the fake git always exits 0).

Under the harness the fake git answers every call with exit 0 and touches no files, so `init-metrics-url/single` prints `Cloned …` with no clone chatter and the metrics dir is never created — identical on both sides.

### 6. `config` — `status` (`status.go`)

```go
type StatusData struct {
    Mode         Mode
    NoConfig     bool     // neither user/legacy nor org file exists → the one-line form
    UserConfRead string   // "" when omitted (org-only)
    Version      int
    OrgConf      string   // "" when org.conf does not exist
    User, Machine string
    MetricsDir   string
    MetricsFound bool
    LastSync     string   // "never" or "{relative} ({ISO})"
    AutoSync     bool
    Home         string   // for Tildefy in Lines()
}

// Status is the TS runStatus: reads config (via Load) and state; returns the data
// and the cascade warnings the edge prints on stderr. now is injected.
func Status(p Paths, env Env, now time.Time) (StatusData, []string)

// Lines renders the layouts §13 block; padding is the literal 13-column label.
func (s StatusData) Lines() []string

// RelativeTime is the TS relativeTime: floor to seconds (negative → 0);
// <60 s "<1m ago"; <60 min "{m}m ago"; <24 h "{h}h ago"; else "{d}d ago".
func RelativeTime(d time.Duration) string
```

Behavior (TS `runStatus`):

1. `orgExists = fileExists(p.OrgConf)`; `selected` = the user-conf path `Load` would read (readable-test, same rule as §3 step 2). If `selected == "" && !orgExists` → `NoConfig`; `Lines()` is the single line `Mode:        single (no ~/.config/tu/tu.conf)` and nothing else. No `Load`, no warnings.
2. Otherwise `cfg, warnings := Load(p, env, Overrides{})`. `configLine` = `Config:      {Tildefy(selected)} (v{Version})` when `selected != ""` (the version is echoed even when newer than supported — the warning goes to stderr).
3. `Mode == Single` → lines: `Mode:        single`, then `configLine` if any, then `Org config:  {Tildefy(OrgConf)}` if `orgExists`.
4. `Mode == Multi` → lines: `Mode:        multi`, `User:        {User}`, `Machine:     {Machine}`, `configLine` if any, the `Org config:` line if `orgExists`, `Metrics:     {Tildefy(MetricsDir)}` when the dir exists else `Metrics:     {Tildefy(MetricsDir)} (NOT FOUND — run 'tu init-metrics')`, `Last sync:   {LastSync}`, `Auto-sync:   on` or `off`.
5. `LastSync`: read `StateDir(home)/.last-sync`; absent → `never`; trim; parse as an ISO timestamp (`time.RFC3339Nano` — the file is written by `tu sync` as JavaScript `toISOString()`, e.g. `2026-09-15T18:54:44.502Z`); unparseable or unreadable → `never`; else `{RelativeTime(now − ts)} ({raw trimmed})`.
6. `status` never clones, syncs, or writes.

The label column is exactly 13 characters (`Mode:` + 8 spaces, `Org config:` + 2 spaces, etc.) — reproduce as literal strings, not with `%-13s` (the em dash and the apostrophe in the NOT FOUND suffix are copied verbatim).

### 7. `sync` — the git driver (`git.go`)

```go
// Package sync owns the metrics-repo writer and the git driver (Target
// architecture). B1 lands only the driver; B6 adds the writer, never-shrink
// guard, dry-run report, and sync flow.
package sync

// Exec drives the real git found on PATH. It satisfies config.Git.
type Exec struct{}

// IsRepo runs `git -C dir rev-parse --git-dir` with stdout/stderr discarded
// and reports exit 0 (TS: execSync with stdio "pipe" inside try/catch).
func (Exec) IsRepo(dir string) bool

// Clone runs `git clone url dir` with the given writers attached to the child's
// stdout and stderr (TS: stdio "inherit"); a non-zero exit is returned as
// *exec.ExitError. No timeout, no GIT_TERMINAL_PROMPT — those belong to B3's
// auto-clone guard, not to the interactive init-metrics clone.
func (Exec) Clone(ctx context.Context, url, dir string, stdout, stderr io.Writer) error
```

The argv shapes are exactly the two the fake git memory lists (`rev-parse --git-dir` behind `-C <dir>`, and `clone <url> <dir>`), so the call-log sets match the TS after home normalization (§10). `git` is resolved through `PATH` (`exec.LookPath` semantics of `exec.Command("git", …)`), which is how both sides reach the fake in the harness. The test uses a fake git on PATH (a shell script or the built `fakegit`) to assert argv and exit mapping.

### 8. `command` — `Args`, the exit table, two usage errors, and the parse order

```go
// Exit codes — the shll toolkit convention (spec § Exit Codes).
const (
    ExitOK          = 0 // success, incl. benign no-ops and warn-and-continue guards
    ExitOperational = 1 // well-formed invocation that could not complete; also the placeholder
    ExitUsage       = 2 // the invocation itself was wrong
)

type Request struct {
    …
    Command string   // unchanged
    Args    []string // NEW: positionals after Command (nil for data commands); B8 reads shell-init's arg from here
}
```

`Parse` keeps every V2 behavior and adds three things, placed to reproduce the TS `main()` order (`parseGlobalFlags` → version → help → `--dry-run` guard → non-data dispatch → `parseDataArgs`):

1. **Version** (unchanged): after flag validation, any `--version`/`-V`/`-v` → `Request.Version`.
2. **Help first**: a first positional in {`help`, `-h`, `--help`} → `Command`, return (so `tu help --dry-run` prints help — the TS help check precedes the guard).
3. **`--dry-run` misuse guard** (moved here from B6's list): `Flags.DryRun && (no positional || positionals[0] != "sync")` → `UsageError{Message: "Error: --dry-run is supported only with 'tu sync' — run 'tu sync --dry-run' to preview a sync.", ShowUsage: false}`, exit 2. `tu sync --dry-run` parses to `Command == "sync"` with `DryRun` set and stays on the placeholder (B6). Harness `dry-run` and `cc-dry-run` go green.
4. **Non-data command**: first positional in the V2 set → `Command`, `Args = positionals[1:]`. Then, **only for `init-metrics`**: `len(Args) > 1` → `UsageError{Message: "Error: init-metrics takes at most one argument (repo-url)", ShowUsage: true}` (message line, then `ShortUsage`, exit 2 — the TS prints both with `console.error`). This fires before `$HOME` is consulted, matching the TS.
5. Data grammar unchanged.

`cmd/tu` replaces its literal `return 0/1/2` with the constants. `UsageError` keeps its shape (exit 2 carrier); `config.Error` is the exit-1 carrier for setup commands; `ErrUnported` stays the placeholder (exit 1).

### 9. `cmd/tu` — dispatch, the data path, write order

`run(args, stdout, stderr) int` in this order:

1. `command.Parse` — a `*UsageError` prints the message, then `ShortUsage` when `ShowUsage`; `ExitUsage`.
2. `Version` → version line, `ExitOK`.
3. `Command != ""`:
   - `init-conf`, `init-metrics`, `status` → `config.ResolvePaths(os.Getenv("HOME"))` (error → its message, `ExitOperational`); then the handler:
     - `init-conf`: `lines, err := config.InitConf(paths)`; print lines; `*config.Error` → message on stderr, `ExitOperational`.
     - `status`: `data, warnings := config.Status(paths, env, time.Now())`; print warnings on stderr, then `data.Lines()` on stdout; `ExitOK`.
     - `init-metrics`: `url := nil or &Args[0]`; `res, err := config.InitMetrics(paths, env, url, sync.Exec{})`; print `res.Lines` as they are known — `Lines` first, then on error the message, `ExitOperational`; if `res.Clone != nil` run `sync.Exec{}.Clone(ctx, URL, Dir, stdout, stderr)`; failure → `Error: git clone failed (exit N).`, `ExitOperational`; success → `config.RemoveCloneMarker(config.StateDir(paths.Home))`, print `ClonedLine`. Warnings from the embedded `Load` are returned alongside (the result carries a `Warnings []string` or the function returns them — the plan picks one shape) and are printed on stderr before the stdout lines that follow the `Load` call.
   - every other `Command` → placeholder, `ExitOperational`.
4. Data path: `ResolvePaths` (error → `ExitOperational`); `cfg, warnings := config.Load(paths, env, Overrides{})`; write `warnings` on stderr; **reserved-user guard**: `cfg.User == "all"` → `Error: config user "all" is reserved (used by -u all)` on stderr, `ExitUsage` (TS `assertUserNotReserved`, after `readConfig`; B3's metrics-dir guard slots between `Load` and this check when it lands); deps as in V2; `command.Run(ctx, req, cfg.Mode, deps)` — `Run`'s signature keeps taking `Mode` for now (B3 changes it to take `Config` when multi mode needs `MetricsDir`/`User`); `ErrUnported` → placeholder; other error → message, `ExitOperational`; `source.WriteWarnings`, lines, `ExitOK`.

`env` at the edge is `config.Env{Getenv: os.Getenv, Hostname: os.Hostname, Username: currentUsername}` where `currentUsername` wraps `user.Current()`.

Data flags on setup commands are ignored, as the TS does (DC-02): `tu status --json`, `tu init-conf --fresh`, `tu status --watch` all reach the handler because `Parse` sets `Command` regardless of flags and the edge dispatches on `Command` before looking at `Format`/`Flags`. The one exception is `--dry-run` (§8 step 3), which the TS rejects before dispatch.

### 10. Harness — home normalization in the comparison

Both sides run with different staged `$HOME`s by design (`Two fresh $HOMEs per case`), and the paths differ only in the side segment: `<tmp>/cases/<case>/node/home` vs `<tmp>/cases/<case>/go/home`. Any output that embeds the absolute home (the `Already initialized`, `Cloned`, and `exists but is not a git repo` messages; the version warning's absolute conf path; call-log argv for `-C <dir>` and `clone <url> <dir>`) therefore diverges even when both binaries are correct.

```go
// NormalizeHome replaces every occurrence of the side's staged home path in b
// with the literal "$HOME" (bytes, no regexp), so paths that legitimately embed
// $HOME compare equal across the two staged homes. Distinct from Redact
// (fixture capture), which anonymizes the developer's real home for the
// committed corpus.
func NormalizeHome(b []byte, home string) []byte
```

- `SideCapture` gains `Home string` (set by `runCase` from `sp.side.home`); `Compare` normalizes `Stdout`/`Stderr`/`TTY` of each side with its own `Home` before the byte comparison, and `firstDivergence`'s offset/line/excerpts are computed on the normalized bytes.
- The report's per-side files (`{node,go}.{stdout,stderr,tty}`) are written **normalized**, so `diff node.stdout go.stdout` under `bin/harness/report/cases/…` shows what `Compare` compared. `exit` files are unaffected.
- `CompareCallLogs` normalizes each side's `argv` entries with that side's home before forming the `{tool, argv}` set (it already ignores `cwd`). The `[calls differ]` marker then means a real sequence difference again.
- A home path that appears in a `tty` transcript is byte-continuous (the pty wraps visually, not in the byte stream), so plain replacement is sufficient.

The green/red semantics, the timeout verdict, the `harness` channel, and the `unconfirmed` marker are untouched. No matrix change: the 20 cases already exist.

### 11. Tests

- `internal/config`: table-driven `Load` tests over temp HOMEs covering every cascade layer and override rule (defaults-only, org-only, user-only, org+user precedence, legacy fallback with the deprecation warning, `tu.conf` present silences legacy, unreadable `tu.conf` falls back, `TU_METRICS_REPO` empty vs set, override beats env with an empty string too, version `"2abc"`/`"x"`/`"9"` with the warning's source attribution for user, org, and defaults, `auto_sync` `false`/`0`/`FALSE`/missing, sentinels exact-match only, `~` expansion); `InitConf` for create/copy/complete/missing/commented (including a conf with only `version = 2` → all five other keys missing → `Updated … added missing fields: metrics_repo, metrics_dir, machine, user, auto_sync.`); `InitMetrics` for the newline check, the three `setMetricsRepoInConf` branches (active line, commented line, append) with the resulting file bytes asserted, unset repo, dir-not-a-repo, already-initialized, clone-needed, and override-beats-env; `Status` for every layouts §13 block with a fixed `now` and a temp `.last-sync` (`15m ago (2026-09-15T18:54:44.502Z)`, `never` for absent/garbage); `RelativeTime` boundaries (59 s, 60 s, 59 min, 60 min, 23 h, 24 h, negative); the defaults drift guard.
- `internal/sync`: `Exec.IsRepo` and `Exec.Clone` against a fake git on PATH asserting argv, exit mapping, and passthrough bytes.
- `internal/command`: new `parse_test.go` rows — `Args` capture, `init-metrics a b` (message + `ShowUsage`), `init-metrics a` ok, `--dry-run` / `cc --dry-run` / `--dry-run --version` (version wins) / `help --dry-run` (help wins) / `sync --dry-run` (no error), `status --json` (Command set, no error).
- `cmd/tu`: `main_test.go` placeholder list drops the three setup commands and gains the usage-error rows; `e2e_test.go`'s `TestMain` also builds `fakegit` into the PATH dir and adds byte-exact cases for `status` in each of the four staged variants (reusing `harness.StageHome`), `init-conf` create/copy/complete, `init-metrics` unset/already-initialized/clone (fake git), `init-metrics a b`, `tu --dry-run`, `tu` with a legacy conf (deprecation line then the empty table, exit 0), `tu` with `user = all` (exit 2), and `tu status` with `HOME` unset (exit 1).
- `internal/harness`: `NormalizeHome` rows (multiple occurrences, prefix inside a longer path, empty home is a no-op); `Compare` on captures that differ only by home → green; `CompareCallLogs` on argv differing only by home → not differ.

### 12. Non-goals

- No multi-mode data path, metrics reader, or auto-clone guard (B3); no `-u`; no sync or `.last-sync` writing (B6); no watch re-read semantics (B7); no help text, help-dump, shell-init, skill, update (B8).
- No spec or `src/node/` change; no change to `harness/matrix.json` or the seed tree; no `justfile`/CI change.
- No new `[DECIDE]` markers: the observed behaviors reproduced here are already documented (DC-02, DC-03, DC-19, DC-20).

## Affected Memory

- `go-port/config-and-setup`: (new) The Go port's config layer and setup commands — `internal/config` (Paths, StateDir, Tildefy, embedded defaults with drift guard, `Load` cascade semantics and its two warnings, `InitConf`, `InitMetrics` with the clone step, `Status`, `RelativeTime`, `RemoveCloneMarker`), `internal/sync/git.go` (the `Exec` driver and its two argv shapes), the setup-command dispatch in `cmd/tu`, and the harness gate status (20 setup cases green). Design Decisions: embedded defaults, setup commands return values and the clone step, git driver in `sync`.
- `go-port/command-edge`: (modify) `Requirement: Minimal config cascade` is superseded by the new file (replace with a pointer); `Request` gains `Args`; `Parse` gains the help-before-guard order, the `--dry-run` misuse guard, and the `init-metrics` arity error; the exit-code constants; `run()` write order gains the command dispatch, `config.Load`, cascade warnings, and the reserved-user guard; the placeholder row map drops `init-conf`/`init-metrics`/`status` and moves the `--dry-run` misuse guard from B6 to B1; the harness gate status line updates (52 → 74 green).
- `harness/differential-harness`: (modify) `Requirement: Comparison — first divergence per case` and the call-log comparison gain home normalization; the `Report` requirement notes that per-side capture files hold normalized bytes; a Design Decision records normalization over shared HOMEs.

`configuration/config-system` (TypeScript behavior) is unchanged — D4 freeze, and it remains the reconciliation source the Go memory cites. `build/toolchain` is unchanged (no recipe or lane edits).

## Impact

- **Code**: `src/go/internal/config/{config,defaults,setup,status}.go` + tests + the embedded `tu.default.conf` copy; `src/go/internal/sync/git.go` + test (new package); `src/go/internal/command/{request,parse}.go` + tests; `src/go/cmd/tu/{main,main_test,e2e_test}.go`; `src/go/internal/harness/diff.go` + test; `src/go/cmd/tudiff/run.go`. Roughly 900–1200 lines of Go including tests.
- **Behavior**: the Go binary answers three more commands and enforces two more usage errors; 22 harness cases flip green; nothing ships (formula and `dist/` untouched).
- **Dependencies**: none added; stdlib `embed`, `os/user`, `os/exec`, `context`.
- **Downstream rows**: B3 consumes `Config` (`MetricsDir`, `User`, `Machine`, `Mode`) and `CloneFailedMarker`/`StateDir`; B6 grows `internal/sync`; B8 reads `Request.Args` for `shell-init`; B7 calls `Load` per tick.
- **Risk**: the embedded-defaults choice changes what R1 must pack (nothing extra for the Go binary); recorded in Open Questions for the plan-doc note. The harness normalization touches a shared comparison path — covered by the "no regression among the 52" gate clause.

## Open Questions

None block the plan. Recorded for the gates:

- **For G1 (checklist wording)**: item 1 says "`source` is the only package that execs or reads files", while the Target architecture table assigns the git driver to `sync` and the conf-file cascade to `config`. B1 follows the table (`sync.Exec` execs git; `config` reads conf files and execs nothing). Suggest the G1 reviewer read item 1 as "no exec or file I/O outside `source`, `sync`'s git driver, `config`'s conf files, and `cmd/tu`".
- **For G0 / R3 expected-diff list**: a failed `init-metrics` clone prints `Error: git clone failed (exit N).` on the Go side where the TS prints Node's uncaught-exception stack trace (exit 1 on both). Not a harness case (the fake git never fails), inherently unmatchable; it belongs on the expected-diff list, not as a `[DECIDE]` marker.
- **For the plan doc (R1 note)**: with `tu.default.conf` embedded, the Go tarball does not need the file beside the binary; R1's "pack binary + `vendor/ccusage/bin/ccusage` + `tu.default.conf`" can drop the third item for the Go build (the TS build still needs it).

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | `Load` reproduces the TS `readConfig` cascade exactly: defaults, org, user (with legacy fallback), env `TU_METRICS_REPO` for `metrics_repo` only, CLI override beating env even when empty; mode derived from the final `metrics_repo`; `mode` key ignored | The TS source and the configuration memory state every rule and the spec pins the order with "no per-key inversion" | S:90 R:85 A:95 D:95 |
| 2 | Certain | Every stdout and stderr line of `init-conf`, `init-metrics`, `status`, the deprecation and version warnings, the reserved-user error, and the arity error is reproduced byte for byte from the node captures and layouts §13/§20 | Captured live from the harness on 2026-09-16 and cross-checked against the spec; these are Goal-frozen surfaces | S:95 R:90 A:95 D:100 |
| 3 | Certain | Data commands route through `Load`; the edge prints the cascade warnings on stderr before fetch warnings and the table, then applies the reserved-user guard (exit 2); `DetectMode` is removed | TS `main()` order is explicit (`readConfig` then `assertUserNotReserved`); the spec lists the reserved-user exit in the data-command row | S:80 R:85 A:90 D:90 |
| 4 | Certain | `StateDir` is `$HOME/.tu`; `metrics_dir` `~` expands against `$HOME`; the cache stays at `$HOME/.tu/cache`; `Tildefy` is a prefix match on the home string | TS `TU_HOME`, `resolveHome`, `tildefy` all derive from `homedir()` which is `$HOME` on POSIX; the config-home spec separates the two roots | S:80 R:90 A:95 D:90 |
| 5 | Certain | The git probe is `git -C <dir> rev-parse --git-dir` with output discarded and the clone is `git clone <url> <dir>` with passthrough writers, both resolved via PATH | These are the two argv shapes the fake-git memory lists and the node call log recorded; call-log parity depends on them | S:75 R:90 A:90 D:85 |
| 6 | Certain | `$HOSTNAME` and `$USER` expand only when the value is exactly the sentinel; `$USER` uses `os/user` `Current()` with an `unknown` fallback; `.last-sync` is parsed as RFC3339Nano; `version` uses `parseInt` leading-digit semantics | Mirrors `expandSentinels`, `safeUsername`, `formatLastSync`, and the `parseInt` call in the TS; the harness pins `machine`/`user` so the sentinel path is exercised only by unit tests | S:70 R:90 A:85 D:80 |
| 7 | Confident | The shipped defaults are embedded (`go:embed` of a byte-identical copy in `internal/config`, drift-guarded by a test) instead of reproducing the TS beside-the-binary and walk-up lookup | Removes a runtime file dependency and a silent empty-defaults failure mode; the install layout is not a Goal-listed surface; swapping back to a file lookup is a local change; R1 packaging note recorded in Open Questions | S:55 R:85 A:80 D:60 |
| 8 | Confident | `init-conf`, `init-metrics`, and `status` live in `internal/config` and return lines plus a typed `config.Error`; the git driver is a new `internal/sync/git.go`; `config` execs nothing | The Target architecture table assigns init-conf and status data to `config` and the git driver to `sync`; init-metrics is named in the B1 row without a package, and it is the config-writing half of the setup story | S:70 R:70 A:80 D:65 |
| 9 | Confident | `InitMetrics` returns a `CloneStep` for the edge to execute rather than taking writers, so the `Set metrics_repo` line reaches stdout before git's clone chatter, as in the TS | Keeps "nothing prints below `cmd/tu`" while preserving the TS interleaving on a TTY; the alternative (writers passed into `config`) would make `config` a writer | S:50 R:80 A:75 D:60 |
| 10 | Confident | The `--dry-run` misuse guard (exit 2, exact message) lands in B1's `Parse` as a row of the exit-code table; B6 keeps `tu sync --dry-run` | The B1 row names the exit-code table; the guard is a pure usage error with a known string and two harness cases; V2's memory assigned all of `--dry-run` to B6 before the table was scoped | S:60 R:90 A:85 D:70 |
| 11 | Confident | `Request` gains `Args` (positionals after a non-data command) and the `init-metrics` arity error is raised in `Parse` with `ShowUsage` | The TS checks arity at dispatch before `$HOME`; raising it in `Parse` keeps every usage error in one place and gives B8 the `shell-init` argument for free | S:65 R:85 A:85 D:75 |
| 12 | Confident | Exit codes become `command.ExitOK`, `ExitOperational`, `ExitUsage`; `UsageError` stays the exit-2 carrier, `config.Error` is the exit-1 carrier, `ErrUnported` stays exit 1 | The spec's exit-code section is the table; constants replace literals in the one writer without reshaping V2's `UsageError` and its tests | S:55 R:90 A:80 D:60 |
| 13 | Confident | The harness normalizes each side's staged home to the literal `$HOME` in stdout, stderr, tty, and call-log argv before comparison, and writes the normalized bytes to the report files | Without it three setup groups can never go green; the two-homes design decision stands (P4) so normalization, not a shared home, is the fix; report files matching what was compared keeps `diff` useful | S:60 R:85 A:85 D:65 |
| 14 | Confident | `Status` lives in `config` as a data struct with a `Lines()` renderer, not as a `view` table through `render` | The plan says "`status` data" belongs to `config`; the layout is seven fixed-label lines with no table semantics, and routing it through the table model would add a column type for one command | S:55 R:90 A:80 D:65 |
| 15 | Confident | A failed interactive clone prints `Error: git clone failed (exit N).` and exits 1 | The TS output on this path is a Node stack trace no port can match; the exit code is the contract, the message follows the `Error: …` house style; listed for the G0 expected-diff list | S:35 R:90 A:55 D:30 |
| 16 | Certain | The auto-clone guard, marker writing and reading, day-file reading, sync, and `.last-sync` writing stay out of B1; B1 exports only `CloneFailedMarker` and `RemoveCloneMarker` | The plan assigns the guard and metrics source to B3 and sync to B6; the B1 row says "clone marker" in the init-metrics context, which is the removal on success | S:85 R:90 A:90 D:90 |

16 assumptions (7 certain, 9 confident, 0 tentative, 0 unresolved).
