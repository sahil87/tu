package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sahil87/tu/internal/harness"
)

// Default flag values; relative defaults resolve against the repo root.
const (
	defaultMatrix     = "harness/matrix.json"
	defaultExpected   = "harness/expected-diffs.json"
	defaultNode       = "dist/tu.mjs"
	defaultGo         = "bin/tu"
	defaultHarnessBin = "bin/harness"
	defaultReport     = "bin/harness/report"
)

// runOptions holds the parsed run flags; set records which were given
// explicitly (relative defaults resolve against the repo root, explicit
// relative values against the caller's cwd).
type runOptions struct {
	matrix, expected, node, goBin, harnessBin, report, filter, fixtures string
	placeholder, list                                                   bool
	jobs                                                                int
	timeout                                                             time.Duration
	set                                                                 map[string]bool
}

func runRun(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	opts := runOptions{set: map[string]bool{}}
	fs.StringVar(&opts.matrix, "matrix", defaultMatrix, "argument matrix file")
	fs.StringVar(&opts.expected, "expected", defaultExpected, "expected-diffs file (DC-keyed intentional divergences)")
	fs.StringVar(&opts.node, "node", defaultNode, "TS oracle bundle (run as `node <staged copy>`)")
	fs.StringVar(&opts.goBin, "go", defaultGo, "Go binary under test")
	fs.StringVar(&opts.harnessBin, "harness-bin", defaultHarnessBin, "directory holding the fake ccusage and git")
	fs.StringVar(&opts.fixtures, "fixtures", "", "comma-separated fixture aliases under harness/fixtures/ (default: automatic D7 resolution)")
	fs.BoolVar(&opts.placeholder, "placeholder", false, "force the _placeholder corpus only (what CI passes)")
	fs.StringVar(&opts.report, "report", defaultReport, "report directory (wiped at the start of a run)")
	fs.StringVar(&opts.filter, "filter", "", "run only cases whose expanded ID contains this substring")
	fs.BoolVar(&opts.list, "list", false, "print the expanded case IDs and exit 0 without running")
	fs.IntVar(&opts.jobs, "jobs", 4, "cases run concurrently (each case is fully isolated)")
	fs.DurationVar(&opts.timeout, "timeout", 60*time.Second, "per-side wall-clock bound; a timeout is a red case")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	fs.Visit(func(f *flag.Flag) { opts.set[f.Name] = true })

	fail := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "tudiff: "+format+"\n", a...)
		return 2
	}

	// Preflight, in R2 order.
	if opts.fixtures != "" && opts.placeholder {
		return fail("--fixtures and --placeholder are mutually exclusive")
	}
	if opts.jobs < 1 {
		return fail("--jobs must be >= 1")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fail("%v", err)
	}
	root, err := harness.FindRepoRoot(cwd)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	m, err := harness.LoadMatrix(opts.resolve(root, "matrix", opts.matrix))
	if err != nil {
		return fail("matrix: %v", err)
	}
	cases := harness.Expand(m)
	if opts.filter != "" {
		cases = harness.Filter(cases, opts.filter)
		if len(cases) == 0 {
			return fail("--filter matched no cases")
		}
	}
	if opts.list {
		for _, c := range cases {
			fmt.Fprintln(stdout, c.ID)
		}
		return 0
	}

	// The expected-diffs file is loaded after the matrix check and after the
	// --list early return; a missing file is a preflight error, never an
	// empty set (the harness never reports a vacuous 0).
	exp, err := harness.LoadExpected(opts.resolve(root, "expected", opts.expected))
	if err != nil {
		if os.IsNotExist(err) {
			return fail("%s not found", opts.expected)
		}
		return fail("expected-diffs: %v", err)
	}

	nodeBin, scriptPath, flavor, code := preflightBinaries(&opts, root, cases, fail)
	if code != 0 {
		return code
	}
	hostname, _ := os.Hostname()
	aliasDirs, err := harness.ResolveFixtures(root, splitComma(opts.fixtures), opts.placeholder, hostname)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	return executeRun(&opts, root, cases, exp, nodeBin, scriptPath, flavor, aliasDirs, stdout, stderr)
}

// resolve absolutizes a path flag: an explicit value resolves against the
// caller's cwd, an untouched default against the repo root.
func (o *runOptions) resolve(root, name, value string) string {
	return resolvePathFlag(root, o.set[name], value)
}

// resolvePathFlag is the flag-resolution rule shared by run and live: an
// explicit value resolves against the caller's cwd, an untouched default
// against the repo root.
func resolvePathFlag(root string, explicit bool, value string) string {
	if filepath.IsAbs(value) || explicit {
		if abs, err := filepath.Abs(value); err == nil {
			return abs
		}
		return value
	}
	return filepath.Join(root, value)
}

