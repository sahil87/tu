# Plan: Sync Drop-at-Cutover Fixes

**Change**: 260928-ubws-sync-drop-at-cutover-fixes
**Intake**: `intake.md`

## Requirements

### Sync: git round trip (`src/go/internal/sync/flow.go`)

#### R1: Pull target follows the repo, not a hard-coded `main` (DC-18)
`SyncMetrics` SHALL NOT pull `origin main` unconditionally. When the current branch has an upstream (`git rev-parse --abbrev-ref --symbolic-full-name @{u}` exits 0 with non-empty output), it MUST run `git pull --rebase` with no remote/refspec. Otherwise it MUST resolve the remote default branch from `git ls-remote --symref origin HEAD` (the `ref: refs/heads/{branch}\tHEAD` line) and run `git pull --rebase origin {branch}`. Failure handling (warning line, rebase abort, return false) is unchanged.

- **GIVEN** a metrics clone whose default branch is `master` with upstream `origin/master`
- **WHEN** `tu sync` runs
- **THEN** git receives `pull --rebase` (no `origin main`) and the sync succeeds

- **GIVEN** a metrics repo with no upstream configured but a remote whose HEAD is `refs/heads/trunk`
- **WHEN** `tu sync` runs
- **THEN** git receives `pull --rebase origin trunk`

#### R2: Empty remote first sync (DC-18)
When no upstream is configured AND the remote advertises no refs (ls-remote prints no `ref:` HEAD line), `SyncMetrics` MUST skip the pull and push with `git push -u origin HEAD` (retry-once behavior as for the ordinary push). An `ls-remote` that itself fails MUST be treated as a pull failure (`Warning: sync pull failed — {err}`, return false).

- **GIVEN** a fresh clone of an empty bare repo
- **WHEN** `tu sync` runs with data to commit
- **THEN** the commit is pushed with `push -u origin HEAD`, stdout prints `Synced to …`, exit 0

#### R3: add/status/commit failures surface the git error (DC-18)
A failing `git add {user}/`, `git status --porcelain {user}/`, or `git commit` MUST append a stderr line `Warning: sync {add|status|commit} failed — {git error}` (em dash U+2014, the same shape as the existing pull line) before returning false. Existing pull/push warning lines and the edge's closing `Error: sync failed — check network and remote config.` are unchanged.

- **GIVEN** a metrics repo with no git identity
- **WHEN** `tu sync` runs and `git commit` fails
- **THEN** stderr contains `Warning: sync commit failed — ` followed by git's error, then the `Error: sync failed …` line; exit 1

#### R4: Commit message uses the local date (DC-21)
`CommitMessage(user, now)` MUST format the date as `now` in local time (`now.Local()`), the same basis the day-file labels use. `.last-sync` remains UTC ISO.

- **GIVEN** local zone UTC−7 and a clock of 2026-09-16 20:00 local (2026-09-17 03:00 UTC)
- **WHEN** a sync commits
- **THEN** the message is `# {user}: update 2026-09-16`

### Sync: dry-run report (`src/go/internal/sync/report.go`, `writer.go`, `flow.go`)

#### R5: Dry-run costs use thousands separators (DC-22)
`Report.Format` MUST render every cost — `Would write` lines (incoming and the `(update: $X → $Y)` note) and `Would skip` lines — with the shared `render.FormatCost` (`$1,234.56`), not `"$" + FixedHalfUp`.

- **GIVEN** an existing day-file at $99,999.00 and incoming $54.93
- **WHEN** `tu sync --dry-run` runs
- **THEN** the skip line reads `incoming $54.93 < existing $99,999.00`

#### R6: Dry-run pull wording has no hard-coded branch (DC-18)
The two `Would commit:` lines MUST read `…, then pull --rebase, then push` (no `origin main`). The dry-run MUST NOT run `rev-parse @{u}`, `ls-remote`, or any other probe beyond the existing read-only `status --porcelain {user}/`.

- **GIVEN** any dry-run
- **WHEN** the report prints
- **THEN** the last-but-one line ends `then pull --rebase, then push`

#### R7: Identical rewrites are "unchanged", not updates (DC-23)
`Write` MUST classify a record whose serialized bytes (`json.Marshal(DayFile)` + `"\n"`) equal the existing file's bytes as a new `ActionUnchanged` decision (the never-shrink check runs first and still wins). Live mode MAY still write the file (same bytes — keep the single write path per code-quality "minimum pathways"). `Report.Format` MUST omit unchanged decisions from the `Would write` list and count, and `FullSync`'s dry-run `anyWrite` MUST count only `ActionWrite`, so `WouldCommit` is true only when bytes would change or `status --porcelain` is non-empty. An equal-cost write with different bytes stays an `ActionWrite` `(update: …)`.

