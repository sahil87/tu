// Package markdown is the GitHub-flavoured Markdown encoder (the human
// paste-target contract): a "## {title}" heading, a blank line, the GFM table
// (header, :---/---: alignment row, data rows, a bold **Total** row when more
// than one data row is visible), and a trailing blank line — the caller's
// per-line Fprintln reproduces the TS emitMarkdown's output + "\n". Numbers
// keep comma grouping, costs the ICU $ rule (render.FormatInt/FormatCost).
// No ANSI, no bars, no delta arrows. Encoders return []string; nothing here
// writes to a stream.
package markdown

import (
	"strconv"
	"strings"

	"github.com/sahil87/tu/internal/query"
	"github.com/sahil87/tu/internal/render"
	"github.com/sahil87/tu/internal/view"
)

// row is one GFM table line.
func row(cells []string) string {
	return "| " + strings.Join(cells, " | ") + " |"
}

// alignRow is the GFM alignment marker line: left for the text column,
// right for numerics.
func alignRow(n int) string {
	marks := make([]string, n)
	for i := range marks {
		marks[i] = "---:"
	}
	marks[0] = ":---"
	return row(marks)
}

// alignRowOf is the alignment line for an explicit marker list (the
// leaderboard's right-aligned rank column).
func alignRowOf(marks []string) string {
	return row(marks)
}

// Snapshot renders the cross-tool snapshot table: rows with TotalTokens > 0
// (cache = write + read combined); a bold Total — summing EVERY input row,
// hidden ones counted — only when more than one row is visible. A breakdown
// with at least one name appends name-headed columns (the TS
// emitMarkdownSnapshot: each sorted name VERBATIM — no letters, no legend —
// right-aligned, FormatCost cells, bold per-name sums over visible rows on
// the Total row).
func Snapshot(rows []view.ToolTotals, period query.Period, bd *view.Breakdown) []string {
	names := bd.Names()
	header := []string{"Tool", "Tokens", "Input", "Output", "Cache", "Cost"}
	header = append(header, names...)
	lines := []string{
		"## Combined Usage (" + period.String() + ")",
		"",
		row(header),
		alignRow(len(header)),
	}

	machineSums := make([]float64, len(names))
	var grand view.ToolTotals
	visible := 0
	for _, r := range rows {
		if r.TotalTokens > 0 {
			visible++
			cells := []string{
				r.Name,
				render.FormatInt(r.TotalTokens),
				render.FormatInt(r.InputTokens),
				render.FormatInt(r.OutputTokens),
				render.FormatInt(r.CacheCreationTokens + r.CacheReadTokens),
				render.FormatCost(r.TotalCost),
			}
			for i, name := range names {
				v := bd.CostOf(r.Name, name)
				machineSums[i] += v
				cells = append(cells, render.FormatCost(v))
			}
			lines = append(lines, row(cells))
		}
		grand.Totals = grand.Totals.Add(r.Totals)
	}
	if visible > 1 {
		cells := []string{
			"**Total**",
			"**" + render.FormatInt(grand.TotalTokens) + "**",
			"**" + render.FormatInt(grand.InputTokens) + "**",
			"**" + render.FormatInt(grand.OutputTokens) + "**",
			"**" + render.FormatInt(grand.CacheCreationTokens+grand.CacheReadTokens) + "**",
			"**" + render.FormatCost(grand.TotalCost) + "**",
		}
		for _, sum := range machineSums {
			cells = append(cells, "**"+render.FormatCost(sum)+"**")
		}
		lines = append(lines, row(cells))
	}
	return append(lines, "")
}

// History renders the single-tool history: one row per entry in input order;
// a bold Total when there is more than one entry. The title is
// "{Name} ({period}[, last 3 months])". A breakdown appends name-headed
// columns (sorted names verbatim, right-aligned, FormatCost cells, bold
// per-name sums over ALL entries on the Total row — the TS
// emitMarkdownHistory).
func History(s view.Series, period query.Period, capActive bool, bd *view.Breakdown) []string {
	names := bd.Names()
	header := []string{"Date", "Input", "Output", "Cache Write", "Cache Read", "Total", "Cost"}
	header = append(header, names...)
	lines := []string{
		"## " + s.Name + " (" + view.PeriodLabel(period, capActive) + ")",
		"",
		row(header),
		alignRow(len(header)),
	}

	machineSums := make([]float64, len(names))
	var sums view.Entry
	for _, e := range s.Entries {
		cells := []string{
			e.Label,
			render.FormatInt(e.InputTokens),
			render.FormatInt(e.OutputTokens),
			render.FormatInt(e.CacheCreationTokens),
			render.FormatInt(e.CacheReadTokens),
			render.FormatInt(e.TotalTokens),
			render.FormatCost(e.TotalCost),
		}
		for i, name := range names {
			v := bd.CostOf(e.Label, name)
			machineSums[i] += v
			cells = append(cells, render.FormatCost(v))
		}
		lines = append(lines, row(cells))
		sums.Totals = sums.Totals.Add(e.Totals)
	}
	if len(s.Entries) > 1 {
		cells := []string{
			"**Total**",
			"**" + render.FormatInt(sums.InputTokens) + "**",
			"**" + render.FormatInt(sums.OutputTokens) + "**",
			"**" + render.FormatInt(sums.CacheCreationTokens) + "**",
			"**" + render.FormatInt(sums.CacheReadTokens) + "**",
			"**" + render.FormatInt(sums.TotalTokens) + "**",
			"**" + render.FormatCost(sums.TotalCost) + "**",
		}
		for _, sum := range machineSums {
			cells = append(cells, "**"+render.FormatCost(sum)+"**")
		}
		lines = append(lines, row(cells))
	}
	return append(lines, "")
}

