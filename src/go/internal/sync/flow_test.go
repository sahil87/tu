package sync

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sahil87/tu/internal/config"
	"github.com/sahil87/tu/internal/fact"
	"github.com/sahil87/tu/internal/source"
)

// --- T005: SyncMetrics with a recording fake Runner ---

// runnerCall is one recorded Runner invocation.
type runnerCall struct {
	dir  string
	args []string
}

func (c runnerCall) String() string {
	return c.dir + " | " + strings.Join(c.args, " ")
}

// fakeRunner is a recording Runner. run scripts (stdout, err) per call; a
// nil run succeeds every call with empty stdout.
type fakeRunner struct {
	calls []runnerCall
	run   func(args []string) (string, error)
}

func (f *fakeRunner) Run(dir string, args ...string) (string, error) {
	f.calls = append(f.calls, runnerCall{dir, args})
	if f.run == nil {
		return "", nil
	}
	return f.run(args)
}

func callStrings(calls []runnerCall) []string {
	out := make([]string, len(calls))
	for i, c := range calls {
		out[i] = c.String()
	}
	return out
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// withUpstream wraps a script so the pull-target probe sees a configured
// upstream: rev-parse answers origin/main, and the round trip takes the
// upstream pull path (`pull --rebase` with no remote/refspec).
func withUpstream(run func(args []string) (string, error)) func(args []string) (string, error) {
	return func(args []string) (string, error) {
		if args[0] == "rev-parse" {
			return "origin/main\n", nil
		}
		return run(args)
	}
}

// R5: an interrupted-rebase marker appends the recovery line and runs
// `rebase --abort`, whose error is ignored.
func TestSyncMetricsRebaseRecovery(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git", "rebase-merge"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "u"), 0o755); err != nil {
		t.Fatal(err)
	}
	git := &fakeRunner{run: withUpstream(func(args []string) (string, error) {
		if args[0] == "rebase" {
			return "", errors.New("no rebase in progress")
		}
		return "", nil
	})}
	ok, lines := SyncMetrics(dir, "u", syncNow, git)
	if !ok {
		t.Error("ok = false, want true (the abort's failure is swallowed)")
	}
	if !equalStrings(lines, []string{rebaseRecoveryLine}) {
		t.Errorf("lines = %v, want [%q]", lines, rebaseRecoveryLine)
	}
	want := []string{
		dir + " | rebase --abort",
		dir + " | add u/",
		dir + " | status --porcelain u/",
		dir + " | rev-parse --abbrev-ref --symbolic-full-name @{u}",
		dir + " | pull --rebase",
		dir + " | push",
	}
	if got := callStrings(git.calls); !equalStrings(got, want) {
		t.Errorf("calls = %v, want %v", got, want)
	}
}

// R5: a missing user dir skips `add` (the TS existsSync gate — `pathspec did
// not match` cannot fail a first run) but the status still runs.
func TestSyncMetricsMissingUserDir(t *testing.T) {
	dir := t.TempDir()
	git := &fakeRunner{run: withUpstream(func(args []string) (string, error) { return "", nil })}
	ok, lines := SyncMetrics(dir, "u", syncNow, git)
	if !ok || len(lines) != 0 {
		t.Errorf("ok = %v, lines = %v, want true, no lines", ok, lines)
	}
	want := []string{
		dir + " | status --porcelain u/",
		dir + " | rev-parse --abbrev-ref --symbolic-full-name @{u}",
		dir + " | pull --rebase",
		dir + " | push",
	}
	if got := callStrings(git.calls); !equalStrings(got, want) {
		t.Errorf("calls = %v, want %v", got, want)
	}
}

// R5: a dirty status commits with the exact `# u: update {local date}`
// message.
func TestSyncMetricsDirtyStatusCommits(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "u"), 0o755); err != nil {
		t.Fatal(err)
	}
	git := &fakeRunner{run: withUpstream(func(args []string) (string, error) {
		if args[0] == "status" {
			return " M x\n", nil
		}
		return "", nil
	})}
	ok, lines := SyncMetrics(dir, "u", syncNow, git)
	if !ok || len(lines) != 0 {
		t.Errorf("ok = %v, lines = %v, want true, no lines", ok, lines)
	}
	want := []string{
		dir + " | add u/",
		dir + " | status --porcelain u/",
		dir + " | commit -m # u: update " + syncNow.Local().Format("2006-01-02"),
		dir + " | rev-parse --abbrev-ref --symbolic-full-name @{u}",
		dir + " | pull --rebase",
		dir + " | push",
	}
	if got := callStrings(git.calls); !equalStrings(got, want) {
		t.Errorf("calls = %v, want %v", got, want)
	}
}

