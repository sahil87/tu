// Command tu is the Go successor implementation of the tu CLI.
//
// It is built and tested in CI but NOT shipped: the Homebrew formula and every
// release artifact still come from src/node/ until the cutover change flips the
// formula (constitution v1.2.0 § Go Transition). Plan and decision log:
// fab/plans/sahil/26-09-15-go-port.md.
//
// The binary answers the snapshot and history grammars in single AND multi
// mode (tu, tu cc, tu m, tu h, tu cc mh --since/--until/--full,
// --json/--csv/--md, --no-color, --fresh, -u <user> / -u all against the
// metrics repo, the single-mode -u notice, and the metrics-dir auto-clone
// guard's stderr lines), the leaderboards lb/lbh in multi mode (user ranking,
// previous-period deltas, --top, --by-machine on lb, the exit-1 single-mode
// gate), the setup commands (init-conf, init-metrics, status), the sync
// surfaces (tu sync, tu sync --dry-run, and --sync on a data command), the
// toolkit surfaces (help/-h/--help, help-dump, skill, shell-init, update),
// and the watch mode (-w on every display, with --no-rain/--interval and the
// flags' silent acceptance without -w) for real; the only remaining
// recognized-but-unported request is --skip-brew-update on a data command,
// which keeps the scaffold's placeholder the differential harness diffs
// against.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"time"

	// TZ data for hosts without /usr/share/zoneinfo: the harness's tz:alt axis
	// (TZ=Asia/Kolkata) must resolve everywhere Node's bundled ICU does. Go
	// consults the embed only when the system database is missing.
	_ "time/tzdata"

	"golang.org/x/term"

	"github.com/sahil87/tu/internal/command"
	"github.com/sahil87/tu/internal/config"
	"github.com/sahil87/tu/internal/render/ansi"
	"github.com/sahil87/tu/internal/source"
	"github.com/sahil87/tu/internal/source/cache"
	"github.com/sahil87/tu/internal/source/ccusage"
	"github.com/sahil87/tu/internal/source/metrics"
	metricsync "github.com/sahil87/tu/internal/sync"
	"github.com/sahil87/tu/internal/toolkit"
	"github.com/sahil87/tu/internal/watch"
)

// version is the binary version, overridden via -ldflags "-X main.version=..." at build time.
// `just go-build` stamps it from package.json (the single version anchor during the
// transition) so `--version` byte-matches the shipped TypeScript binary.
var version = "dev"

const notImplementedMsg = "tu: not implemented (Go port in progress)"

// Interface satisfaction at the edge, where the adapters are assigned:
// *ccusage.Source is the live Fetcher for both command and sync,
// metrics.Source the metrics-repo Repo, and sync.Writer the own-user
// day-file writer command applies before every multi-mode repo read.
var (
	_ command.Fetcher    = (*ccusage.Source)(nil)
	_ command.Repo       = metrics.Source{}
	_ command.Writer     = metricsync.Writer{}
	_ metricsync.Fetcher = (*ccusage.Source)(nil)
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// currentUsername is the edge's Env.Username: os/user Current().Username (the
// TS safeUsername source; Load substitutes "unknown" on error).
func currentUsername() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	return u.Username, nil
}

// defaultWidth is the width budget when stdout is not a TTY (a pipe, a
// bytes.Buffer in tests, or a probe error) — the TS process.stdout.columns ?? 80.
// COLUMNS is never consulted (DC-12).
const defaultWidth = 80

// terminalWidth reports the stdout TTY width, or defaultWidth when stdout is
// not a terminal (or the size probe fails). Only cmd/tu probes; renderers
// receive the width as a value.
func terminalWidth(stdout io.Writer) int {
	f, ok := stdout.(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) {
		return defaultWidth
	}
	w, _, err := term.GetSize(int(f.Fd()))
	if err != nil || w <= 0 {
		return defaultWidth
	}
	return w
}

