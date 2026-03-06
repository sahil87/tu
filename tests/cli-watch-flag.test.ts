import { describe, it, mock } from "node:test";
import assert from "node:assert/strict";
import { parseGlobalFlags } from "../src/cli.js";

// ---------------------------------------------------------------------------
// Helpers: intercept process.exit to test error paths without killing the test
// ---------------------------------------------------------------------------

function captureExit(): { code: number | null; errors: string[] } {
  const state = { code: null as number | null, errors: [] as string[] };
  mock.method(process, "exit", ((code: number) => { state.code = code; }) as never);
  mock.method(console, "error", ((...args: unknown[]) => { state.errors.push(args.map(String).join(" ")); }) as never);
  return state;
}

function restoreMocks() {
  mock.restoreAll();
}

// ---------------------------------------------------------------------------
// --watch / -w flag extraction
// ---------------------------------------------------------------------------

describe("--watch / -w flag extraction", () => {
  it("extracts --watch from args", () => {
    const r = parseGlobalFlags(["cc", "h", "--watch"]);
    assert.equal(r.watchFlag, true);
    assert.deepEqual(r.filteredArgs, ["cc", "h"]);
  });

  it("extracts -w short form", () => {
    const r = parseGlobalFlags(["-w"]);
    assert.equal(r.watchFlag, true);
    assert.deepEqual(r.filteredArgs, []);
  });

  it("watchFlag is false when not present", () => {
    const r = parseGlobalFlags(["cc", "daily"]);
    assert.equal(r.watchFlag, false);
  });

  it("--watch before source is extracted", () => {
    const r = parseGlobalFlags(["--watch", "cc", "h"]);
    assert.equal(r.watchFlag, true);
    assert.deepEqual(r.filteredArgs, ["cc", "h"]);
  });
});

describe("--interval / -i extraction", () => {
  it("extracts --interval value", () => {
    const r = parseGlobalFlags(["cc", "--watch", "--interval", "30"]);
    assert.equal(r.watchInterval, 30);
    assert.deepEqual(r.filteredArgs, ["cc"]);
  });

  it("extracts -i short form value", () => {
    const r = parseGlobalFlags(["cc", "-w", "-i", "60"]);
    assert.equal(r.watchInterval, 60);
  });

  it("defaults to 10 when --interval not present", () => {
    const r = parseGlobalFlags(["cc", "--watch"]);
    assert.equal(r.watchInterval, 10);
  });

  it("errors on interval below minimum (5)", (t) => {
    t.after(restoreMocks);
    const s = captureExit();
    parseGlobalFlags(["cc", "-w", "-i", "3"]);
    assert.equal(s.code, 1);
    assert.ok(s.errors.some(e => e.includes("minimum is 5")));
  });

  it("errors on interval above maximum (3600)", (t) => {
    t.after(restoreMocks);
    const s = captureExit();
    parseGlobalFlags(["cc", "-w", "-i", "7200"]);
    assert.equal(s.code, 1);
    assert.ok(s.errors.some(e => e.includes("maximum is 3600")));
  });

  it("errors on missing interval value with --watch", (t) => {
    t.after(restoreMocks);
    const s = captureExit();
    parseGlobalFlags(["cc", "-w", "-i"]);
    assert.equal(s.code, 1);
    assert.ok(s.errors.some(e => e.includes("requires a numeric value")));
  });

  it("errors on non-numeric interval value with --watch", (t) => {
    t.after(restoreMocks);
    const s = captureExit();
    parseGlobalFlags(["cc", "-w", "-i", "abc"]);
    assert.equal(s.code, 1);
    assert.ok(s.errors.some(e => e.includes("requires a numeric value")));
  });

  it("accepts 5 (minimum boundary)", () => {
    const r = parseGlobalFlags(["-w", "-i", "5"]);
    assert.equal(r.watchInterval, 5);
  });

  it("accepts 3600 (maximum boundary)", () => {
    const r = parseGlobalFlags(["-w", "-i", "3600"]);
    assert.equal(r.watchInterval, 3600);
  });

  it("--interval without --watch is silently ignored (no validation error)", () => {
    const r = parseGlobalFlags(["cc", "--interval", "2"]);
    assert.equal(r.watchFlag, false);
    assert.equal(r.watchInterval, 10); // default, not 2
    assert.deepEqual(r.filteredArgs, ["cc"]);
  });

  it("rejects fractional seconds", (t) => {
    t.after(restoreMocks);
    const s = captureExit();
    parseGlobalFlags(["cc", "-w", "-i", "5.5"]);
    assert.equal(s.code, 1);
    assert.ok(s.errors.some(e => e.includes("requires a numeric value")));
  });
});

