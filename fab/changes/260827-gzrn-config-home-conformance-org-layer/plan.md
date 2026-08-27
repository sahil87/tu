# Plan: Config-home conformance + org config layer

**Change**: 260827-gzrn-config-home-conformance-org-layer
**Intake**: `intake.md`

## Requirements

### Configuration: Path Resolution

#### R1: Fixed config root built from `$HOME` only
Config path resolution SHALL be `resolveConfigPaths(env = process.env): ConfigPaths` in `src/node/core/config.ts`, returning `{ configDir, userConf, orgConf, legacyConf }` = `$HOME/.config/tu`, `$HOME/.config/tu/tu.conf`, `$HOME/.config/tu/org.conf`, `$HOME/.tu.conf`. The resolver MUST use `process.env.HOME` and nothing else — no `XDG_CONFIG_HOME`, no `TU_CONFIG*`/`TU_HOME`, no `os.homedir()` passwd fallback. Constants `TOOL_NAME`, `CONFIG_FILE`, `ORG_CONFIG_FILE`, `LEGACY_CONFIG_FILE` SHALL back the path construction. The eager module-level `CONFIG_PATH` constant MUST be removed.

- **GIVEN** any values for `XDG_CONFIG_HOME`, `TU_CONFIG`, `TU_CONFIG_HOME`
- **WHEN** `resolveConfigPaths()` runs with `HOME=/some/dir`
- **THEN** `userConf` is `/some/dir/.config/tu/tu.conf` regardless of the other variables

#### R2: Lazy resolution with an actionable unset-`$HOME` error
Resolution MUST be lazy (function call at config-read time, not import time). When `$HOME` is unset or empty the resolver MUST throw a `ConfigHomeError` whose message is `tu: $HOME is not set; cannot locate config`; config-reading commands surface it on stderr and exit `1` (operational/environment error). Commands that never touch config (`tu help`, `tu --version`, `tu help-dump`, `tu skill`, `tu shell-init`, `tu update`) MUST keep working with `$HOME` unset.

- **GIVEN** `HOME` is unset in the environment
- **WHEN** `tu --version` runs
- **THEN** it prints the version and exits 0
- **AND WHEN** a config-reading command (e.g. `tu init-conf`) runs
- **THEN** stderr shows `tu: $HOME is not set; cannot locate config` and the exit code is 1

### Configuration: Cascade & Legacy Fallback

#### R3: Org layer and exact cascade order
`readConfig` MUST merge in exactly this order: `tu.default.conf` < `$HOME/.config/tu/org.conf` (optional, silent when absent, same `key = value` format via the same `parseConf`/`readConfFile`) < user conf < env `TU_METRICS_REPO` (metrics_repo only) < CLI overrides argument. No per-key inversions. `readConfig` gains an injectable org path and an `overrides` parameter (CLI-flag layer); the bare-string first argument form is retained for existing tests and treated as userConf only (no org, no legacy fallback).

- **GIVEN** an `org.conf` setting `metrics_repo = org.git` and no user conf
- **WHEN** `readConfig(paths)` runs
- **THEN** mode is `multi` with `metricsRepo === "org.git"`
- **AND GIVEN** `tu.conf` sets `metrics_repo = user.git`
- **THEN** `metricsRepo === "user.git"` (user beats org)
- **AND GIVEN** `TU_METRICS_REPO=env.git` is exported
- **THEN** `metricsRepo === "env.git"` (env beats file)
- **AND GIVEN** an overrides argument `{ metrics_repo: "cli.git" }`
- **THEN** `metricsRepo === "cli.git"` (CLI beats env)

#### R4: Legacy `~/.tu.conf` fallback with once-per-process deprecation warning
User-conf selection MUST follow: `~/.config/tu/tu.conf` exists → read it (legacy silently ignored); else `~/.tu.conf` exists → read it and emit exactly one stderr line per process per legacy path, `tu: ~/.tu.conf is deprecated; move it to ~/.config/tu/tu.conf`; else defaults + org only. No auto-migration (no moving/deleting the legacy file on read). The warning goes to stderr only, in every mode, so stdout formats stay clean.

- **GIVEN** only `~/.tu.conf` exists
- **WHEN** `readConfig` is called twice in one process
- **THEN** both calls read the legacy values and exactly one deprecation line lands on stderr
- **AND GIVEN** both files exist
- **THEN** the new file is read and no warning is emitted

