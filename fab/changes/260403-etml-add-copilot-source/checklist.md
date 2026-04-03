# Quality Checklist: Add Copilot Source

**Change**: 260403-etml-add-copilot-source
**Generated**: 2026-04-03
**Spec**: `spec.md`

## Functional Completeness
- [x] CHK-001 Copilot tool config: `TOOLS.cp` exists with correct `name`, `command`, and `needsFilter` fields
- [x] CHK-002 cp source token: `KNOWN_SOURCES` includes `"cp"` and `parseDataArgs(["cp"])` succeeds
- [x] CHK-003 Help text update: `FULL_HELP` sources line includes `cp (Copilot)`
- [x] CHK-004 npm dependency: `@ccusage/copilot` present in `package.json`

## Scenario Coverage
- [x] CHK-005 TOOLS registry contains cp entry: test asserts `TOOLS.cp` exists with expected values
- [x] CHK-006 Parse cp source: test asserts `parseDataArgs(["cp"])` returns correct DataArgs
- [x] CHK-007 Copilot included in all-tools iteration: `Object.keys(TOOLS)` includes `"cp"`
- [x] CHK-008 needsFilter true: test asserts `TOOLS.cp.needsFilter === true`

## Edge Cases & Error Handling
- [x] CHK-009 Graceful degradation: existing `execAsync` error handling warns on stderr and returns empty data when binary missing

## Code Quality
- [x] CHK-010 Pattern consistency: new TOOLS entry follows exact structure and naming of existing cc/codex/oc entries
- [x] CHK-011 No unnecessary duplication: no new helper functions or abstractions introduced

## Notes

- Check items as you review: `- [x]`
- All items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] CHK-008 **N/A**: {reason}`
