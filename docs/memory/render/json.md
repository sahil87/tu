---
type: memory
description: The JSON encoder's pinned wire shapes — the snapshot object keyed by display name, the history array and all-tools object, the ranked leaderboard array — key order, raw double costs, the conditional label and machines keys, and the hand-ordered writer
---

# JSON Encoding

**Domain**: render

## Overview

`internal/render/json` encodes the three displays as `JSON.stringify(v, null, 2)` lines: two-space indent, hand-ordered keys, raw float costs. Like every encoder it returns `[]string`; the [command edge](/command/run-and-result.md) writes the lines.

## Requirements

### Requirement: Snapshot is an object keyed by display name

`json.Snapshot(rows []view.ToolTotals, bd *view.Breakdown) []string` (`internal/render/json/snapshot.go`) MUST emit an object keyed by `view.ToolTotals.Name` (display name) in input order; per row `"label"` FIRST when `ToolTotals.Label != ""`, then the six pinned totals keys in order — `totalCost`, `inputTokens`, `outputTokens`, `cacheCreationTokens`, `cacheReadTokens`, `totalTokens` (the pinned names carried as the JSON tags on `fact.Totals`) — then, when `bd.Slices(name)` is non-empty, a `"machines"` object with the comma moved onto the `totalTokens` line: one `"name": cost` line per slice in FIRST-SEEN slice order (`bd.Slices`, never sorted), values raw doubles, always cost even under `-t`. Rows with no slices MUST be byte-identical to the no-breakdown output; a zero-usage single-source tool MAY carry `machines` with `0` values and no `label`. Two-space indent.

#### Scenario: breakdown attaches machines after totalTokens
- **GIVEN** one snapshot row `"Claude Code"` whose breakdown carries two slices
- **WHEN** `json.Snapshot` runs
- **THEN** the row object is `"label"` first, the six totals keys, then `"machines": { "Sahil's host": 0.3, "dev box": 0.2 }` in slice order, and a sibling row without slices carries no `machines` key

### Requirement: History is a bare entry array

`json.History(s view.Series, bd *view.Breakdown) []string` (`internal/render/json/history.go`) MUST emit a bare array of entry objects — `[]` inline when `s.Entries` is empty. Entry keys in order: `label` (ALWAYS present — the snapshot's conditional-label rule does not apply), `totalCost`, `inputTokens`, `outputTokens`, `cacheCreationTokens`, `cacheReadTokens`, `totalTokens`, then — when `bd.Slices(label)` is non-empty — a `machines` object in first-seen slice order with cost values. Entries with no slices are byte-identical to the no-breakdown output.

### Requirement: TotalHistory is an object of arrays

`json.TotalHistory(series []view.Series) []string` MUST emit an object keyed by `view.Series.Name` in input order, each value the bare entry array — `[]` inline for an empty series. It takes no breakdown. The leaderboard history reuses this shape unchanged: keys are users in repo order, every user present (`[]` inline when the window holds no entries), and under `--top` the kept users followed by `others` last. See [view/history](/view/history.md) and [view/leaderboard](/view/leaderboard.md) for the model.

### Requirement: Leaderboard is a bare ranked array

`json.Leaderboard(rows []view.LeaderboardRow) []string` (`internal/render/json/leaderboard.go`) MUST emit `[]` inline when empty, else one object per row with keys in order `rank` (int), `user`, `machine` (ONLY when `LeaderboardRow.Machine != ""`), `cost` (raw double from `LeaderboardRow.TotalCost`), `totalTokens` (int), `share` (raw fraction, not a percent), `delta` (raw fraction, or `null` for a `"new"` row — `LeaderboardRow.Delta` nil). The caller passes the already-sliced rows, so `--top` truncates the array. See [view/leaderboard](/view/leaderboard.md) for the row model.

#### Scenario: a new row emits delta null
- **GIVEN** a row with `Delta == nil` (no previous-window value)
- **WHEN** `json.Leaderboard` runs
- **THEN** the object ends with `"delta": null`

### Requirement: Scalars encode by the ES6 float rules

All float costs/shares/deltas MUST go through `encodeFloat` (`internal/render/json/snapshot.go`): the ES6 float rules that `encoding/json` implements (shortest representation, `1e+21`/`1e-7` exponent thresholds), with `-0` normalized to `0`. Strings encode without HTML escaping (`SetEscapeHTML(false)`). Integers encode as bare digits. The two-space indent and trailing-newline layout follow `JSON.stringify(v, null, 2)`; the trailing newline is the caller's per-line `Fprintln` at the [command edge](/command/run-and-result.md). Every shape is pinned by golden files under `internal/render/json/testdata/` (`go test ./internal/render/json/ -update` regenerates).

## Design Decisions

### The writer is hand-ordered
**Decision**: the encoder builds key order by hand and uses `encoding/json` only for scalars.
**Why**: Go maps are unordered and struct marshalling cannot emit the conditional `label` key first — the key order is a byte surface, so parity with the frozen golden corpus (`/harness/golden-corpus.md`, the retired TypeScript implementation's bytes) demands a hand-ordered writer.
**Rejected**: struct marshalling with `json` tags (wrong order for the conditional `label`); map marshalling (unordered).
*Introduced by*: 260916-3am6-query-view-render-snapshot

### Costs/shares/deltas are raw doubles
**Decision**: every float on the JSON wire is the raw summed double (`encodeFloat`); rounding exists only in [number formatting](/render/number-formatting.md) for the human-facing formats.
**Why**: the raw-double surface (DC-10) makes summation order reproducible byte-for-byte; a rounded wire would hide the association and break consumers that re-sum.
**Rejected**: two-decimal costs on the wire — rounds twice and breaks consumers that re-sum.
*Introduced by*: 260915-2y3l-spec-reconciliation

### machines attach in first-seen slice order
**Decision**: the `machines` object's keys follow `Breakdown.Slices` first-seen order, never sorted; a zero-usage single-source tool may carry `machines` with `0` values and no `label`.
**Why**: parity with the frozen golden corpus (`/harness/golden-corpus.md`, the retired TypeScript implementation's bytes) — the insertion-order copy and the single-source zero-fill are byte surfaces the harness diffs.
**Rejected**: sorted keys or dropping zero-value machines — both diverge from the pinned bytes.
*Introduced by*: 260916-pmsd-machine-columns