#### R5: Unchanged behaviors and path-accurate version warning
`resolveHome`, `expandSentinels`, sentinel behavior, `mode` derivation from `metrics_repo` presence, `auto_sync` parsing, the `metrics_dir` default `~/.tu/metrics_repo`, `TU_HOME`, and `~/.tu/` state MUST NOT change. The version-too-new warning MUST name the path actually read (user conf, else org conf, else defaults) instead of the hardcoded `~/.tu.conf`.

- **GIVEN** a user conf with `version = 99` at `~/.config/tu/tu.conf`
- **WHEN** `readConfig` runs
- **THEN** stderr warns `Warning: ~/.config/tu/tu.conf version 99 is newer than tu supports (2). Please update tu.`

### CLI: `tu init-metrics [repo-url]`

#### R6: URL argument writes `metrics_repo` then clones
Grammar: `tu init-metrics [<repo-url>]`. With a URL, the command MUST: resolve config paths (unset `$HOME` → the R2 error, exit 1); ensure `~/.config/tu/tu.conf` exists via a shared `ensureUserConf` helper (creating the dir with `mkdirSync(..., { recursive: true })`; when creating and a legacy `~/.tu.conf` exists, seed the new file from the legacy contents and print `Copied ~/.tu.conf → ~/.config/tu/tu.conf`); write `metrics_repo = <url>` (replace an active line in place, else replace the commented scaffold line, else append the `FIELD_BLOCKS.metrics_repo` block with the value filled) and print `Set metrics_repo = <url> in ~/.config/tu/tu.conf`; then run the existing clone behavior with the URL as the CLI-flag layer for that invocation (it MUST beat an exported `TU_METRICS_REPO`). Idempotency is preserved: an existing git-repo `metricsDir` prints `Already initialized: <dir>` (exit 0) after the config write. More than one positional argument is a usage error: stderr + `SHORT_USAGE`, exit `EXIT_USAGE` (2).

- **GIVEN** no config file and a local bare repo URL
- **WHEN** `tu init-metrics <url>` runs
- **THEN** `~/.config/tu/tu.conf` is created from the scaffold, `metrics_repo = <url>` is written, and the repo is cloned
- **AND GIVEN** `TU_METRICS_REPO` points elsewhere
- **THEN** the clone targets the typed URL
- **AND WHEN** `tu init-metrics a b` runs
- **THEN** exit code is 2 with the short usage on stderr

#### R7: No-argument behavior preserved with updated guidance
Without `<repo-url>` the behavior is unchanged except the error text: a missing `metrics_repo` (from org.conf, tu.conf, or `TU_METRICS_REPO`) prints `Error: metrics_repo is not set. Add it to ~/.config/tu/tu.conf, run 'tu init-metrics <repo-url>', or set TU_METRICS_REPO.` and exits 1.

- **GIVEN** no `metrics_repo` anywhere
- **WHEN** `tu init-metrics` runs without an argument
- **THEN** the new error text prints to stderr and exit code is 1

### CLI: `tu init-conf`

#### R8: init-conf writes the new path via the shared scaffold helper
`runInitConf` MUST write/update `~/.config/tu/tu.conf` (never `~/.tu.conf`), reusing the same `ensureUserConf` helper as `runInitMetrics` (including legacy seeding with the `Copied …` stdout note). Messages are unchanged except for the path they name.

- **GIVEN** no config dir exists and a legacy `~/.tu.conf` with custom fields
- **WHEN** `tu init-conf` runs
- **THEN** `~/.config/tu/tu.conf` is created with the legacy contents and stdout notes the copy
- **AND GIVEN** no legacy file
- **THEN** the new file is a copy of `tu.default.conf` and stdout says `Created ~/.config/tu/tu.conf — edit it to configure multi-machine sync.`

### CLI: `tu status`

#### R9: status shows the selected config path and an optional Org config line
`runStatus` MUST display the path the R4 rule selected (`~/.config/tu/tu.conf`, or `~/.tu.conf` under the legacy fallback). When neither exists: `Mode: single (no ~/.config/tu/tu.conf)`. When `org.conf` exists, an `Org config:  ~/.config/tu/org.conf` line MUST print directly after the `Config:` line in both single and multi layouts; when no user/legacy file was read, the `Config:` line is omitted but the Org line still prints. When `org.conf` is absent the layout stays byte-identical to today.

