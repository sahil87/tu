# Plan: History Displays, Windows, Bars and the CSV/Markdown Encoders (Go port row B2)

**Change**: 260916-9ax5-history-and-periods
**Intake**: `intake.md`

> The intake is the design document: its §3–§12 carry every constant, rule, byte reference and
> node-verified value this plan refers to. Section references below (`intake §4.3` etc.) are
> normative — read the referenced section before implementing the requirement.

## Requirements

### Query: cap floor and composition order

#### R1: ThreeMonthFloor
`query.ThreeMonthFloor(now time.Time) string` SHALL return the first day of the local month two calendar months before `now`, formatted `2006-01-02`, using `now.Location()` and `time.Date` month normalization (intake §3).

- **GIVEN** `now` = 2026-09-16 in `Asia/Kolkata`
- **WHEN** `ThreeMonthFloor(now)` runs
- **THEN** the result is `2026-07-01`; for 2026-01-15 it is `2025-11-01`; for 2026-02-28 it is `2025-12-01`

#### R2: Window before RollUp on the history path
The history pipeline MUST compose `query.RollUp(query.Window(recs, since, until), period)` — the window applies to daily records, the roll-up second. The snapshot's V2 composition (`Window(RollUp(recs, p), cur, cur)`) MUST NOT change.

- **GIVEN** daily records 2026-01-05..07 and `--since 2026-01-01 --until 2026-01-31` with `Period == Weekly`
- **WHEN** the history path runs
- **THEN** one entry labeled `2026-01-04` (the Sunday preceding `since`) sums the three days

### View: model and the two history tables

#### R3: Typed model slots, no ANSI
`view.Table`/`Row`/`Cell` SHALL gain the typed slots of intake §4.1 — `Cell.Style` (`Plain`/`Current`/`Weekend`), `Row.Bar *Bar`, `Row.Delta`, `RowKind` `Separator`, `Table.Footer string`, `Table.Legend []Swatch`, `Table.Scale Scale`, `Table.DeltaSpaced bool` — and MUST carry no escape sequence anywhere. `view.Snapshot` output MUST be unchanged (Style Plain, no Bar, Scale.Width 0) so the four V2 ansi goldens stay byte-identical. Exact field names MAY differ from the intake sketch; the information content MUST NOT.

- **GIVEN** the existing `render/ansi` snapshot goldens
- **WHEN** the model change lands
- **THEN** `go test ./internal/render/ansi/` passes without `-update`

#### R4: view.History (single-tool table)
`view.History(s Series, o HistoryOptions) Table` SHALL reproduce `renderHistory` per intake §4.3: title `📊 {Name} ({period}[, last 3 months])` (metric-independent); `Empty = "  No data"` when no entries; columns Date 12 Left, five token columns 14 Right, last column `Cost`/`Tokens` data-sized `max(9, longest fmtMetric incl. sum)`; `barWidth = min(Width − 97 − 3 − costWidth − 1, 30)` shown when ≥ 10; Data rows with `FormatInt` token cells and a `metricCell` last cell (Dim iff value == 0); `Cells[0].Style` per `labelStyle`; a `Separator` row before a Data row whose `label[:7]` changes, daily only, never first; Divider + Total + Footer only when `len(entries) > 1`; `Delta` from `Prev["{Name}:{label}"]`; `DeltaSpaced = true`; bars solid (`Segments == nil`).

- **GIVEN** the placeholder single-tool series (three January days, `$0.50` each) and `Width 80`
- **WHEN** `History` runs with `Period Daily`, no cap, `Now` = 2026-09-16
- **THEN** the table has header, divider, three plain Data rows, divider, Total `9,000 / 1,200 / 3,000 / 60,000 / 73,200 / $1.50`, footer `avg $0.50/day · peak $0.50 (2026-01-05)`, no bars, no separators
- **GIVEN** a one-entry series (`mh`)
- **WHEN** `History` runs
- **THEN** one Data row and no Divider/Total/Footer

#### R5: view.TotalHistory (cross-tool pivot)
`view.TotalHistory(series []Series, o HistoryOptions) Table` SHALL reproduce `renderTotalHistory` per intake §4.4: title `📊 Combined Cost History (…)` or `📊 Combined Token History (…)`; labels = sorted union; `Empty = "  No data"`; visible tools by the significance rule (`≥ negligibleAbs` AND `≥ 0.001 × grand`, boundary kept) with fallbacks nonzero → all; `rowValue`/`grandTotal` over ALL series, `values`/`toolSums` over visible; widths Date 10, per tool `max(len(Name), 9, cells, toolSum)`, last `max(9, rowValues, grand)`; `barWidth = min(Width − tableWidth − 3 − costWidth − 1 − indicatorReserve, 30)`, `indicatorReserve = 1` iff `Prev != nil`; Data cells via `metricCell`; `Delta` from `Prev["total:{label}"]`; `Bar.Segments = Apportion(values, runes(Main))`; Separator/Total/Footer rules as R4; `Legend` one `Swatch` per visible tool iff bars shown ∧ ≥ 2 visible; `DeltaSpaced = false`.

