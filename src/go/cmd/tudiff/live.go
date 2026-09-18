package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sahil87/tu/internal/harness"
)

// live.go is the `tudiff live` subcommand (intake § 10, plan R14): the
// real-git half of the differential gate. Where `run` answers every git call
// with the fake, live seeds two identical bare remotes from
// harness/metrics-repo/, points each side's staged multi home at a real clone,
// and runs the sync/repair sequence with the real git on PATH — fixed
// identity and commit dates make identical trees yield identical hashes, so
// the two sides' `git log -p` outputs compare byte for byte.

const (
	defaultTurepair   = "bin/turepair"
	defaultLiveReport = "bin/harness/report-live"

	// fixedGitDate pins every commit timestamp the live sequence creates
	// (seed, sync, foreign, repair fixture), so identical trees and messages
	// yield identical commit hashes on both sides.
	fixedGitDate = "2026-01-09T12:00:00Z"

	// liveTimeout bounds one side of one live step (real git over local
	// bares; far above anything the sequence should need).
	liveTimeout = 60 * time.Second

	// rebaseRecoveryNeedle is the stderr line the interrupted-rebase step
	// must produce on both sides (internal/sync rebaseRecoveryLine).
	rebaseRecoveryNeedle = "Warning: recovering from interrupted rebase"
)

// liveOptions holds the parsed live flags; set records explicit flags (same
// resolution rule as run).
type liveOptions struct {
	node, goBin, turepair, harnessBin, report, expected string
	keep                                                bool
	set                                                 map[string]bool
}

func runLive(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("live", flag.ContinueOnError)
	fs.SetOutput(stderr)
	opts := liveOptions{set: map[string]bool{}}
	fs.StringVar(&opts.node, "node", defaultNode, "TS oracle bundle (run as `node <staged copy>`)")
	fs.StringVar(&opts.goBin, "go", defaultGo, "Go binary under test")
	fs.StringVar(&opts.turepair, "turepair", defaultTurepair, "Go repair binary (parried against scripts/repair-metrics.mjs)")
	fs.StringVar(&opts.harnessBin, "harness-bin", defaultHarnessBin, "directory holding the fake ccusage (the fake git is deliberately unused here)")
	fs.StringVar(&opts.report, "report", defaultLiveReport, "report directory (wiped at the start of a run)")
	fs.StringVar(&opts.expected, "expected", defaultExpected, "expected-diffs file (DC-keyed intentional divergences)")
	fs.BoolVar(&opts.keep, "keep", false, "keep the temp dir (bares, clones, staged homes) for inspection")
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
	nodeBin, exp, code := preflightLive(&opts, root, fail)
	if code != 0 {
		return code
	}
	return executeLive(&opts, root, nodeBin, exp, stdout, stderr)
}

// resolve absolutizes a path flag with the shared run/live rule.
func (o *liveOptions) resolve(root, name, value string) string {
	return resolvePathFlag(root, o.set[name], value)
}

// preflightLive runs the intake § 10.1 checks: real git and node on PATH,
// the oracle bundle, both Go binaries, the fake ccusage, and the placeholder
// manifest the fake replays from, then loads the expected-diffs file (same
// R3 messages as run: a missing file is a preflight error, never an empty
// set).
func preflightLive(opts *liveOptions, root string, fail func(string, ...any) int) (nodeBin string, exp *harness.Expected, code int) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", nil, fail("git not found on PATH (live diffs the real git, not the fake)")
	}
	if !fileExists(opts.resolve(root, "node", opts.node)) {
		return "", nil, fail("%s not found (run npm ci && npm run build)", opts.node)
	}
	for _, b := range []struct{ name, path string }{
		{"go", opts.goBin},
		{"turepair", opts.turepair},
	} {
		p := opts.resolve(root, b.name, b.path)
		if !fileExists(p) {
			return "", nil, fail("%s not found (run just go-build)", b.path)
		}
		if !executable(p) {
			return "", nil, fail("%s not executable (run just go-build)", b.path)
		}
	}
	cc := filepath.Join(opts.resolve(root, "harness-bin", opts.harnessBin), "ccusage")
	if !fileExists(cc) {
		return "", nil, fail("%s not found (run just harness-build)", filepath.Join(opts.harnessBin, "ccusage"))
	}
	if !executable(cc) {
		return "", nil, fail("%s not executable (run just harness-build)", filepath.Join(opts.harnessBin, "ccusage"))
	}
	nodeBin, err := exec.LookPath("node")
	if err != nil {
		return "", nil, fail("node not found on PATH (required to run the TS oracle)")
	}
	manifest := filepath.Join(root, "harness", "fixtures", harness.PlaceholderAlias, "manifest.json")
	if !fileExists(manifest) {
		return "", nil, fail("harness/fixtures/%s/manifest.json not found", harness.PlaceholderAlias)
	}
	exp, err = harness.LoadExpected(opts.resolve(root, "expected", opts.expected))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil, fail("%s not found", opts.expected)
		}
		return "", nil, fail("expected-diffs: %v", err)
	}
	return nodeBin, exp, 0
}

