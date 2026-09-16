package command

import (
	"context"
	"errors"
	"time"

	"github.com/sahil87/tu/internal/config"
	"github.com/sahil87/tu/internal/fact"
	"github.com/sahil87/tu/internal/query"
	"github.com/sahil87/tu/internal/render/ansi"
	renderjson "github.com/sahil87/tu/internal/render/json"
	"github.com/sahil87/tu/internal/source"
	"github.com/sahil87/tu/internal/source/ccusage"
	"github.com/sahil87/tu/internal/view"
)

// Fetcher is what command needs from a source; *ccusage.Source satisfies it.
type Fetcher interface {
	Fetch(ctx context.Context, tool ccusage.Tool, period string, extraArgs []string, fresh bool) ([]fact.Record, *source.Error)
	FetchAll(ctx context.Context, period string, extraArgs []string, fresh bool) ([]fact.Record, []*source.Error)
}

// Deps are Run's external inputs.
type Deps struct {
	Source Fetcher
	Now    func() time.Time // time.Now at the edge; fixed in tests
	Colors ansi.Colors
}

// Result is what Run returns; cmd/tu writes it.
type Result struct {
	Lines       []string           // stdout, one per line, no trailing "\n" per element
	Warnings    []*source.Error    // the edge writes them with source.WriteWarnings
	TotalCost   float64            // sum over the rendered rows (watch stats)
	TotalTokens int64              // same
	CostByItem  map[string]float64 // display name → cost (the TS _lastRenderCostMap, snapshot keys)
}

// ErrUnported marks a recognized-but-unported request; cmd/tu maps it to the
// scaffold's placeholder line, exit 1.
var ErrUnported = errors.New("not implemented")

// inScope reports whether the request is V2's single-mode snapshot grammar
// (intake §2): snapshot display, table or JSON, no unported flag, single
// mode, no non-data command, no version.
func inScope(req Request, mode config.Mode) bool {
	if mode != config.Single || req.Display != Snapshot || req.Command != "" || req.Version {
		return false
	}
	if req.Format != Table && req.Format != JSON {
		return false
	}
	f := req.Flags
	if f.Watch || f.Sync || f.DryRun || f.ByMachine || f.Full || f.NoRain || f.SkipBrewUpdate {
		return false
	}
	return f.User == "" && f.Since == "" && f.Until == "" && f.Top == 0
}

// Run composes source → query → view → render for an in-scope request and
// returns the lines plus stats; anything else yields ErrUnported.
func Run(ctx context.Context, req Request, mode config.Mode, deps Deps) (Result, error) {
	if !inScope(req, mode) {
		return Result{}, ErrUnported
	}

	ctx, cancel := context.WithTimeout(ctx, ccusage.DefaultTimeout)
	defer cancel()

	// Fetch daily only — the TS only ever calls daily; roll-up is client-side.
	var recs []fact.Record
	var errs []*source.Error
	tool, _ := ccusage.Lookup(req.Source)
	if req.Source == "" {
		recs, errs = deps.Source.FetchAll(ctx, ccusage.PeriodDaily, nil, req.Flags.Fresh)
	} else {
		var serr *source.Error
		recs, serr = deps.Source.Fetch(ctx, tool, ccusage.PeriodDaily, nil, req.Flags.Fresh)
		if serr != nil {
			errs = []*source.Error{serr}
		}
	}

	cur := query.CurrentLabel(req.Period, deps.Now())
	groups := query.GroupBy(query.Window(query.RollUp(recs, req.Period), cur, cur), query.Tool)
	byTool := make(map[string]fact.Totals, len(groups))
	for _, g := range groups {
		byTool[g.Key.Tool] = g.Totals
	}

	tools := ccusage.Tools
	if req.Source != "" {
		tools = []ccusage.Tool{tool}
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

	res := Result{Warnings: errs, CostByItem: make(map[string]float64, len(rows))}
	for _, r := range rows {
		res.TotalCost += r.TotalCost
		res.TotalTokens += r.TotalTokens
		res.CostByItem[r.Name] = r.TotalCost
	}
	if req.Format == JSON {
		res.Lines = renderjson.Snapshot(rows)
	} else {
		res.Lines = ansi.Table(view.Snapshot(rows, req.Period), deps.Colors)
	}
	return res, nil
}
