// src/node/core/help-dump.ts — bespoke help-dump contract builder for shll.ai.
//
// tu's CLI help is rendered by the shll.ai landing site as a "Command
// reference". tu is the odd one out in the 7-tool rollout: a Node/TS CLI whose
// help is a single static string with no subcommand tree to walk, so the
// emitted document is structurally valid but flat (one root Node, `commands:
// []`). The other six tools are Go/Cobra binaries whose `help-dump` recurses a
// command tree; tu cannot reuse that producer.
//
// This module holds ONLY the pure, side-effect-free contract assembly so it can
// be (a) bundled into the `tu` binary — `tu help-dump` is what shll.ai's pull
// cron actually invokes against the brew-installed binary — and (b) unit-tested
// directly. All I/O (capturing help text, writing the file, validation) lives at
// the call sites.
//
// Contract (frozen, schema_version 1):
//   { tool, version, captured_at, schema_version: 1, root: Node }
//   Node = { name, path, short, usage, text, commands }

export const SCHEMA_VERSION = 1;
export const TOOL = "tu";

export interface HelpNode {
  name: string;
  path: string;
  short: string;
  usage: string;
  /** RAW, byte-for-byte `--help` text — no trim, re-wrap, CRLF, or ANSI. */
  text: string;
  /** tu prints no per-subcommand help pages — always flat. */
  commands: HelpNode[];
}

export interface HelpDoc {
  tool: string;
  version: string;
  captured_at: string;
  schema_version: typeof SCHEMA_VERSION;
  root: HelpNode;
}

export interface BuildHelpDocInput {
  name: string | undefined;
  version: string;
  description: string | undefined;
  helpText: string;
}

/**
 * Extract the `Usage:` line from the help text (the first line of FULL_HELP).
 * Falls back to the first non-empty line if no `Usage:` line is found.
 */
export function extractUsage(helpText: string): string {
  const lines = helpText.split("\n");
  const usage = lines.find((l) => l.startsWith("Usage:"));
  if (usage) return usage;
  return lines.find((l) => l.trim().length > 0) ?? "";
}

/**
 * Pure contract builder — no I/O. Given the package metadata and the raw
 * `--help` text, assemble the frozen contract object. Kept side-effect-free so
 * it can be unit-tested directly with a captured help string and reused by both
 * the in-binary `tu help-dump` command and the legacy `npm run help-dump`
 * wrapper.
 */
export function buildHelpDoc({ name, version, description, helpText }: BuildHelpDocInput): HelpDoc {
  const firstNonEmpty = helpText.split("\n").find((l) => l.trim().length > 0) ?? "";
  // Treat an empty/whitespace-only description the same as a missing one. Both
  // the build-time --define and the dev package.json fallback default to ""
  // when description is absent, and `"" ?? x` keeps the empty string — so guard
  // on content, not just null/undefined, to avoid emitting `short: ""`.
  const short = description && description.trim().length > 0 ? description : firstNonEmpty;
  return {
    tool: name ?? TOOL,
    version,
    captured_at: new Date().toISOString(),
    schema_version: SCHEMA_VERSION,
    root: {
      name: TOOL,
      path: TOOL,
      short,
      usage: extractUsage(helpText),
      // RAW, byte-for-byte: no trimming, re-wrapping, or CRLF conversion.
      text: helpText,
      // tu prints no per-subcommand help pages — flat document, no recursion.
      commands: [],
    },
  };
}
