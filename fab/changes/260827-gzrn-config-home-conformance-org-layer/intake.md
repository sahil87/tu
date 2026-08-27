# Intake: Config-home conformance + org config layer

**Change**: 260827-gzrn-config-home-conformance-org-layer
**Created**: 2026-08-27

## Origin

Promptless dispatch (`/fab-proceed` create-new) from a prior discussion session. The raw input was a synthesized description of the change with decisions already made; no questions were asked during intake (`{questioning-mode} = promptless-defer`).

> `tu` violates the shll toolkit `config-home` standard (binding via Constitution § Toolkit Standards). The standard currently lists tu as "no config file today" — wrong; tu has `~/.tu.conf`. Second goal: make it trivial for a team (noon) to point every employee's tu at a shared metrics repo, without impacting the rest of the world. Target onboarding:
>
> ```
> shll update tu                                          # or: tu update
> tu init-metrics git@github.com:wvrdz/tu-metrics.git     # writes config + clones
> tu sync
> tu status                                               # should say mode: multi
> ```

Key decisions carried over from the discussion (all encoded in `## Assumptions`):

- Config root moves to `$HOME/.config/tu/tu.conf`, built from `$HOME` only; unset `$HOME` is an actionable error.
- Legacy `~/.tu.conf` is read as a fallback with a one-line stderr deprecation warning — **not** auto-migrated.
- New optional org layer `$HOME/.config/tu/org.conf`; cascade `tu.default.conf < org.conf < tu.conf < env < CLI flags`, no per-key inversions.
- `tu init-metrics [repo-url]` writes `metrics_repo` into `tu.conf` before cloning.
- State stays at `~/.tu/` (out of scope to move).
- Org-detection heuristics (email domain, SSH alias, network probes) were considered and **rejected**: they hardcode a company into a public tool, add startup latency, and do not escape the bootstrap problem.
- Auto-sync / stale-triggered sync is a separate backlog item — untouched here.

## Why

**1. Standards conformance.** The shll `config-home` standard (`shll standards config-home`, verified 2026-08-23 text) requires every toolkit tool with a config file to resolve it as `$HOME/.config/<tool-name>/`, built from `$HOME` and nothing else — no `$XDG_CONFIG_HOME`, no `os.UserConfigDir`, unset `$HOME` is an actionable error, and a test pins the path against environment variables. It also fixes the override order at `code defaults < config file < env < CLI flag` and restricts env vars to deployment-bootstrap keys. tu today resolves `CONFIG_PATH = resolve(homedir(), ".tu.conf")` (`src/node/core/config.ts:20`) — wrong root, wrong file name, and `os.homedir()` silently falls back to the passwd entry when `$HOME` is unset. The standard's conformance list even says tu has "no config file today", which is factually wrong; the constitution makes the standard binding, so this is a real violation, not a nit. (Fixing the shll-side conformance list is a separate one-line PR in the shll repo — out of scope here.)

**2. Team onboarding is a copy-paste ritual.** Pointing a team at a shared metrics repo currently takes a Slack message with `tu init-conf` + two `sed` edits of `~/.tu.conf` + `tu init-metrics` + `tu sync` + `tu status`. The sed edits are the fragile step (they assume a specific scaffold layout and break the moment `tu.default.conf` changes). Two mechanisms fix this without teaching tu anything about any specific org:

- `tu init-metrics <repo-url>` — one command writes `metrics_repo` and clones. This is the manual path for individuals and small teams.
- `org.conf` — an org's dotfiles/MDM/bootstrap script can drop `$HOME/.config/tu/org.conf` with `metrics_repo = …` and every employee's tu is in multi mode with zero per-user edits, while personal `tu.conf` overrides still win. Absence is silent, so the rest of the world never notices the layer exists.

**3. If we don't.** tu stays non-conformant to a binding standard; every new teammate goes through the sed ritual; and the eventual move of the config path gets more expensive the longer `~/.tu.conf` accumulates users. Doing the path move and the org layer in one change means users see a single `~/.tu.conf → ~/.config/tu/tu.conf` transition rather than two.

**Why fallback-plus-warning over auto-migration**: auto-migration (moving the file on first run) is a silent write to the user's home directory with rollback surprises (dotfile-managed `~/.tu.conf` re-appearing, a symlinked conf being replaced by a copy). Reading the legacy path with a deprecation warning is cheaper, reversible, and conforms to Constitution II (graceful degradation: warn on stderr, keep working).

## What Changes

