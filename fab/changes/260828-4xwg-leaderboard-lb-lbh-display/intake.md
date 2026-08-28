# Intake: Leaderboard Display (`lb` / `lbh`)

**Change**: 260828-4xwg-leaderboard-lb-lbh-display
**Created**: 2026-08-28

## Origin

Promptless dispatch (`{questioning-mode} = promptless-defer`). The change was handed over as a
fully-specified design brief; no questions were asked of the user. Every decision the brief did
not settle is recorded below as an **Unresolved** row in `## Assumptions` with the rationale
`Deferred — promptless dispatch`.

> **Description: Leaderboard display (`lb` / `lbh`) — rank users by cost or tokens**
>
> **Context (already existing in tu)**
> - Grammar: `tu [source] [period] [display] [flags]`. Display tokens today: bare (snapshot), `h`/`history`, combined `dh`/`wh`/`mh`.
> - Multi mode: metrics repo `{user}/{year}/{machine}/{tool}-{date}.jsonl`. `-u all` (ALL_USERS) aggregates every user via `listUsers` + `readRemoteEntries` (repo-only, so today lags until `--sync`). `-u all --by-machine` already renders per-user letter-coded columns (legend `Users:`) on snapshot and single-tool history via `fetchToolMergedWithMachines` / `aggregateMachineMap`; it is warn-and-ignored on the all-tools history pivot.
> - `--metric cost|tokens` (and its short form `-t` = `--metric tokens`, added in v0.11.4, `src/node/core/cli.ts` ~line 884) scales history bars; on snapshot displays it currently warns "applies to history display — ignoring" and is cleared in `main()`.
> - `--since/--until` are history-only today (warn-and-clear on snapshots). Daily/weekly history has an implicit 3-month cap lifted by `--full`.
> - All-tools history renders a cost pivot (date rows × tool columns) with stacked bars; single-tool history shows token breakdown.
> - Constitution: single-purpose cost CLI (leaderboard = viewing/aggregating usage, in scope), graceful degradation, pure aggregation over UsageEntry/UsageTotals, tests co-located in `src/**/__tests__/`, output-format changes need tests, minor bump for new output surfaces, conform to shll toolkit standards (`shll standards`) for CLI surface/help/README/docs.
> - Memory to consult: docs/memory/cli/data-pipeline.md, docs/memory/display/formatting.md, docs/memory/sync/multi-machine.md; specs docs/specs/usage.md and docs/specs/layouts.md.
>
> **Decisions made**
> 1. Two new **display tokens** (not a flag): `lb` (leaderboard snapshot) and `lbh` (leaderboard history). Rejected alternative: a `--rank` flag layered on `-u all --by-machine` — that pair is already an opaque incantation; a display token is discoverable and parallels `h`/`mh`.
> 2. `tu [source] [period] lb`: one row per user for the current period (today / this week / this month per period token), ranked descending by the metric. Source token scopes tools (`tu cc m lb`). Columns: rank `#`, user, cost, bar, tokens, share %, Δ vs previous same-length period (prev day/week/month; `new` when the user had no prior-period data). Own user (`config.user`) marked (e.g. `◂`). Total footer line. Dim footer `synced Xm ago · tu sync to refresh` because `-u all` reads are repo-only and lag until sync.
> 3. `tu [source] [period] lbh`: history pivot — period rows × user columns — reusing the existing all-tools pivot renderer with users in place of tools; per-row leader highlighted. Existing 3-month cap / `--full` semantics apply for daily/weekly.
> 4. Ranking is by **raw total** (cost by default; tokens with `--metric tokens` / `-t`). Per-active-day normalisation explicitly rejected by the user — not needed.
> 5. Flags on `lb`/`lbh`:
>    - `--metric` / `-t`: existing; becomes sort key + bar scale. The snapshot warn-and-clear guard for `--metric` in `main()` must treat `lb` as metric-aware (no warning).
>    - `--top <n>` (new, long-only, value-taking, positive integer, exit 2 on bad value): show top N rows, collapse remainder into `… +k others` line (others still count toward Total).
>    - `--since`/`--until`: extend to `lb` (arbitrary window replaces the period window); no warning.
>    - `-u <name>`: on `lb`/`lbh` means pin/highlight that user instead of own user; `-u all` is implied and accepted as a no-op.
>    - `--by-machine`: rows become `user/machine` pairs (map already available from `readRemoteEntriesByMachine`).
>    - `--json` / `--csv` / `--md`: supported. JSON rows `{rank, user, cost, totalTokens, share, delta}` (+ `machine` when `--by-machine`).
>    - `--watch`: supported via the same threaded `*Lines` path as other displays.
>    - `--sync`: existing, works.
> 6. Single mode: `lb`/`lbh` fail fast with exit 1 and the existing multi-mode hint (same as `-u all` today). Graceful: missing/unreadable repo → same guard path as `-u all`.
> 7. Layout mockup agreed (snapshot):
> ```
> Leaderboard (monthly) · 2026-08 · by cost
>
>  #  User        Cost      Tokens   Share  Δ vs Jul
>  1  alice     $412.30  ████████▌  38.1%   +12%
>  2  sahil ◂   $301.10  ██████▎    27.8%    -4%
>  3  bob       $220.05  ████▌      20.3%   +31%
>  4  chen      $149.20  ███        13.8%    new
>                        ─────────
>     Total   $1,082.65
> synced 42m ago · tu sync to refresh
> ```
> 8. Docs: update docs/specs/usage.md (grammar table, flags), docs/specs/layouts.md (new layout mockups), README/help text per toolkit standards. Minor version bump (new output surface).
>
> **Constraints**
> - Reuse existing pipeline: `fetchToolMergedWithMachines` (user-keyed map), `filterEntriesByRange`, `aggregateForPeriod`, pivot renderer. Minimum pathways — no new fetch path.
> - Pure aggregation/ranking functions with co-located tests; tests for table, JSON, CSV, MD output and flag guards.

