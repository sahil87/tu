package command

import (
	"time"

	"github.com/sahil87/tu/internal/query"
)

// The warn-and-clear guard notices (byte-exact TS stderr lines; intake §9.1).
const (
	sinceUntilNotice = "Warning: --since/--until apply to history display — ignoring."
	fullNotice       = "Warning: --full applies to daily/weekly history — ignoring."
)

// Normalize applies the TS main() flag guards in TS order and returns the
// request the pipeline runs plus the stderr notice lines the edge prints
// BEFORE any fetch warning. Pure — nothing below cmd/tu writes. (B4 inserts
// its --by-machine pivot guard before step 1; B5 inserts --top after step 2.)
//
//  1. since/until set on a non-history display → notice; both cleared (so the
//     snapshot is in scope after the clear and a well-shaped impossible date
//     like 2026-13-01 warns-and-renders, exit 0).
//  2. Full on a non-history display → notice (the flag is left set; nothing
//     reads it downstream).
//  3. Cap: history ∧ period ≠ monthly ∧ no explicit bound ∧ !Full → Since =
//     ThreeMonthFloor(now), capActive = true. An explicit bound on either
//     side disables the cap entirely (no intersection); mh --full is a silent
//     no-op.
func Normalize(req Request, now time.Time) (Request, []string, bool) {
	var notices []string
	f := &req.Flags
	if (f.Since != "" || f.Until != "") && req.Display != History {
		notices = append(notices, sinceUntilNotice)
		f.Since, f.Until = "", ""
	}
	if f.Full && req.Display != History {
		notices = append(notices, fullNotice)
	}
	capActive := false
	if req.Display == History && req.Period != query.Monthly && f.Since == "" && f.Until == "" && !f.Full {
		f.Since = query.ThreeMonthFloor(now)
		capActive = true
	}
	return req, notices, capActive
}
