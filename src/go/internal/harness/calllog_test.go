package harness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readLogLines(t *testing.T, path string) []callLogLine {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var lines []callLogLine
	for _, l := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
		var entry callLogLine
		if err := json.Unmarshal([]byte(l), &entry); err != nil {
			t.Fatalf("log line %q does not decode: %v", l, err)
		}
		lines = append(lines, entry)
	}
	return lines
}

func TestLogCallAppendsToExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "calls.jsonl")
	if err := os.WriteFile(path, []byte(`{"tool":"git","argv":["push"],"cwd":"/x"}`+"\n"+`{"tool":"git","argv":["pull"],"cwd":"/x"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(CallLogEnv, path)

	LogCall("ccusage", []string{"claude", "daily", "--json"}, "dev-ws-sahil02/claude/daily.json")

	lines := readLogLines(t, path)
	if len(lines) != 3 {
		t.Fatalf("log has %d lines, want 3", len(lines))
	}
	last := lines[2]
	if last.Tool != "ccusage" {
		t.Errorf("tool = %q, want ccusage", last.Tool)
	}
	if len(last.Argv) != 3 || last.Argv[0] != "claude" || last.Argv[1] != "daily" || last.Argv[2] != "--json" {
		t.Errorf("argv = %v, want [claude daily --json]", last.Argv)
	}
	if last.Matched != "dev-ws-sahil02/claude/daily.json" {
		t.Errorf("matched = %q", last.Matched)
	}
	if last.Cwd == "" {
		t.Error("cwd is empty")
	}
}

func TestLogCallMatchedOmittedWhenEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "calls.jsonl")
	t.Setenv(CallLogEnv, path)

	LogCall("git", []string{"status", "--porcelain", "u/"}, "")

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.Contains(string(raw), "matched") {
		t.Errorf("empty matched must be omitted, got %s", raw)
	}
	lines := readLogLines(t, path)
	if len(lines) != 1 || lines[0].Tool != "git" {
		t.Errorf("lines = %+v", lines)
	}
}

func TestLogCallUnsetEnvWritesNothing(t *testing.T) {
	dir := t.TempDir()
	os.Unsetenv(CallLogEnv)

	LogCall("git", []string{"push"}, "")

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("wrote into %s despite unset %s", dir, CallLogEnv)
	}
}

func TestLogCallUnwritablePathIsSilent(t *testing.T) {
	// A path inside a nonexistent directory cannot be created; LogCall must
	// swallow the error without panicking.
	t.Setenv(CallLogEnv, filepath.Join(t.TempDir(), "no", "such", "dir", "calls.jsonl"))
	LogCall("git", []string{"push"}, "")
}
