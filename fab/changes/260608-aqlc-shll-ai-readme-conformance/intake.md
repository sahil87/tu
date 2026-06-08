# Intake: Conform repo to shll.ai README-extraction contract

**Change**: 260608-aqlc-shll-ai-readme-conformance
**Created**: 2026-06-08
**Status**: Draft

## Origin

> Task: conform this repo to shll.ai's README-extraction contract.
>
> shll.ai (the toolkit landing page) renders your tool's page by mechanically pulling a slice of your
> README.md and your docs/site/** tree on a daily schedule — nothing is hand-copied, and you push
> nothing. Your job is to structure your repo so that pull renders cleanly.
>
> Read the contract and follow its §Producer conformance directive end-to-end:
> https://github.com/sahil87/shll.ai/blob/main/docs/specs/readme-extraction-contract.md
>
> 1. Find this repo's row in the directive's per-tool table (by repo name) for your slug and reserved page names.
> 2. Do Part 1 — restructure README.md: head order (# H1 → toolkit blockquote → badges → prose), drop
>    GitHub-footer sections below the tail denylist, make all images absolute https://… URLs, render
>    any mermaid to a committed image, and write any link that leaves the site as an absolute URL
>    (relative links 404 on the site).
> 3. Do Part 2 (optional but encouraged) — add a docs/site/**/*.md tree for depth (install guide,
>    deep-dives, etc.), following the four closed-set rules. Use docs/site/install.md /
>    docs/site/workflows.md for those pages.
> 4. Run the directive's Verify checklist before opening the PR.
>
> Ship it as a single PR in this repo. Do not touch shll.ai — it already pulls and renders automatically.

**Mode**: One-shot, externally-driven. No Linear ticket, no backlog ID (this task is not in
`fab/backlog.md`). The source of truth is the contract at
`sahil87/shll.ai:docs/specs/readme-extraction-contract.md`, fetched and read during intake.

**Relationship to prior shll.ai work** (verified, not assumed — both are DONE in this repo):

- `260602-v76l-help-dump-shll-ai` shipped the **command-reference** producer (`tu help-dump` →
  contract JSON: `{tool, version, captured_at, schema_version, root}`).
- `260603-dmhw-remove-shll-ai-push-wiring` removed the CI push half after shll.ai inverted to a
  **pull** model (its `scheduled-help-refresh.yml` cron `brew install`s tu, runs `tu help-dump`,
  validates, and direct-commits `help/tu.json` to its own `main`).

This change is a **different, complementary slice of the same pull model**: the README-extraction
contract governs how shll.ai pulls a slice of `README.md` + `docs/site/**` to render the *prose* tool
page (intro, install, workflows), whereas v76l/dmhw govern the *command reference* (JSON). They do not
overlap. **The `tu help-dump` command and its contract surface are explicitly out of scope here** — not
touched.

**Reconnaissance performed during intake** (decisions below are grounded in these findings):

1. **The contract was fetched and read.** Its §Producer conformance directive has a per-tool table, a
   six-rule Part 1 (README), a four-rule Part 2 (`docs/site/`), and a Verify checklist. tu's row:
   `tu` → binary `tu`, collector `content/tu/`, URL space `/tools/tu/`, reserved slugs
   `overview` / `readme` / `commands`.
