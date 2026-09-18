# Intake: Harness Full-Matrix Gate (plan row R3)

**Change**: 260918-489t-harness-full-matrix-gate
**Created**: 2026-09-19

## Origin

> Context: fab/plans/sahil/26-09-15-go-port.md, row R3. Read the Decisions and Target architecture sections before writing the intake. Do not change any external surface listed under Goal. Scope: the harness runs the full matrix (just go-diff --placeholder) in CI on every PR as a tudiff job wired into ci-gate; the P1 Drop at cutover list in docs/specs/usage.md section Drop at cutover is applied to the matrix as expected diffs (every marker is still unresolved = keep, so the expected-diff set is empty today; wire the mechanism so a later drop decision becomes one entry, not a code change); the report.txt/json is uploaded as a CI artifact; zero unexpected diffs is the cutover precondition. Current baseline 452/452 green.

One-shot `/fab-new` invocation from the plan's queue (Phase 3, row R3, the last row before human gate G3 and the cutover release X1). No prior discussion thread in this session; the decisions below come from the plan row, the plan's Decisions D6/D7/D10, the gate table (G2, G3), the cutover criteria, the `ci.yml` comments that name R3 as the owner of gating, and the harness memory's "what R3 later turns into a gate failure" clause.

Baseline verified in this session before the intake was written (worktree `r3-harness-full-matrix-gate` at `f3831cb`):

```
$ just go-diff --placeholder
tudiff: 452 cases — 452 green, 0 red, 0 timeout   (fixtures: _placeholder; 0 cases replayed unconfirmed fixtures)
  by conf:  single 182/182  multi 176/176  org 47/47  legacy 47/47
  by env:   default 370/370  nocolor 34/34  envrepo 32/32  pullfail 8/8  pushfail 4/4  dirty 4/4
  by io:    pipe 376/376  tty 76/76
  by tz:    fixed 385/385  alt 67/67
$ just go-live
  … 9/9 green
```

`harness/fixtures/_placeholder/manifest.json` has zero `unconfirmed: true` entries (P3b closed 2026-09-18, all six sources in `confirmed.json`). `docs/specs/usage.md` § Drop at cutover carries 24 `[DECIDE: keep|drop]` markers, all unresolved.

## Why

**The problem.** The differential harness (D6) is the release gate for the Go port, but today it cannot block anything. The `go-diff` CI job runs `just go-diff --placeholder` and `just go-live` with `continue-on-error: true` on both steps and is absent from `ci-gate`'s `needs`, so a PR that turns any of the 452 cases red merges green. That posture was correct while every case was red (P4 landed with the count as a burndown), and it is wrong now that the count is 452/452: from here on a red case is a regression, and the plan's cutover criterion 1 ("zero unexpected diffs") has no enforcement point.

**The consequence of not doing it.** Phase 4 (X1, the 0.12.0 formula flip) is queued directly after R3 and G3. Without the gate, the harness evidence G3 reads is whatever the last person happened to run locally, and a divergence introduced by any PR between now and X1 would ship to every `tu update` user. D10 also keeps the harness running for two releases after cutover as the rollback safety net, which only works if it is a required check.

**Why expected diffs need a mechanism now, although the set is empty.** The spec's § Drop at cutover defines `drop` as "an expected diff in the differential harness (R3)". Every marker is still unresolved, which the plan treats as `keep` (Go reproduces the behavior byte-for-byte), so there is nothing to expect today. But G0/G3 may resolve a marker to `drop` later, and at that moment the Go side deliberately diverges and some cases go red on purpose. If the only way to express that is editing harness code, each drop decision becomes a code change to the gate itself, reviewed under time pressure right before a release. A committed data file keyed by the stable `DC-NN` IDs makes a drop decision one entry, and lets the report distinguish "red, expected under DC-05" from "red, unexpected" — the second number is the cutover precondition.

**Why also gate the unconfirmed-fixture marker.** P3b's row says "Blocks nothing until R3, where unconfirmed fixtures fail the gate", and the harness memory records the `[unconfirmed]` marker as "what R3 later turns into a gate failure". All six placeholder sources are confirmed now, so this is a no-op today, and it is exactly the kind of latent rule that should be wired while the count is zero rather than discovered when it is not.

