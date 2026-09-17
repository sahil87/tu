package command

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/sahil87/tu/internal/config"
	"github.com/sahil87/tu/internal/fact"
	"github.com/sahil87/tu/internal/query"
	"github.com/sahil87/tu/internal/render/ansi"
	"github.com/sahil87/tu/internal/render/csv"
	renderjson "github.com/sahil87/tu/internal/render/json"
	"github.com/sahil87/tu/internal/render/markdown"
	"github.com/sahil87/tu/internal/view"
)

// leaderboardWindow is a resolved date window (ISO YYYY-MM-DD bounds, ""
// open) plus the label the heading / Δ header shows (the TS
// LeaderboardWindow).
type leaderboardWindow struct {
	start, end, label string
}

// shortMonths labels the monthly previous window (the TS SHORT_MONTHS).
var shortMonths = []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

// addDays is day arithmetic on date-only ISO labels, parsed as UTC midnight
// (the TS addDays — timezone-independent, like WeekLabel).
func addDays(iso string, days int) string {
	d, err := time.Parse("2006-01-02", iso)
	if err != nil {
		return iso
	}
	return d.AddDate(0, 0, days).Format("2006-01-02")
}

// diffDays is the whole-day distance a → b on UTC-parsed labels (the TS
// diffDays: Math.round over the millisecond difference).
func diffDays(a, b string) int {
	da, errA := time.Parse("2006-01-02", a)
	db, errB := time.Parse("2006-01-02", b)
	if errA != nil || errB != nil {
		return 0
	}
	return int(math.Round(db.Sub(da).Hours() / 24))
}

// leaderboardWindows computes the current and previous ranking windows (the
// TS currentWindow/previousWindow, intake §3). An explicit --since/--until
// REPLACES the period window (heading label "S → U" / "S →" / "→ U"); its
// previous window is the equal-length range ending the day before S (label
// "prev", nil when only --until is given or the length is < 1). Period
// windows anchor on now in LOCAL time; the previous window is the previous
// calendar day / Sunday-anchored week / calendar month (labelled with the
// English 3-letter month of its last day).
func leaderboardWindows(period query.Period, since, until string, now time.Time) (cur leaderboardWindow, prev *leaderboardWindow) {
	if since != "" || until != "" {
		switch {
		case since != "" && until != "":
			cur = leaderboardWindow{since, until, since + " → " + until}
		case since != "":
			cur = leaderboardWindow{since, "", since + " →"}
		default:
			cur = leaderboardWindow{"", until, "→ " + until}
		}
		if since == "" {
			return cur, nil
		}
		end := until
		if end == "" {
			end = query.CurrentLabel(query.Daily, now)
		}
		length := diffDays(since, end) + 1
		if length < 1 {
			return cur, nil
		}
		return cur, &leaderboardWindow{addDays(since, -length), addDays(since, -1), "prev"}
	}

	today := query.CurrentLabel(query.Daily, now)
	switch period {
	case query.Monthly:
		cur = leaderboardWindow{today[:7] + "-01", today, query.CurrentLabel(query.Monthly, now)}
		y, m, _ := now.Date()
		first := time.Date(y, m-1, 1, 0, 0, 0, 0, now.Location())
		last := time.Date(y, m, 0, 0, 0, 0, 0, now.Location())
		return cur, &leaderboardWindow{
			first.Format("2006-01-02"), last.Format("2006-01-02"),
			shortMonths[int(last.Month())-1],
		}
	case query.Weekly:
		sunday := query.CurrentLabel(query.Weekly, now)
		cur = leaderboardWindow{sunday, today, sunday}
		start := addDays(sunday, -7)
		return cur, &leaderboardWindow{start, addDays(sunday, -1), start}
	default:
		cur = leaderboardWindow{today, today, today}
		yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
		return cur, &leaderboardWindow{yesterday, yesterday, yesterday}
	}
}

