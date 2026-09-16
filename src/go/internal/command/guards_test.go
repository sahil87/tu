package command

import (
	"testing"
	"time"

	"github.com/sahil87/tu/internal/config"
	"github.com/sahil87/tu/internal/query"
)

var guardsNow = time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC) // floor: 2026-07-01

func TestNormalizeSinceUntilGuard(t *testing.T) {
	// Snapshot with a window: notice, both cleared, in scope afterwards.
	req, notices, capActive := Normalize(Request{
		Display: Snapshot,
		Flags:   Flags{Since: "2026-13-01", Until: "2026-01-31"},
	}, config.Single, guardsNow)
	if len(notices) != 1 || notices[0] != sinceUntilNotice {
		t.Errorf("notices = %v, want the since/until notice", notices)
	}
	if req.Flags.Since != "" || req.Flags.Until != "" {
		t.Errorf("flags = %+v, want both cleared", req.Flags)
	}
	if capActive {
		t.Error("capActive on a snapshot")
	}

	// History with a window: no notice, no cap.
	req, notices, capActive = Normalize(Request{
		Display: History,
		Flags:   Flags{Since: "2026-01-01"},
	}, config.Single, guardsNow)
	if len(notices) != 0 || capActive || req.Flags.Since != "2026-01-01" {
		t.Errorf("history+since: notices = %v, capActive = %v, since = %q", notices, capActive, req.Flags.Since)
	}
}

func TestNormalizeFullGuard(t *testing.T) {
	// Full on a snapshot: notice; the flag is left set (nothing reads it).
	req, notices, _ := Normalize(Request{Flags: Flags{Full: true}}, config.Single, guardsNow)
	if len(notices) != 1 || notices[0] != fullNotice {
		t.Errorf("notices = %v, want the --full notice", notices)
	}
	if !req.Flags.Full {
		t.Error("Full cleared, want left set")
	}

	// mh --full: silent no-op (monthly is never capped).
	_, notices, capActive := Normalize(Request{Display: History, Period: query.Monthly, Flags: Flags{Full: true}}, config.Single, guardsNow)
	if len(notices) != 0 || capActive {
		t.Errorf("mh --full: notices = %v, capActive = %v, want silent no-op", notices, capActive)
	}

	// --full with an explicit window: silently accepted, no cap.
	req, _, capActive = Normalize(Request{Display: History, Flags: Flags{Full: true, Since: "2026-01-01"}}, config.Single, guardsNow)
	if capActive || req.Flags.Since != "2026-01-01" {
		t.Errorf("h --full --since: capActive = %v, since = %q", capActive, req.Flags.Since)
	}
}

// The cap predicate over the eight period/since/until/full combinations.
func TestNormalizeCap(t *testing.T) {
	cases := []struct {
		name      string
		period    query.Period
		since     string
		until     string
		full      bool
		wantCap   bool
		wantSince string
	}{
		{"daily bare caps", query.Daily, "", "", false, true, "2026-07-01"},
		{"weekly bare caps", query.Weekly, "", "", false, true, "2026-07-01"},
		{"monthly exempt", query.Monthly, "", "", false, false, ""},
		{"explicit since disables", query.Daily, "2026-01-01", "", false, false, "2026-01-01"},
		{"explicit until disables", query.Daily, "", "2026-01-31", false, false, ""},
		{"both explicit disables", query.Weekly, "2026-01-01", "2026-01-31", false, false, "2026-01-01"},
		{"full disables", query.Daily, "", "", true, false, ""},
		{"full weekly disables", query.Weekly, "", "", true, false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req, notices, capActive := Normalize(Request{
				Display: History,
				Period:  c.period,
				Flags:   Flags{Since: c.since, Until: c.until, Full: c.full},
			}, config.Single, guardsNow)
			if capActive != c.wantCap {
				t.Errorf("capActive = %v, want %v", capActive, c.wantCap)
			}
			if req.Flags.Since != c.wantSince {
				t.Errorf("Since = %q, want %q", req.Flags.Since, c.wantSince)
			}
			if len(notices) != 0 {
				t.Errorf("notices = %v, want none on history", notices)
			}
		})
	}
}

func TestNormalizeCapFloorThreadedFromNow(t *testing.T) {
	jan := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	req, _, capActive := Normalize(Request{Display: History}, config.Single, jan)
	if !capActive || req.Flags.Since != "2025-11-01" {
		t.Errorf("Since = %q capActive = %v, want 2025-11-01 true (year rollover)", req.Flags.Since, capActive)
	}
}

