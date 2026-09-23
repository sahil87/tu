---
type: memory
description: Config path resolution from $HOME only (ResolvePaths/ErrNoHome), INI parsing (ParseConf), the five-layer Load cascade (defaults ∪ org ∪ user ∪ TU_METRICS_REPO ∪ CLI) with legacy and newer-version warnings, embedded drift-guarded tu.default.conf, sentinel/~ expansion, StateDir/Tildefy/ExpandHome
---

# Config Cascade

**Domain**: config

## Overview

`internal/config` resolves all configuration from `$HOME` alone and merges five layers — embedded defaults, `org.conf`, the user conf (with a legacy fallback), the `TU_METRICS_REPO` environment variable, and CLI overrides — into a `Config` value plus ordered stderr warning lines. The package writes nothing to any stream; [command/entry-point](/command/entry-point.md) prints the warnings.

## Requirements

### Requirement: Path resolution from $HOME only
`ResolvePaths(home string)` returns `Paths{Home, ConfigDir, UserConf, OrgConf, LegacyConf}` — `$HOME/.config/tu`, `$HOME/.config/tu/tu.conf`, `$HOME/.config/tu/org.conf`, `$HOME/.tu.conf` — built from the `home` argument and nothing else: no `$XDG_CONFIG_HOME`, no `TU_*` path override, no passwd fallback. An empty home MUST return `ErrNoHome`, whose message is byte-exact: `tu: $HOME is not set; cannot locate config` (the edge prints it verbatim, exit 1).

#### Scenario: $HOME unset
- **GIVEN** an empty home string
- **WHEN** `ResolvePaths("")` runs
- **THEN** it returns the zero `Paths` and `ErrNoHome`

### Requirement: State root and path helpers
`StateDir(home)` is the runtime-state root `$HOME/.tu` — the cache ([source/cache](/source/cache.md)), the metrics-repo clone, `.last-sync`, `.clone-failed` — intentionally separate from the `$HOME/.config/tu` config root. `Tildefy(p, home)` abbreviates a path under home to `~/…` (a prefix match on the home string, no trailing-slash handling); other paths pass through unchanged. `ExpandHome(p, home)` resolves a leading `~/` or a bare `~` against home; anything else passes through.

### Requirement: INI parsing
`ParseConf(raw)` in `internal/config/config.go` MUST trim each line, skip blank lines and lines starting with `#`, split at the FIRST `=`, and trim key and value; later keys overwrite earlier ones. Lines without `=` are ignored. The same parser feeds every file layer.

### Requirement: The five-layer Load cascade
`Load(p Paths, env Env, ov Overrides)` returns the merged `Config`, the stderr warning lines in emission order, and never an error — an unreadable file is an empty layer (`readConf` → empty map, false). The merge order MUST be: `DefaultConf` (embedded) < `org.conf` < user conf (selection rule below), with later keys winning per key; then `metrics_repo` alone is overridden by `TU_METRICS_REPO` when the env var is non-empty, then by `ov.MetricsRepo` when non-nil — the CLI layer beats the env var even when the override is the empty string. `TU_METRICS_REPO` is the only config-bearing environment variable. `Env` injects `Getenv`/`Hostname`/`Username` so tests pin the process view. `Mode` is `Multi` iff the final `MetricsRepo` is non-empty (`Single` otherwise); `AutoSync` is false only when `auto_sync` is exactly `false` or `0`.

#### Scenario: Layer precedence
- **GIVEN** `org.conf` with `metrics_repo = A`, `tu.conf` with `metrics_repo = B`, env `TU_METRICS_REPO=C`, and a CLI override pointing at `D`
- **WHEN** `Load` runs
- **THEN** `MetricsRepo == "D"`; without the override it is `C`; without the env it is `B`; without `tu.conf` it is `A`

### Requirement: User-conf selection and the legacy deprecation warning
`selectUserConf` in `internal/config/config.go` MUST prefer `UserConf` when readable (the legacy file is silently ignored); else `LegacyConf` when readable, adding the literal warning line `tu: ~/.tu.conf is deprecated; move it to ~/.config/tu/tu.conf`; else neither file feeds the merge. "Readable" means `os.ReadFile` succeeds — an existing-but-unreadable file falls back. `Load` reports the path actually read as `Config.UserConfRead` and org readability as `Config.OrgRead`; warnings return in emission order (deprecation first, version warning second). The legacy file MUST NOT be moved or deleted on read. `selectUserConfPath` reports the path the selection rule would read so [status](/config/status.md) mirrors the selection exactly.

### Requirement: Version parsing and the newer-version warning
`Load` parses the merged `version` with JavaScript `parseInt(s, 10)` semantics via `parseIntJS` — optional leading whitespace and sign, then the longest run of ASCII digits (`"2abc"` → 2); no digits or a missing key yields 1. When the parsed version exceeds `CurrentConfigVersion` (2), the warning MUST be `Warning: {source} version {N} is newer than tu supports (2). Please update tu.` where `{source}` is `UserConfRead` when non-empty, else `p.OrgConf` when the org layer was read, else `DefaultConfName` — an absolute path, never tildefied.

