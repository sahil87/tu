# Layouts

> Visual mockups of every distinct output layout produced by `tu`. Each section shows the
> command that triggers it, the ASCII mockup, and notes on column sizing and color.
>
> For data model, flag semantics, output-format rules, and watch mode architecture, see
> [usage.md](usage.md). Mockups in this file were re-verified against the installed
> v0.11.5 binary on 2026-09-16 (capture set: `fab/changes/260915-2y3l-spec-reconciliation/reconciliation.md`).
> A `(DC-NN)` reference points at an entry in usage.md's **Drop at cutover** ledger — the behavior
> is rendered as it exists today and is flagged there for a keep/drop decision.

## 1. Snapshot — All Tools

**Command:** `tu`, `tu d`, `tu m`, `tu all`

```
📊 Combined Usage (daily)

Tool         |       Tokens |        Input |       Output |        Cache |         Cost
─────────────|──────────────|──────────────|──────────────|──────────────|─────────────
Claude Code  |  487,683,047 |        3,734 |    1,121,329 |  486,557,984 |      $465.67
Codex        |    2,345,678 |      987,654 |    1,358,024 |            0 |       $23.45
OpenCode     |      456,789 |      234,567 |      222,222 |            0 |        $4.56
─────────────|──────────────|──────────────|──────────────|──────────────|─────────────
Total        |  490,485,514 |    1,225,955 |    2,701,575 |  486,557,984 |      $493.68
```

- **Columns:** Tool (12 left-aligned), Tokens/Input/Output/Cache/Cost (12 right-aligned each) — full row is 87 visible chars (≤ 90 budget)
- **Cache** is cache write + cache read combined, so the row arithmetic closes: Input + Output + Cache = Tokens
- **Separator:** ` | ` between columns
- **Colors:** header row `boldCyan`, dividers `dim`, Total row `boldWhite`
- Cost cells carry `en-US` thousands separators (`$1,012.34`), matching the token columns
- Tools with zero tokens are omitted; Total row (and its divider) shown only when >1 tool has data
- The output starts and ends with one blank line (the blank line before the heading is part of every one-shot table render)
- A cell wider than its 12-char column pushes that row wider (e.g. `16,809,796,832` in a monthly all-users view); columns are fixed, not data-sized, so the header and divider do not grow with it (DC-24)
- **Empty state:** when no tool has data the table is replaced by two-space-indented `  No usage` under the heading:

  ```
  📊 Combined Usage (daily)

    No usage
  ```

- **Token mode (`--metric tokens` / `-t`):** the table is unchanged — its columns are already token-denominated and the Cost column is kept as context. Only the watch delta indicator follows the metric: under tokens it rides the Tokens cell (`43,376,142 ↑`) and the Cost cell stays plain; under cost it rides Cost

## 2. Snapshot — Single Tool

**Command:** `tu cc`, `tu codex m`, `tu oc`

Same table as Layout 1 with one data row. The heading does **not** change to the tool name — it stays `Combined Usage` for a single-source snapshot (DC-15):

```
📊 Combined Usage (daily)

Tool         |       Tokens |        Input |       Output |        Cache |         Cost
─────────────|──────────────|──────────────|──────────────|──────────────|─────────────
Claude Code  |   24,049,084 |          138 |       25,188 |   24,023,758 |        $5.38
```

No divider/Total row when only one row is present. A single source with no usage renders the `  No usage` empty state exactly as Layout 1.

## 3. History — Single Tool

**Command:** `tu cc h`, `tu cc dh`, `tu codex mh`

```
📊 Claude Code (daily, last 3 months)

Date         |          Input |         Output |    Cache Write |     Cache Read |          Total |       Cost
─────────────|────────────────|────────────────|────────────────|────────────────|────────────────|───────────
2026-07-01   |        509,158 |        782,954 |      7,587,775 |    252,480,412 |    261,360,299 |    $201.46
2026-07-02   |      1,184,699 |      2,337,998 |     22,398,491 |    541,984,252 |    567,905,440 |    $593.80
2026-07-03   |      2,940,669 |      6,937,649 |     66,929,611 |  1,041,656,575 |  1,118,464,504 |  $1,806.24
─────────────|────────────────|────────────────|────────────────|────────────────|────────────────|───────────
Total        |     15,453,087 |    171,182,847 |  1,731,242,450 | 59,108,327,119 | 61,026,205,503 | $59,634.40
avg $784.66/day · this month $8,895.78 · peak $3,069.40 (2026-07-18)
```

The row above is 110 visible chars, so bars appear only on terminals of at least 121 columns (the 10-char bar-area rule below). On a wide terminal with an outlier window — max $4,031.61 > 1.5 × p95 $846.21 — the bars switch to the two-zone scale (green main zone 0→p95, dim `┊` scale-break rule in every row, yellow overflow zone p95→max):

```
Date         |          Input |         Output |    Cache Write |     Cache Read |          Total |      Cost
─────────────|────────────────|────────────────|────────────────|────────────────|────────────────|─────────────────────
2026-06-10   |        234,567 |        345,678 |         10,000 |          5,000 |        595,245 |  $846.21 █████████████████████┊
2026-06-11   |        345,678 |        456,789 |         15,000 |          7,500 |        824,967 | $1,091.67 █████████████████████┊▌
2026-06-12   |        567,890 |        678,901 |         45,678 |         23,456 |      1,315,925 | $4,031.61 █████████████████████┊████████
2026-06-13   |        123,456 |        234,567 |          8,000 |          4,000 |        370,023 |  $172.13 ████▎                 ┊
─────────────|────────────────|────────────────|────────────────|────────────────|────────────────|─────────────────────
Total        |      1,271,591 |      1,715,935 |         78,678 |         39,956 |      3,106,160 | $6,141.62
avg $1,535.41/day · this month $6,141.62 · peak $4,031.61 (2026-06-12) · ┊ = $846.21 (p95)
```

