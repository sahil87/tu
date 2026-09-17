---
type: memory
description: The Go port's metrics-repo writer and git flow — internal/sync (Write + the never-shrink guard and its Number() coercion table, the shared metrics.DayFile encoding, CommitMessage/TouchLastSync/Stale, Exec.Run's Node-shaped error text, SyncMetrics' add/commit/pull/push round trip, FullSync live/dry-run with Report.Format's byte rules, the command.Writer adapter) and the repair twin (sync.Repair, cmd/turepair, the ASCII ICU-root comparator); cmd/tu answers tu sync, --dry-run and --sync
---
# Metrics-Repo Writer and Sync Flow (Go port)

**Domain**: go-port

## Overview

`internal/sync` owns the metrics-repo writer, the git round trip and the dry-run report: `Write` is the never-shrink-guarded day-file writer whose decision path runs identically in live and dry-run mode, `SyncMetrics` is the add/commit/pull/push round trip, `FullSync` composes fetch → write → sync behind one function, and `sync.Repair` behind the maintainer binary `cmd/turepair` restores shrunk day-files from git history. Everything is a byte-exact port of the frozen TypeScript (`src/node/sync/sync.ts`, the `runSync`/`--sync` blocks of `src/node/core/cli.ts`, `scripts/repair-metrics.mjs`) under plan decision D11 — port faithfully, redesign never. Nothing in the package prints: flows return their stderr lines and typed warnings for the edge to write. The reader side of the day-file contract is [fact-and-sources](/go-port/fact-and-sources.md); the edge wiring (`--sync`, `runCommand`'s `sync` case) is [command-edge](/go-port/command-edge.md); the write-then-read composition is [multi-mode](/go-port/multi-mode.md); the gates are [differential-harness](/harness/differential-harness.md).

## Requirements

### Requirement: metrics.DayFile is the one day-file encoding
`internal/source/metrics` exports `DayFile{Label string \`json:"label"\`; fact.Totals}` — the one JSON object a `{tool}-{date}.jsonl` file holds, in the exact key order the TS `toUsageEntry` spread produces (label first, then the six totals in `UsageTotals` order). The writer marshals it; the reader's `readDayFile` decodes into it. `Name(tool, date)` is the basename `{tool.Key}-{date}.jsonl`; `Path(dir, user, machine, tool, date)` is `{dir}/{user}/{year}/{machine}/{Name}` where `year` is the label's first four characters (the TS `label.slice(0, 4)`; a shorter label is used whole). `json.Marshal(DayFile)` reproduces the TS `JSON.stringify(entry)` bytes with no custom encoder: same key order, integers without a fraction, and the same finite-double rules (`0.5`, `211.8`, `1e-7`, `1e+21` and `0.30000000000000004` marshal exactly as JavaScript prints them — pinned against Node-verified strings and the committed seed bytes).

### Requirement: Write runs one decision path in live and dry-run mode
`sync.Write(dir, user, machine string, tool fact.Tool, recs []fact.Record, dryRun bool) ([]Decision, error)` produces one `Decision{Path, Action, IncomingCost, ExistingCost *float64}` per record in record order, in BOTH modes — the dry-run preview shares the exact decision path as the write (toolkit principle №5). `Action` is `ActionWrite` or `ActionSkip`; `ExistingCost` is non-nil only when an existing parseable cost was read. Only the filesystem effects are gated on `dryRun`: in live mode a write decision `MkdirAll`s the file's directory (0o755) and writes the day-file (0o644) as `json.Marshal(DayFile) + "\n"`; in dry-run mode nothing is created or written. A filesystem error stops the walk and is returned (see the unmatchable-crash-paths requirement). Nothing is printed, ever — a skip is silent, as in the TS.

The never-shrink guard `shrinkState(path, incoming) (shrinking bool, existing *float64)` is the TS `readShrinkState`, one file read. Day-file snapshots are high-water marks of complete data — Claude Code purges transcripts older than ~30 days, so a live fetch for an old date collapses toward zero and must never overwrite correct history. Treated as absent (write, `existing` nil): a read error; empty/whitespace content; undecodable JSON; a top-level non-object; a missing `totalCost` key; or a coerced cost that is not finite. Otherwise the value at key `totalCost` goes through the JS `Number()` coercion the TS applies (`jsNumber`):

| JSON value at `totalCost` | Coerced cost |
|---------------------------|--------------|
| number | itself |
| `null` | `0` |
| `true` / `false` | `1` / `0` |
| string | trimmed; empty → `0`; else `strconv.ParseFloat` (failure → NaN) |
| array or object | NaN |

A finite result yields `shrinking = incoming < existing`: strictly lower skips, equal or greater writes (today's file keeps refreshing as the day grows).

#### Scenario: Guard arms across a batch
- **GIVEN** an existing `cc-2026-01-06.jsonl` costing 0.75 and incoming records costing 0.5 for 2026-01-05..07, with `cc-2026-01-05.jsonl` holding 0.25 and `cc-2026-01-07.jsonl` absent
- **WHEN** `Write` runs in either mode
- **THEN** the decisions are write (existing 0.25), skip (existing 0.75), write (existing nil); in live mode only the two writes hit disk

### Requirement: Commit message, .last-sync, staleness
`CommitMessage(user, now)` returns `# {user}: update {now.UTC().Format("2006-01-02")}` — the ONE place the live commit and the dry-run preview derive the message from (DC-21: UTC, which can trail the day-files' local date). `TouchLastSync(stateDir, now)` writes `{stateDir}/.last-sync` (`LastSyncFile`) as the JS `toISOString()` shape `"2006-01-02T15:04:05.000Z" + "\n"` — the format `config.LastSync` parses — creating the state dir when missing. `Stale(stateDir, now)` is the TS `isStale`: true when the file is missing or unreadable, when its trimmed content does not parse as RFC 3339, or when `now − ts > StaleAfter` (`3 * time.Hour`). `Stale` has no caller in the TS (DC-20) and none in the Go port; it is ported because the plan row names the auto-sync TTL and gate G0 may decide to give it one. `StaleAfter` and `config`'s unexported `cloneRetryWindow` are two named constants for two documented rules — `config` is not coupled to `sync`.

### Requirement: Exec.Run reproduces Node's error text
`sync.Runner` is `interface{ Run(dir string, args ...string) (stdout string, err error) }` — the one git verb the sync flow and the repair twin need; `Exec` satisfies it, tests use a recording fake or a PATH-first shell-script git. `Exec.Run` executes `git -C <dir> <args...>` with no timeout (the TS has none), stdout and stderr captured separately, and returns stdout on exit 0. On failure the error's `Error()` reproduces the TS `execFileAsync` wrapper byte for byte: `git -C <dir>... failed: {message}` where the message is `Command failed: git -C <dir> <args joined by single spaces>` + `"\n"` + captured stderr for a non-zero exit (the newline is unconditional — empty stderr still ends the message in `"\n"`, as Node emits), and `spawn git ENOENT` when git is not on PATH. Only the pull and post-retry push warnings surface this text; a commit failure carries it invisibly (DC-18).

### Requirement: SyncMetrics is the TS round trip, lines returned
`SyncMetrics(dir, user string, now time.Time, git Runner) (ok bool, lines []string)` prints nothing; `lines` are the stderr lines the TS would have printed, in order:

1. **Rebase recovery**: when `{dir}/.git/rebase-merge` or `{dir}/.git/rebase-apply` exists (`os.Stat`, exactly the TS `existsSync` — a worktree-style `.git` file is not special-cased) → append `Warning: recovering from interrupted rebase`, run `rebase --abort`, ignore its error.
2. **Stage and commit**: when `{dir}/{user}` exists → `add {user}/` (a missing user dir is never staged, so `pathspec did not match` cannot fail a first run); then `status --porcelain {user}/` **unconditionally** (a tracked-but-deleted user dir still reports); when its trimmed output is non-empty → `commit -m {CommitMessage(user, now)}`. Any error in this block returns `false` with no new line (DC-18: a commit failure prints only the edge's generic error).
3. **Pull**: `pull --rebase origin main` (the branch name is fixed, DC-18). On error → append `Warning: sync pull failed — {err.Error()}`, run `rebase --abort` (error ignored), return `false`.
4. **Push**: `push`; on error `push` once more; on the second error → append `Warning: sync push failed after retry — {err.Error()}`, return `false`.
5. Return `true`.

The em dash in both warning lines is U+2014, matching the TS bytes.

### Requirement: FullSync — one function, one decision path
`FullSync(ctx, in Inputs, dryRun bool) (Outcome, error)` is the TS `fullSync` overload as a single function. `Inputs{Config config.Config; StateDir string; Now time.Time; Source Fetcher; Git Runner}` carries the post-guard config (`MetricsDir`, `User`, `Machine`), the runtime-state dir where `.last-sync` lives (`config.StateDir(home)`), the clock, the fetch seam and the git driver. `Fetcher` is `interface{ FetchAll(ctx, period string, extraArgs []string, fresh bool) ([]fact.Record, []*source.Error) }`, satisfied by `*ccusage.Source` (asserted at the edge); `sync` never imports the adapter. Both modes fetch `FetchAll(ctx, source.PeriodDaily, nil, false)` — the cached daily fetch the TS `fetchHistory` makes — group the records by `Tool`, and call `Write(MetricsDir, User, Machine, tool, recs, dryRun)` per `fact.Tools` in registry order. `Outcome{OK, Report, Warnings, Lines}`: `Warnings` are the fetch's per-source errors in registry order, which the edge writes with `source.WriteWarnings` BEFORE `Lines` — exactly where the TS prints them (the fetch precedes every git call).

- **Dry-run**: collects `ToolReport{Tool, Decisions}` per tool into `Report`; `WouldCommit = anyWrite || dirty` where `dirty` is a non-empty trimmed `status --porcelain {user}/` run through `Git.Run` unconditionally (mirroring `SyncMetrics`' unconditional status), any error meaning not dirty — the dry-run never crashes on an un-synced setup; `CommitMessage` comes from the shared helper. Returns `Outcome{OK: true, Report, Warnings}` having created no directory, written no file, run no other git command, and not touched `.last-sync`.
- **Live**: after the writes, `SyncMetrics(MetricsDir, User, Now, Git)`; on ok `TouchLastSync(StateDir, Now)`. Returns `Outcome{OK, Warnings, Lines}`. A `Write` or touch error is returned as `error`.

### Requirement: Report.Format is formatDrySyncReport
`Report.Format(home string) []string` returns stdout lines (the edge `Fprintln`s them). With `userPrefix = filepath.Join(MetricsDir, User)`, `dir = config.Tildefy(userPrefix, home) + "/"`, per-decision `name` = `Path` with `userPrefix + "/"` stripped when it has that prefix, and `fmt(x) = "$" + render.FixedHalfUp(x, 2)` — the `toFixed(2)` twin, **no thousands separators in either block** (DC-22; the TS uses the same formatter for both):

- Write lines: `  {name}  {fmt(incoming)}  (update: {fmt(existing)} → {fmt(incoming)})` when `ExistingCost != nil`, else `  {name}  {fmt(incoming)}  (new)`.
- Skip lines: `  {name}  incoming {fmt(incoming)} < existing {fmt(existing or 0)}`.
- `Would write {N} day-file(s) under {dir}:` followed by the write lines when N > 0, else the single line `Would write 0 day-file(s) under {dir}.`; then, only when K > 0, `Would skip {K} file(s) (never-shrink guard):` and the skip lines; then `Would commit: "{CommitMessage}", then pull --rebase origin main, then push` when `WouldCommit`, else `Would commit: nothing (no changes), then pull --rebase origin main, then push`; then `Dry run — nothing written, committed, or pushed.`

A relative `metrics_dir` yields relative names and no tildefication; pull/push are reported as the operations that WOULD follow, never executed or probed.

### Requirement: The writer reaches command through Deps.Writer
`command.Writer` is `interface{ Write(user, machine string, tool fact.Tool, recs []fact.Record) error }` — the own-user day-file write that precedes every repo read in multi mode; nil means no write (tests). `sync.Writer{Dir}` satisfies it by calling the package `Write(..., dryRun=false)` and dropping the decisions; the compile-time assertion sits beside the `cmd/tu` assignment (`Deps.Writer = metricsync.Writer{Dir: cfg.MetricsDir}`). The write-then-read composition in `gatherOwn` and its byte-stability argument live in [multi-mode](/go-port/multi-mode.md).

### Requirement: turepair reproduces repair-metrics.mjs byte for byte
`sync.Repair(o RepairOptions{Repo, Write}, git Runner) (stdout, stderr []string, exit int)` is the Go twin of `scripts/repair-metrics.mjs`; `cmd/turepair` (built as `bin/turepair` by `just go-build`) is a maintainer binary outside the `tu` grammar — like the mjs, not bundled into the shipped CLI, run from a checkout. Arg parsing matches the mjs: `--repo <path>` (default `~/.tu/metrics_repo`, expanded and resolved absolute so the printed path matches), `--repo` without a value → `repair-metrics: --repo requires a path`, `--write` flips to restore mode, anything else → `repair-metrics: unknown argument: {arg}`; both failures append the usage line `Usage: node scripts/repair-metrics.mjs [--repo <path>] [--write]` **verbatim, node spelling included**, so the two implementations' outputs diff clean until a post-Z1 follow-up owns changing it. `sync.FailLines(msg)` renders the mjs `fail()` shape — `repair-metrics: {msg}` with an embedded newline + usage line split into lines — and is exported so `cmd/turepair`'s arg-parse failures and `Repair`'s own failures render identically.

The algorithm: `repo not found: {repo}` when the path is absent; `not a git repository: {repo}` via `rev-parse --is-inside-work-tree`; `git ls-files -z -- *.jsonl` filtered by the day-file regexp `-\d{4}-\d{2}-\d{2}\.jsonl$`; ONE `git log --format=%H%x09%cs --name-only -- *.jsonl` walk building per-file newest-first commit lists; `git show {sha}:{path}` per commit (deleted paths and unparseable blobs skipped); first-line JSON parse of `totalCost` with the same finite check and `jsNumber` coercion the writer's guard uses; working-tree cost with missing/unparseable = 0; `centTolerance = 0.01` — a file is "shrunk" only when HEAD is below its historical max by more than a cent. Every git call carries `-c core.quotePath=false` before the verb (passed through `Exec.Run`'s own `-C <dir>` prefix). The report: `repair-metrics: scanned {repo}`, `  {n} tracked day-files, {m} commits touching *.jsonl`, blank, then either `Nothing to repair — every day-file is at its historical maximum.` or `Shrunk day-files ({k}):` + the padded `FILE/CURRENT/MAX/DELTA/MAX COMMIT` table (`padEnd`/`padStart` widths, `money = "$" + FixedHalfUp(v, 2)`, `MAX COMMIT` as `{sha[:7]} ({date})`) + `Per-user totals:` rows `  {user}: +{money} across {n} file(s)` + `Grand total: +{money} across {k} file(s)`. Dry-run tail: ``, `Dry run — nothing modified. Re-run with --write to restore shrunk files.` Under `--write` the full historical-max blob is restored byte-exact into the **working tree only** (each day-file stays an atomic snapshot that was real at some point; review/commit/push are left to the user), followed by ``, `Restored {k} file(s) in the working tree.`, `Review with: git -C {repo} diff`, `Then commit and push manually.` A write failure during restore is ignored — the mjs's uncaught throw leaves the same half-restored tree.

Two orderings are non-trivial ports, both Node-verified and pinned in `localecmp_test.go`: the shrunk list sorts by `localeCompare` — String.prototype.localeCompare under the ICU root collation restricted to the ASCII repertoire day-file paths contain (`_` < `-` < `.` < `/` < digits by value < letters case-insensitively, no numeric collation, tertiary lowercase-before-uppercase, shorter prefix first; any other byte falls back to byte order at its position, a documented limit) — observable because `sahil/2026/Sahils-Mac-mini.local/…` sorts after `sahil/2026/dev-ws-sahil02/…` under ICU and before it in byte order. The per-user block sorts as `[...byUser.entries()].sort()` — the default JS sort on `[user, obj]` arrays stringified to `"{user},[object Object]"` compared by code units, i.e. byte order of the decorated name (the decoration matters only when one name is a prefix of another whose next byte sorts below `,`).

### Requirement: Unmatchable crash paths are reported, never harnessed
The TS crashes with an uncaught Node stack trace on a filesystem failure inside the writer or the `.last-sync` write. The Go port returns those errors up to the edge, which prints `err.Error()` on stderr and exits 1 — the same posture as the init-metrics clone failure. These paths are unmatchable by construction (a stack trace is not a diffable surface) and are recorded as expected diffs, never harness cases. The same class covers `repair`'s ignored restore-write failure.

### Requirement: Harness copy/excerpt helpers are exported for tudiff live
`internal/harness` exports `CopyTree` (recursive copy used for seeding), `CopyFile` (one file, parent dirs created, explicit mode) and `Excerpt` (quoted ≤40-byte divergence excerpt), which `cmd/tudiff/live.go` reuses for its seeding and comparators. One look-alike remains as a review-verified follow-up: `liveFilePresent` in `cmd/tudiff/live.go` duplicates the unexported `filePresent` in `internal/harness/diff.go` — a recorded should-fix reuse finding, not a deletion.

### Requirement: Spec and memory inaccuracies recorded for gate G0
The shipped code disagrees with the spec and TS-facing memory text in four places; the code is the harness-enforced truth (D4 freezes the TS; the specs are human-curated), and all four are recorded for the human to resolve at gate G0:

1. `docs/specs/usage.md` § Sync Flow and `docs/specs/layouts.md` § 20 say `--sync` prints `syncing metrics... ` and then nothing more on success; the shipped TS prints `syncing metrics... synced.`
2. DC-18 / § Sync Flow say a push failure prints only the generic error; the TS prints `Warning: sync push failed after retry — {reason}` before it. Only the commit failure is silent.
3. DC-22 says the `Would write` lines carry thousands separators; both report blocks use the same `toFixed(2)` formatter — neither does.
4. The TS-facing memory [multi-machine](/sync/multi-machine.md) says the dry-run's `git status` is guarded by `existsSync` of the user dir; the code runs it unconditionally (a tracked-but-deleted user dir must still count as dirty). TS-facing memory is outside this row's hydrate scope; the correction is a docs follow-up.

## Design Decisions

### Returned lines instead of writers below the edge
**Decision**: `SyncMetrics`/`FullSync` return their stderr lines (and the fetch's typed warnings) in order; `cmd/tu` writes them.
**Why**: Nothing prints below `cmd/tu` (the G1 only-writer rule); the TS emits these lines from inside the flow, but no other output can interleave, so returned lines reproduce the bytes exactly.
**Rejected**: Passing an `io.Writer` into `sync` — makes `sync` a writer and breaks the edge invariant.
*Introduced by*: 260916-lsml-sync-metrics-writer

### One FullSync with a dryRun flag
**Decision**: A single function runs fetch → per-tool `Write(dryRun)` → (dry) local git half / (live) `SyncMetrics` + touch.
**Why**: Toolkit principle №5 — the preview must share the live decision path; two functions would drift.
**Rejected**: Separate `Preview`/`Sync` functions sharing helpers.
*Introduced by*: 260916-lsml-sync-metrics-writer

### The live fetch reaches sync through a Fetcher interface
**Decision**: `sync.Fetcher` is a one-method interface satisfied by `*ccusage.Source`, asserted at the edge; `sync` imports `fact`, `source`, `source/metrics`, `config` and `render`, never the ccusage adapter or `command`.
**Why**: The port plan's target architecture assigns the writer, driver and dry-run report to `sync` and forbids the adapter dependency — the same seam shape `command` uses for its own fetch.
**Rejected**: Importing the adapter from `sync` (couples the flow package to an I/O adapter the architecture keeps at the ends).
*Introduced by*: 260916-lsml-sync-metrics-writer

### The writer reaches command through Deps.Writer
**Decision**: `command` sees the writer as a one-method interface; `sync.Writer{Dir}` adapts the package function; the edge assigns it.
**Why**: `command` stays filesystem-free and testable with a recording fake, exactly like `Repo` and `Fetcher`.
**Rejected**: Importing `sync` from `command` (couples the composer to an I/O package the G1 gate keeps at the ends).
*Introduced by*: 260916-lsml-sync-metrics-writer

### Stale is ported caller-less
**Decision**: `Stale` ships with no caller, mirroring the TS `isStale`.
**Why**: The plan row names the auto-sync TTL and DC-20 leaves its fate to gate G0; porting the helper keeps the surface ready without inventing a call site the TS does not have.
**Rejected**: Wiring `Stale` into an auto-sync trigger (a behavior change the row forbids); dropping it (loses the ported TTL semantics G0 may choose to activate).
*Introduced by*: 260916-lsml-sync-metrics-writer

### turepair is a maintainer binary, not a tu subcommand
**Decision**: The repair twin ships as `cmd/turepair` over `sync.Repair`, built by `go-build`, never in the formula's grammar.
**Why**: The port plan's Goal freezes the CLI grammar and `--help`; the mjs was likewise outside `dist/tu.mjs`; the `tudiff` precedent exists for repo-run maintainer tools.
**Rejected**: A hidden `tu repair-metrics` subcommand (grammar change) or leaving the mjs as the only helper (dies with Z1).
*Introduced by*: 260916-lsml-sync-metrics-writer

### TouchLastSync creates the state dir
**Decision**: `MkdirAll(stateDir)` precedes the `.last-sync` write.
**Why**: The TS would crash with an uncaught error there (a path no harness case reaches); Constitution II prefers degrading to crashing.
**Rejected**: Reproducing the crash.
*Introduced by*: 260916-lsml-sync-metrics-writer
