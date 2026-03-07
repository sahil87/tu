import { TOOLS, EMPTY, fetchHistory, fetchAllTotals, fetchAllHistory, aggregateMonthly, mergeEntries, currentLabel } from "./fetcher.js";
import { printHistory, printTotal, printTotalHistory, renderHistory, renderTotal, renderTotalHistory } from "../tui/formatter.js";
import type { FormatOptions } from "../tui/formatter.js";
import { readConfig, CONFIG_PATH, TU_HOME, THREE_HOURS_MS, resolveHome, DEFAULT_CONFIG_PATH } from "./config.js";
import { writeMetrics, readRemoteEntries, fullSync } from "../sync/sync.js";
import { runWatch } from "../tui/watch.js";
import { setNoColor } from "../tui/colors.js";
import { existsSync, readFileSync, writeFileSync, appendFileSync, mkdirSync, unlinkSync } from "node:fs";
import { execSync, execFileSync } from "node:child_process";
import { dirname, join } from "node:path";
import { homedir } from "node:os";
import { fileURLToPath } from "node:url";
import type { UsageEntry, UsageTotals } from "./types.js";
import type { TuConfig } from "./config.js";

const _dbg = process.env.TU_DEBUG === "1";
const _t0 = _dbg ? Number(process.env.TU_DEBUG_T0 || Date.now()) : 0;
function _mark(label: string) {
  if (_dbg) process.stderr.write(`[tu] +${Date.now() - _t0}ms  ${label}\n`);
}
_mark("tsx loaded, imports done");

const __cli_dirname = dirname(fileURLToPath(import.meta.url));

// Walk up to project root (where package.json lives) — works from both
// src/node/core/ (dev/test) and dist/ (bundled)
let _pkgDir = __cli_dirname;
while (_pkgDir !== dirname(_pkgDir)) {
  if (existsSync(join(_pkgDir, "package.json"))) break;
  _pkgDir = dirname(_pkgDir);
}
const PKG_VERSION = JSON.parse(readFileSync(join(_pkgDir, "package.json"), "utf-8")).version as string;

function tildefy(p: string): string {
  const home = homedir();
  return p.startsWith(home) ? "~" + p.slice(home.length) : p;
}

export const SHORT_USAGE = `Usage: tu [source] [period] [display]

  tu                Today's cost, all tools
  tu cc             Today's cost, Claude Code
  tu mh             Monthly cost history, all tools
  tu -h             Show full help

Run 'tu help' for all commands.`;

export const FULL_HELP = `Usage: tu [source] [period] [display]

Sources: cc (Claude Code), codex/co (Codex), oc (OpenCode), all (default)
Periods: d/daily (default), m/monthly
Display: (bare) = snapshot, h/history = history
Combined: dh (daily history), mh (monthly history)

Examples:
  tu                   Today's cost, all tools (snapshot)
  tu cc                Today's cost, Claude Code
  tu h                 Daily cost history, all tools (pivot)
  tu cc mh             Monthly cost history, Claude Code
  tu m                 This month's cost, all tools

Setup:
  tu init-conf         Scaffold ~/.tu.conf
  tu init-metrics      Clone metrics repo
  tu sync              Push/pull metrics manually
  tu status            Show config and sync state
  tu update            Update tu to latest version

Help: tu help | tu -h | tu --help

Flags:
  --json               Output data as JSON (data commands only)
  --sync               Sync metrics before fetching (multi mode)
  --fresh / -f         Bypass cache, fetch fresh data (data commands only)
  --watch / -w         Persistent polling mode with live display (data commands only)
  --interval / -i <s>  Poll interval in seconds (default: 10, range: 5-3600)
  --user / -u <user>   Show usage for a specific user (multi mode only)
  --no-color           Disable ANSI color output
  --no-rain            Disable matrix rain animation in watch mode`;


const FIELD_BLOCKS: Record<string, string> = {
  version: "\n# Config schema version\nversion = 2\n",
  mode: "\n# mode: single (default) or multi (enables cross-machine sync)\nmode = single\n",
  metrics_repo:
    "\n# Required for multi mode: git repo URL for metrics storage\n# Uncomment and set before running tu init-metrics\n# metrics_repo = git@github.com:you/tu-metrics.git\n",
  metrics_dir:
    "\n# Optional: local path where the metrics repo is cloned (default: ~/.tu/metrics_repo)\n# metrics_dir = ~/.tu/metrics_repo\n",
  machine:
    "\n# Optional: label for this machine in the metrics repo (default: system hostname)\n# machine = my-macbook\n",
  user:
    "\n# Optional: profile name — groups your machines in the metrics repo (default: system username)\n# user = your-name\n",
  auto_sync:
    "\n# Auto-sync: no longer auto-triggers; use 'tu <cmd> --sync' to sync before fetch\nauto_sync = true\n",
};

