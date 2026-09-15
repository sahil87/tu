package harness

import (
	"os"
	"path/filepath"
	"testing"
)

// writeAlias builds a minimal fixture alias dir with one claude/daily fixture.
func writeAlias(t *testing.T, root, alias string, payload []byte, exitCode int) string {
	t.Helper()
	dir := filepath.Join(root, alias)
	if err := os.MkdirAll(filepath.Join(dir, "claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "claude", "daily.json"), payload, 0o644); err != nil {
		t.Fatal(err)
	}
	m := &Manifest{
		Schema:         SchemaVersion,
		Machine:        alias,
		CcusageVersion: "20.0.19",
		Fixtures: []Fixture{{
			Source: "claude", Period: "daily", Args: []string{"--json"},
			File: "claude/daily.json", ExitCode: exitCode, Days: 1,
			FirstDate: "2026-09-08", LastDate: "2026-09-16",
			Unconfirmed: alias == PlaceholderAlias,
		}},
	}
	if err := WriteManifest(dir, m); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCorpusLookupFirstHitWins(t *testing.T) {
	root := t.TempDir()
	real := writeAlias(t, root, "dev-ws-sahil02", []byte(`{"daily":[{"date":"2026-09-08"}]}`+"\n"), 0)
	placeholder := writeAlias(t, root, PlaceholderAlias, []byte(`{"daily":[{"date":"2026-01-05"}]}`+"\n"), 0)

	c, err := LoadCorpus([]string{real, placeholder})
	if err != nil {
		t.Fatalf("LoadCorpus: %v", err)
	}
	hit, ok := c.Lookup("claude", "daily", []string{"--json"})
	if !ok {
		t.Fatal("Lookup missed")
	}
	if hit.Matched != "dev-ws-sahil02/claude/daily.json" {
		t.Errorf("matched = %q, want the first alias", hit.Matched)
	}
	if string(hit.Stdout) != `{"daily":[{"date":"2026-09-08"}]}`+"\n" {
		t.Errorf("stdout = %q", hit.Stdout)
	}

	// Reverse the order and the placeholder must win instead.
	c2, err := LoadCorpus([]string{placeholder, real})
	if err != nil {
		t.Fatalf("LoadCorpus: %v", err)
	}
	hit2, ok := c2.Lookup("claude", "daily", []string{"--json"})
	if !ok || hit2.Matched != PlaceholderAlias+"/claude/daily.json" {
		t.Errorf("reversed order: matched = %q ok=%v", hit2.Matched, ok)
	}
}

func TestCorpusLookupFlagOrderInsensitive(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "m")
	if err := os.MkdirAll(filepath.Join(dir, "claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "claude", "daily.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := &Manifest{
		Schema:  SchemaVersion,
		Machine: "m",
		Fixtures: []Fixture{{
			Source: "claude", Period: "daily", Args: []string{"--json", "--since", "2026-01-01"},
			File: "claude/daily.json",
		}},
	}
	if err := WriteManifest(dir, m); err != nil {
		t.Fatal(err)
	}
	c, err := LoadCorpus([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Lookup("claude", "daily", []string{"--since", "2026-01-01", "--json"}); !ok {
		t.Error("flag order must not affect matching")
	}
	if _, ok := c.Lookup("claude", "daily", []string{"--json"}); ok {
		t.Error("subset of args must not match")
	}
}

func TestCorpusLookupMissAndVersion(t *testing.T) {
	root := t.TempDir()
	dir := writeAlias(t, root, "dev-ws-sahil02", []byte("{}"), 0)
	c, err := LoadCorpus([]string{dir, filepath.Join(root, "no-such-alias")})
	if err != nil {
		t.Fatalf("LoadCorpus must skip dirs without manifests: %v", err)
	}
	if _, ok := c.Lookup("kimi", "weekly", []string{"--json"}); ok {
		t.Error("unknown cell must miss")
	}
	if got := c.Version(); got != "20.0.19" {
		t.Errorf("Version = %q, want 20.0.19", got)
	}

	empty, err := LoadCorpus(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := empty.Version(); got != "" {
		t.Errorf("empty corpus Version = %q, want empty", got)
	}
}

// A manifest entry naming a stderr sidecar whose file is missing is a
// manifest/fixture mismatch: Lookup must continue to the next alias instead
// of replaying a hit with silently empty stderr.
func TestCorpusLookupMissingStderrSidecar(t *testing.T) {
	root := t.TempDir()
	broken := writeAlias(t, root, "broken", []byte(`{"daily":[]}`+"\n"), 3)
	good := writeAlias(t, root, "good", []byte(`{"daily":[{"date":"2026-09-08"}]}`+"\n"), 0)

	// Point the broken alias's entry at a sidecar that does not exist.
	m, err := ReadManifest(broken)
	if err != nil {
		t.Fatal(err)
	}
	m.Fixtures[0].StderrFile = "claude/daily.stderr.txt"
	if err := WriteManifest(broken, m); err != nil {
		t.Fatal(err)
	}

	c, err := LoadCorpus([]string{broken, good})
	if err != nil {
		t.Fatal(err)
	}
	hit, ok := c.Lookup("claude", "daily", []string{"--json"})
	if !ok || hit.Matched != "good/claude/daily.json" {
		t.Errorf("matched = %q ok=%v, want the good alias to win", hit.Matched, ok)
	}

	// With no fallback alias the corrupt entry misses loudly.
	c2, err := LoadCorpus([]string{broken})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c2.Lookup("claude", "daily", []string{"--json"}); ok {
		t.Error("missing sidecar must not replay as a hit")
	}
}
