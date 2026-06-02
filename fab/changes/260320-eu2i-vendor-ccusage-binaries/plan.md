# Plan: Vendor ccusage Binaries for Single-Bundle Distribution

**Change**: 260320-eu2i-vendor-ccusage-binaries
**Status**: In Progress
**Intake**: `intake.md`
**Spec**: `spec.md`

## Requirements

<!-- migrated from spec.md on 2026-06-02 -->

### Non-Goals

- Homebrew formula update — separate repo (`sahil87/homebrew-tap`), downstream beneficiary only
- Programmatic API migration — option A was evaluated and rejected in favor of vendoring
- Modifying ccusage packages themselves — they are consumed as-is

### Build: Vendor Copy

#### Requirement: Vendor copy step

The build process MUST copy the `dist/` directory contents from all three ccusage packages into `dist/vendor/` subdirectories after esbuild completes.

The vendor directory structure SHALL be:

```
dist/
  tu.mjs
  vendor/
    ccusage/
      index.js
      data-loader-*.js
      calculate-cost-*.js
      _types-*.js
      logger-*.js
      prompt-*.js
      debug-*.js
    ccusage-codex/
      index.js
      prompt-*.js
    ccusage-opencode/
      index.js
      prompt-*.js
```

The copy step MUST glob all `.js` files from each source `dist/` directory (chunk filenames contain content hashes that change across versions).
<!-- clarified: explicit source-to-dest mapping added for scoped packages -->

The source-to-destination mapping SHALL be:

| npm package | Source path | Vendor dest |
|-------------|-------------|-------------|
| `ccusage` | `node_modules/ccusage/dist/*.js` | `dist/vendor/ccusage/` |
| `@ccusage/codex` | `node_modules/@ccusage/codex/dist/*.js` | `dist/vendor/ccusage-codex/` |
| `@ccusage/opencode` | `node_modules/@ccusage/opencode/dist/*.js` | `dist/vendor/ccusage-opencode/` |

The vendor subdirectory names SHALL match the bin command names: `ccusage`, `ccusage-codex`, `ccusage-opencode`.

##### Scenario: Build produces vendor directory

- **GIVEN** ccusage packages are installed as devDependencies
- **WHEN** `npm run build` executes
- **THEN** `dist/vendor/ccusage/index.js` exists
- **AND** `dist/vendor/ccusage-codex/index.js` exists
- **AND** `dist/vendor/ccusage-opencode/index.js` exists
- **AND** all `.js` chunk files from each package's `dist/` are copied

##### Scenario: Chunk files are included

- **GIVEN** `ccusage` dist contains chunk files like `data-loader-sVkn4Ind.js`
- **WHEN** the vendor copy step runs
- **THEN** all `.js` files (not just `index.js`) are present in `dist/vendor/ccusage/`

#### Requirement: Build script integration

The `package.json` `build` script and the `justfile` `build` target MUST both include the vendor copy step after the esbuild command. The vendor copy SHALL use shell commands (mkdir + cp with glob) rather than a separate script.

The vendor copy step SHOULD remove and recreate `dist/vendor/` before copying to prevent stale chunk files from lingering after ccusage version upgrades.
<!-- clarified: clean-before-copy prevents stale chunk files from previous ccusage versions -->

##### Scenario: Build script runs vendor copy

- **GIVEN** the project is freshly cloned with dependencies installed
- **WHEN** `npm run build` is executed
- **THEN** `dist/tu.mjs` is created by esbuild
- **AND** `dist/vendor/` is populated with all three ccusage package files

##### Scenario: Justfile build runs vendor copy

- **GIVEN** the project is freshly cloned with dependencies installed
- **WHEN** `just build` is executed
- **THEN** `dist/tu.mjs` is created by esbuild
- **AND** `dist/vendor/` is populated with all three ccusage package files
<!-- clarified: added justfile scenario to match requirement that both build targets include vendor copy -->

##### Scenario: Clean build removes stale vendor files

- **GIVEN** `dist/vendor/ccusage/` contains a stale chunk file `data-loader-OLD.js` from a previous ccusage version
- **WHEN** `npm run build` is executed
- **THEN** `dist/vendor/ccusage/data-loader-OLD.js` no longer exists
- **AND** only the current ccusage `.js` files are present
<!-- clarified: stale file cleanup scenario for version upgrades -->

### CLI: Binary Resolution

#### Requirement: Vendor-first BIN resolution

The `BIN` constant in `src/node/core/fetcher.ts` MUST resolve to `vendor/` relative to the script's own location using `import.meta.url`. When `vendor/` does not exist (development mode), it SHALL fall back to `node_modules/.bin/` relative to the project root.

The existing `__dirname` derivation (`dirname(fileURLToPath(import.meta.url))`) and `_rootDir` walk-up logic SHALL be retained. The new resolution adds a vendor check between them:

```typescript
const vendorDir = join(__dirname, "vendor");
const useVendor = existsSync(vendorDir);
const BIN = useVendor ? vendorDir : join(_rootDir, "node_modules", ".bin");
```
<!-- clarified: noted that __dirname already exists in fetcher.ts via import.meta.url; no new import needed -->

##### Scenario: Bundled execution (production)

- **GIVEN** `dist/tu.mjs` is running and `dist/vendor/` exists
- **WHEN** `import.meta.url` resolves to `dist/tu.mjs`
- **THEN** `__dirname` is `dist/`
- **AND** `vendorDir` is `dist/vendor/` which exists
- **AND** `BIN` resolves to `dist/vendor/`

