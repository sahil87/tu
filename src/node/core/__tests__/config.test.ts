import { describe, it, beforeEach, afterEach } from "node:test";
import assert from "node:assert/strict";
import { writeFileSync, mkdirSync, rmSync } from "node:fs";
import { join } from "node:path";
import { tmpdir, hostname, userInfo } from "node:os";

import { readConfig, resolveConfigPaths, ConfigHomeError, CURRENT_CONFIG_VERSION, DEFAULT_CONFIG_PATH } from "../config.js";

const TEST_DIR = join(tmpdir(), "tu-config-test-" + process.pid);

function confPath(name = "tu.conf"): string {
  return join(TEST_DIR, name);
}

function writeConf(content: string, name = "tu.conf"): string {
  const p = confPath(name);
  writeFileSync(p, content);
  return p;
}

// Helper: write a minimal defaults file for tests that need to isolate from the real one
function writeDefaults(content: string): string {
  const p = join(TEST_DIR, "tu.default.conf");
  writeFileSync(p, content);
  return p;
}

const STOCK_DEFAULTS = `version = 2\nmetrics_dir = ~/.tu/metrics_repo\nmachine = $HOSTNAME\nuser = $USER\nauto_sync = true\n`;

describe("readConfig", () => {
  beforeEach(() => mkdirSync(TEST_DIR, { recursive: true }));
  afterEach(() => rmSync(TEST_DIR, { recursive: true, force: true }));

  it("returns single-mode config when user file does not exist", () => {
    const defaults = writeDefaults(STOCK_DEFAULTS);
    const cfg = readConfig(confPath("nonexistent"), defaults);
    assert.equal(cfg.mode, "single");
    assert.equal(cfg.version, 2);
    assert.equal(cfg.machine, hostname());
    assert.equal(cfg.user, userInfo().username);
    assert.equal(cfg.autoSync, true);
  });

  it("returns TuConfig with mode=multi when user sets mode=multi with metrics_repo", () => {
    const defaults = writeDefaults(STOCK_DEFAULTS);
    const path = writeConf("mode = multi\nmetrics_repo = git@github.com:you/tu-metrics.git\n");
    const cfg = readConfig(path, defaults);
    assert.equal(cfg.mode, "multi");
    assert.equal(cfg.metricsRepo, "git@github.com:you/tu-metrics.git");
    assert.equal(typeof cfg.metricsDir, "string");
    assert.equal(typeof cfg.machine, "string");
    assert.equal(cfg.version, 2);
    assert.equal(cfg.user, userInfo().username);
    assert.equal(cfg.autoSync, true);
  });

  it("derives multi mode from metrics_repo presence, ignoring explicit mode=single", () => {
    const defaults = writeDefaults(STOCK_DEFAULTS);
    const path = writeConf("mode = single\nmetrics_repo = git@example.com:repo.git\n");
    const cfg = readConfig(path, defaults);
    assert.equal(cfg.mode, "multi");
  });

  it("derives multi mode when user file has metrics_repo but no mode field", () => {
    const defaults = writeDefaults(STOCK_DEFAULTS);
    const path = writeConf("metrics_repo = git@example.com:repo.git\n");
    const cfg = readConfig(path, defaults);
    assert.equal(cfg.mode, "multi");
  });

  it("derives single mode when mode=multi but metrics_repo is missing", () => {
    const defaults = writeDefaults(STOCK_DEFAULTS);
    const path = writeConf("mode = multi\n");
    const cfg = readConfig(path, defaults);
    assert.equal(cfg.mode, "single");
  });

  it("applies default metricsDir and machine when omitted", () => {
    const defaults = writeDefaults(STOCK_DEFAULTS);
    const path = writeConf("mode = multi\nmetrics_repo = git@example.com:repo.git\n");
    const cfg = readConfig(path, defaults);
    assert.ok(cfg.metricsDir.length > 0);
    assert.ok(cfg.machine.length > 0);
    assert.ok(!cfg.metricsDir.includes("~"));
  });

  it("uses provided metrics_dir and machine from user config", () => {
    const defaults = writeDefaults(STOCK_DEFAULTS);
    const path = writeConf("mode = multi\nmetrics_repo = git@example.com:repo.git\nmetrics_dir = /data/metrics\nmachine = macbook\n");
    const cfg = readConfig(path, defaults);
    assert.equal(cfg.metricsDir, "/data/metrics");
    assert.equal(cfg.machine, "macbook");
  });

  it("ignores comment lines", () => {
    const defaults = writeDefaults(STOCK_DEFAULTS);
    const path = writeConf("# comment\nmode = multi\n# another comment\nmetrics_repo = git@example.com:repo.git\n");
    const cfg = readConfig(path, defaults);
    assert.equal(cfg.mode, "multi");
  });

  it("trims whitespace around keys and values", () => {
    const defaults = writeDefaults(STOCK_DEFAULTS);
    const path = writeConf("  mode  =  multi  \n  metrics_repo  =  git@example.com:repo.git  \n");
    const cfg = readConfig(path, defaults);
    assert.equal(cfg.mode, "multi");
    assert.equal(cfg.metricsRepo, "git@example.com:repo.git");
  });

  it("returns single mode for empty user file", () => {
    const defaults = writeDefaults(STOCK_DEFAULTS);
    const path = writeConf("");
    const cfg = readConfig(path, defaults);
    assert.equal(cfg.mode, "single");
  });

  it("returns single mode for file with only comments", () => {
    const defaults = writeDefaults(STOCK_DEFAULTS);
    const path = writeConf("# just comments\n# nothing else\n");
    const cfg = readConfig(path, defaults);
    assert.equal(cfg.mode, "single");
  });

  it("user version overrides default version", () => {
    const defaults = writeDefaults(STOCK_DEFAULTS);
    const path = writeConf("version = 1\nmode = multi\nmetrics_repo = git@example.com:repo.git\n");
    const cfg = readConfig(path, defaults);
    assert.equal(cfg.version, 1);
  });

  it("parses explicit version as integer", () => {
    const defaults = writeDefaults(STOCK_DEFAULTS);
    const path = writeConf("version = 2\nmode = multi\nmetrics_repo = git@example.com:repo.git\n");
    const cfg = readConfig(path, defaults);
    assert.equal(cfg.version, 2);
  });

  it("warns on stderr when version is ahead of CURRENT_CONFIG_VERSION", () => {
    const defaults = writeDefaults(STOCK_DEFAULTS);
    const path = writeConf("version = 99\nmode = multi\nmetrics_repo = git@example.com:repo.git\n");
    const errors: string[] = [];
    const origError = console.error;
    console.error = (...args: unknown[]) => errors.push(String(args[0]));
    try {
      const cfg = readConfig(path, defaults);
      assert.equal(cfg.version, 99);
      assert.ok(errors.some((e) => e.includes("version 99 is newer than tu supports")));
    } finally {
      console.error = origError;
    }
  });

  it("does not warn when version equals CURRENT_CONFIG_VERSION", () => {
    const defaults = writeDefaults(STOCK_DEFAULTS);
    const path = writeConf(`version = ${CURRENT_CONFIG_VERSION}\nmode = multi\nmetrics_repo = git@example.com:repo.git\n`);
    const errors: string[] = [];
    const origError = console.error;
    console.error = (...args: unknown[]) => errors.push(String(args[0]));
    try {
      readConfig(path, defaults);
      assert.equal(errors.length, 0);
    } finally {
      console.error = origError;
    }
  });

  it("expands $USER sentinel to system username", () => {
    const defaults = writeDefaults("version = 2\nmode = single\nuser = $USER\n");
    const cfg = readConfig(confPath("nonexistent"), defaults);
    assert.equal(cfg.user, userInfo().username);
  });

  it("expands $HOSTNAME sentinel to system hostname", () => {
    const defaults = writeDefaults("version = 2\nmode = single\nmachine = $HOSTNAME\n");
    const cfg = readConfig(confPath("nonexistent"), defaults);
    assert.equal(cfg.machine, hostname());
  });

  it("user value overrides sentinel default", () => {
    const defaults = writeDefaults("version = 2\nmode = single\nuser = $USER\nmachine = $HOSTNAME\n");
    const path = writeConf("user = sahil\nmachine = macbook\n");
    const cfg = readConfig(path, defaults);
    assert.equal(cfg.user, "sahil");
    assert.equal(cfg.machine, "macbook");
  });

  it("uses explicit user from user config", () => {
    const defaults = writeDefaults(STOCK_DEFAULTS);
    const path = writeConf("mode = multi\nmetrics_repo = git@example.com:repo.git\nuser = sahil\n");
    const cfg = readConfig(path, defaults);
    assert.equal(cfg.user, "sahil");
  });

  it("defaults autoSync to true from defaults file", () => {
    const defaults = writeDefaults(STOCK_DEFAULTS);
    const path = writeConf("mode = multi\nmetrics_repo = git@example.com:repo.git\n");
    const cfg = readConfig(path, defaults);
    assert.equal(cfg.autoSync, true);
  });

  it("parses auto_sync = false as false", () => {
    const defaults = writeDefaults(STOCK_DEFAULTS);
    const path = writeConf("mode = multi\nmetrics_repo = git@example.com:repo.git\nauto_sync = false\n");
    const cfg = readConfig(path, defaults);
    assert.equal(cfg.autoSync, false);
  });

  it("parses auto_sync = 0 as false", () => {
    const defaults = writeDefaults(STOCK_DEFAULTS);
    const path = writeConf("mode = multi\nmetrics_repo = git@example.com:repo.git\nauto_sync = 0\n");
    const cfg = readConfig(path, defaults);
    assert.equal(cfg.autoSync, false);
  });

  it("parses auto_sync = true as true", () => {
    const defaults = writeDefaults(STOCK_DEFAULTS);
    const path = writeConf("mode = multi\nmetrics_repo = git@example.com:repo.git\nauto_sync = true\n");
    const cfg = readConfig(path, defaults);
    assert.equal(cfg.autoSync, true);
  });

  it("includes all new fields in valid multi-mode config", () => {
    const defaults = writeDefaults(STOCK_DEFAULTS);
    const path = writeConf("version = 2\nmode = multi\nmetrics_repo = git@example.com:repo.git\nuser = sahil\nauto_sync = false\n");
    const cfg = readConfig(path, defaults);
    assert.equal(cfg.version, 2);
    assert.equal(cfg.user, "sahil");
    assert.equal(cfg.autoSync, false);
  });

  it("works with the real default conf shipped in the package", () => {
    const cfg = readConfig(confPath("nonexistent"), DEFAULT_CONFIG_PATH);
    assert.equal(cfg.version, 2);
    assert.equal(cfg.machine, hostname());
    assert.equal(cfg.user, userInfo().username);
    assert.equal(cfg.mode, "single");
    assert.equal(cfg.metricsRepo, "");
  });

  it("TU_METRICS_REPO env var overrides config file and enables multi mode", () => {
    const defaults = writeDefaults(STOCK_DEFAULTS);
    const path = writeConf("version = 2\n");
    const origEnv = process.env.TU_METRICS_REPO;
    process.env.TU_METRICS_REPO = "git@github.com:team/metrics.git";
    try {
      const cfg = readConfig(path, defaults);
      assert.equal(cfg.metricsRepo, "git@github.com:team/metrics.git");
      assert.equal(cfg.mode, "multi");
    } finally {
      if (origEnv === undefined) delete process.env.TU_METRICS_REPO;
      else process.env.TU_METRICS_REPO = origEnv;
    }
  });

  it("empty TU_METRICS_REPO does not override config file value", () => {
    const defaults = writeDefaults(STOCK_DEFAULTS);
    const path = writeConf("metrics_repo = git@example.com:repo.git\n");
    const origEnv = process.env.TU_METRICS_REPO;
    process.env.TU_METRICS_REPO = "";
    try {
      const cfg = readConfig(path, defaults);
      assert.equal(cfg.metricsRepo, "git@example.com:repo.git");
      assert.equal(cfg.mode, "multi");
    } finally {
      if (origEnv === undefined) delete process.env.TU_METRICS_REPO;
      else process.env.TU_METRICS_REPO = origEnv;
    }
  });

  it("falls back gracefully when both user and defaults files are missing", () => {
    const cfg = readConfig(confPath("nonexistent"), confPath("also-nonexistent"));
    assert.equal(cfg.mode, "single");
    assert.equal(cfg.metricsRepo, "");
  });
});

