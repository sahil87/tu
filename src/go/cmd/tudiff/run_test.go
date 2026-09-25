package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sahil87/tu/internal/harness"
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

// smokeNow is the pinned clock the smoke corpus's manifest carries.
const smokeNow = "2026-09-26T12:00:00"

// smokeEnv is a self-contained fake checkout for run tests: a repo root with
// justfile, tu.default.conf, a minimal _placeholder manifest and the seed
// dir; a stand-in --go shell script; stub ccusage/git fakes; a golden corpus
// under <root>/harness/golden whose manifest matches the matrix; and a report
// dir, all under temp space. The real harness/golden is never touched.
type smokeEnv struct {
	root       string
	matrix     string
	goBin      string
	harnessBin string
	golden     string
	report     string
}

const smokeMatrix = `{"schema":1,"cases":[
  {"id":"same","args":["ok"]},
  {"id":"diff","args":["nope"]}
]}`

// goStandIn is the default --go script: a --version line for ProbeVersion and
// per-arg stdout ("diff" diverges from its golden).
const goStandIn = `#!/bin/sh
if [ "${1:-}" = "--version" ]; then
  printf 'tu version v0.0.0-smoke\n'
elif [ "${1:-}" = "ok" ]; then
  printf 'hello\n'
else
  printf 'different\n'
fi
`

func newSmokeEnv(t *testing.T, matrixBody string) *smokeEnv {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "justfile", "go-build:\n\ttrue\n")
	writeFile(t, root, "tu.default.conf", "version = 2\n")
	writeFile(t, root, "harness/fixtures/_placeholder/manifest.json",
		`{"schema":1,"machine":"_placeholder","captured_at":"2026-09-16T00:00:00Z","ccusage_version":"20.0.19","ccusage_path":"","platform":"derived","timezone":"","fixtures":[]}`+"\n")
	writeFile(t, root, "harness/expected-diffs.json", "{\n  \"schema\": 1,\n  \"expected\": []\n}\n")
	writeFile(t, root, "harness/metrics-repo/docs/README.md", "seed\n")

	e := &smokeEnv{root: root}
	e.matrix = writeFile(t, t.TempDir(), "matrix.json", matrixBody)
	e.goBin = filepath.Join(t.TempDir(), "tu")
	writeExe(t, e.goBin, goStandIn)
	e.harnessBin = t.TempDir()
	writeExe(t, filepath.Join(e.harnessBin, "ccusage"), "#!/bin/sh\nexit 0\n")
	writeExe(t, filepath.Join(e.harnessBin, "git"), "#!/bin/sh\nexit 0\n")
	e.report = filepath.Join(t.TempDir(), "report")
	e.golden = filepath.Join(root, "harness", "golden")

	m, err := harness.LoadMatrix(e.matrix)
	if err != nil {
		t.Fatal(err)
	}
	e.writeGoldenManifest(t, len(harness.Expand(m)))
	for _, id := range []string{"same/single/default/pipe/fixed", "diff/single/default/pipe/fixed"} {
		e.writeGolden(t, id, "hello\n", "", 0)
	}
	return e
}

// writeGoldenManifest writes the corpus manifest matching the current matrix.
func (e *smokeEnv) writeGoldenManifest(t *testing.T, cases int) {
	t.Helper()
	sum, err := harness.MatrixSHA256(e.matrix)
	if err != nil {
		t.Fatal(err)
	}
	m := &harness.GoldenManifest{
		Schema:        harness.SchemaVersion,
		Oracle:        "bin/tu",
		OracleVersion: "v0.0.0-smoke",
		CapturedAt:    "2026-09-25T06:30:00Z",
		Now:           smokeNow,
		Script:        "util-linux",
		Platform:      "linux/amd64",
		Fixtures:      []string{harness.PlaceholderAlias},
		MatrixSHA256:  sum,
		Cases:         cases,
	}
	if err := harness.WriteGoldenManifest(e.golden, m); err != nil {
		t.Fatal(err)
	}
}

// writeGolden writes one case's pipe golden under the corpus.
func (e *smokeEnv) writeGolden(t *testing.T, id, stdout, stderr string, exit int) {
	t.Helper()
	cap := harness.SideCapture{Stdout: []byte(stdout), Stderr: []byte(stderr), Exit: exit}
	tree := harness.Tree{Files: map[string]harness.TreeFile{}}
	if err := harness.WriteGoldenCase(harness.GoldenCaseDir(e.golden, id), cap, tree); err != nil {
		t.Fatal(err)
	}
}

