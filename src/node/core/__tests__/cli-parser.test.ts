import { describe, it, mock } from "node:test";
import assert from "node:assert/strict";

import { parseDataArgs, parseGlobalFlags } from "../cli.js";

function captureExit(): { code: number | null; errors: string[] } {
  const state = { code: null as number | null, errors: [] as string[] };
  mock.method(process, "exit", ((code: number) => { state.code = code; }) as never);
  mock.method(console, "error", ((...args: unknown[]) => { state.errors.push(args.map(String).join(" ")); }) as never);
  return state;
}

function restoreMocks() {
  mock.restoreAll();
}

describe("parseDataArgs", () => {
  describe("source detection", () => {
    it("recognizes cc as source", () => {
      const result = parseDataArgs(["cc"]);
      assert.equal(result.source, "cc");
    });

    it("recognizes codex as source", () => {
      const result = parseDataArgs(["codex", "m"]);
      assert.equal(result.source, "codex");
      assert.equal(result.period, "monthly");
    });

    it("resolves co alias to codex", () => {
      const result = parseDataArgs(["co", "h"]);
      assert.equal(result.source, "codex");
      assert.equal(result.display, "history");
    });

    it("recognizes oc as source", () => {
      const result = parseDataArgs(["oc"]);
      assert.equal(result.source, "oc");
    });

    it("recognizes all as explicit source", () => {
      const result = parseDataArgs(["all", "mh"]);
      assert.equal(result.source, "all");
      assert.equal(result.period, "monthly");
      assert.equal(result.display, "history");
    });

    it("defaults source to all when first arg is not a source", () => {
      const result = parseDataArgs(["m"]);
      assert.equal(result.source, "all");
      assert.equal(result.period, "monthly");
    });
  });

  describe("period parsing", () => {
    it("parses d as daily", () => {
      const result = parseDataArgs(["cc", "d"]);
      assert.equal(result.period, "daily");
    });

    it("parses daily as daily", () => {
      const result = parseDataArgs(["cc", "daily"]);
      assert.equal(result.period, "daily");
    });

    it("parses m as monthly", () => {
      const result = parseDataArgs(["cc", "m"]);
      assert.equal(result.period, "monthly");
    });

    it("parses monthly as monthly", () => {
      const result = parseDataArgs(["cc", "monthly"]);
      assert.equal(result.period, "monthly");
    });

    it("defaults period to daily", () => {
      const result = parseDataArgs(["cc"]);
      assert.equal(result.period, "daily");
    });
  });

  describe("display parsing", () => {
    it("parses h as history", () => {
      const result = parseDataArgs(["h"]);
      assert.equal(result.display, "history");
      assert.equal(result.source, "all");
    });

    it("parses history as history", () => {
      const result = parseDataArgs(["cc", "history"]);
      assert.equal(result.display, "history");
    });

    it("defaults display to snapshot", () => {
      const result = parseDataArgs(["cc"]);
      assert.equal(result.display, "snapshot");
    });
  });

  describe("combined modifiers", () => {
    it("parses dh as daily + history", () => {
      const result = parseDataArgs(["dh"]);
      assert.equal(result.period, "daily");
      assert.equal(result.display, "history");
      assert.equal(result.source, "all");
    });

    it("parses mh as monthly + history", () => {
      const result = parseDataArgs(["mh"]);
      assert.equal(result.period, "monthly");
      assert.equal(result.display, "history");
      assert.equal(result.source, "all");
    });

    it("parses source + mh", () => {
      const result = parseDataArgs(["cc", "mh"]);
      assert.equal(result.source, "cc");
      assert.equal(result.period, "monthly");
      assert.equal(result.display, "history");
    });

    it("parses source + dh", () => {
      const result = parseDataArgs(["oc", "dh"]);
      assert.equal(result.source, "oc");
      assert.equal(result.period, "daily");
      assert.equal(result.display, "history");
    });
  });

  describe("separate period + display equivalence", () => {
    it("tu cc d h is equivalent to tu cc dh", () => {
      const separate = parseDataArgs(["cc", "d", "h"]);
      const combined = parseDataArgs(["cc", "dh"]);
      assert.deepEqual(separate, combined);
    });

    it("tu cc m h is equivalent to tu cc mh", () => {
      const separate = parseDataArgs(["cc", "m", "h"]);
      const combined = parseDataArgs(["cc", "mh"]);
      assert.deepEqual(separate, combined);
    });

    it("tu d h is equivalent to tu dh (no source)", () => {
      const separate = parseDataArgs(["d", "h"]);
      const combined = parseDataArgs(["dh"]);
      assert.deepEqual(separate, combined);
    });
  });

  describe("defaults (empty args)", () => {
    it("returns all/daily/snapshot for empty args", () => {
      const result = parseDataArgs([]);
      assert.equal(result.source, "all");
      assert.equal(result.period, "daily");
      assert.equal(result.display, "snapshot");
    });
  });

  describe("error handling", () => {
    it("throws on unknown argument", () => {
      assert.throws(() => parseDataArgs(["foo"]), /Unknown argument: foo/);
    });

    it("throws on old-style total", () => {
      assert.throws(() => parseDataArgs(["total"]), /Unknown argument: total/);
    });

    it("throws on old-style total-history", () => {
      assert.throws(() => parseDataArgs(["total-history"]), /Unknown argument: total-history/);
    });

    it("throws on unknown arg after valid source", () => {
      assert.throws(() => parseDataArgs(["cc", "xyz"]), /Unknown argument: xyz/);
    });
  });
});

