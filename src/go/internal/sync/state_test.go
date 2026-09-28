package sync

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sahil87/tu/internal/config"
)

// syncNow is a fixed instant so every assertion is deterministic.
var syncNow = time.Date(2026, 9, 15, 19, 9, 44, 502*int(time.Millisecond), time.UTC)

// R4: the commit message carries the local date — the same basis the
// day-file labels use.
func TestCommitMessage(t *testing.T) {
	now := time.Date(2026, 9, 15, 18, 54, 44, 0, time.UTC)
	want := "# harness-user: update " + now.Local().Format("2006-01-02")
	if got := CommitMessage("harness-user", now); got != want {
		t.Errorf("CommitMessage = %q, want %q", got, want)
	}
}

// R4/DC-21: when local and UTC dates straddle midnight, the message uses the
// LOCAL date — an evening sync no longer titles the commit with the next UTC
// day.
func TestCommitMessageLocalDate(t *testing.T) {
	// Process zone UTC−7: 2026-09-17 03:00 UTC is 2026-09-16 20:00 local.
	zone := time.FixedZone("minus7", -7*60*60)
	old := time.Local
	time.Local = zone
	t.Cleanup(func() { time.Local = old })
	now := time.Date(2026, 9, 17, 3, 0, 0, 0, time.UTC)
	if got := CommitMessage("harness-user", now); got != "# harness-user: update 2026-09-16" {
		t.Errorf("CommitMessage = %q, want the local date 2026-09-16", got)
	}
}

// R3: TouchLastSync creates a missing state dir and writes the JS
// toISOString() shape + "\n", round-tripping through config.LastSync.
func TestTouchLastSyncRoundTrip(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "state") // does not exist yet
	now := time.Date(2026, 9, 15, 18, 54, 44, 502*int(time.Millisecond), time.UTC)
	if err := TouchLastSync(stateDir, now); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(stateDir, LastSyncFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "2026-09-15T18:54:44.502Z\n" {
		t.Errorf(".last-sync = %q, want %q", string(raw), "2026-09-15T18:54:44.502Z\n")
	}
	if got := config.LastSync(stateDir, now.Add(15*time.Minute)); got != "15m ago (2026-09-15T18:54:44.502Z)" {
		t.Errorf("config.LastSync = %q", got)
	}
}

// R3: the four isStale cases from sync.test.ts § isStale, plus the exact
// three-hour boundary.
func TestStale(t *testing.T) {
	write := func(t *testing.T, content string) string {
		t.Helper()
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, LastSyncFile), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return dir
	}

	t.Run("missing → true", func(t *testing.T) {
		if !Stale(t.TempDir(), syncNow) {
			t.Error("Stale = false, want true (no .last-sync)")
		}
	})
	t.Run("within 3 hours → false", func(t *testing.T) {
		// 2 h old, as in the TS test.
		ts := syncNow.Add(-2 * time.Hour).UTC().Format("2006-01-02T15:04:05.000Z")
		if Stale(write(t, ts+"\n"), syncNow) {
			t.Error("Stale = true, want false")
		}
	})
	t.Run("older than 3 hours → true", func(t *testing.T) {
		// 4 h old, as in the TS test.
		ts := syncNow.Add(-4 * time.Hour).UTC().Format("2006-01-02T15:04:05.000Z")
		if !Stale(write(t, ts+"\n"), syncNow) {
			t.Error("Stale = false, want true")
		}
	})
	t.Run("unparseable → true", func(t *testing.T) {
		if !Stale(write(t, "not-a-date\n"), syncNow) {
			t.Error("Stale = false, want true")
		}
	})
	t.Run("boundary: exactly 3 h is not stale, one second past is", func(t *testing.T) {
		// R3's GIVEN: 2026-09-15T18:54:44.502Z checked at 21:54:44Z (2 h 59 m
		// 59.498 s — fresh) and again past the three-hour mark.
		dir := write(t, "2026-09-15T18:54:44.502Z\n")
		if Stale(dir, time.Date(2026, 9, 15, 21, 54, 44, 0, time.UTC)) {
			t.Error("Stale = true at <3h, want false")
		}
		if !Stale(dir, time.Date(2026, 9, 15, 21, 54, 45, 502*int(time.Millisecond), time.UTC)) {
			t.Error("Stale = false at >3h, want true")
		}
	})
}
