package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sahil87/tu/internal/harness"
)

// live_test.go covers the live subcommand's seeding helpers, the golden-mode
// preflight, and a full update→compare round trip against shell stand-ins —
// git is a hard requirement of the subcommand, so nothing here skips. The
// real-binary gate is the `just go-live` run itself.

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

// The seeded bare is byte-reproducible: two independent seeds from the same
// tree share the main hash, and a clone carries the full seed tree,
// docs/README.md included.
func TestSeedLiveRemoteReproducible(t *testing.T) {
	seed := func(t *testing.T) (tmp, bare string) {
		tmp = t.TempDir()
		seedDir := filepath.Join(tmp, "seed-src")
		writeSeedTree(t, seedDir)
		bare, err := seedLiveRemote(tmp, seedDir, liveTestEnv(t))
		if err != nil {
			t.Fatal(err)
		}
		return tmp, bare
	}
	tmpA, bareA := seed(t)
	_, bareB := seed(t)
	env := liveTestEnv(t)
	mainOf := func(bare string) string {
		out, err := liveGit(env, bare, "rev-parse", "main")
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(out)
	}
	if a, b := mainOf(bareA), mainOf(bareB); a != b {
		t.Errorf("main hashes differ across identical seeds: %s vs %s", a, b)
	}

	clone := filepath.Join(tmpA, "check-clone")
	if _, err := liveGit(env, tmpA, "clone", bareA, clone); err != nil {
		t.Fatal(err)
	}
	if got := readLiveFile(t, filepath.Join(clone, "docs", "README.md")); got != "# metrics seed\n" {
		t.Errorf("docs/README.md = %q, want the seed bytes", got)
	}
	if got := readLiveFile(t, filepath.Join(clone, "harness-user/2026/harness-machine/cc-2026-01-05.jsonl")); !strings.Contains(got, `"totalCost":0.25`) {
		t.Errorf("day-file = %q, want the seed bytes", got)
	}
}

