# Intake: Add Copilot Source

**Change**: 260403-etml-add-copilot-source
**Created**: 2026-04-03
**Status**: Draft

## Origin

> User asked during a `/fab-discuss` session whether `tu` measures Copilot CLI tokens. The answer was no — only Claude Code (`cc`), Codex (`codex`/`co`), and OpenCode (`oc`) are supported. User requested adding GitHub Copilot CLI as a fourth data source with source token `cp` and a `ccusage-copilot` binary, following the exact same pattern as the existing tools.

## Why

`tu` aggregates cost/usage data across AI coding assistants, but currently omits GitHub Copilot — one of the most widely used AI coding tools. Users who use Copilot alongside Claude Code or Codex have no way to see their Copilot usage in the same dashboard. Adding Copilot as a data source fills this gap and makes `tu` a more complete cost-tracking tool.

Without this change, Copilot users must track that tool's usage separately, losing the benefit of `tu`'s unified aggregation, caching, multi-machine sync, and watch mode.

## What Changes

### New tool config entry in `TOOLS` registry

Add a `cp` key to the `TOOLS` object in `src/node/core/fetcher.ts`:

```typescript
cp: { name: "Copilot", command: useVendor ? `node ${BIN}/ccusage-copilot/index.js` : `${BIN}/ccusage-copilot`, needsFilter: true },
```

`needsFilter: true` follows the Codex/OpenCode pattern — `ccusage-copilot` output may contain noise lines starting with `[` that need stripping before JSON parsing.

### CLI source token and alias

In `src/node/core/cli.ts`, add `cp` to `KNOWN_SOURCES` and update help text:

- `KNOWN_SOURCES`: add `"cp"`
- `FULL_HELP` sources line: `Sources: cc (Claude Code), codex/co (Codex), oc (OpenCode), cp (Copilot), all (default)`
- `SHORT_USAGE`: no structural change needed, but examples could mention `tu cp`

### npm dependency

Add `@ccusage/copilot` (or equivalent package providing the `ccusage-copilot` binary) to `package.json` devDependencies, matching the pattern of existing `ccusage`, `@ccusage/codex`, `@ccusage/opencode` packages.

### Aggregation and dispatch

No changes needed — Copilot entries flow through the existing `TOOLS` registry. `Object.keys(TOOLS)` and `Object.entries(TOOLS)` already drive `fetchAllTotals`, `fetchAllHistory`, the `all`-source dispatch paths, multi-machine sync (`writeMetrics`/`readMetrics`), and watch mode. Adding a key to `TOOLS` automatically includes it everywhere.

### Cache

Automatically handled — the cache key is derived from the tool key (`cp-daily.json`).

### Tests

Update existing tests that assert on the `TOOLS` registry contents or `KNOWN_SOURCES` set to include the new `cp` entry.

## Affected Memory

- `cli/data-pipeline`: (modify) Add `cp` to the documented sources list and `TOOLS` registry description

## Impact

- **`src/node/core/fetcher.ts`**: New entry in `TOOLS` object
- **`src/node/core/cli.ts`**: `KNOWN_SOURCES` set, help text
- **`package.json`**: New devDependency for `ccusage-copilot`
- **All output tables**: Copilot will appear as a new column/row in snapshot and history tables when data is available
- **Multi-machine sync**: Copilot metrics files will be written to the metrics repo under `{user}/{year}/{machine}/cp-{date}.jsonl`

## Open Questions

- What is the exact npm package name for the `ccusage-copilot` binary? (Assuming `@ccusage/copilot` by pattern, but needs verification)

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Source token is `cp` | Discussed — user explicitly specified `cp` | S:95 R:90 A:95 D:95 |
| 2 | Certain | Binary name is `ccusage-copilot` | Discussed — user explicitly specified `ccusage-copilot` | S:95 R:90 A:95 D:95 |
| 3 | Certain | Included in `all` aggregate | Discussed — user explicitly requested inclusion in `all` | S:95 R:90 A:95 D:95 |
| 4 | Certain | Uses existing `UsageEntry`/`UsageTotals` data model | Constitution V mandates all data sources conform to these types | S:90 R:95 A:95 D:95 |
| 5 | Confident | `needsFilter: true` | Follows Codex/OpenCode pattern — ccusage-* tools tend to produce noise lines | S:60 R:90 A:70 D:65 |
| 6 | Confident | Display name is "Copilot" | Natural short name; matches the pattern of "Claude Code", "Codex", "OpenCode" | S:70 R:95 A:80 D:70 |
| 7 | Confident | npm package is `@ccusage/copilot` | Follows `@ccusage/codex` and `@ccusage/opencode` naming pattern | S:50 R:85 A:70 D:60 |

7 assumptions (4 certain, 3 confident, 0 tentative, 0 unresolved).
