# Plan: All-Users Aggregate View and Token-Metric Bars

**Change**: 260826-svlv-all-users-aggregate-token-bars
**Intake**: `intake.md`

## Requirements

### Sync: All-Users Read

#### R1: `listUsers` enumerates user directories
`src/node/sync/sync.ts` MUST export a pure `listUsers(metricsDir: string): string[]` that returns the sorted top-level directory names of the metrics repo, excluding the non-user `docs` dir (named constant `NON_USER_DIRS`) and any dot-prefixed entry. A missing or unreadable `metricsDir` MUST return `[]` without throwing or writing to stderr.

- **GIVEN** a metrics dir containing `alice/`, `bob/`, `docs/`, `.git/`, and a file `.last-sync`
- **WHEN** `listUsers(dir)` is called
- **THEN** it returns `["alice", "bob"]`

- **GIVEN** a path that does not exist
- **WHEN** `listUsers(path)` is called
- **THEN** it returns `[]`

### CLI: `-u all` Aggregate

#### R2: `-u all` sums every user's repo entries
In multi mode, `fetchToolMerged` MUST treat `targetUser === ALL_USERS` (`"all"`, a named constant) as a repo-only read over every user returned by `listUsers(config.metricsDir)`: per-user `readRemoteEntries(metricsDir, u, null, toolKey)` results are combined with `mergeEntries` (per-label sum, no `maxMergeEntries`), then `filterEntriesByRange` and `aggregateForPeriod` apply exactly as on the `-u <other-user>` path. No `fetchHistory` and no `writeMetrics` run on this branch.

- **GIVEN** two users each with day-files for `2026-08-01` (cost 1.0 / 2.0, tokens 100 / 200)
- **WHEN** the all-users merge runs for `monthly`
- **THEN** the `2026-08` entry has `totalCost` 3.0 and `totalTokens` 300

#### R3: `-u all` per-user breakdown via `--by-machine`
`fetchToolMergedWithMachines` MUST handle `targetUser === ALL_USERS` by building the map **keyed by user name** (`u → readRemoteEntries(metricsDir, u, null, toolKey)`), then reusing the existing `filterMachineMap` → flatten → `mergeEntries` → per-period aggregation path, so `--by-machine -u all` renders per-user columns through the `machineCosts` formatter path; the column legend reads `Users:` (via `FormatOptions.machineLegend`, default `Machines`) so the breakdown is not mislabeled.

- **GIVEN** multi mode, `tu --by-machine -u all`
- **WHEN** the snapshot renders
- **THEN** one breakdown column per user appears, summing to the total, with a `Users: A = …` legend

#### R4: `all` is a reserved username
Right after `readConfig()` in `main()`, `assertUserNotReserved(config)` (module-private guard over the exported pure `isUserReserved(user)` in `src/node/core/cli.ts`) MUST write `Error: config user "all" is reserved (used by -u all)` to stderr and exit `EXIT_USAGE` (2) when `config.user === ALL_USERS`. The guard runs for every path that loads config to read or write metrics — `main()` data commands and `runSync()` (`tu sync`), which is the other day-file writer.

- **GIVEN** `~/.tu.conf` with `user = all`
- **WHEN** `tu cc` runs
- **THEN** stderr carries the reserved-user error and the exit code is 2

#### R5: Single-mode `-u all` warns and clears
`-u all` in single mode MUST hit the existing `-u` guard (`Warning: -u flag requires multi mode — ignoring.`) with no new code.

- **GIVEN** single mode
- **WHEN** `tu mh -u all` runs
- **THEN** the existing warning prints and the command proceeds with local data, exit 0

### CLI: `--metric` Flag

#### R6: `--metric tokens|cost` parsing
`parseGlobalFlags` MUST accept a value-taking `--metric <m>` (no short alias) and expose `metricFlag: BarMetric` (`"cost"` default) on `GlobalFlags`. A missing or invalid value MUST print `Error: --metric requires 'tokens' or 'cost'` to stderr and exit `EXIT_USAGE` (2). The flag and its value MUST be stripped from `filteredArgs`.

- **GIVEN** `["mh", "--metric", "tokens"]`
- **WHEN** parsed
- **THEN** `metricFlag === "tokens"` and `filteredArgs` is `["mh"]`

- **GIVEN** `["mh", "--metric", "bogus"]` or `["mh", "--metric"]`
- **WHEN** the CLI runs
- **THEN** exit code is 2

#### R7: `--metric` threading and snapshot guard
`main()` MUST warn once on stderr (`Warning: --metric applies to history display — ignoring.`) when `metricFlag !== "cost"` and `display !== "history"`, at the same top-level spot as the `--since/--until` guard (so watch snapshot warns once at startup). For history displays the metric MUST be stamped onto the `FormatOptions` handed to both the one-shot dispatchers and the watch-mode `*Lines` variants via the existing `withCap` helper (extended to also carry `metric`). JSON/CSV/MD emitters ignore it silently.

- **GIVEN** `tu -w h --metric tokens`
- **WHEN** each poll renders
- **THEN** bars scale by tokens on every poll, and no per-poll warning is printed

### Display: Metric-Aware Bars

