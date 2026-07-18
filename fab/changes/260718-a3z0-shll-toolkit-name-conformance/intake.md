# Intake: shll Toolkit Name Conformance

**Change**: 260718-a3z0-shll-toolkit-name-conformance
**Created**: 2026-07-18

## Origin

One-shot `/fab-new` invocation with a fully-specified task:

> Task: Conform this repo to the toolkit's standardized name — "shll toolkit".
>
> The toolkit formerly named "sahil87 toolkit" is now the **shll toolkit** (sahil87/shll#56). The readme-extraction standard's canonical README blockquote changed accordingly. This repo's constitution already binds it to revised standards without amendment — this task is the conformance work.
>
> 1. **README blockquote** — replace with the exact line, byte-identical, keeping head order (H1 → blockquote → badges): `> Part of the [shll toolkit](https://shll.ai) — see all projects there.`
> 2. **Prose sweep** — `sahil87 toolkit` → `shll toolkit`, `sahil87 tool(s)` → `shll tool(s)` wherever they appear as prose: README, `docs/site/**`, CLI help text and user-visible strings (+ test goldens), `fab/project/` files. Re-run doc-embed sync if applicable.
> 3. **Constitution (cosmetic, same PR)** — Toolkit Standards article: "part of the sahil87 toolkit" → "part of the shll toolkit"; bump `Last Amended`. Nothing else in the article changes.
> 4. **Do NOT touch identifiers**: `sahil87/tap`, `github.com/sahil87/…`, `raw.githubusercontent.com/sahil87/…`, the `sahil87/shll` canonical-source reference, GitHub-owner constants. `fab/changes/` archives stay untouched.

**Precondition verified at intake** (2026-07-18): `shll standards readme-extraction` runs on this machine and shows the new canonical blockquote byte-identically: `> Part of the [shll toolkit](https://shll.ai) — see all projects there.` — no `shll update` needed; proceeding is authorized (not from memory).

**Repo-wide occurrence scan completed at intake** (results in What Changes): every prose occurrence of the old name was located with `grep -rniE 'sahil87 (toolkit|tools?)'` plus a variant sweep for the old blockquote phrasing (`@sahil87's open source toolkit`, which the simple pattern does not match).

## Why

The toolkit was renamed upstream: "sahil87 toolkit" → "shll toolkit" (sahil87/shll#56), and the readme-extraction standard's canonical README blockquote changed with it. The constitution's Toolkit Standards article (v1.1.0) states that standards revised in the canonical source **bind this repo without further amendment** — so tu is already non-conformant until this sweep lands.

Consequences of not fixing: (a) shll.ai's daily README pull renders the outdated blockquote on `/tu/readme` — the blockquote is a byte-exact contract across all seven toolkit repos; (b) stale "sahil87 toolkit" prose contradicts the toolkit's published identity in README, specs, memory, and code comments.

Approach: a single mechanical rename sweep in one fab change → one PR, per the task. No behavior changes anywhere; identifiers (GitHub owner paths, tap names) are deliberately preserved because they are addresses, not prose.

## What Changes

### 1. README.md — blockquote (byte-exact) + two prose lines

- **Line 3** (current): `> Part of [@sahil87's open source toolkit](https://shll.ai) — see all projects there.`
  → replace with the canonical line, byte-identical:
  `> Part of the [shll toolkit](https://shll.ai) — see all projects there.`
  Head order is already H1 (`# tu`) → blockquote → badges → prose and MUST remain exactly that (readme-extraction standard rule 1). Only the blockquote line's content changes.
- **Line 16**: `To install the entire sahil87 toolkit instead:` → `To install the entire shll toolkit instead:`
- **Line 39**: `> 💡 Have other sahil87 tools? …` → `> 💡 Have other shll tools? …` (rest of line unchanged, including the `github.com/sahil87/shll` URL — identifier, untouched).

### 2. docs/specs/usage.md — one prose line

- **Line 84**: `` `tu` follows the sahil87 toolkit convention (principle №4 — *fail fast with actionable errors*): `` → `sahil87 toolkit` becomes `shll toolkit`.

### 3. docs/memory — two prose occurrences (present-truth docs, not historical artifacts)

- **`docs/memory/build/toolchain.md:58`**: `tu is a member of the sahil87 toolkit` → `… member of the shll toolkit`.
- **`docs/memory/cli/data-pipeline.md:37`**: `the CLI MUST follow the sahil87 toolkit convention` → `… the shll toolkit convention`.

### 4. Code comments (not user-visible strings — comments only)

- **`src/node/core/cli.ts:67`**: `// Exit-code convention (sahil87 toolkit principle №4): 0 = success,` → `(shll toolkit principle №4)`.
- **`src/node/core/__tests__/cli-exit-codes.test.ts:8`**: `// End-to-end exit-code contract (sahil87 toolkit principle №4):` → `(shll toolkit principle №4)`.

### 5. fab/project/constitution.md — Toolkit Standards article (cosmetic amendment)

- **Line 44**: `This tool is part of the sahil87 toolkit and MUST conform …` → `This tool is part of the shll toolkit and MUST conform …`. The same line's `the canonical sources are the sahil87/shll repository's docs/site/standards/ tree` stays untouched (canonical-source reference — task item 4).
- **Governance line (line 48)**: set `Last Amended` to today, 2026-07-18 — which equals the current value, so the line is byte-unchanged. `Version` stays 1.1.0 (task mandates only the `Last Amended` bump; nothing else in the article changes).

### 6. Verified non-changes (scanned at intake — nothing to do)

- **`docs/site/**`** (`install.md`, `skill.md`, `workflows.md`): zero occurrences of the old name in any phrasing. Therefore the skill bundle needs **no re-sync**: `tu skill`'s content is injected at build time via esbuild `--define:__SKILL_MD__` reading `docs/site/skill.md` directly (no generated/committed intermediate file), and the drift-guard test (`src/node/core/__tests__/skill.test.ts`) reads the canonical file under tsx — byte-identity is by construction.
- **CLI help text / user-visible strings**: zero occurrences of `sahil87 toolkit|tool(s)` in any user-visible string (verified by grep over `src/`). Help output is unchanged → `tu help-dump` output unchanged → no golden updates, JSON shape untouched, **no `schema_version` bump**.
- **Identifiers preserved everywhere**: `sahil87/tap`, `github.com/sahil87/…`, `raw.githubusercontent.com/sahil87/…`, `sahil87/shll` canonical-source reference, GitHub-owner constants.
- **`fab/changes/`** historical artifacts: untouched.
- **`dist/`**: not committed (nothing to rebuild in-repo).

## Affected Memory

- `build/toolchain`: (modify) rename "sahil87 toolkit" → "shll toolkit" in the toolkit-standards conformance posture entry (edited directly as part of the sweep; hydrate verifies and may add a note that the toolkit was renamed upstream)
- `cli/data-pipeline`: (modify) rename in the Exit-Code Convention entry (same treatment)

## Impact

- **Files touched (7)**: `README.md`, `docs/specs/usage.md`, `docs/memory/build/toolchain.md`, `docs/memory/cli/data-pipeline.md`, `src/node/core/cli.ts` (comment), `src/node/core/__tests__/cli-exit-codes.test.ts` (comment), `fab/project/constitution.md`.
- **Behavior**: none — prose and comments only. No runtime strings, no help output, no JSON shapes, no exit codes.
- **Tests**: full suite must stay green; no goldens change. The two comment edits cannot affect test behavior.
- **Versioning**: no package version bump needed (no output change; Output Stability rule not triggered).
- **Conformance check**: post-edit, README head must still satisfy readme-extraction rule 1 (H1 → blockquote → badges) and the blockquote must be byte-identical to the standard's canonical line.

## Open Questions

*(none — the task is fully specified and the precondition was verified live)*

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | README blockquote replaced byte-identically with the canonical line from the live standard; head order H1 → blockquote → badges preserved | Task gives the exact line; `shll standards readme-extraction` verified live at intake and matches byte-for-byte | S:95 R:90 A:100 D:100 |
| 2 | Certain | `docs/site/**` needs no edits and the skill bundle needs no re-sync | Verified: zero old-name occurrences in `docs/site/**`; embed is a build-time esbuild define reading `docs/site/skill.md` directly — no committed intermediate to drift | S:90 R:95 A:100 D:95 |
| 3 | Certain | No CLI help/user-visible string changes → no test-golden updates, no help-dump change, no `schema_version` bump | Verified by grep over `src/`: the only source occurrences are two code comments | S:90 R:95 A:100 D:95 |
| 4 | Confident | Sweep extends to `docs/specs/usage.md` and the two `docs/memory` files even though the task's location list doesn't name them | "wherever they appear as prose" is the operative instruction; these are present-truth docs, not historical artifacts (only `fab/changes/` is excluded) | S:60 R:90 A:80 D:60 |
| 5 | Confident | The two code comments (`cli.ts:67`, `cli-exit-codes.test.ts:8`) count as prose and are updated | Comments are prose references to the toolkit's name, not identifiers; zero behavioral risk | S:65 R:95 A:80 D:70 |
| 6 | Confident | Constitution `Last Amended` set to today (2026-07-18) — equal to current value, so the governance line is byte-unchanged; `Version` stays 1.1.0 | Task says bump `Last Amended` and change nothing else; today already equals the recorded date (the standards-binding amendment landed earlier today) | S:70 R:90 A:80 D:70 |

6 assumptions (3 certain, 3 confident, 0 tentative, 0 unresolved).