// The pinned identity and fixed commit dates make the foreign-commit flow
// byte-reproducible across independent runs (log -p, hashes included) — the
// property the committed log.txt goldens rest on.
func TestLiveFlowDeterminism(t *testing.T) {
	runOnce := func(t *testing.T) string {
		t.Helper()
		tmp := t.TempDir()
		seedDir := filepath.Join(tmp, "seed-src")
		writeSeedTree(t, seedDir)
		env := liveTestEnv(t)
		bare, err := seedLiveRemote(tmp, seedDir, env)
		if err != nil {
			t.Fatal(err)
		}
		lr := &liveRunner{cfg: liveConfig{tmpRoot: tmp}, baseEnv: env}
		side, err := stageLiveSide(tmp, "go", bare, seedDir, env)
		if err != nil {
			t.Fatal(err)
		}
		lr.side = side

		if err := lr.pushForeignCommit(); err != nil {
			t.Fatal(err)
		}
		// A local change, then the sync round trip's git half: commit,
		// pull --rebase (integrating the foreign commit), push.
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

		log, err := lr.sideLog()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(log, "# other-user: update 2026-01-08") ||
			!strings.Contains(log, "# harness-user: update 2026-01-09") {
			t.Errorf("log lacks the foreign or the local commit:\n%s", log)
		}
		// The local commit sits on top of the foreign one (pull --rebase order).
		if strings.Index(log, "# harness-user") > strings.Index(log, "# other-user") {
			t.Errorf("local commit is not on top after the rebase:\n%s", log)
		}
		if !fileExists(filepath.Join(side.repo, "other-user/2026/laptop/cc-2026-01-08.jsonl")) {
			t.Error("the foreign file is missing after the pull")
		}
		return log
	}
	if first, second := runOnce(t), runOnce(t); first != second {
		t.Errorf("log -p outputs differ across identical runs:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

// R14: the fabricated .git/rebase-merge state is abortable — git rebase
// --abort (what the recovery path runs) succeeds and removes it.
func TestFabricateRebaseMergeAbortable(t *testing.T) {
	tmp := t.TempDir()
	seedDir := filepath.Join(tmp, "seed-src")
	writeSeedTree(t, seedDir)
	env := liveTestEnv(t)
	bare, err := seedLiveRemote(tmp, seedDir, env)
	if err != nil {
		t.Fatal(err)
	}
	clone := filepath.Join(tmp, "clone")
	if _, err := liveGit(env, tmp, "clone", bare, clone); err != nil {
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

// liveTuStandIn is the --go shell script for the live smoke tests: a
// --version line for ProbeVersion, the recovery needle when the fabricated
// rebase-merge state is present, exit 1 with the pull-failure bytes when
// origin points at the missing bare, and the pinned clock on stdout otherwise
// (which proves TUDIFF_NOW reaches the child).
const liveTuStandIn = `#!/bin/sh
repo="$HOME/.tu/metrics_repo"
if [ "${1:-}" = "--version" ]; then printf 'tu version v0.0.0-live\n'; exit 0; fi
if [ -d "$repo/.git/rebase-merge" ]; then
  printf 'Warning: recovering from interrupted rebase\n' >&2
  git -C "$repo" rebase --abort >/dev/null 2>&1 || true
fi
url=$(git -C "$repo" remote get-url origin 2>/dev/null || echo none)
case "$url" in
  *missing.git)
    printf 'Error: sync failed — check network and remote config.\n' >&2
    exit 1 ;;
esac
printf '%s\n' "${TUDIFF_NOW:-unset}"
exit 0
`

// liveSmokeEnv is a self-contained fake checkout for live golden-mode tests:
// a repo root (justfile, tu.default.conf, a minimal _placeholder corpus, the
// matrix and expected-diffs files, the seed tree), shell stand-ins for
// --go and --turepair, the fake ccusage, and a golden dir — all under temp
// space. The real harness/golden is never touched.
type liveSmokeEnv struct {
	root       string
	goBin      string
	turepair   string
	harnessBin string
	golden     string
	report     string
}

func newLiveSmokeEnv(t *testing.T) *liveSmokeEnv {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "justfile", "go-build:\n\ttrue\n")
	writeFile(t, root, "tu.default.conf", "version = 2\n")
	writeFile(t, root, "harness/fixtures/_placeholder/manifest.json",
		`{"schema":1,"machine":"_placeholder","captured_at":"2026-09-16T00:00:00Z","ccusage_version":"20.0.19","ccusage_path":"","platform":"derived","timezone":"","fixtures":[]}`+"\n")
	writeFile(t, root, "harness/fixtures/_placeholder/claude/daily.json",
		"{\n  \"daily\": [\n    {\"date\": \"2026-01-07\", \"totalCost\": 0.5}\n  ]\n}\n")
	writeFile(t, root, "harness/matrix.json", "{\"schema\":1,\"cases\":[]}\n")
	writeFile(t, root, "harness/expected-diffs.json", "{\n  \"schema\": 1,\n  \"expected\": []\n}\n")
	writeSeedTree(t, filepath.Join(root, "harness", "metrics-repo"))

	e := &liveSmokeEnv{root: root}
	e.goBin = filepath.Join(t.TempDir(), "tu")
	writeExe(t, e.goBin, liveTuStandIn)
	e.turepair = filepath.Join(t.TempDir(), "turepair")
	writeExe(t, e.turepair, "#!/bin/sh\nprintf 'repair: scanned\\n'\n")
	e.harnessBin = t.TempDir()
	writeExe(t, filepath.Join(e.harnessBin, "ccusage"), "#!/bin/sh\nexit 0\n")
	e.golden = filepath.Join(t.TempDir(), "golden")
	e.report = filepath.Join(t.TempDir(), "report")
	return e
}

// invoke runs `live` through the top-level dispatcher with every path flag
// pinned at this environment, from the fake root as cwd.
func (e *liveSmokeEnv) invoke(t *testing.T, extra ...string) (int, string, string) {
	t.Helper()
	t.Chdir(e.root)
	args := []string{"live",
		"--go", e.goBin,
		"--turepair", e.turepair,
		"--harness-bin", e.harnessBin,
		"--golden", e.golden,
		"--report", e.report,
	}
	args = append(args, extra...)
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// writeLiveManifest writes a corpus manifest whose matrix hash matches the
// fake root's harness/matrix.json.
func (e *liveSmokeEnv) writeLiveManifest(t *testing.T) {
	t.Helper()
	sum, err := harness.MatrixSHA256(filepath.Join(e.root, "harness", "matrix.json"))
	if err != nil {
		t.Fatal(err)
	}
	m := &harness.GoldenManifest{
		Schema:        harness.SchemaVersion,
		Oracle:        "bin/tu",
		OracleVersion: "v0.0.0-live",
		CapturedAt:    "2026-09-25T06:30:00Z",
		Now:           smokeNow,
		Script:        "util-linux",
		Platform:      "linux/amd64",
		Fixtures:      []string{harness.PlaceholderAlias},
		MatrixSHA256:  sum,
		LiveSteps:     len(liveSteps),
	}
	if err := harness.WriteGoldenManifest(e.golden, m); err != nil {
		t.Fatal(err)
	}
}

// R4: the golden-mode preflight failures are their exact one-line messages,
// exit 2.
func TestLiveGoldenPreflight(t *testing.T) {
	t.Run("manifest missing", func(t *testing.T) {
		e := newLiveSmokeEnv(t)
		code, _, stderr := e.invoke(t)
		if code != 2 || stderr != "tudiff: "+e.golden+"/manifest.json not found (run tudiff live --update)\n" {
			t.Errorf("code = %d, stderr = %q", code, stderr)
		}
	})
	t.Run("matrix changed since capture", func(t *testing.T) {
		e := newLiveSmokeEnv(t)
		e.writeLiveManifest(t)
		writeFile(t, e.root, "harness/matrix.json", "{\"schema\":1,\"cases\":[],\"x\":1}\n")
		code, _, stderr := e.invoke(t)
		if code != 2 || stderr != "tudiff: harness/matrix.json changed since the goldens were captured (run tudiff run --update and review the diff)\n" {
			t.Errorf("code = %d, stderr = %q", code, stderr)
		}
	})
	t.Run("step without a golden", func(t *testing.T) {
		e := newLiveSmokeEnv(t)
		e.writeLiveManifest(t)
		for _, step := range liveSteps {
			if step == "sync" {
				continue
			}
			if err := os.MkdirAll(harness.GoldenLiveDir(e.golden, step), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		code, _, stderr := e.invoke(t)
		if code != 2 || stderr != "tudiff: no golden for live/sync (run tudiff live --update)\n" {
			t.Errorf("code = %d, stderr = %q", code, stderr)
		}
	})
}

// R4: --update captures the nine step goldens from the Go side and a compare
// run against them is green — the live golden round trip, TUDIFF_NOW, the
// extras, and the repair goldens included.
func TestLiveUpdateCompareRoundTrip(t *testing.T) {
	e := newLiveSmokeEnv(t)
	code, stdout, stderr := e.invoke(t, "--update", "--now", smokeNow)
	if code != 0 {
		t.Fatalf("--update exit = %d, stderr = %q", code, stderr)
	}
	if stdout != "tudiff: wrote 9 goldens under "+e.golden+"/live (now "+smokeNow+")\n" {
		t.Errorf("stdout = %q", stdout)
	}

	m, err := harness.LoadGoldenManifest(e.golden)
	if err != nil {
		t.Fatal(err)
	}
	if m.Now != smokeNow || m.Oracle != e.goBin || m.OracleVersion != "v0.0.0-live" ||
		m.NodeVersion != "" || m.LiveSteps != len(liveSteps) {
		t.Errorf("manifest = %+v", m)
	}

	// The pinned clock reached the child as TUDIFF_NOW.
	if got := readLiveFile(t, filepath.Join(harness.GoldenLiveDir(e.golden, "sync"), "stdout")); got != smokeNow+"\n" {
		t.Errorf("sync golden stdout = %q, want the pinned clock", got)
	}
	// The extras: log.txt holds the seed commit, status.txt is empty, the
	// pull-failure step pinned exit 1, and the repair steps have trees.
	if got := readLiveFile(t, filepath.Join(harness.GoldenLiveDir(e.golden, "sync"), "log.txt")); !strings.Contains(got, "seed") {
		t.Errorf("sync log.txt lacks the seed commit:\n%s", got)
	}
	if got := readLiveFile(t, filepath.Join(harness.GoldenLiveDir(e.golden, "sync"), "status.txt")); got != "" {
		t.Errorf("sync status.txt = %q, want empty", got)
	}
	if got := readLiveFile(t, filepath.Join(harness.GoldenLiveDir(e.golden, "pull-failure"), "exit")); got != "1\n" {
		t.Errorf("pull-failure exit = %q, want 1", got)
	}
	if got := readLiveFile(t, filepath.Join(harness.GoldenLiveDir(e.golden, "rebase-recovery"), "stderr")); !strings.Contains(got, rebaseRecoveryNeedle) {
		t.Errorf("rebase-recovery stderr lacks the needle:\n%s", got)
	}
	if !fileExists(filepath.Join(harness.GoldenLiveDir(e.golden, "repair-write"), "tree.json")) {
		t.Error("repair-write tree.json missing")
	}

	code, stdout, stderr = e.invoke(t)
	if code != 0 {
		t.Fatalf("compare exit = %d, want 0 (stderr: %s)\n%s", code, stderr, stdout)
	}
	for _, sub := range []string{
		"GREEN   sync",
		"GREEN   repair-write",
		"tudiff: 9 cases — 9 green, 0 red (0 expected, 0 unexpected), 0 timeout",
	} {
		if !strings.Contains(stdout, sub) {
			t.Errorf("stdout lacks %q:\n%s", sub, stdout)
		}
	}
	// The header's golden provenance line lands in report.txt.
	report := readLiveFile(t, filepath.Join(e.report, "report.txt"))
	for _, sub := range []string{
		"golden: " + e.golden + " (captured ",
		"from " + e.goBin + " v0.0.0-live; now " + smokeNow + ")\n",
	} {
		if !strings.Contains(report, sub) {
			t.Errorf("report.txt lacks %q:\n%s", sub, report)
		}
	}
	if !fileExists(filepath.Join(e.report, "report.json")) {
		t.Error("report.json missing")
	}
}

// R4: a live step whose bytes diverge from the golden is red with golden=/go=
// excerpts and fails the gate.
func TestLiveCompareRedOnDivergence(t *testing.T) {
	e := newLiveSmokeEnv(t)
	if code, _, stderr := e.invoke(t, "--update", "--now", smokeNow); code != 0 {
		t.Fatalf("--update exit = %d, stderr = %q", code, stderr)
	}
	// The compare run's binary prints something else for cc-sync.
	writeExe(t, e.goBin, strings.Replace(liveTuStandIn, `printf '%s\n' "${TUDIFF_NOW:-unset}"`,
		`if [ "${1:-}" = "cc" ]; then printf 'different\n'; exit 0; fi
printf '%s\n' "${TUDIFF_NOW:-unset}"`, 1))
	code, stdout, stderr := e.invoke(t)
	if code != 1 {
		t.Fatalf("compare exit = %d, want 1 (stderr: %s)\n%s", code, stderr, stdout)
	}
	if !strings.Contains(stdout, "RED     cc-sync") || !strings.Contains(stdout, `golden="`) || !strings.Contains(stdout, `go="different\n"`) {
		t.Errorf("stdout lacks the golden=/go= red line:\n%s", stdout)
	}
}

// R3: live accepts --expected and its preflight rejects a missing file with
// the one-line message and exit 2 (the binary checks must pass first; the
// expected load precedes the golden checks, so no corpus is needed).
func TestLiveExpectedPreflight(t *testing.T) {
	e := newLiveSmokeEnv(t)

	t.Run("missing file", func(t *testing.T) {
		code, _, stderr := e.invoke(t, "--expected", "/nonexistent")
		if code != 2 || stderr != "tudiff: /nonexistent not found\n" {
			t.Errorf("code = %d, stderr = %q", code, stderr)
		}
	})
	t.Run("invalid file", func(t *testing.T) {
		bad := writeFile(t, t.TempDir(), "bad.json", `{"schema":2,"expected":[]}`+"\n")
		code, _, stderr := e.invoke(t, "--expected", bad)
		if code != 2 || stderr != "tudiff: expected-diffs: schema must be 1, got 2\n" {
			t.Errorf("code = %d, stderr = %q", code, stderr)
		}
	})
	t.Run("default missing under the fake root", func(t *testing.T) {
		if err := os.Remove(filepath.Join(e.root, "harness", "expected-diffs.json")); err != nil {
			t.Fatal(err)
		}
		code, _, stderr := e.invoke(t)
		if code != 2 || stderr != "tudiff: harness/expected-diffs.json not found\n" {
			t.Errorf("code = %d, stderr = %q", code, stderr)
		}
	})
}
