package view

import (
	"sort"
	"strings"

	"github.com/sahil87/tu/internal/fact"
)

// Breakdown is the --by-machine (or -u all, by user) column set for one table
// (the TS machineCosts map): row key → slices in first-seen order. The row key
// is the tool display name on the snapshot and the ISO label on the
// single-tool history. Slices carry full Totals, so the ANSI table picks the
// display metric while JSON/CSV/Markdown read TotalCost — one structure, no
// second build (the TS builds two maps).
type Breakdown struct {
	Noun string             // "Machines" or "Users" — the legend label
	Rows map[string][]Slice // row key → slices in first-seen order
}

// Slice is one breakdown cell's source: the machine/user name and its totals.
type Slice struct {
	Name string
	fact.Totals
}

// Names is the sorted union of every slice name across all rows (byte order,
// equal to the JS default sort for ASCII hostnames) — the TS
// collectMachineNames / buildMachineColumns name set. A nil breakdown — or
// one whose rows carry no slices — yields nil: today's output byte-for-byte.
func (bd *Breakdown) Names() []string {
	if bd == nil {
		return nil
	}
	seen := make(map[string]bool)
	var names []string
	for _, slices := range bd.Rows {
		for _, s := range slices {
			if !seen[s.Name] {
				seen[s.Name] = true
				names = append(names, s.Name)
			}
		}
	}
	sort.Strings(names)
	return names
}

// letter is a machine column's header: A…Z then [, \… exactly as the TS
// String.fromCharCode(65+i) — no cap.
func letter(i int) string {
	return string(rune('A' + i))
}

// note is the trailing dim legend line (the TS renderMachineLegend):
// "{Noun}: A = name, B = name, …".
func (bd *Breakdown) note(names []string) string {
	parts := make([]string, len(names))
	for i, name := range names {
		parts[i] = letter(i) + " = " + name
	}
	return bd.Noun + ": " + strings.Join(parts, ", ")
}

// value is one row's cell value for name in the given metric — 0 when the row
// has no slice for it (the TS `rowMachines?.get(mc.name) ?? 0`).
func sliceValue(slices []Slice, name string, m Metric) float64 {
	for _, s := range slices {
		if s.Name == name {
			return metricValue(s.Totals, m)
		}
	}
	return 0
}

// Slices is one row's slices in first-seen order — the order the JSON
// `machines` object keys follow. Nil-safe on a nil breakdown.
func (bd *Breakdown) Slices(key string) []Slice {
	if bd == nil {
		return nil
	}
	return bd.Rows[key]
}

// CostOf is one row's cost for name, 0 when the row has no slice for it — the
// value the machine formats (JSON/CSV/Markdown) read even under -t, since that
// contract stays cost-denominated. One lookup path for every encoder.
func (bd *Breakdown) CostOf(key, name string) float64 {
	return sliceValue(bd.Slices(key), name, Cost)
}

// machineColumns appends the letter-coded machine columns (one per name, all
// sharing the data-sized width, floored at metricFloor — the TS
// MACHINE_COL_WIDTH = COST_WIDTH) after the metric column.
func machineColumns(cols []Column, names []string, width int) []Column {
	out := make([]Column, 0, len(cols)+len(names))
	out = append(out, cols...)
	for i := range names {
		out = append(out, Column{Title: letter(i), Width: width, Align: Right})
	}
	return out
}

// machineCells is one Data row's machine cells: metricCell per name in name
// order (dim on exact zero). A nil breakdown (or no names) yields nil.
func machineCells(bd *Breakdown, key string, names []string, m Metric) []Cell {
	if len(names) == 0 {
		return nil
	}
	cells := make([]Cell, len(names))
	for i, name := range names {
		cells[i] = metricCell(sliceValue(bd.Rows[key], name, m), m)
	}
	return cells
}

// machineSums accumulates the per-name sums the Total row carries and returns
// every visited cell value for the width pre-pass. The caller decides which
// rows visit (snapshot: visible rows only; history: every entry).
func machineSums(bd *Breakdown, keys []string, names []string, m Metric) (sums []float64, cellValues []float64) {
	sums = make([]float64, len(names))
	for _, key := range keys {
		for i, name := range names {
			v := sliceValue(bd.Rows[key], name, m)
			cellValues = append(cellValues, v)
			sums[i] += v
		}
	}
	return sums, cellValues
}

// machineWidth is the shared machine-column width: the data-sized metric
// width over every cell value plus the per-name Total sums (floor 9).
func machineWidth(cellValues, sums []float64, m Metric) int {
	return metricColumnWidth(append(cellValues, sums...), m)
}

// machineTotalCells is the Total row's per-name sums (fmtMetric, never dim —
// the encoder BoldWhite-wraps Total cells). No names yields nil.
func machineTotalCells(sums []float64, m Metric) []Cell {
	if len(sums) == 0 {
		return nil
	}
	cells := make([]Cell, len(sums))
	for i, v := range sums {
		cells[i] = Cell{Text: fmtMetric(v, m)}
	}
	return cells
}
