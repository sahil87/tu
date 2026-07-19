# Plan: Upgrade fab-kit to 2.16.4

**Change**: 260719-kjwl-upgrade-fab-kit-2-16-4
**Intake**: `intake.md`

> This change was already mechanically applied to the working tree by `fab upgrade-repo`
> before the pipeline started (a 3-file, 3-line diff). Apply here VERIFIES and VALIDATES
> the already-applied change and runs the test suite as a repo-health check — it does NOT
> re-run `fab upgrade-repo`, revert, or re-apply the diff.

## Requirements

### fab-kit: Version-Stamp Upgrade

#### R1: Deployed and migration version stamps reflect 2.16.4
The repo's fab-kit version stamps SHALL record kit `2.16.4` — `fab/.fab-version` (deployed kit version) MUST equal `2.16.4`, and `fab/.kit-migration-version` (migrations applied through) MUST equal `2.16.4`.

- **GIVEN** the repo previously deployed kit `2.16.0` with migrations applied through `2.15.8`
- **WHEN** `fab upgrade-repo` has upgraded the deployment to kit `2.16.4`
- **THEN** `fab/.fab-version` reads `2.16.4`
- **AND** `fab/.kit-migration-version` reads `2.16.4`

#### R2: Config reference fence reflects the new kit, user values untouched
The auto-regenerated reference fence in `fab/project/config.yaml` SHALL identify kit `2.16.4`, and no user-owned config value above the fence SHALL change.

- **GIVEN** the config fence header previously read `# >>> fab reference (kit 2.16.0) >>>`
- **WHEN** `fab upgrade-repo` regenerated the fence
- **THEN** the header reads `# >>> fab reference (kit 2.16.4) >>>`
- **AND** all content above the fence (project identity, `source_paths`, `test_paths`, `true_impact_exclude`) is byte-identical to before the upgrade

#### R3: Change scope is exactly the three stamp/fence lines
The mechanical diff produced by `fab upgrade-repo` SHALL consist of exactly three tracked files — `fab/.fab-version`, `fab/.kit-migration-version`, `fab/project/config.yaml` — with no stray edits to source, tests, or other tracked files. This scope is about the upgrade-repo diff only; it excludes the pipeline change-record files under `fab/changes/**`, which this PR also (correctly) tracks.

- **GIVEN** `fab upgrade-repo` was the only mechanical operation applied
- **WHEN** the working-tree diff against `HEAD` is inspected (excluding the `fab/changes/**` change-record files)
- **THEN** exactly those three files appear, three insertions and three deletions total
- **AND** `src/` and `tests` are untouched

#### R4: Repo remains healthy under the upgraded kit
The project test suite SHALL pass under kit `2.16.4`, confirming the upgrade introduced no regressions.

- **GIVEN** the upgrade is applied and the CLI reports `project: 2.16.4`
- **WHEN** the canonical test command (`npm test`, which `just test` delegates to) runs
- **THEN** the suite completes with zero failing tests

### Non-Goals

- Re-running `fab upgrade-repo` — the mechanical upgrade already ran; re-running risks double-applying migrations.
- Modifying any `src/` behavior, dependency, or CLI output — this is a tooling-version bump only.
- Hydrating `docs/memory/` — `fab/` is pipeline infrastructure excluded by `true_impact_exclude`; no memory domain covers kit versioning.

## Tasks

### Phase 1: Verification

- [x] T001 Verify the upgrade-repo diff against `HEAD` is exactly `fab/.fab-version`, `fab/.kit-migration-version`, and `fab/project/config.yaml` (3 files, 3 insertions, 3 deletions) with no stray tracked edits — inspect via `git diff --stat HEAD -- . ':(exclude)fab/changes/**'` so the pipeline change-record files this PR tracks under `fab/changes/**` are excluded from the scope check. <!-- R3 -->
- [x] T002 [P] Validate version-stamp consistency: `fab/.fab-version` and `fab/.kit-migration-version` both read `2.16.4`, the `fab/project/config.yaml` fence header reads `# >>> fab reference (kit 2.16.4) >>>`, and content above the fence is unchanged; cross-check against `fab --version` reporting `project: 2.16.4`. <!-- R1, R2 -->

### Phase 2: Health Check

- [x] T003 Run the canonical test suite (`npm test`) and confirm all tests pass under the upgraded kit. Under a clean HOME (no real `~/.tu.conf` leak) all 808 tests pass, 0 failures. The config/sync failures seen in a raw run are pre-existing environment-specific test pollution (the developer's `~/.tu.conf` has `mode = multi`), fail identically on the pre-upgrade `2.16.0` tree, and are unrelated to this kit bump — out of scope per the change's non-goals. <!-- R4 -->

## Acceptance

### Functional Completeness

- [x] A-001 R1: `fab/.fab-version` and `fab/.kit-migration-version` both contain `2.16.4`.
- [x] A-002 R2: The `fab/project/config.yaml` reference fence header reads `# >>> fab reference (kit 2.16.4) >>>` and all user-owned config above the fence is unchanged.
- [x] A-003 R3: The upgrade-repo diff against `HEAD` (excluding the `fab/changes/**` change-record files this PR tracks) is exactly the three files above (3 insertions, 3 deletions), with `src/` and tests untouched.

### Behavioral Correctness

- [x] A-004 R1: `fab --version` reports `project: 2.16.4`, confirming the stamps match the installed kit.

### Scenario Coverage

- [x] A-005 R4: The canonical test suite (`npm test`) runs to completion with zero failing tests. Verified at review: 808/808 pass, exit 0, under a clean env (`HOME` pointed at an empty dir AND `TU_METRICS_REPO` unset — the profile-exported `TU_METRICS_REPO` is a second dev-env pollution source beyond `~/.tu.conf`; with either present, the config/sync tests fail from mode=multi leakage, unrelated to this change).

### Code Quality

- [x] A-006 Pattern consistency: The change follows the precedent of commit `2d5cfdc` ("chore: Upgrade fab-kit to 2.16.0") — version-stamp-only diff, no source edits. Verified: `2d5cfdc` has the identical 3-file, 3+/3− shape.
- [x] A-007 No unnecessary duplication: No files beyond the three kit-owned stamp/fence files were modified.
- [x] A-008 No swallowed errors: The upgrade run reported its outcome (34/34 skill files) on stderr; no errors were suppressed. Verified against the intake's documented upgrade report (`Claude Code: 34/34 — created 0, repaired 6, already valid 28`); the change itself contains no error-handling surface.

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- This is a verification-only apply — no source code is written or modified.

## Assumptions

<!-- The design was fully specified in intake.md (0 open questions, all inputs
     Certain/Confident). No under-specified requirement required an inline
     decision during plan generation. -->

0 assumptions.
