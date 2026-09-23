---
type: memory
description: query.Period values and heading strings, label formats (daily YYYY-MM-DD, Sunday-anchored weekly via WeekLabel/CurrentLabel, monthly YYYY-MM), ThreeMonthFloor's month-aligned implicit-cap floor, and the pure Window/ByTool/ByUser record filters with inclusive bounds and open sides
---

# Periods & Windows

**Domain**: query

## Overview

`package query` (`src/go/internal/query/`) is the pure filter/roll-up stage of the pipeline: it consumes `[]fact.Record` (see [fact/records](/fact/records.md)) and produces filtered or bucketed records with no I/O, exec, or registry imports. This file covers its period vocabulary (`Period`, label formats, `ThreeMonthFloor`) and its record filters (`Window`, `ByTool`, `ByUser`); summation and grouping live in [query/aggregation](/query/aggregation.md).

## Requirements

### Requirement: Period is the roll-up granularity
`query.Period` is an int enum — `Daily`, `Weekly`, `Monthly` in declaration order. `Period.String()` MUST return the table heading text: `"daily"`, `"weekly"`, or `"monthly"` (`src/go/internal/query/period.go`).

### Requirement: CurrentLabel resolves "now" per period in local time
`CurrentLabel(p, now)` MUST compute the period's label in `now.Location()`: daily as `2006-01-02`; weekly as the ISO date of the current week's Sunday via `now.AddDate(0, 0, -int(now.Weekday()))` (which normalizes month/year underflow); monthly as `2006-01`. Callers pass `time.Now()`; `time.Local` honors `$TZ`. The weekly case deliberately uses local-time arithmetic while `WeekLabel` uses UTC arithmetic — grouping a fixed label and resolving "now" answer different questions (wkly).

#### Scenario: week Sunday crosses a month or year boundary
- **GIVEN** `now` is Thursday 2026-01-01 in UTC
- **WHEN** `CurrentLabel(Weekly, now)` runs
- **THEN** the label is `2025-12-28` (the Sunday in the previous year), while `CurrentLabel(Monthly, now)` is `2026-01`

### Requirement: WeekLabel anchors weeks on Sunday
`WeekLabel(daily)` MUST map a daily ISO label to its week's Sunday using UTC date arithmetic on the date-only string (`time.Parse("2006-01-02", daily)` then `AddDate(0, 0, -int(d.Weekday()))`) — immune to local DST transitions, so every day label of a week shares one bucket label regardless of the user's timezone (wkly).

#### Scenario: malformed label degrades to its own bucket
- **GIVEN** a label that does not parse as `2006-01-02` (e.g. `"garbage"`, `"2026-09"`, or `""`)
- **WHEN** `WeekLabel` runs on it
- **THEN** the label is returned unchanged and becomes its own bucket instead of crashing aggregation (graceful degradation)

#### Scenario: Thursday maps to the previous-year Sunday
- **GIVEN** the daily label `2026-01-01` (a Thursday)
- **WHEN** `WeekLabel` runs
- **THEN** the result is `2025-12-28`; a Sunday label such as `2026-03-01` maps to itself

### Requirement: ThreeMonthFloor is the implicit history cap's floor
`ThreeMonthFloor(now)` MUST return the first day of the local month two calendar months back, formatted `2006-01-02`, so the default history window spans three calendar months including the current one. `time.Date(now.Year(), now.Month()-2, 1, ...)` normalizes month underflow across the year boundary. The caller passes `deps.Now()` so the harness timezone axis holds; the cap itself is applied at the guard seam in [command/guards](/command/guards.md) (yuuj).

#### Scenario: January now rolls back into the previous year
- **GIVEN** `now` is 2026-01-15
- **WHEN** `ThreeMonthFloor(now)` runs
- **THEN** the floor is `2025-11-01`; for `now` = 2026-09-16 it is `2026-07-01`

### Requirement: Window filters daily records on inclusive bounds
`Window(recs, since, until)` MUST keep records with `since <= Date <= until` by lexicographic comparison on the ISO labels — valid because ISO labels order by byte comparison — inclusive on both ends; an empty bound is open on that side. It MUST NOT mutate its input or return the input slice.

#### Scenario: bounds are inclusive and sides are open when empty
- **GIVEN** daily records on 2026-01-05, 2026-01-06, 2026-01-07
- **WHEN** `Window` runs with `since` = `"2026-01-06"` and `until` = `"2026-01-06"`
- **THEN** exactly the 2026-01-06 record survives; with `until` = `""` both 2026-01-06 and 2026-01-07 survive; with both bounds empty all three survive

### Requirement: ByTool and ByUser are equality filters
`ByTool(recs, key)` MUST keep records whose `Tool` equals the registry key; `ByUser(recs, user)` MUST keep records whose `User` equals the argument. Both are pure: the input is never mutated and never returned.

### Requirement: every query function is pure
No function in `package query` may perform I/O or mutate or alias its input slice: filters build fresh output, `Relabel` and `SortByDate` copy before transforming, and `RollUp` returns a fresh slice even at daily granularity (see [query/aggregation](/query/aggregation.md)). The purity rule is verified by the package's tests, which assert inputs survive unmutated and unaliased.

## Design Decisions

### Sunday-anchored week labels
**Decision**: Weekly buckets are labeled with the ISO date of the week's Sunday (`WeekLabel`), and `CurrentLabel(Weekly, ...)` resolves "now" the same way in local time.
**Why**: The Sunday ISO date aligns row-for-row with `ccusage weekly --json`'s default `--start-of-week sunday` output and needs no new label format; UTC date arithmetic on the date-only string is DST-immune, while the local-time "now" computation honors `$TZ`.
**Rejected**: A distinct `YYYY-Www` ISO-week form — a new label vocabulary that nothing else emits; Monday-anchored weeks — would not match the upstream weekly output.
*Introduced by*: 260703-wkly (wkly)

### Month-aligned three-calendar-month floor
**Decision**: The implicit history cap's floor is the first day of the local month two calendar months back (`ThreeMonthFloor`), so the default daily/weekly window is exactly three calendar months including the current one.
**Why**: A month-aligned floor makes the default window (and the resulting Total row) a clean calendar-month figure, and reusing the `--since` label machinery means the floor needs no filtering logic of its own; the caller-side injection (explicit `--since`/`--until` disables the cap entirely) lives at the guard seam.
**Rejected**: A rolling day-precision floor (e.g. `now - 90 days`) — produces ragged month windows and a partial leading month; intersecting the floor with an explicit window — a past `--until` would silently empty the output.
*Introduced by*: 260717-yuuj (yuuj)

### Malformed labels degrade instead of failing
**Decision**: `WeekLabel` returns an unparseable label unchanged, letting it form its own bucket.
**Why**: Aggregation MUST NOT crash on a stray label in stored data; a self-bucket keeps the record visible and countable.
**Rejected**: Panicking or dropping the record — the first kills the CLI on bad data, the second silently loses usage.
*Introduced by*: 260916-9ax5-history-and-periods (9ax5)