function fieldPresent(content: string, field: string): boolean {
  return content.split("\n").some((line) => {
    const trimmed = line.trimStart();
    return !trimmed.startsWith("#") && new RegExp(`^${field}\\s*=`).test(trimmed);
  });
}

function fieldMentioned(content: string, field: string): boolean {
  return new RegExp(`^\\s*#?\\s*${field}\\s*=`, "m").test(content);
}

export function runInitConf(configPath: string = CONFIG_PATH, defaultsPath: string = DEFAULT_CONFIG_PATH): void {
  const dp = tildefy(configPath);
  if (!existsSync(configPath)) {
    mkdirSync(dirname(configPath), { recursive: true });
    writeFileSync(configPath, readFileSync(defaultsPath, "utf-8"));
    console.log(`Created ${dp} — edit it to configure multi-machine sync.`);
    return;
  }

  const content = readFileSync(configPath, "utf-8");
  const missing: string[] = [];
  const commented: string[] = [];
  for (const field of Object.keys(FIELD_BLOCKS)) {
    if (!fieldPresent(content, field)) {
      if (fieldMentioned(content, field)) {
        commented.push(field);
      } else {
        missing.push(field);
      }
    }
  }

  if (missing.length === 0 && commented.length === 0) {
    console.log(`${dp} is already complete.`);
    return;
  }

  if (missing.length > 0) {
    let append = "";
    for (const field of missing) {
      append += FIELD_BLOCKS[field];
    }
    appendFileSync(configPath, append);
    console.log(`Updated ${dp} — added missing fields: ${missing.join(", ")}.`);
  }

  if (commented.length > 0) {
    console.log(`${dp} has commented-out fields that need uncommenting: ${commented.join(", ")}.`);
  }
}

export function runInitMetrics(configPath: string = CONFIG_PATH, defaultsPath: string = DEFAULT_CONFIG_PATH, tuHome: string = TU_HOME): void {
  const dp = tildefy(configPath);
  const config = readConfig(configPath, defaultsPath);

  if (!config.metricsRepo) {
    console.error(`Error: metrics_repo is not set. Add it to ${dp} or tu.default.conf.`);
    process.exit(1);
  }

  if (config.mode !== "multi") {
    console.error(
      `Error: mode=${config.mode}. Set mode=multi to use multi-machine sync.`,
    );
    process.exit(1);
  }

  const metricsDir = config.metricsDir;
  const metricsRepo = config.metricsRepo;

  if (existsSync(metricsDir)) {
    try {
      execSync(`git -C "${metricsDir}" rev-parse --git-dir`, { stdio: "pipe" });
      console.log(`Already initialized: ${metricsDir}`);
      return;
    } catch {
      console.error(
        `Error: ${metricsDir} exists but is not a git repo. Remove it or set a different metrics_dir in ${dp}.`,
      );
      process.exit(1);
    }
  }

  execSync(`git clone "${metricsRepo}" "${metricsDir}"`, { stdio: "inherit" });
  removeCloneMarker(tuHome);
  console.log(`Cloned ${metricsRepo} → ${metricsDir}`);
}

export function relativeTime(ms: number): string {
  const seconds = Math.floor(Math.max(0, ms) / 1000);
  if (seconds < 60) return "<1m ago";
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}


function formatLastSync(tuHome: string, now: Date): string {
  const syncFile = join(tuHome, ".last-sync");
  if (!existsSync(syncFile)) return "never";
  try {
    const syncRaw = readFileSync(syncFile, "utf-8").trim();
    const syncTs = new Date(syncRaw).getTime();
    if (Number.isNaN(syncTs)) return "never";
    return `${relativeTime(now.getTime() - syncTs)} (${syncRaw})`;
  } catch {
    return "never";
  }
}

