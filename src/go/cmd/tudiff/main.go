// Command tudiff is the differential-harness driver for the Go port (plan
// rows P3a and P4 in fab/plans/sahil/26-09-15-go-port.md; constitution v1.2.0
// § Go Transition): `capture` records real ccusage output into
// harness/fixtures/<machine-alias>/, `placeholder` writes the schema-derived,
// committed placeholder corpus (real captures carry spend data and stay
// local), and `run` byte-diffs node dist/tu.mjs against the Go binary over
// harness/matrix.json. `live` is the real-git half of the gate: the intake
// § 10 sync/repair sequence against temp bare repos (the fake git stays
// unused), reported under bin/harness/report-live/.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sahil87/tu/internal/harness"
)

const usageText = `usage: tudiff <capture|placeholder|run|live> [flags]
  capture      record real ccusage output into harness/fixtures/<machine-alias>/
  placeholder  write the schema-derived placeholder corpus (all six sources by default)
  run          byte-diff node dist/tu.mjs against the Go binary over harness/matrix.json
  live         real-git parity: the sync/repair sequence against temp bare repos
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is the testable entry point: it dispatches on the subcommand and writes
// to the given streams, returning the process exit code instead of exiting.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usageText)
		return 2
	}
	switch args[0] {
	case "capture":
		return runCapture(args[1:], stdout, stderr)
	case "placeholder":
		return runPlaceholder(args[1:], stdout, stderr)
	case "run":
		return runRun(args[1:], stdout, stderr)
	case "live":
		return runLive(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "tudiff: unknown subcommand %q\n%s", args[0], usageText)
		return 2
	}
}

// splitComma parses a comma-separated flag value; empty yields nil so the
// callee's default applies.
func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

func runCapture(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("capture", flag.ContinueOnError)
	fs.SetOutput(stderr)
	machine := fs.String("machine", "", "fixture alias (default: hostname)")
	ccusage := fs.String("ccusage", "", "path to the ccusage binary (default: dist/vendor → node_modules → PATH)")
	out := fs.String("out", "harness/fixtures", "fixture root")
	sources := fs.String("sources", "", "comma-separated ccusage subcommands (default: all six)")
	periods := fs.String("periods", "", "comma-separated periods (default: daily)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "tudiff: %v\n", err)
		return 1
	}
	root, err := harness.FindRepoRoot(cwd)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	binary, err := harness.ResolveCcusage(root, *ccusage)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	_, err = harness.Capture(harness.CaptureOptions{
		Machine:     *machine,
		CcusagePath: binary,
		OutDir:      *out,
		Sources:     splitComma(*sources),
		Periods:     splitComma(*periods),
		RepoRoot:    root,
	}, stdout)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

// sourceList collects the repeatable --source flag.
type sourceList []string

func (s *sourceList) String() string { return strings.Join(*s, ",") }
func (s *sourceList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func runPlaceholder(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("placeholder", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var sources sourceList
	fs.Var(&sources, "source", "ccusage subcommand to generate a placeholder for (repeatable; default: all six)")
	out := fs.String("out", "harness/fixtures/"+harness.PlaceholderAlias, "placeholder alias directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if len(sources) == 0 {
		sources = append(sources, harness.DefaultSources...)
	}
	if err := harness.WritePlaceholders(*out, sources); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	// Report from the manifest just written — one source of truth for what
	// the ledger merge actually produced.
	m, err := harness.ReadManifest(*out)
	if err != nil {
		fmt.Fprintf(stderr, "tudiff: cannot read back %s/manifest.json: %v\n", *out, err)
		return 1
	}
	for _, fx := range m.Fixtures {
		state := "(unconfirmed)"
		if fx.ConfirmedBy != nil {
			state = fmt.Sprintf("(confirmed: %s %s)", fx.ConfirmedBy.Machine, fx.ConfirmedBy.Date)
		}
		fmt.Fprintf(stdout, "%s %s %s  placeholder %s  -> %s/%s\n", fx.Source, fx.Period, strings.Join(fx.Args, " "), state, *out, fx.File)
	}
	return 0
}
