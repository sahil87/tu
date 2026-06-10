// scripts/repair-metrics.mjs — one-time repair: restore shrunk metrics
// day-files to their historical maximum from the metrics repo's git history.
//
// Why: Claude Code purges session transcripts older than ~30 days, so a
// machine's live ccusage view of an old day collapses toward zero. Until the
// never-shrink guard in writeMetrics() (src/node/sync/sync.ts) shipped, every
// sync overwrote correct per-day JSONL snapshots in the shared metrics repo
// with that post-purge residue. Nothing was ever deleted from git history —
// only overwritten in newer commits — so every shrunk day-file can be
// restored losslessly by picking the commit where its totalCost was highest.
//
// Standalone ops script (precedent: scripts/help-dump.mjs): NOT bundled into
// dist/tu.mjs and not imported by anything under src/ (Constitution III
// untouched). Runs under plain `node` with `node:`-prefixed built-ins only.
//
// IMPORTANT sequencing: run this only AFTER the guarded binary is installed
// on every actively-syncing machine — otherwise the next sync from an old
// binary re-clobbers restored values at the rolling retention edge.
//
// Usage:
//   node scripts/repair-metrics.mjs [--repo <path>] [--write]
//
//   --repo <path>  Metrics repo to scan (default: ~/.tu/metrics_repo)
//   --write        Restore shrunk files in the working tree. Default is a
//                  dry-run report. Committing/pushing is left to the user
//                  for review. Idempotent — re-running reports nothing left.

import { readFileSync, writeFileSync, existsSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { join, resolve } from "node:path";
import { homedir } from "node:os";

// A file is "shrunk" only when HEAD is below its historical max by more than
// a cent — float noise within a cent is not worth touching.
const CENT_TOLERANCE = 0.01;

// Day-file names follow {toolKey}-{YYYY-MM-DD}.jsonl (see writeMetrics).
const DAY_FILE_RE = /-\d{4}-\d{2}-\d{2}\.jsonl$/;

// git log over a long history can be large; well above any realistic size.
const MAX_GIT_BUFFER = 64 * 1024 * 1024;

const USAGE = "Usage: node scripts/repair-metrics.mjs [--repo <path>] [--write]";

function fail(msg) {
  process.stderr.write(`repair-metrics: ${msg}\n`);
  process.exit(1);
}

function resolveHome(p) {
  if (p === "~") return homedir();
  if (p.startsWith("~/")) return resolve(homedir(), p.slice(2));
  return resolve(p);
}

function parseArgs(argv) {
  let repo = resolveHome("~/.tu/metrics_repo");
  let write = false;
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    if (arg === "--repo") {
      const value = argv[++i];
      if (!value) fail(`--repo requires a path\n${USAGE}`);
      repo = resolveHome(value);
    } else if (arg === "--write") {
      write = true;
    } else {
      fail(`unknown argument: ${arg}\n${USAGE}`);
    }
  }
  return { repo, write };
}

/** Run git against the repo; returns stdout or null on failure. */
function git(repo, args) {
  // core.quotePath=false → raw UTF-8 paths (no C-style quoting to unescape).
  const result = spawnSync("git", ["-C", repo, "-c", "core.quotePath=false", ...args], {
    encoding: "utf-8",
    maxBuffer: MAX_GIT_BUFFER,
  });
  if (result.error || result.status !== 0) return null;
  return result.stdout;
}

/** Parse the first line of a day-file blob; returns a finite totalCost or null. */
function parseCost(content) {
  if (typeof content !== "string" || !content.trim()) return null;
  try {
    const parsed = JSON.parse(content.trim().split("\n")[0]);
    const cost = Number(parsed?.totalCost);
    return Number.isFinite(cost) ? cost : null;
  } catch {
    return null;
  }
}

/** Tracked day-files at HEAD (paths relative to the repo root). */
function listTrackedDayFiles(repo) {
  const out = git(repo, ["ls-files", "-z", "--", "*.jsonl"]);
  if (out === null) fail("git ls-files failed — is this a metrics repo checkout?");
  return out.split("\0").filter((p) => p && DAY_FILE_RE.test(p));
}

/**
 * One history walk (avoids a `git log` per file): every commit on the
 * checked-out branch that touches a *.jsonl file, with the paths it touched.
 * Returns { commitCount, fileCommits: Map<path, Array<{sha, date}>> } —
 * commit lists are newest-first (git log order).
 */
function buildFileCommitMap(repo) {
  const out = git(repo, ["log", "--format=%H%x09%cs", "--name-only", "--", "*.jsonl"]);
  if (out === null) fail("git log failed — is this a metrics repo checkout?");
  const fileCommits = new Map();
  let current = null;
  let commitCount = 0;
  for (const line of out.split("\n")) {
    const commitMatch = line.match(/^([0-9a-f]{40})\t(\d{4}-\d{2}-\d{2})$/);
    if (commitMatch) {
      current = { sha: commitMatch[1], date: commitMatch[2] };
      commitCount++;
      continue;
    }
    if (!line || !current || !DAY_FILE_RE.test(line)) continue;
    if (!fileCommits.has(line)) fileCommits.set(line, []);
    fileCommits.get(line).push(current);
  }
  return { commitCount, fileCommits };
}

