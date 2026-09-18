package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFile writes body at <root>/<rel>, creating parent directories.
func writeFile(t *testing.T, root, rel, body string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// writeExe writes an executable shell script.
func writeExe(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

// smokeEnv is a self-contained fake checkout for run tests: a repo root with
// package.json, tu.default.conf, a minimal _placeholder manifest and the seed
// dir; stand-in --node/--go shell scripts (the node side runs through a
// `node` shim on PATH, so the smoke never needs Node); stub ccusage/git
// fakes; and a report dir, all under temp space.
type smokeEnv struct {
	root       string
	matrix     string
	nodeBundle string
	goBin      string
	harnessBin string
	binDir     string // holds the `node` shim; prepend to PATH
	report     string
}

const smokeMatrix = `{"schema":1,"cases":[
  {"id":"same","args":["ok"]},
  {"id":"diff","args":["nope"]}
]}`

func newSmokeEnv(t *testing.T, matrixBody string) *smokeEnv {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "package.json", "{}\n")
	writeFile(t, root, "tu.default.conf", "version = 2\n")
	writeFile(t, root, "harness/fixtures/_placeholder/manifest.json",
		`{"schema":1,"machine":"_placeholder","captured_at":"2026-09-16T00:00:00Z","ccusage_version":"20.0.19","ccusage_path":"","platform":"derived","timezone":"","fixtures":[]}`+"\n")
	writeFile(t, root, "harness/expected-diffs.json", "{\n  \"schema\": 1,\n  \"expected\": []\n}\n")
	writeFile(t, root, "harness/metrics-repo/docs/README.md", "seed\n")

	e := &smokeEnv{root: root}
	e.matrix = writeFile(t, t.TempDir(), "matrix.json", matrixBody)

	e.binDir = t.TempDir()
	writeExe(t, filepath.Join(e.binDir, "node"), "#!/bin/sh\nexec /bin/sh \"$@\"\n")
	e.nodeBundle = writeFile(t, t.TempDir(), "oracle.mjs", "#!/bin/sh\nprintf 'hello\\n'\n")

	e.goBin = filepath.Join(t.TempDir(), "tu")
	writeExe(t, e.goBin, `#!/bin/sh
if [ "${1:-}" = "ok" ]; then
  printf 'hello\n'
else
  printf 'different\n'
fi
`)

	e.harnessBin = t.TempDir()
	writeExe(t, filepath.Join(e.harnessBin, "ccusage"), "#!/bin/sh\nexit 0\n")
	writeExe(t, filepath.Join(e.harnessBin, "git"), "#!/bin/sh\nexit 0\n")
	e.report = filepath.Join(t.TempDir(), "report")
	return e
}

// invoke runs `run` through the top-level dispatcher with every path flag
// pinned at this environment, from the fake root as cwd.
func (e *smokeEnv) invoke(t *testing.T, extra ...string) (int, string, string) {
	t.Helper()
	t.Chdir(e.root)
	args := []string{"run",
		"--matrix", e.matrix,
		"--node", e.nodeBundle,
		"--go", e.goBin,
		"--harness-bin", e.harnessBin,
		"--report", e.report,
	}
	args = append(args, extra...)
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// withShimmedPATH prepends the node-shim dir to PATH for the duration of f.
func (e *smokeEnv) withShimmedPATH(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", e.binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// R19: end-to-end smoke — stand-in executables, one green and one red case
// in report.json, exit 1.
func TestRunEndToEndSmoke(t *testing.T) {
	e := newSmokeEnv(t, smokeMatrix)
	e.withShimmedPATH(t)
	code, stdout, stderr := e.invoke(t)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (stderr: %s)", code, stderr)
	}
	for _, sub := range []string{
		"GREEN   same/single/default/pipe/fixed",
		"RED     diff/single/default/pipe/fixed  stdout @0 (line 1):",
		"tudiff: 2 cases — 1 green, 1 red (0 expected, 1 unexpected), 0 timeout",
	} {
		if !strings.Contains(stdout, sub) {
			t.Errorf("stdout lacks %q:\n%s", sub, stdout)
		}
	}
	// Matrix order holds in the report regardless of completion order.
	if strings.Index(stdout, "same/") > strings.Index(stdout, "diff/") {
		t.Errorf("case lines out of matrix order:\n%s", stdout)
	}

	raw, err := os.ReadFile(filepath.Join(e.report, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Summary struct {
			Green int `json:"green"`
			Red   int `json:"red"`
		} `json:"summary"`
		Cases []struct {
			ID      string `json:"id"`
			Status  string `json:"status"`
			Channel string `json:"channel"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Summary.Green != 1 || doc.Summary.Red != 1 {
		t.Errorf("summary = %+v", doc.Summary)
	}
	if len(doc.Cases) != 2 || doc.Cases[0].Status != "green" || doc.Cases[1].Status != "red" || doc.Cases[1].Channel != "stdout" {
		t.Errorf("cases = %+v", doc.Cases)
	}
	// Raw captures for the red case are inspectable.
	if _, err := os.Stat(filepath.Join(e.report, "cases", "diff", "single", "default", "pipe", "fixed", "go.stdout")); err != nil {
		t.Errorf("missing go.stdout capture: %v", err)
	}
}

// R15: a local-date change between the two sides triggers exactly one re-run.
func TestRunDateRolloverRerun(t *testing.T) {
	e := newSmokeEnv(t, `{"schema":1,"cases":[{"id":"roll","args":["ok"]}]}`)
	e.withShimmedPATH(t)

	// The stand-ins count their own executions (the child env is scrubbed, so
	// the counter paths are baked into the scripts).
	countDir := t.TempDir()
	nodeCount := filepath.Join(countDir, "node.count")
	goCount := filepath.Join(countDir, "go.count")
	writeFile(t, filepath.Dir(e.nodeBundle), filepath.Base(e.nodeBundle),
		"#!/bin/sh\necho node >>"+nodeCount+"\nprintf 'hello\\n'\n")
	writeExe(t, e.goBin, "#!/bin/sh\nif [ \"${1:-}\" = \"ok\" ]; then echo go >>"+goCount+"; fi\nprintf 'hello\\n'\n")

	old := localDateInTZ
	t.Cleanup(func() { localDateInTZ = old })
	dates := []string{"2026-09-16", "2026-09-17"}
	calls := 0
	localDateInTZ = func(string) string {
		if calls < len(dates) {
			d := dates[calls]
			calls++
			return d
		}
		return dates[len(dates)-1]
	}

	code, _, stderr := e.invoke(t)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	count := func(path string) int {
		raw, err := os.ReadFile(path)
		if err != nil {
			return 0
		}
		return len(strings.Split(strings.TrimRight(string(raw), "\n"), "\n"))
	}
	if count(nodeCount) != 2 || count(goCount) != 2 {
		t.Errorf("executions node=%d go=%d, want 2 each", count(nodeCount), count(goCount))
	}
	raw, err := os.ReadFile(filepath.Join(e.report, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Cases []struct {
			Rerun bool `json:"rerun"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Cases) != 1 || !doc.Cases[0].Rerun {
		t.Errorf("rerun not recorded: %+v", doc.Cases)
	}
}

// R2: --list prints the expanded, filtered IDs and exits 0 without touching
// the report dir — before any binary check.
func TestRunList(t *testing.T) {
	e := newSmokeEnv(t, smokeMatrix)
	// No PATH shim, no binaries needed: --list never reaches those checks.
	code, stdout, stderr := e.invoke(t, "--list")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	want := "same/single/default/pipe/fixed\ndiff/single/default/pipe/fixed\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
	if _, err := os.Stat(e.report); !os.IsNotExist(err) {
		t.Errorf("--list created the report dir")
	}
}

// R2: every preflight failure is its exact one-line message and exit 2.
func TestRunPreflight(t *testing.T) {
	t.Run("fixtures and placeholder mutually exclusive", func(t *testing.T) {
		e := newSmokeEnv(t, smokeMatrix)
		code, _, stderr := e.invoke(t, "--fixtures", "a", "--placeholder")
		if code != 2 || stderr != "tudiff: --fixtures and --placeholder are mutually exclusive\n" {
			t.Errorf("code = %d, stderr = %q", code, stderr)
		}
	})
	t.Run("jobs below 1", func(t *testing.T) {
		e := newSmokeEnv(t, smokeMatrix)
		code, _, stderr := e.invoke(t, "--jobs", "0")
		if code != 2 || stderr != "tudiff: --jobs must be >= 1\n" {
			t.Errorf("code = %d, stderr = %q", code, stderr)
		}
	})
	t.Run("matrix unreadable", func(t *testing.T) {
		e := newSmokeEnv(t, smokeMatrix)
		code, _, stderr := e.invoke(t, "--matrix", filepath.Join(t.TempDir(), "nope.json"))
		if code != 2 || !strings.HasPrefix(stderr, "tudiff: matrix: ") {
			t.Errorf("code = %d, stderr = %q", code, stderr)
		}
	})
	t.Run("filter matches nothing", func(t *testing.T) {
		e := newSmokeEnv(t, smokeMatrix)
		code, _, stderr := e.invoke(t, "--filter", "zzz")
		if code != 2 || stderr != "tudiff: --filter matched no cases\n" {
			t.Errorf("code = %d, stderr = %q", code, stderr)
		}
	})
	t.Run("node bundle missing", func(t *testing.T) {
		e := newSmokeEnv(t, smokeMatrix)
		code, _, stderr := e.invoke(t, "--node", "/no/such/tu.mjs")
		if code != 2 || stderr != "tudiff: /no/such/tu.mjs not found (run npm ci && npm run build)\n" {
			t.Errorf("code = %d, stderr = %q", code, stderr)
		}
	})
	t.Run("go binary missing names the path as given", func(t *testing.T) {
		e := newSmokeEnv(t, smokeMatrix)
		// Default --go resolves against the repo root; the fake root has no bin/tu.
		code, _, stderr := e.invoke(t, "--go", defaultGo)
		if code != 2 || stderr != "tudiff: bin/tu not found (run just go-build)\n" {
			t.Errorf("code = %d, stderr = %q", code, stderr)
		}
	})
	t.Run("go binary not executable", func(t *testing.T) {
		e := newSmokeEnv(t, smokeMatrix)
		notExe := writeFile(t, t.TempDir(), "tu", "not executable\n")
		code, _, stderr := e.invoke(t, "--go", notExe)
		if code != 2 || stderr != "tudiff: "+notExe+" not executable (run just go-build)\n" {
			t.Errorf("code = %d, stderr = %q", code, stderr)
		}
	})
	t.Run("harness bin lacks fakes", func(t *testing.T) {
		e := newSmokeEnv(t, smokeMatrix)
		bare := t.TempDir()
		code, _, stderr := e.invoke(t, "--harness-bin", bare)
		if code != 2 || stderr != "tudiff: "+filepath.Join(bare, "ccusage")+" not found (run just harness-build)\n" {
			t.Errorf("code = %d, stderr = %q", code, stderr)
		}
	})
	t.Run("node not on PATH", func(t *testing.T) {
		e := newSmokeEnv(t, smokeMatrix)
		t.Setenv("PATH", t.TempDir())
		code, _, stderr := e.invoke(t)
		if code != 2 || stderr != "tudiff: node not found on PATH (required to run the TS oracle)\n" {
			t.Errorf("code = %d, stderr = %q", code, stderr)
		}
	})
	t.Run("placeholder manifest absent", func(t *testing.T) {
		e := newSmokeEnv(t, smokeMatrix)
		e.withShimmedPATH(t)
		if err := os.RemoveAll(filepath.Join(e.root, "harness", "fixtures")); err != nil {
			t.Fatal(err)
		}
		code, _, stderr := e.invoke(t)
		if code != 2 || stderr != "tudiff: harness/fixtures/_placeholder/manifest.json not found\n" {
			t.Errorf("code = %d, stderr = %q", code, stderr)
		}
	})
	t.Run("fixtures alias without manifest", func(t *testing.T) {
		e := newSmokeEnv(t, smokeMatrix)
		e.withShimmedPATH(t)
		if err := os.MkdirAll(filepath.Join(e.root, "harness", "fixtures", "bare"), 0o755); err != nil {
			t.Fatal(err)
		}
		code, _, stderr := e.invoke(t, "--fixtures", "bare")
		if code != 2 || !strings.Contains(stderr, `tudiff: fixtures alias "bare" has no manifest.json`) {
			t.Errorf("code = %d, stderr = %q", code, stderr)
		}
	})
}

// A-028: script(1) absence is an error only when the filtered cases include a
// tty case.
func TestRunScriptGate(t *testing.T) {
	matrix := `{"schema":1,"cases":[
	  {"id":"same","args":["ok"]},
	  {"id":"via-tty","args":["ok"],"io":["pipe","tty"]}
	]}`
	binNoScript := t.TempDir()
	writeExe(t, filepath.Join(binNoScript, "node"), "#!/bin/sh\nexec /bin/sh \"$@\"\n")

	t.Run("tty case without script", func(t *testing.T) {
		e := newSmokeEnv(t, matrix)
		t.Setenv("PATH", binNoScript)
		code, _, stderr := e.invoke(t)
		if code != 2 || stderr != "tudiff: script not found on PATH (required for tty cases)\n" {
			t.Errorf("code = %d, stderr = %q", code, stderr)
		}
	})
	t.Run("pipe-only filter runs without script", func(t *testing.T) {
		e := newSmokeEnv(t, matrix)
		t.Setenv("PATH", binNoScript)
		code, _, stderr := e.invoke(t, "--filter", "same/")
		if code == 2 {
			t.Errorf("code = 2, stderr = %q", stderr)
		}
	})
}

// R5: a red case matched by an expected-diffs entry passes the gate (exit 0)
// and carries the [expected <id>] marker.
func TestRunExpectedRedExitsZero(t *testing.T) {
	e := newSmokeEnv(t, smokeMatrix)
	e.withShimmedPATH(t)
	expFile := writeFile(t, t.TempDir(), "expected.json",
		`{"schema":1,"expected":[{"id":"DC-99","cases":["diff"],"reason":"smoke"}]}`+"\n")
	code, stdout, stderr := e.invoke(t, "--expected", expFile)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	for _, sub := range []string{
		"RED     diff/single/default/pipe/fixed  stdout @0 (line 1): node=\"hello\\n\" go=\"different\\n\" [expected DC-99]",
		"tudiff: 2 cases — 1 green, 1 red (1 expected, 0 unexpected), 0 timeout",
		"  expected: DC-99  1/1 red",
	} {
		if !strings.Contains(stdout, sub) {
			t.Errorf("stdout lacks %q:\n%s", sub, stdout)
		}
	}
}

// R5: an entry whose matched cases are all green is stale and fails the gate.
func TestRunStaleExpectedExitsOne(t *testing.T) {
	e := newSmokeEnv(t, smokeMatrix)
	e.withShimmedPATH(t)
	expFile := writeFile(t, t.TempDir(), "expected.json",
		`{"schema":1,"expected":[{"id":"DC-99","cases":["same"],"reason":"smoke"}]}`+"\n")
	code, stdout, stderr := e.invoke(t, "--expected", expFile)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "  expected: DC-99  0/1 red (stale)") {
		t.Errorf("stdout lacks the stale line:\n%s", stdout)
	}
}

// A harness-channel red (a capture-level failure, not a comparison
// divergence) is never annotated as expected: a matching entry must not hide
// a broken harness run behind exit 0.
func TestRunHarnessChannelRedIsNeverExpected(t *testing.T) {
	e := newSmokeEnv(t, smokeMatrix)
	e.withShimmedPATH(t)
	// A bad shebang passes the preflight executable check (mode bits) but
	// fails exec with a non-ExitError, so both cases go harness-channel red.
	writeExe(t, e.goBin, "#!/nonexistent/interpreter\n")
	expFile := writeFile(t, t.TempDir(), "expected.json",
		`{"schema":1,"expected":[{"id":"DC-99","cases":["*"],"reason":"smoke"}]}`+"\n")
	code, stdout, stderr := e.invoke(t, "--expected", expFile)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (stderr: %s)", code, stderr)
	}
	for _, sub := range []string{
		"RED     same/single/default/pipe/fixed  harness @0 (line 0):",
		"tudiff: 2 cases — 0 green, 2 red (0 expected, 2 unexpected), 0 timeout",
		"  expected: DC-99  2/2 red",
	} {
		if !strings.Contains(stdout, sub) {
			t.Errorf("stdout lacks %q:\n%s", sub, stdout)
		}
	}
	if strings.Contains(stdout, "[expected DC-99]") {
		t.Errorf("harness-channel red must not carry the expected marker:\n%s", stdout)
	}
}

// R3: --expected preflight — a missing file is its exact one-line message, an
// invalid file is one `tudiff: expected-diffs: …` line, both exit 2; --list
// never loads the file.
func TestRunExpectedPreflight(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		e := newSmokeEnv(t, smokeMatrix)
		code, _, stderr := e.invoke(t, "--expected", "/nonexistent")
		if code != 2 || stderr != "tudiff: /nonexistent not found\n" {
			t.Errorf("code = %d, stderr = %q", code, stderr)
		}
	})
	t.Run("invalid file", func(t *testing.T) {
		e := newSmokeEnv(t, smokeMatrix)
		bad := writeFile(t, t.TempDir(), "bad.json", `{"schema":2,"expected":[]}`+"\n")
		code, _, stderr := e.invoke(t, "--expected", bad)
		if code != 2 || stderr != "tudiff: expected-diffs: schema must be 1, got 2\n" {
			t.Errorf("code = %d, stderr = %q", code, stderr)
		}
	})
	t.Run("list does not load the file", func(t *testing.T) {
		e := newSmokeEnv(t, smokeMatrix)
		bad := writeFile(t, t.TempDir(), "bad.json", "not json\n")
		code, stdout, stderr := e.invoke(t, "--list", "--expected", bad)
		if code != 0 {
			t.Fatalf("exit = %d, stderr = %q", code, stderr)
		}
		want := "same/single/default/pipe/fixed\ndiff/single/default/pipe/fixed\n"
		if stdout != want {
			t.Errorf("stdout = %q, want %q", stdout, want)
		}
	})
	t.Run("default missing under the fake root", func(t *testing.T) {
		e := newSmokeEnv(t, smokeMatrix)
		if err := os.Remove(filepath.Join(e.root, "harness", "expected-diffs.json")); err != nil {
			t.Fatal(err)
		}
		code, _, stderr := e.invoke(t)
		if code != 2 || stderr != "tudiff: harness/expected-diffs.json not found\n" {
			t.Errorf("code = %d, stderr = %q", code, stderr)
		}
	})
}

