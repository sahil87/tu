---
type: memory
description: "The harness replayer binaries — fakeccusage serving fixture bytes verbatim by (source, period, flags) from TUDIFF_FIXTURES with a loud exit-2 miss, fakegit answering every git argv from TUDIFF_GIT_SCRIPT rules or a silent exit 0, the shared TUDIFF_CALL_LOG JSONL line shape, and how the fake ccusage is staged into the oracle's fixed vendor slot."
---
# Replayers

**Domain**: harness

## Overview

`src/go/cmd/fakeccusage` and `src/go/cmd/fakegit` are the two replayer binaries `tudiff run` places first on the child `PATH` (built into `bin/harness/` by `just harness-build`); both append to the shared call log via `harness.LogCall`. The per-case environment that configures them is built in [matrix-and-staging](/harness/matrix-and-staging.md).

## Requirements

### Requirement: Fake ccusage
`src/go/cmd/fakeccusage` (built as `bin/harness/ccusage`) SHALL be a static binary configured only by environment variables, because its argv belongs to tu. `TUDIFF_FIXTURES` (required) is an OS-path-list of fixture alias directories searched in order, first hit wins; unset or empty prints `fakeccusage: TUDIFF_FIXTURES not set` to stderr and exits 2. argv parses as `<source> <period> [flags…]`; the key is `(source, period, sorted flags)` and must equal a manifest entry's `(source, period, args)` exactly. On a hit the fake writes the fixture file bytes verbatim to stdout, the recorded stderr (if any) to stderr, and exits with the recorded `exit_code`. On a miss it prints `fakeccusage: no fixture for argv […]` to stderr and exits 2 — deliberately loud, because a Go port sending ccusage an argv the TypeScript binary never sent is itself a divergence the harness must surface. `--version`/`-v` as the sole argument prints `ccusage <ccusage_version>` from the first manifest found and exits 0 (no manifest found → exit 2).

#### Scenario: Verbatim replay and loud miss
- **GIVEN** `TUDIFF_FIXTURES=harness/fixtures/<local-capture>:harness/fixtures/_placeholder` (or `_placeholder` alone on a machine without a local capture)
- **WHEN** `bin/harness/ccusage claude daily --json` runs
- **THEN** stdout is byte-identical to the first alias's `claude/daily.json` and the exit code is 0
- **GIVEN** the same environment
- **WHEN** `bin/harness/ccusage kimi weekly --json` runs
- **THEN** the exit code is 2 and stderr starts with `fakeccusage: no fixture for argv`

### Requirement: Fake git
`src/go/cmd/fakegit` (built as `bin/harness/git`) sits first on `PATH` so every git invocation tu makes is intercepted (tu reaches `git` through `PATH` on both sides). It SHALL log every invocation to the shared call log with `tool: "git"`, then answer from `TUDIFF_GIT_SCRIPT` when set: a JSON array of rules `{"match":[…],"stdout":"…","stderr":"…","exit":n}` where `match` is a prefix match on argv after stripping a leading `-C <dir>` pair; the first matching rule wins. With no script or no matching rule it MUST write nothing to stdout or stderr and exit 0. It MUST perform no filesystem or network operations beyond the call log, and MUST accept (not special-case or reject) every argv shape tu issues: `rebase --abort`, `add <user>/`, `status --porcelain <user>/`, `commit -m <msg>`, `pull --rebase origin main`, `push`, `rev-parse --git-dir`, `clone <url> <dir>`.

#### Scenario: Scripted and unscripted responses
- **GIVEN** `TUDIFF_GIT_SCRIPT='[{"match":["status","--porcelain"],"stdout":" M u/x\n","exit":0}]'`
- **WHEN** `git -C /tmp/m status --porcelain u/` runs
- **THEN** stdout is ` M u/x\n`, exit 0, and the call-log line has `argv` equal to `["-C","/tmp/m","status","--porcelain","u/"]`
- **GIVEN** no script
- **WHEN** `git -C /tmp/m pull --rebase origin main` runs
- **THEN** stdout and stderr are empty and exit is 0

### Requirement: Shared call log
Both fakes SHALL append one JSON line per invocation to the file named by `TUDIFF_CALL_LOG` when set (create if absent, append otherwise), via the shared `harness.LogCall`. Line shape: `{"tool":"ccusage"|"git","argv":[…],"cwd":"…","matched":"<alias>/<file>"}` — `matched` is present only for fake-ccusage hits. A missing or unwritable log path MUST NOT change a fake's exit code or output (best-effort, errors swallowed silently — the fake impersonates a tool whose stderr is being compared). The log lets the harness byte-compare the *sequence* of ccusage/git calls between the two binaries, not just their output.

#### Scenario: Append to existing log
- **GIVEN** `TUDIFF_CALL_LOG=/tmp/x/calls.jsonl` and an existing file with 2 lines
- **WHEN** the fake ccusage is invoked once
- **THEN** the file has 3 lines and the last decodes with `tool == "ccusage"` and a 3-element `argv`

### Requirement: TS-side staging of the fake ccusage
The TypeScript fetcher does NOT look `ccusage` up on `PATH` — it execs the fixed path `dist/vendor/ccusage/bin/ccusage` when `dist/vendor/` exists, else `node_modules/.bin/ccusage`. Staging the fake for the TS side therefore means copying the self-contained `bin/harness/ccusage` binary to `<staged-dist>/vendor/ccusage/bin/ccusage` (what `StageOracle` does into `<tmp>/oracle/dist/`); the Go side resolves vendor-first relative to `os.Executable()` then `PATH`. The fake git needs no staging — `PATH`-first placement suffices because both sides reach `git` through `PATH`.

## Design Decisions

### Fakes are separate env-configured Go binaries
**Decision**: `cmd/fakeccusage` and `cmd/fakegit`, built under the impersonated names into `bin/harness/`, configured only by `TUDIFF_*` env vars.
**Why**: Their argv belongs to tu; separate mains read better than an argv[0]-dispatch trick; a static binary can be copied into the TS side's fixed `dist/vendor/ccusage/bin/ccusage` slot, which a shell shim could not do portably.
**Rejected**: Busybox-style single binary dispatching on `os.Args[0]`; shell shims exec'ing `tudiff fake-…`.
*Introduced by*: 260915-r7dh-harness-fixture-capture
