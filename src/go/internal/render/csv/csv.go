// Package csv is the RFC 4180 encoder (the machine contract): headers pinned
// per kind, integers raw, costs via the exact-binary half-up rule (Cost —
// the TS csvCost, toFixed(2)), every field quoted when it contains a comma,
// quote, CR or LF. Encoders return []string; nothing here writes to a stream.
package csv

import (
	"strconv"
	"strings"

	"github.com/sahil87/tu/internal/render"
	"github.com/sahil87/tu/internal/view"
)

// The pinned headers (layouts; intake §6).
var (
	snapshotHeader    = []string{"tool", "tokens", "input", "output", "cache", "cost"}
	historyHeader     = []string{"date", "input", "output", "cache_write", "cache_read", "total", "cost"}
	totalHistoryTotal = "total"
)

// Snapshot renders the cross-tool snapshot: the header, one row per tool with
// TotalTokens > 0 (cache = write + read combined), and a Total row — summing
// EVERY input row, hidden ones counted, as in the table — only when more than
// one row is visible. A breakdown with at least one name appends
// machine_{name}_cost columns (the TS emitCsvSnapshot: sorted names, Cost(v)
// per name with 0.00 fill, the Total row summing visible rows only); under
// -u all the names are user names but the header prefix stays machine_.
func Snapshot(rows []view.ToolTotals, bd *view.Breakdown) []string {
	names := bd.Names()
	header := append([]string{}, snapshotHeader...)
	for _, name := range names {
		header = append(header, "machine_"+name+"_cost")
	}
	lines := []string{row(header)}

	machineSums := make([]float64, len(names))
	var grandInput, grandOutput, grandCache, grandTotal int64
	var grandCost float64
	visible := 0
	for _, r := range rows {
		if r.TotalTokens > 0 {
			visible++
			cells := []string{
				r.Name,
				num(r.TotalTokens),
				num(r.InputTokens),
				num(r.OutputTokens),
				num(r.CacheCreationTokens + r.CacheReadTokens),
				Cost(r.TotalCost),
			}
			for i, name := range names {
				v := bd.CostOf(r.Name, name)
				machineSums[i] += v
				cells = append(cells, Cost(v))
			}
			lines = append(lines, row(cells))
		}
		grandInput += r.InputTokens
		grandOutput += r.OutputTokens
		grandCache += r.CacheCreationTokens + r.CacheReadTokens
		grandTotal += r.TotalTokens
		grandCost += r.TotalCost
	}
	if visible > 1 {
		cells := []string{
			"Total", num(grandTotal), num(grandInput), num(grandOutput), num(grandCache), Cost(grandCost),
		}
		for _, sum := range machineSums {
			cells = append(cells, Cost(sum))
		}
		lines = append(lines, row(cells))
	}
	return lines
}

// History renders the single-tool history: the header plus one row per entry
// in input order. NEVER a Total row. An empty window is the header alone. A
// breakdown appends machine_{name}_cost columns (sorted names, Cost(v) per
// name with 0.00 fill — the TS emitCsvHistory).
func History(s view.Series, bd *view.Breakdown) []string {
	names := bd.Names()
	header := append([]string{}, historyHeader...)
	for _, name := range names {
		header = append(header, "machine_"+name+"_cost")
	}
	lines := []string{row(header)}
	for _, e := range s.Entries {
		cells := []string{
			e.Label,
			num(e.InputTokens),
			num(e.OutputTokens),
			num(e.CacheCreationTokens),
			num(e.CacheReadTokens),
			num(e.TotalTokens),
			Cost(e.TotalCost),
		}
		for _, name := range names {
			cells = append(cells, Cost(bd.CostOf(e.Label, name)))
		}
		lines = append(lines, row(cells))
	}
	return lines
}

