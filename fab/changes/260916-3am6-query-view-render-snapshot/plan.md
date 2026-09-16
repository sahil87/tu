# Plan: Query, View, Render and the Snapshot Command (Go port row V2)

**Change**: 260916-3am6-query-view-render-snapshot
**Intake**: `intake.md`

> Read `intake.md` first: §2 defines the scope and the 52-case gate, §3–§9 the package APIs, §10 the byte-exact reference outputs, §11 the tests. This plan restates the contract as requirements; the intake carries the worked examples.

## Requirements

### Go Port: `internal/query`

#### R1: Period, CurrentLabel, WeekLabel
`package query` SHALL define `Period` (`Daily`, `Weekly`, `Monthly`) with `String()` returning `daily`/`weekly`/`monthly`; `CurrentLabel(p, now time.Time)` MUST compute the label in `now.Location()`: daily `2006-01-02`, weekly the ISO date of the current week's Sunday (`now.AddDate(0, 0, -int(now.Weekday()))`), monthly `2006-01`. `WeekLabel(daily string)` MUST map an ISO daily label to its week's Sunday using UTC date arithmetic, returning a label that does not parse as `2006-01-02` unchanged.

- **GIVEN** `now` = 2026-09-16 12:00 in `Asia/Kolkata`
- **WHEN** `CurrentLabel` is called for the three periods
- **THEN** the results are `2026-09-16`, `2026-09-13`, `2026-09`
- **GIVEN** the daily label `2026-03-01` (a Sunday) and `2026-01-01` (a Thursday)
- **WHEN** `WeekLabel` is called
- **THEN** the results are `2026-03-01` and `2025-12-28`; `WeekLabel("garbage")` returns `garbage`

#### R2: Window, ByTool, ByUser are pure filters
`Window(recs, since, until)` MUST keep records with `since <= Date <= until` by string comparison, inclusive, an empty bound being open on that side; `ByTool(recs, key)` and `ByUser(recs, user)` MUST keep records whose `Tool`/`User` equals the argument. None MUST mutate its input or return the input slice.

- **GIVEN** records dated 2026-01-05, 06, 07
- **WHEN** `Window(recs, "2026-01-06", "")` runs
- **THEN** two records are returned and the input is unchanged

#### R3: RollUp re-labels and sums per (label, Tool, User, Machine)
`RollUp(recs, p)` MUST relabel each record (daily identity, weekly `WeekLabel(Date)`, monthly `Date[:7]`), sum `Totals` with `Totals.Add` over records sharing the new label and the three string dims, and return the buckets sorted ascending by label with first-seen order inside a label. Daily MUST return a copy, never the input slice. `TotalTokens` is summed, never recomputed.

- **GIVEN** six `cc` records for 2026-01-05..10 each with `TotalCost 0.5, TotalTokens 24400`
- **WHEN** `RollUp(recs, Monthly)` runs
- **THEN** one record labeled `2026-01` with `TotalCost 3.0` and `TotalTokens 146400`; with a seventh record dated 2026-01-11 added, `RollUp(recs, Weekly)` yields labels `2026-01-04` (six records) and `2026-01-11` (one)

#### R4: GroupBy is the one group-by
`type Dim int` (`Date`, `Tool`, `User`, `Machine`) and `GroupBy(recs, dims ...Dim) []Group` where `Group{Key fact.Record; fact.Totals}` MUST sum `Totals` per distinct tuple of the requested dims (other string fields of `Key` empty), emit groups in first-seen input order, and never mutate the input. No other aggregation function MAY exist in the module.

- **GIVEN** records for tools `cc, codex, cc` in that order
- **WHEN** `GroupBy(recs, Tool)` runs
- **THEN** two groups, `cc` first with the sum of both `cc` records, then `codex`

### Go Port: `internal/view`

#### R5: The table model carries no ANSI
`package view` SHALL define `Align`, `RowKind` (`Header`, `Divider`, `Data`, `Total`), `Column{Title, Width, Align}`, `Cell{Text, Dim}`, `Row{Kind, Cells, Bar, Delta}`, `Table{Title, Columns, Rows, Empty, Legend, Footer}`, and `ToolTotals{Name, Label string; fact.Totals}`. Nothing in the package MAY import `os`, `io`, `fmt` print functions, or any `source`/`ccusage` package.

- **GIVEN** the package's import list
- **WHEN** inspected
- **THEN** it contains only `fact`, `query`, `render` (for number formatting) and stdlib string/sort packages

