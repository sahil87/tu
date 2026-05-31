import { describe, it, beforeEach, afterEach } from "node:test";
import assert from "node:assert/strict";

import { runUpdate } from "../cli.js";

// ---------------------------------------------------------------------------
// `tu update --skip-brew-update`
//
// Contract (shared across the 6 sibling toolkit tools): the flag must skip
// ONLY the `brew update --quiet` tap-metadata refresh. The `brew info` version
// check and `brew upgrade` must all still run. Default (flag absent) must
// invoke `brew update` as before.
//
// runUpdate takes injectable `exec` and `isHomebrew` (defaults preserve the
// real behavior) so we can observe which brew commands are issued without
// shelling out. We force `isHomebrew: () => true` to exercise the upgrade path,
// and have the fake `brew info` report a version that differs from the running
// one so the "already up to date" short-circuit does not fire.
// ---------------------------------------------------------------------------

interface Io {
  logs: string[];
  errors: string[];
  exitCode: number | null;
}

const orig = {
  log: console.log,
  error: console.error,
  exit: process.exit,
};

function captureIo(): Io {
  const io: Io = { logs: [], errors: [], exitCode: null };
  console.log = (...args: unknown[]) => io.logs.push(args.map(String).join(" "));
  console.error = (...args: unknown[]) => io.errors.push(args.map(String).join(" "));
  process.exit = ((code?: number) => {
    io.exitCode = code ?? 0;
    throw new Error(`__exit_${code ?? 0}`); // halt like the real exit would
  }) as never;
  return io;
}

function restoreIo(): void {
  console.log = orig.log;
  console.error = orig.error;
  process.exit = orig.exit;
}

// A fake `exec` that records commands and returns a `brew info` payload whose
// stable version is clearly newer than any real release, so the upgrade path
// runs (latest !== current).
function makeExec(calls: string[]) {
  return ((command: string) => {
    calls.push(command);
    if (command.startsWith("brew info")) {
      return Buffer.from(
        JSON.stringify({ formulae: [{ versions: { stable: "999.999.999" } }] }),
      );
    }
    return Buffer.from("");
  }) as never;
}

describe("runUpdate --skip-brew-update", () => {
  beforeEach(() => captureIo());
  afterEach(() => restoreIo());

  it("with the flag, does NOT invoke 'brew update' but still runs 'brew upgrade'", () => {
    const calls: string[] = [];
    runUpdate({ skipBrewUpdate: true, exec: makeExec(calls), isHomebrew: () => true });

    assert.ok(
      !calls.some((c) => c.startsWith("brew update")),
      `expected 'brew update' to be skipped, got: ${JSON.stringify(calls)}`,
    );
    assert.ok(
      calls.some((c) => c.startsWith("brew info")),
      "brew info version check should still run",
    );
    assert.ok(
      calls.some((c) => c === "brew upgrade tu"),
      `expected 'brew upgrade tu' to run, got: ${JSON.stringify(calls)}`,
    );
  });

  it("without the flag (default), invokes 'brew update --quiet' and 'brew upgrade tu'", () => {
    const calls: string[] = [];
    runUpdate({ exec: makeExec(calls), isHomebrew: () => true });

    assert.ok(
      calls.some((c) => c === "brew update --quiet"),
      `expected 'brew update --quiet' to run, got: ${JSON.stringify(calls)}`,
    );
    assert.ok(
      calls.some((c) => c === "brew upgrade tu"),
      `expected 'brew upgrade tu' to run, got: ${JSON.stringify(calls)}`,
    );
  });

  it("preserves brew-info -> upgrade ordering when the flag is set", () => {
    const calls: string[] = [];
    runUpdate({ skipBrewUpdate: true, exec: makeExec(calls), isHomebrew: () => true });

    const infoIdx = calls.findIndex((c) => c.startsWith("brew info"));
    const upgradeIdx = calls.indexOf("brew upgrade tu");
    assert.ok(infoIdx >= 0 && upgradeIdx >= 0, "both info and upgrade should run");
    assert.ok(infoIdx < upgradeIdx, "version check must precede upgrade");
  });

  it("non-Homebrew install prints manual-update guidance and skips all brew calls", () => {
    const calls: string[] = [];
    runUpdate({ skipBrewUpdate: true, exec: makeExec(calls), isHomebrew: () => false });
    assert.equal(calls.length, 0, "no brew commands when not a Homebrew install");
  });
});
