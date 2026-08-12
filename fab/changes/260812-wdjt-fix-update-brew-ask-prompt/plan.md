# Plan: Fix `tu update` hang on Homebrew 6 ask-mode prompt

**Change**: 260812-wdjt-fix-update-brew-ask-prompt
**Intake**: `intake.md`

## Requirements

### CLI: Update

#### R1: Homebrew ask mode suppressed on the upgrade call
The `brew upgrade tu` call inside `runUpdate` (`src/node/core/cli.ts`) MUST carry `env: { ...process.env, HOMEBREW_NO_ASK: "1" }` in its `execSync` options, so Homebrew 6's default ask-mode prompt (`Do you want to proceed with the upgrade? [y/n]`) never blocks the update.

- **GIVEN** a Homebrew 6 install with a real TTY (both stdin and stdout TTYs)
- **WHEN** `tu update` runs an out-of-date upgrade
- **THEN** `brew upgrade tu` executes without prompting for y/n confirmation
- **AND** the full inherited environment (PATH, user HOMEBREW_* settings) is preserved via the `process.env` spread — only `HOMEBREW_NO_ASK` is added

#### R2: Brew-safety posture preserved
The `brew upgrade tu` call MUST keep `stdio: "inherit"` (upgrade progress visible, Ctrl-C works) and MUST NOT gain a `timeout` option (toolkit `update` standard: killing brew mid-keg-swap corrupts the install). On Homebrew < 6, the unrecognized `HOMEBREW_NO_ASK` env var MUST be harmlessly ignored (behavior unchanged).

- **GIVEN** the modified `execSync("brew upgrade tu", ...)` call site
- **WHEN** the source is inspected (or run on any Homebrew version)
- **THEN** the call carries `stdio: "inherit"` and `env: { ...process.env, HOMEBREW_NO_ASK: "1" }`, and no `timeout` key
- **AND** on Homebrew < 6 the env var is ignored and behavior is byte-identical to before

#### R3: Adjacent comment documents the ask-mode rationale
The comment above the `brew upgrade tu` call MUST be extended to note why `HOMEBREW_NO_ASK` is set (Homebrew 6 ask-mode default) and why the env var is used over the `--no-ask` flag (cross-version safety — the flag errors on Homebrew < 6).

- **GIVEN** the `runUpdate` function in `src/node/core/cli.ts`
- **WHEN** a reader inspects the comment above the `brew upgrade tu` call
- **THEN** it explains both the existing no-timeout posture and the new ask-mode suppression rationale

#### R4: Out-of-scope surfaces untouched
`brew update --quiet` (600s timeout) and `brew info --json=v2 tu` (60s timeout) calls MUST stay byte-identical. The `--skip-brew-update` contract MUST be preserved (literal substring in `tu update --help` output; flag skips only the internal `brew update`). Exit-code contract MUST be unchanged: 0 on success (including already-up-to-date and non-Homebrew installs), 1 only on genuine brew failure.

- **GIVEN** the modified `runUpdate`
- **WHEN** the existing test suite (`cli-update-help.test.ts`, `cli-skip-brew-update-flag.test.ts`) runs
- **THEN** all pre-existing pins still pass unchanged

#### R5: New posture pinned by a co-located source-level test
A test in `src/node/core/__tests__/` MUST pin the new posture following the existing source-level-pin precedent (statically-imported `execSync` is not interceptable under ESM/tsx, per the constraint documented in `cli-update-help.test.ts` and `cli-skip-brew-update-flag.test.ts`): the `brew upgrade tu` call site carries `HOMEBREW_NO_ASK: "1"` in its `env` option and still carries `stdio: "inherit"`.

- **GIVEN** the modified `src/node/core/cli.ts`
- **WHEN** the co-located test suite runs via `npx tsx --test`
- **THEN** the pin test passes, and would fail if the `env`/`HOMEBREW_NO_ASK` option were removed or `stdio: "inherit"` dropped

### Non-Goals

- Amending the toolkit `update` standard itself (lives in sahil87/shll) — this change fixes tu's `runUpdate` only
- New CLI flags or help-text changes — the `--skip-brew-update` substring contract is a preservation constraint, not a change
- Refactoring `runUpdate` for `execSync` injection — rejected by the established test constraint; source-level pin is the precedent

### Design Decisions

