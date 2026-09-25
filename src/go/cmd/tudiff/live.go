package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sahil87/tu/internal/harness"
)

// live.go is the `tudiff live` subcommand (intake § 10, plan R4): the
// real-git half of the differential gate. Where `run` answers every git call
// with the fake, live seeds one bare remote from harness/metrics-repo/,
// points the staged multi home at a real clone of it, and runs the
// sync/repair sequence with the real git on PATH — the pinned identity and
// fixed commit dates make the tree reproducible, so the golden corpus's
// log.txt (hashes included) is stable across runs. Compare mode diffs the Go
// binary against harness/golden/live/<step>/; --update rewrites those
// goldens.

const (
	defaultTurepair   = "bin/turepair"
	defaultLiveReport = "bin/harness/report-live"

	// fixedGitDate pins every commit timestamp the live sequence creates
	// (seed, sync, foreign, repair fixture), so identical trees and messages
	// yield identical commit hashes on every run.
	fixedGitDate = "2026-01-09T12:00:00Z"

	// liveTimeout bounds one execution of one live step (real git over a
	// local bare; far above anything the sequence should need).
	liveTimeout = 60 * time.Second

	// rebaseRecoveryNeedle is the stderr line the interrupted-rebase step
	// must produce (internal/sync rebaseRecoveryLine).
	rebaseRecoveryNeedle = "Warning: recovering from interrupted rebase"
)

// liveSteps lists the sequence's result IDs in order: the seven sync steps
// plus the two repair steps. Compare-mode preflight requires a golden dir for
// each, and --update's manifest live_steps count is the number written.
var liveSteps = []string{
	"sync-dry-run", "sync", "sync-again", "foreign-sync", "rebase-recovery",
	"pull-failure", "cc-sync", "repair-dry-run", "repair-write",
}

// liveOptions holds the parsed live flags; set records explicit flags (same
// resolution rule as run).
type liveOptions struct {
	goBin, turepair, harnessBin, report, expected string
	golden, now                                   string
	keep, update                                  bool
	set                                           map[string]bool
}

func runLive(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("live", flag.ContinueOnError)
	fs.SetOutput(stderr)
	opts := liveOptions{set: map[string]bool{}}
	fs.StringVar(&opts.goBin, "go", defaultGo, "Go binary under test")
	fs.StringVar(&opts.turepair, "turepair", defaultTurepair, "Go repair binary (the repair steps' compare/capture side)")
	fs.StringVar(&opts.harnessBin, "harness-bin", defaultHarnessBin, "directory holding the fake ccusage (the fake git is deliberately unused here)")
	fs.StringVar(&opts.report, "report", defaultLiveReport, "report directory (wiped at the start of a run)")
	fs.StringVar(&opts.expected, "expected", defaultExpected, "expected-diffs file (DC-keyed intentional divergences)")
	fs.BoolVar(&opts.keep, "keep", false, "keep the temp dir (bare, clone, staged home) for inspection")
	fs.StringVar(&opts.golden, "golden", harness.DefaultGoldenDir, "golden corpus directory (manifest.json plus live/<step>/ captures)")
	fs.BoolVar(&opts.update, "update", false, "rewrite the live goldens from the Go side instead of comparing")
	fs.StringVar(&opts.now, "now", "", "pin the golden clock (zone-less 2006-01-02T15:04:05) for --update")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	fs.Visit(func(f *flag.Flag) { opts.set[f.Name] = true })

	fail := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "tudiff: "+format+"\n", a...)
		return 2
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
	goldenDir := opts.resolve(root, "golden", opts.golden)
	manifest, exp, code := preflightLive(&opts, root, goldenDir, fail)
	if code != 0 {
		return code
	}
	if opts.update {
		return executeLiveUpdate(&opts, root, manifest, goldenDir, stdout, stderr)
	}
	return executeLive(&opts, root, manifest, exp, stdout, stderr)
}

// resolve absolutizes a path flag with the shared run/live rule.
func (o *liveOptions) resolve(root, name, value string) string {
	return resolvePathFlag(root, o.set[name], value)
}

// preflightLive runs the live checks: real git on PATH, both Go binaries, the
// fake ccusage, the placeholder manifest the fake replays from, and the
// expected-diffs file (same messages as run: a missing file is a preflight
// error, never an empty set), then the golden guards.
func preflightLive(opts *liveOptions, root, goldenDir string, fail func(string, ...any) int) (*harness.GoldenManifest, *harness.Expected, int) {
	if _, err := exec.LookPath("git"); err != nil {
		return nil, nil, fail("git not found on PATH (live diffs the real git, not the fake)")
	}
	for _, b := range []struct{ name, path string }{
		{"go", opts.goBin},
		{"turepair", opts.turepair},
	} {
		p := opts.resolve(root, b.name, b.path)
		if !fileExists(p) {
			return nil, nil, fail("%s not found (run just go-build)", b.path)
		}
		if !executable(p) {
			return nil, nil, fail("%s not executable (run just go-build)", b.path)
		}
	}
	cc := filepath.Join(opts.resolve(root, "harness-bin", opts.harnessBin), "ccusage")
	if !fileExists(cc) {
		return nil, nil, fail("%s not found (run just harness-build)", filepath.Join(opts.harnessBin, "ccusage"))
	}
	if !executable(cc) {
		return nil, nil, fail("%s not executable (run just harness-build)", filepath.Join(opts.harnessBin, "ccusage"))
	}
	manifest := filepath.Join(root, "harness", "fixtures", harness.PlaceholderAlias, "manifest.json")
	if !fileExists(manifest) {
		return nil, nil, fail("harness/fixtures/%s/manifest.json not found", harness.PlaceholderAlias)
	}
	exp, err := harness.LoadExpected(opts.resolve(root, "expected", opts.expected))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fail("%s not found", opts.expected)
		}
		return nil, nil, fail("expected-diffs: %v", err)
	}
	golden, code := preflightLiveGolden(opts, root, goldenDir, fail)
	if code != 0 {
		return nil, nil, code
	}
	return golden, exp, 0
}