### Correction to the brief's context (verified against the working tree at v0.11.4)

The brief states that `--metric` "on snapshot displays currently warns *applies to history display
— ignoring* and is cleared in `main()`". **This is no longer true.** Change
`260828-018g-tokens-table-mode-t-flag` (merged as `981a8ea`, released v0.11.4) made `--metric`
reach **every** display, snapshot included, through the shared `withCap` `FormatOptions` merge in
`main()` (`src/node/core/cli.ts` ~lines 1704–1737). There is **no** `--metric` warn-and-clear guard
in `main()` today — only a comment block describing the metric's reach. Consequently decision 5's
first bullet ("must treat `lb` as metric-aware (no warning)") requires **no code change**: `lb` and
`lbh` inherit metric-awareness for free from `withCap`. The warn-and-clear guards that DO exist in
`main()` and that this change must touch are `--since`/`--until` (snapshot-only guard) and
`--by-machine` (all-tools-history guard).

## Why

**The problem.** `tu` can already aggregate every user in the metrics repo (`-u all`), and can
already break that aggregate down per user (`-u all --by-machine`, which renders letter-coded
`A`/`B`/`C` columns with a `Users:` legend). But that view answers "how much did the org spend, by
tool" — it does not answer "who spent the most". To get a ranking today a user must (a) know the
non-obvious `-u all --by-machine` incantation, (b) mentally decode letter-coded columns against a
legend line, and (c) sort the columns by eye. There is no rank, no share-of-total, no
period-over-period delta, and no way to cap the output to the top few users.

**The consequence of not fixing it.** The most common team-level question about AI-assistant spend
("who is driving the cost, and is it going up or down?") stays unanswerable from `tu`'s own output,
pushing users to export `--csv` and sort elsewhere. The data is already in the metrics repo and
already flows through the existing pipeline — only the presentation is missing.

**Why a display token over a flag.** A `--rank` flag would have to be layered on top of
`-u all --by-machine`, compounding an already-opaque incantation into a three-part one. A display
token (`tu m lb`) is discoverable through `tu -h`, shell completion, and the existing grammar; it
parallels the `h` / `dh` / `wh` / `mh` tokens users already know, and it lets the leaderboard own
its own column set rather than bending the machine-breakdown columns into a shape they were not
designed for.

**Why in scope.** Constitution I limits `tu` to "viewing, aggregating, and syncing usage/cost
data". A leaderboard is pure viewing + aggregation over data the tool already holds — no new data
source, no new fetch path, no orchestration.

## What Changes

### 1. Grammar — two new display tokens

`parseDataArgs` (`src/node/core/cli.ts` ~line 1054) gains two tokens in its positional loop,
alongside the existing `d`/`w`/`m`/`h`/`dh`/`wh`/`mh` arms:

```ts
} else if (arg === "lb") {
  display = "leaderboard";
} else if (arg === "lbh") {
  display = "leaderboard-history";
}
```

Neither token sets `period` — the period comes from the separate period token, so the full grammar
`tu [source] [period] lb` works as `tu lb` (daily), `tu w lb`, `tu m lb`, `tu cc m lb`. **No
combined `dlb`/`wlb`/`mlb` shorthands are introduced** (see Assumptions).

