---
type: memory
description: "Maintainer dogfood recipes — just dogfood-install [tag] downloads the host's tu-go-<os>-<arch>.tar.gz and tu-go-SHA256SUMS from a GitHub Release via gh, verifies the sha256, installs under ~/.local/lib/tu-go/ behind a ~/.local/bin/tu symlink (never clobbering a hand-installed binary) and reports PATH shadowing; just dogfood-uninstall removes only that symlink and directory, idempotently."
---
# Dogfood Install

**Domain**: build

## Overview

`just dogfood-install [tag]` and `just dogfood-uninstall` install and remove a released Go build under `~/.local` for maintainer use, outside Homebrew. The assets they download are produced by the pipeline in [go-release-pipeline](/build/go-release-pipeline.md); the `tu update` gate the dogfood binary trips is in [update](/toolkit/update.md).

## Requirements

### Requirement: `just dogfood-install [tag]`
`scripts/dogfood-install.sh` (wrapped by `just dogfood-install tag=""`) requires an authenticated `gh`, plus `tar` and `shasum`. It resolves the tag (`$1`, else `gh release view --repo sahil87/tu --json tagName -q .tagName` — the latest published release) and prints `Release: <tag>`; detects the host (`uname -s` → `darwin`/`linux`, `uname -m` → `x86_64`→`amd64`, `arm64`/`aarch64`→`arm64`; anything else is `error: unsupported host OS: …` / `error: unsupported host arch: …`, exit 1); checks the release's asset list first via `gh release view --json assets` and, when it truly lacks `tu-go-<os>-<arch>.tar.gz` or `tu-go-SHA256SUMS`, fails with `error: release <tag> has no <asset> asset — Go assets exist only for releases cut after plan row R1 landed` (exit 1); otherwise downloads both into a temp dir via `gh release download` (a gh download/auth/network failure aborts loud rather than surfacing as the no-asset error); verifies the tarball's `shasum -a 256` against the matching sums line (mismatch → `error: sha256 mismatch for <asset> (expected <a>, got <b>)`, exit 1, nothing installed); refuses when `~/.local/bin/tu` exists and is not a symlink into `~/.local/lib/tu-go/` (`error: ~/.local/bin/tu exists and is not a dogfood symlink — remove it first`, exit 1 — a hand-installed binary is never clobbered); otherwise replaces `~/.local/lib/tu-go/` with the extracted archive and `ln -sfn ~/.local/lib/tu-go/tu ~/.local/bin/tu`. The symlink is what makes vendor-first resolution work: `ResolveBinary` resolves `~/.local/bin/tu` to the real file and finds `vendor/` beside it. The report is `Installed: ~/.local/bin/tu -> ~/.local/lib/tu-go/tu (<tu --version output>)`, a numbered `PATH order for tu:` list from `which -a tu` annotated `<- dogfood (Go)` / `<- brew (Node)` (linuxbrew/homebrew/Cellar paths and the Intel-Homebrew `/usr/local/bin/tu` symlink) / `<- other`, then either `OK: the dogfood build shadows the brew tu. Run \`hash -r\` (or open a new shell) if \`tu\` still resolves to brew.` (when the first entry is the dogfood symlink) or `WARNING: <first> wins on PATH — the brew tu still runs. Prepend ~/.local/bin to PATH (e.g. in ~/.zshrc) and re-run.` (exit 0 either way — the install succeeded), plus a note that `tu update` on this binary prints the not-installed-via-Homebrew message (the `/Cellar/tu/` gate — see [update](/toolkit/update.md)) and that `just dogfood-install` refreshes the build (0118).

#### Scenario: No Go assets on the release
- **GIVEN** the latest release predates the pipeline
- **WHEN** `just dogfood-install` runs
- **THEN** it prints `Release: <tag>` then the no-asset error, exits 1, and nothing under `~/.local` is touched

#### Scenario: Successful install shadows brew
- **GIVEN** a release with `tu-go-*` assets and a host whose `~/.local/bin` precedes the brew bin on PATH
- **WHEN** `just dogfood-install <tag>` runs
- **THEN** the sha verifies, `~/.local/lib/tu-go/{tu,vendor/ccusage/bin/ccusage,tu.default.conf}` exist, `~/.local/bin/tu` is the symlink, and the report ends with the `OK:` line

### Requirement: `just dogfood-uninstall`
`scripts/dogfood-uninstall.sh` removes `~/.local/bin/tu` **only** when it is a symlink whose target lies under `~/.local/lib/tu-go/` (otherwise it prints `leaving ~/.local/bin/tu in place (not a dogfood symlink)`), removes `~/.local/lib/tu-go/`, and prints `Removed dogfood build; tu now resolves to: <first which -a tu entry>` or `Removed dogfood build; no tu on PATH`. It is idempotent — a second run exits 0 with the same final line (0118).

#### Scenario: Idempotent removal
- **GIVEN** a dogfood install exists
- **WHEN** `just dogfood-uninstall` runs twice
- **THEN** both runs exit 0, the symlink and lib dir are gone after the first, and `tu` resolves to the brew binary

## Design Decisions

### Dogfood binary lives in `~/.local/lib/tu-go/` behind a `~/.local/bin/tu` symlink
**Decision**: Extract the archive to `~/.local/lib/tu-go/` and symlink `~/.local/bin/tu` to its `tu`.
**Why**: The vendor tree must sit beside the real binary and `~/.local/bin` should hold only the entry point; the symlink is what the resolver follows.
**Rejected**: Copying `tu` and a `vendor/` directory straight into `~/.local/bin/` — pollutes the bin dir and is hard to uninstall cleanly.
*Introduced by*: 260917-0118-go-release-pipeline
