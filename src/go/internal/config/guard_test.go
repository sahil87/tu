package config

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// fakeCloner is a test Cloner recording its calls and answering with the
// configured stderr/error.
type fakeCloner struct {
	stderr string
	err    error
	calls  []cloneCall
}

type cloneCall struct{ url, dir string }

func (f *fakeCloner) CloneQuiet(_ context.Context, url, dir string) (string, error) {
	f.calls = append(f.calls, cloneCall{url, dir})
	return f.stderr, f.err
}

var guardNow = time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)

// guardCfg is a Multi config whose MetricsDir does not exist.
func guardCfg(dir string) Config {
	return Config{Mode: Multi, MetricsRepo: "git@example.invalid:harness/tu-metrics.git", MetricsDir: dir}
}

// writeMarker writes stateDir/.clone-failed with the given raw content.
func writeMarker(t *testing.T, stateDir, content string) {
	t.Helper()
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, CloneFailedMarker), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// R6: not multi mode, or a metrics dir that exists — the guard is a no-op.
func TestMetricsDirGuardNoOp(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), ".tu")
	existing := t.TempDir()
	cloner := &fakeCloner{}

	// Single mode: returned unchanged, no lines, no clone.
	cfg := Config{Mode: Single, MetricsDir: existing}
	got, lines := MetricsDirGuard(cfg, stateDir, guardNow, cloner)
	if got != cfg || lines != nil || len(cloner.calls) != 0 {
		t.Errorf("single mode: cfg = %+v, lines = %v, calls = %v", got, lines, cloner.calls)
	}

	// Multi with the dir present (an existence check, not an is-repo check).
	cfg = guardCfg(existing)
	got, lines = MetricsDirGuard(cfg, stateDir, guardNow, cloner)
	if got != cfg || lines != nil || len(cloner.calls) != 0 {
		t.Errorf("dir exists: cfg = %+v, lines = %v, calls = %v", got, lines, cloner.calls)
	}
}

// R6: no marker, clone succeeds — still Multi, the Cloned line, the marker
// (from an earlier failure) removed, one clone call with (repo, dir).
func TestMetricsDirGuardCloneSuccess(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), ".tu")
	writeMarker(t, stateDir, guardNow.Add(-4*time.Hour).UTC().Format(cloneMarkerFormat)) // stale
	dir := filepath.Join(t.TempDir(), "metrics_repo")
	cloner := &fakeCloner{}

	cfg := guardCfg(dir)
	got, lines := MetricsDirGuard(cfg, stateDir, guardNow, cloner)
	if got.Mode != Multi {
		t.Errorf("Mode = %v, want Multi (the fake created no directory)", got.Mode)
	}
	if want := []string{"Cloned metrics repo → " + dir}; !reflect.DeepEqual(lines, want) {
		t.Errorf("lines = %v, want %v", lines, want)
	}
	if len(cloner.calls) != 1 || cloner.calls[0] != (cloneCall{cfg.MetricsRepo, dir}) {
		t.Errorf("calls = %v, want one clone of (%s, %s)", cloner.calls, cfg.MetricsRepo, dir)
	}
	if _, err := os.Stat(filepath.Join(stateDir, CloneFailedMarker)); !os.IsNotExist(err) {
		t.Errorf("marker still present after a successful clone")
	}
}

// R6: a fresh marker (1 h ago) suppresses the clone — Single, the
// not-available warning, git never called.
func TestMetricsDirGuardFreshMarker(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), ".tu")
	writeMarker(t, stateDir, "  "+guardNow.Add(-time.Hour).UTC().Format(cloneMarkerFormat)+"\n")
	cloner := &fakeCloner{}

	cfg := guardCfg(filepath.Join(t.TempDir(), "metrics_repo"))
	got, lines := MetricsDirGuard(cfg, stateDir, guardNow, cloner)
	if got.Mode != Single {
		t.Errorf("Mode = %v, want Single", got.Mode)
	}
	if want := []string{guardWarnUnavailable}; !reflect.DeepEqual(lines, want) {
		t.Errorf("lines = %v, want %v", lines, want)
	}
	if len(cloner.calls) != 0 {
		t.Errorf("calls = %v, want none", cloner.calls)
	}
}

