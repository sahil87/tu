# Plan: Add -v Version Shorthand

**Change**: 260401-kuuh-add-v-version-shorthand
**Status**: In Progress
**Intake**: `intake.md`
**Spec**: `spec.md`

## Requirements

<!-- migrated from spec.md on 2026-06-02 -->

### CLI: Version Flag

#### Requirement: Lowercase -v as version alias

The CLI SHALL accept `-v` (lowercase) as an alias for `--version`, printing the package version string to stdout and exiting immediately. The existing `--version` and `-V` flags SHALL continue to work unchanged.

##### Scenario: User runs tu -v
- **GIVEN** the `tu` CLI is installed
- **WHEN** the user runs `tu -v`
- **THEN** the CLI prints the package version (e.g., `0.4.3`) to stdout
- **AND** exits with code 0 without running any other command

##### Scenario: Existing --version flag unchanged
- **GIVEN** the `tu` CLI is installed
- **WHEN** the user runs `tu --version`
- **THEN** the CLI prints the package version to stdout and exits

##### Scenario: Existing -V flag unchanged
- **GIVEN** the `tu` CLI is installed
- **WHEN** the user runs `tu -V`
- **THEN** the CLI prints the package version to stdout and exits

##### Scenario: -v with other arguments
- **GIVEN** the user runs `tu -v cc`
- **WHEN** the CLI processes raw arguments
- **THEN** the version check fires first (rawArgs scan) and prints the version
- **AND** no data command is executed

## Tasks

### Phase 1: Core Implementation

- [x] T001 Add `-v` to version flag condition in `src/node/core/cli.ts` line 1014 — append `|| rawArgs.includes("-v")` to the existing `--version` / `-V` check

---

## Execution Order

No dependencies — single task.

## Acceptance

### Functional Completeness
- [x] CHK-001 `-v` flag prints version: Running `tu -v` outputs the package version string to stdout
- [x] CHK-002 `--version` unchanged: Running `tu --version` still prints version
- [x] CHK-003 `-V` unchanged: Running `tu -V` still prints version

### Behavioral Correctness
- [x] CHK-004 `-v` with other args: `tu -v cc` prints version and exits (version check fires first)

### Scenario Coverage
- [x] CHK-005 All three version flags produce identical output

### Code Quality
- [x] CHK-006 Pattern consistency: New condition follows the same `rawArgs.includes()` pattern
- [x] CHK-007 No unnecessary duplication: Single condition line, no helper abstraction

### Notes

- Check items as you review: `- [x]`
- All items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] CHK-008 **N/A**: {reason}`
