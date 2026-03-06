# Configuration System

## Overview

Configuration is managed via INI-style `.conf` files (`src/config.ts`). A layered system merges defaults from the package (`tu.default.conf`) with user overrides (`~/.tu.conf`). The `WEAVER_DEV` env var switches the default config to `tu.default.weaver.conf` (pre-configured for multi mode with the wvrdz metrics repo).

## Requirements

- Config MUST be read from `~/.tu.conf` with fallback to package defaults
- Config format MUST be INI-style: `key = value`, `#` comments, blank lines ignored
- User fields MUST override default fields (simple merge)
- Config version MUST be tracked (`version` field, current: 2); warn if newer than supported
- Sentinel values MUST be expanded at runtime: `$HOSTNAME` -> `os.hostname()`, `$USER` -> `os.userInfo().username`
- `~` prefix in paths MUST be resolved to `homedir()`
- `TuConfig` interface fields: `version`, `mode` (single/multi), `metricsRepo`, `metricsDir`, `machine`, `user`, `autoSync`
- `mode=multi` without `metrics_repo` MUST warn and fall back to single
- `auto_sync` MUST default to true; only `"false"` or `"0"` disable it
- `init-conf` MUST scaffold `~/.tu.conf` from defaults if missing, or append missing fields if present
- `init-conf` MUST detect commented-out fields and suggest uncommenting them
- `status` command MUST display mode, user, machine, config path, metrics path, last sync time, auto-sync state
- Last sync time MUST be formatted as relative time (e.g., "3h ago") with ISO timestamp

## Design Decisions

- **INI over YAML/JSON**: Simpler to hand-edit, no indentation issues, trivial to parse. Good fit for a small number of flat config fields.
- **Layered defaults**: The package ships `tu.default.conf` so the tool works out of the box in single mode. Users only need to override fields they want to change. `tu.default.weaver.conf` provides team-specific defaults.
- **Sentinel expansion**: `$HOSTNAME` and `$USER` sentinels allow the same config file to work across machines without per-machine customization.
- **Version field**: Enables future config migrations. Currently only warns on newer versions.
- **Home directory `~/.tu/`**: All runtime state (cache, metrics, sync markers) lives under `~/.tu/`. Config file is at `~/.tu.conf` (top level, not nested).

## Changelog

| Date | Change |
|------|--------|
| 2026-03-06 | Generated from code analysis |