#### R6: Snapshot builds the cross-tool table exactly as renderTotal
`Snapshot(rows []ToolTotals, p Period) Table` MUST set `Title` to `📊 Combined Usage ({p})`; columns `Tool` (12, left) then `Tokens`, `Input`, `Output`, `Cache`, `Cost` (12 each, right); one `Data` row per input row with `TotalTokens > 0`, in input order, cells `Name`, `FormatInt(TotalTokens)`, `FormatInt(InputTokens)`, `FormatInt(OutputTokens)`, `FormatInt(CacheCreationTokens+CacheReadTokens)`, `FormatCost(TotalCost)`; a `Header` row then a `Divider` row first; a `Divider` then a `Total` row only when more than one row is visible, the Total summing every input row including hidden ones; `Empty = "  No usage"` and no rows when every input row has `TotalTokens == 0`. Widths are fixed; a wider cell overflows.

- **GIVEN** two rows with tokens and one all-zero row
- **WHEN** `Snapshot` runs for `Daily`
- **THEN** rows are Header, Divider, Data, Data, Divider, Total and Total's Tokens cell is the two rows' sum
- **GIVEN** one populated row
- **WHEN** `Snapshot` runs
- **THEN** rows are Header, Divider, Data only
- **GIVEN** a hidden row with `TotalCost 0.25, TotalTokens 0` and two visible rows
- **WHEN** `Snapshot` runs
- **THEN** the Total Cost cell includes the 0.25

### Go Port: `internal/render`

#### R7: FormatInt and FormatCost reproduce en-US toLocaleString
`render.FormatInt(int64)` MUST group thousands with commas. `render.FormatCost(float64)` MUST return `$` + the value rounded to exactly two decimals with grouping, where rounding is performed on the shortest round-trip decimal representation (`strconv.FormatFloat(x, 'f', -1, 64)`) half away from zero with carry, never on the binary value with half-even; `-0` renders `$0.00`.

- **GIVEN** the values 1.005, 0.125, 0.015, 2.675, 999999.995, 4.936068800000001, 1234567.891, 0.5, 0
- **WHEN** formatted
- **THEN** the results are `$1.01`, `$0.13`, `$0.02`, `$2.68`, `$1,000,000.00`, `$4.94`, `$1,234,567.89`, `$0.50`, `$0.00`
- **GIVEN** 1234567890123
- **WHEN** `FormatInt` runs
- **THEN** `1,234,567,890,123`

#### R8: ansi primitives with Colors as a value
`render/ansi` SHALL define `Colors{Enabled bool}` with methods `BoldWhite`, `BoldCyan`, `Dim`, `Bold`, `Green`, `Red`, `Cyan`, `Yellow`, `Magenta`, `Blue`, `BrightGreen`, `DimGreen` that wrap with the Color Reference codes (`\x1b[1;37m`, `\x1b[1;36m`, `\x1b[2m`, `\x1b[1m`, `\x1b[32m`, `\x1b[31m`, `\x1b[36m`, `\x1b[33m`, `\x1b[35m`, `\x1b[34m`, `\x1b[92m`, `\x1b[2;32m`) and close with `\x1b[0m`, returning the input unchanged when `!Enabled`; `StripANSI` removing `\x1b\[[0-9;]*m`; `PadLeft`/`PadRight` by rune count. No global color state MAY exist.

- **GIVEN** `Colors{Enabled: false}`
- **WHEN** `BoldWhite("x")` is called
- **THEN** the result is `x`; with `Enabled: true` it is `"\x1b[1;37mx\x1b[0m"` and `StripANSI` of it is `x`

#### R9: ansi.Table emits the renderTotal line layout
`ansi.Table(t view.Table, c Colors) []string` MUST return: `""`, `c.BoldWhite(t.Title)`, `""`; then if `t.Empty != ""`: `t.Empty`, `""`; else the Header row as cells padded per column then each wrapped individually in `BoldCyan`, joined by ` | `; Divider rows as `Dim` of `─`×width per column joined by `─|─`; Data rows as padded cells joined by ` | ` with no escapes; the Total row as padded cells each wrapped in `BoldWhite`, joined by ` | `; then `Legend`/`Footer` when non-empty; then `""`. The snapshot divider is 87 visible characters.

- **GIVEN** the populated two-tool table and `Colors{Enabled: true}`
- **WHEN** rendered
- **THEN** the lines equal intake §10's populated reference byte for byte, and with `Enabled: false` equal the same lines stripped of escapes
- **GIVEN** the empty table
- **WHEN** rendered
- **THEN** the five lines are `""`, title, `""`, `  No usage`, `""`

