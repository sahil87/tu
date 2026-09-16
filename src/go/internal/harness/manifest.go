// Package harness implements the fixture corpus behind the differential
// harness (plan rows P3a/P4 in fab/plans/sahil/26-09-15-go-port.md): a
// per-machine manifest describing recorded ccusage output, byte-preserving
// redaction, capture against the real binary, schema-derived placeholders,
// replay for the fake ccusage, and the call log shared by both fakes.
//
// The corpus lives under harness/fixtures/<machine-alias>/ at the repo root,
// one manifest.json per alias; the placeholder corpus uses the reserved alias
// "_placeholder". Fixture bytes are stored verbatim — never decoded and
// re-encoded — so serializer traits like "-0.0" totals and key order survive.
package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SchemaVersion is the manifest schema version written by this package.
const SchemaVersion = 1

// PlaceholderAlias is the reserved fixture alias holding schema-derived
// placeholder fixtures; it is the only alias whose entries carry
// unconfirmed: true.
const PlaceholderAlias = "_placeholder"

// Manifest describes one machine directory of recorded ccusage fixtures.
// Field order is the serialized key order.
type Manifest struct {
	Schema         int       `json:"schema"`
	Machine        string    `json:"machine"`
	CapturedAt     string    `json:"captured_at"` // RFC 3339 UTC
	CcusageVersion string    `json:"ccusage_version"`
	CcusagePath    string    `json:"ccusage_path"` // repo-relative when under the repo root
	Platform       string    `json:"platform"`     // GOOS/GOARCH
	Timezone       string    `json:"timezone"`     // IANA zone ccusage bucketed dates by
	DerivedFrom    string    `json:"derived_from,omitempty"`
	Fixtures       []Fixture `json:"fixtures"`
}

// Fixture is one recorded (source, period, args) cell of the capture matrix.
// Source is the ccusage subcommand name (claude, codex, …), never tu's key.
type Fixture struct {
	Source      string   `json:"source"`
	Period      string   `json:"period"`
	Args        []string `json:"args"`
	File        string   `json:"file"`
	StderrFile  string   `json:"stderr_file"` // empty when the capture had no stderr
	ExitCode    int      `json:"exit_code"`
	Sha256      string   `json:"sha256"` // hex digest of the committed file bytes
	Days        int      `json:"days"`
	FirstDate   string   `json:"first_date"`
	LastDate    string   `json:"last_date"`
	Empty       bool     `json:"empty"`
	Redactions  int      `json:"redactions"`
	Unconfirmed bool     `json:"unconfirmed"`
	// ConfirmedBy is present exactly when a _placeholder entry's shape has
	// been human-confirmed against a real capture (plan row P3b); it is
	// merged from <alias>/confirmed.json by WritePlaceholders and never
	// appears on a real capture's entries.
	ConfirmedBy *ConfirmedBy `json:"confirmed_by,omitempty"`
}

// ConfirmedBy records who confirmed that a placeholder's key-path set matches
// a real ccusage capture: the machine the comparison ran on, the day it ran,
// and the ccusage version that produced the real output. Field order is the
// serialized key order.
type ConfirmedBy struct {
	Machine        string `json:"machine"`
	Date           string `json:"date"` // YYYY-MM-DD
	CcusageVersion string `json:"ccusage_version"`
}

// FixturePath is the manifest-relative path of a fixture's recorded stdout.
func FixturePath(source, period string) string {
	return source + "/" + period + ".json"
}

// pathComponent rejects values that could escape their parent directory when
// joined into a fixture path: empty, "." / "..", or containing a separator.
// Capture validates machine, source, and period with it before any
// FixturePath/StderrPath result is joined under the fixture root.
func pathComponent(kind, value string) error {
	if value == "" || value == "." || value == ".." ||
		strings.ContainsRune(value, '/') || strings.ContainsRune(value, '\\') {
		return fmt.Errorf("tudiff: invalid %s %q: must be a single path component", kind, value)
	}
	return nil
}

// StderrPath is the manifest-relative path of a fixture's recorded stderr
// sidecar (written only when the captured stderr is non-empty).
func StderrPath(source, period string) string {
	return source + "/" + period + ".stderr.txt"
}

// Key is the canonical lookup key for a (source, period, args) triple; args
// are sorted so flag order on the command line does not affect matching.
func Key(source, period string, args []string) string {
	sorted := append([]string(nil), args...)
	sort.Strings(sorted)
	return source + "\x00" + period + "\x00" + strings.Join(sorted, "\x00")
}

// ReadManifest loads <dir>/manifest.json.
func ReadManifest(dir string) (*Manifest, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// WriteManifest writes <dir>/manifest.json with 2-space indentation and a
// trailing newline, creating the directory as needed.
func WriteManifest(dir string, m *Manifest) error {
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "manifest.json"), append(raw, '\n'), 0o644)
}

// Summarize derives the manifest's day fields from recorded stdout by parsing
// it as {"daily":[{"date":…}, …]}. Non-JSON input yields zero values with
// empty=true (a failed capture is itself a fixture).
func Summarize(stdout []byte) (days int, first, last string, empty bool) {
	var report struct {
		Daily []struct {
			Date string `json:"date"`
		} `json:"daily"`
	}
	if err := json.Unmarshal(stdout, &report); err != nil {
		return 0, "", "", true
	}
	days = len(report.Daily)
	if days == 0 {
		return 0, "", "", true
	}
	return days, report.Daily[0].Date, report.Daily[days-1].Date, false
}
