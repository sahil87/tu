package watch

import (
	"strings"
	"testing"
	"time"

	"github.com/sahil87/tu/internal/render/ansi"
)

var panelNow = time.Date(2026, 9, 17, 9, 0, 0, 0, time.UTC)

func TestFormatElapsed(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{0, "0s"},
		{500 * time.Millisecond, "0s"},
		{11 * time.Second, "11s"},
		{59 * time.Second, "59s"},
		{60 * time.Second, "1m 0s"},
		{95 * time.Second, "1m 35s"},
		{time.Hour, "1h 0m 0s"},
		{2*time.Hour + 3*time.Minute + 4*time.Second, "2h 3m 4s"},
		{-5 * time.Second, "0s"},
	}
	for _, c := range cases {
		if got := FormatElapsed(c.d); got != c.want {
			t.Errorf("FormatElapsed(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestBurnRate(t *testing.T) {
	at := func(sec int) time.Time { return panelNow.Add(time.Duration(sec) * time.Second) }
	t.Run("below two polls", func(t *testing.T) {
		if _, ok := BurnRate([]PollPoint{{At: at(0), Cost: 1}}); ok {
			t.Errorf("one poll: ok = true, want false")
		}
		if _, ok := BurnRate(nil); ok {
			t.Errorf("no polls: ok = true, want false")
		}
	})
	t.Run("two polls", func(t *testing.T) {
		// $0.07 over 11 s → 0.07/11*3600 = 22.9090…/hr.
		rate, ok := BurnRate([]PollPoint{{At: at(0), Cost: 31.79}, {At: at(11), Cost: 31.86}})
		if !ok {
			t.Fatal("ok = false")
		}
		if got := round2(rate); got != 22.91 {
			t.Errorf("rate = %v (~%v), want ~22.91", rate, got)
		}
	})
	t.Run("zero delta time", func(t *testing.T) {
		rate, ok := BurnRate([]PollPoint{{At: at(0), Cost: 1}, {At: at(0), Cost: 5}})
		if !ok || rate != 0 {
			t.Errorf("Δt==0 → (%v, %v), want (0, true)", rate, ok)
		}
	})
	t.Run("negative rate", func(t *testing.T) {
		rate, ok := BurnRate([]PollPoint{{At: at(0), Cost: 5}, {At: at(10), Cost: 1}})
		if !ok || rate >= 0 {
			t.Errorf("negative rate = (%v, %v)", rate, ok)
		}
	})
	t.Run("rolling window of five", func(t *testing.T) {
		// 6 polls: the window is the last 5 (indices 1..5) — $1 → $6 over 50 s.
		polls := []PollPoint{{At: at(0), Cost: 100}}
		for i := 1; i <= 5; i++ {
			polls = append(polls, PollPoint{At: at(i * 10), Cost: float64(i)})
		}
		rate, ok := BurnRate(polls)
		if !ok {
			t.Fatal("ok = false")
		}
		// (6 − 1) / 50 s → 0.1/s → 360/hr.
		if got := round2(rate); got != 360 {
			t.Errorf("rate = %v (~%v), want 360 (window excludes the $100 poll)", rate, got)
		}
	})
}

// round2 is a test-local two-decimal rounding for float comparisons.
func round2(x float64) float64 {
	if x < 0 {
		return -round2(-x)
	}
	return float64(int(x*100+0.5)) / 100
}

// The R9 scenario: polls at t=0 ($31.79) and t=11s ($31.86), tokens
// 64,946,471 → 73,409,000, now = t+11s at 09:00 local, todayCost 31.86.
func TestStatsGridTwoPolls(t *testing.T) {
	start := panelNow.Add(-11 * time.Second)
	s := Session{
		StartTime:   start,
		StartCost:   31.79,
		StartTokens: 64946471,
		Polls: []PollPoint{
			{At: start, Cost: 31.79},
			{At: panelNow, Cost: 31.86},
		},
		TotalTokens: 73409000,
	}
	lines := StatsGrid(s, 31.86, panelNow, ansi.Colors{Enabled: true})
	if len(lines) != 4 {
		t.Fatalf("grid lines = %d, want 4", len(lines))
	}
	// 8,462,529 tokens over 11/60 min → ~46,159,249/min… the exact values are
	// pinned by the golden frames; here the shapes and colors are asserted.
	row1 := ansi.StripANSI(lines[0])
	if !strings.HasPrefix(row1, " Elapsed  11s") {
		t.Errorf("row 1 = %q, want %q prefix", row1, " Elapsed  11s")
	}
	if want := "Tok/min   ~"; !strings.Contains(row1, want) {
		t.Errorf("row 1 lacks %q: %q", want, row1)
	}
	row2 := ansi.StripANSI(lines[1])
	if want := "Session  +$0.07"; !strings.Contains(row2, want) {
		t.Errorf("row 2 lacks %q: %q", want, row2)
	}
	if want := "Rate      ~$22.91/hr"; !strings.Contains(row2, want) {
		t.Errorf("row 2 lacks %q: %q", want, row2)
	}
	// Rate is yellow when shown.
	if !strings.Contains(lines[1], "\x1b[33m~$22.91/hr\x1b[0m") {
		t.Errorf("Rate not yellow: %q", lines[1])
	}
	// Proj. day: 31.86 + 22.9090…×(24 − 9 − 0) = 31.86 + 343.636… = 375.496…
	row3 := ansi.StripANSI(lines[2])
	if want := "Proj. day ~$375.50"; !strings.Contains(row3, want) {
		t.Errorf("row 3 lacks %q: %q", want, row3)
	}
	// The separator matches the widest row's visible width.
	sep := ansi.StripANSI(lines[3])
	if len([]rune(sep)) < 35 {
		t.Errorf("separator = %d runes, want ≥ 35 once populated", len([]rune(sep)))
	}
}

// A single poll: Session "$0.00", the rest "--", separator 35 wide.
func TestStatsGridSinglePoll(t *testing.T) {
	s := Session{
		StartTime:   panelNow,
		StartCost:   31.79,
		StartTokens: 64946471,
		Polls:       []PollPoint{{At: panelNow, Cost: 31.79}},
		TotalTokens: 64946471,
	}
	lines := StatsGrid(s, 31.79, panelNow, ansi.Colors{})
	if len(lines) != 4 {
		t.Fatalf("grid lines = %d, want 4", len(lines))
	}
	for i, want := range []string{" Elapsed  0s", " Session  $0.00", "                       Proj. day --"} {
		if !strings.HasPrefix(lines[i], want) {
			t.Errorf("row %d = %q, want prefix %q", i+1, lines[i], want)
		}
	}
	for i, want := range []string{"Tok/min   --", "Rate      --", "Proj. day --"} {
		if !strings.HasSuffix(lines[i], want) {
			t.Errorf("row %d = %q, want suffix %q", i+1, lines[i], want)
		}
	}
	if got := len([]rune(lines[3])); got != 35 {
		t.Errorf("separator = %d runes, want 35", got)
	}
}

// The sign rule: a negative session delta renders "-$x.xx".
func TestStatsGridNegativeSessionDelta(t *testing.T) {
	s := Session{
		StartTime: panelNow.Add(-20 * time.Second),
		StartCost: 10.00,
		Polls: []PollPoint{
			{At: panelNow.Add(-20 * time.Second), Cost: 10.00},
			{At: panelNow, Cost: 9.50},
		},
		TotalTokens: 0,
	}
	lines := StatsGrid(s, 9.50, panelNow, ansi.Colors{})
	if !strings.Contains(ansi.StripANSI(lines[1]), "Session  -$0.50") {
		t.Errorf("row 2 = %q", lines[1])
	}
	// Negative burn rate → Rate/Proj. day stay "--".
	if !strings.Contains(ansi.StripANSI(lines[1]), "Rate      --") {
		t.Errorf("row 2 rate = %q", lines[1])
	}
}