### 1. Config path resolution (`src/node/core/config.ts`)

Replace the eager module-level constant `CONFIG_PATH = resolve(homedir(), ".tu.conf")` with a **lazy resolver** built from `process.env.HOME` only:

```ts
export const TOOL_NAME = "tu";
export const CONFIG_FILE = "tu.conf";
export const ORG_CONFIG_FILE = "org.conf";
export const LEGACY_CONFIG_FILE = ".tu.conf";

export interface ConfigPaths {
  configDir: string;   // $HOME/.config/tu
  userConf: string;    // $HOME/.config/tu/tu.conf
  orgConf: string;     // $HOME/.config/tu/org.conf
  legacyConf: string;  // $HOME/.tu.conf
}

// Built from $HOME and nothing else — no $XDG_CONFIG_HOME, no os.homedir()
// passwd fallback (shll config-home standard). Throws a ConfigHomeError with
// the actionable message when $HOME is unset/empty.
export function resolveConfigPaths(env: NodeJS.ProcessEnv = process.env): ConfigPaths
```

- **`$HOME` unset or empty** → the resolver throws; callers that need config surface `tu: $HOME is not set; cannot locate config` on stderr and exit `1` (operational/environment error per the exit-code convention in `docs/specs/usage.md § Exit Codes` — recovery is "fix the environment"). No `os.homedir()` fallback on the config path.
- **Lazy, not import-time**: commands that never touch config (`tu help`, `tu --version`, `tu help-dump`, `tu skill`, `tu shell-init`, `tu update`) MUST keep working with `$HOME` unset. `TU_HOME` (state dir) and `resolveHome()` (for `metrics_dir = ~/...`) are **not** changed — state is out of scope; only the config-resolution path is bound by the standard.
- **No env var can move the config path**: `XDG_CONFIG_HOME`, `TU_CONFIG`, `TU_CONFIG_HOME`, `TU_HOME` etc. are all ignored by the resolver.
- `tildefy()` output for the new path is `~/.config/tu/tu.conf` (verify `tildefy` handles the nested path; it prefix-replaces `homedir()` so it should).

### 2. Cascade with legacy fallback and org layer (`readConfig`)

`readConfig` currently merges `defaults < user` then applies `TU_METRICS_REPO`. New merge order, **exactly**:

```
tu.default.conf   (shipped beside the bundle, found via findDefaultConf — unchanged)
  < $HOME/.config/tu/org.conf   (optional; absence is silent — no warning, no log)
  < user conf                   (see selection rule below)
  < env: TU_METRICS_REPO        (metrics_repo only — the sole env key, unchanged)
  < CLI flag / argument         (today: the init-metrics <repo-url> argument; see §3)
```

**User-conf selection rule** (the only place the legacy path is consulted):

| `~/.config/tu/tu.conf` exists | `~/.tu.conf` exists | Read | stderr |
|---|---|---|---|
| yes | any | `~/.config/tu/tu.conf` | nothing (legacy ignored silently) |
| no | yes | `~/.tu.conf` | one line: `tu: ~/.tu.conf is deprecated; move it to ~/.config/tu/tu.conf` |
| no | no | (defaults + org only) | nothing |

The deprecation warning is emitted **once per process** (guard with a module flag — `readConfig` is called from several paths and watch mode re-reads) and goes to stderr only, so `--json`/`--csv`/`--md` stdout stays clean. It fires in every mode including `tu status`.

`org.conf` uses the identical `key = value` / `#` comment format and the same `parseConf`. It is read via the same `readConfFile` (returns `null` when missing → merged as `{}`). No per-key inversions: a key set in `org.conf` is always overridden by the same key in `tu.conf`, which is always overridden by env, which is always overridden by a CLI argument. The `version` key is taken from the merged result as today (an org.conf carrying `version` behaves like any other key).

`readConfig`'s signature keeps its injectable-path shape for tests but gains the org path:

```ts
export function readConfig(
  paths: ConfigPaths | string = resolveConfigPaths(),   // string form kept for existing tests: treated as userConf only
  defaultsPath: string = DEFAULT_CONFIG_PATH,
  overrides: Partial<Pick<RawConf, "metrics_repo">> = {},  // CLI-flag layer
): TuConfig
```

Implementers MAY choose a different exact shape (e.g. an options object) as long as: (a) all existing tests that pass a bare user-conf path still work or are updated to the new shape, (b) the org path and the CLI-override layer are injectable, and (c) the cascade order above is honoured.

