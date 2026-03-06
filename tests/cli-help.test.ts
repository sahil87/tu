import { describe, it } from "node:test";
import assert from "node:assert/strict";

import { SHORT_USAGE, FULL_HELP } from "../src/cli.js";

describe("SHORT_USAGE", () => {
  it("contains Usage header with new grammar", () => {
    assert.ok(SHORT_USAGE.includes("Usage: tu [source] [period] [display]"));
  });

  it("contains common examples using new grammar", () => {
    assert.ok(SHORT_USAGE.includes("tu cc"));
    assert.ok(SHORT_USAGE.includes("tu mh"));
  });

  it("contains pointer to full help", () => {
    assert.ok(SHORT_USAGE.includes("Run 'tu help' for all commands."));
  });

  it("does not contain old-style commands", () => {
    assert.ok(!SHORT_USAGE.includes("tu total daily"));
    assert.ok(!SHORT_USAGE.includes("tu total-history"));
    assert.ok(!SHORT_USAGE.includes("<command>"));
  });

  it("is concise (~8 lines or fewer)", () => {
    const lines = SHORT_USAGE.trim().split("\n");
    assert.ok(lines.length <= 8, `Expected <= 8 lines, got ${lines.length}`);
  });
});

describe("FULL_HELP", () => {
  it("contains Sources section with all source values", () => {
    assert.ok(FULL_HELP.includes("Sources:"));
    assert.ok(FULL_HELP.includes("cc"));
    assert.ok(FULL_HELP.includes("codex"));
    assert.ok(FULL_HELP.includes("co"));
    assert.ok(FULL_HELP.includes("oc"));
    assert.ok(FULL_HELP.includes("all"));
  });

  it("contains Periods section with short and long forms", () => {
    assert.ok(FULL_HELP.includes("Periods:"));
    assert.ok(FULL_HELP.includes("d/daily"));
    assert.ok(FULL_HELP.includes("m/monthly"));
  });

  it("contains Display section", () => {
    assert.ok(FULL_HELP.includes("Display:"));
    assert.ok(FULL_HELP.includes("h/history"));
  });

  it("contains Combined modifiers", () => {
    assert.ok(FULL_HELP.includes("Combined:"));
    assert.ok(FULL_HELP.includes("dh"));
    assert.ok(FULL_HELP.includes("mh"));
  });

  it("contains Setup section with all setup commands", () => {
    assert.ok(FULL_HELP.includes("Setup:"));
    assert.ok(FULL_HELP.includes("tu init-conf"));
    assert.ok(FULL_HELP.includes("tu init-metrics"));
    assert.ok(FULL_HELP.includes("tu sync"));
    assert.ok(FULL_HELP.includes("tu status"));
  });

  it("contains all help forms", () => {
    assert.ok(FULL_HELP.includes("tu help"));
    assert.ok(FULL_HELP.includes("tu -h"));
    assert.ok(FULL_HELP.includes("tu --help"));
  });

  it("contains Flags section", () => {
    assert.ok(FULL_HELP.includes("Flags:"));
    assert.ok(FULL_HELP.includes("--json"));
    assert.ok(FULL_HELP.includes("--sync"));
    assert.ok(FULL_HELP.includes("--fresh"));
    assert.ok(FULL_HELP.includes("-f"));
  });

  it("contains Examples using new grammar", () => {
    assert.ok(FULL_HELP.includes("Examples:"));
    assert.ok(FULL_HELP.includes("tu cc"));
    assert.ok(FULL_HELP.includes("tu h"));
    assert.ok(FULL_HELP.includes("tu cc mh"));
  });

  it("does not contain old-style commands", () => {
    assert.ok(!FULL_HELP.includes("tu total daily"));
    assert.ok(!FULL_HELP.includes("total-history"));
    assert.ok(!FULL_HELP.includes("tu-claude"));
    assert.ok(!FULL_HELP.includes("tu-codex"));
    assert.ok(!FULL_HELP.includes("tu-opencode"));
    assert.ok(!FULL_HELP.includes("tu setup"), "help should not reference old 'tu setup' command");
  });
});
