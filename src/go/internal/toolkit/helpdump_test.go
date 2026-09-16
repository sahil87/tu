package toolkit

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// findPackageRoot walks up from the package directory (the test's cwd) to the
// first directory containing package.json — the repo root. ok is false when no
// package.json exists above (post-cutover tree), so transitional guards can
// skip instead of fail.
func findPackageRoot(t *testing.T) (root string, ok bool) {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// TestHelpDocEncodeGolden pins the wire bytes for a fixed input: the envelope
// field order, the bare version, the one-line "commands": [], the trailing
// newline — exactly JSON.stringify(doc, null, 2) + "\n".
func TestHelpDocEncodeGolden(t *testing.T) {
	doc := BuildHelpDoc("1.2.3", "Usage: tu [source] [period] [display]\n\nbody\n")
	var buf bytes.Buffer
	if err := doc.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	want := `{
  "tool": "tu",
  "version": "1.2.3",
  "schema_version": 1,
  "root": {
    "name": "tu",
    "path": "tu",
    "short": "AI coding assistant cost tracking CLI",
    "usage": "Usage: tu [source] [period] [display]",
    "text": "Usage: tu [source] [period] [display]\n\nbody\n",
    "commands": []
  }
}
`
	if buf.String() != want {
		t.Errorf("Encode = %q, want %q", buf.String(), want)
	}
}

// TestHelpDocEncodeNoHTMLEscape pins SetEscapeHTML(false): the help text's
// `<date>` (and friends) must appear raw where encoding/json's default would
// emit <date>. This is the one silent-divergence trap in the port.
func TestHelpDocEncodeNoHTMLEscape(t *testing.T) {
	doc := BuildHelpDoc("0.11.5", "Usage: tu x\n--since / -s <date>  Only on/after\n")
	var buf bytes.Buffer
	if err := doc.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `<date>`) {
		t.Errorf("Encode HTML-escaped the help text: %q", buf.String())
	}
	if strings.Contains(buf.String(), `\u003c`) {
		t.Errorf("Encode emitted an HTML escape: %q", buf.String())
	}
}

func TestExtractUsage(t *testing.T) {
	cases := []struct {
		name, text, want string
	}{
		{"usage line", "Usage: tu [source] [period] [display]\n\nrest\n", "Usage: tu [source] [period] [display]"},
		{"fallback first non-empty", "\n  \nno usage here\nmore\n", "no usage here"},
		{"empty", "", ""},
		{"usage wins over earlier lines", "preamble\nUsage: later\n", "Usage: later"},
	}
	for _, c := range cases {
		if got := extractUsage(c.text); got != c.want {
			t.Errorf("%s: extractUsage(%q) = %q, want %q", c.name, c.text, got, c.want)
		}
	}
}

// TestDescriptionMatchesPackageJSON is the transitional guard for the
// Description constant while package.json still anchors the metadata (it goes
// away at the cutover — then this guard skips).
func TestDescriptionMatchesPackageJSON(t *testing.T) {
	root, ok := findPackageRoot(t)
	if !ok {
		t.Skip("no package.json above the package directory")
	}
	raw, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	var pkg struct {
		Description string `json:"description"`
	}
	if err := json.Unmarshal(raw, &pkg); err != nil {
		t.Fatal(err)
	}
	if pkg.Description != Description {
		t.Errorf("Description = %q, package.json description = %q", Description, pkg.Description)
	}
}