export function runStatus(
  configPath: string = CONFIG_PATH,
  tuHome: string = TU_HOME,
  now: Date = new Date(),
  defaultsPath: string = DEFAULT_CONFIG_PATH,
): void {
  if (!existsSync(configPath)) {
    console.log(`Mode:        single (no ${tildefy(configPath)})`);
    return;
  }

  const config = readConfig(configPath, defaultsPath);

  if (config.mode !== "multi") {
    console.log("Mode:        single");
    console.log(`Config:      ${tildefy(configPath)} (v${config.version})`);
    return;
  }

  const metricsExists = existsSync(config.metricsDir);
  const metricsLine = metricsExists
    ? tildefy(config.metricsDir)
    : `${tildefy(config.metricsDir)} (NOT FOUND — run 'tu init-metrics')`;

  console.log("Mode:        multi");
  console.log(`User:        ${config.user}`);
  console.log(`Machine:     ${config.machine}`);
  console.log(`Config:      ${tildefy(configPath)} (v${config.version})`);
  console.log(`Metrics:     ${metricsLine}`);
  console.log(`Last sync:   ${formatLastSync(tuHome, now)}`);
  console.log(`Auto-sync:   ${config.autoSync ? "on" : "off"}`);
}

export function runUpdate(): void {
  if (!_pkgDir.includes("/Cellar/tu/")) {
    console.log(`tu v${PKG_VERSION} was not installed via Homebrew.`);
    console.log("Update manually, or reinstall with: brew install wvrdz/tap/tu");
    return;
  }

  console.log(`Current version: v${PKG_VERSION}`);

  try {
    execSync("brew update --quiet", { stdio: "pipe", timeout: 30_000 });
  } catch {
    console.error("Error: could not check for updates (brew update failed). Check your network connection.");
    process.exit(1);
  }

  let latest: string;
  try {
    const infoRaw = execSync("brew info --json=v2 tu", { stdio: "pipe", timeout: 10_000 });
    const info = JSON.parse(infoRaw.toString());
    latest = info.formulae[0].versions.stable;
  } catch {
    console.error("Error: could not determine latest version.");
    process.exit(1);
  }

  if (latest === PKG_VERSION) {
    console.log(`Already up to date (v${PKG_VERSION}).`);
    return;
  }

  console.log(`Updating v${PKG_VERSION} → v${latest}...`);

  try {
    execSync("brew upgrade tu", { stdio: "inherit", timeout: 120_000 });
  } catch {
    console.error("Error: brew upgrade failed.");
    process.exit(1);
  }

  console.log(`Updated to v${latest}.`);
}

const CLONE_FAILED_MARKER = ".clone-failed";
const CLONE_RETRY_MS = THREE_HOURS_MS;

function isCloneMarkerFresh(tuHome: string, now: Date = new Date()): boolean {
  const markerPath = join(tuHome, CLONE_FAILED_MARKER);
  if (!existsSync(markerPath)) return false;
  try {
    const raw = readFileSync(markerPath, "utf-8").trim();
    const ts = new Date(raw).getTime();
    if (Number.isNaN(ts)) return false; // malformed → treat as stale
    return (now.getTime() - ts) < CLONE_RETRY_MS;
  } catch {
    return false; // unreadable → treat as stale
  }
}

function writeCloneMarker(tuHome: string): void {
  mkdirSync(tuHome, { recursive: true });
  writeFileSync(join(tuHome, CLONE_FAILED_MARKER), new Date().toISOString());
}

export function removeCloneMarker(tuHome: string = TU_HOME): void {
  const markerPath = join(tuHome, CLONE_FAILED_MARKER);
  try {
    if (existsSync(markerPath)) unlinkSync(markerPath);
  } catch {
    // best-effort cleanup
  }
}

export function checkMetricsDirGuard(config: TuConfig, tuHome: string = TU_HOME): TuConfig {
  if (config.mode !== "multi" || existsSync(config.metricsDir)) return config;

  // No metricsRepo → can't clone, fall back
  if (!config.metricsRepo) {
    process.stderr.write(`Warning: metrics repo not found at ${config.metricsDir} — falling back to single mode. Run 'tu init-metrics' to enable multi-machine sync.\n`);
    return { ...config, mode: "single" };
  }

  // Check for recent clone failure
  if (isCloneMarkerFresh(tuHome)) {
    process.stderr.write("Warning: metrics repo not available — falling back to single mode.\n");
    return { ...config, mode: "single" };
  }

  // Attempt auto-clone
  try {
    execFileSync("git", ["clone", config.metricsRepo, config.metricsDir], {
      stdio: "pipe",
      env: { ...process.env, GIT_TERMINAL_PROMPT: "0" },
      timeout: 30_000,
    });
    process.stderr.write(`Cloned metrics repo → ${config.metricsDir}\n`);
    removeCloneMarker(tuHome);
    return config;
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err);
    writeCloneMarker(tuHome);
    process.stderr.write(`Warning: could not clone metrics repo (${msg}) — falling back to single mode.\n`);
    return { ...config, mode: "single" };
  }
}

