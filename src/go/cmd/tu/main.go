// Command tu is the Go successor implementation of the tu CLI.
//
// It is built and tested in CI but NOT shipped: the Homebrew formula and every
// release artifact still come from src/node/ until the cutover change flips the
// formula (constitution v1.2.0 § Go Transition). Plan and decision log:
// fab/plans/sahil/26-09-15-go-port.md.
//
// This scaffold implements only the toolkit `version` contract. Every other
// invocation is a deliberate not-implemented placeholder that the differential
// harness (plan row P4) starts from; the real command surface lands in later
// rows.
package main

import (
	"fmt"
	"io"
	"os"
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

// run is the testable entry point: it dispatches on args and writes to the given
// streams, returning the process exit code instead of exiting.
func run(args []string, stdout, stderr io.Writer) int {
	if hasVersionFlag(args) {
		fmt.Fprintln(stdout, versionLine(version))
		return 0
	}
	fmt.Fprintln(stderr, notImplementedMsg)
	return 1
}

// hasVersionFlag mirrors the TypeScript CLI's `rawArgs.includes(...)` check: any of
// the three version flags anywhere in args selects the version path.
func hasVersionFlag(args []string) bool {
	for _, a := range args {
		switch a {
		case "--version", "-V", "-v":
			return true
		}
	}
	return false
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