##### Scenario: Source execution (development)

- **GIVEN** the CLI is run via `tsx src/node/core/cli.ts`
- **WHEN** `import.meta.url` resolves to `src/node/core/fetcher.ts`
- **THEN** `vendorDir` is `src/node/core/vendor/` which does not exist
- **AND** `BIN` falls back to `{_rootDir}/node_modules/.bin/`

#### Requirement: Tool invocation commands

The `TOOLS` record MUST construct command strings based on the resolution mode:

- **Vendor mode**: `node {BIN}/{package}/index.js` — explicit `node` invocation of the vendored entry point
- **Dev mode**: `{BIN}/{bin-name}` — direct invocation of the `node_modules/.bin/` stub

The package-to-bin name mapping:

| Package source | Vendor dir name | Bin stub name |
|----------------|----------------|---------------|
| `ccusage` | `ccusage` | `ccusage` |
| `@ccusage/codex` | `ccusage-codex` | `ccusage-codex` |
| `@ccusage/opencode` | `ccusage-opencode` | `ccusage-opencode` |

##### Scenario: Vendor mode tool execution

- **GIVEN** `useVendor` is `true` and `BIN` is `dist/vendor/`
- **WHEN** the `cc` tool is invoked
- **THEN** the command is `node dist/vendor/ccusage/index.js daily --json`

##### Scenario: Dev mode tool execution

- **GIVEN** `useVendor` is `false` and `BIN` is `{root}/node_modules/.bin/`
- **WHEN** the `cc` tool is invoked
- **THEN** the command is `{root}/node_modules/.bin/ccusage daily --json`

### Package: Dependency Classification

#### Requirement: Move ccusage to devDependencies

`ccusage`, `@ccusage/codex`, and `@ccusage/opencode` MUST be moved from `dependencies` to `devDependencies` in `package.json`. They are only needed at build time for the vendor copy step.

##### Scenario: Production install excludes ccusage

- **GIVEN** the package is installed via `npm install --omit=dev`
- **WHEN** the install completes
- **THEN** `node_modules/ccusage/` does not exist
- **AND** `dist/vendor/` contains the vendored binaries (from the build step)

#### Requirement: Published files include vendor

The `files` array in `package.json` already includes `dist/`. No changes needed — `dist/vendor/` is covered by the existing `dist/` entry.

##### Scenario: npm pack includes vendor

- **GIVEN** the build has been run and `dist/vendor/` is populated
- **WHEN** `npm pack` is executed
- **THEN** the tarball includes `dist/vendor/ccusage/index.js` and all chunk files

## Tasks

### Phase 1: Setup

- [x] T001 [P] Move `ccusage`, `@ccusage/codex`, `@ccusage/opencode` from `dependencies` to `devDependencies` in `package.json`
- [x] T002 [P] Add vendor copy step to `package.json` `build` script — after esbuild, clean `dist/vendor/`, mkdir vendor subdirs, cp `*.js` from each package's `dist/` per source-to-dest mapping in spec
- [x] T003 [P] Add vendor copy step to `justfile` `build` target — same commands as T002, after the esbuild line

### Phase 2: Core Implementation

- [x] T004 Update BIN resolution in `src/node/core/fetcher.ts` — add `vendorDir` check using existing `__dirname`, set `useVendor` flag, fall back to `node_modules/.bin/` via existing `_rootDir` when vendor absent
- [x] T005 Update `TOOLS` record in `src/node/core/fetcher.ts` — vendor mode uses `node {BIN}/{pkg}/index.js`, dev mode uses `{BIN}/{name}` directly. The old `const BIN = join(_rootDir, "node_modules", ".bin")` line is replaced by T004's dual-mode resolution (the `_rootDir` walk-up logic itself is retained for the dev-mode fallback)
<!-- clarified: clarified that _rootDir walk-up is retained; only the single-path BIN constant is replaced by the vendor-first logic from T004 -->

### Phase 3: Integration & Edge Cases

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

## Acceptance

### Functional Completeness

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

### Behavioral Correctness

- [ ] CHK-011 Bundled execution: Running `dist/tu.mjs` resolves BIN to `dist/vendor/`
- [ ] CHK-012 Source execution: Running via `tsx src/node/core/cli.ts` resolves BIN to `node_modules/.bin/`

### Scenario Coverage

- [ ] CHK-013 Build produces vendor directory: All three `index.js` files exist after build
- [ ] CHK-014 Stale file cleanup: Previous version chunk files are removed on rebuild
- [ ] CHK-015 npm pack includes vendor: `dist/vendor/` is covered by existing `files` entry

### Edge Cases & Error Handling

- [ ] CHK-016 Missing vendor dir: When `vendor/` absent, fallback to `node_modules/.bin/` works without error
- [ ] CHK-017 Existing `_rootDir` walk-up logic still functions for dev-mode fallback

### Code Quality

- [ ] CHK-018 Pattern consistency: New code follows existing `fetcher.ts` style (functional, `node:` imports, no classes)
- [ ] CHK-019 No unnecessary duplication: Reuses existing `__dirname` and `_rootDir` variables
- [ ] CHK-020 Readability: Vendor resolution logic is clear and minimal
- [ ] CHK-021 No god functions: No function exceeds 50 lines

### Notes

- Check items as you review: `- [x]`
- All items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] CHK-008 **N/A**: {reason}`
