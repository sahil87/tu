package view

import (
	"time"

	"github.com/sahil87/tu/internal/fact"
	"github.com/sahil87/tu/internal/query"
	"github.com/sahil87/tu/internal/render"
)

// Entry is one history row candidate: an ISO label and its totals.
type Entry struct {
	Label string
	fact.Totals
}

// Series is one tool's history: the display name (command resolves it — view
// knows no registry) and its entries ascending by label.
type Series struct {
	Name    string
	Entries []Entry
}

// Metric is the unit cells, bars and footer render in (the TS BarMetric).
type Metric int

const (
	Cost Metric = iota
	Tokens
)

// HistoryOptions are the two history tables' shared inputs. Prev is the watch
// delta map, keyed "{Name}:{label}" / "total:{label}"; nil in B2 (B7 fills
// it).
type HistoryOptions struct {
	Period    query.Period
	Now       time.Time // for CurrentLabel(Period) and the footer's "this month" prefix
	Width     int       // terminal width budget (80 when stdout is not a TTY)
	CapActive bool      // append ", last 3 months" to the heading
	Metric    Metric
	Prev      map[string]float64
}

// PeriodLabel builds the parenthetical that follows a history title: the
// period word plus the implicit-cap hint when the 3-month cap is active (the
// TS periodLabel).
func PeriodLabel(p query.Period, capActive bool) string {
	if capActive {
		return p.String() + ", last 3 months"
	}
	return p.String()
}

// emptyHistory is the history tables' no-data line (two leading spaces).
const emptyHistory = "  No data"

// rowDelta resolves a row's watch delta from the Prev map: up when the value
// grew, down when it shrank, none without a map entry.
func rowDelta(prev map[string]float64, key string, value float64) Delta {
	if prev == nil {
		return DeltaNone
	}
	p, ok := prev[key]
	if !ok {
		return DeltaNone
	}
	if value > p {
		return DeltaUp
	}
	if value < p {
		return DeltaDown
	}
	return DeltaNone
}

// Single-tool history column layout (layouts §3): Date 12-wide left, the five
// token columns 14-wide right; the metric column is data-sized (floor 9). The
// fixed part measures 97 visible chars (12 + 5×14 + 5×3).
const (
	historyDateWidth = 12
	historyNumWidth  = 14
	historyBodyWidth = historyDateWidth + 5*historyNumWidth + 5*3
)

// gutterWidth is the " | " separator between the table body and the metric
// column; the bar renders with a leading space (the trailing 1).
const (
	gutterWidth = 3
	barLeading  = 1
)

// barBudget is the one bar-width rule both history tables share:
// min(width − bodyWidth − gutter − costWidth − leading − reserve, maxBarWidth),
// shown when the result is at least minBarArea. bodyWidth is the table's
// fixed body (historyBodyWidth for the single-tool table; the pivot's
// computed width); reserve is extra width the bar area yields — the pivot's
// Prev-indicator column, or the single-tool table's machine columns
// (len(names) × (machineWidth + gutter)).
func barBudget(width, bodyWidth, costWidth, reserve int) (barWidth int, show bool) {
	barWidth = min(width-bodyWidth-gutterWidth-costWidth-barLeading-reserve, maxBarWidth)
	return barWidth, barWidth >= minBarArea
}

