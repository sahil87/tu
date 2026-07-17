# Plan: Adopt Toolkit `skill` Standard

**Change**: 260717-uch0-adopt-skill-standard
**Intake**: `intake.md`

## Requirements

### Skill Bundle: Canonical `docs/site/skill.md`

#### R1: Canonical agent usage bundle exists
A new file `docs/site/skill.md` SHALL be the single canonical source of the `tu` agent usage bundle. It MUST be a **usage briefing** (when-to-use, capabilities map, composition patterns, output/exit-code contracts, gotchas) — NOT a README clone and NOT a flag reference table. It MUST be **static-only**: no timestamps, no environment lookups, no dynamic content. Its length MUST be ≤150 lines.

- **GIVEN** a repo checkout
- **WHEN** `docs/site/skill.md` is opened
- **THEN** it contains the five standard sections keyed to tu's real CLI surface
- **AND** the file is at most 150 lines with no dynamic markers (dates, `process.env`, timestamps)

#### R2: Bundle documents tu's ACTUAL exit-code behavior
The bundle's output/exit-code contract MUST reflect the binary's real behavior: **exit 0 on success, exit 1 on any error** (usage errors, incompatible flags, unknown shell, and — where surfaced — data-source failures). It MUST NOT assert a `0/1/2` exit convention, because tu implements no such split (the usage-error `1→2` change is a deferred, unadopted backlog item `[8h6g]`). It MUST document `stdout` = data, `stderr` = diagnostics/warnings, and the `--json`/`-j`, `--csv`, `--md` output formats as data-command-only.

- **GIVEN** the shipped `tu` binary
- **WHEN** any command errors (bad grammar, incompatible flags, unknown shell)
- **THEN** the process exits with code 1
- **AND** the bundle documents exactly this (0 success / 1 error), never a fabricated `0/1/2` convention

#### R3: Bundle content matches the real command grammar
The capabilities map MUST reflect the real sources (`cc`, `codex`/`co`, `oc`, `gemini`/`gem`, `copilot`/`cop`, `all` default), periods (`d`/`w`/`m` plus `dh`/`wh`/`mh`), display tokens (bare snapshot, `h`/`history`), and the real non-data subcommands (`init-conf`, `init-metrics`, `sync`, `status`, `update`, `shell-init <sh>`, `help`, `help-dump`, and `skill` itself). Gotchas MUST cover cached data + `--fresh`/`-f`, single vs multi mode via `~/.tu.conf` (shown by `tu status`), `--watch`/`-w` being an interactive TUI agents should not invoke, `--no-color` for parseable output, and the implicit 3-month daily/weekly history cap (`--full` to disable).

- **GIVEN** the bundle's capabilities map
- **WHEN** compared against `FULL_HELP` and `cli/data-pipeline` memory
- **THEN** every documented source/period/subcommand is one the binary actually accepts

### Runtime: `tu skill` Subcommand

#### R4: `tu skill` prints the bundle byte-identically to stdout
The CLI MUST accept a `skill` non-data subcommand that writes the bundle to stdout **byte-identical** to `docs/site/skill.md`, with **empty stderr** and **exit code 0**. No rendering, pager, or added framing. It MUST be dispatched in the non-data command block in `src/node/core/cli.ts` alongside `shell-init`/`help-dump`, before grammar parsing.

- **GIVEN** the built binary
- **WHEN** `tu skill` is invoked
- **THEN** stdout is byte-identical to `docs/site/skill.md`, stderr is empty, and exit code is 0

#### R5: Bundle resolution module with build define + dev fallback
A new `src/node/core/skill.ts` MUST export the resolved bundle string `SKILL_MD`, mirroring the `__PKG_*__` typeof-guard pattern in `cli.ts:31-58`. In the built bundle, an esbuild `--define:__SKILL_MD__` string constant supplies the content (static, no I/O — Constitution III). In dev/tsx (define absent), it reads the canonical `docs/site/skill.md` from the repo via `readFileSync(new URL(...))`. The shipped bundle MUST NOT perform a runtime file read.

- **GIVEN** the bundled binary (define present)
- **WHEN** `SKILL_MD` is read
- **THEN** it resolves from the `__SKILL_MD__` define with no filesystem access
- **AND** under tsx (define absent) it resolves by reading the canonical `docs/site/skill.md`

### Build: Embed + Drift Guard

#### R6: Build-time embed via `--define:__SKILL_MD__`
`scripts/build.sh` MUST add a `SKILL_DEF=$(node -p 'JSON.stringify(require("fs").readFileSync("docs/site/skill.md","utf8"))')` step and pass `--define:__SKILL_MD__="$SKILL_DEF"` to the esbuild invocation, following the exact `__PKG_*__` pattern already present. Byte-identity is by construction (the define reads the canonical file) — there is no committed embedded copy and no sync script.

