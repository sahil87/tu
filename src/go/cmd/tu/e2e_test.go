package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sahil87/tu/internal/command"
	"github.com/sahil87/tu/internal/config"
	"github.com/sahil87/tu/internal/harness"
)

// e2eHome is the temp HOME every end-to-end test runs against; the fake
// ccusage built by TestMain sits first on PATH and replays the _placeholder
// corpus only (never a local capture), so these tests are deterministic in CI
// and on dev machines. TZ=UTC and NO_COLOR unset pin the clock and color axes.
var e2eHome string

func TestMain(m *testing.M) {
	fixtures, err := filepath.Abs("../../../../harness/fixtures/_placeholder")
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve placeholder corpus: %v\n", err)
		os.Exit(1)
	}
	if _, err := os.Stat(fixtures); err != nil {
		fmt.Fprintf(os.Stderr, "placeholder corpus missing at %s: %v\n", fixtures, err)
		os.Exit(1)
	}

	tmp, err := os.MkdirTemp("", "tu-e2e")
	if err != nil {
		fmt.Fprintf(os.Stderr, "mktemp: %v\n", err)
		os.Exit(1)
	}
	fake := filepath.Join(tmp, "ccusage")
	build := exec.Command("go", "build", "-o", fake, "../../cmd/fakeccusage")
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build fakeccusage: %v\n%s", err, out)
		os.RemoveAll(tmp)
		os.Exit(1)
	}
	// The fake git also sits first on PATH: the setup commands reach git
	// through PATH, exactly as the harness stages it.
	fakeGit := filepath.Join(tmp, "git")
	build = exec.Command("go", "build", "-o", fakeGit, "../../cmd/fakegit")
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build fakegit: %v\n%s", err, out)
		os.RemoveAll(tmp)
		os.Exit(1)
	}

	e2eHome = filepath.Join(tmp, "home")
	if err := os.MkdirAll(e2eHome, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir home: %v\n", err)
		os.RemoveAll(tmp)
		os.Exit(1)
	}

	os.Setenv("TUDIFF_FIXTURES", fixtures)
	os.Setenv("HOME", e2eHome)
	os.Setenv("TZ", "UTC")
	os.Unsetenv("NO_COLOR")
	// An exported TU_METRICS_REPO flips mode detection into multi; the tests
	// must be deterministic regardless of the developer's shell env.
	os.Unsetenv("TU_METRICS_REPO")
	os.Setenv("PATH", tmp+string(os.PathListSeparator)+os.Getenv("PATH"))

	code := m.Run()
	os.RemoveAll(tmp)
	os.Exit(code)
}

// assertRun runs the CLI and checks exit code, stdout, and stderr byte for byte.
func assertRun(t *testing.T, args []string, wantCode int, wantStdout, wantStderr string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr)
	if code != wantCode {
		t.Errorf("run(%q) exit = %d, want %d", args, code, wantCode)
	}
	if got := stdout.String(); got != wantStdout {
		t.Errorf("run(%q) stdout = %q, want %q", args, got, wantStdout)
	}
	if got := stderr.String(); got != wantStderr {
		t.Errorf("run(%q) stderr = %q, want %q", args, got, wantStderr)
	}
}

// The intake §10 empty state: the placeholder corpus's dates (2026-01-05..07)
// never match today, so every snapshot is the empty state.
func TestE2EEmptySnapshot(t *testing.T) {
	for period, want := range map[string]string{
		"daily":   "\n\x1b[1;37m📊 Combined Usage (daily)\x1b[0m\n\n  No usage\n\n",
		"weekly":  "\n\x1b[1;37m📊 Combined Usage (weekly)\x1b[0m\n\n  No usage\n\n",
		"monthly": "\n\x1b[1;37m📊 Combined Usage (monthly)\x1b[0m\n\n  No usage\n\n",
	} {
		t.Run(period, func(t *testing.T) {
			var args []string
			switch period {
			case "weekly":
				args = []string{"w"}
			case "monthly":
				args = []string{"m"}
			}
			assertRun(t, args, 0, want, "")
			// A single source renders the same empty state (combined heading).
			ccArgs := append([]string{"cc"}, args...)
			assertRun(t, ccArgs, 0, want, "")
		})
	}
}

func TestE2EEmptySnapshotNoColor(t *testing.T) {
	assertRun(t, []string{"--no-color"}, 0, "\n📊 Combined Usage (daily)\n\n  No usage\n\n", "")
}

