package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// harnessConf is the staged home body the differential harness uses
// (harness.HomeConfBody) — machine/user pinned so the sentinels never fire.
const harnessConf = `version = 2
metrics_repo = git@example.invalid:harness/tu-metrics.git
metrics_dir = ~/.tu/metrics_repo
machine = harness-machine
user = harness-user
auto_sync = true
`

var statusNow = time.Date(2026, 9, 15, 19, 9, 44, 502000000, time.UTC)

func assertWarnings(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("warnings = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("warnings[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// R7: no config file at all → the one-line form, no Load, no warnings.
func TestStatusNoConfig(t *testing.T) {
	home := t.TempDir()
	p, _ := ResolvePaths(home)
	data, warnings := Status(p, testEnv(nil), statusNow)
	assertWarnings(t, warnings)
	assertLines(t, data.Lines(), "Mode:        single (no ~/.config/tu/tu.conf)")
	if !data.NoConfig || data.Mode != Single {
		t.Errorf("data = %+v", data)
	}
}

// R7: single mode with a user conf.
func TestStatusSingleWithConfig(t *testing.T) {
	home := t.TempDir()
	writeConf(t, home, ".config/tu/tu.conf", "version = 2\n")
	p, _ := ResolvePaths(home)
	data, warnings := Status(p, testEnv(nil), statusNow)
	assertWarnings(t, warnings)
	assertLines(t, data.Lines(),
		"Mode:        single",
		"Config:      ~/.config/tu/tu.conf (v2)")
}

// R7: single mode with the legacy fallback — Config names ~/.tu.conf and the
// deprecation warning is returned.
func TestStatusSingleLegacy(t *testing.T) {
	home := t.TempDir()
	writeConf(t, home, ".tu.conf", "version = 2\n")
	p, _ := ResolvePaths(home)
	data, warnings := Status(p, testEnv(nil), statusNow)
	assertWarnings(t, warnings, legacyWarning)
	assertLines(t, data.Lines(),
		"Mode:        single",
		"Config:      ~/.tu.conf (v2)")
}

// R7: an org-only single-mode setup omits the Config line but prints the Org
// line.
func TestStatusSingleOrgOnly(t *testing.T) {
	home := t.TempDir()
	writeConf(t, home, ".config/tu/org.conf", "version = 2\n")
	p, _ := ResolvePaths(home)
	data, warnings := Status(p, testEnv(nil), statusNow)
	assertWarnings(t, warnings)
	assertLines(t, data.Lines(),
		"Mode:        single",
		"Org config:  ~/.config/tu/org.conf")
}

// R7: the harness org variant — org-only multi.
func TestStatusOrgMulti(t *testing.T) {
	home := t.TempDir()
	writeConf(t, home, ".config/tu/org.conf", harnessConf)
	if err := os.MkdirAll(filepath.Join(home, ".tu", "metrics_repo"), 0o755); err != nil {
		t.Fatal(err)
	}
	p, _ := ResolvePaths(home)
	data, warnings := Status(p, testEnv(nil), statusNow)
	assertWarnings(t, warnings)
	assertLines(t, data.Lines(),
		"Mode:        multi",
		"User:        harness-user",
		"Machine:     harness-machine",
		"Org config:  ~/.config/tu/org.conf",
		"Metrics:     ~/.tu/metrics_repo",
		"Last sync:   never",
		"Auto-sync:   on")
}

// R7: the harness multi variant — the full block with the Config line.
func TestStatusMultiWithConfig(t *testing.T) {
	home := t.TempDir()
	writeConf(t, home, ".config/tu/tu.conf", harnessConf)
	if err := os.MkdirAll(filepath.Join(home, ".tu", "metrics_repo"), 0o755); err != nil {
		t.Fatal(err)
	}
	p, _ := ResolvePaths(home)
	data, warnings := Status(p, testEnv(nil), statusNow)
	assertWarnings(t, warnings)
	assertLines(t, data.Lines(),
		"Mode:        multi",
		"User:        harness-user",
		"Machine:     harness-machine",
		"Config:      ~/.config/tu/tu.conf (v2)",
		"Metrics:     ~/.tu/metrics_repo",
		"Last sync:   never",
		"Auto-sync:   on")
}

// R7: the harness legacy variant — Config names the legacy file, deprecation
// on stderr.
func TestStatusLegacyMulti(t *testing.T) {
	home := t.TempDir()
	writeConf(t, home, ".tu.conf", harnessConf)
	if err := os.MkdirAll(filepath.Join(home, ".tu", "metrics_repo"), 0o755); err != nil {
		t.Fatal(err)
	}
	p, _ := ResolvePaths(home)
	data, warnings := Status(p, testEnv(nil), statusNow)
	assertWarnings(t, warnings, legacyWarning)
	assertLines(t, data.Lines(),
		"Mode:        multi",
		"User:        harness-user",
		"Machine:     harness-machine",
		"Config:      ~/.tu.conf (v2)",
		"Metrics:     ~/.tu/metrics_repo",
		"Last sync:   never",
		"Auto-sync:   on")
}

// R7: a missing metrics dir gets the NOT FOUND suffix (em dash, apostrophe).
func TestStatusMetricsNotFound(t *testing.T) {
	home := t.TempDir()
	writeConf(t, home, ".config/tu/tu.conf", harnessConf)
	p, _ := ResolvePaths(home)
	data, warnings := Status(p, testEnv(nil), statusNow)
	assertWarnings(t, warnings)
	assertLines(t, data.Lines(),
		"Mode:        multi",
		"User:        harness-user",
		"Machine:     harness-machine",
		"Config:      ~/.config/tu/tu.conf (v2)",
		"Metrics:     ~/.tu/metrics_repo (NOT FOUND — run 'tu init-metrics')",
		"Last sync:   never",
		"Auto-sync:   on")
}

// R7: auto_sync false renders "off".
func TestStatusAutoSyncOff(t *testing.T) {
	home := t.TempDir()
	writeConf(t, home, ".config/tu/tu.conf", harnessConf+"auto_sync = false\n")
	p, _ := ResolvePaths(home)
	data, _ := Status(p, testEnv(nil), statusNow)
	lines := data.Lines()
	if got := lines[len(lines)-1]; got != "Auto-sync:   off" {
		t.Errorf("last line = %q, want Auto-sync:   off", got)
	}
}

// R7/A-018: a parseable .last-sync renders "{relative} ({ISO})" from the
// fixed now.
func TestStatusLastSync(t *testing.T) {
	home := t.TempDir()
	writeConf(t, home, ".config/tu/tu.conf", harnessConf)
	writeConf(t, home, ".tu/.last-sync", "2026-09-15T18:54:44.502Z\n")
	p, _ := ResolvePaths(home)
	data, _ := Status(p, testEnv(nil), statusNow)
	if data.LastSync != "15m ago (2026-09-15T18:54:44.502Z)" {
		t.Errorf("LastSync = %q", data.LastSync)
	}
}

// R7/A-023: absent, garbage, and unreadable .last-sync render "never".
func TestStatusLastSyncNever(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content *string
	}{
		{"absent", nil},
		{"garbage", ptrString("not a timestamp")},
		{"empty", ptrString("")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			writeConf(t, home, ".config/tu/tu.conf", harnessConf)
			if tc.content != nil {
				writeConf(t, home, ".tu/.last-sync", *tc.content)
			}
			p, _ := ResolvePaths(home)
			data, _ := Status(p, testEnv(nil), statusNow)
			if data.LastSync != "never" {
				t.Errorf("LastSync = %q, want never", data.LastSync)
			}
		})
	}
}

func ptrString(s string) *string { return &s }

// R7: RelativeTime boundaries.
func TestRelativeTime(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{0, "<1m ago"},
		{-5 * time.Second, "<1m ago"}, // negatives clamp to 0
		{59 * time.Second, "<1m ago"},
		{60 * time.Second, "1m ago"},
		{59*time.Second + 999*time.Millisecond, "<1m ago"}, // floor to seconds
		{59 * time.Minute, "59m ago"},
		{60 * time.Minute, "1h ago"},
		{23 * time.Hour, "23h ago"},
		{24 * time.Hour, "1d ago"},
		{49 * time.Hour, "2d ago"},
	}
	for _, c := range cases {
		if got := RelativeTime(c.d); got != c.want {
			t.Errorf("RelativeTime(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}
