import { describe, it, beforeEach, afterEach } from "node:test";
import assert from "node:assert/strict";
import { mkdirSync, rmSync, readFileSync, writeFileSync, existsSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { execSync } from "node:child_process";

import { writeMetrics, readRemoteEntries, isStale, touchLastSync, syncMetrics } from "../sync.js";
import type { UsageEntry } from "../../core/types.js";

const TEST_DIR = join(tmpdir(), "tu-sync-test-" + process.pid);

const entry = (label: string, cost: number): UsageEntry => ({
  label,
  totalCost: cost,
  inputTokens: 100,
  outputTokens: 50,
  cacheCreationTokens: 0,
  cacheReadTokens: 0,
  totalTokens: 150,
});

describe("writeMetrics", () => {
  beforeEach(() => mkdirSync(TEST_DIR, { recursive: true }));
  afterEach(() => rmSync(TEST_DIR, { recursive: true, force: true }));

  it("creates date-partitioned path and writes JSONL file", () => {
    const entries = [entry("2026-02-20", 1.5)];
    writeMetrics(TEST_DIR, "sahil", "macbook", "cc", entries);
    const filePath = join(TEST_DIR, "sahil", "2026", "macbook", "cc-2026-02-20.jsonl");
    assert.ok(existsSync(filePath));
    const parsed = JSON.parse(readFileSync(filePath, "utf-8").trim());
    assert.equal(parsed.label, "2026-02-20");
    assert.equal(parsed.totalCost, 1.5);
  });

  it("writes one file per entry", () => {
    const entries = [entry("2026-02-19", 1.0), entry("2026-02-20", 2.0)];
    writeMetrics(TEST_DIR, "sahil", "macbook", "cc", entries);
    assert.ok(existsSync(join(TEST_DIR, "sahil", "2026", "macbook", "cc-2026-02-19.jsonl")));
    assert.ok(existsSync(join(TEST_DIR, "sahil", "2026", "macbook", "cc-2026-02-20.jsonl")));
  });

  it("handles entries spanning multiple years", () => {
    const entries = [entry("2025-12-31", 1.0), entry("2026-01-01", 2.0)];
    writeMetrics(TEST_DIR, "sahil", "macbook", "cc", entries);
    assert.ok(existsSync(join(TEST_DIR, "sahil", "2025", "macbook", "cc-2025-12-31.jsonl")));
    assert.ok(existsSync(join(TEST_DIR, "sahil", "2026", "macbook", "cc-2026-01-01.jsonl")));
  });

  it("overwrites existing file on re-run", () => {
    writeMetrics(TEST_DIR, "sahil", "macbook", "cc", [entry("2026-02-20", 1.0)]);
    writeMetrics(TEST_DIR, "sahil", "macbook", "cc", [entry("2026-02-20", 2.0)]);
    const filePath = join(TEST_DIR, "sahil", "2026", "macbook", "cc-2026-02-20.jsonl");
    const parsed = JSON.parse(readFileSync(filePath, "utf-8").trim());
    assert.equal(parsed.totalCost, 2.0);
  });
});

describe("readRemoteEntries", () => {
  beforeEach(() => mkdirSync(TEST_DIR, { recursive: true }));
  afterEach(() => rmSync(TEST_DIR, { recursive: true, force: true }));

  it("does NOT read entries from other users", () => {
    const dir = join(TEST_DIR, "bob", "2026", "laptop");
    mkdirSync(dir, { recursive: true });
    writeFileSync(join(dir, "cc-2026-02-20.jsonl"), JSON.stringify(entry("2026-02-20", 0.5)));

    const result = readRemoteEntries(TEST_DIR, "sahil", "macbook", "cc");
    assert.equal(result.length, 0);
  });

  it("reads entries from same user, different machine", () => {
    const dir = join(TEST_DIR, "sahil", "2026", "workstation");
    mkdirSync(dir, { recursive: true });
    writeFileSync(join(dir, "cc-2026-02-20.jsonl"), JSON.stringify(entry("2026-02-20", 0.8)));

    const result = readRemoteEntries(TEST_DIR, "sahil", "macbook", "cc");
    assert.equal(result.length, 1);
    assert.equal(result[0].totalCost, 0.8);
  });

  it("skips excluded machine entries", () => {
    const dir = join(TEST_DIR, "sahil", "2026", "macbook");
    mkdirSync(dir, { recursive: true });
    writeFileSync(join(dir, "cc-2026-02-20.jsonl"), JSON.stringify(entry("2026-02-20", 1.0)));

    const result = readRemoteEntries(TEST_DIR, "sahil", "macbook", "cc");
    assert.equal(result.length, 0);
  });

  it("reads all machines when excludeMachine is null", () => {
    const dirs = [
      join(TEST_DIR, "bob", "2026", "laptop"),
      join(TEST_DIR, "bob", "2026", "desktop"),
    ];
    for (const d of dirs) {
      mkdirSync(d, { recursive: true });
      writeFileSync(join(d, "cc-2026-02-20.jsonl"), JSON.stringify(entry("2026-02-20", 0.5)));
    }

    const result = readRemoteEntries(TEST_DIR, "bob", null, "cc");
    assert.equal(result.length, 2);
  });

  it("reads from same user, multiple machines", () => {
    const dirs = [
      join(TEST_DIR, "sahil", "2026", "workstation"),
      join(TEST_DIR, "sahil", "2026", "desktop"),
    ];
    for (const d of dirs) {
      mkdirSync(d, { recursive: true });
      writeFileSync(join(d, "cc-2026-02-20.jsonl"), JSON.stringify(entry("2026-02-20", 0.5)));
    }

    const result = readRemoteEntries(TEST_DIR, "sahil", "macbook", "cc");
    assert.equal(result.length, 2);
  });

  it("returns empty array when target user dir does not exist", () => {
    const result = readRemoteEntries(TEST_DIR, "alice", null, "cc");
    assert.equal(result.length, 0);
  });

  it("returns empty array when metrics dir does not exist", () => {
    const result = readRemoteEntries(join(TEST_DIR, "nonexistent"), "sahil", "macbook", "cc");
    assert.equal(result.length, 0);
  });

  it("returns empty array when no remote data exists", () => {
    const dir = join(TEST_DIR, "sahil", "2026", "macbook");
    mkdirSync(dir, { recursive: true });
    writeFileSync(join(dir, "cc-2026-02-20.jsonl"), JSON.stringify(entry("2026-02-20", 1.0)));

    const result = readRemoteEntries(TEST_DIR, "sahil", "macbook", "cc");
    assert.equal(result.length, 0);
  });

  it("only reads files matching the toolKey prefix", () => {
    const dir = join(TEST_DIR, "sahil", "2026", "workstation");
    mkdirSync(dir, { recursive: true });
    writeFileSync(join(dir, "cc-2026-02-20.jsonl"), JSON.stringify(entry("2026-02-20", 0.5)));
    writeFileSync(join(dir, "codex-2026-02-20.jsonl"), JSON.stringify(entry("2026-02-20", 0.2)));

    const result = readRemoteEntries(TEST_DIR, "sahil", "macbook", "cc");
    assert.equal(result.length, 1);
    assert.equal(result[0].totalCost, 0.5);
  });
});

describe("isStale", () => {
  beforeEach(() => mkdirSync(TEST_DIR, { recursive: true }));
  afterEach(() => rmSync(TEST_DIR, { recursive: true, force: true }));

  it("returns true when .last-sync does not exist", () => {
    assert.equal(isStale(TEST_DIR), true);
  });

  it("returns true when .last-sync is older than 3 hours", () => {
    const old = new Date(Date.now() - 4 * 60 * 60 * 1000).toISOString();
    writeFileSync(join(TEST_DIR, ".last-sync"), old + "\n");
    assert.equal(isStale(TEST_DIR), true);
  });

  it("returns false when .last-sync is within 3 hours", () => {
    const recent = new Date(Date.now() - 2 * 60 * 60 * 1000).toISOString();
    writeFileSync(join(TEST_DIR, ".last-sync"), recent + "\n");
    assert.equal(isStale(TEST_DIR), false);
  });

  it("returns true when .last-sync has invalid content", () => {
    writeFileSync(join(TEST_DIR, ".last-sync"), "not-a-date\n");
    assert.equal(isStale(TEST_DIR), true);
  });
});

describe("touchLastSync", () => {
  beforeEach(() => mkdirSync(TEST_DIR, { recursive: true }));
  afterEach(() => rmSync(TEST_DIR, { recursive: true, force: true }));

  it("creates .last-sync with current timestamp", () => {
    const before = Date.now();
    touchLastSync(TEST_DIR);
    const after = Date.now();
    const content = readFileSync(join(TEST_DIR, ".last-sync"), "utf-8").trim();
    const ts = new Date(content).getTime();
    assert.ok(ts >= before && ts <= after);
  });

  it("overwrites existing .last-sync", () => {
    writeFileSync(join(TEST_DIR, ".last-sync"), "2020-01-01T00:00:00.000Z\n");
    touchLastSync(TEST_DIR);
    const content = readFileSync(join(TEST_DIR, ".last-sync"), "utf-8").trim();
    const ts = new Date(content).getTime();
    assert.ok(Date.now() - ts < 5000);
  });
});

// --- syncMetrics tests (T006) ---

const GIT_DIR = join(tmpdir(), "tu-sync-git-test-" + process.pid);
const BARE_DIR = join(GIT_DIR, "bare.git");
const CLONE_DIR = join(GIT_DIR, "clone");

function gitSetup() {
  mkdirSync(GIT_DIR, { recursive: true });
  const opts = { stdio: "pipe" as const };
  execSync(`git init --bare "${BARE_DIR}"`, opts);
  execSync(`git clone "${BARE_DIR}" "${CLONE_DIR}"`, opts);
  // Initial commit so the repo has a branch
  execSync(`git -C "${CLONE_DIR}" config user.email "test@test.com"`, opts);
  execSync(`git -C "${CLONE_DIR}" config user.name "Test"`, opts);
  const initFile = join(CLONE_DIR, ".gitkeep");
  writeFileSync(initFile, "");
  execSync(`git -C "${CLONE_DIR}" add .gitkeep`, opts);
  execSync(`git -C "${CLONE_DIR}" commit -m "init"`, opts);
  execSync(`git -C "${CLONE_DIR}" push`, opts);
}

function gitTeardown() {
  rmSync(GIT_DIR, { recursive: true, force: true });
}

describe("syncMetrics", () => {
  beforeEach(() => gitSetup());
  afterEach(() => gitTeardown());

  it("returns true and pushes committed files on success", async () => {
    writeMetrics(CLONE_DIR, "sahil", "macbook", "cc", [entry("2026-02-22", 1.5)]);
    const result = await syncMetrics(CLONE_DIR, "sahil");
    assert.equal(result, true);

    // Verify the commit reached the bare repo
    const log = execSync(`git -C "${BARE_DIR}" log --oneline`, { encoding: "utf-8" });
    assert.ok(log.includes("# sahil: update"));
  });

  it("returns true when no local changes exist", async () => {
    // First sync to establish the user directory in the repo
    writeMetrics(CLONE_DIR, "sahil", "macbook", "cc", [entry("2026-02-21", 1.0)]);
    await syncMetrics(CLONE_DIR, "sahil");

    // Second sync — no new files
    const result = await syncMetrics(CLONE_DIR, "sahil");
    assert.equal(result, true);
  });

  it("returns false when metricsDir is not a git repo", async () => {
    const plainDir = join(GIT_DIR, "plain");
    mkdirSync(join(plainDir, "sahil"), { recursive: true });
    const result = await syncMetrics(plainDir, "sahil");
    assert.equal(result, false);
  });

  it("returns false and warns to stderr when remote is unreachable", async () => {
    // Point remote to a nonexistent path
    execSync(`git -C "${CLONE_DIR}" remote set-url origin /nonexistent/repo.git`, {
      stdio: "pipe",
    });
    writeMetrics(CLONE_DIR, "sahil", "macbook", "cc", [entry("2026-02-22", 1.0)]);

    const errors: string[] = [];
    const origError = console.error;
    console.error = (...args: unknown[]) => errors.push(String(args[0]));
    try {
      const result = await syncMetrics(CLONE_DIR, "sahil");
      assert.equal(result, false);
      assert.ok(
        errors.some((e) => e.startsWith("Warning: sync pull failed")),
        `Expected pull warning, got: ${errors.join("; ")}`,
      );
    } finally {
      console.error = origError;
    }
  });

  it("retries push once when first attempt fails", async () => {
    // Install a pre-receive hook that rejects the first push, then allows the second
    const hookPath = join(BARE_DIR, "hooks", "pre-receive");
    const counterPath = join(GIT_DIR, "push-counter");
    writeFileSync(counterPath, "0");
    writeFileSync(
      hookPath,
      `#!/bin/sh
count=$(cat "${counterPath}")
count=$((count + 1))
echo $count > "${counterPath}"
if [ "$count" -eq 1 ]; then
  echo "Rejecting first push" >&2
  exit 1
fi
exit 0
`,
    );
    execSync(`chmod +x "${hookPath}"`, { stdio: "pipe" });

    writeMetrics(CLONE_DIR, "sahil", "macbook", "cc", [entry("2026-02-22", 1.0)]);
    const result = await syncMetrics(CLONE_DIR, "sahil");
    assert.equal(result, true);

    // Verify push counter reached 2 (first rejected, second accepted)
    const count = readFileSync(counterPath, "utf-8").trim();
    assert.equal(count, "2");
  });

  it("returns false and warns when both push attempts fail", async () => {
    // Install a pre-receive hook that always rejects
    const hookPath = join(BARE_DIR, "hooks", "pre-receive");
    writeFileSync(
      hookPath,
      `#!/bin/sh
echo "Push rejected" >&2
exit 1
`,
    );
    execSync(`chmod +x "${hookPath}"`, { stdio: "pipe" });

    writeMetrics(CLONE_DIR, "sahil", "macbook", "cc", [entry("2026-02-22", 1.0)]);

    const errors: string[] = [];
    const origError = console.error;
    console.error = (...args: unknown[]) => errors.push(String(args[0]));
    try {
      const result = await syncMetrics(CLONE_DIR, "sahil");
      assert.equal(result, false);
      assert.ok(
        errors.some((e) => e.startsWith("Warning: sync push failed after retry")),
        `Expected push retry warning, got: ${errors.join("; ")}`,
      );
    } finally {
      console.error = origError;
    }
  });

  it("integrates upstream changes via pull before pushing", async () => {
    // Create a second clone, push a commit from it
    const clone2 = join(GIT_DIR, "clone2");
    const opts = { stdio: "pipe" as const };
    execSync(`git clone "${BARE_DIR}" "${clone2}"`, opts);
    execSync(`git -C "${clone2}" config user.email "test@test.com"`, opts);
    execSync(`git -C "${clone2}" config user.name "Test"`, opts);
    mkdirSync(join(clone2, "bob", "2026", "laptop"), { recursive: true });
    writeFileSync(join(clone2, "bob", "2026", "laptop", "cc-2026-02-22.jsonl"), "{}");
    execSync(`git -C "${clone2}" add .`, opts);
    execSync(`git -C "${clone2}" commit -m "bob: update 2026-02-22"`, opts);
    execSync(`git -C "${clone2}" push`, opts);

    // Now sync from the first clone — should pull bob's commit, then push sahil's
    writeMetrics(CLONE_DIR, "sahil", "macbook", "cc", [entry("2026-02-22", 1.0)]);
    const result = await syncMetrics(CLONE_DIR, "sahil");
    assert.equal(result, true);

    // Verify both commits exist in the bare repo
    const log = execSync(`git -C "${BARE_DIR}" log --oneline`, { encoding: "utf-8" });
    assert.ok(log.includes("# sahil: update"));
    assert.ok(log.includes("bob: update"));

    // Verify sahil's commit is on top (pull --rebase puts local on top)
    const lines = log.trim().split("\n");
    assert.ok(lines[0].includes("sahil"));
  });

  it("recovers from interrupted rebase and syncs successfully", async () => {
    const opts = { stdio: "pipe" as const };

    // Create a local commit
    writeMetrics(CLONE_DIR, "sahil", "macbook", "cc", [entry("2026-02-22", 1.0)]);
    execSync(`git -C "${CLONE_DIR}" add .`, opts);
    execSync(`git -C "${CLONE_DIR}" commit -m "local commit"`, opts);

    // Use GIT_SEQUENCE_EDITOR=true to start an interactive rebase that
    // pauses at "edit" — this reliably leaves .git/rebase-merge/
    execSync(
      `git -C "${CLONE_DIR}" rebase -i --root`,
      { ...opts, env: { ...process.env, GIT_SEQUENCE_EDITOR: "sed -i.bak 's/^pick/edit/'" } },
    );

    // Verify repo is in a rebase state
    assert.ok(
      existsSync(join(CLONE_DIR, ".git", "rebase-merge")) ||
      existsSync(join(CLONE_DIR, ".git", "rebase-apply")),
      "Expected interrupted rebase state",
    );

    const errors: string[] = [];
    const origError = console.error;
    console.error = (...args: unknown[]) => errors.push(String(args[0]));
    try {
      writeMetrics(CLONE_DIR, "sahil", "macbook", "cc", [entry("2026-02-23", 2.0)]);
      const result = await syncMetrics(CLONE_DIR, "sahil");
      assert.equal(result, true);
      assert.ok(
        errors.some((e) => e.includes("recovering from interrupted rebase")),
        `Expected recovery warning, got: ${errors.join("; ")}`,
      );
    } finally {
      console.error = origError;
    }
  });

  // --- Post-exec-migration: metricsDir with spaces must still work ---
  // execFile takes each argv entry as a literal string, so paths with spaces
  // no longer need quoting. This would have broken under the previous exec(cmd)
  // implementation because shell-word-splitting would split on the space.
  it("handles metricsDir with spaces (execFile literal argv)", async () => {
    const opts = { stdio: "pipe" as const };
    const spacedBare = join(GIT_DIR, "bare with space.git");
    const spacedClone = join(GIT_DIR, "My Data", "clone with space");
    execSync(`git init --bare "${spacedBare}"`, opts);
    mkdirSync(join(GIT_DIR, "My Data"), { recursive: true });
    execSync(`git clone "${spacedBare}" "${spacedClone}"`, opts);
    execSync(`git -C "${spacedClone}" config user.email "test@test.com"`, opts);
    execSync(`git -C "${spacedClone}" config user.name "Test"`, opts);
    writeFileSync(join(spacedClone, ".gitkeep"), "");
    execSync(`git -C "${spacedClone}" add .gitkeep`, opts);
    execSync(`git -C "${spacedClone}" commit -m "init"`, opts);
    execSync(`git -C "${spacedClone}" push`, opts);

    writeMetrics(spacedClone, "sahil", "macbook", "cc", [entry("2026-02-22", 1.5)]);
    const result = await syncMetrics(spacedClone, "sahil");
    assert.equal(result, true, "expected sync to succeed with spaces in metricsDir");

    const log = execSync(`git -C "${spacedBare}" log --oneline`, { encoding: "utf-8" });
    assert.ok(log.includes("# sahil: update"), "expected sahil commit in bare repo log");
  });
});
