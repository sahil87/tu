# Intake: Filter Usage by User

**Change**: 260307-v2bu-filter-usage-by-user
**Created**: 2026-03-07
**Status**: Draft

## Origin

> Check if multiple users' usage is getting mixed in the output. Multiple machines should get added, but not multiple users! I want to see "my" usage. Lets say I do want to see someone else's usage, maybe add a -u command?

One-shot input. The user identified that multi-mode output currently aggregates data from all users in the metrics repo rather than scoping to the current user's machines only.

## Why

In multi-machine mode, the purpose is to aggregate your own usage across your different machines (e.g., laptop + desktop). However, `readRemoteEntries()` in `src/node/sync/sync.ts` iterates over **all** user directories in the metrics repo and only skips the exact `user+machine` pair (line 112: `if (userDir === user && machineDir === machine) continue`). This means:

- If `sahil` has machines `macbook` and `desktop`, and `bob` also has data in the repo, running `tu` as `sahil` on `macbook` will include:
  - `sahil/2026/desktop/*` (correct — your other machine)
  - `bob/2026/*` (wrong — another user's data)

This inflates cost/token totals with other people's usage, making the tool unreliable for personal cost tracking. Since multi-mode is specifically for "cross-machine aggregation" (per the constitution and memory), mixing users is a bug, not a feature.

Without a fix, anyone using a shared metrics repo sees everyone's combined costs, defeating the purpose of per-user cost tracking.

## What Changes

### 1. Fix `readRemoteEntries()` to scope to current user only

In `src/node/sync/sync.ts`, change `readRemoteEntries()` so it only reads from the current user's directory by default. The current logic:

```typescript
// Current: skips only own user+machine
if (userDir === user && machineDir === machine) continue;
```

Should become (conceptually):

```typescript
// Fixed: only read own user's other machines
if (userDir !== user) continue;
if (machineDir === machine) continue;
```

This ensures multi-mode aggregates across the current user's machines only. The outer loop can be eliminated entirely — just iterate `{metricsDir}/{user}/{year}/*/` and skip the local machine.

### 2. Add `-u <username>` flag to view another user's usage

Add a new global flag `-u <username>` (or `--user <username>`) to the CLI grammar. When provided:

- Override the "target user" for `readRemoteEntries()` so it reads from `<username>`'s directory instead of the current user's
- Local data (fetched via `ccusage`) still represents the current machine, so when `-u` is set, local data should be **excluded** — show only the target user's metrics repo data
- This flag only applies in multi mode; in single mode, warn and ignore

Example usage:
```
tu -u bob          # Today's cost for bob across all his machines
tu -u bob cc mh    # Bob's monthly Claude Code history
```

### 3. Update CLI help text

Add `-u <user>` to the flags table in `FULL_HELP` and `SHORT_USAGE` as appropriate.

## Affected Memory

- `sync/multi-machine`: (modify) Update requirements for user-scoped reading and `-u` flag
- `cli/data-pipeline`: (modify) Add `-u` flag to CLI grammar and flags

## Impact

- **`src/node/sync/sync.ts`**: `readRemoteEntries()` signature and filtering logic
- **`src/node/core/cli.ts`**: flag parsing (`parseGlobalFlags`), dispatch functions (pass target user), help text
- **`src/node/core/config.ts`**: No changes expected (user field already exists)
- **Tests**: `src/node/sync/__tests__/sync.test.ts` — existing tests for `readRemoteEntries` will need updating (currently test cross-user reads as expected behavior)

## Open Questions

- None — the scope is clear from the description and codebase analysis.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Fix scopes `readRemoteEntries` to current user only | Directly stated by user: "Multiple machines should get added, but not multiple users!" | S:95 R:70 A:95 D:95 |
| 2 | Certain | New flag is `-u <username>` | User said "add a -u command" — explicit flag choice | S:90 R:90 A:90 D:95 |
| 3 | Confident | `-u` also supports `--user` long form | Consistent with existing flag conventions (`-f`/`--fresh`, `-w`/`--watch`) | S:50 R:90 A:85 D:80 |
| 4 | Confident | `-u` in single mode warns and is ignored | Single mode has no metrics repo to read from; graceful degradation per constitution | S:40 R:90 A:90 D:85 |
| 5 | Confident | When `-u` is set, exclude local `ccusage` data | Local data is always the current user's; showing it alongside a different user's metrics data would be misleading | S:60 R:75 A:80 D:70 |
| 6 | Certain | `writeMetrics` and `syncMetrics` remain unchanged | These write the current user's data — unrelated to the read-side filtering bug | S:80 R:95 A:95 D:95 |
| 7 | Confident | `-u` works with `--watch` mode | No reason to exclude it; watch mode already supports all flags except `--json` | S:40 R:85 A:80 D:85 |

7 assumptions (3 certain, 4 confident, 0 tentative, 0 unresolved).
