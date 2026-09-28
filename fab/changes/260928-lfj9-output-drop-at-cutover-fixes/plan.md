# Plan: Output Drop-at-Cutover Fixes

**Change**: 260928-lfj9-output-drop-at-cutover-fixes
**Intake**: `intake.md`

## Requirements

### Render: snapshot JSON

#### R1: Stable snapshot JSON key set (DC-01)
Every tool object in snapshot `--json` MUST carry `label` first — for tools with and without data — valued with the current period label. The single-mode daily label clear in `src/go/internal/command/run.go` (~L402) MUST NOT remove labels from JSON output. Under `--by-machine`, every tool object MUST carry a trailing `machines` object, `{}` when the tool has no slices.

- **GIVEN** single mode, daily, Claude Code has data and Codex has none
- **WHEN** `tu --json` runs
- **THEN** both `"Claude Code"` and `"Codex"` objects start with `"label": "{today}"`
- **AND** under `--by-machine` the Codex object ends with `"machines": {}`

### View: headings and layout

#### R2: Leaderboard heading carries 📊 (DC-07)
The ANSI `lb` heading MUST be `📊 Leaderboard ({period}) · {window} · by {cost|tokens}`. Markdown headings keep their existing emoji convention.

- **GIVEN** multi mode
- **WHEN** `tu lb` runs
- **THEN** the heading line starts with `📊 Leaderboard (`

#### R3: `others` is always the last lbh user column (DC-08)
Under `lbh --top n`, the folded `others` column MUST be excluded from the descending-total column ordering and render after every user column in the ANSI table. CSV/Markdown ordering is unchanged.

- **GIVEN** `others` has a larger total than every kept user
- **WHEN** `tu lbh --top 2` runs
- **THEN** the columns are the two kept users (descending), then `others`

#### R4: One-sided leaderboard window headings (DC-13)
The `{window}` part of the leaderboard heading MUST read `until {U}` for an `--until`-only window and `since {S}` for a `--since`-only window; a two-sided window stays `{S} → {U}`. Δ semantics are unchanged.

- **GIVEN** `tu lb --until 2026-09-10`
- **WHEN** the table renders
- **THEN** the heading contains `· until 2026-09-10 ·` and every Δ is `new`

#### R5: Single-mode guard names the invoked command (DC-14)
The single-mode leaderboard guard MUST print `Error: {lb|lbh} requires multi mode — run tu init-metrics <repo-url> to set up a metrics repo`, naming the command actually run; exit 1 unchanged.

- **GIVEN** single mode
- **WHEN** `tu lbh` runs
- **THEN** stderr is `Error: lbh requires multi mode — …`, exit 1

#### R6: Single-source snapshot title names the tool (DC-15)
A single-source snapshot MUST be titled `📊 {Tool} Usage ({period})` in the ANSI table, the watch compact table, and the Markdown snapshot heading (Markdown without emoji if that is its convention); the all-tools snapshot keeps `Combined Usage`.

- **GIVEN** `tu cc`
- **WHEN** the snapshot renders
- **THEN** the heading is `📊 Claude Code Usage (daily)`

#### R7: Data-sized snapshot numeric columns (DC-24)
Each snapshot numeric column width MUST be `max(12, widest rendered cell in that column across header, data and Total rows)`; the Tool column stays 12. With all values ≤ 12 chars the output MUST be byte-identical to today.

- **GIVEN** a Tokens total of `16,809,796,832` (14 chars)
- **WHEN** the snapshot renders
- **THEN** the Tokens column is 14 wide and header, dividers and every row stay aligned

### Specs

#### R8: Spec rewritten, ledger resolved, goldens regenerated
Lines in `docs/specs/usage.md` and `docs/specs/layouts.md` carrying `(DC-01)`, `(DC-07)`, `(DC-08)`, `(DC-13)`, `(DC-14)`, `(DC-15)`, `(DC-24)` MUST describe the new behavior and drop the tag; those seven ledger entries MUST become `[DECIDED: drop]` with a `Now: … (dropped in 260928-lfj9-output-drop-at-cutover-fixes).` line; DC-05, DC-06, DC-11, DC-16 MUST become `[DECIDED: keep]` with a `Now: kept — {reason}.` line (reasons in intake § Why). Package and harness goldens MUST be regenerated and all gates green.

