package csv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sahil87/tu/internal/fact"
	"github.com/sahil87/tu/internal/view"
)

// lbRows is the R11 given: the seed window rows (harness-user $1.70 /
// 97,600 tokens, other-user $1.30 / 48,800), shares folded over the ranked
// set, deltas nil (new).
func lbRows() []view.LeaderboardRow {
	rows := []view.LeaderboardRow{
		{Rank: 1, User: "harness-user", Totals: fact.Totals{TotalCost: 1.7, TotalTokens: 97600}},
		{Rank: 2, User: "other-user", Totals: fact.Totals{TotalCost: 1.3, TotalTokens: 48800}},
	}
	rows[0].Share = rows[0].TotalCost / (rows[0].TotalCost + rows[1].TotalCost)
	rows[1].Share = rows[1].TotalCost / (rows[0].TotalCost + rows[1].TotalCost)
	return rows
}

// R11: csvShare drops trailing zeros and keeps the JS Math.round sign rule.
func TestCsvShare(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0.69, "0.69"},
		{0.567, "0.567"},
		{0.433, "0.433"},
		{-0.3, "-0.3"},
		{-0.553, "-0.553"},
		{17.309, "17.309"},
		{0, "0"},
		{1, "1"},
		{-0.0004, "0"}, // Math.round(-0.4) = -0, normalized
		{0.6905, "0.691"},
	}
	for _, c := range cases {
		if got := csvShare(c.in); got != c.want {
			t.Errorf("csvShare(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestLeaderboardGoldens(t *testing.T) {
	rows := lbRows()
	bm := lbRows()
	bm[0].Machine = "harness-machine"
	bm[1].Machine = "laptop"
	top := lbRows()
	d := -0.51
	top[1].Delta = &d
	cases := []struct {
		name   string
		golden string
		lines  []string
	}{
		{"leaderboard", "leaderboard.golden", Leaderboard(rows, rows, false)},
		{"leaderboard by machine", "leaderboard_by_machine.golden", Leaderboard(bm, bm, true)},
		// --top 1 slices the data rows; the Total still sums the full set.
		{"leaderboard top total", "leaderboard_top_total.golden", Leaderboard(top[:1], top, false)},
		{"leaderboard empty", "leaderboard_empty.golden", Leaderboard(nil, nil, false)},
		// An empty --by-machine result keeps the machine column in the schema.
		{"leaderboard empty by machine", "leaderboard_empty_by_machine.golden", Leaderboard(nil, nil, true)},
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
				t.Errorf("csv output differs from %s (-update to regenerate)\ngot:\n%q\nwant:\n%q", c.golden, got, string(raw))
			}
		})
	}
}