`DataArgs.display` is a plain `string` today, so no type widening is needed; every `display ===
"history"` comparison in `main()` and the dispatchers must be audited (see §5).

### 2. `lb` — leaderboard snapshot

**Data.** One row per user (or per `user/machine` pair under `--by-machine`), for the current
period label (`currentLabel(period)` — today / this week's Sunday / this month), or for the
`--since`/`--until` window when one is given. The source token scopes which tools are summed:
`tu cc m lb` ranks users by Claude Code spend only; `tu m lb` (source `all`) sums every tool.

**Pipeline (no new fetch path).** Reuse `fetchToolMergedWithMachines(config, toolKey, period, [],
skipCache, ALL_USERS, since, until)` — with `targetUser === ALL_USERS` it already returns
`{ entries, machineMap }` where `machineMap` is **keyed by user name** (`readAllUsersByUser` →
`aggregateMachineMap`). Summing that map across the source's tool keys yields
`Map<user, UsageEntry[]>`, which is everything the leaderboard needs. For `--by-machine` the map
must instead be keyed by `user/machine`; `readRemoteEntriesByMachine(metricsDir, user, null,
toolKey)` per user from `listUsers(metricsDir)` supplies that, mirroring `readAllUsersByUser`'s
shape.

**Pure aggregation/ranking.** A new pure module (e.g. `src/node/core/leaderboard.ts`) exporting:

```ts
export interface LeaderboardRow {
  rank: number;          // 1-based, after sorting
  user: string;          // user name, or "user/machine" under --by-machine
  machine?: string;      // present only under --by-machine
  totals: UsageTotals;   // summed across the source's tools and the window
  share: number;         // 0..1 of the visible grand total, in the display metric
  delta?: number;        // fractional change vs the previous window; undefined ⇒ "new"
}

export function buildLeaderboard(
  byUser: Map<string, UsageEntry[]>,
  prevByUser: Map<string, UsageEntry[]> | undefined,
  metric: BarMetric,
): LeaderboardRow[];
```

Rules:
- Sum every in-window entry per key into one `UsageTotals` (all six numeric fields).
- Drop keys whose `totalTokens === 0` **and** `totalCost === 0` (mirrors the snapshot renderer's
  existing "tools with zero tokens are omitted" rule).
- Sort descending by `metricValue(totals, metric)` (the exported helper from
  `src/node/tui/formatter.ts`); ties break by key name ascending so output is deterministic.
- `share = metricValue(row) / grandTotal`, `0` when the grand total is `0`.
- `delta = (current − previous) / previous` when the key had a nonzero previous value; `undefined`
  when the key is absent from `prevByUser` or its previous value is `0` (rendered `new`).
- Pure: no mutation of inputs, no I/O (Constitution V, code-quality "functions and plain objects").

**Previous-period window.** For a period token the previous window is the immediately preceding
period of the same kind — previous calendar day / previous week (the prior Sunday-anchored week) /
previous calendar month. Because the pipeline already fetches full daily history and windows it
client-side, the previous window is obtained by a **second `filterEntriesByRange` pass over the
same fetched entries** — not a second fetch. Concretely: fetch unwindowed (or window to
`[prevStart, currentEnd]`), then slice twice.

**Rendering** — a new `renderLeaderboard(...) : string[]` + `printLeaderboard(...)` pair in
`src/node/tui/formatter.ts`, following the established render/print split. Agreed layout:

```
Leaderboard (monthly) · 2026-08 · by cost

 #  User        Cost      Tokens   Share  Δ vs Jul
 1  alice     $412.30  ████████▌  38.1%   +12%
 2  sahil ◂   $301.10  ██████▎    27.8%    -4%
 3  bob       $220.05  ████▌      20.3%   +31%
 4  chen      $149.20  ███        13.8%    new
                       ─────────
    Total   $1,082.65
synced 42m ago · tu sync to refresh
```

- Heading: `Leaderboard ({period}) · {window label} · by {cost|tokens}`. Under an explicit
  `--since`/`--until` the window label is the range (`2026-08-01 → 2026-08-27`).
- Columns: rank `#`, `User`, `Cost`, bar, `Tokens`, `Share`, `Δ vs {prev label}`. Both the Cost and
  Tokens columns render in **every** metric mode — only the bar scale, the sort key, the share
  denominator and the heading's `by …` suffix follow `--metric`.
- Numeric columns are data-sized via the existing shared `metricColumnWidth(values, metric)` helper
  with its `COST_WIDTH` (9) floor; costs go through `fmtCost` (thousands separators), tokens
  through `fmtMetric(v, "tokens")`.
- Bars use the existing `renderBar` / `computeBarScale` / `renderScaledBar` primitives, scaled to
  the max row value in the display metric, with the same `MAX_BAR_WIDTH` / `MIN_BAR_AREA` budget
  rules the other renderers use. Bars are solid green (single-series, no stacking).
- The pinned user's row carries the ` ◂` marker after the name (`-u <name>` pins that user;
  otherwise `config.user`). The marker occupies real width in the User column (it is a glyph, not
  a color), so the column must be sized including it.
