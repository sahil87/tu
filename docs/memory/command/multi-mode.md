---
type: memory
description: Multi-mode record composition — command.gather's four record paths by mode and -u, the own-user write-then-MaxMerge path, the repo-only -u paths, the mode-keyed snapshot label rule, and the auto-clone guard's placement in the data path
---
# Multi Mode

**Domain**: command

## Overview

Multi mode turns the per-machine live views into one per-user (and, with `-u all`, one per-org) view by reading the metrics-repo clone. `command.gather` (`internal/command/run.go`) picks one of four record paths by mode and `-u` and returns stamped, un-collapsed daily records; the snapshot/history tails then run identically on every path — the mode changes only where the daily records come from.

## Requirements

### Requirement: The data path runs with the post-guard config
`config.MetricsDirGuard` decides the effective mode before any record is read: it may demote `Mode` to `Single` (missing metrics dir plus a fresh `.clone-failed` marker or a failed quiet clone) or leave it `Multi` after a successful quiet clone, and `command.Run` receives the demoted config — the record-path selection and the leaderboard gate see the same mode. The guard sits between `config.Load` and the reserved-user check in [cmd/tu](/command/entry-point.md) and runs on the data path only — setup commands never clone here (`status` only reads; `init-metrics` has its own interactive clone) ([metrics-dir-guard](/config/metrics-dir-guard.md)). (xivf)

### Requirement: gather selects the record path by mode and -u
`gather(ctx, req, cfg, deps)` (`internal/command/run.go`) iterates tools in registry order (all `fact.Tools`, or the one requested tool via `fact.Lookup`) and selects the record path by mode and `-u`:

- **single** — the live fetch: `FetchAll` for all tools or `Fetch` for one, `source.PeriodDaily`, no extra args, `Fresh` honored, errors collected into `Result.Warnings` ([ccusage-adapter](/source/ccusage-adapter.md)).
- **multi, `-u ""` or `-u == cfg.User`** — `gatherOwn`, the write-then-read path: the live fetch, then per tool in registry order `Writer.Write(cfg.User, cfg.Machine, tool, live-by-tool)` when a `Writer` is wired (the never-shrink day-file write, [day-file-writer](/sync/day-file-writer.md)), THEN `Repo.Read(cfg.User, tool)` split on `Machine == cfg.Machine` into `own`/`others`; the records are `query.MaxMerge(live, own)` followed by `others` in walk order. A write error propagates out of `gather` and `Run` — the edge prints it, exit 1. `-u <cfg.User>` is byte-identical to no `-u`.
- **multi, `-u all`** — `gatherAllUsers`: for each `u` in `Repo.Users()`, `Repo.Read(u, tool)` per tool — no live fetch, no source errors.
- **multi, `-u <other>`** — `gatherUser`: `Repo.Read(user) (tool)` per tool — no live fetch, no source errors.

The repo-only paths make no ccusage call at all ([metrics-reader](/source/metrics-reader.md), [ccusage-adapter](/source/ccusage-adapter.md)). Both leaderboards are a fifth repo-only consumer: `lb`/`lbh` read `gatherAllUsers(deps.Repo, tools)` directly — no live fetch, no source warnings, no writes ([guards](/command/guards.md), [leaderboard](/view/leaderboard.md)). (xivf) (lsml) (2gbb)

#### Scenario: The repo-only paths never fetch
- **GIVEN** multi mode and `tu cc -u other-user` or `tu -u all`
- **WHEN** `gather` runs
- **THEN** zero ccusage calls are made and no source warnings are produced

### Requirement: MaxMerge is the own-machine high-water merge
Per key `(Date, Tool, User, Machine)`, `query.MaxMerge(live, own)` keeps whichever WHOLE record — live or stored — has the greater `TotalCost`; ties go to live ([aggregation](/query/aggregation.md)). Never field-wise, never summed: stored day-files are never-shrink high-water marks ([day-file-writer](/sync/day-file-writer.md)), so the max reconciles the live view (which under-reports days whose transcripts the tool has purged) with the machine's own snapshots without double-counting a partially-purged day's residual. (srmi)

