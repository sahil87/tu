# Spec: Add Self-Update Command

**Change**: 260307-9mfs-add-update-command
**Created**: 2026-03-07
**Affected memory**: `docs/memory/cli/data-pipeline.md`

## Non-Goals

- No npm/manual auto-update fallback — Homebrew is the sole distribution channel
- No background update checking integrated into `tu status` or startup — would add network latency and violate Fast Startup principle (constitution IV)
- No `SHORT_USAGE` changes — `tu update` is a setup command, not a daily command

## CLI: Self-Update Command

### Requirement: Command Dispatch

The CLI MUST dispatch `tu update` as a non-data command before grammar parsing, alongside `init-conf`, `init-metrics`, `sync`, and `status`.

#### Scenario: Update command dispatched

- **GIVEN** the user runs `tu update`
- **WHEN** the CLI parses arguments
- **THEN** `runUpdate()` is called and the function returns (no grammar parsing occurs)

### Requirement: Homebrew Detection

The CLI MUST detect Homebrew installation by checking if `_pkgDir` contains `/Cellar/tu/`. This leverages the existing `_pkgDir` variable (computed at module top level, lines ~27-31 of `cli.ts`).

#### Scenario: Homebrew-installed binary detected

- **GIVEN** `_pkgDir` is `/opt/homebrew/Cellar/tu/0.2.6/libexec/lib/node_modules/tu`
- **WHEN** `runUpdate()` checks the install path
- **THEN** the Homebrew update flow proceeds

#### Scenario: Non-Homebrew binary detected

- **GIVEN** `_pkgDir` is `/home/user/projects/tu`
- **WHEN** `runUpdate()` checks the install path
- **THEN** the non-Homebrew message is displayed and the function returns with exit code 0

### Requirement: Non-Homebrew Message

When not installed via Homebrew, the CLI MUST print a message indicating the install method and suggesting Homebrew, then return (exit 0, not crash — per constitution principle II, Graceful Degradation).

The message format SHALL be:
```
tu v{PKG_VERSION} was not installed via Homebrew.
Update manually, or reinstall with: brew install wvrdz/tap/tu
```

#### Scenario: Non-Homebrew install shows message

- **GIVEN** tu is not installed via Homebrew
- **WHEN** the user runs `tu update`
- **THEN** the message `tu v{version} was not installed via Homebrew.` is printed to stdout
- **AND** the message `Update manually, or reinstall with: brew install wvrdz/tap/tu` is printed
- **AND** the process exits with code 0

### Requirement: Homebrew Update Flow

When installed via Homebrew, the CLI MUST execute the following steps in order:

1. Print `Current version: v{PKG_VERSION}`
2. Run `brew update --quiet` with `stdio: "pipe"` and 30-second timeout
3. Parse latest version from `brew info --json=v2 tu` (`formulae[0].versions.stable`) with `stdio: "pipe"` and 10-second timeout
4. If latest version equals `PKG_VERSION`: print `Already up to date (v{version}).` and return
5. If different: print `Updating v{current} → v{latest}...`, run `brew upgrade tu` with `stdio: "inherit"` and 120-second timeout
6. Print `Updated to v{latest}.`

#### Scenario: Already up to date

- **GIVEN** tu is installed via Homebrew at version `0.2.7`
- **AND** the latest Homebrew formula version is `0.2.7`
- **WHEN** the user runs `tu update`
- **THEN** `Current version: v0.2.7` is printed
- **AND** `Already up to date (v0.2.7).` is printed
- **AND** the process exits with code 0

#### Scenario: Update available

- **GIVEN** tu is installed via Homebrew at version `0.2.6`
- **AND** the latest Homebrew formula version is `0.2.7`
- **WHEN** the user runs `tu update`
- **THEN** `Current version: v0.2.6` is printed
- **AND** `Updating v0.2.6 → v0.2.7...` is printed
- **AND** `brew upgrade tu` runs with inherited stdio (user sees progress)
- **AND** `Updated to v0.2.7.` is printed

### Requirement: Error Handling

Each brew command failure MUST produce a specific error message on stderr and exit with code 1, per constitution principle II (Graceful Degradation — the CLI MUST NOT crash with an unhandled exception).

| Failure | Error message |
|---------|---------------|
| `brew update` fails | `Error: could not check for updates (brew update failed). Check your network connection.` |
| `brew info` fails | `Error: could not determine latest version.` |
| `brew upgrade` fails | `Error: brew upgrade failed.` |

#### Scenario: brew update fails

- **GIVEN** tu is installed via Homebrew
- **WHEN** `brew update --quiet` throws (timeout or non-zero exit)
- **THEN** `Error: could not check for updates (brew update failed). Check your network connection.` is printed to stderr
- **AND** the process exits with code 1

