import { describe, it, beforeEach, afterEach, mock } from "node:test";
import assert from "node:assert/strict";

import { fmtNum, fmtCost, renderBar, printHistory, printTotal, printTotalHistory, renderHistory, renderTotal, renderTotalHistory, emitCsv, emitMarkdown } from "../formatter.js";
import { setNoColor, stripAnsi } from "../colors.js";
import type { UsageTotals, UsageEntry } from "../../core/types.js";

// Disable ANSI colors for formatter tests to keep assertions simple
setNoColor(true);

// ---------------------------------------------------------------------------
// Helpers: capture console.log output
// ---------------------------------------------------------------------------
let logged: string[];

function captureLog() {
  logged = [];
  mock.method(console, "log", (...args: unknown[]) => {
    logged.push(args.map(String).join(" "));
  });
}

function restoreLog() {
  mock.restoreAll();
}

// ---------------------------------------------------------------------------
// fmtNum
// ---------------------------------------------------------------------------
describe("fmtNum", () => {
  it("formats integers with thousands separators", () => {
    assert.equal(fmtNum(1000), "1,000");
    assert.equal(fmtNum(1234567), "1,234,567");
  });

  it("formats zero", () => {
    assert.equal(fmtNum(0), "0");
  });

  it("formats small numbers without separator", () => {
    assert.equal(fmtNum(42), "42");
  });
});

// ---------------------------------------------------------------------------
// fmtCost
// ---------------------------------------------------------------------------
describe("fmtCost", () => {
  it("formats with dollar sign and 2 decimal places", () => {
    assert.equal(fmtCost(1.5), "$1.50");
    assert.equal(fmtCost(0), "$0.00");
    assert.equal(fmtCost(12.345), "$12.35");
  });

  it("formats thousands with comma separators", () => {
    assert.equal(fmtCost(4031.61), "$4,031.61");
    assert.equal(fmtCost(1000), "$1,000.00");
    assert.equal(fmtCost(999999.99), "$999,999.99");
  });
});

// ---------------------------------------------------------------------------
// renderBar
// ---------------------------------------------------------------------------
describe("renderBar", () => {
  it("returns empty string for zero value", () => {
    assert.equal(renderBar(0, 100, 20), "");
  });

  it("returns empty string when maxValue is zero (all-zero)", () => {
    assert.equal(renderBar(5, 0, 20), "");
  });

  it("returns full blocks for max value", () => {
    const bar = renderBar(100, 100, 20);
    assert.equal(bar, "\u2588".repeat(20));
  });

  it("scales proportionally — half value gets half width", () => {
    const bar = renderBar(50, 100, 20);
    assert.equal(bar, "\u2588".repeat(10));
  });

  it("renders fractional block for non-integer widths", () => {
    // value=50, max=100, barWidth=15 → scaled=7.5 → 7 full + 4/8 (▌)
    const bar = renderBar(50, 100, 15);
    assert.equal(bar, "\u2588".repeat(7) + "\u258C"); // 7 full + ▌ (4/8)
  });

  it("renders minimum bar ▏ for near-zero non-zero value", () => {
    const bar = renderBar(0.01, 100, 20);
    assert.equal(bar, "\u258F");
  });

  it("handles eighths=8 rounding by adding extra full block", () => {
    // Craft a case where fractional part × 8 rounds to 8
    // value=99, max=100, barWidth=20 → scaled=19.8 → floor=19, frac=0.8, 0.8*8=6.4 → round=6 (▊)
    // Let's use value=997, max=1000, barWidth=10 → scaled=9.97 → floor=9, frac=0.97, 0.97*8=7.76 → round=8
    const bar = renderBar(997, 1000, 10);
    assert.equal(bar, "\u2588".repeat(10));
  });

  it("renders 7/8 fractional block (▉)", () => {
    // value=3, max=8, barWidth=5 → scaled=1.875 → 1 full + round(0.875*8)=7 → ▉
    const bar = renderBar(3, 8, 5);
    assert.equal(bar, "\u2588" + "\u2589"); // 1 full + 7/8 (▉)
  });

  it("renders 5/8 fractional block (▋) per spec scenario", () => {
    // scaled = 29/160 * 20 = 3.625 → 3 full + round(0.625*8)=5 → ▋
    const bar = renderBar(29, 160, 20);
    assert.equal(bar, "\u2588".repeat(3) + "\u258B"); // ███▋
  });
});

// ---------------------------------------------------------------------------
// printHistory
// ---------------------------------------------------------------------------
describe("printHistory", () => {
  beforeEach(() => captureLog());

  it("prints 'No data' for empty entries", (t) => {
    t.after(restoreLog);
    printHistory("Claude Code", "daily", []);
    const output = logged.join("\n");
    assert.match(output, /No data/);
  });

  it("prints header row and single entry with inline bar and pipe separators", (t) => {
    t.after(restoreLog);
    const entries: UsageEntry[] = [{
      label: "2026-02-14",
      totalCost: 1.50,
      inputTokens: 100,
      outputTokens: 200,
      cacheCreationTokens: 10,
      cacheReadTokens: 20,
      totalTokens: 330,
    }];
    printHistory("Claude Code", "daily", entries, 150);
    const output = logged.join("\n");
    assert.match(output, /Claude Code.*daily/);
    assert.match(output, /Date/);
    assert.match(output, /Input/);
    assert.match(output, /2026-02-14/);
    assert.match(output, /\$1\.50/);
    // Pipe separators in header and data rows
    assert.ok(output.includes(" | "), "expected pipe separators between columns");
    // Pipe in divider rows
    assert.ok(output.includes("─|─"), "expected pipe in divider rows");
    // Inline bar present (single entry = max, gets full blocks)
    assert.ok(output.includes("\u2588"), "expected inline bar with █ blocks");
  });

  it("does not print totals row for single entry", (t) => {
    t.after(restoreLog);
    const entries: UsageEntry[] = [{
      label: "2026-02-14",
      totalCost: 1.50,
      inputTokens: 100,
      outputTokens: 200,
      cacheCreationTokens: 0,
      cacheReadTokens: 0,
      totalTokens: 300,
    }];
    printHistory("Test", "daily", entries, 140);
    const lines = logged.filter(l => l.startsWith("Total"));
    assert.equal(lines.length, 0);
  });

  it("prints totals row for multiple entries — totals row has no bar", (t) => {
    t.after(restoreLog);
    const entries: UsageEntry[] = [
      { label: "2026-02-13", totalCost: 1, inputTokens: 100, outputTokens: 50, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 150 },
      { label: "2026-02-14", totalCost: 2, inputTokens: 200, outputTokens: 100, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 300 },
    ];
    printHistory("Test", "daily", entries, 140);
    const output = logged.join("\n");
    assert.match(output, /Total/);
    assert.match(output, /\$3\.00/);
    // Totals row should not have blocks
    const totalLine = logged.find(l => l.startsWith("Total"));
    assert.ok(totalLine, "expected a Total line");
    assert.ok(!totalLine!.includes("\u2588"), "totals row should not have bar blocks");
  });

  it("omits bar column on narrow terminals", (t) => {
    t.after(restoreLog);
    // tableWidth = 97, GUTTER = 3, costWidth = 9 (floor), separator = 1
    // termWidth = 110 → barWidth = 110 - 97 - 3 - 9 - 1 = 0 < MIN_BAR_AREA
    const entries: UsageEntry[] = [{
      label: "2026-02-14",
      totalCost: 5,
      inputTokens: 100,
      outputTokens: 200,
      cacheCreationTokens: 0,
      cacheReadTokens: 0,
      totalTokens: 300,
    }];
    printHistory("Test", "daily", entries, 110);
    const output = logged.join("\n");
    assert.ok(!output.includes("\u2588"), "expected no bars on narrow terminal");
    // Cost value should still render in merged area
    assert.ok(output.includes("$5.00"), "cost should still render when bars hidden");
  });

  it("does not render a separate bar chart section", (t) => {
    t.after(restoreLog);
    const entries: UsageEntry[] = [{
      label: "2026-02-14",
      totalCost: 1.50,
      inputTokens: 100,
      outputTokens: 200,
      cacheCreationTokens: 0,
      cacheReadTokens: 0,
      totalTokens: 300,
    }];
    printHistory("Test", "daily", entries, 140);
    // Should NOT have standalone "Cost" header (the old printBarChart format)
    const costHeaders = logged.filter(l => l.trim() === "Cost");
    assert.equal(costHeaders.length, 0, "should not have standalone Cost header from bar chart");
  });

  it("caps bar width at MAX_BAR_WIDTH on ultra-wide terminals", (t) => {
    t.after(restoreLog);
    const entries: UsageEntry[] = [{
      label: "2026-02-14",
      totalCost: 10,
      inputTokens: 100,
      outputTokens: 200,
      cacheCreationTokens: 0,
      cacheReadTokens: 0,
      totalTokens: 300,
    }];
    // termWidth=188, tableWidth=97, GUTTER=3, costWidth=9 (floor), sep=1 → uncapped=78, capped at 30
    printHistory("Test", "daily", entries, 188);
    const dataLine = logged.find(l => l.includes("2026-02-14"));
    assert.ok(dataLine);
    const blocks = (dataLine!.match(/\u2588/g) || []).length;
    assert.equal(blocks, 30, "expected bar capped at MAX_BAR_WIDTH=30");
  });
});

