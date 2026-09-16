package view

import (
	"testing"

	"github.com/sahil87/tu/internal/fact"
)

// sampleBreakdown is two machines over two snapshot rows (one hidden) — the
// names union spans every row, visible or not.
func sampleBreakdown() *Breakdown {
	return &Breakdown{Noun: "Machines", Rows: map[string][]Slice{
		"Claude Code": {
			{Name: "dev-ws-sahil02", Totals: fact.Totals{TotalCost: 6288.75}},
			{Name: "Sahils-Mac-mini.local", Totals: fact.Totals{TotalCost: 8.27}},
		},
		"Codex": {
			{Name: "dev-ws-sahil02", Totals: fact.Totals{TotalCost: 1.0}},
		},
	}}
}

func TestBreakdownNamesSortedUnion(t *testing.T) {
	got := sampleBreakdown().Names()
	want := []string{"Sahils-Mac-mini.local", "dev-ws-sahil02"} // byte order: 'S' < 'd'
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("names = %v, want %v", got, want)
	}
	var nilBd *Breakdown
	if nilBd.Names() != nil {
		t.Error("nil breakdown must yield nil names")
	}
	empty := &Breakdown{Noun: "Machines", Rows: map[string][]Slice{"Claude Code": {}}}
	if empty.Names() != nil {
		t.Error("a breakdown with no slices must yield nil names")
	}
}

// A-019: letters run A…Z and continue past Z as rune('A'+i) — the 27th name
// is "[", exactly as String.fromCharCode(65+26).
func TestBreakdownLettersPastZ(t *testing.T) {
	names := make([]string, 27)
	bd := &Breakdown{Noun: "Machines", Rows: map[string][]Slice{"r": {}}}
	for i := range names {
		names[i] = string(rune('a'+i%26)) + string(rune('0'+i/26))
		bd.Rows["r"] = append(bd.Rows["r"], Slice{Name: names[i]})
	}
	cols := machineColumns(nil, bd.Names(), metricFloor)
	if len(cols) != 27 {
		t.Fatalf("columns = %d, want 27", len(cols))
	}
	for i, want := range map[int]string{0: "A", 1: "B", 24: "Y", 25: "Z", 26: "["} {
		if cols[i].Title != want {
			t.Errorf("column %d title = %q, want %q", i, cols[i].Title, want)
		}
	}
}

// A-019: the shared width floors at 9 (MACHINE_COL_WIDTH = COST_WIDTH) even
// when every cell is $0.00, and grows to the longest sum.
func TestBreakdownWidthFloorAndGrowth(t *testing.T) {
	if w := machineWidth([]float64{0, 0}, []float64{0, 0}, Cost); w != metricFloor {
		t.Errorf("zero-cell width = %d, want %d", w, metricFloor)
	}
	// $6,288.75 is 9 chars; the $6,289.75 sum is 9 too — still the floor.
	if w := machineWidth([]float64{6288.75}, []float64{6288.75}, Cost); w != 9 {
		t.Errorf("width = %d, want 9", w)
	}
	if w := machineWidth([]float64{6288.75, 8000}, []float64{14288.75}, Cost); w != 10 {
		t.Errorf("width = %d, want 10 ($14,288.75)", w)
	}
}

func TestBreakdownCellsAndSums(t *testing.T) {
	bd := sampleBreakdown()
	names := bd.Names()
	cells := machineCells(bd, "Claude Code", names, Cost)
	if cells[0].Text != "$8.27" || cells[0].Dim {
		t.Errorf("cell 0 = %+v, want $8.27 not dim", cells[0])
	}
	if cells[1].Text != "$6,288.75" {
		t.Errorf("cell 1 = %+v, want $6,288.75", cells[1])
	}
	// A name the row has no slice for is 0 and dim.
	missing := machineCells(bd, "Codex", names, Cost)
	if missing[0].Text != "$0.00" || !missing[0].Dim {
		t.Errorf("missing slice cell = %+v, want dim $0.00", missing[0])
	}
	sums, vals := machineSums(bd, []string{"Claude Code", "Codex"}, names, Cost)
	if sums[0] != 8.27 || sums[1] != 6289.75 {
		t.Errorf("sums = %v, want [8.27 6289.75]", sums)
	}
	if len(vals) != 4 {
		t.Errorf("cell values = %v, want 4 (2 rows × 2 names)", vals)
	}
	// Token metric reads TotalTokens, not the cost.
	bd.Rows["Claude Code"][1].TotalTokens = 42
	if got := sliceValue(bd.Rows["Claude Code"], "Sahils-Mac-mini.local", Tokens); got != 42 {
		t.Errorf("token slice value = %v, want 42", got)
	}
}

func TestBreakdownNote(t *testing.T) {
	bd := sampleBreakdown()
	if got := bd.note(bd.Names()); got != "Machines: A = Sahils-Mac-mini.local, B = dev-ws-sahil02" {
		t.Errorf("note = %q", got)
	}
	users := &Breakdown{Noun: "Users", Rows: map[string][]Slice{"r": {{Name: "harness-user"}, {Name: "other-user"}}}}
	if got := users.note(users.Names()); got != "Users: A = harness-user, B = other-user" {
		t.Errorf("users note = %q", got)
	}
}
