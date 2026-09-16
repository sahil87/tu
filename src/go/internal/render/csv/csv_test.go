package csv

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sahil87/tu/internal/fact"
	"github.com/sahil87/tu/internal/view"
)

// update regenerates the golden files: go test ./internal/render/csv/ -update
var update = flag.Bool("update", false, "regenerate golden files")

var dayTotals = fact.Totals{TotalCost: 0.5, InputTokens: 3000, OutputTokens: 400, CacheCreationTokens: 1000, CacheReadTokens: 20000, TotalTokens: 24400}

// monthlySeries is the intake §12 cc-mh window: the three placeholder days
// rolled up to 2026-01.
var monthlySeries = view.Series{Name: "Claude Code", Entries: []view.Entry{
	{Label: "2026-01", Totals: dayTotals.Add(dayTotals).Add(dayTotals)},
}}

func dailySeries(name string) view.Series {
	return view.Series{Name: name, Entries: []view.Entry{
		{Label: "2026-01-05", Totals: dayTotals},
		{Label: "2026-01-06", Totals: dayTotals},
		{Label: "2026-01-07", Totals: dayTotals},
	}}
}

func sixSeries() []view.Series {
	names := []string{"Claude Code", "Codex", "OpenCode", "Gemini", "Copilot", "Kimi"}
	series := make([]view.Series, len(names))
	for i, n := range names {
		series[i] = dailySeries(n)
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

func TestCostRounding(t *testing.T) {
	// The thirteen node-verified values (intake §6): toFixed(2) rounds the
	// EXACT binary value half-up — neither FormatFloat('f', 2) nor
	// render.FormatCost agrees on these.
	cases := []struct {
		in   float64
		want string
	}{
		{1.005, "1.00"},
		{0.125, "0.13"},
		{0.375, "0.38"},
		{2.675, "2.67"},
		{0.015, "0.01"},
		{1.045, "1.04"},
		{8.345, "8.35"},
		{0.005, "0.01"},
		{0.045, "0.04"},
		{999999.995, "999999.99"},
		{1234567.891, "1234567.89"},
		{0.5, "0.50"},
		{0, "0.00"},
	}
	for _, c := range cases {
		if got := Cost(c.in); got != c.want {
			t.Errorf("Cost(%v) = %q, want %q", c.in, got, c.want)
		}
	}
	if got := Cost(0); got != "0.00" {
		t.Errorf("Cost(0) = %q", got)
	}
	if got := Cost(negativeZero()); got != "0.00" {
		t.Errorf("Cost(-0) = %q, want 0.00", got)
	}
}

func negativeZero() float64 {
	f := 0.0
	return -f
}

func TestQuote(t *testing.T) {
	cases := []struct{ in, want string }{
		{"plain", "plain"},
		{"with,comma", `"with,comma"`},
		{`with"quote`, `"with""quote"`},
		{"with\nnewline", "\"with\nnewline\""},
		{"with\rcr", "\"with\rcr\""},
	}
	for _, c := range cases {
		if got := quote(c.in); got != c.want {
			t.Errorf("quote(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	// The rule threads through every field, headers included.
	s := view.Series{Name: "Tool, \"One\"", Entries: []view.Entry{{Label: "2026-01", Totals: dayTotals}}}
	got := TotalHistory([]view.Series{s})[0]
	if got != `date,"Tool, ""One""",total` {
		t.Errorf("quoted header = %q", got)
	}
}

func TestGoldens(t *testing.T) {
	empty := []view.Series{
		{Name: "Claude Code"}, {Name: "Codex"}, {Name: "OpenCode"},
		{Name: "Gemini"}, {Name: "Copilot"}, {Name: "Kimi"},
	}
	cases := []struct {
		name   string
		golden string
		lines  []string
	}{
		{"snapshot populated", "snapshot_populated.golden", Snapshot(snapshotRows(), nil)},
		{"snapshot empty", "snapshot_empty.golden", Snapshot([]view.ToolTotals{{Name: "Claude Code"}}, nil)},
		{"history populated", "history_populated.golden", History(monthlySeries, nil)},
		{"history empty", "history_empty.golden", History(view.Series{Name: "Claude Code"}, nil)},
		{"total history populated", "total_history_populated.golden", TotalHistory(sixSeries())},
		{"total history empty", "total_history_empty.golden", TotalHistory(empty)},
		// R10: machine_{name}_cost columns, sorted names, 0.00 fill, the
		// snapshot Total summing visible rows.
		{"snapshot machines", "snapshot_machines.golden", Snapshot(snapshotRows(), &view.Breakdown{Noun: "Machines", Rows: map[string][]view.Slice{
			"Claude Code": {
				{Name: "dev-ws-sahil02", Totals: fact.Totals{TotalCost: 0.2}},
				{Name: "Sahils-Mac-mini.local", Totals: fact.Totals{TotalCost: 0.3}},
			},
		}})},
		{"history machines", "history_machines.golden", History(monthlySeries, &view.Breakdown{Noun: "Machines", Rows: map[string][]view.Slice{
			"2026-01": {
				{Name: "harness-machine", Totals: fact.Totals{TotalCost: 1.0}},
				{Name: "other-box", Totals: fact.Totals{TotalCost: 0.5}},
			},
		}})},
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
				t.Errorf("CSV output differs from %s (-update to regenerate)\ngot:\n%q\nwant:\n%q", c.golden, got, string(raw))
			}
		})
	}
}

// The intake §12 captures pin the CSV shapes byte for byte.
func TestIntakeByteReferences(t *testing.T) {
	if got := strings.Join(History(monthlySeries, nil), "\n") + "\n"; got !=
		"date,input,output,cache_write,cache_read,total,cost\n"+
			"2026-01,9000,1200,3000,60000,73200,1.50\n" {
		t.Errorf("cc-mh-csv bytes = %q", got)
	}
	empty := []view.Series{
		{Name: "Claude Code"}, {Name: "Codex"}, {Name: "OpenCode"},
		{Name: "Gemini"}, {Name: "Copilot"}, {Name: "Kimi"},
	}
	if got := strings.Join(TotalHistory(empty), "\n") + "\n"; got !=
		"date,Claude Code,Codex,OpenCode,Gemini,Copilot,Kimi,total\n" {
		t.Errorf("h-csv bytes = %q", got)
	}
	if got := strings.Join(Snapshot(nil, nil), "\n") + "\n"; got != "tool,tokens,input,output,cache,cost\n" {
		t.Errorf("empty snapshot CSV = %q", got)
	}
}

func TestSnapshotTotalRowRules(t *testing.T) {
	// One visible row → no Total; a hidden (zero-token) row still sums in.
	hidden := fact.Totals{TotalCost: 0.25}
	rows := []view.ToolTotals{
		{Name: "Claude Code", Totals: dayTotals},
		{Name: "Codex", Totals: dayTotals},
		{Name: "OpenCode", Totals: hidden},
	}
	lines := Snapshot(rows, nil)
	last := lines[len(lines)-1]
	if !strings.HasPrefix(last, "Total,") {
		t.Fatalf("last line = %q, want the Total row", last)
	}
	if !strings.HasSuffix(last, ",1.25") {
		t.Errorf("Total cost = %q, want 1.25 (hidden row counted)", last)
	}

	one := Snapshot([]view.ToolTotals{{Name: "Claude Code", Totals: dayTotals}, {Name: "Codex"}}, nil)
	if len(one) != 2 {
		t.Errorf("single visible row: %d lines, want header + 1 (no Total)", len(one))
	}
}

func TestHistoryKindsNeverCarryTotal(t *testing.T) {
	for _, lines := range [][]string{History(dailySeries("Claude Code"), nil), TotalHistory(sixSeries())} {
		for _, l := range lines {
			if strings.HasPrefix(l, "Total,") {
				t.Errorf("history CSV must not carry a Total row: %q", l)
			}
		}
	}
}