func TestE2EEmptySnapshotJSON(t *testing.T) {
	want := `{
  "Claude Code": {
    "totalCost": 0,
    "inputTokens": 0,
    "outputTokens": 0,
    "cacheCreationTokens": 0,
    "cacheReadTokens": 0,
    "totalTokens": 0
  },
  "Codex": {
    "totalCost": 0,
    "inputTokens": 0,
    "outputTokens": 0,
    "cacheCreationTokens": 0,
    "cacheReadTokens": 0,
    "totalTokens": 0
  },
  "OpenCode": {
    "totalCost": 0,
    "inputTokens": 0,
    "outputTokens": 0,
    "cacheCreationTokens": 0,
    "cacheReadTokens": 0,
    "totalTokens": 0
  },
  "Gemini": {
    "totalCost": 0,
    "inputTokens": 0,
    "outputTokens": 0,
    "cacheCreationTokens": 0,
    "cacheReadTokens": 0,
    "totalTokens": 0
  },
  "Copilot": {
    "totalCost": 0,
    "inputTokens": 0,
    "outputTokens": 0,
    "cacheCreationTokens": 0,
    "cacheReadTokens": 0,
    "totalTokens": 0
  },
  "Kimi": {
    "totalCost": 0,
    "inputTokens": 0,
    "outputTokens": 0,
    "cacheCreationTokens": 0,
    "cacheReadTokens": 0,
    "totalTokens": 0
  }
}
`
	assertRun(t, []string{"--json"}, 0, want, "")
}

func TestE2EUsageError(t *testing.T) {
	assertRun(t, []string{"bogus"}, 2, "", "Unknown argument: bogus\n"+command.ShortUsage+"\n")
}

// ── B1: config cascade and setup commands ──────────────────────────────────

// e2eSeedDir is the committed metrics-repo seed StageHome copies into the
// multi/org/legacy variants.
var e2eSeedDir = func() string {
	p, err := filepath.Abs("../../../../harness/metrics-repo")
	if err != nil {
		panic(err)
	}
	return p
}()

// stageVariant stages a fresh $HOME of the given conf variant and points HOME
// at it for the test.
func stageVariant(t *testing.T, variant string) string {
	t.Helper()
	home := filepath.Join(t.TempDir(), "home")
	if err := harness.StageHome(home, variant, e2eSeedDir); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	return home
}

// callLogEntry decodes one line of the fake git/ccusage call log.
type callLogEntry struct {
	Tool string   `json:"tool"`
	Argv []string `json:"argv"`
}

// gitCalls returns the argv of every git invocation logged at path.
func gitCalls(t *testing.T, path string) [][]string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var calls [][]string
	for _, line := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n") {
		if line == "" {
			continue
		}
		var e callLogEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("decode call log line %q: %v", line, err)
		}
		if e.Tool == "git" {
			calls = append(calls, e.Argv)
		}
	}
	return calls
}

// R11: `tu status` byte-exact in each of the four staged conf variants (the
// intake §2 reference bytes; the harness pins machine/user).
func TestE2EStatus(t *testing.T) {
	cases := []struct {
		variant string
		stdout  string
		stderr  string
	}{
		{"single", "Mode:        single (no ~/.config/tu/tu.conf)\n", ""},
		{"multi", "Mode:        multi\n" +
			"User:        harness-user\n" +
			"Machine:     harness-machine\n" +
			"Config:      ~/.config/tu/tu.conf (v2)\n" +
			"Metrics:     ~/.tu/metrics_repo\n" +
			"Last sync:   never\n" +
			"Auto-sync:   on\n", ""},
		{"org", "Mode:        multi\n" +
			"User:        harness-user\n" +
			"Machine:     harness-machine\n" +
			"Org config:  ~/.config/tu/org.conf\n" +
			"Metrics:     ~/.tu/metrics_repo\n" +
			"Last sync:   never\n" +
			"Auto-sync:   on\n", ""},
		{"legacy", "Mode:        multi\n" +
			"User:        harness-user\n" +
			"Machine:     harness-machine\n" +
			"Config:      ~/.tu.conf (v2)\n" +
			"Metrics:     ~/.tu/metrics_repo\n" +
			"Last sync:   never\n" +
			"Auto-sync:   on\n",
			"tu: ~/.tu.conf is deprecated; move it to ~/.config/tu/tu.conf\n"},
	}
	for _, c := range cases {
		t.Run(c.variant, func(t *testing.T) {
			stageVariant(t, c.variant)
			assertRun(t, []string{"status"}, 0, c.stdout, c.stderr)
			// Data flags on a setup command are ignored (DC-02/A-016).
			assertRun(t, []string{"status", "--json"}, 0, c.stdout, c.stderr)
		})
	}
}

// R11/A-022: status with HOME unset is the config-home error, exit 1.
func TestE2EStatusNoHome(t *testing.T) {
	t.Setenv("HOME", "")
	for _, args := range [][]string{{"status"}, {"init-conf"}, {"init-metrics"}} {
		assertRun(t, args, 1, "", "tu: $HOME is not set; cannot locate config\n")
	}
}

