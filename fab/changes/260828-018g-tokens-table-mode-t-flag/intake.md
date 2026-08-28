# Intake: Token Table Mode and the `-t` Short Flag

**Change**: 260828-018g-tokens-table-mode-t-flag
**Created**: 2026-08-28

## Origin

Promptless dispatch (`/fab-proceed` create-new, `{questioning-mode} = promptless-defer`) from a user conversation that followed change `260828-7x4i-cost-column-autosize-dim-zeros` (id `7x4i`, draft PR https://github.com/sahil87/tu/pull/73). This change is **stacked on 7x4i**: its branch forks from `260828-7x4i-cost-column-autosize-dim-zeros` (the current HEAD, containing `costColumnWidth()`, `costCell()`, `significantCostTools()` with `NEGLIGIBLE_COST_ABS`/`NEGLIGIBLE_COST_SHARE`, data-sized Cost/machine columns and dimmed exact-zero cells in `src/node/tui/formatter.ts`), and its PR MUST be opened with `--base 260828-7x4i-cost-column-autosize-dim-zeros`, not `main`. Synthesized description:

> **Problem.** `--metric tokens` exists but only rescales the **bars and footer stats** in history views: every numeric cell and the Cost column stay in dollars, it warns-and-ignores on snapshot views, and there is no way to see a token-denominated *table*. The user wants a first-class token view of every graph/table. Also, `--metric tokens` is too long to type for something this central.
>
> **Change 1 — `-t` short flag.** Add `-t` as a boolean shorthand for `--metric tokens`. Keep `--metric <cost|tokens>` as the explicit long form (scripts/docs); `-t` is sugar that sets the same option. Decided: NOT `-m <value>` — two summable facts (cost, tokens) want a toggle not a parameter, and `-m` is reserved for a future `--by-machine` short flag. Short flags already in use: `-j -f -w -i -u -s -V -v -h`; `-t` and `-m` are free. `-t` combined with an explicit `--metric cost` is a usage error (exit 2) — default: conflict → usage error, since the two say opposite things. Update `FULL_HELP`, `README.md`'s flag list, `docs/specs/usage.md`'s flag table + exit-code row. Check the CLI-surface/help/README change against `shll standards`.
>
> **Change 2 — token mode applies to every table cell in every view.** Pivot: per-tool cells, row total column, Total row show total tokens (`fmtNum`, no `$`); header `Cost` → `Tokens`. Single-tool history: keep the table shape, swap the last column's unit `Cost` → `Tokens` (user's decision — least surprising). Machine columns show per-machine tokens if the data exists. Snapshot: stop warning-and-ignoring; its columns are already tokens, so it renders unchanged. Compact/watch variants show tokens instead of cost; the watch delta indicator compares the displayed metric (prev map: key stays, value becomes the metric value). Width sizing reuses the 7x4i helpers generically (formatter or metric parameter); floor stays 9; dim-zero applies to `0` tokens; `significantCostTools` keys on the displayed metric (default) with sensible token constants. `--json`/`--csv`/`--md` unchanged. Cost mode byte-identical to today; token mode is a new layout → "requires minor release" in the PR body; do not edit `package.json`.
>
> **Out of scope:** a cost/tokens ratio (`$/Mtok`) column or footer stat (an *averageable* fact needing volume-weighted `Σcost/Σtokens` — separate future change); input/output/cache-read/cache-write as bar metrics; `-m`.

Decisions the conversation settled (carried as Certain rows below): `-t` as a boolean toggle rather than `-m <value>`; `-m` reserved; the single-tool history keeps its shape and swaps the last column's unit; snapshot in token mode renders unchanged and stops warning; emitters unchanged; the three out-of-scope items; the stacked-PR base.

## Why

1. **The pain point.** `tu -u all mh --metric tokens` today shows dollar cells next to token-scaled bars: the bar for a `$15,429.88` month is proportional to tokens while every number on the row is money, so the table cannot be read *as* a token table — the user has to trust the bars and ignore the cells. Cost is a derived, price-dependent fact (a model price cut halves the column overnight); tokens are the volume fact underneath. For capacity/usage discussions the volume fact is the one the team needs to read, cell by cell, with a Total row.
2. **Why now.** Change svlv introduced the metric seam (`FormatOptions.metric`, `barValue`, `fmtMetric`) but deliberately stopped at bars/footer to avoid a stability break (memory DD "Bar metric rides `FormatOptions`, defaulting to cost"). Change 7x4i then made every cost column data-sized, dimmed exact-zero cells and made column omission threshold-based — the exact helpers a token table needs, but all three are hard-wired to `fmtCost`/dollars. Generalising them over the displayed metric is now a small, mechanical step; delaying it means either a second render path later or another round of touching every renderer.
3. **Why `-t`.** `--metric tokens` is 15 characters for a toggle the user reaches for on most history invocations. Two summable facts want a boolean, not a parameter: `-t` reads as "tokens", `-m <value>` would burn the letter the future `--by-machine` short flag wants and force the user to type a value anyway. The long form stays for scripts and docs (toolkit principle №3 — help is a published contract; the sugar adds to it, removes nothing).
4. **Consequence of not fixing.** The half-applied metric is the worst of both: it is discoverable enough that users try it, then see a table whose numbers disagree with its bars. The snapshot warn-and-ignore compounds it — `tu -t` (the most natural first try) prints a warning instead of a table.
5. **Why extend the seam instead of a token renderer.** `code-quality.md` "minimum pathways": every renderer already receives `FormatOptions.metric`; the change is to make the *cell formatter*, *column width*, *zero-dim* and *significance* helpers take the metric, and to swap headers. One render path, one seam, cost output byte-identical (Constitution § Output Stability), token mode a new layout shipped under a minor release.

