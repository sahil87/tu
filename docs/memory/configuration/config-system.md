---
type: memory
description: INI config format, $HOME-only path resolution (~/.config/tu/tu.conf), cascade tu.default.conf < org.conf < tu.conf < TU_METRICS_REPO < CLI, legacy ~/.tu.conf fallback with deprecation warning, sentinel expansion, init-conf/init-metrics scaffolding
---

# Configuration System

## Overview

Configuration is managed via INI-style `.conf` files (`src/node/core/config.ts`). The user config lives at `$HOME/.config/tu/tu.conf`, resolved lazily from `process.env.HOME` only. A fixed cascade merges the shipped defaults (`tu.default.conf`), an optional org layer (`$HOME/.config/tu/org.conf`), the user conf (with a legacy `~/.tu.conf` fallback), the `TU_METRICS_REPO` env var, and CLI-argument overrides.

## Requirements

- Config paths MUST be resolved by `resolveConfigPaths(env = process.env): ConfigPaths`, returning `{ configDir, userConf, orgConf, legacyConf }` = `$HOME/.config/tu`, `$HOME/.config/tu/tu.conf`, `$HOME/.config/tu/org.conf`, `$HOME/.tu.conf`, backed by the `TOOL_NAME`/`CONFIG_FILE`/`ORG_CONFIG_FILE`/`LEGACY_CONFIG_FILE` constants
- The resolver MUST build paths from `process.env.HOME` and nothing else — no `XDG_CONFIG_HOME`, no `TU_CONFIG*`/`TU_HOME`, no `os.homedir()` passwd fallback. Unset or empty `$HOME` MUST throw `ConfigHomeError` (`tu: $HOME is not set; cannot locate config`); config-reading commands surface it on stderr and exit `1` (operational/environment error — recovery is fixing the environment)
- Resolution MUST be lazy (a function call at config-read time, not an import-time constant), so commands that never touch config (`help`, `--version`, `help-dump`, `skill`, `shell-init`, `update`) keep working with `$HOME` unset
- A test MUST pin the resolved path against environment variables: with `HOME` set, `XDG_CONFIG_HOME`/`TU_CONFIG`/`TU_CONFIG_HOME` pointing elsewhere MUST NOT move `userConf` (`src/node/core/__tests__/config.test.ts`)
- The merge cascade MUST be exactly: `tu.default.conf` (shipped beside the bundle, located via `findDefaultConf`) < `$HOME/.config/tu/org.conf` (optional; absence is silent — no warning, no log) < user conf (selection rule below) < env `TU_METRICS_REPO` (`metrics_repo` only — the sole config-bearing env key) < the CLI-overrides argument (`readConfig`'s third parameter). No per-key inversions: a key set in `org.conf` is always overridden by the same key in `tu.conf`, which is always overridden by env, which is always overridden by a CLI argument
- `org.conf` MUST use the identical `key = value` / `#` comment format, parsed by the same `parseConf`/`readConfFile`
- User-conf selection MUST follow: `~/.config/tu/tu.conf` exists → read it (legacy silently ignored); else `~/.tu.conf` exists → read it and emit exactly one stderr line per process per legacy path, `tu: ~/.tu.conf is deprecated; move it to ~/.config/tu/tu.conf` (guarded by a module-level `Set<string>` — `readConfig` is called from several paths and watch mode re-reads); else defaults + org only. The warning MUST go to stderr only, in every mode, so `--json`/`--csv`/`--md` stdout stays clean. The legacy file MUST NOT be moved or deleted on read (no auto-migration)
- `readConfig(paths: ConfigPaths | string = resolveConfigPaths(), defaultsPath, overrides)` MUST keep its injectable shape; the bare-string first-argument form is treated as userConf only (no org layer, no legacy fallback) for existing tests
- Config format MUST be INI-style: `key = value`, `#` comments, blank lines ignored
- Config version MUST be tracked (`version` field, current: 2); warn if newer than supported, naming the path actually read (user conf, else org conf when present, else the defaults path)
- Sentinel values MUST be expanded at runtime: `$HOSTNAME` -> `os.hostname()`, `$USER` -> `os.userInfo().username`
- `~` prefix in paths MUST be resolved to `homedir()` (`resolveHome`, used for `metrics_dir` — state paths are not bound by the config-path rule)
- `TuConfig` interface fields: `version`, `mode` (single/multi, derived), `metricsRepo`, `metricsDir`, `machine`, `user`, `autoSync`
- `mode` MUST be derived from `metricsRepo` presence: non-empty → `"multi"`, empty → `"single"`
- `TU_METRICS_REPO` env var (when non-empty) MUST take precedence over config-file `metrics_repo`; a CLI-layer override beats the env var
- `mode` field in config files MUST be silently ignored (backward compat)
- `auto_sync` MUST default to true; only `"false"` or `"0"` disable it
- `init-conf` MUST write/update `~/.config/tu/tu.conf` (never `~/.tu.conf`) via the shared `ensureUserConf` helper (also used by `init-metrics`): creating the config dir with `mkdirSync(..., { recursive: true })`; when creating the file and a legacy `~/.tu.conf` exists, the new file MUST be seeded from the legacy contents with a stdout `Copied ~/.tu.conf → ~/.config/tu/tu.conf` note; otherwise it is copied from `tu.default.conf` with `Created ~/.config/tu/tu.conf — edit it to configure multi-machine sync.`
- `init-conf` MUST detect commented-out fields and suggest uncommenting them; the scaffold MUST NOT include a `mode` field
- `status` MUST display mode, user, machine, config path, metrics path, last sync time, auto-sync state. The config path shown MUST be the one the selection rule actually read — `~/.config/tu/tu.conf`, or `~/.tu.conf` under the legacy fallback (its deprecation line goes to stderr as usual); when neither exists: `Mode:        single (no ~/.config/tu/tu.conf)`
- `status` MUST print `Org config:  ~/.config/tu/org.conf` directly after the `Config:` line (both single and multi layouts) only when `org.conf` exists; in an org-only setup (no user/legacy file) the `Config:` line is omitted but the Org line still prints; with no org.conf the layout stays byte-identical to the no-org form
- Last sync time MUST be formatted as relative time (e.g., "3h ago") with ISO timestamp

## Design Decisions

- **INI over YAML/JSON**: Simpler to hand-edit, no indentation issues, trivial to parse. Good fit for a small number of flat config fields.
- **Layered defaults**: The package ships `tu.default.conf` so the tool works out of the box in single mode. Users only need to override fields they want to change.
- **Derived mode**: `mode` is computed from `metricsRepo !== ""` rather than stored as a config field. This eliminates the redundancy where `mode=multi` without `metrics_repo` was meaningless, and makes the config surface smaller.
- **TU_METRICS_REPO env var**: Replaces the old `WEAVER_DEV` mechanism. Any user or CI can set `TU_METRICS_REPO` to enable multi mode without editing config files. Empty string is treated as unset.
- **Sentinel expansion**: `$HOSTNAME` and `$USER` sentinels allow the same config file to work across machines without per-machine customization.
- **Version field**: Enables future config migrations. Currently only warns on newer versions.
- **Home directory `~/.tu/` for state, `~/.config/tu/` for config**: All runtime state (cache, metrics repo clone, sync markers) lives under `~/.tu/` (`TU_HOME`). Config files live under `$HOME/.config/tu/` per the config-home rule — the two roots are intentionally separate.

### Fixed config root with no XDG honor
**Decision**: Resolve config as `$HOME/.config/tu/tu.conf`, built from `$HOME` only; no `$XDG_CONFIG_HOME`, no `os.homedir()` fallback; unset `$HOME` is an actionable error.
**Why**: The binding shll `config-home` standard requires one environment-independent root so daemon/CLI/agent contexts provably read the same file; an env-movable path forks behavior silently across process contexts.
**Rejected**: XDG honor (`$XDG_CONFIG_HOME`, `os.UserConfigDir`) — conventional but non-deterministic across launchd/systemd/shell/tmux contexts; a dotfiles user who wants it elsewhere symlinks.
*Introduced by*: 260827-gzrn-config-home-conformance-org-layer

### Fallback-plus-warning over auto-migration
**Decision**: Legacy `~/.tu.conf` is read (with a one-line stderr deprecation warning, once per process per path) only when the new file is absent; it is never moved or deleted by tu. The legacy fallback is intended to be dropped after at least two minor versions.
**Why**: Auto-migration is a silent write to the user's home with rollback surprises (dotfile-managed or symlinked confs); read-plus-warn is cheaper, reversible, and conforms to Constitution II (warn on stderr, keep working).
**Rejected**: Read-time auto-migration of `~/.tu.conf` → `~/.config/tu/tu.conf` — silent home-directory mutation with surprise failure modes.
*Introduced by*: 260827-gzrn-config-home-conformance-org-layer

### Org layer over org detection
**Decision**: An optional `$HOME/.config/tu/org.conf` participates in the cascade (`defaults < org.conf < tu.conf < env < CLI`); absence is silent. No org-detection heuristics.
**Why**: An org's dotfiles/MDM/bootstrap script drops the file once and every employee's tu is in multi mode with zero per-user edits, while personal overrides still win; heuristics would hardcode a company into a public tool, add startup latency, and not escape the bootstrap problem.
**Rejected**: Org detection via email domain / SSH alias / network probes — company-specific, latent, and still requires bootstrap.
*Introduced by*: 260827-gzrn-config-home-conformance-org-layer

### Write-time legacy seeding
**Decision**: When tu creates `~/.config/tu/tu.conf` (via `init-conf` or `init-metrics <url>`) and a legacy `~/.tu.conf` exists, the new file is seeded from the legacy contents (with a stdout `Copied …` note) rather than from `tu.default.conf`.
**Why**: Seeding avoids silently orphaning the user's `machine`/`user`/`metrics_dir` overrides the moment they run the new one-liner; it is a write tu is already making, so it does not reintroduce the rejected read-time auto-migration.
**Rejected**: Always scaffolding from `tu.default.conf` — simpler, but loses existing user settings.
*Introduced by*: 260827-gzrn-config-home-conformance-org-layer
