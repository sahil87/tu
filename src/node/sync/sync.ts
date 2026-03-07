import { exec } from "node:child_process";
import { mkdirSync, writeFileSync, readFileSync, readdirSync, existsSync } from "node:fs";
import { join } from "node:path";
import type { UsageEntry } from "../core/types.js";
import { TU_HOME, THREE_HOURS_MS } from "../core/config.js";
import type { TuConfig } from "../core/config.js";
import { TOOLS, fetchHistory } from "../core/fetcher.js";

function execAsync(cmd: string): Promise<string> {
  return new Promise((resolve, reject) => {
    exec(cmd, { encoding: "utf-8", maxBuffer: 10 * 1024 * 1024 }, (error, stdout) => {
      if (error) {
        reject(new Error(`${cmd.split(" ").slice(0, 3).join(" ")}... failed: ${error.message}`));
      } else {
        resolve(stdout);
      }
    });
  });
}

export function writeMetrics(
  metricsDir: string,
  user: string,
  machine: string,
  toolKey: string,
  entries: UsageEntry[],
): void {
  for (const entry of entries) {
    const yyyy = entry.label.slice(0, 4);
    const dir = join(metricsDir, user, yyyy, machine);
    mkdirSync(dir, { recursive: true });
    const filePath = join(dir, `${toolKey}-${entry.label}.jsonl`);
    writeFileSync(filePath, JSON.stringify(entry) + "\n");
  }
}

export async function syncMetrics(metricsDir: string, user: string): Promise<boolean> {
  const git = (args: string) => execAsync(`git -C "${metricsDir}" ${args}`);
  try {
    await git(`add "${user}/"`);
    const status = await git(`status --porcelain "${user}/"`);
    if (status.trim()) {
      const date = new Date().toISOString().slice(0, 10);
      await git(`commit -m "${user}: update ${date}"`);
    }
  } catch {
    return false;
  }
  try {
    await git("pull --rebase");
  } catch (err) {
    const reason = err instanceof Error ? err.message : String(err);
    console.error(`Warning: sync pull failed — ${reason}`);
    return false;
  }
  try {
    await git("push");
  } catch {
    try {
      await git("push");
    } catch (err) {
      const reason = err instanceof Error ? err.message : String(err);
      console.error(`Warning: sync push failed after retry — ${reason}`);
      return false;
    }
  }
  return true;
}

export function readRemoteEntries(
  metricsDir: string,
  targetUser: string,
  excludeMachine: string | null,
  toolKey: string,
): UsageEntry[] {
  const userPath = join(metricsDir, targetUser);
  if (!existsSync(userPath)) return [];

  const entries: UsageEntry[] = [];
  const prefix = `${toolKey}-`;

  let yearDirs: string[];
  try {
    yearDirs = readdirSync(userPath, { withFileTypes: true })
      .filter((d) => d.isDirectory())
      .map((d) => d.name);
  } catch {
    return [];
  }

  for (const yearDir of yearDirs) {
    let machineDirs: string[];
    try {
      machineDirs = readdirSync(join(userPath, yearDir), { withFileTypes: true })
        .filter((d) => d.isDirectory())
        .map((d) => d.name);
    } catch {
      continue;
    }

    for (const machineDir of machineDirs) {
      if (excludeMachine !== null && machineDir === excludeMachine) continue;

      const machPath = join(userPath, yearDir, machineDir);
      let files: string[];
      try {
        files = readdirSync(machPath)
          .filter((f) => f.startsWith(prefix) && f.endsWith(".jsonl"));
      } catch {
        continue;
      }

      for (const file of files) {
        try {
          const raw = readFileSync(join(machPath, file), "utf-8").trim();
          if (raw) {
            entries.push(JSON.parse(raw) as UsageEntry);
          }
        } catch {
          // Invalid file — skip silently
        }
      }
    }
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

export async function fullSync(config: TuConfig, tuHome: string = TU_HOME): Promise<boolean> {
  const toolKeys = Object.keys(TOOLS);
  const allLocal = await Promise.all(toolKeys.map((k) => fetchHistory(k, "daily", [])));
  for (let i = 0; i < toolKeys.length; i++) {
    writeMetrics(config.metricsDir, config.user, config.machine, toolKeys[i], allLocal[i]);
  }
  const ok = await syncMetrics(config.metricsDir, config.user);
  if (ok) touchLastSync(tuHome);
  return ok;
}

