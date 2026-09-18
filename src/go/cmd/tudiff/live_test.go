package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sahil87/tu/internal/harness"
)

// live_test.go covers the live subcommand's seeding helpers and log/tree
// comparators against a real temp git — git is a hard requirement of the
// subcommand (as node is of run), so nothing here skips. Full-sequence
// coverage is the `just go-live` run itself.

// liveTestEnv is the pinned identity/date env the helpers under test run
// under, with the test process's PATH and a scratch HOME.
func liveTestEnv(t *testing.T) []string {
	t.Helper()
	return pinnedGitEnv(os.Getenv("PATH"), t.TempDir())
}

// writeSeedTree builds a minimal metrics-repo seed: docs/README.md plus one
// day-file, mirroring the committed harness/metrics-repo shape.
func writeSeedTree(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, dir, "docs/README.md", "# metrics seed\n")
	writeFile(t, dir, "harness-user/2026/harness-machine/cc-2026-01-05.jsonl",
		`{"label":"2026-01-05","totalCost":0.25,"inputTokens":3000,"outputTokens":400,"cacheCreationTokens":1000,"cacheReadTokens":20000,"totalTokens":24400}`+"\n")
}

// R14: the seeded bares are byte-equivalent remotes — same main hash, and a
// clone of either carries the full seed tree, docs/README.md included.
func TestSeedLiveRemotesEquivalence(t *testing.T) {
	tmp := t.TempDir()
	seedDir := filepath.Join(tmp, "seed-src")
	writeSeedTree(t, seedDir)
	env := liveTestEnv(t)

	nodeBare, goBare, err := seedLiveRemotes(tmp, seedDir, env)
	if err != nil {
		t.Fatal(err)
	}
	mainOf := func(bare string) string {
		out, err := liveGit(env, bare, "rev-parse", "main")
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(out)
	}
	if n, g := mainOf(nodeBare), mainOf(goBare); n != g {
		t.Errorf("main hashes differ: node %s, go %s", n, g)
	}

	clone := filepath.Join(tmp, "check-clone")
	if _, err := liveGit(env, tmp, "clone", goBare, clone); err != nil {
		t.Fatal(err)
	}
	if got := readLiveFile(t, filepath.Join(clone, "docs", "README.md")); got != "# metrics seed\n" {
		t.Errorf("docs/README.md = %q, want the seed bytes", got)
	}
	if got := readLiveFile(t, filepath.Join(clone, "harness-user/2026/harness-machine/cc-2026-01-05.jsonl")); !strings.Contains(got, `"totalCost":0.25`) {
		t.Errorf("day-file = %q, want the seed bytes", got)
	}
}

// R14: identical foreign commits pushed to the two bares hash equal, and a
// local commit plus pull --rebase on each side integrates them into
// byte-identical log -p outputs (hashes included).
func TestForeignCommitRebaseIntegration(t *testing.T) {
	tmp := t.TempDir()
	seedDir := filepath.Join(tmp, "seed-src")
	writeSeedTree(t, seedDir)
	env := liveTestEnv(t)
	nodeBare, goBare, err := seedLiveRemotes(tmp, seedDir, env)
	if err != nil {
		t.Fatal(err)
	}

	lr := &liveRunner{cfg: liveConfig{tmpRoot: tmp}, baseEnv: env}
	for _, sp := range []struct {
		side *liveSide
		name string
		bare string
	}{
		{&lr.node, "node", nodeBare},
		{&lr.goSide, "go", goBare},
	} {
		s, err := stageLiveSide(tmp, sp.name, sp.bare, seedDir, env)
		if err != nil {
			t.Fatal(err)
		}
		*sp.side = s
	}

	if err := lr.pushForeignCommit(); err != nil {
		t.Fatal(err)
	}
	foreignOf := func(bare string) string {
		out, err := liveGit(env, bare, "rev-parse", "main")
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(out)
	}
	if n, g := foreignOf(nodeBare), foreignOf(goBare); n != g {
		t.Fatalf("foreign commit hashes differ: node %s, go %s", n, g)
	}

	// A local change on each side, then the sync round trip's git half:
	// commit, pull --rebase (integrating the foreign commit), push.
	for _, side := range []liveSide{lr.node, lr.goSide} {
		writeFile(t, side.repo, "harness-user/2026/harness-machine/cc-2026-01-07.jsonl",
			`{"label":"2026-01-07","totalCost":0.9,"inputTokens":3000,"outputTokens":400,"cacheCreationTokens":1000,"cacheReadTokens":20000,"totalTokens":24400}`+"\n")
		if _, err := liveGit(env, side.repo, "add", "harness-user/"); err != nil {
			t.Fatal(err)
		}
		if _, err := liveGit(env, side.repo, "commit", "-m", "# harness-user: update 2026-01-09"); err != nil {
			t.Fatal(err)
		}
		if _, err := liveGit(env, side.repo, "pull", "--rebase", "origin", "main"); err != nil {
			t.Fatal(err)
		}
		if _, err := liveGit(env, side.repo, "push"); err != nil {
			t.Fatal(err)
		}
	}

	nodeLog, err := lr.sideLog(lr.node)
	if err != nil {
		t.Fatal(err)
	}
	goLog, err := lr.sideLog(lr.goSide)
	if err != nil {
		t.Fatal(err)
	}
	if nodeLog != goLog {
		t.Errorf("log -p outputs differ:\n--- node ---\n%s\n--- go ---\n%s", nodeLog, goLog)
	}
	if !strings.Contains(nodeLog, "# other-user: update 2026-01-08") ||
		!strings.Contains(nodeLog, "# harness-user: update 2026-01-09") {
		t.Errorf("log lacks the foreign or the local commit:\n%s", nodeLog)
	}
	// The local commit sits on top of the foreign one (pull --rebase order).
	if strings.Index(nodeLog, "# harness-user") > strings.Index(nodeLog, "# other-user") {
		t.Errorf("local commit is not on top after the rebase:\n%s", nodeLog)
	}
	for _, side := range []liveSide{lr.node, lr.goSide} {
		if !liveFilePresent(filepath.Join(side.repo, "other-user/2026/laptop/cc-2026-01-08.jsonl")) {
			t.Errorf("%s: the foreign file is missing after the pull", side.name)
		}
	}
}

