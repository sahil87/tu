package view

import (
	"math"
	"sort"
	"strings"
	"unicode/utf8"
)

// Bar geometry constants, lifted from the TS formatter (intake §4.6).
const (
	fullBlock = "█" // U+2588
	minBar    = "▏" // U+258F (1/8) — the floor for a nonzero sliver
	scaleRule = "┊" // U+250A — the two-zone scale-break rule

	minBarArea  = 10 // bars are suppressed below this width
	maxBarWidth = 30

	p95Percentile       = 95.0
	p95TriggerFactor    = 1.5 // two-zone engages when max > 1.5 × p95
	overflowZoneMin     = 4
	overflowZoneDivisor = 4 // overflow zone ≈ 1/4 of the bar area
)

// eighths maps a fractional eighth (0–7) to its block glyph (U+258F..U+2589).
var eighths = []string{"", "▏", "▎", "▍", "▌", "▋", "▊", "▉"}

// Glyphs renders value against max over width cells as raw block glyphs:
// "" when value == 0 or max == 0; scaled = value/max × width;
// full = floor(scaled); e = round((scaled − full) × 8) (JS Math.round: half
// up — identical for non-negatives); e == 8 carries to "█" × (full+1); else
// "█" × full + eighths[e]; an empty result becomes "▏".
func Glyphs(value, max float64, width int) string {
	if value == 0 || max == 0 {
		return ""
	}
	scaled := value / max * float64(width)
	full := int(math.Floor(scaled))
	e := int(math.Round((scaled - float64(full)) * 8))
	if e == 8 {
		return strings.Repeat(fullBlock, full+1)
	}
	bar := strings.Repeat(fullBlock, full) + eighths[e]
	if bar == "" {
		return minBar
	}
	return bar
}

// Percentile is the p-th percentile (0–100) of a sorted-ascending sample by
// linear interpolation: idx = p/100 × (n−1), between floor and ceil.
func Percentile(sortedAsc []float64, p float64) float64 {
	if len(sortedAsc) == 0 {
		return 0
	}
	idx := p / 100 * float64(len(sortedAsc)-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	return sortedAsc[lo] + (sortedAsc[hi]-sortedAsc[lo])*(idx-float64(lo))
}

// ComputeScale picks the bar scale for a visible window of row values:
// max over values; nonzero values sorted ascending; none → single-zone.
// p95 = Percentile(nonzero, 95); unless max > 1.5 × p95 → single-zone; else
// two-zone with OverflowZone = max(4, round(barWidth/4)) and
// MainZone = barWidth − OverflowZone − 1.
func ComputeScale(values []float64, barWidth int) Scale {
	maxValue := 0.0
	var nonzero []float64
	for _, v := range values {
		if v > maxValue {
			maxValue = v
		}
		if v > 0 {
			nonzero = append(nonzero, v)
		}
	}
	s := Scale{Max: maxValue, Width: barWidth}
	if len(nonzero) == 0 {
		return s
	}
	sort.Float64s(nonzero)
	p95 := Percentile(nonzero, p95Percentile)
	if !(maxValue > p95TriggerFactor*p95) {
		return s
	}
	s.TwoZone = true
	s.P95 = p95
	s.OverflowZone = max(overflowZoneMin, int(math.Round(float64(barWidth)/overflowZoneDivisor)))
	s.MainZone = barWidth - s.OverflowZone - 1
	return s
}

// Apportion distributes total glyphs over shares by largest remainder: floor
// each quota, then hand the remainder to the largest fractional parts, ties
// to the earlier index; a zero share gets zero; the result sums exactly to
// total.
func Apportion(shares []float64, total int) []int {
	counts := make([]int, len(shares))
	if total <= 0 {
		return counts
	}
	shareSum := 0.0
	for _, s := range shares {
		shareSum += s
	}
	if shareSum <= 0 {
		return counts
	}
	type remainder struct {
		i    int
		frac float64
	}
	rems := make([]remainder, len(shares))
	left := total
	for i, s := range shares {
		q := s / shareSum * float64(total)
		counts[i] = int(math.Floor(q))
		left -= counts[i]
		rems[i] = remainder{i, q - float64(counts[i])}
	}
	sort.SliceStable(rems, func(a, b int) bool {
		if rems[a].frac != rems[b].frac {
			return rems[a].frac > rems[b].frac
		}
		return rems[a].i < rems[b].i
	})
	for _, r := range rems {
		if left <= 0 {
			break
		}
		counts[r.i]++
		left--
	}
	return counts
}

// rowBar builds one row's bar under the scale: single-zone fills the whole
// width; two-zone caps the main zone at p95 and renders overflow only when
// the value exceeds it (a row at exactly p95 ends at the rule). segments is
// nil for the single-tool table (solid fill); the pivot passes its per-tool
// values, apportioned over Main's rune count.
func rowBar(v float64, s Scale, segments []float64) *Bar {
	var b Bar
	if !s.TwoZone {
		b.Main = Glyphs(v, s.Max, s.Width)
	} else {
		b.Main = Glyphs(min(v, s.P95), s.P95, s.MainZone)
		if v > s.P95 {
			b.Overflow = Glyphs(v-s.P95, s.Max-s.P95, s.OverflowZone)
		}
	}
	if segments != nil {
		b.Segments = Apportion(segments, utf8.RuneCountInString(b.Main))
	}
	return &b
}
