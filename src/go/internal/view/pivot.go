package view

import (
	"sort"

	"github.com/sahil87/tu/internal/query"
)

// Significance thresholds for the pivot's column omission (the TS
// NEGLIGIBLE_* constants): a column is noise below $1.00 / 1,000 tokens or
// below 0.1% of the window grand total — boundary values are kept.
const (
	negligibleCostAbs   = 1.0
	negligibleTokensAbs = 1000.0
	negligibleShare     = 0.001
)

// negligibleAbs is the absolute significance floor in the displayed unit.
func negligibleAbs(m Metric) float64 {
	if m == Tokens {
		return negligibleTokensAbs
	}
	return negligibleCostAbs
}

// pivotDateWidth is the pivot's Date column width (the TS PIVOT_DATE_WIDTH):
// ISO daily labels are 10 chars.
const pivotDateWidth = 10

// TotalHistory builds the cross-tool history pivot (the TS
// renderTotalHistory), plus the lbh hooks on HistoryOptions:
//
//   - Title "📊 Combined Cost History ({period}[, last 3 months])", or "Token"
//     under tokens — or o.Title verbatim when set (lbh).
//   - Labels are the sorted union of every series' labels; none →
//     Empty = "  No data".
//   - Visible tools by the significance rule (≥ negligibleAbs AND ≥ 0.001 ×
//     grand, boundary kept), falling back to nonzero, then to all; with
//     KeepAllColumns every series is a column, all-zero ones included.
//     rowValue and grandTotal sum over ALL series (an omitted column still
//     counts).
//   - With RankColumns the visible columns (and each row's values) are
//     reordered by descending window total in the metric, stable on ties,
//     AFTER pivotData computes toolSums over the input order; widths, header,
//     cells, bar segments, legend swatches and the Total row follow the
//     reorder.
//   - With HighlightLeader each Data row's max cell (strict >, first wins;
//     index 0 when all equal) gets Cell.Leader.
//   - Widths: Date 10; per tool max(len(Name), 9, its cells, its Total);
//     last column data-sized floor 9.
//   - barWidth = min(Width − tableWidth − 3 − costWidth − 1 −
//     indicatorReserve, 30), indicatorReserve = 1 when Prev != nil; bars when
//     ≥ 10; the bar's Main is apportioned over the visible tools' values.
//   - Separator/Total/Footer rules as in History; Legend one Swatch per
//     visible tool iff bars shown ∧ ≥ 2 visible; DeltaSpaced = false (the
//     space-less "$128.13↑" form).
func TotalHistory(series []Series, o HistoryOptions) Table {
	m := o.Metric
	title := "📊 Combined Cost History"
	if m == Tokens {
		title = "📊 Combined Token History"
	}
	if o.Title != "" {
		title = o.Title
	} else {
		title += " (" + PeriodLabel(o.Period, o.CapActive) + ")"
	}
	t := Table{Title: title}

	labels := LabelUnion(series)
	if len(labels) == 0 {
		t.Empty = emptyHistory
		return t
	}

	valueMap := pivotValues(series, m)
	visible := significant(series, valueMap, labels, m)
	if o.KeepAllColumns {
		visible = make([]int, len(series))
		for i := range series {
			visible[i] = i
		}
	}
	rows, toolSums, grandTotal := pivotData(series, valueMap, visible, labels)
	if o.RankColumns {
		visible, toolSums = rankColumns(visible, toolSums, rows)
	}
	toolWidths, rowValues, costWidth := pivotWidths(series, visible, rows, toolSums, grandTotal, m)

	indicatorReserve := 0
	if o.Prev != nil {
		indicatorReserve = 1
	}
	barWidth, showBars := barBudget(o.Width, pivotBodyWidth(toolWidths), costWidth, indicatorReserve)
	if showBars {
		t.Scale = ComputeScale(rowValues, barWidth)
	}

	t.Columns = pivotColumns(series, visible, toolWidths, costWidth, m)
	t.Rows = pivotRows(rows, visible, t.Columns, o, m, t.Scale, showBars)
	if len(labels) > 1 {
		pivotTotals(&t, labels, rowValues, toolSums, grandTotal, series, visible, o, m, showBars)
	}
	return t
}

