import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { parseGlobalFlags } from "../src/cli.js";

// ---------------------------------------------------------------------------
// --fresh / -f flag: argument filtering (using real parseGlobalFlags)
// ---------------------------------------------------------------------------

describe("--fresh flag: argument filtering", () => {
  it("extracts --fresh from args", () => {
    const result = parseGlobalFlags(["cc", "h", "--fresh"]);
    assert.equal(result.freshFlag, true);
    assert.deepEqual(result.filteredArgs, ["cc", "h"]);
  });

  it("extracts -f short form from args", () => {
    const result = parseGlobalFlags(["-f"]);
    assert.equal(result.freshFlag, true);
    assert.deepEqual(result.filteredArgs, []);
  });

  it("handles --fresh with --json together", () => {
    const result = parseGlobalFlags(["cc", "h", "--fresh", "--json"]);
    assert.equal(result.freshFlag, true);
    assert.equal(result.jsonFlag, true);
    assert.deepEqual(result.filteredArgs, ["cc", "h"]);
  });

  it("handles --fresh with --sync together", () => {
    const result = parseGlobalFlags(["h", "--fresh", "--sync"]);
    assert.equal(result.freshFlag, true);
    assert.equal(result.syncFlag, true);
    assert.deepEqual(result.filteredArgs, ["h"]);
  });

  it("handles all three flags together", () => {
    const result = parseGlobalFlags(["cc", "mh", "--fresh", "--json", "--sync"]);
    assert.equal(result.freshFlag, true);
    assert.equal(result.jsonFlag, true);
    assert.equal(result.syncFlag, true);
    assert.deepEqual(result.filteredArgs, ["cc", "mh"]);
  });

  it("freshFlag is false when neither --fresh nor -f is present", () => {
    const result = parseGlobalFlags(["cc", "daily"]);
    assert.equal(result.freshFlag, false);
  });

  it("--fresh before source is still extracted", () => {
    const result = parseGlobalFlags(["--fresh", "cc", "h"]);
    assert.equal(result.freshFlag, true);
    assert.deepEqual(result.filteredArgs, ["cc", "h"]);
  });

  it("-f before source is still extracted", () => {
    const result = parseGlobalFlags(["-f", "cc"]);
    assert.equal(result.freshFlag, true);
    assert.deepEqual(result.filteredArgs, ["cc"]);
  });

  it("handles --fresh with --watch together (no error)", () => {
    const result = parseGlobalFlags(["cc", "--fresh", "--watch"]);
    assert.equal(result.freshFlag, true);
    assert.equal(result.watchFlag, true);
    assert.deepEqual(result.filteredArgs, ["cc"]);
  });

  it("handles -f with -w together (no error)", () => {
    const result = parseGlobalFlags(["cc", "-f", "-w"]);
    assert.equal(result.freshFlag, true);
    assert.equal(result.watchFlag, true);
    assert.deepEqual(result.filteredArgs, ["cc"]);
  });
});