#### R10: json.Snapshot writes the ordered snapshot object
`render/json.Snapshot(rows []view.ToolTotals) []string` MUST produce the lines of `JSON.stringify(obj, null, 2)` plus the trailing newline semantics of `console.log`: an object keyed by `Name` in input order; per row `"label"` first when `Label != ""`, then `totalCost`, `inputTokens`, `outputTokens`, `cacheCreationTokens`, `cacheReadTokens`, `totalTokens`; two-space indentation; numbers encoded via `encoding/json` (ES6 float rules) with `-0` normalized to `0`; strings encoded without HTML escaping.

- **GIVEN** rows `Claude Code{Label "2026-09-16", 0.5, 3000, 400, 1000, 20000, 24400}` and `Codex{Label "", zero}`
- **WHEN** rendered and joined with `\n`
- **THEN** the text equals intake §10's `tu cc --json`-style shape for the first object followed by the six-zero object for Codex, with `label` absent on Codex

### Go Port: `internal/config`

#### R11: Minimal config cascade for mode detection
`config.ResolvePaths(home)` MUST return `Paths{Home, ConfigDir, UserConf, OrgConf, LegacyConf}` for `$HOME/.config/tu/tu.conf`, `$HOME/.config/tu/org.conf`, `$HOME/.tu.conf`, and `ErrNoHome` (message exactly `tu: $HOME is not set; cannot locate config`) when `home == ""`. `ParseConf(raw)` MUST trim lines, skip blanks and `#` lines, split at the first `=`, trim both sides, later keys overwriting. `DetectMode(p, getenv)` MUST return `Multi` when `getenv("TU_METRICS_REPO") != ""`, else merge `org.conf` then the user conf (tu.conf, falling back to the legacy file when tu.conf is unreadable) and return `Multi` iff the merged `metrics_repo` is non-empty, else `Single`. The package MUST write nothing to any stream.

- **GIVEN** a temp HOME with only `.config/tu/org.conf` containing `metrics_repo = x`
- **WHEN** `DetectMode` runs with an empty env
- **THEN** `Multi`; with an additional `tu.conf` containing `metrics_repo =` the result is `Single`; with no files and `TU_METRICS_REPO=y` the result is `Multi`; with nothing the result is `Single`

### Go Port: `internal/command`

#### R12: Parse implements the flag pass and validation with byte-exact errors
`Parse(args) (Request, *UsageError)` MUST implement intake §8.2 steps 1–2: strip the sixteen boolean flags, consume values for `--interval/-i` (digits only), `--user/-u`, `--since/-s`, `--until`, `--metric`, `--top` (next token not starting with `-`), then validate in the TS order with the exact messages: interval (only with `--watch`), the six format conflicts in order (watch+json, json+csv, json+md, csv+md, watch+csv, watch+md), `-u requires a username`, since/until shape (`YYYY-MM-DD` or `YYYYMMDD`, normalized), `--since must be on or before --until`, `--metric requires 'tokens' or 'cost'`, `-t and --metric cost are incompatible`, `--top requires a positive integer`. Format precedence json > csv > md > table. `UsageError{Message, ShowUsage}`; `ShortUsage` is the byte-exact constant from intake §8.1.

- **GIVEN** `["--json", "--csv"]`
- **WHEN** parsed
- **THEN** `UsageError{Message: "Error: --json and --csv are incompatible", ShowUsage: false}`
- **GIVEN** `["-t", "--metric", "cost"]`, `["--metric"]`, `["--metric", "foo"]`, `["--since", "2026-02-01", "--until", "2026-01-01"]`, `["-u"]`, `["--top", "x"]`, `["--watch", "--interval", "3"]`
- **WHEN** parsed
- **THEN** each returns its TS message; `["--since", "2026-13-01"]` parses successfully with `Since == "2026-13-01"`; `["--since", "20260101"]` yields `Since == "2026-01-01"`

#### R13: Version, non-data tokens, positionals and Unknown argument
After validation, `Parse` MUST set `Version` when `--version`, `-V` or `-v` appears anywhere in the original args; then set `Command` when the first positional is one of `help -h --help init-conf init-metrics sync status update shell-init help-dump skill`; otherwise classify positionals per `parseDataArgs`: a first token in `cc codex co oc gemini gem copilot cop kimi ki all` is the source (aliases `co→codex`, `gem→gemini`, `cop→copilot`, `ki→kimi`, `all→""`); remaining tokens `d`/`daily`, `w`/`weekly`, `m`/`monthly` set Period, `h`/`history` History, `lb`, `lbh`, `dh`/`wh`/`mh` set both; any other token yields `UsageError{Message: "Unknown argument: {tok}", ShowUsage: true}`.

