import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { buildStatsGrid } from "../panel.js";
import type { PanelSession } from "../panel.js";
import { setNoColor } from "../colors.js";

// Disable colors for simpler assertions
setNoColor(true);

function makeSession(overrides: Partial<PanelSession> = {}): PanelSession {
  const now = Date.now();
  return {
    startTime: now - 300_000, // 5 minutes ago
    startCost: 10,
    startTokens: 0,
    pollHistory: [
      { time: now - 300_000, cost: 10 },
      { time: now - 200_000, cost: 15 },
      { time: now - 100_000, cost: 20 },
      { time: now, cost: 25 },
    ],
    totalTokens: 50000,
    ...overrides,
  };
}

describe("buildStatsGrid", () => {
  it("returns 4 lines (3 grid rows + separator)", () => {
    const result = buildStatsGrid(makeSession(), 25);
    assert.equal(result.length, 4, "should return 4 lines");
  });

  it("shows Elapsed and Session labels", () => {
    const result = buildStatsGrid(makeSession(), 25);
    const text = result.join("\n");
    assert.ok(text.includes("Elapsed"), "should show Elapsed");
    assert.ok(text.includes("Session"), "should show Session delta");
  });

  it("shows Tok/min when session has tokens and 2+ polls", () => {
    const result = buildStatsGrid(makeSession({ totalTokens: 100000 }), 25);
    const text = result.join("\n");
    assert.ok(text.includes("Tok/min"), "should show Tok/min");
    assert.ok(!text.includes("Tok/min   --"), "should show a real value, not placeholder");
  });

  it("shows dashes for Tok/min on first poll", () => {
    const now = Date.now();
    const result = buildStatsGrid(makeSession({
      totalTokens: 100000,
      pollHistory: [{ time: now, cost: 10 }],
    }), 25);
    const text = result.join("\n");
    assert.ok(text.includes("Tok/min"), "should show Tok/min label");
    assert.ok(text.includes("--"), "should show placeholder");
  });

  it("shows $0.00 for Session delta on first poll", () => {
    const now = Date.now();
    const result = buildStatsGrid(makeSession({
      pollHistory: [{ time: now, cost: 10 }],
    }), 25);
    const text = result.join("\n");
    const lines = result.filter(l => l.includes("Session"));
    assert.ok(lines.length > 0, "should have Session line");
    assert.ok(lines[0].includes("$0.00"), "should show $0.00 for session delta on first poll");
  });

  it("shows burn rate when poll history has 2+ entries", () => {
    const result = buildStatsGrid(makeSession(), 25);
    const text = result.join("\n");
    assert.ok(text.includes("Rate"), "should show burn rate");
  });

  it("shows dashes for Rate when only 1 poll", () => {
    const now = Date.now();
    const result = buildStatsGrid(makeSession({
      pollHistory: [{ time: now, cost: 10 }],
    }), 25);
    const text = result.join("\n");
    const rateLine = result.find(l => l.includes("Rate"));
    assert.ok(rateLine, "should have Rate label");
    assert.ok(rateLine!.includes("--"), "should show -- placeholder for Rate");
  });

  it("shows projected daily cost", () => {
    const result = buildStatsGrid(makeSession(), 25);
    const text = result.join("\n");
    assert.ok(text.includes("Proj. day"), "should show projected daily cost");
  });

  it("shows session cost delta with + prefix when 2+ polls", () => {
    const result = buildStatsGrid(makeSession(), 25);
    const text = result.join("\n");
    assert.ok(text.includes("+$"), "should show positive delta with + prefix");
  });

  it("ends with a separator line (dim horizontal rule)", () => {
    const result = buildStatsGrid(makeSession(), 25);
    const lastLine = result[result.length - 1];
    assert.ok(lastLine.includes("\u2500"), "last line should contain horizontal rule characters");
  });

  it("grid stays fixed at 3 rows + separator even with no data", () => {
    const now = Date.now();
    const result = buildStatsGrid(makeSession({
      startTime: now,
      startCost: 0,
      pollHistory: [],
      totalTokens: 0,
    }), 0);
    assert.equal(result.length, 4, "should always return 4 lines");
  });

  it("shows dashes for Tok/min when zero tokens after multiple polls", () => {
    const result = buildStatsGrid(makeSession({ totalTokens: 0 }), 25);
    const text = result.join("\n");
    const tokLine = result.find(l => l.includes("Tok/min"));
    assert.ok(tokLine, "should have Tok/min label");
    assert.ok(tokLine!.includes("--"), "should show -- for zero tokens");
  });
});
