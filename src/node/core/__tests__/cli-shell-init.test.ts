import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

// ---------------------------------------------------------------------------
// Toolkit `shell-init` standard conformance (shll standards shell-init):
//   - a missing shell arg MUST exit non-zero (usage convention: 2) with usage
//     on stderr and stdout EMPTY — `shll shell-init` drops tools only on
//     non-zero exit, so anything on stdout would be eval'd verbatim into
//     every shell startup
//   - the emitted script MUST be eval-safe: evaling it in a subshell exits 0
//
// All contracts run the real CLI as a subprocess (pattern per
// cli-exit-codes.test.ts). The eval tests pipe the CLI's actual stdout to
// `<shell> -c 'eval "$(cat)"'`, matching the standard's install idiom
// `eval "$(tu shell-init <shell>)"` — using the emitted output (not the
// static completion constants) pins the full contract, including exit 0 and
// stdout carrying only the script. The zsh test is skip-guarded on PATH
// availability — CI runners may lack zsh.
// ---------------------------------------------------------------------------

const __dirname = dirname(fileURLToPath(import.meta.url));
const CLI = join(__dirname, "..", "cli.ts");

function runCli(args: string[]): { status: number | null; stderr: string; stdout: string } {
  const r = spawnSync("npx", ["tsx", CLI, ...args], { encoding: "utf-8" });
  return { status: r.status, stderr: r.stderr, stdout: r.stdout };
}

function evalInSubshell(shell: string, script: string): { status: number | null; stderr: string } {
  const r = spawnSync(shell, ["-c", 'eval "$(cat)"'], { input: script, encoding: "utf-8" });
  return { status: r.status, stderr: r.stderr };
}

function shellAvailable(shell: string): boolean {
  return spawnSync(shell, ["--version"], { encoding: "utf-8" }).status === 0;
}

describe("shell-init: missing shell arg (subprocess contract)", () => {
  it("exits 2 with usage on stderr and stdout empty", () => {
    const r = runCli(["shell-init"]);
    assert.equal(r.status, 2);
    assert.equal(r.stdout, "", `stdout must be empty (it may be eval'd); got: ${JSON.stringify(r.stdout)}`);
    assert.ok(r.stderr.includes("Usage: tu shell-init <bash|zsh|fish>"), `stderr: ${r.stderr}`);
  });
});

describe("shell-init: emitted script is eval-safe in a subshell", () => {
  it("bash: eval of the emitted script exits 0", () => {
    const emitted = runCli(["shell-init", "bash"]);
    assert.equal(emitted.status, 0, `shell-init bash failed; stderr: ${emitted.stderr}`);
    const r = evalInSubshell("bash", emitted.stdout);
    assert.equal(r.status, 0, `bash eval failed; stderr: ${r.stderr}`);
  });

  it("zsh: eval of the emitted script exits 0 (skipped when zsh is not on PATH)", (t) => {
    if (!shellAvailable("zsh")) {
      t.skip("zsh not available on PATH — eval guard runs only where zsh exists");
      return;
    }
    const emitted = runCli(["shell-init", "zsh"]);
    assert.equal(emitted.status, 0, `shell-init zsh failed; stderr: ${emitted.stderr}`);
    const r = evalInSubshell("zsh", emitted.stdout);
    assert.equal(r.status, 0, `zsh eval failed; stderr: ${r.stderr}`);
  });
});
