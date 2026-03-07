import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { parseGlobalFlags, parseDataArgs } from "../cli.js";

describe("--by-machine flag parsing", () => {
  it("sets byMachineFlag to true when present", () => {
    const result = parseGlobalFlags(["cc", "h", "--by-machine"]);
    assert.equal(result.byMachineFlag, true);
    assert.deepEqual(result.filteredArgs, ["cc", "h"]);
  });

  it("defaults byMachineFlag to false when absent", () => {
    const result = parseGlobalFlags(["cc", "h"]);
    assert.equal(result.byMachineFlag, false);
  });

  it("filters --by-machine from positional args", () => {
    const result = parseGlobalFlags(["--by-machine", "cc", "dh"]);
    assert.equal(result.byMachineFlag, true);
    assert.deepEqual(result.filteredArgs, ["cc", "dh"]);
  });

  it("works alongside other flags", () => {
    const result = parseGlobalFlags(["cc", "--by-machine", "--json", "--fresh"]);
    assert.equal(result.byMachineFlag, true);
    assert.equal(result.jsonFlag, true);
    assert.equal(result.freshFlag, true);
    assert.deepEqual(result.filteredArgs, ["cc"]);
  });
});

describe("--by-machine incompatible with all-tools history pivot", () => {
  it("all-tools history is detected from parsed args", () => {
    // The warning logic in main() checks: source === "all" && display === "history"
    // Verify parseDataArgs resolves these correctly for the incompatible combos
    const dh = parseDataArgs(["h"]);
    assert.equal(dh.source, "all");
    assert.equal(dh.display, "history");

    const mh = parseDataArgs(["mh"]);
    assert.equal(mh.source, "all");
    assert.equal(mh.display, "history");
  });

  it("single-tool history is NOT the incompatible combo", () => {
    const ccH = parseDataArgs(["cc", "h"]);
    assert.equal(ccH.source, "cc");
    assert.equal(ccH.display, "history");
    // source !== "all" so --by-machine is compatible
  });

  it("all-tools snapshot is NOT the incompatible combo", () => {
    const snapshot = parseDataArgs([]);
    assert.equal(snapshot.source, "all");
    assert.equal(snapshot.display, "snapshot");
    // display !== "history" so --by-machine is compatible
  });
});