#### Scenario: Unparseable version
- **GIVEN** `tu.conf` containing `version = abc`
- **WHEN** `Load` runs
- **THEN** `Version` is 1 and no newer-version warning is emitted

### Requirement: Embedded shipped defaults with a drift guard
`internal/config/defaults.go` embeds the sibling `tu.default.conf` into `DefaultConf` via `//go:embed`, so the binary carries no runtime file dependency. The embedded copy MUST stay byte-identical to the repo-root `tu.default.conf`; `TestDefaultConfDriftGuard` in `defaults_test.go` walks up from the package directory to the directory containing `package.json` and asserts byte equality. `DefaultConfName` is `"tu.default.conf"`, the source string the newer-version warning prints for the defaults layer. `Load` uses `DefaultConf` as its defaults layer; [InitConf](/config/setup-commands.md) writes it when scaffolding from defaults.

### Requirement: Sentinel and tilde expansion
`expandSentinels` replaces a value that is exactly `$HOSTNAME` with the hostname (error → `""`) and exactly `$USER` with the username (error → `unknown`); any other value — including those merely containing a sentinel as a substring — passes through unchanged. `Load` applies it to `metrics_dir` (default `~/.tu/metrics_repo`, then `ExpandHome` against `Home`), `machine` (default `$HOSTNAME`), and `user` (default `$USER`), with an empty value falling back to the default (`orDefault`).

## Design Decisions

### Fixed config root with no XDG honor
**Decision**: Resolve config as `$HOME/.config/tu/tu.conf`, built from `$HOME` only; no `$XDG_CONFIG_HOME`, no `os.UserConfigDir` fallback; unset `$HOME` is an actionable error.
**Why**: The binding shll `config-home` standard requires one environment-independent root so daemon/CLI/agent contexts provably read the same file; an env-movable path forks behavior silently across process contexts.
**Rejected**: XDG honor — conventional but non-deterministic across launchd/systemd/shell/tmux contexts; a dotfiles user who wants it elsewhere symlinks.
*Introduced by*: 260827-gzrn-config-home-conformance-org-layer (gzrn)

### Fallback-plus-warning over auto-migration
**Decision**: Legacy `~/.tu.conf` is read, with a one-line stderr deprecation warning, only when the new file is absent; it is never moved or deleted by tu.
**Why**: Auto-migration is a silent write to the user's home with rollback surprises (dotfile-managed or symlinked confs); read-plus-warn is cheaper, reversible, and keeps stderr the warning channel while stdout stays clean.
**Rejected**: Read-time auto-migration of `~/.tu.conf` → `~/.config/tu/tu.conf` — silent home-directory mutation with surprise failure modes.
*Introduced by*: 260827-gzrn-config-home-conformance-org-layer (gzrn)

### Org layer over org detection
**Decision**: An optional `$HOME/.config/tu/org.conf` participates in the cascade (`defaults < org.conf < tu.conf < env < CLI`); absence is silent. No org-detection heuristics.
**Why**: An org's dotfiles/MDM/bootstrap script drops the file once and every employee's tu is in multi mode with zero per-user edits, while personal overrides still win; heuristics would hardcode a company into a public tool, add startup latency, and not escape the bootstrap problem.
**Rejected**: Org detection via email domain / SSH alias / network probes — company-specific, latent, and still requires bootstrap.
*Introduced by*: 260827-gzrn-config-home-conformance-org-layer (gzrn)

### Embedded shipped defaults
**Decision**: `tu.default.conf` is embedded into the binary from a byte-identical copy under `internal/config`, drift-guarded by `TestDefaultConfDriftGuard`.
**Why**: No runtime file lookup, no silent empty-defaults failure mode when the file is missing, and tests need no path injection.
**Rejected**: A beside-the-binary then walk-up runtime lookup — a runtime dependency on a file the tarball would otherwise not need, and a missing file degrades `init-conf` to an empty scaffold.
*Introduced by*: 260916-4fs0-config-and-setup-commands (4fs0)

### JavaScript parseInt semantics for the version field
**Decision**: `parseIntJS` reproduces `parseInt(s, 10)` — whitespace/sign prefix, longest digit run, fallback to version 1 on no digits — instead of Go's stricter `strconv.Atoi` on the whole string.
**Why**: Parity with the frozen `src/node/` oracle (until plan row Z1); the harness feeds values like `"2abc"` that the oracle accepts as 2.
**Rejected**: Whole-string integer parsing — rejects inputs the oracle accepts and changes which confs trigger the newer-version warning.
*Introduced by*: 260916-4fs0-config-and-setup-commands (4fs0)
