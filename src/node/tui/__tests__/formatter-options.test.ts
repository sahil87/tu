import { describe, it, beforeEach, afterEach } from "node:test";
import assert from "node:assert/strict";
import { fmtCostDelta, printHistory, printTotal, printTotalHistory } from "../formatter.js";
import { setNoColor } from "../colors.js";
import type { FormatOptions } from "../formatter.js";
import type { UsageEntry, UsageTotals } from "../../core/types.js";

// Disable ANSI colors for formatter tests
setNoColor(true);

// --- fmtCostDelta unit tests ---

describe("fmtCostDelta", () => {
  it("returns plain cost when no prevCosts", () => {
    assert.equal(fmtCostDelta(45.20, "cc", undefined), "$45.20");
  });

  it("returns plain cost when key not in prevCosts", () => {
    const prev = new Map([["other", 10]]);
    assert.equal(fmtCostDelta(45.20, "cc", prev), "$45.20");
  });

  it("appends ↑ when cost increased", () => {
    const prev = new Map([["cc", 40]]);
    assert.equal(fmtCostDelta(45.20, "cc", prev), "$45.20 ↑");
  });

  it("appends ↓ when cost decreased", () => {
    const prev = new Map([["cc", 50]]);
    assert.equal(fmtCostDelta(45.20, "cc", prev), "$45.20 ↓");
  });

  it("no indicator when cost unchanged", () => {
    const prev = new Map([["cc", 45.20]]);
    assert.equal(fmtCostDelta(45.20, "cc", prev), "$45.20");
  });
});

// --- Capture console.log output for formatter tests ---

let captured: string[] = [];
let originalLog: typeof console.log;

function captureStart() {
  captured = [];
  originalLog = console.log;
  console.log = (...args: unknown[]) => {
    captured.push(args.map(String).join(" "));
  };
}

function captureStop(): string {
  console.log = originalLog;
  return captured.join("\n");
}

const sampleEntries: UsageEntry[] = [
  { label: "2026-03-01", totalCost: 12.40, inputTokens: 100, outputTokens: 50, cacheCreationTokens: 10, cacheReadTokens: 5, totalTokens: 165 },
  { label: "2026-03-02", totalCost: 45.20, inputTokens: 400, outputTokens: 200, cacheCreationTokens: 50, cacheReadTokens: 30, totalTokens: 680 },
];

const sampleToolTotals = new Map<string, UsageTotals>([
  ["Claude Code", { totalCost: 45.20, inputTokens: 400, outputTokens: 200, cacheCreationTokens: 50, cacheReadTokens: 30, totalTokens: 680 }],
  ["Codex", { totalCost: 3.10, inputTokens: 30, outputTokens: 15, cacheCreationTokens: 2, cacheReadTokens: 1, totalTokens: 48 }],
]);

describe("printHistory with FormatOptions", () => {
  beforeEach(() => captureStart());
  afterEach(() => captureStop());

  it("renders delta indicators when prevCosts provided", () => {
    const opts: FormatOptions = {
      prevCosts: new Map([["Claude Code:2026-03-01", 10.00], ["Claude Code:2026-03-02", 45.20]]),
    };
    printHistory("Claude Code", "daily", sampleEntries, 120, opts);
    // 2026-03-01 cost went from $10.00 → $12.40 (↑); 2026-03-02 unchanged at $45.20 (no indicator)
    const line1 = captured.find(l => l.includes("2026-03-01"));
    const line2 = captured.find(l => l.includes("2026-03-02"));
    assert.ok(line1, "should have a line for 2026-03-01");
    assert.ok(line1!.includes("↑"), "2026-03-01 should have ↑ (cost increased from $10 to $12.40)");
    assert.ok(line2, "should have a line for 2026-03-02");
    assert.ok(!line2!.includes("↑") && !line2!.includes("↓"), "2026-03-02 should have no indicator (unchanged)");
  });

  it("renders without indicators when FormatOptions omitted", () => {
    printHistory("Claude Code", "daily", sampleEntries, 120);
    const output = captured.join("\n");
    assert.ok(!output.includes("↑"));
    assert.ok(!output.includes("↓"));
  });
});

