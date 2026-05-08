# CLI & Data Pipeline

## Overview

`tu` is a Node.js CLI tool (binary name: `tu`) that tracks AI coding assistant costs across Claude Code, Codex, and OpenCode. It parses positional arguments into a `{source, period, display}` grammar and dispatches to data-fetching and formatting pipelines.

Entry point: `src/node/core/cli.ts`. Data types: `src/node/core/types.ts`. Data fetching: `src/node/core/fetcher.ts`. Config: `src/node/core/config.ts`.

## Requirements

- The CLI MUST support positional argument grammar: `tu [source] [period] [display]`
- Sources MUST include: `cc` (Claude Code), `codex`/`co` (Codex), `oc` (OpenCode), `all` (default)
- Periods MUST include: `d`/`daily` (default), `m`/`monthly`; combined shorthands `dh`, `mh`
- Display MUST include: bare (snapshot, default), `h`/`history`
- Global flags: `--json`, `--csv`, `--md`, `--sync`, `--fresh`/`-f`, `--watch`/`-w`, `--interval`/`-i <s>`, `--user`/`-u <user>`, `--by-machine`, `--no-color`, `--no-rain`, `--version`/`-V`/`-v`
- `--by-machine` MUST show per-machine cost breakdown columns in tables; works with snapshot and single-tool history; incompatible with all-tools history pivot (warn on stderr and ignore); compatible with `--watch`, `--json`, and `-u`; in single mode, shows one machine (hostname); uses `fetchToolMergedWithMachines` which returns `{ entries, machineMap }` where machineMap is `Map<string, UsageEntry[]>` (machine → entries)
- `--user`/`-u` MUST set a target user for data display (multi mode only); in single mode, warn on stderr and ignore; when targeting a different user, display only metrics repo data (no local ccusage data); when targeting the same user as `config.user`, behave identically to the default (no `-u`) path — perform a local fetch (using cache unless `--fresh`/`-f` is provided), write to the metrics repo, then merge with other machines
- `--json`, `--csv`, and `--md` MUST be mutually exclusive with each other; `--watch` MUST be incompatible with any of them. Violations print `Error: {flag-a} and {flag-b} are incompatible` to stderr and exit 1 (`parseGlobalFlags` in `src/node/core/cli.ts`)
- `--interval` range: 5-3600 seconds, default 10
- Non-data commands (`init-conf`, `init-metrics`, `sync`, `status`, `update`, `completions`, `help`) MUST be dispatched before grammar parsing
- `tu completions <shell>` MUST emit a static bash/zsh/fish completion script to stdout (scripts live in `src/node/core/completions.ts` as `BASH_COMPLETION`/`ZSH_COMPLETION`/`FISH_COMPLETION` string constants); no argument prints usage + install examples and exits 0; unknown shell prints `Unknown shell: {shell}. Supported: bash, zsh, fish` to stderr and exits 1. Scripts cover the full grammar (sources, periods, display tokens, non-data subcommands, every long/short flag, and the `bash`/`zsh`/`fish` args to `completions` itself)
- `tu update` MUST detect Homebrew installation via `_pkgDir.includes("/Cellar/tu/")`, show a helpful message for non-brew installs (exit 0), and run `brew update` → `brew info` → `brew upgrade` for brew installs with specific error messages per failure
- Data fetching MUST use `ccusage` family binaries (`ccusage`, `ccusage-codex`, `ccusage-opencode`) via `child_process.execFile` — invoked with the tool's `binary` as the first argument and a single argv array as the second (`[...tool.prefixArgs, period, "--json", ...extraArgs]`); no shell subprocess is forked. On error, warn on stderr (`warning: {toolName} fetch failed (...), showing zero data`) and resolve the wrapper Promise with `""` so downstream parsing yields `EMPTY` totals. `maxBuffer` remains 10MB
- Fetch results MUST be cached in `~/.tu/cache/` with 60-second TTL; `--fresh` bypasses cache
- The `TOOLS` registry maps tool keys to `ToolConfig` objects with `name` (display name), `binary` (executable path), `prefixArgs` (string[] prepended before period/flags — e.g., `["<vendor>/ccusage/index.js"]` in vendor mode, `[]` in non-vendor mode with the on-PATH binary), and `needsFilter` fields
- `needsFilter` tools (Codex, OpenCode) require stripping lines starting with `[` before JSON parsing
- Date labels MUST be normalized from human-readable (`"Feb 14, 2026"`) to ISO (`"2026-02-14"`)
- Monthly aggregation MUST be computed client-side from daily entries (slice label to `YYYY-MM`, sum tokens/cost)
- Output format is represented by `outputFormat: "table" | "json" | "csv" | "md"` on `GlobalFlags` (`src/node/core/cli.ts`). `parseGlobalFlags` resolves the enum from mutually-exclusive flags; dispatch functions (`dispatchAllHistory`, `dispatchAllSnapshot`, `dispatchSingleTool`) fetch once and switch on `outputFormat` to call `emitJson`, `emitCsv`, `emitMarkdown`, or the existing `print*` renderers. Legacy `jsonFlag` boolean is retained for internal compatibility