// R3/DC-18: an add, status, or commit failure appends its
// `Warning: sync {step} failed — {err}` line before returning false and
// stops before pull/push.
func TestSyncMetricsStageCommitFailures(t *testing.T) {
	cases := []struct {
		name     string
		run      func(args []string) (string, error)
		wantLine string
	}{
		{"add", func(args []string) (string, error) {
			if args[0] == "add" {
				return "", errors.New("pathspec boom")
			}
			return "", nil
		}, "Warning: sync add failed — pathspec boom"},
		{"status", func(args []string) (string, error) {
			if args[0] == "status" {
				return "", errors.New("status boom")
			}
			return "", nil
		}, "Warning: sync status failed — status boom"},
		{"commit", func(args []string) (string, error) {
			switch args[0] {
			case "status":
				return " M x\n", nil
			case "commit":
				return "", errors.New("missing identity")
			}
			return "", nil
		}, "Warning: sync commit failed — missing identity"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, "u"), 0o755); err != nil {
				t.Fatal(err)
			}
			git := &fakeRunner{run: tc.run}
			ok, lines := SyncMetrics(dir, "u", syncNow, git)
			if ok {
				t.Error("ok = true, want false")
			}
			if !equalStrings(lines, []string{tc.wantLine}) {
				t.Errorf("lines = %v, want [%q]", lines, tc.wantLine)
			}
			for _, c := range git.calls {
				if c.args[0] == "pull" || c.args[0] == "push" {
					t.Errorf("unexpected call after %s failure: %v", tc.name, c)
				}
			}
		})
	}
}

// R1/R2: the pull target follows the repo — the configured upstream when one
// is set, else the remote's default branch from ls-remote; both no-upstream
// paths push `-u origin HEAD` (a branch with no upstream cannot plain-push);
// an empty remote additionally skips the pull; an ls-remote failure is a pull
// failure.
func TestSyncMetricsPullTarget(t *testing.T) {
	cases := []struct {
		name      string
		run       func(args []string) (string, error)
		wantTail  []string // calls after `status --porcelain u/`
		wantOK    bool
		wantLines []string
	}{
		{"upstream configured", func(args []string) (string, error) {
			if args[0] == "rev-parse" {
				return "origin/trunk\n", nil
			}
			return "", nil
		}, []string{
			"rev-parse --abbrev-ref --symbolic-full-name @{u}",
			"pull --rebase",
			"push",
		}, true, nil},
		{"no upstream, remote default branch", func(args []string) (string, error) {
			switch args[0] {
			case "rev-parse":
				return "", errors.New("fatal: no upstream configured")
			case "ls-remote":
				return "ref: refs/heads/trunk\tHEAD\ndeadbeef\tHEAD\n", nil
			}
			return "", nil
		}, []string{
			"rev-parse --abbrev-ref --symbolic-full-name @{u}",
			"ls-remote --symref origin HEAD",
			"pull --rebase origin trunk",
			"push -u origin HEAD",
		}, true, nil},
		{"empty remote skips pull, pushes -u", func(args []string) (string, error) {
			return "", nil // rev-parse and ls-remote both answer empty
		}, []string{
			"rev-parse --abbrev-ref --symbolic-full-name @{u}",
			"ls-remote --symref origin HEAD",
			"push -u origin HEAD",
		}, true, nil},
		{"ls-remote failure is a pull failure", func(args []string) (string, error) {
			if args[0] == "ls-remote" {
				return "", errors.New("network boom")
			}
			return "", nil
		}, []string{
			"rev-parse --abbrev-ref --symbolic-full-name @{u}",
			"ls-remote --symref origin HEAD",
			"rebase --abort",
		}, false, []string{"Warning: sync pull failed — network boom"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			git := &fakeRunner{run: tc.run}
			ok, lines := SyncMetrics(dir, "u", syncNow, git)
			if ok != tc.wantOK {
				t.Errorf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !equalStrings(lines, tc.wantLines) {
				t.Errorf("lines = %v, want %v", lines, tc.wantLines)
			}
			want := []string{dir + " | status --porcelain u/"}
			for _, c := range tc.wantTail {
				want = append(want, dir+" | "+c)
			}
			if got := callStrings(git.calls); !equalStrings(got, want) {
				t.Errorf("calls = %v, want %v", got, want)
			}
		})
	}
}

// R5: a pull failure appends its exact line and runs `rebase --abort`
// (error ignored); push never runs.
func TestSyncMetricsPullFailure(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "u"), 0o755); err != nil {
		t.Fatal(err)
	}
	git := &fakeRunner{run: withUpstream(func(args []string) (string, error) {
		switch args[0] {
		case "pull":
			return "", errors.New("boom")
		case "rebase":
			return "", errors.New("already clean")
		}
		return "", nil
	})}
	ok, lines := SyncMetrics(dir, "u", syncNow, git)
	if ok {
		t.Error("ok = true, want false")
	}
	wantLine := "Warning: sync pull failed — boom"
	if !equalStrings(lines, []string{wantLine}) {
		t.Errorf("lines = %v, want [%q]", lines, wantLine)
	}
	last := git.calls[len(git.calls)-1]
	if last.args[0] != "rebase" {
		t.Errorf("last call = %v, want the follow-up rebase --abort", last)
	}
	for _, c := range git.calls {
		if c.args[0] == "push" {
			t.Errorf("unexpected push after pull failure: %v", c)
		}
	}
}

