# Plan: Leaderboard display (`lb` / `lbh`)

**Change**: 260828-4xwg-leaderboard-lb-lbh-display
**Intake**: `intake.md`

> The intake's `## What Changes` §1–§10 is the authoritative design; requirements below cite those
> sections rather than restating every detail. Apply MUST read the intake alongside this plan.

## Requirements

### CLI: Grammar and flags

#### R1: `lb` / `lbh` display tokens
`parseDataArgs` MUST accept `lb` (→ `display = "leaderboard"`) and `lbh` (→ `display = "leaderboard-history"`) as display tokens composing with any source and period token. No combined `dlb`/`wlb`/`mlb` shorthands SHALL be added.

- **GIVEN** argv `cc m lb`
- **WHEN** parsed
- **THEN** source is Claude Code, period `monthly`, display `leaderboard`
- **AND** `tu lbh` yields period `daily`, display `leaderboard-history`

#### R2: `--top <n>` flag
`parseGlobalFlags` MUST parse long-only value-taking `--top <n>` into `GlobalFlags.topFlag`; a missing/non-integer/`< 1` value MUST print `Error: --top requires a positive integer` and exit 2. On displays other than `lb`/`lbh`, `main()` MUST warn once `Warning: --top applies to leaderboard display — ignoring.` and clear the flag (same spot as the since/until guard).

- **GIVEN** `tu m lb --top 3` → top 3 rows + dim `… +k others` line (omitted when k = 0); collapsed users still count toward Total and share
- **GIVEN** `tu lbh --top 2` → the 2 user columns with highest window total kept, the rest folded into one `others` column
- **GIVEN** `tu h --top 3` → warning once, table unchanged
- **GIVEN** `tu lb --top 0` → exit 2

#### R3: `main()` guard audit for the new displays
Guards MUST behave per intake §5: `lb`/`lbh` in single mode → stderr `Error: lb requires multi mode — run tu init-metrics <repo-url> to set up a metrics repo`, exit 1, before any fetch; `--since`/`--until` accepted silently on `lb` and `lbh`; `--full` treated as history for `lbh` (warns on `lb`); `capApplies` engages for `leaderboard-history` (daily/weekly) and never for `lb`; `--by-machine` on `lbh` warns and is cleared like the all-tools pivot; `-u all` on `lb`/`lbh` is a no-op, `-u <name>` pins that user. Every `display === "history"` comparison in `cli.ts` MUST be audited and extended where the leaderboard-history is history-shaped.

- **GIVEN** single mode, `tu lb` → exit 1 with the message, no fetch
- **GIVEN** multi mode, `tu lb --since 2026-08-01 --until 2026-08-15` → no warning; window replaces the period window
- **GIVEN** `tu lbh` in multi mode → heading carries the `last 3 months` hint; `tu m lbh` never capped

### Core: Ranking

#### R4: Pure `buildLeaderboard`
A new module `src/node/core/leaderboard.ts` MUST export `LeaderboardRow` and `buildLeaderboard(byUser, prevByUser, metric)` per intake §2: sum per key into `UsageTotals`, drop keys with zero tokens and zero cost, sort descending by `metricValue`, tie-break by key ascending, `share = value / grandTotal` (0 when total 0), `delta = (cur − prev)/prev` when prev nonzero else `undefined`. Inputs MUST NOT be mutated.

- **GIVEN** alice $10, bob $10, chen $0/0 tokens; prev alice $8, bob absent
- **WHEN** built with metric `cost`
- **THEN** rows are [alice(rank 1, share 0.5, delta 0.25), bob(rank 2, share 0.5, delta undefined)]; chen omitted

#### R5: Previous-window derivation
The previous window MUST be the immediately preceding period of the same kind (day/Sunday-anchored week/month), or under `--since/--until` an equal-length window ending the day before `since`. It MUST be produced by a second `filterEntriesByRange` pass over the same fetched daily entries — no second fetch.