// rankLeaderboard ranks the keys (user, or user/machine under --by-machine)
// over the current window (the TS sumByKey + buildLeaderboard, intake §3):
//
//   - The per-day collapse (Collapse over dims + Date) sums each key's tools
//     and machines per day in RECORD INPUT ORDER (the TS mergeEntries fold),
//     then SortByDate orders the days so the window sum folds ascending by
//     label — the JSON cost/share/delta doubles are byte surfaces (DC-10).
//   - Keys with TotalTokens == 0 && TotalCost == 0 in the window are dropped
//     (a key with previous-window data only is not in cur and drops too).
//   - Sort: descending by metricValue in the display metric, ties by key
//     ascending byte order (the TS string compare on ASCII keys).
//   - grand folds metricValue over the RANKED rows from 0 (order is
//     byte-visible in share); Share = value/grand, 0 when grand is 0.
//   - Delta = (value − prevValue)/prevValue when the previous-window value is
//     nonzero, else nil (rendered "new", JSON null, CSV empty).
func rankLeaderboard(raw []fact.Record, byMachine bool, cur leaderboardWindow, prev *leaderboardWindow, metric view.Metric) []view.LeaderboardRow {
	dims := []query.Dim{query.User}
	if byMachine {
		dims = append(dims, query.Machine)
	}
	daily := query.SortByDate(query.Collapse(raw, append(dims, query.Date)...))

	type keyed struct {
		key    string
		totals fact.Totals
	}
	kept := make([]keyed, 0)
	for _, g := range query.GroupBy(query.Window(daily, cur.start, cur.end), dims...) {
		if g.TotalTokens == 0 && g.TotalCost == 0 {
			continue
		}
		kept = append(kept, keyed{leaderboardGroupKey(g.Key, byMachine), g.Totals})
	}
	sort.Slice(kept, func(a, b int) bool {
		va := leaderboardMetricValue(kept[a].totals, metric)
		vb := leaderboardMetricValue(kept[b].totals, metric)
		if va != vb {
			return va > vb
		}
		return kept[a].key < kept[b].key
	})

	prevTotals := make(map[string]fact.Totals)
	if prev != nil {
		for _, g := range query.GroupBy(query.Window(daily, prev.start, prev.end), dims...) {
			prevTotals[leaderboardGroupKey(g.Key, byMachine)] = g.Totals
		}
	}

	grand := 0.0
	for _, k := range kept {
		grand += leaderboardMetricValue(k.totals, metric)
	}
	rows := make([]view.LeaderboardRow, len(kept))
	for i, k := range kept {
		value := leaderboardMetricValue(k.totals, metric)
		rows[i] = view.LeaderboardRow{Rank: i + 1, Totals: k.totals}
		if byMachine {
			slash := strings.Index(k.key, "/")
			rows[i].User, rows[i].Machine = k.key[:slash], k.key[slash+1:]
		} else {
			rows[i].User = k.key
		}
		if grand > 0 {
			rows[i].Share = value / grand
		}
		prevValue := 0.0
		if pt, ok := prevTotals[k.key]; ok {
			prevValue = leaderboardMetricValue(pt, metric)
		}
		if prevValue != 0 {
			d := (value - prevValue) / prevValue
			rows[i].Delta = &d
		}
	}
	return rows
}

// leaderboardGroupKey is a group's ranking key: the user, or "user/machine"
// under --by-machine (the TS map key).
func leaderboardGroupKey(key fact.Record, byMachine bool) string {
	if byMachine {
		return key.User + "/" + key.Machine
	}
	return key.User
}

// leaderboardMetricValue is the Totals field the leaderboard ranks, shares
// and deltas in (the TS metricValue): cost, or tokens under -t.
func leaderboardMetricValue(t fact.Totals, m view.Metric) float64 {
	if m == view.Tokens {
		return float64(t.TotalTokens)
	}
	return t.TotalCost
}

// runLeaderboard is the lb path (intake §11): the explicit window replaces
// the period window; the ranked list is computed in full and --top slices
// what each format EMITS (shares, the Total row and the Result totals cover
// the full set). Repo-only: raw is gatherAllUsers' output; no fetch, no
// warnings.
func runLeaderboard(req Request, cfg config.Config, raw []fact.Record, notices []string, deps Deps) Result {
	cur, prev := leaderboardWindows(req.Period, req.Flags.Since, req.Flags.Until, deps.Now())
	metric := view.Cost
	if req.Flags.Metric == Tokens {
		metric = view.Tokens
	}
	rows := rankLeaderboard(raw, req.Flags.ByMachine, cur, prev, metric)
	deltaLabel := "prev"
	if prev != nil {
		deltaLabel = prev.label
	}

	sliced := rows
	if req.Flags.Top > 0 && req.Flags.Top < len(rows) {
		sliced = rows[:req.Flags.Top]
	}

	res := Result{Notices: notices, CostByItem: make(map[string]float64, len(rows))}
	for _, r := range rows {
		res.TotalCost += r.TotalCost
		res.TotalTokens += r.TotalTokens
		res.CostByItem[leaderboardRowKey(r)] = leaderboardMetricValue(r.Totals, metric)
	}

	switch req.Format {
	case JSON:
		res.Lines = renderjson.Leaderboard(sliced)
	case CSV:
		res.Lines = csv.Leaderboard(sliced, rows, req.Flags.ByMachine)
	case Markdown:
		res.Lines = markdown.Leaderboard(sliced, rows, req.Period, deltaLabel, req.Flags.ByMachine)
	default:
		pinned := req.Flags.User
		if pinned == "" {
			pinned = cfg.User
		}
		res.Lines = ansi.Table(view.Leaderboard(rows, view.LeaderboardOptions{
			Period:      req.Period,
			WindowLabel: cur.label,
			DeltaLabel:  deltaLabel,
			Metric:      metric,
			PinnedUser:  pinned,
			Top:         req.Flags.Top,
			Width:       deps.Width,
			LastSync:    deps.lastSync(),
			Prev:        deps.live().Prev,
		}), deps.Colors)
	}
	return res
}

