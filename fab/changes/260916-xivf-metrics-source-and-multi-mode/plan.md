# Plan: Metrics-Repo Source and Multi-Mode Merge (Go port row B3)

**Change**: 260916-xivf-metrics-source-and-multi-mode
**Intake**: `intake.md`

## Requirements

> Every surface below is Goal-frozen: the Go binary reproduces the shipped TypeScript bytes (intake §9 byte references). Byte references live under `bin/harness/report/cases/<id>/<conf>/<env>/<io>/<tz>/node.*` after `just go-diff --placeholder`; the oracle sources are `src/node/core/cli.ts` (`checkMetricsDirGuard`, `fetchToolMerged`, `dispatchAllSnapshot`, `dispatchAllHistory`, `dispatchSingleTool`, `main()`), `src/node/sync/sync.ts` (`listUsers`, `readRemoteEntriesByMachine`) and `src/node/core/fetcher.ts` (`mergeEntries`, `maxMergeEntries`). Read intake §2–§9 before starting: the design values there are normative.

### Sources: the metrics-repo reader

#### R1: `metrics.Source.Users`
`internal/source/metrics` SHALL define `type Source struct{ Dir string }` with `Users() []string`: the direct children of `Dir` that are directories, excluding dot-prefixed names and the `NON_USER_DIRS` set `{"docs"}`, sorted ascending in byte order. A missing or unreadable `Dir` MUST return `nil` with no error, no panic and no output.

- **GIVEN** a `Dir` containing directories `other-user`, `harness-user`, `docs`, `.git` and a plain file `README`
- **WHEN** `Users()` runs
- **THEN** it returns `[harness-user other-user]`
- **AND** for a nonexistent `Dir` it returns `nil`

#### R2: `metrics.Source.Read`
`Read(user string, tool fact.Tool) []fact.Record` SHALL return `user`'s records for `tool` across every machine in **walk order**: year directories ascending, machine directories ascending, files ascending; only files whose name has prefix `tool.Key + "-"` and suffix `.jsonl` are read. Per file: read, trim whitespace, skip when empty, decode exactly one JSON object; skip silently on any decode error. `Record.Date` MUST be the JSON `label` (never the filename date), `Tool` is `tool.Key`, `User` is `user`, `Machine` is the machine directory name, `Totals` are the six pinned keys with a missing key decoding as `0`. A missing user directory, an unreadable level, or a non-directory at the year or machine level MUST be skipped silently (`nil` when nothing is read). The package MUST NOT write, print, or exec anything, and MUST NOT import `internal/source/ccusage`, `internal/command`, `internal/config`, or `internal/sync`.

- **GIVEN** the committed seed `harness/metrics-repo/` copied to `Dir`
- **WHEN** `Read("harness-user", cc)` runs
- **THEN** it returns, in order, `harness-machine/cc-2026-01-05` (`0.25`), `harness-machine/cc-2026-01-06` (`0.75`), `other-box/cc-2026-01-06` (`0.40`), each stamped `User: harness-user` and `Machine:` the directory name, with Date from the JSON label
- **AND** `Read("harness-user", codex)` returns only `other-box/codex-2026-01-07` (`0.30`)
- **AND** `Read("nobody", cc)` returns `nil`
- **AND** a file `ccx-2026-01-05.jsonl` is never matched for tool `cc`

### Query: the merge arithmetic

#### R3: `query.MaxMerge`
`query.MaxMerge(a, b []fact.Record) []fact.Record` SHALL keep, per key `(Date, Tool, User, Machine)`, whichever WHOLE record — from `a` or from `b` — has the greater `TotalCost`; on a tie the record from `a` wins. It MUST never mix fields across records and never sum. Output order is `a`'s records in `a`'s order (replaced in place when `b` wins), then `b`'s unmatched records in `b`'s order. Inputs MUST NOT be mutated or returned.

- **GIVEN** `a` = live cc `2026-01-05` (`0.50`, tokens T1) and `2026-01-06` (`0.50`, T1); `b` = stored cc `2026-01-05` (`0.25`, T2) and `2026-01-06` (`0.75`, T2) and `2026-01-04` (`0.10`, T2)
- **WHEN** `MaxMerge(a, b)` runs
- **THEN** it returns `2026-01-05` (`0.50`, T1), `2026-01-06` (`0.75`, **T2**), `2026-01-04` (`0.10`, T2) in that order
- **AND** with equal costs the `a` record (its tokens included) is kept

