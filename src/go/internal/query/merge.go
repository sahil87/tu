package query

import (
	"github.com/sahil87/tu/internal/fact"
)

// MaxMerge is the self-view high-water merge (the TS maxMergeEntries): per key
// (Date, Tool, User, Machine) it keeps whichever WHOLE record — from a or from
// b — has the greater TotalCost; on a tie the record from a wins. Never
// field-wise, never summed (Constitution V). Output order is a's records in
// a's order (replaced in place when b wins), then b's unmatched records in b's
// order. Pure: the inputs are never mutated or returned.
func MaxMerge(a, b []fact.Record) []fact.Record {
	type recordKey struct {
		date, tool, user, machine string
	}
	index := make(map[recordKey]int, len(a))
	out := make([]fact.Record, 0, len(a)+len(b))
	for _, list := range [][]fact.Record{a, b} {
		for _, r := range list {
			k := recordKey{r.Date, r.Tool, r.User, r.Machine}
			if i, ok := index[k]; ok {
				if r.TotalCost > out[i].TotalCost {
					out[i] = r
				}
				continue
			}
			index[k] = len(out)
			out = append(out, r)
		}
	}
	return out
}

// Collapse sums Totals per distinct tuple of dims and returns one record per
// group carrying only the grouped dims (the other string fields empty), in
// first-seen order, with additions performed in input order (the TS
// mergeEntries). It is a thin wrapper over the one GroupBy — no second
// aggregation loop.
func Collapse(recs []fact.Record, dims ...Dim) []fact.Record {
	groups := GroupBy(recs, dims...)
	out := make([]fact.Record, 0, len(groups))
	for _, g := range groups {
		r := g.Key
		r.Totals = g.Totals
		out = append(out, r)
	}
	return out
}
