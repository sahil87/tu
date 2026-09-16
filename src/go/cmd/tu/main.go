// Command tu is the Go successor implementation of the tu CLI.
//
// It is built and tested in CI but NOT shipped: the Homebrew formula and every
// release artifact still come from src/node/ until the cutover change flips the
// formula (constitution v1.2.0 § Go Transition). Plan and decision log:
// fab/plans/sahil/26-09-15-go-port.md.
//
// The binary answers the single-mode snapshot grammar (tu, tu cc, tu m, --json,
// --no-color, --fresh, …) for real; everything else prints the deliberate
// not-implemented placeholder the differential harness diffs against.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	// TZ data for hosts without /usr/share/zoneinfo: the harness's tz:alt axis
	// (TZ=Asia/Kolkata) must resolve everywhere Node's bundled ICU does. Go
	// consults the embed only when the system database is missing.
	_ "time/tzdata"

	"github.com/sahil87/tu/internal/command"
	"github.com/sahil87/tu/internal/config"
	"github.com/sahil87/tu/internal/render/ansi"
	"github.com/sahil87/tu/internal/source"
	"github.com/sahil87/tu/internal/source/cache"
	"github.com/sahil87/tu/internal/source/ccusage"
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

// run is the testable entry point: it dispatches on args and writes to the
// given streams, returning the process exit code instead of exiting. It is the
// ONLY writer: nothing below cmd/tu touches stdout/stderr or calls os.Exit.
//
// The order mirrors the TS main(): grammar parse (usage errors, exit 2) →
// version (after validation) → non-data command placeholder → $HOME config
// check → mode detection → fetch/render via command.Run → warnings → lines.
func run(args []string, stdout, stderr io.Writer) int {
	req, uerr := command.Parse(args)
	if uerr != nil {
		fmt.Fprintln(stderr, uerr.Message)
		if uerr.ShowUsage {
			fmt.Fprintln(stderr, command.ShortUsage)
		}
		return 2
	}
	if req.Version {
		fmt.Fprintln(stdout, versionLine(version))
		return 0
	}
	if req.Command != "" {
		fmt.Fprintln(stderr, notImplementedMsg)
		return 1
	}

	paths, err := config.ResolvePaths(os.Getenv("HOME"))
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	mode := config.DetectMode(paths, os.Getenv)

	// Unreachable after the HOME check above; a nil store means no caching.
	store, _ := cache.Default()
	src := &ccusage.Source{Cache: store}
	deps := command.Deps{
		Source: src,
		Now:    time.Now,
		Colors: ansi.Colors{Enabled: !req.Flags.NoColor && os.Getenv("NO_COLOR") == ""},
	}

	res, err := command.Run(context.Background(), req, mode, deps)
	if errors.Is(err, command.ErrUnported) {
		fmt.Fprintln(stderr, notImplementedMsg)
		return 1
	}
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	source.WriteWarnings(stderr, res.Warnings)
	for _, l := range res.Lines {
		fmt.Fprintln(stdout, l)
	}
	return 0
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
