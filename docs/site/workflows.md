# Workflows

Task-oriented recipes for `tu`, derived from `tu --help`. Every command and flag below matches what the CLI ships. New to `tu`? Start with the [install guide](install.md) for setup, including multi-machine sync.

## Command grammar

`tu` reads a positional command line of the form:

```
tu [source] [period] [display]
```

- **Source** — which tool's usage to report:
  - `cc` — Claude Code
  - `codex` / `co` — Codex
  - `oc` — OpenCode
  - `all` — every source (default)
- **Period** — the time window:
  - `d` / `daily` — daily (default)
  - `m` / `monthly` — monthly
- **Display** — how to present it:
  - *(bare)* — snapshot
  - `h` / `history` — history
- **Combined** — period and display fused into one token:
  - `dh` — daily history
  - `mh` — monthly history

Any token may be omitted; omitted tokens fall back to their defaults (`all`, daily, snapshot).

## Common recipes

```bash
tu                   # Today's cost, all tools (snapshot)
tu cc                # Today's cost, Claude Code
tu h                 # Daily cost history, all tools (pivot)
tu cc mh             # Monthly cost history, Claude Code
tu m                 # This month's cost, all tools
```

## Output formats

By default `tu` renders a formatted table. For scripting and piping, data commands accept structured output formats (data commands only):

- `--json` — output data as JSON
- `--csv` — output data as CSV
- `--md` — output data as Markdown

```bash
tu --json            # today's cost, all tools, as JSON
tu cc mh --csv       # Claude Code monthly history, as CSV
```

## Watch mode

Watch mode turns any data command into a live, continuously-refreshing display (data commands only):

- `--watch` / `-w` — persistent polling mode with live display
- `--interval` / `-i <s>` — poll interval in seconds (default: 10, range: 5-3600)
- `--no-rain` — disable the matrix rain animation in watch mode

```bash
tu -w                # live snapshot, refreshing every 10s
tu -w -i 30          # live, polling every 30 seconds
tu -w --no-rain      # live, without the matrix rain animation
```

## Multi-machine

When multi mode is configured (see the [install guide](install.md)), these flags control cross-machine aggregation:

- `--sync` — sync metrics before fetching (multi mode)
- `--user` / `-u <user>` — show usage for a specific user (multi mode only)
- `--by-machine` — show per-machine cost breakdown (data commands only)

```bash
tu --sync            # pull/push metrics, then show today's cost
tu -u alice          # show usage recorded under the 'alice' profile
tu --by-machine      # break today's cost down per machine
```

## Cache control

By default `tu` reads from cached data. To bypass the cache and fetch fresh data (data commands only):

- `--fresh` / `-f` — bypass cache, fetch fresh data

```bash
tu -f                # ignore cache, re-fetch today's cost
```
