---
type: memory
description: Differential-harness fixture corpus for the Go port — harness/fixtures/<alias>/<source>/<period>.json layout with per-alias manifest.json (schema v1), raw-bytes home-path redaction, tudiff capture/placeholder/run commands, fake ccusage/git replayers configured by TUDIFF_* env vars, the shared TUDIFF_CALL_LOG, the corpus validation test, and the TS-side vendor-path staging handoff
---
# Differential Harness

**Domain**: harness

## Overview

The differential harness replays recorded ccusage output and stubs git so the shipped TypeScript binary and the Go successor can be byte-diffed over a deterministic corpus (plan rows P3a/P4 in `fab/plans/sahil/26-09-15-go-port.md`). The corpus lives under `harness/fixtures/` at the repo root; the tooling is three stdlib-only Go commands (`src/go/cmd/tudiff`, `src/go/cmd/fakeccusage`, `src/go/cmd/fakegit`) sharing `src/go/internal/harness/` — the module's first `internal/` package. Two kinds of alias directory coexist there: **real captures** (`harness/fixtures/<machine>/`, written by `tudiff capture`) carry the capturing user's spend and are **gitignored — local-only, never committed**; the **committed corpus** is the schema-derived `_placeholder/` set covering all six sources. The harness (P4) points `TUDIFF_FIXTURES` at a local capture when one exists and at `_placeholder` otherwise. Build/test wiring lives in [toolchain](/build/toolchain.md); the upstream per-agent JSON shapes live in [data-pipeline](/cli/data-pipeline.md).

## Requirements

### Requirement: Fixture corpus layout
Fixtures SHALL live under `harness/fixtures/<machine-alias>/<source>/<period>.json`, with an optional sibling `<period>.stderr.txt` when the recorded stderr is non-empty, and exactly one `manifest.json` per alias directory. The placeholder corpus SHALL use the reserved alias `_placeholder`. `.gitignore` ignores every `harness/fixtures/*/` directory except `_placeholder/`, so a real capture can never be committed by accident. Re-running a capture for one alias SHALL overwrite only that alias's directory and MUST NOT read or modify any other alias directory.

#### Scenario: Capture writes per-alias files
- **GIVEN** a capture for machine `dev-ws-sahil02` of source `claude`, period `daily`
- **WHEN** it is written
- **THEN** the file is `harness/fixtures/dev-ws-sahil02/claude/daily.json` and `harness/fixtures/dev-ws-sahil02/manifest.json` has an entry with `"file": "claude/daily.json"`

