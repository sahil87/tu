# Plan: Cutover formula — release.yml pushes the Go formula to the tap

**Change**: 260923-jmh4-cutover-formula
**Intake**: `intake.md`

## Requirements

### Build: the tap step ships the generated Go formula

#### R1: The "Update Homebrew tap" step copies `dist/tu.rb` over the tap's formula
`.github/workflows/release.yml`'s `release` job MUST replace the `sed -i "s|tag: \"v.*\"|tag: \"v${version}\"|" /tmp/tap/Formula/tu.rb` line in its "Update Homebrew tap" step with `cp dist/tu.rb /tmp/tap/Formula/tu.rb`. The rest of the step (clone of `sahil87/homebrew-tap` with `HOMEBREW_TAP_TOKEN`, bot identity, `git add Formula/tu.rb`, commit message `tu ${version}`, push) MUST stay byte-identical, and the step MUST remain the last step of the job, after "Create GitHub Release". No guard on bump type or version shape SHALL be added.

- **GIVEN** a `v*` tag push, a `workflow_dispatch`, or a release-labeled merge to `main`
- **WHEN** the `release` job reaches "Update Homebrew tap"
- **THEN** the tap's `Formula/tu.rb` is replaced by the `dist/tu.rb` rendered earlier in the same run (Go formula: `version "<v>"`, four `tu-go-*` tarball URLs with sha256s, `libexec` + `bin` symlink install, no `depends_on`), committed as `tu <version>` and pushed
- **AND** the release assets the formula points at already exist, because `gh release create` ran first

#### R2: `release.yml` prose reflects the shipped state; the Node steps stay
The `release` job's header comment MUST state that from the cutover (plan row X1) the formula pushed to the tap is the generated Go formula (`dist/tu.rb`, prebuilt tarballs, no `depends_on "node"`) and that the Node build still runs as the fail-loud check that the D10 rollback artifact builds. The step name `Generate Go formula (dist/tu.rb — not pushed until cutover, plan row X1)` MUST become `Generate Go formula (dist/tu.rb — pushed to the tap below)`, keeping the `cat dist/tu.rb`. The fail-loud comment above "Install dependencies" MUST say why `npm ci` survives (`scripts/package-go.sh` and the justfile read `package.json`/`package-lock.json` via `node -p`; `npm run build` proves the Node rollback build). `npm ci`, `npm run build`, `tag-on-release-merge`, and every Go step MUST be unchanged.

- **GIVEN** the edited workflow
- **WHEN** it is read top to bottom
- **THEN** no comment or step name claims the formula is "not pushed" or "ships the Node build until cutover", and the job's step list differs from `main` only in the tap step's copy line, the step name, and comments

#### R3: Stale "not pushed until cutover" comments in `scripts/go-formula.sh` and `justfile` are reworded
`scripts/go-formula.sh`'s header comment MUST read "Generate the Go Homebrew formula into dist/tu.rb — release.yml pushes it to sahil87/homebrew-tap (plan row X1)." The `justfile`'s `go-formula` recipe comment MUST drop "NOT pushed to the tap until cutover" in favor of "release.yml pushes it to the tap", and the Go section banner ("built and tested in CI, NOT shipped until cutover") MUST state the shipped state: the Go binary is what the formula ships from the first release after X1; `src/node/` remains the harness oracle and the D10 rollback build until Z1. No recipe body changes.

- **GIVEN** the edited files
- **WHEN** `grep -n -i "until cutover" scripts/go-formula.sh justfile` runs
- **THEN** it prints nothing, and `just --list` still succeeds

#### R4: The plan doc records the version shift and the row status
`fab/plans/sahil/26-09-15-go-port.md` MUST be updated: the X1 row's Status cell becomes landed text naming `260923-jmh4-cutover-formula`, stating that 0.12.0 shipped as a Node release on 2026-09-23 before the flip was wired, and that the cutover release is the next minor bump, 0.13.0, cut by Sahil with `git fetch origin && git checkout origin/main && just release minor`; the **Status (2026-09-23)** paragraph's "X1 = `just release minor` → 0.12.0" becomes 0.13.0 with a one-clause reason; D9's Rationale cell gains a trailing note that 0.12.0 was consumed by a Node release on 2026-09-23 and the cutover number moved to 0.13.0 (the Decision cell text is not rewritten); Z1's "after 0.12.0" becomes "after the cutover release (0.13.0)". No table cell may contain a literal pipe character.

- **GIVEN** the edited plan doc
- **WHEN** `grep -n "0.12.0" fab/plans/sahil/26-09-15-go-port.md` runs
- **THEN** every remaining occurrence is either the historical fact (0.12.0 shipped as Node) or D9's original decision text, and the X1 row, Status paragraph and Z1 row name 0.13.0

### Non-Goals

