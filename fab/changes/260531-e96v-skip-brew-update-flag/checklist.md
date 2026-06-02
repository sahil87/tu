# Quality Checklist: Add `--skip-brew-update` flag to `update` command

**Change**: 260531-e96v-skip-brew-update-flag
**Generated**: 2026-05-31
**Spec**: `spec.md`

## Functional Completeness
<!-- Every requirement in spec.md has working implementation -->
- [x] CHK-001 Flag gates only the refresh: `runUpdate` accepts a boolean param; when set, `execSync("brew update --quiet", ...)` is skipped and flow proceeds to `brew info`.
- [x] CHK-002 Version check / short-circuit / upgrade preserved: `brew info --json=v2 tu`, the `latest === PKG_VERSION` short-circuit, and `brew upgrade tu` are byte-for-byte unchanged from the pre-change code.
- [x] CHK-003 Dispatcher detection: the `update` dispatch passes `process.argv.includes("--skip-brew-update")` into `runUpdate`; flag NOT added to `parseGlobalFlags`/`GlobalFlags`.
- [x] CHK-004 Help text: `FULL_HELP` `Flags:` block contains a `--skip-brew-update` entry, column-aligned, scoped to `tu update`.
- [x] CHK-005 Test exists: `src/node/core/__tests__/cli-skip-brew-update-flag.test.ts` present, follows Node-runner + `cli-sync-flag.test.ts` inline-mirror pattern.

## Behavioral Correctness
<!-- Changed requirements behave as specified, not as before -->
- [x] CHK-006 Flag name is EXACTLY `--skip-brew-update` (no alias, no abbreviation) — matches cross-toolkit contract.
- [x] CHK-007 Default (flag absent) preserves current behavior exactly: `brew update --quiet` still runs when the flag is not passed (encoded via `skipBrewUpdate = false` default).
- [x] CHK-008 `runUpdate` remains callable with zero arguments (existing call sites/tests unbroken by the new default param).

## Removal Verification
<!-- Every deprecated requirement is actually gone -->
- [x] CHK-009 **N/A**: This change is purely additive — no requirements or code paths are removed.

## Scenario Coverage
<!-- Key scenarios from spec.md have been exercised -->
- [x] CHK-010 "Flag present skips refresh" + "Upgrade still runs when flag set": covered by the new test asserting skip=true omits `brew update` but keeps `brew upgrade tu`.
- [x] CHK-011 "Flag absent runs refresh": covered by the new test asserting skip=false includes both `brew update --quiet` and `brew upgrade tu`.
- [x] CHK-012 Flag-detection idiom: covered by the test asserting `includes("--skip-brew-update")` returns the correct boolean.

## Edge Cases & Error Handling
<!-- Error states, boundary conditions, failure modes -->
- [x] CHK-013 Non-Homebrew install guard unaffected: with the flag set, the `/Cellar/tu/` guard still returns early before any `brew` call.
- [x] CHK-014 Skipping `brew update` does not break the `brew info` failure path: the existing `try/catch` around `brew info` (errors → "could not determine latest version" → exit 1) is untouched and still reachable when the flag is set.

## Code Quality
<!-- From fab/project/code-quality.md principles + anti-patterns relevant to this change -->
- [x] CHK-015 Pattern consistency: flag detection uses the existing `rawArgs.includes(...)` / `process.argv.includes(...)` idiom; subprocess uses `execSync` (no deviation from convention).
- [x] CHK-016 No unnecessary duplication: reuses existing `execSync` import and the established flag-membership pattern; no parallel flag-parsing path introduced.
- [x] CHK-017 No magic strings without intent: the literal `"--skip-brew-update"` is the contract-mandated flag string used in dispatch (acceptable as a single-use literal, consistent with `"--sync"`/`"--fresh"` in `parseGlobalFlags`).
- [x] CHK-018 Minimum pathways: gating is a single boolean branch around one `execSync` call — no second code path duplicating the update flow.
- [x] CHK-019 No silently swallowed errors: the gated block's existing error handling (warn on stderr + exit) is preserved; skipping it does not hide a failure (the refresh simply isn't attempted).
- [x] CHK-020 Strict TypeScript: `npm run build` passes under strict mode; the new param is correctly typed `boolean` with a default.

## Security
<!-- Subprocess command construction -->
- [x] CHK-021 No command injection surface: the `brew update`/`brew info`/`brew upgrade` command strings remain static literals; the flag only toggles whether a fixed command runs — no user input is interpolated into any shell command.

## Notes

- Check items as you review: `- [x]`
- All items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] CHK-XXX **N/A**: {reason}`

<!-- Migrated to plan.md on 2026-06-02 — safe to delete. -->
