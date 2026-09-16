package command

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sahil87/tu/internal/fact"
	"github.com/sahil87/tu/internal/query"
	"github.com/sahil87/tu/internal/view"
)

// lbNow is the R4 GIVEN: 2026-09-17 (a Thursday) local noon UTC.
var lbNow = time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)

// R4: the six window rows.
func TestLeaderboardWindows(t *testing.T) {
	cases := []struct {
		name     string
		period   query.Period
		since    string
		until    string
		wantCur  leaderboardWindow
		wantPrev *leaderboardWindow
	}{
		{"daily no bounds", query.Daily, "", "",
			leaderboardWindow{"2026-09-17", "2026-09-17", "2026-09-17"},
			&leaderboardWindow{"2026-09-16", "2026-09-16", "2026-09-16"}},
		{"weekly no bounds", query.Weekly, "", "",
			leaderboardWindow{"2026-09-13", "2026-09-17", "2026-09-13"},
			&leaderboardWindow{"2026-09-06", "2026-09-12", "2026-09-06"}},
		{"monthly no bounds", query.Monthly, "", "",
			leaderboardWindow{"2026-09-01", "2026-09-17", "2026-09"},
			&leaderboardWindow{"2026-08-01", "2026-08-31", "Aug"}},
		{"both bounds", query.Monthly, "2026-01-01", "2026-01-31",
			leaderboardWindow{"2026-01-01", "2026-01-31", "2026-01-01 → 2026-01-31"},
			&leaderboardWindow{"2025-12-01", "2025-12-31", "prev"}},
		{"since only", query.Daily, "2026-01-01", "",
			leaderboardWindow{"2026-01-01", "", "2026-01-01 →"},
			&leaderboardWindow{"2025-04-16", "2025-12-31", "prev"}},
		{"until only: nil previous, every row new (DC-13)", query.Daily, "", "2026-01-31",
			leaderboardWindow{"", "2026-01-31", "→ 2026-01-31"},
			nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cur, prev := leaderboardWindows(c.period, c.since, c.until, lbNow)
			if cur != c.wantCur {
				t.Errorf("cur = %+v, want %+v", cur, c.wantCur)
			}
			if c.wantPrev == nil {
				if prev != nil {
					t.Errorf("prev = %+v, want nil", prev)
				}
				return
			}
			if prev == nil || *prev != *c.wantPrev {
				t.Errorf("prev = %+v, want %+v", prev, *c.wantPrev)
			}
		})
	}
}

// R4/A-025: a --since after today (or any reversed/empty range) yields
// length < 1 → a nil previous window.
func TestLeaderboardWindowsSinceAfterToday(t *testing.T) {
	cur, prev := leaderboardWindows(query.Daily, "2026-12-01", "", lbNow)
	if cur.label != "2026-12-01 →" {
		t.Errorf("cur label = %q", cur.label)
	}
	if prev != nil {
		t.Errorf("prev = %+v, want nil (length < 1)", prev)
	}
}

// The monthly previous window across the year boundary: January's prev is
// December of the previous year (time.Date normalizes the month underflow).
func TestLeaderboardWindowsMonthlyYearBoundary(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	cur, prev := leaderboardWindows(query.Monthly, "", "", now)
	if cur != (leaderboardWindow{"2026-01-01", "2026-01-15", "2026-01"}) {
		t.Errorf("cur = %+v", cur)
	}
	if prev == nil || *prev != (leaderboardWindow{"2025-12-01", "2025-12-31", "Dec"}) {
		t.Errorf("prev = %+v, want 2025-12-01/2025-12-31 Dec", prev)
	}
}

// The daily/weekly anchors are local-time (the harness tz axis): the same
// instant in +05:30 is already the next day.
func TestLeaderboardWindowsLocalAnchors(t *testing.T) {
	ist := time.FixedZone("IST", 5*3600+1800)
	now := time.Date(2026, 9, 17, 1, 0, 0, 0, ist) // 2026-09-16 19:30 UTC
	cur, prev := leaderboardWindows(query.Daily, "", "", now)
	if cur.start != "2026-09-17" || prev.start != "2026-09-16" {
		t.Errorf("daily window = %+v / %+v, want local-day anchors", cur, prev)
	}
}

