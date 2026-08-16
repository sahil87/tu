# Intake: Green-Dominant Stack Palette

**Change**: 260816-w61k-green-dominant-stack-palette
**Created**: 2026-08-16

## Origin

Conversational follow-up to change `260816-3tah-stacked-tool-bars-pivot` (shipped in v0.10.2), dispatched promptless via `/fab-proceed`.

> Reorder the stacked-bar tool palette so the dominant tool renders dark green again.

Change 3tah introduced per-tool colored segments in the cross-tool pivot history bars with `STACK_PALETTE = [cyan, magenta, blue, green]` assigned in visible pivot column order. Claude Code is the first visible column, so it got cyan — the history chart flipped overnight from its long-standing solid dark green (ANSI `\x1b[32m`, still used by the unstacked single-tool bars) to teal-dominant. The user wants the dominant look restored: dark green for the first column (Claude Code, in practice always the dominant tool).

Key decisions from the conversation (see Assumptions for grades):
1. Reorder `STACK_PALETTE` so slot 0 is `green`.
2. Keep the assignment **positional** (visible column order) — cost-ranked assignment was considered and rejected.
3. Recommended full order: `[green, magenta, blue, cyan]` — the slot-3/4 ordering is a recommended default ("minor either way"), not a hard decision.

## Why

1. **Pain point**: v0.10.2's stacked pivot bars assigned cyan to the first visible column (Claude Code). Since Claude Code dominates the user's cost data, the whole history chart reads teal — a jarring overnight break from the familiar dark-green chart the unstacked single-tool history still renders.
2. **Consequence of not fixing**: the pivot history permanently loses its visual continuity with the single-tool history and with every pre-v0.10.2 release; the dominant bar color contradicts the tool's established green identity for "the main bar".
3. **Why this approach**: reordering the palette so slot 0 is green makes the chart read as "the same green bar as before, with small colored slivers for the other tools" — a one-array-literal change. The alternative (dynamically assigning green to whichever tool has the highest cost share) was rejected because it makes colors unstable: colors would swap mid-history if another tool overtook Claude Code for a period, hurting day-to-day comparability. Positional assignment is deterministic and gives the same result today.

## What Changes

### 1. Palette reorder in `src/node/tui/formatter.ts`

The array literal at line 199 (plus its adjacent comment if it references order):

```ts
// before
const STACK_PALETTE: Array<(s: string) => string> = [cyan, magenta, blue, green];

// after
const STACK_PALETTE: Array<(s: string) => string> = [green, magenta, blue, cyan];
```

Effective per-tool mapping (visible column order is registry order filtered to nonzero-cost tools; registry order is Claude Code, Codex, OpenCode, Gemini, Copilot, Kimi — see `src/node/core/fetcher.ts`):

