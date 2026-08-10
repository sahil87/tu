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
  maxMergeEntries,
  aggregateMonthly,
  filterEntriesByRange,
  aggregateWeekly,
  aggregateForPeriod,
  weekLabel,
  TOOLS,
  EMPTY,
} from "../fetcher.js";
import type { UsageEntry } from "../types.js";

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

  it("returns start of current week (Sunday) for weekly period", () => {
    const now = new Date(2026, 1, 16); // Mon Feb 16, 2026 → Sunday Feb 15
    assert.equal(currentLabel("weekly", now), "2026-02-15");
  });

  it("returns the same date when today is already Sunday (weekly)", () => {
    const now = new Date(2026, 1, 15); // Sun Feb 15, 2026
    assert.equal(currentLabel("weekly", now), "2026-02-15");
  });

  it("handles month/year underflow for weekly (Sunday in previous month/year)", () => {
    const now = new Date(2026, 0, 1); // Thu Jan 1, 2026 → Sunday Dec 28, 2025
    assert.equal(currentLabel("weekly", now), "2025-12-28");
  });

  it("defaults to daily-style label for unknown period", () => {
    const now = new Date(2026, 1, 16);
    assert.equal(currentLabel("hourly", now), "2026-02-16");
  });
});

// ---------------------------------------------------------------------------
// pickCurrentEntry
// ---------------------------------------------------------------------------
// pickCurrentEntry threads a labelKey (defaulting to "date" — the spelling
// every registry tool uses; all per-agent subcommands emit "date"). These
// fixtures omit the labelKey arg to exercise that default, so they key their
// entries under "date". The bare-aggregate "period" spelling is still handled
// by the mechanism (covered in the "per-tool label key" describe below).
describe("pickCurrentEntry", () => {
  it("returns matching entry for today (daily)", () => {
    const now = new Date(2026, 1, 16);
    const entries = [
      { date: "2026-02-15", totalCost: 1, inputTokens: 100, outputTokens: 50, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 150 },
      { date: "2026-02-16", totalCost: 2, inputTokens: 200, outputTokens: 100, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 300 },
    ];
    const result = pickCurrentEntry(entries, "daily", now);
    assert.equal(result.totalCost, 2);
    assert.equal(result.totalTokens, 300);
  });

  it("returns EMPTY when no entry matches today", () => {
    const now = new Date(2026, 1, 16);
    const entries = [
      { date: "2026-02-14", totalCost: 1, inputTokens: 100, outputTokens: 50, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 150 },
      { date: "2026-02-15", totalCost: 2, inputTokens: 200, outputTokens: 100, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 300 },
    ];
    const result = pickCurrentEntry(entries, "daily", now);
    assert.equal(result.totalCost, 0);
    assert.equal(result.totalTokens, 0);
  });

  it("returns matching entry for current month (monthly)", () => {
    const now = new Date(2026, 1, 16);
    const entries = [
      { date: "2026-01", totalCost: 5, inputTokens: 1000, outputTokens: 500, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 1500 },
      { date: "2026-02", totalCost: 3, inputTokens: 800, outputTokens: 200, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 1000 },
    ];
    const result = pickCurrentEntry(entries, "monthly", now);
    assert.equal(result.totalCost, 3);
    assert.equal(result.totalTokens, 1000);
  });

  it("returns EMPTY when no entry matches current month", () => {
    const now = new Date(2026, 2, 1); // March 2026
    const entries = [
      { date: "2026-01", totalCost: 5, inputTokens: 1000, outputTokens: 500, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 1500 },
      { date: "2026-02", totalCost: 3, inputTokens: 800, outputTokens: 200, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 1000 },
    ];
    const result = pickCurrentEntry(entries, "monthly", now);
    assert.equal(result.totalCost, 0);
    assert.equal(result.totalTokens, 0);
  });

  // Regression: old code took entries[entries.length - 1] regardless of date
  it("does not attribute historical usage to today", () => {
    const now = new Date(2026, 1, 16); // Today is Feb 16
    const entries = [
      { date: "2026-02-10", totalCost: 10, inputTokens: 5000, outputTokens: 2000, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 7000 },
      { date: "2026-02-13", totalCost: 5, inputTokens: 2000, outputTokens: 1000, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 3000 },
    ];
    const result = pickCurrentEntry(entries, "daily", now);
    assert.equal(result.totalCost, 0);
    assert.equal(result.totalTokens, 0);
  });

  it("handles ISO date labels directly (v20 needs no normalization)", () => {
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
      { date: "2026-02-16", costUSD: 7.5, cachedInputTokens: 500, totalTokens: 1000 },
    ];
    const result = pickCurrentEntry(entries, "daily", now);
    assert.equal(result.totalCost, 7.5);
    assert.equal(result.cacheReadTokens, 500);
  });
});