// ── B5: ranking and the lb run path ─────────────────────────────────────────

// R5: the seed-derived window scenario — harness-user $1.70 / 97,600 tokens,
// other-user $1.30 / 48,800; shares 56.7%/43.3%; both deltas nil; Total
// $3.00 / 146,400. No ccusage call (the leaderboard is repo-only).
func TestRunLeaderboardSeedWindow(t *testing.T) {
	f := &fakeFetcher{byTool: liveCorpus()}
	repo := seedRepo()
	res, err := Run(context.Background(), Request{
		Display: Leaderboard,
		Flags:   Flags{Since: "2026-01-01", Until: "2026-01-31", Interval: 10},
	}, multiCfg, multiDeps(f, repo))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 0 {
		t.Errorf("fetch calls = %+v, want none on the repo-only leaderboard", f.calls)
	}
	if len(res.Warnings) != 0 || len(res.Notices) != 0 {
		t.Errorf("Warnings = %v, Notices = %v, want both empty", res.Warnings, res.Notices)
	}
	joined := strings.Join(res.Lines, "\n")
	for _, want := range []string{
		"Leaderboard (daily) · 2026-01-01 → 2026-01-31 · by cost",
		"1 | harness-user ◂",
		"2 | other-user",
		"$1.70", "$1.30",
		"56.7%", "43.3%", "new",
		"97,600", "48,800",
		"$3.00", "146,400",
		"never synced · tu sync to refresh",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("lb lines missing %q:\n%s", want, joined)
		}
	}
	if res.TotalCost != 3.0 || res.TotalTokens != 146400 {
		t.Errorf("stats = %v / %v, want 3.0 / 146400", res.TotalCost, res.TotalTokens)
	}
	if res.CostByItem["harness-user"] != 1.7 || res.CostByItem["other-user"] != 1.3 {
		t.Errorf("CostByItem = %v", res.CostByItem)
	}
}

// R5: ties break by key ascending byte order — the three-way 48,800-token tie
// under --by-machine -t ranks harness-user/harness-machine,
// harness-user/other-box, other-user/laptop.
func TestRunLeaderboardByMachineTokenTie(t *testing.T) {
	repo := seedRepo()
	res, err := Run(context.Background(), Request{
		Display: Leaderboard,
		Flags:   Flags{ByMachine: true, Metric: Tokens, Since: "2026-01-01", Until: "2026-01-31", Interval: 10},
	}, multiCfg, multiDeps(&fakeFetcher{}, repo))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(res.Lines, "\n")
	i1 := strings.Index(joined, "harness-user/harness-machine ◂")
	i2 := strings.Index(joined, "harness-user/other-box ◂")
	i3 := strings.Index(joined, "other-user/laptop")
	if i1 < 0 || i2 < 0 || i3 < 0 || !(i1 < i2 && i2 < i3) {
		t.Errorf("tie order wrong (idx %d, %d, %d):\n%s", i1, i2, i3, joined)
	}
	if !strings.Contains(joined, "by tokens") {
		t.Errorf("missing the by-tokens heading:\n%s", joined)
	}
	// CostByItem keyed by user/machine, valued in the display metric.
	if res.CostByItem["harness-user/harness-machine"] != 48800 {
		t.Errorf("CostByItem = %v", res.CostByItem)
	}
}

