# Intake: Dim Weekend Dates in History

**Change**: 260816-kw96-dim-weekend-dates-history
**Created**: 2026-08-16

## Origin

Conversational — a follow-up `/fab-discuss` output-DX review of `tu dh` on v0.10.1 (after the month-anchors/p95-bars and zero-column changes shipped). The agent's finding 4:

> Weekend rhythm is invisible. The June dips ($79 on 06-06, $142 on 06-07) are weekends but nothing marks them. Dimming the date cell on Sat/Sun would explain the sawtooth pattern for free — cheaper and quieter than the week separators we skipped earlier.

User instruction: "Create an intake for the 4 in the main worktree."

## Why

1. **Pain point**: Daily history views show a weekly sawtooth (low weekend usage) that reads as unexplained noise. Nothing distinguishes a Saturday from a mid-week day, so low-cost days look like anomalies rather than rhythm.
2. **Consequence of not fixing**: Users repeatedly re-derive "oh, that dip is a weekend" by counting dates; the table explains less than it could at zero width cost.
3. **Why this approach**: Dimming the date cell reuses the existing `dim` color channel (already used for dividers/footer), adds no characters (no column-width or row-shape change), and degrades to no-op under `--no-color`/`NO_COLOR`. Week separator lines were considered in the earlier month-anchors change and deliberately skipped as too heavy; cell dimming is the quiet version.

## What Changes

### Weekend date-cell dimming (daily period only)

In `src/node/tui/formatter.ts`, `renderHistory` and `renderTotalHistory`: when `period` is daily and the row label falls on a Saturday or Sunday, render the **date cell only** with `dim` — not the whole row (cost/token cells stay full-intensity; the data is not less important, only the calendar position is annotated).

- **Weekday derivation**: labels are pure ISO dates (`YYYY-MM-DD`). Parse with `new Date(label)` (ISO date-only strings parse as UTC midnight) and test `getUTCDay() === 0 || getUTCDay() === 6`. Using UTC accessors on a UTC-parsed date makes the weekday a pure calendar fact, immune to local timezone.
- **Precedence with the today marker**: the existing today marker (boldWhite date cell) wins — a weekend today renders boldWhite, not dim. One cell, one style.
- **Scope**: ANSI renderers only, daily period only (monthly labels have no weekday). Compact mode (watch <60 cols) is excluded — its label column doubles as the only identifier and dimming harms scanability there. CSV/Markdown emitters carry no color and are untouched.
- **Month separators / totals / footer**: unaffected; the dim date cell composes with the existing row construction (the date cell is already styled independently for the today marker, so this extends the same seam).

Mock (June window; 06-06/06-07 and 06-13/06-14 are weekend days, shown here as ░dimmed░):

```
2026-06-05 |   $137.92 █▋
░2026-06-06░ |    $79.35 █
░2026-06-07░ |   $142.23 █▊
2026-06-08 |   $482.06 █████▉
```

### Versioning

Color-only change: no table shape, column widths, or parseable structure changes. Constitution "Output Stability" lists color usage under the SHOULD-stay-stable clause, but nothing breaks downstream parsing — a patch release is acceptable; fold into the next minor if one is already queued.

## Affected Memory

- `display/formatting`: (modify) weekend date-cell dimming rule and its precedence with the today marker

## Impact

- `src/node/tui/formatter.ts` — `renderHistory`, `renderTotalHistory` date-cell styling; small weekday helper
- Formatter tests — cases: Saturday/Sunday cells dim, weekday cells not, weekend-today renders boldWhite (precedence), monthly period unaffected, `--no-color` output byte-identical to pre-change
- `docs/specs/layouts.md` — §3/§4 color notes gain the weekend-dim rule; Color Reference table row for `dim` gains "weekend dates"

## Open Questions

*(none)*

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | Dim the date cell only, not the whole row | Weekend data is not less important; the annotation is about calendar position. Whole-row dimming would visually demote real cost data | S:65 R:85 A:80 D:70 |
| 2 | Confident | Today marker takes precedence over weekend dim | Both style the same cell; today is the stronger, rarer signal | S:60 R:90 A:85 D:80 |
| 3 | Certain | Weekday via `new Date(label).getUTCDay()` | ISO date-only strings parse as UTC midnight; UTC accessors make the weekday timezone-independent | S:70 R:90 A:95 D:90 |
| 4 | Confident | Daily period + ANSI renderers only; compact mode excluded | Monthly has no weekday; compact's label is the only row identifier; emitters carry no color | S:60 R:85 A:85 D:80 |
| 5 | Confident | Patch-level release acceptable (no shape change) | Constitution's minor-bump mandate targets breaking output changes; color-only additions don't break parsers, and NO_COLOR output is byte-identical | S:55 R:80 A:75 D:70 |

5 assumptions (1 certain, 4 confident, 0 tentative, 0 unresolved).
