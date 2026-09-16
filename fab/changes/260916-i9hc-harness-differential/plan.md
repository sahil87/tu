# Plan: Harness Differential (`tudiff run`)

**Change**: 260916-i9hc-harness-differential
**Intake**: `intake.md`

## Requirements

> Plan row P4 of `fab/plans/sahil/26-09-15-go-port.md`. Everything below is verification tooling around `src/go/cmd/tu`; no external tu surface changes (plan Goal), no `src/node/` file changes (D4). All Go code is stdlib-only (no `go.sum` yet) and lives in `src/go/cmd/tudiff` and `src/go/internal/harness`.

### Harness: `tudiff run` command surface

#### R1: `run` subcommand, flags and defaults
`src/go/cmd/tudiff/main.go` SHALL dispatch `run` to `runRun(args, stdout, stderr) int` in a new `src/go/cmd/tudiff/run.go`, replacing the P3a stub branch, and the `usageText` line for `run` SHALL read `run          byte-diff node dist/tu.mjs against the Go binary over harness/matrix.json`. `runRun` SHALL accept exactly these flags via `flag.NewFlagSet("run", flag.ContinueOnError)`: `--matrix` (default `harness/matrix.json`), `--node` (`dist/tu.mjs`), `--go` (`bin/tu`), `--harness-bin` (`bin/harness`), `--fixtures <alias>[,<alias>…]` (default empty = automatic D7 resolution), `--placeholder` (bool), `--report` (`bin/harness/report`), `--filter` (substring, default empty), `--list` (bool), `--jobs` (int, default 4), `--timeout` (duration, default `60s`). Every relative path default MUST resolve against the repo root returned by `harness.FindRepoRoot(cwd)`; an explicit relative flag value resolves against the caller's cwd.

- **GIVEN** cwd is `src/go/` inside the checkout
- **WHEN** `tudiff run --list` is invoked with no other flags
- **THEN** the matrix read is `<repo root>/harness/matrix.json` and the command exits 0

