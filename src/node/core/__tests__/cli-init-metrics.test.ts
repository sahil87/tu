import { describe, it, beforeEach, afterEach } from "node:test";
import assert from "node:assert/strict";
import { writeFileSync, mkdirSync, rmSync, existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { execSync } from "node:child_process";

import { runInitMetrics, checkMetricsDirGuard, removeCloneMarker } from "../cli.js";

const TEST_DIR = join(tmpdir(), "tu-init-metrics-test-" + process.pid);

const STOCK_DEFAULTS = `version = 2\nmode = single\nmetrics_dir = ~/.tu/metrics_repo\nmachine = $HOSTNAME\nuser = $USER\nauto_sync = true\n`;

function confPath(): string {
  return join(TEST_DIR, "tu.conf");
}

function defaultsPath(): string {
  return join(TEST_DIR, "tu.default.conf");
}

function writeConf(content: string): string {
  const p = confPath();
  writeFileSync(p, content);
  return p;
}

describe("runInitMetrics", () => {
  beforeEach(() => {
    mkdirSync(TEST_DIR, { recursive: true });
    writeFileSync(defaultsPath(), STOCK_DEFAULTS);
  });
  afterEach(() => rmSync(TEST_DIR, { recursive: true, force: true }));

  it("errors when no metrics_repo available (from defaults)", () => {
    // No user config, defaults have mode=single and no metrics_repo → should error about metrics_repo
    const errors: string[] = [];
    const origError = console.error;
    const origExit = process.exit;
    let exitCode: number | undefined;
    console.error = (...args: unknown[]) => errors.push(String(args[0]));
    process.exit = ((code: number) => {
      exitCode = code;
      throw new Error("exit");
    }) as never;
    try {
      runInitMetrics(join(TEST_DIR, "nonexistent.conf"), defaultsPath());
    } catch {
      // expected
    } finally {
      console.error = origError;
      process.exit = origExit;
    }
    assert.equal(exitCode, 1);
    assert.ok(errors.some((e) => e.includes("metrics_repo is not set")));
  });

  it("errors when user sets mode=single", () => {
    const errors: string[] = [];
    const origError = console.error;
    const origExit = process.exit;
    let exitCode: number | undefined;
    console.error = (...args: unknown[]) => errors.push(String(args[0]));
    process.exit = ((code: number) => {
      exitCode = code;
      throw new Error("exit");
    }) as never;
    try {
      const path = writeConf("mode = single\nmetrics_repo = git@example.com:repo.git\n");
      runInitMetrics(path, defaultsPath());
    } catch {
      // expected
    } finally {
      console.error = origError;
      process.exit = origExit;
    }
    assert.equal(exitCode, 1);
    assert.ok(errors.some((e) => e.includes("mode=single")));
  });

  it("errors when metrics_repo is missing from both defaults and user config", () => {
    const errors: string[] = [];
    const origError = console.error;
    const origExit = process.exit;
    let exitCode: number | undefined;
    console.error = (...args: unknown[]) => errors.push(String(args[0]));
    process.exit = ((code: number) => {
      exitCode = code;
      throw new Error("exit");
    }) as never;
    try {
      const path = writeConf("mode = multi\n");
      runInitMetrics(path, defaultsPath());
    } catch {
      // expected
    } finally {
      console.error = origError;
      process.exit = origExit;
    }
    assert.equal(exitCode, 1);
    assert.ok(errors.some((e) => e.includes("metrics_repo is not set")));
  });

  it("uses metrics_repo from defaults when user config omits it", () => {
    const metricsDir = join(TEST_DIR, "metrics");
    mkdirSync(metricsDir, { recursive: true });
    execSync(`git init "${metricsDir}"`, { stdio: "pipe" });

    // Defaults have metrics_repo, user config just sets mode and metricsDir
    writeFileSync(defaultsPath(), `version = 2\nmode = multi\nmetrics_repo = git@example.com:repo.git\nmetrics_dir = ${metricsDir}\n`);
    const path = writeConf("mode = multi\n");

    const logs: string[] = [];
    const orig = console.log;
    console.log = (...args: unknown[]) => logs.push(String(args[0]));
    try {
      runInitMetrics(path, defaultsPath());
      assert.ok(logs.some((l) => l.includes("Already initialized")));
    } finally {
      console.log = orig;
    }
  });

  it("reports already initialized when metricsDir is a git repo", () => {
    const metricsDir = join(TEST_DIR, "metrics");
    mkdirSync(metricsDir, { recursive: true });
    execSync(`git init "${metricsDir}"`, { stdio: "pipe" });

    const logs: string[] = [];
    const orig = console.log;
    console.log = (...args: unknown[]) => logs.push(String(args[0]));
    try {
      const path = writeConf(`mode = multi\nmetrics_repo = git@example.com:repo.git\nmetrics_dir = ${metricsDir}\n`);
      runInitMetrics(path, defaultsPath());
      assert.ok(logs.some((l) => l.includes("Already initialized")));
    } finally {
      console.log = orig;
    }
  });

  it("clones when metricsDir does not exist", () => {
    const metricsDir = join(TEST_DIR, "metrics-clone");
    // Create a bare repo to clone from
    const bareRepo = join(TEST_DIR, "bare-repo.git");
    execSync(`git init --bare "${bareRepo}"`, { stdio: "pipe" });

    const logs: string[] = [];
    const orig = console.log;
    console.log = (...args: unknown[]) => logs.push(String(args[0]));
    try {
      const path = writeConf(`mode = multi\nmetrics_repo = ${bareRepo}\nmetrics_dir = ${metricsDir}\n`);
      runInitMetrics(path, defaultsPath());
      assert.ok(existsSync(metricsDir), "metricsDir should exist after clone");
      assert.ok(logs.some((l) => l.includes("Cloned")));
      assert.ok(logs.some((l) => l.includes(bareRepo)));
    } finally {
      console.log = orig;
    }
  });

  it("errors when metricsDir exists but is not a git repo", () => {
    const metricsDir = join(TEST_DIR, "metrics");
    mkdirSync(metricsDir, { recursive: true });

    const errors: string[] = [];
    const origError = console.error;
    const origExit = process.exit;
    let exitCode: number | undefined;
    console.error = (...args: unknown[]) => errors.push(String(args[0]));
    process.exit = ((code: number) => {
      exitCode = code;
      throw new Error("exit");
    }) as never;
    try {
      const path = writeConf(`mode = multi\nmetrics_repo = git@example.com:repo.git\nmetrics_dir = ${metricsDir}\n`);
      runInitMetrics(path, defaultsPath());
    } catch {
      // expected
    } finally {
      console.error = origError;
      process.exit = origExit;
    }
    assert.equal(exitCode, 1);
    assert.ok(errors.some((e) => e.includes("not a git repo")));
  });
});

describe("checkMetricsDirGuard", () => {
  const tuHome = join(TEST_DIR, "tu-home");

  beforeEach(() => {
    mkdirSync(TEST_DIR, { recursive: true });
    mkdirSync(tuHome, { recursive: true });
  });
  afterEach(() => rmSync(TEST_DIR, { recursive: true, force: true }));

  it("passes through single mode unchanged", () => {
    const input = {
      version: 2,
      mode: "single" as const,
      metricsRepo: "",
      metricsDir: "/nonexistent/path/tu-metrics",
      machine: "test",
      user: "test",
      autoSync: true,
    };
    const result = checkMetricsDirGuard(input, tuHome);
    assert.equal(result.mode, "single");
  });

  it("passes through multi mode when metricsDir exists", () => {
    const dir = join(TEST_DIR, "existing-metrics");
    mkdirSync(dir, { recursive: true });
    const result = checkMetricsDirGuard({
      version: 2,
      mode: "multi",
      metricsRepo: "git@example.com:repo.git",
      metricsDir: dir,
      machine: "test",
      user: "test",
      autoSync: true,
    }, tuHome);
    assert.equal(result.mode, "multi");
  });

  it("falls back to single when metricsRepo is empty (no clone attempted)", () => {
    const stderrChunks: string[] = [];
    const origWrite = process.stderr.write;
    process.stderr.write = ((chunk: string) => { stderrChunks.push(chunk); return true; }) as typeof process.stderr.write;
    try {
      const result = checkMetricsDirGuard({
        version: 2,
        mode: "multi",
        metricsRepo: "",
        metricsDir: "/nonexistent/path/tu-metrics",
        machine: "test",
        user: "test",
        autoSync: true,
      }, tuHome);
      assert.equal(result.mode, "single");
      assert.ok(stderrChunks.some((s) => s.includes("falling back to single mode")));
    } finally {
      process.stderr.write = origWrite;
    }
  });

  it("auto-clones when metricsDir missing and metricsRepo set", () => {
    const metricsDir = join(TEST_DIR, "auto-clone-target");
    const bareRepo = join(TEST_DIR, "bare-repo.git");
    execSync(`git init --bare "${bareRepo}"`, { stdio: "pipe" });

    const stderrChunks: string[] = [];
    const origWrite = process.stderr.write;
    process.stderr.write = ((chunk: string) => { stderrChunks.push(chunk); return true; }) as typeof process.stderr.write;
    try {
      const result = checkMetricsDirGuard({
        version: 2,
        mode: "multi",
        metricsRepo: bareRepo,
        metricsDir,
        machine: "test",
        user: "test",
        autoSync: true,
      }, tuHome);
      assert.equal(result.mode, "multi");
      assert.ok(existsSync(metricsDir), "metricsDir should exist after auto-clone");
      assert.ok(stderrChunks.some((s) => s.includes("Cloned metrics repo")));
    } finally {
      process.stderr.write = origWrite;
    }
  });

  it("writes .clone-failed marker on clone failure", () => {
    const metricsDir = join(TEST_DIR, "auto-clone-fail");

    const stderrChunks: string[] = [];
    const origWrite = process.stderr.write;
    process.stderr.write = ((chunk: string) => { stderrChunks.push(chunk); return true; }) as typeof process.stderr.write;
    try {
      const result = checkMetricsDirGuard({
        version: 2,
        mode: "multi",
        metricsRepo: "git@nonexistent-host.invalid:repo.git",
        metricsDir,
        machine: "test",
        user: "test",
        autoSync: true,
      }, tuHome);
      assert.equal(result.mode, "single");
      assert.ok(existsSync(join(tuHome, ".clone-failed")), "marker should exist");
      const marker = readFileSync(join(tuHome, ".clone-failed"), "utf-8").trim();
      assert.ok(!Number.isNaN(new Date(marker).getTime()), "marker should be valid ISO date");
      assert.ok(stderrChunks.some((s) => s.includes("could not clone metrics repo")));
    } finally {
      process.stderr.write = origWrite;
    }
  });

  it("skips clone when fresh marker exists", () => {
    // Write a fresh marker (now)
    writeFileSync(join(tuHome, ".clone-failed"), new Date().toISOString());

    const stderrChunks: string[] = [];
    const origWrite = process.stderr.write;
    process.stderr.write = ((chunk: string) => { stderrChunks.push(chunk); return true; }) as typeof process.stderr.write;
    try {
      const result = checkMetricsDirGuard({
        version: 2,
        mode: "multi",
        metricsRepo: "git@example.com:repo.git",
        metricsDir: "/nonexistent/path",
        machine: "test",
        user: "test",
        autoSync: true,
      }, tuHome);
      assert.equal(result.mode, "single");
      assert.ok(stderrChunks.some((s) => s.includes("metrics repo not available")));
      // Should NOT contain "could not clone" (no clone attempted)
      assert.ok(!stderrChunks.some((s) => s.includes("could not clone")));
    } finally {
      process.stderr.write = origWrite;
    }
  });

  it("retries clone when marker is stale (> 3 hours)", () => {
    // Write a stale marker (4 hours ago)
    const staleDate = new Date(Date.now() - 4 * 60 * 60 * 1000);
    writeFileSync(join(tuHome, ".clone-failed"), staleDate.toISOString());

    const bareRepo = join(TEST_DIR, "bare-repo-stale.git");
    execSync(`git init --bare "${bareRepo}"`, { stdio: "pipe" });
    const metricsDir = join(TEST_DIR, "auto-clone-stale");

    const stderrChunks: string[] = [];
    const origWrite = process.stderr.write;
    process.stderr.write = ((chunk: string) => { stderrChunks.push(chunk); return true; }) as typeof process.stderr.write;
    try {
      const result = checkMetricsDirGuard({
        version: 2,
        mode: "multi",
        metricsRepo: bareRepo,
        metricsDir,
        machine: "test",
        user: "test",
        autoSync: true,
      }, tuHome);
      assert.equal(result.mode, "multi");
      assert.ok(existsSync(metricsDir), "metricsDir should exist after retry clone");
      // Marker should be cleaned up
      assert.ok(!existsSync(join(tuHome, ".clone-failed")), "marker should be removed on success");
    } finally {
      process.stderr.write = origWrite;
    }
  });

  it("treats malformed marker as stale (allows retry)", () => {
    writeFileSync(join(tuHome, ".clone-failed"), "not-a-date");

    const bareRepo = join(TEST_DIR, "bare-repo-malformed.git");
    execSync(`git init --bare "${bareRepo}"`, { stdio: "pipe" });
    const metricsDir = join(TEST_DIR, "auto-clone-malformed");

    const stderrChunks: string[] = [];
    const origWrite = process.stderr.write;
    process.stderr.write = ((chunk: string) => { stderrChunks.push(chunk); return true; }) as typeof process.stderr.write;
    try {
      const result = checkMetricsDirGuard({
        version: 2,
        mode: "multi",
        metricsRepo: bareRepo,
        metricsDir,
        machine: "test",
        user: "test",
        autoSync: true,
      }, tuHome);
      assert.equal(result.mode, "multi");
      assert.ok(existsSync(metricsDir));
    } finally {
      process.stderr.write = origWrite;
    }
  });
});

describe("runInitMetrics marker cleanup", () => {
  beforeEach(() => {
    mkdirSync(TEST_DIR, { recursive: true });
    writeFileSync(defaultsPath(), STOCK_DEFAULTS);
  });
  afterEach(() => rmSync(TEST_DIR, { recursive: true, force: true }));

  it("removes .clone-failed marker after successful clone", () => {
    const tuHome = join(TEST_DIR, "tu-home");
    mkdirSync(tuHome, { recursive: true });
    writeFileSync(join(tuHome, ".clone-failed"), new Date().toISOString());

    const metricsDir = join(TEST_DIR, "metrics-clone");
    const bareRepo = join(TEST_DIR, "bare-repo.git");
    execSync(`git init --bare "${bareRepo}"`, { stdio: "pipe" });

    const logs: string[] = [];
    const orig = console.log;
    console.log = (...args: unknown[]) => logs.push(String(args[0]));
    try {
      const path = writeConf(`mode = multi\nmetrics_repo = ${bareRepo}\nmetrics_dir = ${metricsDir}\n`);
      runInitMetrics(path, defaultsPath(), tuHome);
      assert.ok(existsSync(metricsDir), "metricsDir should exist after clone");
      assert.ok(!existsSync(join(tuHome, ".clone-failed")), ".clone-failed should be removed after successful init-metrics");
    } finally {
      console.log = orig;
    }
  });
});

describe("removeCloneMarker", () => {
  const tuHome = join(TEST_DIR, "tu-home");

  beforeEach(() => {
    mkdirSync(TEST_DIR, { recursive: true });
    mkdirSync(tuHome, { recursive: true });
  });
  afterEach(() => rmSync(TEST_DIR, { recursive: true, force: true }));

  it("removes marker when it exists", () => {
    writeFileSync(join(tuHome, ".clone-failed"), new Date().toISOString());
    assert.ok(existsSync(join(tuHome, ".clone-failed")));
    removeCloneMarker(tuHome);
    assert.ok(!existsSync(join(tuHome, ".clone-failed")));
  });

  it("is a no-op when marker does not exist", () => {
    assert.ok(!existsSync(join(tuHome, ".clone-failed")));
    removeCloneMarker(tuHome);
    assert.ok(!existsSync(join(tuHome, ".clone-failed")));
  });
});