// R5: keys with zero tokens AND zero cost in the window drop; a key with only
// previous-window data drops too (it is not in cur).
func TestRunLeaderboardDropRule(t *testing.T) {
	repo := &fakeRepo{
		users: []string{"harness-user", "ghost", "other-user"},
		recs: map[string]map[string][]fact.Record{
			"harness-user": {"cc": {storedRec("harness-user", "m", "2026-01-05", "cc", 1.0)}},
			// ghost's record is zero on BOTH tokens and cost in the window.
			"ghost":      {"cc": {{Date: "2026-01-05", Tool: "cc", User: "ghost", Machine: "m"}}},
			"other-user": {"cc": {storedRec("other-user", "m", "2025-12-05", "cc", 2.0)}},
		},
	}
	res, err := Run(context.Background(), Request{
		Display: Leaderboard,
		Flags:   Flags{Since: "2026-01-01", Until: "2026-01-31", Interval: 10},
	}, multiCfg, multiDeps(&fakeFetcher{}, repo))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(res.Lines, "\n")
	if strings.Contains(joined, "ghost") {
		t.Errorf("a zero window key must drop:\n%s", joined)
	}
	if strings.Contains(joined, "other-user") {
		t.Errorf("a previous-window-only key must drop:\n%s", joined)
	}
	if !strings.Contains(joined, "harness-user") {
		t.Errorf("the kept row missing:\n%s", joined)
	}
}

// R5/A-019: the per-day collapse sums in record input order and the window
// sum folds ascending by date — 0.1 + (0.2 + 0.3) = 0.6, never
// (0.1 + 0.2) + 0.3 = 0.6000000000000001.
func TestRunLeaderboardAssociation(t *testing.T) {
	a, b, c := 0.1, 0.2, 0.3
	if a+(b+c) == (a+b)+c {
		t.Fatal("the test values do not distinguish the summation orders")
	}
	repo := &fakeRepo{
		users: []string{"harness-user"},
		recs: map[string]map[string][]fact.Record{
			"harness-user": {"cc": {
				storedRec("harness-user", "machine-a", "2026-01-05", "cc", a),
				storedRec("harness-user", "machine-a", "2026-01-06", "cc", b),
				storedRec("harness-user", "machine-b", "2026-01-06", "cc", c),
			}},
		},
	}
	res, err := Run(context.Background(), Request{
		Display: Leaderboard, Format: JSON,
		Flags: Flags{Since: "2026-01-01", Until: "2026-01-31", Interval: 10},
	}, multiCfg, multiDeps(&fakeFetcher{}, repo))
	if err != nil {
		t.Fatal(err)
	}
	if joined := strings.Join(res.Lines, "\n"); !strings.Contains(joined, `"cost": 0.6,`) {
		t.Errorf("the window cost must associate per-day-first (0.6):\n%s", joined)
	}
}

// R5: the delta rule — nil when the key is absent from the previous window or
// the previous value is exactly 0; (value − prev)/prev otherwise. --until
// alone yields a nil previous window and every row "new" (DC-13).
func TestRunLeaderboardDelta(t *testing.T) {
	repo := &fakeRepo{
		users: []string{"harness-user", "other-user", "zero-prev"},
		recs: map[string]map[string][]fact.Record{
			"harness-user": {"cc": {
				storedRec("harness-user", "m", "2026-01-05", "cc", 2.0),
				storedRec("harness-user", "m", "2025-12-05", "cc", 1.0),
			}},
			"other-user": {"cc": {storedRec("other-user", "m", "2026-01-05", "cc", 1.0)}},
			"zero-prev": {"cc": {
				storedRec("zero-prev", "m", "2026-01-05", "cc", 0.5),
				{Date: "2025-12-05", Tool: "cc", User: "zero-prev", Machine: "m",
					Totals: fact.Totals{TotalCost: 0, TotalTokens: 24400}},
			}},
		},
	}
	res, err := Run(context.Background(), Request{
		Display: Leaderboard, Format: JSON,
		Flags: Flags{Since: "2026-01-01", Until: "2026-01-31", Interval: 10},
	}, multiCfg, multiDeps(&fakeFetcher{}, repo))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(res.Lines, "\n")
	if !strings.Contains(joined, `"delta": 1`) { // (2.0 − 1.0) / 1.0
		t.Errorf("harness-user delta = 1:\n%s", joined)
	}
	if !strings.Contains(joined, `"user": "other-user",`) || !strings.Contains(joined, `"delta": null`) {
		t.Errorf("other-user (no prev) must carry delta null:\n%s", joined)
	}
	// zero-prev has a previous record whose cost is exactly 0 → null.
	zi := strings.Index(joined, `"user": "zero-prev",`)
	if zi < 0 || !strings.Contains(joined[zi:], `"delta": null`) {
		t.Errorf("zero-prev must carry delta null:\n%s", joined[zi:])
	}

	// --until only: every row new, heading "→ {until}".
	res, err = Run(context.Background(), Request{
		Display: Leaderboard,
		Flags:   Flags{Until: "2026-01-31", Interval: 10},
	}, multiCfg, multiDeps(&fakeFetcher{}, seedRepo()))
	if err != nil {
		t.Fatal(err)
	}
	joined = strings.Join(res.Lines, "\n")
	if !strings.Contains(joined, "· → 2026-01-31 ·") {
		t.Errorf("missing the until-only heading:\n%s", joined)
	}
	if strings.Contains(joined, "%") && !strings.Contains(joined, "new") {
		t.Errorf("until-only rows must all be new:\n%s", joined)
	}
}

