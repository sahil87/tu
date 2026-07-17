# Plan: Cap History Default Window

**Change**: 260717-yuuj-cap-history-default-window
**Intake**: `intake.md`

## Requirements

### CLI: Implicit 3-Month Cap on Daily/Weekly History

#### R1: Floor-computation helper
The CLI SHALL provide a pure helper that computes the implicit cap floor as the first day of the month two calendar months back, using the **local** system date, returning an ISO `YYYY-MM-DD` string.

- **GIVEN** the local date is 2026-07-17
- **WHEN** the floor is computed
- **THEN** the result is `2026-05-01` (May, June, July = 3 calendar months including current)
- **AND GIVEN** the local date is 2026-01-15, **THEN** the result is `2025-11-01` (year rollover handled)
- **AND GIVEN** the local date is 2026-02-28, **THEN** the result is `2025-12-01`

#### R2: Cap injection at the since/until guard seam
The CLI SHALL default `sinceFlag` to the computed floor in `main()` (adjacent to the existing since/until history-only guard) when ALL of: `display === "history"`, `period !== "monthly"`, `sinceFlag === undefined`, `untilFlag === undefined`, and the `--full` flag is absent. It SHALL also set a `capActive` marker used to drive the heading hint.

- **GIVEN** `tu h` (daily history, no explicit window, no `--full`) on 2026-07-17
- **WHEN** `main()` runs before dispatch
- **THEN** `sinceFlag` is set to `2026-05-01` and `capActive` is true
- **AND GIVEN** `tu wh`, **THEN** the same floor is applied (weekly history is capped)
- **AND** the defaulted `sinceFlag` flows through the existing dispatch plumbing (`filterEntriesByRange`) unchanged — no new filtering logic, cache untouched

#### R3: Monthly history is exempt
The cap SHALL NOT apply to monthly history (`mh` / `m h`).

- **GIVEN** `tu mh` (monthly history)
- **WHEN** `main()` runs
- **THEN** `sinceFlag` remains `undefined` and `capActive` is false (full monthly history shown)

#### R4: Cap applies to history display only, never snapshot
The cap SHALL NOT apply to snapshot displays (bare, no `h`/`history`).

- **GIVEN** `tu` or `tu m` (snapshot)
- **WHEN** `main()` runs
- **THEN** `sinceFlag` remains `undefined` and `capActive` is false

#### R5: Explicit `--since` OR `--until` disables the cap entirely
Any explicit `--since` or `--until` SHALL disable the implicit cap (no intersection with the implicit floor).

- **GIVEN** `tu h --until 2026-03-01` (a past `--until`, no `--since`)
- **WHEN** `main()` runs
- **THEN** the implicit floor is NOT applied (`sinceFlag` stays `undefined`), `capActive` is false, and the user's explicit window is honored — output is not silently emptied
- **AND GIVEN** `tu h --since 2026-01-01`, **THEN** the implicit floor is NOT applied and the explicit floor is used

### CLI: New `--full` Escape-Hatch Flag

#### R6: `--full` boolean global flag parsing
`parseGlobalFlags` SHALL recognize a new long-only boolean flag `--full` (no short alias), expose it as `fullFlag: boolean` on `GlobalFlags`, and strip it from `filteredArgs` in the same boolean-strip pass as the other boolean flags.

- **GIVEN** `parseGlobalFlags(["h", "--full"])`
- **WHEN** parsed
- **THEN** `fullFlag === true` and `filteredArgs === ["h"]`
- **AND GIVEN** `parseGlobalFlags(["cc", "h"])`, **THEN** `fullFlag === false`

#### R7: `--full` disables the cap on daily/weekly history
When `--full` is present, daily/weekly history SHALL show full (uncapped) history with no heading hint.

- **GIVEN** `tu h --full` on 2026-07-17
- **WHEN** `main()` runs
- **THEN** `sinceFlag` stays `undefined`, `capActive` is false, full daily history is shown

