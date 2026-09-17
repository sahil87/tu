# Plan: Go Release Pipeline (Go port row R1)

**Change**: 260917-0118-go-release-pipeline
**Intake**: `intake.md`

## Requirements

### Build: ccusage pin

#### R1: `CCUSAGE_VERSION` is the single ccusage pin
A repo-root file `CCUSAGE_VERSION` MUST hold the bare ccusage version on one newline-terminated line (`20.0.19` today). The packaging script MUST read it and MUST fail loud (stderr, exit 1) when it differs from `package-lock.json`'s `packages["node_modules/ccusage"].version`, with the message `error: CCUSAGE_VERSION (<a>) != package-lock.json ccusage (<b>) — bump both together`. The guard MUST run before any network fetch.

- **GIVEN** `CCUSAGE_VERSION` says `20.0.19` and the lockfile locks `20.0.19`
- **WHEN** `scripts/package-go.sh` runs
- **THEN** the guard passes silently and packaging proceeds

- **GIVEN** the two versions differ
- **WHEN** `scripts/package-go.sh` runs
- **THEN** it prints the mismatch error to stderr and exits 1 before downloading anything

### Build: cross-compile recipes

#### R2: `just go-build-target os arch` and `just go-build-all`
`justfile` MUST gain `go-build-target os arch` writing `dist/bin/tu-<os>-<arch>` with `CGO_ENABLED=0 GOOS=<os> GOARCH=<arch>` and `-ldflags "-X main.version=v{{go_version}}"` for `./cmd/tu` only, and `go-build-all` running the four targets `darwin arm64`, `darwin amd64`, `linux arm64`, `linux amd64`. The skill drift guard (`cmp -s docs/site/skill.md src/go/internal/toolkit/skill.md || …`) MUST be lifted into one private recipe `_go-skill-guard` used by both `go-build` and `go-build-target`. `go-build`'s outputs (`bin/tu`, `bin/turepair`) and its error text MUST be unchanged. `turepair` MUST NOT be cross-compiled.

- **GIVEN** a clean checkout
- **WHEN** `just go-build-all` runs
- **THEN** `dist/bin/tu-darwin-arm64`, `dist/bin/tu-darwin-amd64`, `dist/bin/tu-linux-arm64`, `dist/bin/tu-linux-amd64` exist and `file` reports Mach-O / ELF for the respective targets

- **GIVEN** `src/go/internal/toolkit/skill.md` drifted from `docs/site/skill.md`
- **WHEN** `just go-build-target linux amd64` runs
- **THEN** it fails before `go build` with the existing drift error text

### Build: packaging

#### R3: `scripts/package-go.sh` produces the four `tu-go-*` archives
`just go-package` MUST run `scripts/package-go.sh`. For each platform the script MUST: (a) fail with `ERROR: Missing dist/bin/tu-<os>-<arch> — run 'just go-build-all' first.` (exit 1) when the binary is absent; (b) map `amd64→x64` (arm64 unchanged, os unchanged) and fetch `https://registry.npmjs.org/@ccusage/ccusage-<npm_os>-<npm_arch>/-/ccusage-<npm_os>-<npm_arch>-${CCUSAGE_VERSION}.tgz` with `curl -fsSL` into `dist/ccusage-tarballs/` (skipping an already-present file), where a non-2xx response is a hard error naming the URL; (c) extract exactly `package/bin/ccusage` from the tarball; (d) stage `tu` (0755), `vendor/ccusage/bin/ccusage` (0755), and a copy of the repo-root `tu.default.conf` in `dist/staging-tu-go-<os>-<arch>/`; (e) write `dist/tu-go-<os>-<arch>.tar.gz` via `COPYFILE_DISABLE=1 tar czf … -C <staging> tu vendor tu.default.conf` (no top-level directory), remove the staging dir, and print `  tu-go-<os>-<arch>.tar.gz (<bytes> bytes)`.

