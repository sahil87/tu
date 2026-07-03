# Plan: Since/Until Date Filters + -j JSON Alias

**Change**: 260703-sntl-since-until-filters-json-alias
**Intake**: `intake.md`

## Requirements

### CLI: Flag Parsing

#### R1: `--since`/`-s` and `--until` date flags
`parseGlobalFlags` in `src/node/core/cli.ts` MUST parse `--since <date>` (short alias `-s <date>`) and `--until <date>` (long-only — `-u` is `--user`) as space-separated value-taking flags, in the same loop as `--interval`/`--user`. Accepted date shapes are `YYYY-MM-DD` and `YYYYMMDD`, normalized to ISO `YYYY-MM-DD` and exposed on `GlobalFlags` as `sinceFlag: string | undefined` and `untilFlag: string | undefined`.

- **GIVEN** args `["cc", "h", "--since", "2026-06-01"]`
- **WHEN** `parseGlobalFlags` runs
- **THEN** `sinceFlag === "2026-06-01"`, `untilFlag === undefined`, and `filteredArgs === ["cc", "h"]`
- **AND** args `["-s", "20260601"]` produce `sinceFlag === "2026-06-01"` (normalized, dashes inserted)
- **AND** args `["--until", "2026-06-30"]` produce `untilFlag === "2026-06-30"` while `-u` continues to parse as `--user`

#### R2: `-j` short alias for `--json`
`parseGlobalFlags` MUST treat `-j` as an alias of `--json`: it participates in `--json` detection (`rawArgs.includes("--json") || rawArgs.includes("-j")`), is added to the filtered-args skip list, and resolves to `outputFormat: "json"` and `jsonFlag: true`. It joins the existing output-format incompatibility matrix through `jsonFlag`; error messages keep the canonical `--json` wording, unchanged.

- **GIVEN** args `["cc", "-j"]`
- **WHEN** `parseGlobalFlags` runs
- **THEN** `outputFormat === "json"`, `jsonFlag === true`, and `filteredArgs === ["cc"]`
- **AND** args `["cc", "-j", "--csv"]` exit 1 with `Error: --json and --csv are incompatible`
- **AND** args `["cc", "-j", "--watch"]` exit 1 with `Error: --watch and --json are incompatible`

#### R3: Date-flag validation errors
Malformed or missing values for `--since`/`--until`, and an inverted window, MUST print an error to stderr and exit 1, mirroring the `--interval requires a numeric value` pattern. Date validation is shape-only (regex) — no calendar validity check.

- **GIVEN** args `["--since"]` (no value) or `["--since", "june"]` (malformed)
- **WHEN** `parseGlobalFlags` runs
- **THEN** it prints `Error: --since requires a date (YYYY-MM-DD or YYYYMMDD)` and exits 1 (same for `--until` with its own name)
- **AND** args `["--since", "2026-06-30", "--until", "2026-06-01"]` print `Error: --since must be on or before --until` and exit 1
- **AND** an impossible-but-well-shaped date like `2026-13-01` parses successfully (shape-only) and simply yields an empty window downstream

### Fetcher: Client-Side Date Filter

#### R4: `filterEntriesByRange` pure function
`src/node/core/fetcher.ts` MUST export a pure function `filterEntriesByRange(entries: UsageEntry[], since?: string, until?: string): UsageEntry[]` that returns entries whose ISO `label` satisfies `(!since || label >= since) && (!until || label <= until)` — inclusive on both ends, lexicographic compare, no input mutation, alongside `aggregateMonthly`/`mergeEntries` (Constitution V).

- **GIVEN** entries labeled `2026-06-01`, `2026-06-15`, `2026-07-01`
- **WHEN** `filterEntriesByRange(entries, "2026-06-01", "2026-06-30")` runs
- **THEN** it returns exactly the `2026-06-01` and `2026-06-15` entries (both bounds inclusive)
- **AND** `filterEntriesByRange(entries, "2026-06-15", undefined)` returns `2026-06-15` and `2026-07-01` (since-only, open-ended)
- **AND** `filterEntriesByRange(entries, undefined, "2026-06-15")` returns `2026-06-01` and `2026-06-15` (until-only)
- **AND** `filterEntriesByRange(entries, undefined, undefined)` returns all entries unchanged
- **AND** the input array and its entries are not mutated

### CLI: Filter Dispatch Wiring

