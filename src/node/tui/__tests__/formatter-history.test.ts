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
// Weekend date-cell dimming (daily period only)
// ---------------------------------------------------------------------------
describe("weekend date-cell dimming", () => {
  // 2026-06-05 Fri, 06-06 Sat, 06-07 Sun, 06-08 Mon — all safely in the past
  const week = [
    entry("2026-06-05", 100),
    entry("2026-06-06", 100),
    entry("2026-06-07", 100),
    entry("2026-06-08", 100),
  ];

  // A dimmed date cell puts the dim code at the very start of the data row
  function dateCellDim(line: string): boolean {
    return line.startsWith("\x1b[2m");
  }

  it("renderHistory dims Saturday and Sunday date cells only, width unchanged", () => {
    setNoColor(false);
    const colored = renderHistory("Claude Code", "daily", week, 200);
    setNoColor(true);
    const plain = renderHistory("Claude Code", "daily", week, 200);

    for (const label of ["2026-06-06", "2026-06-07"]) {
      const line = colored.find((l) => l.includes(label))!;
      assert.ok(dateCellDim(line), `${label} date cell should be dim`);
      assert.equal(stripAnsi(line), plain.find((l) => l.includes(label))!, "dimming adds no width");
    }
    for (const label of ["2026-06-05", "2026-06-08"]) {
      assert.ok(!dateCellDim(colored.find((l) => l.includes(label))!), `${label} date cell should not be dim`);
    }
  });

  it("renderTotalHistory dims weekend date cells", () => {
    const allToolEntries = new Map<string, UsageEntry[]>([
      ["Claude Code", week],
      ["Codex", week.map((e) => entry(e.label, 5))],
    ]);
    setNoColor(false);
    const lines = renderTotalHistory("daily", allToolEntries, 200);
    setNoColor(true);
    assert.ok(dateCellDim(lines.find((l) => l.includes("2026-06-06"))!));
    assert.ok(dateCellDim(lines.find((l) => l.includes("2026-06-07"))!));
    assert.ok(!dateCellDim(lines.find((l) => l.includes("2026-06-08"))!));
  });

  it("the today marker wins over the weekend dim (one cell, one style)", () => {
    // Fully exercised when today falls on a weekend; on weekdays it still
    // verifies the marker renders boldWhite with no dim prefix.
    const today = currentLabel("daily");
    setNoColor(false);
    const lines = renderHistory("Claude Code", "daily", [entry("2026-06-06", 100), entry(today, 100)], 200);
    setNoColor(true);
    const todayLine = lines.find((l) => l.includes(today))!;
    assert.ok(todayLine.startsWith("\x1b[1;37m"), "today's date cell should be boldWhite");
    assert.ok(!dateCellDim(todayLine), "today's date cell should never be dim");
  });

  it("monthly periods never dim, even when the month starts on a weekend", () => {
    // 2020-08 parses to 2020-08-01, a Saturday — only the period gate protects it
    setNoColor(false);
    const lines = renderHistory("Claude Code", "monthly", [entry("2020-08", 100), entry("2020-09", 100)], 200);
    setNoColor(true);
    assert.ok(!dateCellDim(lines.find((l) => l.includes("2020-08"))!));
  });

  it("weekly periods never dim, even though week labels fall on Sundays", () => {
    setNoColor(false);
    const lines = renderHistory("Claude Code", "weekly", [entry("2026-08-02", 100), entry("2026-08-09", 100)], 200);
    setNoColor(true);
    assert.ok(!dateCellDim(lines.find((l) => l.includes("2026-08-02"))!));
    assert.ok(!dateCellDim(lines.find((l) => l.includes("2026-08-09"))!));
  });

  it("no-color output carries no ANSI codes for weekend rows", () => {
    const lines = renderHistory("Claude Code", "daily", week, 200);
    assert.ok(lines.every((l) => !l.includes("\x1b[")), "NO_COLOR output should be ANSI-free");
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

// ---------------------------------------------------------------------------
// --metric: bar scale by tokens vs cost
// ---------------------------------------------------------------------------
describe("FormatOptions.metric", () => {
  // Any Unicode block element (full block + fractional eighths) counts as bar fill.
  function barLen(line: string): number {
    return (stripAnsi(line).match(/[\u2588-\u258F]/g) ?? []).length;
  }
  function dataRows(lines: string[]): string[] {
    return lines.filter((l) => /^\d{4}-\d{2}/.test(stripAnsi(l)));
  }
  function longestRowLabel(lines: string[]): string {
    const rows = dataRows(lines);
    let best = rows[0];
    for (const r of rows) if (barLen(r) > barLen(best)) best = r;
    return stripAnsi(best).slice(0, 10);
  }

  // Highest cost on 2026-06-01; highest tokens on 2026-06-02.
  const entries: UsageEntry[] = [
    { label: "2026-06-01", totalCost: 100, inputTokens: 100, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 100 },
    { label: "2026-06-02", totalCost: 10, inputTokens: 9000, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 9000 },
    { label: "2026-06-03", totalCost: 50, inputTokens: 500, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 500 },
  ];

  it("renderHistory: bars scale by cost by default and by tokens under metric: tokens", () => {
    assert.equal(longestRowLabel(renderHistory("Claude Code", "daily", entries, 200)), "2026-06-01");
    assert.equal(longestRowLabel(renderHistory("Claude Code", "daily", entries, 200, { metric: "tokens" })), "2026-06-02");
  });

  it("renderHistory: metric: cost is byte-identical to no option", () => {
    assert.deepEqual(renderHistory("Claude Code", "daily", entries, 200, { metric: "cost" }), renderHistory("Claude Code", "daily", entries, 200));
  });

  it("renderHistory: footer formats token values with fmtNum (no $) under metric: tokens", () => {
    const lines = renderHistory("Claude Code", "daily", entries, 200, { metric: "tokens" }).map(stripAnsi);
    const footer = lines.find((l) => l.includes("avg "))!;
    assert.ok(footer.includes("avg 3,200/day"), footer);
    assert.ok(footer.includes("peak 9,000 (2026-06-02)"), footer);
    assert.ok(!footer.includes("$"), footer);
    // Cost cells are still cost-denominated.
    assert.ok(lines.some((l) => l.startsWith("2026-06-02") && l.includes("$10.00")));
  });

  it("renderTotalHistory: bars and footer follow the metric; cells stay cost", () => {
    const allToolEntries = new Map<string, UsageEntry[]>([
      ["Claude Code", entries],
      ["Codex", [{ label: "2026-06-03", totalCost: 1, inputTokens: 100, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 100 }]],
    ]);
    assert.equal(longestRowLabel(renderTotalHistory("daily", allToolEntries, 200)), "2026-06-01");
    const tokenLines = renderTotalHistory("daily", allToolEntries, 200, { metric: "tokens" });
    assert.equal(longestRowLabel(tokenLines), "2026-06-02");
    const plain = tokenLines.map(stripAnsi);
    const footer = plain.find((l) => l.includes("avg "))!;
    assert.ok(footer.includes("peak 9,000 (2026-06-02)"), footer);
    assert.ok(!footer.includes("$"), footer);
    assert.ok(plain.some((l) => l.startsWith("2026-06-03") && l.includes("$51.00")), "row cost cell unchanged");
    assert.deepEqual(renderTotalHistory("daily", allToolEntries, 200, { metric: "cost" }), renderTotalHistory("daily", allToolEntries, 200));
  });
});

describe("FormatOptions.machineLegend", () => {
  const entries: UsageEntry[] = [
    { label: "2026-06-01", totalCost: 3, inputTokens: 1, outputTokens: 1, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 2 },
    { label: "2026-06-02", totalCost: 4, inputTokens: 1, outputTokens: 1, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 2 },
  ];
  const machineCosts = new Map<string, Map<string, number>>([
    ["2026-06-01", new Map([["alice", 1], ["bob", 2]])],
    ["2026-06-02", new Map([["alice", 4], ["bob", 0]])],
  ]);

  it("defaults the breakdown legend to Machines", () => {
    const lines = renderHistory("Claude Code", "daily", entries, 200, { machineCosts }).map(stripAnsi);
    assert.ok(lines.some((l) => l.startsWith("Machines: A = alice, B = bob")), lines.join("\n"));
  });

  it("relabels the legend when machineLegend is set (-u all --by-machine keys columns by user)", () => {
    const lines = renderHistory("Claude Code", "daily", entries, 200, { machineCosts, machineLegend: "Users" }).map(stripAnsi);
    assert.ok(lines.some((l) => l.startsWith("Users: A = alice, B = bob")), lines.join("\n"));
    assert.ok(!lines.some((l) => l.startsWith("Machines:")));
  });
});

// ---------------------------------------------------------------------------
// FormatOptions.total — collapsed all-tools history (Date | value | bar)
// ---------------------------------------------------------------------------
describe("FormatOptions.total", () => {
  function barLen(line: string): number {
    return (stripAnsi(line).match(/[\u2588-\u258F]/g) ?? []).length;
  }
  const mk = (label: string, cost: number, tokens: number): UsageEntry =>
    ({ label, totalCost: cost, inputTokens: tokens, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: tokens });
  const allToolEntries = new Map<string, UsageEntry[]>([
    ["Claude Code", [mk("2026-06-01", 100, 1000), mk("2026-06-02", 10, 9000), mk("2026-07-01", 50, 500)]],
    ["Codex", [mk("2026-06-02", 1, 100), mk("2026-07-01", 2, 200)]],
  ]);

  it("renders Date | Cost | bar with no tool columns, a grand-total-only Total row and no legend", () => {
    setNoColor(false);
    const lines = renderTotalHistory("daily", allToolEntries, 80, { total: true });
    const plain = lines.map(stripAnsi);
    assert.equal(plain[3], "Date       |      Cost");
    assert.ok(!plain.some((l) => l.includes("Claude Code") || l.includes("Codex")), plain.join("\n"));
    const rows = lines.filter((l) => /^\d{4}-\d{2}-\d{2}/.test(stripAnsi(l)));
    assert.equal(rows.length, 3);
    assert.ok(rows.every((r) => barLen(r) > 0), "every row has a bar");
    const longest = rows.reduce((a, b) => (barLen(b) > barLen(a) ? b : a));
    assert.ok(stripAnsi(longest).startsWith("2026-06-01"), stripAnsi(longest));
    assert.ok(stripAnsi(rows[0]).includes("$100.00"));
    const total = plain.find((l) => l.startsWith("Total"))!;
    assert.equal(total, "Total      |   $163.00");
    const footer = plain.find((l) => l.includes("avg "))!;
    assert.ok(!footer.includes("█"), footer);
    // Daily month separator still present between 2026-06-02 and 2026-07-01
    const idx = lines.findIndex((l) => stripAnsi(l).startsWith("2026-07-01"));
    assert.ok(isDivider(lines[idx - 1]), "month separator before 2026-07-01");
    // Full 30-char bar fits at 80 cols
    assert.equal(barLen(longest), 30);
  });

  it("shows a Tokens header and fmtNum values under metric: tokens", () => {
    const plain = renderTotalHistory("daily", allToolEntries, 80, { total: true, metric: "tokens" }).map(stripAnsi);
    assert.ok(plain[3].startsWith("Date       |") && plain[3].endsWith("Tokens"), plain[3]);
    assert.ok(plain.some((l) => l.startsWith("2026-06-02") && l.includes("9,100")), plain.join("\n"));
    const total = plain.find((l) => l.startsWith("Total"))!;
    assert.ok(total.endsWith("10,800"), total);
    const footer = plain.find((l) => l.includes("avg "))!;
    assert.ok(footer.includes("peak 9,100 (2026-06-02)") && !footer.includes("$"), footer);
  });

  it("total: false and absent are byte-identical", () => {
    assert.deepEqual(renderTotalHistory("daily", allToolEntries, 200, { total: false }), renderTotalHistory("daily", allToolEntries, 200));
  });

  it("maxRows still truncates under total", () => {
    const rows = renderTotalHistory("daily", allToolEntries, 80, { total: true, maxRows: 2 }).filter((l) => /^\d{4}/.test(stripAnsi(l)));
    assert.equal(rows.length, 2);
  });
});