#### Scenario: brew info fails

- **GIVEN** tu is installed via Homebrew
- **AND** `brew update` succeeds
- **WHEN** `brew info --json=v2 tu` throws
- **THEN** `Error: could not determine latest version.` is printed to stderr
- **AND** the process exits with code 1

#### Scenario: brew upgrade fails

- **GIVEN** tu is installed via Homebrew
- **AND** `brew update` and `brew info` succeed
- **AND** an update is available
- **WHEN** `brew upgrade tu` throws
- **THEN** `Error: brew upgrade failed.` is printed to stderr
- **AND** the process exits with code 1

### Requirement: Help Text

`tu update` MUST appear in the Setup section of `FULL_HELP` with the description `Update tu to latest version`. It MUST NOT appear in `SHORT_USAGE`.

#### Scenario: FULL_HELP contains update command

- **GIVEN** the CLI help text
- **WHEN** the user runs `tu help`
- **THEN** the output contains `tu update` in the Setup section
- **AND** the description reads `Update tu to latest version`

### Requirement: Test Coverage

The `cli-help.test.ts` file MUST include an assertion verifying that `FULL_HELP` contains `tu update`.

#### Scenario: Help test validates update command

- **GIVEN** the test suite in `src/node/core/__tests__/cli-help.test.ts`
- **WHEN** the test for the Setup section runs
- **THEN** it asserts `FULL_HELP.includes("tu update")` is true

## Design Decisions

1. **Shell out to brew rather than download binaries directly**: Simplest approach for a Homebrew-distributed personal tool. Keeps the update flow consistent with user expectations and avoids binary replacement complexity.
   - *Why*: tu is distributed exclusively via Homebrew tap; using the package manager's own update mechanism is the most reliable path.
   - *Rejected*: Direct binary download — would require managing checksums, architectures, and Homebrew formula drift.

2. **Use `execSync` for all brew commands**: Synchronous execution is appropriate since the update command is a terminal operation (no further CLI logic runs after it).
   - *Why*: Simpler control flow, consistent with existing patterns in `cli.ts` (e.g., `runInitMetrics` uses `execSync`).
   - *Rejected*: `execFileSync` — `brew` is a shell script on some systems, and `execSync` handles PATH resolution more reliably.

3. **`brew upgrade` uses `stdio: "inherit"` while other commands use `stdio: "pipe"`**: The upgrade step may take seconds (npm install + esbuild), so the user should see progress. `brew update` and `brew info` are fast/quiet operations where output would be noise.
   - *Why*: Good UX — progress feedback for the slow step, silence for the fast steps.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Shell out to `brew update && brew upgrade tu` | Confirmed from intake #1 — simplest approach for Homebrew-distributed tool | S:95 R:85 A:90 D:95 |
| 2 | Certain | Detect Homebrew via `_pkgDir.includes("/Cellar/tu/")` | Confirmed from intake #2 — reliable detection, Homebrew always uses Cellar path | S:90 R:85 A:90 D:90 |
| 3 | Certain | Non-brew installs get message + exit 0 | Confirmed from intake #3 — constitution principle II (Graceful Degradation) | S:90 R:90 A:95 D:95 |
| 4 | Certain | `brew info --json=v2 tu` for version check | Confirmed from intake #4 — reliable JSON, no fragile text parsing | S:85 R:90 A:90 D:90 |
| 5 | Certain | No npm fallback auto-update | Confirmed from intake #5 — Homebrew is sole distribution channel | S:85 R:90 A:85 D:90 |
| 6 | Certain | No `tu status` integration for update checking | Confirmed from intake #6 — would violate Fast Startup principle | S:80 R:90 A:90 D:90 |
| 7 | Certain | `tu update` in FULL_HELP Setup section only | Confirmed from intake #7 — setup command, not daily use | S:80 R:95 A:85 D:90 |
| 8 | Confident | No dedicated unit test for `runUpdate` | Confirmed from intake #8 — function shells out to brew, unit testing requires mocking execSync for minimal value | S:60 R:90 A:80 D:80 |
| 9 | Confident | `brew upgrade tu` uses `stdio: "inherit"` | Confirmed from intake #9 — user should see build progress | S:50 R:90 A:85 D:85 |
| 10 | Certain | Use `execSync` (not `execFileSync`) for brew commands | Codebase pattern — `runInitMetrics` uses `execSync` for git commands; brew is a shell script on some systems | S:85 R:90 A:90 D:85 |
| 11 | Certain | Error messages go to stderr via `console.error` + `process.exit(1)` | Codebase pattern — all error paths in cli.ts follow this convention | S:90 R:95 A:95 D:95 |

11 assumptions (9 certain, 2 confident, 0 tentative, 0 unresolved).
