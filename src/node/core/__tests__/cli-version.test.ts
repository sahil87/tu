import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

// ---------------------------------------------------------------------------
// Toolkit `version` standard conformance (shll standards version):
//   - `--version` MUST exit 0 with the version on stdout
//   - the version token MUST appear on the first non-empty stdout line, in the
//     RECOMMENDED `<tool> version vX.Y.Z` shape
//   - no diagnostics: stderr is empty on success
//
// Runs the real CLI as a subprocess (via tsx) — the same machine-observable
// contract `shll version` composes over. Pattern per cli-exit-codes.test.ts.
// ---------------------------------------------------------------------------

const __dirname = dirname(fileURLToPath(import.meta.url));
const CLI = join(__dirname, "..", "cli.ts");

function runCli(args: string[]): { status: number | null; stderr: string; stdout: string } {
  const r = spawnSync("npx", ["tsx", CLI, ...args], { encoding: "utf-8" });
  return { status: r.status, stderr: r.stderr, stdout: r.stdout };
}

// RECOMMENDED shape from the standard: `<tool> version vX.Y.Z`, and the
// version token itself matches v\d+(\.\d+)*.
const VERSION_LINE_RE = /^tu version v\d+(\.\d+)*$/;

function firstNonEmptyLine(s: string): string {
  return s.split("\n").find((l) => l.trim() !== "") ?? "";
}

describe("--version: toolkit version-standard contract", () => {
  it("exits 0 with `tu version vX.Y.Z` as the first non-empty stdout line, stderr empty", () => {
    const r = runCli(["--version"]);
    assert.equal(r.status, 0);
    const line = firstNonEmptyLine(r.stdout);
    assert.match(line, VERSION_LINE_RE, `first non-empty stdout line: ${JSON.stringify(line)}`);
    assert.equal(r.stderr, "", `stderr must be empty; got: ${JSON.stringify(r.stderr)}`);
  });

  it("-V alias exits 0 with the version line on stdout", () => {
    const r = runCli(["-V"]);
    assert.equal(r.status, 0);
    assert.match(firstNonEmptyLine(r.stdout), VERSION_LINE_RE);
  });

  it("-v alias exits 0 with the version line on stdout", () => {
    const r = runCli(["-v"]);
    assert.equal(r.status, 0);
    assert.match(firstNonEmptyLine(r.stdout), VERSION_LINE_RE);
  });
});
