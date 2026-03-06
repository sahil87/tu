# Tasks: Reorganize src/ into Node Namespace

**Change**: 260306-x861-reorganize-src-node-namespace
**Spec**: `spec.md`
**Intake**: `intake.md`

## Phase 1: Setup

<!-- Create target directory structure -->

- [x] T001 Create directory scaffold: `src/node/core/`, `src/node/tui/`, `src/node/sync/`, `src/node/scripts/`, `src/node/core/__tests__/`, `src/node/tui/__tests__/`, `src/node/sync/__tests__/`

## Phase 2: Core Implementation

<!-- Move files and update imports. Ordered: standalone modules first, then modules with cross-dir imports. -->

- [x] T002 [P] Move `src/types.ts` to `src/node/core/types.ts` (no import changes — standalone)
- [x] T003 [P] Move `src/config.ts` to `src/node/core/config.ts` (no import changes — only node builtins)
- [x] T004 [P] Move `src/colors.ts` to `src/node/tui/colors.ts` (no import changes — standalone)
- [x] T005 [P] Move `src/fetcher.ts` to `src/node/core/fetcher.ts`; update import `./types.js` — stays `./types.js` (same dir, no change needed)
- [x] T006 [P] Move `src/sparkline.ts` to `src/node/tui/sparkline.ts`; update import `./colors.js` — stays `./colors.js` (same dir, no change needed)
- [x] T007 [P] Move `src/rain.ts` to `src/node/tui/rain.ts`; update import `./colors.js` — stays `./colors.js` (same dir, no change needed)
- [x] T008 [P] Move `src/formatter.ts` to `src/node/tui/formatter.ts`; update imports: `./types.js` → `../core/types.js`, `./colors.js` stays `./colors.js`
- [x] T009 [P] Move `src/panel.ts` to `src/node/tui/panel.ts`; update imports: `./types.js` → `../core/types.js`, `./sparkline.js` and `./colors.js` stay same
- [x] T010 [P] Move `src/compositor.ts` to `src/node/tui/compositor.ts`; update imports: `./types.js` → `../core/types.js`, `./panel.js`/`./rain.js`/`./colors.js` stay same
- [x] T011 [P] Move `src/watch.ts` to `src/node/tui/watch.ts`; update imports: `./types.js` → `../core/types.js`, `./formatter.js`/`./panel.js`/`./compositor.js`/`./colors.js` stay same
- [x] T012 Move `src/sync.ts` to `src/node/sync/sync.ts`; update imports: `./types.js` → `../core/types.js`, `./config.js` → `../core/config.js`, `./fetcher.js` → `../core/fetcher.js`
- [x] T013 Move `src/cli.ts` to `src/node/core/cli.ts`; update imports: `./formatter.js` → `../tui/formatter.js`, `./watch.js` → `../tui/watch.js`, `./colors.js` → `../tui/colors.js`, `./sync.js` → `../sync/sync.js`, `./fetcher.js`/`./config.js`/`./types.js` stay same
- [x] T014 Move `src/scripts/release.sh` to `src/node/scripts/release.sh`

## Phase 3: Test Co-location

<!-- Move test files to __tests__/ directories and update imports -->

- [x] T015 [P] Move core test files (13 files) from `tests/` to `src/node/core/__tests__/`: `cli-fresh-flag.test.ts`, `cli-help.test.ts`, `cli-init-conf.test.ts`, `cli-init-metrics.test.ts`, `cli-json.test.ts`, `cli-parser.test.ts`, `cli-status.test.ts`, `cli-sync.test.ts`, `cli-sync-flag.test.ts`, `cli-watch-flag.test.ts`, `config.test.ts`, `fetcher.test.ts`, `fetch-warning.test.ts`; update `../src/cli.js` → `../cli.js`, `../src/config.js` → `../config.js`, `../src/types.js` → `../types.js`, `../src/fetcher.js` → `../fetcher.js`, `../src/colors.js` → `../../tui/colors.js`
- [x] T016 [P] Move TUI test files (7 files) from `tests/` to `src/node/tui/__tests__/`: `colors.test.ts`, `formatter.test.ts`, `formatter-options.test.ts`, `panel.test.ts`, `rain.test.ts`, `sparkline.test.ts`, `watch.test.ts`; update `../src/formatter.js` → `../formatter.js`, `../src/colors.js` → `../colors.js`, `../src/types.js` → `../../core/types.js`, `../src/rain.js` → `../rain.js`, `../src/sparkline.js` → `../sparkline.js`, `../src/panel.js` → `../panel.js`, `../src/watch.js` → `../watch.js`
- [x] T017 [P] Move sync test file from `tests/sync.test.ts` to `src/node/sync/__tests__/sync.test.ts`; update `../src/sync.js` → `../sync.js`, `../src/types.js` → `../../core/types.js`
- [x] T018 Remove empty `tests/` directory

## Phase 4: Build Config & Verification

<!-- Update build configuration and verify everything works -->

- [x] T019 Update `justfile`: build entry point `src/cli.ts` → `src/node/core/cli.ts`, test glob `tests/*.test.ts` → `src/node/**/__tests__/*.test.ts`, release path `src/scripts/release.sh` → `src/node/scripts/release.sh`
- [x] T020 Update `package.json`: `scripts.build` entry point `src/cli.ts` → `src/node/core/cli.ts`, `scripts.test` glob `tests/*.test.ts` → `src/node/**/__tests__/*.test.ts`
- [x] T021 Remove leftover files: delete any remaining `.ts` files in `src/` root, delete `src/scripts/` if empty
- [x] T022 Run `npx tsc --noEmit` to verify TypeScript compilation succeeds with zero errors
- [x] T023 Run test suite (`npx tsx --test 'src/node/**/__tests__/*.test.ts'`) to verify all 21 test files pass
- [x] T024 Run build (`esbuild src/node/core/cli.ts --bundle --platform=node --format=esm --outfile=dist/tu.mjs --banner:js='#!/usr/bin/env node'`) to verify bundle generation succeeds

---

## Execution Order

- T001 (scaffold) blocks all Phase 2 tasks
- T002-T007 (standalone files) are independent — can run in parallel
- T008-T011 (cross-dir TUI files) depend on T002 (types) and T004 (colors) being in place
- T012 (sync) depends on T002 (types), T003 (config), T005 (fetcher)
- T013 (cli) depends on T005 (fetcher), T008 (formatter), T011 (watch), T012 (sync), T004 (colors)
- T015-T017 (test moves) depend on their respective source files being in place (Phase 2 complete)
- T018 (remove tests/) depends on T015-T017
- T019-T020 (build config) are independent of file moves
- T021-T024 (verification) depend on all prior tasks
