package view

import (
	"unicode/utf8"

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

// Snapshot column layout: Tool 12-wide left (fixed); the five numerics
// 12-wide right FLOORED at 12 and data-sized beyond that — max(12, widest
// rendered cell in the column across the data and Total rows). With every
// value ≤ 12 chars the full row measures 87 visible characters
// (12 + 5×12 + 5×3); a wider value widens only its own column, keeping the
// header, dividers and rows aligned.
const (
	snapshotNameWidth = 12
	snapshotNumWidth  = 12
)

// emptyText is the snapshot's no-data line (two leading spaces).
const emptyText = "  No usage"

// snapshotColumns is the snapshot layout's base: titles, alignment, and the
// width floors snapshotColumnsSized starts from.
var snapshotColumns = []Column{
	{Title: "Tool", Width: snapshotNameWidth, Align: Left},
	{Title: "Tokens", Width: snapshotNumWidth, Align: Right},
	{Title: "Input", Width: snapshotNumWidth, Align: Right},
	{Title: "Output", Width: snapshotNumWidth, Align: Right},
	{Title: "Cache", Width: snapshotNumWidth, Align: Right},
	{Title: "Cost", Width: snapshotNumWidth, Align: Right},
}

// SnapshotOptions carries the snapshot's render inputs: the metric (only the
// watch delta indicator follows it — the columns stay token/dollar-mixed),
// the watch Prev map keyed by tool display name (nil one-shot), and Single,
// the single-source marker that titles the table after the tool.
type SnapshotOptions struct {
	Metric Metric
	Prev   map[string]float64
	Single bool
}

// snapshotTitle is the snapshot table's title: the all-tools "📊 Combined
// Usage ({p})", or the single-source "📊 {Tool} Usage ({p})" — the source,
// not the row count, decides (a one-tool all-tools snapshot is still
// "Combined").
func snapshotTitle(rows []ToolTotals, p query.Period, single bool) string {
	if single && len(rows) > 0 {
		return "📊 " + rows[0].Name + " Usage (" + p.String() + ")"
	}
	return "📊 Combined Usage (" + p.String() + ")"
}

// Snapshot builds the cross-tool snapshot table, reproducing the TS
// renderTotal rules:
//   - Title "📊 Combined Usage ({period})", or "📊 {Tool} Usage ({period})"
//     on a single-source snapshot (o.Single).
//   - One Data row per input row with TotalTokens > 0, in input order.
//   - Empty = "  No usage" and no rows when every input row has TotalTokens == 0.
//   - A Divider + Total row only when more than one row is visible; the Total
//     sums every input row, visible or not (the TS accumulates the grand
//     totals before checking visibility — a tool with cost but zero tokens is
//     hidden yet counted).
//   - Token mode changes nothing in the one-shot table; only the delta
//     indicator follows the metric: under tokens it rides the Tokens cell
//     (the Cost cell stays plain), under cost the Cost cell. The arrow sits
//     IN the cell text before padding (DeltaPadsArrow — the JS raw-length
//     padding quirk: with color on the cell is effectively unpadded).
//
// A non-nil breakdown with at least one name appends the letter-coded machine
// columns after Cost (the TS renderTotal machineCosts branch): cells in the
// DISPLAYED metric even though the base columns are metric-neutral (`tu -t
// --by-machine` shows token machine cells beside a $ Cost column), dim on
// exact zero, per-name sums on the Total row, and the legend as Table.Note.
// The names union spans every row (a hidden tool can still carry slices); the
// width pre-pass and the sums cover visible rows only. Nil ⇒ today's output
// byte-for-byte.
func Snapshot(rows []ToolTotals, p query.Period, bd *Breakdown, o SnapshotOptions) Table {
	m := o.Metric
	t := Table{
		Title:       snapshotTitle(rows, p, o.Single),
		DeltaInCell: DeltaPadsArrow,
	}

	visible := 0
	for _, r := range rows {
		if r.TotalTokens > 0 {
			visible++
		}
	}
	if visible == 0 {
		t.Empty = emptyText
		t.Columns = snapshotColumns
		return t
	}

	// The data cells build ahead of the columns: the numeric widths are
	// data-sized over them (and the Total row).
	var grand fact.Totals
	dataRows := make([]Row, 0, visible)
	visibleKeys := make([]string, 0, visible)
	for _, r := range rows {
		if r.TotalTokens > 0 {
			cells := []Cell{
				{Text: r.Name},
				{Text: render.FormatInt(r.TotalTokens)},
				{Text: render.FormatInt(r.InputTokens)},
				{Text: render.FormatInt(r.OutputTokens)},
				{Text: render.FormatInt(r.CacheCreationTokens + r.CacheReadTokens)},
				{Text: render.FormatCost(r.TotalCost)},
			}
			// Only the delta follows the metric (the TS renderTotal watch
			// branch): under tokens it rides the Tokens cell and Cost stays
			// plain; under cost it rides the Cost cell.
			if m == Tokens {
				cells[1].Delta = rowDelta(o.Prev, r.Name, float64(r.TotalTokens))
			} else {
				cells[5].Delta = rowDelta(o.Prev, r.Name, r.TotalCost)
			}
			dataRows = append(dataRows, Row{Kind: Data, Cells: cells})
			visibleKeys = append(visibleKeys, r.Name)
		}
		grand = grand.Add(r.Totals)
	}

	var totalCells []Cell
	if visible > 1 {
		totalCells = []Cell{
			{Text: "Total"},
			{Text: render.FormatInt(grand.TotalTokens)},
			{Text: render.FormatInt(grand.InputTokens)},
			{Text: render.FormatInt(grand.OutputTokens)},
			{Text: render.FormatInt(grand.CacheCreationTokens + grand.CacheReadTokens)},
			{Text: render.FormatCost(grand.TotalCost)},
		}
	}
	t.Columns = snapshotColumnsSized(dataRows, totalCells)

	names := bd.Names()
	var machineSumsSnapshot []float64
	if len(names) > 0 {
		sums, cellValues := machineSums(bd, visibleKeys, names, m)
		machineSumsSnapshot = sums
		t.Columns = machineColumns(t.Columns, names, machineWidth(cellValues, sums, m))
		t.Note = bd.note(names)
	}

	t.Rows = append(t.Rows, headerRow(t.Columns), Row{Kind: Divider})
	for i := range dataRows {
		dataRows[i].Cells = append(dataRows[i].Cells, machineCells(bd, visibleKeys[i], names, m)...)
		t.Rows = append(t.Rows, dataRows[i])
	}

	if visible > 1 {
		totalCells = append(totalCells, machineTotalCells(machineSumsSnapshot, m)...)
		t.Rows = append(t.Rows, Row{Kind: Divider}, Row{Kind: Total, Cells: totalCells})
	}
	return t
}

// snapshotColumnsSized is the snapshot layout with data-sized numeric columns
// (the machineWidth pattern): each numeric column is max(snapshotNumWidth,
// the widest rendered cell across the data and Total rows) — the header
// titles are all shorter than the floor, and the Tool column stays fixed at
// snapshotNameWidth. Every value ≤ 12 chars ⇒ the byte-identical 87-char
// layout.
func snapshotColumnsSized(dataRows []Row, totalCells []Cell) []Column {
	cols := make([]Column, len(snapshotColumns))
	copy(cols, snapshotColumns)
	measure := func(cells []Cell) {
		for i := 1; i < len(cols) && i < len(cells); i++ {
			cols[i].Width = max(cols[i].Width, utf8.RuneCountInString(cells[i].Text))
		}
	}
	for _, r := range dataRows {
		measure(r.Cells)
	}
	measure(totalCells)
	return cols
}

// headerRow builds the Header row from the column titles.
func headerRow(cols []Column) Row {
	cells := make([]Cell, len(cols))
	for i, c := range cols {
		cells[i] = Cell{Text: c.Title}
	}
	return Row{Kind: Header, Cells: cells}
}
