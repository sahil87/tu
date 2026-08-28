import { describe, it, afterEach, mock } from "node:test";
import assert from "node:assert/strict";

import { renderHistory, renderTotal, renderTotalHistory, emitMarkdown } from "../formatter.js";
import { setNoColor, stripAnsi } from "../colors.js";
import type { UsageTotals, UsageEntry } from "../../core/types.js";

// Color assertions in this file need deterministic ANSI state regardless of the
// invoking shell's environment (each test file runs in its own process).
delete process.env.NO_COLOR;
setNoColor(true);
afterEach(() => setNoColor(true));

function entry(label: string, totalCost: number): UsageEntry {
  return { label, totalCost, inputTokens: 100, outputTokens: 50, cacheCreationTokens: 10, cacheReadTokens: 5, totalTokens: 165 };
}

function totals(totalCost: number): UsageTotals {
  return { totalCost, inputTokens: 100, outputTokens: 50, cacheCreationTokens: 10, cacheReadTokens: 5, totalTokens: 165 };
}

const DIM = "\x1b[2m";
const BOLD_WHITE = "\x1b[1;37m";

// Data rows are the lines between the header divider and the Total row's
// divider — the lines that start with a date label.
function dataLines(lines: string[]): string[] {
  return lines.filter((l) => /^\d{4}-\d{2}/.test(stripAnsi(l)));
}

// Column where a row's bar begins: the first block character (any eighths
// precision), or -1 when the row has no bar (zero cost).
function barStartIdx(line: string): number {
  return stripAnsi(line).search(/[█▏▎▍▌▋▊▉]/);
}

// ---------------------------------------------------------------------------
// Data-sized cost columns — pivot (renderTotalHistory)
// ---------------------------------------------------------------------------
describe("renderTotalHistory data-sized Cost column", () => {
  it("keeps every cost decimal point and every bar start at one index across rows and Total", () => {
    const data = new Map<string, UsageEntry[]>([
      ["Claude Code", [entry("2026-06", 0.27), entry("2026-07", 15429.88), entry("2026-08", 90831.65)]],
    ]);
    // Grand total $106,261.80 → 11-wide Cost column (floor 9 exceeded).
    const lines = renderTotalHistory("monthly", data, 200);
    const rows = [...dataLines(lines), lines.find((l) => stripAnsi(l).startsWith("Total"))!];
    const costDotIdx = rows.map((l) => stripAnsi(l).lastIndexOf("."));
    assert.ok(costDotIdx.every((i) => i === costDotIdx[0]), `cost '.' misaligned: ${costDotIdx}`);
    // The bar's first block starts at the same column on every row.
    const barStart = dataLines(lines).map(barStartIdx);
    assert.ok(barStart.every((i) => i > 0 && i === barStart[0]), `bar start misaligned: ${barStart}`);
  });

  it("sizes the Cost column from the grand total even when rows are small", () => {
    const data = new Map<string, UsageEntry[]>([
      ["Claude Code", [entry("2026-06", 30000), entry("2026-07", 30000), entry("2026-08", 30000)]] ,
    ]);
    // Rows are $30,000.00 (10) but the Total is $90,000.00 (10) — and with a
    // $210,191.65-class grand the column must fit the Total cell.
    const lines = renderTotalHistory("monthly", data, 200);
    const totalLine = lines.find((l) => stripAnsi(l).startsWith("Total"))!;
    const rowDot = stripAnsi(dataLines(lines)[0]).lastIndexOf(".");
    assert.equal(stripAnsi(totalLine).lastIndexOf("."), rowDot);
  });

  it("renders byte-identical to the fixed 9-wide layout when every cell fits the floor", () => {
    const data = new Map<string, UsageEntry[]>([
      ["Claude Code", [entry("2026-07-01", 123.45), entry("2026-07-02", 234.56)]],
      ["Codex", [entry("2026-07-01", 12.34), entry("2026-07-02", 23.45)]],
    ]);
    const lines = renderTotalHistory("daily", data, 200).map(stripAnsi);
    const header = lines.find((l) => l.includes("Claude Code"))!;
    // 9-wide floor columns: Date 10, Claude Code 11 (name), Codex 9, Cost 9.
    assert.equal(header, "Date       | Claude Code |     Codex |      Cost");
    const first = stripAnsi(dataLines(lines)[0]);
    // Row through the Cost cell: 10 + 3 + 11 + 3 + 9 + 3 + 9 = 48 chars.
    assert.equal(first.slice(0, first.indexOf("$135.79") + 7).length, 48);
  });

  it("widens a tool column that holds a five-figure cell, keeping Cost and bars aligned", () => {
    const data = new Map<string, UsageEntry[]>([
      ["Claude Code", [entry("2026-06", 100), entry("2026-07", 100)]],
      ["Codex", [entry("2026-06", 10000), entry("2026-07", 100)]],
    ]);
    // Codex total $10,100.00 is 10 chars → its column is 10 wide (floor 9).
    const lines = renderTotalHistory("monthly", data, 200);
    const rows = [...dataLines(lines), lines.find((l) => stripAnsi(l).startsWith("Total"))!];
    const costDotIdx = rows.map((l) => stripAnsi(l).lastIndexOf("."));
    assert.ok(costDotIdx.every((i) => i === costDotIdx[0]), `cost '.' misaligned: ${costDotIdx}`);
    const barStart = dataLines(lines).map(barStartIdx);
    assert.ok(barStart.every((i) => i > 0 && i === barStart[0]), `bar start misaligned: ${barStart}`);
  });

  it("keeps every line within the terminal width at 100 cols with an 11-wide Cost column", () => {
    const data = new Map<string, UsageEntry[]>([
      ["Claude Code", [entry("2026-06", 0.27), entry("2026-07", 15429.88), entry("2026-08", 90831.65)]],
    ]);
    const lines = renderTotalHistory("monthly", data, 100);
    for (const line of lines) {
      const w = stripAnsi(line).length;
      assert.ok(w <= 100, `line exceeds 100 cols (got ${w}): "${stripAnsi(line)}"`);
    }
    // Watch mode: the space-less indicator (+1) still fits, indicatorReserve holds.
    const watch = renderTotalHistory("monthly", data, 100, { prevCosts: new Map([["total:2026-08", 1]]) });
    for (const line of watch) {
      const w = stripAnsi(line).length;
      assert.ok(w <= 100, `watch line exceeds 100 cols (got ${w}): "${stripAnsi(line)}"`);
    }
  });
});

