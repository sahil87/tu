package command

import (
	"context"
	"errors"
	"time"

	"github.com/sahil87/tu/internal/config"
	"github.com/sahil87/tu/internal/fact"
	"github.com/sahil87/tu/internal/query"
	"github.com/sahil87/tu/internal/render/ansi"
	"github.com/sahil87/tu/internal/render/csv"
	renderjson "github.com/sahil87/tu/internal/render/json"
	"github.com/sahil87/tu/internal/render/markdown"
	"github.com/sahil87/tu/internal/source"
	"github.com/sahil87/tu/internal/view"
)

// Fetcher is what command needs from a source. *ccusage.Source satisfies it
// (asserted where cmd/tu assigns it).
type Fetcher interface {
	Fetch(ctx context.Context, tool fact.Tool, period string, extraArgs []string, fresh bool) ([]fact.Record, *source.Error)
	FetchAll(ctx context.Context, period string, extraArgs []string, fresh bool) ([]fact.Record, []*source.Error)
}

// Repo is what command needs from the metrics repo: the profile list and one
// user's records for one tool. metrics.Source satisfies it (asserted where
// cmd/tu assigns it). Distinct from Fetcher on purpose — a repo read has no
// context, no period, no cache, no fresh flag and no error channel.
type Repo interface {
	Users() []string
	Read(user string, tool fact.Tool) []fact.Record
}

// Deps are Run's external inputs.
type Deps struct {
	Source Fetcher
	Repo   Repo             // the metrics clone; consulted only in multi mode
	Now    func() time.Time // time.Now at the edge; fixed in tests
	Colors ansi.Colors
	Width  int // stdout TTY width, probed at the edge (80 when piped)
}

// Result is what Run returns; cmd/tu writes it.
type Result struct {
	Lines       []string           // stdout, one per line, no trailing "\n" per element
	Notices     []string           // guard warnings; the edge writes them BEFORE Warnings
	Warnings    []*source.Error    // the edge writes them with source.WriteWarnings
	TotalCost   float64            // sum over the rendered rows (watch stats)
	TotalTokens int64              // same
	CostByItem  map[string]float64 // snapshot: "{Name}"; history: "{Name}:{label}" and "total:{label}", valued in the display metric
}

// ErrUnported marks a recognized-but-unported request; cmd/tu maps it to the
// scaffold's placeholder line, exit 1.
var ErrUnported = errors.New("not implemented")

// inScope reports whether the (normalized) request is in the ported grammar
// (intake §2): snapshot or history display, any of the four formats, single
// or multi mode, -u in any mode (Normalize already cleared it for single),
// --by-machine on the snapshots and the single-tool history (Normalize already
// cleared it on the all-tools pivot), no non-data command, no version;
// Since/Until/Full are history flags (the guards clear them on snapshots).
// Still out: --top, watch, sync, dry-run, no-rain, skip-brew-update, and the
// leaderboard displays.
func inScope(req Request) bool {
	if req.Command != "" || req.Version {
		return false
	}
	if req.Display != Snapshot && req.Display != History {
		return false
	}
	f := req.Flags
	if f.Watch || f.Sync || f.DryRun || f.NoRain || f.SkipBrewUpdate {
		return false
	}
	return f.Top == 0
}

// Run composes source → query → view → render for an in-scope request and
// returns the lines plus stats; anything else yields ErrUnported (without the
// guard notices — the TS prints them and then does the unported thing; no
// harness case combines them). cfg is the post-guard config: the mode selects
// the record path and the snapshot label rule.
func Run(ctx context.Context, req Request, cfg config.Config, deps Deps) (Result, error) {
	req, notices, capActive := Normalize(req, cfg.Mode, deps.Now())
	if !inScope(req) {
		return Result{}, ErrUnported
	}

	ctx, cancel := context.WithTimeout(ctx, source.DefaultTimeout)
	defer cancel()

	recs, errs := gather(ctx, req, cfg, deps)

	if req.Display == History {
		return runHistory(req, cfg, recs, errs, notices, capActive, deps), nil
	}
	return runSnapshot(req, cfg, recs, errs, notices, deps), nil
}

// breakdownDim selects the --by-machine breakdown dimension (R3): machines,
// or users under multi-mode -u all (the TS usersLegend noun). Single mode
// already cleared -u in Normalize, so -u all there warns and falls back to
// machine columns.
func breakdownDim(req Request, cfg config.Config) (query.Dim, string) {
	if cfg.Mode == config.Multi && req.Flags.User == "all" {
		return query.User, "Users"
	}
	return query.Machine, "Machines"
}

