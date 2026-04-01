# tu

AI coding assistant cost tracking CLI.
Track your token usage in style!

<img width="1025" height="675" alt="image" src="https://github.com/user-attachments/assets/d6d1c930-8230-4910-ba1b-985e7df17e7c" />


## Install

```bash
brew tap sahil87/tap
brew install tu
```

## Update

```bash
tu update
# brew update
# brew upgrade tu
```

## Usage

```bash
tu                   # Today's cost, all tools
tu cc                # Today's cost, Claude Code
tu h                 # Daily cost history, all tools
tu cc mh             # Monthly cost history, Claude Code
tu m                 # This month's cost, all tools
```

Sources: `cc` (Claude Code), `codex`/`co` (Codex), `oc` (OpenCode), `all` (default)

### Flags

```
  --json               Output data as JSON (data commands only)
  --sync               Sync metrics before fetching (multi mode)
  --fresh / -f         Bypass cache, fetch fresh data (data commands only)
  --watch / -w         Persistent polling mode with live display (data commands only)
  --interval / -i <s>  Poll interval in seconds (default: 10, range: 5-3600)
  --user / -u <user>   Show usage for a specific user (multi mode only)
  --by-machine         Show per-machine cost breakdown (data commands only)
  --no-color           Disable ANSI color output
  --no-rain            Disable matrix rain animation in watch mode
```

### Setup (multi-machine sync)

```bash
tu init-conf         # Scaffold ~/.tu.conf
tu init-metrics      # Clone metrics repo
tu sync              # Push/pull metrics
tu status            # Show config and sync state
```
