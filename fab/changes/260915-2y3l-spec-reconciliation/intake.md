# Intake: Spec Reconciliation

**Change**: 260915-2y3l-spec-reconciliation
**Created**: 2026-09-16

## Origin

One-shot `/fab-new` invocation from the Go-port plan's Phase 0 queue (`fab/plans/sahil/26-09-15-go-port.md`, row P1), run in the `p1-spec-reconciliation` worktree. No prior discussion in this session; the plan doc's Decisions and Target architecture sections were read first, as the row's context line requires. The worktree branch was fast-forwarded to `origin/main` at intake time so that P0 (constitution v1.2.0 Go Transition article, PR #78) and the plan doc itself are present in this checkout. Raw input:

> Context: fab/plans/sahil/26-09-15-go-port.md, row P1. Read the Decisions and Target architecture sections before writing the intake. Do not change any external surface listed under Goal.
>
> Row P1 (spec-reconciliation): Walk docs/specs/usage.md and docs/specs/layouts.md against the 18 memory files and the live binary (tu --help, every display type, every format). Every behavior found in memory or the binary but absent from the specs gets a line added to the spec. Every behavior that looks accidental gets a [DECIDE: keep|drop] marker with a one-line rationale, listed under a new "Drop at cutover" section. Output: the specs become the complete contract. This row proposes; it does not decide the markers itself — those are resolved by the user at gate G0. No code changes in this row.

Gap analysis: `/docs-hydrate-specs` covers the memory→specs half of this walk (top-3 structural gaps, interactive). It does not cover the live-binary walk, the accidental-behavior classification, or the `[DECIDE]` ledger, and the plan makes P1 an explicit fab row consumed by gate G0 and by R3. Proceeding as a tracked change; the apply agent may borrow hydrate-specs' gap-ranking approach but runs the full walk, not a top-3.

## Why

**Problem.** The Go port (plan D1, D6) treats `docs/specs/usage.md` and `docs/specs/layouts.md` as the contract the differential harness (P4, R3) enforces byte-for-byte. Today the specs are incomplete and in places self-contradictory, so they cannot serve as that contract:

- `layouts.md` §14 "Help" is a stale copy of `tu --help`: it lacks the weekly period, the `lb`/`lbh` displays, the `update`/`shell-init`/`skill` setup rows, and eleven flags (`-j`, `--csv`, `--md`, `--since`, `--until`, `--top`, `--dry-run`, `-u`, `--by-machine`, `--skip-brew-update`, and the `-s` alias). Verified by diffing the mockup against the installed v0.11.5 binary at intake.
- `usage.md` "Global Flags" omits `--csv`, `--md`, `--since`, `--until`, `-j`, `--by-machine`, `--skip-brew-update`, and the lowercase `-v` alias that memory records for `--version`.
- `usage.md` contradicts itself on `--dry-run` misuse: the Dry Run subsection says exit 1, the Exit Codes table and the binary say exit 2.
- The toolkit contracts the plan's Goal lists as fixed surfaces (`--version`, `help-dump`, `update`, `shell-init`, `skill`, config-home) have no home in either spec. Memory (`build/toolchain.md`, `cli/data-pipeline.md`, `configuration/config-system.md`) documents them in detail; `usage.md` mentions `shell-init` and `update` only inside the exit-code table.
- `usage.md` pins the JSON output only as "mirrors the internal data shape". The port defines JSON as Go structs (plan, Target architecture), so every key, its presence rules, and number formatting must be written down. At intake the live binary showed zero-usage tools emitting objects without a `label` key while active tools carry one — exactly the kind of accident that must be listed, not silently ported.
- Display semantics recorded in `display/formatting.md` (weekend date dimming, month separator rows, current-period marker, summary footer, p95 two-zone bar scale, stacked pivot bars with legend, negligible-column omission thresholds, exact-zero cell dimming, data-sized columns) appear in `layouts.md` only as mockup fragments or not at all, and `usage.md` never states them.

**Consequence if skipped.** Phase 1 and 2 rows (V2, B1–B8) would each rediscover the contract from the TS source, and the harness burndown (P4) would have no authority to say whether a divergence is a port bug or an accident worth dropping. Plan risk "Scope creep under the banner of rethink" names P1's drop list as one of two guards; without it, every behavior is implicitly a keep and every port disagreement becomes an argument.

