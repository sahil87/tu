# Plan: Add Weekly Period

**Change**: 260703-wkly-add-weekly-period
**Intake**: `intake.md`

## Requirements

### Grammar: Weekly Period Tokens

#### R1: `w`/`weekly` period tokens and `wh` combined shorthand
`parseDataArgs` (`src/node/core/cli.ts`) SHALL recognize `w` and `weekly` as the weekly period token (setting the internal `period` value to the string `"weekly"`), and `wh` as the combined weekly-history shorthand (period `"weekly"`, display `"history"`), alongside the existing daily/monthly tokens. The `-w`/`--watch` flag is unaffected because `parseGlobalFlags` strips dash-prefixed flags before `parseDataArgs` sees positionals.

- **GIVEN** the args `["w"]`
- **WHEN** `parseDataArgs` runs
- **THEN** the result is `{ source: "all", period: "weekly", display: "snapshot" }`
- **AND** `["weekly"]` produces the same period; `["wh"]` produces `{ period: "weekly", display: "history" }`; `["cc", "wh"]` produces `{ source: "cc", period: "weekly", display: "history" }`; and `["cc", "w", "h"]` is equivalent to `["cc", "wh"]`

### Aggregation: Weekly Rollup

#### R2: `weekLabel` — Sunday-start week-start ISO label via UTC arithmetic
`fetcher.ts` SHALL provide a `weekLabel(dailyLabel: string): string` helper that maps a daily ISO label (`YYYY-MM-DD`) to the ISO date of that week's Sunday (`getUTCDay() === 0`), computed with UTC date arithmetic on the date-only label so it is immune to local DST transitions. The label satisfies Constitution V (ISO `YYYY-MM-DD`) and aligns with `ccusage weekly --json`'s default `--start-of-week sunday` output.

- **GIVEN** the daily label `"2026-02-16"` (a Monday)
- **WHEN** `weekLabel("2026-02-16")` runs
- **THEN** it returns `"2026-02-15"` (the preceding Sunday)
- **AND** a label that is itself a Sunday (`"2026-02-15"`) returns itself; a year-boundary day (`"2026-12-28"` … `"2027-01-02"`) returns `"2026-12-27"`; DST-transition Sundays (`"2026-03-08"`, `"2026-11-01"`) return themselves unskewed

#### R3: `aggregateWeekly` — client-side weekly rollup from daily entries
`fetcher.ts` SHALL export `aggregateWeekly(dailyEntries: UsageEntry[]): UsageEntry[]` mirroring `aggregateMonthly`: accumulate all numeric fields (`inputTokens`, `outputTokens`, `cacheCreationTokens`, `cacheReadTokens`, `totalTokens`, `totalCost`) into a map keyed by `weekLabel(e.label)`, returning entries sorted ascending by `label` via `localeCompare`. It MUST be a pure function (inputs not mutated), consistent with Constitution V.

- **GIVEN** daily entries spanning multiple weeks
- **WHEN** `aggregateWeekly` runs
- **THEN** entries in the same Sunday-start week are summed under that week's Sunday label, output is sorted ascending, and the input array is not mutated
- **AND** an empty input returns `[]`; a year-boundary week groups its days under the correct prior-year Sunday; DST-week days group without label skew

#### R4: `aggregateForPeriod` — period-to-aggregator mapping
`fetcher.ts` SHALL export `aggregateForPeriod(period: string, entries: UsageEntry[]): UsageEntry[]` returning `aggregateMonthly(entries)` for `"monthly"`, `aggregateWeekly(entries)` for `"weekly"`, and the entries unchanged (identity) for `"daily"` / any other value.

- **GIVEN** a period string and a daily `UsageEntry[]`
- **WHEN** `aggregateForPeriod(period, entries)` runs
- **THEN** `"monthly"` routes to `aggregateMonthly`, `"weekly"` routes to `aggregateWeekly`, and `"daily"` returns the entries unchanged

