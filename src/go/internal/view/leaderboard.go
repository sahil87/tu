package view

import (
	"strconv"
	"unicode/utf8"

	"github.com/sahil87/tu/internal/fact"
	"github.com/sahil87/tu/internal/query"
	"github.com/sahil87/tu/internal/render"
)

// LeaderboardRow is one ranked key: the user (and machine under
// --by-machine), its window totals, its share of the grand total in the
// display metric, and its fractional change vs the previous window
// (nil = "new").
type LeaderboardRow struct {
	Rank    int
	User    string
	Machine string // "" unless --by-machine
	fact.Totals
	Share float64
	Delta *float64
}

// LeaderboardOptions carries the lb table's render inputs (the TS
// LeaderboardRenderOptions).
type LeaderboardOptions struct {
	Period      query.Period
	WindowLabel string // heading: the current window's label
	DeltaLabel  string // Δ header: the previous window's label, or "prev"
	Metric      Metric
	PinnedUser  string             // rows whose User equals it carry " ◂"
	Top         int                // 0 = all; else rows past Top collapse into "… +k others"
	Width       int                // terminal budget (80 when piped)
	LastSync    string             // config.LastSync text: "never" or "{relative} ({ISO})"
	Prev        map[string]float64 // watch deltas keyed by user or user/machine; nil until B7
}

// leaderboardKey is the row key: the user name, or "user/machine" under
// --by-machine (the TS leaderboardKey).
func leaderboardKey(r LeaderboardRow) string {
	if r.Machine != "" {
		return r.User + "/" + r.Machine
	}
	return r.User
}

// fmtDeltaCell renders a fractional delta as a signed whole-percent cell
// (the TS fmtDeltaCell): "new" when nil, else "{sign}{pct}%" with
// pct = JSRound(delta × 100) (the JS Math.round — half toward +∞) and "+"
// for pct ≥ 0 ("+0%", "-55%", "+4757%").
func fmtDeltaCell(d *float64) string {
	if d == nil {
		return "new"
	}
	pct := render.JSRound(*d * 100)
	sign := ""
	if pct >= 0 {
		sign = "+"
	}
	return sign + strconv.FormatInt(int64(pct), 10) + "%"
}

// DeltaCell is fmtDeltaCell exported for the Markdown encoder (the TS mdDelta
// is the same rule).
func DeltaCell(d *float64) string {
	return fmtDeltaCell(d)
}

// shareCell renders a share fraction as a one-decimal percent (the TS
// `(share * 100).toFixed(1) + "%"` — exact-binary half-up: "69.0%", "100.0%").
func shareCell(share float64) string {
	return render.FixedHalfUp(share*100, 1) + "%"
}

