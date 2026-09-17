// Command turepair is the Go twin of scripts/repair-metrics.mjs (R11): the
// one-time repair that restores shrunk metrics day-files to their historical
// maximum. Like the mjs it is a standalone maintainer binary — not bundled
// into the shipped CLI, run from a checkout (bin/turepair from `just
// go-build`; the tudiff precedent). Arg parsing and every emitted byte match
// the mjs, including the usage line's deliberate `node scripts/…` spelling
// (changing it is a follow-up after Z1, so the two outputs diff clean until
// then). main is the only writer; the algorithm lives in sync.Repair.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sahil87/tu/internal/config"
	"github.com/sahil87/tu/internal/sync"
)

// usageLine is the mjs USAGE, verbatim — node spelling included, so the two
// implementations' outputs diff clean (a follow-up after Z1 owns changing it).
const usageLine = "Usage: node scripts/repair-metrics.mjs [--repo <path>] [--write]"

// defaultRepo is the mjs default metrics repo.
const defaultRepo = "~/.tu/metrics_repo"

func main() {
	home, _ := os.UserHomeDir()
	os.Exit(run(os.Args[1:], home, os.Stdout, os.Stderr))
}

// run is the testable entry point: parse args exactly as the mjs parseArgs
// does, then write Repair's returned lines and return its exit code.
func run(args []string, home string, stdout, stderr io.Writer) int {
	o, failMsg := parseArgs(args, home)
	if failMsg != "" {
		writeLines(stderr, sync.FailLines(failMsg))
		return 1
	}
	out, errLines, exit := sync.Repair(o, sync.Exec{})
	writeLines(stdout, out)
	writeLines(stderr, errLines)
	return exit
}

// parseArgs is the mjs parseArgs: --repo needs a value, --write flips the
// mode, anything else is an unknown argument; both failures carry the usage
// line after a newline inside the fail message.
func parseArgs(args []string, home string) (o sync.RepairOptions, failMsg string) {
	o.Repo = resolveRepairHome(defaultRepo, home)
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--repo":
			i++
			if i >= len(args) || args[i] == "" {
				return o, "--repo requires a path\n" + usageLine
			}
			o.Repo = resolveRepairHome(args[i], home)
		case "--write":
			o.Write = true
		default:
			return o, "unknown argument: " + args[i] + "\n" + usageLine
		}
	}
	return o, ""
}

// resolveRepairHome is the mjs resolveHome: a leading "~"/"~/" expands
// against home, and every path is then resolved absolute (the mjs's
// path.resolve) so the printed repo path matches byte for byte.
func resolveRepairHome(p, home string) string {
	p = config.ExpandHome(p, home)
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

// writeLines emits one line per entry (Repair's lines joined by "\n" plus a
// trailing "\n" are the mjs's bytes).
func writeLines(w io.Writer, lines []string) {
	for _, l := range lines {
		fmt.Fprintln(w, l)
	}
}