export async function runSync(configPath: string = CONFIG_PATH, tuHome: string = TU_HOME, defaultsPath: string = DEFAULT_CONFIG_PATH): Promise<void> {
  const config = readConfig(configPath, defaultsPath);
  if (config.mode !== "multi") {
    console.error(
      "tu sync requires multi-machine mode.\nRun 'tu init-conf' and set mode=multi to enable cross-machine sync.",
    );
    process.exit(1);
  }
  const guardedConfig = checkMetricsDirGuard(config, tuHome);
  if (guardedConfig.mode !== "multi") {
    // Auto-clone failed or metricsDir still missing
    process.exit(1);
  }

  const ok = await fullSync(guardedConfig, tuHome);
  if (!ok) {
    console.error("Error: sync failed — check network and remote config.");
    process.exit(1);
  }
  console.log(`Synced to ${tildefy(config.metricsDir)}`);
}

async function fetchToolMerged(
  config: TuConfig,
  toolKey: string,
  period: string,
  extra: string[],
  skipCache = false,
  targetUser?: string,
): Promise<UsageEntry[]> {
  if (targetUser) {
    _mark(`fetchToolMerged(${toolKey}) → readRemote for ${targetUser}`);
    const entries = readRemoteEntries(config.metricsDir, targetUser, null, toolKey);
    _mark(`fetchToolMerged(${toolKey}) → readRemote done (${entries.length} entries)`);
    if (period === "monthly") return aggregateMonthly(entries);
    return entries;
  }
  _mark(`fetchToolMerged(${toolKey}) → fetchHistory`);
  const local = await fetchHistory(toolKey, "daily", extra, skipCache);
  _mark(`fetchToolMerged(${toolKey}) → fetchHistory done (${local.length} entries)`);
  writeMetrics(config.metricsDir, config.user, config.machine, toolKey, local);
  _mark(`fetchToolMerged(${toolKey}) → writeMetrics done`);
  const remote = readRemoteEntries(config.metricsDir, config.user, config.machine, toolKey);
  _mark(`fetchToolMerged(${toolKey}) → readRemote done (${remote.length} entries)`);
  const merged = mergeEntries(local, remote);
  if (period === "monthly") return aggregateMonthly(merged);
  return merged;
}

function emitJson(data: unknown): void {
  const obj = data instanceof Map ? Object.fromEntries(data) : data;
  console.log(JSON.stringify(obj, null, 2));
}

// Cost tracking for watch mode — set by dispatch functions, read by getCost/getPrevCosts callbacks
let _lastRenderCost = 0;
let _lastRenderCostMap = new Map<string, number>();
let _lastRenderTotalTokens = 0;

export interface GlobalFlags {
  jsonFlag: boolean;
  syncFlag: boolean;
  freshFlag: boolean;
  watchFlag: boolean;
  watchInterval: number;
  noColorFlag: boolean;
  noRainFlag: boolean;
  userFlag: string | undefined;
  filteredArgs: string[];
}

export function parseGlobalFlags(rawArgs: string[]): GlobalFlags {
  const jsonFlag = rawArgs.includes("--json");
  const syncFlag = rawArgs.includes("--sync");
  const freshFlag = rawArgs.includes("--fresh") || rawArgs.includes("-f");
  const watchFlag = rawArgs.includes("--watch") || rawArgs.includes("-w");
  const noColorFlag = rawArgs.includes("--no-color");
  const noRainFlag = rawArgs.includes("--no-rain");

  let watchInterval = 10;
  let hasIntervalFlag = false;
  let rawIntervalVal: string | undefined;
  let userFlag: string | undefined;
  let hasUserFlag = false;
  const filteredArgs: string[] = [];
  for (let i = 0; i < rawArgs.length; i++) {
    const a = rawArgs[i];
    if (a === "--json" || a === "--sync" || a === "--fresh" || a === "-f" || a === "--watch" || a === "-w" || a === "--no-color" || a === "--no-rain") continue;
    if (a === "--interval" || a === "-i") {
      hasIntervalFlag = true;
      const next = rawArgs[i + 1];
      if (next !== undefined && /^\d+$/.test(next)) {
        rawIntervalVal = next;
        i++;
      }
      continue;
    }
    if (a === "--user" || a === "-u") {
      hasUserFlag = true;
      const next = rawArgs[i + 1];
      if (next !== undefined && !next.startsWith("-")) {
        userFlag = next;
        i++;
      }
      continue;
    }
    filteredArgs.push(a);
  }

  if (watchFlag && hasIntervalFlag) {
    if (rawIntervalVal === undefined) {
      console.error("Error: --interval requires a numeric value");
      process.exit(1);
    }
    const num = Number(rawIntervalVal);
    if (num < 5) {
      console.error("Error: --interval minimum is 5 seconds");
      process.exit(1);
    }
    if (num > 3600) {
      console.error("Error: --interval maximum is 3600 seconds");
      process.exit(1);
    }
    watchInterval = num;
  }

  if (watchFlag && jsonFlag) {
    console.error("Error: --watch and --json are incompatible");
    process.exit(1);
  }

  if (hasUserFlag && userFlag === undefined) {
    console.error("Error: -u requires a username");
    process.exit(1);
  }

  return { jsonFlag, syncFlag, freshFlag, watchFlag, watchInterval, noColorFlag, noRainFlag, userFlag, filteredArgs };
}