- **Columns:** Date (12 left-aligned), Input/Output/Cache Write/Cache Read/Total (14 right-aligned), Cost (right-aligned, sized to the longest value in the column including the Total row, floor 9 — `$59,634.40` in the Total row widens it to 10 above)
- **Heading:** `📊 {Tool Name} ({period})`; the period parenthetical carries `, last 3 months` while the implicit daily/weekly cap is active (see usage.md › Snapshot vs History)
- **Bar chart:** green Unicode block elements (full + fractional eighths), max width 30, scaled to max cost — or to max total tokens under `--metric tokens`/`-t` (bars, stacked segments, and the footer `avg`/`this month`/`peak`/`p95` follow the metric and format as plain counts)
- **Token mode (`--metric tokens` / `-t`):** the table shape is unchanged; the last column's unit swaps to `Tokens` = total tokens (duplicating the Total column's numbers), the delta indicator compares token values, and the same data-sizing/dim-zero rules apply in the displayed unit
- **p95 two-zone scale:** when `max > 1.5 × p95` (95th percentile of the nonzero row costs, linear interpolation), the bar area splits into a green main zone (linear 0→p95), a dim `┊` (U+250A) scale-break rule rendered in every row (short bars space-pad up to the rule so it aligns vertically), and a yellow overflow zone (linear p95→max, `max(4, round(barWidth/4))` chars). Rows at exactly p95 end at the rule with no overflow segment. Below the trigger the single linear scale renders unchanged — no rule, no legend, no width change
- **Month separators:** daily views emit a dim divider (same construction as the header divider) before each row whose `YYYY-MM` prefix differs from the previous row's — daily period only, computed on the visible window, never before the first visible row
- **Current-period marker:** the row matching the current period's label (today / this week / this month) renders its date cell in **boldWhite** — color-only, no glyph, width unchanged, stripped by `--no-color`/`NO_COLOR`
- **Weekend dimming:** daily views render Saturday/Sunday date cells in **dim** (date cell only — cost/token cells and the bar stay full-intensity). The today marker wins on a weekend today (one cell, one style). Weekday derived from the UTC-parsed ISO label — timezone-independent. Daily period only; compact mode, CSV, and Markdown are untouched; `--no-color`/`NO_COLOR` output is byte-identical to an undimmed render
- **Summary footer:** one dim line after the Total row (≥2 data rows): `avg $X.XX/day · this month $X,XXX.XX · peak $X,XXX.XX (YYYY-MM-DD)` — avg is window total ÷ data-row count with a per-period unit suffix (`/day`, `/week`, `/month`); `this month` is daily-only and omitted when the window has no current-month rows; two-zone windows append `· ┊ = $X (p95)`. ANSI renderers only — CSV and Markdown output carry no separators, marker, or footer. The footer is one line regardless of terminal width, so it wraps on narrow terminals (DC-12)
- Bars only render when the terminal leaves at least 10 chars after the Cost column and the 3-char gutter
- Total row shown when >1 entry; a single-row window (e.g. `tu cc mh` in a one-month sandbox) renders the row with no divider, no Total, no footer
- **Empty state:** `  No data` (two-space indent) under the heading

## 4. History — All Tools (Pivot)

**Command:** `tu h`, `tu dh`, `tu mh`

Captured at 80 columns (bar budget 12 main + rule + 4 overflow); rows elided, and the `2026-08-01` row is included to show a month separator:

```
📊 Combined Cost History (daily, last 3 months)

Date       | Claude Code |     Codex |      Kimi |       Cost
───────────|─────────────|───────────|───────────|──────────────────────────────
2026-07-01 |     $201.46 |     $0.00 |     $0.00 |    $201.46 █▍          ┊
2026-07-02 |     $593.80 |     $0.00 |     $0.00 |    $593.80 ████        ┊
2026-07-03 |   $1,806.24 |     $0.00 |     $0.00 |  $1,806.24 ████████████┊▎
───────────|─────────────|───────────|───────────|──────────────────────────────
2026-08-01 |     $972.43 |     $0.00 |     $0.00 |    $972.43 ██████▋     ┊
───────────|─────────────|───────────|───────────|──────────────────────────────
Total      |  $59,634.40 | $1,667.70 | $5,934.71 | $67,236.85
avg $862.01/day · this month $12,945.64 · peak $3,069.40 (2026-07-18) · ┊ = $1,755.51 (p95) · █ Claude Code █ Codex █ Kimi
```

Each bar is stacked per tool (colors not shown in ASCII): the Claude Code share of each row renders green, the Codex share magenta, Kimi blue — see the stacking bullet below. The `$0.00` cells render dim.

