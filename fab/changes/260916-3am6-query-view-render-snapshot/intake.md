# Intake: Query, View, Render and the Snapshot Command (Go port row V2)

**Change**: 260916-3am6-query-view-render-snapshot
**Created**: 2026-09-16

## Origin

One-shot `/fab-new` invocation, handed over from the Go-port plan's queue (plan row V2, the second Phase 1 row, after V1 merged as PR #84):

> Context: fab/plans/sahil/26-09-15-go-port.md, row V2. Read the Decisions and Target architecture sections before writing the intake. Do not change any external surface listed under Goal. Build query (pure: window filter by since/until, user/tool filter, daily to weekly/monthly roll-up, GroupBy(dims...) as the one group-by for tool/machine/user pivots), view (pure: query result to table model — columns, rows, cells, deltas, bar scales, legend; no ANSI), and render/ansi (pad, color, NO_COLOR primitives, the snapshot table) plus render/json. command parses the bare grammar: tu, tu cc, tu m, --json, --no-color, --fresh. Single mode only. Harness gate: every single-mode snapshot case must go green.

No prior discussion in this conversation. Sources read to ground every value below: the plan's Decisions (D1–D13) and Target architecture table; the V1 memory `docs/memory/go-port/fact-and-sources.md` and the V1 code (`src/go/internal/fact`, `internal/source`, `internal/source/ccusage`, `internal/source/cache`); the harness memory `docs/memory/harness/differential-harness.md`, `harness/matrix.json`, and a live `just go-diff --placeholder --filter snapshot` run (180 cases, all red — the node-side captures under `bin/harness/report/cases/` are the reference bytes quoted in §10); the specs `docs/specs/usage.md` (§ CLI Grammar, § Global Flags, § Exit Codes, § Data Flow, § Output Formats, § Drop at cutover) and `docs/specs/layouts.md` (§1, §2, §12, §21, Color Reference); the memory `docs/memory/display/formatting.md` and `docs/memory/cli/data-pipeline.md`; and the shipped TypeScript — `src/node/core/cli.ts` (`parseGlobalFlags`, `parseDataArgs`, `main`, `dispatchAllSnapshot`, `dispatchSingleTool`, `emitJson`, `SHORT_USAGE`), `src/node/core/fetcher.ts` (`currentLabel`, `pickCurrentEntry`, `fetchTotals`, `fetchAllTotals`, `fetchHistory`, `aggregateMonthly`, `aggregateWeekly`, `weekLabel`, `filterEntriesByRange`), `src/node/core/config.ts` (`readConfig`, `selectUserConf`, `resolveConfigPaths`), `src/node/tui/formatter.ts` (`renderTotal`, `fmtNum`, `fmtCost`), `src/node/tui/colors.ts`. The TS binary was additionally probed in a scrubbed environment (`env -i`, staged `dist/` with the fake ccusage at `vendor/ccusage/bin/ccusage`, a fixture copy re-dated to today) to pin the populated-table bytes, the JSON `label` quirk and the cache behavior that the placeholder corpus cannot exercise (its dates are 2026-01-05..07, so every harness snapshot is the empty state).

Plan context that shapes this change:

- **D2** — Go lands dark in `main`. This row is the **first wiring into `cmd/tu`**: the Go binary starts answering the single-mode snapshot grammar for real while everything else keeps printing the scaffold's `tu: not implemented (Go port in progress)` (stderr, exit 1). Nothing ships; the formula is untouched.
- **D5** — layout `src/go/internal/<pkg>/`, Go 1.25, `go.mod` still `require`-free.
- **D6** — the differential harness is the gate. The row's bar is: every single-mode snapshot case in `harness/matrix.json` green under `just go-diff --placeholder`. §2 enumerates the exact case IDs.
- **Target architecture** — `query` (pure: window filter, user/tool filter, daily→weekly/monthly roll-up, one `GroupBy(dims...)`), `view` (pure: query result → table model, no ANSI), `render` (encoders from table model to `[]string`; `ansi`, `json` now; `csv`, `markdown` in B2; nothing prints), `command` (parse the positional grammar into `Request{Source, Period, Display, Format, Flags}`, compose source→query→view→render, return `Result{Lines, TotalCost, CostByItem, TotalTokens}`). I/O only at the two ends: `source` and `cmd/tu`.
- **Goal** — every external surface is frozen. V2 reproduces, byte for byte, the single-mode snapshot table, the snapshot JSON, the usage-error messages of the grammar, and the exit codes. Two *undocumented* TS behaviors were found while pinning bytes and are reproduced, then flagged for G0 (Open Questions).
- **G1** — the agent gate reviews V1+V2+B1+B2 as a unit against a checklist: package boundaries (`query`/`view`/`render` import no I/O; `command` returns a `Result`, nothing prints below `cmd/tu`), one group-by, no result globals, typed errors surfaced at the edge, table-driven `query`/`command` tests and golden-file `render` tests with an update flag. This intake records every layering choice so G1 can see it.

## Why

V1 built the input end of the pipeline (`fact`, `source`). Nothing downstream exists yet, so the layering the whole plan rests on — pure `query`/`view`/`render` stages composed by a `command` that returns values, with `cmd/tu` as the only writer — is still a diagram. V2 is the vertical slice that turns it into running code on the smallest real display: the cross-tool snapshot table (`tu`, `tu cc`, `tu m`), its JSON shape, and the grammar that reaches them. It is deliberately the narrowest end-to-end path that exercises every stage once, so G1 can judge the architecture on working code before Phase 2 stacks seven rows on it.

The TypeScript side this replaces is the tangle the plan names: `cli.ts` hand-codes 14 near-identical dispatch functions; the output channel leaks into every layer (`formatter.ts` print/render twins, 107 direct `console`/`exit` sites, results for watch travelling through module globals); tool/user/machine live as map keys and tuple shapes rather than as dimensions of one dataset, so by-tool, by-machine and by-user are three code paths doing one group-by. V2 fixes the shape without changing behavior: one `GroupBy(dims...)` over `[]fact.Record`; one table model that render encoders consume; one `command.Run` that returns lines and stats; one `run(args, stdout, stderr) int` seam in `cmd/tu` that does all the writing.

The harness makes "without changing behavior" checkable. Today all 180 snapshot cases are red because the Go side prints the placeholder. When V2 lands, the 52 single-mode snapshot cases within its grammar flip green, and the report's summary line becomes the burndown the plan tracks. Doing this as one row (rather than splitting query/view/render from command) is what makes the gate meaningful: without `command` and the `cmd/tu` wiring there is nothing for the harness to diff.

## What Changes

### 1. Package layout