// preflightLiveGolden runs live's golden-mode guards (the run.go discipline
// with live's own update hints): compare mode requires a valid manifest whose
// matrix hash matches the current matrix and a golden dir per step; --update
// treats a missing manifest as the bootstrap case and skips both guards.
func preflightLiveGolden(opts *liveOptions, root, goldenDir string, fail func(string, ...any) int) (*harness.GoldenManifest, int) {
	m, err := harness.LoadGoldenManifest(goldenDir)
	if err != nil {
		if os.IsNotExist(err) {
			if opts.update {
				return nil, 0
			}
			return nil, fail("%s not found (run tudiff live --update)", filepath.Join(opts.golden, "manifest.json"))
		}
		return nil, fail("%s: %v", filepath.Join(opts.golden, "manifest.json"), err)
	}
	if opts.update {
		return m, 0
	}
	sum, err := harness.MatrixSHA256(filepath.Join(root, defaultMatrix))
	if err != nil {
		return nil, fail("matrix: %v", err)
	}
	if sum != m.MatrixSHA256 {
		return nil, fail("%s changed since the goldens were captured (run tudiff run --update and review the diff)", defaultMatrix)
	}
	for _, step := range liveSteps {
		if info, err := os.Stat(harness.GoldenLiveDir(goldenDir, step)); err != nil || !info.IsDir() {
			return nil, fail("no golden for live/%s (run tudiff live --update)", step)
		}
	}
	return m, 0
}

// liveConfig carries the resolved, per-run constants shared by every step.
type liveConfig struct {
	root, tmpRoot, livebin          string
	goPath, turepairPath, goldenDir string
	seedDir, placeholder, reportDir string
	// now is the golden manifest's pinned clock, emitted as TUDIFF_NOW in the
	// child's environment.
	now string
	// version is the Go binary's probed --version value, for NormalizeVersion
	// on the byte channels.
	version string
	// hostname/username are the machine's probed identity (ProbeIdentity), for
	// NormalizeIdentity on the byte channels, the extras, and tree.json keys.
	hostname, username string
	keep               bool
	update             bool // write goldens instead of comparing
	expected           *harness.Expected
}

// liveSide holds the staged side's paths.
type liveSide struct {
	name string // "go"
	dir  string // working directory (<tmp>/<side>/)
	home string // staged $HOME (<dir>/home)
	repo string // <home>/.tu/metrics_repo — a real clone of the bare
	bare string // <tmp>/remote.git
}

// liveRunner holds the live sequence's shared state.
type liveRunner struct {
	cfg      liveConfig
	baseEnv  []string // pinned identity/date env for the harness's own git calls
	side     liveSide
	exec     func(args []string, fixtures string) harness.SideCapture // tu side
	repair   func(repo string, write bool) harness.SideCapture        // repair side
	captured int                                                      // goldens written (update modes)
	fatal    error                                                    // update modes: first capture failure; later steps are skipped
}

// executeLive runs the sequence against the live goldens and writes the
// report under --report.
func executeLive(opts *liveOptions, root string, manifest *harness.GoldenManifest, exp *harness.Expected, stdout, stderr io.Writer) int {
	lr, cleanup, code := stageLive(opts, root, stderr)
	if code != 0 {
		return code
	}
	defer cleanup()
	lr.cfg.goldenDir = opts.resolve(root, "golden", opts.golden)
	lr.cfg.now = manifest.Now
	lr.cfg.expected = exp
	lr.exec = lr.execGo
	lr.repair = lr.repairGo
	// A failed version probe degrades to no version normalization (the run.go
	// rule): the steps themselves report the broken binary.
	lr.cfg.version, _ = harness.ProbeVersion(lr.cfg.goPath, "--version")
	lr.cfg.hostname, lr.cfg.username = harness.ProbeIdentity()

	// The header lands in report.txt via WriteReport (the case count is known
	// only after the sequence runs); stdout streams case lines and the
	// summary, as before golden mode.
	header := harness.ReportHeader{
		Timestamp:           time.Now().UTC(),
		GoldenDir:           opts.golden,
		GoldenCapturedAt:    manifest.CapturedAt,
		GoldenOracle:        manifest.Oracle,
		GoldenOracleVersion: manifest.OracleVersion,
		GoldenNow:           manifest.Now,
		GoPath:              opts.goBin,
		GoVersion:           firstLine(exec.Command(lr.cfg.goPath, "--version").Output()),
		Fixtures:            []string{harness.PlaceholderAlias, "live-alias"},
		MatrixPath:          "live",
		ExpectedPath:        opts.expected,
		ExpectedEntries:     len(exp.Entries),
	}
	results, _ := lr.runSequence(stdout)
	header.Cases = len(results)
	summary := harness.SummarizeResults(exp, results)
	for _, line := range harness.RenderSummary(summary, header.Fixtures) {
		fmt.Fprintln(stdout, line)
	}
	if err := harness.WriteReport(lr.cfg.reportDir, header, exp, results); err != nil {
		fmt.Fprintf(stderr, "tudiff: writing report: %v\n", err)
		return 2
	}
	if lr.cfg.keep {
		fmt.Fprintf(stderr, "tudiff: kept temp dir %s\n", lr.cfg.tmpRoot)
	}
	// The gate rule, same as run: an expected red step passes; an unexpected
	// red, a timeout, an unconfirmed-fixture replay, or a stale entry fails.
	if summary.Unexpected+summary.Timeout+summary.Unconfirmed > 0 || len(summary.Stale) > 0 {
		return 1
	}
	return 0
}