/**
 * Historical maximum for one file: highest parseable totalCost across every
 * commit that touched it. Deleted-at-commit paths and unparseable blobs are
 * skipped. Returns { cost, sha, date, content } or null when no version parses.
 */
function findHistoricalMax(repo, path, commits) {
  let max = null;
  for (const { sha, date } of commits) {
    const content = git(repo, ["show", `${sha}:${path}`]);
    if (content === null) continue; // path deleted in this commit — skip
    const cost = parseCost(content);
    if (cost === null) continue; // unparseable historical version — skip
    if (max === null || cost > max.cost) max = { cost, sha, date, content };
  }
  return max;
}

/** Working-tree totalCost; missing/unparseable counts as 0 (fully shrunk). */
function currentCost(repo, path) {
  try {
    return parseCost(readFileSync(join(repo, path), "utf-8")) ?? 0;
  } catch {
    return 0;
  }
}

const money = (v) => `$${v.toFixed(2)}`;

function printReport(repo, shrunk, dayFileCount, commitCount) {
  const out = [];
  out.push(`repair-metrics: scanned ${repo}`);
  out.push(`  ${dayFileCount} tracked day-files, ${commitCount} commits touching *.jsonl`);
  out.push("");

  if (shrunk.length === 0) {
    out.push("Nothing to repair — every day-file is at its historical maximum.");
    process.stdout.write(out.join("\n") + "\n");
    return;
  }

  const pathWidth = Math.max(...shrunk.map((s) => s.path.length), "FILE".length);
  out.push(`Shrunk day-files (${shrunk.length}):`);
  out.push("");
  out.push(
    `  ${"FILE".padEnd(pathWidth)}  ${"CURRENT".padStart(10)}  ${"MAX".padStart(10)}  ${"DELTA".padStart(10)}  MAX COMMIT`,
  );
  for (const s of shrunk) {
    out.push(
      `  ${s.path.padEnd(pathWidth)}  ${money(s.current).padStart(10)}  ${money(s.max.cost).padStart(10)}  ${("+" + money(s.delta)).padStart(10)}  ${s.max.sha.slice(0, 7)} (${s.max.date})`,
    );
  }

  // Per-user totals — the user is the first path segment.
  const byUser = new Map();
  for (const s of shrunk) {
    const user = s.path.split("/")[0];
    const agg = byUser.get(user) ?? { delta: 0, files: 0 };
    agg.delta += s.delta;
    agg.files += 1;
    byUser.set(user, agg);
  }
  out.push("");
  out.push("Per-user totals:");
  for (const [user, agg] of [...byUser.entries()].sort()) {
    out.push(`  ${user}: +${money(agg.delta)} across ${agg.files} file(s)`);
  }

  const grand = shrunk.reduce((sum, s) => sum + s.delta, 0);
  out.push("");
  out.push(`Grand total: +${money(grand)} across ${shrunk.length} file(s)`);
  process.stdout.write(out.join("\n") + "\n");
}

function main() {
  const { repo, write } = parseArgs(process.argv.slice(2));

  if (!existsSync(repo)) fail(`repo not found: ${repo}`);
  if (git(repo, ["rev-parse", "--is-inside-work-tree"]) === null) {
    fail(`not a git repository: ${repo}`);
  }

  const dayFiles = listTrackedDayFiles(repo);
  const { commitCount, fileCommits } = buildFileCommitMap(repo);

  const shrunk = [];
  for (const path of dayFiles) {
    const max = findHistoricalMax(repo, path, fileCommits.get(path) ?? []);
    if (max === null) continue; // no parseable history — nothing to compare
    const current = currentCost(repo, path);
    const delta = max.cost - current;
    if (delta > CENT_TOLERANCE) shrunk.push({ path, current, max, delta });
  }
  shrunk.sort((a, b) => a.path.localeCompare(b.path));

  printReport(repo, shrunk, dayFiles.length, commitCount);
  if (shrunk.length === 0) return;

  if (!write) {
    process.stdout.write("\nDry run — nothing modified. Re-run with --write to restore shrunk files.\n");
    return;
  }

  // Restore the full original content (the exact historical-max blob), not
  // just the cost field — each day-file stays an atomic snapshot that was
  // real at some point in time. Working tree only: review, commit, and push
  // are deliberately left to the user.
  for (const s of shrunk) {
    writeFileSync(join(repo, s.path), s.max.content);
  }
  process.stdout.write(
    `\nRestored ${shrunk.length} file(s) in the working tree.\n` +
      `Review with: git -C ${repo} diff\nThen commit and push manually.\n`,
  );
}

main();
