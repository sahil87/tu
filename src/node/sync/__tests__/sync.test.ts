import { describe, it, beforeEach, afterEach } from "node:test";
import assert from "node:assert/strict";
import { mkdirSync, rmSync, readFileSync, writeFileSync, existsSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { execSync } from "node:child_process";

import { writeMetrics, readRemoteEntries, listUsers, isStale, touchLastSync, syncMetrics, fullSync } from "../sync.js";
import { mergeEntries } from "../../core/fetcher.js";
import type { UsageEntry } from "../../core/types.js";
import type { TuConfig } from "../../core/config.js";

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

  // --- Never-shrink guard: day-files are high-water marks. A live fetch after
  // the transcript retention purge collapses toward zero; writeMetrics must
  // never let that residue overwrite correct historical snapshots. ---

  it("skips the write when incoming totalCost is lower than existing (shrink)", () => {
    writeMetrics(TEST_DIR, "sahil", "macbook", "cc", [entry("2026-04-24", 308.12)]);
    writeMetrics(TEST_DIR, "sahil", "macbook", "cc", [entry("2026-04-24", 9.46)]);
    const filePath = join(TEST_DIR, "sahil", "2026", "macbook", "cc-2026-04-24.jsonl");
    const parsed = JSON.parse(readFileSync(filePath, "utf-8").trim());
    assert.equal(parsed.totalCost, 308.12);
  });

  it("skips shrinking writes silently (no stderr warning)", () => {
    const errors: string[] = [];
    const origError = console.error;
    const origWrite = process.stderr.write;
    console.error = (...args: unknown[]) => errors.push(String(args[0]));
    process.stderr.write = ((chunk: string) => { errors.push(String(chunk)); return true; }) as typeof process.stderr.write;
    try {
      writeMetrics(TEST_DIR, "sahil", "macbook", "cc", [entry("2026-04-24", 100.0)]);
      writeMetrics(TEST_DIR, "sahil", "macbook", "cc", [entry("2026-04-24", 1.0)]);
    } finally {
      console.error = origError;
      process.stderr.write = origWrite;
    }
    assert.deepEqual(errors, []);
  });

  it("writes when incoming totalCost equals existing (idempotent refresh)", () => {
    writeMetrics(TEST_DIR, "sahil", "macbook", "cc", [entry("2026-02-20", 1.5)]);
    const filePath = join(TEST_DIR, "sahil", "2026", "macbook", "cc-2026-02-20.jsonl");
    // Equal cost but different token counts — the newer snapshot must win
    const updated = { ...entry("2026-02-20", 1.5), totalTokens: 999 };
    writeMetrics(TEST_DIR, "sahil", "macbook", "cc", [updated]);
    const parsed = JSON.parse(readFileSync(filePath, "utf-8").trim());
    assert.equal(parsed.totalCost, 1.5);
    assert.equal(parsed.totalTokens, 999);
  });

  it("writes when existing file is empty (treated as absent)", () => {
    const dir = join(TEST_DIR, "sahil", "2026", "macbook");
    mkdirSync(dir, { recursive: true });
    const filePath = join(dir, "cc-2026-02-20.jsonl");
    writeFileSync(filePath, "");
    writeMetrics(TEST_DIR, "sahil", "macbook", "cc", [entry("2026-02-20", 0.5)]);
    const parsed = JSON.parse(readFileSync(filePath, "utf-8").trim());
    assert.equal(parsed.totalCost, 0.5);
  });

  it("writes when existing file is not valid JSON (treated as absent)", () => {
    const dir = join(TEST_DIR, "sahil", "2026", "macbook");
    mkdirSync(dir, { recursive: true });
    const filePath = join(dir, "cc-2026-02-20.jsonl");
    writeFileSync(filePath, "not json at all\n");
    writeMetrics(TEST_DIR, "sahil", "macbook", "cc", [entry("2026-02-20", 0.5)]);
    const parsed = JSON.parse(readFileSync(filePath, "utf-8").trim());
    assert.equal(parsed.totalCost, 0.5);
  });

  it("writes when existing JSON has no numeric totalCost (not a UsageEntry)", () => {
    const dir = join(TEST_DIR, "sahil", "2026", "macbook");
    mkdirSync(dir, { recursive: true });
    const filePath = join(dir, "cc-2026-02-20.jsonl");
    writeFileSync(filePath, JSON.stringify({ label: "2026-02-20", totalCost: "junk" }) + "\n");
    writeMetrics(TEST_DIR, "sahil", "macbook", "cc", [entry("2026-02-20", 0.5)]);
    const parsed = JSON.parse(readFileSync(filePath, "utf-8").trim());
    assert.equal(parsed.totalCost, 0.5);
  });

  it("guards per entry within one batch (skips shrunk, writes grown)", () => {
    writeMetrics(TEST_DIR, "sahil", "macbook", "cc", [
      entry("2026-04-24", 308.12),
      entry("2026-04-25", 1.0),
    ]);
    writeMetrics(TEST_DIR, "sahil", "macbook", "cc", [
      entry("2026-04-24", 9.46), // shrunk → skipped
      entry("2026-04-25", 5.0), // grown → written
    ]);
    const dir = join(TEST_DIR, "sahil", "2026", "macbook");
    const day24 = JSON.parse(readFileSync(join(dir, "cc-2026-04-24.jsonl"), "utf-8").trim());
    const day25 = JSON.parse(readFileSync(join(dir, "cc-2026-04-25.jsonl"), "utf-8").trim());
    assert.equal(day24.totalCost, 308.12);
    assert.equal(day25.totalCost, 5.0);
  });

  // --- Report return value (live mode): the decision report is a
  // non-behavioral addition — the live write still happens and is byte-identical. ---

  it("live mode returns a write decision and still writes the file", () => {
    const decisions = writeMetrics(TEST_DIR, "sahil", "macbook", "cc", [entry("2026-02-20", 1.5)]);
    assert.equal(decisions.length, 1);
    assert.equal(decisions[0].action, "write");
    assert.equal(decisions[0].incomingCost, 1.5);
    assert.equal(decisions[0].existingCost, undefined);
    // File was actually written (live mode unchanged)
    assert.ok(existsSync(join(TEST_DIR, "sahil", "2026", "macbook", "cc-2026-02-20.jsonl")));
  });

  it("live mode reports a skip decision when the never-shrink guard fires", () => {
    writeMetrics(TEST_DIR, "sahil", "macbook", "cc", [entry("2026-04-24", 308.12)]);
    const decisions = writeMetrics(TEST_DIR, "sahil", "macbook", "cc", [entry("2026-04-24", 9.46)]);
    assert.equal(decisions[0].action, "skip");
    assert.equal(decisions[0].incomingCost, 9.46);
    assert.equal(decisions[0].existingCost, 308.12);
  });
});