### Requirement: Manifest schema v1
`manifest.json` MUST be produced from the Go structs in `src/go/internal/harness/manifest.go` (`const SchemaVersion = 1`) with top-level fields `schema`, `machine`, `captured_at` (RFC 3339 UTC), `ccusage_version` (from `<ccusage> --version`, which prints `ccusage 20.0.19`), `ccusage_path` (repo-relative when under the repo root), `platform` (`GOOS/GOARCH`), `timezone` (IANA — `$TZ` when set, else the system zone name with an `/etc/timezone` fallback), optional `derived_from` (placeholder corpus only, `omitempty`), and `fixtures[]`. Each fixture entry MUST carry `source`, `period`, `args` (string array), `file`, `stderr_file` (empty string when none), `exit_code`, `sha256` (hex of the committed file bytes), `days`, `first_date`, `last_date`, `empty`, `redactions`, `unconfirmed`. `source` is the **ccusage subcommand name** (`claude`, not tu's `cc`; `opencode`, not `oc`) because the replayer matches ccusage argv. `days`/`first_date`/`last_date`/`empty` are derived by parsing the recorded stdout as `{"daily":[{"date":…}]}`; stdout that is not parseable JSON (a failed source) yields `0`, `""`, `""`, `true` with `exit_code` carrying the failure. `unconfirmed: true` occurs only under `_placeholder/` — a real exit-0 empty capture is `empty: true, unconfirmed: false`, because an observed empty shape is a confirmed fixture. `sha256` pins the fixture bytes: a hand-edited fixture without a manifest regeneration fails the corpus test.

#### Scenario: Entry derived from recorded stdout
- **GIVEN** a recorded stdout that parses as `{"daily":[{"date":"2026-09-08"},…,{"date":"2026-09-16"}]}` with 9 entries
- **WHEN** the manifest entry is built
- **THEN** `days` is 9, `first_date` is `2026-09-08`, `last_date` is `2026-09-16`, `empty` is false

### Requirement: Home-rooted path redaction
`Redact(raw []byte, home string) ([]byte, int)` in `src/go/internal/harness/redact.go` MUST replace every absolute path rooted in `home` (when non-empty), `/home/<user>`, or `/Users/<user>` — including the remainder of the path up to a closing `"` or whitespace — with `~/redacted-<n>`, where `n` starts at 1 and is assigned in first-seen order per distinct original path within one call. It MUST operate on raw bytes via regexp, never by decoding and re-encoding JSON, and MUST NOT alter Claude's encoded project-directory form (`-home-sahil-code-…`, no leading slash), hostnames, model names, or numbers. The replacement count is recorded as `redactions` in the manifest.

#### Scenario: Redaction with stable first-seen numbering
- **GIVEN** `{"a":"/home/sahil/code/x","b":"/home/sahil/code/x","c":"/Users/bob/work/y","d":"-home-sahil-code-x","e":"gpt-5"}` and `home=/home/sahil`
- **WHEN** redacted
- **THEN** the output is `{"a":"~/redacted-1","b":"~/redacted-1","c":"~/redacted-2","d":"-home-sahil-code-x","e":"gpt-5"}` and the count is 3

### Requirement: tudiff capture
`src/go/cmd/tudiff/main.go` dispatches on the first argument (`capture`, `placeholder`, `run`) behind a testable `run(args, stdout, stderr) int` seam. `capture` accepts `--machine <alias>` (default `os.Hostname()`), `--ccusage <path>`, `--out <dir>` (default `harness/fixtures`), `--sources <a,b,…>` (default the six ccusage subcommands in tu's registry order: `claude,codex,opencode,gemini,copilot,kimi`), and `--periods <a,b,…>` (default `daily`). When `--ccusage` is omitted, the binary is resolved against the repo root (found by walking up to the first directory containing `package.json`) in order: `dist/vendor/ccusage/bin/ccusage` → `node_modules/@ccusage/ccusage-<platform>-<arch>/bin/ccusage` (Node's spelling — `runtime.GOOS` direct, `amd64`→`x64`, `arm64`→`arm64`) → `ccusage` on `PATH`; none found prints `tudiff: no ccusage binary found (run npm ci or pass --ccusage)` to stderr and exits 1. Each cell runs `<ccusage> <source> <period> --json` via `exec.CommandContext` with a 120 s timeout, cwd = repo root, inherited environment. A non-zero exit or timeout is **recorded, not fatal**: it lands in the manifest (`exit_code`, `empty: true`) and the run continues; the final summary names any non-zero cells. stdout passes through redaction and is written **verbatim** — no JSON decode/re-encode, so key order, indentation, and `-0.0` survive; non-empty stderr is written to the `<period>.stderr.txt` sidecar.

#### Scenario: Non-fatal source failure
- **GIVEN** a stub ccusage that prints `{"daily":[],"totals":{"totalCost":-0.0}}` for `opencode daily --json` and exits 3 for `kimi daily --json`
- **WHEN** `capture --ccusage <stub> --sources opencode,kimi` runs
- **THEN** the run exits 0, the opencode fixture contains the literal `-0.0`, the kimi manifest entry has `exit_code: 3` and `empty: true`, and the summary line names `kimi`

### Requirement: tudiff placeholder
`tudiff placeholder [--source <s>…] [--out <dir>]` (default: all six sources in registry order; default out `harness/fixtures/_placeholder`) SHALL write deterministic, schema-derived fixtures from Go structs mirroring the observed ccusage v20 per-agent daily shapes: **codex** in its observed outlier shape (`costUSD`, a `models{}` map keyed by model name with `isFallback`/`reasoningOutputTokens`, and `reasoningOutputTokens` on entries and totals — `PlaceholderCodex`), every other source in the claude-style shape (`totalCost`, `modelBreakdowns[]`, `modelsUsed[]` — `Placeholder`); `PlaceholderFor(source)` dispatches. Each fixture has three fixed consecutive days `2026-01-05..2026-01-07`, model name `placeholder-<source>-model`, alphabetically ordered keys, 2-space indentation, `totals` equal to the column sums, and `totalTokens` equal to the sum of the four token counters in every entry and in `totals` — internally consistent so tu's aggregation can be checked against it. The `_placeholder` manifest carries `machine: "_placeholder"`, `ccusage_version: "20.0.19"`, `ccusage_path: ""`, `platform: "derived"`, a provenance `derived_from` sentence naming the live probe the shapes came from, and `unconfirmed: true` on every entry. The generator refuses (exit 1) only an `--out` that already holds a real alias's manifest; a local capture alongside `_placeholder/` is expected and is never read or modified.

#### Scenario: Placeholder output is deterministic
- **GIVEN** `tudiff placeholder --source opencode --out <tmp>` run twice
- **WHEN** the two outputs are compared
- **THEN** they are byte-identical, decode into the schema structs, and `totals.totalTokens` equals the sum over `daily[].totalTokens`

### Requirement: tudiff run stub
`run` (the byte-diff matrix against the two tu binaries, plan row P4) exists only as a stub: it MUST print `tudiff: run is not implemented (plan row P4)` to stderr and exit 1. No argument, or an unknown subcommand, MUST print usage to stderr and exit 2.

#### Scenario: Stub invocation
- **GIVEN** `tudiff run`
- **WHEN** invoked
- **THEN** the exit code is 1 and stderr is exactly `tudiff: run is not implemented (plan row P4)\n`

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

### Requirement: Corpus validation test
`src/go/internal/harness/corpus_test.go` SHALL walk `../../../../harness/fixtures/*/manifest.json` (Go tests run with cwd = package dir — the committed `_placeholder/` plus any local capture present on the machine) and, for every fixture entry, assert: the file exists; its `sha256` matches; when `exit_code == 0` the content parses as JSON with a `daily` array whose length equals `days`, and `first_date`/`last_date`/`empty` agree with the content; no string value contains `/home/` or `/Users/`; and `unconfirmed: true` occurs only under the `_placeholder` alias. The test MUST fail (not skip) when the fixtures directory is absent. It runs in the existing `go-build-and-test` CI lane through `just go-test` — no CI workflow edit.

#### Scenario: Tampered fixture fails on sha256
- **GIVEN** the committed placeholder corpus
- **WHEN** one byte of a fixture is edited without regenerating the manifest
- **THEN** `just go-test` fails the corpus test on the `sha256` assertion

### Requirement: TS-side staging of the fake ccusage (P4 handoff)
The TypeScript fetcher does NOT look `ccusage` up on `PATH` — it execs the fixed path `dist/vendor/ccusage/bin/ccusage` when `dist/vendor/` exists, else `node_modules/.bin/ccusage`. Staging the fake for the TS side therefore means copying the self-contained `bin/harness/ccusage` binary to `<staged-dist>/vendor/ccusage/bin/ccusage`; the Go side resolves vendor-first relative to `os.Executable()` then `PATH`. The fake git needs no staging — `PATH`-first placement suffices because both sides reach `git` through `PATH`.

## Design Decisions

### Manifest is JSON, one per machine directory
**Decision**: `manifest.json` produced from Go structs, regenerated wholesale per alias.
**Why**: Zero-dependency posture (no YAML in stdlib); per-machine files let captures from other machines land without merge conflicts; structs make the schema code.
**Rejected**: A single repo-wide manifest (every machine's capture rewrites one file); YAML (needs a dependency).
*Introduced by*: 260915-r7dh-harness-fixture-capture

### Fixtures are verbatim bytes, redacted by regexp on the raw stream
**Decision**: stdout is never decoded and re-encoded before being written.
**Why**: Byte fidelity is the harness's purpose; `-0.0` totals and serializer key order are real traits a re-encode would erase.
**Rejected**: Redacting via JSON walk (loses formatting); redacting nothing (blocks committing `--instances`-style captures later).
*Introduced by*: 260915-r7dh-harness-fixture-capture

### Real captures are local-only; the committed corpus is placeholder-only
**Decision**: `harness/fixtures/<machine>/` is gitignored (only `_placeholder/` is tracked); `tudiff placeholder` generates all six sources by default and coexists with local captures; the replayer's ordered `TUDIFF_FIXTURES` list picks the local capture first when present.
**Why**: A real capture is the capturing user's daily spend, token counts, and model names, and `sahil87/tu` is public. Byte-fidelity for the harness comes from the serializer's *structure*, which the placeholders mirror and a local capture exercises for real on the developer's machine; the numbers themselves never need to be public.
**Rejected**: Committing real captures (published spend data); scrubbing numeric values in place (loses the "verbatim bytes" guarantee and adds a redaction surface that is easy to get wrong).
*Introduced by*: 260915-r7dh-harness-fixture-capture

### Real empty captures are confirmed; only `_placeholder/` is unconfirmed
**Decision**: An exit-0 `daily: []` capture is `empty: true, unconfirmed: false`; placeholders are the only `unconfirmed: true` entries.
**Why**: "We observed nothing" and "we guessed the shape" are different facts; the harness must report the second separately.
**Rejected**: Marking empties unconfirmed (conflates the two).
*Introduced by*: 260915-r7dh-harness-fixture-capture

### Placeholder shapes follow the observed per-agent serializer
**Decision**: codex placeholders use the codex shape (`costUSD`, `models{}`, `reasoningOutputTokens`); every other source uses the claude-style shape (`totalCost`, `modelBreakdowns[]`, `modelsUsed[]`).
**Why**: Both shapes were live-verified; the empty opencode/copilot outputs use `totalCost` in `totals`, so claude-style is the evidence-based guess for them, while codex is a known outlier that must be mirrored or the Go adapter's `costUSD` path goes untested in CI.
**Rejected**: One shape for all placeholders (contradicts observed output for one side or the other).
*Introduced by*: 260915-r7dh-harness-fixture-capture

### Fakes are separate env-configured Go binaries
**Decision**: `cmd/fakeccusage` and `cmd/fakegit`, built under the impersonated names into `bin/harness/`, configured only by `TUDIFF_*` env vars.
**Why**: Their argv belongs to tu; separate mains read better than an argv[0]-dispatch trick; a static binary can be copied into the TS side's fixed `dist/vendor/ccusage/bin/ccusage` slot, which a shell shim could not do portably.
**Rejected**: Busybox-style single binary dispatching on `os.Args[0]`; shell shims exec'ing `tudiff fake-…`.
*Introduced by*: 260915-r7dh-harness-fixture-capture
