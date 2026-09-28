# Intake: Sync Drop-at-Cutover Fixes

**Change**: 260928-ubws-sync-drop-at-cutover-fixes
**Created**: 2026-09-28

## Origin

> Sync drop-at-cutover fixes (DC-18, DC-21, DC-22, DC-23 in docs/specs/usage.md § Drop at cutover; backlog [o89t]).

Conversational. In a `/fab-discuss` session the user asked to see the 24 `[DECIDE: keep|drop]` markers in `docs/specs/usage.md` § Drop at cutover, the agent proposed a keep/drop lean per marker, and the user said "wherever you have strong recommendations — go ahead with those". The strong-recommendation set was split into three changes: this one (sync: DC-18, DC-21, DC-22, DC-23), an output-shapes/headings change (DC-01, 07, 08, 13, 14, 15, 24 + recording the keeps DC-05, 06, 11, 16), and a CLI/watch change (DC-04, 09, 17). DC-02, 03, 10, 12, 19, 20 stay open. The `fab/backlog.md` [o89t] triage note recording this split is already in the working tree and ships with this change.

## Why

The Go binary reproduced the Node version byte-for-byte, including four sync behaviors that look accidental. Go is now the only codebase, so each "drop" is an ordinary behavior change:

1. **DC-18** — `tu sync` hard-codes `git pull --rebase origin main`. A metrics repo whose default branch is `master` (or anything else), or a freshly created empty repo, never syncs (`fatal: couldn't find remote ref main`). On top of that, `git add`/`status`/`commit` failures are swallowed: the user sees only `Error: sync failed — check network and remote config.` with no git error (e.g. a missing git identity is undiagnosable). This is a real bug, not a style issue.
2. **DC-21** — the commit message `# {user}: update {date}` uses the UTC date, while day-files and every displayed label use local dates; a sync in the evening west of UTC writes `cc-2026-09-16.jsonl` under a commit titled `update 2026-09-17`, or vice versa.
3. **DC-22** — the dry-run report prints costs as bare `$` + 2 decimals (`$99999.00`) while every table uses thousands separators.
4. **DC-23** — the dry-run treats an identical rewrite (equal cost, same bytes) as `(update: $X → $X)` and counts it toward `Would commit`, so in steady state the preview predicts a commit that a live sync never makes (git sees no change). A preview that disagrees with the real run defeats its purpose (toolkit principle №5, visible mutation boundaries).

If left alone: DC-18 keeps blocking any non-`main` or empty metrics repo; the others keep the dry-run and git history subtly misleading.

## What Changes

### DC-18 — pull target, empty remote, and surfaced git errors (`src/go/internal/sync/flow.go`)

Today: `pullArgs = []string{"pull", "--rebase", "origin", "main"}`; any error from `git add {user}/`, `git status --porcelain {user}/`, or `git commit` returns `false` with no line.

New behavior of `SyncMetrics`:

- **Pull target**: if the current branch has an upstream (`git rev-parse --abbrev-ref --symbolic-full-name @{u}` succeeds — a local, no-network check), run `git pull --rebase` with no remote/refspec (git uses the upstream). Otherwise resolve the remote's default branch with `git ls-remote --symref origin HEAD` and run `git pull --rebase origin {branch}`.
- **Empty remote**: when the remote has no refs (`git ls-remote --symref origin HEAD` prints nothing / no `ref:` line and `git ls-remote --heads origin` is empty), skip the pull and push with `git push -u origin HEAD` so the first sync to a new repo succeeds.
- **Error surfacing**: add/status/commit failures append a warning line before returning false, in the same shape as the existing pull/push lines: `Warning: sync commit failed — {git error}` (and `sync add failed`, `sync status failed`). The existing `Warning: sync pull failed — …` and `Warning: sync push failed after retry — …` lines are unchanged. The edge's closing `Error: sync failed — check network and remote config.` (cmd/tu/main.go) is unchanged.
- The rebase-abort-before-retry and push-retry-once behavior is unchanged.

The dry-run's `Would commit: "…", then pull --rebase origin main, then push` / `Would commit: nothing (no changes), then pull --rebase origin main, then push` (`internal/sync/report.go`) drop the hard-coded branch: `then pull --rebase, then push`. The dry-run stays read-only and does not probe the remote (it must not run `ls-remote`).

### DC-21 — local date in the commit message (`src/go/internal/sync/state.go`)

`CommitMessage` formats `now` in local time (`now.Local().Format("2006-01-02")` or the same local-date basis the day-file writer uses for labels) instead of `now.UTC()`. It remains the one place both the live commit and the dry-run preview derive the message from. `.last-sync` stays UTC ISO (it is a timestamp, not a label — not part of DC-21).

### DC-22 — thousands separators in the dry-run report (`src/go/internal/sync/report.go`)

`Report.Format`'s `fmtCost` switches from `"$" + render.FixedHalfUp(x, 2)` to the shared cost formatter every table uses (render's `FormatCost` or equivalent, `$1,234.56`). This applies to **both** the `Would write` lines and the `Would skip` lines — note the code comment says the TS used the same bare fmt for both blocks, so the spec's claim that the write lines already carry separators is inaccurate; after this change both do.

### DC-23 — identical rewrites are not updates (`src/go/internal/sync/writer.go`, `flow.go`, `report.go`)