describe("--watch + --json incompatibility", () => {
  it("reports error when both present", (t) => {
    t.after(restoreMocks);
    const s = captureExit();
    parseGlobalFlags(["cc", "--watch", "--json"]);
    assert.equal(s.code, 1);
    assert.ok(s.errors.some(e => e.includes("--watch and --json are incompatible")));
  });

  it("no error when only --watch", () => {
    const r = parseGlobalFlags(["cc", "--watch"]);
    assert.equal(r.watchFlag, true);
    assert.equal(r.jsonFlag, false);
  });

  it("no error when only --json", () => {
    const r = parseGlobalFlags(["cc", "--json"]);
    assert.equal(r.jsonFlag, true);
    assert.equal(r.watchFlag, false);
  });
});

describe("--watch + --fresh interaction", () => {
  it("both flags accepted without error", () => {
    const r = parseGlobalFlags(["cc", "--watch", "--fresh"]);
    assert.equal(r.watchFlag, true);
    assert.equal(r.freshFlag, true);
  });

  it("-w + -f accepted without error", () => {
    const r = parseGlobalFlags(["cc", "-w", "-f"]);
    assert.equal(r.watchFlag, true);
    assert.equal(r.freshFlag, true);
  });
});

describe("--watch + --sync interaction", () => {
  it("both flags accepted", () => {
    const r = parseGlobalFlags(["cc", "--watch", "--sync"]);
    assert.equal(r.watchFlag, true);
    assert.equal(r.syncFlag, true);
  });
});

describe("--no-color flag extraction", () => {
  it("extracts --no-color from args", () => {
    const r = parseGlobalFlags(["cc", "--no-color"]);
    assert.equal(r.noColorFlag, true);
    assert.deepEqual(r.filteredArgs, ["cc"]);
  });

  it("noColorFlag is false when not present", () => {
    const r = parseGlobalFlags(["cc"]);
    assert.equal(r.noColorFlag, false);
  });

  it("--no-color combined with --watch", () => {
    const r = parseGlobalFlags(["cc", "--watch", "--no-color"]);
    assert.equal(r.noColorFlag, true);
    assert.equal(r.watchFlag, true);
    assert.deepEqual(r.filteredArgs, ["cc"]);
  });
});

describe("--no-rain flag extraction", () => {
  it("extracts --no-rain from args", () => {
    const r = parseGlobalFlags(["cc", "--no-rain"]);
    assert.equal(r.noRainFlag, true);
    assert.deepEqual(r.filteredArgs, ["cc"]);
  });

  it("noRainFlag is false when not present", () => {
    const r = parseGlobalFlags(["cc"]);
    assert.equal(r.noRainFlag, false);
  });

  it("--no-rain combined with --watch", () => {
    const r = parseGlobalFlags(["cc", "--watch", "--no-rain"]);
    assert.equal(r.noRainFlag, true);
    assert.equal(r.watchFlag, true);
    assert.deepEqual(r.filteredArgs, ["cc"]);
  });

  it("--no-rain without --watch is accepted (silently ignored)", () => {
    const r = parseGlobalFlags(["cc", "--no-rain"]);
    assert.equal(r.noRainFlag, true);
    assert.equal(r.watchFlag, false);
  });
});

describe("all flag combinations", () => {
  it("-w -i 15 with source", () => {
    const r = parseGlobalFlags(["cc", "-w", "-i", "15"]);
    assert.equal(r.watchFlag, true);
    assert.equal(r.watchInterval, 15);
    assert.deepEqual(r.filteredArgs, ["cc"]);
  });

  it("--watch --interval 60 --sync --fresh with source and display", () => {
    const r = parseGlobalFlags(["all", "mh", "--watch", "--interval", "60", "--sync", "--fresh"]);
    assert.equal(r.watchFlag, true);
    assert.equal(r.watchInterval, 60);
    assert.equal(r.syncFlag, true);
    assert.equal(r.freshFlag, true);
    assert.deepEqual(r.filteredArgs, ["all", "mh"]);
  });
});