#### R4: `query.Collapse`
`query.Collapse(recs []fact.Record, dims ...Dim) []fact.Record` SHALL sum `Totals` per distinct tuple of `dims` and return one record per group carrying only the grouped dims (other string fields empty), in first-seen order, with additions performed in input order. It MUST be implemented over `GroupBy` (no second aggregation loop); it is the TS `mergeEntries`.

- **GIVEN** records cc `2026-01-06` from machine A (`0.75`) then machine B (`0.40`), and codex `2026-01-07` from B (`0.30`)
- **WHEN** `Collapse(recs, Tool, Date)` runs
- **THEN** it returns cc `2026-01-06` (`1.15`, User/Machine empty) then codex `2026-01-07` (`0.30`)
- **AND** on records already unique per key the output equals the input's Totals one-for-one

### Config and sync: the auto-clone guard

#### R5: `sync.Exec.CloneQuiet` and `config.Cloner`
`internal/config` SHALL define `type Cloner interface { CloneQuiet(ctx context.Context, url, dir string) (stderr string, err error) }`. `internal/sync.Exec` SHALL implement it: `git clone <url> <dir>` with stdout and stderr captured (never inherited), `GIT_TERMINAL_PROMPT=0` appended to the child environment, and a 30 s deadline applied inside `CloneQuiet` (the TS `timeout: 30_000`); `err` is `nil` on exit 0, the `*exec.ExitError` on a non-zero exit, and a `context.DeadlineExceeded`-wrapping error on the deadline. `Exec.Clone` (the interactive init-metrics clone) is unchanged.

- **GIVEN** the harness fake git first on `PATH`
- **WHEN** `CloneQuiet(ctx, url, dir)` runs
- **THEN** the call log records argv `["clone", url, dir]`, `err` is `nil`, and nothing is written to the process's stdout or stderr

#### R6: `config.MetricsDirGuard`
`config.MetricsDirGuard(cfg Config, stateDir string, now time.Time, git Cloner) (Config, []string)` SHALL be the TS `checkMetricsDirGuard`. When `cfg.Mode != Multi` or `cfg.MetricsDir` exists (an existence check via `os.Stat`, not an is-repo check) it returns `cfg` unchanged and no lines. Otherwise, in order: (1) if `stateDir/.clone-failed` exists, its trimmed content parses as an RFC 3339 timestamp, and `now - ts < 3h` → return `cfg` with `Mode = Single` and the single line `Warning: metrics repo not available — falling back to single mode.` (a missing, unreadable, or unparseable marker counts as stale); (2) else call `git.CloneQuiet(ctx, cfg.MetricsRepo, cfg.MetricsDir)`: on success return `cfg` unchanged (still `Multi`, even if the directory still does not exist) with the line `Cloned metrics repo → {cfg.MetricsDir}` (absolute path) and delete the marker via `RemoveCloneMarker(stateDir)`; on failure create `stateDir` if needed, write the marker with content `now.UTC().Format("2006-01-02T15:04:05.000Z")`, and return `cfg` with `Mode = Single` and the line `Warning: could not clone metrics repo ({detail}) — falling back to single mode.` where `{detail}` is `Command failed: git clone {url} {dir}` followed by `\n` + the captured stderr when that stderr is non-empty, or `spawnSync git ETIMEDOUT` when the error wraps `context.DeadlineExceeded`. The TS `!metricsRepo` branch is unreachable (Multi implies a non-empty repo) and MUST NOT be ported. The guard MUST NOT print; the edge prints the returned lines.

- **GIVEN** a Multi config whose `MetricsDir` is absent, no marker, and a fake `Cloner` that succeeds
- **WHEN** the guard runs
- **THEN** the fake was called once with `(cfg.MetricsRepo, cfg.MetricsDir)`, the returned config is still `Multi`, and the lines are exactly `[Cloned metrics repo → <dir>]`
- **AND** with a marker written 1 h ago the fake is never called, the config is `Single`, and the line is the `not available` warning
- **AND** with a marker written 4 h ago (or containing garbage) the clone is attempted
- **AND** when the fake fails with stderr `fatal: repository not found` the marker is written with the pinned format, the config is `Single`, and the line reads `Warning: could not clone metrics repo (Command failed: git clone <url> <dir>\nfatal: repository not found) — falling back to single mode.`

