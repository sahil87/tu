# Intake: Fix Metrics History Destruction & Self-Exclusion Blind Spot

**Change**: 260610-srmi-fix-metrics-history-destruction
**Created**: 2026-06-10
**Status**: Draft

## Origin

Conversational. A `/fab-discuss` session investigated why `tu dh` output changed drastically between two runs on the same day (2026-06-10), focusing on 2026-04-24 dropping from $308.12 to $9.46. The investigation reconstructed the full causal chain from the metrics repo's git history (`~/.tu/metrics_repo`), the local ccusage cache, and the source code. The user then queued three fixes as ordered backlog items (`r01f`, `87hw`, `gsji` — in the **main worktree's** `fab/backlog.md`, not this worktree's) and requested this draft bundling all three into one change:

> Fix metrics history destruction and self-exclusion blind spot in multi-machine sync, plus one-time repo repair. Three ordered parts: (1) Guard writeMetrics (src/node/sync/sync.ts:22-36) so a sync can never shrink history — Claude Code purges transcripts >30 days old, so live ccusage data for old dates collapses toward zero, and every fullSync currently overwrites correct historical per-day JSONL files with post-purge residue. Fix: skip the write when the existing file's totals exceed the new entry's. (2) One-time repair script that walks the metrics repo git history and restores every shrunk day-file to its historical max value, across all users/machines; run manually after the guard ships. (3) Fix the self-exclusion blind spot: a machine's own repo dir is excluded from remote reads and replaced by the live local fetch, so once local transcripts are purged a machine cannot see its own synced history. Fix: merge own-machine repo entries into the local view via per-day max.

Key decisions from the discussion: bundle all three parts in one change (user approved); guard = skip-write (not per-field max); repair = standalone script in `scripts/`, dry-run by default, executed manually post-release; self-view fix = per-day whole-entry **max** (not sum, to avoid double-counting partially-purged days).

## Why

**The problem.** Claude Code deletes session transcripts older than ~30 days. ccusage computes daily costs by scanning those transcripts, so a machine's *live* view of any day older than the retention window collapses toward $0. tu's multi-machine sync treats the live fetch as authoritative and **unconditionally overwrites** the per-day JSONL snapshots in the shared metrics repo (`writeMetrics`, `src/node/sync/sync.ts:29-35`). The result is permanent, silent destruction of correct historical data, re-triggered every time the retention window rolls forward.

**Measured damage** (metrics repo, user `sahil`, diff `f509276` → HEAD as of 2026-06-10): **$5,160.63 destroyed across 21 day-files**. Example: commit `0bb81ca` (2026-05-30, from `dev-ws-sahil02`) rewrote 16 April files in one sync — `cc-2026-04-24.jsonl` went $308.12 → $9.46, `cc-2026-04-23.jsonl` $1,016.32 → $49.24, `cc-2026-04-22.jsonl` $840.12 → $5.27. `Sahils-Mac-mini.local` shows the same pattern (`cc-2026-04-26.jsonl` $18.78 → $2.73 on 2026-06-03). Other users (akshay, pulkit, shreyas, vivek) almost certainly have the same rot. The destruction is **ongoing**: early-June syncs shrank 2026-05-03..05-09 files as those dates crossed the retention horizon.

**A second, compounding bug.** When displaying data, a machine excludes its *own* directory from repo reads (`readRemoteEntriesByMachine`, `src/node/sync/sync.ts:125`) and substitutes the live local fetch — under the assumption that live local data is a superset of the machine's repo snapshots. The retention purge breaks that assumption: once transcripts are gone, the machine's own synced history is invisible *to itself*, even though it sits intact in the repo. Concretely: `Sahils-MacBook-Pro.local` has $236.00 recorded for 2026-04-24 in the repo, but `tu dh` on that MacBook shows $0 of it. The true 2026-04-24 spend was ~$544 (devws $308.12 + MacBook $236.00); the user never saw a correct number — old output $308.12 (Bug B hid the MacBook's share), new output $9.46 (Bug A destroyed the devws share, Bug B still hides the MacBook's).

**If we don't fix it:** every machine destroys another slice of shared history each time it syncs after a purge, and every machine under-reports its own past. The repo's git history still holds the true values today; the longer the wait, the more noise accumulates on top.

**Why this approach:** the day-file snapshots are exactly a high-water mark of complete data — "never shrink" restores their intended semantics at the single choke point both callers share. Restoring from git history is lossless because nothing was ever deleted, only overwritten in newer commits. Per-day max for the self-view reuses the same high-water-mark principle for display.

## What Changes

### 1. Never-shrink guard in `writeMetrics` (`src/node/sync/sync.ts`)

