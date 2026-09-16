package sync

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// writeFakeGit writes a shell-script git into dir (placed first on PATH by
// the caller): it appends its argv to $FAKEGIT_LOG (one arg per line, a "--"
// line per invocation), prints $FAKEGIT_STDOUT/$FAKEGIT_STDERR, and exits
// ${FAKEGIT_EXIT:-0}. The package test must not depend on a build step in
// another command (plan assumption 3); the e2e test uses the real fakegit.
func writeFakeGit(t *testing.T, dir string) {
	t.Helper()
	script := `#!/bin/sh
{
  for a in "$@"; do printf '%s\n' "$a"; done
  printf -- '--\n'
} >> "$FAKEGIT_LOG"
printf '%s' "$FAKEGIT_STDOUT"
printf '%s' "$FAKEGIT_STDERR" >&2
exit "${FAKEGIT_EXIT:-0}"
`
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

// readGitLog parses the fake's log into one argv slice per invocation.
func readGitLog(t *testing.T, path string) [][]string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var calls [][]string
	var cur []string
	for _, line := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n") {
		if line == "--" {
			calls = append(calls, cur)
			cur = nil
			continue
		}
		cur = append(cur, line)
	}
	return calls
}

// stageFakeGit puts the fake git first on PATH and returns the log path.
func stageFakeGit(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFakeGit(t, dir)
	log := filepath.Join(dir, "calls.log")
	t.Setenv("FAKEGIT_LOG", log)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return log
}

// R8: IsRepo issues exactly `-C <dir> rev-parse --git-dir` and reports exit 0.
func TestIsRepoArgv(t *testing.T) {
	log := stageFakeGit(t)
	if !(Exec{}).IsRepo("/x") {
		t.Error("IsRepo = false, want true (exit 0)")
	}
	calls := readGitLog(t, log)
	if len(calls) != 1 || strings.Join(calls[0], " ") != "-C /x rev-parse --git-dir" {
		t.Errorf("calls = %v, want [[-C /x rev-parse --git-dir]]", calls)
	}
}

func TestIsRepoFalse(t *testing.T) {
	stageFakeGit(t)
	t.Setenv("FAKEGIT_EXIT", "1")
	if (Exec{}).IsRepo("/x") {
		t.Error("IsRepo = true, want false (exit 1)")
	}
}

// R8: Clone issues exactly `clone <url> <dir>` and passes the child's output
// through the given writers.
func TestClonePassthrough(t *testing.T) {
	log := stageFakeGit(t)
	t.Setenv("FAKEGIT_STDOUT", "clone-out")
	t.Setenv("FAKEGIT_STDERR", "clone-err")
	var stdout, stderr bytes.Buffer
	if err := (Exec{}).Clone(context.Background(), "u", "/y", nil, &stdout, &stderr); err != nil {
		t.Fatalf("Clone err = %v", err)
	}
	if stdout.String() != "clone-out" || stderr.String() != "clone-err" {
		t.Errorf("stdout = %q, stderr = %q", stdout.String(), stderr.String())
	}
	calls := readGitLog(t, log)
	if len(calls) != 1 || strings.Join(calls[0], " ") != "clone u /y" {
		t.Errorf("calls = %v, want [[clone u /y]]", calls)
	}
}

// R8: a non-zero clone exit is an *exec.ExitError with the child's code, and
// the child's stderr still reaches the writer.
func TestCloneFailure(t *testing.T) {
	stageFakeGit(t)
	t.Setenv("FAKEGIT_EXIT", "128")
	t.Setenv("FAKEGIT_STDERR", "boom")
	var stdout, stderr bytes.Buffer
	err := (Exec{}).Clone(context.Background(), "u", "/y", nil, &stdout, &stderr)
	var exitErr *exec.ExitError
	if err == nil || !errors.As(err, &exitErr) {
		t.Fatalf("err = %v, want *exec.ExitError", err)
	}
	if exitErr.ExitCode() != 128 {
		t.Errorf("exit code = %d, want 128", exitErr.ExitCode())
	}
	if stderr.String() != "boom" {
		t.Errorf("stderr = %q, want %q", stderr.String(), "boom")
	}
}
