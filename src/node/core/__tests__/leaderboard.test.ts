import { describe, it } from "node:test";
import assert from "node:assert/strict";

import { buildLeaderboard, currentWindow, previousWindow, sumByKey } from "../leaderboard.js";
import type { UsageEntry } from "../types.js";

function entry(label: string, totalCost: number, totalTokens = 0): UsageEntry {
  return {
    label,
    totalCost,
    totalTokens,
    inputTokens: 0,
    outputTokens: 0,
    cacheCreationTokens: 0,
    cacheReadTokens: 0,
  };
}

describe("buildLeaderboard", () => {
  it("ranks descending by cost, computes share and delta, drops zero rows", () => {
    const byUser = new Map<string, UsageEntry[]>([
      ["alice", [entry("2026-08-01", 6), entry("2026-08-02", 4)]],
      ["bob", [entry("2026-08-01", 10)]],
      ["chen", [entry("2026-08-01", 0, 0)]],
    ]);
    const prevByUser = new Map<string, UsageEntry[]>([
      ["alice", [entry("2026-07-01", 8)]],
    ]);
    const rows = buildLeaderboard(byUser, prevByUser, "cost");
    assert.equal(rows.length, 2);
    // alice and bob tie at $10 — tie-break by key ascending
    assert.equal(rows[0].user, "alice");
    assert.equal(rows[0].rank, 1);
    assert.equal(rows[0].share, 0.5);
    assert.equal(rows[0].delta, 0.25); // (10 - 8) / 8
    assert.equal(rows[1].user, "bob");
    assert.equal(rows[1].rank, 2);
    assert.equal(rows[1].share, 0.5);
    assert.equal(rows[1].delta, undefined); // absent from prev ⇒ "new"
  });

  it("sorts strictly descending by the metric value", () => {
    const byUser = new Map<string, UsageEntry[]>([
      ["bob", [entry("2026-08-01", 5)]],
      ["alice", [entry("2026-08-01", 20)]],
      ["chen", [entry("2026-08-01", 10)]],
    ]);
    const rows = buildLeaderboard(byUser, undefined, "cost");
    assert.deepEqual(rows.map((r) => r.user), ["alice", "chen", "bob"]);
    assert.deepEqual(rows.map((r) => r.rank), [1, 2, 3]);
  });

  it("keys on tokens under the tokens metric", () => {
    const byUser = new Map<string, UsageEntry[]>([
      ["alice", [entry("2026-08-01", 100, 10)]],
      ["bob", [entry("2026-08-01", 1, 500)]],
    ]);
    const prevByUser = new Map<string, UsageEntry[]>([
      ["bob", [entry("2026-07-01", 1, 250)]],
    ]);
    const rows = buildLeaderboard(byUser, prevByUser, "tokens");
    assert.deepEqual(rows.map((r) => r.user), ["bob", "alice"]);
    assert.equal(rows[0].share, 500 / 510);
    assert.equal(rows[0].delta, 1); // (500 - 250) / 250
  });

  it("drops keys whose tokens and cost are both zero, keeps a zero-cost key with tokens", () => {
    const byUser = new Map<string, UsageEntry[]>([
      ["ghost", [entry("2026-08-01", 0, 0)]],
      ["free", [entry("2026-08-01", 0, 300)]],
      ["alice", [entry("2026-08-01", 5, 100)]],
    ]);
    const rows = buildLeaderboard(byUser, undefined, "cost");
    assert.deepEqual(rows.map((r) => r.user), ["alice", "free"]);
  });

  it("treats a previous value of exactly 0 as new", () => {
    const byUser = new Map<string, UsageEntry[]>([["alice", [entry("2026-08-01", 5, 1)]]]);
    const prevByUser = new Map<string, UsageEntry[]>([["alice", [entry("2026-07-01", 0, 0)]]]);
    const rows = buildLeaderboard(byUser, prevByUser, "cost");
    assert.equal(rows[0].delta, undefined);
  });

  it("computes share as 0 when the grand total is 0", () => {
    const byUser = new Map<string, UsageEntry[]>([["free", [entry("2026-08-01", 0, 100)]]]);
    const rows = buildLeaderboard(byUser, undefined, "cost");
    assert.equal(rows[0].share, 0);
  });

  it("shares sum to 1 across rows", () => {
    const byUser = new Map<string, UsageEntry[]>([
      ["a", [entry("2026-08-01", 3)]],
      ["b", [entry("2026-08-01", 7)]],
      ["c", [entry("2026-08-01", 10)]],
    ]);
    const rows = buildLeaderboard(byUser, undefined, "cost");
    const sum = rows.reduce((s, r) => s + r.share, 0);
    assert.ok(Math.abs(sum - 1) < 1e-12);
  });

  it("splits user/machine keys into user + machine fields", () => {
    const byUser = new Map<string, UsageEntry[]>([
      ["alice/laptop", [entry("2026-08-01", 5)]],
      ["bob", [entry("2026-08-01", 3)]],
    ]);
    const rows = buildLeaderboard(byUser, undefined, "cost");
    assert.equal(rows[0].user, "alice");
    assert.equal(rows[0].machine, "laptop");
    assert.equal(rows[1].user, "bob");
    assert.equal(rows[1].machine, undefined);
  });

  it("sums all six UsageTotals fields per key", () => {
    const byUser = new Map<string, UsageEntry[]>([
      ["alice", [
        { label: "2026-08-01", totalCost: 1, inputTokens: 10, outputTokens: 20, cacheCreationTokens: 30, cacheReadTokens: 40, totalTokens: 100 },
        { label: "2026-08-02", totalCost: 2, inputTokens: 1, outputTokens: 2, cacheCreationTokens: 3, cacheReadTokens: 4, totalTokens: 10 },
      ]],
    ]);
    const rows = buildLeaderboard(byUser, undefined, "cost");
    assert.deepEqual(rows[0].totals, {
      totalCost: 3,
      inputTokens: 11,
      outputTokens: 22,
      cacheCreationTokens: 33,
      cacheReadTokens: 44,
      totalTokens: 110,
    });
  });

  it("does not mutate its inputs", () => {
    const alice = [entry("2026-08-01", 10, 100)];
    const byUser = new Map<string, UsageEntry[]>([["alice", alice]]);
    const prevByUser = new Map<string, UsageEntry[]>([["alice", [entry("2026-07-01", 5, 50)]]]);
    const aliceSnapshot = JSON.stringify(alice);
    const prevSnapshot = JSON.stringify(prevByUser.get("alice"));
    buildLeaderboard(byUser, prevByUser, "cost");
    assert.equal(JSON.stringify(alice), aliceSnapshot);
    assert.equal(JSON.stringify(prevByUser.get("alice")), prevSnapshot);
  });
});

