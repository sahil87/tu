# Intake: All-Users Aggregate View and Token-Metric Bars

**Change**: 260826-svlv-all-users-aggregate-token-bars
**Created**: 2026-08-26

## Origin

Conversational — a discussion session about the multi-mode metrics repo (`~/.tu/metrics_repo`), dispatched promptless via `/fab-proceed` (no questions asked at intake; every would-be question is graded in `## Assumptions`). The user's need, as synthesized from the live conversation:

> `tu` has no way to show total usage across all users. Today `-u <user>` scopes to exactly one user. The metrics repo has one top-level dir per user (sahil, akshay, pulkit, vivek, shreyas, shreyasarun, ashishy) plus a non-user `docs/` dir. I specifically need a graph of total **token** usage across months, across all users.

Key decisions reached in the conversation (all carried into this intake verbatim):

1. **One change**, covering both the all-users aggregate and the token-metric bars — the user explicitly chose a single change over two.
2. **Flag surface is `-u all` / `--user all`** — reuse the existing user-scoping flag; a separate `--all-users` flag was rejected as redundant.
3. **Bars default to cost**; `--metric tokens` opts in. Graphing tokens by default was rejected because it would break output stability more than needed.
4. **Repo-only data for `-u all`** (same semantics as `-u <other-user>` today); the caller's own live ccusage fetch is not merged in.

## Why

1. **Pain point.** Multi mode stores every user's day-files under `{metricsDir}/{user}/{year}/{machine}/{tool}-{YYYY-MM-DD}.jsonl`, but the read side (`fetchToolMerged` / `fetchToolMergedWithMachines` in `src/node/core/cli.ts`) only ever calls `readRemoteEntries(metricsDir, targetUser, null, toolKey)` for a single user directory. There is no team-wide total — the one number a team lead actually wants from a shared metrics repo. Separately, every history bar chart scales on `totalCost` (`computeBarScale(entries.map((e) => e.totalCost), barWidth)` in `renderHistory` and `rowData.map((r) => r.rowTotal)` in `renderTotalHistory`, `src/node/tui/formatter.ts`), so there is no way to *see* token volume as a graph even though `totalTokens` is already on every `UsageEntry`.
2. **Consequence of not fixing.** Team-level reporting has to be done by hand (cloning the repo and summing JSONL files) or not at all. Token-volume trends — which drive plan/quota decisions independently of cost — stay invisible.
3. **Why this approach.** `-u all` is the smallest possible surface: it composes with every existing period/display/format flag and reuses the exact repo-read path that `-u <other-user>` already exercises, so the aggregate is a pure sum over per-user reads (correct because day-files are never-shrink high-water marks — see `docs/memory/sync/multi-machine.md`). `--metric tokens|cost` is a single rendering knob threaded through the existing `FormatOptions` object, mirroring how `capActive` was threaded for the 3-month cap; defaulting to `cost` keeps every existing invocation byte-identical.

## What Changes

### 1. `listUsers(metricsDir)` — new pure function in `src/node/sync/sync.ts`

```ts
// Top-level user directories of the metrics repo: directories only, sorted,
// skipping the non-user `docs/` dir and dot-prefixed entries (`.git`, `.last-sync`
// is a file and is excluded by the directory filter anyway).
export function listUsers(metricsDir: string): string[]
```

Behavior:

- `readdirSync(metricsDir, { withFileTypes: true })`, keep `isDirectory()`, drop `docs` and any name starting with `.`, return sorted.
- Missing/unreadable `metricsDir` → `[]` (try/catch, same silent-skip posture as `readRemoteEntriesByMachine`). No throw, no stderr — a missing repo is already reported by `checkMetricsDirGuard`.
- Named constant for the excluded dir (e.g. `NON_USER_DIRS = new Set(["docs"])`) — no magic string inline.

### 2. `-u all` in `fetchToolMerged` (`src/node/core/cli.ts`)

Add a reserved-value constant `ALL_USERS = "all"` and a branch **before** the existing `targetUser !== config.user` branch:

```ts
if (targetUser === ALL_USERS) {
  const perUser = listUsers(config.metricsDir).map((u) => readRemoteEntries(config.metricsDir, u, null, toolKey));
  const summed = perUser.reduce((acc, entries) => mergeEntries(acc, entries), [] as UsageEntry[]);
  return aggregateForPeriod(period, filterEntriesByRange(summed, since, until));
}
```

