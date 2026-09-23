---
type: memory
description: Build & test toolchain for the Go module github.com/sahil87/tu — the justfile's go-*/harness-* recipes, the gofmt+vet lint gate, the golden -update test convention, ci.yml's build-and-test / go-build-and-test / tudiff lanes aggregated by the ci-gate job plus the ci-gate-ruleset helper, and the tag-anchored release.sh/release.yml pipeline; packaging and tap detail in go-release-pipeline.md; the retired tree's toolchain is summarized in the Retired section below
---

# Build & Test Toolchain

**Domain**: build

## Overview

The shipped `tu` binary builds from the Go module under `src/go/` (`module github.com/sahil87/tu`) through the `justfile`'s `go-*` / `harness-*` recipes, gated by `ci.yml`'s three lanes and the aggregating `ci-gate` job, and released by the tag-anchored `release.yml` pipeline. Packaging, the Homebrew formula, and the tap update live in [go-release-pipeline](/build/go-release-pipeline.md); the differential-harness binaries the `harness-*` recipes build are documented in [differential-harness](/harness/differential-harness.md).

## Requirements

### Requirement: The Go module
`src/go/go.mod` declares `module github.com/sahil87/tu` with the `go 1.26.0` directive and exactly two direct dependencies — `golang.org/x/term v0.46.0` (the TTY width probe) and `golang.org/x/sys v0.48.0` (`x/sys/unix` for the watch terminal) — with a committed `src/go/go.sum`. Entry points live under `src/go/cmd/<name>/` (`cmd/tu`, `cmd/turepair`, `cmd/tudiff`, `cmd/fakeccusage`, `cmd/fakegit`) and shared code under `src/go/internal/<pkg>/`. CI pins the toolchain from this file via setup-go's `go-version-file: src/go/go.mod`, so the pinned toolchain's gofmt rules are the ones the lint gate enforces.

### Requirement: `just go-build` — the dev binary
`just go-build` MUST run the private `_go-skill-guard` recipe first — `cmp -s docs/site/skill.md src/go/internal/toolkit/skill.md`; on drift it prints `error: src/go/internal/toolkit/skill.md drifted from docs/site/skill.md — run scripts/sync-skill.sh` to stderr and exits 1 **before** `go build` runs (`scripts/sync-skill.sh` is the refresh) — then build `bin/tu` from `./cmd/tu` stamped with `-ldflags "-X main.version=v{{go_version}}"` and `bin/turepair` from `./cmd/turepair`. `go_version` is read from `package.json`'s version via `node -p`. `bin/` is gitignored; release artifacts stage under `dist/` (see [go-release-pipeline](/build/go-release-pipeline.md)).

#### Scenario: Drifted skill bundle blocks the build
- **GIVEN** `src/go/internal/toolkit/skill.md` differs from `docs/site/skill.md`
- **WHEN** `just go-build` runs
- **THEN** the guard's drift error is on stderr, the exit code is 1, and no binary is built

### Requirement: `just go-lint` — the gofmt + vet gate
`just go-lint` MUST fail when `gofmt -l .` (run in `src/go/`) lists any file, printing `The following files are not gofmt-clean:`, the file list, and `Run: (cd src/go && gofmt -w .)` to stderr before exiting 1; otherwise it MUST run `go vet ./...`. There is no golangci-lint; the `go-build-and-test` CI lane invokes this same recipe, so "lint" has exactly one definition.

#### Scenario: A gofmt-dirty file fails before vet
- **GIVEN** one file under `src/go/` is not gofmt-clean
- **WHEN** `just go-lint` runs
- **THEN** the file is named on stderr with the `Run: (cd src/go && gofmt -w .)` hint, the exit code is 1, and `go vet` does not run

### Requirement: `just go-test` and the golden `-update` convention
`just go-test` MUST run `cd src/go && go test ./... -count=1`. Golden tests keep expected output bytes in `testdata/*.golden` beside each package and MUST regenerate them through a per-package `-update` flag (`var update = flag.Bool("update", false, "regenerate golden files")` — e.g. `src/go/internal/render/csv/csv_test.go`, `src/go/internal/render/ansi/table_test.go`, `src/go/internal/render/json/snapshot_test.go`, `src/go/internal/render/markdown/markdown_test.go`, `src/go/internal/watch/rain_test.go`), so the committed goldens pin the output bytes the differential harness compares.

