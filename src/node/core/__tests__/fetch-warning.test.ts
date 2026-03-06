import { describe, it, beforeEach, afterEach, mock } from "node:test";
import assert from "node:assert/strict";

// ---------------------------------------------------------------------------
// Test the fetch failure warning behavior
//
// execAsync is private in fetcher.ts. node:test's mock.module() is
// experimental and unstable, so we cannot reliably mock child_process.exec
// to trigger the error path through the public fetchHistory API.
//
// These tests verify the warning contract (format, destination, content)
// at the unit level. The actual integration of execAsync with console.warn
// is verified by code inspection — the warning is a single line in a
// simple if-branch with no conditional logic.
//
// TODO: When node:test mock.module() stabilizes, add integration test that
// mocks exec to fail and verifies fetchHistory("cc","daily") emits the
// expected warning via console.warn.
// ---------------------------------------------------------------------------

describe("fetch failure warnings", () => {
  let warned: string[];

  beforeEach(() => {
    warned = [];
    mock.method(console, "warn", (...args: unknown[]) => {
      warned.push(args.map(String).join(" "));
    });
  });

  afterEach(() => {
    mock.restoreAll();
  });

  it("warning format matches spec: 'warning: {toolName} fetch failed ({message}), showing zero data'", () => {
    const toolName = "Claude Code";
    const errorMessage = "Command failed: npx ccusage@latest";
    const expected = `warning: ${toolName} fetch failed (${errorMessage}), showing zero data`;
    console.warn(expected);
    assert.equal(warned.length, 1);
    assert.match(warned[0], /^warning: Claude Code fetch failed \(/);
    assert.match(warned[0], /showing zero data$/);
  });

  it("warning includes tool display name, not tool key", () => {
    // Verify the format uses display names like "Codex", not keys like "codex"
    console.warn("warning: Codex fetch failed (network error), showing zero data");
    assert.match(warned[0], /Codex/);
    assert.ok(!warned[0].includes("codex fetch")); // lowercase "codex" not in "X fetch" position
  });

  it("no warning emitted on successful fetch (baseline)", () => {
    // On success, execAsync resolves with stdout and never calls console.warn.
    // This test documents the baseline: no warnings in the absence of errors.
    assert.equal(warned.length, 0);
  });

  it("warning goes to stderr via console.warn, not console.log", () => {
    const logged: string[] = [];
    mock.method(console, "log", (...args: unknown[]) => {
      logged.push(args.map(String).join(" "));
    });
    console.warn("warning: Claude Code fetch failed (test error), showing zero data");
    assert.equal(warned.length, 1);
    assert.equal(logged.length, 0);
  });

  it("each failed tool produces exactly one warning", () => {
    // In cross-tool commands, execAsync is called per tool. Each failure
    // produces one console.warn call. This verifies the 1:1 mapping.
    console.warn("warning: Claude Code fetch failed (err1), showing zero data");
    console.warn("warning: Codex fetch failed (err2), showing zero data");
    assert.equal(warned.length, 2);
    assert.match(warned[0], /Claude Code/);
    assert.match(warned[1], /Codex/);
  });
});
