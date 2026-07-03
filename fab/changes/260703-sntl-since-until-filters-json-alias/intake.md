# Intake: Since/Until Date Filters + -j JSON Alias

**Change**: 260703-sntl-since-until-filters-json-alias
**Created**: 2026-07-03

## Origin

Backlog item `[sntl]` (2026-07-03), invoked one-shot via `/fab-new sntl`:

> Add --since/-s and --until date filters plus -j short alias for --json (ccusage option alignment; independent of [ccfx]/[gmcp]/[wkly] — only merge-conflict adjacency in FULL_HELP/completions/tests). PARSING: extend parseGlobalFlags/GlobalFlags in src/node/core/cli.ts — --since/-s <date> and --until <date> accept YYYY-MM-DD or YYYYMMDD and normalize to ISO (mirror ccusage grammar); --until stays LONG-ONLY because -u is already tu --user (do NOT break existing users); add -j in the same flag scan as --json. DESIGN: implement as a CLIENT-SIDE filter on merged entries (ISO label string compare — lexicographic order works), NOT as ccusage pass-through: extra ccusage args currently bypass the 60s fetch cache in fetchHistory (fetcher.ts), and pass-through would miss multi-mode remote entries entirely; filter daily entries BEFORE aggregateMonthly so monthly rollups reflect the window. SEMANTICS TO DECIDE AT INTAKE: (a) history display — filter entries (the core use case); (b) snapshot display — either warn-and-ignore on stderr (precedent: -u in single mode) or apply when the current label falls outside the window; (c) watch mode — accept and filter each poll; (d) validation — error when since > until. Touch points: FULL_HELP Flags block, completions.ts (all three shells), cli tests (flag parsing, existing incompat matrix unchanged, filter behavior on the multi-mode merge path). help-dump/shll.ai contract picks up new help text automatically (raw FULL_HELP passthrough — additive change, no drift concern).

The backlog entry pre-resolved semantics (a), (c), and (d). The one open decision, (b) snapshot semantics, was asked interactively at intake: **the user chose warn-and-ignore** over apply-the-filter.

## Why

1. **Pain point**: tu always renders the full date range ccusage (or the metrics repo) returns. There is no way to scope output to a window — "what did June cost me?" requires eyeballing or exporting to another tool. ccusage itself supports `--since`/`--until`, so users coming from ccusage expect the option and find it missing.
2. **Consequence of not fixing**: history tables keep growing (multi-mode metrics repos accumulate indefinitely), making windows the primary way users will want to read them; every downstream consumer (`--json`/`--csv`/`--md` piping) has to re-implement date filtering.
3. **Why this approach**: a client-side filter on merged entries (rather than passing `--since`/`--until` through to ccusage) because (i) extra ccusage args bypass the 60s fetch cache in `fetchHistory` (`fetcher.ts` caches only vanilla calls — `extraArgs.length === 0`), (ii) pass-through cannot filter multi-mode remote entries read from the metrics repo (they never go through ccusage), and (iii) labels are already normalized ISO strings, so lexicographic compare is a correct total order. `-j` is pure ergonomics riding along (ccusage alignment; `-j` is unclaimed).

Short-flag audit (verified against `parseGlobalFlags`): taken short flags are `-f -w -i -u -v -V -h`. `-s` and `-j` are free. `-u` is `--user`, which is why **`--until` stays long-only**.

## What Changes

### 1. Flag parsing — `src/node/core/cli.ts`

Extend `GlobalFlags` and `parseGlobalFlags`:

```ts
export interface GlobalFlags {
  // ...existing fields...
  sinceFlag: string | undefined;  // normalized ISO YYYY-MM-DD
  untilFlag: string | undefined;  // normalized ISO YYYY-MM-DD
}
```

- `-j` is added to the `--json` detection (`rawArgs.includes("--json") || rawArgs.includes("-j")`) and to the skip list in the filtered-args loop. It participates in the existing incompatibility matrix through `jsonFlag` — error message wording keeps the canonical `--json` name, unchanged.
- `--since`/`-s <date>` and `--until <date>` are value-taking flags parsed in the same loop as `--interval`/`--user` (space-separated value only — no `=` form exists anywhere in the parser today).
- Accepted date shapes: `YYYY-MM-DD` or `YYYYMMDD` (regex `^\d{4}-?\d{2}-?\d{2}$` with consistent dashes — i.e. `^\d{4}-\d{2}-\d{2}$` or `^\d{8}$`), normalized to ISO `YYYY-MM-DD`. Shape validation only; no calendar validity check (an impossible date like `2026-13-01` simply produces an empty window — lexicographic compare stays total).
- Errors (stderr + exit 1, mirroring the `--interval requires a numeric value` pattern):
  - missing/malformed value: `Error: --since requires a date (YYYY-MM-DD or YYYYMMDD)` (same for `--until`)
  - inverted window: `Error: --since must be on or before --until`

