---
type: memory
description: Differential harness — fixture corpus harness/fixtures/<alias>/<source>/<period>.json; fake ccusage/git replayers (TUDIFF_* env, call log); tudiff capture/placeholder/run byte-diffs node dist/tu.mjs vs bin/tu over harness/matrix.json groups (conf/env/io/tz axes, staged HOMEs, TTY, tree channel); tudiff live runs real-git sync/repair parity on temp bares; harness/expected-diffs.json keys intentional divergences by spec DC id — unexpected red/timeout/unconfirmed/stale fail the run
---
# Differential Harness

**Domain**: harness

## Overview

The differential harness replays recorded ccusage output and stubs git so the shipped TypeScript binary and the Go successor can be byte-diffed over a deterministic corpus (plan rows P3a/P4 in `fab/plans/sahil/26-09-15-go-port.md`). The corpus lives under `harness/fixtures/` at the repo root; the tooling is three stdlib-only Go commands (`src/go/cmd/tudiff`, `src/go/cmd/fakeccusage`, `src/go/cmd/fakegit`) sharing `src/go/internal/harness/` — the module's first `internal/` package. Two kinds of alias directory coexist there: **real captures** (`harness/fixtures/<machine>/`, written by `tudiff capture`) carry the capturing user's spend and are **gitignored — local-only, never committed**; the **committed corpus** is the schema-derived `_placeholder/` set covering all six sources. `tudiff run` resolves `TUDIFF_FIXTURES` to the machine's local capture when one exists and to `_placeholder` otherwise (D7). Build/test wiring lives in [toolchain](/build/toolchain.md); the upstream per-agent JSON shapes live in [data-pipeline](/cli/data-pipeline.md).

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
`src/go/cmd/tudiff/main.go` dispatches on the first argument (`capture`, `placeholder`, `run`) behind a testable `run(args, stdout, stderr) int` seam. `capture` accepts `--machine <alias>` (default `os.Hostname()`), `--ccusage <path>`, `--out <dir>` (default `harness/fixtures`), `--sources <a,b,…>` (default the six ccusage subcommands in tu's registry order: `claude,codex,opencode,gemini,copilot,kimi`), and `--periods <a,b,…>` (default `daily`). When `--ccusage` is omitted, the binary is resolved against the repo root (found by walking up to the first directory containing `package.json`) in order: `dist/vendor/ccusage/bin/ccusage` → `node_modules/@ccusage/ccusage-<platform>-<arch>/bin/ccusage` (Node's spelling — `runtime.GOOS` direct, `amd64`→`x64`, `arm64`→`arm64`) → `ccusage` on `PATH`; none found prints `tudiff: no ccusage binary found (run npm ci or pass --ccusage)` to stderr and exits 1. Each cell runs `<ccusage> <source> <period> --json` via `exec.CommandContext` with a 120 s timeout, cwd = repo root, inherited environment. A non-zero exit or timeout is **recorded, not fatal**: it lands in the manifest (`exit_code`, `empty: true`) and the run continues; the final summary names any non-zero cells. stdout passes through redaction and is written **verbatim** — no JSON decode/re-encode, so key order, indentation, and `-0.0` survive; non-empty stderr is written to the `<period>.stderr.txt` sidecar.

#### Scenario: Non-fatal source failure
- **GIVEN** a stub ccusage that prints `{"daily":[],"totals":{"totalCost":-0.0}}` for `opencode daily --json` and exits 3 for `kimi daily --json`
- **WHEN** `capture --ccusage <stub> --sources opencode,kimi` runs
- **THEN** the run exits 0, the opencode fixture contains the literal `-0.0`, the kimi manifest entry has `exit_code: 3` and `empty: true`, and the summary line names `kimi`

