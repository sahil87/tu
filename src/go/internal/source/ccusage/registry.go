// Package ccusage is the source adapter for the ccusage binary: it owns the
// ordered six-tool registry, argv composition, binary resolution, the
// CommandContext exec with Node-shaped failure details, and JSON
// normalization into fact.Record values.
package ccusage

// Tool is one entry of the ccusage registry.
type Tool struct {
	Key        string   // tu's key: cc, codex, oc, gemini, copilot, kimi
	Name       string   // display name
	PrefixArgs []string // ccusage per-agent subcommand
	LabelKey   string   // JSON key carrying the ISO date label
}

// Tools is the registry in column order (Output Stability: insertion order is
// the all-tools column order; new tools append). It is a slice, never a map:
// Go maps are unordered. Source aliases (co, gem, cop, ki) are command
// grammar and deliberately not part of this package.
var Tools = []Tool{
	{Key: "cc", Name: "Claude Code", PrefixArgs: []string{"claude"}, LabelKey: "date"},
	{Key: "codex", Name: "Codex", PrefixArgs: []string{"codex"}, LabelKey: "date"},
	{Key: "oc", Name: "OpenCode", PrefixArgs: []string{"opencode"}, LabelKey: "date"},
	{Key: "gemini", Name: "Gemini", PrefixArgs: []string{"gemini"}, LabelKey: "date"},
	{Key: "copilot", Name: "Copilot", PrefixArgs: []string{"copilot"}, LabelKey: "date"},
	{Key: "kimi", Name: "Kimi", PrefixArgs: []string{"kimi"}, LabelKey: "date"},
}

// PeriodDaily is the daily period argument.
const PeriodDaily = "daily"

// Lookup returns the tool with the given registry key.
func Lookup(key string) (Tool, bool) {
	for _, tool := range Tools {
		if tool.Key == key {
			return tool, true
		}
	}
	return Tool{}, false
}

// argv composes the argument vector handed to the binary for a fetch:
// PrefixArgs, then the period, then --json, then any extra args (e.g.
// "claude daily --json"). No shell is involved; the caller passes the slice
// to os/exec verbatim.
func argv(tool Tool, period string, extraArgs []string) []string {
	args := append([]string{}, tool.PrefixArgs...)
	args = append(args, period, "--json")
	return append(args, extraArgs...)
}
