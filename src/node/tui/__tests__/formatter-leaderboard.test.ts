import { describe, it, afterEach } from "node:test";
import assert from "node:assert/strict";

import { renderLeaderboard } from "../formatter.js";
import { setNoColor, stripAnsi } from "../colors.js";
import type { LeaderboardRenderRow, LeaderboardRenderOptions } from "../formatter.js";
import type { UsageTotals } from "../../core/types.js";

// Match the other formatter suites: NO_COLOR is unset at module scope so
// setNoColor alone controls coloring (the dev shell exports NO_COLOR=1).
delete process.env.NO_COLOR;
setNoColor(true);

afterEach(() => setNoColor(true));

function totals(totalCost: number, totalTokens: number): UsageTotals {
  return { totalCost, totalTokens, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0 };
}

// The intake mockup fixture: alice/sahil/bob/chen, ranked by cost.
function mockRows(): LeaderboardRenderRow[] {
  return [
    { rank: 1, user: "alice", totals: totals(412.30, 9_000_000), share: 0.381, delta: 0.12 },
    { rank: 2, user: "sahil", totals: totals(301.10, 6_500_000), share: 0.278, delta: -0.04 },
    { rank: 3, user: "bob", totals: totals(220.05, 4_800_000), share: 0.203, delta: 0.31 },
    { rank: 4, user: "chen", totals: totals(149.20, 3_200_000), share: 0.138, delta: undefined },
  ];
}

const BASE_OPTS: LeaderboardRenderOptions = {
  period: "monthly",
  windowLabel: "2026-08",
  deltaLabel: "Jul",
  termWidth: 100,
};

