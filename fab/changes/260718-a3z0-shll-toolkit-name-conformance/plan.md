# Plan: shll Toolkit Name Conformance

**Change**: 260718-a3z0-shll-toolkit-name-conformance
**Intake**: `intake.md`

## Requirements

<!-- Requirements derived from the intake's What Changes section: a mechanical,
     prose-only rename sweep of "sahil87 toolkit" → "shll toolkit" (and
     "sahil87 tool(s)" → "shll tool(s)"), with identifiers deliberately
     preserved. No behavior change. -->

### README: Head Blockquote Conformance

#### R1: Canonical toolkit blockquote
README.md line 3 MUST become byte-identical to the readme-extraction standard's canonical line, and the README head order (H1 → blockquote → badges) MUST be preserved.

- **GIVEN** README.md line 3 reads `> Part of [@sahil87's open source toolkit](https://shll.ai) — see all projects there.`
- **WHEN** the sweep runs
- **THEN** line 3 reads exactly `> Part of the [shll toolkit](https://shll.ai) — see all projects there.`
- **AND** line 1 remains the H1 `# tu`, line 3 remains the blockquote, and the badges line remains below it (head order unchanged)

#### R2: README prose occurrences
The two remaining prose references to the old toolkit name in README.md MUST be renamed, leaving the embedded `github.com/sahil87/…` identifier URL untouched.

- **GIVEN** README.md line 16 contains `the entire sahil87 toolkit instead:` and line 39 contains `Have other sahil87 tools?`
- **WHEN** the sweep runs
- **THEN** line 16 reads `… the entire shll toolkit instead:` and line 39 reads `Have other shll tools? …`
- **AND** the `https://github.com/sahil87/shll#…` URL on line 39 is unchanged

### Docs: Prose Sweep

#### R3: Spec and memory prose
Present-truth docs prose (specs + memory) referencing the old toolkit name MUST be renamed.

- **GIVEN** `docs/specs/usage.md:84`, `docs/memory/build/toolchain.md:58`, and `docs/memory/cli/data-pipeline.md:37` each contain `sahil87 toolkit`
- **WHEN** the sweep runs
- **THEN** each occurrence reads `shll toolkit`
- **AND** no other text on those lines changes (the `260717-rdo3`/`260717-8h6g` change-id citations and all surrounding prose are preserved)

### Code: Comment Sweep

#### R4: Code comments
The two non-user-visible code comments referencing the old toolkit name MUST be renamed. These are comments only — no runtime string, help output, or golden is affected.

- **GIVEN** `src/node/core/cli.ts:67` reads `// Exit-code convention (sahil87 toolkit principle №4): 0 = success,` and `src/node/core/__tests__/cli-exit-codes.test.ts:8` reads `// End-to-end exit-code contract (sahil87 toolkit principle №4):`
- **WHEN** the sweep runs
- **THEN** both comments read `(shll toolkit principle №4)`
- **AND** the full test suite still passes (comment edits cannot change behavior)

### Constitution: Toolkit Standards Article

#### R5: Constitution cosmetic amendment
The constitution's Toolkit Standards article sentence MUST be renamed; the same line's `sahil87/shll` canonical-source identifier reference MUST remain untouched; the governance line's `Last Amended` MUST read `2026-07-18` (its current value — byte-unchanged) and `Version` MUST stay `1.1.0`.

- **GIVEN** `fab/project/constitution.md:44` reads `This tool is part of the sahil87 toolkit and MUST conform …` and line 48 records `Last Amended: 2026-07-18`, `Version: 1.1.0`
- **WHEN** the sweep runs
- **THEN** line 44 reads `This tool is part of the shll toolkit and MUST conform …`
- **AND** the `the sahil87/shll repository's docs/site/standards/ tree` reference later in line 44 is unchanged
- **AND** line 48 is byte-unchanged (`Last Amended` already equals today, `Version` stays `1.1.0`)

### Verification: Zero Prose Occurrences

#### R6: Post-sweep scan is clean
After the sweep, `grep -rniE 'sahil87 (toolkit|tools?)' README.md docs/ src/ fab/project/` MUST return zero matches, and identifiers MUST be preserved everywhere.

- **GIVEN** the sweep has completed
- **WHEN** the verification grep runs over `README.md docs/ src/ fab/project/`
- **THEN** it returns no matches (exit 1)
- **AND** the constitution's `sahil87/shll repository` reference does not match this pattern (it is an identifier, not `sahil87 toolkit|tool(s)` prose) and remains present
- **AND** `sahil87/tap`, `github.com/sahil87/…`, and `raw.githubusercontent.com/sahil87/…` are unchanged everywhere

### Non-Goals

- `docs/site/**` (install.md, skill.md, workflows.md) — verified at intake to contain zero old-name occurrences; no edits, no skill-bundle re-sync (embed is a build-time esbuild define reading the canonical file directly).
- CLI help text / user-visible strings — verified zero occurrences; no help-dump change, no golden updates, no `schema_version` bump.
- `package.json` version — no output change, so no version bump (Output Stability rule not triggered).
- `fab/changes/` archives and `dist/` — out of scope, untouched.

### Design Decisions

