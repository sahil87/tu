# tu

> Part of [@sahil87's open source toolkit](https://shll.ai) — see all projects there.

[![Latest release](https://img.shields.io/github/v/release/sahil87/tu)](https://github.com/sahil87/tu/releases) [![Downloads](https://img.shields.io/github/downloads/sahil87/tu/total)](https://github.com/sahil87/tu/releases) [![Stars](https://img.shields.io/github/stars/sahil87/tu?style=social)](https://github.com/sahil87/tu/stargazers)

AI coding assistant cost tracking CLI.
Track your token usage in style!

<img width="1025" height="675" alt="image" src="https://github.com/user-attachments/assets/d6d1c930-8230-4910-ba1b-985e7df17e7c" />


## Install

```bash
brew tap sahil87/tap
brew install tu
```

### Shell completions

```bash
# bash
echo 'eval "$(tu shell-init bash)"' >> ~/.bashrc

# zsh
echo 'eval "$(tu shell-init zsh)"' >> ~/.zshrc

# fish
tu shell-init fish > ~/.config/fish/completions/tu.fish
```

> 💡 Have other sahil87 tools? [`shll shell-install`](https://github.com/sahil87/shll#shll-shell-install--wire-the-rc-file-recommended) handles all of their shell integrations and autocompletions at once.

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

## CI / branch protection

`main` is gated by a required status check named **`ci-gate`**. The
[`CI` workflow](.github/workflows/ci.yml) runs the build and the test suite on
every pull request targeting `main` (and on pushes to `main`); the aggregating
`ci-gate` job passes only when `build-and-test` succeeds. A branch ruleset on
`main` requires `ci-gate` to be green before a PR can be merged.

Reproduce CI locally before opening a PR:

```bash
npm ci && npm run build && npm test
# or, with the task runner:
just test
```

Applying or adjusting the ruleset is an admin action (needs a `gh` token with
admin scope on the repo). The exact, idempotent command is captured in
[`scripts/ci-gate-ruleset.sh`](scripts/ci-gate-ruleset.sh):

```bash
scripts/ci-gate-ruleset.sh           # dry-run: preview the ruleset payload
scripts/ci-gate-ruleset.sh --apply   # create/update the ruleset (admin only)
```

The script degrades gracefully — if `gh` is missing, unauthenticated, or lacks
admin scope, it prints the manual steps instead of failing.
