package markdown

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sahil87/tu/internal/fact"
	"github.com/sahil87/tu/internal/query"
	"github.com/sahil87/tu/internal/view"
)

// lbRows is the R12 given: the layouts § 16 leaderboard data (sahil/beatriz,
// deltas set), shares folded over the ranked set.
func lbRows() []view.LeaderboardRow {
	d := func(v float64) *float64 { return &v }
	rows := []view.LeaderboardRow{
		{Rank: 1, User: "sahil", Totals: fact.Totals{TotalCost: 12945.64, TotalTokens: 15962442751}, Delta: d(-0.55)},
		{Rank: 2, User: "beatriz", Totals: fact.Totals{TotalCost: 3333.40, TotalTokens: 5562910079}, Delta: d(-0.51)},
	}
	grand := rows[0].TotalCost + rows[1].TotalCost
	rows[0].Share = rows[0].TotalCost / grand
	rows[1].Share = rows[1].TotalCost / grand
	return rows
}

func TestLeaderboardGoldens(t *testing.T) {
	rows := lbRows()
	bm := lbRows()
	bm[0].Machine = "dev-ws-sahil02"
	bm[1].Machine = "laptop"
	newRows := lbRows()
	newRows[0].Delta = nil
	newRows[1].Delta = nil
	cases := []struct {
		name   string
		golden string
		lines  []string
	}{
		{"leaderboard", "leaderboard.golden", Leaderboard(rows, rows, query.Monthly, "Aug", false)},
		{"leaderboard by machine", "leaderboard_by_machine.golden", Leaderboard(bm, bm, query.Monthly, "Aug", true)},
		// --top 1: one data row, the Total still sums the full set; nil deltas.
		{"leaderboard top total", "leaderboard_top_total.golden", Leaderboard(newRows[:1], newRows, query.Monthly, "Aug", false)},
		{"leaderboard empty", "leaderboard_empty.golden", Leaderboard(nil, nil, query.Monthly, "prev", false)},
		// The lbh pivot title (the TS mdTitle, without the emoji).
		{"total history lbh title", "total_history_lbh_title.golden", TotalHistory([]view.Series{
			{Name: "otheruser", Entries: []view.Entry{{Label: "2026-08-05", Totals: fact.Totals{TotalCost: 24.15}}}},
			{Name: "sbuser", Entries: []view.Entry{{Label: "2026-08-05", Totals: dayTotals}}},
		}, query.Daily, true, "Leaderboard History")},
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
				t.Errorf("markdown output differs from %s (-update to regenerate)\ngot:\n%q\nwant:\n%q", c.golden, got, string(raw))
			}
		})
	}
}
