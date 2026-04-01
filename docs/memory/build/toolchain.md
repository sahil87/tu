# Build & Test Toolchain

## Overview

The project uses esbuild for bundling and Node.js built-in test runner (via `tsx`) for testing. Distributed via Homebrew tap from the sahil87 GitHub org.

## Requirements

- Build MUST use esbuild: `esbuild src/node/core/cli.ts --bundle --platform=node --format=esm --outfile=dist/tu.mjs --banner:js='#!/usr/bin/env node'`
- Tests MUST run via `npx tsx --test 'src/node/**/__tests__/*.test.ts'` (Node.js built-in test runner, not Jest/Vitest)
- TypeScript config: `target: ES2022`, `module: NodeNext`, `strict: true`
- Package MUST be distributed as ESM (`"type": "module"`)
- Binary name MUST be `tu` (via `"bin": { "tu": "dist/tu.mjs" }`)
- `prepublishOnly` MUST run `npm run build`
- Published files: `dist/`, `tu.default.conf`
- Node.js engine requirement: `>= 18`
- Dependencies: `ccusage`, `@ccusage/codex`, `@ccusage/opencode` (runtime); `@types/node`, `esbuild`, `tsx`, `typescript` (dev)
- License: MIT
- Distribution: Homebrew tap at `sahil87/tap`

## Design Decisions

- **esbuild over tsc**: Single-file bundle avoids `node_modules` resolution at runtime and produces a self-contained CLI script. ESM format with node shebang.
- **Node.js test runner over Jest/Vitest**: Zero extra test dependencies. `tsx` provides TypeScript support. Test files use `node:test` and `node:assert`.
- **`src/node/` directory structure**: All TypeScript source lives under `src/node/` with subdirectories `core/` (CLI entry, types, config, fetcher), `tui/` (formatter, compositor, panels, watch, rain, sparkline, colors), `sync/` (multi-machine sync), and `scripts/` (release tooling). This namespaces the Node implementation to allow a future `src/rust/` sibling. Tests are co-located in `__tests__/` folders within each subdirectory.
- **Homebrew distribution**: Public tap at `sahil87/homebrew-tap` handles versioning and installation.

## Changelog

| Date | Change |
|------|--------|
| 2026-03-06 | Generated from code analysis |
| 2026-03-06 | Updated esbuild entry point to `src/node/core/cli.ts`, test glob to `src/node/**/__tests__/*.test.ts`, added `src/node/` directory structure note |
| 2026-04-01 | Relicense MIT & migrate to sahil87: updated org refs from wvrdz to sahil87, license from PolyForm to MIT, removed SSH note, removed weaver conf from published files (260401-lomt) |
