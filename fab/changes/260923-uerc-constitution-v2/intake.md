# Intake: Constitution v2.0.0 — Go Cutover

**Change**: 260923-uerc-constitution-v2
**Created**: 2026-09-23

## Origin

One-shot `/fab-new` invocation from the Go-port plan's Phase 4 queue. Raw input:

> Context: fab/plans/sahil/26-09-15-go-port.md, row X2. Read the Decisions and Target architecture sections before writing the intake. Do not change any external surface listed under Goal. Scope: cutover has shipped (v0.12.1, the Go binary is now what tu update installs). Rewrite constitution Principle III (single static Go binary, ccusage vendored beside it, no depends_on node) and Principle IV (no init-time network; the TypeScript import-timing language is irrelevant now). Rewrite the TypeScript Conventions section as Go Conventions (gofmt, internal/ layout, no globals for results, errors returned not printed, matching the src/go/internal package boundaries). Rewrite the Test Runner and Test Location constraints for go test ./..., _test.go siblings, golden files. Remove the now-obsolete Go Transition transitional article (src/node/ is no longer the shipped tree; src/go/ is). Bump constitution version to 2.0.0.

Plan row X2 (`constitution-v2`, depends on X1, size S): *"Principles III (single static Go binary, ccusage vendored beside it) and IV (no init-time network; imports irrelevant) rewritten; TypeScript Conventions → Go Conventions (gofmt, `internal/`, no globals for results, errors returned not printed); Test Runner/Location → `go test ./...`, `_test.go` siblings, golden files. Version 2.0.0."*

**Precedent.** Row P0 (`260915-n18z-constitution-go-transition`, constitution v1.2.0) added the transitional `## Go Transition` article this change removes. Its own text says: *"It is removed at cutover (v2.0.0), when Principles III and IV, the TypeScript Conventions, and the Test Runner / Test Location constraints are rewritten for the Go implementation."* That change also updated `fab/project/context.md` and hydrated `docs/memory/build/toolchain.md`; this change follows the same footprint.

