import type { UsageEntry, UsageTotals } from "./types.js";
import type { BarMetric } from "../tui/formatter.js";
import { metricValue } from "../tui/formatter.js";
import { currentLabel } from "./fetcher.js";

// Pure leaderboard aggregation for the `lb` / `lbh` displays (intake §2).
// No I/O, no mutation of inputs (Constitution V) — the dispatch layer fetches
// and windows daily entries, this module ranks them.

export interface LeaderboardRow {
  rank: number;          // 1-based, after sorting
  user: string;          // user name, or "user/machine" under --by-machine
  machine?: string;      // present only under --by-machine
  totals: UsageTotals;   // summed across the source's tools and the window
  share: number;         // 0..1 of the grand total, in the display metric
  delta?: number;        // fractional change vs the previous window; undefined ⇒ "new"
}

// A resolved date window (ISO YYYY-MM-DD bounds) plus the label used in the
// Δ column header / heading. Bounds are optional only for an open-ended
// explicit --since/--until window.
export interface LeaderboardWindow {
  start?: string;
  end?: string;
  label: string;
}

const SHORT_MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
const DAY_MS = 86_400_000;

function isoLocal(d: Date): string {
  const mm = String(d.getMonth() + 1).padStart(2, "0");
  const dd = String(d.getDate()).padStart(2, "0");
  return `${d.getFullYear()}-${mm}-${dd}`;
}

// Day arithmetic on date-only ISO labels, parsed as UTC midnight (the same
// treatment weekLabel gives daily labels — timezone-independent).
function addDays(iso: string, days: number): string {
  const d = new Date(`${iso}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() + days);
  return d.toISOString().slice(0, 10);
}

function diffDays(a: string, b: string): number {
  return Math.round((new Date(`${b}T00:00:00Z`).getTime() - new Date(`${a}T00:00:00Z`).getTime()) / DAY_MS);
}

// The window the leaderboard ranks: the current period (today / this
// Sunday-anchored week / this month), or the explicit --since/--until range
// when one is given (it replaces the period window). The window label feeds
// the heading: the period's current label, or the range itself.
export function currentWindow(period: string, since: string | undefined, until: string | undefined, now: Date = new Date()): LeaderboardWindow {
  if (since !== undefined || until !== undefined) {
    const label = since !== undefined && until !== undefined
      ? `${since} → ${until}`
      : since !== undefined ? `${since} →` : `→ ${until}`;
    return { start: since, end: until, label };
  }
  const today = currentLabel("daily", now);
  if (period === "monthly") {
    return { start: `${today.slice(0, 7)}-01`, end: today, label: currentLabel("monthly", now) };
  }
  if (period === "weekly") {
    const sunday = currentLabel("weekly", now);
    return { start: sunday, end: today, label: sunday };
  }
  return { start: today, end: today, label: today };
}

// The immediately preceding window of the same kind: previous calendar day /
// previous Sunday-anchored week / previous calendar month — or, under an
// explicit window, the equal-length range ending the day before `since`
// (label "prev"). Returns undefined when no well-defined previous window
// exists (an open-ended explicit window), in which case every row is "new".
export function previousWindow(period: string, since: string | undefined, until: string | undefined, now: Date = new Date()): LeaderboardWindow | undefined {
  if (since !== undefined || until !== undefined) {
    if (since === undefined) return undefined;
    const end = until ?? currentLabel("daily", now);
    const length = diffDays(since, end) + 1;
    if (length < 1) return undefined;
    return { start: addDays(since, -length), end: addDays(since, -1), label: "prev" };
  }
  if (period === "monthly") {
    const first = new Date(now.getFullYear(), now.getMonth() - 1, 1);
    const last = new Date(now.getFullYear(), now.getMonth(), 0);
    return { start: isoLocal(first), end: isoLocal(last), label: SHORT_MONTHS[last.getMonth()] };
  }
  if (period === "weekly") {
    const sunday = currentLabel("weekly", now);
    const start = addDays(sunday, -7);
    return { start, end: addDays(sunday, -1), label: start };
  }
  const prev = new Date(now);
  prev.setDate(prev.getDate() - 1);
  const label = isoLocal(prev);
  return { start: label, end: label, label };
}

// Sum every key's entries into one UsageTotals (all six numeric fields).
// Pure: entries and the input map are never mutated.
export function sumByKey(byUser: Map<string, UsageEntry[]>): Map<string, UsageTotals> {
  const out = new Map<string, UsageTotals>();
  for (const [key, entries] of byUser) {
    const totals: UsageTotals = { totalCost: 0, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 0 };
    for (const e of entries) {
      totals.totalCost += e.totalCost;
      totals.inputTokens += e.inputTokens;
      totals.outputTokens += e.outputTokens;
      totals.cacheCreationTokens += e.cacheCreationTokens;
      totals.cacheReadTokens += e.cacheReadTokens;
      totals.totalTokens += e.totalTokens;
    }
    out.set(key, totals);
  }
  return out;
}

// Rank keys descending by the display metric. Keys with zero tokens AND zero
// cost in the window are dropped (mirrors the snapshot renderer's zero-row
// omission); ties break by key name ascending for deterministic output.
// `share` is the row's fraction of the grand total in the display metric
// (0 when the grand total is 0). `delta` is the fractional change vs the
// previous window; undefined (rendered "new") when the key had no previous
// window or a previous value of exactly 0 (no divide-by-zero).
export function buildLeaderboard(
  byUser: Map<string, UsageEntry[]>,
  prevByUser: Map<string, UsageEntry[]> | undefined,
  metric: BarMetric,
): LeaderboardRow[] {
  const summed = sumByKey(byUser);
  const prevSummed = prevByUser !== undefined ? sumByKey(prevByUser) : undefined;

  const kept = [...summed.entries()].filter(([, t]) => !(t.totalTokens === 0 && t.totalCost === 0));
  kept.sort((a, b) => metricValue(b[1], metric) - metricValue(a[1], metric) || (a[0] < b[0] ? -1 : a[0] > b[0] ? 1 : 0));

  let grand = 0;
  for (const [, t] of kept) grand += metricValue(t, metric);

  return kept.map(([key, totals], i) => {
    const value = metricValue(totals, metric);
    const prevTotals = prevSummed?.get(key);
    const prevValue = prevTotals !== undefined ? metricValue(prevTotals, metric) : 0;
    const slash = key.indexOf("/");
    return {
      rank: i + 1,
      user: slash === -1 ? key : key.slice(0, slash),
      ...(slash === -1 ? {} : { machine: key.slice(slash + 1) }),
      totals,
      share: grand > 0 ? value / grand : 0,
      delta: prevValue !== 0 ? (value - prevValue) / prevValue : undefined,
    };
  });
}