describe("printHistory compact mode", () => {
  beforeEach(() => captureStart());
  afterEach(() => captureStop());

  it("uses compact layout when compact=true", () => {
    const opts: FormatOptions = { compact: true };
    printHistory("Claude Code", "daily", sampleEntries, 50, opts);
    const output = captured.join("\n");
    // Compact should NOT have column headers like "Input", "Output", etc.
    assert.ok(!output.includes("Input"));
    assert.ok(!output.includes("Output"));
    // Should have dates and costs
    assert.ok(output.includes("2026-03-01"));
    assert.ok(output.includes("$12.40"));
  });

  it("compact mode with delta indicators", () => {
    const opts: FormatOptions = {
      compact: true,
      prevCosts: new Map([["Claude Code:2026-03-01", 10.00]]),
    };
    printHistory("Claude Code", "daily", sampleEntries, 50, opts);
    const output = captured.join("\n");
    assert.ok(output.includes("↑"));
  });
});

describe("printTotal with FormatOptions", () => {
  beforeEach(() => captureStart());
  afterEach(() => captureStop());

  it("renders delta indicators when prevCosts provided", () => {
    const opts: FormatOptions = {
      prevCosts: new Map([["Claude Code", 40.00], ["Codex", 3.10]]),
    };
    printTotal("daily", sampleToolTotals, opts);
    const output = captured.join("\n");
    assert.ok(output.includes("↑"), "Claude Code cost increased, should show ↑");
  });

  it("renders without indicators when FormatOptions omitted", () => {
    printTotal("daily", sampleToolTotals);
    const output = captured.join("\n");
    assert.ok(!output.includes("↑"));
    assert.ok(!output.includes("↓"));
  });
});

describe("printTotal compact mode", () => {
  beforeEach(() => captureStart());
  afterEach(() => captureStop());

  it("uses compact layout when compact=true", () => {
    const opts: FormatOptions = { compact: true };
    printTotal("daily", sampleToolTotals, opts);
    const output = captured.join("\n");
    // Compact should NOT have "Tokens", "Input", "Output" headers
    assert.ok(!output.includes("Tokens"));
    assert.ok(!output.includes("Input"));
    // Should have tool names and costs
    assert.ok(output.includes("Claude Code"));
    assert.ok(output.includes("$45.20"));
    assert.ok(output.includes("Total"));
  });
});

describe("printTotalHistory with FormatOptions", () => {
  beforeEach(() => captureStart());
  afterEach(() => captureStop());

  it("renders delta indicators on row totals", () => {
    const allToolEntries = new Map<string, UsageEntry[]>([
      ["Claude Code", sampleEntries],
    ]);
    const opts: FormatOptions = {
      prevCosts: new Map([["total:2026-03-01", 10.00]]),
    };
    printTotalHistory("daily", allToolEntries, 120, opts);
    const output = captured.join("\n");
    assert.ok(output.includes("↑"));
  });

  it("renders without indicators when FormatOptions omitted", () => {
    const allToolEntries = new Map<string, UsageEntry[]>([
      ["Claude Code", sampleEntries],
    ]);
    printTotalHistory("daily", allToolEntries, 120);
    const output = captured.join("\n");
    assert.ok(!output.includes("↑"));
    assert.ok(!output.includes("↓"));
  });
});

describe("printTotalHistory compact mode", () => {
  beforeEach(() => captureStart());
  afterEach(() => captureStop());

  it("uses compact layout when compact=true", () => {
    const allToolEntries = new Map<string, UsageEntry[]>([
      ["Claude Code", sampleEntries],
    ]);
    const opts: FormatOptions = { compact: true };
    printTotalHistory("daily", allToolEntries, 50, opts);
    const output = captured.join("\n");
    // Compact should NOT have tool name columns
    assert.ok(!output.includes("Claude Code") || output.includes("Combined Cost"));
    // Should have dates and costs
    assert.ok(output.includes("2026-03-01"));
    assert.ok(output.includes("$12.40"));
  });
});

describe("FormatOptions backward compatibility", () => {
  beforeEach(() => captureStart());
  afterEach(() => captureStop());

  it("printHistory output unchanged without opts", () => {
    printHistory("Claude Code", "daily", sampleEntries, 120);
    const withoutOpts = captured.join("\n");
    captured = [];
    printHistory("Claude Code", "daily", sampleEntries, 120, undefined);
    const withUndefined = captured.join("\n");
    assert.equal(withoutOpts, withUndefined);
  });

  it("printTotal output unchanged without opts", () => {
    printTotal("daily", sampleToolTotals);
    const withoutOpts = captured.join("\n");
    captured = [];
    printTotal("daily", sampleToolTotals, undefined);
    const withUndefined = captured.join("\n");
    assert.equal(withoutOpts, withUndefined);
  });
});
