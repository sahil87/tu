package harness

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/sahil87/tu/internal/source/ccusage"
)

// CaptureTimeout bounds a single ccusage cell invocation.
const CaptureTimeout = 120 * time.Second

// DefaultSources are the six ccusage subcommands tu uses, in the retired
// TypeScript implementation's registry order.
var DefaultSources = []string{"claude", "codex", "opencode", "gemini", "copilot", "kimi"}

// CaptureOptions parameterizes one Capture run. Zero values default as
// documented per field.
type CaptureOptions struct {
	Machine     string   // fixture alias; default os.Hostname()
	CcusagePath string   // resolved path to the ccusage binary (required)
	OutDir      string   // fixture root; default "harness/fixtures"
	Sources     []string // ccusage subcommands; default DefaultSources
	Periods     []string // default ["daily"]
	RepoRoot    string   // cwd for ccusage invocations
	Home        string   // redaction root; default os.UserHomeDir()
}

// CaptureSummary reports a finished Capture run.
type CaptureSummary struct {
	Manifest *Manifest
	Failed   []string // "<source> <period>" cells that recorded a non-zero exit
}

// FindRepoRoot walks up from start to the first directory containing
// justfile.
func FindRepoRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "justfile")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("tudiff: no justfile found above %s", start)
		}
		dir = parent
	}
}

// ResolveCcusage picks the ccusage binary per the capture resolution order:
// an explicit --ccusage path, then dist/vendor/ccusage/bin/ccusage under the
// repo root, then the vendored binary beside the tu found on PATH (symlinks
// resolved, as ccusage.ResolveBinary does), then ccusage on PATH.
func ResolveCcusage(root, explicit string) (string, error) {
	if explicit != "" {
		if _, err := os.Stat(explicit); err != nil {
			return "", fmt.Errorf("tudiff: ccusage binary not found: %s", explicit)
		}
		// Capture runs cells with Dir = RepoRoot, so a relative explicit path
		// would resolve against the repo root instead of the caller's cwd.
		abs, err := filepath.Abs(explicit)
		if err != nil {
			return "", err
		}
		return abs, nil
	}
	vendored := filepath.Join(root, "dist", "vendor", "ccusage", "bin", "ccusage")
	if _, err := os.Stat(vendored); err == nil {
		return vendored, nil
	}
	if vendor, ok := ccusage.VendorBeside("tu"); ok {
		return vendor, nil
	}
	if p, err := exec.LookPath("ccusage"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("tudiff: no ccusage binary found (pass --ccusage)")
}

// CcusageVersion runs `<path> --version` and extracts the bare version from
// the `ccusage 20.0.19` output line.
func CcusageVersion(path string) (string, error) {
	out, err := exec.Command(path, "--version").Output()
	if err != nil {
		return "", fmt.Errorf("tudiff: %s --version failed: %w", path, err)
	}
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(out)), "ccusage ")), nil
}

// SystemTimezone is the IANA zone ccusage buckets dates by: $TZ when set,
// else the system zone name, with an /etc/timezone fallback for Linux boxes
// where Go reports "Local".
func SystemTimezone() string {
	if tz := os.Getenv("TZ"); tz != "" {
		return tz
	}
	loc := time.Now().Location().String()
	if loc != "Local" {
		return loc
	}
	if raw, err := os.ReadFile("/etc/timezone"); err == nil {
		if tz := strings.TrimSpace(string(raw)); tz != "" {
			return tz
		}
	}
	return loc
}

