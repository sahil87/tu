// Package view is the table model: a query result shaped into columns, rows,
// and cells, with no ANSI and no I/O. Render encoders (render/ansi,
// render/json, render/csv, render/markdown) consume the model.
//
// The snapshot (snapshot.go) and the two history tables (history.go,
// pivot.go) share the model; bar.go holds the bar geometry (raw glyphs plus
// Scale — the encoder colors, pads and slices) and footer.go the history
// summary line. No escape sequence exists anywhere in this package, and
// nothing here imports os, fmt.Print*, io writers, or the ccusage package.
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
	// Separator is the dim month-boundary divider between daily Data rows.
	Separator
)

// LabelStyle marks a Data row's first (Date) cell: Current when the label is
// the current period's (boldWhite), Weekend on a daily Saturday/Sunday (dim),
// Plain otherwise. The encoder decides the colors.
type LabelStyle int

const (
	Plain LabelStyle = iota
	Current
	Weekend
)

// Column is one column's title, fixed width in runes, and alignment.
type Column struct {
	Title string
	Width int
	Align Align
	// BarAfter renders the bar area (exactly 1 + Scale.Width visible chars)
	// immediately after this column's cell instead of trailing the last cell —
	// the leaderboard's mid-row bar. False everywhere else ⇒ the bar trails.
	BarAfter bool
}

// Cell is one rendered cell. Text is pre-formatted; Dim marks exact-zero
// cell styling (pivot/machine columns) — false everywhere in the snapshot.
// Style is used only by a Data row's first (Date) cell. Leader wraps the
// padded cell (after any Dim wrap) in BoldWhite — the lbh per-row leader.
// Delta is the watch-mode in-cell arrow (the snapshot and leaderboard
// placement); the table's DeltaInCell selects how it composes with padding.
type Cell struct {
	Text   string
	Dim    bool
	Style  LabelStyle
	Leader bool
	Delta  Delta
}

// DeltaInCell selects how a Cell.Delta arrow composes with the cell's
// padding — the TS places the watch arrow three different ways and these are
// the two in-cell forms (the history tables keep the trailing Row.Delta).
type DeltaInCell int

const (
	// DeltaPadsArrow pads text + arrow together: PadLeft(Text + " ↑", width).
	// The JS padStart measures raw string length, so a colored arrow's escape
	// codes count toward the width — with color on, the cell renders
	// effectively unpadded (the snapshot and compact tables).
	DeltaPadsArrow DeltaInCell = iota
	// DeltaAfterPad pads the text first, then appends the arrow:
	// PadLeft(Text, width) + " ↑"; the exact-zero Dim wrap covers the
	// composite (the leaderboard).
	DeltaAfterPad
)

// Delta is a watch-mode delta direction (B7 feeds the Prev map; B2 wires the
// seam): none, up (green ↑), or down (red ↓).
type Delta int

const (
	DeltaNone Delta = iota
	DeltaUp
	DeltaDown
)

// Bar is one row's bar, already reduced to raw glyphs and geometry (no ANSI).
type Bar struct {
	Main     string // raw block glyphs for the main zone (or the whole bar single-zone)
	Overflow string // raw glyphs past the p95 rule; "" unless two-zone and value > p95
	// Segments apportions Main over the visible tools (pivot stacked bars);
	// nil = solid fill (single-tool history).
	Segments []int
}

// Scale is the bar geometry shared by every row of a table; Width 0 means
// bars are suppressed. TwoZone engages the p95 scale-break: MainZone glyphs
// scale 0→P95, one rule char separates, OverflowZone glyphs scale P95→Max.
type Scale struct {
	TwoZone      bool
	Max, P95     float64
	Width        int // barWidth; 0 = bars suppressed
	MainZone     int // Width − OverflowZone − 1 (two-zone only)
	OverflowZone int // max(4, round(Width / 4)) (two-zone only)
}

// Swatch is one stacked-bar legend entry: tool name plus palette slot
// (0 green, 1 magenta, 2 blue, 3 cyan, ≥4 uncolored).
type Swatch struct {
	Name    string
	Palette int
}

// Row is one table row. Bar is nil when bars are suppressed or the row is not
// Data; Delta is DeltaNone without a Prev map.
type Row struct {
	Kind  RowKind
	Cells []Cell
	Bar   *Bar
	Delta Delta
}

// Table is the render-agnostic table model.
type Table struct {
	Title   string   // e.g. "📊 Combined Usage (daily)"
	Columns []Column // fixed layout; Rows is nil when Empty != ""
	Rows    []Row    // header, divider, data…, [divider, total]
	Empty   string   // "  No usage" (snapshot) / "  No data" (history), else ""
	Footer  string   // history summary line text (dim at encode); "" when absent
	Legend  []Swatch // stacked-bar legend; nil unless bars shown ∧ ≥2 visible tools
	// Note is a trailing dim line (the --by-machine legend), preceded by a
	// blank line and rendered after Footer; "" when absent.
	Note  string
	Scale Scale // bar geometry; Width 0 when no bars
	// DeltaSpaced selects the delta form: true renders " ↑" (single-tool
	// history); false renders "↑" abutting the cell (the pivot's width contract).
	DeltaSpaced bool
	// DeltaInCell selects the in-cell delta placement (DeltaPadsArrow for the
	// snapshot, DeltaAfterPad for the leaderboard); only Data-row cells with
	// Delta != DeltaNone are affected.
	DeltaInCell DeltaInCell
}
