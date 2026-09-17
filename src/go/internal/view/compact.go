package view

import (
	"github.com/sahil87/tu/internal/query"
)

// The compact table layout (watch mode on narrow terminals, the TS compact
// renderers): Name/Label 14-wide left, the value 12-wide right, the divider
// "─" × (14 + 1 + 12).
const (
	CompactNameWidth  = 14
	CompactValueWidth = 12
	CompactDivWidth   = CompactNameWidth + 1 + CompactValueWidth
)

// CompactRow is one compact-table line: the left Name/Label, the pre-formatted
// Value in the display metric, and the watch delta (rendered in-cell, padded
// with the value — the DeltaPadsArrow placement).
type CompactRow struct {
	Name  string
	Value string
	Delta Delta
}

// CompactTable is the render-agnostic compact table model (the TS compact
// renderers): two space-joined columns, no header row, no gutters, no bars,
// no machine columns and no legend. Total is nil unless more than one row is
// visible; Empty is the no-data line ("  No usage" / "  No data") — the empty
// check runs BEFORE the compact branch, as in the TS.
type CompactTable struct {
	Title string
	Rows  []CompactRow
	Total *CompactRow
	Empty string
}

// CompactSnapshot builds the compact cross-tool snapshot (the TS
// renderCompactSnapshot): one row per tool with TotalTokens > 0 carrying the
// metric value and the Prev delta keyed by name; the Total sums the metric
// over ALL input rows (visible or not) and appears only when more than one
// row is visible. Machine columns and the legend are dropped (the TS returns
// before them).
func CompactSnapshot(rows []ToolTotals, p query.Period, m Metric, prev map[string]float64) CompactTable {
	t := CompactTable{Title: "📊 Combined Usage (" + p.String() + ")"}

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

	grand := 0.0
	for _, r := range rows {
		v := metricValue(r.Totals, m)
		if r.TotalTokens > 0 {
			t.Rows = append(t.Rows, CompactRow{
				Name:  r.Name,
				Value: fmtMetric(v, m),
				Delta: rowDelta(prev, r.Name, v),
			})
		}
		grand += v
	}
	if visible > 1 {
		t.Total = &CompactRow{Name: "Total", Value: fmtMetric(grand, m)}
	}
	return t
}

// CompactHistory builds the compact single-tool history (the TS
// renderCompactHistory): the same title as the full table; the empty check
// runs first, then the MaxRows window (the TS truncates before the compact
// branch), then one row per entry keyed "{Name}:{label}" with the Total
// summing the visible (windowed) entries.
func CompactHistory(s Series, o HistoryOptions) CompactTable {
	t := CompactTable{Title: "📊 " + s.Name + " (" + PeriodLabel(o.Period, o.CapActive) + ")"}
	if len(s.Entries) == 0 {
		t.Empty = emptyHistory
		return t
	}
	entries := s.Entries
	if o.MaxRows > 0 && len(entries) > o.MaxRows {
		entries = entries[len(entries)-o.MaxRows:]
	}

	m := o.Metric
	sum := 0.0
	for _, e := range entries {
		v := metricValue(e.Totals, m)
		t.Rows = append(t.Rows, CompactRow{
			Name:  e.Label,
			Value: fmtMetric(v, m),
			Delta: rowDelta(o.Prev, s.Name+":"+e.Label, v),
		})
		sum += v
	}
	if len(entries) > 1 {
		t.Total = &CompactRow{Name: "Total", Value: fmtMetric(sum, m)}
	}
	return t
}

// CompactTotalHistory builds the compact cross-tool history (the TS
// renderCompactTotalHistory): one row per label of the sorted union carrying
// the sum over ALL series (an omitted tool still counts), keyed
// "total:{label}"; the MaxRows window applies to the labels BEFORE the empty
// check (the TS order); the Total is the grand over the windowed labels.
func CompactTotalHistory(series []Series, o HistoryOptions) CompactTable {
	m := o.Metric
	title := "📊 Combined Cost History"
	if m == Tokens {
		title = "📊 Combined Token History"
	}
	t := CompactTable{Title: title + " (" + PeriodLabel(o.Period, o.CapActive) + ")"}

	labels := LabelUnion(series)
	if o.MaxRows > 0 && len(labels) > o.MaxRows {
		labels = labels[len(labels)-o.MaxRows:]
	}
	if len(labels) == 0 {
		t.Empty = emptyHistory
		return t
	}

	valueMap := make(map[string]float64, len(labels))
	for _, s := range series {
		for _, e := range s.Entries {
			valueMap[e.Label] += metricValue(e.Totals, m)
		}
	}

	grand := 0.0
	for _, label := range labels {
		v := valueMap[label]
		t.Rows = append(t.Rows, CompactRow{
			Name:  label,
			Value: fmtMetric(v, m),
			Delta: rowDelta(o.Prev, "total:"+label, v),
		})
		grand += v
	}
	if len(labels) > 1 {
		t.Total = &CompactRow{Name: "Total", Value: fmtMetric(grand, m)}
	}
	return t
}
