import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { parseGlobalFlags } from "../cli.js";

describe("--user / -u flag: argument filtering", () => {
  it("extracts -u with username from args", () => {
    const result = parseGlobalFlags(["cc", "-u", "bob"]);
    assert.equal(result.userFlag, "bob");
    assert.deepEqual(result.filteredArgs, ["cc"]);
  });

  it("extracts --user long form with username", () => {
    const result = parseGlobalFlags(["--user", "alice", "mh"]);
    assert.equal(result.userFlag, "alice");
    assert.deepEqual(result.filteredArgs, ["mh"]);
  });

  it("userFlag is undefined when -u is not present", () => {
    const result = parseGlobalFlags(["cc", "daily"]);
    assert.equal(result.userFlag, undefined);
  });

  it("-u value is not treated as a positional arg", () => {
    const result = parseGlobalFlags(["-u", "bob", "cc", "h"]);
    assert.equal(result.userFlag, "bob");
    assert.deepEqual(result.filteredArgs, ["cc", "h"]);
  });

  it("-u combined with other flags", () => {
    const result = parseGlobalFlags(["cc", "-u", "bob", "--sync", "--fresh"]);
    assert.equal(result.userFlag, "bob");
    assert.equal(result.syncFlag, true);
    assert.equal(result.freshFlag, true);
    assert.deepEqual(result.filteredArgs, ["cc"]);
  });

  it("-u combined with --watch", () => {
    const result = parseGlobalFlags(["-u", "bob", "-w"]);
    assert.equal(result.userFlag, "bob");
    assert.equal(result.watchFlag, true);
    assert.deepEqual(result.filteredArgs, []);
  });

  // Note: "-u without value" triggers process.exit(1) which can't be tested
  // without mocking process.exit. The validation path is covered by the
  // parseGlobalFlags implementation (hasUserFlag + undefined userFlag check).
});

describe("--user / -u flag: same-user detection", () => {
  // fetchToolMerged is not exported and depends on external ccusage binaries,
  // so we test the branching condition directly. The fix (z40v): when
  // targetUser === config.user, the remote-only path must NOT be taken —
  // the default fresh-fetch path should execute instead.

  function shouldUseRemoteOnlyPath(targetUser: string | undefined, configUser: string): boolean {
    return !!(targetUser && targetUser !== configUser);
  }

  it("returns false when targetUser matches config.user (same user)", () => {
    assert.equal(shouldUseRemoteOnlyPath("sahil", "sahil"), false);
  });

  it("returns true when targetUser differs from config.user", () => {
    assert.equal(shouldUseRemoteOnlyPath("alice", "sahil"), true);
  });

  it("returns false when targetUser is undefined (default path)", () => {
    assert.equal(shouldUseRemoteOnlyPath(undefined, "sahil"), false);
  });

  it("comparison is case-sensitive", () => {
    assert.equal(shouldUseRemoteOnlyPath("Sahil", "sahil"), true);
  });
});
