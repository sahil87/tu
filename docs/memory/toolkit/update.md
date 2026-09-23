---
type: memory
description: The `tu update` Homebrew self-update — the Brew driver interface with BrewExec, SIGTERM-graceful 600 s/60 s bounds on the metadata calls, the unbounded interactive `brew upgrade` with HOMEBREW_NO_ASK=1, the /Cellar/tu/ install gate, --skip-brew-update, and the frozen wrapper lines and failure messages.
---

# Update

**Domain**: toolkit

## Overview

`tu update` refreshes a Homebrew-installed tu in place through three brew subprocesses driven by the injected `Brew` seam in `internal/toolkit/update.go`; `cmd/tu`'s `runUpdate` sequences the steps and prints every wrapper line — the toolkit package never writes to stdout/stderr itself. Dispatch order is in [entry-point](/command/entry-point.md); the release pipeline that ships the formula is in [go-release-pipeline](/build/go-release-pipeline.md).

## Requirements

### Requirement: Brew driver seam
- `Brew` in `internal/toolkit/update.go` MUST be an interface — `Update(ctx) error`, `Info(ctx) ([]byte, error)`, `Upgrade(ctx, stdin io.Reader, stdout, stderr io.Writer) error` — with `BrewExec` driving the real brew found on PATH. Tests substitute a fake (`update_test.go`), so the full Homebrew sequence is unit-testable without brew. This mirrors the `internal/sync` Exec precedent.
- The argv MUST be verbatim: `brew update --quiet` (streams captured), `brew info --json=v2 tu` (stdout returned), `brew upgrade tu` (interactive, streams passed through — not the fully-qualified `sahil87/tap/tu`).

### Requirement: Bounded metadata calls, unbounded upgrade
- `BrewExec.Update` MUST be bounded at 600 s (`brewUpdateTimeout` in `internal/toolkit/update.go`) and `BrewExec.Info` at 60 s (`brewInfoTimeout`), both via `context.WithTimeout` — bounds sized for a network transfer (ba5w).
- Every bounded brew call MUST go through `boundedBrewCmd`, which overrides `cmd.Cancel` to send SIGTERM (trappable — brew can finish or roll back its transaction) instead of the `exec.CommandContext` default SIGKILL, and sets `cmd.WaitDelay = brewGraceDelay` (10 s) of grace before the runtime's forced kill. No code path may SIGKILL a package-manager subprocess mid-transaction (ba5w, vcur).
- `BrewExec.Upgrade` MUST be unbounded: `exec.Command`, not `CommandContext` — no deadline, no `Cancel` override, `WaitDelay` 0. A kill landing mid-transaction corrupts the keg; the interactive call's escape hatch is Ctrl-C (ba5w).
- The upgrade child environment MUST append `HOMEBREW_NO_ASK=1` to `os.Environ()` — Homebrew 6 ask-mode suppression via the env var, not `--no-ask`, so Homebrew < 6 is unaffected (wdjt).

#### Scenario: a metadata call exceeds its deadline
- **GIVEN** `brew update --quiet` still running 600 s after launch
- **WHEN** the context deadline expires
- **THEN** the subprocess receives SIGTERM, gets 10 s (`brewGraceDelay`) to unwind before the runtime's forced kill, and `CheckLatest` maps the failure to `Error: could not check for updates (brew update failed). Check your network connection.` on stderr, exit 1

### Requirement: Homebrew install gate
- `IsBrewInstall(resolvedExe)` in `internal/toolkit/update.go` MUST report whether the symlink-resolved executable path contains the substring `/Cellar/tu/` — narrower than the sibling tools' `/Cellar/` gate (ba5w, vcur).
- `cmd/tu`'s `runUpdate` MUST resolve `os.Executable()` through `filepath.EvalSymlinks` before gating; a resolution error or a failing gate MUST print the two `NotBrewInstallLines` to stdout and exit 0 — no brew command runs.

#### Scenario: dev build runs tu update
- **GIVEN** the executable resolves outside `/Cellar/tu/` (a `go build` dev binary, the `go test` binary, a dogfood install)
- **WHEN** `tu update` runs
- **THEN** stdout is exactly `tu v0.11.5 was not installed via Homebrew.` and `Update manually, or reinstall with: brew install sahil87/tap/tu` (display form; an unstamped dev build prints `tu dev was not installed via Homebrew.`), stderr is empty, exit 0