const KNOWN_SOURCES = new Set(["cc", "codex", "co", "oc", "all"]);
const SOURCE_ALIASES: Record<string, string> = { co: "codex" };

export interface DataArgs {
  source: string;
  period: string;
  display: string;
}

export function parseDataArgs(args: string[]): DataArgs {
  let source = "all";
  let period = "daily";
  let display = "snapshot";
  const remaining = [...args];

  if (remaining.length > 0 && KNOWN_SOURCES.has(remaining[0])) {
    source = SOURCE_ALIASES[remaining[0]] || remaining[0];
    remaining.shift();
  }

  for (const arg of remaining) {
    if (arg === "d" || arg === "daily") {
      period = "daily";
    } else if (arg === "m" || arg === "monthly") {
      period = "monthly";
    } else if (arg === "h" || arg === "history") {
      display = "history";
    } else if (arg === "dh") {
      period = "daily";
      display = "history";
    } else if (arg === "mh") {
      period = "monthly";
      display = "history";
    } else {
      throw new Error(`Unknown argument: ${arg}`);
    }
  }

  return { source, period, display };
}

async function dispatchAllHistory(config: TuConfig, period: string, jsonFlag: boolean, skipCache = false, fmtOpts?: FormatOptions, targetUser?: string): Promise<void> {
  if (config.mode === "multi") {
    const toolKeys = Object.keys(TOOLS);
    const allMerged = await Promise.all(toolKeys.map((k) => fetchToolMerged(config, k, period, [], skipCache, targetUser)));
    const mergedMap = new Map<string, UsageEntry[]>();
    for (let i = 0; i < toolKeys.length; i++) {
      mergedMap.set(TOOLS[toolKeys[i]].name, allMerged[i]);
    }
    if (jsonFlag) { emitJson(mergedMap); }
    else { printTotalHistory(period, mergedMap, undefined, fmtOpts); }
    _lastRenderCost = sumAllToolCosts(mergedMap);
    _lastRenderCostMap = buildCostMap(mergedMap);
  } else {
    const results = await fetchAllHistory("daily", [], skipCache);
    if (period === "monthly") {
      const aggregated = new Map<string, UsageEntry[]>();
      for (const [name, entries] of results) {
        aggregated.set(name, aggregateMonthly(entries));
      }
      if (jsonFlag) { emitJson(aggregated); }
      else { printTotalHistory(period, aggregated, undefined, fmtOpts); }
      _lastRenderCost = sumAllToolCosts(aggregated);
      _lastRenderCostMap = buildCostMap(aggregated);
    } else {
      if (jsonFlag) { emitJson(results); }
      else { printTotalHistory(period, results, undefined, fmtOpts); }
      _lastRenderCost = sumAllToolCosts(results);
      _lastRenderCostMap = buildCostMap(results);
    }
  }
}

