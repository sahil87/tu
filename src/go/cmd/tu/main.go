// Command tu is the Go successor implementation of the tu CLI.
//
// It is built and tested in CI but NOT shipped: the Homebrew formula and every
// release artifact still come from src/node/ until the cutover change flips the
// formula (constitution v1.2.0 § Go Transition). Plan and decision log:
// fab/plans/sahil/26-09-15-go-port.md.
//
// The binary answers the single-mode snapshot and history grammars (tu,
// tu cc, tu m, tu h, tu cc mh --since/--until/--full, --json/--csv/--md,
// --no-color, --fresh, …) and the setup commands (init-conf, init-metrics,
// status) for real; everything else prints the deliberate not-implemented
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
	metricsync "github.com/sahil87/tu/internal/sync"
)

// version is the binary version, overridden via -ldflags "-X main.version=..." at build time.
// `just go-build` stamps it from package.json (the single version anchor during the
// transition) so `--version` byte-matches the shipped TypeScript binary.
var version = "dev"

const (
	toolName          = "tu"
	notImplementedMsg = "tu: not implemented (Go port in progress)"
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
		fmt.Fprintln(stdout, versionLine(version))
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
	// The reserved-user guard (the TS assertUserNotReserved) runs right after
	// readConfig: a bad config value is invocation-fixable, so it exits with
	// the usage code. (B3's metrics-dir guard slots between Load and this
	// check when it lands.)
	if cfg.User == "all" {
		fmt.Fprintln(stderr, `Error: config user "all" is reserved (used by -u all)`)
		return command.ExitUsage
	}

	// Unreachable after the HOME check above; a nil store means no caching.
	store, _ := cache.Default()
	src := &ccusage.Source{Cache: store}
	deps := command.Deps{
		Source: src,
		Now:    time.Now,
		Colors: ansi.Colors{Enabled: !req.Flags.NoColor && os.Getenv("NO_COLOR") == ""},
		Width:  terminalWidth(stdout),
	}

	res, err := command.Run(context.Background(), req, cfg.Mode, deps)
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

// runCommand dispatches a non-data command: init-conf, init-metrics, and
// status are answered for real; every other command stays on the scaffold's
// placeholder. Data flags on a setup command are ignored (DC-02) — Parse sets
// Command regardless of flags and the handlers never look at Format/Flags.
func runCommand(req command.Request, env config.Env, stdout, stderr io.Writer) int {
	switch req.Command {
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

// versionLine renders the toolkit version standard's recommended shape,
// `<tool> version vX.Y.Z`. A value that starts with a digit gets a `v` prefix so
// both `-X main.version=0.11.5` and `-X main.version=v0.11.5` print identically;
// anything else (the unstamped `dev` fallback) is printed as-is.
func versionLine(v string) string {
	if len(v) > 0 && v[0] >= '0' && v[0] <= '9' {
		v = "v" + v
	}
	return toolName + " version " + v
}