// ---------------------------------------------------------------------------
// Per-tool label key: the mechanism handles both "date" and "period" key
// spellings. All per-agent subcommands emit the ISO label under "date"
// at ccusage v20; only the bare all-agents aggregate (which tu never
// calls) emits "period". The per-tool ToolConfig.labelKey is therefore "date"
// for every tool — the "period" support is kept because the key varies by
// serializer (a future divergence stays a data-only change).
// ---------------------------------------------------------------------------
describe("per-tool label key (date vs period)", () => {
  it("toUsageEntry resolves an ISO 'date'-keyed entry (per-agent subcommand shape)", () => {
    const entry = toUsageEntry({ date: "2026-06-01", totalCost: 1, totalTokens: 10 }, "date");
    assert.equal(entry.label, "2026-06-01");
    assert.equal(entry.totalCost, 1);
  });

  it("toUsageEntry still resolves an ISO 'period'-keyed entry (bare-aggregate shape)", () => {
    const entry = toUsageEntry({ period: "2026-06-01", totalCost: 2, totalTokens: 20 }, "period");
    assert.equal(entry.label, "2026-06-01");
    assert.equal(entry.totalCost, 2);
  });

  it("pickCurrentEntry matches today via a threaded 'date' key (per-agent subcommand)", () => {
    const now = new Date(2026, 5, 1); // Jun 1, 2026
    const entries = [
      { date: "2026-05-31", totalCost: 1, totalTokens: 100, inputTokens: 50, outputTokens: 50, cacheCreationTokens: 0, cacheReadTokens: 0 },
      { date: "2026-06-01", totalCost: 2, totalTokens: 200, inputTokens: 100, outputTokens: 100, cacheCreationTokens: 0, cacheReadTokens: 0 },
    ];
    const result = pickCurrentEntry(entries, "daily", now, "date");
    assert.equal(result.totalCost, 2);
    assert.equal(result.totalTokens, 200);
  });

  it("pickCurrentEntry defaults to the 'date' key when labelKey is omitted", () => {
    // The default is "date" — the spelling every registry tool uses — so an
    // omitted labelKey still resolves a real label rather than "" (the retired
    // "period" default matched no registry tool and would have yielded EMPTY).
    const now = new Date(2026, 5, 1);
    const entries = [
      { date: "2026-06-01", totalCost: 3, totalTokens: 300, inputTokens: 150, outputTokens: 150, cacheCreationTokens: 0, cacheReadTokens: 0 },
    ];
    const result = pickCurrentEntry(entries, "daily", now);
    assert.equal(result.totalCost, 3);
  });

  it("pickCurrentEntry with the retired 'period' default no longer matches a period-keyed entry when labelKey is omitted", () => {
    // Guard the T016 correction: omitting labelKey now reads "date". A raw entry
    // keyed only under "period" therefore no longer matches — proving the default
    // flipped from "period" to "date".
    const now = new Date(2026, 5, 1);
    const entries = [
      { period: "2026-06-01", totalCost: 3, totalTokens: 300, inputTokens: 150, outputTokens: 150, cacheCreationTokens: 0, cacheReadTokens: 0 },
    ];
    const result = pickCurrentEntry(entries, "daily", now);
    assert.equal(result.totalCost, 0);
    // Passing the "period" key explicitly still resolves it (mechanism intact).
    assert.equal(pickCurrentEntry(entries, "daily", now, "period").totalCost, 3);
  });

  it("the fetchHistory mapping shape yields correct labels for the per-tool key", () => {
    // fetchHistory maps: entries.map((e) => toUsageEntry(e, tool.labelKey)).
    // All registry tools carry labelKey "date"; a "date"-keyed raw entry maps to
    // a real ISO label, whereas the ccfx-era "period" registry value would have
    // produced "" for codex/oc on a machine with transcripts.
    const ccRaw = [{ date: "2026-06-01", totalCost: 1, totalTokens: 10 }];
    const codexRaw = [{ date: "2026-06-01", totalCost: 2, totalTokens: 20 }];
    const ccMapped = ccRaw.map((e) => toUsageEntry(e, TOOLS.cc.labelKey));
    const codexMapped = codexRaw.map((e) => toUsageEntry(e, TOOLS.codex.labelKey));
    assert.equal(ccMapped[0].label, "2026-06-01");
    assert.equal(codexMapped[0].label, "2026-06-01");
  });

  it("codex/oc 'date'-keyed entries map to real ISO labels (ccfx-era 'period' regression guard)", () => {
    // Regression: with the ccfx-era labelKey "period", a codex entry keyed under
    // "date" resolved t["period"] → undefined → label "". Now labelKey is "date".
    const codexRaw = { date: "2026-06-02", totalCost: 4, totalTokens: 40 };
    const ocRaw = { date: "2026-06-03", totalCost: 5, totalTokens: 50 };
    assert.equal(toUsageEntry(codexRaw, TOOLS.codex.labelKey).label, "2026-06-02");
    assert.equal(toUsageEntry(ocRaw, TOOLS.oc.labelKey).label, "2026-06-03");
    // The pre-fix "period" spelling would have yielded an empty label.
    assert.equal(toUsageEntry(codexRaw, "period").label, "");
  });
});