- **GIVEN** the six placeholder series (equal `$0.50` cells) and `Width 80`
- **WHEN** `TotalHistory` runs daily
- **THEN** all six tools are visible, `Claude Code` column width 11 and the rest 9, table width 84, no bars, Total row `$1.50 ×6 / $9.00`, footer `avg $3.00/day · peak $3.00 (2026-01-05)`
- **GIVEN** a window where one tool totals `$0.04` and another `$0.00` among real spend
- **WHEN** `TotalHistory` runs under cost
- **THEN** both columns are omitted, their cost still counts in every `rowValue`, the Total and the footer

#### R6: Footer text
The footer text SHALL be `avg {v}{/day|/week|/month}[ · this month {v}] · peak {v} ({label})[ · ┊ = {p95} (p95)]` per intake §4.5: `this month` daily-only over rows prefixed by `CurrentLabel(Monthly, Now)`, omitted when none match; peak is the first strict maximum starting from 0 and an all-zero window prints `peak $0.00` with no parentheses; parts joined by ` · `.

- **GIVEN** three rows all `$0.50` in January and `Now` in September
- **WHEN** the footer is built
- **THEN** it is exactly `avg $0.50/day · peak $0.50 (2026-01-05)`

#### R7: Bar primitives and scale
`view` SHALL provide `Glyphs(value, max, width)`, `Percentile(sortedAsc, p)`, `ComputeScale(values, barWidth)` and `Apportion(shares, total)` with exactly the semantics of intake §4.6 (eighths table, `▏` floor, `eighths == 8` carry, linear-interpolation percentile, `max > 1.5 × p95` trigger, `OverflowZone = max(4, round(barWidth/4))`, `MainZone = barWidth − OverflowZone − 1`, largest remainder with ties to the earlier index). Per-row bars: single-zone `Main = Glyphs(v, Max, Width)`; two-zone `Main = Glyphs(min(v, P95), P95, MainZone)` and `Overflow = Glyphs(v − P95, Max − P95, OverflowZone)` only when `v > P95`.

- **GIVEN** values `[846.21, 1091.67, 4031.61, 172.13]` and `barWidth 30`
- **WHEN** `ComputeScale` runs
- **THEN** it is two-zone with `OverflowZone 8`, `MainZone 21`, and the row at exactly p95 has empty `Overflow`
- **GIVEN** shares `[2, 1, 1]` and `total 5`
- **WHEN** `Apportion` runs
- **THEN** the result sums to 5 and the tie is broken toward the earlier index (`[3, 1, 1]`)

#### R8: Label styles and exact-zero dimming
A Data row's first cell SHALL carry `Current` when its label equals `query.CurrentLabel(Period, Now)`, else `Weekend` when `Period == Daily` and the UTC-parsed label is Saturday or Sunday, else `Plain`; a metric cell SHALL be `Dim` iff its value is exactly 0 (never Header/Total cells).

- **GIVEN** a daily window containing a Saturday label equal to `Now`'s date
- **WHEN** styled
- **THEN** that cell is `Current`, not `Weekend`

#### R9: Token mode on history
Under `Metric == Tokens` both tables SHALL render every metric cell, the Total's metric cell, bars and footer from `TotalTokens` via `FormatInt(round(v))`, the last header SHALL read `Tokens`, the pivot title `Combined Token History`, and the significance thresholds SHALL be `1000` tokens and `0.001 × grand` in tokens. Under `Metric == Cost` output MUST be byte-identical to a metric-less render.

- **GIVEN** the placeholder single-tool series under tokens
- **WHEN** `History` runs
- **THEN** the last column reads `Tokens` with `24,400` cells and Total `73,200`; footer `avg 24,400/day · peak 24,400 (2026-01-05)`

### Render: ansi

#### R10: ansi.Table encodes the new model
`ansi.Table` SHALL add, per intake §5.1: pad-then-wrap label styling (`BoldWhite` Current, `Dim` Weekend); pad-then-`Dim` for `Cell.Dim`; `Separator` rows as `Dim(divider + barDiv)` where `barDiv = "─" + "─"×Scale.Width` when `Scale.Width > 0` (also appended to header/Total dividers); delta `Green("↑")`/`Red("↓")` with a leading space iff `DeltaSpaced`, after the last cell; bars after the delta — single-zone `" " + fill(Main)` only when `Main != ""`, two-zone always `" " + fill(Main) + pad + Dim("┊") + Yellow(Overflow) + pad` with **no empty-string guard** (`Green("")`/`Yellow("")` emit `ESC[32mESC[0m`/`ESC[33mESC[0m` when color is on); `fill` solid `Green` when `Segments == nil`, else palette runs (`Green`, `Magenta`, `Blue`, `Cyan`, slot ≥ 4 plain) over exact rune slices; `Dim(Footer)` then, when `Legend != nil && c.Enabled`, `Dim(" · ")` + swatches `palette(i)("█") + " " + Dim(Name)` joined by `" "`. `StripANSI(colored)` MUST equal the no-color render for every golden.

