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
// Codex populated.
func allTools(label string) []view.ToolTotals {
	return []view.ToolTotals{
		{Name: "Claude Code", Label: label, Totals: populatedTotals},
		{Name: "Codex", Label: label, Totals: populatedTotals},
		{Name: "OpenCode"},
		{Name: "Gemini"},
		{Name: "Copilot"},
		{Name: "Kimi"},
	}
}

func TestSnapshotGoldens(t *testing.T) {
	cases := []struct {
		name   string
		golden string
		rows   []view.ToolTotals
	}{
		// tu m --json shape: labels on the populated tools.
		{"all tools with labels", "all_tools_labeled.golden", allTools("2026-09")},
		// tu --json shape: the daily-all quirk drops every label.
		{"all tools no labels", "all_tools_unlabeled.golden", allTools("")},
		// tu cc --json shape: one tool, label first.
		{"single tool", "single_tool.golden", []view.ToolTotals{{Name: "Claude Code", Label: "2026-09-16", Totals: populatedTotals}}},
		// The placeholder-corpus state: six zero objects, no labels.
		{"all zero", "all_zero.golden", allToolsZero()},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := strings.Join(Snapshot(c.rows), "\n") + "\n"
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

func allToolsZero() []view.ToolTotals {
	rows := allTools("")
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

func TestLabelFirstAndConditional(t *testing.T) {
	lines := Snapshot([]view.ToolTotals{
		{Name: "Claude Code", Label: "2026-09-16", Totals: populatedTotals},
		{Name: "Codex"},
	})
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "\"label\": \"2026-09-16\",\n    \"totalCost\"") {
		t.Errorf("label must be the first key when present:\n%s", joined)
	}
	codex := joined[strings.Index(joined, `"Codex"`):]
	if strings.Contains(codex, "label") {
		t.Errorf("label must be absent when Label is empty:\n%s", codex)
	}
}