- **GIVEN** `just go-build-all` has run and the network is reachable
- **WHEN** `just go-package` runs
- **THEN** four archives exist under `dist/` and `tar tzf` of each lists exactly `tu`, `vendor/`, `vendor/ccusage/`, `vendor/ccusage/bin/`, `vendor/ccusage/bin/ccusage`, `tu.default.conf` (directory entries may be present; no other files)

- **GIVEN** `CCUSAGE_VERSION` names a version the registry does not have
- **WHEN** `just go-package` runs
- **THEN** curl fails and the script exits non-zero with the failing URL on stderr, and no partial archive for that platform is left in `dist/`

#### R4: Checksums and host smoke test
After the four archives, the script MUST write `dist/tu-go-SHA256SUMS` in `sha256sum` format (`<sha>  tu-go-<os>-<arch>.tar.gz`, two spaces, bare filenames, one line per archive) computed with `shasum -a 256`. It MUST then detect the host (`uname -s` → darwin/linux, `uname -m` → x86_64→amd64, arm64|aarch64→arm64), extract the host's archive into a temp dir, and assert that `./tu --version` prints exactly `tu version v<version>` (the package.json version) and that `./vendor/ccusage/bin/ccusage --version` exits 0; either failure exits 1 with a clear message. Non-host archives are checked structurally only (member listing).

- **GIVEN** the four archives were just built on linux/amd64
- **WHEN** the smoke test runs
- **THEN** `tu-go-linux-amd64.tar.gz` is extracted to a temp dir, `./tu --version` matches `tu version v0.11.5` (current package.json), the vendored ccusage runs, and the temp dir is removed

- **GIVEN** `dist/tu-go-SHA256SUMS` exists
- **WHEN** `cd dist && shasum -a 256 -c tu-go-SHA256SUMS` runs
- **THEN** all four lines report `OK`

### Build: formula generation

#### R5: `just go-formula [tag]` writes `dist/tu.rb`
`.github/formula-template.rb` MUST hold the Homebrew formula template from intake § 4 (class `Tu`, desc `AI coding assistant cost tracking CLI`, homepage `https://github.com/sahil87/tu`, license MIT, `on_macos`/`on_linux` × `on_arm`/`on_intel` blocks pointing at `https://github.com/sahil87/tu/releases/download/v#{version}/tu-go-<os>-<arch>.tar.gz` with `SHA_*` placeholders, `def install` = `libexec.install "tu", "vendor", "tu.default.conf"` + `bin.install_symlink libexec/"tu"`, `test do assert_match version.to_s, shell_output("#{bin}/tu --version") end`, no `depends_on`). `scripts/go-formula.sh [tag]` MUST default the tag to `git describe --tags --abbrev=0 HEAD`, strip `v`, fail with `ERROR: Missing dist/tu-go-<platform>.tar.gz — run 'just go-package' first.` when any archive is absent, substitute version and the four sha256s, write `dist/tu.rb`, and print `Generated dist/tu.rb (v<version>)`. `just go-formula tag=""` MUST wrap it. Nothing MUST push or copy the formula to the tap.

- **GIVEN** the four archives exist and the tag `v0.11.5` is passed
- **WHEN** `just go-formula v0.11.5` runs
- **THEN** `dist/tu.rb` contains `version "0.11.5"`, four URLs ending in `tu-go-<os>-<arch>.tar.gz`, four 64-hex sha256 values equal to the sums file, no `depends_on`, and `ruby -c dist/tu.rb` (when ruby is available) reports syntax OK

### CI and release workflow

