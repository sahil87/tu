# Intake: Metrics-Repo Source and Multi-Mode Merge (Go port row B3)

**Change**: 260916-xivf-metrics-source-and-multi-mode
**Created**: 2026-09-17

## Origin

One-shot `/fab-new` invocation, handed over from the Go-port plan's Phase 2 queue (plan row B3 — the second Phase 2 row, after B8 #89; V1 #84, V2 #85, B1 #86, B2 #87 and the G1 rework #88 are merged and G1 returned `GO` on 2026-09-17):

> Context: fab/plans/sahil/26-09-15-go-port.md, row B3. Read the Decisions and Target architecture sections before writing the intake. Do not change any external surface listed under Goal. Build source/metrics reader, multi-mode merge (self-view own-snapshot max-merge), -u user filter, three-month floor/cap rules, all-users aggregate. Harness gate: multi-mode snapshot and history against a fixture metrics repo.

No prior discussion in this conversation. Sources read to ground every value below: the plan's Decisions (D1–D13, in particular D2, D4, D6, D11), Target architecture table, § Execution, § G1 review protocol and § Risks; the G1 verdicts `fab/plans/sahil/reviews/g1-2.md` (follow-ups 3–6); the Go-port memories `docs/memory/go-port/{command-edge,fact-and-sources,query-view-render,config-and-setup}.md` and the code they describe (`src/go/internal/{command,config,fact,query,source,source/ccusage,source/cache,sync,view,render/json}`, `src/go/cmd/tu/main.go`); the TS memories `docs/memory/sync/multi-machine.md` (JSONL layout, never-shrink guard, self-view max-merge, `listUsers`, `readRemoteEntriesByMachine`, the all-users design decision) and `docs/memory/cli/data-pipeline.md` (`-u`, `-u all`, the reserved `all` profile, the own-user merge pipeline, the 3-month cap); the specs `docs/specs/usage.md` (§ Global Flags `--user`, § Data Flow, § Multi-Machine Mode — Configuration, Metrics Repo Layout, Own-machine max-merge, Auto-Clone Guard, Staleness — and DC-19/DC-20); the harness memory `docs/memory/harness/differential-harness.md`, `harness/matrix.json`, `harness/metrics-repo/` (the committed seed) and `src/go/internal/harness/{homes,diff}.go` plus `src/go/cmd/fakegit/main.go`; and the shipped TypeScript — `src/node/core/cli.ts` (`checkMetricsDirGuard`, `isCloneMarkerFresh`, `writeCloneMarker`, `fetchToolMerged`, `fetchToolMergedWithMachines`, `readAllUsersByUser`, `dispatchAllSnapshot`, `dispatchAllHistory`, `dispatchSingleTool`, `main()`), `src/node/sync/sync.ts` (`listUsers`, `readRemoteEntriesByMachine`, `readRemoteEntries`, `writeMetrics`), `src/node/core/fetcher.ts` (`mergeEntries`, `maxMergeEntries`). A full `just go-diff --placeholder` run on this worktree (2026-09-17, after `npm ci`) reproduced the post-B8 baseline — **144 green of 368** — and supplied the node-side reference bytes and call logs quoted in §9 from `bin/harness/report/cases/`.

Plan context that shapes this change:

- **D2 / Goal** — Go lands dark; the formula is untouched; every external surface is frozen. B3 makes the Go binary answer **multi mode** for the snapshot and history displays — the own-user merged view, `-u <other-user>`, `-u all`, the single-mode `-u` warning, and the auto-clone guard's stderr lines — byte for byte against the TS.
- **Target architecture** — `source/metrics` is the second adapter under `source` ("read the metrics repo"), the only new package that touches the filesystem; the max-merge and the cross-machine sum are pure `query` functions; `command` composes; `cmd/tu` stays the only writer. The G1 checklist (package boundaries, one group-by, no result globals, typed errors at the edge, table-driven tests) binds B3 exactly as it bound V1–B2.
- **D11 and the B3/B6 split** — the plan gives B3 the *reader* and B6 the *writer* (never-shrink guard, day-file layout, commit, dry-run). The TS writes day-files on every multi-mode fetch of the own user before it reads them back; §2 records why that write stays in B6 and why the harness gate is unaffected.
- **Row text predates B2** — "three-month floor / cap rules" already landed with B2 (`command.Normalize` step 3 plus `query.ThreeMonthFloor`). B3 changes nothing there; §5 states what it verifies instead.
- **D6** — the harness is the gate. §2 enumerates the 158 case IDs that flip green (144 → 302 of 368).

## Why

Multi mode is the reason tu has a metrics repo at all: it is what turns six per-machine ccusage views into one per-user (and, with `-u all`, one per-org) view, and it is the surface the field failure in the backlog (`0bb81ca`, the 2026-05-30 history destruction) taught the project to treat as a wire protocol between machines on different versions. Every later Phase 2 row builds on it: B4's machine columns and B5's leaderboards are pivots over repo records (`GroupBy(Machine)`, `GroupBy(User)`), and B6's sync writes the files this row learns to read. Until B3 lands, 127 of the harness's 155 non-single cases — every `multi`, `org`, `legacy` and `envrepo` data case — are red purely because `command.Run` returns `ErrUnported` the moment `config.Mode == Multi`, so the burndown cannot move past 144 whatever else is ported.

