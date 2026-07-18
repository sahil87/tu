import { execFile } from "node:child_process";
import { mkdirSync, writeFileSync, readFileSync, readdirSync, existsSync } from "node:fs";
import { join } from "node:path";
import type { UsageEntry } from "../core/types.js";
import { TU_HOME, THREE_HOURS_MS } from "../core/config.js";
import type { TuConfig } from "../core/config.js";
import { TOOLS, fetchHistory } from "../core/fetcher.js";

function execFileAsync(file: string, args: string[]): Promise<string> {
  return new Promise((resolve, reject) => {
    execFile(file, args, { encoding: "utf-8", maxBuffer: 10 * 1024 * 1024 }, (error, stdout) => {
      if (error) {
        const summary = [file, ...args.slice(0, 2)].join(" ");
        reject(new Error(`${summary}... failed: ${error.message}`));
      } else {
        resolve(stdout);
      }
    });
  });
}

// The commit message for a metrics-repo commit: `# {user}: update {date}`
// with today's UTC date. Derived in ONE place so the live commit (syncMetrics)
// and the dry-run preview (fullSync) can never drift (toolkit principle №5).
function commitMessage(user: string): string {
  const date = new Date().toISOString().slice(0, 10);
  return `# ${user}: update ${date}`;
}

// A per-file decision produced by writeMetrics. Returned in both live and
// dry-run mode so a dry-run preview shares the exact decision path as the live
// write (toolkit principle №5: an accurate preview must not drift from the live
// path). In live mode the return value is ignored by existing callers.
export interface WriteDecision {
  filePath: string; // day-file path, `join(metricsDir, ...)` — absolute when metricsDir is (the default `~/…` resolves absolute), relative when a relative metrics_dir is configured
  action: "write" | "skip"; // skip = never-shrink guard would skip this write
  incomingCost: number;
  existingCost?: number; // present only when an existing parseable file was read
}

// Never-shrink guard: day-file snapshots are high-water marks of complete
// data. Claude Code purges transcripts older than ~30 days, so a live fetch
// for an old date collapses toward zero — overwriting would silently destroy
// correct history. Skip the write (whole-entry, keeping the file an atomic
// snapshot) when the incoming entry's totalCost is lower than the existing
// one's. Absent/empty/unparseable files are treated as absent, matching the
// read path's skip-silently posture; equal values still write so today's
// file keeps refreshing as the day grows.
//
// Returns both the shrink verdict and the parsed existing cost (when a valid
// existing file was read) in one pass, so writeMetrics can populate a
// WriteDecision without a second file read.
function readShrinkState(
  filePath: string,
  incoming: UsageEntry,
): { shrinking: boolean; existingCost?: number } {
  let raw: string;
  try {
    raw = readFileSync(filePath, "utf-8").trim();
  } catch {
    return { shrinking: false }; // file absent or unreadable → write
  }
  if (!raw) return { shrinking: false }; // empty file → treat as absent
  try {
    const existing = JSON.parse(raw) as Partial<UsageEntry>;
    const existingCost = Number(existing?.totalCost);
    if (!Number.isFinite(existingCost)) return { shrinking: false }; // not a UsageEntry → treat as absent
    return { shrinking: incoming.totalCost < existingCost, existingCost };
  } catch {
    return { shrinking: false }; // unparseable → treat as absent
  }
}

// Writes local entries to the metrics directory, honoring the never-shrink
// guard. Returns a per-file WriteDecision[] describing what was (or, in dry-run
// mode, WOULD be) written or skipped. When dryRun is true the decision logic
// runs identically but no directory is created and no file is written — the
// shared decision path is exactly what makes the dry-run preview accurate.
export function writeMetrics(
  metricsDir: string,
  user: string,
  machine: string,
  toolKey: string,
  entries: UsageEntry[],
  dryRun = false,
): WriteDecision[] {
  const decisions: WriteDecision[] = [];
  for (const entry of entries) {
    const yyyy = entry.label.slice(0, 4);
    const dir = join(metricsDir, user, yyyy, machine);
    const filePath = join(dir, `${toolKey}-${entry.label}.jsonl`);
    const { shrinking, existingCost } = readShrinkState(filePath, entry);
    const decision: WriteDecision = {
      filePath,
      action: shrinking ? "skip" : "write",
      incomingCost: entry.totalCost,
    };
    if (existingCost !== undefined) decision.existingCost = existingCost;
    decisions.push(decision);
    if (shrinking) continue;
    if (!dryRun) {
      mkdirSync(dir, { recursive: true });
      writeFileSync(filePath, JSON.stringify(entry) + "\n");
    }
  }
  return decisions;
}

