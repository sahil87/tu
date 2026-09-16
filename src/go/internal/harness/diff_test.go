package harness

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func baseCase() Case {
	return Case{ID: "x/single/default/pipe/fixed", Group: "x", Conf: ConfSingle, Env: EnvDefault, IO: IOPipe, TZ: TZFixed}
}

func baseSpec(home string) EnvSpec {
	return EnvSpec{
		HarnessBin: "/abs/harness-bin",
		Home:       home,
		Fixtures:   []string{"/abs/fixtures/_placeholder"},
		CallLog:    "/abs/report/cases/x/node.calls.jsonl",
	}
}

// R9: the child environment is exactly the specified variables per axis —
// nothing inherited.
func TestBuildEnvExact(t *testing.T) {
	env := BuildEnv(baseCase(), baseSpec("/abs/home"))
	want := []string{
		"PATH=/abs/harness-bin:" + os.Getenv("PATH"),
		"HOME=/abs/home",
		"TZ=UTC",
		"LANG=C.UTF-8",
		"LC_ALL=C.UTF-8",
		"TUDIFF_FIXTURES=/abs/fixtures/_placeholder",
		"TUDIFF_CALL_LOG=/abs/report/cases/x/node.calls.jsonl",
	}
	if strings.Join(env, "\n") != strings.Join(want, "\n") {
		t.Errorf("env =\n%s\nwant =\n%s", strings.Join(env, "\n"), strings.Join(want, "\n"))
	}
}

// R9: exported TU_METRICS_REPO / NO_COLOR in the harness process must not
// leak into a default-env case.
func TestBuildEnvNoLeak(t *testing.T) {
	t.Setenv("TU_METRICS_REPO", "git@example.invalid:leak/x.git")
	t.Setenv("NO_COLOR", "1")
	for _, kv := range BuildEnv(baseCase(), baseSpec("/h")) {
		if strings.HasPrefix(kv, "TU_METRICS_REPO=") || strings.HasPrefix(kv, "NO_COLOR=") {
			t.Errorf("leaked %q into default-env case", kv)
		}
	}
}

func TestBuildEnvAxes(t *testing.T) {
	has := func(env []string, key string) (string, bool) {
		for _, kv := range env {
			if strings.HasPrefix(kv, key+"=") {
				return strings.TrimPrefix(kv, key+"="), true
			}
		}
		return "", false
	}
	c := baseCase()
	c.IO, c.Env, c.TZ = IOTTY, EnvEnvrepo, TZAlt
	env := BuildEnv(c, baseSpec("/h"))
	if v, ok := has(env, "TERM"); !ok || v != "xterm-256color" {
		t.Errorf("TERM = %q, %v", v, ok)
	}
	if v, ok := has(env, "TZ"); !ok || v != TZAltName {
		t.Errorf("TZ = %q, %v", v, ok)
	}
	if v, ok := has(env, "TU_METRICS_REPO"); !ok || v != MetricsRepoURL {
		t.Errorf("TU_METRICS_REPO = %q, %v", v, ok)
	}
	if _, ok := has(env, "NO_COLOR"); ok {
		t.Errorf("NO_COLOR present in envrepo case")
	}

	c.Env = EnvNoColor
	env = BuildEnv(c, baseSpec("/h"))
	if v, ok := has(env, "NO_COLOR"); !ok || v != "1" {
		t.Errorf("NO_COLOR = %q, %v", v, ok)
	}
	if _, ok := has(env, "TU_METRICS_REPO"); ok {
		t.Errorf("TU_METRICS_REPO present in nocolor case")
	}
}

// R11: the sentinel line and its preceding line break are parsed off the
// transcript; a missing sentinel is Exit -1 + "no exit sentinel".
func TestParseTTYExit(t *testing.T) {
	tty, exit, err := parseTTYExit([]byte("line one\r\n…table\r\n\r\n__TUDIFF_EXIT=2\r\n"))
	if exit != 2 || err != "" {
		t.Errorf("exit = %d, err = %q", exit, err)
	}
	if string(tty) != "line one\r\n…table\r\n" {
		t.Errorf("tty = %q", tty)
	}

	tty, exit, err = parseTTYExit([]byte("out\n\n__TUDIFF_EXIT=0\n"))
	if exit != 0 || err != "" || string(tty) != "out\n" {
		t.Errorf("LF form: exit = %d, err = %q, tty = %q", exit, err, tty)
	}

	_, exit, err = parseTTYExit([]byte("no sentinel here\r\n"))
	if exit != -1 || err != "no exit sentinel" {
		t.Errorf("missing: exit = %d, err = %q", exit, err)
	}
}

