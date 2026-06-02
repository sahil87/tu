# Plan: Add `--skip-brew-update` flag to `update` command

**Change**: 260531-e96v-skip-brew-update-flag
**Status**: In Progress
**Intake**: `intake.md`
**Spec**: `spec.md`

## Requirements

<!-- migrated from spec.md on 2026-06-02 -->

### Non-Goals

- Changing the behavior of any command other than `update` — the flag is scoped to `tu update` only.
- Adding a flag-parsing library or restructuring `parseGlobalFlags` — tu's hand-rolled `rawArgs.includes(...)` convention is retained.
- Refactoring `runUpdate` for dependency injection / injectable subprocess runner — out of contract scope.
- Altering the `brew info` version check, the up-to-date short-circuit, or `brew upgrade tu` behavior in any way.
- Adding a short alias (e.g. `-s`) — the contract specifies only the long form `--skip-brew-update`.

### CLI: `update` command

#### Requirement: `--skip-brew-update` flag gates only the internal `brew update` refresh

The `update` command SHALL accept a boolean flag named EXACTLY `--skip-brew-update`. When the flag is
present, `runUpdate()` SHALL NOT execute the internal `execSync("brew update --quiet", ...)` tap-metadata
refresh. The flag name MUST match the cross-toolkit contract string `--skip-brew-update` exactly (no alias,
no abbreviation).

##### Scenario: Flag present skips the brew update refresh
- **GIVEN** `tu` is installed via Homebrew (the `/Cellar/tu/` install guard passes)
- **WHEN** the user runs `tu update --skip-brew-update`
- **THEN** `brew update --quiet` is NOT executed
- **AND** execution proceeds directly to the `brew info --json=v2 tu` version check

##### Scenario: Flag absent runs the brew update refresh (default preserved)
- **GIVEN** `tu` is installed via Homebrew
- **WHEN** the user runs `tu update` with no `--skip-brew-update` flag
- **THEN** `brew update --quiet` IS executed exactly as in current behavior
- **AND** all subsequent steps (version check, short-circuit, upgrade) run unchanged

#### Requirement: version check, short-circuit, and upgrade are preserved regardless of the flag

Setting `--skip-brew-update` SHALL affect ONLY the `brew update --quiet` call. The `brew info --json=v2 tu`
version check, the up-to-date short-circuit (`latest === PKG_VERSION` → "Already up to date" → return), and
the `brew upgrade tu` invocation MUST remain intact and behave identically whether or not the flag is set.

##### Scenario: Upgrade still runs when flag is set and a newer version exists
- **GIVEN** `tu` is installed via Homebrew and the latest stable version differs from the installed version
- **WHEN** the user runs `tu update --skip-brew-update`
- **THEN** `brew info --json=v2 tu` is executed to determine the latest version
- **AND** `brew upgrade tu` IS executed
- **AND** the "Updated to vX.Y.Z." message is printed

##### Scenario: Up-to-date short-circuit still applies when flag is set
- **GIVEN** `tu` is installed via Homebrew and the latest stable version equals the installed version
- **WHEN** the user runs `tu update --skip-brew-update`
- **THEN** `brew info --json=v2 tu` is executed
- **AND** the command prints "Already up to date (vX.Y.Z)." and returns
- **AND** `brew upgrade tu` is NOT executed (same short-circuit as default)

##### Scenario: Non-Homebrew install guard is unaffected
- **GIVEN** `tu` was NOT installed via Homebrew (install dir does not contain `/Cellar/tu/`)
- **WHEN** the user runs `tu update --skip-brew-update`
- **THEN** the command prints the "was not installed via Homebrew" message and returns
- **AND** neither `brew update` nor `brew upgrade` is executed (identical to default behavior)

#### Requirement: dispatcher detects the flag and passes a boolean into `runUpdate()`

The `update` dispatch site SHALL detect `--skip-brew-update` using tu's established raw-argument membership
idiom (consistent with `rawArgs.includes("--sync")` in `parseGlobalFlags`) and SHALL pass the resulting
boolean into `runUpdate()`. `runUpdate()` SHALL take a single boolean parameter that defaults to `false`, so
existing zero-argument callers and tests remain valid. Detection SHALL NOT be added to `parseGlobalFlags` or
the `GlobalFlags` interface — `--skip-brew-update` is a command-specific flag for `update`, which ignores
positional arguments.

