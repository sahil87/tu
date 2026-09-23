---
type: memory
description: "Removal records for retired memory domains and files — cli, configuration, display, watch-mode, go-port and sync/multi-machine — each naming the package-aligned successor domain(s) and topic files that now hold the subject."
---
# Removed Domains

**Domain**: _shared

## Overview

Removal records lifted from the generated indexes when the memory tree was reshaped from the Node-era domains to one domain per `src/go/internal/` pipeline package plus the kept `build` and `harness` domains (k7y1). Each record keeps the retired index row verbatim and names where its subject lives now; the retired files themselves remain in git history and in the change folders under `fab/changes/`.

## Removed domains

Rows lifted verbatim from the root `index.md`, with their successors:

| Domain | Description | Successor domain(s) |
|--------|-------------|---------------------|
| [cli](cli/index.md) | — | [command](/command/index.md), [source](/source/index.md), [query](/query/index.md), [fact](/fact/index.md) |
| [configuration](configuration/index.md) | — | [config](/config/index.md) |
| [display](display/index.md) | — | [view](/view/index.md), [render](/render/index.md) |
| [watch-mode](watch-mode/index.md) | — | [watch](/watch/index.md) |
| [go-port](go-port/index.md) | — | all ten package domains: [fact](/fact/index.md), [source](/source/index.md), [query](/query/index.md), [view](/view/index.md), [render](/render/index.md), [command](/command/index.md), [sync](/sync/index.md), [config](/config/index.md), [watch](/watch/index.md), [toolkit](/toolkit/index.md) |

## Removed file in a kept domain

Row lifted verbatim from `sync/index.md`, with its successors:

| File | Description | Successor file(s) |
|------|-------------|-------------------|
| [multi-machine](multi-machine.md) | Git-based metrics sync, JSONL high-water-mark storage (never-shrink writes, self-view max-merge), remote merging, user-directory enumeration for the all-users aggregate (incl. the user/machine-keyed leaderboard reader and its .last-sync staleness footer), auto-clone, init-metrics [repo-url] bootstrap (writes metrics_repo then clones), repair script, dry-run preview (shared decision path, no-mutation, local git preview) | [day-file-writer](/sync/day-file-writer.md), [git-flow](/sync/git-flow.md), [dry-run-report](/sync/dry-run-report.md), [repair](/sync/repair.md), [metrics-reader](/source/metrics-reader.md) |

## Former contents of the removed domains

Topic rows lifted from each retired domain's `index.md` (a literal `|` in one description is escaped), mapped to the files that now cover the subject.

### cli

| File | Description | Successor file(s) |
|------|-------------|-------------------|
| [data-pipeline](data-pipeline.md) | CLI argument parsing (incl. the sync-only --dry-run global flag + fail-fast misuse guard, the --metric/-t display unit, the lb/lbh leaderboard displays + --top flag, -u all aggregate and the reserved "all" profile guard), data fetching, caching, tool registry, live-verified ccusage v20 per-agent daily JSON shapes (codex costUSD outlier; empty agent → daily: [], -0.0 totals) | [request-and-parse](/command/request-and-parse.md), [guards](/command/guards.md), [entry-point](/command/entry-point.md), [ccusage-adapter](/source/ccusage-adapter.md), [ccusage-json-shapes](/source/ccusage-json-shapes.md), [cache](/source/cache.md), [tool-registry](/fact/tool-registry.md), [aggregation](/query/aggregation.md) |

### configuration

| File | Description | Successor file(s) |
|------|-------------|-------------------|
| [config-system](config-system.md) | INI config format, $HOME-only path resolution (~/.config/tu/tu.conf), cascade tu.default.conf < org.conf < tu.conf < TU_METRICS_REPO < CLI, legacy ~/.tu.conf fallback with deprecation warning, sentinel expansion, init-conf/init-metrics scaffolding | [cascade](/config/cascade.md), [setup-commands](/config/setup-commands.md) |

### display

