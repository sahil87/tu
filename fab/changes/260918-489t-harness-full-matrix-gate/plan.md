# Plan: Harness Full-Matrix Gate (plan row R3)

**Change**: 260918-489t-harness-full-matrix-gate
**Intake**: `intake.md`

## Requirements

### Harness: expected-diffs data file

#### R1: Expected-diffs file schema and validation
A committed file `harness/expected-diffs.json` SHALL declare the differential harness's expected diffs as `{"schema": 1, "expected": [ … ]}`. `harness.LoadExpected(path) (*Expected, error)` in a new `src/go/internal/harness/expected.go` MUST decode it with `DisallowUnknownFields` and reject a trailing JSON value (the `LoadMatrix` pattern), and MUST validate: `schema == 1`; the `expected` key present (an empty array is valid, a missing key is invalid); per entry an `id` matching `^DC-\d{2}$` that is unique across entries, a non-empty `cases` array whose every pattern `path.Match` accepts (an `ErrBadPattern` is a load error), and a non-empty `reason`. Every error message MUST name the offending entry `id` (when known) and field. The committed file's initial content is the empty set: `{"schema": 1, "expected": []}` (every `[DECIDE]` marker in `docs/specs/usage.md` is unresolved, which means keep).

- **GIVEN** a file `{"schema":1,"expected":[{"id":"DC-05","cases":["snapshot-*-csv"],"reason":"x"}]}`
- **WHEN** `LoadExpected` runs
- **THEN** it returns one entry with `ID == "DC-05"` and no error
- **GIVEN** a file whose entry has `"id": "DC-5"`, or a duplicate `DC-05`, or `"cases": []`, or a pattern `"[unclosed"`, or an empty `reason`, or an unknown field, or `"schema": 2`, or no `expected` key
- **WHEN** `LoadExpected` runs
- **THEN** it returns an error naming the entry and field, and no `*Expected`

#### R2: Pattern matching
`(*Expected).Match(caseID, group string) (id string, ok bool)` SHALL try entries in file order and return the first entry that has a matching pattern. A pattern matches when `path.Match(pattern, caseID)` is true; a pattern containing no `/` MUST additionally be tried against `group` with `path.Match(pattern, group)`. A nil `*Expected` or an empty set never matches.

- **GIVEN** entries `DC-05: ["snapshot-*-csv"]` and `DC-21: ["sync-dry-run/*/*/*/alt", "sync-dry-run"]`
- **WHEN** matching `snapshot-all-csv/multi/default/pipe/fixed` (group `snapshot-all-csv`), `sync-dry-run/multi/default/pipe/alt` (group `sync-dry-run`), `sync-dry-run/multi/default/pipe/fixed` (group `sync-dry-run`), `sync-dry-run` (group `live`), and `snapshot-all/single/default/pipe/fixed` (group `snapshot-all`)
- **THEN** the results are `DC-05`, `DC-21`, `DC-21` (bare pattern hits the group), `DC-21` (bare pattern hits the live step ID), and no match

### Harness: tudiff run and live gate rule

#### R3: `--expected` flag and preflight
`tudiff run` and `tudiff live` SHALL accept `--expected <path>` (default `harness/expected-diffs.json`, resolved with the shared `resolvePathFlag` rule: an untouched default resolves against the repo root, an explicit relative value against the cwd). Preflight MUST load the file after the matrix check (`run`) or after the binary checks (`live`); a missing file prints exactly `tudiff: <path as given> not found` and any other load error prints exactly one `tudiff: expected-diffs: <loader error>` line, both exit 2. A missing file MUST NOT be treated as an empty set. `run --list` MUST NOT load the file.

- **GIVEN** `--expected /nonexistent.json`
- **WHEN** `tudiff run` starts
- **THEN** stderr is `tudiff: /nonexistent.json not found\n` and the exit code is 2
- **GIVEN** a valid matrix and an invalid expected-diffs file
- **WHEN** `tudiff run --list` runs
- **THEN** it prints the case IDs and exits 0

