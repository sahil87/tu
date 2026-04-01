# Spec: Vendor ccusage Binaries for Single-Bundle Distribution

**Change**: 260320-eu2i-vendor-ccusage-binaries
**Created**: 2026-04-01
**Affected memory**: `docs/memory/build/toolchain.md`

## Non-Goals

- Homebrew formula update — separate repo (`sahil87/homebrew-tap`), downstream beneficiary only
- Programmatic API migration — option A was evaluated and rejected in favor of vendoring
- Modifying ccusage packages themselves — they are consumed as-is

## Build: Vendor Copy

### Requirement: Vendor copy step

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

#### Scenario: Build produces vendor directory

- **GIVEN** ccusage packages are installed as devDependencies
- **WHEN** `npm run build` executes
- **THEN** `dist/vendor/ccusage/index.js` exists
- **AND** `dist/vendor/ccusage-codex/index.js` exists
- **AND** `dist/vendor/ccusage-opencode/index.js` exists
- **AND** all `.js` chunk files from each package's `dist/` are copied

#### Scenario: Chunk files are included

- **GIVEN** `ccusage` dist contains chunk files like `data-loader-sVkn4Ind.js`
- **WHEN** the vendor copy step runs
- **THEN** all `.js` files (not just `index.js`) are present in `dist/vendor/ccusage/`

### Requirement: Build script integration

The `package.json` `build` script and the `justfile` `build` target MUST both include the vendor copy step after the esbuild command. The vendor copy SHALL use shell commands (mkdir + cp with glob) rather than a separate script.

The vendor copy step SHOULD remove and recreate `dist/vendor/` before copying to prevent stale chunk files from lingering after ccusage version upgrades.
<!-- clarified: clean-before-copy prevents stale chunk files from previous ccusage versions -->

#### Scenario: Build script runs vendor copy

- **GIVEN** the project is freshly cloned with dependencies installed
- **WHEN** `npm run build` is executed
- **THEN** `dist/tu.mjs` is created by esbuild
- **AND** `dist/vendor/` is populated with all three ccusage package files

#### Scenario: Justfile build runs vendor copy

- **GIVEN** the project is freshly cloned with dependencies installed
- **WHEN** `just build` is executed
- **THEN** `dist/tu.mjs` is created by esbuild
- **AND** `dist/vendor/` is populated with all three ccusage package files
<!-- clarified: added justfile scenario to match requirement that both build targets include vendor copy -->

#### Scenario: Clean build removes stale vendor files

- **GIVEN** `dist/vendor/ccusage/` contains a stale chunk file `data-loader-OLD.js` from a previous ccusage version
- **WHEN** `npm run build` is executed
- **THEN** `dist/vendor/ccusage/data-loader-OLD.js` no longer exists
- **AND** only the current ccusage `.js` files are present
<!-- clarified: stale file cleanup scenario for version upgrades -->

## CLI: Binary Resolution

### Requirement: Vendor-first BIN resolution

The `BIN` constant in `src/node/core/fetcher.ts` MUST resolve to `vendor/` relative to the script's own location using `import.meta.url`. When `vendor/` does not exist (development mode), it SHALL fall back to `node_modules/.bin/` relative to the project root.

The existing `__dirname` derivation (`dirname(fileURLToPath(import.meta.url))`) and `_rootDir` walk-up logic SHALL be retained. The new resolution adds a vendor check between them:

```typescript
const vendorDir = join(__dirname, "vendor");
const useVendor = existsSync(vendorDir);
const BIN = useVendor ? vendorDir : join(_rootDir, "node_modules", ".bin");
```
<!-- clarified: noted that __dirname already exists in fetcher.ts via import.meta.url; no new import needed -->

#### Scenario: Bundled execution (production)

- **GIVEN** `dist/tu.mjs` is running and `dist/vendor/` exists
- **WHEN** `import.meta.url` resolves to `dist/tu.mjs`
- **THEN** `__dirname` is `dist/`
- **AND** `vendorDir` is `dist/vendor/` which exists
- **AND** `BIN` resolves to `dist/vendor/`

#### Scenario: Source execution (development)

- **GIVEN** the CLI is run via `tsx src/node/core/cli.ts`
- **WHEN** `import.meta.url` resolves to `src/node/core/fetcher.ts`
- **THEN** `vendorDir` is `src/node/core/vendor/` which does not exist
- **AND** `BIN` falls back to `{_rootDir}/node_modules/.bin/`

