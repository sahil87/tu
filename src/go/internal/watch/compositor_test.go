package watch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/sahil87/tu/internal/render/ansi"
)

func goldenFile(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
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
		t.Errorf("differs from %s (-update to regenerate)\ngot:\n%q\nwant:\n%q", name, got, string(raw))
	}
}

var skeletonNow = time.Date(2026, 9, 17, 9, 0, 0, 0, time.UTC)

// The skeleton goldens: full at 100×30 (stats grid + daily snapshot header +
// centered Loading...) and compact at 50×20 (blank + Loading...).
func TestSkeletonGoldens(t *testing.T) {
	color := ansi.Colors{Enabled: true}
	full := Skeleton(100, skeletonNow, color)
	goldenFile(t, "skeleton_100x30.golden", strings.Join(full, "\n")+"\n")
	compact := Skeleton(50, skeletonNow, color)
	goldenFile(t, "skeleton_50x20.golden", strings.Join(compact, "\n")+"\n")

	// The write form: \x1b[H + lines + "\n" each, no \x1b[K.
	if got := string(SkeletonFrame([]string{"a", "b"})); got != "\x1b[Ha\nb\n" {
		t.Errorf("SkeletonFrame = %q", got)
	}
}

// The header is the daily snapshot header for EVERY display type (the TS
// renderSkeleton builds it unconditionally — a G0 flag against layouts §8).
func TestSkeletonContents(t *testing.T) {
	lines := Skeleton(100, skeletonNow, ansi.Colors{})
	// 4 stats lines + "" + title + "" + header + divider + loading = 10.
	if len(lines) != 10 {
		t.Fatalf("skeleton lines = %d, want 10: %q", len(lines), lines)
	}
	if lines[4] != "" || lines[5] != "📊 Combined Usage (daily)" || lines[6] != "" {
		t.Errorf("title block = %q", lines[4:7])
	}
	if lines[7] != "Tool         |       Tokens |        Input |       Output |        Cache |         Cost" {
		t.Errorf("header = %q", lines[7])
	}
	if got := utf8.RuneCountInString(lines[8]); got != 87 {
		t.Errorf("divider = %d visible chars, want 87", got)
	}
	if lines[9] != strings.Repeat(" ", 38)+"Loading..." {
		t.Errorf("loading line = %q", lines[9])
	}
	if got := Skeleton(59, skeletonNow, ansi.Colors{}); len(got) != 2 || got[0] != "" || got[1] != "Loading..." {
		t.Errorf("compact skeleton = %q", got)
	}
}

// Lay's zone selection table (R7): below-content, right-margin at the 10/9
// boundary, disabled, and compact.
func TestLay(t *testing.T) {
	stats := []string{"s1", "s2", "s3", "s4"}
	table := []string{"t1", "t2", "t3"}

	cases := []struct {
		name           string
		stats, tbl     []string
		cols, rows     int
		noRain         bool
		wantCompact    bool
		wantZone       RainZone
		wantStatsLines int
	}{
		// 100×30, 4+3=7 content rows, available 22 → below-content at row 8.
		{"below content", stats, table, 100, 30, false, false, RainZone{Enabled: true, Cols: 100, Rows: 22, StartRow: 8}, 4},
		// R7: 100×30, 4 stats + 12 table → zone {100, 13, 17, 0}.
		{"below content R7", stats, make([]string, 12), 100, 30, false, false, RainZone{Enabled: true, Cols: 100, Rows: 13, StartRow: 17}, 4},
		// 100×12, 16 content lines of width 87 → available −5; right margin
		// 100 − 87 − 2 = 11 ≥ 10 → {11, 11, 1, 89}.
		{"right margin R7", nil, pad16(87), 100, 12, false, false, RainZone{Enabled: true, Cols: 11, Rows: 11, StartRow: 1, StartCol: 89}, 0},
		// Widest 89 → marginCols 9 < 10 → disabled.
		{"right margin too narrow", nil, pad16(89), 100, 12, false, false, RainZone{}, 0},
		{"compact drops stats and rain", stats, table, 59, 20, false, true, RainZone{}, 0},
		{"no-rain disables", stats, table, 100, 30, true, false, RainZone{}, 4},
		// Zero rows available exactly → not below-content (available > 0
		// required); falls to the margin rule.
		{"no rows available", nil, []string{"x"}, 100, 2, false, false, RainZone{Enabled: true, Cols: 97, Rows: 1, StartRow: 1, StartCol: 3}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := Lay(tc.stats, tc.tbl, tc.cols, tc.rows, tc.noRain)
			if l.Compact != tc.wantCompact {
				t.Errorf("Compact = %v, want %v", l.Compact, tc.wantCompact)
			}
			if l.Rain != tc.wantZone {
				t.Errorf("Rain = %+v, want %+v", l.Rain, tc.wantZone)
			}
			if len(l.Stats) != tc.wantStatsLines {
				t.Errorf("stats lines = %d, want %d", len(l.Stats), tc.wantStatsLines)
			}
			if l.Rows != tc.rows {
				t.Errorf("Rows = %d, want %d", l.Rows, tc.rows)
			}
		})
	}
}

// pad16 builds 16 content lines whose widest is w visible characters.
func pad16(w int) []string {
	lines := make([]string, 16)
	lines[0] = strings.Repeat("x", w)
	for i := 1; i < 16; i++ {
		lines[i] = "y"
	}
	return lines
}

// LaySkeleton treats the skeleton lines as the whole content.
func TestLaySkeleton(t *testing.T) {
	lines := []string{"a", "bb", "ccc"}
	l := LaySkeleton(lines, 100, 30, false)
	if l.Compact || len(l.Stats) != 0 || len(l.Table) != 3 {
		t.Errorf("layout = %+v", l)
	}
	want := RainZone{Enabled: true, Cols: 100, Rows: 26, StartRow: 4}
	if l.Rain != want {
		t.Errorf("Rain = %+v, want %+v", l.Rain, want)
	}
}

// Frame bytes (R8): \x1b[H, lines + \x1b[K\n, \x1b[J, the footer at the last
// row, then the rain frame.
func TestFrameBytes(t *testing.T) {
	l := Layout{Stats: []string{"S"}, Table: []string{"T1", "T2"}, Rows: 24}
	got := string(l.Frame("F", "R"))
	want := "\x1b[HS\x1b[K\nT1\x1b[K\nT2\x1b[K\n\x1b[J\x1b[24;1H\x1b[KFR"
	if got != want {
		t.Errorf("Frame = %q, want %q", got, want)
	}
	if got := FooterLine("F", 24); got != "\x1b[24;1H\x1b[KF" {
		t.Errorf("FooterLine = %q", got)
	}
}
