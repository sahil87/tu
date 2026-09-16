package view

import (
	"testing"

	"github.com/sahil87/tu/internal/fact"
)

var toolNames = []string{"Claude Code", "Codex", "OpenCode", "Gemini", "Copilot", "Kimi"}

// sixPlaceholderSeries is the intake §12 pivot window: six tools, three
// January days at $0.50 / 24,400 tokens each.
func sixPlaceholderSeries() []Series {
	series := make([]Series, len(toolNames))
	for i, name := range toolNames {
		series[i] = Series{Name: name, Entries: []Entry{
			{Label: "2026-01-05", Totals: dayTotals},
			{Label: "2026-01-06", Totals: dayTotals},
			{Label: "2026-01-07", Totals: dayTotals},
		}}
	}
	return series
}

func TestTotalHistoryPlaceholderWindow(t *testing.T) {
	tab := TotalHistory(sixPlaceholderSeries(), dailyOpts(80))

	if tab.Title != "📊 Combined Cost History (daily)" {
		t.Errorf("Title = %q", tab.Title)
	}
	if tab.DeltaSpaced {
		t.Error("DeltaSpaced = true, want false (the space-less pivot form)")
	}
	if !equalKinds(kinds(tab.Rows), Header, Divider, Data, Data, Data, Divider, Total) {
		t.Errorf("row kinds = %v", kinds(tab.Rows))
	}

	// Widths: Date 10, Claude Code 11, the rest 9, Cost 9 → table width 84.
	wantWidths := []int{10, 11, 9, 9, 9, 9, 9, 9}
	for i, w := range wantWidths {
		if tab.Columns[i].Width != w {
			t.Errorf("column %d width = %d, want %d", i, tab.Columns[i].Width, w)
		}
	}
	if tab.Scale.Width != 0 {
		t.Errorf("Scale.Width = %d, want 0 (80 − 84 − 3 − 9 − 1 = −17 → no bars)", tab.Scale.Width)
	}
	if tab.Legend != nil {
		t.Errorf("Legend = %v, want nil without bars", tab.Legend)
	}

	data := tab.Rows[2]
	for i, w := range []string{"2026-01-05", "$0.50", "$0.50", "$0.50", "$0.50", "$0.50", "$0.50", "$3.00"} {
		if data.Cells[i].Text != w {
			t.Errorf("data cell %d = %q, want %q", i, data.Cells[i].Text, w)
		}
	}
	total := tab.Rows[6]
	wantTotal := []string{"Total", "$1.50", "$1.50", "$1.50", "$1.50", "$1.50", "$1.50", "$9.00"}
	for i, w := range wantTotal {
		if total.Cells[i].Text != w {
			t.Errorf("total cell %d = %q, want %q", i, total.Cells[i].Text, w)
		}
	}
	if tab.Footer != "avg $3.00/day · peak $3.00 (2026-01-05)" {
		t.Errorf("Footer = %q", tab.Footer)
	}
}

func TestTotalHistoryOmissionCountsEverywhere(t *testing.T) {
	// Copilot totals $0.04 (< $1.00) and Kimi $0.00 among real spend: both
	// columns drop, but their cost still counts in every rowValue, the Total
	// and the footer.
	series := sixPlaceholderSeries()
	for i := range series {
		switch series[i].Name {
		case "Copilot":
			series[i].Entries = []Entry{{Label: "2026-01-05", Totals: fact.Totals{TotalCost: 0.04}}}
		case "Kimi":
			series[i].Entries = nil
		}
	}
	tab := TotalHistory(series, dailyOpts(80))

	wantTitles := []string{"Date", "Claude Code", "Codex", "OpenCode", "Gemini", "Cost"}
	if len(tab.Columns) != len(wantTitles) {
		t.Fatalf("columns = %d, want %d (Copilot and Kimi omitted)", len(tab.Columns), len(wantTitles))
	}
	for i, w := range wantTitles {
		if tab.Columns[i].Title != w {
			t.Errorf("column %d title = %q, want %q", i, tab.Columns[i].Title, w)
		}
	}
	// Row values sum ALL series: 4 × $0.50 + $0.04 on the first day.
	if got := tab.Rows[2].Cells[5].Text; got != "$2.04" {
		t.Errorf("first rowValue = %q, want $2.04 (omitted tools count)", got)
	}
	if got := tab.Rows[3].Cells[5].Text; got != "$2.00" {
		t.Errorf("second rowValue = %q, want $2.00", got)
	}
	// The Total's last cell is the grand total over all series: 6.04.
	total := tab.Rows[len(tab.Rows)-1]
	if got := total.Cells[5].Text; got != "$6.04" {
		t.Errorf("grand total cell = %q, want $6.04", got)
	}
	if tab.Footer != "avg $2.01/day · peak $2.04 (2026-01-05)" {
		t.Errorf("Footer = %q", tab.Footer)
	}
}

