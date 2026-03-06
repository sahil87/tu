// Side panel: sparkline + session stats (including session cost delta and burn rate)
// Composed and returned as string[] for side-by-side merge

import { renderSparkline } from "./sparkline.js";
import { dim, boldWhite, yellow } from "./colors.js";
import type { UsageEntry } from "./types.js";

const ROLLING_WINDOW = 5;

export function formatElapsed(ms: number): string {
  const totalSec = Math.floor(Math.max(0, ms) / 1000);
  const h = Math.floor(totalSec / 3600);
  const m = Math.floor((totalSec % 3600) / 60);
  const s = totalSec % 60;
  if (h > 0) return `${h}h ${m}m ${s}s`;
  if (m > 0) return `${m}m ${s}s`;
  return `${s}s`;
}

export function computeBurnRate(pollHistory: Array<{ time: number; cost: number }>): number | null {
  if (pollHistory.length < 2) return null;
  const windowStart = Math.max(0, pollHistory.length - ROLLING_WINDOW);
  const oldest = pollHistory[windowStart];
  const latest = pollHistory[pollHistory.length - 1];
  const timeDelta = latest.time - oldest.time;
  if (timeDelta === 0) return 0;
  return ((latest.cost - oldest.cost) / timeDelta) * 3600000;
}

export interface PanelSession {
  startTime: number;
  startCost: number;
  pollHistory: Array<{ time: number; cost: number }>;
  totalTokens: number;
}

const PANEL_MIN_WIDTH = 20;

function fmtDollar(n: number): string {
  return `$${Math.abs(n).toFixed(2)}`;
}

export function buildPanel(
  session: PanelSession,
  history: UsageEntry[],
  panelWidth: number,
  todayCost: number,
): string[] {
  if (panelWidth < PANEL_MIN_WIDTH) return [];

  const lines: string[] = [];

  // Sparkline
  if (history.length >= 2) {
    const sparkData = history.map((e) => ({ label: e.label, cost: e.totalCost }));
    const sparkLines = renderSparkline(sparkData, panelWidth);
    lines.push(...sparkLines);
    lines.push(""); // blank separator
  }

  // Session stats
  const now = Date.now();
  const elapsed = now - session.startTime;
  const elapsedMin = elapsed / 60000;

  lines.push(dim(" Session"));
  lines.push(dim(" " + "\u2500".repeat(Math.min(panelWidth - 2, 19))));

  const stats: Array<[string, string]> = [];

  // Session cost delta (only after 2+ polls)
  if (session.pollHistory.length > 1) {
    const currentCost = session.pollHistory[session.pollHistory.length - 1].cost;
    const delta = currentCost - session.startCost;
    const sign = delta >= 0 ? "+" : "-";
    stats.push(["Session", `${sign}${fmtDollar(delta)}`]);
  }

  stats.push(["Elapsed", formatElapsed(elapsed)]);

  if (elapsedMin > 0 && session.totalTokens > 0) {
    const tokPerMin = Math.round(session.totalTokens / elapsedMin);
    stats.push(["Tokens/min", `~${tokPerMin.toLocaleString("en-US")}`]);
  }

  const rate = computeBurnRate(session.pollHistory);
  if (rate !== null && rate > 0) {
    stats.push(["Rate", yellow(`~$${rate.toFixed(2)}/hr`)]);

    // Projected daily cost: burn rate * remaining hours in day + today's cost so far
    const nowDate = new Date(now);
    const hoursRemaining = 24 - nowDate.getHours() - nowDate.getMinutes() / 60;
    const projected = todayCost + rate * hoursRemaining;
    stats.push(["Proj. day", `~$${projected.toFixed(2)}`]);
  }

  const labelWidth = Math.max(...stats.map(([label]) => label.length));
  for (const [label, value] of stats) {
    lines.push(dim(` ${label.padEnd(labelWidth)}`) + "  " + boldWhite(value));
  }

  return lines;
}