The interesting part is not the file walk; it is reproducing the TS merge arithmetic exactly. The own machine's live fetch and its own stored day-files are reconciled per date by **whole-entry max** (never field-wise, never summed — Constitution V and the 260610-srmi post-mortem), the other machines are **summed** onto that, and only then is the result windowed and rolled up. Costs are floats printed raw in `--json`, so the *order* of additions is part of the contract: the TS sums per day across machines first and rolls days into weeks or months second, and the Go pipeline has to associate the same way or a `2.15` can become `2.1500000000000004`. The existing `RollUp` → `GroupBy` order would associate per machine first; §4 inserts one daily-level collapse (still through the one `GroupBy`) so the arithmetic matches. Doing the reader, the merge, the `-u` paths and the clone guard in one row is what makes the gate meaningful — the 158 cases exercise the populated own-user merge (both arms of the max-merge, via the seed's `0.25`/`0.75` cc day-files straddling the placeholder's `0.50`), the repo-only paths, the single-mode warning and the fake-git clone under the `envrepo` axis; splitting would leave most of them red until the second half.

## What Changes

### 1. Package layout (deltas on the post-B8 tree)

```
src/go/
  cmd/tu/
    main.go                     metrics-dir guard between config.Load and the reserved-user check; Repo + stamped
                                ccusage Source in Deps; Run takes the Config
    main_test.go                placeholder list narrowed (multi-mode data cases now succeed)
    e2e_test.go                 + multi/org/legacy/envrepo snapshot + history, -u other/all/single-warn, clone-guard cases
  internal/
    source/
      metrics/                  NEW adapter: Source{Dir} — Users(), Read(user, tool) walking the JSONL tree (read-only)
        metrics.go, metrics_test.go (temp-dir trees, table-driven)
    query/
      merge.go                  NEW: MaxMerge(a, b) whole-record per-key max (ties → a); Collapse(recs, dims...) via GroupBy
      merge_test.go
    config/
      guard.go                  NEW: MetricsDirGuard(cfg, stateDir, now, git) (Config, []string); clone-marker read/write
      guard_test.go             fake Cloner, fixed now, temp stateDir
      setup.go                  CloneFailedMarker/RemoveCloneMarker unchanged (already here)
    sync/
      git.go                    + Exec.CloneQuiet(ctx, url, dir) — 30 s deadline, GIT_TERMINAL_PROMPT=0, streams captured
    command/
      request.go                Deps gains Repo; a Repo interface (Users/Read) beside Fetcher
      guards.go                 Normalize(req, mode, now): the single-mode -u warn-and-clear notice, first
      run.go                    Run(ctx, req, cfg config.Config, deps); inScope admits multi + User; gather() composes
                                the four record paths; Collapse before Window/RollUp; label rule keyed on Single
      run_test.go               + fake Repo; multi-mode and -u Run tests
```

No edit to `src/node/**`, `docs/specs/*`, `harness/**` (matrix and seed unchanged), `justfile`, `.github/workflows/*`, `Formula/`, `package.json`, or the plan document. `just go-build`/`go-test`/`go-lint` and the `go-build-and-test` CI lane pick the new packages up through `./...`.

### 2. Scope and the harness gate

**In scope (produces real output)** — a data command with `Display ∈ {Snapshot, History}`, `Format ∈ {Table, JSON, CSV, Markdown}`, any `Source`, any `Period`, in **single or multi mode**, with flags limited to the B2 set (`--json`/`-j`, `--csv`, `--md`, `--fresh`/`-f`, `--no-color`, `-t`, `--metric`, `--since`/`-s`, `--until`, `--full`) **plus `--user`/`-u <user>`** in any mode. `inScope` drops the `mode != Single` and `f.User == ""` rejections; everything else it rejects today stays rejected.

