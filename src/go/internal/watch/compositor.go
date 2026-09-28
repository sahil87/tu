package watch

import (
	"strings"
	"time"
	"unicode"
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
// 10; else disabled. maxContentWidth is the max StripANSI column width over
// the content lines (visibleWidth — wide runes count 2).
//
// Every stats and table line is clipped to cols on intake (DC-17): a line
// wider than the terminal would wrap and desynchronize the one-row-per-line
// accounting the flush relies on. Clipped widths feed the zone math, so the
// rain zone and the clip agree and a full-width line disables the
// right-margin rain.
func Lay(statsLines, tableLines []string, cols, rows int, noRain bool) Layout {
	compact := cols < CompactThreshold
	l := Layout{Compact: compact, Rows: rows, Table: clipLines(tableLines, cols)}
	if !compact {
		l.Stats = clipLines(statsLines, cols)
	}
	contentHeight := len(l.Stats) + len(l.Table)
	maxContentWidth := 0
	for _, line := range l.Stats {
		if n := visibleWidth(line); n > maxContentWidth {
			maxContentWidth = n
		}
	}
	for _, line := range l.Table {
		if n := visibleWidth(line); n > maxContentWidth {
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

// clipLines returns the lines each clipped to cols visible columns (DC-17),
// in a fresh slice — the caller's lines are never mutated.
func clipLines(lines []string, cols int) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = clipLine(line, cols)
	}
	return out
}

// runeWidth is the terminal column width of one rune: 0 for zero-width runes
// — combining marks (unicode.Mn/Me, e.g. the accent in a decomposed é) and
// the format controls ZWSP/ZWNJ/ZWJ/word joiner/ZWNBSP — 2 for the standard
// wide ranges — emoji (U+1F300–U+1FAFF, so the 📊 heading counts 2), the
// emoji-presentation symbols (U+2600–U+27BF), and the East-Asian
// Wide/Fullwidth blocks — 1 for everything else the compositor draws: ASCII,
// box drawing, block bars, and the half-width katakana (U+FF61–U+FF9F,
// deliberately outside the fullwidth ranges below) the rain uses. The
// zero-width check runs first because the broad CJK ranges below include
// combining marks (e.g. U+3099–U+309A). No wcwidth helper exists in-tree or
// in x/term/x/sys, and a new dependency for one function is not warranted
// (constitution IV).
func runeWidth(r rune) int {
	if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) ||
		r == 0x200B || r == 0x200C || r == 0x200D || r == 0x2060 || r == 0xFEFF {
		return 0
	}
	switch {
	case r >= 0x1100 && r <= 0x115F, // Hangul Jamo
		r >= 0x2E80 && r <= 0xA4CF,   // CJK radicals … Yi
		r >= 0xAC00 && r <= 0xD7A3,   // Hangul syllables
		r >= 0xF900 && r <= 0xFAFF,   // CJK compatibility ideographs
		r >= 0xFE30 && r <= 0xFE4F,   // CJK compatibility forms
		r >= 0xFF00 && r <= 0xFF60,   // fullwidth forms
		r >= 0xFFE0 && r <= 0xFFE6,   // fullwidth signs
		r >= 0x2600 && r <= 0x27BF,   // emoji-presentation symbols
		r >= 0x1F300 && r <= 0x1FAFF, // emoji
		r >= 0x20000 && r <= 0x3FFFD: // CJK extension B and beyond
		return 2
	}
	return 1
}

// visibleWidth measures a line in terminal columns: the StripANSI visible
// text with wide runes counted as 2 (runeWidth).
func visibleWidth(line string) int {
	w := 0
	for _, r := range ansi.StripANSI(line) {
		w += runeWidth(r)
	}
	return w
}

// clipLine clips one frame line to cols visible columns, ANSI-aware: SGR
// escape sequences (the StripANSI shape) pass through uncounted, visible
// columns stop at cols, and a line clipped inside an SGR run is closed with
// \x1b[0m so the clip does not leak styling into the clear-to-EOL that
// follows. A line that fits is returned byte-identical. Width is measured in
// terminal columns (runeWidth — wide runes such as the 📊 heading count 2);
// zero-width runes (combining marks, joiners, variation selectors) pass
// through uncounted even at the clip boundary, so a clipped line never severs
// a combining mark from its base glyph, and a wide rune that would straddle
// the last column is dropped, not half-drawn. cols is always ≥ 1 through the
// Terminal's 80×24 fallback.
func clipLine(line string, cols int) string {
	if visibleWidth(line) <= cols {
		return line
	}
	var b strings.Builder
	b.Grow(len(line))
	visible := 0
	open := false
runes:
	for i := 0; i < len(line); {
		if line[i] == 0x1b && i+1 < len(line) && line[i+1] == '[' {
			j := i + 2
			for j < len(line) && (line[j] == ';' || (line[j] >= '0' && line[j] <= '9')) {
				j++
			}
			if j < len(line) && line[j] == 'm' {
				seq := line[i : j+1]
				b.WriteString(seq)
				open = seq != "\x1b[0m"
				i = j + 1
				continue
			}
		}
		r, size := utf8.DecodeRuneInString(line[i:])
		switch w := runeWidth(r); {
		case w == 0:
			// zero-width runes never consume the column budget
		case visible+w > cols:
			if visible == cols {
				break runes
			}
			i += size // a wide rune straddling the last column is dropped
			continue
		default:
			visible += w
		}
		b.WriteString(line[i : i+size])
		i += size
	}
	if open {
		b.WriteString("\x1b[0m")
	}
	return b.String()
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