// --- Dry-run writeMetrics: computes the same decisions as a live write but
// touches nothing on disk (shared decision path per toolkit principle №5). ---

describe("writeMetrics (dry-run)", () => {
  beforeEach(() => mkdirSync(TEST_DIR, { recursive: true }));
  afterEach(() => rmSync(TEST_DIR, { recursive: true, force: true }));

  it("reports a new-file write and leaves the filesystem untouched", () => {
    const decisions = writeMetrics(TEST_DIR, "sahil", "macbook", "cc", [entry("2026-07-18", 3.21)], true);
    assert.equal(decisions.length, 1);
    assert.equal(decisions[0].action, "write");
    assert.equal(decisions[0].incomingCost, 3.21);
    assert.equal(decisions[0].existingCost, undefined);
    // Nothing written — not even the year/machine directory.
    assert.ok(!existsSync(join(TEST_DIR, "sahil", "2026", "macbook", "cc-2026-07-18.jsonl")));
    assert.ok(!existsSync(join(TEST_DIR, "sahil", "2026", "macbook")));
  });

  it("reports an update (existing < incoming) with the prior cost", () => {
    writeMetrics(TEST_DIR, "sahil", "macbook", "cc", [entry("2026-07-18", 10.2)]);
    const decisions = writeMetrics(TEST_DIR, "sahil", "macbook", "cc", [entry("2026-07-18", 12.34)], true);
    assert.equal(decisions[0].action, "write");
    assert.equal(decisions[0].incomingCost, 12.34);
    assert.equal(decisions[0].existingCost, 10.2);
    // Existing file is unchanged by the dry-run.
    const parsed = JSON.parse(readFileSync(join(TEST_DIR, "sahil", "2026", "macbook", "cc-2026-07-18.jsonl"), "utf-8").trim());
    assert.equal(parsed.totalCost, 10.2);
  });

  it("reports a never-shrink skip (incoming < existing) with the prior cost", () => {
    writeMetrics(TEST_DIR, "sahil", "macbook", "cc", [entry("2026-06-01", 45.67)]);
    const decisions = writeMetrics(TEST_DIR, "sahil", "macbook", "cc", [entry("2026-06-01", 0.0)], true);
    assert.equal(decisions[0].action, "skip");
    assert.equal(decisions[0].incomingCost, 0.0);
    assert.equal(decisions[0].existingCost, 45.67);
  });

  it("treats absent/empty/unparseable existing files as absent (write, no existingCost)", () => {
    const dir = join(TEST_DIR, "sahil", "2026", "macbook");
    mkdirSync(dir, { recursive: true });
    writeFileSync(join(dir, "cc-2026-07-18.jsonl"), ""); // empty
    writeFileSync(join(dir, "cc-2026-07-19.jsonl"), "not json\n"); // unparseable
    const decisions = writeMetrics(TEST_DIR, "sahil", "macbook", "cc", [
      entry("2026-07-17", 1.0), // absent
      entry("2026-07-18", 1.0), // empty existing
      entry("2026-07-19", 1.0), // unparseable existing
    ], true);
    for (const d of decisions) {
      assert.equal(d.action, "write");
      assert.equal(d.existingCost, undefined);
    }
  });

  it("reports per-entry decisions within one batch without writing", () => {
    writeMetrics(TEST_DIR, "sahil", "macbook", "cc", [
      entry("2026-04-24", 308.12),
      entry("2026-04-25", 1.0),
    ]);
    const decisions = writeMetrics(TEST_DIR, "sahil", "macbook", "cc", [
      entry("2026-04-24", 9.46), // shrunk → skip
      entry("2026-04-25", 5.0), // grown → write
    ], true);
    const byLabel = new Map(decisions.map((d, i) => [i === 0 ? "2026-04-24" : "2026-04-25", d]));
    assert.equal(byLabel.get("2026-04-24")!.action, "skip");
    assert.equal(byLabel.get("2026-04-25")!.action, "write");
    // The grown day-file on disk is still the original 1.0 — dry-run wrote nothing.
    const day25 = JSON.parse(readFileSync(join(TEST_DIR, "sahil", "2026", "macbook", "cc-2026-04-25.jsonl"), "utf-8").trim());
    assert.equal(day25.totalCost, 1.0);
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

describe("listUsers", () => {
  beforeEach(() => mkdirSync(TEST_DIR, { recursive: true }));
  afterEach(() => rmSync(TEST_DIR, { recursive: true, force: true }));

  it("returns sorted user dirs, skipping docs/ and dot-prefixed entries", () => {
    for (const d of ["bob", "alice", "docs", ".git"]) mkdirSync(join(TEST_DIR, d));
    writeFileSync(join(TEST_DIR, ".last-sync"), "");
    writeFileSync(join(TEST_DIR, "README.md"), "");
    assert.deepEqual(listUsers(TEST_DIR), ["alice", "bob"]);
  });

  it("returns [] for a missing metrics dir", () => {
    assert.deepEqual(listUsers(join(TEST_DIR, "nope")), []);
  });

  it("returns [] for an empty metrics dir", () => {
    assert.deepEqual(listUsers(TEST_DIR), []);
  });
});

describe("all-users aggregate (listUsers + readRemoteEntries + mergeEntries)", () => {
  beforeEach(() => mkdirSync(TEST_DIR, { recursive: true }));
  afterEach(() => rmSync(TEST_DIR, { recursive: true, force: true }));

  const tokens = (label: string, cost: number, total: number): UsageEntry => ({
    label,
    totalCost: cost,
    inputTokens: total,
    outputTokens: 0,
    cacheCreationTokens: 0,
    cacheReadTokens: 0,
    totalTokens: total,
  });

  it("sums cost and tokens per label across 2 users x 2 machines", () => {
    mkdirSync(join(TEST_DIR, "docs"));
    writeMetrics(TEST_DIR, "alice", "m1", "cc", [tokens("2026-08-01", 1.0, 100), tokens("2026-08-02", 0.5, 50)]);
    writeMetrics(TEST_DIR, "alice", "m2", "cc", [tokens("2026-08-01", 2.0, 200)]);
    writeMetrics(TEST_DIR, "bob", "m1", "cc", [tokens("2026-08-01", 4.0, 400)]);
    writeMetrics(TEST_DIR, "bob", "m2", "cc", [tokens("2026-08-02", 8.0, 800)]);

    const summed = listUsers(TEST_DIR)
      .map((u) => readRemoteEntries(TEST_DIR, u, null, "cc"))
      .reduce((acc, entries) => mergeEntries(acc, entries), [] as UsageEntry[]);

    const byLabel = new Map(summed.map((e) => [e.label, e]));
    assert.equal(byLabel.size, 2);
    assert.equal(byLabel.get("2026-08-01")!.totalCost, 7.0);
    assert.equal(byLabel.get("2026-08-01")!.totalTokens, 700);
    assert.equal(byLabel.get("2026-08-02")!.totalCost, 8.5);
    assert.equal(byLabel.get("2026-08-02")!.totalTokens, 850);
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
  // syncMetrics() pulls/pushes `origin main` (see sync.ts), so the whole fixture
  // MUST be anchored on `main`. `git init` uses the runner's init.defaultBranch
  // (often `master` in CI / older git), so we pin it explicitly rather than
  // depend on the developer's global config. Two things are required:
  //   1. the bare repo's HEAD must advertise `main`, so every `git clone` of it
  //      (including the sub-clone in the upstream-integration test) checks out
  //      `main` rather than an empty `master`;
  //   2. the working clone's branch must be renamed to `main` before pushing.
  // Without (1), a fresh clone lands on `master` with no commits and the sync's
  // `pull --rebase origin main` / `push` fail ("current branch master has no
  // commits yet").
  execSync(`git -C "${BARE_DIR}" symbolic-ref HEAD refs/heads/main`, opts);
  execSync(`git clone "${BARE_DIR}" "${CLONE_DIR}"`, opts);
  // Initial commit so the repo has a branch
  execSync(`git -C "${CLONE_DIR}" config user.email "test@test.com"`, opts);
  execSync(`git -C "${CLONE_DIR}" config user.name "Test"`, opts);
  const initFile = join(CLONE_DIR, ".gitkeep");
  writeFileSync(initFile, "");
  execSync(`git -C "${CLONE_DIR}" add .gitkeep`, opts);
  execSync(`git -C "${CLONE_DIR}" commit -m "init"`, opts);
  execSync(`git -C "${CLONE_DIR}" branch -M main`, opts);
  execSync(`git -C "${CLONE_DIR}" push -u origin main`, opts);
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

  it("returns true when the user dir does not exist (no data yet)", async () => {
    // No writeMetrics call — the user dir is absent, as on a first run or when
    // every ccusage source is unavailable. `git add <user>/` would otherwise
    // fail with "pathspec did not match any files"; sync must treat this as a
    // clean no-op and still succeed.
    const result = await syncMetrics(CLONE_DIR, "sahil");
    assert.equal(result, true);

    // Nothing was staged, so no commit should have been created beyond init.
    const log = execSync(`git -C "${BARE_DIR}" log --oneline`, { encoding: "utf-8" });
    assert.ok(!log.includes("# sahil: update"), `Expected no update commit, got: ${log.trim()}`);
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
    // Anchor on `main` to match syncMetrics()'s hardcoded `origin main` (see the
    // gitSetup() note) — git init.defaultBranch may be `master` on the runner.
    execSync(`git -C "${spacedBare}" symbolic-ref HEAD refs/heads/main`, opts);
    mkdirSync(join(GIT_DIR, "My Data"), { recursive: true });
    execSync(`git clone "${spacedBare}" "${spacedClone}"`, opts);
    execSync(`git -C "${spacedClone}" config user.email "test@test.com"`, opts);
    execSync(`git -C "${spacedClone}" config user.name "Test"`, opts);
    writeFileSync(join(spacedClone, ".gitkeep"), "");
    execSync(`git -C "${spacedClone}" add .gitkeep`, opts);
    execSync(`git -C "${spacedClone}" commit -m "init"`, opts);
    execSync(`git -C "${spacedClone}" branch -M main`, opts);
    execSync(`git -C "${spacedClone}" push -u origin main`, opts);

    writeMetrics(spacedClone, "sahil", "macbook", "cc", [entry("2026-02-22", 1.5)]);
    const result = await syncMetrics(spacedClone, "sahil");
    assert.equal(result, true, "expected sync to succeed with spaces in metricsDir");

    const log = execSync(`git -C "${spacedBare}" log --oneline`, { encoding: "utf-8" });
    assert.ok(log.includes("# sahil: update"), "expected sahil commit in bare repo log");
  });
});

// --- fullSync (dry-run): reports without mutating the working tree, the
// metrics repo, or the network. ---
//
// NOTE on fetch data: fullSync calls fetchHistory, which shells out to ccusage
// (absent in CI → yields []). So the per-tool write decisions are empty here;
// these tests assert the strong no-mutation invariants (no commit, no
// .last-sync, no file writes) and the report shape, which hold regardless of
// fetch data. The would-commit git-half branch is exercised deterministically
// by pre-staging a dirty file under the user dir (independent of ccusage).

describe("fullSync (dry-run)", () => {
  beforeEach(() => gitSetup());
  afterEach(() => gitTeardown());

  const dryConfig = (metricsDir: string): TuConfig => ({
    version: 2,
    mode: "multi",
    metricsRepo: "git@example.com:repo.git",
    metricsDir,
    machine: "macbook",
    user: "sahil",
    autoSync: true,
  });

  it("returns a structured report and performs no git commit, no push, no .last-sync", async () => {
    const tuHome = join(GIT_DIR, "tu-home");
    mkdirSync(tuHome, { recursive: true });

    const before = execSync(`git -C "${BARE_DIR}" log --oneline`, { encoding: "utf-8" }).trim();

    const report = await fullSync(dryConfig(CLONE_DIR), tuHome, true);

    // Report shape
    assert.equal(report.metricsDir, CLONE_DIR);
    assert.equal(report.user, "sahil");
    assert.equal(report.machine, "macbook");
    assert.ok(Array.isArray(report.tools));
    assert.match(report.commitMessage, /^# sahil: update \d{4}-\d{2}-\d{2}$/);

    // No mutation: bare repo log unchanged, no .last-sync touched.
    const after = execSync(`git -C "${BARE_DIR}" log --oneline`, { encoding: "utf-8" }).trim();
    assert.equal(after, before, "dry-run must not add a commit to the bare repo");
    assert.ok(!existsSync(join(tuHome, ".last-sync")), "dry-run must not touch .last-sync");
  });

  it("does not write day-files to the metrics repo", async () => {
    const tuHome = join(GIT_DIR, "tu-home-2");
    mkdirSync(tuHome, { recursive: true });

    await fullSync(dryConfig(CLONE_DIR), tuHome, true);

    // No user dir was created by the dry-run (fetch is empty and, even with
    // data, writeMetrics dry-run creates nothing).
    assert.ok(!existsSync(join(CLONE_DIR, "sahil")), "dry-run must not create the user dir");
  });

  it("reports wouldCommit=true when the user dir is already dirty (read-only git status)", async () => {
    const tuHome = join(GIT_DIR, "tu-home-3");
    mkdirSync(tuHome, { recursive: true });

    // Pre-stage a dirty (untracked) file under the user dir. `git status
    // --porcelain sahil/` will report it, so the dry-run's would-commit
    // decision is true even though the fetch produced no new writes.
    const userDir = join(CLONE_DIR, "sahil", "2026", "macbook");
    mkdirSync(userDir, { recursive: true });
    writeFileSync(join(userDir, "cc-2026-07-18.jsonl"), JSON.stringify(entry("2026-07-18", 5.0)) + "\n");

    const report = await fullSync(dryConfig(CLONE_DIR), tuHome, true);
    assert.equal(report.wouldCommit, true);

    // Still no commit added — the dirty file remains uncommitted.
    const log = execSync(`git -C "${BARE_DIR}" log --oneline`, { encoding: "utf-8" });
    assert.ok(!log.includes("# sahil: update"), "dry-run must not commit the dirty file");
  });

  it("computes the git half read-only — no commit even if the tree is dirty", async () => {
    // Regardless of what the fetch yields (ccusage cache presence is
    // environment-dependent), the dry-run must never commit or push. Pre-stage
    // a dirty file so the would-commit path is active, then prove the bare repo
    // gains no commit and .last-sync is not touched.
    const tuHome = join(GIT_DIR, "tu-home-4");
    mkdirSync(tuHome, { recursive: true });
    const userDir = join(CLONE_DIR, "sahil", "2026", "macbook");
    mkdirSync(userDir, { recursive: true });
    writeFileSync(join(userDir, "cc-2026-07-17.jsonl"), JSON.stringify(entry("2026-07-17", 9.9)) + "\n");

    const before = execSync(`git -C "${BARE_DIR}" log --oneline`, { encoding: "utf-8" }).trim();
    const report = await fullSync(dryConfig(CLONE_DIR), tuHome, true);
    const after = execSync(`git -C "${BARE_DIR}" log --oneline`, { encoding: "utf-8" }).trim();

    assert.equal(report.wouldCommit, true, "a dirty user dir must make wouldCommit true");
    assert.equal(after, before, "dry-run must add no commit even with a dirty tree");
    assert.ok(!existsSync(join(tuHome, ".last-sync")), "dry-run must not touch .last-sync");
  });
});
