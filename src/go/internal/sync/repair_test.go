package sync

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Port of src/node/sync/__tests__/repair-metrics.test.ts against real git
// (R11), plus a byte-parity diff against scripts/repair-metrics.mjs when node
// is on PATH. pinGitEnv and gitDo live in flow_test.go.

// repairEntry is the TS entryLine: one day-file record with the same field
// order. cost is a string so the fixture controls the exact bytes.
func repairEntry(label, cost string, tokens int) string {
	return fmt.Sprintf(`{"label":%q,"totalCost":%s,"inputTokens":100,"outputTokens":50,"cacheCreationTokens":0,"cacheReadTokens":0,"totalTokens":%d}`,
		label, cost, tokens) + "\n"
}

// streamBytes turns Repair's returned lines back into the mjs's stream bytes:
// lines joined by "\n" plus a single trailing "\n" (no lines → no output).
func streamBytes(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

func writeRepairDay(t *testing.T, repo, relPath, content string) {
	t.Helper()
	full := filepath.Join(repo, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitAll(t *testing.T, repo, msg string) string {
	t.Helper()
	gitDo(t, repo, "add", "-A")
	gitDo(t, repo, "commit", "-m", msg)
	return strings.TrimSpace(gitDo(t, repo, "rev-parse", "HEAD"))
}

func commitDate(t *testing.T, repo, sha string) string {
	t.Helper()
	return strings.TrimSpace(gitDo(t, repo, "log", "-1", "--format=%cs", sha))
}

// --- The R11 fixture ---

// seedR11Repo builds the plan's R11 history with real git:
//   - cc-2026-01-01.jsonl peaks at 10.00 in commit A and holds 1.00 at HEAD
//   - cc-2026-01-02.jsonl never shrinks
//   - cc-2026-01-03.jsonl has an unparseable historical blob (commit A) and a
//     parseable 8.00 (commit B); HEAD holds 1.00
//   - cc-2026-01-04.jsonl is deleted in commit B and re-added shrunk in
//     commit C (the delete commit's `git show` fails and is skipped)
//
// Returns the repo and the three commit SHAs (newest last).
func seedR11Repo(t *testing.T) (repo string, shaA, shaB, shaC string) {
	t.Helper()
	pinGitEnv(t)
	repo = filepath.Join(t.TempDir(), "metrics")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	gitDo(t, "", "init", repo)

	const dir = "u/2026/m"
	writeRepairDay(t, repo, dir+"/cc-2026-01-01.jsonl", repairEntry("2026-01-01", "10", 900))
	writeRepairDay(t, repo, dir+"/cc-2026-01-02.jsonl", repairEntry("2026-01-02", "3", 300))
	writeRepairDay(t, repo, dir+"/cc-2026-01-03.jsonl", "totally not json\n")
	writeRepairDay(t, repo, dir+"/cc-2026-01-04.jsonl", repairEntry("2026-01-04", "5", 500))
	shaA = commitAll(t, repo, "high-water marks")

	writeRepairDay(t, repo, dir+"/cc-2026-01-03.jsonl", repairEntry("2026-01-03", "8", 800))
	gitDo(t, repo, "rm", "-q", dir+"/cc-2026-01-04.jsonl")
	shaB = commitAll(t, repo, "garbage out, 04 deleted")

	writeRepairDay(t, repo, dir+"/cc-2026-01-01.jsonl", repairEntry("2026-01-01", "1", 10))
	writeRepairDay(t, repo, dir+"/cc-2026-01-03.jsonl", repairEntry("2026-01-03", "1", 10))
	writeRepairDay(t, repo, dir+"/cc-2026-01-04.jsonl", repairEntry("2026-01-04", "2", 20))
	shaC = commitAll(t, repo, "post-purge shrink")
	return repo, shaA, shaB, shaC
}

// r11DryRunBytes builds the expected dry-run stdout bytes for the R11 fixture.
func r11DryRunBytes(repo, shaA, shaB string, dateA, dateB string) string {
	const (
		p1 = "u/2026/m/cc-2026-01-01.jsonl"
		p3 = "u/2026/m/cc-2026-01-03.jsonl"
		p4 = "u/2026/m/cc-2026-01-04.jsonl"
	)
	var b strings.Builder
	fmt.Fprintf(&b, "repair-metrics: scanned %s\n", repo)
	b.WriteString("  4 tracked day-files, 3 commits touching *.jsonl\n")
	b.WriteString("\n")
	b.WriteString("Shrunk day-files (3):\n")
	b.WriteString("\n")
	fmt.Fprintf(&b, "  %-28s  %10s  %10s  %10s  MAX COMMIT\n", "FILE", "CURRENT", "MAX", "DELTA")
	fmt.Fprintf(&b, "  %-28s  %10s  %10s  %10s  %s (%s)\n", p1, "$1.00", "$10.00", "+$9.00", shaA[:7], dateA)
	fmt.Fprintf(&b, "  %-28s  %10s  %10s  %10s  %s (%s)\n", p3, "$1.00", "$8.00", "+$7.00", shaB[:7], dateB)
	fmt.Fprintf(&b, "  %-28s  %10s  %10s  %10s  %s (%s)\n", p4, "$2.00", "$5.00", "+$3.00", shaA[:7], dateA)
	b.WriteString("\n")
	b.WriteString("Per-user totals:\n")
	b.WriteString("  u: +$19.00 across 3 file(s)\n")
	b.WriteString("\n")
	b.WriteString("Grand total: +$19.00 across 3 file(s)\n")
	b.WriteString("\n")
	b.WriteString("Dry run — nothing modified. Re-run with --write to restore shrunk files.\n")
	return b.String()
}

// R11: the dry run prints the exact report bytes and modifies nothing.
func TestRepairR11DryRunBytes(t *testing.T) {
	repo, shaA, shaB, _ := seedR11Repo(t)
	before := readFileBytes(t, filepath.Join(repo, "u/2026/m/cc-2026-01-01.jsonl"))

	stdout, stderr, exit := Repair(RepairOptions{Repo: repo}, Exec{})
	if exit != 0 || len(stderr) != 0 {
		t.Fatalf("exit = %d, stderr = %v", exit, stderr)
	}
	want := r11DryRunBytes(repo, shaA, shaB, commitDate(t, repo, shaA), commitDate(t, repo, shaB))
	if got := streamBytes(stdout); got != want {
		t.Errorf("stdout bytes differ:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	if after := readFileBytes(t, filepath.Join(repo, "u/2026/m/cc-2026-01-01.jsonl")); after != before {
		t.Error("dry run modified the working tree")
	}
}

// R11: --write restores the historical-max blobs byte-exactly (working tree
// only — no commit), and a re-run reports nothing to repair.
func TestRepairR11WriteRestoresAndIsIdempotent(t *testing.T) {
	repo, _, _, _ := seedR11Repo(t)
	const dir = "u/2026/m"
	commitsBefore := strings.TrimSpace(gitDo(t, repo, "rev-list", "--count", "HEAD"))

	stdout, stderr, exit := Repair(RepairOptions{Repo: repo, Write: true}, Exec{})
	if exit != 0 || len(stderr) != 0 {
		t.Fatalf("exit = %d, stderr = %v", exit, stderr)
	}
	tail := strings.Join(stdout[len(stdout)-4:], "\n")
	wantTail := "\nRestored 3 file(s) in the working tree.\nReview with: git -C " + repo + " diff\nThen commit and push manually."
	if tail != wantTail {
		t.Errorf("write tail = %q, want %q", tail, wantTail)
	}

	// The full original blobs — token fields included, not just the cost.
	if got := readFileBytes(t, filepath.Join(repo, dir+"/cc-2026-01-01.jsonl")); got != repairEntry("2026-01-01", "10", 900) {
		t.Errorf("01 restored = %q, want the commit-A blob", got)
	}
	if got := readFileBytes(t, filepath.Join(repo, dir+"/cc-2026-01-03.jsonl")); got != repairEntry("2026-01-03", "8", 800) {
		t.Errorf("03 restored = %q, want the commit-B blob (unparseable A skipped)", got)
	}
	if got := readFileBytes(t, filepath.Join(repo, dir+"/cc-2026-01-04.jsonl")); got != repairEntry("2026-01-04", "5", 500) {
		t.Errorf("04 restored = %q, want the commit-A blob (delete commit skipped)", got)
	}
	if got := readFileBytes(t, filepath.Join(repo, dir+"/cc-2026-01-02.jsonl")); got != repairEntry("2026-01-02", "3", 300) {
		t.Errorf("02 = %q, want untouched", got)
	}

	// Working tree only: no commit created, changes left for review.
	if after := strings.TrimSpace(gitDo(t, repo, "rev-list", "--count", "HEAD")); after != commitsBefore {
		t.Errorf("commit count %s → %s, want unchanged", commitsBefore, after)
	}
	if status := gitDo(t, repo, "status", "--porcelain"); !strings.Contains(status, dir+"/cc-2026-01-01.jsonl") {
		t.Errorf("status = %q, want the restored file listed as modified", status)
	}

	// Idempotent: a second --write reports nothing to repair.
	stdout, stderr, exit = Repair(RepairOptions{Repo: repo, Write: true}, Exec{})
	if exit != 0 || len(stderr) != 0 {
		t.Fatalf("re-run exit = %d, stderr = %v", exit, stderr)
	}
	want := "repair-metrics: scanned " + repo + "\n" +
		"  4 tracked day-files, 3 commits touching *.jsonl\n" +
		"\n" +
		"Nothing to repair — every day-file is at its historical maximum.\n"
	if got := streamBytes(stdout); got != want {
		t.Errorf("idempotent re-run stdout = %q, want %q", got, want)
	}
}

// R11: failure modes — missing repo and non-git repo.
func TestRepairFailures(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nonexistent")
	stdout, stderr, exit := Repair(RepairOptions{Repo: missing}, Exec{})
	if exit != 1 || len(stdout) != 0 {
		t.Errorf("missing repo: exit = %d, stdout = %v", exit, stdout)
	}
	if got := streamBytes(stderr); got != "repair-metrics: repo not found: "+missing+"\n" {
		t.Errorf("missing repo stderr = %q", got)
	}

	plain := t.TempDir()
	stdout, stderr, exit = Repair(RepairOptions{Repo: plain}, Exec{})
	if exit != 1 || len(stdout) != 0 {
		t.Errorf("non-git repo: exit = %d, stdout = %v", exit, stdout)
	}
	if got := streamBytes(stderr); got != "repair-metrics: not a git repository: "+plain+"\n" {
		t.Errorf("non-git repo stderr = %q", got)
	}
}

// --- The repair-metrics.test.ts port (substring assertions like the TS) ---

// seedTSRepo is the TS seedShrunkRepo: one sahil file shrunk from 308.12 to
// 9.46, one bob file shrunk from 50 to 2, one sahil file that never shrank.
func seedTSRepo(t *testing.T) (repo, shrunk string) {
	t.Helper()
	pinGitEnv(t)
	repo = filepath.Join(t.TempDir(), "metrics")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	gitDo(t, "", "init", repo)

	shrunk = "sahil/2026/devws/cc-2026-04-24.jsonl"
	writeRepairDay(t, repo, shrunk, repairEntry("2026-04-24", "308.12", 9999))
	writeRepairDay(t, repo, "sahil/2026/devws/cc-2026-05-01.jsonl", repairEntry("2026-05-01", "10", 150))
	writeRepairDay(t, repo, "bob/2026/laptop/cc-2026-04-24.jsonl", repairEntry("2026-04-24", "50", 150))
	commitAll(t, repo, "initial high-water marks")
	writeRepairDay(t, repo, shrunk, repairEntry("2026-04-24", "9.46", 12))
	writeRepairDay(t, repo, "bob/2026/laptop/cc-2026-04-24.jsonl", repairEntry("2026-04-24", "2", 150))
	commitAll(t, repo, "post-purge shrink")
	return repo, shrunk
}

func repairStdout(t *testing.T, repo string, write bool) string {
	t.Helper()
	stdout, stderr, exit := Repair(RepairOptions{Repo: repo, Write: write}, Exec{})
	if exit != 0 {
		t.Fatalf("exit = %d, stderr = %v", exit, stderr)
	}
	return streamBytes(stdout)
}

// TS: dry run reports current, max and delta — and modifies nothing; files
// that never shrank are omitted.
func TestRepairTSDryRunReport(t *testing.T) {
	repo, shrunk := seedTSRepo(t)
	out := repairStdout(t, repo, false)
	for _, want := range []string{shrunk, "$9.46", "$308.12", "+$298.66", "Dry run"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "cc-2026-05-01.jsonl") {
		t.Errorf("never-shrunk file must not be listed:\n%s", out)
	}
	if got := readFileBytes(t, filepath.Join(repo, shrunk)); got != repairEntry("2026-04-24", "9.46", 12) {
		t.Error("dry run modified the working tree")
	}
}

// TS: per-user subtotals and a grand total across users (byte order: bob
// before sahil).
func TestRepairTSPerUserTotals(t *testing.T) {
	repo, _ := seedTSRepo(t)
	out := repairStdout(t, repo, false)
	for _, want := range []string{
		"Per-user totals:",
		"  bob: +$48.00 across 1 file(s)",
		"  sahil: +$298.66 across 1 file(s)",
		"Grand total: +$346.66 across 2 file(s)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout lacks %q:\n%s", want, out)
		}
	}
	if strings.Index(out, "bob:") > strings.Index(out, "sahil:") {
		t.Errorf("per-user rows out of order:\n%s", out)
	}
}

// TS: files within a cent of their historical max are not flagged.
func TestRepairTSWithinCentTolerance(t *testing.T) {
	pinGitEnv(t)
	repo := filepath.Join(t.TempDir(), "metrics")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	gitDo(t, "", "init", repo)
	path := "sahil/2026/devws/cc-2026-03-01.jsonl"
	writeRepairDay(t, repo, path, repairEntry("2026-03-01", "1.005", 150))
	commitAll(t, repo, "high")
	writeRepairDay(t, repo, path, repairEntry("2026-03-01", "1", 150))
	commitAll(t, repo, "within tolerance")
	out := repairStdout(t, repo, false)
	if strings.Contains(out, path) {
		t.Errorf("within-a-cent file must not be flagged:\n%s", out)
	}
	if !strings.Contains(out, "Nothing to repair") {
		t.Errorf("expected nothing-to-repair:\n%s", out)
	}
}

// TS: the maximum is picked across 3+ versions (max in the middle).
func TestRepairTSMaxInMiddleOfHistory(t *testing.T) {
	pinGitEnv(t)
	repo := filepath.Join(t.TempDir(), "metrics")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	gitDo(t, "", "init", repo)
	path := "sahil/2026/devws/cc-2026-02-10.jsonl"
	writeRepairDay(t, repo, path, repairEntry("2026-02-10", "5", 150))
	commitAll(t, repo, "v1")
	writeRepairDay(t, repo, path, repairEntry("2026-02-10", "100", 7777))
	commitAll(t, repo, "v2 — the high-water mark")
	writeRepairDay(t, repo, path, repairEntry("2026-02-10", "20", 150))
	commitAll(t, repo, "v3")
	out := repairStdout(t, repo, false)
	for _, want := range []string{"$100.00", "+$80.00"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout lacks %q:\n%s", want, out)
		}
	}
}

// TS: unparseable historical versions are skipped without crashing.
func TestRepairTSUnparseableHistorySkipped(t *testing.T) {
	pinGitEnv(t)
	repo := filepath.Join(t.TempDir(), "metrics")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	gitDo(t, "", "init", repo)
	path := "sahil/2026/devws/cc-2026-02-11.jsonl"
	writeRepairDay(t, repo, path, "totally not json\n")
	commitAll(t, repo, "garbage version")
	writeRepairDay(t, repo, path, repairEntry("2026-02-11", "50", 150))
	commitAll(t, repo, "good version")
	writeRepairDay(t, repo, path, repairEntry("2026-02-11", "5", 150))
	commitAll(t, repo, "shrunk version")
	out := repairStdout(t, repo, false)
	for _, want := range []string{"$50.00", "+$45.00"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout lacks %q:\n%s", want, out)
		}
	}
}

