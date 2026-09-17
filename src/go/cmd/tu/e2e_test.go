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
	"time"

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
	return loggedCalls(t, path, "git")
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

// ── B2: history displays, windows, and the csv/md encoders ─────────────────
//
// Byte references: intake §12 (node v24, placeholder corpus, TZ=UTC, piped).
// The e2e binary writes to a bytes.Buffer, so the width is 80 throughout (no
// bars — R19).

// The populated single-tool window (`cc h --since --until`, §12 cc-h-window;
// identical under TZ=Asia/Kolkata and to `cc h --full`).
const e2eCCHWindow = "\n\x1b[1;37m📊 Claude Code (daily)\x1b[0m\n\n\x1b[1;36mDate        \x1b[0m | \x1b[1;36m         Input\x1b[0m | \x1b[1;36m        Output\x1b[0m | \x1b[1;36m   Cache Write\x1b[0m | \x1b[1;36m    Cache Read\x1b[0m | \x1b[1;36m         Total\x1b[0m | \x1b[1;36m     Cost\x1b[0m\n\x1b[2m─────────────|────────────────|────────────────|────────────────|────────────────|────────────────|──────────\x1b[0m\n2026-01-05   |          3,000 |            400 |          1,000 |         20,000 |         24,400 |     $0.50\n2026-01-06   |          3,000 |            400 |          1,000 |         20,000 |         24,400 |     $0.50\n2026-01-07   |          3,000 |            400 |          1,000 |         20,000 |         24,400 |     $0.50\n\x1b[2m─────────────|────────────────|────────────────|────────────────|────────────────|────────────────|──────────\x1b[0m\n\x1b[1;37mTotal       \x1b[0m | \x1b[1;37m         9,000\x1b[0m | \x1b[1;37m         1,200\x1b[0m | \x1b[1;37m         3,000\x1b[0m | \x1b[1;37m        60,000\x1b[0m | \x1b[1;37m        73,200\x1b[0m | \x1b[1;37m    $1.50\x1b[0m\n\x1b[2mavg $0.50/day · peak $0.50 (2026-01-05)\x1b[0m\n\n"

// The populated all-tools window (`h --since --until`, §12 h-window; the same
// bytes for `h --full` and the `dh`/`history` aliases).
const e2eHWindow = "\n\x1b[1;37m📊 Combined Cost History (daily)\x1b[0m\n\n\x1b[1;36mDate      \x1b[0m | \x1b[1;36mClaude Code\x1b[0m | \x1b[1;36m    Codex\x1b[0m | \x1b[1;36m OpenCode\x1b[0m | \x1b[1;36m   Gemini\x1b[0m | \x1b[1;36m  Copilot\x1b[0m | \x1b[1;36m     Kimi\x1b[0m | \x1b[1;36m     Cost\x1b[0m\n\x1b[2m───────────|─────────────|───────────|───────────|───────────|───────────|───────────|──────────\x1b[0m\n2026-01-05 |       $0.50 |     $0.50 |     $0.50 |     $0.50 |     $0.50 |     $0.50 |     $3.00\n2026-01-06 |       $0.50 |     $0.50 |     $0.50 |     $0.50 |     $0.50 |     $0.50 |     $3.00\n2026-01-07 |       $0.50 |     $0.50 |     $0.50 |     $0.50 |     $0.50 |     $0.50 |     $3.00\n\x1b[2m───────────|─────────────|───────────|───────────|───────────|───────────|───────────|──────────\x1b[0m\n\x1b[1;37mTotal     \x1b[0m | \x1b[1;37m      $1.50\x1b[0m | \x1b[1;37m    $1.50\x1b[0m | \x1b[1;37m    $1.50\x1b[0m | \x1b[1;37m    $1.50\x1b[0m | \x1b[1;37m    $1.50\x1b[0m | \x1b[1;37m    $1.50\x1b[0m | \x1b[1;37m    $9.00\x1b[0m\n\x1b[2mavg $3.00/day · peak $3.00 (2026-01-05)\x1b[0m\n\n"

func TestE2EHistoryWindow(t *testing.T) {
	assertRun(t, []string{"cc", "h", "--since", "2026-01-01", "--until", "2026-01-31"}, 0, e2eCCHWindow, "")
	assertRun(t, []string{"h", "--since", "2026-01-01", "--until", "2026-01-31"}, 0, e2eHWindow, "")
	// --full is the uncapped equivalent; the aliases share the bytes.
	assertRun(t, []string{"cc", "h", "--full"}, 0, e2eCCHWindow, "")
	assertRun(t, []string{"dh", "--full"}, 0, e2eHWindow, "")
}

// The weekly window rolls up to one row labeled with the leading Sunday
// 2026-01-04, which precedes --since (R2).
func TestE2EWeeklyWindow(t *testing.T) {
	want := "\n\x1b[1;37m📊 Combined Cost History (weekly)\x1b[0m\n\n\x1b[1;36mDate      \x1b[0m | \x1b[1;36mClaude Code\x1b[0m | \x1b[1;36m    Codex\x1b[0m | \x1b[1;36m OpenCode\x1b[0m | \x1b[1;36m   Gemini\x1b[0m | \x1b[1;36m  Copilot\x1b[0m | \x1b[1;36m     Kimi\x1b[0m | \x1b[1;36m     Cost\x1b[0m\n\x1b[2m───────────|─────────────|───────────|───────────|───────────|───────────|───────────|──────────\x1b[0m\n2026-01-04 |       $1.50 |     $1.50 |     $1.50 |     $1.50 |     $1.50 |     $1.50 |     $9.00\n\n"
	assertRun(t, []string{"wh", "--since", "2026-01-01", "--until", "2026-01-31"}, 0, want, "")
}

// One-row windows render no divider, Total or footer (§12 mh / cc-mh).
func TestE2EMonthlyHistory(t *testing.T) {
	all := "\n\x1b[1;37m📊 Combined Cost History (monthly)\x1b[0m\n\n\x1b[1;36mDate      \x1b[0m | \x1b[1;36mClaude Code\x1b[0m | \x1b[1;36m    Codex\x1b[0m | \x1b[1;36m OpenCode\x1b[0m | \x1b[1;36m   Gemini\x1b[0m | \x1b[1;36m  Copilot\x1b[0m | \x1b[1;36m     Kimi\x1b[0m | \x1b[1;36m     Cost\x1b[0m\n\x1b[2m───────────|─────────────|───────────|───────────|───────────|───────────|───────────|──────────\x1b[0m\n2026-01    |       $1.50 |     $1.50 |     $1.50 |     $1.50 |     $1.50 |     $1.50 |     $9.00\n\n"
	assertRun(t, []string{"mh"}, 0, all, "")

	cc := "\n\x1b[1;37m📊 Claude Code (monthly)\x1b[0m\n\n\x1b[1;36mDate        \x1b[0m | \x1b[1;36m         Input\x1b[0m | \x1b[1;36m        Output\x1b[0m | \x1b[1;36m   Cache Write\x1b[0m | \x1b[1;36m    Cache Read\x1b[0m | \x1b[1;36m         Total\x1b[0m | \x1b[1;36m     Cost\x1b[0m\n\x1b[2m─────────────|────────────────|────────────────|────────────────|────────────────|────────────────|──────────\x1b[0m\n2026-01      |          9,000 |          1,200 |          3,000 |         60,000 |         73,200 |     $1.50\n\n"
	assertRun(t, []string{"cc", "mh"}, 0, cc, "")
}

// Bare daily history is capped (the placeholder corpus predates the floor):
// the heading carries the hint and the table is the empty state.
func TestE2EHistoryCappedEmpty(t *testing.T) {
	assertRun(t, []string{"h"}, 0,
		"\n\x1b[1;37m📊 Combined Cost History (daily, last 3 months)\x1b[0m\n\n  No data\n\n", "")
	assertRun(t, []string{"cc", "h"}, 0,
		"\n\x1b[1;37m📊 Claude Code (daily, last 3 months)\x1b[0m\n\n  No data\n\n", "")
}

func TestE2EHistoryJSON(t *testing.T) {
	assertRun(t, []string{"h", "--json"}, 0,
		"{\n  \"Claude Code\": [],\n  \"Codex\": [],\n  \"OpenCode\": [],\n  \"Gemini\": [],\n  \"Copilot\": [],\n  \"Kimi\": []\n}\n", "")
	assertRun(t, []string{"cc", "mh", "--json"}, 0,
		"[\n  {\n    \"label\": \"2026-01\",\n    \"totalCost\": 1.5,\n    \"inputTokens\": 9000,\n    \"outputTokens\": 1200,\n    \"cacheCreationTokens\": 3000,\n    \"cacheReadTokens\": 60000,\n    \"totalTokens\": 73200\n  }\n]\n", "")
}

func TestE2EHistoryCSV(t *testing.T) {
	assertRun(t, []string{"h", "--csv"}, 0,
		"date,Claude Code,Codex,OpenCode,Gemini,Copilot,Kimi,total\n", "")
	assertRun(t, []string{"cc", "mh", "--csv"}, 0,
		"date,input,output,cache_write,cache_read,total,cost\n2026-01,9000,1200,3000,60000,73200,1.50\n", "")
	// Snapshot CSV empties (the placeholder corpus never matches today).
	assertRun(t, []string{"--csv"}, 0, "tool,tokens,input,output,cache,cost\n", "")
	assertRun(t, []string{"cc", "--csv"}, 0, "tool,tokens,input,output,cache,cost\n", "")
}

