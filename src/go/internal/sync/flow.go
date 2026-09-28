package sync

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sahil87/tu/internal/config"
	"github.com/sahil87/tu/internal/fact"
	"github.com/sahil87/tu/internal/source"
)

// Git argv the round trip issues beyond the user-dir-dependent ones.
var (
	rebaseAbortArgs     = []string{"rebase", "--abort"}
	upstreamArgs        = []string{"rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"}
	remoteHEADArgs      = []string{"ls-remote", "--symref", "origin", "HEAD"}
	pullUpstreamArgs    = []string{"pull", "--rebase"}
	pushArgs            = []string{"push"}
	pushSetUpstreamArgs = []string{"push", "-u", "origin", "HEAD"}
)

// The stderr warning lines SyncMetrics returns for the edge to write (the em
// dash is U+2014).
const (
	rebaseRecoveryLine = "Warning: recovering from interrupted rebase"
	addFailedPrefix    = "Warning: sync add failed — "
	statusFailedPrefix = "Warning: sync status failed — "
	commitFailedPrefix = "Warning: sync commit failed — "
	pullFailedPrefix   = "Warning: sync pull failed — "
	pushFailedPrefix   = "Warning: sync push failed after retry — "
)

// SyncMetrics is the add/commit/pull/push round trip against the metrics
// repo. It prints nothing — lines are the stderr lines for the edge to
// write, in order.
func SyncMetrics(dir, user string, now time.Time, git Runner) (ok bool, lines []string) {
	// Recover from an interrupted rebase left by a previous failed sync.
	// os.Stat is exactly the TS existsSync — a worktree-style .git file is
	// not special-cased, as in the TS.
	if exists(filepath.Join(dir, ".git", "rebase-merge")) || exists(filepath.Join(dir, ".git", "rebase-apply")) {
		lines = append(lines, rebaseRecoveryLine)
		_, _ = git.Run(dir, rebaseAbortArgs...) // error ignored — try to continue anyway (TS catch {})
	}

	userArg := user + "/"
	// The user dir may not exist yet (first run, or no data because every
	// source was unavailable): `git add <user>/` would fail with "pathspec
	// did not match any files", so staging is skipped — but the status runs
	// unconditionally. A failure in any of the three calls surfaces a
	// warning line carrying git's error before returning false.
	if exists(filepath.Join(dir, user)) {
		if _, err := git.Run(dir, "add", userArg); err != nil {
			lines = append(lines, addFailedPrefix+err.Error())
			return false, lines
		}
	}
	status, err := git.Run(dir, "status", "--porcelain", userArg)
	if err != nil {
		lines = append(lines, statusFailedPrefix+err.Error())
		return false, lines
	}
	if strings.TrimSpace(status) != "" {
		if _, err := git.Run(dir, "commit", "-m", CommitMessage(user, now)); err != nil {
			lines = append(lines, commitFailedPrefix+err.Error())
			return false, lines
		}
	}

	pull, emptyRemote, err := pullTarget(dir, git)
	if err != nil {
		lines = append(lines, pullFailedPrefix+err.Error())
		_, _ = git.Run(dir, rebaseAbortArgs...) // error ignored — already clean (TS catch {})
		return false, lines
	}
	if !emptyRemote {
		if _, err := git.Run(dir, pull...); err != nil {
			lines = append(lines, pullFailedPrefix+err.Error())
			_, _ = git.Run(dir, rebaseAbortArgs...) // error ignored — already clean (TS catch {})
			return false, lines
		}
	}
	push := pushArgs
	if emptyRemote {
		push = pushSetUpstreamArgs
	}
	if _, err := git.Run(dir, push...); err != nil {
		if _, err := git.Run(dir, push...); err != nil {
			lines = append(lines, pushFailedPrefix+err.Error())
			return false, lines
		}
	}
	return true, lines
}

// pullTarget resolves what the pull step pulls. When the current branch has
// an upstream (rev-parse exits 0 with non-empty output — a local check, no
// network), the pull runs with no remote/refspec and git follows the
// upstream. Otherwise the remote's default branch comes from the
// `ref: refs/heads/{branch}\tHEAD` line of `ls-remote --symref origin HEAD`.
// A remote advertising no refs (a fresh, empty repo) reports emptyRemote:
// the caller skips the pull and pushes `-u origin HEAD` so the first sync to
// a new repo succeeds and later syncs take the upstream path. An ls-remote
// failure is returned for the caller to report as a pull failure.
func pullTarget(dir string, git Runner) (args []string, emptyRemote bool, err error) {
	if out, err := git.Run(dir, upstreamArgs...); err == nil && strings.TrimSpace(out) != "" {
		return pullUpstreamArgs, false, nil
	}
	out, err := git.Run(dir, remoteHEADArgs...)
	if err != nil {
		return nil, false, err
	}
	for _, line := range strings.Split(out, "\n") {
		rest, ok := strings.CutPrefix(line, "ref: refs/heads/")
		if !ok {
			continue
		}
		branch, _, _ := strings.Cut(rest, "\t")
		if branch != "" {
			return []string{"pull", "--rebase", "origin", branch}, false, nil
		}
	}
	return nil, true, nil
}

