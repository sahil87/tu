# Intake: Pivot Zero-Column Omission + Cost Thousands Separators

**Change**: 260815-q6fx-pivot-zero-columns-cost-separators
**Created**: 2026-08-15

## Origin

Conversational — a `/fab-discuss` session on improving tu's output DX. The agent reviewed live output of `tu`, `tu h`, and `tu cc h` and proposed five improvements; the user selected ideas 1 and 3 for this change:

> Idea 1: Drop (or dim) all-zero tool columns in the pivot. `tu h` today is six columns of which five are `$0.00` for every one of ~90 rows. Omitting no-data tools (like the snapshot already does) would collapse it to `Date | Claude Code | Cost`, restore the 80-col fit, and give the bar chart room back.
>
> Idea 3: Cost column lacks thousands separators. `$4031.61` sits next to token cells with commas (`606,138,574`).

User instruction: "Create an intake for 1 and 3."

## Why

1. **Pain point**: The cross-tool cost pivot (`tu h`, `tu dh`, `tu mh`) gives every registered tool a column even when the tool has zero cost across the entire visible window. With 6 registered tools and typically 1–2 in active use, ~80% of the table is `$0.00` noise. The all-zero columns also push the full data row to 90 chars, which (a) breaks the 80-col fit and (b) suppresses the inline bar chart at 90–100 col terminals (`MIN_BAR_AREA` threshold) — the zeros literally crowd out the useful visualization. Separately, cost cells render without thousands separators (`$4031.61`) while token cells have them (`606,138,574`) — inconsistent and harder to scan.
2. **Consequence of not fixing**: Every history invocation buries the signal in dead columns; users on standard 80-col terminals get wrapped rows; four-figure daily costs (which occur in real data) are misread at a glance.
3. **Why this approach**: The snapshot renderer (`renderTotal`) already omits zero-token tool *rows* — omitting zero-cost tool *columns* in the pivot applies the same established precedent. Thousands separators via `toLocaleString` matches the existing `fmtNum` implementation exactly.

## What Changes

### 1. Pivot omits all-zero tool columns