export async function syncMetrics(metricsDir: string, user: string): Promise<boolean> {
  const git = (args: string[]) => execFileAsync("git", ["-C", metricsDir, ...args]);

  // Recover from interrupted rebase left by a previous failed sync
  try {
    const rebaseMerge = join(metricsDir, ".git", "rebase-merge");
    const rebaseApply = join(metricsDir, ".git", "rebase-apply");
    if (existsSync(rebaseMerge) || existsSync(rebaseApply)) {
      console.error("Warning: recovering from interrupted rebase");
      await git(["rebase", "--abort"]);
    }
  } catch {
    // rebase --abort failed — try to continue anyway
  }

  try {
    // The user dir may not exist yet (first run, or no data because all
    // ccusage sources were unavailable). `git add <user>/` would fail with
    // "pathspec did not match any files" — skip staging and let the pull/push
    // below run as a clean no-op so sync still succeeds.
    if (existsSync(join(metricsDir, user))) {
      await git(["add", `${user}/`]);
    }
    const status = await git(["status", "--porcelain", `${user}/`]);
    if (status.trim()) {
      await git(["commit", "-m", commitMessage(user)]);
    }
  } catch {
    return false;
  }
  try {
    await git(["pull", "--rebase", "origin", "main"]);
  } catch (err) {
    const reason = err instanceof Error ? err.message : String(err);
    console.error(`Warning: sync pull failed — ${reason}`);
    try { await git(["rebase", "--abort"]); } catch { /* already clean */ }
    return false;
  }
  try {
    await git(["push"]);
  } catch {
    try {
      await git(["push"]);
    } catch (err) {
      const reason = err instanceof Error ? err.message : String(err);
      console.error(`Warning: sync push failed after retry — ${reason}`);
      return false;
    }
  }
  return true;
}

export function readRemoteEntriesByMachine(
  metricsDir: string,
  targetUser: string,
  excludeMachine: string | null,
  toolKey: string,
): Map<string, UsageEntry[]> {
  const userPath = join(metricsDir, targetUser);
  if (!existsSync(userPath)) return new Map();

  const result = new Map<string, UsageEntry[]>();
  const prefix = `${toolKey}-`;

  let yearDirs: string[];
  try {
    yearDirs = readdirSync(userPath, { withFileTypes: true })
      .filter((d) => d.isDirectory())
      .map((d) => d.name)
      .sort();
  } catch {
    return new Map();
  }

  for (const yearDir of yearDirs) {
    let machineDirs: string[];
    try {
      machineDirs = readdirSync(join(userPath, yearDir), { withFileTypes: true })
        .filter((d) => d.isDirectory())
        .map((d) => d.name)
        .sort();
    } catch {
      continue;
    }

    for (const machineDir of machineDirs) {
      if (excludeMachine !== null && machineDir === excludeMachine) continue;

      const machPath = join(userPath, yearDir, machineDir);
      let files: string[];
      try {
        files = readdirSync(machPath)
          .filter((f) => f.startsWith(prefix) && f.endsWith(".jsonl"))
          .sort();
      } catch {
        continue;
      }

      for (const file of files) {
        try {
          const raw = readFileSync(join(machPath, file), "utf-8").trim();
          if (raw) {
            if (!result.has(machineDir)) result.set(machineDir, []);
            result.get(machineDir)!.push(JSON.parse(raw) as UsageEntry);
          }
        } catch {
          // Invalid file — skip silently
        }
      }
    }
  }

  return result;
}

export function readRemoteEntries(
  metricsDir: string,
  targetUser: string,
  excludeMachine: string | null,
  toolKey: string,
): UsageEntry[] {
  const byMachine = readRemoteEntriesByMachine(metricsDir, targetUser, excludeMachine, toolKey);
  const entries: UsageEntry[] = [];
  for (const machineEntries of byMachine.values()) {
    entries.push(...machineEntries);
  }
  return entries;
}

