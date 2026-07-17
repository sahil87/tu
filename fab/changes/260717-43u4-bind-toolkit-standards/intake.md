# Intake: Bind Constitution to Toolkit Standards

**Change**: 260717-43u4-bind-toolkit-standards
**Created**: 2026-07-18

## Origin

One-shot `/fab-new` invocation with a fully-specified task description (verbatim below). No prior conversation; all decisions were provided in the input itself.

> Task: Amend this repo's fab constitution to bind it to the sahil87 toolkit standards. This repo is part of the sahil87 toolkit. The toolkit publishes binding, producer-facing standards — CLI design principles plus mechanical contracts (machine-readable help output, README/docs-site structure, and others over time). They are canonically authored in the sahil87/shll repository's docs/site/standards/ tree, rendered on https://shll.ai, and readable offline via the `shll standards` command. This change adds a constitution article so every future pipeline run in this repo loads and enforces the obligation.
>
> Make this change:
>
> 1. In fab/project/constitution.md, add a new article under Additional Constraints (create the section if this constitution lacks it, matching the file's existing structure): [exact article text — reproduced in full under What Changes]
> 2. Bump the constitution's Last Amended date (and version, per this file's own governance line).
> 3. Deliberate constraint: do NOT copy standard names, counts, or per-standard URLs into the constitution — `shll standards` is the enumeration, and the article must stay correct as standards evolve.
>
> Ship per this repo's normal flow (docs-type fab change → PR). Nothing else is in scope — no conformance fixes in this change.

## Why

1. **Problem**: tu is part of the sahil87 toolkit, which publishes binding, producer-facing standards (CLI design principles plus mechanical contracts such as machine-readable help output and README/docs-site structure). tu already conforms to two of these contracts mechanically (`tu help-dump` from change `v76l`; README/docs-site structure from change `aqlc`), but nothing in this repo's governance *obligates* conformance. The fab constitution is loaded by every pipeline run (it is part of the always-load context layer and the standard subagent context), so it is the one file that makes an obligation binding on all future changes.
2. **Consequence of not fixing**: future changes to the CLI surface, help output, README.md, or docs/site/ can silently drift from the toolkit standards — agents running the pipeline have no instruction to check them, and conformance erodes one PR at a time.
3. **Why this approach**: a constitution article is enforced automatically (every stage agent reads the constitution), survives as standards evolve (it points at the enumeration command `shll standards` rather than copying a snapshot), and requires no tooling changes. Alternatives like copying the standards into this repo or listing them in the article were deliberately rejected — a copied list goes stale, whereas the pull-based pointer stays correct as standards are added or revised upstream (consistent with the toolkit's existing pull-model integration: shll.ai pulls from tu; tu pushes nothing).

## What Changes

### fab/project/constitution.md — new article under Additional Constraints

The constitution already has an `## Additional Constraints` section (articles: Test Integrity, Test Runner, Test Location, Output Stability). Append a new `### Toolkit Standards` article as the last article in that section, immediately before `## Governance`, matching the existing `### {Title}` + prose structure. Exact article text (em dashes rendered as `—`, matching the repo's prose style):

```markdown
### Toolkit Standards

This tool is part of the sahil87 toolkit and MUST conform to the toolkit's published standards. The standards are enumerated by running `shll standards` — each entry names what it governs; read one with `shll standards <name>`. Before changing the CLI surface, help output, README.md, or docs/site/, the change MUST be checked against the standards governing that surface. If shll is unavailable, the canonical sources are the sahil87/shll repository's docs/site/standards/ tree (rendered on https://shll.ai). Standards added or revised there bind this repo without further amendment to this constitution.
```

<!-- assumed: the task text's `--` sequences are plain-text stand-ins for em dashes; rendered as `—` in the committed article -->

### fab/project/constitution.md — governance line bump

Current governance line:

```markdown
**Version**: 1.0.0 | **Ratified**: 2026-03-06 | **Last Amended**: 2026-03-06
```

New governance line:

```markdown
**Version**: 1.1.0 | **Ratified**: 2026-03-06 | **Last Amended**: 2026-07-18
```

Minor version bump (1.0.0 → 1.1.0) because a new article is added; the governance line itself declares no bump rules, so standard constitution semver applies (major = removals/redefinitions, minor = new article/section, patch = wording). Ratified date is unchanged.

### Deliberate constraints (binding on apply)

- Do NOT copy standard names, counts, or per-standard URLs into the constitution. `shll standards` is the enumeration; the article must stay correct as standards evolve. The only literals permitted are the ones in the exact article text above (`shll standards`, `shll standards <name>`, the sahil87/shll repo's docs/site/standards/ tree, https://shll.ai).
- No conformance fixes in this change — even if the current CLI surface, help output, README.md, or docs/site/ is found non-conformant, fixing that is out of scope. This change only installs the obligation.
- No other files change (no src/, no README.md, no docs/site/, no tests — there is nothing to test; this is a governance-document amendment).

## Affected Memory

None. The constitution is fab pipeline governance, not product behavior — no `docs/memory/` domain (build, cli, configuration, display, sync, watch-mode) documents it, and no spec-level product behavior changes.

## Impact

- **Files**: `fab/project/constitution.md` only (one new article, one governance-line edit).
- **Runtime/product impact**: none — no code, output, or distribution changes.
- **Pipeline impact**: every future fab pipeline run in this repo loads the amended constitution (always-load layer + standard subagent context), so all future changes touching the CLI surface, help output, README.md, or docs/site/ become obligated to check `shll standards` first.
- **Ship flow**: normal docs-type flow — apply → review → hydrate → ship (PR) → review-pr.

## Open Questions

None — the task description specified the article text, placement, versioning obligation, and scope constraints explicitly.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Append the article verbatim (as provided) as the last `###` article under the existing `## Additional Constraints`, before `## Governance` | Text supplied in full; section exists; "matching the file's existing structure" makes placement the one obvious reading | S:95 R:90 A:95 D:95 |
| 2 | Confident | Render the task text's `--` sequences as em dashes (`—`) in the committed article | `--` in CLI-passed prose is a plain-text stand-in; repo prose (docs, memory, config comments) consistently uses `—`; trivially reversible | S:60 R:95 A:80 D:70 |
| 3 | Confident | Version bump 1.0.0 → 1.1.0 (minor) | Governance line declares no bump rules; standard constitution semver treats a new article as minor — clear front-runner over patch | S:70 R:95 A:75 D:70 |
| 4 | Certain | Last Amended → 2026-07-18; Ratified unchanged | Task says bump Last Amended; today's date; ratification date is historical fact | S:95 R:95 A:100 D:100 |
| 5 | Certain | change_type = docs; scope is constitution.md only — no conformance fixes, no code, no tests | Explicitly stated in the task ("docs-type fab change → PR", "no conformance fixes in this change") | S:100 R:90 A:95 D:95 |
| 6 | Certain | No Affected Memory entries | Constitution is fab governance, outside all product-behavior memory domains; hydrate has nothing to update | S:80 R:90 A:90 D:90 |

6 assumptions (4 certain, 2 confident, 0 tentative, 0 unresolved).