- **GIVEN** an org.conf exists and a user conf exists
- **WHEN** `tu status` runs
- **THEN** the `Org config:` line appears directly after `Config:`
- **AND GIVEN** no org.conf
- **THEN** no `Org config:` line appears

### User-facing strings and docs

#### R10: CLI strings name the new path
`FULL_HELP` MUST read `tu init-conf         Scaffold ~/.config/tu/tu.conf` and `tu init-metrics [url] Clone metrics repo (url also sets metrics_repo)`; the `tu sync` error MUST read `Add metrics_repo to ~/.config/tu/tu.conf, run 'tu init-metrics <repo-url>', or set TU_METRICS_REPO.`; the fish completion description MUST read `scaffold ~/.config/tu/tu.conf`; `tu.default.conf:2` MUST read `# User overrides go in ~/.config/tu/tu.conf (created by 'tu init-conf'). Org-wide defaults may be dropped in ~/.config/tu/org.conf.` `tu help-dump` MUST stay stderr-clean with exit 0.

- **GIVEN** the built CLI
- **WHEN** `tu help` runs
- **THEN** the Setup section shows the new path and the `[url]` argument

#### R11: Docs updated for the new path, org layer, and one-liner onboarding
`README.md` § Setup MUST use `tu init-metrics <repo-url>` as the one-liner and carry a short Team setup note on `org.conf`; `docs/site/install.md` §1–3 MUST be rewritten around `tu init-metrics <repo-url>` (keeping `init-conf` + edit as the alternative) with a **Config locations & precedence** subsection listing the cascade and the legacy-fallback rule; `docs/site/skill.md` MUST name the new path and `init-metrics [url]` while staying ≤150 lines; `docs/specs/usage.md` MUST update the `init-conf`/`init-metrics [repo-url]` grammar rows, retitle `### Configuration` to the new path with the cascade/org.conf/legacy fallback, document the init-metrics exit-2 row, and delete the stale `WEAVER_DEV`/`mode` rows; `docs/specs/layouts.md` status mockups MUST use the new path and add the optional `Org config:` line; `fab/project/context.md` config.ts row MUST read `Config file reading (~/.config/tu/tu.conf, org.conf layer)`. Only `.tu.conf` mentions change; `~/.tu/` (state) mentions stay. `docs/site/workflows.md` was verified to contain no `.tu.conf`/`init-metrics` recipe — no change needed there.

- **GIVEN** the docs tree
- **WHEN** searched for `~/.tu.conf`
- **THEN** no user-facing mention remains (memory files are hydrate's scope, not apply's)

### Tests

#### R12: Config pin, cascade, and legacy-fallback coverage
`src/node/core/__tests__/config.test.ts` MUST pin the resolved path against `XDG_CONFIG_HOME`/`TU_CONFIG`/`TU_CONFIG_HOME`, assert unset/empty `HOME` throws with `$HOME is not set`, and cover: org-only → multi; org + tu.conf disagreement → tu.conf wins; tu.conf + `TU_METRICS_REPO` → env wins; overrides argument beats env; legacy-only → read + exactly one deprecation line across two calls; both present → new file, no warning.

- **GIVEN** a temp HOME with `XDG_CONFIG_HOME` pointing elsewhere
- **WHEN** `resolveConfigPaths` runs
- **THEN** `userConf` is under the temp HOME's `.config/tu/`

#### R13: CLI and fixture coverage
`cli-init-metrics.test.ts` MUST cover: URL creates tu.conf from defaults + writes `metrics_repo` + clones; replacing active and commented `metrics_repo` lines; idempotent `Already initialized` after the write; URL beats `TU_METRICS_REPO`; two positional args → exit 2. `cli-init-conf.test.ts` MUST cover nested-path creation and legacy seeding. `cli-status.test.ts` MUST cover new/legacy/none path display and the Org line's presence rule. `cli-exit-codes.test.ts` MUST write its temp-HOME conf to `.config/tu/tu.conf` (not `.tu.conf`) so the deprecation line never pollutes stderr assertions.

- **GIVEN** the migrated test suite
- **WHEN** `env -u TU_METRICS_REPO npm test` runs
- **THEN** every test passes

### Non-Goals

