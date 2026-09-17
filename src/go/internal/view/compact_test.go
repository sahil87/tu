package view

import (
	"fmt"
	"testing"
	"time"

	"github.com/sahil87/tu/internal/fact"
	"github.com/sahil87/tu/internal/query"
)

func TestCompactSnapshot(t *testing.T) {
	rows := []ToolTotals{
		{Name: "Claude Code", Totals: ctTotals(0.50, 24400)},
		{Name: "Codex", Totals: ctTotals(0.75, 24400)},
		{Name: "Kimi"}, // zero tokens: hidden, counted in Total
	}
	t.Run("rows and total", func(t *testing.T) {
		ct := CompactSnapshot(rows, query.Daily, Cost, nil)
		if ct.Title != "📊 Combined Usage (daily)" {
			t.Errorf("Title = %q", ct.Title)
		}
		if len(ct.Rows) != 2 || ct.Rows[0].Name != "Claude Code" || ct.Rows[0].Value != "$0.50" {
			t.Errorf("Rows = %+v", ct.Rows)
		}
		if ct.Total == nil || ct.Total.Value != "$1.25" {
			t.Errorf("Total = %+v", ct.Total)
		}
	})
	t.Run("empty check precedes compact", func(t *testing.T) {
		ct := CompactSnapshot([]ToolTotals{{Name: "Kimi"}}, query.Daily, Cost, nil)
		if ct.Empty != "  No usage" || ct.Rows != nil || ct.Total != nil {
			t.Errorf("empty = %+v", ct)
		}
	})
	t.Run("single row no total", func(t *testing.T) {
		ct := CompactSnapshot(rows[:1], query.Daily, Cost, nil)
		if ct.Total != nil || len(ct.Rows) != 1 {
			t.Errorf("Total = %+v, Rows = %+v", ct.Total, ct.Rows)
		}
	})
	t.Run("token mode and delta", func(t *testing.T) {
		ct := CompactSnapshot(rows, query.Daily, Tokens, map[string]float64{"Claude Code": 20000, "Codex": 30000})
		if ct.Rows[0].Value != "24,400" || ct.Rows[0].Delta != DeltaUp {
			t.Errorf("row 0 = %+v", ct.Rows[0])
		}
		if ct.Rows[1].Delta != DeltaDown {
			t.Errorf("row 1 delta = %v, want DeltaDown", ct.Rows[1].Delta)
		}
		if ct.Total.Value != "48,800" {
			t.Errorf("Total = %q", ct.Total.Value)
		}
	})
	t.Run("hidden row counted in total", func(t *testing.T) {
		withHidden := append([]ToolTotals{}, rows...)
		withHidden[2].TotalCost = 0.05 // zero tokens, nonzero cost: hidden yet summed
		ct := CompactSnapshot(withHidden, query.Daily, Cost, nil)
		if ct.Total.Value != "$1.30" {
			t.Errorf("Total = %q, want $1.30", ct.Total.Value)
		}
	})
}