- Exact-zero metric cells render dim via the existing `metricCell(value, width, metric)` helper.
- Total footer row (`boldWhite`, like every other Total row) rendered only when ≥2 visible rows.
- Dim staleness footer: `synced {relativeTime} ago · tu sync to refresh`, sourced from the
  `.last-sync` file in `TU_HOME` — reuse the existing `relativeTime(ms)` export and the
  `formatLastSync(tuHome, now)` helper already in `src/node/core/cli.ts` (used by `tu status`).
  When `.last-sync` is absent the footer reads `never synced · tu sync to refresh`. ANSI table
  output only — CSV/JSON/MD carry no footer, consistent with the existing footer rules.

### 3. `lbh` — leaderboard history

Period rows × **user** columns, reusing `renderTotalHistory` (`src/node/tui/formatter.ts`) with
user names substituted for tool names. The data shape it consumes is already
`Map<columnName, UsageEntry[]>`, which is exactly what `aggregateMachineMap`'s user-keyed
`machineMap` produces once summed across the source's tools — so `lbh` is a data-shaping change,
not a new renderer.

- Title: `📊 Leaderboard History ({period})` (`Leaderboard Token History` under tokens), replacing
  the pivot's `Combined Cost History` / `Combined Token History` string when the columns are users.
  Thread this through a `FormatOptions` field rather than a second renderer (one path — see
  `display/formatting.md` § "One metric-generic render path").
- Column ordering: descending by window total in the display metric (a leaderboard is ranked), not
  registry order. Note this diverges from the tool pivot's fixed registry order, whose stability
  rationale (a tool's color never changes across windows) does not transfer — user sets are not a
  fixed registry.
- Per-row leader highlighted: within each date row, the winning user's cell renders `boldWhite`
  (color-only, width unchanged, stripped by `--no-color`/`NO_COLOR`) — same mechanism as the
  existing current-period date-cell marker.
- Inherited for free from `renderTotalHistory`: month separators, current-period row marker,
  weekend date dimming, stacked per-column bars + legend, the p95 two-zone bar scale, the dim
  summary footer, dim exact-zero cells, and data-sized columns.
- Existing implicit 3-month cap applies exactly as for `h`: daily/weekly `lbh` is capped unless
  `--full` or an explicit `--since`/`--until` is given; monthly `lbh` is never capped. This means
  `capApplies(...)` (`src/node/core/cli.ts` ~line 852) must treat `leaderboard-history` as a
  history display.

### 4. New flag — `--top <n>`

Long-only, value-taking, parsed in `parseGlobalFlags`'s space-separated value-flag loop (the same
loop shape as `--interval` / `--user` / `--since`), exposed as `topFlag: number | undefined` on
`GlobalFlags`, and stripped from `filteredArgs`.

- Valid values: a positive integer (`>= 1`). A missing value, a non-integer, or `< 1` prints
  `Error: --top requires a positive integer` to stderr and exits `EXIT_USAGE` (2) — matching every
  other bad-flag-value site (`docs/specs/usage.md` § Exit Codes).
- On `lb`: render the top N rows, then one dim collapsed line `… +k others` where
  `k = visibleRows − N`. The collapsed users **still count** toward the Total row and toward every
  row's `share` denominator. Omit the collapsed line when `k === 0`.
- On a display other than `lb`/`lbh`: warn once on stderr
  (`Warning: --top applies to leaderboard display — ignoring.`) and clear the flag in `main()`,
  mirroring the existing `--since`/`--until` and `--full` warn-and-clear guards (so watch mode
  warns once at startup, not per poll).
- `--top` applies to `--json`/`--csv`/`--md` as well as the table: unlike the pivot's *automatic*
  negligible-column omission (which CSV opts out of because scripts index columns positionally),
  `--top` is an **explicit** user request, and silently ignoring it in a machine format would be
  the surprise.
