import { describe, it, beforeEach, afterEach, mock } from "node:test";
import assert from "node:assert/strict";

import { fmtNum, fmtCost, renderBar, printHistory, printTotal, printTotalHistory, renderHistory, renderTotal, renderTotalHistory, emitCsv, emitMarkdown } from "../formatter.js";
import { setNoColor } from "../colors.js";
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
    // tableWidth = 97, GUTTER = 3, COST_WIDTH = 8, separator = 1
    // termWidth = 110 → barWidth = 110 - 97 - 3 - 8 - 1 = 1 < MIN_BAR_AREA
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
    // termWidth=188, tableWidth=97, GUTTER=3, COST_WIDTH=8, sep=1 → uncapped=79, capped at 30
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
    // 2 tools → colCount=3 → tableWidth = 12 + 2*17 = 46
    // termWidth=60 → barWidth = 60 - 46 - 3 - 8 - 1 = 2 < MIN_BAR_AREA
    printTotalHistory("daily", data, 60);
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
    // 3 tools → colCount=4 → tableWidth = 12 + 3*17 = 63
    // termWidth=140 → barWidth = min(140-63-3-8-1, 30) = min(65, 30) = 30
    printTotalHistory("daily", data, 140);
    const dataLine = logged.find(l => l.includes("2026-02-14"));
    assert.ok(dataLine);
    // Max value row ($18 total) should have bars
    assert.ok(dataLine!.includes("\u2588"), "expected bars in data row");
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
    it("emits tool,tokens,input,output,cost header with data rows and Total", (t) => {
      captureStdout();
      t.after(restoreStdout);
      const totals = new Map<string, UsageTotals>([
        ["Claude Code", { totalCost: 12.34, inputTokens: 800000, outputTokens: 400000, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 1234567 }],
        ["Codex", { totalCost: 2.45, inputTokens: 150000, outputTokens: 80000, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 234567 }],
      ]);
      emitCsv(totals, "snapshot", { period: "daily" });
      const out = stdoutText();
      const lines = out.split("\n");
      assert.equal(lines[0], "tool,tokens,input,output,cost", "header row must exactly match");
      assert.ok(lines.some((l) => l.startsWith("Claude Code,")), "Claude Code row present");
      assert.ok(lines.some((l) => l.startsWith("Codex,")), "Codex row present");
      assert.ok(lines.some((l) => l.startsWith("Total,")), "Total row present when >1 tool visible");
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
      assert.equal(lines[0], "tool,tokens,input,output,cost,machine_alpha_cost,machine_zebra_cost", "alphabetical machine columns");
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
      assert.equal(lines[2], "| Tool | Tokens | Input | Output | Cost |", "GFM header row");
      assert.equal(lines[3], "| :--- | ---: | ---: | ---: | ---: |", "alignment row — string left, numeric right");
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
