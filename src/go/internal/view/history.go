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

// History builds the single-tool history table (the TS renderHistory):
//
//   - Title "📊 {Name} ({period}[, last 3 months])" — metric-independent.
//   - Empty = "  No data" with no rows when the series has no entries.
//   - Columns Date 12 Left; Input/Output/Cache Write/Cache Read/Total 14
//     Right; last column Cost/Tokens Right, data-sized floor 9 (every row's
//     metric value plus their sum).
//   - barWidth = min(Width − 97 − 3 − costWidth − 1, 30); bars when ≥ 10.
//   - Data rows carry FormatInt token cells, the metric cell (Dim iff exactly
//     0), the label style (Current/Weekend/Plain), the Prev delta and a solid
//     bar (Segments nil); a Separator precedes a Data row whose label[:7]
//     changes, daily only, never the first.
//   - Divider + Total + Footer only when len(entries) > 1.
func History(s Series, o HistoryOptions) Table {
	t := Table{
		Title:       "📊 " + s.Name + " (" + PeriodLabel(o.Period, o.CapActive) + ")",
		DeltaSpaced: true,
	}
	if len(s.Entries) == 0 {
		t.Empty = emptyHistory
		return t
	}

	// The metric column is sized from the data before the bar budget: every
	// row's value plus their sum (the Total row's cell, usually the longest).
	m := o.Metric
	sumValue := 0.0
	values := make([]float64, len(s.Entries))
	for i, e := range s.Entries {
		values[i] = metricValue(e.Totals, m)
		sumValue += values[i]
	}
	costWidth := metricColumnWidth(append(values, sumValue), m)

	barWidth := min(o.Width-historyBodyWidth-gutterWidth-costWidth-barLeading, maxBarWidth)
	showBars := barWidth >= minBarArea
	if showBars {
		t.Scale = ComputeScale(values, barWidth)
	}

	t.Columns = []Column{
		{Title: "Date", Width: historyDateWidth, Align: Left},
		{Title: "Input", Width: historyNumWidth, Align: Right},
		{Title: "Output", Width: historyNumWidth, Align: Right},
		{Title: "Cache Write", Width: historyNumWidth, Align: Right},
		{Title: "Cache Read", Width: historyNumWidth, Align: Right},
		{Title: "Total", Width: historyNumWidth, Align: Right},
		{Title: metricHeader(m), Width: costWidth, Align: Right},
	}
	t.Rows = append(t.Rows, headerRow(t.Columns), Row{Kind: Divider})

	var sums fact.Totals
	prevMonthPrefix := ""
	labels := make([]string, len(s.Entries))
	for i, e := range s.Entries {
		labels[i] = e.Label
		// Month-boundary separator: daily only, never before the first row.
		monthPrefix := monthPrefixOf(e.Label)
		if o.Period == query.Daily && prevMonthPrefix != "" && monthPrefix != prevMonthPrefix {
			t.Rows = append(t.Rows, Row{Kind: Separator})
		}
		prevMonthPrefix = monthPrefix

		row := Row{
			Kind: Data,
			Cells: []Cell{
				{Text: e.Label, Style: labelStyle(e.Label, o.Period, o.Now)},
				{Text: render.FormatInt(e.InputTokens)},
				{Text: render.FormatInt(e.OutputTokens)},
				{Text: render.FormatInt(e.CacheCreationTokens)},
				{Text: render.FormatInt(e.CacheReadTokens)},
				{Text: render.FormatInt(e.TotalTokens)},
				metricCell(values[i], m),
			},
			Delta: rowDelta(o.Prev, s.Name+":"+e.Label, values[i]),
		}
		if showBars {
			row.Bar = rowBar(values[i], t.Scale, nil)
		}
		t.Rows = append(t.Rows, row)
		sums = sums.Add(e.Totals)
	}

	if len(s.Entries) > 1 {
		t.Rows = append(t.Rows, Row{Kind: Divider}, Row{Kind: Total, Cells: []Cell{
			{Text: "Total"},
			{Text: render.FormatInt(sums.InputTokens)},
			{Text: render.FormatInt(sums.OutputTokens)},
			{Text: render.FormatInt(sums.CacheCreationTokens)},
			{Text: render.FormatInt(sums.CacheReadTokens)},
			{Text: render.FormatInt(sums.TotalTokens)},
			{Text: fmtMetric(sumValue, m)},
		}})
		t.Footer = footerText(labels, values, o.Period, t.Scale, o.Now, m)
	}
	return t
}

// monthPrefixOf is a label's YYYY-MM prefix (the TS label.slice(0, 7)).
func monthPrefixOf(label string) string {
	if len(label) >= 7 {
		return label[:7]
	}
	return label
}
