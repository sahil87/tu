import type { UsageTotals, UsageEntry } from "../core/types.js";
import { currentLabel } from "../core/fetcher.js";
import { bold, dim, green, red, cyan, yellow, magenta, blue, boldWhite, boldCyan, colorDisabled, stripAnsi } from "./colors.js";

// The unit table cells, bars and footer stats render in. The snapshot table
// keeps its Cost column in dollars — only the delta indicator follows the
// metric (compact snapshot cells use the metric).
export type BarMetric = "cost" | "tokens";

export interface FormatOptions {
  prevCosts?: Map<string, number>;  // key: "{toolName}:{label}" or "{toolName}"
  compact?: boolean;
  maxRows?: number;  // truncate history to most recent N data rows (watch mode)
  machineCosts?: Map<string, Map<string, number>>;  // key: label/toolName → (machine → cost)
  capActive?: boolean;  // implicit 3-month history cap active → append "last 3 months" heading hint
  metric?: BarMetric;  // cell/bar/footer unit; absent ≡ "cost"
  machineLegend?: string;  // legend noun for the machineCosts columns; absent ≡ "Machines" ("Users" under -u all)
  historyTitle?: string;  // pivot title override (leaderboard history); absent ≡ the Combined {Cost,Token} History default
  columnOrder?: "registry" | "total-desc";  // pivot column order; absent ≡ "registry"
  highlightRowLeader?: boolean;  // pivot: boldWhite each row's max cell (color-only); absent ≡ false
  omitNegligibleColumns?: boolean;  // pivot negligible/zero column omission; absent ≡ true
}

// Build the parenthetical that follows a history title, e.g. "(daily)" or
// "(daily, last 3 months)" when the implicit 3-month cap is active. Shared by
// the ANSI renderers, the Markdown title helpers, and cli.ts's leaderboard
// history titles so the hint stays identical across output formats.
export function periodLabel(period: string, capActive?: boolean): string {
  return capActive ? `${period}, last 3 months` : period;
}

// Whether a daily ISO label (YYYY-MM-DD) falls on a Saturday or Sunday.
// ISO date-only strings parse as UTC midnight, so UTC accessors make the
// weekday a pure calendar fact, immune to the local timezone.
function isWeekendLabel(label: string): boolean {
  const day = new Date(label).getUTCDay();
  return day === 0 || day === 6;
}

export function fmtNum(n: number): string {
  return n.toLocaleString("en-US");
}