## What Changes

All rendering changes are in `src/node/tui/formatter.ts`; flag parsing, help, the snapshot guard, machine-map builders and the watch prev-map builder are in `src/node/core/cli.ts`; shell completions in `src/node/core/completions.ts`; docs in `README.md`, `docs/specs/usage.md`, `docs/specs/layouts.md`, `docs/site/skill.md`, `docs/site/workflows.md`. Tests co-located in `src/node/tui/__tests__/` and `src/node/core/__tests__/` (Node built-in runner, `npx tsx --test`).

### 1. `-t` short flag (`cli.ts`, `completions.ts`, help, docs)

**Parsing** (`parseGlobalFlags`, `src/node/core/cli.ts` ~871–1031):

- `-t` is a **boolean** flag: `const tokensShort = rawArgs.includes("-t");` and `"-t"` joins the boolean skip list on line ~901 (the `if (a === "--json" || a === "-j" || … ) continue;` chain) so it never reaches `filteredArgs`.
- Resolution, after the existing `--metric` validation block (~1017–1023):

  ```ts
  if (tokensShort) {
    if (hasMetricFlag && metricFlag === "cost") {
      console.error("Error: -t and --metric cost are incompatible");
      process.exit(EXIT_USAGE);
    }
    metricFlag = "tokens";
  }
  ```

  `-t` with `--metric tokens` is redundant and accepted silently. `-t` with `--metric cost` (either order) is a usage error, exit `2`, wording following the existing `Error: {flag-a} and {flag-b} are incompatible` pattern (Assumption 2). An invalid `--metric` value still fails first with its own message. `GlobalFlags` is unchanged — `-t` only sets `metricFlag`, there is no new field (the flag is sugar, not state).
- `--metric` and `-t` remain value-agnostic about display: the snapshot warn-and-ignore is **removed** (see §5), so the `-t` path has no display check.

**Help** (`FULL_HELP`, ~127–145): change the `--metric` line and add `-t` directly under it, keeping the 21-column description alignment:

```
  --metric <m>         Show 'cost' (default) or 'tokens' in every table cell, bar and footer stat
  -t                   Shorthand for --metric tokens
```

`README.md` ### Flags block (~66–83) mirrors the two lines verbatim (README rule 7 — command/flag accuracy against help-dump). `docs/site/workflows.md` line 85 example becomes `tu mh -u all -t   # same rows, every cell and bar in tokens` (keep the `--metric tokens` form in one place so both spellings are documented). `docs/site/skill.md` lines 101–103 ("History-only flags") drop `--metric` from the *history-only* sentence and gain: "`--metric tokens` / `-t` renders every table in tokens instead of cost (all displays)". `tu help-dump` embeds `FULL_HELP` as `root.text` (flat tree, the `tu` exception) — no structural change, no schema change.

**Completions** (`src/node/core/completions.ts`): add `-t` to the bash `short_flags` (line 24), zsh `short_flags` (line 82) and zsh `_arguments` list (`'-t[show tokens instead of cost]'` next to the `--metric` spec at line 99), and a fish line `complete -c tu -s t -d 'show tokens instead of cost'` beside line 201. `completions.test.ts` `SHORT_FLAGS` (line 68) gains `"-t"` — the test derives the fish `-s t` assertion from that array.

**Spec** (`docs/specs/usage.md`): Global Flags table row for `--metric` → Short `-t` (boolean, ≡ `--metric tokens`), Description: "Show cost (default) or total tokens in every table cell, bar and footer stat — all displays; no effect on `--json`/`--csv`/`--md`". Exit-code table row for data commands adds `-t` with `--metric cost` to the exit-`2` list (next to "bad/missing `--metric` value"). §Output Formats prose for the three tables gains a one-line "under `--metric tokens`/`-t` the cost cells/column render as total tokens" note.

