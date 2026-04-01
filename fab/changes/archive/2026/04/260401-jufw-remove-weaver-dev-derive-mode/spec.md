# Spec: Remove WEAVER_DEV and derive mode from metrics_repo

**Change**: 260401-jufw-remove-weaver-dev-derive-mode
**Created**: 2026-04-01
**Affected memory**: `docs/memory/configuration/config-system.md`

## Non-Goals

- Changing the Homebrew tap reference (`brew install wvrdz/tap/tu`) in `runUpdate()` — separate change
- Modifying the `TuConfig` interface shape — `mode` field remains for downstream consumers
- Bumping config version — version stays at 2

## Configuration: Mode Derivation

### Requirement: Mode SHALL be derived from metrics_repo presence

The `mode` field on `TuConfig` SHALL be computed at runtime in `readConfig()` as:
- `"multi"` when `metricsRepo !== ""`
- `"single"` when `metricsRepo === ""`

The `mode` key in config files (both `tu.default.conf` and `~/.tu.conf`) SHALL be silently ignored by the parser. Existing user configs with `mode = single` or `mode = multi` continue to load without error — the field is simply not used.

#### Scenario: Config file has metrics_repo set
- **GIVEN** `~/.tu.conf` contains `metrics_repo = git@github.com:user/repo.git`
- **WHEN** `readConfig()` is called
- **THEN** `config.mode` equals `"multi"`

#### Scenario: Config file has no metrics_repo
- **GIVEN** `~/.tu.conf` exists but does not set `metrics_repo`
- **AND** `tu.default.conf` has `metrics_repo` commented out
- **WHEN** `readConfig()` is called
- **THEN** `config.mode` equals `"single"`
- **AND** `config.metricsRepo` equals `""`

#### Scenario: Config file sets mode=multi without metrics_repo
- **GIVEN** `~/.tu.conf` contains `mode = multi` but no `metrics_repo`
- **WHEN** `readConfig()` is called
- **THEN** `config.mode` equals `"single"` (mode field is ignored, no metrics_repo means single)
- **AND** no warning is emitted (the old `mode=multi without metrics_repo` warning is removed)

#### Scenario: Config file sets mode=single with metrics_repo
- **GIVEN** `~/.tu.conf` contains `mode = single` and `metrics_repo = git@github.com:user/repo.git`
- **WHEN** `readConfig()` is called
- **THEN** `config.mode` equals `"multi"` (mode field is ignored, metrics_repo presence drives mode)

## Configuration: TU_METRICS_REPO Environment Variable

### Requirement: TU_METRICS_REPO env var SHALL override config file metrics_repo

When `process.env.TU_METRICS_REPO` is set and non-empty, it SHALL take precedence over both `tu.default.conf` and `~/.tu.conf` values for `metrics_repo`. The precedence order is:

1. `TU_METRICS_REPO` env var (highest)
2. `~/.tu.conf` `metrics_repo` field
3. `tu.default.conf` `metrics_repo` field (lowest)

#### Scenario: Env var enables multi mode without config
- **GIVEN** `~/.tu.conf` has no `metrics_repo`
- **AND** `TU_METRICS_REPO=git@github.com:team/metrics.git` is set
- **WHEN** `readConfig()` is called
- **THEN** `config.metricsRepo` equals `"git@github.com:team/metrics.git"`
- **AND** `config.mode` equals `"multi"`

#### Scenario: Env var overrides config file value
- **GIVEN** `~/.tu.conf` has `metrics_repo = git@github.com:old/repo.git`
- **AND** `TU_METRICS_REPO=git@github.com:new/repo.git` is set
- **WHEN** `readConfig()` is called
- **THEN** `config.metricsRepo` equals `"git@github.com:new/repo.git"`

#### Scenario: Empty env var does not override
- **GIVEN** `~/.tu.conf` has `metrics_repo = git@github.com:user/repo.git`
- **AND** `TU_METRICS_REPO=""` (set but empty)
- **WHEN** `readConfig()` is called
- **THEN** `config.metricsRepo` equals `"git@github.com:user/repo.git"` (env var ignored when empty)

## Configuration: Remove WEAVER_DEV

### Requirement: WEAVER_DEV env var and tu.default.weaver.conf SHALL be removed

The `WEAVER_DEV` env var check in `config.ts` SHALL be removed. `DEFAULT_CONFIG_PATH` SHALL always resolve to `tu.default.conf`. The file `tu.default.weaver.conf` SHALL be deleted from the repository.

#### Scenario: DEFAULT_CONFIG_PATH without WEAVER_DEV
- **GIVEN** the `WEAVER_DEV` env var branching has been removed
- **WHEN** `DEFAULT_CONFIG_PATH` is evaluated
- **THEN** it resolves to `{projectRoot}/tu.default.conf` unconditionally

