// Package sync owns the metrics-repo writer and the git driver (Target
// architecture in fab/plans/sahil/26-09-15-go-port.md). B1 lands only the
// driver; B6 adds the writer, never-shrink guard, dry-run report, and sync
// flow.
package sync

import (
	"context"
	"io"
	"os/exec"
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
// belong to B3's auto-clone guard, not to the interactive init-metrics clone.
func (Exec) Clone(ctx context.Context, url, dir string, stdin io.Reader, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, "git", "clone", url, dir)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}
