package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/sahil87/tu/internal/harness"
)

// Default flag values; relative defaults resolve against the repo root.
const (
	defaultMatrix     = "harness/matrix.json"
	defaultExpected   = "harness/expected-diffs.json"
	defaultGo         = "bin/tu"
	defaultHarnessBin = "bin/harness"
	defaultReport     = "bin/harness/report"
)

// runOptions holds the parsed run flags; set records which were given
// explicitly (relative defaults resolve against the repo root, explicit
// relative values against the caller's cwd).
type runOptions struct {
	matrix, expected, goBin, harnessBin, report, filter, fixtures string
	golden, now, fromNode                                         string
	placeholder, list, update                                     bool
	jobs                                                          int
	timeout                                                       time.Duration
	set                                                           map[string]bool
}

func runRun(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	opts := runOptions{set: map[string]bool{}}
	fs.StringVar(&opts.matrix, "matrix", defaultMatrix, "argument matrix file")
	fs.StringVar(&opts.expected, "expected", defaultExpected, "expected-diffs file (DC-keyed intentional divergences)")
	fs.StringVar(&opts.goBin, "go", defaultGo, "Go binary under test")
	fs.StringVar(&opts.harnessBin, "harness-bin", defaultHarnessBin, "directory holding the fake ccusage and git")
	fs.StringVar(&opts.fixtures, "fixtures", "", "comma-separated fixture aliases under harness/fixtures/ (only with --update and a non-default --golden)")
	fs.BoolVar(&opts.placeholder, "placeholder", false, "force the _placeholder corpus only (what CI passes)")
	fs.StringVar(&opts.report, "report", defaultReport, "report directory (wiped at the start of a run)")
	fs.StringVar(&opts.filter, "filter", "", "run only cases whose expanded ID contains this substring")
	fs.BoolVar(&opts.list, "list", false, "print the expanded case IDs and exit 0 without running")
	fs.IntVar(&opts.jobs, "jobs", 4, "cases run concurrently (each case is fully isolated)")
	fs.DurationVar(&opts.timeout, "timeout", 60*time.Second, "per-side wall-clock bound; a timeout is a red case")
	fs.StringVar(&opts.golden, "golden", harness.DefaultGoldenDir, "golden corpus directory (manifest.json plus run/<case>/ captures)")
	fs.BoolVar(&opts.update, "update", false, "rewrite the goldens from the Go side instead of comparing")
	fs.StringVar(&opts.now, "now", "", "pin the golden clock (zone-less 2006-01-02T15:04:05) for --update")
	fs.StringVar(&opts.fromNode, "from-node", "", "TRANSITIONAL (deleted with src/node): capture the goldens from this Node oracle bundle instead of the Go binary")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	fs.Visit(func(f *flag.Flag) { opts.set[f.Name] = true })

	// --from-node implies --placeholder: the committed corpus never carries
	// real usage data.
	if opts.fromNode != "" {
		opts.placeholder = true
	}

	fail := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "tudiff: "+format+"\n", a...)
		return 2
	}

	// Preflight, in R3 order.
	if opts.fixtures != "" && opts.placeholder {
		return fail("--fixtures and --placeholder are mutually exclusive")
	}
	if opts.fromNode != "" && !opts.update {
		return fail("--from-node requires --update")
	}
	// Golden mode compares against placeholder-only goldens; a real-fixture
	// run against them is meaningless. The one exception: --update targeting
	// a non-default --golden dir (a local golden set for real captures).
	if opts.fixtures != "" && !(opts.update && opts.golden != harness.DefaultGoldenDir) {
		return fail("--fixtures needs an oracle; goldens are placeholder-only")
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

	matrixPath := opts.resolve(root, "matrix", opts.matrix)
	m, err := harness.LoadMatrix(matrixPath)
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

	goldenDir := opts.resolve(root, "golden", opts.golden)
	manifest, code := preflightGolden(&opts, goldenDir, matrixPath, cases, fail)
	if code != 0 {
		return code
	}
	scriptPath, flavor, code := preflightBinaries(&opts, root, cases, fail)
	if code != 0 {
		return code
	}
	hostname, _ := os.Hostname()
	// Golden mode resolves _placeholder only (no hostname alias); an explicit
	// --fixtures list is reachable only through the --update exception above.
	aliasDirs, err := harness.ResolveFixtures(root, splitComma(opts.fixtures), true, hostname)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if opts.update {
		if opts.fromNode != "" {
			nodeBin, code := preflightFromNode(&opts, root, fail)
			if code != 0 {
				return code
			}
			return executeNodeCapture(&opts, root, cases, manifest, goldenDir, matrixPath, scriptPath, flavor, aliasDirs, nodeBin, stdout, stderr)
		}
		return executeUpdate(&opts, root, cases, manifest, goldenDir, matrixPath, scriptPath, flavor, aliasDirs, stdout, stderr)
	}
	return executeRun(&opts, root, cases, exp, manifest, goldenDir, scriptPath, flavor, aliasDirs, stdout, stderr)
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

// preflightGolden loads the golden manifest and runs the golden-mode drift
// guards. Compare mode requires a valid manifest whose matrix hash matches
// the current matrix and a golden dir for every expanded case; --update
// treats a missing manifest as the bootstrap case and skips both guards.
func preflightGolden(opts *runOptions, goldenDir, matrixPath string, cases []harness.Case, fail func(string, ...any) int) (*harness.GoldenManifest, int) {
	m, err := harness.LoadGoldenManifest(goldenDir)
	if err != nil {
		if os.IsNotExist(err) {
			if opts.update {
				return nil, 0
			}
			return nil, fail("%s not found (run tudiff run --update)", filepath.Join(opts.golden, "manifest.json"))
		}
		return nil, fail("%s: %v", filepath.Join(opts.golden, "manifest.json"), err)
	}
	if opts.update {
		return m, 0
	}
	sum, err := harness.MatrixSHA256(matrixPath)
	if err != nil {
		return nil, fail("matrix: %v", err)
	}
	if sum != m.MatrixSHA256 {
		return nil, fail("%s changed since the goldens were captured (run tudiff run --update and review the diff)", opts.matrix)
	}
	for _, c := range cases {
		if info, err := os.Stat(harness.GoldenCaseDir(goldenDir, c.ID)); err != nil || !info.IsDir() {
			return nil, fail("no golden for %s (run tudiff run --update)", c.ID)
		}
	}
	return m, 0
}

// preflightBinaries runs the binary/environment checks after the golden
// guards: --go, the fakes, script for tty cases, and the placeholder
// manifest.
func preflightBinaries(opts *runOptions, root string, cases []harness.Case, fail func(string, ...any) int) (scriptPath, flavor string, code int) {
	goPath := opts.resolve(root, "go", opts.goBin)
	if !fileExists(goPath) {
		return "", "", fail("%s not found (run just go-build)", opts.goBin)
	}
	if !executable(goPath) {
		return "", "", fail("%s not executable (run just go-build)", opts.goBin)
	}
	for _, fake := range []string{"ccusage", "git"} {
		p := opts.resolve(root, "harness-bin", opts.harnessBin)
		if !fileExists(filepath.Join(p, fake)) {
			return "", "", fail("%s not found (run just harness-build)", filepath.Join(opts.harnessBin, fake))
		}
		if !executable(filepath.Join(p, fake)) {
			return "", "", fail("%s not executable (run just harness-build)", filepath.Join(opts.harnessBin, fake))
		}
	}
	needsTTY := false
	for _, c := range cases {
		if c.IO == harness.IOTTY {
			needsTTY = true
			break
		}
	}
	if needsTTY {
		var err error
		if scriptPath, err = exec.LookPath("script"); err != nil {
			return "", "", fail("script not found on PATH (required for tty cases)")
		}
		flavor = harness.ScriptFlavor()
	}
	manifest := filepath.Join(root, "harness", "fixtures", harness.PlaceholderAlias, "manifest.json")
	if !fileExists(manifest) {
		return "", "", fail("harness/fixtures/%s/manifest.json not found", harness.PlaceholderAlias)
	}
	return scriptPath, flavor, 0
}

// executeRun stages the run, executes every case through the worker pool
// against its golden, streams report lines in matrix order, and writes the
// report.
func executeRun(opts *runOptions, root string, cases []harness.Case, exp *harness.Expected, manifest *harness.GoldenManifest, goldenDir, scriptPath, flavor string, aliasDirs []string, stdout, stderr io.Writer) int {
	cfg, cleanup, err := stageRun(opts, root, scriptPath, flavor, aliasDirs)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	defer cleanup()
	cfg.expected = exp
	cfg.goldenDir = goldenDir
	cfg.now = manifest.Now
	// A failed version probe degrades to no version normalization: the cases
	// themselves report the broken binary (harness-channel red), which is
	// more actionable than a preflight refusal.
	cfg.version, _ = harness.ProbeVersion(cfg.goPath, "--version")

	header := reportHeader(opts, cfg, manifest, len(cases))
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
	// The gate rule: an expected red case passes; an unexpected red, a
	// timeout, an unconfirmed-fixture replay, or a stale entry fails.
	if summary.Unexpected+summary.Timeout+summary.Unconfirmed > 0 || len(summary.Stale) > 0 {
		return 1
	}
	return 0
}

// executeUpdate rewrites the goldens from the Go side over the (filtered)
// matrix, then rewrites the manifest. A filtered run rewrites only the
// matched cases (go test -run X -update style).
func executeUpdate(opts *runOptions, root string, cases []harness.Case, old *harness.GoldenManifest, goldenDir, matrixPath, scriptPath, flavor string, aliasDirs []string, stdout, stderr io.Writer) int {
	now := resolveNow(opts.now, old)
	cfg, cleanup, err := stageRun(opts, root, scriptPath, flavor, aliasDirs)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	defer cleanup()
	cfg.goldenDir = goldenDir
	cfg.now = now
	cfg.captureSide = string(harness.SideGo)
	cfg.version, err = harness.ProbeVersion(cfg.goPath, "--version")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	n, err := captureAll(cases, cfg, opts.jobs, func(c harness.Case, s caseSide) harness.SideCapture {
		return execGo(c, cfg, s)
	})
	if err != nil {
		fmt.Fprintf(stderr, "tudiff: %v\n", err)
		return 2
	}
	m, err := newManifest(old, matrixPath, now)
	if err != nil {
		fmt.Fprintf(stderr, "tudiff: %v\n", err)
		return 2
	}
	m.Oracle = opts.goBin
	m.OracleVersion = cfg.version
	m.Cases = n
	if err := harness.WriteGoldenManifest(goldenDir, m); err != nil {
		fmt.Fprintf(stderr, "tudiff: writing manifest: %v\n", err)
		return 2
	}
	fmt.Fprintf(stdout, "tudiff: wrote %d goldens under %s (now %s)\n", n, opts.golden+"/run", now)
	return 0
}

// stageRun prepares a run: resolves every path into a runConfig, wipes and
// recreates the report dir, and creates the temp root. cleanup removes the
// temp root.
func stageRun(opts *runOptions, root, scriptPath, flavor string, aliasDirs []string) (runConfig, func(), error) {
	cfg := runConfig{
		seedDir:    filepath.Join(root, "harness", "metrics-repo"),
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
	return cfg, func() { os.RemoveAll(tmpRoot) }, nil
}

// reportHeader builds the report header from the golden manifest's
// provenance, probing the Go binary's version for the display line.
func reportHeader(opts *runOptions, cfg runConfig, m *harness.GoldenManifest, caseCount int) harness.ReportHeader {
	return harness.ReportHeader{
		Timestamp:           time.Now().UTC(),
		GoldenDir:           opts.golden,
		GoldenCapturedAt:    m.CapturedAt,
		GoldenOracle:        m.Oracle,
		GoldenOracleVersion: m.OracleVersion,
		GoldenNow:           m.Now,
		GoPath:              opts.goBin,
		GoVersion:           firstLine(exec.Command(cfg.goPath, "--version").Output()),
		Fixtures:            aliasNames(cfg.fixtures),
		Script:              cfg.flavor,
		MatrixPath:          opts.matrix,
		Cases:               caseCount,
		Filter:              opts.filter,
	}
}

// runConfig carries the resolved, per-run constants shared by every case.
type runConfig struct {
	seedDir, goPath, harnessBin, reportDir, tmpRoot, goldenDir string
	fixtures                                                   []string
	timeout                                                    time.Duration
	scriptPath, flavor                                         string
	// now is the golden manifest's pinned clock, emitted as TUDIFF_NOW in the
	// child's environment (empty for the transitional Node capture arm — the
	// Node side has no clock seam and runs on real time).
	now string
	// version is the capture/compare side's probed --version value: goldens
	// are stored and Go captures are compared after NormalizeVersion with it.
	version string
	// captureSide names the staged temp/report segment for --update
	// (SideGo; SideNode only in the transitional --from-node arm).
	captureSide string
	expected    *harness.Expected
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

// captureAll is the --update worker pool: every case's golden is written by
// captureCase; the first error aborts the run after in-flight cases finish.
func captureAll(cases []harness.Case, cfg runConfig, jobs int, exec func(harness.Case, caseSide) harness.SideCapture) (int, error) {
	work := make(chan harness.Case)
	errs := make(chan error, len(cases))
	var wg sync.WaitGroup
	for w := 0; w < jobs; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range work {
				if err := captureCase(c, cfg, exec); err != nil {
					errs <- fmt.Errorf("case %s: %w", c.ID, err)
				}
			}
		}()
	}
	for _, c := range cases {
		work <- c
	}
	close(work)
	wg.Wait()
	close(errs)
	for err := range errs {
		return 0, err
	}
	return len(cases), nil
}

// captureCase stages one home, runs the case's capture side, and writes
// run/<id>/ golden files: the byte channels (version-normalized here,
// home-normalized by WriteGoldenCase), the decimal exit code, and tree.json.
func captureCase(c harness.Case, cfg runConfig, exec func(harness.Case, caseSide) harness.SideCapture) error {
	side, err := stageCase(c, cfg, cfg.captureSide)
	if err != nil {
		return err
	}
	cap := exec(c, side)
	if cap.TimedOut {
		return fmt.Errorf("capture timed out")
	}
	if cap.Err != "" {
		return fmt.Errorf("capture failed: %s", cap.Err)
	}
	cap.Home = side.home
	cap.Stdout = harness.NormalizeVersion(cap.Stdout, cfg.version)
	cap.Stderr = harness.NormalizeVersion(cap.Stderr, cfg.version)
	cap.TTY = harness.NormalizeVersion(cap.TTY, cfg.version)
	tree, err := harness.TreeSnapshot(side.home)
	if err != nil {
		return err
	}
	return harness.WriteGoldenCase(harness.GoldenCaseDir(cfg.goldenDir, c.ID), cap, tree)
}

// resolveNow picks the manifest's pinned clock: --now wins, then the existing
// manifest's value, then noon of today (local) for a bootstrap capture.
func resolveNow(flagValue string, old *harness.GoldenManifest) string {
	if flagValue != "" {
		return flagValue
	}
	if old != nil && old.Now != "" {
		return old.Now
	}
	return time.Now().Format("2006-01-02") + "T12:00:00"
}

// newManifest builds the manifest for a capture: fresh captured_at and matrix
// hash and the run constants, with both case counts carried over from the old
// manifest — the caller (run or live) overwrites the count it just wrote.
func newManifest(old *harness.GoldenManifest, matrixPath, now string) (*harness.GoldenManifest, error) {
	sum, err := harness.MatrixSHA256(matrixPath)
	if err != nil {
		return nil, err
	}
	m := &harness.GoldenManifest{
		Schema:       harness.SchemaVersion,
		CapturedAt:   time.Now().UTC().Format(time.RFC3339),
		Now:          now,
		Script:       harness.ScriptFlavor(),
		Platform:     runtime.GOOS + "/" + runtime.GOARCH,
		Fixtures:     []string{harness.PlaceholderAlias},
		MatrixSHA256: sum,
	}
	if old != nil {
		m.Cases = old.Cases
		m.LiveSteps = old.LiveSteps
	}
	return m, nil
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

// runCase executes one case end to end against its golden: one staged $HOME,
// the Go side run under the pinned clock (re-run once on a date rollover),
// the byte comparison against the golden in the oracle position and — for
// cases still green — the tree.json comparison, the unconfirmed-replay
// lookup, and the Go-side report captures.
func runCase(c harness.Case, cfg runConfig) (harness.Result, error) {
	res := harness.Result{Case: c, Status: harness.StatusRed, NodeExit: -1, GoExit: -1}
	goSide, err := stageCase(c, cfg, string(harness.SideGo))
	if err != nil {
		return res, err
	}
	goldenCap, goldenTree, err := harness.LoadGoldenCase(harness.GoldenCaseDir(cfg.goldenDir, c.ID), c.IO)
	if err != nil {
		return res, err
	}

	tzName := harness.TZName(c.TZ)
	before := localDateInTZ(tzName)
	goCap := execGo(c, cfg, goSide)
	rerun := false
	if localDateInTZ(tzName) != before {
		// Midnight rollover during the case: discard and re-run once. The
		// pinned clock makes this moot; the guard is cheap and stays.
		rerun = true
		_ = os.Remove(goSide.callLog)
		goCap = execGo(c, cfg, goSide)
	}
	goCap.Home = goSide.home
	goCap.Stdout = harness.NormalizeVersion(goCap.Stdout, cfg.version)
	goCap.Stderr = harness.NormalizeVersion(goCap.Stderr, cfg.version)
	goCap.TTY = harness.NormalizeVersion(goCap.TTY, cfg.version)

	res = harness.Compare(c, goldenCap, goCap)
	res.Rerun = rerun
	if res.Status == harness.StatusGreen {
		// The bytes agreed; the written tree must agree with tree.json too
		// (the sync writer's half of the case). A tree difference reddens the
		// case on the "tree" channel; a case already red keeps its first
		// channel.
		diff, err := harness.CompareTree(goldenTree, goSide.home)
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
	if err := annotateCalls(&res, goSide, cfg.fixtures); err != nil {
		return res, err
	}
	// A red case may be an intentional divergence: annotate it with the
	// matching expected-diffs entry. Green, timeout, and harness-channel
	// cases never carry one — a capture-level failure (a failed exec, a
	// missing exit sentinel) is not a comparison divergence and must never
	// be masked as expected.
	if res.Status == harness.StatusRed && res.Channel != "harness" {
		if id, ok := cfg.expected.Match(c.ID, c.Group); ok {
			res.Expected = id
		}
	}
	if err := harness.WriteCaseCaptures(cfg.reportDir, res, goCap); err != nil {
		return res, err
	}
	return res, nil
}

// stageCase creates the case's temp and report directories, stages one $HOME,
// and builds its environment. side names the temp/report segment: SideGo for
// the compare and Go-capture paths, SideNode only in the transitional
// --from-node arm.
func stageCase(c harness.Case, cfg runConfig, side string) (caseSide, error) {
	var s caseSide
	safeID := strings.ReplaceAll(c.ID, "/", "-")
	caseTmp := filepath.Join(cfg.tmpRoot, "cases", safeID)
	reportCaseDir := filepath.Join(cfg.reportDir, "cases", filepath.FromSlash(c.ID))
	if err := os.MkdirAll(reportCaseDir, 0o755); err != nil {
		return s, err
	}
	s.dir = filepath.Join(caseTmp, side)
	s.home = filepath.Join(s.dir, "home")
	s.callLog = filepath.Join(reportCaseDir, side+".calls.jsonl")
	if err := harness.StageHome(s.home, c.Conf, cfg.seedDir); err != nil {
		return s, err
	}
	s.env = harness.BuildEnv(c, harness.EnvSpec{
		HarnessBin: cfg.harnessBin,
		Home:       s.home,
		Fixtures:   cfg.fixtures,
		CallLog:    s.callLog,
		Now:        cfg.now,
	})
	return s, nil
}

// execGo runs the Go side of a case: the binary in place, under script(1) for
// tty cases, pipes otherwise.
func execGo(c harness.Case, cfg runConfig, side caseSide) harness.SideCapture {
	if c.IO == harness.IOTTY {
		return harness.RunTTY(cfg.scriptPath, cfg.goPath, c.Args, side.dir, side.env, cfg.timeout, cfg.flavor)
	}
	return harness.RunPipe(cfg.goPath, c.Args, side.dir, side.env, cfg.timeout)
}

// annotateCalls fills the unconfirmed-replay flag and the Go call count from
// the Go side's call log (the node-vs-go call-set comparison is gone with the
// oracle; the Go log still feeds the gate and go.calls.jsonl).
func annotateCalls(res *harness.Result, goSide caseSide, fixtures []string) error {
	unc, err := harness.UnconfirmedReplays(goSide.callLog, fixtures)
	if err != nil {
		return err
	}
	res.Unconfirmed = unc
	n, err := harness.CountCalls(goSide.callLog)
	if err != nil {
		return err
	}
	res.GoCalls = n
	return nil
}

// --- Transitional Node capture arm (plan R5) --------------------------------
// Everything between these markers exists only to capture the committed
// golden corpus from the Node oracle while src/node/ still exists; T006
// deletes the whole arm (the --from-node flag, preflightFromNode,
// captureDateOK/captureClock, executeNodeCapture, execNode) together with
// harness.StageOracle and harness.SideNode.

// captureClock is the injectable wall clock behind the capture-date guard.
var captureClock = time.Now

// preflightFromNode runs the arm's extra checks: the bundle and node on PATH.
func preflightFromNode(opts *runOptions, root string, fail func(string, ...any) int) (nodeBin string, code int) {
	if !fileExists(opts.resolve(root, "from-node", opts.fromNode)) {
		return "", fail("%s not found (run npm ci && npm run build)", opts.fromNode)
	}
	nodeBin, err := exec.LookPath("node")
	if err != nil {
		return "", fail("node not found on PATH (required by --from-node)")
	}
	return nodeBin, 0
}

// captureDateOK reports the local calendar date when it agrees under TZ=UTC
// and TZ=Asia/Kolkata — the 06:00–18:30 UTC window in which a Node capture on
// the real clock yields one date for both tz-axis values.
func captureDateOK(t time.Time) (string, bool) {
	loc, err := time.LoadLocation(harness.TZAltName)
	if err != nil {
		loc = time.FixedZone(harness.TZAltName, 5*60*60+30*60)
	}
	utc := t.UTC().Format("2006-01-02")
	if alt := t.In(loc).Format("2006-01-02"); alt != utc {
		return "", false
	}
	return utc, true
}

// executeNodeCapture is the --update --from-node path: the goldens are
// captured from the staged Node oracle on the real clock, guarded so the
// capture date agrees in both matrix timezones before the first case and
// after the last; manifest.now is that date at noon.
func executeNodeCapture(opts *runOptions, root string, cases []harness.Case, old *harness.GoldenManifest, goldenDir, matrixPath, scriptPath, flavor string, aliasDirs []string, nodeBin string, stdout, stderr io.Writer) int {
	fail := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "tudiff: "+format+"\n", a...)
		return 2
	}
	date, ok := captureDateOK(captureClock())
	if !ok {
		return fail("capture date differs between UTC and Asia/Kolkata — retry between 06:00 and 18:30 UTC")
	}
	cfg, cleanup, err := stageRun(opts, root, scriptPath, flavor, aliasDirs)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	defer cleanup()
	bundle, err := harness.StageOracle(cfg.tmpRoot, opts.resolve(root, "from-node", opts.fromNode),
		filepath.Join(root, "tu.default.conf"), filepath.Join(cfg.harnessBin, "ccusage"))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	cfg.goldenDir = goldenDir
	cfg.now = "" // the Node side runs on the real clock
	cfg.captureSide = string(harness.SideNode)
	if cfg.version, err = harness.ProbeVersion(nodeBin, bundle, "--version"); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	nodeVersion, err := harness.ProbeVersion(nodeBin, "--version")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	n, err := captureAll(cases, cfg, opts.jobs, func(c harness.Case, s caseSide) harness.SideCapture {
		return execNode(c, cfg, nodeBin, bundle, s)
	})
	if err != nil {
		return fail("%v", err)
	}
	if _, ok := captureDateOK(captureClock()); !ok {
		return fail("capture date differs between UTC and Asia/Kolkata — retry between 06:00 and 18:30 UTC")
	}
	m, err := newManifest(old, matrixPath, date+"T12:00:00")
	if err != nil {
		return fail("%v", err)
	}
	m.Oracle = "node " + opts.fromNode
	m.OracleVersion = cfg.version
	m.NodeVersion = nodeVersion
	m.Cases = n
	if err := harness.WriteGoldenManifest(goldenDir, m); err != nil {
		return fail("writing manifest: %v", err)
	}
	fmt.Fprintf(stdout, "tudiff: wrote %d goldens under %s (now %s)\n", n, opts.golden+"/run", m.Now)
	return 0
}

// execNode runs the Node side of a case: the staged oracle bundle through the
// node binary, under script(1) for tty cases, pipes otherwise.
func execNode(c harness.Case, cfg runConfig, nodeBin, bundle string, side caseSide) harness.SideCapture {
	args := append([]string{bundle}, c.Args...)
	if c.IO == harness.IOTTY {
		return harness.RunTTY(cfg.scriptPath, nodeBin, args, side.dir, side.env, cfg.timeout, cfg.flavor)
	}
	return harness.RunPipe(nodeBin, args, side.dir, side.env, cfg.timeout)
}

// --- end of the transitional Node capture arm ---

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
