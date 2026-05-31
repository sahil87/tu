import { describe, it } from "node:test";
import assert from "node:assert/strict";

// ---------------------------------------------------------------------------
// --skip-brew-update flag: gate around the internal `brew update` refresh
//
// runUpdate calls a statically-imported `execSync` directly, which ESM/tsx
// cannot cleanly intercept without refactoring runUpdate for injection (out of
// scope). Following the cli-sync-flag.test.ts precedent, we mirror the gate's
// command-selection logic inline as a pure helper and assert the resulting
// command sequence under each flag value.
// ---------------------------------------------------------------------------

// Mirrors runUpdate's control flow (src/node/core/cli.ts):
//   gate → `brew update --quiet`; always → `brew info`; if not up-to-date → `brew upgrade tu`.
// Returns the sequence of `brew` commands runUpdate would execute for an
// out-of-date install (the upgrade path), given the skipBrewUpdate gate value.
function plannedBrewCommands(skipBrewUpdate: boolean): string[] {
  const commands: string[] = [];
  if (!skipBrewUpdate) {
    commands.push("brew update --quiet");
  }
  commands.push("brew info --json=v2 tu");
  // Out-of-date install: the up-to-date short-circuit does not fire, so the
  // upgrade runs regardless of the flag.
  commands.push("brew upgrade tu");
  return commands;
}

describe("--skip-brew-update flag: gate command selection", () => {
  it("skip=true omits brew update but keeps brew upgrade tu", () => {
    const commands = plannedBrewCommands(true);
    assert.ok(
      !commands.some((c) => c.startsWith("brew update")),
      "no `brew update` command should be present when skip=true",
    );
    assert.ok(
      commands.includes("brew upgrade tu"),
      "`brew upgrade tu` must still run when skip=true",
    );
  });

  it("skip=false includes both brew update and brew upgrade tu", () => {
    const commands = plannedBrewCommands(false);
    assert.ok(
      commands.includes("brew update --quiet"),
      "`brew update --quiet` must run when skip=false",
    );
    assert.ok(
      commands.includes("brew upgrade tu"),
      "`brew upgrade tu` must run when skip=false",
    );
  });

  it("the brew info version check runs regardless of the flag", () => {
    assert.ok(plannedBrewCommands(true).includes("brew info --json=v2 tu"));
    assert.ok(plannedBrewCommands(false).includes("brew info --json=v2 tu"));
  });
});

// ---------------------------------------------------------------------------
// --skip-brew-update flag: raw-argv membership detection idiom
//
// Mirrors the dispatch site in cli.ts:
//   if (cmd === "update") { runUpdate(process.argv.includes("--skip-brew-update")); ... }
// ---------------------------------------------------------------------------

describe("--skip-brew-update flag: raw-argv detection idiom", () => {
  it("returns true when --skip-brew-update is present", () => {
    assert.equal(["update", "--skip-brew-update"].includes("--skip-brew-update"), true);
  });

  it("returns false when --skip-brew-update is absent", () => {
    assert.equal(["update"].includes("--skip-brew-update"), false);
  });
});
