# Plan: Conform repo to shll.ai README-extraction contract

**Change**: 260608-aqlc-shll-ai-readme-conformance
**Status**: In Progress
**Intake**: `intake.md`

## Requirements

<!-- Docs-only change. Requirements scope: README conformance (Part 1), the two
     docs/site pages (Part 2), and the closed-set link/image rules that govern
     both. No src/, test, build, or CI requirements. -->

### README: shll.ai Part 1 Conformance

#### R1: Descriptive screenshot alt text
The README screenshot `<img>` MUST carry descriptive alt text (not the placeholder `alt="image"`), while preserving its existing absolute `src`, `width`, and `height` attributes. The contract makes alt text mandatory; `"image"` satisfies presence but is a non-descriptive placeholder.

- **GIVEN** the README screenshot `<img>` on line 10 with `alt="image"` and `src="https://github.com/user-attachments/assets/d6d1c930-8230-4910-ba1b-985e7df17e7c"`
- **WHEN** the alt text is replaced with `tu terminal output showing today's AI coding assistant costs across Claude Code, Codex, and OpenCode`
- **THEN** the `<img>` retains its absolute `src`, `width="1025"`, and `height="675"` unchanged
- **AND** no other attribute or README line is altered by this edit

#### R2: Natural prose links from README into docs/site
The README MUST link into the new `docs/site/` pages using the natural `[text](docs/site/file.md)` form, placed in prose body — NOT behind badges and NOT as reference-style link definitions (both placements are forbidden by the Verify checklist). The site rewrites these to `/tools/tu/install` and `/tools/tu/workflows`.

- **GIVEN** the README `## Install` section ending with the `> 💡 Have other sahil87 tools? …` blockquote and the `## Usage` section ending with the `### Setup (multi-machine sync)` code block
- **WHEN** a prose link to `docs/site/install.md` is added immediately after the Install-section blockquote, and a prose link to `docs/site/workflows.md` is added at the end of the README after the Setup code block
- **THEN** both links use the `[text](docs/site/file.md)` form in prose body (not behind a badge, not a reference-style definition)
- **AND** these are the only relative links in the README, and both point INTO `docs/site/`

#### R3: README structure preserved verbatim
Every README element already conformant — head order (`# tu` → toolkit blockquote → badges → prose), canonical blockquote, absolute images/links, absence of mermaid fences, absence of `#gh-dark-mode-only`/`#gh-light-mode-only` theme fragments, absence of denylisted footer headings (Contributing/Development/Building/License/Acknowledgements) — MUST remain byte-for-byte unchanged. The `## Install`/`## Update`/`## Usage`/`### Flags`/`### Setup` bodies MUST NOT be reordered or restructured.

- **GIVEN** the current README, whose head, blockquote, badges, and section bodies are already contract-conformant
- **WHEN** only the two edits from R1 and R2 are applied
- **THEN** all other README content is identical to before (no head reorder, no body restructure, no footer additions)

### docs/site: Install Guide

#### R4: install.md install + completions + multi-machine walkthrough
`docs/site/install.md` MUST exist and render at `/tools/tu/install`, covering: Homebrew install (`brew tap sahil87/tap`; `brew install tu`); updating (`tu update`, the `--skip-brew-update` behavior, and `brew upgrade tu`); shell completions for bash/zsh/fish reproducing the README's exact commands with an explanation of each; and multi-machine sync setup (`tu init-conf` → edit `~/.tu.conf` → `tu init-metrics` → `tu sync` → `tu status`) documenting the config schema fields (`version`, `metrics_repo`, `metrics_dir`, `machine`, `user`, `auto_sync`) from `src/node/core/cli.ts`, distinguishing single mode (local-only, default) from multi mode (shared git metrics repo). Content MUST be derived from the README and the config schema, never invented.

