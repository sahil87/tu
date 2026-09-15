# Plan: Spec Reconciliation

**Change**: 260915-2y3l-spec-reconciliation
**Intake**: `intake.md`

## Requirements

### Specs: Binary walk and evidence

#### R1: Every observable surface is captured from the live binary
The apply stage SHALL run the intake §1 binary matrix against the installed `tu` v0.11.5 and record stdout, stderr, and exit code per cell in the change-folder working artifact `reconciliation.md` (or a companion capture directory it indexes), covering every source, period, display, output format, flag variant, non-data command, config mode (single, multi, org-only, legacy, unset `$HOME`), and both terminal breakpoints. Multi-mode cells that mutate state (`sync`, `init-metrics <url>`, day-file writes) MUST run only against a sandboxed temp `$HOME` with a local bare git repo, never the real metrics repo; the real config is used only for read-only observation. `tu update` without `--help` MUST NOT run.

- **GIVEN** the brew-installed binary and a scratch directory
- **WHEN** the walk runs
- **THEN** every matrix cell has a captured result the reviewer can open, and no command from the intake's do-not-run list was executed against the real `$HOME`

#### R2: Every memory requirement is mapped to a spec line or classified internal
Each requirement bullet and design decision in the six memory content files SHALL be mapped in `reconciliation.md` to the `usage.md`/`layouts.md` section that now states it, or marked `internal — not a surface` with a one-phrase reason. Memory statements the captured binary contradicts SHALL be listed separately.

- **GIVEN** the 18 memory files
- **WHEN** the mapping is complete
- **THEN** no requirement bullet is unmapped, and every contradiction has the cell that proves it

### Specs: `usage.md` completeness

#### R3: CLI grammar, flags, commands, and exit codes are complete and self-consistent
`usage.md` MUST list every token, alias, flag (long and short), non-data command, and exit-code row the binary exposes, with no internal contradiction. In particular the Global Flags table gains `--csv`, `--md`, `-j`, `--since`/`-s`, `--until`, `--by-machine`, `--skip-brew-update`, and `-v`; the Setup section gains `update`, `shell-init`, `skill`, `help-dump`, `help` and the `$HOME`-free versus config-reading split; the `--dry-run` misuse exit code reads 2 everywhere.

- **GIVEN** live `tu --help` and the captured error cells
- **WHEN** a reader looks up any flag or command
- **THEN** its syntax, aliases, applicable displays, misuse behavior, and exit code are stated once and agree with the binary

#### R4: Toolkit contracts have a home in the spec
`usage.md` MUST gain a `## Toolkit Contracts` section with one subsection per contract — `--version`, `help-dump`, `update`, `shell-init`, `skill`, config-home — each naming the governing `shll standards <name>` entry, the shll version consulted, and tu's observable behavior (output shape, stdout/stderr split, exit codes, env vars, flags). Only externally observable behavior is stated; `tu skill` is specified by reference to `docs/site/skill.md`.

- **GIVEN** the plan's Goal lists these as fixed surfaces
- **WHEN** B8 implements the toolkit layer
- **THEN** every behavior it must reproduce is stated in this section or in a section it points to

#### R5: Machine output formats are pinned per display
For each data display (snapshot, single-tool history, pivot history, `lb`, `lbh`), `usage.md` MUST state the exact JSON shape (key names, ordering, presence rules including the zero-usage `label` asymmetry, `machines` under `--by-machine`, raw number formatting), the CSV header and row rules, and the Markdown heading and table rules, plus which flags the machine formats ignore.

- **GIVEN** `tu --json`, `tu h --csv`, `tu cc h --md`, `tu lb --json`, `tu lbh --csv` and their `--by-machine` variants captured
- **WHEN** the Go port defines its output structs
- **THEN** every key, column, and formatting rule it needs is written in the spec, not inferred from TS