#### R5: `currentLabel` weekly case — start of current week (local time)
`currentLabel` (`fetcher.ts`) SHALL, for `period === "weekly"`, return the ISO date of the current week's Sunday using local-time date methods (consistent with the existing daily/monthly cases), backing up to Sunday via `setDate(getDate() - getDay())` which normalizes month/year underflow.

- **GIVEN** `now = Mon Feb 16, 2026` (local)
- **WHEN** `currentLabel("weekly", now)` runs
- **THEN** it returns `"2026-02-15"`
- **AND** `now = Thu Jan 1, 2026` returns `"2025-12-28"` (month/year underflow); a `now` that is itself a Sunday returns that Sunday's date

### Wiring: Collapse Monthly Conditionals onto the Mapping

#### R6: Dispatch and fetch call sites route aggregation through `aggregateForPeriod`
The scattered `period === "monthly"` conditionals in `src/node/core/cli.ts` SHALL be collapsed onto `aggregateForPeriod` so weekly works at every dispatch/fetch site with no third scattered branch: `fetchToolMerged` (both the remote-user early return and the merged return), `fetchToolMergedWithMachines` (both branches including the per-machine map aggregation), `dispatchSingleTool` / `dispatchSingleToolLines` (the single-mode `if (period === "monthly") entries = aggregateMonthly(entries)` line becomes `entries = aggregateForPeriod(period, entries)`), `dispatchAllHistory` / `dispatchAllHistoryLines` (single-mode monthly branch generalizes to `if (period !== "daily")` aggregating per tool via `aggregateForPeriod`), and `dispatchAllSnapshot` / `dispatchAllSnapshotLines` (single-mode `period === "monthly"` special branch becomes `period !== "daily"` using `aggregateForPeriod` + `currentLabel(period)`; the daily path stays on `fetchAllTotals`). Snapshot picking (`entries.find((e) => e.label === currentLabel(period))`) is already period-generic and is unchanged beyond `currentLabel` itself.

- **GIVEN** `period === "weekly"` at any of these call sites
- **WHEN** the site fetches daily entries and aggregates
- **THEN** the entries are rolled up via `aggregateForPeriod("weekly", …)` and the weekly snapshot matches `currentLabel("weekly")`
- **AND** `period === "monthly"` and `period === "daily"` behavior is unchanged at every site (monthly still aggregates, daily stays the identity/`fetchAllTotals` path)

### Discoverability: Help, Completions, Spec

#### R7: FULL_HELP advertises the weekly period
`FULL_HELP` (`src/node/core/cli.ts`) SHALL list `w/weekly` in its Periods line and `wh (weekly history)` in its Combined line. Because `tu help-dump` embeds `FULL_HELP` verbatim, the shll.ai contract picks up the new text automatically (additive).

- **GIVEN** `tu --help` output
- **WHEN** a user reads the Periods and Combined lines
- **THEN** `w/weekly` appears among the periods and `wh (weekly history)` among the combined shorthands

#### R8: Shell completions offer weekly tokens (bash/zsh/fish)
`src/node/core/completions.ts` SHALL offer `w`, `weekly`, and `wh` in all three shells: bash `periods="d w m daily weekly monthly"` and `display="h history dh wh mh"`; zsh `periods=(d w m daily weekly monthly)` and `display=(h history dh wh mh)`; fish `complete` lines for `w` ('weekly'), `weekly` ('weekly'), `wh` ('weekly history'), plus extending the non-subcommand catch-all list to `'d w m daily weekly monthly h history dh wh mh'`.

- **GIVEN** a completion script emitted by `tu shell-init <shell>`
- **WHEN** the user tab-completes a period/display token
- **THEN** `w`, `weekly`, and `wh` are among the offered completions in bash, zsh, and fish

