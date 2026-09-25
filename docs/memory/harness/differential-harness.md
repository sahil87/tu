---
type: memory
description: "Differential harness overview and the tudiff run driver — byte-diffs bin/tu against the committed golden corpus (harness/golden/) over the matrix: --golden/--update/--now flags and the preflight exit-2 set, pipe vs script(1) TTY execution with the exit sentinel, the TUDIFF_NOW pinned clock, placeholder fixture resolution and unconfirmed flagging, the date-rollover re-run and worker pool, the report.txt/report.json layout, and the exit-code gate rule."
---
# Differential Harness

**Domain**: harness

## Overview

The differential harness replays recorded ccusage output and stubs git so the shipped Go binary (`bin/tu`) can be byte-diffed against the committed golden corpus ([golden-corpus](/harness/golden-corpus.md)) over a deterministic matrix (plan rows P3a/P4/Z1 in `fab/plans/sahil/26-09-15-go-port.md`). The fixture corpus lives under `harness/fixtures/` at the repo root; the tooling is three stdlib-only Go commands (`src/go/cmd/tudiff`, `src/go/cmd/fakeccusage`, `src/go/cmd/fakegit`) sharing `src/go/internal/harness/`. Two kinds of alias directory coexist there: **real captures** (`harness/fixtures/<machine>/`, written by `tudiff capture`) carry the capturing user's spend and are **gitignored — local-only, never committed**; the **committed corpus** is the schema-derived `_placeholder/` set covering all six sources. In golden mode `tudiff run` resolves `_placeholder` only — the goldens were captured from it, so a real-fixture run against them is meaningless; an explicit `--fixtures` list is accepted only by `--update` targeting a non-default `--golden` dir (D7's local-capture resolution survives inside `ResolveFixtures`, unused by `run`). Build/test wiring lives in [toolchain](/build/toolchain.md); the upstream per-agent JSON shapes live in [ccusage-json-shapes](/source/ccusage-json-shapes.md).

## Requirements

### Requirement: tudiff run
`runRun(args, stdout, stderr) int` in `src/go/cmd/tudiff/run.go` implements `run` — the byte-diff matrix of `bin/tu` against the golden corpus — behind the same `run(args, stdout, stderr) int` dispatch seam as `capture`/`placeholder`/`live`, stdlib-only. Flags (`flag.NewFlagSet("run", flag.ContinueOnError)`): `--matrix` (default `harness/matrix.json`), `--expected` (default `harness/expected-diffs.json` — the DC-keyed intentional divergences; see *Expected-diffs file* in [comparison-and-live](/harness/comparison-and-live.md)), `--go` (`bin/tu`), `--harness-bin` (`bin/harness`, the directory holding the fakes), `--golden` (default `harness/golden` — the corpus directory holding `manifest.json` plus `run/<case>/` captures), `--update` (rewrite the goldens from the Go side instead of comparing), `--now <ts>` (pin the golden clock, zone-less `2006-01-02T15:04:05`, for `--update`), `--fixtures <alias>[,<alias>…]` (only with `--update` and a non-default `--golden`), `--placeholder` (force `_placeholder` only — what CI passes; mutually exclusive with `--fixtures`), `--report` (`bin/harness/report`; wiped at the start of a run), `--filter <substring>` (run only cases whose expanded ID contains it), `--list` (print the expanded, filtered case IDs one per line and exit 0 without touching the report dir), `--jobs <n>` (default 4; cases run concurrently, each fully isolated), `--timeout <dur>` (default 60s per side; a timeout is a red case). Relative flag *defaults* resolve against the repo root (`harness.FindRepoRoot`, walking up to `justfile`); explicit relative values resolve against the caller's cwd. Preflight checks run before any case and on the first failure print exactly one `tudiff: <reason>` line to stderr and exit 2, in order: `--fixtures` and `--placeholder` together; `--fixtures` without `--update` plus a non-default `--golden` (`tudiff: --fixtures needs an oracle; goldens are placeholder-only`); `--jobs < 1`; matrix unreadable or invalid (`tudiff: matrix: <loader error>`); a `--filter` that matches zero cases (`tudiff: --filter matched no cases` — never a vacuous 0); `--list` returns after the matrix check; the expected-diffs file missing (`tudiff: <path as given> not found`) or invalid (`tudiff: expected-diffs: <loader error>`) — loaded after the `--list` early return, and a missing file is never treated as an empty set; the golden manifest missing (`tudiff: harness/golden/manifest.json not found (run tudiff run --update)`) or invalid; `matrix_sha256` differing from the current matrix (`tudiff: harness/matrix.json changed since the goldens were captured (run tudiff run --update and review the diff)`); an expanded (filtered) case with no golden dir (`tudiff: no golden for <case ID> (run tudiff run --update)`) — `--update` treats a missing manifest as the bootstrap case and skips the hash and per-case guards; `--go` missing or not executable (`… (run just go-build)`); `--harness-bin` lacking an executable `ccusage` or `git` (`… (run just harness-build)`); `script` not on `PATH` while the filtered matrix contains any `io: tty` case; `harness/fixtures/_placeholder/manifest.json` absent. Per case the driver stages **one** `$HOME`, runs the Go side with `TUDIFF_NOW=<manifest.now>` in its environment (`harness.BuildEnv`'s `EnvSpec.Now` field), then loads the golden channels into the oracle-position `SideCapture` (the golden side is the line-numbering reference) and reuses `Compare` plus the `tree.json` comparison ([comparison-and-live](/harness/comparison-and-live.md)). After the byte and tree comparisons, a red result is annotated with the matching expected-diffs entry's id (`Result.Expected`; green, timeout, and `harness`-channel results never carry one — a capture-level failure such as a failed exec or a missing exit sentinel is not a comparison divergence and must never be masked as expected). `--update` runs the Go side over the (filtered) matrix, writes `run/<case>/…` and `tree.json` per case, rewrites `manifest.json` (`oracle` as the `--go` path given, `oracle_version` probed from `<go> --version`, fresh `captured_at`/`matrix_sha256`/`cases`, `now` preserved from the existing manifest — or `--now`, or noon-of-today on a bootstrap), prints `tudiff: wrote <n> goldens under <dir>/run (now <now>)`, and exits 0. Exit codes: `1` iff the summary counts any unexpected red (`Red − Expected`), any timeout, any unconfirmed-fixture replay, or any stale expected entry (one that matched ≥ 1 executed case, none red) — an expected red case does not fail the run; `0` otherwise; `2` for usage/preflight errors. No argument or an unknown subcommand prints usage to stderr and exits 2. (489t, 6wpm)

