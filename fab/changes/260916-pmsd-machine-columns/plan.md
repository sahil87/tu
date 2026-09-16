# Plan: Machine Columns (Go port row B4)

**Change**: 260916-pmsd-machine-columns
**Intake**: `intake.md`

## Requirements

> Every surface below is Goal-frozen: the Go binary reproduces the shipped TypeScript bytes. Oracle sources: `src/node/core/cli.ts` (`main()` guards at lines ~1943–1950, `fetchToolMergedWithMachines`, `dispatchAllSnapshot`/`dispatchSingleTool` by-machine branches, `buildSnapshotMachineCosts`, `buildHistoryMachineCosts`, `attachMachinesJson`, `renderSnapshotByFormat`/`renderHistoryByFormat`), `src/node/tui/formatter.ts` (`buildMachineColumns`, `renderMachineLegend`, `MACHINE_COL_WIDTH`, the `machineCosts` branches of `renderHistory`/`renderTotal`, `collectMachineNames`, `emitCsvSnapshot`/`emitCsvHistory`, `emitMarkdownSnapshot`/`emitMarkdownHistory`). Contract text: `docs/specs/usage.md` § Flags (`--by-machine`, `-u`), § Output Formats, § JSON/CSV/Markdown; `docs/specs/layouts.md` § 17 and § 12/15/16. Read `intake.md` § What Changes before starting — its design values are normative. Byte references appear under `bin/harness/report/cases/<id>/…/node.*` after `just go-diff --placeholder`.

### Command: guards and scope

#### R1: The two `--by-machine` warn-and-clear guards
`command.Normalize` SHALL apply, between its existing step 0 (`-u` in single mode) and step 1 (since/until on a snapshot), in this order: (a) `Flags.ByMachine && Source == "" && Display == History` → append the notice `Warning: --by-machine is not supported with all-tools history — ignoring.` and clear `ByMachine`; (b) `Flags.ByMachine && Display == LeaderboardHistory` → append `Warning: --by-machine is not supported with leaderboard history — ignoring.` and clear `ByMachine`. The `-u` notice MUST precede both; the since/until and `--full` notices MUST follow.

- **GIVEN** `Request{Display: History, Source: "", Flags: {ByMachine: true, User: "x"}}` in single mode
- **WHEN** `Normalize` runs
- **THEN** notices are exactly `[-u line, pivot line]` in that order, `ByMachine` is false and `User` is empty
- **AND** `Request{Source: "cc", Display: History, Flags: {ByMachine: true}}` produces no notice and keeps `ByMachine`

#### R2: `--by-machine` is in scope; leaderboards stay placeholder
`inScope` SHALL no longer reject `Flags.ByMachine`. `Display ∈ {Leaderboard, LeaderboardHistory}`, `Top`, `Watch`, `Sync`, `DryRun`, `NoRain`, `SkipBrewUpdate` MUST still return `ErrUnported`.

- **GIVEN** `Request{Flags: {ByMachine: true}}` (snapshot) and `Request{Display: Leaderboard, Flags: {ByMachine: true}}`
- **WHEN** `Run` executes
- **THEN** the first renders (no `ErrUnported`) and the second returns `ErrUnported`

#### R3: Dimension selection
The breakdown dimension SHALL be `query.Machine` with legend noun `Machines`, except when `cfg.Mode == config.Multi && Flags.User == "all"` (evaluated on the normalized request) where it is `query.User` with noun `Users`.

- **GIVEN** a single-mode config and `--by-machine -u all`
- **WHEN** `Run` executes
- **THEN** the `-u` notice is emitted and the table carries machine columns (one column, `cfg.Machine`) with a `Machines:` legend
- **AND** in multi mode the same flags produce user columns with a `Users:` legend

#### R4: `gather` returns un-collapsed records; callers collapse
`gather` SHALL return the stamped, un-collapsed records in today's order (own-machine `MaxMerge(live, own)` output first, then other machines in walk order; `-u all`: users ascending then walk order). `runSnapshot` and `runHistory` SHALL apply `query.Collapse(raw, Tool, Date)` before their existing `Window`/`RollUp`/`GroupBy` tail so the main-table bytes are unchanged.