// preflightBinaries runs the R2 binary/environment checks after the matrix
// has loaded: --node, --go, the fakes, node on PATH, script for tty cases,
// and the placeholder manifest.
func preflightBinaries(opts *runOptions, root string, cases []harness.Case, fail func(string, ...any) int) (nodeBin, scriptPath, flavor string, code int) {
	if !fileExists(opts.resolve(root, "node", opts.node)) {
		return "", "", "", fail("%s not found (run npm ci && npm run build)", opts.node)
	}
	goPath := opts.resolve(root, "go", opts.goBin)
	if !fileExists(goPath) {
		return "", "", "", fail("%s not found (run just go-build)", opts.goBin)
	}
	if !executable(goPath) {
		return "", "", "", fail("%s not executable (run just go-build)", opts.goBin)
	}
	for _, fake := range []string{"ccusage", "git"} {
		p := opts.resolve(root, "harness-bin", opts.harnessBin)
		if !fileExists(filepath.Join(p, fake)) {
			return "", "", "", fail("%s not found (run just harness-build)", filepath.Join(opts.harnessBin, fake))
		}
		if !executable(filepath.Join(p, fake)) {
			return "", "", "", fail("%s not executable (run just harness-build)", filepath.Join(opts.harnessBin, fake))
		}
	}
	nodeBin, err := exec.LookPath("node")
	if err != nil {
		return "", "", "", fail("node not found on PATH (required to run the TS oracle)")
	}
	needsTTY := false
	for _, c := range cases {
		if c.IO == harness.IOTTY {
			needsTTY = true
			break
		}
	}
	if needsTTY {
		if scriptPath, err = exec.LookPath("script"); err != nil {
			return "", "", "", fail("script not found on PATH (required for tty cases)")
		}
		flavor = harness.ScriptFlavor()
	}
	manifest := filepath.Join(root, "harness", "fixtures", harness.PlaceholderAlias, "manifest.json")
	if !fileExists(manifest) {
		return "", "", "", fail("harness/fixtures/%s/manifest.json not found", harness.PlaceholderAlias)
	}
	return nodeBin, scriptPath, flavor, 0
}

// executeRun stages the oracle, runs every case through the worker pool,
// streams report lines in matrix order, and writes the report.
func executeRun(opts *runOptions, root string, cases []harness.Case, exp *harness.Expected, nodeBin, scriptPath, flavor string, aliasDirs []string, stdout, stderr io.Writer) int {
	cfg, cleanup, err := stageRun(opts, root, nodeBin, scriptPath, flavor, aliasDirs)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	defer cleanup()
	cfg.expected = exp

	header := reportHeader(opts, cfg, nodeBin, len(cases))
	header.ExpectedPath = opts.expected
	header.ExpectedEntries = len(exp.Entries)
	for _, line := range harness.RenderHeader(header) {
		fmt.Fprintln(stdout, line)
	}

	results, runErr := runAllCases(cases, cfg, opts.jobs, stdout)
	summary := harness.SummarizeResults(exp, results)
	for _, line := range harness.RenderSummary(summary, header.Fixtures) {
		fmt.Fprintln(stdout, line)
	}
	if err := harness.WriteReport(cfg.reportDir, header, exp, results); err != nil {
		fmt.Fprintf(stderr, "tudiff: writing report: %v\n", err)
		return 2
	}
	if runErr != nil {
		fmt.Fprintf(stderr, "tudiff: %v\n", runErr)
		return 2
	}
	// The R5 gate rule: an expected red case passes; an unexpected red, a
	// timeout, an unconfirmed-fixture replay, or a stale entry fails.
	if summary.Unexpected+summary.Timeout+summary.Unconfirmed > 0 || len(summary.Stale) > 0 {
		return 1
	}
	return 0
}

// stageRun prepares a run: resolves every path into a runConfig, wipes and
// recreates the report dir, creates the temp root, and stages the oracle
// copy. cleanup removes the temp root.
func stageRun(opts *runOptions, root, nodeBin, scriptPath, flavor string, aliasDirs []string) (runConfig, func(), error) {
	cfg := runConfig{
		seedDir:    filepath.Join(root, "harness", "metrics-repo"),
		nodeBin:    nodeBin,
		goPath:     opts.resolve(root, "go", opts.goBin),
		harnessBin: opts.resolve(root, "harness-bin", opts.harnessBin),
		reportDir:  opts.resolve(root, "report", opts.report),
		fixtures:   aliasDirs,
		timeout:    opts.timeout,
		scriptPath: scriptPath,
		flavor:     flavor,
	}
	fail := func(format string, a ...any) (runConfig, func(), error) {
		return cfg, nil, fmt.Errorf("tudiff: "+format, a...)
	}
	if err := os.RemoveAll(cfg.reportDir); err != nil {
		return fail("wiping report dir: %v", err)
	}
	if err := os.MkdirAll(cfg.reportDir, 0o755); err != nil {
		return fail("creating report dir: %v", err)
	}
	tmpRoot, err := os.MkdirTemp("", "tudiff-")
	if err != nil {
		return fail("%v", err)
	}
	cfg.tmpRoot = tmpRoot
	cleanup := func() { os.RemoveAll(tmpRoot) }
	bundle, err := harness.StageOracle(tmpRoot, opts.resolve(root, "node", opts.node),
		filepath.Join(root, "tu.default.conf"), filepath.Join(cfg.harnessBin, "ccusage"))
	if err != nil {
		cleanup()
		return cfg, nil, err
	}
	cfg.bundle = bundle
	return cfg, cleanup, nil
}