// dimValue is a group key's value on the breakdown dimension.
func dimValue(key fact.Record, dim query.Dim) string {
	if dim == query.User {
		return key.User
	}
	return key.Machine
}

// buildSnapshotBreakdown builds the snapshot's machine/user column set from
// the un-collapsed records (R6) — one GroupBy(Tool, Date, dim) pass over the
// relabelled records, summing in record input order (the TS float
// association):
//   - all tools: one slice per (tool, current label, dim) group, in group
//     order (own machine first, then walk order); a tool with no
//     current-label group has no entry (DC-01).
//   - single source: the key set is the first-seen order of dim values over
//     ALL the tool's raw records (un-windowed); each slice's Totals are the
//     current-label group's or the zero fact.Totals{} — the TS
//     toolMachines.set(machine, match ? … : 0) zero-fill, so a zero-usage day
//     still lists every historical machine (a G0 candidate vs the DC-01
//     sentence; the harness byte-diff is the bar).
func buildSnapshotBreakdown(req Request, cfg config.Config, raw []fact.Record, cur string) *view.Breakdown {
	dim, noun := breakdownDim(req, cfg)
	bd := &view.Breakdown{Noun: noun, Rows: make(map[string][]view.Slice)}
	groups := query.GroupBy(query.Relabel(raw, req.Period), query.Tool, query.Date, dim)
	if req.Source == "" {
		for _, g := range groups {
			if g.Key.Date != cur {
				continue
			}
			t, _ := fact.Lookup(g.Key.Tool)
			bd.Rows[t.Name] = append(bd.Rows[t.Name], view.Slice{Name: dimValue(g.Key, dim), Totals: g.Totals})
		}
		return bd
	}
	tool, _ := fact.Lookup(req.Source)
	curTotals := make(map[string]fact.Totals)
	for _, g := range groups {
		if g.Key.Date == cur {
			curTotals[dimValue(g.Key, dim)] = g.Totals
		}
	}
	seen := make(map[string]bool)
	for _, r := range raw {
		v := dimValue(r, dim)
		if seen[v] {
			continue
		}
		seen[v] = true
		bd.Rows[tool.Name] = append(bd.Rows[tool.Name], view.Slice{Name: v, Totals: curTotals[v]})
	}
	return bd
}

// buildHistoryBreakdown builds the single-tool history's column set (R6): the
// same window the main table uses, the RollUp relabel, then ONE
// GroupBy(Tool, Date, dim) pass — Rows[label] appends one slice per group in
// first-seen order (GroupBy's order reproduces the TS machineMap iteration
// order per label; summing in record input order reproduces the TS sequential
// association, which matters under -u all for a multi-machine user).
func buildHistoryBreakdown(req Request, cfg config.Config, raw []fact.Record) *view.Breakdown {
	dim, noun := breakdownDim(req, cfg)
	bd := &view.Breakdown{Noun: noun, Rows: make(map[string][]view.Slice)}
	windowed := query.Window(raw, req.Flags.Since, req.Flags.Until)
	for _, g := range query.GroupBy(query.Relabel(windowed, req.Period), query.Tool, query.Date, dim) {
		bd.Rows[g.Key.Date] = append(bd.Rows[g.Key.Date], view.Slice{Name: dimValue(g.Key, dim), Totals: g.Totals})
	}
	return bd
}

// gather returns the daily records the pipeline consumes plus the source
// errors, choosing the TS path by mode and -u (intake §7.3):
//
//	single                           live fetch (as today)
//	multi, -u "" or -u == cfg.User   live fetch → stored := Read(cfg.User, tool)
//	                                 per tool → own/others split on Machine ==
//	                                 cfg.Machine → MaxMerge(live, own) ++ others
//	multi, -u all                    for u in Repo.Users(): Read(u, tool) per
//	                                 tool — no live fetch, no source errors
//	multi, -u <other>                Read(user, tool) per tool — no live fetch,
//	                                 no source errors
//
// The records are the stamped, UN-COLLAPSED per-machine/per-user records: the
// callers apply Collapse(recs, Tool, Date) followed by a stable date sort for
// the main table (the daily cross-machine/cross-user sum preceding the
// Window/RollUp/GroupBy tail — in single mode the identity on unique keys),
// while the --by-machine breakdown groups the same raw records on the
// machine/user dimension the collapse would drop. Record order is unchanged
// and load-bearing for --json float bytes: own machine first, then other
// machines in walk order; for -u all, users ascending then walk order.
func gather(ctx context.Context, req Request, cfg config.Config, deps Deps) ([]fact.Record, []*source.Error) {
	tools := fact.Tools
	if req.Source != "" {
		tool, _ := fact.Lookup(req.Source)
		tools = []fact.Tool{tool}
	}
	if cfg.Mode == config.Multi {
		if req.Flags.User == "all" {
			return gatherAllUsers(deps.Repo, tools), nil
		}
		if req.Flags.User != "" && req.Flags.User != cfg.User {
			return gatherUser(deps.Repo, req.Flags.User, tools), nil
		}
		return gatherOwn(ctx, req, cfg, deps, tools)
	}
	return fetchLive(ctx, req, deps, tools)
}

