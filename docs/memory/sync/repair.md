---
type: memory
description: sync.Repair and cmd/turepair — the one-time repair that restores shrunk day-files to their historical maximum from the metrics repo's git history, its report byte rules, and the ASCII ICU-root comparator in localecmp.go that orders the report
---

# Repair

**Domain**: sync

## Overview

`sync.Repair` (`src/go/internal/sync/repair.go`) is the one-time repair that restores shrunk day-files to their historical maximum from the metrics repo's git history, behind the standalone maintainer binary `cmd/turepair`. The report orderings come from the ASCII ICU-root comparator in `localecmp.go`. The guard that prevents the damage this repairs lives in [day-file-writer](/sync/day-file-writer.md); the reader of the restored files is [metrics-reader](/source/metrics-reader.md).

## Requirements

### Requirement: Repair restores shrunk day-files from git history
`sync.Repair(o RepairOptions, git Runner) (stdout, stderr []string, exit int)` (`src/go/internal/sync/repair.go`) MUST fail with `repair-metrics: repo not found: {repo}` (exit 1) when the repo path is absent, and `repair-metrics: not a git repository: {repo}` (exit 1) when `rev-parse --is-inside-work-tree` fails. It MUST list tracked day-files with `git ls-files -z -- *.jsonl` filtered by the regexp `-\d{4}-\d{2}-\d{2}\.jsonl$`, then run ONE `git log --format=%H%x09%cs --name-only -- *.jsonl` walk to build per-file newest-first commit lists. For each day-file it MUST find the historical max: `git show {sha}:{path}` per commit that touched the file, skipping deleted-at-commit paths and unparseable blobs; the working-tree cost counts missing/unparseable as 0. A file is "shrunk" only when HEAD is below its historical max by more than `centTolerance` (`0.01`). Every git call MUST carry `-c core.quotePath=false` before the verb (passed through `Exec.Run`'s own `-C <dir>` prefix). Costs MUST parse via the same finite check and `jsNumber` coercion the writer's guard uses; a working-tree file that is missing or unparseable counts as 0 (fully shrunk).

#### Scenario: Nothing to repair
- **GIVEN** a repo where every tracked day-file sits at its historical max
- **WHEN** `sync.Repair` runs
- **THEN** the report ends with `Nothing to repair — every day-file is at its historical maximum.` and exit 0

#### Scenario: Within a cent is not shrunk
- **GIVEN** a day-file whose working-tree cost is within `centTolerance` (`0.01`) of its historical max
- **WHEN** `sync.Repair` runs
- **THEN** the file is not reported as shrunk — float noise within a cent is not worth touching

### Requirement: The report and its two orderings are fixed
The report MUST open with `repair-metrics: scanned {repo}`, then `  {n} tracked day-files, {m} commits touching *.jsonl`, a blank line, then either `Nothing to repair — every day-file is at its historical maximum.` or the shrunk table: `Shrunk day-files ({k}):`, a blank line, the header `  {FILE padded}  {CURRENT right-aligned width 10}  {MAX width 10}  {DELTA width 10}  MAX COMMIT`, one row per shrunk file (`money = "$" + render.FixedHalfUp(v, 2)`, `MAX COMMIT` as `{sha[:7]} ({date})`), then a blank line, `Per-user totals:`, rows `  {user}: +{money} across {n} file(s)`, and finally `Grand total: +{money} across {k} file(s)`. In dry-run (default) the report MUST end with `Dry run — nothing modified. Re-run with --write to restore shrunk files.`; under `--write` it MUST end with `Restored {k} file(s) in the working tree.` / `Review with: git -C {repo} diff` / `Then commit and push manually.` The shrunk table MUST sort by `localeCompare` (the ASCII ICU-root comparator); per-user rows MUST sort by `userEntryCompare`.

### Requirement: --write restores the full historical blob, working tree only
Under `--write`, `sync.Repair` MUST write each shrunk file's full historical-max blob byte-exact into the working tree — never a patch of the cost field — and MUST NOT commit or push (review, commit, and push are left to the user). A write failure during restore MUST be ignored. Dry-run MUST be the default; `--write` flips the mode.

#### Scenario: --write restores the full blob
- **GIVEN** a shrunk day-file whose historical-max commit holds the full snapshot
- **WHEN** `sync.Repair` runs with `--write`
- **THEN** the working-tree file becomes that commit's full blob byte-exact, no commit or push is made, and the report ends with the restore lines

### Requirement: The report orderings come from localecmp.go
`localeCompare` (`src/go/internal/sync/localecmp.go`) MUST order the shrunk table by the ICU root collation restricted to the ASCII repertoire day-file paths contain: punctuation (`_` < `-` < `.` < `/`) < digits by value < letters case-insensitively, with no numeric collation (`"a1" < "a9"` holds, but `"10" < "9"` because `'1' < '9'`); strings equal at the primary level MUST fall to the tertiary level where lowercase sorts before uppercase at the first case difference; a shorter primary-level prefix sorts first; any byte outside the repertoire falls back to plain byte order at that position (a documented limit). `userEntryCompare` MUST order per-user rows as byte order of the decorated string `"{user},[object Object]"` — the decoration matters only when one name is a prefix of another whose next byte sorts below `,`.

#### Scenario: ICU order beats byte order
- **GIVEN** the paths `sahil/2026/Sahils-Mac-mini.local/…` and `sahil/2026/dev-ws-s01/…`
- **WHEN** the shrunk table sorts
- **THEN** `Sahils-Mac-mini.local/…` sorts AFTER `dev-ws-s01/…` (`'d' < 's'` case-insensitively) — plain byte order would put `Sahils` first

### Requirement: turepair is a standalone maintainer binary
`cmd/turepair` (built as `bin/turepair`) MUST be a standalone maintainer binary outside the `tu` grammar, with `main` the only writer and the algorithm in `sync.Repair`. Arg parsing MUST match: `--repo <path>` (default `~/.tu/metrics_repo`, home-expanded then resolved absolute so the printed path matches), `--write` flips the mode, `--repo` without a value fails with `repair-metrics: --repo requires a path`, any other argument fails with `repair-metrics: unknown argument: {arg}`, and both failures append the usage line `Usage: node scripts/repair-metrics.mjs [--repo <path>] [--write]` after a newline — verbatim, node spelling included, so the two implementations' outputs diff clean (a follow-up owns changing it). Every failure line MUST carry the `repair-metrics: ` prefix via `sync.FailLines`. The binary MUST NOT be part of the `tu` grammar.

## Design Decisions

### One-time repair from git history, manual and working-tree only
**Decision**: `sync.Repair` restores each shrunk day-file by writing back the full blob of the commit where its cost peaked — never a patch of just the cost field — into the working tree only; review, commit, and push are left to the user, and dry-run is the default.
**Why**: nothing was ever deleted from the metrics repo's history, only overwritten, so every shrunk day-file is recoverable losslessly; restoring the full blob keeps each day-file an atomic snapshot that was real at some point in time; leaving commit/push to the user keeps the repair deliberate.
**Rejected**: patching just the cost field (breaks the atomic-snapshot invariant); auto-committing (a repair tool must not publish unreviewed).
*Introduced by*: 260610-srmi

### turepair is a standalone maintainer binary
**Decision**: the repair ships as `cmd/turepair` over `sync.Repair`, built by the repo's build, never part of the `tu` grammar.
**Why**: the CLI grammar and `--help` are frozen; a maintainer tool run from a checkout needs no subcommand slot, and the `tudiff` precedent exists for repo-run maintainer binaries.
**Rejected**: a hidden `tu repair-metrics` subcommand (a grammar change); no binary at all (the repair must outlive any single scripting runtime).
*Introduced by*: 260916-lsml-sync-metrics-writer

### The report orderings are ported comparators, not Go defaults
**Decision**: `localeCompare` and `userEntryCompare` in `localecmp.go` reproduce the two JavaScript orderings the report relies on — ICU-root primary/tertiary for paths, decorated-string byte order for per-user rows.
**Why**: parity with the frozen `src/node/` oracle (until plan row Z1) — the shrunk table and per-user rows must diff clean, and Go's `strings.Compare` would order `Sahils-…` before `dev-ws-…` where the ICU primary does the reverse.
**Rejected**: plain byte order (simpler but changes the report's row order).
*Introduced by*: 260916-lsml-sync-metrics-writer
