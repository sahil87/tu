# Quality Checklist: Add -v Version Shorthand

**Change**: 260401-kuuh-add-v-version-shorthand
**Generated**: 2026-04-01
**Spec**: `spec.md`

## Functional Completeness
- [x] CHK-001 `-v` flag prints version: Running `tu -v` outputs the package version string to stdout
- [x] CHK-002 `--version` unchanged: Running `tu --version` still prints version
- [x] CHK-003 `-V` unchanged: Running `tu -V` still prints version

## Behavioral Correctness
- [x] CHK-004 `-v` with other args: `tu -v cc` prints version and exits (version check fires first)

## Scenario Coverage
- [x] CHK-005 All three version flags produce identical output

## Code Quality
- [x] CHK-006 Pattern consistency: New condition follows the same `rawArgs.includes()` pattern
- [x] CHK-007 No unnecessary duplication: Single condition line, no helper abstraction

## Notes

- Check items as you review: `- [x]`
- All items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] CHK-008 **N/A**: {reason}`