- **GIVEN** `scripts/build.sh`
- **WHEN** the build runs
- **THEN** `docs/site/skill.md` is JSON-stringified and injected as `__SKILL_MD__`, and the bundle carries the bundle content statically

#### R7: Post-build drift guard
`scripts/build.sh` MUST, after the esbuild step (and after the vendor step so a full build is validated), run a post-build check that fails the build if the shipped binary's `skill` output drifts from the canonical file: `node dist/tu.mjs skill | cmp -s - docs/site/skill.md` — on mismatch, print `error: tu skill output drifted from docs/site/skill.md` to stderr and `exit 1`. This is the layer that guards the shipped artifact (tsx tests never exercise `dist/tu.mjs`).

- **GIVEN** a build where the bundle content and `docs/site/skill.md` disagree
- **WHEN** `scripts/build.sh` runs the post-build check
- **THEN** the build exits non-zero with the drift error on stderr

### Discoverability: Help + Completions

#### R8: `FULL_HELP` lists `tu skill`
A `tu skill` line MUST be added to `FULL_HELP` in `src/node/core/cli.ts` in the Setup/non-data block, e.g. `tu skill             Print agent usage bundle (markdown)`. This is additive; `help-dump` captures help text dynamically, so no help-dump changes are needed.

- **GIVEN** `tu --help` output
- **WHEN** the Setup block is read
- **THEN** a `tu skill` line is present describing the agent usage bundle

#### R9: Completions list `skill` for all shells
`skill` MUST be added to the non-data subcommand lists in `src/node/core/completions.ts` for **all three shells**: the bash `non_data_subcommands` string (line 19), the zsh `non_data_subcommands` array (line 73), and the fish `__fish_use_subcommand` completion block (add a `complete -c tu -n '__fish_use_subcommand' -a 'skill' -d '...'` line).

- **GIVEN** any of the bash/zsh/fish completion scripts
- **WHEN** the non-data subcommand list is inspected
- **THEN** `skill` is present alongside `help`, `init-conf`, ..., `shell-init`

### Testing: Co-located Drift + Contract Tests

#### R10: Co-located `skill.test.ts` pins bundle invariants and CLI contract
A new `src/node/core/__tests__/skill.test.ts` (tsx runner, co-located per constitution) MUST assert: (a) `SKILL_MD` is byte-identical to `docs/site/skill.md`; (b) the bundle is ≤150 lines; (c) a best-effort static-only sanity check (no obvious dynamic markers); (d) the CLI `skill` dispatch writes the bundle to stdout, writes nothing to stderr, and does not exit non-zero (following the existing `completions.test.ts` mock-capture pattern). The `completions.test.ts` `NON_DATA_SUBCOMMANDS` list MUST also gain `skill` so completion coverage verifies the new entry.

- **GIVEN** the test suite run under `npx tsx --test`
- **WHEN** `skill.test.ts` runs
- **THEN** all four assertion groups pass and the completions coverage test includes `skill`

### Non-Goals

- No `shll`/shll.ai side changes — the site renders `docs/site/skill.md` automatically from the already-pulled `docs/site/**` tree.
- No `shll agent-setup` work (forward design, "planned, not yet built").
- No changes to `run-kit context` or any other tool.
- No committed embedded copy of the bundle and no sync script — byte-identity is by construction (build reads the canonical file).

### Design Decisions

1. **Embed via `--define:__SKILL_MD__`, not an esbuild text loader**: reuses the established `__PKG_*__` build pattern — zero new build machinery. *Rejected*: text loader (adds a new esbuild mechanism for no gain).
2. **`skill.ts` typeof-guard with dev/tsx `readFileSync` fallback**: mirrors the `cli.ts` `__PKG_*__`/`readDevPkg` pattern. "No runtime file read" binds the shipped bundle (define present), not dev mode. *Rejected*: a third committed copy of the content.
3. **Byte-identity by construction, guarded by test + post-build `cmp`**: the shll `go:embed` sync step exists for Go's in-package-copy requirement; esbuild `--define` reads the canonical file directly, so a sync script would guard a copy that does not exist. Drift is instead caught by the tsx test (dev path) and the `cmp` post-build check (shipped artifact).
4. **Document exit 0 / exit 1, never `0/1/2`**: every tu error path calls `process.exit(1)`; there is no `2` convention (the `1→2` usage-error change is deferred backlog `[8h6g]`). Fabricating `0/1/2` would violate the standard's accuracy point.

## Tasks

### Phase 1: Bundle content (canonical source)