func TestTotalHistorySignificanceFallbacks(t *testing.T) {
	day := func(cost float64) []Entry {
		return []Entry{{Label: "2026-01-05", Totals: fact.Totals{TotalCost: cost}}}
	}

	t.Run("boundary values kept", func(t *testing.T) {
		// $1.00 exactly and a share of exactly 0.1% of the grand total.
		series := []Series{
			{Name: "Claude Code", Entries: day(999.0)},
			{Name: "Codex", Entries: day(1.0)},
		}
		tab := TotalHistory(series, dailyOpts(80))
		if len(tab.Columns) != 4 || tab.Columns[1].Title != "Claude Code" || tab.Columns[2].Title != "Codex" {
			t.Errorf("columns = %v, want both tools kept at the boundary", tab.Columns)
		}
	})

	t.Run("below the floor drops", func(t *testing.T) {
		series := []Series{
			{Name: "Claude Code", Entries: day(100.0)},
			{Name: "Codex", Entries: day(0.99)},
		}
		tab := TotalHistory(series, dailyOpts(80))
		if len(tab.Columns) != 3 {
			t.Errorf("columns = %d, want Codex dropped below $1.00", len(tab.Columns))
		}
		// Its cost still counts in the row value.
		if got := tab.Rows[2].Cells[2].Text; got != "$100.99" {
			t.Errorf("rowValue = %q, want $100.99", got)
		}
	})

	t.Run("$0.40 window falls back to nonzero", func(t *testing.T) {
		series := []Series{
			{Name: "Claude Code", Entries: day(0.40)},
			{Name: "Codex"},
		}
		tab := TotalHistory(series, dailyOpts(80))
		if len(tab.Columns) != 3 || tab.Columns[1].Title != "Claude Code" {
			t.Errorf("columns = %v, want only the nonzero tool", tab.Columns)
		}
	})

	t.Run("all-zero falls back to every series", func(t *testing.T) {
		series := []Series{{Name: "Claude Code", Entries: day(0)}, {Name: "Codex", Entries: day(0)}}
		tab := TotalHistory(series, dailyOpts(80))
		if len(tab.Columns) != 4 { // Date + two tools + Cost
			t.Errorf("columns = %d, want every series kept", len(tab.Columns))
		}
	})
}

func TestTotalHistoryBarsAndLegend(t *testing.T) {
	o := dailyOpts(120) // 120 − 84 − 3 − 9 − 1 = 23 ≥ 10 → bars
	tab := TotalHistory(sixPlaceholderSeries(), o)
	if tab.Scale.Width != 23 {
		t.Errorf("Scale.Width = %d, want 23", tab.Scale.Width)
	}
	for i := 2; i <= 4; i++ {
		bar := tab.Rows[i].Bar
		if bar == nil {
			t.Fatalf("row %d has no bar", i)
		}
		// Equal shares over the 23-glyph bar: floor 3 each, the remainder 5
		// handed left to right on the fractional tie.
		want := []int{4, 4, 4, 4, 4, 3}
		if len(bar.Segments) != 6 {
			t.Fatalf("row %d segments = %v", i, bar.Segments)
		}
		for j, w := range want {
			if bar.Segments[j] != w {
				t.Errorf("row %d segment %d = %d, want %d", i, j, bar.Segments[j], w)
			}
		}
	}
	if len(tab.Legend) != 6 {
		t.Fatalf("Legend = %v, want one swatch per visible tool", tab.Legend)
	}
	for i, s := range tab.Legend {
		if s.Name != toolNames[i] || s.Palette != i {
			t.Errorf("legend %d = %+v, want {%s %d}", i, s, toolNames[i], i)
		}
	}
}

