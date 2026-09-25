// repair.go ports the retired TypeScript repair script (R11) — the
// one-time repair that restores shrunk metrics day-files to their historical
// maximum from the metrics repo's git history. Why: Claude Code purges
// session transcripts older than ~30 days, so a machine's live ccusage view
// of an old day collapses toward zero; before the never-shrink guard in
// Write shipped, every sync overwrote correct per-day snapshots with that
// post-purge residue. Nothing was ever deleted from git history, so every
// shrunk day-file is restored losslessly from the commit where its totalCost
// was highest. Output is byte-exact with the retired script (the frozen live goldens under harness/golden/live/ pin the bytes); the returned lines joined
// by "\n" plus a trailing "\n" are the retired script's stdout bytes (streamBytes in
// repair_test.go defines the split), and stderr lines are the retired script's `fail`
// messages.
package sync

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/sahil87/tu/internal/render"
)

// centTolerance is the retired script's CENT_TOLERANCE: a file is "shrunk" only when HEAD
// is below its historical max by more than a cent — float noise within a cent
// is not worth touching.
const centTolerance = 0.01

// dayFileRE is the retired script's DAY_FILE_RE: day-file names follow
// {toolKey}-{YYYY-MM-DD}.jsonl (see Write).
var dayFileRE = regexp.MustCompile(`-\d{4}-\d{2}-\d{2}\.jsonl$`)

// commitLineRE matches one `git log --format=%H%x09%cs` header line.
var commitLineRE = regexp.MustCompile(`^([0-9a-f]{40})\t(\d{4}-\d{2}-\d{2})$`)

// failPrefix leads every repair failure message (the retired script's fail()).
const failPrefix = "repair-metrics: "

// quotePathArgs lead every git call the retired script makes (["-C", repo, "-c",
// "core.quotePath=false", ...]) — raw UTF-8 paths, no C-style quoting to
// unescape. Exec.Run prepends its own "-C <dir>" and passes these through.
var quotePathArgs = []string{"-c", "core.quotePath=false"}

// RepairOptions selects the repo and mode; cmd/turepair fills it from argv.
type RepairOptions struct {
	Repo  string // default ExpandHome("~/.tu/metrics_repo"); --repo <path>
	Write bool   // --write; default dry-run
}

// Repair runs the retired script's algorithm and returns its stdout lines and exit code;
// stderr lines are the retired script's `fail` messages ("repair-metrics: {msg}").
func Repair(o RepairOptions, git Runner) (stdout []string, stderr []string, exit int) {
	repo := o.Repo
	if _, err := os.Stat(repo); err != nil {
		return nil, FailLines("repo not found: " + repo), 1
	}
	if _, ok := repairGit(git, repo, "rev-parse", "--is-inside-work-tree"); !ok {
		return nil, FailLines("not a git repository: " + repo), 1
	}

	dayFiles, stderr, exit := listTrackedDayFiles(git, repo)
	if stderr != nil {
		return nil, stderr, exit
	}
	commitCount, fileCommits, stderr, exit := buildFileCommitMap(git, repo)
	if stderr != nil {
		return nil, stderr, exit
	}

	var shrunk []shrunkFile
	for _, path := range dayFiles {
		max := findHistoricalMax(git, repo, path, fileCommits[path])
		if max == nil {
			continue // no parseable history — nothing to compare
		}
		current := currentCost(repo, path)
		if delta := max.cost - current; delta > centTolerance {
			shrunk = append(shrunk, shrunkFile{path: path, current: current, max: *max, delta: delta})
		}
	}
	slices.SortFunc(shrunk, func(a, b shrunkFile) int { return localeCompare(a.path, b.path) })

	stdout = repairReport(repo, shrunk, len(dayFiles), commitCount)
	if len(shrunk) == 0 {
		return stdout, nil, 0
	}

	if !o.Write {
		return append(stdout, "",
			"Dry run — nothing modified. Re-run with --write to restore shrunk files."), nil, 0
	}

	// Restore the full original content (the exact historical-max blob), not
	// just the cost field — each day-file stays an atomic snapshot that was
	// real at some point in time. Working tree only: review, commit, and push
	// are deliberately left to the user. A write failure is ignored as the
	// retired script's uncaught throw leaves the same half-restored tree.
	for _, s := range shrunk {
		_ = os.WriteFile(filepath.Join(repo, s.path), []byte(s.max.content), 0o644)
	}
	return append(stdout, "",
		fmt.Sprintf("Restored %d file(s) in the working tree.", len(shrunk)),
		"Review with: git -C "+repo+" diff",
		"Then commit and push manually."), nil, 0
}

