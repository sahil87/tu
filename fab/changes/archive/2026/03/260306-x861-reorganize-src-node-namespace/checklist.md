# Quality Checklist: Reorganize src/ into Node Namespace

**Change**: 260306-x861-reorganize-src-node-namespace
**Generated**: 2026-03-06
**Spec**: `spec.md`

## Functional Completeness

- [x] CHK-001 Directory layout: `src/node/` contains exactly `core/`, `tui/`, `sync/`, `scripts/` subdirectories
- [x] CHK-002 Core grouping: `core/` contains exactly `cli.ts`, `types.ts`, `config.ts`, `fetcher.ts`
- [x] CHK-003 TUI grouping: `tui/` contains exactly `formatter.ts`, `panel.ts`, `colors.ts`, `sparkline.ts`, `compositor.ts`, `watch.ts`, `rain.ts`
- [x] CHK-004 Sync grouping: `sync/` contains exactly `sync.ts`
- [x] CHK-005 Scripts: `scripts/` contains `release.sh`
- [x] CHK-006 Cross-directory imports: all 11 cross-dir imports from spec table rewritten correctly
- [x] CHK-007 Intra-directory imports: imports between files in the same subdir use `./` paths
- [x] CHK-008 Test co-location: 13 core tests in `core/__tests__/`, 7 TUI tests in `tui/__tests__/`, 1 sync test in `sync/__tests__/`
- [x] CHK-009 Test import rewrites: all test `../src/` imports replaced with correct relative paths
- [x] CHK-010 esbuild entry point: updated to `src/node/core/cli.ts` in justfile and package.json
- [x] CHK-011 Test runner glob: updated to `src/node/**/__tests__/*.test.ts` in justfile and package.json
- [x] CHK-012 Release script path: updated to `src/node/scripts/release.sh` in justfile

## Behavioral Correctness

- [x] CHK-013 TypeScript compiles: `npx tsc --noEmit` reports zero errors
- [x] CHK-014 All tests pass: all 21 test files pass with zero failures
- [x] CHK-015 Build succeeds: esbuild produces `dist/tu.mjs` without errors

## Scenario Coverage

- [x] CHK-016 No leftover source files: no `.ts` files remain directly in `src/`
- [x] CHK-017 Old tests directory removed: `tests/` directory no longer exists
- [x] CHK-018 tsconfig compatibility: `include` and `rootDir` work without changes

## Edge Cases & Error Handling

- [x] CHK-019 `.js` extension in all imports: every import uses `.js` extension per NodeNext resolution
- [x] CHK-020 No broken type imports: all `import type` statements resolve correctly

## Code Quality

- [x] CHK-021 Pattern consistency: import style matches existing codebase patterns (relative paths, `.js` extensions, `node:` prefixed builtins)
- [x] CHK-022 No unnecessary duplication: no redundant barrel/index files introduced
- [x] CHK-023 Readability: directory grouping reflects functional domains per project context
- [x] CHK-024 No magic strings: no hardcoded paths that should reference the new structure

## Notes

- Check items as you review: `- [x]`
- All items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] CHK-008 **N/A**: {reason}`