#### R9: usage.md documents the weekly grammar and week-label convention
`docs/specs/usage.md` SHALL add `w`/`weekly` to the Periods table ("Weekly granularity (aggregated from daily)"), `wh` to the Display/Combined table, extend the `(daily|monthly)` heading/snapshot enumerations to include `weekly`, and note the week-label convention (Sunday-start, week-start ISO date) in the Data Flow section where aggregation / `currentLabel` filtering is described.

- **GIVEN** the usage spec
- **WHEN** a reader consults the Periods table, the Display table, and the Data Flow section
- **THEN** the weekly period, the `wh` shorthand, and the Sunday-start week-start label convention are all documented

### Non-Goals

- **formatter.ts** — no change; it is period-generic (headings interpolate `(${period})`, the label column header is the literal `"Date"` for all periods, and weekly labels are `YYYY-MM-DD` strings identical in shape to daily). Verified at intake.
- **watch.ts** — no change; it holds no period logic (weekly watch flows through the `dispatch*Lines` variants).
- **sync/** — no change; weekly is never persisted (`writeMetrics` always writes daily entries; aggregation is post-merge and display-only).
- **help-dump.ts** — no change; it embeds `FULL_HELP` verbatim, so the weekly text is additive.
- **No ccusage `weekly` subcommand invocation** — the fetch stays daily-only; weekly is computed client-side to preserve the multi-mode merge pipeline and the daily fetch cache.

### Design Decisions

1. **Client-side weekly aggregation from daily entries** — mirrors `aggregateMonthly`, applied post-merge. *Why*: inherits the fetch cache, multi-mode machine merge (incl. the own-machine max-merge correction), `--user` remote views, `--by-machine`, and watch mode with zero extra plumbing. *Rejected*: passing through to `ccusage weekly` — it would bypass the multi-mode merge pipeline and skip the daily fetch cache (extra ccusage args bypass caching in `fetchHistory`).
2. **Sunday-start, week-start ISO date as the label** — `YYYY-MM-DD` of the week's Sunday. *Why*: satisfies Constitution V with no amendment, and matches `ccusage weekly --json`'s default `--start-of-week sunday` output so `tu weekly` rows compare row-for-row with `ccusage weekly`. *Rejected*: a distinct `YYYY-Www` form — it would require a constitution amendment.
3. **UTC arithmetic in `weekLabel`, local-time in `currentLabel`** — `weekLabel` operates on the timezone-less date-only label string (UTC math is immune to DST), while `currentLabel("weekly")` uses local-time date methods to match the existing daily/monthly cases (which key on the user's local "now"). *Why*: labels are dates without a time component; anchoring the current-week pick to local time matches how daily/monthly already resolve "now".
4. **Thread `aggregateForPeriod(period, entries)` over adding a third scattered branch** — *Why*: every monthly-conditional site is the same two-line pattern; single-mode snapshot/history branches generalize cleanly from `=== "monthly"` to `!== "daily"`. Fewer distinct code paths (code-quality "minimum pathways"). *Rejected*: adding a parallel `period === "weekly"` branch at ~8 sites (duplication, drift risk).

## Tasks

### Phase 1: Core Aggregation (fetcher.ts)

- [x] T001 Add `weekLabel(dailyLabel: string): string` to `src/node/core/fetcher.ts` — UTC arithmetic on the date-only label (`new Date(\`${dailyLabel}T00:00:00Z\`)`, `setUTCDate(getUTCDate() - getUTCDay())`, `toISOString().slice(0, 10)`), with an explanatory comment on the Sunday-start alignment. <!-- R2 -->
- [x] T002 Add exported `aggregateWeekly(dailyEntries: UsageEntry[]): UsageEntry[]` to `src/node/core/fetcher.ts`, mirroring `aggregateMonthly`'s accumulate-into-map shape and `localeCompare` sort, keyed by `weekLabel(e.label)`. <!-- R3 -->
- [x] T003 Add exported `aggregateForPeriod(period: string, entries: UsageEntry[]): UsageEntry[]` to `src/node/core/fetcher.ts` — `"monthly"` → `aggregateMonthly`, `"weekly"` → `aggregateWeekly`, else identity. <!-- R4 -->
- [x] T004 Add the `period === "weekly"` case to `currentLabel` in `src/node/core/fetcher.ts` — local-time back-up to Sunday via `setDate(getDate() - getDay())`, returning the zero-padded `YYYY-MM-DD` of that Sunday. <!-- R5 -->

### Phase 2: Grammar (cli.ts)

- [x] T005 Add the `w`/`weekly` and `wh` token branches to `parseDataArgs` in `src/node/core/cli.ts` (period `"weekly"`; `wh` also sets display `"history"`). <!-- R1 -->

### Phase 3: Wiring — Collapse Monthly Conditionals onto `aggregateForPeriod` (cli.ts)

- [x] T006 Import `aggregateForPeriod` from `./fetcher.js` in `src/node/core/cli.ts` (replacing the direct `aggregateMonthly` import where the collapsed sites no longer call it directly; keep `aggregateMonthly` imported only if a residual direct use remains). <!-- R6 -->
- [x] T007 Collapse the monthly conditionals in `fetchToolMerged` and `fetchToolMergedWithMachines` (`src/node/core/cli.ts`) onto `aggregateForPeriod(period, …)` — both branches of each, including the per-machine map aggregation in `fetchToolMergedWithMachines`. <!-- R6 -->
- [x] T008 Collapse the single-mode monthly branch in `dispatchSingleTool` and `dispatchSingleToolLines` (`src/node/core/cli.ts`): `if (period === "monthly") entries = aggregateMonthly(entries)` → `entries = aggregateForPeriod(period, entries)`. <!-- R6 -->
- [x] T009 Generalize the single-mode monthly branch in `dispatchAllHistory` and `dispatchAllHistoryLines` (`src/node/core/cli.ts`) from `period === "monthly"` to `period !== "daily"`, aggregating each tool's entries via `aggregateForPeriod(period, entries)`. <!-- R6 -->
- [x] T010 Generalize the single-mode snapshot branch in `dispatchAllSnapshot` and `dispatchAllSnapshotLines` (`src/node/core/cli.ts`) from `period === "monthly"` to `period !== "daily"`: fetch daily, `aggregateForPeriod(period, …)`, match `currentLabel(period)`; the daily path stays on `fetchAllTotals`. <!-- R6 -->

### Phase 4: Discoverability (help, completions, spec)

- [x] T011 [P] Update `FULL_HELP` in `src/node/core/cli.ts` — add `w/weekly` to the Periods line and `wh (weekly history)` to the Combined line (optionally a `tu wh` example). <!-- R7 -->
- [x] T012 [P] Update `src/node/core/completions.ts` for all three shells: bash `periods`/`display`, zsh `periods`/`display`, and fish `w`/`weekly`/`wh` `complete` lines plus the non-subcommand catch-all list. <!-- R8 -->
- [x] T013 [P] Update `docs/specs/usage.md` — Periods table (`w`/`weekly`), Display table (`wh`), `(daily|monthly)` heading/snapshot enumerations, and the Data Flow week-label note. <!-- R9 -->

### Phase 5: Tests (co-located `src/node/core/__tests__/`)

- [x] T014 Extend `src/node/core/__tests__/cli-parser.test.ts` — `w`, `weekly`, `wh`, `cc wh`, and `cc w h` ≡ `cc wh` equivalence; confirm invalid combos still throw. <!-- R1 -->
- [x] T015 Extend `src/node/core/__tests__/fetcher.test.ts` — `weekLabel` (Monday→Sunday, Sunday→self, year-boundary, DST Sundays); `aggregateWeekly` (basic grouping, multi-week sort, year-boundary week, DST-transition weeks, empty input, no-mutation); `aggregateForPeriod` identity/monthly/weekly routing; and update the stale `currentLabel("weekly")` assertion (currently expects the daily fallthrough `"2026-02-16"`) to the new Sunday-start spec, adding month/year-underflow coverage. <!-- R2 R3 R4 R5 -->
- [x] T016 Extend `src/node/core/__tests__/completions.test.ts` token-coverage lists to include `w`, `weekly`, and `wh` across bash/zsh/fish. <!-- R8 -->

## Execution Order

- Phase 1 (fetcher exports) must precede Phase 3 (cli.ts wiring imports `aggregateForPeriod`) and Phase 5's fetcher tests.
- Phase 2 (grammar) is independent of Phases 1/3 but precedes the parser tests (T014).
- Within Phase 4, T011/T012/T013 are `[P]` (distinct files).
- Phase 5 tests run after their subject code exists (T014 after T005; T015 after T001–T004; T016 after T012).

## Acceptance

### Functional Completeness

- [x] A-001 R1: `parseDataArgs` recognizes `w`/`weekly` (period `"weekly"`) and `wh` (period `"weekly"`, display `"history"`), with `cc wh` and `cc w h` producing the expected `DataArgs`.
- [x] A-002 R2: `weekLabel` returns the Sunday-start week-start ISO date via UTC arithmetic for Monday inputs, Sunday inputs (self), year-boundary days, and DST Sundays.
- [x] A-003 R3: `aggregateWeekly` sums all fields into Sunday-keyed weeks, sorts ascending, is pure (no input mutation), and handles empty input.
- [x] A-004 R4: `aggregateForPeriod` routes `"monthly"`→`aggregateMonthly`, `"weekly"`→`aggregateWeekly`, `"daily"`/other→identity.
- [x] A-005 R5: `currentLabel("weekly", now)` returns the current week's Sunday in local time, with correct month/year underflow.
- [x] A-006 R6: every collapsed cli.ts site aggregates weekly via `aggregateForPeriod` and picks the snapshot via `currentLabel("weekly")`, while monthly and daily behavior is unchanged.
- [x] A-007 R7: `FULL_HELP` lists `w/weekly` (Periods) and `wh (weekly history)` (Combined).
- [x] A-008 R8: bash/zsh/fish completion scripts each offer `w`, `weekly`, and `wh`.
- [x] A-009 R9: `docs/specs/usage.md` documents the weekly period, the `wh` shorthand, and the Sunday-start week-start label convention.

### Behavioral Correctness

- [x] A-010 R5: the previously stale `currentLabel("weekly")` test assertion is updated from the daily fallthrough (`"2026-02-16"`) to the Sunday-start value (`"2026-02-15"`), conforming the test to the new spec (Constitution Test Integrity).
- [x] A-011 R6: monthly (`m`/`mh`) and daily (`d`/`dh`) output is byte-identical to pre-change behavior at every collapsed dispatch/fetch site (the identity/monthly branches of `aggregateForPeriod` preserve prior results).

### Scenario Coverage

- [x] A-012 R1: parser tests cover `w`, `weekly`, `wh`, `cc wh`, and the `cc w h` ≡ `cc wh` equivalence.
- [x] A-013 R2 R3: fetcher tests cover the year-boundary week (2026-12-28…2027-01-02 → 2026-12-27) and DST-transition weeks (2026-03-08, 2026-11-01) with UTC labels unskewed.
- [x] A-014 R4: fetcher tests cover `aggregateForPeriod` identity/monthly/weekly routing.
- [x] A-015 R8: completion-script coverage tests assert `w`, `weekly`, `wh` in each shell.

### Edge Cases & Error Handling

- [x] A-016 R5: `currentLabel("weekly")` month/year underflow (e.g. Thu Jan 1, 2026 → 2025-12-28) is exercised.
- [x] A-017 R3: `aggregateWeekly([])` returns `[]` and does not mutate inputs.

### Code Quality

- [x] A-018 Pattern consistency: new code follows the naming and structural patterns of surrounding code (`aggregateWeekly` mirrors `aggregateMonthly`; `weekLabel` mirrors the existing label helpers; import/`type`-import/`node:`-prefix conventions preserved).
- [x] A-019 No unnecessary duplication: existing utilities are reused — `aggregateForPeriod` threads through the monthly-conditional sites rather than adding a parallel weekly branch (minimum pathways).
- [x] A-020 Functional style: no classes introduced; `weekLabel`/`aggregateWeekly`/`aggregateForPeriod` are pure functions over `UsageEntry` per Constitution V.
- [x] A-021 Strict TS + imports: strict mode holds; `.js` import extensions, `node:` prefixes, and `type` imports are used as in surrounding code.
- [x] A-022 No magic strings: the period value `"weekly"` matches the existing `"daily"`/`"monthly"` string convention (no new magic numbers introduced).
- [x] A-023 Graceful degradation: no new throw paths on the data pipeline; weekly is display-only and never persisted (Constitution II / sync unaffected). (Reviewer note: `weekLabel` has a latent `RangeError` on a malformed non-ISO label via `toISOString()` on an Invalid Date — unreachable with ccusage@20 ISO labels and repo-persisted entries; recorded as a should-fix robustness finding, not an acceptance violation.)

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Deletion Candidates

None — this change adds new functionality without making existing code redundant. Verified: `aggregateMonthly` retains live call sites (`aggregateForPeriod` at `src/node/core/fetcher.ts:311` plus direct fetcher tests); the now-unneeded direct `aggregateMonthly` import in `cli.ts` was already removed as part of this change; formatter.ts/watch.ts/sync were period-generic before and remain in use unchanged.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Grammar is `w`/`weekly` + `wh`; internal period `"weekly"`; client-side aggregation post-merge, no ccusage weekly pass-through | Intake assumption 1 (Certain) carried verbatim; backlog specifies all of it incl. the rejected alternative | S:90 R:85 A:95 D:95 |
| 2 | Confident | Week label = week-START date, Sunday-start, plain `YYYY-MM-DD` | Intake assumption 2 (Confident); verified `ccusage weekly --json` emits Sunday week-start ISO by default; display-only so cheap to revisit | S:70 R:80 A:85 D:65 |
| 3 | Confident | Thread `aggregateForPeriod` through all monthly-conditional sites; single-mode snapshot/history branches generalize `=== "monthly"` → `!== "daily"` | Intake assumption 3 (Confident); every site is the same two-line pattern, verified cheap | S:75 R:80 A:85 D:75 |
| 4 | Certain | `currentLabel("weekly")` returns start of current week in local time | Intake assumption 4 (Certain); local-time methods match existing daily/monthly cases | S:85 R:90 A:90 D:85 |
| 5 | Certain | `weekLabel` uses UTC arithmetic on the date-only label | Intake assumption 5 (Certain); date-only labels are timezone-less, UTC math immune to DST | S:65 R:90 A:90 D:85 |
| 6 | Certain | No formatter.ts change | Intake assumption 6 (Certain); headings interpolate `(${period})`, label column is literally `"Date"`, monthly already renders non-daily labels through the same path | S:70 R:95 A:95 D:90 |
| 7 | Certain | Weekly snapshot (`tu w`) = entry matching `currentLabel("weekly")`, EMPTY fallback | Intake assumption 7 (Certain); mirrors the monthly snapshot pattern verbatim at every site | S:80 R:90 A:95 D:90 |
| 8 | Certain | The existing `currentLabel("weekly")` test asserting the daily fallthrough (`"2026-02-16"`) is updated to the Sunday-start value (`"2026-02-15"`) | The intake now defines weekly semantics; per Constitution Test Integrity, a stale test is updated to conform to the spec (route (a)) — the implementation follows the spec, not the test | S:90 R:90 A:95 D:90 |

8 assumptions (6 certain, 2 confident, 0 tentative).
