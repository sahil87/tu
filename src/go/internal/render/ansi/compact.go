package ansi

import (
	"strings"

	"github.com/sahil87/tu/internal/view"
)

// CompactTable encodes a view.CompactTable into output lines exactly as the
// TS compact renderers do (watch mode on narrow terminals):
//
//	"", BoldWhite(title), "",
//	if Empty: Empty, ""
//	else: per row PadRight(name, 14) + " " + PadLeft(value + delta, 12) — the
//	      DeltaPadsArrow placement (the JS padStart measures the raw string,
//	      so a colored arrow defeats the padding), [Dim("─"×27),
//	      BoldWhite("Total" padded) + " " + BoldWhite(value padded) when
//	      Total != nil], ""
func CompactTable(t view.CompactTable, c Colors) []string {
	lines := []string{"", c.BoldWhite(t.Title), ""}
	if t.Empty != "" {
		return append(lines, t.Empty, "")
	}
	for _, r := range t.Rows {
		v := r.Value
		switch r.Delta {
		case view.DeltaUp:
			v += " " + c.Green("↑")
		case view.DeltaDown:
			v += " " + c.Red("↓")
		}
		lines = append(lines, PadRight(r.Name, view.CompactNameWidth)+" "+PadLeft(v, view.CompactValueWidth))
	}
	if t.Total != nil {
		lines = append(lines, c.Dim(strings.Repeat("─", view.CompactDivWidth)))
		lines = append(lines, c.BoldWhite(PadRight("Total", view.CompactNameWidth))+" "+c.BoldWhite(PadLeft(t.Total.Value, view.CompactValueWidth)))
	}
	return append(lines, "")
}
