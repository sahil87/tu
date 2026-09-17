package watch

import (
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/sahil87/tu/internal/render"
	"github.com/sahil87/tu/internal/render/ansi"
)

// itoa is the package's one int→string helper (cursor-position sequences and
// the elapsed units).
func itoa[N int | int64](n N) string { return strconv.FormatInt(int64(n), 10) }

// PollPoint is one successful poll: when it landed and the total cost then
// (the TS pollHistory entries).
type PollPoint struct {
	At   time.Time
	Cost float64
}

// Session is the watch session's stats state (the TS PanelSession).
type Session struct {
	StartTime   time.Time
	StartCost   float64
	StartTokens int64
	Polls       []PollPoint
	TotalTokens int64
}

// rollingWindow is the burn-rate window: the last 5 polls (the TS
// ROLLING_WINDOW).
const rollingWindow = 5

// The stats grid layout constants (the TS formatGridRow): a 1-space indent,
// the 9-wide left label, the 8-wide left-value budget, the 5-column gap, the
// 10-wide right label, and the "--" placeholder.
const (
	leftLabelW    = 9
	maxLeftValueW = 8
	gridGap       = 5
	rightLabelW   = 10
	placeholder   = "--"
)

// FormatElapsed renders a duration as the TS formatElapsed: floor seconds,
// "Xh Xm Xs" / "Xm Xs" / "Xs". Negative durations clamp to 0.
func FormatElapsed(d time.Duration) string {
	totalSec := int64(d / time.Second)
	if totalSec < 0 {
		totalSec = 0
	}
	h := totalSec / 3600
	m := (totalSec % 3600) / 60
	s := totalSec % 60
	if h > 0 {
		return itoa(h) + "h " + itoa(m) + "m " + itoa(s) + "s"
	}
	if m > 0 {
		return itoa(m) + "m " + itoa(s) + "s"
	}
	return itoa(s) + "s"
}

// BurnRate is the TS computeBurnRate: $/hour over the last rollingWindow
// polls; !ok below two polls, 0 when the window's Δt is 0.
func BurnRate(polls []PollPoint) (perHour float64, ok bool) {
	if len(polls) < 2 {
		return 0, false
	}
	windowStart := max(0, len(polls)-rollingWindow)
	oldest := polls[windowStart]
	latest := polls[len(polls)-1]
	dt := latest.At.Sub(oldest.At)
	if dt == 0 {
		return 0, true
	}
	return (latest.Cost - oldest.Cost) / (float64(dt) / float64(time.Hour)), true
}

// StatsGrid builds the 2×3 stats grid plus the dim separator (the TS
// buildStatsGrid), with now injected:
//
//	Row 1: Elapsed | Tok/min   Row 2: Session | Rate   Row 3: (blank) | Proj. day
//
// Session shows "$0.00" before two polls (the sign is "+" for a delta ≥ 0);
// Tok/min, Rate and Proj. day show "--" until then (Rate/Proj. day until the
// burn rate is positive). The Rate value is yellow when shown. todayCost is
// the rendered rows' total (the TS passes getCost() unchanged, whatever the
// display). Proj. day uses hours remaining in LOCAL time.
func StatsGrid(s Session, todayCost float64, now time.Time, c ansi.Colors) []string {
	elapsed := now.Sub(s.StartTime)
	elapsedMin := float64(elapsed) / float64(time.Minute)
	hasTwoPolls := len(s.Polls) > 1

	elapsedVal := FormatElapsed(elapsed)

	sessionVal := "$" + render.FixedHalfUp(0, 2)
	if hasTwoPolls {
		delta := s.Polls[len(s.Polls)-1].Cost - s.StartCost
		sign := "+"
		if delta < 0 {
			sign = "-"
		}
		if delta < 0 {
			delta = -delta
		}
		sessionVal = sign + "$" + render.FixedHalfUp(delta, 2)
	}

	tokMinVal := placeholder
	sessionTokens := s.TotalTokens - s.StartTokens
	if hasTwoPolls && sessionTokens > 0 && elapsedMin > 0 {
		tokMinVal = "~" + render.FormatInt(int64(render.JSRound(float64(sessionTokens)/elapsedMin)))
	}

	rateVal := placeholder
	projVal := placeholder
	if rate, ok := BurnRate(s.Polls); ok && rate > 0 {
		rateVal = "~$" + render.FixedHalfUp(rate, 2) + "/hr"
		hoursRemaining := 24 - float64(now.Hour()) - float64(now.Minute())/60
		projVal = "~$" + render.FixedHalfUp(todayCost+rate*hoursRemaining, 2)
	}

	lines := []string{
		gridRow("Elapsed", elapsedVal, "Tok/min", tokMinVal, false, c),
		gridRow("Session", sessionVal, "Rate", rateVal, true, c),
		gridRow("", "", "Proj. day", projVal, false, c),
	}

	maxVisible := 0
	for _, l := range lines {
		if n := utf8.RuneCountInString(ansi.StripANSI(l)); n > maxVisible {
			maxVisible = n
		}
	}
	return append(lines, c.Dim(strings.Repeat("─", maxVisible)))
}

// gridRow is the TS formatGridRow: Dim(" " + PadRight(label, 9)) +
// BoldWhite(value) on the left (12 spaces for the blank row-3 left), then
// max(1, 23 − leftVisible) spaces, then Dim(PadRight(rightLabel, 10)) + the
// value (Yellow on the Rate row when shown, BoldWhite otherwise).
func gridRow(leftLabel, leftValue, rightLabel, rightValue string, rateHighlight bool, c ansi.Colors) string {
	var left string
	leftVisible := 1 + leftLabelW + len(placeholder)
	if leftLabel != "" {
		left = c.Dim(" "+ansi.PadRight(leftLabel, leftLabelW)) + c.BoldWhite(leftValue)
		leftVisible = 1 + leftLabelW + len(leftValue)
	} else {
		left = strings.Repeat(" ", leftVisible)
	}

	rightPad := max(1, 1+leftLabelW+maxLeftValueW+gridGap-leftVisible)

	value := c.BoldWhite(rightValue)
	if rateHighlight && rightValue != placeholder {
		value = c.Yellow(rightValue)
	}
	right := c.Dim(ansi.PadRight(rightLabel, rightLabelW)) + value
	return left + strings.Repeat(" ", rightPad) + right
}