1. **Identifiers preserved, prose renamed**: GitHub-owner paths (`sahil87/tap`, `github.com/sahil87/…`, `raw.githubusercontent.com/sahil87/…`, `sahil87/shll`) are addresses, not prose — *Why*: renaming them would break real URLs and the canonical-source binding — *Rejected*: a blanket `sahil87` → `shll` substitution (would corrupt every identifier).
2. **Constitution governance line left byte-identical**: `Last Amended` already equals today (2026-07-18) — *Why*: the standards-binding amendment landed earlier today; the task mandates only a `Last Amended` bump and today already matches — *Rejected*: bumping `Version` (the task says nothing else in the article changes).

## Tasks

### Phase 1: Prose & Comment Sweep

<!-- All edits are independent (distinct files/lines); the only ordering
     constraint is that verification (Phase 2) runs after every edit. -->

- [x] T001 [P] Replace README.md line 3 blockquote with the byte-identical canonical line `> Part of the [shll toolkit](https://shll.ai) — see all projects there.`, preserving H1 → blockquote → badges head order <!-- R1 -->
- [x] T002 [P] Rename README.md line 16 (`the entire sahil87 toolkit instead:` → `the entire shll toolkit instead:`) and line 39 (`Have other sahil87 tools?` → `Have other shll tools?`), leaving the `github.com/sahil87/shll` URL untouched <!-- R2 -->
- [x] T003 [P] Rename `sahil87 toolkit` → `shll toolkit` in `docs/specs/usage.md:84` <!-- R3 -->
- [x] T004 [P] Rename `sahil87 toolkit` → `shll toolkit` in `docs/memory/build/toolchain.md:58` and `docs/memory/cli/data-pipeline.md:37` <!-- R3 -->
- [x] T005 [P] Rename `(sahil87 toolkit principle №4)` → `(shll toolkit principle №4)` in `src/node/core/cli.ts:67` and `src/node/core/__tests__/cli-exit-codes.test.ts:8` <!-- R4 -->
- [x] T006 [P] Rename `part of the sahil87 toolkit` → `part of the shll toolkit` in `fab/project/constitution.md:44`, leaving the `sahil87/shll` canonical-source reference and the byte-unchanged governance line (`Last Amended: 2026-07-18`, `Version: 1.1.0`) as-is <!-- R5 -->

### Phase 2: Verification

- [x] T007 Run `grep -rniE 'sahil87 (toolkit|tools?)' README.md docs/ src/ fab/project/` and confirm zero matches; confirm the constitution's `sahil87/shll repository` reference remains and all `sahil87/tap`/`github.com/sahil87/…`/`raw.githubusercontent.com/sahil87/…` identifiers are intact <!-- R6 -->
- [x] T008 Run the full test suite (`npm test`) per the constitution's Test Runner convention; all tests MUST pass <!-- R4 -->

## Acceptance

### Functional Completeness

- [x] A-001 R1: README.md line 3 is byte-identical to `> Part of the [shll toolkit](https://shll.ai) — see all projects there.` and the head order (H1 → blockquote → badges) is preserved
- [x] A-002 R2: README.md lines 16 and 39 read `shll toolkit` / `shll tools` respectively, with the `github.com/sahil87/shll` URL unchanged
- [x] A-003 R3: `docs/specs/usage.md:84`, `docs/memory/build/toolchain.md:58`, and `docs/memory/cli/data-pipeline.md:37` each read `shll toolkit`
- [x] A-004 R4: `src/node/core/cli.ts:67` and `src/node/core/__tests__/cli-exit-codes.test.ts:8` each read `(shll toolkit principle №4)`
- [x] A-005 R5: `fab/project/constitution.md:44` reads `part of the shll toolkit`, the `sahil87/shll repository` reference is intact, and the governance line is byte-unchanged (`Last Amended: 2026-07-18`, `Version: 1.1.0`)

### Behavioral Correctness

- [x] A-006 R4: The full test suite passes — the comment-only edits change no runtime behavior, no help output, no goldens, no exit codes

### Scenario Coverage

- [x] A-007 R6: `grep -rniE 'sahil87 (toolkit|tools?)' README.md docs/ src/ fab/project/` returns zero matches after the sweep

### Edge Cases & Error Handling

- [x] A-008 R6: All preserved identifiers (`sahil87/tap`, `github.com/sahil87/…`, `raw.githubusercontent.com/sahil87/…`, the constitution's `sahil87/shll` canonical-source reference) remain byte-identical; only prose/comment occurrences changed

### Code Quality

- [x] A-009 Pattern consistency: Edits touch only the identified prose/comment tokens; no surrounding text, formatting, or change-id citations are disturbed
- [x] A-010 No unnecessary duplication: No new files or utilities introduced — a pure in-place rename

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Sweep scope covers `docs/specs/usage.md` and the two `docs/memory` files even though the task's explicit location list emphasizes README/docs/site/CLI | Intake's operative instruction is "wherever they appear as prose"; these are present-truth docs, only `fab/changes/` is excluded — intake Assumption 4 grades this Confident, but the intake's What Changes section then names these files with exact line numbers, raising certainty | S:90 R:90 A:95 D:90 |
| 2 | Certain | The two code comments are prose references (not identifiers) and are renamed with zero behavioral risk | Comments carry no runtime effect; intake enumerates both with exact lines | S:90 R:95 A:100 D:95 |
| 3 | Certain | Constitution governance line left byte-unchanged (`Last Amended` already 2026-07-18, `Version` stays 1.1.0) | Intake and task both state today equals the recorded date and nothing else in the article changes | S:85 R:90 A:95 D:90 |

3 assumptions (3 certain, 0 confident, 0 tentative).