async function dispatchAllSnapshot(config: TuConfig, period: string, jsonFlag: boolean, skipCache = false, fmtOpts?: FormatOptions, targetUser?: string): Promise<void> {
  if (config.mode === "multi") {
    const toolKeys = Object.keys(TOOLS);
    const allMerged = await Promise.all(toolKeys.map((k) => fetchToolMerged(config, k, period, [], skipCache, targetUser)));
    const result = new Map<string, UsageTotals>();
    for (let i = 0; i < toolKeys.length; i++) {
      const current = allMerged[i].find((e) => e.label === currentLabel(period));
      result.set(TOOLS[toolKeys[i]].name, current ?? { ...EMPTY });
    }
    if (jsonFlag) { emitJson(result); }
    else { printTotal(period, result, fmtOpts); }
    _lastRenderCost = sumToolTotalsCost(result);
    _lastRenderCostMap = buildCostMap(result);
  } else {
    if (period === "monthly") {
      const toolKeys = Object.keys(TOOLS);
      const allDaily = await Promise.all(toolKeys.map((k) => fetchHistory(k, "daily", [], skipCache)));
      const result = new Map<string, UsageTotals>();
      const target = currentLabel("monthly");
      for (let i = 0; i < toolKeys.length; i++) {
        const monthly = aggregateMonthly(allDaily[i]);
        const match = monthly.find((m) => m.label === target);
        result.set(TOOLS[toolKeys[i]].name, match ?? { ...EMPTY });
      }
      if (jsonFlag) { emitJson(result); }
      else { printTotal(period, result, fmtOpts); }
      _lastRenderCost = sumToolTotalsCost(result);
      _lastRenderCostMap = buildCostMap(result);
    } else {
      const results = await fetchAllTotals([]);
      if (jsonFlag) { emitJson(results); }
      else { printTotal(period, results, fmtOpts); }
      _lastRenderCost = sumToolTotalsCost(results);
      _lastRenderCostMap = buildCostMap(results);
    }
  }
}

async function dispatchSingleTool(
  config: TuConfig, toolKey: string, period: string, display: string, jsonFlag: boolean, skipCache = false, fmtOpts?: FormatOptions, targetUser?: string,
): Promise<void> {
  const toolCfg = TOOLS[toolKey];
  if (!toolCfg) {
    console.error(`Unknown tool: ${toolKey}`);
    console.log(SHORT_USAGE);
    process.exit(1);
  }

  _mark(`fetching ${toolKey} ${period}`);
  let entries: UsageEntry[];
  if (config.mode === "multi") {
    entries = await fetchToolMerged(config, toolKey, period, [], skipCache, targetUser);
  } else {
    entries = await fetchHistory(toolKey, "daily", [], skipCache);
    if (period === "monthly") entries = aggregateMonthly(entries);
  }
  _mark("fetch done");

  if (display === "history") {
    if (jsonFlag) { emitJson(entries); }
    else { printHistory(toolCfg.name, period, entries, undefined, fmtOpts); }
    _lastRenderCost = entries.reduce((sum, e) => sum + e.totalCost, 0);
    _lastRenderCostMap = buildCostMap(entries, toolCfg.name);
  } else {
    const target = currentLabel(period);
    const current = entries.find((e) => e.label === target);
    const result = new Map<string, UsageTotals>();
    result.set(toolCfg.name, current ?? { ...EMPTY });
    if (jsonFlag) { emitJson(result); }
    else { printTotal(period, result, fmtOpts); }
    _lastRenderCost = sumToolTotalsCost(result);
    _lastRenderCostMap = buildCostMap(result);
  }
  _mark("done");
}

function sumToolTotalsCost(m: Map<string, UsageTotals>): number {
  let sum = 0;
  for (const t of m.values()) sum += t.totalCost;
  return sum;
}

function sumAllToolCosts(m: Map<string, UsageEntry[]>): number {
  let sum = 0;
  for (const entries of m.values()) {
    for (const e of entries) sum += e.totalCost;
  }
  return sum;
}

function buildCostMap(data: Map<string, UsageTotals> | Map<string, UsageEntry[]> | UsageEntry[], toolName?: string): Map<string, number> {
  const m = new Map<string, number>();
  if (Array.isArray(data)) {
    // Single-tool history entries
    for (const e of data) {
      m.set(`${toolName}:${e.label}`, e.totalCost);
    }
    // Also track total per label for total-history delta keys
    for (const e of data) {
      m.set(`total:${e.label}`, (m.get(`total:${e.label}`) || 0) + e.totalCost);
    }
  } else {
    // Check if values are arrays (history) or UsageTotals (snapshot)
    const first = [...data.values()][0];
    if (Array.isArray(first)) {
      // Map<string, UsageEntry[]> — all-tools history
      for (const [name, entries] of data as Map<string, UsageEntry[]>) {
        for (const e of entries) {
          m.set(`${name}:${e.label}`, e.totalCost);
          m.set(`total:${e.label}`, (m.get(`total:${e.label}`) || 0) + e.totalCost);
        }
      }
    } else {
      // Map<string, UsageTotals> — all-tools snapshot
      for (const [name, t] of data as Map<string, UsageTotals>) {
        m.set(name, t.totalCost);
      }
    }
  }
  return m;
}