#### R6: `release.yml` builds and uploads the Go assets before creating the release
The `release` job MUST add `actions/setup-go` (same pinned SHA as `ci.yml`, `go-version-file: src/go/go.mod`, `cache: false`) after setup-node, and after "Build bundle" three steps: `just go-build-all`, `just go-package`, and `just go-formula "${{ steps.version.outputs.tag }}"` followed by `cat dist/tu.rb`. "Create GitHub Release" MUST pass `dist/tu-go-darwin-arm64.tar.gz dist/tu-go-darwin-amd64.tar.gz dist/tu-go-linux-arm64.tar.gz dist/tu-go-linux-amd64.tar.gz dist/tu-go-SHA256SUMS` as assets. The tap step, `tag-on-release-merge`, and the Node steps MUST be unchanged; no step MUST carry `continue-on-error`. The header comment MUST note that from R1 the release also uploads `tu-go-*` assets (D8) while the formula ships the Node build until X1.

- **GIVEN** a `v*` tag push after this change merges
- **WHEN** the `release` job runs
- **THEN** Go build/package/formula steps run before `gh release create`, a Go failure aborts before the Release or tap is touched, and a green run's Release carries five `tu-go-*` assets

#### R7: `ci.yml` cross-compiles on every PR
The `go-build-and-test` job MUST gain a `Cross-compile` step running `just go-build-all` after the `just go-build` step. No network-dependent packaging step MUST be added to the PR gate; `go-diff` and `ci-gate` MUST be unchanged.

- **GIVEN** a PR touching `src/go/`
- **WHEN** CI runs
- **THEN** all four targets compile in the `go-build-and-test` lane

#### R8: `just go-dist` local aggregate
`justfile` MUST gain `go-dist` depending on `go-build-all`, then running `just go-package` and `just go-formula` (default tag), mirroring fab-kit's `dist` recipe.

- **GIVEN** a maintainer checkout
- **WHEN** `just go-dist` runs
- **THEN** it produces the four archives, the sums file, and `dist/tu.rb`, and exits 0 only when the smoke test passes

### Dogfood installer

#### R9: `just dogfood-install [tag]`
`scripts/dogfood-install.sh [tag]` (wrapped by `just dogfood-install tag=""`) MUST: resolve the tag (`$1`, else `gh release view --repo sahil87/tu --json tagName -q .tagName`) and print `Release: <tag>`; detect the host os/arch as in R4 (unsupported → error, exit 1); `gh release download <tag> --repo sahil87/tu --pattern tu-go-<os>-<arch>.tar.gz --pattern tu-go-SHA256SUMS --dir <tmp>` and, when the tarball is absent afterwards, fail with `error: release <tag> has no tu-go-<os>-<arch>.tar.gz asset — Go assets exist only for releases cut after plan row R1 landed` (exit 1); verify the tarball's `shasum -a 256` against the matching line of the sums file (mismatch → `error: sha256 mismatch for <asset> (expected <a>, got <b>)`, exit 1, nothing installed); refuse when `~/.local/bin/tu` exists and is not a symlink into `~/.local/lib/tu-go/` (`error: ~/.local/bin/tu exists and is not a dogfood symlink — remove it first`, exit 1); otherwise replace `~/.local/lib/tu-go/` with the extracted archive and `ln -sfn ~/.local/lib/tu-go/tu ~/.local/bin/tu`; then print `Installed: ~/.local/bin/tu -> ~/.local/lib/tu-go/tu (<output of tu --version>)`, a numbered `PATH order for tu:` list from `which -a tu` annotated `<- dogfood (Go)` / `<- brew (Node)` / `<- other`, and either `OK: the dogfood build shadows the brew tu. Run \`hash -r\` (or open a new shell) if \`tu\` still resolves to brew.` or `WARNING: <first> wins on PATH — the brew tu still runs. Prepend ~/.local/bin to PATH (e.g. in ~/.zshrc) and re-run.` (exit 0 either way), plus one note that `tu update` on this binary prints the not-installed-via-Homebrew message and that `just dogfood-install` refreshes it.

- **GIVEN** the latest release is `v0.11.5` (no Go assets)
- **WHEN** `just dogfood-install` runs
- **THEN** it prints `Release: v0.11.5` and the no-asset error, exits 1, and nothing under `~/.local` is touched

