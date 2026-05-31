import { describe, it } from "node:test";
import assert from "node:assert/strict";
import type { execSync } from "node:child_process";

import { runUpdate } from "../cli.js";

// A fake execSync that records the commands it receives and returns canned
// output for the `brew info` parse step so runUpdate can proceed to upgrade.
function makeFakeExec(): { exec: typeof execSync; calls: string[] } {
  const calls: string[] = [];
  const exec = ((command: string) => {
    calls.push(command);
    if (command.startsWith("brew info")) {
      // Return a version different from PKG_VERSION so the up-to-date
      // short-circuit does not fire and `brew upgrade` is reached.
      return Buffer.from(
        JSON.stringify({ formulae: [{ versions: { stable: "999.999.999" } }] }),
      );
    }
    return Buffer.from("");
  }) as unknown as typeof execSync;
  return { exec, calls };
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
    const { exec, calls } = makeFakeExec();
    withSilencedConsole(() => {
      runUpdate({ skipBrewUpdate: true, exec, installedViaHomebrew: true });
    });

    assert.ok(
      !calls.some((c) => c.startsWith("brew update")),
      "brew update should NOT be invoked when --skip-brew-update is set",
    );
    assert.ok(
      calls.some((c) => c.startsWith("brew upgrade")),
      "brew upgrade should still be invoked",
    );
    // The version check still runs.
    assert.ok(calls.some((c) => c.startsWith("brew info")));
  });

  it("runs 'brew update' by default (flag absent)", () => {
    const { exec, calls } = makeFakeExec();
    withSilencedConsole(() => {
      runUpdate({ exec, installedViaHomebrew: true });
    });

    assert.ok(
      calls.some((c) => c.startsWith("brew update")),
      "brew update should be invoked by default",
    );
    assert.ok(calls.some((c) => c.startsWith("brew upgrade")));
  });
});