// --- Watch-mode dispatch variants returning string[] ---

async function dispatchAllHistoryLines(config: TuConfig, period: string, skipCache = false, fmtOpts?: FormatOptions, targetUser?: string): Promise<string[]> {
  if (config.mode === "multi") {
    const toolKeys = Object.keys(TOOLS);
    const allMerged = await Promise.all(toolKeys.map((k) => fetchToolMerged(config, k, period, [], skipCache, targetUser)));
    const mergedMap = new Map<string, UsageEntry[]>();
    for (let i = 0; i < toolKeys.length; i++) {
      mergedMap.set(TOOLS[toolKeys[i]].name, allMerged[i]);
    }
    _lastRenderCost = sumAllToolCosts(mergedMap);
    _lastRenderCostMap = buildCostMap(mergedMap);
    _lastRenderTotalTokens = sumAllToolTokens(mergedMap);
    return renderTotalHistory(period, mergedMap, undefined, fmtOpts);
  } else {
    const results = await fetchAllHistory("daily", [], skipCache);
    if (period === "monthly") {
      const aggregated = new Map<string, UsageEntry[]>();
      for (const [name, entries] of results) {
        aggregated.set(name, aggregateMonthly(entries));
      }
      _lastRenderCost = sumAllToolCosts(aggregated);
      _lastRenderCostMap = buildCostMap(aggregated);
      _lastRenderTotalTokens = sumAllToolTokens(aggregated);
      return renderTotalHistory(period, aggregated, undefined, fmtOpts);
    } else {
      _lastRenderCost = sumAllToolCosts(results);
      _lastRenderCostMap = buildCostMap(results);
      _lastRenderTotalTokens = sumAllToolTokens(results);
      return renderTotalHistory(period, results, undefined, fmtOpts);
    }
  }
}

async function dispatchAllSnapshotLines(config: TuConfig, period: string, skipCache = false, fmtOpts?: FormatOptions, targetUser?: string): Promise<string[]> {
  if (config.mode === "multi") {
    const toolKeys = Object.keys(TOOLS);
    const allMerged = await Promise.all(toolKeys.map((k) => fetchToolMerged(config, k, period, [], skipCache, targetUser)));
    const result = new Map<string, UsageTotals>();
    for (let i = 0; i < toolKeys.length; i++) {
      const current = allMerged[i].find((e) => e.label === currentLabel(period));
      result.set(TOOLS[toolKeys[i]].name, current ?? { ...EMPTY });
    }
    _lastRenderCost = sumToolTotalsCost(result);
    _lastRenderCostMap = buildCostMap(result);
    _lastRenderTotalTokens = sumToolTotalsTokens(result);
    return renderTotal(period, result, fmtOpts);
  } else {
    if (period === "monthly") {
      const toolKeys = Object.keys(TOOLS);
      const allDaily = await Promise.all(toolKeys.map((k) => fetchHistory(k, "daily", [], skipCache)));
      const result = new Map<string, UsageTotals>();
      const target = currentLabel("monthly");
      for (let i = 0; i < toolKeys.length; i++) {
        const monthly = aggregateMonthly(allDaily[i]);
        const match = monthly.find((m) => m.label === target);
        result.set(TOOLS[toolKeys[i]].name, match ?? { ...EMPTY });
      }
      _lastRenderCost = sumToolTotalsCost(result);
      _lastRenderCostMap = buildCostMap(result);
      _lastRenderTotalTokens = sumToolTotalsTokens(result);
      return renderTotal(period, result, fmtOpts);
    } else {
      const results = await fetchAllTotals([]);
      _lastRenderCost = sumToolTotalsCost(results);
      _lastRenderCostMap = buildCostMap(results);
      _lastRenderTotalTokens = sumToolTotalsTokens(results);
      return renderTotal(period, results, fmtOpts);
    }
  }
}

