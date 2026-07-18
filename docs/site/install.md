# Install guide

A complete walkthrough for installing `tu`, wiring up shell completions, and (optionally) setting up multi-machine metrics sync. For day-to-day command recipes once you're set up, see [workflows](workflows.md).

## Install with Homebrew

`tu` is distributed through the `sahil87` Homebrew tap. Add the tap once, then install:

```bash
brew tap sahil87/tap
brew install tu
```

- `brew tap sahil87/tap` registers the tap so Homebrew knows where to find the formula.
- `brew install tu` installs the `tu` binary onto your `PATH`.

Verify the install:

```bash
tu --help
```

## Updating

`tu` ships a self-update command:

```bash
tu update
```

`tu update` refreshes the tap (runs `brew update`) and upgrades `tu` to the latest released version. Two related options:

- `tu update --skip-brew-update` — skip the `brew update` tap refresh during the update (useful when you've just tapped or refreshed and want a faster upgrade).
- `brew upgrade tu` — the standard Homebrew path also works if you'd rather manage `tu` like any other formula.

## Shell completions

`tu` emits its own shell init script via `tu shell-init <shell>`. Wire it into your shell so you get tab completions. Pick the line for your shell:

```bash
# bash
echo 'eval "$(tu shell-init bash)"' >> ~/.bashrc

# zsh
echo 'eval "$(tu shell-init zsh)"' >> ~/.zshrc

# fish
tu shell-init fish > ~/.config/fish/completions/tu.fish
```

What each line does:

- **bash** — appends `eval "$(tu shell-init bash)"` to `~/.bashrc`. On each new shell, bash runs `tu shell-init bash` and evaluates the emitted completion script.
- **zsh** — appends `eval "$(tu shell-init zsh)"` to `~/.zshrc`, doing the same for zsh.
- **fish** — writes the completion script directly into fish's completions directory (`~/.config/fish/completions/tu.fish`), where fish autoloads it. No `eval` is needed because fish discovers completions from that directory.

Open a new shell (or `source` the relevant rc file) for completions to take effect.

> Tip: Have other `sahil87` tools? [`shll shell-install`](https://github.com/sahil87/shll#shll-shell-install--wire-the-rc-file-recommended) wires up every `sahil87` tool's shell integration and autocompletions at once, so you don't have to add each tool's line by hand.

## Multi-machine sync setup

By default, `tu` runs in **single mode**: it reads `ccusage` output from the local machine only and shows you that machine's costs. No configuration is required.

**Multi mode** aggregates usage across several machines by pushing and pulling metrics through a shared git repository. Enable it by pointing `tu` at a metrics repo, then cloning it locally. The steps:

### 1. Scaffold the config

```bash
tu init-conf
```

This creates `~/.tu.conf` (or fills in any missing fields if it already exists). Then open `~/.tu.conf` and edit it.

### 2. Edit `~/.tu.conf`

The config fields are:

| Field | Purpose | Default |
|-------|---------|---------|
| `version` | Config schema version. Currently `2`. | `2` |
| `metrics_repo` | Git repo URL for metrics storage — setting it enables multi-machine sync. Can also be supplied via the `TU_METRICS_REPO` environment variable. | (unset) |
| `metrics_dir` | Optional local path where the metrics repo is cloned. | `~/.tu/metrics_repo` |
| `machine` | Optional label for this machine in the metrics repo. | system hostname |
| `user` | Optional profile name that groups your machines in the metrics repo. | system username |
| `auto_sync` | Note: this no longer auto-triggers a sync. Use `tu <cmd> --sync` to sync before a fetch instead. | `true` |

At minimum, set `metrics_repo` (or export `TU_METRICS_REPO`) to a git repo you control, for example:

```ini
metrics_repo = git@github.com:you/tu-metrics.git
```

### 3. Clone the metrics repo

```bash
tu init-metrics
```

Clones `metrics_repo` into `metrics_dir` (defaulting to `~/.tu/metrics_repo`). If `metrics_repo` isn't set, `tu` tells you to add it to `~/.tu.conf` or set `TU_METRICS_REPO`.

### 4. Sync metrics

```bash
tu sync
```

Pushes your local metrics up and pulls everyone else's down. Because `auto_sync` no longer triggers automatically, run `tu sync` (or pass `--sync` to a data command, e.g. `tu --sync`) whenever you want fresh cross-machine data before a fetch. To preview what a sync would write, commit, and push without changing anything, run `tu sync --dry-run`.

### 5. Check status

```bash
tu status
```

Shows your current config and sync state — which machine and user you're recording as, where the metrics repo lives, and whether it's been cloned yet.

Repeat steps 1–4 on each machine you want to include (pointing them all at the same `metrics_repo`), and `tu` will aggregate their costs together in multi mode.