The version-warning string `Warning: ~/.tu.conf version ${version} is newer than tu supports…` (`config.ts:103`) MUST name the path actually read.

`resolveHome`, `expandSentinels`, sentinel behaviour, `mode` derivation, `auto_sync` parsing, and `metrics_dir` default `~/.tu/metrics_repo` are **unchanged**.

### 3. `tu init-metrics [repo-url]` (`src/node/core/cli.ts` `runInitMetrics`)

Grammar: `tu init-metrics [<repo-url>]`. Dispatch in `main()` currently does `if (cmd === "init-metrics") { runInitMetrics(); return; }` — pass the next positional argument (if any) through.

**With `<repo-url>`**:

1. Resolve config paths (§1). If `$HOME` unset → the §1 error, exit 1.
2. Ensure `~/.config/tu/tu.conf` exists: if missing, create it via the same scaffold logic `runInitConf` uses (`mkdirSync(configDir, { recursive: true })`, copy `tu.default.conf`). Reuse — do not duplicate — the scaffold code (extract a shared `ensureUserConf(paths, defaultsPath)` helper that `runInitConf` also calls).
   <!-- assumed: when the new file is being created and a legacy ~/.tu.conf exists, seed the new file from the legacy file's contents instead of tu.default.conf so the user's machine/user/metrics_dir overrides are not silently orphaned; print `Copied ~/.tu.conf → ~/.config/tu/tu.conf` on stdout. This is a write-time copy, not the rejected read-time auto-migration — see Assumptions #6 -->
3. Write `metrics_repo = <repo-url>` into `tu.conf`: if an active `metrics_repo =` line exists → replace its value in place; else if a commented `# metrics_repo = …` line exists (the scaffold ships one) → replace that line with the active assignment; else append the `FIELD_BLOCKS.metrics_repo` block with the value filled in. Print `Set metrics_repo = <repo-url> in ~/.config/tu/tu.conf`.
4. Proceed with the existing clone behaviour, **using `<repo-url>` as the CLI-flag layer** for this invocation (so an exported `TU_METRICS_REPO` pointing elsewhere cannot make the clone diverge from the URL the user just typed — CLI > env per the cascade).
5. Idempotency is preserved: if `metricsDir` already is a git repo → `Already initialized: <dir>` (exit 0), after the config write in step 3 has happened. Existing error paths (`exists but is not a git repo`) unchanged, exit 1.

**Without `<repo-url>`**: behaviour unchanged — requires `metrics_repo` to be set (by org.conf, tu.conf, or `TU_METRICS_REPO`), else `Error: metrics_repo is not set. Add it to ~/.config/tu/tu.conf, run 'tu init-metrics <repo-url>', or set TU_METRICS_REPO.` exit 1.

More than one positional argument → usage error (`EXIT_USAGE = 2`) with the `SHORT_USAGE` line, consistent with the existing usage-error sites.

### 4. `tu init-conf` (`runInitConf`)