#### Scenario: Missing Go binary is an actionable preflight error
- **GIVEN** `bin/tu` does not exist
- **WHEN** `tudiff run` is invoked
- **THEN** stderr is `tudiff: bin/tu not found (run just go-build)\n` and the exit code is 2

#### Scenario: All green against the committed corpus
- **GIVEN** the committed goldens and a `bin/tu` built from HEAD
- **WHEN** `just go-diff --placeholder` runs
- **THEN** the summary is `452 cases — 452 green, 0 red (0 expected, 0 unexpected), 0 timeout` and the exit code is 0, with no `node` binary on `PATH`

### Requirement: Pipe and TTY execution
`io: pipe` runs the side with stdin `/dev/null` and stdout/stderr captured into separate byte buffers, bounded by `--timeout` via `exec.CommandContext`; a deadline records `TimedOut` with exit −1. `io: tty` runs the side under `script(1)` with the wrapper `stty cols 120 rows 40; <cmd> <args…>; printf '\n__TUDIFF_EXIT=%s\n' "$?"` (arguments shell-quoted), invoked as `script -q -e -c "<wrapper>" /dev/null` on util-linux and `script -q /dev/null sh -c "<wrapper>"` on BSD; the flavour is detected once per run (`script --version` succeeding → util-linux) and recorded in the report header. The merged transcript — the pty's `\r\n` endings kept verbatim — is the single `tty` channel; the trailing `__TUDIFF_EXIT=<n>` sentinel (with its preceding line break) is parsed into the exit code and removed before comparison, and a missing or malformed sentinel yields exit −1 with the capture error `no exit sentinel`, never a false green.

#### Scenario: Sentinel parsing
- **GIVEN** a transcript ending `…table\r\n\r\n__TUDIFF_EXIT=2\r\n`
- **WHEN** parsed
- **THEN** the exit code is 2 and the `tty` channel ends with `…table\r\n`

### Requirement: The pinned clock (TUDIFF_NOW)
The Go side of every `run` and `live` case runs with `TUDIFF_NOW=<manifest.now>` in its environment — a zone-less local timestamp that one unexported `now()` function in `src/go/cmd/tu/main.go` parses with `time.ParseInLocation("2006-01-02T15:04:05", v, time.Local)`, falling back to `time.Now()` when the variable is unset or malformed. `now()` backs every edge use of the clock (the `Deps.Now`, `MetricsDirGuard`, `command.Normalize`, `config.Status`, `config.LastSync`, sync `Now`, and watch `Now` sites); the rain PRNG seed keeps `time.Now().UnixNano()` (never a compared surface). The variable is read at call time, never in `init()` or a package-level initializer; it appears nowhere in `--help`, `docs/specs/usage.md`, or `docs/site/skill.md`, and is never read under `internal/` — it is a harness seam like `TUDIFF_FIXTURES`/`TUDIFF_CALL_LOG`, which the fakes read. (6wpm)

