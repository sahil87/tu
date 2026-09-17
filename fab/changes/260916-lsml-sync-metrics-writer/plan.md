# Plan: Sync — Metrics-Repo Writer and Git Flow (Go port row B6)

**Change**: 260916-lsml-sync-metrics-writer
**Intake**: `intake.md`

> Read `intake.md` in full before any task — its § What Changes carries the exact strings, signatures, line rules and orderings this plan's requirements point at. `src/node/sync/sync.ts`, `src/node/core/cli.ts` (lines 586–668, 1931–1938, 703, 820) and `scripts/repair-metrics.mjs` are the byte oracles; `docs/specs/usage.md` § Multi-Machine Mode and `docs/specs/layouts.md` § 20 are the contract text; where spec and code disagree the CODE wins (intake § Spec inaccuracies). No edit under `src/node/`, `scripts/`, or `docs/specs/` (D4). No `git checkout`/`git stash`/`git reset` of tracked files at any point — the working tree is the deliverable.

## Requirements

### Source layer: the shared day-file shape

#### R1: `metrics.DayFile` is the one day-file encoding
`internal/source/metrics` SHALL export `DayFile{Label string \`json:"label"\`; fact.Totals}`, `Name(tool fact.Tool, date string) string` (`{tool.Key}-{date}.jsonl`) and `Path(dir, user, machine string, tool fact.Tool, date string) string` (`{dir}/{user}/{year}/{machine}/{Name}` with `year` = the label's first four characters, or the whole label when shorter). The reader's `readDayFile` MUST decode into `DayFile`; `json.Marshal(DayFile)` MUST reproduce the TS `JSON.stringify(entry)` bytes.

- **GIVEN** `DayFile{Label: "2026-01-05", Totals: {TotalCost: 0.25, InputTokens: 3000, OutputTokens: 400, CacheCreationTokens: 1000, CacheReadTokens: 20000, TotalTokens: 24400}}`
- **WHEN** marshalled
- **THEN** the bytes equal the seed file `harness/metrics-repo/harness-user/2026/harness-machine/cc-2026-01-05.jsonl` minus its trailing newline
- **AND** `0.5`, `211.8`, `1e-7`, `1e+21` and `0.30000000000000004` marshal exactly as JavaScript prints them

### Sync: the writer and the never-shrink guard

#### R2: `sync.Write` runs one decision path in live and dry-run mode
`sync.Write(dir, user, machine string, tool fact.Tool, recs []fact.Record, dryRun bool) ([]Decision, error)` SHALL, per record in order, compute `Path`, call `shrinkState(path, rec.TotalCost)`, and append `Decision{Path, Action (ActionWrite or ActionSkip), IncomingCost, ExistingCost *float64}` with `ExistingCost` non-nil only when an existing parseable cost was read. A skip decision MUST write nothing. A write decision MUST, only when `!dryRun`, `MkdirAll(dir, 0o755)` and write `json.Marshal(DayFile)+"\n"` (0o644); a filesystem error MUST stop the walk and be returned. Nothing MUST be printed. `shrinkState` MUST: treat a read error, empty/whitespace content, or undecodable JSON as absent; otherwise apply the JS `Number()` coercion to the value at key `totalCost` (number → itself; `null` → 0; `true`/`false` → 1/0; string → trimmed, empty → 0, else `ParseFloat`, failure → NaN; missing key, array, object, or a top-level non-object document → NaN); a non-finite result is absent; a finite result yields `shrinking = incoming < existing` and the pointer.

- **GIVEN** an existing `cc-2026-01-06.jsonl` with `totalCost` 0.75 and an incoming record costing 0.5
- **WHEN** `Write` runs in either mode
- **THEN** the decision is `ActionSkip` with `ExistingCost` 0.75 and the file is unchanged
- **GIVEN** an existing file costing 0.25 and an incoming 0.5, and a missing `cc-2026-01-07.jsonl`
- **WHEN** `Write` runs with `dryRun=true`
- **THEN** the decisions are `ActionWrite` (existing 0.25) and `ActionWrite` (existing nil) and no file or directory is created; with `dryRun=false` both files hold the incoming bytes
- **GIVEN** an existing file whose content is empty, `garbage`, `{"label":"x"}`, `null`, `5`, or `{"totalCost":"1.5"}`
- **WHEN** an incoming record costing 1.0 is written
- **THEN** the first five are treated as absent (write, `ExistingCost` nil) and the last yields existing 1.5 and a skip

#### R3: Commit message, `.last-sync`, staleness
`sync.CommitMessage(user string, now time.Time)` SHALL return `# {user}: update {now.UTC().Format("2006-01-02")}`. `sync.TouchLastSync(stateDir string, now time.Time) error` SHALL create `stateDir` when missing and write `{stateDir}/.last-sync` containing `now.UTC().Format("2006-01-02T15:04:05.000Z") + "\n"`. `sync.Stale(stateDir string, now time.Time) bool` SHALL return true when the file is missing or unreadable, when its trimmed content does not parse as RFC 3339, or when `now − ts > StaleAfter` (`3 * time.Hour`); false otherwise. `Stale` has no caller (DC-20).

- **GIVEN** `.last-sync` containing `2026-09-15T18:54:44.502Z`
- **WHEN** `Stale` runs at 2026-09-15T21:54:44Z and again one second past three hours later
- **THEN** it returns false, then true
- **GIVEN** a `stateDir` that does not exist
- **WHEN** `TouchLastSync` runs
- **THEN** the directory and file exist and `config.LastSync(stateDir, now+15m)` renders `15m ago ({content})`

### Sync: the git driver and the round trip

#### R4: `Exec.Run` reproduces Node's error text
`sync.Runner` is `interface{ Run(dir string, args ...string) (string, error) }`; `Exec.Run` SHALL execute `git -C <dir> <args...>` with no timeout, capturing stdout and stderr separately, and return stdout on exit 0. On failure the error's `Error()` MUST be `git -C <dir>... failed: {message}` where `{message}` is `Command failed: git -C <dir> <args joined by single spaces>\n<captured stderr>` for a non-zero exit and `spawn git ENOENT` when the binary is not found.

- **GIVEN** a PATH-first fake git that exits 1 with stderr `fatal: couldn't find remote ref main\n`
- **WHEN** `Run("/r", "pull", "--rebase", "origin", "main")` runs
- **THEN** the error text is exactly `git -C /r... failed: Command failed: git -C /r pull --rebase origin main\nfatal: couldn't find remote ref main\n`
- **GIVEN** an empty PATH
- **WHEN** `Run` runs
- **THEN** the error text ends with `failed: spawn git ENOENT`

#### R5: `SyncMetrics` is the TS round trip, lines returned
`sync.SyncMetrics(dir, user string, now time.Time, git Runner) (ok bool, lines []string)` SHALL: (1) when `{dir}/.git/rebase-merge` or `{dir}/.git/rebase-apply` exists, append `Warning: recovering from interrupted rebase` and run `rebase --abort` ignoring its error; (2) when `{dir}/{user}` exists run `add {user}/`; then run `status --porcelain {user}/` unconditionally; when its trimmed output is non-empty run `commit -m {CommitMessage(user, now)}`; any error in this step returns `(false, lines)` with no new line; (3) run `pull --rebase origin main`; on error append `Warning: sync pull failed — {err.Error()}`, run `rebase --abort` ignoring its error, return false; (4) run `push`; on error run `push` again; on the second error append `Warning: sync push failed after retry — {err.Error()}` and return false; (5) return true. The function MUST print nothing.

- **GIVEN** a fake `Runner` and an existing user dir with a status output of ` M x\n`
- **WHEN** `SyncMetrics` runs
- **THEN** the recorded argv sequence is `add u/`, `status --porcelain u/`, `commit -m # u: update {date}`, `pull --rebase origin main`, `push` and `ok` is true with no lines
- **GIVEN** a real bare repo on `main`, a clone as `dir`, and a second clone that pushed a commit
- **WHEN** a new day-file is written in `dir` and `SyncMetrics` runs with `Exec{}`
- **THEN** it returns true, the bare's `main` contains both commits, and the clone's tree holds both files
- **GIVEN** `{dir}/.git/rebase-merge/` created by hand in a clean real clone
- **WHEN** `SyncMetrics` runs
- **THEN** `lines` is exactly `[Warning: recovering from interrupted rebase]`, the abort's failure is swallowed, and the sync succeeds

#### R6: `FullSync` — one function, one decision path
`sync.FullSync(ctx, in Inputs, dryRun bool) (Outcome, error)` with `Inputs{Config config.Config; StateDir string; Now time.Time; Source Fetcher; Git Runner}`, `Fetcher` = `interface{ FetchAll(ctx, period string, extraArgs []string, fresh bool) ([]fact.Record, []*source.Error) }`, and `Outcome{OK bool; Report *Report; Warnings []*source.Error; Lines []string}` SHALL: fetch `FetchAll(ctx, source.PeriodDaily, nil, false)`; group records by `Tool` and, per `fact.Tools` in registry order, call `Write(Config.MetricsDir, Config.User, Config.Machine, tool, recs, dryRun)`. **Dry-run**: collect `ToolReport{Tool, Decisions}` per tool; `WouldCommit = anyWrite || dirty` where `dirty` is the trimmed output of `Git.Run(MetricsDir, "status", "--porcelain", User+"/")` being non-empty and any error means false; `CommitMessage = CommitMessage(User, Now)`; return `Outcome{OK: true, Report: &r, Warnings}` having created no directory, written no file, run no other git command and not touched `.last-sync`. **Live**: after the writes run `SyncMetrics(MetricsDir, User, Now, Git)`; on ok `TouchLastSync(StateDir, Now)`; return `Outcome{OK, Warnings, Lines}`. A `Write` or touch error is returned as `error`.

- **GIVEN** the seed copied to `MetricsDir`, a fake `Fetcher` returning the placeholder-shaped cc records (0.5 on 2026-01-05..07) and a fake `Runner` answering status with empty output
- **WHEN** `FullSync(dryRun=true)` runs
- **THEN** the cc report holds write(0.25→0.5), skip(0.5<0.75), write(new), `WouldCommit` is true, no file changed, `.last-sync` is absent, and the only git call was the status
- **WHEN** `FullSync(dryRun=false)` runs with the same inputs
- **THEN** `cc-2026-01-05.jsonl` holds the 0.5 bytes, `cc-2026-01-06.jsonl` still holds 0.75, `cc-2026-01-07.jsonl` exists, the git argv sequence is add/status/pull/push, and `.last-sync` was written

#### R7: `Report.Format` is `formatDrySyncReport`
`Report.Format(home string) []string` SHALL produce, with `userPrefix = filepath.Join(MetricsDir, User)`, `dir = config.Tildefy(userPrefix, home) + "/"`, `name` = `Path` with `userPrefix+"/"` stripped when it has that prefix, and `fmt(x) = "$" + render.FixedHalfUp(x, 2)`: the write lines `  {name}  {fmt(in)}  (update: {fmt(ex)} → {fmt(in)})` / `  {name}  {fmt(in)}  (new)`, the skip lines `  {name}  incoming {fmt(in)} < existing {fmt(ex or 0)}`, then `Would write {N} day-file(s) under {dir}:` + writes when N > 0 else `Would write 0 day-file(s) under {dir}.`, then only when K > 0 `Would skip {K} file(s) (never-shrink guard):` + skips, then `Would commit: "{CommitMessage}", then pull --rebase origin main, then push` or `Would commit: nothing (no changes), then pull --rebase origin main, then push`, then `Dry run — nothing written, committed, or pushed.`. No thousands separators anywhere.

- **GIVEN** the R6 dry-run report with `home` = the staged home
- **WHEN** formatted
- **THEN** the lines include `Would write 17 day-file(s) under ~/.tu/metrics_repo/harness-user/:`-style header (N counted from the decisions), `  2026/harness-machine/cc-2026-01-05.jsonl  $0.50  (update: $0.25 → $0.50)`, `  2026/harness-machine/cc-2026-01-07.jsonl  $0.50  (new)`, `Would skip 1 file(s) (never-shrink guard):`, `  2026/harness-machine/cc-2026-01-06.jsonl  incoming $0.50 < existing $0.75`, and end with the commit and dry-run lines
- **GIVEN** a report with no decisions and `WouldCommit` false
- **WHEN** formatted
- **THEN** the lines are exactly `Would write 0 day-file(s) under {dir}.`, `Would commit: nothing (no changes), then pull --rebase origin main, then push`, `Dry run — nothing written, committed, or pushed.`

### Command and edge

#### R8: The own-user path writes before it reads
`command.Writer` is `interface{ Write(user, machine string, tool fact.Tool, recs []fact.Record) error }`; `Deps` gains `Writer Writer` (nil = no write). `sync.Writer{Dir}` satisfies it (calls `Write(..., false)`, discards decisions; asserted at the `cmd/tu` assignment). `gatherOwn` SHALL, per tool in registry order, call `Writer.Write(cfg.User, cfg.Machine, tool, byTool[tool.Key])` and THEN `Repo.Read(cfg.User, tool)`; a write error propagates out of `gather` and `Run`. The repo-only paths (`-u <other>`, `-u all`, `lb`, `lbh`) and single mode MUST never call `Write`. `inScope` SHALL admit `Flags.Sync`.

- **GIVEN** a multi config and a recording fake `Writer`
- **WHEN** `Run` handles `cc h`, `-u other-user`, `-u all`, `lb` and a single-mode `cc`
- **THEN** only the first records a `Write` (user, machine, tool cc, the live records), ordered before that tool's `Read`
- **GIVEN** a fake `Writer` returning an error
- **WHEN** `Run` handles a multi-mode `cc`
- **THEN** `Run` returns that error

#### R9: `--sync` on a data command
`cmd/tu.run` SHALL, after the reserved-user guard and the deps assignment and before `command.Run`, when `req.Flags.Sync && cfg.Mode == config.Multi`: write `syncing metrics... ` (no newline) to stderr; run `FullSync(ctx, Inputs{cfg, StateDir(home), time.Now(), src (the same `*ccusage.Source`), Exec{}}, false)` under `source.DefaultTimeout`; on `error` print `err.Error()` and exit 1; else `source.WriteWarnings(stderr, out.Warnings)`, the `out.Lines`, then `synced.\n` when `out.OK` else `sync failed — using local data.\n`; then continue into `command.Run` unchanged. In single mode `--sync` MUST produce no output and no git call.

- **GIVEN** the multi staged home, the fake ccusage and the stub fake git
- **WHEN** `cc --sync` runs
- **THEN** stderr is `syncing metrics... synced.\n`, stdout is the cc table, exit 0, the day-files exist, `.last-sync` exists, and the call log holds six ccusage calls (the second fetch hits the cache) plus `add`, `status`, `pull`, `push`
- **GIVEN** `TUDIFF_GIT_SCRIPT` failing `pull` with stderr `fatal: couldn't find remote ref main\n`
- **WHEN** `cc --sync` runs
- **THEN** stderr is `syncing metrics... Warning: sync pull failed — git -C {dir}... failed: Command failed: git -C {dir} pull --rebase origin main\nfatal: couldn't find remote ref main\n\nsync failed — using local data.\n`, then the table on stdout, exit 0, and the argv shows `rebase --abort` after the pull

#### R10: `tu sync` and `tu sync --dry-run`
`runCommand`'s `sync` case SHALL, in order: `ResolvePaths` (`ErrNoHome` → message, exit 1); `Load` and print warnings; `cfg.User == "all"` → `Error: config user "all" is reserved (used by -u all)`, exit 2; `cfg.Mode != Multi` → stderr `tu sync requires metrics_repo to be set.` then `Add metrics_repo to ~/.config/tu/tu.conf, run 'tu init-metrics <repo-url>', or set TU_METRICS_REPO.`, exit 1; `MetricsDirGuard` with its lines printed, a demoted config → exit 1 silently; build the data path's `cache.Default()`/`*ccusage.Source`; `Flags.DryRun` → `FullSync(…, true)`, write warnings, then `Report.Format(paths.Home)` lines to stdout, exit 0; live → `FullSync(…, false)`, warnings and `Lines` to stderr, `!OK` → `Error: sync failed — check network and remote config.`, exit 1; else stdout `Synced to {Tildefy(cfg.MetricsDir, home)}`, exit 0. Data flags on `sync` are ignored.

- **GIVEN** the single staged home
- **WHEN** `sync` and `sync --dry-run` run
- **THEN** both print the two-line error on stderr, nothing on stdout, exit 1, no git call
- **GIVEN** the multi staged home with the stub fake git
- **WHEN** `sync --dry-run` runs
- **THEN** stdout is the byte-exact R7 report (captured from `node dist/tu.mjs sync --dry-run` on an identically staged home), exit 0, no day-file or `.last-sync` written, exactly one git call (`status --porcelain harness-user/`)
- **WHEN** `sync` runs
- **THEN** stdout is `Synced to ~/.tu/metrics_repo\n`, exit 0, the day-files and `.last-sync` exist; with the `dirty` script the argv adds `commit -m # harness-user: update {UTC date}`; with the `pushfail` script stderr is `Warning: sync push failed after retry — …` then the generic error, exit 1, two `push` calls
- **GIVEN** a fresh `.clone-failed` marker and no metrics dir
- **WHEN** `sync` runs
- **THEN** stderr is the `not available` warning only, exit 1, no git call

### The repair twin

#### R11: `turepair` reproduces `scripts/repair-metrics.mjs` byte for byte
`sync.Repair(o RepairOptions{Repo string; Write bool}, git Runner) (stdout, stderr []string, exit int)` and `cmd/turepair` (`--repo <path>` defaulting to `~/.tu/metrics_repo` via `config.ExpandHome`, `--write`, anything else → `repair-metrics: unknown argument: {arg}` + the verbatim usage line, exit 1; `--repo` without a value → `repair-metrics: --repo requires a path` + usage) SHALL implement the script's algorithm and output exactly as intake § 8 lists: the two failure messages, `ls-files -z -- *.jsonl` filtered by `-\d{4}-\d{2}-\d{2}\.jsonl$`, one `log --format=%H%x09%cs --name-only -- *.jsonl` walk (newest-first per file), `show {sha}:{path}` per commit skipping failures and unparseable blobs, first-line JSON `totalCost` with the finite check, working-tree cost with missing/unparseable = 0, `CENT_TOLERANCE` 0.01, the report block (`repair-metrics: scanned {repo}`, `  {n} tracked day-files, {m} commits touching *.jsonl`, blank, either `Nothing to repair — every day-file is at its historical maximum.` or `Shrunk day-files ({k}):`, blank, the padded table header and rows with `money` = `$`+`FixedHalfUp(v, 2)`, blank, `Per-user totals:` rows `  {user}: +{money} across {n} file(s)` in byte order of user, blank, `Grand total: +{money} across {k} file(s)`), the dry-run tail `\nDry run — nothing modified. Re-run with --write to restore shrunk files.`, and under `--write` the byte-exact historical blob restored into the working tree plus `\nRestored {k} file(s) in the working tree.\nReview with: git -C {repo} diff\nThen commit and push manually.`. Every git call carries `-c core.quotePath=false` before the verb. The shrunk list is ordered by an ASCII ICU-root comparator (`localeCompare` twin): primary punctuation (`_` < `-` < `.` < `/`) < digits < letters case-insensitively, tertiary lowercase before uppercase, other bytes by byte order.

- **GIVEN** a seeded temp repo where `u/2026/m/cc-2026-01-01.jsonl` peaked at 10.00 in commit A and holds 1.00 at HEAD, `cc-2026-01-02.jsonl` never shrank, `cc-2026-01-03.jsonl` has an unparseable historical blob, and `cc-2026-01-04.jsonl` was deleted after commit B
- **WHEN** `turepair --repo <repo>` runs
- **THEN** stdout matches `node scripts/repair-metrics.mjs --repo <repo>` byte for byte (repo path included), exit 0; with `--write` the restored file equals commit A's blob and a second dry run prints `Nothing to repair …`
- **GIVEN** the paths `sahil/2026/Sahils-Mac-mini.local/cc-2026-04-26.jsonl` and `sahil/2026/dev-ws-sahil02/cc-2026-04-24.jsonl`
- **WHEN** sorted by the comparator
- **THEN** `dev-ws-sahil02` sorts first (as Node's `localeCompare` orders them)

### Harness

#### R12: `tudiff run` compares the written tree
After the byte comparison of a case whose status is still green, `runCase` SHALL compare the two sides' `<home>/.tu/metrics_repo/` trees — the set of relative file paths and each file's bytes — and the presence of `<home>/.tu/.last-sync`; a difference sets `Status` red with `Channel: "tree"` and the first differing relative path (or `.last-sync`) in `NodeExcerpt`/`GoExcerpt` (present/absent or a short byte excerpt); `.tu/cache/` is never compared. The report renders the `tree` channel like any other red channel.

- **GIVEN** a multi-mode data case where the Go side wrote a day-file with different bytes
- **WHEN** `tudiff run` compares it
- **THEN** the case is red on channel `tree` naming that path
- **GIVEN** identical trees and both sides holding `.last-sync`
- **THEN** the case stays green

#### R13: Scripted git failures ride the `env` axis; the matrix covers sync
The `env` axis SHALL accept `pullfail`, `pushfail`, `dirty`; `BuildEnv` appends `TUDIFF_GIT_SCRIPT` with the rule sets in intake § 9 for those values and nothing else changes for them. `harness/matrix.json` SHALL carry the six groups of intake § 9 (`sync-cmd` with the four envs, `sync-dry-run` with `tz` fixed+alt, `sync-flag`/`cc-sync` with `default`+`pullfail`, `cc-sync-json`, `sync-json`); `TestCommittedMatrix`'s 200..600 rail holds. `just go-diff --placeholder` MUST report every case green with no `tree` red and no `[calls differ]` on any sync group.

- **GIVEN** the extended matrix
- **WHEN** `bin/harness/tudiff run --placeholder --list` runs
- **THEN** 438 case IDs are printed and every prior ID is unchanged
- **WHEN** `just go-diff --placeholder` runs
- **THEN** the summary is 438/438 green

#### R14: `tudiff live` — real-git parity
`tudiff live [--node] [--go] [--turepair] [--harness-bin] [--report] [--keep]` SHALL run the intake § 10 sequence: preflight (real git, node, the binaries), a bare seed repo on `main` from `harness/metrics-repo/` copied into two identical bares, one `multi` staged home per side whose `.tu/metrics_repo` is a real clone, a child PATH of a livebin holding only the fake `ccusage`, pinned identity/config/date env; the steps `sync --dry-run` (then both trees clean), `sync` (then day-file trees, `git status --porcelain`, `git log --format=%H%n%s -p main` byte-equal, `.last-sync` present on both), a second `sync` (log unchanged), a foreign commit pushed to each bare followed by `sync` with a one-off fixture alias raising cc 2026-01-07 (log identical, pull integrated), a fabricated `.git/rebase-merge` followed by `sync` (the recovery line), a broken `origin` followed by `sync` (the pull-failure bytes, exit 1), and `cc --sync`; then the repair parity flow (the R11 fixture copied twice, `node scripts/repair-metrics.mjs` vs `bin/turepair`, dry-run then `--write`, repo paths normalized to `$REPO`, trees compared). Each step is compared with `harness.Compare` after home normalization and reported in the `run` report shape under `bin/harness/report-live/`; exit 1 on any red.

- **GIVEN** `just build go-build harness-build` has run
- **WHEN** `just go-live` runs
- **THEN** every step is green and the exit is 0

#### R15: Build and CI
`just go-build` SHALL also write `bin/turepair`; `just go-live *ARGS` (depends on `build go-build harness-build`) SHALL run `bin/harness/tudiff live {{ARGS}}`; `.github/workflows/ci.yml`'s `go-diff` job SHALL gain a `continue-on-error: true` step running `just go-live` after the existing harness step. No new module dependency.

#### R16: Entry-point documentation
`cmd/tu/main.go`'s package comment and `runCommand`'s comment SHALL list `sync` (and `--sync`, `--dry-run`) as answered for real, leaving only `--watch` and its companions on the placeholder; `internal/sync`'s package comment SHALL describe the shipped package.

### Non-Goals

- `--watch`/`-w`, `--interval`, `--no-rain` — B7.
- Any behavior change to the fixed `origin main` branch, the UTC commit date, the over-predicting `WouldCommit`, the caller-less `Stale`, or `auto_sync` — G0 decisions.
- Editing `docs/specs/`, `src/node/`, `scripts/`, or TS-facing memory.
- Resolving the repair usage line's `node scripts/…` spelling — after Z1.

### Design Decisions

#### Returned lines instead of writers below the edge
**Decision**: `SyncMetrics`/`FullSync` return their stderr lines (and the fetch's typed warnings) in order; `cmd/tu` writes them.
**Why**: Nothing prints below `cmd/tu` (G1 checklist item 4); the TS emits these lines from inside the flow, but no other output can interleave, so returned lines reproduce the bytes exactly.
**Rejected**: Passing an `io.Writer` into `sync` — makes `sync` a writer and breaks the edge invariant.
*Introduced by*: 260916-lsml-sync-metrics-writer

#### One `FullSync` with a `dryRun` flag
**Decision**: A single function runs fetch → per-tool `Write(dryRun)` → (dry) local git half / (live) `SyncMetrics` + touch.
**Why**: Toolkit principle №5 and the 260717-xuhk decision — the preview must share the live decision path; two functions would drift.
**Rejected**: Separate `Preview`/`Sync` functions sharing helpers.
*Introduced by*: 260916-lsml-sync-metrics-writer

#### The writer reaches `command` through `Deps.Writer`
**Decision**: `command` sees the writer as a one-method interface; `sync.Writer{Dir}` adapts the package function; the edge assigns it.
**Why**: `command` stays filesystem-free and testable with a recording fake, exactly like `Repo` and `Fetcher`.
**Rejected**: Importing `sync` from `command` (couples the composer to an I/O package the G1 gate keeps at the ends).
*Introduced by*: 260916-lsml-sync-metrics-writer

#### `turepair` is a maintainer binary, not a `tu` subcommand
**Decision**: The repair twin ships as `cmd/turepair` over `sync.Repair`, built by `go-build`, never in the formula's grammar.
**Why**: The Goal freezes the CLI grammar and `--help`; the mjs was likewise outside `dist/tu.mjs`; the `tudiff` precedent exists for repo-run maintainer tools.
**Rejected**: A hidden `tu repair-metrics` subcommand (grammar change) or leaving the mjs as the only helper (dies with Z1).
*Introduced by*: 260916-lsml-sync-metrics-writer

#### Git failure scripting rides the `env` axis
**Decision**: `pullfail`/`pushfail`/`dirty` are `env` values setting `TUDIFF_GIT_SCRIPT`.
**Why**: The axis means "which extra environment variable is set"; a new axis would rename every existing case ID.
**Rejected**: A fifth `git` axis.
*Introduced by*: 260916-lsml-sync-metrics-writer

#### `TouchLastSync` creates the state dir
**Decision**: `MkdirAll(stateDir)` precedes the write.
**Why**: The TS would crash with an uncaught error there (a path no harness case reaches); Constitution II prefers degrading to crashing.
**Rejected**: Reproducing the crash.
*Introduced by*: 260916-lsml-sync-metrics-writer

## Tasks

### Phase 1: Setup

- [x] T001 Export `DayFile`, `Name`, `Path` in `src/go/internal/source/metrics/metrics.go`; switch `readDayFile` to `DayFile`; add marshal tests in `metrics_test.go` pinning the seed bytes and the float shapes (`0.5`, `211.8`, `1e-7`, `1e+21`, `0.1+0.2`) against Node-verified strings <!-- R1 -->
- [x] T002 [P] Add `Runner` and `Exec.Run` to `src/go/internal/sync/git.go` with the Node error-text reproduction; extend `git_test.go` (PATH-first shell-script fake: exit 1 with stderr, exit 1 without stderr, empty PATH → `spawn git ENOENT`, stdout returned on success, argv `-C dir` first) <!-- rework: review cycle 1 must-fix: Exec.Run must append "\n"+stderr UNCONDITIONALLY on a non-zero exit (Node always emits `Command failed: {cmd}\n{stderr}`, trailing newline even when stderr is empty); fix git.go:66-69 + its doc comment, and flip TestRunFailureNoStderr to expect the trailing newline --> <!-- R4 -->

### Phase 2: Core Implementation

- [x] T003 Create `src/go/internal/sync/writer.go` (`Action`, `Decision`, `Write`, `shrinkState` with the `Number()` coercion table) and `writer_test.go` porting `sync.test.ts` § writeMetrics + § dry-run one for one, plus the coercion table cases <!-- R2 -->
- [x] T004 [P] Create `src/go/internal/sync/state.go` (`CommitMessage`, `LastSyncFile`, `StaleAfter`, `TouchLastSync`, `Stale`) and `state_test.go` (UTC date, `.000Z` format + newline round-tripping through `config.LastSync`, missing dir created, the four `Stale` cases from `sync.test.ts` § isStale) <!-- R3 -->
- [x] T005 Create `src/go/internal/sync/flow.go` with `SyncMetrics` and `flow_test.go`: a recording fake `Runner` (rebase-recovery line + abort, missing user dir skips `add`, dirty status commits with the exact message, commit failure silent false, pull failure line text + abort, single push retry, both-pushes-fail line) plus real-git tests against `git init --bare --initial-branch=main` (push on success, clean no-op, non-git dir false, upstream change integrated via pull, fabricated `rebase-merge` recovery, a dir with spaces) with identity/`gpgsign` pinned via `GIT_CONFIG_*`, `GIT_CONFIG_GLOBAL=/dev/null`, `GIT_CONFIG_NOSYSTEM=1` <!-- R5 -->
- [x] T006 Add `Fetcher`, `Inputs`, `Outcome`, `FullSync` to `flow.go` and `ToolReport`, `Report`, `Report.Format` in `src/go/internal/sync/report.go`; tests in `flow_test.go` (R6 scenarios with the seed copy, a fake `Fetcher`, a fake `Runner`: dry-run touches nothing and runs only the status; live writes/syncs/touches; fetch warnings carried in registry order) and `report_test.go` (the R7 line rules incl. `Would write 0 …`, no skip block, a relative `metrics_dir`, `FixedHalfUp` without separators on a 1234.5 cost) <!-- R6 -->
- [x] T007 Add `sync.Writer{Dir}` (in `writer.go`); add `command.Writer` + `Deps.Writer` in `src/go/internal/command/run.go`; make `gatherOwn` write per tool before `Repo.Read`; admit `Flags.Sync` in `inScope`; extend `run_test.go` with a recording fake `Writer` (own-user path writes then reads per tool; `-u other`/`-u all`/`lb`/single never write; nil Writer no-op; Writer error surfaces) <!-- R8 -->
- [x] T008 Wire `src/go/cmd/tu/main.go`: `Deps.Writer = metricsync.Writer{Dir: cfg.MetricsDir}` with the compile-time assertions, the `--sync` block (R9), the `runCommand` `sync` case (R10), the package/row-map comments (R16); flip the `sync` placeholder rows in `e2e_test.go`/`main_test.go` into the R9/R10 assertions (capture the dry-run bytes from `node dist/tu.mjs sync --dry-run` on an identically staged multi home — `harness.StageHome` + the placeholder fixtures — and pin them; `TUDIFF_GIT_SCRIPT` pull/push/dirty variants; six ccusage calls on `cc --sync`; single-home no-op; `sync --json`; the clone-failed marker) <!-- R9 -->
- [x] T009 Create `src/go/internal/sync/repair.go`, `localecmp.go`, `repair_test.go`, `localecmp_test.go` (Node-verified ordering table incl. the `Sahils-Mac-mini.local` vs `dev-ws-sahil02` case and the per-user byte order) and `src/go/cmd/turepair/main.go` (+ `main_test.go` for arg parsing and exit codes); the repair test seeds the R11 temp history with real git and, when `node` is on PATH, also diffs against `scripts/repair-metrics.mjs` <!-- rework: review cycle 1 should-fix: failLines is duplicated in cmd/turepair/main.go:86 and internal/sync/repair.go:113 — export it from sync (or let sync.Repair own the arg-parse failure rendering) and delete the copy --> <!-- R11 -->

### Phase 3: Integration & Edge Cases

- [x] T010 Add `pullfail`/`pushfail`/`dirty` to the `env` axis in `src/go/internal/harness/matrix.go`, the `TUDIFF_GIT_SCRIPT` rule sets in `homes.go` `BuildEnv`, and tests in `matrix_test.go`/`homes_test.go` (axis validation, exact env strings, no other variable added) <!-- R13 -->
- [x] T011 Add the tree comparison: a `CompareTrees(nodeHome, goHome string) (channel-red details)` helper in `src/go/internal/harness/diff.go` (walk `.tu/metrics_repo`, compare path sets and bytes, `.last-sync` presence, skip `.tu/cache`), wire it into `runCase` in `src/go/cmd/tudiff/run.go` after `Compare` for green cases, render `Channel: "tree"` in `report.go`; tests in `diff_test.go`/`report_test.go` (identical trees green; extra file, byte diff, `.last-sync` mismatch each red with the path) <!-- R12 -->
- [x] T012 Edit `harness/matrix.json` per intake § 9 (six groups); confirm `TestCommittedMatrix` and `--list` show 438 cases with prior IDs unchanged; run `just go-diff --placeholder` and fix every divergence until 438/438 green with no `tree` red and no `[calls differ]` on the sync groups; paste the summary block into `## Notes` <!-- R13 -->
- [x] T013 Create `src/go/cmd/tudiff/live.go` + `live_test.go` implementing R14 (seeding helpers, per-step capture/compare, tree and log comparators, the repair parity flow, report dir); add `bin/turepair` to `just go-build`, the `go-live` recipe, and the `go-diff` job's second informational step in `.github/workflows/ci.yml`; run `just go-live` until every step is green and paste its summary into `## Notes` <!-- rework: review cycle 1 should-fix: cmd/tudiff/live.go copyDirTree/copyLiveFile/liveExcerpt duplicate internal/harness copyTree (homes.go:63) / copyFile (diff.go:164) / excerpt (diff.go:381) — export the harness helpers (e.g. harness.CopyTree/CopyFile/Excerpt) and delete the live.go copies; keep go-diff and go-live green --> <!-- R14 -->

### Phase 4: Polish

- [x] T014 Final gate: `just go-lint`, `just go-test`, `just go-diff --placeholder`, `just go-live` all green; `env -u TU_METRICS_REPO -u NO_COLOR npm test` unchanged (no TS edits); update the `internal/sync` package comment (R16); record the four command outputs' summary lines in `## Notes` <!-- R15 -->

## Execution Order

- T001 and T002 first (T003–T006 import `DayFile` and `Runner`)
- T003 → T006 → T007 → T008 (each builds on the previous)
- T004 and T005 can run alongside T003
- T009 needs T002 only
- T010 → T011 → T012 (the matrix run needs the env values and the tree channel); T012 needs T008
- T013 needs T008, T009 and T011 (it reuses the comparators)
- T014 last

## Acceptance

### Functional Completeness

- [x] A-001 R1: `metrics.DayFile`/`Name`/`Path` exist, the reader uses `DayFile`, and the seed-bytes marshal test passes
- [x] A-002 R2: `sync.Write` returns per-record decisions in both modes, writes only in live mode, skips silently on a lower incoming cost, and applies the `Number()` coercion table
- [x] A-003 R3: `CommitMessage`, `TouchLastSync` (dir created, `.000Z` + newline) and `Stale` behave as specified
- [x] A-004 R4: `Exec.Run` returns stdout on success and the exact Node-shaped error text on non-zero exit and on a missing binary — verified in review cycle 2: `git.go:66` now appends `"\n"`+stderr UNCONDITIONALLY (empty stderr still yields the trailing newline), and `TestRunFailureNoStderr` (git_test.go:158) pins the corrected bytes; `go test ./internal/sync` green
- [x] A-005 R5: `SyncMetrics` performs the five steps, returns the two warning lines in the right places, and prints nothing
- [x] A-006 R6: `FullSync` dry-run touches nothing and runs only the status; live writes, syncs and touches `.last-sync`
- [x] A-007 R7: `Report.Format` reproduces the layouts § 20 block including the zero-write and no-skip shapes without thousands separators
- [x] A-008 R8: `gatherOwn` writes per tool before the read through `Deps.Writer`; repo-only and single paths never write; `inScope` admits `Sync`
- [x] A-009 R9: `--sync` on a multi-mode data command prints `syncing metrics... ` then warnings/lines then `synced.` or the failure line, and is silent in single mode
- [x] A-010 R10: `tu sync` follows the eight-step order with the exact messages and exit codes; `--dry-run` prints the report to stdout without writing
- [x] A-011 R11: `turepair` output equals the mjs output byte for byte on the seeded fixture in dry-run and `--write` modes, including the `localeCompare` ordering
- [x] A-012 R12: `tudiff run` reports a `tree` channel red on a day-file or `.last-sync` difference and stays green on identical trees
- [x] A-013 R13: the `env` axis accepts the three new values with the exact scripts; the matrix expands to 442 cases (not 438 — the intake miscounted the pre-existing sync cases; ## Notes T012 records the recount) with prior IDs unchanged; `just go-diff --placeholder` is 442/442 green
- [x] A-014 R14: `just go-live` completes every step green, including the foreign-commit rebase and the repair parity flow
- [x] A-015 R15: `go-build` writes `bin/turepair`, `go-live` exists, `ci.yml` carries the informational step, `go.mod` is unchanged
- [x] A-016 R16: `main.go` comments and the `sync` package comment describe the shipped state

### Behavioral Correctness

- [x] A-017 R8: a multi-mode `cc h --since 2026-01-01 --until 2026-01-31` renders the same bytes as before the change (the write cannot alter the rendered max-merge) while now leaving the written day-files behind
- [x] A-018 R9: `tu --sync` on a cold cache makes exactly six ccusage calls (the data fetch hits the cache the sync warmed)
- [x] A-019 R10: the reserved-user check on `tu sync` fires before the mode check (exit 2 in single mode too)

### Scenario Coverage

- [x] A-020 R2: every `sync.test.ts` § writeMetrics / § dry-run case has a Go twin that passes
- [x] A-021 R5: the real-git `SyncMetrics` tests (push, no-op, non-git, upstream integrate, rebase recovery, spaces in path) pass
- [x] A-022 R10: the e2e `sync --dry-run` bytes were captured from `node dist/tu.mjs` on an identically staged home and pinned
- [x] A-023 R14: `tudiff live`'s `git log --format=%H%n%s -p main` outputs are byte-identical between sides after the foreign-commit sync

### Edge Cases & Error Handling

- [x] A-024 R2: absent, empty, unparseable, `null`, numeric-literal, missing-key and string-valued `totalCost` files are handled per the coercion table
- [x] A-025 R5: a commit failure returns false with no warning line; a pull failure appends its line and runs `rebase --abort`; a second push failure appends its line
- [x] A-026 R9: a `Write` filesystem error surfaces as the error text on stderr with exit 1 (documented unmatchable path)
- [x] A-027 R10: a demoted config (fresh `.clone-failed`) exits 1 after the guard's warning with no git call

### Code Quality

- [x] A-028 Pattern consistency: new Go code follows the package layout, comment style (`// Name is …` doc comments citing the TS twin), error-wrapping and test conventions of `internal/sync`, `internal/config` and `internal/harness`
- [x] A-029 No unnecessary duplication: `render.FixedHalfUp`, `config.Tildefy`/`StateDir`/`ExpandHome`/`LastSync`, `harness.StageHome`/`Compare`/`NormalizeHome`/`BuildEnv` and the existing fake-git test helpers are reused; no second float formatter or path tildefier
- [x] A-030 Functions and plain values over classes; no result globals; no function over ~50 lines without a clear reason
- [x] A-031 Magic strings named: the git argv verbs, the message strings, `StaleAfter`, `CENT_TOLERANCE`, the day-file regexp live as named constants
- [x] A-032 No silently swallowed errors beyond the TS-mandated ones (each swallow carries a comment naming the TS line it mirrors)
- [x] A-033 Minimum pathways: one `Write` for live and dry-run, one `FullSync`, one `CommitMessage`; the edge builds one `*ccusage.Source` shared by the sync and the data fetch
- [x] A-034 `gofmt -l` prints nothing and `go vet ./...` is clean under `src/go/`
- [x] A-035 Test-alongside: every new `.go` file has a `_test.go` sibling; no `__tests__/` under `src/go/`

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`
- Byte references for the e2e assertions come from `node dist/tu.mjs` (v24, placeholder corpus + committed seed, `TZ=UTC`, piped) — the B5 precedent; never from the spec text.
- Worktree hygiene: run `env -u TU_METRICS_REPO -u NO_COLOR` in front of any `npm test`; `npm ci` has already been run here.
- T012 harness summary (`just go-diff --placeholder`, 2026-09-17). Note: the six intake § 9 groups expand to +22 cases (not +18 — the intake counted 16 pre-existing sync cases; there are 12), so the matrix lands at 442 with every prior case ID unchanged:

  ```
  tudiff: 442 cases — 442 green, 0 red, 0 timeout   (fixtures: _placeholder; 167 cases replayed unconfirmed fixtures)
    by conf:  single 174/174  multi 174/174  org 47/47  legacy 47/47
    by env:   default 360/360  nocolor 34/34  envrepo 32/32  pullfail 8/8  pushfail 4/4  dirty 4/4
    by io:    pipe 368/368  tty 74/74
    by tz:    fixed 375/375  alt 67/67
  ```

- T013 live summary (`just go-live`, 2026-09-17). All nine steps green on both sides, exit 0; the pull-failure step's bytes carry the real git stderr verbatim (`fatal: '<tmp>/missing.git' does not appear to be a git repository…`) under the `$HOME`-normalized `git -C $HOME/.tu/metrics_repo pull --rebase origin main` line, and the repair steps' `$REPO`-normalized outputs and post-`--write` trees match byte for byte:

  ```
  GREEN   sync-dry-run
  GREEN   sync
  GREEN   sync-again
  GREEN   foreign-sync
  GREEN   rebase-recovery
  GREEN   pull-failure
  GREEN   cc-sync
  GREEN   repair-dry-run
  GREEN   repair-write
  tudiff: 9 cases — 9 green, 0 red, 0 timeout   (fixtures: _placeholder, live-alias; 0 cases replayed unconfirmed fixtures)
    by conf:  single 0/0  multi 9/9  org 0/0  legacy 0/0
    by env:   default 9/9  nocolor 0/0  envrepo 0/0  pullfail 0/0  pushfail 0/0  dirty 0/0
    by io:    pipe 9/9  tty 0/0
    by tz:    fixed 9/9  alt 0/0
  ```

- T014 final gate (2026-09-17, run by the apply worker on the finished tree): `just go-lint` clean (gofmt prints nothing, `go vet ./...` silent); `just go-test` → all 22 packages `ok` (`gofmt -l . && go vet ./... && go test ./... -count=1` green, incl. the real-git sync/repair/live tests, none skipped); `just go-diff --placeholder` → `tudiff: 442 cases — 442 green, 0 red, 0 timeout` (single 174/174, multi 174/174, org 47/47, legacy 47/47; no `tree` red, no `[calls differ]` on the sync groups); `just go-live` → `tudiff: 9 cases — 9 green, 0 red, 0 timeout`, exit 0; `env -u TU_METRICS_REPO -u NO_COLOR npm test` → `pass 1091, fail 0` (unchanged — no TS edits).

## Deletion Candidates

- None — this change adds new functionality without making existing code redundant. The `sync`/`--sync`/`--dry-run` placeholder routing it replaces was removed in place (`cmd/tu/main.go` `runCommand`'s `default` arm remains as defense for nothing the grammar names — only unreachable requests fall through, so it is not dead code); `notImplementedMsg` and `ErrUnported` stay live for B7's watch surface. The cycle-1 parsimony findings (`copyDirTree`/`copyLiveFile`/`liveExcerpt` in `cmd/tudiff/live.go`, `failLines` in `cmd/turepair/main.go`) were resolved in rework by exporting `harness.CopyTree`/`CopyFile`/`Excerpt` and `sync.FailLines`; cycle 2 re-asked the question against the finished tree and found nothing further (the one remaining look-alike, `liveFilePresent` in `cmd/tudiff/live.go:978` vs. the unexported `filePresent` in `internal/harness/diff.go`, is a should-fix reuse finding in the review result, not a deletion).

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | The plan carries the intake's 16 graded decisions unchanged; no requirement needed a new decision | Intake § What Changes fixes every string, signature and ordering | S:85 R:85 A:90 D:90 |
| 2 | Confident | The dry-run report's `N` on the multi home is derived at apply time from the placeholder corpus (six tools × three dates minus the one skip), not hard-coded in this plan | Only the node oracle's bytes are authoritative; the plan names the shape and the known cc lines | S:65 R:90 A:80 D:75 |
| 3 | Confident | `tudiff live` raises the foreign-commit step's local change through a one-off fixture alias rather than editing the committed corpus | The corpus is shared data; a per-run temp alias keeps the committed placeholder untouched | S:60 R:85 A:75 D:70 |
| 4 | Confident | `CompareTrees` treats `.last-sync` by presence only and compares every other file under `.tu/metrics_repo` by bytes | The timestamp is wall-clock; day-file bytes carry no path or time | S:70 R:90 A:85 D:80 |

4 assumptions (1 certain, 3 confident, 0 tentative).