**Why this approach.** The specs are already language-neutral (plan, "The specs are already language-neutral"), so they are the right vessel. Memory is the post-implementation truth and the binary is the ground truth; the walk triangulates the three so that every observable behavior has exactly one authoritative spec line, and every questionable one has a marker the user resolves at G0. The agent proposes; it decides nothing — that is why the output is a marker ledger rather than edits to behavior.

## What Changes

No source, test, build, or formula file changes. The deliverable is edits to `docs/specs/usage.md`, `docs/specs/layouts.md`, `docs/specs/index.md`, plus one working artifact in the change folder.

### 1. Inputs and the walk procedure

**Sources of truth, in precedence order when they disagree:**

1. **The live binary** — `tu` v0.11.5 at `/home/linuxbrew/.linuxbrew/bin/tu` (brew-installed; every commit after release `5df10fd` is docs/fab only, so it is behaviorally identical to `main`). Ground truth for observable behavior.
2. **The 18 memory files** under `docs/memory/` — six content files (`cli/data-pipeline.md`, `display/formatting.md`, `configuration/config-system.md`, `sync/multi-machine.md`, `watch-mode/tui.md`, `build/toolchain.md`) carrying the requirement bullets and design decisions, plus six `log.md` and six `log.seed.md` change histories. Content files are the behavior source; log files are used to classify (a behavior with no design decision and no log entry is an accident candidate).
3. **The current specs** — the text being reconciled.

**Binary walk matrix.** Every cell below is run, its stdout/stderr/exit captured to the working artifact (§5), and checked against the specs:

| Axis | Values |
|------|--------|
| Source tokens | (none), `cc`, `codex`, `co`, `oc`, `gemini`, `gem`, `copilot`, `cop`, `kimi`, `ki`, `all`, one unknown token |
| Period tokens | (none), `d`, `daily`, `w`, `weekly`, `m`, `monthly` |
| Display tokens | (none), `h`, `history`, `dh`, `wh`, `mh`, `lb`, `lbh` |
| Formats | table, `--json`, `-j`, `--csv`, `--md`; each pairwise incompatibility; each with `--watch` |
| Flags | `--since`/`-s` and `--until` (both date shapes, malformed, inverted, on snapshot, on `lb`/`lbh`), `--full` (on `h`, `mh`, snapshot, with explicit window), `--metric cost\|tokens\|bogus\|missing`, `-t` (alone, with `--metric tokens`, with `--metric cost`), `--top n` (valid, `0`, `-1`, `abc`, missing, on non-leaderboard), `-u <self>`, `-u <other>`, `-u all`, `-u` missing value, `--by-machine` (snapshot, single-tool history, pivot, `lb`, `lbh`, with `-u all`), `--fresh`/`-f`, `--no-color`, `NO_COLOR=1`, `--interval` (valid, `4`, `3601`, non-numeric), `--no-rain`, `--dry-run` on `tu sync` and on every other command shape |
| Non-data commands | `help`, `-h`, `--help`, `--version`, `-V`, `-v`, `help-dump`, `skill`, `shell-init bash\|zsh\|fish\|(none)\|bogus`, `update --help`, `update -h`, `init-conf`, `init-metrics` (no URL and `metrics_repo` unset; two positionals), `status`, `sync --dry-run`, `cc --help` |
| Config modes | single (temp `$HOME`, `env -u TU_METRICS_REPO`), multi (this machine's real `~/.config/tu/tu.conf`), org-only (temp `$HOME` with `org.conf` and no `tu.conf`), legacy (temp `$HOME` with `~/.tu.conf` only), `$HOME` unset |
| Terminal | ≥ 60 columns and < 60 columns (`COLUMNS`/`stty` or a tmux pane) for the compact variants; TTY vs pipe |

**Do-not-run list** (this machine is in multi mode against the real `wvrdz/tu-metrics` repo): never run `tu sync` without `--dry-run`, never pass `--sync`, never run `tu update` without `--help`, never run `tu init-metrics <url>` against the real `$HOME`. Ordinary multi-mode data commands write this machine's own day-files into the local clone and may auto-sync if `.last-sync` is older than 3 hours; that is the machine's normal daily operation and is acceptable. Single-mode cells run under a temp `$HOME` with `TU_METRICS_REPO` unset (it is exported in this shell and would force multi mode; see the `tu test env leaks` note in memory).

**Watch mode** (`-w` on every display type): capture at least one full-mode frame (≥ 60 cols) and one compact frame (< 60 cols) via a tmux pane (`tmux new-session -d -x 100 -y 30 'tu -w'`, `tmux capture-pane -p`, then send `q`), plus the post-quit stdout replay. If a pane capture is not feasible in the apply environment, watch behavior is reconciled from `watch-mode/tui.md` and `layouts.md` §7–§11 alone and the working artifact says so.

### 2. Spec additions — `docs/specs/usage.md`

Every behavior found in memory or the binary and absent from the specs gets a line in the existing section that owns it, in the existing style (prose plus tables, RFC-2119 verbs where the surrounding text uses them). The boundary rule: **only externally observable behavior enters the spec** — command shapes, stdout/stderr text, exit codes, file paths and formats, env vars, timing constants a user can observe (TTLs, cooldowns). Function names, module globals, and TS-specific mechanisms stay in memory. Known additions from the intake-time sample (non-exhaustive; the walk completes the list):

- **Global Flags table**: add `--csv`, `--md`, `-j`, `--since`/`-s`, `--until` (long-only), `--by-machine`, `--skip-brew-update` (update-scoped, raw-argv detected), `-v` as a second `--version` alias; fix the `--dry-run` row and the Dry Run subsection to exit 2 (matching the Exit Codes table and the binary).
- **Setup / non-data commands**: add `tu update`, `tu shell-init <shell>`, `tu skill`, `tu help-dump`, `tu help`; state the dispatch-before-grammar rule and which commands are `$HOME`-free (`help`, `--version`, `help-dump`, `skill`, `shell-init`, `update`) versus config-reading (`tu: $HOME is not set; cannot locate config`, exit 1).
- **New `## Toolkit Contracts` section**: one subsection per contract, each naming the governing `shll standards <name>` entry and stating tu's observable behavior: `--version` output shape (`tu version vX.Y.Z`, first-line token); `help-dump` JSON envelope (`{tool, version, schema_version: 1, root: {name, path, short, usage, text, commands: []}}` — flat, `commands` always empty, `text` byte-identical to `--help`); `update` (Homebrew detection via Cellar path, non-brew message exit 0, `brew update` → `brew info --json=v2` → `brew upgrade` with `HOMEBREW_NO_ASK=1`, no timeout on upgrade, `--skip-brew-update`, `update --help` short-circuits before any brew call); `shell-init` (static bash/zsh/fish completion scripts on stdout, eval-safe: no-arg usage goes to stderr with empty stdout and exit 2, unknown shell exit 2); `skill` (raw markdown byte-identical to `docs/site/skill.md`, empty stderr, exit 0 — the spec references the file rather than duplicating it); config-home (`$HOME/.config/tu/` only, no XDG, cascade `tu.default.conf < org.conf < tu.conf < TU_METRICS_REPO < CLI`, legacy `~/.tu.conf` read-only fallback with one deprecation line, `~/.tu/` for state).
- **Output Formats**: pin the exact JSON shape for each of the five data displays (snapshot map-of-tool-to-totals, single-tool history, pivot history map-of-tool-to-entries, `lb` row array, `lbh` map-of-user-to-entries) including key names, key order, which keys are present under `--by-machine` (`machines`), number formatting (raw floats, no rounding), and per-tool presence rules; add CSV and Markdown subsections for snapshot, single-tool history, and pivot history (headers, numeric conventions, machine-column naming `machine_{name}_cost` in CSV and bare machine name in MD, Total row rules, `## {title}` heading rule) alongside the existing leaderboard ones; state that `--metric`/`-t` does not affect the three machine formats and that CSV/MD strip ANSI, bars, and arrows.
- **Table semantics** (the semantic half; mockups go to layouts.md): month separator rows, current-period label marker, weekend date dimming, the dim summary footer (`avg …`) and its ≥ 2-row condition, the p95 two-zone bar scale and its `maxCost > 1.5 × p95` trigger, stacked per-tool pivot bars and the footer legend, negligible-column omission thresholds and the two-level fallback, exact-zero cell dimming, data-sized metric/machine columns with fixed floors, the combined `Cache` column, Total-row conditions, the 10–30 char bar width rule, compact mode at < 60 columns for every layout.
- **Data flow**: own-machine snapshot max-merge (`maxMergeEntries`, tie → live entry), never-shrink guard semantics as observed (`update: X → Y` / skip), `.last-sync` staleness at 3 hours, `.clone-failed` 3-hour cooldown, `listUsers` excluding `docs`, the cache path and `extraArgs` bypass rule, weekly Sunday anchoring, local-vs-UTC rules for `currentLabel` versus `weekLabel`.
- **Config**: `version` warning when newer than 2 and which path it names, `mode` key silently ignored, `~` expansion for `metrics_dir`, `auto_sync` accepted falsy values (`false`, `0`), `status` output lines including the conditional `Org config:` line, `init-conf` commented-field warnings and legacy seeding.
- **Exit codes**: keep the table; add the rows the walk observes that are missing (`help-dump`, `skill`, `tu cc --help` → 2 unknown argument, `-u`/`--top`/`--metric` value errors already listed).

### 3. Spec additions — `docs/specs/layouts.md`

- §14 Help becomes a verbatim copy of live `tu --help` output (the `--help` text is a fixed external surface; the spec carries it byte-exact and says so).
- §13 Status verified against the binary in single and multi mode; add the `Org config:` variant.
- §12 JSON gains one example per display type (or points to the pinned shapes in usage.md, whichever keeps layouts.md a mockup document).
- New sections for the layouts with no mockup today: CSV output, Markdown output, `--by-machine` columns (snapshot and single-tool history, and `-u all --by-machine` with the `Users:` legend), `-t`/`--metric tokens` variants of the three tables, compact (< 60 col) snapshot and history, `lb --top n` collapsed line and `lbh --top n` `others` column, the staleness footer, the `last 3 months` heading hint, the summary footer.
- Existing mockups re-verified against the binary at 80 and 100 columns with `NO_COLOR=1`; column widths and glyphs corrected where they drifted. Color Reference table checked against `colors.ts` exports and the stack palette decision (green, magenta, blue, cyan positional).

### 4. The `[DECIDE]` ledger — new `## Drop at cutover` section in `usage.md`

One section, at the end of `usage.md`, introduced by a short paragraph stating that every entry is a proposal resolved at gate G0 and that R3 applies the resolved `drop` entries to the harness matrix as expected diffs. Entry format, one per behavior:

```markdown
- **DC-07** `[DECIDE: keep|drop]` Zero-usage tools in snapshot `--json` omit the `label` key while active tools carry it.
  Where: `tu --json` (any mode) — `"Codex": {"totalCost": 0, …}` vs `"Claude Code": {"label": "2026-09-16", …}`.
  Why it looks accidental: the key set differs by data presence, not by design; no memory requirement or design decision mentions it; consumers must special-case it.
  Spec line: Output Formats › JSON Output (snapshot).
```

Rules:

- IDs are `DC-NN`, sequential, stable once assigned (R3 references them).
- The in-body spec line that documents the behavior carries a trailing back-reference `(DC-NN)` so the behavior is specified as it exists today and simultaneously flagged. Nothing is removed from the spec by this row.
- `Where:` names the exact command and, when relevant, the config mode and env, so R3 can turn a `drop` into an expected-diff matcher without re-deriving it.
- The rationale is one line. The agent never fills in `keep` or `drop`; the bracket stays as written.
- `layouts.md` does not host a ledger; a mockup exhibiting a flagged behavior gets the same `(DC-NN)` back-reference in its notes.

**Classification criteria — a behavior "looks accidental" when at least one holds:**

1. It is inconsistent with a sibling behavior for the same class of situation (e.g. `--top` on a non-leaderboard display warns and continues with exit 0 while `--dry-run` off `tu sync` fails fast with exit 2; `tu sync --json` silently runs a real sync while data commands reject format-flag conflicts).
2. No memory requirement or design decision covers it, and no log entry introduced it.
3. Memory itself hedges it: `[INFERRED]`, "defensive no-op", "retained pending cleanup", "structurally unreachable", "sanctioned heuristic", "may over-predict".
4. It leaks an implementation detail into an external surface (JSON key presence depending on data, TS internal names in output, human-readable month labels in a place ISO labels are the rule).
5. It contradicts a toolkit standard (`shll standards <name>`) or the constitution's Output Stability or Graceful Degradation clauses (e.g. `tu cc --help` exiting 2 as an unknown argument).
6. Snapshot CSV omits zero-usage tools while history CSV emits all six tool columns — asymmetries between formats of the same display.

Behaviors that pass none of the criteria are plain spec lines with no marker; anything not in the ledger is a keep by default (plan, Risks).

### 5. Working artifact — `fab/changes/260915-2y3l-spec-reconciliation/reconciliation.md`

A coverage matrix kept in the change folder (excluded from true-impact by `config.yaml`, never under `docs/`): for each memory requirement bullet, the spec section that now covers it or `internal — not a surface`; for each binary matrix cell, the captured stdout/stderr/exit summary and the spec line it verifies; the list of memory statements the binary contradicted. It exists so the reviewer and gate G0 can audit completeness without redoing the walk. It is not part of the shipped docs.

### 6. Index and cross-references

- `docs/specs/index.md`: extend the `usage` description to name toolkit contracts and the Drop-at-cutover ledger; extend `layouts` to name the new format/variant mockups.
- `fab/plans/sahil/26-09-15-go-port.md`: row P1's Status column moves to the change/PR reference when the row ships (the plan says rows update their status column as they land). No other plan edits.

### 7. Explicitly out of scope

- Resolving any `[DECIDE]` marker — gate G0.
- Changing behavior, help text, tests, `docs/site/skill.md`, README, or `docs/site/` — D4 freeze plus the Goal's fixed-surface list. The skill bundle is referenced, not edited.
- Rewriting memory. Memory statements the binary contradicts are listed in the working artifact; correcting them is left to the hydrate stage only where the contradiction is proven by a captured run, and otherwise deferred to X3.
- Restructuring the specs (`/docs-reorg-specs` territory). New sections are added; existing ones keep their headings and order so diffs stay reviewable.
- Version bump — no output changes, so Output Stability is untouched.

## Affected Memory

No memory file is required to change: this row alters no behavior. Hydrate appends the change's entry to the six domain `log.md` files as usual. Conditional content edits, limited to statements the captured binary runs contradict (itemized in `reconciliation.md` by the apply stage):

- `cli/data-pipeline.md`: (modify, conditional) only where a captured run contradicts a requirement bullet
- `display/formatting.md`: (modify, conditional) same rule
- `configuration/config-system.md`: (modify, conditional) same rule
- `sync/multi-machine.md`: (modify, conditional) same rule — note the `mergeEntries` bullet is already tagged `[INFERRED]`
- `watch-mode/tui.md`: (modify, conditional) same rule
- `build/toolchain.md`: (modify, conditional) same rule

## Impact

- **Files edited**: `docs/specs/usage.md` (290 lines today; expected to roughly double), `docs/specs/layouts.md` (442 lines; new mockup sections), `docs/specs/index.md` (two description cells), `fab/plans/sahil/26-09-15-go-port.md` (P1 status cell). New: `fab/changes/260915-2y3l-spec-reconciliation/reconciliation.md`.
- **No code, tests, build, CI, formula, or `docs/site/` changes.** `npm test` is not affected; the review stage verifies the diff touches only the paths above.
- **Downstream consumers**: gate G0 (resolves the ledger), P4 `tudiff run` (argument matrix should be seeded from §1's binary matrix), R3 (applies resolved `drop` entries as expected diffs), V2/B1–B8 (implement against the spec lines), X3 (memory rehydrate references the same spec).
- **Toolkit standards**: the new Toolkit Contracts section cites `shll standards` entries (`version`, `help-dump`, `update`, `shell-init`, `skill`, `config-home`, `install-composition`, `principles`) and records the shll version consulted, per the constitution's Toolkit Standards clause. Since no CLI surface, help output, README, or `docs/site/` changes, the clause's pre-change check is satisfied by citation.
- **Environment hazards for the apply agent**: `TU_METRICS_REPO` is exported in the shell (forces multi mode; unset it for single-mode cells), the machine has a real metrics repo (do-not-run list in §1), and the worktree has no `node_modules` (irrelevant — the brew binary is the oracle, nothing is built).

## Open Questions

- None blocking. The two softest choices, both graded Confident with a stated fallback, are assumptions 13 (tmux-based watch-mode frame capture, memory-only fallback) and 14 (hydrate may correct a memory line only when a captured run proves it wrong; everything else defers to X3). Either can be overridden via `/fab-clarify` before apply.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | The "18 memory files" are the six domain content files plus the six `log.md` and six `log.seed.md` files; content files supply behavior, logs classify accidents | `find docs/memory -name '*.md' ! -name index.md` returns exactly 18; the plan's count matches | S:85 R:95 A:100 D:95 |
| 2 | Certain | The installed brew binary v0.11.5 is the oracle for the binary walk, not a fresh build | Every commit after release `5df10fd` is docs/fab-only (`git log`); the worktree has no `node_modules`; the plan calls the live binary the ground truth | S:90 R:90 A:95 D:90 |
| 3 | Certain | Only externally observable behavior enters the spec; TS function names, module globals and internal mechanisms stay in memory | Plan: specs are language-neutral and the port's contract; the Go target replaces every internal name | S:90 R:85 A:95 D:95 |
| 4 | Certain | No version bump, no code, test, help-text, skill-bundle, README or `docs/site/` edits | Row text "No code changes"; D4 freeze; Goal's fixed-surface list; Output Stability needs a bump only for output changes | S:95 R:90 A:100 D:100 |
| 5 | Certain | `layouts.md` §14 Help becomes a byte-exact copy of live `tu --help`; `tu skill` is specified by reference to `docs/site/skill.md`, not duplicated | `--help` is a fixed surface per the Goal and B8 requires byte-identical help; skill byte-identity is already a memory requirement | S:90 R:90 A:95 D:90 |
| 6 | Certain | The `[DECIDE]` ledger lives as one `## Drop at cutover` section at the end of `usage.md`, with `DC-NN` IDs, a `Where:` command line, and `(DC-NN)` back-references on the in-body spec lines; `layouts.md` hosts no ledger | Row says "a new Drop at cutover section" (singular); R3 must map each drop to a harness case, so the entry needs the reproducing command and a stable ID; a single ledger is easier to resolve at G0 | S:75 R:90 A:80 D:70 |
| 7 | Certain | Toolkit contracts get a new `## Toolkit Contracts` section in `usage.md`, one subsection per `shll standards` entry, citing the standard and recording the shll version | Goal lists them as fixed surfaces; memory documents them fully; the constitution's Toolkit Standards clause wants surfaces checked against named standards; B8 ports them | S:80 R:85 A:85 D:75 |
| 8 | Certain | JSON output is pinned per display type (keys, order, presence rules, number formatting) rather than left as "mirrors internal shape" | Target architecture: JSON becomes Go structs, so the schema must be explicit; observed `label`-key asymmetry shows the current wording hides accidents | S:80 R:85 A:85 D:85 |
| 9 | Confident | Accidental-behavior classification uses the six criteria in §4; anything unflagged is a keep | Plan Risks: "Anything not in the drop list is a behavior to keep"; the criteria are derived from concrete observations (flag-misuse inconsistency, memory hedges, format asymmetries) | S:75 R:85 A:75 D:70 |
| 10 | Certain | The walk includes a coverage-matrix working artifact in the change folder, not under `docs/` | Gate G0 and the reviewer need an audit trail; `true_impact_exclude` already excludes `fab/`; specs stay size-controlled per `docs/specs/index.md` | S:65 R:95 A:85 D:75 |
| 11 | Confident | Multi-mode cells run against this machine's real config (day-file writes and possible auto-sync are normal operation); single/org/legacy cells run under a temp `$HOME` with `TU_METRICS_REPO` unset; `tu sync`, `--sync`, `tu update`, and `init-metrics <url>` on the real `$HOME` are never run | `tu status` shows multi mode with auto-sync on; config paths are `$HOME`-only per memory; the env-leak note names `TU_METRICS_REPO`; the do-not-run list keeps the walk read-only where it matters | S:70 R:80 A:85 D:80 |
| 12 | Certain | `docs/specs/index.md` descriptions and the plan's P1 status cell are updated in this row; no other plan edits | Plan header: "update the status column as they land"; the index is the specs' landing page | S:70 R:95 A:90 D:85 |
| 13 | Confident | Watch-mode frames are captured via a detached tmux pane at ≥ 60 and < 60 columns, with a memory-only fallback | tmux is present on this host (fab pane dispatch depends on it) but the apply environment is not guaranteed; the fallback keeps the row unblocked | S:55 R:85 A:45 D:45 |
| 14 | Confident | Hydrate may correct a memory line only when a captured binary run proves it wrong; all other memory work defers to X3 | Memory is the authoritative post-implementation truth and a proven-wrong line is a bug; but the row's stated output is the specs, and X3 rewrites memory anyway | S:45 R:80 A:55 D:40 |

14 assumptions (10 certain, 4 confident, 0 tentative, 0 unresolved).