// R5/R13: --top slices what is rendered; shares, the Total row and the Result
// totals cover the full set.
func TestRunLeaderboardTop(t *testing.T) {
	res, err := Run(context.Background(), Request{
		Display: Leaderboard,
		Flags:   Flags{Top: 1, Since: "2026-01-01", Until: "2026-01-31", Interval: 10},
	}, multiCfg, multiDeps(&fakeFetcher{}, seedRepo()))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(res.Lines, "\n")
	for _, want := range []string{"… +1 others", "$3.00", "harness-user ◂"} {
		if !strings.Contains(joined, want) {
			t.Errorf("--top 1 lines missing %q:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "2 | other-user") {
		t.Errorf("the sliced row must not render:\n%s", joined)
	}
	if res.TotalCost != 3.0 {
		t.Errorf("TotalCost = %v, want the full-set 3.0", res.TotalCost)
	}
}

// R13: -u <name> pins (the ◂ marker), never filters; -u all is a silent
// no-op (cleared by Normalize step 0'); both still rank every user.
func TestRunLeaderboardUserPin(t *testing.T) {
	window := Flags{Since: "2026-01-01", Until: "2026-01-31", Interval: 10}

	res, err := Run(context.Background(), Request{Display: Leaderboard, Flags: func() Flags {
		f := window
		f.User = "other-user"
		return f
	}()}, multiCfg, multiDeps(&fakeFetcher{}, seedRepo()))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(res.Lines, "\n")
	if !strings.Contains(joined, "other-user ◂") || strings.Contains(joined, "harness-user ◂") {
		t.Errorf("-u other-user must pin other-user:\n%s", joined)
	}

	res, err = Run(context.Background(), Request{Display: Leaderboard, Flags: func() Flags {
		f := window
		f.User = "all"
		return f
	}()}, multiCfg, multiDeps(&fakeFetcher{}, seedRepo()))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Notices) != 0 {
		t.Errorf("-u all on lb emits no notice: %v", res.Notices)
	}
	joined = strings.Join(res.Lines, "\n")
	if !strings.Contains(joined, "harness-user ◂") {
		t.Errorf("-u all falls back to the config user's pin:\n%s", joined)
	}
}

// R13: the lb formats route through the encoders.
func TestRunLeaderboardFormats(t *testing.T) {
	window := Flags{Since: "2026-01-01", Until: "2026-01-31", Interval: 10}

	t.Run("json", func(t *testing.T) {
		res, err := Run(context.Background(), Request{Display: Leaderboard, Format: JSON, Flags: window}, multiCfg, multiDeps(&fakeFetcher{}, seedRepo()))
		if err != nil {
			t.Fatal(err)
		}
		joined := strings.Join(res.Lines, "\n")
		for _, want := range []string{
			`"rank": 1,`, `"user": "harness-user",`, `"cost": 1.7,`,
			`"totalTokens": 97600,`, `"delta": null`,
		} {
			if !strings.Contains(joined, want) {
				t.Errorf("lb --json missing %q:\n%s", want, joined)
			}
		}
		// Shares are raw doubles: 1.7 / 3.0 in the ranked fold order.
		if !strings.Contains(joined, `"share": 0.5666666666666667,`) {
			t.Errorf("share double missing:\n%s", joined)
		}
	})

	t.Run("csv", func(t *testing.T) {
		res, err := Run(context.Background(), Request{Display: Leaderboard, Format: CSV, Flags: window}, multiCfg, multiDeps(&fakeFetcher{}, seedRepo()))
		if err != nil {
			t.Fatal(err)
		}
		want := []string{
			"rank,user,cost,total_tokens,share,delta",
			"1,harness-user,1.70,97600,0.567,",
			"2,other-user,1.30,48800,0.433,",
			"Total,,3.00,146400,,",
		}
		if !reflect.DeepEqual(res.Lines, want) {
			t.Errorf("csv = %q, want %q", res.Lines, want)
		}
	})

	t.Run("markdown", func(t *testing.T) {
		res, err := Run(context.Background(), Request{Display: Leaderboard, Format: Markdown, Flags: window}, multiCfg, multiDeps(&fakeFetcher{}, seedRepo()))
		if err != nil {
			t.Fatal(err)
		}
		if res.Lines[0] != "## Leaderboard (daily)" {
			t.Errorf("md heading = %q", res.Lines[0])
		}
		joined := strings.Join(res.Lines, "\n")
		if !strings.Contains(joined, "| 1 | harness-user | $1.70 | 97,600 | 56.7% | new |") {
			t.Errorf("md row missing:\n%s", joined)
		}
	})

	t.Run("by-machine json carries machine", func(t *testing.T) {
		res, err := Run(context.Background(), Request{
			Display: Leaderboard, Format: JSON,
			Flags: Flags{ByMachine: true, Since: "2026-01-01", Until: "2026-01-31", Interval: 10},
		}, multiCfg, multiDeps(&fakeFetcher{}, seedRepo()))
		if err != nil {
			t.Fatal(err)
		}
		joined := strings.Join(res.Lines, "\n")
		if !strings.Contains(joined, `"user": "harness-user",`+"\n"+`    "machine": "harness-machine",`) {
			t.Errorf("machine key after user missing:\n%s", joined)
		}
	})
}

// ── B5: the lbh run path and the --top column fold ──────────────────────────

// R13: the m lbh seed scenario — one 2026-01 row, columns ranked
// harness-user ($1.70) then other-user ($1.30), the leader cell bold, the row
// total $3.00, no Total row (one label), the Leaderboard History title.
func TestRunLeaderboardHistorySeed(t *testing.T) {
	f := &fakeFetcher{byTool: liveCorpus()}
	res, err := Run(context.Background(), Request{
		Display: LeaderboardHistory, Period: query.Monthly,
		Flags: Flags{Interval: 10},
	}, multiCfg, multiDeps(f, seedRepo()))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 0 {
		t.Errorf("fetch calls = %+v, want none on the repo-only lbh", f.calls)
	}
	joined := strings.Join(res.Lines, "\n")
	for _, want := range []string{
		"\x1b[1;37m📊 Leaderboard History (monthly)\x1b[0m",
		"harness-user", "other-user",
		"2026-01", "$1.70", "$1.30", "$3.00",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("m lbh missing %q:\n%s", want, joined)
		}
	}
	// Ranked columns: harness-user before other-user; the leader cell (the
	// row's max) is boldWhite-wrapped.
	if strings.Index(joined, "harness-user") > strings.Index(joined, "other-user") {
		t.Errorf("columns must rank by window total:\n%s", joined)
	}
	if !strings.Contains(joined, "\x1b[1;37m       $1.70\x1b[0m") {
		t.Errorf("the leader cell must be boldWhite:\n%s", joined)
	}
	if strings.Contains(joined, "Total") {
		t.Errorf("one label has no Total row:\n%s", joined)
	}
	if res.TotalCost != 3.0 || res.TotalTokens != 146400 {
		t.Errorf("stats = %v / %v", res.TotalCost, res.TotalTokens)
	}
	if res.CostByItem["harness-user:2026-01"] != 1.7 || res.CostByItem["total:2026-01"] != 3.0 {
		t.Errorf("CostByItem = %v", res.CostByItem)
	}
}

