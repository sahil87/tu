---
type: memory
description: sync.Write — the never-shrink-guarded day-file writer: one Decision per record in live and dry-run mode, the shrinkState guard and its Number() coercion table, and the shared metrics.DayFile JSON encoding written under {dir}/{user}/{year}/{machine}/{tool}-{date}.jsonl
---

# Day-File Writer

**Domain**: sync

## Overview

`sync.Write` (`src/go/internal/sync/writer.go`) writes one JSON day-file per usage record into the metrics-repo clone, guarded by a never-shrink rule that treats each day-file as a high-water mark of complete data. The same decision path runs in live and dry-run mode — only the filesystem effects are gated. The reader side of the encoding lives in [metrics-reader](/source/metrics-reader.md); the preview that consumes the decisions is [dry-run-report](/sync/dry-run-report.md).

## Requirements

### Requirement: One Decision per record in both modes
`sync.Write(dir, user, machine string, tool fact.Tool, recs []fact.Record, dryRun bool) ([]Decision, error)` MUST produce one `sync.Decision{Path, Action, IncomingCost, ExistingCost *float64}` per record, in record order, in BOTH live and dry-run mode. `Action` is `ActionWrite` or `ActionSkip` (the never-shrink guard would skip); `ExistingCost` is non-nil only when an existing parseable cost was read. In live mode a write decision MUST create the file's directory with `os.MkdirAll` (`0o755`) and write the day-file (`0o644`) as `json.Marshal(metrics.DayFile) + "\n"`; in dry-run mode nothing is created or written. A filesystem error MUST stop the walk and be returned. The writer MUST NOT print anything — a skip is silent.

#### Scenario: Guard arms across a batch
- **GIVEN** an existing `cc-2026-01-06.jsonl` costing 0.75 and incoming records costing 0.5 for 2026-01-05..07, with `cc-2026-01-05.jsonl` holding 0.25 and `cc-2026-01-07.jsonl` absent
- **WHEN** `sync.Write` runs in either mode
- **THEN** the decisions are write (existing 0.25), skip (existing 0.75), write (existing nil); in live mode only the two writes hit disk

### Requirement: metrics.DayFile is the one day-file encoding
`internal/source/metrics/metrics.go` exports `DayFile{Label string `json:"label"`; fact.Totals}` — the one JSON object a `{tool}-{date}.jsonl` file holds, label first, then the six totals in `fact.Totals` order. `metrics.Name(tool, date)` is the basename `{tool.Key}-{date}.jsonl`; `metrics.Path(dir, user, machine, tool, date)` is `{dir}/{user}/{year}/{machine}/{Name}` where `year` is the label's first four characters (a shorter label is used whole). The writer marshals `DayFile{Label: rec.Date, Totals: rec.Totals}`; the reader decodes into the same struct.

### Requirement: The never-shrink guard compares coerced costs
`shrinkState(path, incoming) (shrinking bool, existing *float64)` (`src/go/internal/sync/writer.go`) MUST make its verdict in one file read. Treated as absent (write, `existing` nil): a read error; empty/whitespace content; undecodable JSON; a top-level non-object; a missing `totalCost` key; or a coerced cost that is NaN or infinite. Otherwise the value at key `totalCost` MUST go through the `jsNumber` coercion table:

| JSON value at `totalCost` | Coerced cost |
|---------------------------|--------------|
| number | itself |
| `null` | `0` |
| `true` / `false` | `1` / `0` |
| string | `jsNumberString`: trimmed; empty → `0`; a `0x`/`0X`/`0o`/`0O`/`0b`/`0B` prefix (no sign allowed) → the integer in that base parsed as `big.Float` (53-bit, `ToNearestEven`) so long digit strings round as a double; else `strconv.ParseFloat`; any failure → NaN |
| array | coercion of its `jsJoin` (comma-join) string: `[5]` → 5, `[]`/`[null]` → 0, `[1,2]`/`[true]`/`[{}]` → NaN |
| object | NaN |

A finite result yields `shrinking = incoming < existing`: strictly lower skips, equal or greater writes (today's file keeps refreshing as the day grows). `jsNumberString` MUST validate base-prefixed digits itself and never offer them to `strconv.ParseFloat`, which would accept a hex float like `"0x1.8p1"` that the string-numeric grammar rejects.

### Requirement: The writer reaches command through an adapter
`sync.Writer{Dir}` MUST satisfy `command.Writer` (`Write(user, machine string, tool fact.Tool, recs []fact.Record) error`) by calling the package `Write` in live mode and dropping the decisions; the compile-time assertion sits beside the `cmd/tu` assignment. The write-then-read composition lives in [multi-mode](/command/multi-mode.md).

## Design Decisions

### Day-files are high-water marks
**Decision**: `sync.Write` skips any write whose incoming `totalCost` is strictly below the existing file's coerced cost; equal or greater overwrites.
**Why**: Claude Code purges session transcripts older than ~30 days, so a live fetch for an old date collapses toward zero; unconditional overwrites silently destroyed correct history on every post-purge sync (measured before the guard: $5,160.63 across 21 day-files for one user). Strictly-lower-only keeps today's file refreshing as the day grows.
**Rejected**: unconditional overwrite (destroys history); per-field max (fabricates chimera entries mixing token/cost fields from different snapshots).
*Introduced by*: 260610-srmi

### Dry-run shares the real decision path
**Decision**: `sync.Write` returns a `Decision` per record on EVERY call, live and dry-run; only `os.MkdirAll`/`os.WriteFile` are gated on `dryRun`.
**Why**: an accurate preview must not drift from the live path — the only design that guarantees the match is one where path construction and `shrinkState` run identically in both modes.
**Rejected**: a separate preview function duplicating the decision logic (invites exactly the drift the shared path forbids).
*Introduced by*: 260717-xuhk

### The guard coerces through a Number()-equivalent table
**Decision**: existing costs are coerced by `jsNumber`/`jsNumberString`/`jsJoin` — including base-prefixed integers, array joins, and NaN for objects — before the comparison.
**Why**: parity with the frozen golden corpus (`/harness/golden-corpus.md`, the retired TypeScript implementation's bytes); a day-file's `totalCost` may hold any JSON value when hand-corrupted, and the guard must classify each exactly as the reference coercion does.
**Rejected**: a strict float-only check (a string `"0.75"` or `null` in a corrupted file would diverge from the reference verdict).
*Introduced by*: 260916-lsml-sync-metrics-writer

### One read carries verdict and cost together
**Decision**: `shrinkState` returns `(shrinking, existing)` from a single file read, so `Decision.ExistingCost` needs no second read.
**Why**: minimum pathways — the dry-run report's `(update: $a → $b)` lines carry the existing cost, and reading twice would risk a torn verdict.
**Rejected**: a boolean-only guard plus a separate read for the report.
*Introduced by*: 260717-xuhk
