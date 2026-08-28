import { describe, it, mock } from "node:test";
import assert from "node:assert/strict";

import { emitCsv, emitMarkdown, leaderboardRowsToJson } from "../formatter.js";
import type { LeaderboardRenderRow } from "../formatter.js";
import type { UsageTotals } from "../../core/types.js";

function totals(totalCost: number, totalTokens: number): UsageTotals {
  return { totalCost, totalTokens, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0 };
}

function mockRows(): LeaderboardRenderRow[] {
  return [
    { rank: 1, user: "alice", totals: totals(412.30, 9_000_000), share: 0.381, delta: 0.12 },
    { rank: 2, user: "sahil", totals: totals(301.10, 6_500_000), share: 0.278, delta: -0.04 },
    { rank: 3, user: "chen", totals: totals(149.20, 3_200_000), share: 0.138, delta: undefined },
  ];
}

let stdoutChunks: string[];

function captureStdout() {
  stdoutChunks = [];
  mock.method(process.stdout, "write", ((chunk: string | Uint8Array) => {
    stdoutChunks.push(typeof chunk === "string" ? chunk : Buffer.from(chunk).toString("utf-8"));
    return true;
  }) as never);
}

function restoreStdout() {
  mock.restoreAll();
}

function stdoutText(): string {
  return stdoutChunks.join("");
}

describe("emitCsv leaderboard kind", () => {
  it("emits rank,user,cost,total_tokens,share,delta with raw numbers and a Total row", (t) => {
    captureStdout();
    t.after(restoreStdout);
    emitCsv(mockRows(), "leaderboard", { period: "monthly" });
    const out = stdoutText();
    const lines = out.trimEnd().split("\n");
    assert.equal(lines[0], "rank,user,cost,total_tokens,share,delta");
    assert.equal(lines[1], "1,alice,412.30,9000000,0.381,0.12");
    assert.equal(lines[2], "2,sahil,301.10,6500000,0.278,-0.04");
    // delta empty for a "new" row
    assert.equal(lines[3], "3,chen,149.20,3200000,0.138,");
    // Total row sums every visible row (float arithmetic: 862.5999… → 862.60)
    assert.equal(lines[4], "Total,,862.60,18700000,,");
  });

  it("omits the Total row for a single row", (t) => {
    captureStdout();
    t.after(restoreStdout);
    emitCsv(mockRows().slice(0, 1), "leaderboard", { period: "monthly" });
    assert.ok(!stdoutText().includes("Total"));
  });

  it("under --top the Total row still sums the full row set (totalRows)", (t) => {
    captureStdout();
    t.after(restoreStdout);
    // --top 1 emits only alice; the Total row sums all three users.
    emitCsv(mockRows().slice(0, 1), "leaderboard", { period: "monthly", totalRows: mockRows() });
    const lines = stdoutText().trimEnd().split("\n");
    assert.equal(lines.length, 3);
    assert.equal(lines[2], "Total,,862.60,18700000,,");
  });

  it("adds a machine column after user under --by-machine", (t) => {
    captureStdout();
    t.after(restoreStdout);
    const rows: LeaderboardRenderRow[] = [
      { rank: 1, user: "alice", machine: "laptop", totals: totals(50, 500), share: 1, delta: undefined },
    ];
    emitCsv(rows, "leaderboard", { period: "monthly" });
    const out = stdoutText();
    const lines = out.trimEnd().split("\n");
    assert.equal(lines[0], "rank,user,machine,cost,total_tokens,share,delta");
    assert.equal(lines[1], "1,alice,laptop,50.00,500,1,");
  });

  it("quotes fields containing commas (RFC 4180)", (t) => {
    captureStdout();
    t.after(restoreStdout);
    const rows: LeaderboardRenderRow[] = [
      { rank: 1, user: "smith, john", totals: totals(1, 1), share: 1 },
    ];
    emitCsv(rows, "leaderboard", { period: "monthly" });
    assert.ok(stdoutText().includes('"smith, john"'), stdoutText());
  });
});