- Auto-sync / stale-triggered sync — separate backlog item; `isStale` and the no-op `auto_sync` key are untouched.
- Moving `~/.tu/` state (cache, metrics_repo clone, `.last-sync`, clone marker) to an XDG state dir — `TU_HOME` and the `metrics_dir` default are untouched.
- Org-detection heuristics of any kind (email domain, SSH alias, network probes).
- Auto-migrating (moving/deleting) `~/.tu.conf` on read.
- Updating the shll repo's conformance list — separate one-line PR in that repo.
- Any new env var — `TU_METRICS_REPO` remains the only config-bearing env key.
- `package.json` version bump — ship-time concern (`just release minor`); this change does not edit it.

### Design Decisions

#### Fixed config root with no XDG honor
**Decision**: Resolve config as `$HOME/.config/tu/tu.conf`, built from `$HOME` only; no `$XDG_CONFIG_HOME`, no `os.homedir()` fallback; unset `$HOME` is an actionable error.
**Why**: The binding shll `config-home` standard requires one environment-independent root so daemon/CLI/agent contexts provably read the same file; an env-movable path forks behavior silently across process contexts.
**Rejected**: XDG honor (`$XDG_CONFIG_HOME`, `os.UserConfigDir`) — conventional but non-deterministic across launchd/systemd/shell/tmux contexts; a dotfiles user who wants it elsewhere symlinks.
*Introduced by*: 260827-gzrn-config-home-conformance-org-layer

#### Fallback-plus-warning over auto-migration
**Decision**: Legacy `~/.tu.conf` is read (with a one-line stderr deprecation warning, once per process per path) only when the new file is absent; it is never moved or deleted by tu.
**Why**: Auto-migration is a silent write to the user's home with rollback surprises (dotfile-managed or symlinked confs); read-plus-warn is cheaper, reversible, and conforms to Constitution II (warn on stderr, keep working).
**Rejected**: Read-time auto-migration of `~/.tu.conf` → `~/.config/tu/tu.conf` — silent home-directory mutation with surprise failure modes.
*Introduced by*: 260827-gzrn-config-home-conformance-org-layer

#### Org layer over org detection
**Decision**: An optional `$HOME/.config/tu/org.conf` participates in the cascade (`defaults < org.conf < tu.conf < env < CLI`); absence is silent. No org-detection heuristics.
**Why**: An org's dotfiles/MDM/bootstrap script drops the file once and every employee's tu is in multi mode with zero per-user edits, while personal overrides still win; heuristics would hardcode a company into a public tool, add startup latency, and not escape the bootstrap problem.
**Rejected**: Org detection via email domain / SSH alias / network probes — company-specific, latent, and still requires bootstrap.
*Introduced by*: 260827-gzrn-config-home-conformance-org-layer

#### Write-time legacy seeding
**Decision**: When tu creates `~/.config/tu/tu.conf` (via `init-conf` or `init-metrics <url>`) and a legacy `~/.tu.conf` exists, the new file is seeded from the legacy contents (with a stdout `Copied …` note) rather than from `tu.default.conf`.
**Why**: Seeding avoids silently orphaning the user's `machine`/`user`/`metrics_dir` overrides the moment they run the new one-liner; it is a write tu is already making, so it does not reintroduce the rejected read-time auto-migration.
**Rejected**: Always scaffolding from `tu.default.conf` — simpler, but loses existing user settings.
*Introduced by*: 260827-gzrn-config-home-conformance-org-layer

## Tasks

### Phase 2: Core Implementation

