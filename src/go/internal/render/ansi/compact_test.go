package ansi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sahil87/tu/internal/fact"
	"github.com/sahil87/tu/internal/query"
	"github.com/sahil87/tu/internal/view"
)

var compactNow = time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)

func compactRows() []view.ToolTotals {
	return []view.ToolTotals{
		{Name: "Claude Code", Totals: fact.Totals{TotalCost: 5.38, TotalTokens: 24400}},
		{Name: "Kimi", Totals: fact.Totals{TotalCost: 27.25, TotalTokens: 48000}},
		{Name: "Codex"},
	}
}

func compactSeries() view.Series {
	return view.Series{Name: "Kimi", Entries: []view.Entry{
		{Label: "2026-09-14", Totals: fact.Totals{TotalCost: 1.25, TotalTokens: 1000}},
		{Label: "2026-09-15", Totals: fact.Totals{TotalCost: 2.50, TotalTokens: 2000}},
	}}
}

func compactPivot() []view.Series {
	return []view.Series{
		{Name: "Claude Code", Entries: []view.Entry{
			{Label: "2026-09-14", Totals: fact.Totals{TotalCost: 1.00, TotalTokens: 100}},
			{Label: "2026-09-15", Totals: fact.Totals{TotalCost: 2.00, TotalTokens: 200}},
		}},
		{Name: "Kimi", Entries: []view.Entry{
			{Label: "2026-09-15", Totals: fact.Totals{TotalCost: 4.00, TotalTokens: 400}},
		}},
	}
}

func TestCompactGoldens(t *testing.T) {
	color := Colors{Enabled: true}
	cases := []struct {
		name   string
		golden string
		lines  []string
	}{
		{"snapshot color", "compact_snapshot_color.golden", CompactTable(view.CompactSnapshot(compactRows(), query.Daily, view.Cost, nil), color)},
		{"snapshot no-color", "compact_snapshot_nocolor.golden", CompactTable(view.CompactSnapshot(compactRows(), query.Daily, view.Cost, nil), Colors{})},
		{"snapshot delta color", "compact_snapshot_delta_color.golden", CompactTable(view.CompactSnapshot(compactRows(), query.Daily, view.Cost, map[string]float64{"Claude Code": 5.00, "Kimi": 28.00}), color)},
		{"snapshot delta no-color", "compact_snapshot_delta_nocolor.golden", CompactTable(view.CompactSnapshot(compactRows(), query.Daily, view.Cost, map[string]float64{"Claude Code": 5.00, "Kimi": 28.00}), Colors{})},
		{"snapshot single row", "compact_snapshot_single.golden", CompactTable(view.CompactSnapshot(compactRows()[:1], query.Daily, view.Cost, nil), color)},
		{"snapshot tokens", "compact_snapshot_tokens.golden", CompactTable(view.CompactSnapshot(compactRows(), query.Daily, view.Tokens, map[string]float64{"Claude Code": 20000}), color)},
		{"snapshot empty", "compact_snapshot_empty.golden", CompactTable(view.CompactSnapshot(compactRows()[2:], query.Daily, view.Cost, nil), color)},
		{"history color", "compact_history_color.golden", CompactTable(view.CompactHistory(compactSeries(), view.HistoryOptions{Period: query.Daily, Now: compactNow}), color)},
		{"history delta no-color", "compact_history_delta_nocolor.golden", CompactTable(view.CompactHistory(compactSeries(), view.HistoryOptions{Period: query.Daily, Now: compactNow, Prev: map[string]float64{"Kimi:2026-09-15": 2.00}}), Colors{})},
		{"history single entry", "compact_history_single.golden", CompactTable(view.CompactHistory(view.Series{Name: "Kimi", Entries: compactSeries().Entries[:1]}, view.HistoryOptions{Period: query.Daily, Now: compactNow}), color)},
		{"total history color", "compact_total_history_color.golden", CompactTable(view.CompactTotalHistory(compactPivot(), view.HistoryOptions{Period: query.Daily, Now: compactNow}), color)},
		{"total history delta no-color", "compact_total_history_delta_nocolor.golden", CompactTable(view.CompactTotalHistory(compactPivot(), view.HistoryOptions{Period: query.Daily, Now: compactNow, Prev: map[string]float64{"total:2026-09-15": 7.00}}), Colors{})},
		{"total history tokens", "compact_total_history_tokens.golden", CompactTable(view.CompactTotalHistory(compactPivot(), view.HistoryOptions{Period: query.Daily, Now: compactNow, Metric: view.Tokens}), color)},
		{"total history empty", "compact_total_history_empty.golden", CompactTable(view.CompactTotalHistory(nil, view.HistoryOptions{Period: query.Daily, Now: compactNow}), color)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := strings.Join(c.lines, "\n") + "\n"
			path := filepath.Join("testdata", c.golden)
			if *update {
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden (run with -update to create): %v", err)
			}
			if got != string(raw) {
				t.Errorf("compact output differs from %s (-update to regenerate)\ngot:\n%q\nwant:\n%q", c.golden, got, string(raw))
			}
		})
	}
}