- **GIVEN** the R5 placeholder pivot at `Width 80`, color on
- **WHEN** encoded
- **THEN** the lines equal intake §12's `h-window` capture byte for byte
- **GIVEN** a two-zone window with a zero-value row, color on
- **WHEN** encoded
- **THEN** that row's bar is `" " + ESC[32mESC[0m + spaces(MainZone) + ESC[2m┊ESC[0m + ESC[33mESC[0m + spaces(OverflowZone)`

### Render: csv

#### R11: csv encoder kinds
`render/csv` SHALL provide `Snapshot(rows)`, `History(s)`, `TotalHistory(series)` returning lines per intake §6: headers `tool,tokens,input,output,cache,cost` / `date,input,output,cache_write,cache_read,total,cost` / `date,{Name…},total`; snapshot rows only for `TotalTokens > 0` with a `Total` row (summing every input row) only when more than one row is visible; the two history kinds NEVER carry a Total row; TotalHistory keeps every series column (no omission) over the sorted label union; integers raw; every field passes RFC 4180 quoting (`,` `"` `\n` `\r`); an empty window yields the header alone.

- **GIVEN** the placeholder `cc mh` series
- **WHEN** `History` runs
- **THEN** lines are `date,input,output,cache_write,cache_read,total,cost` and `2026-01,9000,1200,3000,60000,73200,1.50`

#### R12: csv.Cost rounding rule
`csv.Cost(x)` SHALL format the **exact** binary value of `x` rounded half-up to two decimals (JS `toFixed(2)`), implemented on `math/big` (`big.Rat.SetFloat64`), never `strconv.FormatFloat(x, 'f', 2, 64)` and never `render.FormatCost`.

- **GIVEN** the node-verified inputs of intake §6
- **WHEN** formatted
- **THEN** `1.005 → 1.00`, `0.125 → 0.13`, `0.375 → 0.38`, `2.675 → 2.67`, `0.015 → 0.01`, `1.045 → 1.04`, `8.345 → 8.35`, `0.005 → 0.01`, `0.045 → 0.04`, `999999.995 → 999999.99`, `1234567.891 → 1234567.89`, `0.5 → 0.50`, `0 → 0.00`

### Render: markdown

#### R13: markdown encoder kinds
`render/markdown` SHALL provide `Snapshot(rows, period)`, `History(s, period, capActive)`, `TotalHistory(series, period, capActive)` per intake §7: `## {title}`, blank, header, alignment row (`:---` text, `---:` numeric), data rows, `**Total**` row with bold cells when more than one data row (snapshot: more than one visible), then a blank line; titles `Combined Usage ({period})`, `{Name} ({period}[, last 3 months])`, `Combined Cost History ({period}[, last 3 months])` (always Cost, cells always cost); numbers via `render.FormatInt`/`FormatCost`; TotalHistory columns = exact-zero omission over all labels with all-six fallback.

- **GIVEN** the placeholder `cc mh` series
- **WHEN** `History` runs
- **THEN** the lines equal intake §12's `cc-mh-md` capture including the trailing blank line
- **GIVEN** no records, cap active
- **WHEN** `TotalHistory` runs
- **THEN** the lines equal the `h-md` capture (heading with hint, six tool columns, no data rows)

### Render: json

#### R14: json history shapes
`render/json` SHALL provide `History(s)` (bare array of entry objects, `[]` when empty) and `TotalHistory(series)` (object keyed by display name in input order, `[]` inline for an empty series) with entry keys `label, totalCost, inputTokens, outputTokens, cacheCreationTokens, cacheReadTokens, totalTokens`, laid out as `JSON.stringify(v, null, 2)` (intake §8), scalars through the V2 helpers.

- **GIVEN** the placeholder `cc mh` series
- **WHEN** `History` runs
- **THEN** the lines equal intake §12's `cc-mh-json` capture
- **GIVEN** six empty series
- **WHEN** `TotalHistory` runs
- **THEN** the lines are `{`, `  "Claude Code": [],` … `  "Kimi": []`, `}`

### Command

