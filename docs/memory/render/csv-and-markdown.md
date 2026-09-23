---
type: memory
description: The CSV and Markdown encoders — RFC 4180 rows with pinned headers and toFixed(2) costs for machine consumers, and GitHub-flavoured Markdown tables with grouped numbers and a ## heading for paste targets
---

# CSV and Markdown Encoding

**Domain**: render

## Overview

`internal/render/csv` is the machine contract (RFC 4180, raw numerics); `internal/render/markdown` is the human paste-target contract (GFM tables with grouped numbers). Both consume the same [view model](/view/table-model.md) types as the [ANSI encoder](/render/ansi.md) and return `[]string`; the [command edge](/command/run-and-result.md) prints the lines.

## Requirements

### Requirement: CSV is the RFC 4180 machine contract

`internal/render/csv/csv.go` MUST emit one LF-terminated line per row with a header first: no BOM, no ANSI, no bars, no delta arrows. `quote` wraps a field in `"` and doubles inner `"` exactly when it contains `,`, `"`, `\n` or `\r`. Integers render raw via `num` (`strconv.FormatInt`, no grouping); every cost renders via `csv.Cost(x) = render.FixedHalfUp(x, 2)` — the `toFixed(2)` rule, exact-binary half-up, never `render.FormatCost`. The headers are pinned package vars: snapshot `tool,tokens,input,output,cache,cost` (`cache` = cache write + read combined); history `date,input,output,cache_write,cache_read,total,cost`; total-history `date,{Name}…,total`.

### Requirement: csv.Snapshot and csv.History

`csv.Snapshot(rows, bd)` MUST emit one row per input row with `TotalTokens > 0` and a `Total` row — summing EVERY input row, hidden ones counted — only when more than one row is visible. `csv.History(s, bd)` MUST emit one row per entry in input order and NEVER a Total row; an empty window is the header alone. A breakdown with ≥1 name appends one `machine_{name}_cost` column per sorted name (`bd.Names()`), cells `Cost(bd.CostOf(key, name))` with a `0.00` fill when the row has no slice; under `-u all` the names are users but the header prefix stays `machine_`. The snapshot Total row carries the per-name sums over visible rows. See [view/snapshot](/view/snapshot.md) and [view/history](/view/history.md) for the model.

### Requirement: csv.TotalHistory keeps every column

`csv.TotalHistory(series)` MUST keep EVERY series column in input order — scripts index positionally, so the human formats' column-omission rules never apply — over the sorted label union (`view.LabelUnion`), each series' cell `Cost(value)` with `0` when absent, and the row total last. NEVER a Total row; an empty window is the header alone. The leaderboard history reuses this shape with user columns in repo order and `others` last under `--top` ([view/history](/view/history.md)).

### Requirement: csv.Leaderboard

`csv.Leaderboard(rows, all []view.LeaderboardRow, byMachine bool)` MUST emit header `rank,user[,machine],cost,total_tokens,share,delta` — `machine` after `user` keyed on the flag, so an empty result keeps the schema — one row per SLICED row (`--top` truncates), and a `Total,,{cost},{tokens},,` row (one more empty field under `--by-machine`) summing the FULL `all` set when `len(all) > 1`, so `--top 1` on two users still emits it. Share and delta render via `csvShare(n) = FormatFloat(render.JSRound(n*1000)/1000, 'f', -1, 64)` — three-decimal rounding then trailing zeros dropped (`0.69`, `-0.3`, `0`); delta is empty for a `new` row. Header alone when empty. See [view/leaderboard](/view/leaderboard.md).

#### Scenario: --top 1 still emits the Total
- **GIVEN** two ranked users and `--top 1`
- **WHEN** `csv.Leaderboard(rows[:1], all, false)` runs
- **THEN** the output is the header, the single kept row, and `Total,,3.00,146400,,` summing both users

### Requirement: Markdown is the GFM paste-target contract

`internal/render/markdown/markdown.go` MUST emit: a `## {title}` heading, a blank line, the GFM table (`| a | b |` rows; the alignment row `:---` for the text column, `---:` for numerics), a bold `**Total**` row when more than one data row is visible, and a trailing blank line. No ANSI, no bars, no delta arrows, no staleness footer, no 📊. Numbers keep comma grouping via `render.FormatInt`; costs render `$` + two decimals via `render.FormatCost` ([number formatting](/render/number-formatting.md)). A breakdown with ≥1 name appends one right-aligned column per sorted name VERBATIM — no letter codes, no legend line — with `render.FormatCost(bd.CostOf(key, name))` cells and bold per-name sums on the `**Total**` row.

### Requirement: The four Markdown tables