- **GIVEN** `["cc", "codex"]`, `["m", "cc"]`, `["bogus"]`, `["cc", "--help"]`, `["--bogus"]`
- **WHEN** parsed
- **THEN** the messages are `Unknown argument: codex`, `Unknown argument: cc`, `Unknown argument: bogus`, `Unknown argument: --help`, `Unknown argument: --bogus`, all with `ShowUsage`
- **GIVEN** `["--json", "--csv", "--version"]`
- **WHEN** parsed
- **THEN** the format-conflict error, not `Version`
- **GIVEN** `["ki", "m", "-j", "--fresh"]`
- **WHEN** parsed
- **THEN** `Source "kimi"`, `Period Monthly`, `Display Snapshot`, `Format JSON`, `Flags.Fresh true`

#### R14: Run composes the single-mode snapshot and returns a Result
`Run(ctx, req, mode, deps) (Result, error)` MUST return `ErrUnported` for any request outside intake §2's in-scope set (multi mode, non-snapshot display, csv/md, any unported flag, `Command != ""`). Otherwise it MUST: apply `ccusage.DefaultTimeout` to the context once; fetch daily only via `FetchAll` (all tools) or `Fetch` (single source) with `fresh = Flags.Fresh`; compute `cur = query.CurrentLabel(Period, deps.Now())`; take `GroupBy(Window(RollUp(recs, Period), cur, cur), Tool)`; build `[]view.ToolTotals` over registry order (all six, or the one tool) with `Name` from `ccusage.Lookup`, `Label = cur` when a group matched else `""`; clear every `Label` when `Source == "" && Period == Daily` (the TS single-mode daily-all quirk); render via `json.Snapshot` or `ansi.Table(view.Snapshot(...), deps.Colors)`; return `Result{Lines, Warnings (the source errors), TotalCost, TotalTokens, CostByItem}` with stats summed over the rows keyed by display name. The `Fetcher` interface MUST be satisfied by `*ccusage.Source`.

- **GIVEN** a fake `Fetcher` returning today's `cc` and `codex` records, `Now` fixed, `Format JSON`, all tools, `Daily`
- **WHEN** `Run` runs
- **THEN** the lines carry no `label` key; the same with `Period Monthly` carries `"label": "{YYYY-MM}"` on the two populated tools; with `Source "cc"` and `Daily` the single object carries `"label"`
- **GIVEN** `Source "cc"`
- **WHEN** `Run` runs
- **THEN** the fake records exactly one `Fetch` call for the `cc` tool and no `FetchAll`
- **GIVEN** `Flags.Fresh true`
- **WHEN** `Run` runs
- **THEN** the fake observes `fresh == true`
- **GIVEN** `Display History`, or `Format CSV`, or `Flags.ByMachine`, or `mode == Multi`
- **WHEN** `Run` runs
- **THEN** `errors.Is(err, ErrUnported)` and no fetch occurs

### Go Port: `cmd/tu`

#### R15: run() is the only writer and follows the TS order
`run(args, stdout, stderr) int` MUST: parse (usage error → message line, then `ShortUsage` line when `ShowUsage`, to stderr, return 2); `Version` → the existing version line, return 0; `Command != ""` → `notImplementedMsg` to stderr, return 1; `config.ResolvePaths(os.Getenv("HOME"))` error → its message to stderr, return 1; detect mode; build `ccusage.Source{Cache: cache.Default()}` and `Deps{Now: time.Now, Colors: ansi.Colors{Enabled: !NoColor && os.Getenv("NO_COLOR") == ""}}`; `Run`; `ErrUnported` → placeholder, return 1; other error → message, return 1; success → `source.WriteWarnings(stderr, …)`, each line via `fmt.Fprintln(stdout, l)`, return 0. `main.go` MUST import `_ "time/tzdata"`. Nothing below `cmd/tu` MAY write to stdout/stderr or call `os.Exit`.

- **GIVEN** `HOME=""` and args `[]`
- **WHEN** `run` executes
- **THEN** stderr is `tu: $HOME is not set; cannot locate config\n`, exit 1; with args `["bogus"]` and `HOME=""` the exit is 2 with `Unknown argument: bogus` (parse precedes the HOME check)
- **GIVEN** args `["--help"]` or `["m", "dh", "--json"]` or `["--csv"]`
- **WHEN** `run` executes
- **THEN** stderr is `tu: not implemented (Go port in progress)\n`, stdout empty, exit 1