// R14: cloneTree skips .git (the two clones' configs legitimately differ) and
// compareTrees catches an extra file and a byte difference.
func TestCloneTreeAndComparator(t *testing.T) {
	tmp := t.TempDir()
	seedDir := filepath.Join(tmp, "seed-src")
	writeSeedTree(t, seedDir)
	env := liveTestEnv(t)
	nodeBare, goBare, err := seedLiveRemotes(tmp, seedDir, env)
	if err != nil {
		t.Fatal(err)
	}
	cloneA, cloneB := filepath.Join(tmp, "a"), filepath.Join(tmp, "b")
	if _, err := liveGit(env, tmp, "clone", nodeBare, cloneA); err != nil {
		t.Fatal(err)
	}
	if _, err := liveGit(env, tmp, "clone", goBare, cloneB); err != nil {
		t.Fatal(err)
	}

	treeA, err := cloneTree(cloneA)
	if err != nil {
		t.Fatal(err)
	}
	treeB, err := cloneTree(cloneB)
	if err != nil {
		t.Fatal(err)
	}
	for p := range treeA {
		if strings.HasPrefix(p, ".git/") {
			t.Fatalf("cloneTree leaked a .git path: %s", p)
		}
	}
	res := harness.Result{Status: harness.StatusGreen}
	compareTrees(&res, "tree", treeA, treeB)
	if res.Status != harness.StatusGreen {
		t.Errorf("identical clones compared red: %+v", res)
	}

	// An extra file on one side reddens with the path.
	writeFile(t, cloneB, "extra.txt", "x\n")
	treeB, _ = cloneTree(cloneB)
	res = harness.Result{Status: harness.StatusGreen}
	compareTrees(&res, "tree", treeA, treeB)
	if res.Status != harness.StatusRed || !strings.Contains(res.GoExcerpt, "extra.txt") {
		t.Errorf("extra file: status=%s go=%q, want red naming extra.txt", res.Status, res.GoExcerpt)
	}

	// A byte difference reddens with an excerpt from the first differing byte.
	os.Remove(filepath.Join(cloneB, "extra.txt"))
	writeFile(t, cloneB, "docs/README.md", "# metrics seed — edited\n")
	treeB, _ = cloneTree(cloneB)
	res = harness.Result{Status: harness.StatusGreen}
	compareTrees(&res, "tree", treeA, treeB)
	if res.Status != harness.StatusRed || !strings.Contains(res.GoExcerpt, "edited") {
		t.Errorf("byte diff: status=%s go=%q, want red with the excerpt", res.Status, res.GoExcerpt)
	}
}

