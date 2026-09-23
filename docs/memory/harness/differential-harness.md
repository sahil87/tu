---
type: memory
description: "Differential harness overview and the tudiff run driver — byte-diffs the frozen node dist/tu.mjs oracle against bin/tu over the matrix: flags and preflight exit-2 checks, pipe vs script(1) TTY execution with the exit sentinel, D7 fixture resolution and unconfirmed flagging, the date-rollover re-run and worker pool, the report.txt/report.json layout, and the exit-code gate rule."
---
# Differential Harness

**Domain**: harness

## Overview

The differential harness replays recorded ccusage output and stubs git so the frozen TypeScript oracle (`node dist/tu.mjs`, kept until plan row Z1) and the shipped Go binary (`bin/tu`) can be byte-diffed over a deterministic corpus (plan rows P3a/P4 in `fab/plans/sahil/26-09-15-go-port.md`). The corpus lives under `harness/fixtures/` at the repo root; the tooling is three stdlib-only Go commands (`src/go/cmd/tudiff`, `src/go/cmd/fakeccusage`, `src/go/cmd/fakegit`) sharing `src/go/internal/harness/`. Two kinds of alias directory coexist there: **real captures** (`harness/fixtures/<machine>/`, written by `tudiff capture`) carry the capturing user's spend and are **gitignored — local-only, never committed**; the **committed corpus** is the schema-derived `_placeholder/` set covering all six sources. `tudiff run` resolves `TUDIFF_FIXTURES` to the machine's local capture when one exists and to `_placeholder` otherwise (D7). Build/test wiring lives in [toolchain](/build/toolchain.md); the upstream per-agent JSON shapes live in [ccusage-json-shapes](/source/ccusage-json-shapes.md).

## Requirements

### Requirement: tudiff run
`runRun(args, stdout, stderr) int` in `src/go/cmd/tudiff/run.go` implements `run` — the byte-diff matrix against the two tu binaries — behind the same `run(args, stdout, stderr) int` dispatch seam as `capture`/`placeholder`, stdlib-only. Flags (`flag.NewFlagSet("run", flag.ContinueOnError)`): `--matrix` (default `harness/matrix.json`), `--expected` (default `harness/expected-diffs.json` — the DC-keyed intentional divergences; see *Expected-diffs file*), `--node` (`dist/tu.mjs` — the frozen TS oracle, kept until plan row Z1, run as `node <staged copy>`), `--go` (`bin/tu`), `--harness-bin` (`bin/harness`, the directory holding the fakes), `--fixtures <alias>[,<alias>…]` (explicit ordered alias list; overrides automatic resolution), `--placeholder` (force `_placeholder` only — what CI passes; mutually exclusive with `--fixtures`), `--report` (`bin/harness/report`; wiped at the start of a run), `--filter <substring>` (run only cases whose expanded ID contains it), `--list` (print the expanded, filtered case IDs one per line and exit 0 without touching the report dir), `--jobs <n>` (default 4; cases run concurrently, each fully isolated), `--timeout <dur>` (default 60s per side; a timeout is a red case). Relative flag *defaults* resolve against the repo root (`harness.FindRepoRoot`, walking up to `package.json`); explicit relative values resolve against the caller's cwd. Preflight checks run before any case and on the first failure print exactly one `tudiff: <reason>` line to stderr and exit 2, in order: `--fixtures` and `--placeholder` together; `--jobs < 1`; matrix unreadable or invalid (`tudiff: matrix: <loader error>`); the expected-diffs file missing (`tudiff: <path as given> not found`) or invalid (`tudiff: expected-diffs: <loader error>`) — loaded after the matrix check and after the `--list` early return, and a missing file is never treated as an empty set; `--node` missing (`… not found (run npm ci && npm run build)`); `--go` missing or not executable (`… (run just go-build)`); `--harness-bin` lacking an executable `ccusage` or `git` (`… (run just harness-build)`); `node` not on `PATH`; `script` not on `PATH` while the filtered matrix contains any `io: tty` case; `harness/fixtures/_placeholder/manifest.json` absent; a `--fixtures` alias with no `manifest.json`. `--list` runs only the matrix check. After the byte and tree comparisons, a red result is annotated with the matching expected-diffs entry's id (`Result.Expected`; green, timeout, and `harness`-channel results never carry one — a capture-level failure such as a failed exec or a missing exit sentinel is not a comparison divergence and must never be masked as expected). Exit codes: `1` iff the summary counts any unexpected red (`Red − Expected`), any timeout, any unconfirmed-fixture replay, or any stale expected entry (one that matched ≥ 1 executed case, none red) — an expected red case does not fail the run; `0` otherwise; `2` for usage/preflight errors — including a `--filter` that matches zero cases (`tudiff: --filter matched no cases`), never a vacuous 0. (489t) No argument or an unknown subcommand prints usage to stderr and exits 2.

