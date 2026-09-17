package watch

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/sahil87/tu/internal/render/ansi"
)

// Skeleton reproduces the TS renderSkeleton(termWidth): the loading frame
// written on alt-screen entry before the first poll. Full mode: the stats
// grid for a zeroed session (Elapsed "0s", Session "$0.00", the rest "--"),
// "", BoldWhite("📊 Combined Usage (daily)") — the daily snapshot header
// regardless of the display type (a G0 flag; reproduced, not "improved") —
// "", the BoldCyan snapshot header, the Dim 87-char divider, and the centered
// "Loading..." placeholder (floor((87 − 10) / 2) = 38 leading spaces).
// Compact: "" then Dim("Loading...").
func Skeleton(cols int, now time.Time, c ansi.Colors) []string {
	if cols < CompactThreshold {
		return []string{"", c.Dim("Loading...")}
	}

	lines := StatsGrid(Session{StartTime: now}, 0, now, c)
	lines = append(lines, "", c.BoldWhite("📊 Combined Usage (daily)"), "")

	header := c.BoldCyan(ansi.PadRight("Tool", 12)) +
		" | " + c.BoldCyan(ansi.PadLeft("Tokens", 12)) +
		" | " + c.BoldCyan(ansi.PadLeft("Input", 12)) +
		" | " + c.BoldCyan(ansi.PadLeft("Output", 12)) +
		" | " + c.BoldCyan(ansi.PadLeft("Cache", 12)) +
		" | " + c.BoldCyan(ansi.PadLeft("Cost", 12))
	lines = append(lines, header)

	divider := strings.Repeat("─", 12) +
		"─|─" + strings.Repeat("─", 12) +
		"─|─" + strings.Repeat("─", 12) +
		"─|─" + strings.Repeat("─", 12) +
		"─|─" + strings.Repeat("─", 12) +
		"─|─" + strings.Repeat("─", 12)
	lines = append(lines, c.Dim(divider))

	const loadingText = "Loading..."
	pad := (utf8.RuneCountInString(divider) - len(loadingText)) / 2
	if pad < 0 {
		pad = 0
	}
	return append(lines, strings.Repeat(" ", pad)+c.Dim(loadingText))
}

// SkeletonFrame is the skeleton's write form (the TS renderSkeleton's write):
// cursor home, then each line + "\n" (no \x1b[K).
func SkeletonFrame(lines []string) []byte {
	var b strings.Builder
	b.WriteString("\x1b[H")
	for _, line := range lines {
		b.WriteString(line + "\n")
	}
	return []byte(b.String())
}
