import { describe, it, mock } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

import { SKILL_MD } from "../skill.js";
import { runSkill } from "../cli.js";

// Canonical bundle, read directly from the repo. skill.ts's dev/tsx fallback
// reads the same file, so this also pins that fallback's path resolution.
const CANONICAL_PATH = new URL("../../../../docs/site/skill.md", import.meta.url);
const CANONICAL = readFileSync(CANONICAL_PATH, "utf8");

describe("SKILL_MD", () => {
  it("is byte-identical to docs/site/skill.md", () => {
    assert.equal(SKILL_MD, CANONICAL);
  });

  it("is within the ≤150-line budget (toolkit skill standard)", () => {
    // Count newline-terminated lines; a trailing final newline does not add a line.
    const lines = SKILL_MD.replace(/\n$/, "").split("\n");
    assert.ok(lines.length <= 150, `Expected <= 150 lines, got ${lines.length}`);
  });

  it("is static-only — no obvious dynamic markers (best-effort genre check)", () => {
    // The bundle MUST NOT bake in timestamps or environment lookups (contrast
    // run-kit context). This is a genre sanity check, not exhaustive.
    assert.ok(!/process\.env/.test(SKILL_MD), "bundle must not reference process.env");
    assert.ok(!/Date\.now|new Date|toISOString/.test(SKILL_MD), "bundle must not embed a timestamp");
    assert.ok(!/\b\d{4}-\d{2}-\d{2}T\d{2}:\d{2}/.test(SKILL_MD), "bundle must not embed an ISO datetime");
  });
});

// ---------------------------------------------------------------------------
// CLI contract: `tu skill` writes the bundle to stdout, nothing to stderr, and
// does not exit non-zero. Mock-capture pattern mirrors completions.test.ts.
// ---------------------------------------------------------------------------

interface Capture {
  stdout: string[];
  errors: string[];
  exitCode: number | null;
}

function captureIo(): Capture {
  const cap: Capture = { stdout: [], errors: [], exitCode: null };
  mock.method(process.stdout, "write", ((chunk: string | Uint8Array) => {
    cap.stdout.push(typeof chunk === "string" ? chunk : Buffer.from(chunk).toString("utf-8"));
    return true;
  }) as never);
  mock.method(process.stderr, "write", ((chunk: string | Uint8Array) => {
    cap.errors.push(typeof chunk === "string" ? chunk : Buffer.from(chunk).toString("utf-8"));
    return true;
  }) as never);
  mock.method(console, "error", ((...args: unknown[]) => {
    cap.errors.push(args.map(String).join(" "));
  }) as never);
  mock.method(process, "exit", ((code: number) => {
    cap.exitCode = code;
  }) as never);
  return cap;
}

describe("runSkill", () => {
  it("writes the bundle to stdout byte-identically, empty stderr, no failure exit", (t) => {
    t.after(() => mock.restoreAll());
    const cap = captureIo();
    runSkill();
    assert.equal(cap.stdout.join(""), CANONICAL, "stdout must be the bundle byte-for-byte");
    assert.equal(cap.errors.join(""), "", "stderr must be empty");
    assert.equal(cap.exitCode, null, "must not exit with a failure code");
  });
});