func TestE2EHistoryMarkdown(t *testing.T) {
	assertRun(t, []string{"h", "--md"}, 0,
		"## Combined Cost History (daily, last 3 months)\n\n| Date | Claude Code | Codex | OpenCode | Gemini | Copilot | Kimi | Cost |\n| :--- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |\n\n", "")
	assertRun(t, []string{"cc", "mh", "--md"}, 0,
		"## Claude Code (monthly)\n\n| Date | Input | Output | Cache Write | Cache Read | Total | Cost |\n| :--- | ---: | ---: | ---: | ---: | ---: | ---: |\n| 2026-01 | 9,000 | 1,200 | 3,000 | 60,000 | 73,200 | $1.50 |\n\n", "")
	assertRun(t, []string{"--md"}, 0,
		"## Combined Usage (daily)\n\n| Tool | Tokens | Input | Output | Cache | Cost |\n| :--- | ---: | ---: | ---: | ---: | ---: |\n\n", "")
}

// The since/until snapshot guard: a well-shaped impossible date warns on
// stderr and still renders the (empty) snapshot with exit 0 (since-invalid).
func TestE2ESinceInvalidWarnsAndRenders(t *testing.T) {
	assertRun(t, []string{"--since", "2026-13-01"}, 0,
		"\n\x1b[1;37m📊 Combined Usage (daily)\x1b[0m\n\n  No usage\n\n",
		"Warning: --since/--until apply to history display — ignoring.\n")
}

// ── B4: machine columns ────────────────────────────────────────────────────
//
// Byte references: intake §9 (node v24, placeholder corpus + committed seed,
// TZ=UTC, piped, width 80 — no bars).

// The empty-state by-machine surfaces: the all-tools pivot warns and clears
// (capped pivot heading + "  No data", exit 0); the single-tool history keeps
// the flag and renders its capped empty state.
func TestE2EHistoryByMachineEmptyStates(t *testing.T) {
	assertRun(t, []string{"h", "--by-machine"}, 0,
		"\n\x1b[1;37m📊 Combined Cost History (daily, last 3 months)\x1b[0m\n\n  No data\n\n",
		"Warning: --by-machine is not supported with all-tools history — ignoring.\n")
	assertRun(t, []string{"cc", "h", "--by-machine"}, 0,
		"\n\x1b[1;37m📊 Claude Code (daily, last 3 months)\x1b[0m\n\n  No data\n\n", "")
	// The multi-home snapshot on today is "  No usage" (no current-label
	// records) — no columns, no legend; -u all is in scope (multi), no notice.
	stageVariant(t, "multi")
	assertRun(t, []string{"--by-machine", "-u", "all"}, 0,
		"\n\x1b[1;37m📊 Combined Usage (daily)\x1b[0m\n\n  No usage\n\n", "")
}

// The populated multi-mode single-tool window with machine columns: A =
// harness-machine, B = other-box; the 2026-01-06 row splits $0.75 / $0.40;
// the Total row carries the per-machine sums $1.75 / $0.40; the legend trails
// the footer.
const e2eMultiCCHWindowByMachine = "\n\x1b[1;37m📊 Claude Code (daily)\x1b[0m\n\n\x1b[1;36mDate        \x1b[0m | \x1b[1;36m         Input\x1b[0m | \x1b[1;36m        Output\x1b[0m | \x1b[1;36m   Cache Write\x1b[0m | \x1b[1;36m    Cache Read\x1b[0m | \x1b[1;36m         Total\x1b[0m | \x1b[1;36m     Cost\x1b[0m | \x1b[1;36m        A\x1b[0m | \x1b[1;36m        B\x1b[0m\n\x1b[2m─────────────|────────────────|────────────────|────────────────|────────────────|────────────────|───────────|───────────|──────────\x1b[0m\n2026-01-05   |          3,000 |            400 |          1,000 |         20,000 |         24,400 |     $0.50 |     $0.50 | \x1b[2m    $0.00\x1b[0m\n2026-01-06   |          6,000 |            800 |          2,000 |         40,000 |         48,800 |     $1.15 |     $0.75 |     $0.40\n2026-01-07   |          3,000 |            400 |          1,000 |         20,000 |         24,400 |     $0.50 |     $0.50 | \x1b[2m    $0.00\x1b[0m\n\x1b[2m─────────────|────────────────|────────────────|────────────────|────────────────|────────────────|───────────|───────────|──────────\x1b[0m\n\x1b[1;37mTotal       \x1b[0m | \x1b[1;37m        12,000\x1b[0m | \x1b[1;37m         1,600\x1b[0m | \x1b[1;37m         4,000\x1b[0m | \x1b[1;37m        80,000\x1b[0m | \x1b[1;37m        97,600\x1b[0m | \x1b[1;37m    $2.15\x1b[0m | \x1b[1;37m    $1.75\x1b[0m | \x1b[1;37m    $0.40\x1b[0m\n\x1b[2mavg $0.72/day · peak $1.15 (2026-01-06)\x1b[0m\n\n\x1b[2mMachines: A = harness-machine, B = other-box\x1b[0m\n\n"

func TestE2EMultiHistoryByMachine(t *testing.T) {
	stageVariant(t, "multi")
	assertRun(t, []string{"cc", "h", "--by-machine", "--since", "2026-01-01", "--until", "2026-01-31"}, 0, e2eMultiCCHWindowByMachine, "")
}

// The monthly JSON carries the machines object in first-seen key order (own
// machine first) with cost values after totalTokens.
func TestE2EMultiMonthlyByMachineJSON(t *testing.T) {
	stageVariant(t, "multi")
	assertRun(t, []string{"cc", "mh", "--by-machine", "--json"}, 0,
		"[\n  {\n    \"label\": \"2026-01\",\n    \"totalCost\": 2.15,\n    \"inputTokens\": 12000,\n    \"outputTokens\": 1600,\n    \"cacheCreationTokens\": 4000,\n    \"cacheReadTokens\": 80000,\n    \"totalTokens\": 97600,\n    \"machines\": {\n      \"harness-machine\": 1.75,\n      \"other-box\": 0.4\n    }\n  }\n]\n", "")
}

// Under -u all the columns key by user (the header prefix stays machine_).
func TestE2EMultiMonthlyByMachineUserAllCSV(t *testing.T) {
	stageVariant(t, "multi")
	assertRun(t, []string{"cc", "mh", "--by-machine", "-u", "all", "--csv"}, 0,
		"date,input,output,cache_write,cache_read,total,cost,machine_harness-user_cost,machine_other-user_cost\n"+
			"2026-01,12000,1600,4000,80000,97600,2.50,1.40,1.10\n", "")
}

// The single-home pinned-machine window under -u all: the -u warning, then
// machine columns (one column, the local machine) — the TS clears -u before
// computing the users legend.
const e2eSingleCCHWindowByMachine = "\n\x1b[1;37m📊 Claude Code (daily)\x1b[0m\n\n\x1b[1;36mDate        \x1b[0m | \x1b[1;36m         Input\x1b[0m | \x1b[1;36m        Output\x1b[0m | \x1b[1;36m   Cache Write\x1b[0m | \x1b[1;36m    Cache Read\x1b[0m | \x1b[1;36m         Total\x1b[0m | \x1b[1;36m     Cost\x1b[0m | \x1b[1;36m        A\x1b[0m\n\x1b[2m─────────────|────────────────|────────────────|────────────────|────────────────|────────────────|───────────|──────────\x1b[0m\n2026-01-05   |          3,000 |            400 |          1,000 |         20,000 |         24,400 |     $0.50 |     $0.50\n2026-01-06   |          3,000 |            400 |          1,000 |         20,000 |         24,400 |     $0.50 |     $0.50\n2026-01-07   |          3,000 |            400 |          1,000 |         20,000 |         24,400 |     $0.50 |     $0.50\n\x1b[2m─────────────|────────────────|────────────────|────────────────|────────────────|────────────────|───────────|──────────\x1b[0m\n\x1b[1;37mTotal       \x1b[0m | \x1b[1;37m         9,000\x1b[0m | \x1b[1;37m         1,200\x1b[0m | \x1b[1;37m         3,000\x1b[0m | \x1b[1;37m        60,000\x1b[0m | \x1b[1;37m        73,200\x1b[0m | \x1b[1;37m    $1.50\x1b[0m | \x1b[1;37m    $1.50\x1b[0m\n\x1b[2mavg $0.50/day · peak $0.50 (2026-01-05)\x1b[0m\n\n\x1b[2mMachines: A = harness-machine\x1b[0m\n\n"

