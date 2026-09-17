---
type: memory
description: The Go port's multi mode — the auto-clone guard at the edge, command.gather's four record paths (live fetch, the own-user never-shrink day-file write via Deps.Writer then MaxMerge + other-machine sum, the repo-only -u paths — no ccusage calls) returning un-collapsed records the callers Collapse before the tail, the repo-only leaderboards lb/lbh over gatherAllUsers (ranking, lbh's tool-major left fold), float summation order pinned to the TS, the mode-keyed snapshot label rule, the 3-month cap
---
# Multi Mode (Go port)

**Domain**: go-port

## Overview

Multi mode turns the per-machine ccusage views into one per-user (and, with `-u all`, one per-org) view by reading the metrics-repo clone. The composition spans the layers: `cmd/tu` runs `config.MetricsDirGuard` between `config.Load` and the reserved-user check and prints its stderr lines ([config-and-setup](/go-port/config-and-setup.md)), then wires a config-stamped `ccusage.Source` and a `metrics.Source{Dir: cfg.MetricsDir}` into `command.Deps` ([command-edge](/go-port/command-edge.md)); `command.gather` picks one of four record paths by mode and `-u` and returns the stamped, un-collapsed records; the callers run the daily `Collapse` before the tail; the merge arithmetic is the pure `query.MaxMerge` + `query.Collapse` pair ([query-view-render](/go-port/query-view-render.md)); the repo reader is `internal/source/metrics` ([fact-and-sources](/go-port/fact-and-sources.md)). Everything after `gather` — the caller-side `Collapse`, `Window`, `RollUp`, the one `GroupBy`, the by-machine breakdown, the view and render layers — is the single-mode tail unchanged: the mode changes only where the daily records come from. The shipped-TypeScript contract this reproduces lives in [multi-machine](/sync/multi-machine.md) and [data-pipeline](/cli/data-pipeline.md) (frozen under the plan's D4).

## Requirements

### Requirement: The data path runs with the post-guard config
`config.MetricsDirGuard` decides the effective mode before any record is read: it may demote `Mode` to `Single` (missing metrics dir plus a fresh `.clone-failed` marker or a failed clone) or leave it `Multi` after a successful quiet clone, and `command.Run` receives the demoted config. The guard runs on the data path only — setup commands never clone here (`status` only reads; `init-metrics` has its own interactive clone). The guard itself — the marker, the 3 h cooldown, the `Cloner` seam, the byte-exact stderr lines — is documented in [config-and-setup](/go-port/config-and-setup.md).

### Requirement: gather's four record paths
`gather(ctx, req, cfg, deps)` iterates tools in registry order (all `fact.Tools`, or the one requested tool) and selects the TS path by mode and `-u`:

- **single** — the live fetch exactly as in single mode: `FetchAll` for all tools or `Fetch` for one, `source.PeriodDaily`, no extra args, `Fresh` honored, errors collected into `Result.Warnings`.
- **multi, `-u ""` or `-u == cfg.User`** — the live fetch; then per tool in registry order, `Writer.Write(cfg.User, cfg.Machine, tool, live-by-tool)` when a `Writer` is wired (the never-shrink-guarded own-user day-file write — [metrics-sync](/go-port/metrics-sync.md)), THEN `stored := Repo.Read(cfg.User, tool)`, split on `Machine == cfg.Machine` into `own` and `others`; the records are `MaxMerge(live, own)` followed by `others` in walk order. A write error propagates out of `gather` and `Run` (the TS crash path; the edge prints it, exit 1). `-u <cfg.User>` is byte-identical to no `-u`.
- **multi, `-u all`** — for each `u` in `Repo.Users()` (ascending), `Repo.Read(u, tool)` per tool; no live fetch, no source errors.
- **multi, `-u <other>`** — `Repo.Read(user, tool)` per tool; no live fetch, no source errors.

The repo-only paths make no ccusage call at all — the harness compares the call multiset, and `user-other`/`user-all` record zero ccusage calls on the node side. `gather` returns the stamped, **un-collapsed** records in this order (own-machine `MaxMerge(live, own)` output first, then walk order; `-u all`: users ascending then walk order); the callers collapse — `runSnapshot`/`runHistory` apply `query.Collapse(recs, query.Tool, query.Date)` on the raw records before the unchanged `Window`/`RollUp`/`GroupBy` tail, so the main-table bytes are unchanged while the `--by-machine` breakdown groups the same raw records on the machine/user dimension the collapse would drop (pmsd).

### Requirement: MaxMerge is the own-machine high-water merge
Per key `(Date, Tool, User, Machine)` the own-user path keeps whichever WHOLE record — live or stored — has the greater `TotalCost`; ties go to live. Never field-wise, never summed (Constitution V): stored day-files are never-shrink high-water marks, so the max reconciles the live view (which under-reports days whose transcripts Claude Code has purged) with the machine's own snapshots without double-counting a partially-purged day's residual.

#### Scenario: Both arms of the max-merge
- **GIVEN** live cc `2026-01-05..07` at `$0.50` and stored own-machine cc day-files `2026-01-05` (`$0.25`) and `2026-01-06` (`$0.75`), plus other-machine `$0.40` on `2026-01-06`
- **WHEN** `cc h --since 2026-01-01 --until 2026-01-31` runs in multi mode
- **THEN** the rows are `$0.50` (live wins), `$1.15` (stored `$0.75` wins, plus the other machine's `$0.40`), `$0.50`; Total `$2.15`

### Requirement: Collapse before the tail fixes the summation order
`--json` prints costs as raw doubles, so the *association* of float additions is a byte surface. The TS sums per day across machines first (`mergeEntries`, in input order) and rolls days into weeks or months second; the window-then-roll-up-then-group tail alone would sum each machine's days first and the machines second. `Collapse(recs, Tool, Date)` — the daily cross-machine/cross-user sum — therefore runs in the callers on `gather`'s un-collapsed daily records BEFORE the unchanged `Window`/`RollUp`/`GroupBy` tail, reproducing the TS association exactly (a test with an association-sensitive float triple pins it; `2.15`, never `2.1500000000000004`). The collapse runs in single mode too — one record per key, the identity — one pathway, not two. Record order into the collapse is part of the same contract: own machine first, then other machines in walk order (year asc, machine asc, file asc); for `-u all`, users ascending then walk order. The by-machine breakdown keeps the same contract on the raw records: its one `GroupBy(Relabel(Window(raw)), Tool, Date, dim)` pass sums in record input order, so a `-u all` user with two machines in one period sums `((a1+a2)+b1)+b2`, never `(a1+a2)+(b1+b2)` — the TS `buildHistoryMachineCosts` association, byte-visible in the `machines` JSON values and pinned by a `Run` test plus the green `-u all` harness cases (pmsd).

### Requirement: The snapshot label rule is keyed on mode
`runSnapshot` clears `ToolTotals.Label` only when `Mode == Single && Source == "" && Period == Daily && !Flags.ByMachine` — the bare-label behavior is a single-mode artifact of the TS `fetchAllTotals`. In multi mode every snapshot builds from merged entries, so a tool with a record on the current label carries `"label"` in `--json`; zero tools carry nothing, as before. Under `--by-machine` the clear does not apply even in single mode — the TS by-machine path goes through `fetchToolMergedWithMachines` (labelled entries), so `tu --by-machine --json` carries `"label"` (pmsd).

### Requirement: The 3-month cap windows stored records identically
The cap (`Normalize` step 3: history ∧ period ≠ monthly ∧ no explicit bound ∧ not `--full` → `Since = ThreeMonthFloor(now)`) defaults the floor before `gather` runs, and `Window` applies to the collapsed daily records after the merge — so repo-sourced records are windowed by the same floor as live ones: a stored day-file older than the floor does not appear in `h` and does appear in `h --full`.

### Requirement: Seams the later rows build on
`gather` returns the un-collapsed per-machine/per-user records and the callers run the daily collapse for the main table; `--by-machine` groups the same raw records on `Machine` (or `User` under multi-mode `-u all`) — one `GroupBy(Tool, Date, dim)` pass per table ([command-edge](/go-port/command-edge.md), [query-view-render](/go-port/query-view-render.md)). The leaderboards are the fifth consumer of the repo records (2gbb): both `lb` and `lbh` read `gatherAllUsers(deps.Repo, tools)` directly — repo-only, no live fetch, no ccusage call, no source warnings, no writes — and rank/pivot over `Repo.Users()` with one `GroupBy` per window ([command-edge](/go-port/command-edge.md) owns the gate, windows and run paths). The `lbh` value per user and period label is a left fold in record input order over the windowed, relabelled records (tool-major, walk order within a tool) — DISTINCT from the main table's collapse-then-roll-up association (see the Design Decision below). The own-user path is the TS write-then-read shape in full: the never-shrink day-file writer (`command.Deps.Writer`, satisfied by `sync.Writer{Dir: cfg.MetricsDir}`) runs per tool in registry order after the live fetch and before the `Read` (lsml — [metrics-sync](/go-port/metrics-sync.md)). No multi-mode seam remains open.

## Design Decisions

### Daily collapse before roll-up, for float parity
**Decision**: `runSnapshot`/`runHistory` run `Collapse(recs, Tool, Date)` on `gather`'s un-collapsed records; the `Window` → `RollUp` → `GroupBy` tail is unchanged.
**Why**: `--json` prints costs raw. The TS sums per day across machines first (`mergeEntries`) and rolls days into periods second; `RollUp` then `GroupBy` would sum each machine's days first and machines second — a different float association that can differ in the last bit and therefore in the bytes. Collapsing at the daily level reproduces the TS association; running it in single mode too keeps one pathway.
**Rejected**: Changing `RollUp` to ignore User/Machine (breaks the per-machine columns, which group the raw records); accepting the association drift (a `2.1500000000000004` is a harness red and a spec violation).
*Introduced by*: 260916-xivf-metrics-source-and-multi-mode

### Write-then-read on the own-user path
**Decision**: `gatherOwn` runs the never-shrink-guarded day-file write (per tool in registry order, through `Deps.Writer`) after the live fetch and BEFORE `Repo.Read`; a nil `Writer` means no write (tests).
**Why**: The TS writes the own machine's day-file on every multi-mode data command — that write is the wire protocol between machines in a mixed fleet. Write-then-max-merge is arithmetically identical to max-merge alone for the rendered bytes: a stored file only ever holds a value the never-shrink guard let through, and the guard lets through exactly the values that win the max.
**Rejected**: A read-only port as the end state (a Go binary in a mixed fleet would contribute nothing to the shared repo; the plan's worst field failure is a bad sync write, gated by the harness's `tree` channel and `tudiff live`).
*Introduced by*: 260916-lsml-sync-metrics-writer

### lbh values as a tool-major left fold
**Decision**: Per user, the `lbh` series is `GroupBy(Relabel(Window(ByUser(raw, u))), Date)` over the gather-ordered records — one left fold in record input order (tool-major, walk order within a tool) — not `Collapse(Tool, Date)` then `RollUp`.
**Why**: The TS `aggregateMachineMap` aggregates each tool's unmerged machine-major entries into the period and `sumLeaderboardToolMaps` then adds tools in registry order — one left fold over the same ordered sequence. The main history's collapse-then-roll-up association is a different fold and would differ in the last bit, and the `lbh --json` doubles are byte surfaces (DC-10).
**Rejected**: Reusing `runHistory`'s tail — byte drift in `lbh --json`.
*Introduced by*: 260916-2gbb-leaderboard-lb-lbh

### Spec per-label sums on the `-u <other>` path, not the TS quirk
**Decision**: `Collapse(Tool, Date)` runs on every path, including `-u <other>`, so an other user with the same tool and date on two machines gets plain per-label sums — the spec contract (`docs/specs/usage.md` § Own-machine max-merge: "`-u <other user>` and `-u all` read repo entries only (plain per-label sums)").
**Why**: The shipped TS never calls `mergeEntries` on that branch (`fetchToolMerged`): for a multi-machine other user it renders duplicate-label daily rows (the later machine wins the pivot's valueMap, the first match the snapshot `find`) and sums machine-major in `aggregateMonthly`/`aggregateWeekly` — an unreconciled quirk the harness cannot observe (the seed's `other-user` has one machine). The spec is the contract, and one collapse path keeps the summation order uniform.
**Rejected**: Dropping the collapse on that one path to reproduce the quirk byte-for-byte — duplicate rows are a bug, not a surface. The divergence is a candidate `[DECIDE]` item for gate G0/G3; the R2 dogfood window is where a real multi-machine other user would surface it.
*Introduced by*: 260916-xivf-metrics-source-and-multi-mode