// Leaderboard builds the lb table (the TS renderLeaderboard, intake §4):
//
//   - Title "Leaderboard ({period}) · {WindowLabel} · by {cost|tokens}" — NO
//     📊 (DC-07); the period word only, never the cap hint (lb is never
//     capped).
//   - Footer (the staleness line, dim): "synced {LastSync} · tu sync to
//     refresh" when LastSync != "never", else "never synced · tu sync to
//     refresh". Always present, including the empty state.
//   - Empty = "  No data" on zero rows (no columns).
//   - Visible rows are rows[:Top] when 0 < Top < len(rows); the rest collapse
//     into one dim "… +k others" Data row with blank cells and no bar.
//   - The pinned user's rows carry " ◂" (every machine row of the pinned user
//     under --by-machine).
//   - Widths over the VISIBLE rows (the collapsed label counts toward the
//     User width); the Cost/Tokens widths also size the grand total, which
//     folds over ALL rows. All rune-counted.
//   - Columns: # Right, User Left, Cost Right with BarAfter, Tokens Right,
//     Share Right, "Δ vs {DeltaLabel}" Right.
//   - The bar scales over the visible rows' metric values through the shared
//     barBudget with bodyWidth = every non-metric column plus gutters and
//     reserve = 1 when Prev != nil.
//   - A Divider + Total row (blank rank/share/delta cells) when len(rows) ≥ 2
//     — the full set, also under --top.
//   - The watch delta arrow rides the metric cell (Cost under cost, Tokens
//     under tokens) from Prev[key] (key = user or user/machine), appended
//     AFTER the padded text (DeltaAfterPad) with the exact-zero Dim wrap
//     around the composite; the bar budget reserves one column when Prev !=
//     nil.
func Leaderboard(rows []LeaderboardRow, o LeaderboardOptions) Table {
	m := o.Metric
	byMetric := "cost"
	if m == Tokens {
		byMetric = "tokens"
	}
	t := Table{
		Title:       "Leaderboard (" + o.Period.String() + ") · " + o.WindowLabel + " · by " + byMetric,
		Footer:      leaderboardFooter(o.LastSync),
		DeltaInCell: DeltaAfterPad,
	}
	if len(rows) == 0 {
		t.Empty = emptyHistory // "  No data"
		return t
	}

	visible, collapsedLabel := leaderboardVisible(rows, o.Top)
	nameCell := leaderboardNameCell(o.PinnedUser)
	deltaHeader := "Δ vs " + o.DeltaLabel

	var grandCost float64
	var grandTokens int64
	for _, r := range rows {
		grandCost += r.TotalCost
		grandTokens += r.TotalTokens
	}

	w, metricValues := leaderboardMeasure(visible, grandCost, grandTokens, collapsedLabel, deltaHeader, nameCell, m)
	t.Columns = leaderboardColumns(w, deltaHeader)

	// The bar sits between Cost and Tokens; the budget sees every non-metric
	// column and its gutters (the TS tableWidth minus the Cost column and one
	// gutter), plus the watch indicator reserve.
	reserve := 0
	if o.Prev != nil {
		reserve = 1
	}
	bodyWidth := w.rank + gutterWidth + w.name + gutterWidth + w.tokens + gutterWidth + w.share + gutterWidth + w.delta
	barWidth, showBars := barBudget(o.Width, bodyWidth, w.cost, reserve)
	if showBars {
		t.Scale = ComputeScale(metricValues, barWidth)
	}

	t.Rows = leaderboardRows(visible, collapsedLabel, nameCell, t.Columns, t.Scale, showBars, m, o.Prev)
	if len(rows) >= 2 {
		t.Rows = leaderboardTotal(t.Rows, grandCost, grandTokens)
	}
	return t
}

// leaderboardVisible slices the rendered rows under --top (the TS
// rows.slice(0, top)) and computes the collapsed tail's label; collapsed
// rows still count toward the Total row and every share denominator.
func leaderboardVisible(rows []LeaderboardRow, top int) (visible []LeaderboardRow, collapsedLabel string) {
	visible = rows
	if top > 0 && top < len(rows) {
		visible = rows[:top]
	}
	if collapsed := len(rows) - len(visible); collapsed > 0 {
		collapsedLabel = "… +" + strconv.Itoa(collapsed) + " others"
	}
	return visible, collapsedLabel
}

// leaderboardNameCell is the name-cell builder (the TS nameCell): the row
// key plus " ◂" when the row's user is the pinned one (every machine row of
// the pinned user under --by-machine).
func leaderboardNameCell(pinnedUser string) func(LeaderboardRow) string {
	return func(r LeaderboardRow) string {
		name := leaderboardKey(r)
		if r.User == pinnedUser {
			name += " ◂"
		}
		return name
	}
}

// leaderboardWidths is the leaderboard's column widths in runes.
type leaderboardWidths struct {
	rank, name, cost, tokens, share, delta int
}

