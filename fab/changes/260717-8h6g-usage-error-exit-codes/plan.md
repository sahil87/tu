# Plan: Usage-Error Exit Codes

**Change**: 260717-8h6g-usage-error-exit-codes
**Intake**: `intake.md`

## Requirements

<!-- Derived from intake.md. RFC-2119 statements with stable R# IDs and GIVEN/WHEN/THEN scenarios. -->

### CLI: Exit-Code Convention

#### R1: Usage errors MUST exit 2
All 16 enumerated usage-error sites in `src/node/core/cli.ts` MUST terminate the
process with exit code `2` (toolkit convention: `0` success / `1` operational
failure / `2` usage error, principle №4). Error message text and stderr routing
MUST remain byte-identical — only the exit code changes.

- **GIVEN** a user invokes `tu` with a malformed or incompatible argument (unknown
  argument, unknown tool, unknown shell, incompatible format flags, bad `--interval`
  value, missing `-u` value, bad/inverted `--since`/`--until`)
- **WHEN** the corresponding validation branch fires
- **THEN** the same error message is written to stderr as today
- **AND** the process exits with code `2` (not `1`)

The 16 sites (verified against the live file):

| # | Function | Message | Verified line |
|---|----------|---------|---------------|
| 1 | `runShellInit` | `Unknown shell: {shell}. Supported: bash, zsh, fish` | 314-315 |
| 2 | `parseGlobalFlags` | `Error: --interval requires a numeric value` | 707-708 |
| 3 | `parseGlobalFlags` | `Error: --interval minimum is 5 seconds` | 712-713 |
| 4 | `parseGlobalFlags` | `Error: --interval maximum is 3600 seconds` | 716-717 |
| 5 | `parseGlobalFlags` | `Error: --watch and --json are incompatible` | 725-726 |
| 6 | `parseGlobalFlags` | `Error: --json and --csv are incompatible` | 729-730 |
| 7 | `parseGlobalFlags` | `Error: --json and --md are incompatible` | 733-734 |
| 8 | `parseGlobalFlags` | `Error: --csv and --md are incompatible` | 737-738 |
| 9 | `parseGlobalFlags` | `Error: --watch and --csv are incompatible` | 741-742 |
| 10 | `parseGlobalFlags` | `Error: --watch and --md are incompatible` | 745-746 |
| 11 | `parseGlobalFlags` | `Error: -u requires a username` | 750-751 |
| 12 | `parseGlobalFlags` | `Error: --since requires a date (YYYY-MM-DD or YYYYMMDD)` | 757-758 |
| 13 | `parseGlobalFlags` | `Error: --until requires a date (YYYY-MM-DD or YYYYMMDD)` | 764-765 |
| 14 | `parseGlobalFlags` | `Error: --since must be on or before --until` | 769-770 |
| 15 | `dispatchSingleTool` | `Unknown tool: {toolKey}` (+ `SHORT_USAGE`) | 962-966 |
| 16 | `main` (parseDataArgs catch) | `Unknown argument: ...` (+ `SHORT_USAGE`) | 1332-1336 |

#### R2: Operational failures MUST stay exit 1
The 9 operational-failure sites MUST remain at exit code `1`, and
`src/node/tui/watch.ts`'s `process.exit(0)` quit path MUST remain untouched. An
operational failure is a runtime/environment failure (network, config, external
tool), distinct from a caller-invocation error.

- **GIVEN** an operation fails at runtime (metrics_repo unset, clone/sync failure,
  brew update/info/upgrade failure, an unexpected thrown error in `main().catch`)
- **WHEN** the failure branch fires
- **THEN** the process exits with code `1` (unchanged)

Operational sites (verified, unchanged): `runInitMetrics` (metrics_repo unset ~189,
dir-not-a-repo ~204), `runUpdate` (brew update ~333, brew info ~348, brew upgrade
~362), `runSync` (metrics_repo unset ~437, clone/dir-missing ~442, sync failed ~448),
`main().catch` (catch-all ~1433).

#### R3: A named mechanism MUST replace literal `2`s
A single named constant `EXIT_USAGE = 2` MUST be introduced in `src/node/core/cli.ts`
and used at every usage-error `process.exit` site, so no bare literal `2` appears
(code-quality: no magic numbers).

- **GIVEN** the 16 usage-error sites
- **WHEN** each is flipped to exit 2
- **THEN** each calls `process.exit(EXIT_USAGE)` referencing the single named constant
- **AND** no literal `process.exit(2)` remains in the file

