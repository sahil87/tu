## 2026-06-10

- **Update** [data-pipeline](/cli/data-pipeline.md) — Self-view fix: added pure `maxMergeEntries` to `fetcher.ts`; multi-mode `fetchToolMerged`/`fetchToolMergedWithMachines` read all machines (`excludeMachine = null`) and max-merge own-machine snapshots into the live view before the sum-merge; single mode and `-u <other-user>` unchanged (srmi)

## 2026-05-31

- **Update** [data-pipeline](/cli/data-pipeline.md) — Added `--skip-brew-update` flag to `tu update` — skips only the internal `brew update --quiet` refresh; version check and `brew upgrade` unaffected (e96v)

## 2026-04-23

- **Update** [data-pipeline](/cli/data-pipeline.md) — Migrated child process spawning from `exec` to `execFile` with argv arrays; `TOOLS` shape changed from `{name, command, needsFilter}` to `{name, binary, prefixArgs, needsFilter}`; added `--csv`/`--md` global flags and `outputFormat` enum dispatch; added `tu completions <shell>` non-data subcommand with static bash/zsh/fish scripts (lx0g)

## 2026-04-01

- **Update** [data-pipeline](/cli/data-pipeline.md) — Added `-v` (lowercase) as version flag alias alongside `--version` and `-V` (kuuh)

## 2026-03-07

- **Update** [data-pipeline](/cli/data-pipeline.md) — Added `--user`/`-u` flag for viewing another user's usage in multi mode
- **Update** [data-pipeline](/cli/data-pipeline.md) — Added `tu update` self-update command (Homebrew detection, brew update/info/upgrade flow)
- **Update** [data-pipeline](/cli/data-pipeline.md) — Fixed `-u` same-user path to fetch fresh local data instead of reading stale metrics repo
- **Update** [data-pipeline](/cli/data-pipeline.md) — Added `--by-machine` flag for per-machine cost distribution columns (letter-coded A/B/C with legend)

## 2026-03-06

- **Update** [data-pipeline](/cli/data-pipeline.md) — Generated from code analysis
- **Update** [data-pipeline](/cli/data-pipeline.md) — Updated file paths from `src/` to `src/node/core/` for cli, types, fetcher, config