// executeLiveUpdate rewrites the live goldens from the Go side, then rewrites
// the manifest with live_steps set and cases preserved from the old manifest.
func executeLiveUpdate(opts *liveOptions, root string, old *harness.GoldenManifest, goldenDir string, stdout, stderr io.Writer) int {
	now := resolveNow(opts.now, old)
	lr, cleanup, code := stageLive(opts, root, stderr)
	if code != 0 {
		return code
	}
	defer cleanup()
	lr.cfg.goldenDir = goldenDir
	lr.cfg.now = now
	lr.cfg.update = true
	lr.exec = lr.execGo
	lr.repair = lr.repairGo
	version, err := harness.ProbeVersion(lr.cfg.goPath, "--version")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	lr.cfg.version = version
	lr.cfg.hostname, lr.cfg.username = harness.ProbeIdentity()
	if _, err := lr.runSequence(stdout); err != nil {
		fmt.Fprintf(stderr, "tudiff: %v\n", err)
		return 2
	}
	m, err := newManifest(old, filepath.Join(root, defaultMatrix), now)
	if err != nil {
		fmt.Fprintf(stderr, "tudiff: %v\n", err)
		return 2
	}
	m.Oracle = opts.goBin
	m.OracleVersion = lr.cfg.version
	m.LiveSteps = lr.captured
	if err := harness.WriteGoldenManifest(goldenDir, m); err != nil {
		fmt.Fprintf(stderr, "tudiff: writing manifest: %v\n", err)
		return 2
	}
	fmt.Fprintf(stdout, "tudiff: wrote %d goldens under %s (now %s)\n", lr.captured, opts.golden+"/live", now)
	return 0
}

// stageLive prepares a live run: resolves paths, wipes and recreates the
// report dir, creates the temp root and the livebin (only the fake ccusage —
// the fake git is deliberately absent so every git a tu child spawns is the
// real one), seeds the bare remote, and stages the side's home as a real
// clone of it. cleanup removes the temp root unless --keep.
func stageLive(opts *liveOptions, root string, stderr io.Writer) (*liveRunner, func(), int) {
	cfg := liveConfig{
		root:         root,
		goPath:       opts.resolve(root, "go", opts.goBin),
		turepairPath: opts.resolve(root, "turepair", opts.turepair),
		seedDir:      filepath.Join(root, "harness", "metrics-repo"),
		placeholder:  filepath.Join(root, "harness", "fixtures", harness.PlaceholderAlias),
		reportDir:    opts.resolve(root, "report", opts.report),
		keep:         opts.keep,
	}
	fail := func(format string, a ...any) (*liveRunner, func(), int) {
		fmt.Fprintf(stderr, "tudiff: "+format+"\n", a...)
		return nil, nil, 2
	}
	if err := os.RemoveAll(cfg.reportDir); err != nil {
		return fail("wiping report dir: %v", err)
	}
	if err := os.MkdirAll(cfg.reportDir, 0o755); err != nil {
		return fail("creating report dir: %v", err)
	}
	tmpRoot, err := os.MkdirTemp("", "tudiff-live-")
	if err != nil {
		return fail("%v", err)
	}
	cfg.tmpRoot = tmpRoot
	cleanup := func() { os.RemoveAll(tmpRoot) }
	if cfg.keep {
		cleanup = func() {}
	}
	cfg.livebin = filepath.Join(tmpRoot, "livebin")
	if err := harness.CopyFile(filepath.Join(opts.resolve(root, "harness-bin", opts.harnessBin), "ccusage"),
		filepath.Join(cfg.livebin, "ccusage"), 0o755); err != nil {
		cleanup()
		return fail("staging livebin: %v", err)
	}
	lr := &liveRunner{cfg: cfg, baseEnv: pinnedGitEnv(os.Getenv("PATH"), os.Getenv("HOME"))}
	if err := lr.stage(); err != nil {
		cleanup()
		fmt.Fprintln(stderr, err)
		return nil, nil, 2
	}
	return lr, cleanup, 0
}

// stage seeds the bare remote and stages the side's home as a real clone.
func (lr *liveRunner) stage() error {
	bare, err := seedLiveRemote(lr.cfg.tmpRoot, lr.cfg.seedDir, lr.baseEnv)
	if err != nil {
		return err
	}
	s, err := stageLiveSide(lr.cfg.tmpRoot, string(harness.SideGo), bare, lr.cfg.seedDir, lr.baseEnv)
	if err != nil {
		return err
	}
	lr.side = s
	return nil
}

// stepPins declares which extra artefacts a step pins beyond
// stdout/stderr/exit and tree.json (every step pins those): status.txt is the
// clone's `git status --porcelain` after the step, log.txt its
// `git log --format=%H%n%s -p main` — the same artefacts the step checked
// against the other side in the node-vs-go era.
type stepPins struct {
	status, log bool
}

var (
	pinStatus    = stepPins{status: true}
	pinStatusLog = stepPins{status: true, log: true}
	pinLog       = stepPins{log: true}
)