// R6/A-022: a stale (4 h) or garbage marker counts as stale — the clone is
// attempted. The state dir is created when the marker write lands there.
func TestMetricsDirGuardStaleMarkers(t *testing.T) {
	cases := []struct {
		name   string
		marker string // "" = no marker file
	}{
		{"no marker", ""},
		{"4h old", guardNow.Add(-4 * time.Hour).UTC().Format(cloneMarkerFormat)},
		{"garbage", "not a timestamp"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stateDir := filepath.Join(t.TempDir(), ".tu")
			if c.marker != "" {
				writeMarker(t, stateDir, c.marker)
			}
			cloner := &fakeCloner{}
			cfg := guardCfg(filepath.Join(t.TempDir(), "metrics_repo"))
			if _, lines := MetricsDirGuard(cfg, stateDir, guardNow, cloner); len(cloner.calls) != 1 {
				t.Errorf("calls = %v, want one clone attempt (lines %v)", cloner.calls, lines)
			}
		})
	}
}

// R6/A-023: clone failure with stderr — the marker is written with the pinned
// timestamp format into a created state dir, the config demotes to Single, and
// the line carries the composed detail.
func TestMetricsDirGuardCloneFailure(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), ".tu") // does not exist yet
	dir := filepath.Join(t.TempDir(), "metrics_repo")
	cloner := &fakeCloner{stderr: "fatal: repository not found", err: errors.New("exit status 128")}

	cfg := guardCfg(dir)
	got, lines := MetricsDirGuard(cfg, stateDir, guardNow, cloner)
	if got.Mode != Single {
		t.Errorf("Mode = %v, want Single", got.Mode)
	}
	want := []string{"Warning: could not clone metrics repo (Command failed: git clone " +
		cfg.MetricsRepo + " " + dir + "\nfatal: repository not found) — falling back to single mode."}
	if !reflect.DeepEqual(lines, want) {
		t.Errorf("lines = %q, want %q", lines, want)
	}
	raw, err := os.ReadFile(filepath.Join(stateDir, CloneFailedMarker))
	if err != nil {
		t.Fatalf("marker not written: %v", err)
	}
	if string(raw) != "2026-09-17T12:00:00.000Z" {
		t.Errorf("marker = %q, want the pinned toISOString format", string(raw))
	}
}

// R6/A-023: a deadline maps to `spawnSync git ETIMEDOUT`; an empty stderr
// leaves no trailing newline in the Command failed detail.
func TestMetricsDirGuardFailureDetails(t *testing.T) {
	t.Run("timeout", func(t *testing.T) {
		cloner := &fakeCloner{err: context.DeadlineExceeded}
		cfg := guardCfg(filepath.Join(t.TempDir(), "metrics_repo"))
		_, lines := MetricsDirGuard(cfg, filepath.Join(t.TempDir(), ".tu"), guardNow, cloner)
		want := "Warning: could not clone metrics repo (spawnSync git ETIMEDOUT) — falling back to single mode."
		if len(lines) != 1 || lines[0] != want {
			t.Errorf("lines = %q, want %q", lines, want)
		}
	})
	t.Run("empty stderr", func(t *testing.T) {
		cloner := &fakeCloner{err: errors.New("exit status 1")}
		cfg := guardCfg(filepath.Join(t.TempDir(), "metrics_repo"))
		_, lines := MetricsDirGuard(cfg, filepath.Join(t.TempDir(), ".tu"), guardNow, cloner)
		want := "Warning: could not clone metrics repo (Command failed: git clone " +
			cfg.MetricsRepo + " " + cfg.MetricsDir + ") — falling back to single mode."
		if len(lines) != 1 || lines[0] != want {
			t.Errorf("lines = %q, want %q", lines, want)
		}
	})
	// A process-start error never ran git: the detail is the spawnSync errno,
	// not a fabricated Command failed line.
	t.Run("git not found", func(t *testing.T) {
		cloner := &fakeCloner{err: &exec.Error{Name: "git", Err: exec.ErrNotFound}}
		cfg := guardCfg(filepath.Join(t.TempDir(), "metrics_repo"))
		_, lines := MetricsDirGuard(cfg, filepath.Join(t.TempDir(), ".tu"), guardNow, cloner)
		want := "Warning: could not clone metrics repo (spawnSync git ENOENT) — falling back to single mode."
		if len(lines) != 1 || lines[0] != want {
			t.Errorf("lines = %q, want %q", lines, want)
		}
	})
	t.Run("git not executable", func(t *testing.T) {
		cloner := &fakeCloner{err: &os.PathError{Op: "fork/exec", Path: "git", Err: fs.ErrPermission}}
		cfg := guardCfg(filepath.Join(t.TempDir(), "metrics_repo"))
		_, lines := MetricsDirGuard(cfg, filepath.Join(t.TempDir(), ".tu"), guardNow, cloner)
		want := "Warning: could not clone metrics repo (spawnSync git EACCES) — falling back to single mode."
		if len(lines) != 1 || lines[0] != want {
			t.Errorf("lines = %q, want %q", lines, want)
		}
	})
}