### Requirement: Tool invocation commands

The `TOOLS` record MUST construct command strings based on the resolution mode:

- **Vendor mode**: `node {BIN}/{package}/index.js` — explicit `node` invocation of the vendored entry point
- **Dev mode**: `{BIN}/{bin-name}` — direct invocation of the `node_modules/.bin/` stub

The package-to-bin name mapping:

| Package source | Vendor dir name | Bin stub name |
|----------------|----------------|---------------|
| `ccusage` | `ccusage` | `ccusage` |
| `@ccusage/codex` | `ccusage-codex` | `ccusage-codex` |
| `@ccusage/opencode` | `ccusage-opencode` | `ccusage-opencode` |

#### Scenario: Vendor mode tool execution

- **GIVEN** `useVendor` is `true` and `BIN` is `dist/vendor/`
- **WHEN** the `cc` tool is invoked
- **THEN** the command is `node dist/vendor/ccusage/index.js daily --json`

#### Scenario: Dev mode tool execution

- **GIVEN** `useVendor` is `false` and `BIN` is `{root}/node_modules/.bin/`
- **WHEN** the `cc` tool is invoked
- **THEN** the command is `{root}/node_modules/.bin/ccusage daily --json`

## Package: Dependency Classification

### Requirement: Move ccusage to devDependencies

`ccusage`, `@ccusage/codex`, and `@ccusage/opencode` MUST be moved from `dependencies` to `devDependencies` in `package.json`. They are only needed at build time for the vendor copy step.

#### Scenario: Production install excludes ccusage

- **GIVEN** the package is installed via `npm install --omit=dev`
- **WHEN** the install completes
- **THEN** `node_modules/ccusage/` does not exist
- **AND** `dist/vendor/` contains the vendored binaries (from the build step)

### Requirement: Published files include vendor

The `files` array in `package.json` already includes `dist/`. No changes needed — `dist/vendor/` is covered by the existing `dist/` entry.

#### Scenario: npm pack includes vendor

- **GIVEN** the build has been run and `dist/vendor/` is populated
- **WHEN** `npm pack` is executed
- **THEN** the tarball includes `dist/vendor/ccusage/index.js` and all chunk files

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Vendor all three ccusage packages | Confirmed from intake #1 — user chose Option B over A/C/D | S:95 R:85 A:90 D:95 |
| 2 | Certain | Copy entire dist/ directories including chunk files | Confirmed from intake #2 — ccusage imports sibling chunks at runtime | S:95 R:90 A:95 D:95 |
| 3 | Certain | All three packages have zero runtime dependencies | Confirmed from intake #3 — verified `"dependencies": {}` in all three | S:95 R:90 A:95 D:95 |
| 4 | Certain | Move ccusage packages from dependencies to devDependencies | Confirmed from intake #6 — only needed at build time for copy step | S:90 R:90 A:90 D:95 |
| 5 | Confident | Invoke vendored scripts via `node <path>/index.js` | Confirmed from intake #4 — vendored files lack executable permissions; explicit node invocation is cross-platform reliable | S:75 R:85 A:80 D:75 |
| 6 | Confident | Use `import.meta.url` for vendor directory resolution | Confirmed from intake #5 — standard ESM mechanism, esbuild preserves it for platform=node | S:80 R:80 A:85 D:85 |
| 7 | Confident | Dev-mode fallback to `node_modules/.bin/` when vendor/ absent | New — `just run` and tests use `tsx` from source where vendor/ doesn't exist; fallback preserves dev workflow | S:70 R:85 A:80 D:75 |
| 8 | Confident | Vendor dir names match bin names (ccusage, ccusage-codex, ccusage-opencode) | New — consistent naming between vendor dirs and node_modules/.bin/ stubs simplifies the mapping | S:75 R:90 A:80 D:80 |
| 9 | Confident | Homebrew formula update is out-of-scope | Confirmed from intake #7 — separate repo, downstream beneficiary | S:80 R:90 A:70 D:80 |
| 10 | Confident | Use inline shell commands for vendor copy (not a separate script) | New — single build step with mkdir+cp is simpler than a dedicated script for a one-liner operation | S:65 R:90 A:75 D:70 |

10 assumptions (4 certain, 6 confident, 0 tentative, 0 unresolved).