// shll config-home standard: the config root is $HOME/.config/tu, built from
// $HOME and nothing else; no env var can move it; unset $HOME is an
// actionable error.
describe("resolveConfigPaths", () => {
  it("builds all paths under $HOME/.config/tu (plus the legacy path)", () => {
    const p = resolveConfigPaths({ HOME: "/home/alice" });
    assert.equal(p.configDir, "/home/alice/.config/tu");
    assert.equal(p.userConf, "/home/alice/.config/tu/tu.conf");
    assert.equal(p.orgConf, "/home/alice/.config/tu/org.conf");
    assert.equal(p.legacyConf, "/home/alice/.tu.conf");
  });

  it("pin: XDG_CONFIG_HOME, TU_CONFIG, TU_CONFIG_HOME cannot move the path", () => {
    const p = resolveConfigPaths({
      HOME: "/home/alice",
      XDG_CONFIG_HOME: "/elsewhere/xdg",
      TU_CONFIG: "/elsewhere/tu.conf",
      TU_CONFIG_HOME: "/elsewhere/tu",
      TU_HOME: "/elsewhere/state",
    });
    assert.equal(p.userConf, "/home/alice/.config/tu/tu.conf");
  });

  it("throws an actionable error when HOME is unset", () => {
    assert.throws(() => resolveConfigPaths({}), (e: unknown) => {
      assert.ok(e instanceof ConfigHomeError);
      assert.ok(String(e).includes("$HOME is not set"));
      return true;
    });
  });

  it("throws an actionable error when HOME is empty", () => {
    assert.throws(() => resolveConfigPaths({ HOME: "" }), /\$HOME is not set/);
  });
});