- **GIVEN** a clean metrics repo whose day-files already hold exactly what the fetch returns
- **WHEN** `tu sync --dry-run` runs
- **THEN** it prints `Would write 0 day-file(s) under …` and `Would commit: nothing (no changes), then pull --rebase, then push`

### Specs and harness

#### R8: Spec rewritten, ledger resolved, goldens regenerated
`docs/specs/usage.md` lines carrying `(DC-18)`, `(DC-21)`, `(DC-22)`, `(DC-23)` MUST describe the new behavior and drop the tag; each of the four § Drop at cutover entries MUST be marked resolved as drop (keep the entry and its ID; replace `[DECIDE: keep|drop]` with `[DECIDED: drop]` and add a one-line "Now:" note). `docs/specs/layouts.md` sync/dry-run mockups (~L869–891) MUST match. Affected goldens (package `testdata/*.golden` where present, `harness/golden/` run + live via `bin/tudiff --update`) MUST be regenerated so `just go-test` and the tudiff gate are green. The harness `EnvPullfail` fakegit rule (`src/go/internal/harness/diff.go:117`) keeps matching `pull`.

- **GIVEN** the change is applied
- **WHEN** `grep -n 'DC-18\|DC-21\|DC-22\|DC-23' docs/specs/usage.md` runs
- **THEN** only the four resolved ledger entries match

### Non-Goals

- DC-20 auto-sync / `.last-sync` staleness — open, decided with backlog [4d46]
- DC-19 path/channel style of clone messages — open
- Changing the edge's `Error: sync failed — check network and remote config.` line or exit codes

### Design Decisions