describe("renderLeaderboard", () => {
  it("renders heading with period, window label and metric", () => {
    setNoColor(true);
    const out = renderLeaderboard(mockRows(), BASE_OPTS).join("\n");
    assert.ok(out.includes("Leaderboard (monthly) · 2026-08 · by cost"), out);
  });

  it("renders the tokens metric in the heading suffix only — both columns stay", () => {
    setNoColor(true);
    const out = renderLeaderboard(mockRows(), { ...BASE_OPTS, metric: "tokens" }).join("\n");
    assert.ok(out.includes("· by tokens"), out);
    assert.ok(out.includes("Cost"), out);
    assert.ok(out.includes("Tokens"), out);
  });

  it("renders all seven columns ranked by the input order", () => {
    setNoColor(true);
    const lines = renderLeaderboard(mockRows(), BASE_OPTS);
    const body = lines.join("\n");
    assert.ok(body.includes("#"), body);
    assert.ok(body.includes("Δ vs Jul"), body);
    const aliceLine = lines.find((l) => l.includes("alice"))!;
    assert.ok(aliceLine.startsWith("1 "), aliceLine);
    assert.ok(aliceLine.includes("$412.30"), aliceLine);
    assert.ok(aliceLine.includes("9,000,000"), aliceLine);
    assert.ok(aliceLine.includes("38.1%"), aliceLine);
    assert.ok(aliceLine.includes("+12%"), aliceLine);
    const chenLine = lines.find((l) => l.includes("chen"))!;
    assert.ok(chenLine.includes("new"), chenLine);
  });

  it("marks the pinned user with ◂ and counts it in the User column width", () => {
    setNoColor(true);
    const lines = renderLeaderboard(mockRows(), { ...BASE_OPTS, pinnedUser: "sahil" });
    const sahilLine = lines.find((l) => l.includes("sahil"))!;
    assert.ok(sahilLine.includes("sahil ◂"), sahilLine);
    // The marker widens the column: "sahil ◂" (8) is the longest name cell,
    // so every other name cell pads to 8 — alice (5) gains 3 trailing spaces.
    const aliceLine = lines.find((l) => l.includes("alice"))!;
    assert.ok(aliceLine.includes("alice   "), aliceLine);
  });

  it("renders the Total row (boldWhite) only when ≥2 rows, summing every row", () => {
    setNoColor(true);
    const lines = renderLeaderboard(mockRows(), BASE_OPTS);
    const totalLine = lines.find((l) => l.includes("Total"))!;
    assert.ok(totalLine.includes("$1,082.65"), totalLine);
    assert.ok(totalLine.includes("23,500,000"), totalLine);

    const single = renderLeaderboard(mockRows().slice(0, 1), BASE_OPTS);
    assert.ok(!single.some((l) => l.includes("Total")), single.join("\n"));
  });

  it("collapses rows past --top into a dim … +k others line that still counts toward Total", () => {
    setNoColor(true);
    const lines = renderLeaderboard(mockRows(), { ...BASE_OPTS, top: 2 });
    const out = lines.join("\n");
    assert.ok(!lines.some((l) => l.includes("bob") && l.includes("$220.05")), out);
    const others = lines.find((l) => l.includes("others"))!;
    assert.ok(others.includes("… +2 others"), others);
    // Collapsed users still count toward the Total row.
    const totalLine = lines.find((l) => l.includes("Total"))!;
    assert.ok(totalLine.includes("$1,082.65"), totalLine);
  });

  it("omits the collapsed line when k = 0", () => {
    setNoColor(true);
    const out = renderLeaderboard(mockRows(), { ...BASE_OPTS, top: 4 }).join("\n");
    assert.ok(!out.includes("others"), out);
  });

  it("renders the staleness footer from lastSync", () => {
    setNoColor(true);
    const synced = renderLeaderboard(mockRows(), { ...BASE_OPTS, lastSync: "42m ago (2026-08-28T10:00:00.000Z)" }).join("\n");
    assert.ok(synced.includes("synced 42m ago (2026-08-28T10:00:00.000Z) · tu sync to refresh"), synced);
    const never = renderLeaderboard(mockRows(), BASE_OPTS).join("\n");
    assert.ok(never.includes("never synced · tu sync to refresh"), never);
  });

  it("renders an empty leaderboard with heading and footer, never crashing", () => {
    setNoColor(true);
    const out = renderLeaderboard([], BASE_OPTS).join("\n");
    assert.ok(out.includes("Leaderboard (monthly) · 2026-08 · by cost"), out);
    assert.ok(out.includes("No data"), out);
    assert.ok(out.includes("never synced · tu sync to refresh"), out);
    assert.ok(!out.includes("Total"), out);
  });

  it("dims exact-zero metric cells", () => {
    setNoColor(false);
    delete process.env.NO_COLOR;
    try {
      const rows: LeaderboardRenderRow[] = [
        { rank: 1, user: "alice", totals: totals(10, 100), share: 1 },
        { rank: 2, user: "free", totals: totals(0, 50), share: 0 },
      ];
      const out = renderLeaderboard(rows, BASE_OPTS).join("\n");
      const freeLine = out.split("\n").find((l) => l.includes("free"))!;
      assert.ok(freeLine.includes("\x1b[2m"), freeLine);
    } finally {
      setNoColor(true);
    }
  });

  it("--no-color output is byte-equal to the ANSI-stripped colored output", () => {
    setNoColor(false);
    delete process.env.NO_COLOR;
    let colored: string[];
    try {
      colored = renderLeaderboard(mockRows(), { ...BASE_OPTS, pinnedUser: "sahil", lastSync: "5m ago (x)" }).map(stripAnsi);
    } finally {
      setNoColor(true);
    }
    const plain = renderLeaderboard(mockRows(), { ...BASE_OPTS, pinnedUser: "sahil", lastSync: "5m ago (x)" });
    assert.deepEqual(plain, colored);
  });

  it("long user names do not misalign the numeric columns", () => {
    setNoColor(true);
    const rows: LeaderboardRenderRow[] = [
      { rank: 1, user: "a-very-long-user-name", totals: totals(100, 1000), share: 0.9, delta: 0.1 },
      { rank: 2, user: "al", totals: totals(10, 100), share: 0.1 },
    ];
    const lines = renderLeaderboard(rows, BASE_OPTS);
    const dataLines = lines.filter((l) => l.includes("a-very-long-user-name") || l.startsWith("2 "));
    assert.equal(dataLines.length, 2);
    // Both rows split into the same number of pipe-separated cells.
    const cells = dataLines.map((l) => l.split("|").length);
    assert.equal(cells[0], cells[1]);
  });

  it("renders user/machine keys under --by-machine", () => {
    setNoColor(true);
    const rows: LeaderboardRenderRow[] = [
      { rank: 1, user: "alice", machine: "laptop", totals: totals(50, 500), share: 0.5 },
      { rank: 2, user: "alice", machine: "desktop", totals: totals(50, 500), share: 0.5 },
    ];
    const out = renderLeaderboard(rows, BASE_OPTS).join("\n");
    assert.ok(out.includes("alice/laptop"), out);
    assert.ok(out.includes("alice/desktop"), out);
  });

  it("renders the watch delta arrow on the metric cell", () => {
    setNoColor(true);
    const prevCosts = new Map<string, number>([["alice", 400]]);
    const lines = renderLeaderboard(mockRows(), { ...BASE_OPTS, prevCosts });
    const aliceLine = lines.find((l) => l.includes("alice"))!;
    assert.ok(aliceLine.includes("↑"), aliceLine);
    const bobLine = lines.find((l) => l.includes("bob"))!;
    assert.ok(!bobLine.includes("↑") && !bobLine.includes("↓"), bobLine);
  });

  it("pads the bar area so Tokens/Share/Δ start at the same offset on every row", () => {
    setNoColor(true);
    // Uneven costs → uneven single-zone bar lengths; the mid-row bar must not
    // shift the columns that follow it.
    const rows: LeaderboardRenderRow[] = [
      { rank: 1, user: "alice", totals: totals(1000, 9000), share: 0.9, delta: 0.5 },
      { rank: 2, user: "bob", totals: totals(10, 90), share: 0.01, delta: undefined },
      { rank: 3, user: "chen", totals: totals(100, 900), share: 0.09, delta: -0.1 },
    ];
    const lines = renderLeaderboard(rows, BASE_OPTS).map(stripAnsi);
    const dataLines = lines.filter((l) => /alice|bob|chen/.test(l) && l.includes("$"));
    assert.equal(dataLines.length, 3);
    // Split on the " | " separators: rank | user | cost+bar | tokens | share | Δ.
    // The bar is space-padded inside the cost+bar cell, so the cell boundaries
    // (pipe offsets) must be identical on every row.
    const pipeOffsets = dataLines.map((l) => [...l.matchAll(/\|/g)].map((m) => m.index));
    assert.deepEqual(pipeOffsets[0], pipeOffsets[1]);
    assert.deepEqual(pipeOffsets[1], pipeOffsets[2]);
    // And the Tokens column's first digit lands at the same offset per row.
    const tokenOffsets = dataLines.map((l) => l.indexOf(l.match(/\d{1,3}(?:,\d{3})*\s*\|/)![0]));
    assert.deepEqual(tokenOffsets[0], tokenOffsets[1]);
    assert.deepEqual(tokenOffsets[1], tokenOffsets[2]);
  });

  it("keeps columns aligned when --top collapses a long label row", () => {
    setNoColor(true);
    const rows: LeaderboardRenderRow[] = [
      { rank: 1, user: "al", totals: totals(100, 900), share: 0.5 },
      { rank: 2, user: "bo", totals: totals(100, 900), share: 0.5 },
    ];
    const lines = renderLeaderboard(rows, { ...BASE_OPTS, top: 1 }).map(stripAnsi);
    const rowLine = lines.find((l) => l.includes("al"))!;
    const collapsedLine = lines.find((l) => l.includes("others"))!;
    // The collapsed label "… +1 others" is longer than "al" — the User column
    // must have widened for it rather than letting it overflow into the pipes.
    const rowPipes = [...rowLine.matchAll(/\|/g)].map((m) => m.index);
    const collapsedPipes = [...collapsedLine.matchAll(/\|/g)].map((m) => m.index);
    assert.deepEqual(collapsedPipes, rowPipes);
  });

  it("reserves the watch indicator width in the bar budget", () => {
    setNoColor(true);
    const prevCosts = new Map<string, number>([["alice", 400]]);
    const withIndicator = renderLeaderboard(mockRows(), { ...BASE_OPTS, prevCosts }).map(stripAnsi);
    // Every line (incl. the row carrying the ↑ indicator) stays within the
    // terminal width — the indicator char was reserved out of the bar area.
    for (const l of withIndicator) {
      assert.ok(l.length <= BASE_OPTS.termWidth!, `line too wide (${l.length}): ${l}`);
    }
    assert.ok(withIndicator.some((l) => l.includes("↑")), withIndicator.join("\n"));
  });
});
