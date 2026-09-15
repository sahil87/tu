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

func envWith(pairs ...string) func(string) string {
	m := map[string]string{}
	for i := 0; i+1 < len(pairs); i += 2 {
		m[pairs[i]] = pairs[i+1]
	}
	return func(k string) string { return m[k] }
}

// R12 scenario 1: a scripted prefix match answers after -C stripping.
func TestScriptedStatus(t *testing.T) {
	script := `[{"match":["status","--porcelain"],"stdout":" M u/x\n","exit":0}]`
	logPath := filepath.Join(t.TempDir(), "calls.jsonl")
	t.Setenv(harness.CallLogEnv, logPath)

	var stdout, stderr bytes.Buffer
	code := run([]string{"-C", "/tmp/m", "status", "--porcelain", "u/"}, envWith(gitScriptEnv, script), &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	if stdout.String() != " M u/x\n" {
		t.Errorf("stdout = %q, want %q", stdout.String(), " M u/x\n")
	}

	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	var entry struct {
		Tool string   `json:"tool"`
		Argv []string `json:"argv"`
	}
	if err := json.Unmarshal([]byte(strings.TrimRight(string(raw), "\n")), &entry); err != nil {
		t.Fatal(err)
	}
	want := []string{"-C", "/tmp/m", "status", "--porcelain", "u/"}
	if entry.Tool != "git" || strings.Join(entry.Argv, " ") != strings.Join(want, " ") {
		t.Errorf("log line = %+v, want tool git argv %v", entry, want)
	}
}

// R12 scenario 2: no script → silent exit 0.
func TestNoScriptSilentSuccess(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-C", "/tmp/m", "pull", "--rebase", "origin", "main"}, envWith(), &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Errorf("stdout/stderr = %q/%q, want both empty", stdout.String(), stderr.String())
	}
}

// Every argv shape tu issues (sync.ts + cli.ts) must be accepted silently.
func TestTuArgvShapesAccepted(t *testing.T) {
	shapes := [][]string{
		{"-C", "/m", "rebase", "--abort"},
		{"-C", "/m", "add", "sahil/"},
		{"-C", "/m", "status", "--porcelain", "sahil/"},
		{"-C", "/m", "commit", "-m", "# sahil: update 2026-09-16"},
		{"-C", "/m", "pull", "--rebase", "origin", "main"},
		{"-C", "/m", "push"},
		{"-C", "/m", "rev-parse", "--git-dir"},
		{"clone", "git@example.com:org/repo.git", "/tmp/dir"},
	}
	for _, args := range shapes {
		var stdout, stderr bytes.Buffer
		if code := run(args, envWith(), &stdout, &stderr); code != 0 {
			t.Errorf("run(%q) exit = %d, want 0", args, code)
		}
		if stdout.Len() != 0 || stderr.Len() != 0 {
			t.Errorf("run(%q) produced output without a script", args)
		}
	}
}

func TestScriptFirstMatchWinsAndExitCode(t *testing.T) {
	script := `[
	  {"match":["push"],"stdout":"first\n","exit":7},
	  {"match":["push"],"stdout":"second\n","exit":0}
	]`
	var stdout, stderr bytes.Buffer
	code := run([]string{"-C", "/m", "push"}, envWith(gitScriptEnv, script), &stdout, &stderr)
	if code != 7 {
		t.Errorf("exit = %d, want scripted 7", code)
	}
	if stdout.String() != "first\n" {
		t.Errorf("stdout = %q, want the first matching rule", stdout.String())
	}
}

func TestScriptNoMatchFallsThrough(t *testing.T) {
	script := `[{"match":["status"],"stdout":"dirty\n","exit":0}]`
	var stdout, stderr bytes.Buffer
	code := run([]string{"-C", "/m", "fetch"}, envWith(gitScriptEnv, script), &stdout, &stderr)
	if code != 0 || stdout.Len() != 0 {
		t.Errorf("unmatched call: exit = %d, stdout = %q", code, stdout.String())
	}
}

func TestMalformedScriptIsSilentSuccess(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"push"}, envWith(gitScriptEnv, "{not json"), &stdout, &stderr)
	if code != 0 || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Errorf("malformed script: exit = %d", code)
	}
}

func TestStripDashC(t *testing.T) {
	if got := stripDashC([]string{"-C", "/d", "push"}); strings.Join(got, " ") != "push" {
		t.Errorf("stripDashC = %v", got)
	}
	if got := stripDashC([]string{"push"}); strings.Join(got, " ") != "push" {
		t.Errorf("stripDashC without -C = %v", got)
	}
	if got := stripDashC([]string{"-C"}); len(got) != 1 {
		t.Errorf("lone -C must pass through: %v", got)
	}
}
