---
type: memory
description: query package summation and grouping — Relabel/RollUp period bucketing with Sunday-anchored weeks, the single GroupBy over Dim tuples in first-seen order, Collapse as its record-shaped wrapper, MaxMerge whole-record max (ties keep the live record), and the input-order float-summation chain that pins --json bytes
---

# Aggregation

**Domain**: query

## Overview

`package query` (`src/go/internal/query/`) aggregates `[]fact.Record` (see [fact/records](/fact/records.md)) after the filters of [query/periods-and-windows](/query/periods-and-windows.md): `RollUp` buckets and sums by period, one `GroupBy` serves every pivot, `Collapse` and `MaxMerge` are the two merge verbs, and the input-order summation chain pins the raw `--json` float bytes consumed by [render/json](/render/json.md).

## Requirements

### Requirement: Relabel is the pure relabel half of RollUp
`Relabel(recs, p)` MUST return a copy with each `Date` mapped to its period bucket — daily identity, weekly `WeekLabel(Date)`, monthly `Date[:7]` — with no summing, no sorting, and no mutation or aliasing of the input. The `--by-machine` breakdown in [command/run-and-result](/command/run-and-result.md) uses it to group on bucketed labels while summing in record input order itself (pmsd).

#### Scenario: weekly relabel crosses a month boundary
- **GIVEN** daily records on 2026-01-05, 2026-01-06, and 2026-02-01
- **WHEN** `Relabel(recs, Weekly)` runs
- **THEN** the labels are `2026-01-04`, `2026-01-04`, `2026-02-01` — three records, Totals unchanged, input unmutated

### Requirement: RollUp re-labels and sums
`RollUp(recs, p)` MUST re-label each record to its period bucket and sum `Totals` via `fact.Totals.Add` over records sharing `(Date', Tool, User, Machine)` — all six numeric fields, with `TotalTokens` summed, never recomputed from parts. Output MUST be ascending by `Date` via `sort.SliceStable` (byte order on ISO labels) with first-seen order within a label. Daily returns a copy, never the input slice.

#### Scenario: window before roll-up yields a leading partial week
- **GIVEN** daily records on 2026-01-05..07 and the window `--since 2026-01-01 --until 2026-01-31`
- **WHEN** `RollUp(Window(recs, since, until), Weekly)` runs (window on daily records first, roll-up second)
- **THEN** one entry labeled `2026-01-04` — the Sunday preceding `since` — sums the three days; a window starting mid-month likewise sums only in-window days into a partial month

### Requirement: history windows apply before roll-up; the snapshot windows after
The history pipeline MUST compose `RollUp(Window(recs, since, until), period)` — window on the daily records first, roll-up second — so a partial month sums only in-window days. The snapshot composes the other way round, `GroupBy(Window(RollUp(recs, p), cur, cur), Tool)`, because its window is a single period label. The two orders coexist deliberately in [command/run-and-result](/command/run-and-result.md).

### Requirement: GroupBy is the one group-by
`GroupBy(recs, dims ...Dim) []Group` MUST sum `Totals` per distinct tuple of the requested dims — `Date`, `Tool`, `User`, `Machine` — and emit groups in first-seen input order, so a registry-ordered input yields registry-ordered groups. `Group.Key` MUST carry only the grouped dims, its other string fields empty. `GroupBy` never sorts and never mutates its input; every pivot (snapshot, history series, breakdown, leaderboards) shares this one group-by — there is no second aggregation loop anywhere in the pipeline (pmsd).

#### Scenario: first-seen order preserves registry order
- **GIVEN** records for codex then cc (registry order)
- **WHEN** `GroupBy(recs, Tool)` runs
- **THEN** the groups come out codex-then-cc, not alphabetically — a registry-ordered input yields registry-ordered table columns

### Requirement: Collapse is the daily cross-machine sum over the one GroupBy
`Collapse(recs, dims ...Dim)` MUST sum `Totals` per distinct tuple of `dims`, return one record per group carrying only the grouped dims in first-seen order, and perform additions in input order. It is a thin wrapper over the one `GroupBy` — no second aggregation loop. On records already unique per key it is the identity on Totals (the single-mode path); the multi-mode pipeline in [command/run-and-result](/command/run-and-result.md) runs `Collapse(recs, Tool, Date)` followed by `SortByDate` before the window and roll-up.