func TestE2ESingleByMachineUserAll(t *testing.T) {
	// A pinned machine (no metrics_repo → single mode) keeps the legend byte
	// stable across hosts.
	home := stageVariant(t, "single")
	conf := filepath.Join(home, ".config", "tu")
	if err := os.MkdirAll(conf, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(conf, "tu.conf"), []byte("version = 2\nmachine = harness-machine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	assertRun(t, []string{"cc", "h", "--by-machine", "-u", "all", "--since", "2026-01-01", "--until", "2026-01-31"}, 0,
		e2eSingleCCHWindowByMachine,
		"Warning: -u flag requires multi mode — ignoring.\n")
}

// ── B3: metrics-repo source and multi-mode merge ───────────────────────────
//
// Byte references: intake §9 (node v24, placeholder corpus + committed seed,
// TZ=UTC, piped). The e2e binary writes to a bytes.Buffer, so width is 80 and
// color is on (NO_COLOR unset).

// The populated multi-mode single-tool window (`cc h --since --until` against
// the seeded repo: live 0.50 wins 01-05, stored 0.75 wins 01-06 plus the
// other-box 0.40, codex untouched).
const e2eMultiCCHWindow = "\n\x1b[1;37m📊 Claude Code (daily)\x1b[0m\n\n\x1b[1;36mDate        \x1b[0m | \x1b[1;36m         Input\x1b[0m | \x1b[1;36m        Output\x1b[0m | \x1b[1;36m   Cache Write\x1b[0m | \x1b[1;36m    Cache Read\x1b[0m | \x1b[1;36m         Total\x1b[0m | \x1b[1;36m     Cost\x1b[0m\n\x1b[2m─────────────|────────────────|────────────────|────────────────|────────────────|────────────────|──────────\x1b[0m\n2026-01-05   |          3,000 |            400 |          1,000 |         20,000 |         24,400 |     $0.50\n2026-01-06   |          6,000 |            800 |          2,000 |         40,000 |         48,800 |     $1.15\n2026-01-07   |          3,000 |            400 |          1,000 |         20,000 |         24,400 |     $0.50\n\x1b[2m─────────────|────────────────|────────────────|────────────────|────────────────|────────────────|──────────\x1b[0m\n\x1b[1;37mTotal       \x1b[0m | \x1b[1;37m        12,000\x1b[0m | \x1b[1;37m         1,600\x1b[0m | \x1b[1;37m         4,000\x1b[0m | \x1b[1;37m        80,000\x1b[0m | \x1b[1;37m        97,600\x1b[0m | \x1b[1;37m    $2.15\x1b[0m\n\x1b[2mavg $0.72/day · peak $1.15 (2026-01-06)\x1b[0m\n\n"

// The populated multi-mode all-tools window (`h --since --until` against the
// seed): the merged cc column, codex's other-box 01-07 sum, and the $9.95
// grand total.
const e2eMultiHWindow = "\n\x1b[1;37m📊 Combined Cost History (daily)\x1b[0m\n\n\x1b[1;36mDate      \x1b[0m | \x1b[1;36mClaude Code\x1b[0m | \x1b[1;36m    Codex\x1b[0m | \x1b[1;36m OpenCode\x1b[0m | \x1b[1;36m   Gemini\x1b[0m | \x1b[1;36m  Copilot\x1b[0m | \x1b[1;36m     Kimi\x1b[0m | \x1b[1;36m     Cost\x1b[0m\n\x1b[2m───────────|─────────────|───────────|───────────|───────────|───────────|───────────|──────────\x1b[0m\n2026-01-05 |       $0.50 |     $0.50 |     $0.50 |     $0.50 |     $0.50 |     $0.50 |     $3.00\n2026-01-06 |       $1.15 |     $0.50 |     $0.50 |     $0.50 |     $0.50 |     $0.50 |     $3.65\n2026-01-07 |       $0.50 |     $0.80 |     $0.50 |     $0.50 |     $0.50 |     $0.50 |     $3.30\n\x1b[2m───────────|─────────────|───────────|───────────|───────────|───────────|───────────|──────────\x1b[0m\n\x1b[1;37mTotal     \x1b[0m | \x1b[1;37m      $2.15\x1b[0m | \x1b[1;37m    $1.80\x1b[0m | \x1b[1;37m    $1.50\x1b[0m | \x1b[1;37m    $1.50\x1b[0m | \x1b[1;37m    $1.50\x1b[0m | \x1b[1;37m    $1.50\x1b[0m | \x1b[1;37m    $9.95\x1b[0m\n\x1b[2mavg $3.32/day · peak $3.65 (2026-01-06)\x1b[0m\n\n"

// loggedCalls returns the argv of every invocation of tool logged at path.
func loggedCalls(t *testing.T, path, tool string) [][]string {
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
		if e.Tool == tool {
			calls = append(calls, e.Argv)
		}
	}
	return calls
}

// R13/R9: the seeded multi variants render the merged window bytes; the
// legacy variant prefixes its deprecation line. The own-user fetch also
// WRITES the day-files before reading (B6's write-then-read shape): 01-05
// updates to the live $0.50, 01-06 keeps the seeded $0.75 (never-shrink),
// 01-07 is new — while the rendered bytes stay identical.
func TestE2EMultiHistoryWindow(t *testing.T) {
	for _, variant := range []string{"multi", "org", "legacy"} {
		t.Run(variant, func(t *testing.T) {
			home := stageVariant(t, variant)
			stderr := ""
			if variant == "legacy" {
				stderr = "tu: ~/.tu.conf is deprecated; move it to ~/.config/tu/tu.conf\n"
			}
			assertRun(t, []string{"cc", "h", "--since", "2026-01-01", "--until", "2026-01-31"}, 0, e2eMultiCCHWindow, stderr)
			assertRun(t, []string{"h", "--since", "2026-01-01", "--until", "2026-01-31"}, 0, e2eMultiHWindow, stderr)
			machineDir := filepath.Join(home, ".tu", "metrics_repo", "harness-user", "2026", "harness-machine")
			assertDayFile(t, filepath.Join(machineDir, "cc-2026-01-05.jsonl"), "2026-01-05", "0.5")
			assertDayFile(t, filepath.Join(machineDir, "cc-2026-01-06.jsonl"), "2026-01-06", "0.75")
			assertDayFile(t, filepath.Join(machineDir, "cc-2026-01-07.jsonl"), "2026-01-07", "0.5")
		})
	}
}

// R10/A-015: the monthly JSON carries 2.15 (not 2.1500000000000004) — the
// daily collapse reproduces the TS summation order.
func TestE2EMultiMonthlyJSON(t *testing.T) {
	stageVariant(t, "multi")
	assertRun(t, []string{"cc", "mh", "--json"}, 0,
		"[\n  {\n    \"label\": \"2026-01\",\n    \"totalCost\": 2.15,\n    \"inputTokens\": 12000,\n    \"outputTokens\": 1600,\n    \"cacheCreationTokens\": 4000,\n    \"cacheReadTokens\": 80000,\n    \"totalTokens\": 97600\n  }\n]\n", "")
}

// R9/A-019: -u <other> and -u all in multi mode render the (empty) snapshot
// from the repo alone — no ccusage call, no git call, empty stderr.
func TestE2EMultiUserPaths(t *testing.T) {
	emptySnapshot := "\n\x1b[1;37m📊 Combined Usage (daily)\x1b[0m\n\n  No usage\n\n"
	for _, args := range [][]string{{"-u", "other-user"}, {"-u", "all"}} {
		stageVariant(t, "multi")
		log := filepath.Join(t.TempDir(), "calls.jsonl")
		t.Setenv("TUDIFF_CALL_LOG", log)
		assertRun(t, args, 0, emptySnapshot, "")
		if calls := loggedCalls(t, log, "ccusage"); len(calls) != 0 {
			t.Errorf("run(%q) ccusage calls = %v, want none (repo-only path)", args, calls)
		}
		if calls := loggedCalls(t, log, "git"); len(calls) != 0 {
			t.Errorf("run(%q) git calls = %v, want none", args, calls)
		}
	}
}

// R8: -u in single mode warns and ignores, then renders the live snapshot
// (six ccusage calls), exit 0.
func TestE2ESingleUserWarnsAndRenders(t *testing.T) {
	stageVariant(t, "single")
	log := filepath.Join(t.TempDir(), "calls.jsonl")
	t.Setenv("TUDIFF_CALL_LOG", log)
	assertRun(t, []string{"-u", "other-user"}, 0,
		"\n\x1b[1;37m📊 Combined Usage (daily)\x1b[0m\n\n  No usage\n\n",
		"Warning: -u flag requires multi mode — ignoring.\n")
	if calls := loggedCalls(t, log, "ccusage"); len(calls) != 6 {
		t.Errorf("ccusage calls = %v, want six (one per tool)", calls)
	}
}