##### Scenario: Dispatcher passes true when flag present
- **GIVEN** the CLI is invoked with `update` as the command and `--skip-brew-update` among the arguments
- **WHEN** the dispatcher routes to `runUpdate`
- **THEN** `runUpdate` is called with its boolean parameter set to `true`

##### Scenario: Dispatcher passes false when flag absent
- **GIVEN** the CLI is invoked with `update` and no `--skip-brew-update` flag
- **WHEN** the dispatcher routes to `runUpdate`
- **THEN** `runUpdate` is called with its boolean parameter set to `false` (the default)

#### Requirement: help text documents the flag

The `FULL_HELP` constant's `Flags:` block SHALL include a line documenting `--skip-brew-update`, aligned to
the existing column format, with a description that scopes it to `tu update` (it does not apply to data
commands).

##### Scenario: Help lists the flag
- **GIVEN** the user runs `tu help` (or `tu -h` / `tu --help`)
- **WHEN** `FULL_HELP` is printed
- **THEN** the output contains a `--skip-brew-update` entry describing that it skips the `brew update` tap
  refresh during `tu update`

### CLI: test coverage

#### Requirement: a test asserts the gate omits `brew update` but keeps `brew upgrade`

A new test file `src/node/core/__tests__/cli-skip-brew-update-flag.test.ts` SHALL follow the existing tu test
pattern (Node built-in runner, co-located `__tests__/`, `describe`/`it`, `node:assert/strict`) as exemplified
by `cli-sync-flag.test.ts`. Rather than spawning real Homebrew or mocking the static `execSync` import, the
test SHALL mirror the command-selection logic of the gate inline (the same approach `cli-sync-flag.test.ts`
uses for the `--sync` guard) and assert the command sequence under each flag value.

##### Scenario: skip=true omits brew update, keeps brew upgrade
- **GIVEN** the mirrored command-selection helper representing `runUpdate`'s gate
- **WHEN** it is evaluated with `skipBrewUpdate = true`
- **THEN** the resulting command sequence does NOT contain any `brew update` command
- **AND** the sequence DOES contain `brew upgrade tu`

##### Scenario: skip=false includes both brew update and brew upgrade
- **GIVEN** the mirrored command-selection helper
- **WHEN** it is evaluated with `skipBrewUpdate = false`
- **THEN** the sequence contains `brew update --quiet`
- **AND** the sequence contains `brew upgrade tu`

##### Scenario: flag detection idiom yields correct boolean
- **GIVEN** an argument array
- **WHEN** the raw-argv membership test for `--skip-brew-update` is applied
- **THEN** it returns `true` iff `--skip-brew-update` is present in the array

### Design Decisions

1. **Detect the flag via raw-argv membership, not via `parseGlobalFlags`**: The dispatcher passes
   `process.argv.includes("--skip-brew-update")` (or the equivalent membership test against the args array it
   already holds) into `runUpdate`.
   - *Why*: `--skip-brew-update` is specific to the `update` command, which ignores positional/data arguments.
     `parseGlobalFlags` and the `GlobalFlags` interface govern data-display flags (`--json`, `--sync`,
     `--fresh`, etc.); threading a command-specific flag through them would broaden the change surface and risk
     perturbing unrelated flag handling. Membership testing matches the existing `rawArgs.includes(...)` idiom.
   - *Rejected*: Adding `skipBrewUpdate` to `GlobalFlags` and `parseGlobalFlags` — larger blast radius, touches
     the shared data-command flag path, contradicts the contract's "detect from process.argv" wording.

2. **`runUpdate(skipBrewUpdate = false)` — single boolean param with default**:
   - *Why*: Default `false` preserves every existing call site and test (the function is currently called with
     zero arguments) and encodes "absent = current behavior exactly preserved" directly in the signature.
   - *Rejected*: An options object `{ skipBrewUpdate }` — unnecessary for a single boolean; inconsistent with
     the codebase's preference for simple positional parameters in internal helpers.

