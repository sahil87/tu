package json

import (
	"strconv"

	"github.com/sahil87/tu/internal/view"
)

// Leaderboard renders the lb JSON (the TS leaderboardRowsToJson through
// emitJson — JSON.stringify(v, null, 2)): a bare array, [] inline when
// empty, one object per row with keys in order rank (int), user, machine
// (only when the row carries one — the --by-machine conditional spread),
// cost (encodeFloat, raw double), totalTokens (int), share (encodeFloat),
// delta (encodeFloat or null for a "new" row). The caller passes the
// SLICED rows (--top truncates the array).
func Leaderboard(rows []view.LeaderboardRow) []string {
	if len(rows) == 0 {
		return []string{"[]"}
	}
	lines := []string{"["}
	for i, r := range rows {
		lines = append(lines, "  {")
		lines = append(lines, `    "rank": `+strconv.Itoa(r.Rank)+",")
		lines = append(lines, `    "user": `+encodeString(r.User)+",")
		if r.Machine != "" {
			lines = append(lines, `    "machine": `+encodeString(r.Machine)+",")
		}
		lines = append(lines, `    "cost": `+encodeFloat(r.TotalCost)+",")
		lines = append(lines, `    "totalTokens": `+strconv.FormatInt(r.TotalTokens, 10)+",")
		lines = append(lines, `    "share": `+encodeFloat(r.Share)+",")
		delta := "null"
		if r.Delta != nil {
			delta = encodeFloat(*r.Delta)
		}
		lines = append(lines, `    "delta": `+delta)
		closer := "  }"
		if i < len(rows)-1 {
			closer += ","
		}
		lines = append(lines, closer)
	}
	return append(lines, "]")
}
