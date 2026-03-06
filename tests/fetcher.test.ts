import { describe, it } from "node:test";
import assert from "node:assert/strict";

import {
  stripNoise,
  normalizeLabel,
  toUsageTotals,
  toUsageEntry,
  parseJson,
  currentLabel,
  pickCurrentEntry,
  mergeEntries,
  aggregateMonthly,
  TOOLS,
  EMPTY,
} from "../src/fetcher.js";
import type { UsageEntry } from "../src/types.js";

// ---------------------------------------------------------------------------
// stripNoise
// ---------------------------------------------------------------------------
describe("stripNoise", () => {
  it("removes lines starting with '['", () => {
    const input = `[info] fetching data\n{"ok":true}\n[warn] done`;
    assert.equal(stripNoise(input), '{"ok":true}');
  });

  it("returns input unchanged when no noisy lines", () => {
    const input = '{"ok":true}\n{"more":1}';
    assert.equal(stripNoise(input), input);
  });

  it("handles empty string", () => {
    assert.equal(stripNoise(""), "");
  });

  it("handles input where all lines are noise", () => {
    assert.equal(stripNoise("[a]\n[b]\n[c]"), "");
  });
});

// ---------------------------------------------------------------------------
// normalizeLabel
// ---------------------------------------------------------------------------
describe("normalizeLabel", () => {
  it("converts daily format 'Feb 14, 2026' → '2026-02-14'", () => {
    assert.equal(normalizeLabel("Feb 14, 2026"), "2026-02-14");
  });

  it("zero-pads single-digit days", () => {
    assert.equal(normalizeLabel("Jan 3, 2026"), "2026-01-03");
  });

  it("converts monthly format 'Feb 2026' → '2026-02'", () => {
    assert.equal(normalizeLabel("Feb 2026"), "2026-02");
  });

  it("handles all 12 months", () => {
    const months = [
      ["Jan", "01"], ["Feb", "02"], ["Mar", "03"], ["Apr", "04"],
      ["May", "05"], ["Jun", "06"], ["Jul", "07"], ["Aug", "08"],
      ["Sep", "09"], ["Oct", "10"], ["Nov", "11"], ["Dec", "12"],
    ];
    for (const [abbr, num] of months) {
      assert.equal(normalizeLabel(`${abbr} 2026`), `2026-${num}`);
    }
  });

  it("returns unrecognized labels unchanged", () => {
    assert.equal(normalizeLabel("2026-02-14"), "2026-02-14");
    assert.equal(normalizeLabel("something else"), "something else");
  });

  it("uses '00' for unknown month abbreviations in daily format", () => {
    assert.equal(normalizeLabel("Xyz 5, 2026"), "2026-00-05");
  });
});

// ---------------------------------------------------------------------------
// toUsageTotals
// ---------------------------------------------------------------------------
describe("toUsageTotals", () => {
  it("maps standard fields", () => {
    const input = {
      totalCost: 1.5,
      inputTokens: 100,
      outputTokens: 200,
      cacheCreationTokens: 50,
      cacheReadTokens: 75,
      totalTokens: 425,
    };
    const result = toUsageTotals(input);
    assert.deepEqual(result, input);
  });

  it("maps legacy field costUSD → totalCost", () => {
    const result = toUsageTotals({ costUSD: 3.14 });
    assert.equal(result.totalCost, 3.14);
  });

  it("maps legacy field cachedInputTokens → cacheReadTokens", () => {
    const result = toUsageTotals({ cachedInputTokens: 999 });
    assert.equal(result.cacheReadTokens, 999);
  });

  it("prefers standard fields over legacy when both present", () => {
    const result = toUsageTotals({ totalCost: 5, costUSD: 1 });
    assert.equal(result.totalCost, 5);
  });

  it("defaults all fields to 0 for empty input", () => {
    const result = toUsageTotals({});
    assert.deepEqual(result, {
      totalCost: 0,
      inputTokens: 0,
      outputTokens: 0,
      cacheCreationTokens: 0,
      cacheReadTokens: 0,
      totalTokens: 0,
    });
  });

  it("coerces string numbers", () => {
    const result = toUsageTotals({ totalCost: "2.5", inputTokens: "100" });
    assert.equal(result.totalCost, 2.5);
    assert.equal(result.inputTokens, 100);
  });

  it("defaults NaN-producing values to 0", () => {
    const result = toUsageTotals({ totalCost: "not-a-number" });
    assert.equal(result.totalCost, 0);
  });
});