| File | Description | Successor file(s) |
|------|-------------|-------------------|
| [formatting](formatting.md) | Table rendering, the cell/bar/footer unit (cost or tokens via FormatOptions.metric and the -t shorthand), delta indicators, color system, snapshot cache column/width budget, history month separators/footer/p95 bar scale, weekend date dimming, stacked pivot tool bars and footer legend, the lb/lbh leaderboard layouts and emit kinds, machine/user breakdown columns, data-sized metric/machine columns, dimmed exact-zero cells, negligible tool-column omission | [table-model](/view/table-model.md), [snapshot](/view/snapshot.md), [history](/view/history.md), [leaderboard](/view/leaderboard.md), [bars-and-deltas](/view/bars-and-deltas.md), [breakdown](/view/breakdown.md), [number-formatting](/render/number-formatting.md), [ansi](/render/ansi.md), [csv-and-markdown](/render/csv-and-markdown.md) |

### watch-mode

| File | Description | Successor file(s) |
|------|-------------|-------------------|
| [tui](tui.md) | Live polling TUI, compositor architecture, sparkline, rain animation, session stats | [loop-and-terminal](/watch/loop-and-terminal.md), [compositor](/watch/compositor.md), [panel-and-rain](/watch/panel-and-rain.md) |

### go-port

