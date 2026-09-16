---
type: memory
description: The Go port's command edge — internal/command (Request with Args, the byte-exact Parse in TS order — validation, version, help, the --dry-run misuse guard, non-data commands with the init-metrics arity error, data grammar — the exit-code constants, Run composing the pipeline into a Result, the ErrUnported placeholder row map) and cmd/tu as the only writer (setup-command dispatch, Load warnings, reserved-user guard, write order); config cascade: config-and-setup — 95 harness cases green
---
# Command Edge and Entry Point (Go port)

**Domain**: go-port

## Overview

`internal/config` owns the full config cascade and the setup commands ([config-and-setup](/go-port/config-and-setup.md)); `internal/command` parses the complete tu argument grammar into a `Request` and composes the pipeline — the input layer ([fact-and-sources](/go-port/fact-and-sources.md)) through the middle layers ([query-view-render](/go-port/query-view-render.md)) — into a `Result`; `cmd/tu` is the only writer. The binary answers the single-mode snapshot grammar and the setup commands `init-conf`, `init-metrics`, and `status` for real; every recognized-but-unported request keeps the scaffold's placeholder.

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

### Requirement: Warn-and-clear guards render with exit 0
The TS warn-and-clear guards — `--since`/`--until`, `--full`, `--top`, single-mode `-u`, and `--by-machine` on the all-tools pivot — print a `Warning: …` line on stderr and STILL render the snapshot with exit 0; they neither error nor apply the flag. The rows that port those flags (B2 windows/full, B3 `-u`, B4 by-machine, B5 top) MUST implement warn-and-render for snapshot displays, not an error and not silent application. Until those rows land, the flags take the placeholder path and the corresponding single-mode harness cases are deliberately red. (3am6)

### Requirement: Run composes the in-scope request, else ErrUnported
In scope: `Display == Snapshot`; `Format` Table or JSON; `Source` any tool or all; any period; flags limited to `--json`/`-j`, `--fresh`/`-f`, `--no-color`, `-t`, `--metric`; single mode; no non-data command; not `Version`. Anything else returns `ErrUnported` without fetching. For an in-scope request, `Run(ctx, req, mode, deps)`:

1. Applies `context.WithTimeout(ctx, ccusage.DefaultTimeout)` once, for all tools.
2. Fetches **daily only** (`ccusage.PeriodDaily`, no extra args — roll-up is client-side): `FetchAll` for all tools, `Fetch(tool)` for a single source, with `fresh = Flags.Fresh`. `Fetcher` is the seam; `*ccusage.Source` satisfies it.
3. Computes `cur = query.CurrentLabel(Period, deps.Now())` and groups `GroupBy(Window(RollUp(recs, Period), cur, cur), Tool)`.
4. Builds `[]view.ToolTotals` over registry order (all six tools, or the one source) with `Name` from `ccusage.Lookup` and `Label = cur` when a group matched, else `""`.
5. Clears every `Label` when `Source == "" && Period == Daily` — the TS single-mode daily-all path (`fetchAllTotals`) returns bare totals, so `tu --json` never carries `"label"` while `tu cc --json` (any period) and `tu m --json` / `tu w --json` do. The table is unaffected (it never shows the label). (3am6)
6. Renders `json.Snapshot(rows)` for JSON, else `ansi.Table(view.Snapshot(rows, Period), deps.Colors)`.
7. Returns `Result{Lines, Warnings, TotalCost, TotalTokens, CostByItem}` — `Warnings` are the source errors the edge writes via `source.WriteWarnings`; `TotalCost`/`TotalTokens` sum the rows; `CostByItem` maps display name to cost (the watch-stats shape).

Caching is uniform: every path goes through `Source{Cache: cache.Default()}` and `--fresh` skips the read everywhere. The TS daily-all cache bypass (bare `tu`/`tu --json` never touch `~/.tu/cache` and `--fresh` is a no-op there) is deliberately NOT reproduced — the cache is not a Goal-listed external surface, the harness cannot observe it, and Constitution Principle IV wants heavy operations cached; it is recorded as a drop-at-cutover candidate for gate G0. (3am6)

### Requirement: Placeholder policy and the row map
A recognized-but-unported request prints `tu: not implemented (Go port in progress)` on stderr, stdout empty, exit 1 — keeping those harness cases red with the signature the harness already diffs. Ownership of each unported surface: displays `h`/`history`/`dh`/`wh`/`mh`, formats `--csv`/`--md`, and flags `--since`/`-s`/`--until`/`--full` → B2 (history + csv/md); multi mode (non-empty `metrics_repo` or `TU_METRICS_REPO`) and `--user`/`-u` → B3; `--by-machine` → B4; `lb`/`lbh` and `--top` → B5; the `sync` command, `--sync`, and `tu sync --dry-run` → B6; `--watch`/`-w`, `--interval`, `--no-rain` → B7; `help`/`-h`/`--help` (first arg), `help-dump`, `skill`, `shell-init`, `update`, `--skip-brew-update` → B8. The setup commands `init-conf`/`init-metrics`/`status` and the `--dry-run` misuse guard are implemented (4fs0).