### Command: composition

#### R7: `Repo`, `Deps.Repo`, `Run(cfg)`
`internal/command` SHALL define `type Repo interface { Users() []string; Read(user string, tool fact.Tool) []fact.Record }`; `Deps` gains `Repo Repo`; `Run`'s signature becomes `Run(ctx context.Context, req Request, cfg config.Config, deps Deps) (Result, error)`. `metrics.Source` MUST satisfy `Repo` (asserted with a compile-time `var _ command.Repo = metrics.Source{}` where `cmd/tu` assigns it). `command` MUST NOT import `internal/source/metrics` or `internal/source/ccusage`.

- **GIVEN** the existing `Run` tests
- **WHEN** they are updated to pass `config.Config{Mode: config.Single}`
- **THEN** every B2 assertion still holds unchanged

#### R8: `Normalize(req, mode, now)` — the single-mode `-u` notice
`Normalize` SHALL take the post-guard `config.Mode` and, **before** the since/until guard, when `req.Flags.User != "" && mode == config.Single`, append the notice `Warning: -u flag requires multi mode — ignoring.` and clear `Flags.User`. The notice applies to `-u all` exactly as to any name. The existing since/until, `--full` and cap steps and their order are unchanged (guard order: `-u` → since/until → full → cap; B4 and B5 insert their guards later).

- **GIVEN** `tu -u other-user` in single mode
- **WHEN** `Normalize` runs
- **THEN** the notices are exactly `[Warning: -u flag requires multi mode — ignoring.]`, `Flags.User` is `""`, and the request is in scope (renders the live snapshot, exit 0)
- **AND** `tu -u other-user --since 2026-01-01` (a snapshot) yields the `-u` notice first, then the since/until notice

#### R9: `gather` — the four record paths
`Run` SHALL obtain the daily records through one helper (`gather`) selecting the TS path by mode and `-u`, iterating tools in registry order (all `fact.Tools`, or the one requested tool): **single** → the live fetch exactly as today (`FetchAll` or `Fetch`, `source.PeriodDaily`, `Fresh` honored, errors collected). **multi, `-u ""` or `-u == cfg.User`** → live fetch; then for each tool `stored := Repo.Read(cfg.User, tool)`, split into `own` (`Machine == cfg.Machine`) and `others`; records = `MaxMerge(live, own)` followed by `others` in walk order. **multi, `-u all`** → for each `u` in `Repo.Users()`, for each tool `Repo.Read(u, tool)`; no live fetch, no source errors. **multi, `-u <other>`** → `Repo.Read(user, tool)` per tool; no live fetch, no source errors. The repo-only paths MUST NOT call `Fetch`/`FetchAll` at all.

- **GIVEN** a multi config (`User: harness-user`, `Machine: harness-machine`), a fake Fetcher returning live cc `2026-01-05..07` at `0.50`, and a fake Repo mirroring the seed
- **WHEN** `Run` executes `cc h --since 2026-01-01 --until 2026-01-31`
- **THEN** the rows are `2026-01-05 $0.50`, `2026-01-06 $1.15` (tokens `6,000/800/2,000/40,000/48,800`), `2026-01-07 $0.50`, Total `$2.15`, and the Fetcher recorded exactly one call
- **AND** for `-u other-user` and `-u all` the Fetcher recorded zero calls and `Result.Warnings` is empty
- **AND** for `-u harness-user` the result is byte-identical to no `-u`

#### R10: Collapse before the tail; summation order
`gather` SHALL return `query.Collapse(recs, query.Tool, query.Date)` — the daily cross-machine/cross-user sum — and the unchanged B2 tail consumes it: `runSnapshot` = `GroupBy(Window(RollUp(daily, p), cur, cur), Tool)`; `runHistory` = `RollUp(Window(daily, since, until), p)` → one `GroupBy(Tool, Date)`. The collapse runs in single mode too (identity on unique keys). Additions within a key happen in input order (own machine first, then other machines in walk order; for `-u all`, users ascending then walk order).