| File | Description | Successor file(s) |
|------|-------------|-------------------|
| [command-edge](command-edge.md) | The Go port's command edge — internal/command (Request, byte-exact Parse, ShortUsage/FullHelp, Normalize's guards + 3-month cap, Run(cfg) composing snapshot/history and the lb/lbh leaderboards into a Result, Deps.Live carrying per-poll watch render options, the Repo/Fetcher/Writer seams, ErrUnported for --skip-brew-update only) and cmd/tu as the only writer incl. the -w watch branch (watch-mode); sync via metrics-sync; config: config-and-setup; toolkit: toolkit-layer — 452/452 harness green | [request-and-parse](/command/request-and-parse.md), [guards](/command/guards.md), [run-and-result](/command/run-and-result.md), [entry-point](/command/entry-point.md) |
| [config-and-setup](config-and-setup.md) | The Go port's config layer and setup commands — internal/config (path resolution, StateDir/Tildefy/ExpandHome, embedded drift-guarded tu.default.conf, Load's five-layer cascade + legacy/version warnings, InitConf, InitMetrics with the metrics_repo write and CloneStep handoff, Status/Lines/RelativeTime/LastSync (shared with the lb footer), the MetricsDirGuard auto-clone fallback + .clone-failed marker behind Cloner), sync's Exec git driver and Exec.Run (writer: metrics-sync), cmd/tu dispatch | [cascade](/config/cascade.md), [setup-commands](/config/setup-commands.md), [status](/config/status.md), [metrics-dir-guard](/config/metrics-dir-guard.md), [git-flow](/sync/git-flow.md) |
| [fact-and-sources](fact-and-sources.md) | The Go port's input layer — internal/fact (Record/Totals, six-tool registry), internal/source (typed Error, WriteWarnings, PeriodDaily/DefaultTimeout), internal/source/ccusage (invocations map, exec, Fetch/FetchAll, edge-stamped User/Machine), internal/source/metrics (the read-only metrics-repo reader sharing the DayFile/Name/Path encoding with the sync writer — metrics-sync), internal/source/cache (hash-keyed JSON, 60 s TTL); consumed by command and cmd/tu, unshipped until cutover | [records](/fact/records.md), [tool-registry](/fact/tool-registry.md), [errors-and-warnings](/source/errors-and-warnings.md), [ccusage-adapter](/source/ccusage-adapter.md), [metrics-reader](/source/metrics-reader.md), [cache](/source/cache.md) |
| [metrics-sync](metrics-sync.md) | The Go port's metrics-repo writer and git flow — internal/sync (Write + the never-shrink guard and its Number() coercion table, the shared metrics.DayFile encoding, CommitMessage/TouchLastSync/Stale, Exec.Run's Node-shaped error text, SyncMetrics' add/commit/pull/push round trip, FullSync live/dry-run with Report.Format's byte rules, the command.Writer adapter) and the repair twin (sync.Repair, cmd/turepair, the ASCII ICU-root comparator); cmd/tu answers tu sync, --dry-run and --sync | [day-file-writer](/sync/day-file-writer.md), [git-flow](/sync/git-flow.md), [dry-run-report](/sync/dry-run-report.md), [repair](/sync/repair.md) |
| [multi-mode](multi-mode.md) | The Go port's multi mode — the auto-clone guard at the edge, command.gather's four record paths (live fetch, the own-user never-shrink day-file write via Deps.Writer then MaxMerge + other-machine sum, the repo-only -u paths — no ccusage calls) returning un-collapsed records the callers Collapse before the tail, the repo-only leaderboards lb/lbh over gatherAllUsers (ranking, lbh's tool-major left fold), float summation order pinned to the TS, the mode-keyed snapshot label rule, the 3-month cap | [multi-mode](/command/multi-mode.md), [metrics-dir-guard](/config/metrics-dir-guard.md) |
| [query-view-render](query-view-render.md) | The Go port's middle and output layers — query (Period/labels, ThreeMonthFloor, Relabel, RollUp, MaxMerge, Collapse over one GroupBy), view (ANSI-free Table and CompactTable models, Snapshot, History/TotalHistory (lbh hooks, MaxRows watch budget), ranked Leaderboard, bars, cell-level/trailing watch deltas, Breakdown), render (FormatInt/FormatCost, FixedHalfUp/JSRound twins) + ansi (Colors, BarAfter/leader cells, DeltaInCell placements, compact encoder)/csv/markdown/json — pure, golden-pinned | [periods-and-windows](/query/periods-and-windows.md), [aggregation](/query/aggregation.md), [table-model](/view/table-model.md), [snapshot](/view/snapshot.md), [history](/view/history.md), [leaderboard](/view/leaderboard.md), [bars-and-deltas](/view/bars-and-deltas.md), [breakdown](/view/breakdown.md), [number-formatting](/render/number-formatting.md), [ansi](/render/ansi.md), [json](/render/json.md), [csv-and-markdown](/render/csv-and-markdown.md) |
| [toolkit-layer](toolkit-layer.md) | The Go port's toolkit layer — internal/toolkit (help-dump envelope via BuildHelpDoc/Encode, no HTML escaping, BareVersion/DisplayVersion/VersionLine, the Brew driver seam with verbatim argv, SIGTERM-graceful 600s/60s bounds and unbounded HOMEBREW_NO_ASK=1 upgrade, the /Cellar/tu/ install gate, embedded completions and skill.md with drift guards) and cmd/tu sequences + exit codes for help/-h/--help, help-dump, skill, shell-init and update; standards audit record (shll v0.1.32, findings S1/V1) | [version-and-help-dump](/toolkit/version-and-help-dump.md), [update](/toolkit/update.md), [shell-init-and-completions](/toolkit/shell-init-and-completions.md), [skill-bundle](/toolkit/skill-bundle.md), [standards-audit](/toolkit/standards-audit.md) |
| [watch-mode](watch-mode.md) | The Go port's watch mode — internal/watch: the x/term + os/signal TUI loop (D12; Run's single select over keys/SIGWINCH/SIGINT/poll results/countdown/rain, re-entrancy guard, cleanup returning the last lines), the Terminal seam (80×24 fallback, TTY-gated raw mode keeping OPOST\|ONLCR), the pure compositor (Lay/Frame, StatsGrid/BurnRate, footer, skeleton, RainState on an injected rand/v2), golden-frame tests under a fake clock, and the Frame/Stats/Poll contract command.Run fills per poll | [loop-and-terminal](/watch/loop-and-terminal.md), [compositor](/watch/compositor.md), [panel-and-rain](/watch/panel-and-rain.md) |
