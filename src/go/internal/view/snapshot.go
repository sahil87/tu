package view

import (
	"github.com/sahil87/tu/internal/fact"
	"github.com/sahil87/tu/internal/query"
	"github.com/sahil87/tu/internal/render"
)

// ToolTotals is one snapshot row candidate: the display name (resolved by
// command from the registry — view knows no registry), the current label when
// the tool had a matching record (used by render/json), and its totals.
type ToolTotals struct {
	Name  string
	Label string
	fact.Totals
}

// Snapshot column layout: Tool 12-wide left, the five numerics 12-wide right
// — fixed, never data-sized (a wider value overflows its cell). The full row
// measures 87 visible characters (12 + 5×12 + 5×3).
const (
	snapshotNameWidth = 12
	snapshotNumWidth  = 12
)

// emptyText is the snapshot's no-data line (two leading spaces).
const emptyText = "  No usage"

// snapshotColumns is the fixed snapshot layout.
var snapshotColumns = []Column{
	{Title: "Tool", Width: snapshotNameWidth, Align: Left},
	{Title: "Tokens", Width: snapshotNumWidth, Align: Right},
	{Title: "Input", Width: snapshotNumWidth, Align: Right},
	{Title: "Output", Width: snapshotNumWidth, Align: Right},
	{Title: "Cache", Width: snapshotNumWidth, Align: Right},
	{Title: "Cost", Width: snapshotNumWidth, Align: Right},
}

// Snapshot builds the cross-tool snapshot table, reproducing the TS
// renderTotal rules:
//   - Title "📊 Combined Usage ({period})" — also for a single-source snapshot.
//   - One Data row per input row with TotalTokens > 0, in input order.
//   - Empty = "  No usage" and no rows when every input row has TotalTokens == 0.
//   - A Divider + Total row only when more than one row is visible; the Total
//     sums every input row, visible or not (the TS accumulates the grand
//     totals before checking visibility — a tool with cost but zero tokens is
//     hidden yet counted).
//   - Token mode changes nothing in the one-shot table (the delta indicator
//     is watch-only).
func Snapshot(rows []ToolTotals, p query.Period) Table {
	t := Table{
		Title:   "📊 Combined Usage (" + p.String() + ")",
		Columns: snapshotColumns,
	}

	visible := 0
	for _, r := range rows {
		if r.TotalTokens > 0 {
			visible++
		}
	}
	if visible == 0 {
		t.Empty = emptyText
		return t
	}

	t.Rows = append(t.Rows, headerRow(snapshotColumns), Row{Kind: Divider})

	var grand fact.Totals
	for _, r := range rows {
		if r.TotalTokens > 0 {
			t.Rows = append(t.Rows, Row{Kind: Data, Cells: []Cell{
				{Text: r.Name},
				{Text: render.FormatInt(r.TotalTokens)},
				{Text: render.FormatInt(r.InputTokens)},
				{Text: render.FormatInt(r.OutputTokens)},
				{Text: render.FormatInt(r.CacheCreationTokens + r.CacheReadTokens)},
				{Text: render.FormatCost(r.TotalCost)},
			}})
		}
		grand = grand.Add(r.Totals)
	}

	if visible > 1 {
		t.Rows = append(t.Rows, Row{Kind: Divider}, Row{Kind: Total, Cells: []Cell{
			{Text: "Total"},
			{Text: render.FormatInt(grand.TotalTokens)},
			{Text: render.FormatInt(grand.InputTokens)},
			{Text: render.FormatInt(grand.OutputTokens)},
			{Text: render.FormatInt(grand.CacheCreationTokens + grand.CacheReadTokens)},
			{Text: render.FormatCost(grand.TotalCost)},
		}})
	}
	return t
}

// headerRow builds the Header row from the column titles.
func headerRow(cols []Column) Row {
	cells := make([]Cell, len(cols))
	for i, c := range cols {
		cells[i] = Cell{Text: c.Title}
	}
	return Row{Kind: Header, Cells: cells}
}