### Requirement: tudiff placeholder
`tudiff placeholder [--source <s>…] [--out <dir>]` (default: all six sources in registry order; default out `harness/fixtures/_placeholder`) SHALL write deterministic, schema-derived fixtures from Go structs mirroring the observed ccusage v20 per-agent daily shapes: **codex** in its observed outlier shape (`costUSD`, a `models{}` map keyed by model name with `isFallback`/`reasoningOutputTokens`, and `reasoningOutputTokens` on entries and totals — `PlaceholderCodex`), every other source in the claude-style shape (`totalCost`, `modelBreakdowns[]`, `modelsUsed[]` — `Placeholder`); `PlaceholderFor(source)` dispatches. Each fixture has three fixed consecutive days `2026-01-05..2026-01-07`, model name `placeholder-<source>-model`, alphabetically ordered keys, 2-space indentation, `totals` equal to the column sums, and `totalTokens` equal to the sum of the four token counters in every entry and in `totals` — internally consistent so tu's aggregation can be checked against it. The `_placeholder` manifest carries `machine: "_placeholder"`, `ccusage_version: "20.0.19"`, `ccusage_path: ""`, `platform: "derived"`, and a provenance `derived_from` sentence naming the live probe the shapes came from. Every entry is `unconfirmed: true` unless its source is listed in the **confirmation ledger** `<out>/confirmed.json` (`harness.ConfirmedLedgerFile`, read by `ReadConfirmed`): a committed JSON object keyed by source name whose values are `{"machine", "date", "ccusage_version"}`. A listed source's entry is `unconfirmed: false` with that ledger object as `confirmed_by`; an unlisted source keeps `unconfirmed: true` and no `confirmed_by`. A missing ledger is an empty ledger (all entries unconfirmed). The ledger is validated before any file is written and a violation exits 1 with nothing touched: malformed JSON, a document that is not a single JSON object (a top-level `null`, or any trailing value or content after the object), a key outside `DefaultSources`, a missing `machine`/`date`/`ccusage_version`, a `date` not matching `^\d{4}-\d{2}-\d{2}$`, or an unknown field (`DisallowUnknownFields`) — each error names `confirmed.json` and the offending source/field. Ledger keys outside a `--source` subset are validated but emit no entry; the ledger changes flags only, never fixture bytes or `sha256`. The `ccusage_version` in the ledger is recorded, not compared against `PlaceholderVersion`. Per source, stdout reports the merged state as `<source> daily --json  placeholder (confirmed: <machine> <date>)  -> <out>/<file>` or `… placeholder (unconfirmed)  -> …`, read back from the manifest just written. The committed ledger lists `claude`, `codex`, `gemini`, `kimi` (confirmed on `dev-ws-sahil02`, 2026-09-16, ccusage 20.0.19); `opencode` and `copilot` stay unconfirmed until a machine with data confirms them (plan row P3b). The generator refuses (exit 1) an `--out` that already holds a real alias's manifest; a local capture alongside `_placeholder/` is expected and is never read or modified.

#### Scenario: Ledger merge
- **GIVEN** `<out>/confirmed.json` listing `codex` and no other source
- **WHEN** `tudiff placeholder --source codex --source opencode --out <out>` runs
- **THEN** the codex entry is `unconfirmed: false` with `confirmed_by` equal to the ledger object, the opencode entry is `unconfirmed: true` with no `confirmed_by`, and stdout shows `codex … (confirmed: <machine> <date>)` and `opencode … (unconfirmed)`

#### Scenario: Placeholder output is deterministic
- **GIVEN** `tudiff placeholder --source opencode --out <tmp>` run twice
- **WHEN** the two outputs are compared
- **THEN** they are byte-identical, decode into the schema structs, and `totals.totalTokens` equals the sum over `daily[].totalTokens`

### Requirement: tudiff run
`runRun(args, stdout, stderr) int` in `src/go/cmd/tudiff/run.go` implements `run` — the byte-diff matrix against the two tu binaries — behind the same `run(args, stdout, stderr) int` dispatch seam as `capture`/`placeholder`, stdlib-only. Flags (`flag.NewFlagSet("run", flag.ContinueOnError)`): `--matrix` (default `harness/matrix.json`), `--expected` (default `harness/expected-diffs.json` — the DC-keyed intentional divergences; see *Expected-diffs file*), `--node` (`dist/tu.mjs` — the TS oracle, run as `node <staged copy>`), `--go` (`bin/tu`), `--harness-bin` (`bin/harness`, the directory holding the fakes), `--fixtures <alias>[,<alias>…]` (explicit ordered alias list; overrides automatic resolution), `--placeholder` (force `_placeholder` only — what CI passes; mutually exclusive with `--fixtures`), `--report` (`bin/harness/report`; wiped at the start of a run), `--filter <substring>` (run only cases whose expanded ID contains it), `--list` (print the expanded, filtered case IDs one per line and exit 0 without touching the report dir), `--jobs <n>` (default 4; cases run concurrently, each fully isolated), `--timeout <dur>` (default 60s per side; a timeout is a red case). Relative flag *defaults* resolve against the repo root (`harness.FindRepoRoot`, walking up to `package.json`); explicit relative values resolve against the caller's cwd. Preflight checks run before any case and on the first failure print exactly one `tudiff: <reason>` line to stderr and exit 2, in order: `--fixtures` and `--placeholder` together; `--jobs < 1`; matrix unreadable or invalid (`tudiff: matrix: <loader error>`); the expected-diffs file missing (`tudiff: <path as given> not found`) or invalid (`tudiff: expected-diffs: <loader error>`) — loaded after the matrix check and after the `--list` early return, and a missing file is never treated as an empty set; `--node` missing (`… not found (run npm ci && npm run build)`); `--go` missing or not executable (`… (run just go-build)`); `--harness-bin` lacking an executable `ccusage` or `git` (`… (run just harness-build)`); `node` not on `PATH`; `script` not on `PATH` while the filtered matrix contains any `io: tty` case; `harness/fixtures/_placeholder/manifest.json` absent; a `--fixtures` alias with no `manifest.json`. `--list` runs only the matrix check. After the byte and tree comparisons, a red result is annotated with the matching expected-diffs entry's id (`Result.Expected`; green, timeout, and `harness`-channel results never carry one — a capture-level failure such as a failed exec or a missing exit sentinel is not a comparison divergence and must never be masked as expected). Exit codes: `1` iff the summary counts any unexpected red (`Red − Expected`), any timeout, any unconfirmed-fixture replay, or any stale expected entry (one that matched ≥ 1 executed case, none red) — an expected red case does not fail the run; `0` otherwise; `2` for usage/preflight errors — including a `--filter` that matches zero cases (`tudiff: --filter matched no cases`), never a vacuous 0. (489t) No argument or an unknown subcommand prints usage to stderr and exits 2.