2. **The current `README.md` is already ~95% conformant.** A grep audit confirms:
   - Head order is already correct: `# tu` (line 1) → toolkit blockquote (line 3, **already the exact
     canonical text**) → badge run (line 5) → prose (line 7).
   - **All images are already absolute** `https://…` URLs (shields.io badges on line 5; the screenshot
     `<img>` on line 10 uses `https://github.com/user-attachments/…`).
   - **No mermaid fences** anywhere (`grep '```mermaid'` → empty).
   - **No `#gh-dark-mode-only` / `#gh-light-mode-only`** theme fragments.
   - **All links are absolute** `https://…` (shll.ai, GitHub releases/stargazers, the `shll` cross-tool
     link on line 33). No relative links to source files / specs / internals.
   - **No denylisted footer headings** present (no `Contributing` / `Development` / `Building` /
     `License` / `Acknowledgements` as `##`/`###`). The `LICENSE` file lives standalone at repo root, so
     there is no License *section* to drop.
3. **The only Part 1 defect is a placeholder alt text.** The screenshot `<img>` on line 10 has
   `alt="image"`. The contract makes alt text **mandatory**; `"image"` satisfies presence but is a
   non-descriptive placeholder. It will be replaced with meaningful alt text.
4. **`docs/site/` does not exist.** The repo has `docs/memory/` and `docs/specs/` (fab-internal), but no
   `docs/site/`. Part 2 is greenfield.
5. **Authoritative command surface for docs content** is `FULL_HELP` in `src/node/core/cli.ts` (the
   exact string `tu --help` prints). Workflow/usage docs will be written to match it verbatim so the
   docs never drift from the shipped CLI.

## Why

1. **Problem**: shll.ai renders tu's public tool page at `/tools/tu/` by mechanically pulling a slice of
   this repo's `README.md` and `docs/site/**` tree on a daily schedule. If the repo is not structured to
   the contract, the pull renders incorrectly: relative links 404 on the site, relative/theme-scoped
   images break, mermaid fences vanish, and footer chrome (license boilerplate, contribution guides)
   leaks into the rendered page. The page is *generated*, not hand-curated — so the only lever tu has is
   to structure its own repo correctly.
2. **Consequence if not done**: tu's tool page on shll.ai is at best thin (README intro only, no depth
   pages) and at worst broken (placeholder alt text degrades accessibility; if future README edits add a
   relative link or mermaid diagram, the page silently breaks on the next daily pull). Every other tool
   in the 7-tool toolkit conforms; tu's page would be the inconsistent one. Because the pull is daily and
   automatic, a non-conformant repo produces a **persistently** wrong page with no manual remediation
   path on the site side.
3. **Why this approach**: The contract is explicit and closed-set — conforming is a mechanical
   restructuring, not a design problem. Part 1 (README) is nearly done already, so the work is (a) one
   small alt-text fix and (b) a verification pass against the checklist. Part 2 (`docs/site/`) is
   optional but encouraged and is where tu gains a richer page: an install guide and a workflows
   deep-dive render at `/tools/tu/install` and `/tools/tu/workflows`. Writing depth pages now (while the
   command surface is fresh in context) is cheaper than retrofitting later. We do **not** touch shll.ai
   (it pulls automatically) and we do **not** touch the `help-dump` JSON contract (a separate, already-
   shipped slice).

## What Changes

Two parts, both **purely documentation** — no `src/` change, no CLI behavior change, no test change. The
runtime binary, its output, and its grammar are untouched. The deliverable is a single PR.

### 1. Part 1 — `README.md` conformance (minimal: structure already correct)

