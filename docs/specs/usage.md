# Usage Spec

> How the `tu` CLI works: commands, argument grammar, data flow, output modes, configuration, and the toolkit contracts it honors.
>
> This file and [layouts.md](layouts.md) are the complete external contract for `tu`: every observable behavior of the shipped binary — the Go build from `src/go/` (repository at v0.12.1; cutover release 0.13.0). The contract was reconciled 2026-09-16 against the then-shipped v0.11.5 Node binary and the memory tree of that date (capture set in `fab/changes/260915-2y3l-spec-reconciliation/reconciliation.md`). Internal mechanisms stay in `docs/memory/`. A trailing `(DC-NN)` on a line means the behavior is specified as it exists today **and** proposed for a keep/drop decision in the **Drop at cutover** ledger at the end of this file.

## CLI Grammar

```
tu [source] [period] [display] [flags]
```

Positional tokens may appear in any order relative to flags; flags are stripped first, then the positionals are classified. At most one token of each class is accepted — a second source (`tu cc codex`) or any unrecognized word is an unknown argument: `Unknown argument: codex` plus the short usage block on stderr, exit 2.

### Sources

| Token | Resolves to | Tool command |
|-------|-------------|-------------|
| `cc` | Claude Code | `ccusage claude` |
| `codex`, `co` | Codex | `ccusage codex` |
| `oc` | OpenCode | `ccusage opencode` |
| `gemini`, `gem` | Gemini | `ccusage gemini` |
| `copilot`, `cop` | Copilot | `ccusage copilot` |
| `kimi`, `ki` | Kimi | `ccusage kimi` |
| `all` (default) | All six tools | — |

`co` → `codex`, `gem` → `gemini`, `cop` → `copilot`, `ki` → `kimi` are aliases. When no source is given, defaults to `all`. Registry order — `cc, codex, oc, gemini, copilot, kimi` — is the column order in every all-tools view and the key order in every JSON object; new tools append.

### Periods

| Token | Meaning |
|-------|---------|
| `d`, `daily` (default) | Daily granularity |
| `w`, `weekly` | Weekly granularity (aggregated from daily; a week is labeled by its **Sunday**) |
| `m`, `monthly` | Monthly granularity (aggregated from daily; labeled `YYYY-MM`) |

### Display

| Token | Meaning |
|-------|---------|
| (bare, default) | Snapshot — current day/week/month only |
| `h`, `history` | History table (daily/weekly default to the last 3 calendar months; use `--full` for all history — monthly is never capped) |
| `dh` | Combined: daily + history (identical output to `tu h`) |
| `wh` | Combined: weekly + history |
| `mh` | Combined: monthly + history |
| `lb` | Leaderboard — one row per user for the current period, ranked by cost (or tokens under `--metric tokens`), with share and Δ vs the previous same-length period (multi mode only) |
| `lbh` | Leaderboard history — period rows × user columns, columns ranked by window total, per-row leader highlighted (multi mode only; daily/weekly carry the same 3-month cap / `--full` semantics as `h`) |

`lb`/`lbh` take the period from the separate period token (`tu lb`, `tu w lb`, `tu m lb`, `tu cc m lb`); there are no `dlb`/`wlb`/`mlb` shorthands. The source token scopes the leaderboard to that tool's spend.

### Examples

| Command | What it shows |
|---------|---------------|
| `tu` | Today's cost, all tools (snapshot) |
| `tu cc` | Today's cost, Claude Code only |
| `tu h` | Daily cost history, all tools (pivot table) |
| `tu cc mh` | Monthly cost history, Claude Code |
| `tu wh` | Weekly cost history, all tools |
| `tu m` | This month's cost, all tools |
| `tu m lb` | This month's leaderboard — users ranked by cost |
| `tu cc m lb` | This month's leaderboard, Claude Code spend only |
| `tu lbh` | Daily leaderboard history (users as columns) |

## Global Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--json` | `-j` | Output as JSON (data commands only; incompatible with `--csv`, `--md`, `--watch`) |
| `--csv` | — | Output as CSV (data commands only; incompatible with `--json`, `--md`, `--watch`) |
| `--md` | — | Output as GitHub-flavoured Markdown (data commands only; incompatible with `--json`, `--csv`, `--watch`) |
| `--since <date>` | `-s <date>` | Only include entries on/after the date — history displays and the leaderboards (`lb`: replaces the period window); `YYYY-MM-DD` or `YYYYMMDD`, shape-checked only (an impossible date such as `2026-13-01` yields an empty window); on a snapshot display: `Warning: --since/--until apply to history display — ignoring.` (exit 0) |
| `--until <date>` | — | Only include entries on/before the date (same shapes and guards as `--since`; long-only because `-u` is `--user`); `since > until` is exit 2 |
| `--full` | — | Show full history (default is the last 3 months for daily/weekly history and `lbh`); silently accepted on monthly history and with an explicit `--since`/`--until`; on a snapshot or `lb`: `Warning: --full applies to daily/weekly history — ignoring.` |
| `--metric <m>` | `-t` | Show `cost` (default) or `tokens` in table cells, bars and footer stats — all displays; the snapshot table keeps its Cost column in dollars (only the delta indicator follows the metric; compact snapshot cells use the metric). `-t` is a boolean shorthand ≡ `--metric tokens`; `-t --metric tokens` is accepted, `-t --metric cost` is exit 2. Missing/invalid value: `Error: --metric requires 'tokens' or 'cost'`, exit 2. No effect on `--json`/`--csv`/`--md` |
| `--top <n>` | — | Show only the top N leaderboard rows (`lb`, the rest collapse into a dim `… +k others` line) or user columns (`lbh`, the rest fold into one `others` column); positive integer, otherwise `Error: --top requires a positive integer`, exit 2; on other displays: `Warning: --top applies to leaderboard display — ignoring.` |
| `--sync` | — | Sync metrics before fetching (multi mode only); prints `syncing metrics... ` on stderr; a failed sync prints `sync failed — using local data.` and the command continues (exit 0) |
| `--dry-run` | — | Preview a sync without writing (honored only by `tu sync`; any other invocation carrying it — `tu --dry-run`, `tu cc --dry-run`, `tu cc --sync --dry-run` — fails fast: `Error: --dry-run is supported only with 'tu sync' — run 'tu sync --dry-run' to preview a sync.`, exit 2) |
| `--fresh` | `-f` | Bypass the 60-second fetch cache and fetch fresh data |
| `--user <user>` | `-u <user>` | Show usage for a specific user, or `all` to sum every user directory in the metrics repo (multi mode only; `all` reads synced repo data only, so today lags until `--sync`; `all` is a reserved profile name — a config `user = all` is rejected with exit 2). `-u <config user>` behaves exactly like no `-u`. With `--by-machine`, `-u all` breaks the total down per user instead of per machine (legend `Users:`; the JSON `machines` key carries user names). On `lb`/`lbh`, `-u <name>` pins/highlights that user's row instead of filtering and `-u all` is a no-op. In single mode: `Warning: -u flag requires multi mode — ignoring.` (the leaderboard's multi-mode guard fires first). Missing value: `Error: -u requires a username`, exit 2 |
| `--by-machine` | — | Per-machine breakdown columns (data commands only): snapshot and single-tool history gain letter-coded columns and a legend; `lb` rows become `user/machine`; the all-tools pivot and `lbh` warn and ignore it (`Warning: --by-machine is not supported with all-tools history — ignoring.` / `… with leaderboard history — ignoring.`); single mode shows one column (the local hostname); combines with `--watch`, `--json`, `--csv`, `--md`, `-u` |
| `--watch` | `-w` | Persistent polling mode with live TUI display (data commands only) |
| `--interval <s>` | `-i <s>` | Poll interval in seconds (default 10, range 5–3600); `Error: --interval requires a numeric value` / `minimum is 5 seconds` / `maximum is 3600 seconds`, exit 2; silently accepted without `--watch` (DC-03) |
| `--no-color` | — | Disable ANSI color output (also respects a non-empty `NO_COLOR` env var; the two are byte-identical) |
| `--no-rain` | — | Disable the matrix rain animation in watch mode; silently accepted without `--watch` (DC-03) |
| `--skip-brew-update` | — | `tu update` only: skip the internal `brew update` tap refresh (detected anywhere on the command line; ignored elsewhere) |
| `--version` | `-V`, `-v` | Print `tu version vX.Y.Z` and exit 0 (not listed in `--help`) (DC-09) |
| `--help` | `-h` | Print the full help and exit 0 — only as the first argument (`tu -h`, `tu --help`, `tu help`) or after `update`; after any other positional it is an unknown argument, exit 2 (DC-04) |