func TestNormalizeNoticeOrder(t *testing.T) {
	// Both guards fire in TS order on a snapshot carrying both flag kinds.
	_, notices, _ := Normalize(Request{Flags: Flags{Since: "2026-01-01", Full: true}}, config.Single, guardsNow)
	if len(notices) != 2 || notices[0] != sinceUntilNotice || notices[1] != fullNotice {
		t.Errorf("notices = %v, want since/until then full", notices)
	}
}

// R8: the single-mode -u guard fires first and clears the flag, for -u all as
// for any name; in multi mode the flag passes through untouched.
func TestNormalizeUserGuard(t *testing.T) {
	cases := []struct {
		name      string
		req       Request
		mode      config.Mode
		wantUser  string
		wantNotes []string
	}{
		{"single -u name", Request{Flags: Flags{User: "other-user"}}, config.Single, "", []string{userNotice}},
		{"single -u all", Request{Flags: Flags{User: "all"}}, config.Single, "", []string{userNotice}},
		{"multi -u name", Request{Flags: Flags{User: "other-user"}}, config.Multi, "other-user", nil},
		{"multi -u all", Request{Flags: Flags{User: "all"}}, config.Multi, "all", nil},
		{"no -u", Request{}, config.Single, "", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req, notices, _ := Normalize(c.req, c.mode, guardsNow)
			if req.Flags.User != c.wantUser {
				t.Errorf("User = %q, want %q", req.Flags.User, c.wantUser)
			}
			if len(notices) != len(c.wantNotes) {
				t.Fatalf("notices = %v, want %v", notices, c.wantNotes)
			}
			for i, n := range c.wantNotes {
				if notices[i] != n {
					t.Errorf("notices[%d] = %q, want %q", i, notices[i], n)
				}
			}
		})
	}
}

// R8/A-017: on a snapshot carrying both -u and --since, the -u notice
// precedes the since/until notice (the TS main() guard order).
func TestNormalizeUserNoticeFirst(t *testing.T) {
	_, notices, _ := Normalize(Request{
		Flags: Flags{User: "other-user", Since: "2026-01-01"},
	}, config.Single, guardsNow)
	if len(notices) != 2 || notices[0] != userNotice || notices[1] != sinceUntilNotice {
		t.Errorf("notices = %v, want -u then since/until", notices)
	}
}

// R1: the two --by-machine warn-and-clear guards, in TS main() position —
// after the -u guard, before the since/until guard.
func TestNormalizeByMachineGuards(t *testing.T) {
	cases := []struct {
		name      string
		req       Request
		mode      config.Mode
		wantFlag  bool
		wantNotes []string
	}{
		{"all-tools history", Request{Display: History, Flags: Flags{ByMachine: true}}, config.Single, false, []string{byMachinePivotNotice}},
		{"all-tools history multi", Request{Display: History, Flags: Flags{ByMachine: true}}, config.Multi, false, []string{byMachinePivotNotice}},
		{"leaderboard history", Request{Display: LeaderboardHistory, Flags: Flags{ByMachine: true}}, config.Multi, false, []string{byMachineLbhNotice}},
		{"single-source history keeps it", Request{Source: "cc", Display: History, Flags: Flags{ByMachine: true}}, config.Single, true, nil},
		{"all-tools snapshot keeps it", Request{Flags: Flags{ByMachine: true}}, config.Single, true, nil},
		{"single-source snapshot keeps it", Request{Source: "cc", Flags: Flags{ByMachine: true}}, config.Multi, true, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req, notices, _ := Normalize(c.req, c.mode, guardsNow)
			if req.Flags.ByMachine != c.wantFlag {
				t.Errorf("ByMachine = %v, want %v", req.Flags.ByMachine, c.wantFlag)
			}
			if len(notices) != len(c.wantNotes) {
				t.Fatalf("notices = %v, want %v", notices, c.wantNotes)
			}
			for i, n := range c.wantNotes {
				if notices[i] != n {
					t.Errorf("notices[%d] = %q, want %q", i, notices[i], n)
				}
			}
		})
	}
}

// R1: `-u other --by-machine h` in single mode emits the -u line first, then
// the pivot line; both flags are cleared.
func TestNormalizeByMachineNoticeOrder(t *testing.T) {
	req, notices, _ := Normalize(Request{
		Display: History,
		Flags:   Flags{User: "other-user", ByMachine: true},
	}, config.Single, guardsNow)
	if len(notices) != 2 || notices[0] != userNotice || notices[1] != byMachinePivotNotice {
		t.Errorf("notices = %v, want -u then the pivot notice", notices)
	}
	if req.Flags.ByMachine || req.Flags.User != "" {
		t.Errorf("flags = %+v, want both cleared", req.Flags)
	}
}