export function fmtCost(n: number): string {
  return `$${n.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
}

// Width of a right-aligned metric column sized to its data: floor COST_WIDTH
// (so small renders are byte-identical to a fixed 9-wide column), wide enough
// for the longest fmtMetric() among the values it will hold — including the
// Total-row value, which is usually the longest.
function metricColumnWidth(values: number[], metric: BarMetric): number {
  return Math.max(COST_WIDTH, ...values.map((v) => fmtMetric(v, metric).length));
}

// A metric data cell: pad first, then color — the row()/colorRow() builders pad
// by raw string length, which would count ANSI bytes, so a pre-padded cell is
// a no-op for padStart (the same trick labelCell relies on). Exact-zero cells
// dim in either unit — 0 tokens is as much "no data" as $0.00. Total-row cells
// never go through here — they stay boldWhite.
function metricCell(value: number, width: number, metric: BarMetric): string {
  const text = fmtMetric(value, metric).padStart(width);
  return value === 0 ? dim(text) : text;
}

// Format a cell/bar/footer value in the unit the table renders in (see BarMetric).
function fmtMetric(n: number, metric: BarMetric): string {
  return metric === "tokens" ? fmtNum(Math.round(n)) : fmtCost(n);
}

// The UsageTotals field a cell, bar or footer stat renders in the selected
// metric (UsageEntry is the subtype that also carries a label).
export function metricValue(e: UsageTotals, metric: BarMetric): number {
  return metric === "tokens" ? e.totalTokens : e.totalCost;
}

export function deltaIndicator(current: number, key: string, prevCosts?: Map<string, number>, noSpace = false): string {
  if (!prevCosts) return "";
  const prev = prevCosts.get(key);
  if (prev === undefined) return "";
  // The cross-tool pivot (renderTotalHistory) passes noSpace=true so the arrow
  // appends directly to the Cost cell ($128.13\u2191, 1 visible char) rather than
  // with a leading space ( \u2191, 2 chars): the full 6-tool row is 96 chars, so the
  // spaced form would render 98 and wrap on a 96–97-col terminal, corrupting
  // the watch-mode compositor's line-counting. (Zero-cost tool columns are
  // omitted, so typical rows are far narrower — but every active tool counts
  // toward the full row.) Other renderers have width headroom
  // and keep the spaced form.
  const sp = noSpace ? "" : " ";
  if (current > prev) return sp + green("\u2191");
  if (current < prev) return sp + red("\u2193");
  return "";
}

export function fmtMetricDelta(current: number, key: string, metric: BarMetric, prevCosts?: Map<string, number>): string {
  return fmtMetric(current, metric) + deltaIndicator(current, key, prevCosts);
}

export function fmtCostDelta(current: number, key: string, prevCosts?: Map<string, number>): string {
  return fmtMetricDelta(current, key, "cost", prevCosts);
}

// --- Inline bar rendering (fractional Unicode blocks at eighths precision) ---

const FULL_BLOCK = "\u2588"; // █
const MIN_BAR = "\u258F"; // ▏ (1/8)

const BLOCK_EIGHTHS = [
  "",         // 0/8 — no fractional part
  "\u258F",   // 1/8 ▏
  "\u258E",   // 2/8 ▎
  "\u258D",   // 3/8 ▍
  "\u258C",   // 4/8 ▌
  "\u258B",   // 5/8 ▋
  "\u258A",   // 6/8 ▊
  "\u2589",   // 7/8 ▉
];

const MIN_BAR_AREA = 10;
const MAX_BAR_WIDTH = 30;
const GUTTER = 3; // " | " separator between main table and cost area
// Floor for every right-aligned metric column — the rendered width is data-sized
// via metricColumnWidth() (max of this floor and the longest fmtMetric() the
// column will hold, including its Total-row value). 9 chars fits $9,999.99 with
// thousands separators; five-figure monthly cells ($15,429.88) and eight-figure
// token cells (9,999,999) are routine under `-u all`, so columns grow with
// their data instead of overflowing.
const COST_WIDTH = 9;
// Floor for a variable-width cross-tool pivot column (renderTotalHistory): each
// column is sized to max(tool name, this floor, the longest cost cell it holds
// including its Total-row sum). With every cell ≤ $9,999.99 the FULL 6-tool
// data row (Date + tool columns + gutter + Cost) is 10 + (11+9+9+9+9+9) + 6×3
// + 3 + 9 = 96 (97 in watch mode with the space-less delta indicator) — the
// minimum full-row width; larger cells widen their column (and the Cost column
// via costWidth) from there. Negligible-cost columns are omitted, so the
// typically rendered width is far below 80.
const MIN_TOOL_COL_WIDTH = 9;
// Date column width in the cross-tool pivot: ISO daily labels are 10 chars
// ("2026-07-01"), monthly 7 ("2026-07"), the "Date" header 4 — 10 fits all.
const PIVOT_DATE_WIDTH = 10;

// Machine column rendering helpers
export const MACHINE_COL_WIDTH = COST_WIDTH;

export interface MachineColumn {
  letter: string;
  name: string;
}

export function buildMachineColumns(machineNames: string[]): MachineColumn[] {
  const sorted = [...machineNames].sort();
  return sorted.map((name, i) => ({ letter: String.fromCharCode(65 + i), name }));
}

export function renderMachineLegend(columns: MachineColumn[], label = "Machines"): string {
  return `${label}: ` + columns.map((c) => `${c.letter} = ${c.name}`).join(", ");
}

export function renderBar(value: number, maxValue: number, barWidth: number): string {
  if (value === 0 || maxValue === 0) return "";
  const scaled = (value / maxValue) * barWidth;
  const fullBlocks = Math.floor(scaled);
  const eighths = Math.round((scaled - fullBlocks) * 8);

  if (eighths === 8) return FULL_BLOCK.repeat(fullBlocks + 1);

  const bar = FULL_BLOCK.repeat(fullBlocks) + BLOCK_EIGHTHS[eighths];
  return bar.length > 0 ? bar : MIN_BAR;
}

// --- Two-zone bar scaling (p95 cap with a scale-break rule) ---
//
// Outlier days crush a linear max-scaled bar to slivers. When the max row cost
// exceeds P95_TRIGGER_FACTOR × p95 of the nonzero row costs, the bar area splits
// into a main zone (linear 0→p95, green), a dim scale-break rule (┊) drawn in
// every row, and an overflow zone (linear p95→max, yellow). Below the trigger
// the single-zone path delegates to renderBar unchanged.

const P95_PERCENTILE = 95;
const P95_TRIGGER_FACTOR = 1.5;
const OVERFLOW_ZONE_MIN = 4;
const OVERFLOW_ZONE_DIVISOR = 4; // overflow zone ≈ 1/4 of the bar area
const SCALE_BREAK_RULE = "┊"; // ┊

export interface SingleZoneScale {
  mode: "single";
  max: number;
}

export interface TwoZoneScale {
  mode: "two-zone";
  p95: number;
  max: number;
  mainZone: number;
  overflowZone: number;
}

export type BarScale = SingleZoneScale | TwoZoneScale;

// p-th percentile (0–100) of a sorted-ascending sample, linear interpolation.
export function percentile(sortedAscending: number[], p: number): number {
  if (sortedAscending.length === 0) return 0;
  const idx = (p / 100) * (sortedAscending.length - 1);
  const lo = Math.floor(idx);
  const hi = Math.ceil(idx);
  return sortedAscending[lo] + (sortedAscending[hi] - sortedAscending[lo]) * (idx - lo);
}

// Pick the bar scale for a visible window of row costs. Two-zone mode engages
// only when an outlier dominates (max > 1.5 × p95 over nonzero costs).
export function computeBarScale(costs: number[], barWidth: number): BarScale {
  const max = Math.max(...costs);
  const nonzero = costs.filter((c) => c > 0).sort((a, b) => a - b);
  if (nonzero.length === 0) return { mode: "single", max };
  const p95 = percentile(nonzero, P95_PERCENTILE);
  if (!(max > P95_TRIGGER_FACTOR * p95)) return { mode: "single", max };
  const overflowZone = Math.max(OVERFLOW_ZONE_MIN, Math.round(barWidth / OVERFLOW_ZONE_DIVISOR));
  return { mode: "two-zone", p95, max, mainZone: barWidth - overflowZone - 1, overflowZone };
}

// Render one row's bar (leading space included) under the given scale. In
// two-zone mode the result is exactly barWidth visible chars: the main zone is
// space-padded up to the rule so the ┊ column aligns across rows, and the
// overflow zone is space-padded to keep total row width unchanged. A row at
// exactly p95 ends at the rule with no overflow segment.
export function renderScaledBar(value: number, scale: BarScale, barWidth: number): string {
  if (scale.mode === "single") {
    const raw = renderBar(value, scale.max, barWidth);
    return raw ? " " + green(raw) : "";
  }
  const mainRaw = renderBar(Math.min(value, scale.p95), scale.p95, scale.mainZone);
  const main = green(mainRaw) + " ".repeat(scale.mainZone - mainRaw.length);
  const overflowRaw = value > scale.p95 ? renderBar(value - scale.p95, scale.max - scale.p95, scale.overflowZone) : "";
  const overflow = yellow(overflowRaw) + " ".repeat(scale.overflowZone - overflowRaw.length);
  return " " + main + dim(SCALE_BREAK_RULE) + overflow;
}

// --- Stacked per-tool bar fill (cross-tool pivot) ---
//
// The pivot's row bar keeps today's exact length and character sequence; only
// the fill gains meaning. The main zone (or the whole bar in single-zone mode)
// is split into contiguous per-tool segments, left to right in pivot column
// order, apportioned by largest remainder so segments sum exactly to the
// unstacked bar length. The overflow zone stays solid yellow, unsegmented.

// Tool segment palette, assigned in pivot column order. A 5th+ visible tool
// falls back to uncolored segments (the palette is deliberately capped at 4).
// Green leads so the first (dominant) column matches the single-tool history
// bar; yellow is excluded — it is reserved for the overflow zone.
const STACK_PALETTE: Array<(s: string) => string> = [green, magenta, blue, cyan];

// Identity fallback for palette overflow (5th+ visible tool).
const noSegmentColor = (s: string): string => s;

export function stackedBarPalette(toolCount: number): Array<(s: string) => string> {
  return Array.from({ length: toolCount }, (_, i) => STACK_PALETTE[i] ?? noSegmentColor);
}

// Largest-remainder apportionment: distribute `total` characters among the
// shares proportionally — floor each quota, then hand the remaining characters
// to the largest fractional remainders (ties break to the earlier column for
// determinism). The result always sums exactly to `total`; a zero share gets
// zero characters.
export function apportionSegments(shares: number[], total: number): number[] {
  const counts = shares.map(() => 0);
  if (total <= 0) return counts;
  const shareSum = shares.reduce((sum, s) => sum + s, 0);
  if (shareSum <= 0) return counts;
  const quotas = shares.map((s) => (s / shareSum) * total);
  quotas.forEach((q, i) => {
    counts[i] = Math.floor(q);
  });
  let remainder = total - counts.reduce((sum, c) => sum + c, 0);
  const byRemainder = quotas
    .map((q, i) => ({ i, frac: q - Math.floor(q) }))
    .sort((a, b) => b.frac - a.frac || a.i - b.i);
  for (const { i } of byRemainder) {
    if (remainder <= 0) break;
    counts[i]++;
    remainder--;
  }
  return counts;
}

// Re-color an already-rendered raw bar string into contiguous per-tool runs.
// `counts` must sum to raw.length (apportionSegments guarantees this), so the
// runs are exact string slices — stripping ANSI from the result yields `raw`
// unchanged, and a trailing fractional-eighths character rides the last
// (rightmost) nonzero segment.
function colorBarRuns(raw: string, counts: number[], colorFns: Array<(s: string) => string>): string {
  let out = "";
  let pos = 0;
  for (let i = 0; i < counts.length; i++) {
    if (counts[i] === 0) continue;
    out += colorFns[i](raw.slice(pos, pos + counts[i]));
    pos += counts[i];
  }
  return out;
}

// Stacked variant of renderScaledBar for the cross-tool pivot: same geometry,
// same raw characters, but the main zone (or the whole single-zone bar) is
// colored in per-tool segments by cost share instead of solid green. The
// overflow zone keeps its solid yellow rendering — proportional segments
// would be misleading on the compressed overflow scale.
export function renderStackedScaledBar(
  value: number,
  toolCosts: number[],
  colorFns: Array<(s: string) => string>,
  scale: BarScale,
  barWidth: number,
): string {
  if (scale.mode === "single") {
    const raw = renderBar(value, scale.max, barWidth);
    return raw ? " " + colorBarRuns(raw, apportionSegments(toolCosts, raw.length), colorFns) : "";
  }
  const mainRaw = renderBar(Math.min(value, scale.p95), scale.p95, scale.mainZone);
  const main = colorBarRuns(mainRaw, apportionSegments(toolCosts, mainRaw.length), colorFns)
    + " ".repeat(scale.mainZone - mainRaw.length);
  const overflowRaw = value > scale.p95 ? renderBar(value - scale.p95, scale.max - scale.p95, scale.overflowZone) : "";
  const overflow = yellow(overflowRaw) + " ".repeat(scale.overflowZone - overflowRaw.length);
  return " " + main + dim(SCALE_BREAK_RULE) + overflow;
}

// --- History summary footer (avg / this month / peak [+ p95 legend] [+ tool legend]) ---

const PERIOD_UNIT_SUFFIX: Record<string, string> = { daily: "/day", weekly: "/week", monthly: "/month" };

// One legend swatch: the tool's segment color + name (pivot stacked bars only).
interface FooterLegendEntry {
  name: string;
  colorFn: (s: string) => string;
}

// Dim footer appended after the Total row in history views. `this month` is
// daily-only and omitted when the window has no current-month rows; the ┊
// legend appears only when the two-zone scale is active. The tool legend
// (stacked-bar pivot only) is built outside the dim wrapper: each swatch's
// color reset (\x1b[0m) would otherwise strip dim from the rest of the line,
// so the separator and each tool name are dim-wrapped individually.
function renderHistoryFooter(labels: string[], costs: number[], period: string, scale: BarScale, legend?: FooterLegendEntry[], metric: BarMetric = "cost"): string {
  const total = costs.reduce((sum, c) => sum + c, 0);
  const parts: string[] = [`avg ${fmtMetric(total / costs.length, metric)}${PERIOD_UNIT_SUFFIX[period] ?? "/row"}`];
  if (period === "daily") {
    const monthPrefix = currentLabel("monthly");
    let monthSum = 0;
    let hasMonthRows = false;
    for (let i = 0; i < labels.length; i++) {
      if (labels[i].startsWith(monthPrefix)) {
        monthSum += costs[i];
        hasMonthRows = true;
      }
    }
    if (hasMonthRows) parts.push(`this month ${fmtMetric(monthSum, metric)}`);
  }
  let peak = 0;
  let peakLabel = "";
  for (let i = 0; i < labels.length; i++) {
    if (costs[i] > peak) {
      peak = costs[i];
      peakLabel = labels[i];
    }
  }
  parts.push(peakLabel === "" ? `peak ${fmtMetric(peak, metric)}` : `peak ${fmtMetric(peak, metric)} (${peakLabel})`);
  if (scale.mode === "two-zone") parts.push(`${SCALE_BREAK_RULE} = ${fmtMetric(scale.p95, metric)} (p95)`);
  const footer = dim(parts.join(" · "));
  if (!legend || legend.length === 0) return footer;
  const swatches = legend.map((l) => `${l.colorFn(FULL_BLOCK)} ${dim(l.name)}`).join(" ");
  return footer + dim(" · ") + swatches;
}

// --- Single-tool history table (tu cc daily, tu codex monthly, etc.) ---

export function renderHistory(toolName: string, period: string, entries: UsageEntry[], termWidth?: number, opts?: FormatOptions): string[] {
  const lines: string[] = [];
  lines.push("");
  lines.push(boldWhite(`\u{1F4CA} ${toolName} (${periodLabel(period, opts?.capActive)})`));
  lines.push("");

  if (entries.length === 0) {
    lines.push("  No data");
    lines.push("");
    return lines;
  }

  // Truncate to most recent maxRows entries (watch mode)
  if (opts?.maxRows && entries.length > opts.maxRows) {
    entries = entries.slice(-opts.maxRows);
  }

  // Compact mode: date + value only (in the displayed metric)
  const metric = opts?.metric ?? "cost";
  if (opts?.compact) {
    lines.push(...renderCompactHistory(entries, opts.prevCosts, toolName, metric));
    return lines;
  }

  const D = 12;
  const N = 14;
  const numCols = 6;
  const tableWidth = D + (numCols - 1) * N + (numCols - 1) * 3;
  const width = termWidth ?? process.stdout.columns ?? 80;

  // Machine columns setup (computed before bar width so we can subtract their width).
  // machineCosts values are in the displayed metric (the cli.ts builders take it).
  const machineCosts = opts?.machineCosts;
  const machineNames = machineCosts ? [...new Set([...machineCosts.values()].flatMap((m) => [...m.keys()]))] : [];
  const mcols = buildMachineColumns(machineNames);
  const hasMachines = mcols.length > 0;

  // Pre-pass: the last (metric) column and the machine columns are sized from
  // the data before the bar budget is derived. sumValue and the per-machine
  // sums also feed the Total row below.
  let sumValue = 0;
  const machineSums = new Map<string, number>();
  const machineCellValues: number[] = [];
  for (const e of entries) {
    sumValue += metricValue(e, metric);
    const rowMachines = machineCosts?.get(e.label);
    for (const mc of mcols) {
      const value = rowMachines?.get(mc.name) ?? 0;
      machineCellValues.push(value);
      machineSums.set(mc.name, (machineSums.get(mc.name) ?? 0) + value);
    }
  }

  // Last column sized to its data (floor COST_WIDTH), including the Total row.
  const costWidth = metricColumnWidth([...entries.map((e) => metricValue(e, metric)), sumValue], metric);
  // All machine columns share one data-sized width (floored at
  // MACHINE_COL_WIDTH) so the letter-coded columns read as a uniform block.
  const machineColWidth = hasMachines
    ? metricColumnWidth([...machineCellValues, ...mcols.map((mc) => machineSums.get(mc.name) ?? 0)], metric)
    : MACHINE_COL_WIDTH;
  const machineColsWidth = hasMachines ? mcols.length * (machineColWidth + 3) : 0; // " | " + padded cost

  const barWidth = Math.min(width - tableWidth - GUTTER - costWidth - machineColsWidth - 1, MAX_BAR_WIDTH);
  const showBars = barWidth >= MIN_BAR_AREA;

  const row = (...cols: string[]) => cols.map((c, i) => (i === 0 ? c.padEnd(D) : c.padStart(N))).join(" | ");
  const colorRow = (cols: string[], colorFn: (s: string) => string) =>
    cols.map((c, i) => colorFn(i === 0 ? c.padEnd(D) : c.padStart(N))).join(" | ");
  const divStr = [D, N, N, N, N, N].map(w => "─".repeat(w)).join("─|─");
  const costDiv = "─|─" + "─".repeat(costWidth);
  const barDiv = showBars ? "─" + "─".repeat(barWidth) : "";
  const machineDiv = hasMachines ? mcols.map(() => "─|─" + "─".repeat(machineColWidth)).join("") : "";
  const machineHeader = hasMachines ? mcols.map((c) => " | " + boldCyan(c.letter.padStart(machineColWidth))).join("") : "";

  const costHeader = " | " + boldCyan((metric === "tokens" ? "Tokens" : "Cost").padStart(costWidth));
  lines.push(colorRow(["Date", "Input", "Output", "Cache Write", "Cache Read", "Total"], boldCyan) + costHeader + machineHeader);
  lines.push(dim(divStr + costDiv + machineDiv + barDiv));

  const barValues = entries.map((e) => metricValue(e, metric));
  const scale = showBars
    ? computeBarScale(barValues, barWidth)
    : { mode: "single" as const, max: Math.max(...barValues) };
  const current = currentLabel(period);
  const prevCosts = opts?.prevCosts;

  let sumInput = 0;
  let sumOutput = 0;
  let sumCacheW = 0;
  let sumCacheR = 0;
  let sumTotal = 0;
  let prevMonthPrefix = "";

  for (const e of entries) {
    // Month-boundary separator (daily views only) — same construction as the header divider
    const monthPrefix = e.label.slice(0, 7);
    if (period === "daily" && prevMonthPrefix && monthPrefix !== prevMonthPrefix) {
      lines.push(dim(divStr + costDiv + machineDiv + barDiv));
    }
    prevMonthPrefix = monthPrefix;

    // The current period's row renders its date cell in boldWhite (Total-row
    // emphasis); otherwise weekend dates dim (daily only) to expose the weekly
    // rhythm. One cell, one style — the today marker wins on a weekend today.
    const labelCell = e.label === current
      ? boldWhite(e.label.padEnd(D))
      : period === "daily" && isWeekendLabel(e.label)
        ? dim(e.label.padEnd(D))
        : e.label;
    const rowStr = row(labelCell, fmtNum(e.inputTokens), fmtNum(e.outputTokens), fmtNum(e.cacheCreationTokens), fmtNum(e.cacheReadTokens), fmtNum(e.totalTokens));
    const value = metricValue(e, metric);
    const costBase = " | " + metricCell(value, costWidth, metric);
    const indicator = deltaIndicator(value, `${toolName}:${e.label}`, prevCosts);

    let machineCells = "";
    if (hasMachines) {
      const rowMachines = machineCosts?.get(e.label);
      for (const mc of mcols) {
        const mcValue = rowMachines?.get(mc.name) ?? 0;
        machineCells += " | " + metricCell(mcValue, machineColWidth, metric);
      }
    }

    const bar = showBars ? renderScaledBar(metricValue(e, metric), scale, barWidth) : "";
    lines.push(rowStr + costBase + machineCells + indicator + bar);
    sumInput += e.inputTokens;
    sumOutput += e.outputTokens;
    sumCacheW += e.cacheCreationTokens;
    sumCacheR += e.cacheReadTokens;
    sumTotal += e.totalTokens;
  }

  if (entries.length > 1) {
    lines.push(dim(divStr + costDiv + machineDiv + barDiv));
    const totalRow = colorRow(["Total", fmtNum(sumInput), fmtNum(sumOutput), fmtNum(sumCacheW), fmtNum(sumCacheR), fmtNum(sumTotal)], boldWhite);
    const totalCost = " | " + boldWhite(fmtMetric(sumValue, metric).padStart(costWidth));
    let totalMachineCells = "";
    if (hasMachines) {
      for (const mc of mcols) {
        totalMachineCells += " | " + boldWhite(fmtMetric(machineSums.get(mc.name) ?? 0, metric).padStart(machineColWidth));
      }
    }
    lines.push(totalRow + totalCost + totalMachineCells);
    lines.push(renderHistoryFooter(entries.map((e) => e.label), barValues, period, scale, undefined, metric));
  }
  if (hasMachines) {
    lines.push("");
    lines.push(dim(renderMachineLegend(mcols, opts?.machineLegend)));
  }
  lines.push("");
  return lines;
}

export function printHistory(toolName: string, period: string, entries: UsageEntry[], termWidth?: number, opts?: FormatOptions): void {
  renderHistory(toolName, period, entries, termWidth, opts).forEach((l) => console.log(l));
}

// --- Cross-tool snapshot table (tu total daily, tu total monthly) ---

export function renderTotal(period: string, toolTotals: Map<string, UsageTotals>, opts?: FormatOptions): string[] {
  const lines: string[] = [];
  lines.push("");
  lines.push(boldWhite(`\u{1F4CA} Combined Usage (${period})`));
  lines.push("");

  const allZero = [...toolTotals.values()].every((t) => t.totalTokens === 0);
  if (allZero) {
    lines.push("  No usage");
    lines.push("");
    return lines;
  }

  // Compact mode: name + value only (in the displayed metric)
  const metric = opts?.metric ?? "cost";
  if (opts?.compact) {
    lines.push(...renderCompactSnapshot(toolTotals, opts.prevCosts, metric));
    return lines;
  }

  const W = 12;
  const N = 12;
  const prevCosts = opts?.prevCosts;
  const row = (...cols: string[]) =>
    cols.map((c, i) => (i === 0 ? c.padEnd(W) : c.padStart(N))).join(" | ");
  const colorRow = (cols: string[], colorFn: (s: string) => string) =>
    cols.map((c, i) => colorFn(i === 0 ? c.padEnd(W) : c.padStart(N))).join(" | ");
  const divider = [W, N, N, N, N, N].map(w => "─".repeat(w)).join("─|─");

  // Machine columns setup. machineCosts values are in the displayed metric
  // (the cli.ts builders take it); under cost they are dollars, byte-identical.
  const machineCosts = opts?.machineCosts;
  const machineNames = machineCosts ? [...new Set([...machineCosts.values()].flatMap((m) => [...m.keys()]))] : [];
  const mcols = buildMachineColumns(machineNames);
  const hasMachines = mcols.length > 0;

  // Pre-pass over the rendered rows (tools with totalTokens > 0): all machine
  // columns share one data-sized width (floored at MACHINE_COL_WIDTH), sized
  // over the row cells plus the Total-row sums, which the pre-pass also feeds.
  const machineSums = new Map<string, number>();
  const machineCellValues: number[] = [];
  for (const [name, t] of toolTotals) {
    if (t.totalTokens === 0) continue;
    const toolMachines = machineCosts?.get(name);
    for (const mc of mcols) {
      const value = toolMachines?.get(mc.name) ?? 0;
      machineCellValues.push(value);
      machineSums.set(mc.name, (machineSums.get(mc.name) ?? 0) + value);
    }
  }
  const machineColWidth = hasMachines
    ? metricColumnWidth([...machineCellValues, ...mcols.map((mc) => machineSums.get(mc.name) ?? 0)], metric)
    : MACHINE_COL_WIDTH;
  const machineDiv = hasMachines ? mcols.map(() => "─|─" + "─".repeat(machineColWidth)).join("") : "";
  const machineHeader = hasMachines ? mcols.map((c) => " | " + boldCyan(c.letter.padStart(machineColWidth))).join("") : "";

  lines.push(colorRow(["Tool", "Tokens", "Input", "Output", "Cache", "Cost"], boldCyan) + machineHeader);
  lines.push(dim(divider + machineDiv));

  let grandCost = 0;
  let grandInput = 0;
  let grandOutput = 0;
  let grandCache = 0;
  let grandTotal = 0;

  for (const [name, t] of toolTotals) {
    if (t.totalTokens > 0) {
      // The table is metric-neutral — its columns are already token-denominated
      // and Cost is kept as context in token mode. Only the watch delta
      // indicator follows the metric: it compares the value in the prev map
      // (built in the displayed metric), so under tokens it rides the Tokens
      // cell and the Cost cell stays plain; under cost it rides Cost.
      const tokensStr = metric === "tokens"
        ? fmtNum(t.totalTokens) + deltaIndicator(t.totalTokens, name, prevCosts)
        : fmtNum(t.totalTokens);
      const costStr = metric === "tokens" ? fmtCost(t.totalCost) : fmtCostDelta(t.totalCost, name, prevCosts);
      let machineCells = "";
      if (hasMachines) {
        const toolMachines = machineCosts?.get(name);
        for (const mc of mcols) {
          const value = toolMachines?.get(mc.name) ?? 0;
          machineCells += " | " + metricCell(value, machineColWidth, metric);
        }
      }
      lines.push(row(name, tokensStr, fmtNum(t.inputTokens), fmtNum(t.outputTokens), fmtNum(t.cacheCreationTokens + t.cacheReadTokens), costStr) + machineCells);
    }
    grandCost += t.totalCost;
    grandInput += t.inputTokens;
    grandOutput += t.outputTokens;
    grandCache += t.cacheCreationTokens + t.cacheReadTokens;
    grandTotal += t.totalTokens;
  }

  const visibleCount = [...toolTotals.values()].filter(t => t.totalTokens > 0).length;
  if (visibleCount > 1) {
    lines.push(dim(divider + machineDiv));
    let totalMachineCells = "";
    if (hasMachines) {
      for (const mc of mcols) {
        totalMachineCells += " | " + boldWhite(fmtMetric(machineSums.get(mc.name) ?? 0, metric).padStart(machineColWidth));
      }
    }
    lines.push(colorRow(["Total", fmtNum(grandTotal), fmtNum(grandInput), fmtNum(grandOutput), fmtNum(grandCache), fmtCost(grandCost)], boldWhite) + totalMachineCells);
  }
  if (hasMachines) {
    lines.push("");
    lines.push(dim(renderMachineLegend(mcols, opts?.machineLegend)));
  }
  lines.push("");
  return lines;
}

export function printTotal(period: string, toolTotals: Map<string, UsageTotals>, opts?: FormatOptions): void {
  renderTotal(period, toolTotals, opts).forEach((l) => console.log(l));
}

// --- Cross-tool history pivot (tu total-history daily, tu total-history monthly) ---
// Y-axis = dates, X-axis = tool names, cell = the displayed metric (cost or tokens)

// Filter pivot tool columns to those with a nonzero total across the given
// labels (the visible window for the ANSI pivot, all labels for Markdown).
// Falls back to the unfiltered list when the filter would empty it. Shared by
// renderTotalHistory and emitMarkdownTotalHistory (over its cost map — emitters
// ignore the metric); CSV keeps all columns.
function nonzeroTools(toolNames: string[], valueMap: Map<string, Map<string, number>>, labels: string[]): string[] {
  const active = toolNames.filter((tool) =>
    labels.some((label) => (valueMap.get(tool)?.get(label) ?? 0) !== 0));
  return active.length > 0 ? active : toolNames;
}

// Human-facing ANSI pivot: a tool column is worth its ~12 chars of bar area
// only when its visible-window total is significant in the displayed unit.
const NEGLIGIBLE_COST_ABS = 1.0;     // dollars — omit below $1.00 …
const NEGLIGIBLE_TOKENS_ABS = 1_000; // tokens  — omit below 1,000 tokens …
const NEGLIGIBLE_SHARE = 0.001;      // … or below 0.1% of the window grand total (either unit)

function negligibleAbs(metric: BarMetric): number {
  return metric === "tokens" ? NEGLIGIBLE_TOKENS_ABS : NEGLIGIBLE_COST_ABS;
}

// Keep a pivot column iff its visible-window total is ≥ negligibleAbs(metric)
// AND ≥ NEGLIGIBLE_SHARE × the window grand total, both in the displayed unit
// (boundary values are kept) — a zero-priced tool with real tokens is a real
// column in token mode and noise in cost mode. When nothing survives (e.g. a
// $0.40 window), fall back to the exact-zero filter, which itself falls back to
// the full registry list. Markdown keeps the exact-zero rule (nonzeroTools over
// its cost map); CSV is unfiltered.
function significantTools(toolNames: string[], valueMap: Map<string, Map<string, number>>, labels: string[], metric: BarMetric): string[] {
  const totals = new Map<string, number>();
  for (const tool of toolNames) {
    let total = 0;
    for (const label of labels) total += valueMap.get(tool)?.get(label) ?? 0;
    totals.set(tool, total);
  }
  const grand = [...totals.values()].reduce((sum, t) => sum + t, 0);
  const abs = negligibleAbs(metric);
  const kept = toolNames.filter((tool) => {
    const total = totals.get(tool)!;
    return total >= abs && total >= NEGLIGIBLE_SHARE * grand;
  });
  return kept.length > 0 ? kept : nonzeroTools(toolNames, valueMap, labels);
}

export function renderTotalHistory(period: string, allToolEntries: Map<string, UsageEntry[]>, termWidth?: number, opts?: FormatOptions): string[] {
  const lines: string[] = [];
  const metric = opts?.metric ?? "cost";
  lines.push("");
  lines.push(boldWhite(opts?.historyTitle ?? `\u{1F4CA} Combined ${metric === "tokens" ? "Token" : "Cost"} History (${periodLabel(period, opts?.capActive)})`));
  lines.push("");

  const allToolNames = [...allToolEntries.keys()];

  // Collect labels early for compact check
  const labelSet = new Set<string>();
  for (const entries of allToolEntries.values()) {
    for (const e of entries) labelSet.add(e.label);
  }
  let labels = [...labelSet].sort();

  // Truncate to most recent maxRows labels (watch mode)
  if (opts?.maxRows && labels.length > opts.maxRows) {
    labels = labels.slice(-opts.maxRows);
  }

  if (labels.length === 0) {
    lines.push("  No data");
    lines.push("");
    return lines;
  }

  // Compact mode: date + total value only (in the displayed metric)
  if (opts?.compact) {
    const valueMap = new Map<string, number>();
    for (const entries of allToolEntries.values()) {
      for (const e of entries) {
        valueMap.set(e.label, (valueMap.get(e.label) || 0) + metricValue(e, metric));
      }
    }
    lines.push(...renderCompactTotalHistory(labels, valueMap, opts.prevCosts, metric));
    return lines;
  }

  // One lookup: tool -> label -> the displayed value. Cells, the row column,
  // the Total row, bars, stacked segments and the footer all render in the
  // selected metric, so the two maps coincide under both units.
  const valueMap = new Map<string, Map<string, number>>();
  for (const [tool, entries] of allToolEntries) {
    const values = new Map<string, number>();
    for (const e of entries) {
      values.set(e.label, metricValue(e, metric));
    }
    valueMap.set(tool, values);
  }

  // Omit tool columns whose visible-window (post-maxRows) total is negligible
  // in the displayed unit (< $1.00 / < 1,000 tokens, or < 0.1% of the window
  // grand total) — noise columns that widen the row and crowd out the bar
  // chart. Falls back to the exact-zero filter, then to the full registry list
  // (defensive; cannot normally occur with nonempty labels). The leaderboard
  // history passes omitNegligibleColumns: false — silently hiding a low-spend
  // user from a ranking is wrong; --top gives explicit control instead.
  const toolNames = opts?.omitNegligibleColumns === false
    ? allToolNames
    : significantTools(allToolNames, valueMap, labels, metric);
  // Segment colors for the stacked bar, assigned in visible column order
  // (5th+ tool falls back to uncolored segments).
  const barPalette = stackedBarPalette(toolNames.length);

  // Pre-compute per-row value data BEFORE the width budget: the value column
  // and the per-tool columns are sized from this data, and barWidth subtracts
  // the computed costWidth. rowValue/toolSums/grandTotal sum over ALL registry
  // tools — an omitted negligible column still counts in the row value, the
  // Total, the bars and the footer — while values cover only the visible
  // (filtered) columns.
  const rowData: { label: string; values: number[]; rowValue: number }[] = [];
  const toolSums = new Map<string, number>(toolNames.map((t) => [t, 0]));
  let grandTotal = 0;

  for (const label of labels) {
    let rowValue = 0;
    for (const tool of allToolNames) {
      rowValue += valueMap.get(tool)?.get(label) || 0;
    }
    const values: number[] = [];
    for (const tool of toolNames) {
      const value = valueMap.get(tool)?.get(label) || 0;
      values.push(value);
      toolSums.set(tool, (toolSums.get(tool) || 0) + value);
    }
    grandTotal += rowValue;
    rowData.push({ label, values, rowValue });
  }

  // Leaderboard history ranks its columns: descending by window total in the
  // display metric (ties keep first-seen order). The tool pivot keeps registry
  // order so a tool's color/position never shifts across windows — user sets
  // are not a fixed registry, so that rationale does not transfer.
  if (opts?.columnOrder === "total-desc") {
    const order = toolNames
      .map((name, i) => ({ name, i }))
      .sort((a, b) => (toolSums.get(b.name) ?? 0) - (toolSums.get(a.name) ?? 0) || a.i - b.i);
    toolNames.length = 0;
    for (const o of order) toolNames.push(o.name);
    for (const r of rowData) {
      r.values = order.map((o) => r.values[o.i]);
    }
  }

  const D = PIVOT_DATE_WIDTH;
  // Variable per-tool column width: max(tool name, the MIN_TOOL_COL_WIDTH
  // floor, the longest cell in the column including its Total-row sum) —
  // a five-figure cell widens its own column instead of overflowing it and
  // shifting every column to its right (see the constant for the full-row
  // math). Fixed-width columns overflowed 80-col terminals once the pivot grew
  // to 5 tools; sizing per column keeps the full data row as narrow as the
  // cells allow (96 cols at 6 tools, before negligible-column omission).
  const toolWidths = toolNames.map((name, i) =>
    Math.max(
      name.length,
      MIN_TOOL_COL_WIDTH,
      ...rowData.map((r) => fmtMetric(r.values[i], metric).length),
      fmtMetric(toolSums.get(name) || 0, metric).length,
    ));
  // The row value column sized to its data (floor COST_WIDTH) — includes the
  // Total-row grand total, usually the longest value the column holds.
  const costWidth = metricColumnWidth([...rowData.map((r) => r.rowValue), grandTotal], metric);
  // Date + each tool column + the " | " (3-char) separator before each column.
  const tableWidth = D + toolWidths.reduce((sum, w) => sum + w + 3, 0);
  const width = termWidth ?? process.stdout.columns ?? 80;
  // The bar renders with a leading space (the trailing -1). In watch mode a
  // 1-char delta indicator (deltaIndicator noSpace=true) is appended to the Cost
  // cell before the bar, so reserve one more char when prevCosts is set —
  // otherwise the max-cost row measures width+1 and wraps across the bars band.
  const indicatorReserve = opts?.prevCosts ? 1 : 0;
  const barWidth = Math.min(width - tableWidth - GUTTER - costWidth - 1 - indicatorReserve, MAX_BAR_WIDTH);
  const showBars = barWidth >= MIN_BAR_AREA;

  const row = (...cols: string[]) => cols.map((c, i) => (i === 0 ? c.padEnd(D) : c.padStart(toolWidths[i - 1]))).join(" | ");
  const colorRow = (cols: string[], colorFn: (s: string) => string) =>
    cols.map((c, i) => colorFn(i === 0 ? c.padEnd(D) : c.padStart(toolWidths[i - 1]))).join(" | ");
  const divStr = [D, ...toolWidths].map((w) => "─".repeat(w)).join("─|─");
  const costDiv = "─|─" + "─".repeat(costWidth);
  const barDiv = showBars ? "─" + "─".repeat(barWidth) : "";

  const costHeader = " | " + boldCyan((metric === "tokens" ? "Tokens" : "Cost").padStart(costWidth));
  lines.push(colorRow(["Date", ...toolNames], boldCyan) + costHeader);
  lines.push(dim(divStr + costDiv + barDiv));

  const maxBar = Math.max(...rowData.map((r) => r.rowValue));
  const scale = showBars ? computeBarScale(rowData.map((r) => r.rowValue), barWidth) : { mode: "single" as const, max: maxBar };
  const current = currentLabel(period);

  const prevCosts = opts?.prevCosts;
  let prevMonthPrefix = "";

  for (const r of rowData) {
    // Month-boundary separator (daily views only) — same construction as the header divider
    const monthPrefix = r.label.slice(0, 7);
    if (period === "daily" && prevMonthPrefix && monthPrefix !== prevMonthPrefix) {
      lines.push(dim(divStr + costDiv + barDiv));
    }
    prevMonthPrefix = monthPrefix;

    // The current period's row renders its date cell in boldWhite (Total-row
    // emphasis); otherwise weekend dates dim (daily only) to expose the weekly
    // rhythm. One cell, one style — the today marker wins on a weekend today.
    const labelCell = r.label === current
      ? boldWhite(r.label.padEnd(D))
      : period === "daily" && isWeekendLabel(r.label)
        ? dim(r.label.padEnd(D))
        : r.label;
    // Leaderboard history highlights each row's winning cell (boldWhite,
    // color-only — padded width unchanged, stripped by --no-color). The
    // already-padded dim/plain cell is re-wrapped, matching the metricCell
    // pad-first-then-color contract.
    // Loop-based max index: spreading a large values array into Math.max can
    // hit the argument-count limit (RangeError). Strict `>` keeps the first
    // max, matching indexOf(Math.max(...)) semantics.
    let leaderIdx = -1;
    if (opts?.highlightRowLeader && r.values.length > 0) {
      leaderIdx = 0;
      for (let i = 1; i < r.values.length; i++) {
        if (r.values[i] > r.values[leaderIdx]) leaderIdx = i;
      }
    }
    const rowStr = row(labelCell, ...r.values.map((v, i) => {
      const cell = metricCell(v, toolWidths[i], metric);
      return i === leaderIdx ? boldWhite(cell) : cell;
    }));
    const costBase = " | " + metricCell(r.rowValue, costWidth, metric);
    const indicator = deltaIndicator(r.rowValue, `total:${r.label}`, prevCosts, true);
    const bar = showBars ? renderStackedScaledBar(r.rowValue, r.values, barPalette, scale, barWidth) : "";
    lines.push(rowStr + costBase + indicator + bar);
  }

  if (labels.length > 1) {
    lines.push(dim(divStr + costDiv + barDiv));
    const sumCells = toolNames.map((t) => fmtMetric(toolSums.get(t) || 0, metric));
    const totalRow = colorRow(["Total", ...sumCells], boldWhite);
    const totalCost = " | " + boldWhite(fmtMetric(grandTotal, metric).padStart(costWidth));
    lines.push(totalRow + totalCost);
    // Legend: one colored swatch per visible tool, only when stacked bars are
    // actually distinguishable (bars shown, ≥2 tools, color enabled).
    const legend = showBars && toolNames.length >= 2 && !colorDisabled()
      ? toolNames.map((name, i) => ({ name, colorFn: barPalette[i] }))
      : undefined;
    lines.push(renderHistoryFooter(rowData.map((r) => r.label), rowData.map((r) => r.rowValue), period, scale, legend, metric));
  }
  lines.push("");
  return lines;
}

export function printTotalHistory(period: string, allToolEntries: Map<string, UsageEntry[]>, termWidth?: number, opts?: FormatOptions): void {
  renderTotalHistory(period, allToolEntries, termWidth, opts).forEach((l) => console.log(l));
}

// --- Leaderboard snapshot (tu m lb) ---
//
// One row per user (or user/machine pair under --by-machine), ranked
// descending by the display metric. Columns: rank, user (pinned user marked
// with ◂), cost, bar (solid green, existing bar primitives/budget), tokens,
// share, and Δ vs the previous window. Both Cost and Tokens render in every
// metric mode — the metric selects only the sort key, bar scale, share
// denominator and the heading's "by …" suffix.

export interface LeaderboardRenderRow {
  rank: number;
  user: string;
  machine?: string;
  totals: UsageTotals;
  share: number;
  delta?: number;
}

export interface LeaderboardRenderOptions {
  period: string;
  windowLabel: string;   // current window label in the heading (period label, or "since → until")
  deltaLabel: string;    // previous-window label in the Δ header ("Jul", ISO date, week start, "prev")
  metric?: BarMetric;
  pinnedUser?: string;   // row whose user name carries the ◂ marker
  top?: number;          // keep the top N rows; the rest collapse into "… +k others"
  lastSync?: string;     // staleness footer text ("42m ago (…)"); "never"/absent ⇒ never synced
  prevCosts?: Map<string, number>;  // watch deltas, keyed by user (or user/machine), valued in the display metric
  termWidth?: number;
}

// Fractional delta → signed percent cell ("+12%", "-4%", "new").
function fmtDeltaCell(delta: number | undefined): string {
  if (delta === undefined) return "new";
  const pct = Math.round(delta * 100);
  return `${pct >= 0 ? "+" : ""}${pct}%`;
}

// The leaderboard row key: the user name, or "user/machine" under --by-machine.
function leaderboardKey(row: LeaderboardRenderRow): string {
  return row.machine !== undefined ? `${row.user}/${row.machine}` : row.user;
}

export function renderLeaderboard(rows: LeaderboardRenderRow[], opts: LeaderboardRenderOptions): string[] {
  const lines: string[] = [];
  const metric = opts.metric ?? "cost";
  lines.push("");
  lines.push(boldWhite(`Leaderboard (${opts.period}) · ${opts.windowLabel} · by ${metric}`));
  lines.push("");

  if (rows.length === 0) {
    lines.push("  No data");
    lines.push("");
    lines.push(dim(opts.lastSync !== undefined && opts.lastSync !== "never"
      ? `synced ${opts.lastSync} · tu sync to refresh`
      : "never synced · tu sync to refresh"));
    lines.push("");
    return lines;
  }

  // --top keeps the first N ranked rows; the rest collapse into one dim line.
  // Collapsed rows still count toward the Total row and every share
  // denominator (shares were computed over the full set by buildLeaderboard).
  const visible = opts.top !== undefined ? rows.slice(0, opts.top) : rows;
  const collapsed = rows.length - visible.length;
  const collapsedLabel = collapsed > 0 ? `… +${collapsed} others` : "";

  const nameCell = (row: LeaderboardRenderRow): string =>
    leaderboardKey(row) + (row.user === opts.pinnedUser ? " ◂" : "");
  const deltaHeader = `Δ vs ${opts.deltaLabel}`;
  const shareCell = (share: number): string => `${(share * 100).toFixed(1)}%`;

  let grandCost = 0;
  let grandTokens = 0;
  for (const row of rows) {
    grandCost += row.totals.totalCost;
    grandTokens += row.totals.totalTokens;
  }

  const rankWidth = Math.max(1, ...visible.map((r) => String(r.rank).length));
  const nameWidth = Math.max("User".length, collapsedLabel.length, ...visible.map((r) => nameCell(r).length));
  const costWidth = metricColumnWidth([...visible.map((r) => r.totals.totalCost), grandCost], "cost");
  const tokenWidth = metricColumnWidth([...visible.map((r) => r.totals.totalTokens), grandTokens], "tokens");
  const shareWidth = Math.max("Share".length, ...visible.map((r) => shareCell(r.share).length));
  const deltaWidth = Math.max(deltaHeader.length, ...visible.map((r) => fmtDeltaCell(r.delta).length));

  const width = opts.termWidth ?? process.stdout.columns ?? 80;
  const tableWidth = rankWidth + 3 + nameWidth + 3 + costWidth + 3 + tokenWidth + 3 + shareWidth + 3 + deltaWidth;
  // The watch delta indicator appends one visible char to the metric cell; the
  // bar budget reserves it (mirroring the pivot's indicatorReserve) so a watch
  // row never exceeds the terminal width.
  const indicatorReserve = opts.prevCosts ? 1 : 0;
  const barWidth = Math.min(width - tableWidth - 1 - indicatorReserve, MAX_BAR_WIDTH);
  const showBars = barWidth >= MIN_BAR_AREA;

  const barDiv = showBars ? "─" + "─".repeat(barWidth) : "";
  const divStr = [rankWidth, nameWidth, costWidth].map((w) => "─".repeat(w)).join("─|─")
    + barDiv
    + [tokenWidth, shareWidth, deltaWidth].map((w) => "─|─" + "─".repeat(w)).join("");

  const header = boldCyan("#".padStart(rankWidth))
    + " | " + boldCyan("User".padEnd(nameWidth))
    + " | " + boldCyan("Cost".padStart(costWidth))
    + (showBars ? " " + " ".repeat(barWidth) : "")
    + " | " + boldCyan("Tokens".padStart(tokenWidth))
    + " | " + boldCyan("Share".padStart(shareWidth))
    + " | " + boldCyan(deltaHeader.padStart(deltaWidth));
  lines.push(header);
  lines.push(dim(divStr));

  const values = visible.map((r) => metricValue(r.totals, metric));
  // Loop-based max: spreading a large values array into Math.max can hit the
  // argument-count limit (RangeError).
  let maxValue = values.length > 0 ? values[0] : 0;
  for (let i = 1; i < values.length; i++) {
    if (values[i] > maxValue) maxValue = values[i];
  }
  const scale = showBars
    ? computeBarScale(values, barWidth)
    : { mode: "single" as const, max: maxValue };

  for (const row of visible) {
    const value = metricValue(row.totals, metric);
    // The watch delta arrow rides the cell in the displayed metric (Cost under
    // cost, Tokens under tokens), mirroring the snapshot renderer. The arrow
    // is appended to the plain padded text and the exact-zero dim wrap goes
    // around the composite (pad first, then color — width unchanged).
    const costText = fmtCost(row.totals.totalCost).padStart(costWidth)
      + (metric === "cost" ? deltaIndicator(value, leaderboardKey(row), opts.prevCosts) : "");
    const costCell = row.totals.totalCost === 0 ? dim(costText) : costText;
    const tokenText = fmtMetric(row.totals.totalTokens, "tokens").padStart(tokenWidth)
      + (metric === "tokens" ? deltaIndicator(value, leaderboardKey(row), opts.prevCosts) : "");
    const tokenCell = row.totals.totalTokens === 0 ? dim(tokenText) : tokenText;
    const bar = showBars ? renderScaledBar(value, scale, barWidth) : "";
    // The bar sits mid-row (between Cost and Tokens), and renderScaledBar's
    // single-zone path does not pad — pad the bar area to barWidth on every
    // row so Tokens/Share/Δ start at the same offset regardless of bar length.
    const barCell = showBars ? bar + " ".repeat(Math.max(0, barWidth + 1 - stripAnsi(bar).length)) : "";
    lines.push(
      String(row.rank).padStart(rankWidth)
      + " | " + nameCell(row).padEnd(nameWidth)
      + " | " + costCell
      + barCell
      + " | " + tokenCell
      + " | " + shareCell(row.share).padStart(shareWidth)
      + " | " + fmtDeltaCell(row.delta).padStart(deltaWidth),
    );
  }

  if (collapsed > 0) {
    lines.push(
      " ".repeat(rankWidth)
      + " | " + dim(collapsedLabel.padEnd(nameWidth))
      + " | " + " ".repeat(costWidth)
      + (showBars ? " " + " ".repeat(barWidth) : "")
      + " | " + " ".repeat(tokenWidth)
      + " | " + " ".repeat(shareWidth)
      + " | " + " ".repeat(deltaWidth),
    );
  }

  if (rows.length >= 2) {
    lines.push(dim(divStr));
    lines.push(
      boldWhite(" ".repeat(rankWidth))
      + " | " + boldWhite("Total".padEnd(nameWidth))
      + " | " + boldWhite(fmtCost(grandCost).padStart(costWidth))
      + (showBars ? " " + " ".repeat(barWidth) : "")
      + " | " + boldWhite(fmtMetric(grandTokens, "tokens").padStart(tokenWidth))
      + " | " + boldWhite(" ".repeat(shareWidth))
      + " | " + boldWhite(" ".repeat(deltaWidth)),
    );
  }

  // The repo-only read lags until the next sync — surface it (ANSI only;
  // CSV/JSON/MD carry no footer, consistent with the other emitters).
  lines.push(dim(opts.lastSync !== undefined && opts.lastSync !== "never"
    ? `synced ${opts.lastSync} · tu sync to refresh`
    : "never synced · tu sync to refresh"));
  lines.push("");
  return lines;
}

export function printLeaderboard(rows: LeaderboardRenderRow[], opts: LeaderboardRenderOptions): void {
  renderLeaderboard(rows, opts).forEach((l) => console.log(l));
}

// --- Compact mode renderers (watch mode only, narrow terminals) ---

const COMPACT_NAME_W = 14;
const COMPACT_COST_W = 12;
const COMPACT_DIV = "─".repeat(COMPACT_NAME_W + COMPACT_COST_W + 1);

function renderCompactSnapshot(toolTotals: Map<string, UsageTotals>, prevCosts?: Map<string, number>, metric: BarMetric = "cost"): string[] {
  const lines: string[] = [];
  for (const [name, t] of toolTotals) {
    if (t.totalTokens > 0) {
      const costStr = fmtMetricDelta(metricValue(t, metric), name, metric, prevCosts);
      lines.push(`${name.padEnd(COMPACT_NAME_W)} ${costStr.padStart(COMPACT_COST_W)}`);
    }
  }
  let grandCost = 0;
  for (const t of toolTotals.values()) grandCost += metricValue(t, metric);
  const visibleCount = [...toolTotals.values()].filter(t => t.totalTokens > 0).length;
  if (visibleCount > 1) {
    lines.push(dim(COMPACT_DIV));
    lines.push(`${boldWhite("Total".padEnd(COMPACT_NAME_W))} ${boldWhite(fmtMetric(grandCost, metric).padStart(COMPACT_COST_W))}`);
  }
  lines.push("");
  return lines;
}

function renderCompactHistory(entries: UsageEntry[], prevCosts?: Map<string, number>, toolName?: string, metric: BarMetric = "cost"): string[] {
  const lines: string[] = [];
  let sumCost = 0;
  for (const e of entries) {
    const key = toolName ? `${toolName}:${e.label}` : e.label;
    const costStr = fmtMetricDelta(metricValue(e, metric), key, metric, prevCosts);
    lines.push(`${e.label.padEnd(COMPACT_NAME_W)} ${costStr.padStart(COMPACT_COST_W)}`);
    sumCost += metricValue(e, metric);
  }
  if (entries.length > 1) {
    lines.push(dim(COMPACT_DIV));
    lines.push(`${boldWhite("Total".padEnd(COMPACT_NAME_W))} ${boldWhite(fmtMetric(sumCost, metric).padStart(COMPACT_COST_W))}`);
  }
  lines.push("");
  return lines;
}

function renderCompactTotalHistory(labels: string[], valueMap: Map<string, number>, prevCosts?: Map<string, number>, metric: BarMetric = "cost"): string[] {
  const lines: string[] = [];
  let grandTotal = 0;
  for (const label of labels) {
    const value = valueMap.get(label) || 0;
    const costStr = fmtMetricDelta(value, `total:${label}`, metric, prevCosts);
    lines.push(`${label.padEnd(COMPACT_NAME_W)} ${costStr.padStart(COMPACT_COST_W)}`);
    grandTotal += value;
  }
  if (labels.length > 1) {
    lines.push(dim(COMPACT_DIV));
    lines.push(`${boldWhite("Total".padEnd(COMPACT_NAME_W))} ${boldWhite(fmtMetric(grandTotal, metric).padStart(COMPACT_COST_W))}`);
  }
  lines.push("");
  return lines;
}

// ---------------------------------------------------------------------------
// CSV + Markdown renderers
//
// These produce paste-/pipeline-friendly output for the three data kinds
// (snapshot, history, total-history). They share strip rules — no ANSI,
// no inline bars, no delta arrows — but differ in numeric conventions:
//
//   CSV:      raw numbers (no thousands separators), cost without `$`,
//             RFC 4180 quoting, LF line endings, no BOM.
//   Markdown: human-readable numbers (comma thousands), cost with `$`,
//             GFM tables, leading `## {title}` heading, trailing blank line.
// ---------------------------------------------------------------------------

export type EmitKind = "snapshot" | "history" | "total-history" | "leaderboard";

export type EmitData =
  | Map<string, UsageTotals>
  | Map<string, UsageEntry[]>
  | { toolName: string; entries: UsageEntry[] }
  | LeaderboardRenderRow[];

export interface EmitOptions {
  period: string;
  machineCosts?: Map<string, Map<string, number>>;
  capActive?: boolean;  // implicit 3-month history cap active → append "last 3 months" heading hint
  deltaLabel?: string;  // leaderboard: previous-window label for the Δ column header
  totalRows?: LeaderboardRenderRow[];  // leaderboard: full row set the Total row sums (when --top slices the emitted rows)
  byMachine?: boolean;  // leaderboard: explicit --by-machine flag — keeps the machine column in the header even for an empty row set; falls back to row inference when absent
  mdTitle?: string;  // total-history: Markdown heading override (leaderboard history); absent ≡ Combined Cost History
}

// JSON rows for the leaderboard: an array of plain row objects, delta null for
// a "new" row, share a fraction (0.381, not a percent string), machine present
// only under --by-machine.
export function leaderboardRowsToJson(rows: LeaderboardRenderRow[]): Array<Record<string, unknown>> {
  return rows.map((row) => ({
    rank: row.rank,
    user: row.user,
    ...(row.machine !== undefined ? { machine: row.machine } : {}),
    cost: row.totals.totalCost,
    totalTokens: row.totals.totalTokens,
    share: row.share,
    delta: row.delta ?? null,
  }));
}

// --- CSV primitives ---

// RFC 4180 field quoting. Quote fields containing comma, double-quote, or newline;
// double internal double-quotes.
function csvQuote(value: string): string {
  if (value.includes(",") || value.includes('"') || value.includes("\n") || value.includes("\r")) {
    return `"${value.replace(/"/g, '""')}"`;
  }
  return value;
}

function csvRow(cells: string[]): string {
  return cells.map(csvQuote).join(",");
}

// Raw numeric (no thousands separators)
function csvNum(n: number): string {
  return String(n);
}

// Raw cost (two decimals, no `$`)
function csvCost(n: number): string {
  return n.toFixed(2);
}

// Sort machine names alphabetically for deterministic CSV column ordering.
function collectMachineNames(machineCosts?: Map<string, Map<string, number>>): string[] {
  if (!machineCosts || machineCosts.size === 0) return [];
  const names = new Set<string>();
  for (const perMachine of machineCosts.values()) {
    for (const name of perMachine.keys()) names.add(name);
  }
  return [...names].sort();
}

function emitCsvSnapshot(toolTotals: Map<string, UsageTotals>, opts: EmitOptions): string {
  const machines = collectMachineNames(opts.machineCosts);
  const header = ["tool", "tokens", "input", "output", "cache", "cost", ...machines.map((m) => `machine_${m}_cost`)];
  const rows: string[] = [csvRow(header)];

  let grandInput = 0;
  let grandOutput = 0;
  let grandCache = 0;
  let grandTotal = 0;
  let grandCost = 0;
  const machineSums = new Map<string, number>(machines.map((m) => [m, 0]));

  for (const [name, t] of toolTotals) {
    if (t.totalTokens > 0) {
      const toolMachines = opts.machineCosts?.get(name);
      const machineCells = machines.map((m) => {
        const c = toolMachines?.get(m) ?? 0;
        machineSums.set(m, (machineSums.get(m) ?? 0) + c);
        return csvCost(c);
      });
      rows.push(csvRow([name, csvNum(t.totalTokens), csvNum(t.inputTokens), csvNum(t.outputTokens), csvNum(t.cacheCreationTokens + t.cacheReadTokens), csvCost(t.totalCost), ...machineCells]));
    }
    grandInput += t.inputTokens;
    grandOutput += t.outputTokens;
    grandCache += t.cacheCreationTokens + t.cacheReadTokens;
    grandTotal += t.totalTokens;
    grandCost += t.totalCost;
  }

  const visibleCount = [...toolTotals.values()].filter((t) => t.totalTokens > 0).length;
  if (visibleCount > 1) {
    const totalMachines = machines.map((m) => csvCost(machineSums.get(m) ?? 0));
    rows.push(csvRow(["Total", csvNum(grandTotal), csvNum(grandInput), csvNum(grandOutput), csvNum(grandCache), csvCost(grandCost), ...totalMachines]));
  }

  return rows.join("\n") + "\n";
}

function emitCsvHistory(entries: UsageEntry[], opts: EmitOptions): string {
  const machines = collectMachineNames(opts.machineCosts);
  const header = ["date", "input", "output", "cache_write", "cache_read", "total", "cost", ...machines.map((m) => `machine_${m}_cost`)];
  const rows: string[] = [csvRow(header)];

  for (const e of entries) {
    const labelMachines = opts.machineCosts?.get(e.label);
    const machineCells = machines.map((m) => csvCost(labelMachines?.get(m) ?? 0));
    rows.push(csvRow([
      e.label,
      csvNum(e.inputTokens),
      csvNum(e.outputTokens),
      csvNum(e.cacheCreationTokens),
      csvNum(e.cacheReadTokens),
      csvNum(e.totalTokens),
      csvCost(e.totalCost),
      ...machineCells,
    ]));
  }

  return rows.join("\n") + "\n";
}

function emitCsvTotalHistory(allToolEntries: Map<string, UsageEntry[]>, opts: EmitOptions): string {
  const toolNames = [...allToolEntries.keys()];

  const labelSet = new Set<string>();
  for (const entries of allToolEntries.values()) {
    for (const e of entries) labelSet.add(e.label);
  }
  const labels = [...labelSet].sort();

  const costMap = new Map<string, Map<string, number>>();
  for (const [tool, entries] of allToolEntries) {
    const m = new Map<string, number>();
    for (const e of entries) m.set(e.label, e.totalCost);
    costMap.set(tool, m);
  }

  const machines = collectMachineNames(opts.machineCosts);
  const header = ["date", ...toolNames, "total", ...machines.map((m) => `machine_${m}_cost`)];
  const rows: string[] = [csvRow(header)];

  for (const label of labels) {
    const cells: string[] = [label];
    let rowTotal = 0;
    for (const tool of toolNames) {
      const cost = costMap.get(tool)?.get(label) ?? 0;
      cells.push(csvCost(cost));
      rowTotal += cost;
    }
    cells.push(csvCost(rowTotal));
    const labelMachines = opts.machineCosts?.get(label);
    for (const m of machines) cells.push(csvCost(labelMachines?.get(m) ?? 0));
    rows.push(csvRow(cells));
  }

  return rows.join("\n") + "\n";
}

// Leaderboard CSV: raw numbers (machine contract) — no `$`, no thousands
// separators, no ANSI/bars/arrows; delta empty for a "new" row; a machine
// column follows user under --by-machine; a Total row when >1 row. The Total
// sums opts.totalRows (the full row set) when given — collapsed --top rows
// still count toward it. The machine column comes from the explicit
// opts.byMachine flag (falling back to row inference) so an empty leaderboard
// still emits the same header schema.
function emitCsvLeaderboard(rows: LeaderboardRenderRow[], opts: EmitOptions): string {
  const byMachine = opts.byMachine ?? rows.some((r) => r.machine !== undefined);
  const header = byMachine
    ? ["rank", "user", "machine", "cost", "total_tokens", "share", "delta"]
    : ["rank", "user", "cost", "total_tokens", "share", "delta"];
  const out: string[] = [csvRow(header)];

  for (const row of rows) {
    const cells = [String(row.rank), row.user];
    if (byMachine) cells.push(row.machine ?? "");
    cells.push(csvCost(row.totals.totalCost), csvNum(row.totals.totalTokens), csvShare(row.share), row.delta !== undefined ? csvShare(row.delta) : "");
    out.push(csvRow(cells));
  }

  const totalRows = opts.totalRows ?? rows;
  if (totalRows.length > 1) {
    let grandCost = 0;
    let grandTokens = 0;
    for (const row of totalRows) {
      grandCost += row.totals.totalCost;
      grandTokens += row.totals.totalTokens;
    }
    const cells = ["Total", ""];
    if (byMachine) cells.push("");
    cells.push(csvCost(grandCost), csvNum(grandTokens), "", "");
    out.push(csvRow(cells));
  }

  return out.join("\n") + "\n";
}

// Raw fraction for CSV (share/delta) — a plain number, no percent sign.
function csvShare(n: number): string {
  return String(Math.round(n * 1000) / 1000);
}

export function emitCsv(data: EmitData, kind: EmitKind, opts: EmitOptions): void {
  let output: string;
  switch (kind) {
    case "snapshot":
      output = emitCsvSnapshot(data as Map<string, UsageTotals>, opts);
      break;
    case "history": {
      const { entries } = data as { toolName: string; entries: UsageEntry[] };
      output = emitCsvHistory(entries, opts);
      break;
    }
    case "total-history":
      output = emitCsvTotalHistory(data as Map<string, UsageEntry[]>, opts);
      break;
    case "leaderboard":
      output = emitCsvLeaderboard(data as LeaderboardRenderRow[], opts);
      break;
  }
  process.stdout.write(output);
}

// --- Markdown primitives ---

// Markdown numeric values keep thousands separators for readability (targets
// paste into PRs/Slack/docs rather than awk pipelines).
function mdNum(n: number): string {
  return fmtNum(n);
}

function mdCost(n: number): string {
  return fmtCost(n);
}

function mdRow(cells: string[]): string {
  return `| ${cells.join(" | ")} |`;
}

// GFM alignment markers — `:---` left, `---:` right.
function mdAlignRow(aligns: Array<"left" | "right">): string {
  return `| ${aligns.map((a) => (a === "left" ? ":---" : "---:")).join(" | ")} |`;
}

function titleForSnapshot(period: string): string {
  return `Combined Usage (${period})`;
}

function titleForHistory(toolName: string, period: string, capActive?: boolean): string {
  return `${toolName} (${periodLabel(period, capActive)})`;
}

function titleForTotalHistory(period: string, capActive?: boolean): string {
  return `Combined Cost History (${periodLabel(period, capActive)})`;
}

function titleForLeaderboard(period: string): string {
  return `Leaderboard (${period})`;
}

// Leaderboard Markdown: GFM table, left-aligned strings / right-aligned
// numerics, `$`-prefixed costs with thousands separators, **Total** bolded,
// delta rendered "new" when undefined. No bars, no arrows, no staleness footer.
// The Machine column comes from the explicit opts.byMachine flag (falling back
// to row inference) so an empty leaderboard keeps the same header schema.
function emitMarkdownLeaderboard(rows: LeaderboardRenderRow[], opts: EmitOptions): string {
  const byMachine = opts.byMachine ?? rows.some((r) => r.machine !== undefined);
  const deltaHeader = `Δ vs ${opts.deltaLabel ?? "prev"}`;
  const aligns: Array<"left" | "right"> = byMachine
    ? ["right", "left", "left", "right", "right", "right", "right"]
    : ["right", "left", "right", "right", "right", "right"];
  const header = byMachine
    ? ["#", "User", "Machine", "Cost", "Tokens", "Share", deltaHeader]
    : ["#", "User", "Cost", "Tokens", "Share", deltaHeader];

  const lines: string[] = [];
  lines.push(`## ${titleForLeaderboard(opts.period)}`);
  lines.push("");
  lines.push(mdRow(header));
  lines.push(mdAlignRow(aligns));

  for (const row of rows) {
    const cells = [String(row.rank), row.user];
    if (byMachine) cells.push(row.machine ?? "");
    cells.push(mdCost(row.totals.totalCost), mdNum(row.totals.totalTokens), mdShare(row.share), row.delta !== undefined ? mdDelta(row.delta) : "new");
    lines.push(mdRow(cells));
  }

  // The Total sums the full row set when given (collapsed --top rows count).
  const totalRows = opts.totalRows ?? rows;
  if (totalRows.length > 1) {
    let grandCost = 0;
    let grandTokens = 0;
    for (const row of totalRows) {
      grandCost += row.totals.totalCost;
      grandTokens += row.totals.totalTokens;
    }
    const cells = ["**Total**", ""];
    if (byMachine) cells.push("");
    cells.push(`**${mdCost(grandCost)}**`, `**${mdNum(grandTokens)}**`, "", "");
    lines.push(mdRow(cells));
  }

  lines.push("");
  return lines.join("\n");
}

// Human-readable percent cell (Markdown) — "38.1%", "+12%", "-4%". Shares are
// unsigned fractions of the total; deltas are signed.
function mdShare(n: number): string {
  return `${(n * 100).toFixed(1)}%`;
}

function mdDelta(n: number): string {
  const pct = Math.round(n * 100);
  return `${pct >= 0 ? "+" : ""}${pct}%`;
}

function emitMarkdownSnapshot(toolTotals: Map<string, UsageTotals>, opts: EmitOptions): string {
  const machines = collectMachineNames(opts.machineCosts);
  const aligns: Array<"left" | "right"> = ["left", "right", "right", "right", "right", "right", ...machines.map((_) => "right" as const)];
  const header = ["Tool", "Tokens", "Input", "Output", "Cache", "Cost", ...machines];

  const lines: string[] = [];
  lines.push(`## ${titleForSnapshot(opts.period)}`);
  lines.push("");
  lines.push(mdRow(header));
  lines.push(mdAlignRow(aligns));

  let grandInput = 0;
  let grandOutput = 0;
  let grandCache = 0;
  let grandTotal = 0;
  let grandCost = 0;
  const machineSums = new Map<string, number>(machines.map((m) => [m, 0]));

  for (const [name, t] of toolTotals) {
    if (t.totalTokens > 0) {
      const toolMachines = opts.machineCosts?.get(name);
      const machineCells = machines.map((m) => {
        const c = toolMachines?.get(m) ?? 0;
        machineSums.set(m, (machineSums.get(m) ?? 0) + c);
        return mdCost(c);
      });
      lines.push(mdRow([name, mdNum(t.totalTokens), mdNum(t.inputTokens), mdNum(t.outputTokens), mdNum(t.cacheCreationTokens + t.cacheReadTokens), mdCost(t.totalCost), ...machineCells]));
    }
    grandInput += t.inputTokens;
    grandOutput += t.outputTokens;
    grandCache += t.cacheCreationTokens + t.cacheReadTokens;
    grandTotal += t.totalTokens;
    grandCost += t.totalCost;
  }

  const visibleCount = [...toolTotals.values()].filter((t) => t.totalTokens > 0).length;
  if (visibleCount > 1) {
    const totalMachines = machines.map((m) => `**${mdCost(machineSums.get(m) ?? 0)}**`);
    lines.push(mdRow(["**Total**", `**${mdNum(grandTotal)}**`, `**${mdNum(grandInput)}**`, `**${mdNum(grandOutput)}**`, `**${mdNum(grandCache)}**`, `**${mdCost(grandCost)}**`, ...totalMachines]));
  }

  lines.push("");
  return lines.join("\n");
}

function emitMarkdownHistory(toolName: string, entries: UsageEntry[], opts: EmitOptions): string {
  const machines = collectMachineNames(opts.machineCosts);
  const aligns: Array<"left" | "right"> = ["left", "right", "right", "right", "right", "right", "right", ...machines.map((_) => "right" as const)];
  const header = ["Date", "Input", "Output", "Cache Write", "Cache Read", "Total", "Cost", ...machines];

  const lines: string[] = [];
  lines.push(`## ${titleForHistory(toolName, opts.period, opts.capActive)}`);
  lines.push("");
  lines.push(mdRow(header));
  lines.push(mdAlignRow(aligns));

  let sumInput = 0;
  let sumOutput = 0;
  let sumCacheW = 0;
  let sumCacheR = 0;
  let sumTotal = 0;
  let sumCost = 0;
  const machineSums = new Map<string, number>(machines.map((m) => [m, 0]));

  for (const e of entries) {
    const labelMachines = opts.machineCosts?.get(e.label);
    const machineCells = machines.map((m) => {
      const c = labelMachines?.get(m) ?? 0;
      machineSums.set(m, (machineSums.get(m) ?? 0) + c);
      return mdCost(c);
    });
    lines.push(mdRow([
      e.label,
      mdNum(e.inputTokens),
      mdNum(e.outputTokens),
      mdNum(e.cacheCreationTokens),
      mdNum(e.cacheReadTokens),
      mdNum(e.totalTokens),
      mdCost(e.totalCost),
      ...machineCells,
    ]));
    sumInput += e.inputTokens;
    sumOutput += e.outputTokens;
    sumCacheW += e.cacheCreationTokens;
    sumCacheR += e.cacheReadTokens;
    sumTotal += e.totalTokens;
    sumCost += e.totalCost;
  }

  if (entries.length > 1) {
    const totalMachines = machines.map((m) => `**${mdCost(machineSums.get(m) ?? 0)}**`);
    lines.push(mdRow([
      "**Total**",
      `**${mdNum(sumInput)}**`,
      `**${mdNum(sumOutput)}**`,
      `**${mdNum(sumCacheW)}**`,
      `**${mdNum(sumCacheR)}**`,
      `**${mdNum(sumTotal)}**`,
      `**${mdCost(sumCost)}**`,
      ...totalMachines,
    ]));
  }

  lines.push("");
  return lines.join("\n");
}

function emitMarkdownTotalHistory(allToolEntries: Map<string, UsageEntry[]>, opts: EmitOptions): string {
  const allToolNames = [...allToolEntries.keys()];

  const labelSet = new Set<string>();
  for (const entries of allToolEntries.values()) {
    for (const e of entries) labelSet.add(e.label);
  }
  const labels = [...labelSet].sort();

  const costMap = new Map<string, Map<string, number>>();
  for (const [tool, entries] of allToolEntries) {
    const m = new Map<string, number>();
    for (const e of entries) m.set(e.label, e.totalCost);
    costMap.set(tool, m);
  }

  // Same zero-column omission as the ANSI pivot (over all labels — the emit
  // path has no maxRows window); CSV stays unfiltered (positional contract).
  const toolNames = nonzeroTools(allToolNames, costMap, labels);

  const machines = collectMachineNames(opts.machineCosts);
  const aligns: Array<"left" | "right"> = [
    "left",
    ...toolNames.map((_) => "right" as const),
    "right",
    ...machines.map((_) => "right" as const),
  ];
  const header = ["Date", ...toolNames, "Cost", ...machines];

  const lines: string[] = [];
  lines.push(`## ${opts.mdTitle ?? titleForTotalHistory(opts.period, opts.capActive)}`);
  lines.push("");
  lines.push(mdRow(header));
  lines.push(mdAlignRow(aligns));

  const toolSums = new Map<string, number>(toolNames.map((t) => [t, 0]));
  const machineSums = new Map<string, number>(machines.map((m) => [m, 0]));
  let grandTotal = 0;

  for (const label of labels) {
    const cells: string[] = [label];
    let rowTotal = 0;
    for (const tool of toolNames) {
      const cost = costMap.get(tool)?.get(label) ?? 0;
      cells.push(mdCost(cost));
      toolSums.set(tool, (toolSums.get(tool) ?? 0) + cost);
      rowTotal += cost;
    }
    cells.push(mdCost(rowTotal));
    grandTotal += rowTotal;
    const labelMachines = opts.machineCosts?.get(label);
    for (const m of machines) {
      const c = labelMachines?.get(m) ?? 0;
      machineSums.set(m, (machineSums.get(m) ?? 0) + c);
      cells.push(mdCost(c));
    }
    lines.push(mdRow(cells));
  }

  if (labels.length > 1) {
    const totalCells: string[] = ["**Total**"];
    for (const tool of toolNames) totalCells.push(`**${mdCost(toolSums.get(tool) ?? 0)}**`);
    totalCells.push(`**${mdCost(grandTotal)}**`);
    for (const m of machines) totalCells.push(`**${mdCost(machineSums.get(m) ?? 0)}**`);
    lines.push(mdRow(totalCells));
  }

  lines.push("");
  return lines.join("\n");
}

export function emitMarkdown(data: EmitData, kind: EmitKind, opts: EmitOptions): void {
  let output: string;
  switch (kind) {
    case "snapshot":
      output = emitMarkdownSnapshot(data as Map<string, UsageTotals>, opts);
      break;
    case "history": {
      const { toolName, entries } = data as { toolName: string; entries: UsageEntry[] };
      output = emitMarkdownHistory(toolName, entries, opts);
      break;
    }
    case "total-history":
      output = emitMarkdownTotalHistory(data as Map<string, UsageEntry[]>, opts);
      break;
    case "leaderboard":
      output = emitMarkdownLeaderboard(data as LeaderboardRenderRow[], opts);
      break;
  }
  process.stdout.write(output + "\n");
}