// R13: m lbh --top 1 folds other-user into others — JSON keys in
// Repo.Users() order with others LAST; the ANSI columns rank by total.
func TestRunLeaderboardHistoryTopFold(t *testing.T) {
	res, err := Run(context.Background(), Request{
		Display: LeaderboardHistory, Period: query.Monthly, Format: JSON,
		Flags: Flags{Top: 1, Interval: 10},
	}, multiCfg, multiDeps(&fakeFetcher{}, seedRepo()))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(res.Lines, "\n")
	hi := strings.Index(joined, `"harness-user"`)
	oi := strings.Index(joined, `"others"`)
	if hi < 0 || oi < 0 || hi > oi {
		t.Errorf("JSON keys must be harness-user then others:\n%s", joined)
	}
	if strings.Contains(joined, "other-user") {
		t.Errorf("the folded user must not appear:\n%s", joined)
	}
	if !strings.Contains(joined, `"totalCost": 1.3,`) {
		t.Errorf("others folds the dropped user's entries:\n%s", joined)
	}

	// Top ≥ the user count folds nothing.
	res, err = Run(context.Background(), Request{
		Display: LeaderboardHistory, Period: query.Monthly, Format: JSON,
		Flags: Flags{Top: 5, Interval: 10},
	}, multiCfg, multiDeps(&fakeFetcher{}, seedRepo()))
	if err != nil {
		t.Fatal(err)
	}
	joined = strings.Join(res.Lines, "\n")
	if strings.Contains(joined, "others") || !strings.Contains(joined, `"other-user"`) {
		t.Errorf("Top ≥ users must be a no-op:\n%s", joined)
	}
}

