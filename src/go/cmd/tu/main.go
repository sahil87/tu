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
// gate), the setup commands (init-conf, init-metrics, status), and the
// toolkit surfaces (help/-h/--help, help-dump, skill, shell-init, update) for
// real; everything else (sync, watch) prints the deliberate not-implemented
// placeholder the differential harness diffs against.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
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
)

// version is the binary version, overridden via -ldflags "-X main.version=..." at build time.
// `just go-build` stamps it from package.json (the single version anchor during the
// transition) so `--version` byte-matches the shipped TypeScript binary.
var version = "dev"

const notImplementedMsg = "tu: not implemented (Go port in progress)"

// Interface satisfaction at the edge, where the adapters are assigned:
// *ccusage.Source is the live Fetcher, metrics.Source the metrics-repo Repo.
var (
	_ command.Fetcher = (*ccusage.Source)(nil)
	_ command.Repo    = metrics.Source{}
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
		Now:    time.Now,
		Colors: ansi.Colors{Enabled: !req.Flags.NoColor && os.Getenv("NO_COLOR") == ""},
		Width:  terminalWidth(stdout),
		// The leaderboard footer's staleness text — a closure like Now,
		// evaluated only by the lb path so no other command reads the file.
		LastSync: func() string { return config.LastSync(config.StateDir(paths.Home), time.Now()) },
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

// runCommand dispatches a non-data command. The toolkit commands — help,
// help-dump, skill, shell-init, update — are answered for real BEFORE
// config.ResolvePaths: none of them needs $HOME. init-conf, init-metrics, and
// status resolve paths first; every other command (sync, B6's) stays on the
// scaffold's placeholder. Data flags on a setup command are ignored (DC-02) —
// Parse sets Command regardless of flags and the handlers never look at
// Format/Flags.
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
	case "init-conf", "init-metrics", "status":
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
