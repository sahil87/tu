# Plan: Remove shll.ai help-dump push wiring (shll.ai now pulls)

**Change**: 260603-dmhw-remove-shll-ai-push-wiring
**Status**: In Progress
**Intake**: `intake.md`

## Requirements

### CI: Release workflow push-wiring teardown

#### R1: Remove the shll.ai push step
The `release` job in `.github/workflows/release.yml` MUST NOT contain the `PR help/tu.json into shll.ai` step. The cross-repo clone of `sahil87/shll.ai`, the branch/commit/`git push --force-with-lease`, the `gh pr create`/`gh pr merge --auto --squash` calls, and the `SHLLAI_TOKEN`/`GH_TOKEN: ${{ secrets.SHLLAI_TOKEN }}` env block MUST all be removed (they collapse into this single step).

- **GIVEN** `release.yml` currently ends with the `PR help/tu.json into shll.ai` step (lines ~208–249)
- **WHEN** the teardown is applied
- **THEN** that step and its leading `── help-dump → shll.ai ──` comment block are gone
- **AND** no workflow under `.github/workflows/` references `SHLLAI_TOKEN`, clones `sahil87/shll.ai`, or runs `gh pr create`/`gh pr merge` against it

#### R2: Remove the producer step `Generate help/tu.json`
The `release` job MUST NOT contain the `Generate help/tu.json` step (`run: npm run help-dump`). Once the push step is gone this step has no consumer in `release.yml` (its output `help/` is gitignored and was only read by the push step).

- **GIVEN** `release.yml` has a `Generate help/tu.json` step (lines ~180–181)
- **WHEN** the teardown is applied
- **THEN** that step is removed from the workflow

#### R3: Trim the stale "Fail-loud steps FIRST" comment
The comment block at `release.yml` lines ~167–173 MUST be trimmed so it no longer references generating/validating `help/tu.json`. It SHALL describe only the remaining fail-loud steps (install + build) ordered before the external side effects (GitHub Release, Homebrew tap).

- **GIVEN** the comment justifies ordering help-dump generation before external side effects
- **WHEN** both help-dump steps are removed
- **THEN** the comment is rewritten to reference only install + build as the fail-loud steps run before external side effects

#### R4: Preserve everything else in `release.yml` and the help-dump producer surface
The change MUST NOT delete `release.yml` nor alter any other step (`tag-on-release-merge`, tag resolution, `Install dependencies`, `Build bundle`, `Generate release notes`, `Create GitHub Release`, `Update Homebrew tap`). The change MUST NOT touch `scripts/help-dump.mjs`, the `help-dump` npm script in `package.json`, or `src/node/core/__tests__/help-dump.test.ts`.

- **GIVEN** the directive's invariant "removes only the transport, never the command that produces it"
- **WHEN** the teardown is applied
- **THEN** only the two steps + one comment in `release.yml` change; the producer script, npm script, and unit test are byte-identical to before
- **AND** `release.yml` still parses as valid YAML and `npm test` (including the help-dump unit test) passes

### Non-Goals
- Removing the `SHLLAI_TOKEN` repo secret — a GitHub repo-settings action flagged for manual follow-up, not a code change.
- Enriching the help-dump schema (future shll.ai change); tu keeps emitting `schema_version: 1`.
- Editing `.github/workflows/ci.yml` — it never ran help-dump and is explicitly out of scope per the intake (its line-4 comment is a historical doc reference, not live wiring).

### Design Decisions
1. **Delete (not keep) the `Generate help/tu.json` step**: with the push gone it has no consumer and `help/` is gitignored — *Why*: avoids dead CI work; the producer logic stays protected by the `ci.yml` unit test — *Rejected*: retaining it as a smoke test (the unit test already covers the producer; the user confirmed delete).
2. **Confine all edits to `release.yml`**: per the directive's hard invariant — *Why*: the only live push wiring lives in two adjacent `release` job steps — *Rejected*: touching `ci.yml`'s stale comment (explicit scope lock; harmless historical reference).

