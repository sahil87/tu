import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { renderSparkline } from "../sparkline.js";
import { setNoColor } from "../colors.js";

// Disable colors for simpler assertions
setNoColor(true);

describe("renderSparkline", () => {
  it("returns empty for fewer than 2 data points", () => {
    const result = renderSparkline([{ label: "2026-01-01", cost: 10 }], 30);
    assert.deepEqual(result, []);
  });

  it("returns empty for empty data", () => {
    const result = renderSparkline([], 30);
    assert.deepEqual(result, []);
  });

  it("renders sparkline with 7 data points", () => {
    const data = Array.from({ length: 7 }, (_, i) => ({
      label: `2026-01-0${i + 1}`,
      cost: 10 + i * 5,
    }));
    const result = renderSparkline(data, 30);
    assert.ok(result.length > 0, "should return lines");
    assert.ok(result[0].includes("2026-01-01"), "should have first date in title");
    assert.ok(result[0].includes("2026-01-07"), "should have last date in title");
  });

  it("includes y-axis labels", () => {
    const data = [
      { label: "2026-01-01", cost: 40 },
      { label: "2026-01-02", cost: 100 },
      { label: "2026-01-03", cost: 70 },
    ];
    const result = renderSparkline(data, 30);
    const text = result.join("\n");
    assert.ok(text.includes("$100"), "should show max cost label");
    assert.ok(text.includes("$40"), "should show min cost label");
  });

  it("contains braille characters", () => {
    const data = [
      { label: "2026-01-01", cost: 10 },
      { label: "2026-01-02", cost: 50 },
      { label: "2026-01-03", cost: 30 },
    ];
    const result = renderSparkline(data, 30);
    const text = result.join("");
    // Braille characters are in U+2800-U+28FF range
    assert.ok(/[\u2800-\u28FF]/.test(text), "should contain braille characters");
  });

  it("handles all-zero costs (flat line)", () => {
    const data = [
      { label: "2026-01-01", cost: 0 },
      { label: "2026-01-02", cost: 0 },
      { label: "2026-01-03", cost: 0 },
    ];
    const result = renderSparkline(data, 30);
    assert.ok(result.length > 0, "should still render");
  });

  it("limits to available width", () => {
    const data = Array.from({ length: 100 }, (_, i) => ({
      label: `day${i}`,
      cost: Math.random() * 100,
    }));
    const result = renderSparkline(data, 20);
    assert.ok(result.length > 0);
    // Title should show date range (arrow between first and last label)
    assert.ok(result[0].includes("\u2192"), "should have arrow in date range title");
  });
});