Current behavior (sync.ts:29-35): for every entry in the live fetch, unconditionally `writeFileSync` the day-file. Called from **two** sites — `fetchToolMerged` (`src/node/core/cli.ts:467`, i.e. on *every* data-displaying invocation in multi mode) and `fullSync` (`src/node/sync/sync.ts:188`). The guard must therefore live inside `writeMetrics` itself, covering both.

New behavior, per entry:

- If the day-file does not exist → write (unchanged).
- If it exists and parses as a `UsageEntry`: write the incoming entry **only if** `incoming.totalCost >= existing.totalCost`; otherwise skip silently, keeping the existing file.
- If it exists but is empty/unparseable → write (treat as absent; matches the read path's skip-silently posture).
- Equal values → write (idempotent refresh; keeps today's file updating normally as the day grows).

Rejected alternative: per-field `max` across the two entries — it fabricates a chimera entry whose token fields and cost come from different snapshots, violating Constitution V (consistent data model). The whole-entry rule keeps every file an atomic snapshot that was real at some point in time.

Out of scope (possible follow-up, discussed but excluded by the user's backlog selection): a stderr warning when the guard skips shrunk entries.

### 2. One-time repair script (`scripts/repair-metrics.mjs`, new)

Standalone Node script (precedent: `scripts/help-dump.mjs`) — **not** bundled into `dist/tu.mjs`; Constitution III untouched. Run manually: `node scripts/repair-metrics.mjs [--repo ~/.tu/metrics_repo] [--write]`.

Algorithm:

1. Enumerate every commit touching `*.jsonl` once (`git log --format=%H --name-only -- '*/2026/**'` style walk over the full history of `main`), building a per-file commit list — avoids a `git log` per file.
2. For each tracked day-file, `git show <sha>:<path>` across its commits to find the version with **maximum `totalCost`** (the historical high-water mark).
3. Compare with the working-tree/HEAD value. If HEAD is lower by more than a cent, the file is "shrunk".
4. **Dry-run (default):** print a per-file table — path, HEAD value, max value, commit/date of max, delta — plus per-user and grand totals. No writes.
5. **`--write`:** restore each shrunk file's content (the full original JSON line, not just the cost field) in the working tree. Committing and pushing are deliberately left to the user for review. Idempotent — re-running reports nothing left to repair.

Scope: all users and machines in the repo (akshay/pulkit/shreyas/vivek included, not just sahil).

**Sequencing constraint (the reason the backlog items are ordered):** the repair must run only after part 1 has shipped and the actively-syncing machines have upgraded — otherwise the next sync from an old binary re-clobbers restored values at the rolling retention edge (currently ~mid-May dates). April-era dates are already outside every machine's live window and are safe either way.

### 3. Self-view max-merge (`src/node/core/cli.ts` + `src/node/core/fetcher.ts`)

Current behavior (`fetchToolMerged`, cli.ts:449-472): `local = fetchHistory(...)` (live ccusage) → `writeMetrics(local)` → `remote = readRemoteEntries(metricsDir, config.user, /* excludeMachine */ config.machine, toolKey)` → `mergeEntries(local, remote)`, where `mergeEntries` (fetcher.ts:260) **sums** token/cost fields by date label. The machine's own repo snapshots are never read back.

New behavior:

1. Add a pure helper in `fetcher.ts` (Constitution V — pure function over `UsageEntry[]`): `maxMergeEntries(a, b)` — per date label, pick **whichever whole entry has the greater `totalCost`** (no field mixing, no summing). Output sorted by label like `mergeEntries`.
2. In `fetchToolMerged`: read the machine's own snapshots (its single machine dir) and compute `effectiveLocal = maxMergeEntries(local, ownSnapshots)`; then `mergeEntries(effectiveLocal, remote)` as before. For dates within the live window, live wins (equal or greater — it includes in-flight today data); for purged dates, the repo snapshot resurfaces.
3. Same treatment in `fetchToolMergedWithMachines` (cli.ts:481+), so `--by-machine` shows the corrected own-machine column from the same `effectiveLocal`.
4. Unaffected paths: `-u <other-user>` (targetUser ≠ config.user) is already repo-only with `excludeMachine = null`; single mode has no repo. Watch mode uses the same fetch path and is fixed for free.

Why max, not sum: for a partially-purged date the live fetch still returns a residual entry (e.g. $9.46 of the true $308.12); summing residual + snapshot would double-count the surviving transcripts.

Expected user-visible outcome (real data): on the MacBook, 2026-04-24 displays its own $236.00 again (plus whatever the repo holds for other machines — $9.46 pre-repair, $308.12 post-repair). Merged totals only ever increase relative to today's behavior.

## Affected Memory

- `sync/multi-machine.md`: (modify) — `writeMetrics()` requirement gains the never-shrink guard; new requirement for own-machine snapshot max-merge in the display path; design decision "day-files are high-water marks".
- `cli/data-pipeline.md`: (modify) — merge pipeline now includes `maxMergeEntries` step for own-machine entries in `fetchToolMerged`/`fetchToolMergedWithMachines`.

## Impact

- **Code**: `src/node/sync/sync.ts` (writeMetrics guard), `src/node/core/fetcher.ts` (new `maxMergeEntries` pure helper), `src/node/core/cli.ts` (`fetchToolMerged`, `fetchToolMergedWithMachines`), `scripts/repair-metrics.mjs` (new, unbundled).
- **Tests** (node:test via tsx, co-located): `src/node/sync/__tests__/` (guard: fresh write / shrink-skip / grow-overwrite / corrupt-file / equal-value), `src/node/core/__tests__/` (maxMergeEntries; merged-fetch behavior with purged local). Repair script: unit-testable against a fixture git repo (follow `cli-sync.test.ts` bare-repo fixture pattern); keep hermetic per the open backlog note on env isolation.
- **Data flow**: all three tools (cc/codex/oc) — code paths are toolKey-generic.
- **Output stability**: table/JSON *format* unchanged; displayed *values* increase for purge-affected dates. Constitution's Output Stability clause → release as a **minor** version bump (0.4.17 → 0.5.0).
- **Deployment sequencing**: part 2's repair run happens manually after the release is installed on actively-syncing machines (brew upgrade).
- **Backlog**: at archive time, mark `r01f`, `87hw`, `gsji` done in the **main worktree's** `fab/backlog.md` (`idea done <id> --main`).
- **Specs**: `docs/specs/usage.md` describes sync/merge semantics — human-curated; flag for review during hydrate.

## Open Questions

None — the investigation resolved the mechanism end-to-end, and the user confirmed scope and ordering.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | All three backlog items (r01f, 87hw, gsji) bundled into this single change, plan tasks ordered 1→2→3 | Discussed — user approved bundling ("all 3 in one fab change?" → "just go ahead") | S:90 R:80 A:85 D:90 |
| 2 | Confident | Guard = skip whole-entry write when `incoming.totalCost < existing.totalCost`; rejected per-field max (chimera entry violates Constitution V) | One obvious interpretation of "never shrink" that keeps snapshots atomic | S:75 R:80 A:80 D:65 |
| 3 | Confident | Comparison key is `totalCost` alone (not totalTokens) | Cost is the user-facing quantity; live values are monotonic within a day; token/cost divergence has no realistic source here | S:70 R:75 A:75 D:60 |
| 4 | Certain | Guard lives inside `writeMetrics()`, covering both call sites (cli.ts:467 per-invocation write and sync.ts:188 fullSync) | Code analysis — single choke point; both callers verified during investigation | S:85 R:85 A:95 D:90 |
| 5 | Certain | Repair is a standalone `scripts/repair-metrics.mjs`, not bundled into `dist/tu.mjs`, run manually | Matches `scripts/help-dump.mjs` precedent; Constitution III (single bundle) untouched; one-time ops task doesn't belong in the CLI surface | S:85 R:90 A:90 D:85 |
| 6 | Confident | Repair defaults to dry-run report; `--write` restores working tree only; commit/push left to user; restores full historical-max file content | Safest reviewable flow for destructive-adjacent data ops; user didn't specify CLI shape | S:70 R:85 A:80 D:70 |
| 7 | Confident | Self-view fix = per-label whole-entry `max(live, own snapshot)` via new pure `maxMergeEntries`, then existing sum-merge with other machines; applied in both `fetchToolMerged` and `fetchToolMergedWithMachines` | Discussed — max-not-sum explicitly chosen to avoid double-counting partially-purged days | S:80 R:75 A:80 D:70 |
| 8 | Certain | Applies to all three tools (cc/codex/oc) | Code is toolKey-generic at every touched site | S:90 R:85 A:95 D:90 |
| 9 | Confident | Release as minor version bump (0.5.0) | Constitution Output Stability: values change materially for affected dates even though format is identical | S:65 R:90 A:75 D:70 |
| 10 | Certain | Tests: node:test runner, co-located `__tests__/` dirs; repair script tested against a seeded bare-repo git fixture | Constitution Test Runner + Test Location clauses are deterministic | S:95 R:90 A:95 D:95 |
| 11 | Certain | Non-goal: stderr warning when guard skips shrunk entries (discussed hardening) is excluded | Discussed — user selected only items 1–3 for the backlog; trivial to add later | S:90 R:80 A:90 D:85 |

11 assumptions (6 certain, 5 confident, 0 tentative, 0 unresolved).
