package ansi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sahil87/tu/internal/fact"
	"github.com/sahil87/tu/internal/query"
	"github.com/sahil87/tu/internal/view"
)

// lbSix is the layouts §5 / oracle-captured six-user window (2026-09 vs Aug),
// shares computed over the ranked set exactly as buildLeaderboard folds them.
// The fixture's costs, shares and deltas are the node oracle's raw doubles
// (env -i HOME=<staged> TZ=UTC node dist/tu.mjs m lb --json, 2026-09-16).
func lbSix() []view.LeaderboardRow {
	d := func(v float64) *float64 { return &v }
	rows := []view.LeaderboardRow{
		{Rank: 1, User: "sahil", Totals: fact.Totals{TotalCost: 12945.64, TotalTokens: 15962442751}, Delta: d(-0.55)},
		{Rank: 2, User: "beatriz", Totals: fact.Totals{TotalCost: 3333.4, TotalTokens: 5562910079}, Delta: d(-0.51)},
		{Rank: 3, User: "carlos", Totals: fact.Totals{TotalCost: 1598.54, TotalTokens: 1681643674}, Delta: d(-0.9099999999999999)},
		{Rank: 4, User: "dominic", Totals: fact.Totals{TotalCost: 611.44, TotalTokens: 390936777}, Delta: d(-0.85)},
		{Rank: 5, User: "eunice", Totals: fact.Totals{TotalCost: 155.6, TotalTokens: 32774697}, Delta: d(-0.9600000000000001)},
		{Rank: 6, User: "frankie", Totals: fact.Totals{TotalCost: 116.47, TotalTokens: 100240636}, Delta: d(-0.98)},
	}
	return withShares(rows, view.Cost)
}

// withShares folds the metric values over the ranked rows and fills Share —
// the command-side share rule, so the fixtures match a real Run.
func withShares(rows []view.LeaderboardRow, m view.Metric) []view.LeaderboardRow {
	grand := 0.0
	values := make([]float64, len(rows))
	for i, r := range rows {
		v := r.TotalCost
		if m == view.Tokens {
			v = float64(r.TotalTokens)
		}
		values[i] = v
		grand += v
	}
	for i := range rows {
		if grand > 0 {
			rows[i].Share = values[i] / grand
		}
	}
	return rows
}

// lbSixTokens re-ranks the six-user window by tokens (frankie passes eunice),
// with the oracle's token-metric delta doubles (the prev window's 1-token
// rows make them enormous — the Δ column widens and eats the bar budget).
func lbSixTokens() []view.LeaderboardRow {
	d := func(v float64) *float64 { return &v }
	rows := []view.LeaderboardRow{
		{Rank: 1, User: "sahil", Totals: fact.Totals{TotalCost: 12945.64, TotalTokens: 15962442751}, Delta: d(15962442750)},
		{Rank: 2, User: "beatriz", Totals: fact.Totals{TotalCost: 3333.4, TotalTokens: 5562910079}, Delta: d(5562910078)},
		{Rank: 3, User: "carlos", Totals: fact.Totals{TotalCost: 1598.54, TotalTokens: 1681643674}, Delta: d(1681643673)},
		{Rank: 4, User: "dominic", Totals: fact.Totals{TotalCost: 611.44, TotalTokens: 390936777}, Delta: d(390936776)},
		{Rank: 5, User: "frankie", Totals: fact.Totals{TotalCost: 116.47, TotalTokens: 100240636}, Delta: d(100240635)},
		{Rank: 6, User: "eunice", Totals: fact.Totals{TotalCost: 155.6, TotalTokens: 32774697}, Delta: d(32774696)},
	}
	return withShares(rows, view.Tokens)
}

// lbByMachine is the layouts §17 three-row window (user/machine pairs).
func lbByMachine() []view.LeaderboardRow {
	d := func(v float64) *float64 { return &v }
	return withShares([]view.LeaderboardRow{
		{Rank: 1, User: "sahil", Machine: "dev-ws-sahil02", Totals: fact.Totals{TotalCost: 9340.16, TotalTokens: 11455277645}, Delta: d(-0.66)},
		{Rank: 2, User: "beatriz", Machine: "dev-ws-beatri01", Totals: fact.Totals{TotalCost: 3206.02, TotalTokens: 5445803037}, Delta: d(-0.43)},
		{Rank: 3, User: "sahil", Machine: "dev-ws-sahil01", Totals: fact.Totals{TotalCost: 2963.66, TotalTokens: 4041086220}, Delta: d(47.57)},
	}, view.Cost)
}

