package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// Error is a setup command's operational failure: the message is printed on
// stderr verbatim, exit 1 (command.ExitOperational). Usage errors (exit 2)
// never originate here — they are command.UsageError from Parse.
type Error struct{ Message string }

func (e *Error) Error() string { return e.Message }

// FieldBlocks are the six scaffold blocks, in this order, byte-exact (the TS
// FIELD_BLOCKS).
var FieldBlocks = []struct{ Key, Block string }{
	{"version", "\n# Config schema version\nversion = 2\n"},
	{"metrics_repo", "\n# Git repo URL for metrics storage (enables multi-machine sync)\n# Set here or via TU_METRICS_REPO env var\n# metrics_repo = git@github.com:you/tu-metrics.git\n"},
	{"metrics_dir", "\n# Optional: local path where the metrics repo is cloned (default: ~/.tu/metrics_repo)\n# metrics_dir = ~/.tu/metrics_repo\n"},
	{"machine", "\n# Optional: label for this machine in the metrics repo (default: system hostname)\n# machine = my-macbook\n"},
	{"user", "\n# Optional: profile name — groups your machines in the metrics repo (default: system username)\n# user = your-name\n"},
	{"auto_sync", "\n# Auto-sync: no longer auto-triggers; use 'tu <cmd> --sync' to sync before fetch\nauto_sync = true\n"},
}

// fieldBlock looks up a scaffold block by key.
func fieldBlock(key string) string {
	for _, fb := range FieldBlocks {
		if fb.Key == key {
			return fb.Block
		}
	}
	return ""
}

// fieldActiveRe / fieldMentionRe are the TS fieldPresent / fieldMentioned
// shapes, precompiled per FieldBlocks key.
var (
	fieldActiveRe  = map[string]*regexp.Regexp{}
	fieldMentionRe = map[string]*regexp.Regexp{}
)

func init() {
	for _, fb := range FieldBlocks {
		fieldActiveRe[fb.Key] = regexp.MustCompile(`^` + fb.Key + `\s*=`)
		fieldMentionRe[fb.Key] = regexp.MustCompile(`(?m)^\s*#?\s*` + fb.Key + `\s*=`)
	}
}

// fieldPresent is the TS fieldPresent: some line, after leading whitespace is
// trimmed, is not a comment and starts with `{key}\s*=`.
func fieldPresent(content, key string) bool {
	re := fieldActiveRe[key]
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimLeftFunc(line, unicode.IsSpace)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if re.MatchString(trimmed) {
			return true
		}
	}
	return false
}

// fieldMentioned is the TS fieldMentioned: the whole content matches a
// possibly-commented `{key}\s*=` line (multiline).
func fieldMentioned(content, key string) bool {
	return fieldMentionRe[key].MatchString(content)
}

// ensureUserConf is the TS ensureUserConf, shared by InitConf and InitMetrics:
// when the user conf exists, returns created=false; otherwise creates the
// config dir (0o755) and writes the user conf (0o644) — seeded from the
// legacy file's bytes when that exists (the write-time copy; the legacy file
// is never moved or deleted), else from the shipped defaults — and returns
// the stdout line for the creation. A filesystem failure is *Error carrying
// the OS error text (the TS throws — an uncaught exception, exit 1).
func ensureUserConf(p Paths) (created bool, line string, err error) {
	if fileExists(p.UserConf) {
		return false, "", nil
	}
	if err := os.MkdirAll(p.ConfigDir, 0o755); err != nil {
		return false, "", &Error{Message: err.Error()}
	}
	if fileExists(p.LegacyConf) {
		raw, err := os.ReadFile(p.LegacyConf)
		if err != nil {
			return false, "", &Error{Message: err.Error()}
		}
		if err := os.WriteFile(p.UserConf, raw, 0o644); err != nil {
			return false, "", &Error{Message: err.Error()}
		}
		return true, "Copied " + Tildefy(p.LegacyConf, p.Home) + " → " + Tildefy(p.UserConf, p.Home), nil
	}
	if err := os.WriteFile(p.UserConf, DefaultConf, 0o644); err != nil {
		return false, "", &Error{Message: err.Error()}
	}
	return true, "Created " + Tildefy(p.UserConf, p.Home) + " — edit it to configure multi-machine sync.", nil
}

// InitConf is `tu init-conf` (the TS runInitConf): returns the stdout lines.
// It cannot fail operationally except on a filesystem error, returned as
// *Error with the OS error text.
func InitConf(p Paths) ([]string, error) {
	created, line, err := ensureUserConf(p)
	if err != nil {
		return nil, err
	}
	if created {
		return []string{line}, nil
	}

	raw, err := os.ReadFile(p.UserConf)
	if err != nil {
		return nil, &Error{Message: err.Error()}
	}
	content := string(raw)
	var missing, commented []string
	for _, fb := range FieldBlocks {
		if fieldPresent(content, fb.Key) {
			continue
		}
		if fieldMentioned(content, fb.Key) {
			commented = append(commented, fb.Key)
		} else {
			missing = append(missing, fb.Key)
		}
	}

	dp := Tildefy(p.UserConf, p.Home)
	if len(missing) == 0 && len(commented) == 0 {
		return []string{dp + " is already complete."}, nil
	}

	var lines []string
	if len(missing) > 0 {
		var block strings.Builder
		for _, key := range missing {
			block.WriteString(fieldBlock(key))
		}
		f, err := os.OpenFile(p.UserConf, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, &Error{Message: err.Error()}
		}
		if _, err := f.WriteString(block.String()); err != nil {
			_ = f.Close()
			return nil, &Error{Message: err.Error()}
		}
		if err := f.Close(); err != nil {
			return nil, &Error{Message: err.Error()}
		}
		lines = append(lines, "Updated "+dp+" — added missing fields: "+strings.Join(missing, ", ")+".")
	}
	if len(commented) > 0 {
		lines = append(lines, dp+" has commented-out fields that need uncommenting: "+strings.Join(commented, ", ")+".")
	}
	return lines, nil
}