// FailLines renders the retired script's fail(): one stderr write of
// "repair-metrics: {msg}\n" (msg may itself carry a newline + usage line).
// Exported for cmd/turepair's arg-parse failures so both failure surfaces
// render identically.
func FailLines(msg string) []string {
	lines := strings.Split(msg, "\n")
	lines[0] = failPrefix + lines[0]
	return lines
}

// repairGit is the retired script's git() helper: every call carries quotePathArgs before
// the verb; ok is false on any failure (the retired script returns null).
func repairGit(git Runner, repo string, args ...string) (out string, ok bool) {
	argv := append(slices.Clone(quotePathArgs), args...)
	out, err := git.Run(repo, argv...)
	if err != nil {
		return "", false
	}
	return out, true
}

// listTrackedDayFiles is the retired script's listTrackedDayFiles: tracked day-files at
// HEAD (paths relative to the repo root).
func listTrackedDayFiles(git Runner, repo string) (files []string, stderr []string, exit int) {
	out, ok := repairGit(git, repo, "ls-files", "-z", "--", "*.jsonl")
	if !ok {
		return nil, FailLines("git ls-files failed — is this a metrics repo checkout?"), 1
	}
	for _, p := range strings.Split(out, "\x00") {
		if p != "" && dayFileRE.MatchString(p) {
			files = append(files, p)
		}
	}
	return files, nil, 0
}

// fileCommit is one commit that touched a day-file.
type fileCommit struct {
	sha  string
	date string
}

// buildFileCommitMap is the retired script's buildFileCommitMap: ONE history walk (avoids
// a git log per file) — every commit on the checked-out branch that touches a
// *.jsonl file, with the paths it touched. The per-file commit lists are
// newest-first (git log order).
func buildFileCommitMap(git Runner, repo string) (commitCount int, fileCommits map[string][]fileCommit, stderr []string, exit int) {
	out, ok := repairGit(git, repo, "log", "--format=%H%x09%cs", "--name-only", "--", "*.jsonl")
	if !ok {
		return 0, nil, FailLines("git log failed — is this a metrics repo checkout?"), 1
	}
	fileCommits = make(map[string][]fileCommit)
	var current *fileCommit
	for _, line := range strings.Split(out, "\n") {
		if m := commitLineRE.FindStringSubmatch(line); m != nil {
			current = &fileCommit{sha: m[1], date: m[2]}
			commitCount++
			continue
		}
		if line == "" || current == nil || !dayFileRE.MatchString(line) {
			continue
		}
		fileCommits[line] = append(fileCommits[line], *current)
	}
	return commitCount, fileCommits, nil, 0
}

// historicalMax is the retired script's findHistoricalMax result: the commit where the
// file's totalCost was highest, with that commit's full blob.
type historicalMax struct {
	cost    float64
	sha     string
	date    string
	content string
}

// findHistoricalMax is the retired script's findHistoricalMax: the highest parseable
// totalCost across every commit that touched the file. Deleted-at-commit
// paths and unparseable blobs are skipped; nil when no version parses.
func findHistoricalMax(git Runner, repo, path string, commits []fileCommit) *historicalMax {
	var max *historicalMax
	for _, c := range commits {
		content, ok := repairGit(git, repo, "show", c.sha+":"+path)
		if !ok {
			continue // path deleted in this commit — skip
		}
		cost := parseCost(content)
		if cost == nil {
			continue // unparseable historical version — skip
		}
		if max == nil || *cost > max.cost {
			max = &historicalMax{cost: *cost, sha: c.sha, date: c.date, content: content}
		}
	}
	return max
}

