// Package csv is the RFC 4180 encoder (the machine contract): headers pinned
// per kind, integers raw, costs via the exact-binary half-up rule (Cost —
// the TS csvCost, toFixed(2)), every field quoted when it contains a comma,
// quote, CR or LF. Encoders return []string; nothing here writes to a stream.
package csv

import (
	"math/big"
	"strconv"
	"strings"

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
// one row is visible.
func Snapshot(rows []view.ToolTotals) []string {
	lines := []string{row(snapshotHeader)}

	var grandInput, grandOutput, grandCache, grandTotal int64
	var grandCost float64
	visible := 0
	for _, r := range rows {
		if r.TotalTokens > 0 {
			visible++
			lines = append(lines, row([]string{
				r.Name,
				num(r.TotalTokens),
				num(r.InputTokens),
				num(r.OutputTokens),
				num(r.CacheCreationTokens + r.CacheReadTokens),
				Cost(r.TotalCost),
			}))
		}
		grandInput += r.InputTokens
		grandOutput += r.OutputTokens
		grandCache += r.CacheCreationTokens + r.CacheReadTokens
		grandTotal += r.TotalTokens
		grandCost += r.TotalCost
	}
	if visible > 1 {
		lines = append(lines, row([]string{
			"Total", num(grandTotal), num(grandInput), num(grandOutput), num(grandCache), Cost(grandCost),
		}))
	}
	return lines
}

// History renders the single-tool history: the header plus one row per entry
// in input order. NEVER a Total row. An empty window is the header alone.
func History(s view.Series) []string {
	lines := []string{row(historyHeader)}
	for _, e := range s.Entries {
		lines = append(lines, row([]string{
			e.Label,
			num(e.InputTokens),
			num(e.OutputTokens),
			num(e.CacheCreationTokens),
			num(e.CacheReadTokens),
			num(e.TotalTokens),
			Cost(e.TotalCost),
		}))
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

// Cost formats the EXACT binary value of x rounded half-up to two decimals —
// the TS csvCost, toFixed(2). This is NOT strconv.FormatFloat(x, 'f', 2, 64)
// (half-even on the exact value: 0.125 → "0.12") and NOT render.FormatCost
// (ICU shortest-repr half away from zero: 1.005 → "$1.01"). big.Rat holds the
// exact value; scale by 100, floor, round up when the remainder ≥ 1/2. -0 and
// 0 render "0.00".
//
// Node-verified (v24, 2026-09-16): 1.005 → 1.00, 0.125 → 0.13, 2.675 → 2.67,
// 0.015 → 0.01, 1.045 → 1.04, 0.045 → 0.04, 999999.995 → 999999.99.
func Cost(x float64) string {
	// exact = the binary value as a rational; scaled = exact × 100.
	r := new(big.Rat).SetFloat64(x)
	if x < 0 {
		r.Neg(r) // round the magnitude; re-add the sign at the end
	}
	scaled := new(big.Rat).Mul(r, big.NewRat(100, 1))
	intPart := new(big.Int)
	remainder := new(big.Rat)
	intPart.Quo(scaled.Num(), scaled.Denom()) // truncated; scaled ≥ 0 so == floor
	remainder.Sub(scaled, new(big.Rat).SetInt(intPart))
	if remainder.Cmp(big.NewRat(1, 2)) >= 0 {
		intPart.Add(intPart, big.NewInt(1))
	}
	cents := intPart.String()
	if len(cents) < 3 {
		cents = strings.Repeat("0", 3-len(cents)) + cents
	}
	out := cents[:len(cents)-2] + "." + cents[len(cents)-2:]
	if x < 0 && intPart.Sign() != 0 {
		return "-" + out
	}
	return out
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