// Git is the one git question config asks; internal/sync.Exec answers it for
// real.
type Git interface {
	IsRepo(dir string) bool
}

// CloneStep is what the edge must run when InitMetrics decides a clone is
// needed: `git clone <URL> <Dir>` with stdout/stderr passed through.
type CloneStep struct {
	URL, Dir string
}

// InitMetricsResult is the config side of `tu init-metrics [url]`.
type InitMetricsResult struct {
	Lines    []string   // stdout lines produced so far, in order
	Warnings []string   // the Load cascade warnings (stderr at the edge)
	Clone    *CloneStep // nil when nothing is left to do (Already initialized)
}

// InitMetrics is the TS runInitMetrics, up to (not including) the clone; git
// is consulted only to answer "is Dir a git repo" (git -C Dir rev-parse
// --git-dir).
func InitMetrics(p Paths, env Env, url *string, git Git) (InitMetricsResult, error) {
	var res InitMetricsResult
	var ov Overrides
	if url != nil {
		// The URL is written verbatim into tu.conf — reject newline/CR so a
		// crafted argument cannot inject extra config lines into the file.
		if strings.ContainsAny(*url, "\r\n") {
			return res, &Error{Message: "Error: repo-url must be a single line (no newline or carriage-return characters)."}
		}
		// CLI-flag layer: the typed URL is written into tu.conf and beats an
		// exported TU_METRICS_REPO for this invocation's clone (CLI > env).
		created, line, err := ensureUserConf(p)
		if err != nil {
			return res, err
		}
		if created {
			res.Lines = append(res.Lines, line)
		}
		if err := setMetricsRepoInConf(p.UserConf, *url); err != nil {
			return res, err
		}
		res.Lines = append(res.Lines, "Set metrics_repo = "+*url+" in "+Tildefy(p.UserConf, p.Home))
		ov.MetricsRepo = url
	}

	cfg, warnings := Load(p, env, ov)
	res.Warnings = warnings
	dp := Tildefy(p.UserConf, p.Home)

	if cfg.MetricsRepo == "" {
		return res, &Error{Message: "Error: metrics_repo is not set. Add it to " + dp + ", run 'tu init-metrics <repo-url>', or set TU_METRICS_REPO."}
	}

	if fileExists(cfg.MetricsDir) {
		if git.IsRepo(cfg.MetricsDir) {
			res.Lines = append(res.Lines, "Already initialized: "+cfg.MetricsDir)
			return res, nil
		}
		return res, &Error{Message: "Error: " + cfg.MetricsDir + " exists but is not a git repo. Remove it or set a different metrics_dir in " + dp + "."}
	}

	res.Clone = &CloneStep{URL: cfg.MetricsRepo, Dir: cfg.MetricsDir}
	return res, nil
}

// setMetricsRepoInConf is the TS setMetricsRepoInConf: replace an active
// assignment in place, else replace the scaffold's commented sample line,
// else append the metrics_repo FieldBlocks block with the value filled in.
func setMetricsRepoInConf(userConf, repoURL string) error {
	raw, err := os.ReadFile(userConf)
	if err != nil {
		return &Error{Message: err.Error()}
	}
	lines := strings.Split(string(raw), "\n")
	activeRe := fieldActiveRe["metrics_repo"]
	for i, l := range lines {
		t := strings.TrimLeftFunc(l, unicode.IsSpace)
		if !strings.HasPrefix(t, "#") && activeRe.MatchString(t) {
			lines[i] = "metrics_repo = " + repoURL
			return writeUserConf(userConf, strings.Join(lines, "\n"))
		}
	}
	commentedRe := regexp.MustCompile(`^\s*#\s*metrics_repo\s*=`)
	for i, l := range lines {
		if commentedRe.MatchString(l) {
			lines[i] = "metrics_repo = " + repoURL
			return writeUserConf(userConf, strings.Join(lines, "\n"))
		}
	}
	block := strings.Replace(fieldBlock("metrics_repo"),
		"# metrics_repo = git@github.com:you/tu-metrics.git", "metrics_repo = "+repoURL, 1)
	f, err := os.OpenFile(userConf, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return &Error{Message: err.Error()}
	}
	if _, err := f.WriteString(block); err != nil {
		_ = f.Close()
		return &Error{Message: err.Error()}
	}
	if err := f.Close(); err != nil {
		return &Error{Message: err.Error()}
	}
	return nil
}

func writeUserConf(userConf, content string) error {
	if err := os.WriteFile(userConf, []byte(content), 0o644); err != nil {
		return &Error{Message: err.Error()}
	}
	return nil
}

// ClonedLine is the stdout line the edge prints after a successful clone;
// dir is ABSOLUTE (DC-19).
func ClonedLine(url, dir string) string {
	return "Cloned " + url + " → " + dir
}

// CloneFailedMarker is the clone-failure cooldown file under StateDir (B3
// writes and reads it).
const CloneFailedMarker = ".clone-failed"

// RemoveCloneMarker deletes stateDir/.clone-failed if present; best-effort,
// never errors (the TS removeCloneMarker).
func RemoveCloneMarker(stateDir string) {
	_ = os.Remove(filepath.Join(stateDir, CloneFailedMarker))
}