describe("sumByKey", () => {
  it("sums each key's entries into one UsageTotals", () => {
    const map = new Map<string, UsageEntry[]>([
      ["alice", [entry("2026-08-01", 2, 10), entry("2026-08-02", 3, 20)]],
      ["bob", [entry("2026-08-01", 7, 70)]],
    ]);
    const summed = sumByKey(map);
    assert.equal(summed.get("alice")!.totalCost, 5);
    assert.equal(summed.get("alice")!.totalTokens, 30);
    assert.equal(summed.get("bob")!.totalCost, 7);
  });
});

describe("previousWindow", () => {
  const now = new Date(2026, 7, 28); // 2026-08-28 local (a Friday)

  it("daily: the previous calendar day, labeled with its ISO date", () => {
    assert.deepEqual(previousWindow("daily", undefined, undefined, now), {
      start: "2026-08-27",
      end: "2026-08-27",
      label: "2026-08-27",
    });
  });

  it("weekly: the previous Sunday-anchored week, labeled with its Sunday", () => {
    // Current week Sunday is 2026-08-23; previous week is 08-16 .. 08-22.
    assert.deepEqual(previousWindow("weekly", undefined, undefined, now), {
      start: "2026-08-16",
      end: "2026-08-22",
      label: "2026-08-16",
    });
  });

  it("monthly: the previous calendar month, labeled with its short month", () => {
    assert.deepEqual(previousWindow("monthly", undefined, undefined, now), {
      start: "2026-07-01",
      end: "2026-07-31",
      label: "Jul",
    });
  });

  it("monthly across year boundary", () => {
    const jan = new Date(2026, 0, 15);
    assert.deepEqual(previousWindow("monthly", undefined, undefined, jan), {
      start: "2025-12-01",
      end: "2025-12-31",
      label: "Dec",
    });
  });

  it("explicit window: equal-length range ending the day before since, labeled prev", () => {
    assert.deepEqual(previousWindow("daily", "2026-08-10", "2026-08-19", now), {
      start: "2026-07-31",
      end: "2026-08-09",
      label: "prev",
    });
  });

  it("explicit window with only --since: length measured through today", () => {
    // since 2026-08-26, today 2026-08-28 → 3 days; prev is 2026-08-23..25.
    assert.deepEqual(previousWindow("daily", "2026-08-26", undefined, now), {
      start: "2026-08-23",
      end: "2026-08-25",
      label: "prev",
    });
  });

  it("explicit window with only --until has no previous window", () => {
    assert.equal(previousWindow("daily", undefined, "2026-08-19", now), undefined);
  });
});

describe("currentWindow", () => {
  const now = new Date(2026, 7, 28); // 2026-08-28 local

  it("daily: today, labeled with the ISO date", () => {
    assert.deepEqual(currentWindow("daily", undefined, undefined, now), {
      start: "2026-08-28",
      end: "2026-08-28",
      label: "2026-08-28",
    });
  });

  it("weekly: this week's Sunday through today, labeled with the Sunday", () => {
    assert.deepEqual(currentWindow("weekly", undefined, undefined, now), {
      start: "2026-08-23",
      end: "2026-08-28",
      label: "2026-08-23",
    });
  });

  it("monthly: the first of the month through today, labeled YYYY-MM", () => {
    assert.deepEqual(currentWindow("monthly", undefined, undefined, now), {
      start: "2026-08-01",
      end: "2026-08-28",
      label: "2026-08",
    });
  });

  it("explicit --since/--until replaces the period window and labels the range", () => {
    assert.deepEqual(currentWindow("monthly", "2026-08-01", "2026-08-27", now), {
      start: "2026-08-01",
      end: "2026-08-27",
      label: "2026-08-01 → 2026-08-27",
    });
  });
});