#### R4: Result annotation and summary counts
`harness.Result` SHALL gain `Expected string` (the matching entry `id`, or `""`). After the byte and tree comparisons, a red result MUST be annotated with `Match(caseID, group)`; green and timeout results never carry an `Expected` id. `harness.Summary` SHALL gain `Expected int` (red results with a non-empty `Expected`), `Unexpected int` (`Red - Expected`), `Entries []ExpectedStat` (one per file entry, in file order, with `ID`, `Matched` = executed results whose ID or group the entry's patterns match regardless of status, `Red` = matched results that are red), and `Stale []string` (entry IDs with `Matched >= 1` and `Red == 0`). An entry with `Matched == 0` is neither stale nor an error.

- **GIVEN** an expected set with `DC-05: ["diff"]` and results `same` green, `diff` red
- **WHEN** summarized
- **THEN** `Expected == 1`, `Unexpected == 0`, `Entries[0] == {DC-05, Matched: 1, Red: 1}`, `Stale` empty
- **GIVEN** the same set and both results green
- **WHEN** summarized
- **THEN** `Expected == 0`, `Stale == ["DC-05"]`
- **GIVEN** an entry matching no executed case
- **WHEN** summarized
- **THEN** its `Matched == 0` and it is absent from `Stale`

#### R5: Exit code
Both `run` and `live` SHALL exit `1` iff `Unexpected + Timeout + Unconfirmed > 0` or `len(Stale) > 0`, exit `0` otherwise, and exit `2` for usage/preflight errors as today. An expected red case therefore no longer fails the run; a case that replayed an `unconfirmed: true` fixture now does.

- **GIVEN** one red case matched by an expected entry and every other case green
- **WHEN** `tudiff run` finishes
- **THEN** the exit code is 0
- **GIVEN** all cases green and one case whose call log records a replay of an `unconfirmed: true` fixture
- **WHEN** `tudiff run` finishes
- **THEN** the exit code is 1
- **GIVEN** an entry whose matched cases are all green
- **WHEN** `tudiff run` finishes
- **THEN** the exit code is 1

### Harness: report

#### R6: report.txt format
`RenderHeader` SHALL emit, after the `matrix:` line, `expected: <path as given> (<n> entries)`. `RenderCaseLine` SHALL append ` [expected <id>]` to a red case line immediately after the divergence detail and before ` [unconfirmed]` and ` [calls differ …]`. The summary's first line SHALL read `tudiff: <N> cases — <g> green, <r> red (<e> expected, <x> unexpected), <t> timeout   (fixtures: <aliases>; <u> cases replayed unconfirmed fixtures)`; the four `by …` axis lines are unchanged; when the file has one or more entries, one line per entry follows the axis lines: `  expected: <id>  <red>/<matched> red` with ` (stale)` appended when matched ≥ 1 and red = 0, or ` (no executed case)` when matched = 0. With an empty set, no per-entry lines print. `tudiff live` SHALL carry the same header fields (`live` currently sets `MatrixPath: "live"`; it sets the expected path and count the same way).

- **GIVEN** a red result on `stdout` with `Expected == "DC-05"`, `Unconfirmed` true
- **WHEN** rendered
- **THEN** the line ends `… node=<q> go=<q> [expected DC-05] [unconfirmed]`
- **GIVEN** 452 green results and an empty set
- **WHEN** the summary renders
- **THEN** its first line is `tudiff: 452 cases — 452 green, 0 red (0 expected, 0 unexpected), 0 timeout   (fixtures: _placeholder; 0 cases replayed unconfirmed fixtures)` and no `expected:` entry line follows the axis lines

#### R7: report.json fields
`report.json` SHALL stay `schema: 1` and gain `header.expected` (path as given), `header.expected_entries` (int), `summary.expected`, `summary.unexpected`, `summary.stale` (string array, `[]` never `null`), and per case `expected` (the id or `""`). Existing fields and their order are unchanged; new fields are appended within their struct.

- **GIVEN** a run with one expected red case
- **WHEN** `report.json` is decoded
- **THEN** `summary.expected == 1`, `summary.unexpected == 0`, `summary.stale == []`, and that case's `expected` is its entry id

### Build: CI gate

#### R8: The `tudiff` job gates `ci-gate`
`.github/workflows/ci.yml` SHALL rename the job `go-diff` to `tudiff`, keep its steps and pinned action SHAs unchanged, remove `continue-on-error` from both the `just go-diff --placeholder` and `just go-live` steps, keep the `actions/upload-artifact` step with `if: always()`, name `tudiff-report`, and paths `bin/harness/report/` and `bin/harness/report-live/`, and add `tudiff` to `ci-gate`'s `needs` with a third check block in the same shape as the existing two (`if [ "${{ needs.tudiff.result }}" != "success" ]; then echo "tudiff did not succeed (result: ${{ needs.tudiff.result }})"; exit 1; fi`). The workflow header comment MUST name three lanes and the job comment MUST state the gate rule (exit 1 on unexpected red, timeout, unconfirmed replay, or stale expected entry; expected diffs come from `harness/expected-diffs.json`) instead of the "every case is red until Phase 1 lands … R3 removes …" narrative. `scripts/ci-gate-ruleset.sh` is untouched.

- **GIVEN** the edited workflow
- **WHEN** parsed
- **THEN** `jobs.tudiff` exists, `jobs.go-diff` does not, no step under `jobs.tudiff` has `continue-on-error`, `jobs.ci-gate.needs` is `[build-and-test, go-build-and-test, tudiff]`, and the ci-gate script checks `needs.tudiff.result`

#### R9: Justfile and README wording
The `justfile` `go-diff` recipe comment SHALL state the gate rule and name `harness/expected-diffs.json` in place of "not a gate yet"; the `go-live` comment SHALL drop "Exit 1 while any step is red" for the same rule. Recipe names and bodies are unchanged. `README.md` § CI / branch protection SHALL name the third lane (`tudiff`, the differential harness against the placeholder corpus) and say "all three lanes", after the toolkit standards governing README have been checked (`shll standards`, read the entries naming README; if `shll` is unavailable, note that and proceed with the one-paragraph edit).

- **GIVEN** the edited README
- **WHEN** the CI paragraph is read
- **THEN** it lists `build-and-test`, `go-build-and-test`, and `tudiff`, and states `ci-gate` passes only when all three succeed

### Non-Goals

- No change to `harness/matrix.json`, `docs/specs/usage.md`, or any `[DECIDE]` marker.
- No darwin runner; CI stays `ubuntu-latest`.
- No `$GITHUB_STEP_SUMMARY`, PR comment, or `release.yml` change.
- No change under `src/node/`, `src/go/cmd/tu`, `src/go/internal/command`, or any Goal-frozen external surface.

### Design Decisions

#### Expected diffs are a committed data file keyed by spec DC id
**Decision**: `harness/expected-diffs.json` (schema 1) lists entries `{id, cases, reason}` keyed by the spec's stable `DC-NN` IDs; both `tudiff run` and `tudiff live` load it via `--expected`.
**Why**: A `drop` resolution at G0/G3 must become one data entry, not a change to the gate's code; the matrix already models coverage as reviewed data, and DC IDs are declared stable in the spec.
**Rejected**: An `expected` field on matrix groups (a drop spans groups and is keyed by spec id, not group id); a `--expect` CLI flag (state would live in CI YAML, not beside the matrix).
*Introduced by*: 260918-489t-harness-full-matrix-gate

#### A missing expected-diffs file is a preflight error, never an empty set
**Decision**: `tudiff run`/`live` exit 2 when the file is absent or invalid.
**Why**: The harness never reports a vacuous green (`--filter` matching nothing and a missing placeholder manifest are already exit 2); the file is committed, so the default always exists.
**Rejected**: Treating absence as `expected: []` (a mis-resolved path would silently drop every expectation and turn intended reds into gate failures, or worse, hide a stale set).
*Introduced by*: 260918-489t-harness-full-matrix-gate

#### Stale entries fail, unmatched entries only report
**Decision**: An entry that matched at least one executed case and none red exits 1 with a `(stale)` line; an entry that matched zero executed cases prints `(no executed case)` and does not affect the exit code.
**Why**: An always-green expectation is a misconfiguration worth failing on; `--filter` and `live` legitimately exclude cases, so zero-matched must not fail, and this keeps `run` and `live` decoupled (no cross-runner case list).
**Rejected**: A preflight cross-check that every entry matches some case in the full matrix (breaks `live`-only entries and `--filter` runs).
*Introduced by*: 260918-489t-harness-full-matrix-gate

## Tasks

### Phase 1: Setup

- [x] T001 [P] Create `harness/expected-diffs.json` with exactly `{"schema": 1, "expected": []}` (2-space indent, trailing newline). <!-- R1 -->
- [x] T002 [P] Add `src/go/internal/harness/expected.go`: `Expected`/`ExpectedEntry`/`ExpectedStat` types, `LoadExpected` (DisallowUnknownFields, trailing-value rejection, the R1 validations with messages naming entry id and field), `Match` per R2, and `ExpectedStats(exp *Expected, results []Result) []ExpectedStat` + stale derivation used by R4. <!-- R1 -->
- [x] T003 Add `src/go/internal/harness/expected_test.go`: table tests over every R1 validation error, the valid empty and populated files, and the R2 scenario (group pattern, full-ID pattern, axis slice, live step, bare group `live`, first-entry-wins, nil set). <!-- R2 -->

### Phase 2: Core Implementation

- [x] T004 In `src/go/internal/harness/diff.go` add `Result.Expected string`; in `report.go` add the R4 `Summary` fields, extend `SummarizeResults` to take the `*Expected` (nil-safe) and fill `Expected`/`Unexpected`/`Entries`/`Stale`; update every caller (`cmd/tudiff/run.go`, `cmd/tudiff/live.go`, tests). <!-- R4 -->
- [x] T005 In `src/go/internal/harness/report.go`: `ReportHeader` gains `ExpectedPath string` and `ExpectedEntries int`; `RenderHeader` emits the `expected:` line after `matrix:`; `RenderCaseLine` appends ` [expected <id>]` before the unconfirmed marker; `RenderSummary` prints the R6 first line and per-entry lines; `reportHeaderJ`/`reportSummaryJ`/`reportCaseJ` gain the R7 fields (`stale` marshals as `[]`, never `null`); update `report_test.go` (`TestRenderHeader`, `TestRenderCaseLines`, `TestRenderSummary`, `TestWriteReport`) and add cases for the marker, the stale and no-executed-case lines, and the JSON fields. <!-- R6 -->
- [x] T006 In `src/go/cmd/tudiff/run.go`: add `--expected` (default const `defaultExpected = "harness/expected-diffs.json"`, resolved via `opts.resolve`), load it in preflight after the matrix check and after the `--list` early return (R3 messages), carry the path/count into `reportHeader`, annotate red results in `runCase` (after the tree comparison) with `Match(c.ID, c.Group)`, pass the set into `SummarizeResults`, and replace the exit rule with R5. <!-- R3 -->
- [x] T007 In `src/go/cmd/tudiff/live.go`: add the same `--expected` flag to `liveOptions`, load it in `preflightLive` after the binary checks (same R3 messages), set the header's expected path/count, annotate red step results with `Match(res.Case.ID, res.Case.Group)` where `runSequence` emits them (one place: the `emit` closure), pass the set into `SummarizeResults`, and apply the R5 exit rule. <!-- R5 -->
- [x] T008 Tests in `src/go/cmd/tudiff/run_test.go`: `newSmokeEnv` writes a valid empty `harness/expected-diffs.json` under the fake root; existing summary-line assertions move to the R6 text; new tests: (a) an expected red (`--expected` pointing at a file with `{"id":"DC-99","cases":["diff"],"reason":"smoke"}`) exits 0 and the case line carries ` [expected DC-99]`; (b) an entry matching only the green `same` case exits 1 and the summary shows `expected: DC-99  0/1 red (stale)`; (c) `--expected /nonexistent` exits 2 with `tudiff: /nonexistent not found\n`; (d) an invalid file exits 2 with one `tudiff: expected-diffs: …` line; (e) `--list` with an invalid expected file still exits 0; (f) unconfirmed gating: a smoke env whose placeholder manifest lists one fixture `{"source":"claude","period":"daily","args":["--json"],"file":"claude/daily.json",…,"unconfirmed":true}` and whose stub `tu` script appends `{"tool":"ccusage","argv":["claude","daily","--json"],"cwd":"","matched":"_placeholder/claude/daily.json"}` to `$TUDIFF_CALL_LOG` — both cases green, exit 1, summary reports `1 cases replayed unconfirmed fixtures`. Add a `live_test.go` unit test for the live preflight's expected-file error path if `preflightLive` is unit-testable without git fixtures; otherwise cover the flag through `run_test.go` only and note it in the task. <!-- R5 -->

### Phase 3: Integration & Edge Cases

- [x] T009 Edit `.github/workflows/ci.yml` per R8: rename `go-diff` → `tudiff`, remove both `continue-on-error`, keep the artifact step, add `tudiff` to `ci-gate.needs` with the third check block, rewrite the header and job comments. Verify with `python3 -c 'import yaml,sys; d=yaml.safe_load(open(".github/workflows/ci.yml")); print(list(d["jobs"]), d["jobs"]["ci-gate"]["needs"])'` (or `grep`) that `jobs.tudiff` exists, `go-diff` does not, and the needs list is complete. <!-- R8 -->
- [x] T010 Update `justfile` `go-diff`/`go-live` comments and the `README.md` § CI / branch protection paragraph per R9; before the README edit run `shll standards` and read the standards that name README (record which were read in the task's completion note). <!-- R9 -->
  Done: `shll standards` listed the catalog; read `readme-extraction` (the only entry naming README) — the one-paragraph CI-lane edit touches no governed rule (structure/links/images unchanged).

### Phase 4: Polish

- [x] T011 End-to-end verification from the repo root: `just go-lint`, `just go-test`, `just go-diff --placeholder` (expect `tudiff: 452 cases — 452 green, 0 red (0 expected, 0 unexpected), 0 timeout …` and exit 0, header line `expected: harness/expected-diffs.json (0 entries)`), `just go-live` (9/9, exit 0); confirm `bin/harness/report/report.json` has `summary.stale: []` and `header.expected_entries: 0`. Record the summary lines in this task's completion note. <!-- R5 -->
  Done: go-lint silent (exit 0); go-test all `ok` (exit 0); `tudiff: 452 cases — 452 green, 0 red (0 expected, 0 unexpected), 0 timeout   (fixtures: _placeholder; 0 cases replayed unconfirmed fixtures)` exit 0, header `expected: harness/expected-diffs.json (0 entries)`; `tudiff: 9 cases — 9 green, 0 red (0 expected, 0 unexpected), 0 timeout` exit 0; report.json `summary.stale: []`, `header.expected_entries: 0`.

## Execution Order

- T002 blocks T003, T004; T004 blocks T005, T006, T007; T005–T007 block T008; T011 last.
- T001, T009, T010 are independent of the Go tasks.

## Acceptance

### Functional Completeness

- [x] A-001 R1: `harness/expected-diffs.json` is committed with the empty set and `LoadExpected` accepts it and the populated example.
- [x] A-002 R2: `Match` implements the file-order, `path.Match`, bare-pattern-also-tries-group rule and a nil or empty set never matches.
- [x] A-003 R3: Both `run` and `live` accept `--expected`, resolve it with the shared rule, and exit 2 with the exact messages on a missing or invalid file; `--list` does not load it.
- [x] A-004 R4: `Result.Expected` is set only on red results; `Summary` carries `Expected`, `Unexpected`, `Entries`, `Stale` with the stated semantics.
- [x] A-005 R5: Exit code is 1 iff `Unexpected + Timeout + Unconfirmed > 0` or any stale entry, in both `run` and `live`.
- [x] A-006 R6: report.txt has the `expected:` header line, the ` [expected <id>]` marker in the stated position, the new summary first line, and per-entry lines only when the set is non-empty.
- [x] A-007 R7: report.json gains the stated fields at `schema: 1`, `summary.stale` is `[]` when empty, existing fields and order unchanged.
- [x] A-008 R8: `ci.yml` has `jobs.tudiff` (no `go-diff`), no `continue-on-error` in it, the artifact step with `if: always()`, and `ci-gate` needs and checks `tudiff`; `scripts/ci-gate-ruleset.sh` unchanged.
- [x] A-009 R9: justfile comments and the README CI paragraph state the gate and the three lanes; recipe names/bodies unchanged.

### Behavioral Correctness

- [x] A-010 R5: An expected red case no longer fails the run (exit 0 when it is the only red), and an unconfirmed replay now does (exit 1).
- [x] A-011 R6: With the committed empty set, the summary first line is `… 452 green, 0 red (0 expected, 0 unexpected), 0 timeout …` and `just go-diff --placeholder` exits 0; `just go-live` stays 9/9 and exits 0.

### Scenario Coverage

- [x] A-012 R1: `expected_test.go` covers every validation error listed in R1 with an assertion on the entry/field named in the message.
- [x] A-013 R2: `expected_test.go` covers the five-case R2 scenario including the live step ID and the bare `live` group.
- [x] A-014 R5: `run_test.go` covers expected-red exit 0, stale exit 1, missing/invalid file exit 2, `--list` unaffected, and unconfirmed exit 1.

### Edge Cases & Error Handling

- [x] A-015 R4: An entry matching zero executed cases is reported `(no executed case)` and neither fails the run nor appears in `Stale`.
- [x] A-016 R3: An explicit relative `--expected` resolves against the cwd, the default against the repo root (same as `--matrix`).

### Code Quality

- [x] A-017 Pattern consistency: `expected.go` follows `matrix.go`'s loader style (raw read, `DisallowUnknownFields`, trailing-value check, `validate()` with messages naming the offending id/field); stdlib-only.
- [x] A-018 No unnecessary duplication: `resolvePathFlag` is reused for `--expected`; the exit rule is expressed once per runner over `Summary`, not recomputed from results.
- [x] A-019 No magic strings: the default path is a named const beside `defaultMatrix`; the `[expected …]`, `(stale)`, `(no executed case)` literals live in `report.go` next to the existing marker literals.
- [x] A-020 Functions stay focused: no function grows past the file's typical size; `runRun`'s preflight addition is a few lines, with any loader-message shaping in `expected.go`.
- [x] A-021 Errors are never swallowed: every load/annotate error propagates to the existing `fail`/`return 2` paths with the stated messages.

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | `SummarizeResults` takes the `*Expected` as a second, nil-safe parameter and computes per-entry stats itself | One computation site for expected/unexpected/stale keeps `run` and `live` in lockstep; callers are three (run, live, tests) | S:60 R:90 A:85 D:70 |
| 2 | Confident | The missing-file message is `tudiff: <path as given> not found` (mirroring the `--node`/`--go` messages) and all other load errors are `tudiff: expected-diffs: <error>` | Matches the two existing preflight message shapes exactly | S:65 R:90 A:85 D:75 |
| 3 | Confident | Live red steps are annotated in the single `emit` closure so every step (including `harnessFailStep` results) passes through one place | Live builds results at many call sites; one funnel avoids missed annotations | S:55 R:90 A:80 D:70 |
| 4 | Confident | The unconfirmed-gating smoke test drives the call log from the stub `tu` script writing to `$TUDIFF_CALL_LOG` | The smoke env's ccusage fake is never invoked by the stub `tu`; the env variable is already passed to the child | S:50 R:90 A:75 D:65 |
| 5 | Confident | README standards check reads the `shll standards` entries naming README and records them in T010's completion note; if `shll` is absent the edit proceeds with a note | Constitution requires the check; the pipeline cannot block on an external CLI's presence | S:55 R:90 A:75 D:70 |

5 assumptions (0 certain, 5 confident, 0 tentative).