#### Scenario: additions within a key happen in input order
- **GIVEN** three cc records on 2026-01-06 with `TotalCost` 0.1, 0.2, 0.3 (runtime values)
- **WHEN** `Collapse(recs, Tool, Date)` runs
- **THEN** the group's `TotalCost` equals `(0.1 + 0.2) + 0.3` — the input-order association, which can differ in the last bit from any other association

### Requirement: MaxMerge is the self-view high-water merge
`MaxMerge(a, b)` MUST keep, per key `(Date, Tool, User, Machine)`, whichever WHOLE record — from `a` or from `b` — has the greater `TotalCost`; never field-wise, never summed. On a tie the record from `a` wins (callers pass the live fetch as `a`, so ties keep the live record). Output order MUST be `a`'s records in `a`'s order, replaced in place when `b` wins, then `b`'s unmatched records in `b`'s order. Neither input is mutated or returned (srmi).

#### Scenario: tie keeps the live record
- **GIVEN** `a` (live) and `b` (stored snapshot) both carry cc on 2026-01-05 with `TotalCost` 0.50 but different token fields
- **WHEN** `MaxMerge(a, b)` runs
- **THEN** the output keeps `a`'s record, tokens included; the same date from another machine is a different key and is never merged

### Requirement: SortByDate pins the summation order before roll-up
`SortByDate(recs)` MUST return a stably sorted copy ascending by `Date` (byte order on ISO labels), never mutating its input. The main-table pipeline sorts the collapsed daily records before `Window`/`RollUp` because a machine-major gather order (own machine's days first, then other machines in walk order) would otherwise sum a month/week bucket in a different association, changing the raw `--json` float bytes emitted by [render/json](/render/json.md).

## Design Decisions

### Own-machine snapshots max-merged, never summed
**Decision**: `MaxMerge` keeps, per `(Date, Tool, User, Machine)`, the whole record with the greater `TotalCost`; ties keep the live (`a`) record.
**Why**: Claude Code purges transcripts after roughly 30 days, so the live fetch under-reports old days while the metrics repo keeps snapshots; a partially-purged day still yields a live entry, and summing live + snapshot would double-count the surviving portion. Max, not sum; ties keep the live record because it reflects the freshest fetch.
**Rejected**: Field-wise max (mixes cost from one source with tokens from another — an incoherent record); summing the two views (double-counts surviving days).
*Introduced by*: 260610-srmi (srmi)

### One GroupBy, first-seen order, no internal sort
**Decision**: Every pivot routes through the single `GroupBy`, which emits groups in first-seen input order and never sorts; `Collapse` is a thin wrapper over it.
**Why**: Registry-ordered input yields registry-ordered table columns and JSON keys with no extra pass, and one aggregation loop cannot drift into two; callers that need an order sort their input first.
**Rejected**: Sorting by key inside `GroupBy` — reorders tool columns alphabetically and breaks registry order; a second aggregation loop for `Collapse` — two loops that can drift.
*Introduced by*: 260916-3am6-query-view-render-snapshot (3am6)

### Date-sorted collapsed input pins the --json float bytes
**Decision**: The main-table pipeline runs `SortByDate` on the collapsed daily records before `Window`/`RollUp`, so every period bucket sums its days in ascending-date association regardless of gather order.
**Why**: The gather order is machine-major (own machine's days first, then other machines in walk order); float addition is not associative, so summing a bucket in that order can differ in the last bit, and the `--json` cost fields are raw doubles — parity with the frozen golden corpus (`/harness/golden-corpus.md`, the retired TypeScript implementation's bytes) demands the ascending-date association.
**Rejected**: Summing in gather order — machine-major association can differ in the last bit of a raw double, changing `--json` bytes across machines; a canonical re-sort inside `RollUp` — hides the ordering contract from callers like the breakdown that deliberately sum in record input order.
*Introduced by*: 260916-9ax5-history-and-periods (9ax5)

### Breakdown values from one GroupBy over relabelled records
**Decision**: The `--by-machine` breakdown sums via one `GroupBy(Relabel(windowed), Tool, Date, dim)` per table — no `RollUp` on the breakdown path.
**Why**: Summing in record input order reproduces the sequential float association observable in `--json` bytes (visible under `-u all` for a multi-machine user), and `GroupBy`'s first-seen order reproduces the machine column order; it also satisfies the one-group-by rule.
**Rejected**: `RollUp` then `GroupBy(Tool, Date, dim)` — sums each machine's days per bucket first, a different float association that can differ in the last bit of the `--json` machine cost bytes.
*Introduced by*: 260916-pmsd-machine-columns (pmsd)
