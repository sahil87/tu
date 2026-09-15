package main

import (
	"bytes"
	"github.com/sahil87/tu/internal/harness"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// R4: the P4 stub exits 1 with the exact stderr line.
func TestRunStub(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"run"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if got := stderr.String(); got != "tudiff: run is not implemented (plan row P4)\n" {
		t.Errorf("stderr = %q", got)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
}

// R4: no argument and an unknown subcommand exit 2 with usage on stderr.
func TestRunUsageErrors(t *testing.T) {
	for _, args := range [][]string{{}, {"bogus"}} {
		var stdout, stderr bytes.Buffer
		code := run(args, &stdout, &stderr)
		if code != 2 {
			t.Errorf("run(%q) exit = %d, want 2", args, code)
		}
		if !strings.Contains(stderr.String(), "usage: tudiff") {
			t.Errorf("run(%q) stderr lacks usage: %q", args, stderr.String())
		}
		if stdout.Len() != 0 {
			t.Errorf("run(%q) stdout = %q, want empty", args, stdout.String())
		}
	}
}

// R5: an explicit --ccusage path that does not exist is an error (exit 1).
func TestCaptureMissingCcusage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"capture", "--ccusage", "/no/such/ccusage", "--out", t.TempDir()}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "ccusage binary not found") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

// Capture without --ccusage in a directory with no resolvable binary errors
// with the resolution message.
func TestCaptureNoBinaryFound(t *testing.T) {
	// Run from a temp dir as repo root (package.json present, no ccusage
	// anywhere, PATH empty so LookPath fails too).
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	t.Setenv("PATH", filepath.Join(root, "empty-bin"))

	var stdout, stderr bytes.Buffer
	code := run([]string{"capture", "--out", filepath.Join(root, "fixtures")}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if got := stderr.String(); !strings.Contains(got, "tudiff: no ccusage binary found (run npm ci or pass --ccusage)") {
		t.Errorf("stderr = %q", got)
	}
}

func TestPlaceholderDefaultsToAllSources(t *testing.T) {
	out := filepath.Join(t.TempDir(), "_placeholder")
	var stdout, stderr bytes.Buffer
	code := run([]string{"placeholder", "--out", out}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	m, err := harness.ReadManifest(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Fixtures) != len(harness.DefaultSources) {
		t.Errorf("manifest has %d fixtures, want %d (all default sources)", len(m.Fixtures), len(harness.DefaultSources))
	}
}

func TestPlaceholderWrites(t *testing.T) {
	out := filepath.Join(t.TempDir(), "_placeholder")
	var stdout, stderr bytes.Buffer
	code := run([]string{"placeholder", "--source", "opencode", "--source", "copilot", "--out", out}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	for _, source := range []string{"opencode", "copilot"} {
		if _, err := os.Stat(filepath.Join(out, source, "daily.json")); err != nil {
			t.Errorf("missing %s fixture: %v", source, err)
		}
	}
}