// lbOutlier triggers the two-zone scale mid-row (the history outlier sample
// as users: 21 rows at 100…300, one at 4,031.61, one zero row whose dim cost
// cell and unstyled bar gap pin the empty-Main rule).
func lbOutlier() []view.LeaderboardRow {
	rows := []view.LeaderboardRow{{Rank: 1, User: "whale", Totals: fact.Totals{TotalCost: 4031.61, TotalTokens: 24400}}}
	for i := 0; i < 21; i++ {
		rows = append(rows, view.LeaderboardRow{
			Rank:   i + 2,
			User:   "u" + pad2(i+1),
			Totals: fact.Totals{TotalCost: 300 - float64(i)*10, TotalTokens: 24400},
		})
	}
	rows = append(rows, view.LeaderboardRow{Rank: 23, User: "zero", Totals: fact.Totals{}})
	return withShares(rows, view.Cost)
}

// lbhRanked is the layouts §6 two-month pivot as users, ranked by window
// total, leader cells per row.
func lbhRanked() []view.Series {
	cost := func(c float64) view.Entry {
		return view.Entry{Label: "", Totals: fact.Totals{TotalCost: c, TotalTokens: 1}}
	}
	at := func(label string, c float64) view.Entry { e := cost(c); e.Label = label; return e }
	return []view.Series{
		{Name: "eunice", Entries: []view.Entry{at("2026-07", 0), at("2026-08", 0)}},
		{Name: "bob", Entries: []view.Entry{at("2026-07", 169.00), at("2026-08", 220.05)}},
		{Name: "alice", Entries: []view.Entry{at("2026-07", 314.60), at("2026-08", 301.10)}},
		{Name: "sahil", Entries: []view.Entry{at("2026-07", 355.20), at("2026-08", 412.30)}},
	}
}

// lbhOthers is a --top-2-folded pivot whose others column (290) lands
// mid-table between alice (300) and dave (110) — DC-08.
func lbhOthers() []view.Series {
	cost := func(label string, c float64) view.Entry {
		return view.Entry{Label: label, Totals: fact.Totals{TotalCost: c, TotalTokens: 1}}
	}
	return []view.Series{
		{Name: "alice", Entries: []view.Entry{cost("2026-07", 100), cost("2026-08", 200)}},
		{Name: "dave", Entries: []view.Entry{cost("2026-07", 50), cost("2026-08", 60)}},
		{Name: "others", Entries: []view.Entry{cost("2026-07", 250), cost("2026-08", 40)}},
	}
}

// lbTable renders the leaderboard fixture with the given knobs.
func lbTable(rows []view.LeaderboardRow, m view.Metric, top, width int, lastSync string) view.Table {
	return view.Leaderboard(rows, view.LeaderboardOptions{
		Period:      query.Monthly,
		WindowLabel: "2026-09",
		DeltaLabel:  "Aug",
		Metric:      m,
		PinnedUser:  "sahil",
		Top:         top,
		Width:       width,
		LastSync:    lastSync,
	})
}

func lbhTable(series []view.Series, width int) view.Table {
	return view.TotalHistory(series, view.HistoryOptions{
		Period:          query.Monthly,
		Now:             historyNow,
		Width:           width,
		Title:           "📊 Leaderboard History (monthly)",
		RankColumns:     true,
		HighlightLeader: true,
		KeepAllColumns:  true,
	})
}

// synced15m is the layouts §5 staleness footer text (captured from the oracle
// with a .last-sync staged 15 minutes back).
const synced15m = "15m ago (2026-09-16T22:18:19.000Z)"