// leaderboardMeasure is the width pre-pass (the TS renderLeaderboard's width
// block): every width over the VISIBLE rows — the collapsed label counts
// toward the User width, and the grand totals (folded over ALL rows by the
// caller) size the Cost/Tokens columns — plus the visible rows' metric
// values for the bar scale. All rune-counted (Δ, ◂, … are single runes).
func leaderboardMeasure(visible []LeaderboardRow, grandCost float64, grandTokens int64, collapsedLabel, deltaHeader string, nameCell func(LeaderboardRow) string, m Metric) (leaderboardWidths, []float64) {
	w := leaderboardWidths{
		rank:  1,
		name:  max(len("User"), utf8.RuneCountInString(collapsedLabel)),
		share: len("Share"),
		delta: utf8.RuneCountInString(deltaHeader),
	}
	costValues := []float64{grandCost}
	tokenValues := []float64{float64(grandTokens)}
	metricValues := make([]float64, len(visible))
	for i, r := range visible {
		w.rank = max(w.rank, len(strconv.Itoa(r.Rank)))
		w.name = max(w.name, utf8.RuneCountInString(nameCell(r)))
		w.share = max(w.share, len(shareCell(r.Share)))
		w.delta = max(w.delta, utf8.RuneCountInString(fmtDeltaCell(r.Delta)))
		costValues = append(costValues, r.TotalCost)
		tokenValues = append(tokenValues, float64(r.TotalTokens))
		metricValues[i] = metricValue(r.Totals, m)
	}
	w.cost = metricColumnWidth(costValues, Cost)
	w.tokens = metricColumnWidth(tokenValues, Tokens)
	return w, metricValues
}

// leaderboardColumns assembles the columns (the TS column order): # Right,
// User Left, Cost Right with BarAfter (the mid-row bar), Tokens Right, Share
// Right, "Δ vs {label}" Right.
func leaderboardColumns(w leaderboardWidths, deltaHeader string) []Column {
	return []Column{
		{Title: "#", Width: w.rank, Align: Right},
		{Title: "User", Width: w.name, Align: Left},
		{Title: "Cost", Width: w.cost, Align: Right, BarAfter: true},
		{Title: "Tokens", Width: w.tokens, Align: Right},
		{Title: "Share", Width: w.share, Align: Right},
		{Title: deltaHeader, Width: w.delta, Align: Right},
	}
}

// leaderboardRows assembles the header, divider, data rows (rank, name, dim
// exact-zero Cost/Tokens cells, the toFixed(1) share, the Math.round delta,
// the solid bar) and the collapsed "… +k others" row (dim name cell, blank
// cells, no bar). The watch delta arrow rides the metric cell — Cost under
// cost, Tokens under tokens — from prev[leaderboardKey(row)].
func leaderboardRows(visible []LeaderboardRow, collapsedLabel string, nameCell func(LeaderboardRow) string, columns []Column, scale Scale, showBars bool, m Metric, prev map[string]float64) []Row {
	rows := []Row{headerRow(columns), {Kind: Divider}}
	for _, r := range visible {
		row := Row{Kind: Data, Cells: []Cell{
			{Text: strconv.Itoa(r.Rank)},
			{Text: nameCell(r)},
			{Text: render.FormatCost(r.TotalCost), Dim: r.TotalCost == 0},
			{Text: render.FormatInt(r.TotalTokens), Dim: r.TotalTokens == 0},
			{Text: shareCell(r.Share)},
			{Text: fmtDeltaCell(r.Delta)},
		}}
		value := metricValue(r.Totals, m)
		if m == Tokens {
			row.Cells[3].Delta = rowDelta(prev, leaderboardKey(r), value)
		} else {
			row.Cells[2].Delta = rowDelta(prev, leaderboardKey(r), value)
		}
		if showBars {
			row.Bar = rowBar(value, scale, nil)
		}
		rows = append(rows, row)
	}
	if collapsedLabel != "" {
		rows = append(rows, Row{Kind: Data, Cells: []Cell{
			{}, {Text: collapsedLabel, Dim: true}, {}, {}, {}, {},
		}})
	}
	return rows
}

// leaderboardTotal appends the Divider + Total row (blank rank/share/delta
// cells) over the FULL row set — also under --top.
func leaderboardTotal(rows []Row, grandCost float64, grandTokens int64) []Row {
	return append(rows,
		Row{Kind: Divider},
		Row{Kind: Total, Cells: []Cell{
			{}, {Text: "Total"}, {Text: render.FormatCost(grandCost)},
			{Text: render.FormatInt(grandTokens)}, {}, {},
		}})
}

// leaderboardFooter is the staleness line's text (dim at encode — ANSI only;
// CSV/JSON/MD carry no footer).
func leaderboardFooter(lastSync string) string {
	if lastSync != "" && lastSync != "never" {
		return "synced " + lastSync + " · tu sync to refresh"
	}
	return "never synced · tu sync to refresh"
}