### Requirement: CheckLatest, UpToDate, and the skip flag
- `CheckLatest(ctx, brew, skipBrewUpdate)` in `internal/toolkit/update.go` MUST run `brew update --quiet` unless `skipBrewUpdate`, then `brew info --json=v2 tu`, then parse `formulae[0].versions.stable`. `--skip-brew-update` gates ONLY the tap-metadata refresh; the info check, the up-to-date short-circuit, and the upgrade are unaffected (e96v).
- Any `Update` error MUST map to `msgUpdateFailed`; any exec error, invalid JSON, empty `formulae`, or a missing/blank/non-string `stable` MUST map to `msgLatestUnknown` — the typed unmarshal rejects a non-string `stable` the way the frozen oracle's `typeof stable !== "string"` guard does. Both are `*UpdateError` values printed to stderr by `cmd/tu`, exit 1.
- `UpToDate(version, latest)` MUST compare `BareVersion(version) == latest` — brew reports the bare form, so the binary's leading `v` is stripped first. An up-to-date brew short-circuits before `Upgrade` runs.
- The three failure messages are frozen bytes: `Error: could not check for updates (brew update failed). Check your network connection.` / `Error: could not determine latest version.` / `Error: brew upgrade failed.`

#### Scenario: update dispatch and exit codes
- **GIVEN** any of: `--help`/`-h` anywhere in the args; off-Homebrew; up-to-date; a successful upgrade; a brew failure
- **WHEN** `runUpdate` in `cmd/tu/main.go` executes
- **THEN** `--help` prints `command.FullHelp` and runs nothing (exit 0); off-Homebrew prints the two gate lines (exit 0); up-to-date prints `Already up to date (v…).` (exit 0); success ends with `Updated to v….` (exit 0); each brew failure prints its exact line to stderr (exit 1)

### Requirement: Wrapper lines
- The wrapper lines — `CurrentVersionLine` (`Current version: v…`), `AlreadyUpToDateLine` (`Already up to date (v…).`), `UpdatingLine` (`Updating v… → v…...` — arrow `→`, three dots), `UpdatedLine` (`Updated to v….`) — MUST use `DisplayVersion` for every version string (internal/toolkit/update.go).

## Design Decisions

### update through an injected Brew driver
**Decision**: `toolkit` exposes a `Brew` interface with `BrewExec` as the real driver; the package never writes to stdout/stderr — `cmd/tu` sequences the steps and prints the wrapper lines; the interactive `brew upgrade` streams pass through the driver call.
**Why**: nothing below `cmd/tu` prints; the `internal/sync` Exec precedent already execs a subprocess below the edge with streams passed through; a fake driver makes the whole Homebrew sequence unit-testable without brew.
**Rejected**: running the exec directly in `cmd/tu` (untestable, bloats the edge); a `toolkit.Update(...)` that prints (breaks the only-writer rule).
*Introduced by*: 260916-vcur-toolkit-layer

### SIGTERM-graceful bounds on metadata calls; the upgrade unbounded
**Decision**: `brew update --quiet` and `brew info --json=v2 tu` are bounded at 600 s/60 s with `cmd.Cancel` overridden to SIGTERM plus a 10 s `WaitDelay` grace; `brew upgrade tu` carries no deadline machinery at all.
**Why**: a SIGKILL landing mid-transaction corrupts the keg — the cited incident is a 120 s hard timeout that killed a stalled `brew upgrade` mid-swap and left the binary unexecutable; the metadata calls are non-interactive network transfers that must not hang forever, so they get generous bounds with a trappable signal.
**Rejected**: one timeout for all three calls (the incident's shape); the `exec.CommandContext` default SIGKILL (untrappable — no chance to finish or roll back).
*Introduced by*: 260719-ba5w-update-version-shellinit-conformance (mechanism: 260916-vcur-toolkit-layer)

### Homebrew ask-mode suppression via env var
**Decision**: the interactive `brew upgrade tu` child environment appends `HOMEBREW_NO_ASK=1`, keeping the streams inherited; no code path reads stdin for a confirmation.
**Why**: Homebrew 6 made ask mode the default for `brew upgrade`; with inherited stdio both stdin and stdout are TTYs, so the `[y/n]` prompt fires and blocks the update indefinitely — the env var is the version-proof disable, and an unrecognized env var is harmlessly ignored.
**Rejected**: passing `--no-ask`/`--yes`/`-y` to `brew upgrade` — Homebrew < 6 does not know the flag and would error.
*Introduced by*: 260812-wdjt-fix-update-brew-ask-prompt

### `/Cellar/tu/` install gate
**Decision**: the Homebrew-install test is the substring `/Cellar/tu/` on the symlink-resolved executable, narrower than the sibling tools' `/Cellar/`.
**Why**: only tu's own formula keg should take the in-place-upgrade path; a bare `/Cellar/` substring would also match a tu binary that happens to resolve under another formula's keg.
**Rejected**: the siblings' `/Cellar/` (too broad); a prefix match on a fixed Cellar root (Cellar location differs across Intel, Apple Silicon, and Linuxbrew hosts).
*Introduced by*: 260719-ba5w-update-version-shellinit-conformance (substring reproduced verbatim: 260916-vcur-toolkit-layer)
