# Project Context

## Stack

- **Language**: TypeScript (strict mode, ES2022 target, NodeNext modules)
- **Runtime**: Node.js >= 18
- **Bundler**: esbuild (single-file ESM bundle to `dist/tu.mjs`)
- **Test runner**: Node.js built-in (`npx tsx --test`)
- **Task runner**: justfile
- **Distribution**: Homebrew tap (`wvrdz/tap`), binary name `tu`
- **License**: PolyForm Internal Use 1.0.0

## Architecture

CLI tool that aggregates cost/usage data from multiple AI coding assistant tools:
- **Claude Code** via `ccusage`
- **Codex** via `ccusage-codex`
- **OpenCode** via `ccusage-opencode`

### Module layout (`src/`)

| Module | Responsibility |
|--------|---------------|
| `cli.ts` | Entry point, argument parsing, command dispatch |
| `types.ts` | Core data interfaces (`UsageEntry`, `UsageTotals`, `ToolConfig`) |
| `fetcher.ts` | Tool execution, JSON parsing, caching, data aggregation |
| `formatter.ts` | Table rendering (print to stdout, render to string[]) |
| `config.ts` | Config file reading (`~/.tu.conf`) |
| `sync.ts` | Multi-machine metrics sync via git repo |
| `watch.ts` | Live polling mode with terminal refresh |
| `rain.ts` | Matrix rain animation for watch mode |
| `panel.ts` | Box/panel drawing for TUI output |
| `sparkline.ts` | Sparkline chart rendering |
| `compositor.ts` | Terminal compositor for watch layout |
| `colors.ts` | ANSI color helpers with `--no-color` support |

### Modes

- **Single mode** (default): reads from local ccusage output only
- **Multi mode**: syncs metrics to a shared git repo for cross-machine aggregation
