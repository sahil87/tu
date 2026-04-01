# Quality Checklist: Remove WEAVER_DEV and derive mode from metrics_repo

**Change**: 260401-jufw-remove-weaver-dev-derive-mode
**Generated**: 2026-04-01
**Spec**: `spec.md`

## Functional Completeness
- [x] CHK-001 Mode derivation: `readConfig()` returns `mode=multi` when `metrics_repo` is set, `mode=single` when absent
- [x] CHK-002 TU_METRICS_REPO override: env var takes precedence over config file `metrics_repo`
- [x] CHK-003 Empty TU_METRICS_REPO: empty string does not override config file value
- [x] CHK-004 DEFAULT_CONFIG_PATH: always resolves to `tu.default.conf` unconditionally
- [x] CHK-005 init-conf scaffold: generated `~/.tu.conf` has no `mode` line
- [x] CHK-006 Error messages: `runSync` and `runInitMetrics` reference `metrics_repo`/`TU_METRICS_REPO`

## Behavioral Correctness
- [x] CHK-007 Mode field ignored: config files with `mode = single` + `metrics_repo` set produce `mode=multi`
- [x] CHK-008 Mode field ignored: config files with `mode = multi` + no `metrics_repo` produce `mode=single`
- [x] CHK-009 Backward compat: existing `~/.tu.conf` with `mode` lines loads without error
- [x] CHK-010 TuConfig interface: `mode` field still present, downstream consumers unchanged

## Removal Verification
- [x] CHK-011 WEAVER_DEV: no references to `WEAVER_DEV` in source code
- [x] CHK-012 tu.default.weaver.conf: file deleted from repository
- [x] CHK-013 mode in FIELD_BLOCKS: `mode` entry removed from init-conf scaffold
- [x] CHK-014 mode=multi warning: old "mode=multi but no metrics_repo" warning removed

## Scenario Coverage
- [x] CHK-015 Test: metrics_repo set → mode=multi
- [x] CHK-016 Test: no metrics_repo → mode=single
- [x] CHK-017 Test: TU_METRICS_REPO env var override
- [x] CHK-018 Test: empty TU_METRICS_REPO ignored
- [x] CHK-019 Test: real default conf (no WEAVER_DEV branching)

## Edge Cases & Error Handling
- [x] CHK-020 Both config files missing: graceful fallback to single mode
- [x] CHK-021 Config with only mode field (no metrics_repo): produces single mode, no crash

## Code Quality
- [x] CHK-022 Pattern consistency: new code follows functional style, `node:` prefixed imports
- [x] CHK-023 No unnecessary duplication: TU_METRICS_REPO check is a single point in readConfig
- [x] CHK-024 No dynamic import: no new `import()` calls introduced
- [x] CHK-025 Error paths warn on stderr: any new error/warning uses stderr

## Notes

- Check items as you review: `- [x]`
- All items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] CHK-008 **N/A**: {reason}`
