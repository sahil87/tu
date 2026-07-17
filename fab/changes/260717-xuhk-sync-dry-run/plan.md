# Plan: Add `--dry-run` to sync

**Change**: 260717-xuhk-sync-dry-run
**Intake**: `intake.md`

## Requirements

<!-- Derived from intake.md. RFC 2119 keywords. Every requirement has a stable R#
     ID and at least one GIVEN/WHEN/THEN scenario. -->

### Flag Surface: `--dry-run`

#### R1: `--dry-run` is a globally-parsed flag honored only by `tu sync`
`parseGlobalFlags` (src/node/core/cli.ts) MUST recognize `--dry-run`, expose it on `GlobalFlags` as `dryRunFlag: boolean`, and strip it from `filteredArgs` in the same boolean-flag pass as `--sync`/`--fresh`/`--full`. Any invocation OTHER than `tu sync` that carries `--dry-run` MUST fail fast on stderr with an actionable message naming the supported form, exit code 1.

- **GIVEN** the argv `["sync", "--dry-run"]`
- **WHEN** `parseGlobalFlags` runs
- **THEN** `dryRunFlag === true` AND `filteredArgs === ["sync"]`
- **AND** `main()` routes the flag into `runSync`

- **GIVEN** the argv `["cc", "--dry-run"]` or `["cc", "--sync", "--dry-run"]`
- **WHEN** `main()` dispatches
- **THEN** it prints `Error: --dry-run is supported only with 'tu sync' — run 'tu sync --dry-run' to preview a sync.` to stderr and exits 1
- **AND** no data fetch, write, or sync runs

#### R2: `--dry-run` alone (no `tu sync`) is misuse
A bare `tu --dry-run` (no subcommand, no data source) MUST be treated as misuse under the same fail-fast contract as R1 — `--dry-run` is honored ONLY by the `sync` subcommand.

- **GIVEN** the argv `["--dry-run"]`
- **WHEN** `main()` dispatches (after flag stripping `filteredArgs === []`)
- **THEN** it prints the R1 misuse error to stderr and exits 1

### Preview-Capable Write Path

#### R3: `writeMetrics` gains an optional dry-run mode and returns a per-file decision report
`writeMetrics(metricsDir, user, machine, toolKey, entries, dryRun?)` (src/node/sync/sync.ts) MUST accept an optional trailing `dryRun` boolean (default `false`) and MUST return a `WriteDecision[]` describing, per entry, whether the file WOULD be written or skipped (never-shrink guard), including the incoming cost and the existing cost when an existing parseable file was read. The decision logic (path construction + `isShrinkingWrite`) MUST run identically in live and dry-run mode; only `mkdirSync`/`writeFileSync` MUST be gated on `!dryRun`.

- **GIVEN** an existing day-file at cost 10.20 and an incoming entry at cost 12.34
- **WHEN** `writeMetrics(..., entries, true)` runs
- **THEN** it returns `[{ action: "write", incomingCost: 12.34, existingCost: 10.20, filePath }]`
- **AND** the filesystem is unchanged (no dir created, no file written)

- **GIVEN** an existing day-file at cost 45.67 and an incoming entry at cost 0.00 (shrinking)
- **WHEN** `writeMetrics(..., entries, true)` runs
- **THEN** the decision for that file is `{ action: "skip", incomingCost: 0.00, existingCost: 45.67, filePath }`

- **GIVEN** no existing day-file and an incoming entry at cost 3.21
- **WHEN** `writeMetrics(..., entries, true)` runs
- **THEN** the decision is `{ action: "write", incomingCost: 3.21, filePath }` with `existingCost` absent

#### R4: Live `writeMetrics` behavior is byte-identical to today
The report addition MUST NOT change any live write behavior. Every existing live caller (`fetchToolMerged`, `fetchToolMergedWithMachines` in cli.ts, and live `fullSync`) MUST keep its current call shape (the return value is ignored) and MUST produce identical filesystem effects to the pre-change code.

- **GIVEN** the existing `writeMetrics` test suite (src/node/sync/__tests__/sync.test.ts)
- **WHEN** run against the modified `writeMetrics`
- **THEN** all existing assertions pass unchanged (write, skip-shrink, empty/unparseable-as-absent, per-entry-within-batch)