// leaderboardRowKey is a row's CostByItem/watch-map key (the TS
// leaderboardPrevMap): the user, or user/machine under --by-machine.
func leaderboardRowKey(r view.LeaderboardRow) string {
	if r.Machine != "" {
		return r.User + "/" + r.Machine
	}
	return r.User
}

// runLeaderboardHistory is the lbh path (intake §11): one view.Series per
// Repo.Users() profile, in that order, each entry a period label — the value
// per (user, label) is the LEFT FOLD in record input order over the
// windowed, relabelled records (tool-major, walk order within a tool: the TS
// aggregateMachineMap aggregates each tool's unmerged machine-major entries
// and sumLeaderboardToolMaps adds the tools in registry order) — NOT the main
// history's collapse-then-roll-up. Entries sort ascending by label after
// grouping (the TS mergeEntries sort); a user with no records in the window
// yields an empty series (kept — KeepAllColumns). Repo-only: no fetch, no
// warnings.
func runLeaderboardHistory(req Request, cfg config.Config, raw []fact.Record, notices []string, capActive bool, deps Deps) Result {
	metric := view.Cost
	if req.Flags.Metric == Tokens {
		metric = view.Tokens
	}
	series := make([]view.Series, 0)
	for _, u := range deps.Repo.Users() {
		recs := query.ByUser(raw, u) // gather order preserved
		groups := query.GroupBy(query.Relabel(query.Window(recs, req.Flags.Since, req.Flags.Until), req.Period), query.Date)
		entries := make([]view.Entry, len(groups))
		for i, g := range groups {
			entries[i] = view.Entry{Label: g.Key.Date, Totals: g.Totals}
		}
		sort.SliceStable(entries, func(a, b int) bool { return entries[a].Label < entries[b].Label })
		series = append(series, view.Series{Name: u, Entries: entries})
	}
	if req.Flags.Top > 0 {
		series = foldColumns(series, req.Flags.Top, metric)
	}

	// Result stats as runHistory computes them (the TS buildCostMap over the
	// folded map): {user}:{label} and total:{label} in the display metric.
	res := Result{Notices: notices, CostByItem: make(map[string]float64)}
	for _, s := range series {
		for _, e := range s.Entries {
			res.TotalCost += e.TotalCost
			res.TotalTokens += e.TotalTokens
			v := leaderboardMetricValue(e.Totals, metric)
			res.CostByItem[s.Name+":"+e.Label] = v
			res.CostByItem["total:"+e.Label] += v
		}
	}

	base := "Leaderboard History"
	if metric == view.Tokens {
		base = "Leaderboard Token History"
	}
	switch req.Format {
	case JSON:
		res.Lines = renderjson.TotalHistory(series)
	case CSV:
		res.Lines = csv.TotalHistory(series)
	case Markdown:
		res.Lines = markdown.TotalHistory(series, req.Period, capActive, base)
	default:
		// lbh carries Prev (the delta map) but no MaxRows (the TS
		// lbhFormatOptions never carries it) and no compact form.
		res.Lines = ansi.Table(view.TotalHistory(series, view.HistoryOptions{
			Period:          req.Period,
			Now:             deps.Now(),
			Width:           deps.Width,
			CapActive:       capActive,
			Metric:          metric,
			Prev:            deps.live().Prev,
			Title:           "📊 " + base + " (" + view.PeriodLabel(req.Period, capActive) + ")",
			RankColumns:     true,
			HighlightLeader: true,
			KeepAllColumns:  true,
		}), deps.Colors)
	}
	return res
}

// foldColumns is the lbh --top fold (the TS foldLeaderboardColumns): the
// series total is the left fold of the metric values over its entries
// (ascending) from 0; a stable descending sort picks the top N names; the
// kept series stay in their ORIGINAL order; the folded series' entries
// concatenate in original order and re-group per label (left fold in that
// order, sorted by label) into one "others" series appended LAST. n ≥ len
// folds nothing; an "others" with no entries is not appended.
func foldColumns(series []view.Series, top int, metric view.Metric) []view.Series {
	if top >= len(series) {
		return series
	}
	totals := make([]float64, len(series))
	for i, s := range series {
		for _, e := range s.Entries {
			totals[i] += leaderboardMetricValue(e.Totals, metric)
		}
	}
	order := make([]int, len(series))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return totals[order[a]] > totals[order[b]] })
	keep := make(map[int]bool, top)
	for _, i := range order[:top] {
		keep[i] = true
	}
	out := make([]view.Series, 0, top+1)
	var folded []fact.Record
	for i, s := range series {
		if keep[i] {
			out = append(out, s)
			continue
		}
		for _, e := range s.Entries {
			folded = append(folded, fact.Record{Date: e.Label, Totals: e.Totals})
		}
	}
	if len(folded) == 0 {
		return out
	}
	groups := query.GroupBy(folded, query.Date)
	entries := make([]view.Entry, len(groups))
	for i, g := range groups {
		entries[i] = view.Entry{Label: g.Key.Date, Totals: g.Totals}
	}
	sort.SliceStable(entries, func(a, b int) bool { return entries[a].Label < entries[b].Label })
	return append(out, view.Series{Name: "others", Entries: entries})
}
