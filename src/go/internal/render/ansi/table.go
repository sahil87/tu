package ansi

import (
	"strings"

	"github.com/sahil87/tu/internal/view"
)

// cellSep joins padded cells; the divider's columns are joined by divSep.
const (
	cellSep = " | "
	divSep  = "─|─"
)

// Table encodes a view.Table into output lines exactly as the TS renderTotal
// does:
//
//	"", BoldWhite(title), "",
//	if Empty: Empty, ""
//	else: header (each cell padded per column, then BoldCyan-wrapped
//	      INDIVIDUALLY, joined " | "), Dim(divider), data rows plain (padded
//	      cells joined " | "), [Dim(divider), total (each cell padded then
//	      BoldWhite-wrapped, joined " | ")], [Legend/Footer when non-empty], ""
//
// The divider is "─"×width per column joined with "─|─" (87 visible chars for
// the snapshot).
func Table(t view.Table, c Colors) []string {
	lines := []string{"", c.BoldWhite(t.Title), ""}
	if t.Empty != "" {
		return append(lines, t.Empty, "")
	}
	for _, row := range t.Rows {
		switch row.Kind {
		case view.Header:
			lines = append(lines, joinStyled(t.Columns, row.Cells, c.BoldCyan))
		case view.Divider:
			lines = append(lines, c.Dim(divider(t.Columns)))
		case view.Total:
			lines = append(lines, joinStyled(t.Columns, row.Cells, c.BoldWhite))
		default: // Data
			lines = append(lines, joinPlain(t.Columns, row.Cells))
		}
	}
	if t.Legend != "" {
		lines = append(lines, t.Legend)
	}
	if t.Footer != "" {
		lines = append(lines, t.Footer)
	}
	return append(lines, "")
}

// padCell pads a cell to its column width and alignment.
func padCell(col view.Column, cell view.Cell) string {
	if col.Align == view.Left {
		return PadRight(cell.Text, col.Width)
	}
	return PadLeft(cell.Text, col.Width)
}

// joinPlain renders a data row: padded cells joined " | ", no escapes.
func joinPlain(cols []view.Column, cells []view.Cell) string {
	parts := make([]string, len(cells))
	for i, cell := range cells {
		parts[i] = padCell(cols[i], cell)
	}
	return strings.Join(parts, cellSep)
}

// joinStyled renders a header or total row: each cell padded first, then
// wrapped in the style individually; the " | " separators stay unstyled.
func joinStyled(cols []view.Column, cells []view.Cell, style func(string) string) string {
	parts := make([]string, len(cells))
	for i, cell := range cells {
		parts[i] = style(padCell(cols[i], cell))
	}
	return strings.Join(parts, cellSep)
}

// divider builds "─"×width per column joined with "─|─".
func divider(cols []view.Column) string {
	parts := make([]string, len(cols))
	for i, col := range cols {
		parts[i] = strings.Repeat("─", col.Width)
	}
	return strings.Join(parts, divSep)
}
