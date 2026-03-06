# Multi-Machine Sync

## Overview

Multi-machine sync (`src/node/sync/sync.ts`) enables aggregating AI usage costs across multiple machines via a shared git repository. Each machine writes per-day JSONL files to a structured directory hierarchy, then syncs via git commit/pull/push.

## Requirements

- Sync MUST require `mode=multi` in config; single-mode rejects sync operations
- Metrics directory structure MUST follow: `{metricsDir}/{user}/{year}/{machine}/{toolKey}-{YYYY-MM-DD}.jsonl`
- Each JSONL file MUST contain a single `UsageEntry` JSON object
- `writeMetrics()` MUST write local entries to the metrics directory (creates dirs as needed)
- `readRemoteEntries()` MUST read entries from all user/machine combinations EXCEPT the local user+machine pair (to avoid double-counting)
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
- **Exclude self from remote reads**: `readRemoteEntries` skips the local user+machine to prevent double-counting when merging local + remote data.
- **Graceful degradation**: If multi mode is configured but the metrics repo is unavailable, the tool falls back to single mode rather than failing. This ensures the tool always works for local data.
- **Clone-failed marker with cooldown**: Prevents repeated clone attempts on every invocation when the repo is unreachable (e.g., no network). 3-hour cooldown matches the staleness threshold.

## Changelog

| Date | Change |
|------|--------|
| 2026-03-06 | Generated from code analysis |
| 2026-03-06 | Updated file path from `src/sync.ts` to `src/node/sync/sync.ts` |