3. **Test mirrors gate logic inline rather than mocking `execSync`**:
   - *Why*: `runUpdate` calls a statically imported `execSync` binding; ESM/tsx cannot reassign that binding
     from a test without refactoring `runUpdate` for injection, which is out of scope. The established
     precedent (`cli-sync-flag.test.ts`) mirrors guard logic inline. This satisfies the contract's assertion
     ("omits `brew update` but still runs `brew upgrade`") with zero production refactor.
   - *Rejected*: Refactoring `runUpdate` to accept an injectable runner — adds indirection beyond the contract;
     *Rejected*: spawning real `brew` — non-hermetic, network-dependent, violates fast/isolated test norms.

## Tasks

### Phase 1: Setup

<!-- No scaffolding or dependencies needed — single existing file plus one new co-located test. -->

(none)

### Phase 2: Core Implementation

- [x] T001 In `src/node/core/cli.ts`, change `runUpdate()` (~L273) to accept a single boolean parameter with a default: `export function runUpdate(skipBrewUpdate = false): void`. Wrap ONLY the `execSync("brew update --quiet", { stdio: "pipe", timeout: 30_000 })` block (~L283, including its `try/catch` that errors + `process.exit(1)`) so it runs only when `!skipBrewUpdate`. Leave `brew info --json=v2 tu` (~L291), the `latest === PKG_VERSION` up-to-date short-circuit (~L303), and `brew upgrade tu` (~L311) completely unchanged.

- [x] T002 In `src/node/core/cli.ts`, update the `update` dispatch line (~L1138) from `if (cmd === "update") { runUpdate(); return; }` to detect the flag via tu's raw-argv membership idiom and pass the boolean in: `if (cmd === "update") { runUpdate(process.argv.includes("--skip-brew-update")); return; }`. Do NOT add the flag to `parseGlobalFlags` or the `GlobalFlags` interface.

### Phase 3: Integration & Edge Cases

- [x] T003 In `src/node/core/cli.ts`, add a `--skip-brew-update` entry to the `FULL_HELP` `Flags:` block (~L77–88), aligned to the existing column format (match the alignment of `--by-machine`). Suggested line: `  --skip-brew-update   Skip 'brew update' tap refresh during 'tu update'`.

### Phase 4: Polish

- [x] T004 [P] Create `src/node/core/__tests__/cli-skip-brew-update-flag.test.ts` following the `cli-sync-flag.test.ts` pattern (Node built-in test runner: `import { describe, it } from "node:test"` + `import assert from "node:assert/strict"`). Mirror the gate's command-selection logic inline (do NOT mock `execSync`, do NOT spawn `brew`). Assert: (a) with `skipBrewUpdate = true`, the planned command sequence contains `brew upgrade tu` but NO `brew update ...` command; (b) with `skipBrewUpdate = false`, the sequence contains both `brew update --quiet` and `brew upgrade tu`; (c) the raw-argv membership test `["update","--skip-brew-update"].includes("--skip-brew-update")` is `true` and absent → `false`.

- [x] T005 Build and verify: run `npm run build` (must succeed — single ESM bundle, strict TS) and run the new test via the project's Node test-runner convention (e.g. `npx tsx --test src/node/core/__tests__/cli-skip-brew-update-flag.test.ts`). Both must pass before the change is considered apply-complete. Also run the two sibling flag tests (`cli-fresh-flag.test.ts`, `cli-sync-flag.test.ts`) to confirm no regression in flag handling.

---

## Execution Order

- T001 → T002 → T003 are edits to the same file (`cli.ts`); do them in sequence to avoid overlapping edits. T002 depends conceptually on T001's new signature.
- T004 (new test file) is independent of the `cli.ts` edits and may be written in parallel `[P]`, but its assertions mirror the gate logic from T001 — author it consistent with the final T001 behavior.
- T005 (build + test) is the final gate and depends on T001–T004 all being complete.

## Acceptance

