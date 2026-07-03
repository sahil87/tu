import { execFile } from "node:child_process";
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

// ccusage@20 ships a single all-agent CLI. In vendor mode this is the native
// Rust binary vendored at dist/vendor/ccusage/bin/ccusage (exec'd directly, no
// node interpreter); in dev mode it is the npm launcher at node_modules/.bin/ccusage
// (a JS shim that resolves the host's optional native package). Per-tool
// subcommands are expressed via prefixArgs: cc→claude, codex→codex, oc→opencode.
// Bare `ccusage daily` is a v20 all-agents aggregate, so cc must use the
// per-agent `claude` subcommand to avoid over/double-counting other agents.
// The claude subcommand emits the ISO label under "date"; codex/opencode use
// "period" — hence the per-tool labelKey.
const CCUSAGE = useVendor ? `${BIN}/ccusage/bin/ccusage` : `${BIN}/ccusage`;

export const TOOLS: Record<string, ToolConfig> = {
  cc: {
    name: "Claude Code",
    binary: CCUSAGE,
    prefixArgs: ["claude"],
    labelKey: "date",
    needsFilter: false,
  },
  codex: {
    name: "Codex",
    binary: CCUSAGE,
    prefixArgs: ["codex"],
    labelKey: "period",
    needsFilter: true,
  },
  oc: {
    name: "OpenCode",
    binary: CCUSAGE,
    prefixArgs: ["opencode"],
    labelKey: "period",
    needsFilter: true,
  },
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

function execFileAsync(file: string, args: string[], toolName: string): Promise<string> {
  return new Promise((resolve) => {
    execFile(file, args, { encoding: "utf-8", maxBuffer: 10 * 1024 * 1024 }, (error, stdout) => {
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

// The JSON key carrying an entry's ISO date label varies by tool, not by
// period: the `claude` subcommand emits it under "date", while codex/opencode
// emit it under "period". That key lives per-tool as ToolConfig.labelKey.
// normalizeLabel passes ISO labels through unchanged for either spelling.

// Current ISO label for filtering entries to "now"
export function currentLabel(period: string, now: Date = new Date()): string {
  const yyyy = now.getFullYear();
  const mm = String(now.getMonth() + 1).padStart(2, "0");
  if (period === "monthly") return `${yyyy}-${mm}`;
  if (period === "weekly") {
    // Start of the current week (Sunday), local time — consistent with the
    // daily/monthly cases which key on the user's local "now". setDate
    // normalizes month/year underflow (e.g. a Sunday in the previous month).
    const d = new Date(now);
    d.setDate(d.getDate() - d.getDay()); // getDay(): 0 = Sunday
    return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
  }
  const dd = String(now.getDate()).padStart(2, "0");
  return `${yyyy}-${mm}-${dd}`;
}

// Pick the entry matching the current date/month, or return EMPTY.
// labelKey is the JSON key carrying the entry's ISO date label (per-tool:
// "date" for the claude subcommand, "period" for codex/opencode). Defaults to
// "period" so callers with period-keyed fixtures need not pass it.
export function pickCurrentEntry(
  entries: Record<string, unknown>[],
  period: string,
  now: Date = new Date(),
  labelKey: string = "period"
): UsageTotals {
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
  const args = [...tool.prefixArgs, period, "--json", ...extraArgs];
  return execFileAsync(tool.binary, args, tool.name);
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

  return pickCurrentEntry(dailyRaw, "daily", new Date(), tool.labelKey);
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

  const result = entries.map((e) => toUsageEntry(e, tool.labelKey));

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

// --- Client-side date-range filter (used by --since/--until) ---
//
// Inclusive on both ends; `since`/`until` are normalized ISO strings
// (YYYY-MM-DD). Labels are already normalized ISO, so lexicographic string
// compare is a correct total order — an out-of-range or impossible-but-shaped
// bound simply yields a narrower (possibly empty) window. Pure: never mutates
// its input (Constitution V). An undefined bound is open-ended on that side.
export function filterEntriesByRange(
  entries: UsageEntry[],
  since?: string,
  until?: string,
): UsageEntry[] {
  return entries.filter((e) => (!since || e.label >= since) && (!until || e.label <= until));
}

// --- Weekly aggregation from daily entries ---

// Week label: ISO date of the week's Sunday (aligned with ccusage weekly's
// default --start-of-week sunday, so tu weekly rows match ccusage weekly rows).
// UTC arithmetic on the date-only label — immune to local DST transitions.
export function weekLabel(dailyLabel: string): string {
  const d = new Date(`${dailyLabel}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() - d.getUTCDay()); // getUTCDay(): 0 = Sunday
  return d.toISOString().slice(0, 10);
}

export function aggregateWeekly(dailyEntries: UsageEntry[]): UsageEntry[] {
  const map = new Map<string, UsageEntry>();
  for (const e of dailyEntries) {
    const wLabel = weekLabel(e.label);
    const existing = map.get(wLabel);
    if (existing) {
      existing.inputTokens += e.inputTokens;
      existing.outputTokens += e.outputTokens;
      existing.cacheCreationTokens += e.cacheCreationTokens;
      existing.cacheReadTokens += e.cacheReadTokens;
      existing.totalTokens += e.totalTokens;
      existing.totalCost += e.totalCost;
    } else {
      map.set(wLabel, { ...e, label: wLabel });
    }
  }
  return [...map.values()].sort((a, b) => a.label.localeCompare(b.label));
}

// --- Period-to-aggregator mapping ---
// Threads the period dimension through cli.ts dispatch/fetch sites so weekly
// works everywhere monthly does, without a third scattered `period === ...`
// branch. daily is the identity (no rollup).
export function aggregateForPeriod(period: string, entries: UsageEntry[]): UsageEntry[] {
  if (period === "monthly") return aggregateMonthly(entries);
  if (period === "weekly") return aggregateWeekly(entries);
  return entries;
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

// --- Max-merge: per-label whole-entry high-water mark (own-machine self-view) ---
//
// Picks, per date label, whichever whole entry has the greater totalCost —
// never mixing fields across entries and never summing; on ties the entry
// from `a` wins. Used to merge a machine's live fetch (`a`) with its own
// synced repo snapshots (`b`): once Claude Code purges old transcripts, the
// live view of an old day collapses toward zero while the snapshot still
// holds the full value — and summing them would double-count the surviving
// transcripts of partially-purged days.
export function maxMergeEntries(a: UsageEntry[], b: UsageEntry[]): UsageEntry[] {
  const map = new Map<string, UsageEntry>();

  for (const e of [...a, ...b]) {
    const existing = map.get(e.label);
    if (!existing || e.totalCost > existing.totalCost) {
      map.set(e.label, { ...e });
    }
  }

  return [...map.values()].sort((x, y) => x.label.localeCompare(y.label));
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
