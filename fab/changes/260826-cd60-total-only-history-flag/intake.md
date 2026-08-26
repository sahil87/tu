# Intake: Total-only history view (`--total` / `-t`) for the all-tools pivot

**Change**: 260826-cd60-total-only-history-flag
**Created**: 2026-08-27

## Origin

Conversational — synthesized from a live discussion in which the user showed a `tu mh -u all` screenshot and asked for a view without the per-tool columns. Dispatched promptless via `/fab-proceed` (no clarifying questions asked; every decision is recorded in `## Assumptions`).

> The all-tools history (`tu h`, `tu wh`, `tu mh`, including `-u all`) always renders the pivot with one cost column per active tool (Claude Code / Codex / OpenCode / Gemini / Kimi …). I just want the date, the total, and the bar — no per-tool columns. I'm going to use this a lot, so it needs a short flag.

Key decisions reached in the conversation:

- Flag is `--total` with short alias `-t` (both forms). `-t` is currently unused.
- Scope is the all-tools **history** display only; snapshot and single-tool uses warn and are ignored.
- Reuse `renderTotalHistory` with a branch — do not fork a second renderer.
- Rejected: a separate `tu total` command (grammar creep, Constitution I); auto-collapsing on narrow terminals (implicit behavior change, Output Stability); a long-only flag (user explicitly wants a short form).

## Why

**Problem.** `renderTotalHistory` (`src/node/tui/formatter.ts`) is the only all-tools history layout: `Date | <one cost column per nonzero tool> | Cost | bar`. For a team view (`tu mh -u all`) the per-tool breakdown is noise when the question is simply "how much did we spend per month, and what is the trend?". Worse, the columns crowd out the bar chart: the full data row is ~96 visible chars with six active tools, so `showBars` (`barWidth >= MIN_BAR_AREA`) is false below roughly 110 columns and the graph — the thing the user actually wants — disappears on an ordinary 80–100-col terminal. A total-only row (`Date | Cost` ≈ 22 chars) leaves room for the full 30-char bar in any 80-col terminal.

**If we don't fix it.** Users widen their terminal or squint past five columns to read one number; the trend graph is unavailable at default widths for exactly the multi-user, multi-tool case where it matters most.

**Why this approach.** A boolean flag on the existing history grammar is the smallest surface addition that fits the flag-based grammar (Constitution I — no new subcommand). Making it explicit (not width-triggered) keeps default output byte-identical (Constitution § Output Stability) and keeps the behavior predictable in scripts and watch mode. Compact mode (`< 60` cols) already renders date + cost only, so `--total` is the deliberate, wide-terminal counterpart that keeps the bar.

## What Changes

### 1. CLI flag parsing — `src/node/core/cli.ts`

- Add `totalFlag: boolean` to `GlobalFlags` (next to `fullFlag` / `metricFlag`).
- In `parseGlobalFlags`: `const totalFlag = rawArgs.includes("--total") || rawArgs.includes("-t");` (same pattern as `fullFlag` / `freshFlag`).
- Add `a === "--total" || a === "-t"` to the boolean skip list on the `filteredArgs` loop (the `if (a === "--json" || a === "-j" || ... || a === "--full" || a === "--skip-brew-update") continue;` line) so it never reaches `parseDataArgs`.
- Return `totalFlag` from `parseGlobalFlags` and destructure it in `main()`.

### 2. Scope guard — `main()` in `src/node/core/cli.ts`

Add a warn-once-and-clear guard at the same top-level spot as the `--by-machine` / `--since/--until` / `--full` / `--metric` guards (before `withCap` is built), so watch mode warns once at startup, not per poll:

```ts
// --total collapses the all-tools history pivot to Date + total + bar. On a
// snapshot display or a single-tool source it warns once and is cleared
// (same spot as the since/until and --metric guards). JSON/CSV/MD have no
// pivot columns to collapse and ignore it silently.
if (totalFlag && !(source === "all" && display === "history")) {
  process.stderr.write("Warning: --total applies to all-tools history — ignoring.\n");
  totalFlag = false;
}
```

Exit code stays 0 (warning, not usage error) — mirrors the existing guards. `--total` + `--by-machine` on all-tools history needs no new handling: `--by-machine` is already warned-and-cleared there.