#### R16: End-to-end against the fake ccusage reproduces the empty state
A `cmd/tu` test with a `TestMain` that builds `../fakeccusage`, sets `TUDIFF_FIXTURES` to the `_placeholder` alias only, `HOME` to a temp dir, `TZ=UTC`, `NO_COLOR` unset, and `PATH` with the fake first MUST assert `run` output byte for byte for `tu`, `tu cc`, `tu m`, `tu w`, `tu --json`, `tu --no-color` (the intake §10 empty-state and all-zero JSON shapes) and exit 2 plus the two stderr blocks for `tu bogus`.

- **GIVEN** the built fake and the placeholder corpus
- **WHEN** `run([]string{}, …)` executes
- **THEN** stdout is `"\n\x1b[1;37m📊 Combined Usage (daily)\x1b[0m\n\n  No usage\n\n"`, stderr empty, exit 0

### Harness gate

#### R17: The 52 single-mode snapshot cases are green
`just go-lint`, `just go-test` MUST pass; `just go-diff --placeholder --filter snapshot` MUST report `GREEN` for every ID listed in intake §2 (the 52) and the summary line MUST show at least 52 green. The six csv/md/by-machine single cases and every multi/envrepo case remain red.

- **GIVEN** a clean checkout with node, go, just, script on PATH and `npm ci` done
- **WHEN** `just go-diff --placeholder --filter snapshot` runs
- **THEN** exit 1 (other cases still red) and the report lists all 52 gate IDs as `GREEN`

### Non-Goals

- History, csv, markdown, bars, footers, deltas (B2); machine columns and legend (B4); metrics source and multi mode (B3); leaderboards (B5); sync and the `--dry-run` guard (B6); watch (B7); help, help-dump, skill, shell-init, update, completions (B8); the full config cascade, warnings, sentinels, setup commands (B1).
- Reproducing the TS single-mode daily-all cache bypass (intake §8.3 decision).
- Any edit to `src/node/**`, `docs/specs/**`, `harness/**`, `justfile`, CI, `Formula/`, `package.json`, or the plan document.

### Design Decisions

#### Colors is a value, not a global
**Decision**: `ansi.Colors{Enabled}` is computed once in `cmd/tu` and passed to renderers.
**Why**: The TS `setNoColor()` module global is the output-channel leak the plan removes; a value keeps `render` pure and testable.
**Rejected**: A package-level `SetNoColor` mirroring the TS.
*Introduced by*: 260916-3am6-query-view-render-snapshot

#### Number formatting lives in the parent render package
**Decision**: `render.FormatInt`/`FormatCost` sit in `internal/render`, shared by `render/ansi` now and `render/markdown` later.
**Why**: One implementation of the ICU rounding rule; CSV (B2) needs a different rule and must not reuse it.
**Rejected**: Formatting inside `view` (the model would carry pre-formatted strings only, which it does, but the rule belongs to the encoder family).
*Introduced by*: 260916-3am6-query-view-render-snapshot

#### view receives display names, not registry keys
**Decision**: `command` resolves `ccusage.Lookup(key).Name` and hands `view.ToolTotals{Name}`.
**Why**: G1 checklist item 1 keeps `query`/`view`/`render` free of the exec package.
**Rejected**: Moving the registry into `fact` (churns V1 for no gain).
*Introduced by*: 260916-3am6-query-view-render-snapshot

#### GroupBy emits groups in first-seen order
**Decision**: No sorting inside `GroupBy`; callers order their input.
**Why**: Registry-ordered input yields registry-ordered columns with no extra pass; history callers sort by label before grouping.
**Rejected**: Sorting by key inside `GroupBy` (would reorder tool columns alphabetically).
*Introduced by*: 260916-3am6-query-view-render-snapshot

#### Reproduce the JSON label quirk, not the cache quirk
**Decision**: `Run` clears labels on the single-mode daily-all path; `Run` caches on every path.
**Why**: The label is a compared byte surface; the cache is not, and Principle IV wants heavy work cached.
**Rejected**: Reproducing both (a faithful port of a non-surface quirk against the constitution); reproducing neither (a harness red on populated data).
*Introduced by*: 260916-3am6-query-view-render-snapshot