// liveConfig carries the resolved, per-run constants shared by every step.
type liveConfig struct {
	root, tmpRoot, livebin, bundle              string
	nodeBin, goPath, turepairPath, repairScript string
	seedDir, placeholder, reportDir             string
	keep                                        bool
	expected                                    *harness.Expected
}

// liveSide holds one side's staged paths.
type liveSide struct {
	name string // "node" / "go"
	dir  string // working directory (<tmp>/<side>/)
	home string // staged $HOME (<dir>/home)
	repo string // <home>/.tu/metrics_repo — a real clone of the side's bare
	bare string // <tmp>/<side>.git
}

// liveRunner holds the live sequence's shared state.
type liveRunner struct {
	cfg     liveConfig
	baseEnv []string // pinned identity/date env for the harness's own git calls
	node    liveSide
	goSide  liveSide
}

func executeLive(opts *liveOptions, root, nodeBin string, exp *harness.Expected, stdout, stderr io.Writer) int {
	cfg := liveConfig{
		root:         root,
		nodeBin:      nodeBin,
		goPath:       opts.resolve(root, "go", opts.goBin),
		turepairPath: opts.resolve(root, "turepair", opts.turepair),
		repairScript: filepath.Join(root, "scripts", "repair-metrics.mjs"),
		seedDir:      filepath.Join(root, "harness", "metrics-repo"),
		placeholder:  filepath.Join(root, "harness", "fixtures", harness.PlaceholderAlias),
		reportDir:    opts.resolve(root, "report", opts.report),
		keep:         opts.keep,
		expected:     exp,
	}
	fail := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "tudiff: "+format+"\n", a...)
		return 2
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
	if !cfg.keep {
		defer os.RemoveAll(tmpRoot)
	}

	// livebin holds ONLY the fake ccusage: the child PATH is livebin followed
	// by the process PATH, so the fake git is deliberately absent — every git
	// a tu child spawns is the real one.
	cfg.livebin = filepath.Join(tmpRoot, "livebin")
	if err := harness.CopyFile(filepath.Join(opts.resolve(root, "harness-bin", opts.harnessBin), "ccusage"),
		filepath.Join(cfg.livebin, "ccusage"), 0o755); err != nil {
		return fail("staging livebin: %v", err)
	}
	bundle, err := harness.StageOracle(tmpRoot, opts.resolve(root, "node", opts.node),
		filepath.Join(root, "tu.default.conf"), filepath.Join(opts.resolve(root, "harness-bin", opts.harnessBin), "ccusage"))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	cfg.bundle = bundle

	lr := &liveRunner{cfg: cfg, baseEnv: pinnedGitEnv(os.Getenv("PATH"), os.Getenv("HOME"))}
	if err := lr.stage(); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	header := harness.ReportHeader{
		Timestamp:       time.Now().UTC(),
		NodePath:        opts.node,
		NodeVersion:     firstLine(exec.Command(nodeBin, "--version").Output()),
		GoPath:          opts.goBin,
		GoVersion:       firstLine(exec.Command(cfg.goPath, "--version").Output()),
		Fixtures:        []string{harness.PlaceholderAlias, "live-alias"},
		MatrixPath:      "live",
		ExpectedPath:    opts.expected,
		ExpectedEntries: len(exp.Entries),
	}
	results := lr.runSequence(stdout)
	header.Cases = len(results)
	summary := harness.SummarizeResults(exp, results)
	for _, line := range harness.RenderSummary(summary, header.Fixtures) {
		fmt.Fprintln(stdout, line)
	}
	if err := harness.WriteReport(cfg.reportDir, header, exp, results); err != nil {
		fmt.Fprintf(stderr, "tudiff: writing report: %v\n", err)
		return 2
	}
	if cfg.keep {
		fmt.Fprintf(stderr, "tudiff: kept temp dir %s\n", tmpRoot)
	}
	// The R5 gate rule, same as run.
	if summary.Unexpected+summary.Timeout+summary.Unconfirmed > 0 || len(summary.Stale) > 0 {
		return 1
	}
	return 0
}

