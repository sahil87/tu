package ccusage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sahil87/tu/internal/source"
)

// DefaultTimeout is the per-source deadline the command edge should put on
// the context it hands to Fetch. The source itself imposes no deadline: it
// honors the given context only.
const DefaultTimeout = 120 * time.Second

// ResolveBinary returns the ccusage binary path, in order: (1) the vendor
// sibling of the running executable (the tarball layout: binary +
// vendor/ccusage/bin/ccusage beside it, with executable symlinks resolved so
// a Homebrew bin/ symlink finds the vendor tree beside the real file), (2)
// ccusage on PATH, (3) an error wrapping exec.ErrNotFound. It never walks to
// a repo root or probes node_modules. Source.Binary non-empty bypasses
// resolution entirely.
func ResolveBinary() (string, error) {
	if exe, err := os.Executable(); err == nil {
		if resolved, rerr := filepath.EvalSymlinks(exe); rerr == nil {
			exe = resolved
		}
		vendor := filepath.Join(filepath.Dir(exe), "vendor", "ccusage", "bin", "ccusage")
		if _, err := os.Stat(vendor); err == nil {
			return vendor, nil
		}
	}
	if path, err := exec.LookPath("ccusage"); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("ccusage: no ccusage binary found: %w", exec.ErrNotFound)
}

// run executes binary with argv (no shell), inheriting the environment and
// cwd, capturing stdout and stderr into separate buffers with no size cap. A
// failed run is classified by R8: context deadline → KindTimeout; a start
// failure → KindExec with the spawn detail; a non-zero exit or signal →
// KindExec with the Node-shaped "Command failed" detail. stderr of a
// successful run is discarded, never forwarded.
//
// Output goes to real files, not in-memory pipes: a killed child (context
// deadline) can leave grandchildren holding a pipe open, and exec.Wait would
// then block past the deadline waiting for pipe EOF.
func run(ctx context.Context, tool Tool, binary string, args []string) ([]byte, *source.Error) {
	outFile, err := os.CreateTemp("", "tu-ccusage-stdout")
	if err != nil {
		return nil, &source.Error{Tool: tool.Key, Name: tool.Name, Kind: source.KindExec, Detail: spawnDetail(binary, err), Err: err}
	}
	defer os.Remove(outFile.Name())
	defer outFile.Close()
	errFile, err := os.CreateTemp("", "tu-ccusage-stderr")
	if err != nil {
		return nil, &source.Error{Tool: tool.Key, Name: tool.Name, Kind: source.KindExec, Detail: spawnDetail(binary, err), Err: err}
	}
	defer os.Remove(errFile.Name())
	defer errFile.Close()

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Stdout = outFile
	cmd.Stderr = errFile

	start := time.Now()
	runErr := cmd.Run()
	stderr := string(readAll(errFile))
	if runErr == nil {
		return readAll(outFile), nil
	}

	switch {
	case ctx.Err() == context.DeadlineExceeded:
		elapsed := time.Since(start).Round(time.Millisecond)
		return nil, &source.Error{
			Tool: tool.Key, Name: tool.Name, Kind: source.KindTimeout,
			Detail: fmt.Sprintf("timeout after %s", elapsed),
			Err:    runErr,
		}
	case isExitError(runErr):
		return nil, &source.Error{
			Tool: tool.Key, Name: tool.Name, Kind: source.KindExec,
			Detail: "Command failed: " + strings.Join(append([]string{binary}, args...), " ") + "\n" + stderr,
			Err:    runErr,
		}
	default:
		return nil, &source.Error{
			Tool: tool.Key, Name: tool.Name, Kind: source.KindExec,
			Detail: spawnDetail(binary, runErr),
			Err:    runErr,
		}
	}
}

// readAll returns the full contents of f from the start; a read failure
// yields an empty slice (the capture is best-effort beside a run failure).
func readAll(f *os.File) []byte {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil
	}
	raw, err := io.ReadAll(f)
	if err != nil {
		return nil
	}
	return raw
}

// isExitError reports whether the command ran and failed (non-zero exit or
// signal), as opposed to failing to start.
func isExitError(err error) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr)
}

// spawnDetail renders a start failure the way Node's error.message does:
// "spawn {binary} ENOENT" when the binary is missing, otherwise "spawn
// {binary} " plus Go's error text.
func spawnDetail(binary string, err error) string {
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, fs.ErrNotExist) {
		return "spawn " + binary + " ENOENT"
	}
	return "spawn " + binary + " " + err.Error()
}