- **GIVEN** a release with `tu-go-*` assets and a host whose `~/.local/bin` precedes the brew bin on PATH
- **WHEN** `just dogfood-install <tag>` runs
- **THEN** the sha verifies, `~/.local/lib/tu-go/{tu,vendor/ccusage/bin/ccusage,tu.default.conf}` exist, `~/.local/bin/tu` is the symlink, and the report ends with the `OK:` line

#### R10: `just dogfood-uninstall`
`scripts/dogfood-uninstall.sh` MUST remove `~/.local/bin/tu` only when it is a symlink whose target lies under `~/.local/lib/tu-go/` (otherwise print `leaving ~/.local/bin/tu in place (not a dogfood symlink)`), remove `~/.local/lib/tu-go/`, and print `Removed dogfood build; tu now resolves to: <first which -a tu entry>` or `Removed dogfood build; no tu on PATH`. It MUST be idempotent (a second run exits 0 with the same final line).

- **GIVEN** a dogfood install exists
- **WHEN** `just dogfood-uninstall` runs twice
- **THEN** both runs exit 0, the symlink and lib dir are gone after the first, and `tu` resolves to the brew binary

### Resolution verification

#### R11: Vendor-first resolution through a symlinked executable is tested
`src/go/internal/source/ccusage/exec.go` MUST NOT change. A new test in `src/go/internal/source/ccusage/source_test.go` MUST build a temp tree `<dir>/real/vendor/ccusage/bin/ccusage` (a stub) and assert that the resolver's symlink-following vendor lookup finds it when the executable path is `<dir>/bin/tu → ../real/tu`. Because `ResolveBinary` reads `os.Executable()`, the test MUST exercise the same logic through a small extracted helper (e.g. `resolveVendor(exe string) (string, bool)`) that `ResolveBinary` calls — a pure refactor with identical behavior — rather than by faking `os.Executable`.

- **GIVEN** `<dir>/bin/tu` is a symlink to `<dir>/real/tu` and `<dir>/real/vendor/ccusage/bin/ccusage` exists
- **WHEN** the vendor lookup runs for `<dir>/bin/tu`
- **THEN** it returns `<dir>/real/vendor/ccusage/bin/ccusage`

- **GIVEN** no `vendor/` beside the resolved executable
- **WHEN** the vendor lookup runs
- **THEN** it reports not-found and `ResolveBinary` falls through to PATH (existing `TestResolveBinaryPathFallback` still passes)

### Plan bookkeeping

#### R12: Plan row R1 is updated
`fab/plans/sahil/26-09-15-go-port.md` row R1 MUST get this change's folder name in the PR column and a one-line landed status in the Status column, in the style of the Phase 0 rows.

- **GIVEN** the plan doc
- **WHEN** the change is applied
- **THEN** row R1's Status column no longer reads `not started`

### Non-Goals

- Pushing or copying `dist/tu.rb` to the tap; changing the tap's `Formula/tu.rb` — X1.
- A `tu-go` formula, a `TU_IMPL` switch, or changing `tu update`'s behavior off-Homebrew.
- Windows assets; adding `go-diff` to `ci-gate` (R3); cutting a release (R2).
- Uploading `dist/tu.rb` as a release asset.

### Design Decisions

#### Release artifacts live under `dist/`, the dev binary stays in `bin/`
**Decision**: Cross-compiled binaries go to `dist/bin/tu-<os>-<arch>`; archives, sums, and the generated formula to `dist/`; `bin/tu` remains the harness's dev binary.
**Why**: The plan names `dist/` for the formula; fab-kit uses `dist/bin/`; nothing collides with `dist/tu.mjs`/`dist/vendor/`; tu is not published to npm so the `files` sweep concern behind the earlier `bin/` decision does not apply to release artifacts.
**Rejected**: `bin/release/` — diverges from every sibling's layout for no gain; `dist/go/` — extra nesting the formula script would have to know about.
*Introduced by*: 260917-0118-go-release-pipeline

