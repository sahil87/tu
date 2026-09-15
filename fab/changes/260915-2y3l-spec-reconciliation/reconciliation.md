# Reconciliation record — P1 spec-reconciliation

Working artifact for change `260915-2y3l-spec-reconciliation`. It exists so the reviewer and gate G0 can audit the walk without redoing it. It is not part of the shipped docs.

## 1. Method and environment

- **Oracle**: the installed `tu` v0.11.5 (`/home/linuxbrew/.linuxbrew/bin/tu` → `Cellar/tu/0.11.5/libexec/tu.mjs`, vendored ccusage). `main` at the time of the walk differs from the release only by docs/fab commits.
- **Machine**: dev-ws-sahil02, 2026-09-15/16 (UTC 18:55–19:30; local date already 2026-09-16). Real config: multi mode against the real `wvrdz/tu-metrics` clone, used **read-only** (no `tu sync`, no `--sync`, no `tu update`, no `init-metrics <url>` under the real `$HOME`). Ordinary multi-mode data commands wrote this machine's own day-files into the local clone, as they do on every invocation.
- **Sandboxes** (scratchpad, not committed): temp `$HOME`s with the real assistant data directories symlinked in (`~/.claude`, `~/.codex`, `~/.gemini`, `~/.copilot`, `~/.local/share/opencode`) so ccusage finds usage; `TU_METRICS_REPO` unset (it is exported in the shell and forces multi mode). Multi-mode sandbox: `tu.conf` → a local **bare git repo seeded with an initial commit on `main`** plus two synthetic user/machine trees (`otheruser/2026/othermach`, `sbuser/2026/othermach`) and a `docs/` dir; a `.gitconfig` with a user identity (a first attempt without one made every `tu sync` fail at the commit step, which is itself recorded as DC-18).
- **Scripts** (copied to `walk/`, runnable with `WALK_OUT=<dir>`): `run.sh` (306 cells: toolkit commands, single mode, config variants, real-config read-only multi), `run-multi.sh` (78 cells: seeded multi sandbox), `run-sync.sh` (24 cells: sync, rebase, auto-sync probe), `watch.sh` (11 watch sessions × 5 frames via an isolated tmux server), `tty.sh` (26 one-shot TTY captures at 50–140 columns). Cell indexes: `walk/index-*.tsv` (id, name, exit, stdout bytes, stderr bytes).
- **Watch and TTY frames** were captured through tmux; tmux re-encodes SGR sequences, so color bytes were compared only from piped captures, layout from tmux.

## 2. Coverage against the intake §1 matrix

| Axis | Covered | Notes |
|------|---------|-------|
| Source tokens | all 12 + unknown + two sources | `two-sources`, `bogus-arg` |
| Period tokens | all 7 | |
| Display tokens | all 8 | `dh` ≡ `h` verified byte-identical |
| Formats | table, `--json`, `-j`, `--csv`, `--md`; all 6 pairwise conflicts; each with `--watch` | |
| Flags | every variant listed in intake §1, including bad values, missing values, impossible date, inverted window, `-t` with `--metric cost`, `--top 0/-1/abc/missing`, `-u` missing | |
| Non-data commands | all, incl. `update --help`/`-h`, `cc --help`, `h -h`, `shell-init` none/bogus | `tu update` live path not run (do-not-run) |
| Config modes | single, multi (real + sandbox), org-only, org+user, legacy, `version = 9`, `mode = multi` + `auto_sync = 0` + `metrics_dir = ~/…`, `user = all`, bad repo URL, dir-not-a-git-repo, `$HOME` unset, `$HOME` empty, `TU_METRICS_REPO` alone, env vs URL precedence | |
| Terminal | TTY 50/59/60/80/100/120/140 cols; pipe with `COLUMNS` 50–120; watch 100×30, 100×12, 59×20, 50×20; resize 100→50 | |
| Sync | dry-run (new / update / skip), live sync, second sync, `--sync`, remote change + `pull --rebase`, empty-remote failure, no-identity failure, auto-sync probe (missing / 4 h old `.last-sync`, `auto_sync = false`) | rebase **conflict** with abort/retry and push-retry not reproduced (needs two clones of one user/machine) |

## 3. Observations that changed the spec

