package view

import (
	"testing"
	"time"

	"github.com/sahil87/tu/internal/query"
)

var historyNow = time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)

// placeholderSeries is the intake §12 single-tool window: three January days
// at $0.50 / 24,400 tokens each.
func placeholderSeries() Series {
	return Series{Name: "Claude Code", Entries: []Entry{
		{Label: "2026-01-05", Totals: dayTotals},
		{Label: "2026-01-06", Totals: dayTotals},
		{Label: "2026-01-07", Totals: dayTotals},
	}}
}

func dailyOpts(width int) HistoryOptions {
	return HistoryOptions{Period: query.Daily, Now: historyNow, Width: width}
}

func TestHistoryPlaceholderWindow(t *testing.T) {
	tab := History(placeholderSeries(), dailyOpts(80))

	if tab.Title != "📊 Claude Code (daily)" {
		t.Errorf("Title = %q", tab.Title)
	}
	if tab.Empty != "" {
		t.Errorf("Empty = %q, want empty", tab.Empty)
	}
	if !equalKinds(kinds(tab.Rows), Header, Divider, Data, Data, Data, Divider, Total) {
		t.Errorf("row kinds = %v", kinds(tab.Rows))
	}
	if !tab.DeltaSpaced {
		t.Error("DeltaSpaced = false, want true (the spaced single-tool form)")
	}
	if tab.Scale.Width != 0 {
		t.Errorf("Scale.Width = %d, want 0 (no bars at 80 columns)", tab.Scale.Width)
	}
	for _, r := range tab.Rows {
		if r.Bar != nil {
			t.Errorf("row %v carries a Bar with bars suppressed", r.Kind)
		}
		if r.Delta != DeltaNone {
			t.Errorf("row %v carries a Delta with Prev nil", r.Kind)
		}
	}

	total := tab.Rows[6]
	wantTotal := []string{"Total", "9,000", "1,200", "3,000", "60,000", "73,200", "$1.50"}
	for i, w := range wantTotal {
		if total.Cells[i].Text != w {
			t.Errorf("total cell %d = %q, want %q", i, total.Cells[i].Text, w)
		}
	}
	if tab.Footer != "avg $0.50/day · peak $0.50 (2026-01-05)" {
		t.Errorf("Footer = %q", tab.Footer)
	}

	// Column layout: Date 12, five 14-wide numerics, Cost at the 9 floor.
	wantWidths := []int{12, 14, 14, 14, 14, 14, 9}
	for i, w := range wantWidths {
		if tab.Columns[i].Width != w {
			t.Errorf("column %d width = %d, want %d", i, tab.Columns[i].Width, w)
		}
	}
	if tab.Columns[6].Title != "Cost" {
		t.Errorf("last column title = %q, want Cost", tab.Columns[6].Title)
	}
}

func TestHistoryCapHint(t *testing.T) {
	tab := History(Series{Name: "Claude Code"}, HistoryOptions{Period: query.Daily, Now: historyNow, Width: 80, CapActive: true})
	if tab.Title != "📊 Claude Code (daily, last 3 months)" {
		t.Errorf("Title = %q", tab.Title)
	}
	if tab.Empty != "  No data" {
		t.Errorf("Empty = %q, want %q", tab.Empty, "  No data")
	}
	if tab.Rows != nil {
		t.Errorf("Rows = %v, want nil in the empty state", tab.Rows)
	}
}

func TestHistorySingleRow(t *testing.T) {
	s := Series{Name: "Claude Code", Entries: []Entry{{Label: "2026-01", Totals: fact3x}}}
	tab := History(s, HistoryOptions{Period: query.Monthly, Now: historyNow, Width: 80})
	if !equalKinds(kinds(tab.Rows), Header, Divider, Data) {
		t.Errorf("row kinds = %v, want Header Divider Data (no Total for one row)", kinds(tab.Rows))
	}
	if tab.Footer != "" {
		t.Errorf("Footer = %q, want none for one row", tab.Footer)
	}
}

func TestHistoryMetricColumnGrowth(t *testing.T) {
	big := dayTotals
	big.TotalCost = 59634.40
	s := Series{Name: "Claude Code", Entries: []Entry{{Label: "2026-01-05", Totals: big}}}
	tab := History(s, dailyOpts(80))
	if tab.Columns[6].Width != 10 {
		t.Errorf("cost column width = %d, want 10 ($59,634.40)", tab.Columns[6].Width)
	}
}

func TestHistorySeparatorsDailyOnly(t *testing.T) {
	entries := []Entry{
		{Label: "2026-01-30", Totals: dayTotals},
		{Label: "2026-01-31", Totals: dayTotals},
		{Label: "2026-02-01", Totals: dayTotals},
		{Label: "2026-02-02", Totals: dayTotals},
	}
	tab := History(Series{Name: "Claude Code", Entries: entries}, dailyOpts(80))
	if !equalKinds(kinds(tab.Rows), Header, Divider, Data, Data, Separator, Data, Data, Divider, Total) {
		t.Errorf("row kinds = %v, want a Separator between the January and February rows only", kinds(tab.Rows))
	}

	// Weekly: same month boundary, no separator.
	weekly := []Entry{
		{Label: "2026-01-25", Totals: dayTotals},
		{Label: "2026-02-01", Totals: dayTotals},
	}
	tab = History(Series{Name: "Claude Code", Entries: weekly}, HistoryOptions{Period: query.Weekly, Now: historyNow, Width: 80})
	if !equalKinds(kinds(tab.Rows), Header, Divider, Data, Data, Divider, Total) {
		t.Errorf("weekly row kinds = %v, want no Separator", kinds(tab.Rows))
	}
}