#### R8: History bars scale by the selected metric
`FormatOptions` MUST gain `metric?: BarMetric` (`export type BarMetric = "cost" | "tokens"`). In `renderHistory`, bar values and the footer values MUST be `e.totalTokens` when `metric === "tokens"` (else `e.totalCost`). In `renderTotalHistory`, the per-row bar total and per-tool stacked segments MUST use per-tool `totalTokens` when `metric === "tokens"`; the pivot cells and the `Cost` column remain cost. With `metric` absent or `"cost"`, output MUST be byte-identical to today.

- **GIVEN** entries where the highest-tokens row is not the highest-cost row
- **WHEN** rendered with `{ metric: "tokens" }`
- **THEN** the longest bar is on the highest-tokens row (both renderers)

#### R9: Footer formats by metric
`renderHistoryFooter` MUST receive the same values the bars use and format them via a `fmtMetric(value, metric)` helper — `fmtCost` for cost, `fmtNum` for tokens — so `avg`/`this month`/`peak`/`p95` describe what the bar shows.

- **GIVEN** `{ metric: "tokens" }`
- **WHEN** the footer renders
- **THEN** it reads e.g. `avg 1,234/day` with no `$`

### Docs: Surface Lockstep

#### R10: Help, completions, docs, specs, version
`FULL_HELP` MUST document `all` on the `--user` line and add a `--metric <m>` line; `README.md` `### Flags`, `docs/site/workflows.md` § Multi-machine (plus recipes `tu mh -u all` and `tu mh -u all --metric tokens`), and `docs/site/skill.md` MUST mirror it; `src/node/core/completions.ts` MUST add `--metric` to bash/zsh/fish (zsh completes `(cost tokens)`, bash treats it as value-taking); `docs/specs/usage.md` MUST add `--user`/`--metric` rows and the bad-`--metric` exit-2 case; `docs/specs/layouts.md` MUST note the bar scale follows `--metric`; `package.json` version MUST bump to `0.11.0`.

- **GIVEN** the change is applied
- **WHEN** `tu help` prints
- **THEN** it contains `--metric` and the `--user` line mentions `all`

### Non-Goals
- Merging the caller's live ccusage data into `-u all` — repo-only by design; today lags until `--sync`.
- Switching pivot cells to token counts under `--metric tokens` — bars/segments/footer only.
- A `--all-users` flag or a short alias for `--metric`.

### Design Decisions

#### Cross-user merge is a plain sum
**Decision**: `-u all` combines per-user reads with `mergeEntries` only.
**Why**: `maxMergeEntries` exists to reconcile a machine's live view with its own snapshots; across users there is no live view, and day-files are never-shrink high-water marks, so a sum is exact.
**Rejected**: max-merging across users — would under-count.
*Introduced by*: 260826-svlv-all-users-aggregate-token-bars

#### Bar metric rides `FormatOptions`, defaulting to cost
**Decision**: `--metric` becomes `FormatOptions.metric`, threaded via the existing `withCap` helper; absent means cost.
**Why**: Mirrors how `capActive` reaches every history renderer (one-shot and watch) through one seam; the default keeps all existing output byte-identical.
**Rejected**: Tokens-by-default (output-stability break); a separate rendering entry point (second code path).
*Introduced by*: 260826-svlv-all-users-aggregate-token-bars

## Tasks

### Phase 1: Core Implementation

- [x] T001 Add `NON_USER_DIRS` + `listUsers(metricsDir)` to `src/node/sync/sync.ts`; tests in `src/node/sync/__tests__/sync.test.ts` (2 users × 2 machines + `docs/` + `.git/` fixture; missing dir → `[]`; summed per-user `readRemoteEntries` via `mergeEntries` equals hand-computed cost/token totals) <!-- R1, R2 -->
- [x] T002 In `src/node/core/cli.ts` add `ALL_USERS`, the `-u all` branches in `fetchToolMerged` (sum) and `fetchToolMergedWithMachines` (user-keyed map), and exported `assertUserNotReserved(config)` called after `readConfig()`; tests in `src/node/core/__tests__/cli-user-flag.test.ts` (`-u all` parses; guard rejects `user: "all"` and passes others) <!-- R2, R3, R4, R5 --> <!-- rework: review cycle 1 — reserved-user guard was missing from runSync (tu sync write path); fixed -->
- [x] T003 Add `--metric` parsing to `parseGlobalFlags` (`metricFlag: BarMetric`), the snapshot warn-and-clear in `main()`, and thread `metric` through the `withCap` helper to one-shot and watch dispatchers in `src/node/core/cli.ts`; tests in `cli-parser.test.ts`/`cli-exit-codes.test.ts` (parse `tokens`/`cost`; `--metric bogus` and bare `--metric` exit 2; `-u all` single mode exits 0 with the multi-mode warning) <!-- R6, R7 -->
- [x] T004 In `src/node/tui/formatter.ts` add `BarMetric`, `FormatOptions.metric`, `fmtMetric`, and make `renderHistory`/`renderTotalHistory`/`renderHistoryFooter` metric-aware (bars, stacked segments, footer); tests in `src/node/tui/__tests__/formatter-history.test.ts` (longest bar on highest-tokens row in both renderers; cost/no-option output unchanged; footer uses `fmtNum` under tokens) <!-- R8, R9 -->