### Requirement: Cross-compile and release-packaging recipes
`just go-build-target <os> <arch>` cross-compiles `./cmd/tu` only (`turepair` is a maintainer tool, not shipped) with `CGO_ENABLED=0` and the stamped version into `dist/bin/tu-<os>-<arch>`, after the skill guard. `just go-build-all` builds the four release targets (darwin/linux × arm64/amd64 — Homebrew's matrix). `just go-package` (`scripts/package-go.sh`) packs the four `tu-go-<os>-<arch>.tar.gz` archives plus `tu-go-SHA256SUMS` and a host smoke test; `just go-formula [tag]` (`scripts/go-formula.sh`) renders the Homebrew formula into `dist/tu.rb`; `just go-dist` is the local aggregate (`go-build-all` → `go-package` → `go-formula`) — everything the release job runs minus the uploads. `just dogfood-install [tag]` / `just dogfood-uninstall` manage the maintainer build under `~/.local`. Contracts for all of these live in [go-release-pipeline](/build/go-release-pipeline.md) and [dogfood-install](/build/dogfood-install.md).

### Requirement: Harness recipes
`just harness-build` builds the three harness binaries into `bin/harness/` (gitignored via `bin/`): `tudiff` plus the fakes `ccusage` and `git`, named for the tools they impersonate so `tudiff run` can prepend `bin/harness` to `PATH`. `just harness-capture *ARGS` records real ccusage output into `harness/fixtures/<alias>/`. `just go-diff *ARGS` (depends on `build go-build harness-build`) runs `bin/harness/tudiff run`; `just go-live *ARGS` runs `bin/harness/tudiff live`. Both gate identically: exit 1 on an unexpected red case, a timeout, an unconfirmed-fixture replay, or a stale entry in `harness/expected-diffs.json`. Corpus, matrix, and gate detail live in [fixture-corpus](/harness/fixture-corpus.md), [matrix-and-staging](/harness/matrix-and-staging.md), and [differential-harness](/harness/differential-harness.md).

### Requirement: CI lanes and the `ci-gate` aggregator
`.github/workflows/ci.yml` runs on pull requests targeting `main` and pushes to `main`, in three lanes. `build-and-test` builds and tests the retired tree's toolchain on Node 20 (see the Retired section). `go-build-and-test` (setup-go pinned SHA, `go-version-file: src/go/go.mod`, `cache: false`; setup-just pinned SHA) runs `just go-lint`, `just go-build`, a `Cross-compile` step (`just go-build-all`, no network), and `just go-test`. `tudiff` (same pinned SHAs plus setup-node 20) installs the retired tree's dependencies, then runs `just go-diff --placeholder` and `just go-live`, uploading `bin/harness/report/` and `bin/harness/report-live/` as the `tudiff-report` artifact with `if: always()`. The aggregating `ci-gate` job (`needs: [build-and-test, go-build-and-test, tudiff]`, `if: always()`) MUST succeed only when all three lanes succeeded, printing the failing lane's name and exiting 1 otherwise. ci.yml publishes nothing — the external side effects live in `release.yml`.

#### Scenario: A red lane fails the gate with a named lane
- **GIVEN** the `tudiff` lane exits 1 on an unexpected red case
- **WHEN** `ci-gate` runs
- **THEN** it prints `tudiff did not succeed (result: failure)`, exits 1, and the required check stays red even though `ci-gate` itself ran (`if: always()` makes the result a definitive pass/fail, never a hung skipped check)

### Requirement: The `ci-gate` branch ruleset helper
`scripts/ci-gate-ruleset.sh` is the idempotent, dry-run-default helper that creates or updates the `Require CI gate` branch ruleset on `main`, requiring the `ci-gate` status-check context on `refs/heads/main` (`strict` policy off). With no arguments it MUST print the payload and plan and change nothing; `--apply` performs the create/update via `gh api` (PUT in place when the ruleset already exists) and requires an admin-scoped `gh`. When `gh` is missing, unauthenticated, or lacks admin scope, the script MUST print the manual steps and exit 0. Applying it is a documented admin action, never performed by the pipeline.

#### Scenario: Dry-run changes nothing
- **GIVEN** the ruleset does not exist
- **WHEN** `scripts/ci-gate-ruleset.sh` runs without `--apply`
- **THEN** it prints the ruleset payload and plan, makes no API mutation, and exits 0

### Requirement: The tag-anchored release
`scripts/release.sh <patch|minor|major>` (wrapped by `just release`) MUST refuse a dirty working tree (`ERROR: Working tree not clean — commit or stash first`, exit 1) and a detached HEAD, then bump the version in `package.json` (the package manager's `version` command, `--no-git-tag-version`), commit `package.json` and `package-lock.json` as `release: <v*>`, tag `v<version>`, and push the branch and the tag. `release.yml` runs the `release` job on a `v*` tag push, on `workflow_dispatch` (which invokes `scripts/release.sh` for the bump), and on a `release`-labeled merge to `main` — the `tag-on-release-merge` job detects the labeled merge, creates and pushes the `v*` tag idempotently, and exposes outputs so the `release` job runs in the same workflow run via `needs`/`always()`. The `release` job applies the fail-loud-first ordering: every build step (retired-tree bundle, `just go-build-all`, `just go-package`, `just go-formula <tag>`) runs before "Create GitHub Release", and the Homebrew tap update is the last step. Asset and tap contracts live in [go-release-pipeline](/build/go-release-pipeline.md).

#### Scenario: A dirty tree refuses to release
- **GIVEN** uncommitted changes in the working tree
- **WHEN** `just release patch` runs
- **THEN** stderr is `ERROR: Working tree not clean — commit or stash first` and the exit code is 1 — nothing is bumped, committed, tagged, or pushed

## Design Decisions

### `package.json` is the version anchor
**Decision**: `just go-build` stamps the binary from `package.json`'s version (`go_version := \`node -p 'require("./package.json").version'\``, `-X main.version=v<version>`), not from `git describe`.
**Why**: the differential harness byte-diffs `--version` across both binaries and the frozen oracle prints `v` + the `package.json` version; `scripts/release.sh` and `release.yml` also derive the `v*` tag from `package.json`, so it is the single anchor until plan row Z1 removes it and switches the stamp to `git describe`.
**Rejected**: `git describe --tags --always` as the sibling Go tools use — yields `v0.11.5-3-gabc123` on non-tag commits and would break harness parity.
*Introduced by*: 260915-h1iv-go-scaffold

### Dev binaries in `bin/`, release artifacts in `dist/`
**Decision**: `just go-build` writes the gitignored dev binaries `bin/tu` and `bin/turepair`; release artifacts — the cross-compiled `dist/bin/tu-<os>-<arch>` binaries, the `tu-go-*` archives and sums, and the generated `dist/tu.rb` — live under `dist/`.
**Why**: `dist/` is the packaged artifact tree listed in `package.json` `files`, so a dev binary there would be swept into the published package; release artifacts are exempt from that concern (tu is not published to a package registry) and `dist/` is where the sibling tools stage theirs.
**Rejected**: `dist/tu-go` for the dev binary — same packaging hazard; `bin/release/` for release artifacts — diverges from every sibling's layout for no gain.
*Introduced by*: 260915-h1iv-go-scaffold (extended by 260917-0118-go-release-pipeline)

### Minimal-dependency module; CI invokes the `just` recipes
**Decision**: the module's only direct dependencies are `golang.org/x/term` and `golang.org/x/sys` — no cobra — and the `go-build-and-test` CI lane invokes `just go-lint` / `go-build` / `go-build-all` / `go-test` rather than inlining `go` commands, with setup-go's `cache: false`.
**Why**: gofmt + `go vet` is the single lint definition and the justfile is the single build definition, shared by local runs and CI; `x/term` is the toolkit's standard terminal package; cobra would be a dependency with no current use.
**Rejected**: pulling cobra — a dependency with no current consumer; inlining gofmt/vet in the workflow YAML as the sibling repos do — two definitions of lint that can drift.
*Introduced by*: 260915-h1iv-go-scaffold

### Cross-compile in the PR gate, package at release time
**Decision**: `go-build-and-test` runs `just go-build-all` on every PR (no network), while the network-dependent `just go-package` registry fetch runs only at release time and via `just go-dist`.
**Why**: GOOS-specific compile breaks surface on every PR while a registry hiccup can never block an unrelated PR.
**Rejected**: packaging in the PR gate — a registry outage would block unrelated merges; no cross-compile lane — GOOS-specific breaks would surface only at release time.
*Introduced by*: 260917-0118-go-release-pipeline

### Tag-anchored releases; the release-merge path runs in the same workflow run
**Decision**: releases anchor on a `v*` tag; the `release`-labeled-merge path creates the tag in the `tag-on-release-merge` job and runs the `release` job in the same workflow run via `needs`/`outputs`, rather than relying on the tag push to re-trigger the workflow.
**Why**: GitHub does not re-trigger workflows from pushes authenticated with the default `GITHUB_TOKEN` (documented loop-prevention), so the tag push authored by the workflow would never fire a second run; job outputs make the release actually execute while releases remain tag-anchored.
**Rejected**: relying on the pushed tag re-triggering `release.yml` — suppressed by GitHub, so release-merge releases would silently never run.
*Introduced by*: 260602-v76l-release-workflow

## Retired src/node tree

The `src/node/` tree is the frozen differential-harness oracle (`node dist/tu.mjs`) and the D10 rollback build; it stays until plan row Z1 removes it. Its toolchain, in brief: `npm ci && npm run build && npm test` — esbuild bundles the CLI entry into `dist/tu.mjs`, and the Node.js built-in test runner (via `tsx`) runs the suite; the `justfile`'s `setup` / `test` / `run` / `build` recipes wrap this toolchain, and `scripts/release.sh` bumps `package.json` with the package manager's `version` command. Plan row Z1 removes the tree, its `build-and-test` CI lane, and the `go_version` `package.json` stamp (switched to `git describe`).