### 3. Threading — `withCap` merge in `main()` and `FormatOptions`

- `src/node/tui/formatter.ts`: add `total?: boolean;  // all-tools history: Date + total + bar only, no per-tool columns; absent ≡ false` to `FormatOptions`.
- Extend the existing `withCap` helper (which already stamps `capActive`, `metric`, `machineLegend`) so both one-shot (`dispatchAllHistory`) and watch (`dispatchAllHistoryLines`) paths receive it:

```ts
const withCap = (fmtOpts?: FormatOptions): FormatOptions | undefined => {
  if (!capActive && metricFlag === "cost" && !usersLegend && !totalFlag) return fmtOpts;
  return {
    ...fmtOpts,
    ...(capActive ? { capActive: true } : {}),
    ...(metricFlag !== "cost" ? { metric: metricFlag } : {}),
    ...(usersLegend ? { machineLegend: "Users" } : {}),
    ...(totalFlag ? { total: true } : {}),
  };
};
```

Nothing is stamped when the flag is false, so default output stays byte-identical. `--json` / `--csv` / `--md`: the flag reaches `emitMarkdown` via `fmtOpts` only as an ignored field — CSV keeps every registry column (positional contract), JSON structure unchanged, Markdown unchanged. `--total` is an ANSI-renderer-only option, exactly like `--metric`.

### 4. Rendering — `renderTotalHistory` in `src/node/tui/formatter.ts`

When `opts?.total` is truthy (and not compact — compact already renders date + cost only, so `--total` in compact mode is a no-op), the pivot branch renders:

- **Header**: `Date | Cost` (or `Date | Tokens` under `--metric tokens`) — no tool-name columns. Header cells styled `boldCyan` as today.
- **Divider**: `divStr + costDiv + barDiv` construction with `toolWidths = []`, i.e. `"─".repeat(D) + "─|─" + "─".repeat(COST_WIDTH) + barDiv`.
- **Data rows**: `label | value | bar`. The value cell is `fmtMetric(barTotal, metric)` — `fmtCost(rowCost)` under cost, `fmtNum(Math.round(tokenTotal))` under `--metric tokens` (so the bar always has a visible number next to it, unlike the multi-column pivot whose cells stay cost-denominated). Width `COST_WIDTH` for both.
- **Bar**: solid (unstacked) `renderScaledBar(r.barTotal, scale, barWidth)` using the same `computeBarScale` two-zone scale as today — no stacked per-tool segments, since there are no tool columns to key colors to.
- **`Total` row**: `Total | <grand total>` only (`boldWhite`), no per-tool sums. Under `--metric tokens` the Total row shows the summed token total via `fmtNum`.
- **Footer**: `renderHistoryFooter(...)` with `legend = undefined` — keeps `avg` / `this month` / `peak` / `p95` but drops the tool swatches.
- **Unchanged**: heading `📊 Combined Cost History (monthly[, last 3 months])`; `maxRows` truncation; "No data" path; daily month-boundary separators; current-period `boldWhite` label; weekend dimming; `prevCosts` space-less delta indicator and its `indicatorReserve`.
- **Bar width**: `tableWidth = D` (no tool columns), so `barWidth = min(width - D - GUTTER - COST_WIDTH - 1 - indicatorReserve, MAX_BAR_WIDTH)` → the full 30-char bar fits at 80 cols.

Implementation shape: compute `toolNames`/`toolWidths` as today, then `const collapsed = opts?.total === true;` and derive `visibleTools = collapsed ? [] : toolNames`, `visibleWidths = collapsed ? [] : toolWidths`, so `row`/`colorRow`/`divStr`/`tableWidth` fall out of the existing per-column-width machinery with an empty array; branch only at the value cell, the bar call (stacked vs. solid), the Total row cells, and the legend. Target ≤ ~25 LOC delta in the renderer.

Example (`tu mh -u all -t`, 80 cols):

```
📊 Combined Cost History (monthly)

Date       |      Cost
───────────|───────────────────────────────────────────
2026-03    |   $412.20 ████████▍
2026-04    | $1,893.55 ██████████████████████████████
2026-05    | $1,102.07 █████████████████▍
...
───────────|───────────────────────────────────────────
Total      | $6,031.88
avg $1,005.31/month · this month $1,102.07 · peak $1,893.55 (2026-04)
```