// ---------------------------------------------------------------------------
// Data-sized cost columns — single-tool history (renderHistory)
// ---------------------------------------------------------------------------
describe("renderHistory data-sized Cost and machine columns", () => {
  it("aligns the cost decimal point across rows and Total with five-figure rows", () => {
    const entries = [entry("2026-06", 0.27), entry("2026-07", 15429.88), entry("2026-08", 90831.65)];
    const lines = renderHistory("Claude Code", "monthly", entries, 220);
    const rows = [...dataLines(lines), lines.find((l) => stripAnsi(l).startsWith("Total"))!];
    const costDotIdx = rows.map((l) => stripAnsi(l).lastIndexOf("."));
    assert.ok(costDotIdx.every((i) => i === costDotIdx[0]), `cost '.' misaligned: ${costDotIdx}`);
  });

  it("shares one data-sized width across machine columns", () => {
    const entries = [entry("2026-07-01", 12350.67), entry("2026-07-02", 5)];
    const machineCosts = new Map<string, Map<string, number>>([
      ["2026-07-01", new Map([["m1", 12345.67], ["m2", 5]])],
      ["2026-07-02", new Map([["m1", 0], ["m2", 5]])],
    ]);
    const lines = renderHistory("Claude Code", "daily", entries, 220, { machineCosts });
    // $12,345.67 is 10 chars → both machine columns are 10 wide.
    const header = lines.find((l) => l.includes("Cache Write"))!;
    assert.ok(stripAnsi(header).endsWith("         A |          B"), `machine header: "${stripAnsi(header)}"`);
    const totalLine = lines.find((l) => stripAnsi(l).startsWith("Total"))!;
    assert.ok(stripAnsi(totalLine).includes("$12,345.67"), "Total machine cell renders the five-figure sum");
    // The Total row's machine cells align with the data rows above.
    const rowIdx = stripAnsi(dataLines(lines)[0]).indexOf("$12,345.67");
    assert.equal(stripAnsi(totalLine).indexOf("$12,345.67"), rowIdx);
  });
});

