# Intake: Watch Mode (Go port row B7)

**Change**: 260917-4pze-watch-mode-tui
**Created**: 2026-09-17

## Origin

> Context: fab/plans/sahil/26-09-15-go-port.md, row B7. Read the Decisions and Target architecture sections before writing the intake. Do not change any external surface listed under Goal. Build watch mode (D12): hand-rolled on x/term, no TUI framework. Ticker loop, compositor, panel (burn rate, session stats), rain layer, key handling, resize, compact threshold, -w on every display type. Harness gate: non-interactive --once-style frame capture if feasible, otherwise golden frames from the compositor with a fake clock.

One-shot `/fab-new` invocation from the Go-port operator queue (Phase 2, the last row before G2). The plan row reads: *"D12: `watch` loop, compositor, panel (burn rate, session stats), rain layer, key handling, resize, compact threshold, `-w` on every display type. Harness: non-interactive `--once`-style frame capture if feasible, otherwise golden frames from the compositor with a fake clock."* — size L, depends on B4 (PR #91) and B5 (PR #93), both merged; B6 (PR #94) is also in.

Key readings that shaped this intake (2026-09-17, worktree at `ca4493a`):

- **Watch is the last placeholder.** `command.inScope` rejects `Flags.Watch` and `Flags.NoRain`; `cmd/tu` prints `tu: not implemented (Go port in progress)`, exit 1, for `-w`. Every other data surface is green (442/442 harness cases per the command-edge memory).
- **The delta seam is half-wired.** `view.Row.Delta` + `ansi.dataRow` render a trailing arrow (spaced or abutting per `Table.DeltaSpaced`); `view.HistoryOptions.Prev` drives it through `rowDelta` for the single-tool history and the pivot; `view.LeaderboardOptions.Prev` exists and reserves one bar column but no leaderboard row ever gets an arrow; `view.Snapshot` has no `Prev` at all. `command.Result` already carries `TotalCost`, `TotalTokens` and `CostByItem` for all four displays with the TS keys (`{Name}`, `{Name}:{label}`, `total:{label}`, `user` or `user/machine`).
- **The TS arrow sits in three different places** (`src/node/tui/formatter.ts`): the single-tool history and the pivot append it after the last cell (after the machine columns, before the bar); the snapshot appends it to the metric cell *before* `padStart(12)` — JS pads by raw string length, so a colored arrow (10 code units) defeats the padding and the cell renders unpadded; the leaderboard pads the metric cell first and appends the arrow after, then dims the composite for an exact-zero value. The compact renderers pad like the snapshot.
- **No compact or row-budget code exists in Go.** The TS `FormatOptions.compact` / `maxRows` branches live in `renderTotal`, `renderHistory`, `renderTotalHistory` (`renderCompactSnapshot` / `renderCompactHistory` / `renderCompactTotalHistory`); `renderLeaderboard` ignores both, and the `lbh` watch path passes only `prevCosts`.
- **`maxRows` is the constant 15**, not "rows that fit the terminal height": `watch.ts` builds `formatOpts = { prevCosts, compact, maxRows: 15 }`. The spec sentence (usage § Table semantics "Row budget in watch mode", layouts § 7) describes intent; the bytes are the oracle. Flagged for G0 below, not "fixed".
- **The TS reference** is `src/node/tui/watch.ts` (293 lines), `compositor.ts` (375), `panel.ts` (150), `rain.ts` (187), the `*Lines` dispatch twins and the `_lastRender*` globals in `src/node/core/cli.ts` (1672–1836, 2029–2049). Contract text: `docs/specs/usage.md` § Watch Mode, § Flags (`--watch`, `--interval`, `--no-rain`), DC-03, DC-12, DC-17; `docs/specs/layouts.md` § 7–11, § 21. Memory: `docs/memory/watch-mode/tui.md` (the TS behaviour, authoritative for constants and byte sequences).
- **Precedents in Go**: `Deps.Now func() time.Time` and `cache.Now` are the fake-clock seams; `render/ansi` goldens use the `-update` flag; `ansi.Colors` already has `BrightGreen`, `DimGreen`, `StripANSI`, `PadLeft`/`PadRight` (rune-count padding — the JS `.length` twin for BMP text); `render.FixedHalfUp` is `toFixed`, `render.JSRound` is `Math.round`, `render.FormatInt` is `toLocaleString("en-US")`. `cmd/tu` already probes the TTY width with `x/term` and imports `golang.org/x/term v0.46.0`; no other dependency is added.

## Why

**Problem.** `tu -w` — on any display type — is the one data-command surface the Go binary still refuses. Watch is how the maintainers actually run tu during a work session (R2's dogfood checklist has "one watch session with a terminal resize" as a required event), so without B7 the dogfood window cannot start and the cutover criteria cannot be met.

**Consequence of not doing it.** R1 depends on B8 only, but R2 (dogfood) requires B1–B7 all green; G2 is blocked on this row. The `--no-rain` and `--interval` flags' DC-03 silent acceptance without `--watch` is also unreproduced (`tu --no-rain` prints the placeholder today).

**Why this shape (D12).** Hand-rolled on `x/term`: goroutines for the three inputs (keys, `SIGWINCH`, poll results), `time.Ticker` for the rain, a one-second countdown timer, and a single select loop that owns every terminal write. The compositor stays a concept — a pure function from (session state, table lines, terminal size, clock) to the byte frame — so it is golden-testable without a terminal. No bubbletea: the siblings avoid frameworks, the constitution prizes fast startup, and one mode does not justify a large dependency. The target architecture puts I/O only at the two ends; `watch` is named as one of them, `command.Run` is called for lines plus stats each poll, and no result travels through globals (G1 checklist item 3).

**Constraint from the Goal.** Every byte the TS writes to the terminal during a watch session is reproduced: the alt-screen and cursor sequences, the skeleton, the stats grid, the footer and its truncation, the delta arrows in their three placements (including the JS raw-length padding quirk), the compact tables, the 15-row budget, the rain geometry and colors, and the exit output. Where the TS and the spec disagree, the TS bytes win and the spec line is flagged for G0.

**Harness gate decision.** A `--once`-style frame capture is **not feasible** under the Goal: a session frame depends on wall-clock (Elapsed, the countdown) and on `Math.random` (rain), so a byte-diff between the two binaries would need a new flag or env switch on *both* sides — a new external surface, which the Goal forbids — and a Go-only switch has nothing to diff against. The gate is therefore (a) **golden frames from the Go compositor under a fake clock and a seeded RNG** (D12's fallback, named in the row), (b) an end-to-end loop test against a fake terminal asserting the exit output equals the one-shot `command.Run` lines, and (c) **deterministic harness cases for the watch flag family** — the exit-2 incompatibility and `--interval` validation cases and the DC-03 silent-acceptance cases, which are byte-diffable today and exercise the same parse and dispatch path.

## What Changes

### 1. New package `internal/watch` — the loop

Files: `watch.go` (the `Run` loop and session state), `compositor.go` (pure layout + frame bytes), `panel.go` (stats grid), `rain.go` (rain state machine), `status.go` (footer), `skeleton.go`, `terminal.go` (the terminal seam). Tests as `_test.go` siblings with goldens under `internal/watch/testdata/`.

```go
// Poll runs one refresh: the caller composes command.Run with the per-poll
// render options and the live width. It is the only thing watch knows about
// command — watch imports no source, query, view or render/* package except
// render/ansi for Colors, StripANSI and the pad helpers.
type Poll func(ctx context.Context, f Frame) (Lines []string, Stats Stats, err error)

type Frame struct {
    Prev    map[string]float64 // previous poll's CostByItem; nil on the first poll
    Compact bool               // width < CompactThreshold
    MaxRows int                // always 15 (watchMaxRows)
    Width   int                // live terminal width (80 when not a TTY)
}

type Stats struct{ TotalCost float64; TotalTokens int64; CostByItem map[string]float64 }

type Options struct {
    Interval int           // seconds, already validated by Parse (5–3600, default 10)
    NoRain   bool
    Poll     Poll
    Term     Terminal      // see § 2
    Stderr   io.Writer
    Colors   ansi.Colors
    Now      func() time.Time
    Rand     *rand.Rand    // math/rand/v2; seeded in tests
}

// Run blocks until q, Ctrl-C or SIGINT, then returns the last rendered table
// lines for cmd/tu to print on the normal screen. It never calls os.Exit.
func Run(ctx context.Context, o Options) (last []string)
```

**Loop shape.** One goroutine owns the terminal (all writes happen inside its `select`). Inputs: a key channel fed by a reader goroutine on stdin (only when stdin is a TTY, raw mode), a resize channel fed by `signal.Notify(SIGWINCH)`, an interrupt channel fed by `signal.Notify(SIGINT)`, a poll-result channel fed by a worker goroutine, the rain `time.Ticker` (107 ms, only when rain is enabled), and the countdown `time.Timer` (1 s). A `polling` flag is the re-entrancy guard (a second poll request while one is in flight is dropped, as in the TS).

**Startup order** (the TS `runWatch`): enter alt screen (`\x1b[?1049h` then `\x1b[?25l`); write the skeleton (§ 6); lay out the rain zone against the skeleton lines; start the rain ticker; register resize and keys; run the first poll.

**Poll sequence** (the TS `doPoll`): footer → `Refreshing...` (§ 5); build `Frame{Prev: previousCosts if non-empty else nil, Compact: width < 60, MaxRows: 15, Width: width}`; call `Poll`. On error: write `Warning: fetch failed, retrying next cycle` + `\n` to stderr and restart the countdown (no frame change). On success: `cost = Stats.TotalCost`, `now`; on the first successful poll set `startTime = now`, `startCost = cost`, `startTokens = Stats.TotalTokens`; append `{now, cost}` to `pollHistory`; `totalTokens = Stats.TotalTokens`; `last = Lines`; layout (§ 3) + flush; `previousCosts = copy of Stats.CostByItem`; start the countdown at `Interval`.

**Countdown** (the TS `startCountdown`): set `value = Interval`, render the footer; every 1000 ms: `value--`; when `value <= 0` run a poll, else render the footer and re-arm. Enter or Space cancels the timer and polls immediately. The countdown is push-driven: the footer is rewritten only on these events, never on a periodic compositor tick.

**Keys** (the TS `stdin.on("data")`): a read chunk equal to `q` or `\x03` (Ctrl-C in raw mode) → cleanup; equal to `\r`, `\n` or ` ` → cancel countdown + poll. Anything else (arrow keys, multi-byte chunks) is ignored. Stdin is put in raw mode with `term.MakeRaw` only when it is a TTY; otherwise no key handling (the TS `process.stdin.isTTY` guard) and SIGINT is the only exit.

**Cleanup** (the TS `cleanup`, once): stop the rain ticker and countdown timer; restore the stdin state; write `\x1b[?25h` then `\x1b[?1049l`; return `last`. `cmd/tu` prints each line + `\n` to stdout and exits 0.

**Resize** (`SIGWINCH`): the TS `rerender()` — a no-op until the first successful poll (the skeleton's rain zone is *not* re-laid-out on an early resize); afterwards re-layout from the cached table lines, session and cost, then flush.

### 2. The terminal seam

```go
type Terminal interface {
    Size() (cols, rows int)          // 80×24 when not a TTY or on probe error (columns ?? 80, rows ?? 24)
    Write(p []byte) (int, error)     // stdout
    Keys() <-chan []byte             // nil when stdin is not a TTY
    Resize() <-chan struct{}         // SIGWINCH
    Interrupt() <-chan struct{}      // SIGINT
    Close() error                    // restore raw mode, stop signal delivery
}
```

`terminal.go` holds the `x/term` + `os/signal` implementation (`term.IsTerminal`, `term.GetSize`, `term.MakeRaw`/`term.Restore`, `syscall.SIGWINCH` — the release targets are linux and darwin only, D8, so no build tags). Tests use a fake with a scripted key sequence, a settable size, and a `bytes.Buffer` capturing every write. `cmd/tu` constructs the real one from `os.Stdout`/`os.Stdin`.

### 3. Compositor — pure layout and frame bytes

`compositor.go` exposes two pure functions plus the layout record; nothing in it touches a stream.

```go
const (
    CompactThreshold = 60
    rainTick         = 107 * time.Millisecond
    countdownTick    = time.Second
    minRainCols      = 10
    rainGutter       = 2
    footerRows       = 1
    watchMaxRows     = 15
)

type Layout struct {
    Compact bool
    Stats   []string // 4 lines (3 rows + separator), nil when compact
    Table   []string
    Rain    RainZone // Enabled, Cols, Rows, StartRow (1-based), StartCol (0-based)
}

func Lay(statsLines, tableLines []string, cols, rows int, noRain bool) Layout
func (l Layout) Frame(footer string, rain string) []byte
```

**`Lay`** reproduces `layoutAndUpdate` + `setupRainZone`: `compact = cols < 60`; `contentHeight = len(stats) + len(table)`; `maxContentWidth = max StripANSI rune width over stats+table lines`; `available = rows − contentHeight − 1`; `wantRain = !noRain && !compact`. `wantRain && available > 0` → below-content zone `{cols, available, startRow: contentHeight+1, startCol: 0}`; else `wantRain` → `marginCols = cols − maxContentWidth − 2`; `marginCols >= 10` → right-margin zone `{marginCols, rows−1, startRow: 1, startCol: maxContentWidth+2}`, else disabled; else disabled. `layoutForSkeleton(lines)` is `Lay(nil, lines, …)` with the skeleton's own lines as content.

**`Frame`** reproduces `flush`: `\x1b[H`; each content line (stats then table) + `\x1b[K\n`; `\x1b[J`; the footer as `\x1b[{rows};1H\x1b[K{footer}`; then the current rain frame re-emitted (the flush's clears erased every drawn rain cell; the rain renderer rewrites every occupied cell, so the re-emit restores them in the same write and polls do not blink the rain). The footer-only path (`renderStatus`) writes just the `\x1b[{rows};1H\x1b[K{footer}` part.

The `RainState` keeps its geometry across polls: `setup` on an existing state calls `resize`, which is a no-op when `(cols, rows, startCol)` are unchanged and re-initialises the drops otherwise (so the first real poll after the skeleton is a no-op resize when the geometry matches).

### 4. Panel — the stats grid

`panel.go` is the TS `panel.ts` byte-for-byte, with `now` injected:

```go
type Session struct {
    StartTime   time.Time
    StartCost   float64
    StartTokens int64
    Polls       []PollPoint // {At time.Time; Cost float64}
    TotalTokens int64
}
const rollingWindow = 5
func FormatElapsed(d time.Duration) string          // floor seconds; "Xh Xm Xs" / "Xm Xs" / "Xs"
func BurnRate(polls []PollPoint) (perHour float64, ok bool) // <2 polls → !ok; window = last 5; Δt == 0 → 0
func StatsGrid(s Session, todayCost float64, now time.Time, c ansi.Colors) []string // 3 rows + dim separator
```

Values: `hasTwoPolls = len(Polls) > 1`. **Elapsed** = `FormatElapsed(now − StartTime)`. **Session** = `hasTwoPolls ? sign + "$" + FixedHalfUp(|latest.Cost − StartCost|, 2) : "$0.00"` with sign `+` when the delta is ≥ 0, `-` otherwise. **Tok/min** = `hasTwoPolls && (TotalTokens − StartTokens) > 0 && elapsedMin > 0 ? "~" + FormatInt(JSRound(sessionTokens / elapsedMin)) : "--"`. **Rate** = `ok && rate > 0 ? "~$" + FixedHalfUp(rate, 2) + "/hr" : "--"` (yellow when shown). **Proj. day** = shown only when Rate is: `"~$" + FixedHalfUp(todayCost + rate × hoursRemaining, 2)` with `hoursRemaining = 24 − now.Hour() − now.Minute()/60` in local time. `todayCost` is `Stats.TotalCost` — the sum over the rendered rows, whatever the display (the TS passes `getCost()` unchanged).

Row layout (`formatGridRow`): constants `leftLabelW = 9`, `maxLeftValueW = 8`, `gap = 5`, `rightLabelW = 10`, placeholder `--`. Left = `Dim(" " + PadRight(label, 9)) + BoldWhite(value)`, or 12 spaces for the blank row-3 left; `leftVisible = 1 + 9 + len(value)` (12 for blank); `rightPad = max(1, 23 − leftVisible)`; right = `Dim(PadRight(rightLabel, 10)) + (Yellow(value) if the Rate row and value != "--" else BoldWhite(value))`. Rows: (`Elapsed`, `Tok/min`), (`Session`, `Rate`), (blank, `Proj. day`). Fourth line: `Dim("─" × max visible width of the three rows)` — 35 while the right values are `--`, wider once populated.

### 5. Status footer

`status.go`: parts `[Dim("Next refresh: {n}s") | Dim("Refreshing...")]` and `Dim("↵ refresh · q quit")`, joined with `Dim(" · ")`; while the visible (StripANSI) width exceeds the terminal width and more than one part remains, drop the last part and re-join (controls first, then only the status text remains).

### 6. Skeleton

`skeleton.go` reproduces `renderSkeleton(termWidth)`. Full mode: the stats grid for `Session{StartTime: now}` with `todayCost 0` (Elapsed `0s`, Session `$0.00`, the rest `--`); `""`; `BoldWhite("📊 Combined Usage (daily)")`; `""`; the header `Tool | Tokens | Input | Output | Cache | Cost` with `BoldCyan` per cell (Tool `PadRight` 12, the rest `PadLeft` 12) joined ` | `; `Dim` of the 87-char divider (`─×12` joined `─|─`); then `38 spaces + Dim("Loading...")` (`floor((87 − 10)/2)`). Compact: `""` then `Dim("Loading...")`. The skeleton is written as `\x1b[H` + each line + `\n` (no `\x1b[K`) and hardcodes the daily snapshot header regardless of the display type — reproduce, do not "improve".

### 7. Rain layer

`rain.go` is `rain.ts` with an injected `*rand.Rand`. Constants: pool = half-width katakana `ｦｱｲｳｴｵｶｷｸｹｺｻｼｽｾｿﾀﾁﾂﾃﾄﾅﾆﾇﾈﾉﾊﾋﾌﾍﾎﾏﾐﾑﾒﾓﾔﾕﾖﾗﾘﾙﾚﾛﾜﾝ` + digits + `A–Za–z`; `density 0.3`, `densityRefRows 20`, `maxDensityScale 3`, speed `0.3–1.0` rows/tick, length `3–8`, respawn delay `0–5` ticks, shimmer `0.05`.

- `ActiveDropCount(cols, rows) = JSRound(cols × 0.3 × min(3, max(1, rows/20)))`.
- `initDrops`: Fisher–Yates shuffle of the column indices, drop *i* on `columns[i % cols]`, scattered (`row = randFloat(−length, rows)`, `delay 0`).
- `tick`: `delay > 0 → delay--, continue`; `row += speed`; each trail char replaced with probability 0.05; when `floor(row) − length > rows` respawn in place (`row = −length`, new speed, length, `delay = randInt(0, 5)`, new chars).
- `render(startRow)`: for each active drop and `i` in `0..length−1`: `r = floor(row) − i`, skip when `r < 0 || r >= rows`; emit `\x1b[{startRow+r};{startCol+col+1}H` + colored char (`BrightGreen` for `i == 0`, `Green` for `i < length−2`, `DimGreen` otherwise); track occupied `(r, col)`; then for every previously occupied cell not occupied now emit `\x1b[{row};{col}H ` (a space). Returns the string; `""` when `rows <= 0` or disabled.

The rain ticker fires every 107 ms: `tick()` then write `render()` when non-empty. `--no-rain` disables the ticker entirely; compact mode disables the zone (§ 3).

### 8. `view` and `render/ansi` — deltas everywhere, compact tables, the row budget

**Cell-level delta.** Add `Cell.Delta Delta` and a table-level placement:

```go
type DeltaInCell int
const (
    DeltaPadsArrow DeltaInCell = iota // snapshot + compact: pad(text + arrow) — JS raw-length padding
    DeltaAfterPad                     // leaderboard: pad(text) + arrow, then the exact-zero Dim wraps the composite
)
// Table gains: DeltaInCell DeltaInCell
```

`ansi.dataRow`: a cell with `Delta != DeltaNone` renders under the table's `DeltaInCell` rule using `PadLeft(text + Green("↑")|Red("↓"), width)` (rune count over the colored string equals the JS `.length`: 10 code units per colored arrow, 1 uncolored — so with color on the snapshot cell is effectively unpadded and with `NO_COLOR` it pads to 12, both as the TS does) or `PadLeft(text, width) + arrow`. The existing `Row.Delta` trailing form stays for the history tables (after the machine columns, before the bar; spaced for the single-tool table, abutting for the pivot).

- **Snapshot**: `view.Snapshot(rows, p, bd, m)` becomes `view.Snapshot(rows, p, bd, SnapshotOptions{Metric, Prev})`; the Data row's Cost cell (under cost) or Tokens cell (under tokens) gets `Delta` from `Prev[Name]`; `Table.DeltaInCell = DeltaPadsArrow`. Only the delta follows the metric; the Cost column stays in dollars.
- **Leaderboard**: `view.Leaderboard` fills the Cost cell's (or Tokens cell's under tokens) `Delta` from `Prev[user]` / `Prev[user/machine]`; `Table.DeltaInCell = DeltaAfterPad`. The bar `reserve` already exists.
- **History / pivot / lbh**: unchanged wiring (`rowDelta` over `HistoryOptions.Prev`); `command` now passes `Prev`.

**Row budget.** `HistoryOptions.MaxRows int` (0 = unlimited): `History` keeps the last `MaxRows` entries after the empty check; `TotalHistory` keeps the last `MaxRows` labels of the sorted union before the empty check. Significance, separators, the p95 scale, the footer and the legend are computed on the truncated window (they already operate on the entries the function sees). `lbh` gets no `MaxRows` (the TS `lbhFormatOptions` never carries it).

**Compact tables.** New `view/compact.go` + `render/ansi/compact.go`:

```go
const compactNameW, compactValueW = 14, 12 // divider "─" × 27
type CompactTable struct {
    Title string            // the same title line the full table carries
    Rows  []CompactRow      // Name/Label 14 left; Value 12 right (in the display metric) with Cell.Delta
    Total *CompactRow       // nil unless more than one row
    Empty string            // "  No usage" / "  No data" — the empty check runs BEFORE the compact branch
}
func CompactSnapshot(rows []ToolTotals, p query.Period, m Metric, prev map[string]float64) CompactTable
func CompactHistory(s Series, o HistoryOptions) CompactTable        // key "{Name}:{label}"; Total sums visible entries
func CompactTotalHistory(series []Series, o HistoryOptions) CompactTable // key "total:{label}"; Total = grand over all series
```

Encoder lines: `""`, `BoldWhite(title)`, `""`, then per row `PadRight(name, 14) + " " + PadLeft(value + arrow, 12)`, then when more than one row: `Dim("─"×27)` and `BoldWhite(PadRight("Total", 14)) + " " + BoldWhite(PadLeft(total, 12))`, then `""`. The compact snapshot lists tools with `TotalTokens > 0` and its Total sums the metric over *all* input rows; `--by-machine` columns and the legend are dropped in compact (the TS returns before them); token mode puts total tokens in the value cell. Leaderboards have no compact form (the TS `renderLeaderboard` ignores `compact`); `lb -w` and `lbh -w` at < 60 cols render the full table with no stats grid and no rain.

### 9. `command` — admit `--watch`, carry the per-poll render options

- `inScope`: remove `f.Watch` and `f.NoRain` from the exclusion (`DryRun` and `SkipBrewUpdate` stay out). `tu --no-rain` and `tu --interval 30` without `-w` therefore render the ordinary one-shot output (DC-03 silent acceptance).
- `Deps` gains `Live *LiveOptions` (`nil` for one-shot):

```go
type LiveOptions struct {
    Prev    map[string]float64
    Compact bool
    MaxRows int
}
```

`runSnapshot`, `runHistory`, `runLeaderboard`, `runLeaderboardHistory` pass `Prev` into the view options; `runSnapshot`/`runHistory` branch to the compact tables when `Live.Compact` and the format is the table; the history paths set `MaxRows`. `Result.TotalCost`, `TotalTokens` and `CostByItem` are computed over the *untruncated* data as today (the TS sums before rendering). `Deps.Width` is whatever the caller passes per poll.

- The watch poll sets `req.Flags.Fresh = true` (the TS `action(true, …)` — every poll bypasses the 60 s cache) and `Format == Table` is guaranteed by Parse (the format conflicts are exit 2).

### 10. `cmd/tu` — the watch branch

After the reserved-user guard and the `--sync` block, when `req.Flags.Watch`:

1. `req, notices, _ := command.Normalize(req, cfg.Mode, time.Now())`; print the notices to stderr **once, before the alt screen** (the TS prints the guards in `main()` ahead of `runWatch`). Per-poll `Result.Notices` are discarded (the `--full` notice would otherwise repeat every poll).
2. Build `watch.Options{Interval: req.Flags.Interval, NoRain: req.Flags.NoRain, Colors, Now: time.Now, Rand: rand.New(rand.NewPCG(seed from time)), Term: watch.NewTerminal(os.Stdout, os.Stdin), Stderr: stderr}` and a `Poll` closure that copies `deps`, sets `Width: f.Width`, `Live: &command.LiveOptions{Prev: f.Prev, Compact: f.Compact, MaxRows: f.MaxRows}`, calls `command.Run(ctx, reqFresh, cfg, deps)`, writes `source.WriteWarnings(stderr, res.Warnings)` (the TS fetcher warns on stderr every poll, alt screen or not), and returns `res.Lines` + stats; `ErrLeaderboardMode` is caught before entering watch exactly as for one-shot (exit 1, no alt screen).
3. `last := watch.Run(ctx, o)`; print `last` to stdout line by line; return `command.ExitOK`.

The package doc comment's "everything else (watch and its companions)" sentence is rewritten: after B7 the only `ErrUnported` trigger left is `--skip-brew-update` on a data command (out of scope, § Out of scope).

### 11. Tests

- **Golden frames** (`internal/watch/testdata/*.golden`, `-update` flag as in `render/ansi`): skeleton at 100×30 and 50×20; a first-poll frame (stats with `--`, table, footer `Next refresh: 10s`) at 100×30 with fixed lines; a second-poll frame with populated Rate/Proj. day under a fixed clock (`now` advanced 11 s, cost +0.07); compact frames at 59×20 for snapshot and history; right-margin rain geometry at 100×12 (zone starts at `maxContentWidth + 2`, width `cols − maxContentWidth − 2`); `--no-rain` frames; footer truncation at 30, 20 and 10 columns. Rain goldens use a seeded `rand.Rand` (one tick, one render, one clear pass).
- **Table-driven**: `FormatElapsed`, `BurnRate` (window, Δt = 0, negative rate → `--`), `StatsGrid` values incl. the `$0.00` before two polls and the sign rule, `ActiveDropCount` (the TS `rain.test.ts` cases: 1× at ≤ 20 rows, 3× cap at 60+), footer truncation, `Lay` zone selection (below-content vs right-margin vs disabled at `marginCols` 9/10).
- **Loop test** with the fake terminal and a manual clock: skeleton → first poll → `q` returns the poll's lines; `\r` cancels the countdown and polls (re-entrancy guard drops a second `\r` mid-poll); SIGWINCH before the first poll writes nothing, after it re-flushes; a `Poll` error writes the warning to stderr and the frame is unchanged; Ctrl-C byte and SIGINT both exit; the stream ends with `\x1b[?25h\x1b[?1049l`.
- **view/ansi**: goldens for the snapshot with `Prev` under cost and tokens, colored and `NO_COLOR` (the raw-length padding quirk pinned both ways); the leaderboard with `Prev` (arrow after the padded cell, zero-cost composite Dim); `MaxRows` on `History`/`TotalHistory` (footer and significance on the window); the three compact tables incl. Total-row rules and delta arrows.
- **cmd/tu**: `run([]string{"--no-rain"}, …)` and `--interval 30` render the one-shot table (no placeholder); `-w --json` etc. stay exit 2.

### 12. Harness matrix — deterministic watch-family cases

Add to `harness/matrix.json` (all byte-diffable today, none touch the live loop): `watch-csv` (`-w --csv`), `watch-md` (`-w --md`), `watch-interval-min` (`-w -i 3`), `watch-interval-max` (`-w -i 4000`), `watch-interval-nan` (`-w -i abc`) — usage errors, exit 2; `no-rain-without-watch` (`--no-rain`, conf single/multi, io pipe/tty) and `interval-without-watch-long` (`--interval 30`) — one-shot output, DC-03. `watch-json` and `interval-unsupported` stay. No `-w` session case is added: the frame is time- and RNG-dependent and byte-diff needs an identical capture on both sides (§ Why).

### Out of scope (owned elsewhere)

- Any change to `src/node/` (D4 freeze) or to the specs; the two spec sentences that disagree with the TS bytes are flagged for G0 in the hydrate notes: the "rows that fit the terminal height" row budget (usage § Table semantics, layouts § 7 — the TS is a constant 15) and the skeleton's hardcoded daily snapshot header on non-snapshot displays (layouts § 8 shows it for `tu -w` only).
- DC-17 (tables wider than the frame wrap and break the line accounting) is reproduced, not guarded.
- `--skip-brew-update` on a data command still returns `ErrUnported` (the TS silently accepts it); it belongs to the toolkit flag family (B8) and is noted for the operator as a one-line follow-up, not folded in here.
- The `watch-mode/tui.md` memory (TS behaviour) is untouched; the Go watch memory is a new go-port file.

## Affected Memory

- `go-port/watch-mode`: (new) the `internal/watch` package — loop, terminal seam, compositor `Lay`/`Frame`, panel, footer, skeleton, rain, key/signal handling, exit path, golden-frame test strategy, the harness-gate decision and the two G0 flags
- `go-port/command-edge`: (modify) `inScope` admits `--watch`/`--no-rain`; `Deps.Live`; the `cmd/tu` watch branch (Normalize-once notices, per-poll width and warnings, exit 0 with the last lines); `ErrUnported` narrowed to `--skip-brew-update` on a data command
- `go-port/query-view-render`: (modify) `Cell.Delta` + `Table.DeltaInCell` and the three arrow placements incl. the JS raw-length padding; `SnapshotOptions`; `HistoryOptions.MaxRows`; the compact table model and encoder
- `harness/differential-harness`: (modify) the seven added deterministic watch-family matrix cases and the recorded reason no live `-w` case exists

## Impact

- **New**: `src/go/internal/watch/{watch,terminal,compositor,panel,status,skeleton,rain}.go` + tests + `testdata/`; `src/go/internal/view/compact.go`; `src/go/internal/render/ansi/compact.go`.
- **Modified**: `internal/view/{table,snapshot,history,pivot,leaderboard}.go` (cell delta, options, `MaxRows`); `internal/render/ansi/table.go` (`DeltaInCell`); `internal/command/{run,leaderboard}.go` (`inScope`, `LiveOptions`, `Prev`/compact/`MaxRows` wiring); `cmd/tu/main.go` (watch branch, doc comment); `harness/matrix.json`; `render/ansi` and `command` tests for the changed signatures.
- **Dependencies**: none added (`golang.org/x/term` already required; `os/signal`, `syscall`, `math/rand/v2` are stdlib).
- **Risk**: the delta-cell padding quirk and the compact tables are the byte-parity hot spots (they are pinned by goldens against TS output captured with and without color); the loop itself is not byte-diffed against the TS — the golden frames are hand-verified against `docs/memory/watch-mode/tui.md` and the layouts mockups, and R2's dogfood session is the live check.
- **CI**: `just go-test` covers everything; `just go-diff --placeholder` gains seven cases (denominator grows, count must stay all-green).

## Open Questions

- None blocking. The harness-gate choice (golden frames, not `--once`) is decided above with its reasoning; if a live `-w` byte-diff is wanted later it needs a G0-level decision to add a capture switch to both binaries.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Scope is `-w` on every display type (snapshot, history, pivot, lb, lbh, with `--by-machine`, `-u`, `-t`, `--since/--until`), plus `--no-rain`/`--interval` silent acceptance without `-w`; no TS change, no spec change | Plan row B7 and the Goal fix the surfaces; D4 freezes `src/node/` | S:85 R:75 A:90 D:90 |
| 2 | Certain | Hand-rolled on `x/term` and `os/signal`, no TUI framework; a single select loop owns all terminal writes | D12 is explicit; the siblings avoid frameworks; `x/term` is already a dependency | S:90 R:70 A:90 D:95 |
| 3 | Confident | Harness gate is golden frames from a pure compositor under a fake clock and seeded RNG, plus a fake-terminal loop test and seven deterministic matrix cases; no `--once` capture | A live frame is time- and RNG-dependent and a capture switch would be a new external surface on both binaries, which the Goal forbids; the row names this fallback | S:70 R:70 A:85 D:75 |
| 4 | Certain | The watch row budget is the constant 15 (`watchMaxRows`), not the terminal height; the spec sentence is flagged for G0 | `watch.ts` passes `maxRows: 15`; the TS bytes are the oracle and the drop list is G0's (P1) | S:80 R:85 A:95 D:85 |
| 5 | Confident | Delta arrows move to the cell level with two placements (`DeltaPadsArrow` for snapshot and compact, `DeltaAfterPad` for the leaderboard); the trailing `Row.Delta` stays for the history tables | The TS places the arrow three different ways and pads by raw length; `PadLeft` counts runes of the colored string, which is the JS `.length` twin | S:65 R:75 A:85 D:70 |
| 6 | Confident | Compact tables are a separate `view.CompactTable` model with its own ANSI encoder rather than a flag on `Table` | Two space-joined columns, no header row, no gutters — a different shape; keeps the full `Table` encoder untouched | S:60 R:80 A:80 D:65 |
| 7 | Confident | Per-poll render options ride `Deps.Live` (`Prev`, `Compact`, `MaxRows`) and `Deps.Width`; `Request` stays the CLI grammar | They are render state, not flags; `Deps` is already the per-call value type | S:60 R:85 A:80 D:70 |
| 8 | Certain | Every poll fetches fresh (`Fresh = true`) and stats are computed over the untruncated data | `action(true, …)` in `watch.ts`; `_lastRenderCost` sums before render | S:85 R:90 A:95 D:95 |
| 9 | Confident | Guard notices are printed once before the alt screen via one edge call to `Normalize`; per-poll `Result.Notices` are discarded; source warnings go to stderr every poll | The TS prints the guards in `main()` ahead of `runWatch` and the fetcher warns per poll; the `--full` notice is not cleared by Normalize and would repeat | S:65 R:85 A:85 D:75 |
| 10 | Certain | Leaderboards get no compact form and `lbh` gets no row budget; `--by-machine` columns and the legend are dropped in compact snapshot/history | `renderLeaderboard` ignores `compact`; `lbhFormatOptions` carries only `prevCosts`; the TS compact branches return before the machine columns | S:80 R:85 A:95 D:90 |
| 11 | Certain | Resize before the first successful poll is a no-op; the skeleton hardcodes the daily snapshot header for every display type | `rerender()` returns when `lastTableLines` is empty; `renderSkeleton` builds the header unconditionally — reproduced, flagged for G0 | S:60 R:85 A:90 D:80 |
| 12 | Certain | Panel numbers use `render.FixedHalfUp` (`toFixed(2)`), `render.JSRound` + `FormatInt` (`Math.round().toLocaleString`), local time for hours remaining | The twins exist and are Node-verified; `getHours()` is local | S:70 R:90 A:90 D:85 |
| 13 | Certain | Change type is `feat`, pinned explicitly | Sibling rows shipped as `feat`; the quoted Goal's "redesigned" would re-infer `refactor` | S:80 R:95 A:95 D:95 |
| 14 | Confident | Seven deterministic matrix cases are added (exit-2 watch conflicts and interval bounds, DC-03 silent acceptance); no live `-w` case | D6 makes the harness first-class; these cases are byte-diffable today and exercise the watch parse and dispatch path | S:60 R:90 A:80 D:70 |
| 15 | Confident | `--skip-brew-update` on a data command stays `ErrUnported` and is handed to the operator as a follow-up | It is the toolkit flag family (B8), not watch; folding it in widens the row | S:55 R:90 A:75 D:65 |

15 assumptions (8 certain, 7 confident, 0 tentative, 0 unresolved).
