# Intake: Cost Column Autosize, Dim Zeros, Negligible-Column Omission

**Change**: 260828-7x4i-cost-column-autosize-dim-zeros
**Created**: 2026-08-28

## Origin

Promptless dispatch (`/fab-proceed` create-new, `{questioning-mode} = promptless-defer`) from a user conversation about a screenshot of `tu -u all mh` — the cross-tool pivot "Combined Cost History (monthly)" view. Synthesized description:

> **Problem observed**: the Cost column's decimal points do not line up, and the stacked bars start at different columns on different rows. Root cause: `COST_WIDTH = 9` in `src/node/tui/formatter.ts` is a fixed width sized for `$9,999.99`. Five-figure monthly costs (`$15,429.88`, 10 chars) and the six-figure grand total (`$270,191.65`, 11 chars) overflow the cell — `padStart` is a no-op — shifting the bar start by 1–2 chars on those rows and misaligning the Total row. The code comment describes this case as "far beyond" typical; at ~$90k/month it is now routine.
>
> **Change 1 — data-sized Cost column.** Compute the Cost column width as `max(COST_WIDTH, longest fmtCost() among row costs and the grand total)` before `barWidth` is derived, and use it for header, divider, every row cell and the Total cell. `renderHistory` has the same latent bug (Cost + machine columns) — fix the same way.
>
> **Change 2 — dim `$0.00` cells** with the existing `dim()` helper; cell text only, width unchanged, respects `--no-color`. Do not dim the Total row.
>
> **Change 3 — omit negligible tool columns.** Extend `nonzeroCostTools()` so columns whose visible-window total is < $1.00 OR < 0.1% of the grand total are omitted (Gemini survives today with $0.04 and costs ~12 chars of bar width). Keep the all-omitted fallback. Omitted tools still count in the Cost column, Total row, grand total, bars, footer. `--json`/`--csv`/`--md` unchanged.
>
> **Out of scope**: Kimi swatch colour; footer avg computation; a tokens display mode for table cells (follow-up change B stacked on this one).

Decisions the conversation settled (carried as Certain/Confident rows below): the negligible thresholds (`< $1.00 OR < 0.1%` of the window grand total), the "zeros are noise" intent behind dimming, the Total row staying bold and undimmed, and the three explicit out-of-scope items.

## Why

1. **The pain point.** The pivot is the primary view for an org-wide monthly readout (`tu -u all mh`). Cost cells are right-aligned by `padStart(COST_WIDTH)`; once a value exceeds 9 characters the pad is a no-op and the cell grows to the right. Rows with `$15,429.88` (10 chars) shift their bar by one column, the `$270,191.65` Total (11 chars) shifts by two, and the decimal points in the Cost column no longer form a vertical line. A misaligned bar chart defeats the reason the bars exist (visual comparison of row magnitudes), and the Total row visibly disagrees with the column it sums.
2. **Why now.** The `MIN_TOOL_COL_WIDTH` comment (formatter.ts ~94–102) argues five-figure cells are "far beyond any realistic single-day/tool cost". That was true for a single user's daily view; the all-users monthly aggregate (`-u all`, change svlv) routinely produces five-figure monthly rows and a six-figure Total. The assumption the layout rests on no longer holds for a supported invocation.
3. **Consequence of not fixing.** Every org-scale monthly render is misaligned; the misalignment worsens as costs grow (seven-figure totals would shift by three). Because `barWidth` is derived from the fixed `COST_WIDTH`, an overflowing cell also makes the longest-bar row wider than the terminal budget, which is the exact wrap class that corrupts the watch-mode compositor's line counting (memory: `display/formatting.md`, gmcp).
4. **Why data-sizing over a bigger constant.** Raising `COST_WIDTH` to 11 would fix today's screenshot but permanently steal two bar characters from every small-cost user and would break again at `$1,000,000.00`. Sizing from the rendered data (floored at 9) keeps today's byte-identical output for everyone whose costs fit, grows only when needed, and never overflows. The pivot already sizes tool columns from data (`toolWidths`), so the pattern is established.
5. **Why dim zeros and drop negligible columns in the same change.** Both serve the same readability goal surfaced by the same screenshot: with six registered tools and org-wide data, most cells are `$0.00` or near-zero, and a `$0.04` Gemini column costs 12 characters of bar area. Dimming makes real numbers stand out at zero width cost; the negligible rule reclaims bar width the same way the existing exact-zero omission does (q6fx precedent).