#### Scenario: Same local date in both matrix zones
- **GIVEN** `TUDIFF_NOW=2026-09-26T12:00:00` and `TZ=Asia/Kolkata`
- **WHEN** `tu` runs a snapshot
- **THEN** `query.CurrentLabel` sees local date `2026-09-26`, and the same binary under `TZ=UTC` sees `2026-09-26` too

#### Scenario: Malformed value falls back silently
- **GIVEN** `TUDIFF_NOW=banana`
- **WHEN** `tu` runs
- **THEN** behaviour is identical to an unset variable (`time.Now`), with no error line

### Requirement: Fixture resolution and unconfirmed flagging
`ResolveFixtures(root, explicit, placeholderOnly, hostname)` returns the ordered alias directories: an explicit list wins (each alias must hold a `manifest.json`, else preflight exit 2); placeholder-only forces `_placeholder` alone; otherwise `harness/fixtures/<hostname>/` precedes `_placeholder` when its manifest exists, else `_placeholder` alone. `run` always resolves placeholder-only (the `--update --golden <other>` exception aside), and the report header prints the resolved aliases (`fixtures: _placeholder`). After each case, `UnconfirmedReplays` reads the Go side's call log and, for every line with a non-empty `matched` (`<alias>/<file>`), looks the file up in that alias's manifest: a case that replayed any `unconfirmed: true` fixture is flagged in the report as a separate marker (`[unconfirmed]` on the case line, counted in the summary), never a colour — the "harness reports them separately" clause of D7 — and any unconfirmed replay fails the run (exit 1) under the gate rule (489t). The Go call log is still written to `cases/<id>/go.calls.jsonl` in the report; the node-vs-go call-set comparison is gone with the oracle, and the log's remaining consumer is this gate (6wpm).

### Requirement: Date-rollover re-run and case concurrency
The Go side runs under the pinned clock, so the local calendar date cannot advance mid-case; the driver still records the local date in the case's TZ immediately before and after the side runs and, if they differ, discards the capture and re-runs exactly once (`rerun: true` in `report.json`) — the guard is cheap and stays. Cases execute through a worker pool of `--jobs` goroutines; each case owns its temp tree (its staged HOME and working directory) and its report directories, so parallelism is safe, and report lines stream in matrix order regardless of completion order (a flush cursor prints contiguous completed results).

### Requirement: Report
The report lives under `--report` (default `bin/harness/report`, gitignored via `bin/`), wiped and recreated at the start of every non-`--list` run: `report.txt` (header, per-case verdict lines, summary burndown) streamed to stdout as it renders, `report.json` (the same data structured, schema 1), and the per-case `go.*` capture files under `cases/` — the full shapes are in [report](/harness/report.md).

## Design Decisions

### TUDIFF_NOW pins the edge clock — one binary, one code path
**Decision**: One env read in `cmd/tu`'s `now()` — a zone-less local timestamp, harness-namespaced, set by `tudiff run`/`live` from `manifest.now` for the Go side; unset or malformed falls back to `time.Now`.
**Why**: Frozen goldens cannot follow the calendar — snapshot "today" labels, the default history window, leaderboard windows, `.last-sync` ages, and the sync commit message all derive from the edge clock; a seam in the shipped binary keeps the harness testing the exact `bin/tu` through one code path.
**Rejected**: A `//go:build harness` tagged binary (two build paths; the harness stops testing the shipped artifact); libfaketime (absent on CI); normalising date labels in captures (the default history window changes which rows exist as months pass).
*Introduced by*: 260925-6wpm-remove-src-node

### `stty` inside the pty, exit via sentinel
**Decision**: The `script` wrapper pins `cols 120 rows 40` and appends `__TUDIFF_EXIT=<n>`.
**Why**: An unsized pty reports 0 columns and the goldens were captured at 120×40, so the wrapper pins the size the corpus holds; util-linux and BSD `script` disagree on exit-code propagation, a sentinel is portable.
**Rejected**: `COLUMNS` env (spec says it is never read); util-linux `-e` alone (not on BSD).
*Introduced by*: 260916-i9hc-harness-differential

### The golden side sits in the oracle position
**Decision**: Golden channels are loaded into a `SideCapture` that takes the oracle position in `Compare`, and the report's `golden=`/`go=` detail names keep the comparison reading "expected vs actual".
**Why**: The corpus is the expected side — it is the line-numbering reference and its bytes are already fully normalised at capture, so loading it as a Home-less capture reuses the existing comparison machinery untouched.
**Rejected**: A golden-specific comparator (duplicates `Compare`/`firstDivergence`); swapping the positions (the line numbers would follow the Go capture, not the corpus being diffed against).
*Introduced by*: 260925-6wpm-remove-src-node
