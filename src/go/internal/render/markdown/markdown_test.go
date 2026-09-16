package markdown

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sahil87/tu/internal/fact"
	"github.com/sahil87/tu/internal/query"
	"github.com/sahil87/tu/internal/view"
)

// update regenerates the golden files: go test ./internal/render/markdown/ -update
var update = flag.Bool("update", false, "regenerate golden files")

var dayTotals = fact.Totals{TotalCost: 0.5, InputTokens: 3000, OutputTokens: 400, CacheCreationTokens: 1000, CacheReadTokens: 20000, TotalTokens: 24400}

// monthlySeries is the intake §12 cc-mh window: the three placeholder days
// rolled up to 2026-01.
var monthlySeries = view.Series{Name: "Claude Code", Entries: []view.Entry{
	{Label: "2026-01", Totals: dayTotals.Add(dayTotals).Add(dayTotals)},
}}

func sixSeries() []view.Series {
	names := []string{"Claude Code", "Codex", "OpenCode", "Gemini", "Copilot", "Kimi"}
	series := make([]view.Series, len(names))
	for i, n := range names {
		series[i] = view.Series{Name: n, Entries: []view.Entry{
			{Label: "2026-01-05", Totals: dayTotals},
			{Label: "2026-01-06", Totals: dayTotals},
			{Label: "2026-01-07", Totals: dayTotals},
		}}
	}
	return series
}

func sixEmptySeries() []view.Series {
	names := []string{"Claude Code", "Codex", "OpenCode", "Gemini", "Copilot", "Kimi"}
	series := make([]view.Series, len(names))
	for i, n := range names {
		series[i] = view.Series{Name: n}
	}
	return series
}

func snapshotRows() []view.ToolTotals {
	return []view.ToolTotals{
		{Name: "Claude Code", Totals: dayTotals},
		{Name: "Codex", Totals: dayTotals},
		{Name: "OpenCode"},
		{Name: "Gemini"},
		{Name: "Copilot"},
		{Name: "Kimi"},
	}
}

func TestGoldens(t *testing.T) {
	cases := []struct {
		name   string
		golden string
		lines  []string
	}{
		{"snapshot populated", "snapshot_populated.golden", Snapshot(snapshotRows(), query.Daily, nil)},
		{"snapshot empty", "snapshot_empty.golden", Snapshot(nil, query.Daily, nil)},
		{"history populated", "history_populated.golden", History(monthlySeries, query.Monthly, false, nil)},
		{"history with total", "history_with_total.golden", History(view.Series{Name: "Claude Code", Entries: []view.Entry{
			{Label: "2026-01-05", Totals: dayTotals},
			{Label: "2026-01-06", Totals: dayTotals},
		}}, query.Daily, false, nil)},
		{"history empty cap", "history_empty_cap.golden", History(view.Series{Name: "Claude Code"}, query.Daily, true, nil)},
		// R11: name-headed machine columns (verbatim names, ---: alignment,
		// FormatCost cells, bold Total sums).
		{"snapshot machines", "snapshot_machines.golden", Snapshot(snapshotRows(), query.Daily, &view.Breakdown{Noun: "Machines", Rows: map[string][]view.Slice{
			"Claude Code": {
				{Name: "dev-ws-sahil02", Totals: fact.Totals{TotalCost: 0.2}},
				{Name: "Sahils-Mac-mini.local", Totals: fact.Totals{TotalCost: 0.3}},
			},
		}})},
		{"history machines", "history_machines.golden", History(view.Series{Name: "Claude Code", Entries: []view.Entry{
			{Label: "2026-01-05", Totals: dayTotals},
			{Label: "2026-01-06", Totals: dayTotals},
		}}, query.Daily, false, &view.Breakdown{Noun: "Machines", Rows: map[string][]view.Slice{
			"2026-01-05": {{Name: "harness-machine", Totals: dayTotals}},
			"2026-01-06": {
				{Name: "harness-machine", Totals: fact.Totals{TotalCost: 0.75}},
				{Name: "other-box", Totals: fact.Totals{TotalCost: 0.40}},
			},
		}})},
		{"total history populated", "total_history_populated.golden", TotalHistory(sixSeries(), query.Daily, false)},
		{"total history empty cap", "total_history_empty_cap.golden", TotalHistory(sixEmptySeries(), query.Daily, true)},
		{"total history omission", "total_history_omission.golden", TotalHistory([]view.Series{
			{Name: "Claude Code", Entries: []view.Entry{{Label: "2026-01-05", Totals: dayTotals}, {Label: "2026-01-06", Totals: dayTotals}}},
			{Name: "Codex", Entries: []view.Entry{{Label: "2026-01-05", Totals: fact.Totals{TotalCost: 0.004}}}},
			{Name: "Kimi"},
		}, query.Daily, false)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := strings.Join(c.lines, "\n") + "\n"
			path := filepath.Join("testdata", c.golden)
			if *update {
				if err := os.MkdirAll("testdata", 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden (run with -update to create): %v", err)
			}
			if got != string(raw) {
				t.Errorf("Markdown output differs from %s (-update to regenerate)\ngot:\n%q\nwant:\n%q", c.golden, got, string(raw))
			}
		})
	}
}

