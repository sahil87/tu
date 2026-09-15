# Intake: Constitution Go Transition

**Change**: 260915-n18z-constitution-go-transition
**Created**: 2026-09-15

## Origin

One-shot `/fab-new` invocation from the Go-port plan's Phase 0 queue. Raw input:

> Context: fab/plans/sahil/26-09-15-go-port.md, row P0. Read the Decisions and Target architecture sections before writing the intake. Do not change any external surface listed under Goal.
>
> Row P0 (constitution-go-transition): D3 transitional article — src/go/ is the successor implementation, unshipped until cutover; Go tests are _test.go siblings (Go idiom), the __tests__/ clause applies to src/node/ only. Update config.yaml to add src/go/ to source_paths and **/*_test.go to test_paths. Note the transition in fab/project/context.md. Bump constitution to version 1.2.0. Principles I, II, V unchanged; III, IV and the test sections get their Go rewrite at cutover (v2.0.0), not now.

The plan doc lives on the `sunny-turtle` worktree branch (commit `7412729 plan: tu Go port — rebuild internals in Go behind unchanged contracts`) and is **not yet on `main`**; it is not present in this change's tree. The intake agent read it from that worktree. Relevant plan content is reproduced below so the apply agent needs no access to it.

**Plan decisions this row implements** (from the plan's Decisions table):

- **D2** — Both codebases in `main`, Go unshipped until cutover. `src/node/` stays the shipped artifact; `src/go/` lands dark, change by change, gated by CI. No long-lived integration branch. The formula decides what ships, not the branch.
- **D3** — Constitution amended first (Phase 0), not last. Add a transitional article: `src/go/` is the successor implementation, unshipped until cutover; Go tests are `_test.go` siblings (Go idiom), the `__tests__/` clause applies to `src/node/` only. `config.yaml` gains `src/go/` in `source_paths` and `**/*_test.go` in `test_paths`. Principles I, II, V unchanged. III, IV and the test sections get their Go rewrite at cutover (v2.0.0). *Rationale:* without it every Go change violates Principle III ("MUST compile to a single ESM bundle") and the fab reviewer flags it each time.

**Plan decisions deliberately NOT encoded by this row** (they belong to later rows or are operational policy, not constitution):

- D4 (feature freeze on `src/node/`) — operational policy in the plan; not constitutionalized here.
- D5 (`src/go/go.mod`, `cmd/tu/`, `internal/<pkg>/` layout) — lands with row P2 `go-scaffold`. The article names only the `src/go/` root.
- D6/D7 (differential harness) — rows P3a/P4.
- Cutover rewrite of Principles III, IV, TypeScript Conventions, Test Runner, Test Location — row X2 `constitution-v2` (v2.0.0).

**Goal freeze** (from the plan's Goal — an instruction to this change, not article content): every external surface stays fixed — the CLI grammar, `--help` text, table/JSON/CSV/Markdown output, exit codes, `tu.conf` and `org.conf`, the metrics-repo JSONL layout and its never-shrink guard, and the toolkit contracts (`--version`, `help-dump`, `update`, `shell-init`, `skill`, config-home). This change touches only `fab/project/*` and MUST NOT touch `src/`, `README.md`, `docs/site/`, `docs/specs/`, `package.json`, `scripts/`, `Formula/`, or `.github/`.

## Why

**Problem.** The constitution (v1.1.0) describes a single-implementation Node/TypeScript project. Principle III says the CLI "MUST compile to a single ESM bundle via esbuild (`dist/tu.mjs`)"; Principle IV forbids dynamic `import()` on core paths; the TypeScript Conventions and the Test Runner / Test Location constraints assume every source file is TypeScript and every test lives in a `__tests__/` folder. The Go port (plan `26-09-15-go-port.md`) lands `src/go/` into `main` change by change starting with row P2, while `src/node/` continues to ship. Under the current text, the very first Go file is a constitutional violation on four counts, and the fab review sub-agent (which loads the constitution as always-load context) will flag it on every Phase 1–3 change.

**Consequence of not doing it.** Every Go row would either carry a review-ignore for the same four findings, or the reviewer's must-fix findings would fail the rework budget on unrelated work. Neither is acceptable for eight-plus serial rows running unattended between gates G1 and G2. The plan makes this Phase 0's first row for exactly that reason — the constitution is a contract everything after G0 is built against.

**Why this approach.** A *transitional* article that scopes the existing Node-specific text to `src/node/` and states what `src/go/` is (successor, unshipped) is the smallest amendment that unblocks Go changes without pre-deciding the Go-era constitution. Rewriting III/IV/tests for Go now (the alternative) would describe a binary that does not exist yet and would have to be revised again after the V3 slice review changes the architecture. The plan defers that rewrite to X2, after cutover, when the Go code is the shipped truth. A minor version bump (1.1.0 → 1.2.0) is correct: the amendment adds an article and scopes existing ones; it removes and rewrites nothing.

**Secondary fix folded in.** While scoping the Test Runner clause to `src/node/`, correct its stale text: it currently says tests run via `npx tsx --test tests/*.test.ts` and that new tests follow `tests/{module}.test.ts` — both false since the co-location change (`a1116a5`, 2026-03-06) and the `src/node/` move (`260306-x861`). The actual convention is `npm test` → `find src/node -path '*/__tests__/*.test.ts' -exec npx tsx --test {} +`, tests co-located as `src/node/{area}/__tests__/{module}.test.ts`. The Test Location example `src/__tests__/fetcher.test.ts` is likewise stale (real path `src/node/core/__tests__/fetcher.test.ts`). These clauses contradict each other today; fixing the text at the point of edit keeps the scoped clauses truthful. Similarly, `fab/project/context.md`'s module-layout table lists a flat `src/` that has not existed since 2026-03-06 — the transition note must reference `src/node/`, so the table is corrected in the same edit rather than left contradicting the note beside it.

## What Changes

Three files under `fab/project/`. No source, docs, or CI changes.

### 1. `fab/project/constitution.md` — add the transitional article, scope four existing clauses, bump version

#### 1a. New top-level section `## Go Transition`

Insert as a new `##` section **after `## Additional Constraints` and before `## Governance`**. It is a peer-level article (not a `###` under Additional Constraints) because it changes how Principles III and IV, the TypeScript Conventions, and two constraints are read. Wording:

```markdown
## Go Transition

> Transitional article, added in v1.2.0 (2026-09-15). It governs the period in which `src/node/` and `src/go/` coexist in `main`. It is removed at cutover (v2.0.0), when Principles III and IV, the TypeScript Conventions, and the Test Runner / Test Location constraints are rewritten for the Go implementation. Plan: `fab/plans/sahil/26-09-15-go-port.md` (decisions D2, D3).

- `src/node/` is the **shipped** implementation. `dist/tu.mjs`, the Homebrew formula, and every release artifact are built from it until cutover.
- `src/go/` is the **successor** implementation. It lands in `main` change by change and is built and tested in CI, but it MUST NOT be shipped — not through the formula, not as the `tu` binary users install — until the cutover change flips the formula.
- Principles I (Single-Purpose CLI), II (Graceful Degradation), and V (Consistent Data Model) are language-neutral and bind both trees.
- Principles III (Single-Bundle Distribution) and IV (Fast Startup) and the TypeScript Conventions describe the shipped artifact and bind `src/node/` only. Code under `src/go/` is not in violation of Principle III by existing outside the ESM bundle.
- The Test Runner and Test Location constraints bind `src/node/` only. Go tests MUST be `_test.go` siblings of the code they test, in the same directory (Go idiom). `__tests__/` folders MUST NOT be created under `src/go/`.
- The external surfaces frozen by the plan's Goal — the CLI grammar, `--help` text, table/JSON/CSV/Markdown output, exit codes, `tu.conf` and `org.conf`, the metrics-repo JSONL layout and its never-shrink guard, and the toolkit contracts — are the contract `src/go/` reproduces. Output Stability and Toolkit Standards apply to both trees.
```

#### 1b. Scope the Test Runner constraint to `src/node/` and correct its stale text

Current:

```markdown
### Test Runner
Tests use Node.js built-in test runner via `npx tsx --test tests/*.test.ts`. New test files MUST follow the `tests/{module}.test.ts` naming convention. No additional test frameworks SHOULD be introduced without justification.
```

New:

```markdown
### Test Runner
For `src/node/`: tests use the Node.js built-in test runner via `npm test` (`find src/node -path '*/__tests__/*.test.ts' -exec npx tsx --test {} +`). New test files MUST follow the `{module}.test.ts` naming convention inside a `__tests__/` folder (see Test Location). No additional test frameworks SHOULD be introduced without justification. Go tests are governed by the Go Transition article.
```

#### 1c. Scope the Test Location constraint to `src/node/` and correct its stale example

Current:

```markdown
### Test Location
Node/TypeScript test files MUST be co-located with the source code they test, in `__tests__/` folders within the same directory (e.g., `src/__tests__/fetcher.test.ts` for `src/fetcher.ts`).
```

New:

```markdown
### Test Location
For `src/node/`: test files MUST be co-located with the source code they test, in `__tests__/` folders within the same directory (e.g., `src/node/core/__tests__/fetcher.test.ts` for `src/node/core/fetcher.ts`). This clause does not apply to `src/go/` — see the Go Transition article.
```

#### 1d. Leave Principles I–V and the TypeScript Conventions text unchanged

No edits to the `### I.` through `### V.` bodies or to the `## TypeScript Conventions` bullets. Their scoping is expressed once, in the Go Transition article, not by editing each clause. This keeps the diff to III/IV at zero, matching D3 ("III, IV … get their Go rewrite at cutover, not now").

#### 1e. Governance line

Current: `**Version**: 1.1.0 | **Ratified**: 2026-03-06 | **Last Amended**: 2026-07-18`
New: `**Version**: 1.2.0 | **Ratified**: 2026-03-06 | **Last Amended**: 2026-09-15`

### 2. `fab/project/config.yaml` — source and test paths

`source_paths` is currently the single entry `src/`, which already covers `src/go/` as a prefix. Replace it with the two explicit trees so the config documents the transition and stays correct if a non-implementation directory ever appears under `src/`:

```yaml
source_paths:
    - src/node/
    - src/go/
```

Add the Go test pattern to `test_paths`, keeping the existing four JS/TS patterns:

```yaml
test_paths:
    - "**/*.spec.ts"
    - "**/*.test.ts"
    - "**/*.spec.js"
    - "**/*.test.js"
    - "**/*_test.go"
