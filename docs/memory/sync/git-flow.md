---
type: memory
description: The git driver and round trip behind tu sync — sync.Exec (PATH-resolved git, maxBuffer-capped captures, Node-style error text), SyncMetrics' add/commit/pull/push order with upstream-following pull and push targets, rebase-abort recovery, per-step failure warnings, CommitMessage, and TouchLastSync/Stale with the 3-hour auto-sync TTL
---

# Git Flow

**Domain**: sync

## Overview

`src/go/internal/sync/git.go` and `flow.go` own the git round trip behind `tu sync`: `sync.Exec` drives the real `git` found on PATH, and `sync.SyncMetrics` runs the add/commit/pull/push round trip against the metrics-repo clone — resolving the pull target from the branch's upstream or the remote's default branch — with interrupted-rebase recovery. `state.go` holds the commit message and the `.last-sync` staleness helpers. The write side is [day-file-writer](/sync/day-file-writer.md); the dry-run preview that skips this round trip is [dry-run-report](/sync/dry-run-report.md).

## Requirements

### Requirement: Exec drives the git on PATH
`sync.Exec` MUST satisfy `sync.Runner` (`Run(dir string, args ...string) (stdout string, err error)`) by executing `git -C <dir> <args...>` resolved through PATH, with no timeout and stdout/stderr captured separately. Each stream MUST be capped at `Exec.MaxBuffer` bytes: zero selects `MaxBufferSync` (10 MiB, `10 * 1024 * 1024`); `cmd/turepair` passes `MaxBufferRepair` (64 MiB, `64 * 1024 * 1024`). A stream overflowing its cap MUST kill the child, and the overflow error MUST be reported even when the killed child exits 0. `Exec.IsRepo(dir)` MUST run `git -C <dir> rev-parse --git-dir` with both streams discarded and report exit 0.

### Requirement: Exec.Run error text is fixed
On failure, `Exec.Run`'s error MUST read `git -C <dir>... failed: {message}` where the message is: `Command failed: git -C <dir> <args joined by single spaces>\n<stderr>` for a non-zero exit (the newline is unconditional — empty stderr still ends the message in `"\n"`); `{stdout|stderr} maxBuffer length exceeded` on a capture overflow (stdout checked before stderr, and reported even when the killed child exits 0); `spawn git ENOENT` when git is not on PATH. There MUST be no timeout on `Run`.

#### Scenario: Overflow reported over a clean exit
- **GIVEN** a git verb whose stdout exceeds `MaxBuffer`
- **WHEN** the overflow kills the child and the killed child exits 0
- **THEN** `Exec.Run` returns an error reading `{stdout|stderr} maxBuffer length exceeded`, not the captured prefix

### Requirement: SyncMetrics runs add/commit/probe/pull/push in order
`sync.SyncMetrics(dir, user string, now time.Time, git Runner) (ok bool, lines []string)` MUST print nothing and MUST return the stderr lines in order:

1. **Rebase recovery**: when `{dir}/.git/rebase-merge` or `{dir}/.git/rebase-apply` exists (`os.Stat`; a worktree-style `.git` file is not special-cased) → append `Warning: recovering from interrupted rebase`, run `rebase --abort`, ignore its error.
2. **Stage and commit**: when `{dir}/{user}` exists → `add {user}/`; `status --porcelain {user}/` runs unconditionally; when its trimmed output is non-empty → `commit -m {CommitMessage(user, now)}`. A failing `add`, `status`, or `commit` MUST append `Warning: sync {add|status|commit} failed — {git error}` before returning `false`.
3. **Pull target** (`pullTarget`): when the current branch has an upstream — `rev-parse --abbrev-ref --symbolic-full-name @{u}` exits 0 with non-empty output (a local, no-network check) — the pull is `pull --rebase` with no remote/refspec. Otherwise the remote's default branch comes from the `ref: refs/heads/{branch}\tHEAD` line of `ls-remote --symref origin HEAD`, and the pull is `pull --rebase origin {branch}`. A remote advertising no refs (a fresh, empty repo) skips the pull entirely. An `ls-remote` failure is treated as a pull failure.
4. **Pull**: on error → append `Warning: sync pull failed — {err}`, run `rebase --abort` (error ignored), return `false`.
5. **Push**: `push` when the branch has an upstream; `push -u origin HEAD` on BOTH no-upstream paths (default-branch fallback and empty remote) — a branch without an upstream cannot plain-push, and the `-u` sets one, so the first sync to a new or upstream-less repo succeeds and later syncs take the upstream path; on error retry once; on a second error append `Warning: sync push failed after retry — {err}` and return `false`.

Every warning line uses the em dash U+2014. `SyncMetrics` MUST NOT print; the returned lines are the stderr lines for the edge to write.

#### Scenario: Missing user dir still runs status
- **GIVEN** a metrics repo with no `{user}/` directory yet (first sync, or every source unavailable)
- **WHEN** `SyncMetrics` runs
- **THEN** `add` is skipped (no `pathspec did not match` failure) but `status --porcelain {user}/` still runs, and a non-empty result still commits