**Why this shape over alternatives.**
- Renaming the job to `tudiff` and adding it to `ci-gate`'s `needs`, rather than making `go-diff` itself a required check: `ci-gate` is the single stable required status check enforced by the branch ruleset (`scripts/ci-gate-ruleset.sh`), and adding a lane means adding a `needs` entry, never touching the ruleset. The plan row and the invocation both name the job `tudiff`.
- A JSON data file beside `harness/matrix.json` rather than an `expected` field on matrix groups: a drop decision spans groups (DC-05 touches every CSV snapshot group) and is keyed by a spec ID, not a group ID; keeping the matrix a pure coverage description mirrors the "file is data, reviewed at G0" posture the matrix already has.
- Gating `go-live` in the same change rather than leaving it informational: cutover criterion 2 (live sync parity) is what G3 reads next to the matrix; the `ci.yml` comment on that step says "R3 owns gating it"; it is green (9/9) today.

## What Changes

### 1. The expected-diffs file: `harness/expected-diffs.json`

A new committed file, sibling of `harness/matrix.json`, loaded by both `tudiff run` and `tudiff live`. Initial content (the set is empty today because every `[DECIDE]` marker is unresolved and unresolved means keep):

```json
{
  "schema": 1,
  "expected": []
}
```

Entry shape (an example of what a future `drop` decision adds; NOT committed now):

```json
{
  "schema": 1,
  "expected": [
    {
      "id": "DC-05",
      "cases": ["snapshot-*-csv", "snapshot-all-csv/*/*/*/*"],
      "reason": "G0 dropped: CSV snapshot keeps zero-usage rows like CSV history"
    }
  ]
}
```