#### R6: Table semantics, data flow, and config rules are stated in prose
`usage.md` MUST state, in observable terms, the display semantics memory records (month separators, current-period marker, weekend dimming, summary footer, p95 two-zone scale, stacked bars and legend, negligible-column omission thresholds and fallbacks, exact-zero dimming, data-sized columns and floors, compact mode, Total-row rules, bar width bounds), the data-flow rules (own-machine max-merge, never-shrink guard, `.last-sync` and `.clone-failed` thresholds, `listUsers` exclusions, cache path and bypass, weekly anchoring, local-vs-UTC label rules), and the config rules (version warning, ignored `mode`, `~` expansion, `auto_sync` falsy values, `status` lines, `init-conf` behaviors). Semantics live in `usage.md`; visual mockups live in `layouts.md`; each behavior appears in at least one and is cross-referenced from the other when both apply.

- **GIVEN** a memory requirement about display or data behavior
- **WHEN** the reader searches `usage.md`
- **THEN** the observable rule is present without TS function names or module globals

### Specs: `layouts.md` completeness

#### R7: Mockups match the binary and cover every layout
`layouts.md` §14 MUST be a byte-exact copy of live `tu --help`; §13 MUST match captured `tu status` in single, multi, org, and legacy modes; existing mockups MUST be re-verified against captures at 80 and 100 columns; and new sections MUST cover CSV output, Markdown output, `--by-machine` columns (including `-u all --by-machine` with the `Users:` legend), `-t` variants, compact snapshot and history, `lb --top` and `lbh --top`, the staleness footer, the `last 3 months` hint, and the summary footer. The Color Reference MUST match the exported color set and stack palette order.

- **GIVEN** the captured frames
- **WHEN** a mockup is compared to its capture
- **THEN** column widths, glyphs, separators, and heading text agree, or the difference is explained in the notes

### Specs: The `[DECIDE]` ledger

#### R8: Accidental-looking behaviors are listed under `## Drop at cutover` and never decided
`usage.md` MUST end with a `## Drop at cutover` section whose intro states that entries are proposals resolved at gate G0 and consumed by R3 as expected diffs. Each entry MUST follow the intake §4 format: `DC-NN` stable ID, literal `[DECIDE: keep|drop]` bracket left unfilled, one-line behavior, `Where:` reproducing command with mode/env, one-line rationale citing which of the six criteria applies, and the spec section it back-references. The in-body spec line that documents the behavior MUST carry a trailing `(DC-NN)`; `layouts.md` mockup notes exhibiting the behavior carry the same reference. Nothing is removed from the spec.

- **GIVEN** a behavior meeting at least one classification criterion
- **WHEN** it is documented
- **THEN** it appears both as a normal spec line with `(DC-NN)` and as a ledger entry, and the bracket still reads `[DECIDE: keep|drop]`

### Specs: Index and plan bookkeeping

#### R9: Index descriptions and the plan's P1 row are updated; nothing else changes
`docs/specs/index.md` descriptions MUST name the toolkit contracts, the machine-format pins, and the ledger; the plan's P1 Status cell MUST reference this change. The diff MUST touch only `docs/specs/*.md`, `fab/plans/sahil/26-09-15-go-port.md`, files under `fab/changes/260915-2y3l-spec-reconciliation/`, and — at hydrate, per the intake's conditional Affected Memory — `docs/memory/**` lines a captured run proved wrong; no `src/`, test, build, formula, README, or `docs/site/` file changes.

- **GIVEN** the completed change
- **WHEN** `git diff --name-only` is run against the base
- **THEN** only those paths appear

### Non-Goals

- Resolving any `[DECIDE]` marker — gate G0 owns that.
- Changing behavior, help text, tests, the skill bundle, README, or `docs/site/` — D4 freeze.
- Rewriting memory beyond proven contradictions — X3 owns the rehydrate.
- Restructuring existing spec sections — existing headings and order are preserved for reviewable diffs.

### Design Decisions

