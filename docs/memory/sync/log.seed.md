## 2026-06-10

- **Update** [multi-machine](/sync/multi-machine.md) — Never-shrink guard in `writeMetrics` (skip silently when incoming `totalCost` is lower than the existing day-file's); display path max-merges own-machine snapshots back into the live view (all production reads now pass `excludeMachine = null`); added standalone one-time repair script `scripts/repair-metrics.mjs` (dry-run default, `--write` working-tree only) (srmi)

## 2026-04-23

- **Update** [multi-machine](/sync/multi-machine.md) — Migrated git invocations from `exec("git -C ... ...")` to `execFile("git", [...argv])` — no shell fork, paths with spaces/quotes pass through as literal argv entries (lx0g)

## 2026-03-07

- **Update** [multi-machine](/sync/multi-machine.md) — readRemoteEntries scoped to single target user; excludeMachine parameter replaces user+machine skip; supports `-u` flag for viewing other users' data
- **Update** [multi-machine](/sync/multi-machine.md) — Fixed `-u` same-user: falls through to fresh-fetch path instead of reading stale repo data
- **Update** [multi-machine](/sync/multi-machine.md) — Added `readRemoteEntriesByMachine` returning grouped `Map<string, UsageEntry[]>`; refactored `readRemoteEntries` to delegate and flatten

## 2026-03-06

- **Update** [multi-machine](/sync/multi-machine.md) — Generated from code analysis
- **Update** [multi-machine](/sync/multi-machine.md) — Updated file path from `src/sync.ts` to `src/node/sync/sync.ts`
