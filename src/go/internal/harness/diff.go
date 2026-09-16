package harness

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Side names one binary under test.
type Side string

const (
	SideNode Side = "node"
	SideGo   Side = "go"
)

// Case verdicts.
const (
	StatusGreen   = "green"
	StatusRed     = "red"
	StatusTimeout = "timeout"
)

// ExitSentinel is the trailing marker the pty wrapper prints so the harness
// can recover the child's exit code from the merged transcript; util-linux
// and BSD script disagree on exit-code propagation, so a sentinel is the
// portable path.
const ExitSentinel = "__TUDIFF_EXIT="

// PtyCols/PtyRows pin the pty size: a pty spawned without a controlling
// terminal (CI) reports 0 columns, which the TS width fallback never catches.
const (
	PtyCols = 120
	PtyRows = 40
)

// SideCapture is one side's recorded behavior for a case. Stdout/Stderr apply
// to pipe cases, TTY to tty cases. Err carries harness-level problems with
// the capture itself (e.g. a missing exit sentinel).
type SideCapture struct {
	Stdout   []byte
	Stderr   []byte
	TTY      []byte
	Exit     int
	TimedOut bool
	Duration time.Duration
	Err      string
}

// Result is the comparison outcome of one case, plus the informational
// call-log comparison and run metadata the report renders.
type Result struct {
	Case        Case
	Status      string
	Channel     string // first differing channel (red), or "timeout"
	Offset      int    // first differing byte index (byte channels)
	Line        int    // 1-based line of Offset in the node capture
	NodeExcerpt string // strconv.Quote of ≤40 bytes from Offset
	GoExcerpt   string
	NodeExit    int
	GoExit      int
	NodeMs      int64
	GoMs        int64
	NodeTimeout bool // unexported into report.json; backs the timeout case line
	GoTimeout   bool
	Unconfirmed bool
	CallsDiffer bool
	NodeCalls   int
	GoCalls     int
	Rerun       bool
}

// EnvSpec parameterizes BuildEnv: everything the child environment needs that
// is not already on the Case.
type EnvSpec struct {
	HarnessBin string   // absolute dir holding the fake ccusage/git (PATH-first)
	Home       string   // this side's staged $HOME
	Fixtures   []string // absolute fixture alias dirs, search order
	CallLog    string   // this side's TUDIFF_CALL_LOG path
}

// BuildEnv constructs the child environment from scratch — nothing is
// inherited from the harness process beyond PATH, so an exported
// TU_METRICS_REPO or NO_COLOR in the developer's shell cannot tilt a case.
func BuildEnv(c Case, spec EnvSpec) []string {
	env := []string{
		"PATH=" + spec.HarnessBin + string(os.PathListSeparator) + os.Getenv("PATH"),
		"HOME=" + spec.Home,
		"TZ=" + TZName(c.TZ),
		"LANG=C.UTF-8",
		"LC_ALL=C.UTF-8",
		"TUDIFF_FIXTURES=" + strings.Join(spec.Fixtures, string(os.PathListSeparator)),
		"TUDIFF_CALL_LOG=" + spec.CallLog,
	}
	if c.IO == IOTTY {
		env = append(env, "TERM=xterm-256color")
	}
	if c.Env == EnvNoColor {
		env = append(env, "NO_COLOR=1")
	}
	if c.Env == EnvEnvrepo {
		env = append(env, "TU_METRICS_REPO="+MetricsRepoURL)
	}
	return env
}

