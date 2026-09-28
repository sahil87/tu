---
type: memory
description: sync.FullSync and sync.Report.Format — one decision path for live and dry-run sync (fetch → per-tool Write → git half), the Report/ToolReport preview shape, WouldCommit (unchanged byte-identical rewrites excluded), and the exact stdout byte rules of Report.Format
---

# Dry-Run Report

**Domain**: sync

## Overview

`sync.FullSync` (`src/go/internal/sync/flow.go`) is one function covering both sync modes: fetch → per-tool `sync.Write` → (dry-run) a read-only git preview collected into a `sync.Report`, or (live) the `SyncMetrics` round trip and `.last-sync` touch. `Report.Format` (`src/go/internal/sync/report.go`) renders the dry-run preview as stdout lines. The write decisions come from [day-file-writer](/sync/day-file-writer.md); the live round trip is [git-flow](/sync/git-flow.md); the edge that prints the outcome is [multi-mode](/command/multi-mode.md).

## Requirements

### Requirement: FullSync runs one decision path for both modes
`sync.FullSync(ctx, in Inputs, dryRun bool) (Outcome, error)` (`src/go/internal/sync/flow.go`) MUST be one function covering both modes. `sync.Inputs{Config, StateDir, Now, Source, Git}` carries the post-guard `config.Config`, the runtime-state dir where `.last-sync` lives, the clock, the fetch seam, and the git driver. `sync.Fetcher` MUST be the one-method interface `FetchAll(ctx, period string, extraArgs []string, fresh bool) ([]fact.Record, []*source.Error)` satisfied by `*ccusage.Source` at the edge; `sync` MUST NOT import the ccusage adapter. Both modes MUST fetch `FetchAll(ctx, source.PeriodDaily, nil, false)` (the cached daily fetch), group records by tool, and call `sync.Write(Config.MetricsDir, Config.User, Config.Machine, tool, recs, dryRun)` per `fact.Tools` in registry order. `sync.Outcome{OK, Report, Warnings, Lines}` carries the fetch's per-source errors in registry order as `Warnings` (the edge writes them BEFORE `Lines`, exactly where the fetch sits relative to every git call).

- **Dry-run** MUST collect `sync.ToolReport{Tool, Decisions}` per tool into a `sync.Report`, compute the git half read-only, touch nothing on disk, skip `SyncMetrics` and `TouchLastSync` entirely, and return `Outcome{OK: true, Report, Warnings}`.
- **Live** MUST run `SyncMetrics(Config.MetricsDir, Config.User, Now, Git)` after the writes; on `ok` it MUST call `TouchLastSync(StateDir, Now)`; a `Write` or touch error MUST be returned as `error`.

#### Scenario: Dry-run touches nothing
- **GIVEN** a configured metrics dir and a fetchable source
- **WHEN** `FullSync(ctx, in, true)` runs
- **THEN** no directory is created, no file is written, no git command other than one read-only `status --porcelain {user}/` runs, and `.last-sync` is untouched

