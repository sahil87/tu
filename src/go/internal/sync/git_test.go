package sync

import (
	"bytes"
	"context"
	"errors"
	"io"
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

// R4: Run returns stdout on exit 0 and issues exactly `-C <dir> <args...>`.
func TestRunSuccess(t *testing.T) {
	log := stageFakeGit(t)
	t.Setenv("FAKEGIT_STDOUT", "stdout-text")
	stdout, err := (Exec{}).Run("/r", "pull", "--rebase", "origin", "main")
	if err != nil {
		t.Fatalf("Run err = %v", err)
	}
	if stdout != "stdout-text" {
		t.Errorf("stdout = %q, want %q", stdout, "stdout-text")
	}
	calls := readGitLog(t, log)
	if len(calls) != 1 || strings.Join(calls[0], " ") != "-C /r pull --rebase origin main" {
		t.Errorf("calls = %v, want [[-C /r pull --rebase origin main]]", calls)
	}
}

// R4: a non-zero exit reproduces Node's "Command failed: <cmd>\n<stderr>"
// message, the captured stderr verbatim (its trailing newline included).
func TestRunFailureWithStderr(t *testing.T) {
	stageFakeGit(t)
	t.Setenv("FAKEGIT_EXIT", "1")
	t.Setenv("FAKEGIT_STDERR", "fatal: couldn't find remote ref main\n")
	_, err := (Exec{}).Run("/r", "pull", "--rebase", "origin", "main")
	want := "git -C /r... failed: Command failed: git -C /r pull --rebase origin main\nfatal: couldn't find remote ref main\n"
	if err == nil || err.Error() != want {
		t.Errorf("err = %v, want %q", err, want)
	}
}

// R4: with empty stderr Node's message has no trailing newline (the `\n`
// separator is added only when stderr is non-empty).
func TestRunFailureNoStderr(t *testing.T) {
	stageFakeGit(t)
	t.Setenv("FAKEGIT_EXIT", "1")
	_, err := (Exec{}).Run("/r", "status", "--porcelain")
	want := "git -C /r... failed: Command failed: git -C /r status --porcelain\n"
	if err == nil || err.Error() != want {
		t.Errorf("err = %v, want %q", err, want)
	}
}

// R4: git missing from PATH reproduces Node's spawn message.
func TestRunMissingBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := (Exec{}).Run("/r", "status")
	want := "git -C /r... failed: spawn git ENOENT"
	if err == nil || err.Error() != want {
		t.Errorf("err = %v, want %q", err, want)
	}
}

// A stream overflowing MaxBuffer kills the child and yields Node's maxBuffer
// message (pinned against Node v24's execFile) — here on stdout with the
// child exiting 0, matching Node erroring on the overflow regardless.
func TestRunStdoutMaxBufferExceeded(t *testing.T) {
	stageFakeGit(t)
	t.Setenv("FAKEGIT_STDOUT", strings.Repeat("x", 32))
	_, err := (Exec{MaxBuffer: 16}).Run("/r", "log")
	want := "git -C /r... failed: stdout maxBuffer length exceeded"
	if err == nil || err.Error() != want {
		t.Errorf("err = %v, want %q", err, want)
	}
}

// Same on stderr, on a non-zero exit: the overflow message replaces the
// Command failed text, as in Node.
func TestRunStderrMaxBufferExceeded(t *testing.T) {
	stageFakeGit(t)
	t.Setenv("FAKEGIT_EXIT", "1")
	t.Setenv("FAKEGIT_STDERR", strings.Repeat("e", 32))
	_, err := (Exec{MaxBuffer: 16}).Run("/r", "push")
	want := "git -C /r... failed: stderr maxBuffer length exceeded"
	if err == nil || err.Error() != want {
		t.Errorf("err = %v, want %q", err, want)
	}
}

// Output at exactly the cap is not an overflow; the default cap is the sync
// 10 MiB (MaxBufferSync), the repair twin's is MaxBufferRepair.
func TestRunMaxBufferBoundaries(t *testing.T) {
	stageFakeGit(t)
	t.Setenv("FAKEGIT_STDOUT", strings.Repeat("x", 16))
	stdout, err := (Exec{MaxBuffer: 16}).Run("/r", "status", "--porcelain")
	if err != nil {
		t.Fatalf("Run err = %v, want nil at exactly the cap", err)
	}
	if len(stdout) != 16 {
		t.Errorf("len(stdout) = %d, want 16", len(stdout))
	}
	if (Exec{}).maxBuffer() != MaxBufferSync {
		t.Errorf("zero MaxBuffer = %d, want MaxBufferSync %d", (Exec{}).maxBuffer(), MaxBufferSync)
	}
	if (Exec{MaxBuffer: MaxBufferRepair}).maxBuffer() != MaxBufferRepair {
		t.Errorf("MaxBufferRepair not honored")
	}
}

// captureProcessStreams swaps os.Stdout/os.Stderr for pipes while fn runs and
// returns whatever was written to them — CloneQuiet must leave both empty.
func captureProcessStreams(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	origOut, origErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW
	defer func() { os.Stdout, os.Stderr = origOut, origErr }()
	fn()
	outW.Close()
	errW.Close()
	var outBuf, errBuf bytes.Buffer
	if _, err := io.Copy(&outBuf, outR); err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(&errBuf, errR); err != nil {
		t.Fatal(err)
	}
	return outBuf.String(), errBuf.String()
}

// R5: CloneQuiet issues exactly `clone <url> <dir>`, captures the child's
// streams (nothing reaches the process's stdout/stderr), and reports exit 0
// as a nil error with the captured stderr.
func TestCloneQuietCapturesStreams(t *testing.T) {
	log := stageFakeGit(t)
	t.Setenv("FAKEGIT_STDOUT", "clone-out")
	t.Setenv("FAKEGIT_STDERR", "clone-err")
	var stderr string
	var err error
	out, errOut := captureProcessStreams(t, func() {
		stderr, err = (Exec{}).CloneQuiet(context.Background(), "u", "/y")
	})
	if err != nil {
		t.Fatalf("CloneQuiet err = %v", err)
	}
	if stderr != "clone-err" {
		t.Errorf("captured stderr = %q, want %q", stderr, "clone-err")
	}
	if out != "" || errOut != "" {
		t.Errorf("process streams = %q / %q, want both empty", out, errOut)
	}
	calls := readGitLog(t, log)
	if len(calls) != 1 || strings.Join(calls[0], " ") != "clone u /y" {
		t.Errorf("calls = %v, want [[clone u /y]]", calls)
	}
}

// R5: a non-zero exit returns the *exec.ExitError and the captured stderr.
func TestCloneQuietFailure(t *testing.T) {
	stageFakeGit(t)
	t.Setenv("FAKEGIT_EXIT", "128")
	t.Setenv("FAKEGIT_STDERR", "fatal: repository not found")
	stderr, err := (Exec{}).CloneQuiet(context.Background(), "u", "/y")
	var exitErr *exec.ExitError
	if err == nil || !errors.As(err, &exitErr) {
		t.Fatalf("err = %v, want *exec.ExitError", err)
	}
	if stderr != "fatal: repository not found" {
		t.Errorf("captured stderr = %q", stderr)
	}
}
