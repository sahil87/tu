---
type: memory
description: "Go release pipeline — four cross-compiled tu-go-<os>-<arch>.tar.gz assets (flat: tu + vendored platform ccusage + tu.default.conf) plus tu-go-SHA256SUMS, the CCUSAGE_VERSION pin with a lockfile guard, npm-registry curl fetch, the generated dist/tu.rb formula (libexec + bin symlink, no depends_on) that release.yml copies over the tap's Formula/tu.rb as its last step, the rollback path (a higher-numbered Node release; brew never downgrades), and the per-PR cross-compile lane"
---
# Go Release Pipeline

**Domain**: build

## Overview

The release pipeline for the Go binary the Homebrew formula ships: `just go-build-all` cross-compiles four targets, `just go-package` packs each binary with the target platform's vendored ccusage and `tu.default.conf` into a flat `tu-go-<os>-<arch>.tar.gz`, `just go-formula` renders the Go Homebrew formula into `dist/tu.rb`, and `release.yml` uploads the five assets to the GitHub Release and then copies `dist/tu.rb` over the tap's `Formula/tu.rb` as its last step (plan row X1). Maintainers can install a specific build with `just dogfood-install [tag]`. The recipes sit beside the dev recipes documented in [toolchain](/build/toolchain.md); the vendor-first resolution the layouts satisfy is in [ccusage-adapter](/source/ccusage-adapter.md).

## Requirements

### Requirement: The `tu-go-*` asset contract
Each release carries four archives `tu-go-<os>-<arch>.tar.gz` for `darwin arm64`, `darwin amd64`, `linux arm64`, `linux amd64` (Go's GOOS/GOARCH spellings) plus `tu-go-SHA256SUMS`. Every archive is **flat** — no top-level directory — and contains exactly three file members: `tu` (0755), `vendor/ccusage/bin/ccusage` (0755), and `tu.default.conf` (a copy of the repo-root file, for layout parity with the Node formula's `libexec/`; the Go binary reads its `go:embed`ded copy). The sums file is in `sha256sum` format — `<sha>  tu-go-<os>-<arch>.tar.gz`, two spaces, bare filenames, one line per archive — produced with `shasum -a 256` (present on macOS and Linux). This layout is what `ccusage.ResolveBinary` expects: the vendor tree sits beside the real binary (0118).

#### Scenario: Archive member check
- **GIVEN** the four archives were built
- **WHEN** `tar tzf` lists each archive's non-directory members
- **THEN** they are exactly `tu`, `tu.default.conf`, `vendor/ccusage/bin/ccusage`; any deviation fails `scripts/package-go.sh` with the member-contract error on stderr

### Requirement: `CCUSAGE_VERSION` pin and lockfile guard
The repo-root file `CCUSAGE_VERSION` holds the bare ccusage version on one newline-terminated line (`20.0.19`). `scripts/package-go.sh` reads it **first, before any network fetch**, and fails loud (stderr, exit 1) when it differs from `package-lock.json`'s `packages["node_modules/ccusage"].version`, printing exactly `error: CCUSAGE_VERSION (<a>) != package-lock.json ccusage (<b>) — bump both together`. Bumping ccusage during the transition means `npm install ccusage@X` **and** editing `CCUSAGE_VERSION`. After the lockfile is removed (plan row Z1), the file is the only pin and the guard is deleted (0118).

#### Scenario: Mismatch fails before downloading
- **GIVEN** `CCUSAGE_VERSION` says `20.0.19` and the lockfile locks `20.0.20`
- **WHEN** `just go-package` runs
- **THEN** it prints the mismatch error and exits 1 before any download

### Requirement: ccusage fetch from the npm registry
For each platform the script maps Go arch to npm arch — `amd64`→`x64`, `arm64` unchanged, os unchanged — and fetches `https://registry.npmjs.org/@ccusage/ccusage-<npm_os>-<npm_arch>/-/ccusage-<npm_os>-<npm_arch>-<CCUSAGE_VERSION>.tgz` with `curl -fsSL` into `dist/ccusage-tarballs/`, skipping a tarball that is already cached there. A non-2xx response removes the partial file and fails the script with `error: failed to fetch <url>` on stderr. Exactly `package/bin/ccusage` is extracted from the tarball into the staging tree. No node/npm is involved in the fetch — `curl` is what survives Z1 (0118).

#### Scenario: A version the registry does not have
- **GIVEN** `CCUSAGE_VERSION` names an unpublished version
- **WHEN** `just go-package` runs
- **THEN** curl fails, the failing URL is on stderr, the script exits non-zero, and no partial archive for that platform is left in `dist/`

