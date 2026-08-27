# Plan: Total-Only History Flag (`--total` / `-t`)

**Change**: 260826-cd60-total-only-history-flag
**Intake**: `intake.md`

## Requirements

### CLI: Flag Parsing and Scope

#### R1: `--total` / `-t` boolean flag
`parseGlobalFlags` (`src/node/core/cli.ts`) MUST detect `--total` or `-t` as a boolean (`totalFlag: boolean` on `GlobalFlags`, default `false`), strip both spellings from `filteredArgs` via the boolean skip list, and return the flag for `main()`.

- **GIVEN** `["mh", "-t"]` or `["mh", "--total"]`
- **WHEN** parsed
- **THEN** `totalFlag === true` and `filteredArgs` is `["mh"]`

#### R2: All-tools-history scope guard
`main()` MUST warn once on stderr (`Warning: --total applies to all-tools history — ignoring.`) and clear `totalFlag` when it is set but the invocation is not `source === "all" && display === "history"`, at the same top-level spot as the `--since/--until` and `--metric` guards (watch snapshot warns once at startup). Exit code stays 0.

- **GIVEN** `tu -t` (snapshot) or `tu cc mh -t` (single tool)
- **WHEN** run
- **THEN** the warning prints, output renders as without the flag, exit 0

#### R3: Threading through `withCap`
`FormatOptions` MUST gain `total?: boolean` (absent ≡ false); `main()`'s `withCap` merge MUST stamp `total: true` only when the flag is set, so one-shot (`dispatchAllHistory`) and watch (`dispatchAllHistoryLines`) paths both receive it and default output stays byte-identical. JSON/CSV/MD emitters ignore it.

- **GIVEN** `tu -w mh -t`
- **WHEN** each poll renders
- **THEN** the collapsed layout renders every poll; no per-poll warning

### Display: Collapsed Pivot

#### R4: Total-only layout in `renderTotalHistory`
When `opts.total` is set (non-compact), `renderTotalHistory` MUST render `Date | <value> | bar` with no tool columns: header `Date | Cost` (or `Date | Tokens` under `metric: "tokens"`), value cell `fmtMetric(barTotal, metric)` at `COST_WIDTH` (widened to fit token counts when tokens), a **solid** `renderScaledBar` bar (same `computeBarScale` two-zone scale), a `Total` row with the grand value only, and the footer without the tool legend. Month separators, current-period marker, weekend dimming, `maxRows`, `prevCosts` indicator and "No data" behave as today. The heading is unchanged. With `total` absent/false output MUST be byte-identical to today.

- **GIVEN** two tools with entries, `{ total: true }`
- **WHEN** rendered
- **THEN** the header (stripped) is `Date       |      Cost` followed by the divider, no tool names appear anywhere, bars are present with the longest on the highest-cost row, the Total row has one value, and the footer has no `█ <tool>` swatches

- **GIVEN** `{ total: true, metric: "tokens" }`
- **WHEN** rendered
- **THEN** the header shows `Tokens`, value cells are `fmtNum` token totals, and the footer formats via `fmtNum`

### Docs: Surface Lockstep

#### R5: Help, completions, docs, specs, version
`FULL_HELP` MUST add `  --total / -t         Collapse all-tools history to Date + total + bar (no per-tool columns)` after `--metric`; `README.md` § Flags, `docs/site/workflows.md` (bullet + recipe `tu mh -u all -t`), `docs/site/skill.md`, `docs/specs/usage.md` (Global Flags row), `docs/specs/layouts.md` § 4 (collapsed-layout bullet) MUST mirror it; `src/node/core/completions.ts` MUST add `--total`/`-t` to bash/zsh/fish; `package.json` MUST bump to `0.12.0`.

- **GIVEN** the change is applied
- **WHEN** `tu help` prints
- **THEN** it contains `--total / -t`

### Non-Goals
- Collapsing CSV/Markdown output (CSV has a positional column contract; Markdown mirrors the ANSI pivot columns).
- Auto-collapsing on narrow terminals (implicit output change).
- A `tu total` command.

### Design Decisions

#### Collapse inside `renderTotalHistory`, not a new renderer
**Decision**: `total` is a branch in `renderTotalHistory` that empties the visible tool list so the existing per-column-width machinery yields the `Date | value | bar` shape; only the value cell, bar call (solid vs stacked), Total row and legend branch.
**Why**: one renderer, one code path for separators/markers/footer/scale; ≤ ~25 LOC delta.
**Rejected**: a separate `renderTotalOnlyHistory` (duplicates every row-decoration rule).
*Introduced by*: 260826-cd60-total-only-history-flag

## Tasks

### Phase 1: Core Implementation

