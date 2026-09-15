# Plan: Constitution Go Transition

**Change**: 260915-n18z-constitution-go-transition
**Intake**: `intake.md`

## Requirements

### Constitution: Go Transition article

#### R1: Transitional article exists as a peer section
`fab/project/constitution.md` MUST contain a new top-level `## Go Transition` section placed after `## Additional Constraints` and before `## Governance`, with the wording given in `intake.md` § What Changes 1a (a blockquote header naming it transitional, v1.2.0, 2026-09-15, removed at v2.0.0, plan `fab/plans/sahil/26-09-15-go-port.md` D2/D3; then six bullets: `src/node/` shipped; `src/go/` successor MUST NOT ship until cutover; I/II/V bind both trees; III/IV/TypeScript Conventions bind `src/node/` only and `src/go/` is not in violation of III; Test Runner/Test Location bind `src/node/` only, Go tests MUST be `_test.go` siblings, no `__tests__/` under `src/go/`; the frozen external surfaces are the contract `src/go/` reproduces, Output Stability and Toolkit Standards bind both trees).

- **GIVEN** the amended constitution
- **WHEN** a reviewer reads the section list
- **THEN** `## Go Transition` appears exactly once, between `## Additional Constraints` and `## Governance`
- **AND** every clause listed above is present

#### R2: Principles I–V and TypeScript Conventions are unchanged
The bodies of `### I.` through `### V.` and the `## TypeScript Conventions` bullet list MUST be byte-identical to v1.1.0 (`git show main:fab/project/constitution.md`). Scoping is expressed once, in the Go Transition article.

- **GIVEN** `git diff main -- fab/project/constitution.md`
- **WHEN** hunks are inspected
- **THEN** no hunk touches lines 3–27 of the v1.1.0 file (Core Principles + TypeScript Conventions)

#### R3: Test Runner and Test Location constraints are scoped to `src/node/` with corrected text
The `### Test Runner` and `### Test Location` constraints MUST begin with `For \`src/node/\`:`, MUST use the wording in `intake.md` § What Changes 1b and 1c, and MUST no longer contain the stale strings `tests/*.test.ts`, `tests/{module}.test.ts`, or `src/__tests__/fetcher.test.ts`. Each MUST end with a pointer to the Go Transition article.

- **GIVEN** the amended constitution
- **WHEN** `grep -n 'tests/\*.test.ts\|tests/{module}\|src/__tests__/fetcher' fab/project/constitution.md` runs
- **THEN** it prints nothing
- **AND** both constraints reference `npm test` / `src/node/core/__tests__/fetcher.test.ts` respectively and point at the Go Transition article

#### R4: Governance line bumped
The Governance line MUST read exactly `**Version**: 1.2.0 | **Ratified**: 2026-03-06 | **Last Amended**: 2026-09-15`.

- **GIVEN** the amended constitution
- **WHEN** the `## Governance` section is read
- **THEN** the version is 1.2.0, Ratified is unchanged, Last Amended is 2026-09-15

### Config: source and test paths

#### R5: `source_paths` lists both trees; `test_paths` includes the Go pattern
`fab/project/config.yaml` `source_paths` MUST be exactly `[src/node/, src/go/]` (replacing `src/`), and `test_paths` MUST contain the four existing JS/TS patterns plus `"**/*_test.go"`. Nothing else in the file changes; the `>>> fab reference` fence is untouched.

- **GIVEN** the amended config
- **WHEN** `fab preflight n18z` and `fab config explain source_paths` run
- **THEN** both succeed and report the two source paths
- **AND** `git diff main -- fab/project/config.yaml` shows only the `source_paths` and `test_paths` hunks

### Context: transition note and layout table

#### R6: `context.md` carries the transition note and a truthful layout
`fab/project/context.md` MUST (a) amend the Stack "Language" line to name `src/node/` as shipped and `src/go/` as the successor, (b) correct the "Test runner" line to the `npm test` form, (c) add a `## Go Transition` section with the two-tree table and the `_test.go` sibling note, and (d) replace the `### Module layout (\`src/\`)` table with the `src/node/`-rooted table from `intake.md` § What Changes 3 (15 rows; no `sparkline.ts` row; trailing co-located-tests sentence). Every path in the table MUST exist on disk.

- **GIVEN** the amended context file
- **WHEN** each `core/…`, `sync/…`, `tui/…` path in the layout table is checked with `test -f src/node/<path>`
- **THEN** every check passes
- **AND** the `## Go Transition` section and the `## Architecture` intro / `### Modes` subsection are all present

### Non-Goals

- No edits under `src/`, `README.md`, `docs/`, `package.json`, `scripts/`, `Formula/`, `.github/` — the plan's Goal freezes every external surface, and this row is `fab/project/*` only.
- No `src/go/` scaffold, `go.mod`, justfile recipe, or CI job (row P2).
- No rewrite of Principles III/IV or a Go Conventions section (row X2, v2.0.0).
- No encoding of D4 (feature freeze) or D5 (Go layout) in the constitution.
- No update to the plan doc's P0 status column (file not in this tree).

