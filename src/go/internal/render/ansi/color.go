// Package ansi is the terminal-table encoder: it turns a view.Table into
// output lines with ANSI color, honoring NO_COLOR via the Colors value (the
// TS setNoColor module global becomes a parameter). Encoders return []string;
// nothing here writes to a stream.
package ansi

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// Colors carries the color-enabled decision as a value, computed once at the
// command edge (!--no-color && NO_COLOR == "") and passed down.
type Colors struct {
	Enabled bool
}

// The palette codes from the layouts Color Reference; reset closes every wrap.
const (
	codeBold        = "\x1b[1m"
	codeDim         = "\x1b[2m"
	codeGreen       = "\x1b[32m"
	codeRed         = "\x1b[31m"
	codeCyan        = "\x1b[36m"
	codeYellow      = "\x1b[33m"
	codeMagenta     = "\x1b[35m"
	codeBlue        = "\x1b[34m"
	codeBoldWhite   = "\x1b[1;37m"
	codeBoldCyan    = "\x1b[1;36m"
	codeBrightGreen = "\x1b[92m"
	codeDimGreen    = "\x1b[2;32m"
	codeReset       = "\x1b[0m"
)

// wrap encloses s in the given code when color is enabled, else returns s.
func (c Colors) wrap(code, s string) string {
	if !c.Enabled {
		return s
	}
	return code + s + codeReset
}

func (c Colors) Bold(s string) string        { return c.wrap(codeBold, s) }
func (c Colors) Dim(s string) string         { return c.wrap(codeDim, s) }
func (c Colors) Green(s string) string       { return c.wrap(codeGreen, s) }
func (c Colors) Red(s string) string         { return c.wrap(codeRed, s) }
func (c Colors) Cyan(s string) string        { return c.wrap(codeCyan, s) }
func (c Colors) Yellow(s string) string      { return c.wrap(codeYellow, s) }
func (c Colors) Magenta(s string) string     { return c.wrap(codeMagenta, s) }
func (c Colors) Blue(s string) string        { return c.wrap(codeBlue, s) }
func (c Colors) BoldWhite(s string) string   { return c.wrap(codeBoldWhite, s) }
func (c Colors) BoldCyan(s string) string    { return c.wrap(codeBoldCyan, s) }
func (c Colors) BrightGreen(s string) string { return c.wrap(codeBrightGreen, s) }
func (c Colors) DimGreen(s string) string    { return c.wrap(codeDimGreen, s) }

// palette is the stacked-segment color lookup (the TS STACK_PALETTE): slots
// 0–3 are Green, Magenta, Blue, Cyan; slot ≥ 4 is uncolored. Yellow is
// excluded — it is reserved for the two-zone overflow.
func (c Colors) palette(slot int) func(string) string {
	switch slot {
	case 0:
		return c.Green
	case 1:
		return c.Magenta
	case 2:
		return c.Blue
	case 3:
		return c.Cyan
	default:
		return func(s string) string { return s }
	}
}

// ansiRE matches one SGR escape sequence (the TS stripAnsi pattern).
var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// StripANSI removes SGR escape sequences from s.
func StripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

// PadRight pads s with trailing spaces to width w by rune count (the TS
// padEnd). Snapshot text is ASCII; the 📊 title is never padded.
func PadRight(s string, w int) string {
	if n := utf8.RuneCountInString(s); n < w {
		return s + strings.Repeat(" ", w-n)
	}
	return s
}

// PadLeft pads s with leading spaces to width w by rune count (padStart).
func PadLeft(s string, w int) string {
	if n := utf8.RuneCountInString(s); n < w {
		return strings.Repeat(" ", w-n) + s
	}
	return s
}
