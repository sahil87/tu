# Quality Checklist: Vendor ccusage Binaries for Single-Bundle Distribution

**Change**: 260320-eu2i-vendor-ccusage-binaries
**Generated**: 2026-04-01
**Spec**: `spec.md`

## Functional Completeness

- [ ] CHK-001 Vendor copy step: `npm run build` produces `dist/vendor/` with all three subdirectories
- [ ] CHK-002 Vendor copy mapping: `ccusage` -> `dist/vendor/ccusage/`, `@ccusage/codex` -> `dist/vendor/ccusage-codex/`, `@ccusage/opencode` -> `dist/vendor/ccusage-opencode/`
- [ ] CHK-003 Chunk files: All `.js` files from each package's `dist/` are copied (not just `index.js`)
- [ ] CHK-004 Clean-before-copy: Vendor copy removes and recreates `dist/vendor/` to prevent stale files
- [ ] CHK-005 Build script integration: Both `package.json` `build` script and `justfile` `build` target include vendor copy
- [ ] CHK-006 BIN resolution: `fetcher.ts` resolves to `vendor/` relative to `__dirname` when it exists
- [ ] CHK-007 Dev fallback: BIN falls back to `node_modules/.bin/` when `vendor/` absent
- [ ] CHK-008 Tool commands vendor mode: TOOLS uses `node {BIN}/{pkg}/index.js` when vendored
- [ ] CHK-009 Tool commands dev mode: TOOLS uses `{BIN}/{name}` when in dev mode
- [ ] CHK-010 Dependency classification: ccusage packages are in `devDependencies`, not `dependencies`

## Behavioral Correctness

- [ ] CHK-011 Bundled execution: Running `dist/tu.mjs` resolves BIN to `dist/vendor/`
- [ ] CHK-012 Source execution: Running via `tsx src/node/core/cli.ts` resolves BIN to `node_modules/.bin/`

## Scenario Coverage

- [ ] CHK-013 Build produces vendor directory: All three `index.js` files exist after build
- [ ] CHK-014 Stale file cleanup: Previous version chunk files are removed on rebuild
- [ ] CHK-015 npm pack includes vendor: `dist/vendor/` is covered by existing `files` entry

## Edge Cases & Error Handling

- [ ] CHK-016 Missing vendor dir: When `vendor/` absent, fallback to `node_modules/.bin/` works without error
- [ ] CHK-017 Existing `_rootDir` walk-up logic still functions for dev-mode fallback

## Code Quality

- [ ] CHK-018 Pattern consistency: New code follows existing `fetcher.ts` style (functional, `node:` imports, no classes)
- [ ] CHK-019 No unnecessary duplication: Reuses existing `__dirname` and `_rootDir` variables
- [ ] CHK-020 Readability: Vendor resolution logic is clear and minimal
- [ ] CHK-021 No god functions: No function exceeds 50 lines

## Notes

- Check items as you review: `- [x]`
- All items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] CHK-008 **N/A**: {reason}`
