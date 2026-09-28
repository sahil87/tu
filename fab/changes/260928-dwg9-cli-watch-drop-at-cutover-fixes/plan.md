# Plan: CLI and Watch Drop-at-Cutover Fixes

**Change**: 260928-dwg9-cli-watch-drop-at-cutover-fixes
**Intake**: `intake.md`

## Requirements

### Command: help recognition

#### R1: `-h`/`--help` anywhere print the full help (DC-04)
`command.Parse` MUST return the help command when any positional (after the flag scan — values consumed by value-taking flags are not positionals) is `-h` or `--help`, before every other guard; the bare word `help` MUST remain first-position-only. Help MUST NOT run the named command (no sync, clone, config write, or update). Exactly one help path: if Parse now catches `tu update --help`, `runUpdate`'s own probe in `cmd/tu/main.go` is removed (or vice versa), with the update standard's probe behavior (full help incl. `--skip-brew-update`, exit 0, nothing run) preserved.

- **GIVEN** `tu cc --help`
- **WHEN** it runs
- **THEN** stdout is the full help, exit 0

- **GIVEN** `tu sync --help` in multi mode
- **WHEN** it runs
- **THEN** stdout is the full help, exit 0, and no git command runs

- **GIVEN** `tu cc help`
- **WHEN** it runs
- **THEN** it is still `Unknown argument: help`, exit 2

### Toolkit: help text

#### R2: The help lists `--version` (DC-09)
`command.FullHelp` MUST include `  --version / -V / -v  Print the version and exit` in the `Flags:` block, aligned with its neighbors, and the `Help:` line MUST reflect that `-h`/`--help` also work after a command. `help-dump` output follows automatically. Any hand-maintained flags list in `docs/site/skill.md` or README that mirrors the help MUST gain the same entry.

- **GIVEN** `tu --help`
- **WHEN** it prints
- **THEN** the Flags block contains the `--version / -V / -v` line

### Watch: frame integrity

#### R3: Frame lines never wrap (DC-17)
Before `Frame` emits them, every stats, table, and skeleton line MUST be clipped to the terminal width in columns, ANSI-aware: escape sequences pass through uncounted, visible characters stop at `cols`, and a line clipped inside an SGR run ends with `\x1b[0m`. Lines that fit MUST be byte-identical to today. Width MUST be measured in terminal columns: wide runes (emoji such as `📊`, East-Asian Wide/Fullwidth) count as 2, everything else the compositor draws (ASCII, box drawing, block bars, half-width katakana used by the rain) as 1. No width helper exists in-tree or in x/term/x/sys, so add a small pure `runeWidth` over the standard wide ranges (no new dependency). The same width measure MUST feed `maxContentWidth` in `Lay`, so the rain-zone math and the clip agree. A wide rune that would straddle the last column is dropped rather than half-drawn.

- **GIVEN** a 110-char single-tool history in a 100-column terminal
- **WHEN** watch draws a frame
- **THEN** each table line is at most 100 visible columns and the footer lands on the last row

- **GIVEN** a 45-column terminal (compact mode) and the heading `📊 Combined Cost History (daily, last 3 months)` (47 columns: the emoji is 2 wide)
- **WHEN** watch draws a frame
- **THEN** the heading is clipped to 45 columns and does not wrap onto a second row

### Specs

#### R4: Spec rewritten, ledger resolved, goldens regenerated
Lines carrying `(DC-04)`, `(DC-09)`, `(DC-17)` in `docs/specs/usage.md` and `docs/specs/layouts.md` (including the verbatim help block) MUST describe the new behavior and drop the tag; the three ledger entries MUST be `[DECIDED: drop]` with a `Now: … (dropped in 260928-dwg9-cli-watch-drop-at-cutover-fixes).` line. Package and harness goldens MUST be regenerated; all gates green.

