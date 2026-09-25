package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeStubCcusage installs a POSIX shell script standing in for the real
// ccusage binary (hermetic CI — see plan Assumptions): --version works,
// opencode yields the real empty shape with a literal -0.0, claude yields two
// days plus a home-rooted path to redact, and kimi fails with exit 3.
func writeStubCcusage(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "ccusage")
	stub := `#!/bin/sh
if [ "$1" = "--version" ]; then
  echo "ccusage 20.0.19"
  exit 0
fi
case "$1" in
  opencode)
    printf '{"daily":[],"totals":{"totalCost":-0.0}}\n'
    ;;
  claude)
    printf '{"daily":[{"date":"2026-09-08"},{"date":"2026-09-16"}],"projectPath":"/home/tester/proj/x"}\n'
    ;;
  kimi)
    echo "kimi exploded" >&2
    exit 3
    ;;
esac
`
	if err := os.WriteFile(path, []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func fixtureBySource(m *Manifest, source string) Fixture {
	for _, fx := range m.Fixtures {
		if fx.Source == source {
			return fx
		}
	}
	return Fixture{ExitCode: -999}
}

// The R6 scenario: an empty -0.0 cell, a non-zero cell recorded (not fatal),
// and a redacted path.
func TestCaptureWithStub(t *testing.T) {
	tmp := t.TempDir()
	stub := writeStubCcusage(t, tmp)
	out := filepath.Join(tmp, "fixtures")
	var log strings.Builder

	summary, err := Capture(CaptureOptions{
		Machine:     "t",
		CcusagePath: stub,
		OutDir:      out,
		Sources:     []string{"opencode", "kimi", "claude"},
		RepoRoot:    tmp,
		Home:        "/home/tester",
	}, &log)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(out, "t", "opencode", "daily.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(raw), "-0.0") {
		t.Errorf("opencode fixture lost the literal -0.0: %s", raw)
	}

	m := summary.Manifest
	kimi := fixtureBySource(m, "kimi")
	if kimi.ExitCode != 3 || !kimi.Empty || kimi.Days != 0 {
		t.Errorf("kimi fixture = %+v, want exit_code 3, empty, days 0", kimi)
	}
	if kimi.StderrFile != "kimi/daily.stderr.txt" {
		t.Errorf("kimi stderr_file = %q", kimi.StderrFile)
	}
	if _, err := os.Stat(filepath.Join(out, "t", "kimi", "daily.stderr.txt")); err != nil {
		t.Errorf("stderr sidecar missing: %v", err)
	}
	if len(summary.Failed) != 1 || summary.Failed[0] != "kimi daily" {
		t.Errorf("Failed = %v, want [kimi daily]", summary.Failed)
	}
	if !strings.Contains(log.String(), "non-zero exit: kimi daily") {
		t.Errorf("summary line does not name kimi:\n%s", log.String())
	}

	claude := fixtureBySource(m, "claude")
	if claude.Days != 2 || claude.FirstDate != "2026-09-08" || claude.LastDate != "2026-09-16" || claude.Empty {
		t.Errorf("claude fixture = %+v", claude)
	}
	if claude.Redactions != 1 {
		t.Errorf("claude redactions = %d, want 1", claude.Redactions)
	}
	craw, err := os.ReadFile(filepath.Join(out, "t", "claude", "daily.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(craw), `"~/redacted-1"`) || strings.Contains(string(craw), "/home/tester") {
		t.Errorf("claude fixture redaction wrong: %s", craw)
	}

	opencode := fixtureBySource(m, "opencode")
	if opencode.ExitCode != 0 || !opencode.Empty || opencode.StderrFile != "" {
		t.Errorf("opencode fixture = %+v", opencode)
	}

	// sha256 is recorded over the committed bytes.
	stored, err := ReadManifest(filepath.Join(out, "t"))
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if len(stored.Fixtures) != 3 {
		t.Fatalf("stored manifest has %d fixtures, want 3", len(stored.Fixtures))
	}
	for _, fx := range stored.Fixtures {
		if len(fx.Sha256) != 64 {
			t.Errorf("%s sha256 = %q", fx.Source, fx.Sha256)
		}
	}
}

// R3: re-running a capture for alias A must not touch a sibling alias dir.
func TestCaptureLeavesSiblingAliasUntouched(t *testing.T) {
	tmp := t.TempDir()
	stub := writeStubCcusage(t, tmp)
	out := filepath.Join(tmp, "fixtures")

	sibling := filepath.Join(out, "_placeholder")
	if err := os.MkdirAll(sibling, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := []byte(`{"machine":"_placeholder"}` + "\n")
	if err := os.WriteFile(filepath.Join(sibling, "manifest.json"), marker, 0o644); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 2; i++ {
		var log strings.Builder
		if _, err := Capture(CaptureOptions{
			Machine: "t", CcusagePath: stub, OutDir: out,
			Sources: []string{"opencode"}, RepoRoot: tmp, Home: "/home/tester",
		}, &log); err != nil {
			t.Fatalf("Capture run %d: %v", i, err)
		}
	}
	got, err := os.ReadFile(filepath.Join(sibling, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(marker) {
		t.Errorf("sibling alias modified:\n got %s\nwant %s", got, marker)
	}
}

func TestFindRepoRoot(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "justfile"), []byte("x:\n\ttrue\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(tmp, "a", "b")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := FindRepoRoot(deep)
	if err != nil {
		t.Fatalf("FindRepoRoot: %v", err)
	}
	if root != tmp {
		t.Errorf("FindRepoRoot = %q, want %q", root, tmp)
	}
	_, err = FindRepoRoot(filepath.Join(string(filepath.Separator), "definitely-no-justfile-here"))
	if err == nil {
		t.Error("FindRepoRoot without justfile must error")
	}
	if want := "tudiff: no justfile found above "; !strings.HasPrefix(err.Error(), want) {
		t.Errorf("error = %q, want prefix %q", err, want)
	}
}

func TestResolveCcusageExplicitAndMissing(t *testing.T) {
	tmp := t.TempDir()
	stub := writeStubCcusage(t, tmp)
	got, err := ResolveCcusage(tmp, stub)
	if err != nil || got != stub {
		t.Errorf("explicit: got %q, %v; want %q, nil", got, err, stub)
	}
	if _, err := ResolveCcusage(tmp, filepath.Join(tmp, "nope")); err == nil {
		t.Error("missing explicit path must error")
	}
}

// The repo-root dist/vendor copy wins over PATH; the vendor beside the tu on
// PATH (symlinks resolved) wins over a bare ccusage on PATH.
func TestResolveCcusageVendorOrder(t *testing.T) {
	tmp := t.TempDir()
	vendored := filepath.Join(tmp, "dist", "vendor", "ccusage", "bin")
	if err := os.MkdirAll(vendored, 0o755); err != nil {
		t.Fatal(err)
	}
	stub := filepath.Join(vendored, "ccusage")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveCcusage(tmp, "")
	if err != nil {
		t.Fatalf("ResolveCcusage: %v", err)
	}
	if got != stub {
		t.Errorf("ResolveCcusage = %q, want %q", got, stub)
	}

	// Without the repo-root vendor copy, the vendor tree beside the `tu`
	// found on PATH is next (the brew layout; bin/tu symlinked into libexec).
	t.Run("vendor beside tu on PATH", func(t *testing.T) {
		root := t.TempDir()
		libexec := filepath.Join(root, "libexec")
		vendorBin := filepath.Join(libexec, "vendor", "ccusage", "bin")
		if err := os.MkdirAll(vendorBin, 0o755); err != nil {
			t.Fatal(err)
		}
		vendorStub := filepath.Join(vendorBin, "ccusage")
		if err := os.WriteFile(vendorStub, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(libexec, "tu"), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		binDir := filepath.Join(root, "bin")
		if err := os.MkdirAll(binDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(libexec, "tu"), filepath.Join(binDir, "tu")); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", binDir)
		got, err := ResolveCcusage(t.TempDir(), "")
		if err != nil {
			t.Fatalf("ResolveCcusage: %v", err)
		}
		if got != vendorStub {
			t.Errorf("ResolveCcusage = %q, want %q", got, vendorStub)
		}
	})
}

func TestCcusageVersion(t *testing.T) {
	stub := writeStubCcusage(t, t.TempDir())
	v, err := CcusageVersion(stub)
	if err != nil {
		t.Fatalf("CcusageVersion: %v", err)
	}
	if v != "20.0.19" {
		t.Errorf("CcusageVersion = %q, want 20.0.19", v)
	}
}

func TestSystemTimezone(t *testing.T) {
	t.Setenv("TZ", "Asia/Kolkata")
	if got := SystemTimezone(); got != "Asia/Kolkata" {
		t.Errorf("SystemTimezone with TZ set = %q", got)
	}
}

// A fixture-write failure is an infrastructure fault of the capture itself,
// not a failing source: it must propagate as the run's error, name the path,
// and leave no manifest behind.
func TestCaptureWriteErrorPropagates(t *testing.T) {
	tmp := t.TempDir()
	stub := writeStubCcusage(t, tmp)

	// Point --out at an existing file so MkdirAll under it fails.
	outFile := filepath.Join(tmp, "not-a-dir")
	if err := os.WriteFile(outFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	var log strings.Builder
	_, err := Capture(CaptureOptions{
		Machine: "t", CcusagePath: stub, OutDir: outFile,
		Sources: []string{"opencode"}, RepoRoot: tmp, Home: "/home/tester",
	}, &log)
	if err == nil {
		t.Fatal("expected a write error, got nil")
	}
	if !strings.Contains(err.Error(), outFile) {
		t.Errorf("error must name the failing path: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(outFile, "t", "manifest.json")); statErr == nil {
		t.Error("no manifest may be written when a fixture write fails")
	}
}

// Machine, source, and period become fixture path components; values with
// separators or traversal must be rejected before any path is joined.
func TestCaptureRejectsPathTraversal(t *testing.T) {
	tmp := t.TempDir()
	stub := writeStubCcusage(t, tmp)
	base := CaptureOptions{
		Machine: "t", CcusagePath: stub, OutDir: filepath.Join(tmp, "fixtures"),
		Sources: []string{"opencode"}, RepoRoot: tmp, Home: "/home/tester",
	}

	bad := base
	bad.Machine = "../outside"
	if _, err := Capture(bad, &strings.Builder{}); err == nil {
		t.Error("machine alias with traversal must error")
	}

	bad = base
	bad.Sources = []string{"../outside"}
	if _, err := Capture(bad, &strings.Builder{}); err == nil {
		t.Error("source with traversal must error")
	}

	bad = base
	bad.Periods = []string{"a/b"}
	if _, err := Capture(bad, &strings.Builder{}); err == nil {
		t.Error("period with a separator must error")
	}

	if _, err := os.Stat(filepath.Join(tmp, "outside")); !os.IsNotExist(err) {
		t.Error("nothing may be written outside the fixture root")
	}
}

// An explicit relative --ccusage resolves against the caller's cwd, not the
// repo root Capture later chdirs into: ResolveCcusage returns it absolute.
func TestResolveCcusageExplicitRelativeIsAbsolute(t *testing.T) {
	tmp := t.TempDir()
	writeStubCcusage(t, tmp)
	t.Chdir(tmp)
	got, err := ResolveCcusage(tmp, "ccusage")
	if err != nil {
		t.Fatalf("ResolveCcusage: %v", err)
	}
	if want := filepath.Join(tmp, "ccusage"); got != want {
		t.Errorf("ResolveCcusage = %q, want absolute %q", got, want)
	}
}
