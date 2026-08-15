import { describe, it, afterEach } from "node:test";
import assert from "node:assert/strict";

import {
  percentile,
  computeBarScale,
  renderScaledBar,
  renderBar,
  renderHistory,
  renderTotalHistory,
  fmtCost,
} from "../formatter.js";
import { setNoColor, stripAnsi } from "../colors.js";
import { currentLabel } from "../../core/fetcher.js";
import type { UsageEntry } from "../../core/types.js";

// Color assertions in this file need deterministic ANSI state regardless of the
// invoking shell's environment (each test file runs in its own process).
delete process.env.NO_COLOR;
setNoColor(true);
afterEach(() => setNoColor(true));

function entry(label: string, totalCost: number): UsageEntry {
  return { label, totalCost, inputTokens: 100, outputTokens: 50, cacheCreationTokens: 10, cacheReadTokens: 5, totalTokens: 165 };
}

const FULL_BLOCK = "█";
const RULE = "┊";

// Divider lines are dim-wrapped ─ runs (header, month separators, total divider)
function isDivider(line: string): boolean {
  return stripAnsi(line).startsWith("─");
}

// ---------------------------------------------------------------------------
// percentile
// ---------------------------------------------------------------------------
describe("percentile", () => {
  it("returns 0 for an empty sample", () => {
    assert.equal(percentile([], 95), 0);
  });

  it("returns the single value for a one-element sample", () => {
    assert.equal(percentile([42], 95), 42);
  });

  it("interpolates linearly between ranks", () => {
    // idx = 0.95 * 3 = 2.85 → 30 + 0.85 * (40 - 30)
    assert.equal(percentile([10, 20, 30, 40], 95), 38.5);
    // idx = 0.5 * 3 = 1.5 → 20 + 0.5 * (30 - 20)
    assert.equal(percentile([10, 20, 30, 40], 50), 25);
  });

  it("clamps to the sample ends at p0/p100", () => {
    assert.equal(percentile([10, 20, 30], 0), 10);
    assert.equal(percentile([10, 20, 30], 100), 30);
  });
});

// ---------------------------------------------------------------------------
// computeBarScale
// ---------------------------------------------------------------------------
describe("computeBarScale", () => {
  it("returns single mode when there are no nonzero costs", () => {
    assert.deepEqual(computeBarScale([0, 0, 0], 30), { mode: "single", max: 0 });
  });

  it("returns single mode when max ≤ 1.5 × p95 (well-behaved window)", () => {
    const scale = computeBarScale([100, 200, 300], 30);
    assert.deepEqual(scale, { mode: "single", max: 300 });
  });

  it("returns two-zone mode when an outlier dominates", () => {
    const costs = [...Array(21).keys()].map((i) => 100 + i * 10).concat([1091.67, 4031.61]);
    const scale = computeBarScale(costs, 30);
    assert.equal(scale.mode, "two-zone");
    if (scale.mode !== "two-zone") return;
    // idx = 0.95 * 22 = 20.9 → 300 + 0.9 * (1091.67 - 300)
    assert.ok(Math.abs(scale.p95 - 1012.503) < 1e-9, `p95 was ${scale.p95}`);
    assert.equal(scale.max, 4031.61);
    assert.equal(scale.overflowZone, 8); // max(4, round(30/4))
    assert.equal(scale.mainZone, 21); // 30 - 8 - 1
  });

  it("keeps both zones legible at the minimum bar width", () => {
    const costs = [...Array(21).keys()].map((i) => 100 + i * 10).concat([1091.67, 4031.61]);
    const scale = computeBarScale(costs, 10);
    assert.equal(scale.mode, "two-zone");
    if (scale.mode !== "two-zone") return;
    assert.equal(scale.overflowZone, 4); // floor
    assert.equal(scale.mainZone, 5);
  });
});

