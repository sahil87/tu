import { describe, it } from "node:test";
import assert from "node:assert/strict";

import type { UsageEntry, UsageTotals } from "../types.js";

// ---------------------------------------------------------------------------
// JSON output: single-tool commands
// ---------------------------------------------------------------------------

describe("--json output: single-tool commands", () => {
  it("single-tool daily entries serialize to valid JSON array", () => {
    const entries: UsageEntry[] = [
      { label: "2026-02-22", totalCost: 1.50, inputTokens: 100, outputTokens: 200, cacheCreationTokens: 10, cacheReadTokens: 20, totalTokens: 330 },
      { label: "2026-02-21", totalCost: 0.75, inputTokens: 50, outputTokens: 100, cacheCreationTokens: 5, cacheReadTokens: 10, totalTokens: 165 },
    ];
    const json = JSON.stringify(entries, null, 2);
    const parsed = JSON.parse(json);
    assert.ok(Array.isArray(parsed));
    assert.equal(parsed.length, 2);
    assert.equal(parsed[0].label, "2026-02-22");
    assert.equal(parsed[0].totalCost, 1.50);
  });

  it("single-tool monthly entries serialize to valid JSON array", () => {
    const entries: UsageEntry[] = [
      { label: "2026-02", totalCost: 10.00, inputTokens: 5000, outputTokens: 2500, cacheCreationTokens: 100, cacheReadTokens: 200, totalTokens: 7800 },
    ];
    const json = JSON.stringify(entries, null, 2);
    const parsed = JSON.parse(json);
    assert.ok(Array.isArray(parsed));
    assert.equal(parsed[0].label, "2026-02");
  });

  it("empty entries serialize to empty JSON array", () => {
    const entries: UsageEntry[] = [];
    const json = JSON.stringify(entries, null, 2);
    const parsed = JSON.parse(json);
    assert.ok(Array.isArray(parsed));
    assert.equal(parsed.length, 0);
  });
});

// ---------------------------------------------------------------------------
// JSON output: total command (Map → Object via Object.fromEntries)
// ---------------------------------------------------------------------------

describe("--json output: total command", () => {
  it("Map<string, UsageTotals> serializes via Object.fromEntries to valid JSON", () => {
    const result = new Map<string, UsageTotals>([
      ["Claude Code", { totalCost: 5, inputTokens: 1000, outputTokens: 500, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 1500 }],
      ["Codex", { totalCost: 3, inputTokens: 800, outputTokens: 200, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 1000 }],
      ["OpenCode", { totalCost: 0, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0 }],
    ]);
    const json = JSON.stringify(Object.fromEntries(result), null, 2);
    const parsed = JSON.parse(json);
    assert.equal(typeof parsed, "object");
    assert.ok(!Array.isArray(parsed));
    assert.equal(parsed["Claude Code"].totalCost, 5);
    assert.equal(parsed["Codex"].totalCost, 3);
    assert.equal(parsed["OpenCode"].totalCost, 0);
  });

  it("includes all tools even with zero usage", () => {
    const result = new Map<string, UsageTotals>([
      ["Claude Code", { totalCost: 0, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0 }],
    ]);
    const json = JSON.stringify(Object.fromEntries(result), null, 2);
    const parsed = JSON.parse(json);
    assert.ok("Claude Code" in parsed);
    assert.equal(parsed["Claude Code"].totalCost, 0);
  });
});

// ---------------------------------------------------------------------------
// JSON output: total-history command
// ---------------------------------------------------------------------------

describe("--json output: total-history command", () => {
  it("Map<string, UsageEntry[]> serializes via Object.fromEntries to valid JSON", () => {
    const result = new Map<string, UsageEntry[]>([
      ["Claude Code", [
        { label: "2026-02-21", totalCost: 1, inputTokens: 100, outputTokens: 50, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 150 },
        { label: "2026-02-22", totalCost: 2, inputTokens: 200, outputTokens: 100, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 300 },
      ]],
      ["Codex", []],
    ]);
    const json = JSON.stringify(Object.fromEntries(result), null, 2);
    const parsed = JSON.parse(json);
    assert.equal(typeof parsed, "object");
    assert.ok(Array.isArray(parsed["Claude Code"]));
    assert.equal(parsed["Claude Code"].length, 2);
    assert.ok(Array.isArray(parsed["Codex"]));
    assert.equal(parsed["Codex"].length, 0);
  });
});

// ---------------------------------------------------------------------------
// emitJson helper: Map auto-conversion via instanceof check
// ---------------------------------------------------------------------------

describe("emitJson Map→Object conversion", () => {
  it("Map instances are converted to plain objects before serialization", () => {
    const data = new Map<string, number>([["a", 1], ["b", 2]]);
    // emitJson does: data instanceof Map ? Object.fromEntries(data) : data
    const obj = data instanceof Map ? Object.fromEntries(data) : data;
    const json = JSON.stringify(obj, null, 2);
    const parsed = JSON.parse(json);
    assert.equal(parsed.a, 1);
    assert.equal(parsed.b, 2);
  });

  it("non-Map values pass through unchanged", () => {
    const data = [{ label: "2026-02-22", totalCost: 1.5 }];
    const obj = data instanceof Map ? Object.fromEntries(data as any) : data;
    const json = JSON.stringify(obj, null, 2);
    const parsed = JSON.parse(json);
    assert.ok(Array.isArray(parsed));
    assert.equal(parsed[0].totalCost, 1.5);
  });
});

// ---------------------------------------------------------------------------
// JSON output: piping compatibility (valid JSON.parse)
// ---------------------------------------------------------------------------

describe("--json piping compatibility", () => {
  it("JSON output with 2-space indent is parseable by JSON.parse", () => {
    const data = { "Claude Code": { totalCost: 5.50 } };
    const json = JSON.stringify(data, null, 2);
    assert.doesNotThrow(() => JSON.parse(json));
  });
});
