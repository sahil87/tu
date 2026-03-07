# Intake: Reorganize src/ into Node Namespace

**Change**: 260306-x861-reorganize-src-node-namespace
**Created**: 2026-03-06
**Status**: Draft

## Origin

> User wants to reorganize the flat `src/` directory to prepare for a future Rust implementation of the CLI. Discussion via `/fab-discuss` established the target structure: move all TypeScript source into `src/node/` with `core/`, `tui/`, and `sync/` subdirectories. The user confirmed the grouping and naming (`src/node/` over `src/node-based/`).

## Why

The project plans a parallel Rust implementation of the CLI. Currently all 12 TypeScript source files sit flat in `src/`, which would conflict with a `src/rust/` sibling. Moving the Node implementation into `src/node/` establishes a clean namespace boundary. As a secondary benefit, the flat layout doesn't reflect the natural module clusters (core data pipeline, TUI rendering, sync) — subdirectories make the architecture self-documenting.

## What Changes

### Directory Structure

Move all TypeScript files from `src/` into `src/node/` with three subdirectories:

```
src/
├── node/
│   ├── core/
│   │   ├── cli.ts
│   │   ├── types.ts
│   │   ├── config.ts
│   │   └── fetcher.ts
│   ├── tui/
│   │   ├── formatter.ts
│   │   ├── panel.ts
│   │   ├── colors.ts
│   │   ├── sparkline.ts
│   │   ├── compositor.ts
│   │   ├── watch.ts
│   │   └── rain.ts
│   ├── sync/
│   │   └── sync.ts
│   └── scripts/
│       └── release.sh
```

### Import Path Updates

All inter-module imports must be updated to reflect the new directory depth. The project uses `.js` extensions in imports per constitution (NodeNext module resolution). Examples:

- `cli.ts` currently imports `./types.js` → becomes `./types.js` (same dir, no change) or cross-dir like `../tui/formatter.js`
- `watch.ts` imports `./compositor.js`, `./rain.js` → stays `./compositor.js`, `./rain.js` (same `tui/` dir)
- `watch.ts` imports `./fetcher.js` → becomes `../core/fetcher.js`
- `formatter.ts` imports `./colors.js` → stays `./colors.js` (same `tui/` dir)
- `formatter.ts` imports `./types.js` → becomes `../core/types.js`

### Build Configuration

- **esbuild** entry point (in `justfile` or build script): update from `src/cli.ts` to `src/node/core/cli.ts`
- **tsconfig.json**: update `include` or `rootDir` if they reference `src/` paths explicitly

### Test Co-location

Per the constitution's "Test Location" rule, tests must be co-located in `__tests__/` folders. Current tests in `tests/*.test.ts` move to:

```
src/node/core/__tests__/
src/node/tui/__tests__/
src/node/sync/__tests__/
```

Each test file goes into the `__tests__/` folder of the module it tests. Test runner config (`npx tsx --test`) needs a glob update to find the new locations.

### justfile / Task Runner

Update any `just` recipes that reference `src/` paths (build, test, lint commands).

## Affected Memory

- `build/toolchain`: (modify) Update esbuild entry point, test runner glob, directory structure description
- `cli/data-pipeline`: (modify) Update file paths to reflect `src/node/core/` location
- `display/formatting`: (modify) Update file paths to reflect `src/node/tui/` location
- `watch-mode/tui`: (modify) Update file paths to reflect `src/node/tui/` location
- `sync/multi-machine`: (modify) Update file paths to reflect `src/node/sync/` location

## Impact

- **All source files**: Every `.ts` file moves and may need import updates
- **Build pipeline**: esbuild entry point changes
- **Test runner**: Test file glob pattern changes
- **justfile**: Build/test recipes update
- **No runtime behavior change**: This is purely structural — the compiled bundle (`dist/tu.mjs`) is identical

## Open Questions

None — the structure was discussed and agreed upon.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Use `src/node/` not `src/node-based/` | Discussed — user agreed to shorter name | S:95 R:90 A:95 D:95 |
| 2 | Certain | Three subdirs: `core/`, `tui/`, `sync/` | Discussed — user confirmed grouping | S:90 R:85 A:90 D:95 |
| 3 | Certain | `cli.ts`, `types.ts`, `config.ts`, `fetcher.ts` → `core/` | Discussed — these are the data pipeline modules | S:90 R:85 A:90 D:90 |
| 4 | Certain | All TUI/display modules → `tui/` | Discussed — user confirmed grouping | S:90 R:85 A:90 D:90 |
| 5 | Certain | `sync.ts` → `sync/` | Discussed — distinct domain with own memory file | S:85 R:85 A:90 D:90 |
| 6 | Certain | Do NOT create `src/rust/` in this change | Discussed — user explicitly stated future only | S:95 R:95 A:95 D:95 |
| 7 | Certain | Co-locate tests in `__tests__/` folders | Constitution rule — updated during this session | S:95 R:80 A:95 D:95 |
| 8 | Confident | `scripts/release.sh` moves to `src/node/scripts/` | Discussed — release script is Node-build specific | S:75 R:85 A:70 D:80 |
| 9 | Confident | No barrel/index files needed | Codebase uses direct imports, no re-export pattern exists | S:60 R:90 A:80 D:75 |

9 assumptions (7 certain, 2 confident, 0 tentative, 0 unresolved).
