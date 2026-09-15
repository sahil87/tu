// Command fakegit is the fake git of the differential harness (plan rows
// P3a/P4 in fab/plans/sahil/26-09-15-go-port.md). Built as bin/harness/git, it
// sits first on PATH so every git invocation tu makes is recorded to the
// shared call log, and answers from an optional script ($TUDIFF_GIT_SCRIPT)
// of prefix-matched rules; with no script or no matching rule it is a silent
// exit-0 stub. It performs no filesystem or network operations of its own —
// the D11/B6 live-sync parity check uses real git against a temp bare repo;
// this fake exists for the deterministic, network-free matrix and for
// comparing the *sequence* of git calls between implementations.
package main

import (
	"encoding/json"
	"io"
	"os"

	"github.com/sahil87/tu/internal/harness"
)

// gitScriptEnv names the optional JSON array of response rules.
const gitScriptEnv = "TUDIFF_GIT_SCRIPT"

// gitRule is one scripted response: a prefix match on argv (after stripping a
// leading `-C <dir>` pair) plus the stdout/stderr/exit to answer with.
type gitRule struct {
	Match  []string `json:"match"`
	Stdout string   `json:"stdout"`
	Stderr string   `json:"stderr"`
	Exit   int      `json:"exit"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}

// run is the testable entry point: env is the environment lookup, so tests
// inject values instead of mutating the process environment.
func run(args []string, env func(string) string, stdout, stderr io.Writer) int {
	harness.LogCall("git", args, "")

	argv := stripDashC(args)
	for _, rule := range parseScript(env(gitScriptEnv)) {
		if prefixMatch(rule.Match, argv) {
			_, _ = io.WriteString(stdout, rule.Stdout)
			_, _ = io.WriteString(stderr, rule.Stderr)
			return rule.Exit
		}
	}
	return 0
}

// stripDashC drops a leading `-C <dir>` pair, matching how tu invokes git
// (execFile("git", ["-C", dir, …]) in sync.ts).
func stripDashC(args []string) []string {
	if len(args) >= 2 && args[0] == "-C" {
		return args[2:]
	}
	return args
}

// parseScript decodes $TUDIFF_GIT_SCRIPT; a missing or malformed script
// yields no rules (the fake then answers every call with silent exit 0).
func parseScript(raw string) []gitRule {
	if raw == "" {
		return nil
	}
	var rules []gitRule
	if err := json.Unmarshal([]byte(raw), &rules); err != nil {
		return nil
	}
	return rules
}

func prefixMatch(prefix, argv []string) bool {
	if len(prefix) > len(argv) {
		return false
	}
	for i, p := range prefix {
		if argv[i] != p {
			return false
		}
	}
	return true
}