// StageOracle copies the TS oracle into tmpDir so the repository's dist/ is
// never mutated: the bundle, the shipped default conf beside it (the bundled
// layout findDefaultConf checks first), and the fake ccusage in the fixed
// vendor slot the TS fetcher execs.
func StageOracle(tmpDir, nodePath, defaultConf, fakeCcusage string) (bundlePath string, err error) {
	dist := filepath.Join(tmpDir, "oracle", "dist")
	bundlePath = filepath.Join(dist, "tu.mjs")
	if err := copyFile(nodePath, bundlePath, 0o644); err != nil {
		return "", fmt.Errorf("tudiff: staging oracle bundle: %w", err)
	}
	if err := copyFile(defaultConf, filepath.Join(dist, "tu.default.conf"), 0o644); err != nil {
		return "", fmt.Errorf("tudiff: staging tu.default.conf: %w", err)
	}
	vendor := filepath.Join(dist, "vendor", "ccusage", "bin", "ccusage")
	if err := copyFile(fakeCcusage, vendor, 0o755); err != nil {
		return "", fmt.Errorf("tudiff: staging fake ccusage: %w", err)
	}
	return bundlePath, nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	raw, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, raw, mode)
}

// RunPipe executes one side with stdin from /dev/null and stdout/stderr
// captured into separate buffers, bounded by timeout. A deadline sets
// TimedOut and Exit -1.
func RunPipe(name string, args []string, dir string, env []string, timeout time.Duration) SideCapture {
	var c SideCapture
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = env
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	start := time.Now()
	err := cmd.Run()
	c.Duration = time.Since(start)
	c.Stdout = outBuf.Bytes()
	c.Stderr = errBuf.Bytes()
	c.Exit, c.TimedOut = exitOf(ctx, err)
	return c
}

// ScriptFlavor detects the script(1) implementation once per run: util-linux
// when `script --version` succeeds, BSD otherwise. Returns "" when script is
// absent entirely.
func ScriptFlavor() string {
	path, err := exec.LookPath("script")
	if err != nil {
		return ""
	}
	if exec.Command(path, "--version").Run() == nil {
		return "util-linux"
	}
	return "bsd"
}

// RunTTY executes one side under a pseudo-terminal via script(1) with the pty
// size pinned and the exit code recovered from the ExitSentinel line the
// wrapper prints; the transcript (stdout and stderr merged by the pty, \r\n
// endings kept verbatim) lands in Capture.TTY.
func RunTTY(scriptPath string, name string, args []string, dir string, env []string, timeout time.Duration, flavor string) SideCapture {
	inner := shellQuote(name)
	for _, a := range args {
		inner += " " + shellQuote(a)
	}
	wrapper := fmt.Sprintf("stty cols %d rows %d; %s; printf '\\n%s%%s\\n' \"$?\"",
		PtyCols, PtyRows, inner, ExitSentinel)

	var argv []string
	if flavor == "util-linux" {
		argv = []string{"-q", "-e", "-c", wrapper, os.DevNull}
	} else {
		argv = []string{"-q", os.DevNull, "sh", "-c", wrapper}
	}
	c := RunPipe(scriptPath, argv, dir, env, timeout)
	c.TTY, c.Exit, c.Err = parseTTYExit(c.Stdout)
	c.Stdout = nil
	c.Stderr = nil
	return c
}

// parseTTYExit splits the trailing "__TUDIFF_EXIT=<n>" line (and its
// preceding line break) off a pty transcript. A missing or malformed
// sentinel yields Exit -1 and the "no exit sentinel" error.
func parseTTYExit(raw []byte) (tty []byte, exit int, err string) {
	s := string(raw)
	idx := strings.LastIndex(s, ExitSentinel)
	if idx < 0 {
		return raw, -1, "no exit sentinel"
	}
	rest := s[idx+len(ExitSentinel):]
	numLen := 0
	for numLen < len(rest) && rest[numLen] >= '0' && rest[numLen] <= '9' {
		numLen++
	}
	if numLen == 0 || strings.Trim(rest[numLen:], "\r\n") != "" {
		return raw, -1, "no exit sentinel"
	}
	n, convErr := strconv.Atoi(rest[:numLen])
	if convErr != nil {
		return raw, -1, "no exit sentinel"
	}
	start := idx
	if start >= 2 && s[start-2:start] == "\r\n" {
		start -= 2
	} else if start >= 1 && s[start-1] == '\n' {
		start--
	}
	return []byte(s[:start]), n, ""
}