#### Formula installs into `libexec` and symlinks `bin/tu`
**Decision**: `libexec.install "tu", "vendor", "tu.default.conf"` + `bin.install_symlink libexec/"tu"`.
**Why**: `ccusage.ResolveBinary` resolves symlinks then looks for `vendor/ccusage/bin/ccusage` beside the real file; the `/Cellar/tu/` update gate passes on the libexec path; mirrors the Node formula's libexec layout.
**Rejected**: `bin.install "tu"` — would need `vendor/` inside Homebrew's `bin/`.
*Introduced by*: 260917-0118-go-release-pipeline

#### Dogfood binary lives in `~/.local/lib/tu-go/` behind a `~/.local/bin/tu` symlink
**Decision**: Extract the archive to `~/.local/lib/tu-go/` and symlink `~/.local/bin/tu` to its `tu`.
**Why**: The vendor tree must sit beside the real binary and `~/.local/bin` should hold only the entry point; the symlink is what the resolver follows.
**Rejected**: Copying `tu` and a `vendor/` directory straight into `~/.local/bin/` — pollutes the bin dir and is hard to uninstall cleanly.
*Introduced by*: 260917-0118-go-release-pipeline

#### ccusage tarballs come from the npm registry via curl, pinned by `CCUSAGE_VERSION`
**Decision**: `curl -fsSL` the `@ccusage/ccusage-<os>-<arch>` tarball URL and extract `package/bin/ccusage`; no node/npm in the path; no separate upstream integrity check.
**Why**: curl survives Z1 (node goes away); `tu-go-SHA256SUMS` over our own archives is what installs verify; the registry's integrity field comes from the same source.
**Rejected**: `npm pack` — needs node; `npm install` of the host package — vendors only the host's platform.
*Introduced by*: 260917-0118-go-release-pipeline

## Tasks

### Phase 1: Setup

- [x] T001 Create repo-root `CCUSAGE_VERSION` containing `20.0.19` (one line, newline-terminated) <!-- R1 -->
- [x] T002 In `justfile`, extract the skill drift guard into a private `_go-skill-guard` recipe, make `go-build` call it (outputs/error text unchanged), and add `go-build-target os arch` (→ `dist/bin/tu-<os>-<arch>`, `CGO_ENABLED=0`, ldflags from `go_version`, `./cmd/tu` only) and `go-build-all` (the four targets) <!-- R2 -->

### Phase 2: Core Implementation