### Phase 2: Polish

- [x] T005 Update `FULL_HELP` (`--user` line + `--metric` line), `src/node/core/completions.ts` (bash/zsh/fish), `README.md` § Flags, `docs/site/workflows.md` § Multi-machine + recipes, `docs/site/skill.md`, `docs/specs/usage.md` (flags table + exit table), `docs/specs/layouts.md` (bar-scale note), bump `package.json` to `0.11.0`; extend `cli-help.test.ts` and `completions.test.ts` LONG_FLAGS <!-- R10 -->

## Acceptance

### Functional Completeness

- [x] A-001 R1: `listUsers` returns sorted user dirs, skipping `docs` and dot-prefixed entries; missing dir → `[]`
- [x] A-002 R2: `fetchToolMerged` with `-u all` sums every user's repo entries via `mergeEntries` then filters/aggregates; no `fetchHistory`/`writeMetrics` on that branch
- [x] A-003 R3: `fetchToolMergedWithMachines` with `-u all` builds a user-keyed map and reuses the existing filter/merge/aggregate path; the legend reads `Users:`
- [x] A-004 R4: `assertUserNotReserved` rejects `config.user === "all"` with the specified stderr message and exit 2, and runs after `readConfig()` in `main()`
- [x] A-005 R6: `parseGlobalFlags` exposes `metricFlag` (`"cost"` default), strips `--metric <v>` from `filteredArgs`, and exits 2 on missing/invalid value
- [x] A-006 R7: `--metric` reaches both one-shot and watch history renderers via `FormatOptions`; snapshot display warns once and clears
- [x] A-007 R8: `renderHistory` and `renderTotalHistory` scale bars (and stacked segments) by `totalTokens` under `metric: "tokens"`
- [x] A-008 R9: footer values use `fmtNum` under tokens and `fmtCost` under cost, via a single `fmtMetric` helper
- [x] A-009 R10: help/README/workflows/skill/completions/specs updated in lockstep; `package.json` at `0.11.0`

### Behavioral Correctness

- [x] A-010 R8: With `metric` absent or `"cost"`, `renderHistory`/`renderTotalHistory` output is byte-identical to pre-change
- [x] A-011 R5: `tu mh -u all` in single mode prints the existing multi-mode warning and exits 0

### Scenario Coverage

- [x] A-012 R2: Test proves two users' day-files sum per label for both cost and tokens through `mergeEntries`
- [x] A-013 R8: Test with a fixture whose highest-tokens row differs from its highest-cost row shows the longest bar on the tokens row (both renderers)
- [x] A-014 R6: Subprocess tests pin exit 2 for `--metric bogus` and bare `--metric`

### Edge Cases & Error Handling

- [x] A-015 R1: Unreadable/missing `metricsDir` and malformed day-files are skipped silently (no throw, no stderr)
- [x] A-016 R7: `--metric tokens` combined with `--json`/`--csv`/`--md` is a silent no-op
- [x] A-017 R4: A metrics repo lacking any user dir yields `No data` rather than a crash under `-u all`

### Code Quality

- [x] A-018 Pattern consistency: new flag parsing mirrors `--since`; guards mirror existing warn-and-clear blocks; `node:` imports, `.js` extensions, `type` imports
- [x] A-019 No unnecessary duplication: reuses `mergeEntries`, `filterEntriesByRange`, `aggregateForPeriod`, `filterMachineMap`, `computeBarScale`; footer not forked
- [x] A-020 Named constants: `ALL_USERS`, `NON_USER_DIRS` — no inline magic strings
- [x] A-021 Minimum pathways: metric threads through the single `withCap` seam rather than parallel option plumbing
- [x] A-022 Error paths warn on stderr or exit with a message — nothing swallowed silently
- [x] A-023 Functions stay focused (no new >50-line god function; all-users branches are small early returns)

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | Ship the `--by-machine -u all` per-user breakdown (R3) rather than warn-and-clear | Reuses the existing machine-map branch with a key swap — smaller than a new guard and gives a useful view | S:60 R:85 A:80 D:70 |
| 2 | Confident | Extend `withCap` into a single `withHistoryOpts`-style helper carrying both `capActive` and `metric` | One seam for all history-only options; avoids a second plumbing path | S:65 R:90 A:85 D:75 |
| 3 | Confident | `--metric cost` explicitly on a snapshot does not warn (only non-default `tokens` warns) | `cost` is the default; warning on a no-op value would be noise | S:55 R:85 A:80 D:65 |
| 4 | Tentative | Pivot stacked segments apportion by per-tool `totalTokens` under `tokens`; cells stay cost | Carries intake Assumption 8 forward; bars-only keeps the stability break minimal | S:40 R:70 A:60 D:40 |
| 5 | Certain | Five tasks, one per file cluster, tests co-located per task | Each is a focused session; keeps the change in the light lane honestly | S:80 R:90 A:90 D:85 |

5 assumptions (1 certain, 3 confident, 1 tentative).
