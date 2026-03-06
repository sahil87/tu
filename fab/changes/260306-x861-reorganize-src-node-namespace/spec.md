# Spec: Reorganize src/ into Node Namespace

**Change**: 260306-x861-reorganize-src-node-namespace
**Created**: 2026-03-06
**Affected memory**: `docs/memory/build/toolchain.md`, `docs/memory/cli/data-pipeline.md`, `docs/memory/display/formatting.md`, `docs/memory/watch-mode/tui.md`, `docs/memory/sync/multi-machine.md`

## Non-Goals

- Creating `src/rust/` or any Rust scaffolding — future-only, not part of this change
- Adding barrel/index files — the codebase uses direct imports, no re-export pattern exists
- Changing any runtime behavior — the compiled bundle (`dist/tu.mjs`) MUST be identical before and after

## Directory Structure

### Requirement: Node Namespace Directory Layout

The `src/` directory SHALL be reorganized into `src/node/` with three functional subdirectories (`core/`, `tui/`, `sync/`) plus a `scripts/` directory. The flat `src/*.ts` layout SHALL be eliminated.

#### Scenario: Target directory tree after reorganization

- **GIVEN** the reorganization is complete
- **WHEN** listing the `src/` directory tree
- **THEN** the structure MUST be:
  ```
  src/
  └── node/
      ├── core/
      │   ├── cli.ts
      │   ├── types.ts
      │   ├── config.ts
      │   ├── fetcher.ts
      │   └── __tests__/
      ├── tui/
      │   ├── formatter.ts
      │   ├── panel.ts
      │   ├── colors.ts
      │   ├── sparkline.ts
      │   ├── compositor.ts
      │   ├── watch.ts
      │   ├── rain.ts
      │   └── __tests__/
      ├── sync/
      │   ├── sync.ts
      │   └── __tests__/
      └── scripts/
          └── release.sh
  ```
- **AND** no `.ts` files SHALL remain directly in `src/`

#### Scenario: No leftover files in old locations

- **GIVEN** the reorganization is complete
- **WHEN** checking the old `src/` root and `tests/` directory
- **THEN** `src/` SHALL contain only the `node/` directory (no loose `.ts` files)
- **AND** the `tests/` directory SHALL be removed (all tests co-located)

### Requirement: Module Grouping

Files SHALL be grouped by functional domain:

- **`core/`**: Entry point and data pipeline — `cli.ts`, `types.ts`, `config.ts`, `fetcher.ts`
- **`tui/`**: Terminal UI and display — `formatter.ts`, `panel.ts`, `colors.ts`, `sparkline.ts`, `compositor.ts`, `watch.ts`, `rain.ts`
- **`sync/`**: Multi-machine sync — `sync.ts`
- **`scripts/`**: Build/release scripts — `release.sh`

#### Scenario: Core modules are data pipeline files

- **GIVEN** the `core/` directory
- **WHEN** listing its TypeScript files
- **THEN** it SHALL contain exactly: `cli.ts`, `types.ts`, `config.ts`, `fetcher.ts`

#### Scenario: TUI modules are display and watch files

- **GIVEN** the `tui/` directory
- **WHEN** listing its TypeScript files
- **THEN** it SHALL contain exactly: `formatter.ts`, `panel.ts`, `colors.ts`, `sparkline.ts`, `compositor.ts`, `watch.ts`, `rain.ts`

## Import Paths

### Requirement: Intra-Directory Imports Unchanged

Imports between files within the same subdirectory SHALL use `./` relative paths and remain unchanged from the current pattern.

#### Scenario: TUI-internal imports stay relative

- **GIVEN** `watch.ts` and `compositor.ts` are both in `tui/`
- **WHEN** `watch.ts` imports from `compositor.ts`
- **THEN** the import path SHALL be `./compositor.js`

### Requirement: Cross-Directory Import Rewrites

Imports between files in different subdirectories SHALL use `../` relative paths to traverse the directory structure. All imports MUST use `.js` extensions per constitution (NodeNext module resolution).

#### Scenario: Core importing from TUI

- **GIVEN** `cli.ts` is in `core/` and `formatter.ts` is in `tui/`
- **WHEN** `cli.ts` imports from `formatter.ts`
- **THEN** the import path SHALL be `../tui/formatter.js`

#### Scenario: Core importing from sync

- **GIVEN** `cli.ts` is in `core/` and `sync.ts` is in `sync/`
- **WHEN** `cli.ts` imports from `sync.ts`
- **THEN** the import path SHALL be `../sync/sync.js`

#### Scenario: TUI importing from core

