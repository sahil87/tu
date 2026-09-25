// Command turepair is the Go port of the retired TS repair script (R11): the
// one-time repair that restores shrunk metrics day-files to their historical
// maximum. It is a standalone maintainer binary — not bundled into the
// shipped CLI, run from a checkout (bin/turepair from `just go-build`; the
// tudiff precedent). main is the only writer; the algorithm lives in
// sync.Repair.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sahil87/tu/internal/config"
	"github.com/sahil87/tu/internal/sync"
)

const usageLine = "Usage: turepair [--repo <path>] [--write]"

// defaultRepo is the retired script's default metrics repo.
const defaultRepo = "~/.tu/metrics_repo"

func main() {
	home, _ := os.UserHomeDir()
	os.Exit(run(os.Args[1:], home, os.Stdout, os.Stderr))
}

// run is the testable entry point: parse args exactly as the retired script's parseArgs
// does, then write Repair's returned lines and return its exit code.
func run(args []string, home string, stdout, stderr io.Writer) int {
	o, failMsg := parseArgs(args, home)
	if failMsg != "" {
		writeLines(stderr, sync.FailLines(failMsg))
		return 1
	}
	out, errLines, exit := sync.Repair(o, sync.Exec{MaxBuffer: sync.MaxBufferRepair})
	writeLines(stdout, out)
	writeLines(stderr, errLines)
	return exit
}

// parseArgs is the retired script's parseArgs: --repo needs a value, --write flips the
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

// resolveRepairHome is the retired script's resolveHome: a leading "~"/"~/" expands
// against home, and every path is then resolved absolute (the retired script's
// path.resolve) so the printed repo path matches byte for byte.
func resolveRepairHome(p, home string) string {
	p = config.ExpandHome(p, home)
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

// writeLines emits one line per entry (Repair's lines joined by "\n" plus a
// trailing "\n" are the retired script's bytes).
func writeLines(w io.Writer, lines []string) {
	for _, l := range lines {
		fmt.Fprintln(w, l)
	}
}