// ---------------------------------------------------------------------------
// Data-sized machine columns — snapshot (renderTotal)
// ---------------------------------------------------------------------------
describe("renderTotal data-sized machine columns", () => {
  it("shares one data-sized width and aligns the Total row's machine cells", () => {
    const toolTotals = new Map<string, UsageTotals>([
      ["Claude Code", totals(12350.67)],
      ["Codex", totals(5)],
    ]);
    const machineCosts = new Map<string, Map<string, number>>([
      ["Claude Code", new Map([["m1", 12345.67], ["m2", 5]])],
      ["Codex", new Map([["m1", 0], ["m2", 5]])],
    ]);
    const lines = renderTotal("monthly", toolTotals, { machineCosts });
    const header = lines.find((l) => l.includes("Tokens"))!;
    assert.ok(stripAnsi(header).endsWith("         A |          B"), `machine header: "${stripAnsi(header)}"`);
    const ccLine = lines.find((l) => stripAnsi(l).startsWith("Claude Code"))!;
    const totalLine = lines.find((l) => stripAnsi(l).startsWith("Total"))!;
    assert.equal(stripAnsi(totalLine).indexOf("$12,345.67"), stripAnsi(ccLine).indexOf("$12,345.67"));
    // The snapshot's own 12-wide Cost cell (fmtCostDelta) is unchanged.
    assert.ok(stripAnsi(ccLine).includes("$12,350.67"), "snapshot Cost cell renders");
  });
});

// ---------------------------------------------------------------------------
// Dim $0.00 data cells
// ---------------------------------------------------------------------------
describe("dim zero data cells", () => {
  it("dims exact-zero pivot cells and row Cost cells; Total row stays boldWhite", () => {
    setNoColor(false);
    const data = new Map<string, UsageEntry[]>([
      ["Claude Code", [entry("2026-07-01", 0), entry("2026-07-02", 5)]],
      ["Codex", [entry("2026-07-01", 0), entry("2026-07-02", 0)]],
    ]);
    const lines = renderTotalHistory("daily", data, 200);
    const zeroRow = dataLines(lines).find((l) => stripAnsi(l).includes("2026-07-01"))!;
    // Codex $0.00 cell (11-wide Claude Code column precedes it) and the $0.00
    // row Cost cell (after the gutter) are both dimmed.
    assert.ok(zeroRow.includes(`${DIM}      $0.00\x1b[0m`), `zero tool cell dimmed: ${JSON.stringify(zeroRow)}`);
    assert.ok(zeroRow.includes(`| ${DIM}    $0.00\x1b[0m`), `zero row Cost cell dimmed: ${JSON.stringify(zeroRow)}`);
    // The Total row is never dimmed — boldWhite throughout.
    const totalLine = lines.find((l) => stripAnsi(l).startsWith("Total"))!;
    assert.ok(!totalLine.includes(DIM), "Total row is never dimmed");
    assert.ok(totalLine.includes(BOLD_WHITE), "Total row stays boldWhite");
    // stripAnsi output is identical to the undimmed render.
    setNoColor(true);
    const plain = renderTotalHistory("daily", data, 200);
    assert.deepEqual(lines.map(stripAnsi), plain.map(stripAnsi));
  });

  it("dims zero Cost and machine cells in renderHistory", () => {
    setNoColor(false);
    const entries = [entry("2026-07-01", 0), entry("2026-07-02", 5)];
    const machineCosts = new Map<string, Map<string, number>>([
      ["2026-07-01", new Map([["m1", 0]])],
      ["2026-07-02", new Map([["m1", 5]])],
    ]);
    const lines = renderHistory("Claude Code", "daily", entries, 220, { machineCosts });
    const zeroRow = dataLines(lines).find((l) => stripAnsi(l).includes("2026-07-01"))!;
    assert.ok(zeroRow.includes(`${DIM}    $0.00\x1b[0m`), `zero cells dimmed: ${JSON.stringify(zeroRow)}`);
    const totalLine = lines.find((l) => stripAnsi(l).startsWith("Total"))!;
    assert.ok(!totalLine.includes(DIM), "Total row is never dimmed");
  });

  it("dims zero machine cells in renderTotal", () => {
    setNoColor(false);
    const toolTotals = new Map<string, UsageTotals>([
      ["Claude Code", totals(5)],
      ["Codex", totals(3)],
    ]);
    const machineCosts = new Map<string, Map<string, number>>([
      ["Claude Code", new Map([["m1", 0]])],
      ["Codex", new Map([["m1", 3]])],
    ]);
    const lines = renderTotal("daily", toolTotals, { machineCosts });
    const ccLine = lines.find((l) => stripAnsi(l).startsWith("Claude Code"))!;
    assert.ok(ccLine.includes(`${DIM}    $0.00\x1b[0m`), `zero machine cell dimmed: ${JSON.stringify(ccLine)}`);
    const totalLine = lines.find((l) => stripAnsi(l).startsWith("Total"))!;
    assert.ok(!totalLine.includes(DIM), "Total row is never dimmed");
  });

  it("does not dim a sub-cent nonzero cost that formats as $0.00", () => {
    setNoColor(false);
    const data = new Map<string, UsageEntry[]>([
      ["Claude Code", [entry("2026-07-01", 0.004), entry("2026-07-02", 5)]],
    ]);
    const lines = renderTotalHistory("daily", data, 200);
    const row = dataLines(lines).find((l) => stripAnsi(l).includes("2026-07-01"))!;
    assert.ok(stripAnsi(row).includes("$0.00"), "sub-cent cost formats as $0.00");
    assert.ok(!row.includes(`${DIM}    $0.00\x1b[0m`), "sub-cent nonzero is not dimmed");
  });

  it("produces escape-free output under setNoColor(true), identical to the stripped colored render", () => {
    // Weekday labels — a weekend date would legitimately dim its date cell
    // under color (pre-existing weekend dimming), which this check is not about.
    // Single visible tool: the footer legend (color-only, omitted under
    // --no-color by design) never renders, isolating the dim-zero change.
    const data = new Map<string, UsageEntry[]>([
      ["Claude Code", [entry("2026-07-01", 0), entry("2026-07-02", 5)]],
    ]);
    setNoColor(false);
    const colored = renderTotalHistory("daily", data, 200).map(stripAnsi);
    setNoColor(true);
    const plain = renderTotalHistory("daily", data, 200);
    assert.deepEqual(plain, colored);
    assert.ok(plain.every((l) => !l.includes("\x1b")), "no escape codes with color disabled");
  });
});