// stage seeds the two bare remotes and both sides' staged homes.
func (lr *liveRunner) stage() error {
	nodeBare, goBare, err := seedLiveRemotes(lr.cfg.tmpRoot, lr.cfg.seedDir, lr.baseEnv)
	if err != nil {
		return err
	}
	for _, sp := range []struct {
		side *liveSide
		name string
		bare string
	}{
		{&lr.node, string(harness.SideNode), nodeBare},
		{&lr.goSide, string(harness.SideGo), goBare},
	} {
		s, err := stageLiveSide(lr.cfg.tmpRoot, sp.name, sp.bare, lr.cfg.seedDir, lr.baseEnv)
		if err != nil {
			return err
		}
		*sp.side = s
	}
	return nil
}

// runSequence runs the intake § 10.3 sequence plus the repair parity flow,
// streaming one report line per step in sequence order.
func (lr *liveRunner) runSequence(stdout io.Writer) []harness.Result {
	var results []harness.Result
	emit := func(res harness.Result) {
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

	// a. sync --dry-run: the report, and neither tree mutates.
	res, _, _ := lr.compareStep("sync-dry-run", []string{"sync", "--dry-run"}, lr.cfg.placeholder)
	lr.checkStatusClean(&res)
	emit(res)

	// b. sync: the round trip; then trees, status, logs, .last-sync.
	res, _, _ = lr.compareStep("sync", []string{"sync"}, lr.cfg.placeholder)
	lr.checkCloneTrees(&res)
	lr.checkStatusClean(&res)
	lr.checkLogsEqual(&res)
	lr.checkLastSyncPresent(&res)
	emit(res)

	// c. sync again: the steady-state no-commit path; the log is unchanged.
	beforeNode, errN := lr.sideLog(lr.node)
	beforeGo, errG := lr.sideLog(lr.goSide)
	res, _, _ = lr.compareStep("sync-again", []string{"sync"}, lr.cfg.placeholder)
	if errN != nil || errG != nil {
		redden(&res, "log", fmt.Sprint(errN), fmt.Sprint(errG))
	} else {
		afterNode, errN := lr.sideLog(lr.node)
		afterGo, errG := lr.sideLog(lr.goSide)
		if errN != nil || errG != nil {
			redden(&res, "log", fmt.Sprint(errN), fmt.Sprint(errG))
		} else {
			checkEqual(&res, "log", beforeNode, afterNode)
			checkEqual(&res, "log", beforeGo, afterGo)
		}
	}
	emit(res)

	// d. foreign commit: a third clone of each bare pushes a day-file; each
	// side then syncs a fresh local change (the one-off alias raises cc
	// 2026-01-07) and pull --rebase integrates. The fetch cache key excludes
	// TUDIFF_FIXTURES and steps a–c warmed it, so both sides' .tu/cache must go
	// first — otherwise the alias's raised value is never read and the step
	// exercises only a clean pull.
	if err := lr.pushForeignCommit(); err != nil {
		emit(harnessFailStep("foreign-sync", []string{"sync"}, err))
	} else {
		alias := filepath.Join(lr.cfg.tmpRoot, "live-alias")
		if err := buildLiveAlias(lr.cfg.placeholder, alias, "2026-01-07", 0.9); err != nil {
			emit(harnessFailStep("foreign-sync", []string{"sync"}, err))
		} else if err := lr.clearFetchCaches(); err != nil {
			emit(harnessFailStep("foreign-sync", []string{"sync"}, err))
		} else {
			res, _, _ = lr.compareStep("foreign-sync", []string{"sync"}, alias)
			lr.checkLogsEqual(&res)
			lr.checkCloneTrees(&res)
			lr.checkStatusClean(&res)
			emit(res)
		}
	}

	// e. interrupted rebase: a fabricated .git/rebase-merge in both clones;
	// sync recovers and proceeds.
	var recoveryErr error
	for _, side := range []liveSide{lr.node, lr.goSide} {
		if err := fabricateRebaseMerge(side.repo, lr.baseEnv); err != nil && recoveryErr == nil {
			recoveryErr = err
		}
	}
	if recoveryErr != nil {
		emit(harnessFailStep("rebase-recovery", []string{"sync"}, recoveryErr))
	} else {
		res, nodeCap, goCap := lr.compareStep("rebase-recovery", []string{"sync"}, lr.aliasOrPlaceholder())
		checkStderrContains(&res, nodeCap, goCap, rebaseRecoveryNeedle)
		lr.checkStatusClean(&res)
		emit(res)
	}

	// f. pull failure: both clones point origin at a missing bare; sync fails
	// with the pull-failure bytes and exit 1 (same real git, same remote
	// path, so the text matches after home normalization). The working
	// origin is restored afterwards so cc --sync can run.
	var remoteErr error
	missing := filepath.Join(lr.cfg.tmpRoot, "missing.git")
	for _, side := range []liveSide{lr.node, lr.goSide} {
		if _, err := liveGit(lr.baseEnv, side.repo, "remote", "set-url", "origin", missing); err != nil && remoteErr == nil {
			remoteErr = err
		}
	}
	if remoteErr != nil {
		emit(harnessFailStep("pull-failure", []string{"sync"}, remoteErr))
	} else {
		res, _, _ = lr.compareStep("pull-failure", []string{"sync"}, lr.aliasOrPlaceholder())
		checkExit(&res, 1)
		emit(res)
	}
	for _, side := range []liveSide{lr.node, lr.goSide} {
		_, _ = liveGit(lr.baseEnv, side.repo, "remote", "set-url", "origin", side.bare)
	}

	// g. cc --sync: the sync line on stderr, the table on stdout.
	res, _, _ = lr.compareStep("cc-sync", []string{"cc", "--sync"}, lr.aliasOrPlaceholder())
	lr.checkStatusClean(&res)
	emit(res)

	// Repair parity (intake § 10.4): the R11 fixture history seeded once,
	// copied twice; the mjs and the Go binary run dry-run then --write, with
	// each side's repo path normalized to $REPO before comparing.
	repoA, repoB, err := lr.stageRepairRepos()
	if err != nil {
		emit(harnessFailStep("repair-dry-run", []string{"repair"}, err))
	} else {
		emit(lr.repairStep("repair-dry-run", repoA, repoB, false))
		res := lr.repairStep("repair-write", repoA, repoB, true)
		lr.checkRepairTrees(&res, repoA, repoB)
		emit(res)
	}

	return results
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
// harness-level failure before either side executed).
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

// compareStep runs one tu argv on both sides back-to-back (re-run once on a
// UTC date rollover, the runCase rule) and byte-compares the captures after
// home normalization.
func (lr *liveRunner) compareStep(id string, args []string, fixtures string) (harness.Result, harness.SideCapture, harness.SideCapture) {
	c := liveCase(id, args)
	before := localDateInTZ(harness.TZFixedName)
	nodeCap := lr.execSide(true, args, fixtures)
	goCap := lr.execSide(false, args, fixtures)
	rerun := false
	if localDateInTZ(harness.TZFixedName) != before {
		// Midnight rollover between the two sides: discard and re-run once.
		rerun = true
		nodeCap = lr.execSide(true, args, fixtures)
		goCap = lr.execSide(false, args, fixtures)
	}
	nodeCap.Home, goCap.Home = lr.node.home, lr.goSide.home
	res := harness.Compare(c, nodeCap, goCap)
	res.Rerun = rerun
	if err := harness.WriteCaseCaptures(lr.cfg.reportDir, res, nodeCap, goCap); err != nil {
		redden(&res, "harness", strconv.Quote(err.Error()), strconv.Quote(err.Error()))
	}
	return res, nodeCap, goCap
}

// execSide runs one side of a step: the staged oracle bundle through node, or
// the Go binary in place.
func (lr *liveRunner) execSide(node bool, args []string, fixtures string) harness.SideCapture {
	side := lr.goSide
	name, argv := lr.cfg.goPath, args
	if node {
		side = lr.node
		name, argv = lr.cfg.nodeBin, append([]string{lr.cfg.bundle}, args...)
	}
	return harness.RunPipe(name, argv, side.dir, lr.childEnv(side.home, fixtures), liveTimeout)
}

// childEnv is one tu child's environment: livebin first on PATH (fake
// ccusage, real git), the staged HOME, TZ=UTC, the pinned git identity/date,
// and the fixture search path.
func (lr *liveRunner) childEnv(home, fixtures string) []string {
	env := pinnedGitEnv(lr.cfg.livebin+string(os.PathListSeparator)+os.Getenv("PATH"), home)
	return append(env, "TUDIFF_FIXTURES="+fixtures)
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

// seedLiveRemotes builds one bare repo on main from seedDir (a scratch clone
// receives the full tree, docs/README.md included, one "seed" commit, push)
// and copies it to <tmp>/node.git and <tmp>/go.git — two identical remotes,
// never shared.
func seedLiveRemotes(tmpRoot, seedDir string, env []string) (nodeBare, goBare string, err error) {
	seed := filepath.Join(tmpRoot, "seed.git")
	if _, err := liveGit(env, tmpRoot, "init", "--bare", "--initial-branch=main", seed); err != nil {
		return "", "", err
	}
	scratch := filepath.Join(tmpRoot, "scratch")
	if _, err := liveGit(env, tmpRoot, "clone", seed, scratch); err != nil {
		return "", "", err
	}
	if err := harness.CopyTree(seedDir, scratch); err != nil {
		return "", "", err
	}
	if _, err := liveGit(env, scratch, "add", "-A"); err != nil {
		return "", "", err
	}
	if _, err := liveGit(env, scratch, "commit", "-m", "seed"); err != nil {
		return "", "", err
	}
	if _, err := liveGit(env, scratch, "push", "origin", "main"); err != nil {
		return "", "", err
	}
	nodeBare = filepath.Join(tmpRoot, "node.git")
	goBare = filepath.Join(tmpRoot, "go.git")
	if err := harness.CopyTree(seed, nodeBare); err != nil {
		return "", "", err
	}
	if err := harness.CopyTree(seed, goBare); err != nil {
		return "", "", err
	}
	return nodeBare, goBare, nil
}

// stageLiveSide stages one side's $HOME as the multi variant, then REPLACES
// the seeded .tu/metrics_repo copy with a real clone of the side's bare.
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

// pushForeignCommit clones each bare a third time, adds the other-user
// day-file, commits with the pinned date, and pushes — the upstream change
// the next sync's pull --rebase integrates. Identical tree, message, parent,
// and dates make the two bares' new commits hash-equal.
func (lr *liveRunner) pushForeignCommit() error {
	const foreignFile = "other-user/2026/laptop/cc-2026-01-08.jsonl"
	const foreignBody = `{"label":"2026-01-08","totalCost":2.5,"inputTokens":6000,"outputTokens":800,"cacheCreationTokens":2000,"cacheReadTokens":40000,"totalTokens":48800}` + "\n"
	for _, side := range []liveSide{lr.node, lr.goSide} {
		clone := filepath.Join(lr.cfg.tmpRoot, "foreign-"+side.name)
		if _, err := liveGit(lr.baseEnv, lr.cfg.tmpRoot, "clone", side.bare, clone); err != nil {
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
	}
	return nil
}

// clearFetchCaches wipes both sides' $HOME/.tu/cache so a step running under a
// different TUDIFF_FIXTURES alias re-fetches instead of replaying the records
// an earlier step cached under the same (tool, period, args) key.
func (lr *liveRunner) clearFetchCaches() error {
	for _, side := range []liveSide{lr.node, lr.goSide} {
		if err := os.RemoveAll(filepath.Join(side.home, ".tu", "cache")); err != nil {
			return err
		}
	}
	return nil
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

// --- repair parity ---

// liveRepairEntry is the R11 fixture's day-file line (repair_test.go's
// repairEntry): cost is a string so the fixture controls the exact bytes.
func liveRepairEntry(label, cost string, tokens int) string {
	return fmt.Sprintf(`{"label":%q,"totalCost":%s,"inputTokens":100,"outputTokens":50,"cacheCreationTokens":0,"cacheReadTokens":0,"totalTokens":%d}`,
		label, cost, tokens) + "\n"
}

// stageRepairRepos seeds the R11 fixture history in one repo and copies it
// twice — the mjs repairs A, the Go binary repairs B.
func (lr *liveRunner) stageRepairRepos() (repoA, repoB string, err error) {
	src := filepath.Join(lr.cfg.tmpRoot, "repair-src")
	if err := seedRepairHistory(src, lr.baseEnv); err != nil {
		return "", "", err
	}
	repoA = filepath.Join(lr.cfg.tmpRoot, "repair-node")
	repoB = filepath.Join(lr.cfg.tmpRoot, "repair-go")
	if err := harness.CopyTree(src, repoA); err != nil {
		return "", "", err
	}
	if err := harness.CopyTree(src, repoB); err != nil {
		return "", "", err
	}
	return repoA, repoB, nil
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

// repairStep runs the mjs against repoA and bin/turepair against repoB and
// compares the captures, each side's repo path replaced by $REPO first (the
// paths differ by construction; Home stays empty so Compare adds nothing).
func (lr *liveRunner) repairStep(id string, repoA, repoB string, write bool) harness.Result {
	caseArgs := []string{"repair"}
	if write {
		caseArgs = append(caseArgs, "--write")
	}
	c := liveCase(id, caseArgs)
	env := pinnedGitEnv(os.Getenv("PATH"), lr.cfg.tmpRoot)
	nodeCap := harness.RunPipe(lr.cfg.nodeBin,
		append([]string{lr.cfg.repairScript, "--repo", repoA}, writeFlag(write)...), lr.cfg.tmpRoot, env, liveTimeout)
	goCap := harness.RunPipe(lr.cfg.turepairPath,
		append([]string{"--repo", repoB}, writeFlag(write)...), lr.cfg.tmpRoot, env, liveTimeout)
	nodeCap.Stdout = normalizeRepo(nodeCap.Stdout, repoA)
	nodeCap.Stderr = normalizeRepo(nodeCap.Stderr, repoA)
	goCap.Stdout = normalizeRepo(goCap.Stdout, repoB)
	goCap.Stderr = normalizeRepo(goCap.Stderr, repoB)
	res := harness.Compare(c, nodeCap, goCap)
	if err := harness.WriteCaseCaptures(lr.cfg.reportDir, res, nodeCap, goCap); err != nil {
		redden(&res, "harness", strconv.Quote(err.Error()), strconv.Quote(err.Error()))
	}
	return res
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

// sideLog is the side's `git log --format=%H%n%s -p main` bytes — hashes
// included, which the pinned dates make comparable across sides.
func (lr *liveRunner) sideLog(side liveSide) (string, error) {
	return liveGit(lr.baseEnv, side.repo, "log", "--format=%H%n%s", "-p", "main")
}

// checkLogsEqual reddens when the two clones' log -p outputs differ.
func (lr *liveRunner) checkLogsEqual(res *harness.Result) {
	n, errN := lr.sideLog(lr.node)
	g, errG := lr.sideLog(lr.goSide)
	if errN != nil || errG != nil {
		redden(res, "log", fmt.Sprint(errN), fmt.Sprint(errG))
		return
	}
	checkEqual(res, "log", n, g)
}

// checkStatusClean reddens when either clone's `git status --porcelain` is
// non-empty.
func (lr *liveRunner) checkStatusClean(res *harness.Result) {
	n, errN := liveGit(lr.baseEnv, lr.node.repo, "status", "--porcelain")
	g, errG := liveGit(lr.baseEnv, lr.goSide.repo, "status", "--porcelain")
	if errN != nil || errG != nil {
		redden(res, "status", fmt.Sprint(errN), fmt.Sprint(errG))
		return
	}
	if strings.TrimSpace(n) != "" || strings.TrimSpace(g) != "" {
		redden(res, "status", strconv.Quote(n), strconv.Quote(g))
	}
}

// cloneTree lists the relative slash paths and bytes of every file under
// root, .git excluded — the two clones' .git contents legitimately differ
// (the remote path), their tracked trees must not.
func cloneTree(root string) (map[string][]byte, error) {
	files := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			if rel == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = raw
		return nil
	})
	return files, err
}

// compareTrees reddens on the first path-set or byte difference between two
// file maps (sorted order, so the reported path is stable).
func compareTrees(res *harness.Result, channel string, node, goTree map[string][]byte) {
	paths := make([]string, 0, len(node)+len(goTree))
	for p := range node {
		paths = append(paths, p)
	}
	for p := range goTree {
		if _, ok := node[p]; !ok {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	for _, p := range paths {
		nb, nOK := node[p]
		gb, gOK := goTree[p]
		if nOK != gOK {
			nState, gState := p+": absent", p+": present"
			if nOK {
				nState, gState = gState, nState
			}
			redden(res, channel, nState, gState)
			return
		}
		if !bytes.Equal(nb, gb) {
			m := min(len(nb), len(gb))
			off := m
			for i := 0; i < m; i++ {
				if nb[i] != gb[i] {
					off = i
					break
				}
			}
			redden(res, channel, p+": "+harness.Excerpt(nb, off), p+": "+harness.Excerpt(gb, off))
			return
		}
	}
}

// checkCloneTrees reddens when the two clones' day-file trees (minus .git)
// differ.
func (lr *liveRunner) checkCloneTrees(res *harness.Result) {
	nt, errN := cloneTree(lr.node.repo)
	gt, errG := cloneTree(lr.goSide.repo)
	if errN != nil || errG != nil {
		redden(res, "tree", fmt.Sprint(errN), fmt.Sprint(errG))
		return
	}
	compareTrees(res, "tree", nt, gt)
}

// checkRepairTrees reddens when the two repaired repos' working trees differ.
func (lr *liveRunner) checkRepairTrees(res *harness.Result, repoA, repoB string) {
	nt, errN := cloneTree(repoA)
	gt, errG := cloneTree(repoB)
	if errN != nil || errG != nil {
		redden(res, "tree", fmt.Sprint(errN), fmt.Sprint(errG))
		return
	}
	compareTrees(res, "tree", nt, gt)
}

// checkLastSyncPresent reddens unless .last-sync exists on BOTH sides.
func (lr *liveRunner) checkLastSyncPresent(res *harness.Result) {
	n := liveFilePresent(filepath.Join(lr.node.home, ".tu", ".last-sync"))
	g := liveFilePresent(filepath.Join(lr.goSide.home, ".tu", ".last-sync"))
	if !n || !g {
		nState, gState := ".last-sync: present", ".last-sync: present"
		if !n {
			nState = ".last-sync: absent"
		}
		if !g {
			gState = ".last-sync: absent"
		}
		redden(res, "state", nState, gState)
	}
}

// checkStderrContains reddens unless both sides' stderr carries needle (byte
// equality alone cannot catch both sides silently missing a mandated line).
func checkStderrContains(res *harness.Result, nodeCap, goCap harness.SideCapture, needle string) {
	n := bytes.Contains(nodeCap.Stderr, []byte(needle))
	g := bytes.Contains(goCap.Stderr, []byte(needle))
	if !n || !g {
		redden(res, "expect",
			fmt.Sprintf("stderr contains %q: %t", needle, n),
			fmt.Sprintf("stderr contains %q: %t", needle, g))
	}
}

// checkExit reddens unless both sides exited with want.
func checkExit(res *harness.Result, want int) {
	if res.NodeExit != want || res.GoExit != want {
		redden(res, "expect",
			fmt.Sprintf("exit %d, want %d", res.NodeExit, want),
			fmt.Sprintf("exit %d, want %d", res.GoExit, want))
	}
}

// --- file helpers ---

// liveFilePresent reports whether path exists as a regular file.
func liveFilePresent(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
