// Package render holds the number formatting shared by the encoder
// subpackages (ansi now, markdown later): en-US thousands grouping and the
// ICU rounding rule for costs. The encoders return []string; nothing in the
// render tree writes to a stream.
package render

import (
	"math"
	"math/big"
	"strconv"
	"strings"
)

// FormatInt renders n with en-US thousands grouping (the TS fmtNum on
// integers): 24400 → "24,400".
func FormatInt(n int64) string {
	if n < 0 {
		// Magnitude through unsigned arithmetic: -n overflows on MinInt64.
		return "-" + group(strconv.FormatUint(uint64(-(n+1))+1, 10))
	}
	return group(strconv.FormatInt(n, 10))
}

// FormatCost renders x as "$" + the value with exactly two decimals and
// grouping, reproducing toLocaleString("en-US", {minimumFractionDigits: 2,
// maximumFractionDigits: 2}).
//
// This is NOT strconv.FormatFloat(x, 'f', 2, 64): ICU rounds the shortest
// round-trip decimal representation of the double half away from zero, while
// Go's 'f' rounds the exact binary value half-even (1.005 → "$1.01" vs Go's
// "1.00"; 999999.995 → "$1,000,000.00" vs Go's "999999.99"). The algorithm
// takes the shortest repr ('f', -1 — Go's and V8's shortest-digit algorithms
// agree), rounds that decimal string to two fraction digits half away from
// zero with carry, then groups the integer part. Negative zero renders
// "$0.00" (costs are non-negative sums).
func FormatCost(x float64) string {
	if x == 0 {
		return "$0.00"
	}
	sign := ""
	if x < 0 {
		sign = "-"
		x = -x
	}
	return "$" + sign + round2(strconv.FormatFloat(x, 'f', -1, 64))
}

// FixedHalfUp formats x with exactly digits fraction digits, rounding the
// EXACT binary value half-up — the JS toFixed(digits) rule (the leaderboard's
// share cell is toFixed(1); the CSV cost field is toFixed(2)). This is NOT
// strconv.FormatFloat(x, 'f', digits, 64): Go's 'f' rounds the exact value
// half-EVEN on ties (12.25 → "12.2" at one digit; toFixed(1) gives "12.3").
// big.Rat holds the exact value; scale by 10^digits, truncate, round up when
// the remainder ≥ 1/2; the magnitude is rounded and the sign re-added only
// when the result is nonzero, so -0 (and a negative that rounds to zero)
// renders unsigned.
//
// Node-verified (v24, 2026-09-16), two digits: 1.005 → 1.00, 0.125 → 0.13,
// 2.675 → 2.67, 0.015 → 0.01, 1.045 → 1.04, 0.045 → 0.04, 999999.995 →
// 999999.99, 1234567.891 → 1234567.89, 0.5 → 0.50, 0 → 0.00.
func FixedHalfUp(x float64, digits int) string {
	scale := int64(1)
	for range digits {
		scale *= 10
	}
	// exact = the binary value as a rational; scaled = exact × 10^digits.
	r := new(big.Rat).SetFloat64(x)
	if x < 0 {
		r.Neg(r) // round the magnitude; re-add the sign at the end
	}
	scaled := new(big.Rat).Mul(r, big.NewRat(scale, 1))
	intPart := new(big.Int)
	remainder := new(big.Rat)
	intPart.Quo(scaled.Num(), scaled.Denom()) // truncated; scaled ≥ 0 so == floor
	remainder.Sub(scaled, new(big.Rat).SetInt(intPart))
	if remainder.Cmp(big.NewRat(1, 2)) >= 0 {
		intPart.Add(intPart, big.NewInt(1))
	}
	digitsStr := intPart.String()
	var out string
	if digits == 0 {
		out = digitsStr
	} else {
		if len(digitsStr) < digits+1 {
			digitsStr = strings.Repeat("0", digits+1-len(digitsStr)) + digitsStr
		}
		out = digitsStr[:len(digitsStr)-digits] + "." + digitsStr[len(digitsStr)-digits:]
	}
	if x < 0 && intPart.Sign() != 0 {
		return "-" + out
	}
	return out
}

// JSRound is the Math.round twin: floor(x + 0.5), i.e. half toward +∞ — NOT
// Go's math.Round, which rounds half away from zero (Math.round(-553.5) is
// -553; math.Round gives -554). Negative zero is normalized to 0. The
// leaderboard's Δ percent and CSV share/delta fractions are byte surfaces
// rounded by this rule.
func JSRound(x float64) float64 {
	r := math.Floor(x + 0.5)
	if r == 0 {
		return 0 // normalize -0
	}
	return r
}

// round2 rounds a non-negative decimal string (strconv 'f' form, no exponent)
// to exactly two fraction digits, half away from zero with carry into the
// integer part, and groups the integer part with commas.
func round2(s string) string {
	intPart, frac, _ := strings.Cut(s, ".")

	// All significant digits, integer then fraction, as a byte slice; the
	// split point tracks where the decimal point sits.
	digits := []byte(intPart + frac)
	point := len(intPart)

	if len(digits) > point+2 {
		// Round half away from zero on the magnitude: look at the first
		// dropped digit.
		drop := digits[point+2]
		digits = digits[:point+2]
		if drop >= '5' {
			for i := len(digits) - 1; ; i-- {
				if i < 0 {
					// Carry ran off the front: "99.9" → "100.0".
					digits = append([]byte{'1'}, digits...)
					point++
					break
				}
				if digits[i] == '9' {
					digits[i] = '0'
					continue
				}
				digits[i]++
				break
			}
		}
	}

	// Pad short fractions ("0.5" → digits "05", point 1 → "0.50").
	for len(digits) < point+2 {
		digits = append(digits, '0')
	}

	intDigits := string(digits[:point])
	fracDigits := string(digits[point:])
	// Strip leading zeros beyond the first ("007" → "7" cannot occur from a
	// shortest repr, but a carry can produce it: "0.995" → "1.00" via the
	// front-carry path already handles the point shift).
	intDigits = strings.TrimLeft(intDigits, "0")
	if intDigits == "" {
		intDigits = "0"
	}
	return group(intDigits) + "." + fracDigits
}

// group inserts thousands separators into a digit string.
func group(digits string) string {
	n := len(digits)
	if n <= 3 {
		return digits
	}
	var b strings.Builder
	lead := n % 3
	if lead > 0 {
		b.WriteString(digits[:lead])
	}
	for i := lead; i < n; i += 3 {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(digits[i : i+3])
	}
	return b.String()
}