// runSequence runs the intake § 10.3 sequence plus the repair flow against
// the one staged side, streaming one report line per step in compare mode.
// Update modes write each step's golden instead and abort on the first
// capture failure (lr.fatal; the manifest is never written after one).
func (lr *liveRunner) runSequence(stdout io.Writer) ([]harness.Result, error) {
	var results []harness.Result
	emit := func(res harness.Result) {
		if lr.cfg.update {
			return // update modes stream nothing; lr.fatal carries failures
		}
		// The single annotation funnel: a red step may be an intentional
		// divergence (green, timeout, and harness-channel steps never carry
		// an expected id — a capture-level failure is not a comparison
		// divergence and must never be masked as expected).
		if res.Status == harness.StatusRed && res.Channel != "harness" {
			if id, ok := lr.cfg.expected.Match(res.Case.ID, res.Case.Group); ok {
				res.Expected = id
			}
		}
		results = append(results, res)
		fmt.Fprintln(stdout, harness.RenderCaseLine(res))
	}

	// a. sync --dry-run: the report; the tree and status stay untouched.
	emit(lr.runStepN("sync-dry-run", []string{"sync", "--dry-run"}, lr.cfg.placeholder, pinStatus))

	// b. sync: the round trip; then tree, status, log, .last-sync.
	emit(lr.runStepN("sync", []string{"sync"}, lr.cfg.placeholder, pinStatusLog))

	// c. sync again: the steady-state no-commit path; the log is unchanged
	// (log.txt is the post-sync log, so matching it proves no new commit).
	emit(lr.runStepN("sync-again", []string{"sync"}, lr.cfg.placeholder, pinLog))

	// d. foreign commit: a second clone of the bare pushes a day-file; the
	// side then syncs a fresh local change (the one-off alias raises cc
	// 2026-01-07) and pull --rebase integrates. The fetch cache key excludes
	// TUDIFF_FIXTURES and steps a–c warmed it, so .tu/cache must go first —
	// otherwise the alias's raised value is never read and the step exercises
	// only a clean pull.
	if lr.fatal == nil {
		emit(lr.foreignSyncStep())
	}

	// e. interrupted rebase: a fabricated .git/rebase-merge in the clone;
	// sync recovers and proceeds.
	if lr.fatal == nil {
		if err := fabricateRebaseMerge(lr.side.repo, lr.baseEnv); err != nil {
			emit(harnessFailStep("rebase-recovery", []string{"sync"}, err))
		} else {
			res, oracleCap, cap := lr.runStep("rebase-recovery", []string{"sync"}, lr.aliasOrPlaceholder(), pinStatus)
			checkStderrContains(&res, oracleCap, cap, rebaseRecoveryNeedle)
			emit(res)
		}
	}

	// f. pull failure: the clone points origin at a missing bare; sync fails
	// with the pull-failure bytes and exit 1 (the golden corpus pinned the
	// same real git's text). The working origin is restored afterwards so cc
	// --sync can run.
	if lr.fatal == nil {
		missing := filepath.Join(lr.cfg.tmpRoot, "missing.git")
		if _, err := liveGit(lr.baseEnv, lr.side.repo, "remote", "set-url", "origin", missing); err != nil {
			emit(harnessFailStep("pull-failure", []string{"sync"}, err))
		} else {
			res, _, _ := lr.runStep("pull-failure", []string{"sync"}, lr.aliasOrPlaceholder(), stepPins{})
			checkExit(&res, 1)
			emit(res)
		}
		_, _ = liveGit(lr.baseEnv, lr.side.repo, "remote", "set-url", "origin", lr.side.bare)
	}

	// g. cc --sync: the sync line on stderr, the table on stdout.
	emit(lr.runStepN("cc-sync", []string{"cc", "--sync"}, lr.aliasOrPlaceholder(), pinStatus))

	// Repair (intake § 10.4): the R11 fixture history seeded once, copied
	// once; the repair binary runs dry-run then --write, the repo path
	// normalized to $REPO before comparing/storing.
	if lr.fatal == nil {
		repo, err := lr.stageRepairRepo()
		if err != nil {
			emit(harnessFailStep("repair-dry-run", []string{"repair"}, err))
		} else {
			emit(lr.repairStep("repair-dry-run", repo, false))
			emit(lr.repairStep("repair-write", repo, true))
		}
	}

	return results, lr.fatal
}

// foreignSyncStep is sequence step d: the foreign commit pushed to the bare,
// the one-off raised-cost alias built, the fetch caches wiped, then the sync.
func (lr *liveRunner) foreignSyncStep() harness.Result {
	if err := lr.pushForeignCommit(); err != nil {
		return harnessFailStep("foreign-sync", []string{"sync"}, err)
	}
	alias := filepath.Join(lr.cfg.tmpRoot, "live-alias")
	if err := buildLiveAlias(lr.cfg.placeholder, alias, "2026-01-07", 0.9); err != nil {
		return harnessFailStep("foreign-sync", []string{"sync"}, err)
	}
	if err := lr.clearFetchCaches(); err != nil {
		return harnessFailStep("foreign-sync", []string{"sync"}, err)
	}
	return lr.runStepN("foreign-sync", []string{"sync"}, alias, pinStatusLog)
}

// aliasOrPlaceholder returns the one-off alias once the foreign-commit step
// built it, the committed placeholder before that — the later steps run
// steady-state against whichever corpus step d left active.
func (lr *liveRunner) aliasOrPlaceholder() string {
	alias := filepath.Join(lr.cfg.tmpRoot, "live-alias")
	if fileExists(filepath.Join(alias, "manifest.json")) {
		return alias
	}
	return lr.cfg.placeholder
}

// harnessFailStep builds the red result for a step that could not run (a
// harness-level failure before the side executed).
func harnessFailStep(id string, args []string, err error) harness.Result {
	return harness.Result{
		Case:        liveCase(id, args),
		Status:      harness.StatusRed,
		Channel:     "harness",
		NodeExcerpt: strconv.Quote(err.Error()),
		GoExcerpt:   strconv.Quote(err.Error()),
		NodeExit:    -1,
		GoExit:      -1,
	}
}

// liveCase builds the synthetic Case one live step compares under: always the
// multi home, default env, pipe, fixed tz.
func liveCase(id string, args []string) harness.Case {
	return harness.Case{
		ID:    id,
		Group: "live",
		Args:  args,
		Conf:  harness.ConfMulti,
		Env:   harness.EnvDefault,
		IO:    harness.IOPipe,
		TZ:    harness.TZFixed,
	}
}

// runStepN is runStep for steps without post-step assertions.
func (lr *liveRunner) runStepN(id string, args []string, fixtures string, pins stepPins) harness.Result {
	res, _, _ := lr.runStep(id, args, fixtures, pins)
	return res
}