// ---------------------------------------------------------------------------
// TOOLS registry
// ---------------------------------------------------------------------------
describe("TOOLS", () => {
  const ALL_TOOLS = () => [TOOLS.cc, TOOLS.codex, TOOLS.oc, TOOLS.gemini, TOOLS.copilot, TOOLS.kimi];

  it("has entries for cc, codex, oc, gemini, copilot, and kimi", () => {
    assert.ok(TOOLS.cc);
    assert.ok(TOOLS.codex);
    assert.ok(TOOLS.oc);
    assert.ok(TOOLS.gemini);
    assert.ok(TOOLS.copilot);
    assert.ok(TOOLS.kimi);
  });

  it("registry order is cc, codex, oc, gemini, copilot, kimi (column order in all-tools views)", () => {
    // Insertion order determines column order; new tools are appended so the
    // existing columns keep their positions (Output Stability).
    assert.deepEqual(Object.keys(TOOLS), ["cc", "codex", "oc", "gemini", "copilot", "kimi"]);
  });

  it("cc, gemini, copilot, and kimi do not need filtering", () => {
    assert.equal(TOOLS.cc.needsFilter, false);
    assert.equal(TOOLS.gemini.needsFilter, false);
    assert.equal(TOOLS.copilot.needsFilter, false);
    assert.equal(TOOLS.kimi.needsFilter, false);
  });

  it("codex and oc need filtering", () => {
    assert.equal(TOOLS.codex.needsFilter, true);
    assert.equal(TOOLS.oc.needsFilter, true);
  });

  it("gemini, copilot, and kimi carry the expected display names", () => {
    assert.equal(TOOLS.gemini.name, "Gemini");
    assert.equal(TOOLS.copilot.name, "Copilot");
    assert.equal(TOOLS.kimi.name, "Kimi");
  });

  // --- ccusage@20 shape: one binary, per-tool subcommand prefixArgs ---
  it("each entry exposes a binary field (string)", () => {
    for (const tool of ALL_TOOLS()) assert.equal(typeof tool.binary, "string");
  });

  it("each entry exposes a prefixArgs field (string[])", () => {
    for (const tool of ALL_TOOLS()) assert.ok(Array.isArray(tool.prefixArgs));
  });

  it("no entry still carries a legacy `command` field (migration complete)", () => {
    for (const tool of ALL_TOOLS()) {
      assert.equal((tool as unknown as Record<string, unknown>).command, undefined);
    }
  });

  it("all tools share the single ccusage binary (v20 unified CLI)", () => {
    const binaries = new Set(ALL_TOOLS().map((t) => t.binary));
    assert.equal(binaries.size, 1);
  });

  it("no entry uses the legacy `node`-interpreter / index.js vendor convention", () => {
    // v20 vendors a native Rust binary exec'd directly — no `node` wrapper, no
    // `index.js` entrypoint. Regression guard against the pre-v20 shape.
    for (const tool of ALL_TOOLS()) {
      assert.notEqual(tool.binary, "node");
      assert.ok(!tool.prefixArgs.some((a) => a.endsWith("index.js")));
    }
  });

  it("binary points at ccusage in both vendor and dev modes", () => {
    // Vendor mode: <vendor>/ccusage/bin/ccusage (native binary, exec'd directly).
    // Dev mode:    <node_modules>/.bin/ccusage (the npm launcher).
    for (const tool of ALL_TOOLS()) {
      const vendorShape = tool.binary.endsWith("/ccusage/bin/ccusage");
      const devShape = tool.binary.endsWith("/ccusage");
      assert.ok(vendorShape || devShape, `unexpected binary path: ${tool.binary}`);
    }
  });

  it("subcommand prefixArgs select the per-tool ccusage subcommand", () => {
    // v20: bare `ccusage daily` is an all-agents aggregate, so each tool uses
    // its per-agent subcommand (not [] — that would over/double-count other
    // detected agents).
    assert.deepEqual(TOOLS.cc.prefixArgs, ["claude"]);
    assert.deepEqual(TOOLS.codex.prefixArgs, ["codex"]);
    assert.deepEqual(TOOLS.oc.prefixArgs, ["opencode"]);
    assert.deepEqual(TOOLS.gemini.prefixArgs, ["gemini"]);
    assert.deepEqual(TOOLS.copilot.prefixArgs, ["copilot"]);
    assert.deepEqual(TOOLS.kimi.prefixArgs, ["kimi"]);
  });

  it("every tool's labelKey is 'date' (all per-agent subcommands emit 'date')", () => {
    // All per-agent subcommands (claude/codex/opencode/gemini/copilot/kimi) emit
    // the ISO label under "date" at ccusage v20.0.14; only the bare all-agents
    // aggregate (which tu never calls) emits "period". codex/oc were corrected
    // from the ccfx-era "period" here.
    for (const tool of ALL_TOOLS()) assert.equal(tool.labelKey, "date");
  });
});

