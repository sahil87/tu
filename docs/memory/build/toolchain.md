# Build & Test Toolchain

## Overview

The project uses esbuild for bundling and Node.js built-in test runner (via `tsx`) for testing. Distributed via Homebrew tap from the wvrdz GitHub org.

## Requirements

- Build MUST use esbuild: `esbuild src/cli.ts --bundle --platform=node --format=esm --outfile=dist/tu.mjs --banner:js='#!/usr/bin/env node'`
- Tests MUST run via `npx tsx --test tests/*.test.ts` (Node.js built-in test runner, not Jest/Vitest)
- TypeScript config: `target: ES2022`, `module: NodeNext`, `strict: true`
- Package MUST be distributed as ESM (`"type": "module"`)
- Binary name MUST be `tu` (via `"bin": { "tu": "dist/tu.mjs" }`)
- `prepublishOnly` MUST run `npm run build`
- Published files: `dist/`, `tu.default.conf`, `tu.default.weaver.conf`
- Node.js engine requirement: `>= 18`
- Dependencies: `ccusage`, `@ccusage/codex`, `@ccusage/opencode` (runtime); `@types/node`, `esbuild`, `tsx`, `typescript` (dev)
- License: PolyForm-Internal-Use-1.0.0
- Distribution: Homebrew tap at `wvrdz/tap` (requires SSH access to wvrdz GitHub org)

## Design Decisions

- **esbuild over tsc**: Single-file bundle avoids `node_modules` resolution at runtime and produces a self-contained CLI script. ESM format with node shebang.
- **Node.js test runner over Jest/Vitest**: Zero extra test dependencies. `tsx` provides TypeScript support. Test files use `node:test` and `node:assert`.
- **Homebrew distribution**: Private tap at `wvrdz/homebrew-tap` handles versioning and installation. SSH-gated access controls distribution.

## Changelog

| Date | Change |
|------|--------|
| 2026-03-06 | Generated from code analysis |
