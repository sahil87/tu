package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readFile is a test helper: read a file or fail.
func readFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// assertLines compares the returned stdout lines.
func assertLines(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("lines = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("lines[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// R5: an empty $HOME (harness single) scaffolds from the shipped defaults.
func TestInitConfCreateFromDefaults(t *testing.T) {
	home := t.TempDir()
	p, _ := ResolvePaths(home)
	lines, err := InitConf(p)
	if err != nil {
		t.Fatal(err)
	}
	assertLines(t, lines, "Created ~/.config/tu/tu.conf — edit it to configure multi-machine sync.")
	if got := readFile(t, p.UserConf); !bytes.Equal([]byte(got), DefaultConf) {
		t.Errorf("tu.conf bytes differ from DefaultConf:\n%q", got)
	}
	info, err := os.Stat(p.UserConf)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Errorf("mode = %o, want 644", info.Mode().Perm())
	}
}

// R5: a legacy ~/.tu.conf seeds the new file (write-time copy).
func TestInitConfCopyFromLegacy(t *testing.T) {
	home := t.TempDir()
	writeConf(t, home, ".tu.conf", "version = 2\nmachine = my-box\n")
	p, _ := ResolvePaths(home)
	lines, err := InitConf(p)
	if err != nil {
		t.Fatal(err)
	}
	assertLines(t, lines, "Copied ~/.tu.conf → ~/.config/tu/tu.conf")
	if got := readFile(t, p.UserConf); got != "version = 2\nmachine = my-box\n" {
		t.Errorf("tu.conf = %q", got)
	}
	// The legacy file is never moved or deleted.
	if got := readFile(t, p.LegacyConf); got != "version = 2\nmachine = my-box\n" {
		t.Errorf("legacy conf modified: %q", got)
	}
}

// R5: the harness multi conf (all six keys active) is already complete.
func TestInitConfAlreadyComplete(t *testing.T) {
	home := t.TempDir()
	content := "version = 2\nmetrics_repo = git@example.invalid:harness/tu-metrics.git\nmetrics_dir = ~/.tu/metrics_repo\nmachine = harness-machine\nuser = harness-user\nauto_sync = true\n"
	writeConf(t, home, ".config/tu/tu.conf", content)
	p, _ := ResolvePaths(home)
	lines, err := InitConf(p)
	if err != nil {
		t.Fatal(err)
	}
	assertLines(t, lines, "~/.config/tu/tu.conf is already complete.")
	if got := readFile(t, p.UserConf); got != content {
		t.Errorf("tu.conf modified: %q", got)
	}
}

// R5: a conf with only `version = 2` gets the five other blocks appended in
// FieldBlocks order.
func TestInitConfMissingFields(t *testing.T) {
	home := t.TempDir()
	writeConf(t, home, ".config/tu/tu.conf", "version = 2\n")
	p, _ := ResolvePaths(home)
	lines, err := InitConf(p)
	if err != nil {
		t.Fatal(err)
	}
	assertLines(t, lines, "Updated ~/.config/tu/tu.conf — added missing fields: metrics_repo, metrics_dir, machine, user, auto_sync.")
	var want strings.Builder
	want.WriteString("version = 2\n")
	for _, fb := range FieldBlocks {
		if fb.Key != "version" {
			want.WriteString(fb.Block)
		}
	}
	if got := readFile(t, p.UserConf); got != want.String() {
		t.Errorf("tu.conf = %q, want %q", got, want.String())
	}
}

// R5: commented-out fields are reported, not appended.
func TestInitConfCommentedFields(t *testing.T) {
	home := t.TempDir()
	content := "version = 2\n# metrics_repo = git@x:y.git\nmetrics_dir = ~/.tu/metrics_repo\nmachine = m\nuser = u\nauto_sync = true\n"
	writeConf(t, home, ".config/tu/tu.conf", content)
	p, _ := ResolvePaths(home)
	lines, err := InitConf(p)
	if err != nil {
		t.Fatal(err)
	}
	assertLines(t, lines, "~/.config/tu/tu.conf has commented-out fields that need uncommenting: metrics_repo.")
	if got := readFile(t, p.UserConf); got != content {
		t.Errorf("tu.conf modified: %q", got)
	}
}

// R5: missing and commented keys produce both lines (Updated first).
func TestInitConfMissingAndCommented(t *testing.T) {
	home := t.TempDir()
	content := "version = 2\n# machine = m\nuser = u\nauto_sync = true\n"
	writeConf(t, home, ".config/tu/tu.conf", content)
	p, _ := ResolvePaths(home)
	lines, err := InitConf(p)
	if err != nil {
		t.Fatal(err)
	}
	assertLines(t, lines,
		"Updated ~/.config/tu/tu.conf — added missing fields: metrics_repo, metrics_dir.",
		"~/.config/tu/tu.conf has commented-out fields that need uncommenting: machine.")
	want := content + fieldBlock("metrics_repo") + fieldBlock("metrics_dir")
	if got := readFile(t, p.UserConf); got != want {
		t.Errorf("tu.conf = %q, want %q", got, want)
	}
}

// R5: a whitespace-indented active assignment counts as present (the TS
// trims the line before testing).
func TestInitConfIndentedActive(t *testing.T) {
	home := t.TempDir()
	writeConf(t, home, ".config/tu/tu.conf", "  version = 2\n\tmetrics_repo = x\nmetrics_dir = d\nmachine = m\nuser = u\nauto_sync = true\n")
	p, _ := ResolvePaths(home)
	lines, err := InitConf(p)
	if err != nil {
		t.Fatal(err)
	}
	assertLines(t, lines, "~/.config/tu/tu.conf is already complete.")
}

// fakeGit answers config.Git for tests and records its calls.
type fakeGit struct {
	isRepo bool
	calls  []string
}

func (f *fakeGit) IsRepo(dir string) bool {
	f.calls = append(f.calls, dir)
	return f.isRepo
}

func assertError(t *testing.T, err error, want string) {
	t.Helper()
	var cerr *Error
	if err == nil || !errors.As(err, &cerr) {
		t.Fatalf("err = %v, want *Error %q", err, want)
	}
	if cerr.Message != want {
		t.Errorf("error message = %q, want %q", cerr.Message, want)
	}
}

// R6/A-020: a URL containing a newline or CR is rejected before any file is
// written.
func TestInitMetricsNewlineURL(t *testing.T) {
	for _, url := range []string{"x\nversion = 9", "x\ry"} {
		home := t.TempDir()
		p, _ := ResolvePaths(home)
		git := &fakeGit{}
		_, err := InitMetrics(p, testEnv(nil), &url, git)
		assertError(t, err, "Error: repo-url must be a single line (no newline or carriage-return characters).")
		if _, statErr := os.Stat(p.ConfigDir); statErr == nil {
			t.Errorf("url %q: config dir was created", url)
		}
		if len(git.calls) != 0 {
			t.Errorf("url %q: git called %v", url, git.calls)
		}
	}
}

// R6: the three setMetricsRepoInConf branches, with the resulting file bytes
// asserted.
func TestInitMetricsSetRepoBranches(t *testing.T) {
	url := "git@example.invalid:harness/tu-metrics.git"
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{
			"active line replaced in place",
			"version = 2\nmetrics_repo = old\nmachine = m\n",
			"version = 2\nmetrics_repo = " + url + "\nmachine = m\n",
		},
		{
			"commented sample line replaced",
			"version = 2\n# metrics_repo = git@github.com:you/tu-metrics.git\nmachine = m\n",
			"version = 2\nmetrics_repo = " + url + "\nmachine = m\n",
		},
		{
			"block appended",
			"version = 2\nmachine = m\n",
			"version = 2\nmachine = m\n" + strings.Replace(fieldBlock("metrics_repo"),
				"# metrics_repo = git@github.com:you/tu-metrics.git", "metrics_repo = "+url, 1),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			home := t.TempDir()
			writeConf(t, home, ".config/tu/tu.conf", c.content)
			p, _ := ResolvePaths(home)
			git := &fakeGit{}
			res, err := InitMetrics(p, testEnv(nil), &url, git)
			if err != nil {
				t.Fatal(err)
			}
			assertLines(t, res.Lines, "Set metrics_repo = "+url+" in ~/.config/tu/tu.conf")
			if got := readFile(t, p.UserConf); got != c.want {
				t.Errorf("tu.conf = %q, want %q", got, c.want)
			}
			if res.Clone == nil || res.Clone.URL != url {
				t.Errorf("Clone = %+v, want URL %q", res.Clone, url)
			}
		})
	}
}