- **GIVEN** `formatter.ts` is in `tui/` and `types.ts` is in `core/`
- **WHEN** `formatter.ts` imports from `types.ts`
- **THEN** the import path SHALL be `../core/types.js`

#### Scenario: Sync importing from core

- **GIVEN** `sync.ts` is in `sync/` and `config.ts` is in `core/`
- **WHEN** `sync.ts` imports from `config.ts`
- **THEN** the import path SHALL be `../core/config.js`

### Requirement: Complete Cross-Directory Import Map

The following cross-directory imports MUST be rewritten:

| Source File (new location) | Import Target | New Import Path |
|---|---|---|
| `core/cli.ts` | `tui/formatter.ts` | `../tui/formatter.js` |
| `core/cli.ts` | `tui/watch.ts` | `../tui/watch.js` |
| `core/cli.ts` | `tui/colors.ts` | `../tui/colors.js` |
| `core/cli.ts` | `sync/sync.ts` | `../sync/sync.js` |
| `tui/formatter.ts` | `core/types.ts` | `../core/types.js` |
| `tui/compositor.ts` | `core/types.ts` | `../core/types.js` |
| `tui/panel.ts` | `core/types.ts` | `../core/types.js` |
| `tui/watch.ts` | `core/types.ts` | `../core/types.js` |
| `sync/sync.ts` | `core/types.ts` | `../core/types.js` |
| `sync/sync.ts` | `core/config.ts` | `../core/config.js` |
| `sync/sync.ts` | `core/fetcher.ts` | `../core/fetcher.js` |

#### Scenario: All cross-directory imports resolve correctly

- **GIVEN** all files have been moved and imports rewritten per the table above
- **WHEN** running `npx tsc --noEmit`
- **THEN** TypeScript SHALL report zero errors

## Test Co-location

### Requirement: Tests in `__tests__/` Directories

Per the constitution's "Test Location" rule, all test files SHALL be co-located in `__tests__/` folders within the subdirectory of the module they test.

#### Scenario: Test file mapping

- **GIVEN** the current `tests/` directory with 21 test files
- **WHEN** the reorganization is complete
- **THEN** the test files SHALL be distributed as follows:

**`src/node/core/__tests__/`** (13 files):
- `cli-fresh-flag.test.ts`
- `cli-help.test.ts`
- `cli-init-conf.test.ts`
- `cli-init-metrics.test.ts`
- `cli-json.test.ts`
- `cli-parser.test.ts`
- `cli-status.test.ts`
- `cli-sync.test.ts`
- `cli-sync-flag.test.ts`
- `cli-watch-flag.test.ts`
- `config.test.ts`
- `fetcher.test.ts`
- `fetch-warning.test.ts`

**`src/node/tui/__tests__/`** (7 files):
- `colors.test.ts`
- `formatter.test.ts`
- `formatter-options.test.ts`
- `panel.test.ts`
- `rain.test.ts`
- `sparkline.test.ts`
- `watch.test.ts`

**`src/node/sync/__tests__/`** (1 file):
- `sync.test.ts`

### Requirement: Test Import Path Rewrites

Test imports SHALL be updated to reflect the new relative path from `__tests__/` to the source files.

#### Scenario: Test importing from same subdirectory

- **GIVEN** `formatter.test.ts` is in `tui/__tests__/` and `formatter.ts` is in `tui/`
- **WHEN** the test imports from `formatter.ts`
- **THEN** the import path SHALL be `../formatter.js` (was `../src/formatter.js`)

#### Scenario: Test importing from different subdirectory

- **GIVEN** `formatter.test.ts` is in `tui/__tests__/` and `types.ts` is in `core/`
- **WHEN** the test imports from `types.ts`
- **THEN** the import path SHALL be `../../core/types.js` (was `../src/types.js`)

#### Scenario: Core test importing from same subdirectory

- **GIVEN** `cli-init-conf.test.ts` is in `core/__tests__/` and `config.ts` is in `core/`
- **WHEN** the test imports from `config.ts`
- **THEN** the import path SHALL be `../config.js` (was `../src/config.js`)

#### Scenario: TUI test importing colors from same subdirectory

- **GIVEN** `rain.test.ts` is in `tui/__tests__/` and `colors.ts` is in `tui/`
- **WHEN** the test imports `setNoColor` from `colors.ts`
- **THEN** the import path SHALL be `../colors.js` (was `../src/colors.js`)

## Build Configuration

### Requirement: esbuild Entry Point Update

The esbuild bundle entry point SHALL be updated from `src/cli.ts` to `src/node/core/cli.ts` in all locations where it is configured.