// exists is the TS existsSync.
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Fetcher is the live-fetch seam: *ccusage.Source satisfies it (asserted at
// the edge). sync never imports the adapter (target architecture).
type Fetcher interface {
	FetchAll(ctx context.Context, period string, extraArgs []string, fresh bool) ([]fact.Record, []*source.Error)
}

// Inputs is everything FullSync needs: the post-guard Config (MetricsDir,
// User, Machine), the runtime-state dir where .last-sync lives
// (config.StateDir(home)), the clock, the fetch seam, and the git driver.
type Inputs struct {
	Config   config.Config
	StateDir string
	Now      time.Time
	Source   Fetcher
	Git      Runner
}

// Outcome is what the edge prints after FullSync. OK is SyncMetrics' result
// live and always true in dry-run; Report is set in dry-run only; Warnings
// are the fetch's per-source errors in registry order — the edge writes them
// with source.WriteWarnings BEFORE Lines, exactly where the TS prints them
// (the fetch precedes every git call); Lines are SyncMetrics' stderr lines
// in order (nil in dry-run).
type Outcome struct {
	OK       bool
	Report   *Report
	Warnings []*source.Error
	Lines    []string
}

// FullSync is the TS fullSync — one function, one decision path for both
// modes (the TS overload). Both modes: FetchAll(PeriodDaily, nil,
// fresh=false) — the cached daily fetch the TS fetchHistory makes — then,
// per fact.Tools in registry order, Write(MetricsDir, User, Machine, tool,
// recs, dryRun) on the records grouped by Tool. Dry-run additionally
// collects the decisions into Report and computes the git half read-only:
// WouldCommit = any write-decision OR a non-empty `status --porcelain
// {user}/` — run unconditionally, mirroring SyncMetrics' unconditional
// status (a tracked-but-deleted user dir still reports), with any error
// meaning not dirty (the dry-run never crashes on an un-synced setup). Live:
// after the writes SyncMetrics runs and on ok TouchLastSync(StateDir, Now).
// A Write or touch error is returned as error (the TS throws).
func FullSync(ctx context.Context, in Inputs, dryRun bool) (Outcome, error) {
	recs, warnings := in.Source.FetchAll(ctx, source.PeriodDaily, nil, false)
	byTool := make(map[string][]fact.Record)
	for _, r := range recs {
		byTool[r.Tool] = append(byTool[r.Tool], r)
	}

	if dryRun {
		report := &Report{
			MetricsDir:    in.Config.MetricsDir,
			User:          in.Config.User,
			Machine:       in.Config.Machine,
			CommitMessage: CommitMessage(in.Config.User, in.Now),
		}
		anyWrite := false
		for _, tool := range fact.Tools {
			decisions, err := Write(in.Config.MetricsDir, in.Config.User, in.Config.Machine, tool, byTool[tool.Key], true)
			if err != nil {
				return Outcome{}, err
			}
			for _, d := range decisions {
				if d.Action == ActionWrite {
					anyWrite = true
				}
			}
			report.Tools = append(report.Tools, ToolReport{Tool: tool, Decisions: decisions})
		}
		dirty := false
		if status, err := in.Git.Run(in.Config.MetricsDir, "status", "--porcelain", in.Config.User+"/"); err == nil {
			dirty = strings.TrimSpace(status) != ""
		} // any git failure = "no dirty files" (TS catch {})
		report.WouldCommit = anyWrite || dirty
		return Outcome{OK: true, Report: report, Warnings: warnings}, nil
	}

	for _, tool := range fact.Tools {
		if _, err := Write(in.Config.MetricsDir, in.Config.User, in.Config.Machine, tool, byTool[tool.Key], false); err != nil {
			return Outcome{}, err
		}
	}
	ok, lines := SyncMetrics(in.Config.MetricsDir, in.Config.User, in.Now, in.Git)
	if ok {
		if err := TouchLastSync(in.StateDir, in.Now); err != nil {
			return Outcome{}, err
		}
	}
	return Outcome{OK: ok, Warnings: warnings, Lines: lines}, nil
}