// rankColumns reorders the visible columns by descending window total (ties
// keep first-seen order — the TS columnOrder "total-desc" sort with the
// first-seen index as tie-break) and permutes each row's values to match.
func rankColumns(visible []int, toolSums []float64, rows []pivotRow) ([]int, []float64) {
	order := make([]int, len(visible)) // positions into the current visible set
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		return toolSums[order[a]] > toolSums[order[b]]
	})
	newVisible := make([]int, len(visible))
	newSums := make([]float64, len(toolSums))
	for ni, oi := range order {
		newVisible[ni] = visible[oi]
		newSums[ni] = toolSums[oi]
	}
	for ri := range rows {
		values := make([]float64, len(order))
		for ni, oi := range order {
			values[ni] = rows[ri].values[oi]
		}
		rows[ri].values = values
	}
	return newVisible, newSums
}

// pivotValues builds one lookup per series: label → the displayed metric
// value. Cells, the row column, the Total row, bars, segments and the footer
// all render in the metric.
func pivotValues(series []Series, m Metric) []map[string]float64 {
	valueMap := make([]map[string]float64, len(series))
	for i, s := range series {
		values := make(map[string]float64, len(s.Entries))
		for _, e := range s.Entries {
			values[e.Label] = metricValue(e.Totals, m)
		}
		valueMap[i] = values
	}
	return valueMap
}

// pivotRow is one label's pivot data: the per-visible-tool values and the
// row total over ALL series (an omitted column still counts in the row, the
// Total, and the footer).
type pivotRow struct {
	label    string
	values   []float64
	rowValue float64
}

// pivotData pre-computes the per-row value data before the width budget:
// rowValue and grandTotal sum over ALL series; values/toolSums cover the
// visible set.
func pivotData(series []Series, valueMap []map[string]float64, visible []int, labels []string) (rows []pivotRow, toolSums []float64, grandTotal float64) {
	rows = make([]pivotRow, len(labels))
	toolSums = make([]float64, len(visible))
	for li, label := range labels {
		rowValue := 0.0
		for i := range series {
			rowValue += valueMap[i][label]
		}
		values := make([]float64, len(visible))
		for vi, si := range visible {
			values[vi] = valueMap[si][label]
			toolSums[vi] += values[vi]
		}
		grandTotal += rowValue
		rows[li] = pivotRow{label, values, rowValue}
	}
	return rows, toolSums, grandTotal
}

// pivotWidths sizes the columns: per visible tool max(name, floor, longest
// cell including its Total-row sum); the row column is data-sized over every
// row value plus the grand total. rowValues is returned for the bar scale
// and the footer.
func pivotWidths(series []Series, visible []int, rows []pivotRow, toolSums []float64, grandTotal float64, m Metric) (toolWidths []int, rowValues []float64, costWidth int) {
	toolWidths = make([]int, len(visible))
	for vi, si := range visible {
		w := max(len(series[si].Name), metricFloor)
		for _, r := range rows {
			w = max(w, len(fmtMetric(r.values[vi], m)))
		}
		toolWidths[vi] = max(w, len(fmtMetric(toolSums[vi], m)))
	}
	rowValues = make([]float64, len(rows))
	for i, r := range rows {
		rowValues[i] = r.rowValue
	}
	return toolWidths, rowValues, metricColumnWidth(append(rowValues, grandTotal), m)
}

// pivotBodyWidth is the pivot's fixed body width: the Date column plus each
// visible tool column and its gutter.
func pivotBodyWidth(toolWidths []int) int {
	w := pivotDateWidth
	for _, tw := range toolWidths {
		w += tw + gutterWidth
	}
	return w
}

// pivotColumns assembles the pivot's columns: Date, one per visible tool,
// then the data-sized metric column.
func pivotColumns(series []Series, visible []int, toolWidths []int, costWidth int, m Metric) []Column {
	cols := make([]Column, 0, len(visible)+2)
	cols = append(cols, Column{Title: "Date", Width: pivotDateWidth, Align: Left})
	for vi, si := range visible {
		cols = append(cols, Column{Title: series[si].Name, Width: toolWidths[vi], Align: Right})
	}
	return append(cols, Column{Title: metricHeader(m), Width: costWidth, Align: Right})
}