Loader `harness.LoadExpected(path) (*Expected, error)` in a new `src/go/internal/harness/expected.go`, `DisallowUnknownFields`, validating:
- `schema` must be `1`.
- `expected` key must be present; an empty array is valid, a missing key is invalid (mirrors the matrix's `args` rule).
- Each entry: `id` matches `^DC-\d{2}$` and is unique across entries; `cases` is a non-empty array of patterns each of which `path.Match` accepts (an `ErrBadPattern` is a load error naming the entry and pattern); `reason` is a non-empty string.
- Every error message names `expected-diffs.json`, the offending entry `id` (when known) and field.

**Matching** (`(*Expected).Match(caseID, group string) (id string, ok bool)`): entries are tried in file order and the first entry with a matching pattern wins. A pattern matches when `path.Match(pattern, caseID)` is true; a pattern containing no `/` is additionally tried against the group name (`path.Match(pattern, group)`). Case IDs are `<group>/<conf>/<env>/<io>/<tz>` for `run` and the bare step name (group `live`) for `live`, so `snapshot-*-csv` names whole groups, `sync-dry-run/*/*/*/alt` names one axis slice, `sync-dry-run` names both the matrix group and the live step of that name, and `live` names every live step. `*` in `path.Match` does not cross `/`, which is why axis-level patterns spell all five segments.

### 2. `tudiff run` and `tudiff live`: the gate rule

Both subcommands gain `--expected <path>` (default `harness/expected-diffs.json`, resolved against the repo root like `--matrix`; an explicit relative value resolves against the cwd, as the other path flags do). Preflight, in the existing order after the matrix check: an unreadable, missing or invalid file prints exactly one `tudiff: expected-diffs: <loader error>` line (missing: `tudiff: harness/expected-diffs.json not found`) and exits 2. A missing file is never treated as an empty set, matching the harness's "never a vacuous 0" posture. `--list` does not load it.

Per result, after `Compare`/`CompareTrees` and `annotateCalls`: when `Status == red`, `Result.Expected` (new `string` field) is set to the matching entry's `id` or stays `""`. Timeouts and green cases never carry an expected ID (a timeout is never an expected diff).

New summary fields (`harness.Summary`): `Expected int` (red cases with a non-empty `Expected`), `Unexpected int` (`Red - Expected`), and `Stale []string` (entry IDs that matched at least one *executed* case and none of the matched cases is red). Per-entry tallies (`matched`, `red`) are computed over the executed (post-`--filter`) results; an entry that matched zero executed cases is reported but is neither stale nor an error, because `--filter` and `live` legitimately exclude cases.

**Exit code** for both `run` and `live` (replaces `Red+Timeout > 0`):

```
exit 1  iff  Unexpected + Timeout + Unconfirmed > 0  OR  len(Stale) > 0
exit 0  otherwise
exit 2  usage/preflight, unchanged
```

`Unconfirmed` is the existing count of cases that replayed an `unconfirmed: true` fixture; making it fail the run is the P3b/R3 clause. It is 0 today.

### 3. Report format

`report.txt` (streamed to stdout as before):
- Header gains one line after `matrix:`: `expected: <path> (<n> entries)`.
- Red case line gains ` [expected DC-05]` immediately after the divergence detail and before ` [unconfirmed]` / ` [calls differ …]`.
- Summary first line becomes:

  ```
  tudiff: <N> cases — <g> green, <r> red (<e> expected, <x> unexpected), <t> timeout   (fixtures: <aliases>; <u> cases replayed unconfirmed fixtures)
  ```

  Today's output reads `452 cases — 452 green, 0 red (0 expected, 0 unexpected), 0 timeout`. The four `by …` axis lines are unchanged.
- When the file has one or more entries, one line per entry follows the axis lines: `  expected: DC-05  <red>/<matched> red` with ` (stale)` appended when matched ≥ 1 and red = 0, and ` (no executed case)` when matched = 0. With an empty file no such lines print, so the report shape for the empty set differs from today only by the header line and the summary's parenthetical.

`report.json` (`schema` stays 1; the additions are optional-additive): `header.expected` (path) and `header.expected_entries` (int); `summary.expected`, `summary.unexpected`, `summary.stale` (string array, `[]` when none); each case gains `"expected": "DC-05"` or `""`. Field order follows the Go struct order as today.

The harness report is a maintainer artifact, not one of the Goal-frozen external surfaces, so the Output Stability clause does not apply to this format change. `report_test.go` and `run_test.go` assertions on the summary line move to the new text.

### 4. CI: `.github/workflows/ci.yml`

- Rename job `go-diff` to `tudiff`. Steps unchanged in order and pinned SHAs: checkout, setup-node 20, setup-go (`go-version-file: src/go/go.mod`, `cache: false`), setup-just, `npm ci`, `just go-diff --placeholder`, `just go-live`, upload-artifact.
- Remove `continue-on-error: true` from both harness steps. The `Live harness` step now runs only when the matrix step passed (default step semantics); the upload step keeps `if: always()` so a red run still ships its report.
- The `actions/upload-artifact` step stays as-is: name `tudiff-report`, paths `bin/harness/report/` and `bin/harness/report-live/`. This is the "report uploaded as a CI artifact" requirement; it is already satisfied and is re-verified, not rewritten.
- `ci-gate`: `needs: [build-and-test, go-build-and-test, tudiff]` and a third check block in the same shape as the existing two:

  ```yaml
  if [ "${{ needs.tudiff.result }}" != "success" ]; then
    echo "tudiff did not succeed (result: ${{ needs.tudiff.result }})"
    exit 1
  fi
  ```

- Comments: the workflow header comment lists three lanes; the job comment drops the "every case is red until Phase 1 lands … R3 removes …" narrative and states the gate rule (exit 1 on unexpected red, timeout, unconfirmed replay, or stale expected entry; expected diffs come from `harness/expected-diffs.json`).
- Nothing in the branch ruleset changes: `ci-gate` remains the only required check (`scripts/ci-gate-ruleset.sh` untouched).

### 5. Justfile and README wording

- `justfile`: the `go-diff` recipe comment "Exit 1 while any case is red — the count is the port's burndown, not a gate yet." becomes a one-line statement of the gate rule and names `harness/expected-diffs.json`; the `go-live` comment's "Exit 1 while any step is red" is updated the same way. Recipe names and bodies are unchanged (`go-diff` stays the recipe name; only the CI job is renamed).
- `README.md` § CI / branch protection: the sentence naming the two lanes gains the third (`tudiff`, the differential harness against the placeholder corpus) and "both lanes" becomes "all three lanes". Before editing, check the README against the toolkit standards that govern it (constitution § Toolkit Standards: `shll standards`, read the entries naming README); only this paragraph changes.

### 6. Tests

- `src/go/internal/harness/expected_test.go`: table test over the loader (valid empty file; valid entries; each validation error: schema 2, missing `expected` key, unknown field, bad `id`, duplicate `id`, empty `cases`, bad pattern, empty `reason`) and over `Match` (group pattern, full-ID pattern, axis slice, live step name, bare `live` group, first-entry-wins).
- `src/go/internal/harness/report_test.go`: `[expected DC-NN]` placement on the case line, the new summary first line, the per-entry lines including `(stale)` and `(no executed case)`, and the JSON fields.
- `src/go/cmd/tudiff/run_test.go` (the existing stub-binary harness in the test env gains an `expected-diffs.json`): a red case matched by an entry exits 0 with the marker; an entry whose matched cases are all green exits 1 with the stale line; a missing or invalid file exits 2 with the one-line message; a case that replays an `unconfirmed: true` fixture exits 1 (the test env writes its own placeholder manifest, so it can flag one fixture). `live_test.go`: `--expected` accepted and its preflight error path.
- End-to-end after implementation: `just go-diff --placeholder` prints `452 cases — 452 green, 0 red (0 expected, 0 unexpected), 0 timeout` and exits 0; `just go-live` stays 9/9 and exits 0; `just go-test` and `just go-lint` green.

### Non-goals

- No change to `harness/matrix.json` coverage, to `docs/specs/usage.md` (the 24 markers stay unresolved; the spec's "expected diff in the differential harness (R3)" sentence remains true as written), or to any `[DECIDE]` resolution. Resolving markers is G0/G3's job.
- No darwin-arm64 CI run. Cutover criterion 1 names linux-amd64 and darwin-arm64; the R3 row scopes CI on every PR, which is `ubuntu-latest`. The darwin evidence is a one-off `just go-diff` on a Mac at G3, recorded by hand in the plan, not a macOS runner on every PR (see Open Questions).
- No `$GITHUB_STEP_SUMMARY` rendering, no PR comment bot, no change to `release.yml`.
- No change to any Goal-frozen external surface: CLI grammar, help text, output formats, exit codes of `tu`, conf files, metrics-repo layout, toolkit contracts. Only `tudiff`'s own flags, report and exit rule change.
- The plan row's Status column is updated at ship, as every prior row did (a `plan:` commit), not by this intake.

## Affected Memory

- `harness/differential-harness`: (modify) *tudiff run* requirement gains `--expected` and the new preflight; new requirement for the expected-diffs file (schema, validation, matching); *Report* requirement's header line, case-line marker, summary first line, per-entry lines and `report.json` fields; *Fixture resolution (D7) and unconfirmed flagging* loses the "what R3 later turns into a gate failure" future tense; *tudiff live* gains the same flag and exit rule and its "gating joins ci-gate at plan row R3" clause becomes present truth; a new exit-code rule (unexpected + timeout + unconfirmed + stale); Design Decisions entries for "expected diffs are a committed data file keyed by DC id" and "a missing expected-diffs file is a preflight error, not an empty set".
- `build/toolchain`: (modify) the CI paragraph: third job is `tudiff` (renamed from `go-diff`), no `continue-on-error`, in `ci-gate`'s `needs` with its own check block; the "informational … until plan R3" clauses in the description and body become the gated present; `go-diff`/`go-live` recipe comments.

## Impact

- **Go (harness only, `src/go/`)**: new `internal/harness/expected.go` + test; `internal/harness/report.go` (Summary fields, header line, case-line marker, summary rendering, JSON structs) + test; `internal/harness/diff.go` (`Result.Expected`); `cmd/tudiff/run.go` (flag, preflight, annotate, exit rule) + test; `cmd/tudiff/live.go` (flag, preflight, exit rule) + test. No change under `internal/command`, `internal/render`, `cmd/tu` or any shipped path. `src/node/` untouched (D4 freeze).
- **Data**: new `harness/expected-diffs.json` (empty set).
- **CI**: `.github/workflows/ci.yml` (job rename, gating, `ci-gate` needs and check). CI wall time is unchanged: the job already ran both harness steps; only the failure semantics change. Renaming the job changes the name of one non-required status check on PR pages; the required check `ci-gate` is unchanged and the ruleset is untouched.
- **Docs**: `README.md` § CI / branch protection (one paragraph, standards-checked), `justfile` comments, the two memory files above.
- **Risk**: gating a job that is green today. If a future PR reddens a case for a legitimate reason, the fix is either a Go correction (a real regression) or, after a `[DECIDE] drop` resolution, one entry in `harness/expected-diffs.json` plus the Go change in the same PR. Flaky risk from the date-rollover re-run and `--timeout 60s` per side is already handled by the harness (single re-run on rollover; a timeout is red and visible in the report).

## Open Questions

- Cutover criterion 1 lists darwin-arm64 alongside linux-amd64, but R2's checklist completed only on dev-ws-sahil02 (Linux) and this change keeps CI on `ubuntu-latest`. At G3, decide whether a one-off `just go-diff --placeholder` on a Mac (recorded in the plan) is the darwin evidence, or whether a macOS runner is wanted. Not blocking for R3.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Rename the CI job `go-diff` to `tudiff` and add it to `ci-gate`'s `needs` with its own check block; the branch ruleset is untouched because `ci-gate` stays the sole required check | The plan row and the invocation both name the job `tudiff`; `ci-gate` is the documented single required check (README, toolchain memory, `scripts/ci-gate-ruleset.sh`) | S:85 R:90 A:85 D:80 |
| 2 | Confident | Gate `just go-live` in the same job (remove its `continue-on-error` too), not only the matrix step | The `ci.yml` comment on that step says "R3 owns gating it"; cutover criterion 2 is live sync parity; it is 9/9 green today; the invocation names only the matrix, so this widens by one step | S:60 R:85 A:80 D:70 |
| 3 | Confident | Expected diffs live in a committed `harness/expected-diffs.json` (schema 1) beside `matrix.json`, keyed by the spec's stable `DC-NN` IDs with `cases` patterns and a `reason`, loaded by both `run` and `live` | The invocation asks that a drop decision be "one entry, not a code change"; the matrix already models "coverage is data reviewed at G0"; DC IDs are declared stable in the spec | S:75 R:80 A:80 D:70 |
| 4 | Confident | Patterns use `path.Match` against the full case ID, and a pattern without `/` is also tried against the group name; first entry in file order wins | Consistent with the existing five-segment ID contract (`*` never crosses `/`), gives group-level, axis-level and live-step-level addressing with one rule; trivially changed later since the set is empty | S:45 R:85 A:65 D:45 |
| 5 | Certain | A case that replayed an `unconfirmed: true` fixture makes the run exit 1 | P3b's row ("R3, where unconfirmed fixtures fail the gate") and the harness memory's "what R3 later turns into a gate failure" both name R3; the count is 0 today so the change is behavior-neutral | S:70 R:90 A:85 D:85 |
| 6 | Confident | Stale rule: an entry that matched at least one executed case, none red, exits 1 with a `(stale)` line; an entry matching zero executed cases is reported as `(no executed case)` and does not affect the exit code | An always-green expected entry is a misconfiguration worth failing on; `--filter` and `live` legitimately exclude cases, so zero-matched must not fail; no cross-runner coupling needed | S:35 R:85 A:70 D:55 |
| 7 | Certain | Keep the existing `actions/upload-artifact` step unchanged (name `tudiff-report`, both report dirs, `if: always()`) as the artifact requirement | Already present and correct in `ci.yml`; the invocation's requirement is satisfied by verifying it, not rewriting it | S:80 R:95 A:95 D:90 |
| 8 | Confident | CI stays linux-only (`ubuntu-latest`); darwin-arm64 matrix evidence is a G3 by-hand item, recorded as an Open Question, not a macOS runner | The R3 row scopes "CI on every PR"; a macOS lane doubles cost for a one-time criterion; the plan already treats G3 as a human read of evidence | S:50 R:70 A:60 D:60 |
| 9 | Certain | The committed expected set is empty; no `[DECIDE]` marker is resolved, no matrix or spec edit | The invocation states every marker is unresolved and unresolved means keep; resolving markers is G0/G3's job per the plan | S:95 R:90 A:95 D:95 |
| 10 | Confident | The summary first line changes to `<r> red (<e> expected, <x> unexpected)` and the header gains an `expected:` line; `report.json` additions are optional-additive at `schema: 1` | The harness report is a maintainer artifact, not a Goal-frozen surface, so Output Stability does not bind it; "unexpected" is the number G3 reads and belongs on the first line | S:55 R:90 A:80 D:60 |
| 11 | Confident | A missing or invalid `expected-diffs.json` is a preflight exit 2, never an empty set | Matches the harness's existing "never a vacuous 0" posture (placeholder manifest absent is exit 2; `--filter` matching nothing is exit 2); the file is committed so the default always exists | S:45 R:90 A:75 D:55 |
| 12 | Certain | Change type is `ci` (pinned explicitly; the description mentions `docs/specs/…`, which the keyword inference would read as `docs`) | The deliverable is a CI gate; the flat 3.0 intake gate makes the type non-load-bearing for the pipeline | S:60 R:95 A:90 D:80 |
| 13 | Confident | Update the README CI paragraph to name the third lane after checking the governing toolkit standards; no other README change | Constitution § Toolkit Standards requires the check before README edits; the paragraph enumerates the lanes, so it would become false without the edit | S:55 R:95 A:80 D:75 |

13 assumptions (5 certain, 8 confident, 0 tentative, 0 unresolved).