// ---------------------------------------------------------------------------
// printTotal
// ---------------------------------------------------------------------------
describe("printTotal", () => {
  beforeEach(() => captureLog());

  it("prints combined usage table", (t) => {
    t.after(restoreLog);
    const totals = new Map<string, UsageTotals>([
      ["Claude Code", { totalCost: 5, inputTokens: 1000, outputTokens: 500, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 1500 }],
      ["Codex", { totalCost: 3, inputTokens: 800, outputTokens: 200, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 1000 }],
    ]);
    printTotal("daily", totals);
    const output = logged.join("\n");
    assert.match(output, /Combined Usage.*daily/);
    assert.match(output, /Claude Code/);
    assert.match(output, /Codex/);
    assert.match(output, /\$8\.00/); // grand total
  });

  it("prints 'No usage' when all tools have zero tokens", (t) => {
    t.after(restoreLog);
    const totals = new Map<string, UsageTotals>([
      ["Claude Code", { totalCost: 0, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0 }],
      ["Codex", { totalCost: 0, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0 }],
    ]);
    printTotal("daily", totals);
    const output = logged.join("\n");
    assert.match(output, /Combined Usage.*daily/);
    assert.match(output, /No usage/);
    assert.ok(!output.includes("──────"), "should not render table dividers");
  });

  it("prints 'No usage' for empty toolTotals map", (t) => {
    t.after(restoreLog);
    const totals = new Map<string, UsageTotals>();
    printTotal("daily", totals);
    const output = logged.join("\n");
    assert.match(output, /Combined Usage.*daily/);
    assert.match(output, /No usage/);
    assert.ok(!output.includes("──────"), "should not render table dividers");
  });

  it("prints 'No usage' for single tool with zero tokens", (t) => {
    t.after(restoreLog);
    const totals = new Map<string, UsageTotals>([
      ["Claude Code", { totalCost: 0, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0 }],
    ]);
    printTotal("daily", totals);
    const output = logged.join("\n");
    assert.match(output, /No usage/);
  });

  it("omits Total row when only one tool has data", (t) => {
    t.after(restoreLog);
    const totals = new Map<string, UsageTotals>([
      ["Claude Code", { totalCost: 5, inputTokens: 1000, outputTokens: 500, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 1500 }],
    ]);
    printTotal("daily", totals);
    const totalLines = logged.filter(l => l.includes("Total") && !l.includes("Combined"));
    assert.equal(totalLines.length, 0, "should not render Total row with single tool");
  });

  it("shows Total row when multiple tools have data", (t) => {
    t.after(restoreLog);
    const totals = new Map<string, UsageTotals>([
      ["Claude Code", { totalCost: 5, inputTokens: 1000, outputTokens: 500, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 1500 }],
      ["Codex", { totalCost: 3, inputTokens: 800, outputTokens: 200, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 1000 }],
    ]);
    printTotal("daily", totals);
    const totalLines = logged.filter(l => l.includes("Total") && !l.includes("Combined"));
    assert.equal(totalLines.length, 1, "should render Total row with multiple tools");
  });

  it("omits Total row in compact mode when only one tool has data", (t) => {
    t.after(restoreLog);
    const totals = new Map<string, UsageTotals>([
      ["Claude Code", { totalCost: 5, inputTokens: 1000, outputTokens: 500, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 1500 }],
    ]);
    printTotal("daily", totals, { compact: true });
    const totalLines = logged.filter(l => l.includes("Total"));
    assert.equal(totalLines.length, 0, "should not render Total row in compact with single tool");
  });

  it("shows Total row in compact mode when multiple tools have data", (t) => {
    t.after(restoreLog);
    const totals = new Map<string, UsageTotals>([
      ["Claude Code", { totalCost: 5, inputTokens: 1000, outputTokens: 500, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 1500 }],
      ["Codex", { totalCost: 3, inputTokens: 800, outputTokens: 200, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 1000 }],
    ]);
    printTotal("daily", totals, { compact: true });
    const totalLines = logged.filter(l => l.includes("Total"));
    assert.equal(totalLines.length, 1, "should render Total row in compact with multiple tools");
  });

  it("skips tools with zero tokens but still includes in grand total", (t) => {
    t.after(restoreLog);
    const totals = new Map<string, UsageTotals>([
      ["Claude Code", { totalCost: 5, inputTokens: 1000, outputTokens: 500, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 1500 }],
      ["Empty", { totalCost: 0, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0 }],
    ]);
    printTotal("daily", totals);
    const lines = logged.filter(l => l.includes("Empty"));
    assert.equal(lines.length, 0);
  });
});