// runStep executes one sequence step on the staged side. Compare mode loads
// live/<id>/ into the oracle position and compares (bytes via Compare, the
// gitless written tree against tree.json, the pinned extras as text); update
// modes write that golden from the fresh capture. It returns the result and
// both positions' captures (identical in update mode) for the step-specific
// assertions.
func (lr *liveRunner) runStep(id string, args []string, fixtures string, pins stepPins) (harness.Result, harness.SideCapture, harness.SideCapture) {
	c := liveCase(id, args)
	if lr.fatal != nil {
		return harness.Result{Case: c}, harness.SideCapture{}, harness.SideCapture{}
	}
	dir := harness.GoldenLiveDir(lr.cfg.goldenDir, id)
	cap, rerun := lr.execStep(args, fixtures)
	normalizeLive(&cap, lr.side.home, lr.cfg.tmpRoot, lr.cfg.version, lr.cfg.hostname, lr.cfg.username)
	if lr.cfg.update {
		res, err := lr.writeStepGolden(dir, c, cap, pins)
		if err != nil {
			lr.fatal = fmt.Errorf("step %s: %w", id, err)
			return harnessFailStep(id, args, err), cap, cap
		}
		return res, cap, cap
	}

	goldenCap, goldenTree, err := harness.LoadGoldenCase(dir, c.IO)
	if err != nil {
		return harnessFailStep(id, args, err), harness.SideCapture{}, cap
	}
	res := harness.Compare(c, goldenCap, cap)
	res.Rerun = rerun
	lr.checkGoldenTree(&res, goldenTree)
	lr.checkGoldenExtras(&res, dir, pins)
	if err := harness.WriteCaseCaptures(lr.cfg.reportDir, res, cap); err != nil {
		redden(&res, "harness", strconv.Quote(err.Error()), strconv.Quote(err.Error()))
	}
	return res, goldenCap, cap
}

// normalizeLive normalizes one live capture's byte channels for storage and
// comparison: the staged home first (it lives under the temp root), then the
// temp root itself (real git's error text — the pull-failure step's — embeds
// the missing-remote path verbatim), then the version, then the machine's
// identity. cap.Home stays set: Compare's and WriteCaseCaptures' home
// normalization are no-ops on the normalized bytes, and the tree-red go.tree
// copy still finds the home.
func normalizeLive(cap *harness.SideCapture, home, tmpRoot, version, hostname, username string) {
	norm := func(b []byte) []byte {
		b = harness.NormalizeHome(b, home)
		b = bytes.ReplaceAll(b, []byte(tmpRoot), []byte("$TMP"))
		b = harness.NormalizeVersion(b, version)
		return harness.NormalizeIdentity(b, hostname, username)
	}
	cap.Stdout = norm(cap.Stdout)
	cap.Stderr = norm(cap.Stderr)
	cap.Home = home
}

// execStep runs the capture/compare side's tu, re-running once on a UTC date
// rollover (the runCase rule; moot under the pinned clock but cheap).
func (lr *liveRunner) execStep(args []string, fixtures string) (cap harness.SideCapture, rerun bool) {
	before := localDateInTZ(harness.TZFixedName)
	cap = lr.exec(args, fixtures)
	if localDateInTZ(harness.TZFixedName) != before {
		return lr.exec(args, fixtures), true
	}
	return cap, false
}

// execGo runs the Go binary against the staged home.
func (lr *liveRunner) execGo(args []string, fixtures string) harness.SideCapture {
	return harness.RunPipe(lr.cfg.goPath, args, lr.side.dir, lr.childEnv(lr.side.home, fixtures), liveTimeout)
}

// writeStepGolden writes one step's golden: the byte channels (normalized by
// runStep), the decimal exit, the gitless tree.json, and the pinned extras.
// The returned result carries the capture's exit code in both positions so
// the step assertions (checkExit, checkStderrContains) validate the fresh
// capture exactly as they validate a comparison.
func (lr *liveRunner) writeStepGolden(dir string, c harness.Case, cap harness.SideCapture, pins stepPins) (harness.Result, error) {
	res := harness.Result{Case: c, Status: harness.StatusGreen, NodeExit: cap.Exit, GoExit: cap.Exit}
	if cap.TimedOut {
		return res, fmt.Errorf("capture timed out")
	}
	if cap.Err != "" {
		return res, fmt.Errorf("capture failed: %s", cap.Err)
	}
	tree, err := harness.TreeSnapshotLive(lr.side.home)
	if err != nil {
		return res, err
	}
	tree = harness.NormalizeTreeIdentity(tree, lr.cfg.hostname, lr.cfg.username)
	if err := harness.WriteGoldenCase(dir, cap, tree); err != nil {
		return res, err
	}
	if pins.status {
		out, err := liveGit(lr.baseEnv, lr.side.repo, "status", "--porcelain")
		if err != nil {
			return res, err
		}
		if err := lr.writeExtra(dir, "status.txt", out); err != nil {
			return res, err
		}
	}
	if pins.log {
		out, err := lr.sideLog()
		if err != nil {
			return res, err
		}
		if err := lr.writeExtra(dir, "log.txt", out); err != nil {
			return res, err
		}
	}
	lr.captured++
	return res, nil
}

// checkGoldenTree reddens when the clone's written tree (or .last-sync
// presence) diverges from tree.json.
func (lr *liveRunner) checkGoldenTree(res *harness.Result, goldenTree harness.Tree) {
	diff, err := harness.CompareLiveTree(goldenTree, lr.side.home, lr.cfg.hostname, lr.cfg.username)
	if err != nil {
		redden(res, "tree", strconv.Quote(err.Error()), strconv.Quote(err.Error()))
		return
	}
	if diff != nil {
		redden(res, "tree", diff.NodeExcerpt, diff.GoExcerpt)
	}
}

// checkGoldenExtras compares the pinned text artefacts: status.txt against
// the clone's git status --porcelain, log.txt against its log -p.
func (lr *liveRunner) checkGoldenExtras(res *harness.Result, dir string, pins stepPins) {
	if pins.status {
		lr.checkGoldenText(res, dir, "status.txt", "status", func() (string, error) {
			return liveGit(lr.baseEnv, lr.side.repo, "status", "--porcelain")
		})
	}
	if pins.log {
		lr.checkGoldenText(res, dir, "log.txt", "log", lr.sideLog)
	}
}

// checkGoldenText compares one harness-side text artefact against the step's
// stored extra (the actual text is identity-normalized first, as the stored
// extra was at capture).
func (lr *liveRunner) checkGoldenText(res *harness.Result, dir, name, channel string, actual func() (string, error)) {
	want, err := harness.LoadGoldenExtra(dir, name)
	if err != nil {
		redden(res, channel, strconv.Quote(err.Error()), strconv.Quote(err.Error()))
		return
	}
	got, err := actual()
	if err != nil {
		redden(res, channel, string(want), fmt.Sprint(err))
		return
	}
	got = string(harness.NormalizeIdentity([]byte(got), lr.cfg.hostname, lr.cfg.username))
	checkEqual(res, channel, string(want), got)
}