// R5: a failed push is retried once; a successful retry is silent true.
func TestSyncMetricsPushRetry(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "u"), 0o755); err != nil {
		t.Fatal(err)
	}
	pushes := 0
	git := &fakeRunner{run: withUpstream(func(args []string) (string, error) {
		if args[0] == "push" {
			pushes++
			if pushes == 1 {
				return "", errors.New("boom1")
			}
		}
		return "", nil
	})}
	ok, lines := SyncMetrics(dir, "u", syncNow, git)
	if !ok || len(lines) != 0 {
		t.Errorf("ok = %v, lines = %v, want true, no lines", ok, lines)
	}
	if pushes != 2 {
		t.Errorf("pushes = %d, want 2", pushes)
	}
}

// R5: when both pushes fail the line carries the SECOND error's text.
func TestSyncMetricsPushFailsTwice(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "u"), 0o755); err != nil {
		t.Fatal(err)
	}
	pushes := 0
	git := &fakeRunner{run: withUpstream(func(args []string) (string, error) {
		if args[0] == "push" {
			pushes++
			return "", errors.New("boom" + string(rune('0'+pushes)))
		}
		return "", nil
	})}
	ok, lines := SyncMetrics(dir, "u", syncNow, git)
	if ok {
		t.Error("ok = true, want false")
	}
	wantLine := "Warning: sync push failed after retry — boom2"
	if !equalStrings(lines, []string{wantLine}) {
		t.Errorf("lines = %v, want [%q]", lines, wantLine)
	}
	if pushes != 2 {
		t.Errorf("pushes = %d, want 2", pushes)
	}
}

// --- T005: SyncMetrics against real git (the TS sync.test.ts § syncMetrics) ---

// pinGitEnv pins identity and commit.gpgsign through GIT_CONFIG_* and cuts
// the global/system config out; Exec.Run's child processes inherit the test
// process env, so t.Setenv reaches them.
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

// gitDo runs git (with -C dir when dir is non-empty) and fails the test on a
// non-zero exit.
func gitDo(t *testing.T, dir string, args ...string) string {
	t.Helper()
	argv := args
	if dir != "" {
		argv = append([]string{"-C", dir}, args...)
	}
	cmd := exec.Command("git", argv...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(argv, " "), err, out)
	}
	return string(out)
}