// ---------------------------------------------------------------------------
// printTotalHistory
// ---------------------------------------------------------------------------
describe("printTotalHistory", () => {
  beforeEach(() => captureLog());

  it("prints 'No data' when all tools have empty entries", (t) => {
    t.after(restoreLog);
    const data = new Map<string, UsageEntry[]>([
      ["Claude Code", []],
      ["Codex", []],
    ]);
    printTotalHistory("daily", data);
    const output = logged.join("\n");
    assert.match(output, /No data/);
  });

  it("prints pivot table with row totals and inline bars", (t) => {
    t.after(restoreLog);
    const data = new Map<string, UsageEntry[]>([
      ["Claude Code", [
        { label: "2026-02-13", totalCost: 1, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0 },
        { label: "2026-02-14", totalCost: 2, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0 },
      ]],
      ["Codex", [
        { label: "2026-02-13", totalCost: 0.5, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0 },
        { label: "2026-02-14", totalCost: 1.5, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0 },
      ]],
    ]);
    printTotalHistory("daily", data, 140);
    const output = logged.join("\n");

    // Headers: data columns + merged area header
    assert.match(output, /Claude Code/);
    assert.match(output, /Codex/);
    assert.match(output, /Cost/);

    // Date rows present
    assert.match(output, /2026-02-13/);
    assert.match(output, /2026-02-14/);

    // Grand total: 1 + 2 + 0.5 + 1.5 = 5.00
    assert.match(output, /\$5\.00/);

    // Inline bars present
    assert.ok(output.includes("\u2588"), "expected inline bar with █ blocks");

    // No standalone bar chart section
    const costHeaders = logged.filter(l => l.trim() === "Cost");
    assert.equal(costHeaders.length, 0, "should not have standalone Cost header");
  });

  it("totals row has no bar", (t) => {
    t.after(restoreLog);
    const data = new Map<string, UsageEntry[]>([
      ["Tool", [
        { label: "2026-02-13", totalCost: 1, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0 },
        { label: "2026-02-14", totalCost: 2, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0 },
      ]],
    ]);
    printTotalHistory("daily", data, 140);
    const totalLine = logged.find(l => l.startsWith("Total"));
    assert.ok(totalLine, "expected a Total line");
    assert.ok(!totalLine!.includes("\u2588"), "totals row should not have bar blocks");
  });

  it("sorts labels chronologically", (t) => {
    t.after(restoreLog);
    const data = new Map<string, UsageEntry[]>([
      ["Tool", [
        { label: "2026-02-14", totalCost: 2, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0 },
        { label: "2026-02-12", totalCost: 1, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0 },
      ]],
    ]);
    printTotalHistory("daily", data, 140);

    const dateLines = logged.filter(l => l.match(/2026-02-\d{2}/));
    assert.ok(dateLines.length >= 2);
    const firstDateIdx = logged.indexOf(dateLines[0]);
    const secondDateIdx = logged.indexOf(dateLines[1]);
    assert.ok(firstDateIdx < secondDateIdx);
    assert.match(dateLines[0], /2026-02-12/);
    assert.match(dateLines[1], /2026-02-14/);
  });

  it("does not print totals row for single date", (t) => {
    t.after(restoreLog);
    const data = new Map<string, UsageEntry[]>([
      ["Tool", [
        { label: "2026-02-14", totalCost: 2, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0 },
      ]],
    ]);
    printTotalHistory("daily", data, 140);
    const totalLines = logged.filter(l => l.startsWith("Total"));
    assert.equal(totalLines.length, 0);
  });

  it("omits bars on narrow terminals", (t) => {
    t.after(restoreLog);
    const data = new Map<string, UsageEntry[]>([
      ["Claude Code", [
        { label: "2026-02-14", totalCost: 5, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0 },
      ]],
      ["Codex", [
        { label: "2026-02-14", totalCost: 3, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0 },
      ]],
    ]);
    // Variable-width columns: Claude Code → max(11,9)=11, Codex → max(5,9)=9.
    // Date column is 10. tableWidth = 10 + (11+3) + (9+3) = 36.
    // termWidth=50 → barWidth = 50 - 36 - 3 - 9 - 1 = 1 < MIN_BAR_AREA
    printTotalHistory("daily", data, 50);
    const output = logged.join("\n");
    assert.ok(!output.includes("\u2588"), "expected no bars on narrow terminal");
  });

  it("adapts bar width to tool count", (t) => {
    t.after(restoreLog);
    const data = new Map<string, UsageEntry[]>([
      ["A", [{ label: "2026-02-14", totalCost: 10, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0 }]],
      ["B", [{ label: "2026-02-14", totalCost: 5, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0 }]],
      ["C", [{ label: "2026-02-14", totalCost: 3, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0 }]],
    ]);
    // Variable-width columns: A/B/C each → max(1,9)=9. Date column is 10.
    // tableWidth = 10 + 3*(9+3) = 46.
    // termWidth=140 → barWidth = min(140-46-3-9-1, 30) = min(81, 30) = 30
    printTotalHistory("daily", data, 140);
    const dataLine = logged.find(l => l.includes("2026-02-14"));
    assert.ok(dataLine);
    // Max value row ($18 total) should have bars
    assert.ok(dataLine!.includes("\u2588"), "expected bars in data row");
  });

  it("6-tool pivot full data row (through the Cost cell) is 96 chars and fits within 97 columns", (t) => {
    t.after(restoreLog);
    const mk = (cost: number): UsageEntry[] => [
      { label: "2026-07-01", totalCost: cost, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0 },
    ];
    // Full 6-tool registry order, all with significant cost (negligible-cost
    // columns are omitted — that case is covered by the omission tests below).
    // Every cell is ≥ $1.00 and ≥ 0.1% of the row total, and ≤ $9,999.99, so
    // all six columns render at the 9-char floor and the row stays 96 chars.
    const data = new Map<string, UsageEntry[]>([
      ["Claude Code", mk(123.45)],
      ["Codex", mk(12.34)],
      ["OpenCode", mk(4.56)],
      ["Gemini", mk(1.23)],
      ["Copilot", mk(2.34)],
      ["Kimi", mk(3.45)],
    ]);
    printTotalHistory("daily", data, 97);
    const output = logged.join("\n");

    // All six tool columns render (all have nonzero cost in the window).
    for (const name of ["Claude Code", "Codex", "OpenCode", "Gemini", "Copilot", "Kimi"]) {
      assert.match(output, new RegExp(name));
    }

    // The FULL rendered data row \u2014 Date + the six tool columns + the 3-char
    // gutter + the 9-wide Cost cell \u2014 must fit within 97 cols. Per-column
    // widths: Date 10, Claude Code max(11,9)=11, the other five max(<=9,9)=9. So
    // the row is 10 + (11+9+9+9+9+9) + 6x3 (the " | " before each tool column) + 3
    // (gutter) + 9 (Cost) = 96.
    // Measure the ACTUAL rendered data row through the Cost cell \u2014 not the
    // body, not recomputed arithmetic against a constant.
    const dataLine = logged.find((l) => stripAnsi(l).includes("2026-07-01"));
    assert.ok(dataLine, "expected the 2026-07-01 data row");
    const stripped = stripAnsi(dataLine!);
    // Slice through the end of the Cost cell (the row's grand total, $147.37).
    const costCell = "$147.37";
    const costEnd = stripped.indexOf(costCell) + costCell.length;
    const fullRow = stripped.slice(0, costEnd);
    assert.equal(fullRow.length, 96, `full data row must be 96 chars (got ${fullRow.length}): "${fullRow}"`);
    assert.ok(fullRow.length <= 97, `full data row exceeds 97 cols (${fullRow.length})`);
    // And no inline bar renders at 97 cols (barWidth = 97 - 84 - 3 - 9 - 1 = 0 < MIN_BAR_AREA).
    assert.ok(!output.includes("\u2588"), "expected no inline bars for the 6-tool pivot at 97 cols");

    // Watch mode: with prevCosts set (watch.ts populates it after the first
    // poll) the row appends a delta indicator after the Cost cell. In this pivot
    // it is rendered WITHOUT its leading space (the arrow abuts the cost \u2014
    // "$147.37\u2191", 1 visible char) so the row is 96 + 1 = 97 exactly and
    // does NOT wrap at 97 cols. The spaced form ( \u2191, 2 chars) would render
    // 98 and wrap, corrupting the watch compositor's line-counting.
    captureLog();
    // Row total for 2026-07-01 is 123.45 + 12.34 + 4.56 + 1.23 + 2.34 + 3.45 =
    // 147.37; a lower prev triggers the up-arrow (\u2191).
    printTotalHistory("daily", data, 97, { prevCosts: new Map([["total:2026-07-01", 100]]) });
    const watchOutput = logged.join("\n");
    const watchLine = logged.find((l) => stripAnsi(l).includes("2026-07-01"));
    assert.ok(watchLine, "expected the 2026-07-01 watch-mode data row");
    const watchStripped = stripAnsi(watchLine!);
    // Sanity: the indicator rendered space-lessly, directly against the cost.
    assert.ok(watchStripped.includes("$147.37\u2191"), `expected space-less indicator "$147.37\u2191", got: "${watchStripped}"`);
    // Measure the full row through the indicator (the arrow is the last glyph).
    const arrowEnd = watchStripped.indexOf("\u2191") + 1;
    const watchRow = watchStripped.slice(0, arrowEnd);
    assert.equal(watchRow.length, 97, `watch-mode row (through the delta indicator) must be 97 chars (got ${watchRow.length}): "${watchRow}"`);
    assert.ok(watchRow.length <= 97, `watch-mode row exceeds 97 cols (${watchRow.length})`);
    // Still no inline bar at 97 cols in watch mode.
    assert.ok(!watchOutput.includes("\u2588"), "expected no inline bars for the 6-tool watch pivot at 97 cols");
  });

  it("6-tool watch-mode pivot: no line exceeds terminal width across the bars band (107/110/120)", (t) => {
    t.after(restoreLog);
    const mk = (cost: number): UsageEntry[] => [
      { label: "2026-07-01", totalCost: cost, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0 },
    ];
    // Full 6-tool registry order, all significant (≥ $1.00 and ≥ 0.1% of the
    // row total, ≤ $9,999.99 — negligible-cost columns are omitted); the
    // max-cost row is the one that renders the longest bar (full width) — it
    // is the wrap risk once a delta indicator is appended. tableWidth = 84,
    // so bars render from ~width 107 (barWidth >= 10).
    const data = new Map<string, UsageEntry[]>([
      ["Claude Code", mk(123.45)],
      ["Codex", mk(12.34)],
      ["OpenCode", mk(4.56)],
      ["Gemini", mk(1.23)],
      ["Copilot", mk(2.34)],
      ["Kimi", mk(3.45)],
    ]);
    // A lower prev triggers the up-arrow (row total 147.37 > 100), exercising the
    // watch-mode delta indicator on the max-cost row.
    const prevCosts = new Map([["total:2026-07-01", 100]]);
    for (const termWidth of [107, 110, 120]) {
      captureLog();
      printTotalHistory("daily", data, termWidth, { prevCosts });
      // Reserving the indicator char shifts the bars threshold up by one: with
      // tableWidth 84, barWidth = width - 84 - 3 - 9 - 1 - 1 (indicator reserve),
      // so bars render from width 108 (>=10). 107 is the last no-bar width; 110/120
      // render bars and exercise the bar + indicator interaction on the max-cost
      // row (the historical width+1 wrap).
      if (termWidth >= 108) {
        assert.ok(logged.join("\n").includes("█"), `expected inline bars at ${termWidth} cols`);
      }
      // EVERY rendered line — headers, dividers, data rows (bar + indicator),
      // totals — must measure <= termWidth once ANSI is stripped, or it wraps and
      // corrupts the watch-mode compositor's line-counting.
      for (const line of logged) {
        const w = stripAnsi(line).length;
        assert.ok(w <= termWidth, `line exceeds ${termWidth} cols (got ${w}): "${stripAnsi(line)}"`);
      }
    }
  });

  it("tool columns are sized to max(name.length, 9)", (t) => {
    t.after(restoreLog);
    const data = new Map<string, UsageEntry[]>([
      // "Claude Code" (11) exceeds the 9 floor; "AB" (2) is padded up to 9.
      ["Claude Code", [{ label: "2026-07-01", totalCost: 1, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0 }]],
      ["AB", [{ label: "2026-07-01", totalCost: 2, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0 }]],
    ]);
    printTotalHistory("daily", data, 140);
    const header = logged.find(l => l.includes("Claude Code"));
    assert.ok(header);
    const stripped = stripAnsi(header!);
    // Columns joined by " | ": Date padEnd(10) = "Date" + 6 spaces, then the
    // separator's leading space → 7 spaces before the pipe; "Claude Code"
    // padStart(11) (fits exactly, so no pad); "AB" padStart(9) — padStart
    // right-aligns, so 7 leading pad spaces + the separator space = 8 before "AB".
    assert.match(stripped, /^Date {7}\| Claude Code \| {8}AB \|/);
  });

  it("omits tool columns with zero cost across the visible window", (t) => {
    t.after(restoreLog);
    const e = (label: string, totalCost: number): UsageEntry => ({
      label, totalCost, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0,
    });
    const data = new Map<string, UsageEntry[]>([
      ["Claude Code", [e("2026-02-13", 1), e("2026-02-14", 2)]],
      ["Codex", [e("2026-02-13", 0)]],
      ["Kimi", []],
    ]);
    printTotalHistory("daily", data, 140);
    const output = logged.join("\n");

    // Header is Date | Claude Code | Cost — the zero-cost columns are gone.
    const header = logged.find((l) => l.includes("Claude Code"));
    assert.ok(header, "expected a header line");
    assert.equal(stripAnsi(header!), "Date       | Claude Code |      Cost");
    assert.ok(!output.includes("Codex"), "zero-cost Codex column omitted");
    assert.ok(!output.includes("Kimi"), "no-entry Kimi column omitted");
    // No $0.00-only cells anywhere (rows or Total).
    assert.ok(!output.includes("$0.00"), "no $0.00 cells from omitted columns");
    // Total row renders against the same filtered column set.
    const totalLine = logged.find((l) => l.startsWith("Total"));
    assert.ok(totalLine, "expected a Total line");
    assert.equal(stripAnsi(totalLine!), "Total      |       $3.00 |     $3.00");
  });

  it("keeps the Date | Tool | Cost pivot shape when one tool remains", (t) => {
    t.after(restoreLog);
    const e = (label: string, totalCost: number): UsageEntry => ({
      label, totalCost, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0,
    });
    const data = new Map<string, UsageEntry[]>([
      ["Claude Code", [e("2026-02-14", 5)]],
      ["Codex", [e("2026-02-14", 0)]],
    ]);
    printTotalHistory("daily", data, 140);
    const output = logged.join("\n");
    const header = logged.find((l) => l.includes("Claude Code"));
    assert.ok(header, "expected a header line");
    // Pivot shape preserved — no collapse to the single-tool history layout
    // (which would carry Input/Output/Cache token columns).
    assert.equal(stripAnsi(header!), "Date       | Claude Code |      Cost");
    assert.ok(!output.includes("Input"), "must not switch to the single-tool history layout");
    assert.ok(!output.includes("Codex"), "zero-cost Codex column omitted");
  });

  it("filters on the post-maxRows visible window", (t) => {
    t.after(restoreLog);
    const e = (label: string, totalCost: number): UsageEntry => ({
      label, totalCost, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0,
    });
    const data = new Map<string, UsageEntry[]>([
      ["Claude Code", [e("2026-03-01", 1), e("2026-03-02", 1), e("2026-03-03", 1)]],
      // Codex has cost only on 2026-03-01 — outside the maxRows=2 window.
      ["Codex", [e("2026-03-01", 5)]],
    ]);
    printTotalHistory("daily", data, 140, { maxRows: 2 });
    let output = logged.join("\n");
    assert.ok(!output.includes("Codex"), "tool with cost only outside the window is omitted");
    assert.ok(!output.includes("2026-03-01"), "window truncated to the last 2 labels");

    // Without truncation the same tool keeps its column.
    captureLog();
    printTotalHistory("daily", data, 140);
    output = logged.join("\n");
    assert.ok(output.includes("Codex"), "tool with cost in the full window keeps its column");
  });

  it("falls back to the unfiltered tool list when every tool sums to zero", (t) => {
    t.after(restoreLog);
    const e = (label: string, totalCost: number): UsageEntry => ({
      label, totalCost, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0,
    });
    const data = new Map<string, UsageEntry[]>([
      ["Claude Code", [e("2026-02-14", 0)]],
      ["Codex", [e("2026-02-14", 0)]],
    ]);
    printTotalHistory("daily", data, 140);
    const output = logged.join("\n");
    // Defensive guard: no degenerate Date | Cost table.
    assert.ok(output.includes("Claude Code"), "fallback keeps Claude Code column");
    assert.ok(output.includes("Codex"), "fallback keeps Codex column");
    assert.ok(output.includes("$0.00"), "fallback renders the $0.00 cells");
  });

  it("renders a ≥$1,000 cost cell at exactly the 9-char column width (no overflow)", (t) => {
    t.after(restoreLog);
    const data = new Map<string, UsageEntry[]>([
      ["Claude Code", [
        { label: "2026-07-01", totalCost: 4031.61, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0 },
      ]],
    ]);
    printTotalHistory("daily", data, 140);
    const dataLine = logged.find((l) => stripAnsi(l).includes("2026-07-01"));
    assert.ok(dataLine, "expected the 2026-07-01 data row");
    const stripped = stripAnsi(dataLine!);
    assert.ok(stripped.includes("$4,031.61"), "thousands separator in the cost cell");
    // "$4,031.61" is 9 chars — exactly COST_WIDTH — so padStart adds nothing and
    // the row through the Cost cell stays 10 + 3 + 11 + 3 + 9 = 36.
    const costCell = "$4,031.61";
    const costEnd = stripped.indexOf(costCell, stripped.indexOf(costCell) + 1) + costCell.length;
    const fullRow = stripped.slice(0, costEnd);
    assert.equal(fullRow.length, 36, `row through the Cost cell must be 36 chars (got ${fullRow.length}): "${fullRow}"`);
  });
});