**Toolkit standards check** (Constitution § Toolkit Standards): `shll standards` lists `principles`, `help-dump`, `readme-extraction`, `skill`, `update`, `version`, `shell-init`, `install-composition`, `config-home`. Governing this change: **principles** №3 (help is a published contract — the new flag appears in `FULL_HELP`, hence in `help-dump`'s `root.text`, README and the spec together), №4 (fail fast with an actionable error — the `-t`/`--metric cost` conflict exits 2 with a message naming both flags), №2 (the error goes to stderr); **help-dump** (`tu` exception: flat tree, `FULL_HELP` verbatim in `root.text`, exit 0/empty stderr — untouched structurally; the existing help-dump test keeps pinning it); **readme-extraction** rule 7 (README flag list must match help-dump — the two new lines are copied verbatim) and rule 5 (no new relative links); **skill** (the `docs/site/skill.md` briefing sentence is updated so `tu skill` output stays accurate — `SKILL_MD` is embedded from that file at build time, drift guard in `build/toolchain.md`). No standard governs short-flag letter choice; `-t` is free in `tu`.

### 2. Metric-generic helpers (`formatter.ts` top section, ~34–68)

Generalise the three 7x4i helpers so the same floor/data-size/dim logic serves token cells. `BarMetric` keeps its exported name (cli.ts imports it) but its comment becomes "the unit every table cell, bar and footer stat renders in" (Assumption 12). `fmtMetric(n, metric)` already exists (`fmtCost` under cost, `fmtNum(Math.round(n))` under tokens) and becomes the single cell formatter:

```ts
// Width of a right-aligned metric column sized to its data: floor COST_WIDTH
// (so small renders are byte-identical to a fixed 9-wide column), wide enough
// for the longest fmtMetric() among the values — including the Total-row value.
function metricColumnWidth(values: number[], metric: BarMetric): number {
  return Math.max(COST_WIDTH, ...values.map((v) => fmtMetric(v, metric).length));
}

// A metric data cell: pad first, then color (row()/colorRow() pad by raw string
// length). Exact-zero cells dim in either unit — 0 tokens is as much "no data"
// as $0.00. Total-row cells never go through here.
function metricCell(value: number, width: number, metric: BarMetric): string {
  const text = fmtMetric(value, metric).padStart(width);
  return value === 0 ? dim(text) : text;
}

function fmtMetricDelta(current: number, key: string, metric: BarMetric, prevCosts?: Map<string, number>): string {
  return fmtMetric(current, metric) + deltaIndicator(current, key, prevCosts);
}
```

`costColumnWidth`/`costCell` are **replaced**, not kept alongside (every call site passes the renderer's `metric`; under `"cost"` the output is byte-identical because `fmtMetric(v, "cost") === fmtCost(v)`). `fmtCostDelta` stays exported (tests import it) and becomes `fmtMetricDelta(current, key, "cost", prevCosts)`. `COST_WIDTH = 9` stays the floor for both units: `fmtNum` lengths for reference — `0` 1, `999,999` 7, `9,999,999` 9, `99,999,999` 10, `487,683,047` 11, `1,234,567,890` 13; a column with eight-figure token cells grows exactly as a five-figure cost column does today. `MACHINE_COL_WIDTH` (aliased to `COST_WIDTH`) is unchanged.

**Significance filter** (~627–660): generalise over the displayed metric so the visible table is self-consistent (Assumption 4):

```ts
const NEGLIGIBLE_COST_ABS = 1.0;        // dollars — omit below $1.00 …
const NEGLIGIBLE_TOKENS_ABS = 1_000;    // tokens  — omit below 1,000 tokens …
const NEGLIGIBLE_SHARE = 0.001;         // … or below 0.1% of the window grand total (either unit)

function negligibleAbs(metric: BarMetric): number {
  return metric === "tokens" ? NEGLIGIBLE_TOKENS_ABS : NEGLIGIBLE_COST_ABS;
}

// nonzeroTools / significantTools take the value map in the displayed metric.
function significantTools(toolNames, valueMap, labels, metric): string[] { /* same body, abs = negligibleAbs(metric), share = NEGLIGIBLE_SHARE */ }
```

`NEGLIGIBLE_COST_SHARE` is renamed `NEGLIGIBLE_SHARE` (one share constant for both units; memory/spec text updated). `nonzeroCostTools` → `nonzeroTools` (body unchanged — a generic value map). The Markdown emitter keeps calling the exact-zero filter over its cost map (emitters ignore the metric). Why key on the displayed metric: a tool with tokens but `$0.00` cost (a zero-priced model or a free tier) is a real column in token mode and noise in cost mode; keying on cost would drop a non-empty token column, keying on tokens would keep an all-`$0.00` cost column.

### 3. `renderTotalHistory` (cross-tool pivot, ~662–842) in token mode

Today the pivot keeps two maps: `costMap` (cells, Cost column, Total row — always cost) and `barMap` (bars/segments/footer — follows the metric). In token mode the **cells follow the metric too**, so the two maps coincide under both metrics and collapse into one:

- Replace `costMap`/`barMap` with a single `valueMap: tool → label → barValue(e, metric)` (`totalCost` under cost, `totalTokens` under tokens). `rowData.costs` → `values`, `rowCost` → `rowValue`, `toolSums`/`grandTotal` sum the metric value. Filter: `significantTools(allToolNames, valueMap, labels, metric)`.
- Widths: `toolWidths` use `fmtMetric(v, metric).length`; `costWidth = metricColumnWidth([...rowValues, grandTotal], metric)`.
- Cells: `metricCell(v, toolWidths[i], metric)` and `metricCell(rowValue, costWidth, metric)`; Total row `boldWhite(fmtMetric(sum, metric).padStart(w))`.
- Header: the last column header is `Cost` under cost and `Tokens` under tokens (`const valueHeader = metric === "tokens" ? "Tokens" : "Cost"`). Title: `📊 Combined Cost History (…)` under cost, `📊 Combined Token History (…)` under tokens (Assumption 7); `titleForTotalHistory` (Markdown) is untouched.
- Delta indicator: `deltaIndicator(rowValue, \`total:${label}\`, prevCosts, true)` — compares the displayed value against the prev map, whose values are now in the displayed metric (§6).
- Bars, stacked segments, legend and footer: unchanged (they already consume the metric values).

Under `metric === "cost"` every intermediate equals today's, so output is byte-identical (existing test `renderTotalHistory: metric: cost is byte-identical to no option` guards it; the test `bars and footer follow the metric; cells stay cost` at `formatter-history.test.ts` ~522 encodes the old half-applied contract and MUST be rewritten to assert cells are tokens — Constitution § Test Integrity, spec wins).

Compact pivot (`renderCompactTotalHistory`, ~890–905): the `costMap: label → number` built in the compact branch sums `barValue(e, metric)`; cells render via `fmtMetricDelta(value, \`total:${label}\`, metric, prevCosts)`; Total via `fmtMetric`. `COMPACT_COST_W = 12` fits `999,999,999` (11 chars) — no width change (Assumption 9).

### 4. `renderHistory` (single-tool history, ~362–510) in token mode

Table shape identical (Date | Input | Output | Cache Write | Cache Read | Total are already tokens). The **last column swaps unit** (user's decision — the bar, delta indicator and footer stay anchored to the last column):

- Header `Cost` → `Tokens` (`valueHeader` as above); the column value is `barValue(e, metric)` (= `totalTokens` under tokens). `sumCost` → `sumValue`; `costWidth = metricColumnWidth([...values, sumValue], metric)`; cells via `metricCell`; Total via `fmtMetric`. Delta: `deltaIndicator(value, \`${toolName}:${label}\`, prevCosts)`.
- Under tokens the last column duplicates the `Total` column's numbers. This is accepted knowingly (the user chose shape stability over dropping a column); it is recorded in Open Questions as a possible follow-up, not changed here.
- **Machine columns follow the metric** (Assumption 5): `FormatOptions.machineCosts` keeps its name and shape (`label/toolName → machine → number`) but its *values are in the displayed metric*. The builders in `cli.ts` gain a metric parameter — `buildHistoryMachineCosts(machineMap, metric)` sums `barValue(e, metric)` (line ~1283 currently `+ e.totalCost`), `buildSnapshotMachineCosts(toolKeys, allResults, period, metric)` and the inline single-tool snapshot builder (~1495–1501) pick `match.totalTokens` under tokens. `machineColWidth = metricColumnWidth(cells ∪ machine totals, metric)`; cells `metricCell`; Total `fmtMetric`. The legend line is unchanged. Import `barValue` into cli.ts? No — keep cli.ts free of formatter internals: export a tiny `metricValue(e: UsageTotals, metric)` from `formatter.ts` (rename of `barValue`, which today is `UsageEntry`-typed; `UsageTotals` is the supertype and the snapshot builders pass `UsageTotals`) and use it in both files.
- **JSON is unchanged**: `attachMachinesJson` MUST keep receiving a **cost** map. In `renderHistoryByFormat`/`renderSnapshotByFormat` (~1131–1137, ~1198–1204) the `json`/`csv`/`md` arms build the cost map (`metric = "cost"`) while the `table` arm builds the map in `fmtOpts.metric ?? "cost"` — or, simpler and preferred: call the builder once with the display metric for the table arm and once with `"cost"` for the emitter arms only when `metric !== "cost"` (under cost mode a single map serves all arms exactly as today).

Compact history (`renderCompactHistory`, ~873–888): cells `fmtMetricDelta(value, key, metric, prevCosts)`, Total `fmtMetric`; signature gains `metric`.

### 5. `renderTotal` (snapshot, ~518–618) in token mode

- **Remove the warn-and-ignore** in `main()` (`cli.ts` ~1668–1675): delete the block; `metricFlag` now reaches every display through `withCap` unchanged (the merge stamps `metric` only when non-default, so cost-mode output is byte-identical). `docs/memory/cli/data-pipeline.md`'s `--metric` bullet and `docs/specs/usage.md`'s row lose the "warns and is ignored on snapshots" clause. The `--since/--until` and `--full` guards are untouched.
- Full snapshot table: columns already token-denominated (`Tokens | Input | Output | Cache | Cost`), so **the table renders unchanged in token mode** — header, widths and the Cost column all stay (Assumption 6). The one metric-sensitive element is the watch **delta indicator**: today `fmtCostDelta(t.totalCost, name, prevCosts)` rides the Cost cell. In token mode the prev map holds tokens (§6), so the indicator moves to the **Tokens cell** — `fmtNum(t.totalTokens) + deltaIndicator(t.totalTokens, name, prevCosts)` — and the Cost cell is plain `fmtCost(t.totalCost)` (Assumption 8). Under cost mode the indicator stays on Cost, byte-identical. (The 12-wide numeric column absorbs the ` ↑` as it does today on Cost.)
- Machine columns follow the metric per §4 (`machineCosts` values in the displayed unit; `metricColumnWidth`/`metricCell`/`fmtMetric`).
- Compact snapshot (`renderCompactSnapshot`, ~854–871): cells `fmtMetricDelta(metricValue(t, metric), name, metric, prevCosts)`, Total `fmtMetric(grand, metric)`.
- The watch loading skeleton (`renderSkeleton`, `watch.ts`) mirrors the snapshot header, which is unchanged — no edit.

### 6. Watch-mode prev map holds the displayed metric (`cli.ts`)

`_lastRenderCostMap` is built by `buildCostMap(data, toolName?)` (~1353–1390) with `e.totalCost`/`t.totalCost` values and read back as `FormatOptions.prevCosts`. Renderers compare the value they display against the map, so in token mode the map MUST hold token values: `buildCostMap(data, metric, toolName?)` uses `metricValue(x, metric)` at each of its four `totalCost` sites; every `_lastRenderCostMap = buildCostMap(...)` call in the `*Lines` dispatchers (~1395–1530) passes `fmtOpts?.metric ?? "cost"`. Keys are unchanged (`{tool}:{label}`, `total:{label}`, `{tool}`), so `FormatOptions.prevCosts` keeps its name (the memory DD "Delta via callback" gains a sentence: values are in the displayed metric). `_lastRenderCost`/`getCost` (session cost, burn rate, projected daily in the stats grid) and `_lastRenderTotalTokens` are **untouched** — the stats grid stays cost-denominated in token mode (Assumption 10). `watch.ts` needs no change: it passes `formatOpts` through and `withCap` stamps the metric on every poll.

### 7. Docs, spec and memory touch points

- `docs/specs/usage.md`: flag row + short column, exit-code row, Output Formats notes (§1 above).
- `docs/specs/layouts.md`: Layout 3 and 4 gain a "Token mode (`--metric tokens` / `-t`)" bullet each (last column `Tokens`/`totalTokens`, pivot cells and Total in tokens, title `Combined Token History`, same floor/data-sizing/dim rules, negligible rule `< 1,000 tokens OR < 0.1%`); Layout 1 notes the table is unchanged in token mode with the delta indicator on the Tokens cell; Layout 5/compact notes cells show tokens; Layout 12 help mockup gains the two flag lines. Spec updates are hydrate's job (`/docs-hydrate-specs`); listed so the plan carries them.
- Memory (hydrate): see Affected Memory.
- Version: table layout change in token mode → Constitution § Output Stability → **requires a minor release** (`just release minor`), stated in the PR body; `package.json` is not edited (memory DD "Output-Stability version bumps happen at release time").
- PR: `--base 260828-7x4i-cost-column-autosize-dim-zeros` (stacked on PR #73); the PR body says so. The `/git-pr` skill defaults to `main` — the ship step MUST override the base.

## Affected Memory

- `display/formatting`: (modify) — the "History bars MUST scale on the field selected by `FormatOptions.metric`" bullet becomes "every table cell, bar and footer stat renders in the unit selected by `FormatOptions.metric`" (pivot cells/row column/Total/header `Tokens`/title `Combined Token History`; single-tool last column `Tokens` = `totalTokens`; machine columns in the displayed unit; snapshot table unchanged with the delta on the Tokens cell; compact variants in the metric; cost mode byte-identical; CSV/MD/JSON ignore the metric). The data-sized-column, dim-zero and negligible-omission bullets are re-stated over `metricColumnWidth`/`metricCell`/`significantTools` with the token constants (`NEGLIGIBLE_TOKENS_ABS = 1000`, shared `NEGLIGIBLE_SHARE = 0.001`). Design Decisions: revise "Bar metric rides `FormatOptions`, defaulting to cost" (the "only bars, stacked segments and the footer follow the metric" sentence is superseded — record why: the stability concern is now served by the cost default + minor release); revise "Delta via callback" (values in the displayed metric); add "Token mode swaps the last history column's unit rather than dropping it", "Snapshot table is metric-neutral; only its delta indicator follows the metric", "Significance filter keys on the displayed metric", "Machine columns follow the displayed metric; JSON keeps cost".
- `cli/data-pipeline`: (modify) — the Global-flags bullet adds `-t` (boolean, ≡ `--metric tokens`; conflict with `--metric cost` exits 2); the `--metric` bullet drops the snapshot warn-and-ignore and states the flag reaches every display via `withCap`; the Exit-Code Convention bullet's usage-site count grows by one (`parseGlobalFlags` `-t`/`--metric cost` conflict) and the exit-`2` list names it; the `tu shell-init` bullet's "every long/short flag" now includes `-t`; `buildCostMap`/machine-map builders take the metric; the JSON `machines` key stays cost.
- `watch-mode/tui`: (modify, only if it documents `prevCosts`/delta semantics — check at hydrate; otherwise no change) — prev map values are in the displayed metric; the stats grid stays cost.

## Impact

- **Code**: `src/node/tui/formatter.ts` — helpers `metricColumnWidth`, `metricCell`, `fmtMetricDelta`, `metricValue` (renamed `barValue`, `UsageTotals`-typed, exported), `nonzeroTools`, `significantTools` + `NEGLIGIBLE_TOKENS_ABS`/`NEGLIGIBLE_SHARE`; `renderTotalHistory` (single `valueMap`, header/title swap), `renderHistory` (last column unit, machine columns), `renderTotal` (delta placement, machine columns), the three compact renderers (metric parameter). `src/node/core/cli.ts` — `parseGlobalFlags` (`-t`, conflict), `FULL_HELP`, `main()` (delete the snapshot guard), `buildCostMap`/`buildHistoryMachineCosts`/`buildSnapshotMachineCosts` + inline snapshot builder (metric parameter), `renderHistoryByFormat`/`renderSnapshotByFormat` (cost map for emitters). `src/node/core/completions.ts` — `-t` in bash/zsh/fish. `watch.ts`, `compositor.ts`, `panel.ts`, emitters (`emitCsv*`, `emitMarkdown*`), `colors.ts`: not touched.
- **Tests**: `src/node/tui/__tests__/formatter-history.test.ts` — rewrite the `FormatOptions.metric` block (~479–536): pivot cells/Total/header/title in tokens, single-tool last column `Tokens` with `totalTokens`, cost-mode byte-identity (keep), compact variants in tokens, `0` token cells dimmed, token column width grows past 9 for eight-figure cells with aligned bar starts, significance filter over tokens (`999` omitted, `1,000` kept, `< 0.1%` omitted, a `$0.00`-but-tokens column kept in token mode), machine columns in tokens, snapshot unchanged in token mode except the delta on the Tokens cell, `--no-color` byte-identity for dimmed token zeros. `src/node/core/__tests__/cli-parser.test.ts` — `-t` sets `metricFlag: "tokens"` and is stripped; `-t --metric tokens` accepted; `-t --metric cost` → exit 2 (via the subprocess harness in `cli-exit-codes.test.ts` with the exact message). `cli-help.test.ts` — `FULL_HELP` has a `-t` line mentioning `--metric tokens`. `completions.test.ts` — `SHORT_FLAGS` gains `-t`. A snapshot-with-`-t` subprocess test asserting **no** warning on stderr (the removed guard). Run scoped: `npx tsx --test src/node/tui/__tests__/formatter*.test.ts src/node/core/__tests__/cli-parser.test.ts src/node/core/__tests__/cli-exit-codes.test.ts src/node/core/__tests__/cli-help.test.ts src/node/core/__tests__/completions.test.ts`.
- **Behaviour/compat**: with the metric absent or `cost`, every ANSI, JSON, CSV and Markdown byte is unchanged (guarded by the existing byte-identity tests, extended to snapshot/compact). Token mode is a new layout → minor release. Stderr: `tu -t` on a snapshot no longer warns. Watch mode: delta arrows track the displayed unit; the stats grid stays in dollars.
- **Docs**: `README.md`, `docs/specs/usage.md`, `docs/specs/layouts.md`, `docs/site/skill.md`, `docs/site/workflows.md`, memory (hydrate).
- **Stacking**: branch from and PR against `260828-7x4i-cost-column-autosize-dim-zeros`; if 7x4i is merged first, rebase onto `main` and retarget the PR.

## Open Questions

- Under tokens the single-tool history's last column duplicates the `Total` column. The user chose shape stability; a follow-up could drop the last column in token mode (or replace it with the `$/Mtok` ratio the user deferred). Not changed here.
- Should the snapshot's Cost column be hidden in token mode for a "pure" token table? Default here: no — the snapshot is already token-denominated and the Cost column is useful context (Assumption 6).
- Is `NEGLIGIBLE_TOKENS_ABS = 1,000` the right absolute floor, or should it be derived (e.g. the token equivalent of `$1.00` at a reference price)? Default: a fixed named constant, trivially tunable (Assumption 4).

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | `-t` is a boolean shorthand that sets `metricFlag = "tokens"`; `--metric <cost or tokens>` stays the explicit long form; no `-m`; no new `GlobalFlags` field | Discussed — user chose a toggle over `-m <value>` and reserved `-m` for `--by-machine`; `-t` is unused (`-j -f -w -i -u -s -V -v -h` taken) | S:95 R:90 A:95 D:95 |
| 2 | Confident | `-t` combined with an explicit `--metric cost` is a usage error: `Error: -t and --metric cost are incompatible`, exit `2`; `-t --metric tokens` is accepted silently | Deferred — promptless dispatch. The description offered conflict→error (default) or last-wins; the two flags assert opposite units and every other contradictory flag pair in `parseGlobalFlags` exits 2 with the `X and Y are incompatible` wording (toolkit principle №4). Reversible to last-wins in one branch | S:70 R:90 A:80 D:70 |
| 3 | Certain | In token mode every table cell that shows cost today shows total tokens via `fmtNum` (no `$`): pivot per-tool cells, row column, Total row and header `Tokens`; single-tool history keeps its shape and swaps the last column's unit to `Tokens` = `totalTokens`; compact variants show tokens; `--json`/`--csv`/`--md` unchanged | Discussed — user's explicit decisions, including "keep table shape, swap the last column's unit — least surprising" | S:95 R:85 A:90 D:90 |
| 4 | Confident | The pivot significance filter keys on the **displayed metric**: keep iff `total ≥ negligibleAbs(metric) AND total ≥ NEGLIGIBLE_SHARE × grand`, with `NEGLIGIBLE_TOKENS_ABS = 1_000`, `NEGLIGIBLE_COST_ABS = 1.0`, one shared `NEGLIGIBLE_SHARE = 0.001`; fallback chain unchanged (exact-nonzero in the same metric → full list) | Deferred — promptless dispatch. Description's default is "key on the displayed metric so the visible table is self-consistent"; a zero-priced tool with real tokens must appear in token mode and vanish in cost mode. The 1,000-token floor mirrors the `$1.00` floor's role (tiny windows only — the share rule does the real work); a named constant is a one-line tune | S:60 R:90 A:70 D:65 |
| 5 | Confident | Machine columns (`--by-machine`, `renderHistory` and `renderTotal`) follow the displayed metric: the `cli.ts` builders take the metric and read `totalTokens` from the per-machine `UsageEntry`s; `FormatOptions.machineCosts` keeps its name/shape with values in the displayed unit; JSON `machines` stays cost (emitter arms receive a cost map) | Deferred — promptless dispatch. The description conditioned this on the data existing — it does (`machineMap: Map<string, UsageEntry[]>` carries full entries; only the derived `machineCosts` map is cost-valued). A cost-denominated machine block inside a token table would be the same half-applied inconsistency this change removes | S:60 R:85 A:85 D:75 |
| 6 | Confident | Snapshot (`renderTotal`) in token mode renders its table unchanged (columns are already `Tokens/Input/Output/Cache/Cost`) and the `main()` warn-and-ignore is removed; the Cost column is kept as context | Deferred — promptless dispatch. Description's stated pick. Dropping the Cost column would be a second snapshot layout for no new information; removing the warning makes `tu -t` (the natural first try) just work | S:75 R:90 A:85 D:75 |
| 7 | Confident | Pivot title reads `📊 Combined Token History (…)` in token mode (cost mode keeps `Combined Cost History`); Markdown titles unchanged | Deferred — promptless dispatch. Not named by the user; a token table titled "Cost History" contradicts its own header. Title text is one ternary and has no parser downstream (CSV/JSON have no heading) | S:45 R:95 A:85 D:75 |
| 8 | Confident | Watch prev map (`_lastRenderCostMap` → `FormatOptions.prevCosts`) holds values in the displayed metric with unchanged keys; renderers compare the value they display. In the full snapshot under tokens the delta indicator therefore rides the **Tokens** cell and the Cost cell is plain | Description prescribes "key stays, value becomes the metric value". Given that, the only self-consistent snapshot placement is the Tokens cell — comparing a dollar cell against a token map would be a bug, and building two maps per poll for one arrow is a second pathway. Cost mode byte-identical | S:65 R:85 A:85 D:75 |
| 9 | Certain | Compact renderers keep `COMPACT_COST_W = 12` for both units | `fmtNum(999_999_999)` is 11 chars; compact mode is a <60-col watch view where a billion-token day is not a realistic cell. No width change keeps cost mode byte-identical | S:60 R:95 A:90 D:85 |
| 10 | Confident | The watch stats grid (`getCost`/`_lastRenderCost`: session delta, burn rate, projected daily) stays cost-denominated in token mode; only the table and its delta arrows follow the metric | Description scopes the change to table cells/graphs; the grid's `Tok/min` already exists and `Rate`/`Proj. day` are inherently `$/hr`/`$`. Out of scope unless asked | S:55 R:90 A:85 D:80 |
| 11 | Certain | `costColumnWidth`/`costCell` are replaced by `metricColumnWidth(values, metric)`/`metricCell(value, width, metric)` (floor `COST_WIDTH = 9`, exact-`0` dims in either unit); `fmtCostDelta` stays as a thin wrapper over `fmtMetricDelta`; one render path — no token renderer | Description: "reuse the A-change helpers generically"; `code-quality.md` minimum pathways; `fmtMetric(v,"cost") === fmtCost(v)` guarantees byte-identity | S:90 R:90 A:95 D:90 |
| 12 | Confident | The exported type keeps the name `BarMetric` (comment updated to "the unit every cell, bar and footer stat renders in"); `barValue` is renamed/exported as `metricValue(e: UsageTotals, metric)` for reuse by `cli.ts` builders | Renaming the type touches `cli.ts` imports and memory text for no behavioural gain; `barValue` must widen its parameter type anyway (snapshot builders pass `UsageTotals`), so its rename is free. Either choice is a mechanical rename | S:50 R:95 A:85 D:70 |
| 13 | Certain | Cost mode (metric absent or `cost`) is byte-identical across ANSI/JSON/CSV/MD; token mode is a new layout → "requires minor release" in the PR body, `package.json` untouched | Constitution § Output Stability + memory DD "Output-Stability version bumps happen at release time" (3tah); `withCap` stamps nothing at defaults | S:90 R:95 A:100 D:95 |
| 14 | Certain | Stacked change: branch forks from `260828-7x4i-cost-column-autosize-dim-zeros` (current HEAD) and the PR opens with `--base 260828-7x4i-cost-column-autosize-dim-zeros`, noted in the PR body | Discussed — user stated the stacking and the base explicitly; the code it builds on exists only on that branch | S:95 R:85 A:95 D:95 |
| 15 | Certain | Out of scope: `$/Mtok` ratio column/footer stat, input/output/cache-read/cache-write as bar metrics, `-m` | Discussed — user explicitly deferred these (ratio is an averageable fact needing `Σcost/Σtokens`) | S:95 R:95 A:95 D:95 |
| 16 | Certain | Toolkit-standards check performed: `shll standards` → principles №2/№3/№4, help-dump (`tu` flat-tree exception, `FULL_HELP` verbatim), readme-extraction rule 7 (README flags mirror help), skill (`docs/site/skill.md` sentence updated); no standard constrains the short-flag letter | Constitution § Toolkit Standards; `shll standards` was available and read | S:85 R:95 A:95 D:95 |
| 17 | Certain | The existing test `renderTotalHistory: bars and footer follow the metric; cells stay cost` (`formatter-history.test.ts` ~522) is rewritten to the new contract; new tests co-located in `src/node/tui/__tests__/` and `src/node/core/__tests__/` on the Node built-in runner; `completions.test.ts` `SHORT_FLAGS` gains `-t` | Constitution § Test Integrity (spec is the truth), § Test Runner, § Test Location; `code-review.md` "CLI output changes SHOULD include test coverage" | S:85 R:95 A:100 D:95 |
| 18 | Certain | `-t` is added to bash/zsh/fish completion scripts in `completions.ts` alongside the existing short flags | Memory: the completion scripts "cover … every long/short flag"; `completions.test.ts` derives per-shell assertions from `SHORT_FLAGS`, so omitting it would fail the standing contract | S:60 R:95 A:95 D:90 |

18 assumptions (11 certain, 7 confident, 0 tentative, 0 unresolved).