#### R15: Normalize — guards and the cap
`command.Normalize(req, now) (Request, []string, bool)` SHALL be pure and apply, in order (intake §9.1): (1) since/until set and `Display != History` → notice `Warning: --since/--until apply to history display — ignoring.` and clear both; (2) `Full` and `Display != History` → notice `Warning: --full applies to daily/weekly history — ignoring.`; (3) `Display == History && Period != Monthly && Since == "" && Until == "" && !Full` → `Since = query.ThreeMonthFloor(now)`, `capActive = true`.

- **GIVEN** `tu --since 2026-13-01` (snapshot)
- **WHEN** normalized
- **THEN** the notice is emitted, `Since` is empty, and the request is in scope
- **GIVEN** `tu mh --full`, `tu h --since 2026-01-01`, `tu wh`
- **WHEN** normalized
- **THEN** no notice and no cap; no notice and no cap; cap active with `Since` = the floor

#### R16: Scope
`inScope` SHALL evaluate the normalized request and accept `Display ∈ {Snapshot, History}`, all four `Format`s, and `Since`/`Until`/`Full`; it MUST still reject multi mode, `User`, `ByMachine`, `Top`, `Watch`, `Sync`, `DryRun`, `NoRain`, `SkipBrewUpdate`, any `Command`, and `Version` with `ErrUnported` (no notices printed on that path).

- **GIVEN** `tu h --by-machine` in single mode
- **WHEN** `Run` executes
- **THEN** it returns `ErrUnported` without fetching

#### R17: Run — history composition and Result
`Run` SHALL (intake §9.3): normalize; fetch daily only as V2; for `Snapshot` keep V2's path and add the `CSV`/`Markdown` branches; for `History` compute `RollUp(Window(recs, Since, Until), Period)`, build registry-ordered `[]view.Series` from ONE `query.GroupBy(recs, query.Tool, query.Date)` pass (all six tools, or the one source), and render by source count and format through `view.History`/`view.TotalHistory`, `json.History`/`json.TotalHistory`, `csv.*`, `markdown.*` with `HistoryOptions{Period, Now: deps.Now(), Width: deps.Width, CapActive, Metric}` and nil `Prev`. `Result` SHALL gain `Notices []string`; `Deps` SHALL gain `Width int`.

- **GIVEN** a fake `Fetcher` returning the placeholder records, `Now` = 2026-01-06T12:00:00Z, `Width 80`, request `cc h --since 2026-01-01 --until 2026-01-31`
- **WHEN** `Run` executes
- **THEN** `Lines` equal the R4 golden and `Notices` is empty

#### R18: Result stats for history
For history requests `Result.TotalCost`/`TotalTokens` SHALL sum every entry and `CostByItem` SHALL carry `{Name}:{label}` per entry plus `total:{label}` per label, valued in the display metric (intake §9.3, the TS `buildCostMap`).

- **GIVEN** the R17 run
- **WHEN** `Result` is inspected
- **THEN** `TotalCost == 1.5`, `CostByItem["Claude Code:2026-01-05"] == 0.5`, `CostByItem["total:2026-01-05"] == 0.5`

### Edge: cmd/tu

#### R19: Terminal width probe
`cmd/tu` SHALL compute `Deps.Width` as the stdout TTY width via `golang.org/x/term` (`IsTerminal` + `GetSize` on the `*os.File` descriptor) and **80** otherwise (a `bytes.Buffer`, a pipe, or a probe error); `COLUMNS` MUST NOT be read. `go.mod` gains `require golang.org/x/term` and `go.sum` is committed.

- **GIVEN** the e2e test's `bytes.Buffer` stdout
- **WHEN** `run` executes a history command
- **THEN** the width used is 80 (no bars)

#### R20: Write order
After `Run` succeeds the edge SHALL write `Result.Notices` (one `Fprintln(stderr)` each), then `source.WriteWarnings(stderr, res.Warnings)`, then `Lines` on stdout, exit 0. Everything before `Run` is unchanged (B1 order).

- **GIVEN** `tu --since 2026-13-01` with the fake ccusage
- **WHEN** `run` executes
- **THEN** stderr is exactly `Warning: --since/--until apply to history display — ignoring.\n`, stdout is the empty snapshot, exit 0

#### R21: Placeholder list narrowed
`main_test.go`'s placeholder cases SHALL drop `{"m","dh","--json"}` and `{"--csv"}` and add `{"h","--by-machine"}`; `{"--help"}` stays.

- **GIVEN** `just go-test`
- **WHEN** run
- **THEN** `TestRunNotImplemented` passes with the new list

### Verification