## Tasks

### Phase 2: Core Implementation

- [x] T001 Remove the `PR help/tu.json into shll.ai` step and its leading `── help-dump → shll.ai ──` comment block (lines ~208–249) from the `release` job in `.github/workflows/release.yml` <!-- R1 -->
- [x] T002 Remove the `Generate help/tu.json` step (`run: npm run help-dump`, lines ~180–181) from the `release` job in `.github/workflows/release.yml` <!-- R2 -->
- [x] T003 Trim the "Fail-loud steps FIRST" comment block (lines ~167–173) in `.github/workflows/release.yml` to reference only install + build, dropping help-dump generation/validation language <!-- R3 -->

### Phase 3: Integration & Edge Cases

- [x] T004 Verify `release.yml` still parses as valid YAML, run `npm test` (help-dump unit test must still pass), and grep `.github/` for `SHLLAI_TOKEN`/`shll.ai`/`help/tu.json`/`help-dump` to confirm no live push wiring remains (only docs/comments) <!-- R4 -->

## Execution Order

- T001, T002, T003 all edit `release.yml` and are sequential (same file). T004 verifies after all edits.

## Acceptance

### Functional Completeness

- [x] A-001 R1: The `PR help/tu.json into shll.ai` step (clone, branch, push, `gh pr create`/`merge`, `SHLLAI_TOKEN` env) is fully removed from `release.yml`
- [x] A-002 R2: The `Generate help/tu.json` step (`npm run help-dump`) is removed from `release.yml`
- [x] A-003 R3: The "Fail-loud steps FIRST" comment references only install + build, with no dangling help-dump references
- [x] A-004 R4: `release.yml` still exists and all other steps (tag-on-release-merge, tag resolution, Install, Build, Generate release notes, Create GitHub Release, Update Homebrew tap) are intact

### Removal Verification

- [x] A-005 R1: `grep -rn "SHLLAI_TOKEN\|shll.ai\|help/tu.json\|help-dump" .github/` returns only docs/comments — no clone of shll.ai, no `gh pr create`/`merge` against it, no `npm run help-dump` step (single hit: stale `ci.yml:4` comment — flagged should-fix)

### Behavioral Correctness

- [x] A-006 R4: `release.yml` parses as valid YAML (`python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))"` succeeds)

### Scenario Coverage

- [x] A-007 R4: `npm test` — the help-dump unit test passes (8/8), guarding the untouched producer contract. (16 pre-existing failures in `config.test.ts`/`cli-sync.test.ts` are environmental — a real `~/.tu.conf` bleeds into config-resolution tests; identical on the base commit with `release.yml` reverted, so NOT caused by this change. Clean in CI.)

### Removal Verification (producer surface preserved)

- [x] A-008 R4: `scripts/help-dump.mjs`, the `help-dump` entry in `package.json`, and `src/node/core/__tests__/help-dump.test.ts` are unchanged (`git diff --name-only HEAD` shows none of them)

### Code Quality

- [x] A-009 Pattern consistency: The trimmed comment matches the existing comment style/voice in `release.yml`
- [x] A-010 No unnecessary duplication: No leftover or orphaned fragments (env keys, comment lines) from the removed steps

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Confine all code edits to `release.yml`; do not touch `ci.yml` even though its line-4 comment mentions "the help-dump→shll.ai step" | The directive's hard invariant ("All code changes are confined to release.yml") and the intake's Impact section ("ci.yml — unchanged") both lock scope; the ci.yml reference is a historical doc comment, which the verification grep explicitly tolerates ("only docs/comments") | S:95 R:90 A:95 D:90 |
| 2 | Confident | Remove the `── help-dump → shll.ai ──` comment header above the push step along with the step itself | The comment exists solely to explain the step being deleted; leaving it orphaned would dangle a stale reference. Low blast radius | S:90 R:90 A:90 D:85 |

2 assumptions (1 certain, 1 confident, 0 tentative, 0 unresolved).