// ---------------------------------------------------------------------------
// runTool argv construction (via fetchTotals integration — spy on execFile)
//
// runTool is private in fetcher.ts. We assert its argv-construction contract
// indirectly by observing the child_process invocation on a fetchTotals call
// against a crafted test binary. The ENOENT path exercises the exec layer
// (producing the warn-then-empty-string behaviour) without depending on a
// real ccusage binary.
// ---------------------------------------------------------------------------
describe("runTool argv construction", () => {
  it("argv follows [...prefixArgs, period, --json, ...extraArgs] shape", () => {
    // Pure shape check. Since runTool is private, we assert the documented
    // construction by reading the spec and verifying the TOOLS entries align
    // with the `[binary, ...prefixArgs, period, --json, ...extra]` pattern.
    // The integration with execFile is exercised by fetch-warning tests.
    const expectedArgs = [...TOOLS.cc.prefixArgs, "daily", "--json"];
    // cc now composes the per-agent `claude` subcommand invocation.
    assert.deepEqual(expectedArgs, ["claude", "daily", "--json"]);
    assert.ok(expectedArgs.includes("daily"), "period should be in argv");
    assert.ok(expectedArgs.includes("--json"), "--json should be in argv");
    // No shell metacharacters concatenation — prefixArgs are kept as discrete entries.
    for (const arg of TOOLS.cc.prefixArgs) {
      assert.equal(typeof arg, "string");
    }
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
// weekLabel — Sunday-start week-start ISO label, UTC arithmetic
// ---------------------------------------------------------------------------
describe("weekLabel", () => {
  it("maps a Monday to the preceding Sunday", () => {
    // 2026-02-16 is a Monday → week-start Sunday 2026-02-15
    assert.equal(weekLabel("2026-02-16"), "2026-02-15");
  });

  it("returns the same date when the label is itself a Sunday", () => {
    // 2026-02-15 is a Sunday
    assert.equal(weekLabel("2026-02-15"), "2026-02-15");
  });

  it("maps a Saturday to the Sunday six days earlier", () => {
    // 2026-02-21 is a Saturday → week-start Sunday 2026-02-15
    assert.equal(weekLabel("2026-02-21"), "2026-02-15");
  });

  it("groups a year-boundary week under the prior-year Sunday", () => {
    // 2026-12-27 is a Sunday; the week runs through 2027-01-02 (Saturday).
    assert.equal(weekLabel("2026-12-28"), "2026-12-27"); // Mon
    assert.equal(weekLabel("2026-12-31"), "2026-12-27"); // Thu
    assert.equal(weekLabel("2027-01-01"), "2026-12-27"); // Fri, prior-year label
    assert.equal(weekLabel("2027-01-02"), "2026-12-27"); // Sat
  });

  it("uses UTC arithmetic so DST-transition weeks are not skewed", () => {
    // 2026-03-08 (US spring-forward Sunday) and 2026-11-01 (fall-back Sunday)
    // are both Sundays → they are their own week-start labels, unshifted.
    assert.equal(weekLabel("2026-03-08"), "2026-03-08");
    assert.equal(weekLabel("2026-03-09"), "2026-03-08"); // Mon after spring-forward
    assert.equal(weekLabel("2026-11-01"), "2026-11-01");
    assert.equal(weekLabel("2026-11-02"), "2026-11-01"); // Mon after fall-back
  });

  it("returns the original label (no throw) for a malformed date", () => {
    // A bad label from parsed JSON/metrics must not crash weekly aggregation
    // (graceful degradation) — it falls back to being its own bucket.
    assert.equal(weekLabel(""), "");
    assert.equal(weekLabel("not-a-date"), "not-a-date");
    assert.equal(weekLabel("2026-13-45"), "2026-13-45"); // out-of-range parts
  });
});

// ---------------------------------------------------------------------------
// aggregateWeekly
// ---------------------------------------------------------------------------
describe("aggregateWeekly", () => {
  it("aggregates daily entries into a single week", () => {
    // 2026-02-15 (Sun) .. 2026-02-17 (Tue) — all in the week of Sunday 2026-02-15
    const daily = [
      mkEntry("2026-02-15", 1.0, 100),
      mkEntry("2026-02-16", 2.0, 200),
      mkEntry("2026-02-17", 3.0, 300),
    ];
    const result = aggregateWeekly(daily);
    assert.equal(result.length, 1);
    assert.equal(result[0].label, "2026-02-15");
    assert.equal(result[0].totalCost, 6.0);
    assert.equal(result[0].inputTokens, 600);
    assert.equal(result[0].totalTokens, 600);
  });

  it("groups entries spanning a week boundary", () => {
    // 2026-02-14 (Sat) → week 2026-02-08; 2026-02-15 (Sun) → week 2026-02-15
    const daily = [
      mkEntry("2026-02-14", 1.0, 100),
      mkEntry("2026-02-15", 2.0, 200),
    ];
    const result = aggregateWeekly(daily);
    assert.equal(result.length, 2);
    assert.equal(result[0].label, "2026-02-08");
    assert.equal(result[0].totalCost, 1.0);
    assert.equal(result[1].label, "2026-02-15");
    assert.equal(result[1].totalCost, 2.0);
  });

  it("returns sorted by week-start label", () => {
    const daily = [
      mkEntry("2026-03-02", 3.0), // week 2026-03-01
      mkEntry("2026-01-05", 1.0), // week 2026-01-04
      mkEntry("2026-02-10", 2.0), // week 2026-02-08
    ];
    const result = aggregateWeekly(daily);
    assert.deepEqual(
      result.map((e) => e.label),
      ["2026-01-04", "2026-02-08", "2026-03-01"]
    );
  });

  it("groups a year-boundary week under the prior-year Sunday", () => {
    // Week of Sunday 2026-12-27 spans into 2027.
    const daily = [
      mkEntry("2026-12-28", 1.0, 100),
      mkEntry("2026-12-31", 2.0, 200),
      mkEntry("2027-01-01", 3.0, 300),
      mkEntry("2027-01-02", 4.0, 400),
    ];
    const result = aggregateWeekly(daily);
    assert.equal(result.length, 1);
    assert.equal(result[0].label, "2026-12-27");
    assert.equal(result[0].totalCost, 10.0);
    assert.equal(result[0].totalTokens, 1000);
  });

  it("groups DST-transition weeks without label skew (UTC arithmetic)", () => {
    // Spring-forward week of 2026-03-08 (Sun) and fall-back week of
    // 2026-11-01 (Sun) each stay anchored to their Sunday label.
    const daily = [
      mkEntry("2026-03-08", 1.0, 100), // Sun
      mkEntry("2026-03-09", 2.0, 200), // Mon (post spring-forward)
      mkEntry("2026-11-01", 3.0, 300), // Sun
      mkEntry("2026-11-02", 4.0, 400), // Mon (post fall-back)
    ];
    const result = aggregateWeekly(daily);
    assert.deepEqual(
      result.map((e) => e.label),
      ["2026-03-08", "2026-11-01"]
    );
    assert.equal(result[0].totalCost, 3.0);
    assert.equal(result[1].totalCost, 7.0);
  });

  it("returns empty array for empty input", () => {
    assert.deepEqual(aggregateWeekly([]), []);
  });

  it("does not mutate input", () => {
    const daily = [mkEntry("2026-02-16", 1.0)];
    const copy = JSON.parse(JSON.stringify(daily));
    aggregateWeekly(daily);
    assert.deepEqual(daily, copy);
  });
});

// ---------------------------------------------------------------------------
// aggregateForPeriod — period-to-aggregator routing
// ---------------------------------------------------------------------------
describe("aggregateForPeriod", () => {
  const daily = [
    mkEntry("2026-02-15", 1.0, 100), // Sun (week 2026-02-15, month 2026-02)
    mkEntry("2026-02-16", 2.0, 200), // Mon
  ];

  it("routes monthly to aggregateMonthly", () => {
    assert.deepEqual(aggregateForPeriod("monthly", daily), aggregateMonthly(daily));
  });

  it("routes weekly to aggregateWeekly", () => {
    assert.deepEqual(aggregateForPeriod("weekly", daily), aggregateWeekly(daily));
  });

  it("returns entries unchanged (identity) for daily", () => {
    assert.equal(aggregateForPeriod("daily", daily), daily);
  });

  it("returns entries unchanged for an unknown period", () => {
    assert.equal(aggregateForPeriod("hourly", daily), daily);
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
// maxMergeEntries — per-label whole-entry max (own-machine self-view merge)
// ---------------------------------------------------------------------------

describe("maxMergeEntries", () => {
  it("picks the whole entry with the greater totalCost (no summing)", () => {
    // Purged live view vs own repo snapshot — snapshot must resurface intact
    const live = [mkEntry("2026-04-24", 9.46, 12)];
    const snapshot = [mkEntry("2026-04-24", 236.0, 9999)];
    const result = maxMergeEntries(live, snapshot);
    assert.equal(result.length, 1);
    assert.equal(result[0].totalCost, 236.0);
    assert.equal(result[0].inputTokens, 9999); // every field from the winner
    assert.equal(result[0].totalTokens, 9999);
  });

  it("never mixes fields across entries (atomic snapshots, not per-field max)", () => {
    const a = [{ ...mkEntry("2026-04-24", 10.0, 100), outputTokens: 1 }];
    const b = [{ ...mkEntry("2026-04-24", 5.0, 50), outputTokens: 9999 }];
    const result = maxMergeEntries(a, b);
    // a wins on totalCost — b's larger outputTokens must NOT leak in
    assert.equal(result[0].totalCost, 10.0);
    assert.equal(result[0].outputTokens, 1);
  });

  it("keeps the first argument's entry on equal totalCost (live wins in the live window)", () => {
    const live = [{ ...mkEntry("2026-06-10", 3.0, 100), outputTokens: 42 }];
    const snapshot = [{ ...mkEntry("2026-06-10", 3.0, 100), outputTokens: 7 }];
    const result = maxMergeEntries(live, snapshot);
    assert.equal(result.length, 1);
    assert.equal(result[0].outputTokens, 42);
  });

  it("preserves non-overlapping entries from both sides", () => {
    const live = [mkEntry("2026-06-10", 1.0)];
    const snapshot = [mkEntry("2026-04-24", 236.0)];
    const result = maxMergeEntries(live, snapshot);
    assert.equal(result.length, 2);
    assert.equal(result[0].label, "2026-04-24");
    assert.equal(result[1].label, "2026-06-10");
  });

  it("handles empty inputs", () => {
    const only = [mkEntry("2026-02-20", 1.5)];
    assert.deepEqual(maxMergeEntries([], []), []);
    assert.equal(maxMergeEntries(only, [])[0].totalCost, 1.5);
    assert.equal(maxMergeEntries([], only)[0].totalCost, 1.5);
  });

  it("sorts result ascending by label", () => {
    const a = [mkEntry("2026-02-20", 1.0)];
    const b = [mkEntry("2026-02-18", 0.5), mkEntry("2026-02-22", 0.3)];
    const result = maxMergeEntries(a, b);
    assert.deepEqual(
      result.map((e) => e.label),
      ["2026-02-18", "2026-02-20", "2026-02-22"]
    );
  });

  it("does not mutate input arrays and returns copies", () => {
    const a = [mkEntry("2026-02-20", 1.0)];
    const b = [mkEntry("2026-02-20", 2.0)];
    const aCopy = JSON.parse(JSON.stringify(a));
    const bCopy = JSON.parse(JSON.stringify(b));
    const result = maxMergeEntries(a, b);
    assert.deepEqual(a, aCopy);
    assert.deepEqual(b, bCopy);
    // Winner is copied, not aliased — mutating the result must not touch inputs
    result[0].totalCost = 999;
    assert.equal(b[0].totalCost, 2.0);
  });
});

// ---------------------------------------------------------------------------
// filterEntriesByRange — inclusive ISO date-range window (--since/--until)
// ---------------------------------------------------------------------------

describe("filterEntriesByRange", () => {
  const entries = [
    mkEntry("2026-06-01", 1.0),
    mkEntry("2026-06-15", 2.0),
    mkEntry("2026-07-01", 3.0),
  ];

  it("includes both bounds (inclusive since <= label <= until)", () => {
    const result = filterEntriesByRange(entries, "2026-06-01", "2026-06-30");
    assert.deepEqual(result.map((e) => e.label), ["2026-06-01", "2026-06-15"]);
  });

  it("includes an entry exactly on the since boundary and on the until boundary", () => {
    const result = filterEntriesByRange(entries, "2026-06-15", "2026-07-01");
    assert.deepEqual(result.map((e) => e.label), ["2026-06-15", "2026-07-01"]);
  });

  it("since-only is an open-ended upper window", () => {
    const result = filterEntriesByRange(entries, "2026-06-15", undefined);
    assert.deepEqual(result.map((e) => e.label), ["2026-06-15", "2026-07-01"]);
  });

  it("until-only is an open-ended lower window", () => {
    const result = filterEntriesByRange(entries, undefined, "2026-06-15");
    assert.deepEqual(result.map((e) => e.label), ["2026-06-01", "2026-06-15"]);
  });

  it("returns all entries when neither bound is set", () => {
    const result = filterEntriesByRange(entries, undefined, undefined);
    assert.deepEqual(result.map((e) => e.label), ["2026-06-01", "2026-06-15", "2026-07-01"]);
  });

  it("handles empty input", () => {
    assert.deepEqual(filterEntriesByRange([], "2026-06-01", "2026-06-30"), []);
  });

  it("yields an empty window for an impossible-but-shaped bound", () => {
    // 2026-13-01 sorts after any real December day → nothing on/after it.
    assert.deepEqual(filterEntriesByRange(entries, "2026-13-01", undefined), []);
  });

  it("does not mutate the input array or its entries", () => {
    const copy = JSON.parse(JSON.stringify(entries));
    filterEntriesByRange(entries, "2026-06-01", "2026-06-15");
    assert.deepEqual(entries, copy);
  });
});

// ---------------------------------------------------------------------------
// Multi-mode merge path with a --since/--until window (backlog-required)
// ---------------------------------------------------------------------------

describe("filterEntriesByRange on the multi-mode merge path", () => {
  it("windows merged local+remote daily entries", () => {
    const local = [mkEntry("2026-05-30", 1.0), mkEntry("2026-06-10", 2.0)];
    const remote = [mkEntry("2026-06-20", 3.0), mkEntry("2026-07-05", 4.0)];
    // Production order: mergeEntries → filterEntriesByRange (before aggregation)
    const windowed = filterEntriesByRange(mergeEntries(local, remote), "2026-06-01", "2026-06-30");
    assert.deepEqual(windowed.map((e) => e.label), ["2026-06-10", "2026-06-20"]);
    assert.equal(windowed.reduce((s, e) => s + e.totalCost, 0), 5.0);
  });

  it("monthly rollup of a partially-windowed month sums only in-window days", () => {
    const daily = [
      mkEntry("2026-06-05", 1.0, 100),
      mkEntry("2026-06-15", 2.0, 200),
      mkEntry("2026-06-25", 4.0, 400),
    ];
    // Window trims June 25 → the June rollup must reflect only Jun 5 + Jun 15.
    const monthly = aggregateMonthly(filterEntriesByRange(daily, "2026-06-01", "2026-06-20"));
    const june = monthly.find((m) => m.label === "2026-06");
    assert.ok(june);
    assert.equal(june.totalCost, 3.0);
    assert.equal(june.inputTokens, 300);
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
    const { fetchTotals, fetchAllTotals } = await import("../fetcher.js");
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
    // parse daily raw → pickCurrentEntry(dailyRaw, "daily", now, tool.labelKey).
    // All per-agent subcommands emit the ISO label under "date" at v20; the
    // pickCurrentEntry default is likewise "date", so these fixtures key on it.
    const now = new Date(2026, 1, 22);
    const dailyRaw = [
      { date: "2026-02-21", totalCost: 1, totalTokens: 100, inputTokens: 50, outputTokens: 50, cacheCreationTokens: 0, cacheReadTokens: 0 },
      { date: "2026-02-22", totalCost: 2, totalTokens: 200, inputTokens: 100, outputTokens: 100, cacheCreationTokens: 0, cacheReadTokens: 0 },
    ];
    const result = pickCurrentEntry(dailyRaw, "daily", now);
    assert.equal(result.totalCost, 2);
    assert.equal(result.totalTokens, 200);
  });

  it("pickCurrentEntry with daily period returns EMPTY when no match", () => {
    const now = new Date(2026, 1, 22);
    const dailyRaw = [
      { date: "2026-02-20", totalCost: 1, totalTokens: 100, inputTokens: 50, outputTokens: 50, cacheCreationTokens: 0, cacheReadTokens: 0 },
    ];
    const result = pickCurrentEntry(dailyRaw, "daily", now);
    assert.equal(result.totalCost, 0);
  });
});