#### R2: Preflight errors are actionable and exit 2
Before any case runs, `runRun` MUST check, in order, and on the first failure print exactly one line `tudiff: <reason>` to stderr and return 2: `--fixtures` and `--placeholder` both given (`--fixtures and --placeholder are mutually exclusive`); matrix unreadable or invalid (the loader's error, prefixed `tudiff: matrix: `); `--node` missing (`… not found (run npm ci && npm run build)`); `--go` missing or not executable (`… not found (run just go-build)`); `--harness-bin` lacking an executable `ccusage` or `git` (`… (run just harness-build)`); `node` not on `PATH`; `script` not on `PATH` while the expanded, filtered matrix contains at least one `io: tty` case; `harness/fixtures/_placeholder/manifest.json` absent; a `--fixtures` alias with no `manifest.json`. `--list` MUST run only the matrix check and then print the expanded, filtered case IDs (one per line, matrix order) and return 0 without touching `--report`.

- **GIVEN** `bin/tu` does not exist
- **WHEN** `tudiff run` is invoked
- **THEN** stderr is `tudiff: bin/tu not found (run just go-build)\n` (path as given) and the exit code is 2

#### R3: Exit codes of `tudiff run`
`runRun` SHALL return `0` when every executed case is green, `1` when at least one executed case is red or timed out, and `2` for preflight/usage errors. A `--filter` that matches zero cases is a usage error (`tudiff: --filter matched no cases`, exit 2), never a vacuous 0.

- **GIVEN** a matrix whose only group is `version` with `args: ["--version"]` and both binaries print `tu version v0.11.5`
- **WHEN** `tudiff run` executes it
- **THEN** the exit code is 0
- **GIVEN** the committed matrix against the P4-state Go binary
- **WHEN** `tudiff run` executes it
- **THEN** the exit code is 1 and the summary line counts the `--version` cases green

### Harness: matrix

#### R4: Matrix schema, validation and expansion
`src/go/internal/harness/matrix.go` SHALL define `Matrix{Schema int; Cases []CaseGroup}` and `CaseGroup{ID string; Args []string; Conf, Env, IO, TZ []string}` decoded from JSON with `json.Decoder.DisallowUnknownFields`. `LoadMatrix(path)` MUST reject: `schema != 1`; empty or duplicate `id`; an `id` that fails `pathComponent`; a missing `args` key (an empty array is valid); any axis value outside `conf ∈ {single,multi,org,legacy}`, `env ∈ {default,nocolor,envrepo}`, `io ∈ {pipe,tty}`, `tz ∈ {fixed,alt}`; a duplicate value within one axis. Errors MUST name the offending group id and field. `Expand(m) []Case` SHALL produce, in matrix order and per group in the nested order conf → env → io → tz, one `Case{ID, Group, Args, Conf, Env, IO, TZ}` per combination, using the base value (`single`, `default`, `pipe`, `fixed`) for every omitted axis, with `Case.ID = "<id>/<conf>/<env>/<io>/<tz>"`. `Filter(cases, substr)` SHALL keep cases whose `ID` contains `substr` (empty keeps all).

- **GIVEN** a group `{"id":"x","args":[],"conf":["single","multi"],"io":["pipe","tty"]}`
- **WHEN** expanded
- **THEN** the IDs are exactly, in order, `x/single/default/pipe/fixed`, `x/single/default/tty/fixed`, `x/multi/default/pipe/fixed`, `x/multi/default/tty/fixed`
- **GIVEN** a group with `"env": ["loud"]`
- **WHEN** loaded
- **THEN** the error message contains the group id and `env` and the value `loud`

#### R5: The committed initial matrix
`harness/matrix.json` SHALL be committed, load without error, and expand to between 200 and 400 cases. It MUST contain: a `version` group (`["--version"]`) and a `version-short` group (`["-v"]`) with base axes; toolkit groups for `--help`, `help`, `help-dump`, `skill`, `shell-init bash|zsh|fish`, `shell-init` (no shell) and `shell-init tcsh`; snapshot groups for `[]` and `cc` with all four `conf` values, all three `env` values, both `io` values and both `tz` values; snapshot groups for every other source token (`codex`, `co`, `oc`, `gemini`, `gem`, `copilot`, `cop`, `kimi`, `ki`) plus `w`, `m`, `cc m` with `conf: [single, multi]` and both `io` values (trimmed from four conf values to keep the total inside the 200–400 bound; G0 may restore); format/flag variants of `[]` and `cc` (`--json`, `-j`, `--csv`, `--md`, `--by-machine`, `-t`, `--metric tokens`, `--metric cost`, `--fresh`) with `conf: [single, multi]`; history groups for `h`, `dh`, `wh`, `mh`, `cc h`, `cc mh`, `history` each in default-window, `--since 2026-01-01 --until 2026-01-31` (both `tz`) and `--full` forms with `conf: [single, multi]`, plus `--json`/`--csv`/`--md` on `h` and `cc mh`, and `--by-machine` on `cc h` and `h`; leaderboard groups `lb`, `m lb`, `cc m lb`, `lbh`, `w lbh`, `lb --top 1`, `lb --top 0`, `lb -u other-user`, `lb -u all`, `lb -u harness-user`, `lb --json`, `lbh --md` with all four `conf` values; multi-flag groups `-u other-user`, `-u all`, `-u` (missing value), `--by-machine -u all`, `--sync`, `cc --sync` with `conf: [single, multi]`; setup groups `status`, `init-conf`, `init-metrics`, `init-metrics git@example.invalid:harness/tu-metrics.git`, `init-metrics a b`, `sync`, `sync --dry-run` with all four `conf` values; and usage-error groups `bogus`, `cc codex`, `cc --help`, `--json --csv`, `--watch --json`, `--dry-run`, `cc --dry-run`, `--top x`, `--metric`, `--metric foo`, `-t --metric cost`, `--since 2026-13-01`, `--since 2026-02-01 --until 2026-01-01`, `--interval 3`, `--interval abc`, `--until`. It MUST NOT contain any group whose args include `--watch`/`-w` other than the `--watch --json` incompatibility case, nor any `update` group. Every axis value MUST appear in at least one group.

- **GIVEN** the committed `harness/matrix.json`
- **WHEN** `matrix_test.go` loads and expands it
- **THEN** no error, `200 ≤ len(cases) ≤ 400`, `version/single/default/pipe/fixed` is present, and each of the twelve axis values occurs at least once

### Harness: temp `$HOME` and seed data

#### R6: The four `$HOME` skeletons, two per case
`src/go/internal/harness/homes.go` SHALL expose `StageHome(dir, variant, seedDir string) error` creating `dir` as a fresh `$HOME` for `variant ∈ {single, multi, org, legacy}`: `single` creates the directory and nothing inside it; `multi` writes `.config/tu/tu.conf`; `org` writes `.config/tu/org.conf` and no `tu.conf`; `legacy` writes `.tu.conf` and no `.config/` directory. The three conf files MUST be byte-identical and exactly:

```
version = 2
metrics_repo = git@example.invalid:harness/tu-metrics.git
metrics_dir = ~/.tu/metrics_repo
machine = harness-machine
user = harness-user
auto_sync = true
```

For `multi`/`org`/`legacy` the seed tree (R7) MUST be copied recursively to `<dir>/.tu/metrics_repo/`; no `.git/` is created. The driver MUST stage two independent HOMEs per case — `<case>/node/home` and `<case>/go/home` — never sharing a path.

- **GIVEN** variant `legacy`
- **WHEN** staged into an empty dir
- **THEN** `<dir>/.tu.conf` exists with the bytes above, `<dir>/.config` does not exist, and `<dir>/.tu/metrics_repo/harness-user/2026/harness-machine/cc-2026-01-06.jsonl` exists

#### R7: Seeded metrics repo
`harness/metrics-repo/` SHALL be committed with exactly these files, each `.jsonl` holding one JSON line in `UsageEntry` shape (`label`, `totalCost`, `inputTokens`, `outputTokens`, `cacheCreationTokens`, `cacheReadTokens`, `totalTokens`) with `label` equal to the date in the filename:

| File | totalCost | Purpose |
|------|-----------|---------|
| `harness-user/2026/harness-machine/cc-2026-01-05.jsonl` | `0.25` | own machine, lower than the fixture's `0.5` → live wins (max-merge) |
| `harness-user/2026/harness-machine/cc-2026-01-06.jsonl` | `0.75` | own machine, higher than the fixture → stored wins; never-shrink skips the live write |
| `harness-user/2026/other-box/cc-2026-01-06.jsonl` | `0.40` | own user, other machine |
| `harness-user/2026/other-box/codex-2026-01-07.jsonl` | `0.30` | second tool on the other machine |
| `other-user/2026/laptop/cc-2026-01-05.jsonl` | `1.10` | second profile → `lb`/`lbh` have two rows |
| `other-user/2026/laptop/gemini-2026-01-06.jsonl` | `0.20` | second tool for the second profile |
| `docs/README.md` | — | one paragraph stating the tree's purpose; the spec excludes `docs/` from user scanning |

Token counters use the placeholder constants (`inputTokens` 3000, `outputTokens` 400, `cacheCreationTokens` 1000, `cacheReadTokens` 20000, `totalTokens` 24400).

- **GIVEN** the committed seed
- **WHEN** `homes_test.go` walks it
- **THEN** every `.jsonl` decodes into a struct with exactly those seven fields and `label` equals the filename's date

### Harness: execution

#### R8: Oracle staging
Once per run the driver SHALL create `<tmp>/oracle/dist/` and copy `--node` to `<tmp>/oracle/dist/tu.mjs`, `<repo root>/tu.default.conf` to `<tmp>/oracle/dist/tu.default.conf`, and `<harness-bin>/ccusage` to `<tmp>/oracle/dist/vendor/ccusage/bin/ccusage` with mode `0755`. The repository's `dist/` directory MUST NOT be written to. The TS side of every case runs `node <tmp>/oracle/dist/tu.mjs <args>`; the Go side runs `--go` in place.

- **GIVEN** a run
- **WHEN** it finishes
- **THEN** `dist/` has the same file set and mtimes as before, and `<tmp>/oracle/dist/vendor/ccusage/bin/ccusage` was byte-identical to `bin/harness/ccusage`

#### R9: Child environment and working directory
`BuildEnv(side Side, c Case, …) []string` SHALL return an environment containing exactly: `PATH=<abs harness-bin>:<harness process's PATH>`, `HOME=<side's staged home>`, `TZ=UTC` (`fixed`) or `TZ=Asia/Kolkata` (`alt`), `LANG=C.UTF-8`, `LC_ALL=C.UTF-8`, `TUDIFF_FIXTURES=<resolved alias dirs, absolute, OS path list>`, `TUDIFF_CALL_LOG=<report>/cases/<case path>/<side>.calls.jsonl`; plus `TERM=xterm-256color` only for `io: tty`; plus `NO_COLOR=1` only for `env: nocolor`; plus `TU_METRICS_REPO=git@example.invalid:harness/tu-metrics.git` only for `env: envrepo`. No other variable from the parent environment MAY be present. Each side's working directory SHALL be `<case>/<side>/` (a sibling of its `home`), never the repo root.

- **GIVEN** the harness process has `TU_METRICS_REPO=x` and `NO_COLOR=1` exported
- **WHEN** `BuildEnv` is called for a `default` env case
- **THEN** neither `TU_METRICS_REPO` nor `NO_COLOR` appears in the result

#### R10: Pipe execution
For `io: pipe` the driver SHALL run each side with stdin `/dev/null`, stdout and stderr captured into separate byte buffers, bounded by `--timeout` via `exec.CommandContext`, recording `Capture{Stdout, Stderr, Exit, TimedOut, Duration}`. A context deadline sets `TimedOut = true` and `Exit = -1`.

- **GIVEN** a stand-in `--go` script that sleeps 5 s and `--timeout 1s`
- **WHEN** the case runs
- **THEN** the Go capture has `TimedOut == true`, `Exit == -1`, and the case status is `timeout`

#### R11: TTY execution via `script`
For `io: tty` the driver SHALL run each side under `script` with the wrapper command `stty cols 120 rows 40; <cmd> <args…>; printf '\n__TUDIFF_EXIT=%s\n' "$?"` (arguments shell-quoted), as `script -q -e -c "<wrapper>" /dev/null` when `script --version` succeeds (util-linux) and `script -q /dev/null sh -c "<wrapper>"` otherwise (BSD). The transcript MUST be captured as `Capture.TTY`; the trailing sentinel line `__TUDIFF_EXIT=<n>` (with its preceding `\r?\n`) MUST be parsed into `Capture.Exit` and removed from `TTY`; a missing sentinel MUST set `Exit = -1` and `Capture.Err = "no exit sentinel"`. The detected flavour (`util-linux` or `bsd`) SHALL be recorded once per run in the report header.

- **GIVEN** a transcript ending `…table\r\n\r\n__TUDIFF_EXIT=2\r\n`
- **WHEN** parsed
- **THEN** `Exit == 2` and `TTY` ends with `…table\r\n`

### Harness: fixtures, comparison, report

#### R12: Fixture alias resolution (D7) and unconfirmed flagging
`src/go/internal/harness/fixtures.go` SHALL expose `ResolveFixtures(root string, explicit []string, placeholderOnly bool, hostname string) ([]string, error)` returning absolute alias directories: `explicit` when non-empty (each must contain `manifest.json`); `[_placeholder]` when `placeholderOnly`; otherwise `[<root>/harness/fixtures/<hostname>, <root>/harness/fixtures/_placeholder]` when the hostname dir has a manifest, else `[<root>/harness/fixtures/_placeholder]`. `UnconfirmedReplays(callLogPath string, aliases []string) (bool, error)` SHALL read the JSON-lines call log, and for every line with a non-empty `matched` (`<alias>/<file>`) look the file up in that alias's manifest and return true if any matched entry has `unconfirmed: true`.

- **GIVEN** a fixture root with only `_placeholder/`
- **WHEN** resolved with hostname `nohost`
- **THEN** the result is exactly `[<root>/harness/fixtures/_placeholder]`
- **GIVEN** a call log with `"matched":"_placeholder/claude/daily.json"`
- **WHEN** checked
- **THEN** the case is flagged unconfirmed

#### R13: Comparison and first divergence
`Compare(c Case, node, go Capture) Result` SHALL compare channels byte-for-byte in this order and stop at the first difference: `pipe` → `exit`, `stdout`, `stderr`; `tty` → `exit`, `tty`. If either side `TimedOut`, the status is `timeout` (channel `timeout`). Otherwise the status is `green` when all compared channels are identical and `red` with `Result.Channel` naming the first differing one; for a byte channel `Result.Offset` is the first differing byte index, `Result.Line` the 1-based line containing it (counting `\n` in the node capture up to `Offset`), and `Result.NodeExcerpt`/`GoExcerpt` are `strconv.Quote` of at most 40 bytes of each side starting at `Offset`; for `exit` the two codes are recorded. The call logs SHALL be compared as sorted sets of `tool + "\x00" + strings.Join(argv, "\x00")` (ignoring `cwd` and `matched`) into `Result.CallsDiffer bool` and `Result.NodeCalls`/`GoCalls int`; `CallsDiffer` MUST NOT affect the status.

- **GIVEN** node stdout `"Usage: tu\n"` and go stdout `""`, equal exit codes
- **WHEN** compared
- **THEN** status `red`, channel `stdout`, offset 0, line 1, node excerpt `"Usage: tu\n"`, go excerpt `""`
- **GIVEN** identical captures but call logs of 6 and 0 lines
- **WHEN** compared
- **THEN** status `green` and `CallsDiffer == true`

#### R14: Report
`src/go/internal/harness/report.go` SHALL render and write three things under `--report` (which is removed and recreated at the start of every non-`--list` run): `report.txt`, `report.json`, and `cases/<case ID as nested dirs>/{node,go}.{stdout,stderr,tty,exit,calls.jsonl}` (only the files that apply to the case's `io`). `report.txt` — also streamed line-by-line to the harness stdout as cases complete, then the summary — SHALL consist of: a header (`tudiff run  <UTC RFC3339>`, `node: <path> (<node --version>)`, `go: <path> (<go --version> output)`, `fixtures: <alias names, comma-separated>`, `script: util-linux|bsd|n/a`, `matrix: <path> (<N> cases[, filter "<f>"])`); one line per case in matrix order formatted `<STATUS padded to 7> <case ID>` followed, for red, by two spaces and either `exit: node=<n> go=<n>` or `<channel> @<offset> (line <l>): node=<q> go=<q>`, for timeout by `  timeout: node=<bool> go=<bool>`, then ` [unconfirmed]` when flagged and ` [calls differ: node=<n> go=<n>]` when `CallsDiffer`; and a summary block:

```
tudiff: <N> cases — <g> green, <r> red, <t> timeout   (fixtures: <aliases>; <u> cases replayed unconfirmed fixtures)
  by conf:  single <g>/<n>  multi <g>/<n>  org <g>/<n>  legacy <g>/<n>
  by env:   default <g>/<n>  nocolor <g>/<n>  envrepo <g>/<n>
  by io:    pipe <g>/<n>  tty <g>/<n>
  by tz:    fixed <g>/<n>  alt <g>/<n>
```

`report.json` SHALL carry `{"schema":1, "header":{…same fields…}, "summary":{"total","green","red","timeout","unconfirmed"}, "cases":[{"id","group","args","conf","env","io","tz","status","channel","offset","line","node_excerpt","go_excerpt","node_exit","go_exit","node_ms","go_ms","unconfirmed","calls_differ","node_calls","go_calls"}]}`, 2-space indented, trailing newline.

- **GIVEN** a run of three cases where one is red on stdout
- **WHEN** it completes
- **THEN** `report.txt` has the header, three case lines, and a summary whose first line reads `tudiff: 3 cases — 2 green, 1 red, 0 timeout …`, and `report.json` decodes with `summary.red == 1`

#### R15: Date-rollover re-run
The driver SHALL compute the local calendar date in the case's `TZ` immediately before the node side runs and immediately after the go side finishes; if they differ, it MUST discard both captures and re-run the case exactly once, recording `Result.Rerun = true` (surfaced in `report.json`). A second mismatch is reported as-is.

- **GIVEN** a clock helper injected to return `2026-09-16` then `2026-09-17` then `2026-09-17` twice
- **WHEN** one case runs
- **THEN** both sides executed twice and `Rerun == true`

#### R16: Concurrency and isolation
Cases SHALL execute through a worker pool of `--jobs` goroutines; each case owns `<tmp>/cases/<safe id>/` (both homes, both cwds) and its call-log paths under the report dir; results are collected and rendered in matrix order regardless of completion order. `--jobs < 1` is a usage error (exit 2). Temp dirs (under `os.MkdirTemp`) are removed at the end of the run; the report dir persists.

- **GIVEN** `--jobs 4` and a matrix of 20 cases
- **WHEN** the run completes
- **THEN** `report.txt` lists the 20 cases in matrix order and no `<tmp>` path remains

### Build: wiring

#### R17: `just go-diff`
`justfile` SHALL gain, after `harness-capture`, the recipe `go-diff *ARGS: build go-build harness-build` running `bin/harness/tudiff run {{ARGS}}`, with a two-line comment naming plan row P4 and stating that exit 1 is expected while any case is red.

- **GIVEN** `npm ci` has run and `node` is on PATH
- **WHEN** `just go-diff --placeholder` runs
- **THEN** `dist/tu.mjs`, `bin/tu`, and the three harness binaries are built first, a report lands in `bin/harness/report/`, and the recipe exits 1 (the P4 red baseline)

#### R18: CI job `go-diff`
`.github/workflows/ci.yml` SHALL gain a job `go-diff` (`ubuntu-latest`) after `go-build-and-test` with steps: checkout (same pinned SHA), `actions/setup-node` (same SHA, node 20), `actions/setup-go` (same SHA, `go-version-file: src/go/go.mod`, `cache: false`), `extractions/setup-just` (same SHA), `npm ci`, a step `Differential harness (placeholder matrix)` running `just go-diff --placeholder` with `continue-on-error: true`, and `actions/upload-artifact@ea165f8d65b6e75b540449e92b4886f43607fa02 # v4` with `if: always()`, `name: tudiff-report`, `path: bin/harness/report/`. The job comment MUST state that every case is red until Phase 1 lands and that plan row R3 removes `continue-on-error` and adds the job to `ci-gate`. `ci-gate`'s `needs` list MUST remain `[build-and-test, go-build-and-test]`.

- **GIVEN** the edited workflow
- **WHEN** parsed as YAML
- **THEN** `jobs.go-diff` exists with the steps above, `jobs.ci-gate.needs` equals `[build-and-test, go-build-and-test]`, and `jobs.go-diff.steps[*].continue-on-error` is true only on the harness step

### Harness: tests

#### R19: Test coverage
`_test.go` siblings SHALL cover: matrix load/validate/expand/filter incl. the committed file (R4, R5); home staging for all four variants and the seed's shape (R6, R7); env construction incl. the leak test (R9); sentinel parsing (R11); comparison for green, exit-first, stdout offset/line/excerpts, timeout, sorted calls (R13); report text and JSON rendering (R14); rerun logic via an injected clock (R15); fixture resolution and unconfirmed flagging (R12); and in `src/go/cmd/tudiff/run_test.go` the preflight exit-2 messages (R2), `--list` (R2), `--filter` no-match (R3), and an end-to-end smoke using two stand-in executables written by the test (shell scripts under `t.TempDir()` printing fixed stdout/stderr/exit) passed as `--node`/`--go` together with a temp matrix, a temp fixture root holding a minimal `_placeholder` manifest, and a stub `--harness-bin` — asserting one green and one red case in `report.json` and the exit code 1. The stand-in for `--node` MUST be invoked through a `node` shim on the test's PATH so the smoke does not require Node. `TestRunStub` in `main_test.go` MUST be removed. `just go-lint` and `just go-test` MUST pass.

- **GIVEN** the completed change
- **WHEN** `cd src/go && go test ./... -count=1 && gofmt -l . && go vet ./...` run
- **THEN** all pass and `gofmt -l .` prints nothing

#### R20: Live baseline recorded
The apply agent SHALL run the real harness once (`npm ci`, then `just go-diff` — with `--placeholder` if no local capture exists) and record the summary line and the number of green cases in this plan's `## Notes` as the P4 burndown baseline. Both `version` groups MUST be green in that run and every other group red. `src/go/cmd/tu/main.go` MUST NOT be modified.

- **GIVEN** the finished change on the reference machine
- **WHEN** `just go-diff` runs
- **THEN** exactly the `version/…` and `version-short/…` cases are green and the summary line is recorded under `## Notes`

### Non-Goals

- Implementing any tu behavior in Go — `cmd/tu` stays the P2 placeholder; the red baseline is the deliverable.
- `--watch` frame capture (B7), live-sync parity against a real bare repo (B6, D11), a fuller fixture metrics repo (B3), gating CI on the harness (R3).
- Making the call-log comparison red-making; it stays informational until V1.
- Cross-platform pty handling beyond `script` (no `creack/pty`, no raw `/dev/ptmx`).

### Design Decisions

#### JSON case groups with axis defaults, not a cross-product
**Decision**: `harness/matrix.json` lists case groups; omitted axes take one base value.
**Why**: A full product of 4×3×2×2 over ~90 argv lines is thousands of cases; explicit axes keep CI to minutes and make G0's coverage review a read of one file.
**Rejected**: A line-oriented text file (no per-line axis control without inventing syntax); a full cross-product with a `--sample` flag (nondeterministic coverage).
*Introduced by*: 260916-i9hc-harness-differential

#### Two fresh `$HOME`s per case
**Decision**: Each side gets its own staged skeleton.
**Why**: The TS side writes `~/.tu/cache` and multi-mode day-files; a shared HOME would let the Go side read TS state and mask divergences or alter its call sequence.
**Rejected**: One HOME with `--fresh` on every case (changes the matrix's meaning); running Go first (same problem mirrored).
*Introduced by*: 260916-i9hc-harness-differential

#### Call log is informational
**Decision**: `calls` differences are printed but never redden a case.
**Why**: The TS fetcher issues its per-tool ccusage calls via `Promise.all`; order is nondeterministic and a sorted comparison still can't distinguish "Go hasn't fetched yet" from "Go fetched differently" while every case is red.
**Rejected**: Compared channel (flaky now); dropping it (loses the P3a seam's value for V1).
*Introduced by*: 260916-i9hc-harness-differential

#### No clock injection; re-run once on date rollover
**Decision**: Both sides run back-to-back under the real clock; a local-date change between them triggers a single re-run.
**Why**: Node has no fake clock without a dependency and the TS side is frozen (D4).
**Rejected**: `faketime`/`libfaketime` (platform-specific, absent on CI); accepting the rare midnight flake.
*Introduced by*: 260916-i9hc-harness-differential

#### `stty` inside the pty, exit via sentinel
**Decision**: The `script` wrapper pins `cols 120 rows 40` and appends `__TUDIFF_EXIT=<n>`.
**Why**: An unsized pty reports 0 columns, which the TS `?? 80` fallback does not catch; util-linux and BSD `script` disagree on exit-code propagation, a sentinel is portable.
**Rejected**: `COLUMNS` env (spec says it is never read); util-linux `-e` alone (not on BSD).
*Introduced by*: 260916-i9hc-harness-differential

## Tasks

### Phase 1: Setup

- [x] T001 Write `harness/matrix.json` per R5: schema 1, the full group list by area (toolkit, snapshots, formats/flags, history, leaderboard, multi flags, setup, usage errors), axes per group as specified, no `update`, no `--watch` beyond the incompatibility case. <!-- R5 -->
- [x] T002 [P] Create the seed tree `harness/metrics-repo/` per R7: six `.jsonl` day-files with the listed costs and placeholder token counters, plus `docs/README.md`. <!-- R7 -->

### Phase 2: Core Implementation

- [x] T003 Implement `src/go/internal/harness/matrix.go` (`Matrix`, `CaseGroup`, `Case`, `LoadMatrix`, `Expand`, `Filter`, axis constants and base values) with `matrix_test.go` covering validation errors, expansion order/IDs, defaults, filter, and the committed file's bounds and axis coverage. <!-- R4 R5 -->
- [x] T004 [P] Implement `src/go/internal/harness/homes.go` (`StageHome`, the shared conf bytes constant, recursive seed copy) with `homes_test.go` covering the four variants' exact file sets, conf byte-identity, seed presence, and the seed's `UsageEntry` shape. <!-- R6 R7 -->
- [x] T005 [P] Implement `src/go/internal/harness/fixtures.go` (`ResolveFixtures`, `UnconfirmedReplays`) with `fixtures_test.go` over a temp fixture root (hostname present/absent, explicit list, placeholder-only, missing manifest error, unconfirmed flag from a call log). <!-- R12 -->
- [x] T006 Implement `src/go/internal/harness/diff.go`: `Side`, `Capture`, `Result`, `StageOracle`, `BuildEnv`, `RunPipe`, `RunTTY` (script flavour detection, wrapper composition with shell quoting, sentinel parsing), `Compare` (channel order, first divergence with offset/line/excerpts, sorted call-set comparison), with `diff_test.go` covering env exactness and leak-proofing, sentinel parsing, and every comparison branch. <!-- R8 R9 R10 R11 R13 -->
- [x] T007 [P] Implement `src/go/internal/harness/report.go`: header/summary types, per-case line rendering, the by-axis summary, `report.txt`/`report.json` writers, and the `cases/` raw-capture writer, with `report_test.go` asserting the exact line and summary formats and the JSON schema. <!-- R14 -->
- [x] T008 <!-- rework: runAllCases streams case lines in completion order; must stream in matrix order after the pool drains (review must-fix, A-016/A-019/A-025) --> Implement `src/go/cmd/tudiff/run.go` (`runRun`): flag set, preflight checks and messages in R2 order, `--list`, D7 resolution call, oracle staging, worker pool with `--jobs`, per-case orchestration (stage homes → run node → run go → date-rollover check and single re-run → compare → unconfirmed lookup → write captures), streaming report lines, final summary, exit codes per R3, temp cleanup; wire `case "run": return runRun(args[1:], stdout, stderr)` in `main.go` and update the `run` usage line. <!-- R1 R2 R3 R15 R16 -->

### Phase 3: Integration & Edge Cases

- [x] T009 Write `src/go/cmd/tudiff/run_test.go`: each R2 preflight message and exit 2, `--list` output and no report dir, `--filter` no-match exit 2, `--jobs 0` exit 2, and the end-to-end smoke with stand-in executables (a `node` shim on PATH, stub `ccusage`/`git` in a temp harness-bin, minimal `_placeholder` manifest) asserting one green and one red case in `report.json` and exit 1; remove `TestRunStub` from `main_test.go`. <!-- R19 R2 R3 -->
- [x] T010 [P] Add the `go-diff` recipe to `justfile` per R17. <!-- R17 -->
- [x] T011 [P] Add the `go-diff` job to `.github/workflows/ci.yml` per R18; leave `ci-gate` untouched. <!-- R18 -->
- [x] T012 Run the live baseline: `npm ci` (if `node_modules/` is absent), then `just go-diff` (add `--placeholder` when `harness/fixtures/<hostname>/manifest.json` is absent); confirm only the two `version` groups are green, `dist/` is unmodified, and record the summary block under `## Notes`. <!-- R20 R8 -->

### Phase 4: Polish

- [x] T013 Run `just go-lint` and `just go-test`; fix any gofmt/vet finding; confirm `src/go/cmd/tu/main.go` and `src/node/**` are untouched (`git status`). <!-- R19 R20 -->

## Execution Order

- T003, T004, T005, T007 are independent once T001/T002 exist; T006 depends on T005's `Result`-adjacent types being agreed (build them in the same package so compilation is the arbiter).
- T008 depends on T003–T007; T009 depends on T008; T012 depends on T008, T010, T002; T013 last.

## Acceptance

### Functional Completeness

- [x] A-001 R1: `tudiff run` accepts exactly the eleven flags with the stated defaults, resolves relative defaults against the repo root, and the usage text names `run` correctly.
- [x] A-002 R2: Each preflight condition produces its exact one-line `tudiff: …` message and exit 2, in the specified order; `--list` prints expanded IDs and exits 0 without creating the report dir.
- [x] A-003 R3: Exit code is 0 all-green, 1 any red/timeout, 2 usage/preflight; an empty `--filter` match exits 2.
- [x] A-004 R4: `LoadMatrix` rejects every listed invalid shape with an error naming the group id and field; `Expand` yields the nested conf→env→io→tz order with base defaults and `<id>/<conf>/<env>/<io>/<tz>` IDs.
- [x] A-005 R5: The committed `harness/matrix.json` loads, expands to 200–400 cases, contains every listed group family, no `update` and no `--watch` beyond the incompatibility case, and covers all twelve axis values.
- [x] A-006 R6: `StageHome` produces exactly the per-variant file sets with byte-identical conf contents and the seed copied for the three multi variants; the driver stages two disjoint HOMEs per case.
- [x] A-007 R7: `harness/metrics-repo/` contains exactly the seven listed files with the stated costs and seven-field `UsageEntry` lines.
- [x] A-008 R8: The oracle is staged as a copy with `tu.default.conf` beside it and the fake in the vendor slot at mode 0755; `dist/` is never written.
- [x] A-009 R9: `BuildEnv` returns exactly the specified variables per axis and nothing inherited; cwd is the side's case directory.
- [x] A-010 R10: Pipe cases capture stdout and stderr separately with stdin from `/dev/null`; a timeout yields `TimedOut`, `Exit -1`, status `timeout`.
- [x] A-011 R11: TTY cases run under `script` with the exact wrapper, flavour-dependent invocation, `stty cols 120 rows 40`, and sentinel-derived exit; the sentinel is stripped from the transcript.
- [x] A-012 R12: `ResolveFixtures` follows D7 for all four input shapes and errors on a manifest-less alias; `UnconfirmedReplays` flags a case from its call log.
- [x] A-013 R13: `Compare` honours the channel order, reports offset/line/40-byte quoted excerpts for the first byte divergence, records exit codes for an exit divergence, and never lets `CallsDiffer` change the status.
- [x] A-014 R14: `report.txt` (streamed and written), `report.json`, and `cases/<id>/…` raw captures match the specified formats, including the by-axis summary block.
- [x] A-015 R15: A local-date change between the two sides triggers exactly one re-run with `Rerun` recorded.
- [x] A-016 R16: Cases run through a `--jobs` pool in isolated per-case directories, results render in matrix order, temp dirs are removed, `--jobs 0` exits 2. (Rework verified: `runAllCases` now flushes completed results through a matrix-order cursor; `TestRunEndToEndSmoke` order assertion passed 5/5 targeted runs plus two full `just go-test` sweeps, and the live 368-case report lists cases in matrix order.)
- [x] A-017 R17: `just go-diff *ARGS` exists with the stated dependencies, command, and comment.
- [x] A-018 R18: `ci.yml` has the `go-diff` job with the exact steps, pins, `continue-on-error` on the harness step only, `if: always()` upload of `bin/harness/report/` named `tudiff-report`, and `ci-gate.needs` unchanged.
- [x] A-019 R19: All listed test files exist and cover their requirements; `TestRunStub` is gone; `just go-lint` and `just go-test` pass. (Re-verified post-rework: `just go-lint` clean, `just go-test` green across all five packages, repeated.)
- [x] A-020 R20: The live baseline summary is recorded under `## Notes`, only the two `version` groups are green, and `src/go/cmd/tu/main.go` is unmodified.

### Behavioral Correctness

- [x] A-021 R1: `tudiff run` no longer prints `tudiff: run is not implemented (plan row P4)`; `tudiff` with no args and `tudiff bogus` still exit 2 with usage.
- [x] A-022 R13: A `green` verdict requires byte-identity of every compared channel; a single differing byte anywhere in `stderr` (pipe) or the transcript (tty) makes the case red.

### Scenario Coverage

- [x] A-023 R4: The two-axis expansion example (`x` with `conf` and `io`) is pinned by a test producing the four IDs in the stated order.
- [x] A-024 R9: A test exports `TU_METRICS_REPO` and `NO_COLOR` in the test process and asserts a `default`-env case's environment excludes both.
- [x] A-025 R19: The end-to-end smoke runs without Node or a built `dist/` (stand-ins + `node` shim) and asserts one green, one red, exit 1. (Re-verified post-rework: passes consistently, including the matrix-order line assertion.)

### Edge Cases & Error Handling

- [x] A-026 R11: A transcript without the sentinel yields `Exit -1` and `Err = "no exit sentinel"` rather than a panic or a false green.
- [x] A-027 R12: A `--fixtures` alias directory that exists but lacks `manifest.json` is a preflight exit 2, not a runtime miss.
- [x] A-028 R2: `script` absence is an error only when the filtered matrix contains a `tty` case; a `--filter` restricted to pipe cases runs without `script`.

### Code Quality

- [x] A-029 Pattern consistency: New files follow the P3a harness package conventions (testable `run…(args, stdout, stderr) int` seams, `flag.NewFlagSet` with `ContinueOnError`, `tudiff: `-prefixed errors, `pathComponent` reuse, struct-driven JSON with declared field order).
- [x] A-030 No unnecessary duplication: `FindRepoRoot`, `pathComponent`, `ReadManifest`, `LoadCorpus`/`Key` are reused rather than re-implemented; no second JSON-lines call-log parser shape beyond `callLogLine`.
- [x] A-031 Readability over cleverness: no function exceeds ~50 lines without a stated reason; the per-case orchestration in `run.go` is split into named steps.
- [x] A-032 Named constants: axis values, base defaults, the conf body, the seed URL, the sentinel prefix, the pty size, and both TZ names are constants, not repeated literals.
- [x] A-033 Errors surfaced, never swallowed: every I/O failure in staging, execution, or report writing is returned and reported (exit 2 or a `timeout`/error status), not ignored.
- [x] A-034 Stdlib only: `src/go/go.mod` still declares no dependencies and no `go.sum` appears.
- [x] A-035 Minimum pathways: one execution path per `io` value; pipe and tty share `Compare`, `BuildEnv`, staging, and reporting.

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`
- **P4 baseline (T012, 2026-09-16, dev-ws-sahil02, `just go-diff --placeholder`, exit 1 as expected)**:

  ```
  tudiff: 368 cases — 2 green, 366 red, 0 timeout   (fixtures: _placeholder; 267 cases replayed unconfirmed fixtures)
    by conf:  single 2/155  multi 0/127  org 0/43  legacy 0/43
    by env:   default 2/304  nocolor 0/32  envrepo 0/32
    by io:    pipe 2/295  tty 0/73
    by tz:    fixed 2/306  alt 0/62
  ```

  Only the `version/…` and `version-short/…` cases are green; every other group is red. `src/go/cmd/tu/main.go` is unmodified, and the harness never writes `dist/` (the recipe's own `build` step rebuilds it; the harness only reads it).
- **Deviation (matrix size)**: the R5 group/axis enumeration expands to a strict minimum of 416 cases, which exceeds R5's own 200–400 bound. Resolved by trimming the twelve secondary snapshot groups (`codex`, `co`, `oc`, `gemini`, `gem`, `copilot`, `cop`, `kimi`, `ki`, `w`, `m`, `cc m`) to `conf: [single, multi]` (keeping both `io` values, since TTY layout parity of the formatter is the plan's top risk); `snapshot-all`/`snapshot-cc` keep all four conf values × all env × both io × both tz. Result: 368 cases, all group families and all twelve axis values covered. G0 owns any further trim/restore.

## Deletion Candidates

- None — this change adds new functionality without making existing code redundant. The one removal it performed (the P3a `run` stub branch in `src/go/cmd/tudiff/main.go` and its `TestRunStub` in `main_test.go`) was a planned replacement declared in the requirements (R1/R19), executed during apply.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Files split as `matrix.go`, `homes.go`, `fixtures.go`, `diff.go`, `report.go` in `internal/harness` plus `cmd/tudiff/run.go` | Mirrors P3a's one-concern-per-file layout (`capture.go`, `placeholder.go`, `replay.go`, …) | S:70 R:90 A:90 D:85 |
| 2 | Certain | `--list` runs only the matrix check; every other preflight check is skipped for it | Listing is a read-only developer aid and must work before binaries are built | S:60 R:95 A:90 D:90 |
| 3 | Confident | Excerpts are `strconv.Quote` of up to 40 bytes starting at the first differing byte; line number counts `\n` in the node capture | One-line readability in the report; the node side is the oracle so its line numbering is the reference | S:60 R:90 A:85 D:75 |
| 4 | Confident | The e2e smoke uses shell-script stand-ins and a `node` shim on PATH so `go test` never needs Node or a built bundle | Keeps `just go-test` self-contained in the `go-build-and-test` lane, which has no Node | S:55 R:90 A:85 D:75 |
| 5 | Confident | `--filter` with zero matches is exit 2 | A vacuous green exit 0 would be a silent false pass | S:45 R:90 A:85 D:80 |
| 6 | Confident | Call-set comparison drops `cwd` and `matched`, keeps `tool` + `argv` | `cwd` differs by side by construction; `matched` is alias metadata | S:55 R:90 A:85 D:80 |
| 7 | Confident | Case directories under the report use the case ID's slashes as nesting (`cases/snapshot-all/multi/nocolor/pipe/fixed/`) | Readable, `ls`-navigable by axis; IDs are already validated path components | S:50 R:90 A:85 D:75 |
| 8 | Confident | The report dir is wiped at the start of a run; temp staging lives under `os.MkdirTemp` and is removed at the end | Stale reports would mislead the burndown; captures needed for inspection live in the report, not in temp | S:55 R:90 A:85 D:80 |
| 9 | Confident | A `timeout` counts as red for the exit code and summary but is listed separately in the summary line | It is a failure to reach parity, but a different kind from a byte divergence | S:50 R:90 A:80 D:75 |
| 10 | Tentative | Matrix size target 200–400 cases; the exact initial list follows the R5 enumeration and may be trimmed at G0 | Balances CI time against coverage; G0 owns the final say | S:50 R:95 A:70 D:55 |

10 assumptions (2 certain, 7 confident, 1 tentative).
