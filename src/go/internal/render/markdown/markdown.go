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

// Snapshot renders the cross-tool snapshot table: rows with TotalTokens > 0
// (cache = write + read combined); a bold Total — summing EVERY input row,
// hidden ones counted — only when more than one row is visible.
func Snapshot(rows []view.ToolTotals, period query.Period) []string {
	lines := []string{
		"## Combined Usage (" + period.String() + ")",
		"",
		row([]string{"Tool", "Tokens", "Input", "Output", "Cache", "Cost"}),
		alignRow(6),
	}

	var grand view.ToolTotals
	visible := 0
	for _, r := range rows {
		if r.TotalTokens > 0 {
			visible++
			lines = append(lines, row([]string{
				r.Name,
				render.FormatInt(r.TotalTokens),
				render.FormatInt(r.InputTokens),
				render.FormatInt(r.OutputTokens),
				render.FormatInt(r.CacheCreationTokens + r.CacheReadTokens),
				render.FormatCost(r.TotalCost),
			}))
		}
		grand.Totals = grand.Totals.Add(r.Totals)
	}
	if visible > 1 {
		lines = append(lines, row([]string{
			"**Total**",
			"**" + render.FormatInt(grand.TotalTokens) + "**",
			"**" + render.FormatInt(grand.InputTokens) + "**",
			"**" + render.FormatInt(grand.OutputTokens) + "**",
			"**" + render.FormatInt(grand.CacheCreationTokens+grand.CacheReadTokens) + "**",
			"**" + render.FormatCost(grand.TotalCost) + "**",
		}))
	}
	return append(lines, "")
}

// History renders the single-tool history: one row per entry in input order;
// a bold Total when there is more than one entry. The title is
// "{Name} ({period}[, last 3 months])".
func History(s view.Series, period query.Period, capActive bool) []string {
	lines := []string{
		"## " + s.Name + " (" + view.PeriodLabel(period, capActive) + ")",
		"",
		row([]string{"Date", "Input", "Output", "Cache Write", "Cache Read", "Total", "Cost"}),
		alignRow(7),
	}

	var sums view.Entry
	for _, e := range s.Entries {
		lines = append(lines, row([]string{
			e.Label,
			render.FormatInt(e.InputTokens),
			render.FormatInt(e.OutputTokens),
			render.FormatInt(e.CacheCreationTokens),
			render.FormatInt(e.CacheReadTokens),
			render.FormatInt(e.TotalTokens),
			render.FormatCost(e.TotalCost),
		}))
		sums.Totals = sums.Totals.Add(e.Totals)
	}
	if len(s.Entries) > 1 {
		lines = append(lines, row([]string{
			"**Total**",
			"**" + render.FormatInt(sums.InputTokens) + "**",
			"**" + render.FormatInt(sums.OutputTokens) + "**",
			"**" + render.FormatInt(sums.CacheCreationTokens) + "**",
			"**" + render.FormatInt(sums.CacheReadTokens) + "**",
			"**" + render.FormatInt(sums.TotalTokens) + "**",
			"**" + render.FormatCost(sums.TotalCost) + "**",
		}))
	}
	return append(lines, "")
}

// TotalHistory renders the cross-tool pivot. The title is ALWAYS
// "Combined Cost History" and the cells always cost — the Markdown pivot
// ignores --metric (the TS emitMarkdownTotalHistory reads totalCost). Columns
// drop exact-zero tools over all labels (the ANSI significance rule does not
// apply here); when every tool is exact-zero all stay. A bold Total follows
// when more than one label exists. An empty window is heading, blank, header,
// alignment row, blank — no data rows.
func TotalHistory(series []view.Series, period query.Period, capActive bool) []string {
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
		"## Combined Cost History (" + view.PeriodLabel(period, capActive) + ")",
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
