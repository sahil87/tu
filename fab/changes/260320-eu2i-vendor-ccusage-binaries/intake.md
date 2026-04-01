# Intake: Vendor ccusage Binaries for Single-Bundle Distribution

**Change**: 260320-eu2i-vendor-ccusage-binaries
**Created**: 2026-03-20
**Status**: Draft

## Origin

> Conversational — arose from a `/fab-discuss` session exploring TU's distribution model. User asked how TU is distributed, whether the TypeScript/ccusage dependency forced the language choice, and whether bundling as a single `.mjs` file is feasible to simplify Homebrew distribution.

Key discussion findings:
- TU does **not** import ccusage as a library — it shells out to `ccusage`, `ccusage-codex`, `ccusage-opencode` as CLI binaries via `exec()` from `node_modules/.bin/`
- ccusage does expose a programmatic API (`ccusage/data-loader`, `ccusage/calculate-cost`), but TU doesn't use it
- All three ccusage packages have **zero runtime dependencies** and ship as pre-bundled JS files
- User evaluated four options (A: programmatic API, B: vendor binaries, C: npm install in formula, D: PATH dependency) and chose **Option B: vendor the binaries**

## Why

1. **Distribution friction**: The current Homebrew formula must run `npm install` to populate `node_modules/.bin/` with the ccusage binaries. This adds complexity to the formula, slows install time, and requires npm tooling on the user's machine.
2. **If we don't fix it**: Homebrew distribution remains coupled to npm. Every install downloads all transitive deps just to get three pre-bundled CLI scripts. The constitution mandates "no `node_modules` required at install time" (Principle III), but the current implementation violates this because the ccusage binaries are only available via `node_modules/.bin/`.
3. **Why vendoring over alternatives**: Vendoring is the simplest approach — it preserves the existing CLI-to-CLI integration (no rewrite of `fetcher.ts`'s exec logic), works for all three tools (including `@ccusage/codex` and `@ccusage/opencode` which lack programmatic APIs), and produces a self-contained `dist/` directory.

## What Changes

### Build: Vendor copy step

Add a post-build step that copies the ccusage dist directories into `dist/vendor/`:

```
dist/
  tu.mjs                          # existing — the main bundle
  vendor/
    ccusage/
      index.js                    # ~129 KB
      data-loader-*.js            # chunk files
      calculate-cost-*.js
      _types-*.js
      logger-*.js
      prompt-*.js
      debug-*.js
    ccusage-codex/
      index.js                    # ~309 KB
      prompt-*.js
    ccusage-opencode/
      index.js                    # ~339 KB
      prompt-*.js
```

The chunk filenames contain content hashes (e.g., `data-loader-sVkn4Ind.js`) that change across versions. The copy step must glob all `.js` files from each package's `dist/` directory.

### fetcher.ts: Resolve BIN relative to script location

Change the `BIN` constant from:
```typescript
const BIN = join(_rootDir, "node_modules", ".bin");
```

To resolve relative to the script's own location using `import.meta.url`:
```typescript
const BIN = join(dirname(fileURLToPath(import.meta.url)), "vendor");
```

Update the `TOOLS` config to point to `node index.js` within each vendor subdirectory (since these are no longer symlinked bin stubs, they need to be invoked as `node <path>/index.js`):
```typescript
export const TOOLS: Record<string, ToolConfig> = {
  cc: { name: "Claude Code", command: `node ${BIN}/ccusage/index.js`, needsFilter: false },
  codex: { name: "Codex", command: `node ${BIN}/ccusage-codex/index.js`, needsFilter: true },
  oc: { name: "OpenCode", command: `node ${BIN}/ccusage-opencode/index.js`, needsFilter: true },
};
```

### package.json: Move ccusage to devDependencies

Move `ccusage`, `@ccusage/codex`, and `@ccusage/opencode` from `dependencies` to `devDependencies`. They are only needed at build time (to copy their dist files). Update the `build` script to include the vendor copy step.

### package.json: Update `files` field

Ensure `dist/vendor/` is included in the published files (it's already covered by `dist/`).

### Homebrew formula update

The formula in `sahil87/homebrew-tap` will need updating to remove any `npm install` step. This is out-of-scope for this change but is the downstream beneficiary.

## Affected Memory

- `build/toolchain`: (modify) Update to document vendor distribution model, new BIN resolution, and build step

## Impact

- **`src/node/core/fetcher.ts`** — BIN resolution and TOOLS command strings
- **`package.json`** — dependency classification, build script
- **`dist/` output** — new `vendor/` subdirectory (~1.2 MB total)
- **Homebrew formula** (out-of-scope) — downstream simplification

## Open Questions

None — the discussion session resolved all key decisions.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Vendor all three ccusage packages (ccusage, @ccusage/codex, @ccusage/opencode) | Discussed — user chose Option B over A/C/D | S:95 R:85 A:90 D:95 |
| 2 | Certain | Copy entire dist/ directories (not just index.js) because ccusage uses chunk imports | Verified by reading ccusage/dist/index.js — imports sibling chunks | S:95 R:90 A:95 D:95 |
| 3 | Certain | All three packages have zero runtime dependencies | Verified — each has `"dependencies": {}` and no nested node_modules | S:95 R:90 A:95 D:95 |
| 4 | Confident | Invoke vendored scripts via `node <path>/index.js` instead of direct execution | Bin stubs in node_modules/.bin/ are symlinks with shebangs; vendored copies need explicit node invocation to be reliable cross-platform | S:70 R:85 A:75 D:70 |
| 5 | Confident | Use `import.meta.url` for BIN resolution | Standard ESM way to get script location; works in bundled output since esbuild preserves import.meta.url for platform=node | S:75 R:80 A:80 D:85 |
| 6 | Certain | Move ccusage packages from dependencies to devDependencies | They're only needed at build time for the copy step | S:90 R:90 A:90 D:95 |
| 7 | Confident | Homebrew formula update is out-of-scope for this change | Separate repo (wvrdz/homebrew-tap); this change makes the formula simpler but doesn't modify it | S:80 R:90 A:70 D:80 |

7 assumptions (4 certain, 3 confident, 0 tentative, 0 unresolved).
