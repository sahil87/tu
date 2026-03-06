import { describe, it, beforeEach, afterEach } from "node:test";
import assert from "node:assert/strict";
import { writeFileSync, mkdirSync, rmSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";

import { runStatus, relativeTime } from "../src/cli.js";

const TEST_DIR = join(tmpdir(), "tu-status-test-" + process.pid);

const STOCK_DEFAULTS = `version = 2\nmode = single\nmetrics_dir = ~/.tu/metrics_repo\nmachine = $HOSTNAME\nuser = $USER\nauto_sync = true\n`;

function confPath(): string {
  return join(TEST_DIR, "tu.conf");
}

function tuHome(): string {
  return join(TEST_DIR, "tu-home");
}

function defaultsPath(): string {
  return join(TEST_DIR, "tu.default.conf");
}

function capture(fn: () => void): string[] {
  const logs: string[] = [];
  const orig = console.log;
  console.log = (...args: unknown[]) => logs.push(String(args[0]));
  try {
    fn();
  } finally {
    console.log = orig;
  }
  return logs;
}

describe("relativeTime", () => {
  it("shows <1m for less than 60 seconds", () => {
    assert.equal(relativeTime(30_000), "<1m ago");
  });

  it("shows minutes", () => {
    assert.equal(relativeTime(45 * 60 * 1000), "45m ago");
  });

  it("shows hours", () => {
    assert.equal(relativeTime(2 * 60 * 60 * 1000), "2h ago");
  });

  it("shows days", () => {
    assert.equal(relativeTime(3 * 24 * 60 * 60 * 1000), "3d ago");
  });

  it("clamps negative durations to <1m ago", () => {
    assert.equal(relativeTime(-5000), "<1m ago");
  });
});

describe("runStatus", () => {
  beforeEach(() => {
    mkdirSync(TEST_DIR, { recursive: true });
    mkdirSync(tuHome(), { recursive: true });
    writeFileSync(defaultsPath(), STOCK_DEFAULTS);
  });
  afterEach(() => rmSync(TEST_DIR, { recursive: true, force: true }));

  it("single mode — no config file", () => {
    const logs = capture(() => runStatus(confPath(), tuHome(), new Date(), defaultsPath()));
    assert.equal(logs.length, 1);
    assert.ok(logs[0].includes("single"));
    assert.ok(logs[0].includes("no "));
    assert.ok(logs[0].includes("tu.conf"));
  });

  it("single mode — config exists, mode not multi", () => {
    writeFileSync(confPath(), "version = 2\nmode = single\n");
    const logs = capture(() => runStatus(confPath(), tuHome(), new Date(), defaultsPath()));
    assert.equal(logs.length, 2);
    assert.ok(logs[0].includes("single"));
    assert.ok(logs[1].includes("(v2)"));
  });

  it("multi mode — complete setup", () => {
    const metricsDir = join(TEST_DIR, "metrics");
    mkdirSync(metricsDir, { recursive: true });
    writeFileSync(
      confPath(),
      `version = 2\nmode = multi\nmetrics_repo = git@example.com:repo.git\nmetrics_dir = ${metricsDir}\nuser = sahil\nmachine = testbox\nauto_sync = true\n`,
    );
    const syncTime = "2026-02-22T08:15:00Z";
    writeFileSync(join(tuHome(), ".last-sync"), syncTime + "\n");
    const now = new Date("2026-02-22T10:15:00Z"); // 2h later

    const logs = capture(() => runStatus(confPath(), tuHome(), now, defaultsPath()));
    assert.ok(logs.some((l) => l.includes("multi")));
    assert.ok(logs.some((l) => l.includes("sahil")));
    assert.ok(logs.some((l) => l.includes("testbox")));
    assert.ok(logs.some((l) => l.includes("(v2)")));
    assert.ok(logs.some((l) => l.includes(metricsDir) && !l.includes("NOT FOUND")));
    assert.ok(logs.some((l) => l.includes("2h ago") && l.includes(syncTime)));
    assert.ok(logs.some((l) => l.includes("on")));
  });

  it("multi mode — missing metrics dir", () => {
    const metricsDir = join(TEST_DIR, "nonexistent-metrics");
    writeFileSync(
      confPath(),
      `version = 2\nmode = multi\nmetrics_repo = git@example.com:repo.git\nmetrics_dir = ${metricsDir}\nuser = sahil\nmachine = testbox\nauto_sync = true\n`,
    );
    const logs = capture(() => runStatus(confPath(), tuHome(), new Date(), defaultsPath()));
    assert.ok(logs.some((l) => l.includes("NOT FOUND") && l.includes("tu init-metrics")));
  });

  it("multi mode — no prior sync", () => {
    const metricsDir = join(TEST_DIR, "metrics2");
    mkdirSync(metricsDir, { recursive: true });
    writeFileSync(
      confPath(),
      `version = 2\nmode = multi\nmetrics_repo = git@example.com:repo.git\nmetrics_dir = ${metricsDir}\nuser = sahil\nmachine = testbox\nauto_sync = true\n`,
    );
    // no .last-sync file
    const logs = capture(() => runStatus(confPath(), tuHome(), new Date(), defaultsPath()));
    assert.ok(logs.some((l) => l.includes("never")));
  });

  it("multi mode — auto_sync off", () => {
    const metricsDir = join(TEST_DIR, "metrics3");
    mkdirSync(metricsDir, { recursive: true });
    writeFileSync(
      confPath(),
      `version = 2\nmode = multi\nmetrics_repo = git@example.com:repo.git\nmetrics_dir = ${metricsDir}\nuser = sahil\nmachine = testbox\nauto_sync = false\n`,
    );
    const logs = capture(() => runStatus(confPath(), tuHome(), new Date(), defaultsPath()));
    assert.ok(logs.some((l) => l.includes("off")));
  });
});