- **GIVEN** period `monthly`, current 2026-08
- **WHEN** the prev window is derived
- **THEN** it is 2026-07-01..2026-07-31 and the Δ header reads `Δ vs Jul`
- **AND** with `--since 2026-08-10 --until 2026-08-19` prev is 2026-07-31..2026-08-09, header `Δ vs prev`

#### R6: Data shaping through the existing pipeline
`lb`/`lbh` MUST obtain `Map<user, UsageEntry[]>` from `fetchToolMergedWithMachines(..., ALL_USERS, ...)`'s user-keyed `machineMap`, summed across the source's tool keys; `--by-machine` on `lb` uses a `user/machine`-keyed reader built from `listUsers` + `readRemoteEntriesByMachine`. No changes to `fetcher.ts` or `sync.ts` read semantics; the 60s cache is untouched.

- **GIVEN** source `all`, two users each with cc + codex entries
- **WHEN** shaped for `lb`
- **THEN** each user's row sums both tools

### Display: Rendering and formats

#### R7: `renderLeaderboard` / `printLeaderboard`
`formatter.ts` MUST gain `renderLeaderboard(rows, opts): string[]` + `printLeaderboard` producing the intake §2 layout: heading `Leaderboard ({period}) · {window} · by {cost|tokens}`; columns `#`, `User` (pinned user gets ` ◂`, counted in width), `Cost`, bar (solid green, existing bar primitives/budget), `Tokens`, `Share`, `Δ vs {label}` (`new` when delta undefined); data-sized numeric columns via `metricColumnWidth`; dim exact-zero cells via `metricCell`; `boldWhite` Total row when ≥ 2 rows; dim footer `synced {rel} ago · tu sync to refresh` / `never synced · tu sync to refresh` from `.last-sync`; `--no-color` output byte-equal to the ANSI-stripped colored output. Both Cost and Tokens columns render under every metric.

- **GIVEN** the four-row fixture from the intake mockup, metric cost
- **WHEN** rendered at 100 cols
- **THEN** rank/name/cost/bar/tokens/share/delta columns align and the Total is `$1,082.65`

#### R8: `lbh` reuses `renderTotalHistory`
`lbh` MUST render via `renderTotalHistory` with user columns, controlled by `FormatOptions` fields (not a second renderer): title `📊 Leaderboard History ({period})` / `Leaderboard Token History`, columns ordered by descending window total, per-row leader cell `boldWhite`, negligible-column omission disabled for user columns. With every new option at its default, `tu h` output MUST be byte-identical to today.

- **GIVEN** `tu m lbh` with users alice/bob
- **WHEN** rendered
- **THEN** one row per month, columns alice then bob if alice's total is larger, each row's max cell bold
- **AND** existing formatter tests for `tu h` pass unchanged

#### R9: JSON / CSV / Markdown for the leaderboard
`emitJson`/`emitCsv`/`emitMarkdown` MUST support the leaderboard per intake §7: JSON rows `{rank,user,cost,totalTokens,share,delta}` (+`machine`), `delta: null` for new, `share` a fraction; CSV kind `"leaderboard"` header `rank,user,cost,total_tokens,share,delta` (+`machine` after `user`), raw numbers, `Total` row when > 1 row; Markdown kind `"leaderboard"` with `## Leaderboard ({period})`, right-aligned numerics, `**Total**`. `lbh` JSON keeps the pivot's map-of-columns shape. `--top` applies to all three.

- **GIVEN** `tu m lb --json --top 1` with two users
- **WHEN** emitted
- **THEN** one row object; `share` reflects the two-user denominator

#### R10: Dispatch and watch mode
New `dispatchLeaderboard`, `dispatchLeaderboardHistory` and their `*Lines` watch variants MUST fetch once and switch on `outputFormat`; `main()` MUST route on the new displays before the `source === "all"` split in both the one-shot branch and the watch closure; watch variants set `_lastRenderCost`, `_lastRenderTotalTokens`, `_lastRenderCostMap` (keyed by plain user name, valued in the display metric).