// Capture runs the (source × period) matrix against the real ccusage binary,
// writing redacted-but-otherwise-verbatim fixtures plus a regenerated
// manifest under <OutDir>/<Machine>/. Other alias directories under OutDir
// are neither read nor modified. A non-zero exit or timeout in a cell is
// recorded in the manifest and never aborts the run.
func Capture(opts CaptureOptions, stdout io.Writer) (*CaptureSummary, error) {
	if opts.Machine == "" {
		host, err := os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("tudiff: hostname: %w", err)
		}
		opts.Machine = host
	}
	if opts.OutDir == "" {
		opts.OutDir = "harness/fixtures"
	}
	if len(opts.Sources) == 0 {
		opts.Sources = DefaultSources
	}
	if len(opts.Periods) == 0 {
		opts.Periods = []string{"daily"}
	}
	// Machine, sources, and periods become fixture path components; reject
	// separators and traversal so a flag value cannot escape OutDir.
	if err := pathComponent("machine alias", opts.Machine); err != nil {
		return nil, err
	}
	for _, source := range opts.Sources {
		if err := pathComponent("source", source); err != nil {
			return nil, err
		}
	}
	for _, period := range opts.Periods {
		if err := pathComponent("period", period); err != nil {
			return nil, err
		}
	}
	if opts.Home == "" {
		opts.Home, _ = os.UserHomeDir()
	}
	if opts.CcusagePath == "" {
		return nil, fmt.Errorf("tudiff: no ccusage binary found (pass --ccusage)")
	}

	version, err := CcusageVersion(opts.CcusagePath)
	if err != nil {
		return nil, err
	}

	aliasDir := filepath.Join(opts.OutDir, opts.Machine)
	m := &Manifest{
		Schema:         SchemaVersion,
		Machine:        opts.Machine,
		CapturedAt:     time.Now().UTC().Format(time.RFC3339),
		CcusageVersion: version,
		CcusagePath:    repoRelative(opts.RepoRoot, opts.CcusagePath),
		Platform:       runtime.GOOS + "/" + runtime.GOARCH,
		Timezone:       SystemTimezone(),
	}
	summary := &CaptureSummary{Manifest: m}

	for _, source := range opts.Sources {
		for _, period := range opts.Periods {
			fx, err := captureCell(opts, source, period, aliasDir)
			if err != nil {
				return nil, err
			}
			m.Fixtures = append(m.Fixtures, fx)
			if fx.ExitCode != 0 {
				summary.Failed = append(summary.Failed, source+" "+period)
			}
			fmt.Fprintf(stdout, "%s %s --json  exit=%d  days=%d  %s..%s  redactions=%d  -> %s/%s\n",
				source, period, fx.ExitCode, fx.Days, fx.FirstDate, fx.LastDate, fx.Redactions,
				opts.Machine, fx.File)
		}
	}

	if err := WriteManifest(aliasDir, m); err != nil {
		return nil, err
	}
	if len(summary.Failed) > 0 {
		fmt.Fprintf(stdout, "captured %d cells into %s (non-zero exit: %s)\n",
			len(m.Fixtures), aliasDir, strings.Join(summary.Failed, ", "))
	} else {
		fmt.Fprintf(stdout, "captured %d cells into %s\n", len(m.Fixtures), aliasDir)
	}
	return summary, nil
}

// captureCell records one (source, period) cell and writes its fixture files.
// A write failure is an infrastructure fault of the capture itself — not a
// failing source, which R6 confines to ccusage exit codes/timeouts — so it is
// returned as an error for Capture to propagate.
func captureCell(opts CaptureOptions, source, period, aliasDir string) (Fixture, error) {
	fx := Fixture{
		Source: source,
		Period: period,
		Args:   []string{"--json"},
		File:   FixturePath(source, period),
	}

	ctx, cancel := context.WithTimeout(context.Background(), CaptureTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, opts.CcusagePath, source, period, "--json")
	cmd.Dir = opts.RepoRoot
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()

	switch {
	case ctx.Err() == context.DeadlineExceeded:
		fx.ExitCode = -1
	case err == nil:
		fx.ExitCode = 0
	default:
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			fx.ExitCode = exitErr.ExitCode()
		} else {
			fx.ExitCode = -1
		}
	}

	redacted, n := Redact(outBuf.Bytes(), opts.Home)
	fx.Redactions = n
	fx.Days, fx.FirstDate, fx.LastDate, fx.Empty = Summarize(redacted)

	path := filepath.Join(aliasDir, filepath.FromSlash(fx.File))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fx, fmt.Errorf("tudiff: cannot create fixture dir for %s: %w", path, err)
	}
	if err := os.WriteFile(path, redacted, 0o644); err != nil {
		return fx, fmt.Errorf("tudiff: cannot write fixture %s: %w", path, err)
	}
	sum := sha256.Sum256(redacted)
	fx.Sha256 = hex.EncodeToString(sum[:])
	if errBuf.Len() > 0 {
		fx.StderrFile = StderrPath(source, period)
		sidecar := filepath.Join(aliasDir, filepath.FromSlash(fx.StderrFile))
		if err := os.WriteFile(sidecar, errBuf.Bytes(), 0o644); err != nil {
			return fx, fmt.Errorf("tudiff: cannot write fixture stderr %s: %w", sidecar, err)
		}
	}
	return fx, nil
}

// repoRelative renders path relative to root when it lies underneath it, so
// committed manifests carry no absolute, machine-specific prefixes.
func repoRelative(root, path string) string {
	if root == "" {
		return path
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return path
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return path
	}
	return filepath.ToSlash(rel)
}