#### R8: `--full` on monthly history is a silent no-op
`tu mh --full` SHALL be a vacuous no-op — monthly is never capped, so full history is already shown; no warning is printed (the flag's request is satisfied, not ignored).

- **GIVEN** `tu mh --full`
- **WHEN** `main()` runs
- **THEN** no `--full` warning is written to stderr and full monthly history is shown

#### R9: `--full` on snapshot warns-and-ignores
`--full` on a snapshot display SHALL warn once on stderr and be ignored, mirroring the since/until snapshot guard. The warning is printed in `main()` (not inside dispatch) so watch mode warns once at startup.

- **GIVEN** `tu --full` or `tu m --full` (snapshot)
- **WHEN** `main()` runs
- **THEN** stderr receives `Warning: --full applies to daily/weekly history — ignoring.` exactly once and the flag has no other effect

#### R10: `--full` combined with `--since`/`--until` is silently accepted
`--full` together with an explicit `--since`/`--until` SHALL be silently accepted (both express "no implicit cap"; the explicit window still applies; no warning).

- **GIVEN** `tu h --full --since 2026-01-01`
- **WHEN** `main()` runs
- **THEN** no warning is printed, the cap is off, and the explicit `--since 2026-01-01` window applies

### Display: Heading Hint When Cap Active

#### R11: `last 3 months` heading hint on capped history tables
When the implicit cap is active, the history table heading SHALL indicate it with the text `last 3 months`, appended inside the period parenthetical, in both the ANSI renderers and the Markdown emitter. It SHALL NOT appear when the cap is inactive (`--full`, explicit window, monthly, or snapshot).

- **GIVEN** the cap is active and `renderHistory` renders a single-tool daily history for Claude Code
- **WHEN** the heading is produced
- **THEN** it reads `📊 Claude Code (daily, last 3 months)`
- **AND GIVEN** the cap is active and `renderTotalHistory` renders the cross-tool pivot, **THEN** the heading reads `📊 Combined Cost History (daily, last 3 months)`
- **AND GIVEN** the Markdown emitter renders history with the cap active, **THEN** the `## {title}` heading carries `, last 3 months`
- **AND GIVEN** the cap is inactive, **THEN** headings are unchanged (`(daily)`, `(weekly)`, etc.)

### CLI: Help Text, Completions, Version

#### R12: `FULL_HELP` documents `--full`
`FULL_HELP` SHALL include a Flags line for `--full` describing that it shows full history (default is last 3 months for daily/weekly history).

- **GIVEN** `tu --help`
- **WHEN** `FULL_HELP` is printed
- **THEN** it contains a `--full` line documenting the escape hatch

#### R13: Completions include `--full`
The bash, zsh, and fish completion scripts SHALL include `--full`.

- **GIVEN** each shell's completion script
- **WHEN** inspected
- **THEN** bash `long_flags` contains `--full`, the zsh `_arguments` list and `long_flags` array contain `--full` (with a description), and fish has a `complete -c tu -l full` line

#### R14: Minor version bump
`package.json` version SHALL be bumped `0.7.0` → `0.8.0` (Output Stability rule — this is a breaking default-output change).

- **GIVEN** `package.json`
- **WHEN** inspected
- **THEN** `version` is `0.8.0`

### Non-Goals

- Capping monthly history (`mh`) — it is the compact long-term view (Assumption #2)
- Week-boundary snapping for the implicit cap — weekly aggregation inherits the exact explicit-`--since` semantics, so a partial leading week may appear (Assumption #9)
- Per-format carve-outs — the cap applies uniformly via the single defaulted `sinceFlag` (Assumption #8); the hint is added only where a heading exists (table + Markdown), CSV/JSON have no heading

### Design Decisions

1. **Client-side defaulted `sinceFlag` at the guard seam**: reuse the shipped `--since`/`--until` machinery (`filterEntriesByRange`) by defaulting `sinceFlag` in `main()` — *Why*: one code path, zero new filtering logic, cache untouched, multi-mode remote entries filtered too — *Rejected*: passing `--since` through to ccusage (bypasses the 60s fetch cache and cannot filter multi-mode remote entries).
2. **Heading hint via a `capActive` FormatOptions/EmitOptions field, not by mutating `period`**: thread an optional `capActive: boolean` into `FormatOptions` and `EmitOptions`, consumed only at the four heading-render points — *Why*: `period` drives `currentLabel`/`aggregateForPeriod`/label logic and must stay pure; a boolean touched only where the heading is built keeps behavior uniform across table + Markdown without perturbing data logic — *Rejected*: appending ` last 3 months` to the `period` string handed to renderers (would corrupt every `period`-keyed computation) (resolves Assumption #12).
3. **`--full` is long-only, named `--full` not `--all`**: avoids the cognitive collision with the positional source token `all` (`tu all dh --all`) (Assumption #5).

## Tasks

### Phase 1: Version bump

- [x] T001 [P] Bump `version` `0.7.0` → `0.8.0` in `package.json` <!-- R14 -->

### Phase 2: Core Implementation (cli.ts)

- [x] T002 Add `fullFlag: boolean` to the `GlobalFlags` interface, detect `--full` via `rawArgs.includes("--full")`, add `--full` to the boolean-flag strip list in the `filteredArgs` loop, and include `fullFlag` in the returned object in `parseGlobalFlags` (`src/node/core/cli.ts`) <!-- R6 -->
- [x] T003 Add a `threeMonthFloor(now = new Date()): string` helper in `src/node/core/cli.ts` that returns `YYYY-MM-01` for the local month two months back (handle year underflow) <!-- R1 -->
- [x] T004 In `main()`, destructure `fullFlag` from `parseGlobalFlags`; add a `let capActive = false`. After the existing since/until snapshot warn-and-clear guard, add the cap-injection block: when `display === "history" && period !== "monthly" && sinceFlag === undefined && untilFlag === undefined && !fullFlag`, set `sinceFlag = threeMonthFloor()` and `capActive = true` (`src/node/core/cli.ts`) <!-- R2 R3 R4 R5 R7 -->
- [x] T005 In `main()`, add a `--full` snapshot warn-and-ignore guard: when `fullFlag && display !== "history"`, write `Warning: --full applies to daily/weekly history — ignoring.\n` to stderr once (before dispatch, mirroring the since/until guard). No warning for `mh`/`--full` (history display) or `--full` + explicit window (`src/node/core/cli.ts`) <!-- R8 R9 R10 -->

### Phase 3: Heading hint (formatter.ts)

- [x] T006 Add optional `capActive?: boolean` to `FormatOptions` and to `EmitOptions` in `src/node/tui/formatter.ts` <!-- R11 -->
- [x] T007 In `renderHistory` (line ~104) and `renderTotalHistory` (line ~310), append `, last 3 months` inside the period parenthetical when `opts?.capActive` is true; in `titleForHistory` / `titleForTotalHistory` (or their emit callers) append the same suffix when `opts.capActive` is true so the Markdown `## {title}` heading carries it (`src/node/tui/formatter.ts`) <!-- R11 -->

### Phase 4: Threading capActive from dispatch (cli.ts)

- [x] T008 Thread `capActive` from `main()` into the dispatch functions so it reaches the table renderers and the Markdown emitters: pass a `FormatOptions`/`EmitOptions` carrying `capActive` on the history dispatch paths (`dispatchAllHistory`, `dispatchSingleTool`, and the watch `*Lines` variants) and into `renderTotalHistoryByFormat`/`renderHistoryByFormat` so `printHistory`/`printTotalHistory`/`emitMarkdown` receive it. Snapshot paths and monthly/`--full`/explicit-window paths pass `capActive` false/absent (`src/node/core/cli.ts`) <!-- R11 -->

### Phase 5: Help text and completions

- [x] T009 [P] Add a `--full` Flags line to `FULL_HELP` in `src/node/core/cli.ts` (e.g. `--full               Show full history (default: last 3 months for daily/weekly history)`) <!-- R12 -->
- [x] T010 [P] Add `--full` to bash `long_flags`, the zsh `_arguments` list + `long_flags` array (description `show full history (no 3-month cap)`), and a fish `complete -c tu -l full -d 'show full history (no 3-month cap)'` line in `src/node/core/completions.ts` <!-- R13 -->

### Phase 6: Tests

- [x] T011 [P] Extend `src/node/core/__tests__/cli-parser.test.ts`: `--full` parsing (stripped from `filteredArgs`, `fullFlag` set true/false); `threeMonthFloor` cases (mid-year 2026-07-17 → 2026-05-01, year rollover 2026-01-15 → 2025-11-01, 2026-02-28 → 2025-12-01) if the helper is exported for test, else cover via a directly-testable export <!-- R1 R6 -->
- [x] T012 [P] Add `--full` presence assertions to `src/node/core/__tests__/cli-help.test.ts` (FULL_HELP) and `src/node/core/__tests__/completions.test.ts` (all three shells) <!-- R12 R13 -->
- [x] T013 [P] Add heading-hint tests to `src/node/tui/__tests__/formatter.test.ts`: `renderHistory`/`renderTotalHistory` include `last 3 months` when `capActive: true`, absent when false/omitted; `emitMarkdown` history/total-history heading carries `, last 3 months` when `capActive: true` <!-- R11 -->

## Execution Order

- T003 blocks T004 (cap block calls `threeMonthFloor`)
- T002 blocks T004 (cap block reads `fullFlag`)
- T006 blocks T007 and T008 (field must exist before renderers/dispatch use it)
- T007 blocks T013 (behavior must exist before its test)
- T001, T009, T010, T012 are independent and parallelizable

## Acceptance

### Functional Completeness

- [x] A-001 R1: `threeMonthFloor` returns the first day of the month two months back in local time, with year rollover handled
- [x] A-002 R2: `tu h`/`tu wh` with no explicit window and no `--full` default `sinceFlag` to the floor and set `capActive`, flowing through the existing filter plumbing
- [x] A-003 R3: `tu mh` is not capped (`sinceFlag` undefined, `capActive` false)
- [x] A-004 R4: snapshot displays are never capped
- [x] A-005 R6: `parseGlobalFlags` exposes `fullFlag` and strips `--full` from `filteredArgs`
- [x] A-006 R7: `tu h --full` shows uncapped daily history with no hint
- [x] A-007 R11: the `last 3 months` hint appears in ANSI (`renderHistory`, `renderTotalHistory`) and Markdown headings when the cap is active
- [x] A-008 R12: `FULL_HELP` documents `--full`
- [x] A-009 R13: bash/zsh/fish completions include `--full`
- [x] A-010 R14: `package.json` version is `0.8.0`

### Behavioral Correctness

- [x] A-011 R5: an explicit `--since` or `--until` disables the implicit cap (no intersection), so `tu h --until <past>` is not silently emptied
- [x] A-012 R11: the hint is absent for `--full`, explicit windows, monthly, and snapshot

### Edge Cases & Error Handling

- [x] A-013 R9: `tu --full`/`tu m --full` (snapshot) warns `Warning: --full applies to daily/weekly history — ignoring.` once on stderr
- [x] A-014 R8: `tu mh --full` prints no `--full` warning (silent no-op)
- [x] A-015 R10: `tu h --full --since <date>` prints no warning and honors the explicit window
- [x] A-016 R1: year-boundary floor (2026-01-15 → 2025-11-01) is correct

### Code Quality

- [x] A-017 Pattern consistency: the cap block and `--full` guard mirror the existing since/until warn-and-clear guard style in `main()`; `threeMonthFloor` follows the functional, `node:`-import, no-class conventions
- [x] A-018 No unnecessary duplication: the cap reuses `filterEntriesByRange` (no new filtering path); the hint reuses the existing heading-render points via a single `capActive` field rather than forking renderers
- [x] A-019 Minimum pathways: the cap is a single defaulted `sinceFlag` through existing dispatch — no per-format or per-mode branch is added for filtering

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- `docs/specs/usage.md` prose update (new default window + `--full`) is deferred to hydrate/ship per the intake Impact section

## Deletion Candidates

None — this change adds new functionality without making existing code redundant. The cap rides the existing `--since` machinery (`filterEntriesByRange`) unchanged; no code path, symbol, or config was superseded.

## Assumptions

<!-- These carry forward the intake's graded decisions that were resolved inline during plan generation. Assumption #12 (hint plumbing) is decided here. -->

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | Heading hint plumbed via an optional `capActive: boolean` on `FormatOptions`/`EmitOptions`, consumed only at the four heading-render points — NOT by mutating the `period` string | Intake Assumption #12 deferred this to plan generation. `period` drives `currentLabel`/`aggregateForPeriod`/label logic and must stay pure; a boolean touched only where the heading is built keeps table + Markdown uniform without perturbing data. No user-visible difference beyond the agreed heading text | S:70 R:85 A:80 D:75 |
| 2 | Confident | `threeMonthFloor(now = new Date())` is exported from `cli.ts` (alongside `parseGlobalFlags`/`parseDataArgs`) so the year-rollover cases are unit-testable in `cli-parser.test.ts` | Existing test file imports pure helpers directly from `../cli.js`; exporting the floor helper matches that pattern and is the cleanest way to test the rollover math deterministically without mocking the system clock in `main()` | S:60 R:85 A:80 D:70 |
| 3 | Certain | The `--full` snapshot warning wording is `Warning: --full applies to daily/weekly history — ignoring.` (em-dash, mirrors the since/until guard) | Intake §What-Changes item 2 specifies this exact string; matches the house warn-and-ignore pattern verbatim | S:90 R:85 A:90 D:90 |

3 assumptions (1 certain, 2 confident, 0 tentative).