#### R4: Pinned test assertions MUST conform to the exit-2 spec
The ~21 test assertions that pin usage-error exits to `1` MUST be updated to `2`
(Test Integrity: tests verify conformance to the spec). Operational-path tests
(`cli-sync.test.ts`, `cli-init-metrics.test.ts`) MUST remain untouched.

- **GIVEN** `cli-parser.test.ts` (14 usage-error `assert.equal(s.code, 1)`),
  `cli-watch-flag.test.ts` (6), `completions.test.ts` (1 `cap.exitCode`)
- **WHEN** the assertions are updated
- **THEN** each pinned usage-error exit assertion reads `2`
- **AND** the four `cli-parser.test.ts` `it()` titles that literally say "exits 1"
  are renamed to "exits 2"
- **AND** the four `notEqual(cap.exitCode, 1)` success-path assertions in
  `completions.test.ts` are left as-is (still valid)

#### R5: Missing usage-error coverage SHOULD be added
Exit-2 coverage SHOULD be added for `Unknown tool` (`dispatchSingleTool`) and
`Unknown argument` (the `main` parseDataArgs catch) — sites 15 and 16 have no
existing exit-code test. `-u requires a username` (site 11) MAY be added.

- **GIVEN** no existing test asserts the exit code for unknown tool / unknown argument
- **WHEN** coverage is added
- **THEN** a test asserts `dispatchSingleTool` on an unknown tool key exits `2` and
  writes `Unknown tool:` + `SHORT_USAGE` to stderr
- **AND** a test asserts the unknown-argument path exits `2`

### Docs: Exit-Code Table

#### R6: usage.md MUST document exit codes per subcommand
`docs/specs/usage.md` MUST gain an `## Exit Codes` section stating the `0/1/2`
convention and a per-subcommand table (principle №4: exit codes documented in the
CLI-surface spec). README and `docs/site/` are out of scope.

- **GIVEN** the CLI-surface spec `docs/specs/usage.md`
- **WHEN** the section is added
- **THEN** it states `0` success / `1` operational failure / `2` usage error
- **AND** a table maps each subcommand family to its possible exit codes

### Non-Goals
- No `package.json` version edit — the release flow (`just release minor`) owns the
  bump; the PR description flags that the next release must be `minor`, not `patch` (R6 is docs, not release).
- No change to message text, stderr-vs-stdout routing, or any operational exit code.
- No change to `dispatchSingleToolLines` (watch-mode unknown-tool path returns a
  string array, never calls `process.exit`) — it is not one of the 16 sites.

