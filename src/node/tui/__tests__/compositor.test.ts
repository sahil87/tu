import { describe, it, beforeEach, afterEach } from "node:test";
import assert from "node:assert/strict";
import { Compositor } from "../compositor.js";
import type { PanelSession } from "../panel.js";
import { setNoColor, stripAnsi } from "../colors.js";

// Disable colors for simpler assertions
setNoColor(true);

// Rain geometry is driven by Math.random. Pin it to a deterministic seeded PRNG
// (mulberry32, same approach as rain.test.ts) so the drop layout is reproducible
// and renderDirect() reliably produces output within the configured zone.
const SEED = 0x9e3779b1;
function mulberry32(seed: number): () => number {
  let a = seed;
  return () => {
    a |= 0;
    a = (a + 0x6d2b79f5) | 0;
    let t = Math.imul(a ^ (a >>> 15), 1 | a);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

function makeCompositor(cols: number, rows: number, noRain = false): Compositor {
  return new Compositor({
    noRain,
    getTermWidth: () => cols,
    getTermRows: () => rows,
  });
}

// Capture everything written to stdout during fn().
function captureStdout(fn: () => void): string {
  const original = process.stdout.write.bind(process.stdout);
  let buf = "";
  (process.stdout as unknown as { write: (s: string) => boolean }).write = (
    s: string,
  ): boolean => {
    buf += s;
    return true;
  };
  try {
    fn();
  } finally {
    (process.stdout as unknown as { write: typeof original }).write = original;
  }
  return buf;
}

// Extract the 1-based columns from cursor-positioned writes: \x1b[row;colH
function extractCols(output: string): number[] {
  return [...output.matchAll(/\x1b\[\d+;(\d+)H/g)].map((m) => Number(m[1]));
}

describe("Compositor.layoutForSkeleton", () => {
  let originalRandom: () => number;
  beforeEach(() => {
    originalRandom = Math.random;
    Math.random = mulberry32(SEED);
  });
  afterEach(() => {
    Math.random = originalRandom;
  });

  it("enables rain before any poll (below-content, rows available)", () => {
    // Wide terminal, tall enough that content leaves rows below for rain.
    const c = makeCompositor(120, 40);
    assert.equal(c.rainLayer.isEnabled(), false, "rain disabled before layout");

    const skeleton = Array.from({ length: 8 }, (_, i) => `line ${i}`);
    c.layoutForSkeleton(skeleton);

    assert.equal(
      c.rainLayer.isEnabled(),
      true,
      "rain enabled after skeleton layout (before first poll)",
    );
    // renderDirect should produce cursor-positioned output.
    const out = c.rainLayer.renderDirect();
    assert.match(out, /\x1b\[\d+;\d+H/, "skeleton rain renders positioned cells");
  });

  it("does not enable rain when noRain is set", () => {
    const c = makeCompositor(120, 40, /* noRain */ true);
    c.layoutForSkeleton(["a", "b", "c"]);
    assert.equal(c.rainLayer.isEnabled(), false);
  });

  it("does not enable rain in compact terminals", () => {
    const c = makeCompositor(50 /* < COMPACT_THRESHOLD */, 40);
    c.layoutForSkeleton(["a", "b", "c"]);
    assert.equal(c.rainLayer.isEnabled(), false);
  });
});

describe("Compositor right-margin gutter", () => {
  let originalRandom: () => number;
  beforeEach(() => {
    originalRandom = Math.random;
    Math.random = mulberry32(SEED);
  });
  afterEach(() => {
    Math.random = originalRandom;
  });

  // Force right-margin mode: the terminal must be non-compact (tw >= 60) AND
  // content must fill all rows (no rows below) while leaving horizontal room to
  // the right of the widest content line.
  it("starts rain a 2-column gutter past the widest content line", () => {
    const cols = 120;
    const rows = 10;
    const c = makeCompositor(cols, rows);

    // contentHeight = rows so availableRainRows = rows - contentHeight - 1 < 0.
    const contentWidth = 90; // leaves cols - 90 - 2 = 28 margin columns
    const content = Array.from({ length: rows }, () => "x".repeat(contentWidth));
    c.layoutForSkeleton(content);

    assert.equal(c.rainLayer.isEnabled(), true, "right-margin rain enabled");

    const out = c.rainLayer.renderDirect();
    const columns = extractCols(out);
    assert.ok(columns.length > 0, "right-margin rain produced positioned cells");

    // 1-based screen col = startCol + drop.col + 1, startCol = contentWidth + gutter(2).
    // So the minimum observed column must be >= contentWidth + 2 + 1.
    const minCol = Math.min(...columns);
    assert.ok(
      minCol >= contentWidth + 2 + 1,
      `min rain col ${minCol} should be >= ${contentWidth + 2 + 1} (content ${contentWidth} + gutter 2 + 1-based)`,
    );
  });

  it("disables rain when the post-gutter margin is below the minimum", () => {
    // tw 120, content 109 => margin 120 - 109 - 2 = 9 < MIN_RAIN_COLS(10).
    const cols = 120;
    const rows = 10;
    const contentWidth = 109;
    const c = makeCompositor(cols, rows);

    const content = Array.from({ length: rows }, () => "x".repeat(contentWidth));
    c.layoutForSkeleton(content);

    assert.equal(
      c.rainLayer.isEnabled(),
      false,
      "rain disabled when post-gutter width < MIN_RAIN_COLS",
    );
  });

  it("keeps rain enabled when the post-gutter margin meets the minimum", () => {
    // tw 120, content 108 => margin 120 - 108 - 2 = 10 == MIN_RAIN_COLS(10): still enabled.
    const cols = 120;
    const rows = 10;
    const contentWidth = 108;
    const c = makeCompositor(cols, rows);

    const content = Array.from({ length: rows }, () => "x".repeat(contentWidth));
    c.layoutForSkeleton(content);

    assert.equal(c.rainLayer.isEnabled(), true);
  });
});

describe("Compositor.flush rain redraw", () => {
  let originalRandom: () => number;
  beforeEach(() => {
    originalRandom = Math.random;
    Math.random = mulberry32(SEED);
  });
  afterEach(() => {
    Math.random = originalRandom;
  });

  it("re-emits the rain frame after clearing content", () => {
    const cols = 120;
    const rows = 40;
    const c = makeCompositor(cols, rows);

    const session: PanelSession = {
      startTime: Date.now(),
      startCost: 0,
      startTokens: 0,
      pollHistory: [{ time: Date.now(), cost: 1 }],
      totalTokens: 0,
    };
    const tableLines = ["Tool | Cost", "a | $1"];
    c.updateAfterPoll(tableLines, session, 1);
    assert.equal(c.rainLayer.isEnabled(), true, "below-content rain enabled after poll");

    const out = captureStdout(() => c.flush());

    // The screen-clear (\x1b[J) must be followed by rain output — i.e. rain is
    // restored within the same flush rather than left blank until the next tick.
    const clearIdx = out.indexOf("\x1b[J");
    assert.ok(clearIdx >= 0, "flush performs a screen clear (\\x1b[J)");

    // The footer (written after the clear) is ITSELF a cursor-positioned write
    // (\x1b[${rows};1H), so asserting on any positioned sequence after the clear
    // would pass even if the rain frame were omitted — a false positive. flush()
    // emits the rain frame strictly AFTER the footer, so slice past the footer's
    // position sequence and assert rain cells appear in that tail.
    const footerSeq = `\x1b[${rows};1H`;
    const footerIdx = out.indexOf(footerSeq, clearIdx);
    assert.ok(footerIdx >= 0, "flush writes the footer after the clear");
    const afterFooter = out.slice(footerIdx + footerSeq.length);
    assert.match(
      afterFooter,
      /\x1b\[\d+;\d+H/,
      "rain frame is re-emitted after the footer (no per-poll blink)",
    );
  });
});

describe("Compositor push-driven footer", () => {
  it("writes the footer synchronously on setRefreshing (no interval needed)", () => {
    const c = makeCompositor(120, 40, /* noRain */ true);
    const out = captureStdout(() => c.setRefreshing());
    assert.ok(
      stripAnsi(out).includes("Refreshing..."),
      "setRefreshing pushes the footer immediately",
    );
  });

  it("writes the countdown footer synchronously on startCountdown", () => {
    const c = makeCompositor(120, 40, /* noRain */ true);
    const out = captureStdout(() => c.startCountdown(10, () => {}));
    c.cancelCountdown();
    assert.ok(
      stripAnsi(out).includes("Next refresh: 10s"),
      "startCountdown pushes the footer immediately",
    );
  });
});