- [x] T001 `src/node/core/config.ts`: add `TOOL_NAME`/`CONFIG_FILE`/`ORG_CONFIG_FILE`/`LEGACY_CONFIG_FILE`, `ConfigPaths`, `ConfigHomeError`, lazy `resolveConfigPaths(env)` built from `$HOME` only; remove the eager `CONFIG_PATH` constant <!-- R1, R2 -->
- [x] T002 `src/node/core/config.ts`: `readConfig` cascade (defaults < org.conf < user conf < env < CLI `overrides`), user-conf selection rule with once-per-process-per-path legacy deprecation warning on stderr, version warning naming the path actually read; keep string-form first arg as userConf-only for existing tests <!-- R3, R4, R5 -->
- [x] T003 `src/node/core/cli.ts`: shared `normalizePaths` + `ensureUserConf` (legacy seeding, `Copied`/`Created` stdout lines); `runInitConf` on the new path; update imports (drop `CONFIG_PATH`) and `main()` dispatch for init-conf/status/sync to the lazy resolver <!-- R8 -->
- [x] T004 `src/node/core/cli.ts`: `runInitMetrics [repo-url]` — ensureUserConf + `setMetricsRepoInConf` (active-line replace / commented-line replace / FIELD_BLOCKS append) + `Set metrics_repo …` line, URL as CLI override layer, preserved idempotency, new no-repo error text, >1 positional arg → `EXIT_USAGE` with `SHORT_USAGE` in `main()` <!-- R6, R7 -->
- [x] T005 `src/node/core/cli.ts`: `runStatus` — display the R4-selected path, `Mode: single (no ~/.config/tu/tu.conf)` when neither exists, optional `Org config:` line directly after `Config:` in both layouts <!-- R9 -->
- [x] T006 [P] Strings: `FULL_HELP` setup rows + `tu sync` error (`src/node/core/cli.ts`), fish completion description (`src/node/core/completions.ts`), `tu.default.conf:2` comment <!-- R10 -->

### Phase 3: Integration & Edge Cases

- [x] T007 `src/node/core/__tests__/config.test.ts`: pin test (env vars cannot move the path; unset/empty HOME throws), cascade tests (org-only, org<user, user<env, env<CLI override), legacy fallback (read + exactly one warning across two calls; both-present → no warning) <!-- R12 -->
- [x] T008 `src/node/core/__tests__/cli-init-metrics.test.ts`: URL creates conf + writes `metrics_repo` + clones (bare-repo fixture); active/commented line replacement; `Already initialized` after write; URL beats `TU_METRICS_REPO`; two positional args → exit 2 (subprocess) <!-- R6 -->
- [x] T009 `src/node/core/__tests__/cli-init-conf.test.ts`: nested-path creation; legacy seeding case <!-- R8 -->
- [x] T010 `src/node/core/__tests__/cli-status.test.ts`: new/legacy/none path display; `Org config:` line present only with org.conf <!-- R9 -->
- [x] T011 `src/node/core/__tests__/cli-exit-codes.test.ts`: write temp-HOME conf to `.config/tu/tu.conf` instead of `.tu.conf` <!-- R13 -->
- [x] T012 Verify: scoped tests green; `npm run build`; `env -u HOME node dist/tu.mjs --version` works; `tu help-dump` stderr-clean exit 0 <!-- R2, R10 -->

### Phase 4: Polish

- [x] T013 [P] `README.md` § Setup (init-metrics one-liner + Team setup note), `docs/site/install.md` §1–3 rewrite + Config locations & precedence subsection, `docs/site/skill.md` new path + `init-metrics [url]` (≤150 lines) <!-- R11 -->
- [x] T014 [P] `docs/specs/usage.md` (grammar rows, Configuration retitle + cascade/org/legacy, init-metrics exit-2 row, delete stale `WEAVER_DEV`/`mode` rows) and `docs/specs/layouts.md` (status mockups + help mockup new path, optional `Org config:` line) <!-- R11 -->
- [x] T015 [P] `fab/project/context.md` config.ts row → `Config file reading (~/.config/tu/tu.conf, org.conf layer)` <!-- R11 -->
- [x] T016 Full suite: `env -u TU_METRICS_REPO npm test` green <!-- R12, R13 -->

## Acceptance

### Functional Completeness