### Requirement: cmd/tu run() write order and exit codes
`run(args, stdout, stderr) int` is the only writer — nothing below `cmd/tu` touches stdout/stderr or calls `os.Exit` — in the TS `main()` order: (1) `command.Parse` — a usage error prints the message, then `ShortUsage` when `ShowUsage`, exit `ExitUsage`; (2) `Version` → the version line on stdout, exit `ExitOK`; (3) `Command != ""` → `runCommand`: `init-conf`/`init-metrics`/`status` are answered for real (paths resolved first — `ErrNoHome` → its byte-exact message, `ExitOperational`; the per-command handlers and the clone execution are in [config-and-setup](/go-port/config-and-setup.md)); every other command → placeholder, `ExitOperational`; (4) `config.ResolvePaths(os.Getenv("HOME"))` error → its message, `ExitOperational` (parse precedes the HOME check: `tu bogus` with HOME unset still exits 2); (5) `config.Load(paths, env, Overrides{})` — its cascade warnings print on stderr before anything else; (6) the reserved-user guard — `cfg.User == "all"` → `Error: config user "all" is reserved (used by -u all)` on stderr, `ExitUsage` (B3's metrics-dir guard slots between Load and this check when it lands); (7) deps — `&ccusage.Source{Cache: cache.Default()}`, `Now: time.Now`, `ansi.Colors{Enabled: !NoColor && os.Getenv("NO_COLOR") == ""}`; (8) `command.Run(ctx, req, cfg.Mode, deps)` — `ErrUnported` → placeholder exit 1, any other error → its text on stderr exit 1; (9) `source.WriteWarnings(stderr, res.Warnings)`, then each line via `fmt.Fprintln(stdout, l)`, exit 0. The edge's `env` is `config.Env{Getenv: os.Getenv, Hostname: os.Hostname, Username: currentUsername}` (`os/user` `Current()`). `main.go` imports `_ "time/tzdata"` so `TZ=Asia/Kolkata` (the harness `tz: alt` axis) resolves on hosts without `/usr/share/zoneinfo`; Go consults the embed only when the system database is missing (~450 KB, no startup cost). (4fs0)

### Requirement: End-to-end test against the fake ccusage
`src/go/cmd/tu/e2e_test.go` runs `run` against `TestMain`-built `fakeccusage` and `fakegit` (as `git`, first on PATH) replaying the `_placeholder` corpus only (`TUDIFF_FIXTURES`), with staged `$HOME`s (`harness.StageHome` for the four conf variants against the committed metrics-repo seed), `TZ=UTC`, `NO_COLOR` and `TU_METRICS_REPO` unset. It asserts byte-exact stdout/stderr/exit for the empty-state table (`tu`, `tu cc`, `tu m`, `tu w`, `tu --no-color`), the all-zero JSON (`tu --json`), the `tu bogus` usage error, `status` in each of the four staged variants, `init-conf` create/copy/complete, `init-metrics` unset/already-initialized/clone (asserting the recorded git argv via `TUDIFF_CALL_LOG`), `init-metrics a b`, `tu --dry-run`, a legacy-conf snapshot (deprecation line then the empty table, exit 0), `tu cc` with `user = all` (exit 2), and the three setup commands with HOME unset (exit 1). The populated path is covered by the render goldens and the fake-`Fetcher` `Run` tests with a fixed `Now` (see [query-view-render](/go-port/query-view-render.md)). (4fs0)

### Requirement: Harness gate status
`just go-diff --placeholder` reports 95 of 368 cases green: the single-mode snapshot cases, the usage-error groups, the 20 setup-command cases (`status`/`init-conf`/`init-metrics`/`init-metrics-url`/`init-metrics-extra` × the four conf variants), and the two `--dry-run` misuse cases (`dry-run`, `cc-dry-run`) — the last 22 green via the setup commands, the arity error, and the misuse guard, with no `[calls differ]` marker on the `init-metrics` groups. The csv/md/by-machine single cases and every multi/envrepo case stay red by design, each owned by a named later row. Caveat: the placeholder corpus's dates (2026-01-05..07) never match "today", so snapshot cases exercise only the empty table and the all-zero JSON; the populated path's real-bytes check is the local-capture harness run. See [differential-harness](/harness/differential-harness.md).

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
