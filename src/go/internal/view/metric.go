package view

import (
	"math"
	"time"

	"github.com/sahil87/tu/internal/fact"
	"github.com/sahil87/tu/internal/query"
	"github.com/sahil87/tu/internal/render"
)

// metricFloor is the floor for every data-sized right-aligned metric column
// (the TS COST_WIDTH / MIN_TOOL_COL_WIDTH — 9 fits "$9,999.99").
const metricFloor = 9

// fmtMetric formats a cell/bar/footer value in the table's unit: the ICU
// $-cost under Cost, the grouped rounded integer under Tokens (the TS
// fmtMetric: fmtNum(Math.round(n))).
func fmtMetric(v float64, m Metric) string {
	if m == Tokens {
		return render.FormatInt(int64(math.Round(v)))
	}
	return render.FormatCost(v)
}

// metricValue is the Totals field a cell, bar or footer stat renders in the
// selected metric (the TS metricValue).
func metricValue(t fact.Totals, m Metric) float64 {
	if m == Tokens {
		return float64(t.TotalTokens)
	}
	return t.TotalCost
}

// metricColumnWidth sizes a right-aligned metric column to its data:
// max(metricFloor, longest fmtMetric among the values the column will hold —
// including its Total-row value, which callers append).
func metricColumnWidth(values []float64, m Metric) int {
	w := metricFloor
	for _, v := range values {
		if n := len(fmtMetric(v, m)); n > w {
			w = n
		}
	}
	return w
}

// metricCell is a metric data cell: Dim iff the value is exactly 0 (a
// sub-cent value that formats as "$0.00" is NOT dimmed — the TS tests
// `=== 0`, never the formatted string). Header/Total cells never go through
// here. Padding and color are the encoder's (pad first, then Dim).
func metricCell(v float64, m Metric) Cell {
	return Cell{Text: fmtMetric(v, m), Dim: v == 0}
}

// metricHeader is the last column's title in the selected metric.
func metricHeader(m Metric) string {
	if m == Tokens {
		return "Tokens"
	}
	return "Cost"
}

// labelStyle marks a Data row's Date cell: Current when the label is the
// current period's label (boldWhite), else Weekend on a daily Saturday or
// Sunday (dim), else Plain. The marker wins on a weekend today — one cell,
// one style.
func labelStyle(label string, p query.Period, now time.Time) LabelStyle {
	if label == query.CurrentLabel(p, now) {
		return Current
	}
	if p == query.Daily && isWeekend(label) {
		return Weekend
	}
	return Plain
}

// isWeekend reports whether a daily ISO label falls on a Saturday or Sunday.
// The label parses as UTC midnight (the TS `new Date(label).getUTCDay()`), so
// the weekday is timezone-independent; a malformed label is never a weekend.
func isWeekend(label string) bool {
	d, err := time.Parse("2006-01-02", label)
	if err != nil {
		return false
	}
	wd := d.Weekday()
	return wd == time.Saturday || wd == time.Sunday
}