Numbered for the review; `→` names the spec location and, where flagged, the ledger entry. Bare cell names refer to `walk/index-single-real.tsv`; sandbox cells are prefixed `cells-multi/` or `cells-sync/`.

1. `layouts.md` §14 Help was stale (no weekly, `lb`/`lbh`, `update`/`shell-init`/`skill`, eleven flags). → replaced verbatim from `help-long`.
2. `usage.md` said `--dry-run` misuse exits 1 in one place and 2 in another; binary: 2 (`s-dryrun-*`). → 2 everywhere.
3. `usage.md` said `--interval` "requires `--watch`"; binary accepts it silently (`s-interval-nowatch`). → corrected, DC-03.
4. `layouts.md` §2 claimed a single-source snapshot is titled by tool; binary prints `📊 Combined Usage (daily)` (`s-cc`). → corrected, DC-15.
5. Snapshot JSON: zero-usage tools lack `label`; `--by-machine` adds `machines` only to tools with data (`r-snapshot-json`, `m-bymachine-json`). → pinned, DC-01.
6. Single-tool history `--by-machine --json` is a bare array with `machines` per entry (`r-cc-h-bymachine-json`); pivot/`lbh` JSON are tool/user → array maps; `lb` JSON is a rank array with `delta: null` for `new`. → JSON table.
7. CSV: snapshot omits zero rows, pivot keeps every column (`s-csv` vs `s-h-csv`); leaderboard `share`/`delta` variable precision (`r-m-lb-csv`, `m-lb-csv`); `lbh` columns alphabetical (`r-m-lbh-csv`, `m-lbh-csv`) while the table ranks them. → CSV table, DC-05, DC-11, DC-16.
8. Markdown pivot drops exact-zero columns only (Gemini `$0.04` kept — `r-h-md`); ANSI uses the significance rule; CSV none. → DC-06.
9. `lb` heading lacks `📊`; `lbh` has it (`r-m-lb`, `r-lbh`). → DC-07.
10. `lbh --top 2` renders `others` first when its total is largest (`r-m-lbh-top2`). → DC-08.
11. `tu --help` lists no `--version`; `-v` works (`version-v`). → DC-09.
12. JSON floats carry summation artifacts (`4.936068800000001`). → DC-10.
13. Width: `COLUMNS` ignored when piped (all `r-narrow*`/`r-width*` identical to 80-col TTY output, `tty/h-80`); one-shot tables never go compact (`tty/snap-50` wraps); compact exists only in watch (`watch/snap-50x20`); footer wraps at 80 (`tty/h-80`). Bars: pivot 17 at 80, 30 at ≥100; single-tool history none below 121; `lb` 19 at 80, 30 at 120. → Terminal width section, layouts §21, DC-12.
14. `--until`-only leaderboard heading `· → DATE ·`, all Δ `new` (`r-lb-until-only`, `m-lb-until-only`). → DC-13.
15. `lbh` in single mode says `lb requires multi mode` (`s-lbh`). → DC-14.
16. Watch: single-tool history and `lbh` wrap at 100 cols and corrupt the frame (`watch/cc-h-100x30-1-first`, `watch/lbh-100x30-1-first`); history rows truncated to the terminal height (`watch/pivot-100x30-2-second`: 15 rows); stats values carry `~`, session delta signed; separator width follows the grid; Ctrl-C and `q` both replay the table (`snap-ctrlc-afterquit`, `*-4-afterquit`). → Watch sections, DC-17.
17. Sync: `pull --rebase origin main` hard-coded (`cells/182-m-sync` first run, empty remote); commit failure (no identity) prints only the generic error (`cells-multi/006-m-sync`); success `Synced to ~/.tu/metrics_repo`; commit message UTC-dated while day-files are local-dated (`cells-sync/008-repo-log` vs `011-dayfile-today`). → Sync Flow, DC-18, DC-21.
18. Dry-run: `(new)`, `(update: $X → $X)`, `Would skip N file(s) (never-shrink guard):` with `incoming $A < existing $B` and no thousands separators; equal-cost rewrites listed and counted (`cells-sync/001-sync-dryrun`, ad-hoc `(new)` capture after deleting two day-files). → Dry Run, layouts §20, DC-22, DC-23.
19. Never-shrink + max-merge: inflating today's day-file to $10,724 then `tu --fresh` showed $11,724.37 (snapshot took the stored file over the live $5.72; othermach $999.99 summed) and the dry-run skipped the file (`cells-multi/075…078`, `cells-sync/012-lb-after-sync`). → Metrics Repo Layout.
20. `auto_sync`: with `.last-sync` deleted or 4 h old, `tu`/`tu h` never synced; `auto_sync = false` only flips the status line and `tu sync` still works (`cells-sync/013…020`). → Configuration, Staleness, DC-20.
21. Path/channel inconsistencies (`~` vs absolute; auto-clone stderr vs init-metrics stdout; git chatter passthrough) (`m-snapshot-autoclone.err`, `m-initmetrics-url`, `notgit-initmetrics`, `vnew-status`). → DC-19.
22. Snapshot 12-wide cells overflow with 14-char values (`r-u-all-m`, `r-bymachine-m`). → DC-24.
23. Empty states: `  No usage` (snapshot), `  No data` (history), `No data` + footer (lb). → Empty results.
24. `init-metrics <url>` three stdout lines; env vs URL precedence confirmed (`m-initmetrics-env-vs-url-remote` = URL); legacy seeding + appended `metrics_repo` block (`m-initmetrics-legacy`). → Setup Commands.
25. `help-dump` envelope keys/order, `commands: []`, `text` == `--help` output exactly, which already ends with a newline (python check on `help-dump`); `skill` byte-identical to `docs/site/skill.md` (`cmp`); shell-init: bash/zsh/fish cover every flag and token, none list `help-dump`; no-arg usage on stderr exit 2. → Toolkit Contracts.
26. Flag guards: the three-way policy (warn / silent / exit 2) enumerated from the stderr dump. → Global Flags, DC-03.
27. `tu sync --json` performs a real sync (`m-sync-json`, and the intake-time probe on the real repo). → Setup Commands, DC-02.