// writeExtra stores one pinned text artefact, identity-normalized (log.txt
// and status.txt are git output — no identity today, but the pipeline keeps
// the corpus provably free of it).
func (lr *liveRunner) writeExtra(dir, name, text string) error {
	return harness.WriteGoldenExtra(dir, name,
		harness.NormalizeIdentity([]byte(text), lr.cfg.hostname, lr.cfg.username))
}

// childEnv is one tu child's environment: livebin first on PATH (fake
// ccusage, real git), the staged HOME, TZ=UTC, the pinned git identity/date,
// the fixture search path, and — when the golden manifest pins a clock —
// TUDIFF_NOW.
func (lr *liveRunner) childEnv(home, fixtures string) []string {
	env := pinnedGitEnv(lr.cfg.livebin+string(os.PathListSeparator)+os.Getenv("PATH"), home)
	env = append(env, "TUDIFF_FIXTURES="+fixtures)
	if lr.cfg.now != "" {
		env = append(env, "TUDIFF_NOW="+lr.cfg.now)
	}
	return env
}

// pinnedGitEnv builds an environment from scratch (the BuildEnv rule: nothing
// inherited beyond PATH/HOME) with the pinned identity, config cut-outs, and
// fixed commit dates the live sequence's byte-identical hashes rest on.
func pinnedGitEnv(path, home string) []string {
	return []string{
		"PATH=" + path,
		"HOME=" + home,
		"TZ=UTC",
		"LANG=C.UTF-8",
		"LC_ALL=C.UTF-8",
		"GIT_AUTHOR_NAME=tudiff",
		"GIT_AUTHOR_EMAIL=tudiff@example.invalid",
		"GIT_COMMITTER_NAME=tudiff",
		"GIT_COMMITTER_EMAIL=tudiff@example.invalid",
		"GIT_CONFIG_GLOBAL=" + os.DevNull,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=commit.gpgsign",
		"GIT_CONFIG_VALUE_0=false",
		"GIT_AUTHOR_DATE=" + fixedGitDate,
		"GIT_COMMITTER_DATE=" + fixedGitDate,
	}
}

// --- harness-side git helpers ---

// liveGit runs real git (with -C dir when dir is non-empty) under env,
// returning stdout; stderr decorates a failure.
func liveGit(env []string, dir string, args ...string) (string, error) {
	argv := args
	if dir != "" {
		argv = append([]string{"-C", dir}, args...)
	}
	cmd := exec.Command("git", argv...)
	cmd.Env = env
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %v: %s", strings.Join(argv, " "), err, strings.TrimSpace(errBuf.String()))
	}
	return out.String(), nil
}

// seedLiveRemote builds the bare repo on main from seedDir (a scratch clone
// receives the full tree, docs/README.md included, one "seed" commit, push).
func seedLiveRemote(tmpRoot, seedDir string, env []string) (bare string, err error) {
	bare = filepath.Join(tmpRoot, "remote.git")
	if _, err := liveGit(env, tmpRoot, "init", "--bare", "--initial-branch=main", bare); err != nil {
		return "", err
	}
	scratch := filepath.Join(tmpRoot, "scratch")
	if _, err := liveGit(env, tmpRoot, "clone", bare, scratch); err != nil {
		return "", err
	}
	if err := harness.CopyTree(seedDir, scratch); err != nil {
		return "", err
	}
	if _, err := liveGit(env, scratch, "add", "-A"); err != nil {
		return "", err
	}
	if _, err := liveGit(env, scratch, "commit", "-m", "seed"); err != nil {
		return "", err
	}
	if _, err := liveGit(env, scratch, "push", "origin", "main"); err != nil {
		return "", err
	}
	return bare, nil
}

// stageLiveSide stages the side's $HOME as the multi variant, then REPLACES
// the seeded .tu/metrics_repo copy with a real clone of the bare.
func stageLiveSide(tmpRoot, name, bare, seedDir string, env []string) (liveSide, error) {
	s := liveSide{name: name, bare: bare}
	s.dir = filepath.Join(tmpRoot, name)
	s.home = filepath.Join(s.dir, "home")
	if err := harness.StageHome(s.home, harness.ConfMulti, seedDir); err != nil {
		return s, err
	}
	s.repo = filepath.Join(s.home, ".tu", "metrics_repo")
	if err := os.RemoveAll(s.repo); err != nil {
		return s, err
	}
	if _, err := liveGit(env, tmpRoot, "clone", bare, s.repo); err != nil {
		return s, err
	}
	return s, nil
}

// pushForeignCommit clones the bare a second time, adds the other-user
// day-file, commits with the pinned date, and pushes — the upstream change
// the next sync's pull --rebase integrates.
func (lr *liveRunner) pushForeignCommit() error {
	const foreignFile = "other-user/2026/laptop/cc-2026-01-08.jsonl"
	const foreignBody = `{"label":"2026-01-08","totalCost":2.5,"inputTokens":6000,"outputTokens":800,"cacheCreationTokens":2000,"cacheReadTokens":40000,"totalTokens":48800}` + "\n"
	clone := filepath.Join(lr.cfg.tmpRoot, "foreign")
	if _, err := liveGit(lr.baseEnv, lr.cfg.tmpRoot, "clone", lr.side.bare, clone); err != nil {
		return err
	}
	full := filepath.Join(clone, filepath.FromSlash(foreignFile))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(full, []byte(foreignBody), 0o644); err != nil {
		return err
	}
	if _, err := liveGit(lr.baseEnv, clone, "add", "other-user/"); err != nil {
		return err
	}
	if _, err := liveGit(lr.baseEnv, clone, "commit", "-m", "# other-user: update 2026-01-08"); err != nil {
		return err
	}
	if _, err := liveGit(lr.baseEnv, clone, "push", "origin", "main"); err != nil {
		return err
	}
	return nil
}

