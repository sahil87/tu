package main

import (
	"os"
	"testing"
	"time"
)

// now() is the edge clock seam: TUDIFF_NOW pins it (a zone-less local
// timestamp in the process TZ) for the differential harness's golden runs;
// unset or malformed falls back to time.Now with no error line.

func TestNowPinned(t *testing.T) {
	t.Setenv(nowEnvVar, "2026-09-26T12:00:00")
	if got := now().Format("2006-01-02T15:04:05"); got != "2026-09-26T12:00:00" {
		t.Errorf("now() = %s, want 2026-09-26T12:00:00", got)
	}
}

func TestNowUnsetFallsBack(t *testing.T) {
	os.Unsetenv(nowEnvVar)
	before := time.Now()
	got := now()
	after := time.Now()
	if got.Before(before) || got.After(after) {
		t.Errorf("now() = %v, want within [%v, %v]", got, before, after)
	}
}

func TestNowMalformedFallsBack(t *testing.T) {
	t.Setenv(nowEnvVar, "banana")
	before := time.Now()
	got := now()
	after := time.Now()
	if got.Before(before) || got.After(after) {
		t.Errorf("now() = %v, want within [%v, %v]", got, before, after)
	}
}

// The pinned timestamp is zone-less on purpose: the matrix's tz axis runs
// cases under TZ=UTC and TZ=Asia/Kolkata and both must see the same local
// calendar date (the date the goldens were captured on).
func TestNowSameDateAcrossZones(t *testing.T) {
	t.Setenv(nowEnvVar, "2026-09-26T12:00:00")
	orig := time.Local
	t.Cleanup(func() { time.Local = orig })
	for _, zone := range []string{"UTC", "Asia/Kolkata"} {
		loc, err := time.LoadLocation(zone)
		if err != nil {
			t.Fatalf("LoadLocation(%q): %v", zone, err)
		}
		time.Local = loc
		if got := now().Format("2006-01-02"); got != "2026-09-26" {
			t.Errorf("TZ=%s: now() local date = %s, want 2026-09-26", zone, got)
		}
	}
}