### Requirement: Cross-compile and packaging recipes
`just go-build-target <os> <arch>` writes `dist/bin/tu-<os>-<arch>` with `CGO_ENABLED=0 GOOS=<os> GOARCH=<arch>` and `-ldflags "-X main.version=v{{go_version}}"` for `./cmd/tu` only — `turepair` is a maintainer tool, not shipped. The skill drift guard lives in the private `_go-skill-guard` recipe shared by `go-build` and `go-build-target`. `just go-build-all` runs the four targets. `just go-package` runs `scripts/package-go.sh`, which requires the four `dist/bin/tu-<os>-<arch>` binaries (`ERROR: Missing dist/bin/tu-<os>-<arch> — run 'just go-build-all' first.`, exit 1), stages `dist/staging-tu-go-<os>-<arch>/`, writes the archive via `COPYFILE_DISABLE=1 tar czf … -C <staging> tu vendor tu.default.conf`, prints `  tu-go-<os>-<arch>.tar.gz (<bytes> bytes)`, then writes the sums file and runs the **host smoke test**: the archive matching `uname -s`/`uname -m` is extracted to a temp dir and must print exactly `tu version v<package.json version>` from `./tu --version` and exit 0 from `./vendor/ccusage/bin/ccusage --version` — the one pre-release execution of the tarball layout against vendor-first resolution; the other three archives are checked structurally only. `just go-dist` is the local aggregate (`go-build-all` → `go-package` → `go-formula`) — everything the release job runs minus the uploads (0118).

#### Scenario: Host smoke test
- **GIVEN** the four archives built on linux/amd64
- **WHEN** the smoke test runs
- **THEN** `tu-go-linux-amd64.tar.gz` is extracted, `./tu --version` matches `tu version v<version>` byte-for-byte, the vendored ccusage runs, and the temp dir is removed

### Requirement: The generated formula
`.github/formula-template.rb` is a Homebrew formula template (class `Tu`, desc `AI coding assistant cost tracking CLI`, homepage `https://github.com/sahil87/tu`, license MIT, `VERSION_PLACEHOLDER` and four `SHA_*` placeholders) with `on_macos`/`on_linux` × `on_arm`/`on_intel` blocks whose URLs are `https://github.com/sahil87/tu/releases/download/v#{version}/tu-go-<os>-<arch>.tar.gz`. `scripts/go-formula.sh [tag]` (wrapped by `just go-formula tag=""`) defaults the tag to `git describe --tags --abbrev=0 HEAD` (erroring `ERROR: No tag found. Pass a tag or ensure HEAD is tagged.` when there is none), requires the four archives (`ERROR: Missing dist/tu-go-<platform>.tar.gz — run 'just go-package' first.`), substitutes the version (tag without `v`) and the four sha256s, writes `dist/tu.rb`, and prints `Generated dist/tu.rb (v<version>)`. The formula installs with `libexec.install "tu", "vendor", "tu.default.conf"` + `bin.install_symlink libexec/"tu"`, carries **no `depends_on`** (the point of the port), and its `test do` block asserts `tu --version` matches the version. The generated formula is written to `dist/`, `cat`'d into the release workflow log, and copied over the tap's `Formula/tu.rb` by the release job's last step (0118, jmh4).

#### Scenario: Formula renders from the template
- **GIVEN** the four archives exist and tag `v0.11.5` is passed
- **WHEN** `just go-formula v0.11.5` runs
- **THEN** `dist/tu.rb` contains `version "0.11.5"`, four asset URLs, four 64-hex sha256s equal to the sums file, and no `depends_on`