func TestLeaderboardGoldens(t *testing.T) {
	color := Colors{Enabled: true}
	cases := []struct {
		name   string
		golden string
		lines  []string
	}{
		{"leaderboard color", "leaderboard_color.golden", Table(lbTable(lbSix(), view.Cost, 0, 80, synced15m), color)},
		{"leaderboard no-color", "leaderboard_nocolor.golden", Table(lbTable(lbSix(), view.Cost, 0, 80, synced15m), Colors{})},
		{"leaderboard tokens", "leaderboard_tokens.golden", Table(lbTable(lbSixTokens(), view.Tokens, 0, 80, synced15m), color)},
		{"leaderboard top", "leaderboard_top.golden", Table(lbTable(lbSix(), view.Cost, 2, 80, synced15m), color)},
		{"leaderboard by machine", "leaderboard_by_machine.golden", Table(lbTable(lbByMachine(), view.Cost, 0, 80, synced15m), color)},
		{"leaderboard wide", "leaderboard_wide.golden", Table(lbTable(lbSix(), view.Cost, 0, 120, synced15m), color)},
		{"leaderboard two-zone", "leaderboard_two_zone.golden", Table(lbTable(lbOutlier(), view.Cost, 0, 120, synced15m), color)},
		{"leaderboard empty", "leaderboard_empty.golden", Table(lbTable(nil, view.Cost, 0, 80, synced15m), color)},
		{"leaderboard never synced", "leaderboard_never_synced.golden", Table(lbTable(lbSix(), view.Cost, 0, 80, "never"), color)},
		{"leaderboard single row", "leaderboard_single_row.golden", Table(lbTable(lbSix()[:1], view.Cost, 0, 80, synced15m), color)},
		{"pivot lbh ranked", "pivot_lbh_ranked.golden", Table(lbhTable(lbhRanked(), 80), color)},
		{"pivot lbh top others", "pivot_lbh_top_others.golden", Table(lbhTable(lbhOthers(), 80), color)},
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

// The leaderboard keeps the strip-ANSI-equals-no-color invariant, mid-row bar
// included.
func TestLeaderboardStripANSIInvariant(t *testing.T) {
	for _, tab := range []view.Table{
		lbTable(lbSix(), view.Cost, 0, 80, synced15m),
		lbTable(lbSix(), view.Cost, 2, 80, synced15m),
		lbTable(lbOutlier(), view.Cost, 0, 120, synced15m),
		lbTable(nil, view.Cost, 0, 80, synced15m),
		lbhTable(lbhRanked(), 80),
	} {
		color := Table(tab, Colors{Enabled: true})
		plain := Table(tab, Colors{})
		if len(color) != len(plain) {
			t.Fatalf("line counts differ: %d vs %d", len(color), len(plain))
		}
		for i := range color {
			if StripANSI(color[i]) != plain[i] {
				t.Errorf("line %d: StripANSI(%q) = %q, want %q", i, color[i], StripANSI(color[i]), plain[i])
			}
		}
	}
}

// lbDeltaTable renders lbSix with the watch Prev map (B7): the delta arrow
// rides the metric cell AFTER the padded text (DeltaAfterPad); the bar budget
// reserves one column.
func lbDeltaTable(m view.Metric, width int) view.Table {
	return view.Leaderboard(lbSix(), view.LeaderboardOptions{
		Period:      query.Monthly,
		WindowLabel: "2026-09",
		DeltaLabel:  "Aug",
		Metric:      m,
		PinnedUser:  "sahil",
		Width:       width,
		LastSync:    "never",
		Prev: map[string]float64{
			"sahil":   12000,   // up
			"beatriz": 3400,    // down
			"carlos":  1598.54, // equal: no arrow
		},
	})
}

// lbZeroDeltaRow pins the exact-zero composite: the Dim wrap covers
// PadLeft("$0.00", w) + " " + arrow together (R13).
func lbZeroDeltaRow() []view.LeaderboardRow {
	return withShares([]view.LeaderboardRow{
		{Rank: 1, User: "sahil", Totals: fact.Totals{TotalCost: 10, TotalTokens: 100}},
		{Rank: 2, User: "frodo", Totals: fact.Totals{}},
	}, view.Cost)
}

func TestLeaderboardDeltaGoldens(t *testing.T) {
	color := Colors{Enabled: true}
	zero := view.Leaderboard(lbZeroDeltaRow(), view.LeaderboardOptions{
		Period: query.Monthly, WindowLabel: "2026-09", DeltaLabel: "Aug",
		Metric: view.Cost, Width: 80, LastSync: "never",
		Prev: map[string]float64{"frodo": -1}, // 0 > -1: up arrow on the zero row
	})
	cases := []struct {
		name   string
		golden string
		lines  []string
	}{
		{"cost color", "leaderboard_delta_color.golden", Table(lbDeltaTable(view.Cost, 80), color)},
		{"cost no-color", "leaderboard_delta_nocolor.golden", Table(lbDeltaTable(view.Cost, 80), Colors{})},
		{"tokens color", "leaderboard_delta_tokens.golden", Table(view.Leaderboard(lbSixTokens(), view.LeaderboardOptions{
			Period: query.Monthly, WindowLabel: "2026-09", DeltaLabel: "Aug",
			Metric: view.Tokens, PinnedUser: "sahil", Width: 80, LastSync: "never",
			Prev: map[string]float64{"sahil": 15000000000, "beatriz": 5600000000},
		}), color)},
		{"zero cost composite dim", "leaderboard_delta_zero.golden", Table(zero, color)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := strings.Join(c.lines, "\n") + "\n"
			path := filepath.Join("testdata", c.golden)
			if *update {
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