// run is the testable entry point: it dispatches on args and writes to the
// given streams, returning the process exit code instead of exiting. It is the
// ONLY writer: nothing below cmd/tu touches stdout/stderr or calls os.Exit.
//
// The order mirrors the TS main(): grammar parse (usage errors, exit 2) →
// version (after validation) → non-data command dispatch → $HOME config check →
// config.Load cascade (warnings, reserved-user guard) → fetch/render via
// command.Run → warnings → lines.
func run(args []string, stdout, stderr io.Writer) int {
	req, uerr := command.Parse(args)
	if uerr != nil {
		fmt.Fprintln(stderr, uerr.Message)
		if uerr.ShowUsage {
			fmt.Fprintln(stderr, command.ShortUsage)
		}
		return command.ExitUsage
	}
	if req.Version {
		fmt.Fprintln(stdout, toolkit.VersionLine(version))
		return command.ExitOK
	}

	env := config.Env{Getenv: os.Getenv, Hostname: os.Hostname, Username: currentUsername}

	if req.Command != "" {
		return runCommand(req, env, stdout, stderr)
	}

	paths, err := config.ResolvePaths(os.Getenv("HOME"))
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return command.ExitOperational
	}
	cfg, warnings := config.Load(paths, env, config.Overrides{})
	for _, w := range warnings {
		fmt.Fprintln(stderr, w)
	}
	// The metrics-dir auto-clone guard (the TS checkMetricsDirGuard) runs
	// between Load and the reserved-user check on the data path only — setup
	// commands never clone here (init-metrics has its own interactive clone;
	// status only reads). Its lines go to stderr in order.
	cfg, guardLines := config.MetricsDirGuard(cfg, config.StateDir(paths.Home), time.Now(), metricsync.Exec{})
	writeLines(stderr, guardLines)
	// The reserved-user guard (the TS assertUserNotReserved) runs right after
	// the guard: a bad config value is invocation-fixable, so it exits with
	// the usage code.
	if cfg.User == "all" {
		fmt.Fprintln(stderr, `Error: config user "all" is reserved (used by -u all)`)
		return command.ExitUsage
	}

	// Unreachable after the HOME check above; a nil store means no caching.
	store, _ := cache.Default()
	src := &ccusage.Source{Cache: store, User: cfg.User, Machine: cfg.Machine}
	deps := command.Deps{
		Source: src,
		Repo:   metrics.Source{Dir: cfg.MetricsDir},
		Writer: metricsync.Writer{Dir: cfg.MetricsDir},
		Now:    time.Now,
		Colors: ansi.Colors{Enabled: !req.Flags.NoColor && os.Getenv("NO_COLOR") == ""},
		Width:  terminalWidth(stdout),
		// The leaderboard footer's staleness text — a closure like Now,
		// evaluated only by the lb path so no other command reads the file.
		LastSync: func() string { return config.LastSync(config.StateDir(paths.Home), time.Now()) },
	}

	// The --sync block (TS main() lines 1931–1938): it sits after the
	// reserved-user guard and before the normalize notices — the edge prints
	// it before command.Run, and Run's notices only reach stderr after Run
	// returns, so the byte order matches. Single mode stays silent (no line,
	// no git call). The sync's fetch warms the shared cache, so Run's own
	// fetch makes no new ccusage calls (six total for `tu --sync` on a cold
	// cache, not twelve).
	if req.Flags.Sync && cfg.Mode == config.Multi {
		fmt.Fprint(stderr, "syncing metrics... ")
		ctx, cancel := syncCtx()
		out, err := metricsync.FullSync(ctx, metricsync.Inputs{
			Config:   cfg,
			StateDir: config.StateDir(paths.Home),
			Now:      time.Now(),
			Source:   src,
			Git:      metricsync.Exec{},
		}, false)
		cancel()
		if err != nil {
			// A filesystem failure — the TS crashes uncaught; print, exit 1.
			fmt.Fprintln(stderr, err.Error())
			return command.ExitOperational
		}
		source.WriteWarnings(stderr, out.Warnings)
		writeLines(stderr, out.Lines)
		if out.OK {
			fmt.Fprintln(stderr, "synced.")
		} else {
			fmt.Fprintln(stderr, "sync failed — using local data.")
		}
	}

	// The watch branch (the TS main() watch dispatch): after the reserved-user
	// guard and the --sync block, before the one-shot command.Run.
	if req.Flags.Watch {
		return runWatchBranch(req, cfg, deps, stdout, stderr)
	}

	res, err := command.Run(context.Background(), req, cfg, deps)
	if errors.Is(err, command.ErrUnported) {
		fmt.Fprintln(stderr, notImplementedMsg)
		return command.ExitOperational
	}
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return command.ExitOperational
	}
	// The TS main() order: guard notices first (printed before any fetch),
	// then the source fetch warnings, then stdout.
	for _, n := range res.Notices {
		fmt.Fprintln(stderr, n)
	}
	source.WriteWarnings(stderr, res.Warnings)
	for _, l := range res.Lines {
		fmt.Fprintln(stdout, l)
	}
	return command.ExitOK
}

