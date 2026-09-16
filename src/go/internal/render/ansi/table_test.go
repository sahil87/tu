package ansi

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

// update regenerates the golden files: go test ./internal/render/ansi/ -update
var update = flag.Bool("update", false, "regenerate golden files")

var populatedTotals = fact.Totals{TotalCost: 0.5, InputTokens: 3000, OutputTokens: 400, CacheCreationTokens: 1000, CacheReadTokens: 20000, TotalTokens: 24400}

// snapshotRows mirrors intake §10's populated fixture: Claude Code and Codex
// carry today's totals; the other four registry tools are all-zero.
func snapshotRows() []view.ToolTotals {
	return []view.ToolTotals{
		{Name: "Claude Code", Label: "2026-09-16", Totals: populatedTotals},
		{Name: "Codex", Label: "2026-09-16", Totals: populatedTotals},
		{Name: "OpenCode"},
		{Name: "Gemini"},
		{Name: "Copilot"},
		{Name: "Kimi"},
	}
}

func TestTableGoldens(t *testing.T) {
	color := Colors{Enabled: true}
	cases := []struct {
		name   string
		golden string
		lines  []string
	}{
		{"populated color", "snapshot_color.golden", Table(view.Snapshot(snapshotRows(), query.Daily), color)},
		{"populated no-color", "snapshot_nocolor.golden", Table(view.Snapshot(snapshotRows(), query.Daily), Colors{})},
		{"single row", "snapshot_single.golden", Table(view.Snapshot(snapshotRows()[:1], query.Daily), color)},
		{"empty", "snapshot_empty.golden", Table(view.Snapshot(nil, query.Daily), color)},
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
				t.Errorf("table output differs from %s (-update to regenerate)\ngot:\n%q\nwant:\n%q", c.golden, got, string(raw))
			}
		})
	}
}

// StripANSI of the colored render must equal the no-color render, byte for
// byte (the TS invariant behind --no-color/NO_COLOR equivalence).
func TestStripANSIMatchesNoColor(t *testing.T) {
	color := Table(view.Snapshot(snapshotRows(), query.Daily), Colors{Enabled: true})
	plain := Table(view.Snapshot(snapshotRows(), query.Daily), Colors{})
	if len(color) != len(plain) {
		t.Fatalf("line counts differ: %d vs %d", len(color), len(plain))
	}
	for i := range color {
		if StripANSI(color[i]) != plain[i] {
			t.Errorf("line %d: StripANSI(%q) = %q, want %q", i, color[i], StripANSI(color[i]), plain[i])
		}
	}
}

func TestEmptyStateLines(t *testing.T) {
	got := Table(view.Snapshot(nil, query.Daily), Colors{Enabled: true})
	want := []string{"", "\x1b[1;37m📊 Combined Usage (daily)\x1b[0m", "", "  No usage", ""}
	if len(got) != len(want) {
		t.Fatalf("got %d lines, want %d: %q", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestDividerWidth(t *testing.T) {
	lines := Table(view.Snapshot(snapshotRows(), query.Daily), Colors{})
	var div string
	for _, l := range lines {
		if strings.Contains(l, "─") {
			div = l
			break
		}
	}
	if n := len([]rune(div)); n != 87 {
		t.Errorf("divider = %d visible chars, want 87: %q", n, div)
	}
}

func TestColors(t *testing.T) {
	off := Colors{}
	if got := off.BoldWhite("x"); got != "x" {
		t.Errorf("disabled BoldWhite = %q, want %q", got, "x")
	}
	on := Colors{Enabled: true}
	cases := []struct {
		name string
		fn   func(string) string
		want string
	}{
		{"Bold", on.Bold, "\x1b[1mx\x1b[0m"},
		{"Dim", on.Dim, "\x1b[2mx\x1b[0m"},
		{"Green", on.Green, "\x1b[32mx\x1b[0m"},
		{"Red", on.Red, "\x1b[31mx\x1b[0m"},
		{"Cyan", on.Cyan, "\x1b[36mx\x1b[0m"},
		{"Yellow", on.Yellow, "\x1b[33mx\x1b[0m"},
		{"Magenta", on.Magenta, "\x1b[35mx\x1b[0m"},
		{"Blue", on.Blue, "\x1b[34mx\x1b[0m"},
		{"BoldWhite", on.BoldWhite, "\x1b[1;37mx\x1b[0m"},
		{"BoldCyan", on.BoldCyan, "\x1b[1;36mx\x1b[0m"},
		{"BrightGreen", on.BrightGreen, "\x1b[92mx\x1b[0m"},
		{"DimGreen", on.DimGreen, "\x1b[2;32mx\x1b[0m"},
	}
	for _, c := range cases {
		if got := c.fn("x"); got != c.want {
			t.Errorf("%s = %q, want %q", c.name, got, c.want)
		}
		if StripANSI(c.fn("x")) != "x" {
			t.Errorf("StripANSI(%s) did not remove the wrap", c.name)
		}
	}
}

func TestPad(t *testing.T) {
	if got := PadRight("Claude Code", 12); got != "Claude Code " {
		t.Errorf("PadRight = %q", got)
	}
	if got := PadLeft("24,400", 12); got != "      24,400" {
		t.Errorf("PadLeft = %q", got)
	}
	if got := PadLeft("1,234,567,890,123", 12); got != "1,234,567,890,123" {
		t.Errorf("PadLeft overflow = %q (a wider value is returned unchanged)", got)
	}
	if got := PadRight("é", 3); got != "é  " {
		t.Errorf("PadRight pads by rune count, got %q", got)
	}
}