## 4. Memory requirement mapping

Each requirement bullet of the six content files, mapped to where the observable behavior now lives, or classified `internal` (function names, module structure, test files, build mechanics — not a surface). `log.md`/`log.seed.md` files carry no requirements; they were used to check whether a behavior had a recorded origin (criterion 2).

### cli/data-pipeline.md

| Bullet | Spec |
|--------|------|
| positional grammar | CLI Grammar |
| sources incl. aliases | Sources |
| periods, `w`/`wh`, flag stripping before positionals | Periods; Global Flags (parsing note) |
| displays `lb`/`lbh`, no `dlb` shorthands, source scoping | Display |
| global flag list incl. `-j`, `--until` long-only, `--full`, `--top`, `--dry-run`, `-t`, `--skip-brew-update` raw-argv | Global Flags |
| `--by-machine` compat matrix, single-mode one column | Global Flags; layouts §17 |
| `--user` semantics, same-user path | Global Flags |
| `-u all` repo-only aggregate, `Users:` legend, JSON `machines` user keys | Global Flags; JSON Output; layouts §17 |
| `all` reserved profile, exit 2, both paths | Configuration; Exit Codes |
| `aggregateMachineMap` shared tail | internal |
| leaderboard ranking purity, tie-break, zero-row omission, share/delta formulas | Snapshot vs History › Leaderboard |
| current/previous windows, `prev` label, `--until`-only → all `new` | Leaderboard; DC-13 |
| `lb`/`lbh` single-mode fail-fast exit 1 | Exit Codes; DC-14 |
| `-u` on leaderboards pins, `-u all` no-op | Global Flags |
| `--top` parsing, errors, warn elsewhere, `lb` collapse, `lbh` fold, machine formats | Global Flags; Leaderboard tables; CSV/JSON/MD; DC-08 |
| `--by-machine` on `lb`/`lbh` | Global Flags; layouts §17 |
| leaderboard two-dispatch-path convention, `_lastRender*` globals | internal |
| `maxMergeEntries` semantics | Metrics Repo Layout (own-machine max-merge) |
| own-user merge pipeline order | Metrics Repo Layout |
| format-flag mutual exclusion, `--watch` incompatibility, exit 2 | Global Flags; Exit Codes |
| `-j` alias, canonical wording | Global Flags |
| `--since`/`--until` shapes, shape-only validation, errors, inverted | Global Flags |
| window applied before roll-up, all paths and formats, cache untouched | Snapshot vs History; Caching |
| `--metric` parsing, `-t` shorthand, errors, reaches every display, machine formats ignore | Global Flags; Output Formats intro |
| prev map / machine maps valued in metric; JSON cost-valued | Delta Indicators; JSON Output |
| `--since`/`--until` on snapshot warn once | Global Flags |
| implicit 3-month cap, floor date, weekly partial week | Snapshot vs History |
| monthly exempt, snapshots never capped | Snapshot vs History |
| explicit bound disables cap entirely | Snapshot vs History |
| `--full` behaviors incl. silent no-op on `mh`, warning on snapshot/`lb` | Global Flags |
| `--dry-run` parsed globally, misuse exit 2 | Global Flags; Dry Run |
| `runSync` dry-run guards and report | Dry Run |
| `--interval` range | Global Flags |
| exit-code convention, site classification, `EXIT_USAGE` const | Exit Codes (constant: internal) |
| non-data dispatch before grammar, `init-metrics` positional rule, `$HOME`-free set | Setup Commands |
| `tu skill` contract | Toolkit Contracts › skill |
| `tu shell-init` contract, coverage, no-arg stderr | Toolkit Contracts › shell-init |
| `tu update --help` short-circuit | Toolkit Contracts › update |
| `tu update` brew flow, `HOMEBREW_NO_ASK`, no timeout | Toolkit Contracts › update |
| `--skip-brew-update` | Global Flags; Toolkit Contracts › update |
| single ccusage binary, per-tool subcommand, fetch failure warning, `maxBuffer` | Fetching |
| cache TTL, `--fresh` | Caching |
| `TOOLS` registry, vendor/dev binary path, insertion order = column order | Sources; Fetching (path resolution: internal) |
| `needsFilter` defensive no-op | internal |
| label normalization, `labelKey` | Fetching (key: Data Model) |
| client-side weekly/monthly, Sunday anchoring, UTC arithmetic | Fetching |
| `currentLabel` local time | Snapshot vs History |
| `outputFormat` enum plumbing | internal |