```
src/go/
  cmd/tu/
    main.go                 run(): flags → version → placeholder/dispatch → write; the ONLY writer
    main_test.go            updated: {} and {"cc"} now succeed; placeholder list narrowed
  internal/
    config/                 NEW (minimal — B1 grows it)
      config.go             Paths, ResolvePaths, ParseConf, DetectMode
      config_test.go
    query/                  NEW
      period.go             Period (Daily/Weekly/Monthly), String(), CurrentLabel, WeekLabel
      query.go              Window, ByTool, ByUser, RollUp, Dim, GroupBy, Group
      *_test.go             table-driven
    view/                   NEW
      table.go              Table, Column, Row, Cell, Align, RowKind — the model; no ANSI
      snapshot.go           ToolTotals, Snapshot(rows, period) Table
      *_test.go
    render/                 NEW
      format.go             FormatInt, FormatCost (ICU half-expand rule), package render
      format_test.go
      ansi/
        color.go            Colors{Enabled}, the palette, StripANSI, PadLeft/PadRight
        table.go            Table(t view.Table, c Colors) []string
        *_test.go           golden files under testdata/, -update flag
      json/
        snapshot.go         Snapshot(rows []view.ToolTotals) []string (ordered keys, ES6 floats)
        *_test.go           golden files
    command/                NEW
      request.go            Request, Flags, Display, Format, Metric, UsageError, ShortUsage
      parse.go              Parse(args) (Request, *UsageError)  — the whole TS grammar
      run.go                Deps, Fetcher, Result, Run(ctx, req, deps) (Result, error), ErrUnported
      *_test.go             table-driven parse; fake-Fetcher run; golden lines
```

No new module dependencies; `go.mod` stays `require`-free (`time/tzdata` is a stdlib embed, §8). `just go-build` / `go-test` / `go-lint` and the `go-build-and-test` CI lane pick the packages up through `./...`. No justfile, workflow, `src/node/`, spec, or `harness/` edit.

### 2. Scope: "single mode, bare grammar" made precise, and the harness gate

**In scope (produces real output)** — a data command whose parsed `Request` has: `Display == Snapshot`; `Format ∈ {Table, JSON}`; `Source` any of the six tools or all; `Period` any of daily/weekly/monthly; flags limited to `--json`/`-j`, `--fresh`/`-f`, `--no-color`, `-t`, `--metric <cost|tokens>`; and the process is in **single mode** (§7). Plus every **usage error** the grammar itself produces (§8.2), and the `$HOME`-unset error (§7).

**Recognized but unported (placeholder)** — everything else the parser accepts stays on the scaffold's path: stderr `tu: not implemented (Go port in progress)`, stdout empty, exit 1. Concretely: multi mode (a non-empty `metrics_repo` or `TU_METRICS_REPO`); displays `h`/`history`/`dh`/`wh`/`mh`/`lb`/`lbh`; formats `--csv`/`--md`; flags `--by-machine`, `--since`/`-s`, `--until`, `--full`, `--top`, `--user`/`-u`, `--sync`, `--dry-run`, `--watch`/`-w`, `--no-rain`, `--skip-brew-update`; and the non-data commands (`help`/`-h`/`--help` as first arg, `help-dump`, `skill`, `shell-init`, `update`, `init-conf`, `init-metrics`, `sync`, `status`). Each of these belongs to a named later row (B1 config/setup, B2 history + csv/md, B3 multi mode, B4 machines, B5 leaderboard, B6 sync, B7 watch, B8 toolkit) and stays red in the harness *by design*, which is what G1 checklist item 6 asks for ("every remaining red case is explained by a not-yet-ported row"). Note: `--interval N`/`-i N` is only validated when `--watch` is present; without it the TS silently drops the flag and value and renders normally, and so does the Go parser (`tu --interval 3` is a real snapshot, not a placeholder) — a later row must not "fix" that into a divergence. Likewise the TS warn-and-clear guards for `--since`/`--until`, `--full`, `--top` and single-mode `-u` on a snapshot WARN on stderr and still render the table with exit 0; the rows that port those flags (B2, B3, B5) must implement warn-and-render for snapshot displays, not an error.

**The gate.** `just go-diff --placeholder --filter snapshot` lists 58 case IDs with `conf: single` and `env ≠ envrepo`. Six of them carry a format or flag owned by another row (`snapshot-all-csv`, `snapshot-cc-csv`, `snapshot-all-md`, `snapshot-cc-md` → B2; `snapshot-all-by-machine`, `snapshot-cc-by-machine` → B4). The remaining **52 must be green**:

```
snapshot-all/single/{default,nocolor}/{pipe,tty}/{fixed,alt}          (8)
snapshot-cc/single/{default,nocolor}/{pipe,tty}/{fixed,alt}           (8)
snapshot-{codex,co,oc,gemini,gem,copilot,cop,kimi,ki}/single/default/{pipe,tty}/fixed   (18)
snapshot-{w,m,cc-m}/single/default/{pipe,tty}/fixed                   (6)
snapshot-all-{json,json-short,truncate,metric-tokens,metric-cost,fresh}/single/default/pipe/fixed   (6)
snapshot-cc-{json,json-short,truncate,metric-tokens,metric-cost,fresh}/single/default/pipe/fixed    (6)
```

Every one of these renders the **empty state** under the placeholder corpus (fixture dates are 2026-01-05..07, snapshots are "today"), so the populated table and the populated JSON are covered by golden-file unit tests with an injected clock (§11) and — on dev-ws-sahil02, where a local capture exists — by `just go-diff --filter snapshot` without `--placeholder`. The usage-error groups (`bogus`, `cc-codex`, `cc-help`, `json-csv`, `watch-json`, `metric-*`, `truncate-metric`, `since-invalid`, `window-reversed`, `interval-*`, `until-missing`, `user-missing`, `top-invalid`) fall out of the complete parser (§8.2) and are expected to go green too, but they are a bonus, not the gate.

### 3. `query` — pure filters, roll-up, and the one group-by

```go
package query

type Period int
const (Daily Period = iota; Weekly; Monthly)
func (p Period) String() string          // "daily" | "weekly" | "monthly" — the heading text

// CurrentLabel is the period's label for `now` in now's LOCATION (local time,
// mirroring the TS currentLabel): daily "2006-01-02"; weekly the ISO date of
// the current week's Sunday, now.AddDate(0,0,-int(now.Weekday())); monthly "2006-01".
func CurrentLabel(p Period, now time.Time) string

// WeekLabel maps a daily ISO label to its week's Sunday using UTC date
// arithmetic on the date-only string (DST-immune, mirrors the TS weekLabel).
// A label that does not parse as 2006-01-02 is returned unchanged (its own bucket).
func WeekLabel(daily string) string

// Window keeps records with since <= Date <= until (lexicographic on ISO labels,
// inclusive; an empty bound is open on that side). Pure.
func Window(recs []fact.Record, since, until string) []fact.Record

// ByTool / ByUser keep records whose Tool / User equals key. Pure.
func ByTool(recs []fact.Record, key string) []fact.Record
func ByUser(recs []fact.Record, user string) []fact.Record

// RollUp re-labels each record to its period bucket (daily: identity; weekly:
// WeekLabel(Date); monthly: Date[:7]) and sums Totals over records sharing
// (Date', Tool, User, Machine). Output ascending by Date then first-seen order
// within a label. Pure; daily returns a copy, never the input slice.
func RollUp(recs []fact.Record, p Period) []fact.Record

type Dim int
const (Date Dim = iota; Tool; User; Machine)

// Group is one bucket of GroupBy: Key carries only the grouped dims (other
// string fields empty), Totals is the field-wise sum.
type Group struct { Key fact.Record; fact.Totals }

// GroupBy is the ONE group-by (plan: tool, machine and user pivots share it;
// G1 checklist item 2). Groups appear in first-seen input order, so a registry-
// ordered input yields registry-ordered groups. Pure.
func GroupBy(recs []fact.Record, dims ...Dim) []Group
```