func TestHistoryLabelStyles(t *testing.T) {
	// 2026-01-10 is a Saturday; inject Now so it is also "today" for the
	// Current-wins case.
	entries := []Entry{
		{Label: "2026-01-05", Totals: dayTotals}, // Monday
		{Label: "2026-01-10", Totals: dayTotals}, // Saturday
	}
	s := Series{Name: "Claude Code", Entries: entries}

	tab := History(s, dailyOpts(80))
	if tab.Rows[2].Cells[0].Style != Plain || tab.Rows[3].Cells[0].Style != Weekend {
		t.Errorf("styles = %v, %v; want Plain, Weekend", tab.Rows[2].Cells[0].Style, tab.Rows[3].Cells[0].Style)
	}

	saturdayNow := time.Date(2026, 1, 10, 9, 0, 0, 0, time.UTC)
	tab = History(s, HistoryOptions{Period: query.Daily, Now: saturdayNow, Width: 80})
	if tab.Rows[3].Cells[0].Style != Current {
		t.Errorf("weekend-today style = %v, want Current (the marker wins)", tab.Rows[3].Cells[0].Style)
	}
}

func TestHistoryTokenMode(t *testing.T) {
	tab := History(placeholderSeries(), HistoryOptions{Period: query.Daily, Now: historyNow, Width: 80, Metric: Tokens})
	if tab.Columns[6].Title != "Tokens" {
		t.Errorf("last column title = %q, want Tokens", tab.Columns[6].Title)
	}
	if got := tab.Rows[2].Cells[6].Text; got != "24,400" {
		t.Errorf("metric cell = %q, want 24,400", got)
	}
	total := tab.Rows[6]
	if got := total.Cells[6].Text; got != "73,200" {
		t.Errorf("total metric cell = %q, want 73,200", got)
	}
	// 73,200 is 6 chars — the column stays at the 9 floor.
	if tab.Columns[6].Width != 9 {
		t.Errorf("tokens column width = %d, want 9", tab.Columns[6].Width)
	}
	if tab.Footer != "avg 24,400/day · peak 24,400 (2026-01-05)" {
		t.Errorf("Footer = %q", tab.Footer)
	}
	// The title is metric-independent.
	if tab.Title != "📊 Claude Code (daily)" {
		t.Errorf("Title = %q", tab.Title)
	}
}

func TestHistoryDeltasFromPrev(t *testing.T) {
	prev := map[string]float64{
		"Claude Code:2026-01-05": 0.25, // up
		"Claude Code:2026-01-06": 0.75, // down
		// 2026-01-07 absent: none
	}
	o := dailyOpts(80)
	o.Prev = prev
	tab := History(placeholderSeries(), o)
	want := []Delta{DeltaUp, DeltaDown, DeltaNone}
	for i, w := range want {
		if tab.Rows[2+i].Delta != w {
			t.Errorf("row %d delta = %v, want %v", i, tab.Rows[2+i].Delta, w)
		}
	}
}

func TestHistoryBarBudget(t *testing.T) {
	// 80 columns: 80 − 97 − 3 − 9 − 1 = −30 → no bars.
	if tab := History(placeholderSeries(), dailyOpts(80)); tab.Scale.Width != 0 {
		t.Errorf("Scale.Width at 80 = %d, want 0", tab.Scale.Width)
	}
	// 140 columns: min(140 − 97 − 3 − 9 − 1, 30) = 30 → single-zone bars of 30.
	tab := History(placeholderSeries(), dailyOpts(140))
	if tab.Scale.Width != 30 || tab.Scale.TwoZone {
		t.Errorf("Scale at 140 = %+v, want single-zone width 30", tab.Scale)
	}
	for i := 2; i <= 4; i++ {
		bar := tab.Rows[i].Bar
		if bar == nil || bar.Segments != nil {
			t.Errorf("row %d bar = %+v, want a solid bar (Segments nil)", i, bar)
		}
	}
	// Equal values: every row is the max and fills the bar.
	if got := tab.Rows[2].Bar.Main; got != "██████████████████████████████" {
		t.Errorf("first row bar = %q, want 30 full blocks (0.5/0.5 × 30)", got)
	}
	// The footer carries no p95 term in single-zone mode.
	if tab.Footer != "avg $0.50/day · peak $0.50 (2026-01-05)" {
		t.Errorf("Footer = %q", tab.Footer)
	}
}

// fact3x is the monthly roll-up of the three placeholder days.
var fact3x = dayTotals.Add(dayTotals).Add(dayTotals)
