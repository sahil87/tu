# Quality Checklist: Relicense MIT & Migrate to sahil87

**Change**: 260401-lomt-relicense-mit-migrate-sahil87
**Generated**: 2026-04-01
**Spec**: `spec.md`

## Functional Completeness
- [x] CHK-001 MIT License File: `LICENSE` contains MIT text with "Copyright (c) 2026 Sahil Ahuja"
- [x] CHK-002 Package License Field: `package.json` `license` is `"MIT"`
- [x] CHK-003 CLI Update Message: `src/node/core/cli.ts` references `sahil87/tap/tu`
- [x] CHK-004 README Install: Install section uses `brew tap sahil87/tap` with no SSH URL
- [x] CHK-005 Context File: `fab/project/context.md` references `sahil87/tap` and `MIT`
- [x] CHK-006 Toolchain Memory: `docs/memory/build/toolchain.md` uses sahil87 org, MIT license, no SSH note

## Behavioral Correctness
- [x] CHK-007 No wvrdz in update message: `src/node/core/cli.ts` has no remaining `wvrdz/tap` reference in the update instruction
- [x] CHK-008 No SSH requirement in README: `README.md` install section has no SSH URL or wvrdz reference

## Removal Verification
- [x] CHK-009 PolyForm license removed: `LICENSE` contains no PolyForm text
- [x] CHK-010 Old brew tap removed: No `wvrdz/tap` in `README.md` install section

## Scenario Coverage
- [x] CHK-011 **N/A**: `tu.default.weaver.conf` was already removed by change 260401-jufw-remove-weaver-dev-derive-mode; no weaver config to preserve
- [x] CHK-012 Config test preserved: `src/node/core/__tests__/config.test.ts` exists and is unmodified
- [x] CHK-013 Historical artifacts preserved: No changes to files under `fab/changes/` outside current change folder

## Edge Cases & Error Handling
- [x] CHK-014 No stray wvrdz references in changed files: grep confirmed zero wvrdz hits in src/, docs/memory/, fab/project/

## Code Quality
- [x] CHK-015 Pattern consistency: Changes follow existing file formatting and style
- [x] CHK-016 No unnecessary duplication: No redundant references introduced

## Notes

- Check items as you review: `- [x]`
- All items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] CHK-008 **N/A**: {reason}`
