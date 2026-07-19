# Plan: shll update/version/shell-init Standards Conformance

**Change**: 260719-ba5w-update-version-shellinit-conformance
**Intake**: `intake.md`

## Requirements

### CLI: `tu update` surface (update standard)

#### R1: `tu update --help` prints help instead of running the update
The `update` dispatch in `main()` (`src/node/core/cli.ts`) MUST detect `--help` or `-h` among the invocation's args and print `FULL_HELP` (which contains the literal `--skip-brew-update` substring in its Flags section) to stdout, exit 0, WITHOUT invoking `runUpdate` (no brew command runs, no state mutates). The short-circuit SHALL be scoped to the `update` subcommand only — other surfaces' `--help` behavior (e.g. `tu cc --help` → exit 2) stays untouched.

- **GIVEN** tu is invoked as `tu update --help` (or `tu update -h`)
- **WHEN** `main()` dispatches the `update` command
- **THEN** `FULL_HELP` is printed to stdout containing the literal substring `--skip-brew-update`, the process exits 0, and no brew subprocess is executed

#### R2: no short hard timeout on brew; generous bounds on piped calls
`runUpdate` (`src/node/core/cli.ts`) MUST NOT impose a hard timeout on the interactive `brew upgrade tu` call (`stdio: "inherit"`) — the `timeout: 120_000` option is removed entirely. The non-interactive (piped) brew calls SHOULD keep bounds but generous ones: `brew update --quiet` raised 30_000 → 600_000 (10 min, sized for a network transfer); `brew info --json=v2 tu` raised 10_000 → 60_000. Node's default `killSignal` (SIGTERM) satisfies graceful termination.

- **GIVEN** `runUpdate` reaches the upgrade path on a brew-installed binary
- **WHEN** `execSync("brew upgrade tu", …)` runs
- **THEN** the options object carries no `timeout` property, so brew is never killed mid-transaction
- **AND** the `brew update --quiet` and `brew info` call sites carry timeouts of 600_000 and 60_000 respectively

### CLI: `tu shell-init` missing-arg contract (shell-init standard)

#### R3: missing shell arg → usage on stderr, exit 2, stdout empty
`runShellInit` (`src/node/core/cli.ts`) with `shell === undefined` MUST write `SHELL_INIT_USAGE` (content unchanged) to stderr and exit `EXIT_USAGE` (2), leaving stdout empty — stdout may be eval'd by shells (`eval "$(tu shell-init …)"`), so usage text must never reach it. The unknown-shell branch (already stderr + exit 2) stays untouched.

- **GIVEN** tu is invoked as `tu shell-init` with no shell argument
- **WHEN** `runShellInit(undefined)` runs
- **THEN** stdout is empty, stderr carries `Usage: tu shell-init <bash|zsh|fish>` + install examples, and the process exits 2

### Tests: conformance pinning (all three standards' "Verifying conformance" clauses)