#### Recognized-but-unported requests keep the placeholder
**Decision**: The parser knows the full grammar; `Run` returns `ErrUnported` for what later rows own; `cmd/tu` prints the scaffold's placeholder, exit 1.
**Why**: Unported harness cases stay red with today's signature (G1 item 6), and `Unknown argument` fires only where the TS fires it.
**Rejected**: Treating unported flags as unknown arguments (false divergence on the error text).
*Introduced by*: 260916-3am6-query-view-render-snapshot

## Tasks

### Phase 1: Setup

- [x] T001 Create `src/go/internal/query/period.go` with `Period`, `String()`, `CurrentLabel`, `WeekLabel` and `period_test.go` table tests (Kolkata and UTC clocks, Sunday and month-start cases, malformed passthrough) <!-- R1 -->

### Phase 2: Core Implementation

- [x] T002 Create `src/go/internal/query/query.go` with `Window`, `ByTool`, `ByUser`, `RollUp`, `Dim`, `Group`, `GroupBy` and `query_test.go` table tests incl. purity checks <!-- R2 R3 R4 -->
- [x] T003 Create `src/go/internal/render/format.go` with `FormatInt`, `FormatCost` (shortest-repr half-away-from-zero rounding, grouping, `-0` → `$0.00`) and `format_test.go` with the R7 value table <!-- R7 -->
- [x] T004 Create `src/go/internal/view/table.go` (model types) and `view/snapshot.go` (`ToolTotals`, `Snapshot`) with `snapshot_test.go` covering omission, Total-when->1, hidden cost, empty, heading per period <!-- R5 R6 -->
- [x] T005 [P] Create `src/go/internal/render/ansi/color.go` (`Colors`, palette, `StripANSI`, `PadLeft`, `PadRight`) and `ansi/table.go` (`Table`) with golden tests under `ansi/testdata/` (`snapshot_color.golden`, `snapshot_nocolor.golden`, `snapshot_single.golden`, `snapshot_empty.golden`) regenerated by a `-update` flag <!-- R8 R9 -->
- [x] T006 [P] Create `src/go/internal/render/json/snapshot.go` (`Snapshot`, ordered writer, ES6 floats, `-0` normalization) with golden tests under `json/testdata/` (all-tools two populated with labels, all-tools no labels, single tool, all-zero) <!-- R10 -->
- [x] T007 [P] Create `src/go/internal/config/config.go` (`Paths`, `ResolvePaths`, `ErrNoHome`, `ParseConf`, `Mode`, `DetectMode`) and `config_test.go` over temp HOMEs (tu.conf, org.conf, legacy, env, none, empty value override) <!-- R11 -->
- [x] T008 Create `src/go/internal/command/request.go` (`Request`, `Flags`, `Display`, `Format`, `Metric`, `UsageError`, `ShortUsage`) and `command/parse.go` (`Parse`) with `parse_test.go` table tests over every argv in `harness/matrix.json` the grammar reaches plus the R12/R13 error cases with exact messages <!-- R12 R13 -->
- [x] T009 Create `src/go/internal/command/run.go` (`Fetcher`, `Deps`, `Result`, `ErrUnported`, `Run`) with `run_test.go` using an in-memory fake `Fetcher` and a fixed `Now`: golden lines, label drop, single-tool fetch, fresh propagation, `ErrUnported` matrix, `Result` stats <!-- R14 -->

### Phase 3: Integration & Edge Cases

- [x] T010 Rewrite `src/go/cmd/tu/main.go` `run()` per R15 (parse → version → placeholder → HOME → mode → deps → Run → warnings → lines), add `import _ "time/tzdata"`, update `main_test.go` (`TestRunNotImplemented` keeps `--help` and `m dh --json`, adds `--csv`; new `tu --json --csv --version` exit-2 case; HOME-unset cases) <!-- R15 -->
- [x] T011 Add `src/go/cmd/tu/e2e_test.go` with the `TestMain`-built fake ccusage (`_placeholder` only, temp HOME, `TZ=UTC`, `NO_COLOR` unset) asserting the empty-state bytes for `tu`, `tu cc`, `tu m`, `tu w`, `tu --json`, `tu --no-color` and the `tu bogus` stderr/exit <!-- R16 -->
- [x] T012 Run `just go-lint`, `just go-test`, then `just go-diff --placeholder --filter snapshot`; confirm every intake §2 gate ID is `GREEN` and record the summary line (`tudiff: N cases — g green, r red, t timeout`) in this plan's `## Notes`; fix any divergence by comparing `bin/harness/report/cases/<id>/{node,go}.stdout` <!-- R17 -->

## Execution Order

