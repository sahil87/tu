# Usage Spec

> How the `tu` CLI works: commands, argument grammar, data flow, output modes, and configuration.

## CLI Grammar

```
tu [source] [period] [display] [flags]
```

### Sources

| Token | Resolves to | Tool command |
|-------|-------------|-------------|
| `cc` | Claude Code | `ccusage claude` |
| `codex`, `co` | Codex | `ccusage codex` |
| `oc` | OpenCode | `ccusage opencode` |
| `gemini`, `gem` | Gemini | `ccusage gemini` |
| `copilot`, `cop` | Copilot | `ccusage copilot` |
| `all` (default) | All five tools | — |

`co` → `codex`, `gem` → `gemini`, `cop` → `copilot` are aliases. When no source is given, defaults to `all`.

### Periods

| Token | Meaning |
|-------|---------|
| `d`, `daily` (default) | Daily granularity |
| `w`, `weekly` | Weekly granularity (aggregated from daily) |
| `m`, `monthly` | Monthly granularity (aggregated from daily) |

### Display

| Token | Meaning |
|-------|---------|
| (bare, default) | Snapshot — current day/week/month only |
| `h`, `history` | History table (daily/weekly default to the last 3 calendar months; use `--full` for all history — monthly is never capped) |
| `dh` | Combined: daily + history |
| `wh` | Combined: weekly + history |
| `mh` | Combined: monthly + history |

### Examples

| Command | What it shows |
|---------|---------------|
| `tu` | Today's cost, all tools (snapshot) |
| `tu cc` | Today's cost, Claude Code only |
| `tu h` | Daily cost history, all tools (pivot table) |
| `tu cc mh` | Monthly cost history, Claude Code |
| `tu wh` | Weekly cost history, all tools |
| `tu m` | This month's cost, all tools |

## Global Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--json` | — | Output as JSON (data commands only, incompatible with `--watch`) |
| `--sync` | — | Sync metrics before fetching (multi mode only) |
| `--dry-run` | — | Preview a sync without writing (honored only by `tu sync`; other invocations error) |
| `--fresh` | `-f` | Bypass cache, fetch fresh data |
| `--full` | — | Show full history (default is the last 3 months for daily/weekly history; no effect on monthly or snapshot) |
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

## Exit Codes

`tu` follows the sahil87 toolkit convention (principle №4 — *fail fast with actionable errors*):

- **`0`** — success. The command did what was asked (this includes benign no-op outcomes, e.g. `tu update` on a non-Homebrew install, "already up to date", or `tu shell-init` with no argument printing its usage listing).
- **`1`** — operational failure. The invocation was well-formed but the operation could not complete: a network/git/Homebrew failure, a missing/misconfigured metrics repo, or an unexpected runtime error. The caller's recovery is to retry or fix the environment/config, not the command line.
- **`2`** — usage error. The invocation itself was wrong: an unknown argument or tool, an unknown shell, a bad flag value, or incompatible format flags. The caller's recovery is to fix the arguments. Error text (and, for the data commands, a short usage hint) is written to stderr.

Per-subcommand exit codes:

| Command | `0` | `1` | `2` |
|---------|-----|-----|-----|
| `tu [source] [period] [display]` (data commands, incl. `--watch`) | success | unexpected runtime error | unknown argument/tool, bad flag value, incompatible format flags (`--json`/`--csv`/`--md`/`--watch`), bad/inverted `--since`/`--until`, bad `--interval`, missing `-u` value |
| `tu sync` | success | `metrics_repo` unset, clone/sync failure | — |
| `tu init-metrics` | success | `metrics_repo` unset, metrics dir exists but is not a git repo | — |
| `tu update` | success (incl. non-Homebrew install message, "already up to date") | `brew update`/`brew info`/`brew upgrade` failure | — |
| `tu shell-init [shell]` | success (script emitted; no-arg usage listing) | — | unknown shell |
| `tu init-conf`, `tu status`, `tu help`, `tu --version` | success | unexpected runtime error | — |

