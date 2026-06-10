import { describe, it, beforeEach, afterEach } from "node:test";
import assert from "node:assert/strict";
import { mkdirSync, rmSync, readFileSync, writeFileSync } from "node:fs";
import { join, dirname } from "node:path";
import { tmpdir } from "node:os";
import { execSync, spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

// scripts/repair-metrics.mjs is a standalone ops script (not bundled, not
// imported by src/), so it is driven end-to-end as a child process against a
// seeded local git fixture — hermetic: explicit --repo, no HOME/TU_* reads,
// and never the real ~/.tu/metrics_repo. Fixture pattern follows
// sync.test.ts / cli-sync.test.ts.
const SCRIPT = fileURLToPath(new URL("../../../../scripts/repair-metrics.mjs", import.meta.url));

const TEST_DIR = join(tmpdir(), "tu-repair-test-" + process.pid);
const REPO = join(TEST_DIR, "metrics");

const opts = { stdio: "pipe" as const };

function initRepo(): void {
  mkdirSync(REPO, { recursive: true });
  execSync(`git init "${REPO}"`, opts);
  execSync(`git -C "${REPO}" config user.email "test@test.com"`, opts);
  execSync(`git -C "${REPO}" config user.name "Test"`, opts);
  // The developer's global config could enable signing and stall on a missing
  // key — pin it off for the fixture.
  execSync(`git -C "${REPO}" config commit.gpgsign false`, opts);
}

function writeDay(relPath: string, content: string): void {
  const full = join(REPO, relPath);
  mkdirSync(dirname(full), { recursive: true });
  writeFileSync(full, content);
}

function commitAll(msg: string): string {
  execSync(`git -C "${REPO}" add -A`, opts);
  execSync(`git -C "${REPO}" commit -m "${msg}"`, opts);
  return execSync(`git -C "${REPO}" rev-parse HEAD`, { encoding: "utf-8" }).trim();
}

function commitCount(): number {
  return Number(execSync(`git -C "${REPO}" rev-list --count HEAD`, { encoding: "utf-8" }).trim());
}

const entryLine = (label: string, cost: number, tokens = 150): string =>
  JSON.stringify({
    label,
    totalCost: cost,
    inputTokens: 100,
    outputTokens: 50,
    cacheCreationTokens: 0,
    cacheReadTokens: 0,
    totalTokens: tokens,
  }) + "\n";

function runScript(...args: string[]): { status: number | null; stdout: string; stderr: string } {
  const result = spawnSync(process.execPath, [SCRIPT, ...args], { encoding: "utf-8" });
  return { status: result.status, stdout: result.stdout ?? "", stderr: result.stderr ?? "" };
}

describe("repair-metrics script", () => {
  beforeEach(() => {
    rmSync(TEST_DIR, { recursive: true, force: true });
    initRepo();
  });
  afterEach(() => rmSync(TEST_DIR, { recursive: true, force: true }));

  const SHRUNK = "sahil/2026/devws/cc-2026-04-24.jsonl";
  const HIGH = entryLine("2026-04-24", 308.12, 9999);
  const LOW = entryLine("2026-04-24", 9.46, 12);

  function seedShrunkRepo(): void {
    writeDay(SHRUNK, HIGH);
    writeDay("sahil/2026/devws/cc-2026-05-01.jsonl", entryLine("2026-05-01", 10.0));
    writeDay("bob/2026/laptop/cc-2026-04-24.jsonl", entryLine("2026-04-24", 50.0));
    commitAll("initial high-water marks");
    writeDay(SHRUNK, LOW);
    writeDay("bob/2026/laptop/cc-2026-04-24.jsonl", entryLine("2026-04-24", 2.0));
    commitAll("post-purge shrink");
  }

  it("dry run reports shrunk files with current, max, and delta — and modifies nothing", () => {
    seedShrunkRepo();
    const result = runScript("--repo", REPO);
    assert.equal(result.status, 0, result.stderr);
    assert.ok(result.stdout.includes(SHRUNK), `expected shrunk path in report:\n${result.stdout}`);
    assert.ok(result.stdout.includes("$9.46"), "expected current value");
    assert.ok(result.stdout.includes("$308.12"), "expected historical max");
    assert.ok(result.stdout.includes("+$298.66"), "expected delta");
    assert.ok(result.stdout.includes("Dry run"), "expected dry-run notice");
    // Working tree untouched
    assert.equal(readFileSync(join(REPO, SHRUNK), "utf-8"), LOW);
  });

  it("dry run omits files that never shrank", () => {
    seedShrunkRepo();
    const result = runScript("--repo", REPO);
    assert.equal(result.status, 0, result.stderr);
    assert.ok(!result.stdout.includes("cc-2026-05-01.jsonl"), "never-shrunk file must not be listed");
  });

  it("reports per-user subtotals and a grand total across users", () => {
    seedShrunkRepo();
    const result = runScript("--repo", REPO);
    assert.equal(result.status, 0, result.stderr);
    assert.ok(result.stdout.includes("Per-user totals:"), "expected per-user section");
    assert.ok(/sahil: \+\$298\.66 across 1 file/.test(result.stdout), `expected sahil subtotal:\n${result.stdout}`);
    assert.ok(/bob: \+\$48\.00 across 1 file/.test(result.stdout), `expected bob subtotal:\n${result.stdout}`);
    // 298.66 + 48.00
    assert.ok(/Grand total: \+\$346\.66 across 2 file/.test(result.stdout), `expected grand total:\n${result.stdout}`);
  });

  it("does not flag files within a cent of their historical max", () => {
    const path = "sahil/2026/devws/cc-2026-03-01.jsonl";
    writeDay(path, entryLine("2026-03-01", 1.005));
    commitAll("high");
    writeDay(path, entryLine("2026-03-01", 1.0));
    commitAll("within tolerance");
    const result = runScript("--repo", REPO);
    assert.equal(result.status, 0, result.stderr);
    assert.ok(!result.stdout.includes(path), "within-a-cent file must not be flagged");
    assert.ok(result.stdout.includes("Nothing to repair"), `expected nothing-to-repair:\n${result.stdout}`);
  });

  it("picks the maximum across 3+ versions (max in the middle of history)", () => {
    const path = "sahil/2026/devws/cc-2026-02-10.jsonl";
    writeDay(path, entryLine("2026-02-10", 5.0));
    commitAll("v1");
    writeDay(path, entryLine("2026-02-10", 100.0, 7777));
    commitAll("v2 — the high-water mark");
    writeDay(path, entryLine("2026-02-10", 20.0));
    commitAll("v3");
    const result = runScript("--repo", REPO);
    assert.equal(result.status, 0, result.stderr);
    assert.ok(result.stdout.includes("$100.00"), `expected middle-commit max:\n${result.stdout}`);
    assert.ok(result.stdout.includes("+$80.00"), "expected delta against v3");
  });

  it("skips unparseable historical versions without crashing", () => {
    const path = "sahil/2026/devws/cc-2026-02-11.jsonl";
    writeDay(path, "totally not json\n");
    commitAll("garbage version");
    writeDay(path, entryLine("2026-02-11", 50.0));
    commitAll("good version");
    writeDay(path, entryLine("2026-02-11", 5.0));
    commitAll("shrunk version");
    const result = runScript("--repo", REPO);
    assert.equal(result.status, 0, result.stderr);
    assert.ok(result.stdout.includes("$50.00"), `expected max from parseable versions:\n${result.stdout}`);
    assert.ok(result.stdout.includes("+$45.00"), "expected delta vs current");
  });

  it("--write restores the full historical-max content byte-exactly, without committing", () => {
    seedShrunkRepo();
    const commitsBefore = commitCount();
    const result = runScript("--repo", REPO, "--write");
    assert.equal(result.status, 0, result.stderr);
    assert.ok(/Restored 2 file/.test(result.stdout), `expected restore summary:\n${result.stdout}`);
    // Full original content — token fields included, not just the cost
    assert.equal(readFileSync(join(REPO, SHRUNK), "utf-8"), HIGH);
    // Working tree only: no commit created, changes left for review
    assert.equal(commitCount(), commitsBefore);
    const status = execSync(`git -C "${REPO}" status --porcelain`, { encoding: "utf-8" });
    assert.ok(status.includes(SHRUNK), "expected restored file to show as modified");
  });

  it("is idempotent — a re-run after --write reports nothing to repair", () => {
    seedShrunkRepo();
    runScript("--repo", REPO, "--write");
    const second = runScript("--repo", REPO, "--write");
    assert.equal(second.status, 0, second.stderr);
    assert.ok(second.stdout.includes("Nothing to repair"), `expected idempotent re-run:\n${second.stdout}`);
    assert.equal(readFileSync(join(REPO, SHRUNK), "utf-8"), HIGH);
  });

  it("exits 1 with a stderr message when --repo is not a git repository", () => {
    const plain = join(TEST_DIR, "plain");
    mkdirSync(plain, { recursive: true });
    const result = runScript("--repo", plain);
    assert.equal(result.status, 1);
    assert.ok(result.stderr.includes("not a git repository"), `expected error, got: ${result.stderr}`);
  });

  it("exits 1 with a stderr message when --repo does not exist", () => {
    const result = runScript("--repo", join(TEST_DIR, "nonexistent"));
    assert.equal(result.status, 1);
    assert.ok(result.stderr.includes("repo not found"), `expected error, got: ${result.stderr}`);
  });

  it("exits 1 on unknown arguments", () => {
    const result = runScript("--frobnicate");
    assert.equal(result.status, 1);
    assert.ok(result.stderr.includes("unknown argument"), `expected usage error, got: ${result.stderr}`);
  });
});