#### Scenario: Missing Go binary is an actionable preflight error
- **GIVEN** `bin/tu` does not exist
- **WHEN** `tudiff run` is invoked
- **THEN** stderr is `tudiff: bin/tu not found (run just go-build)\n` and the exit code is 2

### Requirement: The argument matrix
`harness/matrix.json` (committed, JSON, `schema: 1`) lists **case groups** — `{"id", "args", "conf"?, "env"?, "io"?, "tz"?}` decoded with `DisallowUnknownFields`; a missing `args` key is invalid while an empty array is valid. Axis values: `conf ∈ {single, multi, org, legacy}` (which `$HOME` skeleton is staged), `env ∈ {default, nocolor, envrepo, pullfail, pushfail, dirty}` (`nocolor` sets `NO_COLOR=1`; `envrepo` sets `TU_METRICS_REPO=git@example.invalid:harness/tu-metrics.git`, flipping even `single` into multi mode through the env layer of the cascade; the last three set `TUDIFF_GIT_SCRIPT` to scripted git failure rule sets — see the child-environment requirement (lsml)), `io ∈ {pipe, tty}` (separate pipes vs. a pseudo-terminal via `script`), `tz ∈ {fixed, alt}` (`fixed` = `TZ=UTC`; `alt` = `TZ=Asia/Kolkata`, a non-integer offset and the zone the local captures were bucketed in). An omitted axis takes its base value (`single`/`default`/`pipe`/`fixed`). `LoadMatrix` rejects a schema other than 1, an empty/duplicate/non-path-component `id`, an unknown axis value, and a duplicate value within one axis — every error names the offending group id and field. `Expand` crosses each group in the nested order conf → env → io → tz; the expanded **case ID** is `<id>/<conf>/<env>/<io>/<tz>` — every axis always appears, so IDs stay stable when a group later gains an axis. `Filter` keeps cases whose ID contains the substring. The committed matrix covers the toolkit commands, snapshots for every source token, format/flag variants, windowed and full history, the `--by-machine` pivot (the original empty-state flag cases plus nine populated groups — the windowed and monthly single-tool history in table/json/csv/md, token mode, `-u all` on both, and the `snapshot-cc-by-machine-json` single-source case that pins the zero-fill quirk), the leaderboards (the baseline `lb*`/`lbh*` gate/empty-state groups plus 28 populated groups — the seeded January window in table/json/csv/md, `--by-machine` including the `-t` three-way tie, `--top 1` on both displays, `-t`, `-u other-user`, `--until`-only/`--since`-only, `cc lb`, the `m lbh` family, `lbh --full` under both time zones, the `top-snapshot` warn-and-ignore guard, and the `lb-tty` case — the leaderboard's one `io: tty` case, which exercises the mid-row bar on a real 120-column terminal), multi-mode flags, the six sync groups (`sync-cmd` on the four confs × `default`/`pullfail`/`pushfail`/`dirty`, `sync-dry-run` on the four confs × both time zones — the `tz: alt` cases are the DC-21 byte check, a UTC commit date beside local day-file dates — `sync-flag` and `cc-sync` on `single`/`multi` × `default`/`pullfail`, `cc-sync-json`, and `sync-json` — every multi-mode sync case also proving the written tree through the `tree` channel) (lsml), setup commands, usage errors, and the watch flag family (4pze) — the exit-2 incompatibilities `watch-json` (`--watch --json`), `watch-csv` (`-w --csv`) and `watch-md` (`-w --md`), the `--interval` validation cases `watch-interval-min` (`-w -i 3`), `watch-interval-max` (`-w -i 4000`) and `watch-interval-nan` (`-w -i abc`), and the DC-03 silent-acceptance cases `no-rain-without-watch` (`--no-rain`; conf single/multi × io pipe/tty) and `interval-without-watch-long` (`--interval 30`) — 159 groups expanding to 452 cases — and contains no `update` group. No live `-w` session case exists: a watch frame is wall-clock- and RNG-dependent (the elapsed counter, the countdown, `Math.random` rain), so a byte-diff between the two binaries would need an identical capture switch on both sides — a new external surface the port plan's Goal forbids, and a Go-only switch has nothing to diff against; the Go watch loop is gated instead by golden frames under a fake clock plus a fake-terminal loop test ([watch-mode](/go-port/watch-mode.md)). The file is data: coverage is reviewed at G0 and edited without code changes.

#### Scenario: Expansion order and base defaults
- **GIVEN** a group `{"id":"x","args":[],"conf":["single","multi"],"io":["pipe","tty"]}`
- **WHEN** expanded
- **THEN** the IDs are exactly, in order, `x/single/default/pipe/fixed`, `x/single/default/tty/fixed`, `x/multi/default/pipe/fixed`, `x/multi/default/tty/fixed`

### Requirement: Expected-diffs file
`harness/expected-diffs.json` (committed, JSON, `schema: 1`, sibling of `matrix.json`) declares the divergences a spec drop decision makes intentional: `{"schema": 1, "expected": [{"id": "DC-05", "cases": ["snapshot-*-csv", "…"], "reason": "…"}]}`, keyed by the spec's stable `DC-NN` ids. `LoadExpected(path)` (`src/go/internal/harness/expected.go`) decodes with `DisallowUnknownFields` plus a trailing-value rejection (the `LoadMatrix` pattern) and validates: `schema == 1`; the `expected` key present (an empty array is valid, a missing key invalid — the matrix's `args` rule); per entry an `id` matching `^DC-\d{2}$` that is unique across entries, a non-empty `cases` array whose every pattern `path.Match` accepts (an `ErrBadPattern` is a load error), and a non-empty `reason` — every error names the offending entry `id` (when known) and field. `(*Expected).Match(caseID, group)` tries entries in file order and the first matching entry wins: a pattern matches when `path.Match(pattern, caseID)` is true, and a pattern containing no `/` is additionally tried against the group name. Case IDs are `<group>/<conf>/<env>/<io>/<tz>` for `run` and the bare step name under group `live` for `live`, so `snapshot-*-csv` names whole groups, `sync-dry-run/*/*/*/alt` one axis slice, `sync-dry-run` both the matrix group and the live step of that name, and `live` every live step (`*` never crosses `/`, which is why axis-level patterns spell all five segments). A nil or empty set never matches. The committed set is empty: every `[DECIDE]` marker in `docs/specs/usage.md` § Drop at cutover is unresolved, and unresolved means keep — a later `drop` resolution is one entry here, not a code change. (489t)

