import { describe, it, mock } from "node:test";
import assert from "node:assert/strict";

import { parseGlobalFlags } from "../cli.js";

function captureExit(): { code: number | null; errors: string[] } {
  const state = { code: null as number | null, errors: [] as string[] };
  mock.method(process, "exit", ((code: number) => { state.code = code; }) as never);
  mock.method(console, "error", ((...args: unknown[]) => { state.errors.push(args.map(String).join(" ")); }) as never);
  return state;
}

function restoreMocks() {
  mock.restoreAll();
}

describe("parseGlobalFlags: --top <n> extraction", () => {
  it("parses --top with a positive integer and strips flag + value", () => {
    const r = parseGlobalFlags(["m", "lb", "--top", "3"]);
    assert.equal(r.topFlag, 3);
    assert.deepEqual(r.filteredArgs, ["m", "lb"]);
  });

  it("topFlag is undefined when --top is absent", () => {
    const r = parseGlobalFlags(["m", "lb"]);
    assert.equal(r.topFlag, undefined);
  });

  it("--top 1 is accepted (boundary)", () => {
    const r = parseGlobalFlags(["lb", "--top", "1"]);
    assert.equal(r.topFlag, 1);
  });

  it("--top composes with other flags", () => {
    const r = parseGlobalFlags(["--top", "5", "cc", "m", "lb", "--json"]);
    assert.equal(r.topFlag, 5);
    assert.equal(r.outputFormat, "json");
    assert.deepEqual(r.filteredArgs, ["cc", "m", "lb"]);
  });
});

describe("parseGlobalFlags: --top validation errors (exit 2)", () => {
  it("bare --top (missing value) exits 2", (t) => {
    t.after(restoreMocks);
    const s = captureExit();
    parseGlobalFlags(["lb", "--top"]);
    assert.equal(s.code, 2);
    assert.ok(s.errors.some((e) => e.includes("Error: --top requires a positive integer")), `got: ${s.errors.join("; ")}`);
  });

  it("--top 0 exits 2", (t) => {
    t.after(restoreMocks);
    const s = captureExit();
    parseGlobalFlags(["lb", "--top", "0"]);
    assert.equal(s.code, 2);
    assert.ok(s.errors.some((e) => e.includes("Error: --top requires a positive integer")));
  });

  it("--top with a non-integer value exits 2", (t) => {
    t.after(restoreMocks);
    const s = captureExit();
    parseGlobalFlags(["lb", "--top", "abc"]);
    assert.equal(s.code, 2);
    assert.ok(s.errors.some((e) => e.includes("Error: --top requires a positive integer")));
  });

  it("--top with a decimal value exits 2", (t) => {
    t.after(restoreMocks);
    const s = captureExit();
    parseGlobalFlags(["lb", "--top", "2.5"]);
    assert.equal(s.code, 2);
    assert.ok(s.errors.some((e) => e.includes("Error: --top requires a positive integer")));
  });

  it("--top followed by another flag counts as a missing value", (t) => {
    t.after(restoreMocks);
    const s = captureExit();
    parseGlobalFlags(["lb", "--top", "--json"]);
    assert.equal(s.code, 2);
    assert.ok(s.errors.some((e) => e.includes("Error: --top requires a positive integer")));
  });
});
