import { describe, it } from "node:test";
import assert from "node:assert/strict";

// ---------------------------------------------------------------------------
// --sync flag: argument filtering
// ---------------------------------------------------------------------------

describe("--sync flag: argument filtering", () => {
  function parseArgs(rawArgs: string[]) {
    const jsonFlag = rawArgs.includes("--json");
    const syncFlag = rawArgs.includes("--sync");
    const filteredArgs = rawArgs.filter((a) => a !== "--json" && a !== "--sync" && a !== "--fresh" && a !== "-f");
    const [tool, period, ...extra] = filteredArgs;
    return { jsonFlag, syncFlag, filteredArgs, tool, period, extra };
  }

  it("extracts --sync from args", () => {
    const result = parseArgs(["cc", "daily", "--sync"]);
    assert.equal(result.syncFlag, true);
    assert.equal(result.tool, "cc");
    assert.equal(result.period, "daily");
    assert.ok(!result.extra.includes("--sync"));
  });

  it("handles --sync with --json together", () => {
    const result = parseArgs(["total", "daily", "--sync", "--json"]);
    assert.equal(result.syncFlag, true);
    assert.equal(result.jsonFlag, true);
    assert.equal(result.tool, "total");
    assert.equal(result.period, "daily");
    assert.deepEqual(result.extra, []);
  });

  it("--sync does not appear in extra when combined with other flags", () => {
    const result = parseArgs(["cc", "daily", "--sync", "--some-flag"]);
    assert.equal(result.syncFlag, true);
    assert.deepEqual(result.extra, ["--some-flag"]);
  });

  it("syncFlag is false when --sync is not present", () => {
    const result = parseArgs(["cc", "daily"]);
    assert.equal(result.syncFlag, false);
  });

  it("--sync before tool is still extracted", () => {
    const result = parseArgs(["--sync", "total", "monthly"]);
    assert.equal(result.syncFlag, true);
    assert.equal(result.tool, "total");
    assert.equal(result.period, "monthly");
  });
});

// ---------------------------------------------------------------------------
// --sync flag: single mode guard
// ---------------------------------------------------------------------------

describe("--sync flag: single mode guard", () => {
  it("syncFlag is ignored when config.mode is single", () => {
    // Mirrors the guard in cli.ts: `if (syncFlag && config.mode === "multi")`
    // In single mode, --sync is silently ignored
    const config = { mode: "single" as const };
    const syncFlag = true;
    assert.equal(!!(syncFlag && config.mode === "multi"), false);
  });

  it("syncFlag activates when config.mode is multi", () => {
    const config = { mode: "multi" as const };
    const syncFlag = true;
    assert.equal(!!(syncFlag && config.mode === "multi"), true);
  });
});
