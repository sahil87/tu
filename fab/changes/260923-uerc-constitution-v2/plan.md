# Plan: Constitution v2.0.0 — Go Cutover

**Change**: 260923-uerc-constitution-v2
**Intake**: `intake.md`

## Requirements

### Constitution: Go-era principles

#### R1: Principle III describes the shipped Go artifact
`fab/project/constitution.md` § III MUST be retitled `Single Static Binary` and MUST require: a statically linked Go binary (`CGO_ENABLED=0`, version via `-ldflags "-X main.version=…"`) built from `src/go/cmd/tu`; distribution as a prebuilt per-platform tarball containing exactly `tu`, `vendor/ccusage/bin/ccusage`, `tu.default.conf`; a generated formula with no `depends_on "node"` or other runtime dependency; vendor-first ccusage resolution relative to the resolved `os.Executable()` with a `PATH` fallback; the pin in `CCUSAGE_VERSION`; runtime data files embedded with `go:embed`. No mention of esbuild, `dist/tu.mjs`, or `node_modules` may remain in § III.

- **GIVEN** the v2.0.0 constitution
- **WHEN** a reviewer reads § III
- **THEN** every clause names an artifact or mechanism that exists in `justfile`, `scripts/package-go.sh`, `.github/formula-template.rb`, or `src/go/internal/source/ccusage/exec.go`
- **AND** no TypeScript-era term (esbuild, ESM bundle, `dist/tu.mjs`, `node_modules`) appears

#### R2: Principle IV forbids init-time work and keeps the caching MUST
§ IV MUST keep its title `Fast Startup`, MUST forbid I/O, process execution, and network access in `init()` functions and package-level `var` initializers (permitting compile-time tables, compiled regexes, sentinel errors, `go:embed` payloads), MUST keep a caching MUST for heavy operations under a context deadline, SHOULD state the minimal-dependency posture, and MUST NOT mention dynamic `import()`.

- **GIVEN** the v2.0.0 constitution
- **WHEN** a reviewer reads § IV
- **THEN** the dynamic-`import()` sentence is gone and the `init()` rule is present
- **AND** the caching requirement is still a MUST

#### R3: Principle V names the Go fact types
§ V MUST reference `fact.Record` and `fact.Totals` (not `UsageEntry`/`UsageTotals`), require new sources to produce `[]fact.Record`, keep the pure-aggregation MUST, and keep the ISO-label MUST for the `Date` field.

- **GIVEN** § V
- **WHEN** grepped for `UsageEntry` or `UsageTotals`
- **THEN** there are no matches, and `fact.Record` / `fact.Totals` appear