### 2. Client-side filter — `src/node/core/fetcher.ts`

New pure exported function alongside `aggregateMonthly`/`mergeEntries` (Constitution V — pure functions over `UsageEntry[]`):

```ts
// Inclusive on both ends; since/until are normalized ISO strings.
// Lexicographic compare is a correct total order for ISO labels.
export function filterEntriesByRange(entries: UsageEntry[], since?: string, until?: string): UsageEntry[] {
  return entries.filter((e) => (!since || e.label >= since) && (!until || e.label <= until));
}
```

Bounds are **inclusive** on both ends (`since <= label <= until`), matching ccusage semantics. `--since`-only and `--until`-only are open-ended windows.

### 3. Dispatch wiring — `src/node/core/cli.ts`

The filter applies to **daily entries, after merge, before `aggregateMonthly`** — so monthly rollups reflect the window (a partial month sums only in-window days). It covers every history path:

- **Multi mode**: inside `fetchToolMerged` / `fetchToolMergedWithMachines` (thread `since`/`until` through), after `mergeEntries`, before the `period === "monthly"` aggregation. This covers the `-u <other-user>` repo-only path too (filter after `readRemoteEntries`).
- **Single mode**: after `fetchHistory`, before `aggregateMonthly`, in `dispatchAllHistory` / `dispatchSingleTool` and their `*Lines` watch variants.
- **`--by-machine` history**: filter the per-machine `machineMap` entries consistently with the flattened `entries`, so `buildHistoryMachineCosts` columns can't show out-of-window dates.
- **Watch mode (history display)**: filters are accepted and applied on **each poll** — free, since the `*Lines` variants share the threaded fetch path.
- All output formats (`table`/`--json`/`--csv`/`--md`) see filtered data — the filter sits at the fetch/dispatch layer, above rendering.

The 60s fetch cache is untouched: `fetchHistory` still fetches/caches the full daily set (vanilla call, `extraArgs` stays `[]`); filtering happens after.

### 4. Snapshot display — warn and ignore *(asked at intake)*

When `--since`/`--until` is present and display is snapshot (bare, no `h`), tu warns on stderr and ignores the flags:

```
Warning: --since/--until apply to history display — ignoring.
```

Implemented once in `main()` — warn + clear the flags before dispatch, exactly like the existing `--by-machine` all-tools-history and `-u`-single-mode warn-and-clear guards (cli.ts `main()`). Printing from `main()` (not inside dispatch) means watch-mode snapshot warns **once at startup**, not per poll. Rejected alternative: applying the window to snapshots (when today falls outside it, every table renders all-zero EMPTY rows — strict but confusing; user chose against it).

### 5. Help text + completions

- `FULL_HELP` Flags block (`cli.ts`) gains:
  ```
  --json / -j          Output data as JSON (data commands only)
  --since / -s <date>  Only include entries on/after date (YYYY-MM-DD, history display)
  --until <date>       Only include entries on/before date (YYYY-MM-DD, history display)
  ```
  (`--json` line gains the `-j` alias; exact phrasing/order to match the existing block style.)
- `src/node/core/completions.ts`, all three shells: `--since --until` join `long_flags`, `-s -j` join `short_flags`; value-taking wiring follows `--interval`/`--user` precedent (bash: add `--since|-s|--until` to the no-completion `prev` case; zsh: `'--since[...]:date:'` / `'-s[...]:date:'` / `'--until[...]:date:'` plus `-j`; fish: `complete -c tu -l since -r`, `-l until -r`, `-s s -r`, `-s j`).
- `help-dump`/shll.ai contract: automatic pickup — `runHelpDump` passes raw `FULL_HELP` through byte-for-byte; additive change, no drift concern.

### 6. Tests — co-located `src/node/core/__tests__/`

- `cli-parser.test.ts` (or a new `cli-date-filter.test.ts` for the flag-parsing cases): `-j` sets `outputFormat: "json"`; `--since`/`-s`/`--until` parse + normalize both date shapes; malformed/missing value errors; `since > until` error; existing incompatibility matrix unchanged (`-j` + `--csv` etc. still rejected).
- `fetcher.test.ts`: `filterEntriesByRange` — inclusive bounds, since-only, until-only, both, empty input, no-mutation.
- Filter behavior on the **multi-mode merge path** (backlog-required): windowed history over merged local+remote entries, and monthly rollup of a partially-windowed month.
- `completions.test.ts`: new tokens present in all three scripts.

