# Intake: History Displays, Windows, Bars and the CSV/Markdown Encoders (Go port row B2)

**Change**: 260916-9ax5-history-and-periods
**Created**: 2026-09-16

## Origin

One-shot `/fab-new` invocation, handed over from the Go-port plan's queue (plan row B2, the fourth Phase 1 row, after V1 #84, V2 #85 and B1 #86 merged):

> Context: fab/plans/sahil/26-09-15-go-port.md, row B2. Read the Decisions and Target architecture sections before writing the intake. Do not change any external surface listed under Goal. Build dh/wh/mh history displays, weekly Sunday alignment, --since/--until, deltas, bars, bar scale/percentile, render/csv, render/markdown. Harness gate: all history cases, single mode.

No prior discussion in this conversation. Sources read to ground every value below: the plan's Decisions (D1–D13), Target architecture table, § G1 review protocol and § Risks; the Go-port memories `docs/memory/go-port/query-view-render.md` and `docs/memory/go-port/command-edge.md` and the code they describe (`src/go/internal/{query,view,render,render/ansi,render/json,command}`, `src/go/cmd/tu`); the TS memories `docs/memory/display/formatting.md` (history bullets, bar/p95/stacked/footer/omission/data-sizing rules, CSV and Markdown emitters) and `docs/memory/cli/data-pipeline.md` (`--since`/`--until`, the implicit 3-month cap, `--full`, weekly aggregation); the specs `docs/specs/usage.md` (§ Periods, § Display, § Global Flags, § Data Flow › Snapshot vs History, § Output Formats — history tables, table semantics, CSV, Markdown, JSON — and § Drop at cutover DC-03/05/06/10/12) and `docs/specs/layouts.md` (§3, §4, §15, §16, §21, Color Reference); the harness memory and `harness/matrix.json`; and the shipped TypeScript — `src/node/tui/formatter.ts` (`renderHistory`, `renderTotalHistory`, `renderBar`, `percentile`, `computeBarScale`, `renderScaledBar`, `apportionSegments`, `renderStackedScaledBar`, `renderHistoryFooter`, `significantTools`, `nonzeroTools`, `metricCell`, `metricColumnWidth`, `emitCsv*`, `emitMarkdown*`), `src/node/tui/colors.ts`, `src/node/core/cli.ts` (`main()` guard block, `capApplies`, `threeMonthFloor`, `dispatchAllHistory`, `dispatchSingleTool`, `renderHistoryByFormat`, `renderTotalHistoryByFormat`, `emitJson`, `buildCostMap`), `src/node/core/fetcher.ts` (`filterEntriesByRange`, `aggregateWeekly`, `aggregateMonthly`, `aggregateForPeriod`). A full `just go-diff --placeholder` run on this worktree (2026-09-16: 95 green of 368) supplied the node-side reference bytes quoted in §12 from `bin/harness/report/cases/`, and node v24 was probed directly for the CSV rounding rule and the nested JSON layout.

Plan context that shapes this change:

- **D2** — Go lands dark; the formula is untouched. B2 makes the Go binary answer the single-mode **history** grammar (`h`/`history`/`dh`/`wh`/`mh`), the `--since`/`--until`/`--full` flags, and the `--csv`/`--md` formats for real. Everything else keeps the placeholder.
- **Target architecture** — history is `RollUp` + `Window` + `GroupBy(Date…)` in `query`; `view` shapes the two history tables including bar scales, deltas and the legend (no ANSI); `render/ansi` gains bars, `render/csv` and `render/markdown` are new encoders; `render/json` gains the two history shapes; `command` composes and returns a `Result`; `cmd/tu` stays the only writer.
- **D6** — the harness is the gate. §2 enumerates the 39 single-mode case IDs that flip green. The placeholder corpus (dates 2026-01-05..07, equal `0.5` cost per tool per day) exercises the **populated** history path through the `--since 2026-01-01 --until 2026-01-31`, `--full` and `mh` cases — unlike V2's snapshot, which only ever saw the empty state.
- **G1** runs after this row and reviews V1+V2+B1+B2 as a unit: package boundaries (`query`/`view`/`render` import no I/O), one group-by, no result globals, typed errors at the edge, table-driven `query`/`command` tests and golden-file `render` tests. This intake records the layering choices for that review, in particular where the terminal width and the flag-guard warnings enter the pipeline.
- **Goal** — every external surface is frozen. B2 reproduces byte for byte: both history tables (with bars where the width allows), the `Warning:` guard lines, the 3-month cap and its heading hint, CSV and Markdown for the snapshot and both history kinds, and the two history JSON shapes.

## Why