// R13: the kept series stay in ORIGINAL order (not rank order) — ranking is
// the view's job (RankColumns); others appends last.
func TestFoldColumnsKeepsOriginalOrder(t *testing.T) {
	entry := func(label string, cost float64) view.Entry {
		return view.Entry{Label: label, Totals: fact.Totals{TotalCost: cost, TotalTokens: 1}}
	}
	series := []view.Series{
		{Name: "a", Entries: []view.Entry{entry("2026-01", 2)}},
		{Name: "b", Entries: []view.Entry{entry("2026-01", 1)}},
		{Name: "c", Entries: []view.Entry{entry("2026-01", 3)}},
	}
	out := foldColumns(series, 2, view.Cost)
	names := []string{out[0].Name, out[1].Name, out[2].Name}
	if !reflect.DeepEqual(names, []string{"a", "c", "others"}) {
		t.Errorf("fold order = %v, want [a c others]", names)
	}
	if out[2].Entries[0].TotalCost != 1 {
		t.Errorf("others = %+v", out[2])
	}
}

// R13/A-020: the lbh value per (user, label) is the tool-major left fold over
// the windowed, relabelled records — ((m1_d5 + m1_d6) + m2_d6) + codex =
// 1.4000000000000001, NOT the main history's collapse-then-roll-up
// (m1_d5 + (m1_d6 + m2_d6)) + codex = 1.4.
func TestRunLeaderboardHistoryAssociation(t *testing.T) {
	a, b, c, d := 0.1, 0.2, 0.3, 0.8
	if ((a+b)+c)+d == (a+(b+c))+d {
		t.Fatal("the test values do not distinguish the summation orders")
	}
	repo := &fakeRepo{
		users: []string{"harness-user"},
		recs: map[string]map[string][]fact.Record{
			"harness-user": {
				"cc": {
					storedRec("harness-user", "machine-a", "2026-01-05", "cc", a),
					storedRec("harness-user", "machine-a", "2026-01-06", "cc", b),
					storedRec("harness-user", "machine-b", "2026-01-06", "cc", c),
				},
				"codex": {storedRec("harness-user", "machine-b", "2026-01-06", "codex", d)},
			},
		},
	}
	res, err := Run(context.Background(), Request{
		Display: LeaderboardHistory, Period: query.Monthly, Format: JSON,
		Flags: Flags{Interval: 10},
	}, multiCfg, multiDeps(&fakeFetcher{}, repo))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(res.Lines, "\n")
	if !strings.Contains(joined, `"totalCost": 1.4000000000000001,`) {
		t.Errorf("lbh must associate tool-major (1.4000000000000001):\n%s", joined)
	}
	if strings.Contains(joined, `"totalCost": 1.4,`) {
		t.Errorf("collapse-then-roll-up association leaked:\n%s", joined)
	}
}