- **GIVEN** the B3 multi-mode `Run` tests (`TestRunMultiOwnUserMerge`, `TestRunMultiAssociationOrder`, `TestRunMultiMonthlyJSON`) and the single-mode `Run` tests
- **WHEN** the suite runs after the refactor
- **THEN** every existing assertion still passes unchanged

### Query and view: the breakdown

#### R5: `query.Relabel`
`query` SHALL export `Relabel(recs []fact.Record, p Period) []fact.Record` — a pure copy of the input with each `Date` mapped to its period bucket (daily identity, weekly `WeekLabel`, monthly `Date[:7]`), no summing, no sorting; `RollUp` SHALL use the same relabel step.

- **GIVEN** records dated `2026-01-05`, `2026-01-06` with `Period == Monthly`
- **WHEN** `Relabel` runs
- **THEN** both records carry `Date == "2026-01"`, order and Totals unchanged, input not mutated

#### R6: `view.Breakdown` and its construction
`view` SHALL define `Breakdown{Noun string; Rows map[string][]Slice}` and `Slice{Name string; fact.Totals}`. `command` SHALL build it as follows, from the un-collapsed records:

- **History (single source)**: `GroupBy(Relabel(Window(raw, Since, Until), Period), Tool, Date, dim)` in one pass; `Rows[Key.Date]` appends `Slice{Key.<dim>, Totals}` in group order (first-seen).
- **Snapshot, all tools**: `GroupBy(Relabel(raw, Period), Tool, Date, dim)`; keep groups whose `Key.Date == CurrentLabel(Period, now)`; `Rows[displayName]` in group order. A tool with no matching group has no entry.
- **Snapshot, single source**: the key set is the first-seen order of `dim` values over ALL the tool's raw records (un-windowed); each slice's Totals are the current-label group's or the zero `fact.Totals{}` (the TS `toolMachines.set(machine, match ? … : 0)` zero-fill). `Rows[displayName]` always exists when the tool has any record, even on a zero-usage day.

