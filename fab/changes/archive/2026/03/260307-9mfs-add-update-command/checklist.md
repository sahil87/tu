# Quality Checklist: Add Self-Update Command

**Change**: 260307-9mfs-add-update-command
**Generated**: 2026-03-07
**Spec**: `spec.md`

## Functional Completeness
- [x] CHK-001 Command Dispatch: `tu update` dispatched as non-data command before grammar parsing
- [x] CHK-002 Homebrew Detection: `_pkgDir.includes("/Cellar/tu/")` correctly identifies brew installs
- [x] CHK-003 Non-Homebrew Message: prints version + suggestion message and exits 0
- [x] CHK-004 Homebrew Update Flow: runs brew update, checks version, upgrades if needed
- [x] CHK-005 Already Up To Date: prints `Already up to date (v{version}).` when versions match
- [x] CHK-006 Help Text: `tu update` appears in FULL_HELP Setup section
- [x] CHK-007 Test Coverage: cli-help.test.ts includes `tu update` assertion

## Behavioral Correctness
- [x] CHK-008 Version display: `Current version: v{PKG_VERSION}` printed before brew operations
- [x] CHK-009 Update message: `Updating v{current} → v{latest}...` shown when versions differ
- [x] CHK-010 Completion message: `Updated to v{latest}.` printed after successful upgrade

## Scenario Coverage
- [x] CHK-011 Non-Homebrew install scenario: message + exit 0 (no crash)
- [x] CHK-012 Already up to date scenario: early return after version comparison
- [x] CHK-013 Update available scenario: brew upgrade runs with inherited stdio

## Edge Cases & Error Handling
- [x] CHK-014 brew update failure: prints specific error to stderr + exit 1
- [x] CHK-015 brew info failure: prints specific error to stderr + exit 1
- [x] CHK-016 brew upgrade failure: prints specific error to stderr + exit 1

## Code Quality
- [x] CHK-017 Pattern consistency: `runUpdate` follows naming/structure of `runStatus`, `runInitConf`
- [x] CHK-018 No unnecessary duplication: reuses `PKG_VERSION`, `execSync` already imported
- [x] CHK-019 Readability: function is clear, no god-function (reasonable length)
- [x] CHK-020 No magic strings: timeouts and paths are explicit constants or inline with clear context

## Notes

- Check items as you review: `- [x]`
- All items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] CHK-008 **N/A**: {reason}`