Writes/updates `~/.config/tu/tu.conf` (never `~/.tu.conf`). Creating the directory with `mkdirSync(..., { recursive: true })` is already in place. Messages change path only: `Created ~/.config/tu/tu.conf — edit it to configure multi-machine sync.`, `~/.config/tu/tu.conf is already complete.`, etc. If only a legacy conf exists, the same seeding rule as §3 step 2 applies (Assumptions #6).

### 5. `tu status` (`runStatus`)

Already prints `Config: <path> (vN)` and `Mode: single (no <path>)`. It MUST show the path actually selected by the §2 rule — `~/.config/tu/tu.conf`, or `~/.tu.conf` when the legacy fallback is in use (the deprecation line goes to stderr as usual). When neither exists: `Mode: single (no ~/.config/tu/tu.conf)`.

Add an `Org config: ~/.config/tu/org.conf` line, printed only when `org.conf` exists, directly after the `Config:` line in both single and multi layouts — so a teammate can see why they are in multi mode without a `metrics_repo` in their own file. Omitting the line when absent keeps today's layout byte-identical for everyone without an org layer (Assumptions #7).

### 6. User-facing strings and docs

Every `~/.tu.conf` mention in user-facing surfaces becomes `~/.config/tu/tu.conf`; `org.conf`, the cascade, and `tu init-metrics <url>` get documented:

| File | Change |
|---|---|
| `src/node/core/cli.ts:117` (`FULL_HELP`) | `tu init-conf         Scaffold ~/.config/tu/tu.conf`; `tu init-metrics [url] Clone metrics repo (url also sets metrics_repo)` |
| `src/node/core/cli.ts:541` (`tu sync` error) | `Add metrics_repo to ~/.config/tu/tu.conf, run 'tu init-metrics <repo-url>', or set TU_METRICS_REPO.` |
| `src/node/core/config.ts:103` | version-warning names the path read |
| `src/node/core/completions.ts:153` | fish description `scaffold ~/.config/tu/tu.conf` |
| `tu.default.conf:2` | `# User overrides go in ~/.config/tu/tu.conf (created by 'tu init-conf'). Org-wide defaults may be dropped in ~/.config/tu/org.conf.` |
| `README.md` § Setup | new path; `tu init-metrics <repo-url>` as the one-liner; a short "Team setup" note on `org.conf` |
| `docs/site/install.md` §1–3 | rewrite the setup flow around `tu init-metrics <repo-url>`; keep the `init-conf` + edit path as the alternative; add a **Config locations & precedence** subsection listing the cascade table verbatim and the legacy-fallback rule |
| `docs/site/skill.md:42,85` | new path; mention `init-metrics [url]` (keep ≤150 lines per the `skill` standard) |
| `docs/site/workflows.md` | multi-machine recipe uses `tu init-metrics <repo-url>` (verify current content first) |
| `docs/specs/usage.md:78–79,172–186` | CLI grammar rows for `init-conf`/`init-metrics [repo-url]`; `### Configuration` retitled to the new path, with the cascade, org.conf, legacy fallback; **also delete the stale `WEAVER_DEV` / `mode` rows** (already removed by 260401-jufw) while touching the section |
| `docs/specs/layouts.md:282–297,329` | status layout mockups use the new path; add the optional `Org config:` line |
| `fab/project/context.md:28` | `config.ts` row: `Config file reading (~/.config/tu/tu.conf, org.conf layer)` |

Only `.tu.conf` mentions are in scope; `~/.tu/` (state) mentions stay.

### 7. Tests (co-located `__tests__/`, Node test runner)

New/updated coverage:

- `src/node/core/__tests__/config.test.ts` — **pin test**: with `HOME=<tmp>`, setting `XDG_CONFIG_HOME`, `TU_CONFIG`, `TU_CONFIG_HOME` to other dirs does not move `resolveConfigPaths().userConf`; `delete process.env.HOME` (and `HOME=""`) → throws with a message containing `$HOME is not set`. Cascade tests: org-only → multi; org + tu.conf disagreeing → tu.conf wins; tu.conf + `TU_METRICS_REPO` → env wins; override argument beats env. Legacy fallback: only `~/.tu.conf` present → read + exactly one deprecation line on stderr across two `readConfig` calls; both present → new file read, no warning.
- `src/node/core/__tests__/cli-init-metrics.test.ts` — `runInitMetrics` with a URL: creates `tu.conf` from defaults, writes `metrics_repo`, clones (existing local-bare-repo fixture); replaces an existing active/commented `metrics_repo` line; idempotent `Already initialized`; URL beats `TU_METRICS_REPO`; two positional args → exit 2.
- `src/node/core/__tests__/cli-init-conf.test.ts` — writes the nested path; seeding-from-legacy case (Assumptions #6).
- `src/node/core/__tests__/cli-status.test.ts` — path shown for new/legacy/none; `Org config:` line present only with org.conf.
- `src/node/core/__tests__/cli-exit-codes.test.ts:82` currently writes `.tu.conf` into a temp `HOME` — update to write `.config/tu/tu.conf` (otherwise the legacy warning lands on stderr and may break stderr assertions). Same sweep for `cli-sync`, `cli-dry-run-flag`, `src/node/sync/__tests__/repair-metrics.test.ts` wherever they build a conf path.
- `help-dump.test.ts` / `cli-help.test.ts` — update any snapshot of the help text.

Run scoped first: `npx tsx --test src/node/core/__tests__/config.test.ts src/node/core/__tests__/cli-init-*.test.ts src/node/core/__tests__/cli-status.test.ts`, then the full `npm test` with `env -u TU_METRICS_REPO`.

### 8. Ship-time requirements (not done in this change's code)

- **Minor version bump** (`just release minor`, 0.11.x → 0.12.0) — user-visible path change and a new `tu status` line fall under Constitution § Output Stability. The change itself does not edit `package.json`.
- Check the touched surfaces against the shll standards before review: `config-home` (this change's whole point — walk its "Verifying conformance" checklist), `help-dump` (help text changes must keep `tu help-dump` stderr-clean, exit 0), `readme-extraction` (README/docs/site structure), `skill` (`docs/site/skill.md` ≤150 lines, static-only), `principles` (№4 actionable errors, №6 same invocation same behaviour). Run `shll standards <name>` for each at apply entry; if shll is unavailable, read `sahil87/shll` `docs/site/standards/<name>.md`.

### Explicitly out of scope

- Auto-sync / stale-triggered sync (separate backlog item; `isStale` dead code and the no-op `auto_sync` key are NOT touched).
- Moving `~/.tu/` state (cache, `metrics_repo` clone, `.last-sync`, clone marker) to an XDG state dir; `TU_HOME` and the `metrics_dir` default are untouched.
- Org-detection heuristics of any kind.
- Auto-migrating (moving/deleting) `~/.tu.conf` on read.
- Updating the shll repo's conformance list.
- Any new env var. `TU_METRICS_REPO` remains the only config-bearing env key (deployment bootstrap — conformant).

## Affected Memory

- `configuration/config-system`: (modify) config path `~/.config/tu/tu.conf` built from `$HOME` only; unset-HOME error; legacy `~/.tu.conf` fallback + deprecation warning; `org.conf` layer; exact cascade `defaults < org.conf < tu.conf < env < CLI`; `init-conf` writes the new path; pin test; Design Decisions entries for "fixed root, no XDG (config-home standard)", "fallback-plus-warning over auto-migration", "org layer over org detection". Rewrite the `Home directory ~/.tu/` decision to say config is no longer top-level.
- `cli/data-pipeline`: (modify) `init-metrics [repo-url]` grammar row; positional-arg handling and the new usage-error site (exit 2); the `$HOME`-unset operational error (exit 1); help text; non-data command dispatch note.
- `sync/multi-machine`: (modify) `init-metrics` requirement: URL argument writes `metrics_repo` then clones; URL is the CLI layer (beats `TU_METRICS_REPO`); idempotency preserved.

Specs (human-curated, updated as part of §6): `docs/specs/usage.md` (§ CLI grammar rows, § Configuration), `docs/specs/layouts.md` (status mockups).

## Impact

- **Code**: `src/node/core/config.ts` (resolver, cascade, warning), `src/node/core/cli.ts` (`runInitConf`, `runInitMetrics`, `runStatus`, `main()` dispatch, `FULL_HELP`, sync error string, shared scaffold helper), `src/node/core/completions.ts` (one string), `tu.default.conf` (comment).
- **Tests**: `config`, `cli-init-conf`, `cli-init-metrics`, `cli-status`, `cli-exit-codes`, `cli-sync`, `cli-dry-run-flag`, `help-dump`/`cli-help`, `sync/__tests__/repair-metrics` — mostly path fixtures; new pin/cascade/URL tests.
- **Docs**: README.md, docs/site/{install,skill,workflows}.md, docs/specs/{usage,layouts}.md, fab/project/context.md, memory files above.
- **Users**: existing `~/.tu.conf` users keep working and see one stderr line per invocation until they move the file; `--json` consumers unaffected (stderr). New `tu status` line only for org.conf users. Minor version bump.
- **Distribution**: no bundle/Homebrew changes; `tu.default.conf` stays shipped beside `dist/tu.mjs`.
- **Risk**: the lazy resolver must not regress `tu --version`/`help`/`update` under an unset `$HOME`; test suites that write `.tu.conf` into a temp HOME must be migrated or the deprecation line will pollute stderr assertions.

## Open Questions

- Should `tu init-conf` / `tu init-metrics <url>` seed the newly created `~/.config/tu/tu.conf` from an existing `~/.tu.conf` (proposed default: yes, with a stdout note) — or always from `tu.default.conf`? See Assumptions #6.
- Should `tu status` show an `Org config:` line when `org.conf` exists (proposed default: yes, only when present)? See Assumptions #7.
- Which release drops the legacy `~/.tu.conf` fallback? Not needed for this change; record the intent ("at least two minor versions") in the memory Design Decisions so it is not forgotten.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Config root is `$HOME/.config/tu/tu.conf`, built from `$HOME` only; no `$XDG_CONFIG_HOME`, no `os.homedir()`/passwd fallback; unset `$HOME` → stderr `tu: $HOME is not set; cannot locate config`, exit 1 | Dictated verbatim by the binding `config-home` standard and the discussion; exit 1 follows the documented exit-code convention (environment fix) | S:95 R:70 A:95 D:95 |
| 2 | Certain | Legacy `~/.tu.conf` is read only when the new file is absent, with a one-line stderr deprecation warning; no auto-migration; new file present → legacy silently ignored | Discussed — user chose fallback-plus-warning over auto-migration (cheaper, reversible; Constitution II) | S:90 R:85 A:90 D:90 |
| 3 | Certain | Optional `$HOME/.config/tu/org.conf`, same format; cascade exactly `tu.default.conf < org.conf < tu.conf < TU_METRICS_REPO < CLI arg`; absence silent; no per-key inversions | Discussed and fixed; matches the standard's single-cascade rule | S:95 R:75 A:90 D:95 |
| 4 | Certain | `tu init-metrics <repo-url>` writes `metrics_repo` into `~/.config/tu/tu.conf` (creating it from the init-conf scaffold if needed) then clones; without the argument behaviour is unchanged; stays idempotent | Discussed with the exact target onboarding flow | S:95 R:80 A:90 D:90 |
| 5 | Certain | State (`~/.tu/`: cache, metrics_repo clone, `.last-sync`, clone marker), `TU_HOME`, and the `metrics_dir` default are untouched | Explicitly scoped out in the discussion; the standard permits state elsewhere | S:95 R:90 A:95 D:95 |
| 6 | Tentative | When tu creates `~/.config/tu/tu.conf` (via `init-conf` or `init-metrics <url>`) and a legacy `~/.tu.conf` exists, seed the new file from the legacy contents (write-time copy, stdout note) rather than from `tu.default.conf` | Not discussed. Seeding avoids silently orphaning the user's `machine`/`user`/`metrics_dir` overrides the moment they run the new one-liner; it is a write we are already making, so it does not reintroduce the rejected read-time auto-migration. Alternative (always defaults) is simpler but loses settings. Easily reversed | S:25 R:75 A:40 D:40 |
| 7 | Confident | `tu status` prints `Org config: ~/.config/tu/org.conf` only when the file exists; layout otherwise byte-identical | Not discussed. Small UX choice; helps explain why a teammate is in multi mode with no `metrics_repo` in their own file. Rides the minor bump already required | S:35 R:85 A:55 D:45 |
| 8 | Confident | Path resolution is lazy (function), not an import-time constant, so `help`/`--version`/`help-dump`/`skill`/`shell-init`/`update` work with `$HOME` unset; the error surfaces only on config-reading commands | Follows from Constitution II/IV and the standard's "actionable error" — crashing `tu --version` on a missing `$HOME` would be a regression, and shll's version probe (`version` standard) must keep working | S:60 R:80 A:85 D:85 |
| 9 | Confident | The `<repo-url>` argument acts as the CLI-flag layer for that invocation: it beats an exported `TU_METRICS_REPO`, so the clone always targets the URL typed | Direct consequence of the fixed cascade (CLI > env); the alternative (clone the env value after writing the URL to file) would violate the cascade and surprise the user | S:65 R:85 A:90 D:85 |
| 10 | Confident | Writing `metrics_repo` replaces an active line in place, else replaces the scaffold's commented `# metrics_repo` line, else appends the `FIELD_BLOCKS.metrics_repo` block | Scaffold layout is known (`tu.default.conf`, `FIELD_BLOCKS`); keeps the file tidy and idempotent on re-run | S:60 R:90 A:85 D:80 |
| 11 | Confident | Deprecation warning is emitted once per process and only to stderr; extra positional args to `init-metrics` → `EXIT_USAGE` (2) with `SHORT_USAGE` | Watch mode/re-reads would otherwise spam; stderr-only keeps `--json`/`--csv` clean; exit 2 matches the documented usage-error convention | S:55 R:90 A:85 D:85 |
| 12 | Certain | Minor version bump at ship time (`just release minor`); no `package.json` edit in this change; standards `config-home`, `help-dump`, `readme-extraction`, `skill`, `principles` checked at apply entry | Constitution § Output Stability and § Toolkit Standards | S:90 R:90 A:95 D:95 |
| 13 | Confident | Stale `WEAVER_DEV` and `mode` rows in `docs/specs/usage.md § Configuration` are removed while the section is rewritten | Both were removed from code by 260401-jufw; leaving them while rewriting the surrounding section would be a known inaccuracy | S:50 R:95 A:90 D:90 |

13 assumptions (6 certain, 6 confident, 1 tentative, 0 unresolved).