- **GIVEN** `tu m lb -w`
- **WHEN** polled twice
- **THEN** the board re-renders through the compositor with per-user cost deltas available

### Docs: Surfaces

#### R11: Help, completions, skill bundle, README, specs
`FULL_HELP`/`SHORT_USAGE`, `completions.ts` (bash/zsh/fish), `docs/site/skill.md` (drift-guarded vs embedded `SKILL_MD`), `README.md`, `docs/specs/usage.md` (Display table, `--top`, `--since/--until` applicability, Exit Codes rows) and `docs/specs/layouts.md` (two new layout sections) MUST document `lb`, `lbh`, `--top`. Check each surface against the governing `shll standards` entry before editing. `package.json` MUST NOT be bumped (release-time minor bump; PR body notes "requires minor release").

- **GIVEN** `tu --help`
- **WHEN** printed
- **THEN** the Display line lists `lb`/`lbh`, an example shows `tu m lb`, and `--top <n>` appears in flags
- **AND** `just build` passes the skill.md drift guard

### Non-Goals
- Per-active-day normalisation of the ranking — rejected by the user.
- `--by-machine` on `lbh` — warn-and-ignore, same as the tool pivot.
- Any change to fetch/sync/cache semantics.

### Design Decisions

#### Display token, not a `--rank` flag
**Decision**: `lb`/`lbh` are display tokens in the positional grammar.
**Why**: Discoverable and parallel to `h`/`mh`; `-u all --by-machine` is already an opaque incantation.
**Rejected**: `--rank` flag layered on `-u all --by-machine`.
*Introduced by*: 260828-4xwg-leaderboard-lb-lbh-display

#### `lbh` is the tool pivot with users as columns
**Decision**: Reuse `renderTotalHistory` via `FormatOptions` hooks (title, ordering, leader highlight, omission off).
**Why**: Minimum pathways — data shape `Map<column, UsageEntry[]>` already matches; separators, cap, bars, footer inherited.
**Rejected**: A dedicated leaderboard-history renderer (duplicate of ~200 lines with divergence risk).
*Introduced by*: 260828-4xwg-leaderboard-lb-lbh-display

#### Previous window from a second filter pass
**Decision**: Derive Δ by slicing the already-fetched daily entries twice.
**Why**: Fetch is daily-only and cached; a second fetch would bypass the cache and add a pathway.
**Rejected**: Separate fetch for the previous period.
*Introduced by*: 260828-4xwg-leaderboard-lb-lbh-display

## Tasks

### Phase 1: Setup

- [x] T001 Create `src/node/core/leaderboard.ts` with `LeaderboardRow`, `buildLeaderboard`, and pure helpers `previousWindow(period, since, until)` → `{start,end,label}` and `sumByKey(map)`; add `src/node/core/__tests__/leaderboard.test.ts` covering ranking, tie-break, share, delta/new, zero-row omission, immutability, metric switch, previous-window for day/week/month/explicit range <!-- R4, R5 -->

### Phase 2: Core Implementation

