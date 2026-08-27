import { TOOLS, EMPTY, fetchHistory, fetchAllTotals, fetchAllHistory, aggregateForPeriod, mergeEntries, maxMergeEntries, currentLabel, filterEntriesByRange } from "./fetcher.js";
import { printHistory, printTotal, printTotalHistory, renderHistory, renderTotal, renderTotalHistory, emitCsv, emitMarkdown } from "../tui/formatter.js";
import type { FormatOptions, BarMetric } from "../tui/formatter.js";
import { readConfig, resolveConfigPaths, selectUserConfPath, TU_HOME, THREE_HOURS_MS, resolveHome, DEFAULT_CONFIG_PATH } from "./config.js";
import { writeMetrics, readRemoteEntries, readRemoteEntriesByMachine, listUsers, fullSync } from "../sync/sync.js";
import type { DrySyncReport } from "../sync/sync.js";
import { runWatch } from "../tui/watch.js";
import { setNoColor } from "../tui/colors.js";
import { BASH_COMPLETION, ZSH_COMPLETION, FISH_COMPLETION } from "./completions.js";
import { buildHelpDoc } from "./help-dump.js";
import { SKILL_MD } from "./skill.js";
import { existsSync, readFileSync, writeFileSync, appendFileSync, mkdirSync, unlinkSync } from "node:fs";
import { execSync, execFileSync } from "node:child_process";
import { dirname, join } from "node:path";
import { homedir } from "node:os";
import { fileURLToPath } from "node:url";
import type { UsageEntry, UsageTotals } from "./types.js";
import type { TuConfig, ConfigPaths } from "./config.js";

const _dbg = process.env.TU_DEBUG === "1";
const _t0 = _dbg ? Number(process.env.TU_DEBUG_T0 || Date.now()) : 0;
function _mark(label: string) {
  if (_dbg) process.stderr.write(`[tu] +${Date.now() - _t0}ms  ${label}\n`);
}
_mark("tsx loaded, imports done");

const __cli_dirname = dirname(fileURLToPath(import.meta.url));

// Package metadata injected at build time by esbuild --define; falls back to
// package.json for dev. The bottle ships only dist/ (no package.json at
// runtime), so help-dump's name/description MUST be embedded at build time —
// hence all three are --define'd, read from one resolved package.json in dev.
declare const __PKG_VERSION__: string | undefined;
declare const __PKG_NAME__: string | undefined;
declare const __PKG_DESCRIPTION__: string | undefined;

// Dev fallback only: when the defines are absent (running via tsx, not the
// bundled binary), resolve package.json once. In the bundled binary all three
// are --define'd, so this walk never runs — preserving fast startup.
function readDevPkg(): { version?: string; name?: string; description?: string } {
  let dir = __cli_dirname;
  while (dir !== dirname(dir)) {
    const p = join(dir, "package.json");
    if (existsSync(p)) return JSON.parse(readFileSync(p, "utf-8"));
    dir = dirname(dir);
  }
  return {};
}

const _devPkg =
  typeof __PKG_VERSION__ === "undefined" ||
  typeof __PKG_NAME__ === "undefined" ||
  typeof __PKG_DESCRIPTION__ === "undefined"
    ? readDevPkg()
    : {};

const PKG_VERSION: string = typeof __PKG_VERSION__ !== "undefined" ? __PKG_VERSION__ : (_devPkg.version ?? "0.0.0");
const PKG_NAME: string = typeof __PKG_NAME__ !== "undefined" ? __PKG_NAME__ : (_devPkg.name ?? "tu");
const PKG_DESCRIPTION: string =
  typeof __PKG_DESCRIPTION__ !== "undefined" ? __PKG_DESCRIPTION__ : (_devPkg.description ?? "");

function tildefy(p: string): string {
  const home = homedir();
  return p.startsWith(home) ? "~" + p.slice(home.length) : p;
}

// Exit-code convention (shll toolkit principle №4): 0 = success,
// 1 = operational failure (retry / check env), 2 = usage error (fix the
// invocation). Usage-error sites exit with EXIT_USAGE; operational failures
// keep their literal 1.
const EXIT_USAGE = 2;

// Reserved `-u` value: aggregate every user profile in the metrics repo. A real
// profile named "all" would be indistinguishable from the flag and would be
// double-counted by listUsers, so config.user === ALL_USERS is rejected.
const ALL_USERS = "all";

export function isUserReserved(user: string): boolean {
  return user === ALL_USERS;
}

// Runs right after readConfig() on every path that loads config for data or
// writes (main() data commands, `tu sync`): a bad config value is
// invocation-fixable, so it exits with the usage code.
function assertUserNotReserved(config: TuConfig): void {
  if (isUserReserved(config.user)) {
    console.error(`Error: config user "${ALL_USERS}" is reserved (used by -u ${ALL_USERS})`);
    process.exit(EXIT_USAGE);
  }
}

export const SHORT_USAGE = `Usage: tu [source] [period] [display]

  tu                Today's cost, all tools
  tu cc             Today's cost, Claude Code
  tu mh             Monthly cost history, all tools
  tu -h             Show full help

Run 'tu help' for all commands.`;

export const FULL_HELP = `Usage: tu [source] [period] [display]

Sources: cc (Claude Code), codex/co (Codex), oc (OpenCode), gemini/gem (Gemini), copilot/cop (Copilot), kimi/ki (Kimi), all (default)
Periods: d/daily (default), w/weekly, m/monthly
Display: (bare) = snapshot, h/history = history
Combined: dh (daily history), wh (weekly history), mh (monthly history)

Examples:
  tu                   Today's cost, all tools (snapshot)
  tu cc                Today's cost, Claude Code
  tu h                 Daily cost history, all tools (pivot)
  tu cc mh             Monthly cost history, Claude Code
  tu wh                Weekly cost history, all tools
  tu m                 This month's cost, all tools

Setup:
  tu init-conf         Scaffold ~/.config/tu/tu.conf
  tu init-metrics [url] Clone metrics repo (url also sets metrics_repo)
  tu sync              Push/pull metrics manually
  tu status            Show config and sync state
  tu update            Update tu to latest version
  tu shell-init <sh>   Emit shell init script (bash/zsh/fish)
  tu skill             Print agent usage bundle (markdown)

Help: tu help | tu -h | tu --help

Flags:
  --json / -j          Output data as JSON (data commands only)
  --csv                Output data as CSV (data commands only)
  --md                 Output data as Markdown (data commands only)
  --since / -s <date>  Only include entries on/after date (YYYY-MM-DD or YYYYMMDD, history display)
  --until <date>       Only include entries on/before date (YYYY-MM-DD or YYYYMMDD, history display)
  --full               Show full history (default: last 3 months for daily/weekly history)
  --metric <m>         Scale history bars by 'cost' (default) or 'tokens' (history display)
  --sync               Sync metrics before fetching (multi mode)
  --dry-run            Preview sync without writing (tu sync only)
  --fresh / -f         Bypass cache, fetch fresh data (data commands only)
  --watch / -w         Persistent polling mode with live display (data commands only)
  --interval / -i <s>  Poll interval in seconds (default: 10, range: 5-3600)
  --user / -u <user>   Show usage for a specific user, or 'all' for every user
                       in the metrics repo (multi mode only; repo data — sync for today)
  --by-machine         Show per-machine cost breakdown (data commands only)
  --skip-brew-update   Skip 'brew update' tap refresh during 'tu update'
  --no-color           Disable ANSI color output
  --no-rain            Disable matrix rain animation in watch mode`;