describe("emitMarkdown leaderboard kind", () => {
  it("emits a ## heading, GFM table, $ costs, and a bolded Total row", (t) => {
    captureStdout();
    t.after(restoreStdout);
    emitMarkdown(mockRows(), "leaderboard", { period: "monthly", deltaLabel: "Jul" });
    const out = stdoutText();
    assert.ok(out.startsWith("## Leaderboard (monthly)\n"), out);
    assert.ok(out.includes("| # | User | Cost | Tokens | Share | Δ vs Jul |"), out);
    assert.ok(out.includes("---:"), out);
    assert.ok(out.includes("$412.30"), out);
    assert.ok(out.includes("9,000,000"), out);
    assert.ok(out.includes("38.1%"), out);
    assert.ok(out.includes("+12%"), out);
    assert.ok(out.includes("| 3 | chen | $149.20 | 3,200,000 | 13.8% | new |"), out);
    assert.ok(out.includes("**$862.60**"), out);
    // No bars, no arrows, no staleness footer.
    assert.ok(!out.includes("█"), out);
    assert.ok(!out.includes("synced"), out);
  });

  it("omits the Total row for a single row and carries a machine column under --by-machine", (t) => {
    captureStdout();
    t.after(restoreStdout);
    const rows: LeaderboardRenderRow[] = [
      { rank: 1, user: "alice", machine: "laptop", totals: totals(50, 500), share: 1 },
    ];
    emitMarkdown(rows, "leaderboard", { period: "monthly" });
    const out = stdoutText();
    assert.ok(out.includes("| # | User | Machine | Cost | Tokens | Share | Δ vs prev |"), out);
    assert.ok(out.includes("| 1 | alice | laptop | $50.00 | 500 | 100.0% | new |"), out);
    assert.ok(!out.includes("**Total**"), out);
  });

  it("under --top the Total row still sums the full row set (totalRows)", (t) => {
    captureStdout();
    t.after(restoreStdout);
    emitMarkdown(mockRows().slice(0, 1), "leaderboard", { period: "monthly", deltaLabel: "Jul", totalRows: mockRows() });
    const out = stdoutText();
    assert.ok(out.includes("**$862.60**"), out);
    assert.ok(out.includes("**18,700,000**"), out);
  });

  it("mdTitle overrides the total-history Markdown heading (leaderboard history)", (t) => {
    captureStdout();
    t.after(restoreStdout);
    const data = new Map([
      ["alice", [{ label: "2026-08", totalCost: 10, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 100 }]],
      ["bob", [{ label: "2026-08", totalCost: 5, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 50 }]],
    ]);
    // The dispatch layer strips the 📊 emoji for the Markdown heading (every
    // other ## heading is emoji-free); the ANSI title keeps it.
    emitMarkdown(data, "total-history", { period: "monthly", mdTitle: "Leaderboard History (monthly)" });
    const out = stdoutText();
    assert.ok(out.startsWith("## Leaderboard History (monthly)\n"), out);
    assert.ok(!out.includes("Combined Cost History"), out);
  });

  it("total-history Markdown keeps the default heading without mdTitle", (t) => {
    captureStdout();
    t.after(restoreStdout);
    const data = new Map([
      ["alice", [{ label: "2026-08", totalCost: 10, inputTokens: 0, outputTokens: 0, cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 100 }]],
    ]);
    emitMarkdown(data, "total-history", { period: "monthly" });
    assert.ok(stdoutText().startsWith("## Combined Cost History (monthly)\n"));
  });
});

describe("leaderboardRowsToJson", () => {
  it("builds row objects with delta null for new rows and share as a fraction", () => {
    const json = leaderboardRowsToJson(mockRows());
    assert.deepEqual(json, [
      { rank: 1, user: "alice", cost: 412.30, totalTokens: 9_000_000, share: 0.381, delta: 0.12 },
      { rank: 2, user: "sahil", cost: 301.10, totalTokens: 6_500_000, share: 0.278, delta: -0.04 },
      { rank: 3, user: "chen", cost: 149.20, totalTokens: 3_200_000, share: 0.138, delta: null },
    ]);
  });

  it("includes the machine field only under --by-machine", () => {
    const rows: LeaderboardRenderRow[] = [
      { rank: 1, user: "alice", machine: "laptop", totals: totals(50, 500), share: 1 },
      { rank: 2, user: "bob", totals: totals(10, 100), share: 0 },
    ];
    const json = leaderboardRowsToJson(rows);
    assert.deepEqual(json[0], { rank: 1, user: "alice", machine: "laptop", cost: 50, totalTokens: 500, share: 1, delta: null });
    assert.ok(!("machine" in json[1]));
  });
});
