package main

import (
	"bytes"
	"regexp"
	"testing"
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

func TestRunNotImplemented(t *testing.T) {
	for _, args := range [][]string{{}, {"cc"}, {"--help"}, {"m", "dh", "--json"}} {
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

func firstNonEmptyLine(s string) string {
	for _, line := range bytes.Split([]byte(s), []byte("\n")) {
		if len(bytes.TrimSpace(line)) > 0 {
			return string(line)
		}
	}
	return ""
}
