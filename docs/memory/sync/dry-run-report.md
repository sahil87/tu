---
type: memory
description: sync.FullSync and sync.Report.Format — one decision path for live and dry-run sync (fetch → per-tool Write → git half), the Report/ToolReport preview shape, WouldCommit, and the exact stdout byte rules of Report.Format
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
`sync.Report.Format(home string) []string` (`src/go/internal/sync/report.go`) MUST return the stdout lines the edge prints. With `userPrefix = filepath.Join(MetricsDir, User)`, the header directory MUST be `config.Tildefy(userPrefix, home) + "/"`, and each decision's display name MUST be `Path` with the `userPrefix + "/"` prefix stripped when present. Costs MUST render as `"$" + render.FixedHalfUp(x, 2)` — no thousands separators in either block.

- Write lines: `  {name}  {cost}  (new)` when `ExistingCost == nil`, else `  {name}  {cost}  (update: {existing} → {incoming})`.
- Skip lines: `  {name}  incoming {incoming} < existing {existing}` (existing defaults to `$0.00` when nil).
- Header: `Would write {N} day-file(s) under {dir}:` followed by the write lines when N > 0, else the single line `Would write 0 day-file(s) under {dir}.`
- Skip block, only when K > 0: `Would skip {K} file(s) (never-shrink guard):` followed by the skip lines.
- Commit line: `Would commit: "{CommitMessage}", then pull --rebase origin main, then push` when `WouldCommit`, else `Would commit: nothing (no changes), then pull --rebase origin main, then push`.
- Final line: `Dry run — nothing written, committed, or pushed.` (em dash U+2014).

Pull/push MUST be reported as the operations that would follow, never executed or probed. A relative `metrics_dir` yields relative names and no tildefication.

#### Scenario: WouldCommit may over-predict
- **GIVEN** an incoming record whose cost equals the existing day-file's (a byte-identical rewrite)
- **WHEN** the dry-run runs
- **THEN** the decision is a write, `WouldCommit` is true, and a live sync may still commit nothing — the preview errs toward showing the commit and never under-reports one

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
**Decision**: `WouldCommit` comes from the write decisions plus one read-only `status --porcelain {user}/`; pull/push are reported as would-follow operations, never invoked or probed; the commit message comes from the same `sync.CommitMessage` helper the live commit calls.
**Why**: the preview must run without touching the working tree, the metrics repo, or the network; `status --porcelain` is read-only and is the same idiom the live round trip uses, so no new git path is introduced. Single-sourcing the message makes live and preview strings unable to drift.
**Rejected**: probing pull/push with `git ls-remote` or a dry-run push (touches the network).
*Introduced by*: 260717-xuhk

### WouldCommit's steady-state over-prediction is sanctioned
**Decision**: `WouldCommit = anyWrite || dirty`, even though an equal-cost rewrite produces a byte-identical day-file and a live sync then commits nothing.
**Why**: the preview must err toward showing the commit and must never under-report one that would happen; detecting byte-identical rewrites would require reading and comparing file contents — work that can only downgrade a "Would commit" to "nothing", never surface a missed commit.
**Rejected**: content-compare each would-write (extra reads whose only effect is downgrading true positives).
*Introduced by*: 260717-xuhk

### The live fetch reaches sync through a Fetcher interface
**Decision**: `sync.Fetcher` is a one-method interface satisfied by `*ccusage.Source` at the edge; `sync` imports `fact`, `source`, `source/metrics` and `config`, never the ccusage adapter.
**Why**: the architecture assigns the writer, driver and dry-run report to `sync` and keeps I/O adapters at the ends — the same seam shape `command` uses for its own fetch.
**Rejected**: importing the adapter from `sync` (couples the flow package to an I/O adapter the architecture keeps at the ends).
*Introduced by*: 260916-lsml-sync-metrics-writer

### Both report blocks share one cost formatter
**Decision**: write and skip lines both render costs as `"$" + render.FixedHalfUp(x, 2)`, with no thousands separators in either block.
**Why**: parity with the frozen golden corpus (`/harness/golden-corpus.md`, the retired TypeScript implementation's bytes), where one formatter serves both blocks.
**Rejected**: locale-grouped costs in the write block only (diverges from the reference bytes).
*Introduced by*: 260916-lsml-sync-metrics-writer