// The watch seams (testability): the terminal constructor and the loop
// itself. Production uses the process's real streams and watch.Run; the e2e
// tests substitute a fake terminal over their buffers and a scripted loop.
var (
	newWatchTerminal = func() watch.Terminal { return watch.NewTerminal(os.Stdout, os.Stdin) }
	runWatchLoop     = watch.Run
)

// runWatchBranch is the -w path (the TS watch dispatch in main(), cli.ts
// 2029–2049). Order: the guard notices print ONCE, ahead of the alt screen
// (the TS prints them in main() before runWatch); the lb single-mode gate
// fires the same way (exit 1, no alt screen); then the loop owns the
// terminal until q/Ctrl-C/SIGINT, and the last rendered lines go to stdout
// with exit 0.
func runWatchBranch(req command.Request, cfg config.Config, deps command.Deps, stdout, stderr io.Writer) int {
	// The notices come from one edge call to Normalize; per-poll
	// Result.Notices are discarded (the --full notice would otherwise repeat
	// every poll — Normalize does not clear it).
	_, notices, _ := command.Normalize(req, cfg.Mode, time.Now())
	// The polls run the ORIGINAL request — Run normalizes per poll.
	for _, n := range notices {
		fmt.Fprintln(stderr, n)
	}
	// The leaderboard gate before the alt screen, exactly as the one-shot
	// path (Run returns it with no notices, no fetch, no lines).
	if (req.Display == command.Leaderboard || req.Display == command.LeaderboardHistory) && cfg.Mode == config.Single {
		fmt.Fprintln(stderr, command.ErrLeaderboardMode.Error())
		return command.ExitOperational
	}

	poll := func(ctx context.Context, f watch.Frame) ([]string, watch.Stats, error) {
		d := deps
		d.Width = f.Width
		prev := f.Prev
		if prev == nil && (req.Display == command.Leaderboard || req.Display == command.LeaderboardHistory) {
			// The TS passes _lastRenderCostMap — a live Map, empty on the
			// first poll — so the leaderboards reserve the indicator column
			// from the first frame (a nil map would not).
			prev = map[string]float64{}
		}
		d.Live = &command.LiveOptions{Prev: prev, Compact: f.Compact, MaxRows: f.MaxRows}
		reqFresh := req
		reqFresh.Flags.Fresh = true // the TS action(true, …): every poll bypasses the 60 s cache
		res, err := command.Run(ctx, reqFresh, cfg, d)
		if err != nil {
			return nil, watch.Stats{}, err
		}
		// The TS fetcher warns on stderr every poll, alt screen or not.
		source.WriteWarnings(stderr, res.Warnings)
		return res.Lines, watch.Stats{TotalCost: res.TotalCost, TotalTokens: res.TotalTokens, CostByItem: res.CostByItem}, nil
	}

	seed := uint64(time.Now().UnixNano())
	last := runWatchLoop(context.Background(), watch.Options{
		Interval: req.Flags.Interval,
		NoRain:   req.Flags.NoRain,
		Poll:     poll,
		Term:     newWatchTerminal(),
		Stderr:   stderr,
		Colors:   deps.Colors,
		Now:      time.Now,
		Rand:     rand.New(rand.NewPCG(seed, ^seed)),
	})
	for _, l := range last {
		fmt.Fprintln(stdout, l)
	}
	return command.ExitOK
}

