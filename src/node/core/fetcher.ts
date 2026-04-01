import { exec } from "node:child_process";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { existsSync, mkdirSync, readFileSync, writeFileSync, statSync } from "node:fs";
import { homedir } from "node:os";
import type { UsageTotals, UsageEntry, ToolConfig } from "./types.js";

const __dirname = dirname(fileURLToPath(import.meta.url));

// Walk up to project root (where package.json lives) — works from both
// src/node/core/ (dev/test) and dist/ (bundled)
let _rootDir = __dirname;
while (_rootDir !== dirname(_rootDir)) {
  if (existsSync(join(_rootDir, "package.json"))) break;
  _rootDir = dirname(_rootDir);
}
const vendorDir = join(__dirname, "vendor");
const useVendor = existsSync(vendorDir);
const BIN = useVendor ? vendorDir : join(_rootDir, "node_modules", ".bin");

// --- Fetch cache: avoids re-scanning 500MB+ of JSONL files on every call ---
const CACHE_DIR = join(homedir(), ".tu", "cache");
const CACHE_TTL_MS = 60 * 1000; // 60 seconds

interface CacheEnvelope {
  ts: number;
  entries: UsageEntry[];
}

function cacheKey(toolKey: string): string {
  return join(CACHE_DIR, `${toolKey}-daily.json`);
}

function readCache(toolKey: string): UsageEntry[] | null {
  const path = cacheKey(toolKey);
  try {
    if (!existsSync(path)) return null;
    const age = Date.now() - statSync(path).mtimeMs;
    if (age > CACHE_TTL_MS) return null;
    const envelope: CacheEnvelope = JSON.parse(readFileSync(path, "utf-8"));
    return envelope.entries;
  } catch {
    return null;
  }
}

function writeCache(toolKey: string, entries: UsageEntry[]): void {
  try {
    mkdirSync(CACHE_DIR, { recursive: true });
    const envelope: CacheEnvelope = { ts: Date.now(), entries };
    writeFileSync(cacheKey(toolKey), JSON.stringify(envelope));
  } catch {
    // Non-fatal — next call will just re-fetch
  }
}

export const TOOLS: Record<string, ToolConfig> = {
  cc: { name: "Claude Code", command: useVendor ? `node ${BIN}/ccusage/index.js` : `${BIN}/ccusage`, needsFilter: false },
  codex: { name: "Codex", command: useVendor ? `node ${BIN}/ccusage-codex/index.js` : `${BIN}/ccusage-codex`, needsFilter: true },
  oc: { name: "OpenCode", command: useVendor ? `node ${BIN}/ccusage-opencode/index.js` : `${BIN}/ccusage-opencode`, needsFilter: true },
};

export const EMPTY: UsageTotals = {
  totalCost: 0,
  inputTokens: 0,
  outputTokens: 0,
  cacheCreationTokens: 0,
  cacheReadTokens: 0,
  totalTokens: 0,
};

export function stripNoise(output: string): string {
  return output
    .split("\n")
    .filter((line) => !line.startsWith("["))
    .join("\n");
}

function execAsync(cmd: string, toolName: string): Promise<string> {
  return new Promise((resolve) => {
    exec(cmd, { encoding: "utf-8", maxBuffer: 10 * 1024 * 1024 }, (error, stdout) => {
      if (error) {
        console.warn(`warning: ${toolName} fetch failed (${error.message}), showing zero data`);
        resolve("");
      } else {
        resolve(stdout);
      }
    });
  });
}

export function toUsageTotals(t: Record<string, unknown>): UsageTotals {
  return {
    totalCost: Number(t.totalCost ?? t.costUSD) || 0,
    inputTokens: Number(t.inputTokens) || 0,
    outputTokens: Number(t.outputTokens) || 0,
    cacheCreationTokens: Number(t.cacheCreationTokens) || 0,
    cacheReadTokens: Number(t.cacheReadTokens ?? t.cachedInputTokens) || 0,
    totalTokens: Number(t.totalTokens) || 0,
  };
}

const MONTHS: Record<string, string> = {
  Jan: "01", Feb: "02", Mar: "03", Apr: "04", May: "05", Jun: "06",
  Jul: "07", Aug: "08", Sep: "09", Oct: "10", Nov: "11", Dec: "12",
};

// Normalize human-readable dates ("Feb 14, 2026", "Feb 2026") to ISO ("2026-02-14", "2026-02")
export function normalizeLabel(label: string): string {
  const daily = label.match(/^(\w{3})\s+(\d{1,2}),\s+(\d{4})$/);
  if (daily) {
    const [, mon, day, year] = daily;
    return `${year}-${MONTHS[mon] || "00"}-${day.padStart(2, "0")}`;
  }
  const monthly = label.match(/^(\w{3})\s+(\d{4})$/);
  if (monthly) {
    const [, mon, year] = monthly;
    return `${year}-${MONTHS[mon] || "00"}`;
  }
  return label;
}

export function toUsageEntry(t: Record<string, unknown>, labelKey: string): UsageEntry {
  return {
    label: normalizeLabel(String(t[labelKey] || "")),
    ...toUsageTotals(t),
  };
}

// Label key varies by period: daily entries have "date", monthly entries have "month"
const LABEL_KEY: Record<string, string> = { daily: "date", monthly: "month" };

