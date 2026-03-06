import { describe, it, beforeEach, afterEach } from "node:test";
import assert from "node:assert/strict";
import { writeFileSync, mkdirSync, rmSync, existsSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { execSync } from "node:child_process";

import { runSync } from "../cli.js";

const TEST_DIR = join(tmpdir(), "tu-sync-test-" + process.pid);

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

describe("runSync", () => {
  beforeEach(() => {
    mkdirSync(TEST_DIR, { recursive: true });
    writeFileSync(defaultsPath(), STOCK_DEFAULTS);
  });
  afterEach(() => rmSync(TEST_DIR, { recursive: true, force: true }));

  it("errors when config resolves to single mode (no user file)", async () => {
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
      await runSync(join(TEST_DIR, "nonexistent.conf"), join(TEST_DIR, "tu-home"), defaultsPath());
    } catch {
      // expected
    } finally {
      console.error = origError;
      process.exit = origExit;
    }
    assert.equal(exitCode, 1);
    assert.ok(errors.some((e) => e.includes("multi-machine mode")));
    assert.ok(errors.some((e) => e.includes("tu init-conf")));
  });

  it("errors when mode is not multi", async () => {
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
      const path = writeConf("mode = single\n");
      await runSync(path, join(TEST_DIR, "tu-home"), defaultsPath());
    } catch {
      // expected
    } finally {
      console.error = origError;
      process.exit = origExit;
    }
    assert.equal(exitCode, 1);
    assert.ok(errors.some((e) => e.includes("multi-machine mode")));
  });

  it("errors when metrics dir is missing and auto-clone fails", async () => {
    const stderrChunks: string[] = [];
    const origWrite = process.stderr.write;
    const origExit = process.exit;
    let exitCode: number | undefined;
    process.stderr.write = ((chunk: string) => { stderrChunks.push(chunk); return true; }) as typeof process.stderr.write;
    process.exit = ((code: number) => {
      exitCode = code;
      throw new Error("exit");
    }) as never;
    const tuHome = join(TEST_DIR, "tu-home");
    mkdirSync(tuHome, { recursive: true });
    try {
      const path = writeConf(
        `mode = multi\nmetrics_repo = /nonexistent/repo.git\nmetrics_dir = ${join(TEST_DIR, "nonexistent")}\n`,
      );
      await runSync(path, tuHome, defaultsPath());
    } catch {
      // expected
    } finally {
      process.stderr.write = origWrite;
      process.exit = origExit;
    }
    assert.equal(exitCode, 1);
    assert.ok(stderrChunks.some((s) => s.includes("could not clone metrics repo")));
  });

  it("writes date-partitioned metric files and touches .last-sync in tuHome", async () => {
    const bareDir = join(TEST_DIR, "bare.git");
    const metricsDir = join(TEST_DIR, "metrics");
    const tuHome = join(TEST_DIR, "tu-home");
    const opts = { stdio: "pipe" as const };
    mkdirSync(tuHome, { recursive: true });
    execSync(`git init --bare "${bareDir}"`, opts);
    execSync(`git clone "${bareDir}" "${metricsDir}"`, opts);
    execSync(`git -C "${metricsDir}" config user.email "test@test.com"`, opts);
    execSync(`git -C "${metricsDir}" config user.name "test"`, opts);
    // Initial commit so the repo has a branch
    writeFileSync(join(metricsDir, ".gitkeep"), "");
    execSync(`git -C "${metricsDir}" add .gitkeep`, opts);
    execSync(`git -C "${metricsDir}" commit -m "init"`, opts);
    execSync(`git -C "${metricsDir}" push`, opts);

    const path = writeConf(
      `mode = multi\nmetrics_repo = git@example.com:repo.git\nmetrics_dir = ${metricsDir}\nmachine = testbox\nuser = testuser\n`,
    );

    const logs: string[] = [];
    const origLog = console.log;
    const origError = console.error;
    console.error = () => {};
    console.log = (...args: unknown[]) => logs.push(String(args[0]));
    try {
      await runSync(path, tuHome, defaultsPath());

      // Verify success message
      assert.ok(
        logs.some((l) => l.includes("Synced")),
        `Expected success message, got: ${logs.join("; ")}`,
      );

      // Verify date-partitioned files exist under user dir
      const userDir = join(metricsDir, "testuser");
      assert.ok(existsSync(userDir), "Expected user directory to exist");

      // Verify .last-sync was touched in tuHome, not metricsDir
      assert.ok(existsSync(join(tuHome, ".last-sync")), "Expected .last-sync in tuHome");
      assert.ok(!existsSync(join(metricsDir, ".last-sync")), "Expected no .last-sync in metricsDir");
    } finally {
      console.log = origLog;
      console.error = origError;
    }
  });

  it("does not touch .last-sync when sync fails and exits 1", async () => {
    const metricsDir = join(TEST_DIR, "metrics-fail");
    const tuHome = join(TEST_DIR, "tu-home-fail");
    const opts = { stdio: "pipe" as const };
    mkdirSync(tuHome, { recursive: true });
    // Init a repo with no remote — push/pull will fail
    mkdirSync(metricsDir, { recursive: true });
    execSync(`git init "${metricsDir}"`, opts);
    execSync(`git -C "${metricsDir}" config user.email "test@test.com"`, opts);
    execSync(`git -C "${metricsDir}" config user.name "test"`, opts);
    writeFileSync(join(metricsDir, ".gitkeep"), "");
    execSync(`git -C "${metricsDir}" add .gitkeep`, opts);
    execSync(`git -C "${metricsDir}" commit -m "init"`, opts);

    const path = writeConf(
      `mode = multi\nmetrics_repo = git@example.com:repo.git\nmetrics_dir = ${metricsDir}\nmachine = testbox\nuser = testuser\n`,
    );

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
      await runSync(path, tuHome, defaultsPath());
    } catch {
      // expected — process.exit throws
    } finally {
      console.error = origError;
      process.exit = origExit;
    }

    assert.equal(exitCode, 1);
    assert.ok(errors.some((e) => e.includes("sync failed")), "Expected sync failure error");
    assert.ok(!existsSync(join(tuHome, ".last-sync")), "Expected .last-sync NOT to exist after failed sync");
  });
});
