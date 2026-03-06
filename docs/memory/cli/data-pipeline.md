# CLI & Data Pipeline

## Overview

`tu` is a Node.js CLI tool (binary name: `tu`) that tracks AI coding assistant costs across Claude Code, Codex, and OpenCode. It parses positional arguments into a `{source, period, display}` grammar and dispatches to data-fetching and formatting pipelines.

Entry point: `src/cli.ts`. Data types: `src/types.ts`. Data fetching: `src/fetcher.ts`.

## Requirements

- The CLI MUST support positional argument grammar: `tu [source] [period] [display]`
- Sources MUST include: `cc` (Claude Code), `codex`/`co` (Codex), `oc` (OpenCode), `all` (default)
- Periods MUST include: `d`/`daily` (default), `m`/`monthly`; combined shorthands `dh`, `mh`
- Display MUST include: bare (snapshot, default), `h`/`history`
- Global flags: `--json`, `--sync`, `--fresh`/`-f`, `--watch`/`-w`, `--interval`/`-i <s>`, `--no-color`, `--no-rain`
- `--watch` and `--json` MUST be mutually exclusive
- `--interval` range: 5-3600 seconds, default 10
- Non-data commands (`init-conf`, `init-metrics`, `sync`, `status`, `help`) MUST be dispatched before grammar parsing
- Data fetching MUST use `ccusage` family binaries (`ccusage`, `ccusage-codex`, `ccusage-opencode`) via child process `exec`
- Fetch results MUST be cached in `~/.tu/cache/` with 60-second TTL; `--fresh` bypasses cache
- The `TOOLS` registry maps tool keys to `ToolConfig` objects with `name`, `command`, and `needsFilter` fields
- `needsFilter` tools (Codex, OpenCode) require stripping lines starting with `[` before JSON parsing
- Date labels MUST be normalized from human-readable (`"Feb 14, 2026"`) to ISO (`"2026-02-14"`)
- Monthly aggregation MUST be computed client-side from daily entries (slice label to `YYYY-MM`, sum tokens/cost)

## Design Decisions

- **Single binary via esbuild**: All TypeScript is bundled into `dist/tu.mjs` with `--platform=node --format=esm`. No runtime transpilation needed.
- **ccusage as data source**: Token/cost data is fetched by shelling out to `ccusage` binaries (from npm `ccusage`, `@ccusage/codex`, `@ccusage/opencode` packages) rather than reading JSONL files directly. The `--json` flag is always passed to get structured output.
- **Cache-first fetching**: A filesystem cache (`~/.tu/cache/{toolKey}-daily.json`) avoids re-scanning 500MB+ of JSONL files on every invocation. 60s TTL balances freshness with performance.
- **Positional grammar over subcommands**: `tu cc mh` reads more naturally than `tu history --source cc --period monthly`. Source aliases (`co` -> `codex`) keep it terse.
- **Two dispatch paths**: Non-watch mode calls `dispatchAllHistory`/`dispatchAllSnapshot`/`dispatchSingleTool` which print directly. Watch mode calls `*Lines` variants returning `string[]` for compositor consumption.

## Changelog

| Date | Change |
|------|--------|
| 2026-03-06 | Generated from code analysis |
