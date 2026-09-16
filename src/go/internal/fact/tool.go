package fact

// Tool is one entry of the tool registry: tu's key and the display name.
// Adapter-specific data (how a source is driven for the tool) lives in the
// adapter, keyed by Key.
type Tool struct {
	Key  string // cc, codex, oc, gemini, copilot, kimi
	Name string // Claude Code, Codex, OpenCode, Gemini, Copilot, Kimi
}

// Tools is the registry in column order (Output Stability: insertion order is
// the all-tools column order; new tools append). It is a slice, never a map:
// Go maps are unordered. Source aliases (co, gem, cop, ki) are command
// grammar and deliberately not part of this package.
var Tools = []Tool{
	{Key: "cc", Name: "Claude Code"},
	{Key: "codex", Name: "Codex"},
	{Key: "oc", Name: "OpenCode"},
	{Key: "gemini", Name: "Gemini"},
	{Key: "copilot", Name: "Copilot"},
	{Key: "kimi", Name: "Kimi"},
}

// Lookup returns the tool with the given registry key.
func Lookup(key string) (Tool, bool) {
	for _, tool := range Tools {
		if tool.Key == key {
			return tool, true
		}
	}
	return Tool{}, false
}
