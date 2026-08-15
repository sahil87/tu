# Intake: History Month Anchors + p95 Bar Scaling

**Change**: 260815-oojd-history-month-anchors-p95-bars
**Created**: 2026-08-15

## Origin

Conversational — a `/fab-discuss` session on improving tu's output DX. The agent reviewed live output of `tu h` and `tu cc h` and proposed five improvements; the user selected ideas 4 and 5 for this change:

> Idea 4: 90 daily rows with no visual anchors. "Last 3 months" oldest-first means the reader scans a wall of rows to find today. Cheap wins: a dim separator (or blank) line at month boundaries, and highlight/mark today's row. A step further: a summary footer beyond `Total` — avg/day, this-month-so-far, peak day.
>
> Idea 5: Bar scale collapses under outliers. A single $4,031 day makes every normal $150–400 day render as a sliver, so the bars carry almost no information for the whole window. Cap the scale at e.g. p95 and mark clipped bars.

User instruction: "Then the same for 4 and 5."

Design refinement (follow-up conversation): the user rejected a bare overflow arrow because it renders $1,091 and $4,031 identically, and asked for something that shows the difference — "change the scale after the arrow, or a line that separates the 'hundreds' from the 'thousands'". The agreed design is a **two-zone bar with a scale-break rule** (see What Changes §4): a dim `┊` rule at the cap column in every row, with clipped rows continuing past it into an overflow zone scaled cap→max. The multiplier-arrows alternative (each `▸` = one bar-length) was rejected — it reads as a count, not a length, and provides no vertical separator.

## Why

1. **Pain point**: The daily history views (`tu h`, `tu cc h` — both capped to the last 3 months) print ~90 undifferentiated rows. There is no visual grouping, so finding "start of this month" or "today" means reading dates row by row. Independently, the inline bar chart scales linearly to the max row cost; real data contains outlier days (e.g. $4,031.61 against a typical $150–400), which compresses every normal day's bar to a sliver — the chart carries almost no information exactly when the window contains its most interesting day.
2. **Consequence of not fixing**: The two main history affordances (the table and the bars) both degrade with real usage patterns; users fall back to reading raw numbers.
3. **Why this approach**: Month separators reuse the existing dim divider idiom (already used for header/total dividers). Percentile capping is the standard fix for outlier-dominated linear scales; an explicit overflow marker keeps clipped bars honest.

## What Changes

All changes are in `src/node/tui/formatter.ts` (`renderHistory` and `renderTotalHistory`) unless noted.

### 1. Month-boundary separators (daily period only)

While iterating entries/labels in `renderHistory` and `renderTotalHistory`, when the `YYYY-MM` prefix of the current label differs from the previous label's, emit a dim divider line (same construction as the existing header divider: `divStr + costDiv + [machineDiv] + barDiv`) before the row. Applies only when `period` is daily — monthly views have ≤12 rows and need no grouping. The compositor counts lines per frame, so extra lines are safe in watch mode; `maxRows` truncation happens before rendering, so separators reflect the visible window.

### 2. Today marker (daily period only)

The row whose label equals today's ISO date (`YYYY-MM-DD`, local time) renders its date cell in `boldWhite` (the existing emphasis color used for the Total row). No extra glyph/column — a color-only change keeps every row the same width and is stripped by `--no-color`/`NO_COLOR` automatically. In monthly views, the current month's row gets the same treatment.

### 3. Summary footer (history views)

After the Total row, add one dim footer line (same visual weight as the machine legend):

```
avg $XX.XX/day · this month $X,XXX.XX · peak $X,XXX.XX (2026-06-12)
```

- `avg` = window total / number of data rows (days with data, not calendar days — consistent with what the table shows)
- `this month` = sum of rows in the current calendar month (omit if the window contains no current-month rows)
- `peak` = max row cost with its date
- Monthly period: `avg $X/month`, drop `this month`, keep `peak` with `YYYY-MM` label.
- Rendered in ANSI renderers only (not compact mode, not CSV/Markdown emitters — CSV is a machine contract; Markdown tables are consumed standalone).

### 4. p95-capped bar scaling with a two-zone scale break

Bar scaling in `renderHistory` and `renderTotalHistory` currently uses `maxCost = max(costs)`. Change to a **two-zone piecewise scale** (a "broken axis"):

- Compute `p95` = the 95th percentile of the nonzero row costs (linear interpolation, sorted ascending).
- **Trigger**: the two-zone mode activates only when `maxCost > 1.5 × p95`; otherwise the bar renders exactly as today (single linear zone, `barScale = maxCost`, no rule, no behavior change).
- **When active**, the bar area (`barWidth` chars) splits into three parts:
  - **Main zone** — linear `0 → p95`, occupying the bar area minus the divider and overflow zone (overflow zone ≈ 1/4 of the bar area, minimum 4 chars; exact split is a plan-level decision).
  - **Scale-break rule** — a single dim `┊` (U+250A) at the cap column, rendered in **every** row, including rows whose bar doesn't reach it (trailing spaces pad short bars up to the rule). This draws a vertical ruler down the table separating the normal range from the outlier range.
  - **Overflow zone** — linear `p95 → maxCost`; only rows with `cost > p95` render into it, using the same fractional-eighths blocks. The overflow segment renders in `yellow` (the existing burn-rate color) to signal "compressed scale beyond this point"; the main zone stays `green`.
