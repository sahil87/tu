# Layouts

> Visual mockups of every distinct output layout produced by `tu`. Each section shows the
> command that triggers it, the ASCII mockup, and notes on column sizing and color.
>
> For data model, flag semantics, and watch mode architecture, see [usage.md](usage.md).

## 1. Snapshot — All Tools

**Command:** `tu`, `tu d`, `tu m`, `tu all`

```
📊 Combined Usage (daily)

Tool           |        Tokens |        Input |       Output |         Cost
────────────────────────────────────────────────────────────────────────────
Claude Code    |     1,234,567 |      567,890 |      666,677 |       $12.34
Codex          |     2,345,678 |      987,654 |    1,358,024 |       $23.45
OpenCode       |       456,789 |      234,567 |      222,222 |        $4.56
────────────────────────────────────────────────────────────────────────────
Total          |     4,037,034 |    1,790,111 |    2,246,923 |       $40.35
```

- **Columns:** Tool (14 left-aligned), Tokens/Input/Output/Cost (14 right-aligned each)
- **Separator:** ` | ` between columns
- **Colors:** header row `boldCyan`, dividers `dim`, Total row `boldWhite`
- Tools with zero tokens are omitted; Total row shown only when >1 tool has data

## 2. Snapshot — Single Tool

**Command:** `tu cc`, `tu codex m`, `tu oc`

Same table as Layout 1 but with a single data row. Title uses tool name:

```
📊 Claude Code (daily)

Tool           |        Tokens |        Input |       Output |         Cost
────────────────────────────────────────────────────────────────────────────
Claude Code    |     1,234,567 |      567,890 |      666,677 |       $12.34
```

No divider/Total row when only one row is present.

## 3. History — Single Tool

**Command:** `tu cc h`, `tu cc dh`, `tu codex mh`

```
📊 Claude Code (daily)

Date         |        Input |       Output |  Cache Write |   Cache Read |        Total |     Cost
──────────────────────────────────────────────────────────────────────────────────────────────────────
2026-03-06   |      567,890 |      678,901 |       45,678 |       23,456 |    1,315,925 |    $2.85  ██████████████████
2026-03-05   |      456,789 |      567,890 |       23,456 |       12,345 |    1,060,480 |    $2.10  █████████████▍
2026-03-04   |      234,567 |      345,678 |       10,000 |        5,000 |      595,245 |    $1.50  █████████▌
──────────────────────────────────────────────────────────────────────────────────────────────────────
Total        |    1,259,246 |    1,592,469 |       79,134 |       40,801 |    2,971,650 |    $6.45
```

- **Columns:** Date (12 left-aligned), Input/Output/Cache Write/Cache Read/Total (14 right-aligned), Cost (8 right-aligned)
- **Bar chart:** green Unicode block elements (full + fractional eighths), max width 30, scaled to max cost
- Bars only render when terminal width allows (>= 10 chars remaining after cost column)
- Total row shown when >1 entry

## 4. History — All Tools (Pivot)

**Command:** `tu h`, `tu dh`, `tu mh`

```
📊 Combined Cost History (daily)

Date         | Claude Code    | Codex          | OpenCode       |     Cost
──────────────────────────────────────────────────────────────────────────────
2026-03-06   |         $2.85  |         $5.45  |         $0.67  |    $8.97  ██████████████████
2026-03-05   |         $2.10  |         $3.20  |         $0.45  |    $5.75  ███████████▌
2026-03-04   |         $1.50  |         $6.10  |         $1.20  |    $8.80  █████████████████▋
──────────────────────────────────────────────────────────────────────────────
Total        |         $6.45  |        $14.75  |         $2.32  |   $23.52
```

- **Columns:** Date (12 left-aligned), one per tool (14 right-aligned, cost values), row Cost (8 right-aligned)
- Bars scale to row total cost, same rendering as Layout 3
- Tool columns only appear if that tool has data for the period

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

Date         | Claude Code    | Codex          |     Cost
──────────────────────────────────────────────────────────
2026-03-06   |         $2.85  |         $5.45  |    $8.97 ↑
2026-03-05   |         $2.10  |         $3.20  |    $5.75
2026-03-04   |         $1.50  |         $6.10  |    $8.80
──────────────────────────────────────────────────────────
Total        |         $6.45  |        $14.75  |   $23.52

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

Tool           |        Tokens |        Input |       Output |         Cost
────────────────────────────────────────────────────────────────────────────
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

Sources: cc (Claude Code), codex/co (Codex), oc (OpenCode), all (default)
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
| `dim` | `\x1b[2m` | dividers, labels, footer |
| `green` | `\x1b[32m` | bar charts, up-arrow delta |
| `red` | `\x1b[31m` | down-arrow delta |
| `yellow` | `\x1b[33m` | burn rate |
| `brightGreen` | `\x1b[92m` | rain head |
| `dimGreen` | `\x1b[2;32m` | rain tail |

All colors disabled by `--no-color` flag or `NO_COLOR` env var.