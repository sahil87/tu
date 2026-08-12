import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

// ---------------------------------------------------------------------------
// Toolkit `update` standard conformance (shll standards update):
//   - `tu update --help` MUST print help advertising the literal
//     `--skip-brew-update` flag and exit 0 WITHOUT running a real update —
//     `shll update`'s flag-discovery probe invokes exactly this and must
//     neither mutate state nor miss the flag substring
//   - the `brew upgrade` call MUST NOT carry a short hard timeout — killing
//     brew mid-transaction corrupts the keg mid-swap (the standard's cited
//     2026-07-19 incident)
//   - the `brew upgrade` call MUST carry HOMEBREW_NO_ASK=1 in its child env —
//     Homebrew 6 made ask mode the default, and with stdio: "inherit" both
//     fds are TTYs so the "Do you want to proceed? [y/n]" prompt would block
//     the update. The env var (not the --no-ask flag) is version-proof:
//     Homebrew < 6 harmlessly ignores it.
//
// The --help contract runs the real CLI as a subprocess (pattern per
// cli-exit-codes.test.ts); the short-circuit returns before runUpdate, so the
// test has no dependency on brew being installed. The no-timeout posture is
// pinned at the source level (runUpdate calls a statically-imported execSync
// that ESM/tsx cannot intercept — same constraint documented in
// cli-skip-brew-update-flag.test.ts).
// ---------------------------------------------------------------------------

const __dirname = dirname(fileURLToPath(import.meta.url));
const CLI = join(__dirname, "..", "cli.ts");

function runCli(args: string[]): { status: number | null; stderr: string; stdout: string } {
  const r = spawnSync("npx", ["tsx", CLI, ...args], { encoding: "utf-8" });
  return { status: r.status, stderr: r.stderr, stdout: r.stdout };
}

describe("update --help: prints help instead of running the update", () => {
  it("exits 0 and stdout contains the literal --skip-brew-update", () => {
    const r = runCli(["update", "--help"]);
    assert.equal(r.status, 0);
    assert.ok(r.stdout.includes("--skip-brew-update"), `stdout must advertise --skip-brew-update; got: ${r.stdout}`);
    assert.ok(r.stdout.includes("Usage: tu"), "stdout carries the full help text");
  });

  it("-h alias exits 0 with the same help", () => {
    const r = runCli(["update", "-h"]);
    assert.equal(r.status, 0);
    assert.ok(r.stdout.includes("--skip-brew-update"), `stdout: ${r.stdout}`);
  });
});

describe("brew upgrade: no hard timeout (source-level pin)", () => {
  it("the `brew upgrade tu` execSync call site carries no timeout option", () => {
    const source = readFileSync(CLI, "utf-8");
    const callSites = source.match(/execSync\(\s*"brew upgrade tu"[^)]*\)/g);
    assert.ok(callSites && callSites.length === 1, "expected exactly one `brew upgrade tu` call site");
    assert.ok(
      !callSites[0].includes("timeout"),
      `brew upgrade MUST NOT carry a timeout (kills brew mid-transaction); call site: ${callSites[0]}`,
    );
  });
});

describe("brew upgrade: Homebrew 6 ask mode suppressed (source-level pin)", () => {
  it("the `brew upgrade tu` execSync call site carries HOMEBREW_NO_ASK in env and keeps stdio: \"inherit\"", () => {
    const source = readFileSync(CLI, "utf-8");
    const callSites = source.match(/execSync\(\s*"brew upgrade tu"[^)]*\)/g);
    assert.ok(callSites && callSites.length === 1, "expected exactly one `brew upgrade tu` call site");
    assert.ok(
      callSites[0].includes('HOMEBREW_NO_ASK: "1"'),
      `brew upgrade MUST carry HOMEBREW_NO_ASK: "1" in its env (Homebrew 6 ask mode would block the update); call site: ${callSites[0]}`,
    );
    assert.ok(
      callSites[0].includes('stdio: "inherit"'),
      `brew upgrade MUST keep stdio: "inherit" (progress visible, Ctrl-C works); call site: ${callSites[0]}`,
    );
  });
});
