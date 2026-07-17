# Intake: Toolkit Standards Conformance

**Change**: 260717-rdo3-toolkit-standards-conformance
**Created**: 2026-07-18

## Origin

One-shot `/fab-new` invocation. The task text (verbatim):

> Task: Bring this repo and its tool into conformance with the sahil87 toolkit standards.
>
> Precondition: `shll standards` runs on this machine (if the subcommand is missing, run `shll update`; if it still fails, stop and report -- do not proceed from memory or the website). This repo's constitution carries the Toolkit Standards article; this task is the conformance work it mandates.
>
> 1. Enumerate at runtime: run `shll standards`, then `shll standards <name>` for every listed entry. The list is authoritative -- do not assume which standards exist or what they require.
> 2. Audit this repo against each standard. For mechanical contracts (machine help output, README/docs-site structure), execute the standard's own verification checklist verbatim. For the principles, assess each numbered principle against the tool's actual behavior -- prompts and TTY handling, stdout/stderr separation, --json/--dry-run/--yes coverage, exit codes and error wording, idempotency, output volume.
> 3. Fix what is proportionate here: all mechanical-contract violations, and principle gaps that are small and additive (a missing flag, a misrouted stream, an unhelpful error). Larger gaps that would restructure the tool are NOT for this change -- record each as a draft change or issue per this repo's convention and reference it.
> 4. Deliverable: one fab change whose PR body contains a conformance report -- one section per standard with PASS or the gaps found, each gap dispositioned as fixed here (with the commit) or deferred to <ref>. Include the shll version audited against (`shll version`'s shll row), since standards are versioned with the shll release. Tests green; if the command tree changed, re-verify the machine-help contract afterward.
>
> Note on the "skill" standard specifically: if this repo has not yet implemented a `<tool> skill` subcommand, that is a known, deferred gap (per the toolkit's phased per-repo adoption -- no seven-repo flag-day) -- report it as "deferred, not yet adopted" rather than treating it as an in-scope fix for this change.

**Precondition verified at intake** (2026-07-18, this machine): `shll standards` exits 0 and lists exactly four standards — `principles` (foundation), `help-dump` (binary), `readme-extraction` (repo), `skill` (binary+repo). `shll version` reports `shll v0.0.23`. No `shll update` was needed.

**Key intake-time findings** (facts, verified on this machine — they seed the audit; the apply agent re-verifies everything at runtime):

- `tu help-dump` (v0.8.1) **emits `captured_at`** in its envelope — the current help-dump standard forbids this ("Do not emit `captured_at`. The capture timestamp is owned by shll.ai"). tu conforms to the June 2026 frozen contract (backlog item `[v76l]`, which *included* `captured_at`); the standard has since evolved. This is a known mechanical-contract violation, in scope.
- tu's constitution (v1.0.0) does **not** carry a Toolkit Standards article, despite the task's premise. Sibling repos `wt` and `shll` both gained one today (constitutions v1.1.0, Last Amended 2026-07-18); `wt`'s variant references `shll standards` and is the template for non-shll repos. hop/idea/run-kit do not have it yet — the rollout is per-repo, in progress.
- README head conforms to readme-extraction rule 1 (single `#` H1 → canonical toolkit blockquote → badge line → tagline prose).
- `docs/site/` exists with `install.md` and `workflows.md` (no reserved names used; no `skill.md`).
- No `tu skill` subcommand exists — expected; the skill standard itself states "No tool ships `skill` today".
- The help-dump standard's "tu exception" prose describes tu as "flag-based with no subcommands" emitting a flat tree (`root.commands: []`), but tu has since grown real subcommands (`shell-init`, hidden `help-dump`, per README and the CLI source under `src/node/core/`).

## Why

1. **The pain point**: The toolkit's standards are now versioned, runtime-enumerable contracts (`shll standards`, shipped with shll v0.0.23), but tu's conformance predates their publication. tu adopted pieces early — help-dump in change 260602-v76l against a June "frozen contract", README shaping in 260608-aqlc against the pre-formalization shll-ai notes — and at least one of those early contracts now *contradicts* the published standard (the `captured_at` field). No audit has ever run tu against the published set, and the ten CLI principles have never been systematically assessed against tu's actual behavior.
2. **The consequence of not fixing**: shll.ai consumes tu's `help-dump` output and README/docs-site tree mechanically; every contract violation is a live defect on tu's public pages (a rejected capture keeps stale docs published; a relative link 404s). Agents operating tu rely on the toolkit-wide contracts (stream split, exit codes, non-interactive behavior) — silent divergence makes tu the unreliable member of the fleet. And drift compounds: every release that ships against the wrong contract widens the gap.
3. **Why this approach**: Runtime enumeration (audit against what `shll standards` serves *today*, not memory or website copies) is what the standards system is designed for — standards are versioned with the shll release, so the audit pins the version it ran against. Proportionate fixing (mechanical violations + small additive principle gaps here; restructuring deferred with references) keeps this change reviewable while leaving an auditable trail for the rest. This mirrors the conformance passes done in sibling repos today (wt, shll — both constitutions amended 2026-07-18).

## What Changes

### 1. Audit procedure (runtime enumeration)

At apply time, the agent runs:

```sh
shll standards                       # authoritative list (4 entries at intake time)
shll standards principles            # ten CLI principles (foundation)
shll standards help-dump             # machine-help contract (binary)
shll standards readme-extraction     # README + docs/site structure (repo)
shll standards skill                 # agent skill bundle (binary+repo)
shll version                         # record the shll row — the version audited against
```

The apply-time list and content are authoritative — if shll was upgraded between intake and apply, audit against what it serves then and report *that* version. Each standard is audited as follows:

- **Mechanical contracts** (`help-dump`, `readme-extraction`): execute the standard's own "Verifying conformance" checklist verbatim, item by item, recording pass/fail per item.
- **Principles**: assess each of the ten numbered principles against tu's actual behavior — prompts and TTY handling, stdout/stderr separation, `--json`/`--dry-run`/`--yes` coverage, exit codes and error wording, idempotency, output volume. Exercise the real binary/bundle (e.g. `node dist/tu.mjs` after `npm run build`, or the repo's run path), not just source reading, for stream-split and exit-code claims.
- **skill**: verify the known state (no `tu skill` subcommand, no `docs/site/skill.md`) and report per §6 below.

### 2. Constitution: add the Toolkit Standards article

Add a `### Toolkit Standards` subsection under `## Additional Constraints` in `fab/project/constitution.md` (last subsection before `## Governance`), using wt's article text — the variant for repos that consume (rather than publish) the standards:

```markdown
### Toolkit Standards

This tool is part of the sahil87 toolkit and MUST conform to the toolkit's published standards. The standards are enumerated by running `shll standards` — each entry names what it governs; read one with `shll standards <name>`. Before changing the CLI surface, help output, README.md, or docs/site/, the change MUST be checked against the standards governing that surface. If shll is unavailable, the canonical sources are the sahil87/shll repository's docs/site/standards/ tree (rendered on https://shll.ai). Standards added or revised there bind this repo without further amendment to this constitution.
```

Governance line updates: **Version 1.0.0 → 1.1.0** (new article = minor bump, matching wt/shll which are both at 1.1.0 for the same amendment), **Last Amended → 2026-07-18**. This makes the task's premise true and is what mandates this conformance work going forward.

### 3. help-dump: remove `captured_at` (known mechanical violation) + full checklist

The producer lives at `src/node/core/help-dump.ts` (pinned by `src/node/core/__tests__/help-dump.test.ts`; backlog `[v76l]` also references `scripts/help-dump.mjs` / `npm run help-dump` — the apply agent locates all emit paths). Fix: stop emitting `captured_at`; the envelope becomes exactly `{tool, version, schema_version, root}`. Update the pinning test to assert `captured_at` is **absent**. shll.ai's puller stamps `captured_at` itself after capture (per the standard), so removal is what the consumer expects.

Then execute the standard's full verification checklist verbatim:

- `tu help-dump` exits 0, writes valid JSON to stdout only, stderr empty.
- Envelope is `{tool, version, schema_version, root}` — no `captured_at`.
- `completion`, `help`, and all hidden commands are absent from the tree (including `help-dump` itself).
- `version` reflects the built binary, not a literal.
- A minimal test pins the above (exit 0, valid JSON, expected `tool`/`schema_version`).

**The "tu exception" wrinkle**: the standard (at shll v0.0.23) documents tu as flag-based with no subcommands, emitting `root.commands: []`. tu now has real subcommands (`shell-init`, hidden `help-dump`, possibly others). Conformance is judged against the standard's invocation contract + verification checklist (which are shape-agnostic), not by flattening tu's tree to match stale prose. If tu's producer does not yet recurse its real subcommands, follow backlog `[v76l]`'s recorded intent ("If tu ever grows real subcommands, recurse the same way") **only if** the fix is small and additive; otherwise defer with a backlog reference. Either way, the report notes the standard's stale tu-exception prose as an upstream (shll repo) doc issue — not a tu violation.

### 4. README + docs/site: readme-extraction checklist verbatim

Execute the standard's checklist against `README.md` and `docs/site/**`:

- Head order: `#` H1 → canonical toolkit blockquote (exact line) → contiguous badges → tagline prose (verified conforming at intake — re-verify).
- Tail: everything site-worthy sits above the first footer heading (`Contributing`/`Development`/`Building`/`License`/`Acknowledgements`, case-insensitive `##`/`###`).
- Grep for relative targets (`](./`, `](../`, `](docs/`): each either points into `docs/site/` from the README, stays inside `docs/site/` between tree pages, or must be made absolute (`https://…`). No relative images anywhere; all images absolute.
- No ```` ```mermaid ```` fences destined for the site; no `#gh-dark-mode-only`/`#gh-light-mode-only` fragments.
- No `docs/site/` page named `overview`, `readme`, or `commands` (current pages: `install.md`, `workflows.md` — conforming).
- README cross-links its `docs/site/` pages (natural repo-relative form) and the generated command reference by absolute URL `https://shll.ai/tu/commands/`.
- Rule 7 (command/flag accuracy): spot-check README prose against tu's real flags/commands; fix stale mentions in the README.

All violations found are mechanical-contract violations → fixed in this change.

### 5. Principles: assess all ten, fix small additive gaps

Assess №1–№10 against tu's actual behavior. Known surface to exercise: the default read-only cost display (all sources/periods), `--json` output paths, `--sync` (git push — a mutation), `shell-init`, watch mode (TTY-dependent), `--no-color`, error paths (missing ccusage binaries, missing config — constitution's Graceful Degradation article covers №8), exit codes across success/operational-failure/usage-error, and behavior when stdout is not a TTY.