// fetchLive is the B2 live fetch: daily only (roll-up is client-side),
// FetchAll for all tools or Fetch for one, Fresh honored, errors collected.
func fetchLive(ctx context.Context, req Request, deps Deps, tools []fact.Tool) ([]fact.Record, []*source.Error) {
	if req.Source == "" {
		return deps.Source.FetchAll(ctx, source.PeriodDaily, nil, req.Flags.Fresh)
	}
	recs, serr := deps.Source.Fetch(ctx, tools[0], source.PeriodDaily, nil, req.Flags.Fresh)
	if serr != nil {
		return recs, []*source.Error{serr}
	}
	return recs, nil
}

// gatherOwn is the multi-mode own-user path: the live fetch reconciled with
// the machine's own stored day-files by whole-record max (never summed — a
// stored file only ever holds the fullest snapshot), then the other machines'
// records in walk order.
func gatherOwn(ctx context.Context, req Request, cfg config.Config, deps Deps, tools []fact.Tool) ([]fact.Record, []*source.Error) {
	live, errs := fetchLive(ctx, req, deps, tools)
	byTool := make(map[string][]fact.Record, len(tools))
	for _, r := range live {
		byTool[r.Tool] = append(byTool[r.Tool], r)
	}
	var recs []fact.Record
	for _, tool := range tools {
		var own, others []fact.Record
		for _, r := range deps.Repo.Read(cfg.User, tool) {
			if r.Machine == cfg.Machine {
				own = append(own, r)
			} else {
				others = append(others, r)
			}
		}
		recs = append(recs, query.MaxMerge(byTool[tool.Key], own)...)
		recs = append(recs, others...)
	}
	return recs, errs
}

// gatherUser is the repo-only -u <other> path: the user's records per tool in
// walk order, no live fetch, no source errors.
func gatherUser(repo Repo, user string, tools []fact.Tool) []fact.Record {
	var recs []fact.Record
	for _, tool := range tools {
		recs = append(recs, repo.Read(user, tool)...)
	}
	return recs
}

// gatherAllUsers is the repo-only -u all path: every profile's records per
// tool, users ascending then walk order (the TS readAllUsersByUser flattened).
func gatherAllUsers(repo Repo, tools []fact.Tool) []fact.Record {
	var recs []fact.Record
	for _, u := range repo.Users() {
		for _, tool := range tools {
			recs = append(recs, repo.Read(u, tool)...)
		}
	}
	return recs
}

// runSnapshot is V2's snapshot path, extended with the CSV/Markdown branches
// and the --by-machine breakdown (B4).
func runSnapshot(req Request, cfg config.Config, raw []fact.Record, errs []*source.Error, notices []string, deps Deps) Result {
	cur := query.CurrentLabel(req.Period, deps.Now())
	groups := query.GroupBy(query.Window(query.RollUp(query.SortByDate(query.Collapse(raw, query.Tool, query.Date)), req.Period), cur, cur), query.Tool)
	byTool := make(map[string]fact.Totals, len(groups))
	for _, g := range groups {
		byTool[g.Key.Tool] = g.Totals
	}

	tools := fact.Tools
	if req.Source != "" {
		tool, _ := fact.Lookup(req.Source)
		tools = []fact.Tool{tool}
	}
	rows := make([]view.ToolTotals, 0, len(tools))
	for _, t := range tools {
		row := view.ToolTotals{Name: t.Name}
		if totals, ok := byTool[t.Key]; ok {
			row.Totals = totals
			row.Label = cur
		}
		rows = append(rows, row)
	}
	// The label clear is a SINGLE-MODE artifact of the TS fetchAllTotals (bare
	// totals without a label): `tu --json` never carries "label" in single
	// mode. In multi mode every snapshot builds from fetchToolMerged entries,
	// so a tool with a record on the current label carries "label". Under
	// --by-machine the TS goes through fetchToolMergedWithMachines (labelled
	// entries) even in single mode — the clear does NOT apply (R12).
	if cfg.Mode == config.Single && req.Source == "" && req.Period == query.Daily && !req.Flags.ByMachine {
		for i := range rows {
			rows[i].Label = ""
		}
	}

	var bd *view.Breakdown
	if req.Flags.ByMachine {
		bd = buildSnapshotBreakdown(req, cfg, raw, cur)
	}

	res := Result{Notices: notices, Warnings: errs, CostByItem: make(map[string]float64, len(rows))}
	for _, r := range rows {
		res.TotalCost += r.TotalCost
		res.TotalTokens += r.TotalTokens
		res.CostByItem[r.Name] = r.TotalCost
	}
	metric := view.Cost
	if req.Flags.Metric == Tokens {
		metric = view.Tokens
	}
	switch req.Format {
	case JSON:
		res.Lines = renderjson.Snapshot(rows, bd)
	case CSV:
		res.Lines = csv.Snapshot(rows, bd)
	case Markdown:
		res.Lines = markdown.Snapshot(rows, req.Period, bd)
	default:
		res.Lines = ansi.Table(view.Snapshot(rows, req.Period, bd, metric), deps.Colors)
	}
	return res
}

