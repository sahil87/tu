// localecmp.go ports the two JavaScript orderings scripts/repair-metrics.mjs
// relies on, so the repair report lists files and per-user rows in the same
// order as the mjs (R11). Both tables below are Node-verified (v24,
// 2026-09-17) and pinned in localecmp_test.go.
package sync

import "strings"

// localeCompare is String.prototype.localeCompare under the ICU root
// collation (Node's default) restricted to the ASCII repertoire day-file
// paths contain — `/`, `-`, `.`, `_`, digits, letters. Primary order:
// punctuation (`_` < `-` < `.` < `/`) < digits (by value) < letters
// (case-insensitively, no numeric collation: "10" < "9" because '1' < '9').
// Strings equal at the primary level fall to the tertiary: at the first case
// difference the lowercase letter sorts first ("mac" < "Mac"). A shorter
// string that is a primary-level prefix sorts first.
//
// Documented limit: any byte outside that repertoire (punctuation other than
// the four above, non-ASCII) falls back to plain byte order at the position
// where it appears — real metrics paths never contain such bytes.
//
// Node-verified table: "_", "-", ".", "/", "0", "9", "a", "A", "b", "B",
// "m", "M", "z", "Z" is a sorted list; "aa" > "a-a" and "a1" < "aa" (digits
// and punctuation before letters); "sahil/2026/Sahils-Mac-mini.local/…" sorts
// AFTER "sahil/2026/dev-ws-sahil02/…" ('d' < 's' case-insensitively — byte
// order would put "Sahils" first).
func localeCompare(a, b string) int {
	n := min(len(a), len(b))
	for i := 0; i < n; i++ {
		ca, cb := a[i], b[i]
		if ca == cb {
			continue
		}
		wa, oka := icuPrimary(ca)
		wb, okb := icuPrimary(cb)
		if !oka || !okb {
			// Outside the repertoire → byte order (documented limit above).
			if ca < cb {
				return -1
			}
			return 1
		}
		if wa != wb {
			if wa < wb {
				return -1
			}
			return 1
		}
		// Equal primary weights with different bytes: case variants of one
		// letter — the tertiary pass below decides after all primary
		// differences (ICU compares level by level over the whole string).
	}
	if len(a) != len(b) {
		if len(a) < len(b) {
			return -1
		}
		return 1
	}
	for i := 0; i < n; i++ {
		ca, cb := a[i], b[i]
		if ca == cb {
			continue
		}
		// Bytes differ but primaries were equal, so both are the same letter
		// in different cases; lowercase first (the byte value of the
		// lowercase letter is higher).
		if ca >= 'a' && ca <= 'z' {
			return -1
		}
		return 1
	}
	return 0
}

// icuPrimary maps a byte of the day-file repertoire to its ICU root primary
// weight. The second return value is false for any other byte (the caller
// falls back to byte order).
func icuPrimary(c byte) (int, bool) {
	switch c {
	case '_':
		return 0, true
	case '-':
		return 1, true
	case '.':
		return 2, true
	case '/':
		return 3, true
	}
	if c >= '0' && c <= '9' {
		return 4 + int(c-'0'), true
	}
	if c >= 'a' && c <= 'z' {
		return 14 + int(c-'a'), true
	}
	if c >= 'A' && c <= 'Z' {
		return 14 + int(c-'A'), true
	}
	return 0, false
}

// userEntryCompare is the mjs per-user row order: [...byUser.entries()].sort()
// — the default JS sort, which stringifies each [user, agg] entry to
// "{user},[object Object]" and compares those by UTF-16 code units. For
// ASCII user names that is byte order of the decorated string; the decoration
// matters only when one name is a prefix of another whose next byte sorts
// below ',' (Node: "a!" sorts before "a"). Beyond ASCII this is UTF-8 byte
// order, not UTF-16 code-unit order (documented limit — equal inside the
// BMP, and day-file user dirs are account names).
func userEntryCompare(a, b string) int {
	const entrySuffix = ",[object Object]"
	return strings.Compare(a+entrySuffix, b+entrySuffix)
}
