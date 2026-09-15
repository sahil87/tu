package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sahil87/tu/internal/harness"
)

// buildAlias writes one alias dir with a claude/daily fixture whose stdout is
// payload, plus optional recorded stderr and exit code.
func buildAlias(t *testing.T, root, alias string, payload, stderrPayload []byte, exitCode int) string {
	t.Helper()
	dir := filepath.Join(root, alias)
	if err := os.MkdirAll(filepath.Join(dir, "claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "claude", "daily.json"), payload, 0o644); err != nil {
		t.Fatal(err)
	}
	stderrFile := ""
	if len(stderrPayload) > 0 {
		stderrFile = "claude/daily.stderr.txt"
		if err := os.WriteFile(filepath.Join(dir, "claude", "daily.stderr.txt"), stderrPayload, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m := &harness.Manifest{
		Schema:         harness.SchemaVersion,
		Machine:        alias,
		CcusageVersion: "20.0.19",
		Fixtures: []harness.Fixture{{
			Source: "claude", Period: "daily", Args: []string{"--json"},
			File: "claude/daily.json", StderrFile: stderrFile, ExitCode: exitCode,
		}},
	}
	if err := harness.WriteManifest(dir, m); err != nil {
		t.Fatal(err)
	}
	return dir
}

func envWith(pairs ...string) func(string) string {
	m := map[string]string{}
	for i := 0; i+1 < len(pairs); i += 2 {
		m[pairs[i]] = pairs[i+1]
	}
	return func(k string) string { return m[k] }
}

func TestUnsetFixturesEnv(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"claude", "daily", "--json"}, envWith(), &stdout, &stderr)
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if got := stderr.String(); got != "fakeccusage: TUDIFF_FIXTURES not set\n" {
		t.Errorf("stderr = %q", got)
	}
}

func TestHitReplaysVerbatim(t *testing.T) {
	root := t.TempDir()
	payload := []byte(`{"daily":[],"totals":{"totalCost":-0.0}}` + "\n")
	dir := buildAlias(t, root, "dev-ws-sahil02", payload, []byte("a warning\n"), 0)

	var stdout, stderr bytes.Buffer
	code := run([]string{"claude", "daily", "--json"}, envWith(fixturesEnv, dir), &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	if !bytes.Equal(stdout.Bytes(), payload) {
		t.Errorf("stdout = %q, want verbatim fixture bytes", stdout.Bytes())
	}
	if stderr.String() != "a warning\n" {
		t.Errorf("stderr = %q, want recorded stderr", stderr.String())
	}
}

func TestHitRecordedExitCode(t *testing.T) {
	root := t.TempDir()
	dir := buildAlias(t, root, "broken", []byte(""), nil, 3)
	var stdout, stderr bytes.Buffer
	code := run([]string{"claude", "daily", "--json"}, envWith(fixturesEnv, dir), &stdout, &stderr)
	if code != 3 {
		t.Errorf("exit = %d, want recorded 3", code)
	}
}

func TestMissIsLoud(t *testing.T) {
	root := t.TempDir()
	dir := buildAlias(t, root, "dev-ws-sahil02", []byte("{}"), nil, 0)
	var stdout, stderr bytes.Buffer
	code := run([]string{"kimi", "weekly", "--json"}, envWith(fixturesEnv, dir), &stdout, &stderr)
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.HasPrefix(stderr.String(), "fakeccusage: no fixture for argv") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestVersionFromFirstManifest(t *testing.T) {
	root := t.TempDir()
	dir := buildAlias(t, root, "dev-ws-sahil02", []byte("{}"), nil, 0)
	for _, flag := range []string{"--version", "-v"} {
		var stdout, stderr bytes.Buffer
		code := run([]string{flag}, envWith(fixturesEnv, dir), &stdout, &stderr)
		if code != 0 {
			t.Errorf("%s: exit = %d, want 0", flag, code)
		}
		if got := stdout.String(); got != "ccusage 20.0.19\n" {
			t.Errorf("%s: stdout = %q", flag, got)
		}
	}
}

// --version against a corpus whose dirs all lack manifests exits 2 rather
// than printing an empty version.
func TestVersionWithoutManifest(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--version"}, envWith(fixturesEnv, t.TempDir()), &stdout, &stderr)
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if got := stderr.String(); got != "fakeccusage: no manifest found in TUDIFF_FIXTURES\n" {
		t.Errorf("stderr = %q", got)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
}

// R11: every invocation appends one JSON line with matched set on a hit.
func TestCallLogOnHit(t *testing.T) {
	root := t.TempDir()
	payload := []byte("{}\n")
	dir := buildAlias(t, root, "dev-ws-sahil02", payload, nil, 0)
	logPath := filepath.Join(root, "calls.jsonl")
	if err := os.WriteFile(logPath, []byte(`{"tool":"git","argv":["push"],"cwd":"/x"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(harness.CallLogEnv, logPath)

	var stdout, stderr bytes.Buffer
	code := run([]string{"claude", "daily", "--json"}, envWith(fixturesEnv, dir), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("log has %d lines, want 2", len(lines))
	}
	var entry struct {
		Tool    string   `json:"tool"`
		Argv    []string `json:"argv"`
		Matched string   `json:"matched"`
	}
	if err := json.Unmarshal([]byte(lines[1]), &entry); err != nil {
		t.Fatal(err)
	}
	if entry.Tool != "ccusage" || len(entry.Argv) != 3 || entry.Matched != "dev-ws-sahil02/claude/daily.json" {
		t.Errorf("log line = %+v", entry)
	}
}

// R11: a miss is logged too, without a matched field.
func TestCallLogOnMiss(t *testing.T) {
	root := t.TempDir()
	dir := buildAlias(t, root, "dev-ws-sahil02", []byte("{}"), nil, 0)
	logPath := filepath.Join(root, "calls.jsonl")
	t.Setenv(harness.CallLogEnv, logPath)

	var stdout, stderr bytes.Buffer
	if code := run([]string{"kimi", "weekly", "--json"}, envWith(fixturesEnv, dir), &stdout, &stderr); code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "matched") {
		t.Errorf("miss must not carry matched: %s", raw)
	}
	if !strings.Contains(string(raw), `"tool":"ccusage"`) {
		t.Errorf("miss was not logged: %s", raw)
	}
}

// R11: --version and the pre-corpus error exits are invocations too — each
// must append a log line.
func TestCallLogOnVersionAndEarlyExits(t *testing.T) {
	root := t.TempDir()
	dir := buildAlias(t, root, "dev-ws-sahil02", []byte("{}"), nil, 0)
	logPath := filepath.Join(root, "calls.jsonl")
	t.Setenv(harness.CallLogEnv, logPath)

	var stdout, stderr bytes.Buffer
	if code := run([]string{"--version"}, envWith(fixturesEnv, dir), &stdout, &stderr); code != 0 {
		t.Fatalf("--version: exit = %d, want 0", code)
	}
	if code := run([]string{"claude", "daily", "--json"}, envWith(), &stdout, &stderr); code != 2 {
		t.Fatalf("no-env: exit = %d, want 2", code)
	}
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("log has %d lines, want 2 (version + no-env): %s", len(lines), raw)
	}
	if !strings.Contains(lines[0], `"argv":["--version"]`) {
		t.Errorf("version invocation not logged: %s", lines[0])
	}
}