// ---------------------------------------------------------------------------
// renderScaledBar
// ---------------------------------------------------------------------------
describe("renderScaledBar", () => {
  const costs = [...Array(21).keys()].map((i) => 100 + i * 10).concat([1091.67, 4031.61]);
  const scale = computeBarScale(costs, 30);
  assert.equal(scale.mode, "two-zone");

  it("single mode delegates to renderBar byte-identically", () => {
    const single = computeBarScale([100, 200, 300], 30);
    assert.equal(renderScaledBar(0, single, 30), "");
    assert.equal(renderScaledBar(200, single, 30), " " + renderBar(200, 300, 30));
  });

  it("two-zone output is exactly barWidth visible chars in every row", () => {
    for (const value of [0, 100, 300, scale.mode === "two-zone" ? scale.p95 : 0, 1091.67, 4031.61]) {
      assert.equal(stripAnsi(renderScaledBar(value, scale, 30)).length, 31); // leading space + 30
    }
  });

  it("renders the rule in every row, including zero-cost rows", () => {
    const bar = stripAnsi(renderScaledBar(0, scale, 30));
    assert.equal(bar.indexOf(RULE), 22); // leading space + 21-char main zone
  });

  it("a row at exactly p95 ends at the rule with no overflow segment", () => {
    if (scale.mode !== "two-zone") return;
    const bar = stripAnsi(renderScaledBar(scale.p95, scale, 30));
    assert.equal(bar.slice(1, 22), FULL_BLOCK.repeat(21));
    assert.equal(bar.split(RULE)[1].trim(), "");
  });

  it("renders the overflow segment in yellow and the main zone in green", () => {
    setNoColor(false);
    const bar = renderScaledBar(4031.61, scale, 30);
    assert.ok(bar.includes("\x1b[32m"), "main zone should be green");
    assert.ok(bar.includes("\x1b[33m"), "overflow zone should be yellow");
    assert.ok(bar.includes("\x1b[2m"), "rule should be dim");
  });

  it("distinguishes overflow magnitudes ($1,091.67 vs $4,031.61)", () => {
    const small = stripAnsi(renderScaledBar(1091.67, scale, 30)).split(RULE)[1].trim();
    const large = stripAnsi(renderScaledBar(4031.61, scale, 30)).split(RULE)[1].trim();
    assert.ok(small.length > 0, "1,091.67 should cross the rule");
    assert.ok(large.length > small.length, "4,031.61 should render a longer overflow segment");
  });
});

// ---------------------------------------------------------------------------
// Month-boundary separators (daily period only)
// ---------------------------------------------------------------------------
describe("month-boundary separators", () => {
  const crossMonth = [
    entry("2026-06-28", 100),
    entry("2026-06-29", 100),
    entry("2026-06-30", 100),
    entry("2026-07-01", 100),
    entry("2026-07-02", 100),
  ];

  it("renderHistory emits exactly one separator, right before the month-crossing row", () => {
    const lines = renderHistory("Claude Code", "daily", crossMonth, 200);
    const dividers = lines.filter(isDivider);
    assert.equal(dividers.length, 3, "header + 1 month separator + total divider");
    const julyIdx = lines.findIndex((l) => l.includes("2026-07-01"));
    assert.ok(isDivider(lines[julyIdx - 1]), "separator sits immediately before the 2026-07-01 row");
    const juneIdx = lines.findIndex((l) => l.includes("2026-06-29"));
    assert.ok(!isDivider(lines[juneIdx - 1]), "no separator between same-month rows");
  });

  it("renderHistory emits no separators for monthly periods", () => {
    const lines = renderHistory("Claude Code", "monthly", [entry("2026-06", 100), entry("2026-07", 100)], 200);
    assert.equal(lines.filter(isDivider).length, 2, "header + total divider only");
  });

  it("renderHistory emits no separators for a daily-labeled window under a non-daily period", () => {
    const lines = renderHistory("Claude Code", "weekly", crossMonth, 200);
    assert.equal(lines.filter(isDivider).length, 2, "header + total divider only");
  });

  it("separators reflect the post-maxRows window", () => {
    const lines = renderHistory("Claude Code", "daily", crossMonth, 200, { maxRows: 2 });
    assert.equal(lines.filter(isDivider).length, 2, "truncated window stays within July — no separator");
  });

  it("renderTotalHistory emits a separator before the month-crossing row", () => {
    const allToolEntries = new Map<string, UsageEntry[]>([
      ["Claude Code", crossMonth],
      ["Codex", crossMonth.map((e) => entry(e.label, 5))],
    ]);
    const lines = renderTotalHistory("daily", allToolEntries, 200);
    const dividers = lines.filter(isDivider);
    assert.equal(dividers.length, 3, "header + 1 month separator + total divider");
    const julyIdx = lines.findIndex((l) => l.includes("2026-07-01"));
    assert.ok(isDivider(lines[julyIdx - 1]));
  });

  it("renderTotalHistory emits no separators for monthly periods", () => {
    const allToolEntries = new Map<string, UsageEntry[]>([
      ["Claude Code", [entry("2026-06", 100), entry("2026-07", 100)]],
    ]);
    const lines = renderTotalHistory("monthly", allToolEntries, 200);
    assert.equal(lines.filter(isDivider).length, 2);
  });
});

