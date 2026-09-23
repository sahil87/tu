---
type: memory
description: command.Run and its result contract — the Deps seams (Fetcher/Repo/Writer/Now/Colors/Width/Live/LastSync), the in-scope check and ErrUnported, the Result fields (Lines/Notices/Warnings/TotalCost/TotalTokens/CostByItem), and the per-poll LiveOptions watch inputs
---
# Run and Result

**Domain**: command

## Overview

`command.Run` (`run.go`) composes source → query → view → render for an in-scope [Request](/command/request-and-parse.md) and returns a `Result`; anything out of scope yields `ErrUnported`. `Run` never touches the process streams — its external inputs arrive through `Deps` and its outputs ride the `Result`; [cmd/tu](/command/entry-point.md) is the only writer.

## Requirements

### Requirement: Run gates, normalizes, then composes
`Run(ctx, req, cfg config.Config, deps Deps) (Result, error)` (`internal/command/run.go`) takes the post-guard config: it needs `Mode` (the record path, the leaderboard gate, the snapshot label rule) plus `User` and `Machine` (the own/other split, the leaderboard's default pinned user). In order:

1. **Leaderboard gate** — `leaderboard(req.Display) && cfg.Mode == config.Single` → `Result{}, ErrLeaderboardMode`, evaluated BEFORE `Normalize` and with no notices ([guards](/command/guards.md)).
2. **Normalize** — `Normalize(req, cfg.Mode, deps.Now())` returns the request the pipeline runs, the notice lines, and `capActive` ([guards](/command/guards.md)).
3. **Scope check** (`inScope`) on the normalized request: displays {Snapshot, History, Leaderboard, LeaderboardHistory}, any of the four formats, any period, single or multi mode, `-u` in any mode (Normalize already cleared the out-of-scope cases), no non-data `Command`, not `Version`. `DryRun` or `SkipBrewUpdate` on a data request → `ErrUnported` without fetching and without the notices; `Sync` is admitted because the edge consumes `--sync` before `Run`.
4. **Leaderboards are repo-only**: both lb and lbh read `gatherAllUsers(deps.Repo, tools)` directly — no live fetch, no source warnings, no writes ([multi-mode](/command/multi-mode.md)).
5. Otherwise `context.WithTimeout(ctx, source.DefaultTimeout)` (120 s, `internal/source/fetch.go`) wraps one `gather` for all tools ([multi-mode](/command/multi-mode.md), [ccusage-adapter](/source/ccusage-adapter.md)), then `runSnapshot` or `runHistory` composes [query](/query/aggregation.md) → [view](/view/table-model.md) → [render](/render/ansi.md).

(3am6) (9ax5) (lsml) (2gbb) (4pze)

### Requirement: Deps declares the seams
`command.Deps` (`run.go`) is Run's external input set:

- `Source Fetcher` — the live seam: `Fetch(ctx, fact.Tool, period, extraArgs, fresh)` and `FetchAll(...)`; `*ccusage.Source` satisfies it, asserted at the cmd/tu assignment ([ccusage-adapter](/source/ccusage-adapter.md)).
- `Repo Repo` — the metrics-clone seam: `Users() []string` and `Read(user) (tool)`; consulted only in multi mode; `metrics.Source` satisfies it ([metrics-reader](/source/metrics-reader.md)).
- `Writer Writer` — the own-user day-file write that precedes every repo read in multi mode; `sync.Writer` satisfies it; nil means no write (tests) ([day-file-writer](/sync/day-file-writer.md)).
- `Now func() time.Time` — `time.Now` at the edge, fixed in tests.
- `Colors ansi.Colors`, `Width int` — the stdout TTY width probed at the edge (80 when piped), per-poll under watch ([ansi](/render/ansi.md)).
- `Live *LiveOptions` — nil for one-shot runs; set per poll under watch (below).
- `LastSync func() string` — the leaderboard footer's staleness text, a closure evaluated only on the lb path so no other command reads `.last-sync`; nil reads as `"never"`.

(xivf) (2gbb) (4pze)

### Requirement: Result carries everything the edge writes
`command.Result` carries `Lines []string` (stdout, one per line, no trailing newline per element), `Notices []string` (the guard lines — the edge writes them BEFORE the warnings), `Warnings []*source.Error` (written via `source.WriteWarnings`, [errors-and-warnings](/source/errors-and-warnings.md)), and the watch stats: `TotalCost float64` / `TotalTokens int64` (sums over the rendered rows) and `CostByItem map[string]float64` — keyed `{Name}` on the snapshot, `{Name}:{label}` plus `total:{label}` on history, and the user or `user/machine` on the leaderboards, valued in the display metric (cost, or tokens under `-t`) like every other display path, while `TotalCost` stays dollar-valued for the watch session stats ([bars-and-deltas](/view/bars-and-deltas.md), [loop-and-terminal](/watch/loop-and-terminal.md)). Totals sum the UNTRUNCATED data — a watch row budget only limits what renders. (4pze)

### Requirement: Deps.Live carries the per-poll watch render options
`Deps.Live *LiveOptions{Prev map[string]float64, Compact bool, MaxRows int}` is nil for one-shot runs — every path with a nil `Live` is byte-identical to the one-shot output — and set per poll by the watch loop's `Poll` closure together with the per-poll `Deps.Width`. It is render state, not CLI grammar — `Request` stays the parse tree. `Prev` is the previous poll's per-item values (the delta maps: in-cell deltas on the snapshot and leaderboard, the trailing row delta on history); `Compact` switches the snapshot and history tables to their compact forms below the watch compositor's width threshold; `MaxRows` is the history row budget ([loop-and-terminal](/watch/loop-and-terminal.md), [compositor](/watch/compositor.md)). `Result.TotalCost`/`TotalTokens`/`CostByItem` are computed over the untruncated data either way. (4pze)

### Requirement: ErrUnported marks a recognized-but-unported request
`command.ErrUnported` marks a recognized-but-unported request; cmd/tu maps it to the placeholder line on stderr with exit 1 and empty stdout. The only remaining trigger is `--skip-brew-update` on a data command (the flag is silently accepted on `tu update` itself). `Run` returns it without fetching and without the guard notices. (3am6) (4pze)

## Design Decisions

### A Repo interface beside Fetcher, not a second Fetcher
**Decision**: `command` consumes the metrics clone through `Repo{Users, Read}`; `metrics.Source` implements it, asserted at the cmd/tu assignment.
**Why**: a repo read has no context, no period, no extra args, no cache, no fresh flag and no error channel — forcing it through `Fetcher` would mean ignored parameters and a fake `*source.Error`. Two small honest interfaces keep `command` adapter-free and give callers `Users()` and per-user reads directly.
**Rejected**: `metrics.Source` satisfying `Fetcher` — ignored parameters and no way to express "all users" or "one user" without smuggling them through `extraArgs`.
*Introduced by*: 260916-xivf-metrics-source-and-multi-mode

### Placeholder for recognized-but-unported requests
**Decision**: `Run` returns `ErrUnported` for out-of-scope requests; cmd/tu prints the placeholder line with exit 1 and empty stdout.
**Why**: unported harness cases keep the signature the differential harness already knows, and `Unknown argument` fires only where the grammar fires it.
**Rejected**: treating unported flags as unknown arguments (a false divergence on the error text).
*Introduced by*: 260916-3am6-query-view-render-snapshot

### LiveOptions is render state, not grammar
**Decision**: the per-poll watch render inputs (`Prev`, `Compact`, `MaxRows`) ride `Deps.Live` — nil for one-shot runs, set per poll by the watch loop's `Poll` closure together with the per-poll `Deps.Width`; every path with a nil `Live` is byte-identical to the one-shot output.
**Why**: watch reuses the one-shot pipeline per poll; only the render options vary per poll, so they arrive as a dependency, not new grammar — `Request` stays the pure parse tree.
**Rejected**: watch fields on `Request` (pollutes the parse tree with values no flag can express); a separate watch render pipeline (a second pathway to keep byte-aligned).
*Introduced by*: 4pze