#### Sandboxed multi mode for the walk
**Decision**: Multi-mode cells that write (day-files, `sync`, `init-metrics <url>`) run under a temp `$HOME` whose config points `metrics_repo` at a local bare git repository created for the walk; the real `~/.config/tu/tu.conf` is used only for read-only observation of real data shapes.
**Why**: It lets the walk capture the full sync, init-metrics, and auto-clone surfaces byte-for-byte without risking the real `wvrdz/tu-metrics` repo, and it is exactly the fixture shape P4's harness will need.
**Rejected**: Running `tu sync` against the real repo (irreversible pushes from an unattended agent); skipping the sync cells (leaves B6's contract unspecified).
*Introduced by*: 260915-2y3l-spec-reconciliation

#### One ledger in `usage.md`, back-references everywhere else
**Decision**: The `## Drop at cutover` section is the single list of `[DECIDE]` entries; spec lines and mockup notes carry `(DC-NN)` pointers rather than inline brackets.
**Why**: G0 resolves one list; R3 maps one list to harness expectations; the spec body stays a description of today's behavior.
**Rejected**: Inline `[DECIDE]` brackets scattered through both files (hard to resolve, easy to miss).
*Introduced by*: 260915-2y3l-spec-reconciliation

## Tasks

### Phase 1: Setup

- [x] T001 Run the intake §1 binary matrix with a capture script under the scratchpad: sandboxed temp `$HOME`s (single, multi with a local bare repo, org-only, legacy, unset `$HOME`), the real config for read-only multi-mode observation, `NO_COLOR` and colored variants, 80/100/50-column widths, tmux frame captures for `-w`; write `fab/changes/260915-2y3l-spec-reconciliation/reconciliation.md` with the cell index, notable observations, the memory-requirement mapping skeleton, and the list of memory statements the binary contradicts <!-- R1, R2 -->

### Phase 2: Core Implementation

- [x] T002 Reconcile `docs/specs/usage.md` grammar and commands: complete the Global Flags table, Setup/non-data commands with the `$HOME`-free split, the Exit Codes table, the Data Flow and Multi-Machine sections (max-merge, never-shrink, staleness, clone marker, `listUsers`, cache rules, weekly anchoring), the Configuration section (version warning, ignored `mode`, `~`, `auto_sync`, `status` lines, `init-conf`), and add the new `## Toolkit Contracts` section citing `shll standards` entries and the shll version <!-- R3, R4, R6 -->
- [x] T003 <!-- rework: Markdown leaderboard Total-row rule contradicted the binary (`**Total**` sits in the # cell, User is blank) — fixed in usage.md and layouts.md §16 --> Reconcile `docs/specs/usage.md` Output Formats: pin JSON/CSV/Markdown per display type from the captures, add the table-semantics prose (separators, marker, weekend dimming, footer, p95 scale, stacked bars/legend, omission thresholds, zero dimming, data sizing, compact mode, Total rules, bar bounds), and state which flags the machine formats ignore <!-- R5, R6 -->
- [x] T004 Reconcile `docs/specs/layouts.md`: replace §14 with live `tu --help` verbatim, verify §13 against captured `status` variants, re-verify existing mockups at 80/100 columns, add mockup sections for CSV, Markdown, `--by-machine` (incl. `-u all --by-machine`), `-t` variants, compact snapshot/history, `lb --top`/`lbh --top`, staleness footer, cap hint, summary footer; check the Color Reference against the palette <!-- R7 -->

### Phase 3: Integration & Edge Cases

- [x] T005 <!-- rework: DC-19 ledger entry had no (DC-19) back-reference in usage.md — added to Setup Commands init-metrics row and Auto-Clone Guard step 3 --> Write the `## Drop at cutover` ledger at the end of `docs/specs/usage.md` from the observations classified under the six criteria, add `(DC-NN)` back-references to the spec lines and `layouts.md` notes, update `docs/specs/index.md` descriptions and the plan's P1 Status cell in `fab/plans/sahil/26-09-15-go-port.md`, complete the memory-requirement mapping in `reconciliation.md`, and verify `git diff --name-only` touches only the permitted paths <!-- R8, R9, R2 -->

## Acceptance

### Functional Completeness

- [x] A-001 R1: `reconciliation.md` indexes every matrix cell in intake §1 (captures held locally, reproducible from the committed scripts), and no do-not-run command was executed against the real `$HOME`
- [x] A-002 R2: every memory requirement bullet in the six content files is mapped to a spec section or marked internal with a reason; contradictions list the proving cell
- [x] A-003 R3: `usage.md` Global Flags lists every flag from live `tu --help` plus `-j`, `-v`, and the update-scoped `--skip-brew-update`; the Setup section lists all nine non-data commands; `--dry-run` misuse reads exit 2 in every place it is mentioned
- [x] A-004 R4: `usage.md` has a `## Toolkit Contracts` section with six subsections, each citing its `shll standards` entry and stating stdout/stderr/exit behavior; `tu skill` is specified by reference to `docs/site/skill.md`
- [x] A-005 R5: JSON, CSV, and Markdown are pinned for all five data displays, including `--by-machine` key/column rules and the zero-usage `label` presence rule
- [x] A-006 R6: each display-semantics, data-flow, and config rule enumerated in R6 has a sentence in `usage.md` free of TS identifiers
- [x] A-007 R7: `layouts.md` §14 is byte-identical to `tu --help` output; §13 covers single, multi, org, legacy; the new mockup sections listed in R7 exist
- [x] A-008 R8: `usage.md` ends with `## Drop at cutover`; every entry has `DC-NN`, an unfilled `[DECIDE: keep|drop]`, `Where:`, a one-line rationale naming a criterion, and a spec back-reference; every referenced spec line carries `(DC-NN)`
- [x] A-009 R9: `docs/specs/index.md` descriptions mention toolkit contracts and the ledger; the plan's P1 Status cell names this change

### Behavioral Correctness

- [x] A-010 R3: the Dry Run subsection of `usage.md` no longer says exit 1 for misuse
- [x] A-011 R7: no stale flag or missing command remains in `layouts.md` §14 (diff against `tu --help` is empty)

### Scenario Coverage

- [x] A-012 R1: single-mode cells ran with `TU_METRICS_REPO` unset under a temp `$HOME`; multi-mode write cells ran against the local bare repo sandbox
- [x] A-013 R5: the captured `tu --json` shows the zero-usage `label` asymmetry and it is both specified and ledgered

### Edge Cases & Error Handling

- [x] A-014 R1: the unset-`$HOME` cell shows `tu: $HOME is not set; cannot locate config` with exit 1 for a config-reading command and success for a `$HOME`-free one, and the spec states both
- [x] A-015 R9: `git diff --name-only <base>...HEAD` lists only `docs/specs/*.md`, the plan file, this change's folder, and the hydrate-stage `docs/memory/**` corrections

### Code Quality

- [x] A-016 Pattern consistency: new spec text follows the surrounding style (tables for enumerations, RFC-2119 verbs where neighbors use them, no TS identifiers), and new `layouts.md` sections follow the existing Command / mockup / notes shape
- [x] A-017 No unnecessary duplication: a behavior's semantics live once in `usage.md` and its visual once in `layouts.md`, cross-referenced rather than restated; `tu skill` and `--help` text are referenced or copied verbatim, never paraphrased in two places
- [x] A-018 Readability over cleverness: ledger entries are one behavior each with a one-line rationale; no entry bundles multiple behaviors

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Multi-mode write cells run against a temp `$HOME` with a local bare git repo, not the real metrics repo | Intake §1 do-not-run list; irreversibility of pushes; the sandbox is also the P4 fixture shape | S:85 R:90 A:95 D:90 |
| 2 | Certain | The plan has five tasks, so the pipeline's light lane runs apply and hydrate inline in the orchestrator | Task count ≤ 5 by the pipeline's fork rule; the work is judgment-heavy prose the orchestrator already holds context for | S:80 R:90 A:95 D:90 |
| 3 | Confident | Toolkit standards are cited by name and shll version (v0.1.32) rather than quoted at length | Standards are versioned upstream and change; the spec states tu's behavior and points at the binding text | S:70 R:90 A:85 D:75 |
| 4 | Confident | Watch-mode frames are captured with an isolated tmux server socket at 100 and 50 columns; a capture failure falls back to memory-only reconciliation with a note | tmux 3.7 is present; the intake grades the same choice Confident | S:60 R:85 A:60 D:55 |

4 assumptions (2 certain, 2 confident, 0 tentative).