func TestCompareGreen(t *testing.T) {
	cap := SideCapture{Stdout: []byte("same\n"), Stderr: []byte("warn\n"), Exit: 0}
	r := Compare(baseCase(), cap, cap)
	if r.Status != StatusGreen {
		t.Errorf("status = %q", r.Status)
	}
}

// R13 (B1): NormalizeHome replaces every occurrence of the staged home with
// the literal "$HOME".
func TestNormalizeHome(t *testing.T) {
	cases := []struct {
		name string
		in   string
		home string
		want string
	}{
		{"no occurrence", "no path here\n", "/tmp/c/node/home", "no path here\n"},
		{"multiple occurrences", "/h/.tu/a then /h/.tu/b\n", "/h", "$HOME/.tu/a then $HOME/.tu/b\n"},
		{"prefix inside a longer path", "/h/.tu/metrics_repo/x\n", "/h", "$HOME/.tu/metrics_repo/x\n"},
		{"empty home is a no-op", "/h/a\n", "", "/h/a\n"},
		{"empty input", "", "/h", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := NormalizeHome([]byte(c.in), c.home); string(got) != c.want {
				t.Errorf("NormalizeHome(%q, %q) = %q, want %q", c.in, c.home, got, c.want)
			}
		})
	}
}

// R13: exit is the first compared channel.
func TestCompareExitFirst(t *testing.T) {
	node := SideCapture{Stdout: []byte("a"), Stderr: []byte("b"), Exit: 0}
	goCap := SideCapture{Stdout: []byte("DIFFERENT"), Stderr: []byte("b"), Exit: 1}
	r := Compare(baseCase(), node, goCap)
	if r.Status != StatusRed || r.Channel != "exit" || r.NodeExit != 0 || r.GoExit != 1 {
		t.Errorf("result = %+v", r)
	}
}

// R13: a byte divergence reports offset, 1-based line, and quoted excerpts.
func TestCompareStdoutDivergence(t *testing.T) {
	node := SideCapture{Stdout: []byte("Usage: tu\n"), Exit: 0}
	goCap := SideCapture{Stdout: []byte(""), Exit: 0}
	r := Compare(baseCase(), node, goCap)
	if r.Status != StatusRed || r.Channel != "stdout" {
		t.Fatalf("result = %+v", r)
	}
	if r.Offset != 0 || r.Line != 1 {
		t.Errorf("offset/line = %d/%d", r.Offset, r.Line)
	}
	if r.NodeExcerpt != `"Usage: tu\n"` || r.GoExcerpt != `""` {
		t.Errorf("excerpts = %q / %q", r.NodeExcerpt, r.GoExcerpt)
	}
}

func TestCompareStderrAndLineCount(t *testing.T) {
	node := SideCapture{Stdout: []byte("same\n"), Stderr: []byte("l1\nl2\nl3-diverges\n"), Exit: 0}
	goCap := SideCapture{Stdout: []byte("same\n"), Stderr: []byte("l1\nl2\nl3-is-different\n"), Exit: 0}
	r := Compare(baseCase(), node, goCap)
	if r.Channel != "stderr" || r.Offset != len("l1\nl2\nl3-") || r.Line != 3 {
		t.Errorf("channel/offset/line = %q/%d/%d", r.Channel, r.Offset, r.Line)
	}
}

func TestCompareTTYChannel(t *testing.T) {
	c := baseCase()
	c.IO = IOTTY
	node := SideCapture{TTY: []byte("table\r\n"), Exit: 0}
	goCap := SideCapture{TTY: []byte("tableX\r\n"), Exit: 0}
	r := Compare(c, node, goCap)
	if r.Status != StatusRed || r.Channel != "tty" || r.Offset != 5 {
		t.Errorf("result = %+v", r)
	}
}

func TestCompareTimeout(t *testing.T) {
	node := SideCapture{Exit: 0}
	goCap := SideCapture{TimedOut: true, Exit: -1}
	r := Compare(baseCase(), node, goCap)
	if r.Status != StatusTimeout || r.Channel != "timeout" {
		t.Errorf("result = %+v", r)
	}
}