### Requirement: Commit message, .last-sync, staleness
`sync.CommitMessage(user, now)` MUST return `# {user}: update {now.Local().Format("2006-01-02")}` — the local date, the same basis the day-file labels use — and MUST be the one helper both the live commit and the dry-run preview call. `sync.TouchLastSync(stateDir, now)` MUST create `stateDir` (`0o755`) when missing and write `{stateDir}/.last-sync` (`sync.LastSyncFile`, `0o644`) as the ISO-8601 UTC shape `2006-01-02T15:04:05.000Z` + `"\n"` — the format `config.LastSync` parses. `sync.Stale(stateDir, now)` MUST report true when the file is missing, unreadable, unparseable as RFC 3339 after trimming, or older than `sync.StaleAfter` (`3 * time.Hour`, the auto-sync TTL). `Stale` has no caller; it carries the ported TTL semantics. The status surface that reads `.last-sync` is [status](/config/status.md).

### Requirement: Repo probe and clone drivers
`Exec.IsRepo` MUST answer "is this a git repo" via `git -C <dir> rev-parse --git-dir` with both streams discarded. `Exec.Clone` (interactive init-metrics) MUST inherit the caller's stdin/stdout/stderr with no timeout. `Exec.CloneQuiet` (the auto-clone guard) MUST capture both streams, append `GIT_TERMINAL_PROMPT=0` to the child environment, apply a 30-second deadline (`cloneQuietTimeout = 30 * time.Second`), and wrap `context.DeadlineExceeded` when the deadline fires.

## Design Decisions

### Git as sync transport
**Decision**: the metrics repo syncs through git add/commit/pull --rebase/push over whatever transport the clone's origin uses.
**Why**: no sync server to build or run; pull --rebase avoids merge commits so per-day files stay append-and-overwrite friendly.
**Rejected**: a bespoke sync server (build and operate a service for what git gives free).
*Introduced by*: 260610-srmi

### Upstream-first pull and push targets
**Decision**: the pull step pulls the current branch's upstream when one is configured (`pull --rebase`, no remote/refspec); else the remote's default branch from `ls-remote --symref origin HEAD` (`pull --rebase origin {branch}`); a remote advertising no refs skips the pull. The push is plain `push` on the upstream path only; both no-upstream paths push `-u origin HEAD` (a branch with no upstream cannot plain-push, and the `-u` re-sets it).
**Why**: `git clone` sets an upstream for the default branch, so the common path costs one local `rev-parse` and no extra network call; the fallbacks cover non-`main` or manually set-up repos and the empty-repo first sync. `git pull --rebase origin {branch}` does not configure an upstream, so the fallback must carry its no-upstream state into push selection or the push fails with "current branch has no upstream branch".
**Rejected**: pulling a hard-coded `origin main` (an empty remote or a non-`main` default branch fails every sync); `git pull --rebase origin HEAD` alone (still fails on an empty remote); parsing git's `couldn't find remote ref` stderr to pick a fallback (brittle across git versions and locales); plain `push` on the fallback path (no upstream — the push cannot resolve a destination).
*Introduced by*: 260928-ubws-sync-drop-at-cutover-fixes

### Rebase-abort recovery
**Decision**: before staging, `SyncMetrics` checks for `.git/rebase-merge` or `.git/rebase-apply` and runs `rebase --abort` with its error ignored; a failed pull is also followed by an ignored `rebase --abort`.
**Why**: a sync interrupted mid-rebase leaves the repo un-mergeable; aborting restores a clean state so the next sync proceeds, and ignoring the abort error lets the flow try to continue regardless.
**Rejected**: surfacing the interrupted rebase to the user (a failed sync already fails loudly; silent recovery keeps routine syncs unattended).
*Introduced by*: 260610-srmi

### Git invoked as argv, never through a shell
**Decision**: every git call is `exec.Command("git", "-C", dir, args...)` with argv entries passed as separate strings — no shell subprocess.
**Why**: paths with spaces or quotes pass through as literal argv entries; a shell fork would re-parse quoting.
**Rejected**: `sh -c "git -C ... ..."` (shell quoting bugs on paths with spaces or quotes).
*Introduced by*: 260423-lx0g

### The auto-sync TTL ships caller-less
**Decision**: `sync.Stale` and `sync.StaleAfter` (`3 * time.Hour`) ship with no caller.
**Why**: the plan row names the auto-sync TTL and leaves wiring it to a gate decision; porting the helper preserves the TTL semantics without inventing a call site.
**Rejected**: wiring `Stale` into an auto-sync trigger (a behavior change); dropping it (loses the ported TTL semantics).
*Introduced by*: 260916-lsml-sync-metrics-writer

### TouchLastSync creates the state dir
**Decision**: `os.MkdirAll(stateDir, 0o755)` precedes the `.last-sync` write.
**Why**: the reference behavior crashes with an uncaught error when the state dir is missing, a path no harness case reaches; degrading beats crashing.
**Rejected**: reproducing the crash.
*Introduced by*: 260916-lsml-sync-metrics-writer

### Exec error text reproduces the reference wrapper
**Decision**: `Exec.Run` formats failures as `git -C <dir>... failed: {message}` with the unconditional newline, the maxBuffer overflow text, and `spawn git ENOENT`.
**Why**: parity with the frozen golden corpus (`/harness/golden-corpus.md`, the retired TypeScript implementation's bytes) — the pull/push warning lines surface this text verbatim, so the bytes must match.
**Rejected**: Go-native `exec.ExitError` text (diverges on every surfaced warning line).
*Introduced by*: 260916-lsml-sync-metrics-writer
