# Plan: Build-time help-dump → shll.ai Command reference

**Change**: 260602-v76l-help-dump-shll-ai
**Status**: In Progress
**Intake**: `intake.md`

## Requirements

<!-- Derived from intake.md (confidence 4.4/5.0; 17 resolved assumptions). RFC-2119
     statements with GIVEN/WHEN/THEN scenarios and stable R# IDs. Under-specified
     points are resolved inline and recorded as graded SRAD rows in ## Assumptions. -->

### Producer: Help-dump contract document

#### R1: Pure contract builder
The producer SHALL expose a pure function `buildHelpDoc({name, version, description, helpText})` that returns the frozen contract object `{tool, version, captured_at, schema_version, root}` with `root = {name, path, short, usage, text, commands}`, with no I/O side effects, so it can be unit-tested directly from a captured help string. Field derivations: `tool` = `name` (falling back to `"tu"`); `version` = `version`; `captured_at` = `new Date().toISOString()`; `schema_version` = literal `1`; `root.name` = `root.path` = `"tu"`; `root.short` = `description` (falling back to the first non-empty line of `helpText`); `root.usage` = the `Usage:` line extracted from `helpText`; `root.text` = `helpText` unmodified (byte-for-byte); `root.commands` = `[]`.

- **GIVEN** `name="tu"`, `version="0.4.14"`, `description="AI coding assistant cost tracking CLI"`, and `helpText` beginning with `"Usage: tu [source] [period] [display]\n..."`
- **WHEN** `buildHelpDoc` is called with those inputs
- **THEN** it returns an object where `tool === "tu"`, `version === "0.4.14"`, `schema_version === 1`, `root.name === "tu"`, `root.path === "tu"`, `root.short === "AI coding assistant cost tracking CLI"`, `root.usage === "Usage: tu [source] [period] [display]"`, `root.text === helpText` (byte-identical), and `root.commands` deep-equals `[]`
- **AND** `captured_at` is a `Z`-suffixed ISO-8601 UTC string parseable by `Date.parse`

#### R2: Byte-for-byte help capture from the built binary
The producer SHALL capture the CLI help by executing the **built** bundle `node dist/tu.mjs --help`, capturing stdout exactly with no trimming, re-wrapping, CRLF conversion, or ANSI injection, and SHALL force color off by setting `NO_COLOR=1` in the child process environment.

- **GIVEN** a built `dist/tu.mjs` exists
- **WHEN** the producer runs the child process `node dist/tu.mjs --help` with `NO_COLOR=1` in its env
- **THEN** the captured stdout is used verbatim as `root.text`, preserving all newlines and containing no ANSI escape sequences

#### R3: Runtime version/metadata read from package.json
The producer SHALL read `version`, `name`, and `description` from `package.json` at runtime (not a hard-coded constant), so the emitted `version` always matches the package being built.

- **GIVEN** `package.json` declares `version: "0.4.14"`, `name: "tu"`, `description: "AI coding assistant cost tracking CLI"`
- **WHEN** the producer assembles the contract
- **THEN** the emitted `version` is `"0.4.14"` and `tool` is `"tu"`, sourced from `package.json`

#### R4: Emit pretty-printed help/tu.json
The producer SHALL write the contract object to `help/tu.json` (repo-relative), pretty-printed (2-space indent) with a trailing newline.

- **GIVEN** the contract object has been assembled
- **WHEN** the producer writes the artifact
- **THEN** `help/tu.json` exists, is valid pretty-printed JSON, and `JSON.parse` of its contents round-trips to the contract object

#### R5: Self-validation and fail-loud
The producer SHALL re-read the written `help/tu.json`, `JSON.parse` it, and assert the required keys are present and well-typed (`tool` non-empty string, `version` non-empty string, `captured_at` non-empty string, `schema_version === 1`, `root` object with non-empty string `text`, string `name`/`path`/`short`/`usage`, and array `commands`). The producer SHALL exit with a non-zero status if the CLI errors, stdout is empty, or any validation assertion fails — it MUST NOT degrade or emit a partial artifact.

