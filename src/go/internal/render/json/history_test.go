package json

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sahil87/tu/internal/fact"
	"github.com/sahil87/tu/internal/view"
)

// monthlySeries is the intake §12 cc-mh window: the three placeholder days
// rolled up to 2026-01.
var monthlySeries = view.Series{Name: "Claude Code", Entries: []view.Entry{
	{Label: "2026-01", Totals: fact.Totals{TotalCost: 1.5, InputTokens: 9000, OutputTokens: 1200, CacheCreationTokens: 3000, CacheReadTokens: 60000, TotalTokens: 73200}},
}}

func sixEmptySeries() []view.Series {
	return []view.Series{
		{Name: "Claude Code"}, {Name: "Codex"}, {Name: "OpenCode"},
		{Name: "Gemini"}, {Name: "Copilot"}, {Name: "Kimi"},
	}
}

func TestHistoryGoldens(t *testing.T) {
	cases := []struct {
		name   string
		golden string
		lines  []string
	}{
		{"history populated", "history_populated.golden", History(monthlySeries, nil)},
		{"history empty", "history_empty.golden", History(view.Series{Name: "Claude Code"}, nil)},
		// R9: machines after totalTokens, first-seen slice order per label.
		{"history machines", "history_machines.golden", History(monthlySeries, &view.Breakdown{Noun: "Machines", Rows: map[string][]view.Slice{
			"2026-01": {
				{Name: "harness-machine", Totals: fact.Totals{TotalCost: 1.0}},
				{Name: "other-box", Totals: fact.Totals{TotalCost: 0.5}},
			},
		}})},
		{"total history mixed", "total_history_mixed.golden", TotalHistory([]view.Series{
			monthlySeries,
			{Name: "Codex"},
		})},
		{"total history all empty", "total_history_all_empty.golden", TotalHistory(sixEmptySeries())},
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
				t.Errorf("JSON output differs from %s (-update to regenerate)\ngot:\n%q\nwant:\n%q", c.golden, got, string(raw))
			}
		})
	}
}

// The intake §12 captures pin the two history JSON shapes byte for byte.
func TestIntakeByteReferences(t *testing.T) {
	got := strings.Join(History(monthlySeries, nil), "\n") + "\n"
	want := "[\n" +
		"  {\n" +
		`    "label": "2026-01",` + "\n" +
		`    "totalCost": 1.5,` + "\n" +
		`    "inputTokens": 9000,` + "\n" +
		`    "outputTokens": 1200,` + "\n" +
		`    "cacheCreationTokens": 3000,` + "\n" +
		`    "cacheReadTokens": 60000,` + "\n" +
		`    "totalTokens": 73200` + "\n" +
		"  }\n" +
		"]\n"
	if got != want {
		t.Errorf("cc-mh-json bytes =\n%q\nwant:\n%q", got, want)
	}

	got = strings.Join(TotalHistory(sixEmptySeries()), "\n") + "\n"
	want = "{\n" +
		`  "Claude Code": [],` + "\n" +
		`  "Codex": [],` + "\n" +
		`  "OpenCode": [],` + "\n" +
		`  "Gemini": [],` + "\n" +
		`  "Copilot": [],` + "\n" +
		`  "Kimi": []` + "\n" +
		"}\n"
	if got != want {
		t.Errorf("h-json bytes =\n%q\nwant:\n%q", got, want)
	}

	if got := strings.Join(History(view.Series{Name: "Claude Code"}, nil), "\n"); got != "[]" {
		t.Errorf("empty single-tool history = %q, want []", got)
	}
}
