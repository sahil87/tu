# Intake: Remove WEAVER_DEV and derive mode from metrics_repo

**Change**: 260401-jufw-remove-weaver-dev-derive-mode
**Created**: 2026-04-01
**Status**: Draft

## Origin

> User described this change with full implementation decisions pre-made from a prior discussion. The input specifies exactly which files change, what the new runtime behavior is, and which config artifacts are deleted. This is a detailed, decision-complete request.

## Why

The `WEAVER_DEV` env var and `tu.default.weaver.conf` file exist solely to configure the wvrdz team's multi-mode setup. This leaks a project-specific concern (the wvrdz metrics repo URL) into the generic codebase. Additionally, having `mode` as an explicit config field creates a redundancy: `mode=multi` is meaningless without `metrics_repo`, and `metrics_repo` being set implies multi mode. Deriving `mode` from `metrics_repo` presence eliminates both the redundancy and the project-specific config machinery, making the codebase fully generic.

Without this change, any fork or external user encounters a confusing `WEAVER_DEV` env var with no documentation, two defaults files with unclear selection logic, and a `mode` field that can desync from `metrics_repo`.

## What Changes

### 1. Remove `WEAVER_DEV` env var detection (`src/node/core/config.ts`)

The `DEFAULT_CONFIG_PATH` export currently branches on `process.env.WEAVER_DEV` (line 33) to select between `tu.default.conf` and `tu.default.weaver.conf`. This branching is removed entirely. `DEFAULT_CONFIG_PATH` always resolves to `tu.default.conf`:

```typescript
export const DEFAULT_CONFIG_PATH = resolve(_rootDir, "tu.default.conf");
```

### 2. Add `TU_METRICS_REPO` env var (`src/node/core/config.ts`)

In `readConfig()`, after merging defaults and user config, check `process.env.TU_METRICS_REPO`. If set, it overrides the config file's `metrics_repo` value:

```typescript
const metricsRepo = process.env.TU_METRICS_REPO || merged.metrics_repo || "";
```

This is the generic replacement for what `WEAVER_DEV` + `tu.default.weaver.conf` did. Any user or CI can set `TU_METRICS_REPO` to enable multi mode without editing config files.

### 3. Derive `mode` at runtime (`src/node/core/config.ts`)

Remove the explicit `mode` parsing from the merged config. Instead, derive it from `metricsRepo` presence:

```typescript
const mode: TuConfig["mode"] = metricsRepo !== "" ? "multi" : "single";
```

The `mode` field remains on the `TuConfig` interface so all downstream consumers (cli.ts, sync.ts, etc.) are unchanged. The `mode=` line in user config files is silently ignored (the parser still reads it but `readConfig()` no longer uses it).

The existing warning for `mode=multi` without `metrics_repo` is removed since that state is now impossible.

### 4. Delete `tu.default.weaver.conf`

This file is deleted entirely. There is only one defaults file (`tu.default.conf`).

### 5. Update `tu.default.conf`

Remove the `mode = single` line. The `metrics_repo` line remains commented out (single mode by default, since no `metrics_repo` means `mode=single`). Config version stays at 2.

### 6. Update `init-conf` scaffold (`src/node/core/cli.ts`)

Remove the `mode` entry from `FIELD_BLOCKS` so the generated `~/.tu.conf` template does not include a `mode` line. The remaining fields are unchanged.

### 7. Update `init-metrics` mode check (`src/node/core/cli.ts`)

The `runInitMetrics` function currently checks `config.mode !== "multi"` and errors. After this change, `mode` is derived from `metricsRepo`, so this check naturally works: if `metrics_repo` is set, `mode` will be `multi`. The explicit error message should be updated to reference `metrics_repo` instead of `mode=multi`.

### 8. Update `runSync` error message (`src/node/core/cli.ts`)

The sync error message currently says "set mode=multi". Update to reference `metrics_repo` or `TU_METRICS_REPO` instead.

### 9. Update tests (`src/node/core/__tests__/config.test.ts`)

- Remove `WEAVER_DEV` branching in the "works with the real default conf" test (line 244). After this change, the real default conf always produces `mode=single` with empty `metricsRepo`.
- Update tests that explicitly set `mode=multi` to verify that mode is now derived from `metrics_repo` presence rather than explicit `mode` field.
- Add a test for `TU_METRICS_REPO` env var override.
- Update the `STOCK_DEFAULTS` constant to remove the `mode = single` line.
- Update the test "returns single mode when user sets mode=single" to verify that `mode` in config is ignored (metrics_repo drives mode).
- Update the test "falls back to single mode when mode=multi but metrics_repo is missing" since the warning message changes (or is removed entirely).

## Affected Memory

- `configuration/config-system`: (modify) Remove WEAVER_DEV documentation, add TU_METRICS_REPO, document mode derivation from metrics_repo

## Impact

- **`src/node/core/config.ts`** — main changes: remove WEAVER_DEV branching, add TU_METRICS_REPO, derive mode from metricsRepo
- **`src/node/core/cli.ts`** — update FIELD_BLOCKS (remove mode), update error messages referencing mode=multi
- **`src/node/core/__tests__/config.test.ts`** — remove WEAVER_DEV branching, update mode derivation tests, add TU_METRICS_REPO test
- **`tu.default.conf`** — remove mode line
- **`tu.default.weaver.conf`** — delete entirely
- **No breaking changes to downstream consumers** — `TuConfig.mode` field and all its usages remain identical; only the derivation logic changes

## Open Questions

(none)

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Config version stays at 2 | Explicitly stated in the description; no config schema change, only behavior change | S:95 R:90 A:95 D:95 |
| 2 | Certain | `TuConfig` interface keeps `mode` field | Explicitly stated; all downstream consumers unchanged | S:95 R:85 A:95 D:95 |
| 3 | Certain | `mode` lines in existing user configs are silently ignored | Explicitly stated; parseConf still reads them but readConfig does not use the value | S:90 R:90 A:90 D:90 |
| 4 | Certain | `TU_METRICS_REPO` takes precedence over config file `metrics_repo` | Explicitly stated in the description | S:95 R:85 A:90 D:95 |
| 5 | Certain | `tu.default.weaver.conf` is deleted, not deprecated | Explicitly stated in the description | S:95 R:80 A:90 D:95 |
| 6 | Certain | `brew install wvrdz/tap/tu` hardcoding in cli.ts is out of scope | Explicitly stated in the description | S:95 R:95 A:95 D:95 |
| 7 | Confident | Error messages in `runInitMetrics` and `runSync` should reference `metrics_repo`/`TU_METRICS_REPO` instead of `mode=multi` | Logical consequence of removing mode from config; user can no longer "set mode=multi" | S:70 R:85 A:80 D:80 |

7 assumptions (6 certain, 1 confident, 0 tentative, 0 unresolved).
