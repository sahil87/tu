// Package ccusage is the source adapter for the ccusage binary: it owns the
// adapter-private invocation map (keyed by the fact registry key), argv
// composition, binary resolution, the CommandContext exec with Node-shaped
// failure details, and JSON normalization into fact.Record values. The tool
// registry itself (key, display name, column order) lives in fact.
package ccusage

import "github.com/sahil87/tu/internal/fact"

// invocation is how ccusage is driven for one registry tool: the per-agent
// subcommand and the JSON key carrying the ISO date label.
type invocation struct {
	prefixArgs []string
	labelKey   string
}

// invocations is keyed by fact.Tool.Key; every registry tool has an entry
// (asserted by registry_test.go against fact.Tools).
var invocations = map[string]invocation{
	"cc":      {prefixArgs: []string{"claude"}, labelKey: "date"},
	"codex":   {prefixArgs: []string{"codex"}, labelKey: "date"},
	"oc":      {prefixArgs: []string{"opencode"}, labelKey: "date"},
	"gemini":  {prefixArgs: []string{"gemini"}, labelKey: "date"},
	"copilot": {prefixArgs: []string{"copilot"}, labelKey: "date"},
	"kimi":    {prefixArgs: []string{"kimi"}, labelKey: "date"},
}

// argv composes the argument vector handed to the binary for a fetch: the
// tool's prefix args, then the period, then --json, then any extra args (e.g.
// "claude daily --json"). No shell is involved; the caller passes the slice
// to os/exec verbatim.
func argv(tool fact.Tool, period string, extraArgs []string) []string {
	args := append([]string{}, invocations[tool.Key].prefixArgs...)
	args = append(args, period, "--json")
	return append(args, extraArgs...)
}
