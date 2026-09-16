---
type: memory
description: The Go port's command edge — internal/command (Request, byte-exact Parse, exit codes, the pure Normalize guards and 3-month cap, Run composing snapshot and history through one GroupBy(Tool, Date) into a Result carrying Notices, the ErrUnported placeholder row map) and cmd/tu as the only writer (setup-command dispatch, Load warnings, reserved-user guard, x/term width probe, notices→warnings→lines write order); config cascade: config-and-setup — 134 harness cases green
---
# Command Edge and Entry Point (Go port)

**Domain**: go-port

## Overview

`internal/config` owns the full config cascade and the setup commands ([config-and-setup](/go-port/config-and-setup.md)); `internal/command` parses the complete tu argument grammar into a `Request` and composes the pipeline — the input layer ([fact-and-sources](/go-port/fact-and-sources.md)) through the middle layers ([query-view-render](/go-port/query-view-render.md)) — into a `Result`; `cmd/tu` is the only writer. For fetching, `command` imports `source` and `fact` and no `source/*` adapter; `cmd/tu` is the only package that names `ccusage`. The binary answers the single-mode snapshot and history grammar (with `--since`/`--until`/`--full` and the table, JSON, CSV and Markdown formats) and the setup commands `init-conf`, `init-metrics`, and `status` for real; every recognized-but-unported request keeps the scaffold's placeholder.

## Requirements

### Requirement: Config cascade
`internal/config` owns the full `tu.conf`/`org.conf`/legacy cascade — path resolution from `$HOME` only, the embedded shipped defaults, `Load(p, env, ov) (Config, []string)` merging defaults ∪ org ∪ user ∪ env ∪ CLI overrides and returning the cascade warning lines, and the setup commands. See [config-and-setup](/go-port/config-and-setup.md).

### Requirement: Request, Flags, UsageError, ShortUsage
`command` defines `Display` (`Snapshot`/`History`/`Leaderboard`/`LeaderboardHistory`), `Format` (`Table`/`JSON`/`CSV`/`Markdown`), `Metric` (`Cost`/`Tokens`), `Flags` (the booleans `Fresh, NoColor, Watch, Sync, DryRun, ByMachine, Full, NoRain, SkipBrewUpdate`; `Interval` defaulting to 10 and validated only when `Watch`; `User`/`Since`/`Until`; `Metric`; `Top` with 0 = unset), and `Request{Source, Period, Display, Format, Flags, Command, Args, Version}`. `Source` is `""` for all tools, else a registry key with aliases resolved; `Command` holds the first positional when it is a non-data command; `Args` carries the positionals after `Command` (nil for data commands) (4fs0). `command` exports the exit-code constants `ExitOK = 0`, `ExitOperational = 1`, `ExitUsage = 2` — the shll toolkit convention: 0 = success (including benign no-ops and warn-and-continue guards), 1 = operational failure (a well-formed invocation that could not complete; also the placeholder), 2 = usage error; `cmd/tu` returns only these values (4fs0). `UsageError{Message, ShowUsage}` means exit 2 with the message on stderr; `ShowUsage` appends `ShortUsage`, the byte-exact TS constant:

```
Usage: tu [source] [period] [display]

  tu                Today's cost, all tools
  tu cc             Today's cost, Claude Code
  tu mh             Monthly cost history, all tools
  tu -h             Show full help

Run 'tu help' for all commands.
```

### Requirement: Parse implements the complete TS grammar
`Parse(args) (Request, *UsageError)` is `parseGlobalFlags` + the version/non-data checks + `parseDataArgs`, in the TS order, so the same argv produces the same first error:

1. **Flag pass** over args in order. Sixteen boolean flags are stripped: `--json`, `-j`, `--csv`, `--md`, `--sync`, `--dry-run`, `--fresh`, `-f`, `--watch`, `-w`, `--no-color`, `--no-rain`, `--by-machine`, `--full`, `--skip-brew-update`, `-t`. Value flags consume the next token only when it qualifies — `^\d+$` for `--interval`/`-i`; a token not starting with `-` for `--user`/`-u`, `--since`/`-s`, `--until`, `--metric`, `--top` — and the flag is remembered as present either way. Everything else lands in the positional list, including `--help` when it is not the first token and unknown flags like `--bogus`.
2. **Validation, in this order, first failure wins** (message on stderr, exit 2, no usage block): `--interval` only when `--watch` — missing/non-numeric `Error: --interval requires a numeric value`; `< 5` `Error: --interval minimum is 5 seconds`; `> 3600` `Error: --interval maximum is 3600 seconds`. Then the six format conflicts in order — watch+json, json+csv, json+md, csv+md, watch+csv, watch+md — each `Error: {a} and {b} are incompatible` (`-j` counts as `--json`; the message always says `--json`). Then `-u` present without value: `Error: -u requires a username`. Then `--since`/`--until` shape — `YYYY-MM-DD` or `YYYYMMDD`, the compact form normalized to dashed — else `Error: --since requires a date (YYYY-MM-DD or YYYYMMDD)` (and the `--until` twin); shape-only, so `2026-13-01` parses successfully. Then both present and `since > until`: `Error: --since must be on or before --until`. Then `--metric` not `tokens`/`cost`: `Error: --metric requires 'tokens' or 'cost'`. Then `-t` with an explicit `--metric cost`: `Error: -t and --metric cost are incompatible` (`-t` alone or with `--metric tokens` sets `Metric = Tokens`). Then `--top` not a positive integer: `Error: --top requires a positive integer`. Format precedence is json > csv > md > table.
3. **Version after validation**: `--version`, `-V` or `-v` anywhere in the original args sets `Request.Version` — `tu --json --csv --version` is exit 2, not a version line.
4. **Help first**: a first positional in {`help`, `-h`, `--help`} sets `Request.Command` and returns — the help check precedes the `--dry-run` guard, so `tu help --dry-run` parses as help (4fs0).
5. **`--dry-run` misuse guard**: when `Flags.DryRun` and the first positional is not `sync` (including no positional), the error is `UsageError{Message: "Error: --dry-run is supported only with 'tu sync' — run 'tu sync --dry-run' to preview a sync.", ShowUsage: false}` — exit 2 with no usage block. `tu sync --dry-run` parses to `Command == "sync"` with `DryRun` set and no error (4fs0).
6. **Non-data commands**: a first positional in {`init-conf`, `init-metrics`, `sync`, `status`, `update`, `shell-init`, `help-dump`, `skill`} sets `Request.Command`, and the positionals after it become `Args` (nil when none). Only for `init-metrics`: `len(Args) > 1` → `UsageError{Message: "Error: init-metrics takes at most one argument (repo-url)", ShowUsage: true}` — message line, then `ShortUsage`, exit 2, raised before `$HOME` is consulted. Data flags on a setup command do not error: `status --json` sets both `Command` and `Format` (4fs0).
7. **Positionals**: a first token in `cc codex co oc gemini gem copilot cop kimi ki all` is the source (aliases `co→codex`, `gem→gemini`, `cop→copilot`, `ki→kimi`; `all` → `""`). Each remaining token: `d`/`daily`, `w`/`weekly`, `m`/`monthly` set Period; `h`/`history`, `lb`, `lbh` set Display; `dh`/`wh`/`mh` set Period and History together. Anything else — a second source, a source after a period, an unknown word or flag, `--help` after a positional — is `Unknown argument: {tok}` with `ShowUsage: true` (message line, then `ShortUsage`), exit 2.

#### Scenario: Byte-exact first errors
- **GIVEN** argv `--json --csv`, `-t --metric cost`, `cc codex`, `cc --help`, `--bogus`
- **WHEN** parsed
- **THEN** the messages are `Error: --json and --csv are incompatible`, `Error: -t and --metric cost are incompatible`, `Unknown argument: codex`, `Unknown argument: --help`, `Unknown argument: --bogus` (the last three with `ShowUsage`)