Summation MUST happen in record input order within each group (GroupBy's `Totals.Add` in input order), reproducing the TS sequential association.

- **GIVEN** multi-mode `-u all`, monthly, user `harness-user` with machines `harness-machine` (`0.25`, `0.75`) then `other-box` (`0.40`) for cc in January
- **WHEN** the history breakdown is built with `dim == User`
- **THEN** `Rows["2026-01"]` is `[{harness-user, 1.4}, …]` summed as `((0.25+0.75)+0.40)` and `TestRunMultiAssociationOrder`-style bytes match the TS
- **AND** for the single-source snapshot on a day with no cc data in the multi seed, `Rows["Claude Code"]` is `[{harness-machine, 0}, {other-box, 0}]`

#### R7: `view.Snapshot` and `view.History` accept a breakdown
Both builders SHALL accept an optional `*Breakdown` (nil ⇒ today's output byte-for-byte; the snapshot also takes the `Metric` for its breakdown cells). With a non-nil breakdown that yields ≥1 name:

- **Names** = sorted union (`sort.Strings`) of all slice names over the rows the table renders; letters `string(rune('A'+i))`, uncapped.
- **Columns**: one `Column{Title: letter, Width: machineWidth, Align: Right}` per name appended after the metric column; `machineWidth = metricColumnWidth(all breakdown cell values ∪ per-name sums, m)` (floor 9).
- **Cells**: `metricCell(v, m)` per name in name order (`0` ⇒ dim), value = `metricValue(slice.Totals, m)` or 0 when absent. Snapshot cells use the displayed metric even though its base columns are metric-neutral.
- **Total row**: per-name sums via `fmtMetric` (never dim). Snapshot sums and the width pre-pass cover visible rows only (`TotalTokens > 0`); history covers every entry.
- **History bar budget**: `barBudget` subtracts `len(names) × (machineWidth + 3)` — bars follow the machine cells.
- **`Table.Note`** = `"{Noun}: A = name, B = name, …"`; set only when columns exist. Empty states keep today's early return (no columns, no note).
- `view.TotalHistory` is unchanged.

- **GIVEN** the snapshot `ToolTotals` rows for six tools where cc has `TotalTokens > 0` and a breakdown `{Machines, {"Claude Code": [{"Sahils-Mac-mini.local", 8.27}, {"dev-ws-sahil02", 6288.75}]}}` under `Cost`
- **WHEN** `view.Snapshot` builds
- **THEN** columns 7–8 are `A`, `B` width 9, the cc row's cells 7–8 read `$8.27`, `$6,288.75`, and `Note == "Machines: A = Sahils-Mac-mini.local, B = dev-ws-sahil02"`
- **AND** with a nil breakdown the table equals today's golden

#### R8: `ansi.Table` renders `Note`
After the footer line (or the Total row when no footer) and before the trailing `""`, when `t.Note != ""` the encoder SHALL append `""` then `c.Dim(t.Note)`. Headers (BoldCyan per cell), dividers over the extra columns, dim zero cells and BoldWhite Total cells MUST fall out of the existing encoder unchanged; machine cells precede the delta indicator and bar.

- **GIVEN** the golden inputs `snapshot_machines_*` and `history_machines_*`
- **WHEN** `ansi.Table` encodes
- **THEN** the color and no-color goldens agree under `StripANSI`, the history 80-column golden has no bar and the wide golden has machine cells followed by ` █…`, and the last two lines before the trailing blank are `""` and the dim legend

### Render: machine formats

#### R9: JSON `machines`
`json.Snapshot(rows, bd)` / `json.History(s, bd)` SHALL, for a row/entry whose breakdown has ≥1 slice, emit `"machines": {` after `totalTokens` (with the comma added to `totalTokens`), one line per slice `"{name}": {encodeFloat(TotalCost)}` in slice (first-seen) order — never sorted — then `}`, at the row's indent. Values are always cost. Rows with no slices are byte-identical to today. A zero-usage single-source snapshot tool MAY carry `machines` with `0` values (R6 zero-fill) and no `label`.

- **GIVEN** cc with `Label` set and slices `[{harness-machine, 0.75}, {other-box, 0.4}]`
- **WHEN** `json.Snapshot` encodes
- **THEN** the object body is `label, totalCost, …, totalTokens, machines: { "harness-machine": 0.75, "other-box": 0.4 }` with two-space indentation per level

#### R10: CSV `machine_{name}_cost`
`csv.Snapshot(rows, bd)` / `csv.History(s, bd)` SHALL append `machine_{name}_cost` per sorted name after `cost`; data rows append `Cost(v)` (`0.00` when absent); the snapshot `Total` row appends per-name sums over visible rows; history has no Total row. Under `-u all` the header prefix stays `machine_`.

- **GIVEN** the multi-seed January window for cc
- **WHEN** `csv.History` encodes with machine slices
- **THEN** the header is `date,input,output,cache_write,cache_read,total,cost,machine_harness-machine_cost,machine_other-box_cost` and the `2026-01-06` row ends `1.15,0.75,0.40`

#### R11: Markdown name-headed columns
`markdown.Snapshot(rows, period, bd)` / `markdown.History(s, period, capActive, bd)` SHALL append each sorted name verbatim as a header (alignment `---:`), cells `render.FormatCost`, and `**{sum}**` per name on the `**Total**` row (snapshot: visible rows; history: when `len(entries) > 1`). No letters, no legend line.

- **GIVEN** the same January window
- **WHEN** `markdown.History` encodes
- **THEN** the header row is `| Date | Input | Output | Cache Write | Cache Read | Total | Cost | harness-machine | other-box |` and the Total row ends `| **$1.00** | **$0.40** |`

### Edge and harness

#### R12: Label rule under `--by-machine`
`runSnapshot`'s single-mode daily-all `Label` clear MUST NOT apply when `Flags.ByMachine` is set (the TS by-machine path yields labelled entries).

- **GIVEN** single mode, `tu --by-machine --json`, cc with a current-label record
- **WHEN** `Run` executes
- **THEN** the cc object carries `"label"` first

#### R13: e2e and harness gate
`cmd/tu/e2e_test.go` SHALL replace `TestE2EHistoryByMachinePlaceholder` with populated and empty-state by-machine assertions (intake § 9); `harness/matrix.json` SHALL gain the nine cases listed in intake § 10. `just go-diff --placeholder` MUST report every `*by-machine*` case green with no previously green case regressed.

- **GIVEN** the rebuilt `bin/tu` and `dist/tu.mjs`
- **WHEN** `env -u TU_METRICS_REPO -u NO_COLOR just go-diff --placeholder` runs
- **THEN** the report lists no red case whose id contains `by-machine`, and the green count is at least 302 + 10 + 18

### Non-Goals
- `lb --by-machine`, `--top`, `lbh` rendering — B5.
- `--watch --by-machine`, `Prev` deltas on machine rows — B7.
- Any edit to `docs/specs/` or `src/node/`.

### Design Decisions

#### One breakdown structure carrying Totals
**Decision**: `view.Breakdown` slices carry `fact.Totals`; the ANSI table picks the display metric, the machine formats read `TotalCost`.
**Why**: The TS builds two maps (display metric and cost) per render; one structure with the consumer choosing the field removes the duplicate build and cannot drift.
**Rejected**: Two `map[string]map[string]float64` builds (the TS shape) — a second aggregation loop over the same records.
*Introduced by*: 260916-pmsd-machine-columns

#### Breakdown values from one GroupBy over relabelled records
**Decision**: `GroupBy(Relabel(Window(raw)), Tool, Date, dim)` in a single pass; no per-dimension aggregation and no `RollUp` on the breakdown path.
**Why**: Summing in record input order reproduces the TS sequential float association (observable in `--json` bytes under `-u all` with multi-machine users) and GroupBy's first-seen order reproduces the `machines` key order; it also satisfies the one-group-by architecture rule.
**Rejected**: `RollUp` then `GroupBy(Tool, Date, dim)` — sums each machine's days first, a different association that can differ in the last bit.
*Introduced by*: 260916-pmsd-machine-columns

#### Reproduce the single-source snapshot zero-fill
**Decision**: `tu cc --by-machine` lists every machine (or user) that has any record for the tool, zero-filled, in every format — including `machines` with `0` values on a zero-usage tool in JSON.
**Why**: D6 makes the byte-diff against the TS the release bar; the spec sentence "zero-usage tools gain nothing" (DC-01) describes only the all-tools path and is a G0 decision, not this row's.
**Rejected**: Following the spec sentence — a harness red on `snapshot-cc-by-machine-json`.
*Introduced by*: 260916-pmsd-machine-columns

## Tasks

### Phase 1: Query and command plumbing

- [x] T001 Export `Relabel(recs, p)` in `src/go/internal/query/query.go` (pure copy, relabel only) and make `RollUp` use it; table-driven test in `query_test.go` <!-- R5 -->
- [x] T002 In `src/go/internal/command/guards.go` add the two `--by-machine` notice constants and guard steps between the `-u` and since/until guards; add rows and an ordering case to `guards_test.go` <!-- R1 -->
- [x] T003 In `src/go/internal/command/run.go` remove `ByMachine` from `inScope`; make `gather`/`gatherOwn`/`gatherUser`/`gatherAllUsers`/single path return un-collapsed records and collapse in `runSnapshot`/`runHistory` before the existing tail; confirm the B3 `Run` tests still pass <!-- R2, R4 -->

### Phase 2: View model

- [x] T004 Add `src/go/internal/view/breakdown.go` with `Breakdown`, `Slice`, the sorted-name/letter derivation, shared-width computation, and the legend `Note` text; add `Table.Note` to `table.go`; unit tests in `breakdown_test.go` (letters past `Z`, width floor 9, sums) <!-- R7 -->
- [x] T005 Extend `view.Snapshot` in `snapshot.go` to take the breakdown and metric: machine columns after Cost, dim zero cells, per-name Total sums over visible rows, `Note`; nil breakdown byte-identical; tests in `snapshot_test.go` <!-- R7 -->
- [x] T006 Extend `view.History` in `history.go` to take the breakdown: columns after the metric column, cells before delta/bar, bar budget minus `len(names)×(w+3)`, Total sums over all entries, `Note`; tests in `history_test.go` (80-col no bar, wide bar) <!-- R7 -->

### Phase 3: Encoders

- [x] T007 In `src/go/internal/render/ansi/table.go` render `Note` (`""` then `Dim(note)` before the trailing blank); add goldens `snapshot_machines_color/nocolor/tokens/users`, `history_machines_80col/wide/two_zone` with the StripANSI twin assertions <!-- R8 -->
- [x] T008 [P] In `src/go/internal/render/json/{snapshot.go,history.go}` add the `machines` object per R9 (first-seen order, cost values, after `totalTokens`); goldens `snapshot_machines`, `snapshot_machines_zero_usage`, `history_machines` <!-- R9 -->
- [x] T009 [P] In `src/go/internal/render/csv/csv.go` add `machine_{name}_cost` columns per R10; goldens `snapshot_machines`, `history_machines` <!-- R10 -->
- [x] T010 [P] In `src/go/internal/render/markdown/markdown.go` add name-headed columns per R11; goldens `snapshot_machines`, `history_machines` <!-- R11 -->

### Phase 4: Composition, edge, harness

- [x] T011 In `src/go/internal/command/run.go` build the breakdown per R6 (dimension per R3; history, all-tools snapshot, single-source zero-fill), thread it to `view`/`json`/`csv`/`markdown`, and skip the label clear under `ByMachine` (R12); `run_test.go`: move the by-machine rows out of `TestRunUnported`, add dimension-switch, zero-fill key set, JSON key order, `-u all` association and single-mode `-u all` fallback tests <!-- R3, R6, R12 -->
- [x] T012 Update `src/go/cmd/tu/e2e_test.go`: replace `TestE2EHistoryByMachinePlaceholder` with the intake § 9 assertions (pivot warning + `  No data` for `h --by-machine`; multi-home `cc h --by-machine --since 2026-01-01 --until 2026-01-31` bytes with legend; `cc mh --by-machine --json` key order; `cc mh --by-machine -u all --csv`; single-home `--by-machine -u all` warning + machine column) <!-- R13 -->
- [x] T013 Add the nine matrix cases from intake § 10 to `harness/matrix.json`; run `just go-lint`, `just go-test`, and `env -u TU_METRICS_REPO -u NO_COLOR just go-diff --placeholder` (after `npm ci` if `node_modules` is absent); fix any red `*by-machine*` case until the report shows none and no prior green case regressed <!-- R13 -->

## Execution Order

- T001–T003 first (T004–T006 compile against `Table.Note` and the un-collapsed `gather`).
- T004 blocks T005/T006; T005/T006 block T007.
- T008–T010 are independent of each other and of T007.
- T011 needs T005–T010; T012–T013 need T011.

## Acceptance

### Functional Completeness

- [x] A-001 R1: Both `--by-machine` notices exist byte-exact in `guards.go`, positioned after the `-u` guard and before the since/until guard, and clear the flag
- [x] A-002 R2: `inScope` admits `ByMachine`; `lb`/`lbh` with `--by-machine` still return `ErrUnported`
- [x] A-003 R3: Machine vs User dimension and `Machines:`/`Users:` noun selected exactly as specified, including the single-mode `-u all` fallback to machines
- [x] A-004 R5: `query.Relabel` exported, pure, used by `RollUp`
- [x] A-005 R6: The breakdown is built in one `GroupBy` pass per table; the single-source snapshot zero-fills every historical key; the all-tools snapshot includes only current-label groups
- [x] A-006 R7: `view.Snapshot`/`view.History` append letter columns with a shared width, dim zero cells, per-name Total sums, `Note`, and the history bar budget subtracts the machine columns
- [x] A-007 R8: `ansi.Table` renders `Note` as a blank line then the dim legend, after the footer, before the trailing blank
- [x] A-008 R9: JSON `machines` objects appear after `totalTokens`, first-seen order, cost values, only for rows with slices
- [x] A-009 R10: CSV `machine_{name}_cost` columns, sorted, `0.00` fill, snapshot Total sums over visible rows
- [x] A-010 R11: Markdown name-headed columns with bold Total sums
- [x] A-011 R12: Single-mode daily-all label clear is skipped under `--by-machine`
- [x] A-012 R13: The nine matrix cases exist; `just go-diff --placeholder` shows every `*by-machine*` case green and no regression

### Behavioral Correctness

- [x] A-013 R4: Existing single-mode and B3 multi-mode `Run` tests pass unchanged after `gather` stops collapsing
- [x] A-014 R7: A nil breakdown leaves every existing view/ansi/json/csv/markdown golden byte-identical

### Scenario Coverage

- [x] A-015 R6: A `Run` test pins the `-u all` monthly association (`((a1+a2)+b1)` style) and the own-machine-first `machines` key order
- [x] A-016 R13: e2e asserts the multi-home `cc h --by-machine --since … --until …` bytes including `Machines: A = harness-machine, B = other-box`
- [x] A-017 R1: e2e asserts `h --by-machine` prints the pivot warning then the capped pivot heading + `  No data`, exit 0

### Edge Cases & Error Handling

- [x] A-018 R7: Empty states (`  No usage`, `  No data`) render no machine columns and no legend
- [x] A-019 R7: 27 names yield letters `A`…`Z`, `[`; width floor 9 holds when every cell is `$0.00`
- [x] A-020 R9: A zero-usage single-source tool carries `machines` with `0` values and no `label` (goldens `snapshot_machines_zero_usage`)

### Code Quality

- [x] A-021 Pattern consistency: new code follows the `view`/`render` helper style (small pure functions, doc comments naming the TS oracle), no ANSI or I/O below `render/ansi` and `cmd/tu`
- [x] A-022 No unnecessary duplication: the breakdown reuses `metricCell`, `metricColumnWidth`, `fmtMetric`, `barBudget`, `GroupBy`; no per-dimension aggregation loop
- [x] A-023 Readability: no god functions — `view.Snapshot`/`view.History` stay decomposed into helpers as today
- [x] A-024 Minimum pathways: one collapse path in `runSnapshot`/`runHistory`; no by-machine-specific fetch path
- [x] A-025 Error paths: no new `os.Exit`, no stderr writes below `cmd/tu`; `gofmt -l` and `go vet` clean (`just go-lint`)

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`
- Environment: run tests with `env -u TU_METRICS_REPO -u NO_COLOR …`; the shell exports both and they tilt config tests and manual probes.

## Deletion Candidates

- None — this change adds new functionality without making existing code redundant

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | `view.Snapshot`/`view.History` take the breakdown as an extra pointer parameter rather than an options struct | Two call sites each; nil keeps today's bytes; avoids reshaping `HistoryOptions` for one field | S:65 R:85 A:80 D:70 |
| 2 | Confident | The breakdown's name set for the ANSI/CSV/MD table is derived inside `view` (sorted union), while `command` only supplies slices in first-seen order | Keeps `command` free of column concerns and gives JSON its insertion order from the same data | S:70 R:80 A:85 D:75 |
| 3 | Confident | The all-tools snapshot breakdown windows nothing (the TS `aggregateMachineMap` gets no since/until on a snapshot because Normalize cleared them) | Since/until are cleared for snapshots at step 1, so `raw` is the full record set on both paths | S:60 R:85 A:85 D:80 |

3 assumptions (0 certain, 3 confident, 0 tentative).