- [x] T003 Write `scripts/package-go.sh` (executable): lockfile guard for `CCUSAGE_VERSION` → per-platform binary check, curl fetch into `dist/ccusage-tarballs/` (skip if present, `-f` fail on 404), extract `package/bin/ccusage`, stage `tu` + `vendor/ccusage/bin/ccusage` + `tu.default.conf`, `COPYFILE_DISABLE=1 tar czf dist/tu-go-<os>-<arch>.tar.gz` (flat), size line; then `dist/tu-go-SHA256SUMS` via `shasum -a 256`; then the host smoke test (`./tu --version` byte-exact vs `tu version v<package.json version>`, vendored `ccusage --version` exit 0) and structural member check of the other archives. Add `go-package` recipe to `justfile` <!-- R3 R4 -->
- [x] T004 [P] Add `.github/formula-template.rb` (intake § 4 verbatim) and `scripts/go-formula.sh [tag]` (executable; fab-kit's `brew-formula.sh` adapted: default tag from `git describe`, archive presence check, four `shasum -a 256` values, `sed` into `dist/tu.rb`, `Generated dist/tu.rb (v<version>)`). Add `go-formula tag=""` and `go-dist` recipes to `justfile` <!-- R5 R8 -->
- [x] T005 [P] Write `scripts/dogfood-install.sh` (executable) per R9: tag resolution via `gh release view`, host detection, `gh release download` of the two assets into `mktemp -d`, no-asset error, sha verification, non-dogfood-symlink refusal, install to `~/.local/lib/tu-go/` + `ln -sfn`, report with `which -a tu` ordering, OK/WARNING verdict, `tu update` note. Add `dogfood-install tag=""` recipe <!-- R9 -->
- [x] T006 [P] Write `scripts/dogfood-uninstall.sh` (executable) per R10, idempotent. Add `dogfood-uninstall` recipe <!-- R10 -->
- [x] T007 [P] In `src/go/internal/source/ccusage/exec.go`, extract the symlink-resolve + `vendor/ccusage/bin/ccusage` stat into a small unexported helper `ResolveBinary` calls (behavior identical); add a test in `source_test.go` building `<dir>/real/vendor/ccusage/bin/ccusage` and `<dir>/bin/tu → ../real/tu` that asserts the helper resolves through the symlink, plus a negative case. Run `just go-test` and `just go-lint` <!-- R11 -->

### Phase 3: Integration & Edge Cases

- [x] T008 Edit `.github/workflows/release.yml`: add the pinned `actions/setup-go` step (`go-version-file: src/go/go.mod`, `cache: false`) after setup-node; add `Cross-compile Go binaries` (`just go-build-all`), `Package tu-go assets` (`just go-package`), and `Generate Go formula` (`just go-formula "${{ steps.version.outputs.tag }}"` + `cat dist/tu.rb`) after `Build bundle`; append the five `dist/tu-go-*` asset paths to `gh release create`; add the header-comment line about R1/D8. Leave the tap step untouched <!-- R6 -->
- [x] T009 [P] Edit `.github/workflows/ci.yml`: add a `Cross-compile` step running `just go-build-all` after `just go-build` in `go-build-and-test`; nothing else changes <!-- R7 -->
- [x] T010 Run `just go-dist` end to end on this host and verify: four archives present, `tar tzf` member lists correct, `cd dist && shasum -a 256 -c tu-go-SHA256SUMS` all OK, `dist/tu.rb` has `version "0.11.5"` and four sha256s matching the sums file and no `depends_on`, smoke test passed. Then run `just dogfood-install` (expect `Release: v0.11.5` + the no-asset error, exit 1) and `just dogfood-uninstall` twice (exit 0 both times). Finally `just go-lint`, `just go-test`, `just go-build`, and `just go-diff` (still 452/452 green — the harness is untouched by this change). Fix anything red <!-- R3 R4 R5 R8 R9 R10 -->

### Phase 4: Polish

- [x] T011 Update `fab/plans/sahil/26-09-15-go-port.md` row R1: PR column `260917-0118-go-release-pipeline`, Status column one line (landed: recipes, `package-go.sh`, `dist/tu.rb` generation, release.yml assets, dogfood recipes; assets appear at the next release) <!-- R12 -->

## Acceptance

### Functional Completeness

- [x] A-001 R1: `CCUSAGE_VERSION` exists with `20.0.19` and `scripts/package-go.sh` exits 1 with the documented message when it disagrees with `package-lock.json`
- [x] A-002 R2: `just go-build-all` writes the four `dist/bin/tu-<os>-<arch>` binaries; `_go-skill-guard` is shared by `go-build` and `go-build-target`; `turepair` is not cross-compiled
- [x] A-003 R3: `just go-package` produces `dist/tu-go-{darwin,linux}-{arm64,amd64}.tar.gz` with exactly `tu`, `vendor/ccusage/bin/ccusage`, `tu.default.conf` as file members and no top-level directory
- [x] A-004 R4: `dist/tu-go-SHA256SUMS` verifies with `shasum -a 256 -c`; the host smoke test runs `./tu --version` and the vendored ccusage from an extracted archive
- [x] A-005 R5: `just go-formula v0.11.5` writes `dist/tu.rb` from `.github/formula-template.rb` with version, four URLs, four matching sha256s, libexec install + `bin.install_symlink`, and no `depends_on`
- [x] A-006 R6: `release.yml` has setup-go and the three Go steps before `gh release create`, which lists the five `tu-go-*` assets; the tap step is byte-identical to before
- [x] A-007 R7: `ci.yml`'s `go-build-and-test` runs `just go-build-all`; `go-diff` and `ci-gate` are unchanged
- [x] A-008 R8: `just go-dist` runs build-all → package → formula
- [x] A-009 R9: `scripts/dogfood-install.sh` implements tag resolution, host detection, download, sha verification, symlink refusal, install, and the PATH report with the exact messages in R9
- [x] A-010 R10: `scripts/dogfood-uninstall.sh` removes only a dogfood symlink and the lib dir, idempotently
- [x] A-011 R11: `exec.go`'s `ResolveBinary` behavior is unchanged; the new symlink-chain test and negative case pass
- [x] A-012 R12: Plan row R1 shows the change and a landed status

### Behavioral Correctness

- [x] A-013 R2: `just go-build` still writes `bin/tu` and `bin/turepair` with the same version stamp and drift-guard error text
- [x] A-014 R6: No Go step in `release.yml` carries `continue-on-error`; all Go steps precede "Create GitHub Release"

### Scenario Coverage

- [x] A-015 R9: `just dogfood-install` against the current latest release (`v0.11.5`, no assets) prints `Release: v0.11.5` then the no-asset error and exits 1 without touching `~/.local`
- [x] A-016 R10: Two consecutive `just dogfood-uninstall` runs both exit 0

### Edge Cases & Error Handling

- [x] A-017 R3: A missing `dist/bin/tu-<os>-<arch>` stops `package-go.sh` with the documented `ERROR: Missing …` message before any download
- [x] A-018 R3: A 404 from the registry (wrong version) fails the script with the URL on stderr and leaves no partial archive for that platform
- [x] A-019 R9: An existing non-symlink `~/.local/bin/tu` is refused, not overwritten

### Code Quality

- [x] A-020 Pattern consistency: new scripts follow `scripts/build.sh`/`release.sh` style (`#!/usr/bin/env bash`, `set -euo pipefail`, `error:`/`ERROR:` prefixes on stderr, fail loud) and the justfile recipes follow the existing `go-*` comment style
- [x] A-021 No unnecessary duplication: one `_go-skill-guard`; host-detection logic is not copy-pasted between `package-go.sh` and `dogfood-install.sh` beyond a small shared shape (a shared `scripts/lib/host.sh` sourced by both is acceptable; duplication of two `case` statements is also acceptable — flag only if the mappings disagree)
- [x] A-022 No magic strings: platform list, asset name prefix `tu-go-`, and `~/.local/lib/tu-go` appear as named variables at the top of each script
- [x] A-023 Errors are never swallowed: every curl/tar/gh failure surfaces on stderr and exits non-zero
- [x] A-024 `just go-lint`, `just go-test`, and `just go-diff` are green after the change

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Deletion Candidates

None — this change adds new functionality without making existing code redundant.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | The `ResolveBinary` symlink test is written against a small extracted helper rather than by faking `os.Executable` | `os.Executable` cannot be redirected in-process; extracting the vendor lookup is a behavior-preserving refactor the intake left open ("extend or add a test") | S:60 R:90 A:85 D:70 |
| 2 | Confident | `dist/ccusage-tarballs/` caches downloaded npm tarballs between local runs | Avoids re-downloading ~4 × 10 MB on every `just go-dist`; `dist/` is gitignored and cleaned by the CI runner anyway | S:55 R:95 A:80 D:70 |
| 3 | Confident | `just go-dist` runs `go-formula` with the default tag (`git describe`), which resolves to the last tag on this branch | Local runs have no explicit tag; `release.yml` passes the resolved tag explicitly | S:60 R:95 A:80 D:65 |
| 4 | Certain | The tar member check accepts directory entries (`vendor/`, `vendor/ccusage/`, …) and requires exactly the three files | GNU/BSD tar list directories by default; the contract is about files shipped | S:80 R:95 A:90 D:85 |

4 assumptions (1 certain, 3 confident, 0 tentative).
