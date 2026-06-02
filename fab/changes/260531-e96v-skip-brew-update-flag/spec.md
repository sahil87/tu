# Spec: Add `--skip-brew-update` flag to `update` command

**Change**: 260531-e96v-skip-brew-update-flag
**Created**: 2026-05-31
**Affected memory**: `docs/memory/cli/data-pipeline.md`

## Non-Goals

- Changing the behavior of any command other than `update` — the flag is scoped to `tu update` only.
- Adding a flag-parsing library or restructuring `parseGlobalFlags` — tu's hand-rolled `rawArgs.includes(...)` convention is retained.
- Refactoring `runUpdate` for dependency injection / injectable subprocess runner — out of contract scope.
- Altering the `brew info` version check, the up-to-date short-circuit, or `brew upgrade tu` behavior in any way.
- Adding a short alias (e.g. `-s`) — the contract specifies only the long form `--skip-brew-update`.

## CLI: `update` command

### Requirement: `--skip-brew-update` flag gates only the internal `brew update` refresh

The `update` command SHALL accept a boolean flag named EXACTLY `--skip-brew-update`. When the flag is
present, `runUpdate()` SHALL NOT execute the internal `execSync("brew update --quiet", ...)` tap-metadata
refresh. The flag name MUST match the cross-toolkit contract string `--skip-brew-update` exactly (no alias,
no abbreviation).

#### Scenario: Flag present skips the brew update refresh
- **GIVEN** `tu` is installed via Homebrew (the `/Cellar/tu/` install guard passes)
- **WHEN** the user runs `tu update --skip-brew-update`
- **THEN** `brew update --quiet` is NOT executed
- **AND** execution proceeds directly to the `brew info --json=v2 tu` version check

#### Scenario: Flag absent runs the brew update refresh (default preserved)
- **GIVEN** `tu` is installed via Homebrew
- **WHEN** the user runs `tu update` with no `--skip-brew-update` flag
- **THEN** `brew update --quiet` IS executed exactly as in current behavior
- **AND** all subsequent steps (version check, short-circuit, upgrade) run unchanged

### Requirement: version check, short-circuit, and upgrade are preserved regardless of the flag

Setting `--skip-brew-update` SHALL affect ONLY the `brew update --quiet` call. The `brew info --json=v2 tu`
version check, the up-to-date short-circuit (`latest === PKG_VERSION` → "Already up to date" → return), and
the `brew upgrade tu` invocation MUST remain intact and behave identically whether or not the flag is set.

#### Scenario: Upgrade still runs when flag is set and a newer version exists
- **GIVEN** `tu` is installed via Homebrew and the latest stable version differs from the installed version
- **WHEN** the user runs `tu update --skip-brew-update`
- **THEN** `brew info --json=v2 tu` is executed to determine the latest version
- **AND** `brew upgrade tu` IS executed
- **AND** the "Updated to vX.Y.Z." message is printed

#### Scenario: Up-to-date short-circuit still applies when flag is set
- **GIVEN** `tu` is installed via Homebrew and the latest stable version equals the installed version
- **WHEN** the user runs `tu update --skip-brew-update`
- **THEN** `brew info --json=v2 tu` is executed
- **AND** the command prints "Already up to date (vX.Y.Z)." and returns
- **AND** `brew upgrade tu` is NOT executed (same short-circuit as default)

#### Scenario: Non-Homebrew install guard is unaffected
- **GIVEN** `tu` was NOT installed via Homebrew (install dir does not contain `/Cellar/tu/`)
- **WHEN** the user runs `tu update --skip-brew-update`
- **THEN** the command prints the "was not installed via Homebrew" message and returns
- **AND** neither `brew update` nor `brew upgrade` is executed (identical to default behavior)

### Requirement: dispatcher detects the flag and passes a boolean into `runUpdate()`

The `update` dispatch site SHALL detect `--skip-brew-update` using tu's established raw-argument membership
idiom (consistent with `rawArgs.includes("--sync")` in `parseGlobalFlags`) and SHALL pass the resulting
boolean into `runUpdate()`. `runUpdate()` SHALL take a single boolean parameter that defaults to `false`, so
existing zero-argument callers and tests remain valid. Detection SHALL NOT be added to `parseGlobalFlags` or
the `GlobalFlags` interface — `--skip-brew-update` is a command-specific flag for `update`, which ignores
positional arguments.

#### Scenario: Dispatcher passes true when flag present
- **GIVEN** the CLI is invoked with `update` as the command and `--skip-brew-update` among the arguments
- **WHEN** the dispatcher routes to `runUpdate`
- **THEN** `runUpdate` is called with its boolean parameter set to `true`

