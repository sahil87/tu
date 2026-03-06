import { describe, it, beforeEach, afterEach, mock } from "node:test";
import assert from "node:assert/strict";

import { fmtNum, fmtCost, renderBar, printHistory, printTotal, printTotalHistory, renderHistory, renderTotal, renderTotalHistory } from "../src/formatter.js";
import { setNoColor } from "../src/colors.js";
import type { UsageTotals, UsageEntry } from "../src/types.js";

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
