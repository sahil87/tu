# Spec: Add Copilot Source

**Change**: 260403-etml-add-copilot-source
**Created**: 2026-04-03
**Affected memory**: `docs/memory/cli/data-pipeline.md`

## CLI: Tool Registry

### Requirement: Copilot tool config

The `TOOLS` registry in `src/node/core/fetcher.ts` SHALL include a `cp` entry with:
- `name`: `"Copilot"`
- `command`: vendor-aware path to `ccusage-copilot` (same pattern as `codex`/`oc`)
- `needsFilter`: `true`

#### Scenario: TOOLS registry contains cp entry
- **GIVEN** the application is initialized
- **WHEN** `TOOLS.cp` is accessed
- **THEN** it SHALL return a `ToolConfig` with `name` "Copilot", a command path ending in `ccusage-copilot`, and `needsFilter` true

#### Scenario: Copilot included in all-tools iteration
- **GIVEN** the `TOOLS` registry contains the `cp` entry
- **WHEN** `Object.keys(TOOLS)` is called
- **THEN** the result SHALL include `"cp"` alongside `"cc"`, `"codex"`, and `"oc"`

### Requirement: Copilot npm dependency

`package.json` SHALL include `@ccusage/copilot` in `dependencies` (matching the version pattern of existing `ccusage`, `@ccusage/codex`, `@ccusage/opencode` packages).
<!-- assumed: npm package is @ccusage/copilot — follows @ccusage/codex and @ccusage/opencode naming pattern -->

#### Scenario: ccusage-copilot binary available after install
- **GIVEN** `npm install` has been run
- **WHEN** the `cp` tool command is executed
- **THEN** the `ccusage-copilot` binary SHALL be available at the expected path

## CLI: Source Parsing

### Requirement: cp source token

The `KNOWN_SOURCES` set in `src/node/core/cli.ts` SHALL include `"cp"`. The `parseDataArgs` function SHALL accept `cp` as a valid source token.

#### Scenario: Parse cp source
- **GIVEN** a user invokes `tu cp`
- **WHEN** `parseDataArgs(["cp"])` is called
- **THEN** it SHALL return `{ source: "cp", period: "daily", display: "snapshot" }`

#### Scenario: Parse cp with period and display
- **GIVEN** a user invokes `tu cp mh`
- **WHEN** `parseDataArgs(["cp", "mh"])` is called
- **THEN** it SHALL return `{ source: "cp", period: "monthly", display: "history" }`

### Requirement: Help text update

The `FULL_HELP` string SHALL list Copilot in the Sources line:
```
Sources: cc (Claude Code), codex/co (Codex), oc (OpenCode), cp (Copilot), all (default)
```

#### Scenario: Help text includes Copilot
- **GIVEN** a user invokes `tu help` or `tu -h`
- **WHEN** the help text is displayed
- **THEN** the Sources line SHALL include `cp (Copilot)`

## CLI: Data Flow

### Requirement: Copilot data fetching

Copilot data fetching SHALL use the same pipeline as existing tools: `runTool` → `execAsync` → parse JSON → normalize labels → cache. No special-case code paths.

#### Scenario: Fetch Copilot daily data
- **GIVEN** `ccusage-copilot` is installed and returns valid JSON
- **WHEN** `fetchHistory("cp", "daily")` is called
- **THEN** it SHALL return `UsageEntry[]` with normalized ISO date labels

#### Scenario: Copilot output filtering
- **GIVEN** `ccusage-copilot` output contains lines starting with `[`
- **WHEN** the output is parsed
- **THEN** noise lines SHALL be stripped via `stripNoise` before JSON parsing (because `needsFilter: true`)

### Requirement: Copilot caching

Copilot fetch results SHALL be cached at `~/.tu/cache/cp-daily.json` with the standard 60-second TTL.

#### Scenario: Cache hit
- **GIVEN** a cached `cp-daily.json` file exists and is less than 60 seconds old
- **WHEN** `fetchHistory("cp", "daily")` is called without `--fresh`
- **THEN** it SHALL return cached entries without invoking `ccusage-copilot`

### Requirement: Copilot in all-tools aggregate

When `source` is `all` (default), Copilot SHALL be included in `fetchAllTotals` and `fetchAllHistory` results alongside Claude Code, Codex, and OpenCode.

#### Scenario: All-tools snapshot includes Copilot
- **GIVEN** the user runs `tu` (default all-tools snapshot)
- **WHEN** data is fetched for all tools
- **THEN** Copilot cost/tokens SHALL appear as a row in the snapshot table (if non-zero)

#### Scenario: All-tools history includes Copilot
- **GIVEN** the user runs `tu h` (all-tools history)
- **WHEN** the history pivot table is rendered
- **THEN** a "Copilot" column SHALL appear alongside existing tool columns

### Requirement: Copilot graceful degradation

When `ccusage-copilot` is not installed or fails, the CLI SHALL warn on stderr and return zero data for the Copilot tool, per Constitution II (Graceful Degradation).

#### Scenario: ccusage-copilot not installed
- **GIVEN** the `ccusage-copilot` binary is not found
- **WHEN** `fetchHistory("cp", "daily")` is called
- **THEN** a warning SHALL be emitted on stderr and an empty array returned

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Source token is `cp` | Confirmed from intake #1 — user explicitly specified | S:95 R:90 A:95 D:95 |
| 2 | Certain | Binary name is `ccusage-copilot` | Confirmed from intake #2 — user explicitly specified | S:95 R:90 A:95 D:95 |
| 3 | Certain | Included in `all` aggregate | Confirmed from intake #3 — user explicitly requested | S:95 R:90 A:95 D:95 |
| 4 | Certain | Uses existing `UsageEntry`/`UsageTotals` data model | Confirmed from intake #4 — Constitution V mandates | S:90 R:95 A:95 D:95 |
| 5 | Certain | No changes to aggregation/dispatch/sync logic | Adding a TOOLS entry auto-includes via Object.keys iteration — verified in code | S:90 R:95 A:95 D:95 |
| 6 | Confident | `needsFilter: true` | Confirmed from intake #5 — follows Codex/OpenCode pattern | S:60 R:90 A:70 D:65 |
| 7 | Confident | Display name is "Copilot" | Confirmed from intake #6 — natural short name | S:70 R:95 A:80 D:70 |
| 8 | Confident | npm package is `@ccusage/copilot` | Confirmed from intake #7 — follows existing naming pattern, version `^18.0.8` | S:50 R:85 A:70 D:60 |

8 assumptions (5 certain, 3 confident, 0 tentative, 0 unresolved).
