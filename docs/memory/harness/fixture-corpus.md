---
type: memory
description: "Differential-harness fixture corpus — harness/fixtures/<alias>/<source>/<period>.json with a per-alias manifest.json (schema v1, sha256-pinned), home-rooted path redaction to ~/redacted-<n>, tudiff capture recording real ccusage output verbatim into a gitignored local alias, and the corpus validation test that fails on tampered or unredacted fixtures."
---
# Fixture Corpus

**Domain**: harness

## Overview

The fixture corpus is the recorded ccusage output the differential harness replays: real captures under `harness/fixtures/<machine>/` (gitignored) and the committed `_placeholder/` set ([placeholder-corpus](/harness/placeholder-corpus.md)), each alias described by a `manifest.json` produced from `src/go/internal/harness/manifest.go` and checked by `corpus_test.go`. The run driver that consumes them is in [differential-harness](/harness/differential-harness.md).

## Requirements

### Requirement: Fixture corpus layout
Fixtures SHALL live under `harness/fixtures/<machine-alias>/<source>/<period>.json`, with an optional sibling `<period>.stderr.txt` when the recorded stderr is non-empty, and exactly one `manifest.json` per alias directory. The placeholder corpus SHALL use the reserved alias `_placeholder`; its directory holds exactly one hand-written file, `confirmed.json` (the confirmation ledger — see *tudiff placeholder*), and everything else in it is generated. `.gitignore` ignores every `harness/fixtures/*/` directory except `_placeholder/`, so a real capture can never be committed by accident. Re-running a capture for one alias SHALL overwrite only that alias's directory and MUST NOT read or modify any other alias directory.

#### Scenario: Capture writes per-alias files
- **GIVEN** a capture for machine `dev-ws-sahil02` of source `claude`, period `daily`
- **WHEN** it is written
- **THEN** the file is `harness/fixtures/dev-ws-sahil02/claude/daily.json` and `harness/fixtures/dev-ws-sahil02/manifest.json` has an entry with `"file": "claude/daily.json"`

### Requirement: Manifest schema v1
`manifest.json` MUST be produced from the Go structs in `src/go/internal/harness/manifest.go` (`const SchemaVersion = 1`) with top-level fields `schema`, `machine`, `captured_at` (RFC 3339 UTC), `ccusage_version` (from `<ccusage> --version`, which prints `ccusage 20.0.19`), `ccusage_path` (repo-relative when under the repo root), `platform` (`GOOS/GOARCH`), `timezone` (IANA — `$TZ` when set, else the system zone name with an `/etc/timezone` fallback), optional `derived_from` (placeholder corpus only, `omitempty`), and `fixtures[]`. Each fixture entry MUST carry `source`, `period`, `args` (string array), `file`, `stderr_file` (empty string when none), `exit_code`, `sha256` (hex of the committed file bytes), `days`, `first_date`, `last_date`, `empty`, `redactions`, `unconfirmed`, and an optional `confirmed_by` object (`machine`, `date` as `YYYY-MM-DD`, `ccusage_version`; Go `*ConfirmedBy` with `omitempty`) that is present exactly when a `_placeholder` entry is `unconfirmed: false` and never on any other alias. `source` is the **ccusage subcommand name** (`claude`, not tu's `cc`; `opencode`, not `oc`) because the replayer matches ccusage argv. `days`/`first_date`/`last_date`/`empty` are derived by parsing the recorded stdout as `{"daily":[{"date":…}]}`; stdout that is not parseable JSON (a failed source) yields `0`, `""`, `""`, `true` with `exit_code` carrying the failure. `unconfirmed: true` occurs only under `_placeholder/` — a real exit-0 empty capture is `empty: true, unconfirmed: false`, because an observed empty shape is a confirmed fixture. `sha256` pins the fixture bytes: a hand-edited fixture without a manifest regeneration fails the corpus test.

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
`src/go/cmd/tudiff/main.go` dispatches on the first argument (`capture`, `placeholder`, `run`, `live`) behind a testable `run(args, stdout, stderr) int` seam; with no argument or an unknown subcommand it prints the usage block (`run  byte-diff bin/tu against harness/golden over harness/matrix.json`, `live  real-git parity: the sync/repair sequence against temp bare repos`) to stderr and exits 2. `capture` accepts `--machine <alias>` (default `os.Hostname()`), `--ccusage <path>`, `--out <dir>` (default `harness/fixtures`), `--sources <a,b,…>` (default the six ccusage subcommands in tu's registry order: `claude,codex,opencode,gemini,copilot,kimi`), and `--periods <a,b,…>` (default `daily`). When `--ccusage` is omitted, the binary is resolved against the repo root (found by `harness.FindRepoRoot`, walking up to the first directory containing `justfile`; none found prints `tudiff: no justfile found above <start>` and exits non-zero) in order: `dist/vendor/ccusage/bin/ccusage` under the repo root → the `vendor/ccusage/bin/ccusage` beside the `tu` found on `PATH` (symlinks resolved, as `ccusage.ResolveBinary` does — the brew install symlinks `bin/tu` into libexec) → bare `ccusage` on `PATH`; none found prints `tudiff: no ccusage binary found (pass --ccusage)` to stderr and exits 1. Each cell runs `<ccusage> <source> <period> --json` via `exec.CommandContext` with a 120 s timeout, cwd = repo root, inherited environment. A non-zero exit or timeout is **recorded, not fatal**: it lands in the manifest (`exit_code`, `empty: true`) and the run continues; the final summary names any non-zero cells. stdout passes through redaction and is written **verbatim** — no JSON decode/re-encode, so key order, indentation, and `-0.0` survive; non-empty stderr is written to the `<period>.stderr.txt` sidecar.

#### Scenario: Non-fatal source failure
- **GIVEN** a stub ccusage that prints `{"daily":[],"totals":{"totalCost":-0.0}}` for `opencode daily --json` and exits 3 for `kimi daily --json`
- **WHEN** `capture --ccusage <stub> --sources opencode,kimi` runs
- **THEN** the run exits 0, the opencode fixture contains the literal `-0.0`, the kimi manifest entry has `exit_code: 3` and `empty: true`, and the summary line names `kimi`

### Requirement: Corpus validation test
`src/go/internal/harness/corpus_test.go` SHALL walk `../../../../harness/fixtures/*/manifest.json` (Go tests run with cwd = package dir — the committed `_placeholder/` plus any local capture present on the machine) and, for every fixture entry, assert: the file exists; its `sha256` matches; when `exit_code == 0` the content parses as JSON with a `daily` array whose length equals `days`, and `first_date`/`last_date`/`empty` agree with the content; no string value contains `/home/` or `/Users/`; `unconfirmed: true` and `confirmed_by` occur only under the `_placeholder` alias; and, under `_placeholder`, the manifest agrees with `confirmed.json` — every listed source is `unconfirmed: false` with `confirmed_by` equal to its ledger object, every unlisted source is `unconfirmed: true` with none, and every ledger key has a `daily` fixture (a malformed ledger fails the test; each mismatch message ends `re-run tudiff placeholder`). The test MUST fail (not skip) when the fixtures directory is absent. It runs in the existing `go-build-and-test` CI lane through `just go-test` — no CI workflow edit.

#### Scenario: Tampered fixture fails on sha256
- **GIVEN** the committed placeholder corpus
- **WHEN** one byte of a fixture is edited without regenerating the manifest
- **THEN** `just go-test` fails the corpus test on the `sha256` assertion

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