// R13: the call-log comparison is informational — a difference never changes
// a green verdict.
func TestCompareCallsInformational(t *testing.T) {
	dir := t.TempDir()
	nodeLog := filepath.Join(dir, "node.calls.jsonl")
	goLog := filepath.Join(dir, "go.calls.jsonl")
	lines := []string{
		`{"tool":"ccusage","argv":["claude","daily","--json"],"cwd":"/a"}`,
		`{"tool":"git","argv":["push"],"cwd":"/a"}`,
	}
	if err := os.WriteFile(nodeLog, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Same calls, reversed order, different cwd — must compare equal.
	reversed := []string{
		`{"tool":"git","argv":["push"],"cwd":"/b"}`,
		`{"tool":"ccusage","argv":["claude","daily","--json"],"cwd":"/b"}`,
	}
	if err := os.WriteFile(goLog, []byte(strings.Join(reversed, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nodeN, goN, differ, err := CompareCallLogs(nodeLog, goLog, "", "")
	if err != nil || differ || nodeN != 2 || goN != 2 {
		t.Errorf("CompareCallLogs = %d/%d differ=%v err=%v", nodeN, goN, differ, err)
	}

	// Six calls vs none: differ, but the case status stays green.
	six := strings.Repeat(lines[0]+"\n", 6)
	if err := os.WriteFile(nodeLog, []byte(six), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(goLog); err != nil {
		t.Fatal(err)
	}
	nodeN, goN, differ, err = CompareCallLogs(nodeLog, goLog, "", "")
	if err != nil || !differ || nodeN != 6 || goN != 0 {
		t.Errorf("CompareCallLogs = %d/%d differ=%v err=%v", nodeN, goN, differ, err)
	}
	cap := SideCapture{Stdout: []byte("x"), Exit: 0}
	r := Compare(baseCase(), cap, cap)
	r.CallsDiffer = differ
	r.NodeCalls, r.GoCalls = nodeN, goN
	if r.Status != StatusGreen || !r.CallsDiffer {
		t.Errorf("status = %q, CallsDiffer = %v", r.Status, r.CallsDiffer)
	}
}

// R10: a deadline-bound pipe run reports the timeout.
func TestRunPipeTimeout(t *testing.T) {
	c := RunPipe("sh", []string{"-c", "sleep 2"}, t.TempDir(), nil, 50*time.Millisecond)
	if !c.TimedOut || c.Exit != -1 {
		t.Errorf("capture = %+v", c)
	}
}

func TestRunPipeCapturesSeparately(t *testing.T) {
	c := RunPipe("sh", []string{"-c", "printf out; printf err >&2; exit 3"}, t.TempDir(), nil, 5*time.Second)
	if c.TimedOut || c.Exit != 3 || string(c.Stdout) != "out" || string(c.Stderr) != "err" {
		t.Errorf("capture = %+v", c)
	}
}

// R11: an end-to-end tty run under script(1) recovers the exit code and the
// merged transcript. Skipped where script is absent.
func TestRunTTYSmoke(t *testing.T) {
	flavor := ScriptFlavor()
	if flavor == "" {
		t.Skip("script(1) not on PATH")
	}
	scriptPath, err := exec.LookPath("script")
	if err != nil {
		t.Skip("script(1) not on PATH")
	}
	c := RunTTY(scriptPath, "sh", []string{"-c", "echo tty-hi; exit 3"}, t.TempDir(), os.Environ(), 10*time.Second, flavor)
	if c.TimedOut {
		t.Fatalf("timed out: %+v", c)
	}
	if c.Exit != 3 || c.Err != "" {
		t.Errorf("exit = %d, err = %q", c.Exit, c.Err)
	}
	if !strings.Contains(string(c.TTY), "tty-hi") {
		t.Errorf("tty = %q", c.TTY)
	}
	if strings.Contains(string(c.TTY), ExitSentinel) {
		t.Errorf("sentinel left in transcript: %q", c.TTY)
	}
}

// R8: the oracle is staged as copies beside a default conf, with the fake in
// the vendor slot at 0755.
func TestStageOracle(t *testing.T) {
	src := t.TempDir()
	for name, body := range map[string]string{"tu.mjs": "bundle\n", "tu.default.conf": "conf\n", "ccusage": "fake\n"} {
		if err := os.WriteFile(filepath.Join(src, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tmp := t.TempDir()
	bundle, err := StageOracle(tmp, filepath.Join(src, "tu.mjs"), filepath.Join(src, "tu.default.conf"), filepath.Join(src, "ccusage"))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.ToSlash(bundle) != filepath.ToSlash(filepath.Join(tmp, "oracle", "dist", "tu.mjs")) {
		t.Errorf("bundle = %q", bundle)
	}
	vendor := filepath.Join(tmp, "oracle", "dist", "vendor", "ccusage", "bin", "ccusage")
	info, err := os.Stat(vendor)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("vendor mode = %o", info.Mode().Perm())
	}
	raw, err := os.ReadFile(vendor)
	if err != nil || string(raw) != "fake\n" {
		t.Errorf("vendor bytes = %q, %v", raw, err)
	}
	// Sources untouched.
	for _, name := range []string{"tu.mjs", "tu.default.conf", "ccusage"} {
		if raw, err := os.ReadFile(filepath.Join(src, name)); err != nil || len(raw) == 0 {
			t.Errorf("source %s modified", name)
		}
	}
}

// R13 (B1): captures that differ only in the staged home path compare green,
// and divergence details are computed on the normalized bytes.
func TestCompareHomeNormalized(t *testing.T) {
	node := SideCapture{
		Stdout: []byte("Already initialized: /tmp/c/node/home/.tu/metrics_repo\n"),
		Exit:   0,
		Home:   "/tmp/c/node/home",
	}
	goCap := SideCapture{
		Stdout: []byte("Already initialized: /tmp/c/go/home/.tu/metrics_repo\n"),
		Exit:   0,
		Home:   "/tmp/c/go/home",
	}
	r := Compare(baseCase(), node, goCap)
	if r.Status != StatusGreen {
		t.Errorf("status = %q, want green (home-only difference)", r.Status)
	}

	// A real difference past the home is still red, with offset and excerpts
	// computed on the normalized bytes.
	goCap.Stdout = []byte("Already initialized: /tmp/c/go/home/.tu/other\n")
	r = Compare(baseCase(), node, goCap)
	if r.Status != StatusRed || r.Channel != "stdout" {
		t.Fatalf("result = %+v", r)
	}
	if r.Offset != len("Already initialized: $HOME/.tu/") {
		t.Errorf("offset = %d, want %d (normalized bytes)", r.Offset, len("Already initialized: $HOME/.tu/"))
	}
	if r.NodeExcerpt != `"metrics_repo\n"` || r.GoExcerpt != `"other\n"` {
		t.Errorf("excerpts = %q / %q", r.NodeExcerpt, r.GoExcerpt)
	}
}

// R13 (B1): call-log argv that differ only by the two staged homes do not
// differ.
func TestCompareCallLogsHomeNormalized(t *testing.T) {
	dir := t.TempDir()
	nodeLog := filepath.Join(dir, "node.calls.jsonl")
	goLog := filepath.Join(dir, "go.calls.jsonl")
	nodeLine := `{"tool":"git","argv":["-C","/tmp/c/node/home/.tu/metrics_repo","rev-parse","--git-dir"],"cwd":"/a"}` + "\n"
	goLine := `{"tool":"git","argv":["-C","/tmp/c/go/home/.tu/metrics_repo","rev-parse","--git-dir"],"cwd":"/b"}` + "\n"
	if err := os.WriteFile(nodeLog, []byte(nodeLine), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(goLog, []byte(goLine), 0o644); err != nil {
		t.Fatal(err)
	}
	nodeN, goN, differ, err := CompareCallLogs(nodeLog, goLog, "/tmp/c/node/home", "/tmp/c/go/home")
	if err != nil || differ || nodeN != 1 || goN != 1 {
		t.Errorf("CompareCallLogs = %d/%d differ=%v err=%v, want 1/1 differ=false", nodeN, goN, differ, err)
	}

	// A real argv difference still differs under normalization.
	goLine = `{"tool":"git","argv":["-C","/tmp/c/go/home/.tu/other","rev-parse","--git-dir"],"cwd":"/b"}` + "\n"
	if err := os.WriteFile(goLog, []byte(goLine), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, differ, err = CompareCallLogs(nodeLog, goLog, "/tmp/c/node/home", "/tmp/c/go/home")
	if err != nil || !differ {
		t.Errorf("CompareCallLogs differ=%v err=%v, want differ=true", differ, err)
	}
}
