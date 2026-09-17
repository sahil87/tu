package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Arg parsing and exit codes, byte-exact with the mjs (R11); the algorithm
// itself is covered by internal/sync/repair_test.go.

// pinGitEnv pins identity and commit.gpgsign through GIT_CONFIG_* and cuts
// the global/system config out (the sync package's flow_test.go pattern).
func pinGitEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_COUNT", "3")
	t.Setenv("GIT_CONFIG_KEY_0", "user.email")
	t.Setenv("GIT_CONFIG_VALUE_0", "test@test.com")
	t.Setenv("GIT_CONFIG_KEY_1", "user.name")
	t.Setenv("GIT_CONFIG_VALUE_1", "Test")
	t.Setenv("GIT_CONFIG_KEY_2", "commit.gpgsign")
	t.Setenv("GIT_CONFIG_VALUE_2", "false")
}

func gitDo(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func TestUnknownArgument(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--frobnicate"}, t.TempDir(), &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	want := "repair-metrics: unknown argument: --frobnicate\n" + usageLine + "\n"
	if stderr.String() != want {
		t.Errorf("stderr = %q, want %q", stderr.String(), want)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
}

func TestRepoRequiresAPath(t *testing.T) {
	for _, args := range [][]string{{"--repo"}, {"--repo", ""}} {
		var stdout, stderr bytes.Buffer
		code := run(args, t.TempDir(), &stdout, &stderr)
		if code != 1 {
			t.Errorf("run(%q) exit = %d, want 1", args, code)
		}
		want := "repair-metrics: --repo requires a path\n" + usageLine + "\n"
		if stderr.String() != want {
			t.Errorf("run(%q) stderr = %q, want %q", args, stderr.String(), want)
		}
	}
}

func TestDefaultRepoMissing(t *testing.T) {
	home := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := run(nil, home, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	want := "repair-metrics: repo not found: " + filepath.Join(home, ".tu", "metrics_repo") + "\n"
	if stderr.String() != want {
		t.Errorf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestRelativeRepoResolvedAbsolute(t *testing.T) {
	var stderr bytes.Buffer
	var stdout bytes.Buffer
	// "somewhere" resolves against the process cwd (the mjs path.resolve).
	code := run([]string{"--repo", "definitely-not-here-turepair"}, t.TempDir(), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	want := "repair-metrics: repo not found: " + filepath.Join(cwd, "definitely-not-here-turepair") + "\n"
	if stderr.String() != want {
		t.Errorf("stderr = %q, want %q", stderr.String(), want)
	}
}

// The wiring end to end: a seeded repo, --repo, exit 0 and the scanned
// header on stdout.
func TestDryRunAgainstSeededRepo(t *testing.T) {
	pinGitEnv(t)
	repo := filepath.Join(t.TempDir(), "metrics")
	if err := os.MkdirAll(filepath.Join(repo, "u/2026/m"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitDo(t, "", "init", repo)
	day := `{"label":"2026-01-01","totalCost":10,"totalTokens":900}` + "\n"
	if err := os.WriteFile(filepath.Join(repo, "u/2026/m/cc-2026-01-01.jsonl"), []byte(day), 0o644); err != nil {
		t.Fatal(err)
	}
	gitDo(t, repo, "add", "-A")
	gitDo(t, repo, "commit", "-m", "high")
	if err := os.WriteFile(filepath.Join(repo, "u/2026/m/cc-2026-01-01.jsonl"),
		[]byte(`{"label":"2026-01-01","totalCost":1,"totalTokens":10}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitDo(t, repo, "add", "-A")
	gitDo(t, repo, "commit", "-m", "shrink")

	var stdout, stderr bytes.Buffer
	code := run([]string{"--repo", repo}, t.TempDir(), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	out := stdout.String()
	if !strings.HasPrefix(out, "repair-metrics: scanned "+repo+"\n") {
		t.Errorf("stdout lacks the scanned header:\n%s", out)
	}
	if !strings.Contains(out, "u/2026/m/cc-2026-01-01.jsonl") || !strings.Contains(out, "+$9.00") {
		t.Errorf("stdout lacks the shrunk row:\n%s", out)
	}
	if !strings.HasSuffix(out, "Dry run — nothing modified. Re-run with --write to restore shrunk files.\n") {
		t.Errorf("stdout lacks the dry-run tail:\n%s", out)
	}
}