- **GIVEN** the R9 fixture with `cc mh --json`
- **WHEN** `Run` executes in multi mode
- **THEN** the JSON is exactly `"label": "2026-01", "totalCost": 2.15, "inputTokens": 12000, "outputTokens": 1600, "cacheCreationTokens": 4000, "cacheReadTokens": 80000, "totalTokens": 97600`
- **AND** a test with three machine costs whose sum is association-sensitive (e.g. `0.1`, `0.2`, `0.3` across two dates rolled to a month) asserts the Go float equals the TS association `((d1) + (d2a + d2b))`

#### R11: Snapshot label rule keyed on mode
`runSnapshot` SHALL clear `ToolTotals.Label` only when `cfg.Mode == config.Single && req.Source == "" && req.Period == query.Daily`. In multi mode a tool with a record on the current label carries `"label"` in `--json`; zero tools carry nothing.

- **GIVEN** multi mode, `Now` on `2026-01-06`, live cc on that date
- **WHEN** `tu --json` runs
- **THEN** the `Claude Code` object begins with `"label": "2026-01-06",` and the zero tools have no label key
- **AND** in single mode the same request carries no label key (the B2 test still passes)

#### R12: Scope
`inScope` SHALL admit multi mode and a non-empty `Flags.User`, and continue to reject `Watch`, `Sync`, `DryRun`, `ByMachine`, `NoRain`, `SkipBrewUpdate`, `Top != 0`, non-data commands, `Version`, and displays other than Snapshot/History. `ErrUnported` keeps `--by-machine` (B4), `lb`/`lbh`/`--top` (B5), `sync`/`--sync`/`--dry-run` (B6), watch (B7).

- **GIVEN** `tu h --by-machine` in multi mode and `tu lb` in multi mode
- **WHEN** `run` executes
- **THEN** both print `tu: not implemented (Go port in progress)` on stderr, exit 1

### Edge: `cmd/tu`

#### R13: Edge order and wiring
`cmd/tu.run` SHALL, after `config.Load` and its warnings: call `config.MetricsDirGuard(cfg, config.StateDir(paths.Home), time.Now(), metricsync.Exec{})` and print the returned lines to stderr; then run the reserved-user guard on the returned `cfg.User`; then build `Deps` with `&ccusage.Source{Cache: store, User: cfg.User, Machine: cfg.Machine}` and `Repo: metrics.Source{Dir: cfg.MetricsDir}`; then `command.Run(ctx, req, cfg, deps)`; then notices → warnings → lines as today. The guard MUST NOT run for `Command != ""` (setup commands). Only `cmd/tu` imports `internal/source/ccusage` and `internal/source/metrics`. The file-header comment lists multi mode and `-u` among the answered surfaces.

- **GIVEN** the `single/envrepo` staged home (`TU_METRICS_REPO` set, no metrics dir, fake git on `PATH`)
- **WHEN** `tu` runs
- **THEN** the call log records one `git clone <url> <abs dir>` followed by the six ccusage calls, stderr is exactly `Cloned metrics repo → <abs dir>` + `\n`, stdout is the empty snapshot table, exit 0
- **AND** `tu status` in the same home records no git call

### Verification

#### R14: Harness gate
`just go-diff --placeholder` MUST report **302 green, 66 red, 0 timeout** of 368, with by-conf `single 135/155  multi 107/127  org 30/43  legacy 30/43` and by-env `envrepo 32/32`. Every one of the 158 IDs listed in intake §2 MUST be green; the 66 red MUST be exactly the `lb*`/`*-lb*`/`lbh*` groups (B5), `sync-cmd`/`sync-dry-run`/`sync-flag`/`cc-sync` (B6), the `*-by-machine*` groups and `by-machine-user-all` (B4) — 44 + 12 + 10. (The recipe exits 1 while any case is red — read the summary line.)

- **GIVEN** a clean `just go-build` and `just harness-build`
- **WHEN** `env -u TU_METRICS_REPO -u NO_COLOR just go-diff --placeholder` runs
- **THEN** the summary line reads `tudiff: 368 cases — 302 green, 66 red, 0 timeout` and no listed gate ID appears as `RED`

#### R15: Toolchain and test shape
`gofmt -l src/go` MUST be empty; `go vet ./...` and `just go-test` MUST pass (run tests with `env -u TU_METRICS_REPO -u NO_COLOR`). New tests are table-driven (`metrics`, `query`, `config` guard, `command` Run) or e2e through `cmd/tu`'s `run` with staged homes; no string-assertion sprawl copied from the TS tests.