Flag parsing strips all flags before positional argument parsing. Unknown positional args produce `Unknown argument: {arg}` plus the short usage hint on stderr, exit 2. Format-flag conflicts print `Error: {a} and {b} are incompatible` with `--watch` named first when involved (`Error: --watch and --json are incompatible`), exit 2. Value-taking flags take their value as the next argument (`--since 2026-08-01`, `-i 30`); a missing value is exit 2 with the flag's own message. Off-target flags follow three policies — warn-and-continue, silent acceptance, or fail-fast — as listed per flag above (DC-03).

## Setup Commands

Non-data commands are recognized by their first argument and dispatched before grammar parsing. They ignore the data flags (`--json`, `--csv`, `--md`, `--fresh`, `--watch`, `-t`, …) silently — `tu sync --json` runs a real sync, `tu status --watch` prints status (DC-02).

| Command | Description |
|---------|-------------|
| `tu init-conf` | Scaffold `~/.config/tu/tu.conf` from the shipped defaults: `Created ~/.config/tu/tu.conf — edit it to configure multi-machine sync.`; when a legacy `~/.tu.conf` exists the new file is seeded from it instead: `Copied ~/.tu.conf → ~/.config/tu/tu.conf`; if the file already exists, reports commented-out fields: `~/.config/tu/tu.conf has commented-out fields that need uncommenting: metrics_repo.` Exit 0 in every case |
| `tu init-metrics [repo-url]` | Clone the metrics git repo into `metrics_dir`. With `<repo-url>`: first write `metrics_repo = <url>` into `~/.config/tu/tu.conf` (creating/seeding the file as `init-conf` does, replacing an active or commented `metrics_repo` line, else appending a commented block plus the value) and print `Set metrics_repo = <url> in ~/.config/tu/tu.conf`; the typed URL beats an exported `TU_METRICS_REPO` for the clone; then `Cloned <url> → <absolute metrics dir>`. Idempotent: an existing git repo prints `Already initialized: <absolute dir>` (exit 0) after the config write. Without `<repo-url>` it requires `metrics_repo` from org.conf, tu.conf, or `TU_METRICS_REPO`, else exit 1. A `metrics_dir` that exists but is not a git repo is exit 1. More than one positional is exit 2. Clears the clone-failure marker on success. Git's own clone chatter passes through on stderr; paths in these messages are absolute while `status` abbreviates them (DC-19) |
| `tu sync` | Manually push/pull metrics (requires multi mode): fetch fresh local data, write day-files, `git add {user}/`, commit if changed, `pull --rebase origin main`, `push` (one retry). Success prints `Synced to ~/.tu/metrics_repo` (exit 0) and touches `~/.tu/.last-sync`. Single mode: two stderr lines starting `tu sync requires metrics_repo to be set.`, exit 1 |
| `tu sync --dry-run` | Preview a sync without touching the working tree, repo, or network (see Dry Run below) |
| `tu status` | Show mode, user, machine, config path(s), metrics dir, last sync time, auto-sync state (layouts §13). Never clones or syncs |
| `tu update` | Update tu in place via Homebrew (see Toolkit Contracts › update) |
| `tu shell-init <bash\|zsh\|fish>` | Emit a static shell completion script on stdout (see Toolkit Contracts › shell-init) |
| `tu skill` | Print the agent usage bundle as raw markdown (see Toolkit Contracts › skill) |
| `tu help-dump` | Print the machine-readable help contract as JSON (see Toolkit Contracts › help-dump); not advertised in `--help` or completions |
| `tu help` | Print the full help (same as `-h`/`--help`), exit 0 |

**`$HOME` dependence.** Config-reading commands (all data commands, `init-conf`, `init-metrics`, `sync`, `status`) fail with `tu: $HOME is not set; cannot locate config` on stderr, exit 1, when `$HOME` is unset or empty. `help`, `--version`, `help-dump`, `skill`, `shell-init`, and `update` work without `$HOME`.

## Toolkit Contracts

`tu` is a member of the shll toolkit and honors its published standards (`shll standards`, read with `shll standards <name>`; this section was audited against **shll v0.1.32**, 2026-09-16). Each subsection states tu's observable behavior; the named standard is the binding text.

### `--version` (standard: `version`)

`tu --version`, `tu -V`, and `tu -v` print exactly one line, `tu version v0.11.5`, to stdout, exit 0, with no network I/O and no config read (works with `$HOME` unset). The binary name on `PATH` is `tu`, equal to the repo, formula leaf, and roster name.

### `help-dump` (standard: `help-dump`)

`tu help-dump` prints one pretty-printed JSON document (two-space indent, trailing newline) to stdout with nothing on stderr, exit 0, and no config read:

```json
{
  "tool": "tu",
  "version": "0.11.5",
  "schema_version": 1,
  "root": {
    "name": "tu",
    "path": "tu",
    "short": "AI coding assistant cost tracking CLI",
    "usage": "Usage: tu [source] [period] [display]",
    "text": "<the full --help text, byte-for-byte — it already ends with a newline>",
    "commands": []
  }
}
```

Key order is as shown. `root.text` is byte-identical to `tu --help` output including its trailing newline. `commands` is always an empty array (tu has no per-subcommand help pages — a flat document). No `captured_at` field is emitted (the puller stamps it). `help-dump` is hidden: it appears in neither `--help` nor the completion scripts.

### `update` (standard: `update`)

- `tu update --help` and `tu update -h` print the full help (which contains the literal `--skip-brew-update`) and exit 0 without running anything.
- Non-Homebrew install (the executable does not resolve under a `/Cellar/tu/` path): prints a help message and exits 0.
- Homebrew install: runs `brew update --quiet` (skipped with `--skip-brew-update`), then `brew info --json=v2 tu` to read the latest version; if it equals the running version prints an already-up-to-date message and exits 0; otherwise runs `brew upgrade tu` interactively with `HOMEBREW_NO_ASK=1` in its environment and no timeout, so it never prompts and is never killed mid-transaction. A failure of any brew step is a specific error on stderr, exit 1.
- The `update` subcommand ignores positional and data flags.

### `shell-init` (standard: `shell-init`)

`tu shell-init bash|zsh|fish` prints a static completion script for the named shell on stdout (eval-safe: shell source only, no color, no diagnostics), exit 0, no config read. Every script covers the full grammar — the non-data subcommands `help init-conf init-metrics sync status update shell-init skill` (not `help-dump`), all source/period/display tokens including `w`/`weekly`/`wh`/`lb`/`lbh`/`kimi`/`ki`/`gem`/`cop`, every long flag (`--json --csv --md --since --until --full --metric --top --sync --dry-run --fresh --watch --interval --user --by-machine --skip-brew-update --no-color --no-rain --version --help`), every short flag (`-f -w -i -u -s -j -t -v -V -h`), `cost tokens` as `--metric` values, and `bash zsh fish` as `shell-init` arguments. The scripts begin with a `# tu(1) {shell} completion` comment and an install hint (`eval "$(tu shell-init zsh)"`, or for fish a redirect into `~/.config/fish/completions/tu.fish`).

Failure paths write to **stderr with stdout empty** and exit 2: no argument prints the usage block (`Usage: tu shell-init <bash|zsh|fish>` plus three install lines); an unknown shell prints `Unknown shell: {shell}. Supported: bash, zsh, fish`.

### `skill` (standard: `skill`)