// ---------------------------------------------------------------------------
// parseGlobalFlags: --csv / --md parsing + conflict matrix
// ---------------------------------------------------------------------------

describe("parseGlobalFlags: --csv / --md extraction", () => {
  it("extracts --csv and reports outputFormat: csv", () => {
    const r = parseGlobalFlags(["cc", "--csv"]);
    assert.equal(r.outputFormat, "csv");
    assert.deepEqual(r.filteredArgs, ["cc"]);
  });

  it("extracts --md and reports outputFormat: md", () => {
    const r = parseGlobalFlags(["m", "--md"]);
    assert.equal(r.outputFormat, "md");
    assert.deepEqual(r.filteredArgs, ["m"]);
  });

  it("--json is preserved as outputFormat: json", () => {
    const r = parseGlobalFlags(["cc", "--json"]);
    assert.equal(r.outputFormat, "json");
  });

  it("default outputFormat is table", () => {
    const r = parseGlobalFlags(["cc"]);
    assert.equal(r.outputFormat, "table");
  });
});

describe("parseGlobalFlags: format-flag conflicts", () => {
  it("--json + --csv is rejected", (t) => {
    t.after(restoreMocks);
    const s = captureExit();
    parseGlobalFlags(["cc", "--json", "--csv"]);
    assert.equal(s.code, 1);
    assert.ok(s.errors.some((e) => e.includes("--json and --csv are incompatible")), `got errors: ${s.errors.join("; ")}`);
  });

  it("--csv + --md is rejected", (t) => {
    t.after(restoreMocks);
    const s = captureExit();
    parseGlobalFlags(["cc", "--csv", "--md"]);
    assert.equal(s.code, 1);
    assert.ok(s.errors.some((e) => e.includes("--csv and --md are incompatible")));
  });

  it("--json + --md is rejected", (t) => {
    t.after(restoreMocks);
    const s = captureExit();
    parseGlobalFlags(["cc", "--json", "--md"]);
    assert.equal(s.code, 1);
    assert.ok(s.errors.some((e) => e.includes("--json and --md are incompatible")));
  });

  it("--csv + --watch is rejected", (t) => {
    t.after(restoreMocks);
    const s = captureExit();
    parseGlobalFlags(["cc", "--csv", "--watch"]);
    assert.equal(s.code, 1);
    assert.ok(s.errors.some((e) => e.includes("--watch and --csv are incompatible")));
  });

  it("--md + --watch is rejected", (t) => {
    t.after(restoreMocks);
    const s = captureExit();
    parseGlobalFlags(["cc", "--md", "--watch"]);
    assert.equal(s.code, 1);
    assert.ok(s.errors.some((e) => e.includes("--watch and --md are incompatible")));
  });

  it("--csv + -w (short form) is rejected", (t) => {
    t.after(restoreMocks);
    const s = captureExit();
    parseGlobalFlags(["cc", "--csv", "-w"]);
    assert.equal(s.code, 1);
    assert.ok(s.errors.some((e) => e.includes("--watch and --csv are incompatible")));
  });

  it("--md + -w (short form) is rejected", (t) => {
    t.after(restoreMocks);
    const s = captureExit();
    parseGlobalFlags(["cc", "--md", "-w"]);
    assert.equal(s.code, 1);
    assert.ok(s.errors.some((e) => e.includes("--watch and --md are incompatible")));
  });

  it("existing --watch + --json error preserved with original wording", (t) => {
    t.after(restoreMocks);
    const s = captureExit();
    parseGlobalFlags(["cc", "--watch", "--json"]);
    assert.equal(s.code, 1);
    assert.ok(s.errors.some((e) => e.includes("--watch and --json are incompatible")));
  });
});

// ---------------------------------------------------------------------------
// parseGlobalFlags: -j alias for --json
// ---------------------------------------------------------------------------