- **GIVEN** the change is applied
- **WHEN** the ledger is grepped for `[DECIDE:` on this branch
- **THEN** exactly DC-02, 03, 04, 09, 10, 12, 17, 18, 19, 20, 21, 22, 23 match (04/09/17 belong to the separate CLI/watch change; 18/21/22/23 to change 260928-ubws, PR #103)

### Non-Goals

- DC-12 (COLUMNS / compact layout outside watch), DC-10 (JSON number rounding), DC-02/03/19/20 — open
- Changing CSV or Markdown column sets (DC-05/06/16 are kept)

### Design Decisions

#### Stable JSON key set over presence-dependent keys
**Decision**: Every snapshot JSON tool object carries `label` (current period label) and, under `--by-machine`, `machines` (`{}` when empty).
**Why**: Consumers parse one shape; the period label is known without data.
**Rejected**: Omitting zero-usage tools from JSON — breaks the "every registry tool present" contract consumers already rely on.
*Introduced by*: 260928-lfj9-output-drop-at-cutover-fixes

## Tasks

### Phase 1: Core Implementation

- [x] T001 DC-01: `src/go/internal/render/json/snapshot.go` always emits `label` and (with a breakdown) `machines`; make zero-usage rows carry the current period label and stop the single-mode daily clear from reaching JSON in `src/go/internal/command/run.go`; tests in `render/json` (goldens) and `command` <!-- R1 -->
- [x] T002 [P] DC-07 + DC-13: `src/go/internal/view/leaderboard.go` title gets `📊 `; `src/go/internal/command/leaderboard.go` window labels `since S` / `until U`; tests <!-- R2 R4 -->
- [x] T003 [P] DC-08: `src/go/internal/command/leaderboard.go` (and the lbh column ordering in `view`) keep `others` last; tests <!-- R3 -->
- [x] T004 [P] DC-14: `ErrLeaderboardMode` in `src/go/internal/command/run.go` names the invoked command (lb/lbh); update `cmd/tu/main.go` ~L274 and e2e tests <!-- R5 -->
- [x] T005 DC-15: single-source title in `src/go/internal/view/snapshot.go`, `compact.go`, and the Markdown snapshot encoder, wired from `command/run.go` (`req.Source`); tests <!-- R6 -->
- [x] T006 DC-24: data-sized numeric columns in `src/go/internal/view/snapshot.go` following the `machineWidth` pattern; test with a 14-char value and a byte-identical normal case <!-- R7 -->

### Phase 2: Integration

- [x] T007 Regenerate package goldens (`go test ./internal/render/... ./internal/watch -update` only where the output legitimately changed), `just go-build`, regenerate harness goldens via `bin/tudiff --update` (run + live); re-run `env -u TU_METRICS_REPO -u NO_COLOR go test ./... -count=1`, `just go-lint`, `just go-diff`, `just go-live` without `--update` — all green; inspect `git diff --stat harness/golden` for only expected case groups <!-- R8 -->
- [x] T008 Update `docs/specs/usage.md` and `docs/specs/layouts.md` (drop-lines rewritten, seven `[DECIDED: drop]` + four `[DECIDED: keep]` ledger entries) <!-- R8 -->

## Acceptance

### Functional Completeness

- [x] A-001 R1: Every snapshot JSON tool object has `label`; under `--by-machine` every one has `machines` (`{}` when empty), including single-mode daily
- [x] A-002 R2: ANSI lb heading starts with `📊 Leaderboard (`
- [x] A-003 R3: `others` renders last in the ANSI lbh table under `--top` — reworked: `foldColumns` returns whether it appended `others` (`command/leaderboard.go:294`), plumbed via `HistoryOptions.FoldedLast` into `rankColumns` (`view/pivot.go:118`), which excludes the last column from the descending-total sort; verified by `TestRunLeaderboardHistoryOthersLast` (others $110 > kept $100 renders last) and the regenerated harness goldens
- [x] A-004 R4: One-sided windows read `since S` / `until U`; Δ unchanged
- [x] A-005 R5: The single-mode guard names lb or lbh as invoked; exit 1
- [x] A-006 R6: Single-source snapshot title is `📊 {Tool} Usage ({period})` in ANSI, compact, and Markdown
- [x] A-007 R7: A >12-char value widens its column with header/dividers aligned; normal snapshots byte-identical
- [x] A-008 R8: Spec lines rewritten; seven drop + four keep ledger entries resolved (grep-verified: the remaining `[DECIDE:` set is exactly DC-02, 03, 04, 09, 10, 12, 17, 18, 19, 20, 21, 22, 23, matching the scenario)

### Scenario Coverage

- [x] A-009 R1: Test covers a zero-usage tool with and without `--by-machine` in single-mode daily
- [x] A-010 R3 R4: Tests cover `others` with the largest total and both one-sided windows — one-sided windows covered (`TestLeaderboardWindows`, `TestRunLeaderboardDelta`); `others`-with-largest-total now covered through the command wiring by `TestRunLeaderboardHistoryOthersLast` (`command/leaderboard_test.go`), plus the view-level `TestTotalHistoryFoldedLast`
- [x] A-011 R7: Test covers a 14-char Tokens total

### Edge Cases & Error Handling

- [x] A-012 R8: `go test ./... -count=1`, `just go-lint`, `just go-diff`, `just go-live` all pass; golden diffs only in the affected case groups

### Code Quality

- [x] A-013 Pattern consistency: New code follows naming and structural patterns of surrounding code (options structs, named constants for titles)
- [x] A-014 No unnecessary duplication: column sizing reuses the machine-column sizing pattern; title logic lives in one place per table
- [x] A-015 Pure stages: view/render stay I/O-free and return values (constitution Go Conventions)

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Deletion Candidates

- None — this change modifies behavior in place and already removes the code it makes redundant (the single-mode label-clear block in `command/run.go`, the `all_tools_unlabeled.golden` fixture). `ErrLeaderboardMode` is kept deliberately as the `errors.Is` sentinel for the new `LeaderboardModeError`.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | Markdown headings keep their existing emoji convention; only ANSI lb gains 📊 | DC-07 is about the ANSI heading set being inconsistent | S:75 R:90 A:80 D:75 |
| 2 | Confident | Title selection is passed from command via an options field, not inferred inside view from row count | A one-tool all-tools snapshot is still "Combined"; the source is the real signal | S:80 R:90 A:85 D:80 |

2 assumptions (0 certain, 2 confident, 0 tentative).