`tu skill` writes the agent usage bundle — the canonical `docs/site/skill.md` — to stdout **byte-for-byte**, with empty stderr, exit 0, no config read. The bundle is specified by reference to that file (it is embedded at build time and drift-guarded); it is not duplicated here. tu ships no `skill topics` pages.

### config-home (standard: `config-home`)

The config directory is `$HOME/.config/tu/` and nothing else moves it: `XDG_CONFIG_HOME`, `TU_CONFIG*`, and `TU_HOME` are ignored; an unset `$HOME` is the exit-1 error above. Runtime state lives separately under `~/.tu/` (fetch cache, metrics repo clone, `.last-sync`, `.clone-failed`). The override cascade is exactly `tu.default.conf < ~/.config/tu/org.conf < ~/.config/tu/tu.conf < TU_METRICS_REPO < CLI argument` with no per-key inversion; `TU_METRICS_REPO` is the only config-bearing environment variable (a deployment-bootstrap key). Details in Multi-Machine Mode › Configuration.

### install-composition (standard: `install-composition`)

tu's formula declares no dependency on any sibling toolkit formula and tu never invokes a sibling tool; install instructions live on hexokit.com, not in the README.

## Exit Codes

`tu` follows the shll toolkit convention (principle №4 — *fail fast with actionable errors*):

- **`0`** — success. The command did what was asked (this includes benign no-op outcomes, e.g. `tu update` on a non-Homebrew install, "already up to date", an empty table with `No usage`/`No data`, and every warn-and-continue flag guard).
- **`1`** — operational failure. The invocation was well-formed but the operation could not complete: a network/git/Homebrew failure, a missing/misconfigured metrics repo, `$HOME` unset, or an unexpected runtime error. The caller's recovery is to retry or fix the environment/config, not the command line.
- **`2`** — usage error. The invocation itself was wrong: an unknown argument or tool, an unknown shell, a bad flag value, or incompatible format flags. The caller's recovery is to fix the arguments. Error text (and, for the data commands, a short usage hint) is written to stderr.

Per-subcommand exit codes:

| Command | `0` | `1` | `2` |
|---------|-----|-----|-----|
| `tu [source] [period] [display]` (data commands, incl. `--watch`) | success, incl. empty results and warn-and-ignore guards | unexpected runtime error; `$HOME` unset; `lb`/`lbh` in single mode (`Error: lb requires multi mode — run tu init-metrics <repo-url> to set up a metrics repo`) (DC-14) | unknown argument/tool/second positional, `--help`/`-h` after a positional, bad flag value (incl. bad `--top`, `--metric`, `--interval`), incompatible format flags (`--json`/`-j`/`--csv`/`--md`/`--watch`), `-t` with `--metric cost`, bad/inverted `--since`/`--until`, missing `-u` value, config `user = all` (reserved), `--dry-run` without `tu sync` |
| `tu sync` | success (incl. `--dry-run`) | `metrics_repo` unset, clone/dir-missing fallback, commit/pull/push failure | config `user = all` |
| `tu init-metrics [repo-url]` | success, `Already initialized` | `metrics_repo` unset, metrics dir exists but is not a git repo, clone failure, `$HOME` unset | more than one positional argument |
| `tu update` | success (incl. non-Homebrew install message, "already up to date", `--help`) | `brew update`/`brew info`/`brew upgrade` failure | — |
| `tu shell-init [shell]` | success (script emitted) | — | missing or unknown shell |
| `tu init-conf`, `tu status` | success | unexpected runtime error, `$HOME` unset | — |
| `tu help`, `tu --version`, `tu help-dump`, `tu skill` | success | unexpected runtime error | — |

Diagnostics on any error path go to stderr; stdout carries data only (principle №2). Warnings (`Warning: …`, `warning: …`, `syncing metrics...`, deprecation and clone notices) also go to stderr so `--json`/`--csv`/`--md` stdout stays clean.

## Data Model

All data flows through two core types:

```go
type Totals struct {
	TotalCost           float64 `json:"totalCost"`
	InputTokens         int64   `json:"inputTokens"`
	OutputTokens        int64   `json:"outputTokens"`
	CacheCreationTokens int64   `json:"cacheCreationTokens"`
	CacheReadTokens     int64   `json:"cacheReadTokens"`
	TotalTokens         int64   `json:"totalTokens"`
}

type Record struct {
	Date    string // ISO label: "YYYY-MM-DD" or "YYYY-MM"
	Tool    string // registry key: cc, codex, oc, gemini, copilot, kimi
	User    string
	Machine string
	Totals
}
```

`totalTokens` = input + output + cache creation + cache read. Costs are floating-point dollars carried unrounded through aggregation; rounding happens only at render time (two decimals in tables/CSV/Markdown, raw in JSON).

Tool configs define the six supported tools (`cc`, `codex`, `oc`, `gemini`, `copilot`, `kimi`), each with a display name (`Claude Code`, `Codex`, `OpenCode`, `Gemini`, `Copilot`, `Kimi`), the per-agent `ccusage` subcommand, and the JSON key carrying the ISO date label (`date` for every tool at ccusage v20).

## Data Flow

### Fetching

