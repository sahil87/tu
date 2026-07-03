# Intake: Add Weekly Period

**Change**: 260703-wkly-add-weekly-period
**Created**: 2026-07-03

## Origin

One-shot invocation: `/fab-new wkly` (backlog ID). Backlog entry `[wkly]` from `fab/backlog.md` (2026-07-03):

> Add weekly period to the tu grammar: w/weekly tokens + wh combined shorthand in parseDataArgs (src/node/core/cli.ts); independent of [ccfx]/[gmcp]/[sntl] (merge-conflict adjacency only). Aggregate CLIENT-SIDE from daily entries exactly like monthly — new aggregateWeekly in src/node/core/fetcher.ts mirroring aggregateMonthly, applied post-merge so cache, multi-mode, and watch work for free (do NOT pass through to the ccusage weekly subcommand: that would bypass the multi-mode merge pipeline and the daily fetch cache). LABEL DECISION (intake): Constitution V requires ISO labels (YYYY-MM-DD or YYYY-MM) — recommend the week-START date as the label (pick Monday vs Sunday deliberately; check what ccusage weekly --json emits for alignment); if a distinct weekly form like YYYY-Www is preferred instead, that needs a constitution amendment. WIRING: the period === "monthly" branches are scattered across cli.ts dispatch (dispatchAllHistory/dispatchAllSnapshot/dispatchSingleTool, the *Lines watch variants, fetchToolMerged/fetchToolMergedWithMachines) — prefer threading a period-to-aggregator mapping over adding a third scattered branch if cheap; currentLabel (fetcher.ts) needs a weekly case (start of current week) so snapshot picks the right entry; formatter.ts period headers need a weekly variant. Touch points: FULL_HELP Periods/Combined lines, completions.ts (w/weekly/wh tokens, all three shells), docs/specs/usage.md grammar section, tests (parseDataArgs tokens, aggregateWeekly incl. year-boundary and DST weeks, currentLabel weekly, dispatch snapshot/history for weekly).

Intake-time verification performed against the live codebase and the vendored ccusage binary:

- `ccusage weekly --json` (v20.0.14) emits `"week": "2026-05-31"` — the **week-start date in ISO `YYYY-MM-DD`**, and 2026-05-31 is a **Sunday**. `ccusage weekly --help` confirms `--start-of-week` defaults to `sunday`. → Label decision resolved: Sunday-start, week-start date as label, no constitution amendment needed.
- `src/node/tui/formatter.ts` is fully period-generic: table headings interpolate `(${period})` verbatim (`renderHistory` line 87, `renderTotal` line 202, `renderTotalHistory` line 293, and the CSV/MD title helpers), and the label column header is the literal `"Date"` for every period (already used for monthly `YYYY-MM` labels). → **No formatter change is needed**; the backlog's "formatter.ts period headers need a weekly variant" touch point is outdated and dropped.
- `src/node/tui/watch.ts` contains no period logic — watch flows through the `dispatch*Lines` variants in cli.ts. → Watch works for free once dispatch is wired.

## Why

1. **Pain point**: `tu` supports only daily and monthly granularity. Weekly is the natural budgeting window for AI-assistant spend (sprints, weekly caps, "how much did I burn this week?"), and ccusage itself ships a `weekly` subcommand — tu users currently drop down to raw `ccusage weekly`, which sees only the local machine.
2. **Consequence of not fixing**: users bypass tu for weekly views and lose exactly what tu exists to provide — the multi-machine merged view, the 60s fetch cache, and watch mode. Raw ccusage weekly under-reports for anyone syncing multiple machines.
3. **Why this approach**: aggregate client-side from daily entries exactly like monthly (`aggregateMonthly` precedent). Applied post-merge, the weekly rollup inherits the fetch cache, multi-mode machine merge (including the own-machine max-merge purge correction), `--user` remote views, `--by-machine` breakdowns, and watch mode with zero extra plumbing. Passing through to `ccusage weekly` was rejected in the backlog analysis: it would bypass the multi-mode merge pipeline entirely and skip the daily fetch cache (extra ccusage args bypass caching in `fetchHistory`).

## What Changes

### Grammar: `w`/`weekly` tokens + `wh` shorthand (src/node/core/cli.ts)

`parseDataArgs` gains a third period alongside daily/monthly:

```ts
} else if (arg === "w" || arg === "weekly") {
  period = "weekly";
} else if (arg === "wh") {
  period = "weekly";
  display = "history";
}
```

