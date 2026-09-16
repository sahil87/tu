package query

import (
	"testing"
	"time"
)

func TestPeriodString(t *testing.T) {
	cases := []struct {
		p    Period
		want string
	}{
		{Daily, "daily"},
		{Weekly, "weekly"},
		{Monthly, "monthly"},
	}
	for _, c := range cases {
		if got := c.p.String(); got != c.want {
			t.Errorf("Period(%d).String() = %q, want %q", int(c.p), got, c.want)
		}
	}
}

func mustLoadLocation(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("LoadLocation(%q): %v", name, err)
	}
	return loc
}

func TestCurrentLabel(t *testing.T) {
	kolkata := mustLoadLocation(t, "Asia/Kolkata")
	cases := []struct {
		name string
		now  time.Time
		want map[Period]string
	}{
		{
			name: "kolkata wednesday",
			now:  time.Date(2026, 9, 16, 12, 0, 0, 0, kolkata),
			want: map[Period]string{Daily: "2026-09-16", Weekly: "2026-09-13", Monthly: "2026-09"},
		},
		{
			name: "utc wednesday",
			now:  time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC),
			want: map[Period]string{Daily: "2026-09-16", Weekly: "2026-09-13", Monthly: "2026-09"},
		},
		{
			name: "kolkata sunday",
			now:  time.Date(2026, 3, 1, 12, 0, 0, 0, kolkata), // a Sunday
			want: map[Period]string{Daily: "2026-03-01", Weekly: "2026-03-01", Monthly: "2026-03"},
		},
		{
			name: "utc month-start wednesday",
			now:  time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC), // week Sunday is in August
			want: map[Period]string{Daily: "2026-09-02", Weekly: "2026-08-30", Monthly: "2026-09"},
		},
		{
			name: "utc jan 1 thursday",
			now:  time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC), // week Sunday is in 2025
			want: map[Period]string{Daily: "2026-01-01", Weekly: "2025-12-28", Monthly: "2026-01"},
		},
	}
	for _, c := range cases {
		for _, p := range []Period{Daily, Weekly, Monthly} {
			if got := CurrentLabel(p, c.now); got != c.want[p] {
				t.Errorf("%s: CurrentLabel(%v) = %q, want %q", c.name, p, got, c.want[p])
			}
		}
	}
}

func TestWeekLabel(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"2026-03-01", "2026-03-01"}, // a Sunday maps to itself
		{"2026-01-01", "2025-12-28"}, // a Thursday; week Sunday is in the previous year
		{"2026-09-16", "2026-09-13"}, // Wednesday
		{"2026-09-02", "2026-08-30"}, // week Sunday in the previous month
		{"garbage", "garbage"},       // malformed passes through unchanged
		{"2026-09", "2026-09"},       // a monthly label is not a daily one
		{"", ""},
	}
	for _, c := range cases {
		if got := WeekLabel(c.in); got != c.want {
			t.Errorf("WeekLabel(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