V2 proved the layering on the narrowest display. B2 is where the layering earns its keep: the history tables are the largest byte-exact surface in tu (the plan's § Risks names the 1842-line formatter as the single largest parity risk), and they exercise every reserved slot V2 left in the model — `Row.Bar`, `Row.Delta`, `Table.Footer`, `Table.Legend`, `Cell.Dim` — plus the window filter and the weekly/monthly roll-up that `query` already ships but nothing consumes yet. The TS renders these tables through two 150–200-line functions that interleave data shaping, width budgeting, ANSI styling and bar drawing; the port separates them (view computes widths, scale and raw glyphs; ansi colors and pads) so that "strip the ANSI and you get the no-color bytes" is a structural invariant rather than a test assertion.

Doing history, the date window, the encoders and the bars in one row (rather than table-then-formats) is what makes the gate meaningful: 39 harness cases cover the pivot, the single-tool table, weekly Sunday labels, the 3-month cap heading, the explicit window under two time zones, `--full`, the empty state, and the JSON/CSV/Markdown shapes of all three display kinds. Splitting would leave every history case red until the second half landed. Bars cannot be observed by the harness at all — piped output is 80 columns and neither table fits a bar there — so they are pinned by width-injected golden files, which is exactly the G1 checklist's item 5.

## What Changes

### 1. Package layout (deltas on the V2/B1 tree)

```
src/go/
  go.mod                        + require golang.org/x/term (first module dependency; go.sum appears)
  cmd/tu/
    main.go                     terminal width probe → Deps.Width; writes Result.Notices before the source warnings
    main_test.go                placeholder list narrowed ({"--csv"} and {"m","dh","--json"} now succeed)
    e2e_test.go                 + populated history, csv/md, cap heading, since-warning cases
  internal/
    query/
      period.go                 + ThreeMonthFloor(now)
      query.go                  unchanged (Window, RollUp, GroupBy already exist)
    view/
      table.go                  Row.Bar / Row.Delta / Table.Footer / Table.Legend become typed (no ANSI)
      bar.go                    NEW: Bar geometry — Glyphs, Scale (single/two-zone), Percentile, Apportion
      history.go                NEW: Series/Entry input types, History(...) single-tool table
      pivot.go                  NEW: TotalHistory(...) cross-tool pivot (significance, widths, stacked segments)
      footer.go                 NEW: the summary footer text (avg / this month / peak / p95)
      *_test.go                 table-driven
    render/
      format.go                 unchanged (FormatInt, FormatCost)
      ansi/
        color.go                unchanged palette; + palette lookup for stacked segments
        table.go                + bar/delta/footer/legend encoding, month separators, styled label cells
        *_test.go, testdata/    + history goldens (80-col, wide, two-zone, stacked, token mode, empty, no-color twins)
      csv/                      NEW: Snapshot, History, TotalHistory encoders; Cost (exact-value half-up)
      markdown/                 NEW: Snapshot, History, TotalHistory encoders
      json/
        history.go              NEW: History(entries), TotalHistory(series) — nested ordered writer
    command/
      request.go                Result gains Notices []string; Deps gains Width int
      guards.go                 NEW: the flag warn-and-clear guards and the 3-month cap (pure)
      run.go                    scope widened to History + CSV/Markdown + Since/Until/Full; history composition
      *_test.go                 + history Run tests with a fake Fetcher, fixed Now and Width
```

No edit to `src/node/**`, `docs/specs/*`, `justfile`, `.github/workflows/*`, `harness/**`, `Formula/`, `package.json`, or the plan document. `just go-build`/`go-test`/`go-lint` and the `go-build-and-test` CI lane pick the new packages up through `./...`. The CI lane already sets `cache: false` on `setup-go` "until a `go.sum` exists" — the file appearing needs no workflow edit (turning the cache on is a later tidy-up, not this row).

### 2. Scope and the harness gate

**In scope (produces real output)** — a data command in **single mode** whose parsed `Request` has `Display ∈ {Snapshot, History}`, `Format ∈ {Table, JSON, CSV, Markdown}`, any `Source`, any `Period`, and flags limited to `--json`/`-j`, `--csv`, `--md`, `--fresh`/`-f`, `--no-color`, `-t`, `--metric`, `--since`/`-s`, `--until`, `--full`. The V2 snapshot path is unchanged except for the two new encoders and the guards in §9.

**Still recognized-but-unported (placeholder, exit 1)**: multi mode and `-u` (B3); `--by-machine` (B4 — including its warn-and-clear on the all-tools pivot, `Warning: --by-machine is not supported with all-tools history — ignoring.`, which B4 slots into §9's guard order ahead of the since/until guard); `lb`/`lbh`/`--top` (B5); `sync`/`--sync`/`--dry-run` (B6); `--watch`/`-w`/`--interval`/`--no-rain`, `maxRows` truncation and compact layouts (B7); the non-data commands (B8/B1 as before).

**The gate.** These **39** expanded IDs from `just go-diff --placeholder` MUST be green (all currently red with `exit: node=0 go=1`):

```
h, dh, wh, mh, history, cc-h, cc-mh                       /single/default/pipe/fixed        (7)
{h,dh,wh,mh,history,cc-h,cc-mh}-window                    /single/default/pipe/{fixed,alt}  (14)
{h,dh,wh,mh,history,cc-h,cc-mh}-full                      /single/default/pipe/fixed        (7)
h-json, h-csv, h-md, cc-mh-json, cc-mh-csv, cc-mh-md      /single/default/pipe/fixed        (6)
snapshot-all-csv, snapshot-all-md, snapshot-cc-csv, snapshot-cc-md   /single/default/pipe/fixed  (4)
since-invalid                                             /single/default/pipe/fixed        (1)
```

`since-invalid` (`tu --since 2026-13-01`) is a snapshot with a well-shaped impossible date: the TS warns `Warning: --since/--until apply to history display — ignoring.` on stderr and renders the empty snapshot with exit 0 — the warn-and-render contract V2's memory reserved for B2. Expected summary after B2: **134 green of 368** (95 + 39). `h-by-machine`/`cc-h-by-machine` stay red for B4; every multi/org/legacy/envrepo case stays red for B3.

What the placeholder corpus exercises: the `-window` and `-full` cases render **populated** tables (three days, six tools at `$0.50` each, `$3.00` rows, `$9.00` total; single-tool `3,000 / 400 / 1,000 / 20,000 / 24,400` per day); `mh`/`cc-mh` render a **one-row** window (no divider, Total or footer); `wh-window` yields the single Sunday label `2026-01-04`; bare `h`/`dh`/`wh`/`cc-h` are **empty and capped** (`(daily, last 3 months)` + `  No data`); `h-json` is six empty arrays; `h-csv` is a header line. Not exercised by the harness and therefore pinned by goldens: bars (80 columns never leaves room), negligible-column omission and dim zero cells (every placeholder tool has equal nonzero cost), month separators (all January), the current-period marker and weekend dimming (no fixture date is today or a weekend), token mode on history, the two-zone scale, the stacked legend, and the `this month` footer term. On dev-ws-sahil02 the local capture (`just go-diff` without `--placeholder`) exercises omission, dim zeros, separators and weekends against real bytes.

### 3. `query` — one addition

```go
// ThreeMonthFloor is the implicit history cap's floor: the first day of the
// local month two calendar months back, so the window spans three calendar
// months including the current one (2026-09-16 → "2026-07-01"; 2026-01-15 →
// "2025-11-01" — time.Date normalizes the month underflow). Mirrors the TS
// threeMonthFloor; the caller passes deps.Now() so the harness tz axis holds.
func ThreeMonthFloor(now time.Time) string
```

Everything else the history path needs already exists: `Window(recs, since, until)` (inclusive, lexicographic, empty bound open), `RollUp(recs, p)` (weekly → `WeekLabel` Sunday, monthly → `Date[:7]`, sums `Totals`, ascending by `Date`), `GroupBy(recs, dims...)` (first-seen order), `CurrentLabel(p, now)`. **Order of operations is load-bearing**: history applies `Window` to the **daily** records first and `RollUp` second — `RollUp(Window(recs, since, until), p)` — so a partial month sums only in-window days and a window that starts mid-week yields a leading partial week labeled by its Sunday (`wh --since 2026-01-01 --until 2026-01-31` → `2026-01-04`, which precedes `since`). V2's snapshot composes the other way round (`Window(RollUp(recs, p), cur, cur)`) because its window is a single period label; both stay as they are.

### 4. `view` — the history models (no ANSI)

#### 4.1 Model changes to `table.go`

The reserved V2 slots become typed. Nothing here carries an escape sequence; the encoder decides colors.

```go
type LabelStyle int            // Plain, Current (boldWhite: label == CurrentLabel), Weekend (dim: daily Sat/Sun)
type Cell struct { Text string; Dim bool; Style LabelStyle }   // Style is used only by the first (Date) cell

type Delta int                 // 0 none, +1 up (green ↑), −1 down (red ↓); B7 feeds the prev map, B2 wires the seam
type Row struct {
    Kind      RowKind          // Header, Divider, Data, Total — plus Separator (the dim month-boundary divider)
    Cells     []Cell
    Bar       *Bar             // nil when bars are not shown or the row is not Data
    Delta     Delta
}

// Bar is one row's bar, already reduced to raw glyphs and geometry.
type Bar struct {
    Main       string          // raw block glyphs for the main zone (or the whole bar in single-zone mode)
    Overflow   string          // raw glyphs past the p95 rule; "" unless two-zone and value > p95
    Segments   []int           // per-visible-tool glyph counts over Main (largest remainder); nil = solid (single-tool history)
}
type Scale struct {
    TwoZone      bool
    Max, P95     float64
    Width        int           // barWidth; 0 = bars suppressed
    MainZone     int           // barWidth − OverflowZone − 1 (two-zone only)
    OverflowZone int           // max(4, round(barWidth / 4)) (two-zone only)
}
type Swatch struct { Name string; Palette int }   // legend entry: tool name + palette slot (0 green, 1 magenta, 2 blue, 3 cyan, ≥4 uncolored)

type Table struct {
    Title      string
    Columns    []Column        // Date + numeric columns (+ per-tool columns in the pivot), widths already data-sized
    Rows       []Row
    Empty      string          // "  No usage" (snapshot) or "  No data" (history)
    Footer     string          // dim summary line text: "avg $3.00/day · peak $3.00 (2026-01-05)"; "" when absent
    Legend     []Swatch        // stacked-bar legend; nil unless bars shown ∧ ≥2 visible tools (ansi drops it when color is off)
    Scale      Scale           // bar geometry shared by every row; Width 0 when no bars
    DeltaSpaced bool           // true: " ↑" (single-tool history); false: "↑" abutting the cell (pivot)
}
```

`Snapshot` keeps producing exactly what it produces today (no `Style`, no `Bar`, `Scale.Width` 0); the V2 ansi goldens must not change.

#### 4.2 Shared inputs

```go
// Entry is one history row candidate: an ISO label and its totals.
type Entry struct { Label string; fact.Totals }
// Series is one tool's history: display name (command resolves it — view knows no registry) and its entries ascending by label.
type Series struct { Name string; Entries []Entry }

type Metric int  // Cost, Tokens — the unit cells, bars and footer render in (the TS BarMetric)

type HistoryOptions struct {
    Period    query.Period
    Now       time.Time            // for CurrentLabel(period) and the footer's "this month" prefix
    Width     int                  // terminal width budget (80 when stdout is not a TTY)
    CapActive bool                 // append ", last 3 months" to the heading
    Metric    Metric
    Prev      map[string]float64   // watch delta map, keyed "{Name}:{label}" / "total:{label}"; nil in B2 (B7 fills it)
}
func History(s Series, o HistoryOptions) Table            // single-tool table (renderHistory)
func TotalHistory(series []Series, o HistoryOptions) Table // cross-tool pivot (renderTotalHistory); series in registry order
```

Shared helpers (unexported unless tests need them): `fmtMetric(v, m)` = `render.FormatCost(v)` under cost, `render.FormatInt(int64(math.Round(v)))` under tokens; `metricValue(t, m)` = `TotalCost` or `float64(TotalTokens)`; `metricColumnWidth(values, m)` = `max(9, longest fmtMetric)`; `metricCell(v, w, m)` = `Cell{Text: fmtMetric(v,m), Dim: v == 0}` (exact zero only — a sub-cent value that formats as `$0.00` is not dimmed); `labelStyle(label, period, current)` = `Current` when `label == current`, else `Weekend` when `period == Daily` and the UTC-parsed date is Saturday/Sunday, else `Plain` (the marker wins on a weekend today); `isWeekend(label)` parses `2006-01-02` in UTC (a malformed label is never a weekend).

#### 4.3 `History` — the single-tool table (layouts §3)

- **Title** `📊 {Name} ({period}[, last 3 months])` — the tool name and the period word, unchanged by `Metric` (only the pivot's title changes under tokens). `periodLabel(p, cap)` is `"{p}, last 3 months"` when `CapActive`, else `p.String()`.
- **Empty**: no entries → `Empty = "  No data"`, no rows.
- **Columns**: `Date` (12, Left); `Input`, `Output`, `Cache Write`, `Cache Read`, `Total` (14, Right); last column `Cost` under cost / `Tokens` under tokens, Right, width `metricColumnWidth(every row's metric value + their sum)`, floor 9. `tableWidth = 12 + 5×14 + 5×3 = 97`.
- **Bar budget**: `barWidth = min(Width − 97 − 3 − costWidth − 1, 30)`; `showBars = barWidth ≥ 10` (with a 9-wide last column that needs `Width ≥ 120`; layouts §3 quotes 121 for a 10-wide column). `Scale` per §4.6 over the rows' metric values; when bars are suppressed `Scale.Width = 0` and no row has a `Bar`.
- **Rows**, in input order: `Data` rows with cells `[label, FormatInt(Input), FormatInt(Output), FormatInt(CacheCreation), FormatInt(CacheRead), FormatInt(TotalTokens), metricCell(value, costWidth)]`, `Cells[0].Style = labelStyle(...)`, `Delta` from `Prev["{Name}:{label}"]` (nil map → 0), `Bar` per §4.6 (solid; `Segments == nil`). A `Separator` row precedes any `Data` row whose `label[:7]` differs from the previous row's — **daily period only**, never before the first row.
- **Total** when `len(entries) > 1`: `Divider`, then `Total` with `["Total", the five FormatInt sums, fmtMetric(sumValue)]`, then `Footer` per §4.5. A one-row window has no divider, Total or footer (`cc-mh` in §12).
- `DeltaSpaced = true`.

#### 4.4 `TotalHistory` — the cross-tool pivot (layouts §4)

- **Title** `📊 Combined Cost History ({period}[, last 3 months])`, or `📊 Combined Token History (…)` under tokens.
- **Labels** = the sorted union of every series' labels (ascending byte order). None → `Empty = "  No data"`.
- **valueMap**: tool → label → `metricValue`. Missing = 0.
- **Visible tools** (`significantTools`): keep a series iff its total over the labels is `≥ negligibleAbs` (`1.0` under cost, `1000` under tokens) **and** `≥ 0.001 × grand`, where `grand` sums **every** series; boundary values kept. If nothing survives, fall back to `nonzeroTools` (series with any nonzero cell); if that is empty too, every series. Visible order = input (registry) order.
- **Row data**: per label, `rowValue` sums **all** series (omitted tools still count), `values[i]` per visible tool, `toolSums[i]` accumulated over visible tools, `grandTotal` over all.
- **Widths**: Date 10; per visible tool `max(len(Name), 9, longest fmtMetric among its cells, fmtMetric(toolSum))`; last column `metricColumnWidth(rowValues + grandTotal)`, floor 9. `tableWidth = 10 + Σ(toolWidth + 3)`. With six visible tools at `$0.50` cells: `10 + (11+9+9+9+9+9) + 6×3 = 84`.
- **Bar budget**: `barWidth = min(Width − tableWidth − 3 − costWidth − 1 − indicatorReserve, 30)` with `indicatorReserve = 1` when `Prev != nil`; `showBars = barWidth ≥ 10`. At 80 columns with the six-tool placeholder pivot: `80 − 84 − 3 − 9 − 1 = −17` → no bars (matches the §12 capture).
- **Rows**: header `["Date", visible names…, "Cost" or "Tokens"]`; `Data` rows `[label, metricCell(values[i], toolWidth[i])…, metricCell(rowValue, costWidth)]`, `Cells[0].Style`, `Delta` from `Prev["total:{label}"]`, `Bar` with `Segments = Apportion(values, len(Main))` (per §4.6). `Separator` rows as in §4.3 (daily only).
- **Total** when `len(labels) > 1`: `Divider`, `Total` `["Total", fmtMetric(toolSums)…, fmtMetric(grandTotal)]`, `Footer`; `Legend` = one `Swatch{Name, i}` per visible tool when `showBars ∧ len(visible) ≥ 2` (the encoder additionally requires color to be enabled).
- `DeltaSpaced = false` (the space-less `$128.13↑` form — the pivot's width contract).
- The `lbh` hooks (`columnOrder`, `highlightRowLeader`, `historyTitle`, `omitNegligibleColumns: false`) are **B5**; B2 builds the pivot with the tool defaults only, but keeps the visible-tool list and the row `values` as ordinary slices so B5 can reorder them without reshaping.

#### 4.5 Footer text (`footer.go`)

`footerText(labels, values, period, scale, now, metric) string` reproduces `renderHistoryFooter` minus the legend:

- `avg {fmtMetric(sum/len)}{unit}` with unit `/day`, `/week`, `/month` by period.
- Daily only: `this month {fmtMetric(sum of rows whose label has prefix CurrentLabel(Monthly, now))}` — omitted when no row matches (every placeholder case: January rows, September now).
- `peak {fmtMetric(max)} ({label})` where the max is the **first strict maximum** (`v > peak` starting from `peak = 0`); when every value is 0 no label is found and the term is `peak $0.00` with **no parentheses**.
- Two-zone only: `┊ = {fmtMetric(p95)} (p95)`.
- Parts joined by ` · ` (U+00B7 middle dot with spaces).

The encoder wraps the whole text in `Dim` and appends the legend after it (§5.2) — the legend swatches carry their own resets, which is why the legend is not part of the dim text.

#### 4.6 Bars (`bar.go`) — lifted from `renderBar`, `percentile`, `computeBarScale`, `apportionSegments`

```go
// Glyphs renders value against max over width cells as raw block glyphs:
//   "" when value == 0 or max == 0;
//   scaled = value/max × width; full = floor(scaled); eighths = round((scaled − full) × 8)  [JS Math.round: half up — identical for non-negatives];
//   eighths == 8 → "█" × (full+1); else "█" × full + EIGHTHS[eighths]; an empty result becomes "▏".
// EIGHTHS = ["", "▏", "▎", "▍", "▌", "▋", "▊", "▉"] (U+258F..U+2589); "█" is U+2588; the rule is "┊" U+250A.
func Glyphs(value, max float64, width int) string

// Percentile: linear interpolation over a sorted-ascending sample; idx = p/100 × (n−1); lo = floor, hi = ceil.
func Percentile(sortedAsc []float64, p float64) float64

// ComputeScale: max over values; nonzero values sorted ascending; none → single-zone;
// p95 = Percentile(nonzero, 95); if !(max > 1.5 × p95) → single-zone;
// else two-zone with OverflowZone = max(4, round(barWidth/4)) and MainZone = barWidth − OverflowZone − 1.
func ComputeScale(values []float64, barWidth int) Scale

// Apportion distributes total glyphs over shares by largest remainder: floor each quota, then hand the
// remainder to the largest fractional parts, ties to the earlier index; a zero share gets zero; sums exactly to total.
func Apportion(shares []float64, total int) []int
```

Per-row bar construction (`view` side): single-zone → `Main = Glyphs(v, Max, Width)`, `Overflow = ""`; two-zone → `Main = Glyphs(min(v, P95), P95, MainZone)`, `Overflow = Glyphs(v − P95, Max − P95, OverflowZone)` only when `v > P95`, else `""`. A row at exactly p95 ends at the rule. For the pivot `Segments = Apportion(values, utf8.RuneCountInString(Main))`; for the single-tool table `Segments = nil`.

### 5. `render/ansi` — encoding the new model

#### 5.1 Table encoding additions (`table.go`)

`Table(t view.Table, c Colors) []string` keeps the V2 layout and adds:

- **Label cell styling**: a `Data` row's first cell is padded first, then wrapped — `BoldWhite` for `Current`, `Dim` for `Weekend`, plain otherwise. Only the date cell; widths unchanged.
- **Dim zero cells**: a `Data` cell with `Dim` is padded first, then `Dim`-wrapped. Header/Total cells are never dimmed. (V2's `joinPlain` learns the two per-cell styles.)
- **Separator rows** render exactly like the header divider — `Dim` of `divider(cols) + barDiv` — where `barDiv = "─" + "─"×Scale.Width` when `Scale.Width > 0` (also appended to the header and Total dividers). This is the TS `divStr + costDiv + barDiv`.
- **Delta**: `+1` → `Green("↑")`, `−1` → `Red("↓")`, prefixed with `" "` when `DeltaSpaced`; appended right after the last cell, before the bar.
- **Bar**: appended after the delta. Single-zone: `" " + fill(Main)` when `Main != ""`, else nothing. Two-zone: `" " + fill(Main) + spaces(MainZone − runes(Main)) + Dim("┊") + Yellow(Overflow) + spaces(OverflowZone − runes(Overflow))` — **always** emitted in two-zone mode, including for a zero row, and `Yellow("")` / `Green("")` DO wrap the empty string (`ESC[33mESC[0m`) because the TS `wrap()` has no empty guard and neither does the Go `Colors.wrap`; the goldens pin this. `fill`: `Segments == nil` → `Green(Main)`; else contiguous slices of `Main` colored by palette slot (`Green`, `Magenta`, `Blue`, `Cyan`; slot ≥ 4 uncolored), zero-count segments skipped — stripping ANSI from the result yields `Main` unchanged.
- **Footer**: `Dim(Footer)` on its own line after the Total row; when `Legend != nil` and `c.Enabled`, append `Dim(" · ")` + swatches joined by `" "`, each swatch `palette(i)("█") + " " + Dim(Name)`. With color disabled the legend is omitted entirely (the TS gates on `colorDisabled()`).
- Everything else (blank line, `BoldWhite` title, blank, `Empty` + blank, header cells `BoldCyan` individually, plain ` | ` separators, closing blank) is V2's.

Line shape of a populated single-tool row at 80 columns (no bars) is therefore `row cells joined " | "` + `" | " + cost cell` — identical bytes to the TS, see §12.

#### 5.2 What stays out of ansi

No width probing, no `os`, no `NO_COLOR` read — `Colors` and `Width` arrive as values. Compact layouts, `maxRows` and the machine legend line are not encoded in B2 (B7/B4).

### 6. `render/csv` — new encoder (RFC 4180, machine contract)

```go
package csv
func Snapshot(rows []view.ToolTotals) []string          // header + one row per tool with TotalTokens > 0 + Total when >1 visible
func History(s view.Series) []string                    // header + one row per entry; NEVER a Total row
func TotalHistory(series []view.Series) []string        // header + one row per label; every series (registry) column; NEVER a Total row
func Cost(x float64) string                             // the TS csvCost: toFixed(2) — exact binary value rounded half-up
```

- Headers: `tool,tokens,input,output,cache,cost`; `date,input,output,cache_write,cache_read,total,cost`; `date,{Name1},…,{Name6},total`.
- Snapshot rows: `name, tokens, input, output, cacheCreation+cacheRead, Cost(cost)`; Total sums **every** input row (hidden rows counted, as in the table) and appears only when more than one row is visible (`Total,64290220,…,31.43`).
- History rows: `label, input, output, cache_write, cache_read, total, Cost(cost)` in input order.
- TotalHistory: labels = sorted union; per label `Cost(value per series, 0 when absent)` then `Cost(rowTotal)`. **No column omission** (positional contract, DC-05/DC-06).
- Integers via `strconv.FormatInt`; every field passes `quote` (wrap in `"` and double inner `"` when it contains `,`, `"`, `\n` or `\r` — no shipped name needs it, but the rule is pinned by a unit test).
- An empty window prints the header line alone (`h-csv`: `date,Claude Code,Codex,OpenCode,Gemini,Copilot,Kimi,total`).
- **`Cost` is a third rounding rule**, distinct from `render.FormatCost` (ICU: shortest-repr, half away from zero, grouped) — V2's memory warned about exactly this. `toFixed(2)` rounds the **exact** binary value half-up. Implementation: `r := new(big.Rat).SetFloat64(x)` (exact), scale by 100, take the floor and round up when the remainder `≥ 1/2`; format as `int.frac2`; `-0` and `0` → `0.00`. Verified against node v24 (2026-09-16): `1.005 → 1.00` (ICU `1.01`), `0.125 → 0.13`, `0.375 → 0.38`, `2.675 → 2.67` (ICU `2.68`), `0.015 → 0.01` (ICU `0.02`), `1.045 → 1.04` (ICU `1.05`), `8.345 → 8.35`, `0.005 → 0.01`, `0.045 → 0.04` (ICU `0.05`), `999999.995 → 999999.99` (ICU `1,000,000.00`), `1234567.891 → 1234567.89`, `0.5 → 0.50`, `0 → 0.00`. `strconv.FormatFloat(x, 'f', 2, 64)` is wrong on the exact ties (`0.125 → 0.12`, half-even).

### 7. `render/markdown` — new encoder (GFM, human contract)

```go
package markdown
func Snapshot(rows []view.ToolTotals, period query.Period) []string
func History(s view.Series, period query.Period, capActive bool) []string
func TotalHistory(series []view.Series, period query.Period, capActive bool) []string
```

- Lines: `## {title}`, `""`, `| h1 | h2 | … |`, the alignment row (`:---` for the text column, `---:` for numerics), data rows, `**Total**` row when applicable, then `""` — the caller's per-line `Fprintln` reproduces the TS `output + "\n"` trailing blank line (§12 `cc-mh-md`).
- Titles: `Combined Usage ({period})`; `{Name} ({period}[, last 3 months])`; `Combined Cost History ({period}[, last 3 months])` — **always "Cost"**: the Markdown pivot ignores `--metric` and renders `TotalCost` cells (the TS `emitMarkdownTotalHistory` reads `e.totalCost`; only `lbh` overrides the title, B5). No `📊`.
- Numbers `render.FormatInt`, costs `render.FormatCost` (ICU rule, `$` and grouping).
- Snapshot: rows with `TotalTokens > 0`; `| **Total** | **64,290,220** | … | **$31.43** |` when more than one is visible (sums every row).
- History: one row per entry; `**Total**` (bold cells) when `len(entries) > 1`.
- TotalHistory: columns = `nonzeroTools` over the cost map across **all** labels (exact-zero columns dropped; if none is nonzero, all six stay — the empty `h-md` header lists every tool); last column `Cost`; `**Total**` when `len(labels) > 1`.
- Empty window: heading, blank, header, alignment row, blank — no data rows (`h-md` in §12).

### 8. `render/json` — the two history shapes

```go
func History(s view.Series) []string             // tu cc h --json: a bare array of entry objects
func TotalHistory(series []view.Series) []string // tu h --json: {"{Name}": [entries…], …} — every registry tool, [] when empty
```

- Entry object keys in order: `label`, `totalCost`, `inputTokens`, `outputTokens`, `cacheCreationTokens`, `cacheReadTokens`, `totalTokens` (`label` is always present on history entries — `toUsageEntry` sets it; the V2 conditional applies to snapshots only).
- Layout is `JSON.stringify(v, null, 2)`: an empty array is the two bytes `[]` inline (`"Codex": []`, or the whole output `[]` for an empty single-tool history); a populated nested array puts each object on its own indented block:

```
{
  "Claude Code": [
    {
      "label": "2026-01",
      "totalCost": 1.5,
      …
      "totalTokens": 73200
    }
  ],
  "Codex": []
}
```

- Scalars through the V2 helpers (`encodeString`, `encodeFloat` with `-0 → 0`). Costs are the raw summed doubles (DC-10) — the Go sum order is per-tool ascending by date within `RollUp`, the same order the TS `aggregateMonthly` accumulates, so `1.5` and `4.936068800000001`-style values match.

### 9. `command` — guards, cap, scope, composition

#### 9.1 Guards and the cap (`guards.go`, pure)

```go
// Normalize applies the TS main() flag guards in TS order and returns the request the pipeline
// runs plus the stderr notice lines the edge prints BEFORE any fetch warning. Pure.
func Normalize(req Request, now time.Time) (Request, []string, capActive bool)
```

In this order (B4 inserts its `--by-machine` pivot guard before step 1; B5 inserts `--top` after step 2):

1. `Since != "" || Until != ""` and `Display != History` → notice `Warning: --since/--until apply to history display — ignoring.`; clear both.
2. `Full` and `Display != History` → notice `Warning: --full applies to daily/weekly history — ignoring.` (the flag is left set; nothing reads it downstream).
3. **Cap**: `Display == History && Period != Monthly && Since == "" && Until == "" && !Full` → `Since = query.ThreeMonthFloor(now)`, `capActive = true`. An explicit bound on either side disables the cap entirely (no intersection); `mh --full` is a silent no-op; `--full` with an explicit window is silently accepted.

(`lb`/`lbh` are excluded from these guards in the TS; they are B5's and remain placeholder here.)

#### 9.2 Scope

`inScope` accepts `Display ∈ {Snapshot, History}`, all four `Format`s, and additionally `Since`, `Until`, `Full` — evaluated on the **normalized** request (so `--since` on a snapshot is in scope after the guard clears it). Still out: multi mode, `User`, `ByMachine`, `Top`, `Watch`, `Sync`, `DryRun`, `NoRain`, `SkipBrewUpdate`, any `Command`, `Version`. A request out of scope returns `ErrUnported` **without** printing the notices (the TS prints them and then does the unported thing; no harness case combines them — B4/B5 inherit the ordering when they land).

#### 9.3 Run

```go
type Deps struct { Source Fetcher; Now func() time.Time; Colors ansi.Colors; Width int }
type Result struct {
    Lines       []string
    Notices     []string            // guard warnings; the edge writes them before Warnings
    Warnings    []*source.Error
    TotalCost   float64
    TotalTokens int64
    CostByItem  map[string]float64  // snapshot: "{Name}"; history: "{Name}:{label}" and "total:{label}", valued in the display metric
}
```

1. `req, notices, capActive := Normalize(req, deps.Now())`; scope check; timeout; **daily-only fetch** exactly as V2 (`FetchAll` or `Fetch(tool)`, `fresh = Flags.Fresh`).
2. **Snapshot** → V2's path unchanged, then render by format: `JSON` → `json.Snapshot`; `CSV` → `csv.Snapshot(rows)`; `Markdown` → `markdown.Snapshot(rows, period)`; `Table` → `ansi.Table(view.Snapshot(...))`.
3. **History** → `recs = query.RollUp(query.Window(recs, Since, Until), Period)`; build `[]view.Series` in registry order (all six tools, or the one source) from **one** `query.GroupBy(recs, query.Tool, query.Date)` pass — each group's `Key.Tool` selects the series, `Key.Date` the entry label; entries stay ascending because `RollUp` sorted and `GroupBy` preserves first-seen order. No per-tool aggregation loop.
   - Single source: `s := series[0]`; `Table` → `ansi.Table(view.History(s, opts), Colors)`; `JSON` → `json.History(s)`; `CSV` → `csv.History(s)`; `Markdown` → `markdown.History(s, period, capActive)`.
   - All tools: `Table` → `ansi.Table(view.TotalHistory(series, opts), Colors)`; `JSON` → `json.TotalHistory(series)`; `CSV` → `csv.TotalHistory(series)`; `Markdown` → `markdown.TotalHistory(series, period, capActive)`.
   - `opts = view.HistoryOptions{Period, Now: deps.Now(), Width: deps.Width, CapActive: capActive, Metric: from Flags.Metric}` — `Prev` nil.
4. `Result.TotalCost`/`TotalTokens` sum every entry; `CostByItem` per the key scheme above (the TS `buildCostMap(…, metric)`); `Notices` from step 1; `Warnings` from the fetch.

#### 9.4 Not changed

`Parse` is complete since V2 — no grammar or message changes. The JSON label quirk and the uniform-cache decision stand.

### 10. `cmd/tu` — width probe and write order

- `Deps.Width` = `terminalWidth(stdout)`: when `stdout` is an `*os.File` whose descriptor `term.IsTerminal` reports a TTY, `term.GetSize` (columns); otherwise **80** (the TS `process.stdout.columns ?? 80` — `COLUMNS` is never consulted, DC-12). `bytes.Buffer` in tests → 80; under the harness's `script` wrapper (`stty cols 120`) both sides see 120 — no history case runs on the tty axis, so nothing in the gate depends on it.
- `golang.org/x/term` becomes the module's first dependency (`go get golang.org/x/term`, `go.sum` committed). It is the toolkit's standard terminal library and plan D12 builds watch mode on it; a stdlib `ioctl(TIOCGWINSZ)` would need per-OS build tags for a five-line probe.
- Write order after `Run`: `Notices` (each `Fprintln(stderr)`), then `source.WriteWarnings(stderr, Warnings)`, then `Lines` on stdout, exit 0. Everything before `Run` (parse, version, commands, HOME, `Load` warnings, reserved-user guard) is B1's order, unchanged.
- `main_test.go`: `TestRunNotImplemented` drops `{"m","dh","--json"}` and `{"--csv"}` (both now succeed) and keeps `{"--help"}`; a `{"h","--by-machine"}` case pins that B4's flag still routes to the placeholder.

### 11. Tests

- **`query`**: `ThreeMonthFloor` for `2026-09-16` → `2026-07-01`, `2026-01-15` → `2025-11-01`, `2026-02-28` → `2025-12-01`, in `UTC` and `Asia/Kolkata`; a window-then-roll-up case proving the leading partial week (`Window(01-01..01-31)` then weekly → `2026-01-04`).
- **`view`** (table-driven): `Glyphs` (zero, exact eighths, `eighths == 8` carry, the `▏` floor); `Percentile` (n = 1, interpolation); `ComputeScale` (no nonzero → single; `max == 1.5 × p95` stays single; two-zone geometry at widths 10 and 30 → overflow 4/8, main 5/21); `Apportion` (sums exactly, ties to the earlier index, zero shares); `History` widths (floor 9, growth on a `$59,634.40` total), separators only on daily and only between months, label styles (current wins over weekend; weekend only on daily), Total/footer only when > 1 row, footer terms (`this month` present/absent, all-zero peak without parentheses, unit suffix per period, p95 term), token mode swaps the last column and its values; `TotalHistory` significance (boundary kept, `$0.40` window falls back to nonzero, all-zero falls back to all six), rowValue over omitted tools, per-tool widths (`Claude Code` → 11), bar budget at 80 vs 120 columns, `indicatorReserve` when `Prev != nil`, segments per row, legend only with bars and ≥ 2 tools, delta from `Prev`.
- **`render/ansi`** goldens (`-update`): `history_80col_color` (the §12 `cc-h-window` bytes), `history_80col_nocolor`, `history_wide_bars` (Width 140, single-zone green bars), `history_two_zone` (an outlier window: rule, yellow overflow, empty-wrap escapes on a zero row, footer p95 term), `history_single_row`, `history_empty`, `history_tokens`, `history_separators_weekend_today` (a window spanning a month boundary with a Saturday and an injected `Now` equal to one label), `pivot_80col_color` (the §12 `h-window` bytes), `pivot_80col_nocolor`, `pivot_wide_stacked` (Width 120, three visible tools, segments + legend), `pivot_wide_nocolor` (legend omitted), `pivot_omission_dim_zero` (a `$0.04` tool dropped, `$0.00` cells dim), `pivot_single_row`, `pivot_empty`. Every colored golden equals its no-color twin under `StripANSI`; every bar row's stripped bar equals `Main` + padding + `┊` + `Overflow` + padding. The four V2 snapshot goldens are byte-unchanged.
- **`render/csv`**: `Cost` table with the thirteen node-verified values in §6; goldens for the three kinds populated and empty; the quoting rule on a synthetic name containing a comma and a quote.
- **`render/markdown`**: goldens for the three kinds populated (`cc-mh-md` bytes), with Total, empty (`h-md` bytes), pivot zero-column omission with the all-zero fallback, cap hint in the heading.
- **`render/json`**: goldens `history_populated` (`cc-mh-json` bytes), `history_empty` (`[]`), `total_history_mixed` (populated and empty arrays), `total_history_all_empty` (`h-json` bytes).
- **`command`**: `Normalize` — each guard's notice text, clear semantics, cap on/off for the eight combinations of period/since/until/full, `ThreeMonthFloor` threaded from `Now`; `Run` with the fake `Fetcher`, `Now = 2026-01-06T12:00:00Z`, `Width = 80` and `120`: single-source history lines equal the goldens, all-tools pivot, window applied before roll-up, JSON/CSV/Markdown lines, `CostByItem` keys, `Notices` for a snapshot with `--since`, `ErrUnported` for `--by-machine`/`-u`/multi mode with history; `inScope` on the normalized request.
- **`cmd/tu` e2e** (`TestMain` fake ccusage, `_placeholder`, `TZ=UTC`): `cc h --since 2026-01-01 --until 2026-01-31` (populated, exit 0), `h` (capped heading + `  No data`), `mh` (one row, no Total), `h --json`, `cc mh --csv`, `cc mh --md`, `--csv` and `--md` snapshot empties, `--since 2026-13-01` (stderr warning + empty snapshot, exit 0), `h --by-machine` (placeholder), `wh --since 2026-01-01 --until 2026-01-31` (`2026-01-04` row). The e2e binary sees a `bytes.Buffer`, so width is 80 throughout.
- **Harness gate** (acceptance, quoted in the PR): `just go-diff --placeholder` reports the 39 IDs in §2 `GREEN` and the summary `134 green`; `just go-lint` and `just go-test` clean; on dev-ws-sahil02 `just go-diff --filter h` against the local capture SHOULD also be green (the real-bytes check for omission, dim zeros, separators and weekend dimming).

### 12. Byte-exact references (node v24, TS v0.11.5, placeholder corpus, `TZ=UTC`, piped — from `bin/harness/report/cases/`)

`ESC` stands for `\x1b`; every line ends `\n`. Dashes in dividers are U+2500 `─`.

**`tu cc h --since 2026-01-01 --until 2026-01-31`** (`cc-h-window`, identical under `TZ=Asia/Kolkata` and to `cc h --full`):

```
(blank)
ESC[1;37m📊 Claude Code (daily)ESC[0m
(blank)
ESC[1;36mDate        ESC[0m | ESC[1;36m         InputESC[0m | ESC[1;36m        OutputESC[0m | ESC[1;36m   Cache WriteESC[0m | ESC[1;36m    Cache ReadESC[0m | ESC[1;36m         TotalESC[0m | ESC[1;36m     CostESC[0m
ESC[2m─────────────|────────────────|────────────────|────────────────|────────────────|────────────────|──────────ESC[0m
2026-01-05   |          3,000 |            400 |          1,000 |         20,000 |         24,400 |     $0.50
2026-01-06   |          3,000 |            400 |          1,000 |         20,000 |         24,400 |     $0.50
2026-01-07   |          3,000 |            400 |          1,000 |         20,000 |         24,400 |     $0.50
ESC[2m─────────────|────────────────|────────────────|────────────────|────────────────|────────────────|──────────ESC[0m
ESC[1;37mTotal       ESC[0m | ESC[1;37m         9,000ESC[0m | ESC[1;37m         1,200ESC[0m | ESC[1;37m         3,000ESC[0m | ESC[1;37m        60,000ESC[0m | ESC[1;37m        73,200ESC[0m | ESC[1;37m    $1.50ESC[0m
ESC[2mavg $0.50/day · peak $0.50 (2026-01-05)ESC[0m
(blank)
```

(The divider is `─`×12, then five `─|─` + `─`×14 groups, then `─|─` + `─`×9 — no bar segment at 80 columns. Data rows carry no escapes: no zero cells, no weekend, not today.)

**`tu h --since 2026-01-01 --until 2026-01-31`** (`h-window`; same bytes for `h --full`, `dh-*`, `history-*`):

```
(blank)
ESC[1;37m📊 Combined Cost History (daily)ESC[0m
(blank)
ESC[1;36mDate      ESC[0m | ESC[1;36mClaude CodeESC[0m | ESC[1;36m    CodexESC[0m | ESC[1;36m OpenCodeESC[0m | ESC[1;36m   GeminiESC[0m | ESC[1;36m  CopilotESC[0m | ESC[1;36m     KimiESC[0m | ESC[1;36m     CostESC[0m
ESC[2m───────────|─────────────|───────────|───────────|───────────|───────────|───────────|──────────ESC[0m
2026-01-05 |       $0.50 |     $0.50 |     $0.50 |     $0.50 |     $0.50 |     $0.50 |     $3.00
2026-01-06 |       $0.50 |     $0.50 |     $0.50 |     $0.50 |     $0.50 |     $0.50 |     $3.00
2026-01-07 |       $0.50 |     $0.50 |     $0.50 |     $0.50 |     $0.50 |     $0.50 |     $3.00
ESC[2m───────────|─────────────|───────────|───────────|───────────|───────────|───────────|──────────ESC[0m
ESC[1;37mTotal     ESC[0m | ESC[1;37m      $1.50ESC[0m | ESC[1;37m    $1.50ESC[0m | ESC[1;37m    $1.50ESC[0m | ESC[1;37m    $1.50ESC[0m | ESC[1;37m    $1.50ESC[0m | ESC[1;37m    $1.50ESC[0m | ESC[1;37m    $9.00ESC[0m
ESC[2mavg $3.00/day · peak $3.00 (2026-01-05)ESC[0m
(blank)
```

**`tu mh`** (uncapped, one row → no divider, Total or footer): title `📊 Combined Cost History (monthly)`, the same header/divider, one row `2026-01    |       $1.50 |     $1.50 |     $1.50 |     $1.50 |     $1.50 |     $1.50 |     $9.00`, blank. **`tu cc mh`**: `📊 Claude Code (monthly)`, one row `2026-01      |          9,000 |          1,200 |          3,000 |         60,000 |         73,200 |     $1.50`. **`tu wh --since … --until …`**: `(weekly)`, one row labeled `2026-01-04`.

**`tu h`** (capped, empty): blank, `ESC[1;37m📊 Combined Cost History (daily, last 3 months)ESC[0m`, blank, `  No data`, blank. **`tu cc h`**: `📊 Claude Code (daily, last 3 months)` + `  No data`.

**`tu h --json`**: `{`, then `  "Claude Code": [],` … `  "Kimi": []` (six lines, registry order, last without comma), `}`. **`tu cc mh --json`**:

```
[
  {
    "label": "2026-01",
    "totalCost": 1.5,
    "inputTokens": 9000,
    "outputTokens": 1200,
    "cacheCreationTokens": 3000,
    "cacheReadTokens": 60000,
    "totalTokens": 73200
  }
]
```

**`tu h --csv`**: `date,Claude Code,Codex,OpenCode,Gemini,Copilot,Kimi,total` (header only). **`tu cc mh --csv`**: `date,input,output,cache_write,cache_read,total,cost` then `2026-01,9000,1200,3000,60000,73200,1.50`. **`tu --csv`** and **`tu cc --csv`** (empty snapshot): `tool,tokens,input,output,cache,cost`.

**`tu cc mh --md`**:

```
## Claude Code (monthly)
(blank)
| Date | Input | Output | Cache Write | Cache Read | Total | Cost |
| :--- | ---: | ---: | ---: | ---: | ---: | ---: |
| 2026-01 | 9,000 | 1,200 | 3,000 | 60,000 | 73,200 | $1.50 |
(blank)
```

**`tu h --md`** (empty, capped): `## Combined Cost History (daily, last 3 months)`, blank, `| Date | Claude Code | Codex | OpenCode | Gemini | Copilot | Kimi | Cost |`, `| :--- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |`, blank. **`tu --md`**: `## Combined Usage (daily)`, blank, `| Tool | Tokens | Input | Output | Cache | Cost |`, `| :--- | ---: | ---: | ---: | ---: | ---: |`, blank.

**`tu --since 2026-13-01`** (`since-invalid`): stderr `Warning: --since/--until apply to history display — ignoring.`; stdout the V2 empty snapshot (`📊 Combined Usage (daily)` + `  No usage`); exit 0.

### 13. Explicitly not in B2

Multi-mode records, `-u`, the metrics-dir guard (B3); `--by-machine`, machine columns in any format, the `Machines:` legend line, the pivot's by-machine warning (B4); `lb`/`lbh`, `--top`, the `lbh` pivot hooks, leaderboard CSV/Markdown/JSON kinds (B5); sync (B6); watch — `Prev` maps filled, `maxRows`, compact layouts, delta arrows with live data (B7 — B2 only encodes the `Delta`/`Prev` seam and pins it with a golden); help/help-dump/skill/shell-init/update (B8). No spec, harness, workflow, justfile, `src/node/` or plan-doc edits.

## Affected Memory

- `go-port/query-view-render`: (modify) `query.ThreeMonthFloor`; the window-before-roll-up rule for history; the typed `view` model (`LabelStyle`, `Delta`, `Bar`, `Scale`, `Swatch`, `Footer`, `Legend`, `DeltaSpaced`); `view.History` and `view.TotalHistory` rules (widths, significance and its two fallbacks, separators, marker/weekend, footer terms incl. the all-zero peak, bar budget and `indicatorReserve`, stacked segments, token mode); the bar primitives (`Glyphs`, `Percentile`, `ComputeScale`, `Apportion`) with the exact geometry; `render/ansi` encoding of styles, dividers with bar segments, deltas, bars (including the empty-wrap escapes in two-zone mode), footer and legend; `render/json` history shapes and nested layout; the new `render/csv` (three kinds, `Cost` exact-half-up rule with the verified table, no Total on history kinds, no omission) and `render/markdown` (three kinds, Cost-only pivot title, exact-zero omission, `**Total**` rule); golden inventory. Design Decisions: view emits raw glyphs plus geometry and ansi only colors (strip-ANSI invariant structural); `csv.Cost` is a third rounding rule implemented on `big.Rat`; `x/term` as the width source.
- `go-port/command-edge`: (modify) `Normalize` guards and their order with the B4/B5 insertion points; the cap; the widened `inScope`; the history composition (one `GroupBy(Tool, Date)` pass, registry-ordered series); `Result.Notices` and the edge write order (notices → source warnings → lines); `Deps.Width` and the TTY probe (80 when piped); the row map update (B2's surfaces now implemented; `since-invalid` green); the harness status line (134/368) and the placeholder-corpus coverage caveats.
- `build/toolchain`: (modify) `go.mod` gains `golang.org/x/term` and a `go.sum`; the `cmd/tu` bullet's implemented-surface list; the `go-diff` green count.

`display/formatting`, `cli/data-pipeline`, `harness/differential-harness` are not modified: no TS or harness code changes. G0 items surfaced here are recorded under Open Questions, not hydrated.

## Impact

- **New Go code**: `view` (~550 lines: two tables, bars, footer), `render/ansi` additions (~150), `render/csv` (~130), `render/markdown` (~150), `render/json` (~70), `command` (~150), `cmd/tu` (~30); tests and goldens roughly 1,300–1,600 lines. This is at the **top of M and plausibly L**; if apply finds it growing, the natural seam is to land the CSV/Markdown encoders in a follow-up row *after* the tables — but note the gate includes 10 csv/md IDs, so a split changes the row's acceptance and must be reported, not silently taken.
- **Dependencies**: `golang.org/x/term` (and transitively `golang.org/x/sys`), the module's first `require`. CI's `setup-go` already runs with `cache: false`, so no workflow change; enabling the cache is a later tidy-up.
- **Harness**: +39 green (95 → 134). `go-diff` CI job stays informational.
- **Downstream rows**: B3 hands multi-mode records to the same `Run` and reuses every encoder; B4 adds machine columns to `History`/`Snapshot` (view, csv, markdown, json) and its guard line; B5 builds `lbh` on `TotalHistory` via the hooks described in §4.4 and adds the leaderboard kinds to each encoder; B7 fills `Prev` and adds `maxRows`/compact; G1 reviews the width and notice seams introduced here.
- **Risk**: bars, omission, dim zeros, separators, marker and weekend styling are verified in CI only through goldens (placeholder dates and equal costs never trigger them); the dev-ws-sahil02 local-capture run is the real-bytes check. The `csv.Cost` rule and the empty-wrap escapes in two-zone bars are the two places a plausible-looking implementation silently diverges; both are pinned with node-verified values.

## Open Questions

- **G0 / spec note**: `docs/specs/usage.md` § Output Formats says the machine formats honor `--full` — true; but it does not say the Markdown pivot ignores `--metric` entirely (title stays `Combined Cost History`, cells stay cost) while the ANSI pivot switches to `Token History`. B2 reproduces the binary; the spec's Markdown row could state it explicitly (it currently implies "ignores the metric" only via the general sentence).
- **Guard ordering with unported flags**: a request combining an in-scope guard (`--since` on a snapshot) with a B4/B5 flag routes to the placeholder without printing the notice, whereas the TS prints the notice first. Byte parity for such combinations arrives when B4/B5 land; recorded so those rows keep §9.1's order.
- **`x/term` as the first dependency**: reversible in one file if G1 prefers a stdlib probe or deferring TTY width to B7 (in which case `tu h` in a real terminal would render without bars until B7).

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Scope = single-mode history (`h`/`history`/`dh`/`wh`/`mh`) plus `--since`/`--until`/`--full`, `--csv`/`--md` on snapshot and history, and the two history JSON shapes; everything else stays on the placeholder | Row text; V2's command-edge memory assigns exactly these surfaces to B2; the JSON history cases (`h-json`, `cc-mh-json`) are history cases and the row says "all history cases" | S:85 R:80 A:90 D:85 |
| 2 | Certain | Harness gate = the 39 IDs in §2 (38 history/csv/md IDs plus `since-invalid`), expected summary 134 green | Enumerated from `harness/matrix.json` and the live `just go-diff --placeholder` report; `since-invalid` is red today with node exit 0 and is the since/until warn-and-render case V2 reserved for B2 | S:85 R:85 A:95 D:85 |
| 3 | Certain | History applies `Window` to daily records before `RollUp`; snapshot keeps V2's `Window(RollUp(...), cur, cur)` | The TS filters before `aggregateForPeriod` on every history path (memory, spec); the harness `wh-window` case yields the leading partial-week label `2026-01-04` only with this order | S:80 R:85 A:95 D:90 |
| 4 | Confident | The pivot's series come from one `GroupBy(recs, Tool, Date)` pass split by registry order; the single-tool table from the same pass | G1 item 2 (one group-by, no per-dimension aggregation); `RollUp` already sorts so first-seen order is ascending | S:60 R:85 A:85 D:75 |
| 5 | Confident | `view` emits raw glyph strings plus geometry (`Bar{Main, Overflow, Segments}`, `Scale`) and `render/ansi` only colors and pads; strip-ANSI equals the raw bar by construction | Plan: view owns "bar scales", render owns ANSI; the TS stacked-bar decision already colors an already-rendered glyph string; makes the no-color twin invariant structural | S:65 R:75 A:85 D:70 |
| 6 | Certain | Bar geometry lifted verbatim: eighths glyph table, `▏` floor, p95 by linear interpolation over nonzero values, `max > 1.5 × p95` trigger, overflow `max(4, round(w/4))`, rule 1 char, largest-remainder apportionment with ties to the earlier column | Read from `formatter.ts`; spec and memory state the same numbers | S:90 R:85 A:95 D:90 |
| 7 | Certain | Two-zone bars wrap empty strings in color codes (`ESC[33mESC[0m`) exactly as the TS `wrap()` does; no empty guard is added | `colors.ts` `wrap` has no empty check and neither does the Go `Colors.wrap`; a "fix" would diverge bytes on every zero row of an outlier window | S:70 R:90 A:95 D:85 |
| 8 | Certain | Significance rule: keep a pivot column iff total ≥ $1.00 (1,000 tokens) and ≥ 0.1% of the grand total over all tools; fallbacks nonzero then all; Markdown uses exact-zero only; CSV omits nothing | Memory, spec § All-Tools History Pivot Table, DC-06; reproduced not unified (plan: nothing outside the drop list changes) | S:85 R:80 A:95 D:90 |
| 9 | Certain | Single-tool widths Date 12 and 14 for the five token columns; pivot Date 10 and per-tool `max(name, 9, cells incl. Total)`; last column `max(9, longest value incl. Total)`; bars need ≥ 10 chars after a 3-char gutter and cap at 30 | Constants in `formatter.ts`; verified against the §12 captures (84-wide pivot body, 97-wide history body) | S:90 R:85 A:95 D:95 |
| 10 | Certain | Footer text `avg X/unit [· this month X] · peak X (label) [· ┊ = X (p95)]`, dim as one span; legend appended outside the dim span and only when bars show, ≥ 2 visible tools and color is on; first strict max picks the peak and an all-zero window prints `peak $0.00` without parentheses | Lifted from `renderHistoryFooter`; captures confirm `avg $3.00/day · peak $3.00 (2026-01-05)` | S:85 R:85 A:95 D:90 |
| 11 | Certain | `csv.Cost` = exact binary value rounded half-up to two decimals via `big.Rat`, not `FormatFloat('f', 2)` and not `render.FormatCost` | node v24 probe: `1.005 → 1.00`, `0.125 → 0.13`, `1.045 → 1.04`, `0.045 → 0.04`, `999999.995 → 999999.99`; Go `'f',2` gives `0.12` on the exact tie; V2's memory recorded the rule difference | S:75 R:90 A:90 D:85 |
| 12 | Certain | CSV history kinds never carry a Total row; the snapshot CSV carries one when more than one tool is visible; an empty window prints the header alone | `emitCsvHistory`/`emitCsvTotalHistory` have no Total branch; spec § CSV Output; `h-csv` capture | S:85 R:85 A:95 D:90 |
| 13 | Certain | Markdown lines end with one trailing blank line; `**Total**` when more than one data row; the pivot title is always `Combined Cost History` and its cells always cost regardless of `--metric` | `emitMarkdown` writes `output + "\n"` after a trailing empty element; `emitMarkdownTotalHistory` reads `totalCost` and `titleForTotalHistory` has no metric parameter; `cc-mh-md` capture | S:80 R:85 A:95 D:90 |
| 14 | Certain | JSON history: single-tool = bare array of entries with `label` first; all-tools = object keyed by display name in registry order, `[]` inline for empty; nested layout per `JSON.stringify(v, null, 2)` | `renderHistoryByFormat`/`renderTotalHistoryByFormat` call `emitJson`; `h-json` and `cc-mh-json` captures; node probe of the nested layout | S:85 R:85 A:95 D:90 |
| 15 | Confident | Guards live in a pure `command.Normalize(req, now)` returning the normalized request, notice lines and `capActive`; `Result.Notices` is written by the edge before source warnings; scope is checked on the normalized request | The TS prints the guards in `main()` before dispatch and before any fetch warning; nothing below `cmd/tu` may write (G1 item 1); `since-invalid` needs the cleared flags to be in scope | S:60 R:85 A:85 D:70 |
| 16 | Certain | Guard order since/until → full → cap, with B4's by-machine guard slotting first and B5's top guard after full; the cap is `History ∧ Period ≠ Monthly ∧ no explicit bound ∧ !Full` with floor `ThreeMonthFloor(now)` in local time | `main()` block and `capApplies` read verbatim; memory 260717-yuuj; `h` vs `h-full` vs `h-window` captures show hint present/absent | S:85 R:85 A:95 D:90 |
| 17 | Certain | Warn-and-clear texts: `Warning: --since/--until apply to history display — ignoring.` and `Warning: --full applies to daily/weekly history — ignoring.`, exit 0, snapshot still rendered | `since-invalid` node capture; spec § Global Flags (DC-03) | S:90 R:90 A:95 D:95 |
| 18 | Confident | Terminal width is probed in `cmd/tu` with `golang.org/x/term` (`IsTerminal` + `GetSize` on stdout, else 80) and passed as `Deps.Width`; the module gains its first dependency | TS uses `process.stdout.columns ?? 80` and never `COLUMNS` (DC-12); plan D12 builds watch mode on `x/term`; the six sibling tools use it; stdlib `ioctl` needs per-OS build tags; no gate case depends on the TTY branch | S:40 R:90 A:80 D:60 |
| 19 | Confident | `Delta`/`Prev` seam is built and golden-pinned in B2 with `Prev` nil from `Run`; `maxRows`, compact layouts and live prev maps are B7 | Row text lists "deltas"; the TS keys (`{Name}:{label}`, `total:{label}`) and the space-less pivot form are byte surfaces B7 must not reinvent; nothing in B2 observes them at runtime | S:55 R:85 A:80 D:65 |
| 20 | Confident | Token mode (`-t`/`--metric tokens`) is implemented for both history tables in this row (last column `Tokens`, `Token History` title, thresholds in tokens, bars and footer in tokens); JSON/CSV/Markdown ignore it | `-t`/`--metric` are already in scope since V2, so `tu h -t` must render or placeholder; the TS has one metric-generic path; no gate case exercises it — goldens do | S:50 R:80 A:85 D:70 |
| 21 | Certain | `Result.CostByItem` for history uses keys `{Name}:{label}` and `total:{label}` valued in the display metric; `TotalCost`/`TotalTokens` sum every entry | `buildCostMap(data, metric)` and `sumAllToolCosts`/`sumAllToolTokens` in `cli.ts` | S:75 R:90 A:90 D:85 |
| 22 | Certain | Tests: table-driven `query`/`view`/`command`, golden files with `-update` for every encoder (ansi, csv, markdown, json) including width-injected bar goldens and no-color twins, fake-`Fetcher` `Run` tests with fixed `Now` and `Width`, e2e through the fake ccusage on `_placeholder` | G1 item 5; V1/V2 conventions; bars are unobservable in the harness at 80 columns, so goldens are the only CI check | S:75 R:90 A:90 D:85 |
| 23 | Confident | Memory: modify `go-port/query-view-render`, `go-port/command-edge`, `build/toolchain`; no new memory file; TS memories untouched | One file per layer (V2 decision); B2 extends the same layers; X3 reshapes at cutover | S:45 R:90 A:75 D:65 |
| 24 | Certain | No external surface changes: no `src/node`, spec, justfile, CI, harness, plan-doc edits; Go stays unshipped | Row text and plan Goal; D2; the operator owns plan row status | S:95 R:90 A:95 D:95 |

24 assumptions (17 certain, 7 confident, 0 tentative, 0 unresolved).