- T001 blocks T002 (query types) and T004 (Period in headings)
- T003 blocks T004, T005 (formatting)
- T004 blocks T005, T006, T009
- T002, T007, T008 block T009; T009 blocks T010; T010 blocks T011; T011 blocks T012
- T005, T006, T007 may run in parallel once T003/T004 exist

## Acceptance

### Functional Completeness

- [x] A-001 R1: `Period.String()`, `CurrentLabel` (local zone, three periods, Sunday arithmetic), `WeekLabel` (UTC, passthrough) exist and pass their table tests
- [x] A-002 R2: `Window`, `ByTool`, `ByUser` filter as specified and never mutate or alias input
- [x] A-003 R3: `RollUp` relabels, sums all six fields via `Totals.Add`, sorts ascending, returns a copy for daily
- [x] A-004 R4: `GroupBy(dims...)` is the only aggregation in the module and preserves first-seen order
- [x] A-005 R5: `view` defines the model types and imports no I/O or exec packages
- [x] A-006 R6: `view.Snapshot` follows every `renderTotal` rule (omission, Total gate, hidden-cost total, empty text, fixed widths, heading)
- [x] A-007 R7: `FormatInt`/`FormatCost` pass the value table incl. the ICU-parity cases
- [x] A-008 R8: `ansi.Colors` methods, palette codes, `StripANSI`, padding primitives exist with no global state
- [x] A-009 R9: `ansi.Table` output equals the golden files and the intake §10 references
- [x] A-010 R10: `json.Snapshot` output equals the golden files, `label` conditional and first, `-0` normalized
- [x] A-011 R11: `config` resolves paths, returns the byte-exact `ErrNoHome`, parses conf, detects mode per the cascade
- [x] A-012 R12: `Parse` strips/consumes flags and validates in TS order with byte-exact messages
- [x] A-013 R13: version-after-validation, non-data tokens, positional grammar, aliases, `Unknown argument` with usage
- [x] A-014 R14: `Run` fetches daily only, applies the timeout, groups via `GroupBy`, drops labels on daily-all, renders, returns stats, and returns `ErrUnported` for out-of-scope requests
- [x] A-015 R15: `run()` write order and exit codes; `time/tzdata` imported; nothing below `cmd/tu` writes or exits
- [x] A-016 R16: the end-to-end test asserts the empty-state bytes and the usage-error path against the fake ccusage
- [x] A-017 R17: `just go-lint`, `just go-test` pass; `just go-diff --placeholder --filter snapshot` shows every intake §2 ID `GREEN`

### Behavioral Correctness

