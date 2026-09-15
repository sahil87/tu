# Project Context

## Stack

- **Language**: TypeScript (strict mode, ES2022 target, NodeNext modules) — the shipped implementation under `src/node/`. A Go successor is being built under `src/go/` (see Go Transition below).
- **Runtime**: Node.js >= 18
- **Bundler**: esbuild (single-file ESM bundle to `dist/tu.mjs`)
- **Test runner**: Node.js built-in (`npm test` → `npx tsx --test` over `src/node/**/__tests__/*.test.ts`)
- **Task runner**: justfile
- **Distribution**: Homebrew tap (`sahil87/tap`), binary name `tu`
- **License**: MIT

## Go Transition

tu is being ported to Go behind unchanged external contracts (plan: `fab/plans/sahil/26-09-15-go-port.md`). Two implementation trees coexist in `main` during the port:

| Tree | Role | Ships? |
|------|------|--------|
| `src/node/` | Current TypeScript implementation | Yes — `dist/tu.mjs` and the Homebrew formula are built from it until cutover |
| `src/go/` | Successor Go implementation, landing change by change | No — built and tested in CI only, until the cutover change flips the formula |

Go tests are `_test.go` siblings of the code they test (no `__tests__/` under `src/go/`). The constitution's Go Transition article (v1.2.0) is the binding statement; this section is the orientation note.

## Architecture

CLI tool that aggregates cost/usage data from multiple AI coding assistant tools:
- **Claude Code** via `ccusage`
- **Codex** via `ccusage-codex`
- **OpenCode** via `ccusage-opencode`

### Module layout (`src/node/`)

| Module | Responsibility |
|--------|---------------|
| `core/cli.ts` | Entry point, argument parsing, command dispatch |
| `core/types.ts` | Core data interfaces (`UsageEntry`, `UsageTotals`, `ToolConfig`) |
| `core/fetcher.ts` | Tool execution, JSON parsing, caching, data aggregation |
| `core/config.ts` | Config file reading (`~/.config/tu/tu.conf`, org.conf layer) |
| `core/leaderboard.ts` | Leaderboard (`lb`/`lbh`) data shaping |
| `core/help-dump.ts` | `tu help-dump` contract document |
| `core/skill.ts` | `tu skill` agent bundle |
| `core/completions.ts` | Shell completions |
| `sync/sync.ts` | Multi-machine metrics sync via git repo |
| `tui/formatter.ts` | Table rendering (print to stdout, render to string[]) |
| `tui/watch.ts` | Live polling mode with terminal refresh |
| `tui/rain.ts` | Matrix rain animation for watch mode |
| `tui/panel.ts` | Box/panel drawing for TUI output |
| `tui/compositor.ts` | Terminal compositor for watch layout |
| `tui/colors.ts` | ANSI color helpers with `--no-color` support |

Tests are co-located in `__tests__/` folders within each subdirectory (`core/__tests__/`, `sync/__tests__/`, `tui/__tests__/`).

### Modes

- **Single mode** (default): reads from local ccusage output only
- **Multi mode**: syncs metrics to a shared git repo for cross-machine aggregation