| Visible column slot | Tool (user's typical data) | Old color | New color |
|---------------------|---------------------------|-----------|-----------|
| 1 | Claude Code | cyan `\x1b[36m` | **green `\x1b[32m`** |
| 2 | Codex | magenta `\x1b[35m` | magenta (unchanged) |
| 3 | Gemini | blue `\x1b[34m` | blue (unchanged) |
| 4 | Kimi | green `\x1b[32m` | **cyan `\x1b[36m`** |

Slot-3/4 rationale: ANSI blue is the muddiest color on dark backgrounds, so it goes to the rarest tool; in the user's real data Kimi appears more often than Gemini, so Kimi gets the more legible cyan. The alternative `[green, magenta, cyan, blue]` was discussed and called "minor either way" — the chosen order is a recommended default (Confident), not a hard decision.

**Constraint — yellow MUST NOT be used in the palette**: the overflow zone past the p95 scale-break rule renders solid yellow (`renderScaledBar`, formatter.ts:185), and a yellow segment would be ambiguous with it. This reservation is already documented (memory bullet 260816-3tah; layouts.md Color Reference) and MUST survive this change.

The footer legend swatches use the same palette via `stackedBarPalette()` (formatter.ts:204, consumed at formatter.ts:613), so the legend follows automatically — **no separate legend change**.

Under `--no-color`/`NO_COLOR` the segments collapse to solid blocks; that output is byte-identical before and after this change.

### 2. Test expectations in `src/node/tui/__tests__/formatter-stacked.test.ts`

Tests assert segment colors/order and MUST be updated to the new palette (Constitution Test Integrity: tests conform to spec — updating expectations to the new palette is the correct direction). Known touch points:

- Line 28: local mirror `const PALETTE = [cyan, magenta, blue, green]` → `[green, magenta, blue, cyan]`
- Lines ~121–145: "colors segments in palette order" — first-slot assertions move from cyan (`\x1b[36m`) to green (`\x1b[32m`); e.g. line 145 `cyan("██") + magenta("▎")` → `green("██") + magenta("▎")`
- Lines ~203–212: "assigns cyan/magenta/blue in column order" → green/magenta/blue; the zero-rounded-share check (3rd column Kimi, blue) is positionally unchanged
- Lines ~267–269: footer legend swatches — Claude Code swatch `\x1b[36m█` → `\x1b[32m█`; Codex magenta and Kimi (3rd column in that fixture) blue unchanged
- Line ~163: explicit 5-tool palette literal `[cyan, magenta, blue, green, identity]` → `[green, magenta, blue, cyan, identity]`
- Lines ~327–333: "keeps green bars with no stacked palette colors" (single-tool history) — the exclusion set `{magenta, blue, cyan█}` happens to remain exactly the non-green stacked colors under the new order, but assertion text/comments should be re-checked for accuracy (green is now both the single-tool bar color and stack slot 0)

Any other color-order assertions found in the file follow the same slot mapping (slot 1 cyan→green, slot 4 green→cyan, slots 2–3 unchanged).

### 3. Spec updates in `docs/specs/layouts.md`

Three documented locations MUST be updated to match the new order:

- **Line ~107** (Layout 4 intro prose): "the Claude Code share of each row renders cyan, the Codex share magenta" → "renders green, the Codex share magenta"
- **Line ~127** (Layout 4 stacking bullet): "fixed palette **cyan, magenta, blue, green**, assigned in visible column order" → "**green, magenta, blue, cyan**"
- **Lines ~353–357** (Color Reference table): swap the pivot-segment slot annotations —
  - `green` row: "single-tool history bars, up-arrow delta, pivot bar segment (4th tool)" → "(1st tool)"
  - `cyan` row: "pivot bar segment (1st tool)" → "(4th tool)"
  - `magenta` (2nd tool) and `blue` (3rd tool) rows unchanged
  - `yellow` row (overflow-zone reservation) unchanged

### 4. Memory (hydrate stage)

`docs/memory/display/formatting.md` covers the stacked pivot tool bars and footer legend; its stacked-bar bullet quotes `STACK_PALETTE = [cyan, magenta, blue, green]` literally (line ~48) and MUST be updated. See Affected Memory.

### Versioning

Constitution "Output Stability": color usage SHOULD stay stable across patch versions. This is an immediate follow-up correction to a palette that shipped yesterday (v0.10.2), which the conversation deemed acceptable as a patch. No structural output change; `--no-color` output is byte-identical.

## Affected Memory

- `display/formatting`: (modify) update the stacked-bar bullet's `STACK_PALETTE` literal to `[green, magenta, blue, cyan]` and any legend-example color references; the yellow overflow-zone reservation and all other stacked-bar semantics are unchanged

## Impact

- **Code**: `src/node/tui/formatter.ts` — one array literal (line 199) plus its adjacent order-describing comment. No logic changes; `stackedBarPalette()`, `apportionSegments()`, `renderStackedScaledBar()`, and the legend rendering are untouched.
- **Tests**: `src/node/tui/__tests__/formatter-stacked.test.ts` — color expectation updates only (see What Changes §2).
- **Docs**: `docs/specs/layouts.md` — three locations (see What Changes §3).
- **Behavior**: ANSI color assignment of pivot history bar segments and footer legend swatches. Bar lengths, characters, apportionment, overflow zone, `--no-color` output: all byte-identical.
- **Systems/APIs**: none. JSON/CSV/Markdown emitters carry no bar colors.

## Open Questions

- None — all decision points were resolved in the originating conversation or are within agent competence (see Assumptions).

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Reorder `STACK_PALETTE` so slot 0 is `green` — first visible pivot column (Claude Code) renders dark green | Discussed — explicit user decision; single array literal, trivially reversible | S:95 R:90 A:95 D:95 |
| 2 | Certain | Keep assignment positional (visible column order), NOT cost-ranked | Discussed — user rejected dynamic ranking: colors would swap mid-history if another tool overtook Claude Code, hurting comparability | S:95 R:85 A:90 D:90 |
| 3 | Confident | Full order `[green, magenta, blue, cyan]` — slot 3 blue (Gemini, rarest), slot 4 cyan (Kimi, more frequent) | Discussed — recommended default; alternative `[green, magenta, cyan, blue]` deemed "minor either way"; blue is muddiest on dark backgrounds so it goes to the rarest tool | S:70 R:90 A:70 D:60 |
| 4 | Certain | Yellow stays excluded from the palette | Hard constraint — overflow zone past p95 rule renders solid yellow; a yellow segment would be ambiguous with it | S:95 R:70 A:95 D:95 |
| 5 | Certain | No separate legend change — footer swatches follow via `stackedBarPalette()` | Code fact verified at formatter.ts:204/613; legend consumes the same palette array | S:90 R:95 A:100 D:95 |
| 6 | Confident | Ship as a patch version despite Output Stability's color-stability SHOULD | Discussed — immediate correction to a palette that shipped yesterday (v0.10.2); no structural change, `--no-color` byte-identical | S:75 R:60 A:65 D:70 |
| 7 | Certain | Update test expectations to the new palette (not the implementation to the old tests) | Constitution Test Integrity — tests conform to spec; the spec (layouts.md) is being updated to the new order | S:85 R:90 A:95 D:95 |
| 8 | Certain | Update all three `docs/specs/layouts.md` locations (intro prose ~107, stacking bullet ~127, Color Reference ~353–357) in this change | Spec/implementation consistency; locations verified against the file | S:90 R:95 A:95 D:95 |

8 assumptions (6 certain, 2 confident, 0 tentative, 0 unresolved).