- **GIVEN** the change is applied
- **WHEN** the ledger is grepped for `[DECIDE:` on this branch
- **THEN** DC-04, DC-09, DC-17 no longer match (other DCs are resolved in PRs #103/#104 or stay open)

### Non-Goals

- DC-02/DC-03 (off-target flag policies), DC-12 (one-shot width), DC-19, DC-20, DC-10 — open
- A general compact layout for wide tables in watch — clipping only

### Design Decisions

#### Clip, don't wrap, in watch
**Decision**: The compositor clips each frame line to the terminal width, ANSI-aware.
**Why**: The compositor's row accounting assumes one row per line; clipping preserves it with a local change and no layout redesign.
**Rejected**: Letting lines wrap and counting wrapped rows — couples the compositor to terminal wrap semantics; shrinking tables — a layout redesign out of scope (DC-12 territory).
*Introduced by*: 260928-dwg9-cli-watch-drop-at-cutover-fixes

## Tasks

### Phase 1: Core Implementation

- [x] T001 DC-04: in `src/go/internal/command/parse.go`, recognize `-h`/`--help` among all positionals before every guard (bare `help` first-only); dedupe with `runUpdate`'s probe in `src/go/cmd/tu/main.go`; table-driven tests in `parse_test.go` and e2e tests in `src/go/cmd/tu/` for `tu cc --help`, `tu h -h`, `tu sync --help` (no git), `tu update --help`, `tu cc help` (still exit 2) <!-- R1 -->
- [x] T002 [P] DC-09: add the `--version / -V / -v` line to `FullHelp` in `src/go/internal/command/request.go` and adjust the `Help:` line; update `docs/site/skill.md` / README flag lists if they mirror the help; update tests pinning the help text <!-- R2 -->
- [x] T003 [P] DC-17: ANSI-aware width clip in `src/go/internal/watch/compositor.go` applied in `Lay`/`LaySkeleton` (or `Frame`) to stats, table, and skeleton lines; unit tests with colored wide lines, fitting lines (byte-identical), and a clip inside an SGR run <!-- R3 --> <!-- rework: orchestrator tmux smoke at 45 cols — the 📊 heading still wraps (rune count 45 = 46 columns); measure terminal columns, wide runes = 2 -->

### Phase 2: Integration

- [x] T004 Regenerate package goldens (`go test ./internal/watch -update`, others only if they legitimately changed), `just go-build`, `bin/tudiff --update` (run + live); re-run `env -u TU_METRICS_REPO -u NO_COLOR go test ./... -count=1`, `just go-lint`, `just go-diff`, `just go-live` without `--update` — all green; `git diff --stat harness/golden` shows only help, help-dump, former unknown-argument-help, and watch cases <!-- R4 -->
- [x] T005 Update `docs/specs/usage.md` (~L87–88, ~L458, ledger DC-04/09/17) and `docs/specs/layouts.md` (~L197, ~L235, ~L566–567, verbatim help block) <!-- R4 -->
- [x] T006 Live smoke: build `bin/tu`, run `bin/tu h -w` in a tmux pane sized narrower than the history table (e.g. `tmux new-window` + `resize-pane -x 90`), capture it with `tmux capture-pane -p`, confirm no wrapped lines and the footer on the last row, then kill the window (fake-terminal tests have missed raw-mode issues before) <!-- R3 --> <!-- rework: re-run the smoke at 45 columns (compact mode) as well as a width narrower than the full table -->

## Acceptance

### Functional Completeness

- [x] A-001 R1: `-h`/`--help` anywhere among positionals print full help, exit 0; bare `help` still first-only
- [x] A-002 R1: Help never runs the named command (sync/init-metrics/update run nothing); exactly one help path
- [x] A-003 R2: The help Flags block lists `--version / -V / -v`; help-dump includes it
- [x] A-004 R3: Watch frame lines are clipped to the terminal width, ANSI-aware; fitting lines byte-identical
- [x] A-005 R4: Spec lines rewritten; DC-04/09/17 ledger entries `[DECIDED: drop]`

### Scenario Coverage

- [x] A-006 R1: Tests cover `tu cc --help`, `tu h -h`, `tu sync --help`, `tu update --help`, `tu cc help`
- [x] A-007 R3: Tests cover a clipped colored line, a clip inside SGR, and a fitting line; the tmux smoke confirmed no wrap

### Edge Cases & Error Handling

- [x] A-008 R1: A `-h` consumed as a flag value (e.g. after `-u`) is not treated as help — behavior matches the flag's existing missing/invalid-value handling
- [x] A-009 R4: `go test ./... -count=1`, `just go-lint`, `just go-diff`, `just go-live` pass; golden diffs only in the expected case groups

### Code Quality

- [x] A-010 Pattern consistency: New code follows naming and structural patterns of surrounding code
- [x] A-011 No unnecessary duplication: one help-detection path; clip helper reuses `StripANSI`-style parsing rather than a second ANSI parser where possible
- [x] A-012 Pure stages: the clip is a pure function over strings; no I/O added to the compositor

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Deletion Candidates

None — this change adds new functionality without making existing code redundant. The two removals it carries (runUpdate's own `--help` probe in `cmd/tu/main.go` and the first-position-only `helpCommands` map in `command/parse.go`) were planned dedups from `## Requirements` R1, already executed in the diff.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | Help detection runs over scan positionals, not raw argv | Flag values are already separated by the scan; this avoids treating `-u -h` as help | S:75 R:85 A:80 D:75 |
| 2 | Confident | A tmux live smoke is part of apply for the watch change | Memory records fake-terminal tests missing a raw-mode bug; a real terminal check is cheap | S:80 R:95 A:85 D:80 |

2 assumptions (0 certain, 2 confident, 0 tentative).
