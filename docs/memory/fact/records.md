---
type: memory
description: internal/fact's data model — fact.Record (Date/Tool/User/Machine + Totals), fact.Totals (six pinned JSON names, TotalTokens stored not recomputed), pure Add and IsZero; the one record type every downstream stage consumes
---

# Fact Records

**Domain**: fact

## Overview

`package fact` (`src/go/internal/fact/fact.go`) defines the one record type every downstream stage consumes: `fact.Record` is what a tool cost a user on a machine on a date, and `fact.Totals` is cost plus the five token counters. Sources produce `[]fact.Record`; query, view, and render consume it and nothing else. The tool registry lives beside it in the same package — see [tool-registry](/fact/tool-registry.md).

## Requirements

### Requirement: fact.Record and fact.Totals shape
`package fact` SHALL define `Record{Date, Tool, User, Machine string; Totals}` and `Totals{TotalCost float64; InputTokens, OutputTokens, CacheCreationTokens, CacheReadTokens, TotalTokens int64}` (`src/go/internal/fact/fact.go`). `Date` is an ISO label **string** — `YYYY-MM-DD` for daily facts, `YYYY-MM` for monthly roll-ups — never a `time.Time`. `Tool` holds the registry key (`cc`, `codex`, …), not the display name. `User` and `Machine` are caller-supplied identity fields, stamped by the producing source after any cache read/write. `Record`'s four string fields carry no JSON tags (Go default names); `Totals`' six JSON tags are the pinned external names from `docs/specs/usage.md` § Data Model (`totalCost`, `inputTokens`, `outputTokens`, `cacheCreationTokens`, `cacheReadTokens`, `totalTokens`), so render/json and the cache envelope encode the struct directly. `TotalTokens` is stored, never recomputed from the other counters (byte parity with ccusage's own value). The zero `Totals{}` is the "no data" value.

#### Scenario: JSON encoding uses the pinned names
- **GIVEN** a `Totals{TotalCost: 0.5, InputTokens: 3000}`
- **WHEN** encoded with `encoding/json`
- **THEN** the output contains `"totalCost":0.5` and `"inputTokens":3000` plus the four other pinned keys (`fact_test.go` `TestTotalsJSONTagsArePinned`)

### Requirement: Totals arithmetic is pure
`func (t Totals) Add(o Totals) Totals` MUST return the field-wise sum of all six fields without mutating either operand. `func (t Totals) IsZero() bool` MUST report true only when all six fields are zero (it compares against `Totals{}`).

#### Scenario: Add is field-wise and non-mutating
- **GIVEN** `a := Totals{TotalCost: 1, InputTokens: 2}` and `b := Totals{TotalCost: 0.5, OutputTokens: 3}`
- **WHEN** `c := a.Add(b)`
- **THEN** `c == Totals{TotalCost: 1.5, InputTokens: 2, OutputTokens: 3}`, `a` and `b` are unchanged, `Totals{}.IsZero()` is true, and `c.IsZero()` is false

### Requirement: Records flow downstream unmodified
Query aggregation sums records via `Totals.Add` and groups by `Date`/`User`/`Machine` (see [aggregation](/query/aggregation.md)); the live adapter produces them from ccusage JSON (see [ccusage-adapter](/source/ccusage-adapter.md)). `fact` itself MUST stay pure: no I/O, no printing, no exec.

## Design Decisions

### Date is an ISO label string, not time.Time
**Decision**: `fact.Record.Date` is `string` in the two label forms `YYYY-MM-DD` and `YYYY-MM`.
**Why**: `YYYY-MM` roll-up labels have no instant; every filter relies on lexicographic ISO order being the total order; it keeps the fact type free of time-zone semantics the render layer never needs.
**Rejected**: `time.Time` — forces a fake instant for months and re-introduces local/UTC ambiguity into the data layer.
*Introduced by*: 260916-v0as-fact-source-ccusage
