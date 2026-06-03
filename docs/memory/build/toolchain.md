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

### Help-dump → shll.ai (build-time CLI help artifact)

- A build-time **help-dump producer** (`scripts/help-dump.mjs`, run via `npm run help-dump`) emits tu's CLI help as `help/tu.json` conforming to a frozen cross-tool contract `{tool, version, captured_at (ISO-8601 UTC, Z), schema_version: 1, root: Node}` where `Node = {name, path, short, usage, text (raw byte-for-byte --help), commands: Node[]}`. tu's document is flat (`commands: []`) because tu prints no per-subcommand help pages.
- The producer captures help by executing the built bundle `node dist/tu.mjs --help` with `NO_COLOR=1`, byte-for-byte (no trim/re-wrap/CRLF/ANSI); reads `version`/`name`/`description` from `package.json`; self-validates the written JSON and exits non-zero on any error (a build artifact fails loud — distinct from the runtime CLI's Constitution II graceful-degradation rule).
- `help/tu.json` is a **build-local artifact** — gitignored (`help/`), never committed to the tu repo; `sahil87/shll.ai` is the source of truth. tu no longer emits or transports it via CI (see below); the producer is now invoked only locally / by future tooling.
- The producer/contract are **retained as the surface shll.ai pulls from**, but tu no longer *pushes* the result anywhere. As of 260603-dmhw, shll.ai inverted the contract: it now **pulls** via its own scheduled cron (`scheduled-help-refresh.yml` in `sahil87/shll.ai`) that `brew install`s tu, runs `tu help-dump`, stamps `captured_at`, validates, and **direct-commits** to its own `main` using the default `GITHUB_TOKEN`. The cron is the single trusted writer of `help/tu.json`.
- A **`.github/workflows/ci.yml`** runs build + tests (`npm ci`, `npm run build`, `npm test`) on push to `main` (node 20, action SHAs pinned identically to `release.yml`); it does NOT publish anything (GitHub Release, Homebrew tap) — those external side effects live only on `release.yml`'s tag-push path.
- **`release.yml`**: on the `v*` tag-push path it builds and publishes (GitHub Release + Homebrew tap update) but **no longer runs the help-dump producer step or pushes `help/tu.json` to `sahil87/shll.ai`** — both the `Generate help/tu.json` step and the `PR help/tu.json into shll.ai` step (clone, branch, `gh pr create`/`gh pr merge --auto --squash`, `SHLLAI_TOKEN` env) were removed when shll.ai inverted to pull (260603-dmhw). `SHLLAI_TOKEN` is no longer used by any workflow (the repo secret is flagged for manual deletion). It retains the release-PR-merge entry point — a `tag-on-release-merge` job (on push to `main`) detects a `release`-labeled merged PR, creates+pushes the `v*` tag, and exposes outputs so the `release` job runs in the SAME workflow run via `needs`/`always()`.

## Design Decisions

- **esbuild over tsc**: Single-file bundle avoids `node_modules` resolution at runtime and produces a self-contained CLI script. ESM format with node shebang.
- **Node.js test runner over Jest/Vitest**: Zero extra test dependencies. `tsx` provides TypeScript support. Test files use `node:test` and `node:assert`.
- **`src/node/` directory structure**: All TypeScript source lives under `src/node/` with subdirectories `core/` (CLI entry, types, config, fetcher), `tui/` (formatter, compositor, panels, watch, rain, sparkline, colors), `sync/` (multi-machine sync), and `scripts/` (release tooling). This namespaces the Node implementation to allow a future `src/rust/` sibling. Tests are co-located in `__tests__/` folders within each subdirectory.
- **Homebrew distribution**: Public tap at `sahil87/homebrew-tap` handles versioning and installation.
- **Vendored ccusage binaries**: ccusage packages are vendored into `dist/vendor/` at build time rather than resolved from `node_modules/.bin/` at runtime. This satisfies Constitution Principle III (no `node_modules` at install time) and simplifies Homebrew distribution by removing the need for `npm install` in the formula. Source-to-vendor mapping: `ccusage` -> `dist/vendor/ccusage/`, `@ccusage/codex` -> `dist/vendor/ccusage-codex/`, `@ccusage/opencode` -> `dist/vendor/ccusage-opencode/`. Dev-mode fallback ensures `tsx`-based development workflow is unaffected (260320-eu2i).
- **Bespoke Node help-dump producer (not the shared Go/Cobra producer)**: tu is a flag-based Node/TS CLI with no walkable subcommand tree, so it cannot reuse the 6 other tools' Go producer that recurses a Cobra tree; a `node:`-only `.mjs` script emits a structurally-valid flat contract document instead. The pure `buildHelpDoc()` is extracted for direct unit testing (co-located TS test imports the `.mjs`); the producer is `.mjs` (not `.ts`) to keep the production/CI capture path free of `tsx` (Constitution III — no new runtime deps) (260602-v76l).
- **Release-merge drives the pipeline via `needs`/outputs, not a tag re-trigger**: a tag pushed with the default `GITHUB_TOKEN` does NOT re-trigger workflows (GitHub's documented loop-prevention), so the release-merge path runs the `release` job in the same run via `needs: tag-on-release-merge` + an `always()`-gated `if` (tolerating the upstream job being skipped on the tag-push/dispatch paths), rather than relying on the suppressed tag push to start a second run. Releases remain tag-anchored. (Caught and corrected during review.) (260602-v76l)
- **shll.ai pull model (push wiring removed)**: tu no longer writes `help/tu.json` to `sahil87/shll.ai` at all — shll.ai inverted the contract to pull on its own daily cron (`scheduled-help-refresh.yml`, direct-commit with `GITHUB_TOKEN`), so a single trusted cron is now the only writer. *Historical (superseded):* the original design pushed via a PR + `gh pr merge --auto --squash` (never a direct push) to serialize the 7-tool concurrent writes through GitHub and avoid a multi-repo push race; with one cron writer that race is moot, so the entire push path was removed (260603-dmhw, superseding 260602-v76l).

## Changelog

| Date | Change |
|------|--------|
| 2026-03-06 | Generated from code analysis |
| 2026-03-06 | Updated esbuild entry point to `src/node/core/cli.ts`, test glob to `src/node/**/__tests__/*.test.ts`, added `src/node/` directory structure note |
| 2026-04-01 | Relicense MIT & migrate to sahil87: updated org refs from wvrdz to sahil87, license from PolyForm to MIT, removed SSH note, removed weaver conf from published files (260401-lomt) |
| 2026-04-01 | Vendor ccusage binaries: moved ccusage/codex/opencode from dependencies to devDependencies, added build-time vendor copy step (clean-before-copy into dist/vendor/), added vendor-first BIN resolution with dev-mode fallback in fetcher.ts (260320-eu2i) |
| 2026-06-03 | Build-time help-dump → shll.ai: added `scripts/help-dump.mjs` producer (frozen `help/tu.json` contract, transient/gitignored), new `ci.yml` (build+test on main), and `release.yml` help-dump→shll.ai PR step + release-PR-merge entry point (260602-v76l) |
| 2026-06-03 | Removed tu's shll.ai push wiring (shll.ai now pulls): deleted the `Generate help/tu.json` and `PR help/tu.json into shll.ai` steps (clone + PR + `gh pr merge --auto --squash` + `SHLLAI_TOKEN` env) from `release.yml`, trimmed the stale "Fail-loud steps FIRST" comment, and updated the `ci.yml` comment. shll.ai now pulls via its own cron (`scheduled-help-refresh.yml`, direct-commit). The `help-dump` producer script, npm script, contract (`schema_version: 1`), and unit test are retained unchanged; `SHLLAI_TOKEN` secret flagged for manual deletion (260603-dmhw) |
