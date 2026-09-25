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

// The golden manifest's pinned clock rides the child environment as
// TUDIFF_NOW; an empty Now leaves the variable unset (real clock).
func TestBuildEnvNow(t *testing.T) {
	spec := baseSpec("/abs/home")
	spec.Now = "2026-09-26T12:00:00"
	env := BuildEnv(baseCase(), spec)
	want := append(append([]string(nil), BuildEnv(baseCase(), baseSpec("/abs/home"))...),
		"TUDIFF_NOW=2026-09-26T12:00:00")
	if strings.Join(env, "\n") != strings.Join(want, "\n") {
		t.Errorf("env =\n%s\nwant =\n%s", strings.Join(env, "\n"), strings.Join(want, "\n"))
	}
	for _, kv := range BuildEnv(baseCase(), baseSpec("/abs/home")) {
		if strings.HasPrefix(kv, "TUDIFF_NOW=") {
			t.Errorf("TUDIFF_NOW present with an empty Now: %q", kv)
		}
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

// R13: the failure-injection env values are the default env plus exactly one
// TUDIFF_GIT_SCRIPT variable with the byte-contract rule set.
func TestBuildEnvGitScript(t *testing.T) {
	scripts := map[string]string{
		EnvPullfail: `[{"match":["pull"],"stderr":"fatal: couldn't find remote ref main\n","exit":1}]`,
		EnvPushfail: `[{"match":["push"],"stderr":"error: failed to push some refs\n","exit":1}]`,
		EnvDirty:    `[{"match":["status","--porcelain"],"stdout":" M harness-user/x\n","exit":0}]`,
	}
	base := BuildEnv(baseCase(), baseSpec("/h"))
	for envValue, script := range scripts {
		c := baseCase()
		c.Env = envValue
		got := BuildEnv(c, baseSpec("/h"))
		want := append(append([]string(nil), base...), "TUDIFF_GIT_SCRIPT="+script)
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("%s env =\n%s\nwant =\n%s", envValue, strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
	}
	// The base values never set the variable.
	for _, envValue := range []string{EnvDefault, EnvNoColor, EnvEnvrepo} {
		c := baseCase()
		c.Env = envValue
		for _, kv := range BuildEnv(c, baseSpec("/h")) {
			if strings.HasPrefix(kv, "TUDIFF_GIT_SCRIPT=") {
				t.Errorf("TUDIFF_GIT_SCRIPT present in %s case", envValue)
			}
		}
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

// CountCalls backs report.json's go_calls: the number of logged invocations,
// a missing log counting as empty.
func TestCountCalls(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "go.calls.jsonl")
	lines := []string{
		`{"tool":"ccusage","argv":["claude","daily","--json"],"cwd":"/a"}`,
		`{"tool":"git","argv":["push"],"cwd":"/a"}`,
	}
	if err := os.WriteFile(log, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if n, err := CountCalls(log); err != nil || n != 2 {
		t.Errorf("CountCalls = %d, %v; want 2", n, err)
	}
	if n, err := CountCalls(filepath.Join(dir, "absent")); err != nil || n != 0 {
		t.Errorf("CountCalls (missing log) = %d, %v; want 0", n, err)
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

// writeTreeFile writes body at <root>/<rel> (slash-separated), creating
// parent directories.
func writeTreeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// stageTreePair stages two homes whose .tu/metrics_repo each hold the given
// files (same content on both sides).
func stageTreePair(t *testing.T, files map[string]string) (nodeHome, goHome string) {
	t.Helper()
	root := t.TempDir()
	nodeHome = filepath.Join(root, "node")
	goHome = filepath.Join(root, "go")
	for rel, body := range files {
		writeTreeFile(t, nodeHome, ".tu/metrics_repo/"+rel, body)
		writeTreeFile(t, goHome, ".tu/metrics_repo/"+rel, body)
	}
	return nodeHome, goHome
}

// R12: identical trees and matching .last-sync presence stay green.
func TestCompareTreesGreen(t *testing.T) {
	files := map[string]string{
		"harness-user/2026/harness-machine/cc-2026-01-06.jsonl": "{\"label\":\"2026-01-06\"}\n",
		"docs/README.md": "seed\n",
	}
	nodeHome, goHome := stageTreePair(t, files)
	writeTreeFile(t, nodeHome, ".tu/.last-sync", "2026-09-17T00:00:00Z\n")
	writeTreeFile(t, goHome, ".tu/.last-sync", "2026-09-17T00:00:01Z\n") // content never compared
	diff, err := CompareTrees(nodeHome, goHome)
	if err != nil || diff != nil {
		t.Errorf("CompareTrees = %+v, %v, want nil", diff, err)
	}

	// A metrics_repo missing on both sides is equal too.
	bare1, bare2 := filepath.Join(t.TempDir(), "a"), filepath.Join(t.TempDir(), "b")
	diff, err = CompareTrees(bare1, bare2)
	if err != nil || diff != nil {
		t.Errorf("CompareTrees (both missing) = %+v, %v, want nil", diff, err)
	}
}

// R12: an extra file on one side is a tree difference naming that path.
func TestCompareTreesExtraFile(t *testing.T) {
	files := map[string]string{"docs/README.md": "seed\n"}
	nodeHome, goHome := stageTreePair(t, files)
	writeTreeFile(t, goHome, ".tu/metrics_repo/harness-user/2026/harness-machine/cc-2026-01-07.jsonl", "{}\n")
	diff, err := CompareTrees(nodeHome, goHome)
	if err != nil {
		t.Fatal(err)
	}
	if diff == nil || diff.Path != "harness-user/2026/harness-machine/cc-2026-01-07.jsonl" {
		t.Fatalf("diff = %+v", diff)
	}
	if diff.NodeExcerpt != diff.Path+": absent" || diff.GoExcerpt != diff.Path+": present" {
		t.Errorf("excerpts = %q / %q", diff.NodeExcerpt, diff.GoExcerpt)
	}
}

// R12: a byte difference in a shared file is a tree difference naming the
// file, with a byte excerpt from each side.
func TestCompareTreesByteDiff(t *testing.T) {
	rel := "harness-user/2026/harness-machine/cc-2026-01-06.jsonl"
	nodeHome, goHome := stageTreePair(t, map[string]string{rel: "{\"totalCost\":1.00}\n"})
	writeTreeFile(t, goHome, ".tu/metrics_repo/"+rel, "{\"totalCost\":2.00}\n")
	diff, err := CompareTrees(nodeHome, goHome)
	if err != nil {
		t.Fatal(err)
	}
	if diff == nil || diff.Path != rel {
		t.Fatalf("diff = %+v", diff)
	}
	wantNode := rel + ": " + `"1.00}\n"`
	wantGo := rel + ": " + `"2.00}\n"`
	if diff.NodeExcerpt != wantNode || diff.GoExcerpt != wantGo {
		t.Errorf("excerpts = %q / %q, want %q / %q", diff.NodeExcerpt, diff.GoExcerpt, wantNode, wantGo)
	}
}

// R12: a .last-sync presence mismatch is a tree difference naming it; a
// metrics_repo missing on one side only is a difference too.
func TestCompareTreesPresence(t *testing.T) {
	nodeHome, goHome := stageTreePair(t, map[string]string{"docs/README.md": "seed\n"})
	writeTreeFile(t, nodeHome, ".tu/.last-sync", "2026-09-17T00:00:00Z\n")
	diff, err := CompareTrees(nodeHome, goHome)
	if err != nil {
		t.Fatal(err)
	}
	if diff == nil || diff.Path != ".last-sync" {
		t.Fatalf("diff = %+v", diff)
	}
	if diff.NodeExcerpt != ".last-sync: present" || diff.GoExcerpt != ".last-sync: absent" {
		t.Errorf("excerpts = %q / %q", diff.NodeExcerpt, diff.GoExcerpt)
	}

	// Metrics repo on the go side only.
	nodeBare := filepath.Join(t.TempDir(), "home")
	_, goTree := stageTreePair(t, map[string]string{"docs/README.md": "seed\n"})
	diff, err = CompareTrees(nodeBare, goTree)
	if err != nil {
		t.Fatal(err)
	}
	if diff == nil || diff.Path != "docs/README.md" {
		t.Fatalf("diff = %+v", diff)
	}
	if diff.NodeExcerpt != "docs/README.md: absent" || diff.GoExcerpt != "docs/README.md: present" {
		t.Errorf("excerpts = %q / %q", diff.NodeExcerpt, diff.GoExcerpt)
	}
}
