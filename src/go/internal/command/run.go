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
// (asserted where cmd/tu assigns it); B3's *metrics.Source will too.
type Fetcher interface {
	Fetch(ctx context.Context, tool fact.Tool, period string, extraArgs []string, fresh bool) ([]fact.Record, *source.Error)
	FetchAll(ctx context.Context, period string, extraArgs []string, fresh bool) ([]fact.Record, []*source.Error)
}

// Deps are Run's external inputs.
type Deps struct {
	Source Fetcher
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
// mode, no non-data command, no version; Since/Until/Full are history flags
// (the guards clear them on snapshots). Still out: multi mode, -u,
// --by-machine, --top, watch, sync, dry-run, no-rain, skip-brew-update.
func inScope(req Request, mode config.Mode) bool {
	if mode != config.Single || req.Command != "" || req.Version {
		return false
	}
	if req.Display != Snapshot && req.Display != History {
		return false
	}
	f := req.Flags
	if f.Watch || f.Sync || f.DryRun || f.ByMachine || f.NoRain || f.SkipBrewUpdate {
		return false
	}
	return f.User == "" && f.Top == 0
}

// Run composes source → query → view → render for an in-scope request and
// returns the lines plus stats; anything else yields ErrUnported (without the
// guard notices — the TS prints them and then does the unported thing; no
// harness case combines them).
func Run(ctx context.Context, req Request, mode config.Mode, deps Deps) (Result, error) {
	req, notices, capActive := Normalize(req, deps.Now())
	if !inScope(req, mode) {
		return Result{}, ErrUnported
	}

	ctx, cancel := context.WithTimeout(ctx, source.DefaultTimeout)
	defer cancel()

	// Fetch daily only — the TS only ever calls daily; roll-up is client-side.
	var recs []fact.Record
	var errs []*source.Error
	tool, _ := fact.Lookup(req.Source)
	if req.Source == "" {
		recs, errs = deps.Source.FetchAll(ctx, source.PeriodDaily, nil, req.Flags.Fresh)
	} else {
		var serr *source.Error
		recs, serr = deps.Source.Fetch(ctx, tool, source.PeriodDaily, nil, req.Flags.Fresh)
		if serr != nil {
			errs = []*source.Error{serr}
		}
	}

	if req.Display == History {
		return runHistory(req, recs, errs, notices, capActive, deps), nil
	}
	return runSnapshot(req, recs, errs, notices, deps), nil
}

// runSnapshot is V2's snapshot path, extended with the CSV/Markdown branches.
func runSnapshot(req Request, recs []fact.Record, errs []*source.Error, notices []string, deps Deps) Result {
	cur := query.CurrentLabel(req.Period, deps.Now())
	groups := query.GroupBy(query.Window(query.RollUp(recs, req.Period), cur, cur), query.Tool)
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
	// The TS single-mode daily-all path goes through fetchAllTotals, which
	// returns bare totals without a label — `tu --json` never carries "label".
	if req.Source == "" && req.Period == query.Daily {
		for i := range rows {
			rows[i].Label = ""
		}
	}

	res := Result{Notices: notices, Warnings: errs, CostByItem: make(map[string]float64, len(rows))}
	for _, r := range rows {
		res.TotalCost += r.TotalCost
		res.TotalTokens += r.TotalTokens
		res.CostByItem[r.Name] = r.TotalCost
	}
	switch req.Format {
	case JSON:
		res.Lines = renderjson.Snapshot(rows)
	case CSV:
		res.Lines = csv.Snapshot(rows)
	case Markdown:
		res.Lines = markdown.Snapshot(rows, req.Period)
	default:
		res.Lines = ansi.Table(view.Snapshot(rows, req.Period), deps.Colors)
	}
	return res
}

// runHistory composes the history pipeline: the window applies to the DAILY
// records first and the roll-up second (a partial month sums only in-window
// days; a mid-week window yields a leading partial week labeled by its
// Sunday), then ONE GroupBy(Tool, Date) pass builds the registry-ordered
// series — no per-tool aggregation loop.
func runHistory(req Request, daily []fact.Record, errs []*source.Error, notices []string, capActive bool, deps Deps) Result {
	recs := query.RollUp(query.Window(daily, req.Flags.Since, req.Flags.Until), req.Period)

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
	switch {
	case single && req.Format == JSON:
		res.Lines = renderjson.History(series[0])
	case single && req.Format == CSV:
		res.Lines = csv.History(series[0])
	case single && req.Format == Markdown:
		res.Lines = markdown.History(series[0], req.Period, capActive)
	case single:
		res.Lines = ansi.Table(view.History(series[0], opts), deps.Colors)
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
