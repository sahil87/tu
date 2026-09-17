package sync

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CommitMessage is the TS commitMessage: "# {user}: update {UTC date}" — the
// ONE place the live commit and the dry-run preview derive it from (DC-21:
// UTC, which can trail the day-files' local date).
func CommitMessage(user string, now time.Time) string {
	return "# " + user + ": update " + now.UTC().Format("2006-01-02")
}

// LastSyncFile is the runtime-state marker touchLastSync writes and isStale
// and config.LastSync read.
const LastSyncFile = ".last-sync"

// StaleAfter is the TS THREE_HOURS_MS: a .last-sync older than this is stale.
// config keeps its own unexported cloneRetryWindow for the clone cooldown —
// two named constants for two documented rules.
const StaleAfter = 3 * time.Hour

// lastSyncFormat is the JS toISOString() shape for a UTC time ("000" is the
// reference-time millisecond layout) — the format config.LastSync parses.
const lastSyncFormat = "2006-01-02T15:04:05.000Z"

// TouchLastSync is the TS touchLastSync: writes {stateDir}/.last-sync as JS
// toISOString() + "\n". The state dir is created when missing (Design
// Decision: the TS would crash with an uncaught error there, a path no
// harness case reaches; Constitution II prefers degrading to crashing).
func TouchLastSync(stateDir string, now time.Time) error {
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	content := now.UTC().Format(lastSyncFormat) + "\n"
	return os.WriteFile(filepath.Join(stateDir, LastSyncFile), []byte(content), 0o644)
}

// Stale is the TS isStale: true when .last-sync is missing, unreadable, does
// not parse as RFC 3339 after trimming, or is older than StaleAfter. It has
// no caller in the TS (DC-20) and none here; it is ported because the plan
// row names the auto-sync TTL and gate G0 may decide to give it one.
func Stale(stateDir string, now time.Time) bool {
	raw, err := os.ReadFile(filepath.Join(stateDir, LastSyncFile))
	if err != nil {
		return true
	}
	ts, err := time.Parse(time.RFC3339, strings.TrimSpace(string(raw)))
	if err != nil {
		return true
	}
	return now.Sub(ts) > StaleAfter
}