- **Columns:** Date (10 left-aligned — ISO daily labels are 10 chars, monthly 7), one per tool sized to `max(toolName.length, 9, longest cost cell in the column including its Total-row sum)` right-aligned (variable per-tool width — `Claude Code` → 11, short names with small costs floor to 9), row Cost (right-aligned, sized to the longest value including the Total row, floor 9 — 10 above)
- **Heading:** `📊 Combined Cost History ({period}[, last 3 months])`
- **Negligible-column omission:** tool columns whose visible-window total is under $1.00 or under 0.1% of the window grand total are omitted entirely — above, Gemini/Copilot/OpenCode have no cost in the window, so only Claude Code, Codex and Kimi render. If every tool is negligible the renderer falls back to the exact-zero filter, then to the full list. The Markdown pivot keeps the exact-zero rule (only all-`$0.00` columns drop); the CSV emitter is the exception: it keeps every registry column with raw `0.00` cells (positional machine contract) (DC-06). Omitted tools still count in the row Cost, the Total row, the bars, and the footer
- **Token mode (`--metric tokens` / `-t`):** every per-tool cell, the row column and the Total row render as total tokens (no `$`), the last header reads `Tokens`, and the title is `📊 Combined Token History (…)`. The omission rule keys on the displayed unit (under 1,000 tokens or under 0.1% of the window grand total in tokens — a `$0.00`-cost tool with real tokens is kept in token mode); bars, segments, legend, data-sizing and dim-zero rules are unchanged. See Layout 18
- Variable-width columns keep the **full 6-tool data row — Date + tool columns + the 3-char gutter + the Cost cell — at a 96-char minimum**: `10 + (11+9+9+9+9+9) + 6×3 + 3 + 9 = 96` whenever every cell fits `$9,999.99`, so the all-tools-active pivot needs a ≥96-col terminal; larger cells widen their own column from there. With negligible-column omission the typically rendered width is far below 80, restoring the inline bar chart on standard terminals
- **Bar geometry by width** (three visible tools, 10-wide Cost, as captured): bar area = terminal width − table width − 3 − 1; suppressed under 10; at 80 cols it is 17 (12 + `┊` + 4 in two-zone mode); at 100 cols and above it is the 30-char cap (21 + `┊` + 8). Piped output uses 80 (see Layout 21)
- **Dim zero cells:** a data cell whose cost is exactly `$0.00` renders dim (cell text padded first, then colored — width unchanged, stripped under `--no-color`/`NO_COLOR`); the Total row, headers, and dividers are never dimmed, and a sub-cent value that rounds to `$0.00` stays full-intensity. The same rule dims Layout 3's Cost and machine cells and the snapshot's machine columns
- **Watch mode** appends a delta indicator (`↑`/`↓`) after the Cost cell when a previous poll exists. In this pivot only, the indicator is rendered **without its leading space** (`$1,013.30↑` — 1 visible char), so the minimum watch-mode row is 96 + 1 = **97 chars** and fits a ≥97-col terminal without wrapping (the spaced ` ↑` form would add one more char and wrap, corrupting the watch compositor's line-counting). Other renderers keep the spaced form. A tool crossing the omission threshold mid-watch gains a column on the next render — the compositor re-measures every frame
- **Stacked bars:** bars scale to row total cost with the same length and two-zone geometry as Layout 3, but the main-zone fill (or the whole bar in a single-zone window) is split into contiguous per-tool segments, left to right in column order, each segment's length proportional to that tool's share of the row cost. Segments are apportioned by largest-remainder rounding over the bar's character count (ties break to the earlier column), so they always sum exactly to the unstacked bar's length; the fractional-eighths character rides the last (rightmost) segment; a tool whose share rounds to zero characters gets no segment. Segment colors come from the fixed palette **green, magenta, blue, cyan**, assigned in visible column order — a 5th+ visible tool renders uncolored. The overflow zone past the `┊` rule stays solid yellow with no segmentation. Under `--no-color`/`NO_COLOR` the segments collapse to solid blocks indistinguishable from a single-color bar
- **Legend:** when stacked bars render (bars visible ∧ ≥2 visible tools ∧ color enabled), the summary footer appends one colored `█` swatch per visible tool in column order, each followed by the dim tool name — `· █ Claude Code █ Codex █ Kimi`. Omitted under `--no-color`/`NO_COLOR`, when bars are suppressed (narrow terminal), and with a single visible tool
- **Month separators, current-period marker, weekend dimming, summary footer, and p95 two-zone scale:** same rules as Layout 3 — separators daily-only (the `2026-08-01` row above is preceded by one), the current-period row's date cell renders boldWhite, Saturday/Sunday date cells render dim, the dim footer follows the Total row (≥2 labels), and the `┊` rule with yellow overflow engages when `max > 1.5 × p95` of the nonzero row totals
- **Monthly and weekly** headings drop the cap hint (`📊 Combined Cost History (monthly)`); weekly rows are labeled by the week's Sunday (`2026-06-28`, `2026-07-05`, …); the footer unit follows (`avg $5,603.07/week`, `avg $9,337.68/month`) and `this month` is omitted
- **Empty state:** `  No data` under the heading

## 5. Leaderboard — Snapshot (`lb`)

**Command:** `tu lb`, `tu m lb`, `tu cc m lb`, `tu w lb --top 3`

Captured at 80 columns:

```
Leaderboard (monthly) · 2026-09 · by cost

# | User    |       Cost                     |         Tokens | Share | Δ vs Aug
──|─────────|────────────────────────────────|────────────────|───────|─────────
1 | sahil ◂ | $12,945.64 ███████████████████ | 15,962,442,751 | 69.0% |     -55%
2 | beatriz |  $3,333.40 ████▉               |  5,562,910,079 | 17.8% |     -51%
3 | carlos  |  $1,598.54 ██▍                 |  1,681,643,674 |  8.5% |     -91%
4 | dominic |    $611.44 ▉                   |    390,936,777 |  3.3% |     -85%
5 | eunice  |    $155.60 ▎                   |     32,774,697 |  0.8% |     -96%
6 | frankie |    $116.47 ▏                   |    100,240,636 |  0.6% |     -98%
──|─────────|────────────────────────────────|────────────────|───────|─────────
  | Total   | $18,761.10                     | 23,730,948,614 |       |
synced 15m ago (2026-09-15T18:54:44.502Z) · tu sync to refresh
```

- **Heading:** `Leaderboard ({period}) · {window} · by {cost|tokens}` — **no `📊` prefix**, unlike every other table heading (DC-07). `{window}` is the period's current label (`2026-09-16`, the week's Sunday `2026-09-13`, or `2026-09`), or `{since} → {until}` under an explicit window (`2026-09-01 → 2026-09-10`; an `--until`-only window renders `→ 2026-09-10` with an empty left side) (DC-13)
- **Columns:** `#` rank (right-aligned, 1 char wide up to 9 rows, 2 from 10 rows), User (left, data-sized; the pinned user — `-u <name>`, else the config user — carries a ` ◂` marker that counts toward the column width), Cost (right, data-sized with the 9-char floor, thousands separators), inline bar (solid green, scaled to the max row in the display metric, 19 chars at 80 cols, 30-char cap from ~91 cols), Tokens (right, data-sized), Share (percent of the grand total in the display metric, one decimal; `100.0%` widens the column), `Δ vs {prev label}` (whole-percent change vs the previous same-length window — `Δ vs 2026-09-15` for daily, `Δ vs 2026-09-06` for weekly, short-month `Δ vs Aug` for monthly, `Δ vs prev` under an explicit window; `new` when the user had no prior-window data; large changes render as-is, e.g. `+4757%`). Both Cost and Tokens render under every metric — `--metric tokens`/`-t` changes only the sort key, bar scale, share denominator, and the heading's `by …` suffix
- **Ranking:** descending by raw total in the display metric (cost default); ties break by user name ascending; users with zero cost and zero tokens in the window are omitted
- **Total row:** bolded (`boldWhite`), only when ≥2 rows; sums every row including any collapsed by `--top`; the `#` and Share/Δ cells are blank
- **`--top <n>`:** rows past N collapse into one dim `… +k others` line (omitted when `k = 0`); collapsed users still count toward the Total and every share denominator. See Layout 19
- **Staleness footer:** one dim line — `synced {relative} ago ({ISO timestamp}) · tu sync to refresh`, or `never synced · tu sync to refresh` — because the leaderboard reads the synced metrics repo only. ANSI table output only; CSV/JSON/MD carry no footer
- **`--by-machine`:** rows become `user/machine` pairs (`sahil/dev-ws-sahil02 ◂`); every row of the pinned user carries the marker; the bar is usually suppressed because the wider User column consumes the budget. See Layout 17
- **Source filter:** `tu cc m lb` ranks the same users on Claude Code spend only; a user with no spend in that source is omitted
- **Dim zero cells** and `--no-color` byte-equality follow the same rules as Layout 4. An empty window renders the heading, `No data`, and the footer — never a crash
- **Multi mode only:** single mode exits 1 with `Error: lb requires multi mode — run tu init-metrics <repo-url> to set up a metrics repo` (the message names `lb` even for `lbh`) (DC-14)

## 6. Leaderboard — History Pivot (`lbh`)

**Command:** `tu lbh`, `tu m lbh`, `tu w lbh --top 2`

```
📊 Leaderboard History (monthly)

Date       |     sahil |      alice |       bob |      Cost
───────────|───────────|────────────|───────────|──────────────────────
2026-07    |   $355.20 |    $314.60 |   $169.00 |   $838.80 ██████████████▎
2026-08    |   $412.30 |    $301.10 |   $220.05 |   $933.45 ██████████████████████████████
───────────|───────────|────────────|───────────|──────────────────────
Total      |   $767.50 |    $615.70 |   $389.05 | $1,772.25
avg $886.13/month · peak $933.45 (2026-08) · █ sahil █ alice █ bob
```

