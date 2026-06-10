# Plan: Fix Metrics History Destruction & Self-Exclusion Blind Spot

**Change**: 260610-srmi-fix-metrics-history-destruction
**Status**: In Progress
**Intake**: `intake.md`

## Requirements

### Sync: Never-Shrink Guard in `writeMetrics` (Part 1)

#### R1: writeMetrics never shrinks a day-file
`writeMetrics()` (`src/node/sync/sync.ts`) MUST skip writing a per-day JSONL file when the existing file parses as a `UsageEntry` and the incoming entry's `totalCost` is strictly less than the existing entry's `totalCost`. The skip MUST be silent (no stderr output). In all other cases (file absent, file empty, file unparseable as a `UsageEntry`, incoming `totalCost >= existing totalCost`) the write MUST proceed exactly as before. The guard MUST live inside `writeMetrics` itself so both call sites (`fetchToolMerged` in `src/node/core/cli.ts` and `fullSync` in `src/node/sync/sync.ts`) are covered. The comparison key is `totalCost` alone.

- **GIVEN** a day-file `cc-2026-04-24.jsonl` containing an entry with `totalCost: 308.12`
- **WHEN** `writeMetrics` is called with an incoming entry for `2026-04-24` with `totalCost: 9.46`
- **THEN** the file is left untouched (still contains the `308.12` entry) and no error or warning is emitted

- **GIVEN** a day-file containing an entry with `totalCost: 1.0`
- **WHEN** `writeMetrics` is called with an incoming entry for the same date with `totalCost: 2.0` (or exactly `1.0`)
- **THEN** the file is overwritten with the incoming entry (grow and equal-value refresh both write)

