import { describe, it, beforeEach, afterEach } from "node:test";
import assert from "node:assert/strict";
import { writeFileSync, mkdirSync, rmSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";

import { runInitConf } from "../cli.js";
import { DEFAULT_CONFIG_PATH } from "../config.js";

const TEST_DIR = join(tmpdir(), "tu-init-conf-test-" + process.pid);

function confPath(): string {
  return join(TEST_DIR, "tu.conf");
}

function writeConf(content: string): string {
  const p = confPath();
  writeFileSync(p, content);
  return p;
}

describe("runInitConf", () => {
  beforeEach(() => mkdirSync(TEST_DIR, { recursive: true }));
  afterEach(() => rmSync(TEST_DIR, { recursive: true, force: true }));

  it("creates config as a copy of the active default conf when file does not exist", () => {
    const logs: string[] = [];
    const orig = console.log;
    console.log = (...args: unknown[]) => logs.push(String(args[0]));
    try {
      const path = confPath();
      runInitConf(path);
      const content = readFileSync(path, "utf-8");
      const defaults = readFileSync(DEFAULT_CONFIG_PATH, "utf-8");
      assert.equal(content, defaults);
      assert.ok(content.includes("version = 2"));
      assert.ok(content.includes("metrics_dir = ~/.tu/metrics_repo"));
      assert.ok(content.includes("machine = $HOSTNAME"));
      assert.ok(content.includes("user = $USER"));
      assert.ok(content.includes("auto_sync = true"));
      assert.ok(logs.some((l) => l.includes("Created")));
    } finally {
      console.log = orig;
    }
  });

  it("scaffold is an exact copy of tu.default.conf", () => {
    const path = confPath();
    const orig = console.log;
    console.log = () => {};
    try {
      runInitConf(path);
    } finally {
      console.log = orig;
    }
    const scaffolded = readFileSync(path, "utf-8");
    const defaults = readFileSync(DEFAULT_CONFIG_PATH, "utf-8");
    assert.equal(scaffolded, defaults);
  });

  it("reports already complete when all fields present", () => {
    const logs: string[] = [];
    const orig = console.log;
    console.log = (...args: unknown[]) => logs.push(String(args[0]));
    try {
      const path = writeConf(
        "version = 2\nmode = multi\nmetrics_repo = git@example.com:repo.git\nmetrics_dir = /data\nmachine = macbook\nuser = sahil\nauto_sync = true\n",
      );
      runInitConf(path);
      assert.ok(logs.some((l) => l.includes("already complete")));
    } finally {
      console.log = orig;
    }
  });

  it("warns about commented-out fields without re-appending them", () => {
    const logs: string[] = [];
    const orig = console.log;
    console.log = (...args: unknown[]) => logs.push(String(args[0]));
    try {
      const path = writeConf(
        "version = 2\nmode = single\n# metrics_repo = git@example.com:repo.git\n# metrics_dir = ~/.tu/metrics_repo\n# machine = macbook\n# user = someone\nauto_sync = true\n",
      );
      const before = readFileSync(path, "utf-8");
      runInitConf(path);
      const after = readFileSync(path, "utf-8");
      // file should not grow (no duplicate appends)
      assert.equal(before, after);
      // should warn about commented-out fields
      assert.ok(logs.some((l) => l.includes("commented-out") && l.includes("uncommenting")));
    } finally {
      console.log = orig;
    }
  });

  it("considers uncommented fields as present", () => {
    const logs: string[] = [];
    const orig = console.log;
    console.log = (...args: unknown[]) => logs.push(String(args[0]));
    try {
      const path = writeConf(
        "version = 2\nmode = multi\nmetrics_repo = git@example.com:repo.git\nmetrics_dir = /data\nmachine = macbook\nuser = sahil\nauto_sync = true\n",
      );
      runInitConf(path);
      assert.ok(logs.some((l) => l.includes("already complete")));
    } finally {
      console.log = orig;
    }
  });

  it("detects mixed commented and uncommented — uncommented wins", () => {
    const logs: string[] = [];
    const orig = console.log;
    console.log = (...args: unknown[]) => logs.push(String(args[0]));
    try {
      const path = writeConf(
        "version = 2\nmode = single\n# metrics_repo = old\nmetrics_repo = new\n# metrics_dir = ~/.tu/metrics_repo\nmetrics_dir = /data\n# machine = old\nmachine = macbook\n# user = old\nuser = sahil\nauto_sync = true\n",
      );
      runInitConf(path);
      assert.ok(logs.some((l) => l.includes("already complete")));
    } finally {
      console.log = orig;
    }
  });

  it("appends missing fields to existing config", () => {
    const logs: string[] = [];
    const orig = console.log;
    console.log = (...args: unknown[]) => logs.push(String(args[0]));
    try {
      const path = writeConf("mode = multi\nmetrics_repo = git@example.com:repo.git\n");
      runInitConf(path);
      const content = readFileSync(path, "utf-8");
      assert.ok(content.includes("# metrics_dir"));
      assert.ok(content.includes("# machine"));
      assert.ok(content.includes("version ="));
      assert.ok(content.includes("# user"));
      assert.ok(content.includes("auto_sync"));
      assert.ok(content.startsWith("mode = multi"));
      assert.ok(logs.some((l) => l.includes("added missing fields")));
    } finally {
      console.log = orig;
    }
  });

  it("preserves existing content when appending", () => {
    const original = "mode = multi\nmetrics_repo = git@example.com:my-repo.git\n";
    const path = writeConf(original);
    const orig = console.log;
    console.log = () => {};
    try {
      runInitConf(path);
    } finally {
      console.log = orig;
    }
    const content = readFileSync(path, "utf-8");
    assert.ok(content.startsWith(original));
  });

  it("is idempotent — second run on complete config does nothing", () => {
    const path = confPath();
    const orig = console.log;
    console.log = () => {};
    try {
      runInitConf(path);
      const first = readFileSync(path, "utf-8");
      runInitConf(path);
      const second = readFileSync(path, "utf-8");
      assert.equal(first, second);
    } finally {
      console.log = orig;
    }
  });
});
