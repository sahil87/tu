import { readFileSync } from "node:fs";
import { homedir, hostname, userInfo } from "node:os";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";

export const CURRENT_CONFIG_VERSION = 2;

export interface TuConfig {
  version: number;
  mode: "single" | "multi";
  metricsRepo: string;
  metricsDir: string;
  machine: string;
  user: string;
  autoSync: boolean;
}

export const TU_HOME = resolve(homedir(), ".tu");
export const THREE_HOURS_MS = 3 * 60 * 60 * 1000;
export const CONFIG_PATH = resolve(homedir(), ".tu.conf");

const __dirname = dirname(fileURLToPath(import.meta.url));

// Walk up to project root (where package.json lives) — works from both
// src/node/core/ (dev/test) and dist/ (bundled)
let _rootDir = __dirname;
while (_rootDir !== dirname(_rootDir)) {
  try { readFileSync(resolve(_rootDir, "package.json")); break; } catch { _rootDir = dirname(_rootDir); }
}

export const DEFAULT_CONFIG_PATH = resolve(
  _rootDir,
  process.env.WEAVER_DEV ? "tu.default.weaver.conf" : "tu.default.conf",
);

function parseConf(raw: string): Record<string, string> {
  const fields: Record<string, string> = {};
  for (const line of raw.split("\n")) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith("#")) continue;
    const idx = trimmed.indexOf("=");
    if (idx < 0) continue;
    fields[trimmed.slice(0, idx).trim()] = trimmed.slice(idx + 1).trim();
  }
  return fields;
}

function safeUsername(): string {
  try {
    return userInfo().username;
  } catch {
    return "unknown";
  }
}

function expandSentinels(value: string): string {
  if (value === "$HOSTNAME") return hostname();
  if (value === "$USER") return safeUsername();
  return value;
}

export function resolveHome(p: string): string {
  if (p.startsWith("~/")) return resolve(homedir(), p.slice(2));
  if (p === "~") return homedir();
  return p;
}

function readConfFile(path: string): Record<string, string> | null {
  try {
    return parseConf(readFileSync(path, "utf-8"));
  } catch {
    return null;
  }
}

export function readConfig(
  path: string = CONFIG_PATH,
  defaultsPath: string = DEFAULT_CONFIG_PATH,
): TuConfig {
  const defaults = readConfFile(defaultsPath) ?? {};
  const user = readConfFile(path) ?? {};

  // User fields override defaults; expand sentinels on the merged result
  const merged: Record<string, string> = { ...defaults, ...user };

  const mode = (merged.mode === "multi" ? "multi" : "single") as TuConfig["mode"];

  // Warn if multi mode but no metrics_repo
  if (mode === "multi" && !merged.metrics_repo) {
    console.error("Warning: ~/.tu.conf has mode=multi but no metrics_repo — falling back to single mode");
    merged.mode = "single";
  }

  const versionRaw = merged.version ? parseInt(merged.version, 10) : 1;
  const version = Number.isNaN(versionRaw) ? 1 : versionRaw;
  if (version > CURRENT_CONFIG_VERSION) {
    console.error(
      `Warning: ~/.tu.conf version ${version} is newer than tu supports (${CURRENT_CONFIG_VERSION}). Please update tu.`,
    );
  }

  const autoSyncRaw = merged.auto_sync;
  const autoSync = autoSyncRaw === "false" || autoSyncRaw === "0" ? false : true;

  return {
    version,
    mode: merged.mode === "multi" ? "multi" : "single",
    metricsRepo: merged.metrics_repo || "",
    metricsDir: resolveHome(expandSentinels(merged.metrics_dir || "~/.tu/metrics_repo")),
    machine: expandSentinels(merged.machine || "$HOSTNAME"),
    user: expandSentinels(merged.user || "$USER"),
    autoSync,
  };
}