// ---------------------------------------------------------------------------
// Negligible-column omission (ANSI pivot)
// ---------------------------------------------------------------------------
describe("significantTools omission", () => {
  it("omits a tool below $1.00 and keeps one at exactly $1.00", () => {
    const data = new Map<string, UsageEntry[]>([
      ["Claude Code", [entry("2026-07-01", 100), entry("2026-07-02", 100)]],
      ["Gemini", [entry("2026-07-01", 0.99)]],
      ["Kimi", [entry("2026-07-01", 1.0)]],
    ]);
    const lines = renderTotalHistory("daily", data, 200);
    const output = lines.join("\n");
    assert.ok(!output.includes("Gemini"), "$0.99 tool omitted");
    assert.ok(output.includes("Kimi"), "$1.00 boundary tool kept");
  });

  it("omits a tool below 0.1% of the window grand total", () => {
    const data = new Map<string, UsageEntry[]>([
      ["Claude Code", [entry("2026-07-01", 10000), entry("2026-07-02", 10000)]],
      ["Codex", [entry("2026-07-01", 5)]], // $5 vs $20,005 grand = 0.025%
    ]);
    const lines = renderTotalHistory("daily", data, 200);
    const output = lines.join("\n");
    assert.ok(!output.includes("Codex"), "$5 against a $20,005 window is omitted");
  });

  it("keeps the omitted cost in the row Cost, the Total, and the footer", () => {
    const data = new Map<string, UsageEntry[]>([
      ["Claude Code", [entry("2026-07-01", 10000), entry("2026-07-02", 10000)]],
      ["Codex", [entry("2026-07-01", 5)]],
      ["Gemini", [entry("2026-07-01", 0.99)]],
    ]);
    const lines = renderTotalHistory("daily", data, 200).map(stripAnsi);
    const firstRow = dataLines(lines).find((l) => l.includes("2026-07-01"))!;
    assert.ok(firstRow.includes("$10,005.99"), `row Cost includes the omitted tools: "${firstRow}"`);
    const totalLine = lines.find((l) => l.startsWith("Total"))!;
    assert.ok(totalLine.includes("$20,005.99"), `grand Total includes the omitted tools: "${totalLine}"`);
    const footer = lines.find((l) => l.startsWith("avg "))!;
    assert.ok(footer.includes("avg $10,002.99/day"), `footer avg includes the omitted tools: "${footer}"`);
    assert.ok(footer.includes("peak $10,005.99 (2026-07-01)"), `footer peak includes the omitted tools: "${footer}"`);
  });

  it("keeps the stacked bar equal to the unstacked bar for the full row total", () => {
    setNoColor(false);
    const data = new Map<string, UsageEntry[]>([
      ["Claude Code", [entry("2026-07-01", 100), entry("2026-07-02", 50)]],
      ["Codex", [entry("2026-07-01", 50), entry("2026-07-02", 50)]],
      ["Gemini", [entry("2026-07-01", 0.04)]], // omitted — its share is absorbed
    ]);
    const lines = renderTotalHistory("daily", data, 200);
    const row = stripAnsi(dataLines(lines).find((l) => stripAnsi(l).includes("2026-07-01"))!);
    // The bar for the $150.04 row renders at full row-total length even though
    // Gemini's column is gone (apportionSegments normalises over visible shares).
    const bar = row.match(/[█▏▎▍▌▋▊▉]+/);
    assert.ok(bar, `stacked bar renders: "${row}"`);
    // Max row ($150.04) fills the whole 30-char bar area.
    assert.equal(bar![0].length, 30);
  });

  it("renders the legend from the filtered column set only", () => {
    setNoColor(false);
    const data = new Map<string, UsageEntry[]>([
      ["Claude Code", [entry("2026-07-01", 10000), entry("2026-07-02", 10000)]],
      ["Kimi", [entry("2026-07-01", 100), entry("2026-07-02", 100)]],
      ["Codex", [entry("2026-07-01", 5)]],
    ]);
    const lines = renderTotalHistory("daily", data, 200);
    const footer = lines.find((l) => stripAnsi(l).startsWith("avg "))!;
    assert.ok(stripAnsi(footer).includes("█ Claude Code █ Kimi"), `legend follows the filtered set: "${stripAnsi(footer)}"`);
    assert.ok(!stripAnsi(footer).includes("Codex"), "omitted tool has no legend swatch");
  });

  it("falls back to nonzero tools when the whole window is below $1.00", () => {
    const data = new Map<string, UsageEntry[]>([
      ["Claude Code", [entry("2026-07-01", 0.30), entry("2026-07-02", 0.05)]],
      ["Codex", [entry("2026-07-01", 0.05)]],
      ["Gemini", [entry("2026-07-01", 0)]],
    ]);
    const lines = renderTotalHistory("daily", data, 200);
    const output = lines.join("\n");
    assert.ok(output.includes("Claude Code"), "first fallback keeps the nonzero tool");
    assert.ok(output.includes("Codex"), "first fallback keeps every nonzero tool");
    assert.ok(!output.includes("Gemini"), "exact-zero tool still omitted under the fallback");
  });

  it("falls back to the full registry when every tool is zero", () => {
    const data = new Map<string, UsageEntry[]>([
      ["Claude Code", [entry("2026-07-01", 0)]],
      ["Codex", [entry("2026-07-01", 0)]],
    ]);
    const output = renderTotalHistory("daily", data, 200).join("\n");
    assert.ok(output.includes("Claude Code"), "second fallback keeps Claude Code");
    assert.ok(output.includes("Codex"), "second fallback keeps Codex");
  });

  it("keeps the Markdown pivot on the exact-zero rule", () => {
    const data = new Map<string, UsageEntry[]>([
      ["Claude Code", [entry("2026-07-01", 10000), entry("2026-07-02", 10000)]],
      ["Codex", [entry("2026-07-01", 5)]], // negligible for ANSI, nonzero for Markdown
      ["Gemini", [entry("2026-07-01", 0)]],
    ]);
    let output = "";
    mock.method(process.stdout, "write", (s: string) => { output += String(s); return true; });
    try {
      emitMarkdown(data, "total-history", { period: "daily" });
    } finally {
      mock.restoreAll();
    }
    assert.ok(output.includes("| Codex "), "Markdown keeps the negligible-but-nonzero column");
    assert.ok(!output.includes("Gemini"), "Markdown still omits the exact-zero column");
  });
});