#### Upstream-first pull target
**Decision**: Pull the current branch's upstream when one is configured; else the remote's default branch from `ls-remote --symref origin HEAD`; an empty remote skips the pull and pushes `-u origin HEAD`.
**Why**: `git clone` sets an upstream for the default branch, so the common path costs one local `rev-parse` and no extra network call; the fallbacks cover manually set-up repos and the empty-repo first sync the DC-18 ledger names.
**Rejected**: `git pull --rebase origin HEAD` alone — still fails on an empty remote; parsing git's `couldn't find remote ref` stderr to decide a fallback — brittle across git versions and locales.
*Introduced by*: 260928-ubws-sync-drop-at-cutover-fixes

#### Byte equality decides "unchanged"
**Decision**: A new `ActionUnchanged` decision when the would-be bytes equal the existing file; live mode still writes through the one write path.
**Why**: Byte equality is exactly what git sees, so the preview's commit prediction matches the live run; keeping the live write avoids a second code path (code-quality "minimum pathways").
**Rejected**: Treating equal *cost* as unchanged — a token-only change with equal cost would be hidden from the preview although git commits it.
*Introduced by*: 260928-ubws-sync-drop-at-cutover-fixes

## Tasks

### Phase 1: Core Implementation

- [x] T001 In `src/go/internal/sync/flow.go`, replace `pullArgs` with pull-target resolution (rev-parse `@{u}` → `pull --rebase`; else `ls-remote --symref origin HEAD` → `pull --rebase origin {branch}`; no refs → skip pull, `push -u origin HEAD` with the existing retry-once), extract it into a small helper so `SyncMetrics` stays under ~50 lines; update the `DC-18` comments; table-driven tests in `flow_test.go` for upstream / no-upstream-with-default / empty-remote / ls-remote failure using the fake `Runner` <!-- R1 R2 -->
- [x] T002 In `src/go/internal/sync/flow.go`, add named constants and warning lines `Warning: sync add failed — `, `Warning: sync status failed — `, `Warning: sync commit failed — ` on those three failures; tests in `flow_test.go` <!-- R3 -->
- [x] T003 [P] In `src/go/internal/sync/state.go`, make `CommitMessage` use `now.Local()`; update its doc comment; test in `state_test.go` with a non-UTC zone crossing midnight <!-- R4 -->
- [x] T004 In `src/go/internal/sync/writer.go`, add `ActionUnchanged` and classify by byte equality after the shrink check (reuse the bytes already read in `shrinkState` or re-read once — keep one write path); in `flow.go` dry-run count only `ActionWrite` toward `anyWrite`; tests in `writer_test.go` and `flow_test.go` (identical bytes → unchanged; equal cost, different tokens → write) <!-- R7 -->
- [x] T005 In `src/go/internal/sync/report.go`, switch `fmtCost` to `render.FormatCost`, omit `ActionUnchanged` from writes, change both `Would commit` lines to `then pull --rebase, then push`; update the DC-22 comment; tests in `report_test.go` <!-- R5 R6 R7 -->

### Phase 2: Integration

- [x] T006 Run `cd src/go && go test ./... -count=1` (unset `TU_METRICS_REPO`/`NO_COLOR` via `env -u`), regenerate any golden-bearing package that changed with `-update`, then `just go-build` and regenerate the harness corpus with `bin/tudiff --update` for the run and live goldens (see `justfile` targets); confirm `just go-test` and the tudiff gate (`just` tudiff/live targets) are green; inspect the golden diff — only sync/dry-run cases and live sync steps may change <!-- R8 -->
- [x] T007 Update `docs/specs/usage.md` (§ Multi-machine sync steps ~L406, output ~L409, dry-run ~L425, and the four ledger entries DC-18/21/22/23 → `[DECIDED: drop]` + "Now:" line) and `docs/specs/layouts.md` (~L869–891 sync + dry-run mockups) <!-- R8 -->

## Acceptance

### Functional Completeness

- [x] A-001 R1: With an upstream configured, sync issues `pull --rebase` with no `origin main`; without one, it pulls the ls-remote default branch
- [x] A-002 R2: An empty remote skips the pull and pushes `-u origin HEAD`; an ls-remote failure yields the pull-failed warning and false
- [x] A-003 R3: add/status/commit failures each emit `Warning: sync {step} failed — {err}` before returning false
- [x] A-004 R4: `CommitMessage` uses the local date; `.last-sync` still UTC
- [x] A-005 R5: Both write and skip lines use thousands-separated costs
- [x] A-006 R6: Dry-run `Would commit` lines read `then pull --rebase, then push` and the dry-run runs no new git probes
- [x] A-007 R7: Byte-identical rewrites are `ActionUnchanged`, omitted from `Would write`, and do not set `WouldCommit`
- [x] A-008 R8: Spec lines rewritten, four ledger entries marked `[DECIDED: drop]`, layouts mockups updated

### Scenario Coverage

- [x] A-009 R1 R2: `flow_test.go` covers upstream, default-branch, empty-remote, and ls-remote-failure paths
- [x] A-010 R4: `state_test.go` covers a zone where local and UTC dates differ
- [x] A-011 R7: tests cover identical bytes → unchanged and equal cost/different bytes → write

### Edge Cases & Error Handling

- [x] A-012 R2: A failed `push -u origin HEAD` after retry prints the existing push-failed warning
- [x] A-013 R8: `go test ./... -count=1` and the tudiff run + live gates pass; golden diffs touch only sync/dry-run cases and live sync steps

### Code Quality

- [x] A-014 Pattern consistency: New code follows naming and structural patterns of surrounding code (named argv vars/constants like `pullArgs`, warning prefixes as consts)
- [x] A-015 No unnecessary duplication: `render.FormatCost` reused; no second write path in `Write`
- [x] A-016 No god functions: `SyncMetrics` stays readable, pull-target resolution in its own helper
- [x] A-017 No swallowed errors: every new git failure path surfaces a warning line

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Deletion Candidates

- None — this change adds new functionality without making existing code redundant (the superseded `pullArgs` var and `report.go`'s `fmtCost` closure were removed within the diff itself; `render.FixedHalfUp` keeps its other callers in repair.go, csv, view, watch, markdown)

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | Live mode still writes unchanged files (no-op bytes) rather than skipping | code-quality "Keep minimum pathways" prefers one write path over optimizing away a write | S:80 R:90 A:85 D:80 |
| 2 | Confident | ls-remote failure is reported as a pull failure | It is the pull step's network half; reuses the existing warning and rebase-abort path | S:75 R:85 A:80 D:75 |
| 3 | Confident | Ledger entries become `[DECIDED: drop]` with a one-line "Now:" note | IDs are stable per the ledger preamble; the marker must stop matching `[DECIDE:` | S:80 R:90 A:80 D:75 |
| 4 | Certain | Warning lines use the existing `Warning: sync {step} failed — ` shape | Matches the pull line already shipped | S:90 R:90 A:90 D:90 |

4 assumptions (1 certain, 3 confident, 0 tentative).