// shellQuote wraps s in single quotes for the wrapper's /bin/sh.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// exitOf derives the exit code from a finished command; a context deadline
// means the side timed out (Exit -1).
func exitOf(ctx context.Context, err error) (exit int, timedOut bool) {
	if ctx.Err() == context.DeadlineExceeded {
		return -1, true
	}
	if err == nil {
		return 0, false
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), false
	}
	return -1, false
}

// byteChannel is one compared byte channel of a case.
type byteChannel struct {
	name string
	node []byte
	goCh []byte
}

// Compare byte-diffs one case's two captures, stopping at the first differing
// channel: pipe cases compare exit, stdout, stderr; tty cases compare exit,
// tty. A timeout on either side is a timeout verdict regardless of bytes.
func Compare(c Case, node, goCap SideCapture) Result {
	r := Result{
		Case:     c,
		Status:   StatusGreen,
		NodeExit: node.Exit,
		GoExit:   goCap.Exit,
		NodeMs:   node.Duration.Milliseconds(),
		GoMs:     goCap.Duration.Milliseconds(),
	}
	if node.TimedOut || goCap.TimedOut {
		r.Status = StatusTimeout
		r.Channel = "timeout"
		r.NodeTimeout = node.TimedOut
		r.GoTimeout = goCap.TimedOut
		return r
	}
	if node.Exit != goCap.Exit {
		r.Status = StatusRed
		r.Channel = "exit"
		return r
	}
	var channels []byteChannel
	if c.IO == IOTTY {
		channels = []byteChannel{{"tty", node.TTY, goCap.TTY}}
	} else {
		channels = []byteChannel{
			{"stdout", node.Stdout, goCap.Stdout},
			{"stderr", node.Stderr, goCap.Stderr},
		}
	}
	for _, ch := range channels {
		if !bytes.Equal(ch.node, ch.goCh) {
			r.Status = StatusRed
			r.Channel = ch.name
			r.Offset, r.Line, r.NodeExcerpt, r.GoExcerpt = firstDivergence(ch.node, ch.goCh)
			return r
		}
	}
	return r
}

// firstDivergence locates the first differing byte of two captures: its
// index, the 1-based line containing it (counting '\n' in the node side up to
// the offset), and quoted ≤40-byte excerpts of each side from that offset.
func firstDivergence(node, goCap []byte) (offset, line int, nodeExcerpt, goExcerpt string) {
	n := len(node)
	if len(goCap) < n {
		n = len(goCap)
	}
	offset = n
	for i := 0; i < n; i++ {
		if node[i] != goCap[i] {
			offset = i
			break
		}
	}
	line = 1 + bytes.Count(node[:offset], []byte{'\n'})
	return offset, line, excerpt(node, offset), excerpt(goCap, offset)
}

// excerpt quotes at most 40 bytes of b starting at offset.
func excerpt(b []byte, offset int) string {
	end := offset + 40
	if end > len(b) {
		end = len(b)
	}
	return strconv.Quote(string(b[offset:end]))
}

// CompareCallLogs parses both sides' call logs and compares them as sorted
// sets of tool+argv pairs (cwd differs by side by construction; matched is
// alias metadata). Missing logs count as empty. The result is informational
// — it must never redden a case.
func CompareCallLogs(nodePath, goPath string) (nodeN, goN int, differ bool, err error) {
	nodeSet, err := callSet(nodePath)
	if err != nil {
		return 0, 0, false, err
	}
	goSet, err := callSet(goPath)
	if err != nil {
		return 0, 0, false, err
	}
	nodeList, goList := sortedKeys(nodeSet), sortedKeys(goSet)
	return len(nodeList), len(goList), !equalStrings(nodeList, goList), nil
}

// callSet reads a JSON-lines call log into a multiset of canonical call keys.
func callSet(path string) (map[string]int, error) {
	set := map[string]int{}
	err := readCallLog(path, func(cl callLogLine) error {
		set[cl.Tool+"\x00"+strings.Join(cl.Argv, "\x00")]++
		return nil
	})
	return set, err
}

func sortedKeys(set map[string]int) []string {
	var out []string
	for k, n := range set {
		for i := 0; i < n; i++ {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