#### R5: Filter applied to history across all paths, before monthly aggregation
The filter MUST apply to daily entries **after merge, before `aggregateMonthly`**, so monthly rollups reflect the window (a partial month sums only in-window days). Coverage MUST include: multi-mode (`fetchToolMerged` / `fetchToolMergedWithMachines`, including the `-u <other-user>` repo-only path), single mode (`dispatchAllHistory` / `dispatchSingleTool`), the `--by-machine` per-machine `machineMap` (filtered consistently with the flattened entries), and the watch-mode `*Lines` variants (filtered each poll via the shared threaded path). All output formats (`table`/`--json`/`--csv`/`--md`) see filtered data. The 60s fetch cache is untouched — `fetchHistory` still fetches/caches the full daily set (vanilla `extraArgs === []`); filtering happens after.

- **GIVEN** merged local+remote daily entries spanning `2026-05-*` through `2026-07-*` and `--since 2026-06-01 --until 2026-06-30`
- **WHEN** a history command dispatches (multi or single mode, any output format)
- **THEN** only June entries are rendered
- **AND** a monthly (`mh`) rollup over a partially-windowed June sums only the in-window June days
- **AND** with `--by-machine`, no per-machine column shows an out-of-window date
- **AND** `fetchHistory` is still invoked with `extraArgs === []` (cache path preserved)

#### R6: Snapshot display — warn and ignore
When `--since`/`--until` is present and the display is snapshot (bare, no `h`/`history`), tu MUST warn once on stderr and ignore the flags, clearing them in `main()` before dispatch (mirroring the existing `--by-machine` all-tools-history and `-u`-single-mode warn-and-clear guards). In watch snapshot mode the warning is printed once at startup, not per poll.

- **GIVEN** args resolving to a snapshot display with `--since`/`--until` present
- **WHEN** `main()` runs
- **THEN** it writes `Warning: --since/--until apply to history display — ignoring.` to stderr and dispatches with the date flags cleared
- **AND** in watch snapshot mode the warning appears once at startup, not on every poll

### CLI: Help Text & Completions

#### R7: `FULL_HELP` Flags block
The `FULL_HELP` Flags block in `src/node/core/cli.ts` MUST document the new/updated flags: the `--json` line gains the `-j` alias, and new `--since / -s <date>` and `--until <date>` lines are added, matching the existing block's phrasing/order style. `help-dump`/shll.ai picks this up automatically (raw `FULL_HELP` passthrough — additive, no drift concern).

- **GIVEN** `tu --help` output (or `runHelpDump` JSON)
- **WHEN** rendered
- **THEN** the Flags block shows `--json / -j`, a `--since / -s <date>` line, and a `--until <date>` line

#### R8: Shell completions in all three shells
`src/node/core/completions.ts` MUST add `--since`/`--until` to `long_flags`, `-s`/`-j` to `short_flags`, and value-taking wiring following the `--interval`/`--user` precedent: bash adds `--since|-s|--until` to the no-completion `prev` case; zsh adds `--since`/`-s`/`--until` value specs plus `-j`; fish adds `-l since -r`, `-l until -r`, `-s s -r`, `-s j`.

- **GIVEN** each of `BASH_COMPLETION`, `ZSH_COMPLETION`, `FISH_COMPLETION`
- **WHEN** inspected
- **THEN** each contains the literal tokens `--since`, `--until`, `-s`, `-j`
- **AND** `--since`/`-s`/`--until` are wired as value-taking (no positional completion after them)

### Design Decisions

1. **Client-side filter over ccusage pass-through**: filter merged `UsageEntry[]` in `tu`, not by forwarding `--since`/`--until` to ccusage — *Why*: extra ccusage args bypass the 60s fetch cache in `fetchHistory` (`extraArgs.length === 0` gate), and pass-through cannot filter multi-mode remote entries (read from the metrics repo, never through ccusage); labels are already normalized ISO strings so lexicographic compare is a correct total order — *Rejected*: ccusage pass-through (cache bypass + multi-mode blind spot).
2. **Filter before `aggregateMonthly`**: apply to daily entries then aggregate — *Why*: monthly rollups must reflect the window; a partial month sums only in-window days — *Rejected*: filtering monthly labels after aggregation (would include full-month sums for boundary months).
3. **Snapshot warn-and-ignore** (user-chosen at intake): snapshot display warns and drops the flags — *Why*: applying the window to snapshots renders all-zero EMPTY rows whenever today falls outside it (strict but confusing); precedent is `-u` in single mode — *Rejected*: apply-the-filter-to-snapshot.
4. **`--until` long-only**: no `-u` alias — *Why*: `-u` is already `--user`; adding it would break existing users (short-flag audit: `-f -w -i -u -v -V -h` taken; `-s`/`-j` free).

### Non-Goals

- No calendar validity checking of dates (shape-only regex; impossible dates degrade to an empty window).
- No `--since=<date>` equals-form (no `=` form exists anywhere in the parser today).
- No version bump or changelog in this change (additive flags; the minor bump happens at release, normal `feat` flow).

