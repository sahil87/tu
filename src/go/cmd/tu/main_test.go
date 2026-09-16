package main

import (
	"bytes"
	"regexp"
	"testing"

	"github.com/sahil87/tu/internal/command"
)

// Same shape the TypeScript pinning test enforces (src/node/core/__tests__/cli-version.test.ts)
// and the toolkit `version` standard recommends: `<tool> version vX.Y.Z`.
var versionLineRE = regexp.MustCompile(`^tu version v\d+(\.\d+)*$`)

func TestVersionLine(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"v0.11.5", "tu version v0.11.5"},
		{"0.11.5", "tu version v0.11.5"},
		{"dev", "tu version dev"},
		{"", "tu version "},
	}
	for _, c := range cases {
		if got := versionLine(c.in); got != c.want {
			t.Errorf("versionLine(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRunVersionFlags(t *testing.T) {
	orig := version
	version = "v1.2.3"
	t.Cleanup(func() { version = orig })

	for _, args := range [][]string{{"--version"}, {"-V"}, {"-v"}, {"cc", "--version"}} {
		var stdout, stderr bytes.Buffer
		code := run(args, &stdout, &stderr)
		if code != 0 {
			t.Errorf("run(%q) exit = %d, want 0", args, code)
		}
		if got := stdout.String(); got != "tu version v1.2.3\n" {
			t.Errorf("run(%q) stdout = %q, want %q", args, got, "tu version v1.2.3\n")
		}
		if stderr.Len() != 0 {
			t.Errorf("run(%q) stderr = %q, want empty", args, stderr.String())
		}
		first := firstNonEmptyLine(stdout.String())
		if !versionLineRE.MatchString(first) {
			t.Errorf("run(%q) first stdout line %q does not match %s", args, first, versionLineRE)
		}
	}
}

func TestRunDevFallback(t *testing.T) {
	orig := version
	version = "dev"
	t.Cleanup(func() { version = orig })

	var stdout, stderr bytes.Buffer
	if code := run([]string{"--version"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if got := stdout.String(); got != "tu version dev\n" {
		t.Errorf("stdout = %q, want %q", got, "tu version dev\n")
	}
}

// TestRunNotImplemented covers the recognized-but-unported surface: non-data
// commands and unported flags keep the scaffold's placeholder (stderr, exit
// 1). {} and {"cc"} left this list in V2; {"m","dh","--json"} and {"--csv"}
// left it in B2 (history + the csv/md encoders) — they now fetch and render
// (covered end-to-end in e2e_test.go). {"h","--by-machine"} pins that B4's
// flag still routes to the placeholder.
func TestRunNotImplemented(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"h", "--by-machine"}} {
		var stdout, stderr bytes.Buffer
		code := run(args, &stdout, &stderr)
		if code != 1 {
			t.Errorf("run(%q) exit = %d, want 1", args, code)
		}
		if stdout.Len() != 0 {
			t.Errorf("run(%q) stdout = %q, want empty", args, stdout.String())
		}
		if got := stderr.String(); got != notImplementedMsg+"\n" {
			t.Errorf("run(%q) stderr = %q, want %q", args, got, notImplementedMsg+"\n")
		}
	}
}

// TestRunVersionAfterValidation pins the TS order: flag validation runs
// before the version check, so a conflicting argv is a usage error, not a
// version line.
func TestRunVersionAfterValidation(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--json", "--csv", "--version"}, &stdout, &stderr)
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	if got := stderr.String(); got != "Error: --json and --csv are incompatible\n" {
		t.Errorf("stderr = %q", got)
	}
}

// TestRunDryRunMisuse pins the --dry-run misuse guard (B1's row of the
// exit-code table): byte-exact message, no usage block, exit 2 — and it fires
// before $HOME is consulted.
func TestRunDryRunMisuse(t *testing.T) {
	t.Setenv("HOME", "")
	for _, args := range [][]string{{"--dry-run"}, {"cc", "--dry-run"}, {"status", "--dry-run"}} {
		var stdout, stderr bytes.Buffer
		code := run(args, &stdout, &stderr)
		if code != 2 {
			t.Errorf("run(%q) exit = %d, want 2", args, code)
		}
		if stdout.Len() != 0 {
			t.Errorf("run(%q) stdout = %q, want empty", args, stdout.String())
		}
		want := "Error: --dry-run is supported only with 'tu sync' — run 'tu sync --dry-run' to preview a sync.\n"
		if got := stderr.String(); got != want {
			t.Errorf("run(%q) stderr = %q, want %q", args, got, want)
		}
	}
}

// TestRunInitMetricsArity pins the arity usage error: the message, then
// ShortUsage, exit 2 — before $HOME is consulted (the TS checks arity at
// dispatch).
func TestRunInitMetricsArity(t *testing.T) {
	t.Setenv("HOME", "")
	var stdout, stderr bytes.Buffer
	code := run([]string{"init-metrics", "a", "b"}, &stdout, &stderr)
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	want := "Error: init-metrics takes at most one argument (repo-url)\n" + command.ShortUsage + "\n"
	if got := stderr.String(); got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
}

// TestRunNoHome pins the config-home failure: exit 1 with the byte-exact
// message, but only after a successful parse — a usage error with HOME unset
// still exits 2. The setup commands hit the same error (arity precedes it).
func TestRunNoHome(t *testing.T) {
	t.Setenv("HOME", "")

	var stdout, stderr bytes.Buffer
	if code := run(nil, &stdout, &stderr); code != 1 {
		t.Errorf("run(nil) exit = %d, want 1", code)
	}
	if got := stderr.String(); got != "tu: $HOME is not set; cannot locate config\n" {
		t.Errorf("run(nil) stderr = %q", got)
	}
	if stdout.Len() != 0 {
		t.Errorf("run(nil) stdout = %q, want empty", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"bogus"}, &stdout, &stderr); code != 2 {
		t.Errorf("run(bogus) exit = %d, want 2 (parse precedes the HOME check)", code)
	}
	if got := stderr.String(); got != "Unknown argument: bogus\n"+command.ShortUsage+"\n" {
		t.Errorf("run(bogus) stderr = %q", got)
	}
}

func firstNonEmptyLine(s string) string {
	for _, line := range bytes.Split([]byte(s), []byte("\n")) {
		if len(bytes.TrimSpace(line)) > 0 {
			return string(line)
		}
	}
	return ""
}

// terminalWidth falls back to 80 for a non-*os.File writer (the e2e suite's
// bytes.Buffer) — R19; COLUMNS is never read.
func TestTerminalWidthFallback(t *testing.T) {
	t.Setenv("COLUMNS", "199")
	var buf bytes.Buffer
	if got := terminalWidth(&buf); got != 80 {
		t.Errorf("terminalWidth(bytes.Buffer) = %d, want 80", got)
	}
}