Resulting grammar: `tu w` (this week's snapshot), `tu weekly`, `tu wh` (weekly history), `tu cc wh` (weekly history, Claude Code), `tu w h` (equivalent long form). The internal `period` value is the string `"weekly"` (consistent with `"daily"`/`"monthly"`).

No conflict with the `-w`/`--watch` flag: `parseGlobalFlags` strips dash-prefixed flags before `parseDataArgs` sees positionals, so bare `w` is unambiguous. Worth one help-text glance but no code hazard.

### Weekly aggregation (src/node/core/fetcher.ts)

New `aggregateWeekly` mirroring `aggregateMonthly` (same accumulate-into-map shape, same field sums, same `localeCompare` sort), keyed by the week-start label:

```ts
// Week label: ISO date of the week's Sunday (aligned with ccusage weekly's
// default --start-of-week sunday, so tu weekly rows match ccusage weekly rows).
// UTC arithmetic on the date-only label — immune to local DST transitions.
function weekLabel(dailyLabel: string): string {
  const d = new Date(`${dailyLabel}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() - d.getUTCDay()); // getUTCDay(): 0 = Sunday
  return d.toISOString().slice(0, 10);
}

export function aggregateWeekly(dailyEntries: UsageEntry[]): UsageEntry[] {
  // identical accumulation loop to aggregateMonthly, keyed by weekLabel(e.label)
}
```

Label decision (resolved at intake): **week-start date, Sunday-start, plain `YYYY-MM-DD`**. Satisfies Constitution V (ISO label) with no amendment; matches `ccusage weekly --json` default output so users can cross-check tu against ccusage row-for-row. Weekly labels are display-only — `writeMetrics` always persists daily entries (aggregation happens post-merge), so nothing weekly is baked into the metrics repo.

### Period-to-aggregator mapping (fetcher.ts + cli.ts wiring)

The backlog prefers threading a mapping over adding a third scattered `period === "weekly"` branch — verified cheap. New export in fetcher.ts:

```ts
export function aggregateForPeriod(period: string, entries: UsageEntry[]): UsageEntry[] {
  if (period === "monthly") return aggregateMonthly(entries);
  if (period === "weekly") return aggregateWeekly(entries);
  return entries; // daily = identity
}
```

Call sites in cli.ts currently patterned `if (period === "monthly") return aggregateMonthly(x); return x;` collapse to `return aggregateForPeriod(period, x);`:

- `fetchToolMerged` — both the remote-user early return and the merged return
- `fetchToolMergedWithMachines` — both branches, including the per-machine map aggregation (aggregate each machine's entries with `aggregateForPeriod` too)
- `dispatchAllHistory` / `dispatchAllHistoryLines` — single-mode monthly aggregation branch generalizes: `if (period !== "daily")` aggregate per tool
- `dispatchAllSnapshot` / `dispatchAllSnapshotLines` — single-mode: the `period === "monthly"` special branch (fetch daily → aggregate → match `currentLabel`) becomes the `period !== "daily"` branch using `aggregateForPeriod` + `currentLabel(period)`; the daily path stays on `fetchAllTotals`
- `dispatchSingleTool` / `dispatchSingleToolLines` — the `if (period === "monthly") entries = aggregateMonthly(entries)` line becomes `entries = aggregateForPeriod(period, entries)`

Snapshot picking (`entries.find((e) => e.label === currentLabel(period))`) is already period-generic at every site — no change beyond `currentLabel` itself.

### `currentLabel` weekly case (fetcher.ts)

```ts
if (period === "weekly") {
  const d = new Date(now);
  d.setDate(d.getDate() - d.getDay()); // back up to Sunday; setDate normalizes month/year underflow
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}
```

Local-time date methods, consistent with the existing daily/monthly cases (which use `getFullYear`/`getMonth`/`getDate`). `tu w` snapshot then picks the entry labeled with the current week's Sunday.

### Help text (cli.ts FULL_HELP)

```
Periods: d/daily (default), w/weekly, m/monthly
Display: (bare) = snapshot, h/history = history
Combined: dh (daily history), wh (weekly history), mh (monthly history)
```

Optionally one example line (e.g., `tu wh` — weekly cost history). `tu help-dump` embeds FULL_HELP verbatim (raw passthrough), so the shll.ai contract picks the new text up automatically — additive, no drift concern. Per Constitution "Output Stability", the added period is additive; ship in a minor version bump.

### Shell completions (src/node/core/completions.ts — all three shells)

- **bash**: `periods="d w m daily weekly monthly"`, `display="h history dh wh mh"`
- **zsh**: `periods=(d w m daily weekly monthly)`, `display=(h history dh wh mh)`
- **fish**: add `complete` lines for `w` ('weekly'), `weekly` ('weekly'), `wh` ('weekly history'), and extend the non-subcommand catch-all list `'d m daily monthly h history dh mh'` → `'d w m daily weekly monthly h history dh wh mh'`

### Spec update (docs/specs/usage.md)

Grammar section: add `w`, `weekly` to the Periods table ("Weekly granularity (aggregated from daily)"), `wh` to the Combined table, and extend the snapshot/heading descriptions that enumerate `(daily|monthly)` to include `weekly`. Note the week-label convention (Sunday-start week-start date) in the data-flow section where `currentLabel` filtering is described.

### Explicitly not changing

- **formatter.ts** — period-generic already (verified; see Origin). Weekly labels are `YYYY-MM-DD` strings, same shape as daily.
- **watch.ts** — no period logic; weekly watch works via `dispatch*Lines`.
- **sync/** — weekly is never persisted; metrics repo stays daily-only.
- **help-dump.ts** — raw FULL_HELP passthrough, additive.
- **No ccusage `weekly` subcommand invocation** — fetch stays daily-only.

## Affected Memory

- `cli/data-pipeline`: (modify) — add the weekly period to the grammar description (w/weekly/wh tokens), the aggregation story (aggregateWeekly + aggregateForPeriod mapping, Sunday-start week-start labels), and the currentLabel weekly case

## Impact

**Code** (all under `src/node/core/` unless noted):

- `cli.ts` — parseDataArgs (+2 token branches), FULL_HELP (2–3 lines), ~8 dispatch/fetch call sites collapse their monthly conditionals onto `aggregateForPeriod`
- `fetcher.ts` — `aggregateWeekly`, `weekLabel` helper, `aggregateForPeriod`, `currentLabel` weekly case
- `completions.ts` — token lists in bash/zsh/fish
- `docs/specs/usage.md` — grammar + data-flow sections

**Tests** (co-located `src/node/core/__tests__/`):

- `cli-parser.test.ts` — parseDataArgs: `w`, `weekly`, `wh`, combined with sources (`cc wh`), invalid combos unchanged
- `fetcher.test.ts` — `aggregateWeekly`: basic grouping, multi-week sort, **year-boundary week** (e.g., days 2026-12-28…2027-01-02 grouping under Sunday 2026-12-27), **DST-transition weeks** (e.g., 2026-03-08, 2026-11-01 — UTC label arithmetic must not skew), `currentLabel("weekly")` incl. month/year underflow (a Sunday in the previous month/year); `aggregateForPeriod` identity/monthly/weekly routing
- dispatch-level coverage mirroring whatever exists for monthly snapshot/history on the merge path

**Adjacency**: independent of `[ccfx]`/`[gmcp]`/`[sntl]` — only merge-conflict adjacency in FULL_HELP, completions.ts, and test files.

**Risk**: low — additive grammar, pure-function aggregation, display-only labels. The widest edit is mechanical (collapsing monthly conditionals onto the mapping), each site individually trivial.

## Open Questions

None — the backlog entry's single flagged decision (week label convention) was resolved at intake by inspecting `ccusage weekly --json` output (see Origin).

## Assumptions

<!-- STATE TRANSFER: This table is the sole continuity mechanism between the intake-stage
     agent and the apply-entry agent (which co-generates plan.md). -->

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Grammar is `w`/`weekly` period tokens + `wh` combined shorthand; internal period value `"weekly"`; client-side aggregation post-merge, no ccusage weekly pass-through | Backlog specifies all of this exactly, including the rejected alternative and why | S:90 R:85 A:95 D:95 |
| 2 | Confident | Week label = week-START date, **Sunday-start**, plain `YYYY-MM-DD` | Backlog says pick deliberately + check ccusage alignment; verified `ccusage weekly --json` emits Sunday week-start ISO dates by default → row-for-row comparability; Constitution V satisfied; label is display-only (never persisted), so cheap to revisit | S:70 R:80 A:85 D:65 |
| 3 | Confident | Thread `aggregateForPeriod(period, entries)` mapping through all monthly-conditional sites instead of adding a third scattered branch | Backlog prefers this "if cheap"; verified cheap — every site is the same two-line pattern; single-mode snapshot branches generalize from `=== "monthly"` to `!== "daily"` | S:75 R:80 A:85 D:75 |
| 4 | Certain | `currentLabel("weekly")` returns start of current week in local time | Backlog states it; local-time date methods match the existing daily/monthly cases | S:85 R:90 A:90 D:85 |
| 5 | Certain | `weekLabel` uses UTC arithmetic on the date-only label string | Labels are timezone-less dates; UTC math is immune to DST transitions (backlog explicitly calls for DST-week tests) | S:65 R:90 A:90 D:85 |
| 6 | Certain | No formatter.ts change (backlog touch point dropped as outdated) | Verified in code: headings interpolate `(${period})`, label column is literally `"Date"` for all periods, monthly already renders non-daily labels through the same path | S:70 R:95 A:95 D:90 |
| 7 | Certain | Weekly snapshot (`tu w`) = entry matching `currentLabel("weekly")`, EMPTY fallback | Mirrors the existing monthly snapshot pattern at every dispatch site verbatim | S:80 R:90 A:95 D:90 |

7 assumptions (5 certain, 2 confident, 0 tentative, 0 unresolved).