## What Changes

All code changes are in `src/node/tui/formatter.ts`; tests in `src/node/tui/__tests__/formatter.test.ts` (existing file; add cases there or in a co-located sibling following the `formatter-*.test.ts` naming already present).

### 1. Data-sized cost columns (the fix)

**Shared helper** (new, module-private, placed near `fmtCost`):

```ts
// Width of a right-aligned cost column sized to its data: never narrower than
// COST_WIDTH (so small-cost renders are byte-identical to today), wide enough
// for the longest fmtCost() among the values it will hold — including the
// Total-row value, which is usually the longest.
function costColumnWidth(values: number[]): number {
  return Math.max(COST_WIDTH, ...values.map((v) => fmtCost(v).length));
}
```

`COST_WIDTH = 9` stays as the **floor** constant (rename not required; update its comment to say "floor" rather than "fits"). `fmtCost` output lengths for reference: `$0.00` 5, `$999.99` 7, `$9,999.99` 9, `$99,999.99` 10, `$999,999.99` 11, `$9,999,999.99` 13.

**`renderTotalHistory`** (~579–740). Today the order is: filter tools → `toolWidths` → `tableWidth` → `barWidth` → header → *then* the `rowData` loop computes `rowCost`/`grandTotal`. Reorder so the cost data exists before the width budget:

1. Build `rowData` (per-label `rowCost`, `barTotal`, `cells`, `toolBars`) and `grandTotal` **before** computing `barWidth`. The loop has no dependency on `barWidth` or the header, so this is a pure move.
2. `const costWidth = costColumnWidth([...rowData.map((r) => r.rowCost), grandTotal]);`
3. `barWidth = Math.min(width - tableWidth - GUTTER - costWidth - 1 - indicatorReserve, MAX_BAR_WIDTH)` — replaces `COST_WIDTH` in the budget.
4. Use `costWidth` in all four places that currently use `COST_WIDTH`: `costDiv` (`"─|─" + "─".repeat(costWidth)`), `costHeader` (`"Cost".padStart(costWidth)`), each row's `costBase`, and the Total row's `totalCost`.

Result: every row's cost cell has identical visible width, so `indicator` and the bar begin at the same column on every row, and the Total row's decimal point aligns with the rows above it.

**Per-tool pivot columns** (same bug class, same fix — see Assumption 4): a tool column is currently `max(name.length, MIN_TOOL_COL_WIDTH)`. `Codex` (5 → 9) overflows at `$10,000.00` exactly as the Cost column does, and an overflowing tool cell shifts *every* column to its right, including Cost and the bar. Extend `toolWidths` to `max(name.length, MIN_TOOL_COL_WIDTH, longest fmtCost() in that column including its Total-row sum)`. This needs the per-tool sums (`toolSums`) computed before `toolWidths`, which the reorder in step 1 already provides. Update the `MIN_TOOL_COL_WIDTH` comment: the floor is 9; columns grow with their data; the "96 chars for the full 6-tool row" figure becomes the **minimum** full-row width (holds whenever every cell is ≤ `$9,999.99`).

**`renderHistory`** (~340–468):

1. `const costWidth = costColumnWidth([...entries.map((e) => e.totalCost), sum of entries' totalCost]);` — computed before `barWidth`.
2. Machine columns: one shared width for all machine columns, `machineColWidth = costColumnWidth(every per-label per-machine cost ∪ every per-machine total)`, floored at `MACHINE_COL_WIDTH` (= `COST_WIDTH`, unchanged as the floor). Shared rather than per-column keeps the letter-coded columns visually uniform (see Assumption 5). This requires summing `machineSums` before the row loop — compute the machine sums in a pre-pass (or fold them into the same pre-pass that collects costs).
3. `machineColsWidth = mcols.length * (machineColWidth + 3)`; `barWidth = Math.min(width - tableWidth - GUTTER - costWidth - machineColsWidth - 1, MAX_BAR_WIDTH)`.
4. Replace `COST_WIDTH` in `costDiv`, `costHeader`, row `costBase`, Total `totalCost`; replace `MACHINE_COL_WIDTH` in `machineDiv`, `machineHeader`, row `machineCells`, Total `totalMachineCells`.