#### R4: `--version` pinning test
A test MUST pin the `--version` contract as a subprocess (per `cli-exit-codes.test.ts`'s `runCli` pattern): `tu --version` exits 0, stdout's first non-empty line matches `^tu version v\d+(\.\d+)*`, stderr is empty. The `-V` and `-v` aliases get one representative assertion each (same code path).

- **GIVEN** the new `src/node/core/__tests__/cli-version.test.ts`
- **WHEN** the suite runs `tu --version` / `-V` / `-v` as subprocesses
- **THEN** each exits 0 with the version token on the first non-empty stdout line and no stderr output

#### R5: shell-init eval-in-subshell + missing-arg contract tests
Tests MUST (a) eval the emitted bash script in a `bash` subshell and assert a clean exit 0; (b) do the same for zsh **guarded by availability** (skip with a note when `zsh` is not on PATH); (c) pin the missing-arg contract end-to-end as a subprocess (exit 2, empty stdout, usage on stderr). The existing mock-based unit test in `completions.test.ts` ("runShellInit: no argument") is updated to the new contract (Test Integrity: standards are the spec, the test moves).

- **GIVEN** the emitted `tu shell-init bash` script
- **WHEN** it is eval'd in a bash subshell (`bash -c 'eval "$(cat)"'` with the script on stdin)
- **THEN** the subshell exits 0
- **AND** the zsh equivalent passes where `zsh` exists on PATH, and is skipped with a note otherwise

#### R6: update-help + no-timeout pinning tests
Tests MUST pin (a) `tu update --help` as a subprocess: exit 0, stdout contains the literal substring `--skip-brew-update`, no dependency on brew being installed (the short-circuit returns before `runUpdate`); (b) that the `brew upgrade tu` execSync call site carries no `timeout` option (source-level assertion reading `cli.ts`, mirroring how `cli-skip-brew-update-flag.test.ts` pins the gate).

- **GIVEN** the new `src/node/core/__tests__/cli-update-help.test.ts`
- **WHEN** `tu update --help` runs as a subprocess on a machine with or without brew
- **THEN** it exits 0 and stdout contains `--skip-brew-update`
- **AND** the source-level assertion fails if a `timeout` option is ever re-added to the `brew upgrade tu` call

### Non-Goals

- `fish` shell-init support — kept as-is (standard scope is zsh/bash; fish is additive, not a violation)
- Global `--help`-anywhere handling — only the `update` dispatch gets the short-circuit; other subcommands' `--help` behavior is pinned elsewhere and unchanged
- Qualifying `brew upgrade tu` → `brew upgrade sahil87/tap/tu` — not required by the standard
- Version bump — the release-PR flow assigns the minor bump the Output Stability article asks for; no version file is edited here
- README/docs/site updates — the three standards impose no repo-doc obligations beyond binary behavior

## Tasks

### Phase 1: Core Implementation (`src/node/core/cli.ts`)

- [x] T001 In `main()`'s `update` dispatch (`src/node/core/cli.ts` ~line 1414), short-circuit `--help`/`-h` to `console.log(FULL_HELP); return;` before `runUpdate` is invoked <!-- R1 -->
- [x] T002 In `runUpdate` (`src/node/core/cli.ts`): remove `timeout: 120_000` from the `brew upgrade tu` execSync options entirely; raise `brew update --quiet` timeout 30_000 → 600_000 and `brew info --json=v2 tu` timeout 10_000 → 60_000 <!-- R2 -->
- [x] T003 In `runShellInit` (`src/node/core/cli.ts` ~line 315): change the `shell === undefined` branch from `console.log(SHELL_INIT_USAGE); return;` to `console.error(SHELL_INIT_USAGE); process.exit(EXIT_USAGE);` (with a `return;` after the exit so the mocked-exit unit test cannot fall through to the unknown-shell branch) <!-- R3 -->

### Phase 2: Tests

- [x] T004 Update `src/node/core/__tests__/completions.test.ts` "runShellInit: no argument" to assert the new contract: usage on stderr (captured `console.error`), exit code 2, nothing on stdout/`console.log` <!-- R3 -->
- [x] T005 [P] New `src/node/core/__tests__/cli-version.test.ts` (subprocess via `runCli` pattern from `cli-exit-codes.test.ts`): `tu --version` exits 0, first non-empty stdout line matches `^tu version v\d+(\.\d+)*$`, stderr empty; one representative assertion each for `-V` and `-v` <!-- R4 -->
- [x] T006 [P] New `src/node/core/__tests__/cli-shell-init.test.ts` (subprocess): missing-arg contract (exit 2, empty stdout, usage on stderr); bash eval-in-subshell test (`bash -c 'eval "$(cat)"'` with the emitted script on stdin, assert exit 0); zsh eval test guarded by `zsh` availability on PATH (skip with note when absent) <!-- R5 -->
- [x] T007 [P] New `src/node/core/__tests__/cli-update-help.test.ts`: subprocess `tu update --help` exits 0 with `--skip-brew-update` in stdout and no brew dependency; source-level assertion that the `brew upgrade tu` execSync call site in `cli.ts` carries no `timeout` option <!-- R6 -->

## Acceptance

### Functional Completeness

- [ ] A-001 R1: `tu update --help` and `tu update -h` print `FULL_HELP` (containing literal `--skip-brew-update`) to stdout and exit 0 without executing any brew command
- [ ] A-002 R2: the `brew upgrade tu` call site has no `timeout` option; `brew update --quiet` is bounded at 600_000 and `brew info --json=v2 tu` at 60_000
- [ ] A-003 R3: `tu shell-init` with no argument writes usage to stderr, exits 2, and emits nothing on stdout

### Behavioral Correctness

- [ ] A-004 R3: the previously-pinned wrong behavior (usage on stdout, exit 0) is gone — `completions.test.ts` "no argument" now asserts stderr + exit 2 and passes

### Scenario Coverage

- [ ] A-005 R4: `cli-version.test.ts` exists and pins `--version`/`-V`/`-v`: exit 0, first non-empty stdout line matches `^tu version v\d+(\.\d+)*`, stderr empty
- [ ] A-006 R5: eval-in-subshell tests exist — bash eval always asserted exit 0; zsh eval skip-guarded on PATH availability; missing-arg subprocess contract pinned
- [ ] A-007 R6: `tu update --help` subprocess test passes without brew installed; source-level no-timeout assertion pins R2

### Edge Cases & Error Handling

- [ ] A-008 R1: other surfaces' `--help` behavior is untouched — existing pinned behaviors (e.g. `tu cc --help` exit 2, first-arg `--help` global help) still pass
- [ ] A-009 R5: on a machine without `zsh`, the zsh eval test skips with a note rather than failing (suite stays green)

### Code Quality

- [ ] A-010 Pattern consistency: new code follows surrounding conventions — `console.error` + `process.exit(EXIT_USAGE)` idiom, `runCli` subprocess helper shape, `node:` imports, `type` imports where type-only
- [ ] A-011 No unnecessary duplication: reuses `FULL_HELP`, `SHELL_INIT_USAGE`, `EXIT_USAGE` constants and the established `runCli`/`spawnSync` test pattern instead of new machinery
- [ ] A-012 Errors on stderr: the changed error path (missing shell arg) diagnoses on stderr, never stdout; no errors are silently swallowed

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | Subprocess-level shell-init tests (missing-arg contract, eval-in-subshell) go in a new `cli-shell-init.test.ts`; the existing mock-based unit test in `completions.test.ts` is updated in place | `completions.test.ts` is unit/mock style; subprocess tests follow the established separate-file pattern (`cli-exit-codes.test.ts`); intake explicitly allowed "a new cli-shell-init.test.ts if cleaner" | S:70 R:90 A:85 D:70 |
| 2 | Confident | update-help subprocess test + no-timeout source assertion go in a new `cli-update-help.test.ts` rather than extending `cli-skip-brew-update-flag.test.ts` | The existing file is pure-unit mirror style with no subprocess machinery; intake offered "new file or extension" — a subprocess file matches the `cli-exit-codes.test.ts` precedent | S:65 R:90 A:80 D:65 |
| 3 | Confident | No-timeout pinning idiom: the test reads `cli.ts` source and asserts the `brew upgrade tu` execSync options carry no `timeout` key | Intake deferred idiom choice to apply time ("least brittle"); a scoped source read mirrors how `cli-skip-brew-update-flag.test.ts` pins un-mockable execSync behavior without refactoring `runUpdate` for injection | S:60 R:85 A:75 D:60 |
| 4 | Confident | Eval tests pipe the emitted script to `bash -c 'eval "$(cat)"'` / `zsh -c 'eval "$(cat)"'` via stdin | Matches the standard's `eval "$(tool shell-init sh)"` install idiom without nesting a tsx invocation inside the subshell; the zsh script lazy-loads compinit itself so a bare `zsh -c` eval is expected to succeed | S:60 R:90 A:75 D:65 |

4 assumptions (0 certain, 4 confident, 0 tentative).
