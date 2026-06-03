// scripts/help-dump.mjs — bespoke Node help-dump producer for shll.ai.
//
// Emits tu's CLI help as the frozen `help/tu.json` contract, which the shll.ai
// landing site renders as a "Command reference". tu is the odd one out in the
// 7-tool rollout: a Node/TS CLI whose help is a single static string with no
// subcommand tree to walk, so the emitted document is structurally valid but
// flat (one root Node, `commands: []`).
//
// Contract (frozen): { tool, version, captured_at, schema_version: 1, root: Node }
//   Node = { name, path, short, usage, text, commands }
//
// Constraints (constitution): `node:`-prefixed built-ins only, no new runtime
// dependency, functional style (no classes). Run under plain `node` in CI —
// the production path needs no tsx.
//
// Usage: node scripts/help-dump.mjs   (also `npm run help-dump`)
// Requires a built `dist/tu.mjs` (run `npm run build` first).

import { readFileSync, writeFileSync, mkdirSync } from "node:fs";
import { execFileSync } from "node:child_process";
import { dirname, join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const SCHEMA_VERSION = 1;
const TOOL = "tu";

const __dirname = dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = dirname(__dirname);
const BUILT_CLI = join(REPO_ROOT, "dist", "tu.mjs");
const OUTPUT_PATH = join(REPO_ROOT, "help", "tu.json");

/**
 * Extract the `Usage:` line from the help text (the first line of FULL_HELP).
 * Falls back to the first non-empty line if no `Usage:` line is found.
 */
function extractUsage(helpText) {
  const lines = helpText.split("\n");
  const usage = lines.find((l) => l.startsWith("Usage:"));
  if (usage) return usage;
  return lines.find((l) => l.trim().length > 0) ?? "";
}

/**
 * Pure contract builder — no I/O. Given the package metadata and the raw
 * `--help` text, assemble the frozen contract object. Kept side-effect-free so
 * it can be unit-tested directly with a captured help string.
 */
export function buildHelpDoc({ name, version, description, helpText }) {
  const firstNonEmpty = helpText.split("\n").find((l) => l.trim().length > 0) ?? "";
  return {
    tool: name ?? TOOL,
    version,
    captured_at: new Date().toISOString(),
    schema_version: SCHEMA_VERSION,
    root: {
      name: TOOL,
      path: TOOL,
      short: description ?? firstNonEmpty,
      usage: extractUsage(helpText),
      // RAW, byte-for-byte: no trimming, re-wrapping, or CRLF conversion.
      text: helpText,
      // tu prints no per-subcommand help pages — flat document, no recursion.
      commands: [],
    },
  };
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

/**
 * Capture `node dist/tu.mjs --help` byte-for-byte with color forced off.
 * Throws on a non-zero CLI exit or empty stdout.
 */
function captureHelp() {
  let stdout;
  try {
    stdout = execFileSync(process.execPath, [BUILT_CLI, "--help"], {
      // Force plain text — guarantees the captured `text` carries no ANSI.
      env: { ...process.env, NO_COLOR: "1" },
      encoding: "utf-8",
      // Generous buffer; help output is tiny but be safe.
      maxBuffer: 10 * 1024 * 1024,
    });
  } catch (err) {
    throw new Error(`failed to run \`node ${BUILT_CLI} --help\`: ${err.message}`);
  }
  if (!stdout || stdout.length === 0) {
    throw new Error(`\`node ${BUILT_CLI} --help\` produced empty output`);
  }
  return stdout;
}

function main() {
  const pkg = JSON.parse(readFileSync(join(REPO_ROOT, "package.json"), "utf-8"));
  const helpText = captureHelp();

  const doc = buildHelpDoc({
    name: pkg.name,
    version: pkg.version,
    description: pkg.description,
    helpText,
  });

  // Pretty-printed, 2-space, trailing newline.
  mkdirSync(dirname(OUTPUT_PATH), { recursive: true });
  writeFileSync(OUTPUT_PATH, JSON.stringify(doc, null, 2) + "\n", "utf-8");

  // Re-read and self-validate — never emit a malformed artifact.
  validateArtifact(OUTPUT_PATH);

  process.stderr.write(`help-dump: wrote ${OUTPUT_PATH} (tu v${doc.version})\n`);
}

// Run main() only when invoked as the entry module, not when imported by tests.
if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  try {
    main();
  } catch (err) {
    process.stderr.write(`help-dump: ${err instanceof Error ? err.message : String(err)}\n`);
    process.exit(1);
  }
}