describe("parseGlobalFlags: -j alias for --json", () => {
  it("-j resolves to outputFormat: json and jsonFlag: true", () => {
    const r = parseGlobalFlags(["cc", "-j"]);
    assert.equal(r.outputFormat, "json");
    assert.equal(r.jsonFlag, true);
    assert.deepEqual(r.filteredArgs, ["cc"]);
  });

  it("-j is filtered out of positional args", () => {
    const r = parseGlobalFlags(["-j", "cc", "mh"]);
    assert.deepEqual(r.filteredArgs, ["cc", "mh"]);
  });

  it("-j + --csv is rejected with canonical --json wording", (t) => {
    t.after(restoreMocks);
    const s = captureExit();
    parseGlobalFlags(["cc", "-j", "--csv"]);
    assert.equal(s.code, 1);
    assert.ok(s.errors.some((e) => e.includes("--json and --csv are incompatible")), `got errors: ${s.errors.join("; ")}`);
  });

  it("-j + --watch is rejected with canonical --json wording", (t) => {
    t.after(restoreMocks);
    const s = captureExit();
    parseGlobalFlags(["cc", "-j", "--watch"]);
    assert.equal(s.code, 1);
    assert.ok(s.errors.some((e) => e.includes("--watch and --json are incompatible")), `got errors: ${s.errors.join("; ")}`);
  });
});

// ---------------------------------------------------------------------------
// parseGlobalFlags: --since / -s / --until date filters
// ---------------------------------------------------------------------------

describe("parseGlobalFlags: --since / -s / --until parsing", () => {
  it("parses --since with an ISO date and filters it from args", () => {
    const r = parseGlobalFlags(["cc", "h", "--since", "2026-06-01"]);
    assert.equal(r.sinceFlag, "2026-06-01");
    assert.equal(r.untilFlag, undefined);
    assert.deepEqual(r.filteredArgs, ["cc", "h"]);
  });

  it("parses -s short alias for --since", () => {
    const r = parseGlobalFlags(["h", "-s", "2026-06-01"]);
    assert.equal(r.sinceFlag, "2026-06-01");
    assert.deepEqual(r.filteredArgs, ["h"]);
  });

  it("normalizes YYYYMMDD to ISO for --since and --until", () => {
    const r = parseGlobalFlags(["h", "--since", "20260601", "--until", "20260630"]);
    assert.equal(r.sinceFlag, "2026-06-01");
    assert.equal(r.untilFlag, "2026-06-30");
  });

  it("--until stays long-only — -u still parses as --user", () => {
    const r = parseGlobalFlags(["h", "--until", "2026-06-30", "-u", "bob"]);
    assert.equal(r.untilFlag, "2026-06-30");
    assert.equal(r.userFlag, "bob");
  });

  it("sinceFlag/untilFlag are undefined when absent", () => {
    const r = parseGlobalFlags(["cc", "h"]);
    assert.equal(r.sinceFlag, undefined);
    assert.equal(r.untilFlag, undefined);
  });

  it("accepts a well-shaped but impossible date (shape-only validation)", () => {
    const r = parseGlobalFlags(["h", "--since", "2026-13-01"]);
    assert.equal(r.sinceFlag, "2026-13-01");
  });
});

describe("parseGlobalFlags: --since / --until validation errors", () => {
  it("--since with no value exits 1", (t) => {
    t.after(restoreMocks);
    const s = captureExit();
    parseGlobalFlags(["h", "--since"]);
    assert.equal(s.code, 1);
    assert.ok(s.errors.some((e) => e.includes("--since requires a date (YYYY-MM-DD or YYYYMMDD)")), `got: ${s.errors.join("; ")}`);
  });

  it("--since with a malformed value exits 1", (t) => {
    t.after(restoreMocks);
    const s = captureExit();
    parseGlobalFlags(["h", "--since", "june"]);
    assert.equal(s.code, 1);
    assert.ok(s.errors.some((e) => e.includes("--since requires a date (YYYY-MM-DD or YYYYMMDD)")));
  });

  it("--until with a malformed value exits 1 with its own name", (t) => {
    t.after(restoreMocks);
    const s = captureExit();
    parseGlobalFlags(["h", "--until", "2026-6-1"]);
    assert.equal(s.code, 1);
    assert.ok(s.errors.some((e) => e.includes("--until requires a date (YYYY-MM-DD or YYYYMMDD)")));
  });

  it("inverted window (since > until) exits 1", (t) => {
    t.after(restoreMocks);
    const s = captureExit();
    parseGlobalFlags(["h", "--since", "2026-06-30", "--until", "2026-06-01"]);
    assert.equal(s.code, 1);
    assert.ok(s.errors.some((e) => e.includes("--since must be on or before --until")));
  });

  it("equal since and until is a valid (single-day) window", () => {
    const r = parseGlobalFlags(["h", "--since", "2026-06-15", "--until", "2026-06-15"]);
    assert.equal(r.sinceFlag, "2026-06-15");
    assert.equal(r.untilFlag, "2026-06-15");
  });
});