- **GIVEN** the CLI errors, emits empty stdout, or the written JSON fails any validation assertion
- **WHEN** the producer runs
- **THEN** it exits non-zero with a diagnostic on stderr and does not silently continue
- **AND GIVEN** a valid capture and a well-formed written file, **THEN** it exits zero

#### R6: npm run help-dump entry
`package.json` SHALL expose an `npm run help-dump` script that invokes the producer with the production runtime (`node scripts/help-dump.mjs`), requiring no test/dev tooling (no tsx) on the production path.

- **GIVEN** a built `dist/tu.mjs`
- **WHEN** a developer or CI runs `npm run help-dump`
- **THEN** the producer runs under plain `node` and produces a valid `help/tu.json`

### Repo hygiene: transient artifact

#### R7: help/ is gitignored
`help/tu.json` is a transient CI artifact (shll.ai is the source of truth) and SHALL NOT be committed to the tu repo. `.gitignore` SHALL ignore `help/`.

- **GIVEN** the producer has written `help/tu.json`
- **WHEN** `git status` is inspected
- **THEN** `help/tu.json` is untracked/ignored and does not appear as a staged or modifiable change

### CI: build-verification workflow

#### R8: New ci.yml on push to main
A new workflow `.github/workflows/ci.yml` SHALL run build verification + tests on `push` to `main`: checkout, `actions/setup-node` (node 20), install deps, `npm run build`, `npm test`. It SHALL NOT run the help-dump. Action SHAs SHALL be pinned to the exact same SHAs already used in `release.yml` (`actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5 # v4`, `actions/setup-node@49933ea5288caeca8642d1e84afbd3f7d6820020 # v4`).

- **GIVEN** a commit is pushed to `main`
- **WHEN** `ci.yml` runs
- **THEN** it checks out, sets up node 20, installs, builds the bundle, and runs the test suite — and contains no help-dump or shll.ai step
- **AND** its `actions/checkout` and `actions/setup-node` SHAs match those in `release.yml` byte-for-byte

### CI: release pipeline help-dump → shll.ai

#### R9: Help-dump → shll.ai PR with auto-merge on the tag-push path
`.github/workflows/release.yml` SHALL, on the existing `v*` tag-push path and AFTER the build, run the producer, **fail the job if validation fails**, then clone `sahil87/shll.ai` (authenticated via `secrets.SHLLAI_TOKEN`), write the emitted `help/tu.json` there on a fresh branch `tu-help-dump-v{version}`, commit, push, open a PR, and enable auto-merge (`gh pr create` then `gh pr merge --auto --squash`). It SHALL NOT push directly to shll.ai `main`. The `GH_TOKEN` for these `gh` calls SHALL be `secrets.SHLLAI_TOKEN` (the default `GITHUB_TOKEN` cannot cross repos).

- **GIVEN** a `v*` tag push triggers `release.yml` and the build has produced `dist/tu.mjs`
- **WHEN** the help-dump step runs
- **THEN** the producer emits and self-validates `help/tu.json`, and on success the workflow creates a branch `tu-help-dump-v{version}` in `sahil87/shll.ai`, commits the file at `help/tu.json`, pushes, opens a PR, and enables `--auto --squash` merge — using `SHLLAI_TOKEN` for both git auth and `gh`
- **AND GIVEN** producer validation fails, **THEN** the job fails and no PR is opened

#### R10: Release-PR-merge → release-pipeline entry point
`.github/workflows/release.yml` SHALL add an `on: push: branches: [main]` trigger plus a guarded `tag-on-release-merge` job that detects a release (a version-bump / release-labeled merge) on `main` and creates+pushes the `v{version}` tag (the durable version anchor the Homebrew tap and history depend on). That job SHALL expose an `is_release` output (plus the resolved `tag`/`version`); the `release` job SHALL depend on it (`needs: tag-on-release-merge`) and run **in the same workflow run** when `is_release == 'true'`, so the full pipeline executes without relying on the tag push re-triggering anything. (A tag pushed with the default `GITHUB_TOKEN` does NOT re-trigger a `release` run — GitHub suppresses workflow runs from `GITHUB_TOKEN`-authored pushes; the `needs`/outputs dependency is what drives the pipeline.) The `release` job's `if` SHALL also still fire on a real `v*` tag push and on `workflow_dispatch`, using `always()` so a skipped `tag-on-release-merge` (the tag-push / dispatch paths) does not block it. The mechanism SHALL be minimal and well-commented. Releases remain tag-anchored.

