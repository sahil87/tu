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

// Git argv the round trip issues beyond the user-dir-dependent ones (the TS
// syncMetrics verb strings; the branch name is fixed, DC-18).
var (
	rebaseAbortArgs = []string{"rebase", "--abort"}
	pullArgs        = []string{"pull", "--rebase", "origin", "main"}
	pushArgs        = []string{"push"}
)

// The stderr warning lines SyncMetrics returns for the edge to write (the em
// dash is U+2014, matching the TS bytes).
const (
	rebaseRecoveryLine = "Warning: recovering from interrupted rebase"
	pullFailedPrefix   = "Warning: sync pull failed — "
	pushFailedPrefix   = "Warning: sync push failed after retry — "
)

// SyncMetrics is the TS syncMetrics (src/node/sync/sync.ts): the
// add/commit/pull/push round trip against the metrics repo. It prints
// nothing — lines are the stderr lines the TS would have printed, in order,
// for the edge to write.
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
	// unconditionally. Any error in this block returns false with no new
	// line (DC-18: a commit failure prints only the edge's generic error).
	if exists(filepath.Join(dir, user)) {
		if _, err := git.Run(dir, "add", userArg); err != nil {
			return false, lines
		}
	}
	status, err := git.Run(dir, "status", "--porcelain", userArg)
	if err != nil {
		return false, lines
	}
	if strings.TrimSpace(status) != "" {
		if _, err := git.Run(dir, "commit", "-m", CommitMessage(user, now)); err != nil {
			return false, lines
		}
	}

	if _, err := git.Run(dir, pullArgs...); err != nil {
		lines = append(lines, pullFailedPrefix+err.Error())
		_, _ = git.Run(dir, rebaseAbortArgs...) // error ignored — already clean (TS catch {})
		return false, lines
	}
	if _, err := git.Run(dir, pushArgs...); err != nil {
		if _, err := git.Run(dir, pushArgs...); err != nil {
			lines = append(lines, pushFailedPrefix+err.Error())
			return false, lines
		}
	}
	return true, lines
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