In scope (fix here): a missing flag that is a small addition, a status/hint line printed to stdout instead of stderr, an error message lacking the what/why/what-next triple, an undocumented exit code made consistent with the `0`/`1`/`2` convention.
Out of scope (defer with reference): anything restructuring — e.g. adding `--json` to a whole command family that lacks it, redesigning prompt/consent flows, reworking caching for statelessness (№6 tension with tu's constitution Article IV, which *mandates* caching with TTL — if these conflict, report the tension rather than "fixing" either side unilaterally).

### 6. skill standard: report as deferred

Report verbatim disposition: **"deferred, not yet adopted"** — per the toolkit's phased per-repo adoption (no seven-repo flag-day) and the standard's own text ("No tool ships `skill` today… A tool without a `skill` subcommand is not yet in violation"). Add a backlog entry (`fab/backlog.md`, convention `- [ ] [4char] YYYY-MM-DD: …`) for future adoption of `tu skill` + `docs/site/skill.md` (embedded, ≤150 lines, byte-identical drift-guarded), and reference that entry's ID in the report as the deferral target.

### 7. Deferred-gap convention

Every deferred gap (from any standard) gets a `fab/backlog.md` entry with a fresh 4-char ID and enough context to be actionable cold (this repo's convention — see existing entries like `[ccfx]`, `[gmcp]`). A gap large enough to be scheduled near-term work MAY instead be a `/fab-draft` change; backlog is the default. The conformance report references each deferral by its ID.

### 8. Deliverable: PR body conformance report

One fab change (this one); the PR body carries the report. Shape:

```markdown
## Conformance report

Audited against: shll v0.0.23 (`shll standards`, 2026-07-18)

### principles — <PASS | N gaps>
| № | Principle | Verdict | Disposition |
|---|-----------|---------|-------------|
| 1 | Non-interactive by default | PASS | — |
| 2 | stdout is data, stderr is diagnostics | gap: <specific> | fixed in <sha> |
…

### help-dump — <N gaps>
- `captured_at` emitted in envelope → fixed in <sha>
- <checklist item> → PASS
…

### readme-extraction — <PASS | N gaps>
…

### skill — deferred, not yet adopted
Phased per-repo adoption; tracked as backlog [<id>].
```

Every gap row is dispositioned either **fixed here (with the commit)** or **deferred to <backlog-id/draft>**. Tests green (`npm test`); if the command tree changed (e.g. help-dump shape work), re-run the help-dump verification checklist afterward.

## Affected Memory

- `build/toolchain`: (modify) — the help-dump producer contract changes (envelope drops `captured_at`; the file currently documents the producer shll.ai pulls); record the standards-conformance posture and the constitution's new Toolkit Standards article binding CLI/README/docs-site changes.
- `cli/data-pipeline`: (modify) — only if principle fixes touch argument parsing, stream routing, exit codes, or error surfaces (conditional on audit findings; skip if untouched).

## Impact

- **Source**: `src/node/core/help-dump.ts` (+ its test `src/node/core/__tests__/help-dump.test.ts`); possibly `scripts/help-dump.mjs` and `src/node/core/cli.ts` (small principle fixes: stream routing, error wording, exit codes).
- **Docs**: `README.md`, `docs/site/install.md`, `docs/site/workflows.md` (link/image/structure fixes per checklist).
- **Governance**: `fab/project/constitution.md` (new article, version 1.0.0 → 1.1.0).
- **Process**: `fab/backlog.md` (deferred-gap entries, incl. skill adoption).
- **External consumer**: shll.ai pulls `tu help-dump` output and README/docs-site — the changes move tu *toward* what its Zod validation and extraction lints expect; no consumer-side change is needed (pull model, tu pushes nothing).
- **Tests**: `npm test` (runs `find src/node -path '*/__tests__/*.test.ts' -exec npx tsx --test {} +`). Caveat from backlog: some suites are env-sensitive on configured dev boxes (`TU_METRICS_REPO` leakage — backlog entry of 2026-06-03); green on a clean env is the bar, and pre-existing env flakes are not this change's to fix.
- **Versioning**: constitution's Output Stability article — if any user-visible output format changes (help text wording is fine; table/JSON structure is not expected to change), it must ride a minor version bump. The `captured_at` removal changes `help-dump` JSON (a machine format): strictly a schema-visible change, but it is the *standard-mandated* shape and shll.ai is the only consumer; note it in the report and let the release flow decide the bump.

## Open Questions

*(none — the task text is unusually complete; all decision points resolved as graded assumptions below)*

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Audit scope is the runtime-enumerated `shll standards` list (4 standards at shll v0.0.23); no standards assumed from memory or website | Task states it explicitly; precondition verified on this machine at intake | S:95 R:90 A:95 D:95 |
| 2 | Certain | The skill standard is dispositioned "deferred, not yet adopted" — no `tu skill` implementation in this change | Task says so verbatim; the standard itself states no tool ships `skill` today | S:100 R:90 A:100 D:100 |
| 3 | Certain | Proportionality boundary: fix all mechanical-contract violations + small additive principle gaps here; defer restructuring gaps with references | Task defines the boundary explicitly with examples | S:90 R:80 A:85 D:85 |
| 4 | Certain | PR body carries the conformance report: one section per standard, PASS or per-gap disposition (fixed-with-commit / deferred-to-ref), plus the shll version row | Task specifies the deliverable; exact markdown formatting is agent's choice | S:90 R:95 A:90 D:85 |
| 5 | Certain | Tests green via `npm test`; help-dump checklist re-verified after any command-tree change | Task requirement; test command confirmed in package.json | S:95 R:90 A:95 D:90 |
| 6 | Confident | Add the Toolkit Standards article to tu's constitution (wt's text variant, verbatim), bumping 1.0.0 → 1.1.0, Last Amended 2026-07-18 | Task presupposes the article exists but it does not; wt and shll gained it today with this exact shape — tu is next in the same rollout | S:70 R:85 A:80 D:75 |
| 7 | Confident | Deferred gaps recorded as `fab/backlog.md` entries (4-char-ID convention) referenced from the report; `/fab-draft` only for near-term scheduled work | "Per this repo's convention" — backlog.md is the established queue (entries [ccfx], [gmcp], [sntl], [wkly]); GitHub issues are not used this way here | S:65 R:90 A:75 D:70 |
| 8 | Confident | Removing `captured_at` from the help-dump envelope is in scope and safe for the consumer | The standard forbids emitting it and assigns stamping to shll.ai's puller; tu's emission conforms to a superseded June contract | S:80 R:70 A:85 D:85 |
| 9 | Confident | The standard's stale "tu exception" prose (flat tree) is judged by the checklist, reported as an upstream doc issue — tu's tree is not flattened to match it; producer recursion of real subcommands is fixed here only if small, else deferred | Checklist + invocation contract are the testable surface; backlog [v76l] recorded "if tu ever grows real subcommands, recurse the same way" | S:50 R:75 A:60 D:55 |

9 assumptions (5 certain, 4 confident, 0 tentative, 0 unresolved).