- **Same renderer as Layout 4** with users in place of tools — month separators, current-period row marker, weekend dimming, stacked per-column bars + legend, p95 two-zone scale, dim summary footer, dim exact-zero cells, and data-sized columns are all inherited unchanged
- **Title:** `📊 Leaderboard History ({period}[, last 3 months])` (`Leaderboard Token History` under `--metric tokens`/`-t`) in place of `Combined Cost History`
- **Column order:** descending by window total in the display metric (a leaderboard is ranked), not registry order — ties keep first-seen order. The CSV and Markdown emitters order the same columns **alphabetically** instead (DC-16)
- **Per-row leader:** each row's winning user cell renders `boldWhite` (color-only, width unchanged, stripped by `--no-color`/`NO_COLOR`)
- **No negligible-column omission:** every user column renders — a low-spend user is never silently hidden from a ranking (unlike the tool pivot's omission rule); `--top <n>` is the explicit control, keeping the N highest-total user columns and folding the rest into a single `others` column so row totals are preserved (no `others` column when nothing was folded). The `others` column is sorted by its own total like any user column, so it can land first or in the middle (DC-08); see Layout 19
- **Width:** with 15 users the row is ~215 chars and wraps on any ordinary terminal (in watch mode this corrupts the frame) (DC-17)
- **Cap:** daily/weekly `lbh` carries the same implicit 3-month cap / `--full` semantics as `h` (heading hint `last 3 months`); monthly is never capped
- **`--by-machine` warns and is ignored** (`Warning: --by-machine is not supported with leaderboard history — ignoring.`), exactly as on the all-tools pivot; multi mode only (same exit-1 guard as `lb`)

## 7. Watch Mode — Full Screen

**Command:** `tu -w`, `tu cc h -w`, `tu mh -w`

Enters the alternate screen buffer with the cursor hidden. Layout adapts to terminal dimensions.

### Full mode (>= 60 cols): stats grid + table + rain

Captured at 100×30 on the second poll:

```
 Elapsed  11s          Tok/min   ~769,319
 Session  +$0.07       Rate      ~$23.14/hr
                       Proj. day ~$571.38
───────────────────────────────────────────

📊 Combined Usage (daily)

Tool         |       Tokens |        Input |       Output |        Cache |         Cost
─────────────|──────────────|──────────────|──────────────|──────────────|─────────────
Claude Code  |   21,871,261 |          130 |       24,644 |   21,846,487 |        $4.94
Kimi         |   43,075,210 |      964,636 |      161,707 |   41,948,867 |     $26.86 ↑
─────────────|──────────────|──────────────|──────────────|──────────────|─────────────
Total        |   64,946,471 |      964,766 |      186,351 |   63,795,354 |       $31.79

ﾏ    ﾈ                                            K   N       ﾝ                           ﾘ
                        ｿ                         ｲ   F                                  I
              ﾄ         I                         6   ﾐ                                  g
Next refresh: 4s · ↵ refresh · q quit
```

- **Stats grid:** 2x3 grid above the table — session stats left (Elapsed, Session), cost stats right (Tok/min, Rate, Proj. day); rate values carry a `~` prefix, the session delta a sign
- **Separator:** dim horizontal rule between stats grid and table title, as wide as the widest grid line (35 chars while the values are `--`, 43 above)
- **Table:** any of Layouts 1–6, depending on command args — same render functions as non-watch mode. History tables are **truncated to the rows that fit the terminal height** (a 30-row terminal shows the last ~15 daily rows; separators, p95 scale and footer are computed on that visible window)
- **Width:** a table wider than the terminal wraps inside the frame — the single-tool history (110 chars) at 100 cols, or `lbh` with many users — and the compositor's line accounting breaks (DC-17)
- **Rain:** matrix rain fills vertical space below content (or the right margin if no vertical space)
- **Footer:** status line at the terminal's bottom row, all `dim`
- Unavailable stats show `--` placeholder; grid stays fixed at 3 rows

### Compact mode (< 60 cols)

Captured at 50×20 (`tu -w`) and 59×20 (`tu h -w`):

```
📊 Combined Usage (daily)
Claude Code           $5.38
Kimi                 $27.25
───────────────────────────
Total                $32.64

Next refresh: 4s · ↵ refresh · q quit
```

```
📊 Combined Cost History (daily, last 3 months)
2026-09-15          $220.71
2026-09-16         $32.66 ↑
───────────────────────────
Total            $11,741.81

Next refresh: 4s · ↵ refresh · q quit
```

- Two columns only: name/date (14 left-padded) + cost (12 right-padded); the heading line is kept
- No token breakdown, no stats grid, no rain, no bars, no month separators, no footer stats
- **Compact mode exists only in watch mode.** One-shot output (`tu` without `-w`) always renders the full-width table and wraps on a narrow terminal (DC-12)
- **Token mode (`--metric tokens` / `-t`):** compact cells show total tokens instead of cost; the 12-char width is unchanged

### Exit

`q` and Ctrl-C both restore the normal screen and print the last rendered table (heading through Total row, no stats grid, no footer) to stdout, exit 0.

## 8. Watch Mode — Stats Grid Detail

### Full stats (2+ polls)

```
 Elapsed  5m 32s     Tok/min   ~12,345
 Session  +$0.50     Rate      ~$1.25/hr
                     Proj. day ~$15.00
```

- **Elapsed:** `Xh Xm Xs` / `Xm Xs` / `Xs`
- **Session:** cost delta since watch start (shown as `$0.00` before 2 polls, then signed `+$0.07`)
- **Tokens/min:** `--` until 2+ polls with totalTokens > 0, then `~N`
- **Rate:** 5-poll rolling window burn rate, `~$X.XX/hr`, shown in `yellow`; `--` until 2+ polls
- **Proj. day:** today's cost + rate × remaining hours, `~$X.XX`; `--` until 2+ polls
- Labels `dim`, values `boldWhite`

### Loading skeleton (before first fetch)

```
 Elapsed  0s           Tok/min   --
 Session  $0.00        Rate      --
                       Proj. day --
───────────────────────────────────

📊 Combined Usage (daily)

Tool         |       Tokens |        Input |       Output |        Cache |         Cost
─────────────|──────────────|──────────────|──────────────|──────────────|─────────────
                                      Loading...
```

Rain animates from the first tick, before the first fetch completes.

## 9. Watch Mode — Delta Indicators

In watch mode, cost cells gain directional arrows after each poll:

```
$12.34 ↑     green up-arrow: cost increased since last poll
$23.45 ↓     red down-arrow: cost decreased since last poll
$4.56        no indicator: first poll or no change
```

Tracked per item via `{toolName}:{label}` (history rows), `total:{label}` (pivot row totals) or `{toolName}` (snapshot rows). The snapshot and single-tool history use the spaced form (`$26.86 ↑`); the pivot uses the space-less form (`$1,013.30↑`); under `-t` the arrow rides the Tokens cell (`43,376,142 ↑`). Leaderboards key on the user name (`user/machine` under `--by-machine`).

## 10. Watch Mode — Matrix Rain

Fills available terminal space with falling characters:

```
        ﾗ0ﾑa                    7ﾘ
        ﾗ                        ﾘk        ← bright green (head)
        ﾗ                          Z       ← green (body)
         5                         q       ← dim green (tail)
                                   ﾝ
```

- **Characters:** half-width katakana + digits + latin
- **Density:** ~30% of available columns active at 20 rows or fewer; taller zones scale the drop count linearly up to 3× at 60+ rows
- **Speed:** 0.3-1.0 rows per 107ms tick (fractional)
- **Trail:** 3-8 chars with brightness gradient (brightGreen head, green body, dimGreen tail)
- **Shimmer:** ~5% of trail chars randomly replaced each tick
- **Positioning:** below content (preferred) or right margin (fallback, needs ≥ 10 cols after a 2-col gutter); disabled with `--no-rain`

## 11. Watch Mode — Footer States

```
Next refresh: 45s · ↵ refresh · q quit     ← countdown (dim)
Refreshing... · ↵ refresh · q quit         ← fetching (dim)
```

Truncates progressively in narrow terminals: controls dropped first, then status text.

## 12. JSON Output

**Command:** `tu --json`, `tu -j`, `tu cc h --json`, `tu m lb --json`, `tu lbh --json`

Pretty-printed with two-space indentation, trailing newline. The exact key rules per display are in [usage.md › Output Formats › JSON Output](usage.md#json-output--json). One example per shape:

**Snapshot** (`tu --json`, `tu cc --json`) — an object keyed by tool display name in registry order; a tool with data carries `label` first, a zero-usage tool carries no `label` (DC-01); every registry tool is present for an all-tools command, and only the selected tool for a single-source command (`tu cc --json`):

```json
{
  "Claude Code": {
    "label": "2026-09-16",
    "totalCost": 4.936068800000001,
    "inputTokens": 130,
    "outputTokens": 24644,
    "cacheCreationTokens": 23644,
    "cacheReadTokens": 21822843,
    "totalTokens": 21871261
  },
  "Codex": {
    "totalCost": 0,
    "inputTokens": 0,
    "outputTokens": 0,
    "cacheCreationTokens": 0,
    "cacheReadTokens": 0,
    "totalTokens": 0
  }
}
```

**Snapshot with `--by-machine`** — tools with data gain a trailing `machines` object (machine name → cost; user name → cost under `-u all`); zero-usage tools gain nothing:

```json
{
  "Claude Code": {
    "label": "2026-09-16",
    "totalCost": 1005.7143808,
    "inputTokens": 149,
    "outputTokens": 25743,
    "cacheCreationTokens": 26687,
    "cacheReadTokens": 25648577,
    "totalTokens": 25701156,
    "machines": {
      "sandbox-mach": 5.7243808000000005,
      "othermach": 999.99
    }
  }
}
```

**Single-tool history** (`tu cc h --json`) — a bare array of entries ascending by label; with `--by-machine` each entry gains the same `machines` object:

```json
[
  {
    "label": "2026-09-08",
    "totalCost": 211.79787615000015,
    "inputTokens": 5609,
    "outputTokens": 420529,
    "cacheCreationTokens": 5506878,
    "cacheReadTokens": 152651395,
    "totalTokens": 158584411
  }
]
```

**All-tools history** (`tu h --json`, `tu mh --json`) — an object keyed by tool display name in registry order, each an array of entries (empty array for a tool with no data; every registry tool present):

```json
{
  "Claude Code": [ { "label": "2026-07-01", "totalCost": 201.46, "inputTokens": 509158, "outputTokens": 782954, "cacheCreationTokens": 7587775, "cacheReadTokens": 252480412, "totalTokens": 261360299 } ],
  "Codex": [],
  "OpenCode": [],
  "Gemini": [],
  "Copilot": [],
  "Kimi": []
}
```

**Leaderboard** (`tu m lb --json`) — an array of rank rows; `machine` appears after `user` under `--by-machine`; `delta` is `null` for a `new` row:

```json
[
  {
    "rank": 1,
    "user": "sahil",
    "cost": 12945.642098040002,
    "totalTokens": 15962442751,
    "share": 0.6900258986630633,
    "delta": -0.5532817482507824
  }
]
```

**Leaderboard history** (`tu m lbh --json`) — the all-tools history shape with user names (alphabetical) as keys.

Numbers are raw IEEE doubles as summed (`4.936068800000001`), never rounded (DC-10). Incompatible with `--watch`, `--csv`, `--md` (exit 2).

## 13. Status

**Command:** `tu status`

### Single mode

```
Mode:        single
Config:      ~/.config/tu/tu.conf (v2)
```

Or when no config file exists:

```
Mode:        single (no ~/.config/tu/tu.conf)
```

The `(vN)` suffix echoes the file's `version` field even when it is newer than supported (`(v9)`, with the newer-version warning on stderr).

When a legacy `~/.tu.conf` is the file actually read (fallback), the `Config:` line shows `~/.tu.conf` instead, and the one-line deprecation warning goes to stderr:

```
Mode:        single
Config:      ~/.tu.conf (v2)
```

When an org config exists, an `Org config:` line prints directly after the `Config:` line (in both single and multi layouts):

```
Mode:        single
Config:      ~/.config/tu/tu.conf (v2)
Org config:  ~/.config/tu/org.conf
```

For an org-only setup (no personal or legacy file) the `Config:` line is omitted:

```
Mode:        multi
User:        orguser
Machine:     dev-ws-sahil02
Org config:  ~/.config/tu/org.conf
Metrics:     ~/.tu/metrics_repo (NOT FOUND — run 'tu init-metrics')
Last sync:   never
Auto-sync:   on
```

### Multi mode

```
Mode:        multi
User:        sahil
Machine:     dev-ws-sahil02
Config:      ~/.config/tu/tu.conf (v2)
Metrics:     ~/.tu/metrics_repo
Last sync:   15m ago (2026-09-15T18:54:44.502Z)
Auto-sync:   on
```

- `Last sync:` is `never` when `~/.tu/.last-sync` is absent, else `{relative} ago ({ISO timestamp})`
- `Auto-sync:` is `on` unless `auto_sync` is `false` or `0`, then `off`
- When the metrics dir is missing: `Metrics:     ~/.tu/metrics_repo (NOT FOUND — run 'tu init-metrics')`
- `status` never triggers a clone or sync; the config path shown is the one the selection rule read

## 14. Help

**Command:** `tu help`, `tu -h`, `tu --help`, `tu update --help`, `tu update -h`

The `--help` text is a fixed external surface. This block is a verbatim copy of the v0.11.5 output (also embedded byte-for-byte as `root.text` in `tu help-dump`, with one trailing newline):

```
Usage: tu [source] [period] [display]

Sources: cc (Claude Code), codex/co (Codex), oc (OpenCode), gemini/gem (Gemini), copilot/cop (Copilot), kimi/ki (Kimi), all (default)
Periods: d/daily (default), w/weekly, m/monthly
Display: (bare) = snapshot, h/history = history, lb = leaderboard, lbh = leaderboard history
Combined: dh (daily history), wh (weekly history), mh (monthly history)

Examples:
  tu                   Today's cost, all tools (snapshot)
  tu cc                Today's cost, Claude Code
  tu h                 Daily cost history, all tools (pivot)
  tu cc mh             Monthly cost history, Claude Code
  tu wh                Weekly cost history, all tools
  tu m                 This month's cost, all tools
  tu m lb              This month's leaderboard — users ranked by cost (multi mode)
  tu lbh               Daily leaderboard history — rows x user columns (multi mode)

Setup:
  tu init-conf         Scaffold ~/.config/tu/tu.conf
  tu init-metrics [url] Clone metrics repo (url also sets metrics_repo)
  tu sync              Push/pull metrics manually
  tu status            Show config and sync state
  tu update            Update tu to latest version
  tu shell-init <sh>   Emit shell init script (bash/zsh/fish)
  tu skill             Print agent usage bundle (markdown)

Help: tu help | tu -h | tu --help

Flags:
  --json / -j          Output data as JSON (data commands only)
  --csv                Output data as CSV (data commands only)
  --md                 Output data as Markdown (data commands only)
  --since / -s <date>  Only include entries on/after date (YYYY-MM-DD or YYYYMMDD, history display)
  --until <date>       Only include entries on/before date (YYYY-MM-DD or YYYYMMDD, history display)
  --full               Show full history (default: last 3 months for daily/weekly history)
  --metric <m>         Show 'cost' (default) or 'tokens' in table cells, bars and footer stats (snapshot keeps its Cost column in dollars)
  -t                   Shorthand for --metric tokens
  --top <n>            Show only the top N rows/columns on the lb/lbh leaderboard
  --sync               Sync metrics before fetching (multi mode)
  --dry-run            Preview sync without writing (tu sync only)
  --fresh / -f         Bypass cache, fetch fresh data (data commands only)
  --watch / -w         Persistent polling mode with live display (data commands only)
  --interval / -i <s>  Poll interval in seconds (default: 10, range: 5-3600)
  --user / -u <user>   Show usage for a specific user, or 'all' for every user
                       in the metrics repo (multi mode only; repo data — sync for today)
  --by-machine         Show per-machine cost breakdown (data commands only)
  --skip-brew-update   Skip 'brew update' tap refresh during 'tu update'
  --no-color           Disable ANSI color output
  --no-rain            Disable matrix rain animation in watch mode
```

- `--version` / `-V` / `-v` are not listed in the help text (DC-09)
- `--help` in any position other than first (`tu cc --help`, `tu h -h`) is an unknown argument (exit 2) except for `tu update --help` / `tu update -h` (DC-04). The short usage printed on an unknown argument is:

```
Unknown argument: bogus
Usage: tu [source] [period] [display]

  tu                Today's cost, all tools
  tu cc             Today's cost, Claude Code
  tu mh             Monthly cost history, all tools
  tu -h             Show full help

Run 'tu help' for all commands.
```

## 15. CSV Output

**Command:** `tu --csv`, `tu cc h --csv`, `tu h --csv`, `tu m lb --csv`, `tu m lbh --csv`

RFC 4180, comma-separated, LF line endings, no BOM, header row first, raw numbers (no thousands separators, no `$`), costs with two decimals. A `Total` row is appended only for the snapshot (more than one tool with data) and the leaderboard (more than one ranked user in the full set — also under `--top`); the two history kinds never carry one. Rules per kind are in [usage.md › Output Formats › CSV Output](usage.md#csv-output--csv).

**Snapshot** (zero-usage tools omitted, like the table) (DC-05):

```
tool,tokens,input,output,cache,cost
Claude Code,21871261,130,24644,21846487,4.94
Kimi,42418959,959883,159435,41299641,26.49
Total,64290220,960013,184079,63146128,31.43
```

**Single-tool history** (no Total row):

```
date,input,output,cache_write,cache_read,total,cost
2026-09-08,5609,420529,5506878,152651395,158584411,211.80
2026-09-16,138,25188,25447,23998311,24049084,5.38
```

**All-tools history** (every registry column, positional; no Total row):

```
date,Claude Code,Codex,OpenCode,Gemini,Copilot,Kimi,total
2026-07-01,201.46,0.00,0.00,0.00,0.00,0.00,201.46
2026-09-16,4.94,0.00,0.00,0.00,0.00,26.49,31.43
```

**Machine columns** (`--by-machine`): `machine_{name}_cost` columns appended after `cost`, alphabetical by name; under `-u all` the names are user names:

```
tool,tokens,input,output,cache,cost,machine_othermach_cost,machine_sandbox-mach_cost
Claude Code,25701156,149,25743,25675264,1005.71,999.99,5.72
```

**Leaderboard** (`share`/`delta` are fractions rounded to 3 decimals with trailing zeros dropped — `0.69`, `-0.3` (DC-11); `delta` empty for a `new` row; `machine` after `user` under `--by-machine`):

```
rank,user,cost,total_tokens,share,delta
1,sahil,12945.64,15962442751,0.69,-0.553
2,beatriz,3333.40,5562910079,0.178,-0.509
Total,,18761.10,23730948614,,
```

```
rank,user,machine,cost,total_tokens,share,delta
1,sbuser,othermach,999.99,10,0.986,
2,otheruser,othermach,8.75,110,0.009,-0.3
Total,,,1014.46,25701266,,
```

**Leaderboard history** (user columns alphabetical (DC-16); an `others` column under `--top` only when at least one user was folded; last column `total`; no Total row):

```
date,gordon,nadia,beatriz,sahil,total
2026-08,5930.55,1645.73,6786.31,28979.43,108178.25
```

```
date,eunice,sahil,others,total
2026-02,1047.91,2535.60,917.92,4501.43
```

No header-only edge case: an empty window still prints the header line alone.

## 16. Markdown Output

**Command:** `tu --md`, `tu cc h --md`, `tu h --md`, `tu m lb --md`, `tu m lbh --md`

A `## {title}` heading (the ANSI heading without the `📊`), a blank line, then a GFM table: string columns `:---`, numeric columns `---:`, thousands separators kept, `$` costs, trailing blank line. A bolded `**Total**` row follows when more than one data row is visible (snapshot, single-tool history, pivot) or, for the leaderboards, when the full ranked set has more than one user (also under `--top`); a single-row window has no Total. Rules per kind are in [usage.md › Output Formats › Markdown Output](usage.md#markdown-output--md).

**Snapshot:**

```
## Combined Usage (daily)

| Tool | Tokens | Input | Output | Cache | Cost |
| :--- | ---: | ---: | ---: | ---: | ---: |
| Claude Code | 21,871,261 | 130 | 24,644 | 21,846,487 | $4.94 |
| Kimi | 42,418,959 | 959,883 | 159,435 | 41,299,641 | $26.49 |
| **Total** | **64,290,220** | **960,013** | **184,079** | **63,146,128** | **$31.43** |
```

**Single-tool history:**

```
## Claude Code (daily, last 3 months)

| Date | Input | Output | Cache Write | Cache Read | Total | Cost |
| :--- | ---: | ---: | ---: | ---: | ---: | ---: |
| 2026-09-08 | 5,609 | 420,529 | 5,506,878 | 152,651,395 | 158,584,411 | $211.80 |
| **Total** | **196,119** | **9,818,198** | **85,693,916** | **3,294,698,542** | **3,390,406,775** | **$2,458.30** |
```

**All-tools history** (exact-zero columns dropped — Gemini kept at `$0.04` total, OpenCode/Copilot dropped):

```
## Combined Cost History (daily, last 3 months)

| Date | Claude Code | Codex | Gemini | Kimi | Cost |
| :--- | ---: | ---: | ---: | ---: | ---: |
| 2026-07-01 | $201.46 | $0.00 | $0.00 | $0.00 | $201.46 |
| **Total** | **$59,634.40** | **$1,667.70** | **$0.04** | **$5,934.71** | **$67,236.85** |
```

**Machine columns** use the machine (or user) name directly as the header, no letter codes, no legend line:

```
| Tool | Tokens | Input | Output | Cache | Cost | othermach | sandbox-mach |
```

**Leaderboard** (`share` and `delta` as percent cells; `new` for an undefined delta; `Machine` after `User` under `--by-machine`; the Total row carries `**Total**` in the `#` cell and blanks User, Share, Δ):

```
## Leaderboard (monthly)

| # | User | Cost | Tokens | Share | Δ vs Aug |
| ---: | :--- | ---: | ---: | ---: | ---: |
| 1 | sahil | $12,945.64 | 15,962,442,751 | 69.0% | -55% |
| 2 | beatriz | $3,333.40 | 5,562,910,079 | 17.8% | -51% |
| **Total** |  | **$18,761.10** | **23,730,948,614** |  |  |
```

**Leaderboard history** (`## Leaderboard History ({period})`, user columns alphabetical (DC-16), last column `Cost`):

```
## Leaderboard History (daily, last 3 months)

| Date | otheruser | sbuser | Cost |
| :--- | ---: | ---: | ---: |
| 2026-08-05 | $0.00 | $24.15 | $24.15 |
```

An empty window prints the heading and the two header lines with no data rows.

## 17. Machine Columns (`--by-machine`)

**Command:** `tu --by-machine`, `tu m --by-machine`, `tu cc h --by-machine`, `tu -u all --by-machine`, `tu m lb --by-machine`

**Snapshot** — letter-coded columns after Cost, alphabetical by machine name, one shared data-sized width (floor 9), a dim legend line after a blank line:

```
📊 Combined Usage (monthly)

Tool         |       Tokens |        Input |       Output |        Cache |         Cost |         A |         B |         C |         D
─────────────|──────────────|──────────────|──────────────|──────────────|──────────────|───────────|───────────|───────────|──────────
Claude Code  | 9,589,725,131 |      362,665 |   27,178,926 | 9,562,183,540 |    $8,895.78 |     $8.27 |   $629.91 | $1,968.85 | $6,288.75
Codex        | 1,059,910,858 |   35,216,611 |    3,150,567 | 1,021,543,680 |      $697.44 |     $0.54 |     $3.10 |     $3.84 |   $689.95
Kimi         | 5,312,806,762 |  135,977,392 |   20,173,936 | 5,156,655,434 |    $3,352.42 |     $0.00 |     $0.00 |   $990.96 | $2,361.46
─────────────|──────────────|──────────────|──────────────|──────────────|──────────────|───────────|───────────|───────────|──────────
Total        | 15,962,442,751 |  171,556,668 |   50,503,429 | 15,740,382,654 |   $12,945.64 |     $8.82 |   $633.01 | $2,963.66 | $9,340.16

Machines: A = Sahils-Mac-mini.local, B = Sahils-MacBook-Pro.local, C = dev-ws-sahil01, D = dev-ws-sahil02
```

- Machine cells hold cost under the default metric and tokens under `-t`; exact-zero cells render dim; the Total row carries per-machine totals
- Single mode shows one column, the local hostname
- **`-u all --by-machine`** keys the columns by **user** and the legend reads `Users: A = otheruser, B = sbuser`
- **Single-tool history** appends the same letter columns after Cost (Layout 3 table + columns + legend); the summary footer precedes the legend
- **All-tools pivot** and **`lbh`** warn and ignore the flag (`Warning: --by-machine is not supported with all-tools history — ignoring.` / `… with leaderboard history — ignoring.`)
- **Leaderboard** rows become `user/machine` pairs:

```
 # | User                              |       Cost |         Tokens | Share | Δ vs Aug
───|───────────────────────────────────|────────────|────────────────|───────|─────────
 1 | sahil/dev-ws-sahil02 ◂            |  $9,340.16 | 11,455,277,645 | 49.8% |     -66%
 2 | beatriz/dev-ws-beatri01           |  $3,206.02 |  5,445,803,037 | 17.1% |     -43%
 3 | sahil/dev-ws-sahil01 ◂            |  $2,963.66 |  4,041,086,220 | 15.8% |   +4757%
```

- Machine formats: JSON adds a `machines` object per tool/entry (Layout 12), CSV adds `machine_{name}_cost` columns (Layout 15), Markdown adds name-headed columns (Layout 16); all three stay cost-denominated under `-t`

## 18. Token Mode (`--metric tokens` / `-t`)

**Command:** `tu h -t`, `tu cc h -t`, `tu m lb -t`, `tu m lbh -t`, `tu -t`

**Pivot** — title `Combined Token History`, last header `Tokens`, cells and footer as plain counts; the two-zone scale keys on token p95:

```
📊 Combined Token History (daily, last 3 months)

Date       |    Claude Code |         Codex |           Kimi |         Tokens
───────────|────────────────|───────────────|────────────────|───────────────
2026-07-01 |    261,360,299 |             0 |              0 |    261,360,299
2026-07-02 |    567,905,440 |             0 |              0 |    567,905,440
───────────|────────────────|───────────────|────────────────|───────────────
Total      | 61,026,205,503 | 2,424,172,918 | 10,936,878,739 | 74,387,389,577
avg 953,684,482/day · this month 15,962,442,751 · peak 3,051,512,214 (2026-07-18)
```

**Single-tool history** — the last column reads `Tokens` and repeats the Total column's value:

```
Date         |          Input |         Output |    Cache Write |     Cache Read |          Total |        Tokens
─────────────|────────────────|────────────────|────────────────|────────────────|────────────────|──────────────
2026-09-08   |          5,609 |        420,529 |      5,506,878 |    152,651,395 |    158,584,411 |   158,584,411
```

**Leaderboard** — heading `by tokens`, rows re-ranked by tokens, Share in tokens, Δ in tokens; Cost and Tokens columns both remain:

```
Leaderboard (monthly) · 2026-09 · by tokens

# | User    |       Cost                     |         Tokens | Share | Δ vs Aug
──|─────────|────────────────────────────────|────────────────|───────|─────────
1 | sahil ◂ | $12,945.64 ███████████████████ | 15,962,442,751 | 67.3% |     -55%
2 | beatriz |  $3,333.40 ██████▋             |  5,562,910,079 | 23.4% |     -22%
```

**Snapshot** — unchanged (Layout 1). **Machine formats** — unchanged (JSON/CSV/MD ignore the metric).

## 19. Leaderboard `--top`

**`lb --top 2`** — a dim collapsed line replaces the tail; the User column widens to fit it:

```
Leaderboard (monthly) · 2026-09 · by cost

# | User        |       Cost                 |         Tokens | Share | Δ vs Aug
──|─────────────|────────────────────────────|────────────────|───────|─────────
1 | sahil ◂     | $12,945.64 ███████████████ | 15,962,442,751 | 69.0% |     -55%
2 | beatriz     |  $3,333.40 ███▉            |  5,562,910,079 | 17.8% |     -51%
  | … +4 others |                            |                |       |
──|─────────────|────────────────────────────|────────────────|───────|─────────
  | Total       | $18,761.10                 | 23,730,948,614 |       |
synced 15m ago (2026-09-15T18:54:44.502Z) · tu sync to refresh
```

**`lbh --top 2`** — the folded `others` column is sorted with the user columns by total, so it can render first (DC-08):

```
📊 Leaderboard History (monthly)

Date       |      others |       sahil |     eunice |        Cost
───────────|─────────────|─────────────|────────────|───────────────────────────
2026-06    |  $12,283.77 |  $12,558.30 | $11,715.02 |  $36,557.09 ████▊
2026-07    |  $50,269.58 |  $25,311.77 |  $9,659.17 |  $85,240.52 ███████████
```

`--top` applies to JSON (array/keys truncated), CSV and Markdown too — the Total row still sums every user and is still emitted when the full set has more than one user, even with `--top 1`. On any non-leaderboard display it warns and is ignored.

## 20. Setup, Sync, and Diagnostic Messages

All exact lines observed; stdout unless marked stderr. Absolute paths are printed as-is where noted, `~`-abbreviated elsewhere (DC-19).

**`tu init-conf`:**

```
Created ~/.config/tu/tu.conf — edit it to configure multi-machine sync.
```

```
~/.config/tu/tu.conf has commented-out fields that need uncommenting: metrics_repo.
```

```
Copied ~/.tu.conf → ~/.config/tu/tu.conf
```

**`tu init-metrics <url>`** (three stdout lines; git's own `Cloning into '…'…` / `done.` passes through on stderr):

```
Created ~/.config/tu/tu.conf — edit it to configure multi-machine sync.
Set metrics_repo = git@github.com:org/tu-metrics.git in ~/.config/tu/tu.conf
Cloned git@github.com:org/tu-metrics.git → /home/user/.tu/metrics_repo
```

Re-run on an initialized repo: `Set metrics_repo = …` then `Already initialized: /home/user/.tu/metrics_repo`. Without a URL and nothing configured (stderr, exit 1): `Error: metrics_repo is not set. Add it to ~/.config/tu/tu.conf, run 'tu init-metrics <repo-url>', or set TU_METRICS_REPO.` Directory present but not a git repo (stderr, exit 1): `Error: /home/user/.tu/metrics_repo exists but is not a git repo. Remove it or set a different metrics_dir in ~/.config/tu/tu.conf.` Two positionals (stderr, exit 2): `Error: init-metrics takes at most one argument (repo-url)` followed by the short usage.

**Auto-clone on a data command** (stderr):

```
Cloned metrics repo → /home/user/.tu/metrics_repo
```

```
Warning: could not clone metrics repo (Command failed: git clone /bad/url /home/user/.tu/metrics_repo
fatal: repository '/bad/url' does not exist
) — falling back to single mode.
```

```
Warning: metrics repo not available — falling back to single mode.
```

**`tu sync`** success (stdout): `Synced to ~/.tu/metrics_repo`. Failure (stderr, exit 1): an optional `Warning: sync pull failed — git -C … failed: Command failed: git -C /home/user/.tu/metrics_repo pull --rebase origin main\nfatal: couldn't find remote ref main\n` line, then `Error: sync failed — check network and remote config.` (the generic line is all that is printed when the commit or push step fails) (DC-18). In single mode (stderr, exit 1):

```
tu sync requires metrics_repo to be set.
Add metrics_repo to ~/.config/tu/tu.conf, run 'tu init-metrics <repo-url>', or set TU_METRICS_REPO.
```

**`--sync` on a data command** (stderr, exit 0 either way): `syncing metrics... ` then either nothing more (the table follows on stdout) or `Warning: sync pull failed — …` and `sync failed — using local data.`

**`tu sync --dry-run`** (stdout, exit 0):

```
Would write 25 day-file(s) under ~/.tu/metrics_repo/sbuser/:
  2026/sandbox-mach/cc-2026-09-08.jsonl  $211.80  (update: $211.80 → $211.80)
  2026/sandbox-mach/cc-2026-09-14.jsonl  $26.76  (new)
Would skip 2 file(s) (never-shrink guard):
  2026/sandbox-mach/cc-2026-09-15.jsonl  incoming $54.93 < existing $99999.00
  2026/sandbox-mach/cc-2026-09-16.jsonl  incoming $5.95 < existing $10724.38
Would commit: "# sbuser: update 2026-09-15", then pull --rebase origin main, then push
Dry run — nothing written, committed, or pushed.
```

The `Would skip` block appears only when at least one file would be skipped; the skip line's costs carry no thousands separators (DC-22); an equal-cost rewrite is still listed as `(update: $X → $X)` and still counts toward `Would commit` (DC-23). The commit message date is the UTC date, which can trail the day-file's local date (DC-21).

**Config diagnostics** (stderr): `tu: ~/.tu.conf is deprecated; move it to ~/.config/tu/tu.conf` (once per process); `Warning: /home/user/.config/tu/tu.conf version 9 is newer than tu supports (2). Please update tu.`; `tu: $HOME is not set; cannot locate config` (exit 1); `Error: config user "all" is reserved (used by -u all)` (exit 2).

**Flag guards** (stderr, exit 0, output continues): `Warning: --full applies to daily/weekly history — ignoring.`, `Warning: --since/--until apply to history display — ignoring.`, `Warning: --top applies to leaderboard display — ignoring.`, `Warning: -u flag requires multi mode — ignoring.`, `Warning: --by-machine is not supported with all-tools history — ignoring.`, `Warning: --by-machine is not supported with leaderboard history — ignoring.`

**Usage errors** (stderr, exit 2): `Error: --since requires a date (YYYY-MM-DD or YYYYMMDD)`, `Error: --since must be on or before --until`, `Error: --metric requires 'tokens' or 'cost'`, `Error: -t and --metric cost are incompatible`, `Error: --top requires a positive integer`, `Error: -u requires a username`, `Error: --interval requires a numeric value`, `Error: --interval minimum is 5 seconds`, `Error: --interval maximum is 3600 seconds`, `Error: --json and --csv are incompatible` (and the other five pairs, `--watch` named first: `Error: --watch and --json are incompatible`), `Error: --dry-run is supported only with 'tu sync' — run 'tu sync --dry-run' to preview a sync.`, `Unknown shell: tcsh. Supported: bash, zsh, fish`, and `tu shell-init` with no argument:

```
Usage: tu shell-init <bash|zsh|fish>

Install:
  bash: echo 'eval "$(tu shell-init bash)"' >> ~/.bashrc
  zsh:  echo 'eval "$(tu shell-init zsh)"' >> ~/.zshrc
  fish: tu shell-init fish > ~/.config/fish/completions/tu.fish
```

**Operational errors** (stderr, exit 1): `Error: lb requires multi mode — run tu init-metrics <repo-url> to set up a metrics repo`, `warning: {toolName} fetch failed (...), showing zero data` (fetch failure, output continues with zeros).

## 21. Terminal Width

- **Width source:** the width of a TTY stdout; **80 when stdout is not a TTY** (a pipe or file). The `COLUMNS` environment variable is never consulted (DC-12)
- **No compact layouts outside watch mode:** one-shot tables always render at full column width and wrap on narrow terminals (Layout 1 at 50 cols wraps every row into two lines)
- **Bar visibility:** bars render only when the width leaves at least 10 chars after the last column and the 3-char gutter; capped at 30. Observed thresholds with the captured data: pivot bars from ~70 cols (17 at 80, 30 at ≥ 100); single-tool history bars from 121 cols; leaderboard bars 19 at 80 and 30 at ≥ 91; `lb --by-machine` usually none at 80
- **Watch mode:** the terminal's live size drives the ≥60/<60 breakpoint, the history row budget, and the rain zone; the table itself is never narrowed to fit

## Color Reference

| Function | ANSI | Usage |
|----------|------|-------|
| `boldWhite` | `\x1b[1;37m` | titles, total rows, stat values, current-period date cell, `lbh` per-row leader |
| `boldCyan` | `\x1b[1;36m` | column headers |
| `dim` | `\x1b[2m` | dividers, labels, footer, legend names, weekend dates, exact-zero cells, collapsed `… +k others`, staleness footer, `┊` rule |
| `green` | `\x1b[32m` | single-tool history bars, leaderboard bars, up-arrow delta, pivot bar segment (1st tool) |
| `red` | `\x1b[31m` | down-arrow delta |
| `magenta` | `\x1b[35m` | pivot bar segment (2nd tool) |
| `blue` | `\x1b[34m` | pivot bar segment (3rd tool) |
| `cyan` | `\x1b[36m` | pivot bar segment (4th tool) |
| `yellow` | `\x1b[33m` | p95 overflow zone (reserved — beyond-scale bars only), burn rate |
| `brightGreen` | `\x1b[92m` | rain head |
| `dimGreen` | `\x1b[2;32m` | rain tail |
| `bold` | `\x1b[1m` | available, unused by the shipped layouts |

Every styled span closes with `\x1b[0m`. All colors are disabled by the `--no-color` flag or a non-empty `NO_COLOR` environment variable, and the two produce byte-identical output; color is emitted regardless of whether stdout is a TTY.
