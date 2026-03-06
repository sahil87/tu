# token-usage

AI coding assistant cost tracking CLI.

## Install

Requires SSH access to the wvrdz GitHub org:

```bash
brew tap wvrdz/tap git@github.com:wvrdz/homebrew-tap.git
brew install tu
```

## Update

```bash
brew update        # pull latest formula
brew upgrade tu
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
--json               Output as JSON
--fresh / -f         Bypass cache
--watch / -w         Live polling mode
--interval / -i <s>  Poll interval (default: 10s)
--no-color           Disable colors
--no-rain            Disable matrix rain in watch mode
--sync               Sync metrics before fetch (multi mode)
```

### Setup (multi-machine sync)

```bash
tu init-conf         # Scaffold ~/.tu.conf
tu init-metrics      # Clone metrics repo
tu sync              # Push/pull metrics
tu status            # Show config and sync state
```