- [x] T002 In `src/node/core/cli.ts`: add `lb`/`lbh` to `parseDataArgs`; add `--top` parsing/validation to `parseGlobalFlags` + `GlobalFlags.topFlag`; extend `src/node/core/__tests__/cli-parser.test.ts` and add `src/node/core/__tests__/cli-top-flag.test.ts` <!-- R1, R2 -->
- [x] T003 In `src/node/tui/formatter.ts`: add `renderLeaderboard`/`printLeaderboard` (heading, columns, bar, pinned marker, dim zeros, Total, `… +k others`, staleness footer) and `src/node/tui/__tests__/formatter-leaderboard.test.ts` incl. `--no-color` byte-equality <!-- R7 --> <!-- rework: bar cell must be padded to barWidth on every data row so Tokens/Share/Δ align; collapsed-row label must fit nameWidth; reserve watch indicator width -->
- [x] T004 In `src/node/tui/formatter.ts`: add `FormatOptions` hooks consumed by `renderTotalHistory` — `historyTitle`, `columnOrder: "registry" | "total-desc"`, `highlightRowLeader`, `omitNegligibleColumns` — all defaulting to today's behaviour; add tests for the hooks and confirm existing history tests pass unchanged <!-- R8 -->
- [x] T005 [P] In `src/node/tui/formatter.ts`: add `"leaderboard"` kinds to `emitCsv`/`emitMarkdown` and a JSON row builder for `lb`; tests for JSON/CSV/MD shape incl. `--top`, `--by-machine` `machine` field, `delta: null`/empty <!-- R9 --> <!-- rework: CSV/MD Total under --top must sum ALL rows (collapsed included), and lbh --md heading must read Leaderboard History, not Combined Cost History -->
- [x] T006 In `src/node/core/cli.ts`: add a `user/machine`-keyed all-users reader (mirroring `readAllUsersByUser`) for `--by-machine` on `lb`; data-shaping helper that sums the user-keyed `machineMap` across the source's tool keys into `Map<user, UsageEntry[]>` for current and previous windows <!-- R6, R5 -->

### Phase 3: Integration & Edge Cases

- [x] T007 In `src/node/core/cli.ts`: implement `dispatchLeaderboard`, `dispatchLeaderboardHistory`, `dispatchLeaderboardLines`, `dispatchLeaderboardHistoryLines` (single fetch, `switch (outputFormat)`, watch bookkeeping); route on the new displays in both one-shot and watch paths <!-- R10 --> <!-- rework: lbh historyTitle must go through periodLabel(period, capActive) so the 'last 3 months' hint renders; one-shot lb table path must call printLeaderboard (currently dead code) -->
- [x] T008 In `src/node/core/cli.ts` `main()`: single-mode exit-1 guard for `lb`/`lbh`; `--since/--until`, `--full`, `--top`, `--by-machine`, `-u` guard changes; extend `capApplies` to `leaderboard-history`; audit every `display === "history"` comparison; extend `src/node/core/__tests__/cli-exit-codes.test.ts` (single-mode `lb` → 1, `--top 0` → 2) and guard-warning tests <!-- R3, R2 -->
- [x] T009 Run `npx tsx --test 'src/**/__tests__/*.test.ts'` and `just build`; fix any regressions; verify `tu h` output byte-identical via existing tests <!-- R8, R10 -->

### Phase 4: Polish

- [x] T010 Run `shll standards` and check the governing entries; update `FULL_HELP`/`SHORT_USAGE` in `src/node/core/cli.ts`, `src/node/core/completions.ts` (3 shells), `docs/site/skill.md`, `README.md` <!-- R11 -->
- [x] T011 [P] Update `docs/specs/usage.md` (Display table, `--top`, flag applicability, Exit Codes rows, leaderboard Output Formats) and `docs/specs/layouts.md` (Leaderboard + Leaderboard History layout sections with mockups) <!-- R11 -->

## Execution Order

- T001 blocks T006, T007
- T002 blocks T007, T008
- T003, T004, T005 block T007
- T007 blocks T008, T009
- T010, T011 after T009

## Acceptance

### Functional Completeness

