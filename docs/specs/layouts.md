# Layouts

> Visual mockups of every distinct output layout produced by `tu`. Each section shows the
> command that triggers it, the ASCII mockup, and notes on column sizing and color.
>
> For data model, flag semantics, and watch mode architecture, see [usage.md](usage.md).

## 1. Snapshot — All Tools

**Command:** `tu`, `tu d`, `tu m`, `tu all`

```
📊 Combined Usage (daily)

Tool         |       Tokens |        Input |       Output |        Cache |         Cost
─────────────|──────────────|──────────────|──────────────|──────────────|─────────────
Claude Code  |  487,683,047 |        3,734 |    1,121,329 |  486,557,984 |      $465.67
Codex        |    2,345,678 |      987,654 |    1,358,024 |            0 |       $23.45
OpenCode     |      456,789 |      234,567 |      222,222 |            0 |        $4.56
─────────────|──────────────|──────────────|──────────────|──────────────|─────────────
Total        |  490,485,514 |    1,225,955 |    2,701,575 |  486,557,984 |      $493.68
```

- **Columns:** Tool (12 left-aligned), Tokens/Input/Output/Cache/Cost (12 right-aligned each) — full row is 87 visible chars (≤ 90 budget)
- **Cache** is cache write + cache read combined, so the row arithmetic closes: Input + Output + Cache = Tokens
- **Separator:** ` | ` between columns
- **Colors:** header row `boldCyan`, dividers `dim`, Total row `boldWhite`
- Cost cells carry `en-US` thousands separators (`$1,012.34`), matching the token columns
- Tools with zero tokens are omitted; Total row shown only when >1 tool has data

## 2. Snapshot — Single Tool

**Command:** `tu cc`, `tu codex m`, `tu oc`

Same table as Layout 1 but with a single data row. Title uses tool name:

```
📊 Claude Code (daily)

Tool         |       Tokens |        Input |       Output |        Cache |         Cost
─────────────|──────────────|──────────────|──────────────|──────────────|─────────────
Claude Code  |  487,683,047 |        3,734 |    1,121,329 |  486,557,984 |      $465.67
```

No divider/Total row when only one row is present.

## 3. History — Single Tool

**Command:** `tu cc h`, `tu cc dh`, `tu codex mh`

```
📊 Claude Code (daily)

Date         |          Input |         Output |    Cache Write |     Cache Read |          Total |      Cost
─────────────|────────────────|────────────────|────────────────|────────────────|────────────────|─────────────────────
2026-03-05   |        456,789 |        567,890 |         23,456 |         12,345 |      1,060,480 |     $2.10 ▏
─────────────|────────────────|────────────────|────────────────|────────────────|────────────────|─────────────────────
2026-04-01   |        234,567 |        345,678 |         10,000 |          5,000 |        595,245 |     $1.50 ▏
2026-04-02   |        567,890 |        678,901 |         45,678 |         23,456 |      1,315,925 | $1,284.85 ██████████
─────────────|────────────────|────────────────|────────────────|────────────────|────────────────|─────────────────────
Total        |      1,259,246 |      1,592,469 |         79,134 |         40,801 |      2,971,650 | $1,288.45
avg $429.48/day · this month $1,286.35 · peak $1,284.85 (2026-04-02)
```

Outlier window — max $4,031.61 > 1.5 × p95 $846.21, so the bars switch to the two-zone scale (green main zone 0→p95, dim `┊` scale-break rule in every row, yellow overflow zone p95→max):

```
Date         |          Input |         Output |    Cache Write |     Cache Read |          Total |      Cost
─────────────|────────────────|────────────────|────────────────|────────────────|────────────────|─────────────────────
2026-06-10   |        234,567 |        345,678 |         10,000 |          5,000 |        595,245 |  $846.21 █████████████████████┊
2026-06-11   |        345,678 |        456,789 |         15,000 |          7,500 |        824,967 | $1,091.67 █████████████████████┊▌
2026-06-12   |        567,890 |        678,901 |         45,678 |         23,456 |      1,315,925 | $4,031.61 █████████████████████┊████████
2026-06-13   |        123,456 |        234,567 |          8,000 |          4,000 |        370,023 |  $172.13 ████▎                 ┊
─────────────|────────────────|────────────────|────────────────|────────────────|────────────────|─────────────────────
Total        |      1,271,591 |      1,715,935 |         78,678 |         39,956 |      3,106,160 | $6,141.62
avg $1,535.41/day · this month $6,141.62 · peak $4,031.61 (2026-06-12) · ┊ = $846.21 (p95)
```