## Configuration: Default Config File

### Requirement: tu.default.conf SHALL NOT contain a mode field

The `mode = single` line SHALL be removed from `tu.default.conf`. The `metrics_repo` line remains commented out (absence of metrics_repo naturally produces single mode). Config version remains 2.

#### Scenario: Default config produces single mode
- **GIVEN** `tu.default.conf` has no `mode` line and `metrics_repo` is commented out
- **WHEN** a fresh install runs without `~/.tu.conf`
- **THEN** `config.mode` equals `"single"`
- **AND** `config.metricsRepo` equals `""`

## Configuration: init-conf Scaffold

### Requirement: init-conf scaffold SHALL NOT include a mode field

The `FIELD_BLOCKS` record in `cli.ts` SHALL remove the `mode` entry. When `tu init-conf` scaffolds a new `~/.tu.conf`, the generated file SHALL not contain a `mode` line. The `metrics_repo` block comment SHALL be updated to mention `TU_METRICS_REPO` as an alternative.

#### Scenario: New scaffold has no mode field
- **GIVEN** `~/.tu.conf` does not exist
- **WHEN** `tu init-conf` runs
- **THEN** the generated file contains `version`, `metrics_repo` (commented), `metrics_dir`, `machine`, `user`, `auto_sync`
- **AND** does not contain a `mode` line

## Configuration: Error Message Updates

### Requirement: Error messages SHALL reference metrics_repo instead of mode=multi

The `runInitMetrics` error for non-multi mode SHALL reference setting `metrics_repo` (or `TU_METRICS_REPO`) instead of `mode=multi`. The `runSync` error SHALL similarly be updated.

#### Scenario: runSync error message
- **GIVEN** config has no `metrics_repo` set
- **WHEN** `tu sync` is invoked
- **THEN** stderr shows a message referencing `metrics_repo` or `TU_METRICS_REPO`
- **AND** does not mention `mode=multi`

#### Scenario: runInitMetrics error message
- **GIVEN** config has no `metrics_repo` set
- **WHEN** `tu init-metrics` is invoked
- **THEN** stderr shows a message referencing `metrics_repo` or `TU_METRICS_REPO`
- **AND** does not mention `mode=multi`

## Deprecated Requirements

### mode config field

**Reason**: Redundant — `mode` is now derived from `metrics_repo` presence. The explicit `mode` field in config files served no purpose that `metrics_repo` doesn't already convey.
**Migration**: Remove `mode` lines from config files (or leave them — they're silently ignored).

### WEAVER_DEV env var

**Reason**: Project-specific (wvrdz) configuration mechanism. Replaced by generic `TU_METRICS_REPO` env var.
**Migration**: Replace `WEAVER_DEV=1` with `TU_METRICS_REPO=git@github.com:wvrdz/tu-metrics.git`.

### tu.default.weaver.conf

**Reason**: Only existed to provide wvrdz-specific defaults when `WEAVER_DEV` was set. No longer needed.
**Migration**: Use `TU_METRICS_REPO` env var or `~/.tu.conf` `metrics_repo` field.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Config version stays at 2 | Confirmed from intake #1 — no schema change | S:95 R:90 A:95 D:95 |
| 2 | Certain | TuConfig interface keeps mode field | Confirmed from intake #2 — downstream consumers unchanged | S:95 R:85 A:95 D:95 |
| 3 | Certain | Existing mode= lines in user configs silently ignored | Confirmed from intake #3 — parser reads but readConfig doesn't use | S:90 R:90 A:90 D:90 |
| 4 | Certain | TU_METRICS_REPO takes precedence over config file | Confirmed from intake #4 | S:95 R:85 A:90 D:95 |
| 5 | Certain | tu.default.weaver.conf is deleted | Confirmed from intake #5 | S:95 R:80 A:90 D:95 |
| 6 | Certain | brew install hardcoding is out of scope | Confirmed from intake #6 | S:95 R:95 A:95 D:95 |
| 7 | Certain | Error messages reference metrics_repo/TU_METRICS_REPO | Upgraded from intake Confident #7 — logically required by removing mode from config | S:85 R:85 A:85 D:90 |
| 8 | Certain | Empty TU_METRICS_REPO does not override config | Standard env var convention — empty string treated as unset | S:80 R:90 A:90 D:90 |
| 9 | Certain | Old mode=multi-without-metrics_repo warning is removed | State is now impossible — no metrics_repo means single, with metrics_repo means multi | S:85 R:85 A:95 D:95 |

9 assumptions (9 certain, 0 confident, 0 tentative, 0 unresolved).