// R6: no metrics_repo anywhere and no url → the unset error.
func TestInitMetricsUnsetRepo(t *testing.T) {
	home := t.TempDir()
	p, _ := ResolvePaths(home)
	git := &fakeGit{}
	_, err := InitMetrics(p, testEnv(nil), nil, git)
	assertError(t, err, "Error: metrics_repo is not set. Add it to ~/.config/tu/tu.conf, run 'tu init-metrics <repo-url>', or set TU_METRICS_REPO.")
}

// R6: the metrics dir exists but is not a git repo.
func TestInitMetricsDirNotARepo(t *testing.T) {
	home := t.TempDir()
	writeConf(t, home, ".config/tu/tu.conf", "version = 2\nmetrics_repo = git@example.invalid:harness/tu-metrics.git\n")
	dir := filepath.Join(home, ".tu", "metrics_repo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p, _ := ResolvePaths(home)
	git := &fakeGit{isRepo: false}
	_, err := InitMetrics(p, testEnv(nil), nil, git)
	assertError(t, err, "Error: "+dir+" exists but is not a git repo. Remove it or set a different metrics_dir in ~/.config/tu/tu.conf.")
	if len(git.calls) != 1 || git.calls[0] != dir {
		t.Errorf("git calls = %v, want [%q]", git.calls, dir)
	}
}

// R6: the metrics dir exists and is a git repo → Already initialized.
func TestInitMetricsAlreadyInitialized(t *testing.T) {
	home := t.TempDir()
	writeConf(t, home, ".config/tu/tu.conf", "version = 2\nmetrics_repo = git@example.invalid:harness/tu-metrics.git\n")
	dir := filepath.Join(home, ".tu", "metrics_repo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p, _ := ResolvePaths(home)
	git := &fakeGit{isRepo: true}
	res, err := InitMetrics(p, testEnv(nil), nil, git)
	if err != nil {
		t.Fatal(err)
	}
	assertLines(t, res.Lines, "Already initialized: "+dir)
	if res.Clone != nil {
		t.Errorf("Clone = %+v, want nil", res.Clone)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("warnings = %v, want none", res.Warnings)
	}
}

// R6: the harness legacy variant — Already initialized with the deprecation
// warning, and no file is written.
func TestInitMetricsLegacyAlreadyInitialized(t *testing.T) {
	home := t.TempDir()
	writeConf(t, home, ".tu.conf", "version = 2\nmetrics_repo = git@example.invalid:harness/tu-metrics.git\n")
	dir := filepath.Join(home, ".tu", "metrics_repo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p, _ := ResolvePaths(home)
	git := &fakeGit{isRepo: true}
	res, err := InitMetrics(p, testEnv(nil), nil, git)
	if err != nil {
		t.Fatal(err)
	}
	assertLines(t, res.Lines, "Already initialized: "+dir)
	if res.Clone != nil {
		t.Errorf("Clone = %+v, want nil", res.Clone)
	}
	if len(res.Warnings) != 1 || res.Warnings[0] != legacyWarning {
		t.Errorf("warnings = %v, want the deprecation line", res.Warnings)
	}
	if _, statErr := os.Stat(p.ConfigDir); statErr == nil {
		t.Errorf("config dir was created")
	}
}

// R6: the harness init-metrics-url single scenario — Created + Set lines and
// a clone step; git is never consulted (the dir does not exist).
func TestInitMetricsCloneNeeded(t *testing.T) {
	home := t.TempDir()
	p, _ := ResolvePaths(home)
	url := "git@example.invalid:harness/tu-metrics.git"
	git := &fakeGit{}
	res, err := InitMetrics(p, testEnv(nil), &url, git)
	if err != nil {
		t.Fatal(err)
	}
	assertLines(t, res.Lines,
		"Created ~/.config/tu/tu.conf — edit it to configure multi-machine sync.",
		"Set metrics_repo = "+url+" in ~/.config/tu/tu.conf")
	dir := filepath.Join(home, ".tu", "metrics_repo")
	if res.Clone == nil || res.Clone.URL != url || res.Clone.Dir != dir {
		t.Errorf("Clone = %+v, want {%q %q}", res.Clone, url, dir)
	}
	if len(git.calls) != 0 {
		t.Errorf("git called %v, want never (dir absent)", git.calls)
	}
	want := strings.Replace(string(DefaultConf),
		"# metrics_repo = git@github.com:you/tu-metrics.git", "metrics_repo = "+url, 1)
	if got := readFile(t, p.UserConf); got != want {
		t.Errorf("tu.conf = %q, want %q", got, want)
	}
}

// R6/A-017: the init-metrics-url legacy scenario — the Copied and Set lines,
// and NO deprecation warning (the new file exists when Load runs).
func TestInitMetricsURLLegacy(t *testing.T) {
	home := t.TempDir()
	writeConf(t, home, ".tu.conf", "version = 2\nmetrics_repo = old\nmachine = my-box\n")
	p, _ := ResolvePaths(home)
	url := "git@example.invalid:harness/tu-metrics.git"
	git := &fakeGit{}
	res, err := InitMetrics(p, testEnv(nil), &url, git)
	if err != nil {
		t.Fatal(err)
	}
	assertLines(t, res.Lines,
		"Copied ~/.tu.conf → ~/.config/tu/tu.conf",
		"Set metrics_repo = "+url+" in ~/.config/tu/tu.conf")
	if len(res.Warnings) != 0 {
		t.Errorf("warnings = %v, want none (the new file exists before Load)", res.Warnings)
	}
	// The copied file then has its active metrics_repo line replaced.
	want := "version = 2\nmetrics_repo = " + url + "\nmachine = my-box\n"
	if got := readFile(t, p.UserConf); got != want {
		t.Errorf("tu.conf = %q, want %q", got, want)
	}
	if got := readFile(t, p.LegacyConf); got != "version = 2\nmetrics_repo = old\nmachine = my-box\n" {
		t.Errorf("legacy conf modified: %q", got)
	}
}

// R6: a CLI url beats an exported TU_METRICS_REPO for this invocation's clone.
func TestInitMetricsOverrideBeatsEnv(t *testing.T) {
	home := t.TempDir()
	p, _ := ResolvePaths(home)
	url := "git@example.invalid:harness/tu-metrics.git"
	git := &fakeGit{}
	env := testEnv(map[string]string{"TU_METRICS_REPO": "git@example.invalid:other/env.git"})
	res, err := InitMetrics(p, env, &url, git)
	if err != nil {
		t.Fatal(err)
	}
	if res.Clone == nil || res.Clone.URL != url {
		t.Errorf("Clone = %+v, want URL %q (override beats env)", res.Clone, url)
	}
}

// R6: RemoveCloneMarker removes the marker when present and never fails.
func TestRemoveCloneMarker(t *testing.T) {
	dir := t.TempDir()
	RemoveCloneMarker(dir) // absent: no error
	marker := filepath.Join(dir, CloneFailedMarker)
	if err := os.WriteFile(marker, []byte("2026-09-16T00:00:00.000Z\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	RemoveCloneMarker(dir)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Errorf("marker still present: %v", err)
	}
}

func TestClonedLine(t *testing.T) {
	got := ClonedLine("git@example.invalid:harness/tu-metrics.git", "/h/.tu/metrics_repo")
	want := "Cloned git@example.invalid:harness/tu-metrics.git → /h/.tu/metrics_repo"
	if got != want {
		t.Errorf("ClonedLine = %q, want %q", got, want)
	}
}