**Still recognized-but-unported (placeholder, exit 1)**: `--by-machine` (B4 — including `-u all --by-machine`'s `Users:` legend and the pivot warn-and-clear); `lb`/`lbh`/`--top` (B5 — including the `Error: lb requires multi mode …` exit-1 guard and the leaderboard `-u` pin/no-op semantics, which the TS applies only on those displays); the `sync` command, `--sync`, `--dry-run` (B6); `--watch`/`-w`/`--interval`/`--no-rain` (B7). Combinations of an in-scope request with any of these keep the placeholder without printing notices, exactly as today.

**The gate.** These **158** expanded IDs from `just go-diff --placeholder` MUST be green (all currently red with `exit: node=0 go=1`, most also `[calls differ]`):

```
snapshot-all, snapshot-cc                        /{multi,org,legacy}/{default,nocolor,envrepo}/{pipe,tty}/{fixed,alt}   (72)
snapshot-all, snapshot-cc                        /single/envrepo/{pipe,tty}/{fixed,alt}                                  (8)
snapshot-{codex,co,oc,gemini,gem,copilot,cop,kimi,ki,w,m,cc-m}   /multi/default/{pipe,tty}/fixed                         (24)
snapshot-all-{json,json-short,csv,md,truncate,metric-tokens,metric-cost,fresh}   /multi/default/pipe/fixed              (8)
snapshot-cc-{json,json-short,csv,md,truncate,metric-tokens,metric-cost,fresh}    /multi/default/pipe/fixed              (8)
h, dh, wh, mh, history, cc-h, cc-mh              /multi/default/pipe/fixed                                               (7)
{h,dh,wh,mh,history,cc-h,cc-mh}-window           /multi/default/pipe/{fixed,alt}                                         (14)
{h,dh,wh,mh,history,cc-h,cc-mh}-full             /multi/default/pipe/fixed                                               (7)
h-json, h-csv, h-md, cc-mh-json, cc-mh-csv, cc-mh-md   /multi/default/pipe/fixed                                        (6)
user-other, user-all                             /{single,multi}/default/pipe/fixed                                      (4)
```

(`user-missing` in both confs is already green — `-u` without a value is a parse error before config is read.) Expected summary after B3: **302 green of 368** (144 + 158); by conf `single 135/155`, `multi 107/127`, `org 30/43`, `legacy 30/43`; by env `envrepo 32/32`. The 66 that stay red are all owned by later rows: `lb*`/`*-lb`/`lbh*` (B5, 40), `sync-cmd`/`sync-dry-run`/`sync-flag`/`cc-sync` (B6, 12), `*-by-machine*` (B4, 10), (44 + 12 + 10). Any other red is a B3 defect.

**What the fixtures exercise.** The seed `harness/metrics-repo/` (copied to `~/.tu/metrics_repo/` for `multi`/`org`/`legacy`; absent for `single`) holds `harness-user/2026/harness-machine/cc-2026-01-05.jsonl` (`0.25`), `…/harness-machine/cc-2026-01-06.jsonl` (`0.75`), `harness-user/2026/other-box/cc-2026-01-06.jsonl` (`0.40`), `…/other-box/codex-2026-01-07.jsonl` (`0.30`), `other-user/2026/laptop/cc-2026-01-05.jsonl` (`1.10`), `…/laptop/gemini-2026-01-06.jsonl`, and `docs/README.md`; every entry carries the placeholder token counters (`3000/400/1000/20000/24400`). The conf pins `machine = harness-machine`, `user = harness-user`. Against the placeholder live fetch (`0.50` per tool per day on 2026-01-05..07) the `-window`/`-full`/`mh` cases therefore render a **populated merged** view: cc 2026-01-05 stays `$0.50` (live `0.50` beats stored `0.25`), cc 2026-01-06 is `$1.15` (stored `0.75` beats live `0.50`, plus other-box `0.40`; tokens `6,000/800/2,000/40,000/48,800`), codex 2026-01-07 is `$0.80` (`0.50` + `0.30`); cc month `$2.15`, grand total `$9.95`. The bare snapshots and capped `h` stay empty ("today" is not in January); `user-other`/`user-all` snapshots are empty for the same reason and prove only the repo-only path and the absence of ccusage calls. The `envrepo` axis (`TU_METRICS_REPO` set, dir absent) proves the auto-clone guard: one `git clone <url> <dir>` call, `Cloned metrics repo → <abs dir>` on stderr, then a live-only multi-mode render.

**The write that is not in B3.** In the TS, every multi-mode fetch of the own user calls `writeMetrics(metricsDir, user, machine, toolKey, local)` (never-shrink guarded) *before* reading the repo back. The plan assigns the writer, the guard and the day-file layout to B6 (D11: "port faithfully; no redesign", gated on live-sync parity). B3 therefore **reads only**: a Go `tu` in multi mode creates no directory and writes no file until B6 lands. This is invisible to the gate — `tudiff` byte-diffs stdout, stderr and exit and compares the git/ccusage call multiset, none of which the write touches — and invisible to the rendered bytes: write-then-max-merge is arithmetically identical to max-merge alone (a stored file only ever holds a value the guard let through, and the guard lets through exactly the values that would win the max). It is a **temporary, documented divergence** from the spec sentence "a plain `tu` in multi mode is a write", closed by B6; the memory file records it so nobody mistakes the read-only edge for the final shape.

### 3. `source/metrics` — the repo reader (I/O, read-only, silent)

```go
// Package metrics reads the metrics repo clone: the second adapter under
// source. It walks {Dir}/{user}/{year}/{machine}/{tool}-{date}.jsonl and
// returns []fact.Record stamped with User = the profile directory and
// Machine = the machine directory. It never writes (B6 owns the writer),
// never prints, and never returns an error: a missing or unreadable
// directory or file is simply absent data, exactly as the TS readers
// swallow every fs error (the repo's absence is reported once, by the
// metrics-dir guard).
package metrics

type Source struct{ Dir string }

// Users lists the profile directories: direct children of Dir that are
// directories, excluding dot-prefixed names (".git") and the NON_USER_DIRS
// set {"docs"}, sorted ascending (byte order — the TS Array.sort on ASCII
// names). Missing/unreadable Dir → nil.
func (s Source) Users() []string

// Read returns user's records for tool across every machine, in walk order:
// year dirs ascending, machine dirs ascending, files ascending — each file
// whose name has prefix "{tool.Key}-" and suffix ".jsonl". Per file: read,
// TrimSpace, skip when empty; decode the single JSON object; skip silently on
// any decode error. The record's Date is the JSON "label" (NOT the filename
// date); Tool is tool.Key; User/Machine are the directory names; Totals are
// the six pinned keys (a missing key decodes as 0).
func (s Source) Read(user string, tool fact.Tool) []fact.Record
```

Walk details mirrored from `readRemoteEntriesByMachine`: the user path is checked for existence first (absent → nil); `readdir` failures at any level skip that level; only directories are descended (files at the year or machine level are ignored); the `excludeMachine` parameter of the TS is **not** ported — every production call site passes `null` and the TS memory flags it as a deletion candidate. Non-directory `docs` handling: `NON_USER_DIRS` applies to `Users()` only, as in `listUsers`.

Accepted, documented divergence: a day-file missing one of the six keys decodes to `0` in Go where the TS produces `NaN` through `existing.x += undefined`. Only a hand-corrupted file can trigger it; the never-shrink writer always emits all six keys.

### 4. `query` — the merge arithmetic (pure)

```go
// MaxMerge is the self-view high-water merge: per key (Date, Tool, User,
// Machine) it keeps whichever WHOLE record — from a or from b — has the
// greater TotalCost; on a tie the record from a wins. Never field-wise,
// never summed (Constitution V; the TS maxMergeEntries). Output: a's records
// in a's order (replaced in place when b wins), then b's unmatched records in
// b's order. Pure — inputs are never mutated or returned.
func MaxMerge(a, b []fact.Record) []fact.Record

// Collapse sums Totals per distinct tuple of dims and returns one record per
// group carrying only the grouped dims (the other string fields empty), in
// first-seen order — GroupBy's groups turned back into records. It is the TS
// mergeEntries: the daily cross-machine (or cross-user) sum that precedes
// the window filter and the period roll-up.
func Collapse(recs []fact.Record, dims ...Dim) []fact.Record
```

`Collapse` is a thin wrapper over the one `GroupBy` (G1 checklist item 2 — no second aggregation loop). Everything else B3 needs already exists: `Window`, `RollUp`, `GroupBy`, `CurrentLabel`, `ThreeMonthFloor`, `Totals.Add`.

**Why the collapse exists — summation order is contract.** The TS merged pipeline is `mergeEntries` (per-label sum across machines, in input order) → `filterEntriesByRange` → `aggregateForPeriod` (sum labels into weeks/months in label order). The B2 pipeline is `Window` → `RollUp` (per (Date', Tool, User, Machine) — per *machine* month sums) → `GroupBy(Tool, Date)` (sum machines). Both are correct sums but associate the float additions differently; `--json` prints costs raw, so `(a+b)+c` vs `a+(b+c)` is a visible byte. Collapsing to `(Tool, Date)` on the **daily** records first, then running the unchanged `Window` → `RollUp` → `GroupBy` tail, reproduces the TS association exactly: per day across machines in input order, then per period in date order. The collapse runs in single mode too (one record per key, so it is the identity) — one pathway, not two.

**Record order into the collapse is load-bearing for the same reason.** The own-user list is `MaxMerge(live, ownStored)` followed by the other machines' records in `Read`'s walk order (year asc, machine asc, file asc) — the TS `mergeEntries(effectiveLocal, remote)` where `remote` is the by-machine map's insertion order minus the own machine. The `-u <other>` list is `Read`'s walk order. The `-u all` list is `Users()` ascending, each user's walk order — the TS `readAllUsersByUser` flattened. Within a key, additions happen in that order.

### 5. The 3-month floor and cap — nothing to add

B2 landed the cap exactly as the TS `capApplies` + `threeMonthFloor`: in `Normalize`, history ∧ period ≠ monthly ∧ no explicit bound ∧ not `--full` → `Since = query.ThreeMonthFloor(now)`, `capActive = true`; an explicit bound on either side disables the cap; `mh --full` is a silent no-op; the heading carries `, last 3 months`. In the TS the defaulted `sinceFlag` flows into `fetchToolMerged`'s `filterEntriesByRange` on the **merged daily** entries, so repo-sourced records are windowed by the same floor as live ones. B3 satisfies that by construction: `Window` runs on the collapsed daily records after the merge (§6). The B3 tests assert it on the multi-mode path (a stored day-file older than the floor does not appear in `h`, does appear in `h --full`); no code changes in `guards.go` step 3 or `period.go`.

### 6. `config` + `sync` — the auto-clone guard (edge-adjacent I/O, returns lines)

```go
// Cloner is the one git question the guard asks. sync.Exec satisfies it.
type Cloner interface {
    // CloneQuiet runs `git clone <url> <dir>` with stdout/stderr captured,
    // GIT_TERMINAL_PROMPT=0 in the child env, and a 30 s deadline; returns
    // the child's stderr and the exec error (nil on exit 0).
    CloneQuiet(ctx context.Context, url, dir string) (stderr string, err error)
}

// MetricsDirGuard is the TS checkMetricsDirGuard: returns the config the
// data path runs with (Mode possibly demoted to Single) and the stderr
// lines the edge prints, in order. Pure apart from the marker file and the
// clone it delegates to git.
func MetricsDirGuard(cfg Config, stateDir string, now time.Time, git Cloner) (Config, []string)
```

Behavior, in TS order, keyed on `cfg.Mode == Multi && !exists(cfg.MetricsDir)` (an **existence** check — not `IsRepo`; the harness seed has no `.git/`):

1. `stateDir/.clone-failed` exists, parses as a timestamp (`time.Parse(time.RFC3339Nano)` on the trimmed content — the TS `new Date(raw)` on an ISO string), and is **younger than 3 h** → line `Warning: metrics repo not available — falling back to single mode.`; `Mode = Single`. A missing, unreadable or unparseable marker counts as stale.
2. Otherwise `git.CloneQuiet(ctx, cfg.MetricsRepo, cfg.MetricsDir)`:
   - exit 0 → line `Cloned metrics repo → {cfg.MetricsDir}` (absolute — DC-19), `RemoveCloneMarker(stateDir)`, config unchanged (**still Multi, even though the fake git created no directory** — the readers then find nothing, which is the harness `envrepo` case).
   - failure → write the marker (`stateDir` created, content `now.UTC().Format("2006-01-02T15:04:05.000Z")` — the TS `toISOString()`), line `Warning: could not clone metrics repo ({detail}) — falling back to single mode.`, `Mode = Single`. `{detail}` reproduces Node's `execFileSync` message: `Command failed: git clone {url} {dir}` followed by `\n{stderr}` when stderr is non-empty; on the deadline, `spawnSync git ETIMEDOUT`. <!-- assumed: the clone-failure detail follows Node's execFileSync error message shape; the fake git always exits 0 so no harness case pins these bytes — verify against the real binary before cutover (R2) -->
3. The TS branch for `!config.metricsRepo` (`Warning: metrics repo not found at … Run 'tu init-metrics' …`) is **unreachable** — `Mode == Multi` is derived from a non-empty `metrics_repo` — and is not ported.

`Exec.CloneQuiet` lives beside `Exec.Clone` in `internal/sync/git.go` (the interactive clone keeps its inherited stdin and no deadline; the guard's clone is the quiet one — the split `git.go`'s own comment reserved for B3). `config` gains no new import of `os/exec`; it sees git only through `Cloner`.

### 7. `command` — the composition

#### 7.1 Interfaces and `Deps`

```go
// Repo is what command needs from the metrics repo: the profile list and
// one user's records for one tool. metrics.Source satisfies it (asserted
// where cmd/tu assigns it). Distinct from Fetcher on purpose — a repo read
// has no context, no period, no cache, no fresh flag and no error channel.
type Repo interface {
    Users() []string
    Read(user string, tool fact.Tool) []fact.Record
}

type Deps struct {
    Source Fetcher   // live ccusage, stamped with cfg.User / cfg.Machine at the edge
    Repo   Repo      // the metrics clone; consulted only in multi mode
    Now    func() time.Time
    Colors ansi.Colors
    Width  int
}

func Run(ctx context.Context, req Request, cfg config.Config, deps Deps) (Result, error)
```

`Run` takes the resolved `config.Config` (the post-guard one) instead of the bare `Mode`: it needs `Mode`, `User` and `Machine`. `command` already imports `config`.

#### 7.2 `Normalize(req, mode, now)` — one new notice, first

The single-mode `-u` guard slots **before** step 1 (the TS prints it right after `assertUserNotReserved`, before every other guard): `f.User != "" && mode == config.Single` → notice `Warning: -u flag requires multi mode — ignoring.`, `f.User = ""`. It applies to `-u all` as to any name; the TS excludes only `lb`/`lbh`, which are placeholder here (B5 wires their exclusion). The guard order becomes: `-u` (B3) → [B4 pivot guard] → since/until → full → [B5 top] → cap.

#### 7.3 `gather` — the four record paths

```go
// gather returns the daily records the pipeline consumes plus the source
// errors, choosing the TS path by mode and -u:
//
//   single                         live fetch (as today)
//   multi, -u "" or -u == cfg.User live fetch → stored := Read(cfg.User, tool) per tool (walk order)
//                                  → own, others := split stored on Machine == cfg.Machine
//                                  → MaxMerge(live, own) ++ others
//   multi, -u all                  for u in Repo.Users(): Read(u, tool) per tool — no live fetch, no errors
//   multi, -u <other>              Read(user, tool) per tool — no live fetch, no errors
//
// then Collapse(recs, Tool, Date) — the daily cross-machine/user sum — before
// the caller's Window/RollUp/GroupBy tail (unchanged from B2).
```

Per-tool iteration is in registry order (`fact.Tools`, or the one requested tool). The repo-only paths make **no** ccusage call — the harness compares the call multiset, and `user-other`/`user-all` record zero ccusage calls on the node side. `-u <cfg.User>` is literally the no-flag path (the TS `targetUser && targetUser !== config.user` test). The live fetch keeps the B2 shape (`FetchAll` for all tools, `Fetch` for one; daily only; `Fresh` honored; errors → `Result.Warnings`).

#### 7.4 Snapshot label rule, corrected for mode

The B2 rule "the daily all-tools snapshot carries no `label`" is a **single-mode** artifact of `fetchAllTotals`. In multi mode the TS builds every snapshot from `fetchToolMerged` entries, so a tool with a record on the current label carries `"label"` in `--json` (zero tools carry nothing, as before). `runSnapshot` clears labels only when `cfg.Mode == config.Single && req.Source == "" && req.Period == query.Daily`.

#### 7.5 Everything after `gather` is B2

`runSnapshot` (`GroupBy(Window(RollUp(daily, p), cur, cur), Tool)`) and `runHistory` (`RollUp(Window(daily, since, until), p)` → one `GroupBy(Tool, Date)`) are untouched; `Result` keeps its shape (`Lines`, `Notices`, `Warnings`, `TotalCost`, `TotalTokens`, `CostByItem`). `ErrUnported` still marks the B4–B7 surfaces.

### 8. `cmd/tu` — the edge order

`run()` after `config.Load` becomes, in the TS `main()` order: (5) `Load` → its warnings; (5a) **`cfg, lines := config.MetricsDirGuard(cfg, config.StateDir(paths.Home), time.Now(), metricsync.Exec{})` → print `lines` to stderr** (the seam `main.go`'s comment reserved); (6) the reserved-user guard on `cfg.User`; (7) deps — `&ccusage.Source{Cache: store, User: cfg.User, Machine: cfg.Machine}` (live records now stamped, which B4's `GroupBy(Machine)` will rely on) and `metrics.Source{Dir: cfg.MetricsDir}`; (8) `command.Run(ctx, req, cfg, deps)`; (9) notices → warnings → lines, exit 0. The guard is skipped for `Command != ""` (setup commands: `status` never clones; `init-metrics` has its own interactive clone). Only `cmd/tu` names `ccusage` and `metrics`.

### 9. Byte-exact references (node v24, TS v0.11.5, placeholder corpus + committed seed, `TZ=UTC`, piped — from `bin/harness/report/cases/`)

`snapshot-all/single/envrepo/pipe/fixed` — stderr one line, stdout the empty table, exit 0; node call log = 1 git + 6 ccusage:

```
Cloned metrics repo → /tmp/tudiff-…/cases/snapshot-all-single-envrepo-pipe-fixed/node/home/.tu/metrics_repo
```
```
{"tool":"git","argv":["clone","git@example.invalid:harness/tu-metrics.git","/tmp/…/node/home/.tu/metrics_repo"]}
{"tool":"ccusage","argv":["claude","daily","--json"]}   … codex, opencode, gemini, copilot, kimi
```

`user-other/single/default/pipe/fixed` — stderr `Warning: -u flag requires multi mode — ignoring.`, then the live empty snapshot (6 ccusage calls), exit 0. `user-other/multi` and `user-all/multi` — empty stderr, empty snapshot, **no call log at all** (no ccusage, no git), exit 0.

`cc-h-window/multi/default/pipe/fixed` (ANSI stripped):

```
📊 Claude Code (daily)

Date         |          Input |         Output |    Cache Write |     Cache Read |          Total |      Cost
─────────────|────────────────|────────────────|────────────────|────────────────|────────────────|──────────
2026-01-05   |          3,000 |            400 |          1,000 |         20,000 |         24,400 |     $0.50
2026-01-06   |          6,000 |            800 |          2,000 |         40,000 |         48,800 |     $1.15
2026-01-07   |          3,000 |            400 |          1,000 |         20,000 |         24,400 |     $0.50
─────────────|────────────────|────────────────|────────────────|────────────────|────────────────|──────────
Total        |         12,000 |          1,600 |          4,000 |         80,000 |         97,600 |     $2.15
avg $0.72/day · peak $1.15 (2026-01-06)
```

`h-window/multi/default/pipe/fixed` rows: `2026-01-05 | $0.50 ×6 | $3.00`, `2026-01-06 | $1.15 | $0.50 ×5 | $3.65`, `2026-01-07 | $0.50 | $0.80 | $0.50 ×4 | $3.30`; Total `$2.15 | $1.80 | $1.50 ×4 | $9.95`; footer `avg $3.32/day · peak $3.65 (2026-01-06)`. `cc-mh-json/multi`: one object `"label": "2026-01", "totalCost": 2.15, "inputTokens": 12000, "outputTokens": 1600, "cacheCreationTokens": 4000, "cacheReadTokens": 80000, "totalTokens": 97600`. `h-json/multi`: the six-key object with `[]` values (capped window, no January). The `single` twins of every history case are unchanged from B2 (`$0.50` everywhere, `$9.00`).

### 10. Tests

- `source/metrics`: table-driven over temp-dir trees — the walk order, the `label`-not-filename rule, prefix/suffix filtering (`cc-` must not match `ccx-`), empty/whitespace/garbage files skipped, missing user dir, `Users()` exclusions (`.git`, `docs`, plain files), unreadable dir → nil.
- `query`: `MaxMerge` (a wins tie; b wins strictly greater; unmatched from both sides; whole record — b's tokens ride along with b's cost; inputs unmutated) and `Collapse` (order, dropped dims, identity on already-unique keys).
- `config`: the guard with a fake `Cloner` and fixed `now` — dir present → no-op; fresh marker → warning + Single, git never called; stale/garbage/missing marker → clone attempted; clone ok → `Cloned …` line, marker removed, still Multi; clone failure → warning with the composed detail, marker written with the pinned timestamp format, Single.
- `command`: `Run` with the fake `Fetcher` plus a fake `Repo` — own-user merge (both max-merge arms, other-machine sum, order of additions asserted through a tie-sensitive float triple), `-u other` and `-u all` make no fetch call, `-u <cfg.User>` equals no flag, single-mode `-u` notice ordering, multi-mode `--json` label presence, the cap on stored records (`h` vs `h --full`).
- `cmd/tu` e2e (existing harness fakes, staged homes, `TZ=UTC`): the `multi`/`org`/`legacy` populated `cc h --since … --until …` bytes above, `-u other-user`/`-u all` (no ccusage call recorded), `-u` in single mode, the `envrepo` clone path (git argv asserted via `TUDIFF_CALL_LOG`, `Cloned` line), and a fresh-marker fallback.
- Gate: `just go-diff --placeholder` at **302/368** with the §2 red set explained; `gofmt`, `go vet`, `just go-test` clean.

### 11. Explicitly not in B3

The day-file write (B6 — with the never-shrink guard, `writeMetrics` decisions, dry-run); `--by-machine` and per-machine roll-ups (B4 — `gather` leaves the un-collapsed records available to it); leaderboards and their `-u` pin/no-op and `lb requires multi mode` guard (B5); `--sync` and the `sync` command incl. its fallback warning + exit 1 (B6); watch (B7); `tu status`'s `NOT FOUND` line (already B1). No TS edit (D4), no spec edit (frozen), no matrix or seed edit.

## Affected Memory

- `go-port/multi-mode`: (new) the multi-mode composition end to end — metrics-dir guard, the four `gather` paths, `MaxMerge` + `Collapse` and the summation-order rationale, the label rule, the read-only-until-B6 divergence, what B4/B5/B6 inherit
- `go-port/fact-and-sources`: (modify) `source/metrics` as the second adapter (walk rules, silence, stamping); `ccusage.Source` stamped from config at the edge
- `go-port/query-view-render`: (modify) `MaxMerge` and `Collapse` in the query section; the one-group-by note extended
- `go-port/config-and-setup`: (modify) `MetricsDirGuard`, the clone marker's write/read, `sync.Exec.CloneQuiet` beside `Clone`
- `go-port/command-edge`: (modify) `Run(cfg)`, `Repo`, `Normalize(mode)` and the `-u` notice, the widened scope and shrunken `ErrUnported` map, the edge order with the guard, the 302 count

## Impact

- **Code**: new `internal/source/metrics`, `internal/query/merge.go`, `internal/config/guard.go`; edits to `internal/sync/git.go`, `internal/command/{request,guards,run}.go`, `cmd/tu/main.go` and their tests. Roughly 400 lines of implementation, more of tests.
- **Dependencies**: none new (stdlib only).
- **Harness**: 158 cases flip; the `envrepo` axis goes fully green; the call-multiset comparison becomes load-bearing for the first time on the ccusage side (repo-only paths must not exec).
- **Behavior contracts**: all frozen surfaces reproduced; one temporary, memory-documented divergence (no day-file write until B6).
- **Downstream rows**: B4 groups the un-collapsed records by `Machine`; B5 by `User` over `Repo.Users()`; B6 adds the writer call in `gather`'s own-user path before the `Read`.

## Open Questions

None blocking. The one unverifiable byte sequence (the clone-failure `{detail}`) is marked inline in §6 and carried as a Tentative assumption; the R2 dogfood window is where it gets checked against a real failing clone.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Scope is multi-mode snapshot + history, the three `-u` paths, the single-mode `-u` notice and the auto-clone guard; `--by-machine`, `lb`/`lbh`/`--top`, `sync`/`--sync`, watch stay placeholder | Plan row text plus the ErrUnported ownership map in command-edge memory name exactly these surfaces for B3 | S:90 R:85 A:95 D:95 |
| 2 | Confident | B3 reads only; the pre-fetch day-file write lands with B6's writer, a temporary divergence recorded in memory | Plan splits reader (B3) from writer + never-shrink guard (B6, D11); the harness compares streams and call multisets, and write-then-max-merge equals max-merge alone for the rendered bytes | S:70 R:85 A:80 D:70 |
| 3 | Confident | `metrics.Source` satisfies a new `command.Repo` (Users/Read), not `Fetcher` | A repo read has no ctx, period, cache, fresh flag or error channel; forcing it through Fetcher would mean ignored parameters. run.go's comment predicted Fetcher, so the plan may revisit | S:60 R:90 A:80 D:65 |
| 4 | Certain | Max-merge is pure `query.MaxMerge` keyed (Date, Tool, User, Machine), whole record, ties to live | TS maxMergeEntries verbatim; Constitution V and the 260610-srmi design decision forbid field-wise or summed variants | S:95 R:85 A:95 D:95 |
| 5 | Certain | Collapse User/Machine at the daily level via GroupBy before Window/RollUp so float summation associates as the TS does | JSON prints raw floats; TS sums per day across machines then rolls up; the B2 order would associate per machine first | S:80 R:85 A:85 D:85 |
| 6 | Certain | Record order into the collapse: max-merged own machine first, then other machines in walk order; `-u all` in ascending user order | Mirrors mergeEntries(effectiveLocal, remote) and readAllUsersByUser flattening; addition order is part of the float contract | S:85 R:85 A:90 D:90 |
| 7 | Certain | `-u <other>` and `-u all` make no ccusage call and surface no fetch warnings | TS fetchToolMerged returns before fetchHistory on those branches; node call logs for user-other and user-all are empty | S:95 R:90 A:95 D:95 |
| 8 | Certain | Snapshot labels are cleared only in single mode daily all-tools; multi-mode JSON carries `label` on tools with a current record | TS multi paths build snapshots from fetchToolMerged entries which carry label; fetchAllTotals is single-mode only | S:90 R:90 A:95 D:95 |
| 9 | Confident | The guard lives in `config.MetricsDirGuard` returning lines, with git behind a `Cloner` interface implemented by `sync.Exec.CloneQuiet` (30 s, GIT_TERMINAL_PROMPT=0) | config already owns the marker constants and Load; sync owns the git driver and git.go reserved the quiet clone for B3; the edge stays the only writer | S:80 R:80 A:80 D:70 |
| 10 | Tentative | Clone-failure `{detail}` reproduces Node's execFileSync message (`Command failed: git clone {url} {dir}` + `\n{stderr}`; `spawnSync git ETIMEDOUT` on timeout) | The spec pins only `({git error})`; the fake git always exits 0 so no harness case covers it; verify in R2 | S:40 R:85 A:30 D:30 |
| 11 | Certain | The TS guard's `!metricsRepo` branch is not ported | Unreachable: Mode Multi is derived from a non-empty metrics_repo; dead code has no external surface | S:70 R:95 A:90 D:80 |
| 12 | Certain | `Run` takes the post-guard `config.Config`; the ccusage Source is stamped with cfg.User/cfg.Machine at the edge | command needs User and Machine to split own from others; stamping is what lets MaxMerge and B4's GroupBy(Machine) key on identity | S:80 R:90 A:90 D:90 |
| 13 | Certain | `Normalize` gains `mode` and emits the `-u` notice first, before the since/until guard | TS main() prints the -u warning immediately after assertUserNotReserved; guards.go already documents the B4/B5 slot order | S:85 R:90 A:90 D:85 |
| 14 | Certain | Gate is the 158 listed IDs, expected 302/368; the remaining 66 red are B4/B5/B6/B7-owned | Computed from matrix.json against the fresh 144/368 baseline run on this worktree | S:90 R:90 A:95 D:95 |
| 15 | Confident | Hydrate creates `go-port/multi-mode` and modifies the four package-layer files | The composition spans four packages; prior rows extended per-package files, but a cross-cutting flow needs one owner so B4–B6 have a single place to update | S:60 R:90 A:70 D:60 |
| 16 | Confident | Malformed day-files are skipped silently and missing JSON keys decode as 0 (TS would propagate NaN) | TS swallows parse errors identically; the NaN case needs a hand-corrupted file the writer never produces | S:70 R:90 A:80 D:70 |
| 17 | Certain | `-u <cfg.User>` is the no-flag path; `-u all` in single mode takes the same warn-and-clear as any name | TS condition `targetUser && targetUser !== config.user`; the single-mode guard excludes only the leaderboard displays | S:90 R:90 A:95 D:95 |

17 assumptions (11 certain, 5 confident, 1 tentative, 0 unresolved).