### Design Decisions

#### Scope by article, not by editing each principle
**Decision**: One `## Go Transition` article states which trees each existing clause binds; the principle bodies are not edited.
**Why**: Keeps the diff to Principles III/IV at zero, matching D3's "Go rewrite at cutover, not now", and gives X2 a single section to delete.
**Rejected**: Adding "(applies to `src/node/`)" parentheticals to III, IV, TypeScript Conventions, Test Runner, Test Location — five scattered edits that X2 would have to unwind individually.
*Introduced by*: 260915-n18z-constitution-go-transition

#### Replace `src/` with the two explicit trees in `source_paths`
**Decision**: `source_paths: [src/node/, src/go/]`.
**Why**: `src/` already covers `src/go/` as a prefix, so "add `src/go/`" as an append would be a no-op; the explicit pair is semantically identical today and documents the transition in config.
**Rejected**: Appending `src/go/` under `src/` (redundant); leaving `src/` alone (config would not reflect the two-tree state).
*Introduced by*: 260915-n18z-constitution-go-transition

## Tasks

### Phase 1: Core Implementation

- [x] T001 Amend `fab/project/constitution.md`: insert the `## Go Transition` section (intake 1a) before `## Governance`; rewrite `### Test Runner` and `### Test Location` per intake 1b/1c; bump the Governance line to 1.2.0 / 2026-09-15. Leave Principles I–V and TypeScript Conventions byte-identical. <!-- R1, R2, R3, R4 -->
- [x] T002 [P] Amend `fab/project/config.yaml`: `source_paths` → `src/node/`, `src/go/`; append `"**/*_test.go"` to `test_paths`. Touch nothing else. <!-- R5 -->
- [x] T003 [P] Amend `fab/project/context.md`: Stack "Language" and "Test runner" lines; add `## Go Transition` section with the two-tree table; replace the module layout table with the `src/node/`-rooted 15-row table (intake § 3). <!-- R6 -->

### Phase 2: Verification

- [x] T004 Verify: `git diff main -- fab/project/constitution.md` touches no Core Principles / TypeScript Conventions lines; stale-string grep returns nothing; `fab preflight n18z` succeeds; every path in the context layout table exists (`test -f`); `git status --porcelain` shows only the three `fab/project/*` files plus this change folder. <!-- R2, R3, R5, R6 -->

## Acceptance

### Functional Completeness

- [x] A-001 R1: `## Go Transition` exists once, between `## Additional Constraints` and `## Governance`, with all six bullets and the transitional blockquote
- [x] A-002 R4: Governance line reads `**Version**: 1.2.0 | **Ratified**: 2026-03-06 | **Last Amended**: 2026-09-15`
- [x] A-003 R5: `source_paths` is exactly `src/node/`, `src/go/`; `test_paths` has the four JS/TS patterns plus `**/*_test.go`; fence untouched
- [x] A-004 R6: `context.md` has the amended Stack lines, a `## Go Transition` section with the two-tree table, and the `src/node/`-rooted layout table

### Behavioral Correctness

- [x] A-005 R2: `git diff main -- fab/project/constitution.md` has no hunk inside Core Principles or TypeScript Conventions
- [x] A-006 R3: Test Runner and Test Location begin `For \`src/node/\`:`, cite `npm test` / `src/node/core/__tests__/fetcher.test.ts`, and point at the Go Transition article; the three stale strings are absent

### Scenario Coverage

- [x] A-007 R5: `fab preflight n18z` and `fab config explain source_paths` succeed against the amended config
- [x] A-008 R6: every `core/…`, `sync/…`, `tui/…` path in the layout table resolves to a real file under `src/node/`

### Edge Cases & Error Handling

- [x] A-009 R5: no line inside the `>>> fab reference … <<< end fab reference` fence changed (it is regenerated on upgrade)
- [x] A-010 R1: no file outside `fab/project/` and `fab/changes/260915-n18z-constitution-go-transition/` is modified (Goal freeze)

### Code Quality

- [x] A-011 Pattern consistency: new constitution prose uses the existing RFC 2119 voice (MUST/SHOULD/MUST NOT) and `###`/`##` heading levels; context.md table style matches the existing tables
- [x] A-012 No unnecessary duplication: the scoping rule is stated once (in the article) and referenced by pointer from the two test constraints, not restated

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Article wording is taken verbatim from intake § 1a; no re-drafting at apply | Intake carries the full text; apply implements, does not redesign | S:95 R:95 A:95 D:95 |
| 2 | Confident | The "Test runner" Stack line in context.md is corrected to the `npm test` form alongside the Language line | Intake § 3 shows the corrected line in its Stack block; same stale-text class as the constitution fix | S:75 R:95 A:90 D:80 |

2 assumptions (1 certain, 1 confident, 0 tentative).
