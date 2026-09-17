# Intake: Sync — Metrics-Repo Writer and Git Flow (Go port row B6)

**Change**: 260916-lsml-sync-metrics-writer
**Created**: 2026-09-17

## Origin

> Context: fab/plans/sahil/26-09-15-go-port.md, row B6. Read the Decisions and Target architecture sections before writing the intake. Do not change any external surface listed under Goal. Port the metrics-repo writer faithfully (D11): never-shrink guard, day-file layout, commit message, rebase-abort recovery, dry-run report on the live decision path, auto-sync TTL, repair helper parity. Harness gate: dry-run byte-diff plus live sync against a temp bare repo, then git log -p diff between TS and Go runs. Port faithfully; no redesign.

One-shot `/fab-new` invocation from the Go-port operator queue (Phase 2, after B5 merged as PR #93). The row text in the plan reads: *"D11: writer with never-shrink guard, day-file layout, commit message, rebase-abort recovery, dry-run report on the live decision path, auto-sync TTL, repair helper parity. Harness: dry-run byte-diff + live sync against a temp bare repo, then `git log -p` diff between TS and Go runs. Port faithfully; no redesign."* — size L, depends on B3 (merged as PR #90). D11 is the decision this row executes: *"Metrics-repo format and never-shrink guard are ported faithfully first, redesigned never in this plan. Covered by the harness (dry-run and live against a temp bare repo) before any surrounding code is touched."* The Deployment section names why: *"The worst field failure is a bad sync write, not a wrong table."* — a mixed TS/Go fleet co-writes the metrics repo for weeks after cutover (cutover criterion 2: identical day-files and an identical `git status --porcelain` from the same local state).

Key readings that shaped this intake (2026-09-17, worktree at `f389cbc`):

- **What is red**: `just go-diff --placeholder` is at 408/420; the 12 red cases are exactly this row's — `sync-cmd` and `sync-dry-run` on the four conf variants, `sync-flag` and `cc-sync` on `single`/`multi`. `command.inScope` rejects `Flags.Sync`, and `cmd/tu.runCommand` sends `sync` to the placeholder. `--dry-run` misuse is already a byte-exact usage error in `command.Parse` (4fs0); `tu sync --dry-run` parses to `Command == "sync"` with `DryRun` set.
- **What exists on the Go side**: `internal/sync/git.go` holds only the git driver `Exec` (`IsRepo`, `Clone`, `CloneQuiet`) — its package comment already reserves the writer, guard, dry-run report and flow for B6. `internal/source/metrics` is the read-only reader (private `dayFile{Label; fact.Totals}` — the exact TS key order). `command.gatherOwn` fetches live, then reads the repo and `MaxMerge`s — the `multi-mode` memory's Design Decision "Read-only multi mode until B6" records the deliberate gap: *"The sync row (B6) adds the never-shrink day-file writer call in the own-user path before the `Read`, completing the TS write-then-read shape — the only multi-mode seam still open."* `config.LastSync`/`StateDir`/`Tildefy` exist; `render.FixedHalfUp` is the `toFixed` twin (2gbb). The harness fake git (`cmd/fakegit`) records every argv and answers from `TUDIFF_GIT_SCRIPT` prefix rules — *"`TUDIFF_GIT_SCRIPT` is unset — the fake git is the silent exit-0 stub for every call; B6 scripts it."* The seed `harness/metrics-repo/` was built for this row: own-machine cc day-files at `0.25` and `0.75` straddle the placeholder corpus's `0.5`, so a dry-run against it shows an update, a never-shrink skip and new files in one report.
- **The TS reference** is `src/node/sync/sync.ts` (350 lines: `commitMessage`, `readShrinkState`, `writeMetrics`, `syncMetrics`, `isStale`, `touchLastSync`, `fullSync` with its `DrySyncReport`), the `formatDrySyncReport`/`runSync` block and the `--sync` branch of `main()` in `src/node/core/cli.ts` (lines 586–668, 1931–1938), the pre-fetch `writeMetrics` call in `fetchToolMerged`/`fetchToolMergedWithMachines` (lines 703, 820), and the standalone ops script `scripts/repair-metrics.mjs` (245 lines) with its test `src/node/sync/__tests__/repair-metrics.test.ts`. Contract text: `docs/specs/usage.md` § Global Flags (`--sync`, `--dry-run`), § Setup Commands (`tu sync`, `tu sync --dry-run`), § Exit Codes, § Multi-Machine Mode › Sync Flow, Dry Run, Auto-Clone Guard (item 5), Staleness; `docs/specs/layouts.md` § 20; DC-02, DC-18, DC-20, DC-21, DC-22, DC-23. Memory: `sync/multi-machine` (the frozen TS behavior, incl. the 260610-srmi data-loss history and the 260717-xuhk dry-run design), `go-port/{multi-mode,command-edge,config-and-setup,fact-and-sources}`, `harness/differential-harness`, `build/toolchain`.

## Why

**Problem.** The Go binary reads the metrics repo but never writes it, never syncs it, and answers `tu sync`, `tu sync --dry-run` and `--sync` with the placeholder. In multi mode the shipped TS writes a never-shrink-guarded day-file for every tool on **every** data command and syncs on demand; that write is the wire protocol between machines. Until this row lands, a Go binary in a fleet contributes nothing to the shared repo, and the Deployment section's worst field failure — a bad sync write — has no Go implementation to gate.

**Consequence of not doing it.** The 12 red harness cases stay red for one reason; B7 (watch) can start but R1/R2 cannot (the dogfood checklist's item (b) is *"one sync where a foreign machine's commit arrived between your pull and push"*); cutover criterion 2 (live-sync parity) is unverifiable; and after Z1 removes `src/node/` the metrics repo would have no repair helper at all.

**Why this shape.** The plan assigns everything here to one package: `sync` — *"Metrics-repo writer (never-shrink guard, day-file layout, commit message), git driver, dry-run report sharing the live decision path. Depends on `fact` + `source/metrics`, not on the ccusage adapter."* The TS design is already right for the port and D11 forbids redesign, so the Go code reproduces it function for function: one `Write` whose decision logic runs identically in live and dry-run mode with only the filesystem effects gated (toolkit principle №5 — the 260717-xuhk decision), one `commitMessage` shared by the live commit and the preview, one git round trip with the rebase-abort recovery and the single push retry, and the dry-run's git half computed locally with a read-only `git status --porcelain`. What changes is only the seam shape the Go architecture already uses everywhere: nothing below `cmd/tu` prints, so the flow **returns** its stderr lines in order and the edge writes them; the live fetch reaches `sync` through a small `Fetcher` interface the ccusage adapter satisfies (the same way `command` gets it); the writer reaches `command.gatherOwn` through a `Deps.Writer` interface (the same way the reader does).

**The gate is the row's own harness work.** The fake-git matrix proves the bytes on stdout/stderr and — new in this row — the **written tree**; a new `tudiff live` subcommand runs the real sequence (dry-run, sync, steady-state sync, foreign-commit rebase, pull failure) against two identical temp bare repos with **real git** and diffs the day-files, `git status --porcelain` and `git log -p` between the TS and Go runs; the repair twin is diffed the same way against a seeded history. The `[DECIDE]` markers touching this surface (DC-18, DC-20, DC-21, DC-22, DC-23) are gate G0's; this row reproduces, never resolves — and records three places where the spec text does not match the shipped TS (§ Spec inaccuracies) for the human to fix.

## What Changes

### 1. `internal/source/metrics` — the day-file shape becomes shared

Export the reader's private type so the writer and the reader agree by construction:

```go
// DayFile is the one JSON object a {tool}-{date}.jsonl file holds, in the
// exact key order the TS toUsageEntry spread produces (label first, then the
// six totals in UsageTotals order). The writer marshals it; the reader
// unmarshals it.
type DayFile struct {
    Label string `json:"label"`
    fact.Totals
}

// Name is the day-file basename: "{tool}-{date}.jsonl".
func Name(tool fact.Tool, date string) string

// Path is the day-file path: {dir}/{user}/{year}/{machine}/{Name} where
// year is the label's first four characters (the TS label.slice(0, 4); a
// shorter label is used whole).
func Path(dir, user, machine string, tool fact.Tool, date string) string
```

`readDayFile` switches to `DayFile`; behavior unchanged. `encoding/json` on this struct emits `{"label":"2026-01-05","totalCost":0.25,"inputTokens":3000,"outputTokens":400,"cacheCreationTokens":1000,"cacheReadTokens":20000,"totalTokens":24400}` — byte-identical to the seed files and to `JSON.stringify(entry)`: same key order, integers without a fraction, and the same finite-double rules (shortest round-trip, exponent form only below `1e-6` or at `1e21` and above, `e-07` cleaned to `e-7`). No struct tag or encoder option beyond the defaults; a unit test pins the seed bytes and a handful of float shapes against Node-verified strings (`0.5`, `211.8`, `1e-7`, `1e+21`, `0.1+0.2`).

### 2. `internal/sync` — the writer and the never-shrink guard (`writer.go`)

```go
// Action is a per-file write decision.
type Action string
const (
    ActionWrite Action = "write"
    ActionSkip  Action = "skip"   // the never-shrink guard would skip this write
)

// Decision is the TS WriteDecision: produced for every record in BOTH live and
// dry-run mode, so the preview shares the exact decision path as the write.
type Decision struct {
    Path         string   // filepath.Join(dir, user, year, machine, name) — absolute when dir is
    Action       Action
    IncomingCost float64
    ExistingCost *float64 // nil unless an existing parseable file was read
}

// Write is the TS writeMetrics: one Decision per record, in record order.
// In live mode a write-decision creates the directory (0o755) and writes the
// day-file (0o644) as DayFile JSON + "\n"; in dry-run mode nothing is created
// or written. Only the filesystem effects are gated on dryRun. A filesystem
// error stops the walk and is returned (the TS throws — an uncaught crash;
// the edge prints it, exit 1; recorded as an unmatchable path, never a case).
func Write(dir, user, machine string, tool fact.Tool, recs []fact.Record, dryRun bool) ([]Decision, error)
```

`shrinkState(path string, incoming float64) (shrinking bool, existing *float64)` is the TS `readShrinkState`, one file read: a read error → `(false, nil)`; `strings.TrimSpace` empty → `(false, nil)`; JSON that does not decode → `(false, nil)`; otherwise the value at key `totalCost` goes through the JS `Number()` coercion the TS applies (`Number(existing?.totalCost)`): a JSON number → itself; `null` → `0`; `true`/`false` → `1`/`0`; a string → trimmed, empty → `0`, else `strconv.ParseFloat` (failure → NaN); a missing key, an array, an object, or a top-level non-object document → NaN. A non-finite result → `(false, nil)` (treated as absent). Finite → `(incoming < existingCost, &existingCost)`: strictly lower skips, equal or greater writes (today's file keeps refreshing). The record's `Date` is the label; `Totals.TotalCost` is the incoming cost. No stderr, ever — the skip is silent.

Unit tests port `sync.test.ts` § `writeMetrics` and § `writeMetrics (dry-run)` one for one: path layout across two years, one file per record, overwrite on equal cost, skip on lower, silent skip, absent/empty/unparseable/no-numeric-`totalCost` files treated as absent, per-record guarding within a batch, live-mode decisions, dry-run decisions with `ExistingCost` present only when read, dry-run creates nothing.

### 3. `internal/sync` — commit message, `.last-sync`, staleness (`state.go`)

```go
// CommitMessage is "# {user}: update {UTC date}" — the ONE place the live
// commit and the dry-run preview derive it from (DC-21: UTC, which can trail
// the day-files' local date).
func CommitMessage(user string, now time.Time) string   // now.UTC().Format("2006-01-02")

const LastSyncFile = ".last-sync"

// TouchLastSync writes {stateDir}/.last-sync as JS toISOString() + "\n"
// ("2006-01-02T15:04:05.000Z" in UTC) — the format config.LastSync parses.
// The state dir is created when missing (see Assumptions).
func TouchLastSync(stateDir string, now time.Time) error

// Stale is the TS isStale: true when .last-sync is missing, unreadable, does
// not parse as RFC 3339 after trimming, or is older than 3 h. It has no caller
// in the TS (DC-20) and none here; it is ported because the plan row names the
// auto-sync TTL and gate G0 may decide to give it one.
func Stale(stateDir string, now time.Time) bool
```

The 3 h constant is the same value `config.MetricsDirGuard` uses for the clone cooldown; it is declared once in `sync` (`const StaleAfter = 3 * time.Hour`) and `config` keeps its own unexported `cloneRetryWindow` — two named constants for two documented rules is fine; do not couple `config` to `sync`.

### 4. `internal/sync` — `Exec.Run` and the git flow (`flow.go`)

**Driver.** `Exec` gains the general call the TS `git` closure makes:

```go
// Runner is the one git verb the sync flow needs; Exec satisfies it, tests
// use a fake or the PATH-first shell-script git (git_test.go's pattern).
type Runner interface {
    Run(dir string, args ...string) (stdout string, err error)
}

// Run executes `git -C <dir> <args...>` with no timeout (the TS has none),
// stdout and stderr captured. On failure the error's text reproduces the TS
// execFileAsync wrapper exactly: "{summary}... failed: {message}" where
// summary = "git -C <dir>" (the binary plus the first two args) and message
// is Node's — "Command failed: git -C <dir> <args joined by single spaces>\n<stderr>"
// for a non-zero exit, "spawn git ENOENT" when git is not on PATH.
func (Exec) Run(dir string, args ...string) (string, error)
```

**Round trip.** `SyncMetrics(dir, user string, now time.Time, git Runner) (ok bool, lines []string)` is the TS `syncMetrics`; `lines` are the stderr lines it would have printed, in order, for the edge to write:

1. **Rebase recovery**: if `{dir}/.git/rebase-merge` or `{dir}/.git/rebase-apply` exists (`os.Stat`, exactly the TS `existsSync` — a worktree-style `.git` file is not special-cased, as in the TS) → append `Warning: recovering from interrupted rebase`, run `rebase --abort`, ignore its error.
2. **Stage and commit**: if `{dir}/{user}` exists → `add {user}/` (the TS skips staging a missing user dir so `pathspec did not match` cannot fail a first run); then `status --porcelain {user}/` **unconditionally**; if its trimmed output is non-empty → `commit -m {CommitMessage(user, now)}`. Any error in this block → return `false` with no line (DC-18: a commit failure prints only the edge's generic error).
3. **Pull**: `pull --rebase origin main` (the branch name is fixed, DC-18). On error → append `Warning: sync pull failed — {err.Error()}`, run `rebase --abort` (error ignored), return `false`.
4. **Push**: `push`; on error `push` once more; on the second error → append `Warning: sync push failed after retry — {err.Error()}`, return `false`.
5. Return `true`.

**Full sync.** One function, one decision path, mirroring the TS overload:

```go
// Fetcher is the live-fetch seam: *ccusage.Source satisfies it (asserted at
// the edge). sync never imports the adapter (target architecture).
type Fetcher interface {
    FetchAll(ctx context.Context, period string, extraArgs []string, fresh bool) ([]fact.Record, []*source.Error)
}

type Inputs struct {
    Config   config.Config // post-guard: MetricsDir, User, Machine
    StateDir string        // config.StateDir(home) — .last-sync lives here
    Now      time.Time
    Source   Fetcher
    Git      Runner
}

type Outcome struct {
    OK       bool            // live: SyncMetrics' result; dry-run: always true
    Report   *Report         // dry-run only
    Warnings []*source.Error // the fetch's per-source errors, registry order — the edge writes them with source.WriteWarnings BEFORE Lines
    Lines    []string        // SyncMetrics' stderr lines, in order (nil in dry-run)
}

// FullSync is the TS fullSync. Both modes: FetchAll(PeriodDaily, nil, fresh=false)
// — the cached daily fetch the TS fetchHistory makes — then per tool in
// registry order Write(dir, user, machine, tool, recs, dryRun).
// Dry-run: collect the decisions into Report.Tools; WouldCommit = any
// write-decision OR a read-only `status --porcelain {user}/` (run
// unconditionally, any error = not dirty) is non-empty; CommitMessage from
// the shared helper; no SyncMetrics, no TouchLastSync, no mkdir, no write.
// Live: Write, SyncMetrics, and on ok TouchLastSync(StateDir, Now).
func FullSync(ctx context.Context, in Inputs, dryRun bool) (Outcome, error)
```

The fetch runs under `source.DefaultTimeout` applied by the caller's context (the edge), like the data path. The TS fetches with `Promise.all` and the TS fetch failure line (`warning: {Tool} fetch failed ({message}), showing zero data`) is printed at failure time; the Go `FetchAll` collects typed errors in registry order and the edge writes them via `source.WriteWarnings` — before the git lines, exactly where they fall in the TS (the fetch precedes every git call). Records are grouped by `Tool` into registry order for the per-tool `Write` (the TS iterates `Object.keys(TOOLS)`).

Tests: a fake `Runner` recording argv and scripting failures (rebase-recovery line and `rebase --abort` call, missing user dir skips `add`, dirty status commits with the exact message, pull failure's line text and the follow-up abort, single push retry, both-pushes-fail line, the no-line commit failure); plus **real-git** tests against a `git init --bare --initial-branch=main` fixture (the TS `sync.test.ts` § `syncMetrics` cases: push on success, clean tree no-op, non-git dir → false, upstream change integrated via pull before push, interrupted-rebase directory recovery, a `metricsDir` with spaces) with identity and `commit.gpgsign=false` pinned through `GIT_CONFIG_*` env, `GIT_CONFIG_GLOBAL=/dev/null`, `GIT_CONFIG_NOSYSTEM=1`.

### 5. `internal/sync` — the dry-run report and its formatter (`report.go`)

```go
type ToolReport struct {
    Tool      fact.Tool
    Decisions []Decision
}

// Report is the TS DrySyncReport.
type Report struct {
    MetricsDir    string
    User          string
    Machine       string
    Tools         []ToolReport // registry order
    WouldCommit   bool
    CommitMessage string
}

// Format is the TS formatDrySyncReport, returned as stdout lines (the edge
// joins with "\n" via Fprintln). home tildefies the user prefix.
func (r Report) Format(home string) []string
```

Byte-exact rules, from the TS:

- `userPrefix = filepath.Join(MetricsDir, User)`; `dir = config.Tildefy(userPrefix, home) + "/"`.
- Per decision, `name` = `Path` with `userPrefix + "/"` stripped when `Path` has the prefix, else `Path` unchanged; `fmt(x)` = `"$" + render.FixedHalfUp(x, 2)` — the `toFixed(2)` twin; **no thousands separators in either block** (the TS uses the same `fmt` for both; see § Spec inaccuracies re DC-22).
- Writes: `  {name}  {fmt(incoming)}  (update: {fmt(existing)} → {fmt(incoming)})` when `ExistingCost != nil`, else `  {name}  {fmt(incoming)}  (new)`. Skips: `  {name}  incoming {fmt(incoming)} < existing {fmt(existing or 0)}`.
- Lines: `Would write {N} day-file(s) under {dir}:` followed by the write lines when `N > 0`, else the single line `Would write 0 day-file(s) under {dir}.`; then, only when `K > 0`, `Would skip {K} file(s) (never-shrink guard):` and the skip lines; then `Would commit: "{CommitMessage}", then pull --rebase origin main, then push` when `WouldCommit`, else `Would commit: nothing (no changes), then pull --rebase origin main, then push`; then `Dry run — nothing written, committed, or pushed.`

Against the seed plus the placeholder corpus on the `multi` home, the cc lines are `  2026/harness-machine/cc-2026-01-05.jsonl  $0.50  (update: $0.25 → $0.50)`, `  2026/harness-machine/cc-2026-01-07.jsonl  $0.50  (new)` and, in the skip block, `  2026/harness-machine/cc-2026-01-06.jsonl  incoming $0.50 < existing $0.75`; the other five tools contribute `(new)` lines for 2026-01-05..07 in registry order. The exact bytes are captured from `node dist/tu.mjs` at apply time (the B5 precedent) and pinned in the e2e test.

### 6. `internal/command` — the writer seam and `--sync` in scope

```go
// Writer is what command needs from the metrics-repo writer: the own-user
// day-file write that precedes every repo read in multi mode. sync.Writer
// satisfies it (asserted where cmd/tu assigns it). Nil = no write (tests).
type Writer interface {
    Write(user, machine string, tool fact.Tool, recs []fact.Record) error
}
// Deps gains: Writer Writer
```

`internal/sync` provides the adapter `type Writer struct{ Dir string }` whose `Write` calls the package `Write(..., dryRun=false)` and drops the decisions.

`gatherOwn` becomes the TS write-then-read shape: after `fetchLive`, **per tool in registry order**, `deps.Writer.Write(cfg.User, cfg.Machine, tool, byTool[tool.Key])` (when `Writer != nil`), **then** `deps.Repo.Read(cfg.User, tool)`, then the split and `MaxMerge` unchanged. A write error propagates out of `gather` and `Run` (the TS crash path; the edge prints it, exit 1). The rendered bytes cannot change — a stored file only ever holds a value the guard let through, and the guard lets through exactly the values that win the max — which is what made the read-only interim safe; the tree comparison (§ 9) is what now observes the write. The repo-only paths (`-u <other>`, `-u all`, both leaderboards) never write, as in the TS.

`inScope`: `Flags.Sync` is admitted — the edge consumes it before `Run` (§ 7). `Watch`, `DryRun` (unreachable here: Parse rejects it off `sync`, and `sync` is a `Command`), `NoRain`, `SkipBrewUpdate` stay out (B7). `Normalize` is untouched: `--sync` has no guard and emits no notice.

### 7. `cmd/tu` — the two entry points

**`--sync` on a data command** (TS `main()` lines 1931–1938, which sit after the `-u` single-mode notice and the leaderboard gate and before the `--by-machine`/`--since`/`--full`/`--top` notices — no ordering conflict arises, because the `-u` notice fires only in single mode and the sync only in multi mode, and the leaderboard gate is `Run`'s first statement). In `run`, after the reserved-user guard and the deps assignment, before `command.Run`:

```go
if req.Flags.Sync && cfg.Mode == config.Multi {
    fmt.Fprint(stderr, "syncing metrics... ")          // no newline
    out, err := metricsync.FullSync(ctx, inputs, false) // the same src (cache-sharing) and Exec{}
    // err = a filesystem failure: print err.Error(), exit 1 (unmatchable TS crash)
    source.WriteWarnings(stderr, out.Warnings)
    writeLines(stderr, out.Lines)
    if out.OK { fmt.Fprintln(stderr, "synced.") } else { fmt.Fprintln(stderr, "sync failed — using local data.") }
}
```

then `command.Run` proceeds (its fetch hits the cache the sync just warmed — the TS makes six ccusage calls for `tu --sync` on a cold cache, not twelve, and the harness call multiset must match), and its own-user write rewrites byte-identical files. The `ctx` for the sync carries `source.DefaultTimeout` like the data path's.

**`tu sync`** — a new `runCommand` case, the TS `runSync` order exactly:

1. `config.ResolvePaths` — `ErrNoHome` → its message, exit 1.
2. `config.Load` — warnings to stderr.
3. Reserved user: `cfg.User == "all"` → `Error: config user "all" is reserved (used by -u all)`, exit 2 (**before** the mode check, as in `runSync`).
4. `cfg.Mode != Multi` → stderr `tu sync requires metrics_repo to be set.` and `Add metrics_repo to ~/.config/tu/tu.conf, run 'tu init-metrics <repo-url>', or set TU_METRICS_REPO.` (two lines, the literal `~/.config/tu/tu.conf`), exit 1.
5. `config.MetricsDirGuard(cfg, StateDir, now, Exec{})` — its lines to stderr; a demoted config → exit 1 with nothing more (spec § Auto-Clone Guard item 5). So `tu sync --dry-run` with a missing dir still clones first, as the spec says.
6. Build `store, _ := cache.Default()`, `src := &ccusage.Source{Cache: store, User: cfg.User, Machine: cfg.Machine}` (identical to the data path's deps), `inputs := Inputs{Config: cfg, StateDir: config.StateDir(paths.Home), Now: time.Now(), Source: src, Git: Exec{}}`.
7. `req.Flags.DryRun` → `out, _ := FullSync(ctx, inputs, true)`; `source.WriteWarnings(stderr, out.Warnings)`; each line of `out.Report.Format(paths.Home)` to stdout; exit 0.
8. Live → `out, err := FullSync(ctx, inputs, false)`; warnings, then `out.Lines` to stderr; `!out.OK` → `Error: sync failed — check network and remote config.` on stderr, exit 1; else stdout `Synced to {config.Tildefy(cfg.MetricsDir, paths.Home)}`, exit 0.

Data flags on `sync` are ignored (DC-02) — `Parse` already sets `Command` regardless. `main.go`'s package comment and the row map sentence move `sync` from the placeholder list to the answered-for-real list; only `--watch` and its companions remain unported.

### 8. The repair twin — `internal/sync/repair.go` + `cmd/turepair`

`scripts/repair-metrics.mjs` is a standalone `node` ops script (not bundled, not in the `tu` grammar — Constitution III kept it out of `dist/tu.mjs`). Z1 deletes node, so its successor is a Go maintainer binary of the same posture: not shipped, run from a checkout. `src/go/cmd/turepair/main.go` (`bin/turepair` from `just go-build`) wraps `sync.Repair`:

```go
type RepairOptions struct {
    Repo  string // default ExpandHome("~/.tu/metrics_repo"); --repo <path>
    Write bool   // --write; default dry-run
}
// Repair runs the mjs algorithm and returns its stdout lines and exit code;
// stderr lines are the mjs `fail` messages ("repair-metrics: {msg}").
func Repair(o RepairOptions, git Runner) (stdout []string, stderr []string, exit int)
```

Byte-exact reproduction of the script: arg parsing (`--repo` needs a value, unknown arg → `repair-metrics: unknown argument: {arg}` + the `Usage: node scripts/repair-metrics.mjs [--repo <path>] [--write]` line — kept **verbatim**, node spelling included, so the two outputs diff clean; the usage text is a follow-up for after Z1), `repo not found: {repo}` / `not a git repository: {repo}` (via `rev-parse --is-inside-work-tree`), `git ls-files -z -- *.jsonl` filtered by the day-file regexp, ONE `git log --format=%H%x09%cs --name-only -- *.jsonl` walk building per-file newest-first commit lists, `git show {sha}:{path}` per commit (deleted paths and unparseable blobs skipped), the first-line JSON parse of `totalCost` with the same finite check, working-tree cost with missing/unparseable = 0, `CENT_TOLERANCE = 0.01`, the report (`repair-metrics: scanned {repo}`, the `{n} tracked day-files, {m} commits touching *.jsonl` line, the `FILE/CURRENT/MAX/DELTA/MAX COMMIT` table with `padEnd`/`padStart` widths and `money = "$" + FixedHalfUp(v, 2)`, per-user totals, the grand total, `Nothing to repair — every day-file is at its historical maximum.`), the `Dry run — nothing modified. Re-run with --write to restore shrunk files.` tail, and under `--write` the full historical-max blob restored byte-exact into the working tree only, then `Restored {n} file(s) in the working tree.`, `Review with: git -C {repo} diff`, `Then commit and push manually.`. Every git call carries `-c core.quotePath=false` as the script does (a `Runner` call with those two leading args; `Exec.Run` passes them through).

Two ordering details are the only non-trivial ports: the shrunk list is sorted with `a.path.localeCompare(b.path)` — ICU root collation, **not** byte order (observable in the real repo: `sahil/2026/Sahils-Mac-mini.local/…` sorts after `sahil/2026/dev-ws-sahil02/…` under ICU and before it in byte order) — reproduced by a small comparator for the ASCII repertoire day-file paths contain (`/`, `-`, `.`, `_`, digits, letters): primary order punctuation (`_` < `-` < `.` < `/`) < digits < letters case-insensitively, then the tertiary lowercase-before-uppercase tie-break; any other byte falls back to byte order (documented limit). The per-user block sorts `[...byUser.entries()].sort()` — the default JS sort on `[user, obj]` arrays, i.e. the string `"{user},[object Object]"` compared by UTF-16 code units — byte order of the user names. Both are pinned by a Node-verified table.

`repair_test.go` ports `repair-metrics.test.ts` against a seeded temp repo (a file that shrank, one that did not, one whose historical blob is unparseable, one deleted at a later commit; dry-run bytes, `--write` restores the exact blob, idempotent re-run, missing repo and non-git repo failures, unknown argument).

### 9. Harness — `tudiff run` compares the written tree; the fake git gets scripted

**Tree comparison.** After the stdout/stderr/exit comparison, `runCase` compares the two sides' `<home>/.tu/metrics_repo/` trees — the relative path set and every file's bytes (day-file content carries no home path, so no normalization) — and the **presence** of `<home>/.tu/.last-sync` on each side (its content is a wall-clock timestamp). A difference is a **red** case with `Channel: "tree"`, the first differing relative path in the excerpt fields, reported like any other channel; a case red on bytes stays reported on its first differing channel. Nothing under `.tu/cache/` is compared (the two cache formats differ by design). This is the matrix half of the row's gate: every multi-mode data case now also proves the write.

**Scripting.** The `env` axis gains three values that set `TUDIFF_GIT_SCRIPT` (it is an environment variable — the axis's meaning; no new axis, so every existing case ID is unchanged):

| `env` | `TUDIFF_GIT_SCRIPT` | Exercises |
|-------|--------------------|-----------|
| `pullfail` | `[{"match":["pull"],"stderr":"fatal: couldn't find remote ref main\n","exit":1}]` | the pull-failure line with Node's error text, the follow-up `rebase --abort` call, exit 1 / `sync failed — using local data.` |
| `pushfail` | `[{"match":["push"],"stderr":"error: failed to push some refs\n","exit":1}]` | the single retry (two `push` calls) and the post-retry warning line |
| `dirty` | `[{"match":["status","--porcelain"],"stdout":" M harness-user/x\n","exit":0}]` | the commit path and its `commit -m "# harness-user: update {UTC date}"` argv |

`BuildEnv` appends the variable for those values; `LoadMatrix` accepts them; `harness.TZName`/`EnvName` tables and the report's axis columns follow.

**Matrix groups** (additive; `TestCommittedMatrix`'s 200..600 rail holds — 420 today):

```json
{ "id": "sync-cmd", "args": ["sync"], "conf": ["single", "multi", "org", "legacy"], "env": ["default", "pullfail", "pushfail", "dirty"] },
{ "id": "sync-dry-run", "args": ["sync", "--dry-run"], "conf": ["single", "multi", "org", "legacy"], "tz": ["fixed", "alt"] },
{ "id": "sync-flag", "args": ["--sync"], "conf": ["single", "multi"], "env": ["default", "pullfail"] },
{ "id": "cc-sync", "args": ["cc", "--sync"], "conf": ["single", "multi"], "env": ["default", "pullfail"] },
{ "id": "cc-sync-json", "args": ["cc", "--sync", "--json"], "conf": ["multi"] },
{ "id": "sync-json", "args": ["sync", "--json"], "conf": ["multi"] }
```

(+18 cases → 438.) `tz: alt` on the dry-run is the DC-21 byte check (a UTC commit date beside local day-file dates); `sync-json` is DC-02 (a real sync, plain-text result). Under the stub git every multi-mode `sync` writes the placeholder day-files into the staged repo, records `add harness-user/`, `status --porcelain harness-user/`, `pull --rebase origin main`, `push`, prints `Synced to ~/.tu/metrics_repo` and creates `.last-sync` on both sides. The gate: `just go-diff --placeholder` fully green — 438/438 with no `tree` red and no `[calls differ]` on the sync groups.

### 10. Harness — `tudiff live`: real git against temp bare repos

A new subcommand (`src/go/cmd/tudiff/live.go`, `just go-live` depending on `build go-build harness-build`) — the second half of the row's gate and the executable form of cutover criterion 2:

1. **Preflight**: real `git` on `PATH`, `node`, `dist/tu.mjs`, `bin/tu`, `bin/turepair`, the fake `ccusage` in `bin/harness/` (reusing `run`'s preflight helpers). The child `PATH` is `<tmp>/livebin` (holding only a copy of the fake `ccusage`) followed by the process `PATH` — the fake **git** is deliberately absent. Every child gets a pinned identity (`GIT_AUTHOR_NAME/EMAIL`, `GIT_COMMITTER_NAME/EMAIL`, `GIT_CONFIG_GLOBAL=/dev/null`, `GIT_CONFIG_NOSYSTEM=1`, `commit.gpgsign=false` via `GIT_CONFIG_COUNT/KEY_0/VALUE_0`) and **fixed** `GIT_AUTHOR_DATE`/`GIT_COMMITTER_DATE`, so identical trees yield identical commit hashes.
2. **Seed**: one bare repo `git init --bare --initial-branch=main`; a scratch clone receives `harness/metrics-repo/` (minus nothing — `docs/README.md` included), one `seed` commit, push. The bare is then copied to `<tmp>/node.git` and `<tmp>/go.git` — two identical remotes, never shared. Each side's HOME is staged as the `multi` variant by `harness.StageHome` and its `.tu/metrics_repo` is then **replaced** by `git clone <side>.git` (a real clone on `main`).
3. **Sequence**, run on both sides with `TZ=UTC` and the same midnight-rollover re-run rule as `runCase`; each step captures stdout/stderr/exit and is compared with `harness.Compare` after home normalization:
   - `sync --dry-run` → the report; then assert both trees unchanged (`git status --porcelain` empty on both) — the no-mutation guarantee;
   - `sync` → `Synced to …`; then compare the day-file trees byte for byte, `git status --porcelain` (both empty), `git log --format=%H%n%s -p main` (identical bytes — hashes included, given the fixed dates), and `.last-sync` presence;
   - `sync` again → the steady-state no-commit path (equal-cost rewrite leaves the tree clean; DC-23's other half); `git log` unchanged on both;
   - a **foreign commit**: a third clone of each bare adds `other-user/2026/laptop/cc-2026-01-08.jsonl`, commits (fixed date), pushes; then `sync` on each side after staging a fresh local day-file (touch a placeholder-visible change by pointing `TUDIFF_FIXTURES` at a one-off alias whose cc `daily.json` carries a higher 2026-01-07 cost) → `pull --rebase` integrates; `git log -p` identical;
   - an **interrupted rebase**: `mkdir .git/rebase-merge` in both clones, `sync` → the `Warning: recovering from interrupted rebase` line, `rebase --abort` failing silently, sync proceeding;
   - a **pull failure**: `git remote set-url origin <tmp>/missing.git` in both clones, `sync` → `Warning: sync pull failed — git -C $HOME/.tu/metrics_repo... failed: Command failed: git -C $HOME/.tu/metrics_repo pull --rebase origin main\n{real git's stderr}` then `Error: sync failed — check network and remote config.`, exit 1 — same git on both sides, identical text after home normalization;
   - `cc --sync` → `syncing metrics... synced.` on stderr and the table.
4. **Repair parity**: seed a temp history (the § 8 fixture) in one repo, copy it twice, run `node scripts/repair-metrics.mjs --repo <A>` and `bin/turepair --repo <B>`; compare stdout/stderr/exit after replacing each side's repo path with `$REPO`; then both with `--write`; compare the trees and the output again.
5. **Report** to `bin/harness/report-live/` in the `run` report's shape (per-step lines, first divergence, summary); exit 1 on any red. CI: the informational `go-diff` job gains a second `continue-on-error` step `just go-live` (real git and node are on the runner); it joins `ci-gate` with the rest at R3.

### 11. Tests

- `internal/sync`: `writer_test.go`, `state_test.go`, `flow_test.go` (fake `Runner` + real-git bare-repo fixture), `report_test.go` (the § 5 lines incl. the `Would write 0 …` and no-skip-block shapes, a relative `metrics_dir`), `repair_test.go`, `localecmp_test.go` (Node-verified table), `git_test.go` gains `Run`'s error-text cases (non-zero exit with and without stderr, missing binary).
- `internal/source/metrics`: `DayFile` marshal bytes against the seed; float shapes.
- `internal/command/run_test.go`: a fake `Writer` asserting the own-user path writes per tool **before** the read (order recorded), `-u other`/`-u all`/`lb` never write, nil `Writer` is a no-op, a `Writer` error surfaces from `Run`; `inScope` admits `Sync`.
- `cmd/tu/e2e_test.go`: flip the `sync` placeholder rows into real assertions with the fake git (`TestMain` already builds it) — `sync` on the single home (the two-line error, exit 1), on the multi/org/legacy homes (`Synced to ~/.tu/metrics_repo`, exit 0, `.last-sync` created, the day-files written — `cc-2026-01-05.jsonl` now `0.5`, `cc-2026-01-06.jsonl` still `0.75`, `cc-2026-01-07.jsonl` new, the git argv sequence from `TUDIFF_CALL_LOG`); `sync --dry-run` on multi (byte-exact report, no file created, no `.last-sync`); `sync` with `user = all` (exit 2); a fresh `.clone-failed` marker (`not available` line, exit 1, no git call); `TUDIFF_GIT_SCRIPT` pull/push failures (exact stderr, exit 1); `cc --sync` (`syncing metrics... synced.\n` then the table; six ccusage calls total); `--sync` on the single home (no sync line); `sync --json` (DC-02); a multi-home `cc h --since … --until …` now leaves the written day-files behind and renders the same bytes as before.
- `cmd/tudiff`: `live_test.go` covers the seeding helpers and the log/tree comparators with a real temp git (skipping nothing — git is a hard requirement of the subcommand, as node is of `run`).

### 12. Build

`justfile`: `go-build` also writes `bin/turepair`; new `go-live *ARGS: build go-build harness-build` → `bin/harness/tudiff live {{ARGS}}`. `ci.yml`: the `go-diff` job's second informational step (§ 10). No new module dependency; `scripts/repair-metrics.mjs` stays until Z1 (D4/D10).

### Spec inaccuracies recorded for gate G0 (not applied — D4 freezes the TS, the specs are human-curated)

1. `docs/specs/usage.md` § Sync Flow and `layouts.md` § 20 say `--sync` prints `syncing metrics... ` and "then nothing more" on success; the shipped TS prints `syncing metrics... synced.` (cli.ts line 1935). The harness enforces the code.
2. DC-18 / § Sync Flow say a push failure prints only the generic error; the TS prints `Warning: sync push failed after retry — {reason}` before it (sync.ts line 143). Only the **commit** failure is silent.
3. DC-22 says the `Would write` lines carry thousands separators; both blocks use the same `toFixed(2)` formatter — neither does.
4. `sync/multi-machine` memory says the dry-run's `git status` is "guarded by `existsSync` of the user dir"; the code runs it unconditionally (the TS comment explains why: a tracked-but-deleted user dir must still count as dirty). TS-facing memory is out of this row's hydrate scope; noted for a docs follow-up.

### Out of scope (owned elsewhere)

- `--watch`/`-w`, `--interval`, `--no-rain`, the watch loop calling `command.Run` per tick (its per-tick own-user write falls out of § 6) — B7.
- Any behavior change: the fixed `origin main` branch, the UTC commit date, the over-predicting `wouldCommit`, the caller-less `Stale`, `auto_sync` — all G0 decisions, reproduced as-is (D4, D11).
- Resolving the repair usage text's `node scripts/…` spelling — after Z1.
- Real-machine capture runs, the R2 dogfood checklist item (b) — human rows.
- Constitution: Principles I, II (a failed sync degrades to local data; a missing source is a warning, never a crash — the one TS crash path, a filesystem write failure, is reported at the edge with exit 1 instead), V (one fact type in and out of the day-files) hold; the Go Transition article is unchanged; `src/node/` untouched.

## Affected Memory

- `go-port/metrics-sync`: (new) the `sync` package as shipped — `DayFile` sharing, `Write` and the never-shrink guard incl. the `Number()` coercion table, `CommitMessage`/`TouchLastSync`/`Stale`, `Exec.Run`'s error text, `SyncMetrics`' five steps and returned lines, `FullSync`/`Inputs`/`Outcome`, `Report.Format` line rules, the `Writer` adapter, the repair twin (`Repair`, `cmd/turepair`, the ICU-root ASCII comparator, the verbatim usage line), the unmatchable crash paths, and the Design Decisions (returned lines vs. writers; `Fetcher` seam over adapter import; one `FullSync` with a `dryRun` flag; `Stale` ported caller-less; `turepair` as a maintainer binary outside the grammar; `TouchLastSync` creating the state dir).
- `go-port/command-edge`: (modify) `Deps.Writer`; `inScope` admits `Sync`; `run`'s `--sync` block and its position; `runCommand`'s `sync` case (the eight steps, exit codes); the row map (`sync` answered for real; only watch unported); the e2e inventory; the harness gate status (438/438).
- `go-port/multi-mode`: (modify) `gatherOwn` writes per tool before the read; the "Read-only multi mode until B6" Design Decision becomes history (superseded, with the closing row named); § Seams: none open for multi mode.
- `go-port/config-and-setup`: (modify) `sync.Exec.Run` beside `IsRepo`/`Clone`/`CloneQuiet`; `StateDir` as the `.last-sync` home for the writer.
- `go-port/fact-and-sources`: (modify) `metrics.DayFile`/`Name`/`Path` exported and shared with the writer; the reader "never writes" sentence points at `sync`.
- `harness/differential-harness`: (modify) the tree comparison channel, the `pullfail`/`pushfail`/`dirty` env values and `TUDIFF_GIT_SCRIPT` wiring, the new/extended sync groups and the 438 count, `tudiff live` (preflight, seeding, sequence, comparators, report dir), the repair parity flow.
- `build/toolchain`: (modify) `bin/turepair` from `go-build`, the `go-live` recipe, the `go-diff` job's second informational step.

TS-facing memory (`sync/multi-machine`, `cli/data-pipeline`, `configuration/config-system`) and `docs/specs/` are unchanged — the shipped TypeScript is frozen (D4) and the Goal surfaces are the contract; the four inaccuracies above are listed for the humans at G0.

## Impact

- **Code** (`src/go/`): `internal/sync/{writer.go,state.go,flow.go,report.go,repair.go,localecmp.go,git.go,*_test.go}`; `internal/source/metrics/{metrics.go,metrics_test.go}`; `internal/command/{run.go,run_test.go}`; `cmd/tu/{main.go,e2e_test.go,main_test.go}`; `cmd/turepair/main.go (new)`; `cmd/tudiff/{live.go (new),live_test.go (new),run.go,main.go}`; `internal/harness/{matrix.go,homes.go,diff.go,report.go,*_test.go}` (env values, tree channel).
- **Harness data**: `harness/matrix.json` (six groups touched/added, +18 cases). Gate: `just go-diff --placeholder` all green incl. the new `tree` channel; `just go-live` green (dry-run bytes, sync bytes, identical day-file trees, identical `git status --porcelain`, identical `git log -p`, repair parity). Run `npm ci` first in a fresh worktree; `go test ./...` is green at `f389cbc`.
- **Build/CI**: `justfile` (`go-build`, `go-live`), `.github/workflows/ci.yml` (one informational step). No new dependency.
- **Not touched**: `src/node/`, `scripts/repair-metrics.mjs`, `docs/specs/`, the formula. `--help`, `help-dump`, exit codes, `tu.conf`, the day-file layout and JSON bytes, the commit message — all reproduced, none changed.
- **Risk**: the git error text reproduction (Node's `Command failed: …\n{stderr}` shape — pinned by unit tests and by `tudiff live`'s pull-failure step with the same real git on both sides); the `Number()` coercion corners of the guard (unit-tested, and irrelevant to any file the writer itself produced); the `localeCompare` twin in the repair report (Node-verified table plus the repair parity run); the `tree` channel turning previously green multi-mode cases red if any tool's day-file bytes diverge (that is the point — a `--json`-style float or key-order slip becomes visible); `tudiff live`'s dependence on the runner's git version for stderr text (both sides share it).
- **Size**: L, as planned — roughly 900 lines of Go (≈350 sync + repair, ≈250 harness/live, the rest tests) plus matrix data; one PR.

## Open Questions

- None blocking.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Scope is `tu sync`, `tu sync --dry-run`, `--sync` on data commands, the own-user day-file write on every multi-mode data command, `.last-sync`, the caller-less `Stale`, and the repair twin; watch stays B7 | Plan row B6 and D11 name exactly this; the command-edge row map leaves only sync and watch unported; the Goal freezes the surfaces | S:85 R:70 A:90 D:90 |
| 2 | Certain | Everything lands in `internal/sync` beside `Exec`; `sync` imports `fact`, `source`, `source/metrics`, `config`, `render` and never `ccusage` or `command`; the live fetch arrives through a `sync.Fetcher` interface | The Target architecture assigns writer, driver and dry-run report to `sync` and forbids the adapter dependency; the config-and-setup DD "Git driver lands in sync" pre-placed it | S:80 R:75 A:90 D:85 |
| 3 | Confident | `command.Deps.Writer` is the write seam; `gatherOwn` writes per tool after the fetch and before `Repo.Read`; a nil Writer means no write in tests | The multi-mode memory names the seam and its position; mirrors the `Repo`/`Fetcher` interfaces; keeps `command` free of filesystem code | S:70 R:80 A:85 D:75 |
| 4 | Certain | `--sync` and `tu sync` are orchestrated in `cmd/tu`; `FullSync` returns its stderr lines and the fetch warnings in order and prints nothing | The only-writer rule the G1 gate checks; the TS emits these lines from inside the flow, but nothing else can interleave, so returned lines reproduce the bytes | S:75 R:80 A:90 D:80 |
| 5 | Confident | Day-file bytes are `json.Marshal` of `metrics.DayFile` plus a newline; no custom encoder | Same key order as the TS spread; Go's JSON float rules match `JSON.stringify` for finite doubles (same thresholds and shortest round-trip); the seed bytes and the new tree channel pin it | S:70 R:85 A:80 D:80 |
| 6 | Confident | The guard's existing-cost read ports the JS `Number()` coercion for null, bool, string and number, and treats missing keys, arrays, objects and top-level non-objects as absent | The TS is `Number(existing?.totalCost)` with an `isFinite` check; only hand-corrupted files reach these branches, and the writer itself always emits a number | S:60 R:90 A:80 D:70 |
| 7 | Confident | `TouchLastSync` creates the state dir when missing (the TS would crash with an uncaught error there); `Stale` is ported without a caller | Constitution II over reproducing a crash on an unmatchable path; the plan row names the auto-sync TTL and DC-20 leaves its fate to G0 | S:60 R:90 A:80 D:65 |
| 8 | Confident | The repair twin is `cmd/turepair` over `sync.Repair`, byte-identical to the mjs (usage line verbatim), with a small ICU-root comparator for ASCII paths standing in for `localeCompare` | The script is not part of the `tu` grammar (the Goal forbids growing it) and a maintainer binary matches the `tudiff` precedent; the real repo's mixed-case hostnames make the collation observable, so byte order would diverge | S:55 R:80 A:70 D:55 |
| 9 | Confident | `tudiff run` gains a red-making `tree` channel over `.tu/metrics_repo` bytes plus `.last-sync` presence; git failure paths are scripted through new `env` values rather than a new axis | The row's gate needs the written bytes compared, not only stdout; `TUDIFF_GIT_SCRIPT` is an environment variable, and reusing `env` keeps every existing case ID stable | S:65 R:85 A:75 D:65 |
| 10 | Confident | `tudiff live` runs the real-git sequence against two identical temp bare repos with pinned identity and dates, compares captures, trees, `git status --porcelain` and `git log -p`, and carries the repair parity run; `just go-live` joins the informational `go-diff` CI job | The row text and cutover criterion 2 prescribe exactly these comparisons; fixed dates make identical trees yield identical hashes so the log diff is byte-exact; the runner has git and node | S:70 R:85 A:75 D:65 |
| 11 | Certain | Three spec sentences and one TS-memory sentence that contradict the shipped code are recorded in memory for G0 and not applied | D4 freezes the TS; the specs are human-curated; the harness byte-diffs the code; the fact-and-sources precedent handled the caching sentence the same way | S:80 R:90 A:90 D:90 |
| 12 | Certain | Change type is `feat`, pinned explicitly | Sibling rows shipped as `feat`; the quoted Goal's "redesigned" would re-infer `refactor` | S:80 R:95 A:95 D:95 |
| 13 | Certain | The slug is `sync-metrics-writer` (the row name alone fails the two-word rule) and the ID `lsml` drives every fab command; the `b6-sync` placeholder branch is renamed to the change name | Substring resolution on `sync` alone would also match the TS change `260717-xuhk-sync-dry-run`; the placeholder branch belongs to no change | S:75 R:90 A:90 D:90 |
| 14 | Confident | Filesystem failures inside the writer or the `.last-sync` write return errors that the edge prints with exit 1; they are recorded as unmatchable paths and never made harness cases | The TS crashes with a Node stack trace there — the same class as the init-metrics clone failure the config row recorded | S:60 R:85 A:85 D:75 |
| 15 | Certain | Memory updates: one new go-port file plus five modified go-port/harness/build files; TS-facing memory and specs untouched | D4 and the Goal fix the surfaces; hydrate owns the go-port domain; the harness and toolchain files record their own tooling | S:70 R:90 A:85 D:85 |
| 16 | Certain | `Exec.Run` reproduces Node's error text (`git -C <dir>... failed: Command failed: git -C <dir> <args>` newline stderr; `spawn git ENOENT` for a missing binary) and only the pull and post-retry push warnings surface it | The text is a byte surface on the pull-failure path (layouts § 20); the config row already reproduced the same Node message shape for the clone guard | S:70 R:85 A:85 D:80 |

16 assumptions (8 certain, 8 confident, 0 tentative, 0 unresolved).