#### Scenario: First matching entry wins
- **GIVEN** entries `DC-05: ["snapshot-*-csv"]` and `DC-21: ["sync-dry-run/*/*/*/alt", "sync-dry-run"]`
- **WHEN** matching `snapshot-all-csv/multi/default/pipe/fixed` (group `snapshot-all-csv`), `sync-dry-run/multi/default/pipe/alt` (group `sync-dry-run`), `sync-dry-run/multi/default/pipe/fixed` (group `sync-dry-run`), and `sync-dry-run` (group `live`)
- **THEN** the results are `DC-05`, `DC-21` (the axis-slice pattern hits the full ID), `DC-21` (the bare pattern hits the group), and `DC-21` (the bare pattern hits the live step ID)

### Requirement: Per-case $HOME staging and the seeded metrics repo
For each expanded case the driver stages **two** independent HOMEs — `<tmp>/<case>/node/home` and `<tmp>/<case>/go/home`, never sharing a path — via `StageHome(dir, variant, seedDir)`: `single` creates the directory and nothing inside it; `multi` writes `.config/tu/tu.conf`; `org` writes `.config/tu/org.conf` (no `tu.conf`); `legacy` writes `.tu.conf` (no `.config/`). The three conf files are byte-identical and exactly:

```
version = 2
metrics_repo = git@example.invalid:harness/tu-metrics.git
metrics_dir = ~/.tu/metrics_repo
machine = harness-machine
user = harness-user
auto_sync = true
```

Only the file's location varies between variants; the pinned `machine`/`user` defeat the `$HOSTNAME`/`$USER` sentinels, and the `.invalid` TLD can never resolve. For the three multi variants the committed seed `harness/metrics-repo/` is copied recursively to `<dir>/.tu/metrics_repo/`; no `.git/` is created (the metrics-dir guard is an existence check, and the fake git answers every call). The seed follows the metrics-repo layout spec (`<user>/<year>/<machine>/<tool>-<date>.jsonl`, one `UsageEntry` JSON line each, `label` equal to the filename's date) on the placeholder dates 2026-01-05..07 with the placeholder token counters: two profiles (`harness-user`, `other-user`) across two machines so `lb`/`lbh`/`-u` cases have rows; the own-machine cc day-files cost `0.25` and `0.75`, straddling the placeholder fixtures' `0.5` so both arms of the own-machine max-merge (live wins / stored wins via the never-shrink guard) are exercised; `docs/README.md` documents the tree and is never scanned as a user (the spec excludes `docs/`).

#### Scenario: Legacy variant staging
- **GIVEN** variant `legacy` staged into an empty dir
- **WHEN** `StageHome` returns
- **THEN** `<dir>/.tu.conf` exists with the bytes above, `<dir>/.config` does not exist, and `<dir>/.tu/metrics_repo/harness-user/2026/harness-machine/cc-2026-01-06.jsonl` exists

