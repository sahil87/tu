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

**Multi mode** aggregates usage across several machines by pushing and pulling metrics through a shared git repository. Enable it by pointing `tu` at a metrics repo, then cloning it locally.

### 1. Point tu at your metrics repo and clone it (one-liner)

```bash
tu init-metrics git@github.com:you/tu-metrics.git
```

This writes `metrics_repo = <url>` into `~/.config/tu/tu.conf` (creating the file from the scaffold if needed) and clones the repo into `metrics_dir` (defaulting to `~/.tu/metrics_repo`). The URL you type wins over an exported `TU_METRICS_REPO` for that invocation.

**Alternative — edit the config by hand:**

```bash
tu init-conf
```

This creates `~/.config/tu/tu.conf` (or fills in any missing fields if it already exists). Then open `~/.config/tu/tu.conf` and set `metrics_repo`:

```ini
metrics_repo = git@github.com:you/tu-metrics.git
```

The config fields are:

| Field | Purpose | Default |
|-------|---------|---------|
| `version` | Config schema version. Currently `2`. | `2` |
| `metrics_repo` | Git repo URL for metrics storage — setting it enables multi-machine sync. Can also be supplied via the `TU_METRICS_REPO` environment variable. | (unset) |
| `metrics_dir` | Optional local path where the metrics repo is cloned. | `~/.tu/metrics_repo` |
| `machine` | Optional label for this machine in the metrics repo. | system hostname |
| `user` | Optional profile name that groups your machines in the metrics repo. | system username |
| `auto_sync` | Note: this no longer auto-triggers a sync. Use `tu <cmd> --sync` to sync before a fetch instead. | `true` |

Then clone with `tu init-metrics` (no argument). If `metrics_repo` isn't set anywhere, `tu` tells you to add it to `~/.config/tu/tu.conf`, run `tu init-metrics <repo-url>`, or set `TU_METRICS_REPO`.

### 2. Config locations & precedence

`tu` reads its config from a fixed root built from `$HOME` only — no `XDG_CONFIG_HOME`, no other env var can move it. Values stack in exactly this order (later wins, no per-key exceptions):

| Layer | File / source | Purpose |
|-------|---------------|---------|
| 1 | `tu.default.conf` (shipped with tu) | Base defaults |
| 2 | `~/.config/tu/org.conf` (optional) | Org-wide defaults — dropped in by an org's dotfiles/MDM/bootstrap; silent when absent |
| 3 | `~/.config/tu/tu.conf` | Personal overrides (created by `tu init-conf` / `tu init-metrics <url>`) |
| 4 | `TU_METRICS_REPO` env var | Deployment bootstrap for `metrics_repo` only |
| 5 | CLI argument | e.g. the `tu init-metrics <repo-url>` argument |

**Team setup:** an org ships `~/.config/tu/org.conf` with `metrics_repo = …` and every employee's `tu` is in multi mode with zero per-user edits; personal `tu.conf` values still win over the org layer.

**Legacy fallback:** an old `~/.tu.conf` is still read when `~/.config/tu/tu.conf` does not exist, with a one-line deprecation warning on stderr. Move it when convenient (`tu init-conf` or `tu init-metrics <url>` seeds the new file from it automatically); the legacy file is never moved or deleted by tu.

### 3. Sync metrics

```bash
tu sync
```

Pushes your local metrics up and pulls everyone else's down. Because `auto_sync` no longer triggers automatically, run `tu sync` (or pass `--sync` to a data command, e.g. `tu --sync`) whenever you want fresh cross-machine data before a fetch. To preview what a sync would write, commit, and push without changing anything, run `tu sync --dry-run`.

### 4. Check status

```bash
tu status
```

Shows your current config and sync state — which machine and user you're recording as, where the metrics repo lives, and whether it's been cloned yet. When an `org.conf` is in play it also prints an `Org config:` line so you can see where the org defaults come from.

Repeat step 1 on each machine you want to include (pointing them all at the same `metrics_repo`), and `tu` will aggregate their costs together in multi mode.
