# Intake: Backlog Unfreeze (Go-port row Z2)

**Change**: 260925-9qqq-backlog-unfreeze
**Created**: 2026-09-25

## Origin

One-shot `/fab-new` invocation for the last open row of the Go-port plan:

> Context: fab/plans/sahil/26-09-15-go-port.md, row Z2. Scope: re-triage fab/backlog.md — rows written against src/node/core/*.ts paths get re-pointed at the Go packages under src/go/internal/ (per the plan's Target architecture section mapping); rows already shipped by the Go port change breakdown get closed with the PR that shipped them.

Plan row Z2 (`backlog-unfreeze`, depends on Z1, size —): *"Re-triage `fab/backlog.md`: rows written against `src/node/core/*.ts` paths get re-pointed at Go packages."* Decision D4 froze `src/node/` features for the port and said the open backlog rows "become Go-only rows after Phase 4". Z1 (`260925-6wpm-remove-src-node`, [#101](https://github.com/sahil87/tu/pull/101)) deleted `src/node/`, so every Node path in the backlog is now dangling.

The intake was written after a per-row gap analysis against `src/go/` and git history (all findings recorded under **What Changes**). Key finding, which shifts the framing in the scope: four of the "open" rows (`ccfx`, `gmcp`, `sntl`, `wkly`) were shipped in the **Node era** (PRs #39–#42, 2026-07-03) before the port even started; their checkboxes were simply stale, exactly like `v76l`'s. They close against the PR that shipped them, with the Go PR that carried the behavior across noted beside it.

## Why

`fab/backlog.md` is the intake source for `/fab-new <backlog-id>` (`_intake.md` Step 0). Today seven of its nine rows are wrong in one of two ways:

1. **Dangling paths.** Rows cite `src/node/core/fetcher.ts`, `cli.ts`, `completions.ts`, `formatter.ts`, `config.ts`, `sync.ts`, `scripts/repair-metrics.mjs`, `config.test.ts`, `cli-sync.test.ts`, `tui/__tests__/rain.test.ts`. None of these files exist after #101. An agent picking such a row up via `/fab-new 4d46` would spend its first steps discovering the paths are gone and re-deriving the Go package from the plan's Target-architecture table.
2. **Stale checkboxes.** Six rows describe work that is already done — four shipped in July (Node) and carried into Go by V1/B2, one operational task executed on 2026-06-10, one made moot by Z1. Left open, they invite duplicate work: e.g. re-implementing `--since/--until`, which `src/go/internal/command/parse.go` already parses.

If not fixed, the backlog stays frozen in practice (D4's freeze was meant to lift at Z2), and the plan's own "Out of scope" line — *"New features (gemini/copilot column fit, weekly label changes, etc.) — after Z2"* — has no accurate queue to resume from.

Why re-triage in place rather than rewrite the file: the backlog's rows are a historical record (each carries its original date and reasoning, and `v76l` shows the established close convention — flip to `[x]`, append a bold **DONE (date)** note). Keeping that shape keeps `/fab-new <id>` and `/fab-archive`'s backlog marking working unchanged.

## What Changes

Two files. No source code, no tests, no memory.

### `fab/backlog.md` — per-row triage

Evidence gathered 2026-09-25 on this worktree (HEAD `b04f6bb`, post-#101).

| Row | Verdict | Evidence | Action |
|-----|---------|----------|--------|
| `[x] [v76l]` help-dump | already closed | — | **Untouched** (closed history; its `scripts/help-dump.mjs` mention is part of the record) |
| `[ ]` 2026-06-03 hermetic / CI-stable test suite | **Close — moot after Z1** | The tests it names (`config.test.ts`, `cli-sync.test.ts`, `tui/__tests__/rain.test.ts`) were deleted with `src/node/` in #101. The Go suite is hermetic against the same leak: `config.Load` takes an injected env seam (`src/go/internal/config/config.go:160-165`), `harness/diff.go:124-145` pins `TU_METRICS_REPO`/`NO_COLOR` per case, and `TU_METRICS_REPO=… NO_COLOR=1 go test ./... -count=1` passed 23/23 packages on 2026-09-25. | Flip to `[x]`; append **DONE (2026-09-25)** note: made moot by #101, Go suite verified green with the leaked env |
| `[ ] [9ceu]` one-time metrics-repo repair | **Close — executed** | Metrics repo commit `c3b64c7` (2026-06-10): *"repair: restore day-files shrunk by pre-0.5.0 writeMetrics purge bug (tu#34, +$10511.59 across 172 files)"*. The script it names (`node scripts/repair-metrics.mjs`) is now the Go maintainer binary `src/go/cmd/turepair` over `sync.Repair` (#94; memory `sync/repair`). | Flip to `[x]`; append **DONE (2026-06-10)** note with the commit and the `turepair` re-point (`bin/turepair [--repo <path>] [--write]`) for anyone re-running it |
| `[ ] [ccfx]` cc source mapping for ccusage v20 | **Close — shipped** | PR [#39](https://github.com/sahil87/tu/pull/39) (change `260703-ccfx`, review-pr done 2026-07-03). Go carries it: `src/go/internal/source/ccusage/registry.go:20` — `"cc": {prefixArgs: ["claude"], labelKey: "date"}` (V1 #84, moved to the registry file in G1 rework #88). | Flip to `[x]`; **DONE (2026-07-03)**: #39; Go carrier #84/#88; checkbox was stale |
| `[ ] [gmcp]` gemini + copilot sources | **Close — shipped** | PR [#42](https://github.com/sahil87/tu/pull/42) (change `260703-gmcp`). Go: `src/go/internal/fact/tool.go` six-tool registry (`cc, codex, oc, gemini, copilot, kimi`) with aliases `gem`/`cop` in the command grammar (V1 #84). | Flip to `[x]`; **DONE (2026-07-03)**: #42; Go carrier #84 |
| `[ ] [sntl]` `--since`/`--until` + `-j` | **Close — shipped** | PR [#40](https://github.com/sahil87/tu/pull/40) (change `260703-sntl`). Go: `src/go/internal/command/parse.go` cases `--since`/`-s` (l.227), `--until` (l.233), `--json`/`-j` (l.175), and the since>until error (l.308); history-only guard notice in `guards.go:15` (B2 #87). | Flip to `[x]`; **DONE (2026-07-03)**: #40; Go carrier #87 |
| `[ ] [wkly]` weekly period | **Close — shipped** | PR [#41](https://github.com/sahil87/tu/pull/41) (change `260703-wkly`). Go: weekly roll-up in `src/go/internal/query/query.go`, `w`/`weekly`/`wh` tokens in `command/parse.go` (B2 #87, Sunday alignment). | Flip to `[x]`; **DONE (2026-07-03)**: #41; Go carrier #87 |
| `[ ] [s3kd]` delete stale `SHLLAI_TOKEN` secret | **Keep** | Repo-admin action; cites no source path. | **Untouched** |
| `[ ] [4d46]` sync is manual-only, `isStale()` dead, `auto_sync` a no-op | **Keep — re-point** | Still true in Go. `sync.Stale`/`sync.StaleAfter` (`src/go/internal/sync/state.go:24,46`) have no non-test caller (memory `sync/git-flow` DD *"The auto-sync TTL ships caller-less"* records this as a deliberate port decision, wiring left to a gate). `tu status` prints `Auto-sync:   on` via `autoSyncWord` (`src/go/internal/config/status.go:105-109`) from `config.AutoSync` (`config.go:199`); `init-conf` writes the `auto_sync` key with the comment *"no longer auto-triggers"* (`config/setup.go:26`). | Rewrite the Node references in place: `isStale() in sync.ts` → `sync.Stale` in `src/go/internal/sync/state.go`; `tu status` → `src/go/internal/config/status.go`; the "check git log on sync.ts" pointer → `git log -S isStale` (the Node file is gone from the tree; the history still resolves, `df41538` #72 is the Node commit that last touched the status line). Append **Re-pointed (2026-09-25, Z2)** note. The restore-vs-remove decision stays open — it is that row's own intake question, not Z2's |

Close-note shape (mirrors the existing `v76l` row verbatim): the checkbox flips to `[x]`, the row text is kept, and a trailing `**DONE (YYYY-MM-DD)**: …` sentence is appended naming the PR (or commit) that shipped it, the Go PR that carried it across where applicable, and "checkbox was stale" where that is the reason. The `[9ceu]` row's two `\ `-prefixed continuation lines stay as they are.

Re-point shape (`[4d46]` only): edit the path references inline so the row reads correctly on its own, then append `**Re-pointed (2026-09-25, Z2)**: Node paths replaced after #101; …`. No change to the row's ID, date, or the decision it asks for.

Resulting backlog: 9 rows — 7 `[x]`, 2 `[ ]` (`s3kd`, `4d46`).

### `fab/plans/sahil/26-09-15-go-port.md` — close row Z2

Following the convention every other row used:

- Row Z2, **PR** column: `260925-9qqq-backlog-unfreeze` plus the PR link once it exists (the link can be added at ship, after `/git-pr` opens the PR — Z1's row was filled the same way); **Status** column: `**landed**` with a one-line summary (*6 closed, 1 re-pointed, 2 untouched*).
- The **Status (2026-09-25)** paragraph at the top: replace *"Remaining: Z2 (backlog re-triage) — on Sahil's explicit go."* with a sentence saying Z2 landed and the plan is complete.
- D4's sentence *"The open backlog rows (`gmcp` etc.) become Go-only rows after Phase 4"* is left as written — it is the decision record, and the Z2 row's status carries the outcome.

## Affected Memory

None. `fab/backlog.md` and `fab/plans/` are planning artifacts, not memory or specs; no documented behavior changes. The `[4d46]` re-point cites the existing `sync/git-flow` design decision rather than altering it.

## Impact

- `fab/backlog.md` — 7 rows edited (6 close notes, 1 re-point), 2 untouched.
- `fab/plans/sahil/26-09-15-go-port.md` — row Z2 and the top status paragraph.
- No `src/go/` changes, no tests, no goldens, no harness. `fab/` is in `true_impact_exclude`, so the true-impact breakdown is zero.
- Downstream: `/fab-new 4d46` now resolves to a row whose paths exist; the plan's "Out of scope — after Z2" queue can resume.

## Open Questions

None. Every row's verdict is backed by a file, PR, or commit cited above.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Closed rows keep their text and get `[x]` plus a trailing bold **DONE (date)** note, never deleted | The `v76l` row already uses exactly this shape; `/fab-new <id>` and `/fab-archive` backlog marking depend on rows staying in place | S:85 R:95 A:90 D:90 |
| 2 | Certain | `ccfx`/`gmcp`/`sntl`/`wkly` close against the Node-era PR that shipped them (#39–#42), with the Go carrier PR (#84/#87/#88) noted beside it | The scope said "closed with the PR that shipped them"; git history shows these shipped pre-port, so the shipping PR is the July one — the Go PR only carried the behavior across | S:70 R:90 A:85 D:75 |
| 3 | Certain | The unnamed hermetic-test row closes with #101 plus a note that the Go suite was verified green with `TU_METRICS_REPO` and `NO_COLOR` exported | Its subject tests no longer exist; the leak it describes was checked against the Go suite on 2026-09-25 (23/23 packages ok), so nothing carries forward | S:65 R:90 A:85 D:80 |
| 4 | Confident | `[9ceu]` closes as executed on the strength of metrics-repo commit `c3b64c7` (2026-06-10); its FINAL CHECK (`tu dh --fresh` value, guard holding) is not re-run at Z2 | The commit is the row's own "commit and push" step and carries the restored totals; the guard is covered by the Go never-shrink tests and the live harness since B6 | S:60 R:85 A:60 D:70 |
| 5 | Certain | Already-closed `[x]` rows (`v76l`) are not rewritten even though they mention deleted Node files | They are history; rewriting a closed record adds nothing to intake and risks losing the reasoning it carries | S:80 R:95 A:90 D:90 |
| 6 | Certain | `[s3kd]` is left untouched | It is a repo-admin action with no source path — nothing to re-point, nothing shipped | S:75 R:95 A:90 D:90 |
| 7 | Certain | `[4d46]` is re-pointed in place and its restore-vs-remove decision stays open | The Go code preserves the exact condition the row describes (caller-less `sync.Stale`, `Auto-sync: on`); deciding it is that row's own intake, not Z2's scope | S:80 R:90 A:85 D:80 |
| 8 | Confident | The plan file's Z2 row and top status paragraph are updated in this same change | The scope names only the backlog, but every previous row (P0…Z1) recorded its folder/PR/status in the plan in its own PR; the PR link is filled at ship when the number exists | S:55 R:95 A:80 D:80 |
| 9 | Confident | `change_type` is `chore` | Planning-artifact housekeeping: no behavior, no docs/memory, no CI; the keyword inference cannot see this and would default to `feat` | S:60 R:95 A:80 D:70 |

9 assumptions (6 certain, 3 confident, 0 tentative, 0 unresolved).
