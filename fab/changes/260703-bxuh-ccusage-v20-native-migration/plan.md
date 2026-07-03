# Plan: ccusage v20 Native Migration

**Change**: 260703-bxuh-ccusage-v20-native-migration
**Intake**: `intake.md`

## Requirements

### Dependencies: package.json consolidation

#### R1: Single ccusage@20 dependency
The project MUST depend on exactly one ccusage package, `ccusage` at `^20.0.14`, and MUST NOT depend on `@ccusage/codex` or `@ccusage/opencode` (both deprecated upstream, final release 19.0.0). The `engines.node >= 18` floor MUST remain unchanged. The committed `package-lock.json` MUST be regenerated so brew and CI installs stay reproducible.

- **GIVEN** `package.json` `devDependencies`
- **WHEN** the migration is applied
- **THEN** `ccusage` is `^20.0.14`, `@ccusage/codex` and `@ccusage/opencode` are absent, `engines.node` is still `>=18`
- **AND** `package-lock.json` resolves `ccusage@20.x` with its per-platform `optionalDependencies`

### Build: native binary vendoring

#### R2: build.sh vendors the host-platform native ccusage binary
`scripts/build.sh` MUST replace the three JS-copy blocks (`cp node_modules/*/dist/*.js`) with a single step that maps the host platform to the ccusage native package name (`@ccusage/ccusage-${process.platform}-${process.arch}`), copies its `bin/ccusage` into `dist/vendor/ccusage/bin/ccusage`, and sets mode `0755`. The step MUST fail loud (exit 1 with a diagnostic on stderr) when the platform package is absent. The esbuild step and its `--define` metadata embedding MUST remain unchanged.

- **GIVEN** a machine with `ccusage@20` and its optional native dep installed (linux/darwin × x64/arm64)
- **WHEN** `scripts/build.sh` runs
- **THEN** `dist/vendor/ccusage/bin/ccusage` exists, is mode 0755, and is the native Rust executable
- **AND** `dist/tu.mjs` is still produced with the same `--define` values
- **GIVEN** a machine where the platform native package is not installed
- **WHEN** `scripts/build.sh` runs
- **THEN** it prints `error: {pkg} not installed ...` to stderr and exits 1 (no partial `dist/vendor`)

### CLI: single-binary tool registry with subcommand grammar