In `src/node/tui/formatter.ts`, `renderTotalHistory()` (the ANSI pivot) currently builds one column per key of `allToolEntries` (line ~323, `const toolNames = [...allToolEntries.keys()]`). Change: after building the `costMap`, filter `toolNames` to tools with a **nonzero total cost across the visible labels** (the labels remaining after the `maxRows` truncation, so watch mode filters on what is actually displayed). Tools with all-`$0.00` columns are omitted entirely — not dimmed (omission was the user's stated preference: "Omitting no-data tools (like the snapshot already does)").

Concrete before/after for current real data (only Claude Code has cost in the window):

Before (90 chars wide, bars suppressed below 101 cols):
```
Date       | Claude Code |    Codex | OpenCode |   Gemini |  Copilot |     Kimi |     Cost
2026-06-12 |    $4031.61 |    $0.00 |    $0.00 |    $0.00 |    $0.00 |    $0.00 | $4031.61
```

After (bars fit again on an 80-col terminal):
```
Date       | Claude Code |      Cost
2026-06-12 |   $4,031.61 | $4,031.61  ██████████████████████████████
```

Edge cases:
- **All tools zero for the visible window**: cannot happen with nonempty labels in practice (a label exists only if some entry produced it), but guard defensively — if the filter yields an empty set, fall back to the unfiltered tool list.
- **Exactly one tool remains**: keep the pivot shape (Date | Tool | Cost) — do not special-case to the single-tool layout; the row `Cost` column stays (it is the delta-indicator and bar anchor in watch mode).
- **Watch mode**: a tool crossing from $0.00 to nonzero mid-watch legitimately adds its column on the next render; the compositor re-measures every frame, so a layout change between polls is safe. Delta keys (`total:{label}`) are label-based and unaffected.

Scope of the omission rule:
- **ANSI pivot** (`renderTotalHistory`): omit zero columns — the primary change.
- **Markdown emitter** (`emitMarkdownTotalHistory`): apply the same omission — it targets human paste contexts (PRs/Slack).
- **CSV emitter** (`emitCsvTotalHistory`): do NOT change — CSV is a machine contract; scripts may index columns positionally.
- **Compact mode / snapshot / single-tool history**: unaffected (no per-tool columns or already row-omitting).

### 2. Thousands separators in human-facing cost cells

In `src/node/tui/formatter.ts`, change `fmtCost`:

```ts
// before
export function fmtCost(n: number): string {
  return `$${n.toFixed(2)}`;
}
// after
export function fmtCost(n: number): string {
  return `$${n.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
}
```

This automatically propagates to every ANSI renderer and to the Markdown emitter (`mdCost` delegates to `fmtCost`). **CSV is untouched** — `csvCost` uses `toFixed(2)` independently and must stay raw-numeric per its documented contract (RFC 4180, no separators).

Width consequences (must be handled, not ignored):
- `COST_WIDTH` is 8; `$9,999.99` is 9 chars. Bump `COST_WIDTH` 8 → 9 so four-figure costs don't overflow the cell via `padStart` (the existing comment at `MIN_TOOL_COL_WIDTH` documents that overflow widens the row and can wrap the watch compositor).
- `MIN_TOOL_COL_WIDTH` is 8 (floors per-tool pivot columns); bump 8 → 9 for the same reason ($1,000+ single-tool daily cells exist in real data).
- Recompute and update the row-width comments in the formatter (the 90-char math) and the corresponding notes in `docs/specs/layouts.md` §4. With zero-column omission in place, the typical rendered width drops well below 80 despite the +1/+2 col widths.
- `COMPACT_COST_W` is 12 — already fits `$99,999.99`; no change.

### 3. Version bump

Constitution "Output Stability": breaking output changes require a minor version bump. Both changes alter parseable table output → this change ships with a **minor** version bump (coordinate with the release flow; if the release is cut separately, note the requirement in the PR body).

## Affected Memory

- `display/formatting`: (modify) pivot column-omission rule, fmtCost separator format, updated width constants

## Impact

- `src/node/tui/formatter.ts` — `renderTotalHistory`, `fmtCost`, `emitMarkdownTotalHistory`, width constants (`COST_WIDTH`, `MIN_TOOL_COL_WIDTH`) and their comments
- `src/node/tui/__tests__/` (or wherever formatter tests live) — update expected strings for cost formatting; add cases: zero-column omission, single-remaining-tool pivot, ≥$1,000 cost cell width, CSV unchanged
- `docs/specs/layouts.md` — §4 pivot mockup + width math, §1–3 cost examples, Color Reference unaffected
- Watch mode (`src/node/tui/watch.ts`, `compositor.ts`) — no code change expected; verify no wrap at the new widths
- Version: minor bump required

## Open Questions

*(none)*

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | Omit zero columns entirely rather than dim them | Discussed — user's idea 1 framed omission as primary ("Drop (or dim)"), and it's what restores the 80-col fit; dimming keeps the width problem | S:75 R:85 A:80 D:70 |
| 2 | Confident | Zero-column filter applies to ANSI + Markdown, not CSV | CSV documented as machine contract with positional columns; Markdown targets human paste contexts | S:60 R:80 A:85 D:75 |
| 3 | Confident | Filter on cost within the *visible* (post-maxRows) window | Matches what the user sees; avoids ghost columns for tools active only outside the truncated watch window | S:55 R:80 A:75 D:70 |
| 4 | Certain | `fmtCost` uses `toLocaleString("en-US", …)` with fixed 2 decimals | Mirrors existing `fmtNum` implementation exactly | S:80 R:90 A:95 D:90 |
| 5 | Confident | Bump `COST_WIDTH` and `MIN_TOOL_COL_WIDTH` 8 → 9 | Required so `$1,000.00+` cells don't overflow via padStart (documented wrap hazard in watch mode) | S:60 R:85 A:85 D:80 |
| 6 | Certain | Minor version bump ships with this change | Constitution "Output Stability" mandates it for breaking output changes | S:85 R:90 A:95 D:95 |

6 assumptions (2 certain, 4 confident, 0 tentative, 0 unresolved).
