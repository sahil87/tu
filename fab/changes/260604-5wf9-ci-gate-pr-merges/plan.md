# Plan: CI Gate for PR Merges

**Change**: 260604-5wf9-ci-gate-pr-merges
**Status**: In Progress
**Intake**: `intake.md`

## Requirements

### CI: Pull-Request Triggering

#### R1: PR-triggered CI on `main`
The `ci.yml` workflow MUST trigger on pull requests targeting `main` IN ADDITION TO the existing `push` to `main`. The existing `push: branches: [main]` trigger MUST be retained.

- **GIVEN** a pull request is opened or updated against `main`
- **WHEN** the `ci.yml` workflow evaluates its `on:` triggers
- **THEN** the workflow runs in the pull-request context (`build-and-test` executes)
- **AND** a push to `main` (e.g. a release-merge commit) still triggers the same workflow

#### R2: `build-and-test` job steps unchanged
The `build-and-test` job's step list (checkout → setup-node@\<pinned-SHA\> Node 20 → `npm ci` → `npm run build` → `npm test`) MUST remain unchanged. Action SHAs MUST stay pinned to the values shared with `release.yml`.

- **GIVEN** the `build-and-test` job
- **WHEN** the workflow is edited to add the PR trigger and gate job
- **THEN** the job's steps and pinned action SHAs are byte-for-byte identical to before, only now also running on PRs

### CI: Aggregating Gate Job

#### R3: `ci-gate` aggregating job
The workflow MUST define a `ci-gate` job that `needs: [build-and-test]`, runs `if: always()` on `ubuntu-latest` (no matrix), and exits non-zero unless `needs.build-and-test.result == "success"`. This provides one stable required-check name.

- **GIVEN** `build-and-test` completes with result `success`
- **WHEN** `ci-gate` runs
- **THEN** `ci-gate` prints "All required CI jobs passed." and exits 0
- **AND GIVEN** `build-and-test` fails or is cancelled
- **WHEN** `ci-gate` runs (it still runs because `if: always()`)
- **THEN** `ci-gate` prints the non-success result and exits 1 (definitive failure, not skipped)

### Repo Config: Required-Check Enforcement

#### R4: Idempotent, gracefully-degrading ruleset helper
A committed, executable helper script `scripts/ci-gate-ruleset.sh` MUST encapsulate the exact `gh api` rulesets mutation that requires `ci-gate` on `main` for `sahil87/tu`. It MUST be idempotent (update an existing ruleset of the same name rather than create a duplicate), MUST degrade gracefully when `gh` is missing/unauthenticated/lacks admin scope (report the manual steps, exit 0 — never crash, per Constitution II), and MUST default to a safe posture (dry-run by default; the live mutation requires an explicit `--apply` flag). The script MUST pass `bash -n`.

- **GIVEN** an admin runs `scripts/ci-gate-ruleset.sh --apply` with a token that has admin scope
- **WHEN** no ruleset named "Require CI gate on main" exists
- **THEN** the script creates it via `POST repos/sahil87/tu/rulesets`
- **AND GIVEN** a ruleset of that name already exists
- **WHEN** the script runs with `--apply`
- **THEN** it updates the existing ruleset (`PUT .../rulesets/{id}`) rather than creating a duplicate
- **AND GIVEN** `gh` is absent or lacks admin scope
- **WHEN** the script runs
- **THEN** it prints the manual ruleset instructions and exits 0 without crashing

#### R5: Pipeline does not mutate live repo settings unattended
The apply step MUST NOT execute the live `gh api` ruleset mutation against `sahil87/tu`. Enforcement MUST be captured in the reviewable helper script and documentation for an admin to run, rather than being applied silently inside the automated pipeline.

- **GIVEN** the apply stage runs the tasks for this change
- **WHEN** the ruleset enforcement task executes
- **THEN** no live GitHub repo-settings mutation is issued by the pipeline
- **AND** the ready-to-run command + payload exist in `scripts/ci-gate-ruleset.sh` for an admin to invoke

### Docs: Gate Documentation

#### R6: Document the gate
The change MUST document that `main` is gated by the `ci-gate` required check: how to reproduce CI locally (`npm ci && npm run build && npm test`, or `just test`), and how an admin applies/adjusts the ruleset (pointing at `scripts/ci-gate-ruleset.sh`). The workflow header comment MUST note the PR-gating behavior; a concise README section SHOULD describe local reproduction and admin enforcement.

- **GIVEN** a contributor or admin reads the project docs / workflow
- **WHEN** they look for how `main` is protected
- **THEN** they find that `ci-gate` is the required check, the local reproduce command, and a pointer to `scripts/ci-gate-ruleset.sh`

### Non-Goals