### display/formatting.md

| Bullet | Spec |
|--------|------|
| four layouts | Output Formats |
| render/print twins | internal |
| eighths bars | Single-Tool History Table |
| bar width 10–30 | Terminal width and color |
| delta indicators when prev map | Delta Indicators |
| compact < 60 | Watch Mode › Layout; DC-12 (**over-claim**, see §5) |
| `NO_COLOR` / `--no-color` | Terminal width and color |
| color function set | layouts › Color Reference |
| `stripAnsi` | internal |
| Total row only when >1 tool | Snapshot Table |
| combined Cache column, 87-char row, 12-wide cells, skeleton alignment, CSV `cache` column | Snapshot Table; layouts §1, §8; DC-24 |
| leaderboard render options, heading, columns, data-sizing, pin glyph, `No data` | Leaderboard Table; layouts §5 |
| leaderboard bar placement, reserve, watch arrow | Leaderboard Table; Delta Indicators |
| `--top` collapsed line rules | Leaderboard Table; layouts §19 |
| staleness footer text | Leaderboard Table; Staleness |
| `lbh` hooks: title, `total-desc`, leader highlight, no omission | Leaderboard History Table; layouts §6 |
| `EmitOptions` fields, leaderboard JSON shape, `lbh` machine formats | JSON/CSV/MD tables (option names: internal) |
| pivot per-tool widths, 96-char contract | All-Tools History Pivot Table; layouts §4 |
| space-less pivot arrow, 97 | Delta Indicators; layouts §4 |
| bar-area indicator reserve | layouts §4 (formula: internal) |
| `significantTools` rule, fallbacks, MD exact-zero, CSV none | Pivot table; DC-06 |
| `fmtCost` separators, `csvCost` raw, data-sized columns, `COST_WIDTH` 9, `COMPACT_COST_W` 12 | Table semantics › Number formatting, Data-sized columns; Watch › Layout |
| exact-zero dimming | Table semantics |
| cap heading hint via `periodLabel` | Snapshot vs History; headings (helper: internal) |
| watch uses same renderers | Watch › Architecture |
| machine columns letter-coded, shared width, legend noun | layouts §17 |
| machine totals in Total row | layouts §17 |
| machine columns omitted in compact | Watch › Layout |
| `emitCsv` four kinds and rules | CSV Output |
| `emitMarkdown` four kinds and rules | Markdown Output |
| CSV machine columns `machine_{name}_cost` alphabetical | CSV Output |
| MD machine columns named directly | Markdown Output |
| one fetch per dispatch | Output Formats intro |
| month separators | Table semantics |
| current-period marker | Table semantics |
| weekend dimming | Table semantics |
| summary footer + legend | Table semantics |
| metric-generic rendering, snapshot metric-neutral | Output Formats; Snapshot Table |
| p95 two-zone scale geometry | Table semantics |
| stacked segments apportionment, palette | Table semantics |