- **Columns:** Date (12 left-aligned), Input/Output/Cache Write/Cache Read/Total (14 right-aligned), Cost (9 right-aligned — fits `$9,999.99` with thousands separators)
- **Bar chart:** green Unicode block elements (full + fractional eighths), max width 30, scaled to max cost
- **p95 two-zone scale:** when `max > 1.5 × p95` (95th percentile of the nonzero row costs, linear interpolation), the bar area splits into a green main zone (linear 0→p95), a dim `┊` (U+250A) scale-break rule rendered in every row (short bars space-pad up to the rule so it aligns vertically), and a yellow overflow zone (linear p95→max, `max(4, round(barWidth/4))` chars). Rows at exactly p95 end at the rule with no overflow segment. Below the trigger the single linear scale renders unchanged — no rule, no legend, no width change
- **Month separators:** daily views emit a dim divider (same construction as the header divider) before each row whose `YYYY-MM` prefix differs from the previous row's — daily period only, computed on the post-`maxRows` window, never before the first visible row
- **Current-period marker:** the row matching the current period's label (today / this week / this month) renders its date cell in **boldWhite** — color-only, no glyph, width unchanged, stripped by `--no-color`/`NO_COLOR`
- **Weekend dimming:** daily views render Saturday/Sunday date cells in **dim** (date cell only — cost/token cells and the bar stay full-intensity), exposing the weekly sawtooth at zero width cost. The today marker wins on a weekend today (one cell, one style). Weekday derived via `getUTCDay()` on the UTC-parsed ISO label — timezone-independent. Daily period only (monthly/weekly labels carry no per-day weekday); compact mode, CSV, and Markdown are untouched; `--no-color`/`NO_COLOR` output is byte-identical to pre-change
- **Summary footer:** one dim line after the Total row (≥2 data rows): `avg $X.XX/day · this month $X,XXX.XX · peak $X,XXX.XX (YYYY-MM-DD)` — avg is window total ÷ data-row count with a per-period unit suffix (`/day`, `/week`, `/month`); `this month` is daily-only and omitted when the window has no current-month rows; two-zone windows append `· ┊ = $X (p95)`. ANSI renderers only — compact, CSV, and Markdown output carry no separators, marker, or footer
- Bars only render when terminal width allows (>= 10 chars remaining after cost column)
- Total row shown when >1 entry

## 4. History — All Tools (Pivot)

**Command:** `tu h`, `tu dh`, `tu mh`

```
📊 Combined Cost History (daily)

Date       | Claude Code |     Codex |      Cost
───────────|─────────────|───────────|─────────────────────────────────────────
2026-03-31 |   $1,234.50 |     $6.10 | $1,240.60 ██████████████████████████████
───────────|─────────────|───────────|─────────────────────────────────────────
2026-04-01 |     $890.10 |     $3.20 |   $893.30 █████████████████████▋
2026-04-02 |   $1,007.85 |     $5.45 | $1,013.30 ████████████████████████▌
───────────|─────────────|───────────|─────────────────────────────────────────
Total      |   $3,132.45 |    $14.75 | $3,147.20
avg $1,049.07/day · this month $1,906.60 · peak $1,240.60 (2026-03-31) · █ Claude Code █ Codex
```

Each bar is stacked per tool (colors not shown in ASCII): the Claude Code share of each row renders green, the Codex share magenta — see the stacking bullet below.

Outlier window — 21 days in the $100–300 range (shown collapsed) plus two outliers; p95 = $1,012.50 and max $4,031.61 > 1.5 × p95, so the two-zone scale engages (see Layout 3 for the zone rules):

