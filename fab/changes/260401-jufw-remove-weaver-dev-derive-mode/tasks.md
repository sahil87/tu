# Tasks: Remove WEAVER_DEV and derive mode from metrics_repo

**Change**: 260401-jufw-remove-weaver-dev-derive-mode
**Spec**: `spec.md`
**Intake**: `intake.md`

## Phase 1: Config File Changes

- [x] T001 [P] Remove `mode = single` line from `tu.default.conf`; keep `metrics_repo` commented out
- [x] T002 [P] Delete `tu.default.weaver.conf`

## Phase 2: Core Implementation

- [x] T003 In `src/node/core/config.ts`: remove `WEAVER_DEV` env var branching — `DEFAULT_CONFIG_PATH` always resolves to `tu.default.conf`
- [x] T004 In `src/node/core/config.ts` `readConfig()`: add `TU_METRICS_REPO` env var override (non-empty takes precedence over config file), derive `mode` from `metricsRepo !== ""`, remove `mode=multi without metrics_repo` warning
- [x] T005 In `src/node/core/cli.ts`: remove `mode` entry from `FIELD_BLOCKS`, update `metrics_repo` comment to mention `TU_METRICS_REPO`

## Phase 3: Error Message Updates

- [x] T006 [P] In `src/node/core/cli.ts` `runInitMetrics()`: update error message to reference `metrics_repo`/`TU_METRICS_REPO` instead of `mode=multi`
- [x] T007 [P] In `src/node/core/cli.ts` `runSync()`: update error message to reference `metrics_repo`/`TU_METRICS_REPO` instead of `mode=multi`

## Phase 4: Tests

- [x] T008 Update `STOCK_DEFAULTS` in `src/node/core/__tests__/config.test.ts` to remove `mode = single`
- [x] T009 Update existing tests: remove `WEAVER_DEV` branching in "works with the real default conf" test, update "returns single mode when user sets mode=single" to verify mode is now driven by metrics_repo, update "falls back to single mode when mode=multi but metrics_repo is missing" to reflect no warning
- [x] T010 Add new test: `TU_METRICS_REPO` env var overrides config file `metrics_repo` and produces multi mode
- [x] T011 Add new test: empty `TU_METRICS_REPO` does not override config file value

---

## Execution Order

- T001, T002 are independent (parallel)
- T003 before T004 (DEFAULT_CONFIG_PATH must be simplified before readConfig changes)
- T005 depends on T004 (FIELD_BLOCKS update references the new config behavior)
- T006, T007 are independent (parallel), depend on T004
- T008 before T009-T011 (STOCK_DEFAULTS used by all tests)