- `mergeEntries` is the existing per-label **sum** in `src/node/core/fetcher.ts`. Cross-user merge is a pure sum — no `maxMergeEntries` self-view correction, because that correction exists only to reconcile a machine's *live* ccusage view with its own synced snapshots; across users there is no live view, and each user's day-files are never-shrink high-water marks.
- Repo-only: the caller's own user is read from the repo like everyone else (no `fetchHistory`, no `writeMetrics`). Today's numbers therefore lag until `--sync` — identical to `-u <other-user>` today. This must be stated in help/docs (see §6).
- The `-u <user>` single-user and self-view paths are untouched.

### 3. `-u all` in `fetchToolMergedWithMachines` — nice-to-have

Reuse the existing `targetUser !== config.user` machine-map branch but build the map **keyed by user name** instead of machine: for each `u` of `listUsers`, `machineMap.set(u, readRemoteEntries(config.metricsDir, u, null, toolKey))`, then the existing `filterMachineMap` → flatten → `mergeEntries(entries, [])` → per-period aggregation. `--by-machine -u all` then renders a **per-user** breakdown through the existing `machineCosts` columns with zero formatter changes (the column letter comes from the key's first character, as it does for machines today).

This is explicitly **optional for v1**: plan it as a final-phase task that MAY be dropped if it does not fit the ~80–120 LOC budget. If dropped, `--by-machine` combined with `-u all` MUST warn-and-clear on stderr (`Warning: --by-machine is not supported with -u all — ignoring.`), matching the existing `--by-machine` + all-tools-history guard, so the combination never silently renders a wrong breakdown.

### 4. `-u all` mode guards

- **Single mode**: no new code — `-u all` hits the existing guard at `if (userFlag && config.mode !== "multi")` → `Warning: -u flag requires multi mode — ignoring.` and clears the flag.
- **Reserve `all` as a username**: `-u all` would otherwise be indistinguishable from a real user named `all`, and `writeMetrics` would create a `{metricsDir}/all/` dir that `listUsers` then double-counts. Fail loud in `src/node/core/cli.ts` right after `readConfig()` (next to `checkMetricsDirGuard`), for **every** command path that loads config: `Error: config user "all" is reserved (used by -u all)` on stderr, exit `2` (a bad config value is an invocation-fixable error under the Exit-Code Convention). Extract as a small pure guard (e.g. `assertUserNotReserved(config)`) so it is unit-testable without spawning the CLI.

### 5. `--metric tokens|cost` flag

**Parsing** (`parseGlobalFlags`): value-taking flag in the same shape as `--since`:

```ts
if (a === "--metric") {
  hasMetricFlag = true;
  const next = rawArgs[i + 1];
  if (next !== undefined && !next.startsWith("-")) { rawMetricVal = next; i++; }
  continue;
}
// after the loop:
if (hasMetricFlag) {
  if (rawMetricVal !== "tokens" && rawMetricVal !== "cost") {
    console.error("Error: --metric requires 'tokens' or 'cost'");
    process.exit(EXIT_USAGE);   // 2
  }
}
```

- `GlobalFlags` gains `metricFlag: "cost" | "tokens"` (default `"cost"`). Add a `type BarMetric = "cost" | "tokens"` in `formatter.ts` and reuse it in `cli.ts`. No short alias.
- Missing or invalid value → usage error, exit 2 (same as `--since` without a date).

**Threading** (`FormatOptions` in `src/node/tui/formatter.ts`): add `metric?: BarMetric` (absent ≡ `"cost"`). In `cli.ts`, merge it exactly the way `capActive` is merged via `withCap` — one helper (or extend `withCap` into a `withHistoryOpts`) that stamps `metric` onto the `FormatOptions` handed to **both** the one-shot dispatchers and the watch-mode `*Lines` variants (`dispatchAllHistoryLines`, `dispatchSingleToolLines`), so `tu -w h --metric tokens` behaves like `tu h --metric tokens`.

**Rendering** — history displays only:

- `renderHistory` (single-tool): bar values become `e.totalTokens` when `metric === "tokens"`; scale via `computeBarScale(values, barWidth)` unchanged. The table columns are unchanged (the `Total` tokens column already shows the number the bar represents).
- `renderTotalHistory` (all-tools pivot, incl. stacked bars): `rowTotal`/`toolCosts` used for the bar and stacked segments become per-tool `totalTokens` sums when `metric === "tokens"`; cells and the `Cost` column keep showing cost. Stacked segment proportions therefore follow tokens under `--metric tokens`.
- `renderHistoryFooter` receives the same metric values as the bars, so `avg` / `this month` / `peak` / `p95` describe what the bar shows; under `tokens` these format with `fmtNum` (e.g. `avg 12,345,678/day`) instead of `fmtCost`. Add a `fmtMetric(value, metric)` helper; do not fork the footer.
- `--metric` on a **snapshot** display: warn once and clear, mirroring the `--since/--until` guard — `Warning: --metric applies to history display — ignoring.` — placed at the same top-level spot so watch snapshot warns once at startup, not per poll.
- JSON / CSV / Markdown emitters have no bars → `--metric` is silently a no-op there (same as `--no-color`).
- Heading text is unchanged. `--metric cost` is byte-identical to today's output.

### 6. Help, completions, docs, version

- `FULL_HELP` (`src/node/core/cli.ts`): change the `--user` line to `--user / -u <user>   Show usage for a specific user, or 'all' for every user in the metrics repo (multi mode only; repo data, sync for today)` (wrap to keep the two-column alignment), and add `--metric <m>        Scale history bars by 'cost' (default) or 'tokens' (history display)` adjacent to `--full`. `SHORT_USAGE` unchanged.
- `README.md` `### Flags` block and `docs/site/workflows.md` § Multi-machine mirror the help text verbatim (the shll readme-extraction standard cross-checks README flags against `help-dump`; a flag present in help but absent from README is a lint warning, so keep them in lockstep). Add two recipe lines to workflows.md: `tu mh -u all` (team monthly cost) and `tu mh -u all --metric tokens` (team monthly token volume). `docs/site/skill.md` § flags gets a one-line mention of `-u all` and `--metric`.
- `src/node/core/completions.ts`: add `--metric` to both bash and zsh long-flag lists; zsh completes `:metric:(cost tokens)`; bash treats it as value-taking alongside `--interval|--user|--since|--until`.
- `docs/specs/usage.md`: add `-u all` to the CLI grammar table and `--metric` to the history-display notes and the exit-code table (bad `--metric` value → 2).
- `docs/specs/layouts.md`: note that history bar scale follows `--metric`.
- `help-dump` needs no code change (it captures `FULL_HELP` verbatim) — but its snapshot test, if any, must be refreshed.
- **Minor version bump** (0.10.x → 0.11.0) per constitution § Output Stability: new help lines and a new bar-scaling mode are output-format changes.

### 7. Tests (Node built-in runner, co-located `__tests__/`)

- `src/node/sync/__tests__/sync.test.ts` (or a new `sync-list-users.test.ts`): fixture with 2 users × 2 machines plus a `docs/` dir and a `.git/` dir → `listUsers` returns exactly the two users, sorted; a missing `metricsDir` returns `[]`.
- Merged-totals test: the same fixture, summed through `mergeEntries` over per-user `readRemoteEntries`, equals the hand-computed per-label totals for cost and tokens (exercises the all-users path's arithmetic; `fetchToolMerged` itself is not exported, mirroring the note in `cli-user-flag.test.ts`).
- `src/node/core/__tests__/cli-user-flag.test.ts`: `parseGlobalFlags(["mh", "-u", "all"])` yields `userFlag: "all"`; the reserved-user guard rejects `config.user === "all"` (call the extracted guard directly).
- `src/node/core/__tests__/cli-exit-codes.test.ts`: `--metric bogus` and bare `--metric` exit 2; `-u all` in single mode prints the existing multi-mode warning and exits 0.
- `src/node/tui/__tests__/formatter-history.test.ts`: with `{ metric: "tokens" }`, a fixture whose highest-tokens row is not its highest-cost row renders the longest bar on the highest-**tokens** row (both `renderHistory` and `renderTotalHistory`); with `{ metric: "cost" }` / no option output is identical to today's expected strings; footer shows `fmtNum` values under `tokens`.
- `cli-help.test.ts`: `FULL_HELP` contains `--metric` and mentions `all` on the `--user` line.

## Affected Memory

- `sync/multi-machine`: (modify) all-users aggregate read (`listUsers`, pure-sum cross-user merge, repo-only semantics, reserved `all` username)
- `cli/data-pipeline`: (modify) `-u all` resolution in `fetchToolMerged`/`fetchToolMergedWithMachines`, `--metric` global flag parsing + snapshot warn-and-clear, reserved-user config guard
- `display/formatting`: (modify) bar scale metric (`FormatOptions.metric`), token-formatted footer, stacked segments by tokens

## Impact

- `src/node/sync/sync.ts` — `listUsers` (+ constant for non-user dirs)
- `src/node/core/cli.ts` — `ALL_USERS` constant, `fetchToolMerged` / `fetchToolMergedWithMachines` all-users branches, reserved-user guard, `--metric` parsing, snapshot warn-and-clear, `FormatOptions` threading (one-shot + watch), `FULL_HELP`
- `src/node/tui/formatter.ts` — `BarMetric` type, `FormatOptions.metric`, `renderHistory` / `renderTotalHistory` / `renderHistoryFooter` metric-aware values
- `src/node/core/completions.ts` — `--metric`
- `README.md`, `docs/site/workflows.md`, `docs/site/skill.md`, `docs/specs/usage.md`, `docs/specs/layouts.md`
- `package.json` — minor version bump
- Tests as listed in §7
- Expected size: ~80–120 LOC implementation + tests. No new dependencies; single esbuild bundle unchanged; all data still flows through `UsageEntry` / `mergeEntries` / `aggregateForPeriod` / `filterEntriesByRange`.

## Open Questions

- Whether `--by-machine -u all` (per-user columns, §3) ships in v1 or is warn-and-cleared — plan decides against the LOC budget; either outcome is acceptable to the user.
- Whether the all-tools pivot under `--metric tokens` should also switch its **cells** to token counts (so the bar has a visible number) — deliberately kept as bars-only here (Assumption 8); revisit if the user finds the pivot unreadable.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Single change for both `-u all` and `--metric` | Discussed — user explicitly chose one change over two | S:90 R:85 A:90 D:95 |
| 2 | Certain | Reuse `-u all` / `--user all`; no separate `--all-users` flag | Discussed — user rejected a second flag as redundant with `-u` | S:90 R:80 A:90 D:90 |
| 3 | Certain | `-u all` is repo-only (no live ccusage merge for own user); today lags until `--sync` | Discussed — same semantics as `-u <other-user>`; documented in help/docs | S:85 R:75 A:90 D:85 |
| 4 | Certain | Cross-user merge is a pure per-label sum via `mergeEntries`, no `maxMergeEntries` | Discussed — day-files are never-shrink high-water marks; self-view max-merge only reconciles a live view with its own snapshots | S:85 R:80 A:90 D:90 |
| 5 | Certain | Bars default to `cost`; `--metric tokens` opts in | Discussed — user rejected tokens-by-default to limit output-stability impact | S:90 R:85 A:90 D:90 |
| 6 | Certain | Minor version bump ships with this change | Constitution § Output Stability mandates it for help/bar-format changes | S:85 R:90 A:95 D:95 |
| 7 | Certain | `listUsers` skips `docs` and dot-prefixed dirs, directories only, sorted; missing dir → `[]` | Discussed — repo layout has exactly one non-user dir today; dot-skip covers `.git`; silent-skip matches existing sync read posture | S:75 R:85 A:85 D:80 |
| 8 | Tentative | Under `--metric tokens` the pivot keeps cost cells/Cost column; only bars, stacked segments and footer stats follow tokens | User asked for token *bars*; switching cells is a larger stability break and the single-tool history already shows a Total tokens column — but the pivot's bar then has no visible number | S:40 R:70 A:45 D:30 |
| 9 | Confident | Footer stats (`avg`/`this month`/`peak`/`p95`) follow the bar metric and format with `fmtNum` under `tokens` | A cost footer under a token-scaled bar would mislabel the p95 rule; single helper keeps one code path | S:45 R:80 A:70 D:65 |
| 10 | Confident | Reserved-username guard lives in `cli.ts` after `readConfig()` (all commands), stderr error, exit 2 | User asked for fail-loud in `writeMetrics` or config load; a single early guard covers both and `config.ts` has no exit-code convention of its own | S:60 R:85 A:75 D:65 |
| 11 | Confident | `--by-machine -u all` per-user breakdown is a final-phase optional task; if dropped, the combination warns-and-clears | Discussed — user marked it nice-to-have / may be deferred; warn-and-clear mirrors the existing `--by-machine` + all-tools-history guard | S:55 R:85 A:65 D:55 |
| 12 | Confident | `--metric` on snapshot displays warns once and is cleared; silent no-op for JSON/CSV/MD | Mirrors `--since/--until/--full` guards named by the user; non-table emitters have no bars | S:70 R:85 A:80 D:80 |
| 13 | Certain | Invalid/missing `--metric` value → usage error exit 2; no short alias | Named by the user; matches `--since` parsing shape and the Exit-Code Convention | S:80 R:90 A:90 D:85 |
| 14 | Confident | History headings unchanged for `-u all` and `--metric` | `-u <user>` does not annotate headings today; consistency + output stability | S:55 R:85 A:75 D:65 |
| 15 | Certain | README/workflows/skill/completions/specs updated in lockstep with `FULL_HELP` | shll readme-extraction standard cross-checks README flags vs help-dump; constitution § Toolkit Standards | S:75 R:90 A:85 D:85 |

15 assumptions (9 certain, 5 confident, 1 tentative, 0 unresolved).
<!-- assumed: pivot cells stay cost-denominated under --metric tokens; only bars, stacked segments and footer follow the metric -->
