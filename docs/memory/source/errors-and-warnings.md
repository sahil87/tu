---
type: memory
description: Typed per-source errors for tu's data sources — source.Error kinds (exec/timeout/parse), the byte-exact stderr warning line, WriteWarnings as the sole edge writer, and the fetch-contract constants PeriodDaily and DefaultTimeout.
---

# Source Errors and Warnings

**Domain**: source

## Overview

`internal/source` (`src/go/internal/source/error.go`, `src/go/internal/source/fetch.go`) defines the typed per-source error every adapter returns as a value beside its records, the byte-exact warning line that reaches stderr, and the two fetch-contract constants. Sources never print: the warning line reaches a stream only through `source.WriteWarnings`, called by the command edge (see [run-and-result](/command/run-and-result.md)).

## Requirements

### Requirement: Error kinds and fields
`source.Error` SHALL carry `Tool` (registry key), `Name` (display name, for the warning line), `Kind` (one of `KindExec` — binary missing, spawn failure, non-zero exit, or signal; `KindTimeout` — the caller's context deadline fired; `KindParse` — stdout was not the expected JSON document), `Detail` (pre-rendered human detail), and `Err` (wrapped cause, may be nil). `(*Error).Error()` MUST return `Name + ": " + Detail`, and `Unwrap()` MUST return `Err`. Sources MUST return failures as `*source.Error` values beside the (zero) records — never panic, never write to stdout/stderr.

#### Scenario: Unwrap with a nil cause
- GIVEN an `Error{Name: "Codex"}` with `Err` unset
- WHEN `Unwrap()` is called
- THEN it returns nil

### Requirement: Which kinds warn
`(*Error).Warns()` MUST return true for `KindExec` and `KindTimeout` and false for `KindParse` — malformed JSON yields silent zero data, no stderr line.

#### Scenario: Warns per kind
- GIVEN one error of each kind
- WHEN `Warns()` is called on each
- THEN `KindExec` and `KindTimeout` report true and `KindParse` reports false

### Requirement: The byte-exact warning line
`(*Error).Warning()` MUST return exactly `warning: {Name} fetch failed ({Detail}), showing zero data` with no trailing newline. `Error()` is a free readable form; only `Warning()` is byte-pinned.

#### Scenario: Byte-exact warning
- GIVEN `Error{Name: "Kimi", Kind: KindExec, Detail: "Command failed: /x/ccusage kimi daily --json\nboom\n"}`
- WHEN `Warning()` is called
- THEN it returns `warning: Kimi fetch failed (Command failed: /x/ccusage kimi daily --json\nboom\n), showing zero data`

### Requirement: WriteWarnings is the only writer
`source.WriteWarnings(w, errs)` MUST write `Warning() + "\n"` for every error whose `Warns()` is true, in the order given, skipping nil entries and non-warning kinds. It is the only function in the source tree that touches an `io.Writer`, and it is called by the command edge, never by a source.

#### Scenario: Filtering and order
- GIVEN `[]*source.Error{execErr, parseErr, timeoutErr, nil}`
- WHEN `WriteWarnings` runs
- THEN exactly two lines are written: the exec warning, then the timeout warning

### Requirement: Fetch-contract constants
`source.PeriodDaily` MUST be `"daily"` — the one period every adapter is asked for; weekly and monthly are rolled up client-side (see [aggregation](/query/aggregation.md)). `source.DefaultTimeout` MUST be `120 * time.Second` — the per-invocation deadline the command edge puts on the context it hands to a Fetcher (`command.Run` and the sync paths apply it). Adapters impose no deadline of their own; they honor the given context only.

### Requirement: Only the command edge opens the process streams
Nothing below `cmd/tu` touches `os.Stdout`/`os.Stderr` or calls `os.Exit`. `run(args, stdout, stderr)` in `src/go/cmd/tu/main.go` is the only writer; source adapters receive no stream and return errors as values (see [entry-point](/command/entry-point.md)).

## Design Decisions

### Errors as values, warnings at the edge
**Decision**: sources return `*source.Error` beside their records; the stderr warning is written once, by `WriteWarnings` at the command edge.
**Why**: keeps sources pure and testable; a failing tool degrades to zero data with one stderr line instead of aborting the run.
**Rejected**: panicking or exiting on a source failure (one broken tool would kill all six).
*Introduced by*: 260916-v0as-fact-source-ccusage

### Warning detail mimics Node's error.message
**Decision**: `Detail` reproduces the `Command failed: {cmd}\n{stderr}` and `spawn {binary} ENOENT` shapes.
**Why**: the stderr warning is a byte-compared surface — parity with the frozen `src/node/` oracle (until plan row Z1).
**Rejected**: Go-native error text (guarantees a diff on every failure case).
*Introduced by*: 260916-v0as-fact-source-ccusage

### Parse failures stay silent
**Decision**: `KindParse` never warns; malformed JSON yields zero records with no stderr line.
**Why**: the compared stderr surface carries only exec and timeout failures — parity with the frozen `src/node/` oracle (until plan row Z1).
**Rejected**: warning on parse failures (adds a line the compared surface never emits).
*Introduced by*: 260916-v0as-fact-source-ccusage

### Contract constants live in source
**Decision**: `source.PeriodDaily` and `source.DefaultTimeout` live in `internal/source`.
**Why**: both describe the contract every adapter honors (the one period tu fetches; the edge-applied deadline); command and sync fetch under the same rules.
**Rejected**: unexported constants in command (sync would re-declare the deadline).
*Introduced by*: 260916-m9of-g1-rework-1
