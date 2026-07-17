# Plan: Bind Constitution to Toolkit Standards

**Change**: 260717-43u4-bind-toolkit-standards
**Intake**: `intake.md`

## Requirements

### Constitution: Toolkit Standards Article

#### R1: Toolkit Standards article present under Additional Constraints
The constitution MUST carry a `### Toolkit Standards` article as the **last** article in the `## Additional Constraints` section, immediately before `## Governance`. Its text MUST be the exact prose supplied in the intake's What Changes section, with the `--` sequences rendered as em dashes (`—`), matching the repo's existing prose style. The article MUST bind conformance to the toolkit standards enumerated by `shll standards`, obligate a check before changing the CLI surface / help output / README.md / docs/site/, and name the canonical offline sources (the sahil87/shll `docs/site/standards/` tree, rendered on https://shll.ai).

- **GIVEN** the current constitution with articles Test Integrity, Test Runner, Test Location, Output Stability under `## Additional Constraints`
- **WHEN** the change is applied
- **THEN** a `### Toolkit Standards` article appears as the last `###` article in `## Additional Constraints`, positioned immediately before `## Governance`
- **AND** its body matches the intake's exact article text verbatim, with `—` (em dash) wherever the intake source shows `—`

#### R2: No standard names, counts, or per-standard URLs copied in
The article MUST NOT enumerate individual standard names, a standard count, or per-standard URLs. The only literals permitted are those in the exact article text: `shll standards`, `shll standards <name>`, the sahil87/shll `docs/site/standards/` tree reference, and https://shll.ai. This keeps the article correct as the upstream standards set evolves (pull-based pointer, not a copied snapshot).

- **GIVEN** the supplied article text
- **WHEN** the article is written into the constitution
- **THEN** it contains no standard names, no numeric count of standards, and no per-standard URLs beyond the permitted literals above

#### R3: Governance line version + Last Amended bump
The governance line MUST be updated from `**Version**: 1.0.0 | **Ratified**: 2026-03-06 | **Last Amended**: 2026-03-06` to `**Version**: 1.1.0 | **Ratified**: 2026-03-06 | **Last Amended**: 2026-07-18`. The minor bump (1.0.0 → 1.1.0) reflects the addition of a new article; the Ratified date is unchanged.

- **GIVEN** the existing governance line at version 1.0.0
- **WHEN** the change is applied
- **THEN** the governance line reads exactly `**Version**: 1.1.0 | **Ratified**: 2026-03-06 | **Last Amended**: 2026-07-18`

### Non-Goals

- Conformance fixes to the CLI surface, help output, README.md, or docs/site/ — installing the obligation is in scope; acting on it is not.
- Any change to files other than `fab/project/constitution.md` (no src/, no README.md, no docs/site/, no tests).
- Copying, listing, or snapshotting the toolkit's standards into this repo — the article points at `shll standards` deliberately.

### Design Decisions

1. **Pull-based pointer, not a copied list**: the article references the `shll standards` enumeration rather than naming individual standards — *Why*: a copied list goes stale as standards are added/revised upstream; the pointer stays correct without re-amending the constitution — *Rejected*: copying the standards into this repo or listing them in the article (both drift).

## Tasks

### Phase 1: Core Implementation

- [x] T001 Add the `### Toolkit Standards` article as the last `###` article under `## Additional Constraints` in `fab/project/constitution.md`, immediately before `## Governance`, using the intake's exact article text with `--` rendered as em dashes (`—`) <!-- R1 -->
- [x] T002 Update the governance line in `fab/project/constitution.md` to `**Version**: 1.1.0 | **Ratified**: 2026-03-06 | **Last Amended**: 2026-07-18` <!-- R3 -->
- [x] T003 Verify the written article contains no standard names, counts, or per-standard URLs beyond the permitted literals, and that no file other than `fab/project/constitution.md` was modified <!-- R2 -->

## Acceptance

### Functional Completeness

- [x] A-001 R1: `fab/project/constitution.md` contains a `### Toolkit Standards` article as the last article under `## Additional Constraints`, immediately before `## Governance`, with body text matching the intake verbatim (em dashes as `—`)
- [x] A-002 R3: The governance line reads exactly `**Version**: 1.1.0 | **Ratified**: 2026-03-06 | **Last Amended**: 2026-07-18`

### Behavioral Correctness

- [x] A-003 R2: The article enumerates no standard names, no standard count, and no per-standard URLs — only the permitted literals (`shll standards`, `shll standards <name>`, the sahil87/shll `docs/site/standards/` tree, https://shll.ai) appear

### Scenario Coverage

- [x] A-004 R1: The article's obligation covers changing the CLI surface, help output, README.md, and docs/site/, and names the offline canonical sources

### Code Quality

- [x] A-007 Pattern consistency: The new article matches the file's existing `### {Title}` + prose structure and its em-dash usage
- [x] A-008 No unnecessary duplication: No file other than `fab/project/constitution.md` is changed; the standards are referenced by pointer, not duplicated

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | Render the intake/task text's `--` sequences as em dashes (`—`) in the committed article | `--` in CLI-passed prose is a plain-text stand-in; repo prose (docs, memory, config comments) consistently uses `—`; trivially reversible; carried from intake assumption 2 | S:60 R:95 A:80 D:70 |
| 2 | Certain | Version bump 1.0.0 → 1.1.0 (minor) for a new article; Ratified unchanged, Last Amended → 2026-07-18 | Intake specifies the exact new governance line; standard constitution semver treats a new article as minor | S:95 R:95 A:100 D:95 |
| 3 | Certain | Include only the two baseline Code Quality acceptance items (no per-principle items) | code-quality.md principles/anti-patterns are all code-oriented (function size, imports, class avoidance); none apply to a single markdown-article edit | S:90 R:90 A:95 D:90 |

3 assumptions (2 certain, 1 confident, 0 tentative).