// runCommand dispatches a non-data command. The toolkit commands — help,
// help-dump, skill, shell-init, update — are answered for real BEFORE
// config.ResolvePaths: none of them needs $HOME. init-conf, init-metrics,
// status, and sync resolve paths first. Data flags on a non-data command are
// ignored (DC-02) — Parse sets Command regardless of flags and the handlers
// never look at Format/Flags (sync reads only --dry-run). Every command the
// grammar names is now handled; the default keeps the scaffold's placeholder
// as defense.
func runCommand(req command.Request, env config.Env, stdout, stderr io.Writer) int {
	switch req.Command {
	case "help", "-h", "--help":
		// console.log(FULL_HELP) appends exactly one newline.
		fmt.Fprintln(stdout, command.FullHelp)
		return command.ExitOK
	case "help-dump":
		// The envelope's version is bare while the binary is stamped with a
		// leading v. Encode writes JSON.stringify(doc, null, 2) + "\n".
		toolkit.BuildHelpDoc(toolkit.BareVersion(version), command.FullHelp+"\n").Encode(stdout)
		return command.ExitOK
	case "skill":
		toolkit.WriteSkill(stdout)
		return command.ExitOK
	case "shell-init":
		return runShellInit(req.Args, stdout, stderr)
	case "update":
		return runUpdate(req, stdout, stderr)
	case "init-conf", "init-metrics", "status", "sync":
		// handled below
	default:
		fmt.Fprintln(stderr, notImplementedMsg)
		return command.ExitOperational
	}

	paths, err := config.ResolvePaths(os.Getenv("HOME"))
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return command.ExitOperational
	}

	switch req.Command {
	case "init-conf":
		lines, err := config.InitConf(paths)
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return command.ExitOperational
		}
		writeLines(stdout, lines)
		return command.ExitOK
	case "sync":
		return runSync(req, env, paths, stdout, stderr)
	case "status":
		data, warnings := config.Status(paths, env, time.Now())
		writeLines(stderr, warnings)
		writeLines(stdout, data.Lines())
		return command.ExitOK
	default: // init-metrics
		var url *string
		if len(req.Args) > 0 {
			url = &req.Args[0]
		}
		res, err := config.InitMetrics(paths, env, url, metricsync.Exec{})
		writeLines(stderr, res.Warnings)
		writeLines(stdout, res.Lines)
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return command.ExitOperational
		}
		if res.Clone != nil {
			// The clone runs at the edge with the process streams passed
			// through (the TS stdio "inherit": git's "Cloning into …" chatter
			// reaches the user's stderr).
			if cerr := (metricsync.Exec{}).Clone(context.Background(), res.Clone.URL, res.Clone.Dir, os.Stdin, stdout, stderr); cerr != nil {
				fmt.Fprintf(stderr, "Error: git clone failed (exit %d).\n", cloneExitCode(cerr))
				return command.ExitOperational
			}
			config.RemoveCloneMarker(config.StateDir(paths.Home))
			fmt.Fprintln(stdout, config.ClonedLine(res.Clone.URL, res.Clone.Dir))
		}
		return command.ExitOK
	}
}

// syncCtx is the fetch deadline for both sync entry points — the same
// source.DefaultTimeout command.Run applies to the data fetch (the TS fetches
// under the same timeout either way).
func syncCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), source.DefaultTimeout)
}

// runSync implements `tu sync` in the TS runSync order (src/node/core/cli.ts):
// Load (warnings to stderr) → the reserved-user guard (BEFORE the mode check)
// → the single-mode gate → the metrics-dir guard (a demotion exits 1 with
// just the guard's lines, so `tu sync --dry-run` with a missing dir still
// clones first) → the dry-run report on stdout or the live sync. Data flags
// are ignored (DC-02).
func runSync(req command.Request, env config.Env, paths config.Paths, stdout, stderr io.Writer) int {
	cfg, warnings := config.Load(paths, env, config.Overrides{})
	writeLines(stderr, warnings)
	if cfg.User == "all" {
		fmt.Fprintln(stderr, `Error: config user "all" is reserved (used by -u all)`)
		return command.ExitUsage
	}
	if cfg.Mode != config.Multi {
		writeLines(stderr, []string{
			"tu sync requires metrics_repo to be set.",
			"Add metrics_repo to ~/.config/tu/tu.conf, run 'tu init-metrics <repo-url>', or set TU_METRICS_REPO.",
		})
		return command.ExitOperational
	}
	cfg, guardLines := config.MetricsDirGuard(cfg, config.StateDir(paths.Home), time.Now(), metricsync.Exec{})
	writeLines(stderr, guardLines)
	if cfg.Mode != config.Multi {
		// Auto-clone failed or the metrics dir is still missing (demoted).
		return command.ExitOperational
	}

	// The source is built identically to the data path's deps.
	store, _ := cache.Default()
	src := &ccusage.Source{Cache: store, User: cfg.User, Machine: cfg.Machine}
	inputs := metricsync.Inputs{
		Config:   cfg,
		StateDir: config.StateDir(paths.Home),
		Now:      time.Now(),
		Source:   src,
		Git:      metricsync.Exec{},
	}
	ctx, cancel := syncCtx()
	defer cancel()

	if req.Flags.DryRun {
		out, err := metricsync.FullSync(ctx, inputs, true)
		if err != nil {
			// A filesystem failure — the TS crashes uncaught; print, exit 1.
			fmt.Fprintln(stderr, err.Error())
			return command.ExitOperational
		}
		source.WriteWarnings(stderr, out.Warnings)
		writeLines(stdout, out.Report.Format(paths.Home))
		return command.ExitOK
	}

	out, err := metricsync.FullSync(ctx, inputs, false)
	if err != nil {
		// A filesystem failure — the TS crashes uncaught; print, exit 1.
		fmt.Fprintln(stderr, err.Error())
		return command.ExitOperational
	}
	source.WriteWarnings(stderr, out.Warnings)
	writeLines(stderr, out.Lines)
	if !out.OK {
		fmt.Fprintln(stderr, "Error: sync failed — check network and remote config.")
		return command.ExitOperational
	}
	fmt.Fprintln(stdout, "Synced to "+config.Tildefy(cfg.MetricsDir, paths.Home))
	return command.ExitOK
}