- The snapshot for period `p` is the composition `GroupBy(Window(RollUp(recs, p), cur, cur), Tool)` with `cur = CurrentLabel(p, now)`. History (B2) is `RollUp` + `Window` + `GroupBy(Date)` / `GroupBy(Date, Tool)`; machine columns (B4) `GroupBy(Tool, Machine)`; leaderboards (B5) `GroupBy(User)`. No per-dimension aggregation code anywhere.
- Roll-up sums all six `Totals` fields (`Totals.Add`), as `aggregateMonthly`/`aggregateWeekly` do; `TotalTokens` is summed, not recomputed.
- Sort order: the TS sorts labels with `localeCompare`; on ISO labels (digits and hyphens) that equals byte order, so Go uses `sort.SliceStable` on `Date`.
- `CurrentLabel` takes `now time.Time` and uses `now.Location()`; the edge passes `time.Now()` (Go's `time.Local` honors `TZ`, so `TZ=Asia/Kolkata` gives the same local date the TS computes — the harness `tz: alt` axis).

### 4. `view` — the table model (no ANSI)

```go
package view

type Align int   // Left, Right
type RowKind int // Header, Divider, Data, Total

type Column struct { Title string; Width int; Align Align }
type Cell   struct { Text string; Dim bool }          // Dim: exact-zero cell styling (pivot/machine columns, B2/B4); false everywhere in the snapshot
type Row    struct { Kind RowKind; Cells []Cell; Bar string; Delta string } // Bar/Delta: reserved slots, empty in V2 (B2 bars, B7 deltas)
type Table  struct {
    Title  string   // "📊 Combined Usage (daily)"
    Columns []Column
    Rows   []Row    // header, divider, data…, [divider, total]; nil when Empty != ""
    Empty  string   // "  No usage" when no row is visible, else ""
    Legend string   // reserved (B4 machine legend); ""
    Footer string   // reserved (B2 summary footer); ""
}

// ToolTotals is one snapshot row candidate: the display name (resolved by
// command from the registry — view knows no registry), the current label when
// the tool had a matching record (used by render/json), and its totals.
type ToolTotals struct { Name, Label string; fact.Totals }

// Snapshot builds the cross-tool snapshot table (layouts §1/§2).
func Snapshot(rows []ToolTotals, p query.Period) Table
```

`Snapshot` rules, lifted from `renderTotal`:

- Title `📊 Combined Usage ({p.String()})` — also for a single-source snapshot (DC-15: never the tool name).
- Columns: `Tool` (12, Left), `Tokens`, `Input`, `Output`, `Cache`, `Cost` (12, Right each). Fixed, never data-sized (DC-24: a wider value overflows its cell).
- A data row per input row with `TotalTokens > 0`, in input order: `Name`, `FormatInt(TotalTokens)`, `FormatInt(InputTokens)`, `FormatInt(OutputTokens)`, `FormatInt(CacheCreationTokens + CacheReadTokens)`, `FormatCost(TotalCost)`.
- `Empty = "  No usage"` (two leading spaces) when **every** input row has `TotalTokens == 0`; then no header/divider rows.
- Divider + Total row only when **more than one** row is visible. The Total sums **every** input row, visible or not (the TS accumulates `grandCost += t.totalCost` before checking visibility; a tool with cost but zero tokens is hidden yet counted).
- Token mode (`-t`, `--metric tokens`) changes nothing in the one-shot table (the delta indicator is watch-only, B7).

### 5. `render/ansi` — primitives and the table encoder

```go
package ansi

type Colors struct { Enabled bool }     // a VALUE, not a module global: the TS setNoColor()/NO_COLOR check becomes a parameter
func (c Colors) BoldWhite(s string) string   // "\x1b[1;37m" + s + "\x1b[0m" when Enabled, else s
func (c Colors) BoldCyan(s string) string    // "\x1b[1;36m"
func (c Colors) Dim(s string) string         // "\x1b[2m"
// and the rest of the layouts.md Color Reference, defined now for B2/B4/B5/B7:
// Bold "\x1b[1m", Green "\x1b[32m", Red "\x1b[31m", Cyan "\x1b[36m", Yellow "\x1b[33m",
// Magenta "\x1b[35m", Blue "\x1b[34m", BrightGreen "\x1b[92m", DimGreen "\x1b[2;32m"; reset "\x1b[0m"

func StripANSI(s string) string              // regexp `\x1b\[[0-9;]*m` → ""
func PadRight(s string, w int) string        // TS padEnd  — pads by rune count (all snapshot text is ASCII; the 📊 title is never padded)
func PadLeft(s string, w int) string         // TS padStart

// Table encodes a view.Table into output lines exactly as renderTotal does:
//   "", BoldWhite(title), "",
//   if Empty: Empty, ""
//   else: header (each cell padded then BoldCyan-wrapped INDIVIDUALLY, joined " | "),
//         Dim(divider), data rows plain (padded cells joined " | "),
//         [Dim(divider), total (each cell padded then BoldWhite-wrapped, joined " | ")],
//         [Legend/Footer when non-empty — unused in V2], ""
// The divider is "─"×w per column joined with "─|─" (87 visible chars for the snapshot).
func Table(t view.Table, c Colors) []string
```

- `Colors.Enabled = !flags.NoColor && os.Getenv("NO_COLOR") == ""`, computed once in `cmd/tu` and passed down. `--no-color` and a non-empty `NO_COLOR` produce byte-identical output (spec § Terminal width and color). Color is emitted whether or not stdout is a TTY — no `isatty` probing anywhere in V2.
- Nothing in `render/…` imports `os`, `fmt.Print*`, or `io`; encoders return `[]string`.

**Number formatting** lives in the parent `package render` (`render/format.go`) so `ansi` now and `markdown` (B2) share it:

- `FormatInt(n int64) string` — en-US thousands grouping (`24,400`, `1,234,567,890,123`), the TS `fmtNum` on integers.
- `FormatCost(x float64) string` — `"$" + …` with **exactly two decimals and grouping**, reproducing `toLocaleString("en-US", {minimumFractionDigits: 2, maximumFractionDigits: 2})`. This is NOT `strconv.FormatFloat(x, 'f', 2, 64)`: ICU rounds the **shortest round-trip decimal representation** of the double with **half-expand** (away from zero), whereas Go's `'f'` rounds the exact binary value half-even. Verified 2026-09-16 with node v24 against Go 1.26: `1.005 → $1.01` (Go `1.00`), `0.125 → $0.13` (Go `0.12`), `0.015 → $0.02` (Go `0.01`), `2.675 → $2.68` (Go `2.67`), `999999.995 → $1,000,000.00` (Go `999999.99`), `4.936068800000001 → $4.94`, `1234567.891 → $1,234,567.89`. Algorithm: `s := strconv.FormatFloat(x, 'f', -1, 64)` (shortest repr — Go's and V8's shortest-digit algorithms agree), round the decimal string to two fraction digits half-away-from-zero with carry (`0.995 → 1.00`), then group the integer part. Negative zero renders `$0.00` (the TS gives `$-0.00`, unreachable — costs are non-negative sums; the JSON encoder likewise normalizes `-0` to `0`, §6).

### 6. `render/json` — the snapshot object

```go
package json
// Snapshot renders the snapshot JSON (layouts §12, DC-01): an object keyed by
// display name in the given order; per tool, "label" FIRST when Label != "",
// then the six pinned totals keys; two-space indent; one trailing newline
// (console.log). Returned as lines so command.Result is uniform.
func Snapshot(rows []view.ToolTotals) []string
```

- Byte layout reproduces `JSON.stringify(obj, null, 2)`: `{`, `  "Claude Code": {`, `    "label": "2026-09-16",`, `    "totalCost": 0.5,` … `    "totalTokens": 24400`, `  },` … `}`. Built with an ordered writer (Go maps are unordered; `encoding/json` on a struct cannot omit `label` conditionally without a pointer and cannot reorder). Strings and floats are encoded through `encoding/json` (`Marshal(float64)` follows the ES6 rules: shortest repr, `1e+21`/`1e-7` exponent thresholds; `SetEscapeHTML(false)` is irrelevant for display names but set on any encoder used). `-0` is normalized to `0` before encoding (Go writes `-0`, V8 writes `0`).
- The **all-tools** command emits every registry tool in registry order (zero totals, no `label`, for tools without a current-period record); the **single-source** command emits only that tool. Which rows carry a `Label` is decided by `command` (§8.3, the daily-all quirk).

### 7. `config` — the minimal cascade (mode detection only; B1 grows it)

```go
package config

type Paths struct { Home, ConfigDir, UserConf, OrgConf, LegacyConf string }

// ResolvePaths mirrors the TS resolveConfigPaths: built from $HOME and nothing
// else. An unset/empty HOME returns ErrNoHome whose Error() is the byte-exact
// "tu: $HOME is not set; cannot locate config" (the edge prints it, exit 1).
func ResolvePaths(home string) (Paths, error)

// ParseConf is the TS parseConf: per line trim; skip blank and '#' lines; split
// at the FIRST '='; trim key and value; later keys overwrite earlier ones.
func ParseConf(raw string) map[string]string

type Mode int
const (Single Mode = iota; Multi)

// DetectMode derives the mode exactly as readConfig does: TU_METRICS_REPO
// non-empty → Multi; else merge org.conf then the user conf (tu.conf, falling
// back to legacy ~/.tu.conf when tu.conf is unreadable) and Multi iff the
// merged metrics_repo is non-empty; else Single. Unreadable files are empty maps.
func DetectMode(p Paths, getenv func(string) string) Mode
```

- Paths: `$HOME/.config/tu/tu.conf`, `$HOME/.config/tu/org.conf`, `$HOME/.tu.conf`. `tu.default.conf` is not read (it carries no `metrics_repo`; B1 adds the defaults layer with `metrics_dir`, `machine`, `user`, `auto_sync`, the version warning, the legacy deprecation line and sentinel expansion).
- V2 emits nothing from config: the legacy-conf deprecation warning, the `version newer than tu supports` warning and the metrics-dir auto-clone guard are B1/B3 behavior, and every harness variant that exercises them is multi mode (placeholder) anyway.
- In single mode `Source{User, Machine}` stay empty (V1 takes them as plain fields; nothing in the snapshot displays them).

### 8. `command` — parse, compose, return

#### 8.1 Request

```go
package command

type Display int // Snapshot, History, Leaderboard, LeaderboardHistory
type Format  int // Table, JSON, CSV, Markdown
type Metric  int // Cost, Tokens

type Flags struct {
    Fresh, NoColor, Watch, Sync, DryRun, ByMachine, Full, NoRain, SkipBrewUpdate bool
    Interval int          // 10 default; validated only when Watch
    User, Since, Until string
    Metric Metric
    Top int               // 0 = unset
}
type Request struct {
    Source  string        // "" = all, else registry key (aliases resolved: co→codex, gem→gemini, cop→copilot, ki→kimi)
    Period  query.Period
    Display Display
    Format  Format
    Flags   Flags
    Command string        // first positional when it is a non-data command (help, init-conf, …); "" for data commands
    Version bool          // --version / -V / -v seen anywhere
}

type UsageError struct { Message string; ShowUsage bool } // exit 2; ShowUsage appends ShortUsage on stderr
const ShortUsage = "Usage: tu [source] [period] [display]\n\n  tu                Today's cost, all tools\n  tu cc             Today's cost, Claude Code\n  tu mh             Monthly cost history, all tools\n  tu -h             Show full help\n\nRun 'tu help' for all commands."

func Parse(args []string) (Request, *UsageError)
```

#### 8.2 Parse — the complete TS grammar, byte-exact errors

`Parse` is `parseGlobalFlags` + the version/help/non-data checks + `parseDataArgs`, in the TS order, so the same argv produces the same first error:

1. **Flag pass** over `args` in order. Boolean flags are stripped: `--json`, `-j`, `--csv`, `--md`, `--sync`, `--dry-run`, `--fresh`, `-f`, `--watch`, `-w`, `--no-color`, `--no-rain`, `--by-machine`, `--full`, `--skip-brew-update`, `-t`. Value flags consume the next token only when it exists and (a) matches `^\d+$` for `--interval`/`-i`, or (b) does not start with `-` for `--user`/`-u`, `--since`/`-s`, `--until`, `--metric`, `--top`; the flag is remembered as present even when no value was consumed. Everything else lands in the positional list (including `--help` when it is not the first token, and unknown flags like `--bogus`).
2. **Validation, in this order, first failure wins** (message → stderr, exit 2, no usage block):
   - `--interval` (only when `--watch`): missing/non-numeric value `Error: --interval requires a numeric value`; `< 5` `Error: --interval minimum is 5 seconds`; `> 3600` `Error: --interval maximum is 3600 seconds`.
   - Format conflicts, checked in this order: watch+json `Error: --watch and --json are incompatible`; json+csv `Error: --json and --csv are incompatible`; json+md `Error: --json and --md are incompatible`; csv+md `Error: --csv and --md are incompatible`; watch+csv `Error: --watch and --csv are incompatible`; watch+md `Error: --watch and --md are incompatible`. (`-j` counts as `--json`; the message always says `--json`.)
   - `-u` present without value: `Error: -u requires a username`.
   - `--since` present: value must be `YYYY-MM-DD` or `YYYYMMDD` (normalized to dashed); else `Error: --since requires a date (YYYY-MM-DD or YYYYMMDD)`. Same for `--until` with its own name. Shape-only — `2026-13-01` is accepted.
   - both present and `since > until` (string compare): `Error: --since must be on or before --until`.
   - `--metric` present: value must be `tokens` or `cost`, else `Error: --metric requires 'tokens' or 'cost'`.
   - `-t` with an explicit `--metric cost`: `Error: -t and --metric cost are incompatible`; `-t` alone or with `--metric tokens` sets `Metric = Tokens`.
   - `--top` present: value must be `^\d+$` and `≥ 1`, else `Error: --top requires a positive integer`.
   - `Format`: json > csv > md > table, in that precedence (only one can survive validation anyway).
3. **Version**: `--version`, `-V` or `-v` anywhere in the original args → `Request{Version: true}`. This runs **after** step 2, as in `main()`: `tu --json --csv --version` is exit 2, not a version line. (The scaffold checked version first; V2 reorders.)
4. **Non-data commands**: first positional ∈ {`help`, `-h`, `--help`, `init-conf`, `init-metrics`, `sync`, `status`, `update`, `shell-init`, `help-dump`, `skill`} → `Request{Command: tok}` (placeholder in V2, §9). The `--dry-run`-off-`sync` guard and `update --help` belong to B6/B8 and stay behind the placeholder.
5. **Positionals** (`parseDataArgs`): if the first positional is one of `cc codex co oc gemini gem copilot cop kimi ki all` it is the source (alias-resolved; `all` → `""`). Each remaining token: `d`/`daily`, `w`/`weekly`, `m`/`monthly` set Period; `h`/`history` History; `lb` Leaderboard; `lbh` LeaderboardHistory; `dh`/`wh`/`mh` set Period and History. Anything else — a second source token (`tu cc codex`), a source after a period (`tu m cc`), an unknown word, an unknown flag, or `--help` after a positional (DC-04) — is `Unknown argument: {tok}` with `ShowUsage: true` (message line, then `ShortUsage`, then newline — the TS prints two `console.error` calls), exit 2.

Observed reference (node v24, TS v0.11.5): `tu bogus` → stderr `Unknown argument: bogus\n` + ShortUsage + `\n`, exit 2; `tu cc codex` → `Unknown argument: codex`; `tu cc --help` → `Unknown argument: --help`; `tu --json --csv` → `Error: --json and --csv are incompatible\n`; `tu -t --metric cost` → `Error: -t and --metric cost are incompatible\n`; `tu --metric` and `tu --metric foo` → `Error: --metric requires 'tokens' or 'cost'\n`. Stdout is empty on every error path.

#### 8.3 Run — compose source → query → view → render

```go
// Fetcher is what command needs from a source; *ccusage.Source satisfies it.
type Fetcher interface {
    Fetch(ctx context.Context, tool ccusage.Tool, period string, extraArgs []string, fresh bool) ([]fact.Record, *source.Error)
    FetchAll(ctx context.Context, period string, extraArgs []string, fresh bool) ([]fact.Record, []*source.Error)
}
type Deps struct {
    Source Fetcher
    Now    func() time.Time   // time.Now at the edge; fixed in tests
    Colors ansi.Colors
}
type Result struct {
    Lines       []string          // stdout, one per line, no trailing "\n" per element
    Warnings    []*source.Error   // the edge writes them with source.WriteWarnings
    TotalCost   float64           // sum over the rendered rows (watch stats, B7)
    TotalTokens int64
    CostByItem  map[string]float64 // display name → cost (the TS _lastRenderCostMap, snapshot keys)
}
var ErrUnported = errors.New("not implemented")   // main maps it to the placeholder line, exit 1

func Run(ctx context.Context, req Request, mode config.Mode, deps Deps) (Result, error)
```

`Run` for an in-scope request (§2), otherwise `ErrUnported`:

1. `ctx, cancel := context.WithTimeout(ctx, ccusage.DefaultTimeout)` — the V1-exported deadline is applied here, once, for all tools.
2. Fetch **daily only** (`ccusage.PeriodDaily`, no extra args — the TS only ever calls `daily`): `FetchAll` for all tools, `Fetch(tool)` for a single source. Registry-ordered records and errors, as V1 guarantees. `fresh = req.Flags.Fresh`.
3. `cur := query.CurrentLabel(req.Period, deps.Now())`; `groups := query.GroupBy(query.Window(query.RollUp(recs, req.Period), cur, cur), query.Tool)`.
4. Build `[]view.ToolTotals` over the **registry order** (all six for the all-tools command, the one tool for a single source), `Name = tool.Name` via `ccusage.Lookup`, `Totals` from the matching group (zero when absent), `Label = cur` when a group matched else `""`.
5. **The single-mode daily-all quirk (reproduced, flagged in Open Questions).** In the TS, `dispatchAllSnapshot` in single mode with `period === "daily"` goes through `fetchAllTotals → fetchTotals → pickCurrentEntry → toUsageTotals`, which returns **bare totals without a label**, so `tu --json` never carries `"label"` even for a tool with today's data — while `tu cc --json` (any period) and `tu m --json` / `tu w --json` (non-daily, all tools) go through `fetchHistory` → `UsageEntry` and do carry it. Verified 2026-09-16 with a fixture re-dated to today: `tu --json` → Claude Code `{"totalCost": 0.5, …}` with no label; `tu cc --json` → `{"label": "2026-09-16", "totalCost": 0.5, …}`; `tu m --json` → `"label": "2026-09"`; `tu w --json` → `"label": "2026-09-13"`. `Run` therefore clears every `Label` when `req.Source == "" && req.Period == query.Daily`, in one commented line next to the JSON branch. The table is unaffected (it never shows the label).
6. Render: `Format == JSON` → `json.Snapshot(rows)`; else `ansi.Table(view.Snapshot(rows, req.Period), deps.Colors)`.
7. `Result.TotalCost`/`TotalTokens`/`CostByItem` summed over the rows (the TS `sumToolTotalsCost` / `buildCostMap(result)` keyed by display name).

**Cache policy (decision).** The same TS quirk means bare `tu`/`tu --json`/`tu --fresh` in single mode **never read or write `~/.tu/cache`** (`fetchTotals` bypasses it; `--fresh` is a no-op there), while `tu cc`, `tu m`, `tu w` cache per tool. Verified: after `tu`, no cache dir; after `tu cc`, `cc-daily.json`; after `tu m`, all six files. This is not an external surface (the Goal's list does not include the cache; the harness diffs stdout/stderr/exit and stages a fresh HOME per case) and is the opposite of Principle IV's "heavy operations MUST be cached". Go caches **uniformly**: `Run` always hands V1's `Source{Cache: cache.Default()}` (nil when `$HOME` is empty — unreachable after the HOME check) and `fresh` skips the read on every path. The behavior is recorded as a drop-at-cutover candidate for G0 (Open Questions), not silently changed.

### 9. `cmd/tu` — the only writer

`run(args []string, stdout, stderr io.Writer) int` becomes:

1. `req, uerr := command.Parse(args)`; on `uerr`: `fmt.Fprintln(stderr, uerr.Message)`, then `fmt.Fprintln(stderr, command.ShortUsage)` when `ShowUsage`; return 2.
2. `req.Version` → the existing `versionLine(version)` on stdout, return 0 (unchanged output).
3. `req.Command != ""` → placeholder (`notImplementedMsg` on stderr, return 1).
4. `paths, err := config.ResolvePaths(os.Getenv("HOME"))`; on error print `err.Error()` (`tu: $HOME is not set; cannot locate config`) to stderr, return 1 — the TS `main().catch` path for the `ConfigHomeError`, which runs **after** the grammar parse (so `tu bogus` with `HOME=` is still exit 2).
5. `mode := config.DetectMode(paths, os.Getenv)`.
6. `store, _ := cache.Default()`; `src := &ccusage.Source{Cache: store}` (binary resolution, user/machine per V1 defaults); `deps := command.Deps{Source: src, Now: time.Now, Colors: ansi.Colors{Enabled: !req.Flags.NoColor && os.Getenv("NO_COLOR") == ""}}`.
7. `res, err := command.Run(ctx, req, mode, deps)`; `errors.Is(err, command.ErrUnported)` → placeholder, return 1; any other error → its text on stderr, return 1 (the TS catch-all).
8. `source.WriteWarnings(stderr, res.Warnings)`; then `fmt.Fprintln(stdout, l)` for each line; return 0.

The placeholder constant and exit code are unchanged so every unported case stays red with the same signature it has today. `main_test.go`: `TestRunNotImplemented` drops `{}` and `{"cc"}` from its list (they now need a source and are covered by the new end-to-end test, §11) and keeps `{"--help"}` and `{"m","dh","--json"}`; a new case asserts `tu --json --csv --version` is exit 2. `import _ "time/tzdata"` is added to `main.go` so `TZ=Asia/Kolkata` resolves even on a host without `/usr/share/zoneinfo` (Go consults the embed only when the system database is missing; ~450 KB, no startup cost).

### 10. Byte-exact references (from the TS binary, 2026-09-16)

Empty state, `tu` in single mode, color on (`\x1b` shown as `ESC`; every line ends `\n`):

```
(blank)
ESC[1;37m📊 Combined Usage (daily)ESC[0m
(blank)
  No usage
(blank)
```

With `--no-color` or `NO_COLOR=1` the escapes vanish and nothing else changes. `tu m` / `tu w` say `(monthly)` / `(weekly)`. Under the pty (`io: tty`) both binaries produce `\r\n`, so nothing tty-specific is needed.

Populated, `tu` with two tools having today's data (fixture totals `0.5 / 3000 / 400 / 1000 / 20000 / 24400`):

```
(blank)
ESC[1;37m📊 Combined Usage (daily)ESC[0m
(blank)
ESC[1;36mTool        ESC[0m | ESC[1;36m      TokensESC[0m | ESC[1;36m       InputESC[0m | ESC[1;36m      OutputESC[0m | ESC[1;36m       CacheESC[0m | ESC[1;36m        CostESC[0m
ESC[2m─────────────|──────────────|──────────────|──────────────|──────────────|─────────────ESC[0m
Claude Code  |       24,400 |        3,000 |          400 |       21,000 |        $0.50
Codex        |       24,400 |        3,000 |          400 |       21,000 |        $0.50
ESC[2m─────────────|──────────────|──────────────|──────────────|──────────────|─────────────ESC[0m
ESC[1;37mTotal       ESC[0m | ESC[1;37m      48,800ESC[0m | ESC[1;37m       6,000ESC[0m | ESC[1;37m         800ESC[0m | ESC[1;37m      42,000ESC[0m | ESC[1;37m       $1.00ESC[0m
(blank)
```

(The divider is twelve `─`, then five groups of `─|─` + twelve `─`: 87 visible characters. Header and Total cells are padded first, then wrapped individually; the ` | ` separators are unstyled. Data rows carry no escapes.) `tu cc` renders one data row and no divider/Total.

JSON, `tu --json` (all tools, daily, single mode — note **no `label`** on the populated tools, §8.3):

```
{
  "Claude Code": {
    "totalCost": 0.5,
    "inputTokens": 3000,
    "outputTokens": 400,
    "cacheCreationTokens": 1000,
    "cacheReadTokens": 20000,
    "totalTokens": 24400
  },
  "Codex": {
    …same…
  },
  "OpenCode": {
    "totalCost": 0,
    "inputTokens": 0,
    "outputTokens": 0,
    "cacheCreationTokens": 0,
    "cacheReadTokens": 0,
    "totalTokens": 0
  },
  "Gemini": { …zeros… },
  "Copilot": { …zeros… },
  "Kimi": { …zeros… }
}
```

`tu cc --json` → `{ "Claude Code": { "label": "2026-09-16", "totalCost": 0.5, … } }` (label first); `tu m --json` → every tool, `"label": "2026-09"` on populated ones; `tu w --json` → `"label": "2026-09-13"` (the week's Sunday). Under the placeholder corpus every snapshot JSON is the all-zero shape (`snapshot-all-json/single/default/pipe/fixed` node capture: six zero objects, no labels).

### 11. Tests

- **`query`** (table-driven): `Window` inclusive bounds and open sides; `RollUp` weekly with Sunday labels across a month and a year boundary, monthly, daily copy; sums of all six fields; ascending output; input slices unchanged (purity); `GroupBy(Tool)` first-seen order and `GroupBy(Tool, Machine)`; `CurrentLabel` for the three periods with a fixed `now` in `Asia/Kolkata` and in `UTC`, including a Sunday and a month-start Wednesday (week Sunday in the previous month); `WeekLabel` passthrough on a malformed label.
- **`view`**: row omission (`TotalTokens == 0`), Total only for `>1` visible rows, hidden-row cost counted in Total, empty state, heading per period, fixed 12-wide columns and overflow.
- **`render`**: `FormatInt` grouping; `FormatCost` table with the ICU-parity values in §5 (`1.005`, `0.125`, `0.015`, `2.675`, `999999.995`, `4.936068800000001`, `1234567.891`, `0`, `-0`).
- **`render/ansi`**: golden files under `testdata/` for populated color, populated no-color, single-row, and empty (`-update` flag regenerates, G1 item 5); `StripANSI(colored) == nocolor` on every golden; palette codes match the Color Reference.
- **`render/json`**: goldens for all-tools with two populated (labels present and absent), single tool, all-zero; float encoding of `0.5`, `4.936068800000001`, `0`, `-0 → 0`.
- **`command`**: `Parse` table over every argv in `harness/matrix.json` that the grammar reaches, asserting the parsed `Request` or the exact `UsageError` (message, `ShowUsage`) — the byte-exact strings in §8.2; `Run` with an in-memory `Fetcher` fake (records handed back, calls recorded) and a fixed `Now = 2026-01-06T12:00:00+05:30`: lines equal the `render` goldens, the daily-all label drop, single-source fetches exactly one tool, `--fresh` reaches the fake as `fresh=true`, `ErrUnported` for each placeholder class in §2, `Result` stats; `config.DetectMode` over temp HOMEs (`tu.conf`, `org.conf`, legacy, env, none) and `ResolvePaths("")`.
- **`cmd/tu`**: `main_test.go` updated (§9) plus an end-to-end test in the V1 style — `TestMain` builds `../fakeccusage`, sets `TUDIFF_FIXTURES` to `_placeholder` only, `HOME` to a temp dir, `TZ=UTC`, `NO_COLOR` unset — asserting the empty-state bytes of §10 for `tu`, `tu cc`, `tu m`, `tu --json`, `tu --no-color`, and exit 2 + stderr for `tu bogus`.
- **Harness gate** (acceptance, run during apply and quoted in the PR): `just go-diff --placeholder --filter snapshot` reports every one of the 52 IDs in §2 `GREEN`; the six csv/md/by-machine single cases and every multi/envrepo case remain `RED`; `just go-lint` and `just go-test` are clean. On dev-ws-sahil02, `just go-diff --filter snapshot` (local capture) SHOULD also be green — it is the only automated run of the populated path against real bytes.

### 12. Explicitly not in V2

No `render/csv`, `render/markdown`, history, bars, footers, month separators, deltas (B2); no `--by-machine`, machine legend, stacked bars (B4); no `source/metrics`, multi-mode merge, `-u` (B3); no leaderboards (B5); no sync, `--dry-run` guard (B6); no watch, compositor, rain (B7); no help text, `help-dump`, `skill`, `shell-init`, `update`, completions (B8); no full config cascade, `init-conf`, `status`, `init-metrics`, the legacy deprecation warning, sentinels (B1). No edit to `src/node/**`, `docs/specs/*`, `justfile`, `.github/workflows/*`, `harness/**`, `Formula/`, `package.json`, or the plan document (the operator updates row status).

## Affected Memory

- `go-port/query-view-render`: (new) the `query` API (`Period`, `CurrentLabel` local-time semantics, `WeekLabel` UTC arithmetic, `Window`, `RollUp` grouping key and sort, the one `GroupBy(dims...)` with first-seen order); the `view.Table` model and the snapshot rules (row omission, Total-when->1, hidden cost counted, empty text, fixed widths); `render.FormatInt`/`FormatCost` with the ICU half-expand-on-shortest-repr rule and the verified divergence table; `render/ansi` `Colors` value, palette, `StripANSI`, the table line layout; `render/json` ordered object, conditional `label`, ES6 float encoding, `-0` normalization; golden-file test convention (`testdata/`, `-update`). Design Decisions: `Colors` as a value not a global; number formatting in the parent `render` package; view receives display names (no registry import); `GroupBy` first-seen order; reserved `Bar`/`Delta`/`Legend`/`Footer` slots.
- `go-port/command-edge`: (new) `Request`/`Flags`, the complete `Parse` (flag pass, validation order, byte-exact messages, `ShortUsage`, version-after-validation, non-data tokens, positional grammar and aliases); `Run` composition (daily-only fetch, `DefaultTimeout` applied once, registry-ordered rows, the daily-all label drop, uniform caching, `Result` stats); `ErrUnported` and the placeholder policy with the row map from §2; the minimal `config` (`Paths`, `ParseConf`, `DetectMode`, `ErrNoHome`); the `cmd/tu` write order (usage → version → placeholder → HOME → warnings → lines); `time/tzdata`; the 52-case harness gate and its empty-state caveat. Design Decisions: reproduce the JSON label quirk / do not reproduce the cache quirk; full parser in one row; placeholder for recognized-unported requests instead of `Unknown argument`.
- `go-port/fact-and-sources`: (modify) the description and Overview no longer say "unwired into cmd/tu"; note that `DefaultTimeout`, `WriteWarnings`, `cache.Default` and `FetchAll` are consumed by `command`/`cmd/tu` as of V2.
- `build/toolchain`: (modify) the `cmd/tu` bullet — the entry point now dispatches the single-mode snapshot grammar and prints the placeholder only for unported requests; `go-diff --placeholder` has its first green cases (52 snapshot IDs) and the `go-diff` job stays informational.

`harness/differential-harness`, `display/formatting`, `cli/data-pipeline` are not modified: no harness code changes, and the two TS memories describe the shipped binary, which V2 does not touch. The two spec corrections in Open Questions are human edits at G0, not hydrate.

## Impact

- **New Go code**: `internal/{config,query,view,render,render/ansi,render/json,command}` plus `cmd/tu` changes; roughly 900–1,200 lines of implementation and 900–1,100 lines of tests and goldens. Sits at the top of the plan's M size; if apply finds it growing past that, the natural split is `config` + the parser's validation table into a follow-up, never the pipeline stages.
- **Dependencies**: none; stdlib only (`time/tzdata` embed adds ~450 KB to `bin/tu`).
- **Harness**: 52 cases flip green; the `go-diff` CI job (informational, `continue-on-error`) starts producing a non-zero green count; no workflow edit.
- **Downstream rows**: B2 adds `render/csv`, `render/markdown`, history views on `GroupBy(Date…)`, `Window(since, until)` already present, and the `Bar`/`Footer` slots; B1 grows `config` in place; B3 supplies multi-mode records to the same `Run`; B4 uses `GroupBy(Tool, Machine)` and the `Legend` slot; B7 consumes `Result.TotalCost/TotalTokens/CostByItem`. B2 must NOT reuse `render.FormatCost` for CSV: `csvCost` is `toFixed(2)`, which rounds the exact binary value half-up (`1.005 → 1.00`, `0.125 → 0.13`) — a third rule.
- **Risk**: the populated snapshot path is verified in CI only through goldens (placeholder dates never hit "today"); the local-capture harness run on dev-ws-sahil02 is the real-bytes check and depends on that machine having usage today. The ICU rounding rule is the one place a subtle divergence could hide; the golden values in §5 were produced by the actual node runtime.

## Open Questions

- **Spec correction / new DC candidate (human, G0)**: `docs/specs/usage.md` § JSON Output and `layouts.md` §12 state that a tool with data carries `label`. In **single mode** the daily all-tools `tu --json` omits it for every tool (`fetchAllTotals` returns bare `UsageTotals`), while `tu cc --json`, `tu m --json`, `tu w --json`, and every multi-mode path include it. V2 reproduces the binary; the spec should either document the exception or gain a `[DECIDE: keep|drop]` entry (DC-25) alongside DC-01, which it compounds.
- **DC candidate (human, G0/G1)**: bare `tu` / `tu --json` in single mode never touches `~/.tu/cache` and `--fresh` is a no-op there, whereas `tu cc`, `tu m`, `tu w` cache per tool. V2 caches uniformly (§8.3) because the cache is not a Goal-listed surface and Principle IV wants heavy operations cached. If G1 prefers a faithful port here it is a one-line change (`Cache: nil` on the daily-all path); the harness cannot see the difference either way.
- **Rounding rule for B2's CSV**: `render.FormatCost` implements the ICU rule for tables/Markdown; CSV's `toFixed(2)` is a different rule (exact-value half-up). Recording so B2 does not assume one formatter.
- **Warning order (carried from V1)**: the TS prints fetch warnings in completion order; Go prints registry order. Only observable when two or more tools fail in one run — no placeholder-corpus case does.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Seven new packages `internal/{config,query,view,render,render/ansi,render/json,command}`; `cmd/tu` wired; zero new module deps; nothing ships | Plan D5 layout and the Target architecture table name `query`/`view`/`render`/`command`; D2 keeps Go dark; V1 precedent is dependency-free | S:85 R:70 A:90 D:85 |
| 2 | Confident | Scope = single-mode snapshot, table + JSON, all sources/periods, flags `--json/-j --fresh/-f --no-color -t --metric`; everything else recognized-but-unported prints the existing placeholder (exit 1) | Row text: "bare grammar … Single mode only"; `-t`/`--metric` are parse-only on the snapshot (table metric-neutral) and turn six more gate cases green at no render cost; the placeholder keeps unported cases red with today's signature, which G1 item 6 requires | S:70 R:85 A:85 D:70 |
| 3 | Confident | `Parse` implements the **complete** TS grammar and validation (every flag, every message, the TS check order, version after validation, non-data tokens) in this row | The parser is `command`'s deliverable and one function in the TS; splitting it across rows would have B1/B2/B6 re-touch the same table; G1 item 5 wants table-driven parse tests; byte-exact messages were captured | S:55 R:85 A:85 D:65 |
| 4 | Certain | Harness gate = the 52 `snapshot-*` IDs with `conf: single`, `env ≠ envrepo`, and no csv/md/by-machine; the 6 excluded IDs belong to B2/B4 | Enumerated from `tudiff run --list --filter snapshot`; row text "every single-mode snapshot case green"; csv/md/by-machine are named in B2/B4's Scope columns | S:85 R:80 A:95 D:85 |
| 5 | Confident | `query`: `Window`, `ByTool`, `ByUser`, `RollUp(period)`, one `GroupBy(dims...)` (first-seen order), `Period` enum with `String()`, `CurrentLabel(period, now)` local, `WeekLabel` UTC | Row text and Target architecture list exactly these; semantics lifted from `filterEntriesByRange`, `aggregateMonthly/Weekly`, `weekLabel`, `currentLabel`; first-seen order preserves registry order without a sort | S:80 R:70 A:85 D:75 |
| 6 | Confident | `RollUp` groups by (label, Tool, User, Machine) and sorts ascending by byte order | The TS aggregates per single-tool list; generalizing the key to the record's dims is the only way one function serves all pivots; `localeCompare` on ISO labels equals byte order | S:60 R:80 A:80 D:70 |
| 7 | Confident | `view.Table` model with `Column{Title,Width,Align}`, `Row{Kind,Cells,Bar,Delta}`, `Cell{Text,Dim}`, `Empty`, `Legend`, `Footer`; `view.Snapshot(rows []ToolTotals, period)` implements `renderTotal`'s row rules | Row text lists "columns, rows, cells, deltas, bar scales, legend"; reserved slots let B2/B4/B7 extend without reshaping; rules lifted line-for-line from `renderTotal` incl. hidden-row cost in Total | S:75 R:75 A:85 D:70 |
| 8 | Confident | `view` and `query` import no registry: `command` resolves display names via `ccusage.Lookup` and hands `[]view.ToolTotals{Name, Label, Totals}` in registry order | `ccusage` is the exec package; G1 item 1 says `query`/`view`/`render` import no I/O; the registry moved into `fact` would churn V1 for no gain | S:50 R:80 A:85 D:70 |
| 9 | Certain | `render/ansi.Colors{Enabled}` is a value passed down; `Enabled = !--no-color && NO_COLOR == ""`; color emitted regardless of TTY; codes per the Color Reference | Spec § Terminal width and color and `colors.ts`; a value instead of the TS module global is the plan's "no globals" stance | S:85 R:85 A:95 D:90 |
| 10 | Certain | Table line layout: blank, boldWhite title, blank, [empty text, blank] or header (cells wrapped individually), dim divider, plain data rows, [dim divider, boldWhite total], blank; divider `─`×w per column joined by a dash-pipe-dash triple (87 visible chars) | Lifted from `renderTotal`; verified byte-for-byte against the node captures (§10) | S:90 R:90 A:95 D:95 |
| 11 | Certain | `render.FormatCost` = shortest-repr decimal rounded half-away-from-zero to 2 places + grouping (ICU parity), not `FormatFloat('f', 2)`; `FormatInt` = grouping | Verified 2026-09-16: node gives `1.005→$1.01`, `0.125→$0.13`, `0.015→$0.02`, `999999.995→$1,000,000.00`; Go `'f',2` differs on each; shortest-repr algorithms agree | S:70 R:90 A:85 D:80 |
| 12 | Certain | `render/json.Snapshot` writes an ordered object by hand: display-name keys in registry order, `label` first when present, six pinned totals, floats via `encoding/json`, `-0→0`, two-space indent, trailing newline | `JSON.stringify(obj,null,2)` layout captured; Go maps are unordered and struct tags cannot conditionally omit `label` in order; Go's float encoder follows ES6 except `-0` | S:75 R:85 A:90 D:80 |
| 13 | Confident | **Reproduce** the single-mode daily-all JSON `label` omission (clear labels when `Source == all && Period == Daily`); flag as spec correction / DC-25 | Verified against a today-dated fixture: `tu --json` no label, `tu cc --json`/`tu m --json` label present; it is an external byte surface the harness compares; plan: "anything not in the drop list is a behavior to keep" | S:70 R:90 A:80 D:70 |
| 14 | Confident | **Do not reproduce** the daily-all no-cache quirk: Go caches uniformly through V1's `Source{Cache}`; `--fresh` skips the read everywhere; flagged for G0/G1 | Cache is not a Goal-listed surface; harness cannot observe it (fresh HOME per case, stdout/stderr/exit only); Principle IV wants caching; one-line reversal if G1 disagrees | S:45 R:90 A:75 D:60 |
| 15 | Confident | Minimal `internal/config` now (`Paths`, `ResolvePaths` with byte-exact `ErrNoHome`, `ParseConf`, `DetectMode`); B1 grows it; V2 emits no config warnings | Single-vs-multi must be decided to honor "single mode only"; `readConfig`'s mode rule is six lines; starting the package B1 owns avoids a throwaway probe in `command` (minimum pathways) | S:55 R:85 A:80 D:70 |
| 16 | Certain | Fetch is daily-only via `FetchAll`/`Fetch` with `ccusage.DefaultTimeout` applied once in `Run`; roll-up is client-side | Spec § Fetching: "only the daily subcommand is ever called"; V1 exported `DefaultTimeout` for exactly this edge | S:85 R:85 A:95 D:90 |
| 17 | Confident | `command.Run(ctx, req, mode, deps) (Result, error)` with `Deps{Source Fetcher, Now, Colors}`, `Result{Lines, Warnings, TotalCost, TotalTokens, CostByItem}`, `ErrUnported` sentinel; `cmd/tu` is the only writer and maps errors to exit codes | Target architecture's `Result{Lines, TotalCost, CostByItem, TotalTokens}`; G1 items 1, 3, 4; warnings ride the result so `source` never prints (V1 design) | S:75 R:75 A:85 D:75 |
| 18 | Certain | Exit codes: 0 success incl. empty; 2 for every `UsageError` (stderr message, `ShortUsage` only for `Unknown argument`); 1 for placeholder, `$HOME` unset, and any runtime error | Spec § Exit Codes; `EXIT_USAGE` sites in `cli.ts`; captured stderr for `bogus`, `cc codex`, `cc --help`, `--json --csv`, `-t --metric cost`, `--metric`, `--metric foo` | S:90 R:90 A:95 D:90 |
| 19 | Certain | Version check moves after flag validation (`tu --json --csv --version` → exit 2), matching `main()` | `parseGlobalFlags` runs before the `rawArgs.includes("--version")` test in the TS; the scaffold's order was a stand-in | S:80 R:95 A:95 D:90 |
| 20 | Confident | `import _ "time/tzdata"` in `cmd/tu`; `CurrentLabel` uses `time.Now()` in `time.Local` | Harness `tz: alt` (`Asia/Kolkata`) must resolve on every host the way Node's bundled ICU does; Go uses the embed only when the system zoneinfo is absent; +450 KB, no startup cost | S:50 R:95 A:85 D:75 |
| 21 | Certain | Tests: table-driven `query`/`command`, golden files with `-update` for `render/ansi` and `render/json`, fake `Fetcher` for `Run` with a fixed clock, end-to-end `cmd/tu` test via `TestMain`-built fake ccusage on `_placeholder` only | G1 item 5 names these shapes; V1 set the `TestMain`/`_placeholder`-only precedent; the populated path needs an injected clock because placeholder dates never match today | S:70 R:90 A:85 D:80 |
| 22 | Confident | Memory: two new `go-port` files (`query-view-render`, `command-edge`); `go-port/fact-and-sources` and `build/toolchain` modified; TS memories untouched | V1 opened the `go-port` domain; one file per layer keeps each under the FKF size guidance; X3 reshapes at cutover anyway | S:45 R:90 A:75 D:65 |
| 23 | Certain | No external surface changes: no `src/node`, spec, justfile, CI, harness, plan-doc edits; Go stays unshipped | Row text and plan Goal forbid it; D2; the operator owns plan row status | S:95 R:90 A:95 D:95 |

23 assumptions (11 certain, 12 confident, 0 tentative, 0 unresolved).
