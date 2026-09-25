---
type: memory
description: "The tudiff report — report.txt's golden/go header, per-case verdict lines with golden=/go= first-divergence detail and the summary burndown block with expected-entry tallies, report.json's schema-1 field names, and the per-case go.* capture files (plus go.tree/ on a tree red) written under cases/."
---
# Report

**Domain**: harness

## Overview

Every non-`--list` `tudiff run` and `tudiff live` writes a report under `--report` (default `bin/harness/report` resp. `bin/harness/report-live`, gitignored via `bin/`), wiped and recreated at the start of the run. The drivers that produce it are in [differential-harness](/harness/differential-harness.md) and [live](/harness/live.md); the comparison that feeds it is in [comparison-and-live](/harness/comparison-and-live.md).

## Requirements

### Requirement: report.txt and report.json
`report.txt` — also streamed line-by-line to the harness's stdout — consists of a header (`tudiff run  <UTC RFC3339>`, `golden: <dir as given> (captured <captured_at> from <oracle> <oracle_version>; now <now>)`, `go: <path> (<go --version> first line)`, `identity: $MACHINE/$USER normalised`, `fixtures: <alias names>`, `script: util-linux|bsd|n/a`, `matrix: <path> (<N> cases[, filter "<f>"])`, `expected: <path as given> (<n> entries)`), one line per case in matrix order (`<STATUS padded to 7> <case ID>` plus, for red, `exit: golden=<n> go=<n>`, `tree: golden=<q> go=<q>`, or `<channel> @<offset> (line <l>): golden=<q> go=<q>`, for timeout `timeout: golden=<bool> go=<bool>`, then ` [expected <DC-id>]` when a red case matched an expected-diffs entry and ` [unconfirmed]` where applicable), and a summary block:

```
tudiff: <N> cases — <g> green, <r> red (<e> expected, <x> unexpected), <t> timeout   (fixtures: <aliases>; <u> cases replayed unconfirmed fixtures)
  by conf:  single <g>/<n>  multi <g>/<n>  org <g>/<n>  legacy <g>/<n>
  by env:   default <g>/<n>  nocolor <g>/<n>  envrepo <g>/<n>  pullfail <g>/<n>  pushfail <g>/<n>  dirty <g>/<n>
  by io:    pipe <g>/<n>  tty <g>/<n>
  by tz:    fixed <g>/<n>  alt <g>/<n>
```

When the expected-diffs file has one or more entries, one line per entry follows the axis lines: `  expected: <id>  <red>/<matched> red`, with ` (stale)` appended when the entry matched ≥ 1 executed case and none red, or ` (no executed case)` when it matched none (tallies run over the executed, post-`--filter` results — `live` and filtered runs legitimately exclude cases); an empty set prints no entry lines. The summary's first line carries the unexpected-red count the gate rule reads (489t). `report.json` carries the same data structured (`schema: 1`; `header` with `timestamp`, `golden`, `golden_captured_at`, `go`, `go_version`, `fixtures`, `script`, `matrix`, `cases`, `filter`, `expected` (path as given) and `expected_entries`; `summary` with `total`/`green`/`red`/`timeout`/`unconfirmed`/`expected`/`unexpected`/`stale` (string array, `[]` never `null`); `cases[]` with id, group, args, the four axes, status, channel, offset, line, `golden_excerpt`/`go_excerpt`, `golden_exit`/`go_exit`, `go_ms`, `unconfirmed`, `go_calls`, `rerun`, `expected` (the entry id or `""`)), 2-space indented with a trailing newline. (6wpm)

#### Scenario: A red case line names golden and go
- **GIVEN** a case red on channel `stdout` at offset 0, line 1
- **WHEN** the report renders
- **THEN** the case line ends `  stdout @0 (line 1): golden=<q> go=<q>` with the quoted excerpts

### Requirement: Per-case capture files
`cases/<case ID as nested dirs>/` holds the Go side's captures — `go.{stdout,stderr,exit}` for pipe cases, `go.{tty,exit}` for tty cases, plus `go.calls.jsonl` — with the byte channels written home-normalized (the exact bytes `Compare` compared, so `diff` against the golden shows what the harness saw; the `exit` files are unaffected), and — on a `tree` red — the Go side's written tree under `go.tree/`, so a red can be inspected without re-running. Temp staging lives under an `os.MkdirTemp` root removed at the end of the run; the report dir persists.