### Preview-Capable Sync Orchestration

#### R5: `fullSync` gains a dry-run mode that reports without mutating
`fullSync(config, tuHome, dryRun?)` (src/node/sync/sync.ts) MUST accept an optional trailing `dryRun` boolean (default `false`). In dry-run mode it MUST: (a) fetch all tools via the unchanged `fetchHistory` path (read-only, cached); (b) call `writeMetrics` in dry-run mode per tool, collecting the decisions; (c) compute the git half of the preview LOCALLY with no network — would-be writes plus `git status --porcelain {user}/` (read-only) determine whether a commit would happen and its message (`# {user}: update {date}`, the same string as live), with `pull --rebase` / `push` REPORTED as would-follow operations, never executed or probed; (d) skip `syncMetrics` and `touchLastSync` entirely; (e) return a structured report. It MUST NOT touch the working tree, the metrics repo, or the network.

- **GIVEN** a multi-mode config with pending would-be writes
- **WHEN** `fullSync(config, tuHome, true)` runs
- **THEN** no day-file is written, `.last-sync` is NOT touched, and no `git commit`/`pull`/`push` runs
- **AND** it returns a report listing the per-tool write decisions and the would-be commit message

- **GIVEN** dry-run mode
- **WHEN** the git half is computed
- **THEN** only read-only git (`git status --porcelain {user}/`) is invoked; `pull`/`push` are reported strings, not invocations

#### R6: Live `fullSync` behavior is unchanged
When `dryRun` is `false` (default), `fullSync` MUST behave exactly as today — fetch, `writeMetrics` (live), `syncMetrics`, `touchLastSync` on success, return `boolean`.

- **GIVEN** the existing `runSync`/`fullSync` happy-path and failure tests
- **WHEN** run against the modified `fullSync`
- **THEN** all existing assertions pass unchanged

### Preview Output

#### R7: `runSync` prints the dry-run preview to stdout, exit 0
`runSync(configPath?, tuHome?, defaultsPath?, dryRun?)` (src/node/core/cli.ts) MUST accept an optional trailing `dryRun` boolean. In dry-run mode it MUST run the existing config/mode guards (multi-mode required, metrics-dir guard) UNCHANGED before the preview — a dry-run in single mode MUST fail exactly like a live `tu sync` — then format the `fullSync` report and print it to STDOUT, exiting 0. A final line MUST state that nothing was mutated. `syncMetrics` and the `Synced to ...` success message MUST NOT run in dry-run mode.

- **GIVEN** a multi-mode config
- **WHEN** `tu sync --dry-run` runs
- **THEN** the preview is printed to stdout (would-write / would-skip / would-commit lines), stderr is empty, and exit is 0
- **AND** the final stdout line states nothing was written, committed, or pushed

- **GIVEN** a single-mode config (no `metrics_repo`)
- **WHEN** `tu sync --dry-run` runs
- **THEN** it errors on stderr and exits 1, identically to a live `tu sync` (guards run before the dry-run branch)

### Help, Completions, Docs

#### R8: `FULL_HELP` documents `--dry-run`
`FULL_HELP` (src/node/core/cli.ts) MUST carry a `--dry-run` line under Flags describing it as sync-only. Because `tu help-dump` derives from `FULL_HELP` at runtime, no help-dump.ts change is needed.

- **GIVEN** `tu --help`
- **WHEN** the flags block is read
- **THEN** it contains a `--dry-run` line noting it applies to `tu sync` only

#### R9: Completions offer `--dry-run` in all three shells
`completions.ts` (src/node/core/completions.ts) MUST add `--dry-run` to bash `long_flags`, zsh `long_flags` array + a descriptive `_arguments` spec, and a fish `complete -c tu -l dry-run -d '...'` line.

- **GIVEN** each generated completion script
- **WHEN** inspected
- **THEN** bash `long_flags` includes `--dry-run`, zsh has both the `long_flags` entry and a `--dry-run[...]` spec, and fish has a `-l dry-run` completion

#### R10: README and usage.md document `--dry-run`
`README.md` flags list (mirrors `FULL_HELP` per the readme-extraction standard) and `docs/specs/usage.md` (Global Flags table + Sync Flow section) MUST get matching one-line `--dry-run` additions.

