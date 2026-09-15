# Plan: Go Scaffold

**Change**: 260915-h1iv-go-scaffold
**Intake**: `intake.md`

## Requirements

### Build: Go module

#### R1: Module root at `src/go/`
The Go successor implementation SHALL live in a single module at `src/go/go.mod` declaring `module github.com/sahil87/tu` and `go 1.25`, with no `require` block and no `go.sum` (zero third-party dependencies). No `src/go/internal/` directory is created by this change.

- **GIVEN** a checkout of `main` after this change
- **WHEN** `cd src/go && go build ./...` runs under Go ≥ 1.25
- **THEN** it succeeds with no network access and no `go.sum` is produced

### CLI: version contract

#### R2: `--version` prints the toolkit version line
`src/go/cmd/tu/main.go` MUST handle `--version`, `-V`, and `-v` by writing exactly one line `tu version <v>` to stdout and exiting 0, with nothing on stderr and no file or network I/O. `<v>` is the package-level `var version = "dev"`, overridable via `-ldflags "-X main.version=…"`. When the value begins with a digit a `v` is prefixed; otherwise it is printed as-is (so `dev` stays `dev`, never `vdev`).

- **GIVEN** a binary built with `-X main.version=0.11.5` or `-X main.version=v0.11.5`
- **WHEN** invoked as `tu --version` (or `-V`, `-v`)
- **THEN** stdout is exactly `tu version v0.11.5\n`, stderr is empty, exit code is 0
- **AND** the first non-empty stdout line matches `^tu version v\d+(\.\d+)*$` (same regex as `src/node/core/__tests__/cli-version.test.ts`)

- **GIVEN** a binary built with no ldflags
- **WHEN** invoked as `tu --version`
- **THEN** stdout is exactly `tu version dev\n`

#### R3: Every other invocation is a not-implemented placeholder
Any invocation that is not one of the three version flags (no args, `--help`, `cc`, any positional grammar) MUST write `tu: not implemented (Go port in progress)` to stderr and exit 1, with stdout empty. This is the starting state plan row P4's differential harness expects.

- **GIVEN** the Go binary
- **WHEN** invoked as `tu`, `tu cc`, or `tu --help`
- **THEN** exit code is 1, stdout is empty, stderr is non-empty

#### R4: Pinning test
`src/go/cmd/tu/main_test.go` MUST be a `_test.go` sibling (no `__tests__/` under `src/go/`) that pins R2 and R3 through a pure `versionLine(v string) string` and a `run(args []string, stdout, stderr io.Writer) int` seam, without spawning the binary.

- **GIVEN** `cd src/go`
- **WHEN** `go test ./...` runs
- **THEN** it passes, covering: `versionLine` for `v0.11.5`, `0.11.5`, `dev`; `run` for `--version`/`-V`/`-v` (exit 0, exact stdout, empty stderr, regex match); `run` for `[]`, `cc`, `--help` (exit 1, empty stdout, non-empty stderr)

### Build: justfile recipes

#### R5: `go-build`, `go-test`, `go-lint`
The `justfile` SHALL gain three recipes, leaving the existing `setup`/`test`/`run`/`build`/`release`/`release-notes` recipes byte-for-byte untouched:
- `go-build`: stamps `-X main.version=v<package.json version>` (read via `node -p 'require("./package.json").version'`) and writes the binary to `bin/tu`.
- `go-test`: `cd src/go && go test ./... -count=1`.
- `go-lint`: fails when `gofmt -l .` under `src/go` prints any file, then runs `go vet ./...`.
`.gitignore` SHALL gain `bin/`.

- **GIVEN** a checkout with Go and node on PATH
- **WHEN** `just go-lint && just go-build && just go-test` runs
- **THEN** all three succeed, `bin/tu --version` prints `tu version v0.11.5` (the current `package.json` version), and `git status` shows no `bin/` entry

### CI: Go lane