// cloneExitCode extracts the child's exit code; a non-exit failure (e.g. git
// not found) reports 1, the TS uncaught-exception exit.
func cloneExitCode(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return 1
}

func writeLines(w io.Writer, lines []string) {
	for _, l := range lines {
		fmt.Fprintln(w, l)
	}
}

// runShellInit implements `tu shell-init`: a missing shell is the usage block
// on stderr, an unknown shell the unknown-shell line — both exit 2 with stdout
// EMPTY (stdout may be eval'd); a known shell's script goes to stdout with no
// added newline. Arguments after the shell are ignored (the TS reads
// filteredArgs[1] only).
func runShellInit(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, toolkit.ShellInitUsage)
		return command.ExitUsage
	}
	script, ok := toolkit.Completion(args[0])
	if !ok {
		fmt.Fprintln(stderr, toolkit.UnknownShellMessage(args[0]))
		return command.ExitUsage
	}
	stdout.Write(script)
	return command.ExitOK
}

// runUpdate implements `tu update` in the TS runUpdate order, sequencing the
// pure/driver pieces of internal/toolkit and printing the wrapper lines itself
// (cmd/tu is the only writer; the interactive `brew upgrade` streams pass
// through the driver call).
func runUpdate(req command.Request, stdout, stderr io.Writer) int {
	// The update standard's flag-discovery probe: --help/-h anywhere in the
	// args prints the full help and runs nothing (the help carries the
	// literal --skip-brew-update).
	for _, a := range req.Args {
		if a == "--help" || a == "-h" {
			fmt.Fprintln(stdout, command.FullHelp)
			return command.ExitOK
		}
	}
	// The Homebrew gate tests the symlink-resolved executable (dev builds,
	// the go test binary and the R2 dogfood install all land off-Homebrew).
	resolved, err := os.Executable()
	if err == nil {
		resolved, err = filepath.EvalSymlinks(resolved)
	}
	if err != nil || !toolkit.IsBrewInstall(resolved) {
		writeLines(stdout, toolkit.NotBrewInstallLines(version))
		return command.ExitOK
	}

	ctx := context.Background()
	brew := toolkit.BrewExec{}
	fmt.Fprintln(stdout, toolkit.CurrentVersionLine(version))
	latest, uerr := toolkit.CheckLatest(ctx, brew, req.Flags.SkipBrewUpdate)
	if uerr != nil {
		fmt.Fprintln(stderr, uerr.Message)
		return command.ExitOperational
	}
	if toolkit.UpToDate(version, latest) {
		fmt.Fprintln(stdout, toolkit.AlreadyUpToDateLine(version))
		return command.ExitOK
	}
	fmt.Fprintln(stdout, toolkit.UpdatingLine(version, latest))
	if uerr := toolkit.Upgrade(ctx, brew, os.Stdin, stdout, stderr); uerr != nil {
		fmt.Fprintln(stderr, uerr.Message)
		return command.ExitOperational
	}
	fmt.Fprintln(stdout, toolkit.UpdatedLine(latest))
	return command.ExitOK
}