// History builds the single-tool history table (the TS renderHistory):
//
//   - Title "📊 {Name} ({period}[, last 3 months])" — metric-independent.
//   - Empty = "  No data" with no rows when the series has no entries.
//   - Columns Date 12 Left; Input/Output/Cache Write/Cache Read/Total 14
//     Right; last column Cost/Tokens Right, data-sized floor 9 (every row's
//     metric value plus their sum).
//   - barWidth = min(Width − 97 − 3 − costWidth − machineColsWidth − 1, 30);
//     bars when ≥ 10.
//   - Data rows carry FormatInt token cells, the metric cell (Dim iff exactly
//     0), the label style (Current/Weekend/Plain), the Prev delta and a solid
//     bar (Segments nil); a Separator precedes a Data row whose label[:7]
//     changes, daily only, never the first.
//   - Divider + Total + Footer only when len(entries) > 1.
//
// A non-nil breakdown with at least one name appends the letter-coded machine
// columns after the metric column (cells before the delta indicator and bar —
// the existing encoder appends those after the last cell): cells in the
// displayed metric, dim on exact zero, per-name sums over EVERY entry on the
// Total row, and the legend as Table.Note. machineColsWidth = len(names) ×
// (machineWidth + 3) comes out of the bar budget (the TS machineColsWidth).
// Nil ⇒ today's output byte-for-byte.
func History(s Series, o HistoryOptions, bd *Breakdown) Table {
	t := Table{
		Title:       "📊 " + s.Name + " (" + PeriodLabel(o.Period, o.CapActive) + ")",
		DeltaSpaced: true,
	}
	if len(s.Entries) == 0 {
		t.Empty = emptyHistory
		return t
	}

	m := o.Metric
	values, sumValue := historyValues(s, m)
	costWidth := metricColumnWidth(append(values, sumValue), m)

	names := bd.Names()
	machineColsWidth := 0
	var mSums []float64
	if len(names) > 0 {
		keys := make([]string, len(s.Entries))
		for i, e := range s.Entries {
			keys[i] = e.Label
		}
		sums, cellValues := machineSums(bd, keys, names, m)
		mSums = sums
		w := machineWidth(cellValues, sums, m)
		machineColsWidth = len(names) * (w + 3) // the " | " + padded cell per column
		t.Columns = machineColumns(historyColumns(costWidth, m), names, w)
		t.Note = bd.note(names)
	} else {
		t.Columns = historyColumns(costWidth, m)
	}

	barWidth, showBars := barBudget(o.Width, historyBodyWidth, costWidth, machineColsWidth)
	if showBars {
		t.Scale = ComputeScale(values, barWidth)
	}

	rows, labels, sums := historyRows(s, values, o, m, t.Scale, showBars, bd, names)
	t.Rows = append([]Row{headerRow(t.Columns), {Kind: Divider}}, rows...)

	if len(s.Entries) > 1 {
		cells := []Cell{
			{Text: "Total"},
			{Text: render.FormatInt(sums.InputTokens)},
			{Text: render.FormatInt(sums.OutputTokens)},
			{Text: render.FormatInt(sums.CacheCreationTokens)},
			{Text: render.FormatInt(sums.CacheReadTokens)},
			{Text: render.FormatInt(sums.TotalTokens)},
			{Text: fmtMetric(sumValue, m)},
		}
		cells = append(cells, machineTotalCells(mSums, m)...)
		t.Rows = append(t.Rows, Row{Kind: Divider}, Row{Kind: Total, Cells: cells})
		t.Footer = footerText(labels, values, o.Period, t.Scale, o.Now, m)
	}
	return t
}

// historyValues is the series' per-entry metric values plus their sum. The
// metric column is sized from the data before the bar budget: every row's
// value plus their sum (the Total row's cell, usually the longest).
func historyValues(s Series, m Metric) (values []float64, sum float64) {
	values = make([]float64, len(s.Entries))
	for i, e := range s.Entries {
		values[i] = metricValue(e.Totals, m)
		sum += values[i]
	}
	return values, sum
}

// historyColumns is the fixed single-tool layout: Date 12 Left; the five
// token columns 14 Right; the data-sized metric column (floor 9) last.
func historyColumns(costWidth int, m Metric) []Column {
	return []Column{
		{Title: "Date", Width: historyDateWidth, Align: Left},
		{Title: "Input", Width: historyNumWidth, Align: Right},
		{Title: "Output", Width: historyNumWidth, Align: Right},
		{Title: "Cache Write", Width: historyNumWidth, Align: Right},
		{Title: "Cache Read", Width: historyNumWidth, Align: Right},
		{Title: "Total", Width: historyNumWidth, Align: Right},
		{Title: metricHeader(m), Width: costWidth, Align: Right},
	}
}

// historyRows builds the data rows — FormatInt token cells, the metric cell,
// the machine cells (between the metric cell and the delta/bar), the label
// style, the Prev delta and a solid bar (Segments nil); a Separator precedes
// a Data row whose label[:7] changes, daily only, never the first — and sums
// the totals the Total row needs.
func historyRows(s Series, values []float64, o HistoryOptions, m Metric, scale Scale, showBars bool, bd *Breakdown, names []string) (rows []Row, labels []string, sums fact.Totals) {
	labels = make([]string, len(s.Entries))
	prevMonthPrefix := ""
	for i, e := range s.Entries {
		labels[i] = e.Label
		// Month-boundary separator: daily only, never before the first row.
		monthPrefix := monthPrefixOf(e.Label)
		if o.Period == query.Daily && prevMonthPrefix != "" && monthPrefix != prevMonthPrefix {
			rows = append(rows, Row{Kind: Separator})
		}
		prevMonthPrefix = monthPrefix

		cells := []Cell{
			{Text: e.Label, Style: labelStyle(e.Label, o.Period, o.Now)},
			{Text: render.FormatInt(e.InputTokens)},
			{Text: render.FormatInt(e.OutputTokens)},
			{Text: render.FormatInt(e.CacheCreationTokens)},
			{Text: render.FormatInt(e.CacheReadTokens)},
			{Text: render.FormatInt(e.TotalTokens)},
			metricCell(values[i], m),
		}
		cells = append(cells, machineCells(bd, e.Label, names, m)...)
		row := Row{
			Kind:  Data,
			Cells: cells,
			Delta: rowDelta(o.Prev, s.Name+":"+e.Label, values[i]),
		}
		if showBars {
			row.Bar = rowBar(values[i], scale, nil)
		}
		rows = append(rows, row)
		sums = sums.Add(e.Totals)
	}
	return rows, labels, sums
}

// monthPrefixOf is a label's YYYY-MM prefix (the TS label.slice(0, 7)).
func monthPrefixOf(label string) string {
	if len(label) >= 7 {
		return label[:7]
	}
	return label
}