#### R6: `go-build-and-test` job gated by `ci-gate`
`.github/workflows/ci.yml` SHALL gain a job `go-build-and-test` (`ubuntu-latest`; pinned `actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5`, `actions/setup-go@40f1582b2485089dde7abd97c1529aa768e1baff` with `go-version-file: src/go/go.mod` and `cache: false`, `extractions/setup-just@dd310ad5a97d8e7b41793f8ef055398d51ad4de6`) whose steps run `just go-lint`, `just go-build`, `just go-test`. `ci-gate` MUST list `needs: [build-and-test, go-build-and-test]` and fail unless both results are `success`. The existing `build-and-test` job, the `on:` triggers, and `permissions` MUST be unchanged.

- **GIVEN** a PR touching `src/go/`
- **WHEN** CI runs
- **THEN** `go-build-and-test` executes the three recipes and `ci-gate` reports failure if either lane fails

### Docs: plan bookkeeping

#### R7: Plan row P2 status
`fab/plans/sahil/26-09-15-go-port.md` row P2 SHALL carry this change's folder name in the PR column and a landed note in Status, matching row P1's convention. No other plan edits.

- **GIVEN** the plan doc
- **WHEN** row P2 is read
- **THEN** its PR column reads `260915-h1iv-go-scaffold`

### Non-Goals

- No `src/go/internal/` packages, cobra, or toolkit files (`help_dump.go`, `update.go`, …) — those are V1/B8.
- No change to `scripts/build.sh`, `release.yml`, the formula, `dist/`, `package.json`, or any Node source or test.
- No README or `docs/site/` change.

### Design Decisions

#### package.json is the version anchor for both binaries during the transition
**Decision**: `just go-build` stamps the Go binary from `package.json`'s version, not from `git describe`.
**Why**: The TS binary prints `v` + the package.json version, and the P4 differential harness byte-diffs `--version`; `release.sh`/`release.yml` also derive the `v*` tag from package.json, so it is the single anchor until Z1 removes it.
**Rejected**: `git describe --tags --always` as the siblings do — yields `v0.11.5-3-gabc123` on non-tag commits and would break harness parity.
*Introduced by*: 260915-h1iv-go-scaffold

#### Go build output goes to `bin/`, not `dist/`
**Decision**: `just go-build` writes `bin/tu`; `bin/` is gitignored.
**Why**: `dist/` is the shipped Node artifact and is listed in `package.json` `files`, so a Go binary there would be swept into the npm pack and blur the "Go is unshipped" line.
**Rejected**: `dist/tu-go` — same packaging hazard; `dist/bin/` as fab-kit does — fab-kit has no competing shipped artifact in `dist/`.
*Introduced by*: 260915-h1iv-go-scaffold

#### Zero-dependency scaffold; CI runs the just recipes
**Decision**: No cobra or other module dependency in P2; the CI job invokes `just go-lint`/`go-build`/`go-test` rather than inlining `go` commands, and `setup-go` runs with `cache: false`.
**Why**: Cobra vs. a hand-rolled positional parser is a V2/B8 `command`-package decision; one definition of "lint" (code-quality: minimum pathways); `setup-go`'s cache needs a `go.sum` that does not exist yet.
**Rejected**: Pulling cobra now — a dependency with no P2 consumer; inlining gofmt/vet in YAML as the siblings do — duplicates the justfile definition.
*Introduced by*: 260915-h1iv-go-scaffold

## Tasks

### Phase 1: Setup

- [x] T001 Create `src/go/go.mod` (`module github.com/sahil87/tu`, `go 1.25`, no requires) and `src/go/cmd/tu/main.go` with `var version = "dev"`, `versionLine(v string) string`, `run(args []string, stdout, stderr io.Writer) int`, and `main()`; version flags print `tu version <v>` to stdout exit 0, everything else prints `tu: not implemented (Go port in progress)` to stderr exit 1; package doc comment cites constitution § Go Transition and the plan <!-- R1, R2, R3 -->
- [x] T002 Create `src/go/cmd/tu/main_test.go` — table-driven tests over `versionLine` and `run` per R4; run `cd src/go && gofmt -l . && go vet ./... && go test ./...` <!-- R4 -->

### Phase 2: Core Implementation

- [x] T003 [P] Append `go_version` variable and `go-build`/`go-test`/`go-lint` recipes to `justfile` per R5 (existing recipes untouched); add `bin/` to `.gitignore`; verify `just go-lint && just go-build && just go-test` and `bin/tu --version` locally <!-- R5 -->
- [x] T004 [P] Add the `go-build-and-test` job to `.github/workflows/ci.yml` and extend `ci-gate` (`needs` + second result check) per R6; the `build-and-test` job stays byte-identical; validate YAML parses <!-- R6 -->