- [x] A-018 R15: `tu --json --csv --version` exits 2 with the conflict message (the scaffold's version-first order is gone)
- [x] A-019 R15: `tu` and `tu cc` no longer print the placeholder; `tu --help`, `tu m dh --json`, `tu --csv` still do

### Scenario Coverage

- [x] A-020 R14: fake-fetcher tests cover daily-all (no label), monthly-all (label `YYYY-MM`), single-tool daily (label present), single-tool fetches one tool, `fresh` reaches the fetcher
- [x] A-021 R9: golden files exist for color, no-color, single-row and empty tables and `StripANSI(color) == nocolor`
- [x] A-022 R1: `CurrentLabel` tested with a non-UTC zone and a week Sunday that falls in the previous month

### Edge Cases & Error Handling

- [x] A-023 R7: `FormatCost(1.005) == "$1.01"`, `FormatCost(0.125) == "$0.13"`, `FormatCost(999999.995) == "$1,000,000.00"`, `FormatCost(math.Copysign(0, -1)) == "$0.00"`
- [x] A-024 R12: a value flag at the end of args (`--since`, `--metric`, `--top`, `-u`) yields its own missing-value message; `--interval 3` without `--watch` is accepted silently
- [x] A-025 R15: `HOME` unset yields exit 1 with the byte-exact message only after a successful parse; a usage error with `HOME` unset still exits 2
- [x] A-026 R14: a source error (fake returns `*source.Error`) surfaces in `Result.Warnings` and the tool renders as zero data; `cmd/tu` writes it through `source.WriteWarnings`

### Code Quality

- [x] A-027 Pattern consistency: new packages follow V1's conventions (package doc comments, `_test.go` siblings, table-driven tests, relative fixture walk, `TestMain`-built fake)
- [x] A-028 No unnecessary duplication: `Totals.Add`, `ccusage.Lookup`, `ccusage.DefaultTimeout`, `source.WriteWarnings`, `cache.Default` are reused; no second registry, timeout, or warning writer
- [x] A-029 Readability over cleverness: no function over ~50 lines without reason; the ordered JSON writer and the parser validation read top to bottom in TS order
- [x] A-030 Minimum pathways: one `GroupBy`, one fetch path (`FetchAll`/`Fetch`), one render entry per format; no per-dimension aggregation
- [x] A-031 No magic strings: escape codes, `ShortUsage`, error messages, the placeholder line, `"  No usage"`, column widths are named constants
- [x] A-032 No swallowed errors: `cache.Default` failure and source errors are surfaced (warnings) or returned, never discarded silently; `gofmt -l` and `go vet` clean

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`
- Ground truth for bytes: `bin/harness/report/cases/<case id>/node.stdout` after `just go-diff --placeholder --filter snapshot`, and intake §10. To regenerate populated references from the TS binary: stage `dist/tu.mjs` + `tu.default.conf` in a temp `dist/`, copy `bin/harness/ccusage` to `<tmp>/dist/vendor/ccusage/bin/ccusage`, copy `harness/fixtures/_placeholder` to a temp alias and `sed` one fixture's `2026-01-07` to today, then run `env -i HOME=<tmp home> TZ=UTC LANG=C.UTF-8 TUDIFF_FIXTURES=<alias> PATH=<node dir>:/usr/bin:/bin node <tmp>/dist/tu.mjs …`. Never inherit the shell env: an exported `TU_METRICS_REPO` flips the TS into multi mode.
- Apply decision (T002): R3's weekly example dates `2026-01-05..10` all fall inside the week starting Sunday 2026-01-04 (2026-01-11 is the next Sunday), so the two labels R3 names cannot arise from those six dates. `query_test.go` keeps R3's expected labels `2026-01-04`/`2026-01-11` but extends the weekly input to `2026-01-05..11` (7 records); the monthly example uses the six dates exactly as written.
- Apply decision (T008): `tu --interval abc` (no `--watch`) is `Unknown argument: abc` (ShowUsage, exit 2) — the TS does not consume a non-numeric interval value, so it lands in the positional list. The parser reproduces this.
- T012 gate (2026-09-16): `just go-lint` and `just go-test` clean; `just go-diff --placeholder --filter snapshot` → `tudiff: 180 cases — 52 green, 128 red, 0 timeout   (fixtures: _placeholder; 86 cases replayed unconfirmed fixtures)` — every one of the 52 intake §2 gate IDs verified GREEN in `bin/harness/report/report.txt`; the 128 red are the multi/org/legacy/envrepo cases and the six csv/md/by-machine singles, all owned by later rows by design.
- Review re-verification (2026-09-16): `go test ./...`, `gofmt -l`, `go vet`, `just go-lint`, `just go-test` all clean; snapshot-filtered gate re-run reproduces 52 green / 128 red with all 52 intake §2 IDs GREEN (spot-checked `node.*` vs `go.*` captures byte-identical); full-matrix run → `tudiff: 368 cases — 73 green, 295 red, 0 timeout` (21 bonus greens, all parser/usage-error cases incl. `interval-unsupported`; every single-mode red maps to a named later row — see review-result.yaml).

## Deletion Candidates

None — this change adds new functionality without making existing code redundant. The one symbol it obsoleted, `hasVersionFlag` in `src/go/cmd/tu/main.go`, was already removed by the apply diff itself (its role moved into `command.Parse`'s version detection).

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | `RollUp` keys on (label, Tool, User, Machine) and `GroupBy` returns first-seen order | Intake §3; the only key shape that serves every later pivot | S:70 R:80 A:85 D:75 |
| 2 | Confident | `render.FormatCost` rounds the shortest decimal repr half away from zero | Intake §5 verified values against node v24 | S:75 R:90 A:85 D:80 |
| 3 | Confident | `json.Snapshot` is a hand-written ordered writer using `encoding/json` only for scalars | Intake §6; ordered keys and conditional `label` rule out struct marshalling | S:75 R:85 A:90 D:80 |
| 4 | Confident | `Run` clears labels on the daily-all path and caches on every path | Intake §8.3 decisions 13 and 14 | S:70 R:90 A:80 D:70 |
| 5 | Confident | Unported requests return `ErrUnported` and `cmd/tu` prints the unchanged placeholder | Intake §2 and §9 | S:75 R:90 A:85 D:75 |
| 6 | Confident | The e2e test lives in `cmd/tu` with a `TestMain`-built fake, `_placeholder` only | V1 precedent (`source_test.go`), intake §11 | S:65 R:90 A:85 D:75 |

6 assumptions (0 certain, 6 confident, 0 tentative).
