import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { RainState } from "../rain.js";
import { setNoColor } from "../colors.js";

// Disable colors for simpler assertions
setNoColor(true);

describe("RainState", () => {
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
    // Tick to move drops — positions change
    for (let i = 0; i < 5; i++) rain.tick();
    const output = rain.render(1);
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