#### Scenario: Both arms of the max-merge
- **GIVEN** live `cc` records `2026-01-05..07` at `$0.50`, stored own-machine day-files `2026-01-05` (`$0.25`) and `2026-01-06` (`$0.75`), and another machine's `$0.40` on `2026-01-06`
- **WHEN** `tu cc h --since 2026-01-01 --until 2026-01-31` runs in multi mode
- **THEN** the rows are `$0.50` (live wins), `$1.15` (stored `$0.75` wins, plus the other machine's `$0.40`), `$0.50`; the Total is `$2.15`

### Requirement: Un-collapsed records with a caller-side collapse
`gather` returns the stamped, un-collapsed per-machine/per-user records in a fixed order — the own machine first (the `MaxMerge` output), then other machines in walk order; for `-u all`, users ascending then walk order. The callers apply `query.Collapse(recs, Tool, Date)` followed by a stable date sort before the `Window`/`RollUp`/`GroupBy` tail — the daily cross-machine/cross-user sum reproduces the reference summation association for `--json`'s raw float bytes, and in single mode the collapse is the identity (one pathway, not two) — while the `--by-machine` breakdown groups the same raw records on the machine/user dimension the collapse would drop ([aggregation](/query/aggregation.md), [breakdown](/view/breakdown.md), [snapshot](/view/snapshot.md)). (xivf) (pmsd)

### Requirement: The snapshot label rule is keyed on mode
`runSnapshot` clears every `ToolTotals.Label` only when `Mode == Single && Source == "" && Period == Daily && !Flags.ByMachine` — the bare-label behavior is a single-mode artifact of the all-tools daily fetch. In multi mode every snapshot builds from merged entries, so a tool with a record on the current label carries `label` in `--json`; `tu cc --json` (any period) and `tu m --json` carry it in single mode too. Under `--by-machine` the clear does not apply even in single mode. (3am6) (pmsd)

#### Scenario: The mode-keyed label
- **GIVEN** multi mode with a record on today's label, and single mode, both with the same data
- **WHEN** `tu --json` runs in each
- **THEN** the multi-mode output carries `"label"` for the tool with a current record; the single-mode output carries no `"label"` key at all

### Requirement: The 3-month cap windows stored records identically
The cap ([guards](/command/guards.md)) defaults the floor before `gather` runs, and `query.Window` applies to the collapsed daily records after the merge — repo-sourced records are windowed by the same floor as live ones: a stored day-file older than the floor does not appear in `tu h` and does appear in `tu h --full`. (yuuj) (xivf)

## Design Decisions

### Write-then-read on the own-user path
**Decision**: `gatherOwn` runs the never-shrink-guarded day-file write (per tool in registry order, through `Deps.Writer`) after the live fetch and BEFORE `Repo.Read`; a nil `Writer` means no write (tests).
**Why**: the own machine's day-file write on every multi-mode data command is the wire protocol between machines in a mixed fleet. Write-then-max-merge is arithmetically identical to max-merge alone for the rendered bytes: a stored file only ever holds a value the never-shrink guard let through, and the guard lets through exactly the values that win the max.
**Rejected**: a read-only port (a binary in a mixed fleet would contribute nothing to the shared repo; the worst field failure is a bad sync write, gated by the harness's `tree` channel and live runs).
*Introduced by*: 260610-srmi (max-merge), 260916-lsml-sync-metrics-writer (write seam)

### Daily collapse before roll-up, for float parity
**Decision**: `runSnapshot`/`runHistory` run `SortByDate(Collapse(recs, Tool, Date))` on `gather`'s un-collapsed records; the `Window` → `RollUp` → `GroupBy` tail is unchanged.
**Why**: `--json` prints costs as raw floats, so the association of additions is a byte surface. The reference sums per day across machines first and rolls days into periods second; roll-up then group would sum each machine's days first and the machines second — a different float association that can differ in the last bit and therefore in the bytes. Collapsing at the daily level reproduces the association; running it in single mode too keeps one pathway.
**Rejected**: changing `RollUp` to ignore User/Machine (breaks the per-machine breakdown, which groups the raw records); accepting the association drift (a `2.1500000000000004` is a byte divergence).
*Introduced by*: 260916-xivf-metrics-source-and-multi-mode

### lbh values as a tool-major left fold
**Decision**: per user, the `lbh` series is `GroupBy(Relabel(Window(ByUser(raw, u))), Date)` over the gather-ordered records — one left fold in record input order (tool-major, walk order within a tool) — not `Collapse(Tool, Date)` then `RollUp`.
**Why**: the reference aggregates each tool's unmerged machine-major entries into the period and then adds the tools in registry order — one left fold over the same ordered sequence; the main history's collapse-then-roll-up is a different fold and would differ in the last bit, and the `lbh --json` doubles are byte surfaces.
**Rejected**: reusing `runHistory`'s tail — byte drift in `lbh --json`.
*Introduced by*: 260916-2gbb-leaderboard-lb-lbh

### Spec per-label sums on the -u <other> path
**Decision**: `Collapse(Tool, Date)` runs on every path, including `-u <other>`, so an other user with the same tool and date on two machines gets plain per-label sums — the spec contract.
**Why**: the reference never merges on that branch — a multi-machine other user renders duplicate-label daily rows and sums machine-major, an unreconciled quirk no shipped fixture observes. The spec is the contract, and one collapse path keeps the summation order uniform.
**Rejected**: dropping the collapse on that one path to reproduce the quirk byte-for-byte — duplicate rows are a bug, not a surface.
*Introduced by*: 260916-xivf-metrics-source-and-multi-mode

### Reproduce the JSON label quirk
**Decision**: `runSnapshot` clears `ToolTotals.Label` only on the single-mode daily-all path; every other path (any explicit source, any non-daily period, multi mode, `--by-machine`) keeps the label.
**Why**: the label omission on the bare single-mode daily JSON is a harness-compared byte surface; parity with the frozen `src/node/` oracle (until plan row Z1).
**Rejected**: reproducing nothing (a harness red on populated data); clearing labels everywhere (breaks `tu cc --json` and every multi-mode snapshot).
*Introduced by*: 260916-3am6-query-view-render-snapshot