The head order, canonical blockquote, absolute images, absent mermaid, absent theme fragments, absolute
links, and absent footer-denylist headings are **already conformant** (see Reconnaissance #2). Two edits:

**1a. Replace the placeholder alt text** on the screenshot `<img>` (line 10):

```diff
-<img width="1025" height="675" alt="image" src="https://github.com/user-attachments/assets/d6d1c930-8230-4910-ba1b-985e7df17e7c" />
+<img width="1025" height="675" alt="tu terminal output showing today's AI coding assistant costs across Claude Code, Codex, and OpenCode" src="https://github.com/user-attachments/assets/d6d1c930-8230-4910-ba1b-985e7df17e7c" />
```

The `src` is already absolute (`https://github.com/user-attachments/…`); only `alt` changes. Width/height
attributes are preserved.

**1b. Add natural `[text](docs/site/<file>.md)` links from the README into the new `docs/site/` pages**
(Part 2 rule 4). The site rewrites these to `/tools/tu/install` and `/tools/tu/workflows` automatically.
These links are written in the README **prose body** (NOT behind badges, NOT as reference-style
definitions — the Verify checklist forbids both placements). Concretely:

- In the `## Install` section, after the shell-completions block, add a one-line pointer:
  `For the full install and multi-machine setup walkthrough, see the [install guide](docs/site/install.md).`
- In the `## Usage` section, after the flags block, add:
  `For end-to-end recipes (daily snapshot, history pivots, multi-machine sync, watch mode), see
  [workflows](docs/site/workflows.md).`

These are the **only** relative links in the repo, and they point **into** `docs/site/`, which the
checklist explicitly allows.

**Everything else in the README stays verbatim** — `## Install`, `## Update`, `## Usage`, `### Flags`,
`### Setup (multi-machine sync)`. `Install` is explicitly **kept** by the tail rule (pulled to the site);
none of these are denylisted.

### 2. Part 2 — `docs/site/` depth tree (greenfield)

Create exactly two pages (the task names both; neither is a reserved slug). Each renders at
`/tools/tu/<basename>`:

#### `docs/site/install.md` → renders at `/tools/tu/install`

A complete install + setup walkthrough that expands on the README's terse blocks:

- **Homebrew install** (`brew tap sahil87/tap` → `brew install tu`), plus the `tu update` /
  `--skip-brew-update` story.
- **Shell completions** for bash / zsh / fish (verbatim from README, expanded with what each line does).
  Includes the cross-tool note that [`shll shell-install`](https://github.com/sahil87/shll#shll-shell-install--wire-the-rc-file-recommended)
  wires every sahil87 tool's completions at once (absolute URL — leaves the site).
- **Multi-machine sync setup**: `tu init-conf` → edit `~/.tu.conf` (show the `metrics_repo`, `machine`,
  `user`, `auto_sync` fields from the config schema in `src/node/core/cli.ts`) → `tu init-metrics` →
  `tu sync`. Explains single mode (local-only, default) vs multi mode (shared git metrics repo).

#### `docs/site/workflows.md` → renders at `/tools/tu/workflows`

A task-oriented recipe page derived **verbatim from `FULL_HELP`** (`src/node/core/cli.ts`) so it cannot
drift:

- **Grammar**: `tu [source] [period] [display]` — sources (`cc`, `codex`/`co`, `oc`, `all`), periods
  (`d`/`daily`, `m`/`monthly`), display (bare snapshot, `h`/`history`), combined (`dh`, `mh`).
- **Recipes**: today's cost (`tu`), per-tool (`tu cc`), daily history pivot (`tu h`), monthly history
  (`tu cc mh`), this month (`tu m`).
- **Output formats**: `--json`, `--csv`, `--md` (data commands only).
- **Watch mode**: `--watch`/`-w`, `--interval`/`-i`, `--no-rain` (the matrix-rain animation toggle).
- **Multi-machine**: `--sync`, `--user`/`-u`, `--by-machine`.
- **Cache control**: `--fresh`/`-f`.

All internal links within `docs/site/**` (e.g., workflows → install) are written relative and resolve
**inside** `docs/site/` (closure rule). Any link leaving the site (GitHub, shll.ai) is absolute
`https://…`. No images are added in Part 2 (so the "all images absolute" rule is trivially satisfied);
if a screenshot is ever added it must use an absolute URL.

### 3. Out of scope (explicit)

- **shll.ai itself** — never touched; it pulls automatically.
- **`tu help-dump` / `help/tu.json` / the command-reference JSON contract** — a separate slice, already
  shipped (`v76l`/`dmhw`). Not touched.
- **Any `src/`, test, or build change** — this is documentation-only.
- **`docs/memory/` and `docs/specs/`** — fab-internal trees, unrelated to the site pull, untouched.
- **Reserved slugs** — no `docs/site/` file named `overview`, `readme`, or `commands` (site-owned).

## Affected Memory

No memory updates. This change is documentation-only and alters no spec-level CLI behavior — it adds an
external-facing docs tree and polishes the README. The `docs/memory/**` tree (fab-internal) is unrelated
to the shll.ai site pull and is untouched. (`build/`, `cli/`, `sync/`, etc. memory domains describe the
implementation, which does not change.)

## Impact

- **`README.md`** — two surgical edits (alt text on line 10; two prose pointer-links into `docs/site/`).
  All other content verbatim.
- **`docs/site/install.md`** (new) — install + completions + multi-machine setup walkthrough.
- **`docs/site/workflows.md`** (new) — command/flag recipe page derived from `FULL_HELP`.
- **No code, tests, build, or CI** — documentation-only. The single-bundle distribution, fast startup,
  and data-model constitutional principles are unaffected.
- **External dependency**: correctness is validated against the contract's Verify checklist (run before
  the PR). The actual rendering happens on shll.ai's next daily pull — outside this repo's control, which
  is exactly why conforming the repo structure is the deliverable.

## Open Questions

None. The task is fully specified by the external contract; tu's row, reserved slugs, the two page names
(`install.md`, `workflows.md`), and the closed-set rules are all explicit. Content for the two docs pages
is derived from authoritative in-repo sources (`FULL_HELP`, the existing README, the config schema), not
invented.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | tu's slug is `tu`; reserved (forbidden) page slugs are `overview`/`readme`/`commands`; site URL space is `/tools/tu/` | Read directly from the contract's per-tool table (the `tu` row) — deterministic | S:98 R:95 A:95 D:98 |
| 2 | Certain | The two `docs/site/` pages are `install.md` and `workflows.md` (rendering at `/tools/tu/install` and `/tools/tu/workflows`) | The task names both files explicitly; neither is a reserved slug; the contract states `install`/`workflows` ARE allowed | S:98 R:90 A:95 D:95 |
| 3 | Certain | README head order, canonical blockquote, absolute images, no-mermaid, no-theme-fragments, absolute-links, and no-denylisted-footers are already conformant | Verified by direct grep audit of the current README (not inferred) | S:95 R:85 A:98 D:90 |
| 4 | Certain | The only Part 1 README fix is replacing placeholder `alt="image"` with descriptive alt text | Audit shows every other Part 1 rule already satisfied; alt presence is required and `"image"` is a non-descriptive placeholder | S:90 R:90 A:95 D:88 |
| 5 | Certain | The `tu help-dump` JSON command-reference contract (v76l/dmhw) is out of scope and untouched | The README-extraction contract is a distinct slice (prose page) from the help-dump contract (JSON); verified both prior changes are done and separate | S:95 R:80 A:95 D:92 |
| 6 | Certain | README `## Install`/`## Update`/`## Usage`/`### Flags`/`### Setup` sections are kept verbatim (not denylisted) | The tail rule explicitly keeps `Install`; none of these headings are on the denylist (Contributing/Development/Building/License/Acknowledgements) | S:95 R:88 A:95 D:95 |
| 7 | Confident | `docs/site/` content (install steps, workflow recipes) is derived verbatim from `FULL_HELP`, the existing README, and the config schema in `cli.ts` | Authoritative in-repo sources exist; one obvious interpretation (mirror the shipped CLI so docs can't drift). Reversible via later edit | S:80 R:75 A:88 D:82 |
| 8 | Confident | README→docs/site links are placed in prose body (not behind badges, not as reference-style definitions) and are the only relative links in the repo | The Verify checklist explicitly forbids the badge/reference-definition placements; prose placement is the prescribed form. Trivially reversible | S:82 R:85 A:90 D:85 |

8 assumptions (6 certain, 2 confident, 0 tentative, 0 unresolved).
