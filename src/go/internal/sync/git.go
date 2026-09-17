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
type Exec struct {
	// MaxBuffer caps captured stdout and stderr per stream in bytes, the TS
	// maxBuffer: zero selects MaxBufferSync (sync.ts's execFile, 10 MiB);
	// cmd/turepair passes MaxBufferRepair (repair-metrics.mjs, 64 MiB). On
	// overflow the child is killed, as Node does.
	MaxBuffer int
}

// MaxBufferSync is sync.ts's execFile maxBuffer: 10 * 1024 * 1024.
const MaxBufferSync = 10 * 1024 * 1024

// MaxBufferRepair is repair-metrics.mjs's MAX_GIT_BUFFER: 64 * 1024 * 1024.
const MaxBufferRepair = 64 * 1024 * 1024

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
// stdout and stderr captured separately (each capped at e.MaxBuffer, the TS
// maxBuffer — a stream overflowing its cap kills the child, as Node does);
// stdout is returned on exit 0. On failure the error's text reproduces the TS
// execFileAsync wrapper in src/node/sync/sync.ts exactly: "{summary}...
// failed: {message}" where summary = "git -C <dir>" (the binary plus the
// first two args) and message is Node's — "Command failed: git -C <dir> <args
// joined by single spaces>\n<stderr>" for a non-zero exit (the newline is
// unconditional: with empty stderr the message still ends in "\n"), "{stdout|
// stderr} maxBuffer length exceeded" on a capture overflow, and "spawn git
// ENOENT" when git is not on PATH.
func (e Exec) Run(dir string, args ...string) (string, error) {
	argv := append([]string{"-C", dir}, args...)
	summary := "git -C " + dir
	command := "git " + strings.Join(argv, " ")
	cmd := exec.Command("git", argv...)
	outBuf := &boundedBuffer{stream: "stdout", limit: e.maxBuffer()}
	errBuf := &boundedBuffer{stream: "stderr", limit: e.maxBuffer()}
	kill := func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}
	outBuf.onOverflow, errBuf.onOverflow = kill, kill
	cmd.Stdout = outBuf
	cmd.Stderr = errBuf
	if err := cmd.Run(); err != nil {
		var execErr *exec.Error
		if errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound) {
			return "", fmt.Errorf("%s... failed: spawn git ENOENT", summary)
		}
		if over := firstOverflow(outBuf, errBuf); over != "" {
			return "", fmt.Errorf("%s... failed: %s maxBuffer length exceeded", summary, over)
		}
		message := "Command failed: " + command + "\n" + errBuf.String()
		return "", fmt.Errorf("%s... failed: %s", summary, message)
	}
	if over := firstOverflow(outBuf, errBuf); over != "" {
		// Node errors on the overflow even when the killed child exits 0.
		return "", fmt.Errorf("%s... failed: %s maxBuffer length exceeded", summary, over)
	}
	return outBuf.String(), nil
}

// maxBuffer resolves the capture cap: zero selects the sync default.
func (e Exec) maxBuffer() int {
	if e.MaxBuffer > 0 {
		return e.MaxBuffer
	}
	return MaxBufferSync
}

// boundedBuffer is an io.Writer that retains at most limit bytes of a child
// stream and, on the first byte past the cap, flags the stream as overflowed
// and asks for the child to be killed (Node's maxBuffer kills the process).
type boundedBuffer struct {
	stream     string // "stdout" / "stderr" — the Node error text names it
	limit      int
	buf        bytes.Buffer
	overflow   bool
	onOverflow func()
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if b.overflow {
		return len(p), nil // killed already; keep draining so the child cannot block
	}
	if room := b.limit - b.buf.Len(); room < len(p) {
		b.buf.Write(p[:max(room, 0)])
		b.overflow = true
		if b.onOverflow != nil {
			b.onOverflow()
		}
		return len(p), nil
	}
	return b.buf.Write(p)
}

func (b *boundedBuffer) String() string { return b.buf.String() }

// firstOverflow names the first overflowed stream, checking stdout before
// stderr; empty when neither overflowed.
func firstOverflow(buffers ...*boundedBuffer) string {
	for _, b := range buffers {
		if b.overflow {
			return b.stream
		}
	}
	return ""
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