// ---------------------------------------------------------------------------
// FormatOptions backward compatibility (regression guard)
// ---------------------------------------------------------------------------
describe("FormatOptions backward compatibility", () => {
  beforeEach(() => captureLog());

  it("printHistory output identical with and without FormatOptions", (t) => {
    t.after(restoreLog);
    const entries: UsageEntry[] = [
      { label: "2026-03-01", totalCost: 12.40, inputTokens: 100, outputTokens: 50, cacheCreationTokens: 10, cacheReadTokens: 5, totalTokens: 165 },
    ];
    printHistory("Claude Code", "daily", entries, 120);
    const withoutOpts = logged.join("\n");
    logged = [];
    printHistory("Claude Code", "daily", entries, 120, undefined);
    const withUndefined = logged.join("\n");
    assert.equal(withoutOpts, withUndefined);
  });

  it("printTotal output identical with and without FormatOptions", (t) => {
    t.after(restoreLog);
    const totals = new Map<string, UsageTotals>([
      ["Claude Code", { totalCost: 45.20, inputTokens: 400, outputTokens: 200, cacheCreationTokens: 50, cacheReadTokens: 30, totalTokens: 680 }],
    ]);
    printTotal("daily", totals);
    const withoutOpts = logged.join("\n");
    logged = [];
    printTotal("daily", totals, undefined);
    const withUndefined = logged.join("\n");
    assert.equal(withoutOpts, withUndefined);
  });

  it("printTotalHistory output identical with and without FormatOptions", (t) => {
    t.after(restoreLog);
    const data = new Map<string, UsageEntry[]>([
      ["Claude Code", [
        { label: "2026-02-14", totalCost: 5, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0 },
      ]],
    ]);
    printTotalHistory("daily", data, 140);
    const withoutOpts = logged.join("\n");
    logged = [];
    printTotalHistory("daily", data, 140, undefined);
    const withUndefined = logged.join("\n");
    assert.equal(withoutOpts, withUndefined);
  });
});

