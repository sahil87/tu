import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";

// ---------------------------------------------------------------------------
// End-to-end exit-code contract (shll toolkit principle №4):
//   0 = success, 1 = operational failure, 2 = usage error.
//
// These assertions run the real CLI as a subprocess (via tsx) so they exercise
// the actual `process.exit(EXIT_USAGE)` calls in main()/parseGlobalFlags — the
// machine-observable contract downstream scripts branch on. The per-branch
// error MESSAGES are already covered at the unit level (cli-parser.test.ts,
// cli-watch-flag.test.ts, completions.test.ts); this file pins the process
// exit CODE for the previously-uncovered usage-error paths that only surface
// through main() (unknown argument) plus a representative flag-parsing path.
// ---------------------------------------------------------------------------

const __dirname = dirname(fileURLToPath(import.meta.url));
const CLI = join(__dirname, "..", "cli.ts");

function runCli(args: string[]): { status: number | null; stderr: string; stdout: string } {
  const r = spawnSync("npx", ["tsx", CLI, ...args], { encoding: "utf-8" });
  return { status: r.status, stderr: r.stderr, stdout: r.stdout };
}

describe("exit codes: usage errors exit 2", () => {
  it("unknown argument exits 2 and prints the error + short usage to stderr", () => {
    const r = runCli(["bogus-arg"]);
    assert.equal(r.status, 2);
    assert.ok(r.stderr.includes("Unknown argument: bogus-arg"), `stderr: ${r.stderr}`);
    assert.ok(r.stderr.includes("Usage: tu [source] [period] [display]"), `stderr: ${r.stderr}`);
  });

  it("unknown argument after a valid source exits 2", () => {
    const r = runCli(["cc", "xyz"]);
    assert.equal(r.status, 2);
    assert.ok(r.stderr.includes("Unknown argument: xyz"), `stderr: ${r.stderr}`);
  });

  it("-u with no username exits 2", () => {
    const r = runCli(["cc", "-w", "-u"]);
    assert.equal(r.status, 2);
    assert.ok(r.stderr.includes("-u requires a username"), `stderr: ${r.stderr}`);
  });

  it("--metric with an invalid value exits 2", () => {
    const r = runCli(["mh", "--metric", "bogus"]);
    assert.equal(r.status, 2);
    assert.ok(r.stderr.includes("--metric requires 'tokens' or 'cost'"), `stderr: ${r.stderr}`);
  });

  it("bare --metric (no value) exits 2", () => {
    const r = runCli(["mh", "--metric"]);
    assert.equal(r.status, 2);
    assert.ok(r.stderr.includes("--metric requires 'tokens' or 'cost'"), `stderr: ${r.stderr}`);
  });

  it("incompatible format flags exit 2", () => {
    const r = runCli(["cc", "--json", "--csv"]);
    assert.equal(r.status, 2);
    assert.ok(r.stderr.includes("--json and --csv are incompatible"), `stderr: ${r.stderr}`);
  });
});

describe("exit codes: success paths do not use the usage-error code", () => {
  it("`tu help` exits 0 (not a usage error)", () => {
    const r = runCli(["help"]);
    assert.equal(r.status, 0);
  });
});

// Config-dependent paths: run with an isolated HOME so the dev machine's
// ~/.tu.conf and TU_METRICS_REPO never leak into the assertion.
describe("exit codes: -u all and the reserved username guard", () => {
  function runCliWithConf(conf: string, args: string[]): { status: number | null; stderr: string; stdout: string } {
    const home = mkdtempSync(join(tmpdir(), "tu-exit-codes-"));
    try {
      writeFileSync(join(home, ".tu.conf"), conf);
      const env: NodeJS.ProcessEnv = { ...process.env, HOME: home };
      delete env.TU_METRICS_REPO;
      const r = spawnSync("npx", ["tsx", CLI, ...args], { encoding: "utf-8", env });
      return { status: r.status, stderr: r.stderr, stdout: r.stdout };
    } finally {
      rmSync(home, { recursive: true, force: true });
    }
  }

  it("-u all in single mode warns (existing -u guard) and exits 0", () => {
    const r = runCliWithConf("", ["cc", "m", "-u", "all", "--no-color"]);
    assert.equal(r.status, 0, `stderr: ${r.stderr}`);
    assert.ok(r.stderr.includes("-u flag requires multi mode"), `stderr: ${r.stderr}`);
  });

  it("config user = all is rejected with exit 2", () => {
    const r = runCliWithConf("user = all\n", ["cc", "--no-color"]);
    assert.equal(r.status, 2);
    assert.ok(r.stderr.includes('config user "all" is reserved'), `stderr: ${r.stderr}`);
  });

  it("-t on a snapshot display warns and exits 0", () => {
    const r = runCliWithConf("", ["-t", "--no-color"]);
    assert.equal(r.status, 0, `stderr: ${r.stderr}`);
    assert.ok(r.stderr.includes("--total applies to all-tools history"), `stderr: ${r.stderr}`);
  });

  it("-t on a single-tool history warns and exits 0", () => {
    const r = runCliWithConf("", ["cc", "mh", "-t", "--no-color"]);
    assert.equal(r.status, 0, `stderr: ${r.stderr}`);
    assert.ok(r.stderr.includes("--total applies to all-tools history"), `stderr: ${r.stderr}`);
  });

  it("`tu sync` with config user = all is rejected with exit 2 before any write", () => {
    const r = runCliWithConf("user = all\n", ["sync"]);
    assert.equal(r.status, 2);
    assert.ok(r.stderr.includes('config user "all" is reserved'), `stderr: ${r.stderr}`);
  });
});
