import type { UsageTotals, UsageEntry } from "../core/types.js";
import { bold, dim, green, red, cyan, yellow, boldWhite, boldCyan } from "./colors.js";

export interface FormatOptions {
  prevCosts?: Map<string, number>;  // key: "{toolName}:{label}" or "{toolName}"
  compact?: boolean;
  maxRows?: number;  // truncate history to most recent N data rows (watch mode)
}

export function fmtNum(n: number): string {
  return n.toLocaleString("en-US");
}

export function fmtCost(n: number): string {
  return `$${n.toFixed(2)}`;
}

export function deltaIndicator(current: number, key: string, prevCosts?: Map<string, number>): string {
  if (!prevCosts) return "";
  const prev = prevCosts.get(key);
  if (prev === undefined) return "";
  if (current > prev) return " " + green("\u2191");
  if (current < prev) return " " + red("\u2193");
  return "";
}

export function fmtCostDelta(current: number, key: string, prevCosts?: Map<string, number>): string {
  return fmtCost(current) + deltaIndicator(current, key, prevCosts);
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
const COST_WIDTH = 8;

export function renderBar(value: number, maxValue: number, barWidth: number): string {
  if (value === 0 || maxValue === 0) return "";
  const scaled = (value / maxValue) * barWidth;
  const fullBlocks = Math.floor(scaled);
  const eighths = Math.round((scaled - fullBlocks) * 8);

  if (eighths === 8) return FULL_BLOCK.repeat(fullBlocks + 1);

  const bar = FULL_BLOCK.repeat(fullBlocks) + BLOCK_EIGHTHS[eighths];
  return bar.length > 0 ? bar : MIN_BAR;
}

// --- Single-tool history table (tu cc daily, tu codex monthly, etc.) ---

export function renderHistory(toolName: string, period: string, entries: UsageEntry[], termWidth?: number, opts?: FormatOptions): string[] {
  const lines: string[] = [];
  lines.push("");
  lines.push(boldWhite(`\u{1F4CA} ${toolName} (${period})`));
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

  // Compact mode: date + cost only
  if (opts?.compact) {
    lines.push(...renderCompactHistory(entries, opts.prevCosts, toolName));
    return lines;
  }

  const D = 12;
  const N = 14;
  const numCols = 6;
  const tableWidth = D + (numCols - 1) * N + (numCols - 1) * 3;
  const width = termWidth ?? process.stdout.columns ?? 80;
  const barWidth = Math.min(width - tableWidth - GUTTER - COST_WIDTH - 1, MAX_BAR_WIDTH);
  const showBars = barWidth >= MIN_BAR_AREA;

  const row = (...cols: string[]) => cols.map((c, i) => (i === 0 ? c.padEnd(D) : c.padStart(N))).join(" | ");
  const colorRow = (cols: string[], colorFn: (s: string) => string) =>
    cols.map((c, i) => colorFn(i === 0 ? c.padEnd(D) : c.padStart(N))).join(" | ");
  const divStr = [D, N, N, N, N, N].map(w => "─".repeat(w)).join("─|─");
  const costDiv = "─|─" + "─".repeat(COST_WIDTH);
  const barDiv = showBars ? "─" + "─".repeat(barWidth) : "";

  const costHeader = " | " + boldCyan("Cost".padStart(COST_WIDTH));
  lines.push(colorRow(["Date", "Input", "Output", "Cache Write", "Cache Read", "Total"], boldCyan) + costHeader);
  lines.push(dim(divStr + costDiv + barDiv));

  const maxCost = Math.max(...entries.map((e) => e.totalCost));
  const prevCosts = opts?.prevCosts;

  let sumCost = 0;
  let sumInput = 0;
  let sumOutput = 0;
  let sumCacheW = 0;
  let sumCacheR = 0;
  let sumTotal = 0;

  for (const e of entries) {
    const rowStr = row(e.label, fmtNum(e.inputTokens), fmtNum(e.outputTokens), fmtNum(e.cacheCreationTokens), fmtNum(e.cacheReadTokens), fmtNum(e.totalTokens));
    const costBase = " | " + fmtCost(e.totalCost).padStart(COST_WIDTH);
    const indicator = deltaIndicator(e.totalCost, `${toolName}:${e.label}`, prevCosts);
    const rawBar = showBars ? renderBar(e.totalCost, maxCost, barWidth) : "";
    const bar = rawBar ? " " + green(rawBar) : "";
    lines.push(rowStr + costBase + indicator + bar);
    sumCost += e.totalCost;
    sumInput += e.inputTokens;
    sumOutput += e.outputTokens;
    sumCacheW += e.cacheCreationTokens;
    sumCacheR += e.cacheReadTokens;
    sumTotal += e.totalTokens;
  }

  if (entries.length > 1) {
    lines.push(dim(divStr + costDiv + barDiv));
    const totalRow = colorRow(["Total", fmtNum(sumInput), fmtNum(sumOutput), fmtNum(sumCacheW), fmtNum(sumCacheR), fmtNum(sumTotal)], boldWhite);
    const totalCost = " | " + boldWhite(fmtCost(sumCost).padStart(COST_WIDTH));
    lines.push(totalRow + totalCost);
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

  // Compact mode: name + cost only
  if (opts?.compact) {
    lines.push(...renderCompactSnapshot(toolTotals, opts.prevCosts));
    return lines;
  }

  const W = 14;
  const N = 14;
  const prevCosts = opts?.prevCosts;
  const row = (...cols: string[]) =>
    cols.map((c, i) => (i === 0 ? c.padEnd(W) : c.padStart(N))).join(" | ");
  const colorRow = (cols: string[], colorFn: (s: string) => string) =>
    cols.map((c, i) => colorFn(i === 0 ? c.padEnd(W) : c.padStart(N))).join(" | ");
  const divider = [W, N, N, N, N].map(w => "─".repeat(w)).join("─|─");

  lines.push(colorRow(["Tool", "Tokens", "Input", "Output", "Cost"], boldCyan));
  lines.push(dim(divider));

  let grandCost = 0;
  let grandInput = 0;
  let grandOutput = 0;
  let grandTotal = 0;

  for (const [name, t] of toolTotals) {
    if (t.totalTokens > 0) {
      const costStr = fmtCostDelta(t.totalCost, name, prevCosts);
      lines.push(row(name, fmtNum(t.totalTokens), fmtNum(t.inputTokens), fmtNum(t.outputTokens), costStr));
    }
    grandCost += t.totalCost;
    grandInput += t.inputTokens;
    grandOutput += t.outputTokens;
    grandTotal += t.totalTokens;
  }

  const visibleCount = [...toolTotals.values()].filter(t => t.totalTokens > 0).length;
  if (visibleCount > 1) {
    lines.push(dim(divider));
    lines.push(colorRow(["Total", fmtNum(grandTotal), fmtNum(grandInput), fmtNum(grandOutput), fmtCost(grandCost)], boldWhite));
  }
  lines.push("");
  return lines;
}

export function printTotal(period: string, toolTotals: Map<string, UsageTotals>, opts?: FormatOptions): void {
  renderTotal(period, toolTotals, opts).forEach((l) => console.log(l));
}

// --- Cross-tool history pivot (tu total-history daily, tu total-history monthly) ---
// Y-axis = dates, X-axis = tool names, cell = cost

export function renderTotalHistory(period: string, allToolEntries: Map<string, UsageEntry[]>, termWidth?: number, opts?: FormatOptions): string[] {
  const lines: string[] = [];
  lines.push("");
  lines.push(boldWhite(`\u{1F4CA} Combined Cost History (${period})`));
  lines.push("");

  const toolNames = [...allToolEntries.keys()];

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

  // Compact mode: date + total cost only
  if (opts?.compact) {
    const costMap = new Map<string, number>();
    for (const entries of allToolEntries.values()) {
      for (const e of entries) {
        costMap.set(e.label, (costMap.get(e.label) || 0) + e.totalCost);
      }
    }
    lines.push(...renderCompactTotalHistory(labels, costMap, opts.prevCosts));
    return lines;
  }

  // Build lookup: tool -> label -> cost
  const costMap = new Map<string, Map<string, number>>();
  for (const [tool, entries] of allToolEntries) {
    const m = new Map<string, number>();
    for (const e of entries) m.set(e.label, e.totalCost);
    costMap.set(tool, m);
  }

  const D = 12;
  const N = 14;
  const colCount = toolNames.length + 1; // Date + tools (Total moved to merged area)
  const tableWidth = D + (colCount - 1) * (N + 3);
  const width = termWidth ?? process.stdout.columns ?? 80;
  const barWidth = Math.min(width - tableWidth - GUTTER - COST_WIDTH - 1, MAX_BAR_WIDTH);
  const showBars = barWidth >= MIN_BAR_AREA;

  const row = (...cols: string[]) => cols.map((c, i) => (i === 0 ? c.padEnd(D) : c.padStart(N))).join(" | ");
  const colorRow = (cols: string[], colorFn: (s: string) => string) =>
    cols.map((c, i) => colorFn(i === 0 ? c.padEnd(D) : c.padStart(N))).join(" | ");
  const divStr = [D, ...toolNames.map(() => N)].map((w) => "─".repeat(w)).join("─|─");
  const costDiv = "─|─" + "─".repeat(COST_WIDTH);
  const barDiv = showBars ? "─" + "─".repeat(barWidth) : "";

  const costHeader = " | " + boldCyan("Cost".padStart(COST_WIDTH));
  lines.push(colorRow(["Date", ...toolNames], boldCyan) + costHeader);
  lines.push(dim(divStr + costDiv + barDiv));

  // Pre-compute row totals for maxCost (needed for bar scaling)
  const rowData: { label: string; cells: string[]; rowTotal: number }[] = [];
  const toolSums = new Map<string, number>(toolNames.map((t) => [t, 0]));
  let grandTotal = 0;

  for (const label of labels) {
    let rowTotal = 0;
    const cells: string[] = [];
    for (const tool of toolNames) {
      const cost = costMap.get(tool)?.get(label) || 0;
      cells.push(fmtCost(cost));
      toolSums.set(tool, (toolSums.get(tool) || 0) + cost);
      rowTotal += cost;
    }
    grandTotal += rowTotal;
    rowData.push({ label, cells, rowTotal });
  }

  const maxCost = Math.max(...rowData.map((r) => r.rowTotal));

  const prevCosts = opts?.prevCosts;

  for (const r of rowData) {
    const rowStr = row(r.label, ...r.cells);
    const costBase = " | " + fmtCost(r.rowTotal).padStart(COST_WIDTH);
    const indicator = deltaIndicator(r.rowTotal, `total:${r.label}`, prevCosts);
    const rawBar = showBars ? renderBar(r.rowTotal, maxCost, barWidth) : "";
    const bar = rawBar ? " " + green(rawBar) : "";
    lines.push(rowStr + costBase + indicator + bar);
  }

  if (labels.length > 1) {
    lines.push(dim(divStr + costDiv + barDiv));
    const sumCells = toolNames.map((t) => fmtCost(toolSums.get(t) || 0));
    const totalRow = colorRow(["Total", ...sumCells], boldWhite);
    const totalCost = " | " + boldWhite(fmtCost(grandTotal).padStart(COST_WIDTH));
    lines.push(totalRow + totalCost);
  }
  lines.push("");
  return lines;
}

export function printTotalHistory(period: string, allToolEntries: Map<string, UsageEntry[]>, termWidth?: number, opts?: FormatOptions): void {
  renderTotalHistory(period, allToolEntries, termWidth, opts).forEach((l) => console.log(l));
}

// --- Compact mode renderers (watch mode only, narrow terminals) ---

const COMPACT_NAME_W = 14;
const COMPACT_COST_W = 12;
const COMPACT_DIV = "─".repeat(COMPACT_NAME_W + COMPACT_COST_W + 1);

function renderCompactSnapshot(toolTotals: Map<string, UsageTotals>, prevCosts?: Map<string, number>): string[] {
  const lines: string[] = [];
  for (const [name, t] of toolTotals) {
    if (t.totalTokens > 0) {
      const costStr = fmtCostDelta(t.totalCost, name, prevCosts);
      lines.push(`${name.padEnd(COMPACT_NAME_W)} ${costStr.padStart(COMPACT_COST_W)}`);
    }
  }
  let grandCost = 0;
  for (const t of toolTotals.values()) grandCost += t.totalCost;
  const visibleCount = [...toolTotals.values()].filter(t => t.totalTokens > 0).length;
  if (visibleCount > 1) {
    lines.push(dim(COMPACT_DIV));
    lines.push(`${boldWhite("Total".padEnd(COMPACT_NAME_W))} ${boldWhite(fmtCost(grandCost).padStart(COMPACT_COST_W))}`);
  }
  lines.push("");
  return lines;
}

function renderCompactHistory(entries: UsageEntry[], prevCosts?: Map<string, number>, toolName?: string): string[] {
  const lines: string[] = [];
  let sumCost = 0;
  for (const e of entries) {
    const key = toolName ? `${toolName}:${e.label}` : e.label;
    const costStr = fmtCostDelta(e.totalCost, key, prevCosts);
    lines.push(`${e.label.padEnd(COMPACT_NAME_W)} ${costStr.padStart(COMPACT_COST_W)}`);
    sumCost += e.totalCost;
  }
  if (entries.length > 1) {
    lines.push(dim(COMPACT_DIV));
    lines.push(`${boldWhite("Total".padEnd(COMPACT_NAME_W))} ${boldWhite(fmtCost(sumCost).padStart(COMPACT_COST_W))}`);
  }
  lines.push("");
  return lines;
}

function renderCompactTotalHistory(labels: string[], costMap: Map<string, number>, prevCosts?: Map<string, number>): string[] {
  const lines: string[] = [];
  let grandTotal = 0;
  for (const label of labels) {
    const cost = costMap.get(label) || 0;
    const costStr = fmtCostDelta(cost, `total:${label}`, prevCosts);
    lines.push(`${label.padEnd(COMPACT_NAME_W)} ${costStr.padStart(COMPACT_COST_W)}`);
    grandTotal += cost;
  }
  if (labels.length > 1) {
    lines.push(dim(COMPACT_DIV));
    lines.push(`${boldWhite("Total".padEnd(COMPACT_NAME_W))} ${boldWhite(fmtCost(grandTotal).padStart(COMPACT_COST_W))}`);
  }
  lines.push("");
  return lines;
}