- **GIVEN** the finished branch
- **WHEN** `just go-lint && env -u TU_METRICS_REPO -u NO_COLOR just go-test` runs
- **THEN** both exit 0

### Non-Goals
- No day-file write in the multi-mode fetch path (B6 — writer, never-shrink guard, dry-run); no `--by-machine` or per-machine roll-ups (B4); no `lb`/`lbh`, `--top`, or the `lb requires multi mode` guard (B5); no `sync`/`--sync` (B6); no watch (B7).
- No change to `src/node/`, `docs/specs/`, `harness/matrix.json`, `harness/metrics-repo/`, `harness/fixtures/`, `justfile`, `.github/workflows/`, `Formula/`, `package.json`, or the plan document (D4 freeze; the operator owns row status).
- No port of the TS `excludeMachine` parameter (test-only, flagged for deletion) or of the unreachable `!metricsRepo` guard branch.

### Design Decisions

#### A `Repo` interface beside `Fetcher`, not a second `Fetcher`
**Decision**: `command` consumes the metrics clone through `Repo{Users, Read}`; `metrics.Source` implements it.
**Why**: A repo read has no context, period, extra args, cache, fresh flag or error channel — forcing it through `Fetcher` would mean ignored parameters and a fake `*source.Error`. Two small honest interfaces keep `command` adapter-free (G1 item 1) and give B4/B5 `Users()` and per-user reads directly.
**Rejected**: `metrics.Source` satisfying `Fetcher` (run.go's original comment) — ignored parameters and no way to express "all users" or "one user" without smuggling them through `extraArgs`.
*Introduced by*: 260916-xivf-metrics-source-and-multi-mode

#### Daily collapse before roll-up, for float parity
**Decision**: `gather` returns `Collapse(recs, Tool, Date)`; the B2 `Window` → `RollUp` → `GroupBy` tail is unchanged.
**Why**: `--json` prints costs raw. The TS sums per day across machines first (`mergeEntries`) and rolls days into periods second; `RollUp` then `GroupBy` would sum each machine's days first and machines second — a different float association that can differ in the last bit and therefore in the bytes. Collapsing at the daily level reproduces the TS association; running it in single mode too keeps one pathway.
**Rejected**: Changing `RollUp` to ignore User/Machine (breaks B4's per-machine columns); accepting the association drift (a `2.1500000000000004` is a harness red and a spec violation).
*Introduced by*: 260916-xivf-metrics-source-and-multi-mode

#### Read-only multi mode until B6
**Decision**: The Go multi-mode fetch path reads the clone and writes nothing; the TS pre-fetch `writeMetrics` lands with B6's writer.
**Why**: The plan assigns the writer, the never-shrink guard and the day-file layout to B6 under D11 (port faithfully, gate on live-sync parity). The harness compares stdout/stderr/exit and the call multiset, none of which the write touches, and write-then-max-merge is arithmetically identical to max-merge alone for the rendered bytes. It is a temporary, memory-documented divergence from "a plain `tu` in multi mode is a write".
**Rejected**: A minimal writer in B3 (splits D11's guarded surface across two rows and two reviews).
*Introduced by*: 260916-xivf-metrics-source-and-multi-mode

#### The clone guard in `config`, git behind `Cloner`, the edge prints
**Decision**: `MetricsDirGuard` lives in `internal/config` (which already owns `CloneFailedMarker`, `RemoveCloneMarker`, `StateDir` and `Load`), sees git only through the `Cloner` interface, and returns its stderr lines for `cmd/tu` to print.
**Why**: The only-writer rule (G1 items 1/4); the `Git`/`Exec` precedent from B1's `InitMetrics`; a fake `Cloner` makes every branch (marker fresh/stale/garbage, clone ok/fail/timeout) unit-testable without git or a network.
**Rejected**: Running the clone inline in `cmd/tu` (untestable, bloats the edge); putting the guard in `sync` (it decides config mode, not sync behavior).
*Introduced by*: 260916-xivf-metrics-source-and-multi-mode

## Tasks

### Phase 1: Setup

- [x] T001 Create `src/go/internal/query/merge.go` with `MaxMerge` (R3) and `Collapse` (R4, implemented over `GroupBy` — convert each `Group` back to a `fact.Record` carrying only the grouped dims) plus `merge_test.go`: table-driven cases from the R3/R4 scenarios, a tie case, an unmutated-input assertion, and an order assertion. <!-- R3, R4 -->
- [x] T002 [P] Create `src/go/internal/source/metrics/metrics.go` (package doc per intake §3; `Source{Dir}`, `Users`, `Read`, the `nonUserDirs` set) and `metrics_test.go`: build temp-dir trees (including a copy of `harness/metrics-repo/` located by walking up to `package.json`, mirroring `internal/config/defaults_test.go`) and assert the R1/R2 scenarios — walk order, label-not-filename, `cc-` vs `ccx-` prefix, empty/whitespace/garbage files skipped, missing user, `Users()` exclusions, unreadable dir → `nil`. <!-- R1, R2 -->
- [x] T003 [P] Add `CloneQuiet` to `src/go/internal/sync/git.go` (captured streams, `GIT_TERMINAL_PROMPT=0`, 30 s `context.WithTimeout` inside; update the file's comments that reserved this for B3) and a `git_test.go` case running it against a stub `git` on a temp `PATH` that records argv and exits 0 / exits 1 with stderr; create `src/go/internal/config/guard.go` (`Cloner`, `MetricsDirGuard`, marker read/write helpers, the `cloneFailureDetail` composer for R6) and `guard_test.go` with a fake `Cloner` and fixed `now` covering every R6 scenario incl. the pinned marker timestamp format. <!-- R5, R6 -->

### Phase 2: Core Implementation

- [x] T004 In `src/go/internal/command/request.go` add the `Repo` interface and `Deps.Repo`; in `guards.go` change `Normalize` to `(req Request, mode config.Mode, now time.Time)` and add the `-u` notice as step 0 (R8; update the doc comment's guard order); in `run.go` change `Run` to take `cfg config.Config`, widen `inScope` (R12), and update every existing call site and test (`run_test.go`, `guards_test.go`, `cmd/tu`) to the new signatures — all B2 tests must still pass before T005 starts. <!-- R7, R8, R12 -->
- [x] T005 In `src/go/internal/command/run.go` implement `gather(ctx, req, cfg, deps) ([]fact.Record, []*source.Error)` with the four R9 paths ending in `Collapse(recs, Tool, Date)` (R10), and key the snapshot label clear on `cfg.Mode == config.Single` (R11); add a `fakeRepo` to `run_test.go` mirroring the seed and table-driven tests for: the R9 own-user merge rows and fetch-call count, `-u other`/`-u all` zero fetch calls and empty warnings, `-u <cfg.User>` byte-equal to no flag, the R10 `cc mh --json` bytes and the association-sensitive float case, the R11 multi-mode label presence, the R8 notice ordering, and the cap on stored records (`h` hides a stored `2025-10-01` file, `h --full` shows it, with `Now` fixed in January 2026). <!-- R9, R10, R11 -->

### Phase 3: Integration & Edge Cases

- [x] T006 Wire `src/go/cmd/tu/main.go` per R13: the guard call between `config.Load` and the reserved-user check (print its lines), the stamped `ccusage.Source`, `metrics.Source{Dir: cfg.MetricsDir}` in `Deps.Repo` with the compile-time `Repo` assertion, `command.Run(ctx, req, cfg, deps)`; update the file-header comment. <!-- R13, R7 -->
- [x] T007 Extend `src/go/cmd/tu/main_test.go` (`TestRunNotImplemented` now uses `{"h","--by-machine"}` and `{"lb"}` under a multi home — drop any multi-mode data case from the placeholder list) and `src/go/cmd/tu/e2e_test.go`: staged `multi`, `org` and `legacy` homes running `cc h --since 2026-01-01 --until 2026-01-31` (the intake §9 bytes, ANSI stripped via `NO_COLOR` unset but stdout a buffer → no color, width 80), `cc mh --json` (the R10 bytes), `-u other-user` and `-u all` (empty snapshot, **no** ccusage call in `TUDIFF_CALL_LOG`), `-u other-user` in a single home (the warning line, six ccusage calls), the `envrepo`-style clone path (`TU_METRICS_REPO` set on a single home: git argv `clone <url> <abs dir>` asserted, the `Cloned` stderr line), and a fresh `.clone-failed` marker (the `not available` warning, no git call, exit 0). <!-- R13, R9, R8, R6 -->
- [x] T008 Run `just go-lint`, `env -u TU_METRICS_REPO -u NO_COLOR just go-test`, `just go-build`, then `env -u TU_METRICS_REPO -u NO_COLOR just go-diff --placeholder` (exit 1 is expected while any case is red — read the summary line and grep the case lines); iterate on any divergence until the summary reads `302 green, 66 red, 0 timeout` with the R14 by-conf/by-env counts and every intake §2 gate ID `GREEN`. <!-- R14 -->

### Phase 4: Polish

- [x] T009 Verify boundaries and finish: `gofmt -l src/go` empty; `cd src/go && go vet ./...` clean; `grep -rn "source/ccusage\|source/metrics" src/go/internal --include='*.go' | grep -v _test` shows no importer outside the adapters themselves (only `cmd/tu` names them); `internal/source/metrics` imports nothing from `command`, `config`, `sync`, or `ccusage`; no function added this change exceeds 50 lines (split `gather` per path if it does); every `## Tasks` item is `[x]`. <!-- R15 -->

## Execution Order

- T001, T002, T003 are independent and can run in parallel.
- T004 depends on T001 (imports `Collapse`/`MaxMerge` compile) and precedes T005; T005 depends on T002's package existing only for the compile-time assertion in T006.
- T006 depends on T003, T004, T005; T007 on T006; T008 on T007; T009 on T008.

## Acceptance

### Functional Completeness

- [x] A-001 R1: `metrics.Source.Users` returns the sorted profile directories, excludes `.git`/`docs`/plain files, and returns `nil` for a missing `Dir`
- [x] A-002 R2: `metrics.Source.Read` returns the seed's records in walk order with Date from the JSON label, Tool/User/Machine stamped, prefix matching exact to `{key}-`, malformed and empty files skipped silently
- [x] A-003 R3: `query.MaxMerge` keeps whole records per key, ties to `a`, preserves order, never mutates inputs
- [x] A-004 R4: `query.Collapse` sums per grouped tuple in input order over `GroupBy` and drops the ungrouped dims
- [x] A-005 R5: `sync.Exec.CloneQuiet` runs `git clone <url> <dir>` with captured streams, `GIT_TERMINAL_PROMPT=0`, and a 30 s deadline; `Exec.Clone` is unchanged
- [x] A-006 R6: `config.MetricsDirGuard` reproduces every TS branch (no-op, fresh marker, stale/garbage marker, clone ok, clone failure) with the byte-exact lines and the pinned marker format; it prints nothing itself
- [x] A-007 R7: `command.Repo` exists, `Deps.Repo` is set at the edge, `Run` takes `config.Config`, and `metrics.Source` is asserted to satisfy `Repo`
- [x] A-008 R8: `Normalize` emits `Warning: -u flag requires multi mode — ignoring.` first and clears `User` in single mode, for `all` as for any name
- [x] A-009 R9: `gather` implements the four paths; repo-only paths make no `Fetch`/`FetchAll` call and carry no source warnings; `-u <cfg.User>` equals no flag
- [x] A-010 R10: `Collapse(Tool, Date)` precedes the unchanged `Window`/`RollUp`/`GroupBy` tail on every path, single mode included
- [x] A-011 R11: multi-mode `--json` snapshots carry `label` on tools with a current record; the single-mode daily all-tools case still carries none
- [x] A-012 R12: `inScope` admits multi mode and `-u`; `--by-machine`, `lb`/`lbh`, `--top`, `--sync`, `sync`, `--dry-run`, `--watch` still yield the placeholder
- [x] A-013 R13: `cmd/tu` runs the guard between `Load` and the reserved-user check, prints its lines, stamps the ccusage source, wires `metrics.Source`, and skips the guard for setup commands

### Behavioral Correctness

- [x] A-014 R9: multi-mode `cc h --since 2026-01-01 --until 2026-01-31` against the seed renders `$0.50 / $1.15 / $0.50`, Total `$2.15`, footer `avg $0.72/day · peak $1.15 (2026-01-06)`; `h --since … --until …` renders the `$3.00 / $3.65 / $3.30`, `$9.95` pivot
- [x] A-015 R10: multi-mode `cc mh --json` prints `"totalCost": 2.15` (not `2.1500000000000004`) with the pinned token counts
- [x] A-016 R6: after a successful (fake) clone the config stays `Multi` and the readers return nothing for the still-missing directory — output is the live-only render
- [x] A-017 R8: the `-u` notice precedes the since/until notice when both fire on a snapshot

### Scenario Coverage

- [x] A-018 R14: `just go-diff --placeholder` summary reads `302 green, 66 red, 0 timeout`; by conf `single 135/155  multi 107/127  org 30/43  legacy 30/43`; by env `envrepo 32/32`; every intake §2 gate ID is `GREEN`; every red case belongs to a B4/B5/B6 group
- [x] A-019 R13: the e2e `envrepo`-style case asserts the git argv `["clone", url, dir]` and the `Cloned metrics repo → <dir>` stderr line; the `-u other-user` multi case asserts an absent/empty call log
- [x] A-020 R9: the cap applies to stored records — a stored day-file older than the floor is hidden by `h` and shown by `h --full` in a `Run` test with fixed `Now`

### Edge Cases & Error Handling

- [x] A-021 R2: a day-file whose `label` differs from its filename date is keyed by the label; a file lacking a token key decodes that field as `0`
- [x] A-022 R6: a marker with garbage content or one older than 3 h is treated as stale (clone attempted); a future timestamp is fresh (no clone — TS `now - ts < 3h` parity); `.tu` is created when the marker is written into a missing state dir <!-- review: clause corrected by the orchestrator after the reviewer flagged the original "future timestamp … stale" wording as contradicting R6 -->
- [x] A-023 R5: a `CloneQuiet` deadline maps to the `spawnSync git ETIMEDOUT` detail; a non-zero exit maps to `Command failed: git clone <url> <dir>` plus `\n<stderr>` only when stderr is non-empty
- [x] A-024 R9: `-u <user that does not exist in the repo>` renders the empty snapshot/history, exit 0, no warnings

### Code Quality

- [x] A-025 Pattern consistency: `metrics` mirrors `ccusage`'s adapter shape (value receiver, stamped identity, no I/O beyond reads); `guard.go` follows `setup.go`'s `Error`/lines style; `gofmt`/`go vet` clean
- [x] A-026 No unnecessary duplication: `Collapse` is a `GroupBy` wrapper, not a second aggregation loop; the marker path/constant is single-sourced from `setup.go`; the walk-up-to-`package.json` test helper is reused, not re-implemented
- [x] A-027 Minimum pathways: one `gather` feeding one tail; the collapse runs in every mode rather than a single-mode bypass; no result globals
- [x] A-028 Readability over cleverness: message strings live in named constants; `gather` and `MetricsDirGuard` stay under 50 lines each (split per branch if needed); typed errors surface only at the edge
- [x] A-029 Errors never swallowed silently except where the TS is silent by contract (repo reader, marker parse) — and those sites say so in a comment

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`
- Byte references are regenerated by `just go-diff --placeholder` into `bin/harness/report/cases/` (gitignored); an exported `TU_METRICS_REPO` or `NO_COLOR` in the shell breaks the Go unit tests, so prefix test runs with `env -u TU_METRICS_REPO -u NO_COLOR`.

## Deletion Candidates

- None — this change adds new functionality without making existing code redundant. The TS `excludeMachine` parameter was already flagged as a deletion candidate by 260610-srmi and is correctly not ported; `inScope`'s dropped `mode` parameter and `gitCalls`' folded body (now `loggedCalls`) were removed in place, not left behind.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | Exported names as written (`Repo`, `gather`, `MaxMerge`, `Collapse`, `MetricsDirGuard`, `CloneQuiet`); apply MAY rename but not move responsibilities across packages | Intake §3–§8 sketches; names are internal, the harness pins bytes | S:70 R:95 A:85 D:75 |
| 2 | Confident | The 30 s clone deadline lives inside `CloneQuiet` (not the caller's ctx) so the guard's `Cloner` contract is self-contained | The TS bounds the clone itself; a fake `Cloner` in tests never needs a deadline | S:60 R:90 A:85 D:75 |
| 3 | Certain | The association-sensitive float test uses hand-picked values whose TS-order sum differs from the per-machine-first sum in the last bit | The decision exists for parity; a test that cannot distinguish the two orders proves nothing | S:70 R:95 A:85 D:85 |
| 4 | Confident | `TestRunNotImplemented` keeps `{"h","--by-machine"}` and adds `{"lb"}` under a multi home as B5's example | Both remain unported after B3 | S:75 R:95 A:90 D:85 |

4 assumptions (1 certain, 3 confident, 0 tentative).
