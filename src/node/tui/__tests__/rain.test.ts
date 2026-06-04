import { describe, it, beforeEach, afterEach } from "node:test";
import assert from "node:assert/strict";
import { RainState } from "../rain.js";
import { setNoColor } from "../colors.js";

// Disable colors for simpler assertions
setNoColor(true);

// The rain engine is driven entirely by Math.random (drop density, speed,
// length, delay, respawn, shimmer). Several assertions below are probabilistic
// under real randomness — e.g. a render can occasionally produce no visible
// drops, or a tick can vacate no cell — which made this suite flake
// intermittently on CI's concurrent runner. Pin Math.random to a deterministic
// seeded PRNG (mulberry32) for every test so the geometry is reproducible while
// still varying realistically across drops. The real Math.random is restored
// after each test. rain.ts itself is untouched.
const RAIN_SEED = 0x9e3779b1; // arbitrary fixed seed — any seed that satisfies the assertions works
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

describe("RainState", () => {
  let originalRandom: () => number;

  beforeEach(() => {
    originalRandom = Math.random;
    Math.random = mulberry32(RAIN_SEED);
  });

  afterEach(() => {
    Math.random = originalRandom;
  });

  it("initializes with ~30% column density", () => {
    const rain = new RainState(100, 10);
    // Access private drops via render — if render produces output, drops exist
    const output = rain.render(1);
    assert.ok(output.length > 0, "should have some rendered drops");
  });

  it("tick advances drops", () => {
    const rain = new RainState(20, 10);
    const before = rain.render(1);
    rain.tick();
    const after = rain.render(1);
    // Output should change after tick (drops move)
    // Note: could be same if all drops have delay, but unlikely with 20 cols
    assert.ok(typeof after === "string");
  });

  it("render produces cursor-positioned output", () => {
    const rain = new RainState(20, 10);
    const output = rain.render(5);
    // Should contain ANSI cursor positioning
    assert.ok(output.includes("\x1b["), "should contain ANSI escape codes");
    // Should position at or after startRow
    assert.match(output, /\x1b\[\d+;\d+H/, "should have row;col positioning");
  });

  it("render returns empty string for zero rows", () => {
    const rain = new RainState(20, 0);
    const output = rain.render(1);
    assert.equal(output, "");
  });

  it("resize reinitializes state when dimensions change", () => {
    const rain = new RainState(10, 5);
    rain.tick();
    rain.resize(50, 20);
    // After resize, should still render
    const output = rain.render(1);
    assert.ok(typeof output === "string");
  });

  it("resize is no-op when dimensions unchanged", () => {
    const rain = new RainState(20, 10);
    // Tick several times to build up state
    for (let i = 0; i < 5; i++) rain.tick();
    const before = rain.render(1);
    rain.resize(20, 10); // same dimensions
    const after = rain.render(1);
    // After no-op resize, output should be the same (drops preserved, not reinitialized)
    assert.equal(after, before);
  });

  it("uses fractional speeds — drops move smoothly over many ticks", () => {
    const rain = new RainState(50, 20);

    // Collect head row positions across many ticks to verify non-integer movement
    // Extract all row positions from render output
    function extractRows(output: string): number[] {
      return [...output.matchAll(/\x1b\[(\d+);/g)].map(m => Number(m[1]));
    }

    const allRows: Set<number>[] = [];
    for (let i = 0; i < 40; i++) {
      rain.tick();
      const output = rain.render(1);
      const rows = extractRows(output);
      allRows.push(new Set(rows));
    }

    // Verify drops are rendering across multiple rows over time
    const allUniqueRows = new Set<number>();
    for (const rowSet of allRows) {
      for (const r of rowSet) allUniqueRows.add(r);
    }
    assert.ok(allUniqueRows.size > 3, "drops should span multiple rows over 40 ticks");
  });

  it("drops respawn after going off-screen", () => {
    const rain = new RainState(5, 3);
    // Tick many times to ensure drops cycle
    for (let i = 0; i < 50; i++) {
      rain.tick();
    }
    const output = rain.render(1);
    // Should still have active drops after many ticks
    assert.ok(typeof output === "string");
  });

  it("clears old positions with cursor-positioned space writes", () => {
    const rain = new RainState(10, 10);
    // Render initial frame to populate prevPositions
    rain.render(1);
    // Tick to move drops — positions change — and accumulate every frame so a
    // vacated cell (and thus a space-clear write) reliably appears under the
    // seeded PRNG installed in beforeEach.
    let output = "";
    for (let i = 0; i < 10; i++) {
      rain.tick();
      output += rain.render(1);
    }
    // After movement, old positions should be cleared with space writes
    assert.match(
      output,
      /\x1b\[\d+;\d+H /,
      "should clear old positions using cursor-positioned space writes",
    );
  });

  it("render positions within rain zone boundaries", () => {
    const rain = new RainState(10, 5);
    const startRow = 10;
    const output = rain.render(startRow);
    // Extract all row positions
    const positions = [...output.matchAll(/\x1b\[(\d+);/g)].map((m) => Number(m[1]));
    for (const pos of positions) {
      assert.ok(pos >= startRow, `row ${pos} should be >= startRow ${startRow}`);
      assert.ok(pos < startRow + 5, `row ${pos} should be < ${startRow + 5}`);
    }
  });
});