// TotalHistory renders the cross-tool pivot: the header keeps EVERY series
// column (registry order, no omission — scripts index positionally), then one
// row per label over the sorted union, each series' cost (0 when absent) and
// the row total. NEVER a Total row.
func TotalHistory(series []view.Series) []string {
	header := make([]string, 0, len(series)+2)
	header = append(header, "date")
	for _, s := range series {
		header = append(header, s.Name)
	}
	header = append(header, totalHistoryTotal)
	lines := []string{row(header)}

	for _, label := range view.LabelUnion(series) {
		cells := make([]string, 0, len(series)+2)
		cells = append(cells, label)
		rowTotal := 0.0
		for _, s := range series {
			cost := 0.0
			for _, e := range s.Entries {
				if e.Label == label {
					cost = e.TotalCost
					break
				}
			}
			cells = append(cells, Cost(cost))
			rowTotal += cost
		}
		cells = append(cells, Cost(rowTotal))
		lines = append(lines, row(cells))
	}
	return lines
}

// num is a raw integer field (no thousands separators).
func num(n int64) string {
	return strconv.FormatInt(n, 10)
}

// csvShare is the leaderboard's share/delta fraction field (the TS csvShare):
// String(Math.round(n × 1000) / 1000) — JSRound (half toward +∞), then the
// shortest round-trip form with trailing zeros dropped: 0.69, -0.3, 17.309, 0
// (DC-11).
func csvShare(n float64) string {
	return strconv.FormatFloat(render.JSRound(n*1000)/1000, 'f', -1, 64)
}

// Leaderboard renders the lb CSV (the TS emitCsvLeaderboard): header
// rank,user[,machine],cost,total_tokens,share,delta — machine after user when
// byMachine (the explicit flag, so an empty result keeps the schema); one row
// per SLICED row (delta empty for a "new" row); a Total row over the FULL set
// (so --top 1 on two users still emits it) when len(all) > 1: Total,,{cost},
// {tokens},, — one more empty field under byMachine. Header alone when empty.
func Leaderboard(rows, all []view.LeaderboardRow, byMachine bool) []string {
	header := []string{"rank", "user"}
	if byMachine {
		header = append(header, "machine")
	}
	header = append(header, "cost", "total_tokens", "share", "delta")
	lines := []string{row(header)}

	for _, r := range rows {
		cells := []string{num(int64(r.Rank)), r.User}
		if byMachine {
			cells = append(cells, r.Machine)
		}
		delta := ""
		if r.Delta != nil {
			delta = csvShare(*r.Delta)
		}
		cells = append(cells, Cost(r.TotalCost), num(r.TotalTokens), csvShare(r.Share), delta)
		lines = append(lines, row(cells))
	}

	if len(all) > 1 {
		var grandCost float64
		var grandTokens int64
		for _, r := range all {
			grandCost += r.TotalCost
			grandTokens += r.TotalTokens
		}
		cells := []string{"Total", ""}
		if byMachine {
			cells = append(cells, "")
		}
		cells = append(cells, Cost(grandCost), num(grandTokens), "", "")
		lines = append(lines, row(cells))
	}
	return lines
}

// Cost formats the EXACT binary value of x rounded half-up to two decimals —
// the TS csvCost, toFixed(2), shared with the leaderboard's toFixed(1) share
// cell as render.FixedHalfUp. This is NOT strconv.FormatFloat(x, 'f', 2, 64)
// (half-even on the exact value: 0.125 → "0.12") and NOT render.FormatCost
// (ICU shortest-repr half away from zero: 1.005 → "$1.01").
//
// Node-verified (v24, 2026-09-16): 1.005 → 1.00, 0.125 → 0.13, 2.675 → 2.67,
// 0.015 → 0.01, 1.045 → 1.04, 0.045 → 0.04, 999999.995 → 999999.99.
func Cost(x float64) string {
	return render.FixedHalfUp(x, 2)
}

// quote applies RFC 4180 quoting: wrap in " and double inner " when the field
// contains a comma, quote, LF or CR.
func quote(field string) string {
	if strings.ContainsAny(field, ",\"\n\r") {
		return `"` + strings.ReplaceAll(field, `"`, `""`) + `"`
	}
	return field
}

// row joins a line's fields.
func row(cells []string) string {
	quoted := make([]string, len(cells))
	for i, c := range cells {
		quoted[i] = quote(c)
	}
	return strings.Join(quoted, ",")
}