// ---------------------------------------------------------------------------
// toUsageEntry
// ---------------------------------------------------------------------------
describe("toUsageEntry", () => {
  it("extracts and normalizes label from specified key", () => {
    const entry = toUsageEntry(
      { date: "Feb 14, 2026", totalCost: 1, inputTokens: 10, outputTokens: 20, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 30 },
      "date"
    );
    assert.equal(entry.label, "2026-02-14");
    assert.equal(entry.totalCost, 1);
    assert.equal(entry.totalTokens, 30);
  });

  it("uses monthly label key", () => {
    const entry = toUsageEntry({ month: "Feb 2026", totalTokens: 5 }, "month");
    assert.equal(entry.label, "2026-02");
  });

  it("defaults label to empty string when key missing", () => {
    const entry = toUsageEntry({ totalTokens: 5 }, "date");
    assert.equal(entry.label, "");
  });
});

// ---------------------------------------------------------------------------
// parseJson
// ---------------------------------------------------------------------------
describe("parseJson", () => {
  it("parses valid JSON without filtering", () => {
    const result = parseJson('{"a":1}', false);
    assert.deepEqual(result, { a: 1 });
  });

  it("parses valid JSON with filtering enabled", () => {
    const result = parseJson('[info] log\n{"a":1}', true);
    assert.deepEqual(result, { a: 1 });
  });

  it("returns null for empty string", () => {
    assert.equal(parseJson("", false), null);
    assert.equal(parseJson("   ", false), null);
  });

  it("returns null for invalid JSON", () => {
    assert.equal(parseJson("not json", false), null);
  });

  it("returns null when filtering leaves invalid JSON", () => {
    assert.equal(parseJson("[only noise lines]", true), null);
  });
});

// ---------------------------------------------------------------------------
// currentLabel
// ---------------------------------------------------------------------------
describe("currentLabel", () => {
  it("returns ISO date for daily period", () => {
    const now = new Date(2026, 1, 16); // Feb 16, 2026
    assert.equal(currentLabel("daily", now), "2026-02-16");
  });

  it("returns ISO month for monthly period", () => {
    const now = new Date(2026, 1, 16);
    assert.equal(currentLabel("monthly", now), "2026-02");
  });

  it("zero-pads single-digit month and day", () => {
    const now = new Date(2026, 0, 5); // Jan 5, 2026
    assert.equal(currentLabel("daily", now), "2026-01-05");
  });

  it("defaults to daily-style label for unknown period", () => {
    const now = new Date(2026, 1, 16);
    assert.equal(currentLabel("weekly", now), "2026-02-16");
  });
});

