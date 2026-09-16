package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/sahil87/tu/internal/command"
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