### Functional Completeness
<!-- Every requirement in spec.md has working implementation -->
- [x] CHK-001 Flag gates only the refresh: `runUpdate` accepts a boolean param; when set, `execSync("brew update --quiet", ...)` is skipped and flow proceeds to `brew info`.
- [x] CHK-002 Version check / short-circuit / upgrade preserved: `brew info --json=v2 tu`, the `latest === PKG_VERSION` short-circuit, and `brew upgrade tu` are byte-for-byte unchanged from the pre-change code.
- [x] CHK-003 Dispatcher detection: the `update` dispatch passes `process.argv.includes("--skip-brew-update")` into `runUpdate`; flag NOT added to `parseGlobalFlags`/`GlobalFlags`.
- [x] CHK-004 Help text: `FULL_HELP` `Flags:` block contains a `--skip-brew-update` entry, column-aligned, scoped to `tu update`.
- [x] CHK-005 Test exists: `src/node/core/__tests__/cli-skip-brew-update-flag.test.ts` present, follows Node-runner + `cli-sync-flag.test.ts` inline-mirror pattern.

### Behavioral Correctness
<!-- Changed requirements behave as specified, not as before -->
- [x] CHK-006 Flag name is EXACTLY `--skip-brew-update` (no alias, no abbreviation) — matches cross-toolkit contract.
- [x] CHK-007 Default (flag absent) preserves current behavior exactly: `brew update --quiet` still runs when the flag is not passed (encoded via `skipBrewUpdate = false` default).
- [x] CHK-008 `runUpdate` remains callable with zero arguments (existing call sites/tests unbroken by the new default param).

### Removal Verification
<!-- Every deprecated requirement is actually gone -->
- [x] CHK-009 **N/A**: This change is purely additive — no requirements or code paths are removed.

### Scenario Coverage
<!-- Key scenarios from spec.md have been exercised -->
- [x] CHK-010 "Flag present skips refresh" + "Upgrade still runs when flag set": covered by the new test asserting skip=true omits `brew update` but keeps `brew upgrade tu`.
- [x] CHK-011 "Flag absent runs refresh": covered by the new test asserting skip=false includes both `brew update --quiet` and `brew upgrade tu`.
- [x] CHK-012 Flag-detection idiom: covered by the test asserting `includes("--skip-brew-update")` returns the correct boolean.

### Edge Cases & Error Handling
<!-- Error states, boundary conditions, failure modes -->
- [x] CHK-013 Non-Homebrew install guard unaffected: with the flag set, the `/Cellar/tu/` guard still returns early before any `brew` call.
- [x] CHK-014 Skipping `brew update` does not break the `brew info` failure path: the existing `try/catch` around `brew info` (errors → "could not determine latest version" → exit 1) is untouched and still reachable when the flag is set.

### Code Quality
<!-- From fab/project/code-quality.md principles + anti-patterns relevant to this change -->
- [x] CHK-015 Pattern consistency: flag detection uses the existing `rawArgs.includes(...)` / `process.argv.includes(...)` idiom; subprocess uses `execSync` (no deviation from convention).
- [x] CHK-016 No unnecessary duplication: reuses existing `execSync` import and the established flag-membership pattern; no parallel flag-parsing path introduced.
- [x] CHK-017 No magic strings without intent: the literal `"--skip-brew-update"` is the contract-mandated flag string used in dispatch (acceptable as a single-use literal, consistent with `"--sync"`/`"--fresh"` in `parseGlobalFlags`).
- [x] CHK-018 Minimum pathways: gating is a single boolean branch around one `execSync` call — no second code path duplicating the update flow.
- [x] CHK-019 No silently swallowed errors: the gated block's existing error handling (warn on stderr + exit) is preserved; skipping it does not hide a failure (the refresh simply isn't attempted).
- [x] CHK-020 Strict TypeScript: `npm run build` passes under strict mode; the new param is correctly typed `boolean` with a default.

### Security
<!-- Subprocess command construction -->
- [x] CHK-021 No command injection surface: the `brew update`/`brew info`/`brew upgrade` command strings remain static literals; the flag only toggles whether a fixed command runs — no user input is interpolated into any shell command.

### Notes

- Check items as you review: `- [x]`
- All items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] CHK-XXX **N/A**: {reason}`