#### R22: Tests and goldens
The change SHALL carry the tests of intake §11: table-driven `query`/`view`/`command` tests; `-update`-flag goldens for `render/ansi` (populated 80-col color and no-color for both tables, wide single-zone bars, two-zone with a zero row, stacked segments + legend at width 120 and its no-color twin, omission + dim zeros, token mode, separators + weekend + current marker, single-row, empty), `render/csv`, `render/markdown`, `render/json`; every colored ansi golden equal to its no-color twin under `StripANSI`; fake-`Fetcher` `Run` tests; e2e cases through the fake ccusage on `_placeholder`. `just go-lint` and `just go-test` MUST be clean.

- **GIVEN** `cd src/go && go test ./... -count=1` and `just go-lint`
- **WHEN** run
- **THEN** both exit 0

#### R23: Harness gate
`just go-diff --placeholder` SHALL report every one of the 39 case IDs in intake §2 `GREEN` and a summary of `134 green`; the four V2-green snapshot groups and B1's cases MUST stay green; `h-by-machine`, `cc-h-by-machine` and every multi/org/legacy/envrepo case stay red.

- **GIVEN** the finished implementation
- **WHEN** `env -u TU_METRICS_REPO -u NO_COLOR just go-diff --placeholder` runs
- **THEN** the summary line reads `tudiff: 368 cases — 134 green, 234 red, 0 timeout`

### Non-Goals

- Multi-mode records, `-u`, the metrics-dir guard — B3
- `--by-machine` in any form, machine columns/legend, the pivot by-machine warning — B4
- `lb`/`lbh`, `--top`, the `lbh` pivot hooks, leaderboard encoder kinds — B5
- Sync — B6; watch (`Prev` maps filled, `maxRows`, compact layouts) — B7; toolkit commands — B8
- Any edit to `src/node/**`, `docs/specs/*`, `justfile`, `.github/workflows/*`, `harness/**`, `Formula/`, `package.json`, or the plan document

### Design Decisions

#### View emits raw glyphs plus geometry; ansi only colors and pads
**Decision**: `view.Bar` carries the raw block-glyph strings (`Main`, `Overflow`) and per-tool `Segments`; `render/ansi` slices and colors them and adds padding and the `┊` rule.
**Why**: The strip-ANSI-equals-no-color invariant becomes structural — the colored bar is the raw bar with escapes inserted at slice boundaries — and the width budget stays in the pure layer the plan assigns bar scales to.
**Rejected**: Computing glyphs inside `render/ansi` from numeric values (duplicates the scale math in the encoder and makes the no-color twin a test assertion instead of a construction).
*Introduced by*: 260916-9ax5-history-and-periods

#### csv.Cost is a third rounding rule on big.Rat
**Decision**: `render/csv.Cost` rounds the exact binary value half-up to two decimals via `math/big`.
**Why**: JS `toFixed(2)` is defined on the exact value with ties to the larger n; `FormatFloat('f', 2)` is half-even on the exact value and `render.FormatCost` is ICU's shortest-repr half-away-from-zero — both diverge on node-verified inputs.
**Rejected**: Reusing `render.FormatCost` and stripping the `$` (wrong on `1.005`, `0.015`, `1.045`); `FormatFloat('f', 2)` (wrong on `0.125`, `0.375`).
*Introduced by*: 260916-9ax5-history-and-periods

