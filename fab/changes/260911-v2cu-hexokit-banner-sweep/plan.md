# Plan: HexoKit Banner Sweep (tu README blockquote)

**Change**: 260911-v2cu-hexokit-banner-sweep
**Intake**: `intake.md`

## Requirements

### README: Toolkit banner blockquote

#### R1: README head carries the HexoKit banner
`README.md` line 3 MUST be byte-identical to the `readme-extraction` standard's revised mandated blockquote (shll PR #98, branch `260911-ttoa-hexokit-banner-and-policy`): `> Part of [HexoKit](https://hexokit.com) — see all projects there.` (em dash U+2014). The README head order MUST remain `# tu` H1 → blank → blockquote → blank → badge line → blank → tagline, so the site slice still begins at "AI coding assistant cost tracking CLI."

- **GIVEN** the current README with the `shll toolkit` blockquote on line 3
- **WHEN** the change is applied
- **THEN** `grep -Fx '> Part of [HexoKit](https://hexokit.com) — see all projects there.' README.md` exits 0
- **AND** `sed -n '1p;5p;7p' README.md` still prints the H1, the badge line, and the tagline unchanged

### README / docs: run-kit product mentions

#### R2: No present-tense run-kit product mentions remain on live surfaces
tu's live surfaces (`README.md`, `docs/site/**`, `docs/specs/**`) MUST contain no prose that names the dashboard product as "run-kit". Substrate identifiers (`rk`, `run-kit context` as the verb's long-binary form) and historical artifacts (`fab/changes/**`, `docs/memory/**` narrative) are out of scope per D2 and D11 and MUST NOT be edited. The audit result MUST be recorded in this plan's `## Notes` so review can verify the no-op was checked, not skipped.

- **GIVEN** the repo after R1 is applied
- **WHEN** `grep -rn -i 'run-kit\|runkit' README.md docs/site docs/specs` runs
- **THEN** it prints nothing
- **AND** `src/node/core/__tests__/skill.test.ts:26` still reads `// run-kit context)` (untouched substrate)

### Scope containment

#### R3: The change touches only the banner line
Outside `fab/changes/260911-v2cu-hexokit-banner-sweep/`, the branch diff against `main` MUST consist of exactly one changed line in `README.md` (1 insertion, 1 deletion) until hydrate adds its memory edit. `shll.ai` URLs, the "shll toolkit" install prose, badges, `docs/site/**`, and all source files MUST be unchanged. `npm run build && npm test` MUST pass.

- **GIVEN** the applied change
- **WHEN** `git diff --stat main -- . ':!fab/changes'` runs
- **THEN** the only file listed is `README.md` with `1 +` and `1 -`
- **AND** `npm run build && npm test` exits 0

### Non-Goals

- Flipping `https://shll.ai/install`, `https://shll.ai/tu/commands/`, or `docs/site/skill.md`'s shll.ai mentions — Phase 2 (X2/X4) after shll.ai becomes a redirect host (D4, D7)
- Flipping "To install the entire shll toolkit" / "Have other shll tools?" prose — the family-name sweep is the X4-analog pass, not C7
- Any `sahil87/…` repo link or badge change — R2 (Phase 3); tu's own repo is not renamed by any row
- Renaming the `run-kit context` mention in `skill.test.ts` — substrate tier (D2)
- Rewriting `fab/changes/**` or `docs/memory/**` narrative mentioning run-kit — historical tier (D11); the memory *present-truth* sentence about the blockquote is hydrate's job, not apply's
- Version bump — README-only; Output Stability is not engaged

### Design Decisions

#### Adopt the standard's revised text from the unmerged PR branch
**Decision**: Take the mandated blockquote verbatim from shll PR #98's branch (`docs/site/standards/readme-extraction.md`), not from the installed `shll standards readme-extraction` output.
**Why**: The user declared gate C1 met at "PR up, review-pr done"; the installed shll binary predates C1 and still prints the pre-rebrand line. D14 (Confirmed) fixes the wording, and the PR text matches it exactly.
**Rejected**: Waiting for #98 to merge and ship — would serialize all seven satellite sweeps behind a shll release for no content gain; the drift risk (PR wording changing before merge) is recorded in the intake and re-checked at ship.
*Introduced by*: 260911-v2cu-hexokit-banner-sweep

## Tasks

### Phase 2: Core Implementation

- [x] T001 Replace `README.md` line 3 `> Part of the [shll toolkit](https://shll.ai) — see all projects there.` with `> Part of [HexoKit](https://hexokit.com) — see all projects there.`; leave every other byte of the file unchanged <!-- R1 -->

### Phase 3: Integration & Edge Cases

- [x] T002 [P] Audit live surfaces for present-tense run-kit product mentions (`grep -rn -i 'run-kit\|runkit' README.md docs/site docs/specs`; plus a repo-wide grep excluding `node_modules/`, `dist/`, `fab/changes/`, `.agents/`) and record the command + result under `## Notes` in this file; edit nothing <!-- R2 -->
- [x] T003 Verify scope containment and CI health: `git diff --stat main -- . ':!fab/changes'` shows only `README.md` 1+/1-; `sed -n '1,8p' README.md` shows H1 → blockquote → badges → tagline; `npm run build && npm test` passes; record results under `## Notes` <!-- R3 -->