// invoke runs `run` through the top-level dispatcher with every path flag
// pinned at this environment, from the fake root as cwd.
func (e *smokeEnv) invoke(t *testing.T, extra ...string) (int, string, string) {
	t.Helper()
	t.Chdir(e.root)
	args := []string{"run",
		"--matrix", e.matrix,
		"--go", e.goBin,
		"--harness-bin", e.harnessBin,
		"--golden", e.golden,
		"--report", e.report,
	}
	args = append(args, extra...)
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// R3: end-to-end smoke — a stand-in Go binary against the goldens, one green
// and one red case in report.json, exit 1.
func TestRunEndToEndSmoke(t *testing.T) {
	e := newSmokeEnv(t, smokeMatrix)
	code, stdout, stderr := e.invoke(t)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (stderr: %s)", code, stderr)
	}
	for _, sub := range []string{
		"golden: " + e.golden + " (captured 2026-09-25T06:30:00Z from bin/tu v0.0.0-smoke; now " + smokeNow + ")",
		harness.IdentityNote,
		"GREEN   same/single/default/pipe/fixed",
		"RED     diff/single/default/pipe/fixed  stdout @0 (line 1): golden=\"hello\\n\" go=\"different\\n\"",
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
	for _, dropped := range []string{"node_ms", "node_calls", "calls_differ", `"node"`} {
		if strings.Contains(string(raw), dropped) {
			t.Errorf("report.json still carries %s", dropped)
		}
	}
	var doc struct {
		Header struct {
			Golden           string `json:"golden"`
			GoldenCapturedAt string `json:"golden_captured_at"`
		} `json:"header"`
		Summary struct {
			Green int `json:"green"`
			Red   int `json:"red"`
		} `json:"summary"`
		Cases []struct {
			ID            string `json:"id"`
			Status        string `json:"status"`
			Channel       string `json:"channel"`
			GoldenExcerpt string `json:"golden_excerpt"`
			GoldenExit    int    `json:"golden_exit"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Header.Golden != e.golden || doc.Header.GoldenCapturedAt != "2026-09-25T06:30:00Z" {
		t.Errorf("header = %+v", doc.Header)
	}
	if doc.Summary.Green != 1 || doc.Summary.Red != 1 {
		t.Errorf("summary = %+v", doc.Summary)
	}
	if len(doc.Cases) != 2 || doc.Cases[0].Status != "green" || doc.Cases[1].Status != "red" || doc.Cases[1].Channel != "stdout" {
		t.Errorf("cases = %+v", doc.Cases)
	}
	if doc.Cases[1].GoldenExcerpt != `"hello\n"` || doc.Cases[1].GoldenExit != 0 {
		t.Errorf("golden excerpt/exit = %q/%d", doc.Cases[1].GoldenExcerpt, doc.Cases[1].GoldenExit)
	}
	// The Go-side captures for the red case are inspectable; no oracle files.
	caseDir := filepath.Join(e.report, "cases", "diff", "single", "default", "pipe", "fixed")
	if _, err := os.Stat(filepath.Join(caseDir, "go.stdout")); err != nil {
		t.Errorf("missing go.stdout capture: %v", err)
	}
	if _, err := os.Stat(filepath.Join(caseDir, "node.stdout")); !os.IsNotExist(err) {
		t.Errorf("node.stdout capture exists in golden mode")
	}
}

// The date-rollover guard re-runs the Go side exactly once when the local
// date changes mid-case (moot under the pinned clock, but kept).
func TestRunDateRolloverRerun(t *testing.T) {
	e := newSmokeEnv(t, `{"schema":1,"cases":[{"id":"same","args":["ok"]}]}`)
	e.writeGoldenManifest(t, 1)
	if err := os.RemoveAll(harness.GoldenCaseDir(e.golden, "diff/single/default/pipe/fixed")); err != nil {
		t.Fatal(err)
	}

	countDir := t.TempDir()
	goCount := filepath.Join(countDir, "go.count")
	writeExe(t, e.goBin, "#!/bin/sh\nif [ \"${1:-}\" = \"--version\" ]; then printf 'tu version v0.0.0-smoke\\n'; exit 0; fi\necho go >>"+goCount+"\nprintf 'hello\\n'\n")

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
	raw, err := os.ReadFile(goCount)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(strings.Split(strings.TrimRight(string(raw), "\n"), "\n")); n != 2 {
		t.Errorf("go executions = %d, want 2", n)
	}
	docRaw, err := os.ReadFile(filepath.Join(e.report, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Cases []struct {
			Rerun bool `json:"rerun"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(docRaw, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Cases) != 1 || !doc.Cases[0].Rerun {
		t.Errorf("rerun not recorded: %+v", doc.Cases)
	}
}

// R3: --list prints the expanded, filtered IDs and exits 0 without touching
// the report dir — before any binary or golden check.
func TestRunList(t *testing.T) {
	e := newSmokeEnv(t, smokeMatrix)
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

// R3: every preflight failure is its exact one-line message and exit 2.
func TestRunPreflight(t *testing.T) {
	t.Run("fixtures and placeholder mutually exclusive", func(t *testing.T) {
		e := newSmokeEnv(t, smokeMatrix)
		code, _, stderr := e.invoke(t, "--fixtures", "a", "--placeholder")
		if code != 2 || stderr != "tudiff: --fixtures and --placeholder are mutually exclusive\n" {
			t.Errorf("code = %d, stderr = %q", code, stderr)
		}
	})
	t.Run("fixtures rejected in golden mode", func(t *testing.T) {
		e := newSmokeEnv(t, smokeMatrix)
		code, _, stderr := e.invoke(t, "--fixtures", "dev-ws-sahil02")
		if code != 2 || stderr != "tudiff: --fixtures needs an oracle; goldens are placeholder-only\n" {
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
	t.Run("golden manifest missing", func(t *testing.T) {
		e := newSmokeEnv(t, smokeMatrix)
		if err := os.RemoveAll(e.golden); err != nil {
			t.Fatal(err)
		}
		code, _, stderr := e.invoke(t)
		if code != 2 || stderr != "tudiff: "+e.golden+"/manifest.json not found (run tudiff run --update)\n" {
			t.Errorf("code = %d, stderr = %q", code, stderr)
		}
	})
	t.Run("golden manifest missing at the default dir names it as given", func(t *testing.T) {
		e := newSmokeEnv(t, smokeMatrix)
		if err := os.RemoveAll(e.golden); err != nil {
			t.Fatal(err)
		}
		t.Chdir(e.root)
		args := []string{"run", "--matrix", e.matrix, "--go", e.goBin,
			"--harness-bin", e.harnessBin, "--report", e.report}
		var stdout, stderr bytes.Buffer
		code := run(args, &stdout, &stderr)
		if code != 2 || stderr.String() != "tudiff: harness/golden/manifest.json not found (run tudiff run --update)\n" {
			t.Errorf("code = %d, stderr = %q", code, stderr.String())
		}
	})
	t.Run("golden manifest invalid", func(t *testing.T) {
		e := newSmokeEnv(t, smokeMatrix)
		writeFile(t, e.golden, "manifest.json", "not json\n")
		code, _, stderr := e.invoke(t)
		if code != 2 || !strings.HasPrefix(stderr, "tudiff: "+e.golden+"/manifest.json: ") {
			t.Errorf("code = %d, stderr = %q", code, stderr)
		}
	})
	t.Run("matrix changed since capture", func(t *testing.T) {
		e := newSmokeEnv(t, smokeMatrix)
		writeFile(t, filepath.Dir(e.matrix), filepath.Base(e.matrix),
			"{\"schema\":1,\"cases\":[{\"id\":\"same\",\"args\":[\"ok\"]}]}\n")
		code, _, stderr := e.invoke(t)
		if code != 2 || stderr != "tudiff: "+e.matrix+" changed since the goldens were captured (run tudiff run --update and review the diff)\n" {
			t.Errorf("code = %d, stderr = %q", code, stderr)
		}
	})
	t.Run("case without a golden", func(t *testing.T) {
		e := newSmokeEnv(t, smokeMatrix)
		if err := os.RemoveAll(harness.GoldenCaseDir(e.golden, "diff/single/default/pipe/fixed")); err != nil {
			t.Fatal(err)
		}
		code, _, stderr := e.invoke(t)
		if code != 2 || stderr != "tudiff: no golden for diff/single/default/pipe/fixed (run tudiff run --update)\n" {
			t.Errorf("code = %d, stderr = %q", code, stderr)
		}
	})
	t.Run("update with fixtures to a non-default golden is accepted", func(t *testing.T) {
		e := newSmokeEnv(t, smokeMatrix)
		writeFile(t, e.root, "harness/fixtures/bare/manifest.json",
			`{"schema":1,"machine":"bare","captured_at":"2026-09-16T00:00:00Z","ccusage_version":"20.0.19","ccusage_path":"","platform":"real","timezone":"","fixtures":[]}`+"\n")
		golden2 := filepath.Join(t.TempDir(), "golden")
		code, _, stderr := e.invoke(t, "--update", "--fixtures", "bare", "--golden", golden2)
		if code != 0 {
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
	t.Run("placeholder manifest absent", func(t *testing.T) {
		e := newSmokeEnv(t, smokeMatrix)
		if err := os.RemoveAll(filepath.Join(e.root, "harness", "fixtures")); err != nil {
			t.Fatal(err)
		}
		code, _, stderr := e.invoke(t)
		if code != 2 || stderr != "tudiff: harness/fixtures/_placeholder/manifest.json not found\n" {
			t.Errorf("code = %d, stderr = %q", code, stderr)
		}
	})
}

// script(1) absence is an error only when the filtered cases include a tty
// case.
func TestRunScriptGate(t *testing.T) {
	matrix := `{"schema":1,"cases":[
	  {"id":"same","args":["ok"]},
	  {"id":"via-tty","args":["ok"],"io":["pipe","tty"]}
	]}`

	t.Run("tty case without script", func(t *testing.T) {
		e := newSmokeEnv(t, matrix)
		e.writeGolden(t, "via-tty/single/default/pipe/fixed", "hello\n", "", 0)
		ttyCap := harness.SideCapture{TTY: []byte("hello\r\n"), Exit: 0}
		ttyTree := harness.Tree{Files: map[string]harness.TreeFile{}}
		if err := harness.WriteGoldenCase(harness.GoldenCaseDir(e.golden, "via-tty/single/default/tty/fixed"), ttyCap, ttyTree); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", t.TempDir())
		code, _, stderr := e.invoke(t)
		if code != 2 || stderr != "tudiff: script not found on PATH (required for tty cases)\n" {
			t.Errorf("code = %d, stderr = %q", code, stderr)
		}
	})
	t.Run("pipe-only filter runs without script", func(t *testing.T) {
		e := newSmokeEnv(t, matrix)
		t.Setenv("PATH", t.TempDir())
		code, _, stderr := e.invoke(t, "--filter", "same/")
		if code == 2 {
			t.Errorf("code = 2, stderr = %q", stderr)
		}
	})
}

// The gate rule: a red case matched by an expected-diffs entry passes (exit 0)
// and carries the [expected <id>] marker.
func TestRunExpectedRedExitsZero(t *testing.T) {
	e := newSmokeEnv(t, smokeMatrix)
	expFile := writeFile(t, t.TempDir(), "expected.json",
		`{"schema":1,"expected":[{"id":"DC-99","cases":["diff"],"reason":"smoke"}]}`+"\n")
	code, stdout, stderr := e.invoke(t, "--expected", expFile)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	for _, sub := range []string{
		"RED     diff/single/default/pipe/fixed  stdout @0 (line 1): golden=\"hello\\n\" go=\"different\\n\" [expected DC-99]",
		"tudiff: 2 cases — 1 green, 1 red (1 expected, 0 unexpected), 0 timeout",
		"  expected: DC-99  1/1 red",
	} {
		if !strings.Contains(stdout, sub) {
			t.Errorf("stdout lacks %q:\n%s", sub, stdout)
		}
	}
}

// An entry whose matched cases are all green is stale and fails the gate.
func TestRunStaleExpectedExitsOne(t *testing.T) {
	e := newSmokeEnv(t, smokeMatrix)
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

// --expected preflight: a missing file is its exact one-line message, an
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

// A case that replayed an unconfirmed: true fixture fails the gate (exit 1)
// even with every case green.
func TestRunUnconfirmedExitsOne(t *testing.T) {
	e := newSmokeEnv(t, `{"schema":1,"cases":[{"id":"same","args":["ok"]}]}`)
	e.writeGoldenManifest(t, 1)
	if err := os.RemoveAll(harness.GoldenCaseDir(e.golden, "diff/single/default/pipe/fixed")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, e.root, "harness/fixtures/_placeholder/manifest.json",
		`{"schema":1,"machine":"_placeholder","captured_at":"2026-09-16T00:00:00Z","ccusage_version":"20.0.19","ccusage_path":"","platform":"derived","timezone":"","fixtures":[{"source":"claude","period":"daily","args":["--json"],"file":"claude/daily.json","stderr_file":"","exit_code":0,"sha256":"","days":3,"first_date":"2026-01-05","last_date":"2026-01-07","empty":false,"redactions":0,"unconfirmed":true}]}`+"\n")
	writeExe(t, e.goBin, `#!/bin/sh
if [ "${1:-}" = "--version" ]; then printf 'tu version v0.0.0-smoke\n'; exit 0; fi
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

// R3: --update runs the Go side and writes the goldens plus a fresh manifest;
// the version channel is $VERSION-normalized.
func TestRunUpdateWritesGoldens(t *testing.T) {
	matrix := `{"schema":1,"cases":[
	  {"id":"same","args":["ok"]},
	  {"id":"ver","args":["--version"]}
	]}`
	e := newSmokeEnv(t, matrix)
	golden2 := filepath.Join(t.TempDir(), "golden")
	code, stdout, stderr := e.invoke(t, "--golden", golden2, "--update", "--now", "2026-09-20T12:00:00")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if stdout != "tudiff: wrote 2 goldens under "+golden2+"/run (now 2026-09-20T12:00:00)\n" {
		t.Errorf("stdout = %q", stdout)
	}

	m, err := harness.LoadGoldenManifest(golden2)
	if err != nil {
		t.Fatal(err)
	}
	sum, err := harness.MatrixSHA256(e.matrix)
	if err != nil {
		t.Fatal(err)
	}
	if m.Now != "2026-09-20T12:00:00" || m.Oracle != e.goBin || m.OracleVersion != "v0.0.0-smoke" ||
		m.NodeVersion != "" || m.MatrixSHA256 != sum || m.Cases != 2 || m.CapturedAt == "" {
		t.Errorf("manifest = %+v", m)
	}
	if len(m.Fixtures) != 1 || m.Fixtures[0] != harness.PlaceholderAlias {
		t.Errorf("fixtures = %v", m.Fixtures)
	}

	dir := harness.GoldenCaseDir(golden2, "same/single/default/pipe/fixed")
	for name, want := range map[string]string{"stdout": "hello\n", "stderr": "", "exit": "0\n"} {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(raw) != want {
			t.Errorf("%s = %q, %v; want %q", name, raw, err, want)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "tree.json")); err != nil {
		t.Errorf("missing tree.json: %v", err)
	}
	// The --version golden holds $VERSION, not the probed string.
	raw, err := os.ReadFile(filepath.Join(harness.GoldenCaseDir(golden2, "ver/single/default/pipe/fixed"), "stdout"))
	if err != nil || string(raw) != "tu version $VERSION\n" {
		t.Errorf("version golden = %q, %v", raw, err)
	}
}

// R3: a corpus written by --update compares green against the same binary —
// the manifest round-trip through run, TUDIFF_NOW included.
func TestRunUpdateThenCompareGreen(t *testing.T) {
	e := newSmokeEnv(t, smokeMatrix)
	golden2 := filepath.Join(t.TempDir(), "golden")
	if code, _, stderr := e.invoke(t, "--golden", golden2, "--update", "--now", smokeNow); code != 0 {
		t.Fatalf("--update exit = %d, stderr = %q", code, stderr)
	}
	code, stdout, stderr := e.invoke(t, "--golden", golden2)
	if code != 0 {
		t.Fatalf("compare exit = %d, want 0 (stderr: %s)\n%s", code, stderr, stdout)
	}
	for _, sub := range []string{
		"golden: " + golden2 + " (captured ",
		"from " + e.goBin + " v0.0.0-smoke; now " + smokeNow + ")",
		"tudiff: 2 cases — 2 green, 0 red (0 expected, 0 unexpected), 0 timeout",
	} {
		if !strings.Contains(stdout, sub) {
			t.Errorf("stdout lacks %q:\n%s", sub, stdout)
		}
	}
}

// R3: the pinned clock reaches the child as TUDIFF_NOW, in --update and in
// compare mode alike.
func TestRunPinnedClockReachesChild(t *testing.T) {
	e := newSmokeEnv(t, `{"schema":1,"cases":[{"id":"same","args":["ok"]}]}`)
	writeExe(t, e.goBin, `#!/bin/sh
if [ "${1:-}" = "--version" ]; then printf 'tu version v0.0.0-smoke\n'; exit 0; fi
printf '%s\n' "${TUDIFF_NOW:-unset}"
`)
	golden2 := filepath.Join(t.TempDir(), "golden")
	if code, _, stderr := e.invoke(t, "--golden", golden2, "--update", "--now", smokeNow); code != 0 {
		t.Fatalf("--update exit = %d, stderr = %q", code, stderr)
	}
	raw, err := os.ReadFile(filepath.Join(harness.GoldenCaseDir(golden2, "same/single/default/pipe/fixed"), "stdout"))
	if err != nil || string(raw) != smokeNow+"\n" {
		t.Fatalf("captured stdout = %q, %v; want the pinned clock", raw, err)
	}
	if code, stdout, stderr := e.invoke(t, "--golden", golden2); code != 0 {
		t.Errorf("compare exit = %d, stderr = %q\n%s", code, stderr, stdout)
	}
}

// R3: --update keeps the existing manifest's now unless --now overrides it,
// and a filtered --update rewrites only the matched cases.
func TestRunUpdateNowAndFilter(t *testing.T) {
	t.Run("now preserved from the existing manifest", func(t *testing.T) {
		e := newSmokeEnv(t, smokeMatrix)
		if code, _, stderr := e.invoke(t, "--update"); code != 0 {
			t.Fatalf("exit = %d, stderr = %q", code, stderr)
		}
		m, err := harness.LoadGoldenManifest(e.golden)
		if err != nil {
			t.Fatal(err)
		}
		if m.Now != smokeNow {
			t.Errorf("now = %q, want preserved %q", m.Now, smokeNow)
		}
	})
	t.Run("filtered update writes only the matched cases", func(t *testing.T) {
		e := newSmokeEnv(t, smokeMatrix)
		golden2 := filepath.Join(t.TempDir(), "golden")
		code, stdout, stderr := e.invoke(t, "--golden", golden2, "--update", "--filter", "same/", "--now", smokeNow)
		if code != 0 {
			t.Fatalf("exit = %d, stderr = %q", code, stderr)
		}
		if stdout != "tudiff: wrote 1 goldens under "+golden2+"/run (now "+smokeNow+")\n" {
			t.Errorf("stdout = %q", stdout)
		}
		if _, err := os.Stat(harness.GoldenCaseDir(golden2, "same/single/default/pipe/fixed")); err != nil {
			t.Errorf("matched case golden missing: %v", err)
		}
		if _, err := os.Stat(harness.GoldenCaseDir(golden2, "diff/single/default/pipe/fixed")); !os.IsNotExist(err) {
			t.Errorf("unmatched case golden written")
		}
		m, err := harness.LoadGoldenManifest(golden2)
		if err != nil {
			t.Fatal(err)
		}
		if m.Cases != 1 {
			t.Errorf("manifest cases = %d, want 1", m.Cases)
		}
	})
}

// R2: --update identity-normalizes the byte channels (and the compare run
// against such a golden is green on the same machine).
func TestRunUpdateNormalizesIdentity(t *testing.T) {
	e := newSmokeEnv(t, `{"schema":1,"cases":[{"id":"same","args":["ok"]}]}`)
	writeExe(t, e.goBin, `#!/bin/sh
if [ "${1:-}" = "--version" ]; then printf 'tu version v0.0.0-smoke\n'; exit 0; fi
printf '%s/2026/%s/cc.jsonl machine_%s_cost\n' "$(id -un)" "$(hostname)" "$(hostname)"
`)
	golden2 := filepath.Join(t.TempDir(), "golden")
	if code, _, stderr := e.invoke(t, "--golden", golden2, "--update", "--now", smokeNow); code != 0 {
		t.Fatalf("--update exit = %d, stderr = %q", code, stderr)
	}
	raw, err := os.ReadFile(filepath.Join(harness.GoldenCaseDir(golden2, "same/single/default/pipe/fixed"), "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(raw), "$USER/2026/$MACHINE/cc.jsonl machine_$MACHINE_cost\n"; got != want {
		t.Errorf("golden stdout = %q, want %q", got, want)
	}
	if code, stdout, stderr := e.invoke(t, "--golden", golden2); code != 0 {
		t.Errorf("compare exit = %d, stderr = %q\n%s", code, stderr, stdout)
	}
}
