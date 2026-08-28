# Plan: Token Table Mode and the `-t` Short Flag

**Change**: 260828-018g-tokens-table-mode-t-flag
**Intake**: `intake.md`

> Stacked on `260828-7x4i-cost-column-autosize-dim-zeros` (PR #73). Branch forks from that branch; the PR opens with `--base 260828-7x4i-cost-column-autosize-dim-zeros`.

## Requirements

### CLI: `-t` short flag

#### R1: `-t` is a boolean shorthand for `--metric tokens`
`parseGlobalFlags` (`src/node/core/cli.ts`) MUST accept `-t` as a boolean flag that sets `metricFlag = "tokens"`, strip it from `filteredArgs`, and add no new `GlobalFlags` field. `--metric <cost|tokens>` remains the explicit long form. `-t --metric tokens` is accepted silently.

- **GIVEN** `tu -t mh`
- **WHEN** flags are parsed
- **THEN** `metricFlag === "tokens"` and `filteredArgs` equals `["mh"]`

#### R2: `-t` with `--metric cost` is a usage error
`-t` combined with an explicit `--metric cost` (either order) MUST print `Error: -t and --metric cost are incompatible` on stderr and exit `2` (`EXIT_USAGE`). An invalid `--metric` value still fails first with its existing message.

- **GIVEN** `tu -t --metric cost`
- **WHEN** invoked
- **THEN** stderr contains the exact message and the exit code is 2

#### R3: Help, completions and docs carry `-t`
`FULL_HELP` MUST replace the `--metric` line with `  --metric <m>         Show 'cost' (default) or 'tokens' in every table cell, bar and footer stat` and add `  -t                   Shorthand for --metric tokens` directly under it (21-column alignment). `README.md`'s Flags block mirrors both lines verbatim. `completions.ts` adds `-t` to the bash and zsh `short_flags`, the zsh `_arguments` list (`'-t[show tokens instead of cost]'`), and fish (`complete -c tu -s t -d 'show tokens instead of cost'`); the `--metric` descriptions in completions say "show cost or tokens". `docs/site/skill.md` (~101–103) and `docs/site/workflows.md` (~85) are updated per intake §1.

- **GIVEN** the help-dump / completions tests
- **WHEN** run
- **THEN** `FULL_HELP` contains a `-t` line mentioning `--metric tokens`; `completions.test.ts` `SHORT_FLAGS` includes `-t` and passes

#### R4: Snapshot warn-and-ignore is removed
The `main()` block that warns `Warning: --metric applies to history display — ignoring.` and resets `metricFlag` MUST be deleted; `metricFlag` reaches every display via `withCap` unchanged.

- **GIVEN** `tu -t` (snapshot)
- **WHEN** invoked
- **THEN** stderr carries no `--metric` warning and the snapshot renders

### Display: metric-generic helpers

#### R5: Column width, cell dimming, and delta formatting take the metric
`costColumnWidth`/`costCell` MUST be replaced by `metricColumnWidth(values, metric)` (floor `COST_WIDTH`, longest `fmtMetric`) and `metricCell(value, width, metric)` (pad then `dim()` iff `value === 0`). `fmtMetricDelta(current, key, metric, prevCosts)` is added; `fmtCostDelta` stays exported as `fmtMetricDelta(current, key, "cost", prevCosts)`. `barValue` becomes the exported `metricValue(e: UsageTotals, metric)`. `BarMetric` keeps its name with an updated comment.

- **GIVEN** a token column with cells `0`, `9,999,999`, `487,683,047`
- **WHEN** rendered under tokens
- **THEN** the column is 11 wide, the `0` cell is dimmed, and all cells right-align

#### R6: Significance and nonzero filters key on the displayed metric
`nonzeroCostTools` → `nonzeroTools`; `significantCostTools` → `significantTools(toolNames, valueMap, labels, metric)` keeping a tool iff `total ≥ negligibleAbs(metric) AND total ≥ NEGLIGIBLE_SHARE × grand`, with `NEGLIGIBLE_COST_ABS = 1.0`, `NEGLIGIBLE_TOKENS_ABS = 1_000`, and one shared `NEGLIGIBLE_SHARE = 0.001` (renamed from `NEGLIGIBLE_COST_SHARE`). Fallback chain unchanged. `emitMarkdownTotalHistory` keeps exact-zero omission over cost.

- **GIVEN** token mode with tools at `999`, `1,000`, and `5,000` tokens against a `10,000,000` grand
- **WHEN** the pivot renders
- **THEN** `999` (< abs) and `5,000` (0.05%) are omitted and `1,000` is kept only if ≥ 0.1% (here omitted: 0.01%); a `$0.00`-cost tool with real tokens is kept in token mode

### Display: token mode in every renderer

#### R7: Cross-tool pivot renders in the displayed metric
`renderTotalHistory` MUST collapse `costMap`/`barMap` into one `valueMap` of `metricValue(e, metric)`; per-tool cells, the row column, Total row, widths, filter and delta indicator all use it. The last header is `Cost` under cost, `Tokens` under tokens; the title is `Combined Cost History` / `Combined Token History`. Bars, segments, legend and footer are unchanged. Compact pivot (`renderCompactTotalHistory`) renders values via `fmtMetricDelta`/`fmtMetric` with `COMPACT_COST_W = 12` unchanged.

- **GIVEN** `metric: "tokens"`
- **WHEN** the pivot renders
- **THEN** every cell and the Total are `fmtNum` token counts, the header reads `Tokens`, and `stripAnsi` of the row bars equals the unstacked bar of the row token total
- **AND** `metric: "cost"` output is byte-identical to a metric-less render

#### R8: Single-tool history swaps the last column's unit
`renderHistory` MUST keep its six token columns and swap the last column to `Tokens` = `totalTokens` under tokens (header, width via `metricColumnWidth`, cells via `metricCell`, Total via `fmtMetric`, delta on that column). Compact history follows.

- **GIVEN** `metric: "tokens"`
- **WHEN** rendered
- **THEN** the last column header is `Tokens`, its cells equal the `Total` column values, and cost mode is byte-identical

#### R9: Machine columns follow the displayed metric; JSON keeps cost
`buildHistoryMachineCosts(machineMap, metric)`, `buildSnapshotMachineCosts(..., metric)` and the inline single-tool snapshot builder in `cli.ts` MUST value the map in `metricValue(e, metric)`; the table arm receives the display metric, the `json`/`csv`/`md` arms receive a cost map (single map under cost mode). Renderers size/dim machine cells via `metricColumnWidth`/`metricCell`.

- **GIVEN** `tu -u all mh --by-machine -t`
- **WHEN** rendered
- **THEN** machine cells are token counts; `--json` `machines` values remain cost

#### R10: Snapshot renders unchanged in token mode except the delta placement
`renderTotal` MUST keep its columns and Cost cell in token mode; the watch delta indicator rides the `Tokens` cell under tokens (`fmtNum(t.totalTokens) + deltaIndicator(t.totalTokens, name, prevCosts)`) and the Cost cell under cost (byte-identical). Compact snapshot renders `fmtMetricDelta(metricValue(t, metric), name, metric, prevCosts)`.

- **GIVEN** a snapshot in token mode with `prevCosts`
- **WHEN** rendered
- **THEN** the arrow appears after the Tokens value and the Cost cell is plain `fmtCost`

#### R11: Watch prev map holds the displayed metric
`buildCostMap(data, metric, toolName?)` MUST value entries via `metricValue`; every `_lastRenderCostMap = buildCostMap(...)` call passes `fmtOpts?.metric ?? "cost"`. Keys unchanged. `_lastRenderCost`/`getCost`/`_lastRenderTotalTokens` (stats grid) untouched.

- **GIVEN** two watch polls in token mode where a row's tokens grew
- **WHEN** the second frame renders
- **THEN** that row shows `↑` computed from token values

### Docs

#### R12: Spec, README and site docs describe token mode
`docs/specs/usage.md` flag row (Short `-t`), exit-code row, and Output Formats notes; `docs/specs/layouts.md` token-mode bullets (Layouts 1, 3, 4, compact, help mockup) — per intake §7. `README.md`, `docs/site/skill.md`, `docs/site/workflows.md` per R3. Memory is hydrate's.

- **GIVEN** the docs diff
- **WHEN** read
- **THEN** every place that said "history-only"/"warns on snapshot" is gone and `-t` appears beside `--metric`

### Non-Goals
- `$/Mtok` ratio column/footer; input/output/cache as bar metrics; `-m`.
- Hiding the snapshot Cost column or the duplicated history last column in token mode.
- Stats-grid units in watch mode; `package.json` bump (release-time, "requires minor release" in PR body).
- Emitters (`emitCsv*`, `emitMarkdown*`), `watch.ts`, `compositor.ts`, `panel.ts`, `colors.ts`.

### Design Decisions

#### `-t` is a toggle, not `-m <value>`
**Decision**: `-t` ≡ `--metric tokens`; `--metric` stays the long form; `-m` untouched.
**Why**: Two summable facts want a boolean; `-m` is reserved for a future `--by-machine` short flag.
**Rejected**: `-m <cost|tokens>` — burns the letter and still requires a value.
*Introduced by*: 260828-018g-tokens-table-mode-t-flag

#### One metric-generic render path
**Decision**: Generalise `costColumnWidth`/`costCell`/`significantCostTools` over `BarMetric` and collapse the pivot's cost/bar maps into one `valueMap`.
**Why**: `code-quality.md` minimum pathways; `fmtMetric(v,"cost") === fmtCost(v)` makes cost mode provably byte-identical.
**Rejected**: A separate token renderer (second code path, drift risk).
*Introduced by*: 260828-018g-tokens-table-mode-t-flag

#### Token mode swaps the last history column's unit rather than dropping it
**Decision**: `renderHistory` keeps six columns; the last one reads `Tokens` under tokens.
**Why**: Shape stability — bar, delta and footer stay anchored to the last column; least surprising.
**Rejected**: Dropping the column (layout shift) or showing `$/Mtok` there (deferred feature).
*Introduced by*: 260828-018g-tokens-table-mode-t-flag

#### Snapshot table is metric-neutral; only its delta indicator follows the metric
**Decision**: `renderTotal` columns unchanged in token mode; the watch arrow moves to the Tokens cell.
**Why**: The snapshot is already token-denominated; comparing a dollar cell against a token prev map would be wrong, and two maps per poll is a second pathway.
**Rejected**: Hiding the Cost column; keeping the arrow on Cost with a cost map.
*Introduced by*: 260828-018g-tokens-table-mode-t-flag

#### Significance filter keys on the displayed metric
**Decision**: `significantTools` uses `negligibleAbs(metric)` (`$1.00` / `1,000` tokens) and a shared `0.1%` share.
**Why**: A zero-priced tool with real tokens is a real column in token mode and noise in cost mode; the visible table must be self-consistent.
**Rejected**: Always keying on cost (drops non-empty token columns).
*Introduced by*: 260828-018g-tokens-table-mode-t-flag

#### Machine columns follow the displayed metric; JSON keeps cost
**Decision**: `cli.ts` map builders take the metric; emitter arms get a cost map.
**Why**: Per-machine `UsageEntry`s carry tokens, so the data exists; a cost block inside a token table is the inconsistency this change removes. JSON is a machine contract.
**Rejected**: Cost-only machine columns.
*Introduced by*: 260828-018g-tokens-table-mode-t-flag

## Tasks

### Phase 1: Setup

- [x] T001 `src/node/tui/formatter.ts`: replace `costColumnWidth`/`costCell` with `metricColumnWidth(values, metric)`/`metricCell(value, width, metric)`; add `fmtMetricDelta`; make `fmtCostDelta` a wrapper; rename/export `barValue` → `metricValue(e: UsageTotals, metric)`; update the `BarMetric` comment <!-- R5 -->
- [x] T002 [P] `src/node/tui/formatter.ts`: rename `nonzeroCostTools` → `nonzeroTools`, `significantCostTools` → `significantTools(toolNames, valueMap, labels, metric)`; constants `NEGLIGIBLE_COST_ABS`, `NEGLIGIBLE_TOKENS_ABS = 1_000`, `NEGLIGIBLE_SHARE = 0.001` + `negligibleAbs(metric)`; Markdown emitter keeps exact-zero over cost <!-- R6 -->

### Phase 2: Core Implementation

- [x] T003 `renderTotalHistory` + `renderCompactTotalHistory`: single `valueMap` of `metricValue`; cells/row column/Total/widths/filter/delta on it; `valueHeader` (`Cost`/`Tokens`); title `Combined Cost History`/`Combined Token History`; compact renders via `fmtMetricDelta`/`fmtMetric` <!-- R7 -->
- [x] T004 `renderHistory` + `renderCompactHistory`: last column header `valueHeader`, value `metricValue(e, metric)`, width/cells/Total/delta via the metric helpers; machine cells via `metricColumnWidth`/`metricCell`/`fmtMetric` <!-- R8, R9 -->
- [x] T005 `renderTotal` + `renderCompactSnapshot`: delta indicator on Tokens cell under tokens, Cost cell plain; cost mode byte-identical; machine cells via metric helpers; compact via `fmtMetricDelta(metricValue(t, metric), …)` <!-- R9, R10 -->
- [x] T006 `src/node/core/cli.ts`: `-t` parsing in `parseGlobalFlags` (boolean skip list + conflict → `Error: -t and --metric cost are incompatible`, exit `EXIT_USAGE`); update `FULL_HELP` (`--metric` line + `-t` line); delete the snapshot warn-and-ignore block in `main()` <!-- R1, R2, R3, R4 -->
- [x] T007 `src/node/core/cli.ts`: `buildCostMap(data, metric, toolName?)`, `buildHistoryMachineCosts(machineMap, metric)`, `buildSnapshotMachineCosts(..., metric)` and the inline snapshot builder use `metricValue`; every `_lastRenderCostMap = buildCostMap(...)` passes `fmtOpts?.metric ?? "cost"`; `renderHistoryByFormat`/`renderSnapshotByFormat` give the table arm the display metric map and the `json`/`csv`/`md` arms a cost map (single map under cost) <!-- R9, R11 -->

### Phase 3: Integration & Edge Cases

- [x] T008 `src/node/core/completions.ts`: add `-t` to bash/zsh `short_flags`, zsh `_arguments` (`'-t[show tokens instead of cost]'`), fish `complete -c tu -s t -d 'show tokens instead of cost'`; update the `--metric` descriptions to "show cost or tokens"; add `"-t"` to `SHORT_FLAGS` in `src/node/core/__tests__/completions.test.ts` <!-- R3 -->
- [x] T009 Tests: rewrite the `FormatOptions.metric` block in `src/node/tui/__tests__/formatter-history.test.ts` (~479–536, incl. "cells stay cost" → cells are tokens) and add token-mode cases there or in `formatter-widths.test.ts`: pivot cells/Total/header/title in tokens; single-tool last column `Tokens` = `totalTokens`; cost-mode byte-identity for pivot, history, snapshot, compact; `0`-token cells dimmed and `--no-color` byte-identical; token column widening past 9 with aligned bar starts; `significantTools` over tokens (`999` omitted, thresholds, `$0.00`-but-tokens kept in token mode, omitted in cost mode); machine columns in tokens; snapshot unchanged in token mode with delta on Tokens cell. `src/node/core/__tests__/cli-parser.test.ts`: `-t` sets tokens and is stripped; `-t --metric tokens` ok. `cli-exit-codes.test.ts`: `-t --metric cost` → exit 2 with the exact message; snapshot with `-t` → no stderr warning. `cli-help.test.ts`: `-t` line present. Run: `npx tsx --test src/node/tui/__tests__/formatter*.test.ts src/node/core/__tests__/cli-parser.test.ts src/node/core/__tests__/cli-exit-codes.test.ts src/node/core/__tests__/cli-help.test.ts src/node/core/__tests__/completions.test.ts`, then `env -u TU_METRICS_REPO npm test` and `npx tsc --noEmit` <!-- R1, R2, R3, R4, R5, R6, R7, R8, R9, R10, R11 -->

### Phase 4: Polish

- [x] T010 Docs: `README.md` Flags block (two lines verbatim from `FULL_HELP`); `docs/specs/usage.md` (flag row with Short `-t`, exit-code row, Output Formats notes, drop "warns and is ignored on snapshots"); `docs/specs/layouts.md` token-mode bullets for Layouts 1/3/4/compact and the help mockup; `docs/site/skill.md` ~101–103 and `docs/site/workflows.md` ~85 per intake §1 <!-- R3, R12 -->

## Execution Order

- T001 and T002 first (helpers); T003–T005 depend on both
- T006 before T007 (same file; T007 depends on `metricValue` from T001)
- T009 after T003–T008; T010 independent after T006

## Acceptance

### Functional Completeness

- [x] A-001 R1: `-t` parses as a boolean, sets `metricFlag: "tokens"`, is stripped from `filteredArgs`, adds no `GlobalFlags` field
- [x] A-002 R2: `-t` + `--metric cost` exits 2 with `Error: -t and --metric cost are incompatible` on stderr
- [x] A-003 R3: `FULL_HELP`, README, bash/zsh/fish completions, `docs/site/skill.md`, `docs/site/workflows.md` carry `-t`
- [x] A-004 R4: The snapshot `--metric` warning block is gone; `tu -t` snapshot prints no warning
- [x] A-005 R5: `metricColumnWidth`/`metricCell`/`fmtMetricDelta`/`metricValue` exist; `costColumnWidth`/`costCell` are gone; `fmtCostDelta` wraps `fmtMetricDelta`
- [x] A-006 R6: `significantTools`/`nonzeroTools` take the metric; constants `NEGLIGIBLE_COST_ABS`, `NEGLIGIBLE_TOKENS_ABS`, `NEGLIGIBLE_SHARE`; Markdown emitter unchanged (exact-zero over cost)
- [x] A-007 R7: Pivot uses one `valueMap`; header/title swap; compact pivot in the metric
- [x] A-008 R8: `renderHistory` last column swaps unit; shape unchanged; compact history in the metric
- [x] A-009 R9: Machine-map builders take the metric; table arm gets display metric, emitter arms get cost
- [x] A-010 R10: Snapshot columns unchanged in token mode; delta on Tokens cell under tokens
- [x] A-011 R11: `buildCostMap` takes the metric; all `_lastRenderCostMap` call sites pass it; stats grid untouched
- [x] A-012 R12: `docs/specs/usage.md` and `docs/specs/layouts.md` describe token mode and `-t`

### Behavioral Correctness

- [x] A-013 R7: Under `metric: "cost"` (and absent) pivot, history, snapshot and compact renders are byte-identical to pre-change (tests assert)
- [x] A-014 R7: Under tokens, pivot cells and Total are `fmtNum` counts and `stripAnsi(stacked bar) === unstacked bar` of the token row total
- [x] A-015 R8: Under tokens, history last column equals the `Total` column values with header `Tokens`
- [x] A-016 R10: Snapshot under tokens with `prevCosts` places `↑`/`↓` after the Tokens value; Cost cell is plain

### Scenario Coverage

- [x] A-017 R6: Token-mode omission: `999` tokens omitted, sub-0.1% omitted, `$0.00`-cost tool with tokens kept in token mode and omitted in cost mode
- [x] A-018 R9: `--by-machine -t` machine cells are tokens; `--json` `machines` values are cost
- [x] A-019 R1: `-t --metric tokens` is accepted silently
- [x] A-020 R4: Snapshot subprocess with `-t` has empty stderr

### Edge Cases & Error Handling

- [x] A-021 R5: `0`-token cells dim; `setNoColor(true)` output byte-identical to undimmed; token columns widen past 9 with aligned bar starts
- [x] A-022 R2: Invalid `--metric` value still fails first with its own message when combined with `-t`
- [x] A-023 R11: Token-mode watch delta reflects token growth, not cost

### Code Quality

- [x] A-024 Pattern consistency: plain functions, `type` imports, `.js` extensions, `node:` imports; error messages follow `Error: X and Y are incompatible`
- [x] A-025 No unnecessary duplication: one set of metric helpers; no token-specific renderer; `cli.ts` uses exported `metricValue` rather than reimplementing
- [x] A-026 Named constants for thresholds; no magic numbers
- [x] A-027 Minimum pathways: cost mode shares the exact code path with token mode (metric parameter, not branches per renderer)
- [x] A-028 Toolkit standards: `FULL_HELP` ↔ README flag lines identical (readme-extraction rule 7); help-dump structure unchanged
- [x] A-029 Tests co-located, Node built-in runner; scoped suites and `env -u TU_METRICS_REPO npm test` green; `tsc --noEmit` clean

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`
- PR: `gh pr create --draft --base 260828-7x4i-cost-column-autosize-dim-zeros …`; body notes "stacked on #73" and "requires minor release". Do not edit `package.json`.

## Deletion Candidates

None — this change adds new functionality without making existing code redundant. The superseded 7x4i helpers (`costColumnWidth`, `costCell`, `barValue`, `nonzeroCostTools`, `significantCostTools`, `NEGLIGIBLE_COST_SHARE`) were removed by the change itself rather than left behind; `fmtCostDelta` survives as a used wrapper (renderTotal cost mode + test imports).

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | New token-mode tests live in `formatter-history.test.ts` (existing metric block) plus `formatter-widths.test.ts` for width/dim/omission cases | Follows the split-by-concern sibling naming established by 7x4i | S:70 R:95 A:90 D:80 |
| 2 | Certain | `-t` joins the boolean skip list so it never reaches `filteredArgs`; conflict check runs after the existing `--metric` validation | Intake §1 code sketch; invalid value must fail first | S:90 R:95 A:95 D:90 |
| 3 | Confident | Under cost mode the emitter arms reuse the single display map (no second map built) | Intake §4 "simpler and preferred"; keeps cost mode byte-identical and allocation-free | S:70 R:90 A:90 D:80 |

3 assumptions (1 certain, 2 confident, 0 tentative).