### Design Decisions
1. **Named constant `EXIT_USAGE = 2`, not a `usageError()` helper** (intake assumption 4, decided here): The 14 `parseGlobalFlags` sites and site 1 already own their `console.error(msg)` call, and sites 15/16 print a second `console.error(SHORT_USAGE)` line before exiting. A `usageError(msg)` helper printing one message + exit would not compose with the two-line stderr of sites 15/16 without extra parameters, and would move the message text away from each branch (reducing local readability). A plain constant swapped into the existing `process.exit(...)` calls is the minimal, byte-preserving change — *Why*: least surface area, message text stays inline at each site, satisfies no-magic-numbers. *Rejected*: `usageError()` helper (doesn't compose with the SHORT_USAGE two-liner cleanly).
2. **`## Exit Codes` placed after `## Setup Commands`, before `## Data Model`**: keeps CLI-surface semantics together (grammar → flags → setup → exit codes) ahead of the data-model internals.

## Tasks

<!-- Each item carries a <!-- R# --> trace annotation. -->

### Phase 1: Core Implementation

- [x] T001 Add `const EXIT_USAGE = 2;` near the other module-level constants in `src/node/core/cli.ts` (e.g. alongside `SHORT_USAGE`/`FULL_HELP` or the exit-code usage area), with a brief comment naming the toolkit `0/1/2` convention <!-- R3 -->
- [x] T002 Flip site 1 (`runShellInit` unknown-shell, ~line 315) `process.exit(1)` → `process.exit(EXIT_USAGE)` in `src/node/core/cli.ts` <!-- R1 -->
- [x] T003 Flip sites 2-14 (all `parseGlobalFlags` usage-error branches: interval numeric/min/max, the 6-row format-conflict matrix, `-u` username, since/until date/inverted, ~lines 708-770) `process.exit(1)` → `process.exit(EXIT_USAGE)` in `src/node/core/cli.ts` <!-- R1 -->
- [x] T004 Flip site 15 (`dispatchSingleTool` unknown-tool, ~line 966, keeping the `SHORT_USAGE` stderr line) and site 16 (`main` parseDataArgs catch, ~line 1336, keeping the `SHORT_USAGE` stderr line) `process.exit(1)` → `process.exit(EXIT_USAGE)` in `src/node/core/cli.ts` <!-- R1 -->
- [x] T005 Verify the 9 operational sites and `watch.ts` `process.exit(0)` are unchanged (grep confirmation, no edit) <!-- R2 -->

### Phase 2: Tests

- [x] T006 Update `src/node/core/__tests__/cli-parser.test.ts`: change the 14 usage-error `assert.equal(s.code, 1)` → `2` (8 format-conflict + 2 `-j` alias + 4 since/until validation), and rename the four `it()` titles that say "exits 1" → "exits 2" <!-- R4 --> <!-- rework: cycle 1 — only 3 of 4 "exits 1" titles were renamed; cli-parser.test.ts:425 `--until with a malformed value exits 1 with its own name` must read "exits 2" (must-fix, A-008) -->
- [x] T007 [P] Update `src/node/core/__tests__/cli-watch-flag.test.ts`: change the 6 usage-error `assert.equal(s.code, 1)` → `2` (interval min/max/missing/non-numeric/fractional, `--watch`+`--json`) <!-- R4 -->
- [x] T008 [P] Update `src/node/core/__tests__/completions.test.ts`: change the 1 unknown-shell `assert.equal(cap.exitCode, 1)` → `2`; leave the four `notEqual(cap.exitCode, 1)` success-path assertions unchanged <!-- R4 --> <!-- rework: cycle 1 — also rename the stale title at completions.test.ts:131 `emits stderr message and exits 1` → "exits 2" (should-fix, same defect class as the must-fix). The file has FOUR notEqual success-path guards (lines 88/101/112/126), not three — leave them unchanged; the reviewer's nice-to-have tightening is skipped (their suggested `equal(cap.exitCode, undefined)` is also wrong — the capture initializes exitCode to null) -->
- [x] T009 Add exit-2 coverage tests for the previously-uncovered usage-error exit paths in `src/node/core/__tests__/cli-exit-codes.test.ts` (subprocess-based, spawns the real CLI via tsx): unknown argument (site 16), `-u` missing value (site 11), incompatible format flags, plus a `help` exits-0 baseline. NOTE: site 15 (`dispatchSingleTool` `Unknown tool`) is a **defensive branch structurally unreachable via the CLI entry** — every `KNOWN_SOURCES` value resolves to a valid `TOOLS` key today, and `all` routes to `dispatchAll*`, so it can't be triggered end-to-end without adding a test-only export (Test Integrity forbids that); it is code-inspection-verified instead <!-- R5 -->

### Phase 3: Docs

- [x] T010 [P] Add an `## Exit Codes` section to `docs/specs/usage.md` after `## Setup Commands` (before `## Data Model`): state the `0/1/2` convention and a per-subcommand table <!-- R6 -->

## Execution Order

- T001 blocks T002-T004 (they reference `EXIT_USAGE`)
- T006-T010 run after the source flips (T002-T004); T007, T008, T010 are mutually parallel
- T005 is a verification-only step, run any time after T002-T004

## Acceptance

### Functional Completeness

- [x] A-001 R1: All 16 usage-error sites in `src/node/core/cli.ts` exit `2`; a `grep -n "process.exit(1)"` on cli.ts returns only the 9 operational sites and `main().catch`
- [x] A-002 R2: The 9 operational sites still exit `1` and `watch.ts` still exits `0` on quit (unchanged)
- [x] A-003 R3: `const EXIT_USAGE = 2` exists in cli.ts and every usage-error exit references it; no bare `process.exit(2)` literal remains
- [x] A-004 R4: The ~21 pinned usage-error test assertions read `2` (14 + 6 + 1 = 21 verified); `cli-sync.test.ts`/`cli-init-metrics.test.ts` untouched
- [x] A-005 R5: New tests assert exit `2` for the reachable usage-error paths (unknown argument ×2, `-u` missing value, incompatible flags); unknown tool is code-inspection-verified (defensive, unreachable via the CLI entry — every `KNOWN_SOURCES` value normalizes to a valid `TOOLS` key or routes to `dispatchAll*`)
- [x] A-006 R6: `docs/specs/usage.md` has an `## Exit Codes` section with the convention statement and a per-subcommand table

### Behavioral Correctness

- [x] A-007 R1: Error message text and stderr routing are byte-identical to before (only the exit code differs); the format-conflict, since/until, and interval messages are unchanged (diff touches only `process.exit` lines)
- [x] A-008 R4: The four renamed `it()` titles in `cli-parser.test.ts` read "exits 2" — re-verified after rework cycle 1: all four renamed (incl. `cli-parser.test.ts:425` `"--until with a malformed value exits 2 with its own name"`); the only remaining `"exits 1"` title in the test tree is the operational-path `cli-sync.test.ts:160`, correctly untouched

### Scenario Coverage

- [x] A-009 R1: `npx tsx --test` on `cli-parser.test.ts`, `cli-watch-flag.test.ts`, `completions.test.ts` passes with the exit-2 assertions (282/282 pass; new `cli-exit-codes.test.ts` 5/5 pass)
- [x] A-010 R2: `cli-sync.test.ts` and `cli-init-metrics.test.ts` still pass unmodified (files untouched by the diff; their 1+3 pre-existing env/config-resolution failures match the pristine-tree baseline exactly — full-suite fail count unchanged at 13)

### Edge Cases & Error Handling

- [x] A-011 R1: Sites 15/16 still print `SHORT_USAGE` to stderr as a second line before exiting `2` (code-verified both; subprocess test asserts both stderr lines for site 16)

### Code Quality

- [x] A-012 Pattern consistency: The `EXIT_USAGE` constant follows the module's existing const-declaration style; edits are minimal and localized to the exit calls
- [x] A-013 No unnecessary duplication: A single named constant is reused at all 16 sites rather than repeating literal `2`
- [x] A-014 No magic numbers: No bare `2` literal appears at any usage-error exit site (code-quality.md)

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`
- PR description must flag: next release is `just release minor` (0.9.x → 0.10.0), not patch — machine-observable contract change per Output Stability.

## Deletion Candidates

None — this change flips exit codes in place (constant + 16 call-site edits) without making any existing code, function, branch, or config redundant. Re-verified at review cycle 1: no superseded helper exists (`EXIT_USAGE` is the first and only exit-code constant in the module; no bare `process.exit(2)` remains), the new subprocess test file duplicates no existing helper (unit tests use in-process `captureExit`/`captureIo` mocks; no other test file uses `spawnSync` to run the CLI end-to-end), and the four `assert.notEqual(cap.exitCode, 1)` success-path guards in `completions.test.ts` (lines 88/101/112/126) remain live assertions.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Exactly the 16 usage-error sites move to exit 2; the 9 operational sites and `watch.ts` exit(0) stay unchanged | Intake enumerates the split explicitly; every site verified against live cli.ts at plan time | S:95 R:85 A:95 D:90 |
| 2 | Certain | Message text and stderr routing stay byte-identical; only exit codes change | Intake scopes the change to exit codes only | S:90 R:90 A:90 D:90 |
| 3 | Certain | The ~21 exit-1 usage-error test assertions move to 2; operational-path tests untouched | Constitution Test Integrity: spec is source of truth, tests verify conformance | S:85 R:90 A:95 D:90 |
| 4 | Confident | Use a named constant `EXIT_USAGE = 2` rather than a `usageError()` helper | Constant composes cleanly with the two-line stderr of sites 15/16 and keeps message text inline at each site; helper would not (Design Decision 1) | S:60 R:90 A:80 D:70 |
| 5 | Confident | The `## Exit Codes` table lands in `docs/specs/usage.md` after `## Setup Commands`; README/docs-site untouched | Principle №4 names the CLI-surface spec — usage.md is tu's; placement keeps CLI-surface sections together | S:65 R:85 A:75 D:75 |
| 6 | Confident | No `package.json` edit; PR flags the next release as `just release minor` | Release flow owns bumps; constitution requires minor for machine-format changes | S:70 R:80 A:70 D:65 |
| 7 | Confident | Add missing exit-2 coverage for unknown tool and unknown argument | code-review.md: output changes SHOULD carry test coverage; small additive tests | S:55 R:90 A:80 D:70 |

7 assumptions (3 certain, 4 confident, 0 tentative).