- **GIVEN** the README's terse Install/Shell-completions/Setup blocks and the `FIELD_BLOCKS` config schema in `src/node/core/cli.ts`
- **WHEN** `docs/site/install.md` is written
- **THEN** it documents brew install, `tu update`/`--skip-brew-update`/`brew upgrade tu`, the exact bash/zsh/fish completion commands, and the full `init-conf` → `init-metrics` → `sync` → `status` flow with all six config fields explained
- **AND** the `shll shell-install` cross-tool note links to `https://github.com/sahil87/shll#shll-shell-install--wire-the-rc-file-recommended` as an absolute URL (it leaves the site)

### docs/site: Workflows Recipes

#### R5: workflows.md derived verbatim from FULL_HELP
`docs/site/workflows.md` MUST exist and render at `/tools/tu/workflows`, deriving its commands and flags VERBATIM from `FULL_HELP` in `src/node/core/cli.ts` (no invented flags or commands). It MUST be organized into sections: the command grammar (source/period/display/combined), common recipes (the FULL_HELP Examples), output formats (`--json`/`--csv`/`--md`), watch mode (`--watch`/`-w`, `--interval`/`-i`, `--no-rain`), multi-machine (`--sync`, `--user`/`-u`, `--by-machine`), and cache control (`--fresh`/`-f`). It MUST contain no images.

- **GIVEN** the `FULL_HELP` string in `src/node/core/cli.ts` (the exact `tu --help` output)
- **WHEN** `docs/site/workflows.md` is written
- **THEN** every command, flag, source, period, and display token in it appears verbatim in `FULL_HELP` (no drift, nothing invented)
- **AND** it is organized into grammar / recipes / output-formats / watch / multi-machine / cache-control sections with no images

### Cross-cutting: Closed-Set Link & Image Rules

#### R6: Closed-set link and image discipline
Across README AND `docs/site/**`, the contract's closed-set rules MUST hold: every relative link/image inside `docs/site/**` resolves INSIDE `docs/site/` (no `..` escapes); every image is an absolute `https://…` URL with alt text; every link leaving the rendered site is absolute `https://…`; README→docs/site links use the `[text](docs/site/file.md)` natural form; and NO `docs/site/` file is named `overview`, `readme`, or `commands` (reserved slugs) — only `install.md` and `workflows.md` are created. A relative link from `workflows.md` to `install.md` is permitted because it resolves inside `docs/site/`.

- **GIVEN** the conformed README and the two new `docs/site/` pages
- **WHEN** the repo is grepped for relative links/images and theme fragments
- **THEN** no relative image exists anywhere; no relative link inside `docs/site/**` escapes the folder; every site-leaving link is absolute `https://…`; no reserved-slug file exists in `docs/site/`
- **AND** the only relative links are README→`docs/site/*.md` (R2) and an optional intra-`docs/site/` `workflows.md`→`install.md` link

### Non-Goals

- shll.ai itself — never touched; it pulls and renders automatically.
- `tu help-dump` / `help/tu.json` / the command-reference JSON contract — a separate, already-shipped slice (v76l/dmhw). Not touched.
- Any `src/`, test, build, or CI change — this change is documentation-only.
- `docs/memory/` and `docs/specs/` — fab-internal trees, unrelated to the site pull, untouched.
- Reserved slugs `overview`/`readme`/`commands` — site-owned, never created.
- New images in Part 2 — none added (the "all images absolute" rule is thereby trivially satisfied).

### Design Decisions

1. **Docs content derived from in-repo authoritative sources, not invented**: `workflows.md` mirrors `FULL_HELP` verbatim; `install.md` expands the README blocks and the `FIELD_BLOCKS` config schema. — *Why*: the docs can never drift from the shipped CLI. — *Rejected*: writing fresh prose from memory (risks documenting flags/commands that do not exist).
2. **README edits are surgical (two changes only)**: the audit confirmed the head, blockquote, badges, images, links, and footers are already conformant. — *Why*: minimize blast radius; preserve a known-good structure. — *Rejected*: a full README restructure (unnecessary, risks regressing already-conformant elements).

## Tasks

### Phase 1: README conformance