`markdown.Snapshot(rows, period, bd)` emits `## Combined Usage ({period})`, rows with `TotalTokens > 0` (`Cache` combined), and a `**Total**` summing EVERY input row when more than one row is visible. `markdown.History(s, period, capActive, bd)` emits `## {Name} ({view.PeriodLabel})` and a `**Total**` when more than one entry exists. `markdown.TotalHistory(series, period, capActive, title)` takes its heading text from the caller (`Combined Cost History`, or `Leaderboard History` / `Leaderboard Token History` from the leaderboard-history caller), reads costs ONLY (ignores the metric), and drops exact-zero tool columns over all labels — the exact-zero rule, not the pivot table's significance rule — with all columns kept when every tool is exact-zero. `markdown.Leaderboard(rows, all, period, deltaLabel, byMachine)` emits `## Leaderboard ({period})` (the period word only), the header `#, User, [Machine,] Cost, Tokens, Share, Δ vs {deltaLabel}` with alignment `---:, :---, [:---,] ---:, ---:, ---:, ---:`, share as `render.FixedHalfUp(share*100, 1) + "%"`, delta via `view.DeltaCell` (`new` for a nil delta), and a `**Total**` over the FULL set when `len(all) > 1` (also under `--top`). An empty window is heading, blank, header, alignment row, blank. Titles come from the [view models](/view/snapshot.md) and [history](/view/history.md); the leaderboard shape is [view/leaderboard](/view/leaderboard.md).

#### Scenario: the pivot drops dead columns, CSV keeps them
- **GIVEN** a window where one of six tools is exact-zero over all labels
- **WHEN** `markdown.TotalHistory` and `csv.TotalHistory` run on the same series
- **THEN** the Markdown table omits the zero column while the CSV row keeps the `0.00` cell in its positional column

## Design Decisions

### CSV and Markdown split the numeric conventions
**Decision**: CSV renders raw numbers — no grouping, no `$`, `toFixed(2)` costs, snake_case `machine_{name}_cost` headers; Markdown renders grouped numbers, `$` costs, verbatim machine-name headers and a bold `**Total**`.
**Why**: CSV targets machine consumers (`awk`/`cut`, spreadsheets) that index positionally; Markdown targets humans pasting into PRs and docs.
**Rejected**: one numeric rendering path for both formats — optimizes neither consumer.
*Introduced by*: 260423-lx0g-exec-csv-completions

### Markdown always opens with a ## heading
**Decision**: every Markdown encoder emits `## {title}` first.
**Why**: the dominant paste targets (GitHub PRs and issues, docs) read better with the heading; stripping it post-hoc is trivial (`tail -n +2`).
**Rejected**: a `--no-heading` flag — listed as a non-goal, an easy follow-up if requested.
*Introduced by*: 260423-lx0g-exec-csv-completions

### CSV and Markdown strip ANSI, bars and delta arrows
**Decision**: both encoders emit plain text with no escape sequences, no inline bars and no delta arrows.
**Why**: bars are a terminal-visual affordance and arrows only have meaning in the watch context; both are useless or harmful in a paste target.
**Rejected**: carrying the terminal decorations into machine/paste output.
*Introduced by*: 260423-lx0g-exec-csv-completions

### csv.Cost is toFixed(2), not FormatCost
**Decision**: every CSV cost goes through `csv.Cost = render.FixedHalfUp(x, 2)` — the exact-binary half-up `toFixed(2)` rule; share/delta go through `csvShare` (`JSRound(n×1000)/1000`, trailing zeros dropped).
**Why**: parity with the frozen `src/node/` oracle (until plan row Z1) — `toFixed(2)` and `Math.round(n×1000)/1000` are byte surfaces; `FormatCost`'s ICU rule and `FormatFloat`'s half-even both diverge on node-verified inputs (`1.005`, `0.125`, `2.675`).
**Rejected**: reusing `render.FormatCost` for CSV (wrong on `1.005`, `0.015`, `1.045`) or `strconv.FormatFloat(x, 'f', 2, 64)` (half-even).
*Introduced by*: 260916-9ax5-history-and-periods, generalized by 260916-2gbb-leaderboard-lb-lbh

### CSV keeps every positional column
**Decision**: `csv.TotalHistory` keeps every series column with raw `0.00` cells; the human formats' column-omission rules never apply to CSV.
**Why**: CSV is the positional machine contract — scripts index columns positionally, so a column that appears or disappears by data breaks consumers.
**Rejected**: applying the Markdown/pivot omission rules to CSV — a column's presence would depend on the data.
*Introduced by*: 260423-lx0g-exec-csv-completions