## Design Decisions

- **Single binary via esbuild**: All TypeScript is bundled into `dist/tu.mjs` with `--platform=node --format=esm`. No runtime transpilation needed.
- **ccusage as data source**: Token/cost data is fetched by shelling out to `ccusage` binaries (from npm `ccusage`, `@ccusage/codex`, `@ccusage/opencode` packages) rather than reading JSONL files directly. The `--json` flag is always passed to get structured output.
- **Cache-first fetching**: A filesystem cache (`~/.tu/cache/{toolKey}-daily.json`) avoids re-scanning 500MB+ of JSONL files on every invocation. 60s TTL balances freshness with performance.
- **Positional grammar over subcommands**: `tu cc mh` reads more naturally than `tu history --source cc --period monthly`. Source aliases (`co` -> `codex`) keep it terse.
- **Two dispatch paths**: Non-watch mode calls `dispatchAllHistory`/`dispatchAllSnapshot`/`dispatchSingleTool` which print directly. Watch mode calls `*Lines` variants returning `string[]` for compositor consumption.
- **`execFile` over `exec` for child processes** (260423-lx0g): ccusage invocations in `fetcher.ts` and all git calls in `sync/sync.ts` use `execFile(binary, argv[])` instead of `exec(cmdString)`. Removes the `/bin/sh` fork per call, eliminates shell-injection surface on interpolated values (`config.metricsDir`, `config.user`), and passes paths containing spaces/quotes as literal argv entries. `TOOLS` splits the compound command into `binary` + `prefixArgs` to map directly onto the `execFile` signature.
- **Output format as a single enum plumbed through dispatch** (260423-lx0g): `outputFormat: "table" | "json" | "csv" | "md"` is resolved once in `parseGlobalFlags` and each dispatch function switches on it in one place. Avoids multiplying `if (jsonFlag) ... else ...` branches as new formats are added.
- **Static shell completion scripts** (260423-lx0g): Bash/zsh/fish completion scripts are hardcoded string constants in `src/node/core/completions.ts`. No dynamic lookup (the grammar is stable and runtime `tu --list-*` calls would add 50-200ms per tab-press); no `$SHELL` auto-detection (explicit arg avoids silent mismatches when `$SHELL` and the running shell diverge).

## Changelog

| Date | Change |
|------|--------|
| 2026-03-06 | Generated from code analysis |
| 2026-03-06 | Updated file paths from `src/` to `src/node/core/` for cli, types, fetcher, config |
| 2026-03-07 | Added `--user`/`-u` flag for viewing another user's usage in multi mode |
| 2026-03-07 | Added `tu update` self-update command (Homebrew detection, brew update/info/upgrade flow) |
| 2026-03-07 | Fixed `-u` same-user path to fetch fresh local data instead of reading stale metrics repo |
| 2026-03-07 | Added `--by-machine` flag for per-machine cost distribution columns (letter-coded A/B/C with legend) |
| 2026-04-01 | Added `-v` (lowercase) as version flag alias alongside `--version` and `-V` (260401-kuuh) |
| 2026-04-23 | Migrated child process spawning from `exec` to `execFile` with argv arrays; `TOOLS` shape changed from `{name, command, needsFilter}` to `{name, binary, prefixArgs, needsFilter}`; added `--csv`/`--md` global flags and `outputFormat` enum dispatch; added `tu completions <shell>` non-data subcommand with static bash/zsh/fish scripts (260423-lx0g) |
