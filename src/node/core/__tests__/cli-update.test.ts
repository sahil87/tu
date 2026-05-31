import { describe, it } from "node:test";
import assert from "node:assert/strict";

import { runUpdate } from "../cli.js";
import type { UpdateDeps } from "../cli.js";

// Builds an injectable brew seam that records every command and returns
// canned `brew info` output. `latest` controls the version comparison so we
// can drive the flow all the way to `brew upgrade`.
function makeDeps(latest: string): { deps: UpdateDeps; calls: string[] } {
  const calls: string[] = [];
  const deps: UpdateDeps = {
    pkgDir: "/opt/homebrew/Cellar/tu/0.0.1/libexec",
    version: "0.0.1",
    runBrew: (cmd: string) => {
      calls.push(cmd);
      if (cmd.startsWith("brew info")) {
        return Buffer.from(JSON.stringify({ formulae: [{ versions: { stable: latest } }] }));
      }
      return Buffer.from("");
    },
  };
  return { deps, calls };
}

function withSilencedConsole(fn: () => void): void {
  const origLog = console.log;
  const origError = console.error;
  console.log = () => {};
  console.error = () => {};
  try {
    fn();
  } finally {
    console.log = origLog;
    console.error = origError;
  }
}

describe("runUpdate --skip-brew-update", () => {
  it("skips 'brew update' but still runs 'brew upgrade' when skipBrewUpdate is true", () => {
    const { deps, calls } = makeDeps("0.0.2"); // newer → triggers upgrade
    withSilencedConsole(() => runUpdate(true, deps));

    assert.ok(
      !calls.some((c) => c.startsWith("brew update")),
      `'brew update' should NOT be invoked; calls were: ${calls.join(", ")}`,
    );
    assert.ok(
      calls.some((c) => c.startsWith("brew upgrade")),
      `'brew upgrade' SHOULD be invoked; calls were: ${calls.join(", ")}`,
    );
    // brew info version check still runs.
    assert.ok(calls.some((c) => c.startsWith("brew info")));
  });

  it("runs 'brew update' by default (flag absent preserves current behavior)", () => {
    const { deps, calls } = makeDeps("0.0.2");
    withSilencedConsole(() => runUpdate(false, deps));

    assert.ok(calls.some((c) => c.startsWith("brew update")), "'brew update' should run by default");
    assert.ok(calls.some((c) => c.startsWith("brew info")));
    assert.ok(calls.some((c) => c.startsWith("brew upgrade")));
  });

  it("still short-circuits when already up to date, even with --skip-brew-update", () => {
    const { deps, calls } = makeDeps("0.0.1"); // same version → no upgrade
    withSilencedConsole(() => runUpdate(true, deps));

    assert.ok(!calls.some((c) => c.startsWith("brew update")), "'brew update' should be skipped");
    assert.ok(calls.some((c) => c.startsWith("brew info")), "version check still runs");
    assert.ok(!calls.some((c) => c.startsWith("brew upgrade")), "no upgrade when up to date");
  });
});
