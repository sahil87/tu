package harness

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSummarizePopulated(t *testing.T) {
	stdout := []byte(`{"daily":[{"date":"2026-09-08"},{"date":"2026-09-12"},{"date":"2026-09-16"}],"totals":{}}`)
	days, first, last, empty := Summarize(stdout)
	if days != 3 || first != "2026-09-08" || last != "2026-09-16" || empty {
		t.Errorf("Summarize = (%d, %q, %q, %v), want (3, 2026-09-08, 2026-09-16, false)", days, first, last, empty)
	}
}

func TestSummarizeEmptyAndUnparseable(t *testing.T) {
	for name, stdout := range map[string][]byte{
		"empty daily":  []byte(`{"daily":[],"totals":{"totalCost":-0.0}}`),
		"not json":     []byte("ccusage exploded"),
		"empty stdout": []byte(""),
	} {
		days, first, last, empty := Summarize(stdout)
		if days != 0 || first != "" || last != "" || !empty {
			t.Errorf("%s: Summarize = (%d, %q, %q, %v), want (0, \"\", \"\", true)", name, days, first, last, empty)
		}
	}
}

func TestFixturePaths(t *testing.T) {
	if got := FixturePath("claude", "daily"); got != "claude/daily.json" {
		t.Errorf("FixturePath = %q, want %q", got, "claude/daily.json")
	}
	if got := StderrPath("claude", "daily"); got != "claude/daily.stderr.txt" {
		t.Errorf("StderrPath = %q, want %q", got, "claude/daily.stderr.txt")
	}
}

func TestKeySortsArgs(t *testing.T) {
	a := Key("claude", "daily", []string{"--json", "--since", "2026-01-01"})
	b := Key("claude", "daily", []string{"--since", "2026-01-01", "--json"})
	if a != b {
		t.Errorf("Key order-sensitive: %q != %q", a, b)
	}
	if Key("claude", "daily", []string{"--json"}) == Key("claude", "weekly", []string{"--json"}) {
		t.Error("Key ignores period")
	}
}

func TestManifestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	m := &Manifest{
		Schema:         SchemaVersion,
		Machine:        "dev-ws-sahil02",
		CapturedAt:     "2026-09-16T04:55:12Z",
		CcusageVersion: "20.0.19",
		CcusagePath:    "node_modules/@ccusage/ccusage-linux-x64/bin/ccusage",
		Platform:       "linux/amd64",
		Timezone:       "Asia/Kolkata",
		Fixtures: []Fixture{{
			Source:    "claude",
			Period:    "daily",
			Args:      []string{"--json"},
			File:      "claude/daily.json",
			ExitCode:  0,
			Sha256:    "abc123",
			Days:      9,
			FirstDate: "2026-09-08",
			LastDate:  "2026-09-16",
		}},
	}
	if err := WriteManifest(dir, m); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.HasSuffix(string(raw), "\n") {
		t.Error("manifest lacks trailing newline")
	}
	if !strings.Contains(string(raw), "\n  \"schema\": 1,") {
		t.Errorf("manifest is not 2-space indented:\n%s", raw)
	}
	if strings.Contains(string(raw), "derived_from") {
		t.Error("empty DerivedFrom must be omitted")
	}
	back, err := ReadManifest(dir)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if !reflect.DeepEqual(back, m) {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", back, m)
	}
}

func TestManifestDerivedFrom(t *testing.T) {
	dir := t.TempDir()
	m := &Manifest{Schema: SchemaVersion, Machine: PlaceholderAlias, DerivedFrom: "dev-ws-sahil02/claude/daily.json"}
	if err := WriteManifest(dir, m); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}
	back, err := ReadManifest(dir)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if back.DerivedFrom != m.DerivedFrom {
		t.Errorf("DerivedFrom = %q, want %q", back.DerivedFrom, m.DerivedFrom)
	}
}
