# Quality Checklist: Filter Usage by User

**Change**: 260307-v2bu-filter-usage-by-user
**Generated**: 2026-03-07
**Spec**: `spec.md`

## Functional Completeness

- [ ] CHK-001 User-scoped reading: `readRemoteEntries` reads only from the target user's directory, not all users
- [ ] CHK-002 Exclude machine: `readRemoteEntries` skips `excludeMachine` when non-null
- [ ] CHK-003 -u flag parsed: `parseGlobalFlags` extracts `-u`/`--user` value into `userFlag`
- [ ] CHK-004 Local data excluded: when `-u` is set, local ccusage data is not included in output
- [ ] CHK-005 Help text updated: `FULL_HELP` includes `-u`/`--user` flag documentation

## Behavioral Correctness

- [ ] CHK-006 Default behavior preserved: without `-u`, multi-mode still merges local + own remote entries (no other users)
- [ ] CHK-007 Single-mode unchanged: without `-u`, single-mode behavior is identical to before

## Scenario Coverage

- [ ] CHK-008 Default read: own user's other machines returned, own machine and other users excluded
- [ ] CHK-009 Target user read: all of target user's machines returned when excludeMachine is null
- [ ] CHK-010 Target user no data: empty array returned for nonexistent user
- [ ] CHK-011 -u single mode: warning on stderr, command proceeds without filter
- [ ] CHK-012 -u without value: error message and exit code 1
- [ ] CHK-013 -u with watch: watch mode works with -u flag

## Edge Cases & Error Handling

- [ ] CHK-014 -u with own username: shows repo-only data (no local merge), consistent with other -u usage
- [ ] CHK-015 Empty metrics dir: returns empty array gracefully
- [ ] CHK-016 -u with --sync: sync runs for current user, display shows target user

## Code Quality

- [ ] CHK-017 Pattern consistency: new flag parsing follows existing patterns (`-f`/`--fresh`, `-w`/`--watch`)
- [ ] CHK-018 No unnecessary duplication: dispatch function changes reuse existing patterns
- [ ] CHK-019 Readability: function signature change is clear (targetUser + excludeMachine)
- [ ] CHK-020 No magic strings: warning/error messages are descriptive
- [ ] CHK-021 Functional style: no classes introduced, plain functions used
- [ ] CHK-022 Stderr for warnings: all warnings/errors go to stderr per constitution

## Notes

- Check items as you review: `- [x]`
- All items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] CHK-008 **N/A**: {reason}`
