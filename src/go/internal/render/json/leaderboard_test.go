package json

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sahil87/tu/internal/fact"
	"github.com/sahil87/tu/internal/view"
)

// lbRowsJSON is the R10 given: the layouts §12 array shape with the retired
// TypeScript implementation's raw doubles (captured with
// env -i HOME=<staged> TZ=UTC, `m lb --json`).
func lbRowsJSON() []view.LeaderboardRow {
	d := func(v float64) *float64 { return &v }
	return []view.LeaderboardRow{
		{Rank: 1, User: "sahil", Totals: fact.Totals{TotalCost: 12945.642098040002, TotalTokens: 15962442751}, Share: 0.6900258986630633, Delta: d(-0.5532817482507824)},
		{Rank: 2, User: "bob", Totals: fact.Totals{TotalCost: 5333.4, TotalTokens: 5562910079}, Share: 0.3099741013369367},
	}
}

func TestLeaderboardGoldens(t *testing.T) {
	bm := lbRowsJSON()
	bm[0].Machine = "dev-ws-sahil02"
	bm[1].Machine = "laptop"
	d := func(v float64) *float64 { return &v }
	both := lbRowsJSON()
	both[1].Delta = d(-0.51)
	cases := []struct {
		name   string
		golden string
		lines  []string
	}{
		{"leaderboard", "leaderboard.golden", Leaderboard(both)},
		{"leaderboard by machine", "leaderboard_by_machine.golden", Leaderboard(bm)},
		// delta nil → null on the second object.
		{"leaderboard new delta", "leaderboard_new_delta.golden", Leaderboard(lbRowsJSON())},
		{"leaderboard empty", "leaderboard_empty.golden", Leaderboard(nil)},
		// The lbh pivot JSON under --top: kept users in Repo.Users() order,
		// "others" last (the TS foldLeaderboardColumns appends it).
		{"total history lbh top", "total_history_lbh_top.golden", TotalHistory([]view.Series{
			{Name: "harness-user", Entries: []view.Entry{{Label: "2026-01", Totals: fact.Totals{TotalCost: 1.7, TotalTokens: 97600}}}},
			{Name: "others", Entries: []view.Entry{{Label: "2026-01", Totals: fact.Totals{TotalCost: 1.3, TotalTokens: 48800}}}},
		})},
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
				t.Errorf("json output differs from %s (-update to regenerate)\ngot:\n%q\nwant:\n%q", c.golden, got, string(raw))
			}
		})
	}
}