### Requirement: `release.yml` builds the Go assets, publishes the Release, then pushes the formula
The `release` job runs `actions/setup-go` (same pinned SHA as `ci.yml`, `go-version-file: src/go/go.mod`, `cache: false`) after setup-node, and after "Build bundle" three steps — `just go-build-all`, `just go-package`, and `just go-formula "<tag>"` + `cat dist/tu.rb` — all **before** "Create GitHub Release". No step carries `continue-on-error`: a Go failure aborts the run before the Release or the tap is touched (the job's fail-loud-first ordering). `gh release create` passes the five assets (`dist/tu-go-darwin-arm64.tar.gz`, `dist/tu-go-darwin-amd64.tar.gz`, `dist/tu-go-linux-arm64.tar.gz`, `dist/tu-go-linux-amd64.tar.gz`, `dist/tu-go-SHA256SUMS`). The job's **last** step, "Update Homebrew tap", clones `sahil87/homebrew-tap` with `HOMEBREW_TAP_TOKEN`, runs `cp dist/tu.rb /tmp/tap/Formula/tu.rb`, commits as `github-actions[bot]` with message `tu <version>`, and pushes — fab-kit's tap step verbatim. It runs after the Release on purpose: the formula's tarball URLs resolve only once the assets exist. The step is shared by all three entry paths (tag push, `workflow_dispatch`, release-labeled merge via `tag-on-release-merge`) and carries no bump-type or version-shape guard — every release pushes the Go formula. `npm ci` and `npm run build` remain in the job: `scripts/package-go.sh` (lockfile guard, host smoke test) and the justfile's `go_version` read `package.json`/`package-lock.json` via `node -p`, and the bundle build is the proof that the Node rollback artifact (D10) still compiles until Z1. `tu-go-*` assets exist only for releases from v0.11.6 on; the Go formula reaches the tap from the cutover release, 0.13.0 (0118, jmh4).

#### Scenario: A green release carries the Go assets and flips the tap
- **GIVEN** a `v*` tag push
- **WHEN** the `release` job runs
- **THEN** the Go build/package/formula steps run before `gh release create`, the Release carries the five `tu-go-*` assets, and the tap's `Formula/tu.rb` equals the run's `dist/tu.rb` — `version "<v>"`, four sha256s equal to `tu-go-SHA256SUMS`, no `depends_on`

### Requirement: Rollback needs a higher-numbered Node release, not a tap revert alone
`tu update` runs `brew upgrade tu`, and Homebrew never downgrades an installed keg: reverting the tap's `Formula/tu.rb` to the Node formula at `tag: "v0.12.0"` leaves every machine that already installed 0.13.0 on the Go binary (the upgrade is a no-op that reports success). Rolling users back through the normal update path therefore requires a **release whose version is higher than the Go one and whose tap formula is the Node formula**: restore the `sed` tag-bump line in place of `cp dist/tu.rb` in `release.yml`'s tap step (`sed -i "s|tag: \"v.*\"|tag: \"v${version}\"|" /tmp/tap/Formula/tu.rb` against a tap `tu.rb` reverted to the Node formula), cut `just release` (e.g. 0.13.1), and `tu update` then installs the Node build from that tag. `src/node/` stays buildable for exactly this until plan row Z1 (D10). A tap revert on its own only protects machines that have not yet upgraded; a machine already on 0.13.0 needs an explicit `brew reinstall tu` after the revert (jmh4).

#### Scenario: Field divergence after cutover
- **GIVEN** a divergence found in the Go binary after 0.13.0 shipped
- **WHEN** the tap's `Formula/tu.rb` is reverted to the Node formula but no higher-numbered Node release is cut
- **THEN** `tu update` on a machine already at 0.13.0 leaves the Go binary installed (`brew upgrade` does not downgrade); only `brew reinstall tu` — or a 0.13.1 Node release pushed through the restored `sed` step — moves it back

### Requirement: CI cross-compiles on every PR
`ci.yml`'s `go-build-and-test` lane runs a `Cross-compile` step (`just go-build-all`) after `just go-build` — no network — so GOOS-specific compile breaks surface on every PR. The network-dependent packaging runs at release time and via `just go-dist` locally, not in the PR gate (a registry hiccup must not block merges). `go-diff` and `ci-gate` are unchanged (0118).

#### Scenario: A PR touching src/go
- **GIVEN** a pull request against main
- **WHEN** CI runs
- **THEN** all four release targets compile in the `go-build-and-test` lane

## Design Decisions

### Release artifacts live under `dist/`, the dev binary stays in `bin/`
**Decision**: Cross-compiled binaries go to `dist/bin/tu-<os>-<arch>`; archives, sums, and the generated formula to `dist/`; `bin/tu` remains the harness's dev binary.
**Why**: The plan names `dist/` for the formula; fab-kit uses `dist/bin/`; nothing collides with `dist/tu.mjs`/`dist/vendor/`; tu is not published to npm so the `files` sweep concern behind the `bin/` decision does not apply to release artifacts.
**Rejected**: `bin/release/` — diverges from every sibling's layout for no gain; `dist/go/` — extra nesting the formula script would have to know about.
*Introduced by*: 260917-0118-go-release-pipeline

### Formula installs into `libexec` and symlinks `bin/tu`
**Decision**: `libexec.install "tu", "vendor", "tu.default.conf"` + `bin.install_symlink libexec/"tu"`.
**Why**: `ccusage.ResolveBinary` resolves symlinks then looks for `vendor/ccusage/bin/ccusage` beside the real file; the `/Cellar/tu/` update gate passes on the libexec path; mirrors the Node formula's libexec layout.
**Rejected**: `bin.install "tu"` — would need `vendor/` inside Homebrew's `bin/`.
*Introduced by*: 260917-0118-go-release-pipeline

### ccusage tarballs come from the npm registry via curl, pinned by `CCUSAGE_VERSION`
**Decision**: `curl -fsSL` the `@ccusage/ccusage-<os>-<arch>` tarball URL and extract `package/bin/ccusage`; no node/npm in the fetch path; no separate upstream integrity check.
**Why**: curl survives Z1 (node goes away); `tu-go-SHA256SUMS` over the repo's own archives is what installs verify; the registry's integrity field comes from the same source.
**Rejected**: `npm pack` — needs node; `npm install` of the host package — vendors only the host's platform.
*Introduced by*: 260917-0118-go-release-pipeline

### The tap flip is unconditional — no bump-type guard in the workflow
**Decision**: From the first release after the cutover change merged, "Update Homebrew tap" always copies `dist/tu.rb` over the tap's formula; the workflow does not check whether the release is a minor bump. The minor-bump rule (D9) is the operator's `just release minor`.
**Why**: The tag-push path carries no bump type; an `X.Y.0` check would need tap state to know it is the first Go push; and any in-job guard fires after `scripts/release.sh` has already pushed the version commit and the `v*` tag, leaving a dangling tag — a worse mess than a Go formula under a patch number, which is a versioning-policy nit rather than a broken install.
**Rejected**: A `steps.version` regex guard failing the job; a tap-state probe (`depends_on "node"` present ⇒ require a `.0` version).
*Introduced by*: 260923-jmh4-cutover-formula