### configuration/config-system.md

| Bullet | Spec |
|--------|------|
| `resolveConfigPaths` shape/constants | internal (paths: Toolkit Contracts › config-home) |
| `$HOME`-only, `ConfigHomeError` exit 1 | Setup Commands ($HOME dependence); config-home |
| lazy resolution, `$HOME`-free commands | Setup Commands |
| env-pinning test | internal |
| cascade order | Configuration |
| `org.conf` format | Configuration |
| user-conf selection, one deprecation line, stderr only, never moved | Configuration |
| `readConfig` injectable shape | internal |
| INI format | Configuration |
| version tracking + warning names the path read | Configuration (field table) |
| sentinels | Configuration |
| `~` expansion for `metrics_dir` | Configuration |
| `TuConfig` fields | internal (observable ones in the field table) |
| derived mode | Configuration |
| `TU_METRICS_REPO` precedence, CLI beats env | Configuration; Setup Commands (init-metrics) |
| `mode` ignored | Configuration |
| `auto_sync` default/falsy | Configuration; DC-20 |
| `init-conf` writes/seeds, messages | Setup Commands; layouts §20 |
| `init-conf` commented-field hint, no `mode` in scaffold | Setup Commands; Configuration (defaults file) |
| `status` fields and path rule | layouts §13 |
| `status` `Org config:` rule | layouts §13 |
| relative last-sync format | Staleness; layouts §13 |

### sync/multi-machine.md

| Bullet | Spec |
|--------|------|
| multi mode required | Setup Commands (sync) |
| repo layout | Metrics Repo Layout |
| one entry per file | Metrics Repo Layout |
| `writeMetrics` decisions | Dry Run (return shape: internal) |
| never-shrink guard | Metrics Repo Layout |
| `readRemoteEntriesByMachine` / `readRemoteEntries` | internal (behavior: own-machine max-merge, `-u`) |
| `listUsers` exclusions | Metrics Repo Layout |
| `readAllUsersByUserMachine` | internal (behavior: `lb --by-machine`) |
| `all` reserved | Configuration |
| `mergeEntries` sums `[INFERRED]` | Metrics Repo Layout (**verified**, see §5) |
| own-user max-merge pipeline | Metrics Repo Layout |
| `syncMetrics` steps, execFile, rebase abort, retry, commit message helper | Sync Flow (helper: internal) |
| `fullSync` live/dry-run overloads | internal (behavior: Sync Flow, Dry Run) |
| dry-run `writeMetrics` decisions | Dry Run |
| skip ⇔ guard | Dry Run |
| `fullSync` dry-run steps, local git half | Dry Run |
| `DrySyncReport` shape, commit message shared | Dry Run (shape: internal) |
| `wouldCommit` over-prediction | Dry Run; DC-23 |
| `isStale` 3 h | Staleness; DC-20 |
| staleness footer on leaderboard | Leaderboard Table |
| `--sync` inline | Global Flags |
| auto-clone attempt | Auto-Clone Guard |
| clone-failed marker, 3 h | Auto-Clone Guard |
| `init-metrics [url]` full behavior | Setup Commands |
| fallback to single with warning | Auto-Clone Guard |
| `scripts/repair-metrics.mjs` | **not a binary surface** — a repo script; not specified (B6 "repair helper parity" is a plan row concern) |