- **GIVEN** no day-file exists for the entry's date, OR the existing file is empty, OR its content does not parse as a `UsageEntry` with a numeric `totalCost`
- **WHEN** `writeMetrics` is called
- **THEN** the incoming entry is written (treat as absent — matches the read path's skip-silently posture)

### Repair: One-Time History Restoration Script (Part 2)

#### R2: Standalone repair script with single-walk history discovery
A new standalone script `scripts/repair-metrics.mjs` MUST walk the metrics repo's git history **once** (a single `git log --format=... --name-only -- '*.jsonl'` invocation, not one `git log` per file) to build a per-file commit list, then for each tracked day-file find the historical version with **maximum `totalCost`** via `git show <sha>:<path>`. The script MUST be runnable as `node scripts/repair-metrics.mjs [--repo <path>] [--write]` with `--repo` defaulting to `~/.tu/metrics_repo`. It MUST cover all users and machines in the repo. It MUST NOT be bundled into `dist/tu.mjs` and MUST NOT be imported by anything under `src/` (Constitution III untouched; precedent: `scripts/help-dump.mjs`). Historical versions that are unparseable, and commits where a path was deleted, MUST be skipped without crashing. Operational errors (missing repo, not a git repo) MUST print a message to stderr and exit 1.

- **GIVEN** a metrics repo where `sahil/2026/devws/cc-2026-04-24.jsonl` was committed with `totalCost: 308.12` and later overwritten by a commit with `totalCost: 9.46`
- **WHEN** the script scans the repo
- **THEN** it identifies `308.12` as the historical maximum for that file using one history walk plus per-version `git show` reads

- **GIVEN** `--repo` points at a directory that is not a git repository
- **WHEN** the script runs
- **THEN** it prints an error to stderr and exits with code 1

#### R3: Dry-run report is the default
By default (no `--write`), the script MUST print a per-file report of every "shrunk" file — path, current (HEAD/working-tree) `totalCost`, historical max `totalCost`, the commit (short SHA) and date of the max, and the delta — plus per-user subtotals and a grand total, and MUST NOT modify any file. A file is "shrunk" when its current value is lower than its historical max by **more than one cent** (`delta > 0.01`). Files at their historical max (or within a cent) MUST NOT be listed. When nothing is shrunk, the script MUST say so explicitly.

- **GIVEN** a repo with one shrunk file (max `308.12`, current `9.46`) and one never-shrunk file
- **WHEN** the script runs without `--write`
- **THEN** the report lists only the shrunk file with current/max/delta and the max commit reference, prints per-user and grand totals, and the working tree is byte-identical to before the run

- **GIVEN** a file whose current value is lower than its max by `0.005`
- **WHEN** the script runs
- **THEN** the file is not reported as shrunk (within-a-cent tolerance)

#### R4: `--write` restores full content in the working tree only
With `--write`, the script MUST restore each shrunk file's **full original content** (the exact bytes of the historical-max version, not just the cost field) into the working tree. It MUST NOT commit or push — review, commit, and push are deliberately left to the user. The operation MUST be idempotent: a second run after a successful `--write` reports nothing left to repair.

- **GIVEN** a shrunk day-file whose historical-max version had specific token fields and cost
- **WHEN** the script runs with `--write`
- **THEN** the working-tree file is byte-identical to the historical-max blob, no git commit is created, and a re-run reports nothing to repair

### CLI: Self-View Max-Merge (Part 3)

#### R5: `maxMergeEntries` pure helper
`src/node/core/fetcher.ts` MUST export a new pure function `maxMergeEntries(a: UsageEntry[], b: UsageEntry[]): UsageEntry[]` that, per date label, picks **whichever whole entry has the greater `totalCost`** — no field mixing, no summing. On equal `totalCost`, the entry from `a` (the live local fetch at the call sites) wins. Output MUST be sorted ascending by label (like `mergeEntries`) and the function MUST NOT mutate its inputs (Constitution V: pure functions over `UsageEntry` types).

- **GIVEN** `a = [{label: "2026-04-24", totalCost: 9.46, ...}]` and `b = [{label: "2026-04-24", totalCost: 236.00, ...}]`
- **WHEN** `maxMergeEntries(a, b)` is called
- **THEN** the result contains exactly `b`'s whole entry for `2026-04-24` (every field from `b`, none from `a`) — not the sum

- **GIVEN** entries with non-overlapping labels in `a` and `b`
- **WHEN** `maxMergeEntries(a, b)` is called
- **THEN** all entries appear once, sorted ascending by label, and neither input array is mutated

#### R6: `fetchToolMerged` merges own-machine snapshots into the local view
In `fetchToolMerged` (`src/node/core/cli.ts`), the default (own-user) path MUST read the machine's own repo snapshots back and compute `effectiveLocal = maxMergeEntries(local, ownSnapshots)` before the existing sum-merge with other machines (`mergeEntries(effectiveLocal, remote)`). For dates within the live window, live data wins (equal or greater); for purged dates, the repo snapshot resurfaces. The `-u <other-user>` path (repo-only, `excludeMachine = null`) MUST remain unchanged. The change applies to all three tools (the code is toolKey-generic).

- **GIVEN** a machine whose live fetch returns `totalCost: 9.46` for `2026-04-24` (post-purge residue), whose own repo snapshot for that date holds `236.00`, and another machine's snapshot holds `308.12`
- **WHEN** the merged entries are computed
- **THEN** the `2026-04-24` total is `236.00 + 308.12 = 544.12` (own max, then sum across machines) — not `9.46 + 308.12` and not `9.46 + 236.00 + 308.12`

- **GIVEN** a date within the live window where live `totalCost` equals or exceeds the own snapshot (e.g. today, still growing)
- **WHEN** the merged entries are computed
- **THEN** the live entry is used for the own-machine share (idempotent with today's behavior; merged totals only ever increase relative to the pre-change pipeline)

#### R7: `fetchToolMergedWithMachines` applies the same self-view correction
`fetchToolMergedWithMachines` (`src/node/core/cli.ts`) MUST apply the same own-snapshot max-merge in multi mode: the own-machine entry in the returned `machineMap` MUST be `maxMergeEntries(local, ownSnapshots)`, so `--by-machine` shows the corrected own-machine column, and the flattened `entries` sum reflects it. Single mode (no repo) MUST remain unchanged (machineMap contains only the live local entries).

- **GIVEN** multi mode, an own-machine snapshot of `236.00` for a purged date, live residue `9.46`, and a remote machine snapshot of `308.12`
- **WHEN** `--by-machine` data is computed
- **THEN** the own-machine column shows `236.00` for that date, the remote machine column shows `308.12`, and the merged total is `544.12`

- **GIVEN** single mode (`config.mode !== "multi"`)
- **WHEN** `fetchToolMergedWithMachines` runs
- **THEN** behavior is byte-identical to before this change (one machine, live entries only, no repo reads)

### Non-Goals

- No stderr warning when the guard skips shrunk entries — discussed hardening, explicitly excluded by the user's backlog selection (trivial follow-up).
- No per-field `max` across entries — it fabricates a chimera entry violating Constitution V; whole-entry semantics only.
- No automatic execution of the repair script — it ships in `scripts/` and is run manually after the release is installed on actively-syncing machines. This apply stage MUST NOT run it against the real `~/.tu/metrics_repo`.
- No version bump in `package.json` — release (0.5.0 minor, per Output Stability) is handled outside this change.

### Design Decisions

1. **Guard inside `writeMetrics`**: single choke point covering both callers (`fetchToolMerged` per-invocation write and `fullSync`) — *Why*: any caller-side guard would have to be duplicated and can drift — *Rejected*: guarding at each call site.
2. **Skip-write, not per-field max**: when the incoming entry is smaller, keep the existing file verbatim — *Why*: every day-file remains an atomic snapshot that was real at some point in time (Constitution V) — *Rejected*: per-field `max` (chimera entries mixing token/cost fields from different snapshots).
3. **Self-view merge = per-day whole-entry max, then existing sum-merge**: `maxMergeEntries(local, ownSnapshots)` feeds the unchanged `mergeEntries` — *Why*: a partially-purged day still yields a residual live entry; summing residual + snapshot would double-count surviving transcripts — *Rejected*: summing own snapshots into the remote set.
4. **Repair restores full file content from the max commit**: `git show <sha>:<path>` bytes written verbatim — *Why*: lossless; keeps token fields and cost from one real snapshot — *Rejected*: patching only `totalCost` into the current file.
5. **Single directory walk for own + remote snapshots**: the rewired fetch paths call `readRemoteEntriesByMachine(metricsDir, user, /* excludeMachine */ null, toolKey)` once and split the own machine out of the returned map — *Why*: reuses the existing utility unchanged and keeps one well-exercised path (code-quality: minimum pathways) instead of adding a second single-machine read helper — *Rejected*: new `readMachineEntries()` helper plus a second exclude-walk.

## Tasks

### Phase 1: Never-Shrink Guard (intake part 1)

- [x] T001 Add the never-shrink guard inside `writeMetrics` in `src/node/sync/sync.ts`: before each `writeFileSync`, read the existing day-file; if it parses as a `UsageEntry` with numeric `totalCost` and `incoming.totalCost < existing.totalCost`, skip silently; absent/empty/unparseable files and grow/equal cases write as before <!-- R1 -->
- [x] T002 Add guard tests to `src/node/sync/__tests__/sync.test.ts` (`writeMetrics` describe block): fresh write, shrink-skip, grow-overwrite, equal-value refresh, empty file, corrupt/non-entry JSON file; run `npx tsx --test src/node/sync/__tests__/sync.test.ts` <!-- R1 -->

### Phase 2: Repair Script (intake part 2)

- [x] T003 Create `scripts/repair-metrics.mjs` (standalone, plain `node`, `node:`-prefixed builtins only, no imports from `src/`): parse `--repo` (default `~/.tu/metrics_repo`) and `--write`; enumerate tracked day-files via `git ls-files`; build per-file commit lists from one `git log --format --name-only -- '*.jsonl'` walk; find each file's historical-max `totalCost` via `git show`; dry-run report (path, current, max, max-commit short SHA + date, delta, per-user + grand totals, cent tolerance as a named constant); `--write` restores full max-version content into the working tree only; idempotent; stderr + exit 1 on operational errors <!-- R2 R3 R4 -->
- [x] T004 Add `src/node/sync/__tests__/repair-metrics.test.ts` driving the script via `spawnSync(process.execPath, [script, "--repo", fixture])` against a seeded local git fixture repo (per-repo `user.email`/`user.name`, `main` branch — follow the `cli-sync.test.ts`/`sync.test.ts` fixture pattern; hermetic: no HOME/TU_* dependence, never touches the real metrics repo): dry-run reporting + no modification, within-a-cent tolerance, never-shrunk files omitted, multi-user totals, max across 3+ versions, unparseable historical version skipped, `--write` byte-exact restore + no commit + idempotent re-run, non-repo error path; run `npx tsx --test src/node/sync/__tests__/repair-metrics.test.ts` <!-- R2 R3 R4 -->

### Phase 3: Self-View Max-Merge (intake part 3)

- [x] T005 Add pure `maxMergeEntries(a, b)` to `src/node/core/fetcher.ts` next to `mergeEntries`: per-label whole-entry max on `totalCost`, ties keep `a`'s entry, output sorted by label, no input mutation <!-- R5 -->
- [x] T006 Add `maxMergeEntries` tests to `src/node/core/__tests__/fetcher.test.ts`: picks larger whole entry (no field mixing/summing), tie keeps first argument's entry, non-overlapping union, empty inputs, sorted output, no mutation; run `npx tsx --test src/node/core/__tests__/fetcher.test.ts` <!-- R5 -->
- [x] T007 Rewire `fetchToolMerged` in `src/node/core/cli.ts` (default own-user path): replace the `readRemoteEntries(..., config.machine, ...)` call with one `readRemoteEntriesByMachine(..., null, ...)` read; split own-machine snapshots from other machines; `effectiveLocal = maxMergeEntries(local, ownSnapshots)`; `mergeEntries(effectiveLocal, remote)`; `-u` other-user branch untouched; import `maxMergeEntries` from `./fetcher.js` <!-- R6 -->
- [x] T008 Rewire `fetchToolMergedWithMachines` in `src/node/core/cli.ts` (multi-mode branch): read all machines with `excludeMachine = null`, set `machineMap.set(config.machine, maxMergeEntries(local, ownSnapshots))`, copy other machines as-is; single-mode branch untouched <!-- R7 -->
- [x] T009 Add `src/node/core/__tests__/self-view-merge.test.ts`: composition test simulating the rewired pipeline against a temp metrics dir (seeded own-machine + remote-machine day-files, purged live view) — guarded `writeMetrics` → `readRemoteEntriesByMachine(null)` → own/others split → `maxMergeEntries` → `mergeEntries`; asserts purged-date resurfacing (own 236.00 + remote 308.12 = 544.12, not residue-summed), live-window dominance for today, and by-machine own-column correction; hermetic temp dirs only; run `npx tsx --test src/node/core/__tests__/self-view-merge.test.ts` <!-- R6 R7 -->

### Phase 4: Validation

- [x] T010 Run the full suite (`env -u TU_METRICS_REPO npm test`) and the bundle build (`npm run build`); confirm all tests green, the bundle compiles, and `dist/tu.mjs` does not contain repair-script code <!-- R1 R2 R3 R4 R5 R6 R7 -->

## Execution Order

- Phases run strictly 1 → 2 → 3 → 4 (user-mandated ordering of the three intake parts).
- T005 blocks T007/T008 (cli.ts imports the new helper); T001 blocks T009 (composition test exercises the guarded `writeMetrics`).
- Running the repair script against the real `~/.tu/metrics_repo` is NOT part of this plan — it happens manually after release, once actively-syncing machines have upgraded (sequencing constraint from the intake).

## Acceptance

### Functional Completeness

- [x] A-001 R1: `writeMetrics` skips silently when the incoming entry's `totalCost` is lower than the existing parsed day-file's; writes in all other cases; guard lives inside `writeMetrics` covering both call sites
- [x] A-002 R2: `scripts/repair-metrics.mjs` exists, runs standalone under plain `node` with `--repo`/`--write` flags, walks history with a single `git log` pass (no per-file `git log`), covers all users/machines, and is neither bundled into `dist/tu.mjs` nor imported by `src/`
- [x] A-003 R3: default invocation prints the shrunk-file report (path, current, max, max commit + date, delta, per-user and grand totals) and modifies nothing
- [x] A-004 R4: `--write` restores byte-exact historical-max content into the working tree, creates no commit, and a re-run reports nothing to repair
- [x] A-005 R5: `maxMergeEntries` is exported from `fetcher.ts`, picks whole entries by greater `totalCost` per label, sorts output, never mutates inputs
- [x] A-006 R6: `fetchToolMerged` computes `effectiveLocal = maxMergeEntries(local, ownSnapshots)` before `mergeEntries` with other machines; `-u` other-user path unchanged
- [x] A-007 R7: `fetchToolMergedWithMachines` sets the own-machine `machineMap` entry to the max-merged view in multi mode; single mode unchanged

### Behavioral Correctness

- [x] A-008 R1: equal-value and growing writes still overwrite (today's file keeps refreshing as the day grows) — the existing "overwrites existing file on re-run" test still passes
- [x] A-009 R6: for a purged date, merged output equals own-snapshot max plus other machines' sum (no double count of residual live data); merged totals never decrease relative to the pre-change pipeline

### Scenario Coverage

- [x] A-010 R1: tests cover fresh write / shrink-skip / grow-overwrite / equal-value / empty file / corrupt file
- [x] A-011 R2: repair tests run against a seeded fixture git repo (never the real metrics repo) and cover dry-run, tolerance, multi-user totals, max-across-3+-versions, unparseable version skip, `--write`, idempotence, and the non-repo error path
- [x] A-012 R5: `maxMergeEntries` tests cover overlap (whole-entry pick), tie-keeps-`a`, non-overlap union, empties, sort order, input immutability
- [x] A-013 R6 R7: composition test exercises the full rewired pipeline (guarded write → read-back → split → max-merge → sum-merge) including the by-machine own-column

### Edge Cases & Error Handling

- [x] A-014 R1: empty and unparseable existing day-files are treated as absent (write proceeds, no crash, no warning)
- [x] A-015 R2: deleted-at-some-commit paths and unparseable historical blobs are skipped without crashing; operational errors (missing/non-git `--repo`) warn on stderr and exit 1 (no unhandled throw)
- [x] A-016 R6: machines with no own snapshots (fresh machine, empty repo dir) behave as today (`ownSnapshots = []` → `effectiveLocal = local`)

### Code Quality

- [x] A-017 Pattern consistency: new code follows surrounding naming/structure (`node:` imports, `.js` extensions, `type` imports, functional style, no classes)
- [x] A-018 No unnecessary duplication: reuses `readRemoteEntriesByMachine`, `mergeEntries` shape, and existing test fixture patterns instead of new parallel utilities
- [x] A-019 Constitution V: `maxMergeEntries` is a pure function over `UsageEntry[]`; no chimera entries anywhere
- [x] A-020 No magic numbers: the one-cent repair tolerance is a named constant in the script
- [x] A-021 No swallowed errors beyond spec: repair script fails loud (stderr + exit 1) on operational errors; guard's silent skip is the specified behavior, mirroring the read path's posture

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`
- Test-environment caveat: an exported `TU_METRICS_REPO` or the developer's real `~/.tu.conf` leaks into pre-existing config-dependent suites — run validation with `env -u TU_METRICS_REPO npm test`. New tests in this change are hermetic (temp dirs, explicit `--repo`, no ambient TU_* reads).

## Deletion Candidates

- `excludeMachine` parameter of `readRemoteEntriesByMachine`/`readRemoteEntries` (`src/node/sync/sync.ts:121,152,184`) — after this change every production call site passes `null` (cli.ts:459, 474, 502, 527); the non-null filtering branch is exercised only by tests (including this change's pre/post comparison test). The parameter and its skip branch can be removed in a follow-up signature simplification.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | Own + remote snapshots read in one `readRemoteEntriesByMachine(..., null, ...)` walk, own machine split from the map — instead of adding a single-machine read helper | Intake specifies *what* to read (own snapshots), not *how*; reuse of the existing utility honors minimum-pathways and avoids a second directory-walk path | S:70 R:85 A:85 D:70 |
| 2 | Confident | Repair script walks the history of HEAD (the checked-out branch — `main` on the real repo) rather than hardcoding the ref `main` | Intake says "full history of `main`" describing the real repo; HEAD is identical there, matches the working-tree/HEAD comparison baseline, and stays robust to fixture/clone branch names | S:65 R:85 A:80 D:65 |
| 3 | Confident | Repair-script tests live at `src/node/sync/__tests__/repair-metrics.test.ts` | `npm test` only discovers `src/node/**/__tests__/*.test.ts`, so a `scripts/__tests__/` location would never run; the metrics repo is sync-domain | S:70 R:90 A:85 D:75 |
| 4 | Confident | A day-file that parses as JSON but lacks a finite numeric `totalCost` counts as "unparseable as a UsageEntry" → write proceeds | One obvious reading of the intake's absent/empty/unparseable posture; keeps the guard from being wedged open by junk files | S:70 R:85 A:85 D:75 |
| 5 | Confident | Repair script exits 0 after both dry-run and `--write` regardless of findings; non-zero only for operational errors | Report-style ops tool; intake specifies no exit-code contract beyond fail-loud errors | S:60 R:90 A:80 D:70 |
| 6 | Certain | `fetchToolMergedWithMachines` applies the max-merge only inside its existing `config.mode === "multi"` branch | The repo read/write block is already gated on multi mode; single mode has no repo dir (intake: unaffected paths) | S:85 R:90 A:95 D:90 |
| 7 | Certain | On equal `totalCost`, `maxMergeEntries` keeps the first argument's (live local) entry | Intake: "For dates within the live window, live wins (equal or greater — it includes in-flight today data)" | S:90 R:90 A:95 D:90 |

7 assumptions (2 certain, 5 confident, 0 tentative).