- [x] T001 In `/home/sahil/code/sahil87/tu.worktrees/glossy-ridge/README.md`, replace ONLY the `alt="image"` attribute on the screenshot `<img>` (line 10) with `alt="tu terminal output showing today's AI coding assistant costs across Claude Code, Codex, and OpenCode"`, keeping `width`, `height`, and the absolute `src` unchanged. <!-- R1 -->
- [x] T002 In `/home/sahil/code/sahil87/tu.worktrees/glossy-ridge/README.md`, add two natural prose links: (a) in `## Install`, immediately after the `> 💡 Have other sahil87 tools? …` blockquote, a blank line then `For the full install and multi-machine setup walkthrough, see the [install guide](docs/site/install.md).`; (b) at the very end of the README (after the closing fence of the `### Setup (multi-machine sync)` code block), a blank line then `For end-to-end recipes — daily snapshots, history pivots, multi-machine sync, and watch mode — see [workflows](docs/site/workflows.md).` Leave all other README content byte-for-byte identical. <!-- R2 R3 -->

### Phase 2: docs/site pages

- [x] T003 [P] Create `/home/sahil/code/sahil87/tu.worktrees/glossy-ridge/docs/site/install.md` (renders at `/tools/tu/install`): Homebrew install (`brew tap sahil87/tap`, `brew install tu`); updating (`tu update`, `--skip-brew-update` skips the `brew update` tap refresh, `brew upgrade tu`); bash/zsh/fish completions reproducing the README's exact commands with an explanation of each, plus the absolute `https://github.com/sahil87/shll#shll-shell-install--wire-the-rc-file-recommended` cross-tool note; multi-machine sync setup (`tu init-conf` → edit `~/.tu.conf` → `tu init-metrics` → `tu sync` → `tu status`) documenting the six config fields (`version`, `metrics_repo`, `metrics_dir`, `machine`, `user`, `auto_sync`) from `src/node/core/cli.ts`, contrasting single mode (default, local-only) vs multi mode. No images. <!-- R4 R6 -->
- [x] T004 [P] Create `/home/sahil/code/sahil87/tu.worktrees/glossy-ridge/docs/site/workflows.md` (renders at `/tools/tu/workflows`), derived VERBATIM from `FULL_HELP` in `src/node/core/cli.ts`. Sections: command grammar (source/period/display/combined), common recipes (the Examples), output formats (`--json`/`--csv`/`--md`), watch mode (`--watch`/`-w`, `--interval`/`-i`, `--no-rain`), multi-machine (`--sync`, `--user`/`-u`, `--by-machine`), cache control (`--fresh`/`-f`). MAY include a relative `[install guide](install.md)` link (resolves inside `docs/site/`); any site-leaving link MUST be absolute `https://…`. No images. <!-- R5 R6 -->

### Phase 3: Closed-set self-verification

- [x] T005 Self-verify the closed-set contract: grep README + `docs/site/**` for relative images (none allowed), relative links inside `docs/site/**` that escape the folder (none allowed), site-leaving links that are not absolute `https://…` (none allowed), `#gh-dark-mode-only`/`#gh-light-mode-only` theme fragments (none allowed), and reserved-slug files (`overview`/`readme`/`commands`) in `docs/site/` (none allowed). Confirm the only relative links are README→`docs/site/*.md` and the optional `workflows.md`→`install.md`. <!-- R6 -->

## Execution Order

- T001 and T002 both edit README.md — run sequentially (T001 then T002).
- T003 and T004 are independent new files — `[P]`, may run in parallel.
- T005 runs last (verifies the output of T001-T004).

## Acceptance

### Functional Completeness

- [x] A-001 R1: The README screenshot `<img>` has descriptive alt text (`tu terminal output showing today's AI coding assistant costs across Claude Code, Codex, and OpenCode`) with its absolute `src`, `width="1025"`, and `height="675"` preserved.
- [x] A-002 R2: The README contains exactly two prose links into `docs/site/` — `[install guide](docs/site/install.md)` after the Install-section blockquote and `[workflows](docs/site/workflows.md)` at the end of the README — both in prose body, neither behind a badge nor as a reference-style definition.
- [x] A-003 R3: All README content other than the R1/R2 edits is unchanged (head order, blockquote, badges, and Install/Update/Usage/Flags/Setup bodies are byte-for-byte identical to before).
- [x] A-004 R4: `docs/site/install.md` exists and covers brew install, `tu update`/`--skip-brew-update`/`brew upgrade tu`, the exact bash/zsh/fish completion commands with explanations, the `shll shell-install` absolute cross-tool note, and the full `init-conf`→`init-metrics`→`sync`→`status` multi-machine flow with all six config fields explained and single-vs-multi mode distinguished.
- [x] A-005 R5: `docs/site/workflows.md` exists and is organized into grammar / recipes / output-formats / watch / multi-machine / cache-control sections.

