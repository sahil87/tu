# Usage Spec

> How the `tu` CLI works: commands, argument grammar, data flow, output modes, and configuration.

## CLI Grammar

```
tu [source] [period] [display] [flags]
```

### Sources

| Token | Resolves to | Tool binary |
|-------|-------------|-------------|
| `cc` | Claude Code | `ccusage` |
| `codex`, `co` | Codex | `ccusage-codex` |
| `oc` | OpenCode | `ccusage-opencode` |
| `all` (default) | All three tools | — |

`co` is an alias for `codex`. When no source is given, defaults to `all`.

### Periods

| Token | Meaning |
|-------|---------|
| `d`, `daily` (default) | Daily granularity |
| `m`, `monthly` | Monthly granularity (aggregated from daily) |

### Display

| Token | Meaning |
|-------|---------|
| (bare, default) | Snapshot — current day/month only |
| `h`, `history` | Full history table |
| `dh` | Combined: daily + history |
| `mh` | Combined: monthly + history |

### Examples

| Command | What it shows |
|---------|---------------|
| `tu` | Today's cost, all tools (snapshot) |
| `tu cc` | Today's cost, Claude Code only |
| `tu h` | Daily cost history, all tools (pivot table) |
| `tu cc mh` | Monthly cost history, Claude Code |
| `tu m` | This month's cost, all tools |

## Global Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--json` | — | Output as JSON (data commands only, incompatible with `--watch`) |
| `--sync` | — | Sync metrics before fetching (multi mode only) |
| `--fresh` | `-f` | Bypass cache, fetch fresh data |
| `--watch` | `-w` | Persistent polling mode with live TUI display |
| `--interval` | `-i <s>` | Poll interval in seconds (default: 10, range: 5-3600, requires `--watch`) |
| `--no-color` | — | Disable ANSI color output (also respects `NO_COLOR` env var) |
| `--no-rain` | — | Disable matrix rain animation in watch mode |
| `--version` | `-V` | Print version and exit |
| `--help` | `-h` | Print full help and exit |

Flag parsing strips all flags before positional argument parsing. Unknown positional args produce an error with short usage hint.

## Setup Commands

| Command | Description |
|---------|-------------|
| `tu init-conf` | Scaffold `~/.tu.conf` with all fields; if file exists, appends missing fields and warns about commented-out ones |
| `tu init-metrics` | Clone the metrics git repo (requires `mode=multi` and `metrics_repo` set in config) |
| `tu sync` | Manually push/pull metrics (requires multi mode) |
| `tu status` | Show current config: mode, user, machine, metrics dir, last sync time, auto-sync state |

Setup commands are dispatched before positional argument parsing; they ignore `--json`/`--fresh`/`--watch`.

## Data Model

All data flows through two core interfaces:

```typescript
interface UsageTotals {
  totalCost: number;
  inputTokens: number;
  outputTokens: number;
  cacheCreationTokens: number;
  cacheReadTokens: number;
  totalTokens: number;
}

interface UsageEntry extends UsageTotals {
  label: string; // ISO date "YYYY-MM-DD" or month "YYYY-MM"
}
```

Tool configs define the three supported tools (`cc`, `codex`, `oc`), each with a display name, binary command path, and a `needsFilter` flag (Codex/OpenCode output contains noise lines starting with `[` that must be stripped before JSON parsing).

## Data Flow

### Fetching

1. Each tool is invoked via its binary with `daily --json` args
2. Output is parsed as JSON; the `daily` array is extracted as `UsageEntry[]`
3. Labels are normalized from human-readable ("Feb 14, 2026") to ISO format ("2026-02-14")
4. Monthly data is computed by aggregating daily entries (slicing label to "YYYY-MM" and summing fields)

### Caching

- Fetched daily entries are cached per-tool at `~/.tu/cache/{tool}-daily.json`
- Cache TTL: 60 seconds (checked via file mtime)
- Cache is bypassed when `--fresh` flag is set or extra args are passed
- Non-fatal: write failures are silently ignored

### Snapshot vs History

