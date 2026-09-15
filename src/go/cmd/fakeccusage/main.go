// Command fakeccusage is the fake ccusage replayer of the differential
// harness (plan rows P3a/P4 in fab/plans/sahil/26-09-15-go-port.md). Built as
// bin/harness/ccusage, it impersonates the real binary: configured only by
// environment variables (its argv belongs to tu), it replays recorded fixture
// bytes for an exact (source, period, args) key, exiting with the recorded
// code. A request with no fixture is deliberately loud (exit 2) because a Go
// port sending ccusage an argv the TypeScript binary never sent is itself a
// divergence the harness must surface.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sahil87/tu/internal/harness"
)

// fixturesEnv names the OS-path-list of fixture alias directories, searched
// in order, first hit wins.
const fixturesEnv = "TUDIFF_FIXTURES"

func main() {
	os.Exit(run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}

// run is the testable entry point: env is the environment lookup, so tests
// inject values instead of mutating the process environment.
func run(args []string, env func(string) string, stdout, stderr io.Writer) int {
	fixtures := env(fixturesEnv)
	if fixtures == "" {
		harness.LogCall("ccusage", args, "")
		fmt.Fprintln(stderr, "fakeccusage: TUDIFF_FIXTURES not set")
		return 2
	}
	corpus, err := harness.LoadCorpus(filepath.SplitList(fixtures))
	if err != nil {
		harness.LogCall("ccusage", args, "")
		fmt.Fprintf(stderr, "fakeccusage: %v\n", err)
		return 2
	}

	if len(args) == 1 && (args[0] == "--version" || args[0] == "-v") {
		harness.LogCall("ccusage", args, "")
		if v := corpus.Version(); v != "" {
			fmt.Fprintf(stdout, "ccusage %s\n", v)
			return 0
		}
		fmt.Fprintln(stderr, "fakeccusage: no manifest found in TUDIFF_FIXTURES")
		return 2
	}

	if len(args) >= 2 {
		if hit, ok := corpus.Lookup(args[0], args[1], args[2:]); ok {
			harness.LogCall("ccusage", args, hit.Matched)
			_, _ = stdout.Write(hit.Stdout)
			_, _ = stderr.Write(hit.Stderr)
			return hit.ExitCode
		}
	}
	harness.LogCall("ccusage", args, "")
	fmt.Fprintf(stderr, "fakeccusage: no fixture for argv [%s]\n", strings.Join(args, " "))
	return 2
}