// ---------------------------------------------------------------------------
// pickCurrentEntry
// ---------------------------------------------------------------------------
describe("pickCurrentEntry", () => {
  it("returns matching entry for today (daily)", () => {
    const now = new Date(2026, 1, 16);
    const entries = [
      { date: "Feb 15, 2026", totalCost: 1, inputTokens: 100, outputTokens: 50, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 150 },
      { date: "Feb 16, 2026", totalCost: 2, inputTokens: 200, outputTokens: 100, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 300 },
    ];
    const result = pickCurrentEntry(entries, "daily", now);
    assert.equal(result.totalCost, 2);
    assert.equal(result.totalTokens, 300);
  });

  it("returns EMPTY when no entry matches today", () => {
    const now = new Date(2026, 1, 16);
    const entries = [
      { date: "Feb 14, 2026", totalCost: 1, inputTokens: 100, outputTokens: 50, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 150 },
      { date: "Feb 15, 2026", totalCost: 2, inputTokens: 200, outputTokens: 100, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 300 },
    ];
    const result = pickCurrentEntry(entries, "daily", now);
    assert.equal(result.totalCost, 0);
    assert.equal(result.totalTokens, 0);
  });

  it("returns matching entry for current month (monthly)", () => {
    const now = new Date(2026, 1, 16);
    const entries = [
      { month: "Jan 2026", totalCost: 5, inputTokens: 1000, outputTokens: 500, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 1500 },
      { month: "Feb 2026", totalCost: 3, inputTokens: 800, outputTokens: 200, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 1000 },
    ];
    const result = pickCurrentEntry(entries, "monthly", now);
    assert.equal(result.totalCost, 3);
    assert.equal(result.totalTokens, 1000);
  });

  it("returns EMPTY when no entry matches current month", () => {
    const now = new Date(2026, 2, 1); // March 2026
    const entries = [
      { month: "Jan 2026", totalCost: 5, inputTokens: 1000, outputTokens: 500, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 1500 },
      { month: "Feb 2026", totalCost: 3, inputTokens: 800, outputTokens: 200, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 1000 },
    ];
    const result = pickCurrentEntry(entries, "monthly", now);
    assert.equal(result.totalCost, 0);
    assert.equal(result.totalTokens, 0);
  });

  // Regression: old code took entries[entries.length - 1] regardless of date
  it("does not attribute historical usage to today", () => {
    const now = new Date(2026, 1, 16); // Today is Feb 16
    const entries = [
      { date: "Feb 10, 2026", totalCost: 10, inputTokens: 5000, outputTokens: 2000, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 7000 },
      { date: "Feb 13, 2026", totalCost: 5, inputTokens: 2000, outputTokens: 1000, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 3000 },
    ];
    const result = pickCurrentEntry(entries, "daily", now);
    assert.equal(result.totalCost, 0);
    assert.equal(result.totalTokens, 0);
  });

  it("handles entries already in ISO format", () => {
    const now = new Date(2026, 1, 16);
    const entries = [
      { date: "2026-02-16", totalCost: 4, inputTokens: 300, outputTokens: 150, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 450 },
    ];
    const result = pickCurrentEntry(entries, "daily", now);
    assert.equal(result.totalCost, 4);
  });

  it("handles legacy field names in matched entry", () => {
    const now = new Date(2026, 1, 16);
    const entries = [
      { date: "Feb 16, 2026", costUSD: 7.5, cachedInputTokens: 500, totalTokens: 1000 },
    ];
    const result = pickCurrentEntry(entries, "daily", now);
    assert.equal(result.totalCost, 7.5);
    assert.equal(result.cacheReadTokens, 500);
  });
});

// ---------------------------------------------------------------------------
// TOOLS registry
// ---------------------------------------------------------------------------
describe("TOOLS", () => {
  it("has entries for cc, codex, and oc", () => {
    assert.ok(TOOLS.cc);
    assert.ok(TOOLS.codex);
    assert.ok(TOOLS.oc);
  });

  it("cc does not need filtering", () => {
    assert.equal(TOOLS.cc.needsFilter, false);
  });

  it("codex and oc need filtering", () => {
    assert.equal(TOOLS.codex.needsFilter, true);
    assert.equal(TOOLS.oc.needsFilter, true);
  });
});

// ---------------------------------------------------------------------------
// aggregateMonthly
// ---------------------------------------------------------------------------

const mkEntry = (label: string, cost: number, tokens = 100): UsageEntry => ({
  label,
  totalCost: cost,
  inputTokens: tokens,
  outputTokens: tokens / 2,
  cacheCreationTokens: 0,
  cacheReadTokens: 0,
  totalTokens: tokens,
});

