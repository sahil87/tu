# Spec: Filter Usage by User

**Change**: 260307-v2bu-filter-usage-by-user
**Created**: 2026-03-07
**Affected memory**: `docs/memory/sync/multi-machine.md`, `docs/memory/cli/data-pipeline.md`

## Non-Goals

- Modifying `writeMetrics()` or `syncMetrics()` — write-side operations are unrelated to the read-side bug
- Adding user listing, discovery, or management features
- Changing the config file format — the target user is a CLI flag, not a persisted setting

## Sync: User-Scoped Remote Reading

### Requirement: Read Only Target User's Directory

`readRemoteEntries()` SHALL read entries from only the specified target user's directory in the metrics repo. It MUST NOT iterate other user directories.

When an `excludeMachine` parameter is provided (non-null string), entries from that machine directory SHALL be skipped. When `excludeMachine` is `null`, all machines for the target user SHALL be included.

#### Scenario: Default read (own user, other machines)

- **GIVEN** metrics repo contains `sahil/2026/macbook/`, `sahil/2026/desktop/`, and `bob/2026/laptop/`
- **WHEN** `readRemoteEntries` is called with targetUser=`sahil`, excludeMachine=`macbook`
- **THEN** entries from `sahil/2026/desktop/` are returned
- **AND** entries from `sahil/2026/macbook/` and `bob/2026/laptop/` are NOT returned

#### Scenario: Read another user's data (all machines)

- **GIVEN** metrics repo contains `bob/2026/laptop/` and `bob/2026/desktop/`
- **WHEN** `readRemoteEntries` is called with targetUser=`bob`, excludeMachine=`null`
- **THEN** entries from both `bob/2026/laptop/` and `bob/2026/desktop/` are returned

#### Scenario: Target user has no data

- **GIVEN** metrics repo has no directory for user `alice`
- **WHEN** `readRemoteEntries` is called with targetUser=`alice`
- **THEN** an empty array is returned

#### Scenario: Only excluded machine exists

- **GIVEN** metrics repo contains only `sahil/2026/macbook/`
- **WHEN** `readRemoteEntries` is called with targetUser=`sahil`, excludeMachine=`macbook`
- **THEN** an empty array is returned

## CLI: User Filter Flag

### Requirement: -u / --user Global Flag

The CLI SHALL accept a `-u <username>` or `--user <username>` global flag that sets the target user for data display.

#### Scenario: View another user's usage

- **GIVEN** multi mode is configured for user `sahil`
- **WHEN** the user runs `tu -u bob`
- **THEN** the CLI displays bob's usage data from the metrics repo

#### Scenario: Combined with source and period

- **GIVEN** multi mode is configured
- **WHEN** the user runs `tu -u bob cc mh`
- **THEN** the CLI displays bob's monthly Claude Code cost history from the metrics repo

#### Scenario: -u in single mode warns and is ignored

- **GIVEN** single mode is configured (or no config file exists)
- **WHEN** the user runs `tu -u bob`
- **THEN** a warning is printed to stderr indicating `-u` requires multi mode
- **AND** the command proceeds without the -u filter (shows own usage)

#### Scenario: -u with --watch mode

- **GIVEN** multi mode is configured
- **WHEN** the user runs `tu -u bob -w`
- **THEN** watch mode displays bob's usage data with live polling refreshes

#### Scenario: -u without a value

- **GIVEN** any configuration
- **WHEN** the user runs `tu -u` with no value following
- **THEN** an error is printed to stderr indicating `-u` requires a username
- **AND** the CLI exits with code 1

### Requirement: Exclude Local Data When -u Is Set

When `-u <username>` is set, the CLI SHALL display only data from the target user's directories in the metrics repo. Locally-fetched ccusage data (which always belongs to the current user's current machine) SHALL NOT be included in the output.

#### Scenario: Only repo data shown for target user

- **GIVEN** multi mode, user `sahil` on `macbook`
- **AND** local ccusage reports $5.00 for today
- **AND** metrics repo has $3.00 for `bob/2026/laptop/` for today
- **WHEN** the user runs `tu -u bob`
- **THEN** the output shows $3.00 (bob's data only)
- **AND** sahil's local $5.00 is NOT included

#### Scenario: Normal path (no -u) is unchanged

- **GIVEN** multi mode, user `sahil` on `macbook`
- **WHEN** the user runs `tu` (no -u flag)
- **THEN** local data is fetched, written to metrics, merged with sahil's other machines' remote data
- **AND** other users' data is NOT included

### Requirement: Help Text Update

`FULL_HELP` SHALL include the `-u` / `--user` flag in the Flags section with a description indicating it shows usage for a specific user in multi mode.

#### Scenario: Flag appears in help output

- **GIVEN** any configuration
- **WHEN** the user runs `tu -h`
- **THEN** the Flags section includes a line for `--user / -u <user>` with appropriate description

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Fix scopes readRemoteEntries to current user only | Confirmed from intake #1 — directly stated by user | S:95 R:70 A:95 D:95 |
| 2 | Certain | New flag is `-u <username>` | Confirmed from intake #2 — user said "add a -u command" | S:90 R:90 A:90 D:95 |
| 3 | Confident | `-u` also supports `--user` long form | Confirmed from intake #3 — matches `-f`/`--fresh`, `-w`/`--watch` pattern | S:50 R:90 A:85 D:80 |
| 4 | Confident | `-u` in single mode warns on stderr and is ignored | Confirmed from intake #4 — graceful degradation per constitution | S:40 R:90 A:90 D:85 |
| 5 | Confident | When `-u` is set, exclude local ccusage data | Confirmed from intake #5 — local data is always current user; including alongside target would be misleading | S:60 R:75 A:80 D:70 |
| 6 | Certain | `writeMetrics()` and `syncMetrics()` code unchanged | Confirmed from intake #6 — write-side unrelated to read-side filtering fix | S:80 R:95 A:95 D:95 |
| 7 | Confident | `-u` works with `--watch` mode | Confirmed from intake #7 — watch mode supports all data flags; no technical barrier | S:40 R:85 A:80 D:85 |
| 8 | Certain | `readRemoteEntries` uses targetUser + excludeMachine params | Function only needs to scope to one user dir; excludeMachine handles double-count prevention | S:85 R:80 A:90 D:90 |
| 9 | Confident | `-u <own-username>` shows repo data only (no local merge) | `-u` always means repo-only; consistent behavior regardless of username value | S:45 R:80 A:75 D:70 |
| 10 | Confident | `-u` without value exits with error (code 1) | Standard flag validation; consistent with `--interval` behavior in codebase | S:50 R:90 A:90 D:85 |
| 11 | Confident | `--sync` with `-u` syncs own data, then displays target user | Sync and display are independent operations; sync always writes current user's data | S:50 R:85 A:80 D:75 |

11 assumptions (4 certain, 7 confident, 0 tentative, 0 unresolved).