// R13/A-019: the envrepo axis — TU_METRICS_REPO set on a single home, metrics
// dir absent: one `git clone <url> <abs dir>`, the Cloned line on stderr, then
// the live-only render (the fake git creates no directory).
func TestE2EEnvRepoAutoClone(t *testing.T) {
	home := stageVariant(t, "single")
	t.Setenv("TU_METRICS_REPO", harness.MetricsRepoURL)
	log := filepath.Join(t.TempDir(), "calls.jsonl")
	t.Setenv("TUDIFF_CALL_LOG", log)
	dir := filepath.Join(home, ".tu", "metrics_repo")
	assertRun(t, nil, 0,
		"\n\x1b[1;37m📊 Combined Usage (daily)\x1b[0m\n\n  No usage\n\n",
		"Cloned metrics repo → "+dir+"\n")
	calls := loggedCalls(t, log, "git")
	if want := [][]string{{"clone", harness.MetricsRepoURL, dir}}; !reflect.DeepEqual(calls, want) {
		t.Errorf("git calls = %v, want %v", calls, want)
	}
	if calls := loggedCalls(t, log, "ccusage"); len(calls) != 6 {
		t.Errorf("ccusage calls = %v, want six", calls)
	}
	// A successful clone clears the marker and leaves the config multi.
	if _, err := os.Stat(filepath.Join(home, ".tu", ".clone-failed")); !os.IsNotExist(err) {
		t.Error(".clone-failed present after a successful clone")
	}
}

// R6: a fresh .clone-failed marker suppresses the clone — the not-available
// warning, no git call, the single-mode live render, exit 0.
func TestE2ECloneMarkerFreshFallback(t *testing.T) {
	home := stageVariant(t, "single")
	t.Setenv("TU_METRICS_REPO", harness.MetricsRepoURL)
	log := filepath.Join(t.TempDir(), "calls.jsonl")
	t.Setenv("TUDIFF_CALL_LOG", log)
	stateDir := filepath.Join(home, ".tu")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	if err := os.WriteFile(filepath.Join(stateDir, ".clone-failed"), []byte(marker), 0o644); err != nil {
		t.Fatal(err)
	}
	assertRun(t, nil, 0,
		"\n\x1b[1;37m📊 Combined Usage (daily)\x1b[0m\n\n  No usage\n\n",
		"Warning: metrics repo not available — falling back to single mode.\n")
	if calls := loggedCalls(t, log, "git"); len(calls) != 0 {
		t.Errorf("git calls = %v, want none (fresh marker)", calls)
	}
}

// R13: setup commands never run the auto-clone guard — `tu status` in an
// envrepo home records no git call.
func TestE2EStatusNeverClones(t *testing.T) {
	stageVariant(t, "single")
	t.Setenv("TU_METRICS_REPO", harness.MetricsRepoURL)
	log := filepath.Join(t.TempDir(), "calls.jsonl")
	t.Setenv("TUDIFF_CALL_LOG", log)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"status"}, &stdout, &stderr); code != 0 {
		t.Errorf("status exit = %d, want 0", code)
	}
	if calls := loggedCalls(t, log, "git"); len(calls) != 0 {
		t.Errorf("git calls = %v, want none for status", calls)
	}
}

// ── B5: leaderboards (lb / lbh) ────────────────────────────────────────────
//
// Byte references: the node oracle (node v24) against the committed seed with
// the harness-staged multi home, TZ=UTC, piped (width 80), captured
// 2026-09-16. The lb bars render at 80 columns here — the two-user table is
// narrow enough.

const e2eLbGate = "Error: lb requires multi mode — run tu init-metrics <repo-url> to set up a metrics repo\n"

// R1/A-017: in single mode lb/lbh exit 1 with the one gate line (naming lb
// even for lbh — DC-14), empty stdout, and NO -u notice preceding it.
func TestE2ELeaderboardSingleModeGate(t *testing.T) {
	for _, args := range [][]string{{"lb"}, {"lbh"}, {"lb", "-u", "other-user"}} {
		stageVariant(t, "single")
		assertRun(t, args, 1, "", e2eLbGate)
	}
}

// A multi-home bare lb on today's empty window: the heading carries today's
// local (UTC) date, then the empty state and the staleness footer, exit 0.
// -u all is a silent no-op (no notice); --full warns like a snapshot.
func TestE2ELeaderboardTodayEmpty(t *testing.T) {
	today := time.Now().Format("2006-01-02")
	empty := "\n\x1b[1;37mLeaderboard (daily) · " + today + " · by cost\x1b[0m\n\n  No data\n\n\x1b[2mnever synced · tu sync to refresh\x1b[0m\n\n"
	stageVariant(t, "multi")
	assertRun(t, []string{"lb"}, 0, empty, "")
	assertRun(t, []string{"lb", "-u", "all"}, 0, empty, "")
	assertRun(t, []string{"lb", "--full"}, 0, empty,
		"Warning: --full applies to daily/weekly history — ignoring.\n")
}

// --top on a non-leaderboard display warns (byte-exact) and is ignored: the
// snapshot renders, exit 0.
func TestE2ETopSnapshotGuard(t *testing.T) {
	stageVariant(t, "single")
	assertRun(t, []string{"--top", "3"}, 0,
		"\n\x1b[1;37m📊 Combined Usage (daily)\x1b[0m\n\n  No usage\n\n",
		"Warning: --top applies to leaderboard display — ignoring.\n")
}

const e2eLbWindow = "\n\x1b[1;37mLeaderboard (daily) · 2026-01-01 → 2026-01-31 · by cost\x1b[0m\n\n\x1b[1;36m#\x1b[0m | \x1b[1;36mUser          \x1b[0m | \x1b[1;36m     Cost\x1b[0m                   | \x1b[1;36m   Tokens\x1b[0m | \x1b[1;36mShare\x1b[0m | \x1b[1;36mΔ vs prev\x1b[0m\n\x1b[2m──|────────────────|─────────────────────────────|───────────|───────|──────────\x1b[0m\n1 | harness-user ◂ |     $1.70 \x1b[32m█████████████████\x1b[0m |    97,600 | 56.7% |       new\n2 | other-user     |     $1.30 \x1b[32m█████████████\x1b[0m     |    48,800 | 43.3% |       new\n\x1b[2m──|────────────────|─────────────────────────────|───────────|───────|──────────\x1b[0m\n\x1b[1;37m \x1b[0m | \x1b[1;37mTotal         \x1b[0m | \x1b[1;37m    $3.00\x1b[0m                   | \x1b[1;37m  146,400\x1b[0m | \x1b[1;37m     \x1b[0m | \x1b[1;37m         \x1b[0m\n\x1b[2mnever synced · tu sync to refresh\x1b[0m\n\n"

const e2eLbWindowJSON = "[\n  {\n    \"rank\": 1,\n    \"user\": \"harness-user\",\n    \"cost\": 1.7,\n    \"totalTokens\": 97600,\n    \"share\": 0.5666666666666667,\n    \"delta\": null\n  },\n  {\n    \"rank\": 2,\n    \"user\": \"other-user\",\n    \"cost\": 1.3,\n    \"totalTokens\": 48800,\n    \"share\": 0.43333333333333335,\n    \"delta\": null\n  }\n]\n"

const e2eLbWindowCSV = "rank,user,cost,total_tokens,share,delta\n1,harness-user,1.70,97600,0.567,\n2,other-user,1.30,48800,0.433,\nTotal,,3.00,146400,,\n"

const e2eLbWindowMD = "## Leaderboard (daily)\n\n| # | User | Cost | Tokens | Share | Δ vs prev |\n| ---: | :--- | ---: | ---: | ---: | ---: |\n| 1 | harness-user | $1.70 | 97,600 | 56.7% | new |\n| 2 | other-user | $1.30 | 48,800 | 43.3% | new |\n| **Total** |  | **$3.00** | **146,400** |  |  |\n\n"

const e2eLbWindowByMachine = "\n\x1b[1;37mLeaderboard (daily) · 2026-01-01 → 2026-01-31 · by cost\x1b[0m\n\n\x1b[1;36m#\x1b[0m | \x1b[1;36mUser                          \x1b[0m | \x1b[1;36m     Cost\x1b[0m | \x1b[1;36m   Tokens\x1b[0m | \x1b[1;36mShare\x1b[0m | \x1b[1;36mΔ vs prev\x1b[0m\n\x1b[2m──|────────────────────────────────|───────────|───────────|───────|──────────\x1b[0m\n1 | other-user/laptop              |     $1.30 |    48,800 | 43.3% |       new\n2 | harness-user/harness-machine ◂ |     $1.00 |    48,800 | 33.3% |       new\n3 | harness-user/other-box ◂       |     $0.70 |    48,800 | 23.3% |       new\n\x1b[2m──|────────────────────────────────|───────────|───────────|───────|──────────\x1b[0m\n\x1b[1;37m \x1b[0m | \x1b[1;37mTotal                         \x1b[0m | \x1b[1;37m    $3.00\x1b[0m | \x1b[1;37m  146,400\x1b[0m | \x1b[1;37m     \x1b[0m | \x1b[1;37m         \x1b[0m\n\x1b[2mnever synced · tu sync to refresh\x1b[0m\n\n"