// reportHeader builds the report header, probing both binaries' versions.
func reportHeader(opts *runOptions, cfg runConfig, nodeBin string, caseCount int) harness.ReportHeader {
	return harness.ReportHeader{
		Timestamp:   time.Now().UTC(),
		NodePath:    opts.node,
		NodeVersion: firstLine(exec.Command(nodeBin, "--version").Output()),
		GoPath:      opts.goBin,
		GoVersion:   firstLine(exec.Command(cfg.goPath, "--version").Output()),
		Fixtures:    aliasNames(cfg.fixtures),
		Script:      cfg.flavor,
		MatrixPath:  opts.matrix,
		Cases:       caseCount,
		Filter:      opts.filter,
	}
}

// runConfig carries the resolved, per-run constants shared by every case.
type runConfig struct {
	seedDir, bundle, nodeBin, goPath, harnessBin, reportDir, tmpRoot string
	fixtures                                                         []string
	timeout                                                          time.Duration
	scriptPath, flavor                                               string
	expected                                                         *harness.Expected
}

// runAllCases executes the cases through a --jobs worker pool. Report lines
// stream in matrix order: a flush cursor prints contiguous completed results
// as they arrive, so completion order never leaks into the report.
func runAllCases(cases []harness.Case, cfg runConfig, jobs int, stdout io.Writer) ([]harness.Result, error) {
	type outcome struct {
		idx int
		res harness.Result
		err error
	}
	results := make([]harness.Result, len(cases))
	work := make(chan int)
	out := make(chan outcome)
	var wg sync.WaitGroup
	for w := 0; w < jobs; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range work {
				res, err := runCase(cases[idx], cfg)
				out <- outcome{idx, res, err}
			}
		}()
	}
	go func() {
		for i := range cases {
			work <- i
		}
		close(work)
		wg.Wait()
		close(out)
	}()

	var firstErr error
	done := make([]bool, len(cases))
	next := 0
	for o := range out {
		if o.err != nil && firstErr == nil {
			firstErr = fmt.Errorf("case %s: %w", o.res.Case.ID, o.err)
		}
		results[o.idx] = o.res
		done[o.idx] = true
		for next < len(cases) && done[next] {
			fmt.Fprintln(stdout, harness.RenderCaseLine(results[next]))
			next++
		}
	}
	return results, firstErr
}

// localDateInTZ is the injectable clock behind the date-rollover re-run: the
// local calendar date in the case's timezone.
var localDateInTZ = func(tzName string) string {
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		loc = time.UTC
	}
	return time.Now().In(loc).Format("2006-01-02")
}

// caseSide holds one side's staged paths and prepared environment.
type caseSide struct {
	dir     string // working directory (<caseTmp>/<side>/)
	home    string // staged $HOME (<dir>/home)
	callLog string // this side's TUDIFF_CALL_LOG, under the report dir
	env     []string
}