#### Scenario: Guard order — version and help win
- **GIVEN** argv `--dry-run --version`, `help --dry-run`, `sync --dry-run`, `init-metrics a b`, `status --json`
- **WHEN** parsed
- **THEN** the first sets `Version`; the second sets `Command == "help"`; the third sets `Command == "sync"` with `DryRun` and no error; the fourth is the arity usage error with `ShowUsage`; the fifth sets `Command == "status"` and `Format == JSON` with no error

### Requirement: --interval without --watch is parse-inert
A bare `--interval N`/`-i N` without `--watch` is dropped at parse level: the numeric value is consumed and discarded, validation is watch-gated, and the snapshot renders normally — exactly as the TS behaves (harness case `interval-unsupported` is green). A non-numeric value is NOT consumed and lands in the positional list, so `tu --interval abc` (no `--watch`) is `Unknown argument: abc`, exit 2. The rows that port watch MUST NOT turn a bare `--interval N` into an error or a placeholder. (3am6)

### Requirement: Normalize applies the warn-and-clear guards and the cap
`Normalize(req, now) (Request, []string, capActive bool)` in `guards.go` is the pure guard step — it returns the request the pipeline runs plus the stderr notice lines the edge prints BEFORE any fetch warning. In TS order (B4 inserts its `--by-machine` pivot guard before step 1; B5 inserts `--top` after step 2):

1. `Since`/`Until` set on a non-history display → notice `Warning: --since/--until apply to history display — ignoring.`; both cleared (so the snapshot is in scope after the clear and a well-shaped impossible date like `2026-13-01` warns-and-renders with exit 0 — harness case `since-invalid`).
2. `Full` on a non-history display → notice `Warning: --full applies to daily/weekly history — ignoring.` (the flag is left set; nothing reads it downstream).
3. **Cap**: `Display == History && Period != Monthly && Since == "" && Until == "" && !Full` → `Since = query.ThreeMonthFloor(now)`, `capActive = true`. An explicit bound on either side disables the cap entirely (no intersection); `mh --full` is a silent no-op; `--full` with an explicit window is silently accepted.

`lb`/`lbh` are excluded from these guards in the TS; they are B5's and remain placeholder. The single-mode `-u` (B3), `--by-machine` (B4) and `--top` (B5) warn-and-clear guards land with their rows — until then those flags take the placeholder path and their single-mode harness cases stay red. (3am6, 9ax5)

### Requirement: Run composes the in-scope request, else ErrUnported
In scope (evaluated on the **normalized** request): `Display ∈ {Snapshot, History}`; `Format` Table, JSON, CSV or Markdown; `Source` any tool or all; any period; flags limited to `--json`/`-j`, `--csv`, `--md`, `--fresh`/`-f`, `--no-color`, `-t`, `--metric`, `--since`/`-s`, `--until`, `--full`; single mode; no non-data command; not `Version`. Anything else — multi mode, `User`, `ByMachine`, `Top`, `Watch`, `Sync`, `DryRun`, `NoRain`, `SkipBrewUpdate` — returns `ErrUnported` without fetching and without printing the notices (the TS prints them and then does the unported thing; no harness case combines them — B4/B5 inherit the ordering when they land). For an in-scope request, `Run(ctx, req, mode, deps)`:

1. Normalizes: `req, notices, capActive := Normalize(req, deps.Now())`; then the scope check; then `context.WithTimeout(ctx, source.DefaultTimeout)` once, for all tools.
2. Fetches **daily only** (`source.PeriodDaily`, no extra args — roll-up is client-side): `FetchAll` for all tools, `Fetch(tool)` for a single source, with `fresh = Flags.Fresh`. `Fetcher` is the seam — its `Fetch` takes `fact.Tool`; `*ccusage.Source` satisfies it, asserted at the `cmd/tu` assignment.
3. **Snapshot** — computes `cur = query.CurrentLabel(Period, deps.Now())` and groups `GroupBy(Window(RollUp(recs, Period), cur, cur), Tool)`; builds `[]view.ToolTotals` over registry order (all six tools, or the one source) with `Name` from `fact.Lookup` and `Label = cur` when a group matched, else `""`; clears every `Label` when `Source == "" && Period == Daily` — the TS single-mode daily-all path (`fetchAllTotals`) returns bare totals, so `tu --json` never carries `"label"` while `tu cc --json` (any period) and `tu m --json` / `tu w --json` do (3am6). Renders by format: `json.Snapshot(rows)`, `csv.Snapshot(rows)`, `markdown.Snapshot(rows, Period)`, or `ansi.Table(view.Snapshot(rows, Period), deps.Colors)`.
4. **History** — computes `recs = query.RollUp(query.Window(recs, Since, Until), Period)` (window before roll-up, so a mid-week window yields a leading partial week labeled by its Sunday) and builds registry-ordered `[]view.Series` from ONE `query.GroupBy(recs, query.Tool, query.Date)` pass — each group's `Key.Tool` selects the series, `Key.Date` the entry label; entries stay ascending because `RollUp` sorted and `GroupBy` preserves first-seen order. No per-tool aggregation loop. Renders by source count and format with `opts = view.HistoryOptions{Period, Now: deps.Now(), Width: deps.Width, CapActive: capActive, Metric}` and nil `Prev`: single source → `view.History(s, opts)` / `json.History(s)` / `csv.History(s)` / `markdown.History(s, period, capActive)`; all tools → `view.TotalHistory(series, opts)` / `json.TotalHistory(series)` / `csv.TotalHistory(series)` / `markdown.TotalHistory(series, period, capActive)`; tables encode through `ansi.Table(…, deps.Colors)`.
5. Returns `Result{Lines, Notices, Warnings, TotalCost, TotalTokens, CostByItem}` — `Notices` are the guard lines from step 1 (the edge writes them before the source warnings); `Warnings` are the source errors the edge writes via `source.WriteWarnings`; `TotalCost`/`TotalTokens` sum the rows (history: every entry); `CostByItem` maps display name to cost on the snapshot, and `{Name}:{label}` plus `total:{label}` on history, valued in the display metric (the TS `buildCostMap` scheme).

Caching is uniform: every path goes through `Source{Cache: cache.Default()}` and `--fresh` skips the read everywhere. The TS daily-all cache bypass (bare `tu`/`tu --json` never touch `~/.tu/cache` and `--fresh` is a no-op there) is deliberately NOT reproduced — the cache is not a Goal-listed external surface, the harness cannot observe it, and Constitution Principle IV wants heavy operations cached; it is recorded as a drop-at-cutover candidate for gate G0. (3am6)

### Requirement: Placeholder policy and the row map
A recognized-but-unported request prints `tu: not implemented (Go port in progress)` on stderr, stdout empty, exit 1 — keeping those harness cases red with the signature the harness already diffs. Ownership of each unported surface: multi mode (non-empty `metrics_repo` or `TU_METRICS_REPO`) and `--user`/`-u` → B3; `--by-machine` (including its warn-and-clear on the all-tools pivot, which slots into `Normalize` ahead of the since/until guard) → B4; `lb`/`lbh` and `--top` → B5; the `sync` command, `--sync`, and `tu sync --dry-run` → B6; `--watch`/`-w`, `--interval`, `--no-rain` (and the live `Prev` delta maps, `maxRows` truncation, compact layouts) → B7; `help`/`-h`/`--help` (first arg), `help-dump`, `skill`, `shell-init`, `update`, `--skip-brew-update` → B8. Implemented for real: the single-mode snapshot (3am6), the single-mode history displays, windows, `--full`, the CSV/Markdown encoders and both history JSON shapes (9ax5), the setup commands `init-conf`/`init-metrics`/`status` and the `--dry-run` misuse guard (4fs0).

