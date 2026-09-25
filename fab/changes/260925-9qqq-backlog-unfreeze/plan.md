# Plan: Backlog Unfreeze (Go-port row Z2)

**Change**: 260925-9qqq-backlog-unfreeze
**Intake**: `intake.md`

## Requirements

### Backlog: close rows whose work already shipped

#### R1: Shipped rows are closed with the PR or commit that shipped them
Each of the six backlog rows the intake found already done MUST flip from `- [ ]` to `- [x]`, keep its original text verbatim, and gain a trailing `**DONE (YYYY-MM-DD)**: …` sentence naming what shipped it. The six rows and their close facts:

| Row | DONE date | Shipped by | Go carrier |
|-----|-----------|------------|------------|
| unnamed 2026-06-03 hermetic test suite | 2026-09-25 | made moot by [#101](https://github.com/sahil87/tu/pull/101) (the named tests were deleted with `src/node/`); Go suite verified green with `TU_METRICS_REPO` and `NO_COLOR` exported (23/23 packages, 2026-09-25) | — |
| `[9ceu]` | 2026-06-10 | metrics-repo commit `c3b64c7` "repair: restore day-files shrunk by pre-0.5.0 writeMetrics purge bug (tu#34, +$10511.59 across 172 files)" | script is now `src/go/cmd/turepair` over `sync.Repair` (#94): `bin/turepair [--repo <path>] [--write]` |
| `[ccfx]` | 2026-07-03 | [#39](https://github.com/sahil87/tu/pull/39) | `src/go/internal/source/ccusage/registry.go` `cc: prefixArgs ["claude"], labelKey "date"` (#84, moved in #88) |
| `[gmcp]` | 2026-07-03 | [#42](https://github.com/sahil87/tu/pull/42) | `src/go/internal/fact/tool.go` six-tool registry (#84) |
| `[sntl]` | 2026-07-03 | [#40](https://github.com/sahil87/tu/pull/40) | `src/go/internal/command/parse.go` `--since/-s`, `--until`, `-j` (#87) |
| `[wkly]` | 2026-07-03 | [#41](https://github.com/sahil87/tu/pull/41) | `src/go/internal/query/query.go` weekly roll-up, `w`/`weekly`/`wh` in `command/parse.go` (#87) |

The four July rows' notes MUST say the checkbox was stale (closed at the Z2 re-triage, 2026-09-25), mirroring the `v76l` row's wording.

- **GIVEN** `fab/backlog.md` at HEAD `b04f6bb` with nine rows
- **WHEN** the change lands
- **THEN** the six rows above read `- [x]` and end with a `**DONE (…)**:` sentence citing the PR/commit from the table
- **AND** the `[9ceu]` row's two `\ `-prefixed continuation lines are unchanged and still follow the row

#### R2: Re-point the still-open `[4d46]` row at the Go packages
The `[4d46]` row MUST stay `- [ ]`, keep its ID, date, and the restore-vs-remove decision it asks for, and have its Node references rewritten in place so the row is correct against `src/go/`:

- `isStale() in sync.ts` → `sync.Stale`/`sync.StaleAfter` in `src/go/internal/sync/state.go` (no non-test caller; memory `docs/memory/sync/git-flow.md` DD "The auto-sync TTL ships caller-less")
- `auto_sync is a no-op that tu status reports as 'on'` → cite `autoSyncWord` in `src/go/internal/config/status.go` and `config.AutoSync` in `src/go/internal/config/config.go`; `init-conf` writes the key with the comment "no longer auto-triggers" (`src/go/internal/config/setup.go`)
- `Check git log on sync.ts for why auto-sync was removed first` → `git log -S isStale` (the Node file is gone from the tree; `df41538` #72 is the last Node commit that touched the status line)

The row MUST end with `**Re-pointed (2026-09-25, Z2)**: Node paths replaced after #101; …`.

- **GIVEN** the `[4d46]` row cites `sync.ts`
- **WHEN** the change lands
- **THEN** the row cites only paths that exist under `src/go/` and still opens with `- [ ] [4d46] 2026-08-27:`

#### R3: Untouched rows stay byte-identical
The `[x] [v76l]` row and the `[ ] [s3kd]` row MUST NOT change.

- **GIVEN** the two rows at HEAD
- **WHEN** the change lands
- **THEN** `git diff` shows no hunk touching either row

#### R4: No open row references a deleted Node path
After the edit, no `- [ ]` row in `fab/backlog.md` MAY reference `src/node/`, `scripts/repair-metrics.mjs`, `scripts/help-dump.mjs`, `*.test.ts`, or `__tests__/`.

- **GIVEN** the edited file
- **WHEN** open rows are grepped for those strings
- **THEN** there are zero matches (closed `[x]` rows may still mention them as history)

### Plan: record Z2 as landed

#### R5: Plan row Z2 and the status paragraph are updated
In `fab/plans/sahil/26-09-15-go-port.md`, row Z2's **PR** column MUST read `260925-9qqq-backlog-unfreeze` (the PR link is appended at ship once the number exists, as Z1's row was), and its **Status** column MUST read `**landed**` followed by a one-line outcome (6 closed, 1 re-pointed, 2 untouched). The top **Status (2026-09-25)** paragraph MUST replace "Remaining: Z2 (backlog re-triage) — on Sahil's explicit go." with a sentence stating Z2 landed and every row of the plan is done. D4's prose is left unchanged.

- **GIVEN** the plan at HEAD with Z2 "not started"
- **WHEN** the change lands
- **THEN** Z2's Status cell starts with `**landed**` and the top paragraph no longer says "Remaining: Z2"

### Non-Goals

- Deciding `[4d46]` (restore auto-sync vs remove the knob) — that is its own change's intake.
- Deleting or rewording closed rows, including `v76l`'s mention of `scripts/help-dump.mjs`.
- Any `src/go/`, test, golden, harness, or memory edit.

## Tasks

### Phase 1: Core Implementation

- [x] T001 In `fab/backlog.md`, close the six shipped rows per R1's table: flip `- [ ]` → `- [x]` and append the `**DONE (date)**:` sentence to each (hermetic-tests row, `[9ceu]`, `[ccfx]`, `[gmcp]`, `[sntl]`, `[wkly]`); leave `[9ceu]`'s `\ ` continuation lines in place. <!-- R1 -->
- [x] T002 In `fab/backlog.md`, rewrite the `[4d46]` row's Node references in place per R2 and append the `**Re-pointed (2026-09-25, Z2)**:` sentence; leave `[v76l]` and `[s3kd]` untouched. <!-- R2, R3 -->
- [x] T003 In `fab/plans/sahil/26-09-15-go-port.md`, set row Z2's PR column to `260925-9qqq-backlog-unfreeze` and Status to `**landed**` + outcome line; rewrite the top Status paragraph's "Remaining: Z2 …" sentence per R5. <!-- R5 -->

### Phase 2: Verification

- [x] T004 Verify: `grep -c '^- \[x\]' fab/backlog.md` = 7 and `grep -c '^- \[ \]' fab/backlog.md` = 2; open rows contain no `src/node/`, `repair-metrics.mjs`, `help-dump.mjs`, `.test.ts`, or `__tests__` string; `git diff` has no hunk in the `v76l`/`s3kd` rows; `fab docs-index docs/memory --check` still exits 0 (nothing under docs/ touched). <!-- R4, R3 -->

## Acceptance

### Functional Completeness

- [x] A-001 R1: The six rows named in R1 read `- [x]` and each ends with a `**DONE (…)**:` sentence citing the PR or commit from R1's table
- [x] A-002 R2: The `[4d46]` row is still open, cites `src/go/internal/sync/state.go` and `src/go/internal/config/status.go`, no longer cites `sync.ts`, and ends with the `**Re-pointed (2026-09-25, Z2)**:` sentence
- [x] A-003 R5: Plan row Z2's PR column is `260925-9qqq-backlog-unfreeze`, its Status starts with `**landed**`, and the top status paragraph no longer says "Remaining: Z2"

### Behavioral Correctness

- [x] A-004 R1: The four July rows' DONE notes state the checkbox was stale and name both the July PR and the Go carrier PR
- [x] A-005 R1: The `[9ceu]` DONE note names metrics-repo commit `c3b64c7` (2026-06-10) and the `turepair` re-point

### Scenario Coverage

- [x] A-006 R4: No `- [ ]` row in `fab/backlog.md` references `src/node/`, `scripts/repair-metrics.mjs`, `scripts/help-dump.mjs`, a `.test.ts` file, or `__tests__/`
- [x] A-007 R3: `git diff main -- fab/backlog.md` contains no hunk touching the `[v76l]` or `[s3kd]` rows

### Edge Cases & Error Handling

- [x] A-008 R1: The two `\ `-prefixed continuation lines after `[9ceu]` are unchanged and still immediately follow that row

### Code Quality

- [x] A-009 Pattern consistency: close notes use the same `**DONE (date)**:` shape as the existing `v76l` row; row IDs, dates, and ordering are preserved
- [x] A-010 No unnecessary duplication: no row text is duplicated or re-added; the file still has exactly nine rows
- [x] A-011 Minimum pathways: only `fab/backlog.md` and `fab/plans/sahil/26-09-15-go-port.md` change (`git diff --stat main` shows those two files plus `fab/changes/260925-9qqq-*`)

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Close notes are appended sentences, not rewrites; the original row text stays verbatim | Matches the intake's assumption 1 and the `v76l` precedent | S:85 R:95 A:90 D:90 |
| 2 | Confident | The plan row's PR link is left for ship to append (folder name only at apply) | The PR number does not exist until `/git-pr` runs; Z1's row was completed the same way | S:60 R:95 A:85 D:80 |

2 assumptions (1 certain, 1 confident, 0 tentative).
