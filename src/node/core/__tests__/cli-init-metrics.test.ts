import { describe, it, beforeEach, afterEach } from "node:test";
import assert from "node:assert/strict";
import { writeFileSync, mkdirSync, rmSync, existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { execSync, spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { dirname } from "node:path";

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


describe("runInitMetrics with a repo-url argument", () => {
  function makePaths(dir: string) {
    return {
      configDir: join(dir, ".config", "tu"),
      userConf: join(dir, ".config", "tu", "tu.conf"),
      orgConf: join(dir, ".config", "tu", "org.conf"),
      legacyConf: join(dir, ".tu.conf"),
    };
  }

  beforeEach(() => {
    mkdirSync(TEST_DIR, { recursive: true });
    writeFileSync(defaultsPath(), STOCK_DEFAULTS);
  });
  afterEach(() => rmSync(TEST_DIR, { recursive: true, force: true }));

  it("creates tu.conf from defaults, writes metrics_repo, and clones", () => {
    const p = makePaths(TEST_DIR);
    const metricsDir = join(TEST_DIR, "metrics-clone");
    writeFileSync(defaultsPath(), `version = 2\nmetrics_dir = ${metricsDir}\nmachine = $HOSTNAME\nuser = $USER\nauto_sync = true\n`);
    const bareRepo = join(TEST_DIR, "bare-repo.git");
    execSync(`git init --bare "${bareRepo}"`, { stdio: "pipe" });

    const logs: string[] = [];
    const orig = console.log;
    console.log = (...args: unknown[]) => logs.push(String(args[0]));
    try {
      runInitMetrics(p, defaultsPath(), TEST_DIR, bareRepo);
      assert.ok(existsSync(p.userConf), "tu.conf should be created");
      const content = readFileSync(p.userConf, "utf-8");
      assert.ok(content.includes(`metrics_repo = ${bareRepo}`), `conf content:\n${content}`);
      assert.ok(logs.some((l) => l.includes(`Set metrics_repo = ${bareRepo}`)), `logs: ${logs}`);
      assert.ok(existsSync(metricsDir), "metricsDir should exist after clone");
      assert.ok(logs.some((l) => l.includes("Cloned")));
    } finally {
      console.log = orig;
    }
  });

  it("replaces an existing active metrics_repo line in place", () => {
    const p = makePaths(TEST_DIR);
    mkdirSync(p.configDir, { recursive: true });
    const metricsDir = join(TEST_DIR, "metrics-replace");
    mkdirSync(metricsDir, { recursive: true });
    execSync(`git init "${metricsDir}"`, { stdio: "pipe" });
    writeFileSync(p.userConf, `version = 2\nmetrics_repo = git@example.com:old.git\nmetrics_dir = ${metricsDir}\n`);
    const bareRepo = join(TEST_DIR, "bare-repo.git");
    execSync(`git init --bare "${bareRepo}"`, { stdio: "pipe" });

    const orig = console.log;
    console.log = () => {};
    try {
      runInitMetrics(p, defaultsPath(), TEST_DIR, bareRepo);
      const lines = readFileSync(p.userConf, "utf-8").split("\n");
      const active = lines.filter((l) => /^metrics_repo\s*=/.test(l));
      assert.equal(active.length, 1);
      assert.ok(active[0].includes(bareRepo));
    } finally {
      console.log = orig;
    }
  });

  it("replaces the scaffold's commented # metrics_repo line instead of appending", () => {
    const p = makePaths(TEST_DIR);
    mkdirSync(p.configDir, { recursive: true });
    const metricsDir = join(TEST_DIR, "metrics-commented");
    mkdirSync(metricsDir, { recursive: true });
    execSync(`git init "${metricsDir}"`, { stdio: "pipe" });
    writeFileSync(p.userConf, `# Git repo URL for metrics storage\n# metrics_repo = git@github.com:you/tu-metrics.git\nversion = 2\nmetrics_dir = ${metricsDir}\n`);
    const bareRepo = join(TEST_DIR, "bare-repo.git");
    execSync(`git init --bare "${bareRepo}"`, { stdio: "pipe" });

    const orig = console.log;
    console.log = () => {};
    try {
      runInitMetrics(p, defaultsPath(), TEST_DIR, bareRepo);
      const lines = readFileSync(p.userConf, "utf-8").split("\n");
      const active = lines.filter((l) => /^metrics_repo\s*=/.test(l));
      const commented = lines.filter((l) => /^\s*#\s*metrics_repo\s*=/.test(l));
      assert.equal(active.length, 1);
      assert.equal(commented.length, 0);
      assert.ok(active[0].includes(bareRepo));
    } finally {
      console.log = orig;
    }
  });

  it("prints Already initialized (exit 0) after the config write when metricsDir is a git repo", () => {
    const p = makePaths(TEST_DIR);
    mkdirSync(p.configDir, { recursive: true });
    const metricsDir = join(TEST_DIR, "metrics-idem");
    mkdirSync(metricsDir, { recursive: true });
    execSync(`git init "${metricsDir}"`, { stdio: "pipe" });
    writeFileSync(p.userConf, `version = 2\nmetrics_repo = git@example.com:old.git\nmetrics_dir = ${metricsDir}\n`);
    const bareRepo = join(TEST_DIR, "bare-repo.git");
    execSync(`git init --bare "${bareRepo}"`, { stdio: "pipe" });

    const logs: string[] = [];
    const orig = console.log;
    console.log = (...args: unknown[]) => logs.push(String(args[0]));
    try {
      runInitMetrics(p, defaultsPath(), TEST_DIR, bareRepo);
      assert.ok(logs.some((l) => l.includes(`Set metrics_repo = ${bareRepo}`)));
      assert.ok(logs.some((l) => l.includes("Already initialized")));
      const content = readFileSync(p.userConf, "utf-8");
      assert.ok(content.includes(`metrics_repo = ${bareRepo}`));
    } finally {
      console.log = orig;
    }
  });

  it("the URL argument beats an exported TU_METRICS_REPO (CLI > env)", () => {
    const p = makePaths(TEST_DIR);
    const metricsDir = join(TEST_DIR, "metrics-env");
    writeFileSync(defaultsPath(), `version = 2\nmetrics_dir = ${metricsDir}\nmachine = $HOSTNAME\nuser = $USER\nauto_sync = true\n`);
    const bareRepo = join(TEST_DIR, "bare-repo.git");
    execSync(`git init --bare "${bareRepo}"`, { stdio: "pipe" });
    const decoyRepo = join(TEST_DIR, "decoy-repo.git");
    execSync(`git init --bare "${decoyRepo}"`, { stdio: "pipe" });

    const origEnv = process.env.TU_METRICS_REPO;
    process.env.TU_METRICS_REPO = decoyRepo;
    const logs: string[] = [];
    const orig = console.log;
    console.log = (...args: unknown[]) => logs.push(String(args[0]));
    try {
      runInitMetrics(p, defaultsPath(), TEST_DIR, bareRepo);
      assert.ok(logs.some((l) => l.includes(`Cloned ${bareRepo}`)), `logs: ${logs}`);
      const remote = execSync(`git -C "${metricsDir}" remote get-url origin`, { encoding: "utf-8" }).trim();
      assert.equal(remote, bareRepo);
    } finally {
      console.log = orig;
      if (origEnv === undefined) delete process.env.TU_METRICS_REPO;
      else process.env.TU_METRICS_REPO = origEnv;
    }
  });

  it("seeds a newly created tu.conf from a legacy ~/.tu.conf", () => {
    const p = makePaths(TEST_DIR);
    writeFileSync(p.legacyConf, `version = 2\nmetrics_dir = ${join(TEST_DIR, "legacy-metrics")}\nuser = someone\n`);
    const bareRepo = join(TEST_DIR, "bare-repo.git");
    execSync(`git init --bare "${bareRepo}"`, { stdio: "pipe" });

    const logs: string[] = [];
    const orig = console.log;
    console.log = (...args: unknown[]) => logs.push(String(args[0]));
    try {
      runInitMetrics(p, defaultsPath(), TEST_DIR, bareRepo);
      assert.ok(logs.some((l) => l.includes("Copied") && l.includes(".tu.conf")));
      const content = readFileSync(p.userConf, "utf-8");
      assert.ok(content.includes("user = someone"), "legacy fields should be seeded");
      assert.ok(content.includes(`metrics_repo = ${bareRepo}`), "URL should be written into the seeded file");
    } finally {
      console.log = orig;
    }
  });
});

describe("tu init-metrics argument validation (subprocess)", () => {
  const __test_dirname = dirname(fileURLToPath(import.meta.url));
  const CLI = join(__test_dirname, "..", "cli.ts");

  it("two positional args exit 2 with the short usage on stderr", () => {
    const r = spawnSync("npx", ["tsx", CLI, "init-metrics", "a", "b"], { encoding: "utf-8" });
    assert.equal(r.status, 2, `stderr: ${r.stderr}`);
    assert.ok(r.stderr.includes("at most one argument"), `stderr: ${r.stderr}`);
    assert.ok(r.stderr.includes("Usage: tu [source] [period] [display]"), `stderr: ${r.stderr}`);
  });
});
