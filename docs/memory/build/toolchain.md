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
- Dependencies: `@types/node`, `esbuild`, `tsx`, `typescript`, `ccusage`, `@ccusage/codex`, `@ccusage/opencode` (all devDependencies — ccusage packages are build-time only, vendored into `dist/vendor/` during build)
- Vendor distribution: Build step copies ccusage `dist/*.js` files into `dist/vendor/{ccusage,ccusage-codex,ccusage-opencode}/` (clean-before-copy to prevent stale chunks)
- BIN resolution: Vendor-first — resolves `vendor/` relative to `__dirname` (via `import.meta.url`); falls back to `node_modules/.bin/` when vendor dir absent (dev mode)
- Tool invocation: Vendor mode uses `node {BIN}/{pkg}/index.js`; dev mode uses `{BIN}/{name}` directly
- License: MIT
- Distribution: Homebrew tap at `sahil87/tap`

## Design Decisions

- **esbuild over tsc**: Single-file bundle avoids `node_modules` resolution at runtime and produces a self-contained CLI script. ESM format with node shebang.
- **Node.js test runner over Jest/Vitest**: Zero extra test dependencies. `tsx` provides TypeScript support. Test files use `node:test` and `node:assert`.
- **`src/node/` directory structure**: All TypeScript source lives under `src/node/` with subdirectories `core/` (CLI entry, types, config, fetcher), `tui/` (formatter, compositor, panels, watch, rain, sparkline, colors), `sync/` (multi-machine sync), and `scripts/` (release tooling). This namespaces the Node implementation to allow a future `src/rust/` sibling. Tests are co-located in `__tests__/` folders within each subdirectory.
- **Homebrew distribution**: Public tap at `sahil87/homebrew-tap` handles versioning and installation.
- **Vendored ccusage binaries**: ccusage packages are vendored into `dist/vendor/` at build time rather than resolved from `node_modules/.bin/` at runtime. This satisfies Constitution Principle III (no `node_modules` at install time) and simplifies Homebrew distribution by removing the need for `npm install` in the formula. Source-to-vendor mapping: `ccusage` -> `dist/vendor/ccusage/`, `@ccusage/codex` -> `dist/vendor/ccusage-codex/`, `@ccusage/opencode` -> `dist/vendor/ccusage-opencode/`. Dev-mode fallback ensures `tsx`-based development workflow is unaffected (260320-eu2i).

## Changelog

| Date | Change |
|------|--------|
| 2026-03-06 | Generated from code analysis |
| 2026-03-06 | Updated esbuild entry point to `src/node/core/cli.ts`, test glob to `src/node/**/__tests__/*.test.ts`, added `src/node/` directory structure note |
| 2026-04-01 | Relicense MIT & migrate to sahil87: updated org refs from wvrdz to sahil87, license from PolyForm to MIT, removed SSH note, removed weaver conf from published files (260401-lomt) |
| 2026-04-01 | Vendor ccusage binaries: moved ccusage/codex/opencode from dependencies to devDependencies, added build-time vendor copy step (clean-before-copy into dist/vendor/), added vendor-first BIN resolution with dev-mode fallback in fetcher.ts (260320-eu2i) |
