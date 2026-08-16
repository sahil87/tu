import { describe, it, afterEach } from "node:test";
import assert from "node:assert/strict";

import {
  apportionSegments,
  computeBarScale,
  renderBar,
  renderHistory,
  renderScaledBar,
  renderStackedScaledBar,
  renderTotalHistory,
} from "../formatter.js";
import { setNoColor, stripAnsi, cyan, magenta, blue, green } from "../colors.js";
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
const PALETTE = [green, magenta, blue, cyan];

// The scale-break rule is always dim-wrapped in two-zone bars — split helper.
function dimRule(): string {
  return `\x1b[2m${RULE}\x1b[0m`;
}

// ---------------------------------------------------------------------------
// apportionSegments (largest-remainder)
// ---------------------------------------------------------------------------
describe("apportionSegments", () => {
  it("always sums exactly to the total", () => {
    const cases: [number[], number][] = [
      [[300, 100, 0.04], 20],
      [[1, 1, 1], 4],
      [[1, 1], 3],
      [[600, 200, 0.08], 30],
      [[100, 100, 100, 100, 100], 30],
      [[7], 13],
    ];
    for (const [shares, total] of cases) {
      const counts = apportionSegments(shares, total);
      assert.equal(counts.reduce((s, c) => s + c, 0), total, `${shares} over ${total}`);
    }
  });

  it("apportions proportionally (300/100/0.04 over 20 → 15/5/0)", () => {
    assert.deepEqual(apportionSegments([300, 100, 0.04], 20), [15, 5, 0]);
  });

  it("breaks fractional ties by column order (earlier column wins)", () => {
    assert.deepEqual(apportionSegments([1, 1], 3), [2, 1]);
    assert.deepEqual(apportionSegments([1, 1, 1], 4), [2, 1, 1]);
  });

  it("gives zero-share tools no characters", () => {
    assert.deepEqual(apportionSegments([300, 100, 0], 20), [15, 5, 0]);
  });

  it("gives a 1-char bar entirely to the largest-share tool", () => {
    assert.deepEqual(apportionSegments([0.04, 300, 100], 1), [0, 1, 0]);
  });

  it("returns all zeros for a zero total or all-zero shares", () => {
    assert.deepEqual(apportionSegments([1, 2], 0), [0, 0]);
    assert.deepEqual(apportionSegments([0, 0], 5), [0, 0]);
  });
});

