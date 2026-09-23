---
type: memory
description: internal/command request model and parser — Request, Flags, the Display/Format/Metric enums, Parse's fixed-order flag pass and validation, UsageError/ShortUsage/FullHelp, and the 0/1/2 exit-code constants
---
# Request and Parse

**Domain**: command

## Overview

`internal/command` (`request.go`, `parse.go`) defines the parsed invocation model — `Request`, `Flags`, and the `Display`/`Format`/`Metric` enums — and `Parse`, which turns raw argv into a `Request` or a `UsageError`. Config-dependent guards live in [guards](/command/guards.md), pipeline execution in [run-and-result](/command/run-and-result.md), and all stream writes in [cmd/tu](/command/entry-point.md).

## Requirements

### Requirement: The request model
`command.Display` enumerates `Snapshot`, `History`, `Leaderboard`, `LeaderboardHistory`; `command.Format` enumerates `Table`, `JSON`, `CSV`, `Markdown`; `command.Metric` enumerates `Cost`, `Tokens` (`request.go`). `command.Request` carries `Source` (`""` = all tools, aliases resolved), `Period query.Period`, `Display`, `Format`, `Flags`, `Command` (the first positional when it is a non-data command), `Args` (positionals after `Command`, nil for data commands), and `Version`. `command.Flags` carries every parsed flag with its spellings:

- booleans — `Fresh` (`--fresh`/`-f`), `NoColor` (`--no-color`), `Watch` (`--watch`/`-w`), `Sync` (`--sync`), `DryRun` (`--dry-run`), `ByMachine` (`--by-machine`), `Full` (`--full`), `NoRain` (`--no-rain`), `SkipBrewUpdate` (`--skip-brew-update`)
- `Interval int` (`--interval`/`-i <s>`) — defaults to `10`, validated only when `Watch` is set (range 5–3600 in `validate`)
- `User` (`--user`/`-u <user>`), `Since` (`--since`/`-s <date>`), `Until` (`--until <date>`, long-only)
- `Metric` (`--metric <cost|tokens>`; `-t` is boolean sugar for tokens), `Top` (`--top <n>`; 0 = unset)

(4fs0)

### Requirement: Parse reproduces the full grammar in a fixed check order
`command.Parse(args) (Request, *UsageError)` (`parse.go`) runs the flag pass, validation, the version check, help, the `--dry-run` misuse guard, non-data command dispatch, and positional parsing in a fixed order, so the same argv produces the same first error:

1. **Flag pass** (`scanFlags`): sixteen boolean flags are stripped from the positional list (the `boolFlags` map); value flags consume the next token only when it qualifies — `^\d+$` (`digitsOnly`) for `--interval`/`-i`, a non-dash-prefixed token for `--user`/`-u`, `--since`/`-s`, `--until`, `--metric`, `--top` — and are remembered as present either way. Everything else lands in the positional list, including `--help` when not first and unknown flags like `--bogus`.
2. **Validation** (`validate`), first failure wins, every message byte-exact, none shows the usage block: `--interval` with `--watch` — missing/non-numeric, `< 5`, `> 3600`; the six format conflicts in order (watch+json, json+csv, json+md, csv+md, watch+csv, watch+md; `-j` counts as `--json` and the message says `--json`); `-u` without a value; `--since`/`--until` shape (`normalizeDateFlag`: `YYYY-MM-DD` passes through, `YYYYMMDD` is dashed, anything else is invalid) — shape-only, so a well-shaped impossible date parses; an inverted window; `--metric` not `tokens`/`cost`; `-t` with an explicit `--metric cost`; `--top` not a positive integer. Format precedence is json > csv > md.
3. **Version after validation**: `--version`/`-V`/`-v` anywhere in argv sets `Request.Version` — `tu --json --csv --version` is a usage error, not a version line.
4. **Help first**: a first positional in {`help`, `-h`, `--help`} sets `Command` and returns — the help check precedes the `--dry-run` guard, so `tu help --dry-run` parses as help.
5. **`--dry-run` misuse guard**: `DryRun` with a first positional other than `sync` (or no positional) is `UsageError{ShowUsage: false}` — exit 2 with no usage block. `tu sync --dry-run` parses to `Command == "sync"` with `DryRun` set ([dry-run-report](/sync/dry-run-report.md)).
6. **Non-data commands**: a first positional in {`init-conf`, `init-metrics`, `sync`, `status`, `update`, `shell-init`, `help-dump`, `skill`} sets `Command`; the positionals after it become `Args`. Only `init-metrics` has an arity check: more than one argument is a `ShowUsage` error raised before `$HOME` is consulted, with the byte-exact message `Error: init-metrics takes at most one argument (repo-url)`. Data flags on a non-data command do not error — `tu status --json` sets both `Command` and `Format`.
7. **Positionals** (`parsePositionals`): a leading source token (`knownSources`; `sourceAliases` maps `co`→`codex`, `gem`→`gemini`, `cop`→`copilot`, `ki`→`kimi`; `all` yields `Source == ""`, see [tool-registry](/fact/tool-registry.md)), then `d`/`daily`, `w`/`weekly`, `m`/`monthly` set `Period`; `h`/`history`, `lb`, `lbh` set `Display`; `dh`/`wh`/`mh` set Period and `History` together. Anything else is `Unknown argument: {tok}` with `ShowUsage: true`.

