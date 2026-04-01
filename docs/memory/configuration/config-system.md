# Configuration System

## Overview

Configuration is managed via INI-style `.conf` files (`src/node/core/config.ts`). A layered system merges defaults from the package (`tu.default.conf`) with user overrides (`~/.tu.conf`). The `TU_METRICS_REPO` env var can override `metrics_repo` from config files, enabling multi-machine mode without editing config.

## Requirements

- Config MUST be read from `~/.tu.conf` with fallback to package defaults
- Config format MUST be INI-style: `key = value`, `#` comments, blank lines ignored
- User fields MUST override default fields (simple merge)
- Config version MUST be tracked (`version` field, current: 2); warn if newer than supported
- Sentinel values MUST be expanded at runtime: `$HOSTNAME` -> `os.hostname()`, `$USER` -> `os.userInfo().username`
- `~` prefix in paths MUST be resolved to `homedir()`
- `TuConfig` interface fields: `version`, `mode` (single/multi, derived), `metricsRepo`, `metricsDir`, `machine`, `user`, `autoSync`
- `mode` MUST be derived from `metricsRepo` presence: non-empty → `"multi"`, empty → `"single"`
- `TU_METRICS_REPO` env var (when non-empty) MUST take precedence over config file `metrics_repo`
- `mode` field in config files MUST be silently ignored (backward compat)
- `auto_sync` MUST default to true; only `"false"` or `"0"` disable it
- `init-conf` MUST scaffold `~/.tu.conf` from defaults if missing, or append missing fields if present
- `init-conf` MUST detect commented-out fields and suggest uncommenting them
- `init-conf` scaffold MUST NOT include a `mode` field
- `status` command MUST display mode, user, machine, config path, metrics path, last sync time, auto-sync state
- Last sync time MUST be formatted as relative time (e.g., "3h ago") with ISO timestamp

## Design Decisions

- **INI over YAML/JSON**: Simpler to hand-edit, no indentation issues, trivial to parse. Good fit for a small number of flat config fields.
- **Layered defaults**: The package ships `tu.default.conf` so the tool works out of the box in single mode. Users only need to override fields they want to change.
- **Derived mode**: `mode` is computed from `metricsRepo !== ""` rather than stored as a config field. This eliminates the redundancy where `mode=multi` without `metrics_repo` was meaningless, and makes the config surface smaller.
- **TU_METRICS_REPO env var**: Replaces the old `WEAVER_DEV` mechanism. Any user or CI can set `TU_METRICS_REPO` to enable multi mode without editing config files. Empty string is treated as unset.
- **Sentinel expansion**: `$HOSTNAME` and `$USER` sentinels allow the same config file to work across machines without per-machine customization.
- **Version field**: Enables future config migrations. Currently only warns on newer versions.
- **Home directory `~/.tu/`**: All runtime state (cache, metrics, sync markers) lives under `~/.tu/`. Config file is at `~/.tu.conf` (top level, not nested).

## Changelog

| Date | Change |
|------|--------|
| 2026-04-01 | Removed WEAVER_DEV env var and tu.default.weaver.conf. Mode now derived from metrics_repo presence. Added TU_METRICS_REPO env var override. Removed mode from config file format and init-conf scaffold. |
| 2026-03-06 | Generated from code analysis |