const FIELD_BLOCKS: Record<string, string> = {
  version: "\n# Config schema version\nversion = 2\n",
  metrics_repo:
    "\n# Git repo URL for metrics storage (enables multi-machine sync)\n# Set here or via TU_METRICS_REPO env var\n# metrics_repo = git@github.com:you/tu-metrics.git\n",
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

// A bare string argument is treated as userConf only (no org layer, no legacy
// fallback) — the form existing tests use. The directory component keeps
// ensureUserConf's mkdir working for that form.
function normalizePaths(paths: ConfigPaths | string): ConfigPaths {
  return typeof paths === "string"
    ? { configDir: dirname(paths), userConf: paths }
    : paths;
}

// Ensure the user conf exists, creating the config dir and file when missing.
// A legacy ~/.tu.conf seeds the new file (write-time copy — the user's
// machine/user/metrics_dir overrides are not silently orphaned; this is not
// the rejected read-time auto-migration). Returns true when the file was
// created. Shared by runInitConf and runInitMetrics.
export function ensureUserConf(paths: ConfigPaths, defaultsPath: string): boolean {
  if (existsSync(paths.userConf)) return false;
  mkdirSync(paths.configDir, { recursive: true });
  if (paths.legacyConf !== undefined && existsSync(paths.legacyConf)) {
    writeFileSync(paths.userConf, readFileSync(paths.legacyConf, "utf-8"));
    console.log(`Copied ${tildefy(paths.legacyConf)} → ${tildefy(paths.userConf)}`);
  } else {
    writeFileSync(paths.userConf, readFileSync(defaultsPath, "utf-8"));
    console.log(`Created ${tildefy(paths.userConf)} — edit it to configure multi-machine sync.`);
  }
  return true;
}

// Write metrics_repo = <url> into the user conf: replace an active assignment
// in place, else replace the scaffold's commented sample line, else append the
// FIELD_BLOCKS.metrics_repo block with the value filled in.
function setMetricsRepoInConf(userConf: string, repoUrl: string): void {
  const content = readFileSync(userConf, "utf-8");
  const lines = content.split("\n");
  const activeIdx = lines.findIndex((l) => {
    const t = l.trimStart();
    return !t.startsWith("#") && /^metrics_repo\s*=/.test(t);
  });
  if (activeIdx >= 0) {
    lines[activeIdx] = `metrics_repo = ${repoUrl}`;
    writeFileSync(userConf, lines.join("\n"));
    return;
  }
  const commentedIdx = lines.findIndex((l) => /^\s*#\s*metrics_repo\s*=/.test(l));
  if (commentedIdx >= 0) {
    lines[commentedIdx] = `metrics_repo = ${repoUrl}`;
    writeFileSync(userConf, lines.join("\n"));
    return;
  }
  appendFileSync(
    userConf,
    FIELD_BLOCKS.metrics_repo.replace("# metrics_repo = git@github.com:you/tu-metrics.git", `metrics_repo = ${repoUrl}`),
  );
}

export function runInitConf(paths: ConfigPaths | string = resolveConfigPaths(), defaultsPath: string = DEFAULT_CONFIG_PATH): void {
  const p = normalizePaths(paths);
  if (ensureUserConf(p, defaultsPath)) return;

  const configPath = p.userConf;
  const dp = tildefy(configPath);
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

export function runInitMetrics(
  paths: ConfigPaths | string = resolveConfigPaths(),
  defaultsPath: string = DEFAULT_CONFIG_PATH,
  tuHome: string = TU_HOME,
  repoUrl?: string,
): void {
  const p = normalizePaths(paths);
  let overrides: { metrics_repo?: string } = {};
  if (repoUrl !== undefined) {
    // The URL is written verbatim into tu.conf — reject newline/CR so a
    // crafted argument cannot inject extra config lines into the file.
    if (/[\r\n]/.test(repoUrl)) {
      console.error("Error: repo-url must be a single line (no newline or carriage-return characters).");
      process.exit(1);
    }
    // CLI-flag layer: the typed URL is written into tu.conf and beats an
    // exported TU_METRICS_REPO for this invocation's clone (CLI > env).
    ensureUserConf(p, defaultsPath);
    setMetricsRepoInConf(p.userConf, repoUrl);
    console.log(`Set metrics_repo = ${repoUrl} in ${tildefy(p.userConf)}`);
    overrides = { metrics_repo: repoUrl };
  }
  const config = readConfig(p, defaultsPath, overrides);
  const dp = tildefy(p.userConf);

  if (!config.metricsRepo) {
    console.error(`Error: metrics_repo is not set. Add it to ${dp}, run 'tu init-metrics <repo-url>', or set TU_METRICS_REPO.`);
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

// Printed only when an org.conf exists, directly after the Config: line (or
// after the single-mode no-config line) — explains why a teammate is in multi
// mode with no metrics_repo in their own file.
function printOrgLine(p: ConfigPaths): void {
  if (p.orgConf !== undefined && existsSync(p.orgConf)) {
    console.log(`Org config:  ${tildefy(p.orgConf)}`);
  }
}

export function runStatus(
  paths: ConfigPaths | string = resolveConfigPaths(),
  tuHome: string = TU_HOME,
  now: Date = new Date(),
  defaultsPath: string = DEFAULT_CONFIG_PATH,
): void {
  const p = normalizePaths(paths);
  const orgExists = p.orgConf !== undefined && existsSync(p.orgConf);
  // Mirror the readConfig selection rule exactly: a file counts as selected
  // only when it actually reads (an existing-but-unreadable file falls back,
  // same as readConfig). The legacy fallback's deprecation warning goes to
  // stderr (emitted at most once per process).
  const selected = selectUserConfPath(p);

  // No config file at all: the original no-config line. An org-only setup
  // falls through to the full layout (with the Config: line omitted), so the
  // teammate sees the resulting mode.
  if (selected === null && !orgExists) {
    console.log(`Mode:        single (no ${tildefy(p.userConf)})`);
    return;
  }

  const config = readConfig(p, defaultsPath);
  const configLine = selected !== null ? `Config:      ${tildefy(selected)} (v${config.version})` : null;

  if (config.mode !== "multi") {
    console.log("Mode:        single");
    if (configLine !== null) console.log(configLine);
    printOrgLine(p);
    return;
  }

  const metricsExists = existsSync(config.metricsDir);
  const metricsLine = metricsExists
    ? tildefy(config.metricsDir)
    : `${tildefy(config.metricsDir)} (NOT FOUND — run 'tu init-metrics')`;

  console.log("Mode:        multi");
  console.log(`User:        ${config.user}`);
  console.log(`Machine:     ${config.machine}`);
  if (configLine !== null) console.log(configLine);
  printOrgLine(p);
  console.log(`Metrics:     ${metricsLine}`);
  console.log(`Last sync:   ${formatLastSync(tuHome, now)}`);
  console.log(`Auto-sync:   ${config.autoSync ? "on" : "off"}`);
}

// `tu help-dump` — emit the frozen shll.ai contract JSON on STDOUT only.
//
// shll.ai's pull cron runs this against the brew-installed binary, captures
// stdout, and treats ANY stderr on an otherwise-successful run as contract
// drift — so this writes nothing to stderr and exits 0. The raw `--help` text
// is FULL_HELP + "\n", byte-for-byte identical to what `tu --help` prints
// (console.log appends the newline). name/description/version are embedded at
// build time (the bottle ships no package.json). This is a build/interop
// artifact, not a runtime data path, so it does not follow Constitution II
// graceful degradation; it is deterministic and side-effect-free.
export function runHelpDump(): void {
  const doc = buildHelpDoc({
    name: PKG_NAME,
    version: PKG_VERSION,
    description: PKG_DESCRIPTION,
    helpText: FULL_HELP + "\n",
  });
  process.stdout.write(JSON.stringify(doc, null, 2) + "\n");
}

// `tu skill` — print the agent usage bundle to STDOUT, byte-identical to
// docs/site/skill.md. Static content, no rendering/pager/framing: stdout is the
// bundle, stderr is empty, exit 0. See skill.ts for how SKILL_MD is resolved.
export function runSkill(): void {
  process.stdout.write(SKILL_MD);
}

const SHELL_INIT_USAGE = `Usage: tu shell-init <bash|zsh|fish>

Install:
  bash: echo 'eval "$(tu shell-init bash)"' >> ~/.bashrc
  zsh:  echo 'eval "$(tu shell-init zsh)"' >> ~/.zshrc
  fish: tu shell-init fish > ~/.config/fish/completions/tu.fish`;

export function runShellInit(shell: string | undefined): void {
  if (shell === undefined) {
    // Toolkit `shell-init` standard: a missing shell arg is a usage error —
    // usage on stderr, exit 2, stdout EMPTY. stdout may be eval'd by shells
    // (`eval "$(tu shell-init …)"`), so usage text must never reach it.
    console.error(SHELL_INIT_USAGE);
    process.exit(EXIT_USAGE);
    return; // unreachable at runtime; keeps mocked-exit unit tests from falling through
  }
  switch (shell) {
    case "bash":
      process.stdout.write(BASH_COMPLETION);
      return;
    case "zsh":
      process.stdout.write(ZSH_COMPLETION);
      return;
    case "fish":
      process.stdout.write(FISH_COMPLETION);
      return;
    default:
      console.error(`Unknown shell: ${shell}. Supported: bash, zsh, fish`);
      process.exit(EXIT_USAGE);
  }
}

export function runUpdate(skipBrewUpdate = false): void {
  if (!__cli_dirname.includes("/Cellar/tu/")) {
    console.log(`tu v${PKG_VERSION} was not installed via Homebrew.`);
    console.log("Update manually, or reinstall with: brew install sahil87/tap/tu");
    return;
  }

  console.log(`Current version: v${PKG_VERSION}`);

  if (!skipBrewUpdate) {
    try {
      // Generous bound sized for a network transfer (toolkit `update` standard
      // SHOULD) — piped call, so a bound guards against a silent hang; Node's
      // default SIGTERM killSignal terminates gracefully.
      execSync("brew update --quiet", { stdio: "pipe", timeout: 600_000 });
    } catch {
      console.error("Error: could not check for updates (brew update failed). Check your network connection.");
      process.exit(1);
    }
  }

  let latest: string;
  try {
    const infoRaw = execSync("brew info --json=v2 tu", { stdio: "pipe", timeout: 60_000 });
    const info = JSON.parse(infoRaw.toString());
    const stable = info?.formulae?.[0]?.versions?.stable;
    if (typeof stable !== "string" || stable.trim() === "") {
      throw new Error("Invalid stable version in brew info output");
    }
    latest = stable;
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
    // NO timeout here (toolkit `update` standard MUST NOT): killing brew
    // mid-transaction corrupts the keg mid-swap. The call is interactive
    // (stdio: "inherit") — the user can Ctrl-C a genuinely stuck upgrade.
    // HOMEBREW_NO_ASK=1 disables Homebrew 6's default ask mode (the
    // "Do you want to proceed with the upgrade? [y/n]" prompt fires when
    // both stdio fds are TTYs, and would otherwise block the update).
    // The env var is used instead of the `--no-ask` flag because Homebrew
    // < 6 doesn't know the flag and would error, while an unrecognized
    // env var is harmlessly ignored — cross-version safe.
    execSync("brew upgrade tu", { stdio: "inherit", env: { ...process.env, HOMEBREW_NO_ASK: "1" } });
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

// Format a dry-run sync report as the human-readable preview printed to stdout.
// The preview shows would-write files (new vs. update: X → Y), would-skip files
// (never-shrink guard, incoming < existing), the would-be commit + the pull/push
// that would follow, and a closing line stating nothing was mutated.
export function formatDrySyncReport(report: DrySyncReport): string {
  const fmt = (n: number): string => `$${n.toFixed(2)}`;
  const userPrefix = join(report.metricsDir, report.user);
  const dir = tildefy(userPrefix) + "/";
  const writes: string[] = [];
  const skips: string[] = [];
  for (const tool of report.tools) {
    for (const d of tool.decisions) {
      const name = d.filePath.startsWith(userPrefix)
        ? d.filePath.slice(userPrefix.length + 1)
        : d.filePath;
      if (d.action === "write") {
        const note = d.existingCost !== undefined ? `(update: ${fmt(d.existingCost)} → ${fmt(d.incomingCost)})` : "(new)";
        writes.push(`  ${name}  ${fmt(d.incomingCost)}  ${note}`);
      } else {
        skips.push(`  ${name}  incoming ${fmt(d.incomingCost)} < existing ${fmt(d.existingCost ?? 0)}`);
      }
    }
  }

  const lines: string[] = [];
  if (writes.length > 0) {
    lines.push(`Would write ${writes.length} day-file(s) under ${dir}:`);
    lines.push(...writes);
  } else {
    lines.push(`Would write 0 day-file(s) under ${dir}.`);
  }
  if (skips.length > 0) {
    lines.push(`Would skip ${skips.length} file(s) (never-shrink guard):`);
    lines.push(...skips);
  }
  if (report.wouldCommit) {
    lines.push(`Would commit: "${report.commitMessage}", then pull --rebase origin main, then push`);
  } else {
    lines.push("Would commit: nothing (no changes), then pull --rebase origin main, then push");
  }
  lines.push("Dry run — nothing written, committed, or pushed.");
  return lines.join("\n");
}

export async function runSync(
  paths: ConfigPaths | string = resolveConfigPaths(),
  tuHome: string = TU_HOME,
  defaultsPath: string = DEFAULT_CONFIG_PATH,
  dryRun = false,
): Promise<void> {
  const config = readConfig(paths, defaultsPath);
  // `tu sync` is the other path that writes day-files, so the reserved-user
  // guard runs here too — otherwise `user = all` would create {metricsDir}/all/.
  assertUserNotReserved(config);
  if (config.mode !== "multi") {
    console.error(
      "tu sync requires metrics_repo to be set.\nAdd metrics_repo to ~/.config/tu/tu.conf, run 'tu init-metrics <repo-url>', or set TU_METRICS_REPO.",
    );
    process.exit(1);
  }
  const guardedConfig = checkMetricsDirGuard(config, tuHome);
  if (guardedConfig.mode !== "multi") {
    // Auto-clone failed or metricsDir still missing
    process.exit(1);
  }

  // Dry-run: compute the preview without mutating the working tree, the metrics
  // repo, or the network, and print it to stdout (exit 0). The config/mode +
  // metrics-dir guards above run identically to a live sync, so a dry-run in
  // single mode fails exactly like a live `tu sync`.
  if (dryRun) {
    const report = await fullSync(guardedConfig, tuHome, true);
    console.log(formatDrySyncReport(report));
    return;
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
  since?: string,
  until?: string,
): Promise<UsageEntry[]> {
  if (targetUser === ALL_USERS) {
    // Repo-only, like -u <other-user>: every profile's day-files summed per
    // label. A plain sum is exact here — day-files are never-shrink high-water
    // marks and there is no live view to reconcile, so no maxMergeEntries.
    _mark(`fetchToolMerged(${toolKey}) → readRemote for all users`);
    const summed = readAllUsersByUser(config.metricsDir, toolKey);
    const entries: UsageEntry[] = [];
    for (const userEntries of summed.values()) entries.push(...userEntries);
    const merged = filterEntriesByRange(mergeEntries(entries, []), since, until);
    _mark(`fetchToolMerged(${toolKey}) → readRemote done (${merged.length} entries)`);
    return aggregateForPeriod(period, merged);
  }
  if (targetUser && targetUser !== config.user) {
    _mark(`fetchToolMerged(${toolKey}) → readRemote for ${targetUser}`);
    const entries = filterEntriesByRange(readRemoteEntries(config.metricsDir, targetUser, null, toolKey), since, until);
    _mark(`fetchToolMerged(${toolKey}) → readRemote done (${entries.length} entries)`);
    return aggregateForPeriod(period, entries);
  }
  _mark(`fetchToolMerged(${toolKey}) → fetchHistory`);
  const local = await fetchHistory(toolKey, "daily", extra, skipCache);
  _mark(`fetchToolMerged(${toolKey}) → fetchHistory done (${local.length} entries)`);
  writeMetrics(config.metricsDir, config.user, config.machine, toolKey, local);
  _mark(`fetchToolMerged(${toolKey}) → writeMetrics done`);
  // Read ALL machines (excludeMachine = null) in one walk, then split out this
  // machine's own snapshots: once Claude Code purges old transcripts, the live
  // fetch under-reports old days, so the machine's own synced history must be
  // merged back via per-day whole-entry max (sum would double-count the
  // surviving transcripts of partially-purged days).
  const byMachine = readRemoteEntriesByMachine(config.metricsDir, config.user, null, toolKey);
  const ownSnapshots = byMachine.get(config.machine) ?? [];
  const remote: UsageEntry[] = [];
  for (const [machine, machineEntries] of byMachine) {
    if (machine !== config.machine) remote.push(...machineEntries);
  }
  _mark(`fetchToolMerged(${toolKey}) → readRemote done (${remote.length} entries)`);
  const effectiveLocal = maxMergeEntries(local, ownSnapshots);
  const merged = filterEntriesByRange(mergeEntries(effectiveLocal, remote), since, until);
  return aggregateForPeriod(period, merged);
}

interface MergedResult {
  entries: UsageEntry[];
  machineMap: Map<string, UsageEntry[]>;
}

// Apply the --since/--until window to every machine's entries, so both the
// flattened merge and the per-machine --by-machine columns stay in-window.
function filterMachineMap(
  machineMap: Map<string, UsageEntry[]>,
  since?: string,
  until?: string,
): Map<string, UsageEntry[]> {
  if (since === undefined && until === undefined) return machineMap;
  const out = new Map<string, UsageEntry[]>();
  for (const [machine, entries] of machineMap) out.set(machine, filterEntriesByRange(entries, since, until));
  return out;
}

// Every user profile's repo entries (all machines flattened), keyed by user.
// The -u all breakdown reuses the machine-column rendering with users as the
// column keys.
function readAllUsersByUser(metricsDir: string, toolKey: string): Map<string, UsageEntry[]> {
  const byUser = new Map<string, UsageEntry[]>();
  for (const user of listUsers(metricsDir)) byUser.set(user, readRemoteEntries(metricsDir, user, null, toolKey));
  return byUser;
}

// Window a per-key map (machine or user → daily entries), flatten it into one
// summed series, and aggregate both to the requested period.
function aggregateMachineMap(machineMap: Map<string, UsageEntry[]>, period: string, since?: string, until?: string): MergedResult {
  const windowed = filterMachineMap(machineMap, since, until);
  const entries: UsageEntry[] = [];
  for (const mEntries of windowed.values()) entries.push(...mEntries);
  const merged = mergeEntries(entries, []);
  if (period !== "daily") {
    const aggregatedEntries = aggregateForPeriod(period, merged);
    const aggregatedMap = new Map<string, UsageEntry[]>();
    for (const [key, mEntries] of windowed) aggregatedMap.set(key, aggregateForPeriod(period, mEntries));
    return { entries: aggregatedEntries, machineMap: aggregatedMap };
  }
  return { entries: merged, machineMap: windowed };
}

async function fetchToolMergedWithMachines(
  config: TuConfig,
  toolKey: string,
  period: string,
  extra: string[],
  skipCache = false,
  targetUser?: string,
  since?: string,
  until?: string,
): Promise<MergedResult> {
  if (targetUser === ALL_USERS) {
    _mark(`fetchToolMergedWithMachines(${toolKey}) → readRemote for all users`);
    return aggregateMachineMap(readAllUsersByUser(config.metricsDir, toolKey), period, since, until);
  }
  if (targetUser && targetUser !== config.user) {
    _mark(`fetchToolMergedWithMachines(${toolKey}) → readRemoteByMachine for ${targetUser}`);
    return aggregateMachineMap(readRemoteEntriesByMachine(config.metricsDir, targetUser, null, toolKey), period, since, until);
  }

  _mark(`fetchToolMergedWithMachines(${toolKey}) → fetchHistory`);
  const local = await fetchHistory(toolKey, "daily", extra, skipCache);
  _mark(`fetchToolMergedWithMachines(${toolKey}) → fetchHistory done`);

  const machineMap = new Map<string, UsageEntry[]>();
  machineMap.set(config.machine, local);

  if (config.mode === "multi") {
    writeMetrics(config.metricsDir, config.user, config.machine, toolKey, local);
    // Same self-view correction as fetchToolMerged: read all machines in one
    // walk and max-merge this machine's own snapshots into its live view, so
    // the own-machine column resurfaces purge-collapsed days.
    const allMachines = readRemoteEntriesByMachine(config.metricsDir, config.user, null, toolKey);
    machineMap.set(config.machine, maxMergeEntries(local, allMachines.get(config.machine) ?? []));
    for (const [machine, mEntries] of allMachines) {
      if (machine !== config.machine) machineMap.set(machine, mEntries);
    }
  }

  // Window each machine's entries so the flattened view and the per-machine
  // --by-machine columns both reflect the window (after max-merge, so
  // purge-corrected days are still windowed).
  return aggregateMachineMap(machineMap, period, since, until);
}

function emitJson(data: unknown): void {
  const obj = data instanceof Map ? Object.fromEntries(data) : data;
  console.log(JSON.stringify(obj, null, 2));
}

// Cost tracking for watch mode — set by dispatch functions, read by getCost/getPrevCosts callbacks
let _lastRenderCost = 0;
let _lastRenderCostMap = new Map<string, number>();
let _lastRenderTotalTokens = 0;

export type OutputFormat = "table" | "json" | "csv" | "md";

export interface GlobalFlags {
  outputFormat: OutputFormat;
  // jsonFlag retained for downstream callers and test compatibility during transition.
  jsonFlag: boolean;
  syncFlag: boolean;
  dryRunFlag: boolean; // --dry-run: parsed globally, honored only by `tu sync`
  freshFlag: boolean;
  watchFlag: boolean;
  watchInterval: number;
  noColorFlag: boolean;
  noRainFlag: boolean;
  userFlag: string | undefined;
  byMachineFlag: boolean;
  sinceFlag: string | undefined; // normalized ISO YYYY-MM-DD
  untilFlag: string | undefined; // normalized ISO YYYY-MM-DD
  fullFlag: boolean; // --full: disable the implicit 3-month cap on daily/weekly history
  metricFlag: BarMetric; // --metric: history bar scale — "cost" (default) or "tokens"
  filteredArgs: string[];
}

// Implicit 3-month cap floor: first day of the local month two calendar months
// back, so the window covers 3 calendar months INCLUDING the current month
// (e.g. 2026-07-17 → "2026-05-01": May, June, July). Uses local date methods
// (usage labels are local-day based), and Date normalizes month underflow, so
// year rollover is handled (2026-01-15 → "2025-11-01"). Returned as an ISO
// YYYY-MM-01 string suitable as a defaulted sinceFlag.
export function threeMonthFloor(now: Date = new Date()): string {
  const floor = new Date(now.getFullYear(), now.getMonth() - 2, 1);
  const y = floor.getFullYear();
  const m = String(floor.getMonth() + 1).padStart(2, "0");
  return `${y}-${m}-01`;
}

// Whether the implicit 3-month cap applies for the given display context.
// The cap engages only on daily/weekly history (never snapshot, never monthly
// history) and is disabled by an explicit --since/--until window or --full.
// Pure predicate mirroring the guard in main(); extracted so the injection
// decision is unit-testable without invoking the full main() pipeline.
export function capApplies(
  display: string,
  period: string,
  sinceFlag: string | undefined,
  untilFlag: string | undefined,
  fullFlag: boolean,
): boolean {
  return display === "history" && period !== "monthly" && sinceFlag === undefined && untilFlag === undefined && !fullFlag;
}

// Accepts YYYY-MM-DD or YYYYMMDD (consistent-dash shapes only); returns the
// normalized ISO string YYYY-MM-DD, or undefined when the shape is invalid.
// Shape-only — no calendar validity check (an impossible-but-shaped date like
// 2026-13-01 normalizes and simply yields an empty window downstream).
function normalizeDateFlag(value: string): string | undefined {
  if (/^\d{4}-\d{2}-\d{2}$/.test(value)) return value;
  if (/^\d{8}$/.test(value)) return `${value.slice(0, 4)}-${value.slice(4, 6)}-${value.slice(6, 8)}`;
  return undefined;
}

export function parseGlobalFlags(rawArgs: string[]): GlobalFlags {
  const jsonFlag = rawArgs.includes("--json") || rawArgs.includes("-j");
  const csvFlag = rawArgs.includes("--csv");
  const mdFlag = rawArgs.includes("--md");
  const syncFlag = rawArgs.includes("--sync");
  const dryRunFlag = rawArgs.includes("--dry-run");
  const freshFlag = rawArgs.includes("--fresh") || rawArgs.includes("-f");
  const watchFlag = rawArgs.includes("--watch") || rawArgs.includes("-w");
  const noColorFlag = rawArgs.includes("--no-color");
  const noRainFlag = rawArgs.includes("--no-rain");
  const byMachineFlag = rawArgs.includes("--by-machine");
  const fullFlag = rawArgs.includes("--full");

  let watchInterval = 10;
  let hasIntervalFlag = false;
  let rawIntervalVal: string | undefined;
  let userFlag: string | undefined;
  let hasUserFlag = false;
  let sinceFlag: string | undefined;
  let hasSinceFlag = false;
  let rawSinceVal: string | undefined;
  let untilFlag: string | undefined;
  let hasUntilFlag = false;
  let rawUntilVal: string | undefined;
  let metricFlag: BarMetric = "cost";
  let hasMetricFlag = false;
  let rawMetricVal: string | undefined;
  const filteredArgs: string[] = [];
  for (let i = 0; i < rawArgs.length; i++) {
    const a = rawArgs[i];
    if (a === "--json" || a === "-j" || a === "--csv" || a === "--md" || a === "--sync" || a === "--dry-run" || a === "--fresh" || a === "-f" || a === "--watch" || a === "-w" || a === "--no-color" || a === "--no-rain" || a === "--by-machine" || a === "--full" || a === "--skip-brew-update") continue;
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
    if (a === "--since" || a === "-s") {
      hasSinceFlag = true;
      const next = rawArgs[i + 1];
      if (next !== undefined && !next.startsWith("-")) {
        rawSinceVal = next;
        i++;
      }
      continue;
    }
    if (a === "--until") {
      hasUntilFlag = true;
      const next = rawArgs[i + 1];
      if (next !== undefined && !next.startsWith("-")) {
        rawUntilVal = next;
        i++;
      }
      continue;
    }
    if (a === "--metric") {
      hasMetricFlag = true;
      const next = rawArgs[i + 1];
      if (next !== undefined && !next.startsWith("-")) {
        rawMetricVal = next;
        i++;
      }
      continue;
    }
    filteredArgs.push(a);
  }

  if (watchFlag && hasIntervalFlag) {
    if (rawIntervalVal === undefined) {
      console.error("Error: --interval requires a numeric value");
      process.exit(EXIT_USAGE);
    }
    const num = Number(rawIntervalVal);
    if (num < 5) {
      console.error("Error: --interval minimum is 5 seconds");
      process.exit(EXIT_USAGE);
    }
    if (num > 3600) {
      console.error("Error: --interval maximum is 3600 seconds");
      process.exit(EXIT_USAGE);
    }
    watchInterval = num;
  }

  // Output-format flags are mutually exclusive. Existing --watch + --json rejection
  // keeps its original wording; --watch + --csv/--md follow the same pattern.
  if (watchFlag && jsonFlag) {
    console.error("Error: --watch and --json are incompatible");
    process.exit(EXIT_USAGE);
  }
  if (jsonFlag && csvFlag) {
    console.error("Error: --json and --csv are incompatible");
    process.exit(EXIT_USAGE);
  }
  if (jsonFlag && mdFlag) {
    console.error("Error: --json and --md are incompatible");
    process.exit(EXIT_USAGE);
  }
  if (csvFlag && mdFlag) {
    console.error("Error: --csv and --md are incompatible");
    process.exit(EXIT_USAGE);
  }
  if (watchFlag && csvFlag) {
    console.error("Error: --watch and --csv are incompatible");
    process.exit(EXIT_USAGE);
  }
  if (watchFlag && mdFlag) {
    console.error("Error: --watch and --md are incompatible");
    process.exit(EXIT_USAGE);
  }

  if (hasUserFlag && userFlag === undefined) {
    console.error("Error: -u requires a username");
    process.exit(EXIT_USAGE);
  }

  if (hasSinceFlag) {
    sinceFlag = rawSinceVal !== undefined ? normalizeDateFlag(rawSinceVal) : undefined;
    if (sinceFlag === undefined) {
      console.error("Error: --since requires a date (YYYY-MM-DD or YYYYMMDD)");
      process.exit(EXIT_USAGE);
    }
  }
  if (hasUntilFlag) {
    untilFlag = rawUntilVal !== undefined ? normalizeDateFlag(rawUntilVal) : undefined;
    if (untilFlag === undefined) {
      console.error("Error: --until requires a date (YYYY-MM-DD or YYYYMMDD)");
      process.exit(EXIT_USAGE);
    }
  }
  if (sinceFlag !== undefined && untilFlag !== undefined && sinceFlag > untilFlag) {
    console.error("Error: --since must be on or before --until");
    process.exit(EXIT_USAGE);
  }
  if (hasMetricFlag) {
    if (rawMetricVal !== "tokens" && rawMetricVal !== "cost") {
      console.error("Error: --metric requires 'tokens' or 'cost'");
      process.exit(EXIT_USAGE);
    }
    metricFlag = rawMetricVal;
  }

  let outputFormat: OutputFormat = "table";
  if (jsonFlag) outputFormat = "json";
  else if (csvFlag) outputFormat = "csv";
  else if (mdFlag) outputFormat = "md";

  return { outputFormat, jsonFlag, syncFlag, dryRunFlag, freshFlag, watchFlag, watchInterval, noColorFlag, noRainFlag, userFlag, byMachineFlag, sinceFlag, untilFlag, fullFlag, metricFlag, filteredArgs };
}

const KNOWN_SOURCES = new Set(["cc", "codex", "co", "oc", "gemini", "gem", "copilot", "cop", "kimi", "ki", "all"]);
const SOURCE_ALIASES: Record<string, string> = { co: "codex", gem: "gemini", cop: "copilot", ki: "kimi" };

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
    } else if (arg === "w" || arg === "weekly") {
      period = "weekly";
    } else if (arg === "m" || arg === "monthly") {
      period = "monthly";
    } else if (arg === "h" || arg === "history") {
      display = "history";
    } else if (arg === "dh") {
      period = "daily";
      display = "history";
    } else if (arg === "wh") {
      period = "weekly";
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

function renderTotalHistoryByFormat(
  outputFormat: OutputFormat,
  period: string,
  data: Map<string, UsageEntry[]>,
  fmtOpts?: FormatOptions,
): void {
  switch (outputFormat) {
    case "json": emitJson(data); break;
    case "csv": emitCsv(data, "total-history", { period }); break;
    case "md": emitMarkdown(data, "total-history", { period, capActive: fmtOpts?.capActive }); break;
    default: printTotalHistory(period, data, undefined, fmtOpts);
  }
}

async function dispatchAllHistory(config: TuConfig, period: string, outputFormat: OutputFormat, skipCache = false, fmtOpts?: FormatOptions, targetUser?: string, since?: string, until?: string): Promise<void> {
  if (config.mode === "multi") {
    const toolKeys = Object.keys(TOOLS);
    const allMerged = await Promise.all(toolKeys.map((k) => fetchToolMerged(config, k, period, [], skipCache, targetUser, since, until)));
    const mergedMap = new Map<string, UsageEntry[]>();
    for (let i = 0; i < toolKeys.length; i++) {
      mergedMap.set(TOOLS[toolKeys[i]].name, allMerged[i]);
    }
    renderTotalHistoryByFormat(outputFormat, period, mergedMap, fmtOpts);
    _lastRenderCost = sumAllToolCosts(mergedMap);
    _lastRenderCostMap = buildCostMap(mergedMap);
  } else {
    const results = await fetchAllHistory("daily", [], skipCache);
    if (period !== "daily") {
      const aggregated = new Map<string, UsageEntry[]>();
      for (const [name, entries] of results) {
        aggregated.set(name, aggregateForPeriod(period, filterEntriesByRange(entries, since, until)));
      }
      renderTotalHistoryByFormat(outputFormat, period, aggregated, fmtOpts);
      _lastRenderCost = sumAllToolCosts(aggregated);
      _lastRenderCostMap = buildCostMap(aggregated);
    } else {
      const filtered = new Map<string, UsageEntry[]>();
      for (const [name, entries] of results) {
        filtered.set(name, filterEntriesByRange(entries, since, until));
      }
      renderTotalHistoryByFormat(outputFormat, period, filtered, fmtOpts);
      _lastRenderCost = sumAllToolCosts(filtered);
      _lastRenderCostMap = buildCostMap(filtered);
    }
  }
}

function renderSnapshotByFormat(
  outputFormat: OutputFormat,
  period: string,
  data: Map<string, UsageTotals>,
  fmtOpts?: FormatOptions,
  machineCosts?: Map<string, Map<string, number>>,
): void {
  const opts: FormatOptions = machineCosts ? { ...fmtOpts, machineCosts } : (fmtOpts ?? {});
  switch (outputFormat) {
    case "json": emitJson(machineCosts ? attachMachinesJson(data, machineCosts) : data); break;
    case "csv": emitCsv(data, "snapshot", { period, machineCosts }); break;
    case "md": emitMarkdown(data, "snapshot", { period, machineCosts }); break;
    default: printTotal(period, data, opts);
  }
}

async function dispatchAllSnapshot(config: TuConfig, period: string, outputFormat: OutputFormat, skipCache = false, fmtOpts?: FormatOptions, targetUser?: string, byMachine = false): Promise<void> {
  if (byMachine) {
    const toolKeys = Object.keys(TOOLS);
    const allResults = await Promise.all(toolKeys.map((k) => fetchToolMergedWithMachines(config, k, period, [], skipCache, targetUser)));
    const result = new Map<string, UsageTotals>();
    for (let i = 0; i < toolKeys.length; i++) {
      const current = allResults[i].entries.find((e) => e.label === currentLabel(period));
      result.set(TOOLS[toolKeys[i]].name, current ?? { ...EMPTY });
    }
    const machineCosts = buildSnapshotMachineCosts(toolKeys, allResults, period);
    renderSnapshotByFormat(outputFormat, period, result, fmtOpts, machineCosts);
    _lastRenderCost = sumToolTotalsCost(result);
    _lastRenderCostMap = buildCostMap(result);
    return;
  }

  if (config.mode === "multi") {
    const toolKeys = Object.keys(TOOLS);
    const allMerged = await Promise.all(toolKeys.map((k) => fetchToolMerged(config, k, period, [], skipCache, targetUser)));
    const result = new Map<string, UsageTotals>();
    for (let i = 0; i < toolKeys.length; i++) {
      const current = allMerged[i].find((e) => e.label === currentLabel(period));
      result.set(TOOLS[toolKeys[i]].name, current ?? { ...EMPTY });
    }
    renderSnapshotByFormat(outputFormat, period, result, fmtOpts);
    _lastRenderCost = sumToolTotalsCost(result);
    _lastRenderCostMap = buildCostMap(result);
  } else {
    if (period !== "daily") {
      const toolKeys = Object.keys(TOOLS);
      const allDaily = await Promise.all(toolKeys.map((k) => fetchHistory(k, "daily", [], skipCache)));
      const result = new Map<string, UsageTotals>();
      const target = currentLabel(period);
      for (let i = 0; i < toolKeys.length; i++) {
        const aggregated = aggregateForPeriod(period, allDaily[i]);
        const match = aggregated.find((m) => m.label === target);
        result.set(TOOLS[toolKeys[i]].name, match ?? { ...EMPTY });
      }
      renderSnapshotByFormat(outputFormat, period, result, fmtOpts);
      _lastRenderCost = sumToolTotalsCost(result);
      _lastRenderCostMap = buildCostMap(result);
    } else {
      const results = await fetchAllTotals([]);
      renderSnapshotByFormat(outputFormat, period, results, fmtOpts);
      _lastRenderCost = sumToolTotalsCost(results);
      _lastRenderCostMap = buildCostMap(results);
    }
  }
}

function renderHistoryByFormat(
  outputFormat: OutputFormat,
  toolName: string,
  period: string,
  entries: UsageEntry[],
  fmtOpts?: FormatOptions,
  machineCosts?: Map<string, Map<string, number>>,
): void {
  const opts: FormatOptions = machineCosts ? { ...fmtOpts, machineCosts } : (fmtOpts ?? {});
  switch (outputFormat) {
    case "json": emitJson(machineCosts ? attachMachinesJson(entries, machineCosts) : entries); break;
    case "csv": emitCsv({ toolName, entries }, "history", { period, machineCosts }); break;
    case "md": emitMarkdown({ toolName, entries }, "history", { period, machineCosts, capActive: fmtOpts?.capActive }); break;
    default: printHistory(toolName, period, entries, undefined, opts);
  }
}

async function dispatchSingleTool(
  config: TuConfig, toolKey: string, period: string, display: string, outputFormat: OutputFormat, skipCache = false, fmtOpts?: FormatOptions, targetUser?: string, byMachine = false, since?: string, until?: string,
): Promise<void> {
  const toolCfg = TOOLS[toolKey];
  if (!toolCfg) {
    console.error(`Unknown tool: ${toolKey}`);
    // Usage hint is a diagnostic on an error path — stderr, not stdout
    // (toolkit principle №2: stdout is data, stderr is diagnostics).
    console.error(SHORT_USAGE);
    process.exit(EXIT_USAGE);
  }

  _mark(`fetching ${toolKey} ${period}`);

  if (byMachine) {
    const merged = await fetchToolMergedWithMachines(config, toolKey, period, [], skipCache, targetUser, since, until);
    _mark("fetch done (by-machine)");

    if (display === "history") {
      const machineCosts = buildHistoryMachineCosts(merged.machineMap);
      renderHistoryByFormat(outputFormat, toolCfg.name, period, merged.entries, fmtOpts, machineCosts);
      _lastRenderCost = merged.entries.reduce((sum, e) => sum + e.totalCost, 0);
      _lastRenderCostMap = buildCostMap(merged.entries, toolCfg.name);
    } else {
      const target = currentLabel(period);
      const current = merged.entries.find((e) => e.label === target);
      const result = new Map<string, UsageTotals>();
      result.set(toolCfg.name, current ?? { ...EMPTY });
      const machineCosts = new Map<string, Map<string, number>>();
      const toolMachines = new Map<string, number>();
      for (const [machine, entries] of merged.machineMap) {
        const match = entries.find((e) => e.label === target);
        toolMachines.set(machine, match ? match.totalCost : 0);
      }
      machineCosts.set(toolCfg.name, toolMachines);
      renderSnapshotByFormat(outputFormat, period, result, fmtOpts, machineCosts);
      _lastRenderCost = sumToolTotalsCost(result);
      _lastRenderCostMap = buildCostMap(result);
    }
    _mark("done");
    return;
  }

  let entries: UsageEntry[];
  if (config.mode === "multi") {
    entries = await fetchToolMerged(config, toolKey, period, [], skipCache, targetUser, since, until);
  } else {
    entries = filterEntriesByRange(await fetchHistory(toolKey, "daily", [], skipCache), since, until);
    entries = aggregateForPeriod(period, entries);
  }
  _mark("fetch done");

  if (display === "history") {
    renderHistoryByFormat(outputFormat, toolCfg.name, period, entries, fmtOpts);
    _lastRenderCost = entries.reduce((sum, e) => sum + e.totalCost, 0);
    _lastRenderCostMap = buildCostMap(entries, toolCfg.name);
  } else {
    const target = currentLabel(period);
    const current = entries.find((e) => e.label === target);
    const result = new Map<string, UsageTotals>();
    result.set(toolCfg.name, current ?? { ...EMPTY });
    renderSnapshotByFormat(outputFormat, period, result, fmtOpts);
    _lastRenderCost = sumToolTotalsCost(result);
    _lastRenderCostMap = buildCostMap(result);
  }
  _mark("done");
}

// Build machineCosts map for history: label → (machine → cost)
function buildHistoryMachineCosts(machineMap: Map<string, UsageEntry[]>): Map<string, Map<string, number>> {
  const result = new Map<string, Map<string, number>>();
  for (const [machine, entries] of machineMap) {
    for (const e of entries) {
      if (!result.has(e.label)) result.set(e.label, new Map());
      result.get(e.label)!.set(machine, (result.get(e.label)!.get(machine) ?? 0) + e.totalCost);
    }
  }
  return result;
}

// Build machineCosts map for snapshot: toolName → (machine → cost)
function buildSnapshotMachineCosts(
  toolKeys: string[],
  allResults: MergedResult[],
  period: string,
): Map<string, Map<string, number>> {
  const result = new Map<string, Map<string, number>>();
  const target = currentLabel(period);
  for (let i = 0; i < toolKeys.length; i++) {
    const toolName = TOOLS[toolKeys[i]].name;
    const machCosts = new Map<string, number>();
    for (const [machine, entries] of allResults[i].machineMap) {
      const match = entries.find((e) => e.label === target);
      if (match) machCosts.set(machine, match.totalCost);
    }
    result.set(toolName, machCosts);
  }
  return result;
}

// Attach machines breakdown to JSON data
function attachMachinesJson(data: unknown, machineCosts: Map<string, Map<string, number>>): unknown {
  if (data instanceof Map) {
    const obj: Record<string, unknown> = {};
    for (const [key, val] of data) {
      const mCosts = machineCosts.get(key);
      if (mCosts && mCosts.size > 0) {
        const machines: Record<string, number> = {};
        for (const [m, c] of mCosts) machines[m] = c;
        obj[key] = { ...(val as object), machines };
      } else {
        obj[key] = val;
      }
    }
    return obj;
  }
  if (Array.isArray(data)) {
    return data.map((item: UsageEntry) => {
      const mCosts = machineCosts.get(item.label);
      if (mCosts && mCosts.size > 0) {
        const machines: Record<string, number> = {};
        for (const [m, c] of mCosts) machines[m] = c;
        return { ...item, machines };
      }
      return item;
    });
  }
  return data;
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

async function dispatchAllHistoryLines(config: TuConfig, period: string, skipCache = false, fmtOpts?: FormatOptions, targetUser?: string, since?: string, until?: string): Promise<string[]> {
  if (config.mode === "multi") {
    const toolKeys = Object.keys(TOOLS);
    const allMerged = await Promise.all(toolKeys.map((k) => fetchToolMerged(config, k, period, [], skipCache, targetUser, since, until)));
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
    if (period !== "daily") {
      const aggregated = new Map<string, UsageEntry[]>();
      for (const [name, entries] of results) {
        aggregated.set(name, aggregateForPeriod(period, filterEntriesByRange(entries, since, until)));
      }
      _lastRenderCost = sumAllToolCosts(aggregated);
      _lastRenderCostMap = buildCostMap(aggregated);
      _lastRenderTotalTokens = sumAllToolTokens(aggregated);
      return renderTotalHistory(period, aggregated, undefined, fmtOpts);
    } else {
      const filtered = new Map<string, UsageEntry[]>();
      for (const [name, entries] of results) {
        filtered.set(name, filterEntriesByRange(entries, since, until));
      }
      _lastRenderCost = sumAllToolCosts(filtered);
      _lastRenderCostMap = buildCostMap(filtered);
      _lastRenderTotalTokens = sumAllToolTokens(filtered);
      return renderTotalHistory(period, filtered, undefined, fmtOpts);
    }
  }
}

async function dispatchAllSnapshotLines(config: TuConfig, period: string, skipCache = false, fmtOpts?: FormatOptions, targetUser?: string, byMachine = false, since?: string, until?: string): Promise<string[]> {
  if (byMachine) {
    const toolKeys = Object.keys(TOOLS);
    const allResults = await Promise.all(toolKeys.map((k) => fetchToolMergedWithMachines(config, k, period, [], skipCache, targetUser, since, until)));
    const result = new Map<string, UsageTotals>();
    for (let i = 0; i < toolKeys.length; i++) {
      const current = allResults[i].entries.find((e) => e.label === currentLabel(period));
      result.set(TOOLS[toolKeys[i]].name, current ?? { ...EMPTY });
    }
    const machineCosts = buildSnapshotMachineCosts(toolKeys, allResults, period);
    _lastRenderCost = sumToolTotalsCost(result);
    _lastRenderCostMap = buildCostMap(result);
    _lastRenderTotalTokens = sumToolTotalsTokens(result);
    return renderTotal(period, result, { ...fmtOpts, machineCosts });
  }

  if (config.mode === "multi") {
    const toolKeys = Object.keys(TOOLS);
    const allMerged = await Promise.all(toolKeys.map((k) => fetchToolMerged(config, k, period, [], skipCache, targetUser, since, until)));
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
    if (period !== "daily") {
      const toolKeys = Object.keys(TOOLS);
      const allDaily = await Promise.all(toolKeys.map((k) => fetchHistory(k, "daily", [], skipCache)));
      const result = new Map<string, UsageTotals>();
      const target = currentLabel(period);
      for (let i = 0; i < toolKeys.length; i++) {
        const aggregated = aggregateForPeriod(period, allDaily[i]);
        const match = aggregated.find((m) => m.label === target);
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
  config: TuConfig, toolKey: string, period: string, display: string, skipCache = false, fmtOpts?: FormatOptions, targetUser?: string, byMachine = false, since?: string, until?: string,
): Promise<string[]> {
  const toolCfg = TOOLS[toolKey];
  if (!toolCfg) return [`Unknown tool: ${toolKey}`];

  if (byMachine) {
    const merged = await fetchToolMergedWithMachines(config, toolKey, period, [], skipCache, targetUser, since, until);
    if (display === "history") {
      const machineCosts = buildHistoryMachineCosts(merged.machineMap);
      _lastRenderCost = merged.entries.reduce((sum, e) => sum + e.totalCost, 0);
      _lastRenderCostMap = buildCostMap(merged.entries, toolCfg.name);
      _lastRenderTotalTokens = merged.entries.reduce((sum, e) => sum + e.totalTokens, 0);
      return renderHistory(toolCfg.name, period, merged.entries, undefined, { ...fmtOpts, machineCosts });
    } else {
      const target = currentLabel(period);
      const current = merged.entries.find((e) => e.label === target);
      const result = new Map<string, UsageTotals>();
      result.set(toolCfg.name, current ?? { ...EMPTY });
      const machineCosts = new Map<string, Map<string, number>>();
      const toolMachines = new Map<string, number>();
      for (const [machine, entries] of merged.machineMap) {
        const match = entries.find((e) => e.label === target);
        toolMachines.set(machine, match ? match.totalCost : 0);
      }
      machineCosts.set(toolCfg.name, toolMachines);
      _lastRenderCost = sumToolTotalsCost(result);
      _lastRenderCostMap = buildCostMap(result);
      _lastRenderTotalTokens = sumToolTotalsTokens(result);
      return renderTotal(period, result, { ...fmtOpts, machineCosts });
    }
  }

  let entries: UsageEntry[];
  if (config.mode === "multi") {
    entries = await fetchToolMerged(config, toolKey, period, [], skipCache, targetUser, since, until);
  } else {
    entries = filterEntriesByRange(await fetchHistory(toolKey, "daily", [], skipCache), since, until);
    entries = aggregateForPeriod(period, entries);
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
  let { outputFormat, syncFlag, dryRunFlag, freshFlag, watchFlag, watchInterval, noColorFlag, noRainFlag, userFlag, byMachineFlag, sinceFlag, untilFlag, fullFlag, metricFlag, filteredArgs } = parseGlobalFlags(rawArgs);

  if (noColorFlag) setNoColor(true);

  if (rawArgs.includes("--version") || rawArgs.includes("-V") || rawArgs.includes("-v")) {
    const v = PKG_VERSION.startsWith("v") ? PKG_VERSION : `v${PKG_VERSION}`;
    console.log(`tu version ${v}`);
    return;
  }

  // Help — check first arg for help / -h / --help
  if (filteredArgs.length > 0 && (filteredArgs[0] === "help" || filteredArgs[0] === "-h" || filteredArgs[0] === "--help")) {
    console.log(FULL_HELP);
    return;
  }

  // --dry-run is honored ONLY by `tu sync`. Any other invocation carrying it
  // (e.g. `tu cc --dry-run`, `tu cc --sync --dry-run`, or bare `tu --dry-run`)
  // fails fast, exit 1. Rationale: the multi-mode fetch path writes day-files
  // outside the sync boundary on every data command, so a combined
  // preview-then-proceed would mutate the very files it just previewed — a
  // lying dry-run. Fail-fast is the honest contract; strict→loose is the
  // non-breaking direction if combined support is ever wanted. Silently
  // ignoring the flag is ruled out: a user who passed --dry-run must never get
  // a surprise mutation.
  if (dryRunFlag && filteredArgs[0] !== "sync") {
    console.error("Error: --dry-run is supported only with 'tu sync' — run 'tu sync --dry-run' to preview a sync.");
    process.exit(EXIT_USAGE);
  }

  // Non-data commands — dispatch before grammar parsing
  if (filteredArgs.length > 0) {
    const cmd = filteredArgs[0];
    if (cmd === "init-conf") { runInitConf(); return; }
    if (cmd === "init-metrics") {
      if (filteredArgs.length > 2) {
        console.error("Error: init-metrics takes at most one argument (repo-url)");
        console.error(SHORT_USAGE);
        process.exit(EXIT_USAGE);
      }
      runInitMetrics(resolveConfigPaths(), DEFAULT_CONFIG_PATH, TU_HOME, filteredArgs[1]);
      return;
    }
    if (cmd === "sync") { await runSync(resolveConfigPaths(), TU_HOME, DEFAULT_CONFIG_PATH, dryRunFlag); return; }
    if (cmd === "status") { runStatus(); return; }
    if (cmd === "update") {
      // Toolkit `update` standard: `tu update --help` MUST print help (advertising
      // the literal --skip-brew-update flag) instead of running a real update —
      // shll update's flag-discovery probe depends on it. Scoped to `update` only.
      if (rawArgs.includes("--help") || rawArgs.includes("-h")) { console.log(FULL_HELP); return; }
      runUpdate(process.argv.includes("--skip-brew-update"));
      return;
    }
    if (cmd === "shell-init") { runShellInit(filteredArgs[1]); return; }
    if (cmd === "help-dump") { runHelpDump(); return; }
    if (cmd === "skill") { runSkill(); return; }
  }

  // Parse positional data args (source, period, display)
  let parsed: DataArgs;
  try {
    parsed = parseDataArgs(filteredArgs);
  } catch (err: unknown) {
    console.error((err as Error).message);
    // Usage hint is a diagnostic on an error path — stderr, not stdout
    // (toolkit principle №2: stdout is data, stderr is diagnostics).
    console.error(SHORT_USAGE);
    process.exit(EXIT_USAGE);
  }
  const { source, period, display } = parsed;

  _mark("readConfig()");
  const config = checkMetricsDirGuard(readConfig());
  assertUserNotReserved(config);
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

  // --by-machine is incompatible with all-tools history pivot
  if (byMachineFlag && source === "all" && display === "history") {
    process.stderr.write("Warning: --by-machine is not supported with all-tools history — ignoring.\n");
    byMachineFlag = false;
  }

  // --since/--until apply to history display only. Warn once and clear them for
  // snapshot display (mirrors the -u / --by-machine warn-and-clear guards
  // above). Printing here — not inside dispatch — means watch snapshot warns
  // once at startup, not per poll.
  if ((sinceFlag !== undefined || untilFlag !== undefined) && display !== "history") {
    process.stderr.write("Warning: --since/--until apply to history display — ignoring.\n");
    sinceFlag = undefined;
    untilFlag = undefined;
  }

  // --full applies to daily/weekly history only. On a snapshot display it warns
  // once and is ignored (mirrors the since/until guard above, so watch snapshot
  // warns once at startup). On monthly history it is a silent vacuous no-op
  // (monthly is never capped — full history is already shown), and combined with
  // an explicit --since/--until it is silently accepted (both mean "no implicit
  // cap"; the explicit window still applies).
  if (fullFlag && display !== "history") {
    process.stderr.write("Warning: --full applies to daily/weekly history — ignoring.\n");
  }

  // --metric scales history bars only. A non-default value on a snapshot
  // display warns once and is cleared (same spot as the since/until guard, so
  // watch snapshot warns once at startup). JSON/CSV/MD have no bars and
  // ignore it silently.
  if (metricFlag !== "cost" && display !== "history") {
    process.stderr.write("Warning: --metric applies to history display — ignoring.\n");
    metricFlag = "cost";
  }

  // Implicit 3-month cap: daily/weekly history only, no explicit window, no
  // --full. Default sinceFlag to the floor so the cap reuses the existing
  // --since machinery (filterEntriesByRange) with zero new filtering logic;
  // capActive drives the "last 3 months" heading hint. An explicit --since or
  // --until disables the cap entirely (no intersection — otherwise a past
  // --until would silently empty the output).
  let capActive = false;
  if (capApplies(display, period, sinceFlag, untilFlag, fullFlag)) {
    sinceFlag = threeMonthFloor();
    capActive = true;
  }

  // Merge the flag-derived FormatOptions (capActive heading hint, bar metric,
  // and the "Users" legend when -u all --by-machine keys the breakdown columns
  // by user) into whatever options a dispatch path uses, without overriding a
  // caller's other options. Renderers that do not read a field pass it through
  // harmlessly. Nothing is stamped when every field is at its default, so
  // existing output stays byte-identical.
  const usersLegend = userFlag === ALL_USERS && byMachineFlag;
  const withCap = (fmtOpts?: FormatOptions): FormatOptions | undefined => {
    if (!capActive && metricFlag === "cost" && !usersLegend) return fmtOpts;
    return {
      ...fmtOpts,
      ...(capActive ? { capActive: true } : {}),
      ...(metricFlag !== "cost" ? { metric: metricFlag } : {}),
      ...(usersLegend ? { machineLegend: "Users" } : {}),
    };
  };

  if (watchFlag) {
    const action = async (skipCache: boolean, fmtOpts?: FormatOptions): Promise<string[]> => {
      const opts = withCap(fmtOpts);
      if (source === "all") {
        if (display === "history") { return dispatchAllHistoryLines(config, period, skipCache, opts, userFlag, sinceFlag, untilFlag); }
        else { return dispatchAllSnapshotLines(config, period, skipCache, opts, userFlag, byMachineFlag, sinceFlag, untilFlag); }
      } else {
        return dispatchSingleToolLines(config, source, period, display, skipCache, opts, userFlag, byMachineFlag, sinceFlag, untilFlag);
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
      if (display === "history") { await dispatchAllHistory(config, period, outputFormat, freshFlag, withCap(undefined), userFlag, sinceFlag, untilFlag); }
      else { await dispatchAllSnapshot(config, period, outputFormat, freshFlag, withCap(undefined), userFlag, byMachineFlag); }
    } else {
      await dispatchSingleTool(config, source, period, display, outputFormat, freshFlag, withCap(undefined), userFlag, byMachineFlag, sinceFlag, untilFlag);
    }
  }
}

main().catch((err) => {
  console.error(err.message);
  process.exit(1);
});