(3am6) (4fs0)

#### Scenario: Byte-exact first errors
- **GIVEN** argv `--json --csv`, `-t --metric cost`, `cc codex`, `cc --help`, `--bogus`
- **WHEN** parsed
- **THEN** the messages are `Error: --json and --csv are incompatible`, `Error: -t and --metric cost are incompatible`, `Unknown argument: codex`, `Unknown argument: --help`, `Unknown argument: --bogus` — the last three with `ShowUsage`

#### Scenario: Guard order — version and help win
- **GIVEN** argv `--dry-run --version`, `help --dry-run`, `sync --dry-run`, `init-metrics a b`, `status --json`
- **WHEN** parsed
- **THEN** the first sets `Version`; the second sets `Command == "help"`; the third sets `Command == "sync"` with `DryRun` set and no error; the fourth is the `init-metrics` arity usage error with `ShowUsage`; the fifth sets `Command == "status"` and `Format == JSON` with no error

### Requirement: --interval without --watch is parse-inert
A bare `--interval N`/`-i N` without `--watch` is consumed and discarded at parse level — the numeric value is discarded, validation is watch-gated, and the snapshot renders normally. A non-numeric value is NOT consumed and lands in the positional list, so `tu --interval abc` (no `--watch`) is `Unknown argument: abc`, exit 2 (`scanFlags`, `validate`). (3am6)

#### Scenario: Watch-gated interval validation
- **GIVEN** argv `-w -i 4000` and `--interval 30` (no `-w`)
- **WHEN** parsed
- **THEN** the first is `Error: --interval maximum is 3600 seconds`, exit 2; the second parses cleanly with `Interval` left at its default `10`

### Requirement: UsageError, ShortUsage, FullHelp, and the exit-code constants
`command.UsageError{Message, ShowUsage}` is a grammar rejection: the message on stderr, exit 2, with `command.ShortUsage` appended when `ShowUsage` is set. `command.ShortUsage` and `command.FullHelp` are the byte-exact usage and full-help constants; `FullHelp` is grammar documentation, so `command` owns it while `internal/toolkit` receives it as a parameter ([version-and-help-dump](/toolkit/version-and-help-dump.md)). `command` exports the exit-code constants `ExitOK = 0`, `ExitOperational = 1`, `ExitUsage = 2` — the shll toolkit convention: success (including benign no-ops and warn-and-continue guards), operational failure (also the placeholder), usage error. `cmd/tu` returns only these values. (4fs0) (8h6g)

## Design Decisions

### The full parser lands in one change
**Decision**: `Parse` implements the complete grammar and validation — every flag, every byte-exact message, the fixed check order, version after validation, the non-data tokens — even requests later changes own.
**Why**: the parser is a single function; splitting it across changes would have each row re-touch the same validation table, and the byte-exact usage errors are themselves differential-harness cases that go green from the complete parser.
**Rejected**: parsing only the snapshot grammar and growing it per change.
*Introduced by*: 260916-3am6-query-view-render-snapshot

### -t is a toggle, not -m <value>
**Decision**: `-t` is boolean sugar that sets `Metric = Tokens`; `--metric <cost|tokens>` is the explicit long form; `-m` stays untouched and the flag adds no `Flags` field.
**Why**: two summable facts (cost, tokens) want a boolean toggle rather than a value-taking parameter; `-m` is reserved for a future `--by-machine` short flag.
**Rejected**: `-m <cost|tokens>` — burns the letter and still forces the user to type a value.
*Introduced by*: 260828-018g-tokens-table-mode-t-flag

### --dry-run parses globally, is honored only by tu sync, and misuses fail fast
**Decision**: `--dry-run` parses globally but only `tu sync` acts on it; any other invocation carrying it is a usage error (exit 2) before any fetch or dispatch.
**Why**: the multi-mode fetch path writes day-files outside the sync flow on every data command, so a combined `tu cc --sync --dry-run` that previewed then proceeded would write the very day-files it previewed — a lying dry-run. Fail-fast is the honest contract, and strict→loose is the non-breaking direction.
**Rejected**: silently ignoring the flag (a user who passed `--dry-run` must never get a surprise mutation); honoring it on data commands (would require gating the fetch-path writes too — far beyond scope, and still not a pure preview).
*Introduced by*: 260717-xuhk (exit-2 split 260717-8h6g)