- [x] A-001 R1: `resolveConfigPaths` returns `$HOME/.config/tu/{tu.conf,org.conf}` + `$HOME/.tu.conf` built from `$HOME` only; no XDG/TU_* env or `os.homedir()` on the config path; `CONFIG_PATH` constant gone
- [x] A-002 R2: `tu --version`/`help`/`help-dump`/`skill`/`shell-init`/`update` work with `HOME` unset; config-reading commands error `tu: $HOME is not set; cannot locate config` exit 1
- [x] A-003 R3: Cascade is exactly defaults < org.conf < tu.conf < `TU_METRICS_REPO` < CLI overrides; org absence is silent; string-form `readConfig` still works for old tests
- [x] A-004 R4: Legacy fallback reads `~/.tu.conf` only when the new file is absent, warns once per process per path on stderr, never migrates the file
- [x] A-005 R5: `resolveHome`/sentinels/mode derivation/`auto_sync`/`metrics_dir` default/`TU_HOME` unchanged; version warning names the path read
- [x] A-006 R6: `tu init-metrics <url>` writes `metrics_repo` (create/replace/append), prints `Set metrics_repo …`, clones the typed URL even when `TU_METRICS_REPO` differs, stays idempotent, and rejects extra positional args with exit 2
- [x] A-007 R7: `tu init-metrics` without a URL behaves as before with the new error text
- [x] A-008 R8: `tu init-conf` writes `~/.config/tu/tu.conf` via the shared `ensureUserConf` helper, seeding from a legacy conf when present
- [x] A-009 R9: `tu status` shows the selected path and the `Org config:` line exactly when org.conf exists; layout otherwise byte-identical
- [x] A-010 R10: FULL_HELP, sync error, fish completion, and `tu.default.conf` name the new path; `tu help-dump` stays stderr-clean exit 0
- [x] A-011 R11: README, install.md, skill.md (≤150 lines), usage.md (incl. stale-row deletion), layouts.md, and context.md updated; no user-facing `~/.tu.conf` mention remains
- [x] A-012 R12: config.test.ts carries the pin, cascade, and legacy-fallback tests
- [x] A-013 R13: init-metrics/init-conf/status tests cover the new behavior; cli-exit-codes writes `.config/tu/tu.conf`

### Behavioral Correctness

