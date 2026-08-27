# tu

> Part of the [shll toolkit](https://shll.ai) — see all projects there.

[![Latest release](https://img.shields.io/github/v/release/sahil87/tu)](https://github.com/sahil87/tu/releases) [![Downloads](https://img.shields.io/github/downloads/sahil87/tu/total)](https://github.com/sahil87/tu/releases) [![Stars](https://img.shields.io/github/stars/sahil87/tu?style=social)](https://github.com/sahil87/tu/stargazers)

AI coding assistant cost tracking CLI.
Track your token usage in style!

## Install

```sh
curl -fsSL https://shll.ai/install | sh -s -- tu
```

Installs tu (plus the shll meta-CLI) via Homebrew, handling tap trust automatically. To install the entire shll toolkit instead:

```sh
curl -fsSL https://shll.ai/install | sh
```

> 📖 Full walkthrough: the [install guide](docs/site/install.md) covers install, shell completions, and multi-machine setup in depth.

<img width="1025" height="675" alt="tu terminal output showing today's AI coding assistant costs across Claude Code, Codex, OpenCode, Gemini, Copilot, and Kimi" src="https://github.com/user-attachments/assets/d6d1c930-8230-4910-ba1b-985e7df17e7c" />

### Shell completions

```bash
# bash
echo 'eval "$(tu shell-init bash)"' >> ~/.bashrc

# zsh
echo 'eval "$(tu shell-init zsh)"' >> ~/.zshrc

# fish
tu shell-init fish > ~/.config/fish/completions/tu.fish
```

> 💡 Have other shll tools? [`shll shell-install`](https://github.com/sahil87/shll#shll-shell-install--wire-the-rc-file-recommended) handles all of their shell integrations and autocompletions at once.

## Update

```bash
tu update
# brew update
# brew upgrade tu
```

## Usage

> 📖 See [workflows](docs/site/workflows.md) for end-to-end recipes, and the full [command reference](https://shll.ai/tu/commands/) for every command and flag.

```bash
tu                   # Today's cost, all tools
tu cc                # Today's cost, Claude Code
tu h                 # Daily cost history, all tools
tu cc mh             # Monthly cost history, Claude Code
tu m                 # This month's cost, all tools
```

Sources: `cc` (Claude Code), `codex`/`co` (Codex), `oc` (OpenCode), `gemini`/`gem` (Gemini), `copilot`/`cop` (Copilot), `kimi`/`ki` (Kimi), `all` (default)

### Flags

```
  --json / -j          Output data as JSON (data commands only)
  --csv                Output data as CSV (data commands only)
  --md                 Output data as Markdown (data commands only)
  --since / -s <date>  Only include entries on/after date (YYYY-MM-DD or YYYYMMDD, history display)
  --until <date>       Only include entries on/before date (YYYY-MM-DD or YYYYMMDD, history display)
  --full               Show full history (default: last 3 months for daily/weekly history)
  --metric <m>         Scale history bars by 'cost' (default) or 'tokens' (history display)
  --sync               Sync metrics before fetching (multi mode)
  --dry-run            Preview sync without writing (tu sync only)
  --fresh / -f         Bypass cache, fetch fresh data (data commands only)
  --watch / -w         Persistent polling mode with live display (data commands only)
  --interval / -i <s>  Poll interval in seconds (default: 10, range: 5-3600)
  --user / -u <user>   Show usage for a specific user, or 'all' for every user
                       in the metrics repo (multi mode only; repo data — sync for today)
  --by-machine         Show per-machine cost breakdown (data commands only)
  --no-color           Disable ANSI color output
  --no-rain            Disable matrix rain animation in watch mode
```

### Setup (multi-machine sync)

```bash
tu init-metrics git@github.com:you/tu-metrics.git   # Write metrics_repo + clone (one-liner)
tu sync                                             # Push/pull metrics
tu status                                           # Show config and sync state
```

Or set it up by hand: `tu init-conf` scaffolds `~/.config/tu/tu.conf`, edit `metrics_repo` there, then `tu init-metrics` clones it.

**Team setup:** an org can drop `~/.config/tu/org.conf` (via dotfiles/MDM/bootstrap) with `metrics_repo = …` and every machine's `tu` runs in multi mode with zero per-user edits. Personal `~/.config/tu/tu.conf` values still win over org defaults.

For end-to-end recipes — daily snapshots, history pivots, multi-machine sync, and watch mode — see [workflows](docs/site/workflows.md).

## CI / branch protection

`main` is gated by a required status check named **`ci-gate`**. The
[`CI` workflow](https://github.com/sahil87/tu/blob/main/.github/workflows/ci.yml) runs the build and the test suite on
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
[`scripts/ci-gate-ruleset.sh`](https://github.com/sahil87/tu/blob/main/scripts/ci-gate-ruleset.sh):

```bash
scripts/ci-gate-ruleset.sh           # dry-run: preview the ruleset payload
scripts/ci-gate-ruleset.sh --apply   # create/update the ruleset (admin only)
```

The script degrades gracefully — if `gh` is missing, unauthenticated, or lacks
admin scope, it prints the manual steps instead of failing.