- Behavior on `lbh` is **deferred** — see Assumptions.

### 5. `main()` guard changes (`src/node/core/cli.ts` ~lines 1663–1737)

| Guard | Today | After |
|-------|-------|-------|
| `-u` in single mode | warn + clear `userFlag` | unchanged for other displays; on `lb`/`lbh` the single-mode failure below fires first |
| single mode + `lb`/`lbh` | n/a | **new**: print an error naming multi mode and exit `1` (operational — the environment/config must be fixed, per `docs/specs/usage.md` § Exit Codes), before any fetch |
| `--by-machine` + all-tools history | warn + clear | unchanged; `lbh` gets the same warn + clear (Assumption 22) |
| `--since`/`--until` on non-history display | warn + clear | condition must **exempt** `lb` (and `lbh`, already history-shaped) — an explicit window on `lb` replaces the period window with no warning |
| `--full` on non-history display | warn | must treat `lbh` as history (no warning); `--full` on `lb` warns like any snapshot |
| `--metric` | no guard exists (see the Origin correction) | no change needed |
| `--top` | n/a | **new** warn-and-clear on non-leaderboard displays |
| implicit 3-month cap (`capApplies`) | `display === "history"` | must also engage for `display === "leaderboard-history"`; never for `lb` |

`-u all` on `lb`/`lbh` is accepted as an explicit no-op (the leaderboard is inherently all-users);
`-u <name>` pins/highlights that user instead of filtering to it.

Graceful degradation (Constitution II): a missing or unreadable metrics repo takes the existing
`checkMetricsDirGuard` path — which already falls back to single mode with a stderr warning — and
then hits the new single-mode `lb` failure with an actionable message. `listUsers` already returns
`[]` for a missing/unreadable dir with no throw; an empty user list must render an empty
leaderboard with its heading and footer, never crash.

### 6. Dispatch

New `dispatchLeaderboard` / `dispatchLeaderboardHistory` (one-shot, printing) plus
`dispatchLeaderboardLines` / `dispatchLeaderboardHistoryLines` (watch, returning `string[]`),
following the existing two-dispatch-path convention. Each fetches **once** and then runs a single
`switch (outputFormat)` selecting `emitJson` / `emitCsv` / `emitMarkdown` / the `print*` renderer —
duplicating fetch logic across format branches is prohibited (`display/formatting.md`).

Routing in `main()` (both the watch `action` closure and the one-shot branch) dispatches on the new
`display` values before the existing `source === "all"` split, because the leaderboard's source
token only scopes tools — it does not change the display shape.

Watch mode: `_lastRenderCost`, `_lastRenderTotalTokens` and `_lastRenderCostMap` must be set as the
other dispatchers do; the prev-cost map keys on the plain user name (`{user}`), mirroring the
snapshot renderer's plain `{toolName}` keying, and is valued in the displayed metric via
`buildCostMap(..., fmtOpts?.metric ?? "cost")`.

### 7. Machine-readable formats

- **JSON**: an array of row objects `{ rank, user, cost, totalTokens, share, delta }`, plus
  `machine` when `--by-machine`. `delta` is `null` for a `new` row. `share` is a fraction
  (`0.381`), not a percent string. `lbh` JSON keeps the pivot's existing map-of-column-to-entries
  shape (`Object.fromEntries` of the user-keyed map), consistent with `tu h --json`.
- **CSV** (`emitCsv`, new kind `"leaderboard"`): header `rank,user,cost,total_tokens,share,delta`
  (+ `machine` after `user` under `--by-machine`); raw numbers, no `$`, no thousands separators, no
  ANSI/bars/arrows; `delta` empty for a `new` row; a final `Total,...` row when >1 visible row —
  all per the existing CSV contract in `display/formatting.md`.
- **Markdown** (`emitMarkdown`, new kind `"leaderboard"`): `## Leaderboard ({period})` heading, GFM
  table, left-aligned strings / right-aligned numerics, `$`-prefixed costs with thousands
  separators, `**Total**` row bolded. No bars, no arrows, no staleness footer.

### 8. Surfaces to update (toolkit standards)

Per the constitution's Toolkit Standards clause, run `shll standards` and check each surface
against the standard that governs it before editing:

- `FULL_HELP` and `SHORT_USAGE` (`src/node/core/cli.ts` ~lines 92–147): add `lb`/`lbh` to the
  `Display:` line, an example row, and the `--top <n>` flag row.