// ---------------------------------------------------------------------------
// Current-period marker (boldWhite label cell)
// ---------------------------------------------------------------------------
describe("current-period marker", () => {
  it("renderHistory marks today's row boldWhite, others unmarked, width unchanged", () => {
    const today = currentLabel("daily");
    const entries = [entry("2020-01-01", 100), entry(today, 100)];
    setNoColor(false);
    const colored = renderHistory("Claude Code", "daily", entries, 200);
    setNoColor(true);
    const plain = renderHistory("Claude Code", "daily", entries, 200);

    const coloredToday = colored.find((l) => l.includes(today))!;
    const coloredOther = colored.find((l) => l.includes("2020-01-01"))!;
    assert.ok(coloredToday.includes("\x1b[1;37m"), "today's date cell should be boldWhite");
    assert.ok(!coloredOther.includes("\x1b[1;37m"), "other rows stay unmarked");
    assert.equal(stripAnsi(coloredToday), plain.find((l) => l.includes(today))!, "marking adds no width");
  });

  it("renderHistory marks the current month's row in monthly views", () => {
    const month = currentLabel("monthly");
    const entries = [entry("2020-01", 100), entry(month, 100)];
    setNoColor(false);
    const lines = renderHistory("Claude Code", "monthly", entries, 200);
    setNoColor(true);
    assert.ok(lines.find((l) => l.includes(month))!.includes("\x1b[1;37m"));
    assert.ok(!lines.find((l) => l.includes("2020-01"))!.includes("\x1b[1;37m"));
  });

  it("renderTotalHistory marks today's row boldWhite", () => {
    const today = currentLabel("daily");
    const allToolEntries = new Map<string, UsageEntry[]>([
      ["Claude Code", [entry("2020-01-01", 100), entry(today, 100)]],
    ]);
    setNoColor(false);
    const lines = renderTotalHistory("daily", allToolEntries, 200);
    setNoColor(true);
    assert.ok(lines.find((l) => l.includes(today))!.includes("\x1b[1;37m"));
    assert.ok(!lines.find((l) => l.includes("2020-01-01"))!.includes("\x1b[1;37m"));
  });
});

