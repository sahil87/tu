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
//	if Empty: Empty, "", then — when Footer != "" — Dim(Footer), ""
//	else: header (each cell padded per column, then BoldCyan-wrapped
//	      INDIVIDUALLY, joined " | "), Dim(divider + barDiv), data rows
//	      (padded cells joined " | "; the Date cell wrapped per its
//	      LabelStyle; Dim cells dim-wrapped; Leader cells BoldWhite-wrapped
//	      after padding and any Dim wrap; delta indicator and bar
//	      appended), [Dim(divider + barDiv), total (each cell padded then
//	      BoldWhite-wrapped, joined " | ")], [Dim(Footer) + legend],
//	      ["", Dim(Note)], ""
//
// barDiv is "─" + "─"×Scale.Width when bars are shown (Scale.Width > 0) and
// extends every divider — header, month separators, and the Total divider.
// When a column carries BarAfter (the leaderboard's mid-row bar) the bar area
// — exactly 1 + Scale.Width visible characters — renders immediately after
// that column's cell on every non-divider row instead of trailing the last
// cell, and barDiv follows that column's dashes in the dividers; Header,
// Total and bar-less Data rows get " " + spaces(Scale.Width) UNSTYLED
// (outside the cell wraps). The snapshot divider is 87 visible characters.
// Note is the --by-machine legend: a blank line then the dim text, after the
// footer (or the last row when there is none), before the trailing blank.
func Table(t view.Table, c Colors) []string {
	lines := []string{"", c.BoldWhite(t.Title), ""}
	if t.Empty != "" {
		lines = append(lines, t.Empty, "")
		if t.Footer != "" {
			lines = append(lines, c.Dim(t.Footer), "")
		}
		return lines
	}
	barDiv := ""
	if t.Scale.Width > 0 {
		barDiv = "─" + strings.Repeat("─", t.Scale.Width)
	}
	barAfter := -1
	for i, col := range t.Columns {
		if col.BarAfter {
			barAfter = i
			break
		}
	}
	for _, row := range t.Rows {
		switch row.Kind {
		case view.Header:
			lines = append(lines, joinStyled(t.Columns, row.Cells, c.BoldCyan, t.Scale, barAfter))
		case view.Divider, view.Separator:
			lines = append(lines, c.Dim(divider(t.Columns, barDiv, barAfter)))
		case view.Total:
			lines = append(lines, joinStyled(t.Columns, row.Cells, c.BoldWhite, t.Scale, barAfter))
		default: // Data
			lines = append(lines, c.dataRow(t, row, barAfter))
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

// padDeltaCell pads a text+arrow composite per the column alignment — the
// DeltaPadsArrow form, where the arrow's escape codes count toward the width
// (rune count over the colored string, the JS padStart raw-length twin).
func padDeltaCell(col view.Column, text string) string {
	if col.Align == view.Left {
		return PadRight(text, col.Width)
	}
	return PadLeft(text, col.Width)
}

// barGap is the unstyled bar area for a non-bar row under BarAfter: " " +
// spaces(Scale.Width) (the TS header/total/collapsed rows).
func barGap(s view.Scale) string {
	return " " + strings.Repeat(" ", s.Width)
}

// dataRow renders a Data row: padded cells joined " | " with the per-cell
// styles applied pad-first (the Date cell per its LabelStyle, exact-zero
// metric cells Dim), then the delta indicator (spaced or abutting per
// DeltaSpaced), then the bar. A Leader cell is BoldWhite-wrapped after
// padding and any Dim wrap (the TS double wrap: a zero leader cell is
// boldWhite(dim(text))). With a BarAfter column the bar area sits
// immediately after that column's cell (padded single-zone, full-width
// two-zone, an unstyled gap when the row carries no bar) and nothing trails.
//
// An in-cell delta (Cell.Delta — the snapshot and leaderboard placement)
// composes per the table's DeltaInCell BEFORE the Dim wrap: DeltaPadsArrow
// pads Text + arrow together (the JS raw-length padding quirk — the arrow's
// escape codes count toward the width, so a colored cell renders effectively
// unpadded) and DeltaAfterPad appends the arrow after the padded text; the
// exact-zero Dim wrap covers the composite either way.
func (c Colors) dataRow(t view.Table, row view.Row, barAfter int) string {
	parts := make([]string, len(row.Cells))
	for i, cell := range row.Cells {
		padded := padCell(t.Columns[i], cell)
		if cell.Delta != view.DeltaNone {
			arrow := c.Green("↑")
			if cell.Delta == view.DeltaDown {
				arrow = c.Red("↓")
			}
			if t.DeltaInCell == view.DeltaAfterPad {
				padded += " " + arrow
			} else {
				padded = padDeltaCell(t.Columns[i], cell.Text+" "+arrow)
			}
		}
		switch {
		case i == 0 && cell.Style == view.Current:
			padded = c.BoldWhite(padded)
		case i == 0 && cell.Style == view.Weekend:
			padded = c.Dim(padded)
		case cell.Dim:
			padded = c.Dim(padded)
		}
		if cell.Leader {
			padded = c.BoldWhite(padded)
		}
		if i == barAfter && t.Scale.Width > 0 {
			padded += c.barArea(row.Bar, t.Scale)
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
	if barAfter < 0 && row.Bar != nil && t.Scale.Width > 0 {
		line += c.encodeBar(row.Bar, t.Scale)
	}
	return line
}

// barArea renders the mid-row bar area (leading space included) for a Data
// row under BarAfter — the TS renderScaledBar output padded to barWidth + 1
// visible characters. Single-zone: " " + Green(Main) + spaces(Width −
// runes(Main)); an empty Main (and a bar-less row) is Width + 1 unstyled
// spaces (the TS `" ".repeat(barWidth + 1)`). Two-zone: the existing
// full-width form, unchanged.
func (c Colors) barArea(b *view.Bar, s view.Scale) string {
	if b == nil {
		return barGap(s)
	}
	if s.TwoZone {
		return c.encodeBar(b, s)
	}
	if b.Main == "" {
		return barGap(s)
	}
	return " " + c.fill(b.Main, b.Segments) + strings.Repeat(" ", s.Width-utf8.RuneCountInString(b.Main))
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
// wrapped in the style individually; the " | " separators stay unstyled. With
// a BarAfter column and bars shown, the unstyled bar gap (" " +
// spaces(Width)) follows that column's wrapped cell (outside the wrap).
func joinStyled(cols []view.Column, cells []view.Cell, style func(string) string, s view.Scale, barAfter int) string {
	parts := make([]string, len(cells))
	for i, cell := range cells {
		parts[i] = style(padCell(cols[i], cell))
	}
	if barAfter >= 0 && s.Width > 0 {
		parts[barAfter] += barGap(s)
	}
	return strings.Join(parts, cellSep)
}

// divider builds "─"×width per column joined with "─|─". barDiv follows the
// BarAfter column's dashes when set (the leaderboard's mid-row bar), else
// trails the last column (the history tables).
func divider(cols []view.Column, barDiv string, barAfter int) string {
	parts := make([]string, len(cols))
	for i, col := range cols {
		parts[i] = strings.Repeat("─", col.Width)
	}
	if barAfter >= 0 && barDiv != "" {
		parts[barAfter] += barDiv
		return strings.Join(parts, divSep)
	}
	return strings.Join(parts, divSep) + barDiv
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