// TS: --write restores the full historical-max content byte-exactly, without
// committing.
func TestRepairTSWriteRestoresFullBlob(t *testing.T) {
	repo, shrunk := seedTSRepo(t)
	high := repairEntry("2026-04-24", "308.12", 9999)
	commitsBefore := strings.TrimSpace(gitDo(t, repo, "rev-list", "--count", "HEAD"))
	out := repairStdout(t, repo, true)
	if !strings.Contains(out, "Restored 2 file(s)") {
		t.Errorf("expected restore summary:\n%s", out)
	}
	if got := readFileBytes(t, filepath.Join(repo, shrunk)); got != high {
		t.Errorf("restored = %q, want the full original blob %q", got, high)
	}
	if after := strings.TrimSpace(gitDo(t, repo, "rev-list", "--count", "HEAD")); after != commitsBefore {
		t.Errorf("commit count %s → %s, want unchanged (working tree only)", commitsBefore, after)
	}
	if status := gitDo(t, repo, "status", "--porcelain"); !strings.Contains(status, shrunk) {
		t.Errorf("status = %q, want the restored file listed as modified", status)
	}
}

func readFileBytes(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// --- Byte parity with scripts/repair-metrics.mjs ---

// runMJS runs the frozen oracle script and returns its raw streams and exit
// code.
func runMJS(t *testing.T, script, repo string, write bool) (stdout, stderr string, exit int) {
	t.Helper()
	args := []string{script, "--repo", repo}
	if write {
		args = append(args, "--write")
	}
	cmd := exec.Command("node", args...)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err == nil {
		return outBuf.String(), errBuf.String(), 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return outBuf.String(), errBuf.String(), exitErr.ExitCode()
	}
	t.Fatalf("node %v: %v", args, err)
	return "", "", -1
}

// copyTree copies a directory recursively (the mjs and Go sides need
// byte-identical repos — git objects included).
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	cmd := exec.Command("cp", "-a", src, dst)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("cp -a %s %s: %v\n%s", src, dst, err, out)
	}
}