- Shell completions (`src/node/core/completions.ts`): add `lb`/`lbh` to the bash `display` local,
  the zsh `display` array, and both fish `complete` lines; add `--top` to each shell's flag list.
- `docs/site/skill.md` (the `tu skill` agent bundle — must stay byte-identical to the embedded
  `SKILL_MD`; the build embeds it via esbuild `--define` with a drift guard).
- `README.md` (§ Usage, § Flags).
- `docs/specs/usage.md`: the Display table, the Global Flags table (`--top`, and the `--since`/
  `--until`/`--metric` rows' display applicability), the Exit Codes table (`--top` bad value → 2;
  `lb`/`lbh` in single mode → 1), and an Output Formats subsection for the leaderboard.
- `docs/specs/layouts.md`: two new layout sections (leaderboard snapshot, leaderboard history) with
  ASCII mockups and column/color notes, in the same style as Layouts 1–4.

### 9. Versioning

New output surface ⇒ **minor** bump per the constitution's Output Stability rule. Per the standing
decision in `display/formatting.md` ("Output-Stability version bumps happen at release time, not in
the feature diff"), this change **must not** edit `package.json`; the PR body notes "requires minor
release" and the bump happens via `just release minor`.

### 10. Tests (co-located, `src/**/__tests__/`)

- `src/node/core/__tests__/leaderboard.test.ts` — pure `buildLeaderboard`: ranking order,
  tie-break, share arithmetic, delta/`new`, zero-row omission, input immutability, metric switch.
- `src/node/core/__tests__/cli-parser.test.ts` (extend) — `parseDataArgs` accepts `lb`/`lbh` in
  every positional slot and with each period/source token.
- `src/node/core/__tests__/cli-top-flag.test.ts` — `--top` parsing, bad values → exit 2,
  warn-and-clear on non-leaderboard displays.
- `src/node/tui/__tests__/formatter-leaderboard.test.ts` — table output: heading, columns, bar,
  pinned-user marker, Total row, `… +k others` line, staleness footer, dim zeros, `--no-color`
  byte-equality.
- Format emitters — JSON / CSV / MD leaderboard output shape (extend `cli-json.test.ts` or add a
  dedicated file, following the existing per-surface test-file convention).
- `src/node/core/__tests__/cli-exit-codes.test.ts` (extend) — single-mode `lb` exits 1;
  `--top 0` exits 2.

## Affected Memory

- `cli/data-pipeline.md`: (modify) new `lb`/`lbh` display tokens in `parseDataArgs`; the new
  `--top <n>` global flag (parsing, validation, exit 2, warn-and-clear guard); `--since`/`--until`
  extension to `lb`; `capApplies` extension to `leaderboard-history`; the single-mode `lb`/`lbh`
  exit-1 guard and its row in the exit-code site inventory; the new leaderboard dispatch functions
  and their `*Lines` watch variants; `-u` semantics on the leaderboard (pin, `all` no-op).
- `display/formatting.md`: (modify) the new `renderLeaderboard`/`printLeaderboard` layout (columns,
  data-sizing, bar, pinned-user glyph, Total row, `… +k others` collapse, staleness footer); the
  `lbh` reuse of `renderTotalHistory` with user columns (title override, rank-ordered columns,
  per-row leader highlight); the new `"leaderboard"` kinds on `emitCsv`/`emitMarkdown` and the JSON
  row shape.
- `sync/multi-machine.md`: (modify) the all-users repo-only read gains a display consumer whose
  staleness footer surfaces the `-u all` lag directly; note the `user/machine`-keyed variant of
  `readAllUsersByUser` used by `--by-machine` on `lb`.

## Impact

**Code**

| Area | Change |
|------|--------|
| `src/node/core/cli.ts` | `parseDataArgs` (+2 tokens), `parseGlobalFlags` (+`--top`), `GlobalFlags` (+`topFlag`), `capApplies`, `main()` guards + dispatch routing, 4 new dispatch functions, a `user/machine`-keyed all-users reader |
| `src/node/core/leaderboard.ts` | **new** — pure `buildLeaderboard` + row types |
| `src/node/tui/formatter.ts` | new `renderLeaderboard`/`printLeaderboard`; `renderTotalHistory` title/ordering/leader-highlight hooks; `emitCsv`/`emitMarkdown` `"leaderboard"` kinds |
| `src/node/core/completions.ts` | `lb`/`lbh`/`--top` in all three shells |
| `src/node/tui/watch.ts` | none expected — the leaderboard rides the existing `*Lines` → compositor path |
| `docs/site/skill.md` | new display tokens + `--top` in the agent bundle (drift-guarded against the build embed) |
| `README.md`, `docs/specs/usage.md`, `docs/specs/layouts.md` | documentation per §8 |

**Not changed**: the fetch layer (`src/node/core/fetcher.ts`), the sync layer
(`src/node/sync/sync.ts`), the 60s cache, the compositor/rain/panel TUI primitives, and every
existing display's byte-level output (the leaderboard is additive; no existing render path changes
behavior when `lb`/`lbh`/`--top` are absent).