async function dispatchSingleToolLines(
  config: TuConfig, toolKey: string, period: string, display: string, skipCache = false, fmtOpts?: FormatOptions, targetUser?: string,
): Promise<string[]> {
  const toolCfg = TOOLS[toolKey];
  if (!toolCfg) return [`Unknown tool: ${toolKey}`];

  let entries: UsageEntry[];
  if (config.mode === "multi") {
    entries = await fetchToolMerged(config, toolKey, period, [], skipCache, targetUser);
  } else {
    entries = await fetchHistory(toolKey, "daily", [], skipCache);
    if (period === "monthly") entries = aggregateMonthly(entries);
  }

  if (display === "history") {
    _lastRenderCost = entries.reduce((sum, e) => sum + e.totalCost, 0);
    _lastRenderCostMap = buildCostMap(entries, toolCfg.name);
    _lastRenderTotalTokens = entries.reduce((sum, e) => sum + e.totalTokens, 0);
    return renderHistory(toolCfg.name, period, entries, undefined, fmtOpts);
  } else {
    const target = currentLabel(period);
    const current = entries.find((e) => e.label === target);
    const result = new Map<string, UsageTotals>();
    result.set(toolCfg.name, current ?? { ...EMPTY });
    _lastRenderCost = sumToolTotalsCost(result);
    _lastRenderCostMap = buildCostMap(result);
    _lastRenderTotalTokens = sumToolTotalsTokens(result);
    return renderTotal(period, result, fmtOpts);
  }
}

function sumAllToolTokens(m: Map<string, UsageEntry[]>): number {
  let sum = 0;
  for (const entries of m.values()) {
    for (const e of entries) sum += e.totalTokens;
  }
  return sum;
}

function sumToolTotalsTokens(m: Map<string, UsageTotals>): number {
  let sum = 0;
  for (const t of m.values()) sum += t.totalTokens;
  return sum;
}

async function main() {
  _mark("main() entered");
  const rawArgs = process.argv.slice(2);
  let { jsonFlag, syncFlag, freshFlag, watchFlag, watchInterval, noColorFlag, noRainFlag, userFlag, filteredArgs } = parseGlobalFlags(rawArgs);

  if (noColorFlag) setNoColor(true);

  if (rawArgs.includes("--version") || rawArgs.includes("-V")) {
    console.log(PKG_VERSION);
    return;
  }

  // Help — check first arg for help / -h / --help
  if (filteredArgs.length > 0 && (filteredArgs[0] === "help" || filteredArgs[0] === "-h" || filteredArgs[0] === "--help")) {
    console.log(FULL_HELP);
    return;
  }

  // Non-data commands — dispatch before grammar parsing
  if (filteredArgs.length > 0) {
    const cmd = filteredArgs[0];
    if (cmd === "init-conf") { runInitConf(); return; }
    if (cmd === "init-metrics") { runInitMetrics(); return; }
    if (cmd === "sync") { await runSync(); return; }
    if (cmd === "status") { runStatus(); return; }
    if (cmd === "update") { runUpdate(); return; }
  }

  // Parse positional data args (source, period, display)
  let parsed: DataArgs;
  try {
    parsed = parseDataArgs(filteredArgs);
  } catch (err: unknown) {
    console.error((err as Error).message);
    console.log(SHORT_USAGE);
    process.exit(1);
  }
  const { source, period, display } = parsed;

  _mark("readConfig()");
  const config = checkMetricsDirGuard(readConfig());
  _mark(`config loaded (mode=${config.mode})`);

  if (userFlag && config.mode !== "multi") {
    process.stderr.write("Warning: -u flag requires multi mode — ignoring.\n");
    userFlag = undefined;
  }

  if (syncFlag && config.mode === "multi") {
    process.stderr.write("syncing metrics... ");
    const ok = await fullSync(config);
    if (ok) {
      process.stderr.write("synced.\n");
    } else {
      process.stderr.write("sync failed — using local data.\n");
    }
  }

  if (watchFlag) {
    const action = async (skipCache: boolean, fmtOpts?: FormatOptions): Promise<string[]> => {
      if (source === "all") {
        if (display === "history") { return dispatchAllHistoryLines(config, period, skipCache, fmtOpts, userFlag); }
        else { return dispatchAllSnapshotLines(config, period, skipCache, fmtOpts, userFlag); }
      } else {
        return dispatchSingleToolLines(config, source, period, display, skipCache, fmtOpts, userFlag);
      }
    };
    await runWatch({
      interval: watchInterval,
      action,
      getCost: () => _lastRenderCost,
      getPrevCosts: () => new Map(_lastRenderCostMap),
      getTotalTokens: () => _lastRenderTotalTokens,
      noRain: noRainFlag,
    });
  } else {
    if (source === "all") {
      if (display === "history") { await dispatchAllHistory(config, period, jsonFlag, freshFlag, undefined, userFlag); }
      else { await dispatchAllSnapshot(config, period, jsonFlag, freshFlag, undefined, userFlag); }
    } else {
      await dispatchSingleTool(config, source, period, display, jsonFlag, freshFlag, undefined, userFlag);
    }
  }
}

main().catch((err) => {
  console.error(err.message);
  process.exit(1);
});