#### Scenario: Missing Go binary is an actionable preflight error
- **GIVEN** `bin/tu` does not exist
- **WHEN** `tudiff run` is invoked
- **THEN** stderr is `tudiff: bin/tu not found (run just go-build)\n` and the exit code is 2

### Requirement: Pipe and TTY execution
`io: pipe` runs each side with stdin `/dev/null` and stdout/stderr captured into separate byte buffers, bounded by `--timeout` via `exec.CommandContext`; a deadline records `TimedOut` with exit −1. `io: tty` runs each side under `script(1)` with the wrapper `stty cols 120 rows 40; <cmd> <args…>; printf '\n__TUDIFF_EXIT=%s\n' "$?"` (arguments shell-quoted), invoked as `script -q -e -c "<wrapper>" /dev/null` on util-linux and `script -q /dev/null sh -c "<wrapper>"` on BSD; the flavour is detected once per run (`script --version` succeeding → util-linux) and recorded in the report header. The merged transcript — the pty's `\r\n` endings kept verbatim — is the single `tty` channel; the trailing `__TUDIFF_EXIT=<n>` sentinel (with its preceding line break) is parsed into the exit code and removed before comparison, and a missing or malformed sentinel yields exit −1 with the capture error `no exit sentinel`, never a false green.

#### Scenario: Sentinel parsing
- **GIVEN** a transcript ending `…table\r\n\r\n__TUDIFF_EXIT=2\r\n`
- **WHEN** parsed
- **THEN** the exit code is 2 and the `tty` channel ends with `…table\r\n`

### Requirement: Fixture resolution (D7) and unconfirmed flagging
`ResolveFixtures(root, explicit, placeholderOnly, hostname)` returns the ordered alias directories: an explicit `--fixtures` list wins (each alias must hold a `manifest.json`, else preflight exit 2); `--placeholder` forces `_placeholder` alone; otherwise `harness/fixtures/<hostname>/` precedes `_placeholder` when its manifest exists, else `_placeholder` alone. The resolved aliases are printed in the report header (`fixtures: dev-ws-sahil02, _placeholder` / `fixtures: _placeholder`). After each case, `UnconfirmedReplays` reads both sides' call logs and, for every line with a non-empty `matched` (`<alias>/<file>`), looks the file up in that alias's manifest: a case that replayed any `unconfirmed: true` fixture is flagged in the report as a separate marker (`[unconfirmed]` on the case line, counted in the summary), never a colour — the "harness reports them separately" clause of D7 — and any unconfirmed replay fails the run (exit 1) under the gate rule (489t).

### Requirement: Date-rollover re-run and case concurrency
Neither side takes a clock injection, and snapshot displays are "today". The driver records the local calendar date in the case's TZ immediately before the node side runs and immediately after the go side finishes; if they differ, both captures are discarded and the case re-runs exactly once (`rerun: true` in `report.json`), so a midnight rollover cannot produce a false red; a second mismatch is reported as-is. Cases execute through a worker pool of `--jobs` goroutines; each case owns its temp tree (both HOMEs, both working directories) and its report directories, so parallelism is safe, and report lines stream in matrix order regardless of completion order (a flush cursor prints contiguous completed results).

