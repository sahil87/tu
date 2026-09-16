package main

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/sahil87/tu/internal/command"
	"github.com/sahil87/tu/internal/toolkit"
)

// Same shape the TypeScript pinning test enforces (src/node/core/__tests__/cli-version.test.ts)
// and the toolkit `version` standard recommends: `<tool> version vX.Y.Z`.
var versionLineRE = regexp.MustCompile(`^tu version v\d+(\.\d+)*$`)

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
// (covered end-to-end in e2e_test.go). {"--help"} left it in B8 (the toolkit
// layer); {"sync"} pins that B6's command still routes to the placeholder, as
// does {"h","--by-machine"} for B4's flag.
func TestRunNotImplemented(t *testing.T) {
	for _, args := range [][]string{{"sync"}, {"h", "--by-machine"}} {
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

// TestRunHelp covers the four help argv shapes (the help check precedes the
// --dry-run guard, so "help --dry-run" parses as help). None of them touches
// $HOME — R9 runs them with HOME unset.
func TestRunHelp(t *testing.T) {
	t.Setenv("HOME", "")
	for _, args := range [][]string{{"help"}, {"-h"}, {"--help"}, {"help", "--dry-run"}} {
		var stdout, stderr bytes.Buffer
		code := run(args, &stdout, &stderr)
		if code != 0 {
			t.Errorf("run(%q) exit = %d, want 0", args, code)
		}
		if got, want := stdout.String(), command.FullHelp+"\n"; got != want {
			t.Errorf("run(%q) stdout = %q, want FullHelp+newline", args, got[:min(60, len(got))])
		}
		if stderr.Len() != 0 {
			t.Errorf("run(%q) stderr = %q, want empty", args, stderr.String())
		}
	}
}

// TestRunHelpDump pins the help-dump envelope: valid JSON, the bare version
// (the stamp carries a leading v), the help text's `<date>` raw (never
// HTML-escaped), stderr empty, exit 0 — with HOME unset.
func TestRunHelpDump(t *testing.T) {
	t.Setenv("HOME", "")
	orig := version
	version = "v1.2.3"
	t.Cleanup(func() { version = orig })

	var stdout, stderr bytes.Buffer
	if code := run([]string{"help-dump"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
	var doc struct {
		Tool          string `json:"tool"`
		Version       string `json:"version"`
		SchemaVersion int    `json:"schema_version"`
		Root          struct {
			Text string `json:"text"`
		} `json:"root"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("help-dump output is not valid JSON: %v", err)
	}
	if doc.Tool != "tu" || doc.SchemaVersion != 1 {
		t.Errorf("envelope tool/schema_version = %q/%d", doc.Tool, doc.SchemaVersion)
	}
	if doc.Version != "1.2.3" {
		t.Errorf("version = %q, want bare %q", doc.Version, "1.2.3")
	}
	if !strings.Contains(doc.Root.Text, "<date>") {
		t.Error("root.text lost the raw <date> (HTML escaping?)")
	}
	if doc.Root.Text != command.FullHelp+"\n" {
		t.Error("root.text is not FullHelp + newline verbatim")
	}
}

// TestRunSkill pins `tu skill` (any args — the TS ignores them) to the
// embedded bundle, stderr empty, exit 0, HOME unset.
func TestRunSkill(t *testing.T) {
	t.Setenv("HOME", "")
	for _, args := range [][]string{{"skill"}, {"skill", "topics"}} {
		var stdout, stderr bytes.Buffer
		code := run(args, &stdout, &stderr)
		if code != 0 {
			t.Errorf("run(%q) exit = %d, want 0", args, code)
		}
		if !bytes.Equal(stdout.Bytes(), toolkit.Skill) {
			t.Errorf("run(%q) stdout != toolkit.Skill", args)
		}
		if stderr.Len() != 0 {
			t.Errorf("run(%q) stderr = %q, want empty", args, stderr.String())
		}
	}
}

// TestRunShellInit covers the five harness rows plus ignored extra args: the
// three scripts byte-equal the embedded completions with no added newline;
// missing/unknown shell write the exact stderr lines, leave stdout empty, and
// exit 2. HOME unset throughout.
func TestRunShellInit(t *testing.T) {
	t.Setenv("HOME", "")
	for _, shell := range []string{"bash", "zsh", "fish"} {
		for _, args := range [][]string{{"shell-init", shell}, {"shell-init", shell, "extra"}} {
			var stdout, stderr bytes.Buffer
			code := run(args, &stdout, &stderr)
			if code != 0 {
				t.Errorf("run(%q) exit = %d, want 0", args, code)
			}
			script, _ := toolkit.Completion(shell)
			if !bytes.Equal(stdout.Bytes(), script) {
				t.Errorf("run(%q) stdout != embedded %s script", args, shell)
			}
			if stderr.Len() != 0 {
				t.Errorf("run(%q) stderr = %q, want empty", args, stderr.String())
			}
		}
	}

	var stdout, stderr bytes.Buffer
	if code := run([]string{"shell-init"}, &stdout, &stderr); code != 2 {
		t.Errorf("run(shell-init) exit = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("run(shell-init) stdout = %q, want empty", stdout.String())
	}
	if got, want := stderr.String(), toolkit.ShellInitUsage+"\n"; got != want {
		t.Errorf("run(shell-init) stderr = %q, want the usage block", got)
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"shell-init", "tcsh"}, &stdout, &stderr); code != 2 {
		t.Errorf("run(shell-init tcsh) exit = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("run(shell-init tcsh) stdout = %q, want empty", stdout.String())
	}
	if got, want := stderr.String(), "Unknown shell: tcsh. Supported: bash, zsh, fish\n"; got != want {
		t.Errorf("run(shell-init tcsh) stderr = %q, want %q", got, want)
	}
}

// TestRunUpdateHelp pins the flag-discovery probe: --help/-h anywhere in the
// update args prints FullHelp and exits 0 without touching brew — with HOME
// unset.
func TestRunUpdateHelp(t *testing.T) {
	t.Setenv("HOME", "")
	for _, args := range [][]string{{"update", "--help"}, {"update", "-h"}, {"update", "--skip-brew-update", "-h"}} {
		var stdout, stderr bytes.Buffer
		code := run(args, &stdout, &stderr)
		if code != 0 {
			t.Errorf("run(%q) exit = %d, want 0", args, code)
		}
		if got, want := stdout.String(), command.FullHelp+"\n"; got != want {
			t.Errorf("run(%q) stdout = %q, want FullHelp+newline", args, got[:min(60, len(got))])
		}
		if stderr.Len() != 0 {
			t.Errorf("run(%q) stderr = %q, want empty", args, stderr.String())
		}
	}
}

// TestRunUpdateOffHomebrew pins the deterministic gate path: the go test
// binary never resolves under /Cellar/tu/, so `update` prints the two
// off-Homebrew lines and exits 0 without touching brew.
func TestRunUpdateOffHomebrew(t *testing.T) {
	orig := version
	version = "v1.2.3"
	t.Cleanup(func() { version = orig })

	var stdout, stderr bytes.Buffer
	code := run([]string{"update"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	want := "tu v1.2.3 was not installed via Homebrew.\n" +
		"Update manually, or reinstall with: brew install sahil87/tap/tu\n"
	if got := stdout.String(); got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty", stderr.String())
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