- [x] A-001 R1: `lb` and `lbh` parse in every positional slot with any source/period token; no `dlb/wlb/mlb`
- [x] A-002 R2: `--top <n>` parses, validates (exit 2), collapses rows on `lb`, folds columns on `lbh`, warns-and-clears elsewhere
- [x] A-003 R4: `buildLeaderboard` exists in `src/node/core/leaderboard.ts` with the specified row shape and rules
- [x] A-004 R7: `renderLeaderboard`/`printLeaderboard` produce the mockup layout with all seven columns, Total and staleness footer — NOT MET (re-review): all seven columns, Total and footer render, but the bar area is not padded to `barWidth`, so Tokens/Share/Δ columns misalign on every row whose bar is shorter than the max (see review must-fix: `src/node/tui/formatter.ts` `renderLeaderboard` data-row loop) — FIXED (cycle 2): bar area padded to barWidth on every data row; alignment asserted by test
- [x] A-005 R8: `lbh` renders through `renderTotalHistory` with user columns, rank-ordered, per-row leader bold, no omission
- [x] A-006 R9: JSON/CSV/MD leaderboard outputs match the specified shapes
- [x] A-007 R10: four dispatch functions exist, each with a single fetch and one `switch (outputFormat)`; watch routing works
- [x] A-008 R11: help, completions, skill.md, README, usage.md, layouts.md all document `lb`/`lbh`/`--top`

### Behavioral Correctness

- [x] A-009 R3: single-mode `lb`/`lbh` exits 1 with the exact message before any fetch
- [x] A-010 R3: `--since/--until` on `lb` produce no warning and replace the period window; `capApplies` covers `leaderboard-history` and not `lb`
- [x] A-011 R5: prev window is the preceding same-kind period (or equal-length preceding range) and is computed from the same fetched entries — no second fetch call
- [x] A-012 R8: existing `tu h` / history formatter tests pass unchanged (byte-identical default path)
- [x] A-013 R11: `package.json` version unchanged

### Scenario Coverage

- [x] A-014 R4: test covers ranking, tie-break by name, share sum ≈ 1, delta and `new`, zero-row omission, immutability, tokens metric
- [x] A-015 R7: `--no-color` output equals ANSI-stripped colored output byte-for-byte
- [x] A-016 R9: `--top 1 --json` yields one row whose share uses the full denominator
- [x] A-017 R2: `--top 0`, `--top abc`, bare `--top` all exit 2 with the specified message

### Edge Cases & Error Handling

- [x] A-018 R6: empty user list renders heading + footer with no rows and no crash
- [x] A-019 R7: pinned-user glyph is counted in User column width; long user names don't misalign columns
- [x] A-020 R7: `.last-sync` absent → `never synced · tu sync to refresh`
- [x] A-021 R5: previous value exactly 0 → `new` (delta undefined / null / empty per format)

### Code Quality

- [x] A-022 Pattern consistency: new code follows render/print split, `node:` imports, `type` imports, functions over classes
- [x] A-023 No unnecessary duplication: bar/width/cost helpers, `filterEntriesByRange`, `aggregateForPeriod`, `renderTotalHistory` reused; no second fetch path
- [x] A-024 Minimum pathways: one `switch (outputFormat)` per dispatcher; no fetch duplicated across format branches
- [x] A-025 No god functions: `buildLeaderboard` and `renderLeaderboard` stay decomposed (< 50 lines each or justified)
- [x] A-026 Errors warn on stderr / exit with correct codes — nothing swallowed
- [x] A-027 Tests co-located in `__tests__/`, run via `npx tsx --test`

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | `FormatOptions` hook names (`historyTitle`, `columnOrder`, `highlightRowLeader`, `omitNegligibleColumns`) | Names are internal; apply may rename if existing fields conflict | S:60 R:90 A:85 D:80 |
| 2 | Confident | `previousWindow` and `sumByKey` live in `leaderboard.ts` rather than `fetcher.ts` | Keeps the leaderboard module self-contained and pure; fetcher is untouched per intake | S:65 R:85 A:85 D:80 |
| 3 | Confident | Explicit-range Δ header reads `Δ vs prev`; period header uses short month (`Jul`), ISO date for day, week-start date for week | Intake mocks `Δ vs Jul`; others follow existing label conventions | S:60 R:90 A:80 D:75 |

3 assumptions (0 certain, 3 confident, 0 tentative).

## Deletion Candidates

- None — re-verified on rework cycle 2: this change adds new functionality without making existing code redundant. All touched render/parse paths keep their existing consumers; the new `FormatOptions` hooks default to today's behavior, so no legacy branch is orphaned.