Diagnostics on any error path go to stderr; stdout carries data only (principle №2).

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
  label: string; // ISO date "YYYY-MM-DD" (daily; also the week's Sunday for weekly) or month "YYYY-MM"
}
```

Tool configs define the five supported tools (`cc`, `codex`, `oc`, `gemini`, `copilot`), each with a display name, binary command path, a per-tool `labelKey` (the JSON key carrying the ISO date label — all per-agent subcommands emit `"date"`), and a `needsFilter` flag (Codex/OpenCode retain the flag for defensive noise stripping; v20 emits clean JSON).

## Data Flow

### Fetching

1. Each tool is invoked via its binary with `daily --json` args
2. Output is parsed as JSON; the `daily` array is extracted as `UsageEntry[]`
3. Labels are normalized from human-readable ("Feb 14, 2026") to ISO format ("2026-02-14")
4. Weekly and monthly data are computed by aggregating daily entries (via `aggregateForPeriod`, which routes to `aggregateWeekly`/`aggregateMonthly`; daily is the identity). Monthly slices the label to "YYYY-MM"; weekly keys each day under its week's **Sunday** as an ISO date ("YYYY-MM-DD"), computed with UTC arithmetic on the date-only label (immune to DST) and aligned with `ccusage weekly`'s default `--start-of-week sunday`. Both sum the numeric fields.

### Caching

- Fetched daily entries are cached per-tool at `~/.tu/cache/{tool}-daily.json`
- Cache TTL: 60 seconds (checked via file mtime)
- Cache is bypassed when `--fresh` flag is set or extra args are passed
- Non-fatal: write failures are silently ignored

### Snapshot vs History

- **Snapshot**: fetches all entries, then filters to the one matching `currentLabel(period)` (today's date, the current week's Sunday, or the current month). `currentLabel` uses local-time date methods (the weekly case backs up to Sunday via `setDate(getDate() - getDay())`, normalizing month/year underflow). Shows a cross-tool table with one row per tool.
- **History**: fetches all entries, shows a table with one row per date/month. Daily and weekly history default to the last 3 calendar months (an implicit `--since` floor at the first of the month two months back, disabled by `--full` or any explicit `--since`/`--until`); monthly history is never capped. When the cap is active the table heading carries a `last 3 months` hint. Single-tool history shows token breakdown; all-tools history shows a cost pivot table (date rows x tool columns).

## Output Formats

### Snapshot Table (all tools)

Columns: Tool, Tokens, Input, Output, Cost. One row per tool with non-zero tokens, plus a Total row. Heading: "Combined Usage (daily|weekly|monthly)".

### Single-Tool History Table

Columns: Date, Input, Output, Cache Write, Cache Read, Total, Cost. Includes inline bar charts (Unicode block elements at eighths precision, scaled to max cost in the table). Total row when >1 entry. Heading: "{Tool Name} (daily|weekly|monthly)".

### All-Tools History Pivot Table

Columns: Date, {Tool1}, {Tool2}, ..., Cost. Each cell is a cost value. Includes inline bar charts for row totals. Total row with per-tool sums. Heading: "Combined Cost History (daily|weekly|monthly)".

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

#### Dry Run (`tu sync --dry-run`)

`tu sync --dry-run` previews the sync without touching the working tree, the metrics repo, or the network, then prints the preview to stdout and exits 0. It shares the real write-decision path (`writeMetrics`'s never-shrink guard runs identically), so the preview cannot drift from a live sync. It reports:

- which day-files **would be written** (new, or `update: X → Y`)
- which **would be skipped** by the never-shrink guard (incoming cost < existing)
- the commit that **would** be made (same `# {user}: update {date}` message), then the `pull --rebase` / `push` that would follow

The git half is computed locally — only a read-only `git status --porcelain {user}/` is invoked; `pull`/`push` are reported, never executed or probed. The flag is honored **only** by `tu sync`; any other invocation carrying `--dry-run` (e.g. `tu cc --dry-run`, `tu cc --sync --dry-run`) fails fast on stderr with exit 1, because the multi-mode fetch path writes day-files outside the sync boundary and a combined preview-then-proceed would mutate the files it just previewed.

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
