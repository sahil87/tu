import type { UsageTotals, UsageEntry } from "../core/types.js";
import { bold, dim, green, red, cyan, yellow, boldWhite, boldCyan } from "./colors.js";

export interface FormatOptions {
  prevCosts?: Map<string, number>;  // key: "{toolName}:{label}" or "{toolName}"
  compact?: boolean;
  maxRows?: number;  // truncate history to most recent N data rows (watch mode)
  machineCosts?: Map<string, Map<string, number>>;  // key: label/toolName → (machine → cost)
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

export function renderMachineLegend(columns: MachineColumn[]): string {
  return "Machines: " + columns.map((c) => `${c.letter} = ${c.name}`).join(", ");
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

  // Machine columns setup (computed before bar width so we can subtract their width)
  const machineCosts = opts?.machineCosts;
  const machineNames = machineCosts ? [...new Set([...machineCosts.values()].flatMap((m) => [...m.keys()]))] : [];
  const mcols = buildMachineColumns(machineNames);
  const hasMachines = mcols.length > 0;
  const machineColsWidth = hasMachines ? mcols.length * (MACHINE_COL_WIDTH + 3) : 0; // " | " + padded cost

  const barWidth = Math.min(width - tableWidth - GUTTER - COST_WIDTH - machineColsWidth - 1, MAX_BAR_WIDTH);
  const showBars = barWidth >= MIN_BAR_AREA;

  const row = (...cols: string[]) => cols.map((c, i) => (i === 0 ? c.padEnd(D) : c.padStart(N))).join(" | ");
  const colorRow = (cols: string[], colorFn: (s: string) => string) =>
    cols.map((c, i) => colorFn(i === 0 ? c.padEnd(D) : c.padStart(N))).join(" | ");
  const divStr = [D, N, N, N, N, N].map(w => "─".repeat(w)).join("─|─");
  const costDiv = "─|─" + "─".repeat(COST_WIDTH);
  const barDiv = showBars ? "─" + "─".repeat(barWidth) : "";
  const machineDiv = hasMachines ? mcols.map(() => "─|─" + "─".repeat(MACHINE_COL_WIDTH)).join("") : "";
  const machineHeader = hasMachines ? mcols.map((c) => " | " + boldCyan(c.letter.padStart(MACHINE_COL_WIDTH))).join("") : "";

  const costHeader = " | " + boldCyan("Cost".padStart(COST_WIDTH));
  lines.push(colorRow(["Date", "Input", "Output", "Cache Write", "Cache Read", "Total"], boldCyan) + costHeader + machineHeader);
  lines.push(dim(divStr + costDiv + machineDiv + barDiv));

  const maxCost = Math.max(...entries.map((e) => e.totalCost));
  const prevCosts = opts?.prevCosts;

  let sumCost = 0;
  let sumInput = 0;
  let sumOutput = 0;
  let sumCacheW = 0;
  let sumCacheR = 0;
  let sumTotal = 0;
  const machineSums = new Map<string, number>();

  for (const e of entries) {
    const rowStr = row(e.label, fmtNum(e.inputTokens), fmtNum(e.outputTokens), fmtNum(e.cacheCreationTokens), fmtNum(e.cacheReadTokens), fmtNum(e.totalTokens));
    const costBase = " | " + fmtCost(e.totalCost).padStart(COST_WIDTH);
    const indicator = deltaIndicator(e.totalCost, `${toolName}:${e.label}`, prevCosts);

    let machineCells = "";
    if (hasMachines) {
      const rowMachines = machineCosts?.get(e.label);
      for (const mc of mcols) {
        const cost = rowMachines?.get(mc.name) ?? 0;
        machineCells += " | " + fmtCost(cost).padStart(MACHINE_COL_WIDTH);
        machineSums.set(mc.name, (machineSums.get(mc.name) ?? 0) + cost);
      }
    }

    const rawBar = showBars ? renderBar(e.totalCost, maxCost, barWidth) : "";
    const bar = rawBar ? " " + green(rawBar) : "";
    lines.push(rowStr + costBase + machineCells + indicator + bar);
    sumCost += e.totalCost;
    sumInput += e.inputTokens;
    sumOutput += e.outputTokens;
    sumCacheW += e.cacheCreationTokens;
    sumCacheR += e.cacheReadTokens;
    sumTotal += e.totalTokens;
  }

  if (entries.length > 1) {
    lines.push(dim(divStr + costDiv + machineDiv + barDiv));
    const totalRow = colorRow(["Total", fmtNum(sumInput), fmtNum(sumOutput), fmtNum(sumCacheW), fmtNum(sumCacheR), fmtNum(sumTotal)], boldWhite);
    const totalCost = " | " + boldWhite(fmtCost(sumCost).padStart(COST_WIDTH));
    let totalMachineCells = "";
    if (hasMachines) {
      for (const mc of mcols) {
        totalMachineCells += " | " + boldWhite(fmtCost(machineSums.get(mc.name) ?? 0).padStart(MACHINE_COL_WIDTH));
      }
    }
    lines.push(totalRow + totalCost + totalMachineCells);
  }
  if (hasMachines) {
    lines.push("");
    lines.push(dim(renderMachineLegend(mcols)));
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

  // Machine columns setup
  const machineCosts = opts?.machineCosts;
  const machineNames = machineCosts ? [...new Set([...machineCosts.values()].flatMap((m) => [...m.keys()]))] : [];
  const mcols = buildMachineColumns(machineNames);
  const hasMachines = mcols.length > 0;
  const machineDiv = hasMachines ? mcols.map(() => "─|─" + "─".repeat(MACHINE_COL_WIDTH)).join("") : "";
  const machineHeader = hasMachines ? mcols.map((c) => " | " + boldCyan(c.letter.padStart(MACHINE_COL_WIDTH))).join("") : "";

  lines.push(colorRow(["Tool", "Tokens", "Input", "Output", "Cost"], boldCyan) + machineHeader);
  lines.push(dim(divider + machineDiv));

  let grandCost = 0;
  let grandInput = 0;
  let grandOutput = 0;
  let grandTotal = 0;
  const machineSums = new Map<string, number>();

  for (const [name, t] of toolTotals) {
    if (t.totalTokens > 0) {
      const costStr = fmtCostDelta(t.totalCost, name, prevCosts);
      let machineCells = "";
      if (hasMachines) {
        const toolMachines = machineCosts?.get(name);
        for (const mc of mcols) {
          const cost = toolMachines?.get(mc.name) ?? 0;
          machineCells += " | " + fmtCost(cost).padStart(MACHINE_COL_WIDTH);
          machineSums.set(mc.name, (machineSums.get(mc.name) ?? 0) + cost);
        }
      }
      lines.push(row(name, fmtNum(t.totalTokens), fmtNum(t.inputTokens), fmtNum(t.outputTokens), costStr) + machineCells);
    }
    grandCost += t.totalCost;
    grandInput += t.inputTokens;
    grandOutput += t.outputTokens;
    grandTotal += t.totalTokens;
  }

  const visibleCount = [...toolTotals.values()].filter(t => t.totalTokens > 0).length;
  if (visibleCount > 1) {
    lines.push(dim(divider + machineDiv));
    let totalMachineCells = "";
    if (hasMachines) {
      for (const mc of mcols) {
        totalMachineCells += " | " + boldWhite(fmtCost(machineSums.get(mc.name) ?? 0).padStart(MACHINE_COL_WIDTH));
      }
    }
    lines.push(colorRow(["Total", fmtNum(grandTotal), fmtNum(grandInput), fmtNum(grandOutput), fmtCost(grandCost)], boldWhite) + totalMachineCells);
  }
  if (hasMachines) {
    lines.push("");
    lines.push(dim(renderMachineLegend(mcols)));
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

export type EmitKind = "snapshot" | "history" | "total-history";

export type EmitData =
  | Map<string, UsageTotals>
  | Map<string, UsageEntry[]>
  | { toolName: string; entries: UsageEntry[] };

export interface EmitOptions {
  period: string;
  machineCosts?: Map<string, Map<string, number>>;
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
  const header = ["tool", "tokens", "input", "output", "cost", ...machines.map((m) => `machine_${m}_cost`)];
  const rows: string[] = [csvRow(header)];

  let grandInput = 0;
  let grandOutput = 0;
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
      rows.push(csvRow([name, csvNum(t.totalTokens), csvNum(t.inputTokens), csvNum(t.outputTokens), csvCost(t.totalCost), ...machineCells]));
    }
    grandInput += t.inputTokens;
    grandOutput += t.outputTokens;
    grandTotal += t.totalTokens;
    grandCost += t.totalCost;
  }

  const visibleCount = [...toolTotals.values()].filter((t) => t.totalTokens > 0).length;
  if (visibleCount > 1) {
    const totalMachines = machines.map((m) => csvCost(machineSums.get(m) ?? 0));
    rows.push(csvRow(["Total", csvNum(grandTotal), csvNum(grandInput), csvNum(grandOutput), csvCost(grandCost), ...totalMachines]));
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

function titleForHistory(toolName: string, period: string): string {
  return `${toolName} (${period})`;
}

function titleForTotalHistory(period: string): string {
  return `Combined Cost History (${period})`;
}

function emitMarkdownSnapshot(toolTotals: Map<string, UsageTotals>, opts: EmitOptions): string {
  const machines = collectMachineNames(opts.machineCosts);
  const aligns: Array<"left" | "right"> = ["left", "right", "right", "right", "right", ...machines.map((_) => "right" as const)];
  const header = ["Tool", "Tokens", "Input", "Output", "Cost", ...machines];

  const lines: string[] = [];
  lines.push(`## ${titleForSnapshot(opts.period)}`);
  lines.push("");
  lines.push(mdRow(header));
  lines.push(mdAlignRow(aligns));

  let grandInput = 0;
  let grandOutput = 0;
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
      lines.push(mdRow([name, mdNum(t.totalTokens), mdNum(t.inputTokens), mdNum(t.outputTokens), mdCost(t.totalCost), ...machineCells]));
    }
    grandInput += t.inputTokens;
    grandOutput += t.outputTokens;
    grandTotal += t.totalTokens;
    grandCost += t.totalCost;
  }

  const visibleCount = [...toolTotals.values()].filter((t) => t.totalTokens > 0).length;
  if (visibleCount > 1) {
    const totalMachines = machines.map((m) => `**${mdCost(machineSums.get(m) ?? 0)}**`);
    lines.push(mdRow(["**Total**", `**${mdNum(grandTotal)}**`, `**${mdNum(grandInput)}**`, `**${mdNum(grandOutput)}**`, `**${mdCost(grandCost)}**`, ...totalMachines]));
  }

  lines.push("");
  return lines.join("\n");
}

function emitMarkdownHistory(toolName: string, entries: UsageEntry[], opts: EmitOptions): string {
  const machines = collectMachineNames(opts.machineCosts);
  const aligns: Array<"left" | "right"> = ["left", "right", "right", "right", "right", "right", "right", ...machines.map((_) => "right" as const)];
  const header = ["Date", "Input", "Output", "Cache Write", "Cache Read", "Total", "Cost", ...machines];

  const lines: string[] = [];
  lines.push(`## ${titleForHistory(toolName, opts.period)}`);
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
  const aligns: Array<"left" | "right"> = [
    "left",
    ...toolNames.map((_) => "right" as const),
    "right",
    ...machines.map((_) => "right" as const),
  ];
  const header = ["Date", ...toolNames, "Cost", ...machines];

  const lines: string[] = [];
  lines.push(`## ${titleForTotalHistory(opts.period)}`);
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
  }
  process.stdout.write(output + "\n");
}
