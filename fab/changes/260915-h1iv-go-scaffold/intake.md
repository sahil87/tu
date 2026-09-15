# Intake: Go Scaffold

**Change**: 260915-h1iv-go-scaffold
**Created**: 2026-09-16

## Origin

One-shot `/fab-new` invocation, handed over as row **P2** of the Go-port plan (`fab/plans/sahil/26-09-15-go-port.md`). Raw input:

> Context: fab/plans/sahil/26-09-15-go-port.md, row P2. Read the Decisions and Target architecture sections before writing the intake. Do not change any external surface listed under Goal.
>
> Row P2 (go-scaffold): D5 layout — src/go/go.mod (module github.com/sahil87/tu, go 1.25), src/go/cmd/tu/main.go printing "tu version vX.Y.Z" from ldflags. Add just go-build, just go-test, just go-lint recipes. Add a CI job go-build-and-test to .github/workflows/ci.yml and add it to the ci-gate needs list. Node CI stays untouched. This depends on P0 (constitution amendment already merged, so config.yaml already lists src/go/ in source_paths and **/*_test.go in test_paths).

Context read before writing this intake: the plan's **Decisions** (D1–D13) and **Target architecture** sections; constitution v1.2.0 § Go Transition; `docs/memory/build/toolchain.md`; the current `justfile`, `.github/workflows/ci.yml`, `release.yml`, `scripts/build.sh`, `scripts/release.sh`; the sibling repos `idea` and `hop` (`src/go.mod`, `src/cmd/<tool>/main.go`, `justfile`, `ci.yml`, `scripts/build.sh`) and `fab-kit` (`src/go/` precedent, `justfile`, `ci.yml`); `shll standards version`. P0 (`260915-n18z-constitution-go-transition`, PR #78) is merged: the constitution carries the Go Transition article, `config.yaml` lists `src/go/` in `source_paths` and `**/*_test.go` in `test_paths`, and `fab/project/context.md` has the orientation note. `src/go/` does not exist yet in `main`.

## Why

**Problem.** The Go port (plan D1–D13) lands change by change into `main` under `src/go/` (D2), but there is no Go module, no entry point, no way to build or test Go code locally, and no CI lane that compiles it. Every Phase 0/1/2 row after this one (P3a harness capture, P4 differential harness, V1 `fact`/`source`, V2 `query`/`view`/`render`, …) assumes it can drop a package under `src/go/internal/` or a command under `src/go/cmd/` and have `just go-test` and the `ci-gate` required check exercise it. Without the scaffold each of those rows would have to invent the module layout, the version stamping, and the CI wiring itself, and they would do it inconsistently.

**Consequence of not doing it.** Rows P3a and P4 (both `Depends: P2`) and V1 cannot start. Any Go code merged before a CI lane exists is unverified in `main`, which is exactly the "lands dark but gated by CI" guarantee D2 relies on.

**Why this shape.** D5 fixes the layout: mirror the six sibling Go tools (`src/cmd`, `src/internal`, `go.mod` at the module root, version via `-ldflags "-X main.version=…"`, `var version = "dev"` fallback), shifted one level to `src/go/` because `src/node/` already occupies the shipped tree (fab-kit is the `src/go/` precedent). The scaffold is deliberately **minimal**: one module, one `main.go` that implements only the toolkit `version` contract, one test, three `just` recipes, one CI job. It pulls **no dependencies** (no cobra yet — whether the positional grammar is parsed by cobra or by hand is a V2/B8 `command`-package decision, and adding a dependency later is a one-line `go.mod` edit). The Go binary is **not shipped** (constitution § Go Transition): the formula, `dist/tu.mjs`, `scripts/build.sh`, `release.yml`, and the Node CI job are untouched.

## What Changes

### 1. Go module — `src/go/go.mod`

Create the module root at `src/go/` (D5):

```
module github.com/sahil87/tu

go 1.25
```

No `require` block and no `go.sum` — the scaffold has zero third-party dependencies. `go 1.25` is the language/toolchain floor; CI resolves it via `setup-go`'s `go-version-file`. No `src/go/internal/` directory is created in this change (Go/git cannot track an empty directory; V1 creates the first package there).

### 2. Entry point — `src/go/cmd/tu/main.go`

Package `main`. Responsibilities, and nothing more:

- `var version = "dev"` — overridden at build time via `-ldflags "-X main.version=…"`, exactly as `idea`/`hop` do.
- `--version`, `-V`, `-v` → print exactly one line to **stdout**, `tu version vX.Y.Z`, exit `0`, nothing on stderr, no network or file I/O. This is the toolkit `version` standard's RECOMMENDED shape and byte-matches the TS binary (`src/node/core/cli.ts` line ~1843: `tu version ${v}` where `v` is `PKG_VERSION` with a `v` prefix ensured). Normalize with a digit-only rule: prefix `v` when `version` starts with a digit, so `-X main.version=0.11.5` and `-X main.version=v0.11.5` both yield `tu version v0.11.5`, while the unstamped fallback prints `tu version dev` (sibling behavior; `dev` is not a `v`-token and stays unprefixed).
- **Every other invocation** (including no args, `--help`, any positional grammar) → write `tu: not implemented (Go port in progress)` to **stderr** and exit `1`. This is the deliberate "Go side prints unimplemented" state plan row P4 expects the differential harness to start from. It is a placeholder, not a contract — V2 replaces it.

Structure the file so the behavior is testable without spawning the binary: a pure `versionLine(v string) string` and a `run(args []string, stdout, stderr io.Writer) int` that `main()` calls with `os.Args[1:]`, `os.Stdout`, `os.Stderr`, then `os.Exit`s with the returned code. Keep it `gofmt`-clean and `go vet`-clean under Go 1.25.

Add a package doc comment stating: this is the successor implementation (constitution v1.2.0 § Go Transition), built and tested in CI, not shipped until cutover; plan reference `fab/plans/sahil/26-09-15-go-port.md`.

### 3. Test — `src/go/cmd/tu/main_test.go`

`_test.go` sibling in the same directory (constitution § Go Transition; no `__tests__/` under `src/go/`). Table-driven, covering:

| Case | Expect |
|------|--------|
| `versionLine("v0.11.5")` | `tu version v0.11.5` |
| `versionLine("0.11.5")` | `tu version v0.11.5` |
| `versionLine("dev")` | `tu version dev` |
| `run(["--version"])`, `run(["-V"])`, `run(["-v"])` with `version` set to `v1.2.3` | exit `0`, stdout is exactly `tu version v1.2.3\n`, stderr empty |
| stamped-shape check | the first non-empty stdout line matches `^tu version v\d+(\.\d+)*$` (the same regex as `src/node/core/__tests__/cli-version.test.ts`) |
| `run([])`, `run(["cc"])`, `run(["--help"])` | exit `1`, stdout empty, stderr non-empty |

This pins the `version` standard ("keep a minimal test pinning exit 0, version on line 1, matches the shape") on the Go side from day one.

### 4. `justfile` recipes — `go-build`, `go-test`, `go-lint`

Append three recipes; the existing `setup`, `test`, `run`, `build`, `release`, `release-notes` recipes are **untouched** (they remain the Node lanes). Recipe names are prefixed `go-` because the bare names are taken by the shipped Node tree.

```just
# ── Go successor (src/go/) — built and tested, NOT shipped until cutover ──
# Constitution v1.2.0 § Go Transition; plan fab/plans/sahil/26-09-15-go-port.md.

# Version stamp for the Go binary. package.json is the single version anchor
# during the transition (release.sh bumps it; the v* tag is derived from it),
# and the TS binary prints exactly this value — so the differential harness
# (P4) byte-matches `--version` across both implementations. Z1 switches this
# to `git describe` when package.json goes away.
go_version := `node -p 'require("./package.json").version'`

# Build the Go binary into bin/tu (gitignored; NOT dist/ — dist/ is the shipped Node artifact).
go-build:
    mkdir -p bin
    cd src/go && go build -ldflags "-X main.version=v{{go_version}}" -o ../../bin/tu ./cmd/tu

# Run the Go test suite under src/go/.
go-test:
    cd src/go && go test ./... -count=1

# gofmt + go vet over src/go/ (the same two checks the sibling Go tools gate CI on).
go-lint:
    #!/usr/bin/env bash
    set -euo pipefail
    cd src/go
    unformatted="$(gofmt -l .)"
    if [ -n "$unformatted" ]; then
        echo "The following files are not gofmt-clean:" >&2
        echo "$unformatted" >&2
        echo "Run: (cd src/go && gofmt -w .)" >&2
        exit 1
    fi
    go vet ./...
```

Exact recipe bodies may be adjusted at apply for `just` syntax, but the semantics are fixed: `go-build` stamps `v<package.json version>` via `-X main.version`, outputs `bin/tu`; `go-test` is `go test ./...`; `go-lint` is `gofmt -l` (fail on any output) + `go vet ./...`. No `golangci-lint` — the siblings do not use it and it is not installed on the dev machines.

### 5. `.gitignore`

Add `bin/` (the `just go-build` output). `dist/` is already ignored but is not used for the Go binary: `package.json` `files: ["dist/", …]` would sweep a Go binary into the npm pack, and `dist/` is definitionally the shipped Node artifact.

### 6. CI — `.github/workflows/ci.yml`

Add one job, **`go-build-and-test`**, alongside the existing `build-and-test` (which is byte-for-byte untouched). Steps:

```yaml
  go-build-and-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5 # v4

      - uses: actions/setup-go@40f1582b2485089dde7abd97c1529aa768e1baff # v5
        with:
          # Single source of truth for the Go version. Pinning to go.mod also
          # pins gofmt's rules, so the lint check matches the toolchain that
          # wrote the files.
          go-version-file: src/go/go.mod
          # No go.sum yet (zero dependencies) — setup-go's cache needs one.
          # Flip to `cache-dependency-path: src/go/go.sum` when the first
          # dependency lands.
          cache: false

      - uses: extractions/setup-just@dd310ad5a97d8e7b41793f8ef055398d51ad4de6 # v2

      - name: Lint (gofmt + go vet)
        run: just go-lint

      - name: Build
        run: just go-build

      - name: Test
        run: just go-test
```

The job runs the `just` recipes rather than inlining `go` commands so that "what lint means" has one definition (code-quality: minimum pathways). `setup-just` uses the SHA already pinned in `release.yml`; `setup-go` uses the SHA the siblings pin (`40f1582b…` v5). `ubuntu-latest` has `node` preinstalled, which the `go_version` justfile variable needs; the existing Node job's `setup-node` is not reused here on purpose (independent lanes).

Then extend the gate:

```yaml
  ci-gate:
    needs: [build-and-test, go-build-and-test]
    if: always()
    runs-on: ubuntu-latest
    steps:
      - name: Verify required jobs passed
        run: |
          if [ "${{ needs.build-and-test.result }}" != "success" ]; then
            echo "build-and-test did not succeed (result: ${{ needs.build-and-test.result }})"
            exit 1
          fi
          if [ "${{ needs.go-build-and-test.result }}" != "success" ]; then
            echo "go-build-and-test did not succeed (result: ${{ needs.go-build-and-test.result }})"
            exit 1
          fi
          echo "All required CI jobs passed."
```

`ci-gate` remains the single required status check; the branch ruleset (`scripts/ci-gate-ruleset.sh`) needs no change. The workflow's `on:` triggers, `permissions`, and header comment stay as they are (a one-line mention of the Go job in the header comment is fine).

### 7. Plan bookkeeping — `fab/plans/sahil/26-09-15-go-port.md`

Fill row P2's **PR** column with this change's folder name (`260915-h1iv-go-scaffold`, the convention P1 used) and set **Status** to a short "landed" note once shipped. No other plan edits.

### Not in scope (explicitly)

- No `src/go/internal/` packages, no cobra, no `help_dump.go`/`update.go`/`shell_init.go`/`skill.go` (B8), no `tudiff` (P3a/P4).
- No change to `scripts/build.sh`, `release.yml`, `Formula`, `dist/`, `package.json`, or any Node source/test — the Go binary is not shipped and the Node CI job is untouched.
- No README or `docs/site/` change (nothing user-visible changes; the frozen external surfaces in the plan's Goal are not touched).
- No memory rewrite beyond the `build/toolchain` additions listed below.

## Affected Memory

- `build/toolchain`: (modify) add the Go toolchain facts — module `github.com/sahil87/tu` at `src/go/go.mod` (`go 1.25`, zero deps), `src/go/cmd/tu/main.go` with `var version = "dev"` stamped via `-ldflags "-X main.version=v<package.json version>"`, `just go-build`/`go-test`/`go-lint` recipes (`bin/tu` output, gofmt+vet lint), the `go-build-and-test` CI job and the two-job `ci-gate` needs list; a Design Decision entry for "package.json is the version anchor for both binaries during the transition (harness byte-parity on `--version`); `bin/` not `dist/` for the Go build output; zero-dependency scaffold, cobra deferred to V2/B8".

## Impact

- **New files**: `src/go/go.mod`, `src/go/cmd/tu/main.go`, `src/go/cmd/tu/main_test.go`.
- **Modified files**: `justfile` (append three recipes + one variable), `.github/workflows/ci.yml` (one new job, `ci-gate` needs + check), `.gitignore` (`bin/`), `fab/plans/sahil/26-09-15-go-port.md` (row P2 columns), `docs/memory/build/toolchain.md` (hydrate).
- **Runtime/user impact**: none. Nothing ships; `tu` as installed is unchanged.
- **CI**: one additional `ubuntu-latest` job per PR/push (Go toolchain install + a sub-second build/test). `ci-gate` now fails if either lane fails.
- **Dependencies**: none added (no `go.sum`). `just` becomes a CI-time dependency for the Go lane (it already is for `release.yml`).
- **Unblocks**: plan rows P3a, P4, V1.
- **Local verification at apply**: `just go-lint && just go-build && just go-test`, then `bin/tu --version` prints `tu version v0.11.5` (current `package.json` version) and `bin/tu cc` exits 1 with the not-implemented message on stderr. Local toolchain is Go 1.26.2, which builds a `go 1.25` module without a `toolchain` directive.

## Open Questions

- None blocking. One watch item for apply: `gofmt` output can differ slightly across Go minor versions; CI pins 1.25 via `go-version-file`, the dev machine has 1.26.2. If `just go-lint` disagrees between the two, format with the CI version's rules (CI is the gate).

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Module path `github.com/sahil87/tu`, `go 1.25`, layout `src/go/go.mod` + `src/go/cmd/tu/main.go`; recipe names `go-build`/`go-test`/`go-lint`; CI job name `go-build-and-test` added to `ci-gate` needs; Node CI untouched | Stated verbatim in the row and in plan D5 | S:95 R:70 A:95 D:95 |
| 2 | Confident | Zero-dependency scaffold: no cobra, no `go.sum`; `main.go` hand-handles `--version`/`-V`/`-v` only | Whether the positional grammar uses cobra is the V2/B8 `command`-package call; adding a dep later is a one-line edit; a leaner P2 keeps CI cache config trivial | S:60 R:90 A:75 D:70 |
| 3 | Confident | Build-time version comes from `package.json` (`node -p`), stamped as `-X main.version=v<version>`, not `git describe` as the siblings do | The TS binary prints `v` + `package.json` version and P4 byte-diffs `--version`; `release.sh`/`release.yml` derive the tag from `package.json` — it is the single version anchor until Z1 | S:70 R:85 A:85 D:70 |
| 4 | Confident | Unstamped fallback is `var version = "dev"` → `tu version dev`; a leading `v` is added only when the value starts with a digit | Sibling convention (`idea`, `hop`); mirrors the TS `startsWith("v")` normalization without turning `dev` into `vdev` | S:55 R:95 A:85 D:80 |
| 5 | Confident | Any non-version invocation exits 1 with `tu: not implemented (Go port in progress)` on stderr | Plan row P4 states the Go side "prints unimplemented" at this stage; exact wording is a placeholder V2 replaces | S:60 R:95 A:80 D:75 |
| 6 | Confident | `go-lint` = `gofmt -l` (fail on output) + `go vet ./...`; no `golangci-lint` | Exactly the two checks `idea`/`hop`/`fab-kit` gate CI on; `golangci-lint` is not installed locally and would add a tool dependency for no P2 benefit | S:50 R:90 A:80 D:70 |
| 7 | Confident | CI job invokes `just go-lint`/`go-build`/`go-test` via pinned `setup-just`, rather than inlining `go` commands | One definition of "lint" (code-quality: minimum pathways); `setup-just` SHA already pinned in `release.yml`; siblings inline but have no justfile lint recipe to reuse | S:55 R:90 A:70 D:60 |
| 8 | Confident | `setup-go` uses `go-version-file: src/go/go.mod` with `cache: false` until a `go.sum` exists | `setup-go` v5's cache requires a dependency file and errors when the configured/default path is absent; a comment names the flip | S:50 R:95 A:75 D:80 |
| 9 | Confident | Go build output is `bin/tu` (add `bin/` to `.gitignore`), not `dist/` | `dist/` is the shipped Node artifact and is listed in `package.json` `files`; siblings build to `bin/<tool>` | S:40 R:90 A:85 D:75 |
| 10 | Confident | Test is `src/go/cmd/tu/main_test.go`, table-driven over `versionLine` and `run`, pinning the `version` standard's shape regex | Constitution § Go Transition mandates `_test.go` siblings; the `version` standard asks for a minimal pinning test; mirrors `cli-version.test.ts` | S:65 R:95 A:90 D:85 |
| 11 | Confident | Update plan row P2's PR/Status columns as part of this change | The plan doc says rows update the status column as they land; P1 followed that convention | S:60 R:100 A:90 D:85 |
| 12 | Certain | No `src/go/internal/` directory, no toolkit files, no README/site/formula/build.sh/release.yml edits | Row scope + Not-a-goal + § Go Transition "MUST NOT be shipped"; empty dirs are untrackable | S:85 R:95 A:95 D:90 |

12 assumptions (2 certain, 10 confident, 0 tentative, 0 unresolved).
