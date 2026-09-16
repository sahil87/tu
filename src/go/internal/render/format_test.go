package render

import (
	"math"
	"testing"
)

func TestFormatInt(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0"},
		{400, "400"},
		{3000, "3,000"},
		{24400, "24,400"},
		{999999, "999,999"},
		{1234567890123, "1,234,567,890,123"},
		{-24400, "-24,400"},
		{math.MinInt64, "-9,223,372,036,854,775,808"}, // -MinInt64 overflows int64
	}
	for _, c := range cases {
		if got := FormatInt(c.in); got != c.want {
			t.Errorf("FormatInt(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

// The thirteen node-verified toFixed(2) values (csv.Cost delegates here), the
// one-digit ties where half-even differs from half-up, and the sign rules.
func TestFixedHalfUp(t *testing.T) {
	cases := []struct {
		in     float64
		digits int
		want   string
	}{
		// toFixed(2), node-verified (v24, 2026-09-16).
		{1.005, 2, "1.00"},
		{0.125, 2, "0.13"},
		{0.375, 2, "0.38"},
		{2.675, 2, "2.67"},
		{0.015, 2, "0.01"},
		{1.045, 2, "1.04"},
		{8.345, 2, "8.35"},
		{0.005, 2, "0.01"},
		{0.045, 2, "0.04"},
		{999999.995, 2, "999999.99"},
		{1234567.891, 2, "1234567.89"},
		{0.5, 2, "0.50"},
		{0, 2, "0.00"},
		// toFixed(1): exact-binary ties round UP (Go 'f',1 is half-even).
		{12.25, 1, "12.3"},   // Go 'f',1: 12.2
		{0.05, 1, "0.1"},     // 0.05 is above the exact tie; Go 'f',1: 0.1 too, kept for parity
		{2.35, 1, "2.4"},     // exact 2.35 sits above the tie → up (both rules agree)
		{69.0, 1, "69.0"},    // the leaderboard share cell shape
		{100.0, 1, "100.0"},  // a 100% share widens the column
		{0.69, 1, "0.7"},     // exact 0.69 is above the tie
		{-0.3, 1, "-0.3"},    // sign re-added on a nonzero magnitude
		{-2.675, 2, "-2.67"}, // magnitude rounded, sign re-added
	}
	for _, c := range cases {
		if got := FixedHalfUp(c.in, c.digits); got != c.want {
			t.Errorf("FixedHalfUp(%v, %d) = %q, want %q", c.in, c.digits, got, c.want)
		}
	}
	if got := FixedHalfUp(math.Copysign(0, -1), 1); got != "0.0" {
		t.Errorf("FixedHalfUp(-0, 1) = %q, want 0.0", got)
	}
	// A negative that rounds to zero drops the sign.
	if got := FixedHalfUp(-0.004, 2); got != "0.00" {
		t.Errorf("FixedHalfUp(-0.004, 2) = %q, want 0.00", got)
	}
}

// JSRound is floor(x + 0.5) — half toward +∞, where math.Round is half away
// from zero.
func TestJSRound(t *testing.T) {
	cases := []struct {
		in   float64
		want float64
	}{
		{-553.5, -553}, // math.Round: -554
		{2.5, 3},
		{-2.5, -2}, // math.Round: -3
		{0.5, 1},
		{-0.4, 0},
		{0, 0},
		{4757.0, 4757},
		{55.5, 56},
	}
	for _, c := range cases {
		if got := JSRound(c.in); got != c.want {
			t.Errorf("JSRound(%v) = %v, want %v", c.in, got, c.want)
		}
	}
	// Negative zero normalizes (so a formatted result never shows "-0").
	if got := JSRound(-0.4); math.Signbit(got) {
		t.Errorf("JSRound(-0.4) = %v, want +0", got)
	}
}

// The R7 value table: ICU parity values verified against node v24 on
// 2026-09-16 — each differs from strconv.FormatFloat(x, 'f', 2, 64).
func TestFormatCost(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{1.005, "$1.01"},               // Go 'f',2 gives 1.00
		{0.125, "$0.13"},               // Go: 0.12
		{0.015, "$0.02"},               // Go: 0.01
		{2.675, "$2.68"},               // Go: 2.67
		{999999.995, "$1,000,000.00"},  // Go: 999999.99; carry crosses the integer part
		{4.936068800000001, "$4.94"},   // shortest repr rounds down here
		{1234567.891, "$1,234,567.89"}, // grouping plus rounding
		{0.5, "$0.50"},
		{0, "$0.00"},
		{0.994, "$0.99"},
		{0.995, "$1.00"}, // carry into the units digit
		{99.995, "$100.00"},
		{24400, "$24,400.00"},
		{math.Copysign(0, -1), "$0.00"}, // -0 normalizes
	}
	for _, c := range cases {
		if got := FormatCost(c.in); got != c.want {
			t.Errorf("FormatCost(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}