**Risk areas**
- The `display === "history"` string comparisons scattered through `main()` and the dispatchers:
  missing one leaves `lbh` un-capped or mis-guarded. Audit every occurrence.
- `renderTotalHistory` is shared with `tu h` — the title/ordering/leader hooks must default to
  today's behavior exactly, or the tool pivot's output shifts (Output Stability).
- Terminal width: the leaderboard row is narrower than the 6-tool pivot, but the `Δ vs {label}`
  header plus a long user name can still push it; reuse the existing bar-budget arithmetic rather
  than inventing a second one.

## Open Questions

None — the three deferred questions (Δ under an explicit window, `--top` on `lbh`, `--by-machine` on
`lbh`) were resolved during `/fab-proceed`; see Assumptions rows 20–22.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Two display tokens `lb`/`lbh` rather than a `--rank` flag on `-u all --by-machine` | User decided explicitly, with the rejected alternative and its reason stated in the brief | S:95 R:70 A:90 D:95 |
| 2 | Certain | Ranking is by raw total (cost by default, tokens under `--metric tokens`/`-t`); per-active-day normalisation is out of scope | User decided explicitly and explicitly rejected normalisation | S:95 R:80 A:90 D:95 |
| 3 | Certain | No `--metric` warn-and-clear guard exists in `main()` today (018g/v0.11.4 made the metric reach every display via `withCap`), so decision 5's "treat `lb` as metric-aware" needs no code change | Verified directly against `src/node/core/cli.ts` at v0.11.4 and against `docs/memory/cli/data-pipeline.md`; the brief's context statement is stale | S:90 R:85 A:100 D:95 |
| 4 | Certain | Minor version bump is satisfied at release time (`just release minor`), not by editing `package.json` in this diff | Standing project decision recorded in `docs/memory/display/formatting.md` § "Output-Stability version bumps happen at release time" | S:85 R:90 A:100 D:95 |
| 5 | Certain | Snapshot column set, own-user `◂` marker, Total footer, and the `synced Xm ago · tu sync to refresh` staleness line are as mocked in the brief | User supplied the exact mockup | S:95 R:75 A:85 D:90 |
| 6 | Certain | No combined `dlb`/`wlb`/`mlb` shorthands — the period token composes with `lb` (`tu m lb`) | The brief names exactly two tokens; the grammar already supports `[period] [display]` composition, and `dh`/`wh`/`mh` exist only as legacy convenience. Adding shorthands later is purely additive. Confirmed by orchestrator under the user's go-ahead during /fab-proceed | S:90 R:85 A:90 D:90 |
| 7 | Certain | Both Cost and Tokens columns render in every metric mode on `lb`; `--metric` changes only the sort key, bar scale, share denominator, and the heading's `by …` suffix | Direct precedent: `renderTotal` is metric-neutral and keeps its Cost column under `-t`, with only the delta indicator following the metric (`display/formatting.md`). Confirmed by orchestrator under the user's go-ahead during /fab-proceed | S:90 R:85 A:90 D:90 |
| 8 | Certain | Rows with zero cost and zero tokens in the window are omitted from `lb` | Direct precedent: the snapshot renderer omits zero-token tool rows, and the pivot omits negligible columns. Confirmed by orchestrator under the user's go-ahead during /fab-proceed | S:90 R:85 A:90 D:90 |
| 9 | Certain | Ties in the ranking break by key name ascending | Determinism is required for stable output and testability; no other signal exists and the choice is trivially reversible. Confirmed by orchestrator under the user's go-ahead during /fab-proceed | S:90 R:85 A:90 D:90 |
| 10 | Certain | `--top` applies to `--json`/`--csv`/`--md` as well as the table | `--top` is an explicit user request, unlike the *automatic* negligible-column omission that CSV opts out of; silently ignoring an explicit flag in a machine format would be the surprise. Confirmed by orchestrator under the user's go-ahead during /fab-proceed | S:90 R:85 A:90 D:90 |
| 11 | Certain | Single-mode `lb`/`lbh` exits `1` (operational failure), not `2` | User specified exit 1, and it matches the spec's classification — a missing/misconfigured metrics repo is operational, recovery is fixing the environment, not the command line. Confirmed by orchestrator under the user's go-ahead during /fab-proceed | S:90 R:85 A:90 D:90 |
| 12 | Certain | The previous-period window is derived by a second `filterEntriesByRange` pass over the already-fetched daily entries — no second fetch | Constraint: "minimum pathways — no new fetch path"; the pipeline already fetches full daily history and windows client-side. Confirmed by orchestrator under the user's go-ahead during /fab-proceed | S:90 R:85 A:90 D:90 |
| 13 | Certain | Watch-mode prev-cost keys for `lb` use the plain `{user}` form, valued in the displayed metric | Mirrors the snapshot's plain `{toolName}` keying documented in `display/formatting.md` § "Delta via callback". Confirmed by orchestrator under the user's go-ahead during /fab-proceed | S:90 R:85 A:90 D:90 |
| 14 | Certain | `lbh` columns are ordered by descending window total, not a fixed order | A leaderboard is ranked by definition; the tool pivot's fixed registry order exists to keep per-tool colors stable across windows, which does not transfer to a non-fixed user set. Confirmed by orchestrator under the user's go-ahead during /fab-proceed | S:90 R:85 A:90 D:90 |
| 15 | Certain | An empty user list (missing/unreadable repo, or no profiles) renders an empty leaderboard with heading and footer rather than erroring | Constitution II graceful degradation; `listUsers` already returns `[]` with no throw for a missing dir | S:55 R:85 A:90 D:85 |
| 16 | Certain | `lbh` **disables** the pivot's negligible-column omission for user columns, so no user is silently hidden from a leaderboard | The brief says "reuse the existing pivot renderer" without addressing omission. Silently dropping a low-spend user from a ranking is semantically wrong, and `--top` already gives explicit control — but keeping the omission is a defensible width-driven alternative <!-- assumed: lbh disables negligible-column omission so no user is hidden from a ranking -->. Confirmed by orchestrator under the user's go-ahead during /fab-proceed | S:90 R:85 A:90 D:90 |
| 17 | Certain | A user whose previous-window value is exactly `0` renders `new` (same as absent); `delta` is `null`/empty in machine formats | Avoids divide-by-zero / infinite percentage; `new` already means "no prior baseline". Confirmed by orchestrator under the user's go-ahead during /fab-proceed | S:90 R:85 A:90 D:90 |
| 18 | Certain | Single-mode `lb`/`lbh` message is exactly `Error: lb requires multi mode — run tu init-metrics <repo-url> to set up a metrics repo` on stderr, exit 1 | Composed from the existing multi-mode wording; names the fix command. Confirmed by orchestrator under the user's go-ahead during /fab-proceed | S:90 R:85 A:90 D:90 |
| 19 | Certain | The pure ranking logic lives in a new `src/node/core/leaderboard.ts`; the renderers live in `src/node/tui/formatter.ts` | Matches the existing core/tui split and keeps `cli.ts` from growing another god-section; placement is trivially movable. Confirmed by orchestrator under the user's go-ahead during /fab-proceed | S:90 R:85 A:90 D:90 |
| 20 | Certain | Under an explicit `--since`/`--until` window on `lb`, `Δ` compares against the equal-length immediately-preceding window (header `Δ vs prev`) | Keeps the column present and meaningful for any window; omitting it would make `lb --since` a lesser view. Resolved by orchestrator during /fab-proceed. Confirmed by orchestrator under the user's go-ahead during /fab-proceed | S:90 R:85 A:90 D:90 |
| 21 | Certain | On `lbh`, `--top <n>` keeps the N user columns with the highest window total (by the active metric) and folds the rest into a single `others` column, so row totals are preserved | Mirrors the `lb` `… +k others` collapse; dropping columns would make rows not sum to their totals. Resolved by orchestrator during /fab-proceed. Confirmed by orchestrator under the user's go-ahead during /fab-proceed | S:90 R:85 A:90 D:90 |
| 22 | Certain | `--by-machine` on `lbh` warns on stderr and is ignored, exactly as the all-tools pivot does today; it is honoured only on `lb` | Consistency with the existing pivot guard; per-machine columns on a pivot would explode width. Resolved by orchestrator during /fab-proceed. Confirmed by orchestrator under the user's go-ahead during /fab-proceed | S:90 R:85 A:90 D:90 |

22 assumptions (22 certain, 0 confident, 0 tentative, 0 unresolved).