### Behavioral Correctness

- [x] A-006 R5: Every command, flag, source, period, and display token in `docs/site/workflows.md` appears verbatim in `FULL_HELP` (`src/node/core/cli.ts`) — nothing invented, no drift.
- [x] A-007 R4: The config fields documented in `docs/site/install.md` match the `FIELD_BLOCKS` schema in `src/node/core/cli.ts` (names, defaults, and the `auto_sync` "no longer auto-triggers" note).

### Scenario Coverage

- [x] A-008 R6: Grepping README + `docs/site/**` shows no relative image, no `docs/site/**` relative link escaping the folder, and every site-leaving link absolute `https://…`.

### Edge Cases & Error Handling

- [x] A-009 R6: No `docs/site/` file is named `overview`, `readme`, or `commands`; only `install.md` and `workflows.md` exist. No `#gh-dark-mode-only`/`#gh-light-mode-only` theme fragments appear anywhere.
- [x] A-010 R6: The only relative links in the repo are README→`docs/site/*.md` (two) and an optional intra-`docs/site/` `workflows.md`→`install.md` link; no relative links point at source files, specs, or internals.

### Code Quality

- [x] A-011 Pattern consistency: New docs follow Markdown conventions consistent with the existing README (fenced code blocks with language hints, sentence-case headings).
- [x] A-012 No unnecessary duplication: Completion commands and config fields are reproduced from their authoritative sources (README, `cli.ts`) rather than paraphrased into a drifting variant.

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- No automated tests apply — this change touches no code. The constitution's "Test Integrity" is about code specs (N/A here). Self-verification is the grep-based closed-set check in T005.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | tu's slug is `tu`; reserved page slugs are `overview`/`readme`/`commands`; site URL space is `/tools/tu/`; the two pages are `install.md` and `workflows.md` | Read directly from the contract's per-tool table and the task spec; deterministic | S:98 R:95 A:95 D:98 |
| 2 | Certain | The only Part 1 README fix is replacing placeholder `alt="image"` with descriptive alt text; head/blockquote/badges/links/footers are already conformant | Verified by the intake's direct grep audit and re-confirmed by reading the README | S:95 R:88 A:98 D:92 |
| 3 | Certain | README→docs/site links go in prose body (not behind badges, not reference-style); they are the only relative links in the README | The Verify checklist forbids the badge/reference placements; prose is the prescribed form | S:90 R:85 A:92 D:90 |
| 4 | Confident | `docs/site/` content is derived verbatim from `FULL_HELP`, the README, and the `FIELD_BLOCKS` config schema in `cli.ts` | Authoritative in-repo sources exist; one obvious interpretation (mirror the shipped CLI so docs can't drift); reversible via later edit | S:82 R:75 A:88 D:82 |
| 5 | Confident | `workflows.md` includes a relative `[install guide](install.md)` link (resolves inside `docs/site/`, allowed by the closure rule) | The task explicitly permits this intra-folder link; it adds navigational value with no contract risk; trivially reversible | S:80 R:88 A:90 D:85 |
| 6 | Confident | install.md's `tu update` section also documents `brew upgrade tu` and the `--skip-brew-update` semantics as alternatives, mirroring the README's `## Update` block comments | The README `## Update` block lists `brew update`/`brew upgrade tu`; `--skip-brew-update` is described verbatim in `FULL_HELP`; one obvious expansion | S:82 R:85 A:88 D:84 |

6 assumptions (3 certain, 3 confident, 0 tentative).
