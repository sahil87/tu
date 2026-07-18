# tu — agent usage bundle

`tu` is a command-line tool that reports AI coding-assistant token cost and usage
on the machine it runs on. It reads local usage data (via a vendored `ccusage`)
and, in multi mode, aggregates across machines through a shared git repo.

This is a usage briefing for an agent operating an installed `tu`: when to reach
for it, what each command does, how it composes, and how to read its output.

## When to use

Reach for `tu` to answer cost/usage questions about AI coding assistants on
**this machine**:

- "How much did Claude Code / Codex / OpenCode / Gemini / Copilot cost today?"
- Daily, weekly, or monthly spend, as a snapshot or as history.
- Per-machine breakdowns (multi mode) and per-user views.

Do NOT reach for `tu` for:

- Billing management or payment — it only reports usage, it changes nothing.
- Tools it does not track (only the five sources below).
- Per-request granularity beyond what `ccusage` emits — `tu` aggregates by day.

## Capabilities map

### Data grammar: `tu [source] [period] [display]`

- **source** — `cc` (Claude Code), `codex`/`co` (Codex), `oc` (OpenCode),
  `gemini`/`gem` (Gemini), `copilot`/`cop` (Copilot); omit for `all` (default).
- **period** — `d`/`daily` (default), `w`/`weekly`, `m`/`monthly`.
- **display** — bare = snapshot (current period); `h`/`history` = time series.
- **combined shorthands** — `dh` (daily history), `wh` (weekly history),
  `mh` (monthly history).

Examples: `tu` (today, all), `tu cc` (today, Claude Code), `tu h` (daily history
pivot), `tu cc mh` (Claude Code monthly history), `tu wh` (weekly history).

### Non-data commands

- `tu init-conf` — scaffold `~/.tu.conf`.
- `tu init-metrics` — clone the metrics repo (multi mode).
- `tu sync` — push/pull metrics manually (multi mode).
- `tu status` — show config and sync state (reports single vs multi mode).
- `tu update` — update `tu` via Homebrew.
- `tu shell-init <bash|zsh|fish>` — emit a shell completion script for `eval`.
- `tu help` (or `-h`/`--help`) — full help text.
- `tu help-dump` — emit a machine-readable help contract (JSON) for tooling.
- `tu skill` — print this bundle (agent usage briefing) to stdout.

## Composition patterns

- **Shells out to `ccusage`** (vendored) to read local usage data — no separate
  install needed; a single `ccusage` binary serves all five sources.
- **Shells out to `git`** in multi mode to sync per-machine metrics through a
  shared repo, so cost aggregates across machines.
- **Shells out to `brew`** for `tu update` (Homebrew-installed builds only).
- **Is shelled out to** by shll.ai's pull cron, which runs `tu help-dump`
  against the installed binary to render tu's command reference on shll.ai.
- **`tu skill`** (this bundle) also renders at `/tu/skill` on shll.ai and is the
  surface a future `shll agent-setup` will aggregate into agent context.

## Output & exit-code contract

- **stdout is data; stderr is diagnostics.** Parse stdout; treat stderr as
  warnings/errors. Success writes results to stdout with nothing on stderr.
- **Exit codes:** `0` on success; `1` on operational failure (network, git,
  Homebrew, missing/misconfigured metrics repo — retry or fix the
  environment); `2` on usage error (bad grammar, unknown tool/shell,
  incompatible flags, bad flag values — fix the command line).
- **Graceful degradation:** when a data source is unavailable, `tu` warns on
  stderr and falls back to the best available data (cached, local-only, or
  zero) rather than crashing — so a non-empty stderr can accompany exit 0.
- **Machine-readable formats** (data commands only, mutually exclusive):
  - `--json` / `-j` — JSON.
  - `--csv` — CSV.
  - `--md` — Markdown table.
  Prefer `--json` for programmatic parsing. Bare output is a formatted table.

## Gotchas

- **Cached data.** Fetches are cached (~60s TTL). Pass `--fresh` / `-f` to bypass
  the cache and refetch.
- **Single vs multi mode.** Behavior depends on `~/.tu.conf`: single mode reads
  only local data; multi mode aggregates across machines via the metrics repo.
  Run `tu status` to see which mode is active. `--user` / `-u` and multi-machine
  views apply in multi mode only (warned-and-ignored in single mode).
- **`--watch` / `-w` is an interactive TUI** (live-refreshing display). Do not
  invoke it from an agent — it does not terminate on its own and produces no
  parseable single-shot output.
- **Use `--no-color`** for clean, parseable output (disables ANSI color codes).
- **Implicit 3-month cap.** Daily and weekly history default to the last ~3
  calendar months. Pass `--full` for the complete history. Monthly history
  (`mh`) is never capped.
- **History-only flags.** `--since` / `-s` and `--until` bound a history window
  (`YYYY-MM-DD` or `YYYYMMDD`); on a snapshot display they warn and are ignored.