#### Scenario: Dispatcher passes false when flag absent
- **GIVEN** the CLI is invoked with `update` and no `--skip-brew-update` flag
- **WHEN** the dispatcher routes to `runUpdate`
- **THEN** `runUpdate` is called with its boolean parameter set to `false` (the default)

### Requirement: help text documents the flag

The `FULL_HELP` constant's `Flags:` block SHALL include a line documenting `--skip-brew-update`, aligned to
the existing column format, with a description that scopes it to `tu update` (it does not apply to data
commands).

#### Scenario: Help lists the flag
- **GIVEN** the user runs `tu help` (or `tu -h` / `tu --help`)
- **WHEN** `FULL_HELP` is printed
- **THEN** the output contains a `--skip-brew-update` entry describing that it skips the `brew update` tap
  refresh during `tu update`

## CLI: test coverage

### Requirement: a test asserts the gate omits `brew update` but keeps `brew upgrade`

A new test file `src/node/core/__tests__/cli-skip-brew-update-flag.test.ts` SHALL follow the existing tu test
pattern (Node built-in runner, co-located `__tests__/`, `describe`/`it`, `node:assert/strict`) as exemplified
by `cli-sync-flag.test.ts`. Rather than spawning real Homebrew or mocking the static `execSync` import, the
test SHALL mirror the command-selection logic of the gate inline (the same approach `cli-sync-flag.test.ts`
uses for the `--sync` guard) and assert the command sequence under each flag value.

#### Scenario: skip=true omits brew update, keeps brew upgrade
- **GIVEN** the mirrored command-selection helper representing `runUpdate`'s gate
- **WHEN** it is evaluated with `skipBrewUpdate = true`
- **THEN** the resulting command sequence does NOT contain any `brew update` command
- **AND** the sequence DOES contain `brew upgrade tu`

#### Scenario: skip=false includes both brew update and brew upgrade
- **GIVEN** the mirrored command-selection helper
- **WHEN** it is evaluated with `skipBrewUpdate = false`
- **THEN** the sequence contains `brew update --quiet`
- **AND** the sequence contains `brew upgrade tu`

#### Scenario: flag detection idiom yields correct boolean
- **GIVEN** an argument array
- **WHEN** the raw-argv membership test for `--skip-brew-update` is applied
- **THEN** it returns `true` iff `--skip-brew-update` is present in the array

## Design Decisions

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

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Flag name is EXACTLY `--skip-brew-update` (long form only, no alias) | Confirmed from intake #1 — cross-toolkit contract mandates the exact string; identical in 6 tools | S:100 R:90 A:100 D:100 |
| 2 | Certain | Gate ONLY `execSync("brew update --quiet", ...)`; `brew info`, the up-to-date short-circuit, and `brew upgrade tu` stay intact | Confirmed from intake #2 — verified against current `src/node/core/cli.ts` L283/L291/L303/L311 | S:100 R:85 A:100 D:100 |
| 3 | Certain | Default (flag absent) preserves current behavior exactly; encoded as `skipBrewUpdate = false` default param | Confirmed from intake #3 — contract states default = current behavior exactly preserved | S:100 R:90 A:100 D:100 |
| 4 | Confident | `runUpdate(skipBrewUpdate = false)`; dispatcher passes `process.argv.includes("--skip-brew-update")` | Confirmed from intake #4 — matches tu's `rawArgs.includes(...)` idiom; default param keeps existing callers valid | S:92 R:82 A:92 D:88 |
| 5 | Confident | Detect via raw-argv membership, NOT by extending `parseGlobalFlags`/`GlobalFlags` | Confirmed from intake #5 — command-specific flag, lowest blast radius; see Design Decision 1 | S:88 R:78 A:88 D:82 |
| 6 | Confident | Add one `FULL_HELP` flag line scoped to `tu update`, aligned to existing column format | Confirmed from intake #6 — the Flags: block documents flags; matches existing format | S:90 R:90 A:90 D:88 |
| 7 | Confident | New test `src/node/core/__tests__/cli-skip-brew-update-flag.test.ts`, Node built-in runner, co-located | Confirmed from intake #7 — constitution mandates co-located `__tests__/` + Node runner; naming follows `cli-{flag}-flag.test.ts` | S:92 R:85 A:95 D:85 |
| 8 | Confident | Test mirrors the gate's command-selection logic inline (per `cli-sync-flag.test.ts`); no `execSync` mock, no `runUpdate` refactor | Confirmed from intake #8 (user-resolved) — matches existing precedent; see Design Decision 3 | S:90 R:80 A:90 D:90 |

8 assumptions (3 certain, 5 confident, 0 tentative, 0 unresolved).

<!-- Merged into plan.md ## Requirements on 2026-06-02 — safe to delete. -->
