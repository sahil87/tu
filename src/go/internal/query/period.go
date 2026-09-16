// Package query is the pure filter/roll-up/group-by stage of the Go port's
// pipeline: it consumes []fact.Record and produces filtered, re-labeled, or
// grouped records. Nothing here imports I/O or exec packages.
package query

import "time"

// Period is the roll-up granularity.
type Period int

const (
	Daily Period = iota
	Weekly
	Monthly
)

// String returns the heading text: "daily", "weekly", or "monthly".
func (p Period) String() string {
	switch p {
	case Weekly:
		return "weekly"
	case Monthly:
		return "monthly"
	default:
		return "daily"
	}
}

// CurrentLabel is the period's label for now in now's location (local time,
// mirroring the TS currentLabel): daily "2006-01-02"; weekly the ISO date of
// the current week's Sunday (AddDate normalizes month/year underflow, e.g. a
// Sunday in the previous month); monthly "2006-01". The edge passes time.Now();
// time.Local honors $TZ.
func CurrentLabel(p Period, now time.Time) string {
	switch p {
	case Monthly:
		return now.Format("2006-01")
	case Weekly:
		sunday := now.AddDate(0, 0, -int(now.Weekday()))
		return sunday.Format("2006-01-02")
	default:
		return now.Format("2006-01-02")
	}
}

// ThreeMonthFloor is the implicit history cap's floor: the first day of the
// local month two calendar months back, so the window spans three calendar
// months including the current one (2026-09-16 → "2026-07-01"; 2026-01-15 →
// "2025-11-01" — time.Date normalizes the month underflow). Mirrors the TS
// threeMonthFloor; the caller passes deps.Now() so the harness tz axis holds.
func ThreeMonthFloor(now time.Time) string {
	floor := time.Date(now.Year(), now.Month()-2, 1, 0, 0, 0, 0, now.Location())
	return floor.Format("2006-01-02")
}

// WeekLabel maps a daily ISO label to its week's Sunday using UTC date
// arithmetic on the date-only string (DST-immune, mirroring the TS weekLabel).
// A label that does not parse as 2006-01-02 is returned unchanged — it becomes
// its own bucket instead of crashing aggregation (graceful degradation).
func WeekLabel(daily string) string {
	d, err := time.Parse("2006-01-02", daily)
	if err != nil {
		return daily
	}
	sunday := d.AddDate(0, 0, -int(d.Weekday()))
	return sunday.Format("2006-01-02")
}