1. Each tool is invoked as `ccusage {subcommand} daily --json` (the single vendored `ccusage` binary, exec'd directly with a 10 MB output buffer, no shell); only the daily subcommand is ever called
2. Output is parsed as JSON; the `daily` array is extracted as `UsageEntry[]`
3. Labels are normalized to ISO format (`2026-02-14`); a human-readable label (`Feb 14, 2026`) is converted defensively
4. A tool whose fetch fails warns `warning: {toolName} fetch failed (...), showing zero data` on stderr and contributes zero totals; the command still exits 0
5. Weekly and monthly data are computed client-side by aggregating daily entries after any merge and window filter. Monthly slices the label to `YYYY-MM`; weekly keys each day under its week's **Sunday** as an ISO date, computed with UTC arithmetic on the date-only label (immune to DST) and aligned with `ccusage weekly`'s default `--start-of-week sunday`. Both sum the numeric fields. Weekly/monthly labels are display-only; the metrics repo always stores daily entries

### Caching

- Fetched daily entries are cached per tool at `~/.tu/cache/{tool}-daily.json` (`cc-daily.json`, `codex-daily.json`, …); a tool with no data still gets a cache file
- Cache TTL: 60 seconds (checked via file mtime)
- Cache is bypassed when `--fresh`/`-f` is set; the fetch cache is the only cache — every filter, aggregation, and merge happens after it
- Non-fatal: write failures are silently ignored

### Snapshot vs History

- **Snapshot**: fetches all entries, then filters to the one matching the current label (today's date, the current week's Sunday, or the current month), resolved in **local time**. Shows a cross-tool table with one row per tool that has data; tools without data are omitted from tables and CSV but present in JSON.
- **History**: fetches all entries, shows a table with one row per date/week/month. Daily and weekly history default to the last 3 calendar months (an implicit `--since` floor at the first day of the month two months back, e.g. `2026-07-01` on 2026-09-16; disabled by `--full` or any explicit `--since`/`--until`); monthly history is never capped. When the cap is active the table and Markdown headings carry a `last 3 months` hint; CSV and JSON carry no heading but the same data window. Single-tool history shows the token breakdown; all-tools history shows a cost pivot table (date rows × tool columns). The window filter applies to daily entries before weekly/monthly roll-up, so a partial month sums only in-window days, and a weekly window may begin with a partial week labeled by its Sunday.
- **Leaderboard** (`lb`, multi mode only): reads every user's entries from the metrics repo (repo-only, so today lags until `--sync`), windows them to the current period — or to an explicit `--since`/`--until` range, which replaces the period window — sums across the source's tools, and ranks users descending by the display metric (ties by user name ascending). The Δ column compares against the immediately preceding same-length window (previous day / Sunday-anchored week / calendar month, or the equal-length range ending the day before `--since`; an `--until`-only window has no previous window and every row is `new`, DC-13), derived by a second client-side filter pass over the same fetched entries — no second fetch. Rows with zero cost and zero tokens in the window are omitted. Δ is `(current − previous) / previous`, `new` when the previous value is absent or exactly zero.
- **Leaderboard history** (`lbh`, multi mode only): the same repo read shaped as a pivot — period rows × user columns through the same renderer as the all-tools pivot, with columns ordered by descending window total, each row's leading cell highlighted, and the negligible-column omission disabled (no user is silently hidden from a ranking). `--by-machine` warns and is ignored, exactly as on the all-tools pivot.

### Empty results

An empty snapshot prints the heading and `  No usage`; an empty history prints the heading and `  No data`; an empty leaderboard prints the heading, `No data`, and the staleness footer. JSON prints the shape with zero totals or empty arrays; CSV prints the header line only; Markdown prints the heading and the two header lines. All exit 0.

## Output Formats

Four output formats are selected by mutually exclusive flags: the ANSI table (default), `--json`/`-j`, `--csv`, `--md`. One data fetch happens per command whatever the format (`--sync` adds the sync's own fresh fetch before it). The three machine formats carry no bars, arrows, ANSI, separators, footer, legend, or `📊`; they honor `--since`/`--until`, `--full`, `--top`, `-u`, and `--by-machine` on the displays that support them (the same warn-and-ignore guards apply). `--metric`/`-t` leaves their cells cost-denominated (machine columns, history and snapshot values) but still drives the leaderboards: `lb` ranking, `share`, and `delta` are computed in the display metric, `lbh --top` selects columns by it, and the `lbh` Markdown heading reads `Leaderboard Token History`.

### Terminal width and color

- The table width budget is the TTY width of stdout; when stdout is not a TTY (a pipe or file) the budget is **80 columns**, and the `COLUMNS` variable is never read (DC-12).
- One-shot tables never narrow themselves: a table wider than the budget simply wraps in the terminal. Compact layouts exist only in watch mode (DC-12).
- Inline bars render only when at least 10 columns remain after the last column plus a 3-column gutter, and never exceed 30 columns.
- ANSI color is emitted whether or not stdout is a TTY; `--no-color` and a non-empty `NO_COLOR` disable it identically (color-only styling — dim zero cells, the current-period marker, the leader highlight, weekend dimming — disappears without changing widths).
- Every one-shot table is preceded and followed by one blank line; the heading line starts with `📊 ` except the leaderboard's (DC-07).

### Snapshot Table (all tools)

Columns: Tool, Tokens, Input, Output, Cache, Cost (Cache = cache write + cache read combined, so Input + Output + Cache = Tokens). Fixed widths — Tool 12, each numeric column 12 — for an 87-char row; a value wider than 12 chars overflows its cell and misaligns that row (DC-24). One row per tool with non-zero tokens (registry order), plus a divider and a Total row only when more than one tool has data. Heading: `📊 Combined Usage (daily|weekly|monthly)` — also for a single-source snapshot such as `tu cc` (DC-15). The table is unchanged under `--metric tokens`/`-t` (its columns are already token-denominated; the Cost column stays) — only the watch delta indicator moves to the Tokens cell.

### Single-Tool History Table

Columns: Date (12), Input, Output, Cache Write, Cache Read, Total (14 each), Cost (data-sized, floor 9). Rows ascending by label. Includes inline bar charts (Unicode block elements at eighths precision, scaled to the max cost in the visible window) when the width allows — the 110-char row needs a ≥121-column terminal. Divider + Total row + summary footer when >1 entry. Heading: `📊 {Tool Name} (daily|weekly|monthly[, last 3 months])`. Under `--metric tokens`/`-t` the last column renders as `Tokens` = total tokens and the bars/footer scale on token volume.

### All-Tools History Pivot Table

Columns: Date (10), one per **visible** tool (data-sized: `max(name length, 9, longest cell incl. Total)`), Cost (data-sized, floor 9). Each cell is a cost value. Inline stacked bar charts for row totals. Total row with per-tool sums. Heading: `📊 Combined Cost History (daily|weekly|monthly[, last 3 months])`. Under `--metric tokens`/`-t` every cell and the Total row render as total tokens, the last header reads `Tokens`, and the heading is `📊 Combined Token History`.

**Negligible-column omission** (ANSI pivot only): a tool column is kept iff its total over the visible rows is ≥ $1.00 (≥ 1,000 tokens under `-t`) **and** ≥ 0.1% of the visible grand total; boundary values kept. If nothing survives, fall back to the exact-zero filter, then to every registry tool. Omitted tools still count in the row Cost, the Total row, the bars, and the footer. Markdown applies only the exact-zero rule; CSV omits nothing (DC-06).

### Table semantics shared by the history tables

- **Month separators**: in daily views a dim divider line precedes each row whose `YYYY-MM` differs from the previous row's; none before the first visible row; never in weekly/monthly, compact, CSV, or Markdown output.
- **Current-period marker**: the row whose label equals the current period's local-time label renders its date cell in bold white (color only).
- **Weekend dimming**: in daily views a Saturday/Sunday date cell renders dim (date cell only; the weekday is taken from the UTC-parsed ISO label). The current-period marker wins on a weekend today.
- **Summary footer**: one dim line after the Total row when ≥2 data rows: `avg {value}/{day|week|month} · [this month {value} · ]peak {value} ({label})` — `avg` = visible total ÷ visible row count; `this month` = sum of the current calendar month's rows, daily only, omitted when absent; values in the display metric. When the two-zone scale is active the footer appends ` · ┊ = {p95} (p95)`; when the pivot renders stacked bars with ≥2 visible tools and color on, it appends a legend ` · █ {Tool} █ {Tool}…` (swatches in palette colors, names dim). The footer is a single line and wraps on narrow terminals (DC-12).
- **p95 two-zone bar scale**: when `max > 1.5 × p95` (95th percentile by linear interpolation over the nonzero visible row values), bars split into a main zone (linear 0→p95), a dim `┊` (U+250A) rule at the same column in every row, and a yellow overflow zone (linear p95→max, `max(4, round(barWidth / 4))` chars) used only by rows above p95; rows at exactly p95 end at the rule. Otherwise a single linear scale.
- **Stacked pivot bars**: the pivot's main zone is split into contiguous per-tool segments in column order, apportioned by largest-remainder rounding over the bar's visible characters (ties to the earlier column), the fractional final character belonging to the rightmost segment; colors `green, magenta, blue, cyan` by visible column position (a 5th+ tool uncolored); the overflow zone stays solid yellow. Stripping ANSI yields exactly the unstacked bar.
- **Exact-zero dimming**: a metric data cell whose value is exactly 0 renders dim (Total row, headers, dividers never; a sub-cent nonzero value formatting as `$0.00` is not dimmed). Applies to pivot cells, the pivot row-total cell, the single-tool history's last column, and every machine column.
- **Data-sized columns**: right-aligned metric columns (pivot tool and row-total columns, the single-tool history's last column, all machine columns) are sized to the longest formatted value they will hold including the Total row, with a floor of 9; all machine columns share one width. The snapshot's columns are fixed at 12.
- **Row budget in watch mode**: history tables show only the most recent rows that fit the terminal height; separators, the p95 scale, and the footer are computed on that visible window.
- **Number formatting**: costs `$1,234.56` (en-US thousands separators, two decimals); token counts `1,234,567` (rounded integers).

### Leaderboard Table (`lb`)

Columns: `#`, User, Cost, bar, Tokens, Share, Δ vs {previous window label}. One row per user (or `user/machine` pair under `--by-machine`), ranked descending by the display metric; the pinned user (`-u <name>`, else the config user) carries a ` ◂` marker on each of its rows. Both the Cost and Tokens columns render in every metric mode — `--metric` selects only the sort key, bar scale, share denominator and the heading's `by …` suffix. Share is a percentage with one decimal (`69.0%`, `100.0%`); Δ is a signed whole percentage (`-55%`, `+4757%`) or `new`. The rank column is 1 char wide up to 9 rows and 2 from 10. A bolded Total row (rank, Share, Δ blank) follows when the full ranked set has ≥2 users, also under `--top`; a dim staleness footer (`synced {relative} ago ({ISO}) · tu sync to refresh`, or `never synced · tu sync to refresh`) closes the table. Heading: `Leaderboard (daily|weekly|monthly) · {window} · by {cost|tokens}` with no `📊` (DC-07); `{window}` is the period's current label, `{since} → {until}` under an explicit window, or `→ {until}` for an `--until`-only window (DC-13). Under `--top <n>` the rows past N collapse into one dim `… +k others` line (still counted in the Total and every share denominator).

### Leaderboard History Table (`lbh`)

Same shape as the all-tools pivot with users in place of tools: period rows × user columns, ordered by descending window total in the display metric (ties keep first-seen order), each row's leading user cell highlighted. Heading: `📊 Leaderboard History (daily|weekly|monthly[, last 3 months])` (`Leaderboard Token History` under tokens). No negligible-column omission. `--top <n>` keeps the N highest-total user columns (in the display metric) and folds the rest into one `others` column so row totals are preserved — no `others` column when nothing was folded; `others` is sorted with the user columns by its own total (DC-08). Month separators, current-period marker, weekend dimming, stacked bars + legend, p95 scale, footer, and zero dimming are inherited from the pivot.

### JSON Output (`--json`)

Pretty-printed with two-space indentation and a trailing newline; keys in the order listed; numbers are the raw double-precision values as summed (`4.936068800000001`), never rounded (DC-10); tool keys are display names in registry order, user keys alphabetical.

| Display | Shape |
|---------|-------|
| Snapshot (`tu --json`) | object `{ "{Tool}": totals }` with **every registry tool** present (or only the selected tool for a single-source command). A tool with data: `label, totalCost, inputTokens, outputTokens, cacheCreationTokens, cacheReadTokens, totalTokens`. A tool with no data: the six totals only, all `0`, **no `label`** (DC-01) |
| Snapshot `--by-machine` | as above; a tool with data gains a trailing `machines` object `{ "{machine}": cost }` (`{user}` keys under `-u all`); zero-usage tools gain nothing (DC-01) |
| Single-tool history (`tu cc h --json`) | bare array of entries `{ label, totalCost, …, totalTokens }` ascending by label; `--by-machine` adds `machines` to each entry |
| All-tools history (`tu h --json`, `tu mh --json`) | object `{ "{Tool}": [entries] }` with every registry tool present, an empty array for a tool with no data |
| Leaderboard (`tu m lb --json`) | array of `{ rank, user, [machine,] cost, totalTokens, share, delta }` — `machine` only under `--by-machine`; `share` a fraction; `delta` a fraction or `null` for a `new` row; `--top` truncates the array |
| Leaderboard history (`tu m lbh --json`) | object `{ "{user}": [entries] }` (alphabetical user keys); `--top` keeps N users plus an `others` key when at least one user was folded |

`machines` values are costs even under `-t`. Incompatible with `--watch`, `--csv`, `--md` (exit 2).

### CSV Output (`--csv`)

RFC 4180: header row first, comma separator, LF line endings, no BOM, no quoting needed for the shipped names (a field containing `,`, `"`, or a newline would be quoted with `"` and internal `"` doubled). Numbers raw (no thousands separators); costs two decimals without `$`; token counts integers; dates ISO. A `Total,…` row follows only for the snapshot (more than one tool with data) and the leaderboard (more than one ranked user in the full set, also under `--top`); the two history kinds never carry one. The header alone is printed for an empty window.

| Kind | Header | Notes |
|------|--------|-------|
| Snapshot | `tool,tokens,input,output,cache,cost` | one row per tool **with data** (zero-usage tools omitted) (DC-05); machine columns `machine_{name}_cost` appended after `cost`, alphabetical (user names under `-u all`) |
| Single-tool history | `date,input,output,cache_write,cache_read,total,cost` | plus `machine_{name}_cost` columns under `--by-machine` |
| All-tools history | `date,{Tool1},…,{Tool6},total` | **every registry tool** column, positional, `0.00` cells (DC-05) (DC-06) |
| Leaderboard | `rank,user,cost,total_tokens,share,delta` (`machine` after `user` under `--by-machine`) | `share` and `delta` are fractions rounded to 3 decimals with trailing zeros dropped (`0.69`, `-0.3`, `17.309`) (DC-11); `delta` empty for a `new` row; `Total,,{cost},{tokens},,` sums every user even under `--top` |
| Leaderboard history | `date,{user…},total` | user columns **alphabetical** (DC-16); an `others` column under `--top` only when a user was folded; last column `total`; no Total row |

### Markdown Output (`--md`)

A `## {title}` heading (the ANSI heading without `📊`, with the `, last 3 months` hint when active), a blank line, a GFM table (header, alignment row with `:---` for text and `---:` for numbers, data rows), and a trailing blank line. Numbers keep thousands separators; costs `$`-prefixed with two decimals; a `**Total**` row with bolded numbers when more than one data row is visible — for the leaderboards, when the full ranked set has more than one user, also under `--top`; no bars, arrows, footer, or legend.

| Kind | Title | Columns |
|------|-------|---------|
| Snapshot | `Combined Usage ({period})` | Tool, Tokens, Input, Output, Cache, Cost [, one column per machine/user named directly] |
| Single-tool history | `{Tool} ({period}[, last 3 months])` | Date, Input, Output, Cache Write, Cache Read, Total, Cost [, machine columns] |
| All-tools history | `Combined Cost History ({period}[, last 3 months])` | Date, one per tool with a **nonzero** total (exact-zero columns dropped — DC-06), Cost |
| Leaderboard | `Leaderboard ({period})` | #, User, [Machine,] Cost, Tokens, Share (`69.0%`), Δ vs {label} (`-55%`/`new`); the Total row carries `**Total**` in the `#` cell and blanks User, Share, Δ |
| Leaderboard history | `Leaderboard History ({period}[, last 3 months])` (`Leaderboard Token History` under `-t`) | Date, users **alphabetical** (DC-16) [, others when folded], Cost |

### Delta Indicators

In watch mode, cost cells show up/down arrows (green `↑` when the value increased vs the previous poll, red `↓` when it decreased, nothing on the first poll or no change) using per-item tracking keyed by `{toolName}:{label}` (history rows), `total:{label}` (pivot row totals), `{toolName}` (snapshot rows) or the user name (leaderboards). Every renderer uses the spaced form (`$26.86 ↑`) except the all-tools pivot, which abuts the arrow (`$1,013.30↑`). Under `-t` the arrow rides the Tokens cell and compares token values.

## Multi-Machine Mode

### Configuration (`~/.config/tu/tu.conf`)

INI-style `key = value` file (lines starting with `#` are comments, blank lines ignored). Fields:

| Field | Default | Description |
|-------|---------|-------------|
| `version` | 2 | Config schema version; a newer value warns `Warning: {absolute path} version {N} is newer than tu supports (2). Please update tu.` on stderr and continues (`status` still echoes `(vN)`) |
| `metrics_repo` | — | Git repo URL for metrics storage (required for multi) |
| `metrics_dir` | `~/.tu/metrics_repo` | Local clone path; a leading `~` expands to the home directory |
| `machine` | `$HOSTNAME` | Machine label |
| `user` | `$USER` | User/profile label; the value `all` is reserved — `Error: config user "all" is reserved (used by -u all)`, exit 2, on every data command and `tu sync` |
| `auto_sync` | `true` | Only `false` or `0` turn it off. Its only observable effect is the `Auto-sync: on|off` line of `tu status`; no data command syncs on its own whatever its value or the age of `.last-sync` (DC-20) |
| `mode` | — | Silently ignored if present (mode is derived from `metrics_repo`) |

The shipped `tu.default.conf` (also what `init-conf` copies) is:

```
# Default tu configuration — fallback for all properties.
# User overrides go in ~/.config/tu/tu.conf (created by 'tu init-conf'). Org-wide defaults may be dropped in ~/.config/tu/org.conf.
#
# Sentinel values:
#   $HOSTNAME  — resolved to os.hostname() at runtime
#   $USER      — resolved to os.userInfo().username at runtime

version = 2

# Git repo URL for metrics storage
# metrics_repo = git@github.com:you/tu-metrics.git

# Local path where the metrics repo is cloned
metrics_dir = ~/.tu/metrics_repo

# Label for this machine in the metrics repo
machine = $HOSTNAME

# Profile name — groups your machines in the metrics repo
user = $USER

# Auto-sync: use 'tu <cmd> --sync' to sync before fetch
auto_sync = true
```

The config path is built from `$HOME` only (`$HOME/.config/tu/tu.conf`) — no `XDG_CONFIG_HOME`, no other env var can move it; an unset `$HOME` is an actionable error on config-reading commands. Values layer in exactly this order (later wins, no per-key exceptions):

```
tu.default.conf  <  ~/.config/tu/org.conf  <  ~/.config/tu/tu.conf  <  TU_METRICS_REPO  <  CLI argument
 (shipped)          (optional org layer)      (personal overrides)     (metrics_repo      (e.g. the
                                                                       only)               init-metrics URL)
```

- `~/.config/tu/org.conf` is an optional org-wide layer (same format): an org's dotfiles/MDM/bootstrap drops it in and every machine runs in multi mode with zero per-user edits; absence is silent. `tu status` shows an `Org config:` line when it exists.
- A legacy `~/.tu.conf` is read only when `~/.config/tu/tu.conf` does not exist, with one stderr line per process: `tu: ~/.tu.conf is deprecated; move it to ~/.config/tu/tu.conf`. It is never moved or deleted by tu; creating the new file via `tu init-conf` / `tu init-metrics <url>` seeds it from the legacy contents.
- Mode is derived: a non-empty `metrics_repo` (from any layer, including a non-empty `TU_METRICS_REPO`) means multi mode; otherwise single mode. An empty `TU_METRICS_REPO` is treated as unset.
- Sentinel values `$HOSTNAME` and `$USER` are expanded at runtime.

### Metrics Repo Layout

```
{user}/{year}/{machine}/{tool}-{date}.jsonl
```

Each file contains one JSON line with a `UsageEntry` (`{"label":"2026-09-10","totalCost":0.449,"inputTokens":…,"totalTokens":…}`) for one local day; `{tool}` is the source token (`cc`, `codex`, `oc`, `gemini`, `copilot`, `kimi`), `{date}` the entry's local ISO date, `{year}` its year. Top-level directories are user profiles, except `docs/` and dot-directories (`.git`), which are never read as users. Local entries for every tool with data are written to the clone before every multi-mode fetch of the own user (a plain `tu` in multi mode is a write).

**Never-shrink guard.** A day-file is overwritten only when the incoming entry's `totalCost` is greater than or equal to the existing file's; a strictly lower incoming cost is skipped silently (no stderr). A missing, empty, or unparseable existing file is written. Day-files are therefore high-water marks that survive the assistant's own transcript purge.

**Own-machine max-merge.** In multi mode the own user's view reads every machine's day-files from the clone, then for the own machine takes, per date, whichever whole entry — the live fetch or the stored day-file — has the greater `totalCost` (ties → live), and sums that with the other machines' entries. A stored day-file larger than today's live fetch therefore shows up in the snapshot, the `--by-machine` column, and the leaderboard. `-u <other user>` and `-u all` read repo entries only (plain per-label sums, no live data, no writes). Single mode reads no repo.

### Sync Flow (`tu sync` / `--sync`)

1. Fetch fresh local data for all tools
2. Write local entries to the metrics repo (never-shrink guarded)
3. `git add {user}/`, commit if anything changed with the message `# {user}: update {UTC date}` (the UTC date, which can trail the day-files' local date) (DC-21), `git pull --rebase origin main` (the branch name `main` is fixed) (DC-18), `git push` (retry once on failure); an interrupted rebase is aborted before the retry
4. Touch `~/.tu/.last-sync` (an ISO timestamp) on success

Output: `Synced to ~/.tu/metrics_repo` on stdout, exit 0. On failure, stderr gets `Error: sync failed — check network and remote config.` (exit 1), preceded by `Warning: sync pull failed — {git command}... failed: {git stderr}` only when the pull step failed; a failed commit (e.g. no git identity) or push prints the generic line alone (DC-18). `--sync` on a data command prints `syncing metrics... ` to stderr, then the table; a failure prints `sync failed — using local data.` and continues with exit 0. The repo write happens on every multi-mode data command; `sync`/`--sync` add the git round trip.

#### Dry Run (`tu sync --dry-run`)

`tu sync --dry-run` previews the sync without touching the working tree, the metrics repo, or the network, then prints the preview to stdout and exits 0. The mode and metrics-dir guards run first exactly as for a live sync, so a missing metrics dir still triggers the auto-clone (a network operation that creates the clone) before the preview. It shares the real write-decision path (the never-shrink guard runs identically), so the preview cannot drift from a live sync. Format (layouts §20):

```
Would write {N} day-file(s) under ~/.tu/metrics_repo/{user}/:
  {year}/{machine}/{tool}-{date}.jsonl  ${cost}  (new)
  {year}/{machine}/{tool}-{date}.jsonl  ${cost}  (update: ${existing} → ${cost})
Would skip {K} file(s) (never-shrink guard):
  {year}/{machine}/{tool}-{date}.jsonl  incoming ${cost} < existing ${existing}
Would commit: "# {user}: update {UTC date}", then pull --rebase origin main, then push
Dry run — nothing written, committed, or pushed.
```

The `Would skip` block appears only when something would be skipped; costs in the skip lines omit thousands separators (DC-22); an equal-cost rewrite is reported as an update and counts toward `Would commit`, so the preview may predict a commit that a live sync then finds unnecessary (DC-23). The git half is computed locally — only a read-only `git status --porcelain {user}/` is invoked; `pull`/`push` are reported, never executed. The same mode and `metrics_repo` guards as a live sync apply first (single mode is exit 1). The flag is honored **only** by `tu sync`; any other invocation carrying `--dry-run` fails fast with exit 2 (Global Flags).

### Auto-Clone Guard

When multi mode is configured but the metrics dir doesn't exist:
1. If no `metrics_repo` set: warn on stderr, fall back to single mode
2. If a recent clone failure marker exists (`~/.tu/.clone-failed`, an ISO timestamp less than 3 hours old): `Warning: metrics repo not available — falling back to single mode.`
3. Otherwise: attempt `git clone {repo} {dir}` with a 30 s timeout and `GIT_TERMINAL_PROMPT=0`; on success print `Cloned metrics repo → {absolute dir}` to stderr (DC-19) and continue in multi mode; on failure print `Warning: could not clone metrics repo ({git error}) — falling back to single mode.`, write the marker, and continue in single mode
4. A successful clone or `init-metrics` clears the clone failure marker
5. `tu sync` under the fallback prints the same warning and exits 1; `tu status` reports `Metrics: … (NOT FOUND — run 'tu init-metrics')` and never clones

### Staleness

`~/.tu/.last-sync` is written by a successful `tu sync`/`--sync` only. `tu status` renders it as `Last sync: {relative} ago ({ISO})` or `never`; the leaderboard footer renders `synced {relative} ago ({ISO}) · tu sync to refresh` or `never synced · tu sync to refresh`. Relative times read `<1m ago`, `15m ago`, `4h ago`, …. No command acts on the 3-hour staleness threshold other than the clone-failure cooldown (DC-20).

## Watch Mode (`--watch`)

Full-screen TUI using the alternate screen buffer (`\x1b[?1049h`, cursor hidden with `\x1b[?25l`) with compositor-based rendering. Frames in layouts §7–§11.

### Architecture

- **Compositor**: composites independent panels (stats, table, status) and re-renders on events — poll, resize, countdown tick — with no periodic tick
- **StatsPanel**: 2×3 stats grid rendered above the table
- **TablePanel**: data table output from the same render functions as non-watch mode, truncated to the rows that fit
- **StatusPanel**: footer with countdown timer and controls
- **RainLayer**: matrix rain animation (107 ms tick, cursor-positioned overlay, redrawn after every flush so polls do not blink it)

### Layout

- Full mode (>= 60 cols): stats grid + dim separator (as wide as the grid) + full table + rain
- Compact mode (< 60 cols): heading + compact table (name/date 14 wide, value 12 wide) only, no stats grid, no rain, no bars
- Rain fills available space below content, or the right margin (after a 2-column gutter, needs ≥10 columns) if no vertical space remains; drop count ≈ 30% of columns at ≤20 rows, scaling up to 3× at 60+ rows; `--no-rain` disables it
- Loading skeleton renders on alt-screen entry before the first fetch (stats grid with zeros/dashes, separator, table header, centered dim `Loading...`), with rain already animating
- Tables wider than the terminal wrap inside the frame (DC-17)

### Interaction

- `q` or Ctrl+C: exit (restores the normal screen and cursor, prints the last rendered table — heading through Total — to stdout, exit 0)
- Enter/Space: immediate refresh (cancels the countdown)
- Polls on the interval (default 10 s), shows `Refreshing...` in the footer during a fetch
- Terminal resize re-lays-out immediately

### Session Stats (grid above table)

- **Elapsed**: wall-clock time since the first poll (`Xs`, `Xm Xs`, `Xh Xm Xs`)
- **Session cost delta**: current poll cost − first poll cost, signed (`+$0.07`); `$0.00` before 2 polls
- **Tok/min**: `~{totalTokens / elapsedMinutes}`; `--` before 2 polls
- **Rate**: rolling window of the last 5 polls, `~${(latest.cost − oldest.cost) / hours}/hr` in yellow; `--` before 2 polls
- **Proj. day**: `~${todayCost + rate × hoursRemainingInDay}`; `--` before 2 polls
- Grid stays fixed at 3 rows; unavailable stats show `--`

### Matrix Rain

Half-width katakana + digits + latin characters falling at variable speeds (0.3–1.0 rows/tick). Column density ~30% (height-scaled). Trail length 3–8 characters with a brightness gradient (bright head, green body, dim tail). ~5% shimmer rate for random character replacement. Respawns after falling off screen with a random delay. Tick interval: 107 ms.

## Drop at cutover

Every entry below is a **proposal**: a behavior the shipped binary exhibits that looks accidental by at least one of the criteria in `fab/changes/260915-2y3l-spec-reconciliation/intake.md` §4 (numbered as in that intake: 1 inconsistency with a sibling behavior, 2 undocumented in memory, 3 hedged by memory, 4 an implementation detail leaking into a surface, 5 a toolkit-standard or constitution tension, 6 a cross-format asymmetry). The bracket is left unfilled; gate G0 resolves each one. `keep` means the Go port reproduces the behavior byte-for-byte; `drop` means it is an expected diff in the differential harness (R3) and the spec line carrying the same `(DC-NN)` is rewritten at cutover. Nothing here is removed from the spec now. IDs are stable.

- **DC-01** `[DECIDE: keep|drop]` Snapshot `--json` objects for zero-usage tools omit the `label` key (and, under `--by-machine`, the `machines` key) while tools with data carry them.
  Where: `tu --json`, `tu --by-machine --json` (any mode) — `"Codex": {"totalCost": 0, …}` vs `"Claude Code": {"label": "2026-09-16", …, "machines": {…}}`.
  Why it looks accidental: the key set depends on data presence, not on the display; consumers must special-case it; no memory requirement states it (criteria 4, 2).
  Spec: Output Formats › JSON Output; layouts §12.

- **DC-02** `[DECIDE: keep|drop]` Non-data commands silently accept the data flags: `tu sync --json` performs a real sync and prints the plain-text result; `tu status --json`, `tu init-conf --json`, `tu help --json`, `tu status --fresh`, `tu status --watch` all ignore the flag without a warning.
  Where: `tu sync --json` (multi mode), `tu status --json`.
  Why it looks accidental: data commands reject format-flag conflicts with exit 2 and warn on off-target flags; setup commands do neither, and `--json` on `sync` produces a side effect the caller did not expect to be plain-text (criteria 1).
  Spec: Setup Commands.

- **DC-03** `[DECIDE: keep|drop]` Off-target data flags follow three different policies: warn-and-continue (`--top`, `--full`, `--since`/`--until` on a snapshot, `-u` in single mode, `--by-machine` on the pivots), silent acceptance (`--interval`/`--no-rain` without `--watch`, `-t`/`--metric` with `--json`/`--csv`/`--md`), and fail-fast exit 2 (`--dry-run` off `tu sync`).
  Where: `tu --top 3` (exit 0 + warning), `tu --interval 30` (exit 0, silent), `tu cc --dry-run` (exit 2).
  Why it looks accidental: the same class of mistake is handled three ways; memory documents each site individually but names no policy (criteria 1).
  Spec: Global Flags.

- **DC-04** `[DECIDE: keep|drop]` `--help`/`-h` is recognized only as the first argument or after `update`; `tu cc --help` and `tu h -h` are `Unknown argument`, exit 2.
  Where: `tu cc --help`.
  Why it looks accidental: toolkit principle №3 (self-describing) and the `update --help` special case both suggest help should work anywhere; memory records the behavior as "unchanged", not as a decision (criteria 5, 1).
  Spec: Global Flags; layouts §14.

- **DC-05** `[DECIDE: keep|drop]` CSV snapshot output omits zero-usage tool rows, while CSV all-tools history keeps every registry column as a positional contract.
  Where: `tu --csv` (rows for tools with data only) vs `tu h --csv` (`date,Claude Code,Codex,OpenCode,Gemini,Copilot,Kimi,total`).
  Why it looks accidental: the "positional machine contract" rationale that keeps all pivot columns applies equally to snapshot rows; the two CSV kinds disagree (criteria 6).
  Spec: Output Formats › CSV Output; layouts §15.

- **DC-06** `[DECIDE: keep|drop]` The all-tools history applies three different column-omission rules by format: significance threshold ($1.00 / 0.1%) in the ANSI table, exact-zero in Markdown, none in CSV.
  Where: `tu h`, `tu h --md`, `tu h --csv` on the same window (Gemini at `$0.04` total is omitted in the table, kept in Markdown, kept in CSV).
  Why it looks accidental: a deliberate decision per memory, listed because it is a three-way cross-format asymmetry the port must reproduce exactly or consciously unify (criteria 6).
  Spec: Output Formats › All-Tools History Pivot Table, Markdown Output, CSV Output; layouts §4, §16.

- **DC-07** `[DECIDE: keep|drop]` The `lb` heading is the only table heading without the `📊 ` prefix (`lbh`, snapshots, and histories all carry it).
  Where: `tu m lb` → `Leaderboard (monthly) · 2026-09 · by cost`.
  Why it looks accidental: every sibling heading, including the leaderboard history, uses the prefix; no memory decision mentions omitting it (criteria 1).
  Spec: Output Formats › Leaderboard Table; layouts §5.

- **DC-08** `[DECIDE: keep|drop]` Under `lbh --top n`, the folded `others` column is sorted with the user columns by its own total, so it can render first or in the middle instead of last.
  Where: `tu m lbh --top 2` → `Date | others | sahil | eunice | Cost`.
  Why it looks accidental: memory says `--top` "folds the rest into one `others` column" with no placement rule; a fold column that outranks real users reads as a bug (criteria 2).
  Spec: Output Formats › Leaderboard History Table; layouts §6, §19.

- **DC-09** `[DECIDE: keep|drop]` `--version`/`-V`/`-v` are absent from the `--help` text, and the lowercase `-v` alias exists only in memory and the completion scripts.
  Where: `tu --help` (no version line); `tu -v` → `tu version v0.11.5`.
  Why it looks accidental: help is the discoverability surface every other flag uses; the `version` standard requires only `--version`, so `-v` is an undocumented extra (criteria 2, 5).
  Spec: Global Flags; layouts §14.

- **DC-10** `[DECIDE: keep|drop]` JSON numbers are the raw double-precision sums (`4.936068800000001`, `5.7243808000000005`), so the exact digits depend on summation order.
  Where: `tu --json`, `tu h --json`, `tu m lb --json` (`cost`, `share`, `delta`).
  Why it looks accidental: floating-point artifacts leaking into a machine contract; a port that sums in a different order produces different bytes for identical data, which is exactly what the harness will flag (criteria 4).
  Spec: Output Formats › JSON Output; layouts §12.

- **DC-11** `[DECIDE: keep|drop]` Leaderboard CSV renders `share` and `delta` with up to 3 decimals and trailing zeros dropped (`0.69`, `-0.3`, `17.309`) while `cost` is fixed at 2 decimals.
  Where: `tu m lb --csv`.
  Why it looks accidental: JavaScript number-to-string formatting leaking into a machine format; columns in one row use two different precision rules (criteria 4, 6).
  Spec: Output Formats › CSV Output; layouts §15.

- **DC-12** `[DECIDE: keep|drop]` One-shot output ignores terminal width beyond the bar budget: there is no compact layout outside watch mode (a 50-column TTY gets the full 87-char snapshot, wrapped), the footer/legend line is never wrapped or shortened, and when stdout is not a TTY the width is assumed to be 80 with the `COLUMNS` variable ignored.
  Where: `tu` in a 50-column terminal; `tu h | cat` (bars sized for 80); `COLUMNS=120 tu h | cat` (unchanged).
  Why it looks accidental: memory states "Compact mode MUST activate when terminal width < 60" without the watch-only qualifier; the pipe default and the `COLUMNS` behavior are undocumented (criteria 2, 3).
  Spec: Output Formats › Terminal width and color; layouts §7, §21.

- **DC-13** `[DECIDE: keep|drop]` An `--until`-only leaderboard window renders the heading as `· → 2026-09-10 ·` with an empty left side and marks every Δ `new`.
  Where: `tu lb --until 2026-09-10`.
  Why it looks accidental: memory documents the `new` outcome but not the heading form; an open-ended range could name its start (criteria 2).
  Spec: Output Formats › Leaderboard Table; layouts §5.

- **DC-14** `[DECIDE: keep|drop]` The single-mode leaderboard guard message names `lb` even when the command was `lbh`.
  Where: `tu lbh` in single mode → `Error: lb requires multi mode — …`.
  Why it looks accidental: the message is a constant string; nothing in memory says it is intentional for both displays (criteria 2).
  Spec: Exit Codes; layouts §5.

- **DC-15** `[DECIDE: keep|drop]` A single-source snapshot keeps the heading `📊 Combined Usage ({period})` instead of naming the tool, while a single-source history is titled by the tool.
  Where: `tu cc` → `📊 Combined Usage (daily)`; `tu cc h` → `📊 Claude Code (daily, …)`.
  Why it looks accidental: the previous layouts.md §2 claimed the heading used the tool name — the spec author expected it; the snapshot and history disagree (criteria 1, 2).
  Spec: Output Formats › Snapshot Table; layouts §2.

- **DC-16** `[DECIDE: keep|drop]` `lbh` orders user columns by descending total in the table but alphabetically in CSV and Markdown.
  Where: `tu m lbh` vs `tu m lbh --csv` / `tu m lbh --md`.
  Why it looks accidental: the ranking is the point of a leaderboard display; the machine formats fall back to key order and lose it (criteria 6).
  Spec: Output Formats › CSV Output, Markdown Output; layouts §6, §15, §16.

- **DC-17** `[DECIDE: keep|drop]` Watch mode does not guard tables wider than the terminal: the single-tool history (110 chars) at 100 columns and `lbh` with many users wrap inside the frame and break the compositor's line accounting; only the all-tools pivot has a width contract (96/97).
  Where: `tu cc h -w` in a 100×30 terminal; `tu m lbh -w`.
  Why it looks accidental: memory's width contract covers the pivot only; the other tables were never fitted to watch mode (criteria 2).
  Spec: Watch Mode › Layout; layouts §7.

- **DC-18** `[DECIDE: keep|drop]` Sync failure reporting: the pull step is hard-coded to `origin main` (an empty repo or a `master`-default repo never syncs — `fatal: couldn't find remote ref main`), and a commit or push failure prints only `Error: sync failed — check network and remote config.` with the underlying git error swallowed (only pull failures include it).
  Where: `tu sync` against a freshly created empty remote; `tu sync` with no git identity configured.
  Why it looks accidental: the branch name is undocumented in memory; the generic message misdirects the user to network/remote config for a local commit failure (criteria 2, 4).
  Spec: Multi-Machine Mode › Sync Flow; layouts §20.

- **DC-19** `[DECIDE: keep|drop]` Messages mix `~`-abbreviated and absolute paths and channels: `tu status` prints `~/.tu/metrics_repo` while `Cloned … → /home/user/.tu/metrics_repo`, `Already initialized: /home/user/…`, `Error: /home/user/.tu/metrics_repo exists but is not a git repo`, and the config-version warning print absolute paths; auto-clone reports `Cloned metrics repo → …` on stderr while `init-metrics` reports `Cloned {url} → …` on stdout and lets git's own `Cloning into '…'` chatter through.
  Where: `tu init-metrics <url>` vs a first multi-mode `tu`.
  Why it looks accidental: the same event is worded and routed differently by entry point; memory records the strings but no rationale (criteria 1).
  Spec: Setup Commands; Multi-Machine Mode › Auto-Clone Guard; layouts §20.

- **DC-20** `[DECIDE: keep|drop]` The `auto_sync` config key and the 3-hour staleness threshold have no behavior: `auto_sync` only flips the `Auto-sync:` line of `tu status`, and no data command syncs automatically however old `.last-sync` is (the threshold is used only for the clone-failure cooldown and, indirectly, nowhere else).
  Where: multi mode with `.last-sync` deleted or 4 hours old, then `tu` / `tu h` — no sync, `.last-sync` unchanged; `auto_sync = false` then `tu sync` — still syncs.
  Why it looks accidental: a config surface and a documented staleness rule with no observable effect; the shipped defaults file comments it as "use 'tu <cmd> --sync' to sync before fetch", which describes a flag, not the key (criteria 2, 4).
  Spec: Multi-Machine Mode › Configuration, Staleness.

- **DC-21** `[DECIDE: keep|drop]` The sync commit message uses the UTC date (`# sbuser: update 2026-09-15`) while day-files and every displayed label use local dates (the same sync wrote `cc-2026-09-16.jsonl`).
  Where: `tu sync` after local midnight in a UTC-ahead zone; `tu sync --dry-run` (`Would commit: "# {user}: update {UTC date}"`).
  Why it looks accidental: the only UTC-dated surface in a tool that is otherwise local-day based (criteria 1, 4).
  Spec: Multi-Machine Mode › Sync Flow.

- **DC-22** `[DECIDE: keep|drop]` The dry-run `Would skip` lines print costs without thousands separators (`incoming $54.93 < existing $99999.00`) while the `Would write` lines and every table use them.
  Where: `tu sync --dry-run` with a day-file above the live value.
  Why it looks accidental: two formatting paths in one report (criteria 1, 4).
  Spec: Multi-Machine Mode › Dry Run; layouts §20.

- **DC-23** `[DECIDE: keep|drop]` The dry-run reports an equal-cost rewrite as `(update: $X → $X)` and counts it toward `Would commit`, so in steady state it predicts a commit a live sync does not make.
  Where: `tu sync --dry-run` immediately after `tu sync`.
  Why it looks accidental: memory itself labels the over-prediction a "sanctioned heuristic" (criteria 3).
  Spec: Multi-Machine Mode › Dry Run; layouts §20.

- **DC-24** `[DECIDE: keep|drop]` The snapshot's numeric columns are fixed at 12 characters and a wider value (`16,809,796,832`) overflows its cell, shifting that row while the header and dividers keep their width.
  Where: `tu m -u all` on a repo with 10-figure monthly token counts.
  Why it looks accidental: every other numeric column is data-sized precisely to avoid this; memory claims the 12-wide cell "still holds 999,999,999,999", which is 15 characters (criteria 1, 2).
  Spec: Output Formats › Snapshot Table; layouts §1.
