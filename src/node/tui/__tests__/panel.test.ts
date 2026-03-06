import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { buildPanel } from "../panel.js";
import type { PanelSession } from "../panel.js";
import { setNoColor } from "../colors.js";
import type { UsageEntry } from "../../core/types.js";

// Disable colors for simpler assertions
setNoColor(true);

function makeSession(overrides: Partial<PanelSession> = {}): PanelSession {
  const now = Date.now();
  return {
    startTime: now - 300_000, // 5 minutes ago
    startCost: 10,
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

function makeHistory(days: number): UsageEntry[] {
  return Array.from({ length: days }, (_, i) => ({
    label: `2026-01-${String(i + 1).padStart(2, "0")}`,
    totalCost: 10 + i * 2,
    inputTokens: 1000,
    outputTokens: 500,
    cacheCreationTokens: 100,
    cacheReadTokens: 50,
    totalTokens: 1650,
  }));
}

describe("buildPanel", () => {
  it("returns empty array for narrow panel width", () => {
    const result = buildPanel(makeSession(), makeHistory(7), 15, 25);
    assert.deepEqual(result, []);
  });

  it("renders panel with sparkline and session stats", () => {
    const result = buildPanel(makeSession(), makeHistory(7), 30, 25);
    assert.ok(result.length > 0, "should return lines");
    const text = result.join("\n");
    assert.ok(text.includes("Session"), "should have session stats header");
    assert.ok(text.includes("Elapsed"), "should show elapsed time");
  });

  it("renders panel without sparkline when history is empty", () => {
    const result = buildPanel(makeSession(), [], 30, 25);
    assert.ok(result.length > 0, "should return lines");
    const text = result.join("\n");
    assert.ok(text.includes("Session"), "should have session stats");
    assert.ok(!text.includes("Spend"), "should not have sparkline title");
  });

  it("renders panel without sparkline when history has 1 entry", () => {
    const result = buildPanel(makeSession(), makeHistory(1), 30, 25);
    const text = result.join("\n");
    assert.ok(!text.includes("Spend"), "should not have sparkline for single entry");
  });

  it("shows tokens/min when session has tokens", () => {
    const result = buildPanel(makeSession({ totalTokens: 100000 }), [], 30, 25);
    const text = result.join("\n");
    assert.ok(text.includes("Tokens/min"), "should show tokens/min");
  });

  it("shows burn rate when poll history has 2+ entries", () => {
    const result = buildPanel(makeSession(), [], 30, 25);
    const text = result.join("\n");
    assert.ok(text.includes("Rate"), "should show burn rate");
  });

  it("shows session cost delta when poll history has 2+ entries", () => {
    const result = buildPanel(makeSession(), [], 30, 25);
    const text = result.join("\n");
    assert.ok(text.includes("Session"), "should show session cost delta");
    assert.ok(text.includes("+$"), "should show positive delta with + prefix");
  });

  it("omits session cost delta on first poll", () => {
    const now = Date.now();
    const result = buildPanel(makeSession({
      pollHistory: [{ time: now, cost: 10 }],
    }), [], 30, 25);
    const text = result.join("\n");
    // "Session" appears as the section header, but not as a cost delta stat
    const lines = result.filter(l => l.includes("+$"));
    assert.equal(lines.length, 0, "should not show cost delta on first poll");
  });

  it("shows projected daily cost", () => {
    const result = buildPanel(makeSession(), [], 30, 25);
    const text = result.join("\n");
    assert.ok(text.includes("Proj. day"), "should show projected daily cost");
  });
});