// The intake §12 captures pin the Markdown shapes byte for byte, including
// the trailing blank line.
func TestIntakeByteReferences(t *testing.T) {
	got := strings.Join(History(monthlySeries, query.Monthly, false, nil), "\n") + "\n"
	want := "## Claude Code (monthly)\n" +
		"\n" +
		"| Date | Input | Output | Cache Write | Cache Read | Total | Cost |\n" +
		"| :--- | ---: | ---: | ---: | ---: | ---: | ---: |\n" +
		"| 2026-01 | 9,000 | 1,200 | 3,000 | 60,000 | 73,200 | $1.50 |\n" +
		"\n"
	if got != want {
		t.Errorf("cc-mh-md bytes =\n%q\nwant:\n%q", got, want)
	}

	got = strings.Join(TotalHistory(sixEmptySeries(), query.Daily, true), "\n") + "\n"
	want = "## Combined Cost History (daily, last 3 months)\n" +
		"\n" +
		"| Date | Claude Code | Codex | OpenCode | Gemini | Copilot | Kimi | Cost |\n" +
		"| :--- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |\n" +
		"\n"
	if got != want {
		t.Errorf("h-md bytes =\n%q\nwant:\n%q", got, want)
	}

	got = strings.Join(Snapshot(nil, query.Daily, nil), "\n") + "\n"
	want = "## Combined Usage (daily)\n" +
		"\n" +
		"| Tool | Tokens | Input | Output | Cache | Cost |\n" +
		"| :--- | ---: | ---: | ---: | ---: | ---: |\n" +
		"\n"
	if got != want {
		t.Errorf("empty snapshot md =\n%q\nwant:\n%q", got, want)
	}
}

func TestTotalHistoryExactZeroOmission(t *testing.T) {
	series := []view.Series{
		{Name: "Claude Code", Entries: []view.Entry{{Label: "2026-01-05", Totals: dayTotals}}},
		{Name: "Codex", Entries: []view.Entry{{Label: "2026-01-05", Totals: fact.Totals{TotalCost: 0.004}}}},
		{Name: "Kimi"},
	}
	lines := TotalHistory(series, query.Daily, false)
	// Codex is NOT exact-zero ($0.004 → $0.00): the exact-zero rule keeps it
	// (Markdown has no significance floor); Kimi drops.
	header := lines[2]
	if header != "| Date | Claude Code | Codex | Cost |" {
		t.Errorf("header = %q, want Kimi dropped, Codex kept", header)
	}
	row := lines[4]
	if row != "| 2026-01-05 | $0.50 | $0.00 | $0.50 |" {
		t.Errorf("row = %q", row)
	}

	// All-zero: every column stays.
	lines = TotalHistory(sixEmptySeries(), query.Daily, false)
	if lines[2] != "| Date | Claude Code | Codex | OpenCode | Gemini | Copilot | Kimi | Cost |" {
		t.Errorf("all-zero header = %q, want all six tools", lines[2])
	}
}

func TestHistoryTotalRowGate(t *testing.T) {
	one := History(monthlySeries, query.Monthly, false, nil)
	for _, l := range one {
		if strings.Contains(l, "**Total**") {
			t.Errorf("single-entry history must not carry a Total row: %q", l)
		}
	}
	two := History(view.Series{Name: "Claude Code", Entries: []view.Entry{
		{Label: "2026-01-05", Totals: dayTotals},
		{Label: "2026-01-06", Totals: dayTotals},
	}}, query.Daily, false, nil)
	found := false
	for _, l := range two {
		if strings.HasPrefix(l, "| **Total** |") {
			found = true
		}
	}
	if !found {
		t.Errorf("two-entry history must carry the bold Total row:\n%s", strings.Join(two, "\n"))
	}
}
