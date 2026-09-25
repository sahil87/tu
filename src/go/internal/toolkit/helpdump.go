package toolkit

import (
	"encoding/json"
	"io"
	"strings"
)

const (
	// Tool is the toolkit contract's tool name ("tu").
	Tool = "tu"
	// Description is the CLI's one-line description, carried as a Go constant
	// (the long-term shape; the retired TS implementation read it from its
	// package metadata).
	Description = "AI coding assistant cost tracking CLI"
	// HelpSchemaVersion is the help-dump contract schema version.
	HelpSchemaVersion = 1
)

// HelpNode is one node in the serialized CLI help document. The JSON shape is
// a frozen cross-repo contract (shll.ai's pull cron consumes it): field order
// and the never-nil "commands" array are part of that contract — do not
// reorder or rename. tu's document is flat: one root node, "commands": [].
type HelpNode struct {
	Name     string     `json:"name"`
	Path     string     `json:"path"`
	Short    string     `json:"short"`
	Usage    string     `json:"usage"`
	Text     string     `json:"text"`
	Commands []HelpNode `json:"commands"`
}

// HelpDoc is the top-level envelope: exactly {tool, version, schema_version,
// root}. It deliberately does NOT carry captured_at (the capture timestamp is
// owned by shll.ai's puller) nor an aliases key (omitted entirely when there
// are none). Version is the BARE form ("0.11.5", never "v0.11.5").
type HelpDoc struct {
	Tool          string   `json:"tool"`
	Version       string   `json:"version"`
	SchemaVersion int      `json:"schema_version"`
	Root          HelpNode `json:"root"`
}

// BuildHelpDoc assembles the flat document. version is the bare version (the
// caller passes BareVersion(version)); helpText is the raw help text verbatim
// (the caller passes command.FullHelp + "\n", byte-identical to `tu --help`).
func BuildHelpDoc(version, helpText string) HelpDoc {
	return HelpDoc{
		Tool:          Tool,
		Version:       version,
		SchemaVersion: HelpSchemaVersion,
		Root: HelpNode{
			Name:     Tool,
			Path:     Tool,
			Short:    Description,
			Usage:    extractUsage(helpText),
			Text:     helpText,
			Commands: []HelpNode{},
		},
	}
}

// Encode writes the document exactly as the TS `JSON.stringify(doc, null, 2)
// + "\n"` does. SetEscapeHTML(false) is load-bearing: encoding/json escapes
// `<`, `>` and `&` by default where JSON.stringify does not, and the help text
// carries `<date>`, `<m>`, `<n>`, `<s>`, `<user>`, `<sh>`. Encoder.Encode
// appends the trailing "\n" the TS adds explicitly.
func (d HelpDoc) Encode(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(d)
}

// extractUsage returns the first line of helpText starting with "Usage:",
// falling back to the first non-empty line (the TS extractUsage).
func extractUsage(helpText string) string {
	lines := strings.Split(helpText, "\n")
	for _, l := range lines {
		if strings.HasPrefix(l, "Usage:") {
			return l
		}
	}
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			return l
		}
	}
	return ""
}
