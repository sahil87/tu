import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { formatElapsed, computeBurnRate, buildFooter, ROLLING_WINDOW, mergeSideBySide } from "../src/watch.js";
import { setNoColor } from "../src/colors.js";
import type { WatchSession } from "../src/watch.js";

// Disable ANSI colors for test assertions
setNoColor(true);

describe("formatElapsed", () => {
  it("formats seconds only", () => {
    assert.equal(formatElapsed(45_000), "45s");
  });

  it("formats minutes and seconds", () => {
    assert.equal(formatElapsed(90_000), "1m 30s");
  });

  it("formats hours, minutes, seconds", () => {
    assert.equal(formatElapsed(3_661_000), "1h 1m 1s");
  });

  it("handles zero", () => {
    assert.equal(formatElapsed(0), "0s");
  });

  it("handles negative (clamps to 0)", () => {
    assert.equal(formatElapsed(-5_000), "0s");
  });

  it("omits zero leading units", () => {
    assert.equal(formatElapsed(60_000), "1m 0s");
  });

  it("shows hours for exactly 1 hour", () => {
    assert.equal(formatElapsed(3_600_000), "1h 0m 0s");
  });
});

describe("computeBurnRate", () => {
  it("returns null with fewer than 2 polls", () => {
    assert.equal(computeBurnRate([{ time: 1000, cost: 10 }]), null);
    assert.equal(computeBurnRate([]), null);
  });

  it("computes rate from 2 polls", () => {
    const history = [
      { time: 0, cost: 10 },
      { time: 10_000, cost: 20 },
    ];
    // delta $10 over 10s = $3600/hr
    const rate = computeBurnRate(history);
    assert.equal(rate, 3600);
  });

  it("uses rolling window of last 5 polls", () => {
    const history = [
      { time: 0, cost: 0 },      // outside window when 7 polls
      { time: 10_000, cost: 5 },  // outside window
      { time: 20_000, cost: 10 }, // window start
      { time: 30_000, cost: 15 },
      { time: 40_000, cost: 20 },
      { time: 50_000, cost: 25 },
      { time: 60_000, cost: 30 }, // latest
    ];
    // Window: indices 2-6, oldest=10@20s, latest=30@60s
    // delta $20 over 40s = $1800/hr
    const rate = computeBurnRate(history);
    assert.equal(rate, 1800);
  });

  it("returns 0 when no cost change", () => {
    const history = [
      { time: 0, cost: 50 },
      { time: 10_000, cost: 50 },
      { time: 20_000, cost: 50 },
    ];
    assert.equal(computeBurnRate(history), 0);
  });

  it("handles zero time delta gracefully", () => {
    const history = [
      { time: 1000, cost: 10 },
      { time: 1000, cost: 20 },
    ];
    assert.equal(computeBurnRate(history), 0);
  });
});

describe("ROLLING_WINDOW", () => {
  it("is 5", () => {
    assert.equal(ROLLING_WINDOW, 5);
  });
});

function makeSession(overrides: Partial<WatchSession> = {}): WatchSession {
  return {
    startTime: 0,
    startCost: 0,
    previousCosts: new Map(),
    pollHistory: [],
    totalTokens: 0,
    ...overrides,
  };
}

describe("buildFooter", () => {
  it("shows countdown when provided", () => {
    const s = makeSession({ pollHistory: [{ time: 0, cost: 10 }] });
    const footer = buildFooter(14, s, 120);
    assert.ok(footer.includes("Next refresh: 14s"));
  });

  it("shows Refreshing... when countdown is null", () => {
    const s = makeSession({ pollHistory: [{ time: 0, cost: 10 }] });
    const footer = buildFooter(null, s, 120);
    assert.ok(footer.includes("Refreshing..."));
  });

  it("does not include session delta (moved to SparkPanel)", () => {
    const now = Date.now();
    const s = makeSession({
      startTime: now - 60_000,
      startCost: 40,
      pollHistory: [
        { time: now - 60_000, cost: 40 },
        { time: now, cost: 44.32 },
      ],
    });
    const footer = buildFooter(10, s, 120);
    assert.ok(!footer.includes("Session:"), "session delta should not be in footer");
  });

  it("does not include burn rate (moved to SparkPanel)", () => {
    const now = Date.now();
    const s = makeSession({
      startTime: now - 20_000,
      startCost: 40,
      pollHistory: [
        { time: now - 20_000, cost: 40 },
        { time: now - 10_000, cost: 45 },
        { time: now, cost: 50 },
      ],
    });
    const footer = buildFooter(10, s, 120);
    assert.ok(!footer.includes("Rate:"), "burn rate should not be in footer");
  });

  it("includes key hints", () => {
    const s = makeSession({ pollHistory: [{ time: 0, cost: 10 }] });
    const footer = buildFooter(10, s, 120);
    assert.ok(footer.includes("↵ refresh · q quit"));
  });

  it("progressively truncates for narrow terminals", () => {
    const s = makeSession({ pollHistory: [{ time: 0, cost: 10 }] });
    // Full footer with all components
    const fullFooter = buildFooter(10, s, 200);
    // Very narrow footer should drop key hints
    const narrowFooter = buildFooter(10, s, 15);
    assert.ok(narrowFooter.length < fullFooter.length, "narrow footer should be shorter");
  });
});

describe("mergeSideBySide", () => {
  it("returns tableLines as-is when panelLines is empty", () => {
    const tableLines = ["line1", "line2", "line3"];
    const result = mergeSideBySide(tableLines, [], 80, 60);
    assert.deepEqual(result, tableLines);
  });

  it("merges table and panel lines with padding and gutter", () => {
    const tableLines = ["abc", "defgh"];
    const panelLines = ["PAN1", "PAN2"];
    const result = mergeSideBySide(tableLines, panelLines, 80, 10);
    // "abc" is 3 visible chars, padded to 10 = 7 spaces + 3-space gutter + panel
    assert.equal(result.length, 2);
    assert.ok(result[0].includes("PAN1"), "first line should contain panel content");
    assert.ok(result[1].includes("PAN2"), "second line should contain panel content");
  });

  it("pads shorter table when panel has more lines", () => {
    const tableLines = ["abc"];
    const panelLines = ["P1", "P2", "P3"];
    const result = mergeSideBySide(tableLines, panelLines, 80, 10);
    assert.equal(result.length, 3);
    assert.ok(result[2].includes("P3"), "third line should contain panel content");
  });

  it("pads shorter panel when table has more lines", () => {
    const tableLines = ["a", "b", "c"];
    const panelLines = ["P1"];
    const result = mergeSideBySide(tableLines, panelLines, 80, 10);
    assert.equal(result.length, 3);
    assert.ok(result[0].includes("P1"), "first line should contain panel");
    // Lines 2-3 should have table content but no panel content
    assert.ok(!result[2].includes("P"), "third line should not have panel content");
  });
});