// jsonlTree returns the relative *.jsonl paths and their bytes under root.
func jsonlTree(t *testing.T, root string) map[string]string {
	t.Helper()
	tree := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		tree[rel] = readFileBytes(t, path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

// R11/A-011: the Go Repair and the mjs produce byte-identical stdout, stderr
// and exit codes in both modes, and --write leaves byte-identical trees.
// Skipped when node is not on PATH.
func TestRepairNodeParity(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not on PATH")
	}
	script, err := filepath.Abs("../../../../scripts/repair-metrics.mjs")
	if err != nil {
		t.Fatal(err)
	}
	repoA, _, _, _ := seedR11Repo(t)
	repoB := filepath.Join(t.TempDir(), "metrics")
	copyTree(t, repoA, repoB)

	for _, write := range []bool{false, true} {
		mjsOut, mjsErr, mjsExit := runMJS(t, script, repoA, write)
		goOut, goErr, goExit := Repair(RepairOptions{Repo: repoB, Write: write}, Exec{})
		norm := func(s, repo string) string { return strings.ReplaceAll(s, repo, "$REPO") }
		if got, want := norm(streamBytes(goOut), repoB), norm(mjsOut, repoA); got != want {
			t.Errorf("write=%v stdout differs:\n--- go ---\n%s\n--- mjs ---\n%s", write, got, want)
		}
		if got, want := norm(streamBytes(goErr), repoB), norm(mjsErr, repoA); got != want {
			t.Errorf("write=%v stderr differs: go %q, mjs %q", write, got, want)
		}
		if goExit != mjsExit {
			t.Errorf("write=%v exit: go %d, mjs %d", write, goExit, mjsExit)
		}
	}

	treeA, treeB := jsonlTree(t, repoA), jsonlTree(t, repoB)
	if len(treeA) != len(treeB) {
		t.Fatalf("tree sizes differ: mjs %d, go %d", len(treeA), len(treeB))
	}
	for rel, contentA := range treeA {
		if contentB, ok := treeB[rel]; !ok || contentA != contentB {
			t.Errorf("tree differs at %s", rel)
		}
	}
}
