---
type: memory
description: The read-only metrics-repo reader — metrics.Source walking {user}/{year}/{machine}/{tool}-{date}.jsonl, the shared DayFile encoding (label plus six totals), the Name/Path helpers, user enumeration, and the absent-data-is-never-an-error rule.
---

# Metrics Reader

**Domain**: source

## Overview

`internal/source/metrics` reads the local clone of the metrics repo: `metrics.Source{Dir}` walks `{user}/{year}/{machine}/{tool}-{date}.jsonl` and returns `fact.Record` values stamped from the directory names (see [records](/fact/records.md)). It never writes — the writer lives in `internal/sync` (see [day-file-writer](/sync/day-file-writer.md)) — never prints, and never returns an error: missing or unreadable data is absent data, and the repo's absence is reported once by the metrics-dir guard (see [metrics-dir-guard](/config/metrics-dir-guard.md)).

## Requirements

### Requirement: Day-file encoding and naming
A day-file SHALL hold exactly one JSON object, `metrics.DayFile{Label string; fact.Totals}` with the `label` key first and the six totals in `fact.Totals` order. `metrics.Name(tool, date)` MUST be `{tool.Key}-{date}.jsonl`. `metrics.Path(dir, user, machine, tool, date)` MUST be `{dir}/{user}/{year}/{machine}/{Name}` where `year` is the label's first four characters, or the whole label when shorter. A missing totals key MUST decode as 0.

#### Scenario: Name and Path
- GIVEN tool `cc` and date `"2026-01-05"`
- WHEN `Name` and `Path("/repo", "u", "m", cc, "2026-01-05")` are computed
- THEN they are `cc-2026-01-05.jsonl` and `/repo/u/2026/m/cc-2026-01-05.jsonl`

### Requirement: Read walk and stamping
`Source.Read(user, tool)` SHALL return the user's records for the tool across every machine, walking year directories ascending, machine directories ascending, then files ascending, reading only files whose name has prefix `{tool.Key}-` and suffix `.jsonl`. Per file: read, trim, skip when empty, decode the single JSON object, skip silently on any decode error. The record's `Date` MUST be the JSON `label` (never the filename date); `Tool` is `tool.Key`; `User` and `Machine` are the directory names. A missing user directory, an unreadable level, or a non-directory at the year or machine level MUST be skipped silently (nil when nothing is read). A `user` value that is empty, `.`, `..`, or contains a path separator MUST return nil — the walk never escapes `Dir`.

#### Scenario: Walk order on the committed seed
- GIVEN the seed `harness/metrics-repo/` copied to a temp dir
- WHEN `Read("harness-user", cc)` runs
- THEN it returns, in order, `harness-machine/cc-2026-01-05` ($0.25), `harness-machine/cc-2026-01-06` ($0.75), `other-box/cc-2026-01-06` ($0.40), each stamped with the directory identity and dated from the JSON label; `Read("nobody", cc)` returns nil

#### Scenario: Non-object content is skipped
- GIVEN day-files containing `""`, whitespace, `not json`, and `null`
- WHEN `Read` runs
- THEN all four are skipped silently — `null` decodes cleanly into the struct but is no day-file

### Requirement: User enumeration
`Source.Users()` SHALL list the profile directories: direct children of `Dir` that are directories, excluding dot-prefixed names (`.git`), the `nonUserDirs` set (`{"docs"}`), and plain files, sorted ascending in byte order. A missing or unreadable `Dir` MUST yield nil.

#### Scenario: Seed enumeration
- GIVEN the committed seed tree (users `harness-user` and `other-user`, plus `docs`, `.git`, and a plain `README` file)
- WHEN `Users()` runs
- THEN it returns `["harness-user", "other-user"]`

### Requirement: Absent data, never an error
The package MUST NOT return errors: every filesystem or decode failure reads as absent data. It MUST NOT write to the repo, print, or exec.

## Design Decisions

### One DayFile encoding shared with the writer
**Decision**: the reader and the sync writer share `metrics.DayFile` and the `Name`/`Path` helpers.
**Why**: `json.Marshal(DayFile)` reproduces the committed seed file bytes — parity with the frozen golden corpus (`/harness/golden-corpus.md`, the retired TypeScript implementation's bytes) — and one encoding keeps reader and writer in lockstep.
**Rejected**: separate read and write shapes (two places to drift).
*Introduced by*: 260916-lsml-sync-metrics-writer

### Absent data, not errors
**Decision**: the reader swallows every filesystem and decode failure as absent data; the metrics-dir guard reports the repo's absence once, at the edge.
**Why**: a half-written or missing day-file must not fail the whole display; the single actionable condition (no repo) already has a reporter.
**Rejected**: per-file error propagation (one corrupt file would blank the view).
*Introduced by*: 260916-xivf-metrics-source-and-multi-mode

### No machine-exclusion parameter
**Decision**: `Read` walks all machines; there is no exclusion parameter.
**Why**: every production caller reads all machines in one walk; double-counting with live data is prevented downstream by max-merging the own machine's snapshots into the live view (srmi).
**Rejected**: keeping a test-only exclusion knob (a dead parameter on every call site).
*Introduced by*: 260916-xivf-metrics-source-and-multi-mode

### The label, not the filename, keys the record
**Decision**: `Date` comes from the file's JSON `label`.
**Why**: the filename is a storage detail and the label is the data — a misnamed or hand-copied file still reads its true date.
**Rejected**: parsing the date out of the filename (couples reads to the naming convention).
*Introduced by*: 260916-xivf-metrics-source-and-multi-mode
