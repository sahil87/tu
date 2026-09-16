# Intake: Machine Columns (Go port row B4)

**Change**: 260916-pmsd-machine-columns
**Created**: 2026-09-17

## Origin

> Context: fab/plans/sahil/26-09-15-go-port.md, row B4. Read the Decisions and Target architecture sections before writing the intake. Do not change any external surface listed under Goal. Build the by-machine pivot via query.GroupBy(machine), machine legend, token-metric bars, stacked bars. Harness gate: -m cases.

One-shot `/fab-new` invocation from the Go-port operator queue (Phase 2, after B8 and B3). The row text in the plan reads: *"By-machine pivot via `GroupBy(machine)`, machine legend, token-metric bars, stacked bars. Harness: `-m` cases."* — size M, depends on B3 (merged as PR #90).

Key readings that shaped this intake (2026-09-17, from the worktree at `358ee50`):

- **Stacked bars, token-metric bars and the stacked-bar legend already exist in Go.** B2 (`260916-9ax5-history-and-periods`) landed `view.Apportion`, `Bar.Segments`, `Table.Legend`/`Swatch`, `ansi.fill` with the four-slot palette, and `HistoryOptions.Metric` driving cells/bars/footer in both history tables. The row's wording predates B2; B4 reuses these and re-implements nothing.
- **What is actually still red is the `--by-machine` flag itself**: `command.inScope` rejects `Flags.ByMachine`, so all ten `*by-machine*` harness cases print the placeholder (exit 1). The plan's "`-m` cases" means these `--by-machine` matrix cases (the flag has no short form; `m` is the monthly period token).
- **The seam is in place**: `command.gather` computes per-machine, per-user stamped records before `query.Collapse(Tool, Date)`; `query.GroupBy` accepts `Machine` and `User` dims; `metrics.Source.Read` stamps `User`/`Machine` from the directory walk; `ccusage.Source` stamps `cfg.User`/`cfg.Machine` on live records.
- **The TS reference** is `fetchToolMergedWithMachines` + `buildSnapshotMachineCosts` / `buildHistoryMachineCosts` / `attachMachinesJson` in `src/node/core/cli.ts`, `buildMachineColumns` / `renderMachineLegend` and the `machineCosts` branches of `renderHistory` / `renderTotal` / `emitCsv*` / `emitMarkdown*` in `src/node/tui/formatter.ts`, and `collectMachineNames`. Contract text: `docs/specs/usage.md` § Flags (`--by-machine`, `-u`), § Output Formats, § JSON/CSV/Markdown tables, DC-01; `docs/specs/layouts.md` § 17 Machine Columns, § 12/15/16 machine shapes.

## Why

**Problem.** The Go binary answers every snapshot and history request in single and multi mode except when `--by-machine` is present: `command.Run` returns `ErrUnported` and `cmd/tu` prints `tu: not implemented (Go port in progress)`, exit 1. Ten harness cases (`snapshot-all-by-machine`, `snapshot-cc-by-machine`, `cc-h-by-machine`, `h-by-machine`, `by-machine-user-all` × single/multi) are red for that reason alone, and the per-machine breakdown is the one data view multi-mode users reach for daily (which box is burning the budget).

**Consequence of not doing it.** B7 (watch mode) depends on B4 — `-w --by-machine` is a supported combination — and B5's `lb --by-machine` shares the dimension plumbing (`GroupBy` on `Machine`/`User`). Leaving the flag unported blocks the remaining Phase 2 rows and leaves a Goal-listed surface (the CLI grammar, table/JSON/CSV/Markdown output) unreproduced at cutover.

**Why this shape.** The plan's target architecture says *one* group-by for tool, machine and user pivots (§ Target architecture, `query`; G1 checklist item 2). The TS has three code paths (`machineMap` tuples, per-machine `aggregateForPeriod`, `collectMachineNames`) doing one group-by; the Go port computes the breakdown as a `query.GroupBy` over the same un-collapsed records the main table already flows through, hands `view` a render-agnostic breakdown, and lets each encoder append its own column shape. No new fetch path, no new adapter, no result globals (checklist item 3), and the machine legend is a table-model field the ANSI encoder alone renders.

**Constraint from the Goal.** Every external surface stays byte-identical to the shipped TS: letter-coded ANSI columns and legend, `machines` JSON objects (key order and float bytes included), `machine_{name}_cost` CSV columns, name-headed Markdown columns, both warn-and-clear stderr lines, exit codes. Where the TS behaviour and the spec disagree (see § Snapshot key set below), the harness byte-diff against the TS is the bar; the spec sentence is flagged for gate G0, not silently "fixed".

## What Changes

### 1. `command.Normalize` — the two `--by-machine` warn-and-clear guards

Insert between the existing step 0 (`-u` in single mode) and step 1 (`--since/--until` on a snapshot), in TS `main()` order:

```go
byMachinePivotNotice = "Warning: --by-machine is not supported with all-tools history — ignoring."
byMachineLbhNotice   = "Warning: --by-machine is not supported with leaderboard history — ignoring."
```

- `f.ByMachine && req.Source == "" && req.Display == History` → append `byMachinePivotNotice`; `f.ByMachine = false`.
- `f.ByMachine && req.Display == LeaderboardHistory` → append `byMachineLbhNotice`; `f.ByMachine = false`. (`lbh` itself stays `ErrUnported` until B5, and `Run` returns `ErrUnported` without notices, so this second guard changes no bytes today; it lands here because B4 owns the flag's guards and B5 owns the display.)

The `-u` notice still precedes both; the since/until and `--full` notices follow. `guards_test.go` gains the two rows and the ordering case (`-u other --by-machine h` in single mode → `-u` line, then the pivot line).

### 2. `command.inScope` and `Run` — admit `ByMachine`

Remove `f.ByMachine` from the `inScope` exclusion (`Watch`, `Sync`, `DryRun`, `NoRain`, `SkipBrewUpdate`, `Top` stay out; `Leaderboard`/`LeaderboardHistory` displays stay out, so `lb --by-machine` remains B5's placeholder).

**Dimension selection**, on the normalized request and post-guard config:

```go
dim, noun := query.Machine, "Machines"
if cfg.Mode == config.Multi && req.Flags.User == "all" {
    dim, noun = query.User, "Users"
}
```

Single mode already cleared `-u` in step 0, so `tu --by-machine -u all` in single mode prints the `-u` warning and renders machine columns (one column, the local `cfg.Machine`) — exactly the TS (`usersLegend` is computed after the clear).

**`gather` returns un-collapsed records.** Today every `gather` path ends in `query.Collapse(recs, query.Tool, query.Date)`. Move that collapse to the callers: `runSnapshot`/`runHistory` collapse for the main table (bytes unchanged — the collapse order is identical), and the by-machine branch groups the same raw records on the breakdown dim. Record order out of `gather` is unchanged and load-bearing: own-machine live records (the `MaxMerge(live, own)` output) first, then other machines in metrics-walk order (year asc, machine asc, file asc); for `-u all`, users ascending then walk order.

### 3. Breakdown computation — one `GroupBy` pass, TS association preserved

Add to `view`:

```go
// Breakdown is the --by-machine (or -u all, by user) column set for one table.
type Breakdown struct {
    Noun string              // "Machines" or "Users" — the legend label
    Rows map[string][]Slice  // row key → slices in first-seen order
}
// Slice is one breakdown cell's source: the machine/user name and its totals.
type Slice struct {
    Name string
    fact.Totals
}
```

Row key = tool display name for the snapshot, ISO label for the single-tool history. Slices carry `fact.Totals`, so the ANSI table picks `metricValue(t, metric)` (cost by default, tokens under `-t`) while JSON/CSV/Markdown always read `TotalCost` — one structure, no second build (the TS builds two maps).

`command` fills it from the un-collapsed records:

- **Single-tool history**: `recs := query.Window(raw, Since, Until)` (the same window the main table uses), relabel each record's `Date` to the period bucket (the `RollUp` relabel — expose it, e.g. `query.Relabel(recs, p)` returning a copy with re-labelled dates and no summing), then **one** `query.GroupBy(relabelled, query.Tool, query.Date, dim)` pass. `Rows[g.Key.Date]` appends `Slice{g.Key.<dim>, g.Totals}` in group order. GroupBy's first-seen order gives the TS `machineMap` iteration order per label (own machine first, then walk order), and summing in input order reproduces the TS float association (`aggregateMonthly` / `buildHistoryMachineCosts` sum sequentially over the flattened walk-ordered entries — this matters under `-u all` for a user with two machines in one month: `((a1+a2)+b1)+b2`, never `(a1+a2)+(b1+b2)`).
- **All-tools snapshot**: `cur := query.CurrentLabel(Period, now)`; same relabel + `GroupBy(relabelled, Tool, Date, dim)`; keep only groups with `Key.Date == cur`; `Rows[displayName]` gets one slice per matching group. A tool with no current-label group has no entry (its JSON object gains no `machines`; DC-01).
- **Single-source snapshot** (`tu cc --by-machine`): the TS iterates every key of the tool's `machineMap` — every machine (or user) that has **any** record for that tool, un-windowed — and stores `0` for keys without a current-label entry. Reproduce it: the key set is the first-seen order of `dim` values over all the tool's daily records; each slice's totals are the current-label group's totals or the zero `fact.Totals{}`. Consequences the harness can observe: `tu cc --by-machine --json` emits `"machines": {"harness-machine": 0, "other-box": 0}` on a zero-usage day (a `machines` object with zeros, on a tool with no `label`), and the ANSI table shows every historical machine as a dim `$0.00` column. **This contradicts the spec sentence "zero-usage tools gain nothing" (usage.md § JSON, DC-01) for the single-source path** — record it in the plan/hydrate as a G0 candidate (`[DECIDE]`-shaped note in memory; the spec is human-curated and not edited by this row). The harness is the bar.

`Result.CostByItem`/`TotalCost`/`TotalTokens` are unchanged (the TS `buildCostMap` never includes machines).

### 4. `view` — machine columns as ordinary columns, plus a legend note

Extend the table model with one field:

```go
// Note is a trailing dim line (the machine legend), preceded by a blank
// line and rendered after Footer; "" when absent.
Note string
```

`view.Snapshot(rows, p, bd *Breakdown, m Metric)` and `view.History(s, o, bd *Breakdown)` (signatures indicative — keep the existing call shape where possible; nil = today's output byte-for-byte). With a non-nil breakdown that has at least one name:

- **Names**: sorted union of every slice name across all rows (`sort.Strings` — byte order, equal to the JS default sort for ASCII hostnames). Letters are `string(rune('A'+i))`, continuing past `Z` exactly as `String.fromCharCode(65+i)` does — no cap.
- **Columns**: one `Column{Title: letter, Width: machineWidth, Align: Right}` per name appended after the metric column (`Cost`/`Tokens`); all share one width `machineWidth = metricColumnWidth(every machine cell value ∪ every per-machine sum, m)` — floor 9 (the TS `MACHINE_COL_WIDTH = COST_WIDTH`).
- **Cells**: `metricCell(v, m)` per name (dim on exact zero; `0` when the row has no slice for that name). Snapshot cells use the displayed metric even though the snapshot's own columns are metric-neutral (`tu -t --by-machine` shows token machine cells beside a `$` Cost column — the TS does exactly this).
- **Sums**: the Total row carries per-machine sums (`fmtMetric`, never dim). Snapshot sums and the width pre-pass run over **visible** rows only (`TotalTokens > 0`); history over every entry.
- **History bar budget**: `barBudget` subtracts `len(names) × (machineWidth + 3)` before the bar/leading space — the TS `machineColsWidth`. Machine cells sit between the metric cell and the delta indicator/bar (`rowStr + costBase + machineCells + indicator + bar`), which is what appending them as trailing cells gives with the existing encoder (delta and bar are appended after the last cell).
- **Note**: `"{Noun}: A = name, B = name, …"` (`renderMachineLegend`). Set only when columns exist; the empty states (`  No usage`, `  No data`) return early with no columns and no note, as in the TS.
- Dividers and separators extend over the machine columns automatically (`divider(t.Columns)`).

`view.TotalHistory` is untouched (the guard clears the flag before it is reached).

### 5. `render/ansi` — render `Note`

After the footer line and before the trailing `""`: when `t.Note != ""` append `""` then `c.Dim(t.Note)`. Everything else (header letters `BoldCyan` per cell, dividers, dim zero cells, Total `BoldWhite`) falls out of the existing encoder. Goldens: `snapshot_machines_color`, `snapshot_machines_nocolor`, `snapshot_machines_tokens` (token machine cells beside `$` Cost), `snapshot_machines_users` (`Users:` legend), `history_machines_80col` (columns eat the bar budget → no bar), `history_machines_wide` (Width 160: columns + bar), `history_machines_two_zone` (the rule column after the machine cells).

### 6. `render/json` — `machines` objects

`json.Snapshot(rows, bd)` / `json.History(s, bd)`: when the row/entry has ≥1 slice, emit `"machines": {` after `totalTokens` (preceded by a comma on `totalTokens`), one line per slice `"{name}": {encodeFloat(TotalCost)}` in **slice order** (first-seen, NOT alphabetical — `attachMachinesJson` copies the `Map` in insertion order), then `}`. Values are raw doubles (DC-10) and always cost, even under `-t`. Rows with no slices get no key. Under the single-source snapshot rule (§ 3) a zero-usage tool CAN carry `machines` with `0` values — reproduce, flag for G0. Goldens: `snapshot_machines`, `snapshot_machines_zero_usage`, `history_machines`.

### 7. `render/csv` — `machine_{name}_cost` columns

`csv.Snapshot(rows, bd)` / `csv.History(s, bd)`: names sorted (byte order); header gains `machine_{name}_cost` per name after `cost`; each data row appends `Cost(v)` per name (`0.00` when absent — `csv.Cost` on the exact binary value, as today); the snapshot's `Total` row appends per-machine sums over visible rows; history never has a Total row. Under `-u all` the names are user names but the header prefix stays `machine_`. Header only when nothing is visible. Goldens: `snapshot_machines`, `history_machines`.

### 8. `render/markdown` — name-headed columns

`markdown.Snapshot(rows, period, bd)` / `markdown.History(s, period, capActive, bd)`: names sorted; the header uses each name verbatim (no letters, no legend line), alignment `---:`; cells `render.FormatCost` (never dim, always cost); the `**Total**` row appends `**{sum}**` per name (snapshot: visible rows only; history: when `len(entries) > 1`). Goldens: `snapshot_machines`, `history_machines`.

### 9. `cmd/tu` and tests

- `e2e_test.go`: `TestE2EHistoryByMachinePlaceholder` flips — `h --by-machine` prints the pivot warning on stderr then the capped pivot heading + `  No data`, exit 0; `cc h --by-machine` prints the capped single-tool heading + `  No data`, exit 0. Add: multi-home `cc h --by-machine --since 2026-01-01 --until 2026-01-31` (letter columns A = harness-machine, B = other-box; `2026-01-06` row `$0.75` / `$0.40`; Total `$1.00` / `$0.40`; legend `Machines: A = harness-machine, B = other-box`), `cc mh --by-machine --json` (`machines` with `harness-machine` before `other-box`), `--by-machine -u all` in the multi home (`Users:` legend keys `harness-user`, `other-user` — snapshot on today is `  No usage`, so use `cc mh --by-machine -u all --csv` for populated bytes), and single-home `--by-machine -u all` (the `-u` warning, machine columns).
- `run_test.go`: move the three `by-machine` rows out of `TestRunUnported`; add fake-`Fetcher`/fake-`Repo` tests for the dimension switch, the single-source zero-fill key set, JSON key order (own machine first) and the `-u all` float association (an association-sensitive triple like the B3 `2.15` test).
- `view` tests: width sharing, letter assignment (27 names → `[`), the bar-budget subtraction, `Note` presence/absence.
- `guards_test.go`: the two notices and their position.

### 10. Harness matrix — populated `--by-machine` cases

The ten existing `*by-machine*` cases go green but exercise only empty states with the placeholder corpus (today never matches the 2026-01 fixture dates; the capped `cc h` window excludes January). Add cases whose bytes are populated on both confs (additive; the burndown denominator grows from 368):

```json
{ "id": "cc-h-window-by-machine", "args": ["cc", "h", "--since", "2026-01-01", "--until", "2026-01-31", "--by-machine"], "conf": ["single", "multi"] },
{ "id": "cc-mh-by-machine", "args": ["cc", "mh", "--by-machine"], "conf": ["single", "multi"] },
{ "id": "cc-mh-by-machine-json", "args": ["cc", "mh", "--by-machine", "--json"], "conf": ["single", "multi"] },
{ "id": "cc-mh-by-machine-csv", "args": ["cc", "mh", "--by-machine", "--csv"], "conf": ["single", "multi"] },
{ "id": "cc-mh-by-machine-md", "args": ["cc", "mh", "--by-machine", "--md"], "conf": ["single", "multi"] },
{ "id": "cc-mh-by-machine-tokens", "args": ["cc", "mh", "--by-machine", "-t"], "conf": ["single", "multi"] },
{ "id": "cc-mh-by-machine-user-all", "args": ["cc", "mh", "--by-machine", "-u", "all"], "conf": ["single", "multi"] },
{ "id": "cc-mh-by-machine-user-all-json", "args": ["cc", "mh", "--by-machine", "-u", "all", "--json"], "conf": ["single", "multi"] },
{ "id": "snapshot-cc-by-machine-json", "args": ["cc", "--by-machine", "--json"], "conf": ["single", "multi"] }
```

The last case pins the single-source zero-fill quirk (`machines` with `0` values on a zero-usage day). `just go-diff --placeholder` is the gate: every `*by-machine*` case green, no previously green case regresses, the report's green count rises by the ten flipped cases plus the new ones.

### Out of scope (owned elsewhere)

- `lb --by-machine` (`user/machine` rows, JSON `machine` field, CSV `machine` column) — B5.
- `--watch --by-machine` and the `Prev` delta on machine rows — B7.
- The metrics-repo writer before the own-user read — B6.
- Any spec edit: the DC-01 single-source discrepancy is recorded in memory for gate G0, not applied to `docs/specs/`.
- Constitution: Principles I, II, V hold (pure `query`/`view`/`render`, warnings on stderr at the edge, one fact type); Go Transition article unchanged.

## Affected Memory

- `go-port/command-edge`: (modify) `Normalize` gains the two `--by-machine` guards (position between steps 0 and 1); `inScope` admits `ByMachine`; `Run` composes the breakdown (dimension switch, single-source zero-fill key set, first-seen order); the placeholder row map drops B4 from the unported list; the e2e inventory and the harness gate status (green count, remaining red owned by B5/B6/B7) are updated.
- `go-port/query-view-render`: (modify) `query.Relabel` (or the exposed relabel step) and its relation to `RollUp`; `view.Breakdown`/`Slice`, `Table.Note`, the extended `Snapshot`/`History` signatures and column rules (shared width, letters, sums over visible rows, bar-budget subtraction); `ansi.Table` renders `Note`; `json`/`csv`/`markdown` machine-column shapes and key/column ordering; the golden inventory.
- `go-port/multi-mode`: (modify) `gather` returns un-collapsed records and the callers collapse; the "Seams the later rows build on" sentence about B4 becomes present truth; the `-u all` float-association rule for the breakdown.
- `harness/differential-harness`: (modify) the added `*by-machine*` matrix cases and the new case-count.

TS-facing memory (`display/formatting`, `cli/data-pipeline`, `sync/multi-machine`) and `docs/specs/` are unchanged — the shipped TypeScript is frozen (D4) and the Goal surfaces are the contract this row reproduces.

## Impact

- **Code** (`src/go/`): `internal/command/{run.go,guards.go,run_test.go,guards_test.go}`; `internal/query/{query.go,query_test.go}` (relabel export); `internal/view/{table.go,snapshot.go,history.go,breakdown.go (new),breakdown_test.go (new),snapshot_test.go,history_test.go}`; `internal/render/ansi/{table.go,table_test.go,history_test.go,testdata/*}`; `internal/render/json/{snapshot.go,history.go,*_test.go,testdata/*}`; `internal/render/csv/{csv.go,csv_test.go,testdata/*}`; `internal/render/markdown/{markdown.go,markdown_test.go,testdata/*}`; `cmd/tu/e2e_test.go`.
- **Harness**: `harness/matrix.json` (additive cases). Gate: `just go-diff --placeholder` (builds `dist/tu.mjs`, `bin/tu`, `bin/harness/*`; neither binary exists in this fresh worktree yet — run `npm ci` first, see the worktree memory note).
- **Not touched**: `src/node/`, `docs/specs/`, the formula, CI. No new dependency. `--help`, `help-dump`, exit codes, `tu.conf` unchanged.
- **Risk**: JSON float bytes and key order under `-u all` with a multi-machine user (the seed's `harness-user` has two machines, so the matrix cases above observe it); the single-source zero-fill quirk (observable via `snapshot-cc-by-machine-json`); the history bar budget with machine columns (only goldens observe bars — piped output is 80 columns).
- **Size**: M, as planned — roughly 400 lines of Go plus goldens; one PR.

## Open Questions

- None blocking. The single-source snapshot zero-fill behaviour (§ 3) is a TS-vs-spec discrepancy to be resolved at gate G0, not by this row.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Scope is the `--by-machine` surface on the snapshot and single-tool history in all four formats plus both warn-and-clear guards; `lb --by-machine` stays B5, `--watch` B7 | Plan row B4 and the command-edge memory's row map name exactly this split; the Goal freezes the surfaces | S:85 R:70 A:90 D:90 |
| 2 | Certain | Stacked bars, token-metric bars and the stacked-bar legend are reused from B2, not re-implemented | `view.Apportion`, `Bar.Segments`, `Table.Legend`, `HistoryOptions.Metric` and the goldens `pivot_wide_stacked`/`history_tokens` exist on `main`; the row wording predates B2 | S:80 R:85 A:95 D:90 |
| 3 | Confident | One `view.Breakdown` with `fact.Totals` per slice feeds all four encoders; the ANSI legend is a new `Table.Note` field | Avoids the TS double map (display-metric vs cost); the encoders already handle extra columns, only the legend line is new | S:70 R:75 A:80 D:65 |
| 4 | Confident | Breakdown values come from one `GroupBy(Tool, Date, dim)` pass over relabelled, windowed, un-collapsed records, summing in input order | Reproduces the TS sequential float association and the first-seen `machines` key order; honours the one-group-by rule (G1 item 2) | S:65 R:70 A:75 D:60 |
| 5 | Confident | `gather` returns un-collapsed records; `runSnapshot`/`runHistory` collapse for the main table | Same records, same order, same collapse — main-table bytes unchanged; the breakdown needs the dimension the collapse drops | S:70 R:80 A:85 D:75 |
| 6 | Confident | The single-mode daily-all `label` clear does NOT apply under `--by-machine` | The TS by-machine snapshot goes through `fetchToolMergedWithMachines` (labelled entries), never `fetchAllTotals`; `tu --by-machine --json` in single mode carries `label` | S:60 R:85 A:85 D:80 |
| 7 | Confident | Both `--by-machine` guards (all-tools pivot and `lbh`) land in `Normalize` now; `lbh` stays `ErrUnported` until B5 | The flag's guards belong with the flag; the second guard emits no bytes until B5 makes `lbh` in scope | S:60 R:90 A:80 D:70 |
| 8 | Confident | Add nine populated `--by-machine` matrix cases (windowed and monthly history, json/csv/md, `-t`, `-u all`, single-source json) | The ten existing cases only hit empty states with the placeholder corpus; D6 makes the harness a first-class deliverable; additive, denominator grows | S:55 R:85 A:70 D:60 |
| 9 | Certain | Change type is `feat`, pinned explicitly | Sibling rows B3/B8 shipped as `feat`; the quoted Goal's "redesigned" would re-infer `refactor` | S:80 R:95 A:95 D:95 |
| 10 | Confident | Letter codes continue past `Z` as `rune('A'+i)` with no cap | Mirrors `String.fromCharCode(65 + i)` byte-for-byte; a 27th machine is unrealistic but cheap to match | S:40 R:90 A:70 D:60 |
| 11 | Confident | Machine and user names sort in byte order (`sort.Strings`) | Equals the JS default sort for ASCII hostnames and profile names; non-ASCII names are out of scope for parity | S:50 R:90 A:80 D:70 |
| 12 | Certain | Under multi-mode `-u all --by-machine` the dimension is `User` and the legend noun `Users`; single mode clears `-u` first and shows one machine column | Spec § Flags and layouts § 17 say so; the TS computes `usersLegend` after the `-u` guard | S:85 R:85 A:95 D:90 |
| 13 | Confident | Reproduce the single-source snapshot zero-fill quirk (`machines` with `0` for every historical machine, even on a zero-usage tool) and flag the DC-01 sentence for G0 | The harness byte-diff is the bar (D6); the drop list is G0's decision (P1), not this row's | S:55 R:75 A:80 D:65 |
| 14 | Certain | Memory updates touch the three go-port files and the harness memory only; TS-facing memory and specs are untouched | D4 freezes `src/node/`; the Goal fixes the surfaces; hydrate owns the go-port domain | S:70 R:90 A:85 D:85 |

14 assumptions (5 certain, 9 confident, 0 tentative, 0 unresolved).
