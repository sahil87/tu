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
	}
	for _, c := range cases {
		if got := FormatInt(c.in); got != c.want {
			t.Errorf("FormatInt(%d) = %q, want %q", c.in, got, c.want)
		}
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
