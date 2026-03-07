# Spec: Fix Watch Mode Display Bugs

**Change**: 260306-5hu8-fix-watch-mode-display-bugs
**Created**: 2026-03-06
**Affected memory**: `docs/memory/watch-mode/tui.md`, `docs/memory/display/formatting.md`

## Watch Mode: Session Stats Rate Guard

### Requirement: Tokens/min MUST NOT display until meaningful elapsed time

The session stats panel SHALL suppress the Tokens/min stat until at least two poll cycles have completed (`pollHistory.length > 1`). This prevents a divide-by-near-zero producing absurd values (e.g., ~3 trillion tokens/min) on the first render when `elapsedMin` is a tiny fraction of a minute.

The guard SHALL use `session.pollHistory.length > 1` — the same condition already used by Session cost delta (line 72 of `src/node/tui/panel.ts`). This is consistent: both stats require observed change over time, not a single instantaneous sample.

#### Scenario: First render (0s elapsed)
- **GIVEN** watch mode has just started and only one poll has completed (`pollHistory.length === 1`)
- **WHEN** `buildPanel()` renders the session stats
- **THEN** the Tokens/min stat SHALL NOT appear in the output
- **AND** Elapsed time SHALL still appear (it does not depend on poll history)

#### Scenario: After second poll
- **GIVEN** watch mode has completed two or more polls (`pollHistory.length >= 2`)
- **WHEN** `buildPanel()` renders the session stats
- **THEN** the Tokens/min stat SHALL appear with a value computed from `totalTokens / elapsedMin`

#### Scenario: Zero tokens after multiple polls
- **GIVEN** `pollHistory.length >= 2` but `session.totalTokens === 0`
- **WHEN** `buildPanel()` renders the session stats
- **THEN** the Tokens/min stat SHALL NOT appear (existing `totalTokens > 0` guard still applies)

## Display: Total Row Guard in Snapshot Layouts

### Requirement: Total row MUST only render when more than one tool has visible data

In `renderTotal()` (`src/node/tui/formatter.ts`), the divider and Total row SHALL only render when more than one tool row is visible (i.e., more than one tool has `totalTokens > 0`). This matches the existing guards in `renderHistory`, `renderTotalHistory`, `renderCompactHistory`, and `renderCompactTotalHistory`.

The same guard SHALL apply to `renderCompactSnapshot()`, which also unconditionally renders the divider and Total row. This function was missed in the intake but has the same bug.

#### Scenario: Single tool with data in renderTotal
- **GIVEN** `toolTotals` contains entries for multiple tools but only one has `totalTokens > 0`
- **WHEN** `renderTotal()` renders the table
- **THEN** only the single tool row SHALL appear
- **AND** no divider or Total row SHALL be rendered

#### Scenario: Multiple tools with data in renderTotal
- **GIVEN** `toolTotals` contains two or more tools with `totalTokens > 0`
- **WHEN** `renderTotal()` renders the table
- **THEN** all non-zero tool rows SHALL appear
- **AND** a divider and Total row SHALL be rendered with aggregate values

#### Scenario: Single tool with data in renderCompactSnapshot
- **GIVEN** compact mode is active and only one tool has `totalTokens > 0`
- **WHEN** `renderCompactSnapshot()` renders the compact table
- **THEN** only the single tool row SHALL appear
- **AND** no divider or Total row SHALL be rendered

#### Scenario: Multiple tools with data in renderCompactSnapshot
- **GIVEN** compact mode is active and two or more tools have `totalTokens > 0`
- **WHEN** `renderCompactSnapshot()` renders the compact table
- **THEN** all non-zero tool rows SHALL appear
- **AND** a divider and Total row SHALL be rendered

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Use `pollHistory.length > 1` guard for Tokens/min | Confirmed from intake #1 — matches existing Session cost delta guard on line 72; consistent pattern across panel stats | S:90 R:95 A:95 D:95 |
| 2 | Certain | Count visible rows (non-zero `totalTokens`) for Total guard | Confirmed from intake #2 — matches existing guards in renderHistory, renderTotalHistory, renderCompactHistory, renderCompactTotalHistory | S:95 R:95 A:95 D:95 |
| 3 | Certain | Test runner is `npx tsx --test` | Confirmed from intake #3 — constitution specifies this | S:95 R:90 A:95 D:95 |
| 4 | Certain | No changes to CLI interface or config | Confirmed from intake #4 — pure display bug fixes | S:90 R:95 A:90 D:95 |
| 5 | Certain | `renderCompactSnapshot` also needs Total row guard | Code inspection: lines 324-326 unconditionally render divider + Total; intake listed it as already guarded but it is not. Upgraded from intake Confident #5 scope | S:95 R:95 A:95 D:95 |
| 6 | Certain | Rate/Proj. day already guarded by `computeBurnRate` returning null with < 2 polls | Confirmed from intake #5 — `computeBurnRate` at line 21 returns null when `pollHistory.length < 2`; line 87 checks `rate !== null` | S:95 R:90 A:95 D:95 |

6 assumptions (6 certain, 0 confident, 0 tentative, 0 unresolved).