const e2eLbWindowByMachineTokens = "\n\x1b[1;37mLeaderboard (daily) · 2026-01-01 → 2026-01-31 · by tokens\x1b[0m\n\n\x1b[1;36m#\x1b[0m | \x1b[1;36mUser                          \x1b[0m | \x1b[1;36m     Cost\x1b[0m | \x1b[1;36m   Tokens\x1b[0m | \x1b[1;36mShare\x1b[0m | \x1b[1;36mΔ vs prev\x1b[0m\n\x1b[2m──|────────────────────────────────|───────────|───────────|───────|──────────\x1b[0m\n1 | harness-user/harness-machine ◂ |     $1.00 |    48,800 | 33.3% |       new\n2 | harness-user/other-box ◂       |     $0.70 |    48,800 | 33.3% |       new\n3 | other-user/laptop              |     $1.30 |    48,800 | 33.3% |       new\n\x1b[2m──|────────────────────────────────|───────────|───────────|───────|──────────\x1b[0m\n\x1b[1;37m \x1b[0m | \x1b[1;37mTotal                         \x1b[0m | \x1b[1;37m    $3.00\x1b[0m | \x1b[1;37m  146,400\x1b[0m | \x1b[1;37m     \x1b[0m | \x1b[1;37m         \x1b[0m\n\x1b[2mnever synced · tu sync to refresh\x1b[0m\n\n"

const e2eLbWindowTop1 = "\n\x1b[1;37mLeaderboard (daily) · 2026-01-01 → 2026-01-31 · by cost\x1b[0m\n\n\x1b[1;36m#\x1b[0m | \x1b[1;36mUser          \x1b[0m | \x1b[1;36m     Cost\x1b[0m                   | \x1b[1;36m   Tokens\x1b[0m | \x1b[1;36mShare\x1b[0m | \x1b[1;36mΔ vs prev\x1b[0m\n\x1b[2m──|────────────────|─────────────────────────────|───────────|───────|──────────\x1b[0m\n1 | harness-user ◂ |     $1.70 \x1b[32m█████████████████\x1b[0m |    97,600 | 56.7% |       new\n  | \x1b[2m… +1 others   \x1b[0m |                             |           |       |          \n\x1b[2m──|────────────────|─────────────────────────────|───────────|───────|──────────\x1b[0m\n\x1b[1;37m \x1b[0m | \x1b[1;37mTotal         \x1b[0m | \x1b[1;37m    $3.00\x1b[0m                   | \x1b[1;37m  146,400\x1b[0m | \x1b[1;37m     \x1b[0m | \x1b[1;37m         \x1b[0m\n\x1b[2mnever synced · tu sync to refresh\x1b[0m\n\n"

const e2eLbWindowTokens = "\n\x1b[1;37mLeaderboard (daily) · 2026-01-01 → 2026-01-31 · by tokens\x1b[0m\n\n\x1b[1;36m#\x1b[0m | \x1b[1;36mUser          \x1b[0m | \x1b[1;36m     Cost\x1b[0m                   | \x1b[1;36m   Tokens\x1b[0m | \x1b[1;36mShare\x1b[0m | \x1b[1;36mΔ vs prev\x1b[0m\n\x1b[2m──|────────────────|─────────────────────────────|───────────|───────|──────────\x1b[0m\n1 | harness-user ◂ |     $1.70 \x1b[32m█████████████████\x1b[0m |    97,600 | 66.7% |       new\n2 | other-user     |     $1.30 \x1b[32m████████▌\x1b[0m         |    48,800 | 33.3% |       new\n\x1b[2m──|────────────────|─────────────────────────────|───────────|───────|──────────\x1b[0m\n\x1b[1;37m \x1b[0m | \x1b[1;37mTotal         \x1b[0m | \x1b[1;37m    $3.00\x1b[0m                   | \x1b[1;37m  146,400\x1b[0m | \x1b[1;37m     \x1b[0m | \x1b[1;37m         \x1b[0m\n\x1b[2mnever synced · tu sync to refresh\x1b[0m\n\n"

const e2eCCLbWindow = "\n\x1b[1;37mLeaderboard (daily) · 2026-01-01 → 2026-01-31 · by cost\x1b[0m\n\n\x1b[1;36m#\x1b[0m | \x1b[1;36mUser          \x1b[0m | \x1b[1;36m     Cost\x1b[0m                   | \x1b[1;36m   Tokens\x1b[0m | \x1b[1;36mShare\x1b[0m | \x1b[1;36mΔ vs prev\x1b[0m\n\x1b[2m──|────────────────|─────────────────────────────|───────────|───────|──────────\x1b[0m\n1 | harness-user ◂ |     $1.40 \x1b[32m█████████████████\x1b[0m |    73,200 | 56.0% |       new\n2 | other-user     |     $1.10 \x1b[32m█████████████▍\x1b[0m    |    24,400 | 44.0% |       new\n\x1b[2m──|────────────────|─────────────────────────────|───────────|───────|──────────\x1b[0m\n\x1b[1;37m \x1b[0m | \x1b[1;37mTotal         \x1b[0m | \x1b[1;37m    $2.50\x1b[0m                   | \x1b[1;37m   97,600\x1b[0m | \x1b[1;37m     \x1b[0m | \x1b[1;37m         \x1b[0m\n\x1b[2mnever synced · tu sync to refresh\x1b[0m\n\n"

const e2eLbUntilOnly = "\n\x1b[1;37mLeaderboard (daily) · → 2026-01-31 · by cost\x1b[0m\n\n\x1b[1;36m#\x1b[0m | \x1b[1;36mUser          \x1b[0m | \x1b[1;36m     Cost\x1b[0m                   | \x1b[1;36m   Tokens\x1b[0m | \x1b[1;36mShare\x1b[0m | \x1b[1;36mΔ vs prev\x1b[0m\n\x1b[2m──|────────────────|─────────────────────────────|───────────|───────|──────────\x1b[0m\n1 | harness-user ◂ |     $1.70 \x1b[32m█████████████████\x1b[0m |    97,600 | 56.7% |       new\n2 | other-user     |     $1.30 \x1b[32m█████████████\x1b[0m     |    48,800 | 43.3% |       new\n\x1b[2m──|────────────────|─────────────────────────────|───────────|───────|──────────\x1b[0m\n\x1b[1;37m \x1b[0m | \x1b[1;37mTotal         \x1b[0m | \x1b[1;37m    $3.00\x1b[0m                   | \x1b[1;37m  146,400\x1b[0m | \x1b[1;37m     \x1b[0m | \x1b[1;37m         \x1b[0m\n\x1b[2mnever synced · tu sync to refresh\x1b[0m\n\n"

const e2eMLbh = "\n\x1b[1;37m📊 Leaderboard History (monthly)\x1b[0m\n\n\x1b[1;36mDate      \x1b[0m | \x1b[1;36mharness-user\x1b[0m | \x1b[1;36mother-user\x1b[0m | \x1b[1;36m     Cost\x1b[0m\n\x1b[2m───────────|──────────────|────────────|────────────────────────────────────────\x1b[0m\n2026-01    | \x1b[1;37m       $1.70\x1b[0m |      $1.30 |     $3.00 \x1b[32m████████████████\x1b[0m\x1b[35m█████████████\x1b[0m\n\n"

const e2eMLbhJSON = "{\n  \"harness-user\": [\n    {\n      \"label\": \"2026-01\",\n      \"totalCost\": 1.7,\n      \"inputTokens\": 12000,\n      \"outputTokens\": 1600,\n      \"cacheCreationTokens\": 4000,\n      \"cacheReadTokens\": 80000,\n      \"totalTokens\": 97600\n    }\n  ],\n  \"other-user\": [\n    {\n      \"label\": \"2026-01\",\n      \"totalCost\": 1.3,\n      \"inputTokens\": 6000,\n      \"outputTokens\": 800,\n      \"cacheCreationTokens\": 2000,\n      \"cacheReadTokens\": 40000,\n      \"totalTokens\": 48800\n    }\n  ]\n}\n"

const e2eMLbhCSV = "date,harness-user,other-user,total\n2026-01,1.70,1.30,3.00\n"

const e2eMLbhMD = "## Leaderboard History (monthly)\n\n| Date | harness-user | other-user | Cost |\n| :--- | ---: | ---: | ---: |\n| 2026-01 | $1.70 | $1.30 | $3.00 |\n\n"

const e2eMLbhTop1 = "\n\x1b[1;37m📊 Leaderboard History (monthly)\x1b[0m\n\n\x1b[1;36mDate      \x1b[0m | \x1b[1;36mharness-user\x1b[0m | \x1b[1;36m   others\x1b[0m | \x1b[1;36m     Cost\x1b[0m\n\x1b[2m───────────|──────────────|───────────|─────────────────────────────────────────\x1b[0m\n2026-01    | \x1b[1;37m       $1.70\x1b[0m |     $1.30 |     $3.00 \x1b[32m█████████████████\x1b[0m\x1b[35m█████████████\x1b[0m\n\n"

const e2eMLbhTop1JSON = "{\n  \"harness-user\": [\n    {\n      \"label\": \"2026-01\",\n      \"totalCost\": 1.7,\n      \"inputTokens\": 12000,\n      \"outputTokens\": 1600,\n      \"cacheCreationTokens\": 4000,\n      \"cacheReadTokens\": 80000,\n      \"totalTokens\": 97600\n    }\n  ],\n  \"others\": [\n    {\n      \"label\": \"2026-01\",\n      \"totalCost\": 1.3,\n      \"inputTokens\": 6000,\n      \"outputTokens\": 800,\n      \"cacheCreationTokens\": 2000,\n      \"cacheReadTokens\": 40000,\n      \"totalTokens\": 48800\n    }\n  ]\n}\n"