// realRepo seeds a bare repo on main and a clone with an initial commit, so
// the clone has the upstream origin/main SyncMetrics' pull follows. The
// fixture is anchored on main explicitly rather than depending on the
// runner's init.defaultBranch.
func realRepo(t *testing.T, root string) (bare, clone string) {
	t.Helper()
	bare = filepath.Join(root, "bare.git")
	clone = filepath.Join(root, "clone")
	gitDo(t, "", "init", "--bare", "--initial-branch=main", bare)
	gitDo(t, "", "clone", bare, clone)
	if err := os.WriteFile(filepath.Join(clone, ".gitkeep"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	gitDo(t, clone, "add", ".gitkeep")
	gitDo(t, clone, "commit", "-m", "init")
	gitDo(t, clone, "push", "-u", "origin", "main")
	return bare, clone
}

// writeDay is the TS writeMetrics call the sync tests make before syncing.
func writeDay(t *testing.T, dir, user, machine, date string, cost float64) {
	t.Helper()
	if _, err := Write(dir, user, machine, ccTool, []fact.Record{rec(date, cost)}, false); err != nil {
		t.Fatal(err)
	}
}

// TS: returns true and pushes committed files on success.
func TestSyncMetricsRealPushOnSuccess(t *testing.T) {
	pinGitEnv(t)
	bare, clone := realRepo(t, t.TempDir())
	writeDay(t, clone, "sahil", "macbook", "2026-02-22", 1.5)
	ok, lines := SyncMetrics(clone, "sahil", syncNow, Exec{})
	if !ok || len(lines) != 0 {
		t.Fatalf("ok = %v, lines = %v, want true, no lines", ok, lines)
	}
	if log := gitDo(t, bare, "log", "--oneline"); !strings.Contains(log, "# sahil: update") {
		t.Errorf("bare log = %q, want a # sahil: update commit", log)
	}
}

// TS: returns true when no local changes exist (a clean tree is a no-op).
func TestSyncMetricsRealCleanNoOp(t *testing.T) {
	pinGitEnv(t)
	bare, clone := realRepo(t, t.TempDir())
	writeDay(t, clone, "sahil", "macbook", "2026-02-21", 1.0)
	if ok, _ := SyncMetrics(clone, "sahil", syncNow, Exec{}); !ok {
		t.Fatal("first sync failed")
	}
	ok, lines := SyncMetrics(clone, "sahil", syncNow, Exec{})
	if !ok || len(lines) != 0 {
		t.Fatalf("ok = %v, lines = %v, want true, no lines", ok, lines)
	}
	log := gitDo(t, bare, "log", "--oneline")
	if n := strings.Count(log, "# sahil: update"); n != 1 {
		t.Errorf("bare log holds %d update commits, want 1:\n%s", n, log)
	}
}

// TS: returns false when metricsDir is not a git repo — the add failure
// surfaces its warning line (DC-18).
func TestSyncMetricsRealNonGitDir(t *testing.T) {
	pinGitEnv(t)
	plain := filepath.Join(t.TempDir(), "plain")
	if err := os.MkdirAll(filepath.Join(plain, "sahil"), 0o755); err != nil {
		t.Fatal(err)
	}
	ok, lines := SyncMetrics(plain, "sahil", syncNow, Exec{})
	if ok {
		t.Error("ok = true, want false")
	}
	if len(lines) != 1 || !strings.HasPrefix(lines[0], addFailedPrefix) {
		t.Errorf("lines = %v, want one %q line", lines, addFailedPrefix)
	}
}

// TS: returns true when the user dir does not exist (no data yet) — a clean
// no-op with no commit.
func TestSyncMetricsRealMissingUserDir(t *testing.T) {
	pinGitEnv(t)
	bare, clone := realRepo(t, t.TempDir())
	ok, lines := SyncMetrics(clone, "sahil", syncNow, Exec{})
	if !ok || len(lines) != 0 {
		t.Fatalf("ok = %v, lines = %v, want true, no lines", ok, lines)
	}
	if log := gitDo(t, bare, "log", "--oneline"); strings.Contains(log, "# sahil: update") {
		t.Errorf("bare log = %q, want no update commit", log)
	}
}

// TS: integrates upstream changes via pull before pushing.
func TestSyncMetricsRealUpstreamIntegrated(t *testing.T) {
	pinGitEnv(t)
	root := t.TempDir()
	bare, clone := realRepo(t, root)
	clone2 := filepath.Join(root, "clone2")
	gitDo(t, "", "clone", bare, clone2)
	bobFile := filepath.Join(clone2, "bob", "2026", "laptop", "cc-2026-02-22.jsonl")
	if err := os.MkdirAll(filepath.Dir(bobFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bobFile, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitDo(t, clone2, "add", ".")
	gitDo(t, clone2, "commit", "-m", "bob: update 2026-02-22")
	gitDo(t, clone2, "push")

	writeDay(t, clone, "sahil", "macbook", "2026-02-22", 1.0)
	ok, lines := SyncMetrics(clone, "sahil", syncNow, Exec{})
	if !ok || len(lines) != 0 {
		t.Fatalf("ok = %v, lines = %v, want true, no lines", ok, lines)
	}
	log := gitDo(t, bare, "log", "--oneline")
	if !strings.Contains(log, "# sahil: update") || !strings.Contains(log, "bob: update") {
		t.Errorf("bare log = %q, want both commits", log)
	}
	if first := strings.SplitN(strings.TrimSpace(log), "\n", 2)[0]; !strings.Contains(first, "sahil") {
		t.Errorf("newest bare commit = %q, want sahil's on top (pull --rebase)", first)
	}
	if !fileExists(filepath.Join(clone, "bob", "2026", "laptop", "cc-2026-02-22.jsonl")) {
		t.Error("bob's file missing from the clone's tree after the pull")
	}
}

// R1: a metrics clone whose default branch is not main (trunk, with the
// upstream git clone sets) syncs through `pull --rebase` — no `origin main`.
func TestSyncMetricsRealNonMainDefaultBranch(t *testing.T) {
	pinGitEnv(t)
	root := t.TempDir()
	bare := filepath.Join(root, "bare.git")
	clone := filepath.Join(root, "clone")
	gitDo(t, "", "init", "--bare", "--initial-branch=trunk", bare)
	gitDo(t, "", "clone", bare, clone)
	if err := os.WriteFile(filepath.Join(clone, ".gitkeep"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	gitDo(t, clone, "add", ".gitkeep")
	gitDo(t, clone, "commit", "-m", "init")
	gitDo(t, clone, "push", "-u", "origin", "trunk")

	writeDay(t, clone, "sahil", "macbook", "2026-02-22", 1.5)
	ok, lines := SyncMetrics(clone, "sahil", syncNow, Exec{})
	if !ok || len(lines) != 0 {
		t.Fatalf("ok = %v, lines = %v, want true, no lines", ok, lines)
	}
	if log := gitDo(t, bare, "log", "--oneline"); !strings.Contains(log, "# sahil: update") {
		t.Errorf("bare log = %q, want a # sahil: update commit", log)
	}
}

// R1: a clone whose upstream was unset (non-empty remote, default branch
// main) pulls `origin main` explicitly and pushes `-u origin HEAD` — plain
// `push` would fail with "no upstream branch" — and the push re-sets the
// upstream so the next sync takes the upstream path.
func TestSyncMetricsRealNoUpstreamFallback(t *testing.T) {
	pinGitEnv(t)
	bare, clone := realRepo(t, t.TempDir())
	gitDo(t, clone, "branch", "--unset-upstream")

	writeDay(t, clone, "sahil", "macbook", "2026-02-22", 1.5)
	ok, lines := SyncMetrics(clone, "sahil", syncNow, Exec{})
	if !ok || len(lines) != 0 {
		t.Fatalf("ok = %v, lines = %v, want true, no lines", ok, lines)
	}
	if log := gitDo(t, bare, "log", "--oneline"); !strings.Contains(log, "# sahil: update") {
		t.Errorf("bare log = %q, want a # sahil: update commit", log)
	}
	if up := strings.TrimSpace(gitDo(t, clone, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")); up != "origin/main" {
		t.Errorf("upstream = %q, want origin/main (push -u re-set it)", up)
	}
}

// R2: a fresh clone of an empty bare repo has no upstream and no remote refs;
// the first sync skips the pull and pushes `-u origin HEAD`, and the second
// sync takes the upstream path.
func TestSyncMetricsRealEmptyRemote(t *testing.T) {
	pinGitEnv(t)
	root := t.TempDir()
	bare := filepath.Join(root, "bare.git")
	clone := filepath.Join(root, "clone")
	gitDo(t, "", "init", "--bare", bare)
	gitDo(t, "", "clone", bare, clone) // git warns about the empty clone but exits 0

	writeDay(t, clone, "sahil", "macbook", "2026-02-22", 1.5)
	ok, lines := SyncMetrics(clone, "sahil", syncNow, Exec{})
	if !ok || len(lines) != 0 {
		t.Fatalf("first sync: ok = %v, lines = %v, want true, no lines", ok, lines)
	}
	if log := gitDo(t, bare, "log", "--oneline"); !strings.Contains(log, "# sahil: update") {
		t.Errorf("bare log = %q, want a # sahil: update commit", log)
	}
	if branch := strings.TrimSpace(gitDo(t, clone, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")); branch == "" {
		t.Error("push -u did not set an upstream for the next sync")
	}
	ok, lines = SyncMetrics(clone, "sahil", syncNow, Exec{})
	if !ok || len(lines) != 0 {
		t.Fatalf("second sync: ok = %v, lines = %v, want true, no lines", ok, lines)
	}
}

// R5: {dir}/.git/rebase-merge created by hand → exactly the recovery line
// and a successful sync. The fabricated state carries the minimal head-name/
// onto/orig-head files so `rebase --abort` can complete (an empty dir is not
// abortable — git 2.53 then refuses pull --rebase); the swallow of a FAILED
// abort is covered by TestSyncMetricsRebaseRecovery.
func TestSyncMetricsRealRebaseRecovery(t *testing.T) {
	pinGitEnv(t)
	_, clone := realRepo(t, t.TempDir())
	head := strings.TrimSpace(gitDo(t, clone, "rev-parse", "HEAD"))
	rm := filepath.Join(clone, ".git", "rebase-merge")
	if err := os.MkdirAll(rm, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"head-name": "refs/heads/main\n",
		"onto":      head + "\n",
		"orig-head": head + "\n",
	} {
		if err := os.WriteFile(filepath.Join(rm, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeDay(t, clone, "sahil", "macbook", "2026-02-22", 1.0)
	ok, lines := SyncMetrics(clone, "sahil", syncNow, Exec{})
	if !ok {
		t.Error("ok = false, want true")
	}
	if !equalStrings(lines, []string{rebaseRecoveryLine}) {
		t.Errorf("lines = %v, want [%q]", lines, rebaseRecoveryLine)
	}
	if fileExists(rm) {
		t.Error("rebase-merge still present after the recovery")
	}
}

// TS: handles metricsDir with spaces (literal argv — no shell word-splitting).
func TestSyncMetricsRealSpacesInPath(t *testing.T) {
	pinGitEnv(t)
	root := filepath.Join(t.TempDir(), "My Data")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	bare := filepath.Join(root, "bare with space.git")
	clone := filepath.Join(root, "clone with space")
	gitDo(t, "", "init", "--bare", "--initial-branch=main", bare)
	gitDo(t, "", "clone", bare, clone)
	if err := os.WriteFile(filepath.Join(clone, ".gitkeep"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	gitDo(t, clone, "add", ".gitkeep")
	gitDo(t, clone, "commit", "-m", "init")
	gitDo(t, clone, "push", "-u", "origin", "main")

	writeDay(t, clone, "sahil", "macbook", "2026-02-22", 1.5)
	ok, lines := SyncMetrics(clone, "sahil", syncNow, Exec{})
	if !ok || len(lines) != 0 {
		t.Fatalf("ok = %v, lines = %v, want true, no lines", ok, lines)
	}
	if log := gitDo(t, bare, "log", "--oneline"); !strings.Contains(log, "# sahil: update") {
		t.Errorf("bare log = %q, want a # sahil: update commit", log)
	}
}

// --- T006: FullSync (R6) ---

// fakeFetcher is the Fetcher seam: scripted records and errors, and a record
// of the fetch arguments.
type fakeFetcher struct {
	recs   []fact.Record
	errs   []*source.Error
	period string
	args   []string
	fresh  bool
}

func (f *fakeFetcher) FetchAll(_ context.Context, period string, extraArgs []string, fresh bool) ([]fact.Record, []*source.Error) {
	f.period, f.args, f.fresh = period, extraArgs, fresh
	return f.recs, f.errs
}

// toolRec is rec with the registry key set, for FullSync's Tool grouping.
func toolRec(tool, date string, cost float64) fact.Record {
	r := rec(date, cost)
	r.Tool = tool
	return r
}

// fullSyncInputs builds the R6 fixture inputs over the given dirs and seams.
func fullSyncInputs(metricsDir, stateDir string, fetcher Fetcher, git Runner) Inputs {
	return Inputs{
		Config:   config.Config{Mode: config.Multi, MetricsDir: metricsDir, User: "harness-user", Machine: "harness-machine"},
		StateDir: stateDir,
		Now:      syncNow,
		Source:   fetcher,
		Git:      git,
	}
}

// seedCorpus writes the two cc day-files straddling the incoming 0.5:
// 2026-01-05 at 0.25 (an update) and 2026-01-06 at 0.75 (a skip).
func seedCorpus(t *testing.T, metricsDir string) {
	t.Helper()
	if _, err := Write(metricsDir, "harness-user", "harness-machine", ccTool, []fact.Record{
		rec("2026-01-05", 0.25),
		rec("2026-01-06", 0.75),
	}, false); err != nil {
		t.Fatal(err)
	}
}

// placeholderRecs is the placeholder-shaped fetch: cc at 0.5 on
// 2026-01-05..07.
func placeholderRecs() []fact.Record {
	return []fact.Record{
		toolRec("cc", "2026-01-05", 0.5),
		toolRec("cc", "2026-01-06", 0.5),
		toolRec("cc", "2026-01-07", 0.5),
	}
}

func dayPath(metricsDir, date string) string {
	return filepath.Join(metricsDir, "harness-user", "2026", "harness-machine", "cc-"+date+".jsonl")
}

// R6: dry-run — the report shares the live decision path, touches nothing,
// and runs only the read-only status.
func TestFullSyncDryRun(t *testing.T) {
	metricsDir := t.TempDir()
	seedCorpus(t, metricsDir)
	stateDir := filepath.Join(t.TempDir(), "state")
	fetcher := &fakeFetcher{recs: placeholderRecs()}
	git := &fakeRunner{} // status answers empty: not dirty

	out, err := FullSync(context.Background(), fullSyncInputs(metricsDir, stateDir, fetcher, git), true)
	if err != nil {
		t.Fatal(err)
	}
	if !out.OK {
		t.Error("OK = false, want true (dry-run)")
	}
	if len(out.Lines) != 0 {
		t.Errorf("Lines = %v, want none in dry-run", out.Lines)
	}
	if fetcher.period != source.PeriodDaily || fetcher.args != nil || fetcher.fresh {
		t.Errorf("FetchAll got (%q, %v, %v), want (daily, nil, false)", fetcher.period, fetcher.args, fetcher.fresh)
	}

	r := out.Report
	if r == nil {
		t.Fatal("Report = nil")
	}
	if r.MetricsDir != metricsDir || r.User != "harness-user" || r.Machine != "harness-machine" {
		t.Errorf("Report fields = %+v", r)
	}
	if len(r.Tools) != len(fact.Tools) {
		t.Fatalf("len(Tools) = %d, want %d (registry order)", len(r.Tools), len(fact.Tools))
	}
	for i, tool := range fact.Tools {
		if r.Tools[i].Tool != tool {
			t.Errorf("Tools[%d].Tool = %v, want %v", i, r.Tools[i].Tool, tool)
		}
	}
	cc := r.Tools[0].Decisions
	if len(cc) != 3 {
		t.Fatalf("cc decisions = %d, want 3", len(cc))
	}
	if cc[0].Action != ActionWrite || cc[0].ExistingCost == nil || *cc[0].ExistingCost != 0.25 {
		t.Errorf("cc[0] = %+v, want write with existing 0.25", cc[0])
	}
	if cc[1].Action != ActionSkip || cc[1].ExistingCost == nil || *cc[1].ExistingCost != 0.75 {
		t.Errorf("cc[1] = %+v, want skip with existing 0.75", cc[1])
	}
	if cc[2].Action != ActionWrite || cc[2].ExistingCost != nil {
		t.Errorf("cc[2] = %+v, want write (new)", cc[2])
	}
	for _, d := range cc {
		if d.IncomingCost != 0.5 {
			t.Errorf("IncomingCost = %v, want 0.5", d.IncomingCost)
		}
	}
	if !r.WouldCommit {
		t.Error("WouldCommit = false, want true (any would-write)")
	}
	if r.CommitMessage != "# harness-user: update "+syncNow.Local().Format("2006-01-02") {
		t.Errorf("CommitMessage = %q", r.CommitMessage)
	}

	// No mutation: the seeded files are unchanged, the new file was not
	// created, .last-sync was not touched.
	if d := readDay(t, dayPath(metricsDir, "2026-01-05")); d.TotalCost != 0.25 {
		t.Errorf("cc-2026-01-05 totalCost = %v, want 0.25 (unchanged)", d.TotalCost)
	}
	if fileExists(dayPath(metricsDir, "2026-01-07")) {
		t.Error("dry-run created cc-2026-01-07.jsonl")
	}
	if fileExists(filepath.Join(stateDir, LastSyncFile)) {
		t.Error("dry-run touched .last-sync")
	}

	// The ONLY git call was the read-only status.
	want := []string{metricsDir + " | status --porcelain harness-user/"}
	if got := callStrings(git.calls); !equalStrings(got, want) {
		t.Errorf("calls = %v, want %v", got, want)
	}
}

// R6: live — writes, the add/status/pull/push round trip, and .last-sync.
// The fakeRunner answers every probe with empty success, so the round trip
// takes the empty-remote path (no pull, `push -u origin HEAD`).
func TestFullSyncLive(t *testing.T) {
	metricsDir := t.TempDir()
	seedCorpus(t, metricsDir)
	stateDir := filepath.Join(t.TempDir(), "state")
	fetcher := &fakeFetcher{recs: placeholderRecs()}
	git := &fakeRunner{} // every call succeeds; status is empty → no commit

	out, err := FullSync(context.Background(), fullSyncInputs(metricsDir, stateDir, fetcher, git), false)
	if err != nil {
		t.Fatal(err)
	}
	if !out.OK {
		t.Error("OK = false, want true")
	}
	if out.Report != nil {
		t.Error("Report set in live mode")
	}
	if len(out.Lines) != 0 {
		t.Errorf("Lines = %v, want none", out.Lines)
	}

	if d := readDay(t, dayPath(metricsDir, "2026-01-05")); d.TotalCost != 0.5 {
		t.Errorf("cc-2026-01-05 totalCost = %v, want 0.5", d.TotalCost)
	}
	if d := readDay(t, dayPath(metricsDir, "2026-01-06")); d.TotalCost != 0.75 {
		t.Errorf("cc-2026-01-06 totalCost = %v, want 0.75 (never-shrink)", d.TotalCost)
	}
	if d := readDay(t, dayPath(metricsDir, "2026-01-07")); d.TotalCost != 0.5 {
		t.Errorf("cc-2026-01-07 totalCost = %v, want 0.5", d.TotalCost)
	}

	want := []string{
		metricsDir + " | add harness-user/",
		metricsDir + " | status --porcelain harness-user/",
		metricsDir + " | rev-parse --abbrev-ref --symbolic-full-name @{u}",
		metricsDir + " | ls-remote --symref origin HEAD",
		metricsDir + " | push -u origin HEAD",
	}
	if got := callStrings(git.calls); !equalStrings(got, want) {
		t.Errorf("calls = %v, want %v", got, want)
	}

	raw, err := os.ReadFile(filepath.Join(stateDir, LastSyncFile))
	if err != nil {
		t.Fatal(".last-sync not written:", err)
	}
	if string(raw) != "2026-09-15T19:09:44.502Z\n" {
		t.Errorf(".last-sync = %q", string(raw))
	}
}

// R6: a dirty user dir makes WouldCommit true even with no would-writes; a
// git error means not dirty (never a crash) and OK stays true.
func TestFullSyncDryRunDirtyStatus(t *testing.T) {
	metricsDir := t.TempDir()
	stateDir := t.TempDir()

	dirty := &fakeRunner{run: func(args []string) (string, error) { return " M x\n", nil }}
	out, err := FullSync(context.Background(), fullSyncInputs(metricsDir, stateDir, &fakeFetcher{}, dirty), true)
	if err != nil {
		t.Fatal(err)
	}
	if !out.OK || !out.Report.WouldCommit {
		t.Errorf("OK = %v, WouldCommit = %v, want true/true (dirty tree)", out.OK, out.Report.WouldCommit)
	}

	broken := &fakeRunner{run: func(args []string) (string, error) { return "", errors.New("not a git repo") }}
	out, err = FullSync(context.Background(), fullSyncInputs(metricsDir, stateDir, &fakeFetcher{}, broken), true)
	if err != nil {
		t.Fatal(err)
	}
	if !out.OK || out.Report.WouldCommit {
		t.Errorf("OK = %v, WouldCommit = %v, want true/false (git error = not dirty)", out.OK, out.Report.WouldCommit)
	}
}

// R7: a byte-identical rewrite decides ActionUnchanged, so a steady-state
// dry-run reports no writes and WouldCommit stays false on a clean tree.
func TestFullSyncDryRunUnchanged(t *testing.T) {
	metricsDir := t.TempDir()
	seedCorpus(t, metricsDir)
	// The fetch returns exactly the seeded records — same cost AND token
	// counters, so the serialized bytes equal the files on disk.
	fetcher := &fakeFetcher{recs: []fact.Record{
		toolRec("cc", "2026-01-05", 0.25),
		toolRec("cc", "2026-01-06", 0.75),
	}}
	git := &fakeRunner{} // status answers empty: not dirty

	out, err := FullSync(context.Background(), fullSyncInputs(metricsDir, t.TempDir(), fetcher, git), true)
	if err != nil {
		t.Fatal(err)
	}
	cc := out.Report.Tools[0].Decisions
	if len(cc) != 2 || cc[0].Action != ActionUnchanged || cc[1].Action != ActionUnchanged {
		t.Errorf("decisions = %+v, want two unchanged", cc)
	}
	if out.Report.WouldCommit {
		t.Error("WouldCommit = true, want false (byte-identical rewrites are not writes)")
	}
}

// R6: the fetch's per-source errors ride the Outcome in registry order,
// unchanged, in both modes.
func TestFullSyncWarningsRegistryOrder(t *testing.T) {
	errs := []*source.Error{
		{Tool: "cc", Name: "Claude Code", Kind: source.KindExec, Detail: "boom"},
		{Tool: "gemini", Name: "Gemini", Kind: source.KindTimeout, Detail: "deadline"},
	}
	for _, dryRun := range []bool{true, false} {
		fetcher := &fakeFetcher{errs: errs}
		git := &fakeRunner{}
		in := fullSyncInputs(t.TempDir(), t.TempDir(), fetcher, git)
		out, err := FullSync(context.Background(), in, dryRun)
		if err != nil {
			t.Fatal(err)
		}
		if len(out.Warnings) != len(errs) {
			t.Fatalf("dryRun=%v: len(Warnings) = %d, want %d", dryRun, len(out.Warnings), len(errs))
		}
		for i := range errs {
			if out.Warnings[i] != errs[i] {
				t.Errorf("dryRun=%v: Warnings[%d] = %v, want %v", dryRun, i, out.Warnings[i], errs[i])
			}
		}
	}
}

// R6: a Write filesystem error is returned as error (the TS throws).
func TestFullSyncWriteError(t *testing.T) {
	metricsDir := t.TempDir()
	// A regular file where Write needs to mkdir {dir}/harness-user.
	if err := os.WriteFile(filepath.Join(metricsDir, "harness-user"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	fetcher := &fakeFetcher{recs: placeholderRecs()}
	git := &fakeRunner{}
	if _, err := FullSync(context.Background(), fullSyncInputs(metricsDir, t.TempDir(), fetcher, git), false); err == nil {
		t.Fatal("err = nil, want the Write error")
	}
	if len(git.calls) != 0 {
		t.Errorf("calls = %v, want none — the flow stops at the Write error", callStrings(git.calls))
	}
}
