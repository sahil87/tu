# Intake: Go Release Pipeline (Go port row R1)

**Change**: 260917-0118-go-release-pipeline
**Created**: 2026-09-17

## Origin

One-shot `/fab-new` invocation from the Go-port operator queue (Phase 3, row R1). Raw input:

> Context: fab/plans/sahil/26-09-15-go-port.md, row R1. Read the Decisions and Target architecture sections before writing the intake. Do not change any external surface listed under Goal. Build the Go release pipeline (D8): cross-compile four targets (copy fab-kit build-target/package-brew), fetch @ccusage/ccusage-<platform>-<arch> npm tarballs pinned in CCUSAGE_VERSION, pack binary plus vendor/ccusage/bin/ccusage plus tu.default.conf, upload as tu-go-*.tar.gz release assets alongside the existing release. Vendor-first resolution relative to os.Executable(). Generated Formula/tu.rb written to dist/ but NOT pushed to the tap yet. Add just dogfood-install [tag] (download the host's tu-go-* asset of the latest or named release, verify sha, install to ~/.local/bin/tu, print the PATH order so it shadows the brew tu) and just dogfood-uninstall. Assets appear only when a release is cut.

Context read before writing: the plan's § Decisions (D2 both trees in `main`, D5 sibling layout, D8 dogfood via release assets, D13 single formula flip), § Target architecture, § Releases and installing a build, § Deployment; constitution v1.2.0 § Go Transition; `docs/memory/build/toolchain.md` (release.yml shape, vendoring, the `bin/` vs `dist/` decision); `docs/memory/go-port/toolkit-layer.md` (the `/Cellar/tu/` update gate); fab-kit's `justfile`, `scripts/just/package-brew.sh`, `scripts/just/brew-formula.sh`, `.github/formula-template.rb`, and `release.yml` (the siblings' prebuilt-tarball pipeline this row copies).

State of the repo at intake (verified 2026-09-17):

- B1–B8 have all merged (`git log`: B7 watch mode is `7de6abc`, PR #95). The Go binary answers every surface; the harness reports 452/452 green (memory `go-port/command-edge`). R1 is the next row in the queue.
- `ccusage.ResolveBinary` (`src/go/internal/source/ccusage/exec.go`) **already** implements vendor-first resolution: `os.Executable()` → `filepath.EvalSymlinks` → `<dir>/vendor/ccusage/bin/ccusage`, then `ccusage` on PATH, then an `exec.ErrNotFound` error. It never probes `node_modules`. R1 does not change it; R1 produces the tarball layout it expects and proves the layout works through the Homebrew and `~/.local/bin` symlink chains.
- `tu.default.conf` is `go:embed`ded (`src/go/internal/config/defaults.go`) with a drift-guard test against the repo-root file. The tarball ships a sibling copy for layout parity with the Node `libexec/` (the formula installs `tu.default.conf` beside the artifact today), not because the Go binary reads it.
- `just go-build` writes `bin/tu` stamped `-X main.version=v<package.json version>`; `bin/` is gitignored; `dist/` is gitignored and holds the Node artifact (`dist/tu.mjs`, `dist/vendor/`).
- `release.yml`'s `release` job: checkout at the tag → setup-node 20 → (dispatch-only `release.sh`) → resolve tag → setup-just → `npm ci` → `npm run build` → `just release-notes` → `gh release create <tag> --title --notes-file` (no assets) → tap update by `sed`-bumping the tag in `homebrew-tap/Formula/tu.rb`. No `setup-go`. The latest release `v0.11.5` (2026-08-28) has zero assets.
- `package-lock.json` locks `ccusage` at **20.0.19** and lists the six `@ccusage/ccusage-{darwin,linux,win32}-{arm64,x64}` optional packages. There is no `CCUSAGE_VERSION` file yet. The npm registry tarball URL shape is `https://registry.npmjs.org/@ccusage/ccusage-<os>-<arch>/-/ccusage-<os>-<arch>-<version>.tgz` (verified with `npm view`; the binary sits at `package/bin/ccusage` inside it). npm arch names are `x64`/`arm64`; Go's are `amd64`/`arm64`.
- The tap formula (`sahil87/homebrew-tap/Formula/tu.rb`) builds from source: `npm install --include=dev`, `libexec.install "dist/tu.mjs"`, `libexec.install "dist/vendor"`, `libexec.install "tu.default.conf"`, `bin/tu` as an env script with node on PATH. R1 does not touch it.
- On dev-ws-sahil02 `$PATH` has `~/.local/bin` ahead of `/home/linuxbrew/.linuxbrew/bin`, and `which -a tu` resolves only the brew binary today.

## Why

**The problem.** Phase 3's dogfood (row R2, gate G2) needs the Go binary on three maintainer machines for a day of real use, and the plan forbids the two obvious delivery routes: no `tu-go` formula (it would touch the toolkit roster, `shll install`, and the install-composition standard for a throwaway — D8) and no formula flip yet (that is X1, after the harness gate R3). The only channel left is the GitHub Release itself: prebuilt per-platform tarballs uploaded next to the Node release, installed by hand. Nothing produces those tarballs today — `release.yml` has no Go toolchain, no cross-compile, no vendored ccusage for platforms other than the CI host, and no asset upload.

**What the row also buys.** The formula that X1 will push to the tap has to exist and be right before X1. Generating it every release from R1 onward (into `dist/`, never pushed) means its URL scheme, sha256 wiring and install layout are exercised on every release for the whole dogfood window, and X1 reduces to "copy `dist/tu.rb` into the tap" — a one-step change to a pipeline that already works. It also settles the ccusage vendoring question for the Go world: the Node build vendors whatever `npm install` picked for the CI host, which only works because Homebrew builds from source on the user's machine. A prebuilt tarball must carry the *target* platform's ccusage, fetched by name and pinned by version, so the four tarballs are byte-reproducible and the pin is visible in one file.

**If we do not.** R2 cannot start; G2 is blocked; the cutover timeline slips with no signal. The alternative — maintainers building from source on each machine — needs the Go toolchain on the Mac mini and MacBook Pro, produces unstamped or differently stamped binaries, and tests nothing about the distribution path X1 will actually use.

**Why this shape.** Copy fab-kit's `build-target` / `package-brew` / `brew-formula` split rather than inventing one: the six siblings distribute exactly this way, and the plan (D5, out-of-scope list) says tu copies rather than shares. Keep the Node release path untouched and additive: the Go steps run in the same job, before the GitHub Release is created, so a Go failure aborts the whole release before any external side effect (the existing fail-loud-first ordering), and a green release carries both the Node formula bump and the `tu-go-*` assets. The dogfood installer is a maintainer `just` recipe, not a CLI feature — no external surface changes.

## What Changes

### 1. `CCUSAGE_VERSION` — the single ccusage pin

- New repo-root file `CCUSAGE_VERSION` containing the bare version, one line, newline-terminated: `20.0.19` (the version `package-lock.json` locks today).
- The packaging script (§ 3) reads it and **fails loud** if it differs from the lockfile's `packages["node_modules/ccusage"].version`, so the Node and Go artifacts vendor the same ccusage for as long as both exist (the differential harness assumes one ccusage). Error text on stderr, exit 1:

  ```
  error: CCUSAGE_VERSION (20.0.19) != package-lock.json ccusage (20.0.20) — bump both together
  ```

- Bumping ccusage during the transition therefore means: `npm install ccusage@X` (lockfile) **and** edit `CCUSAGE_VERSION`. After Z1 removes the lockfile, the file is the only pin and the guard is deleted.

### 2. Cross-compile recipes (`justfile`)

Copy fab-kit's shape, shifted to tu's single-binary case:

```just
# Cross-compile bin for one target into dist/bin/tu-<os>-<arch> (release artifact staging; bin/tu stays the dev binary).
go-build-target os arch:
    just _go-skill-guard
    mkdir -p dist/bin
    cd src/go && CGO_ENABLED=0 GOOS={{os}} GOARCH={{arch}} go build -ldflags "-X main.version=v{{go_version}}" -o ../../dist/bin/tu-{{os}}-{{arch}} ./cmd/tu

# The four release targets (Homebrew's matrix: darwin/linux x arm64/amd64).
go-build-all:
    just go-build-target darwin arm64
    just go-build-target darwin amd64
    just go-build-target linux arm64
    just go-build-target linux amd64
```

- `_go-skill-guard` is the existing `cmp -s docs/site/skill.md src/go/internal/toolkit/skill.md || { … exit 1; }` line lifted out of `go-build` into a private recipe so `go-build` and `go-build-target` share one definition (code-quality: minimum pathways). `go-build`'s observable behavior (`bin/tu`, `bin/turepair`, the same error text) is unchanged — the harness recipes (`go-diff`, `go-live`) depend on it.
- Version stamp reuses the existing `go_version` variable (`node -p 'require("./package.json").version'`) — at the release checkout `package.json` is already bumped on all three `release.yml` entry paths, so the asset prints the tag's version.
- Only `cmd/tu` is cross-compiled. `turepair` is a maintainer tool built by `go-build` locally and is not shipped (the formula never shipped a repair binary).
- `CGO_ENABLED=0` for static binaries (fab-kit's `_build-binary`); the module's only dependency is `x/term`, which cross-compiles cleanly.

### 3. Packaging — `scripts/package-go.sh` and `just go-package`

Copy of fab-kit's `package-brew.sh`, plus the ccusage fetch. `just go-package` runs it; it requires `just go-build-all` to have run (missing `dist/bin/tu-<os>-<arch>` → `ERROR: Missing dist/bin/tu-<os>-<arch> — run 'just go-build-all' first.`, exit 1).

For each of `darwin/arm64 darwin/amd64 linux/arm64 linux/amd64`:

1. Map to the npm package: `darwin→darwin`, `linux→linux`, `arm64→arm64`, `amd64→x64` → `@ccusage/ccusage-<npm_os>-<npm_arch>`.
2. Fetch `https://registry.npmjs.org/@ccusage/ccusage-<npm_os>-<npm_arch>/-/ccusage-<npm_os>-<npm_arch>-${CCUSAGE_VERSION}.tgz` with `curl -fsSL` into `dist/ccusage-tarballs/` (kept between runs so a local re-package does not re-download; `curl -f` turns a 404 — wrong version, unpublished platform — into a hard error naming the URL). No node/npm involvement: `curl` is what survives Z1.
3. Extract exactly `package/bin/ccusage` from the tarball into a staging dir `dist/staging-tu-go-<os>-<arch>/`, laid out as:

   ```
   tu                          # dist/bin/tu-<os>-<arch>, renamed, 0755
   vendor/ccusage/bin/ccusage  # from the npm tarball, 0755
   tu.default.conf             # repo-root copy
   ```

   No top-level directory inside the archive (fab-kit's `brew-*` shape), so the formula and the dogfood installer extract in place.
4. `COPYFILE_DISABLE=1 tar czf dist/tu-go-<os>-<arch>.tar.gz -C <staging> tu vendor tu.default.conf`; remove the staging dir; print `  tu-go-<os>-<arch>.tar.gz (<bytes> bytes)`.

Then:

5. Write `dist/tu-go-SHA256SUMS` in `sha256sum` format over the four archives (`<sha>  tu-go-<os>-<arch>.tar.gz`, bare filenames) — the asset the dogfood installer verifies against. Use `shasum -a 256` (present on macOS and Linux) so the script runs locally on both.
6. **Host smoke test**: extract the tarball matching the running host (`uname -s`/`uname -m` → os/arch) into a temp dir and assert `./tu --version` prints exactly `tu version v${version}` and `./vendor/ccusage/bin/ccusage --version` exits 0. This is the one place the vendor-first resolution meets the real tarball layout before a release; cross-platform archives cannot be executed on the runner and are checked only structurally (`tar tzf` lists the three members).
7. Read `CCUSAGE_VERSION` and run the lockfile guard from § 1 **first**, before any download.

### 4. Formula generation — `.github/formula-template.rb`, `scripts/go-formula.sh`, `just go-formula [tag]`

Copy of fab-kit's `brew-formula.sh` + template, writing **`dist/tu.rb`** (fab-kit writes `dist/fab-kit.rb`). Tag defaults to `git describe --tags --abbrev=0 HEAD`; version is the tag without `v`. Requires the four archives (error text as fab-kit's). The template:

```ruby
class Tu < Formula
  desc "AI coding assistant cost tracking CLI"
  homepage "https://github.com/sahil87/tu"
  version "VERSION_PLACEHOLDER"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/sahil87/tu/releases/download/v#{version}/tu-go-darwin-arm64.tar.gz"
      sha256 "SHA_DARWIN_ARM64"
    end
    on_intel do
      url "https://github.com/sahil87/tu/releases/download/v#{version}/tu-go-darwin-amd64.tar.gz"
      sha256 "SHA_DARWIN_AMD64"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/sahil87/tu/releases/download/v#{version}/tu-go-linux-arm64.tar.gz"
      sha256 "SHA_LINUX_ARM64"
    end
    on_intel do
      url "https://github.com/sahil87/tu/releases/download/v#{version}/tu-go-linux-amd64.tar.gz"
      sha256 "SHA_LINUX_AMD64"
    end
  end

  def install
    libexec.install "tu", "vendor", "tu.default.conf"
    bin.install_symlink libexec/"tu"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/tu --version")
  end
end
```

- **Install layout rationale**: the real binary lives in `libexec/` with `vendor/` beside it, and `bin/tu` is a symlink — `ResolveBinary` resolves symlinks before looking for `vendor/ccusage/bin/ccusage` beside the real file, and the `tu update` gate tests the resolved path for `/Cellar/tu/`, which `…/Cellar/tu/<ver>/libexec/tu` satisfies. This mirrors the current Node formula (`libexec/` holds `tu.mjs`, `vendor/`, `tu.default.conf`). A plain `bin.install "tu"` would need `vendor/` inside Homebrew's `bin/`, which is wrong.
- No `depends_on "node"` — the point of the port.
- **Not pushed**: `release.yml`'s "Update Homebrew tap" step is unchanged (it still `sed`-bumps the tag in the tap's Node formula). The generated formula is `cat` into the workflow log after generation so each release shows the formula X1 would push. X1 replaces the `sed` with `cp dist/tu.rb /tmp/tap/Formula/tu.rb` (fab-kit's step, verbatim).

### 5. `release.yml` — Go assets alongside the Node release

Additive edits to the `release` job, all **before** "Create GitHub Release" (fail-loud-first: a Go failure aborts the run before the Release or the tap is touched):

- After the setup-node step: `actions/setup-go` pinned to the same SHA `ci.yml` uses (`40f1582b…` v5) with `go-version-file: src/go/go.mod` and `cache: false` (matches the CI lanes).
- After "Build bundle" (`npm run build` — the lockfile guard in § 3 needs `package-lock.json`, present regardless):

  ```yaml
      - name: Cross-compile Go binaries
        run: just go-build-all

      - name: Package tu-go assets
        run: just go-package

      - name: Generate Go formula (dist/tu.rb — not pushed until cutover, plan row X1)
        run: |
          just go-formula "${{ steps.version.outputs.tag }}"
          cat dist/tu.rb
  ```

- "Create GitHub Release" gains the assets:

  ```yaml
          gh release create "${{ steps.version.outputs.tag }}" \
            --title "tu ${{ steps.version.outputs.tag }}" \
            --notes-file dist/release-notes.md \
            dist/tu-go-darwin-arm64.tar.gz \
            dist/tu-go-darwin-amd64.tar.gz \
            dist/tu-go-linux-arm64.tar.gz \
            dist/tu-go-linux-amd64.tar.gz \
            dist/tu-go-SHA256SUMS
  ```

- The header comment gains one line: from R1 the release also uploads `tu-go-*` assets (D8); the formula still ships the Node build until X1.
- `tag-on-release-merge` and the tap step are untouched. Assets exist only for releases cut after this merges — `v0.11.5` and earlier stay asset-less, and the dogfood installer says so (§ 7).

### 6. Local aggregate and CI cross-compile check

- `just go-dist`: `go-build-all` → `go-package` → `go-formula` — everything the release job runs minus the uploads (fab-kit's `dist` recipe). This is how the packaging path is exercised before the first release: run it once during apply and once in review.
- `ci.yml` `go-build-and-test` lane: add a `Cross-compile` step running `just go-build-all` after `just go-build`. It needs no network and catches GOOS-specific compile breaks (e.g. an `x/term` or `os/signal` call that only builds on Linux) on every PR. Packaging (network fetch) is **not** added to the PR gate — a registry hiccup must not block merges; it runs at release time and via `just go-dist` locally. The `go-diff` job and `ci-gate` are unchanged.

### 7. `just dogfood-install [tag]` — `scripts/dogfood-install.sh`

Maintainer recipe, no CLI surface. Requires `gh` (authenticated) and `tar`, `shasum`.

```just
# Install the Go dogfood build from a release's tu-go-* asset into ~/.local (plan rows R1/R2). Default: latest release.
dogfood-install tag="":
    scripts/dogfood-install.sh {{tag}}

# Remove the dogfood build so `tu` resolves to the brew binary again.
dogfood-uninstall:
    scripts/dogfood-uninstall.sh
```

Behavior of `dogfood-install.sh`:

1. **Resolve the tag**: `$1` if given, else `gh release view --repo sahil87/tu --json tagName -q .tagName` (the latest published release). Print `Release: <tag>`.
2. **Detect the host**: `uname -s` → `darwin`/`linux`, `uname -m` → `x86_64→amd64`, `arm64|aarch64→arm64`; anything else is an error. Asset = `tu-go-<os>-<arch>.tar.gz`.
3. **Download** into a temp dir: `gh release download "<tag>" --repo sahil87/tu --pattern "tu-go-<os>-<arch>.tar.gz" --pattern tu-go-SHA256SUMS --dir "$tmp"`. If the tarball is absent afterwards: `error: release <tag> has no tu-go-<os>-<arch>.tar.gz asset — Go assets exist only for releases cut after plan row R1 landed`, exit 1.
4. **Verify**: compute `shasum -a 256` of the tarball and compare with the matching line of `tu-go-SHA256SUMS`; mismatch → `error: sha256 mismatch for <asset> (expected …, got …)`, exit 1, nothing installed.
5. **Install**: `rm -rf ~/.local/lib/tu-go && mkdir -p ~/.local/lib/tu-go && tar xzf "$tmp/<asset>" -C ~/.local/lib/tu-go` (yields `tu`, `vendor/ccusage/bin/ccusage`, `tu.default.conf`); then `ln -sfn ~/.local/lib/tu-go/tu ~/.local/bin/tu`. Guard: if `~/.local/bin/tu` exists and is **not** a symlink into `~/.local/lib/tu-go/`, refuse with `error: ~/.local/bin/tu exists and is not a dogfood symlink — remove it first`, exit 1 (never clobber a hand-installed binary). The symlink is what makes vendor-first resolution work: `ResolveBinary` resolves `~/.local/bin/tu` → `~/.local/lib/tu-go/tu` and finds `vendor/` beside it.
6. **Report**:

   ```
   Installed: ~/.local/bin/tu -> ~/.local/lib/tu-go/tu (tu version v0.11.6)
   PATH order for tu:
     1. /home/sahil/.local/bin/tu      <- dogfood (Go)
     2. /home/linuxbrew/.linuxbrew/bin/tu  <- brew (Node)
   OK: the dogfood build shadows the brew tu. Run `hash -r` (or open a new shell) if `tu` still resolves to brew.
   ```

   The list is `which -a tu` after prepending nothing — it reflects the caller's real `$PATH`. If the first entry is **not** `~/.local/bin/tu`: `WARNING: <first path> wins on PATH — the brew tu still runs. Prepend ~/.local/bin to PATH (e.g. in ~/.zshrc) and re-run.`, exit 0 (the install itself succeeded).
7. Note printed once: `tu update` on this binary prints the not-installed-via-Homebrew message (the `/Cellar/tu/` gate) — refresh with `just dogfood-install` after each fix release, per plan row R2 step 3.

### 8. `just dogfood-uninstall` — `scripts/dogfood-uninstall.sh`

Idempotent: remove `~/.local/bin/tu` **only** if it is a symlink into `~/.local/lib/tu-go/` (otherwise leave it and say so), `rm -rf ~/.local/lib/tu-go`, then print `which -a tu` again with `Removed dogfood build; tu now resolves to: <path>` (or `no tu on PATH`). Plan row R2 step 5 runs this before X1 so `tu update` resolves to the brew binary again.

### 9. Vendor-first resolution relative to `os.Executable()` — verification only

No change to `src/go/internal/source/ccusage/exec.go`. R1 adds the proof that the shipped layouts satisfy it:

- The host smoke test in § 3 runs the real binary from an extracted tarball (direct execution, vendor beside the binary).
- The `~/.local/bin/tu → ~/.local/lib/tu-go/tu` symlink chain is exercised by running `just dogfood-install` against a local tarball during review is not possible before a release exists; instead, a Go test in `internal/source/ccusage` (extend `exec_test.go` if a `ResolveBinary` test exists, else add one) builds a temp tree `<dir>/real/tu` + `<dir>/real/vendor/ccusage/bin/ccusage` + `<dir>/bin/tu → ../real/tu` and asserts `ResolveBinary` resolves through the symlink to the vendored path. If the existing test already covers a symlinked executable, no new test.

### 10. Documentation and plan bookkeeping

- `docs/memory/build/toolchain.md` — hydrate: the new recipes, `release.yml` shape, `CCUSAGE_VERSION`, the CI cross-compile step; amend the design decision "Go build output goes to `bin/`, not `dist/`" to read: the **dev** binary stays `bin/tu`; **release** artifacts (`dist/bin/tu-<os>-<arch>`, `dist/tu-go-*.tar.gz`, `dist/tu-go-SHA256SUMS`, `dist/tu.rb`) go to `dist/`, where the sibling pipelines put theirs and where nothing conflicts with `dist/tu.mjs`/`dist/vendor/`.
- New memory file `docs/memory/build/go-release-pipeline.md` — the asset contract (names, tar layout, sums format), the ccusage fetch + pin + lockfile guard, the formula template and its install layout, the dogfood install/uninstall contract and its error texts, what X1 changes.
- `fab/plans/sahil/26-09-15-go-port.md` row R1: set the PR column to this change and the Status column to landed (one line), as the Phase 0 rows did.
- `README.md`, `docs/site/skill.md`, `docs/specs/*`: **untouched** — no user-facing surface changes; README install prose points at hexokit.com (Policy B).

### Out of scope (explicitly)

- Pushing `dist/tu.rb` to the tap, touching the tap's `Formula/tu.rb`, or removing `depends_on "node"` from the shipped formula — X1.
- A `tu-go` formula, a `TU_IMPL` switch, or any change to `tu update`'s behavior on the dogfood binary — D8/D13.
- Windows assets (ccusage publishes `win32-*`; Homebrew-only stays).
- Adding the harness to `ci-gate` — R3.
- Cutting a release — R2 is Sahil's `just release`.

## Affected Memory

- `build/toolchain`: (modify) `just go-build-target`/`go-build-all`/`go-package`/`go-formula`/`go-dist`/`dogfood-install`/`dogfood-uninstall`, the shared `_go-skill-guard`, `CCUSAGE_VERSION` + lockfile guard, `release.yml`'s setup-go and Go asset steps, `ci.yml`'s cross-compile step; amend the `bin/` vs `dist/` design decision (dev binary vs release artifacts)
- `build/go-release-pipeline`: (new) the `tu-go-*` asset contract, tarball layout, `tu-go-SHA256SUMS`, ccusage fetch mapping and pin, formula template + `libexec` install layout, dogfood install/uninstall behavior and error texts, the X1 handoff

## Impact

- **New files**: `CCUSAGE_VERSION`, `scripts/package-go.sh`, `scripts/go-formula.sh`, `scripts/dogfood-install.sh`, `scripts/dogfood-uninstall.sh`, `.github/formula-template.rb`, `docs/memory/build/go-release-pipeline.md`.
- **Modified**: `justfile` (new recipes, `_go-skill-guard` extraction), `.github/workflows/release.yml` (setup-go, three Go steps, asset args, header comment), `.github/workflows/ci.yml` (one cross-compile step in `go-build-and-test`), `docs/memory/build/toolchain.md`, `fab/plans/sahil/26-09-15-go-port.md` (row R1 status), possibly `src/go/internal/source/ccusage/exec_test.go` (symlink resolution test).
- **Unchanged**: every external surface in the plan's Goal; `src/go/internal/source/ccusage/exec.go`; the tap; `scripts/build.sh`, `scripts/release.sh`; `bin/tu` and the harness recipes.
- **Verification during apply/review**: `just go-lint`, `just go-test`, `just go-build`, then `just go-dist` end to end on dev-ws-sahil02 (four archives, sums, `dist/tu.rb`, host smoke test green), `tar tzf` of each archive showing exactly `tu`, `vendor/ccusage/bin/ccusage`, `tu.default.conf`, `shasum -c` of the sums file, `just go-diff` still 452/452. The workflow YAML cannot be executed pre-merge; review reads it against fab-kit's working `release.yml` line by line. `just dogfood-install` cannot succeed until a release with assets exists — review exercises its error path (`v0.11.5` → the no-asset error) and the uninstall's idempotence.
- **Risk**: the first real run of the Go steps is the first release after merge (R2 step 1). Mitigation is `just go-dist` locally plus the CI cross-compile step; if the release run fails it aborts before the Release/tap are touched, so the failure mode is "no release", not "broken release".
- **Dependencies**: `curl`, `tar`, `shasum` on the runner and maintainer machines (all present on ubuntu-latest and macOS); `gh` authenticated on maintainer machines; Go toolchain on the runner via `setup-go`.

## Open Questions

- None blocking. Two judgment calls are recorded as graded rows below rather than asked: (a) the generated formula is written to `dist/` and echoed in the workflow log but **not** uploaded as a release asset; (b) the PR gate cross-compiles but does not run the network-dependent packaging.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Asset names are `tu-go-<os>-<arch>.tar.gz` with Go's GOOS/GOARCH spellings (`darwin`/`linux` × `arm64`/`amd64`), archives contain `tu`, `vendor/ccusage/bin/ccusage`, `tu.default.conf` with no top-level directory | Plan text names `tu-go-<os>-<arch>.tar.gz`; fab-kit's `brew-<os>-<arch>` uses the same spellings and flat layout; the formula and installer extract in place | S:90 R:70 A:90 D:85 |
| 2 | Certain | `CCUSAGE_VERSION` is a repo-root one-line file pinned to `20.0.19`, and `package-go.sh` fails loud when it differs from `package-lock.json`'s ccusage version | Plan says "pinned in `CCUSAGE_VERSION`"; the lockfile locks 20.0.19 today; the harness assumes both binaries exec the same ccusage | S:85 R:90 A:90 D:85 |
| 3 | Confident | Cross-compiled binaries go to `dist/bin/tu-<os>-<arch>` and archives/sums/formula to `dist/`; `bin/tu` stays the dev binary; the toolchain memory's `bin/` decision is amended, not reversed | Plan says the formula is written to `dist/`; fab-kit uses `dist/bin/`; nothing collides with `dist/tu.mjs`/`dist/vendor/`; the recorded `bin/` decision was about the dev binary and npm `files` (tu is not published to npm) | S:80 R:80 A:70 D:65 |
| 4 | Confident | The generated formula installs `tu`, `vendor`, `tu.default.conf` into `libexec` and symlinks `bin/tu`; template lives at `.github/formula-template.rb`; output is `dist/tu.rb` | `ResolveBinary` resolves symlinks then looks beside the real file; the `/Cellar/tu/` update gate passes on the libexec path; mirrors the Node formula's libexec layout and fab-kit's template location | S:70 R:75 A:80 D:70 |
| 5 | Confident | `dogfood-install` extracts to `~/.local/lib/tu-go/` and symlinks `~/.local/bin/tu`; refuses to clobber a non-dogfood `~/.local/bin/tu` | Plan says install to `~/.local/bin/tu`; the vendor tree must sit beside the real binary, so a symlink is the only way to honor both; siblings' `install.sh` also target `~/.local/bin` | S:75 R:85 A:80 D:65 |
| 6 | Confident | ccusage tarballs are fetched with `curl -fsSL` from `registry.npmjs.org` and `package/bin/ccusage` is extracted; a 404 is a hard error; no separate upstream integrity check | URL shape and member path verified with `npm view`; curl needs no node (survives Z1); `tu-go-SHA256SUMS` over our archives is what installs verify; the registry's own integrity field would come from the same source | S:70 R:85 A:75 D:65 |
| 7 | Confident | Go steps run in the existing `release` job before "Create GitHub Release" and a failure aborts the whole release (no `continue-on-error`) | The job's fail-loud-first ordering already exists for the Node build; D8 says the release job builds both; silently shipping without assets would hide the failure R2 depends on | S:70 R:85 A:80 D:70 |
| 8 | Confident | `dist/tu.rb` is echoed in the workflow log and not uploaded as a release asset or pushed anywhere | Plan says written to `dist/`, not pushed; the log makes it inspectable per release; an uploaded formula asset could be mistaken for an install path during dogfood | S:60 R:90 A:55 D:40 |
| 9 | Confident | `ci.yml`'s `go-build-and-test` lane gains `just go-build-all` (no network); packaging runs only at release time and via `just go-dist` locally | Cross-compile breaks are cheap to catch per PR; a registry fetch in the PR gate would let a network hiccup block merges; the plan says assets appear only when a release is cut | S:55 R:90 A:70 D:55 |
| 10 | Certain | `turepair` is not cross-compiled or shipped | It is a maintainer repair tool built locally by `go-build`; the formula never shipped a repair binary | S:80 R:90 A:90 D:90 |
| 11 | Confident | `package-go.sh` smoke-tests the host's archive (`./tu --version` byte-exact, vendored `ccusage --version` exit 0) and only lists members of the other three | Cross-platform binaries cannot run on the runner; the host check is the one pre-release execution of the tarball layout against `ResolveBinary` | S:60 R:90 A:80 D:70 |
| 12 | Certain | Vendor-first resolution relative to `os.Executable()` needs no source change; R1 verifies it (host smoke test, symlink-chain unit test) | `ccusage.ResolveBinary` already does `os.Executable` → `EvalSymlinks` → `vendor/ccusage/bin/ccusage` → PATH (`exec.go`, memory `go-port/fact-and-sources`) | S:90 R:90 A:95 D:90 |
| 13 | Confident | The checksum asset is `dist/tu-go-SHA256SUMS` in `sha256sum` format (bare filenames), generated with `shasum -a 256`; the installer compares against the matching line | Keeps the `tu-go-` prefix on every Go asset; fab-kit ships `SHA256SUMS`; `shasum` exists on both macOS and Linux while `sha256sum` is Linux-only | S:70 R:85 A:80 D:65 |
| 14 | Confident | `change_type` is pinned `feat` (explicit) for consistency with rows P0–B7 | The row adds new release artifacts and maintainer recipes; the series has shipped every row as `feat:`; inference on "release pipeline"/"CI" wording could land on `ci` | S:60 R:95 A:75 D:60 |
| 15 | Confident | `just go-dist` aggregates `go-build-all` → `go-package` → `go-formula` for local end-to-end runs | fab-kit's `dist` recipe is the precedent; it is the only way to exercise packaging before the first release | S:50 R:95 A:70 D:50 |
| 16 | Confident | `dogfood-install` defaults to the latest published release via `gh release view` and requires an authenticated `gh` | Plan says "latest or named release"; `gh` is already the toolkit's release client and is on every maintainer machine | S:75 R:85 A:80 D:70 |
| 17 | Certain | Row R1's PR and Status columns in the plan doc are updated when the change lands | The plan header says "update the status column as they land" and the Phase 0 rows did | S:65 R:95 A:80 D:75 |

17 assumptions (5 certain, 12 confident, 0 tentative, 0 unresolved).