export function isStale(dir: string): boolean {
  const syncFile = join(dir, ".last-sync");
  try {
    const raw = readFileSync(syncFile, "utf-8").trim();
    const ts = new Date(raw).getTime();
    if (Number.isNaN(ts)) return true;
    return Date.now() - ts > THREE_HOURS_MS;
  } catch {
    return true;
  }
}

export function touchLastSync(dir: string): void {
  writeFileSync(join(dir, ".last-sync"), new Date().toISOString() + "\n");
}

// Per-tool dry-run write decisions, keyed by tool.
export interface ToolWriteReport {
  toolKey: string;
  decisions: WriteDecision[];
}

// The structured preview a dry-run fullSync returns instead of a boolean.
// Everything here is computed WITHOUT touching the working tree, the metrics
// repo, or the network: writes come from writeMetrics' dry-run decisions, and
// the commit decision from would-be writes + a read-only `git status
// --porcelain`. pull/push are reported as the operations that WOULD follow,
// never executed or probed.
export interface DrySyncReport {
  metricsDir: string;
  user: string;
  machine: string;
  tools: ToolWriteReport[];
  wouldCommit: boolean; // true if any would-write or the user dir is already dirty
  commitMessage: string; // the same `# {user}: update {date}` string a live commit uses
}

export async function fullSync(config: TuConfig, tuHome?: string, dryRun?: false): Promise<boolean>;
export async function fullSync(config: TuConfig, tuHome: string | undefined, dryRun: true): Promise<DrySyncReport>;
export async function fullSync(
  config: TuConfig,
  tuHome: string = TU_HOME,
  dryRun = false,
): Promise<boolean | DrySyncReport> {
  const toolKeys = Object.keys(TOOLS);
  const allLocal = await Promise.all(toolKeys.map((k) => fetchHistory(k, "daily", [])));

  if (dryRun) {
    const tools: ToolWriteReport[] = [];
    let anyWouldWrite = false;
    for (let i = 0; i < toolKeys.length; i++) {
      const decisions = writeMetrics(config.metricsDir, config.user, config.machine, toolKeys[i], allLocal[i], true);
      if (decisions.some((d) => d.action === "write")) anyWouldWrite = true;
      tools.push({ toolKey: toolKeys[i], decisions });
    }
    // Read-only check of already-staged/dirty files under the user dir, so an
    // existing dirty working tree is reflected in the would-commit decision.
    // Mirrors live syncMetrics, which runs `git status --porcelain <user>/`
    // UNCONDITIONALLY (only its `git add` is gated on the dir existing) — so the
    // preview must run the status even when the dir is absent on disk, or a
    // tracked-but-deleted user dir (dir gone, but `git status` still reports the
    // deletions) would under-report the commit. No network, no mutation. A
    // non-git repo or any git failure is treated as "no dirty files" — the
    // dry-run must never crash on an un-synced setup.
    let dirty = false;
    try {
      const status = await execFileAsync("git", ["-C", config.metricsDir, "status", "--porcelain", `${config.user}/`]);
      dirty = status.trim().length > 0;
    } catch {
      dirty = false;
    }
    return {
      metricsDir: config.metricsDir,
      user: config.user,
      machine: config.machine,
      tools,
      // Plan-sanctioned heuristic (Design Decision 3): in steady state this can
      // over-predict. An equal-cost incoming entry writes a byte-identical
      // day-file, so a would-write here can still leave the live tree clean —
      // live syncMetrics' `git status` then finds nothing and skips the commit
      // while the preview said "Would commit". The preview errs toward showing
      // the commit; it never under-reports one that would happen.
      wouldCommit: anyWouldWrite || dirty,
      commitMessage: commitMessage(config.user),
    };
  }

  for (let i = 0; i < toolKeys.length; i++) {
    writeMetrics(config.metricsDir, config.user, config.machine, toolKeys[i], allLocal[i]);
  }
  const ok = await syncMetrics(config.metricsDir, config.user);
  if (ok) touchLastSync(tuHome);
  return ok;
}