// clearFetchCaches wipes the side's $HOME/.tu/cache so a step running under a
// different TUDIFF_FIXTURES alias re-fetches instead of replaying the records
// an earlier step cached under the same (tool, period, args) key.
func (lr *liveRunner) clearFetchCaches() error {
	return os.RemoveAll(filepath.Join(lr.side.home, ".tu", "cache"))
}

// fabricateRebaseMerge plants the minimal abortable .git/rebase-merge state
// (the shape internal/sync/flow_test.go's real-git recovery test uses) so the
// next sync hits the interrupted-rebase recovery path.
func fabricateRebaseMerge(repo string, env []string) error {
	head, err := liveGit(env, repo, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	head = strings.TrimSpace(head)
	rm := filepath.Join(repo, ".git", "rebase-merge")
	if err := os.MkdirAll(rm, 0o755); err != nil {
		return err
	}
	for name, content := range map[string]string{
		"head-name": "refs/heads/main\n",
		"onto":      head + "\n",
		"orig-head": head + "\n",
	} {
		if err := os.WriteFile(filepath.Join(rm, name), []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// buildLiveAlias copies the committed placeholder corpus to dst and raises
// the cc daily totalCost of one date to newCost — the foreign-commit step's
// fresh local change, without touching the committed corpus.
func buildLiveAlias(src, dst, date string, newCost float64) error {
	if err := harness.CopyTree(src, dst); err != nil {
		return err
	}
	dailyPath := filepath.Join(dst, "claude", "daily.json")
	raw, err := os.ReadFile(dailyPath)
	if err != nil {
		return err
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return err
	}
	daily, ok := doc["daily"].([]any)
	if !ok {
		return fmt.Errorf("tudiff: %s: no daily array", dailyPath)
	}
	found := false
	for _, e := range daily {
		entry, ok := e.(map[string]any)
		if ok && entry["date"] == date {
			entry["totalCost"] = newCost
			found = true
		}
	}
	if !found {
		return fmt.Errorf("tudiff: %s: no daily entry for %s", dailyPath, date)
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(dailyPath, append(out, '\n'), 0o644)
}

// --- repair ---

// liveRepairEntry is the R11 fixture's day-file line (repair_test.go's
// repairEntry): cost is a string so the fixture controls the exact bytes.
func liveRepairEntry(label, cost string, tokens int) string {
	return fmt.Sprintf(`{"label":%q,"totalCost":%s,"inputTokens":100,"outputTokens":50,"cacheCreationTokens":0,"cacheReadTokens":0,"totalTokens":%d}`,
		label, cost, tokens) + "\n"
}

// stageRepairRepo seeds the R11 fixture history in one repo and copies it
// once — the capture/compare side's repair target.
func (lr *liveRunner) stageRepairRepo() (string, error) {
	src := filepath.Join(lr.cfg.tmpRoot, "repair-src")
	if err := seedRepairHistory(src, lr.baseEnv); err != nil {
		return "", err
	}
	repo := filepath.Join(lr.cfg.tmpRoot, "repair-repo")
	if err := harness.CopyTree(src, repo); err != nil {
		return "", err
	}
	return repo, nil
}

// seedRepairHistory builds the R11 fixture (internal/sync/repair_test.go's
// seedR11Repo): one file shrunk from a 10.00 peak to 1.00, one that never
// shrank, one with an unparseable historical blob, one deleted at a later
// commit and re-added shrunk.
func seedRepairHistory(repo string, env []string) error {
	write := func(rel, content string) error {
		full := filepath.Join(repo, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		return os.WriteFile(full, []byte(content), 0o644)
	}
	commit := func(msg string) error {
		if _, err := liveGit(env, repo, "add", "-A"); err != nil {
			return err
		}
		_, err := liveGit(env, repo, "commit", "-m", msg)
		return err
	}
	if err := os.MkdirAll(repo, 0o755); err != nil {
		return err
	}
	if _, err := liveGit(env, "", "init", repo); err != nil {
		return err
	}
	const dir = "u/2026/m"
	if err := write(dir+"/cc-2026-01-01.jsonl", liveRepairEntry("2026-01-01", "10", 900)); err != nil {
		return err
	}
	if err := write(dir+"/cc-2026-01-02.jsonl", liveRepairEntry("2026-01-02", "3", 300)); err != nil {
		return err
	}
	if err := write(dir+"/cc-2026-01-03.jsonl", "totally not json\n"); err != nil {
		return err
	}
	if err := write(dir+"/cc-2026-01-04.jsonl", liveRepairEntry("2026-01-04", "5", 500)); err != nil {
		return err
	}
	if err := commit("high-water marks"); err != nil {
		return err
	}
	if err := write(dir+"/cc-2026-01-03.jsonl", liveRepairEntry("2026-01-03", "8", 800)); err != nil {
		return err
	}
	if _, err := liveGit(env, repo, "rm", "-q", dir+"/cc-2026-01-04.jsonl"); err != nil {
		return err
	}
	if err := commit("garbage out, 04 deleted"); err != nil {
		return err
	}
	if err := write(dir+"/cc-2026-01-01.jsonl", liveRepairEntry("2026-01-01", "1", 10)); err != nil {
		return err
	}
	if err := write(dir+"/cc-2026-01-03.jsonl", liveRepairEntry("2026-01-03", "1", 10)); err != nil {
		return err
	}
	if err := write(dir+"/cc-2026-01-04.jsonl", liveRepairEntry("2026-01-04", "2", 20)); err != nil {
		return err
	}
	return commit("post-purge shrink")
}

// repairStep runs the repair flow's dry-run or --write half on the staged
// repair repo. Compare mode loads live/<id>/ (repo path stored as $REPO) and
// compares the output and the repo tree; update modes write that golden.
func (lr *liveRunner) repairStep(id string, repo string, write bool) harness.Result {
	caseArgs := []string{"repair"}
	if write {
		caseArgs = append(caseArgs, "--write")
	}
	c := liveCase(id, caseArgs)
	if lr.fatal != nil {
		return harness.Result{Case: c}
	}
	dir := harness.GoldenLiveDir(lr.cfg.goldenDir, id)
	cap := lr.repair(repo, write)
	cap.Stdout = normalizeRepo(cap.Stdout, repo)
	cap.Stderr = normalizeRepo(cap.Stderr, repo)
	cap.Stdout = bytes.ReplaceAll(cap.Stdout, []byte(lr.cfg.tmpRoot), []byte("$TMP"))
	cap.Stderr = bytes.ReplaceAll(cap.Stderr, []byte(lr.cfg.tmpRoot), []byte("$TMP"))
	cap.Stdout = harness.NormalizeIdentity(cap.Stdout, lr.cfg.hostname, lr.cfg.username)
	cap.Stderr = harness.NormalizeIdentity(cap.Stderr, lr.cfg.hostname, lr.cfg.username)
	if lr.cfg.update {
		res := harness.Result{Case: c, Status: harness.StatusGreen, NodeExit: cap.Exit, GoExit: cap.Exit}
		if err := lr.writeRepairGolden(dir, cap, repo); err != nil {
			lr.fatal = fmt.Errorf("step %s: %w", id, err)
			return harnessFailStep(id, caseArgs, err)
		}
		return res
	}
	goldenCap, goldenTree, err := harness.LoadGoldenCase(dir, c.IO)
	if err != nil {
		return harnessFailStep(id, caseArgs, err)
	}
	res := harness.Compare(c, goldenCap, cap)
	actual, err := harness.TreeSnapshotRepo(repo)
	if err != nil {
		redden(&res, "tree", strconv.Quote(err.Error()), strconv.Quote(err.Error()))
	} else if diff := harness.CompareTreeSnapshot(goldenTree,
		harness.NormalizeTreeIdentity(actual, lr.cfg.hostname, lr.cfg.username)); diff != nil {
		redden(&res, "tree", diff.NodeExcerpt, diff.GoExcerpt)
	}
	if err := harness.WriteCaseCaptures(lr.cfg.reportDir, res, cap); err != nil {
		redden(&res, "harness", strconv.Quote(err.Error()), strconv.Quote(err.Error()))
	}
	return res
}

// writeRepairGolden writes one repair step's golden: the capture (repo path
// already normalized to $REPO; the repair output has no version surface) and
// the repo's gitless tree.json.
func (lr *liveRunner) writeRepairGolden(dir string, cap harness.SideCapture, repo string) error {
	if cap.TimedOut {
		return fmt.Errorf("capture timed out")
	}
	if cap.Err != "" {
		return fmt.Errorf("capture failed: %s", cap.Err)
	}
	tree, err := harness.TreeSnapshotRepo(repo)
	if err != nil {
		return err
	}
	tree = harness.NormalizeTreeIdentity(tree, lr.cfg.hostname, lr.cfg.username)
	if err := harness.WriteGoldenCase(dir, cap, tree); err != nil {
		return err
	}
	lr.captured++
	return nil
}

// repairGo runs bin/turepair against the repair repo.
func (lr *liveRunner) repairGo(repo string, write bool) harness.SideCapture {
	env := pinnedGitEnv(os.Getenv("PATH"), lr.cfg.tmpRoot)
	return harness.RunPipe(lr.cfg.turepairPath,
		append([]string{"--repo", repo}, writeFlag(write)...), lr.cfg.tmpRoot, env, liveTimeout)
}

func writeFlag(write bool) []string {
	if write {
		return []string{"--write"}
	}
	return nil
}

// normalizeRepo replaces every occurrence of the repo path with $REPO.
func normalizeRepo(b []byte, repo string) []byte {
	return bytes.ReplaceAll(b, []byte(repo), []byte("$REPO"))
}

// --- per-step checks ---

// redden marks a green result red with the check's channel; an already-red
// result keeps its first channel.
func redden(res *harness.Result, channel, nodeExcerpt, goExcerpt string) {
	if res.Status != harness.StatusGreen {
		return
	}
	res.Status = harness.StatusRed
	res.Channel = channel
	res.NodeExcerpt = nodeExcerpt
	res.GoExcerpt = goExcerpt
}

// checkEqual reddens on a byte difference between two harness-side outputs,
// with quoted excerpts from the first differing byte.
func checkEqual(res *harness.Result, channel, node, goText string) {
	if node == goText {
		return
	}
	n := min(len(node), len(goText))
	off := n
	for i := 0; i < n; i++ {
		if node[i] != goText[i] {
			off = i
			break
		}
	}
	redden(res, channel, harness.Excerpt([]byte(node), off), harness.Excerpt([]byte(goText), off))
}

// sideLog is the clone's `git log --format=%H%n%s -p main` bytes — hashes
// included, which the pinned dates make reproducible across runs.
func (lr *liveRunner) sideLog() (string, error) {
	return liveGit(lr.baseEnv, lr.side.repo, "log", "--format=%H%n%s", "-p", "main")
}

// checkStderrContains reddens unless both positions' stderr carries needle
// (byte equality alone cannot catch the corpus AND the binary both silently
// missing a mandated line).
func checkStderrContains(res *harness.Result, oracleCap, cap harness.SideCapture, needle string) {
	o := bytes.Contains(oracleCap.Stderr, []byte(needle))
	g := bytes.Contains(cap.Stderr, []byte(needle))
	if !o || !g {
		redden(res, "expect",
			fmt.Sprintf("stderr contains %q: %t", needle, o),
			fmt.Sprintf("stderr contains %q: %t", needle, g))
	}
}

// checkExit reddens unless both positions exited with want (in compare mode
// the oracle position is the golden's stored exit code).
func checkExit(res *harness.Result, want int) {
	if res.NodeExit != want || res.GoExit != want {
		redden(res, "expect",
			fmt.Sprintf("exit %d, want %d", res.NodeExit, want),
			fmt.Sprintf("exit %d, want %d", res.GoExit, want))
	}
}