- **GIVEN** README.md `### Flags` and usage.md `## Global Flags`
- **WHEN** read
- **THEN** each documents `--dry-run` as a sync-only preview flag, and usage.md's Sync Flow section notes the dry-run variant

### Non-Goals

- No `--yes`/consent gate is added — sync is additive with a never-shrink guard (preview-for-safety, not destructive-consent).
- No support for `--dry-run` combined with any command other than `tu sync` (fail-fast; strict→loose is the non-breaking future direction).
- No change to any existing command's output format (additive flag only).

### Design Decisions

1. **`--dry-run` parsed globally, honored only by `tu sync`, misuse fails fast**: mirrors `--skip-brew-update` (globally detected, one command honors it) but with an explicit error rather than silent ignore. — *Why*: the multi-mode fetch path (`fetchToolMerged`) writes day-files OUTSIDE the sync boundary on every data command, so a combined `--sync --dry-run` preview-then-proceed would mutate the very files it previewed (a lying dry-run). Fail-fast is the honest contract. — *Rejected*: silently ignoring the flag (a user who passed `--dry-run` must never get a surprise mutation); honoring it on data commands (would require gating the fetch-path writes too, far beyond scope, and still wouldn't be a pure preview).

2. **`writeMetrics` returns `WriteDecision[]` in both modes**: the report is computed on every call; only the fs effects are gated on `dryRun`. — *Why*: this is the only design that shares the real decision path (`isShrinkingWrite` + path construction) per toolkit principle №5's accuracy requirement, while leaving live callers behaviorally unchanged (they ignore the return). — *Rejected*: a separate `previewMetrics` function (would duplicate the decision logic, risking drift — exactly what №5 forbids).

3. **Git half computed locally, read-only**: would-be writes from the decision reports + `git status --porcelain {user}/` determine the commit decision and message; `pull`/`push` are reported strings. — *Why*: backlog is explicit — "without touching the working tree, the metrics repo, or the network". `git status --porcelain` is read-only and already used by live `syncMetrics`, so it reuses the established git-invocation idiom (`execFile`, no shell).

4. **Preview to stdout, exit 0**: matches the reference implementation `shll uninstall --dry-run` (stdout, empty stderr, exit 0) and toolkit principle №2 (the preview IS the data the caller asked for). Guards run before the preview so single-mode/missing-repo failures are identical to live sync.

## Tasks

### Phase 1: Preview-capable write path (sync.ts)

- [x] T001 Add the `WriteDecision` interface and thread an optional `dryRun = false` param through `writeMetrics` in src/node/sync/sync.ts: build a decision per entry (path construction + `isShrinkingWrite`, run identically in both modes), gate `mkdirSync`/`writeFileSync` on `!dryRun`, and return `WriteDecision[]`. Refactor `isShrinkingWrite` to also surface the existing cost (e.g. a small helper returning `{ shrinking, existingCost? }`) so the decision can carry `existingCost` without a second file read. <!-- R3 -->
- [x] T002 <!-- rework: review must-fix — dry-run branch (sync.ts:300,307) re-implements the live commit-message construction from syncMetrics (sync.ts:126-127) instead of sharing it; extract a single `commitMessage(user)` helper used by BOTH the live commit and the preview (this is the №5 no-drift requirement, acceptance A-022). Also add a brief comment at the wouldCommit derivation noting the steady-state over-prediction (equal-cost byte-identical day-file → live skips the commit, preview says Would commit — plan-sanctioned heuristic, Design Decision 3). --> Add the dry-run mode to `fullSync(config, tuHome, dryRun = false)` in src/node/sync/sync.ts: in dry-run mode, fetch via `fetchHistory` (unchanged), call `writeMetrics(..., true)` per tool collecting decisions, compute the git half locally (would-be writes + read-only `git status --porcelain {user}/`) into a would-commit decision + message string, skip `syncMetrics`/`touchLastSync`, and return a structured `DrySyncReport`. Define the report type. Keep the live (`dryRun === false`) path byte-identical, returning `boolean` as today — use an overload or a discriminated return so the live callers' `boolean` contract is preserved. <!-- R5 -->

### Phase 2: Flag surface + preview output (cli.ts)

- [x] T003 Add `dryRunFlag: boolean` to the `GlobalFlags` interface and parse `--dry-run` in `parseGlobalFlags` (src/node/core/cli.ts): add it to the `rawArgs.includes("--dry-run")` detection and to the boolean-flag strip list in the filter loop (same line as `--full`/`--sync`), and include `dryRunFlag` in the returned object. <!-- R1 -->
- [x] T004 <!-- rework: review nice-to-have (low-effort) — cli.ts:444-445 recomputes `join(report.metricsDir, report.user)` twice per decision inside the loop (plus once at :439); hoist the prefix into one variable. --> Add the dry-run preview branch to `runSync` in src/node/core/cli.ts: add an optional `dryRun = false` param; run the existing config/mode + metrics-dir guards UNCHANGED, then in dry-run mode call `fullSync(guardedConfig, tuHome, true)`, format the report to stdout (would-write with `new`/`update: X → Y` annotations, would-skip with never-shrink reason, would-commit message + would-`pull --rebase`/`push`, and a final "nothing written, committed, or pushed" line), and return (exit 0, no `Synced to` message, no `syncMetrics`). <!-- R7 -->
- [x] T005 Wire the flag through `main()` in src/node/core/cli.ts: destructure `dryRunFlag` from `parseGlobalFlags`; in the `sync` command dispatch pass it into `runSync`; and add a fail-fast misuse guard — if `dryRunFlag` is set and the invocation is NOT `tu sync` (i.e. `filteredArgs[0] !== "sync"`, covering `tu cc --dry-run`, `tu cc --sync --dry-run`, and bare `tu --dry-run`), print `Error: --dry-run is supported only with 'tu sync' — run 'tu sync --dry-run' to preview a sync.` to stderr and exit 1, before any data fetch/dispatch. <!-- R1 --> <!-- R2 -->

### Phase 3: Help, completions, docs

- [x] T006 [P] Add a `--dry-run` line to `FULL_HELP` under Flags in src/node/core/cli.ts (e.g. `  --dry-run            Preview sync without writing (tu sync only)`), placed near `--sync`. <!-- R8 -->
- [x] T007 [P] Add `--dry-run` to all three completion scripts in src/node/core/completions.ts: bash `long_flags`, zsh `long_flags` array + a `'--dry-run[preview sync without writing]'` `_arguments` spec, and a fish `complete -c tu -l dry-run -d 'preview sync without writing'` line. <!-- R9 -->
- [x] T008 [P] <!-- rework: review should-fix — docs/site/workflows.md:73 (§ Multi-machine) and docs/site/install.md:108 (§4 Sync metrics) enumerate the sync flags/workflow but omit `tu sync --dry-run`; add the one-line mentions (these pages are pulled by shll.ai per the readme-extraction standard). README.md and usage.md halves of this task are already done — do not redo them. --> Add the `--dry-run` flag row to README.md `### Flags` (mirroring FULL_HELP) and to docs/specs/usage.md `## Global Flags` table, plus a note in usage.md's Sync Flow section describing the dry-run preview variant. <!-- R10 -->

### Phase 4: Tests

- [x] T009 Add dry-run `writeMetrics` tests to src/node/sync/__tests__/sync.test.ts: correct decisions for new / update / never-shrink-skip cases, and that the filesystem is untouched in dry-run mode (no dir created, no file written). Also assert live `writeMetrics` behavior is unchanged (report addition is non-behavioral — existing tests already cover the live path; add an explicit assertion that a live call still returns and writes). <!-- R3 --> <!-- R4 -->
- [x] T010 Add dry-run `fullSync` tests to src/node/sync/__tests__/sync.test.ts using the existing git-fixture harness: assert dry-run performs no git commit/push, no `.last-sync` touch, and no file writes, and returns a report with the expected decisions and would-commit message. <!-- R5 --> <!-- R6 -->
- [x] T011 Add `--dry-run` flag-parsing tests to src/node/core/__tests__/cli-parser.test.ts: `dryRunFlag` set true and stripped from `filteredArgs` when present, false when absent, stripped from the middle of positionals. <!-- R1 -->
- [x] T012 Add misuse-guard tests to src/node/core/__tests__/ (co-located, e.g. a new cli-dry-run-flag.test.ts covering the `main()` misuse contract, or extend cli-sync.test.ts for the runSync dry-run path): `tu cc --dry-run` and `tu cc --sync --dry-run` error on stderr with exit 1; the runSync dry-run happy path prints to stdout / exit 0 (multi-mode fixture) and the single-mode guard still errors + exits 1. <!-- R1 --> <!-- R2 --> <!-- R7 -->

## Execution Order

- T001 blocks T002 (fullSync's dry-run collects `writeMetrics` decisions).
- T002 blocks T004 (runSync formats fullSync's report).
- T003 blocks T005 (main() destructures `dryRunFlag`).
- T004 blocks T005 (main() passes dryRun into runSync).
- Phase 3 tasks (T006–T008) are independent `[P]` — different files/regions.
- Phase 4 tests follow their implementation tasks (test-alongside): T009 after T001, T010 after T002, T011 after T003, T012 after T004+T005.

## Acceptance

### Functional Completeness

- [x] A-001 R1: `parseGlobalFlags(["sync", "--dry-run"])` sets `dryRunFlag === true` and `filteredArgs === ["sync"]`; `tu sync --dry-run` routes the flag into `runSync`.
- [x] A-002 R2: bare `tu --dry-run` (no subcommand) hits the misuse guard and exits 1.
- [x] A-003 R3: dry-run `writeMetrics` returns a `WriteDecision[]` with correct `action`/`incomingCost`/`existingCost` for new, update, and shrink cases.
- [x] A-004 R4: all pre-existing `writeMetrics` assertions pass unchanged; live callers ignore the return and produce identical fs effects.
- [x] A-005 R5: dry-run `fullSync` returns a structured report and performs no write, no `.last-sync` touch, and no `git commit`/`pull`/`push`.
- [x] A-006 R6: live `fullSync` (default `dryRun`) behaves exactly as today (existing runSync/fullSync tests pass).
- [x] A-007 R7: `tu sync --dry-run` prints the preview to stdout with empty stderr and exit 0; the final line states nothing was mutated.
- [x] A-008 R8: `tu --help` / `FULL_HELP` contains a sync-only `--dry-run` line.
- [x] A-009 R9: bash, zsh, and fish completion scripts all offer `--dry-run`.
- [x] A-010 R10: README.md and docs/specs/usage.md document `--dry-run`.

### Behavioral Correctness

- [x] A-011 R1: `tu cc --dry-run` and `tu cc --sync --dry-run` both print `Error: --dry-run is supported only with 'tu sync' — run 'tu sync --dry-run' to preview a sync.` to stderr, exit 1, and perform no fetch/write/sync.
- [x] A-012 R7: `tu sync --dry-run` in single mode (no `metrics_repo`) errors on stderr and exits 1, identical to a live `tu sync` (guards run before the preview).
- [x] A-013 R5: the git half of the dry-run preview invokes only read-only git (`git status --porcelain {user}/`); `pull`/`push` are reported strings, never invoked or probed.

### Scenario Coverage

- [x] A-014 R3: a unit test proves dry-run `writeMetrics` leaves the filesystem untouched (no dir, no file).
- [x] A-015 R5: a git-fixture test proves dry-run `fullSync` adds no commit to the bare repo and no `.last-sync`.
- [x] A-016 R1: a flag-parsing test covers `--dry-run` stripped from the middle of positionals.

### Edge Cases & Error Handling

- [x] A-017 R7: the dry-run preview correctly reports the never-shrink "would skip" case with the incoming < existing reason.
- [x] A-018 R3: dry-run `writeMetrics` treats absent/empty/unparseable existing files as absent (would-write, no `existingCost`), matching the live guard.

### Code Quality

- [x] A-019 Pattern consistency: new code follows the functional style (no classes), `type` imports, `node:` built-in imports, and the existing `execFile`-based git idiom.
- [x] A-020 No unnecessary duplication: the dry-run path reuses `isShrinkingWrite`, path construction, `fetchHistory`, and the `git status --porcelain` idiom rather than reimplementing them (minimum pathways — one decision path serves live and preview).
- [x] A-021 Graceful degradation: the dry-run path does not crash on a missing metrics dir / single mode — it runs the same guards as live sync (Constitution II).
- [x] A-022 No magic strings: the misuse error and would-commit message reuse named/derived strings (the commit message uses the same `# {user}: update {date}` construction as live). *(Met: extracted a single `commitMessage(user)` helper in src/node/sync/sync.ts, called from BOTH the live commit (`syncMetrics`) and the dry-run preview (`fullSync`) — the preview is drift-proof by construction. Rework cycle 1.)*
- [x] A-023 Consistent data model: `WriteDecision`/report types conform to plain-object/interface conventions and the `UsageEntry` cost model (Constitution V).

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- Test runner: `npm test` (the canonical command; `just test` delegates to it). NOTE: the local dev environment exports `TU_METRICS_REPO`, which makes config/sync/init-metrics tests derive `multi` mode and fail spuriously — run with `env -u TU_METRICS_REPO npm test` for a clean baseline (773 pass, 0 fail confirmed pre-change).
- Additive flag ⇒ minor version bump per the constitution's Output Stability rule (handled at release, not in this change).

## Deletion Candidates

None — this change adds new functionality (a dry-run preview mode) without making existing code redundant. Re-verified at review cycle 2: `isShrinkingWrite` was refactored into `readShrinkState` in place (grep confirms no orphaned symbol in `src/`; the stale name survives only in `docs/memory/sync/multi-machine.md`, which hydrate updates), and the inline commit-message construction formerly duplicated in `syncMetrics` was replaced by the shared `commitMessage()` helper (both call sites live). The one prior candidate in this area — the test-only `excludeMachine` parameter on `readRemoteEntries`/`readRemoteEntriesByMachine` (flagged by 260610-srmi) — predates this change and is untouched by it.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | Preview prints to stdout, exit 0 (`tu sync --dry-run`) | Intake Assumption #1, resolved by observing `shll uninstall --dry-run` (stdout, empty stderr) + principle №2: the preview is the data the caller asked for | S:60 R:70 A:90 D:80 |
| 2 | Confident | `--dry-run` honored only by `tu sync`; all other invocations fail fast with the exact error string, exit 1 | Intake Assumptions #2 + #6; the multi-mode fetch path writes day-files outside the sync boundary, so a combined preview-then-proceed would mutate what it previewed; fail-fast is honest, strict→loose is non-breaking; exit 1 matches tu's uniform flag-error convention | S:60 R:70 A:70 D:60 |
| 3 | Certain | No `--yes`/consent gate added | Intake Assumption #3, backlog explicit: never-shrink guard makes sync additive — preview-for-safety, not destructive-consent | S:90 R:80 A:90 D:90 |
| 4 | Confident | `writeMetrics` gains optional `dryRun` param + returns `WriteDecision[]`; live callers ignore the return | Intake Assumption #4; the only design sharing the real decision path (`isShrinkingWrite` + path construction) per №5 while leaving live call sites behaviorally unchanged | S:70 R:75 A:80 D:70 |
| 5 | Certain | Git half of preview computed locally (would-be writes + read-only `git status --porcelain`); pull/push reported, never probed — no network | Intake Assumption #5, backlog explicit: "without touching the working tree, the metrics repo, or the network" | S:85 R:80 A:85 D:80 |
| 6 | Confident | `fullSync` dry-run returns a structured report (not `boolean`); live path keeps its `boolean` contract via an overload / discriminated return | Follows from R5/R7 — the preview needs the decision data, and live callers (`runSync` live path, `main()` `--sync`) still consume `boolean`; the two return shapes are disjoint by the `dryRun` argument | S:65 R:70 A:75 D:65 |
| 7 | Confident | Preview format is plan-level (would-write with new/update annotations, would-skip with reason, would-commit + would-pull/push, final no-mutation line) modeled on the intake's illustrative output | Intake §4 gives an illustrative-but-not-binding format; exact wording is an apply-time decision with one obvious front-runner (the intake sketch); low blast radius (stdout text, no downstream parser — output-stability applies to existing commands, and this is a new flag) | S:55 R:80 A:75 D:65 |

7 assumptions (2 certain, 5 confident, 0 tentative).