```

`fab config explain test_paths` confirms `**/*_test.go` is the kit's own Go example and that these are `:(glob)` pathspecs, so `**` crosses directories. Do not touch `true_impact_exclude`, `docs_index`, or anything inside the `>>> fab reference` fence (it is regenerated on upgrade).

### 3. `fab/project/context.md` — note the transition and correct the layout table

Replace the `## Stack` "Language" line and the `### Module layout (`src/`)` subsection. New content for the affected parts:

```markdown
## Stack

- **Language**: TypeScript (strict mode, ES2022 target, NodeNext modules) — the shipped implementation under `src/node/`. A Go successor is being built under `src/go/` (see Go Transition below).
- **Runtime**: Node.js >= 18
- **Bundler**: esbuild (single-file ESM bundle to `dist/tu.mjs`)
- **Test runner**: Node.js built-in (`npm test` → `npx tsx --test` over `src/node/**/__tests__/*.test.ts`)
- **Task runner**: justfile
- **Distribution**: Homebrew tap (`sahil87/tap`), binary name `tu`
- **License**: MIT

## Go Transition

tu is being ported to Go behind unchanged external contracts (plan: `fab/plans/sahil/26-09-15-go-port.md`). Two implementation trees coexist in `main` during the port:

| Tree | Role | Ships? |
|------|------|--------|
| `src/node/` | Current TypeScript implementation | Yes — `dist/tu.mjs` and the Homebrew formula are built from it until cutover |
| `src/go/` | Successor Go implementation, landing change by change | No — built and tested in CI only, until the cutover change flips the formula |

Go tests are `_test.go` siblings of the code they test (no `__tests__/` under `src/go/`). The constitution's Go Transition article (v1.2.0) is the binding statement; this section is the orientation note.
```

And the module layout table, corrected to the real tree (files read from disk 2026-09-15):

```markdown
### Module layout (`src/node/`)

| Module | Responsibility |
|--------|---------------|
| `core/cli.ts` | Entry point, argument parsing, command dispatch |
| `core/types.ts` | Core data interfaces (`UsageEntry`, `UsageTotals`, `ToolConfig`) |
| `core/fetcher.ts` | Tool execution, JSON parsing, caching, data aggregation |
| `core/config.ts` | Config file reading (`~/.config/tu/tu.conf`, org.conf layer) |
| `core/leaderboard.ts` | Leaderboard (`lb`/`lbh`) data shaping |
| `core/help-dump.ts` | `tu help-dump` contract document |
| `core/skill.ts` | `tu skill` agent bundle |
| `core/completions.ts` | Shell completions |
| `sync/sync.ts` | Multi-machine metrics sync via git repo |
| `tui/formatter.ts` | Table rendering (print to stdout, render to string[]) |
| `tui/watch.ts` | Live polling mode with terminal refresh |
| `tui/rain.ts` | Matrix rain animation for watch mode |
| `tui/panel.ts` | Box/panel drawing for TUI output |
| `tui/compositor.ts` | Terminal compositor for watch layout |
| `tui/colors.ts` | ANSI color helpers with `--no-color` support |

Tests are co-located in `__tests__/` folders within each subdirectory (`core/__tests__/`, `sync/__tests__/`, `tui/__tests__/`).
```

Note: the old table listed `sparkline.ts`; there is no such file on disk (sparkline rendering lives in `tui/formatter.ts`). Drop the row. The `## Architecture` intro paragraph and `### Modes` subsection are unchanged.

### 4. Not in this change

- No edit to the plan doc's P0 status column — the plan file is not in this tree (it lives on the `sunny-turtle` branch). The operator/user updates it when the row lands.
- No `src/go/` directory, `go.mod`, justfile recipe, or CI job — row P2.
- No changes to `docs/memory/` at apply time — hydrate updates `build/toolchain.md` (see Affected Memory).

## Affected Memory

- `build/toolchain`: (modify) Update the toolkit-standards conformance posture line's constitution reference from `v1.1.0 … Last Amended 2026-07-18` to `v1.2.0 … Last Amended 2026-09-15` and record the Go Transition article. Revise the **`src/node/` directory structure** design decision: the "future `src/rust/` sibling" the namespace was reserved for is now the `src/go/` successor tree (plan `26-09-15-go-port.md`, D2/D3); note that `fab/project/config.yaml` `source_paths` lists `src/node/` and `src/go/` and `test_paths` includes `**/*_test.go`. Drop the nonexistent `scripts/` subdirectory from that decision's list if the hydrate agent confirms it is absent on disk (it is, as of 2026-09-15).

## Impact

- **Files**: `fab/project/constitution.md`, `fab/project/config.yaml`, `fab/project/context.md` (3 files, ~60 changed lines).
- **Runtime behavior**: none. No source, build, test, or CI change. `npm test` and `scripts/build.sh` are untouched.
- **fab pipeline behavior**: every subsequent change's review sub-agent loads the amended constitution; Go files under `src/go/` stop being Principle III/IV/Test-Location findings. `source_paths` scoping for apply-context loading is unchanged in effect (`src/node/` + `src/go/` equals the old `src/`). The `/git-pr` true-impact breakdown will attribute `*_test.go` lines to tests once Go tests exist.
- **Downstream rows**: P2 (`go-scaffold`) depends on this; every Phase 1–3 row inherits the scoping.
- **Verification**: `fab preflight` and `fab config` must still parse `config.yaml`; a read-through of the constitution confirms Principles I–V and TypeScript Conventions bodies are byte-identical to v1.1.0.

## Open Questions

None. The row and plan decisions D2/D3 determine every edit; the two in-passing corrections (stale test-clause text, stale layout table) are recorded as graded assumptions below.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Constitution version 1.1.0 → 1.2.0, Last Amended 2026-09-15, Ratified unchanged | Stated verbatim in the row; minor bump matches an additive amendment | S:95 R:95 A:95 D:95 |
| 2 | Certain | `test_paths` gains `**/*_test.go` alongside the four existing JS/TS patterns | Stated in the row; kit's own Go example in `fab config explain test_paths` | S:95 R:95 A:95 D:95 |
| 3 | Confident | `source_paths` becomes `src/node/` + `src/go/` (replacing `src/`) rather than appending `src/go/` under an existing `src/` | Row says "add `src/go/`"; `src/` already covers it as a prefix, so appending is a no-op. The two-entry form is semantically identical today and documents the transition. Reversible one-line edit | S:70 R:95 A:85 D:65 |
| 4 | Confident | The transitional article is a new top-level `## Go Transition` section placed between `## Additional Constraints` and `## Governance` | Row says "transitional article"; it rescopes Principles III/IV, the TS Conventions and two constraints, so it is a peer article, not a sub-constraint. Position before Governance follows the amendment-at-the-end convention | S:70 R:90 A:75 D:60 |
| 5 | Confident | Principles I–V and TypeScript Conventions bodies are left byte-identical; scoping is stated once in the article | Row: "Principles I, II, V unchanged; III, IV … get their Go rewrite at cutover, not now". Editing III/IV inline would be a partial rewrite | S:85 R:90 A:85 D:80 |
| 6 | Confident | Test Runner and Test Location constraints are prefixed "For `src/node/`:" and their stale text (`tests/*.test.ts`, `src/__tests__/fetcher.test.ts`) is corrected to the real convention in the same edit | Row explicitly scopes the `__tests__/` clause to `src/node/`; the stale text is false today for Node too and contradicts Test Location. `package.json` `test` script is the authoritative source. Correcting at the point of edit is not the Go rewrite deferred to X2 | S:65 R:90 A:85 D:65 |
| 7 | Confident | The article encodes D3 only: D4 freeze, D5 layout (`go.mod`, `cmd/`, `internal/`), and any `go test` invocation are left to P2/X2 | Row scope is D3; D4/D5 are still "proposed" and belong to other rows. Keeps the constitution from pinning a layout the V3 slice review may change | S:75 R:90 A:80 D:70 |
| 8 | Confident | `context.md` gets a `## Go Transition` orientation section AND its module-layout table is corrected from flat `src/` to the real `src/node/{core,sync,tui}` tree (drop the nonexistent `sparkline.ts` row) | Row says "note the transition"; the note must reference `src/node/`, and a table beside it saying `src/` would contradict it. Layout read from disk 2026-09-15. Reversible docs-only edit | S:60 R:90 A:85 D:60 |
| 9 | Confident | `build/toolchain` is the only affected memory file; no spec change | Memory records the constitution version and the `src/node/` namespace rationale; specs are language-neutral and change at X3 per the plan | S:75 R:90 A:85 D:80 |
| 10 | Confident | The plan doc's P0 status column is not updated by this change | The file is not in this tree (unmerged `sunny-turtle` branch); the plan assigns status updates to the operator flow | S:70 R:95 A:80 D:80 |

10 assumptions (2 certain, 8 confident, 0 tentative, 0 unresolved).