### Phase 3: Integration & Edge Cases

- [x] T005 Fill row P2's PR column with `260915-h1iv-go-scaffold` and a landed Status note in `fab/plans/sahil/26-09-15-go-port.md` (P1 convention) <!-- R7 -->

## Acceptance

### Functional Completeness

- [x] A-001 R1: `src/go/go.mod` declares `module github.com/sahil87/tu` and `go 1.25`, has no `require` block, and no `go.sum` exists
- [x] A-002 R2: `run(["--version"])`, `run(["-V"])`, `run(["-v"])` write exactly `tu version <v>\n` to stdout, nothing to stderr, and return 0
- [x] A-003 R3: `run([])`, `run(["cc"])`, `run(["--help"])` return 1 with empty stdout and `tu: not implemented (Go port in progress)` on stderr
- [x] A-004 R4: `src/go/cmd/tu/main_test.go` exists as a sibling, and `cd src/go && go test ./...` passes
- [x] A-005 R5: `justfile` has `go-build`, `go-test`, `go-lint` with the specified semantics and the six pre-existing recipes are unchanged; `.gitignore` contains `bin/`
- [x] A-006 R6: `ci.yml` has a `go-build-and-test` job with the pinned action SHAs, `go-version-file: src/go/go.mod`, `cache: false`, and three `just go-*` steps; `ci-gate` needs both jobs and checks both results
- [x] A-007 R7: plan row P2's PR column reads `260915-h1iv-go-scaffold`

### Behavioral Correctness

- [x] A-008 R2: `versionLine("0.11.5")` and `versionLine("v0.11.5")` both yield `tu version v0.11.5`; `versionLine("dev")` yields `tu version dev`
- [x] A-009 R5: `just go-build` produces `bin/tu` whose `--version` output equals `tu version v<package.json version>` (currently `v0.11.5`)

### Scenario Coverage

- [x] A-010 R2: a test asserts the stamped `--version` first line matches `^tu version v\d+(\.\d+)*$`
- [x] A-011 R6: the `build-and-test` job block in `ci.yml` is byte-identical to `main`'s (`git diff origin/main -- .github/workflows/ci.yml` touches only the new job, the `ci-gate` needs/check, and at most the header comment)

### Edge Cases & Error Handling

- [x] A-012 R2: `main.go` is `gofmt`-clean and `go vet`-clean; `just go-lint` exits 0
- [x] A-013 R1: no `src/go/internal/` directory and no `__tests__/` directory exist under `src/go/`

### Code Quality

- [x] A-014 Pattern consistency: `main.go` mirrors the siblings' `var version = "dev"` / `-X main.version` convention and the toolkit `version` standard's `<tool> version vX.Y.Z` shape
- [x] A-015 No unnecessary duplication: lint logic lives once in the `go-lint` recipe; CI calls the recipe rather than restating it
- [x] A-016 Readability: `main.go` is short, single-purpose, with no magic strings beyond the two named message constants
- [x] A-017 No silent error swallowing: the not-implemented path warns on stderr and exits non-zero rather than printing nothing

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Deletion Candidates

None — this change adds new functionality without making existing code redundant.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | `run` treats only `--version`/`-V`/`-v` as version flags, matched anywhere in args (mirroring `cli.ts` `rawArgs.includes`) | TS checks `includes`, not position; keeping the same rule preserves harness parity for odd orderings like `tu cc --version` | S:60 R:95 A:85 D:75 |
| 2 | Confident | `go test ./... -count=1` without `-race` | Nothing concurrent in P2; `-race` can be added when goroutines arrive (V1) | S:50 R:100 A:85 D:80 |
| 3 | Confident | The ci.yml header comment gains one line mentioning the Go lane; nothing else outside the new job and `ci-gate` changes | Keeps the "Node CI untouched" promise while leaving the file's self-description accurate | S:55 R:100 A:90 D:80 |

3 assumptions (0 certain, 3 confident, 0 tentative).
