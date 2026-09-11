# Intake: HexoKit Banner Sweep (tu README blockquote)

**Change**: 260911-v2cu-hexokit-banner-sweep
**Created**: 2026-09-11

## Origin

One-shot `/fab-new` invocation. Raw input:

> Per fab/plans/sahil/26-09-10-hexokit-rebrand.md row C7 (hexokit-banner-sweep), applied to the tu repo, gated on C1 (shll change ttoa, PR shll#98 up, review-pr done): apply the readme-extraction standard's revised mandated blockquote to this repo's README (-> "Part of [HexoKit](https://hexokit.com) -- see all projects there"); flip any PRESENT-TENSE "run-kit" PRODUCT mentions (prose referring to the dashboard product) to HexoKit. The `rk`/`run-kit` SUBSTRATE (binary name, verbs, options) is UNTOUCHED. Repo links stay as-is until R2. Read the plan doc's Decision log (D1-D14) and row C7 first; run `shll standards` and read `readme-extraction` before editing.

Context read before drafting (all binding on this change):

- **Plan doc**: `run-kit` repo, `fab/plans/sahil/26-09-10-hexokit-rebrand.md` — Decision log D1–D14, Naming tiers, row C7, Pickup protocol. D1–D4, D13, D14 are **Confirmed** by Sahil; D5–D12 are Proposed. D11 (historical text is not renamed — only live surfaces change) is the scope rule the prompt's "PRESENT-TENSE" wording restates.
- **Gate C1**: shll PR [#98](https://github.com/sahil87/shll/pull/98) (`260911-ttoa-hexokit-banner-and-policy`) is **open, draft, not merged**; its pipeline ran through review-pr on 2026-09-11. The user declared the gate met at "PR up, review-pr done". The revised standard text was read from that branch (`git show origin/260911-ttoa-hexokit-banner-and-policy:docs/site/standards/readme-extraction.md`), because the installed `shll standards readme-extraction` (shll release binary) still prints the old blockquote.
- **Standard**: `readme-extraction` §"README structure" rule 1 — "The blockquote is this exact line in all seven repos" — revised on the PR branch to:

  ```markdown
  > Part of [HexoKit](https://hexokit.com) — see all projects there.
  ```

  Note the character: the standard uses an em dash `—`; the prompt typed `--` as an ASCII stand-in. The standard's exact line wins.
- **Constitution**: `### Toolkit Standards` — any change to `README.md` or `docs/site/` MUST be checked against the governing standards; standards revised in the shll repo bind this repo without constitutional amendment.

## Why

1. **Problem.** The HexoKit rebrand (Approach B: the dashboard product becomes HexoKit, the `rk` substrate stays) is a five-repo cutover. The README blockquote under each repo's H1 is the single most visible cross-repo brand surface — the first line a visitor reads on all seven GitHub repo pages. C1 revised the `readme-extraction` standard to mandate the HexoKit line; C7 is the satellite sweep that makes each companion README conform. tu is one of those satellites.
2. **Consequence of not doing it.** Once shll #98 merges and ships, tu's README violates the standard its constitution binds it to (`### Toolkit Standards`), and the toolkit's repo pages split across two brands ("shll toolkit → shll.ai" on tu, HexoKit on the others) — exactly the two-brand confusion the rebrand exists to remove. D14's rationale names this: leaving the banner "reintroduces the two-brand split for a saving of seven lines".
3. **Why this scope and not a wider one.** The plan sequences brand surfaces deliberately: C7 is *banner + present-tense product mentions only*. Domain flips (`shll.ai` → `hexokit.com`) wait for Phase 2 (X2 makes shll.ai a redirect host; X4 sweeps the standards' own "shll toolkit" phrases), and repo/formula renames wait for Phase 3 (R1, R2). Doing more here would either point users at a site that is not yet announced or move ahead of the roster fields that are runtime-coupled to `shll install`/`check-updates`. The plan doc's Order line puts C7 after C1 and before X1 — this change sits exactly there.

## What Changes

### 1. README blockquote (the one edit)

`README.md` line 3 currently reads:

```markdown
> Part of the [shll toolkit](https://shll.ai) — see all projects there.
```

Replace with the standard's exact revised line:

```markdown
> Part of [HexoKit](https://hexokit.com) — see all projects there.
```

Everything else in the README head stays byte-identical: `# tu` H1 on line 1, blank line, blockquote on line 3, blank line, the single badge line, then the tagline "AI coding assistant cost tracking CLI." as the first prose line (that is where the site slice begins). This preserves the standard's rule-1 order (H1 → blockquote → badges → prose).

Pipeline effect: none. The consumer extractor (`BLOCKQUOTE_RE` in shll.ai's `extract-readme.ts`, per D14) strips any leading blockquote as chrome, so the `/tu/readme` slice on the site is unchanged by this edit.

### 2. Present-tense "run-kit" product mentions — audit result: none to flip

A repo-wide grep (`run-kit`, `runkit`, `hexokit`, case-insensitive; excluding `node_modules/`, `dist/`, `fab/changes/archive/`, `.agents/`) over `README.md`, `docs/site/**`, `docs/specs/**`, `docs/memory/**`, `src/**`, `scripts/`, `justfile`, `package.json` found **zero** prose references to the run-kit dashboard product on tu's live surfaces. The only non-archive hit is a code comment:

```ts
// src/node/core/__tests__/skill.test.ts:26
// The bundle MUST NOT bake in timestamps or environment lookups (contrast
// run-kit context). This is a genre sanity check, not exhaustive.
```

`run-kit context` names the `rk context` verb (long-binary-name form) — that is **substrate tier** (D2, Naming tiers "Keep") and is left untouched. The remaining hits are `fab/changes/**` intake/plan prose from earlier changes — **historical tier** (D11, "Leave").

Deliverable for this section: nothing is edited; the plan's T-task for it is a recorded verification (the grep command and its result) so review can confirm the no-op was an audit, not an omission.

### 3. Explicitly left as-is (out of C7 scope — do not touch)

| Surface | Current text | Owner row | Why it stays |
|---------|--------------|-----------|--------------|
| `README.md:13`, `:19` | `curl -fsSL https://shll.ai/install \| sh …` | D4 / D10 / X2 | `shll.ai/install` is a live endpoint baked into shipped binaries; hexokit.com/install lands in S5 and shll.ai keeps serving a byte copy forever. Install docs are centralized (install-composition Policy B) — tu does not restate them |
| `README.md:16` | "To install the entire shll toolkit instead" | X4-analog (Phase 2) | "shll toolkit" family phrases flip to "HexoKit toolkit" in the Phase 2 sweep, after shll.ai becomes a redirect host. The user's C7 brief names only the blockquote and run-kit product mentions |
| `README.md:41` | "Have other shll tools? [`shll shell-install`](…sahil87/shll#…)" | D4 | `shll` remains the toolkit manager's name; the link targets the shll repo, which is never renamed |
| `README.md:51` | `https://shll.ai/tu/commands/` | D7 / X1 | The command-reference URL is the standard's rule-8 absolute URL; it becomes a 301 to `hexokit.com/tu/commands/` at X2 and flips in the X4 pass |
| `README.md:5` badges | `sahil87/tu` | — | tu's own repo; not renamed by any row |
| `docs/site/skill.md:63–65` | "shll.ai's pull cron … renders … on shll.ai" | X4-analog | Names the consuming site; a correctness fix only after X2 |
| `src/node/core/cli.ts:69`, `cli-exit-codes.test.ts:10` | "shll toolkit principle №4" | X4-analog | Code comments citing the `principles` standard by its family name; not a product mention |
| `src/node/core/__tests__/skill.test.ts:26` | "run-kit context" | D2 substrate | See §2 |
| `fab/changes/**`, `docs/memory/**` narrative | "run-kit" as a roster tool name | D11 historical | Not rewritten |

### 4. Memory update (`build/toolchain`)

`docs/memory/build/toolchain.md:58` currently states, as present truth:

> The toolkit's standardized name is **"shll toolkit"** — prose and comments across this repo (README head blockquote, `docs/`, code comments, the constitution's Toolkit Standards article) use that name …

After this change the README head blockquote no longer uses that name. Hydrate must rewrite this sentence to present truth, roughly:

- The README head blockquote is the `readme-extraction` standard's HexoKit banner (`> Part of [HexoKit](https://hexokit.com) — see all projects there.`) as revised in shll PR #98 (C1 of the HexoKit rebrand plan, run-kit `fab/plans/sahil/26-09-10-hexokit-rebrand.md`), adopted here by 260911-v2cu (row C7).
- Remaining "shll toolkit" phrases in prose and code comments (install section, `docs/site/skill.md`, exit-code comments) are intentionally unchanged until the Phase 2 sweep; `shll.ai` URLs are unchanged until X2/X4; the `sahil87/…` addresses rule is unchanged.
- The audit pin: `readme-extraction` as on shll PR #98 branch `260911-ttoa-hexokit-banner-and-policy` (pre-release; the installed `shll` binary still prints the pre-C1 line at the time of this change).

The existing sentence about `sahil87`-owned identifiers being addresses (260718-a3z0) stays true and stays.

### 5. Verification (readme-extraction "Verifying conformance" checklist, scoped)

- README top is `#` H1 → toolkit blockquote → badges; first prose line unchanged. Check with `sed -n '1,8p' README.md`.
- The blockquote is byte-identical to the standard's line on the PR branch: `grep -Fx '> Part of [HexoKit](https://hexokit.com) — see all projects there.' README.md` exits 0.
- No test asserts the README banner text (grep of `src/`, `scripts/`, `justfile`, `package.json` for "shll toolkit" / "Part of the" finds only the two exit-code comments) — so `npm test` is unaffected. Run `npm run build && npm test` once anyway; the CI gate requires green `build-and-test`.
- No other line of `README.md` or `docs/site/**` changed: `git diff --stat` shows `README.md` at 1 insertion, 1 deletion, plus the memory file.

### 6. Release / version

README-only plus memory. No CLI output, help text, or `docs/site/` page changes → no version bump (constitution "Output Stability" is not engaged). Commit type `docs`.

### 7. Cross-repo bookkeeping (operator step, outside this repo's PR)

Pickup protocol §4 of the plan doc: update row C7's Status cell in run-kit's `fab/plans/sahil/26-09-10-hexokit-rebrand.md` when the change is created and again when its PR merges. At creation the cell is set to record tu's change (`v2cu`, intake ready 2026-09-11) while noting fab-kit, wt, idea, hop, and the sahil87 profile remain not started. That file is on run-kit `main` with uncommitted edits from the S5 and C1 agents in the same style; the edit is a single-row in-place append and is not committed by this change. At merge, the ship/archive step should append the PR link to the same cell.

## Affected Memory

- `build/toolchain`: (modify) Rewrite the "Toolkit-standards conformance posture" bullet's naming sentence (line 58) to present truth: README head blockquote is the HexoKit banner per revised `readme-extraction` (shll #98, pre-release pin); "shll toolkit" phrases elsewhere deliberately unchanged until Phase 2; add 260911-v2cu to the audited-standards list.

## Impact

- **Files**: `README.md` (1 line), `docs/memory/build/toolchain.md` (1 bullet, at hydrate). No `src/`, `docs/site/`, `docs/specs/`, `package.json`, or workflow changes.
- **Site**: `/tu/readme` slice unchanged (blockquote is stripped chrome). `/tu/commands/` unchanged.
- **Tests / CI**: no test reads `README.md`; the `ci-gate` job must still pass on the PR (`npm ci && npm run build && npm test`).
- **Other repos**: run-kit plan-doc row C7 status cell (operator edit). Sibling repos (fab-kit, wt, idea, hop, sahil87 profile) get their own C7 changes — not this one.
- **Risk**: if shll #98's blockquote wording changes before merge, tu's banner drifts by that delta. Mitigation: the line is quoted verbatim from D14 (Confirmed) and the PR branch; re-check the merged text at ship.

## Open Questions

- None. Every decision below is graded Certain or Confident; no Unresolved rows.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Blockquote is the standard's exact line with the em dash `—` (not the `--` typed in the prompt) | Standard rule 1 says "this exact line in all seven repos"; the prompt's `--` is an ASCII stand-in for the same text | S:90 R:95 A:95 D:95 |
| 2 | Confident | Gate C1 is treated as met with shll #98 open (draft, review-pr done, unmerged); the revised text is taken from the PR branch, not the installed `shll` binary | User declared the gate ("PR up, review-pr done"); D14 is Confirmed and the PR text equals D14's wording; drift risk noted in Impact | S:85 R:90 A:65 D:80 |
| 3 | Certain | `rk`/`run-kit` substrate is untouched, including the `run-kit context` test comment | D2 Confirmed; Naming tiers "Keep"; the prompt says so explicitly | S:95 R:90 A:95 D:95 |
| 4 | Certain | No present-tense run-kit product mentions exist on tu's live surfaces; the "flip" task is recorded as a verified no-op audit rather than dropped | Repo-wide grep over README, docs/site, docs/specs, docs/memory, src, scripts found only the substrate comment and historical fab/changes prose | S:80 R:95 A:90 D:90 |
| 5 | Confident | "shll toolkit" / "shll tools" prose in the Install section, `docs/site/skill.md`'s shll.ai mentions, and code comments citing "shll toolkit principle №4" are left for the Phase 2 sweep | C7's brief is banner + product mentions; D14 routes family-name and domain flips to X4 after X2; touching them now would name a site that is not yet announced | S:70 R:90 A:65 D:65 |
| 6 | Certain | Badges, `sahil87/tu` links, `shll.ai/install`, `shll.ai/tu/commands/` are unchanged | D4/D7/D10 and R2; the plan's C7 row says "Repo links stay `sahil87/run-kit` until R2"; tu's own repo is not renamed by any row | S:90 R:90 A:90 D:90 |
| 7 | Confident | Memory `build/toolchain` is the only affected memory; hydrate rewrites its naming sentence to present truth and pins the audit to shll #98 pre-release | The grep of `docs/memory` for "shll toolkit"/blockquote found exactly this bullet; the `cli/data-pipeline` and `specs/usage` hits cite principle №4 (family name, unchanged) | S:70 R:90 A:75 D:75 |
| 8 | Certain | No version bump; docs-type change | README and memory only; no CLI output or help text changes, so Output Stability is not engaged | S:60 R:95 A:90 D:85 |
| 9 | Certain | Change type overridden to `docs` (inferred `feat`) | The edit is documentation prose; `fab status set-change-type` marks it explicit so refresh keeps it | S:85 R:95 A:95 D:95 |
| 10 | Certain | Run-kit plan-doc row C7 is updated in place at creation (tu sub-entry) as an operator step, not committed by this change | Pickup protocol §4; the S5 and C1 agents did the same on run-kit `main` uncommitted; a one-cell append cannot conflict with their rows | S:75 R:95 A:80 D:80 |

10 assumptions (7 certain, 3 confident, 0 tentative, 0 unresolved).