- [x] A-014 R4: With both files present the legacy file is silently ignored (no warning, new file's values win)
- [x] A-015 R6: `Already initialized` still prints (exit 0) when metricsDir is a git repo — after the config write has happened
- [x] A-016 R9: Single-mode no-config output is `Mode: single (no ~/.config/tu/tu.conf)`

### Scenario Coverage

- [x] A-017 R6: Team onboarding one-liner works end-to-end against a local bare repo: `tu init-metrics <url>` → `tu status` shows multi mode
- [x] A-018 R3: org.conf-only setup yields multi mode with no user file and no warning

### Edge Cases & Error Handling

- [x] A-019 R2: `HOME=""` behaves identically to unset `HOME`
- [x] A-020 R4: Deprecation warning never reaches stdout (json/csv/md stay clean) and fires at most once per process per legacy path
- [x] A-021 R6: Existing commented `# metrics_repo = …` scaffold line is replaced with the active assignment rather than duplicated

### Code Quality

- [x] A-022: Minimum pathways — one `ensureUserConf` scaffold helper shared by `runInitConf` and `runInitMetrics`; `parseConf`/`readConfFile`/`FIELD_BLOCKS`/`tildefy` reused, not duplicated
- [x] A-023: No god functions — new helpers stay focused and comparable in size to neighboring code
- [x] A-024: No magic strings — file names behind `TOOL_NAME`/`CONFIG_FILE`/`ORG_CONFIG_FILE`/`LEGACY_CONFIG_FILE` constants; usage errors use `EXIT_USAGE`
- [x] A-025: No silent error swallowing — unset-HOME and deprecation paths warn/error on stderr with actionable text
- [x] A-026 Pattern consistency: New code follows naming and structural patterns of surrounding code
- [x] A-027 No unnecessary duplication: Existing utilities reused where applicable

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Deletion Candidates

- `src/node/core/config.ts` `CONFIG_PATH` — the eager module-level constant was removed by this change itself (planned removal); superseded by the lazy `resolveConfigPaths`
- None beyond the above — this change adds new functionality (org.conf layer, legacy fallback, `init-metrics <url>` argument) without making any other existing symbol, file, or branch redundant

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Config root is `$HOME/.config/tu/tu.conf`, built from `$HOME` only; no `$XDG_CONFIG_HOME`, no `os.homedir()`/passwd fallback; unset `$HOME` → stderr `tu: $HOME is not set; cannot locate config`, exit 1 | Dictated verbatim by the binding `config-home` standard (intake #1) | S:95 R:70 A:95 D:95 |
| 2 | Certain | Legacy `~/.tu.conf` is read only when the new file is absent, with a one-line stderr deprecation warning; no auto-migration; new file present → legacy silently ignored | Intake #2 — fallback-plus-warning chosen over auto-migration | S:90 R:85 A:90 D:90 |
| 3 | Certain | Optional `$HOME/.config/tu/org.conf`, same format; cascade exactly `tu.default.conf < org.conf < tu.conf < TU_METRICS_REPO < CLI arg`; absence silent; no per-key inversions | Intake #3 — fixed cascade | S:95 R:75 A:90 D:95 |
| 4 | Certain | `tu init-metrics <repo-url>` writes `metrics_repo` into `~/.config/tu/tu.conf` (creating it from the scaffold if needed) then clones; without the argument behavior is unchanged; stays idempotent | Intake #4 | S:95 R:80 A:90 D:90 |
| 5 | Certain | State (`~/.tu/`), `TU_HOME`, and the `metrics_dir` default are untouched | Intake #5 — explicitly scoped out | S:95 R:90 A:95 D:95 |
| 6 | Tentative | When tu creates `~/.config/tu/tu.conf` and a legacy `~/.tu.conf` exists, seed the new file from the legacy contents (write-time copy, stdout note) rather than from `tu.default.conf` | Intake #6 — avoids orphaning user overrides; not the rejected read-time migration | S:25 R:75 A:40 D:40 |
| 7 | Confident | `tu status` prints `Org config: ~/.config/tu/org.conf` only when the file exists; layout otherwise byte-identical | Intake #7 | S:35 R:85 A:55 D:45 |
| 8 | Confident | Path resolution is lazy so `help`/`--version`/`help-dump`/`skill`/`shell-init`/`update` work with `$HOME` unset | Intake #8 — Constitution II/IV + actionable-error standard | S:60 R:80 A:85 D:85 |
| 9 | Confident | The `<repo-url>` argument acts as the CLI-flag layer: it beats an exported `TU_METRICS_REPO` | Intake #9 — direct consequence of the fixed cascade | S:65 R:85 A:90 D:85 |
| 10 | Confident | Writing `metrics_repo` replaces an active line in place, else replaces the scaffold's commented line, else appends the `FIELD_BLOCKS.metrics_repo` block | Intake #10 — keeps the file tidy and idempotent | S:60 R:90 A:85 D:80 |
| 11 | Confident | Deprecation warning emitted once per process, stderr only; extra positional args to `init-metrics` → `EXIT_USAGE` (2) with `SHORT_USAGE` | Intake #11 | S:55 R:90 A:85 D:85 |
| 12 | Certain | Minor version bump at ship time; no `package.json` edit in this change; standards checked at apply entry | Intake #12 — Constitution § Output Stability / § Toolkit Standards | S:90 R:90 A:95 D:95 |
| 13 | Confident | Stale `WEAVER_DEV` and `mode` rows in `docs/specs/usage.md § Configuration` are removed while the section is rewritten | Intake #13 | S:50 R:95 A:90 D:90 |
| 14 | Confident | Once-per-process warning is keyed per legacy path (module-level `Set<string>`) — two reads of the same legacy path warn once; distinct paths each warn | Avoids a test-only reset hook (Test Integrity) while honoring the once-per-process contract; same observable behavior for real usage (one legacy path per process) | S:55 R:85 A:80 D:75 |
| 15 | Confident | `ConfigPaths.orgConf`/`legacyConf` are optional; the string form of `readConfig`/`runInitConf`/`runInitMetrics`/`runStatus` normalizes to userConf-only (no org, no legacy) | Intake grants signature latitude; keeps every existing string-passing test working unchanged | S:70 R:80 A:80 D:75 |
| 16 | Confident | `runStatus` omits the `Config:` line when no user/legacy file was read (org-only setups) but still prints the `Org config:` line; Org line value column-aligned (`Org config:  `, two spaces) | Edge not specified by the intake; omitting a `Config:` line for a file that does not exist is the honest layout; alignment matches the existing column | S:45 R:85 A:60 D:55 |
| 17 | Certain | Extra `init-metrics` positional args error with `Error: init-metrics takes at most one argument (repo-url)` + `SHORT_USAGE`, exit 2 | Follows the existing two-line usage-error idiom (`dispatchSingleTool`, `parseDataArgs` catch) | S:80 R:90 A:85 D:85 |
| 18 | Confident | The version-too-new warning names the user conf path actually read, else the org path when it was read, else the defaults path | Intake requires naming the path read; precedence order covers the no-user-file edge | S:50 R:85 A:70 D:65 |

18 assumptions (5 certain, 12 confident, 1 tentative).