#### R3: TOOLS registry uses one ccusage binary with per-tool subcommand prefixes
`src/node/core/fetcher.ts` `TOOLS` MUST map all three tool keys (`cc`, `codex`, `oc`) to a single ccusage binary — the vendored native binary `${BIN}/ccusage/bin/ccusage` in vendor mode (exec'd directly, no `node` interpreter), or `${BIN}/ccusage` (the npm launcher) in dev mode. Per-tool subcommands MUST be expressed via the existing `prefixArgs` mechanism: `cc → []`, `codex → ["codex"]`, `oc → ["opencode"]`. `runTool`'s argv composition (`[...prefixArgs, period, "--json", ...extraArgs]`) MUST be unchanged. `needsFilter` MUST stay `false` for `cc` and `true` for `codex`/`oc` (retained defensively pending real-output verification).

- **GIVEN** vendor mode (`dist/vendor/` present)
- **WHEN** `runTool("codex", "daily")` executes
- **THEN** it execs `${vendor}/ccusage/bin/ccusage` with argv `["codex", "daily", "--json"]` (no `node` interpreter prepended)
- **GIVEN** dev mode (no vendor dir)
- **WHEN** `runTool("cc", "daily")` executes
- **THEN** it execs `node_modules/.bin/ccusage` with argv `["daily", "--json"]`
- **AND** `ToolConfig` shape (`{name, binary, prefixArgs, needsFilter}`) is unchanged, so `cli.ts`/`sync.ts` consumers (`Object.keys(TOOLS)`, `TOOLS[k].name`) keep working

### Data pipeline: parse contract holds against real v20 output

#### R4: v20 JSON parses through the existing parser contract, verified against live data
The parser contract (`parsed["daily"]`, the label key, `totalCost|costUSD`, `cachedInputTokens`→`cacheReadTokens` fallbacks, `normalizeLabel`, `toUsageTotals`, `pickCurrentEntry`) MUST correctly parse real `ccusage@20` output. Apply MUST verify this against live local `~/.claude` data for all three tools (`ccusage daily --json`, `ccusage codex daily --json`, `ccusage opencode daily --json`) before finalizing. Real drift was observed and resolved: v20 emits the ISO label under `period` (both daily and monthly), replacing v18's `date`/`month` keys, so `LABEL_KEY` was updated to map both periods to `"period"`; `toUsageTotals`/`parseJson` needed no change (token/cost field names are unchanged).

- **GIVEN** live `~/.claude` (and codex/opencode) history on this machine
- **WHEN** the dev launcher and the vendored binary emit `--json` for daily
- **THEN** the output parses through `parseJson` + `toUsageTotals` into non-degenerate `UsageEntry[]` with ISO labels and numeric costs
- **AND** any observed drift is recorded and fixed in the parser; absent drift, the parser is left unchanged

<!-- apply finding: real drift observed. ccusage@20 emits the ISO label under
     `period` for BOTH daily ("2026-07-03") and monthly ("2026-07") entries,
     replacing v18's human-readable `date`/`month` fields. Fix: LABEL_KEY now
     maps daily→"period" and monthly→"period"; fetchHistory's toUsageEntry call
     uses LABEL_KEY.daily. normalizeLabel passes ISO labels through unchanged, so
     no regex change was needed. All token/cost field names (totalCost,
     inputTokens, outputTokens, cacheCreationTokens, cacheReadTokens, totalTokens)
     are unchanged — toUsageTotals/parseJson untouched; the costUSD/cachedInputTokens
     fallbacks are retained (codex still emits costUSD in its totals block). v20
     stdout is clean JSON (no `[...]` noise lines), so stripNoise is a no-op now
     but kept defensively per plan. -->
<!-- assumed: LABEL_KEY monthly→"period" (verified against live `ccusage monthly --json`: entry label is under `period` as "2026-07") -->


### Tests: registry-shape coverage tracks the reshape

#### R5: fetcher tests assert the single-binary registry shape
`src/node/core/__tests__/fetcher.test.ts` MUST be updated so the `TOOLS` describe block asserts the new single-binary convention (vendor mode: `binary` ends with `ccusage/bin/ccusage`, `prefixArgs` per tool; dev mode: `binary` ends with `ccusage`, `prefixArgs` per tool) instead of the old `binary === "node"` / `index.js`-prefix convention. The parser tests (`toUsageTotals`, `normalizeLabel`, `pickCurrentEntry`, `stripNoise`, `mergeEntries`, `maxMergeEntries`, `aggregateMonthly`) MUST remain unchanged. The full suite (`npm test`) MUST pass.

- **GIVEN** the reshaped `TOOLS`
- **WHEN** `npm test` runs
- **THEN** the `TOOLS` describe block passes against the new shape, no test still asserts `binary === "node"` or an `index.js` prefix, and all other tests remain green

### Non-Goals

- **`.github/workflows/release.yml`** — untouched; the release is tag-anchored with no prebuilt platform artifacts.
- **`sahil87/homebrew-tap` `Formula/tu.rb`** — no change expected; exec-bit survival is a ship-time verification with a one-line documented fallback, not part of this change.
- **`~/.tu/cache`** — no migration; cached payloads are already-parsed `UsageEntry[]`, shape unchanged.
- **win32 vendor path** — out of scope (Homebrew-only distribution); Windows dev mode still works via the npm launcher.
- **Removing `needsFilter`/`stripNoise`** — retained defensively; removal is a separate cleanup once proven unnecessary.

### Design Decisions

1. **Single vendored native binary, exec'd directly**: `dist/vendor/ccusage/bin/ccusage` is the Rust executable, run without a `node` interpreter in vendor mode — *Why*: v20 ships no JS impl; the main package is a launcher over a per-platform Rust binary. *Rejected*: v19.0.3 JS stopgap (lands on an equally dead line, forces a second migration).
2. **Platform selection via npm `optionalDependencies` at brew source-build time**: no per-platform release artifacts — *Why*: `Formula/tu.rb` runs `npm install` on the user's machine, so npm installs the host's native package automatically. *Rejected*: shipping prebuilt bottles per platform (unnecessary complexity).
3. **Fail-loud build guard**: build-time absence of the platform package exits 1 — *Why*: Constitution II (graceful degradation) governs runtime data sources, not the build; a missing binary at build time is a hard error.

## Tasks

### Phase 1: Setup

- [x] T001 Update `package.json` `devDependencies`: bump `ccusage` to `^20.0.14`, remove `@ccusage/codex` and `@ccusage/opencode`; keep `engines.node >=18`. Then run `npm install` to regenerate `package-lock.json` and pull the v20 packages + host optional dep. <!-- R1 -->

### Phase 2: Core Implementation

- [x] T002 Rewrite the vendor step in `scripts/build.sh`: replace the three `mkdir`/`cp *.js` blocks with the host-platform native-binary vendoring (map `@ccusage/ccusage-${platform}-${arch}`, copy `bin/ccusage` → `dist/vendor/ccusage/bin/ccusage`, `chmod 0755`, fail-loud when absent). Leave the esbuild `--define` block untouched. <!-- R2 -->
- [x] T003 Reshape `TOOLS` in `src/node/core/fetcher.ts` to a single `CCUSAGE` binary (`${BIN}/ccusage/bin/ccusage` vendor / `${BIN}/ccusage` dev) with `prefixArgs` `[]`/`["codex"]`/`["opencode"]` and unchanged `needsFilter`; leave `runTool`, `ToolConfig`, and all parser/merge functions unchanged. <!-- R3 -->

### Phase 3: Integration & Edge Cases

- [x] T004 Update the `TOOLS` describe block in `src/node/core/__tests__/fetcher.test.ts` to assert the single-binary shape (vendor: `binary` ends with `ccusage/bin/ccusage`; dev: ends with `ccusage`; `prefixArgs` per tool; no `binary === "node"` / no `index.js` prefix). Leave parser tests unchanged. Run `npx tsx --test src/node/core/__tests__/fetcher.test.ts` and fix failures. <!-- R5 -->
- [x] T005 Verify against live data (dev mode): run `node_modules/.bin/ccusage daily --json`, `... codex daily --json`, `... opencode daily --json`; confirm each parses through `parseJson`/`toUsageTotals` with ISO labels and numeric costs. Record findings; adjust `toUsageTotals`/`parseJson` only on observed drift. <!-- R4 -->
- [x] T006 Verify the vendored binary path (vendor mode): run `scripts/build.sh`, then exec `dist/vendor/ccusage/bin/ccusage daily --json` directly and confirm parseable output; smoke `node dist/tu.mjs --version` and a basic `node dist/tu.mjs cc` run. <!-- R2 R4 -->

### Phase 4: Polish

- [x] T007 Run the full suite `npm test` and confirm green. <!-- R5 -->

## Execution Order

- T001 blocks everything (installs v20 + native dep).
- T002 and T003 are independent of each other but both need T001.
- T004 needs T003; T005 needs T001; T006 needs T001+T002+T003.
- T007 last (needs T003, T004).

## Acceptance

### Functional Completeness

- [x] A-001 R1: `package.json` depends only on `ccusage@^20.0.14` (no `@ccusage/codex`/`@ccusage/opencode`), `engines.node` is `>=18`, and `package-lock.json` is regenerated for v20.
- [x] A-002 R2: `scripts/build.sh` vendors `dist/vendor/ccusage/bin/ccusage` (mode 0755) via the platform-package mapping, with the esbuild `--define` block intact.
- [x] A-003 R3: `TOOLS` maps `cc`/`codex`/`oc` to a single ccusage binary with `prefixArgs` `[]`/`["codex"]`/`["opencode"]`; vendor mode execs the native binary directly (no `node`), dev mode uses `${BIN}/ccusage`.
- [x] A-004 R4: v20 `--json` output for all three tools parses through the existing contract into valid `UsageEntry[]`, verified against live local data.
- [x] A-005 R5: `fetcher.test.ts` asserts the single-binary registry shape.

### Behavioral Correctness

- [x] A-006 R3: `runTool` still composes `[...prefixArgs, period, "--json", ...extraArgs]` and `cli.ts`/`sync.ts` consumers of `TOOLS` (keys + `.name`) are unaffected.
- [x] A-007 R2: with the platform native package absent, `scripts/build.sh` fails loud (stderr diagnostic, exit 1) and leaves no partial vendor dir.

### Scenario Coverage

- [x] A-008 R4: the vendored binary (`dist/vendor/ccusage/bin/ccusage daily --json`) and `node dist/tu.mjs` both produce real, parseable data (end-to-end smoke).
- [x] A-009 R5: `npm test` passes in full.

### Removal Verification

- [x] A-010 R1: no source, script, or test still references `@ccusage/codex`, `@ccusage/opencode`, `ccusage-codex`, `ccusage-opencode`, or the `dist/*.js` vendoring.

### Code Quality

- [x] A-011 Pattern consistency: new code follows the surrounding functional style, `node:`-prefixed imports, and existing vendor-first `BIN` resolution.
- [x] A-012 No unnecessary duplication: the change reuses the existing `prefixArgs`/`runTool`/`BIN` machinery rather than adding parallel paths.
- [x] A-013 Graceful degradation: runtime fetch failures still warn on stderr and fall back to zero data (Constitution II); the new fail-loud guard is build-time only.

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Target `ccusage@^20.0.14`; drop `@ccusage/codex`/`@ccusage/opencode`; use built-in `codex`/`opencode` subcommands | Intake Assumption 1 (user explicitly chose v20; standalone packages deprecated at final 19.0.0) | S:95 R:70 A:95 D:95 |
| 2 | Certain | Vendor step maps `@ccusage/ccusage-${platform}-${arch}`, copies `bin/ccusage`, chmod 0755, fail-loud when absent | Intake §build.sh gives the exact script verbatim; platform set = Homebrew's darwin/linux × x64/arm64 | S:90 R:80 A:90 D:90 |
| 3 | Certain | win32 excluded from vendor path (dev mode still works via npm launcher) | Intake Assumption 3; distribution is Homebrew-only | S:75 R:90 A:90 D:85 |
| 4 | Confident | Vendor layout `dist/vendor/ccusage/bin/ccusage` exec'd directly; dev mode `${BIN}/ccusage`; reuse `prefixArgs`/`runTool` unchanged | Intake §fetcher.ts gives the exact TOOLS shape; mirrors upstream `bin/ccusage` layout | S:75 R:80 A:80 D:75 |
| 5 | Confident | Keep `needsFilter`/`stripNoise` defensively for codex/oc | Intake Assumption 7; harmless if v20 emits clean stdout, removal is a separate cleanup | S:55 R:90 A:75 D:70 |
| 6 | Confident | v20 JSON parses with the existing contract; verify against live data before finalizing, adjust parser only on drift | Intake Assumption 6; upstream states v19/v20 output compatibility, verification is an apply task | S:65 R:75 A:70 D:70 |
| 7 | Confident | No `ToolConfig` type change needed; `cli.ts`/`sync.ts` only use `Object.keys(TOOLS)` + `.name` | Verified by grep — no consumer touches `binary`/`prefixArgs` outside `fetcher.ts` | S:80 R:85 A:85 D:80 |
| 8 | Confident | Caret range `^20.0.14` (not exact pin); committed lockfile keeps installs reproducible | Intake Assumption 8; matches existing `^18.0.8` convention and dependabot flow | S:50 R:85 A:60 D:55 |

8 assumptions (3 certain, 5 confident, 0 tentative).