- [x] T001 Author `docs/site/skill.md` — a ≤150-line static-only usage briefing with the five standard sections, keyed to the real CLI surface (`FULL_HELP`, `cli/data-pipeline` memory): when-to-use, capabilities map (data grammar + all non-data subcommands incl. `skill`), composition patterns (ccusage/git/brew; shll.ai pull cron via `help-dump`; forward `shll agent-setup`), output/exit-code contract (stdout=data, stderr=diagnostics, `--json`/`--csv`/`--md`, exit 0 success / exit 1 error), gotchas (cache + `--fresh`, single vs multi mode via `~/.tu.conf`/`tu status`, `--watch` interactive TUI, `--no-color`, implicit 3-month cap + `--full`) <!-- R1 R2 R3 -->

### Phase 2: Runtime resolution + dispatch

- [x] T002 Create `src/node/core/skill.ts` exporting `SKILL_MD: string` via the `__SKILL_MD__` typeof-guard with a dev/tsx `readFileSync(new URL("../../../docs/site/skill.md", import.meta.url), "utf8")` fallback, mirroring `cli.ts:31-58` <!-- R5 -->
- [x] T003 Add `runSkill()` to `src/node/core/cli.ts` as `process.stdout.write(SKILL_MD)` (import `SKILL_MD` from `./skill.js`) and wire `if (cmd === "skill") { runSkill(); return; }` into the non-data dispatch block after the `help-dump` row (`cli.ts:1324`) <!-- R4 -->

### Phase 3: Build embed + drift guard

- [x] T004 In `scripts/build.sh`, add `SKILL_DEF=$(node -p 'JSON.stringify(require("fs").readFileSync("docs/site/skill.md","utf8"))')` and pass `--define:__SKILL_MD__="$SKILL_DEF"` to the esbuild invocation, following the `__PKG_*__` define pattern <!-- R6 -->
- [x] T005 In `scripts/build.sh`, after the esbuild + vendor steps, add the post-build drift guard `node dist/tu.mjs skill | cmp -s - docs/site/skill.md || { echo "error: tu skill output drifted from docs/site/skill.md" >&2; exit 1; }` <!-- R7 -->

### Phase 4: Discoverability

- [x] T006 [P] Add a `tu skill` line to `FULL_HELP` in `src/node/core/cli.ts` Setup block (e.g. `  tu skill             Print agent usage bundle (markdown)`) <!-- R8 -->
- [x] T007 [P] Add `skill` to the non-data subcommand lists in `src/node/core/completions.ts` for all three shells: bash `non_data_subcommands` (line 19), zsh `non_data_subcommands` array (line 73), and a new fish `complete -c tu -n '__fish_use_subcommand' -a 'skill' -d '...'` line <!-- R9 -->

### Phase 5: Tests

- [x] T008 Create `src/node/core/__tests__/skill.test.ts` (tsx runner): byte-identity of `SKILL_MD` vs `docs/site/skill.md`, ≤150-line budget, static-only sanity check, and the CLI `skill` dispatch contract (stdout=bundle, empty stderr, no non-zero exit) using the `completions.test.ts` mock-capture pattern <!-- R10 -->
- [x] T009 Add `skill` to the `NON_DATA_SUBCOMMANDS` array in `src/node/core/__tests__/completions.test.ts` so completion coverage verifies the new entry <!-- R9 R10 -->

## Execution Order