## Affected Memory

- `cli/data-pipeline`: (modify) add `--since`/`-s`, `--until`, `-j` to the documented `GlobalFlags` set; new requirement for client-side date filtering (inclusive ISO window, filter-before-aggregate, snapshot warn-and-ignore); design decision recording client-side filter over ccusage pass-through (cache + multi-mode rationale).

## Impact

- **Code**: `src/node/core/cli.ts` (parseGlobalFlags, GlobalFlags, main() guard, dispatch threading, FULL_HELP), `src/node/core/fetcher.ts` (new pure `filterEntriesByRange`, ~6 lines), `src/node/core/completions.ts` (three script constants).
- **Tests**: `src/node/core/__tests__/` — cli-parser/fetcher/completions plus multi-mode filter coverage.
- **Independence**: independent of backlog items `[ccfx]`/`[gmcp]`/`[wkly]` — only merge-conflict adjacency in `FULL_HELP`, `completions.ts`, and tests.
- **Output stability (Constitution)**: additive flags; default output unchanged. New capability → minor version bump at release (normal `feat` flow, not part of this change).
- **help-dump/shll.ai**: additive help-text change, picked up automatically by the pull contract.

## Open Questions

- None — the single open decision (snapshot semantics) was asked and resolved at intake.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | `--since`/`-s` and `--until` accept `YYYY-MM-DD` or `YYYYMMDD`, normalized to ISO | Stated verbatim in backlog (mirror ccusage grammar) | S:95 R:85 A:95 D:95 |
| 2 | Certain | `--until` long-only; `-s` pairs with `--since`; `-j` pairs with `--json` | Stated in backlog; short-flag collision audit verified `-u` taken, `-s`/`-j` free | S:95 R:80 A:100 D:95 |
| 3 | Certain | Client-side filter on merged entries, NOT ccusage pass-through | Stated with rationale: pass-through bypasses the 60s cache and misses multi-mode remote entries | S:90 R:70 A:90 D:90 |
| 4 | Certain | Filter daily entries before `aggregateMonthly`; partial months sum only in-window days | Stated in backlog; natural consequence of filter-before-aggregate | S:90 R:80 A:90 D:90 |
| 5 | Certain | `since > until` → error on stderr + exit 1 | Stated in backlog ("(d) validation — error when since > until") | S:85 R:90 A:90 D:85 |
| 6 | Certain | Watch mode accepts the filters and filters each poll (history display) | Stated in backlog ("(c) watch mode — accept and filter each poll") | S:85 R:85 A:85 D:85 |
| 7 | Certain | Snapshot display: warn-and-ignore on stderr | Asked — user chose warn-and-ignore over apply-filter (precedent: `-u` in single mode) | S:100 R:80 A:100 D:100 |
| 8 | Confident | Window bounds inclusive on both ends (`since <= label <= until`) | Not stated; matches ccusage semantics and user expectation; lexicographic ISO compare | S:60 R:85 A:80 D:80 |
| 9 | Confident | Space-separated values only (no `--since=` form); missing/malformed value → exit 1 | Matches the existing `--interval`/`--user` parser idiom exactly; no `=` form exists today | S:55 R:85 A:90 D:80 |
| 10 | Confident | Shape-only date validation (regex), no calendar validity check | Impossible dates degrade to an empty window; compare stays total; keeps parser simple | S:45 R:85 A:75 D:70 |
| 11 | Confident | Filter is a pure exported `filterEntriesByRange` in fetcher.ts, threaded through merged/history paths incl. `-u` and `--by-machine` machineMap | Constitution V pure-function pattern; minimum-pathways principle (one filter, all paths) | S:50 R:80 A:85 D:75 |
| 12 | Confident | Snapshot warn-and-clear implemented once in `main()` before dispatch; watch snapshot warns once at startup | Mirrors existing `--by-machine`/`-u` warn-and-clear guards in `main()` | S:50 R:85 A:85 D:80 |
| 13 | Certain | `-j` joins the existing incompatibility matrix via `jsonFlag`; message wording unchanged (canonical `--json`) | Stated ("add -j in the same flag scan as --json"); matrix operates on `jsonFlag` | S:80 R:90 A:90 D:85 |
| 14 | Certain | help-dump/shll.ai contract picks up new help text automatically | Stated verbatim; `runHelpDump` passes raw `FULL_HELP` through byte-for-byte | S:90 R:90 A:95 D:90 |

14 assumptions (9 certain, 5 confident, 0 tentative, 0 unresolved).
