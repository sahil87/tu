# Multi-Machine Sync

## Overview

Multi-machine sync (`src/node/sync/sync.ts`) enables aggregating AI usage costs across multiple machines via a shared git repository. Each machine writes per-day JSONL files to a structured directory hierarchy, then syncs via git commit/pull/push.

## Requirements

- Sync MUST require `mode=multi` in config; single-mode rejects sync operations
- Metrics directory structure MUST follow: `{metricsDir}/{user}/{year}/{machine}/{toolKey}-{YYYY-MM-DD}.jsonl`
- Each JSONL file MUST contain a single `UsageEntry` JSON object
- `writeMetrics()` MUST write local entries to the metrics directory (creates dirs as needed)
- `readRemoteEntries(metricsDir, targetUser, excludeMachine, toolKey)` MUST read entries only from the specified target user's directory; when `excludeMachine` is non-null, that machine's directory is skipped (prevents double-counting with local data)
- `mergeEntries()` MUST sum token counts and costs for entries with matching labels [INFERRED]
- `syncMetrics()` MUST: (1) git add user dir, (2) commit if changes, (3) pull --rebase, (4) push (with one retry)
- `fullSync()` MUST: fetch all tools locally, write metrics, sync via git, touch `.last-sync` timestamp
- `isStale()` MUST return true if `.last-sync` is older than 3 hours or missing
- `--sync` flag MUST trigger sync before data fetch (inline, not auto-triggered)
- Auto-clone MUST be attempted when metricsDir doesn't exist but metricsRepo is configured
- Clone failures MUST write a marker file (`.clone-failed`) with ISO timestamp; retry suppressed for 3 hours
- `init-metrics` MUST clone the metrics repo and clear any clone-failed marker
- When metrics dir is missing and can't be cloned, MUST fall back to single mode with a warning

## Design Decisions

- **Git as sync transport**: Using a git repository avoids building a custom sync server. Commit/pull/push is simple and works over SSH. Rebase on pull avoids merge commits.
- **Per-day JSONL files**: One file per tool per day enables efficient incremental writes and avoids conflicts when multiple machines sync.
- **User-scoped remote reads**: `readRemoteEntries` reads only from the target user's directory (default: config user). The `excludeMachine` parameter (default: config machine) prevents double-counting with local data. When `-u` targets a different user, `excludeMachine` is `null` to include all of that user's machines. When `-u` targets the same user as `config.user`, the default path is used instead (local fetch, cached unless `--fresh`, plus merge), ensuring identical behavior to no `-u` flag.
- **Graceful degradation**: If multi mode is configured but the metrics repo is unavailable, the tool falls back to single mode rather than failing. This ensures the tool always works for local data.
- **Clone-failed marker with cooldown**: Prevents repeated clone attempts on every invocation when the repo is unreachable (e.g., no network). 3-hour cooldown matches the staleness threshold.

## Changelog

| Date | Change |
|------|--------|
| 2026-03-06 | Generated from code analysis |
| 2026-03-06 | Updated file path from `src/sync.ts` to `src/node/sync/sync.ts` |
| 2026-03-07 | readRemoteEntries scoped to single target user; excludeMachine parameter replaces user+machine skip; supports `-u` flag for viewing other users' data |
| 2026-03-07 | Fixed `-u` same-user: falls through to fresh-fetch path instead of reading stale repo data |
