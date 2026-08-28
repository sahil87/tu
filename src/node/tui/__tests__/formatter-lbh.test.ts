import { describe, it, afterEach } from "node:test";
import assert from "node:assert/strict";

import { renderTotalHistory, periodLabel } from "../formatter.js";
import { setNoColor, stripAnsi } from "../colors.js";
import type { FormatOptions } from "../formatter.js";
import type { UsageEntry } from "../../core/types.js";

// Match the other formatter suites: NO_COLOR is unset at module scope so
// setNoColor alone controls coloring (the dev shell exports NO_COLOR=1).
delete process.env.NO_COLOR;
setNoColor(true);

afterEach(() => setNoColor(true));

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

// Two user columns over two months; alice outspends bob overall, but bob wins July.
function userMap(): Map<string, UsageEntry[]> {
  return new Map<string, UsageEntry[]>([
    ["alice", [entry("2026-07", 100, 10_000), entry("2026-08", 300, 30_000)]],
    ["bob", [entry("2026-07", 200, 20_000), entry("2026-08", 50, 5_000)]],
  ]);
}

const LBH_OPTS: FormatOptions = {
  historyTitle: "\u{1F4CA} Leaderboard History (monthly)",
  columnOrder: "total-desc",
  highlightRowLeader: true,
  omitNegligibleColumns: false,
};

describe("renderTotalHistory leaderboard hooks", () => {
  it("defaults are byte-identical to a metric-less render (tu h unchanged)", () => {
    setNoColor(true);
    const map = userMap();
    const plain = renderTotalHistory("monthly", map, 100);
    const defaulted = renderTotalHistory("monthly", map, 100, {
      columnOrder: "registry",
      highlightRowLeader: false,
      omitNegligibleColumns: true,
    });
    assert.deepEqual(defaulted, plain);
    const titled = renderTotalHistory("monthly", map, 100, {});
    assert.deepEqual(titled, plain);
  });

  it("historyTitle replaces the Combined Cost History title", () => {
    setNoColor(true);
    const out = renderTotalHistory("monthly", userMap(), 100, LBH_OPTS).join("\n");
    assert.ok(out.includes("Leaderboard History (monthly)"), out);
    assert.ok(!out.includes("Combined Cost History"), out);
  });

  it("columnOrder total-desc orders columns by descending window total", () => {
    setNoColor(true);
    const lines = renderTotalHistory("monthly", userMap(), 100, LBH_OPTS);
    const header = lines.find((l) => l.includes("alice") && l.includes("bob"))!;
    // alice's window total (400) > bob's (250) → alice's column first.
    assert.ok(header.indexOf("alice") < header.indexOf("bob"), header);
  });

  it("registry order is kept without the hook", () => {
    setNoColor(true);
    // Registry order follows map insertion: bob inserted first.
    const map = new Map<string, UsageEntry[]>([
      ["bob", [entry("2026-07", 1, 100)]],
      ["alice", [entry("2026-07", 999, 100)]],
    ]);
    const lines = renderTotalHistory("monthly", map, 100);
    const header = lines.find((l) => l.includes("alice") && l.includes("bob"))!;
    assert.ok(header.indexOf("bob") < header.indexOf("alice"), header);
  });

  it("highlightRowLeader bolds each row's winning cell (color-only, width unchanged)", () => {
    // The uncolored reference render (legend swatches are color-gated, so the
    // comparison must be made with color disabled for the plain render too).
    setNoColor(true);
    const plain = renderTotalHistory("monthly", userMap(), 100, LBH_OPTS);
    setNoColor(false);
    let colored: string[];
    try {
      colored = renderTotalHistory("monthly", userMap(), 100, LBH_OPTS);
    } finally {
      setNoColor(true);
    }
    const julyLine = colored.find((l) => l.includes("2026-07"))!;
    const augLine = colored.find((l) => l.includes("2026-08"))!;
    // bob wins July ($200 > $100); alice wins August ($300 > $50).
    assert.ok(julyLine.includes("\x1b[1;37m"), julyLine);
    assert.ok(augLine.includes("\x1b[1;37m"), augLine);
    // Color-only: stripping ANSI yields the uncolored render except the
    // color-gated legend swatches in the footer.
    const stripped = colored.map(stripAnsi);
    assert.deepEqual(stripped.slice(0, -2), plain.slice(0, -2));
  });

  it("omitNegligibleColumns false keeps a negligible user column", () => {
    setNoColor(true);
    const map = new Map<string, UsageEntry[]>([
      ["alice", [entry("2026-08", 10_000, 100_000)]],
      ["ghost", [entry("2026-08", 0.10, 10)]],
    ]);
    const kept = renderTotalHistory("monthly", map, 100, LBH_OPTS).join("\n");
    assert.ok(kept.includes("ghost"), kept);
    // The default path would omit the negligible column.
    const omitted = renderTotalHistory("monthly", map, 100).join("\n");
    assert.ok(!omitted.includes("ghost"), omitted);
  });

  it("the lbh title built through periodLabel carries the cap hint like tu h", () => {
    setNoColor(true);
    // The dispatch layer builds the lbh title via periodLabel(period,
    // capActive) — the same helper the default Combined title uses — so a
    // capped daily lbh renders the "last 3 months" hint exactly as `tu h`.
    const capped = renderTotalHistory("daily", userMap(), 100, {
      ...LBH_OPTS,
      historyTitle: `\u{1F4CA} Leaderboard History (${periodLabel("daily", true)})`,
    }).join("\n");
    assert.ok(capped.includes("Leaderboard History (daily, last 3 months)"), capped);
    const uncapped = renderTotalHistory("daily", userMap(), 100, {
      ...LBH_OPTS,
      historyTitle: `\u{1F4CA} Leaderboard History (${periodLabel("daily", false)})`,
    }).join("\n");
    assert.ok(uncapped.includes("Leaderboard History (daily)"), uncapped);
    assert.ok(!uncapped.includes("last 3 months"), uncapped);
  });
});
