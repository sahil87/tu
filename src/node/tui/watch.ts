import type { FormatOptions } from "./formatter.js";
import type { UsageEntry } from "../core/types.js";
import type { PanelSession } from "./panel.js";
import { formatElapsed, computeBurnRate } from "./panel.js";
import { Compositor } from "./compositor.js";
import { dim, stripAnsi } from "./colors.js";

// Re-export for backward compatibility (tests and panel.ts import from watch.ts)
export { formatElapsed, computeBurnRate, stripAnsi };

// Re-export mergeSideBySide from compositor for backward compat
export { mergeSideBySide } from "./compositor.js";

export interface WatchOptions {
  interval: number;        // poll interval in seconds
  action: (skipCache: boolean, formatOpts?: FormatOptions) => Promise<string[]>;
  getCost: () => number;   // callback to get current total cost after render
  getPrevCosts: () => Map<string, number>;  // callback to get per-item costs for delta indicators
  getTotalTokens?: () => number;  // callback to get total tokens for session stats
  fetchHistory?: () => Promise<UsageEntry[]>;  // callback to fetch daily history for sparkline
  noRain?: boolean;        // disable rain animation
}

interface WatchSession {
  startTime: number;
  startCost: number;
  previousCosts: Map<string, number>;
  pollHistory: Array<{ time: number; cost: number }>;
  totalTokens: number;
}

export const ROLLING_WINDOW = 5;
export const COMPACT_THRESHOLD = 60;
const HISTORY_CACHE_TTL = 5 * 60 * 1000; // 5 minutes

function enterAltScreen(): void {
  process.stdout.write("\x1b[?1049h");
}

function exitAltScreen(): void {
  process.stdout.write("\x1b[?1049l");
}

export async function runWatch(opts: WatchOptions): Promise<never> {
  const { interval, action, getCost, getPrevCosts, getTotalTokens, fetchHistory, noRain } = opts;

  const session: WatchSession = {
    startTime: 0,
    startCost: 0,
    previousCosts: new Map(),
    pollHistory: [],
    totalTokens: 0,
  };

  let cleaning = false;
  let polling = false;
  let lastRenderedLines: string[] = [];

  // History cache for sparkline
  let historyCache: UsageEntry[] = [];
  let historyCacheTime = 0;

  const termWidth = () => process.stdout.columns ?? 80;
  const termRows = () => process.stdout.rows ?? 24;

  // Create compositor
  const compositor = new Compositor({
    noRain,
    getTermWidth: termWidth,
    getTermRows: termRows,
  });

  async function refreshHistoryCache(): Promise<void> {
    if (!fetchHistory) return;
    const now = Date.now();
    if (now - historyCacheTime < HISTORY_CACHE_TTL && historyCache.length > 0) return;
    try {
      historyCache = await fetchHistory();
      historyCacheTime = now;
    } catch {
      // Keep stale cache on failure
    }
  }

  function cleanup(): void {
    if (cleaning) return;
    cleaning = true;

    // 1. Stop compositor (clears all timers)
    compositor.stop();

    // 2. Restore raw mode
    if (process.stdin.isTTY && process.stdin.isRaw) {
      process.stdin.setRawMode(false);
    }
    process.stdin.pause();

    // 3. Exit alternate screen
    exitAltScreen();

    // 4. Print last rendered output (no API re-fetch for fast exit)
    for (const l of lastRenderedLines) {
      console.log(l);
    }

    // 5. Exit
    process.exit(0);
  }

  // Register SIGINT handler
  process.on("SIGINT", () => { cleanup(); });

  // Enter alternate screen
  enterAltScreen();

  // Start compositor
  compositor.start();

  // Real-time resize handling
  process.stdout.on("resize", () => {
    compositor.rerender();
  });

  // Setup raw mode stdin for key handling
  if (process.stdin.isTTY) {
    process.stdin.setRawMode(true);
    process.stdin.resume();
    process.stdin.setEncoding("utf-8");
    process.stdin.on("data", (key: string) => {
      if (key === "q" || key === "\u0003") {
        cleanup();
        return;
      }
      if (key === "\r" || key === "\n" || key === " ") {
        // Enter or Space — immediate refresh
        compositor.cancelCountdown();
        doPoll();
        return;
      }
    });
  }

  async function doPoll(): Promise<void> {
    // Re-entrancy guard
    if (polling) return;
    polling = true;

    try {
      // Show "Refreshing..." status
      compositor.setRefreshing();

      // Refresh history cache for sparkline
      await refreshHistoryCache();

      // Build FormatOptions
      const tw = termWidth();
      const compact = tw < COMPACT_THRESHOLD;
      const formatOpts: FormatOptions = {
        prevCosts: session.previousCosts.size > 0 ? session.previousCosts : undefined,
        compact: compact || undefined,
      };

      let tableLines: string[] = [];
      let actionFailed = false;
      try {
        tableLines = await action(true, formatOpts);
      } catch (err) {
        process.stderr.write(`Warning: fetch failed, retrying next cycle\n`);
        actionFailed = true;
      }

      if (actionFailed) {
        compositor.startCountdown(interval, () => doPoll());
        return;
      }

      // Capture cost and per-item costs
      const cost = getCost();
      const now = Date.now();

      if (session.pollHistory.length === 0) {
        session.startTime = now;
        session.startCost = cost;
      }

      session.pollHistory.push({ time: now, cost });
      if (getTotalTokens) session.totalTokens = getTotalTokens();

      // Build panel session
      const panelSession: PanelSession = {
        startTime: session.startTime || Date.now(),
        startCost: session.startCost,
        pollHistory: session.pollHistory,
        totalTokens: getTotalTokens ? getTotalTokens() : session.totalTokens,
      };

      // Store for fast quit
      lastRenderedLines = tableLines;

      // Update compositor with new data
      compositor.updateAfterPoll(tableLines, panelSession, historyCache, cost);

      // Flush all panels to screen
      compositor.flush();

      // Populate previousCosts for next cycle's delta indicators
      session.previousCosts = getPrevCosts();

      // Start countdown for next poll
      compositor.startCountdown(interval, () => doPoll());
    } finally {
      polling = false;
    }
  }

  // Initial history fetch
  await refreshHistoryCache();

  // Initial poll
  await doPoll();

  // Keep process alive
  return new Promise<never>(() => {});
}

// Exported for testing
export { HISTORY_CACHE_TTL };
export type { WatchSession };

// Legacy buildFooter export for backward compatibility in tests.
// The compositor's StatusPanel now handles footer rendering.
// This function only produces controls-only content (no session/rate).
export function buildFooter(
  countdown: number | null,
  _session: WatchSession,
  termWidth: number,
): string {
  const parts: string[] = [];

  if (countdown === null) {
    parts.push(dim("Refreshing..."));
  } else {
    parts.push(dim(`Next refresh: ${countdown}s`));
  }

  parts.push(dim("\u21B5 refresh \u00B7 q quit"));

  let line = parts.join(dim(" \u00B7 "));
  while (stripAnsi(line).length > termWidth && parts.length > 1) {
    parts.pop();
    line = parts.join(dim(" \u00B7 "));
  }

  return line;
}