// R5: a case that replayed an unconfirmed: true fixture fails the gate (exit
// 1) even with every case green.
func TestRunUnconfirmedExitsOne(t *testing.T) {
	e := newSmokeEnv(t, `{"schema":1,"cases":[{"id":"same","args":["ok"]}]}`)
	e.withShimmedPATH(t)
	writeFile(t, e.root, "harness/fixtures/_placeholder/manifest.json",
		`{"schema":1,"machine":"_placeholder","captured_at":"2026-09-16T00:00:00Z","ccusage_version":"20.0.19","ccusage_path":"","platform":"derived","timezone":"","fixtures":[{"source":"claude","period":"daily","args":["--json"],"file":"claude/daily.json","stderr_file":"","exit_code":0,"sha256":"","days":3,"first_date":"2026-01-05","last_date":"2026-01-07","empty":false,"redactions":0,"unconfirmed":true}]}`+"\n")
	writeExe(t, e.goBin, `#!/bin/sh
printf '%s\n' '{"tool":"ccusage","argv":["claude","daily","--json"],"cwd":"","matched":"_placeholder/claude/daily.json"}' >> "$TUDIFF_CALL_LOG"
printf 'hello\n'
`)
	code, stdout, stderr := e.invoke(t)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (stderr: %s)", code, stderr)
	}
	for _, sub := range []string{
		"GREEN   same/single/default/pipe/fixed [unconfirmed]",
		"tudiff: 1 cases — 1 green, 0 red (0 expected, 0 unexpected), 0 timeout   (fixtures: _placeholder; 1 cases replayed unconfirmed fixtures)",
	} {
		if !strings.Contains(stdout, sub) {
			t.Errorf("stdout lacks %q:\n%s", sub, stdout)
		}
	}
}