// ---------------------------------------------------------------------------
// Summary footer
// ---------------------------------------------------------------------------
describe("summary footer", () => {
  function footerLine(lines: string[]): string {
    const line = lines.map(stripAnsi).find((l) => l.startsWith("avg "));
    assert.ok(line, "footer line should exist");
    return line;
  }

  it("renders avg / this month / peak for daily windows with current-month rows", () => {
    const mp = currentLabel("monthly");
    const entries = [entry(`${mp}-01`, 100), entry(`${mp}-02`, 200)];
    const footer = footerLine(renderHistory("Claude Code", "daily", entries, 200));
    assert.equal(footer, `avg $150.00/day · this month $300.00 · peak $200.00 (${mp}-02)`);
  });

  it("omits 'this month' when the window has no current-month rows", () => {
    const entries = [entry("2020-01-01", 100), entry("2020-01-02", 300)];
    const footer = footerLine(renderHistory("Claude Code", "daily", entries, 200));
    assert.equal(footer, "avg $200.00/day · peak $300.00 (2020-01-02)");
  });

  it("uses the /month suffix and drops 'this month' for monthly periods", () => {
    const entries = [entry("2020-01", 100), entry("2020-02", 300)];
    const footer = footerLine(renderHistory("Claude Code", "monthly", entries, 200));
    assert.equal(footer, "avg $200.00/month · peak $300.00 (2020-02)");
  });

  it("omits the peak label parenthetical when all row costs are zero", () => {
    const entries = [entry("2020-01-01", 0), entry("2020-01-02", 0)];
    const footer = footerLine(renderHistory("Claude Code", "daily", entries, 200));
    assert.equal(footer, "avg $0.00/day · peak $0.00");
  });

  it("uses the /week suffix for weekly periods", () => {
    const entries = [entry("2026-08-09", 100), entry("2026-08-16", 200)];
    const footer = footerLine(renderHistory("Claude Code", "weekly", entries, 200));
    assert.ok(footer.startsWith("avg $150.00/week"));
  });

  it("renderTotalHistory footers over row totals", () => {
    const allToolEntries = new Map<string, UsageEntry[]>([
      ["Claude Code", [entry("2020-01-01", 100), entry("2020-01-02", 200)]],
      ["Codex", [entry("2020-01-01", 50), entry("2020-01-02", 100)]],
    ]);
    const footer = footerLine(renderTotalHistory("daily", allToolEntries, 200));
    assert.equal(footer, "avg $225.00/day · peak $300.00 (2020-01-02)");
  });

  it("is dim like the machine legend", () => {
    setNoColor(false);
    const lines = renderHistory("Claude Code", "daily", [entry("2020-01-01", 100), entry("2020-01-02", 300)], 200);
    setNoColor(true);
    const footer = lines.find((l) => stripAnsi(l).startsWith("avg "))!;
    assert.ok(footer.startsWith("\x1b[2m"));
  });

  it("is absent for single-row windows (matches the Total row condition)", () => {
    const lines = renderHistory("Claude Code", "daily", [entry("2020-01-01", 100)], 200);
    assert.ok(!lines.some((l) => stripAnsi(l).startsWith("avg ")));
  });
});

