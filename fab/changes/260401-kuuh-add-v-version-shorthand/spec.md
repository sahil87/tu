# Spec: Add -v Version Shorthand

**Change**: 260401-kuuh-add-v-version-shorthand
**Created**: 2026-04-01
**Affected memory**: `docs/memory/cli/data-pipeline.md`

## CLI: Version Flag

### Requirement: Lowercase -v as version alias

The CLI SHALL accept `-v` (lowercase) as an alias for `--version`, printing the package version string to stdout and exiting immediately. The existing `--version` and `-V` flags SHALL continue to work unchanged.

#### Scenario: User runs tu -v
- **GIVEN** the `tu` CLI is installed
- **WHEN** the user runs `tu -v`
- **THEN** the CLI prints the package version (e.g., `0.4.3`) to stdout
- **AND** exits with code 0 without running any other command

#### Scenario: Existing --version flag unchanged
- **GIVEN** the `tu` CLI is installed
- **WHEN** the user runs `tu --version`
- **THEN** the CLI prints the package version to stdout and exits

#### Scenario: Existing -V flag unchanged
- **GIVEN** the `tu` CLI is installed
- **WHEN** the user runs `tu -V`
- **THEN** the CLI prints the package version to stdout and exits

#### Scenario: -v with other arguments
- **GIVEN** the user runs `tu -v cc`
- **WHEN** the CLI processes raw arguments
- **THEN** the version check fires first (rawArgs scan) and prints the version
- **AND** no data command is executed

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Keep `-V` as valid alias | Confirmed from intake #1 — backward compatibility | S:90 R:95 A:95 D:95 |
| 2 | Certain | No conflict with other flags | Confirmed from intake #2 — `-v` unused in parseGlobalFlags | S:95 R:90 A:95 D:95 |
| 3 | Certain | Same output as `--version` | Confirmed from intake #3 — prints PKG_VERSION, returns | S:95 R:95 A:95 D:95 |
| 4 | Certain | Implementation is a single condition addition on line 1014 | Codebase verified — rawArgs.includes check is the sole version dispatch | S:95 R:95 A:95 D:95 |
| 5 | Certain | Version check uses rawArgs (not filteredArgs) | Codebase verified — rawArgs scan at line 1014 fires before parseGlobalFlags stripping | S:95 R:95 A:95 D:95 |
| 6 | Certain | No help text update needed | FULL_HELP does not list `-V` either — version flags are not documented in help output | S:90 R:95 A:95 D:95 |
| 7 | Certain | No test changes needed for this flag | Existing test suite does not unit-test `-V`; `-v` follows same pattern | S:85 R:90 A:90 D:90 |

7 assumptions (7 certain, 0 confident, 0 tentative, 0 unresolved).
