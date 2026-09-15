# Plan: Harness Fixture Capture

**Change**: 260915-r7dh-harness-fixture-capture
**Intake**: `intake.md`

## Requirements

All paths are repo-relative. "ccusage subcommand" means one of `claude`, `codex`, `opencode`, `gemini`, `copilot`, `kimi` (tu's registry order). Go code is stdlib-only, `gofmt`-clean, `go vet`-clean, with `_test.go` siblings (constitution § Go Transition). Nothing under `src/node/`, `dist/`, `scripts/`, `.github/`, or any surface frozen by the plan's Goal is touched.

### Harness: Fixture Corpus Layout and Manifest

#### R1: Per-machine fixture directory
Fixtures SHALL live under `harness/fixtures/<machine-alias>/<source>/<period>.json`, with an optional sibling `<period>.stderr.txt` when recorded stderr is non-empty, and exactly one `harness/fixtures/<machine-alias>/manifest.json` per machine directory. The placeholder corpus SHALL use the alias `_placeholder`.

- **GIVEN** a capture for machine `dev-ws-sahil02` of source `claude`, period `daily`
- **WHEN** it is written
- **THEN** the file is `harness/fixtures/dev-ws-sahil02/claude/daily.json` and `harness/fixtures/dev-ws-sahil02/manifest.json` has an entry with `"file": "claude/daily.json"`

#### R2: Manifest schema v1
`harness/fixtures/<alias>/manifest.json` MUST be produced from Go structs in `src/go/internal/harness/manifest.go` with these top-level fields: `schema` (int, `1`), `machine`, `captured_at` (RFC 3339 UTC), `ccusage_version`, `ccusage_path`, `platform` (`GOOS/GOARCH`), `timezone` (IANA), optional `derived_from` (placeholder corpus only, omitted when empty), and `fixtures` (array). Each fixture entry MUST carry: `source`, `period`, `args` (string array), `file`, `stderr_file` (empty string when none), `exit_code`, `sha256` (hex of the committed file bytes), `days`, `first_date`, `last_date`, `empty` (bool), `redactions` (int), `unconfirmed` (bool). `source` is the ccusage subcommand name, never tu's key.

- **GIVEN** a recorded stdout that parses as `{"daily":[{"date":"2026-09-08"},…,{"date":"2026-09-16"}]}` with 9 entries
- **WHEN** the manifest entry is built
- **THEN** `days` is 9, `first_date` is `2026-09-08`, `last_date` is `2026-09-16`, `empty` is false
- **GIVEN** a recorded stdout that is not parseable JSON (a failed source)
- **WHEN** the entry is built
- **THEN** `days` is 0, `first_date`/`last_date` are empty, `empty` is true, and `exit_code` carries the process exit code

#### R3: Manifest regeneration is per-machine and wholesale
Re-running a capture for alias A SHALL overwrite only `harness/fixtures/A/` (its fixture files and manifest) and MUST NOT read or modify any other alias directory.

- **GIVEN** `harness/fixtures/_placeholder/` exists
- **WHEN** `tudiff capture --machine dev-ws-sahil02` runs twice
- **THEN** `_placeholder/` is byte-identical before and after, and `dev-ws-sahil02/manifest.json` differs at most in `captured_at` (and in data if new transcripts landed)

### Harness: `tudiff capture`

#### R4: Command surface
`src/go/cmd/tudiff/main.go` SHALL dispatch on the first argument: `capture`, `placeholder`, `run`. `run` MUST print `tudiff: run is not implemented (plan row P4)` to stderr and exit 1. No argument, or an unknown subcommand, MUST print a one-paragraph usage to stderr and exit 2. `capture` accepts `--machine <alias>` (default `os.Hostname()`), `--ccusage <path>`, `--out <dir>` (default `harness/fixtures`), `--sources <a,b,…>` (default the six subcommands in registry order), `--periods <a,b,…>` (default `daily`). The entry point MUST be structured as a testable `run(args []string, stdout, stderr io.Writer) int`, as `cmd/tu/main.go` is.

- **GIVEN** `tudiff run`
- **WHEN** invoked
- **THEN** exit code is 1 and stderr is exactly `tudiff: run is not implemented (plan row P4)\n`

#### R5: ccusage binary resolution
When `--ccusage` is omitted, `capture` MUST locate the repo root by walking up from the working directory to the first directory containing `package.json`, then try in order: `dist/vendor/ccusage/bin/ccusage`; `node_modules/@ccusage/ccusage-<platform>-<arch>/bin/ccusage` where `<platform>` is `runtime.GOOS` and `<arch>` maps `amd64`→`x64`, `arm64`→`arm64`; `ccusage` on `PATH` via `exec.LookPath`. If none exists it MUST print `tudiff: no ccusage binary found (run npm ci or pass --ccusage)` to stderr and exit 1. The chosen path, made repo-relative when under the root, is recorded as `ccusage_path`.

- **GIVEN** a linux/amd64 worktree after `npm ci` and no `dist/`
- **WHEN** `tudiff capture` resolves the binary
- **THEN** it selects `node_modules/@ccusage/ccusage-linux-x64/bin/ccusage`

#### R6: Capture execution and recording
For every (source, period) cell, `capture` SHALL run `<ccusage> <source> <period> --json` with `exec.CommandContext` (120 s timeout, cwd = repo root, inherited environment), record stdout bytes, stderr bytes, and exit code; a non-zero exit or timeout is recorded in the manifest and MUST NOT abort the run. stdout MUST be passed through redaction (R7) and written verbatim — no JSON decode/re-encode. It SHALL print one line per cell of the form `<source> <period> --json  exit=<n>  days=<n>  <first>..<last>  redactions=<n>  -> <alias>/<source>/<period>.json` and a final summary naming any non-zero cells. `ccusage_version` is read from `<ccusage> --version` (stdout `ccusage 20.0.19` → `20.0.19`); `timezone` is `$TZ` when set, else the system zone name.

- **GIVEN** a stub "ccusage" script in a temp dir that prints `{"daily":[],"totals":{"totalCost":-0.0}}` for `opencode daily --json` and exits 3 for `kimi daily --json`
- **WHEN** `capture --ccusage <stub> --out <tmp> --machine t --sources opencode,kimi` runs
- **THEN** exit code is 0, `t/opencode/daily.json` contains the literal `-0.0`, the manifest entry for `kimi` has `exit_code: 3` and `empty: true`, and the summary line lists `kimi`

### Harness: Redaction

#### R7: Home-rooted path redaction on raw bytes
`Redact(raw []byte, home string) ([]byte, int)` in `src/go/internal/harness/redact.go` MUST replace every absolute path rooted in `home` (when non-empty), `/home/<user>`, or `/Users/<user>` — including the remainder of the path up to a closing `"` or whitespace — with `~/redacted-<n>`, where `n` starts at 1 and is assigned in first-seen order per distinct original path within one call. It MUST operate on bytes (regexp), never via JSON decoding. It MUST NOT alter Claude's encoded project-directory form (`-home-sahil-code-…`, no leading slash), hostnames, model names, or numbers. The count returned is the number of replacements performed.

- **GIVEN** `{"a":"/home/sahil/code/x","b":"/home/sahil/code/x","c":"/Users/bob/work/y","d":"-home-sahil-code-x","e":"gpt-5"}` and `home=/home/sahil`
- **WHEN** redacted
- **THEN** output is `{"a":"~/redacted-1","b":"~/redacted-1","c":"~/redacted-2","d":"-home-sahil-code-x","e":"gpt-5"}` and the count is 3

### Harness: Placeholder Corpus

#### R8: Deterministic schema-derived placeholder
`tudiff placeholder --source <s> [--source <s>…] [--out <dir>]` (default out `harness/fixtures/_placeholder`) SHALL write `<out>/<s>/daily.json` from Go structs mirroring the observed v20 per-agent claude-style daily schema: top-level `daily` (array) and `totals`; each entry has `cacheCreationTokens`, `cacheReadTokens`, `date`, `inputTokens`, `modelBreakdowns` (array of `cacheCreationTokens`, `cacheReadTokens`, `cost`, `inputTokens`, `modelName`, `outputTokens`), `modelsUsed`, `outputTokens`, `totalCost`, `totalTokens`; `totals` has `cacheCreationTokens`, `cacheReadTokens`, `inputTokens`, `outputTokens`, `totalCost`, `totalTokens`. Output MUST be deterministic (same bytes on every run), use alphabetically ordered keys and 2-space indentation, contain three consecutive days `2026-01-05`..`2026-01-07`, model name `placeholder-<source>-model`, `totals` equal to the column sums, and `totalTokens` equal to the sum of the four token counters in every entry and in `totals`.

- **GIVEN** `tudiff placeholder --source opencode --out <tmp>` run twice
- **WHEN** the two outputs are compared
- **THEN** they are byte-identical, decode into the schema structs, and `totals.totalTokens` equals the sum over `daily[].totalTokens`

#### R9: Placeholder manifest and shadowing guard
`placeholder` SHALL write `<out>/manifest.json` with `machine: "_placeholder"`, `ccusage_version: "20.0.19"`, `ccusage_path: ""`, `derived_from: "dev-ws-sahil02/claude/daily.json"`, and every fixture `unconfirmed: true` (other entry fields computed as in R2). Before writing, it MUST scan every other alias directory under the parent of `<out>` and exit 1 with `tudiff: <source> already has a confirmed non-empty fixture (<alias>) — refusing to write a placeholder` if any manifest there has a fixture for that source with `unconfirmed: false` and `empty: false`.

- **GIVEN** `harness/fixtures/dev-ws-sahil02/manifest.json` with a confirmed non-empty `codex` fixture
- **WHEN** `tudiff placeholder --source codex` runs
- **THEN** exit code is 1, nothing is written, and stderr names `codex` and `dev-ws-sahil02`

### Harness: Fake `ccusage`

#### R10: Replay by exact argv key
`src/go/cmd/fakeccusage/main.go` (built as `bin/harness/ccusage`) SHALL read `TUDIFF_FIXTURES` as an OS path list of alias directories searched in order. With it unset or empty it MUST print `fakeccusage: TUDIFF_FIXTURES not set` to stderr and exit 2. It SHALL parse argv as `<source> <period> <flags…>`, build the key `(source, period, sorted flags)`, and match a manifest fixture whose `(source, period, sorted args)` is equal. On a hit it MUST write the fixture file bytes verbatim to stdout, the recorded stderr file (if any) to stderr, and exit with the recorded `exit_code`. On a miss it MUST print `fakeccusage: no fixture for argv [<argv>]` to stderr and exit 2. `--version` or `-v` as the sole argument prints `ccusage <ccusage_version>` from the first manifest and exits 0.

- **GIVEN** `TUDIFF_FIXTURES=harness/fixtures/dev-ws-sahil02:harness/fixtures/_placeholder`
- **WHEN** `bin/harness/ccusage claude daily --json` runs
- **THEN** stdout is byte-identical to `harness/fixtures/dev-ws-sahil02/claude/daily.json` and the exit code is 0
- **GIVEN** the same environment
- **WHEN** `bin/harness/ccusage kimi weekly --json` runs
- **THEN** exit code is 2 and stderr starts with `fakeccusage: no fixture for argv`

#### R11: Shared call log
Both fakes SHALL append one JSON line per invocation to the file named by `TUDIFF_CALL_LOG` when set (create if absent, append otherwise), via a shared `src/go/internal/harness/calllog.go`. Line shape: `{"tool":"ccusage"|"git","argv":[…],"cwd":"…","matched":"<alias>/<file>"}` — `matched` is present only for ccusage hits. A missing or unwritable log path MUST NOT change the fake's exit code or output (best-effort, error swallowed silently — the fake is impersonating a tool whose stderr is being compared).

- **GIVEN** `TUDIFF_CALL_LOG=/tmp/x/calls.jsonl` and an existing file with 2 lines
- **WHEN** the fake ccusage is invoked once
- **THEN** the file has 3 lines and the last decodes with `tool == "ccusage"` and a 3-element `argv`

### Harness: Fake `git`

#### R12: Recording stub with scripted responses
`src/go/cmd/fakegit/main.go` (built as `bin/harness/git`) SHALL log every invocation per R11 with `tool: "git"`, then answer from `TUDIFF_GIT_SCRIPT` when set: a JSON array of rules `{"match":[…],"stdout":"…","stderr":"…","exit":n}`, where `match` is a prefix match on argv after stripping a leading `-C <dir>` pair; first matching rule wins. With no script, or no matching rule, it MUST write nothing to stdout or stderr and exit 0. It MUST perform no filesystem or network operations beyond the call log. It MUST accept (i.e. not special-case or reject) the argv shapes tu issues: `rebase --abort`, `add <user>/`, `status --porcelain <user>/`, `commit -m <msg>`, `pull --rebase origin main`, `push`, `rev-parse --git-dir`, `clone <url> <dir>`.

- **GIVEN** `TUDIFF_GIT_SCRIPT='[{"match":["status","--porcelain"],"stdout":" M u/x\n","exit":0}]'`
- **WHEN** `git -C /tmp/m status --porcelain u/` runs
- **THEN** stdout is ` M u/x\n`, exit 0, and the call log line has `argv` equal to `["-C","/tmp/m","status","--porcelain","u/"]`
- **GIVEN** no script
- **WHEN** `git -C /tmp/m pull --rebase origin main` runs
- **THEN** stdout and stderr are empty and exit is 0

### Build: Recipes and Corpus Validation

#### R13: `just` recipes
`justfile` SHALL gain `harness-build` (builds `./cmd/tudiff`, `./cmd/fakeccusage`, `./cmd/fakegit` from `src/go` into `bin/harness/tudiff`, `bin/harness/ccusage`, `bin/harness/git`) and `harness-capture *ARGS` (depends on `harness-build`, runs `bin/harness/tudiff capture {{ARGS}}`), placed under the existing Go section with the comment shape used there. `go-build`, `go-test`, `go-lint`, and every Node recipe MUST be unchanged. No CI workflow file is edited.

- **GIVEN** a clean checkout with Go installed
- **WHEN** `just harness-build` runs
- **THEN** the three binaries exist under `bin/harness/` and `git status --porcelain` shows nothing new (`bin/` is gitignored)

#### R14: Corpus validation test
`src/go/internal/harness/corpus_test.go` SHALL walk `../../../../harness/fixtures/*/manifest.json` and, for every fixture entry, assert: the file exists; its sha256 matches; when `exit_code == 0` the content parses as JSON with a `daily` array whose length equals `days`, and `first_date`/`last_date`/`empty` agree with the content; no string value in the file contains `/home/` or `/Users/`; `unconfirmed: true` occurs only under the `_placeholder` alias; and no source has both a confirmed non-empty fixture in any alias and a placeholder entry. The test MUST fail (not skip) when the fixtures directory is absent.

- **GIVEN** the committed corpus
- **WHEN** `just go-test` runs
- **THEN** the corpus test passes; editing one byte of `harness/fixtures/dev-ws-sahil02/kimi/daily.json` without regenerating the manifest makes it fail on sha256

### Committed Corpus

#### R15: Real capture from this machine and placeholders for the rest
The change SHALL commit the output of `just harness-capture` run on this machine (alias `dev-ws-sahil02`: six fixtures + manifest; claude, codex, gemini, kimi non-empty and confirmed; opencode, copilot empty and confirmed) and of `tudiff placeholder --source opencode --source copilot` (alias `_placeholder`: two fixtures + manifest, `unconfirmed: true`). Every committed fixture MUST report `redactions: 0` (verified expectation for this machine); a non-zero count is not an error but MUST be mentioned in the apply result.

- **GIVEN** the committed tree
- **WHEN** `TUDIFF_FIXTURES=harness/fixtures/dev-ws-sahil02 bin/harness/ccusage codex daily --json | cmp - harness/fixtures/dev-ws-sahil02/codex/daily.json` runs
- **THEN** `cmp` reports no difference

### Plan Bookkeeping

#### R16: Plan document update
`fab/plans/sahil/26-09-15-go-port.md` SHALL have row P3a's **PR** column set to `260915-r7dh-harness-fixture-capture` and **Status** to a short landed note; D7's rationale corrected to "dev-ws-sahil02 has real claude, codex, gemini, kimi data; opencode and copilot are empty there (2026-09-16)"; and P3b's Scope narrowed to one machine with opencode data and one with copilot data. No other plan edits.

- **GIVEN** the updated plan
- **WHEN** row P3b is read
- **THEN** it no longer lists codex, gemini, or kimi as needing external captures

### Non-Goals

- `tudiff run`, the argument matrix, temp `$HOME` staging, env/TZ matrix, staging `dist/` with the fake — P4.
- Any `internal/{fact,source,…}` package (V1); any `src/node/`, `dist/`, `scripts/`, `.github/`, formula, README, or `docs/site/` change.
- Capturing `weekly`/`monthly`/`--since`/`--instances` cells — tu never sends them; the layout stays general.
- Confirming opencode/copilot shapes — P3b, by a human on another machine.
- A YAML manifest, cobra, or any third-party dependency.

### Design Decisions

#### Manifest is JSON, one per machine directory
**Decision**: `manifest.json` produced from Go structs, regenerated wholesale per alias.
**Why**: Zero-dependency posture (no YAML in stdlib); per-machine files let P3b captures from other machines land without merge conflicts; structs make the schema code.
**Rejected**: A single repo-wide manifest (every machine's capture rewrites one file); YAML (needs a dependency).
*Introduced by*: 260915-r7dh-harness-fixture-capture

#### Fixtures are verbatim bytes, redacted by regexp on the raw stream
**Decision**: stdout is never decoded and re-encoded before being written.
**Why**: Byte fidelity is the harness's purpose; `-0.0` totals and serializer key order are real traits a re-encode would erase.
**Rejected**: Redacting via JSON walk (loses formatting); redacting nothing (blocks committing `--instances`-style captures later).
*Introduced by*: 260915-r7dh-harness-fixture-capture

#### Real empty captures are confirmed; only `_placeholder/` is unconfirmed
**Decision**: An exit-0 `daily: []` capture is `empty: true, unconfirmed: false`; placeholders are the only `unconfirmed: true` entries and the generator refuses to shadow a confirmed non-empty fixture.
**Why**: "We observed nothing" and "we guessed the shape" are different facts; the harness must report the second separately (row text) and the corpus must never carry both for one source.
**Rejected**: Marking empties unconfirmed (conflates the two); allowing placeholder + real to coexist (ambiguous replay).
*Introduced by*: 260915-r7dh-harness-fixture-capture

#### Placeholders use the claude-style shape
**Decision**: opencode/copilot placeholders carry `totalCost` + `modelBreakdowns[]` + `modelsUsed[]`.
**Why**: Their real empty outputs use `totalCost` in `totals`; codex (`costUSD`, `models{}`, `reasoningOutputTokens`) is the sole outlier and is captured for real.
**Rejected**: Codex-style shape for all placeholders (contradicts observed totals key).
*Introduced by*: 260915-r7dh-harness-fixture-capture

#### Fakes are separate env-configured Go binaries
**Decision**: `cmd/fakeccusage` and `cmd/fakegit`, built under the impersonated names into `bin/harness/`, configured only by `TUDIFF_*` env vars.
**Why**: Their argv belongs to tu; separate mains read better than an argv[0]-dispatch trick; a static binary can be copied into the TS side's fixed `dist/vendor/ccusage/bin/ccusage` slot, which a shell shim could not do portably.
**Rejected**: Busybox-style single binary dispatching on `os.Args[0]`; shell shims exec'ing `tudiff fake-…`.
*Introduced by*: 260915-r7dh-harness-fixture-capture

## Tasks

### Phase 1: Setup

- [x] T001 Create `src/go/internal/harness/manifest.go`: `Manifest`/`Fixture` structs with JSON tags per R2 (`DerivedFrom` with `omitempty`), `const SchemaVersion = 1`, `FixturePath(source, period string) string` (`<source>/<period>.json`), `StderrPath`, `Key(source, period string, args []string) string` (sorted args), `ReadManifest(dir)`/`WriteManifest(dir, m)` (2-space indent, trailing newline), and `Summarize(stdout []byte) (days int, first, last string, empty bool)` that tolerates non-JSON input. Sibling `manifest_test.go` covering both R2 scenarios and round-trip. <!-- R1, R2 -->
- [x] T002 [P] Create `src/go/internal/harness/calllog.go`: `LogCall(tool string, argv []string, matched string)` appending one JSON line to `$TUDIFF_CALL_LOG` when set, swallowing all errors; `calllog_test.go` covering append-to-existing, unset env (no file), unwritable path (no error, no panic). <!-- R11 -->

### Phase 2: Core Implementation

- [x] T003 [P] Create `src/go/internal/harness/redact.go` with `Redact(raw []byte, home string) ([]byte, int)` per R7 (compile the pattern from `home` + generic `/home/[^/"\s]+` + `/Users/[^/"\s]+`, greedy remainder `[^"\s]*`, stable first-seen numbering) and `redact_test.go` with the R7 scenario plus a no-op case and a `home=""` case. <!-- R7 -->
- [x] T004 <!-- rework: review cycle 1 — captureCell swallows MkdirAll/WriteFile/sidecar errors (capture.go:236-246); propagate or warn on stderr and mark the cell failed (A-028) --> Create `src/go/internal/harness/capture.go`: `ResolveCcusage(root, explicit string) (path string, err error)` per R5 (Node platform/arch spelling), `FindRepoRoot(start string)` (walk up to `package.json`), `CcusageVersion(path)`, `SystemTimezone()`, and `Capture(opts CaptureOptions, stdout io.Writer) (summary, error)` per R6 (timeout, non-fatal non-zero exits, redaction, verbatim write, stderr sidecar, manifest build with sha256, per-cell and summary lines). `capture_test.go` uses a temp-dir stub shell script as the ccusage binary to exercise the R6 scenario (empty `-0.0` output, a non-zero cell, redaction count) and checks R3 (a sibling alias dir is untouched). <!-- R3, R5, R6 -->
- [x] T005 [P] Create `src/go/internal/harness/placeholder.go`: schema structs (`DailyReport`, `DailyEntry`, `ModelBreakdown`, `Totals` — alphabetical field order matching R8), `Placeholder(source string) DailyReport` with the fixed three-day data, `EncodePretty(v any) ([]byte, error)` (2-space indent, trailing newline), and `WritePlaceholders(out string, sources []string) error` implementing the R9 shadowing scan over sibling alias dirs and the `_placeholder` manifest. `placeholder_test.go`: determinism, sums, alphabetical keys, and the R9 refusal case with a temp corpus. <!-- R8, R9 -->
- [x] T006 [P] Create `src/go/internal/harness/replay.go`: `LoadCorpus(dirs []string) (*Corpus, error)` (ordered manifests), `(*Corpus) Lookup(source, period string, flags []string) (Hit, bool)` returning file bytes, stderr bytes, exit code, and `matched` label, plus `(*Corpus) Version() string`. `replay_test.go` with a two-alias temp corpus: first-hit-wins ordering, sorted-flag equality, miss. <!-- R10 -->
- [x] T007 Create `src/go/cmd/tudiff/main.go` with `run(args, stdout, stderr) int` dispatching `capture` / `placeholder` / `run` per R4 (stdlib `flag` sub-FlagSets; `--source` repeatable via a slice flag), package doc comment naming plan rows P3a/P4 and constitution § Go Transition, and `main_test.go` covering `run` stub exit 1 + exact stderr, no-arg/unknown exit 2, and `capture --ccusage <missing>` error path. <!-- R4, R5 -->
- [x] T008 [P] Create `src/go/cmd/fakeccusage/main.go` per R10/R11 with `run(args, env lookup, stdout, stderr) int` and `main_test.go`: hit (verbatim bytes + exit code), miss (exit 2, message prefix), unset `TUDIFF_FIXTURES` (exit 2), `--version`, and the R11 call-log line. <!-- R10, R11 -->
- [x] T009 [P] Create `src/go/cmd/fakegit/main.go` per R12 with `run(args, env lookup, stdout, stderr) int` (strip leading `-C <dir>`, prefix-match rules, default silent exit 0, log via `harness.LogCall`) and `main_test.go` covering both R12 scenarios plus every tu argv shape listed in R12 with no script. <!-- R11, R12 -->
- [x] T010 [P] Add `harness-build` and `harness-capture *ARGS` recipes to `justfile` under the Go section per R13, matching the existing comment style; leave all other recipes byte-identical. <!-- R13 -->

### Phase 3: Integration & Edge Cases

- [x] T011 Run `just go-lint && just go-test && just harness-build`, then `just harness-capture` (alias defaults to `dev-ws-sahil02`) and `bin/harness/tudiff placeholder --source opencode --source copilot`; inspect both manifests (expect claude/codex/gemini/kimi non-empty confirmed, opencode/copilot empty confirmed, two `_placeholder` entries `unconfirmed: true`, all `redactions: 0`) and keep `harness/fixtures/` for commit. <!-- R15 -->
- [x] T012 Create `src/go/internal/harness/corpus_test.go` per R14 (fail, not skip, when `../../../../harness/fixtures` is missing) and run `just go-test` against the committed corpus. <!-- R14 -->
- [x] T013 End-to-end verification per R10/R12/R15: `TUDIFF_FIXTURES=harness/fixtures/dev-ws-sahil02:harness/fixtures/_placeholder bin/harness/ccusage claude daily --json | cmp - harness/fixtures/dev-ws-sahil02/claude/daily.json`; same for `codex` and for `opencode` via the `_placeholder`-first ordering (`TUDIFF_FIXTURES=harness/fixtures/_placeholder:harness/fixtures/dev-ws-sahil02`); `bin/harness/ccusage kimi weekly --json` exits 2; `TUDIFF_CALL_LOG=<tmp> bin/harness/git -C /tmp status --porcelain u/` exits 0 with empty stdout and one log line; re-run `just harness-capture` and confirm `git diff --stat harness/` is confined to `captured_at` (or data). Fix anything that disagrees. <!-- R10, R12, R15 -->

### Phase 4: Polish

- [x] T014 Update `fab/plans/sahil/26-09-15-go-port.md` per R16 (row P3a PR/Status, D7 rationale, P3b scope) and confirm `just go-lint && just go-test` are green as the final state. <!-- R16 -->

## Execution Order

- T001 blocks T004, T005, T006, T008, T009 (they import `harness`); T002 blocks T008, T009
- T003, T005, T006 are independent of each other once T001 is done
- T007 depends on T004 and T005; T011 depends on T007–T010; T012 and T013 depend on T011; T014 last

## Acceptance

### Functional Completeness

- [x] A-001 R1: Committed fixtures follow `harness/fixtures/<alias>/<source>/<period>.json` with one `manifest.json` per alias, and the placeholder alias is `_placeholder`
- [x] A-002 R2: Both committed manifests carry every top-level and per-fixture field listed in R2 (`derived_from` only in `_placeholder`), `source` values are ccusage subcommand names, and `days`/`first_date`/`last_date`/`empty` agree with file contents
- [x] A-003 R4: `tudiff` dispatches `capture`/`placeholder`/`run`; `run` exits 1 with the exact stderr line; no-arg and unknown subcommand exit 2 with usage
- [x] A-004 R5: `ResolveCcusage` tries vendor → `node_modules/@ccusage/ccusage-<platform>-<arch>/bin/ccusage` → `PATH`, uses Node's `x64` spelling, and fails with the specified message
- [x] A-005 R6: Capture records stdout/stderr/exit per cell with a 120 s timeout, continues past non-zero exits, writes verbatim bytes, and prints per-cell and summary lines
- [x] A-006 R7: `Redact` implements the R7 scenario exactly, on raw bytes, with stable first-seen numbering
- [x] A-007 R8: `tudiff placeholder` output is byte-deterministic, alphabetically keyed, 2-space indented, three days 2026-01-05..07, sums consistent
- [x] A-008 R9: `_placeholder/manifest.json` has the specified constant fields and `unconfirmed: true` on every entry; the shadowing guard refuses with the specified message
- [x] A-009 R10: Fake ccusage replays by exact `(source, period, sorted flags)` key across an ordered `TUDIFF_FIXTURES` list, verbatim bytes and recorded exit code; miss → exit 2 with the specified prefix; unset env → exit 2; `--version` prints `ccusage <version>`
- [x] A-010 R11: Both fakes append the specified JSON line to `TUDIFF_CALL_LOG` best-effort, with `matched` only on ccusage hits
- [x] A-011 R12: Fake git strips `-C <dir>`, prefix-matches `TUDIFF_GIT_SCRIPT` rules first-wins, is silent exit 0 otherwise, and does no fs/network work
- [x] A-012 R13: `harness-build` and `harness-capture` recipes exist as specified; every pre-existing recipe and both CI workflow files are byte-identical to `origin/main`
- [x] A-013 R14: `corpus_test.go` performs every listed assertion and fails (not skips) without the fixtures dir
- [x] A-014 R15: `harness/fixtures/dev-ws-sahil02/` has six fixtures (four non-empty, two empty, all confirmed) and `_placeholder/` has opencode + copilot; all `redactions: 0`
- [x] A-015 R16: Plan row P3a, D7 rationale, and P3b scope are updated; no other plan lines changed

### Behavioral Correctness

- [x] A-016 R3: Re-running capture for one alias leaves other alias directories byte-identical (unit test with a sibling dir)
- [x] A-017 R6: The empty opencode/copilot fixtures contain the literal `-0.0` (verbatim-bytes proof)

### Scenario Coverage

- [x] A-018 R10: `bin/harness/ccusage claude daily --json` output `cmp`s clean against the committed fixture; `kimi weekly --json` exits 2
- [x] A-019 R12: The scripted `status --porcelain` scenario and the no-script `pull --rebase origin main` scenario pass as unit tests
- [x] A-020 R14: Flipping one byte of a committed fixture without regenerating the manifest fails the corpus test on sha256 (re-verified during review against `kimi/daily.json`, then reverted; corpus test green again)

### Edge Cases & Error Handling

- [x] A-021 R6: A non-zero or timed-out cell is recorded (`exit_code`, `empty: true`) and named in the summary without aborting the run
- [x] A-022 R11: An unwritable `TUDIFF_CALL_LOG` path changes neither exit code nor stdout/stderr of either fake
- [x] A-023 R7: Claude's encoded `-home-…` directory form and non-path strings are left untouched

### Code Quality

- [x] A-024 Pattern consistency: Go files follow `cmd/tu/main.go`'s shape (testable `run(args, stdout, stderr) int`, package doc comment, `_test.go` siblings, gofmt/vet clean under `just go-lint`)
- [x] A-025 No unnecessary duplication: manifest/redaction/replay/call-log logic lives once in `internal/harness` and is imported by all three commands
- [x] A-026 Readability over cleverness: no argv[0] dispatch, no reflection-driven schema; functions stay focused (no god functions > 50 lines without reason)
- [x] A-027 Minimum pathways: fixture files are read/written through the same `harness` helpers by capture, placeholder, replay, and the corpus test
- [x] A-028 Errors are never swallowed silently except where a requirement mandates best-effort behavior (R11 call log), and every error path prints a specific message to stderr — re-verified rework cycle 1: `captureCell` returns `(Fixture, error)` and `Capture` propagates fixture-write failures (MkdirAll, fixture write, stderr sidecar) with path-naming messages, so no manifest is written on a write fault (TestCaptureWriteErrorPropagates)
- [x] A-029 Magic values are named constants (`SchemaVersion`, the 120 s timeout, `_placeholder` alias, the placeholder dates)

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Deletion Candidates

- None — this change adds new functionality without making existing code redundant (re-verified in rework cycle 1 review: every pre-existing file touched — `justfile`, `fab/plans/sahil/26-09-15-go-port.md` — gained additive edits only; no symbol, file, branch, or config elsewhere in the repo became unused)

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | Redaction regexp remainder is `[^"\s]*` (stops at a closing quote or whitespace) | Fixture strings are JSON string values; a path never legitimately contains `"` or whitespace in ccusage output | S:60 R:90 A:85 D:75 |
| 2 | Confident | `timezone` is `$TZ` when set, else `time.Now().Location().String()` (falls back to reading `/etc/timezone` when that yields `Local`) | Go reports `Local` for the system zone; the manifest wants an IANA name; the fallback covers Linux dev boxes | S:50 R:95 A:80 D:70 |
| 3 | Confident | Placeholder model name is `placeholder-<source>-model` and dates are 2026-01-05..07 | Obviously synthetic, sorts before any real 2026-08+ data, and is fixed in the intake | S:70 R:95 A:90 D:85 |
| 4 | Confident | `--source` on `placeholder` is a repeatable flag implemented with a small `flag.Value` slice type rather than a comma list, matching the intake's `--source a --source b` shape | Intake shows repeated flags; stdlib `flag` supports it via `flag.Var` | S:65 R:95 A:90 D:80 |
| 5 | Confident | Capture tests use a temp-dir POSIX shell stub as the "ccusage" binary rather than the real one | The real binary is machine-dependent and slow; CI must stay hermetic; the stub exercises every recorded field | S:60 R:95 A:90 D:85 |

5 assumptions (0 certain, 5 confident, 0 tentative).
