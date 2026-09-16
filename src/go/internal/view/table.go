// Package view is the table model: a query result shaped into columns, rows,
// and cells, with no ANSI and no I/O. Render encoders (render/ansi,
// render/json, …) consume the model.
package view

// Align is a column's horizontal alignment.
type Align int

const (
	Left Align = iota
	Right
)

// RowKind classifies a table row for the encoder.
type RowKind int

const (
	Header RowKind = iota
	Divider
	Data
	Total
)

// Column is one column's title, fixed width in runes, and alignment.
type Column struct {
	Title string
	Width int
	Align Align
}

// Cell is one rendered cell. Text is pre-formatted; Dim marks exact-zero
// cell styling (pivot/machine columns) — false everywhere in the snapshot.
type Cell struct {
	Text string
	Dim  bool
}

// Row is one table row. Bar and Delta are reserved slots, empty in V2
// (B2 bars, B7 deltas).
type Row struct {
	Kind  RowKind
	Cells []Cell
	Bar   string
	Delta string
}

// Table is the render-agnostic table model.
type Table struct {
	Title   string   // e.g. "📊 Combined Usage (daily)"
	Columns []Column // fixed layout; Rows is nil when Empty != ""
	Rows    []Row    // header, divider, data…, [divider, total]
	Empty   string   // "  No usage" when no row is visible, else ""
	Legend  string   // reserved (machine legend); ""
	Footer  string   // reserved (history summary footer); ""
}