const e2eMLbhTokens = "\n\x1b[1;37m📊 Leaderboard Token History (monthly)\x1b[0m\n\n\x1b[1;36mDate      \x1b[0m | \x1b[1;36mharness-user\x1b[0m | \x1b[1;36mother-user\x1b[0m | \x1b[1;36m   Tokens\x1b[0m\n\x1b[2m───────────|──────────────|────────────|────────────────────────────────────────\x1b[0m\n2026-01    | \x1b[1;37m      97,600\x1b[0m |     48,800 |   146,400 \x1b[32m███████████████████\x1b[0m\x1b[35m██████████\x1b[0m\n\n"

const e2eLbhWindow = "\n\x1b[1;37m📊 Leaderboard History (daily)\x1b[0m\n\n\x1b[1;36mDate      \x1b[0m | \x1b[1;36mharness-user\x1b[0m | \x1b[1;36mother-user\x1b[0m | \x1b[1;36m     Cost\x1b[0m\n\x1b[2m───────────|──────────────|────────────|────────────────────────────────────────\x1b[0m\n2026-01-05 |        $0.25 | \x1b[1;37m     $1.10\x1b[0m |     $1.35 \x1b[32m█████\x1b[0m\x1b[35m████████████████████████\x1b[0m\n2026-01-06 | \x1b[1;37m       $1.15\x1b[0m |      $0.20 |     $1.35 \x1b[32m█████████████████████████\x1b[0m\x1b[35m████\x1b[0m\n2026-01-07 | \x1b[1;37m       $0.30\x1b[0m | \x1b[2m     $0.00\x1b[0m |     $0.30 \x1b[32m██████▌\x1b[0m\n\x1b[2m───────────|──────────────|────────────|────────────────────────────────────────\x1b[0m\n\x1b[1;37mTotal     \x1b[0m | \x1b[1;37m       $1.70\x1b[0m | \x1b[1;37m     $1.30\x1b[0m | \x1b[1;37m    $3.00\x1b[0m\n\x1b[2mavg $1.00/day · peak $1.35 (2026-01-05)\x1b[0m\x1b[2m · \x1b[0m\x1b[32m█\x1b[0m \x1b[2mharness-user\x1b[0m \x1b[35m█\x1b[0m \x1b[2mother-user\x1b[0m\n\n"

const e2eLbhByMachine = "\n\x1b[1;37m📊 Leaderboard History (daily, last 3 months)\x1b[0m\n\n  No data\n\n"

// The populated January window in every format (R5/R12; A-021).
func TestE2ELeaderboardWindow(t *testing.T) {
	stageVariant(t, "multi")
	window := []string{"lb", "--since", "2026-01-01", "--until", "2026-01-31"}
	assertRun(t, window, 0, e2eLbWindow, "")
	assertRun(t, append(append([]string{}, window...), "--json"), 0, e2eLbWindowJSON, "")
	assertRun(t, append(append([]string{}, window...), "--csv"), 0, e2eLbWindowCSV, "")
	assertRun(t, append(append([]string{}, window...), "--md"), 0, e2eLbWindowMD, "")
	// -t re-ranks by tokens (97,600 vs 48,800; shares in tokens).
	assertRun(t, append(append([]string{}, window...), "-t"), 0, e2eLbWindowTokens, "")
	// cc lb: Claude Code only ($1.40 / $1.10).
	assertRun(t, append([]string{"cc"}, window...), 0, e2eCCLbWindow, "")
}

// --by-machine keys rows by user/machine; every machine row of the pinned
// user carries ◂; the wider User column eats the bar budget (no bars at 80).
// Under -t the three keys tie at 48,800 tokens and rank by key name
// (harness-user/harness-machine, harness-user/other-box, other-user/laptop)
// (A-022).
func TestE2ELeaderboardByMachine(t *testing.T) {
	stageVariant(t, "multi")
	window := []string{"lb", "--since", "2026-01-01", "--until", "2026-01-31", "--by-machine"}
	assertRun(t, window, 0, e2eLbWindowByMachine, "")
	assertRun(t, append(append([]string{}, window...), "-t"), 0, e2eLbWindowByMachineTokens, "")
}

// --top 1 collapses the second user into the dim "… +1 others" line; the
// Total still sums the full set (A-022).
func TestE2ELeaderboardTop(t *testing.T) {
	stageVariant(t, "multi")
	assertRun(t, []string{"lb", "--since", "2026-01-01", "--until", "2026-01-31", "--top", "1"}, 0, e2eLbWindowTop1, "")
}