// Current ISO label for filtering entries to "now"
export function currentLabel(period: string, now: Date = new Date()): string {
  const yyyy = now.getFullYear();
  const mm = String(now.getMonth() + 1).padStart(2, "0");
  if (period === "monthly") return `${yyyy}-${mm}`;
  const dd = String(now.getDate()).padStart(2, "0");
  return `${yyyy}-${mm}-${dd}`;
}

// Pick the entry matching the current date/month, or return EMPTY
export function pickCurrentEntry(
  entries: Record<string, unknown>[],
  period: string,
  now: Date = new Date()
): UsageTotals {
  const labelKey = LABEL_KEY[period] || "date";
  const target = currentLabel(period, now);
  const match = entries.find((e) => normalizeLabel(String(e[labelKey] || "")) === target);
  return match ? toUsageTotals(match) : { ...EMPTY };
}

export function parseJson(raw: string, needsFilter: boolean): Record<string, unknown> | null {
  if (!raw.trim()) return null;
  try {
    const cleaned = needsFilter ? stripNoise(raw) : raw;
    return JSON.parse(cleaned);
  } catch {
    return null;
  }
}

async function runTool(toolKey: string, period: string, extraArgs: string[] = []): Promise<string> {
  const tool = TOOLS[toolKey];
  if (!tool) throw new Error(`Unknown tool: ${toolKey}`);
  const args = [period, "--json", ...extraArgs].join(" ");
  return execAsync(`${tool.command} ${args}`, tool.name);
}

// --- Single-value fetchers (used by tu total daily / tu total monthly) ---

export async function fetchTotals(toolKey: string, extraArgs: string[] = []): Promise<UsageTotals> {
  const tool = TOOLS[toolKey];
  if (!tool) throw new Error(`Unknown tool: ${toolKey}`);
  const raw = await runTool(toolKey, "daily", extraArgs);
  const parsed = parseJson(raw, tool.needsFilter);
  if (!parsed) return { ...EMPTY };

  const dailyRaw = parsed["daily"] as Record<string, unknown>[] | undefined;
  if (!dailyRaw || dailyRaw.length === 0) return { ...EMPTY };

  return pickCurrentEntry(dailyRaw, "daily");
}

export async function fetchAllTotals(extraArgs: string[] = []): Promise<Map<string, UsageTotals>> {
  const entries = Object.entries(TOOLS);
  const results = await Promise.all(
    entries.map(async ([key, tool]) => {
      const totals = await fetchTotals(key, extraArgs);
      return [tool.name, totals] as const;
    })
  );
  return new Map(results);
}

// --- History fetchers (used by single-tool commands and total-history) ---

export async function fetchHistory(toolKey: string, period: string, extraArgs: string[] = [], skipCache = false): Promise<UsageEntry[]> {
  const tool = TOOLS[toolKey];
  if (!tool) throw new Error(`Unknown tool: ${toolKey}`);

  // Check cache (only for vanilla calls — extra args and skipCache bypass cache)
  if (!skipCache && extraArgs.length === 0) {
    const cached = readCache(toolKey);
    if (cached) return cached;
  }

  const raw = await runTool(toolKey, "daily", extraArgs);
  const parsed = parseJson(raw, tool.needsFilter);
  if (!parsed) return [];

  const entries = parsed["daily"] as Record<string, unknown>[] | undefined;
  if (!entries || entries.length === 0) return [];

  const result = entries.map((e) => toUsageEntry(e, "date"));

  if (extraArgs.length === 0) writeCache(toolKey, result);
  return result;
}

// --- Monthly aggregation from daily entries ---

export function aggregateMonthly(dailyEntries: UsageEntry[]): UsageEntry[] {
  const map = new Map<string, UsageEntry>();
  for (const e of dailyEntries) {
    const monthLabel = e.label.slice(0, 7); // "2026-02-20" → "2026-02"
    const existing = map.get(monthLabel);
    if (existing) {
      existing.inputTokens += e.inputTokens;
      existing.outputTokens += e.outputTokens;
      existing.cacheCreationTokens += e.cacheCreationTokens;
      existing.cacheReadTokens += e.cacheReadTokens;
      existing.totalTokens += e.totalTokens;
      existing.totalCost += e.totalCost;
    } else {
      map.set(monthLabel, { ...e, label: monthLabel });
    }
  }
  return [...map.values()].sort((a, b) => a.label.localeCompare(b.label));
}

// --- Merge local + remote entries (used by multi-machine mode) ---

export function mergeEntries(
  localEntries: UsageEntry[],
  remoteEntries: UsageEntry[],
): UsageEntry[] {
  const map = new Map<string, UsageEntry>();

  for (const e of [...localEntries, ...remoteEntries]) {
    const existing = map.get(e.label);
    if (existing) {
      existing.inputTokens += e.inputTokens;
      existing.outputTokens += e.outputTokens;
      existing.cacheCreationTokens += e.cacheCreationTokens;
      existing.cacheReadTokens += e.cacheReadTokens;
      existing.totalTokens += e.totalTokens;
      existing.totalCost += e.totalCost;
    } else {
      map.set(e.label, { ...e });
    }
  }

  return [...map.values()].sort((a, b) => a.label.localeCompare(b.label));
}

export async function fetchAllHistory(
  period: string,
  extraArgs: string[] = [],
  skipCache = false,
): Promise<Map<string, UsageEntry[]>> {
  const entries = Object.entries(TOOLS);
  const results = await Promise.all(
    entries.map(async ([key, tool]) => {
      const history = await fetchHistory(key, period, extraArgs, skipCache);
      return [tool.name, history] as const;
    })
  );
  return new Map(results);
}