#### Ask-mode suppression via `HOMEBREW_NO_ASK` env var
**Decision**: Pass `HOMEBREW_NO_ASK: "1"` in the child environment of the `brew upgrade tu` call.
**Why**: Homebrew 6 made ask mode the default for `brew upgrade`; the env var is the version-proof disable — older Homebrew versions harmlessly ignore an unrecognized env var.
**Rejected**: Passing `--no-ask`/`--yes`/`-y` to `brew upgrade` — Homebrew < 6 doesn't know the flag and would error.
*Introduced by*: 260812-wdjt-fix-update-brew-ask-prompt

## Tasks

### Phase 1: Core Implementation

- [x] T001 Add `env: { ...process.env, HOMEBREW_NO_ASK: "1" }` to the `execSync("brew upgrade tu", { stdio: "inherit" })` call in `runUpdate` (`src/node/core/cli.ts`, ~line 386) and extend the adjacent comment with the Homebrew-6 ask-mode rationale (env var over `--no-ask` flag for cross-version safety) <!-- R1, R2, R3 -->

### Phase 2: Test Coverage

- [x] T002 Add a co-located source-level pin test in `src/node/core/__tests__/` (new describe block in `cli-update-help.test.ts` alongside the existing no-timeout pin) asserting the single `brew upgrade tu` call site carries `HOMEBREW_NO_ASK: "1"` in `env` and still carries `stdio: "inherit"`; run the affected tests (`npx tsx --test src/node/core/__tests__/cli-update-help.test.ts src/node/core/__tests__/cli-skip-brew-update-flag.test.ts`) <!-- R1, R2, R4, R5 -->

## Acceptance

### Functional Completeness

- [x] A-001 R1: The `brew upgrade tu` `execSync` call in `src/node/core/cli.ts` carries `env: { ...process.env, HOMEBREW_NO_ASK: "1" }`
- [x] A-002 R2: The call keeps `stdio: "inherit"` and has no `timeout` option
- [x] A-003 R3: The comment above the call explains the ask-mode suppression rationale (Homebrew 6 default; env var over `--no-ask` for cross-version safety)
- [x] A-004 R4: `brew update --quiet` and `brew info --json=v2 tu` calls are byte-identical; `--skip-brew-update` and exit-code contracts unchanged
- [x] A-005 R5: A co-located test pins the `HOMEBREW_NO_ASK` + `stdio: "inherit"` posture and passes under `npx tsx --test`

### Behavioral Correctness

- [x] A-006 R1: With the change applied, `tu update`'s upgrade step cannot block on Homebrew's ask-mode prompt (verified by the source-level pin; runtime behavior was verified empirically in the originating conversation)

### Scenario Coverage

- [x] A-007 R4: All pre-existing update tests (`cli-update-help.test.ts`, `cli-skip-brew-update-flag.test.ts`) still pass unmodified

### Edge Cases & Error Handling

- [x] A-008 R2: Homebrew < 6 ignores the unrecognized env var — no flag-based mechanism that would error on older versions is used
- [x] A-009 R4: Genuine `brew upgrade` failure still exits 1 with `Error: brew upgrade failed.` on stderr

### Code Quality

- [x] A-010 Pattern consistency: New code follows naming and structural patterns of surrounding code
- [x] A-011 No unnecessary duplication: Existing utilities reused where applicable
- [x] A-012 Readability over cleverness: the comment extension is plain prose matching the existing comment's style
- [x] A-013 Follows existing patterns: test pin follows the source-level-pin precedent in `cli-update-help.test.ts`

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Deletion Candidates

- None — this change adds new functionality (an env option on the existing `brew upgrade tu` call) without making existing code redundant.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Extend the existing "no hard timeout" describe block area of `cli-update-help.test.ts` with the `HOMEBREW_NO_ASK` pin rather than creating a new test file | Intake says "extend/add ... alongside the existing no-timeout pin"; same call site, same source-level-pin mechanism — co-locating in the same file is the single obvious reading of the precedent | S:80 R:85 A:90 D:85 |
| 2 | Certain | Assert via substring checks on the call-site match (`HOMEBREW_NO_ASK`, `stdio: "inherit"`, no `timeout`) using the same regex-match idiom as the existing pin | The existing pin (`/execSync\(\s*"brew upgrade tu"[^)]*\)/g` + substring asserts) is the exact precedent; no inference needed | S:85 R:85 A:90 D:85 |

2 assumptions (2 certain, 0 confident, 0 tentative).
