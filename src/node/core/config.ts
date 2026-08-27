import { readFileSync } from "node:fs";
import { homedir, hostname, userInfo } from "node:os";
import { resolve, dirname, join } from "node:path";
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

export const TOOL_NAME = "tu";
export const CONFIG_FILE = "tu.conf";
export const ORG_CONFIG_FILE = "org.conf";
export const LEGACY_CONFIG_FILE = ".tu.conf";

export interface ConfigPaths {
  configDir: string;   // $HOME/.config/tu
  userConf: string;    // $HOME/.config/tu/tu.conf
  orgConf?: string;    // $HOME/.config/tu/org.conf (absent when constructed from a bare user-conf path)
  legacyConf?: string; // $HOME/.tu.conf (absent when constructed from a bare user-conf path)
}

// Thrown by resolveConfigPaths when $HOME is unset/empty — an actionable,
// environment-level error (shll config-home standard; principle №4).
export class ConfigHomeError extends Error {}

// Built from $HOME and nothing else — no $XDG_CONFIG_HOME, no TU_* override,
// no os.homedir() passwd fallback (shll config-home standard). Lazy by design:
// only config-reading commands call this, so `tu --version`/`help`/`help-dump`/
// `skill`/`shell-init`/`update` keep working with $HOME unset.
export function resolveConfigPaths(env: NodeJS.ProcessEnv = process.env): ConfigPaths {
  const home = env.HOME;
  if (home === undefined || home === "") {
    throw new ConfigHomeError("tu: $HOME is not set; cannot locate config");
  }
  const configDir = join(home, ".config", TOOL_NAME);
  return {
    configDir,
    userConf: join(configDir, CONFIG_FILE),
    orgConf: join(configDir, ORG_CONFIG_FILE),
    legacyConf: join(home, LEGACY_CONFIG_FILE),
  };
}

const __dirname = dirname(fileURLToPath(import.meta.url));

// Locate tu.default.conf: check alongside the running script first (bundled),
// then walk up to project root (dev/test)
function findDefaultConf(): string {
  // Bundled: tu.default.conf next to tu.mjs
  const beside = resolve(__dirname, "tu.default.conf");
  try { readFileSync(beside); return beside; } catch {}
  // Dev: walk up to package.json root
  let dir = __dirname;
  while (dir !== dirname(dir)) {
    const candidate = resolve(dir, "tu.default.conf");
    try { readFileSync(candidate); return candidate; } catch {}
    dir = dirname(dir);
  }
  return beside; // fallback — readConfFile handles missing gracefully
}

export const DEFAULT_CONFIG_PATH = findDefaultConf();

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

// The deprecation warning fires at most once per process per legacy path
// (readConfig is called from several paths and watch mode re-reads).
const legacyWarningEmitted = new Set<string>();

// User-conf selection rule: prefer ~/.config/tu/tu.conf; fall back to legacy
// ~/.tu.conf with a one-line stderr deprecation warning; never auto-migrate.
// Returns the parsed fields plus the path actually read (for warnings/status).
function selectUserConf(paths: ConfigPaths): { conf: Record<string, string>; path: string | null } {
  const user = readConfFile(paths.userConf);
  if (user !== null) return { conf: user, path: paths.userConf };
  if (paths.legacyConf !== undefined) {
    const legacy = readConfFile(paths.legacyConf);
    if (legacy !== null) {
      if (!legacyWarningEmitted.has(paths.legacyConf)) {
        legacyWarningEmitted.add(paths.legacyConf);
        console.error(`tu: ~/.tu.conf is deprecated; move it to ~/.config/tu/tu.conf`);
      }
      return { conf: legacy, path: paths.legacyConf };
    }
  }
  return { conf: {}, path: null };
}

// The user-conf path readConfig will actually read (new conf wins, legacy is
// the fallback, null when neither reads) — exported so `tu status` reports the
// same selection readConfig makes. An existsSync-based check would misreport
// an existing-but-unreadable file as selected while readConfig falls back.
export function selectUserConfPath(paths: ConfigPaths): string | null {
  return selectUserConf(paths).path;
}

export function readConfig(
  paths: ConfigPaths | string = resolveConfigPaths(),
  defaultsPath: string = DEFAULT_CONFIG_PATH,
  overrides: Partial<Pick<Record<string, string>, "metrics_repo">> = {},
): TuConfig {
  // String form (existing tests): treated as userConf only — no org layer,
  // no legacy fallback.
  const p: ConfigPaths = typeof paths === "string" ? { configDir: "", userConf: paths } : paths;

  // Cascade: tu.default.conf < org.conf < user conf < env < CLI overrides.
  const defaults = readConfFile(defaultsPath) ?? {};
  const orgRaw = p.orgConf !== undefined ? readConfFile(p.orgConf) : null;
  const userSel = selectUserConf(p);

  const merged: Record<string, string> = { ...defaults, ...(orgRaw ?? {}), ...userSel.conf };

  // TU_METRICS_REPO env var takes precedence over config files; an explicit
  // CLI-layer override (e.g. `tu init-metrics <url>`) beats the env var.
  const envRepo = process.env.TU_METRICS_REPO;
  const metricsRepo = overrides.metrics_repo ?? ((envRepo && envRepo.length > 0) ? envRepo : (merged.metrics_repo || ""));

  // Derive mode from metrics_repo presence
  const mode: TuConfig["mode"] = metricsRepo !== "" ? "multi" : "single";

  const versionRaw = merged.version ? parseInt(merged.version, 10) : 1;
  const version = Number.isNaN(versionRaw) ? 1 : versionRaw;
  if (version > CURRENT_CONFIG_VERSION) {
    // Name the file the value came from: the user conf actually read, else the
    // org layer when it was actually read, else the shipped defaults. (An
    // org.conf that failed to read/parse never fed the merge, so blaming it
    // would misattribute the version's source.)
    const versionSource = userSel.path ?? (orgRaw !== null ? p.orgConf! : defaultsPath);
    console.error(
      `Warning: ${versionSource} version ${version} is newer than tu supports (${CURRENT_CONFIG_VERSION}). Please update tu.`,
    );
  }

  const autoSyncRaw = merged.auto_sync;
  const autoSync = autoSyncRaw === "false" || autoSyncRaw === "0" ? false : true;

  return {
    version,
    mode,
    metricsRepo,
    metricsDir: resolveHome(expandSentinels(merged.metrics_dir || "~/.tu/metrics_repo")),
    machine: expandSentinels(merged.machine || "$HOSTNAME"),
    user: expandSentinels(merged.user || "$USER"),
    autoSync,
  };
}
