import { describe, it } from "node:test";
import assert from "node:assert/strict";

// The pure contract builder lives in `help-dump.ts` so it can be bundled into
// the `tu` binary (the in-binary `tu help-dump` command is what shll.ai's pull
// cron invokes) AND unit-tested directly here — no build/exec needed, and the
// doc-assembly logic lives in exactly one place, not duplicated.
import { buildHelpDoc } from "../help-dump.js";
import { runHelpDump, FULL_HELP } from "../cli.js";

// A representative captured `tu --help` (FULL_HELP). The test asserts the
// builder treats this text byte-for-byte and derives the contract fields.
const HELP_TEXT = `Usage: tu [source] [period] [display]

Sources: cc (Claude Code), codex/co (Codex), oc (OpenCode), gemini/gem (Gemini), copilot/cop (Copilot), all (default)
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

  it("falls back to the first help line when description is empty or whitespace", () => {
    // PKG_DESCRIPTION defaults to "" when no description is injected/found, and
    // `"" ?? x` keeps the empty string — short must still get a useful default.
    for (const empty of ["", "   ", "\t\n"]) {
      const d = buildHelpDoc({ name: "tu", version: "9.9.9", description: empty, helpText: HELP_TEXT });
      assert.equal(d.root.short, "Usage: tu [source] [period] [display]");
    }
  });

  it("produces a document that round-trips through JSON", () => {
    const round = JSON.parse(JSON.stringify(doc));
    assert.deepEqual(round, doc);
  });
});

// Exercise the in-binary command end-to-end (the wiring that broke: shll.ai's
// pull cron runs `tu help-dump` and requires valid JSON on stdout, NOTHING on
// stderr, exit 0). We capture the streams around runHelpDump() rather than
// exec'ing a built bundle so the test stays fast and build-independent.
describe("runHelpDump (in-binary command)", () => {
  function captureRun(): { out: string; err: string } {
    const origOut = process.stdout.write.bind(process.stdout);
    const origErr = process.stderr.write.bind(process.stderr);
    let out = "";
    let err = "";
    (process.stdout.write as unknown) = (chunk: string | Uint8Array): boolean => {
      out += typeof chunk === "string" ? chunk : Buffer.from(chunk).toString("utf-8");
      return true;
    };
    (process.stderr.write as unknown) = (chunk: string | Uint8Array): boolean => {
      err += typeof chunk === "string" ? chunk : Buffer.from(chunk).toString("utf-8");
      return true;
    };
    try {
      runHelpDump();
    } finally {
      process.stdout.write = origOut;
      process.stderr.write = origErr;
    }
    return { out, err };
  }

  it("writes the contract JSON to stdout and nothing to stderr", () => {
    const { out, err } = captureRun();
    assert.equal(err, "", "stderr MUST be empty on success (shll.ai contract §1)");
    assert.ok(out.length > 0, "stdout must be non-empty");
    const doc = JSON.parse(out); // throws if not valid JSON
    assert.equal(doc.schema_version, 1);
    assert.equal(doc.tool, "tu");
    assert.ok(typeof doc.version === "string" && doc.version.length > 0);
    assert.ok(doc.captured_at.endsWith("Z"));
    assert.equal(doc.root.name, "tu");
    assert.deepEqual(doc.root.commands, []);
  });

  it("emits root.text as FULL_HELP byte-for-byte plus the print newline", () => {
    const { out } = captureRun();
    const doc = JSON.parse(out);
    // `tu --help` does `console.log(FULL_HELP)`, which appends one newline; the
    // captured help text must match that exactly.
    assert.equal(doc.root.text, FULL_HELP + "\n");
  });

  it("terminates stdout with a single trailing newline", () => {
    const { out } = captureRun();
    assert.ok(out.endsWith("}\n"), "output should be pretty JSON + one trailing newline");
    assert.ok(!out.endsWith("}\n\n"), "no double trailing newline");
  });
});
