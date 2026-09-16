package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The auto-clone guard's stderr lines and cooldown constants (the TS
// checkMetricsDirGuard; byte-exact).
const (
	guardWarnUnavailable = "Warning: metrics repo not available — falling back to single mode."
	cloneRetryWindow     = 3 * time.Hour // the TS CLONE_RETRY_MS (THREE_HOURS_MS)
	cloneMarkerFormat    = "2006-01-02T15:04:05.000Z"
)

// Cloner is the one git question the guard asks. sync.Exec satisfies it.
type Cloner interface {
	// CloneQuiet runs `git clone <url> <dir>` with stdout/stderr captured,
	// GIT_TERMINAL_PROMPT=0 in the child env, and a 30 s deadline; returns
	// the child's stderr and the exec error (nil on exit 0).
	CloneQuiet(ctx context.Context, url, dir string) (stderr string, err error)
}

// MetricsDirGuard is the TS checkMetricsDirGuard: returns the config the data
// path runs with (Mode possibly demoted to Single) and the stderr lines the
// edge prints, in order. Pure apart from the marker file and the clone it
// delegates to git — it prints nothing itself. The TS branch for an empty
// metricsRepo is unreachable (Multi implies a non-empty repo) and is not
// ported.
func MetricsDirGuard(cfg Config, stateDir string, now time.Time, git Cloner) (Config, []string) {
	// An existence check, not an is-repo check: the harness seed has no .git/.
	if cfg.Mode != Multi || fileExists(cfg.MetricsDir) {
		return cfg, nil
	}
	if cloneMarkerFresh(stateDir, now) {
		cfg.Mode = Single
		return cfg, []string{guardWarnUnavailable}
	}
	stderr, err := git.CloneQuiet(context.Background(), cfg.MetricsRepo, cfg.MetricsDir)
	if err == nil {
		// Still Multi even if the directory still does not exist (the fake git
		// creates nothing): the readers then find nothing.
		RemoveCloneMarker(stateDir)
		return cfg, []string{"Cloned metrics repo → " + cfg.MetricsDir}
	}
	writeCloneMarker(stateDir, now)
	cfg.Mode = Single
	return cfg, []string{"Warning: could not clone metrics repo (" + cloneFailureDetail(cfg, stderr, err) + ") — falling back to single mode."}
}

// cloneMarkerFresh is the TS isCloneMarkerFresh: stateDir/.clone-failed exists
// and its trimmed content parses as an RFC 3339 timestamp younger than 3 h.
// Missing, unreadable, or unparseable counts as stale (the TS is silent here
// by contract — the repo's absence is what the guard reports).
func cloneMarkerFresh(stateDir string, now time.Time) bool {
	raw, err := os.ReadFile(filepath.Join(stateDir, CloneFailedMarker))
	if err != nil {
		return false
	}
	ts, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(string(raw)))
	if err != nil {
		return false
	}
	return now.Sub(ts) < cloneRetryWindow
}

// writeCloneMarker is the TS writeCloneMarker: stateDir is created if needed
// and the marker content is now as a JavaScript toISOString() timestamp.
func writeCloneMarker(stateDir string, now time.Time) {
	_ = os.MkdirAll(stateDir, 0o755)
	_ = os.WriteFile(filepath.Join(stateDir, CloneFailedMarker), []byte(now.UTC().Format(cloneMarkerFormat)), 0o644)
}

// cloneFailureDetail composes the ({detail}) of the clone-failure warning,
// reproducing Node's execFileSync error message: `Command failed: git clone
// {url} {dir}` followed by "\n" + the captured stderr when that is non-empty,
// or `spawnSync git ETIMEDOUT` when the error wraps context.DeadlineExceeded.
func cloneFailureDetail(cfg Config, stderr string, err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "spawnSync git ETIMEDOUT"
	}
	detail := "Command failed: git clone " + cfg.MetricsRepo + " " + cfg.MetricsDir
	if stderr != "" {
		detail += "\n" + stderr
	}
	return detail
}
