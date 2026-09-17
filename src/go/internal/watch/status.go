package watch

import (
	"unicode/utf8"

	"github.com/sahil87/tu/internal/render/ansi"
)

// The footer parts (the TS StatusPanel): the countdown/refreshing status and
// the controls hint, joined by a dim middle dot.
const controlsHint = "↵ refresh · q quit"

// Footer builds the status footer (the TS StatusPanel.render): Dim("Next
// refresh: {n}s") — or Dim("Refreshing...") when refreshing — and
// Dim("↵ refresh · q quit"), joined with Dim(" · "). While the visible width
// exceeds the terminal width and more than one part remains, the last part is
// dropped and the parts re-joined (controls first, then only the status text
// remains).
func Footer(countdown int, refreshing bool, width int, c ansi.Colors) string {
	var parts []string
	if refreshing {
		parts = append(parts, c.Dim("Refreshing..."))
	} else {
		parts = append(parts, c.Dim("Next refresh: "+itoa(countdown)+"s"))
	}
	parts = append(parts, c.Dim(controlsHint))

	line := joinFooter(parts, c)
	for utf8.RuneCountInString(ansi.StripANSI(line)) > width && len(parts) > 1 {
		parts = parts[:len(parts)-1]
		line = joinFooter(parts, c)
	}
	return line
}

func joinFooter(parts []string, c ansi.Colors) string {
	line := parts[0]
	for _, p := range parts[1:] {
		line += c.Dim(" · ") + p
	}
	return line
}