**Plan decisions this row implements** (from the plan's Decisions table and Target architecture):

- **D1** — Full port: Go owns the whole CLI surface at cutover; no Go/TS hybrid, no shim exec'ing node.
- **D3** — Constitution amended first, rewritten last: "III, IV and the test sections get their Go rewrite at cutover (v2.0.0)."
- **D5** — Layout mirrors the sibling Go tools: `src/go/go.mod` (module `github.com/sahil87/tu`), `src/go/cmd/tu/`, `src/go/internal/<pkg>/`.
- **D8 / D13 / X1** — Distribution is a prebuilt per-platform tarball (`tu` + `vendor/ccusage/bin/ccusage` + `tu.default.conf`, flat) behind a generated formula with **no `depends_on "node"`**; `release.yml` pushes it to the tap. `tu update` installs the Go binary from the cutover release on.
- **D10** — `src/node/` stays in the repo until Z1 (two releases after cutover) as the differential harness's oracle and the rollback build. It is **not shipped**.
- **D12** — Watch mode is hand-rolled on `x/term`; no TUI framework. (The constitution's fast-startup posture is one of D12's stated reasons.)
- **Target architecture** — a pipeline of pure stages under `src/go/internal/`: `fact`, `source` (+ `ccusage`, `metrics`, `cache`), `query`, `view`, `render` (+ `ansi`, `json`, `csv`, `markdown`), `command`, `sync`, `config`, `watch`, `toolkit`. I/O only at the two ends (`source` and `cmd/tu` + `watch`); everything between returns values. "Typed per-source errors collected, never thrown through." "**Nothing prints**" (render). `command` returns `Result{Lines, …}` and replaces the TS `_lastRender*` globals. Golden-file tests for `render` with an `-update` flag; table-driven tests for `query` and `command` parsing.

**State of the tree the rewrite describes** (verified on disk 2026-09-23, so the apply agent can cite it without re-deriving):

- `src/go/go.mod`: `module github.com/sahil87/tu`, `go 1.26.0`, single dependency `golang.org/x/term`.
- `src/go/cmd/`: `tu` (the shipped binary), `turepair` (maintainer tool, not shipped), `tudiff`, `fakeccusage`, `fakegit` (harness). `src/go/internal/`: `command`, `config`, `fact`, `harness`, `query`, `render/{ansi,csv,json,markdown}`, `source/{ccusage,metrics,cache}`, `sync`, `toolkit`, `view`, `watch`.
- Build: `just go-build-target` uses `CGO_ENABLED=0 GOOS=… GOARCH=… go build -ldflags "-X main.version=v{{go_version}}"` — a static binary with the version stamped at link time. `just go-lint` = `gofmt -l .` (fail if non-empty) + `go vet ./...`; `just go-test` = `go test ./... -count=1`. CI lane `go-build-and-test` runs lint → build → test; `ci-gate` requires it.
- ccusage resolution (`internal/source/ccusage/exec.go` `ResolveBinary`): `os.Executable()` → `filepath.EvalSymlinks` → `<dir>/vendor/ccusage/bin/ccusage` if present, else `exec.LookPath("ccusage")`, else a typed error. The formula installs `tu`, `vendor`, `tu.default.conf` into `libexec` and symlinks `bin/tu`.
- Errors: `internal/source/error.go` — *"Sources return errors as values beside their (zero) records — they never panic, and never write to stdout/stderr: the warning line reaches an io.Writer only through WriteWarnings, called by the command edge."* `command.Result` carries `Lines []string`, `Notices []string`, `Warnings []*source.Error`, `TotalCost`, `TotalTokens`, `CostByItem`; `cmd/tu/main.go` is `os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))`.
- No `internal/` package (outside `watch`'s terminal seam) references `os.Stdout`/`os.Stderr`/`fmt.Print*` in non-test code. Package-level `var`s are compile-time tables, regexes, sentinel errors and `go:embed` payloads only — no result or render state.
- Init-time work: one `func init()` in `internal/config/setup.go` precompiles regexes from a static table. No `init()` performs I/O, exec, or network anywhere under `src/go/`.
- Tests: 23 packages under `go test ./...`; every test is a `_test.go` sibling; golden files live in `testdata/*.golden` under `render/{ansi,csv,json,markdown}` and `watch`, each test file declaring `var update = flag.Bool("update", false, "regenerate golden files")`. There is no `__tests__/` directory under `src/go/`. `e2e_test.go` in `cmd/tu` exercises the binary edge.
- Embedded assets: `internal/toolkit` embeds `skill.md` and the three completion scripts; `internal/config` embeds `tu.default.conf`; each has a drift guard against the canonical file at the repo root / `docs/site/`.

**Goal freeze** (an instruction to this change, not article content): every external surface stays fixed — the CLI grammar, `--help` text, table/JSON/CSV/Markdown output, exit codes, `tu.conf` and `org.conf`, the metrics-repo JSONL layout and its never-shrink guard, and the toolkit contracts (`--version`, `help-dump`, `update`, `shell-init`, `skill`, config-home). This change touches only `fab/project/*` (and, at hydrate, `docs/memory/build/`). It MUST NOT touch `src/`, `README.md`, `docs/site/`, `docs/specs/`, `package.json`, `scripts/`, `justfile`, `.github/`, or `harness/`.

## Why

**Problem.** The constitution (v1.2.0) still describes the Node/TypeScript artifact as the shipped one. Principle III says the CLI "MUST compile to a single ESM bundle via esbuild (`dist/tu.mjs`)"; Principle IV forbids dynamic `import()`; the `## TypeScript Conventions` section governs `tsconfig` strictness and `.js` import extensions; the Test Runner and Test Location constraints are written "For `src/node/`:" and defer Go to the transitional article. Since X1 landed and the cutover release shipped, the Homebrew formula installs the Go binary from `src/go/`, with no `node` dependency. The document that every fab review sub-agent loads as always-load context therefore describes an artifact that no longer ships, and its binding statement for the Go code is a transitional article whose own text says it is to be removed at cutover.

**Consequence of not doing it.** Every Go-only change after cutover (the backlog rows re-pointed at Go packages by Z2, and any new feature, which D4 says lands Go-only) is reviewed against TypeScript conventions it cannot satisfy and a distribution principle that names a bundle it does not produce. The reviewer either carries the same stale findings on each change or the transitional article keeps papering over them indefinitely, which is what the article was explicitly designed *not* to do. The Go-era rules that actually matter — static binary, vendored ccusage beside it, no `depends_on "node"`, pure stages that return values, `_test.go` siblings and golden files — are today spread across the plan doc, memory files, and code comments, and are binding nowhere.

**Why this approach.** Rewrite the four Node-specific clauses in place for Go and delete the transitional article, exactly as v1.2.0 promised, rather than (a) adding a second "Go" set of clauses beside the TS ones — that keeps two contradicting conventions live while `src/node/` sits frozen as a harness oracle, or (b) deferring the rewrite to Z1 when `src/node/` is deleted — that leaves the shipped binary ungoverned for two releases. The remaining `src/node/` tree needs no constitutional clause of its own: it is frozen under D4/D10 (bug fixes only, each paired with a harness fixture and a Go port), the differential harness is what verifies it, and its build/test recipes are recorded in `docs/memory/build/toolchain.md`. A **major** bump (1.2.0 → 2.0.0) is correct and was pre-announced by v1.2.0: the amendment rewrites three principles' worth of MUSTs and removes an article. Principles I, II and V's substance is language-neutral and is preserved.

## What Changes

Three files under `fab/project/`. No source, docs-site, spec, build, or CI changes.

### 1. `fab/project/constitution.md`

#### 1a. Principle III — `### III. Single-Bundle Distribution` → `### III. Single Static Binary`

Current:

```markdown
### III. Single-Bundle Distribution
The CLI MUST compile to a single ESM bundle via esbuild (`dist/tu.mjs`). Runtime dependencies are bundled — no `node_modules` required at install time. This constraint ensures Homebrew distribution stays simple.
```

New:

```markdown
### III. Single Static Binary
The CLI MUST build to a single statically linked Go binary (`CGO_ENABLED=0`, version stamped via `-ldflags "-X main.version=…"`) from `src/go/cmd/tu`. It is distributed as a prebuilt per-platform tarball containing exactly `tu`, `vendor/ccusage/bin/ccusage`, and `tu.default.conf`, installed through a generated Homebrew formula that MUST NOT declare `depends_on "node"` or any other runtime dependency. The ccusage binary the CLI runs is the one vendored beside it: resolution MUST prefer `vendor/ccusage/bin/ccusage` relative to the resolved `os.Executable()` and fall back to `PATH` only when the vendored copy is absent. The pinned ccusage version lives in `CCUSAGE_VERSION`. Data files the binary needs at runtime (`tu.default.conf`, the `skill` bundle, shell completions) MUST be embedded with `go:embed`, never read from the repo or a package directory.
```

#### 1b. Principle IV — `### IV. Fast Startup` (title kept, body rewritten)

Current:

```markdown
### IV. Fast Startup
The CLI SHOULD minimize startup latency. Heavy operations (network fetches, JSONL scanning) MUST be cached with a reasonable TTL. Imports SHOULD be static (no dynamic `import()` for core paths).
```

New:

```markdown
### IV. Fast Startup
The CLI SHOULD minimize startup latency. Package initialization (`init()` functions and package-level `var` initializers) MUST NOT perform I/O, process execution, or network access — it is limited to compile-time tables, compiled regexes, sentinel errors, and `go:embed` payloads. All fetching happens inside the command that needs it, under a context deadline, and heavy operations (ccusage execution, metrics-repo reads) MUST be cached on disk with a reasonable TTL. Dependencies SHOULD stay minimal (`golang.org/x/term` is the only non-stdlib module today); adding a module requires a stated reason.
```

The dynamic-`import()` sentence is dropped: Go has no import timing to govern, and the `go:embed` clause in III covers the "no runtime file read" concern it served.

#### 1c. `## TypeScript Conventions` → `## Go Conventions`

Current:

```markdown
## TypeScript Conventions

- Strict mode is enabled and MUST remain enabled (`"strict": true` in tsconfig)
- Target ES2022 with NodeNext module resolution — MUST use `.js` extensions in imports
- Prefer `node:` prefixed built-in imports (e.g., `node:fs`, `node:path`)
- Use `type` imports for type-only values (`import type { ... }`)
- No classes in the current codebase — prefer functions and plain objects. New code SHOULD follow this pattern unless a class is genuinely warranted
```

New:

```markdown
## Go Conventions

- The module is `src/go/` (`module github.com/sahil87/tu`). Code MUST be `gofmt`-clean and `go vet`-clean; `just go-lint` is the gate CI enforces and a PR MUST NOT merge with either failing.
- Layout follows the sibling toolkit repos: `src/go/cmd/tu/` is the only shipped `main`; all library code lives under `src/go/internal/`, one package per pipeline stage — `fact`, `source` (adapters `ccusage`, `metrics`, `cache`), `query`, `view`, `render` (encoders `ansi`, `json`, `csv`, `markdown`), `command`, `sync`, `config`, `watch`, `toolkit`. Other `cmd/` entries (`turepair`, `tudiff`, `fakeccusage`, `fakegit`) are maintainer and harness tools and are never packaged.
- The pipeline is pure between its two I/O ends. `source` (exec, files, git) and `cmd/tu` + `watch` (stdout, terminal) are the only packages that touch the outside world. `query`, `view`, and `render` MUST NOT import `os`, exec anything, or write to a stream — they take values and return values; `render` returns `[]string` or writes to an `io.Writer` handed to it. **Nothing under `internal/` prints**: `fmt.Print*`, `os.Stdout`, and `os.Stderr` appear only in `cmd/tu` and in `watch`'s terminal seam.
- No globals for results or render state. A command's outcome is the returned `command.Result` (lines, notices, warnings, totals); package-level `var`s are limited to compile-time tables, compiled regexes, sentinel errors, and `go:embed` payloads.
- Errors are returned, not printed. Data sources return a typed `*source.Error` beside their (zero) records and never panic; the command edge is the only place that turns errors into stderr lines (`source.WriteWarnings`) and exit codes. Library packages MUST NOT call `os.Exit` or `log.Fatal`.
- Dependency direction follows the pipeline: `fact` depends on nothing; `sync` depends on `fact` and `source/metrics`, never on the ccusage adapter; `command` composes `source → query → view → render`; `cmd/tu` wires adapters to `command`'s interfaces (`Fetcher`, `Repo`, `Writer`). A lower stage MUST NOT import a higher one.
- Prefer functions and plain structs; interfaces are defined by the consumer (`command`) and satisfied at the edge (`cmd/tu`), which is where the compile-time `var _ command.Fetcher = (*ccusage.Source)(nil)` assertions live.
```

#### 1d. `### Test Runner` (rewritten)

Current:

```markdown
### Test Runner
For `src/node/`: tests use the Node.js built-in test runner via `npm test` (`find src/node -path '*/__tests__/*.test.ts' -exec npx tsx --test {} +`). New test files MUST follow the `{module}.test.ts` naming convention inside a `__tests__/` folder (see Test Location). No additional test frameworks SHOULD be introduced without justification. Go tests are governed by the Go Transition article.
```

New:

```markdown
### Test Runner
Tests use the standard Go toolchain: `go test ./...` from `src/go/` (`just go-test` runs it with `-count=1`). No assertion or mocking framework SHOULD be introduced; table-driven tests with the standard `testing` package are the norm for `query`, `command` parsing, and `config`. Output encoders (`render/*`) and the watch compositor are pinned by **golden files**: expected output lives in `testdata/*.golden` beside the test, compared byte-for-byte, and regenerated only by an explicit `go test ./... -update` (each golden test file declares `var update = flag.Bool("update", false, …)`). A golden change is an output change and falls under Output Stability. The differential harness (`cmd/tudiff`, `harness/`) is the release gate for the frozen external surfaces and runs in CI as the `tudiff` lane; it complements, not replaces, unit tests.
```

#### 1e. `### Test Location` (rewritten)

Current:

```markdown
### Test Location
For `src/node/`: test files MUST be co-located with the source code they test, in `__tests__/` folders within the same directory (e.g., `src/node/core/__tests__/fetcher.test.ts` for `src/node/core/fetcher.ts`). This clause does not apply to `src/go/` — see the Go Transition article.
```

New:

```markdown
### Test Location
Test files MUST be `_test.go` siblings of the code they test, in the same package directory (e.g., `src/go/internal/source/ccusage/exec_test.go` for `exec.go`). Golden files and fixtures live in that package's `testdata/` directory. `__tests__/` folders MUST NOT exist anywhere under `src/go/`. Binary-level (end-to-end) tests live beside `main.go` in `src/go/cmd/tu/`.
```

#### 1f. Principle V — type names only

Current:

```markdown
### V. Consistent Data Model
All data flows through `UsageEntry` and `UsageTotals` interfaces. New data sources MUST conform to these types. Aggregation (daily-to-monthly, merge) MUST be pure functions operating on these types. The `label` field MUST use ISO date format (`YYYY-MM-DD` or `YYYY-MM`).
```

New (substance unchanged; the TS type names no longer exist in the shipped tree):

```markdown
### V. Consistent Data Model
All data flows through the one fact type: `fact.Record` (`{Date, Tool, User, Machine, Totals}`) and `fact.Totals`. New data sources MUST produce `[]fact.Record`. Aggregation (daily-to-weekly/monthly roll-up, merge, group-by) MUST be pure functions operating on these types. The `Date` label MUST use ISO date format (`YYYY-MM-DD` or `YYYY-MM`).
```

#### 1g. Remove `## Go Transition`

Delete the entire `## Go Transition` section (the blockquote and its six bullets). Nothing replaces it: the rewritten clauses above are the binding statement for `src/go/`, and the frozen-external-surfaces sentence it carried is already stated by Output Stability and Toolkit Standards. The one fact from the article that still needs a home — `src/node/` remains in the repo, unshipped, until Z1 — is recorded in `context.md` (§ 3 below) and `docs/memory/build/toolchain.md`, not in the constitution.

#### 1h. Unchanged

`### I. Single-Purpose CLI`, `### II. Graceful Degradation`, `### Test Integrity`, `### Output Stability`, `### Toolkit Standards` are byte-identical to v1.2.0. Section order is unchanged: Core Principles → Go Conventions → Additional Constraints → Governance.

#### 1i. Governance line

Current: `**Version**: 1.2.0 | **Ratified**: 2026-03-06 | **Last Amended**: 2026-09-15`
New: `**Version**: 2.0.0 | **Ratified**: 2026-03-06 | **Last Amended**: 2026-09-23`

### 2. `fab/project/code-quality.md` — drop the three TypeScript-only bullets

`## Principles` currently carries "Use `type` imports for type-only values" and "Use `node:` prefixed imports for built-in modules"; `## Anti-Patterns` carries "Dynamic `import()` for core paths (hurts startup latency)". These contradict the new Go Conventions for the tree fab now reviews. Replace them with the Go-shaped equivalents so the apply/review agents' guidance matches the constitution:

- Principles: replace the two TS bullets with one — "Follow the constitution's Go Conventions: pure stages return values, errors are returned not printed, no result globals"
- Anti-Patterns: replace the dynamic-`import()` bullet with "I/O, exec, or network in `init()` or package-level initializers (Constitution IV)"

Everything else in the file (readability, existing patterns, functions over classes, minimum pathways, god functions, duplication, magic numbers, swallowed errors, `test-alongside`) is language-neutral and stays.

### 3. `fab/project/context.md` — Stack and the two-tree note

Replace `## Stack` and `## Go Transition`, and retitle the module-layout subsection. The TypeScript layout table is removed in favour of the Go package table (the TS tree is documented in `docs/memory/build/toolchain.md` and is frozen). New content:

```markdown
## Stack

- **Language**: Go (`src/go/go.mod`, `module github.com/sahil87/tu`, go 1.26; sole dependency `golang.org/x/term`)
- **Build**: `just go-build` (dev binary at `bin/tu`), `just go-build-all` (four static cross-compiled targets, `CGO_ENABLED=0`), version via `-ldflags`
- **Lint / test**: `just go-lint` (`gofmt -l` + `go vet ./...`), `just go-test` (`go test ./... -count=1`); golden files regenerate with `go test ./... -update`
- **Task runner**: justfile
- **Distribution**: Homebrew tap (`sahil87/tap`), binary name `tu`; prebuilt `tu-go-<os>-<arch>.tar.gz` release assets (binary + vendored ccusage + `tu.default.conf`) behind a generated formula with no runtime dependencies
- **License**: MIT

## Retired TypeScript tree

`src/node/` is the pre-cutover TypeScript implementation. It no longer ships (the formula flipped at plan row X1, plan: `fab/plans/sahil/26-09-15-go-port.md`) and is frozen: bug fixes only, each paired with a harness fixture and a Go port. It remains in the repo until plan row Z1 (two releases after cutover) as the differential harness's oracle (`node dist/tu.mjs` vs the Go binary via `cmd/tudiff`) and the D10 rollback build. Its toolchain (`npm ci && npm run build && npm test`, esbuild, `__tests__/` co-location) is recorded in `docs/memory/build/toolchain.md`; the constitution no longer governs it.
```

And the architecture section's layout table becomes the Go package table:

```markdown
### Package layout (`src/go/internal/`)

| Package | Responsibility |
|---------|---------------|
| `fact` | `Record` / `Totals`, the six-tool registry |
| `source` | Typed `Error`, `WriteWarnings`; adapters `ccusage` (exec + normalize, vendor-first binary resolution), `metrics` (metrics-repo reader), `cache` (hash-keyed JSON, 60 s TTL) |
| `query` | Pure: window filter, roll-up, `MaxMerge`, `Collapse` over one `GroupBy` |
| `view` | Pure: query result → ANSI-free table models, bars, deltas, leaderboard |
| `render` | Encoders to `[]string` / `io.Writer`: `ansi`, `json`, `csv`, `markdown` — nothing prints |
| `command` | `Request` parsing, `Run` composing source→query→view→render into `Result`; the `Fetcher`/`Repo`/`Writer` seams |
| `sync` | Metrics-repo writer (never-shrink guard), git driver, dry-run report, repair |
| `config` | `tu.conf` / `org.conf` cascade, config-home, `init-conf`, `status`; embeds `tu.default.conf` |
| `watch` | `x/term` TUI loop, compositor, rain, panel |
| `toolkit` | `--version`, `help-dump`, `update`, `shell-init`, `skill` (embedded), completions |
| `harness` | `tudiff` matrix, fixtures, expected-diff ledger (maintainer tooling) |

`cmd/tu` is the only writer to stdout/stderr and the only shipped binary; `cmd/turepair`, `cmd/tudiff`, `cmd/fakeccusage`, `cmd/fakegit` are maintainer/harness tools. Tests are `_test.go` siblings; golden files sit in each package's `testdata/`.
```

The `## Architecture` intro paragraph (the ccusage sources list) and `### Modes` are unchanged.

### 4. Not in this change

- **`fab/project/config.yaml`** — `source_paths` keeps both `src/node/` and `src/go/` and `test_paths` keeps the JS/TS patterns: `src/node/` still exists and can still receive harness-driven fixes until Z1. Z1 prunes them.
- **`README.md` CI section** (still says "unshipped Go successor") and **`docs/site/skill.md` / `docs/specs`** toolchain lines — row X3 (`memory-rehydrate`) owns the docs sweep; README is a Toolkit Standards surface and is not touched here.
- **`docs/memory/**` at apply time** — hydrate updates `build/toolchain.md` (Affected Memory); the 18-file rewrite is X3.
- **The plan doc's X2 status column** — the operator flow updates it when the row lands.
- **`justfile`, `.github/`, `scripts/`, `src/`** — no build or code change; the constitution describes the tree as it is.

## Affected Memory

- `build/toolchain`: (modify) Update the constitution reference in the requirements list (line "the two-tree period is governed by constitution v1.2.0 § Go Transition (rewritten for the shipped Go binary by plan row X2)") to state that constitution v2.0.0 (Last Amended 2026-09-23) governs `src/go/` directly — Principles III (Single Static Binary), IV, the Go Conventions, and the Go Test Runner / Test Location constraints — and that `src/node/` is unshipped and frozen until Z1. Rewrite the **Go Transition: two implementation trees in `main`** design decision as a closed decision: keep its Why, add that the article was removed by this change at v2.0.0 as it promised, and record the four rewritten clauses (*Introduced by*: 260915-n18z; *Superseded by*: 260923-uerc). Update the toolkit-standards posture line's constitution pin from `v1.2.0 … Last Amended 2026-09-15` to `v2.0.0 … Last Amended 2026-09-23`. The Overview's "`src/node/` tree — the differential harness's oracle and the D10 rollback build until Z1" sentence already states the post-cutover roles and needs no change.

## Impact

- **Files**: `fab/project/constitution.md` (≈ 30 lines changed, one section removed), `fab/project/code-quality.md` (3 bullets), `fab/project/context.md` (Stack, transition note, layout table); hydrate: `docs/memory/build/toolchain.md` + regenerated `docs/memory/build/index.md`.
- **Runtime behavior**: none. No source, build, test, release, or CI change. The tarball, formula, `tu update`, and every Goal-frozen surface are untouched.
- **fab pipeline behavior**: every subsequent change's review sub-agent loads the v2.0.0 constitution. Go code is reviewed against Go Conventions (gofmt/vet gate, `internal/` boundaries, pure middle stages, returned errors, no result globals) and the Go test constraints; a change that adds `depends_on`, a runtime dependency, an `init()` with I/O, a print in `internal/`, or a `__tests__/` folder becomes a constitutional finding. `src/node/` edits (harness-fixture-driven fixes until Z1) are no longer covered by a TS convention clause; the differential harness and `docs/memory/build/toolchain.md` govern them.
- **Downstream rows**: X3 (`memory-rehydrate`) and Z2 (`backlog-unfreeze`) inherit the Go-era constitution; Z1 (`remove-src-node`) prunes `config.yaml`'s `src/node/` and JS/TS `test_paths` entries and retires the "Retired TypeScript tree" note.
- **Verification**: `fab preflight` still parses; a read-through confirms Principles I, II, Test Integrity, Output Stability, and Toolkit Standards are byte-identical to v1.2.0 and that no `## Go Transition` heading remains; every path, recipe, and identifier named in the new clauses exists on disk (`just go-lint`, `just go-test`, `CCUSAGE_VERSION`, `internal/source/ccusage/exec.go` `ResolveBinary`, `command.Result`, `source.WriteWarnings`, the `-update` flags, `testdata/*.golden`).

## Open Questions

None. The row, D1/D3/D5/D8/D10/D12/D13, the Target architecture table, and the tree on disk determine every clause; the three scope additions beyond the row's literal list (Principle V type names, `code-quality.md` bullets, `context.md`) are recorded as graded assumptions below and can be struck via `/fab-clarify`.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Constitution version 1.2.0 → 2.0.0, Last Amended 2026-09-23, Ratified unchanged | Stated in the row and pre-announced by the v1.2.0 article; a major bump matches rewriting three MUST clauses and removing an article | S:95 R:95 A:95 D:95 |
| 2 | Certain | The `## Go Transition` article is deleted outright, with no replacement section | The row says remove; the article's own text says it is removed at v2.0.0; the surfaces it froze are already stated by Output Stability and Toolkit Standards | S:95 R:90 A:95 D:90 |
| 3 | Certain | Principle III names the tarball contents (`tu`, `vendor/ccusage/bin/ccusage`, `tu.default.conf`), `CGO_ENABLED=0`, vendor-first resolution relative to `os.Executable()`, and forbids `depends_on "node"` | Row text plus `package-go.sh`, `formula-template.rb`, `justfile go-build-target`, and `exec.go ResolveBinary` read on disk 2026-09-23 | S:90 R:90 A:95 D:90 |
| 4 | Certain | Principle IV drops the dynamic-`import()` sentence and instead forbids I/O/exec/network in `init()` and package-level initializers; caching MUST stays | Row: "no init-time network; imports irrelevant". The one `init()` on disk precompiles regexes, so the rule describes the tree as-is. The caching MUST is language-neutral and cited by go-port memory (command-edge) | S:80 R:90 A:90 D:80 |
| 5 | Certain | Go Conventions encode the Target-architecture boundaries as MUSTs: `source` and `cmd/tu`+`watch` are the only I/O packages; `query`/`view`/`render` never import `os` or print; lower stages never import higher ones | Row: "matching the src/go/internal package boundaries"; the plan's Target architecture states "I/O only at the two ends" and "Nothing prints"; grep of `internal/` confirms it holds today. Reversible wording | S:80 R:85 A:85 D:75 |
| 6 | Certain | "No globals for results" is worded to permit compile-time tables, regexes, sentinel errors, and `go:embed` payloads as package-level `var`s | A blanket "no package-level var" would make 40+ existing declarations (registries, regexes, `ErrUnported`, embedded assets) violations; the TS problem being replaced was `_lastRender*` result state specifically | S:70 R:90 A:90 D:80 |
| 7 | Certain | Test Runner names golden files (`testdata/*.golden`, `-update` flag) and ties a golden change to Output Stability; the differential harness is named as complementary, not a substitute | Row: "golden files"; five `-update` flags and five `testdata/` dirs exist; a golden diff is by construction an output diff. Harness sentence keeps D6's role visible without constitutionalizing its mechanics | S:80 R:90 A:90 D:80 |
| 8 | Confident | Principle V's type names change from `UsageEntry`/`UsageTotals` to `fact.Record`/`fact.Totals`; substance (single type, pure aggregation, ISO labels) unchanged | Not in the row's list, but a v2.0.0 Go constitution naming TS interfaces that no longer ship is stale text at the point of a full rewrite (the n18z precedent corrected stale text in passing). Reversible one-sentence edit | S:40 R:90 A:85 D:60 |
| 9 | Confident | `fab/project/context.md` is rewritten (Go stack, retired-TS note, Go package table) in this change | Its current text says "The constitution's Go Transition article (v1.2.0) is the binding statement" — a dangling reference once the article is deleted — and its Stack calls TypeScript the shipped implementation. The n18z precedent edited `context.md` alongside the constitution. Docs-only, reversible | S:55 R:90 A:85 D:65 |
| 10 | Confident | `fab/project/code-quality.md`'s three TypeScript-only bullets are replaced with Go equivalents | They would contradict the new Go Conventions in the same always-load layer every apply/review agent reads. Three bullets, reversible | S:45 R:90 A:80 D:60 |
| 11 | Certain | `config.yaml` `source_paths`/`test_paths` keep `src/node/` and the JS/TS patterns until Z1 | `src/node/` still exists and receives harness-driven fixes (D4/D10); pruning belongs to `remove-src-node` | S:75 R:95 A:90 D:85 |
| 12 | Certain | `build/toolchain` is the only affected memory file; no spec change | It holds the constitution version pin and the Go Transition design decision; go-port memory cites Principles IV/V by number, which survive; specs are language-neutral and belong to X3 | S:75 R:90 A:85 D:80 |
| 13 | Certain | README, `docs/site/skill.md`, `docs/specs`, and the plan's X2 status column are not touched | README is a Toolkit Standards surface and the docs sweep is X3's stated scope; the plan assigns status updates to the operator flow | S:75 R:95 A:85 D:85 |

13 assumptions (10 certain, 3 confident, 0 tentative, 0 unresolved).