// R11: `tu init-conf` create (single), copy (legacy), complete (multi).
func TestE2EInitConf(t *testing.T) {
	t.Run("create from defaults", func(t *testing.T) {
		home := stageVariant(t, "single")
		assertRun(t, []string{"init-conf"}, 0,
			"Created ~/.config/tu/tu.conf — edit it to configure multi-machine sync.\n", "")
		got, err := os.ReadFile(filepath.Join(home, ".config", "tu", "tu.conf"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, config.DefaultConf) {
			t.Errorf("tu.conf differs from the shipped defaults")
		}
	})
	t.Run("copy from legacy", func(t *testing.T) {
		stageVariant(t, "legacy")
		assertRun(t, []string{"init-conf"}, 0, "Copied ~/.tu.conf → ~/.config/tu/tu.conf\n", "")
	})
	t.Run("already complete", func(t *testing.T) {
		stageVariant(t, "multi")
		assertRun(t, []string{"init-conf"}, 0, "~/.config/tu/tu.conf is already complete.\n", "")
		// Data flags on a setup command are ignored (DC-02/A-016).
		assertRun(t, []string{"init-conf", "--fresh"}, 0, "~/.config/tu/tu.conf is already complete.\n", "")
	})
}

// R11: `tu init-metrics` with no repo configured is the unset error, exit 1.
func TestE2EInitMetricsUnset(t *testing.T) {
	stageVariant(t, "single")
	assertRun(t, []string{"init-metrics"}, 1, "",
		"Error: metrics_repo is not set. Add it to ~/.config/tu/tu.conf, run 'tu init-metrics <repo-url>', or set TU_METRICS_REPO.\n")
}

// R11/A-019: `tu init-metrics` in a multi home probes the seeded clone with
// exactly one rev-parse call and reports Already initialized.
func TestE2EInitMetricsAlreadyInitialized(t *testing.T) {
	home := stageVariant(t, "multi")
	log := filepath.Join(t.TempDir(), "calls.jsonl")
	t.Setenv("TUDIFF_CALL_LOG", log)
	dir := filepath.Join(home, ".tu", "metrics_repo")
	assertRun(t, []string{"init-metrics"}, 0, "Already initialized: "+dir+"\n", "")
	calls := gitCalls(t, log)
	want := [][]string{{"-C", dir, "rev-parse", "--git-dir"}}
	if !reflect.DeepEqual(calls, want) {
		t.Errorf("git calls = %v, want %v", calls, want)
	}
}

// R11/A-019: `tu init-metrics <url>` in a single home creates the conf, sets
// metrics_repo, and clones through the fake git with exactly one clone call.
func TestE2EInitMetricsClone(t *testing.T) {
	home := stageVariant(t, "single")
	log := filepath.Join(t.TempDir(), "calls.jsonl")
	t.Setenv("TUDIFF_CALL_LOG", log)
	url := "git@example.invalid:harness/tu-metrics.git"
	dir := filepath.Join(home, ".tu", "metrics_repo")
	assertRun(t, []string{"init-metrics", url}, 0,
		"Created ~/.config/tu/tu.conf — edit it to configure multi-machine sync.\n"+
			"Set metrics_repo = "+url+" in ~/.config/tu/tu.conf\n"+
			"Cloned "+url+" → "+dir+"\n", "")
	calls := gitCalls(t, log)
	want := [][]string{{"clone", url, dir}}
	if !reflect.DeepEqual(calls, want) {
		t.Errorf("git calls = %v, want %v", calls, want)
	}
}

// R10: the two usage errors, byte-exact at the edge.
func TestE2EUsageErrorsB1(t *testing.T) {
	assertRun(t, []string{"init-metrics", "a", "b"}, 2, "",
		"Error: init-metrics takes at most one argument (repo-url)\n"+command.ShortUsage+"\n")
	assertRun(t, []string{"--dry-run"}, 2, "",
		"Error: --dry-run is supported only with 'tu sync' — run 'tu sync --dry-run' to preview a sync.\n")
}

// R12/A-015: a legacy-only conf without metrics_repo prints the deprecation
// line on stderr and the (unchanged) empty table on stdout, exit 0.
func TestE2ELegacyDeprecationDataPath(t *testing.T) {
	home := stageVariant(t, "legacy")
	// Drop metrics_repo so the data path stays single-mode.
	legacy := filepath.Join(home, ".tu.conf")
	raw, err := os.ReadFile(legacy)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(raw), "\n")
	kept := lines[:0]
	for _, l := range lines {
		if !strings.HasPrefix(l, "metrics_repo") {
			kept = append(kept, l)
		}
	}
	if err := os.WriteFile(legacy, []byte(strings.Join(kept, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	assertRun(t, nil, 0,
		"\n\x1b[1;37m📊 Combined Usage (daily)\x1b[0m\n\n  No usage\n\n",
		"tu: ~/.tu.conf is deprecated; move it to ~/.config/tu/tu.conf\n")
}

// R12: a config user of "all" is reserved (used by -u all) — usage exit 2.
func TestE2EReservedUser(t *testing.T) {
	home := stageVariant(t, "single")
	conf := filepath.Join(home, ".config", "tu")
	if err := os.MkdirAll(conf, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(conf, "tu.conf"), []byte("version = 2\nuser = all\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	assertRun(t, []string{"cc"}, 2, "", `Error: config user "all" is reserved (used by -u all)`+"\n")
}