### Requirement: Report
The report lives under `--report` (default `bin/harness/report`, gitignored via `bin/`), wiped and recreated at the start of every non-`--list` run. `report.txt` — also streamed line-by-line to the harness's stdout — consists of a header (`tudiff run  <UTC RFC3339>`, `node: <path> (<node --version>)`, `go: <path> (<go --version> first line)`, `fixtures: <alias names>`, `script: util-linux|bsd|n/a`, `matrix: <path> (<N> cases[, filter "<f>"])`, `expected: <path as given> (<n> entries)`), one line per case in matrix order (`<STATUS padded to 7> <case ID>` plus, for red, `exit: node=<n> go=<n>` or `<channel> @<offset> (line <l>): node=<q> go=<q>`, for timeout `timeout: node=<bool> go=<bool>`, then ` [expected <DC-id>]` when a red case matched an expected-diffs entry, ` [unconfirmed]` and ` [calls differ: node=<n> go=<n>]` where applicable), and a summary block:

```
tudiff: <N> cases — <g> green, <r> red (<e> expected, <x> unexpected), <t> timeout   (fixtures: <aliases>; <u> cases replayed unconfirmed fixtures)
  by conf:  single <g>/<n>  multi <g>/<n>  org <g>/<n>  legacy <g>/<n>
  by env:   default <g>/<n>  nocolor <g>/<n>  envrepo <g>/<n>  pullfail <g>/<n>  pushfail <g>/<n>  dirty <g>/<n>
  by io:    pipe <g>/<n>  tty <g>/<n>
  by tz:    fixed <g>/<n>  alt <g>/<n>
```

When the expected-diffs file has one or more entries, one line per entry follows the axis lines: `  expected: <id>  <red>/<matched> red`, with ` (stale)` appended when the entry matched ≥ 1 executed case and none red, or ` (no executed case)` when it matched none (tallies run over the executed, post-`--filter` results — `live` and filtered runs legitimately exclude cases); an empty set prints no entry lines. The summary's first line carries the unexpected-red count — the zero-unexpected-diffs cutover precondition the port plan's gates read (489t). `report.json` carries the same data structured (`schema: 1`; `header` with `expected` (path as given) and `expected_entries`; `summary` with `total`/`green`/`red`/`timeout`/`unconfirmed`/`expected`/`unexpected`/`stale` (string array, `[]` never `null`); `cases[]` with id, group, args, the four axes, status, channel, offset, line, excerpts, exit codes, durations, `unconfirmed`, `calls_differ`, call counts, `rerun`, `expected` (the entry id or `""`)), 2-space indented with a trailing newline. `cases/<case ID as nested dirs>/` holds the per-side captures — `{node,go}.{stdout,stderr,exit}` for pipe cases, `{node,go}.{tty,exit}` for tty cases, plus the fakes' own `{node,go}.calls.jsonl` — with the byte channels written home-normalized (the exact bytes `Compare` compared, so `diff node.stdout go.stdout` shows what the harness saw; the `exit` files are unaffected), so a red can be inspected with `diff` without re-running. Temp staging lives under an `os.MkdirTemp` root removed at the end of the run; the report dir persists.

## Design Decisions

### No clock injection; re-run once on date rollover
**Decision**: Both sides run back-to-back under the real clock; a local-date change between them triggers a single re-run.
**Why**: Node has no fake clock without a dependency and the TS side is frozen (D4).
**Rejected**: `faketime`/`libfaketime` (platform-specific, absent on CI); accepting the rare midnight flake.
*Introduced by*: 260916-i9hc-harness-differential

### `stty` inside the pty, exit via sentinel
**Decision**: The `script` wrapper pins `cols 120 rows 40` and appends `__TUDIFF_EXIT=<n>`.
**Why**: An unsized pty reports 0 columns, which the TS `?? 80` fallback does not catch; util-linux and BSD `script` disagree on exit-code propagation, a sentinel is portable.
**Rejected**: `COLUMNS` env (spec says it is never read); util-linux `-e` alone (not on BSD).
*Introduced by*: 260916-i9hc-harness-differential