// An --until-only window heads "→ {until}" with a nil previous window (every
// row new, DC-13); -u <name> pins (◂) instead of filtering.
func TestE2ELeaderboardUntilOnlyAndPin(t *testing.T) {
	stageVariant(t, "multi")
	assertRun(t, []string{"lb", "--until", "2026-01-31"}, 0, e2eLbUntilOnly, "")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"lb", "--since", "2026-01-01", "--until", "2026-01-31", "-u", "other-user"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	got := stdout.String()
	if !strings.Contains(got, "other-user ◂") || strings.Contains(got, "harness-user ◂") {
		t.Errorf("-u other-user must pin other-user:\n%s", got)
	}
	if !strings.Contains(got, "harness-user") || !strings.Contains(got, "2 | other-user ◂") {
		t.Errorf("the pin must not filter rows:\n%s", got)
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
}

// The lbh surfaces: m lbh (ranked columns, the bold leader cell, stacked
// bars, no Total for one label), the formats, --top 1 (the others column,
// JSON keys with others LAST), -t, and the daily window (month separators
// rules, weekend dimming, the legend). lbh --by-machine warns and clears,
// then renders the capped empty state.
func TestE2ELeaderboardHistory(t *testing.T) {
	stageVariant(t, "multi")
	assertRun(t, []string{"m", "lbh"}, 0, e2eMLbh, "")
	assertRun(t, []string{"m", "lbh", "--json"}, 0, e2eMLbhJSON, "")
	assertRun(t, []string{"m", "lbh", "--csv"}, 0, e2eMLbhCSV, "")
	assertRun(t, []string{"m", "lbh", "--md"}, 0, e2eMLbhMD, "")
	assertRun(t, []string{"m", "lbh", "--top", "1"}, 0, e2eMLbhTop1, "")
	assertRun(t, []string{"m", "lbh", "--top", "1", "--json"}, 0, e2eMLbhTop1JSON, "")
	assertRun(t, []string{"m", "lbh", "-t"}, 0, e2eMLbhTokens, "")
	assertRun(t, []string{"lbh", "--since", "2026-01-01", "--until", "2026-01-31"}, 0, e2eLbhWindow, "")
	// --full lifts the cap: the daily seed rows without the heading hint —
	// byte-identical to the explicit window here (all seed days are January).
	assertRun(t, []string{"lbh", "--full"}, 0, e2eLbhWindow, "")
	assertRun(t, []string{"lbh", "--by-machine"}, 0, e2eLbhByMachine,
		"Warning: --by-machine is not supported with leaderboard history — ignoring.\n")
}

// R14: a staged .last-sync file drives the staleness footer ("15m ago" with a
// timestamp written fifteen and a half minutes before the run).
func TestE2ELeaderboardLastSync(t *testing.T) {
	home := stageVariant(t, "multi")
	iso := time.Now().Add(-15*time.Minute - 30*time.Second).UTC().Format("2006-01-02T15:04:05.000Z")
	if err := os.WriteFile(filepath.Join(home, ".tu", ".last-sync"), []byte(iso), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"lb"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	want := "\x1b[2msynced 15m ago (" + iso + ") · tu sync to refresh\x1b[0m"
	if !strings.Contains(stdout.String(), want) {
		t.Errorf("stdout missing the synced footer %q:\n%s", want, stdout.String())
	}
}

// ── B6: the sync writer — tu sync, --dry-run, and --sync on a data command ──
//
// Byte references: the node oracle (node v24.15.0) against the committed seed
// with the harness-staged homes, TZ=UTC, piped, captured 2026-09-17 through
// the StageOracle layout (the fake ccusage in the vendor slot, the fake git
// first on PATH). The commit-message date is computed, not pinned: both sides
// take today's UTC date.

// dayFileBytes is the pinned day-file line for the placeholder corpus's daily
// record (all six tools carry the same token counts) with the given label and
// totalCost, plus the trailing newline writeMetrics appends.
func dayFileBytes(label, cost string) string {
	return `{"label":"` + label + `","totalCost":` + cost + `,"inputTokens":3000,"outputTokens":400,"cacheCreationTokens":1000,"cacheReadTokens":20000,"totalTokens":24400}` + "\n"
}

// assertDayFile pins one day-file's bytes.
func assertDayFile(t *testing.T, path, label, cost string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if want := dayFileBytes(label, cost); string(got) != want {
		t.Errorf("%s = %q, want %q", path, got, want)
	}
}

// assertSyncedTree pins the post-sync state of a staged multi-variant home:
// the 17 written day-files (cc 01-05 updated to the live $0.50, cc 01-07 and
// the other five tools' 01-05..07 files new), the never-shrunk cc 01-06, and
// a .last-sync holding a JavaScript toISOString timestamp plus newline.
func assertSyncedTree(t *testing.T, home string) {
	t.Helper()
	machineDir := filepath.Join(home, ".tu", "metrics_repo", "harness-user", "2026", "harness-machine")
	for _, tool := range []string{"cc", "codex", "oc", "gemini", "copilot", "kimi"} {
		for _, day := range []string{"2026-01-05", "2026-01-06", "2026-01-07"} {
			if tool == "cc" && day == "2026-01-06" {
				continue // never-shrunk, asserted below
			}
			assertDayFile(t, filepath.Join(machineDir, tool+"-"+day+".jsonl"), day, "0.5")
		}
	}
	assertDayFile(t, filepath.Join(machineDir, "cc-2026-01-06.jsonl"), "2026-01-06", "0.75")
	raw, err := os.ReadFile(filepath.Join(home, ".tu", ".last-sync"))
	if err != nil {
		t.Fatalf("read .last-sync: %v", err)
	}
	if !strings.HasSuffix(string(raw), "\n") {
		t.Errorf(".last-sync = %q, want a trailing newline", raw)
	}
	if _, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(string(raw))); err != nil {
		t.Errorf(".last-sync = %q, not an RFC 3339 timestamp: %v", raw, err)
	}
}

// syncDryRunReport is the byte-exact `sync --dry-run` stdout on a staged multi
// home (node oracle, captured 2026-09-17); only the commit-message date moves.
func syncDryRunReport(now time.Time) string {
	return "Would write 17 day-file(s) under ~/.tu/metrics_repo/harness-user/:\n" +
		"  2026/harness-machine/cc-2026-01-05.jsonl  $0.50  (update: $0.25 → $0.50)\n" +
		"  2026/harness-machine/cc-2026-01-07.jsonl  $0.50  (new)\n" +
		"  2026/harness-machine/codex-2026-01-05.jsonl  $0.50  (new)\n" +
		"  2026/harness-machine/codex-2026-01-06.jsonl  $0.50  (new)\n" +
		"  2026/harness-machine/codex-2026-01-07.jsonl  $0.50  (new)\n" +
		"  2026/harness-machine/oc-2026-01-05.jsonl  $0.50  (new)\n" +
		"  2026/harness-machine/oc-2026-01-06.jsonl  $0.50  (new)\n" +
		"  2026/harness-machine/oc-2026-01-07.jsonl  $0.50  (new)\n" +
		"  2026/harness-machine/gemini-2026-01-05.jsonl  $0.50  (new)\n" +
		"  2026/harness-machine/gemini-2026-01-06.jsonl  $0.50  (new)\n" +
		"  2026/harness-machine/gemini-2026-01-07.jsonl  $0.50  (new)\n" +
		"  2026/harness-machine/copilot-2026-01-05.jsonl  $0.50  (new)\n" +
		"  2026/harness-machine/copilot-2026-01-06.jsonl  $0.50  (new)\n" +
		"  2026/harness-machine/copilot-2026-01-07.jsonl  $0.50  (new)\n" +
		"  2026/harness-machine/kimi-2026-01-05.jsonl  $0.50  (new)\n" +
		"  2026/harness-machine/kimi-2026-01-06.jsonl  $0.50  (new)\n" +
		"  2026/harness-machine/kimi-2026-01-07.jsonl  $0.50  (new)\n" +
		"Would skip 1 file(s) (never-shrink guard):\n" +
		"  2026/harness-machine/cc-2026-01-06.jsonl  incoming $0.50 < existing $0.75\n" +
		"Would commit: \"# harness-user: update " + now.UTC().Format("2006-01-02") + "\", then pull --rebase origin main, then push\n" +
		"Dry run — nothing written, committed, or pushed.\n"
}

// R10: `tu sync` in single mode is the two-line gate on stderr, empty stdout,
// exit 1, no git call — identically for --dry-run.
func TestE2ESyncSingleMode(t *testing.T) {
	stageVariant(t, "single")
	log := filepath.Join(t.TempDir(), "calls.jsonl")
	t.Setenv("TUDIFF_CALL_LOG", log)
	want := "tu sync requires metrics_repo to be set.\n" +
		"Add metrics_repo to ~/.config/tu/tu.conf, run 'tu init-metrics <repo-url>', or set TU_METRICS_REPO.\n"
	assertRun(t, []string{"sync"}, 1, "", want)
	assertRun(t, []string{"sync", "--dry-run"}, 1, "", want)
	if calls := gitCalls(t, log); len(calls) != 0 {
		t.Errorf("git calls = %v, want none (single mode)", calls)
	}
}

// R10: `sync --dry-run` on a multi home prints the report on stdout, exit 0,
// writes nothing (no day-file, no .last-sync, the seed untouched) and runs
// exactly one read-only `status --porcelain`.
func TestE2ESyncDryRun(t *testing.T) {
	home := stageVariant(t, "multi")
	log := filepath.Join(t.TempDir(), "calls.jsonl")
	t.Setenv("TUDIFF_CALL_LOG", log)
	assertRun(t, []string{"sync", "--dry-run"}, 0, syncDryRunReport(time.Now()), "")
	dir := filepath.Join(home, ".tu", "metrics_repo")
	want := [][]string{{"-C", dir, "status", "--porcelain", "harness-user/"}}
	if calls := gitCalls(t, log); !reflect.DeepEqual(calls, want) {
		t.Errorf("git calls = %v, want %v", calls, want)
	}
	newFile := filepath.Join(dir, "harness-user", "2026", "harness-machine", "cc-2026-01-07.jsonl")
	if _, err := os.Stat(newFile); !os.IsNotExist(err) {
		t.Error("dry-run created cc-2026-01-07.jsonl")
	}
	if _, err := os.Stat(filepath.Join(home, ".tu", ".last-sync")); !os.IsNotExist(err) {
		t.Error("dry-run created .last-sync")
	}
	// The seeded 01-05 file is byte-identical to the committed seed.
	got, err := os.ReadFile(filepath.Join(dir, "harness-user", "2026", "harness-machine", "cc-2026-01-05.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	seed, err := os.ReadFile(filepath.Join(e2eSeedDir, "harness-user", "2026", "harness-machine", "cc-2026-01-05.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, seed) {
		t.Errorf("dry-run mutated the seeded cc-2026-01-05.jsonl: %q", got)
	}
}

// R10: a live `tu sync` on each multi variant: the Synced line on stdout, the
// add/status/pull/push argv sequence (no commit — the fake git's status is
// clean), six ccusage calls, and the written tree. `sync --json` ignores the
// format flag (DC-02) and renders the same plain-text result.
func TestE2ESyncLive(t *testing.T) {
	for _, variant := range []string{"multi", "org", "legacy"} {
		t.Run(variant, func(t *testing.T) {
			home := stageVariant(t, variant)
			log := filepath.Join(t.TempDir(), "calls.jsonl")
			t.Setenv("TUDIFF_CALL_LOG", log)
			stderr := ""
			if variant == "legacy" {
				stderr = "tu: ~/.tu.conf is deprecated; move it to ~/.config/tu/tu.conf\n"
			}
			assertRun(t, []string{"sync"}, 0, "Synced to ~/.tu/metrics_repo\n", stderr)
			dir := filepath.Join(home, ".tu", "metrics_repo")
			want := [][]string{
				{"-C", dir, "add", "harness-user/"},
				{"-C", dir, "status", "--porcelain", "harness-user/"},
				{"-C", dir, "pull", "--rebase", "origin", "main"},
				{"-C", dir, "push"},
			}
			if calls := gitCalls(t, log); !reflect.DeepEqual(calls, want) {
				t.Errorf("git calls = %v, want %v", calls, want)
			}
			if calls := loggedCalls(t, log, "ccusage"); len(calls) != 6 {
				t.Errorf("ccusage calls = %v, want six (one per tool)", calls)
			}
			assertSyncedTree(t, home)
		})
	}
}

// R10/DC-02: data flags on `sync` are ignored — `sync --json` is a real
// plain-text sync, byte-identical to `sync`.
func TestE2ESyncJSONIgnored(t *testing.T) {
	stageVariant(t, "multi")
	assertRun(t, []string{"sync", "--json"}, 0, "Synced to ~/.tu/metrics_repo\n", "")
}

// R10: a dirty `status --porcelain` inserts the commit (today's UTC date)
// between status and pull.
func TestE2ESyncDirty(t *testing.T) {
	home := stageVariant(t, "multi")
	log := filepath.Join(t.TempDir(), "calls.jsonl")
	t.Setenv("TUDIFF_CALL_LOG", log)
	t.Setenv("TUDIFF_GIT_SCRIPT", `[{"match":["status","--porcelain"],"stdout":" M harness-user/x\n","exit":0}]`)
	assertRun(t, []string{"sync"}, 0, "Synced to ~/.tu/metrics_repo\n", "")
	dir := filepath.Join(home, ".tu", "metrics_repo")
	want := [][]string{
		{"-C", dir, "add", "harness-user/"},
		{"-C", dir, "status", "--porcelain", "harness-user/"},
		{"-C", dir, "commit", "-m", "# harness-user: update " + time.Now().UTC().Format("2006-01-02")},
		{"-C", dir, "pull", "--rebase", "origin", "main"},
		{"-C", dir, "push"},
	}
	if calls := gitCalls(t, log); !reflect.DeepEqual(calls, want) {
		t.Errorf("git calls = %v, want %v", calls, want)
	}
}

// R10: a pull failure warns (the Node-shaped error text, its embedded newline
// producing the blank line), then the generic error, exit 1; the log shows
// the rebase --abort recovery after the pull, and no .last-sync is touched.
func TestE2ESyncPullFail(t *testing.T) {
	home := stageVariant(t, "multi")
	log := filepath.Join(t.TempDir(), "calls.jsonl")
	t.Setenv("TUDIFF_CALL_LOG", log)
	t.Setenv("TUDIFF_GIT_SCRIPT", `[{"match":["pull"],"stderr":"fatal: couldn't find remote ref main\n","exit":1}]`)
	dir := filepath.Join(home, ".tu", "metrics_repo")
	wantStderr := "Warning: sync pull failed — git -C " + dir + "... failed: Command failed: git -C " + dir + " pull --rebase origin main\n" +
		"fatal: couldn't find remote ref main\n" +
		"\n" +
		"Error: sync failed — check network and remote config.\n"
	assertRun(t, []string{"sync"}, 1, "", wantStderr)
	want := [][]string{
		{"-C", dir, "add", "harness-user/"},
		{"-C", dir, "status", "--porcelain", "harness-user/"},
		{"-C", dir, "pull", "--rebase", "origin", "main"},
		{"-C", dir, "rebase", "--abort"},
	}
	if calls := gitCalls(t, log); !reflect.DeepEqual(calls, want) {
		t.Errorf("git calls = %v, want %v", calls, want)
	}
	if _, err := os.Stat(filepath.Join(home, ".tu", ".last-sync")); !os.IsNotExist(err) {
		t.Error(".last-sync created by a failed sync")
	}
}

// R10: a push failure retries once (two push calls), then warns and exits 1
// with the generic error.
func TestE2ESyncPushFail(t *testing.T) {
	home := stageVariant(t, "multi")
	log := filepath.Join(t.TempDir(), "calls.jsonl")
	t.Setenv("TUDIFF_CALL_LOG", log)
	t.Setenv("TUDIFF_GIT_SCRIPT", `[{"match":["push"],"stderr":"error: failed to push some refs\n","exit":1}]`)
	dir := filepath.Join(home, ".tu", "metrics_repo")
	wantStderr := "Warning: sync push failed after retry — git -C " + dir + "... failed: Command failed: git -C " + dir + " push\n" +
		"error: failed to push some refs\n" +
		"\n" +
		"Error: sync failed — check network and remote config.\n"
	assertRun(t, []string{"sync"}, 1, "", wantStderr)
	want := [][]string{
		{"-C", dir, "add", "harness-user/"},
		{"-C", dir, "status", "--porcelain", "harness-user/"},
		{"-C", dir, "pull", "--rebase", "origin", "main"},
		{"-C", dir, "push"},
		{"-C", dir, "push"},
	}
	if calls := gitCalls(t, log); !reflect.DeepEqual(calls, want) {
		t.Errorf("git calls = %v, want %v", calls, want)
	}
}

// R10: a config user of "all" is the reserved-user line and exit 2 — BEFORE
// the mode check, so no sync runs.
func TestE2ESyncReservedUser(t *testing.T) {
	home := stageVariant(t, "multi")
	conf := filepath.Join(home, ".config", "tu", "tu.conf")
	raw, err := os.ReadFile(conf)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(conf, bytes.ReplaceAll(raw, []byte("user = harness-user"), []byte("user = all")), 0o644); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(t.TempDir(), "calls.jsonl")
	t.Setenv("TUDIFF_CALL_LOG", log)
	assertRun(t, []string{"sync"}, 2, "", `Error: config user "all" is reserved (used by -u all)`+"\n")
	if calls := gitCalls(t, log); len(calls) != 0 {
		t.Errorf("git calls = %v, want none (reserved user)", calls)
	}
}

// R10: a fresh .clone-failed marker with the metrics dir missing demotes in
// the guard — the not-available warning only, exit 1, no git call.
func TestE2ESyncCloneMarker(t *testing.T) {
	home := stageVariant(t, "multi")
	if err := os.RemoveAll(filepath.Join(home, ".tu", "metrics_repo")); err != nil {
		t.Fatal(err)
	}
	marker := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	if err := os.WriteFile(filepath.Join(home, ".tu", ".clone-failed"), []byte(marker), 0o644); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(t.TempDir(), "calls.jsonl")
	t.Setenv("TUDIFF_CALL_LOG", log)
	assertRun(t, []string{"sync"}, 1, "",
		"Warning: metrics repo not available — falling back to single mode.\n")
	if calls := gitCalls(t, log); len(calls) != 0 {
		t.Errorf("git calls = %v, want none (fresh marker)", calls)
	}
}

// R9: `cc --sync` on a multi home syncs first (stderr exactly the two-part
// line), then renders the same bytes as `cc` without the flag; the run's own
// fetch hits the cache the sync warmed, so the log holds exactly SIX ccusage
// calls plus the four git calls, and the tree + .last-sync are written.
func TestE2ESyncFlagMulti(t *testing.T) {
	home := stageVariant(t, "multi")
	log := filepath.Join(t.TempDir(), "calls.jsonl")
	t.Setenv("TUDIFF_CALL_LOG", log)
	assertRun(t, []string{"cc", "--sync"}, 0,
		"\n\x1b[1;37m📊 Combined Usage (daily)\x1b[0m\n\n  No usage\n\n",
		"syncing metrics... synced.\n")
	if calls := loggedCalls(t, log, "ccusage"); len(calls) != 6 {
		t.Errorf("ccusage calls = %v, want six (the second fetch hits the cache)", calls)
	}
	dir := filepath.Join(home, ".tu", "metrics_repo")
	want := [][]string{
		{"-C", dir, "add", "harness-user/"},
		{"-C", dir, "status", "--porcelain", "harness-user/"},
		{"-C", dir, "pull", "--rebase", "origin", "main"},
		{"-C", dir, "push"},
	}
	if calls := gitCalls(t, log); !reflect.DeepEqual(calls, want) {
		t.Errorf("git calls = %v, want %v", calls, want)
	}
	assertSyncedTree(t, home)
}

// R9: `cc --sync` with a failing pull warns between the prefix and the
// failure line (the error's embedded newline making the blank line), then the
// table still renders from local data, exit 0, and .last-sync is untouched.
func TestE2ESyncFlagPullFail(t *testing.T) {
	home := stageVariant(t, "multi")
	log := filepath.Join(t.TempDir(), "calls.jsonl")
	t.Setenv("TUDIFF_CALL_LOG", log)
	t.Setenv("TUDIFF_GIT_SCRIPT", `[{"match":["pull"],"stderr":"fatal: couldn't find remote ref main\n","exit":1}]`)
	dir := filepath.Join(home, ".tu", "metrics_repo")
	wantStderr := "syncing metrics... Warning: sync pull failed — git -C " + dir + "... failed: Command failed: git -C " + dir + " pull --rebase origin main\n" +
		"fatal: couldn't find remote ref main\n" +
		"\n" +
		"sync failed — using local data.\n"
	assertRun(t, []string{"cc", "--sync"}, 0,
		"\n\x1b[1;37m📊 Combined Usage (daily)\x1b[0m\n\n  No usage\n\n", wantStderr)
	want := [][]string{
		{"-C", dir, "add", "harness-user/"},
		{"-C", dir, "status", "--porcelain", "harness-user/"},
		{"-C", dir, "pull", "--rebase", "origin", "main"},
		{"-C", dir, "rebase", "--abort"},
	}
	if calls := gitCalls(t, log); !reflect.DeepEqual(calls, want) {
		t.Errorf("git calls = %v, want %v", calls, want)
	}
	if _, err := os.Stat(filepath.Join(home, ".tu", ".last-sync")); !os.IsNotExist(err) {
		t.Error(".last-sync created by a failed sync")
	}
}

// R9: --sync in single mode produces no sync line and no git call — the
// normal live render only (one ccusage call for the single source).
func TestE2ESyncFlagSingle(t *testing.T) {
	stageVariant(t, "single")
	log := filepath.Join(t.TempDir(), "calls.jsonl")
	t.Setenv("TUDIFF_CALL_LOG", log)
	assertRun(t, []string{"cc", "--sync"}, 0,
		"\n\x1b[1;37m📊 Combined Usage (daily)\x1b[0m\n\n  No usage\n\n", "")
	if calls := loggedCalls(t, log, "ccusage"); len(calls) != 1 {
		t.Errorf("ccusage calls = %v, want one (single source, no sync fetch)", calls)
	}
	if calls := gitCalls(t, log); len(calls) != 0 {
		t.Errorf("git calls = %v, want none (single mode)", calls)
	}
}
