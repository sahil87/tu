package query

import (
	"reflect"
	"testing"

	"github.com/sahil87/tu/internal/fact"
)

// The R3 fixture: two live cc records (tokens T1) against three stored ones
// (tokens T2) — one day where live wins, one where stored wins, one stored
// has alone.
var (
	mergeT1 = fact.Totals{TotalCost: 0, InputTokens: 1, OutputTokens: 2, CacheCreationTokens: 3, CacheReadTokens: 4, TotalTokens: 10}
	mergeT2 = fact.Totals{TotalCost: 0, InputTokens: 5, OutputTokens: 6, CacheCreationTokens: 7, CacheReadTokens: 8, TotalTokens: 26}
)

func mergeRec(date string, cost float64, t fact.Totals) fact.Record {
	t.TotalCost = cost
	return fact.Record{Date: date, Tool: "cc", User: "u", Machine: "m", Totals: t}
}

// R3: whole-record max per key, ties to a, output order a-then-unmatched-b.
func TestMaxMerge(t *testing.T) {
	cases := []struct {
		name string
		a, b []fact.Record
		want []fact.Record
	}{
		{
			name: "both arms plus unmatched",
			a:    []fact.Record{mergeRec("2026-01-05", 0.50, mergeT1), mergeRec("2026-01-06", 0.50, mergeT1)},
			b:    []fact.Record{mergeRec("2026-01-05", 0.25, mergeT2), mergeRec("2026-01-06", 0.75, mergeT2), mergeRec("2026-01-04", 0.10, mergeT2)},
			want: []fact.Record{mergeRec("2026-01-05", 0.50, mergeT1), mergeRec("2026-01-06", 0.75, mergeT2), mergeRec("2026-01-04", 0.10, mergeT2)},
		},
		{
			name: "tie keeps a (tokens included)",
			a:    []fact.Record{mergeRec("2026-01-05", 0.50, mergeT1)},
			b:    []fact.Record{mergeRec("2026-01-05", 0.50, mergeT2)},
			want: []fact.Record{mergeRec("2026-01-05", 0.50, mergeT1)},
		},
		{
			name: "empty a",
			a:    nil,
			b:    []fact.Record{mergeRec("2026-01-05", 0.25, mergeT2)},
			want: []fact.Record{mergeRec("2026-01-05", 0.25, mergeT2)},
		},
		{
			name: "empty b",
			a:    []fact.Record{mergeRec("2026-01-05", 0.50, mergeT1)},
			b:    nil,
			want: []fact.Record{mergeRec("2026-01-05", 0.50, mergeT1)},
		},
		{
			// The key is (Date, Tool, User, Machine): the same date from
			// another machine is a different record, never merged.
			name: "different machine is a different key",
			a:    []fact.Record{mergeRec("2026-01-05", 0.50, mergeT1)},
			b: []fact.Record{{Date: "2026-01-05", Tool: "cc", User: "u", Machine: "other",
				Totals: fact.Totals{TotalCost: 0.90}}},
			want: []fact.Record{mergeRec("2026-01-05", 0.50, mergeT1),
				{Date: "2026-01-05", Tool: "cc", User: "u", Machine: "other", Totals: fact.Totals{TotalCost: 0.90}}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Inputs must survive unmutated and unaliased.
			aCopy := append([]fact.Record(nil), c.a...)
			bCopy := append([]fact.Record(nil), c.b...)
			got := MaxMerge(c.a, c.b)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("MaxMerge = %+v, want %+v", got, c.want)
			}
			if !reflect.DeepEqual(c.a, aCopy) || !reflect.DeepEqual(c.b, bCopy) {
				t.Error("MaxMerge mutated an input")
			}
		})
	}
}

// R4: Collapse sums per grouped tuple in input order over GroupBy and drops
// the ungrouped dims.
func TestCollapse(t *testing.T) {
	recs := []fact.Record{
		{Date: "2026-01-06", Tool: "cc", User: "u", Machine: "a", Totals: fact.Totals{TotalCost: 0.75, InputTokens: 1, TotalTokens: 5}},
		{Date: "2026-01-06", Tool: "cc", User: "u", Machine: "b", Totals: fact.Totals{TotalCost: 0.40, InputTokens: 2, TotalTokens: 6}},
		{Date: "2026-01-07", Tool: "codex", User: "u", Machine: "b", Totals: fact.Totals{TotalCost: 0.30, InputTokens: 3, TotalTokens: 7}},
	}
	want := []fact.Record{
		{Date: "2026-01-06", Tool: "cc", Totals: fact.Totals{TotalCost: 1.15, InputTokens: 3, TotalTokens: 11}},
		{Date: "2026-01-07", Tool: "codex", Totals: fact.Totals{TotalCost: 0.30, InputTokens: 3, TotalTokens: 7}},
	}
	if got := Collapse(recs, Tool, Date); !reflect.DeepEqual(got, want) {
		t.Errorf("Collapse = %+v, want %+v", got, want)
	}
}

// On records already unique per key the output equals the input's Totals
// one-for-one (the single-mode identity), dims dropped.
func TestCollapseIdentityOnUniqueKeys(t *testing.T) {
	recs := []fact.Record{
		{Date: "2026-01-05", Tool: "cc", User: "u", Machine: "m", Totals: mergeT1},
		{Date: "2026-01-06", Tool: "cc", User: "u", Machine: "m", Totals: mergeT2},
	}
	got := Collapse(recs, Tool, Date)
	want := []fact.Record{
		{Date: "2026-01-05", Tool: "cc", Totals: mergeT1},
		{Date: "2026-01-06", Tool: "cc", Totals: mergeT2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Collapse = %+v, want %+v", got, want)
	}
}

// Additions within a key happen in input order: an association-sensitive
// triple proves the first-seen order is load-bearing.
func TestCollapseAdditionOrder(t *testing.T) {
	// Runtime variables, not constants: constant arithmetic is exact and
	// would not pin the float association.
	a, b, c := 0.1, 0.2, 0.3
	recs := []fact.Record{
		{Date: "2026-01-06", Tool: "cc", Totals: fact.Totals{TotalCost: a}},
		{Date: "2026-01-06", Tool: "cc", Totals: fact.Totals{TotalCost: b}},
		{Date: "2026-01-06", Tool: "cc", Totals: fact.Totals{TotalCost: c}},
	}
	got := Collapse(recs, Tool, Date)
	if len(got) != 1 {
		t.Fatalf("Collapse = %+v, want one group", got)
	}
	if want := (a + b) + c; got[0].TotalCost != want {
		t.Errorf("TotalCost = %v, want %v (input-order association)", got[0].TotalCost, want)
	}
}