### Requirement: cmd/tu run() write order and exit codes
`run(args, stdout, stderr) int` is the only writer — nothing below `cmd/tu` touches stdout/stderr or calls `os.Exit` — in the TS `main()` order: (1) `command.Parse` — a usage error prints the message, then `ShortUsage` when `ShowUsage`, exit `ExitUsage`; (2) `Version` → the version line on stdout, exit `ExitOK`; (3) `Command != ""` → `runCommand`: `init-conf`/`init-metrics`/`status` are answered for real (paths resolved first — `ErrNoHome` → its byte-exact message, `ExitOperational`; the per-command handlers and the clone execution are in [config-and-setup](/go-port/config-and-setup.md)); every other command → placeholder, `ExitOperational`; (4) `config.ResolvePaths(os.Getenv("HOME"))` error → its message, `ExitOperational` (parse precedes the HOME check: `tu bogus` with HOME unset still exits 2); (5) `config.Load(paths, env, Overrides{})` — its cascade warnings print on stderr before anything else; (6) the reserved-user guard — `cfg.User == "all"` → `Error: config user "all" is reserved (used by -u all)` on stderr, `ExitUsage` (B3's metrics-dir guard slots between Load and this check when it lands); (7) deps — `&ccusage.Source{Cache: cache.Default()}`, `Now: time.Now`, `ansi.Colors{Enabled: !NoColor && os.Getenv("NO_COLOR") == ""}`, `Width: terminalWidth(stdout)` — `terminalWidth` returns `term.GetSize`'s column count when stdout is an `*os.File` whose descriptor `term.IsTerminal` reports a TTY (via `golang.org/x/term`), else **80** (a pipe, a `bytes.Buffer` in tests, or a probe error); `COLUMNS` is never consulted (DC-12); (8) `command.Run(ctx, req, cfg.Mode, deps)` — `ErrUnported` → placeholder exit 1, any other error → its text on stderr exit 1; (9) each of `res.Notices` via `fmt.Fprintln(stderr, n)` (the guard warnings, before any fetch warning — the TS `main()` order), then `source.WriteWarnings(stderr, res.Warnings)`, then each line via `fmt.Fprintln(stdout, l)`, exit 0. The edge's `env` is `config.Env{Getenv: os.Getenv, Hostname: os.Hostname, Username: currentUsername}` (`os/user` `Current()`). `main.go` imports `_ "time/tzdata"` so `TZ=Asia/Kolkata` (the harness `tz: alt` axis) resolves on hosts without `/usr/share/zoneinfo`; Go consults the embed only when the system database is missing (~450 KB, no startup cost). (4fs0)

### Requirement: End-to-end test against the fake ccusage
`src/go/cmd/tu/e2e_test.go` runs `run` against `TestMain`-built `fakeccusage` and `fakegit` (as `git`, first on PATH) replaying the `_placeholder` corpus only (`TUDIFF_FIXTURES`), with staged `$HOME`s (`harness.StageHome` for the four conf variants against the committed metrics-repo seed), `TZ=UTC`, `NO_COLOR` and `TU_METRICS_REPO` unset. It asserts byte-exact stdout/stderr/exit for the empty-state table (`tu`, `tu cc`, `tu m`, `tu w`, `tu --no-color`), the all-zero JSON (`tu --json`), the `tu bogus` usage error, `status` in each of the four staged variants, `init-conf` create/copy/complete, `init-metrics` unset/already-initialized/clone (asserting the recorded git argv via `TUDIFF_CALL_LOG`), `init-metrics a b`, `tu --dry-run`, a legacy-conf snapshot (deprecation line then the empty table, exit 0), `tu cc` with `user = all` (exit 2), the three setup commands with HOME unset (exit 1), and the history surfaces: `cc h --since 2026-01-01 --until 2026-01-31` (populated bytes, exit 0), `h` (capped heading + `  No data`), `mh` (one row, no Total), `wh --since … --until …` (the `2026-01-04` partial-week row), `h --json`, `cc mh --csv`, `cc mh --md`, the `--csv`/`--md` snapshot empties, `--since 2026-13-01` (stderr warning + empty snapshot, exit 0), and `h --by-machine` (placeholder). The e2e binary sees a `bytes.Buffer`, so width is 80 throughout and bars never render. Remaining populated-path coverage lives in the render goldens and the fake-`Fetcher` `Run` tests with a fixed `Now` and injected `Width` (see [query-view-render](/go-port/query-view-render.md)). (4fs0, 9ax5)

### Requirement: Harness gate status
`just go-diff --placeholder` reports 134 of 368 cases green: the single-mode snapshot and history cases (base, `-window` under both time zones, `-full`, the json/csv/md format cases), the snapshot csv/md cases, the `since-invalid` warn-and-render case, the usage-error groups, the 20 setup-command cases (`status`/`init-conf`/`init-metrics`/`init-metrics-url`/`init-metrics-extra` × the four conf variants), and the two `--dry-run` misuse cases. `h-by-machine`/`cc-h-by-machine` and every multi/org/legacy/envrepo case stay red by design, each owned by a named later row. Caveats: the placeholder corpus's dates (2026-01-05..07) never match "today", so snapshot cases exercise only the empty table and the all-zero JSON; the `-window`/`-full`/`mh` cases do exercise **populated** history (three days, six tools at `$0.50` each), but bars (80 columns never leaves room), column omission and dim zero cells (equal nonzero costs), month separators (all January), the current-period marker and weekend dimming, and token mode on history are pinned by goldens only — the local-capture harness run is the real-bytes check for them. See [differential-harness](/harness/differential-harness.md).

## Design Decisions

### Reproduce the JSON label quirk, not the cache quirk
**Decision**: `Run` clears labels on the single-mode daily-all path; `Run` caches on every path.
**Why**: The label omission is a harness-compared byte surface; the cache bypass is not, and Principle IV wants heavy work cached. Both are flagged for gate G0 (a spec correction / DC candidate for the label; a drop-at-cutover candidate for the cache).
**Rejected**: Reproducing both (a faithful port of a non-surface quirk against the constitution); reproducing neither (a harness red on populated data).
*Introduced by*: 260916-3am6-query-view-render-snapshot

### The full parser lands in one row
**Decision**: `Parse` implements the complete TS grammar and validation — every flag, every byte-exact message, the TS check order, version after validation, the non-data tokens — even though most parsed requests route to the placeholder.
**Why**: The parser is a single function in the TS; splitting it across rows would have the config/history/sync rows re-touch the same validation table, and the byte-exact usage errors are themselves harness cases that go green from the complete parser.
**Rejected**: Parsing only the snapshot grammar and growing it per row.
*Introduced by*: 260916-3am6-query-view-render-snapshot

### Placeholder for recognized-but-unported requests
**Decision**: `Run` returns `ErrUnported` for requests later rows own; `cmd/tu` prints the scaffold's unchanged placeholder line with exit 1.
**Why**: Unported harness cases stay red with the signature the harness already knows, and `Unknown argument` fires only where the TS fires it.
**Rejected**: Treating unported flags as unknown arguments (a false divergence on the error text).
*Introduced by*: 260916-3am6-query-view-render-snapshot

### Flag guards are a pure Normalize step whose notices ride the Result
**Decision**: `command.Normalize` clears/defaults flags and returns notice lines; `cmd/tu` prints them before the source warnings.
**Why**: Nothing below `cmd/tu` may write; the TS prints these before any fetch, so ordering notices ahead of `WriteWarnings` reproduces the stderr byte order; `since-invalid` must be in scope after the clear.
**Rejected**: Printing from `command` (breaks the only-writer rule); treating the guarded flags as out of scope (turns a warn-and-render exit 0 into the placeholder's exit 1).
*Introduced by*: 260916-9ax5-history-and-periods

### Terminal width via golang.org/x/term at the edge
**Decision**: `cmd/tu` probes stdout with `x/term` (`IsTerminal` + `GetSize` on the descriptor) and hands `Deps.Width` (80 when not a TTY) down; render never probes, and `COLUMNS` is never read.
**Why**: DC-12 fixes the 80-column pipe default and forbids `COLUMNS`; the plan's watch mode builds on `x/term` and the six sibling tools depend on it; a stdlib ioctl needs per-OS build tags for five lines.
**Rejected**: Hardcoding 80 until B7 (bars invisible in a real terminal during dogfood prep); a stdlib `TIOCGWINSZ` probe (build-tag surface for no gain).
*Introduced by*: 260916-9ax5-history-and-periods