- **Snapshot**: fetches all entries, then filters to the one matching `currentLabel(period)` (today's date or current month). Shows a cross-tool table with one row per tool.
- **History**: fetches all entries, shows full table with one row per date/month. Single-tool history shows token breakdown; all-tools history shows a cost pivot table (date rows x tool columns).

## Output Formats

### Snapshot Table (all tools)

Columns: Tool, Tokens, Input, Output, Cost. One row per tool with non-zero tokens, plus a Total row. Heading: "Combined Usage (daily|monthly)".

### Single-Tool History Table

Columns: Date, Input, Output, Cache Write, Cache Read, Total, Cost. Includes inline bar charts (Unicode block elements at eighths precision, scaled to max cost in the table). Total row when >1 entry. Heading: "{Tool Name} (daily|monthly)".

### All-Tools History Pivot Table

Columns: Date, {Tool1}, {Tool2}, ..., Cost. Each cell is a cost value. Includes inline bar charts for row totals. Total row with per-tool sums. Heading: "Combined Cost History (daily|monthly)".

### JSON Output (`--json`)

Data commands emit `JSON.stringify(data, null, 2)`. Maps are converted to plain objects via `Object.fromEntries`. Structure mirrors the internal data shape (map of tool name to entries/totals).

### Delta Indicators

In watch mode, cost cells show up/down arrows (green up-arrow when cost increased vs previous poll, red down-arrow when decreased) using per-item cost tracking keyed by `{toolName}:{label}` or `total:{label}`.

## Multi-Machine Mode

### Configuration (`~/.tu.conf`)

INI-style key=value file (lines starting with `#` are comments). Fields:

| Field | Default | Description |
|-------|---------|-------------|
| `version` | 2 | Config schema version |
| `mode` | `single` | `single` or `multi` |
| `metrics_repo` | — | Git repo URL for metrics storage (required for multi) |
| `metrics_dir` | `~/.tu/metrics_repo` | Local clone path |
| `machine` | `$HOSTNAME` | Machine label |
| `user` | `$USER` | User/profile label |
| `auto_sync` | `true` | Whether auto-sync is enabled |

A defaults file (`tu.default.conf`) provides base values; user config overrides. Sentinel values `$HOSTNAME` and `$USER` are expanded at runtime. Supports a `WEAVER_DEV` env var that switches the defaults file to `tu.default.weaver.conf`.

### Metrics Repo Layout

```
{user}/{year}/{machine}/{tool}-{date}.jsonl
```

Each file contains one JSON line with a `UsageEntry`. Local entries are written before every multi-mode fetch. Remote entries are read from all user/year/machine paths except the current user+machine combination, then merged with local entries by summing same-label fields.

### Sync Flow (`tu sync` / `--sync`)

1. Fetch fresh local data for all tools
2. Write local entries to metrics repo
3. `git add {user}/` + commit (if changes) + `pull --rebase` + `push` (retry once on push failure)
4. Touch `.last-sync` timestamp file in `~/.tu/`

### Auto-Clone Guard

When multi mode is configured but the metrics dir doesn't exist:
1. If no `metrics_repo` set: warn on stderr, fall back to single mode
2. If a recent clone failure marker exists (`~/.tu/.clone-failed`, < 3 hours): warn, fall back to single mode
3. Otherwise: attempt `git clone` with 30s timeout and `GIT_TERMINAL_PROMPT=0`; on failure, write clone marker and fall back to single mode
4. Successful clone or `init-metrics` clears the clone failure marker

## Watch Mode (`--watch`)

Full-screen TUI using alternate screen buffer with compositor-based rendering.

### Architecture

- **Compositor**: manages independent panel buffers (stats, table, status) with dirty-flag rendering at 16ms ticks
- **StatsPanel**: 2x3 stats grid rendered above the table
- **TablePanel**: data table output from dispatch functions (same render functions as non-watch mode)
- **StatusPanel**: footer with countdown timer and controls
- **RainLayer**: matrix rain animation (107ms tick, cursor-positioned overlay, independent of compositor tick)

### Layout

- Full mode (>= 60 cols): stats grid + dim separator + full table + rain
- Compact mode (< 60 cols): compact table only, no stats grid, no rain
- Rain fills available space below content, or right margin columns if no vertical space remains
- Loading skeleton renders on alt-screen entry before first fetch (stats grid with zeros/dashes, table header, centered "Loading...")

### Interaction

- `q` or Ctrl+C: exit (restores normal screen, prints last rendered output to stdout)
- Enter/Space: immediate refresh (cancels countdown)
- Polls on interval (default 10s), shows "Refreshing..." during fetch

### Session Stats (grid above table)

- **Elapsed**: wall-clock time since first poll
- **Session cost delta**: difference between current poll cost and first poll cost; `$0.00` before 2 polls
- **Tokens/min**: `totalTokens / elapsedMinutes`; `--` before 2 polls
- **Burn rate**: rolling window of last 5 polls, `(latest.cost - oldest.cost) / timeDelta * 3600000` ($/hr); `--` before 2 polls
- **Projected daily cost**: `todayCost + burnRate * hoursRemainingInDay`; `--` before 2 polls
- Grid stays fixed at 3 rows; unavailable stats show `--` placeholder

### Matrix Rain

Katakana + digits + latin characters falling at variable speeds (0.3-1.0 rows/tick). Column density ~30%. Trail length 3-8 characters with brightness gradient (bright head, green body, dim tail). ~5% shimmer rate for random character replacement. Respawns after falling off screen with random delay. Tick interval: 107ms (75% of original 80ms for calmer ambient feel).
