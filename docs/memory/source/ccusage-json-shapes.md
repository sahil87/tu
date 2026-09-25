---
type: memory
description: The ccusage v20 per-agent daily JSON shapes Parse accepts — the {"daily":[...]} document, the ignored totals object, the codex costUSD/models{} outlier, the empty-agent daily: [] / -0.0 form, field fallbacks and coercion, and label normalization.
---

# ccusage JSON Shapes

**Domain**: source

## Overview

`Parse` in `src/go/internal/source/ccusage/normalize.go` converts one ccusage v20 per-agent document into `[]fact.Record`. The accepted shapes are live-verified against real captures of all six subcommands; the committed placeholder corpus under `harness/fixtures/_placeholder/` mirrors them (see [placeholder-corpus](/harness/placeholder-corpus.md)).

## Requirements

### Requirement: Accepted document shape
`Parse(raw, tool)` SHALL decode a JSON object with a `"daily"` array of entry objects. The `"totals"` object and unknown keys (`modelBreakdowns`, `models`, `modelsUsed`, `reasoningOutputTokens`, …) MUST be ignored. A missing `"daily"` key, a non-array `"daily"` value (including `null`), a non-object document, or empty/garbage input MUST yield a `KindParse` error (detail one of `stdout is not a JSON object`, `stdout has no "daily" array`, `"daily" is not an array`) and nil records.

#### Scenario: Empty versus malformed
- GIVEN `{"daily":[],"totals":{"totalCost":-0.0}}`
- WHEN parsed
- THEN it returns a non-nil empty slice and no error
- GIVEN `""`, `"not json"`, `"[]"`, `{"totals":{}}`, `{"daily":{}}`, `{"daily":"daily"}`, or `{"daily":null}`
- WHEN parsed
- THEN each returns a `KindParse` error (which never warns — see [errors-and-warnings](/source/errors-and-warnings.md)) and nil records

### Requirement: Entry field mapping
Each `daily[]` entry SHALL map to `fact.Record{Date, Tool, Totals}` with: `TotalCost` ← `totalCost`, else `costUSD`, else 0; `CacheReadTokens` ← `cacheReadTokens`, else `cachedInputTokens`, else 0; `InputTokens`/`OutputTokens`/`CacheCreationTokens`/`TotalTokens` ← the same-named keys, else 0. A present key whose value is not a JSON number MUST coerce to 0 — never an error. `User`/`Machine` stay empty; the adapter stamps them (see [ccusage-adapter](/source/ccusage-adapter.md)).

#### Scenario: Fallbacks and coercion
- GIVEN `{"daily":[{"date":"2026-01-05","costUSD":1.25}]}`
- WHEN parsed
- THEN `TotalCost` is 1.25 via the `costUSD` fallback
- GIVEN `{"daily":[{"date":"Feb 14, 2026","inputTokens":"12","cachedInputTokens":7}]}`
- WHEN parsed
- THEN `InputTokens` is 0 (non-numeric coerces to 0) and `CacheReadTokens` is 7 via `cachedInputTokens`

### Requirement: Per-agent shapes at ccusage v20
`claude`, `gemini`, `kimi`, `opencode`, and `copilot` daily entries carry `totalCost`, the token counters, `modelBreakdowns[]`, and `modelsUsed[]`. `codex` is the outlier: `costUSD` in place of `totalCost`, a `models{}` map keyed by model name, and `reasoningOutputTokens` — the reason the cost and cache-read mappings carry fallbacks. An agent with no transcripts returns exit 0 with `daily: []` and `totals.totalCost: -0.0`; `Parse` MUST treat that as a legitimate zero result, not an error.

#### Scenario: Placeholder corpus parses uniformly
- GIVEN `harness/fixtures/_placeholder/codex/daily.json`
- WHEN parsed with the codex tool
- THEN it yields 3 records dated 2026-01-05, 2026-01-06, 2026-01-07, each `Totals{TotalCost: 0.5, InputTokens: 3000, OutputTokens: 400, CacheCreationTokens: 1000, CacheReadTokens: 20000, TotalTokens: 24400}` via the `costUSD` path; the other five fixtures yield the same dates and totals

### Requirement: Label normalization
`Date` SHALL come from the entry's label key (`invocations[tool.Key].labelKey`, `"date"` for all six per-agent subcommands at ccusage v20) passed through `normalizeLabel`: ISO labels pass through unchanged; `"Feb 14, 2026"` becomes `"2026-02-14"` (day zero-padded); `"Feb 2026"` becomes `"2026-02"`; an unknown 3-letter month maps to `"00"`; a missing or non-string label yields `""`.

#### Scenario: Label forms
- GIVEN the labels `"2026-02-14"`, `"Feb 14, 2026"`, `"Jan 3, 2026"`, `"Feb 2026"`, `"Xyz 5, 2026"`
- WHEN normalized
- THEN they become `"2026-02-14"`, `"2026-02-14"`, `"2026-01-03"`, `"2026-02"`, and `"2026-00-05"`

## Design Decisions

### Cost and cache-read fallbacks
**Decision**: cost reads `totalCost` then `costUSD`; cache-read tokens read `cacheReadTokens` then `cachedInputTokens`.
**Why**: live-verified ccusage v20 captures show codex emitting `costUSD` and `cachedInputTokens` where the other five agents emit `totalCost`/`cacheReadTokens`; one mapping with fallbacks covers both shapes (r7dh).
**Rejected**: per-agent parsers (one mapping covers the divergence).
*Introduced by*: 260915-r7dh-harness-fixture-capture

### Empty agent is a legitimate zero
**Decision**: `{"daily":[]}` returns zero records and no error, and the adapter returns before any cache write.
**Why**: an agent with no transcripts exits 0 with `daily: []` and `totals.totalCost: -0.0`; treating it as an error would warn about agents the user simply has not used (r7dh).
**Rejected**: treating an empty `daily` as a parse failure.
*Introduced by*: 260915-r7dh-harness-fixture-capture

### Per-tool label key, uniformly "date"
**Decision**: the entry's label key lives per tool in the invocations map; every value is `"date"` at ccusage v20.
**Why**: the key can vary by serializer — the unused bare all-agents aggregate emits `"period"` — so a per-tool field keeps a future divergence a data-only change; all six per-agent subcommands emit `"date"` (gmcp, live-verified r7dh).
**Rejected**: a single shared const (cannot distinguish tools once the spelling diverges by tool).
*Introduced by*: 260703-ccfx-fix-cc-source-mapping

### Coercion to 0, never an error
**Decision**: a present non-number value coerces to 0; a missing key falls through to the next fallback.
**Why**: mirrors the upstream coercion semantics — parity with the frozen golden corpus (`/harness/golden-corpus.md`, the retired TypeScript implementation's bytes); ccusage never emits numeric strings, so the difference from a numeric-string parse is unobservable on real data.
**Rejected**: erroring on a malformed entry (one bad row would fail the whole document).
*Introduced by*: 260916-v0as-fact-source-ccusage

### No noise-stripping pre-pass
**Decision**: non-JSON stdout is a `KindParse` error; there is no line-stripping pre-pass.
**Why**: ccusage v20 emits clean JSON on stdout (live-verified); a strip pass would silently mask a real upstream breakage.
**Rejected**: stripping `[`-prefixed lines for every tool (a defensive no-op that hides upstream regressions).
*Introduced by*: 260916-v0as-fact-source-ccusage
