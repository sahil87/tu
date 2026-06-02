import { describe, it } from "node:test";
import assert from "node:assert/strict";

// The producer is a standalone `.mjs` build script (run under plain `node` in
// CI). tsx can import a `.mjs` from a `.ts` test, so we exercise the pure
// `buildHelpDoc` directly — no build/exec needed, and the doc-assembly logic
// lives in exactly one place (the producer), not duplicated here.
import { buildHelpDoc } from "../../../../scripts/help-dump.mjs";

// A representative captured `tu --help` (FULL_HELP). The test asserts the
// builder treats this text byte-for-byte and derives the contract fields.
const HELP_TEXT = `Usage: tu [source] [period] [display]

Sources: cc (Claude Code), codex/co (Codex), oc (OpenCode), all (default)
Periods: d/daily (default), m/monthly
Display: (bare) = snapshot, h/history = history

Flags:
  --json               Output data as JSON (data commands only)
  --no-color           Disable ANSI color output`;

describe("buildHelpDoc", () => {
  const doc = buildHelpDoc({
    name: "tu",
    version: "0.4.14",
    description: "AI coding assistant cost tracking CLI",
    helpText: HELP_TEXT,
  });

  it("stamps the frozen contract scalars", () => {
    assert.equal(doc.schema_version, 1);
    assert.equal(doc.tool, "tu");
    assert.equal(doc.version, "0.4.14");
  });

  it("emits a Z-suffixed ISO-8601 captured_at", () => {
    assert.equal(typeof doc.captured_at, "string");
    assert.ok(doc.captured_at.endsWith("Z"), "captured_at must be Z-suffixed UTC");
    assert.ok(!Number.isNaN(Date.parse(doc.captured_at)), "captured_at must be parseable");
  });

  it("builds the root Node with the expected identity fields", () => {
    assert.equal(doc.root.name, "tu");
    assert.equal(doc.root.path, "tu");
    assert.equal(doc.root.short, "AI coding assistant cost tracking CLI");
    assert.equal(doc.root.usage, "Usage: tu [source] [period] [display]");
  });

  it("carries the raw help text byte-for-byte", () => {
    assert.equal(doc.root.text, HELP_TEXT);
    assert.ok(doc.root.text.includes("Usage: tu"));
  });

  it("is flat — root.commands is an empty array", () => {
    assert.deepEqual(doc.root.commands, []);
  });

  it("falls back to literal 'tu' when name is absent", () => {
    const d = buildHelpDoc({ name: undefined, version: "9.9.9", description: "x", helpText: HELP_TEXT });
    assert.equal(d.tool, "tu");
  });

  it("falls back to the first non-empty help line when description is absent", () => {
    const d = buildHelpDoc({ name: "tu", version: "9.9.9", description: undefined, helpText: HELP_TEXT });
    assert.equal(d.root.short, "Usage: tu [source] [period] [display]");
  });

  it("produces a document that round-trips through JSON", () => {
    const round = JSON.parse(JSON.stringify(doc));
    assert.deepEqual(round, doc);
  });
});