### watch-mode/tui.md

| Bullet | Spec |
|--------|------|
| alt screen | Watch Mode |
| cursor hidden/restored | Watch Mode; Interaction |
| exit replays output | Interaction |
| `--interval` | Global Flags |
| Enter/Space refresh | Interaction |
| three panels + rain | Architecture |
| rain 107 ms own timer | Architecture; Matrix Rain |
| rain overlay writes | internal |
| rain below/right modes | Layout |
| drop count height-scaled, constants | Layout; Matrix Rain |
| ≤20 rows byte-identical | Layout |
| stats grid content/colors | Session Stats; layouts §8 |
| dim rule | Layout |
| `--` placeholders | Session Stats |
| session `$0.00` before 2 polls | Session Stats |
| rate stats gated on 2 polls | Session Stats |
| burn rate 5-poll window | Session Stats |
| breakpoints 60 | Layout |
| skeleton + rain from first tick | Layout; layouts §8 |
| right-margin gutter 2, min 10 cols | Layout |
| resize re-layout | Interaction (**partially verified**, see §5) |
| push-driven footer | Architecture |
| flush re-emits rain | Architecture |
| raw stdin | internal |

### build/toolchain.md

| Bullet | Spec |
|--------|------|
| esbuild build, tests, TS config, ESM, `bin`, `prepublishOnly`, files, engine, deps, vendoring, platform mapping, fail-loud guard, BIN resolution, license, tap | internal / repo (not binary surfaces); the only observable consequence — `ccusage` exec'd directly — is in Fetching |
| Go transition rules | internal (constitution) |
| `help-dump` in-binary, envelope, no `captured_at`, flat `commands: []`, `text` = help + `\n`, build-time defines | Toolkit Contracts › help-dump |
| `scripts/help-dump.mjs` wrapper, `help/tu.json` gitignored, shll.ai pull cron | repo/external — not a binary surface |
| CI/ci-gate/release workflows | repo — not a surface |
| skill embed via `--define`, byte-identity, drift guard, 95-line bundle | Toolkit Contracts › skill (embedding: internal; line count: see §5) |

## 5. Memory statements the walk contradicts or qualifies

| File | Statement | Finding | Proposed hydrate action |
|------|-----------|---------|-------------------------|
| display/formatting.md | "Compact mode (date + cost only) MUST activate when terminal width < 60" | Only in watch mode; one-shot output never goes compact (`tty/snap-50`, `tty/h-50`) | qualify "in watch mode" |
| display/formatting.md | 12-wide snapshot cells "still hold `999,999,999,999`" | that value is 15 chars; 14-char values overflow the cell (`r-u-all-m`) | correct the claim (DC-24) |
| sync/multi-machine.md | "`mergeEntries()` MUST sum … `[INFERRED]`" | verified: `-u all` Claude Code $1,012.96 = 999.99 + 5.72 + 7.25 (`m-u-all`) | drop the `[INFERRED]` tag |
| build/toolchain.md | skill bundle "95 lines" | `docs/site/skill.md` is 117 lines (≤150 cap holds) | update the count or drop it |
| watch-mode/tui.md | "Terminal resize MUST trigger immediate re-layout" | after a tmux 100→50 resize the stats grid disappeared but the full table stayed until the next poll (`watch/resize-100to50`); tmux `resize-window` may not deliver SIGWINCH like a real terminal, so this is **unconfirmed**, not a contradiction | note as unverified; no edit |
| sync/multi-machine.md | `syncMetrics` "pull --rebase" | the branch is fixed to `origin main` (bundle grep) | add the branch name (DC-18) |

Spec-side corrections (not memory): the `--dry-run` exit-code contradiction, the `--interval requires --watch` claim, and the layouts §2 heading claim (§3 items 2–4).

## 6. Not exercised

- `tu update` without `--help` (mutates the install).
- A real `pull --rebase` conflict with rebase-abort and push retry (requires two clones of the same user/machine racing).
- `init-metrics <url>` and `sync` against the real repo (do-not-run list).
- Windows/darwin behavior; only linux-amd64 was run.
- Color bytes inside watch mode (tmux re-encodes SGR).
