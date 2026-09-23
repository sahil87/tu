---
type: memory
description: The render package's shared number formatting — FormatInt en-US grouping, FormatCost's ICU two-decimal rule, the FixedHalfUp (toFixed) and JSRound (Math.round) twins, and where each rounding rule is used
---

# Number Formatting

**Domain**: render

## Overview

`internal/render/format.go` holds the number formatting shared by every encoder ([ansi](/render/ansi.md), [markdown/csv](/render/csv-and-markdown.md), [json](/render/json.md)): en-US thousands grouping plus three distinct rounding rules. Costs are carried unrounded from [fact records](/fact/records.md) through the [view models](/view/table-model.md); rounding happens only here, never upstream.

## Requirements

### Requirement: FormatInt groups thousands

`render.FormatInt(n int64)` MUST render `n` with en-US comma grouping (`24400` → `"24,400"`). A negative renders its grouped magnitude behind `-`; the magnitude is computed in unsigned arithmetic (`uint64(-(n+1))+1`) so `math.MinInt64` does not overflow. Every token cell in the ANSI tables and the Markdown encoder renders through it.

#### Scenario: MinInt64 groups correctly
- **GIVEN** `n = math.MinInt64`
- **WHEN** `FormatInt` runs
- **THEN** it returns `"-9,223,372,036,854,775,808"`

### Requirement: FormatCost is the ICU two-decimal rule

`render.FormatCost(x float64)` MUST return `$` + the value with exactly two decimals and comma grouping. Zero (including negative zero) renders `"$0.00"`; a negative renders the sign after `$`. The rounding operates on the **shortest round-trip decimal representation** of the double (`strconv.FormatFloat(x, 'f', -1, 64)`) — the internal `round2` rounds that string to two fraction digits half away from zero, with carry into the integer part, then `group` commas the integer part. It MUST NOT be `strconv.FormatFloat(x, 'f', 2, 64)`, which rounds the exact binary value half-even.

#### Scenario: node-verified divergences from Go 'f'
- **GIVEN** costs `1.005` and `999999.995`
- **WHEN** `FormatCost` runs
- **THEN** it returns `"$1.01"` and `"$1,000,000.00"` (the carry runs off the front) where `FormatFloat('f', 2)` would give `"1.00"` and `"999999.99"`

### Requirement: FixedHalfUp is the toFixed twin

`render.FixedHalfUp(x float64, digits int) string` MUST format the **exact binary value** of `x` rounded half-up to exactly `digits` fraction digits: a `big.Rat` holds the exact value, scaled by `10^digits`, truncated, rounded up when the remainder ≥ 1/2. The magnitude is rounded and the sign re-added only when the result is nonzero, so `-0` and a negative that rounds to zero render unsigned. It MUST NOT be `strconv.FormatFloat(x, 'f', digits, 64)` (half-even on exact ties). Users: the leaderboard's share cell at one digit (`render.FixedHalfUp(share*100, 1) + "%"` in `internal/view/leaderboard.go`'s `shareCell` and in `render/markdown.Leaderboard`) and `csv.Cost` at two.

#### Scenario: the one-digit tie
- **GIVEN** `x = 12.25`, `digits = 1`
- **WHEN** `FixedHalfUp` runs
- **THEN** it returns `"12.3"` where Go's `'f', 1` gives `"12.2"`; `FixedHalfUp(-0.004, 2)` returns `"0.00"` (sign dropped)

### Requirement: JSRound is the Math.round twin

`render.JSRound(x float64) float64` MUST be `math.Floor(x + 0.5)` — half toward +∞, NOT Go's `math.Round` (half away from zero) — with `-0` normalized to `0`. It backs the leaderboard's Δ percent (`fmtDeltaCell`/`view.DeltaCell` in `internal/view/leaderboard.go`), the CSV share/delta fractions (`csv.csvShare`), and the watch panel's session stats.

#### Scenario: negative half
- **GIVEN** `x = -553.5`
- **WHEN** `JSRound` runs
- **THEN** it returns `-553` where `math.Round` gives `-554`

## Design Decisions

### Costs carried unrounded to render time

**Decision**: cost is a `float64` dollar value from `fact.Totals.TotalCost` through every aggregation and view model; only the render functions round (two decimals in tables/CSV/Markdown), and [JSON](/render/json.md) emits the raw summed double.
**Why**: the raw-double JSON surface (DC-10) makes summation order observable, and rounding early would compound across the per-day, roll-up and group folds.
**Rejected**: rounding to cents at aggregation — a value rounded twice can differ from one rounded once.
*Introduced by*: 260915-2y3l-spec-reconciliation

### ICU shortest-repr rounding for costs

**Decision**: `FormatCost` rounds the shortest decimal representation of the double, half away from zero, with carry — not the exact binary value.
**Why**: parity with the frozen `src/node/` oracle (until plan row Z1) — the shortest-digit algorithms agree on both sides, so rounding that string matches the oracle where Go's half-even `'f', 2` diverges on node-verified inputs (`1.005`, `0.125`, `0.015`, `2.675`, `999999.995`).
**Rejected**: `strconv.FormatFloat(x, 'f', 2, 64)` — wrong on every tie above.
*Introduced by*: 260916-3am6-query-view-render-snapshot

### Exact JS rounding twins

**Decision**: `FixedHalfUp` (a `big.Rat` over the exact binary value, magnitude rounded half-up, sign re-added) is the `toFixed(digits)` twin for digits ∈ {1, 2}; `JSRound = floor(x + 0.5)` is the `Math.round` twin.
**Why**: parity with the frozen `src/node/` oracle (until plan row Z1) — the leaderboard share/Δ cells and the CSV fractions are byte surfaces, and Go's `FormatFloat('f', d)` and `math.Round` each differ on ties the harness seed can hit.
**Rejected**: `strconv.FormatFloat(x, 'f', digits, 64)` / `math.Round` — correct on almost every input, wrong on the ties; reusing `FormatCost` for CSV — wrong on `1.005`, `0.015`, `1.045` (a different rule, not a closer one).
*Introduced by*: 260916-9ax5-history-and-periods, generalized by 260916-2gbb-leaderboard-lb-lbh

### Formatting lives in the parent render package

**Decision**: `FormatInt`/`FormatCost`/`FixedHalfUp`/`JSRound` sit in `internal/render`, imported by the encoder subpackages.
**Why**: one implementation of each rule serves the encoders that share it.
**Rejected**: formatting inside `view` (the rules belong to the encoder family); one shared cost function for tables and CSV (two different rounding rules — see the twins decision).
*Introduced by*: 260916-3am6-query-view-render-snapshot