- **Legend**: when the two-zone mode is active, the §3 summary footer appends ` · ┊ = $XXX (p95)` so the break value is explicit.

Mock (window where p95 ≈ $846, outlier $4,031.61; overflow zone shown in yellow):

```
Date         |        ... |     Cost
2026-06-10   |        ... |  $846.21  ████████████████████████┊
2026-06-11   |        ... | $1,091.67 ████████████████████████┊▌
2026-06-12   |        ... | $4,031.61 ████████████████████████┊███████
2026-06-13   |        ... |  $645.11  ██████████████████▎     ┊
2026-06-14   |        ... |  $172.13  ████▉                   ┊
──────────────────────────────────────────────────────────────────────
avg $205.14/day · this month $1,204.50 · peak $4,031.61 (2026-06-12) · ┊ = $846 (p95)
```

The distance past the rule shows the magnitude difference among outliers ($1,091 barely crosses; $4,031 fills the overflow zone) — a bare overflow marker would render them identically. Compare today's rendering, where 2026-06-14 would be a 1.3-char sliver and 06-10 vs 06-13 are nearly indistinguishable.

Edge cases:
- Rows at exactly `p95` end at the rule with no overflow segment.
- The rule and trailing pad spaces are part of the bar area; total row width is unchanged from the single-zone case (`barWidth` is already terminal-width-derived).
- `--no-color`: the rule and both zones render uncolored; the `┊` glyph still carries the break semantics.
- Small-N guard: with fewer than ~10 nonzero rows, p95 ≈ max and the `1.5×` trigger rarely fires — acceptable; no special-casing beyond the trigger condition.

### 5. Version bump

The footer line and separators add lines to parseable output → minor version bump per the constitution's Output Stability rule (may share the bump with change q6fx if released together).

## Affected Memory

- `display/formatting`: (modify) month separators, today marker, summary footer, p95 bar-scale rule

## Impact

- `src/node/tui/formatter.ts` — `renderHistory`, `renderTotalHistory`, new percentile helper, footer builder
- Formatter tests — new cases: month-boundary separator placement, today-marker color, footer math (avg/this-month/peak), p95 trigger on/off, scale-break rule alignment across rows, overflow-zone proportions ($1,091 vs $4,031 render differently), footer p95 legend, monthly-period behavior
- `docs/specs/layouts.md` — §3/§4 mockups gain separators, footer, and clipped-bar examples
- Watch mode — line-count changes per frame; compositor re-measures each render, verify no assumptions on fixed row counts
- Version: minor bump required

## Open Questions

*(none)*

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | Separators at month boundaries only, daily period only | Discussed — user's idea 4 names month boundaries; weekly grouping was not requested | S:70 R:85 A:80 D:75 |
| 2 | Confident | Today marker is color-only (boldWhite date cell), no glyph | Keeps row width stable and degrades cleanly under --no-color | S:55 R:85 A:80 D:70 |
| 3 | Confident | Summary footer included in scope (avg/this-month/peak) | Discussed — user included the "step further" footer in idea 4 as presented; single dim line, low risk | S:65 R:80 A:75 D:70 |
| 4 | Tentative | p95 with 1.5× trigger and linear interpolation | User said "cap the scale at e.g. p95" — p95 chosen per suggestion; the 1.5× trigger avoids changing well-behaved windows but the exact factor is a judgment call | S:55 R:80 A:65 D:55 |
| 5 | Confident | Two-zone bar with `┊` scale-break rule; overflow zone linear cap→max | Discussed — user explicitly asked for a separator line showing the difference between 1k and 4k; multiplier-arrows alternative rejected in conversation | S:75 R:85 A:80 D:75 |
| 8 | Tentative | Overflow zone ≈ 1/4 of bar area (min 4 chars); overflow segment in yellow | Zone split and color are layout judgment calls; yellow reuses the existing "attention" color, but green-throughout is defensible | S:50 R:85 A:65 D:55 |
| 6 | Confident | Footer/separators in ANSI renderers only; CSV/Markdown emitters untouched | CSV is a machine contract; Markdown tables can't carry non-row lines cleanly | S:60 R:85 A:85 D:80 |
| 7 | Certain | Minor version bump ships with this change | Constitution "Output Stability" mandates it for output-shape changes | S:85 R:90 A:95 D:95 |

8 assumptions (1 certain, 5 confident, 2 tentative, 0 unresolved).
<!-- assumed: p95 cap with 1.5× trigger — exact percentile and trigger factor are tunable judgment calls; user suggested "e.g. p95" -->
<!-- assumed: overflow zone ≈ 1/4 of bar area, yellow overflow segment — zone split and color are tunable layout judgment calls -->