// runHistory composes the history pipeline: the daily collapse (the TS
// mergeEntries summation order), then a stable date sort on the collapsed
// records (mergeEntries sorts its daily merge by label ahead of the period
// aggregation — without it a machine-major gather order associates a
// month/week bucket's sum differently and the raw JSON float bytes can
// differ), then the window on the DAILY records first and the roll-up second
// (a partial month sums only in-window days; a mid-week window yields a
// leading partial week labeled by its Sunday), then ONE GroupBy(Tool, Date)
// pass builds the registry-ordered series — no per-tool aggregation loop. The
// --by-machine breakdown (single source only — Normalize cleared the flag on
// the all-tools pivot) groups the SAME raw records on the machine/user
// dimension.
func runHistory(req Request, cfg config.Config, raw []fact.Record, errs []*source.Error, notices []string, capActive bool, deps Deps) Result {
	recs := query.RollUp(query.Window(query.SortByDate(query.Collapse(raw, query.Tool, query.Date)), req.Flags.Since, req.Flags.Until), req.Period)

	tools := fact.Tools
	if req.Source != "" {
		tool, _ := fact.Lookup(req.Source)
		tools = []fact.Tool{tool}
	}
	series := make([]view.Series, len(tools))
	byKey := make(map[string]int, len(tools))
	for i, t := range tools {
		series[i] = view.Series{Name: t.Name}
		byKey[t.Key] = i
	}
	// RollUp sorted ascending by Date, so each tool's entries stay ascending
	// (GroupBy preserves first-seen order within the sorted input).
	for _, g := range query.GroupBy(recs, query.Tool, query.Date) {
		i := byKey[g.Key.Tool]
		series[i].Entries = append(series[i].Entries, view.Entry{Label: g.Key.Date, Totals: g.Totals})
	}

	metric := view.Cost
	if req.Flags.Metric == Tokens {
		metric = view.Tokens
	}
	opts := view.HistoryOptions{
		Period:    req.Period,
		Now:       deps.Now(),
		Width:     deps.Width,
		CapActive: capActive,
		Metric:    metric,
	}

	res := Result{Notices: notices, Warnings: errs, CostByItem: make(map[string]float64)}
	for _, s := range series {
		for _, e := range s.Entries {
			res.TotalCost += e.TotalCost
			res.TotalTokens += e.TotalTokens
			v := e.TotalCost
			if metric == view.Tokens {
				v = float64(e.TotalTokens)
			}
			res.CostByItem[s.Name+":"+e.Label] = v
			res.CostByItem["total:"+e.Label] += v
		}
	}

	single := req.Source != ""
	var bd *view.Breakdown
	if single && req.Flags.ByMachine {
		bd = buildHistoryBreakdown(req, cfg, raw)
	}
	switch {
	case single && req.Format == JSON:
		res.Lines = renderjson.History(series[0], bd)
	case single && req.Format == CSV:
		res.Lines = csv.History(series[0], bd)
	case single && req.Format == Markdown:
		res.Lines = markdown.History(series[0], req.Period, capActive, bd)
	case single:
		res.Lines = ansi.Table(view.History(series[0], opts, bd), deps.Colors)
	case req.Format == JSON:
		res.Lines = renderjson.TotalHistory(series)
	case req.Format == CSV:
		res.Lines = csv.TotalHistory(series)
	case req.Format == Markdown:
		res.Lines = markdown.TotalHistory(series, req.Period, capActive)
	default:
		res.Lines = ansi.Table(view.TotalHistory(series, opts), deps.Colors)
	}
	return res
}
