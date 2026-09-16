package view

import (
	"strconv"
	"testing"

	"github.com/sahil87/tu/internal/fact"
	"github.com/sahil87/tu/internal/query"
)

// lbRow builds a leaderboard row with the given cost/tokens (share/delta set
// by the caller when they matter).
func lbRow(rank int, user string, cost float64, tokens int64) LeaderboardRow {
	return LeaderboardRow{
		Rank:   rank,
		User:   user,
		Totals: fact.Totals{TotalCost: cost, TotalTokens: tokens},
	}
}

func lbOpts() LeaderboardOptions {
	return LeaderboardOptions{
		Period:      query.Monthly,
		WindowLabel: "2026-09",
		DeltaLabel:  "Aug",
		Metric:      Cost,
		PinnedUser:  "sahil",
		Width:       80,
		LastSync:    "never",
	}
}

// R6: the title has no 📊 and names the metric; the footer is the staleness
// line; a nil delta renders "new".
func TestLeaderboardBasics(t *testing.T) {
	sahil := lbRow(1, "sahil", 12945.64, 15962442751)
	sahil.Share = 0.795
	rows := []LeaderboardRow{
		sahil,
		lbRow(2, "beatriz", 3333.40, 5562910079),
	}
	tab := Leaderboard(rows, lbOpts())
	if tab.Title != "Leaderboard (monthly) · 2026-09 · by cost" {
		t.Errorf("Title = %q", tab.Title)
	}
	if tab.Footer != "never synced · tu sync to refresh" {
		t.Errorf("Footer = %q", tab.Footer)
	}
	o := lbOpts()
	o.LastSync = "15m ago (2026-09-15T18:54:44.502Z)"
	tab = Leaderboard(rows, o)
	if tab.Footer != "synced 15m ago (2026-09-15T18:54:44.502Z) · tu sync to refresh" {
		t.Errorf("Footer = %q", tab.Footer)
	}

	// Columns and the mid-row bar marker.
	wantTitles := []string{"#", "User", "Cost", "Tokens", "Share", "Δ vs Aug"}
	if len(tab.Columns) != len(wantTitles) {
		t.Fatalf("columns = %+v", tab.Columns)
	}
	for i, w := range wantTitles {
		if tab.Columns[i].Title != w {
			t.Errorf("column %d = %q, want %q", i, tab.Columns[i].Title, w)
		}
	}
	if !tab.Columns[2].BarAfter {
		t.Error("the Cost column must carry BarAfter")
	}
	for i, c := range tab.Columns {
		if i != 2 && c.BarAfter {
			t.Errorf("column %d unexpectedly BarAfter", i)
		}
	}

	// Data cells: pin marker, share toFixed(1), delta "new".
	data := tab.Rows[2]
	if data.Cells[1].Text != "sahil ◂" {
		t.Errorf("name cell = %q, want the pin marker", data.Cells[1].Text)
	}
	if data.Cells[4].Text != "79.5%" {
		t.Errorf("share cell = %q", data.Cells[4].Text)
	}
	if data.Cells[5].Text != "new" {
		t.Errorf("delta cell = %q, want new", data.Cells[5].Text)
	}
	if tab.Rows[3].Cells[1].Text != "beatriz" {
		t.Errorf("unpinned row = %q", tab.Rows[3].Cells[1].Text)
	}
}

// R6: the Total row covers ALL rows (also under --top); the User width counts
// the collapsed label; the collapsed row is dim with blank cells and no bar.
func TestLeaderboardTopCollapse(t *testing.T) {
	rows := []LeaderboardRow{
		lbRow(1, "sahil", 12945.64, 100),
		lbRow(2, "beatriz", 3333.40, 100),
		lbRow(3, "carlos", 1598.54, 100),
		lbRow(4, "dominic", 611.44, 100),
		lbRow(5, "eunice", 155.60, 100),
		lbRow(6, "frankie", 116.47, 100),
	}
	o := lbOpts()
	o.Top = 2
	tab := Leaderboard(rows, o)

	// header, divider, 2 data, collapsed, divider, total
	if len(tab.Rows) != 7 {
		t.Fatalf("rows = %d, want 7: %+v", len(tab.Rows), tab.Rows)
	}
	coll := tab.Rows[4]
	if coll.Cells[1].Text != "… +4 others" || !coll.Cells[1].Dim {
		t.Errorf("collapsed row = %+v", coll.Cells[1])
	}
	if coll.Bar != nil {
		t.Error("the collapsed row carries no bar")
	}
	for _, i := range []int{0, 2, 3, 4, 5} {
		if coll.Cells[i].Text != "" {
			t.Errorf("collapsed cell %d = %q, want blank", i, coll.Cells[i].Text)
		}
	}
	// The collapsed label widens the User column past every visible name
	// (rune-counted: "… +4 others" is 11 runes).
	if tab.Columns[1].Width != 11 {
		t.Errorf("User width = %d, want 11 (the collapsed label)", tab.Columns[1].Width)
	}
	total := tab.Rows[6]
	if total.Kind != Total || total.Cells[2].Text != "$18,761.09" {
		t.Errorf("Total row = %+v", total)
	}

	// Top ≥ len(rows) collapses nothing.
	o.Top = 6
	tab = Leaderboard(rows, o)
	for _, r := range tab.Rows {
		if r.Kind == Data && len(r.Cells) > 1 && r.Cells[1].Dim {
			t.Errorf("Top ≥ len must not collapse: %+v", r.Cells[1])
		}
	}
}