function makePaths(dir: string) {
  return {
    configDir: join(dir, ".config", "tu"),
    userConf: join(dir, ".config", "tu", "tu.conf"),
    orgConf: join(dir, ".config", "tu", "org.conf"),
    legacyConf: join(dir, ".tu.conf"),
  };
}

function captureStderr(fn: () => void): string[] {
  const errors: string[] = [];
  const origError = console.error;
  console.error = (...args: unknown[]) => errors.push(String(args[0]));
  try {
    fn();
  } finally {
    console.error = origError;
  }
  return errors;
}

describe("readConfig org layer and cascade", () => {
  beforeEach(() => mkdirSync(TEST_DIR, { recursive: true }));
  afterEach(() => rmSync(TEST_DIR, { recursive: true, force: true }));

  it("org.conf alone enables multi mode, silently", () => {
    const defaults = writeDefaults(STOCK_DEFAULTS);
    const p = makePaths(TEST_DIR);
    mkdirSync(p.configDir, { recursive: true });
    writeFileSync(p.orgConf, "metrics_repo = git@example.com:org.git\n");
    const errors = captureStderr(() => {
      const cfg = readConfig(p, defaults);
      assert.equal(cfg.mode, "multi");
      assert.equal(cfg.metricsRepo, "git@example.com:org.git");
    });
    assert.equal(errors.length, 0);
  });

  it("user conf overrides org.conf for the same key", () => {
    const defaults = writeDefaults(STOCK_DEFAULTS);
    const p = makePaths(TEST_DIR);
    mkdirSync(p.configDir, { recursive: true });
    writeFileSync(p.orgConf, "metrics_repo = git@example.com:org.git\n");
    writeFileSync(p.userConf, "metrics_repo = git@example.com:user.git\n");
    const cfg = readConfig(p, defaults);
    assert.equal(cfg.metricsRepo, "git@example.com:user.git");
  });

  it("TU_METRICS_REPO overrides user conf", () => {
    const defaults = writeDefaults(STOCK_DEFAULTS);
    const p = makePaths(TEST_DIR);
    mkdirSync(p.configDir, { recursive: true });
    writeFileSync(p.userConf, "metrics_repo = git@example.com:user.git\n");
    const origEnv = process.env.TU_METRICS_REPO;
    process.env.TU_METRICS_REPO = "git@example.com:env.git";
    try {
      const cfg = readConfig(p, defaults);
      assert.equal(cfg.metricsRepo, "git@example.com:env.git");
    } finally {
      if (origEnv === undefined) delete process.env.TU_METRICS_REPO;
      else process.env.TU_METRICS_REPO = origEnv;
    }
  });

  it("CLI overrides argument beats TU_METRICS_REPO", () => {
    const defaults = writeDefaults(STOCK_DEFAULTS);
    const p = makePaths(TEST_DIR);
    mkdirSync(p.configDir, { recursive: true });
    writeFileSync(p.userConf, "metrics_repo = git@example.com:user.git\n");
    const origEnv = process.env.TU_METRICS_REPO;
    process.env.TU_METRICS_REPO = "git@example.com:env.git";
    try {
      const cfg = readConfig(p, defaults, { metrics_repo: "git@example.com:cli.git" });
      assert.equal(cfg.metricsRepo, "git@example.com:cli.git");
    } finally {
      if (origEnv === undefined) delete process.env.TU_METRICS_REPO;
      else process.env.TU_METRICS_REPO = origEnv;
    }
  });

  it("legacy ~/.tu.conf is read when the new file is absent, warning once across two calls", () => {
    const defaults = writeDefaults(STOCK_DEFAULTS);
    const p = makePaths(TEST_DIR);
    writeFileSync(p.legacyConf, "metrics_repo = git@example.com:legacy.git\n");
    const errors = captureStderr(() => {
      const a = readConfig(p, defaults);
      const b = readConfig(p, defaults);
      assert.equal(a.metricsRepo, "git@example.com:legacy.git");
      assert.equal(b.metricsRepo, "git@example.com:legacy.git");
      assert.equal(a.mode, "multi");
    });
    const warnings = errors.filter((e) => e.includes("is deprecated; move it to ~/.config/tu/tu.conf"));
    assert.equal(warnings.length, 1);
  });

  it("new file wins over legacy, silently (no deprecation warning)", () => {
    const defaults = writeDefaults(STOCK_DEFAULTS);
    const p = makePaths(TEST_DIR);
    mkdirSync(p.configDir, { recursive: true });
    writeFileSync(p.userConf, "metrics_repo = git@example.com:new.git\n");
    writeFileSync(p.legacyConf, "metrics_repo = git@example.com:legacy.git\n");
    const errors = captureStderr(() => {
      const cfg = readConfig(p, defaults);
      assert.equal(cfg.metricsRepo, "git@example.com:new.git");
    });
    assert.equal(errors.filter((e) => e.includes("is deprecated")).length, 0);
  });
});