Add a third decision `ActionUnchanged`: `Write` compares the bytes it would write (`json.Marshal(DayFile)` + `"\n"`) against the existing file content; identical bytes ⇒ `ActionUnchanged`. Live mode skips the (no-op) file write for an unchanged decision; dry-run `Format` omits unchanged decisions from the `Would write` list, and `FullSync`'s `anyWrite` counts only `ActionWrite`, so `WouldCommit` is true only when bytes would change or the user dir is already dirty. An equal-cost write whose bytes differ (e.g. token fields changed) is still an `ActionWrite` `(update: …)`. The never-shrink guard (`incoming < existing` ⇒ skip) is unchanged and is evaluated first.

### Spec, ledger, goldens

- `docs/specs/usage.md`: rewrite the lines carrying `(DC-18)`, `(DC-21)`, `(DC-22)`, `(DC-23)` (§ Multi-machine sync steps ~L406, output ~L409, dry-run ~L425) to describe the new behavior and drop the `(DC-NN)` tag; in § Drop at cutover mark each of the four entries resolved as drop (e.g. `[DECIDED: drop — {release/change}]`) rather than deleting them (IDs are stable).
- `docs/specs/layouts.md`: update the sync success/failure and dry-run mockups (~L869, ~L882–891).
- Regenerate goldens: `go test ./internal/sync/... -update` where goldens exist, and the harness corpus via `bin/tudiff --update` (run and live) — the dry-run cases, sync fakegit rule sets, and the live real-git steps all change (new `rev-parse @{u}` call, new argv, new message date, new report text). `harness/expected-diffs.json` is currently empty, so nothing to remove there.
- `fab/backlog.md` [o89t] triage note (already edited in the working tree) ships with this change.

## Affected Memory

- `sync/git-flow`: (modify) pull-target resolution, empty-remote first push, add/status/commit warning lines, local-date commit message
- `sync/dry-run-report`: (modify) thousands-separated costs, branch-less pull wording, unchanged decisions omitted from writes and from Would commit
- `sync/day-file-writer`: (modify) the new `ActionUnchanged` decision and the skipped no-op write
- `harness/live`: (modify) if the live step sequence or its recorded git argv changes
- `harness/replayers`: (modify) only if fakegit rule-set docs name the old pull argv

## Impact

- Code: `src/go/internal/sync/{flow,state,report,writer}.go` and their `_test.go`; `cmd/tu/main.go` only if a line changes there (it should not).
- Harness: `harness/golden/` run and live goldens, possibly `harness/matrix.json` fakegit rule sets (`TUDIFF_GIT_SCRIPT`) that match the literal `pull --rebase origin main` argv.
- Specs: `docs/specs/usage.md`, `docs/specs/layouts.md`.
- Output Stability: user-visible text changes (dry-run report, warning lines, commit message) ⇒ next release is a minor bump. No release is cut in this change.

## Open Questions

- None blocking. The exact wording of the new add/status warning lines follows the existing `Warning: sync {step} failed — {err}` pattern.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Drop DC-18, DC-21, DC-22, DC-23 (change behavior, not keep) | Discussed — user said go ahead with the agent's strong recommendations; these four were listed as drop | S:95 R:80 A:95 D:95 |
| 2 | Confident | Pull the upstream when configured, else the remote default via `ls-remote --symref origin HEAD` | Fixes non-main and manually-set-up repos; the common path adds only a local rev-parse, no extra network call | S:80 R:75 A:80 D:75 |
| 3 | Confident | Empty remote ⇒ skip pull, `git push -u origin HEAD` | The DC-18 ledger names the empty repo as a failing case; pushing with -u makes later syncs take the upstream path | S:80 R:75 A:80 D:75 |
| 4 | Confident | add/status/commit failures get `Warning: sync {step} failed — {err}` lines; edge error line unchanged | Mirrors the existing pull/push warning shape; the edge line keeps its exit-1 contract | S:85 R:85 A:85 D:80 |
| 5 | Confident | Dry-run wording becomes `then pull --rebase, then push`, with no remote probe | Dry-run must stay local and read-only (git half computed locally per spec) | S:80 R:85 A:85 D:75 |
| 6 | Certain | Commit message uses the local date; `.last-sync` stays UTC ISO | DC-21 is about the label basis; `.last-sync` is a timestamp parsed by config | S:90 R:90 A:90 D:90 |
| 7 | Confident | Both Would write and Would skip lines use the shared thousands-separated cost formatter | Code shows both blocks use the bare fmt today; consistency with tables is the point of DC-22 | S:85 R:90 A:85 D:85 |
| 8 | Confident | New `ActionUnchanged` decided by byte equality; live skips the no-op write; dry-run omits it and it does not count toward Would commit | Byte equality is exactly what git sees, so the preview matches the live commit decision; equal cost with different bytes stays an update | S:85 R:80 A:85 D:80 |
| 9 | Certain | Ledger entries are marked resolved, not deleted; spec lines drop their (DC-NN) tag | Ledger says IDs are stable; backlog [o89t] says rewrite the spec line carrying the same (DC-NN) | S:85 R:90 A:85 D:85 |
| 10 | Certain | No release in this change; next release is a minor bump | Constitution Output Stability; releases are separate `release:` commits | S:90 R:95 A:90 D:95 |

10 assumptions (4 certain, 6 confident, 0 tentative, 0 unresolved).