```
Date       | Claude Code |     Codex |      Cost
───────────|─────────────|───────────|──────────────────────────────
2026-06-01 | …           | …         |    $100.00 ██▏                ┊
   …       | …           | …         |       …    …                  ┊
2026-06-22 |   $1,085.56 |     $6.11 | $1,091.67 █████████████████████┊▎
2026-06-23 |   $4,025.50 |     $6.11 | $4,031.61 █████████████████████┊████████
───────────|─────────────|───────────|──────────────────────────────
Total      |   $9,212.67 |   $110.61 | $9,323.28
avg $405.36/day · this month $9,323.28 · peak $4,031.61 (2026-06-23) · ┊ = $1,012.50 (p95) · █ Claude Code █ Codex
```

- **Columns:** Date (10 left-aligned — ISO daily labels are 10 chars, monthly 7), one per tool sized to `max(toolName.length, 9)` right-aligned (variable per-tool width — e.g. `Claude Code` → 11, all shorter names floor to 9), row Cost (9 right-aligned — fits `$9,999.99` with thousands separators)
- **Zero-column omission:** tool columns whose cost totals zero across the visible window (post-`maxRows` labels) are omitted entirely — above, Gemini/Copilot/Kimi/OpenCode have no cost in the window, so only Claude Code and Codex render. If every tool is zero the renderer falls back to the full list. The CSV emitter is the exception: it keeps every registry column with raw `0.00` cells (positional machine contract)
- Variable-width columns keep the **full 6-tool data row — Date + tool columns + the 3-char gutter + the 9-wide Cost cell — at 96 chars**: `10 + (11+9+9+9+9+9) + 6×3 + 3 + 9 = 96`, so the all-tools-active pivot needs a ≥96-col terminal. With zero-column omission the typically rendered width is far below 80, restoring the inline bar chart on standard terminals (at 96–106 cols with all six tools active the bar is suppressed below the `MIN_BAR_AREA` threshold, so no line exceeds 96)
- **Watch mode** appends a delta indicator (`↑`/`↓`) after the Cost cell when `prevCosts` is set. In this pivot only, the indicator is rendered **without its leading space** (`$1,013.30↑` — 1 visible char), so the full watch-mode row is 96 + 1 = **97 chars** and fits a ≥97-col terminal without wrapping (the spaced ` ↑` form would add one more char and wrap, corrupting the watch compositor's line-counting). Other renderers keep the spaced form (they have width headroom). A tool crossing $0 → nonzero mid-watch gains a column on the next render — the compositor re-measures every frame
- **Stacked bars:** bars scale to row total cost with the same length and two-zone geometry as Layout 3, but the main-zone fill (or the whole bar in a single-zone window) is split into contiguous per-tool segments, left to right in column order, each segment's length proportional to that tool's share of the row cost. Segments are apportioned by largest-remainder rounding over the bar's character count (ties break to the earlier column), so they always sum exactly to the unstacked bar's length; the fractional-eighths character rides the last (rightmost) segment; a tool whose share rounds to zero characters gets no segment. Segment colors come from the fixed palette **green, magenta, blue, cyan**, assigned in visible column order — a 5th+ visible tool renders uncolored. The overflow zone past the `┊` rule stays solid yellow with no segmentation (proportional segments would mislead on the compressed scale). Under `--no-color`/`NO_COLOR` the segments collapse to solid blocks indistinguishable from today's total bar
- **Legend:** when stacked bars render (bars visible ∧ ≥2 visible tools ∧ color enabled), the summary footer appends one colored `█` swatch per visible tool in column order, each followed by the tool name — `· █ Claude Code █ Codex`. Omitted under `--no-color`/`NO_COLOR` (uncolored swatches carry no information), when bars are suppressed (narrow terminal), and with a single visible tool
- **Month separators, current-period marker, weekend dimming, summary footer, and p95 two-zone scale:** same rules as Layout 3 — separators daily-only, the current-period row's date cell renders boldWhite, Saturday/Sunday date cells render dim (daily only, today marker wins), the dim footer follows the Total row (≥2 labels), and the `┊` scale-break rule with yellow overflow zone engages when `max > 1.5 × p95` of the nonzero row totals

## 5. Watch Mode — Full Screen

**Command:** `tu -w`, `tu cc h -w`, `tu mh -w`

Enters alternate screen buffer. Layout adapts to terminal dimensions:

### Full mode (>= 60 cols): stats grid + table + rain

```
 Elapsed  5m 32s     Tok/min   ~12,345
 Session  +$0.50     Rate      ~$1.25/hr
                     Proj. day ~$15.00
─────────────────────────────────────────────
📊 Combined Cost History (daily)

Date       | Claude Code |     Codex |      Cost
───────────|─────────────|───────────|─────────────────────────────────────────
2026-03-04 |   $1,234.50 |     $6.10 | $1,240.60 ██████████████████████████████
2026-03-05 |     $890.10 |     $3.20 |   $893.30 █████████████████████▋
2026-03-06 |   $1,007.85 |     $5.45 | $1,013.30↑ ████████████████████████▌
───────────|─────────────|───────────|─────────────────────────────────────────
Total      |   $3,132.45 |    $14.75 | $3,147.20

                  ﾗ0ﾑa                    7ﾘ
                  ﾗ                        ﾘk
                  ﾗ                          Z
                   5                         q
                                             ﾝ
Next refresh: 8s · ↵ refresh · q quit
```

- **Stats grid:** 2x3 grid above the table — session stats left (Elapsed, Session), cost stats right (Tok/min, Rate, Proj. day)
- **Separator:** dim horizontal rule between stats grid and table title
- **Table:** any of Layouts 1-4, depending on command args — same render functions as non-watch mode
- **Rain:** matrix rain fills vertical space below content (or right margin if no vertical space)
- **Footer:** status line at terminal bottom row, all `dim`
- Unavailable stats show `--` placeholder; grid stays fixed at 3 rows

### Compact mode (< 60 cols)

```
Claude Code      $12.34 ↑
Codex            $23.45
OpenCode          $4.56
──────────────────────────
Total            $40.35

Refreshing... · ↵ refresh · q quit
```

- Two columns only: name (14 left-padded) + cost (12 right-padded)
- No token breakdown, no stats grid, no rain, no bars

## 6. Watch Mode — Stats Grid Detail

### Full stats (2+ polls)

```
 Elapsed  5m 32s     Tok/min   ~12,345
 Session  +$0.50     Rate      ~$1.25/hr
                     Proj. day ~$15.00
```

- **Elapsed:** `Xh Xm Xs` / `Xm Xs` / `Xs`
- **Session:** cost delta since watch start (shown as `$0.00` before 2 polls)
- **Tokens/min:** `--` until 2+ polls with totalTokens > 0
- **Rate:** 5-poll rolling window burn rate, shown in `yellow`; `--` until 2+ polls
- **Proj. day:** today's cost + rate * remaining hours; `--` until 2+ polls
- Labels `dim`, values `boldWhite`

### Loading skeleton (before first fetch)

```
 Elapsed  0s         Tok/min   --
 Session  $0.00      Rate      --
                     Proj. day --
─────────────────────────────────────────────
📊 Combined Usage (daily)

Tool         |       Tokens |        Input |       Output |        Cache |         Cost
─────────────|──────────────|──────────────|──────────────|──────────────|─────────────
                                      Loading...
```

## 7. Watch Mode — Delta Indicators

In watch mode, cost cells gain directional arrows after each poll:

```
$12.34 ↑     green up-arrow: cost increased since last poll
$23.45 ↓     red down-arrow: cost decreased since last poll
$4.56        no indicator: first poll or no change
```

Tracked per item via `{toolName}:{label}` or `total:{label}` key.

## 8. Watch Mode — Matrix Rain

Fills available terminal space with falling characters:

```
        ﾗ0ﾑa                    7ﾘ
        ﾗ                        ﾘk        ← bright green (head)
        ﾗ                          Z       ← green (body)
         5                         q       ← dim green (tail)
                                   ﾝ
```

- **Characters:** katakana + digits + latin
- **Density:** ~30% of available columns active
- **Speed:** 0.3-1.0 rows per 107ms tick (fractional)
- **Trail:** 3-8 chars with brightness gradient (brightGreen head, green body, dimGreen tail)
- **Shimmer:** ~5% of trail chars randomly replaced each tick
- **Positioning:** below content (preferred) or right margin (fallback, >= 10 cols); disabled with `--no-rain`

## 9. Watch Mode — Footer States

```
Next refresh: 45s · ↵ refresh · q quit     ← countdown (dim)
Refreshing... · ↵ refresh · q quit         ← fetching (dim)
```

Truncates progressively in narrow terminals: controls dropped first, then status text.

## 10. JSON Output

**Command:** `tu --json`, `tu cc h --json`

```json
{
  "Claude Code": {
    "totalCost": 12.34,
    "inputTokens": 567890,
    "outputTokens": 666677,
    "cacheCreationTokens": 23456,
    "cacheReadTokens": 12345,
    "totalTokens": 1234567
  }
}
```

Incompatible with `--watch` (exits with error).

## 11. Status

**Command:** `tu status`

### Single mode

```
Mode:        single
Config:      ~/.tu.conf (v2)
```

Or when no config file exists:

```
Mode:        single (no ~/.tu.conf)
```

### Multi mode

```
Mode:        multi
User:        sahil
Machine:     my-macbook
Config:      ~/.tu.conf (v2)
Metrics:     ~/.tu/metrics_repo
Last sync:   5m ago (2026-03-06T14:23:45.123Z)
Auto-sync:   on
```

When metrics dir is missing:

```
Metrics:     ~/.tu/metrics_repo (NOT FOUND — run 'tu init-metrics')
```

## 12. Help

**Command:** `tu help`, `tu -h`, `tu --help`

```
Usage: tu [source] [period] [display]

Sources: cc (Claude Code), codex/co (Codex), oc (OpenCode), gemini/gem (Gemini), copilot/cop (Copilot), kimi/ki (Kimi), all (default)
Periods: d/daily (default), m/monthly
Display: (bare) = snapshot, h/history = history
Combined: dh (daily history), mh (monthly history)

Examples:
  tu                   Today's cost, all tools (snapshot)
  tu cc                Today's cost, Claude Code
  tu h                 Daily cost history, all tools (pivot)
  tu cc mh             Monthly cost history, Claude Code
  tu m                 This month's cost, all tools

Setup:
  tu init-conf         Scaffold ~/.tu.conf
  tu init-metrics      Clone metrics repo
  tu sync              Push/pull metrics manually
  tu status            Show config and sync state

Help: tu help | tu -h | tu --help

Flags:
  --json               Output data as JSON (data commands only)
  --sync               Sync metrics before fetching (multi mode)
  --fresh / -f         Bypass cache, fetch fresh data (data commands only)
  --watch / -w         Persistent polling mode with live display (data commands only)
  --interval / -i <s>  Poll interval in seconds (default: 10, range: 5-3600)
  --no-color           Disable ANSI color output
  --no-rain            Disable matrix rain animation in watch mode
```

## Color Reference

| Function | ANSI | Usage |
|----------|------|-------|
| `boldWhite` | `\x1b[1;37m` | titles, total rows, stat values |
| `boldCyan` | `\x1b[1;36m` | column headers |
| `dim` | `\x1b[2m` | dividers, labels, footer, weekend dates |
| `green` | `\x1b[32m` | single-tool history bars, up-arrow delta, pivot bar segment (1st tool) |
| `red` | `\x1b[31m` | down-arrow delta |
| `cyan` | `\x1b[36m` | pivot bar segment (4th tool) |
| `magenta` | `\x1b[35m` | pivot bar segment (2nd tool) |
| `blue` | `\x1b[34m` | pivot bar segment (3rd tool) |
| `yellow` | `\x1b[33m` | p95 overflow zone (reserved — beyond-scale bars only), burn rate |
| `brightGreen` | `\x1b[92m` | rain head |
| `dimGreen` | `\x1b[2;32m` | rain tail |

All colors disabled by `--no-color` flag or `NO_COLOR` env var.