describe("aggregateMonthly", () => {
  it("aggregates daily entries into monthly", () => {
    const daily = [
      mkEntry("2026-02-18", 1.0, 100),
      mkEntry("2026-02-19", 2.0, 200),
      mkEntry("2026-02-20", 3.0, 300),
    ];
    const result = aggregateMonthly(daily);
    assert.equal(result.length, 1);
    assert.equal(result[0].label, "2026-02");
    assert.equal(result[0].totalCost, 6.0);
    assert.equal(result[0].inputTokens, 600);
    assert.equal(result[0].totalTokens, 600);
  });

  it("groups entries spanning multiple months", () => {
    const daily = [
      mkEntry("2026-01-31", 1.0, 100),
      mkEntry("2026-02-01", 2.0, 200),
    ];
    const result = aggregateMonthly(daily);
    assert.equal(result.length, 2);
    assert.equal(result[0].label, "2026-01");
    assert.equal(result[0].totalCost, 1.0);
    assert.equal(result[1].label, "2026-02");
    assert.equal(result[1].totalCost, 2.0);
  });

  it("returns sorted by month label", () => {
    const daily = [
      mkEntry("2026-03-01", 3.0),
      mkEntry("2026-01-15", 1.0),
      mkEntry("2026-02-10", 2.0),
    ];
    const result = aggregateMonthly(daily);
    assert.deepEqual(
      result.map((e) => e.label),
      ["2026-01", "2026-02", "2026-03"]
    );
  });

  it("returns empty array for empty input", () => {
    assert.deepEqual(aggregateMonthly([]), []);
  });

  it("does not mutate input", () => {
    const daily = [mkEntry("2026-02-20", 1.0)];
    const copy = JSON.parse(JSON.stringify(daily));
    aggregateMonthly(daily);
    assert.deepEqual(daily, copy);
  });
});

// ---------------------------------------------------------------------------
// mergeEntries
// ---------------------------------------------------------------------------

describe("mergeEntries", () => {
  it("sums numeric fields for overlapping labels", () => {
    const local = [mkEntry("2026-02-20", 1.2, 100)];
    const remote = [mkEntry("2026-02-20", 0.8, 200)];
    const result = mergeEntries(local, remote);
    assert.equal(result.length, 1);
    assert.equal(result[0].label, "2026-02-20");
    assert.equal(result[0].totalCost, 2.0);
    assert.equal(result[0].inputTokens, 300);
    assert.equal(result[0].totalTokens, 300);
  });

  it("preserves non-overlapping entries from both sources", () => {
    const local = [mkEntry("2026-02-19", 1.0)];
    const remote = [mkEntry("2026-02-20", 2.0)];
    const result = mergeEntries(local, remote);
    assert.equal(result.length, 2);
    assert.equal(result[0].label, "2026-02-19");
    assert.equal(result[1].label, "2026-02-20");
  });

  it("returns local entries when remote is empty", () => {
    const local = [mkEntry("2026-02-20", 1.5)];
    const result = mergeEntries(local, []);
    assert.equal(result.length, 1);
    assert.equal(result[0].totalCost, 1.5);
  });

  it("returns remote entries when local is empty", () => {
    const remote = [mkEntry("2026-02-20", 2.0)];
    const result = mergeEntries([], remote);
    assert.equal(result.length, 1);
    assert.equal(result[0].totalCost, 2.0);
  });

  it("returns empty array when both are empty", () => {
    assert.deepEqual(mergeEntries([], []), []);
  });

  it("sorts result ascending by label", () => {
    const local = [mkEntry("2026-02-20", 1.0)];
    const remote = [mkEntry("2026-02-18", 0.5), mkEntry("2026-02-22", 0.3)];
    const result = mergeEntries(local, remote);
    assert.deepEqual(
      result.map((e) => e.label),
      ["2026-02-18", "2026-02-20", "2026-02-22"]
    );
  });

  it("does not mutate input arrays", () => {
    const local = [mkEntry("2026-02-20", 1.0)];
    const remote = [mkEntry("2026-02-20", 0.5)];
    const localCopy = JSON.parse(JSON.stringify(local));
    const remoteCopy = JSON.parse(JSON.stringify(remote));
    mergeEntries(local, remote);
    assert.deepEqual(local, localCopy);
    assert.deepEqual(remote, remoteCopy);
  });
});

// ---------------------------------------------------------------------------
// Unified monthly path: aggregateMonthly + currentLabel (replaces fetchTotals monthly)
// ---------------------------------------------------------------------------