#### Flag guards are a pure Normalize step whose notices ride the Result
**Decision**: `command.Normalize` clears/defaults flags and returns notice lines; `cmd/tu` prints them before the source warnings.
**Why**: Nothing below `cmd/tu` may write; the TS prints these before any fetch, so ordering notices ahead of `WriteWarnings` reproduces the stderr byte order; `since-invalid` must be in scope after the clear.
**Rejected**: Printing from `command` (breaks the only-writer rule); treating the guarded flags as out of scope (turns a warn-and-render exit 0 into the placeholder's exit 1).
*Introduced by*: 260916-9ax5-history-and-periods

#### Terminal width via golang.org/x/term at the edge
**Decision**: `cmd/tu` probes stdout with `x/term` and hands `Deps.Width` (80 when not a TTY) down; render never probes.
**Why**: DC-12 fixes the 80-column pipe default and forbids `COLUMNS`; plan D12 builds watch mode on `x/term` and the six sibling tools depend on it; a stdlib ioctl needs per-OS build tags for five lines.
**Rejected**: Hardcoding 80 until B7 (bars invisible in a real terminal during dogfood prep); a stdlib `TIOCGWINSZ` probe (build-tag surface for no gain).
*Introduced by*: 260916-9ax5-history-and-periods

## Tasks

### Phase 1: Setup

- [x] T001 Add `golang.org/x/term` to `src/go/go.mod` (`cd src/go && go get golang.org/x/term@latest && go mod tidy`); commit `go.sum`. <!-- R19 -->
- [x] T002 [P] Add `ThreeMonthFloor(now time.Time) string` to `src/go/internal/query/period.go` with table-driven cases (two zones, year rollover) in `period_test.go`; add a `Window`-then-`RollUp` weekly partial-week case to `query_test.go`. <!-- R1, R2 -->

### Phase 2: Core Implementation

- [x] T003 Extend the model in `src/go/internal/view/table.go` per intake §4.1 (`LabelStyle`, `Cell.Style`, `Delta`, `Bar`, `Scale`, `Swatch`, `Separator` row kind, `Table.Footer`/`Legend`/`Scale`/`DeltaSpaced`); keep `Snapshot` output identical; run `go test ./internal/render/ansi/` to confirm the V2 goldens are unchanged. <!-- R3 -->
- [x] T004 [P] Create `src/go/internal/view/bar.go` with `Glyphs`, `Percentile`, `ComputeScale`, `Apportion` and the per-row bar builder (intake §4.6); table-driven `bar_test.go` covering the R7 scenarios, the `▏` floor, the `eighths == 8` carry, and the single-element percentile. <!-- R7 -->
- [x] T005 [P] Create `src/go/internal/view/footer.go` (footer text per intake §4.5) plus the shared helpers `fmtMetric`, `metricValue`, `metricColumnWidth`, `metricCell`, `labelStyle`, `isWeekend` (a `metric.go` or inside `history.go`); `footer_test.go` covers `this month` present/absent, all-zero peak, unit suffixes, the p95 term. <!-- R6, R8, R9 -->
- [x] T006 Create `src/go/internal/view/history.go` with `Entry`, `Series`, `Metric`, `HistoryOptions`, and `History(...)` per intake §4.3; `history_test.go` covers R4's scenarios, widths (floor 9 and growth), separators daily-only, label styles, token mode, `Prev` deltas, bar budget at 80 vs 140. <!-- R4, R8, R9 -->
- [x] T007 Create `src/go/internal/view/pivot.go` with `TotalHistory(...)` per intake §4.4 (significance + fallbacks, widths, `indicatorReserve`, stacked segments, legend); `pivot_test.go` covers R5's scenarios, boundary values kept, the `$0.40` window fallback, the all-zero fallback, legend conditions, `rowValue` over omitted tools. <!-- R5, R8, R9 -->
- [x] T008 Extend `src/go/internal/render/ansi/table.go` per intake §5.1 (styled label cells, dim cells, separators with `barDiv`, deltas, bars with palette runs and the two-zone empty-wrap behavior, footer + gated legend); add the R22 history/pivot goldens under `testdata/` via the existing `-update` flag and assert `StripANSI(colored) == nocolor` for each; verify the `h-window` and `cc-h-window` goldens match intake §12 byte for byte. <!-- R10, R22 -->
- [x] T009 [P] Create `src/go/internal/render/csv/` (`csv.go`: `Snapshot`, `History`, `TotalHistory`, `Cost`, `quote`) per intake §6; `csv_test.go` with the R12 rounding table, goldens for the three kinds populated and empty, and a quoting case. <!-- R11, R12 -->
- [x] T010 [P] Create `src/go/internal/render/markdown/` (`markdown.go`: `Snapshot`, `History`, `TotalHistory`) per intake §7; `markdown_test.go` goldens for populated (`cc-mh-md` bytes), Total row, empty (`h-md` bytes), exact-zero omission + all-six fallback, cap hint. <!-- R13 -->
- [x] T011 [P] Add `src/go/internal/render/json/history.go` (`History`, `TotalHistory` nested ordered writer) per intake §8; goldens `history_populated` (`cc-mh-json` bytes), `history_empty`, `total_history_mixed`, `total_history_all_empty` (`h-json` bytes). <!-- R14 -->
- [x] T012 Create `src/go/internal/command/guards.go` with `Normalize(req, now)` per intake §9.1; `guards_test.go` covers each notice text, the clear semantics, and the eight period/since/until/full cap combinations. <!-- R15 -->
- [x] T013 Update `src/go/internal/command/request.go` (`Result.Notices`, `Deps.Width`) and `run.go` (normalize → scope on the normalized request → V2 snapshot path + CSV/Markdown branches → history composition via one `GroupBy(Tool, Date)` pass → renderers → stats/`CostByItem`); extend `run_test.go` with the fake `Fetcher` at `Now` = 2026-01-06T12:00:00Z and `Width` 80/120: R17/R18 scenarios, all-tools pivot, window-before-roll-up, JSON/CSV/Markdown lines, notices, `ErrUnported` for `--by-machine`/`-u`/multi mode. <!-- R16, R17, R18 -->
- [x] T014 Update `src/go/cmd/tu/main.go`: `terminalWidth(stdout)` via `x/term` (80 fallback), `Deps.Width`, and the notices → warnings → lines write order; update `main_test.go`'s placeholder list per R21. <!-- R19, R20, R21 -->

### Phase 3: Integration & Edge Cases

- [x] T015 Extend `src/go/cmd/tu/e2e_test.go` with the intake §11 e2e cases: `cc h --since 2026-01-01 --until 2026-01-31` (populated bytes), `h` (capped heading + `  No data`), `mh` (one row), `wh --since … --until …` (`2026-01-04`), `h --json`, `cc mh --csv`, `cc mh --md`, `--csv`/`--md` snapshot empties, `--since 2026-13-01` (warning + empty snapshot, exit 0), `h --by-machine` (placeholder). <!-- R20, R22 -->
- [x] T016 Run `just go-lint`, `just go-test`, then `env -u TU_METRICS_REPO -u NO_COLOR just go-diff --placeholder`; for every red case among the 39 gate IDs, diff `bin/harness/report/cases/<id>/node.stdout` against `go.stdout` (and stderr) and fix the Go side until the summary reads `134 green`; record the final summary line and the 39 IDs' status in `## Notes`. <!-- R23 -->

### Phase 4: Polish

- [x] T017 Package doc comments for `view` (history additions), `render/csv`, `render/markdown`, `render/json` (history) and `command/guards.go` state the layering rules (no I/O below `cmd/tu`, no ANSI in view, the three rounding rules); `gofmt -l src/go` empty; `go vet ./...` clean. <!-- R22 -->

## Execution Order

- T001 before T014 (the `x/term` import); T002 before T012/T013 (`ThreeMonthFloor`).
- T003 before T004–T008 (the model types); T004 and T005 before T006/T007; T006/T007 before T008.
- T009, T010, T011 are independent of each other and of T006–T008 but need T003's `Series`/`Entry` (define them in T003 or T006 — whichever lands first — and keep them in `view`).
- T012 before T013; T013 before T014; T014 before T015; T015 before T016; T017 last.

## Acceptance

### Functional Completeness

- [x] A-001 R1: `ThreeMonthFloor` returns `2026-07-01` for 2026-09-16 and `2025-11-01` for 2026-01-15 in both `UTC` and `Asia/Kolkata`
- [x] A-002 R2: the history path windows daily records before rolling up; the weekly window test yields the leading Sunday `2026-01-04`
- [x] A-003 R3: the model carries the typed slots, no escape sequence exists in `internal/view`, and the four V2 ansi snapshot goldens are byte-unchanged
- [x] A-004 R4: `view.History` reproduces the single-tool rules (widths, separators, styles, Total/footer gating, bars, deltas)
- [x] A-005 R5: `view.TotalHistory` reproduces the pivot rules (significance + fallbacks, widths, `indicatorReserve`, segments, legend)
- [x] A-006 R6: the footer text matches every term rule including the parenthesis-free all-zero peak
- [x] A-007 R7: `Glyphs`, `Percentile`, `ComputeScale`, `Apportion` match the TS semantics on the table-driven cases
- [x] A-008 R8: label styles and exact-zero dimming follow the precedence rules (Current over Weekend; `Dim` only on exact 0)
- [x] A-009 R9: token mode swaps unit, header, title and thresholds; cost mode is byte-identical to a metric-less render
- [x] A-010 R10: `ansi.Table` encodes styles, separators, deltas, bars, footer and legend; two-zone empty wraps present
- [x] A-011 R11: the three CSV kinds emit the pinned headers, row rules, quoting, and Total-row rules
- [x] A-012 R12: `csv.Cost` passes the thirteen node-verified values
- [x] A-013 R13: the three Markdown kinds emit the pinned layout, titles, bold Total, exact-zero pivot omission, trailing blank line
- [x] A-014 R14: `json.History`/`json.TotalHistory` emit the pinned nested layout with `[]` inline for empty
- [x] A-015 R15: `Normalize` emits the two notice texts, clears since/until on snapshots, and applies the cap only under the stated predicate
- [x] A-016 R16: `inScope` accepts the widened set on the normalized request and rejects every B3–B8 surface with `ErrUnported`
- [x] A-017 R17: `Run` composes the history pipeline through one `GroupBy(Tool, Date)` pass and renders by source count and format
- [x] A-018 R18: history `Result` stats and `CostByItem` keys match the TS `buildCostMap` scheme
- [x] A-019 R19: `Deps.Width` is the TTY width or 80; `COLUMNS` is never read; `go.mod`/`go.sum` carry `x/term`
- [x] A-020 R20: notices precede source warnings which precede stdout lines; exit 0
- [x] A-021 R21: `TestRunNotImplemented` list updated as specified

### Behavioral Correctness

- [x] A-022 R2: `tu wh --since 2026-01-01 --until 2026-01-31` renders one row labeled `2026-01-04`
- [x] A-023 R15: `tu --since 2026-13-01` warns on stderr and still renders the empty snapshot with exit 0 (harness `since-invalid` green)
- [x] A-024 R10: `tu h --since 2026-01-01 --until 2026-01-31` and `tu cc h --since … --until …` match intake §12 byte for byte at 80 columns

### Scenario Coverage

- [x] A-025 R4: a one-entry history (`cc mh`) renders no divider, Total or footer
- [x] A-026 R15: bare `tu h` renders `📊 Combined Cost History (daily, last 3 months)` + `  No data` under the placeholder corpus
- [x] A-027 R14: `tu h --json` under the placeholder corpus is six inline empty arrays in registry order
- [x] A-028 R11: `tu h --csv` under the placeholder corpus is the header line alone

### Edge Cases & Error Handling

- [x] A-029 R7: a row at exactly p95 has an empty overflow zone; a zero row in a two-zone window still renders the rule and both padded zones
- [x] A-030 R5: a window where every tool is negligible falls back to the nonzero set, then to all six
- [x] A-031 R8: a malformed label is never styled `Weekend` and becomes its own bucket (existing `WeekLabel` passthrough)
- [x] A-032 R16: `tu h --by-machine`, `tu h -u x`, and multi-mode history keep the placeholder (exit 1)

### Code Quality

- [x] A-033 Pattern consistency: new packages follow V2's shapes (pure functions, `[]string` encoders, `-update` goldens, table-driven tests)
- [x] A-034 No unnecessary duplication: `render.FormatInt`/`FormatCost`, `ansi.Colors`, `query.Window`/`RollUp`/`GroupBy`, `json.encodeString`/`encodeFloat` are reused, not reimplemented
- [x] A-035 Readability over cleverness: no function in `view` exceeds the codebase's typical size without a clear reason (the two table builders may be long but MUST be sectioned by comments mirroring the intake's rule lists)
- [x] A-036 Minimum pathways: one metric-generic render path per table (no separate token renderer); one `Normalize` for the guards
- [x] A-037 No magic numbers: widths, thresholds, palette slots and the `1.5`/`0.001`/`4` constants are named
- [x] A-038 No swallowed errors: the width probe falls back to 80 explicitly; `csv.Cost` handles `-0`/`0`
- [x] A-039 Layering: `query`, `view`, `render/*` import no `os`, `fmt.Print*`, `io` writers, or the `ccusage` package; only `cmd/tu` writes

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`
- The harness needs `npm ci` once in a fresh worktree; run the harness with `env -u TU_METRICS_REPO -u NO_COLOR` so an exported shell variable cannot tilt a case.
- **Apply run (2026-09-16)**: `tudiff: 368 cases — 134 green, 234 red, 0 timeout`; all 39 gate IDs GREEN (7 base + 14 `-window` fixed/alt + 7 `-full` + 6 json/csv/md + 4 snapshot csv/md + `since-invalid`); `h-by-machine`/`cc-h-by-machine` and every multi/org/legacy/envrepo case stay red. `just go-lint`/`just go-test` clean.
- **Intake discrepancy noted at apply**: R7's GIVEN values `[846.21, 1091.67, 4031.61, 172.13]` are single-zone under the §4.6 formula (p95 = 3590.62, 1.5×p95 = 5385.93 > max 4031.61 — node-verified); the THEN (OverflowZone 8 / MainZone 21, empty overflow at exactly p95) matches the TS test's canonical 23-value outlier sample, which `bar_test.go` uses instead. The formula as pinned by §4.6 and the TS tests is implemented verbatim.
- `go.mod` went to `go 1.26.0` because `golang.org/x/term v0.46.0` requires it (`go get` raised the directive; CI pins the toolchain via `go-version-file`).

## Deletion Candidates

- None — this change adds new functionality without making existing code redundant. (The only removals are placeholder-case entries inside `main_test.go`'s `TestRunNotImplemented`, updated in place per R21.)

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | `Series`/`Entry` live in `view` (not `query`) as the render-side input types | view already owns `ToolTotals`; keeps `query` about records and groups | S:55 R:90 A:85 D:70 |
| 2 | Confident | Exact struct field names may deviate from the intake sketch when Go idiom or `gofmt` alignment favors it, provided the information content is identical | The intake states the model as a sketch; the tests pin behavior, not names | S:50 R:95 A:85 D:75 |
| 3 | Certain | Bar goldens use injected widths 80/120/140; no golden depends on a real terminal | Bars are unobservable in the harness; injected width is the only deterministic path | S:80 R:90 A:95 D:90 |
| 4 | Confident | `x/term` pinned to the latest stable at apply time; `go mod tidy` decides `x/sys` | Standard toolchain practice; no version constraint in the plan | S:45 R:95 A:85 D:80 |

4 assumptions (1 certain, 3 confident, 0 tentative).