- **GIVEN** a release PR is merged to `main` and the release condition holds
- **WHEN** the `tag-on-release-merge` job runs
- **THEN** it creates and pushes a `v{version}` tag and sets `is_release=true`, and the `release` job (via `needs`/outputs) runs in the same workflow run, executing the full pipeline (release notes, GitHub release, Homebrew tap, help-dump→shll.ai) — exactly once, not relying on a `GITHUB_TOKEN` tag re-trigger
- **AND GIVEN** a non-release commit to `main`, **THEN** the guarded job sets `is_release=false`, does not tag, and the `release` job is skipped (no heavy steps run)

### Non-Goals

- Subcommand recursion in the producer — tu prints no per-subcommand `--help` pages, so `root.commands` stays `[]`; a clearly-marked flat document is correct (intake Q2). Forward extension point only; no speculative recursion is built.
- Committing `help/tu.json` to the tu repo — explicitly transient (intake Q5).
- Provisioning `SHLLAI_TOKEN` or shll.ai branch/auto-merge settings — pre-provisioned, external, out of scope (intake assumption #10).
- Any change to the runtime CLI, its output, flags, or grammar — purely additive build/release tooling.
- A `pull_request` trigger on `ci.yml` — the intake specifies push-to-`main`; PR triggers are not added (no test requires them).

### Design Decisions

1. **Producer language `.mjs` (not `.ts`)**: a standalone build script run by CI under plain `node`, analogous to `scripts/*.sh`. — *Why*: avoids a `tsx` dependency on the production/CI capture path (the constitution forbids new runtime deps; keeping the producer dep-free under `node:` built-ins is the cleanest fit); `node:`-prefixed imports + functional style still honor the constitution. — *Rejected*: `scripts/help-dump.ts` via tsx (viable — repo runs tsx for tests — but adds a tsx invocation to the release-critical path for no benefit).
2. **Pure function extracted, tested directly**: `buildHelpDoc()` is exported from `scripts/help-dump.mjs`; a co-located TS test `src/node/core/__tests__/help-dump.test.ts` imports it and calls it with a captured help string. — *Why*: the existing test runner glob is `src/node/**/__tests__/*.test.ts` and tsx can import a `.mjs` from a `.ts` test (verified by spike); this keeps the test fast (no build/exec needed), DRY (single source of the assembly logic), and inside the constitution's mandated co-located `__tests__/` location. — *Rejected*: duplicating the doc-assembly in the test (drift risk); a `scripts/`-level test (outside the existing glob, would need a new test command).
3. **Release-PR→tag detection via a `release`-labeled merged PR**: the `main`-push guarded job queries the PR associated with the merge commit (`gh pr list --search <sha>`) and tags only when that PR carried a `release` label. — *Why*: explicit, low-false-positive, and decoupled from commit-message parsing; keeps releases tag-anchored while making PR-merge the entry point. — *Rejected*: version-bump commit-message sniffing (brittle); tagging on every `main` push (would tag non-releases).

## Tasks

### Phase 1: Setup

- [x] T001 Add `help/` to `.gitignore` (append a commented entry; file already exists) <!-- R7 -->
- [x] T002 Add `"help-dump": "node scripts/help-dump.mjs"` to `scripts` in `package.json` <!-- R6 -->

### Phase 2: Core Implementation (producer)

- [x] T003 Create `scripts/help-dump.mjs`: export pure `buildHelpDoc({name, version, description, helpText})` building the frozen contract object per the field derivations (`node:` imports only, functional style) <!-- R1 -->
- [x] T004 In `scripts/help-dump.mjs`, add `extractUsage(helpText)` + `main()`: read `package.json` (version/name/description), exec `node dist/tu.mjs --help` with `NO_COLOR=1`, fail non-zero on CLI error/empty stdout, assemble via `buildHelpDoc`, write `help/tu.json` pretty-printed with trailing newline <!-- R2 R3 R4 -->
- [x] T005 In `scripts/help-dump.mjs` `main()`, add self-validation: re-read `help/tu.json`, `JSON.parse`, assert required keys/types, exit non-zero on any failure; wire `main()` to run only when invoked as the entry module (not on import) <!-- R5 -->

### Phase 3: Tests

- [x] T006 [P] Create `src/node/core/__tests__/help-dump.test.ts`: import `buildHelpDoc` from the producer, assert against a captured help string — `schema_version === 1`, `tool === "tu"`, `version` matches a passed value, `root.text` contains `"Usage: tu"` and is byte-identical to input, `root.usage === "Usage: tu [source] [period] [display]"`, `root.commands` deep-equals `[]`, `captured_at` is a parseable `Z`-suffixed ISO string <!-- R1 R5 -->

### Phase 4: CI workflows

- [x] T007 [P] Create `.github/workflows/ci.yml`: `on: push: branches: [main]`; checkout + setup-node@20 (SHAs reused from release.yml), `npm ci`, `npm run build`, `npm test`; no help-dump <!-- R8 -->
- [x] T008 Modify `.github/workflows/release.yml`: add the help-dump→shll.ai step on the tag-push path (run producer → fail on validation → clone shll.ai via `SHLLAI_TOKEN` → branch `tu-help-dump-v{version}` → commit/push → `gh pr create` + `gh pr merge --auto --squash`, `GH_TOKEN=SHLLAI_TOKEN`). Build step must precede it. <!-- R9 -->
- [x] T009 Modify `.github/workflows/release.yml`: add `on: push: branches: [main]` + a guarded `tag-on-release-merge` job that detects a `release`-labeled merged PR for the merge commit and creates+pushes `v{version}` (well-commented) <!-- R10 --> <!-- rework: a tag pushed with the default GITHUB_TOKEN does NOT re-trigger the tag-push `release` job — GitHub suppresses workflow runs from GITHUB_TOKEN-authored pushes (documented loop-prevention). A release-PR merge silently produced a tag but NO GitHub Release, NO Homebrew tap update, and NO help-dump→shll.ai PR. Re-wire so the release pipeline actually runs on a release-merge without relying on the suppressed tag re-trigger. -->

### Phase 5: Verification

- [x] T010 Run `npm run build` then `npm run help-dump`; confirm `help/tu.json` is produced, valid, and gitignored. Run `npm test` (full suite passes). Validate both workflow YAMLs parse via `python3 -c "import yaml; yaml.safe_load(open(...))"`. <!-- R1 R2 R4 R5 R8 R9 R10 --> <!-- rework: re-verify after the R10/release.yml trigger fix -->

## Execution Order

- T001, T002 (setup) independent; do first.
- T003 → T004 → T005 are sequential (same file, building up the producer).
- T006 depends on T003 (imports the pure function); T007 independent of producer.
- T008, T009 both edit `release.yml` — sequential, not `[P]`.
- T010 last (depends on producer + workflows existing).

## Acceptance

### Functional Completeness

- [x] A-001 R1: `buildHelpDoc` returns the frozen `{tool, version, captured_at, schema_version, root{name,path,short,usage,text,commands}}` shape with correct field derivations, proven by `src/node/core/__tests__/help-dump.test.ts`
- [x] A-002 R2: The producer captures help by executing `node dist/tu.mjs --help` with `NO_COLOR=1`, byte-for-byte (no trim/re-wrap/CRLF/ANSI) — verified by reading the producer and confirming the generated `root.text` matches `dist/tu.mjs --help` output
- [x] A-003 R3: `version`/`name`/`description` are read from `package.json` at runtime; generated `help/tu.json` `version` equals `package.json` version (`0.4.14`)
- [x] A-004 R4: The producer writes pretty-printed `help/tu.json` (2-space, trailing newline) that round-trips through `JSON.parse`
- [x] A-005 R5: The producer self-validates the written file and exits non-zero on CLI error / empty stdout / validation failure; exits zero on success
- [x] A-006 R6: `npm run help-dump` runs the producer under plain `node` and emits a valid artifact
- [x] A-007 R7: `.gitignore` ignores `help/`; `help/tu.json` does not appear as a tracked change
- [x] A-008 R8: `.github/workflows/ci.yml` exists, triggers on push to `main`, runs build + test on node 20, carries no help-dump, and reuses release.yml's pinned action SHAs
- [x] A-009 R9: `release.yml` tag-push path runs the producer after build, fails on validation failure, and on success creates a shll.ai branch + PR with `--auto --squash` using `SHLLAI_TOKEN` for git and `gh`; never direct-pushes to shll.ai main
- [x] A-010 R10: `release.yml` has an `on: push: branches: [main]` trigger and a guarded job that, on a detected release merge, actually drives the full release pipeline (tag + release + tap + shll.ai PR) — NOT merely creating a tag that fails to re-trigger anything. Verified by tracing all four paths: on a release-merge, `tag-on-release-merge` sets `is_release=true` and the `release` job runs IN THE SAME RUN via `needs.tag-on-release-merge.result=='success' && ...outputs.is_release=='true'` (clause 3), checking out the just-created tag — it does not depend on the suppressed GITHUB_TOKEN tag re-trigger. Non-release main push: clause 3 false, `release` skipped.

### Behavioral Correctness

- [x] A-011 R2: Captured `root.text` contains `"Usage: tu"` and no ANSI escape sequences (NO_COLOR enforced)
- [x] A-012 R9: The shll.ai write path is PR + auto-merge only — no `git push` to shll.ai `main` anywhere in the step

### Scenario Coverage

- [x] A-013 R1/R5: The co-located test exercises the pure builder (and its required keys) and passes under `npm test`
- [x] A-014 R1/R2/R4/R5: End-to-end local run (`npm run build` → `npm run help-dump`) produces a valid, self-validated `help/tu.json` (first ~15 lines inspected)

### Edge Cases & Error Handling

- [x] A-015 R5: Empty CLI stdout or a malformed written file causes a non-zero exit (fail-loud), not a partial/degraded artifact
- [x] A-016 R8/R9/R10: Both `ci.yml` and `release.yml` parse as valid YAML

### Code Quality

- [x] A-017 Pattern consistency: New code follows surrounding patterns — producer uses `node:`-prefixed imports and functional style (no classes); workflow steps mirror release.yml's pinning/secret idioms; test mirrors existing `src/node/core/__tests__/*.test.ts` style (`node:test` + `node:assert/strict`)
- [x] A-018 No unnecessary duplication: the doc-assembly logic exists in exactly one place (`buildHelpDoc`), reused by both `main()` and the test (no copy in the test)
- [x] A-019 Readability/maintainability over cleverness (code-quality.md): producer functions are small and single-purpose; no god functions
- [x] A-020 No magic strings without intent: contract literals (`"tu"`, `schema_version: 1`, branch-name template) are clearly derived/commented; no silent error swallowing — the producer warns on stderr and exits non-zero

### Constitutional Alignment

- [x] A-021 III (Single-Bundle / no runtime deps): the producer adds no new dependency and uses only `node:` built-ins; the production path (`npm run help-dump`) needs no tsx
- [x] A-022 II (Graceful Degradation is runtime-only): the build artifact producer correctly fails loud rather than degrading (intake assumption #8)

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Producer is `scripts/help-dump.mjs` (ESM `.mjs`, `node:` built-ins only, functional), NOT `.ts` | Standalone CI build script analogous to `scripts/*.sh`; avoids adding tsx to the release-critical capture path; intake explicitly offers `.mjs` and the task directs "prefer `.mjs`". Constitution honored via `node:` imports + functional style | S:90 R:80 A:90 D:88 |
| 2 | Certain | The pure `buildHelpDoc` is exported from the `.mjs` and tested directly via a co-located TS test (`src/node/core/__tests__/help-dump.test.ts`) that imports the `.mjs` under tsx | Verified by spike that tsx can import a `.mjs` from a `.ts` test and that the existing `src/node/**/__tests__/*.test.ts` glob picks it up; keeps test fast (no build/exec), DRY (one source of assembly), and inside the constitution's co-located `__tests__/` mandate | S:88 R:82 A:88 D:85 |
| 3 | Confident | Release-PR detection + same-run pipeline: the `main`-push `tag-on-release-merge` job sets `is_release=true` only when the merge commit's associated PR carried a `release` label (`gh pr list --state merged --search <sha>`). The release PR is assumed to have **already bumped `package.json`**, so the job tags the existing `v{package.json.version}` (idempotently — skips if the tag exists) as the durable version anchor. **The release pipeline then runs in the SAME workflow run** via `release`'s `needs: tag-on-release-merge` + `is_release` output — NOT via the tag push re-triggering the `release` job. **[Corrected during rework cycle 1]** The original design relied on the pushed tag re-triggering the tag-push `release` job, but GitHub deliberately suppresses workflow runs from pushes authenticated with the default `GITHUB_TOKEN` (documented loop-prevention), so that re-trigger never fires; switching to a `needs`/outputs dependency makes the release-merge path actually execute. `scripts/release.sh`'s bump-then-tag flow remains the `workflow_dispatch` path. The `release` job uses `if: always() && (...)` so a skipped `tag-on-release-merge` (tag-push / dispatch paths) does not block it. | Intake left the concrete mechanism open (assumption #17, D:50) and directs recording the choice here. A `release`-label gate is explicit and low-false-positive vs. commit-message sniffing or tagging every push. Tagging the already-bumped version (not re-bumping) matches `release.sh`'s package.json-anchored version model and avoids a double bump. Driving the pipeline via `needs`/outputs (rather than a `GITHUB_TOKEN` tag re-trigger) is the self-contained fix that needs no new PAT secret. Trivially swappable; governed by repo PR conventions | S:70 R:78 A:72 D:62 |
| 4 | Certain | `ci.yml` reuses release.yml's exact pinned SHAs (`actions/checkout@34e1148…`, `actions/setup-node@49933ea…`) and node 20 | Task directs reusing the same pins; release.yml is the in-repo source of truth for both SHAs and node version | S:92 R:75 A:92 D:90 |
| 5 | Certain | `captured_at` uses `new Date().toISOString()` (millisecond, `Z`) | Intake assumption #14 — the contract mandates ISO-8601 UTC `Z`; `toISOString()` is exactly that; sub-second precision is the JS default, trivially trimmed if `wt.json` later pins seconds | S:85 R:88 A:88 D:88 |
| 6 | Certain | `root.commands = []`; flat document, no recursion | Intake assumptions #2/#11 — tu prints no per-subcommand help pages; flat is correct. Forward extension point only | S:92 R:78 A:92 D:90 |
| 7 | Certain | Capture via built binary `node dist/tu.mjs --help` with `NO_COLOR=1`, byte-for-byte | Intake assumptions #3/#9 — "literal output of running the built CLI"; force NO_COLOR for guaranteed plain text | S:90 R:75 A:90 D:88 |
| 8 | Certain | Producer fails the build (non-zero) on capture error / empty stdout / validation failure; never PRs a malformed artifact | Intake assumption #8 + constitution II is runtime-only, so a build artifact must fail loud | S:85 R:72 A:90 D:88 |
| 9 | Certain | shll.ai write is PR + `gh pr merge --auto --squash`, `GH_TOKEN`=`SHLLAI_TOKEN`, branch `tu-help-dump-v{version}`, never a direct push to main | Intake assumptions #6/#13/#15/#16 + task spec; `GITHUB_TOKEN` can't cross repos so `SHLLAI_TOKEN` is required for both git auth and `gh` | S:88 R:65 A:88 D:85 |
| 10 | Confident | `SHLLAI_TOKEN` (contents + PR write on shll.ai) and shll.ai auto-merge are pre-provisioned; out of scope | Intake assumption #10 — external state this change cannot itself verify; backlog says "existing repo secret" | S:80 R:55 A:75 D:80 |
| 11 | Confident | `ci.yml` uses `npm ci` (lockfile present) for reproducible installs | `package-lock.json` exists; `npm ci` is the CI-correct deterministic install. release.yml only sets up node for the workflow_dispatch tag path so offers no install precedent; `npm ci` is the standard choice | S:72 R:80 A:78 D:75 |
| 12 | Confident | No `pull_request` trigger on `ci.yml` (push-to-`main` only) | Intake CI-topology and assumption #16 specify push-to-`main`; no test requires PR triggers. Easily added later if desired | S:75 R:80 A:78 D:78 |

12 assumptions (8 certain, 4 confident, 0 tentative).