### 5. Help, docs, completions (Toolkit Standards lockstep)

- `FULL_HELP` (`cli.ts`), inserted after the `--metric` line:
  `  --total / -t         Collapse all-tools history to Date + total + bar (no per-tool columns)`
- `README.md` `### Flags` block: mirror the same line (shll readme-extraction cross-checks README flags against `help-dump`).
- `docs/site/workflows.md`: flag bullet `- \`--total\` / \`-t\` — all-tools history as Date + total + bar, no per-tool columns` and recipe line `tu mh -u all -t   # team monthly total, graph only`.
- `docs/site/skill.md`: one-liner in the history paragraph: "Pass `--total` / `-t` to collapse the all-tools history to date + total + bar (no per-tool columns)."
- `docs/specs/usage.md` Global Flags table row: `| \`--total\` | \`-t\` | Collapse the all-tools history pivot to Date + total + bar (no per-tool columns, no stacked segments); all-tools history only — warns and is ignored on snapshots and single-tool sources; no effect on \`--json\`/\`--csv\`/\`--md\` |`.
- `docs/specs/layouts.md` § 4 (History — All Tools (Pivot)): bullet describing the `--total` layout (Date | Cost | solid bar; Total row grand total only; footer without legend; `Tokens` header under `--metric tokens`).
- `src/node/core/completions.ts`: `--total` in bash/zsh/fish long lists; `-t` in the short-flag lists; zsh `'--total[all-tools history as Date + total + bar]'` and `'-t[...]'`; fish `complete -c tu -l total -s t -d 'all-tools history as Date + total + bar'`.
- `src/node/core/__tests__/completions.test.ts`: add `--total` to `LONG_FLAGS` and `-t` to `SHORT_FLAGS`.
- `package.json`: minor bump `0.11.0 → 0.12.0` (new output format addition — Constitution § Output Stability). 0.11.0 lives on the unmerged `260826-svlv` branch (PR #70); this change stacks on it.

### 6. Tests (Node built-in runner, co-located `__tests__/`)

- `src/node/core/__tests__/cli-parser.test.ts`: `-t` and `--total` both set `totalFlag: true`; both are stripped from `filteredArgs`; default is `false`.
- `src/node/core/__tests__/cli-exit-codes.test.ts` (or a unit test around the guard): `tu -t` (snapshot) and `tu cc mh -t` (single-tool) print `Warning: --total applies to all-tools history — ignoring.` on stderr and exit 0.
- `src/node/tui/__tests__/formatter-history.test.ts`: with `{ total: true }` the header line is `Date | Cost` (stripped) and contains no tool names; bars still present; longest bar on the highest-cost row; `Total` row contains only the grand total; footer line has no legend swatches; `{ total: true, metric: "tokens" }` header shows `Tokens` and value cells via `fmtNum`; `{ total: false }` and absent are byte-identical to today's output; month separators and `maxRows` still apply.
- `src/node/core/__tests__/cli-help.test.ts`: `FULL_HELP` contains a `--total / -t` line describing the collapse.

## Affected Memory

- `display/formatting`: (modify) add the `--total` collapsed pivot layout requirement (header/value/bar/Total/footer shape, tokens header under `--metric tokens`, compact no-op) and a Design Decision (branch inside `renderTotalHistory` vs. a new renderer; explicit flag vs. width-triggered)
- `cli/data-pipeline`: (modify) `--total` / `-t` flag parsing, `totalFlag` on `GlobalFlags`, the all-tools-history-only warn-and-clear guard, `withCap` stamping `FormatOptions.total`, silent no-op for JSON/CSV/MD

## Impact

- `src/node/core/cli.ts` — `GlobalFlags`, `parseGlobalFlags` (flag + skip list), `main()` guard + `withCap`, `FULL_HELP`
- `src/node/tui/formatter.ts` — `FormatOptions.total`, `renderTotalHistory` branch
- `src/node/core/completions.ts` — three shells
- Tests: `cli-parser.test.ts`, `cli-exit-codes.test.ts`, `cli-help.test.ts`, `completions.test.ts`, `formatter-history.test.ts`
- Docs: `README.md`, `docs/site/workflows.md`, `docs/site/skill.md`, `docs/specs/usage.md`, `docs/specs/layouts.md`
- `package.json` version 0.12.0
- No new fetch path or data type: all data flows through `UsageEntry` / existing aggregation and the existing `dispatchAllHistory[Lines]` calls. Watch mode gets the flag through the same `withCap` seam. Estimated ~60–90 LOC + tests.
- Merge adjacency: stacks on the unmerged `260826-svlv` branch (`--metric`, `withCap`) — expect trivial conflicts in `FULL_HELP`, `withCap`, and completions lists if that PR changes.

## Open Questions

- None blocking. Two soft choices are recorded as Confident assumptions rather than questions: the Markdown pivot stays uncollapsed under `--total` (#13), and the `Total` row under `--metric tokens` shows summed tokens (#12). Either is a one-line reversal via `/fab-clarify`.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Flag is `--total` with short alias `-t`, boolean, no value | Discussed — user explicitly asked for a short form; `-t` verified unused in `parseGlobalFlags` | S:95 R:85 A:95 D:95 |
| 2 | Certain | Scope is all-tools history only (`source === "all" && display === "history"`); elsewhere warn once on stderr and clear, exit 0 | Discussed; mirrors the existing `--since/--until`, `--full`, `--metric` guards at the same spot in `main()` | S:90 R:85 A:95 D:90 |
| 3 | Certain | Thread as `FormatOptions.total?: boolean` through the existing `withCap` merge; nothing stamped when false | Discussed; `withCap` already carries `capActive`/`metric`/`machineLegend` to one-shot and watch paths; keeps default output byte-identical | S:90 R:85 A:95 D:95 |
| 4 | Certain | Branch inside `renderTotalHistory`; no new renderer | Discussed; code-quality "minimum pathways"; the per-column-width machinery degrades to an empty tool list naturally | S:90 R:80 A:95 D:90 |
| 5 | Certain | Minor version bump `0.11.0 → 0.12.0` | Constitution § Output Stability — new output layout; precedent from prior pivot changes | S:85 R:90 A:100 D:95 |
| 6 | Certain | Help/README/docs-site/specs/completions updated in lockstep | Constitution § Toolkit Standards; shll readme-extraction cross-checks README vs `help-dump` | S:85 R:90 A:100 D:95 |
| 7 | Confident | Under `--metric tokens` the value column shows total tokens with header `Tokens` (via `fmtNum`), not cost | Discussed — bar needs a visible matching number; differs from the multi-column pivot whose cells stay cost-denominated | S:75 R:80 A:75 D:70 |
| 8 | Certain | Footer keeps `avg`/`this month`/`peak`/`p95`, drops tool legend swatches; bar is solid (unstacked) | Discussed; no tool columns to key swatch colors to | S:80 R:85 A:85 D:85 |
| 9 | Confident | `--total` is an ANSI-only option: silent no-op for `--json`/`--csv`/`--md` | Same rule as `--metric`; CSV is a positional contract; user did not ask for machine-format changes | S:60 R:85 A:80 D:70 |
| 10 | Certain | Compact mode (`< 60` cols) already renders date + cost; `--total` there is a no-op | Existing compact branch returns before the pivot; nothing to collapse | S:70 R:90 A:90 D:85 |
| 11 | Certain | Heading stays `📊 Combined Cost History (...)` unchanged | Discussed; no signal to rename; keeps Markdown/ANSI titles aligned | S:70 R:90 A:85 D:80 |
| 12 | Confident | Under `--metric tokens` the `Total` row shows the summed token total (not cost grand total) | Keeps the value column homogeneous with a `Tokens` header; the alternative (cost total under a Tokens header) reads inconsistently; one-line reversal | S:45 R:85 A:60 D:50 |
| 13 | Confident | `--total` does not collapse the Markdown pivot (`emitMarkdownTotalHistory` unchanged) | Kept ANSI-only for consistency with `--metric`; Markdown collapse is an easy follow-up if wanted | S:35 R:85 A:55 D:40 |
| 14 | Confident | `FULL_HELP` line reads `--total / -t         Collapse all-tools history to Date + total + bar (no per-tool columns)`, aligned to the existing flag column | Wording supplied in discussion; alignment follows the existing block | S:50 R:95 A:80 D:60 |

14 assumptions (9 certain, 5 confident, 0 tentative, 0 unresolved).