// currentCost is the retired script's currentCost: working-tree totalCost;
// missing/unparseable counts as 0 (fully shrunk).
func currentCost(repo, path string) float64 {
	raw, err := os.ReadFile(filepath.Join(repo, path))
	if err != nil {
		return 0
	}
	if cost := parseCost(string(raw)); cost != nil {
		return *cost
	}
	return 0
}

// parseCost is the retired script's parseCost: parses the first line of a day-file blob;
// returns a finite totalCost or nil. The finite check and the Number()
// coercion reuse the writer's jsNumber table (Number(parsed?.totalCost)).
func parseCost(content string) *float64 {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil
	}
	first, _, _ := strings.Cut(trimmed, "\n")
	var doc any
	if err := json.Unmarshal([]byte(first), &doc); err != nil {
		return nil
	}
	obj, ok := doc.(map[string]any)
	if !ok {
		return nil // top-level non-object → parsed?.totalCost = undefined → NaN
	}
	value, present := obj["totalCost"]
	if !present {
		return nil
	}
	cost := jsNumber(value)
	if math.IsNaN(cost) || math.IsInf(cost, 0) {
		return nil
	}
	return &cost
}

// shrunkFile is one file below its historical max by more than a cent.
type shrunkFile struct {
	path    string
	current float64
	max     historicalMax
	delta   float64
}

// userAgg accumulates one user's delta and file count for the per-user rows.
type userAgg struct {
	delta float64
	files int
}

// money is the retired script's money: "$" + v.toFixed(2) (FixedHalfUp is the toFixed
// twin).
func money(v float64) string {
	return "$" + render.FixedHalfUp(v, 2)
}

// padEnd/padStart are the JS String.prototype pad helpers for the report
// table (space padding; the table is ASCII, so byte length == JS length).
func padEnd(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

func padStart(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return strings.Repeat(" ", width-len(s)) + s
}

// repairReport is the retired script's printReport: the scanned header, either the
// nothing-to-repair line or the shrunk table with per-user and grand totals.
// The retired script emits this as out.join("\n") + "\n", so blank entries are blank
// lines.
func repairReport(repo string, shrunk []shrunkFile, dayFileCount, commitCount int) []string {
	out := []string{
		"repair-metrics: scanned " + repo,
		fmt.Sprintf("  %d tracked day-files, %d commits touching *.jsonl", dayFileCount, commitCount),
		"",
	}

	if len(shrunk) == 0 {
		return append(out, "Nothing to repair — every day-file is at its historical maximum.")
	}

	pathWidth := len("FILE")
	for _, s := range shrunk {
		pathWidth = max(pathWidth, len(s.path))
	}
	out = append(out,
		fmt.Sprintf("Shrunk day-files (%d):", len(shrunk)),
		"",
		"  "+padEnd("FILE", pathWidth)+"  "+padStart("CURRENT", 10)+"  "+padStart("MAX", 10)+"  "+padStart("DELTA", 10)+"  MAX COMMIT",
	)
	for _, s := range shrunk {
		out = append(out,
			"  "+padEnd(s.path, pathWidth)+
				"  "+padStart(money(s.current), 10)+
				"  "+padStart(money(s.max.cost), 10)+
				"  "+padStart("+"+money(s.delta), 10)+
				"  "+s.max.sha[:7]+" ("+s.max.date+")",
		)
	}

	// Per-user totals — the user is the first path segment.
	byUser := make(map[string]userAgg)
	for _, s := range shrunk {
		user, _, _ := strings.Cut(s.path, "/")
		agg := byUser[user]
		agg.delta += s.delta
		agg.files++
		byUser[user] = agg
	}
	out = append(out, "", "Per-user totals:")
	users := make([]string, 0, len(byUser))
	for u := range byUser {
		users = append(users, u)
	}
	slices.SortFunc(users, userEntryCompare)
	for _, u := range users {
		agg := byUser[u]
		out = append(out, fmt.Sprintf("  %s: +%s across %d file(s)", u, money(agg.delta), agg.files))
	}

	grand := 0.0
	for _, s := range shrunk {
		grand += s.delta
	}
	return append(out, "",
		fmt.Sprintf("Grand total: +%s across %d file(s)", money(grand), len(shrunk)))
}