## Tasks

### Phase 1: Core Parsing & Filter

- [x] T001 [P] Add `filterEntriesByRange(entries, since?, until?)` pure exported function to `src/node/core/fetcher.ts`, alongside `mergeEntries`/`aggregateMonthly`, with a comment noting inclusive bounds + lexicographic ISO total order <!-- R4 -->
- [x] T002 Extend `GlobalFlags` in `src/node/core/cli.ts` with `sinceFlag: string | undefined` and `untilFlag: string | undefined` <!-- R1 -->
- [x] T003 In `parseGlobalFlags` (`src/node/core/cli.ts`): add `-j` to `--json` detection and to the filtered-args skip list; parse `--since`/`-s` and `--until` as value-taking flags in the `--interval`/`--user` loop, normalizing `YYYY-MM-DD`/`YYYYMMDD` to ISO; add missing/malformed-value errors and the `since > until` inverted-window error (stderr + exit 1); return `sinceFlag`/`untilFlag` <!-- R1 --> <!-- R2 --> <!-- R3 -->

### Phase 2: Dispatch Wiring

- [x] T004 Thread `since`/`until` into `fetchToolMerged` and `fetchToolMergedWithMachines` (`src/node/core/cli.ts`): filter daily entries after merge, before the `period === "monthly"` aggregation, covering the `-u <other-user>` repo-only branch and the `--by-machine` `machineMap` (filter each machine's entries consistently with the flattened entries) <!-- R5 -->
- [x] T005 Thread `since`/`until` into the single-mode and all-tools history dispatch paths — `dispatchAllHistory`, `dispatchSingleTool`, and the `*Lines` watch variants (`dispatchAllHistoryLines`, `dispatchAllSnapshotLines`, `dispatchSingleToolLines`) — applying `filterEntriesByRange` before `aggregateMonthly` in each `src/node/core/cli.ts` path <!-- R5 -->
- [x] T006 In `main()` (`src/node/core/cli.ts`): pass `sinceFlag`/`untilFlag` through to the dispatch/action calls; add the snapshot warn-and-clear guard (`Warning: --since/--until apply to history display — ignoring.`) before dispatch, alongside the existing `--by-machine`/`-u` guards, so watch snapshot warns once at startup <!-- R5 --> <!-- R6 -->

### Phase 3: Help Text & Completions

- [x] T007 [P] Update the `FULL_HELP` Flags block in `src/node/core/cli.ts`: add `-j` to the `--json` line, add `--since / -s <date>` and `--until <date>` lines in the existing style <!-- R7 -->
- [x] T008 [P] Update `src/node/core/completions.ts` (bash/zsh/fish): add `--since`/`--until` to long flags, `-s`/`-j` to short flags, and value-taking wiring per the `--interval`/`--user` precedent <!-- R8 -->

### Phase 4: Tests

- [x] T009 [P] Add `filterEntriesByRange` tests to `src/node/core/__tests__/fetcher.test.ts`: inclusive both-ends, since-only, until-only, no-bounds passthrough, empty input, no-mutation <!-- R4 -->
- [x] T010 [P] Add flag-parsing tests (`src/node/core/__tests__/cli-parser.test.ts` or a new `cli-date-filter.test.ts`): `-j` → `outputFormat: json` + filtered args; `--since`/`-s`/`--until` parse + normalize both date shapes; malformed/missing-value errors; `since > until` error; `-j` incompatibility matrix (`-j` + `--csv`, `-j` + `--watch`) still rejected with canonical `--json` wording <!-- R1 --> <!-- R2 --> <!-- R3 -->
- [x] T011 [P] Add multi-mode merge-path filter coverage to `src/node/core/__tests__/fetcher.test.ts`: windowed history over merged local+remote entries (`filterEntriesByRange` after `mergeEntries`), and monthly rollup of a partially-windowed month (`filterEntriesByRange` then `aggregateMonthly`) <!-- R5 -->
- [x] T012 [P] Add completion-token coverage to `src/node/core/__tests__/completions.test.ts`: `--since`, `--until`, `-s`, `-j` present in all three scripts <!-- R8 -->

## Execution Order

- T002 blocks T003 (T003 returns the fields added in T002)
- T003 blocks T004, T005, T006 (dispatch threads the parsed flags)
- T001 blocks T004, T005, T009, T011 (filter function must exist)
- T004, T005, T006 are sequential within Phase 2 (same file, dependent wiring)
- Phase 3 (T007, T008) and Phase 4 tests are otherwise independent

## Acceptance

### Functional Completeness

- [x] A-001 R1: `--since`/`-s` and `--until` parse space-separated values, normalize `YYYY-MM-DD`/`YYYYMMDD` to ISO, expose `sinceFlag`/`untilFlag`, and `--until` stays long-only (`-u` still `--user`)
- [x] A-002 R2: `-j` sets `outputFormat: "json"` and `jsonFlag: true`, is filtered from positional args, and joins the format-incompatibility matrix via `jsonFlag`
- [x] A-003 R4: `filterEntriesByRange` is exported from `fetcher.ts`, inclusive on both ends, lexicographic, and pure (no mutation)
- [x] A-004 R5: the filter is applied after merge / before `aggregateMonthly` across multi-mode, single-mode, `-u` repo-only, `--by-machine` machineMap, and watch `*Lines` paths, for all output formats
- [x] A-005 R6: snapshot display warns once on stderr and ignores `--since`/`--until` (watch snapshot warns once at startup)
- [x] A-006 R7: the `FULL_HELP` Flags block documents `--json / -j`, `--since / -s <date>`, and `--until <date>`
- [x] A-007 R8: all three completion scripts contain `--since`, `--until`, `-s`, `-j` with value-taking wiring for `--since`/`-s`/`--until`

### Behavioral Correctness

- [x] A-008 R3: malformed/missing `--since`/`--until` values and an inverted (`since > until`) window each print the specified error to stderr and exit 1
- [x] A-009 R5: a monthly rollup over a partially-windowed month sums only the in-window days; the 60s cache path is preserved (`fetchHistory` called with `extraArgs === []`)

### Scenario Coverage

- [x] A-010 R4: `filterEntriesByRange` tests cover inclusive both-ends, since-only, until-only, no-bounds, empty input, no-mutation
- [x] A-011 R5: a test exercises the multi-mode merge path (windowed merged local+remote) and the partial-month monthly rollup
- [x] A-012 R1 R2 R3: flag-parsing tests cover `-j`, both date shapes, validation errors, and the preserved incompatibility matrix

### Edge Cases & Error Handling

- [x] A-013 R3: a well-shaped impossible date (`2026-13-01`) parses (shape-only) and yields an empty window rather than an error
- [x] A-014 R1: existing `-u` (`--user`) parsing and the existing incompatibility matrix are unchanged by the additions

### Code Quality

- [x] A-015 Pattern consistency: new code follows surrounding naming/structure (value-flag loop idiom, `console.error` + `process.exit(1)`, `type` imports, `node:` imports, functional style)
- [x] A-016 No unnecessary duplication: `filterEntriesByRange` is the single filter reused across all dispatch paths (minimum pathways); no per-path re-implementation
- [x] A-017 Readability: no god functions or magic strings/numbers introduced; regex and error strings are clear
- [x] A-018 Graceful degradation: no new crash surface — invalid dates degrade to an empty window, not an exception (Constitution II)

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Deletion Candidates

None — this change adds new functionality without making existing code redundant

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | `--since`/`-s`/`--until` accept `YYYY-MM-DD` or `YYYYMMDD`, normalized to ISO; `--until` long-only; `-j` aliases `--json` | Stated verbatim in intake; short-flag audit confirms `-u` taken, `-s`/`-j` free | S:95 R:85 A:95 D:95 |
| 2 | Certain | Client-side `filterEntriesByRange` on merged entries, filter-before-`aggregateMonthly`, all history paths incl. `-u` and `--by-machine` machineMap | Stated in intake with cache + multi-mode rationale; Constitution V pure-function pattern | S:90 R:75 A:90 D:90 |
| 3 | Certain | Snapshot display warns-and-ignores; validation errors (missing/malformed value, `since > until`) on stderr + exit 1 | User-chosen at intake (warn-and-ignore); error idiom mirrors `--interval`; precedent `-u`/`--by-machine` guards in `main()` | S:95 R:80 A:95 D:95 |
| 4 | Confident | Window bounds inclusive on both ends; shape-only date validation (no calendar check); space-separated values only (no `=` form) | Not restated per-clause but matches ccusage semantics + existing `--interval`/`--user` parser idiom; impossible dates degrade to empty window | S:60 R:85 A:85 D:80 |
| 5 | Confident | New flag-parsing tests co-located in `cli-parser.test.ts` (extending the existing `parseGlobalFlags` blocks) rather than a new `cli-date-filter.test.ts` | Intake allows either; extending the existing file keeps the parser test surface in one place and matches how `--csv`/`--md` tests were added | S:70 R:90 A:80 D:75 |

5 assumptions (3 certain, 2 confident, 0 tentative).
