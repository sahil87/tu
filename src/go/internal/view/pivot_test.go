package view

import (
	"fmt"
	"testing"

	"github.com/sahil87/tu/internal/fact"
	"github.com/sahil87/tu/internal/query"
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

// ── B5: the lbh hooks ───────────────────────────────────────────────────────

// lbhSeries mirrors the R8 scenario: four users (input order eunice, bob,
// alice, sahil), window totals 767.50 / 615.70 / 389.05 / 0 for
// sahil/alice/bob/eunice, plus an all-zero first row.
func lbhSeries() []Series {
	cost := func(c float64) fact.Totals { return fact.Totals{TotalCost: c, TotalTokens: 1} }
	return []Series{
		{Name: "eunice", Entries: []Entry{{Label: "2026-06", Totals: cost(0)}, {Label: "2026-07", Totals: cost(0)}, {Label: "2026-08", Totals: cost(0)}}},
		{Name: "bob", Entries: []Entry{{Label: "2026-06", Totals: cost(0)}, {Label: "2026-07", Totals: cost(169.05)}, {Label: "2026-08", Totals: cost(220.00)}}},
		{Name: "alice", Entries: []Entry{{Label: "2026-06", Totals: cost(0)}, {Label: "2026-07", Totals: cost(314.60)}, {Label: "2026-08", Totals: cost(301.10)}}},
		{Name: "sahil", Entries: []Entry{{Label: "2026-06", Totals: cost(0)}, {Label: "2026-07", Totals: cost(355.20)}, {Label: "2026-08", Totals: cost(412.30)}}},
	}
}

func lbhOpts(width int) HistoryOptions {
	o := HistoryOptions{Period: query.Monthly, Now: historyNow, Width: width}
	o.Title = "📊 Leaderboard History (monthly)"
	o.RankColumns = true
	o.HighlightLeader = true
	o.KeepAllColumns = true
	return o
}

// R8: columns rank by descending window total (ties keep first-seen order),
// an all-zero column stays and dims, and each row's max cell gets Leader.
func TestTotalHistoryLbhHooks(t *testing.T) {
	tab := TotalHistory(lbhSeries(), lbhOpts(80))

	if tab.Title != "📊 Leaderboard History (monthly)" {
		t.Errorf("Title = %q", tab.Title)
	}
	wantCols := []string{"Date", "sahil", "alice", "bob", "eunice", "Cost"}
	if len(tab.Columns) != len(wantCols) {
		t.Fatalf("columns = %+v", tab.Columns)
	}
	for i, w := range wantCols {
		if tab.Columns[i].Title != w {
			t.Errorf("column %d = %q, want %q (ranked, all kept)", i, tab.Columns[i].Title, w)
		}
	}

	// Rows: header, divider, 3 data, divider, total.
	if !equalKinds(kinds(tab.Rows), Header, Divider, Data, Data, Data, Divider, Total) {
		t.Errorf("row kinds = %v", kinds(tab.Rows))
	}
	// The all-zero first row highlights its FIRST post-reorder column
	// (sahil) as Leader, and the cell stays dim.
	first := tab.Rows[2]
	for i, c := range first.Cells {
		leader := i == 1
		if c.Leader != leader {
			t.Errorf("all-zero row cell %d Leader = %v, want %v", i, c.Leader, leader)
		}
		if i >= 1 && i <= 4 && !c.Dim {
			t.Errorf("all-zero row cell %d = %+v, want dim", i, c)
		}
	}
	// Row 2026-07: sahil 355.20 leads; eunice is dim $0.00.
	jul := tab.Rows[3]
	if jul.Cells[1].Text != "$355.20" || !jul.Cells[1].Leader {
		t.Errorf("leader cell = %+v, want $355.20 Leader", jul.Cells[1])
	}
	for i := 2; i <= 4; i++ {
		if jul.Cells[i].Leader {
			t.Errorf("cell %d unexpectedly Leader: %+v", i, jul.Cells[i])
		}
	}
	if jul.Cells[4].Text != "$0.00" || !jul.Cells[4].Dim {
		t.Errorf("eunice cell = %+v, want dim $0.00", jul.Cells[4])
	}
	// The Total row follows the reordered columns; the row total sums all.
	total := tab.Rows[6]
	for i, w := range []string{"Total", "$767.50", "$615.70", "$389.05", "$0.00", "$1,772.25"} {
		if total.Cells[i].Text != w {
			t.Errorf("total cell %d = %q, want %q", i, total.Cells[i].Text, w)
		}
	}
}

// R8: ties on the window total keep first-seen order.
func TestTotalHistoryRankColumnsTies(t *testing.T) {
	cost := func(c float64) fact.Totals { return fact.Totals{TotalCost: c, TotalTokens: 1} }
	series := []Series{
		{Name: "zeta", Entries: []Entry{{Label: "2026-01-05", Totals: cost(2)}}},
		{Name: "alpha", Entries: []Entry{{Label: "2026-01-05", Totals: cost(3)}}},
		{Name: "mid", Entries: []Entry{{Label: "2026-01-05", Totals: cost(2)}}},
	}
	tab := TotalHistory(series, lbhOpts(80))
	want := []string{"Date", "alpha", "zeta", "mid", "Cost"}
	for i, w := range want {
		if tab.Columns[i].Title != w {
			t.Errorf("column %d = %q, want %q", i, tab.Columns[i].Title, w)
		}
	}
}

// R8: the zero-value hooks reproduce the tool pivot's defaults.
func TestTotalHistoryHookDefaultsUnchanged(t *testing.T) {
	a := TotalHistory(sixPlaceholderSeries(), dailyOpts(80))
	b := TotalHistory(sixPlaceholderSeries(), HistoryOptions{
		Period: dailyOpts(80).Period, Now: dailyOpts(80).Now, Width: 80, Metric: Cost,
	})
	// Column titles and every cell must match today's output.
	for i := range a.Columns {
		if a.Columns[i] != b.Columns[i] {
			t.Errorf("column %d differs: %+v vs %+v", i, a.Columns[i], b.Columns[i])
		}
	}
	for i := range a.Rows {
		if a.Rows[i].Kind != b.Rows[i].Kind || len(a.Rows[i].Cells) != len(b.Rows[i].Cells) {
			t.Fatalf("row %d shape differs", i)
		}
		for j := range a.Rows[i].Cells {
			if a.Rows[i].Cells[j] != b.Rows[i].Cells[j] {
				t.Errorf("row %d cell %d differs: %+v vs %+v", i, j, a.Rows[i].Cells[j], b.Rows[i].Cells[j])
			}
		}
	}
}

// MaxRows (B7): the window truncation runs before the empty check, and the
// significance/omission pass sees only the windowed labels — a tool
// significant over the full range but zero inside the window drops out.
func TestTotalHistoryMaxRowsWindowScoping(t *testing.T) {
	series := []Series{
		{Name: "Claude Code", Entries: []Entry{
			{Label: "2026-01-01", Totals: fact.Totals{TotalCost: 100}},
			{Label: "2026-01-02", Totals: fact.Totals{}},
		}},
		{Name: "Kimi", Entries: []Entry{
			{Label: "2026-01-01", Totals: fact.Totals{}},
			{Label: "2026-01-02", Totals: fact.Totals{TotalCost: 2}},
		}},
	}
	full := TotalHistory(series, HistoryOptions{Period: query.Daily, Now: historyNow})
	// Both tools are significant over the full range (Claude $100, Kimi $2 of
	// the $102 grand): Date + two tools + Cost.
	if len(full.Columns) != 4 {
		t.Errorf("full columns = %d, want 4", len(full.Columns))
	}
	windowed := TotalHistory(series, HistoryOptions{Period: query.Daily, Now: historyNow, MaxRows: 1})
	// The window is {2026-01-02}: Claude Code is zero there, so only Kimi's
	// column survives (Date + Kimi + Cost).
	if len(windowed.Columns) != 3 || windowed.Columns[1].Title != "Kimi" {
		t.Errorf("windowed columns = %+v", windowed.Columns)
	}
	data := 0
	var totalRow *Row
	for i := range windowed.Rows {
		if windowed.Rows[i].Kind == Data {
			data++
		}
		if windowed.Rows[i].Kind == Total {
			totalRow = &windowed.Rows[i]
		}
	}
	if data != 1 {
		t.Errorf("data rows = %d, want 1 (the single windowed label)", data)
	}
	if totalRow != nil {
		t.Errorf("Total row present for a one-label window: %+v", totalRow)
	}
}

// MaxRows 0 leaves the tables untouched (the one-shot path stays
// byte-identical — every existing golden proves it; this pins the field's
// zero value semantics directly).
func TestHistoryMaxRowsZeroIsUnlimited(t *testing.T) {
	entries := make([]Entry, 20)
	for i := range entries {
		entries[i] = Entry{Label: fmt.Sprintf("2026-08-%02d", i+1), Totals: fact.Totals{TotalCost: 1}}
	}
	s := Series{Name: "Kimi", Entries: entries}
	full := History(s, HistoryOptions{Period: query.Daily, Now: historyNow}, nil)
	if len(full.Rows) != 2+20+2 { // header+divider, 20 data, divider+total
		t.Errorf("full rows = %d, want 24", len(full.Rows))
	}
	w := History(s, HistoryOptions{Period: query.Daily, Now: historyNow, MaxRows: 15}, nil)
	if len(w.Rows) != 2+15+2 {
		t.Errorf("windowed rows = %d, want 19", len(w.Rows))
	}
	if got := w.Rows[2].Cells[0].Text; got != "2026-08-06" {
		t.Errorf("first windowed label = %q, want 2026-08-06", got)
	}
}