#### R4: `## TypeScript Conventions` is replaced by `## Go Conventions`
The section MUST be retitled and its body MUST carry, as MUST/SHOULD rules: gofmt + go vet cleanliness gated by `just go-lint`; the `src/go/cmd/tu` + `src/go/internal/<pkg>` layout naming the ten stage packages (`fact`, `source` with `ccusage`/`metrics`/`cache`, `query`, `view`, `render` with `ansi`/`json`/`csv`/`markdown`, `command`, `sync`, `config`, `watch`, `toolkit`) and the non-shipped `cmd/` tools; purity of the middle stages (`query`, `view`, `render` never import `os`, exec, or write to a stream; nothing under `internal/` prints except `cmd/tu` and `watch`'s terminal seam); no globals for results or render state (compile-time tables, regexes, sentinel errors, embeds permitted); errors returned as `*source.Error` and written only at the edge via `source.WriteWarnings`, no `os.Exit`/`log.Fatal` in library packages; dependency direction following the pipeline; consumer-defined interfaces satisfied at the edge.

- **GIVEN** the v2.0.0 constitution
- **WHEN** grepped for `tsconfig`, `NodeNext`, `.js` extensions, `node:` imports, or `import type`
- **THEN** there are no matches under the conventions heading
- **AND** the heading reads `## Go Conventions`

#### R5: Test Runner and Test Location are rewritten for Go
`### Test Runner` MUST name `go test ./...` from `src/go/` (`just go-test`, `-count=1`), discourage assertion/mocking frameworks, describe golden files (`testdata/*.golden`, byte-for-byte, regenerated only by `go test ./... -update`, each golden test declaring `var update = flag.Bool("update", …)`), tie a golden change to Output Stability, and name the differential harness (`cmd/tudiff`, `harness/`, the `tudiff` CI lane) as complementary. `### Test Location` MUST require `_test.go` siblings in the same package directory, `testdata/` for goldens/fixtures, MUST forbid `__tests__/` anywhere under `src/go/`, and MUST place binary-level tests beside `main.go` in `src/go/cmd/tu/`. Neither clause may say "For `src/node/`:" or reference the Go Transition article.

- **GIVEN** the two constraints
- **WHEN** read by an agent adding a Go test
- **THEN** it knows the runner, the file placement, the golden mechanism, and that `__tests__/` is forbidden

#### R6: The Go Transition article is removed and the version bumped
The `## Go Transition` heading and its entire body MUST be deleted with no replacement section. `### I.`, `### II.`, `### Test Integrity`, `### Output Stability`, `### Toolkit Standards` MUST remain byte-identical to v1.2.0. Section order stays Core Principles → Go Conventions → Additional Constraints → Governance. The Governance line MUST read `**Version**: 2.0.0 | **Ratified**: 2026-03-06 | **Last Amended**: 2026-09-23`.

- **GIVEN** `git diff origin/main -- fab/project/constitution.md`
- **WHEN** inspected
- **THEN** the only removed hunks are § III/IV/V bodies, the TS Conventions body, the two test clauses, the Go Transition section, and the Governance line
- **AND** no line of § I, § II, Test Integrity, Output Stability, or Toolkit Standards is in the diff

### Project context files

#### R7: `code-quality.md` loses its TypeScript-only guidance
`fab/project/code-quality.md` MUST replace the two Principles bullets "Use `type` imports…" and "Use `node:` prefixed imports…" with one bullet pointing at the constitution's Go Conventions (pure stages return values, errors returned not printed, no result globals), and MUST replace the Anti-Pattern "Dynamic `import()` for core paths…" with "I/O, exec, or network in `init()` or package-level initializers (Constitution IV)". All other bullets and `## Test Strategy` stay unchanged.

- **GIVEN** the file
- **WHEN** grepped for `import` 
- **THEN** no `type` import / `node:` import / dynamic `import()` bullet remains

#### R8: `context.md` describes the Go stack and the retired TS tree
`fab/project/context.md` MUST replace `## Stack` with the Go stack (module, go 1.26, `x/term`, `just go-build`/`go-build-all`, `just go-lint`/`go-test`, golden `-update`, justfile, Homebrew tap with prebuilt `tu-go-*` assets and a dependency-free formula, MIT), MUST replace `## Go Transition` with a `## Retired TypeScript tree` note (unshipped since X1, frozen, kept until Z1 as harness oracle and rollback build, toolchain recorded in `docs/memory/build/toolchain.md`, constitution no longer governs it), and MUST replace `### Module layout (src/node/)` with `### Package layout (src/go/internal/)` as tabulated in the intake. The `## Architecture` intro (ccusage sources) and `### Modes` stay unchanged. No sentence may point at a "Go Transition article".

- **GIVEN** the file
- **WHEN** grepped for `Go Transition`
- **THEN** there are no matches
- **AND** the stack lists Go, not TypeScript, as the language

### Non-Goals

- No change to `src/`, `README.md`, `docs/site/`, `docs/specs/`, `package.json`, `scripts/`, `justfile`, `.github/`, `harness/` — the Goal freeze
- No change to `fab/project/config.yaml` — `src/node/` and the JS/TS test patterns stay until Z1
- No memory edits at apply — hydrate updates `docs/memory/build/toolchain.md`

### Design Decisions

#### Go Conventions permit compile-time package-level state
**Decision**: "No globals for results" is scoped to result and render state; package-level tables, compiled regexes, sentinel errors, and `go:embed` payloads are explicitly allowed.
**Why**: The TS problem being replaced was mutable `_lastRender*` result globals; the Go tree legitimately holds 40+ compile-time declarations (tool registry, regexes, `ErrUnported`, embedded assets).
**Rejected**: A blanket "no package-level var" — every existing package would violate it on day one.
*Introduced by*: 260923-uerc-constitution-v2

#### The retired TS tree is described in context.md, not the constitution
**Decision**: The constitution carries no clause about `src/node/`; its post-cutover role (frozen, harness oracle, rollback build until Z1) lives in `context.md` and `docs/memory/build/toolchain.md`.
**Why**: The constitution governs the shipped artifact; the TS tree is verified by the differential harness, not by convention clauses, and a clause about it would need removing again at Z1.
**Rejected**: Keeping a slimmed transitional article — leaves two conventions live and defeats the article's own "removed at cutover" promise.
*Introduced by*: 260923-uerc-constitution-v2

## Tasks

### Phase 1: Constitution

- [x] T001 Rewrite `fab/project/constitution.md`: § III → "Single Static Binary", § IV body, § V type names, `## TypeScript Conventions` → `## Go Conventions`, `### Test Runner`, `### Test Location`, delete `## Go Transition`, Governance line to 2.0.0 / 2026-09-23 — text per intake § 1a–1i <!-- R1 R2 R3 R4 R5 R6 -->

### Phase 2: Project context files

- [x] T002 [P] Edit `fab/project/code-quality.md`: replace the two TS import bullets with the Go Conventions pointer and the dynamic-`import()` anti-pattern with the `init()` anti-pattern <!-- R7 -->
- [x] T003 [P] Edit `fab/project/context.md`: Go `## Stack`, `## Retired TypeScript tree`, `### Package layout (src/go/internal/)` per intake § 3 <!-- R8 -->

### Phase 3: Verification

- [x] T004 Verify: `git diff origin/main --stat` touches only the three `fab/project/` files plus `fab/changes/260923-uerc-…`; grep the constitution for `esbuild|tu.mjs|node_modules|import\(|tsconfig|NodeNext|UsageEntry|Go Transition|src/node` (expect none); confirm § I, § II, Test Integrity, Output Stability, Toolkit Standards are not in the diff; confirm every path/recipe named in the new clauses exists (`just go-lint`, `just go-test`, `CCUSAGE_VERSION`, `exec.go ResolveBinary`, `command.Result`, `source.WriteWarnings`, `testdata/*.golden`, `-update` flags); `fab preflight` still parses <!-- R1 R2 R3 R4 R5 R6 R7 R8 -->

## Execution Order

- T001 first; T002 and T003 are independent of each other and of T001
- T004 last

## Acceptance

### Functional Completeness

- [x] A-001 R1: § III is titled "Single Static Binary" and names the static build, the three tarball members, the no-`depends_on` formula, vendor-first ccusage resolution, `CCUSAGE_VERSION`, and `go:embed`
- [x] A-002 R2: § IV forbids I/O/exec/network in `init()` and package-level initializers, keeps the caching MUST, and no longer mentions dynamic `import()`
- [x] A-003 R3: § V names `fact.Record` / `fact.Totals` and no longer names `UsageEntry` / `UsageTotals`
- [x] A-004 R4: `## Go Conventions` exists with the gofmt/vet gate, the layout and ten-package list, the purity rule, the no-result-globals rule, the errors-returned rule, and the dependency-direction rule
- [x] A-005 R5: `### Test Runner` names `go test ./...`, golden files with `-update`, and the harness; `### Test Location` requires `_test.go` siblings and forbids `__tests__/`
- [x] A-006 R6: No `## Go Transition` heading remains; Governance reads `2.0.0` / `2026-09-23`
- [x] A-007 R7: `code-quality.md` has no `type` import, `node:` import, or dynamic `import()` bullets and has the Go Conventions pointer and the `init()` anti-pattern
- [x] A-008 R8: `context.md` Stack lists Go; a `## Retired TypeScript tree` section replaces `## Go Transition`; the layout table is the Go package table

### Behavioral Correctness

- [x] A-009 R1: Every mechanism § III names exists on disk (`CGO_ENABLED=0` in `justfile`, the three archive members in `scripts/package-go.sh`, `libexec.install` in `.github/formula-template.rb`, `resolveVendor` in `src/go/internal/source/ccusage/exec.go`)
- [x] A-010 R4: The ten packages and four `render` encoders named in Go Conventions each exist under `src/go/internal/`, and the non-shipped `cmd/` list matches `src/go/cmd/`
- [x] A-011 R5: Golden `-update` flags exist in the `render/*` and `watch` test files the clause describes; `just go-test` and `just go-lint` recipes exist

### Removal Verification

- [x] A-012 R6: `grep -n "Go Transition\|src/node\|esbuild\|tu.mjs\|node_modules\|tsconfig\|NodeNext" fab/project/constitution.md` returns nothing
- [x] A-013 R8: `grep -n "Go Transition" fab/project/context.md` returns nothing

### Scenario Coverage

- [x] A-014 R6: `git diff origin/main -- fab/project/constitution.md` contains no removed or added line from § I, § II, Test Integrity, Output Stability, or Toolkit Standards

### Edge Cases & Error Handling

- [x] A-015 R6: `fab preflight uerc` exits 0 after the edits (constitution still parses as the always-load file)

### Code Quality

- [x] A-016 Pattern consistency: New constitution clauses use RFC 2119 keywords and the same heading levels as the v1.2.0 sections they replace
- [x] A-017 No unnecessary duplication: The retired-TS role is stated once in `context.md` and not repeated in the constitution
- [x] A-018 Scope: `git diff origin/main --stat` lists only `fab/project/constitution.md`, `fab/project/code-quality.md`, `fab/project/context.md`, and files under `fab/changes/260923-uerc-constitution-v2/`

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Section order is preserved; `## Go Conventions` sits where `## TypeScript Conventions` was | Minimal diff; the intake's § 1h states it | S:90 R:95 A:95 D:90 |
| 2 | Certain | The `## Retired TypeScript tree` heading replaces `## Go Transition` in `context.md` | Intake § 3 gives the heading verbatim | S:90 R:95 A:90 D:90 |
| 3 | Confident | `context.md`'s Go package table includes a `harness` row labelled maintainer tooling | `internal/harness` exists on disk; omitting it would make the "one package per stage" list incomplete for a reader of the tree | S:60 R:95 A:90 D:75 |

3 assumptions (2 certain, 1 confident, 0 tentative).