### Requirement: Oracle staging and the from-scratch child environment
Once per run, `StageOracle` copies the TS oracle into `<tmp>/oracle/dist/`: `tu.mjs`, `tu.default.conf` beside it (the bundled layout `findDefaultConf` checks first, exactly as in the brew bottle), and the fake `ccusage` at `vendor/ccusage/bin/ccusage` with mode 0755 (the fixed slot the TS fetcher execs). The repository's `dist/` is never written; the Go binary runs in place. `BuildEnv` constructs the child environment from scratch — exactly `PATH=<abs harness-bin>:<harness process's PATH>` (fakes first; `node` still reachable), `HOME=<side's staged home>`, `TZ` per the axis, `LANG=C.UTF-8`, `LC_ALL=C.UTF-8`, `TUDIFF_FIXTURES=<resolved alias dirs, absolute, OS path list>`, `TUDIFF_CALL_LOG=<report>/cases/<case>/<side>.calls.jsonl`, plus `TERM=xterm-256color` only for `io: tty`, `NO_COLOR=1` only for `env: nocolor`, and `TU_METRICS_REPO` only for `env: envrepo`. No other variable from the parent environment is present, so an exported `TU_METRICS_REPO` or `NO_COLOR` in the developer's shell cannot tilt a case. Each side's working directory is `<tmp>/<case>/<side>/` (a sibling of its `home`), never the repo root. `TUDIFF_GIT_SCRIPT` is appended by `BuildEnv` (in `internal/harness/diff.go`, beside `StageOracle`) only for the three scripting env values, from the `gitScripts` table whose JSON text is part of the byte contract (lsml):

| `env` | `TUDIFF_GIT_SCRIPT` | Exercises |
|-------|--------------------|-----------|
| `pullfail` | `[{"match":["pull"],"stderr":"fatal: couldn't find remote ref main\n","exit":1}]` | the pull-failure line with Node's error text, the follow-up `rebase --abort` call, exit 1 / `sync failed — using local data.` |
| `pushfail` | `[{"match":["push"],"stderr":"error: failed to push some refs\n","exit":1}]` | the single retry (two `push` calls) and the post-retry warning line |
| `dirty` | `[{"match":["status","--porcelain"],"stdout":" M harness-user/x\n","exit":0}]` | the commit path and its `commit -m "# harness-user: update {UTC date}"` argv |

For every other env value the variable is unset and the fake git is the silent exit-0 stub for every call.

### Requirement: Pipe and TTY execution
`io: pipe` runs each side with stdin `/dev/null` and stdout/stderr captured into separate byte buffers, bounded by `--timeout` via `exec.CommandContext`; a deadline records `TimedOut` with exit −1. `io: tty` runs each side under `script(1)` with the wrapper `stty cols 120 rows 40; <cmd> <args…>; printf '\n__TUDIFF_EXIT=%s\n' "$?"` (arguments shell-quoted), invoked as `script -q -e -c "<wrapper>" /dev/null` on util-linux and `script -q /dev/null sh -c "<wrapper>"` on BSD; the flavour is detected once per run (`script --version` succeeding → util-linux) and recorded in the report header. The merged transcript — the pty's `\r\n` endings kept verbatim — is the single `tty` channel; the trailing `__TUDIFF_EXIT=<n>` sentinel (with its preceding line break) is parsed into the exit code and removed before comparison, and a missing or malformed sentinel yields exit −1 with the capture error `no exit sentinel`, never a false green.

#### Scenario: Sentinel parsing
- **GIVEN** a transcript ending `…table\r\n\r\n__TUDIFF_EXIT=2\r\n`
- **WHEN** parsed
- **THEN** the exit code is 2 and the `tty` channel ends with `…table\r\n`

### Requirement: Fixture resolution (D7) and unconfirmed flagging
`ResolveFixtures(root, explicit, placeholderOnly, hostname)` returns the ordered alias directories: an explicit `--fixtures` list wins (each alias must hold a `manifest.json`, else preflight exit 2); `--placeholder` forces `_placeholder` alone; otherwise `harness/fixtures/<hostname>/` precedes `_placeholder` when its manifest exists, else `_placeholder` alone. The resolved aliases are printed in the report header (`fixtures: dev-ws-sahil02, _placeholder` / `fixtures: _placeholder`). After each case, `UnconfirmedReplays` reads both sides' call logs and, for every line with a non-empty `matched` (`<alias>/<file>`), looks the file up in that alias's manifest: a case that replayed any `unconfirmed: true` fixture is flagged in the report as a separate marker (`[unconfirmed]` on the case line, counted in the summary), never a colour — the "harness reports them separately" clause of D7 — and any unconfirmed replay fails the run (exit 1) under the gate rule (489t).