// R14: the fabricated .git/rebase-merge state is abortable — git rebase
// --abort (what the recovery path runs) succeeds and removes it.
func TestFabricateRebaseMergeAbortable(t *testing.T) {
	tmp := t.TempDir()
	seedDir := filepath.Join(tmp, "seed-src")
	writeSeedTree(t, seedDir)
	env := liveTestEnv(t)
	_, goBare, err := seedLiveRemotes(tmp, seedDir, env)
	if err != nil {
		t.Fatal(err)
	}
	clone := filepath.Join(tmp, "clone")
	if _, err := liveGit(env, tmp, "clone", goBare, clone); err != nil {
		t.Fatal(err)
	}
	if err := fabricateRebaseMerge(clone, env); err != nil {
		t.Fatal(err)
	}
	rm := filepath.Join(clone, ".git", "rebase-merge")
	if _, err := os.Stat(rm); err != nil {
		t.Fatalf("rebase-merge not planted: %v", err)
	}
	if _, err := liveGit(env, clone, "rebase", "--abort"); err != nil {
		t.Fatalf("rebase --abort failed on the fabricated state: %v", err)
	}
	if _, err := os.Stat(rm); !os.IsNotExist(err) {
		t.Error("rebase-merge still present after the abort")
	}
}

// R14: the one-off alias raises the one cc cost and leaves the committed
// corpus untouched.
func TestBuildLiveAlias(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "corpus")
	writeFile(t, src, "manifest.json", "{}\n")
	daily := `{
  "daily": [
    {"date": "2026-01-06", "totalCost": 0.5},
    {"date": "2026-01-07", "totalCost": 0.5}
  ]
}
`
	writeFile(t, src, "claude/daily.json", daily)

	alias := filepath.Join(tmp, "alias")
	if err := buildLiveAlias(src, alias, "2026-01-07", 0.9); err != nil {
		t.Fatal(err)
	}
	raised := readLiveFile(t, filepath.Join(alias, "claude", "daily.json"))
	if !strings.Contains(raised, `"totalCost": 0.9`) {
		t.Errorf("alias daily.json lacks the raised cost:\n%s", raised)
	}
	if strings.Count(raised, "0.9") != 1 {
		t.Errorf("alias daily.json changed more than the one number:\n%s", raised)
	}
	if got := readLiveFile(t, filepath.Join(src, "claude", "daily.json")); got != daily {
		t.Error("the committed corpus was modified")
	}
	if err := buildLiveAlias(src, filepath.Join(tmp, "alias2"), "2026-02-01", 0.9); err == nil {
		t.Error("a missing date must fail loudly")
	}
}

// R14: the $REPO normalization replaces every occurrence of the repo path.
func TestNormalizeRepo(t *testing.T) {
	got := normalizeRepo([]byte("scanned /tmp/a\nReview with: git -C /tmp/a diff\n"), "/tmp/a")
	want := "scanned $REPO\nReview with: git -C $REPO diff\n"
	if string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func readLiveFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// R3: live accepts --expected and its preflight rejects a missing file with
// the one-line message and exit 2 (the binary checks must pass first).
func TestLiveExpectedPreflight(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "package.json", "{}\n")
	writeFile(t, root, "harness/fixtures/_placeholder/manifest.json", "{}\n")

	nodeBundle := writeFile(t, t.TempDir(), "tu.mjs", "// stub\n")
	binDir := t.TempDir()
	goBin := filepath.Join(binDir, "tu")
	writeExe(t, goBin, "#!/bin/sh\nexit 0\n")
	turepair := filepath.Join(binDir, "turepair")
	writeExe(t, turepair, "#!/bin/sh\nexit 0\n")
	harnessBin := t.TempDir()
	writeExe(t, filepath.Join(harnessBin, "ccusage"), "#!/bin/sh\nexit 0\n")
	// preflightLive requires node on PATH; a shim suffices (it is never run).
	shimDir := t.TempDir()
	writeExe(t, filepath.Join(shimDir, "node"), "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Chdir(root)

	invoke := func(extra ...string) (int, string) {
		args := []string{"live",
			"--node", nodeBundle,
			"--go", goBin,
			"--turepair", turepair,
			"--harness-bin", harnessBin,
			"--report", filepath.Join(t.TempDir(), "report"),
		}
		args = append(args, extra...)
		var stdout, stderr bytes.Buffer
		code := run(args, &stdout, &stderr)
		return code, stderr.String()
	}

	t.Run("missing file", func(t *testing.T) {
		code, stderr := invoke("--expected", "/nonexistent")
		if code != 2 || stderr != "tudiff: /nonexistent not found\n" {
			t.Errorf("code = %d, stderr = %q", code, stderr)
		}
	})
	t.Run("invalid file", func(t *testing.T) {
		bad := writeFile(t, t.TempDir(), "bad.json", `{"schema":2,"expected":[]}`+"\n")
		code, stderr := invoke("--expected", bad)
		if code != 2 || stderr != "tudiff: expected-diffs: schema must be 1, got 2\n" {
			t.Errorf("code = %d, stderr = %q", code, stderr)
		}
	})
	t.Run("default missing under the fake root", func(t *testing.T) {
		code, stderr := invoke()
		if code != 2 || stderr != "tudiff: harness/expected-diffs.json not found\n" {
			t.Errorf("code = %d, stderr = %q", code, stderr)
		}
	})
}