func TestTotalHistoryIndicatorReserve(t *testing.T) {
	// At width 107 the budget without a reserve is exactly 10 (bars); the
	// Prev map's 1-char reserve suppresses them.
	without := TotalHistory(sixPlaceholderSeries(), dailyOpts(107))
	if without.Scale.Width != 10 {
		t.Fatalf("Scale.Width without Prev = %d, want 10", without.Scale.Width)
	}
	o := dailyOpts(107)
	o.Prev = map[string]float64{"total:2026-01-05": 1.0}
	with := TotalHistory(sixPlaceholderSeries(), o)
	if with.Scale.Width != 0 {
		t.Errorf("Scale.Width with Prev = %d, want 0 (indicatorReserve = 1)", with.Scale.Width)
	}
	if with.Rows[2].Delta != DeltaUp {
		t.Errorf("first row delta = %v, want DeltaUp from Prev", with.Rows[2].Delta)
	}
}

func TestTotalHistoryTokenMode(t *testing.T) {
	o := dailyOpts(80)
	o.Metric = Tokens
	tab := TotalHistory(sixPlaceholderSeries(), o)
	if tab.Title != "📊 Combined Token History (daily)" {
		t.Errorf("Title = %q", tab.Title)
	}
	if got := tab.Columns[7].Title; got != "Tokens" {
		t.Errorf("last column title = %q, want Tokens", got)
	}
	if got := tab.Rows[2].Cells[1].Text; got != "24,400" {
		t.Errorf("cell = %q, want 24,400", got)
	}
	if tab.Footer != "avg 146,400/day · peak 146,400 (2026-01-05)" {
		t.Errorf("Footer = %q", tab.Footer)
	}
}

func TestTotalHistoryTokenSignificance(t *testing.T) {
	// Under tokens the floor is 1,000: a 24,400-token $0.00 tool is a real
	// column in token mode and dropped in cost mode.
	series := []Series{
		{Name: "Claude Code", Entries: []Entry{{Label: "2026-01-05", Totals: dayTotals}}},
		{Name: "Kimi", Entries: []Entry{{Label: "2026-01-05", Totals: fact.Totals{TotalTokens: 500}}}},
	}
	o := dailyOpts(80)
	o.Metric = Tokens
	tab := TotalHistory(series, o)
	if len(tab.Columns) != 3 {
		t.Errorf("columns = %d, want Kimi dropped below 1,000 tokens", len(tab.Columns))
	}
	if got := tab.Rows[2].Cells[2].Text; got != "24,900" {
		t.Errorf("rowValue = %q, want 24,900 (dropped column still counts)", got)
	}
}

func TestTotalHistoryEmptyAndSingleRow(t *testing.T) {
	series := []Series{{Name: "Claude Code"}, {Name: "Codex"}}
	tab := TotalHistory(series, dailyOpts(80))
	if tab.Empty != "  No data" || tab.Rows != nil {
		t.Errorf("empty pivot = %+v, want Empty + no rows", tab)
	}

	one := sixPlaceholderSeries()
	for i := range one {
		one[i].Entries = one[i].Entries[:1]
	}
	tab = TotalHistory(one, dailyOpts(80))
	if !equalKinds(kinds(tab.Rows), Header, Divider, Data) {
		t.Errorf("row kinds = %v, want Header Divider Data (no Total for one label)", kinds(tab.Rows))
	}
	if tab.Footer != "" {
		t.Errorf("Footer = %q, want none for one label", tab.Footer)
	}
}

func TestTotalHistorySortedLabelUnion(t *testing.T) {
	dollar := fact.Totals{TotalCost: 1.0, TotalTokens: 1000}
	twoDollar := fact.Totals{TotalCost: 2.0, TotalTokens: 2000}
	series := []Series{
		{Name: "Claude Code", Entries: []Entry{{Label: "2026-01-07", Totals: dollar}, {Label: "2026-01-05", Totals: dollar}}},
		{Name: "Codex", Entries: []Entry{{Label: "2026-01-06", Totals: twoDollar}}},
	}
	tab := TotalHistory(series, dailyOpts(80))
	var got []string
	for _, r := range tab.Rows {
		if r.Kind == Data {
			got = append(got, r.Cells[0].Text)
		}
	}
	want := []string{"2026-01-05", "2026-01-06", "2026-01-07"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("label %d = %q, want %q (sorted union)", i, got[i], want[i])
		}
	}
	// Codex is $0.00 on the two days it has no entries.
	if c := tab.Rows[2].Cells[2]; !c.Dim || c.Text != "$0.00" {
		t.Errorf("missing-day cell = %+v, want dim $0.00", c)
	}
}