### Requirement: Comparison — first divergence per case
`Compare` byte-diffs channels in order and stops at the first difference: pipe cases compare `exit`, `stdout`, `stderr`; tty cases compare `exit`, `tty`. Each side's byte channels are home-normalized before comparing: `NormalizeHome(b, home)` replaces every occurrence of the side's staged `$HOME` path in `b` with the literal `$HOME` (byte replacement, no regexp; an empty home is a no-op — distinct from `Redact`, which anonymizes the developer's real home for the committed fixture corpus). The two staged homes differ only in the side segment, and the Goal-frozen absolute-path messages (the setup commands' `Already initialized`/`Cloned` lines, the version warning's conf path) legitimately embed `$HOME`, so unnormalized captures would diverge even when both binaries are correct. `SideCapture.Home` carries each side's staged home (set by `runCase`); the divergence offset, 1-based line (counting `\n` in the node capture — the oracle side is the line-numbering reference), and `strconv.Quote` excerpts of at most 40 bytes are computed on the normalized bytes. Every compared channel identical → **green**; a timeout on either side → **timeout** (channel `timeout`), counted as red for the exit code but listed separately in the summary. Otherwise the case is **red** with `Channel` naming the first differing one; an exit divergence records the two codes. The call logs are compared separately as sorted sets of `{tool, argv}` pairs — `CompareCallLogs(nodePath, goPath, nodeHome, goHome)` normalizes each side's argv entries with that side's staged home before forming the set, so the `-C <dir>` and `clone <url> <dir>` shapes compare equal (`cwd` differs by side by construction; `matched` is alias metadata) — into `CallsDiffer`/`NodeCalls`/`GoCalls` — **informational only**: a difference appends `[calls differ: node=<n> go=<n>]` to the case line and never reddens a case.

#### Scenario: First divergence on stdout
- **GIVEN** node stdout `"Usage: tu\n"` and go stdout `""` with equal exit codes
- **WHEN** compared
- **THEN** the case is red on channel `stdout` at offset 0, line 1, with node excerpt `"Usage: tu\n"` and go excerpt `""`

### Requirement: The tree channel compares the written metrics repo
After the byte comparison, a case that is still green is checked by `harness.CompareTrees(nodeHome, goHome)` (wired into `runCase` after `Compare`): the two sides' `<home>/.tu/metrics_repo/` trees — the set of slash-separated relative file paths and each file's bytes (day-file content carries no home path, so no normalization) — plus the **presence** of `<home>/.tu/.last-sync` (its content is a wall-clock timestamp, so presence only). Nothing under `.tu/cache/` is compared (the two cache formats differ by design). A difference reddens the case with `Channel: "tree"` and the first differing relative path (or `.last-sync`) in `NodeExcerpt`/`GoExcerpt` (present/absent or a short byte excerpt); a case already red keeps its first channel. The report renders the `tree` channel like any other red channel. Every multi-mode data case thereby also proves the write — a `--json`-style float or key-order slip in a written day-file becomes visible. (lsml)

#### Scenario: A day-file byte divergence is red on the tree channel
- **GIVEN** a multi-mode data case where the Go side wrote a day-file with different bytes
- **WHEN** `tudiff run` compares it
- **THEN** the case is red on channel `tree` naming that path; identical trees with `.last-sync` on both sides stay green

### Requirement: tudiff live — real-git parity
`tudiff live [--node] [--go] [--turepair] [--harness-bin] [--report] [--expected] [--keep]` (`src/go/cmd/tudiff/live.go`, run via `just go-live`) runs the real-git half of the sync gate where `run` answers every git call with the fake. Preflight: real `git` on `PATH`, `node`, the oracle bundle, both Go binaries, and the fake `ccusage`, then the expected-diffs file is loaded with the same flag, resolution rule, and preflight messages as `run`; the child `PATH` is a livebin holding only the fake `ccusage` followed by the process `PATH` — the fake git is deliberately absent. Seeding: one bare repo (`git init --bare --initial-branch=main`); a scratch clone receives `harness/metrics-repo/` in one `seed` commit and pushes; the bare is then copied into two identical remotes, never shared; each side's `multi` staged home (via `harness.StageHome`) has its `.tu/metrics_repo` replaced by a real clone of its own bare. Every child carries a pinned identity (`GIT_AUTHOR_NAME/EMAIL`, `GIT_COMMITTER_NAME/EMAIL`, `GIT_CONFIG_GLOBAL=/dev/null`, `GIT_CONFIG_NOSYSTEM=1`, `commit.gpgsign=false` via `GIT_CONFIG_COUNT/KEY_0/VALUE_0`) and FIXED `GIT_AUTHOR_DATE`/`GIT_COMMITTER_DATE` (`2026-01-09T12:00:00Z`), so identical trees yield identical commit hashes. Seeding and comparison reuse the exported `harness.CopyTree`/`CopyFile`/`Excerpt` helpers.

The nine steps run on both sides with `TZ=UTC` and the same midnight-rollover re-run rule as `runCase`; each step's stdout/stderr/exit is compared with `harness.Compare` after home normalization: `sync --dry-run` (then both trees provably unchanged — the no-mutation guarantee), `sync` (then day-file trees byte-equal, `git status --porcelain` empty on both, `git log --format=%H%n%s -p main` byte-identical including hashes, `.last-sync` present on both), a second `sync` (the steady-state no-commit path; the log unchanged), a foreign commit pushed to each bare then `sync` with a one-off fixture alias raising cc `2026-01-07` (both sides' `.tu/cache` wiped first — the cache key excludes `TUDIFF_FIXTURES`, so without the wipe the earlier steps' warm cache would hide the alias's raised value; the sync then commits the local change and `pull --rebase` integrates; the logs identical), a fabricated `.git/rebase-merge` then `sync` (the `Warning: recovering from interrupted rebase` line; the abort's failure swallowed), a broken `origin` then `sync` (the pull-failure bytes carrying real git's stderr verbatim under the `$HOME`-normalized `git -C $HOME/.tu/metrics_repo pull --rebase origin main` line, then `Error: sync failed — check network and remote config.`, exit 1), and `cc --sync` (`syncing metrics... synced.` then the table). Then the repair parity flow: the shrunk-history fixture seeded in one repo, copied twice, `node scripts/repair-metrics.mjs --repo <A>` vs `bin/turepair --repo <B>` compared after replacing each side's repo path with `$REPO` — dry-run, then both with `--write`, outputs and trees compared again. The report lands under `--report` (default `bin/harness/report-live/`) in the `run` report's shape (per-step lines, first divergence, summary); a red step is annotated with the matching expected-diffs entry exactly as in `run`, and the exit-code rule is `run`'s (exit 1 iff unexpected + timeout + unconfirmed > 0 or any entry is stale). CI runs it as the `tudiff` job's second step, and that job gates `ci-gate` — see [toolchain](/build/toolchain.md) (lsml, 489t).

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