// ---------------------------------------------------------------------------
// render* variants
// ---------------------------------------------------------------------------
describe("renderTotal", () => {
  it("returns string[] with table content and pipe separators", () => {
    const totals = new Map<string, UsageTotals>([
      ["Claude Code", { totalCost: 5, inputTokens: 1000, outputTokens: 500, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 1500 }],
    ]);
    const lines = renderTotal("daily", totals);
    assert.ok(Array.isArray(lines), "should return an array");
    assert.ok(lines.length > 0, "should have content");
    const text = lines.join("\n");
    assert.match(text, /Combined Usage.*daily/);
    assert.match(text, /Claude Code/);
    // Pipe separators
    assert.ok(text.includes(" | "), "should have pipe separators");
    assert.ok(text.includes("─|─"), "dividers should have pipe");
    // Header and total rows should align with data rows (pad-then-color)
    const headerLine = lines.find(l => l.includes("Tool"));
    const totalLine = lines.find(l => l.includes("Total"));
    if (headerLine && totalLine) {
      // Both should contain pipe separators
      assert.ok(headerLine.includes(" | "), "header should have pipe separator");
      assert.ok(totalLine.includes(" | "), "total should have pipe separator");
    }
  });

  it("matches printTotal output", (t) => {
    captureLog();
    t.after(restoreLog);
    const totals = new Map<string, UsageTotals>([
      ["Claude Code", { totalCost: 5, inputTokens: 1000, outputTokens: 500, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 1500 }],
    ]);
    printTotal("daily", totals);
    const printOutput = logged.join("\n");
    const renderOutput = renderTotal("daily", totals).join("\n");
    assert.equal(renderOutput, printOutput);
  });

  it("renders a combined Cache column whose cells close the row arithmetic", () => {
    const totals = new Map<string, UsageTotals>([
      ["Claude Code", { totalCost: 465.67, inputTokens: 3734, outputTokens: 1121329, cacheCreationTokens: 400000000, cacheReadTokens: 86557984, totalTokens: 487683047 }],
      ["Codex", { totalCost: 3, inputTokens: 800, outputTokens: 200, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 1000 }],
    ]);
    const lines = renderTotal("daily", totals).map(stripAnsi);
    const header = lines.find((l) => l.includes("Tool"))!;
    assert.deepEqual(header.split(" | ").map((c) => c.trim()), ["Tool", "Tokens", "Input", "Output", "Cache", "Cost"]);
    const ccRow = lines.find((l) => l.startsWith("Claude Code"))!;
    const cells = ccRow.split(" | ").map((c) => c.trim());
    assert.equal(cells[4], "486,557,984", "cache cell is write+read combined");
    const [tokens, input, output, cache] = cells.slice(1, 5).map((c) => Number(c.replace(/,/g, "")));
    assert.equal(input + output + cache, tokens, "Input + Output + Cache = Tokens");
    const totalRow = lines.find((l) => l.startsWith("Total"))!;
    const tCells = totalRow.split(" | ").map((c) => c.trim());
    assert.equal(tCells[4], "486,557,984", "Total row sums the cache column");
  });

  it("renders 0 in the Cache cell for a zero-cache tool", () => {
    const totals = new Map<string, UsageTotals>([
      ["Codex", { totalCost: 3, inputTokens: 800, outputTokens: 200, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 1000 }],
    ]);
    const lines = renderTotal("daily", totals).map(stripAnsi);
    const row = lines.find((l) => l.startsWith("Codex"))!;
    assert.equal(row.split(" | ").map((c) => c.trim())[4], "0");
  });

  it("all rows measure 87 visible chars (within the 90-col budget)", () => {
    const totals = new Map<string, UsageTotals>([
      ["Claude Code", { totalCost: 465.67, inputTokens: 3734, outputTokens: 1121329, cacheCreationTokens: 400000000, cacheReadTokens: 86557984, totalTokens: 487683047 }],
      ["Codex", { totalCost: 3, inputTokens: 800, outputTokens: 200, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 1000 }],
    ]);
    const lines = renderTotal("daily", totals).map(stripAnsi);
    const tableLines = lines.filter((l) => l.includes("|") || l.includes("─"));
    assert.ok(tableLines.length >= 5, "header, divider, 2 data rows, total divider + row");
    for (const l of tableLines) {
      assert.equal(l.length, 87, `row width must be 87: "${l}"`);
    }
  });

  it("appends machine columns after Cost with the new layout", () => {
    const totals = new Map<string, UsageTotals>([
      ["Claude Code", { totalCost: 5, inputTokens: 100, outputTokens: 50, cacheCreationTokens: 10, cacheReadTokens: 5, totalTokens: 165 }],
    ]);
    const machineCosts = new Map<string, Map<string, number>>([
      ["Claude Code", new Map([["alpha", 3.5], ["zebra", 1.5]])],
    ]);
    const lines = renderTotal("daily", totals, { machineCosts }).map(stripAnsi);
    const header = lines.find((l) => l.includes("Tool"))!;
    assert.deepEqual(header.split(" | ").map((c) => c.trim()), ["Tool", "Tokens", "Input", "Output", "Cache", "Cost", "A", "B"]);
    assert.ok(lines.some((l) => l.startsWith("Machines:")), "machine legend present");
  });
});

