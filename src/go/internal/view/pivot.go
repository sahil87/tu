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
// renderTotalHistory with the tool defaults — the lbh hooks are B5):
//
//   - Title "📊 Combined Cost History ({period}[, last 3 months])", or "Token"
//     under tokens.
//   - Labels are the sorted union of every series' labels; none →
//     Empty = "  No data".
//   - Visible tools by the significance rule (≥ negligibleAbs AND ≥ 0.001 ×
//     grand, boundary kept), falling back to nonzero, then to all; rowValue
//     and grandTotal sum over ALL series (an omitted column still counts).
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
	t := Table{Title: title + " (" + PeriodLabel(o.Period, o.CapActive) + ")"}

	labels := LabelUnion(series)
	if len(labels) == 0 {
		t.Empty = emptyHistory
		return t
	}

	// One lookup: tool → label → the displayed value. Cells, the row column,
	// the Total row, bars, segments and the footer all render in the metric.
	valueMap := make([]map[string]float64, len(series))
	for i, s := range series {
		values := make(map[string]float64, len(s.Entries))
		for _, e := range s.Entries {
			values[e.Label] = metricValue(e.Totals, m)
		}
		valueMap[i] = values
	}

	visible := significant(series, valueMap, labels, m)

	// Pre-compute per-row value data before the width budget: rowValue and
	// grandTotal sum over ALL series; values/toolSums cover the visible set.
	type rowData struct {
		label    string
		values   []float64
		rowValue float64
	}
	rows := make([]rowData, len(labels))
	toolSums := make([]float64, len(visible))
	grandTotal := 0.0
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
		rows[li] = rowData{label, values, rowValue}
	}

	// Per-tool column width: max(name, floor, longest cell including its
	// Total-row sum); the row column is data-sized over every row value plus
	// the grand total.
	toolWidths := make([]int, len(visible))
	for vi, si := range visible {
		w := max(len(series[si].Name), metricFloor)
		for _, r := range rows {
			w = max(w, len(fmtMetric(r.values[vi], m)))
		}
		toolWidths[vi] = max(w, len(fmtMetric(toolSums[vi], m)))
	}
	rowValues := make([]float64, len(rows))
	for i, r := range rows {
		rowValues[i] = r.rowValue
	}
	costWidth := metricColumnWidth(append(rowValues, grandTotal), m)

	tableWidth := pivotDateWidth
	for _, w := range toolWidths {
		tableWidth += w + gutterWidth
	}
	indicatorReserve := 0
	if o.Prev != nil {
		indicatorReserve = 1
	}
	barWidth := min(o.Width-tableWidth-gutterWidth-costWidth-barLeading-indicatorReserve, maxBarWidth)
	showBars := barWidth >= minBarArea
	if showBars {
		t.Scale = ComputeScale(rowValues, barWidth)
	}

	t.Columns = make([]Column, 0, len(visible)+2)
	t.Columns = append(t.Columns, Column{Title: "Date", Width: pivotDateWidth, Align: Left})
	for vi, si := range visible {
		t.Columns = append(t.Columns, Column{Title: series[si].Name, Width: toolWidths[vi], Align: Right})
	}
	t.Columns = append(t.Columns, Column{Title: metricHeader(m), Width: costWidth, Align: Right})
	t.Rows = append(t.Rows, headerRow(t.Columns), Row{Kind: Divider})

	prevMonthPrefix := ""
	for _, r := range rows {
		monthPrefix := monthPrefixOf(r.label)
		if o.Period == query.Daily && prevMonthPrefix != "" && monthPrefix != prevMonthPrefix {
			t.Rows = append(t.Rows, Row{Kind: Separator})
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
		if showBars {
			row.Bar = rowBar(r.rowValue, t.Scale, r.values)
		}
		t.Rows = append(t.Rows, row)
	}

	if len(labels) > 1 {
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
	return t
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
