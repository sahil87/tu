package command

import (
	"time"

	"github.com/sahil87/tu/internal/config"
	"github.com/sahil87/tu/internal/query"
)

// The warn-and-clear guard notices (byte-exact TS stderr lines; intake §9.1).
const (
	userNotice           = "Warning: -u flag requires multi mode — ignoring."
	byMachinePivotNotice = "Warning: --by-machine is not supported with all-tools history — ignoring."
	byMachineLbhNotice   = "Warning: --by-machine is not supported with leaderboard history — ignoring."
	sinceUntilNotice     = "Warning: --since/--until apply to history display — ignoring."
	fullNotice           = "Warning: --full applies to daily/weekly history — ignoring."
	topNotice            = "Warning: --top applies to leaderboard display — ignoring."
)

// leaderboard reports whether the display is one of the two leaderboards
// (lb/lbh) — the guard steps that exempt them test this.
func leaderboard(d Display) bool {
	return d == Leaderboard || d == LeaderboardHistory
}

// Normalize applies the TS main() flag guards in TS order and returns the
// request the pipeline runs plus the stderr notice lines the edge prints
// BEFORE any fetch warning. Pure — nothing below cmd/tu writes.
//
//  0. User set in single mode → notice; cleared. The leaderboards are exempt
//     (the TS excludes lb/lbh so the exit-1 ErrLeaderboardMode gate — which
//     runs in Run BEFORE Normalize — fires without a notice line; in multi
//     mode this step never runs).
//     0'. User == "all" on lb/lbh → cleared SILENTLY (the leaderboard is
//     inherently all-users). -u <name> is kept: it pins that user's row,
//     never filters.
//     0a. ByMachine on the all-tools history pivot (Source "" and History) →
//     notice; cleared. The single-tool history and the snapshots keep it.
//     0b. ByMachine on the leaderboard history → notice; cleared. --by-machine
//     on lb is kept (it keys rows by user/machine).
//  1. since/until set on a display outside {h, lb, lbh} → notice; both
//     cleared (so the snapshot is in scope after the clear and a well-shaped
//     impossible date like 2026-13-01 warns-and-renders, exit 0). The
//     leaderboards keep the window — on lb it REPLACES the period window.
//  2. Full on a display outside {h, lbh} → notice (the flag is left set;
//     nothing reads it downstream). lb --full warns like any snapshot; lbh
//     does not.
//     2b. Top set on a display outside {lb, lbh} → notice; cleared (B5 —
//     positioned after the --full notice, before the cap, the TS order).
//  3. Cap: display ∈ {h, lbh} ∧ period ≠ monthly ∧ no explicit bound ∧
//     !Full → Since = ThreeMonthFloor(now), capActive = true. An explicit
//     bound on either side disables the cap entirely (no intersection);
//     mh --full is a silent no-op. lb is never capped.
func Normalize(req Request, mode config.Mode, now time.Time) (Request, []string, bool) {
	var notices []string
	f := &req.Flags
	if f.User != "" && mode == config.Single && !leaderboard(req.Display) {
		notices = append(notices, userNotice)
		f.User = ""
	}
	if f.User == "all" && leaderboard(req.Display) {
		f.User = ""
	}
	if f.ByMachine && req.Source == "" && req.Display == History {
		notices = append(notices, byMachinePivotNotice)
		f.ByMachine = false
	}
	if f.ByMachine && req.Display == LeaderboardHistory {
		notices = append(notices, byMachineLbhNotice)
		f.ByMachine = false
	}
	if (f.Since != "" || f.Until != "") && req.Display != History && !leaderboard(req.Display) {
		notices = append(notices, sinceUntilNotice)
		f.Since, f.Until = "", ""
	}
	if f.Full && req.Display != History && req.Display != LeaderboardHistory {
		notices = append(notices, fullNotice)
	}
	if f.Top != 0 && !leaderboard(req.Display) {
		notices = append(notices, topNotice)
		f.Top = 0
	}
	capActive := false
	if (req.Display == History || req.Display == LeaderboardHistory) && req.Period != query.Monthly && f.Since == "" && f.Until == "" && !f.Full {
		f.Since = query.ThreeMonthFloor(now)
		capActive = true
	}
	return req, notices, capActive
}