// ---------------------------------------------------------------------------
// renderStackedScaledBar — length/character invariants
// ---------------------------------------------------------------------------
describe("renderStackedScaledBar", () => {
  it("single-zone: stripped ANSI yields exactly today's unstacked bar", () => {
    const scale = computeBarScale([100, 200, 300], 30); // single zone
    for (const value of [0, 50, 100, 200, 300]) {
      const costs = [value * 0.75, value * 0.25];
      assert.equal(
        stripAnsi(renderStackedScaledBar(value, costs, PALETTE, scale, 30)),
        stripAnsi(renderScaledBar(value, scale, 30)),
        `value ${value}`,
      );
    }
  });

  it("two-zone: stripped ANSI yields exactly today's unstacked bar (clipped and unclipped)", () => {
    const costs = [...Array(21).keys()].map((i) => 100 + i * 10).concat([1091.67, 4031.61]);
    const scale = computeBarScale(costs, 30);
    assert.equal(scale.mode, "two-zone");
    for (const value of [0, 100, 300, 1091.67, 4031.61]) {
      const toolCosts = [value - 6.11, 6.11];
      assert.equal(
        stripAnsi(renderStackedScaledBar(value, toolCosts, PALETTE, scale, 30)),
        stripAnsi(renderScaledBar(value, scale, 30)),
        `value ${value}`,
      );
    }
  });

  it("two-zone output is exactly barWidth visible chars in every row", () => {
    const costs = [...Array(21).keys()].map((i) => 100 + i * 10).concat([1091.67, 4031.61]);
    const scale = computeBarScale(costs, 30);
    for (const value of [0, 300, 1091.67, 4031.61]) {
      assert.equal(stripAnsi(renderStackedScaledBar(value, [value], PALETTE, scale, 30)).length, 31);
    }
  });

  it("renders zero-total rows with no bar, exactly as today", () => {
    const scale = computeBarScale([100, 200, 300], 30);
    assert.equal(renderStackedScaledBar(0, [0, 0], PALETTE, scale, 30), "");
    assert.equal(renderScaledBar(0, scale, 30), "");
  });

  it("colors segments in palette order", () => {
    setNoColor(false);
    const scale = computeBarScale([450], 30); // single zone, max 450
    const bar = renderStackedScaledBar(450, [300, 100, 50], PALETTE, scale, 30);
    const i32 = bar.indexOf("\x1b[32m");
    const i35 = bar.indexOf("\x1b[35m");
    const i34 = bar.indexOf("\x1b[34m");
    assert.ok(i32 !== -1 && i35 !== -1 && i34 !== -1, "all three segment colors present");
    assert.ok(i32 < i35 && i35 < i34, "green → magenta → blue in column order");
  });

  it("gives a sub-dollar share no segment", () => {
    setNoColor(false);
    const scale = computeBarScale([400.04], 30);
    const bar = renderStackedScaledBar(400.04, [300, 100, 0.04], PALETTE, scale, 30);
    assert.ok(bar.includes("\x1b[32m") && bar.includes("\x1b[35m"));
    assert.ok(!bar.includes("\x1b[34m"), "Kimi's $0.04 share gets no blue segment");
  });

  it("lets the fractional-eighths character ride the last nonzero segment", () => {
    setNoColor(false);
    // scaled = 4.5/16*8 = 2.25 → raw "██▎"; shares apportion 3 chars as [2, 1]
    const scale = { mode: "single" as const, max: 16 };
    const bar = renderStackedScaledBar(4.5, [3.375, 1.125], PALETTE, scale, 8);
    assert.equal(bar, " " + green("██") + magenta("▎"));
  });

  it("keeps the overflow zone solid yellow, unsegmented", () => {
    setNoColor(false);
    const costs = [...Array(21).keys()].map((i) => 100 + i * 10).concat([1091.67, 4031.61]);
    const scale = computeBarScale(costs, 30);
    const bar = renderStackedScaledBar(4031.61, [4025.5, 6.11], PALETTE, scale, 30);
    const overflow = bar.split(dimRule())[1];
    assert.ok(overflow.includes("\x1b[33m"), "overflow is yellow");
    for (const code of ["\x1b[36m", "\x1b[35m", "\x1b[34m", "\x1b[32m"]) {
      assert.ok(!overflow.includes(code), `overflow carries no tool color ${JSON.stringify(code)}`);
    }
  });

  it("leaves a 5th tool's segment uncolored", () => {
    setNoColor(false);
    const scale = computeBarScale([500], 30);
    const palette = [green, magenta, blue, cyan, (s: string) => s];
    const bar = renderStackedScaledBar(500, [100, 100, 100, 100, 100], palette, scale, 30);
    assert.equal((bar.match(/\x1b\[3[2-6]m/g) ?? []).length, 4, "exactly four colored runs");
    assert.ok(bar.endsWith("\x1b[0m" + FULL_BLOCK.repeat(6)), "5th tool's 6 blocks follow the last reset, uncolored");
  });
});

// ---------------------------------------------------------------------------
// renderTotalHistory — stacked pivot integration
// ---------------------------------------------------------------------------
describe("renderTotalHistory stacked bars", () => {
  // Single-zone window: row totals 450 and 800.04 (max < 1.5 × p95)
  const dataset = () => new Map<string, UsageEntry[]>([
    ["Claude Code", [entry("2020-01-01", 300), entry("2020-01-02", 600)]],
    ["Codex", [entry("2020-01-01", 100), entry("2020-01-02", 200)]],
    ["Kimi", [entry("2020-01-01", 50), entry("2020-01-02", 0.04)]],
  ]);
  const isDataRow = (l: string) => /^2020-01-0[12]/.test(stripAnsi(l));

  it("every data row is byte-identical to no-color output once ANSI is stripped", () => {
    setNoColor(false);
    const colored = renderTotalHistory("daily", dataset(), 200);
    setNoColor(true);
    const plain = renderTotalHistory("daily", dataset(), 200);
    for (const pl of plain.filter(isDataRow)) {
      const cl = colored.filter(isDataRow).find((l) => stripAnsi(l) === pl);
      assert.ok(cl, `no colored row strips down to ${JSON.stringify(pl)}`);
    }
  });

  it("no-color bars match the unchanged renderScaledBar primitive byte-for-byte", () => {
    const lines = renderTotalHistory("daily", dataset(), 200);
    // tableWidth = 10 + (11+3) + (9+3) + (9+3) = 48 → barWidth 30 at termWidth 200
    const scale = computeBarScale([450, 800.04], 30);
    const row1 = lines.find((l) => stripAnsi(l).startsWith("2020-01-01"))!;
    const row2 = lines.find((l) => stripAnsi(l).startsWith("2020-01-02"))!;
    assert.ok(row1.endsWith(renderScaledBar(450, scale, 30)), "row 1 bar == v0.10.1 primitive output");
    assert.ok(row2.endsWith(renderScaledBar(800.04, scale, 30)), "row 2 bar == v0.10.1 primitive output");
  });

  it("assigns green/magenta/blue in column order and skips a zero-rounded share", () => {
    setNoColor(false);
    const lines = renderTotalHistory("daily", dataset(), 200);
    const row1 = lines.find((l) => l.includes("2020-01-01"))!;
    const i32 = row1.indexOf("\x1b[32m");
    const i35 = row1.indexOf("\x1b[35m");
    const i34 = row1.indexOf("\x1b[34m");
    assert.ok(i32 !== -1 && i35 !== -1 && i34 !== -1 && i32 < i35 && i35 < i34, "Claude green → Codex magenta → Kimi blue");
    const row2 = lines.find((l) => l.includes("2020-01-02"))!;
    assert.ok(!row2.includes("\x1b[34m"), "Kimi's $0.04 day rounds to zero — no blue segment");
  });

  it("stacks the whole bar in a two-zone window and keeps overflow yellow", () => {
    setNoColor(false);
    const days = [...Array(21).keys()].map((i) => entry(`2026-06-${String(i + 1).padStart(2, "0")}`, 100 + i * 10));
    const allToolEntries = new Map<string, UsageEntry[]>([
      ["Claude Code", [...days.slice(0, 21), entry("2026-06-22", 1085.56), entry("2026-06-23", 4025.5)]],
      ["Codex", [...days.slice(0, 21).map((e) => entry(e.label, 0)), entry("2026-06-22", 6.11), entry("2026-06-23", 6.11)]],
    ]);
    const lines = renderTotalHistory("daily", allToolEntries, 200);
    const clipped = lines.find((l) => l.includes("2026-06-23"))!;
    const overflow = clipped.split(RULE)[1];
    assert.ok(overflow.includes("\x1b[33m"), "overflow stays yellow");
    assert.ok(!overflow.includes("\x1b[32m"), "overflow is not segmented");
    const dataLines = lines.filter((l) => /^2026-06-\d{2}/.test(stripAnsi(l)));
    const ruleCols = new Set(dataLines.map((l) => stripAnsi(l).indexOf(RULE)));
    assert.equal(ruleCols.size, 1, "rule column still aligns across rows");
    setNoColor(true);
  });

  it("renders a 5th visible tool's segments uncolored", () => {
    setNoColor(false);
    const five = new Map<string, UsageEntry[]>([
      ["Claude Code", [entry("2020-01-01", 100), entry("2020-01-02", 100)]],
      ["Codex", [entry("2020-01-01", 100), entry("2020-01-02", 100)]],
      ["Kimi", [entry("2020-01-01", 100), entry("2020-01-02", 100)]],
      ["Gemini", [entry("2020-01-01", 100), entry("2020-01-02", 100)]],
      ["Copilot", [entry("2020-01-01", 100), entry("2020-01-02", 100)]],
    ]);
    const lines = renderTotalHistory("daily", five, 200);
    const row = lines.find((l) => l.includes("2020-01-01"))!;
    assert.ok(row.endsWith("\x1b[0m" + FULL_BLOCK.repeat(6)), "Copilot's 6 blocks are uncolored after the last reset");
    setNoColor(true);
  });
});

// ---------------------------------------------------------------------------
// Footer legend
// ---------------------------------------------------------------------------
describe("footer legend", () => {
  const dataset = () => new Map<string, UsageEntry[]>([
    ["Claude Code", [entry("2020-01-01", 300), entry("2020-01-02", 600)]],
    ["Codex", [entry("2020-01-01", 100), entry("2020-01-02", 200)]],
    ["Kimi", [entry("2020-01-01", 50), entry("2020-01-02", 0.04)]],
  ]);
  const footerOf = (lines: string[]) => lines.find((l) => stripAnsi(l).startsWith("avg "))!;

  it("appends one colored swatch per visible tool in column order", () => {
    setNoColor(false);
    const footer = footerOf(renderTotalHistory("daily", dataset(), 200));
    assert.equal(
      stripAnsi(footer),
      "avg $625.02/day · peak $800.04 (2020-01-02) · █ Claude Code █ Codex █ Kimi",
    );
    assert.ok(footer.includes("\x1b[32m█\x1b[0m \x1b[2mClaude Code\x1b[0m"), "green swatch, dim name");
    assert.ok(footer.includes("\x1b[35m█\x1b[0m \x1b[2mCodex\x1b[0m"), "magenta swatch, dim name");
    assert.ok(footer.includes("\x1b[34m█\x1b[0m \x1b[2mKimi\x1b[0m"), "blue swatch, dim name");
    setNoColor(true);
  });

  it("keeps the surrounding footer text dim across the swatch resets", () => {
    setNoColor(false);
    const footer = footerOf(renderTotalHistory("daily", dataset(), 200));
    assert.ok(footer.startsWith("\x1b[2m"), "main footer is dim");
    assert.ok(footer.includes("\x1b[0m\x1b[2m · \x1b[0m\x1b[32m"), "legend separator is re-dimmed after the footer reset");
    setNoColor(true);
  });

  it("is omitted under --no-color, leaving the footer byte-identical to today's", () => {
    const footer = footerOf(renderTotalHistory("daily", dataset(), 200));
    assert.equal(footer, "avg $625.02/day · peak $800.04 (2020-01-02)");
  });

  it("is omitted when NO_COLOR is set", () => {
    process.env.NO_COLOR = "1";
    const footer = footerOf(renderTotalHistory("daily", dataset(), 200));
    delete process.env.NO_COLOR;
    assert.equal(footer, "avg $625.02/day · peak $800.04 (2020-01-02)");
  });

  it("is omitted when bars are suppressed (narrow terminal)", () => {
    setNoColor(false);
    const footer = footerOf(renderTotalHistory("daily", dataset(), 60));
    assert.ok(!stripAnsi(footer).includes("█"), "no legend without bars");
    setNoColor(true);
  });

  it("is omitted with a single visible tool", () => {
    setNoColor(false);
    const single = new Map<string, UsageEntry[]>([
      ["Claude Code", [entry("2020-01-01", 300), entry("2020-01-02", 600)]],
    ]);
    const footer = footerOf(renderTotalHistory("daily", single, 200));
    assert.ok(!stripAnsi(footer).includes("█"), "no legend for one tool");
    setNoColor(true);
  });

  it("follows the p95 group in two-zone windows", () => {
    setNoColor(false);
    const days = [...Array(21).keys()].map((i) => entry(`2026-06-${String(i + 1).padStart(2, "0")}`, 100 + i * 10));
    const allToolEntries = new Map<string, UsageEntry[]>([
      ["Claude Code", [...days, entry("2026-06-22", 1085.56), entry("2026-06-23", 4025.5)]],
      ["Codex", [...days.map((e) => entry(e.label, 0)), entry("2026-06-22", 6.11), entry("2026-06-23", 6.11)]],
    ]);
    const footer = stripAnsi(footerOf(renderTotalHistory("daily", allToolEntries, 200)));
    assert.ok(footer.includes(`${RULE} = $1,012.50 (p95) · █ Claude Code █ Codex`), `footer was ${footer}`);
    setNoColor(true);
  });
});

// ---------------------------------------------------------------------------
// Single-tool history is out of scope — output unchanged
// ---------------------------------------------------------------------------
describe("renderHistory (single-tool) unchanged", () => {
  it("keeps green bars with no stacked palette colors", () => {
    setNoColor(false);
    const entries = [entry("2020-01-01", 100), entry("2020-01-02", 200), entry("2020-01-03", 300)];
    const lines = renderHistory("Claude Code", "daily", entries, 200);
    const out = lines.join("\n");
    assert.ok(out.includes("\x1b[32m"), "bar stays green");
    assert.ok(!out.includes("\x1b[35m") && !out.includes("\x1b[34m") && !out.includes("\x1b[36m█"), "no stacked palette in single-tool history");
    setNoColor(true);
    const plain = renderHistory("Claude Code", "daily", entries, 200);
    assert.deepEqual(lines.map(stripAnsi), plain, "colored single-tool output strips to the no-color bytes");
    const midLine = plain.find((l) => l.includes("2020-01-02"))!;
    assert.ok(midLine.endsWith(renderBar(200, 300, 30)), "bar matches the renderBar primitive");
  });
});