// runCase executes one case end to end: two staged $HOMEs, both sides
// back-to-back (re-run once on a date rollover), the byte comparison and —
// for cases still green — the written-tree comparison, the informational
// call-log and unconfirmed lookups, and the raw captures.
func runCase(c harness.Case, cfg runConfig) (harness.Result, error) {
	res := harness.Result{Case: c, Status: harness.StatusRed, NodeExit: -1, GoExit: -1}
	node, goSide, err := stageCase(c, cfg)
	if err != nil {
		return res, err
	}

	tzName := harness.TZName(c.TZ)
	before := localDateInTZ(tzName)
	nodeCap := execSide(c, cfg, true, node)
	goCap := execSide(c, cfg, false, goSide)
	rerun := false
	if localDateInTZ(tzName) != before {
		// Midnight rollover between the two sides: discard and re-run once.
		rerun = true
		_ = os.Remove(node.callLog)
		_ = os.Remove(goSide.callLog)
		nodeCap = execSide(c, cfg, true, node)
		goCap = execSide(c, cfg, false, goSide)
	}
	nodeCap.Home, goCap.Home = node.home, goSide.home

	res = harness.Compare(c, nodeCap, goCap)
	res.Rerun = rerun
	if res.Status == harness.StatusGreen {
		// The bytes agreed; the written trees must agree too (the sync
		// writer's half of the case). A tree difference reddens the case on
		// the "tree" channel; a case already red keeps its first channel.
		diff, err := harness.CompareTrees(node.home, goSide.home)
		if err != nil {
			return res, err
		}
		if diff != nil {
			res.Status = harness.StatusRed
			res.Channel = "tree"
			res.NodeExcerpt = diff.NodeExcerpt
			res.GoExcerpt = diff.GoExcerpt
		}
	}
	if err := annotateCalls(&res, node, goSide, cfg.fixtures); err != nil {
		return res, err
	}
	// A red case may be an intentional divergence: annotate it with the
	// matching expected-diffs entry (green and timeout cases never carry one).
	if res.Status == harness.StatusRed {
		if id, ok := cfg.expected.Match(c.ID, c.Group); ok {
			res.Expected = id
		}
	}
	if err := harness.WriteCaseCaptures(cfg.reportDir, res, nodeCap, goCap); err != nil {
		return res, err
	}
	return res, nil
}

// stageCase creates the case's temp and report directories, stages both
// sides' $HOMEs, and builds their environments.
func stageCase(c harness.Case, cfg runConfig) (node, goSide caseSide, err error) {
	safeID := strings.ReplaceAll(c.ID, "/", "-")
	caseTmp := filepath.Join(cfg.tmpRoot, "cases", safeID)
	reportCaseDir := filepath.Join(cfg.reportDir, "cases", filepath.FromSlash(c.ID))
	if err := os.MkdirAll(reportCaseDir, 0o755); err != nil {
		return node, goSide, err
	}
	for _, sp := range []struct {
		side *caseSide
		name string
	}{
		{&node, string(harness.SideNode)},
		{&goSide, string(harness.SideGo)},
	} {
		sp.side.dir = filepath.Join(caseTmp, sp.name)
		sp.side.home = filepath.Join(sp.side.dir, "home")
		sp.side.callLog = filepath.Join(reportCaseDir, sp.name+".calls.jsonl")
		if err := harness.StageHome(sp.side.home, c.Conf, cfg.seedDir); err != nil {
			return node, goSide, err
		}
		sp.side.env = harness.BuildEnv(c, harness.EnvSpec{
			HarnessBin: cfg.harnessBin,
			Home:       sp.side.home,
			Fixtures:   cfg.fixtures,
			CallLog:    sp.side.callLog,
		})
	}
	return node, goSide, nil
}

// execSide runs one side of a case: the staged oracle bundle through node, or
// the Go binary in place; under script(1) for tty cases, pipes otherwise.
func execSide(c harness.Case, cfg runConfig, node bool, side caseSide) harness.SideCapture {
	name, args := cfg.goPath, c.Args
	if node {
		name, args = cfg.nodeBin, append([]string{cfg.bundle}, c.Args...)
	}
	if c.IO == harness.IOTTY {
		return harness.RunTTY(cfg.scriptPath, name, args, side.dir, side.env, cfg.timeout, cfg.flavor)
	}
	return harness.RunPipe(name, args, side.dir, side.env, cfg.timeout)
}

// annotateCalls fills the informational call-log comparison and the
// unconfirmed-replay flag from the two sides' call logs. Each side's argv is
// home-normalized with that side's staged home before comparison.
func annotateCalls(res *harness.Result, node, goSide caseSide, fixtures []string) error {
	nodeN, goN, differ, err := harness.CompareCallLogs(node.callLog, goSide.callLog, node.home, goSide.home)
	if err != nil {
		return err
	}
	res.NodeCalls, res.GoCalls, res.CallsDiffer = nodeN, goN, differ
	nodeUnc, err := harness.UnconfirmedReplays(node.callLog, fixtures)
	if err != nil {
		return err
	}
	goUnc, err := harness.UnconfirmedReplays(goSide.callLog, fixtures)
	if err != nil {
		return err
	}
	res.Unconfirmed = nodeUnc || goUnc
	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func executable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Mode().Perm()&0o111 != 0
}

func firstLine(out []byte, err error) string {
	if err != nil {
		return ""
	}
	s, _, _ := strings.Cut(string(out), "\n")
	return strings.TrimRight(s, "\r")
}

// aliasNames renders the resolved fixture dirs as their alias names for the
// report header.
func aliasNames(dirs []string) []string {
	out := make([]string, 0, len(dirs))
	for _, d := range dirs {
		out = append(out, filepath.Base(d))
	}
	return out
}