## Acceptance

### Functional Completeness

- [x] A-001 R1: `README.md` line 3 is byte-identical to `> Part of [HexoKit](https://hexokit.com) — see all projects there.` (em dash, no leading "the")
- [x] A-002 R2: `grep -rn -i 'run-kit\|runkit' README.md docs/site docs/specs` prints nothing, and the audit command + result are recorded in `## Notes`
- [x] A-003 R3: `git diff --stat main -- . ':!fab/changes'` lists only `README.md` with 1 insertion and 1 deletion (pre-hydrate)

### Behavioral Correctness

- [x] A-004 R1: README head order is H1 → blockquote → badge line → tagline; the first prose line is still "AI coding assistant cost tracking CLI."

### Scenario Coverage

- [x] A-005 R3: `npm run build && npm test` exits 0 on the branch

### Edge Cases & Error Handling

- [x] A-006 R2: `src/node/core/__tests__/skill.test.ts` is unchanged (the `run-kit context` substrate comment remains); no `fab/changes/**` or `docs/memory/**` narrative was rewritten by apply

### Code Quality

- [x] A-007 Pattern consistency: The README head still conforms to the `readme-extraction` standard's rule-1 shape (single `#` H1, blockquote, contiguous badge run, prose), and no other README rule (absolute images, natural `docs/site/` links, absolute command-reference URL) regressed
- [x] A-008 No unnecessary duplication: No new files, scripts, or prose were added; the change is a one-line substitution

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

### Apply record (2026-09-11)

- **T001** — `README.md:3` replaced via a single anchored `sed` substitution. `grep -Fxc '> Part of [HexoKit](https://hexokit.com) — see all projects there.' README.md` → `1`. Head order verified with `sed -n '1,8p'`: `# tu` → blank → blockquote → blank → badge line → blank → `AI coding assistant cost tracking CLI.` / `Track your token usage in style!`.
- **T002 audit** — `grep -rn -i 'run-kit\|runkit' README.md docs/site docs/specs` → no output (exit 1). Repo-wide `grep -rn -i 'run-kit\|runkit' --exclude-dir={node_modules,dist,fab,.agents,.git} .` → exactly one hit, `src/node/core/__tests__/skill.test.ts:26` (`// run-kit context)` — the `rk context` verb, substrate tier, D2; untouched). `fab/changes/**` hits are historical tier (D11; untouched). **No present-tense run-kit product mentions exist on tu's live surfaces; nothing to flip.**
- **T003 verification** — `git diff --stat main -- . ':!fab/changes'` → `README.md | 2 +-  (1 insertion, 1 deletion)`, no other file. `npm ci && npm run build` → exit 0 (`dist/tu.mjs 172.9kb`; the worktree initially lacked the optional `@ccusage/ccusage-linux-x64` package until `npm ci`). `npm test` → 1091 tests, 1091 pass, 0 fail — run with `env -u TU_METRICS_REPO` because this shell exports `TU_METRICS_REPO=git@github.com:wvrdz/tu-metrics.git`, which the config layer honors as a bootstrap key and which makes ~20 config/sync tests fail regardless of branch (environment leak, not a code or README issue; CI on `main` at 02deeea is green).

### Hydrate + review-pr record (2026-09-11)

- **Hydrate** rewrote the `build/toolchain` toolkit-standards bullet and ran the mandated `fab docs-index docs/memory` regen. Because this is the first regen since the fab-kit 2.25 upgrade (#76), the generator migrated every `docs/memory/**/index.md` and `log.md` banner from `fab memory-index` to `fab docs-index` and scaffolded the hand-managed manual block — 13 generated files, content rows unchanged. R3's "one line in README" assertion is pre-hydrate by construction; these generated files are hydrate's output, not sweep scope.
- **Copilot review (PR #77)** — three findings, all acted on: (1) the regen dropped the root landing's "New here?" note → restored via `docs_index.roots[0].nav_note` in `fab/project/config.yaml` (override above the fence), minus its dead `../specs/glossary.md` link (no such file on `main`); (2) shll#98 merged 2026-09-11T17:51Z and shipped as shll v0.1.31 before the review → the memory pin reads the released version, not "pre-release"; (3) the generated-file scope is documented here.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Three tasks; the audit and the verification are tasks with recorded output rather than silent checks | Review needs evidence that the "flip mentions" brief was checked and found empty; recording it in `## Notes` is the lightest durable form | S:85 R:95 A:90 D:90 |
| 2 | Certain | The memory edit to `build/toolchain` is left to hydrate, not done in apply | `_pipeline.md`/`fab-continue.md` assign memory writes to hydrate; R3's scope-containment check is stated pre-hydrate for that reason | S:90 R:95 A:95 D:95 |
| 3 | Confident | `main` is the diff base for R3 (the worktree branch was renamed from a disposable `wt` branch off `main`) | `git merge-base HEAD main` equals the branch point; the review worker uses the same base | S:75 R:90 A:80 D:85 |

3 assumptions (2 certain, 1 confident, 0 tentative).
