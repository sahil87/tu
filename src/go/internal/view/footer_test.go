package view

import (
	"testing"
	"time"

	"github.com/sahil87/tu/internal/query"
)

var footerNow = time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)

func TestFooterText(t *testing.T) {
	labels := []string{"2026-01-05", "2026-01-06", "2026-01-07"}
	half := []float64{0.5, 0.5, 0.5}

	cases := []struct {
		name   string
		labels []string
		values []float64
		p      query.Period
		scale  Scale
		metric Metric
		want   string
	}{
		{
			name:   "daily window, no current-month rows",
			labels: labels, values: half, p: query.Daily,
			want: "avg $0.50/day · peak $0.50 (2026-01-05)",
		},
		{
			name:   "this month term when rows match the current month",
			labels: []string{"2026-09-01", "2026-09-02"}, values: []float64{1.25, 0.75}, p: query.Daily,
			want: "avg $1.00/day · this month $2.00 · peak $1.25 (2026-09-01)",
		},
		{
			name:   "all-zero window: peak without parentheses",
			labels: labels, values: []float64{0, 0, 0}, p: query.Daily,
			want: "avg $0.00/day · peak $0.00",
		},
		{
			name:   "weekly unit suffix",
			labels: []string{"2026-01-04", "2026-01-11"}, values: []float64{2, 4}, p: query.Weekly,
			want: "avg $3.00/week · peak $4.00 (2026-01-11)",
		},
		{
			name:   "monthly unit suffix",
			labels: []string{"2026-01", "2026-02"}, values: []float64{2, 4}, p: query.Monthly,
			want: "avg $3.00/month · peak $4.00 (2026-02)",
		},
		{
			name:   "two-zone appends the p95 term",
			labels: labels, values: half, p: query.Daily, scale: Scale{TwoZone: true, P95: 12.5, Max: 100},
			want: "avg $0.50/day · peak $0.50 (2026-01-05) · ┊ = $12.50 (p95)",
		},
		{
			name:   "token mode formats integers without a unit prefix",
			labels: labels, values: []float64{24400, 24400, 24400}, p: query.Daily, metric: Tokens,
			want: "avg 24,400/day · peak 24,400 (2026-01-05)",
		},
		{
			name:   "peak is the FIRST strict maximum",
			labels: labels, values: []float64{0.5, 0.5, 0.25}, p: query.Daily,
			want: "avg $0.42/day · peak $0.50 (2026-01-05)",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := footerText(c.labels, c.values, c.p, c.scale, footerNow, c.metric); got != c.want {
				t.Errorf("footerText = %q, want %q", got, c.want)
			}
		})
	}
}

func TestLabelStyle(t *testing.T) {
	// 2026-01-05 is a Monday; 2026-01-10 a Saturday; Now is 2026-09-16.
	now := footerNow
	cases := []struct {
		name  string
		label string
		p     query.Period
		now   time.Time
		want  LabelStyle
	}{
		{"plain weekday", "2026-01-05", query.Daily, now, Plain},
		{"weekend", "2026-01-10", query.Daily, now, Weekend},
		{"current wins over weekend", "2026-01-10", query.Daily, time.Date(2026, 1, 10, 9, 0, 0, 0, time.UTC), Current},
		{"current monthly", "2026-09", query.Monthly, now, Current},
		{"weekend only on daily", "2026-01-10", query.Weekly, now, Plain},
		{"malformed label is plain", "garbage", query.Daily, now, Plain},
	}
	for _, c := range cases {
		if got := labelStyle(c.label, c.p, c.now); got != c.want {
			t.Errorf("%s: labelStyle(%q) = %v, want %v", c.name, c.label, got, c.want)
		}
	}
}

func TestMetricCellDimExactZero(t *testing.T) {
	if c := metricCell(0, Cost); !c.Dim || c.Text != "$0.00" {
		t.Errorf("zero cell = %+v, want dim $0.00", c)
	}
	if c := metricCell(0.004, Cost); c.Dim {
		t.Errorf("sub-cent cell = %+v, must NOT be dimmed (exact zero only)", c)
	}
	if c := metricCell(0, Tokens); !c.Dim || c.Text != "0" {
		t.Errorf("zero token cell = %+v, want dim 0", c)
	}
}

func TestMetricColumnWidth(t *testing.T) {
	if got := metricColumnWidth([]float64{0.5, 1.5}, Cost); got != 9 {
		t.Errorf("width = %d, want the floor 9", got)
	}
	// $59,634.40 is 10 chars — the column grows past the floor.
	if got := metricColumnWidth([]float64{59634.40}, Cost); got != 10 {
		t.Errorf("width = %d, want 10", got)
	}
}
