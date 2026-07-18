import { describe, it, beforeEach, afterEach } from "node:test";
import assert from "node:assert/strict";
import { writeFileSync, mkdirSync, rmSync, existsSync } from "node:fs";
import { join, resolve } from "node:path";
import { tmpdir } from "node:os";
import { execFileSync, execSync } from "node:child_process";
import { fileURLToPath } from "node:url";

import { runSync } from "../cli.js";

const TEST_DIR = join(tmpdir(), "tu-dry-run-test-" + process.pid);
const CLI_PATH = resolve(fileURLToPath(import.meta.url), "../../cli.ts");

const STOCK_DEFAULTS = `version = 2\nmetrics_dir = ~/.tu/metrics_repo\nmachine = $HOSTNAME\nuser = $USER\nauto_sync = true\n`;

function defaultsPath(): string {
  return join(TEST_DIR, "tu.default.conf");
}

function writeConf(name: string, content: string): string {
  const p = join(TEST_DIR, name);
  writeFileSync(p, content);
  return p;
}

// ---------------------------------------------------------------------------
// Misuse guard (main()): --dry-run is honored only by `tu sync`; any other
// invocation carrying it fails fast on stderr, exit 2 (usage error). Exercised end-to-end via
// the real CLI (the guard lives in main(), the only faithful surface). The env
// strips TU_METRICS_REPO so the guard result never depends on the dev shell.
// ---------------------------------------------------------------------------

describe("--dry-run misuse guard", () => {
  const EXPECTED = "Error: --dry-run is supported only with 'tu sync' — run 'tu sync --dry-run' to preview a sync.";

  function runCli(args: string[]): { status: number; stderr: string; stdout: string } {
    try {
      const stdout = execFileSync("npx", ["tsx", CLI_PATH, ...args], {
        encoding: "utf-8",
        env: { ...process.env, TU_METRICS_REPO: "" },
        stdio: ["ignore", "pipe", "pipe"],
      });
      return { status: 0, stdout, stderr: "" };
    } catch (err: unknown) {
      const e = err as { status?: number; stderr?: Buffer | string; stdout?: Buffer | string };
      return {
        status: e.status ?? -1,
        stderr: e.stderr ? String(e.stderr) : "",
        stdout: e.stdout ? String(e.stdout) : "",
      };
    }
  }

  it("errors and exits 2 for `tu cc --dry-run` (data command)", () => {
    const r = runCli(["cc", "--dry-run"]);
    assert.equal(r.status, 2);
    assert.ok(r.stderr.includes(EXPECTED), `stderr: ${r.stderr}`);
  });

  it("errors and exits 2 for `tu cc --sync --dry-run` (combined)", () => {
    const r = runCli(["cc", "--sync", "--dry-run"]);
    assert.equal(r.status, 2);
    assert.ok(r.stderr.includes(EXPECTED), `stderr: ${r.stderr}`);
  });

  it("errors and exits 2 for bare `tu --dry-run` (no subcommand)", () => {
    const r = runCli(["--dry-run"]);
    assert.equal(r.status, 2);
    assert.ok(r.stderr.includes(EXPECTED), `stderr: ${r.stderr}`);
  });
});

// ---------------------------------------------------------------------------
// runSync dry-run path: config/mode guards run identically to live sync (single
// mode still errors + exits 1), and the multi-mode happy path prints the
// preview to stdout and exits 0 without mutating the repo or .last-sync.
// ---------------------------------------------------------------------------

describe("runSync (dry-run)", () => {
  beforeEach(() => {
    mkdirSync(TEST_DIR, { recursive: true });
    writeFileSync(defaultsPath(), STOCK_DEFAULTS);
  });
  afterEach(() => rmSync(TEST_DIR, { recursive: true, force: true }));

  it("errors and exits 1 in single mode (guards run before the preview)", async () => {
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
      const path = writeConf("single.conf", "version = 2\n");
      await runSync(path, join(TEST_DIR, "tu-home"), defaultsPath(), true);
    } catch {
      // expected — process.exit throws
    } finally {
      console.error = origError;
      process.exit = origExit;
    }
    assert.equal(exitCode, 1);
    assert.ok(errors.some((e) => e.includes("metrics_repo")));
  });

  it("prints the preview to stdout, exits 0, and mutates nothing (multi mode)", async () => {
    const bareDir = join(TEST_DIR, "bare.git");
    const metricsDir = join(TEST_DIR, "metrics");
    const tuHome = join(TEST_DIR, "tu-home");
    const opts = { stdio: "pipe" as const };
    mkdirSync(tuHome, { recursive: true });
    execSync(`git init --bare "${bareDir}"`, opts);
    execSync(`git -C "${bareDir}" symbolic-ref HEAD refs/heads/main`, opts);
    execSync(`git clone "${bareDir}" "${metricsDir}"`, opts);
    execSync(`git -C "${metricsDir}" config user.email "test@test.com"`, opts);
    execSync(`git -C "${metricsDir}" config user.name "test"`, opts);
    writeFileSync(join(metricsDir, ".gitkeep"), "");
    execSync(`git -C "${metricsDir}" add .gitkeep`, opts);
    execSync(`git -C "${metricsDir}" commit -m "init"`, opts);
    execSync(`git -C "${metricsDir}" branch -M main`, opts);
    execSync(`git -C "${metricsDir}" push -u origin main`, opts);

    // Pre-stage a dirty day-file so the preview reports a would-commit.
    const userDir = join(metricsDir, "testuser", "2026", "testbox");
    mkdirSync(userDir, { recursive: true });
    writeFileSync(
      join(userDir, "cc-2026-07-18.jsonl"),
      JSON.stringify({ label: "2026-07-18", totalCost: 5.0, inputTokens: 1, outputTokens: 1, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 2 }) + "\n",
    );

    const path = writeConf(
      "multi.conf",
      `mode = multi\nmetrics_repo = git@example.com:repo.git\nmetrics_dir = ${metricsDir}\nmachine = testbox\nuser = testuser\n`,
    );

    const beforeLog = execSync(`git -C "${bareDir}" log --oneline`, { encoding: "utf-8" }).trim();

    const logs: string[] = [];
    const origLog = console.log;
    const origError = console.error;
    console.log = (...args: unknown[]) => logs.push(String(args[0]));
    console.error = () => {};
    try {
      await runSync(path, tuHome, defaultsPath(), true);
    } finally {
      console.log = origLog;
      console.error = origError;
    }

    const out = logs.join("\n");
    assert.ok(out.includes("Dry run — nothing written, committed, or pushed."), `preview: ${out}`);
    assert.ok(out.includes("Would commit:"), `preview: ${out}`);

    // No mutation: bare repo unchanged, no .last-sync.
    const afterLog = execSync(`git -C "${bareDir}" log --oneline`, { encoding: "utf-8" }).trim();
    assert.equal(afterLog, beforeLog, "dry-run must not add a commit");
    assert.ok(!existsSync(join(tuHome, ".last-sync")), "dry-run must not touch .last-sync");
  });
});