- Cutting the release — `just release minor` stays Sahil's manual step; nothing here runs it.
- Any bump-type or version-shape guard in the workflow (intake § Why explains the dangling-tag failure mode).
- A `brew install` smoke of the pushed formula in CI — cutover criterion 5 stays a by-hand check.
- `README.md`, `docs/site/**`, `docs/specs/**`, `fab/project/constitution.md`, `fab/project/context.md`, `ci.yml`, `.github/formula-template.rb`, `scripts/package-go.sh`, `scripts/release.sh`, `scripts/dogfood-*.sh` — untouched (X2 owns the constitution/context prose).

### Design Decisions

#### Unconditional flip, no bump-type guard
**Decision**: From the first release after this change merges, the tap step always pushes `dist/tu.rb`; the workflow does not check whether the release is a minor bump.
**Why**: The tag-push path carries no bump type; an `X.Y.0` check would need tap state to know it is the first Go push; and any in-job guard fires after `scripts/release.sh` has already pushed the version commit and the `v*` tag, leaving a dangling tag — worse than a Go formula under a patch number, which is a versioning-policy nit rather than a broken install. D9's minor bump is the operator's `just release minor`.
**Rejected**: A `steps.version` regex guard failing the job; a tap-state probe (`depends_on "node"` present ⇒ require `.0`).
*Introduced by*: 260923-jmh4-cutover-formula

## Tasks

### Phase 1: Core Implementation

- [x] T001 Edit `.github/workflows/release.yml`: replace the tap step's `sed` line with `cp dist/tu.rb /tmp/tap/Formula/tu.rb`; rewrite the `release` job header comment's R1/X1 sentence; rename the Go-formula step to `Generate Go formula (dist/tu.rb — pushed to the tap below)`; extend the fail-loud comment with why `npm ci`/`npm run build` stay <!-- R1, R2 -->
- [x] T002 [P] Reword the comments in `scripts/go-formula.sh` (header) and `justfile` (`go-formula` recipe comment, Go section banner); no recipe or script body changes <!-- R3 -->
- [x] T003 [P] Edit `fab/plans/sahil/26-09-15-go-port.md`: X1 row Status, Status (2026-09-23) paragraph, D9 Rationale note, Z1 wording; no pipes inside cells <!-- R4 -->
- [x] T004 Verify: `python3 -c 'import yaml; yaml.safe_load(open(".github/workflows/release.yml"))'` (or `actionlint` if installed) passes; `just --list` succeeds; `git diff --stat` touches only the five files above plus `fab/changes/`; `grep -n -i "until cutover" scripts/go-formula.sh justfile .github/workflows/release.yml` prints nothing <!-- R1, R2, R3, R4 -->

## Acceptance

### Functional Completeness

- [x] A-001 R1: The tap step's only command change is `sed …` → `cp dist/tu.rb /tmp/tap/Formula/tu.rb`; clone, identity, `git add Formula/tu.rb`, `git commit -m "tu ${version}"`, `git push` are byte-identical to `main`
- [x] A-002 R1: "Update Homebrew tap" remains the last step, after "Create GitHub Release"; no new guard step or condition exists
- [x] A-003 R2: The `release` job header comment, the Go-formula step name and the fail-loud comment describe the pushed Go formula and the retained Node steps; `npm ci`, `npm run build`, `tag-on-release-merge` and all Go steps are unchanged
- [x] A-004 R3: `scripts/go-formula.sh` and `justfile` carry no "until cutover" wording; recipe bodies and script logic are unchanged; `just --list` succeeds
- [x] A-005 R4: The plan doc's X1 row, Status paragraph, D9 note and Z1 wording match R4; no table cell contains a literal pipe

### Behavioral Correctness

- [x] A-006 R1: A dry read of the job shows `dist/tu.rb` exists at the tap step (rendered by "Generate Go formula" earlier in the same job) and the formula it carries has no `depends_on`

### Scenario Coverage

- [x] A-007 R2: `grep -n -i "not pushed\|until cutover\|ships the Node build" .github/workflows/release.yml` prints nothing
- [x] A-008 R4: `grep -n "0.13.0" fab/plans/sahil/26-09-15-go-port.md` hits the X1 row, the Status paragraph, D9's rationale and the Z1 row

### Edge Cases & Error Handling

- [x] A-009 R1: The workflow file parses as YAML after the edit (python `yaml.safe_load` or `actionlint`)

### Code Quality

- [x] A-010 Pattern consistency: The tap step matches fab-kit's `release.yml` tap step shape (clone → cp → add → commit → push)
- [x] A-011 No unnecessary duplication: No second formula-generation or tap-update path is introduced
- [x] A-012 No swallowed errors: No `|| true`, `continue-on-error`, or silenced failure is added to the workflow

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Verification is YAML parse + grep + `just --list`; no `just go-dist` run | The change is workflow YAML, comments and docs; `go-dist` needs network and a tag and exercises unchanged scripts | S:80 R:95 A:90 D:90 |
| 2 | Certain | D9's Decision cell text is preserved; the shift is a trailing note in its Rationale cell | D1–D13 are Sahil's proposals to confirm; the plan header only licenses status-column updates | S:70 R:95 A:85 D:85 |

2 assumptions (2 certain, 0 confident, 0 tentative).