// ---------------------------------------------------------------------------
// Token mode: column width, zero-dim, no-color byte-identity
// ---------------------------------------------------------------------------
describe("token-mode metric columns", () => {
  const tok = (label: string, totalCost: number, totalTokens: number): UsageEntry => ({
    label, totalCost, inputTokens: totalTokens, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens,
  });

  it("widens the pivot Tokens column past the 9 floor with aligned bar starts", () => {
    const data = new Map<string, UsageEntry[]>([
      ["Claude Code", [tok("2026-06", 0.27, 9_999_999), tok("2026-07", 1.5, 487_683_047)]],
    ]);
    const lines = renderTotalHistory("monthly", data, 200, { metric: "tokens" });
    const plain = lines.map(stripAnsi);
    const header = plain.find((l) => l.includes("Tokens"))!;
    // Row total 497,683,046 (11 chars) sizes the column to 11.
    assert.ok(header.endsWith("    Tokens"), header);
    const total = plain.find((l) => l.startsWith("Total"))!;
    assert.ok(total.includes("497,683,046"), total);
    // The bar's first block starts at the same column on every row.
    const barStart = dataLines(lines).map(barStartIdx);
    assert.ok(barStart.every((i) => i > 0 && i === barStart[0]), `bar start misaligned: ${barStart}`);
  });

  it("dims exact-zero token cells; --no-color output is byte-identical to undimmed", () => {
    const data = new Map<string, UsageEntry[]>([
      ["Claude Code", [tok("2026-06-01", 5, 5000), tok("2026-06-02", 0, 0)]],
    ]);
    setNoColor(false);
    const colored = renderTotalHistory("daily", data, 200, { metric: "tokens" });
    setNoColor(true);
    const plain = renderTotalHistory("daily", data, 200, { metric: "tokens" });
    const zeroRow = colored.find((l) => stripAnsi(l).startsWith("2026-06-02"))!;
    assert.ok(zeroRow.includes(`${DIM}          0\x1b[0m`), `zero token cell dimmed: ${JSON.stringify(zeroRow)}`);
    // Strip-and-compare: dimming never shifts the visible layout.
    assert.deepEqual(colored.map(stripAnsi), plain);
    // The history table's last column dims the same way in token mode.
    setNoColor(false);
    const hist = renderHistory("Claude Code", "daily", [tok("2026-06-01", 0, 0), tok("2026-06-02", 5, 5000)], 200, { metric: "tokens" });
    setNoColor(true);
    const histZero = hist.find((l) => stripAnsi(l).startsWith("2026-06-01"))!;
    assert.ok(histZero.includes(`${DIM}        0\x1b[0m`), `history zero cell dimmed: ${JSON.stringify(histZero)}`);
    assert.deepEqual(hist.map(stripAnsi), renderHistory("Claude Code", "daily", [tok("2026-06-01", 0, 0), tok("2026-06-02", 5, 5000)], 200, { metric: "tokens" }));
  });

  it("renders the unstacked bar of the row token total under tokens", () => {
    const data = new Map<string, UsageEntry[]>([
      ["Claude Code", [tok("2026-06-01", 1, 3000)]],
      ["Codex", [tok("2026-06-01", 1, 6000)]],
    ]);
    const lines = renderTotalHistory("daily", data, 200, { metric: "tokens" });
    const row = stripAnsi(dataLines(lines)[0]);
    const bar = row.match(/[█▏▎▍▌▋▊▉]+/)!;
    assert.ok(bar, `stacked bar renders: "${row}"`);
    // 9,000 of a 9,000 row total fills the whole 30-char bar area.
    assert.equal(bar[0].length, 30);
  });
});