func TestCompactHistory(t *testing.T) {
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	entries := make([]Entry, 20)
	for i := range entries {
		entries[i] = Entry{Label: fmt.Sprintf("2026-08-%02d", i+1), Totals: ctTotals(float64(i+1), 1000)}
	}
	s := Series{Name: "Kimi", Entries: entries}
	t.Run("max rows window scopes rows and total", func(t *testing.T) {
		ct := CompactHistory(s, HistoryOptions{Period: query.Daily, Now: now, MaxRows: 15})
		if len(ct.Rows) != 15 || ct.Rows[0].Name != "2026-08-06" || ct.Rows[14].Name != "2026-08-20" {
			t.Fatalf("window = %+v…", ct.Rows[:1])
		}
		// The Total sums the visible window: 6+7+…+20 = 195.
		if ct.Total == nil || ct.Total.Value != "$195.00" {
			t.Errorf("Total = %+v, want $195.00 over the window", ct.Total)
		}
	})
	t.Run("delta keyed name:label", func(t *testing.T) {
		ct := CompactHistory(s, HistoryOptions{Period: query.Daily, Now: now, MaxRows: 15, Prev: map[string]float64{"Kimi:2026-08-20": 19}})
		if ct.Rows[14].Delta != DeltaUp {
			t.Errorf("last row delta = %v, want DeltaUp", ct.Rows[14].Delta)
		}
		if ct.Rows[0].Delta != DeltaNone {
			t.Errorf("first row delta = %v, want DeltaNone (no prev entry)", ct.Rows[0].Delta)
		}
	})
	t.Run("empty", func(t *testing.T) {
		ct := CompactHistory(Series{Name: "Kimi"}, HistoryOptions{Period: query.Daily, Now: now})
		if ct.Empty != "  No data" || ct.Rows != nil {
			t.Errorf("empty = %+v", ct)
		}
	})
	t.Run("single entry no total", func(t *testing.T) {
		ct := CompactHistory(Series{Name: "Kimi", Entries: entries[:1]}, HistoryOptions{Period: query.Daily, Now: now})
		if ct.Total != nil || len(ct.Rows) != 1 {
			t.Errorf("Total = %+v, Rows = %+v", ct.Total, ct.Rows)
		}
	})
}

func TestCompactTotalHistory(t *testing.T) {
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	series := []Series{
		{Name: "Claude Code", Entries: []Entry{
			{Label: "2026-08-01", Totals: ctTotals(1, 100)},
			{Label: "2026-08-02", Totals: ctTotals(2, 100)},
		}},
		{Name: "Codex", Entries: []Entry{
			{Label: "2026-08-02", Totals: ctTotals(3, 100)},
			{Label: "2026-08-03", Totals: ctTotals(4, 100)},
		}},
	}
	t.Run("labels union, rows sum all series", func(t *testing.T) {
		ct := CompactTotalHistory(series, HistoryOptions{Period: query.Daily, Now: now})
		if ct.Title != "📊 Combined Cost History (daily)" {
			t.Errorf("Title = %q", ct.Title)
		}
		if len(ct.Rows) != 3 || ct.Rows[0].Value != "$1.00" || ct.Rows[1].Value != "$5.00" || ct.Rows[2].Value != "$4.00" {
			t.Errorf("Rows = %+v", ct.Rows)
		}
		if ct.Total == nil || ct.Total.Value != "$10.00" {
			t.Errorf("Total = %+v", ct.Total)
		}
	})
	t.Run("max rows truncates labels before the empty check", func(t *testing.T) {
		ct := CompactTotalHistory(series, HistoryOptions{Period: query.Daily, Now: now, MaxRows: 2})
		if len(ct.Rows) != 2 || ct.Rows[0].Name != "2026-08-02" {
			t.Errorf("Rows = %+v", ct.Rows)
		}
		if ct.Total.Value != "$9.00" {
			t.Errorf("Total = %q, want $9.00 over the window", ct.Total.Value)
		}
	})
	t.Run("token title and total:label delta", func(t *testing.T) {
		ct := CompactTotalHistory(series, HistoryOptions{Period: query.Daily, Now: now, Metric: Tokens, Prev: map[string]float64{"total:2026-08-02": 600}})
		if ct.Title != "📊 Combined Token History (daily)" {
			t.Errorf("Title = %q", ct.Title)
		}
		if ct.Rows[1].Value != "200" || ct.Rows[1].Delta != DeltaDown {
			t.Errorf("row 1 = %+v", ct.Rows[1])
		}
	})
	t.Run("empty", func(t *testing.T) {
		ct := CompactTotalHistory(nil, HistoryOptions{Period: query.Daily, Now: now})
		if ct.Empty != "  No data" || ct.Rows != nil {
			t.Errorf("empty = %+v", ct)
		}
	})
}

// ctTotals builds the fixture Totals (cost, total tokens) for the compact tests.
func ctTotals(cost float64, tokens int64) fact.Totals {
	return fact.Totals{TotalCost: cost, TotalTokens: tokens}
}
