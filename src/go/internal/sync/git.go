// Package sync owns the metrics-repo writer and the git driver (Target
// architecture in fab/plans/sahil/26-09-15-go-port.md). B1 landed the
// interactive driver and B3 the quiet clone for the auto-clone guard; B6 adds
// the writer, never-shrink guard, dry-run report, and sync flow.
package sync

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
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