**`renderTotal`** (snapshot, ~476–560) — machine columns only (Assumption 6): the snapshot's own Cost cell lives in a fixed 12-wide numeric column (fits `$999,999.99`) and is untouched. Its machine columns use the fixed `MACHINE_COL_WIDTH` and have the identical overflow at `$10,000.00` per user/machine — realistic for `tu -u all m --by-machine` at org scale. Apply the same shared `machineColWidth` (sized over all machine cells + machine totals, floored at 9). The watch skeleton (`renderSkeleton` in `watch.ts`) mirrors the base column set only, not machine columns, so it is unaffected.

**Invariant to test**: after `stripAnsi`, every data row and the Total row have the cost cell's `.` at the same string index, and the bar's first block (or the `┊` rule) at the same index. Use fixtures with a `$15,429.88`-class row, a sub-`$1,000` row, and a `$270,191.65`-class grand total in one table; and a small-cost fixture asserting output is byte-identical to the current 9-wide render (floor behaviour).

### 2. Dim `$0.00` data cells

A zero-cost **data cell** renders its already-padded text through `dim()`:

```ts
// Pad first, then color: the row()/colorRow() builders pad by string length,
// which would count ANSI bytes — a pre-padded cell is a no-op for padStart.
function costCell(cost: number, width: number): string {
  const text = fmtCost(cost).padStart(width);
  return cost === 0 ? dim(text) : text;
}
```

Apply to:

- `renderTotalHistory`: each per-tool cell in `r.cells` (currently `fmtCost(cost)` — must become pre-padded to `toolWidths[i]` so the `row()` padStart is a no-op, exactly as `labelCell` already relies on) and the row's Cost cell (`costBase`). A row whose total is `$0.00` is a zero data cell and dims — "zeros are noise".
- `renderHistory`: the row Cost cell and each machine cell.
- `renderTotal`: each machine cell (Assumption 6). The tool row's own Cost cell goes through `fmtCostDelta` and is **not** changed (tool rows with zero tokens are already omitted, so a `$0.00` there is rare).

Never dimmed: the **Total row** (all cells stay `boldWhite`, including a `$0.00` per-tool sum — which cannot occur under the omission rule, but the rule is "Total row untouched"), the header, dividers. Only exact `=== 0` dims; `$0.00` produced by rounding a sub-cent nonzero cost (e.g. `0.004`) is **not** dimmed (Assumption 7).

Width/alignment: `dim` adds only escape bytes; `stripAnsi` length is unchanged. `--no-color`/`NO_COLOR`: `dim` returns its input, so output is byte-identical to today. Watch mode: the delta indicator is appended after the (possibly dimmed) Cost cell; `dim`'s reset does not affect it.

### 3. Omit negligible tool columns (ANSI pivot only)

Replace the ANSI pivot's exact-zero filter with a significance filter; keep the exact-zero helper for Markdown (Assumption 3):

```ts
// Human-facing pivot: a tool column is worth its ~12 chars of bar area only
// when its visible-window total is significant.
const NEGLIGIBLE_COST_ABS = 1.0;     // dollars — omit below $1.00 …
const NEGLIGIBLE_COST_SHARE = 0.001; // … or below 0.1% of the window grand total

function significantCostTools(toolNames: string[], costMap, labels: string[]): string[] {
  const totals = new Map(toolNames.map((t) => [t, sum over labels of costMap.get(t)?.get(label) ?? 0]));
  const grand = sum of totals.values();
  const kept = toolNames.filter((t) => {
    const total = totals.get(t)!;
    return total >= NEGLIGIBLE_COST_ABS && total >= NEGLIGIBLE_COST_SHARE * grand;
  });
  if (kept.length > 0) return kept;
  return nonzeroCostTools(toolNames, costMap, labels); // existing helper: nonzero → else all
}
```

Rules:

- **Keep** iff `total >= $1.00 AND total >= 0.1% × grand`. Boundary values (`$1.00` exactly; exactly 0.1%) are kept.
- **Fallback chain** (Assumption 8): when everything would be omitted (e.g. a window whose grand total is `$0.40`), fall back to the existing exact-zero filter, then to the full registry list — the current defensive behaviour, preserved.
- **Window**: totals and the grand total are computed over the visible labels (post-`maxRows`), matching the existing filter's decision (q6fx). In watch mode a tool crossing a threshold gains/loses its column on the next frame; the compositor re-measures every frame (existing precedent).
- **What omission affects**: only the visible per-tool column and its legend swatch (`legend` is built from the filtered `toolNames`; `stackedBarPalette(toolNames.length)`).
- **What omission does NOT affect** — and this is the behavioural change inside the loop: `rowCost`, `grandTotal`, `barTotal`, and the footer must sum over **all** registry tools (`allToolNames`), not the filtered `toolNames`. Today the loop sums only visible tools, which is equivalent under exact-zero omission but wrong under negligible omission (a `$0.04` Gemini must still be in the row's Cost). Concretely: iterate `allToolNames` for `rowCost`/`barTotal`/`grandTotal`; iterate the filtered `toolNames` for `cells`/`toolBars`/`toolSums`.
- **Bars**: bar *length* comes from `barTotal` (all tools). Segments come from `toolBars` (visible tools only); `apportionSegments` normalises by the sum of the shares it receives, so the omitted tool's sub-0.1% share is absorbed proportionally into the visible segments — no code change needed there, but assert it in a test (segments still sum to the unstacked bar length).
- **Emitters unchanged**: `emitMarkdownTotalHistory` keeps calling `nonzeroCostTools` (exact-zero omission, existing behaviour); CSV keeps every column; JSON is untouched.

**Tests that must change** (they encode the old rule): `formatter.test.ts` "6-tool pivot full data row … is 96 chars" (~483) and "6-tool watch-mode pivot: no line exceeds terminal width" (~550) use fixture costs `0.12 / 0.01 / 0.02 / 0.03`, which the new rule omits. Raise every fixture to `≥ $1.00` and `≥ 0.1%` of the row total while keeping each cell `≤ $9,999.99` (e.g. `123.45 / 12.34 / 4.56 / 1.23 / 2.34 / 3.45`, row total `$147.37`) so the 96-char assertion remains valid; update the `$128.19` literals accordingly. The omission tests at ~609–693 remain valid (they use exact zeros or `$1`+ values) — add cases for: `$0.99` omitted, `$1.00` kept, a `$5` tool against a `$10,000` grand total omitted (0.05%), the omitted cost still present in the row Cost/Total/footer, and the two-level fallback.

### 4. Comments, spec and memory touch points

- `COST_WIDTH` / `MIN_TOOL_COL_WIDTH` comments (formatter.ts ~93–103): rewrite — 9 is a floor, columns grow with data, five-figure monthly cells are routine under `-u all`.
- `docs/specs/layouts.md`: Layout 3 "Cost (9 right-aligned — fits `$9,999.99`)" and Layout 4's "9-wide Cost cell … 96 chars" and "Zero-column omission" bullets need the floor/data-sized wording, the dim-zero rule, and the negligible rule. Spec updates are hydrate's (`/docs-hydrate-specs`) job; listed here so the plan carries them.
- Version: table layout changes fall under Constitution § Output Stability → **requires a minor release** (`just release minor`), noted in the PR body; `package.json` is not edited in the feature diff (memory DD "Output-Stability version bumps happen at release time").

## Affected Memory

- `display/formatting`: (modify) — Requirements bullets on `COST_WIDTH`/`MACHINE_COL_WIDTH` (9 becomes a floor; cost columns sized from data incl. the Total value), the pivot width contract (96 chars becomes the minimum full-row width; per-tool columns also grow with data), the zero-column omission bullet (ANSI pivot uses the negligible rule `< $1.00 OR < 0.1%`, two-level fallback, Markdown keeps exact-zero omission, CSV unfiltered), a new bullet for dimmed `$0.00` data cells (which renderers/cells, Total row exempt, `--no-color` byte-identical), and the row-total/bar/footer "sum over all tools, cells over visible tools" invariant. Design Decisions: add entries for data-sized-with-floor vs a larger constant, shared machine-column width, negligible thresholds and why Markdown is left on the exact-zero rule, exact-zero-only dimming. The gmcp DD's "96-char full row" and "97 in watch mode" figures should be re-stated as minimums.

## Impact

- **Code**: `src/node/tui/formatter.ts` — `renderTotalHistory` (reorder data pass before width budget; `costWidth`; data-sized `toolWidths`; sums over all tools; dim cells; `significantCostTools`), `renderHistory` (`costWidth`, shared `machineColWidth`, dim cells), `renderTotal` (machine columns: shared width + dim), new helpers `costColumnWidth`, `costCell`, `significantCostTools` + two named threshold constants; comment rewrites. `MACHINE_COL_WIDTH` stays exported (only formatter.ts consumes it) as the floor.
- **Tests**: `src/node/tui/__tests__/formatter.test.ts` — update the two 6-tool fixtures (~483, ~550) and the `COST_WIDTH` comments (~189, ~235, ~707); add alignment tests (5-/6-figure costs: identical `.` index and bar-start index across rows + Total; floor byte-identity), dim tests (escape present on `$0.00` data cells, absent on Total row, absent under `setNoColor`), negligible-omission tests (thresholds, boundaries, sums-over-all-tools, fallback chain, legend/palette follow the filtered set, Markdown still exact-zero). `formatter-stacked.test.ts` may gain the segments-sum invariant with an omitted tool. Run scoped: `npx tsx --test src/node/tui/__tests__/formatter*.test.ts`.
- **Behaviour/compat**: byte-identical output whenever every cost cell is ≤ 9 chars, no cell is `$0.00`, and no tool is negligible — i.e. most single-user daily renders. Org-scale monthly renders change layout (wider Cost column, fewer columns) → minor release. CSV/JSON/Markdown output unchanged. Watch mode: wider Cost column reduces `barWidth` by 1–2 chars on large-cost windows; the `indicatorReserve` budget still holds because `barWidth` now subtracts the real width.
- **Docs**: `docs/specs/layouts.md` (Layouts 3, 4), `docs/memory/display/formatting.md` (hydrate).
- **Not touched**: `watch.ts`, `compositor.ts`, emitters, `colors.ts`, `STACK_PALETTE`, footer maths.

## Open Questions

- Should the negligible rule also apply to the Markdown pivot emitter for human-facing consistency, or stay on the exact-zero rule as the description's "emitters unchanged" implies? (Deferred; default: unchanged — see Assumption 3.)
- Should per-tool pivot columns be data-sized in this change (same bug class, user did not name it), or left for a follow-up? (Deferred; default: include — see Assumption 4.)
- Should `renderTotal`'s machine columns get the same width/dim treatment (not named in the description; same overflow at `$10,000`)? (Deferred; default: include — see Assumption 6.)

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Negligible thresholds are `total < $1.00 OR total < 0.1% of the visible-window grand total`; keep iff `≥ $1.00 AND ≥ 0.1%`; boundaries kept | Discussed — user agreed these exact values; encoded as named constants `NEGLIGIBLE_COST_ABS` / `NEGLIGIBLE_COST_SHARE` | S:95 R:90 A:90 D:95 |
| 2 | Certain | Cost column width = `max(COST_WIDTH, longest fmtCost() over row costs and the grand total)`, computed before `barWidth`; `COST_WIDTH = 9` remains as the floor so small-cost output is byte-identical | Description specifies the formula; the pivot's `toolWidths` already establishes data-sizing; floor preserves Output Stability for today's common case | S:95 R:90 A:95 D:95 |
| 3 | Confident | `emitMarkdownTotalHistory` keeps the existing exact-zero omission (`nonzeroCostTools` stays); the negligible rule applies to the ANSI pivot only | Deferred — promptless dispatch. The description says emitters keep "existing behavior", though it misdescribes that behaviour (Markdown already omits exact-zero columns, only CSV keeps all). Default honours the stated intent (emitters unchanged); trivially reversible by pointing the emitter at `significantCostTools` | S:55 R:90 A:60 D:55 |
| 4 | Confident | Per-tool pivot columns are also data-sized: `max(name.length, MIN_TOOL_COL_WIDTH, longest cell incl. Total sum)` | Deferred — promptless dispatch. Not named by the user, but it is the same overflow (`Codex` column is 9 wide; a `$10,000.00` Codex month overflows and shifts the Cost column and bar of that row, reintroducing the reported symptom). Same helper, same test invariant; easily dropped at plan time | S:40 R:85 A:75 D:65 |
| 5 | Confident | `renderHistory` machine columns share one width (`max(MACHINE_COL_WIDTH, longest cell or total across all machines)`) rather than sizing each machine column independently | Letter-coded A/B/C columns read as a uniform block; a shared width is one variable and keeps the existing `mcols.length * (w + 3)` budget arithmetic; per-column sizing saves at most a char or two of bar | S:50 R:90 A:80 D:70 |
| 6 | Confident | `renderTotal` (snapshot) machine columns receive the same shared data-sized width and `$0.00` dimming; the snapshot's own Cost cell (12-wide numeric column, `fmtCostDelta`) is unchanged | Deferred — promptless dispatch. Not in the description, but `tu -u all m --by-machine` hits the identical `MACHINE_COL_WIDTH` overflow at `$10,000` per user; the description's "apply consistently to any `$0.00` data cell" covers the dimming half. Dropping it at plan time costs nothing | S:40 R:85 A:75 D:60 |
| 7 | Confident | Only an exact `cost === 0` cell dims; a sub-cent nonzero that rounds to `$0.00` is not dimmed | "Zeros are noise" targets absent data, not tiny spend; a `=== 0` test is unambiguous and needs no epsilon. Reversible one-liner | S:55 R:95 A:80 D:70 |
| 8 | Confident | Fallback chain when the significance filter empties the set: exact-nonzero tools, then the full registry list | Preserves the existing "fall back to all tools" guard the user asked to keep while avoiding resurrecting all-`$0.00` columns in a low-spend window (e.g. `$0.40` grand total) | S:60 R:90 A:85 D:75 |
| 9 | Certain | `rowCost`, `grandTotal`, `barTotal` and the footer sum over all registry tools; `cells`, `toolBars`, `toolSums`, legend and palette use the filtered set; `apportionSegments` absorbs the omitted share by normalising over the shares it receives | Description states omitted costs still count in Cost/Total/bars/footer; `apportionSegments` already normalises by `shareSum` (formatter.ts ~230–249), so no bar-code change is needed | S:90 R:85 A:90 D:90 |
| 10 | Certain | Zero cells are pre-padded to their column width before `dim()` so `row()`/`colorRow()` `padStart` is a no-op (they pad by raw string length, which would count ANSI bytes) | Existing `labelCell` relies on exactly this trick (`boldWhite(r.label.padEnd(D))`); an unpadded colored cell would visibly break alignment | S:85 R:90 A:95 D:95 |
| 11 | Certain | Total row is never dimmed (stays `boldWhite` for every cell); header and dividers unchanged | Explicit in the description | S:95 R:95 A:95 D:95 |
| 12 | Certain | The two 6-tool fixture tests (`formatter.test.ts` ~483, ~550) are updated to `≥ $1.00` per-tool costs that stay `≤ $9,999.99`, keeping the 96-char full-row assertion; the spec (data-sized, floored) is the truth the tests follow | Constitution § Test Integrity — tests conform to spec; fixtures `0.01–0.12` were incidental values, not a contract | S:85 R:90 A:95 D:90 |
| 13 | Certain | Layout change ships under a minor release (`just release minor`, "requires minor release" in the PR body); `package.json` is not edited in the feature diff | Constitution § Output Stability + memory DD "Output-Stability version bumps happen at release time" (3tah) | S:90 R:95 A:100 D:95 |
| 14 | Certain | Out of scope: Kimi swatch colour, footer avg computation, tokens display mode for table cells (follow-up change B) | Discussed — user explicitly rejected/deferred these | S:95 R:95 A:95 D:95 |
| 15 | Certain | Tests use the Node built-in runner (`npx tsx --test`) co-located in `src/node/tui/__tests__/`, added to `formatter.test.ts` or a `formatter-*.test.ts` sibling | Constitution § Test Runner / § Test Location; existing sibling files establish the naming | S:80 R:95 A:100 D:100 |

15 assumptions (9 certain, 6 confident, 0 tentative, 0 unresolved).
