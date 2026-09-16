package ansi

import (
	"strings"
	"unicode/utf8"

	"github.com/sahil87/tu/internal/view"
)

// cellSep joins padded cells; the divider's columns are joined by divSep.
const (
	cellSep = " | "
	divSep  = "─|─"
)

// scaleBreak is the two-zone bar's dim rule (the TS SCALE_BREAK_RULE, U+250A).
const scaleBreak = "┊"

// Table encodes a view.Table into output lines exactly as the TS renderers
// do:
//
//	"", BoldWhite(title), "",
//	if Empty: Empty, ""
//	else: header (each cell padded per column, then BoldCyan-wrapped
//	      INDIVIDUALLY, joined " | "), Dim(divider + barDiv), data rows
//	      (padded cells joined " | "; the Date cell wrapped per its
//	      LabelStyle; Dim cells dim-wrapped; delta indicator and bar
//	      appended), [Dim(divider + barDiv), total (each cell padded then
//	      BoldWhite-wrapped, joined " | ")], [Dim(Footer) + legend],
//	      ["", Dim(Note)], ""
//
// barDiv is "─" + "─"×Scale.Width when bars are shown (Scale.Width > 0) and
// extends every divider — header, month separators, and the Total divider.
// The snapshot divider is 87 visible characters. Note is the --by-machine
// legend: a blank line then the dim text, after the footer (or the last row
// when there is no footer), before the trailing blank.
func Table(t view.Table, c Colors) []string {
	lines := []string{"", c.BoldWhite(t.Title), ""}
	if t.Empty != "" {
		return append(lines, t.Empty, "")
	}
	barDiv := ""
	if t.Scale.Width > 0 {
		barDiv = "─" + strings.Repeat("─", t.Scale.Width)
	}
	for _, row := range t.Rows {
		switch row.Kind {
		case view.Header:
			lines = append(lines, joinStyled(t.Columns, row.Cells, c.BoldCyan))
		case view.Divider, view.Separator:
			lines = append(lines, c.Dim(divider(t.Columns)+barDiv))
		case view.Total:
			lines = append(lines, joinStyled(t.Columns, row.Cells, c.BoldWhite))
		default: // Data
			lines = append(lines, c.dataRow(t, row))
		}
	}
	if t.Footer != "" {
		lines = append(lines, c.footerLine(t.Footer, t.Legend))
	}
	if t.Note != "" {
		lines = append(lines, "", c.Dim(t.Note))
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

// dataRow renders a Data row: padded cells joined " | " with the per-cell
// styles applied pad-first (the Date cell per its LabelStyle, exact-zero
// metric cells Dim), then the delta indicator (spaced or abutting per
// DeltaSpaced), then the bar.
func (c Colors) dataRow(t view.Table, row view.Row) string {
	parts := make([]string, len(row.Cells))
	for i, cell := range row.Cells {
		padded := padCell(t.Columns[i], cell)
		switch {
		case i == 0 && cell.Style == view.Current:
			padded = c.BoldWhite(padded)
		case i == 0 && cell.Style == view.Weekend:
			padded = c.Dim(padded)
		case cell.Dim:
			padded = c.Dim(padded)
		}
		parts[i] = padded
	}
	line := strings.Join(parts, cellSep)
	if row.Delta != view.DeltaNone {
		arrow := c.Green("↑")
		if row.Delta == view.DeltaDown {
			arrow = c.Red("↓")
		}
		if t.DeltaSpaced {
			line += " "
		}
		line += arrow
	}
	if row.Bar != nil && t.Scale.Width > 0 {
		line += c.encodeBar(row.Bar, t.Scale)
	}
	return line
}

// encodeBar renders a row's bar (leading space included). Single-zone: " " +
// fill(Main) only when Main != "". Two-zone: ALWAYS " " + fill(Main) + pad +
// Dim("┊") + Yellow(Overflow) + pad — including a zero row, and empty strings
// DO get wrapped (Green("")/Yellow("") emit ESC[32mESC[0m / ESC[33mESC[0m —
// the TS wrap() has no empty guard; the goldens pin this).
func (c Colors) encodeBar(b *view.Bar, s view.Scale) string {
	if !s.TwoZone {
		if b.Main == "" {
			return ""
		}
		return " " + c.fill(b.Main, b.Segments)
	}
	main := c.fill(b.Main, b.Segments) + strings.Repeat(" ", s.MainZone-utf8.RuneCountInString(b.Main))
	overflow := c.Yellow(b.Overflow) + strings.Repeat(" ", s.OverflowZone-utf8.RuneCountInString(b.Overflow))
	return " " + main + c.Dim(scaleBreak) + overflow
}

// fill colors a bar's raw glyphs: solid Green when segments is nil (the
// single-tool table), else contiguous runs by palette slot over exact rune
// slices (zero-count segments skipped). Stripping ANSI yields Main unchanged.
func (c Colors) fill(main string, segments []int) string {
	if segments == nil {
		return c.Green(main)
	}
	runes := []rune(main)
	var b strings.Builder
	pos := 0
	for i, n := range segments {
		if n == 0 {
			continue
		}
		b.WriteString(c.palette(i)(string(runes[pos : pos+n])))
		pos += n
	}
	return b.String()
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

// footerLine renders the history summary footer: the whole text dim-wrapped,
// then — when a legend exists and color is enabled — a dim separator and one
// swatch per tool (palette block + space + dim name, joined by spaces). The
// swatches carry their own resets, so the legend is NOT part of the dim span.
// With color disabled the legend is omitted entirely (the TS gates on
// colorDisabled()).
func (c Colors) footerLine(footer string, legend []view.Swatch) string {
	line := c.Dim(footer)
	if legend == nil || !c.Enabled {
		return line
	}
	swatches := make([]string, len(legend))
	for i, s := range legend {
		swatches[i] = c.palette(s.Palette)("█") + " " + c.Dim(s.Name)
	}
	return line + c.Dim(" · ") + strings.Join(swatches, " ")
}
