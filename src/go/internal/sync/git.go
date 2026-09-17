// Package sync owns the metrics-repo writer and the git driver (Target
// architecture in fab/plans/sahil/26-09-15-go-port.md): the never-shrink
// guarded day-file writer (writer.go), the commit message, .last-sync and
// staleness helpers (state.go), the git round trip and full-sync flow with the
// dry-run report (flow.go, report.go), the Exec git driver (git.go), and the
// repair-metrics twin behind cmd/turepair (repair.go, localecmp.go). The
// interactive clone and the auto-clone guard's quiet clone also live here
// (B1/B3). Nothing in this package prints: flows return their stderr lines
// and typed warnings for the edge to write.
package sync

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Exec drives the real git found on PATH (exec.Command resolves "git" through
// PATH, which is how both sides reach the fake git in the harness). It
// satisfies config.Git.
type Exec struct{}

// IsRepo runs `git -C dir rev-parse --git-dir` with stdout/stderr discarded
// and reports exit 0 (the TS execSync with stdio "pipe" inside try/catch).
func (Exec) IsRepo(dir string) bool {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--git-dir")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}

// Runner is the one git verb the sync flow needs; Exec satisfies it, tests
// use a fake or the PATH-first shell-script git (git_test.go's pattern).
type Runner interface {
	Run(dir string, args ...string) (stdout string, err error)
}

// Run executes `git -C <dir> <args...>` with no timeout (the TS has none),
// stdout and stderr captured separately; stdout is returned on exit 0. On
// failure the error's text reproduces the TS execFileAsync wrapper in
// src/node/sync/sync.ts exactly: "{summary}... failed: {message}" where
// summary = "git -C <dir>" (the binary plus the first two args) and message
// is Node's — "Command failed: git -C <dir> <args joined by single
// spaces>\n<stderr>" for a non-zero exit (the newline is unconditional: with
// empty stderr the message still ends in "\n"), "spawn git ENOENT" when git
// is not on PATH.
func (Exec) Run(dir string, args ...string) (string, error) {
	argv := append([]string{"-C", dir}, args...)
	summary := "git -C " + dir
	command := "git " + strings.Join(argv, " ")
	cmd := exec.Command("git", argv...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		var execErr *exec.Error
		if errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound) {
			return "", fmt.Errorf("%s... failed: spawn git ENOENT", summary)
		}
		message := "Command failed: " + command + "\n" + errBuf.String()
		return "", fmt.Errorf("%s... failed: %s", summary, message)
	}
	return outBuf.String(), nil
}

// Clone runs `git clone url dir` with the given reader and writers attached to
// the child's stdin, stdout and stderr (the TS stdio "inherit" — stdin included,
// so an interactive clone can still prompt for credentials or a passphrase); a
// non-zero exit is returned as *exec.ExitError. No timeout, no GIT_TERMINAL_PROMPT — those
// belong to CloneQuiet (the auto-clone guard), not to the interactive init-metrics clone.
func (Exec) Clone(ctx context.Context, url, dir string, stdin io.Reader, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, "git", "clone", url, dir)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// cloneQuietTimeout is the TS execFileSync timeout: 30_000.
const cloneQuietTimeout = 30 * time.Second

// CloneQuiet runs `git clone url dir` for the auto-clone guard (R5): stdout
// and stderr captured (never inherited), GIT_TERMINAL_PROMPT=0 appended to the
// child environment, and a 30 s deadline applied here (the TS timeout:
// 30_000). err is nil on exit 0, the *exec.ExitError on a non-zero exit, and
// wraps context.DeadlineExceeded when the deadline fires.
func (Exec) CloneQuiet(ctx context.Context, url, dir string) (stderr string, err error) {
	ctx, cancel := context.WithTimeout(ctx, cloneQuietTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "clone", url, dir)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return errBuf.String(), fmt.Errorf("git clone exceeded %s: %w", cloneQuietTimeout, context.DeadlineExceeded)
		}
		return errBuf.String(), err
	}
	return errBuf.String(), nil
}