describe("unified monthly path (aggregateMonthly + currentLabel)", () => {
  it("picks current month from aggregated daily entries", () => {
    const now = new Date(2026, 1, 22); // Feb 22, 2026
    const daily: UsageEntry[] = [
      mkEntry("2026-02-18", 1.0, 100),
      mkEntry("2026-02-19", 2.0, 200),
      mkEntry("2026-02-20", 3.0, 300),
    ];
    const monthly = aggregateMonthly(daily);
    const target = currentLabel("monthly", now);
    const match = monthly.find((m) => m.label === target);
    assert.ok(match);
    assert.equal(match.label, "2026-02");
    assert.equal(match.totalCost, 6.0);
    assert.equal(match.inputTokens, 600);
  });

  it("returns EMPTY when no entries match current month", () => {
    const now = new Date(2026, 2, 1); // March 2026
    const daily: UsageEntry[] = [
      mkEntry("2026-02-18", 1.0, 100),
      mkEntry("2026-02-19", 2.0, 200),
    ];
    const monthly = aggregateMonthly(daily);
    const target = currentLabel("monthly", now);
    const match = monthly.find((m) => m.label === target);
    assert.equal(match, undefined);
    const result = match ?? { ...EMPTY };
    assert.equal(result.totalCost, 0);
  });

  it("produces identical results whether data comes from single or multi sources", () => {
    const daily: UsageEntry[] = [
      mkEntry("2026-02-18", 1.0, 100),
      mkEntry("2026-02-19", 2.0, 200),
      mkEntry("2026-02-20", 3.0, 300),
    ];
    // Simulate single mode: aggregateMonthly on local entries
    const singleMonthly = aggregateMonthly(daily);

    // Simulate multi mode: mergeEntries then aggregateMonthly
    const local = daily.slice(0, 2);
    const remote = daily.slice(2);
    const merged = mergeEntries(local, remote);
    const multiMonthly = aggregateMonthly(merged);

    assert.equal(singleMonthly.length, multiMonthly.length);
    assert.equal(singleMonthly[0].label, multiMonthly[0].label);
    assert.equal(singleMonthly[0].totalCost, multiMonthly[0].totalCost);
    assert.equal(singleMonthly[0].totalTokens, multiMonthly[0].totalTokens);
  });
});

// ---------------------------------------------------------------------------
// fetchTotals signature: period param removed (compile-time verification)
// ---------------------------------------------------------------------------

describe("fetchTotals/fetchAllTotals signatures", () => {
  it("fetchTotals accepts only toolKey and extraArgs — no period param (@ts-expect-error guard)", async () => {
    // Type-level assertion: if a period parameter is ever reintroduced,
    // the @ts-expect-error lines below will start failing at compile time.
    // Wrapped in dead-code block so the functions don't actually shell out.
    const { fetchTotals, fetchAllTotals } = await import("../src/fetcher.js");
    assert.ok(typeof fetchTotals === "function");
    assert.ok(typeof fetchAllTotals === "function");

    if (false as boolean) {
      // @ts-expect-error too many arguments: period parameter must not be accepted
      void fetchTotals("openai" as keyof typeof TOOLS, "daily", []);
      // @ts-expect-error too many arguments: period parameter must not be accepted
      void fetchAllTotals("daily", []);
    }
  });

  it("pickCurrentEntry with daily period matches today", () => {
    // This verifies what fetchTotals now does internally:
    // parse daily raw → pickCurrentEntry(dailyRaw, "daily")
    const now = new Date(2026, 1, 22);
    const dailyRaw = [
      { date: "Feb 21, 2026", totalCost: 1, totalTokens: 100, inputTokens: 50, outputTokens: 50, cacheCreationTokens: 0, cacheReadTokens: 0 },
      { date: "Feb 22, 2026", totalCost: 2, totalTokens: 200, inputTokens: 100, outputTokens: 100, cacheCreationTokens: 0, cacheReadTokens: 0 },
    ];
    const result = pickCurrentEntry(dailyRaw, "daily", now);
    assert.equal(result.totalCost, 2);
    assert.equal(result.totalTokens, 200);
  });

  it("pickCurrentEntry with daily period returns EMPTY when no match", () => {
    const now = new Date(2026, 1, 22);
    const dailyRaw = [
      { date: "Feb 20, 2026", totalCost: 1, totalTokens: 100, inputTokens: 50, outputTokens: 50, cacheCreationTokens: 0, cacheReadTokens: 0 },
    ];
    const result = pickCurrentEntry(dailyRaw, "daily", now);
    assert.equal(result.totalCost, 0);
  });
});