- Modifying `release.yml` — release/tag/Homebrew side effects stay isolated there (the retained `push: [main]` trigger still fires on the release-merge commit).
- Fixing known test-suite hermeticity flakes (`TU_*` env leakage, `cli-sync` git fixtures, `rain` flake) — tracked separately. The gate surfacing flakiness is the gate working as intended.
- Any `src/` source-code changes — this is workflow + repo-config + helper-script only.
- Multi-version / multi-OS CI matrix — single `ubuntu-latest` + Node 20 lane (intake assumption #8).

### Design Decisions

1. **Enforcement is captured in a committed helper script, not auto-applied during apply** — *Why*: mutating live GitHub repo settings is an outward-facing, hard-to-reverse side effect; running it unattended inside an automated pipeline that the user is reviewing is unsafe. A committed, idempotent, dry-run-default script makes enforcement reproducible and reviewable while leaving the actual mutation for an authorized admin to run. — *Rejected*: auto-running `gh api` during apply (the intake's literal wording), because it silently changes repo state the user has not authorized at apply time. See Assumption #3.
2. **Single `ci-gate` aggregating job as the required check** — *Why*: gives the ruleset one stable check name that survives renaming/splitting individual jobs. — *Rejected*: requiring `build-and-test` directly, which couples the rule to an internal job name.
3. **`if: always()` on `ci-gate`** — *Why*: ensures the required check resolves to a definitive pass/fail even when `build-and-test` fails or is cancelled, rather than being skipped (a skipped required check can block or hang a PR). — *Rejected*: default `if` (success-only), which would skip the gate on upstream failure.

## Tasks

### Phase 1: Workflow

- [x] T001 Edit `.github/workflows/ci.yml`: add `pull_request: branches: [main]` alongside the existing `push: branches: [main]` trigger (keep push); update the leading header comment to note the workflow now gates PRs in addition to verifying pushes to main. Leave the `build-and-test` job steps and pinned SHAs unchanged. <!-- R1 R2 -->
- [x] T002 Edit `.github/workflows/ci.yml`: append the `ci-gate` job (`needs: [build-and-test]`, `if: always()`, `runs-on: ubuntu-latest`, verify-step that exits 1 unless `needs.build-and-test.result == "success"`). <!-- R3 -->
- [x] T003 Validate `.github/workflows/ci.yml` parses as YAML (`python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml'))"`). <!-- R1 R2 R3 -->

### Phase 2: Enforcement Helper

- [x] T004 Create executable `scripts/ci-gate-ruleset.sh`: ready-to-run idempotent ruleset mutation (detect existing ruleset by name → update via `PUT`, else create via `POST`) for context `ci-gate` on `refs/heads/main` of `sahil87/tu`; graceful degradation when `gh` is missing/unauthenticated/lacks admin scope (print manual steps, exit 0); dry-run default with explicit `--apply` to mutate; usage/`-h`; header comment documenting that it mutates live repo settings and needs admin scope. Match existing `scripts/*.sh` style (`#!/usr/bin/env bash`, `set -euo pipefail`, `usage()`). <!-- R4 R5 -->
- [x] T005 Validate `scripts/ci-gate-ruleset.sh` passes `bash -n` and run it in default (dry-run) mode to confirm it does not mutate and does not crash. <!-- R4 R5 -->

### Phase 3: Documentation

- [x] T006 Document the gate: add a concise "CI / branch protection" section to `README.md` (required check `ci-gate`, local reproduce `npm ci && npm run build && npm test` or `just test`, admin enforcement via `scripts/ci-gate-ruleset.sh`). The workflow header comment from T001 already covers the in-workflow note. <!-- R6 -->

### Phase 4: Verification

- [x] T007 Run `npm run build` and `npm test` locally to confirm this change broke nothing; note any pre-existing/unrelated test failures as pre-existing (this change touches no `src/`). Do NOT run the live `gh api` mutation. <!-- R2 R5 -->

## Execution Order

- T001 → T002 → T003 (same file, sequential; validate after both edits)
- T004 → T005 (script then validate)
- T006 depends on T001/T004 existing (references them)
- T007 last (verification)

## Acceptance

### Functional Completeness

- [x] A-001 R1: `ci.yml` `on:` block contains both `pull_request: branches: [main]` and `push: branches: [main]`. (Verified ci.yml:14-18.)
- [x] A-002 R2: `build-and-test` step list (checkout, setup-node Node 20, `npm ci`, `npm run build`, `npm test`) and pinned SHAs are unchanged from the pre-change version. (Verified ci.yml:26-41; SHAs `checkout@34e1148…`/`setup-node@49933ea…` byte-identical to release.yml:53,57,127,132.)
- [x] A-003 R3: A `ci-gate` job exists with `needs: [build-and-test]`, `if: always()`, `runs-on: ubuntu-latest`, and a step that exits non-zero unless `needs.build-and-test.result == "success"`. (Verified ci.yml:47-58.)
- [x] A-004 R4: `scripts/ci-gate-ruleset.sh` exists, is executable, passes `bash -n`, contains the `required_status_checks` payload for context `ci-gate` on `refs/heads/main`, and has idempotency (update-existing) + graceful-degradation logic. (Verified: on-disk mode `-rwxrwxr-x`, `bash -n` OK, payload at lines 69-85, idempotency 136-165, degradation 121-144.)
- [x] A-005 R5: No live `gh api` ruleset mutation was issued during apply; the mutation is gated behind an explicit `--apply` flag in the committed script (default dry-run). (Verified: `APPLY=0` default, dry-run early-exit at ci-gate-ruleset.sh:110-118; apply path only on `--apply`.)
- [x] A-006 R6: Documentation (README section + workflow header comment) describes the `ci-gate` required check, local CI reproduction, and admin enforcement via `scripts/ci-gate-ruleset.sh`. (Verified README:78-104 + ci.yml header:3-13.)

### Behavioral Correctness

- [x] A-007 R3: With `if: always()`, `ci-gate` reports a definitive failure (exit 1) when `build-and-test` fails/cancels rather than being skipped. (Verified ci.yml:49,54-56: `if: always()` + `!= "success"` → exit 1; cancelled/skipped/failure all ≠ success → gate fails.)
- [x] A-008 R4: Running `scripts/ci-gate-ruleset.sh` without `--apply` performs no mutation; running with `--apply` but no admin scope / no `gh` prints manual instructions and exits 0 (no crash, Constitution II). (Verified dry-run path ci-gate-ruleset.sh:110-118; missing-gh 121-125, unauth 127-131, list-fail 140-144, PUT/POST-fail 151-154/161-164 all `exit 0` after manual_instructions. `set -e` interactions tested clean.)

### Scenario Coverage

- [x] A-009 R1: A push to `main` still triggers `ci.yml` (post-merge/release-merge verification retained). (Verified `push: branches: [main]` retained at ci.yml:17-18.)

### Code Quality

- [x] A-010 Pattern consistency: `scripts/ci-gate-ruleset.sh` follows existing `scripts/*.sh` conventions (`#!/usr/bin/env bash`, `set -euo pipefail`, `usage()`, header comment). (Verified: matches build.sh/release.sh/release-notes.sh headers; has `usage()`.)
- [x] A-011 No unnecessary duplication: the ruleset payload/command is defined once in the helper script and referenced (not duplicated) by docs. (Verified: PAYLOAD single source ci-gate-ruleset.sh:69-85; README points to the script rather than re-embedding the payload.)
- [x] A-012 No swallowed errors: the helper warns on stderr / prints actionable guidance on degraded paths rather than failing silently (Constitution II, code-quality anti-pattern). (Verified: every degraded path emits `WARN: … >&2` + `manual_instructions`.)

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`
- **Review fix (should-fix, applied 2026-06-04)**: `justfile` `test` recipe used a stale `'src/node/**/__tests__/*.test.ts'` glob that does not resolve under Node 20 (the same reason `package.json`'s `test` script was changed to a `find`-based runner in 4100495). Since the README documents `just test` as a CI-equivalent local-reproduce path, the recipe now delegates to `npm test` so the two are byte-identical to what `ci-gate` enforces. Verified: `just test` collects 570 tests / 114 suites, 570 pass (env-isolated to avoid the unrelated pre-existing `TU_*` leak). Touches `justfile` only — no `src/` impact.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Add `pull_request: [main]`, keep existing `push: [main]`; `build-and-test` steps/SHAs unchanged | Intake assumptions #1/#4 (Certain) — explicit user choice | S:98 R:85 A:90 D:95 |
| 2 | Certain | Add `ci-gate` aggregating job (`needs` build-and-test, `if: always()`, single ubuntu-latest lane) | Intake assumptions #2/#8 (Certain) — explicit, standard pattern | S:95 R:80 A:88 D:90 |
| 3 | Confident | Capture the ruleset mutation in a committed, dry-run-default, idempotent `scripts/ci-gate-ruleset.sh` (admin runs with `--apply`) INSTEAD of auto-running `gh api` live during apply | Deviation from intake's literal "auto-apply during apply": mutating live GitHub repo settings unattended is an outward-facing, hard-to-reverse side effect the reviewing user has not authorized at apply time; a reviewable, reproducible script preserves the intent (enforce ci-gate) while keeping the actual mutation in the user's hands. Dispatch instructions directed this approach explicitly. | S:90 R:55 A:80 D:80 |
| 4 | Certain | Idempotent (update-existing-by-name) + graceful degradation without admin scope in the helper | Intake assumptions #6/#7 (Certain) + Constitution II | S:95 R:65 A:80 D:80 |
| 5 | Confident | Document the gate in a new README "CI / branch protection" section plus the workflow header comment | Intake §4 allows "workflow comment and/or brief docs section"; README is the natural, existing docs surface; low blast radius | S:85 R:90 A:85 D:80 |
| 6 | Certain | Do not modify `release.yml`; no `src/` changes | Intake Non-Goals / Impact (Certain) — explicit | S:98 R:85 A:92 D:95 |

6 assumptions (4 certain, 2 confident, 0 tentative).