### Requirement: Date-rollover re-run and case concurrency
Neither side takes a clock injection, and snapshot displays are "today". The driver records the local calendar date in the case's TZ immediately before the node side runs and immediately after the go side finishes; if they differ, both captures are discarded and the case re-runs exactly once (`rerun: true` in `report.json`), so a midnight rollover cannot produce a false red; a second mismatch is reported as-is. Cases execute through a worker pool of `--jobs` goroutines; each case owns its temp tree (both HOMEs, both working directories) and its report directories, so parallelism is safe, and report lines stream in matrix order regardless of completion order (a flush cursor prints contiguous completed results).

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
`src/go/internal/harness/corpus_test.go` SHALL walk `../../../../harness/fixtures/*/manifest.json` (Go tests run with cwd = package dir — the committed `_placeholder/` plus any local capture present on the machine) and, for every fixture entry, assert: the file exists; its `sha256` matches; when `exit_code == 0` the content parses as JSON with a `daily` array whose length equals `days`, and `first_date`/`last_date`/`empty` agree with the content; no string value contains `/home/` or `/Users/`; `unconfirmed: true` and `confirmed_by` occur only under the `_placeholder` alias; and, under `_placeholder`, the manifest agrees with `confirmed.json` — every listed source is `unconfirmed: false` with `confirmed_by` equal to its ledger object, every unlisted source is `unconfirmed: true` with none, and every ledger key has a `daily` fixture (a malformed ledger fails the test; each mismatch message ends `re-run tudiff placeholder`). The test MUST fail (not skip) when the fixtures directory is absent. It runs in the existing `go-build-and-test` CI lane through `just go-test` — no CI workflow edit.

#### Scenario: Tampered fixture fails on sha256
- **GIVEN** the committed placeholder corpus
- **WHEN** one byte of a fixture is edited without regenerating the manifest
- **THEN** `just go-test` fails the corpus test on the `sha256` assertion

### Requirement: TS-side staging of the fake ccusage
The TypeScript fetcher does NOT look `ccusage` up on `PATH` — it execs the fixed path `dist/vendor/ccusage/bin/ccusage` when `dist/vendor/` exists, else `node_modules/.bin/ccusage`. Staging the fake for the TS side therefore means copying the self-contained `bin/harness/ccusage` binary to `<staged-dist>/vendor/ccusage/bin/ccusage` (what `StageOracle` does into `<tmp>/oracle/dist/`); the Go side resolves vendor-first relative to `os.Executable()` then `PATH`. The fake git needs no staging — `PATH`-first placement suffices because both sides reach `git` through `PATH`.

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
**Decision**: An exit-0 `daily: []` capture is `empty: true, unconfirmed: false`; placeholders are the only `unconfirmed: true` entries, and `confirmed_by` appears only on placeholder entries a human has confirmed — a real capture is confirmed by being real, not by the ledger.
**Why**: "We observed nothing" and "we guessed the shape" are different facts; the harness must report the second separately.
**Rejected**: Marking empties unconfirmed (conflates the two).
*Introduced by*: 260915-r7dh-harness-fixture-capture

### Confirmation is a committed ledger merged by the generator, never a manifest edit
**Decision**: Human confirmation of a placeholder shape lives in `harness/fixtures/_placeholder/confirmed.json`; `WritePlaceholders` merges it into `manifest.json` (`unconfirmed: false` + `confirmed_by`), and the corpus test asserts the two agree.
**Why**: `manifest.json` is generated and rewritten wholesale, so a hand flip would be lost on the next regeneration and the `sha256` guard (which pins fixture bytes, not the manifest) cannot see it. A separate human-owned input keeps the manifest a pure function of fixtures plus ledger, so regeneration is idempotent with respect to confirmations and a stale pair fails CI.
**Rejected**: Hand-editing the manifest and never regenerating; encoding confirmation in the corpus-wide `derived_from` string (cannot carry per-source state, also overwritten); `--confirmed` flags on `tudiff placeholder` (state would live in shell history, not the repo); bumping `schema` to 2 (the field is optional and additive; real-capture manifests and the fake ccusage are unaffected).
*Introduced by*: 260916-di0s-placeholder-confirmation-ledger