// pivotRows assembles the header, divider, month separators and data rows:
// each data row carries the label style, the Prev delta, per-visible-tool
// metric cells, the row total, and the apportioned bar when bars show.
func pivotRows(rows []pivotRow, visible []int, columns []Column, o HistoryOptions, m Metric, scale Scale, showBars bool) []Row {
	out := []Row{headerRow(columns), {Kind: Divider}}
	prevMonthPrefix := ""
	for _, r := range rows {
		monthPrefix := monthPrefixOf(r.label)
		if o.Period == query.Daily && prevMonthPrefix != "" && monthPrefix != prevMonthPrefix {
			out = append(out, Row{Kind: Separator})
		}
		prevMonthPrefix = monthPrefix

		row := Row{
			Kind:  Data,
			Cells: make([]Cell, 0, len(visible)+2),
			Delta: rowDelta(o.Prev, "total:"+r.label, r.rowValue),
		}
		row.Cells = append(row.Cells, Cell{Text: r.label, Style: labelStyle(r.label, o.Period, o.Now)})
		for _, v := range r.values {
			row.Cells = append(row.Cells, metricCell(v, m))
		}
		row.Cells = append(row.Cells, metricCell(r.rowValue, m))
		// The lbh per-row leader: the first strict maximum over the visible
		// columns (index 0 when all equal — a zero-only row highlights its
		// first column as BoldWhite(Dim(…))).
		if o.HighlightLeader && len(r.values) > 0 {
			leader := 0
			for i := 1; i < len(r.values); i++ {
				if r.values[i] > r.values[leader] {
					leader = i
				}
			}
			row.Cells[1+leader].Leader = true
		}
		if showBars {
			row.Bar = rowBar(r.rowValue, scale, r.values)
		}
		out = append(out, row)
	}
	return out
}

// pivotTotals appends the Total row and sets the Footer and Legend for a
// multi-label window: per-tool sums over the visible set, the grand total,
// and one swatch per visible tool when bars show and ≥ 2 tools are visible.
func pivotTotals(t *Table, labels []string, rowValues, toolSums []float64, grandTotal float64, series []Series, visible []int, o HistoryOptions, m Metric, showBars bool) {
	t.Rows = append(t.Rows, Row{Kind: Divider})
	total := Row{Kind: Total, Cells: make([]Cell, 0, len(visible)+2)}
	total.Cells = append(total.Cells, Cell{Text: "Total"})
	for _, sum := range toolSums {
		total.Cells = append(total.Cells, Cell{Text: fmtMetric(sum, m)})
	}
	total.Cells = append(total.Cells, Cell{Text: fmtMetric(grandTotal, m)})
	t.Rows = append(t.Rows, total)
	t.Footer = footerText(labels, rowValues, o.Period, t.Scale, o.Now, m)
	if showBars && len(visible) >= 2 {
		t.Legend = make([]Swatch, len(visible))
		for vi, si := range visible {
			t.Legend[vi] = Swatch{Name: series[si].Name, Palette: vi}
		}
	}
}

// LabelUnion is the sorted union of every series' labels (ascending byte
// order — the TS [...labelSet].sort()).
func LabelUnion(series []Series) []string {
	seen := make(map[string]bool)
	var labels []string
	for _, s := range series {
		for _, e := range s.Entries {
			if !seen[e.Label] {
				seen[e.Label] = true
				labels = append(labels, e.Label)
			}
		}
	}
	sort.Strings(labels)
	return labels
}

// significant selects the visible pivot columns (the TS significantTools):
// keep a series iff its total over the labels is ≥ negligibleAbs(metric) AND
// ≥ 0.001 × the grand total over ALL series, boundary values kept. An emptied
// set falls back to nonzeroTools, then to every series. Indices are in input
// (registry) order.
func significant(series []Series, valueMap []map[string]float64, labels []string, m Metric) []int {
	totals := make([]float64, len(series))
	grand := 0.0
	for i := range series {
		for _, label := range labels {
			totals[i] += valueMap[i][label]
		}
		grand += totals[i]
	}
	abs := negligibleAbs(m)
	var kept []int
	for i := range series {
		if totals[i] >= abs && totals[i] >= negligibleShare*grand {
			kept = append(kept, i)
		}
	}
	if len(kept) > 0 {
		return kept
	}
	return nonzero(series, valueMap, labels)
}

// nonzero falls back to series with any nonzero cell over the labels, then to
// every series when even that set is empty (the TS nonzeroTools).
func nonzero(series []Series, valueMap []map[string]float64, labels []string) []int {
	var active []int
	for i := range series {
		for _, label := range labels {
			if valueMap[i][label] != 0 {
				active = append(active, i)
				break
			}
		}
	}
	if len(active) > 0 {
		return active
	}
	all := make([]int, len(series))
	for i := range series {
		all[i] = i
	}
	return all
}