- [x] T001 Add `totalFlag` parsing (`--total`/`-t`, boolean skip list) to `parseGlobalFlags`, the all-tools-history warn-and-clear guard in `main()`, and `total: true` stamping in `withCap` in `src/node/core/cli.ts`; add `total?: boolean` to `FormatOptions` in `src/node/tui/formatter.ts`; tests in `cli-parser.test.ts` (parse/strip/default) and `cli-exit-codes.test.ts` (snapshot and single-tool `-t` warn, exit 0) <!-- R1, R2, R3 -->
- [x] T002 Implement the collapsed layout in `renderTotalHistory` (`src/node/tui/formatter.ts`): empty visible tool list, `Cost`/`Tokens` header, `fmtMetric` value cell, solid `renderScaledBar`, grand-total-only Total row, legend-less footer; tests in `formatter-history.test.ts` (header/no tool names, bars present + longest on max row, Total row single value, no legend, tokens header/values, absent/false byte-identical, separators/maxRows still apply) <!-- R4 -->

### Phase 2: Polish

- [x] T003 Update `FULL_HELP`, `src/node/core/completions.ts` (bash/zsh/fish long + short lists), `completions.test.ts` (`LONG_FLAGS` + `SHORT_FLAGS`), `cli-help.test.ts` <!-- R5 -->
- [x] T004 Update `README.md` § Flags, `docs/site/workflows.md` (bullet + recipe), `docs/site/skill.md`, `docs/specs/usage.md`, `docs/specs/layouts.md` § 4 <!-- R5 -->
- [x] T005 Bump `package.json` to `0.12.0`; run `npx tsc --noEmit -p .`, `env -u TU_METRICS_REPO npm test`, `scripts/build.sh`; smoke `tu mh -u all -t` and `tu mh -u all -t --metric tokens` at 80 cols <!-- R5 -->

## Acceptance

### Functional Completeness

- [x] A-001 R1: `-t` and `--total` set `totalFlag`, are stripped from `filteredArgs`; default false
- [x] A-002 R2: Non-all-tools-history invocations with `-t` warn once (`Warning: --total applies to all-tools history — ignoring.`) and clear the flag, exit 0
- [x] A-003 R3: `FormatOptions.total` reaches one-shot and watch all-tools history renders via `withCap`; nothing stamped when false
- [x] A-004 R4: `renderTotalHistory` with `total` renders `Date | Cost | bar`, solid bars, grand-total-only Total row, legend-less footer
- [x] A-005 R4: Under `metric: "tokens"` the collapsed header reads `Tokens` and values/footer use `fmtNum`
- [x] A-006 R5: help/README/workflows/skill/usage/layouts/completions updated in lockstep; `package.json` at `0.12.0`

### Behavioral Correctness

- [x] A-007 R4: With `total` absent or false, `renderTotalHistory` output is byte-identical to pre-change
- [x] A-008 R4: Month separators, current-period marker, weekend dimming, `maxRows`, `prevCosts` indicator and "No data" behave identically under `total`

### Scenario Coverage

- [x] A-009 R4: Test asserts no tool name appears in any line and the longest bar sits on the highest-cost row under `{ total: true }`
- [x] A-010 R2: Subprocess tests pin the warning + exit 0 for `tu -t` and `tu cc mh -t`
- [x] A-011 R1: Completion scripts list `--total` and `-t` (bash/zsh/fish) and the LONG/SHORT flag arrays cover them

### Edge Cases & Error Handling

- [x] A-012 R3: `--total` with `--json`/`--csv`/`--md` is a silent no-op (CSV keeps all columns)
- [x] A-013 R4: Collapsed layout fits an 80-col terminal with the full 30-char bar

### Code Quality

- [x] A-014 Pattern consistency: flag mirrors `--full` boolean shape; guard mirrors existing warn-and-clear blocks; `node:` imports, `.js` extensions
- [x] A-015 No unnecessary duplication: reuses `computeBarScale`, `renderScaledBar`, `fmtMetric`, `renderHistoryFooter`, the column-width machinery; no second renderer
- [x] A-016 Minimum pathways: `total` rides the single `withCap` seam
- [x] A-017 Functions stay focused; `renderTotalHistory` grows by a small branch, not a fork
- [x] A-018 Error paths warn on stderr; nothing swallowed

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | Value column width widens to 15 under `total` + `metric: tokens` (token totals exceed 9 chars) | Team-level token totals run to 11–15 chars with separators; a 9-wide cell would overflow and misalign the bars | S:60 R:90 A:85 D:75 |
| 2 | Certain | Solid (unstacked) bar under `total` | No tool columns to key segment colors to; intake decision | S:85 R:90 A:90 D:90 |
| 3 | Confident | `--total` on all-tools history with `--by-machine` needs no new guard | `--by-machine` is already warn-and-cleared there | S:70 R:90 A:85 D:80 |
| 4 | Certain | Five tasks → light lane | Two code tasks, three lockstep/verification tasks | S:80 R:90 A:90 D:85 |

4 assumptions (2 certain, 2 confident, 0 tentative).
