// scripts/help-dump.mjs — local convenience wrapper for `npm run help-dump`.
//
// The frozen shll.ai contract now lives IN the `tu` binary: `tu help-dump`
// prints the contract JSON to stdout (see src/node/core/help-dump.ts +
// runHelpDump in cli.ts). That is what shll.ai's pull cron invokes against the
// brew-installed binary. This wrapper exists only so a developer can refresh a
// local `help/tu.json` from a freshly-built bundle without remembering the
// redirect — it captures the binary's stdout and writes the file, then
// self-validates. There is NO contract logic here to drift.
//
// Constraints (constitution): `node:`-prefixed built-ins only, no new runtime
// dependency, functional style. Runs under plain `node`.
//
// Usage: npm run build && npm run help-dump
// Requires a built `dist/tu.mjs` (run `npm run build` first).

import { readFileSync, writeFileSync, mkdirSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const SCHEMA_VERSION = 1;

const __dirname = dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = dirname(__dirname);
const BUILT_CLI = join(REPO_ROOT, "dist", "tu.mjs");
const OUTPUT_PATH = join(REPO_ROOT, "help", "tu.json");

/**
 * Run `node dist/tu.mjs help-dump` and capture its stdout (the contract JSON).
 * Throws on a non-zero exit, empty output, or output that has leaked to stderr
 * (the binary contract is stdout-only + empty stderr).
 */
function captureHelpDump() {
  // spawnSync (not execFileSync) so stdout and stderr are returned separately —
  // execFileSync only yields stdout, which would make the empty-stderr
  // assertion below dead code.
  const result = spawnSync(process.execPath, [BUILT_CLI, "help-dump"], {
    encoding: "utf-8",
    maxBuffer: 10 * 1024 * 1024,
  });

  if (result.error) {
    throw new Error(`failed to run \`node ${BUILT_CLI} help-dump\`: ${result.error.message}`);
  }
  if (result.status !== 0) {
    throw new Error(`\`node ${BUILT_CLI} help-dump\` exited ${result.status}: ${result.stderr ?? ""}`);
  }
  if (!result.stdout || result.stdout.length === 0) {
    throw new Error(`\`node ${BUILT_CLI} help-dump\` produced empty output`);
  }
  if (result.stderr && result.stderr.length > 0) {
    throw new Error(`\`node ${BUILT_CLI} help-dump\` wrote to stderr (must be empty): ${result.stderr}`);
  }
  return result.stdout;
}

/**
 * Re-read the written artifact, parse it, and assert the required keys are
 * present and well-typed. Throws on any failure so the caller can fail loud.
 */
function validateArtifact(path) {
  const doc = JSON.parse(readFileSync(path, "utf-8"));
  const isNonEmptyString = (v) => typeof v === "string" && v.length > 0;

  const fail = (msg) => {
    throw new Error(`help/tu.json validation failed: ${msg}`);
  };

  if (!isNonEmptyString(doc.tool)) fail("`tool` must be a non-empty string");
  if (!isNonEmptyString(doc.version)) fail("`version` must be a non-empty string");
  if (!isNonEmptyString(doc.captured_at)) fail("`captured_at` must be a non-empty string");
  if (doc.schema_version !== SCHEMA_VERSION) fail(`\`schema_version\` must be ${SCHEMA_VERSION}`);
  if (typeof doc.root !== "object" || doc.root === null) fail("`root` must be an object");

  const { root } = doc;
  if (!isNonEmptyString(root.name)) fail("`root.name` must be a non-empty string");
  if (!isNonEmptyString(root.path)) fail("`root.path` must be a non-empty string");
  if (typeof root.short !== "string") fail("`root.short` must be a string");
  if (typeof root.usage !== "string") fail("`root.usage` must be a string");
  if (!isNonEmptyString(root.text)) fail("`root.text` must be a non-empty string");
  if (!Array.isArray(root.commands)) fail("`root.commands` must be an array");

  return doc;
}

function main() {
  const json = captureHelpDump();

  // The binary already pretty-prints with a trailing newline; write it as-is so
  // the local artifact is byte-identical to the binary's stdout.
  mkdirSync(dirname(OUTPUT_PATH), { recursive: true });
  writeFileSync(OUTPUT_PATH, json, "utf-8");

  // Re-read and self-validate — never emit a malformed artifact.
  const doc = validateArtifact(OUTPUT_PATH);

  process.stderr.write(`help-dump: wrote ${OUTPUT_PATH} (tu v${doc.version})\n`);
}

try {
  main();
} catch (err) {
  process.stderr.write(`help-dump: ${err instanceof Error ? err.message : String(err)}\n`);
  process.exit(1);
}