- T001 blocks T002 (dev fallback reads the file) and T008 (byte-identity test reads it)
- T002 blocks T003 (`runSkill` imports `SKILL_MD`)
- T004 blocks T005 (define must exist for the built binary's `skill` output to match)
- T006, T007 are independent `[P]` and can run alongside Phases 2-3
- T008, T009 run last (they exercise the module + completions)

## Acceptance

### Functional Completeness

- [x] A-001 R1: `docs/site/skill.md` exists as a static-only usage briefing with the five standard sections, ≤150 lines
- [x] A-002 R4: `tu skill` (built binary) writes stdout byte-identical to `docs/site/skill.md`, empty stderr, exit 0
- [x] A-003 R5: `src/node/core/skill.ts` exports `SKILL_MD` via the typeof-guard; built bundle uses the define (no I/O), dev/tsx reads the canonical file
- [x] A-004 R6: `scripts/build.sh` injects `__SKILL_MD__` from `docs/site/skill.md` via `--define`, following the `__PKG_*__` pattern
- [x] A-005 R7: `scripts/build.sh` post-build check `cmp`s `tu skill` output against `docs/site/skill.md` and fails the build on drift
- [x] A-006 R8: `FULL_HELP` in `cli.ts` lists a `tu skill` line
- [x] A-007 R9: bash, zsh, and fish completion scripts all list `skill` as a non-data subcommand

### Behavioral Correctness

- [x] A-008 R2: the bundle documents exit 0 success / exit 1 error and does NOT assert a fabricated `0/1/2` convention; stdout=data, stderr=diagnostics, `--json`/`--csv`/`--md` are data-command-only
- [x] A-009 R3: every source/period/subcommand documented in the bundle is one the binary actually accepts (verified against `FULL_HELP` + `cli/data-pipeline` memory)
- [x] A-010 R4: `skill` is dispatched in the non-data block before grammar parsing (does not fall through to `parseDataArgs`)

### Scenario Coverage

- [x] A-011 R10: `src/node/core/__tests__/skill.test.ts` asserts byte-identity, ≤150-line budget, static-only sanity, and the CLI contract (stdout/empty-stderr/no-fail-exit)
- [x] A-012 R9: `completions.test.ts` `NON_DATA_SUBCOMMANDS` includes `skill`, so its coverage loop verifies the entry in all three shells

### Edge Cases & Error Handling

- [x] A-013 R5: under tsx (define absent) `SKILL_MD` resolves by reading the canonical file; the shipped bundle path never touches the filesystem
- [x] A-014 R7: the post-build guard runs against `dist/tu.mjs` (the shipped artifact tsx tests never see) and produces a non-zero exit + stderr message on drift

### Code Quality

- [x] A-015 Pattern consistency: `skill.ts` mirrors the `__PKG_*__` typeof-guard + dev fallback; `runSkill` mirrors `runShellInit`/`runHelpDump`; `build.sh` mirrors the `__PKG_*__` define block
- [x] A-016 No unnecessary duplication: no third copy of the bundle content — the define reads the canonical file, dev reads the canonical file; existing test-capture helpers/patterns reused
- [x] A-017 Functional style: `runSkill`/`SKILL_MD` are functions/plain exports (no classes); `node:` imports and `.js` import extensions used
- [x] A-018 No silent error-swallowing: `skill` has no data-source dependency (static bundle), so no fallback path is needed; the build drift guard warns on stderr and fails loud (build-time, not a runtime data source)

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Deletion Candidates

None — this change adds new functionality without making existing code redundant. The bundle is a new agent-facing surface (usage briefing genre) that does not supersede `FULL_HELP` (flag reference), `help-dump` (machine-readable contract for shll.ai), README, or any `docs/site/` page; no sync script or committed embedded copy was ever created, so nothing became obsolete.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Document exit 0 (success) / exit 1 (any error); NOT a `0/1/2` convention | Every tu error path calls `process.exit(1)` (verified across cli.ts); the `1→2` usage-error change is deferred backlog `[8h6g]`, not implemented — asserting `0/1/2` would be a fabrication the intake explicitly forbids | S:80 R:85 A:95 D:90 |
| 2 | Certain | Embed via `--define:__SKILL_MD__` (JSON.stringify of file contents), not an esbuild text loader | Intake assumption 1; established `__PKG_*__` pattern in `build.sh`; zero new build machinery | S:75 R:85 A:90 D:80 |
| 3 | Confident | `src/node/core/skill.ts` module with typeof-guard + dev/tsx `readFileSync(new URL("../../../docs/site/skill.md", import.meta.url))` fallback | Intake assumption 2; path `../../../` resolves from `src/node/core/` to repo root, mirroring how `cli.ts` walks to `package.json`; shipped bundle uses define only | S:65 R:90 A:85 D:75 |
| 4 | Confident | No sync script / committed embedded copy — byte-identity by construction; drift guard = tsx test + post-build `cmp` | Intake assumption 3; the shll sync mechanism exists only for `go:embed`'s in-package copy; `--define` reads the canonical file directly | S:70 R:80 A:75 D:65 |
| 5 | Confident | Post-build `cmp` placed AFTER the vendor step so it validates a complete build; drift message to stderr + `exit 1` | Fail-loud at build time is correct (Constitution II governs runtime, not build — mirrors the existing vendor fail-loud guard); running after vendor exercises the same artifact users ship | S:70 R:85 A:85 D:75 |
| 6 | Confident | Add `skill` to `completions.test.ts` `NON_DATA_SUBCOMMANDS` (not only the scripts) | The existing coverage test hardcodes the subcommand list; leaving it stale would either miss the new entry or (if the test asserted exact parity) fail — keeping the test aligned with the shipped scripts is the low-effort correct move | S:70 R:90 A:85 D:80 |
| 7 | Certain | Bundle content follows the standard's five-section genre with the tu-specific outline; prose authored against the real CLI surface | Intake assumption 5; standard prescribes sections, tu's surface is known and verified, prose is cheap to revise | S:75 R:85 A:80 D:70 |

7 assumptions (3 certain, 4 confident, 0 tentative).