// R6: the rank column flips to width 2 at ten visible rows.
func TestLeaderboardRankWidth(t *testing.T) {
	var rows []LeaderboardRow
	for i := 1; i <= 10; i++ {
		rows = append(rows, lbRow(i, "u"+strconv.Itoa(i), float64(100-i), 1))
	}
	tab := Leaderboard(rows, lbOpts())
	if tab.Columns[0].Width != 2 {
		t.Errorf("rank width = %d, want 2 at 10 rows", tab.Columns[0].Width)
	}
	tab = Leaderboard(rows[:9], lbOpts())
	if tab.Columns[0].Width != 1 {
		t.Errorf("rank width = %d, want 1 at 9 rows", tab.Columns[0].Width)
	}
}

// R6: the empty state is title, "  No data", footer; zero rows never crash.
func TestLeaderboardEmpty(t *testing.T) {
	tab := Leaderboard(nil, lbOpts())
	if tab.Empty != "  No data" {
		t.Errorf("Empty = %q", tab.Empty)
	}
	if len(tab.Columns) != 0 || len(tab.Rows) != 0 {
		t.Errorf("empty table must carry no columns/rows: %+v", tab)
	}
	if tab.Footer != "never synced · tu sync to refresh" {
		t.Errorf("Footer = %q", tab.Footer)
	}
}

// R6: share/delta cell edge values — 100.0%, +0%, -55%, +4757%; a zero-cost
// row dims its Cost cell; the dim rule keys on the exact zero.
func TestLeaderboardCells(t *testing.T) {
	d := func(v float64) *float64 { return &v }
	rows := []LeaderboardRow{
		{Rank: 1, User: "a", Totals: fact.Totals{TotalCost: 100, TotalTokens: 5}, Share: 1.0, Delta: d(0)},
		{Rank: 2, User: "b", Totals: fact.Totals{TotalCost: 0, TotalTokens: 5}, Share: 0, Delta: d(-0.5532817482507824)},
		{Rank: 3, User: "c", Totals: fact.Totals{TotalCost: 0.004, TotalTokens: 0}, Share: 0.000025, Delta: d(47.57)},
	}
	tab := Leaderboard(rows, lbOpts())
	cells := func(row int) []Cell { return tab.Rows[2+row].Cells }
	if got := cells(0)[4].Text; got != "100.0%" {
		t.Errorf("share = %q, want 100.0%%", got)
	}
	if got := cells(0)[5].Text; got != "+0%" {
		t.Errorf("delta = %q, want +0%%", got)
	}
	if got := cells(1)[5].Text; got != "-55%" {
		t.Errorf("delta = %q, want -55%%", got)
	}
	if got := cells(2)[5].Text; got != "+4757%" {
		t.Errorf("delta = %q, want +4757%%", got)
	}
	if !cells(1)[2].Dim {
		t.Error("zero-cost cell must be dim")
	}
	if cells(2)[2].Dim {
		t.Error("a sub-cent cost is NOT dim (the TS tests === 0)")
	}
	if !cells(2)[3].Dim {
		t.Error("zero-token cell must be dim")
	}
}

// R6: a single row has no Total row; the bar budget engages at 80 columns
// for a narrow table (25 = 80 − 42 body − 3 gutter − 9 cost − 1 leading) and
// saturates at the 30 cap with a wide budget.
func TestLeaderboardSingleRowAndBars(t *testing.T) {
	tab := Leaderboard([]LeaderboardRow{lbRow(1, "sahil", 100, 24400)}, lbOpts())
	for _, r := range tab.Rows {
		if r.Kind == Total {
			t.Error("a single row has no Total")
		}
	}
	if tab.Scale.Width != 25 {
		t.Errorf("bar width = %d, want 25 from the budget", tab.Scale.Width)
	}
	if tab.Rows[2].Bar == nil || tab.Rows[2].Bar.Main == "" {
		t.Errorf("the row must carry a bar: %+v", tab.Rows[2].Bar)
	}

	o := lbOpts()
	o.Width = 120
	tab = Leaderboard([]LeaderboardRow{
		lbRow(1, "sahil", 12945.64, 100),
		lbRow(2, "beatriz", 3333.40, 100),
	}, o)
	if tab.Scale.Width != 30 {
		t.Errorf("Width 120 bar width = %d, want 30 (the cap)", tab.Scale.Width)
	}
	if tab.Rows[2].Bar == nil || tab.Rows[2].Bar.Main == "" {
		t.Errorf("the top row must carry a bar: %+v", tab.Rows[2].Bar)
	}
}

// R6: --by-machine joins the key in the name cell and pins every machine row
// of the pinned user.
func TestLeaderboardByMachineNames(t *testing.T) {
	rows := []LeaderboardRow{
		{Rank: 1, User: "sahil", Machine: "dev-ws-sahil02", Totals: fact.Totals{TotalCost: 9340.16, TotalTokens: 1}},
		{Rank: 2, User: "beatriz", Machine: "dev-ws-beatri01", Totals: fact.Totals{TotalCost: 3206.02, TotalTokens: 1}},
		{Rank: 3, User: "sahil", Machine: "dev-ws-sahil01", Totals: fact.Totals{TotalCost: 2963.66, TotalTokens: 1}},
	}
	tab := Leaderboard(rows, lbOpts())
	names := []string{tab.Rows[2].Cells[1].Text, tab.Rows[3].Cells[1].Text, tab.Rows[4].Cells[1].Text}
	want := []string{"sahil/dev-ws-sahil02 ◂", "beatriz/dev-ws-beatri01", "sahil/dev-ws-sahil01 ◂"}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("name %d = %q, want %q", i, names[i], want[i])
		}
	}
}