// ---------------------------------------------------------------------------
// Two-zone bar scale in history views
// ---------------------------------------------------------------------------
describe("two-zone bar scale in history views", () => {
  // 21 well-behaved rows (100…300) + outliers 1,091.67 and 4,031.61 → p95 = 1012.503
  const entries = [
    ...[...Array(21).keys()].map((i) => entry(`2026-06-${String(i + 1).padStart(2, "0")}`, 100 + i * 10)),
    entry("2026-06-22", 1091.67),
    entry("2026-06-23", 4031.61),
  ];

  it("aligns the rule column across every data row", () => {
    const lines = renderHistory("Claude Code", "daily", entries, 200);
    const dataLines = lines.filter((l) => /^2026-06-\d{2}/.test(stripAnsi(l)));
    assert.equal(dataLines.length, 23);
    const cols = new Set(dataLines.map((l) => stripAnsi(l).indexOf(RULE)));
    assert.equal(cols.size, 1, `rule column should be identical across rows, got ${[...cols]}`);
    assert.ok(!cols.has(-1), "every data row should carry the rule");
  });

  it("renders $1,091.67 and $4,031.61 overflow segments with different lengths", () => {
    const lines = renderHistory("Claude Code", "daily", entries, 200);
    const over = (label: string) =>
      stripAnsi(lines.find((l) => l.includes(label))!).split(RULE)[1].trim();
    assert.ok(over("2026-06-22").length > 0);
    assert.ok(over("2026-06-23").length > over("2026-06-22").length);
  });

  it("pads rows below p95 with spaces up to the rule", () => {
    const lines = renderHistory("Claude Code", "daily", entries, 200);
    const low = stripAnsi(lines.find((l) => l.includes("2026-06-01"))!);
    const beforeRule = low.split(RULE)[0];
    assert.ok(beforeRule.endsWith(" ".repeat(15)), "short bar should be space-padded to the rule column");
  });

  it("appends the p95 legend to the footer", () => {
    const lines = renderHistory("Claude Code", "daily", entries, 200);
    const footer = stripAnsi(lines.find((l) => stripAnsi(l).startsWith("avg "))!);
    assert.ok(footer.includes("avg $405.36/day"));
    assert.ok(footer.includes("peak $4,031.61 (2026-06-23)"));
    assert.ok(footer.includes(`${RULE} = ${fmtCost(1012.503)} (p95)`));
  });

  it("activates in renderTotalHistory as well", () => {
    const allToolEntries = new Map<string, UsageEntry[]>([["Claude Code", entries]]);
    const lines = renderTotalHistory("daily", allToolEntries, 200);
    const dataLines = lines.filter((l) => /^2026-06-\d{2}/.test(stripAnsi(l)));
    const cols = new Set(dataLines.map((l) => stripAnsi(l).indexOf(RULE)));
    assert.equal(cols.size, 1);
    assert.ok(stripAnsi(lines.find((l) => stripAnsi(l).startsWith("avg "))!).includes("(p95)"));
  });
});

// ---------------------------------------------------------------------------
// Single-zone windows render unchanged (no rule, no legend, no width change)
// ---------------------------------------------------------------------------
describe("single-zone windows", () => {
  it("keeps the pre-change bar rendering (delegates to renderBar)", () => {
    const entries = [entry("2020-01-01", 100), entry("2020-01-02", 200), entry("2020-01-03", 300)];
    const lines = renderHistory("Claude Code", "daily", entries, 200);
    assert.ok(!lines.some((l) => l.includes(RULE)), "no scale-break rule");
    const footer = stripAnsi(lines.find((l) => stripAnsi(l).startsWith("avg "))!);
    assert.ok(!footer.includes("p95"), "no p95 legend");
    // barWidth = 30 at termWidth 200: max-cost row keeps a full 30-block bar,
    // mid row matches renderBar exactly (no padding, no zones)
    const maxLine = stripAnsi(lines.find((l) => l.includes("2020-01-03"))!);
    assert.ok(maxLine.endsWith(FULL_BLOCK.repeat(30)));
    const midLine = stripAnsi(lines.find((l) => l.includes("2020-01-02"))!);
    assert.ok(midLine.endsWith(renderBar(200, 300, 30)));
  });
});

// ---------------------------------------------------------------------------
// Compact mode untouched
// ---------------------------------------------------------------------------
describe("compact mode", () => {
  it("carries no separators, footer, marker, or rule", () => {
    const today = currentLabel("daily");
    const entries = [entry("2026-06-30", 100), entry(today, 4031.61)];
    setNoColor(false);
    const lines = renderHistory("Claude Code", "daily", entries, 50, { compact: true });
    setNoColor(true);
    const output = lines.join("\n");
    assert.ok(!output.includes(RULE));
    assert.ok(!output.includes("avg "));
    assert.ok(!output.includes("\x1b[1;37m" + today), "no boldWhite date cell in compact mode");
    // month separator would be a second ─ divider; compact renders exactly one (before Total)
    assert.equal(lines.filter(isDivider).length, 1);
  });
});
