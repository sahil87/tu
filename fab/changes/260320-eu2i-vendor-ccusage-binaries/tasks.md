# Tasks: Vendor ccusage Binaries for Single-Bundle Distribution

**Change**: 260320-eu2i-vendor-ccusage-binaries
**Spec**: `spec.md`
**Intake**: `intake.md`

## Phase 1: Setup

- [x] T001 [P] Move `ccusage`, `@ccusage/codex`, `@ccusage/opencode` from `dependencies` to `devDependencies` in `package.json`
- [x] T002 [P] Add vendor copy step to `package.json` `build` script — after esbuild, clean `dist/vendor/`, mkdir vendor subdirs, cp `*.js` from each package's `dist/` per source-to-dest mapping in spec
- [x] T003 [P] Add vendor copy step to `justfile` `build` target — same commands as T002, after the esbuild line

## Phase 2: Core Implementation

- [x] T004 Update BIN resolution in `src/node/core/fetcher.ts` — add `vendorDir` check using existing `__dirname`, set `useVendor` flag, fall back to `node_modules/.bin/` via existing `_rootDir` when vendor absent
- [x] T005 Update `TOOLS` record in `src/node/core/fetcher.ts` — vendor mode uses `node {BIN}/{pkg}/index.js`, dev mode uses `{BIN}/{name}` directly. The old `const BIN = join(_rootDir, "node_modules", ".bin")` line is replaced by T004's dual-mode resolution (the `_rootDir` walk-up logic itself is retained for the dev-mode fallback)
<!-- clarified: clarified that _rootDir walk-up is retained; only the single-path BIN constant is replaced by the vendor-first logic from T004 -->

## Phase 3: Integration & Edge Cases

- [x] T006 Run `npm run build` and verify `dist/vendor/` structure matches spec (all three subdirs with index.js and chunk files)
- [x] T007 Run `npm test` to verify existing tests pass with the new BIN resolution logic

---

## Execution Order

- T001, T002, T003 are independent (parallel)
- T004 is independent of T001–T003 (it modifies fetcher.ts source code; devDependencies are still installed by `npm install` regardless of classification)
<!-- clarified: T004 has no real dependency on T001 — moving packages between deps/devDeps doesn't affect source edits or npm install in dev -->
- T005 depends on T004 (TOOLS depends on the new BIN/useVendor variables)
- T006 depends on T002, T003 (build scripts must be updated)
- T007 depends on T004, T005 (fetcher changes must be in place)
