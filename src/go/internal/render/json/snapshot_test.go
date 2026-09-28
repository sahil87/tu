package json

import (
	"flag"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sahil87/tu/internal/fact"
	"github.com/sahil87/tu/internal/view"
)

// update regenerates the golden files: go test ./internal/render/json/ -update
var update = flag.Bool("update", false, "regenerate golden files")

var populatedTotals = fact.Totals{TotalCost: 0.5, InputTokens: 3000, OutputTokens: 400, CacheCreationTokens: 1000, CacheReadTokens: 20000, TotalTokens: 24400}

// allTools is the registry-ordered six-tool row set with Claude Code and
// Codex populated; every row carries the label (command hands the current
// period's label to every tool, data or not).
func allTools(label string) []view.ToolTotals {
	return []view.ToolTotals{
		{Name: "Claude Code", Label: label, Totals: populatedTotals},
		{Name: "Codex", Label: label, Totals: populatedTotals},
		{Name: "OpenCode", Label: label},
		{Name: "Gemini", Label: label},
		{Name: "Copilot", Label: label},
		{Name: "Kimi", Label: label},
	}
}

func TestSnapshotGoldens(t *testing.T) {
	cases := []struct {
		name   string
		golden string
		rows   []view.ToolTotals
		bd     *view.Breakdown
	}{
		// tu m --json shape: labels on every tool.
		{"all tools monthly", "all_tools_labeled.golden", allTools("2026-09"), nil},
		// tu --json shape: the daily label on every tool.
		{"all tools daily", "all_tools_daily.golden", allTools("2026-09-16"), nil},
		// tu cc --json shape: one tool, label first.
		{"single tool", "single_tool.golden", []view.ToolTotals{{Name: "Claude Code", Label: "2026-09-16", Totals: populatedTotals}}, nil},
		// The placeholder-corpus state: six zero objects, labels carried.
		{"all zero", "all_zero.golden", allToolsZero("2026-09-16"), nil},
		// R9: tu --by-machine --json — machines after totalTokens, first-seen
		// slice order (NOT alphabetical), cost values; slice-less tools carry
		// an empty machines object.
		{"machines", "snapshot_machines.golden", allTools("2026-09"), &view.Breakdown{Noun: "Machines", Rows: map[string][]view.Slice{
			"Claude Code": {
				{Name: "Sahils-Mac-mini.local", Totals: fact.Totals{TotalCost: 0.3}},
				{Name: "dev-ws-sahil02", Totals: fact.Totals{TotalCost: 0.2}},
			},
		}}},
		// A-020: the single-source zero-fill — machines with 0 values on a
		// zero-usage day, label carried like every other row.
		{"machines zero usage", "snapshot_machines_zero_usage.golden", []view.ToolTotals{{Name: "Claude Code", Label: "2026-09-16"}}, &view.Breakdown{Noun: "Machines", Rows: map[string][]view.Slice{
			"Claude Code": {
				{Name: "harness-machine"},
				{Name: "other-box"},
			},
		}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := strings.Join(Snapshot(c.rows, c.bd), "\n") + "\n"
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
				t.Errorf("JSON output differs from %s (-update to regenerate)\ngot:\n%s\nwant:\n%s", c.golden, got, string(raw))
			}
		})
	}
}

func allToolsZero(label string) []view.ToolTotals {
	rows := allTools(label)
	for i := range rows {
		rows[i].Totals = fact.Totals{}
	}
	return rows
}

func TestEncodeFloat(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0.5, "0.5"},
		{4.936068800000001, "4.936068800000001"},
		{0, "0"},
		{math.Copysign(0, -1), "0"}, // Go's encoder writes -0; V8 writes 0
		{24400.0, "24400"},
	}
	for _, c := range cases {
		if got := encodeFloat(c.in); got != c.want {
			t.Errorf("encodeFloat(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

// The key set is stable: "label" is always the first key (command hands
// every row the current period's label, data or not), and under --by-machine
// every object ends with "machines" — "{}" when the tool has no slices.
func TestStableKeySet(t *testing.T) {
	lines := Snapshot([]view.ToolTotals{
		{Name: "Claude Code", Label: "2026-09-16", Totals: populatedTotals},
		{Name: "Codex", Label: "2026-09-16"},
	}, nil)
	joined := strings.Join(lines, "\n")
	if strings.Count(joined, "\"label\": \"2026-09-16\",\n    \"totalCost\"") != 2 {
		t.Errorf("label must be the first key on every object:\n%s", joined)
	}

	bd := &view.Breakdown{Noun: "Machines", Rows: map[string][]view.Slice{
		"Claude Code": {{Name: "devbox", Totals: fact.Totals{TotalCost: 12.3}}},
	}}
	joined = strings.Join(Snapshot([]view.ToolTotals{
		{Name: "Claude Code", Label: "2026-09-16", Totals: populatedTotals},
		{Name: "Codex", Label: "2026-09-16"},
	}, bd), "\n")
	codex := joined[strings.Index(joined, `"Codex"`):]
	if !strings.Contains(codex, "\"totalTokens\": 0,\n    \"machines\": {}") {
		t.Errorf("a tool with no slices must end with an empty machines object:\n%s", codex)
	}
	if !strings.Contains(joined, "\"totalTokens\": 24400,\n    \"machines\": {\n      \"devbox\": 12.3\n    }") {
		t.Errorf("the populated tool's machines object is wrong:\n%s", joined)
	}
}