describe("renderHistory", () => {
  it("returns string[] with table content", () => {
    const entries: UsageEntry[] = [{
      label: "2026-02-14",
      totalCost: 1.50,
      inputTokens: 100,
      outputTokens: 200,
      cacheCreationTokens: 10,
      cacheReadTokens: 20,
      totalTokens: 330,
    }];
    const lines = renderHistory("Claude Code", "daily", entries, 140);
    assert.ok(Array.isArray(lines));
    const text = lines.join("\n");
    assert.match(text, /Claude Code.*daily/);
    assert.match(text, /2026-02-14/);
  });

  const capEntries: UsageEntry[] = [{
    label: "2026-05-14", totalCost: 1.5, inputTokens: 100, outputTokens: 200,
    cacheCreationTokens: 10, cacheReadTokens: 20, totalTokens: 330,
  }];

  it("appends 'last 3 months' to the heading when capActive is true", () => {
    const text = renderHistory("Claude Code", "daily", capEntries, 140, { capActive: true }).join("\n");
    assert.match(stripAnsi(text), /Claude Code \(daily, last 3 months\)/);
  });

  it("omits the hint when capActive is false/absent", () => {
    const off = renderHistory("Claude Code", "daily", capEntries, 140, { capActive: false }).join("\n");
    assert.doesNotMatch(stripAnsi(off), /last 3 months/);
    const absent = renderHistory("Claude Code", "daily", capEntries, 140).join("\n");
    assert.doesNotMatch(stripAnsi(absent), /last 3 months/);
  });
});

describe("renderTotalHistory", () => {
  it("returns string[] with pivot table", () => {
    const data = new Map<string, UsageEntry[]>([
      ["Claude Code", [
        { label: "2026-02-14", totalCost: 5, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0 },
      ]],
    ]);
    const lines = renderTotalHistory("daily", data, 140);
    assert.ok(Array.isArray(lines));
    const text = lines.join("\n");
    assert.match(text, /Combined Cost History.*daily/);
  });

  it("appends 'last 3 months' to the pivot heading when capActive is true", () => {
    const data = new Map<string, UsageEntry[]>([
      ["Claude Code", [
        { label: "2026-05-14", totalCost: 5, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0 },
      ]],
    ]);
    const text = renderTotalHistory("daily", data, 140, { capActive: true }).join("\n");
    assert.match(stripAnsi(text), /Combined Cost History \(daily, last 3 months\)/);
  });

  it("omits the pivot hint when capActive is absent", () => {
    const data = new Map<string, UsageEntry[]>([
      ["Claude Code", [
        { label: "2026-05-14", totalCost: 5, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0 },
      ]],
    ]);
    const text = renderTotalHistory("daily", data, 140).join("\n");
    assert.doesNotMatch(stripAnsi(text), /last 3 months/);
  });
});

// ---------------------------------------------------------------------------
// emitCsv / emitMarkdown stdout capture helpers
// ---------------------------------------------------------------------------
let stdoutChunks: string[];

function captureStdout() {
  stdoutChunks = [];
  mock.method(process.stdout, "write", ((chunk: string | Uint8Array) => {
    stdoutChunks.push(typeof chunk === "string" ? chunk : Buffer.from(chunk).toString("utf-8"));
    return true;
  }) as never);
}

function restoreStdout() {
  mock.restoreAll();
}

function stdoutText(): string {
  return stdoutChunks.join("");
}

