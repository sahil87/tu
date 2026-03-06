# Quality Checklist: Fix Watch Mode Display Bugs

**Change**: 260306-5hu8-fix-watch-mode-display-bugs
**Generated**: 2026-03-06
**Spec**: `spec.md`

## Functional Completeness
- [x] CHK-001 Tokens/min guard: `buildPanel()` suppresses Tokens/min when `pollHistory.length <= 1`
- [x] CHK-002 renderTotal guard: Total row + divider only rendered when >1 tool has `totalTokens > 0`
- [x] CHK-003 renderCompactSnapshot guard: Total row + divider only rendered when >1 tool has `totalTokens > 0`

## Behavioral Correctness
- [x] CHK-004 Tokens/min still appears after 2+ polls with `totalTokens > 0`
- [x] CHK-005 Total row still appears when multiple tools have data (no regression)
- [x] CHK-006 Rate/Proj. day stats unaffected (already guarded by `computeBurnRate`)

## Scenario Coverage
- [x] CHK-007 First render (single poll): Tokens/min absent, Elapsed present
- [x] CHK-008 After second poll: Tokens/min present with reasonable value
- [x] CHK-009 Single tool in renderTotal: no divider, no Total row
- [x] CHK-010 Multiple tools in renderTotal: divider + Total row present
- [x] CHK-011 Single tool in renderCompactSnapshot: no divider, no Total row
- [x] CHK-012 Multiple tools in renderCompactSnapshot: divider + Total row present

## Edge Cases & Error Handling
- [x] CHK-013 Zero tokens after multiple polls: Tokens/min suppressed (existing guard)
- [x] CHK-014 All tools zero tokens in renderTotal: "No usage" message (existing behavior)

## Code Quality
- [x] CHK-015 Pattern consistency: Guards follow same pattern as existing renderHistory/renderTotalHistory guards
- [x] CHK-016 No unnecessary duplication: Reuses existing counting logic where possible
- [x] CHK-017 Functional style: No classes introduced, follows function + plain object pattern
- [x] CHK-018 No god functions: Changes are surgical, no function exceeds 50 lines

## Notes

- Check items as you review: `- [x]`
- All items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] CHK-008 **N/A**: {reason}`
