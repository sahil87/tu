---
type: memory
description: "The committed _placeholder fixture corpus — tudiff placeholder writes deterministic schema-derived daily fixtures for all six ccusage sources (the codex costUSD/models{} outlier, the claude-style shape elsewhere), merges the human-owned confirmed.json ledger into unconfirmed/confirmed_by flags, validates the ledger before any write, and refuses to overwrite a real alias."
---
# Placeholder Corpus

**Domain**: harness

## Overview

`tudiff placeholder` generates the committed, schema-derived `harness/fixtures/_placeholder/` corpus that CI replays, and merges the human-owned `confirmed.json` ledger into its manifest. Layout, manifest schema and capture of real fixtures are in [fixture-corpus](/harness/fixture-corpus.md).

## Requirements

### Requirement: tudiff placeholder
`tudiff placeholder [--source <s>…] [--out <dir>]` (default: all six sources in registry order; default out `harness/fixtures/_placeholder`) SHALL write deterministic, schema-derived fixtures from Go structs mirroring the observed ccusage v20 per-agent daily shapes: **codex** in its observed outlier shape (`costUSD`, a `models{}` map keyed by model name with `isFallback`/`reasoningOutputTokens`, and `reasoningOutputTokens` on entries and totals — `PlaceholderCodex`), every other source in the claude-style shape (`totalCost`, `modelBreakdowns[]`, `modelsUsed[]` — `Placeholder`); `PlaceholderFor(source)` dispatches. Each fixture has three fixed consecutive days `2026-01-05..2026-01-07`, model name `placeholder-<source>-model`, alphabetically ordered keys, 2-space indentation, `totals` equal to the column sums, and `totalTokens` equal to the sum of the four token counters in every entry and in `totals` — internally consistent so tu's aggregation can be checked against it. The `_placeholder` manifest carries `machine: "_placeholder"`, `ccusage_version: "20.0.19"`, `ccusage_path: ""`, `platform: "derived"`, and a provenance `derived_from` sentence naming the live probe the shapes came from. Every entry is `unconfirmed: true` unless its source is listed in the **confirmation ledger** `<out>/confirmed.json` (`harness.ConfirmedLedgerFile`, read by `ReadConfirmed`): a committed JSON object keyed by source name whose values are `{"machine", "date", "ccusage_version"}`. A listed source's entry is `unconfirmed: false` with that ledger object as `confirmed_by`; an unlisted source keeps `unconfirmed: true` and no `confirmed_by`. A missing ledger is an empty ledger (all entries unconfirmed). The ledger is validated before any file is written and a violation exits 1 with nothing touched: malformed JSON, a document that is not a single JSON object (a top-level `null`, or any trailing value or content after the object), a key outside `DefaultSources`, a missing `machine`/`date`/`ccusage_version`, a `date` not matching `^\d{4}-\d{2}-\d{2}$`, or an unknown field (`DisallowUnknownFields`) — each error names `confirmed.json` and the offending source/field. Ledger keys outside a `--source` subset are validated but emit no entry; the ledger changes flags only, never fixture bytes or `sha256`. The `ccusage_version` in the ledger is recorded, not compared against `PlaceholderVersion`. Per source, stdout reports the merged state as `<source> daily --json  placeholder (confirmed: <machine> <date>)  -> <out>/<file>` or `… placeholder (unconfirmed)  -> …`, read back from the manifest just written. The committed ledger lists `claude`, `codex`, `gemini`, `kimi` (confirmed on `dev-ws-sahil02`, 2026-09-16, ccusage 20.0.19); `opencode` and `copilot` stay unconfirmed until a machine with data confirms them (plan row P3b). The generator refuses (exit 1) an `--out` that already holds a real alias's manifest; a local capture alongside `_placeholder/` is expected and is never read or modified.

#### Scenario: Ledger merge
- **GIVEN** `<out>/confirmed.json` listing `codex` and no other source
- **WHEN** `tudiff placeholder --source codex --source opencode --out <out>` runs
- **THEN** the codex entry is `unconfirmed: false` with `confirmed_by` equal to the ledger object, the opencode entry is `unconfirmed: true` with no `confirmed_by`, and stdout shows `codex … (confirmed: <machine> <date>)` and `opencode … (unconfirmed)`

#### Scenario: Placeholder output is deterministic
- **GIVEN** `tudiff placeholder --source opencode --out <tmp>` run twice
- **WHEN** the two outputs are compared
- **THEN** they are byte-identical, decode into the schema structs, and `totals.totalTokens` equals the sum over `daily[].totalTokens`

## Design Decisions

### Real empty captures are confirmed; only `_placeholder/` is unconfirmed
**Decision**: An exit-0 `daily: []` capture is `empty: true, unconfirmed: false`; placeholders are the only `unconfirmed: true` entries, and `confirmed_by` appears only on placeholder entries a human has confirmed — a real capture is confirmed by being real, not by the ledger.
**Why**: "We observed nothing" and "we guessed the shape" are different facts; the harness must report the second separately.
**Rejected**: Marking empties unconfirmed (conflates the two).
*Introduced by*: 260915-r7dh-harness-fixture-capture

### Confirmation is a committed ledger merged by the generator, never a manifest edit
**Decision**: Human confirmation of a placeholder shape lives in `harness/fixtures/_placeholder/confirmed.json`; `WritePlaceholders` merges it into `manifest.json` (`unconfirmed: false` + `confirmed_by`), and the corpus test asserts the two agree.
**Why**: `manifest.json` is generated and rewritten wholesale, so a hand flip would be lost on the next regeneration and the `sha256` guard (which pins fixture bytes, not the manifest) cannot see it. A separate human-owned input keeps the manifest a pure function of fixtures plus ledger, so regeneration is idempotent with respect to confirmations and a stale pair fails CI.
**Rejected**: Hand-editing the manifest and never regenerating; encoding confirmation in the corpus-wide `derived_from` string (cannot carry per-source state, also overwritten); `--confirmed` flags on `tudiff placeholder` (state would live in shell history, not the repo); bumping `schema` to 2 (the field is optional and additive; real-capture manifests and the fake ccusage are unaffected).
*Introduced by*: 260916-di0s-placeholder-confirmation-ledger

### Ledger errors abort before any write
**Decision**: `ReadConfirmed` runs before the first file write in `WritePlaceholders`, and any error (malformed JSON, unknown source, missing or malformed field, unknown field) aborts with nothing touched.
**Why**: The ledger is hand-edited; a typo is the realistic failure, and half-written output would be worse than a loud refusal. Matches the existing all-or-nothing real-alias refusal.
**Rejected**: Warn-and-continue (the misspelled source silently stays unconfirmed, which is exactly the state the ledger exists to record).
*Introduced by*: 260916-di0s-placeholder-confirmation-ledger

### Placeholder shapes follow the observed per-agent serializer
**Decision**: codex placeholders use the codex shape (`costUSD`, `models{}`, `reasoningOutputTokens`); every other source uses the claude-style shape (`totalCost`, `modelBreakdowns[]`, `modelsUsed[]`).
**Why**: Both shapes were live-verified; the empty opencode/copilot outputs use `totalCost` in `totals`, so claude-style is the evidence-based guess for them, while codex is a known outlier that must be mirrored or the Go adapter's `costUSD` path goes untested in CI.
**Rejected**: One shape for all placeholders (contradicts observed output for one side or the other).
*Introduced by*: 260915-r7dh-harness-fixture-capture
