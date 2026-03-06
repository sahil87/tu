// Stats grid: 2x3 grid of session stats rendered above the table
// Replaces the old side panel (sparkline + vertical stats list)

import { dim, boldWhite, yellow } from "./colors.js";

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

function fmtDollar(n: number): string {
  return `$${Math.abs(n).toFixed(2)}`;
}

const PLACEHOLDER = "--";

// Column layout constants
const LEFT_LABEL_W = 9;      // "Elapsed  " or "Session  "
const MAX_LEFT_VALUE_W = 8;  // max width of left-column values (e.g., "+$12.34")
const GAP = 5;               // gap between left value and right label
const RIGHT_LABEL_W = 10;    // "Tok/min   " or "Proj. day "
const GRID_WIDTH = 1 + LEFT_LABEL_W + MAX_LEFT_VALUE_W + GAP + RIGHT_LABEL_W + 12; // 1 leading space + columns + max right value

/**
 * Build a 2x3 stats grid + dim separator line.
 *
 * Left column (session):  Elapsed, Session delta
 * Right column (cost):    Tok/min, Rate, Proj. day
 * Row 3 left is blank.
 *
 * Unavailable stats show "--" to keep the grid fixed at 3 rows.
 */
export function buildStatsGrid(session: PanelSession, todayCost: number): string[] {
  const now = Date.now();
  const elapsed = now - session.startTime;
  const elapsedMin = elapsed / 60000;
  const hasTwoPolls = session.pollHistory.length > 1;

  // Left column values
  const elapsedVal = formatElapsed(elapsed);

  let sessionVal: string;
  if (hasTwoPolls) {
    const currentCost = session.pollHistory[session.pollHistory.length - 1].cost;
    const delta = currentCost - session.startCost;
    const sign = delta >= 0 ? "+" : "-";
    sessionVal = `${sign}${fmtDollar(delta)}`;
  } else {
    // Skeleton / first poll: show $0.00 per spec
    sessionVal = `$${(0).toFixed(2)}`;
  }

  // Right column values
  let tokMinVal: string;
  if (hasTwoPolls && session.totalTokens > 0 && elapsedMin > 0) {
    tokMinVal = `~${Math.round(session.totalTokens / elapsedMin).toLocaleString("en-US")}`;
  } else {
    tokMinVal = PLACEHOLDER;
  }

  const rate = computeBurnRate(session.pollHistory);
  let rateVal: string;
  let projVal: string;

  if (rate !== null && rate > 0) {
    rateVal = `~$${rate.toFixed(2)}/hr`;
    const nowDate = new Date(now);
    const hoursRemaining = 24 - nowDate.getHours() - nowDate.getMinutes() / 60;
    const projected = todayCost + rate * hoursRemaining;
    projVal = `~$${projected.toFixed(2)}`;
  } else {
    rateVal = PLACEHOLDER;
    projVal = PLACEHOLDER;
  }

  // Build grid rows
  const lines: string[] = [];

  // Row 1: Elapsed | Tok/min
  lines.push(
    formatGridRow("Elapsed", elapsedVal, "Tok/min", tokMinVal, false),
  );

  // Row 2: Session | Rate
  lines.push(
    formatGridRow("Session", sessionVal, "Rate", rateVal, true),
  );

  // Row 3: (blank) | Proj. day
  lines.push(
    formatGridRow("", "", "Proj. day", projVal, false),
  );

  // Dim separator
  lines.push(dim("\u2500".repeat(GRID_WIDTH)));

  return lines;
}

function formatGridRow(
  leftLabel: string,
  leftValue: string,
  rightLabel: string,
  rightValue: string,
  rateHighlight: boolean,
): string {
  const left = leftLabel
    ? dim(` ${leftLabel.padEnd(LEFT_LABEL_W)}`) + boldWhite(leftValue)
    : " ".repeat(1 + LEFT_LABEL_W + PLACEHOLDER.length);

  const leftVisible = leftLabel
    ? 1 + LEFT_LABEL_W + leftValue.length
    : 1 + LEFT_LABEL_W + PLACEHOLDER.length;

  const rightPad = Math.max(1, 1 + LEFT_LABEL_W + MAX_LEFT_VALUE_W + GAP - leftVisible);
  const coloredValue = rateHighlight && rightValue !== PLACEHOLDER
    ? yellow(rightValue)
    : boldWhite(rightValue);

  const right = dim(`${rightLabel.padEnd(RIGHT_LABEL_W)}`) + coloredValue;

  return left + " ".repeat(rightPad) + right;
}