// ---------------------------------------------------------------------------
// emitCsv
// ---------------------------------------------------------------------------
describe("emitCsv", () => {
  describe("snapshot kind", () => {
    it("emits tool,tokens,input,output,cache,cost header with data rows and Total", (t) => {
      captureStdout();
      t.after(restoreStdout);
      const totals = new Map<string, UsageTotals>([
        ["Claude Code", { totalCost: 12.34, inputTokens: 800000, outputTokens: 400000, cacheCreationTokens: 20000, cacheReadTokens: 14567, totalTokens: 1234567 }],
        ["Codex", { totalCost: 2.45, inputTokens: 150000, outputTokens: 80000, cacheCreationTokens: 0, cacheReadTokens: 4567, totalTokens: 234567 }],
      ]);
      emitCsv(totals, "snapshot", { period: "daily" });
      const out = stdoutText();
      const lines = out.split("\n");
      assert.equal(lines[0], "tool,tokens,input,output,cache,cost", "header row must exactly match");
      assert.ok(lines.some((l) => l.startsWith("Claude Code,")), "Claude Code row present");
      assert.ok(lines.includes("Claude Code,1234567,800000,400000,34567,12.34"), "cache cell is write+read combined, after output");
      assert.ok(lines.some((l) => l.startsWith("Codex,")), "Codex row present");
      assert.ok(lines.includes("Total,1469134,950000,480000,39134,14.79"), "Total row sums cache when >1 tool visible");
    });

    it("omits Total row when only one tool has data", (t) => {
      captureStdout();
      t.after(restoreStdout);
      const totals = new Map<string, UsageTotals>([
        ["Claude Code", { totalCost: 12.34, inputTokens: 800, outputTokens: 400, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 1200 }],
      ]);
      emitCsv(totals, "snapshot", { period: "daily" });
      const lines = stdoutText().split("\n");
      assert.ok(!lines.some((l) => l.startsWith("Total,")), "Total row omitted when one tool visible");
    });

    it("cost has two decimals and no $ sign", (t) => {
      captureStdout();
      t.after(restoreStdout);
      const totals = new Map<string, UsageTotals>([
        ["Tool A", { totalCost: 3.5, inputTokens: 100, outputTokens: 50, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 150 }],
      ]);
      emitCsv(totals, "snapshot", { period: "daily" });
      const out = stdoutText();
      assert.ok(out.includes(",3.50"), "cost should be formatted with two decimals");
      assert.ok(!out.includes("$"), "no $ sign in CSV output");
    });

    it("numeric fields have no thousands separators", (t) => {
      captureStdout();
      t.after(restoreStdout);
      const totals = new Map<string, UsageTotals>([
        ["Tool", { totalCost: 1, inputTokens: 1234567, outputTokens: 100, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 1234667 }],
      ]);
      emitCsv(totals, "snapshot", { period: "daily" });
      const out = stdoutText();
      assert.ok(out.includes("1234567"), "raw integer (no commas) expected");
      assert.ok(!out.includes("1,234,567"), "no thousands separators in CSV");
    });
  });

  describe("history kind", () => {
    it("emits date,input,output,cache_write,cache_read,total,cost header", (t) => {
      captureStdout();
      t.after(restoreStdout);
      const entries: UsageEntry[] = [
        { label: "2026-04-21", totalCost: 2.34, inputTokens: 80000, outputTokens: 40000, cacheCreationTokens: 20000, cacheReadTokens: 5000, totalTokens: 145000 },
        { label: "2026-04-22", totalCost: 2.89, inputTokens: 90000, outputTokens: 50000, cacheCreationTokens: 25000, cacheReadTokens: 6000, totalTokens: 171000 },
      ];
      emitCsv({ toolName: "Claude Code", entries }, "history", { period: "daily" });
      const lines = stdoutText().split("\n");
      assert.equal(lines[0], "date,input,output,cache_write,cache_read,total,cost", "exact header");
      assert.ok(lines[1].startsWith("2026-04-21,"), "ISO date label");
      assert.ok(lines[2].startsWith("2026-04-22,"), "ISO date label");
    });

    it("ISO month labels pass through for monthly period", (t) => {
      captureStdout();
      t.after(restoreStdout);
      const entries: UsageEntry[] = [
        { label: "2026-04", totalCost: 10, inputTokens: 100, outputTokens: 50, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 150 },
      ];
      emitCsv({ toolName: "Claude Code", entries }, "history", { period: "monthly" });
      const lines = stdoutText().split("\n");
      assert.ok(lines[1].startsWith("2026-04,"), "monthly ISO label");
    });
  });

  describe("total-history kind", () => {
    it("emits date,{tool1},{tool2},total header and rows sorted ascending", (t) => {
      captureStdout();
      t.after(restoreStdout);
      const data = new Map<string, UsageEntry[]>([
        ["Claude Code", [
          { label: "2026-04-22", totalCost: 2.89, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0 },
          { label: "2026-04-21", totalCost: 2.34, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0 },
        ]],
        ["Codex", [
          { label: "2026-04-21", totalCost: 0.5, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0 },
          { label: "2026-04-22", totalCost: 0.6, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0 },
        ]],
      ]);
      emitCsv(data, "total-history", { period: "daily" });
      const lines = stdoutText().split("\n");
      assert.equal(lines[0], "date,Claude Code,Codex,total");
      assert.ok(lines[1].startsWith("2026-04-21,"), "first data row ascending");
      assert.ok(lines[2].startsWith("2026-04-22,"), "second data row ascending");
    });

    it("retains every tool column with raw 0.00 cells (no zero-column omission, no separators)", (t) => {
      captureStdout();
      t.after(restoreStdout);
      // CSV is a machine contract: scripts index columns positionally, so
      // zero-cost tools keep their column (unlike the ANSI/Markdown pivots).
      const e = (label: string, totalCost: number): UsageEntry => ({
        label, totalCost, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0,
      });
      const data = new Map<string, UsageEntry[]>([
        ["Claude Code", [e("2026-04-21", 4031.61)]],
        ["Codex", [e("2026-04-21", 0)]],
        ["Kimi", []],
      ]);
      emitCsv(data, "total-history", { period: "daily" });
      const out = stdoutText();
      const lines = out.split("\n");
      assert.equal(lines[0], "date,Claude Code,Codex,Kimi,total", "all tool columns retained");
      assert.equal(lines[1], "2026-04-21,4031.61,0.00,0.00,4031.61", "raw-numeric cells, no separators");
      assert.ok(!out.includes("$"), "no $ sign in CSV");
      assert.ok(!out.includes("4,031.61"), "no thousands separators in CSV");
    });
  });

  describe("machine columns", () => {
    it("appends machine_{name}_cost columns sorted alphabetically", (t) => {
      captureStdout();
      t.after(restoreStdout);
      const totals = new Map<string, UsageTotals>([
        ["Claude Code", { totalCost: 5, inputTokens: 100, outputTokens: 50, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 150 }],
      ]);
      const machineCosts = new Map<string, Map<string, number>>([
        ["Claude Code", new Map([["zebra", 1.5], ["alpha", 3.5]])],
      ]);
      emitCsv(totals, "snapshot", { period: "daily", machineCosts });
      const lines = stdoutText().split("\n");
      assert.equal(lines[0], "tool,tokens,input,output,cache,cost,machine_alpha_cost,machine_zebra_cost", "alphabetical machine columns after cost");
      assert.ok(lines[1].endsWith("3.50,1.50"), "alpha=3.50, zebra=1.50");
    });
  });

  describe("RFC 4180 quoting", () => {
    it("quotes fields containing commas", (t) => {
      captureStdout();
      t.after(restoreStdout);
      const totals = new Map<string, UsageTotals>([
        ["A, B", { totalCost: 1.0, inputTokens: 10, outputTokens: 5, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 15 }],
      ]);
      emitCsv(totals, "snapshot", { period: "daily" });
      const out = stdoutText();
      assert.ok(out.includes('"A, B"'), "comma-containing field should be quoted");
    });

    it("quotes fields with double-quotes and doubles internal quotes", (t) => {
      captureStdout();
      t.after(restoreStdout);
      const totals = new Map<string, UsageTotals>([
        ['A"B', { totalCost: 1.0, inputTokens: 10, outputTokens: 5, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 15 }],
      ]);
      emitCsv(totals, "snapshot", { period: "daily" });
      const out = stdoutText();
      assert.ok(out.includes('"A""B"'), "internal quotes should be doubled");
    });
  });

  describe("file format", () => {
    it("uses LF line endings (no CRLF)", (t) => {
      captureStdout();
      t.after(restoreStdout);
      const totals = new Map<string, UsageTotals>([
        ["Tool", { totalCost: 1, inputTokens: 10, outputTokens: 5, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 15 }],
      ]);
      emitCsv(totals, "snapshot", { period: "daily" });
      const out = stdoutText();
      assert.ok(!out.includes("\r"), "no carriage returns");
      assert.ok(out.includes("\n"), "LF line terminators expected");
    });

    it("does not start with a BOM", (t) => {
      captureStdout();
      t.after(restoreStdout);
      const totals = new Map<string, UsageTotals>([
        ["Tool", { totalCost: 1, inputTokens: 10, outputTokens: 5, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 15 }],
      ]);
      emitCsv(totals, "snapshot", { period: "daily" });
      const out = stdoutText();
      assert.ok(!out.startsWith("﻿"), "no BOM");
    });

    it("emits no ANSI escape codes", (t) => {
      captureStdout();
      t.after(restoreStdout);
      const totals = new Map<string, UsageTotals>([
        ["Claude Code", { totalCost: 5, inputTokens: 100, outputTokens: 50, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 150 }],
        ["Codex", { totalCost: 3, inputTokens: 80, outputTokens: 40, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 120 }],
      ]);
      emitCsv(totals, "snapshot", { period: "daily" });
      assert.ok(!/\x1b\[/.test(stdoutText()), "no ANSI escape sequences");
    });
  });
});

// ---------------------------------------------------------------------------
// emitMarkdown
// ---------------------------------------------------------------------------
describe("emitMarkdown", () => {
  describe("snapshot kind", () => {
    it("begins with ## Combined Usage ({period}) heading", (t) => {
      captureStdout();
      t.after(restoreStdout);
      const totals = new Map<string, UsageTotals>([
        ["Claude Code", { totalCost: 12.34, inputTokens: 800, outputTokens: 400, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 1234 }],
        ["Codex", { totalCost: 2.45, inputTokens: 150, outputTokens: 80, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 234 }],
      ]);
      emitMarkdown(totals, "snapshot", { period: "monthly" });
      const lines = stdoutText().split("\n");
      assert.equal(lines[0], "## Combined Usage (monthly)", "heading must match ANSI renderer title");
      assert.equal(lines[1], "", "blank line after heading");
      assert.equal(lines[2], "| Tool | Tokens | Input | Output | Cache | Cost |", "GFM header row");
      assert.equal(lines[3], "| :--- | ---: | ---: | ---: | ---: | ---: |", "alignment row — string left, numeric right");
    });

    it("bolds the Total row when >1 tool visible", (t) => {
      captureStdout();
      t.after(restoreStdout);
      const totals = new Map<string, UsageTotals>([
        ["A", { totalCost: 1.0, inputTokens: 100, outputTokens: 50, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 150 }],
        ["B", { totalCost: 2.0, inputTokens: 200, outputTokens: 100, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 300 }],
      ]);
      emitMarkdown(totals, "snapshot", { period: "daily" });
      const out = stdoutText();
      assert.ok(out.includes("**Total**"), "Total cell bolded");
      assert.ok(out.includes("**$3.00**"), "Total cost bolded");
    });

    it("omits Total row when only one tool visible (mirrors visibleCount > 1 guard)", (t) => {
      captureStdout();
      t.after(restoreStdout);
      const totals = new Map<string, UsageTotals>([
        ["Only", { totalCost: 1, inputTokens: 100, outputTokens: 50, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 150 }],
      ]);
      emitMarkdown(totals, "snapshot", { period: "daily" });
      assert.ok(!stdoutText().includes("**Total**"), "no Total row with single visible tool");
    });

    it("numeric values include comma thousands separators", (t) => {
      captureStdout();
      t.after(restoreStdout);
      const totals = new Map<string, UsageTotals>([
        ["Big", { totalCost: 10, inputTokens: 1234567, outputTokens: 100, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 1234667 }],
      ]);
      emitMarkdown(totals, "snapshot", { period: "daily" });
      assert.ok(stdoutText().includes("1,234,567"), "comma thousands in MD");
    });

    it("cost values carry $ prefix", (t) => {
      captureStdout();
      t.after(restoreStdout);
      const totals = new Map<string, UsageTotals>([
        ["Tool", { totalCost: 12.34, inputTokens: 10, outputTokens: 5, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 15 }],
      ]);
      emitMarkdown(totals, "snapshot", { period: "daily" });
      assert.ok(stdoutText().includes("$12.34"), "$ prefix present");
    });

    it("ends with a trailing blank line", (t) => {
      captureStdout();
      t.after(restoreStdout);
      const totals = new Map<string, UsageTotals>([
        ["A", { totalCost: 1, inputTokens: 10, outputTokens: 5, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 15 }],
      ]);
      emitMarkdown(totals, "snapshot", { period: "daily" });
      const out = stdoutText();
      assert.ok(out.endsWith("\n"), "trailing newline present");
    });
  });

  describe("history kind", () => {
    it("begins with ## {toolName} ({period}) heading", (t) => {
      captureStdout();
      t.after(restoreStdout);
      const entries: UsageEntry[] = [
        { label: "2026-04-21", totalCost: 2.34, inputTokens: 80, outputTokens: 40, cacheCreationTokens: 20, cacheReadTokens: 5, totalTokens: 145 },
      ];
      emitMarkdown({ toolName: "Claude Code", entries }, "history", { period: "daily" });
      const lines = stdoutText().split("\n");
      assert.equal(lines[0], "## Claude Code (daily)");
    });

    it("heading carries ', last 3 months' when capActive is true", (t) => {
      captureStdout();
      t.after(restoreStdout);
      const entries: UsageEntry[] = [
        { label: "2026-05-21", totalCost: 2.34, inputTokens: 80, outputTokens: 40, cacheCreationTokens: 20, cacheReadTokens: 5, totalTokens: 145 },
      ];
      emitMarkdown({ toolName: "Claude Code", entries }, "history", { period: "daily", capActive: true });
      assert.equal(stdoutText().split("\n")[0], "## Claude Code (daily, last 3 months)");
    });

    it("date column is left-aligned, numeric columns right-aligned", (t) => {
      captureStdout();
      t.after(restoreStdout);
      const entries: UsageEntry[] = [
        { label: "2026-04-21", totalCost: 2.34, inputTokens: 80, outputTokens: 40, cacheCreationTokens: 20, cacheReadTokens: 5, totalTokens: 145 },
      ];
      emitMarkdown({ toolName: "Claude Code", entries }, "history", { period: "daily" });
      const lines = stdoutText().split("\n");
      // Heading, blank, header row, alignment row
      assert.equal(lines[3], "| :--- | ---: | ---: | ---: | ---: | ---: | ---: |");
    });
  });

  describe("total-history kind", () => {
    it("begins with ## Combined Cost History ({period}) heading", (t) => {
      captureStdout();
      t.after(restoreStdout);
      const data = new Map<string, UsageEntry[]>([
        ["Claude Code", [
          { label: "2026-04-21", totalCost: 2, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0 },
        ]],
      ]);
      emitMarkdown(data, "total-history", { period: "daily" });
      assert.equal(stdoutText().split("\n")[0], "## Combined Cost History (daily)");
    });

    it("heading carries ', last 3 months' when capActive is true", (t) => {
      captureStdout();
      t.after(restoreStdout);
      const data = new Map<string, UsageEntry[]>([
        ["Claude Code", [
          { label: "2026-05-21", totalCost: 2, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0 },
        ]],
      ]);
      emitMarkdown(data, "total-history", { period: "daily", capActive: true });
      assert.equal(stdoutText().split("\n")[0], "## Combined Cost History (daily, last 3 months)");
    });

    it("omits zero-cost tool columns from the GFM table", (t) => {
      captureStdout();
      t.after(restoreStdout);
      const e = (label: string, totalCost: number): UsageEntry => ({
        label, totalCost, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0,
      });
      const data = new Map<string, UsageEntry[]>([
        ["Claude Code", [e("2026-04-21", 2), e("2026-04-22", 3)]],
        ["Codex", [e("2026-04-21", 0)]],
        ["Kimi", []],
      ]);
      emitMarkdown(data, "total-history", { period: "daily" });
      const out = stdoutText();
      const lines = out.split("\n");
      assert.equal(lines[2], "| Date | Claude Code | Cost |", "GFM header omits zero-cost tools");
      assert.ok(!out.includes("Codex"), "zero-cost Codex column omitted");
      assert.ok(!out.includes("Kimi"), "no-entry Kimi column omitted");
      assert.ok(!out.includes("$0.00"), "no $0.00 cells from omitted columns");
      // Total row (labels.length > 1) renders against the filtered set.
      assert.ok(out.includes("| **Total** | **$5.00** | **$5.00** |"), "Total row matches filtered columns");
    });

    it("falls back to all tool columns when every tool sums to zero", (t) => {
      captureStdout();
      t.after(restoreStdout);
      const e = (label: string, totalCost: number): UsageEntry => ({
        label, totalCost, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0,
      });
      const data = new Map<string, UsageEntry[]>([
        ["Claude Code", [e("2026-04-21", 0)]],
        ["Codex", [e("2026-04-21", 0)]],
      ]);
      emitMarkdown(data, "total-history", { period: "daily" });
      const lines = stdoutText().split("\n");
      assert.equal(lines[2], "| Date | Claude Code | Codex | Cost |", "empty filter falls back to all tools");
    });
  });

  describe("machine columns", () => {
    it("uses machine names directly (no A/B/C letter codes, no legend line)", (t) => {
      captureStdout();
      t.after(restoreStdout);
      const totals = new Map<string, UsageTotals>([
        ["Claude Code", { totalCost: 5, inputTokens: 100, outputTokens: 50, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 150 }],
      ]);
      const machineCosts = new Map<string, Map<string, number>>([
        ["Claude Code", new Map([["macbook", 3], ["workstation", 2]])],
      ]);
      emitMarkdown(totals, "snapshot", { period: "daily", machineCosts });
      const out = stdoutText();
      assert.ok(out.includes("| macbook |"), "machine name appears as column header");
      assert.ok(out.includes("| workstation |"), "machine name appears as column header");
      assert.ok(!out.toLowerCase().includes("machines: a = "), "no legend line");
    });
  });

  describe("no ANSI, no bars, no delta arrows", () => {
    it("emits no ANSI escape sequences", (t) => {
      captureStdout();
      t.after(restoreStdout);
      const totals = new Map<string, UsageTotals>([
        ["A", { totalCost: 1, inputTokens: 10, outputTokens: 5, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 15 }],
        ["B", { totalCost: 2, inputTokens: 20, outputTokens: 10, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 30 }],
      ]);
      emitMarkdown(totals, "snapshot", { period: "daily" });
      assert.ok(!/\x1b\[/.test(stdoutText()), "no ANSI in Markdown output");
    });

    it("emits no inline bar characters or delta arrows", (t) => {
      captureStdout();
      t.after(restoreStdout);
      const totals = new Map<string, UsageTotals>([
        ["A", { totalCost: 1, inputTokens: 10, outputTokens: 5, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 15 }],
        ["B", { totalCost: 2, inputTokens: 20, outputTokens: 10, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 30 }],
      ]);
      emitMarkdown(totals, "snapshot", { period: "daily" });
      const out = stdoutText();
      assert.ok(!out.includes("█"), "no full-block bar chars");
      assert.ok(!out.includes("↑"), "no up-arrow delta");
      assert.ok(!out.includes("↓"), "no down-arrow delta");
    });
  });
});