### Ledger errors abort before any write
**Decision**: `ReadConfirmed` runs before the first file write in `WritePlaceholders`, and any error (malformed JSON, unknown source, missing or malformed field, unknown field) aborts with nothing touched.
**Why**: The ledger is hand-edited; a typo is the realistic failure, and half-written output would be worse than a loud refusal. Matches the existing all-or-nothing real-alias refusal.
**Rejected**: Warn-and-continue (the misspelled source silently stays unconfirmed, which is exactly the state the ledger exists to record).
*Introduced by*: 260916-di0s-placeholder-confirmation-ledger

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

### JSON case groups with axis defaults, not a cross-product
**Decision**: `harness/matrix.json` lists case groups; omitted axes take one base value.
**Why**: A full product of 4×3×2×2 over ~90 argv lines is thousands of cases; explicit axes keep CI to minutes and make G0's coverage review a read of one file.
**Rejected**: A line-oriented text file (no per-line axis control without inventing syntax); a full cross-product with a `--sample` flag (nondeterministic coverage).
*Introduced by*: 260916-i9hc-harness-differential

### Two fresh $HOMEs per case
**Decision**: Each side gets its own staged skeleton.
**Why**: The TS side writes `~/.tu/cache` and multi-mode day-files; a shared HOME would let the Go side read TS state and mask divergences or alter its call sequence.
**Rejected**: One HOME with `--fresh` on every case (changes the matrix's meaning); running Go first (same problem mirrored).
*Introduced by*: 260916-i9hc-harness-differential

### Compare-side home normalization
**Decision**: Each side's staged home path is replaced with the literal `$HOME` in the stdout/stderr/tty captures and the call-log argv before comparison, and the report's per-side capture files hold the normalized bytes.
**Why**: Two fresh homes per case (the decision above) is still right, and the setup commands' absolute-path messages and the git argv shapes (`-C <dir>`, `clone <url> <dir>`) are Goal-frozen surfaces that legitimately embed `$HOME` — without normalization those cases could never go green.
**Rejected**: A shared home (masks divergences through shared cache and day-files); staging both sides at the same absolute path via a symlink (fragile across `script`/tty and still leaks the side's real dir through `cwd`).
*Introduced by*: 260916-4fs0-config-and-setup-commands

### Call log is informational
**Decision**: `calls` differences are printed but never redden a case.
**Why**: The TS fetcher issues its per-tool ccusage calls via `Promise.all`; order is nondeterministic and a sorted comparison still can't distinguish "Go hasn't fetched yet" from "Go fetched differently" while every case is red.
**Rejected**: Compared channel (flaky now); dropping it (loses the call-log seam's value for V1).
*Introduced by*: 260916-i9hc-harness-differential

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

### Git failure scripting rides the `env` axis
**Decision**: `pullfail`/`pushfail`/`dirty` are `env` values setting `TUDIFF_GIT_SCRIPT`.
**Why**: The axis means "which extra environment variable is set"; a new axis would rename every existing case ID.
**Rejected**: A fifth `git` axis.
*Introduced by*: 260916-lsml-sync-metrics-writer

### Expected diffs are a committed data file keyed by spec DC id
**Decision**: `harness/expected-diffs.json` (schema 1) lists entries `{id, cases, reason}` keyed by the spec's stable `DC-NN` IDs; both `tudiff run` and `tudiff live` load it via `--expected`.
**Why**: A `drop` resolution at G0/G3 must become one data entry, not a change to the gate's code; the matrix already models coverage as reviewed data, and DC IDs are declared stable in the spec.
**Rejected**: An `expected` field on matrix groups (a drop spans groups and is keyed by spec id, not group id); a `--expect` CLI flag (state would live in CI YAML, not beside the matrix).
*Introduced by*: 260918-489t-harness-full-matrix-gate

### A missing expected-diffs file is a preflight error, never an empty set
**Decision**: `tudiff run`/`live` exit 2 when the file is absent or invalid.
**Why**: The harness never reports a vacuous green (`--filter` matching nothing and a missing placeholder manifest are already exit 2); the file is committed, so the default always exists.
**Rejected**: Treating absence as `expected: []` (a mis-resolved path would silently drop every expectation and turn intended reds into gate failures, or worse, hide a stale set).
*Introduced by*: 260918-489t-harness-full-matrix-gate

### Stale entries fail, unmatched entries only report
**Decision**: An entry that matched at least one executed case and none red exits 1 with a `(stale)` line; an entry that matched zero executed cases prints `(no executed case)` and does not affect the exit code.
**Why**: An always-green expectation is a misconfiguration worth failing on; `--filter` and `live` legitimately exclude cases, so zero-matched must not fail, and this keeps `run` and `live` decoupled (no cross-runner case list).
**Rejected**: A preflight cross-check that every entry matches some case in the full matrix (breaks `live`-only entries and `--filter` runs).
*Introduced by*: 260918-489t-harness-full-matrix-gate