### Requirement: Report carries the whole preview
`sync.Report` MUST carry `MetricsDir, User, Machine, Tools []ToolReport` (in `fact.Tools` registry order), `WouldCommit bool`, and `CommitMessage string`. `WouldCommit` MUST be `anyWrite || dirty` where `anyWrite` is any `ActionWrite` decision and `dirty` is a non-empty trimmed `status --porcelain {user}/` run through `Git.Run` unconditionally (mirroring `SyncMetrics`' unconditional status — a tracked-but-deleted user dir still counts as dirty); any error from that status call MUST count as not dirty. `CommitMessage` MUST come from `sync.CommitMessage` — the same helper the live commit uses.

#### Scenario: Git failure means not dirty
- **GIVEN** a metrics dir that is not a git repo
- **WHEN** `FullSync` runs in dry-run mode
- **THEN** the status error counts as not dirty (never a crash) and `WouldCommit` reflects only the write decisions

### Requirement: Report.Format emits fixed stdout lines
`sync.Report.Format(home string) []string` (`src/go/internal/sync/report.go`) MUST return the stdout lines the edge prints. With `userPrefix = filepath.Join(MetricsDir, User)`, the header directory MUST be `config.Tildefy(userPrefix, home) + "/"`, and each decision's display name MUST be `Path` with the `userPrefix + "/"` prefix stripped when present. Costs MUST render via `render.FormatCost` — the shared thousands-separated formatter every table uses (`$1,234.56`), in both blocks. `ActionUnchanged` decisions (byte-identical rewrites) MUST be omitted from the write list and count.

- Write lines: `  {name}  {cost}  (new)` when `ExistingCost == nil`, else `  {name}  {cost}  (update: {existing} → {incoming})`.
- Skip lines: `  {name}  incoming {incoming} < existing {existing}` (existing defaults to `$0.00` when nil).
- Header: `Would write {N} day-file(s) under {dir}:` followed by the write lines when N > 0, else the single line `Would write 0 day-file(s) under {dir}.`
- Skip block, only when K > 0: `Would skip {K} file(s) (never-shrink guard):` followed by the skip lines.
- Commit line: `Would commit: "{CommitMessage}", then pull --rebase, then push` when `WouldCommit`, else `Would commit: nothing (no changes), then pull --rebase, then push` — no hard-coded branch, since the dry-run does not resolve the pull target.
- Final line: `Dry run — nothing written, committed, or pushed.` (em dash U+2014).

Pull/push MUST be reported as the operations that would follow, never executed or probed. A relative `metrics_dir` yields relative names and no tildefication.

#### Scenario: Byte-identical rewrites predict no commit
- **GIVEN** a clean metrics repo whose day-files already hold exactly what the fetch returns (byte-identical rewrites)
- **WHEN** the dry-run runs
- **THEN** the decisions are `ActionUnchanged`, the report prints `Would write 0 day-file(s) under …` and `Would commit: nothing (no changes), then pull --rebase, then push` — matching what the live run's git sees

## Design Decisions

### One FullSync with a dryRun flag
**Decision**: a single `sync.FullSync` runs fetch → per-tool `Write(dryRun)` → (dry) read-only git half into a `Report`, or (live) `SyncMetrics` + `TouchLastSync`.
**Why**: the preview must share the live decision path; two functions would drift.
**Rejected**: separate `Preview`/`Sync` functions sharing helpers (two call graphs to keep in lock-step).
*Introduced by*: 260916-lsml-sync-metrics-writer

### Dry-run shares the real decision path, not a parallel one
**Decision**: the dry-run computes its preview from the same `sync.Write` decisions a live run produces; only the filesystem effects are gated.
**Why**: an accurate preview must not drift from the live path — a preview that drifts is worse than none. This is preview-for-safety, not consent: sync is additive behind the never-shrink guard, so no confirmation gate is added.
**Rejected**: a separate preview routine duplicating the decision logic (invites exactly the drift the shared path forbids).
*Introduced by*: 260717-xuhk

### The git half of the preview is computed locally
**Decision**: `WouldCommit` comes from the write decisions plus one read-only `status --porcelain {user}/`; pull/push are reported as would-follow operations, never invoked or probed (the dry-run never runs `rev-parse @{u}` or `ls-remote`, so the commit line carries no branch); the commit message comes from the same `sync.CommitMessage` helper the live commit calls.
**Why**: the preview must run without touching the working tree, the metrics repo, or the network; `status --porcelain` is read-only and is the same idiom the live round trip uses, so no new git path is introduced. Single-sourcing the message makes live and preview strings unable to drift.
**Rejected**: probing pull/push with `git ls-remote` or a dry-run push (touches the network).
*Introduced by*: 260717-xuhk

### Byte-identical rewrites are not commits
**Decision**: `WouldCommit = anyWrite || dirty` where `anyWrite` counts only `ActionWrite` decisions — a byte-identical rewrite (`ActionUnchanged`) is neither a write nor a predicted commit.
**Why**: byte equality is exactly what git sees, so the steady-state preview agrees with the live run — a preview that predicts a commit the live run never makes defeats its purpose.
**Rejected**: counting equal-cost rewrites as writes (over-predicts a commit in steady state); treating equal *cost* as unchanged (a token-only change with equal cost would be hidden from the preview although git commits it).
*Introduced by*: 260928-ubws-sync-drop-at-cutover-fixes

### The live fetch reaches sync through a Fetcher interface
**Decision**: `sync.Fetcher` is a one-method interface satisfied by `*ccusage.Source` at the edge; `sync` imports `fact`, `source`, `source/metrics` and `config`, never the ccusage adapter.
**Why**: the architecture assigns the writer, driver and dry-run report to `sync` and keeps I/O adapters at the ends — the same seam shape `command` uses for its own fetch.
**Rejected**: importing the adapter from `sync` (couples the flow package to an I/O adapter the architecture keeps at the ends).
*Introduced by*: 260916-lsml-sync-metrics-writer

### Both report blocks share one cost formatter
**Decision**: write and skip lines both render costs via `render.FormatCost` — the shared thousands-separated formatter every table uses.
**Why**: one formatter serving both blocks keeps the report consistent with every other cost surface in the CLI.
**Rejected**: a bare `"$" + render.FixedHalfUp(x, 2)` without grouping (two formatting styles inside one report).
*Introduced by*: 260928-ubws-sync-drop-at-cutover-fixes