// R13: the per-user entries sort ascending by label after grouping (the TS
// mergeEntries/aggregateMonthly sort), even when the walk order interleaves.
func TestRunLeaderboardHistoryEntryOrder(t *testing.T) {
	repo := &fakeRepo{
		users: []string{"harness-user"},
		recs: map[string]map[string][]fact.Record{
			"harness-user": {"cc": {
				storedRec("harness-user", "machine-a", "2026-01-07", "cc", 0.1),
				storedRec("harness-user", "machine-a", "2026-01-05", "cc", 0.2),
			}},
		},
	}
	res, err := Run(context.Background(), Request{
		Display: LeaderboardHistory, Period: query.Daily, Format: JSON,
		Flags: Flags{Full: true, Interval: 10},
	}, multiCfg, multiDeps(&fakeFetcher{}, repo))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(res.Lines, "\n")
	if strings.Index(joined, "2026-01-05") > strings.Index(joined, "2026-01-07") {
		t.Errorf("entries must sort ascending by label:\n%s", joined)
	}
}

// R13: a user with no records in the window keeps an (empty) column —
// KeepAllColumns; the lbh cap applies to daily/weekly (heading hint), and
// --full lifts it.
func TestRunLeaderboardHistoryEmptyAndCap(t *testing.T) {
	res, err := Run(context.Background(), Request{
		Display: LeaderboardHistory, Flags: Flags{Interval: 10},
	}, multiCfg, multiDeps(&fakeFetcher{}, seedRepo()))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(res.Lines, "\n")
	// historyNow is 2026-09-16: the cap floors at 2026-07-01 and the January
	// seed falls outside → the capped empty state.
	if !strings.Contains(joined, "📊 Leaderboard History (daily, last 3 months)") || !strings.Contains(joined, "  No data") {
		t.Errorf("capped lbh must render the hinted empty state:\n%s", joined)
	}

	// An in-window window shows both users even when one has nothing.
	res, err = Run(context.Background(), Request{
		Display: LeaderboardHistory, Period: query.Weekly,
		Flags: Flags{Since: "2026-01-01", Until: "2026-01-31", Interval: 10},
	}, multiCfg, multiDeps(&fakeFetcher{}, &fakeRepo{
		users: []string{"harness-user", "quiet-user"},
		recs: map[string]map[string][]fact.Record{
			"harness-user": {"cc": {storedRec("harness-user", "m", "2026-01-05", "cc", 0.5)}},
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	joined = strings.Join(res.Lines, "\n")
	if !strings.Contains(joined, "quiet-user") {
		t.Errorf("an empty user stays a column (KeepAllColumns):\n%s", joined)
	}
	if !strings.Contains(joined, "$0.00") {
		t.Errorf("the empty user's cells render dim $0.00:\n%s", joined)
	}
}