// TotalHistory renders the cross-tool pivot. The cells are always cost — the
// Markdown pivot ignores --metric (the TS emitMarkdownTotalHistory reads
// totalCost); title is the heading text ("Combined Cost History" from the
// tool pivot's caller, "Leaderboard History" / "Leaderboard Token History"
// from lbh — the TS mdTitle). Columns drop exact-zero tools over all labels
// (the ANSI significance rule does not apply here); when every tool is
// exact-zero all stay. A bold Total follows when more than one label exists.
// An empty window is heading, blank, header, alignment row, blank — no data
// rows.
func TotalHistory(series []view.Series, period query.Period, capActive bool, title string) []string {
	labels := view.LabelUnion(series)

	// Cost map: tool → label → totalCost; columns = nonzeroTools over all
	// labels with the all-stay fallback.
	visible := make([]int, 0, len(series))
	for i, s := range series {
		nonzero := false
		for _, e := range s.Entries {
			if e.TotalCost != 0 {
				nonzero = true
				break
			}
		}
		if nonzero {
			visible = append(visible, i)
		}
	}
	if len(visible) == 0 {
		for i := range series {
			visible = append(visible, i)
		}
	}

	header := make([]string, 0, len(visible)+2)
	header = append(header, "Date")
	for _, i := range visible {
		header = append(header, series[i].Name)
	}
	header = append(header, "Cost")
	lines := []string{
		"## " + title + " (" + view.PeriodLabel(period, capActive) + ")",
		"",
		row(header),
		alignRow(len(header)),
	}

	toolSums := make([]float64, len(visible))
	grandTotal := 0.0
	for _, label := range labels {
		cells := []string{label}
		rowTotal := 0.0
		for vi, i := range visible {
			cost := 0.0
			for _, e := range series[i].Entries {
				if e.Label == label {
					cost = e.TotalCost
					break
				}
			}
			cells = append(cells, render.FormatCost(cost))
			toolSums[vi] += cost
			rowTotal += cost
		}
		cells = append(cells, render.FormatCost(rowTotal))
		grandTotal += rowTotal
		lines = append(lines, row(cells))
	}
	if len(labels) > 1 {
		cells := []string{"**Total**"}
		for _, sum := range toolSums {
			cells = append(cells, "**"+render.FormatCost(sum)+"**")
		}
		cells = append(cells, "**"+render.FormatCost(grandTotal)+"**")
		lines = append(lines, row(cells))
	}
	return append(lines, "")
}

// Leaderboard renders the lb table (the TS emitMarkdownLeaderboard):
// "## Leaderboard ({period})" (the period word only — no window, no metric),
// blank, the header (#, User, [Machine,] Cost, Tokens, Share, Δ vs
// {deltaLabel}) with alignment ---:, :---, [:---,] ---:, ---:, ---:, ---:,
// data rows (FormatCost, FormatInt, the toFixed(1) share cell, view.DeltaCell
// — "new" for a nil delta), a **Total** row (blank User/Share/Δ cells) over
// the FULL set when len(all) > 1 (also under --top), trailing blank. No bars,
// no arrows, no staleness footer.
func Leaderboard(rows, all []view.LeaderboardRow, period query.Period, deltaLabel string, byMachine bool) []string {
	header := []string{"#", "User"}
	aligns := []string{"---:", ":---"}
	if byMachine {
		header = append(header, "Machine")
		aligns = append(aligns, ":---")
	}
	header = append(header, "Cost", "Tokens", "Share", "Δ vs "+deltaLabel)
	aligns = append(aligns, "---:", "---:", "---:", "---:")

	lines := []string{
		"## Leaderboard (" + period.String() + ")",
		"",
		row(header),
		alignRowOf(aligns),
	}

	for _, r := range rows {
		cells := []string{strconv.Itoa(r.Rank), r.User}
		if byMachine {
			cells = append(cells, r.Machine)
		}
		cells = append(cells,
			render.FormatCost(r.TotalCost),
			render.FormatInt(r.TotalTokens),
			render.FixedHalfUp(r.Share*100, 1)+"%",
			view.DeltaCell(r.Delta),
		)
		lines = append(lines, row(cells))
	}

	if len(all) > 1 {
		var grandCost float64
		var grandTokens int64
		for _, r := range all {
			grandCost += r.TotalCost
			grandTokens += r.TotalTokens
		}
		cells := []string{"**Total**", ""}
		if byMachine {
			cells = append(cells, "")
		}
		cells = append(cells,
			"**"+render.FormatCost(grandCost)+"**",
			"**"+render.FormatInt(grandTokens)+"**",
			"", "",
		)
		lines = append(lines, row(cells))
	}
	return append(lines, "")
}