#### Scenario: justfile build recipe

- **GIVEN** the `justfile` build recipe
- **WHEN** esbuild is invoked
- **THEN** the entry point SHALL be `src/node/core/cli.ts`

#### Scenario: package.json build script

- **GIVEN** the `package.json` `scripts.build` field
- **WHEN** esbuild is invoked
- **THEN** the entry point SHALL be `src/node/core/cli.ts`

### Requirement: Test Runner Glob Update

The test runner glob SHALL be updated to find test files in the new co-located `__tests__/` directories.

#### Scenario: justfile test recipe

- **GIVEN** the `justfile` test recipe
- **WHEN** the test runner is invoked
- **THEN** the glob SHALL be `src/node/**/__tests__/*.test.ts`

#### Scenario: package.json test script

- **GIVEN** the `package.json` `scripts.test` field
- **WHEN** the test runner is invoked
- **THEN** the glob SHALL be `src/node/**/__tests__/*.test.ts`

### Requirement: TypeScript Configuration Compatibility

The `tsconfig.json` SHALL remain compatible with the new directory structure without requiring changes to `include` or `rootDir`.

#### Scenario: tsconfig include covers new paths

- **GIVEN** `tsconfig.json` has `"include": ["src"]`
- **WHEN** TypeScript resolves files
- **THEN** all files under `src/node/` SHALL be included (since `src/node/` is under `src/`)

#### Scenario: tsconfig rootDir remains valid

- **GIVEN** `tsconfig.json` has `"rootDir": "src"`
- **WHEN** TypeScript compiles
- **THEN** the `rootDir` SHALL remain valid (the new structure is still under `src/`)

### Requirement: Release Script Path Update

The `justfile` release recipe SHALL update the path to `release.sh` from `src/scripts/release.sh` to `src/node/scripts/release.sh`.

#### Scenario: justfile release recipe

- **GIVEN** the `justfile` release recipe
- **WHEN** invoking the release script
- **THEN** the path SHALL be `src/node/scripts/release.sh`

## Bundle Output Stability

### Requirement: Identical Bundle Output

The compiled bundle (`dist/tu.mjs`) SHALL be functionally identical before and after the reorganization. This is a pure structural refactor with no runtime behavior changes.

#### Scenario: Build succeeds after reorganization

- **GIVEN** all files have been moved and imports updated
- **WHEN** running the build command
- **THEN** esbuild SHALL produce `dist/tu.mjs` without errors

#### Scenario: All tests pass after reorganization

- **GIVEN** all files and tests have been moved with updated imports
- **WHEN** running the test suite
- **THEN** all 21 test files SHALL pass with zero failures

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Use `src/node/` namespace | Confirmed from intake #1 — user agreed to shorter name | S:95 R:90 A:95 D:95 |
| 2 | Certain | Three subdirs: `core/`, `tui/`, `sync/` | Confirmed from intake #2 — user confirmed grouping | S:90 R:85 A:90 D:95 |
| 3 | Certain | `cli.ts`, `types.ts`, `config.ts`, `fetcher.ts` in `core/` | Confirmed from intake #3 — data pipeline modules | S:90 R:85 A:90 D:90 |
| 4 | Certain | All TUI/display modules in `tui/` | Confirmed from intake #4 — user confirmed | S:90 R:85 A:90 D:90 |
| 5 | Certain | `sync.ts` in `sync/` | Confirmed from intake #5 — distinct domain | S:85 R:85 A:90 D:90 |
| 6 | Certain | No `src/rust/` in this change | Confirmed from intake #6 — explicit exclusion | S:95 R:95 A:95 D:95 |
| 7 | Certain | Co-locate tests in `__tests__/` folders | Confirmed from intake #7 — constitution rule | S:95 R:80 A:95 D:95 |
| 8 | Confident | `scripts/release.sh` moves to `src/node/scripts/` | Confirmed from intake #8 — Node-build specific | S:75 R:85 A:70 D:80 |
| 9 | Certain | No barrel/index files | Confirmed from intake #9 — upgraded: codebase analysis confirms direct imports throughout | S:80 R:90 A:90 D:85 |
| 10 | Certain | `tsconfig.json` needs no `include`/`rootDir` changes | `include: ["src"]` and `rootDir: "src"` already cover the new nested structure | S:95 R:95 A:95 D:95 |
| 11 | Certain | `tests/` directory removed after co-location | All 21 test files mapped to `__tests__/` folders; no tests remain in old location | S:90 R:80 A:90 D:90 |

11 assumptions (10 certain, 1 confident, 0 tentative, 0 unresolved).
