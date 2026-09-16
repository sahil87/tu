package view

import (
	"testing"

	"github.com/sahil87/tu/internal/fact"
	"github.com/sahil87/tu/internal/query"
)

var dayTotals = fact.Totals{TotalCost: 0.5, InputTokens: 3000, OutputTokens: 400, CacheCreationTokens: 1000, CacheReadTokens: 20000, TotalTokens: 24400}

func kinds(rows []Row) []RowKind {
	out := make([]RowKind, len(rows))
	for i, r := range rows {
		out[i] = r.Kind
	}
	return out
}

func equalKinds(got []RowKind, want ...RowKind) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestSnapshotPopulated(t *testing.T) {
	rows := []ToolTotals{
		{Name: "Claude Code", Label: "2026-09-16", Totals: dayTotals},
		{Name: "Codex", Label: "2026-09-16", Totals: dayTotals},
		{Name: "OpenCode"}, // all-zero: omitted from rows, counted in Total
	}
	tab := Snapshot(rows, query.Daily)

	if tab.Title != "📊 Combined Usage (daily)" {
		t.Errorf("Title = %q", tab.Title)
	}
	if tab.Empty != "" {
		t.Errorf("Empty = %q, want empty", tab.Empty)
	}
	if !equalKinds(kinds(tab.Rows), Header, Divider, Data, Data, Divider, Total) {
		t.Errorf("row kinds = %v", kinds(tab.Rows))
	}
	data := tab.Rows[2].Cells
	want := []string{"Claude Code", "24,400", "3,000", "400", "21,000", "$0.50"}
	for i, w := range want {
		if data[i].Text != w {
			t.Errorf("data cell %d = %q, want %q", i, data[i].Text, w)
		}
	}
	total := tab.Rows[5].Cells
	wantTotal := []string{"Total", "48,800", "6,000", "800", "42,000", "$1.00"}
	for i, w := range wantTotal {
		if total[i].Text != w {
			t.Errorf("total cell %d = %q, want %q", i, total[i].Text, w)
		}
	}
}

func TestSnapshotHiddenCostCounted(t *testing.T) {
	hidden := fact.Totals{TotalCost: 0.25} // zero tokens: hidden, still summed
	rows := []ToolTotals{
		{Name: "Claude Code", Totals: dayTotals},
		{Name: "Codex", Totals: dayTotals},
		{Name: "OpenCode", Totals: hidden},
	}
	tab := Snapshot(rows, query.Daily)
	total := tab.Rows[len(tab.Rows)-1]
	if total.Kind != Total {
		t.Fatalf("last row kind = %v, want Total", total.Kind)
	}
	if got := total.Cells[5].Text; got != "$1.25" {
		t.Errorf("Total cost = %q, want $1.25 (hidden row counted)", got)
	}
}

func TestSnapshotSingleRowNoTotal(t *testing.T) {
	rows := []ToolTotals{
		{Name: "Claude Code", Totals: dayTotals},
		{Name: "Codex"},
	}
	tab := Snapshot(rows, query.Daily)
	if !equalKinds(kinds(tab.Rows), Header, Divider, Data) {
		t.Errorf("row kinds = %v, want Header Divider Data (Total only when >1 visible)", kinds(tab.Rows))
	}
}

func TestSnapshotEmpty(t *testing.T) {
	tab := Snapshot([]ToolTotals{{Name: "Claude Code"}, {Name: "Codex"}}, query.Daily)
	if tab.Empty != "  No usage" {
		t.Errorf("Empty = %q, want %q", tab.Empty, "  No usage")
	}
	if tab.Rows != nil {
		t.Errorf("Rows = %v, want nil in the empty state", tab.Rows)
	}
}

func TestSnapshotHeadingPerPeriod(t *testing.T) {
	for p, want := range map[query.Period]string{
		query.Daily:   "📊 Combined Usage (daily)",
		query.Weekly:  "📊 Combined Usage (weekly)",
		query.Monthly: "📊 Combined Usage (monthly)",
	} {
		if got := Snapshot(nil, p).Title; got != want {
			t.Errorf("Snapshot(nil, %v).Title = %q, want %q", p, got, want)
		}
	}
}

func TestSnapshotFixedWidthsOverflow(t *testing.T) {
	for _, c := range snapshotColumns {
		if c.Width != 12 {
			t.Errorf("column %q width = %d, want 12 (fixed, never data-sized)", c.Title, c.Width)
		}
	}
	// A wider value is carried verbatim — it overflows the cell at render time.
	big := fact.Totals{TotalCost: 1, TotalTokens: 1234567890123}
	tab := Snapshot([]ToolTotals{{Name: "Claude Code", Totals: big}}, query.Daily)
	if got := tab.Rows[2].Cells[1].Text; got != "1,234,567,890,123" {
		t.Errorf("overflowing cell = %q", got)
	}
}
