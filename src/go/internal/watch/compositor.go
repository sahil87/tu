package watch

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/sahil87/tu/internal/render/ansi"
)

// The compositor constants (the TS compositor.ts / watch.ts).
const (
	// CompactThreshold: below this width the stats grid and rain drop out and
	// the compact table renders (the TS COMPACT_THRESHOLD).
	CompactThreshold = 60
	// RainTick is the rain animation's interval (the TS RAIN_TICK_MS — 75% of
	// the original 80 ms rate).
	RainTick = 107 * time.Millisecond
	// CountdownTick is the countdown timer's 1 s step (the TS
	// COUNTDOWN_TICK_MS).
	CountdownTick = time.Second
	// minRainCols is the right-margin zone's minimum usable width (the TS
	// MIN_RAIN_COLS).
	minRainCols = 10
	// rainGutter is the gap kept between the widest content line and the
	// first rain column in right-margin mode (the TS RAIN_GUTTER).
	rainGutter = 2
	// footerRows is the footer band's height.
	footerRows = 1
	// MaxRows is the watch row budget (the TS maxRows: 15 — a constant, not
	// "rows that fit the terminal height"; the spec sentence is a G0 flag).
	MaxRows = 15
)

// RainZone is the rain layer's geometry: Enabled, the zone's size, the
// 1-based StartRow and the 0-based StartCol (the right-margin offset).
type RainZone struct {
	Enabled  bool
	Cols     int
	Rows     int
	StartRow int
	StartCol int
}

// Layout is one frame's layout record: the compact switch, the content lines
// (stats nil when compact), the rain zone, and the terminal height the footer
// positions against.
type Layout struct {
	Compact bool
	Stats   []string
	Table   []string
	Rain    RainZone
	Rows    int // terminal rows (footer position)
}

// Lay reproduces the TS layoutAndUpdate + setupRainZone: compact when cols <
// 60 (stats dropped); contentHeight = len(stats) + len(table); available =
// rows − contentHeight − 1; wantRain = !noRain && !compact. Below-content
// zone {cols, available, startRow contentHeight+1, startCol 0} when wantRain
// && available > 0; else the right-margin zone {cols − maxContentWidth − 2,
// rows − 1, startRow 1, startCol maxContentWidth + 2} when that width is ≥
// 10; else disabled. maxContentWidth is the max StripANSI rune width over the
// content lines.
func Lay(statsLines, tableLines []string, cols, rows int, noRain bool) Layout {
	compact := cols < CompactThreshold
	l := Layout{Compact: compact, Rows: rows, Table: tableLines}
	if !compact {
		l.Stats = statsLines
	}
	contentHeight := len(l.Stats) + len(l.Table)
	maxContentWidth := 0
	for _, line := range l.Stats {
		if n := utf8.RuneCountInString(ansi.StripANSI(line)); n > maxContentWidth {
			maxContentWidth = n
		}
	}
	for _, line := range l.Table {
		if n := utf8.RuneCountInString(ansi.StripANSI(line)); n > maxContentWidth {
			maxContentWidth = n
		}
	}

	available := rows - contentHeight - footerRows
	wantRain := !noRain && !compact
	switch {
	case wantRain && available > 0:
		l.Rain = RainZone{Enabled: true, Cols: cols, Rows: available, StartRow: contentHeight + 1}
	case wantRain:
		marginCols := cols - maxContentWidth - rainGutter
		if marginCols >= minRainCols {
			l.Rain = RainZone{Enabled: true, Cols: marginCols, Rows: rows - footerRows, StartRow: 1, StartCol: maxContentWidth + rainGutter}
		}
	}
	return l
}

// LaySkeleton is the TS layoutForSkeleton: the skeleton's own lines are the
// whole content (no stats/table split).
func LaySkeleton(lines []string, cols, rows int, noRain bool) Layout {
	return Lay(nil, lines, cols, rows, noRain)
}

// Frame reproduces the TS flush: cursor home; each content line (stats then
// table) + `\x1b[K\n`; `\x1b[J`; the footer at the last row
// (`\x1b[{rows};1H\x1b[K{footer}`); then the current rain frame re-emitted —
// the flush's clears erased every drawn rain cell, and the rain renderer
// rewrites every occupied cell, so the re-emit restores them in the same
// write and polls do not blink the rain.
func (l Layout) Frame(footer string, rain string) []byte {
	var b strings.Builder
	b.WriteString("\x1b[H")
	for _, line := range l.Stats {
		b.WriteString(line + "\x1b[K\n")
	}
	for _, line := range l.Table {
		b.WriteString(line + "\x1b[K\n")
	}
	b.WriteString("\x1b[J")
	b.WriteString(FooterLine(footer, l.Rows))
	b.WriteString(rain)
	return []byte(b.String())
}

// FooterLine is the footer-only write (the TS writeFooterLine — renderStatus
// on every countdown change, push-driven, never on a periodic tick).
func FooterLine(footer string, rows int) string {
	return "\x1b[" + itoa(rows) + ";1H\x1b[K" + footer
}
