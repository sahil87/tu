import { describe, it, beforeEach, afterEach } from "node:test";
import assert from "node:assert/strict";
import { mkdirSync, rmSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";

import { writeMetrics, readRemoteEntriesByMachine } from "../../sync/sync.js";
import { mergeEntries, maxMergeEntries } from "../fetcher.js";
import type { UsageEntry } from "../types.js";

// Composition test for the self-view max-merge (fetchToolMerged /
// fetchToolMergedWithMachines in src/node/core/cli.ts). Those functions are
// not exported and shell out to ccusage for the live fetch, so — following
// the cli-user-flag.test.ts precedent — the rewired pipeline is mirrored
// here with the live `local` entries injected, exercising the real fs-backed
// pieces end-to-end against a temp metrics dir:
//
//   guarded writeMetrics → readRemoteEntriesByMachine(excludeMachine = null)
//   → own/others split → maxMergeEntries(local, own) → mergeEntries(..., others)
//
// If the production pipeline shape changes, update this mirror to match.

const TEST_DIR = join(tmpdir(), "tu-self-view-test-" + process.pid);

const USER = "sahil";
const MACHINE = "macbook";
const TOOL = "cc";

const entry = (label: string, cost: number, tokens = 150): UsageEntry => ({
  label,
  totalCost: cost,
  inputTokens: 100,
  outputTokens: 50,
  cacheCreationTokens: 0,
  cacheReadTokens: 0,
  totalTokens: tokens,
});

function seedSnapshot(machine: string, e: UsageEntry): void {
  const dir = join(TEST_DIR, USER, e.label.slice(0, 4), machine);
  mkdirSync(dir, { recursive: true });
  writeFileSync(join(dir, `${TOOL}-${e.label}.jsonl`), JSON.stringify(e) + "\n");
}

interface PipelineResult {
  merged: UsageEntry[];
  machineMap: Map<string, UsageEntry[]>;
}

// Mirrors the multi-mode own-user path of fetchToolMerged; machineMap mirrors
// what fetchToolMergedWithMachines exposes for --by-machine.
function mergedPipeline(local: UsageEntry[]): PipelineResult {
  writeMetrics(TEST_DIR, USER, MACHINE, TOOL, local);
  const byMachine = readRemoteEntriesByMachine(TEST_DIR, USER, null, TOOL);
  const ownSnapshots = byMachine.get(MACHINE) ?? [];
  const remote: UsageEntry[] = [];
  const machineMap = new Map<string, UsageEntry[]>();
  machineMap.set(MACHINE, maxMergeEntries(local, ownSnapshots));
  for (const [machine, machineEntries] of byMachine) {
    if (machine !== MACHINE) {
      remote.push(...machineEntries);
      machineMap.set(machine, machineEntries);
    }
  }
  const effectiveLocal = maxMergeEntries(local, ownSnapshots);
  return { merged: mergeEntries(effectiveLocal, remote), machineMap };
}

const byLabel = (entries: UsageEntry[], label: string) => entries.find((e) => e.label === label);

describe("self-view max-merge pipeline (purged local + own snapshots + remote)", () => {
  beforeEach(() => mkdirSync(TEST_DIR, { recursive: true }));
  afterEach(() => rmSync(TEST_DIR, { recursive: true, force: true }));

  it("resurfaces a purged date from the own snapshot without double-counting", () => {
    // Intake's real numbers: own snapshot $236.00, remote machine $308.12,
    // post-purge live residue $9.46.
    seedSnapshot(MACHINE, entry("2026-04-24", 236.0, 9999));
    seedSnapshot("devws", entry("2026-04-24", 308.12));
    const local = [entry("2026-04-24", 9.46, 12)];

    const { merged } = mergedPipeline(local);
    const day = byLabel(merged, "2026-04-24");
    assert.ok(day);
    // 236.00 + 308.12 — NOT 9.46 + 308.12 (blind spot) and NOT
    // 9.46 + 236.00 + 308.12 (double count of surviving transcripts)
    assert.equal(day.totalCost.toFixed(2), "544.12");
  });

  it("keeps the live view dominant inside the live window (today keeps growing)", () => {
    seedSnapshot(MACHINE, entry("2026-06-10", 3.0, 100));
    const local = [entry("2026-06-10", 5.0, 500)];

    const { merged } = mergedPipeline(local);
    const today = byLabel(merged, "2026-06-10");
    assert.ok(today);
    assert.equal(today.totalCost, 5.0);
    assert.equal(today.totalTokens, 500); // whole live entry, not the stale snapshot

    // The grow-write went through: the repo snapshot now holds the live value
    const file = join(TEST_DIR, USER, "2026", MACHINE, "cc-2026-06-10.jsonl");
    assert.equal(JSON.parse(readFileSync(file, "utf-8").trim()).totalCost, 5.0);
  });

  it("write guard keeps the own snapshot intact while the merge resurfaces it", () => {
    seedSnapshot(MACHINE, entry("2026-04-24", 236.0));
    const local = [entry("2026-04-24", 9.46)];

    mergedPipeline(local);

    // The residual live entry must NOT have overwritten the snapshot file
    const file = join(TEST_DIR, USER, "2026", MACHINE, "cc-2026-04-24.jsonl");
    assert.equal(JSON.parse(readFileSync(file, "utf-8").trim()).totalCost, 236.0);
  });

  it("carries the whole snapshot entry (token fields included), not a chimera", () => {
    seedSnapshot(MACHINE, { ...entry("2026-04-24", 236.0), totalTokens: 8888, outputTokens: 777 });
    const local = [{ ...entry("2026-04-24", 9.46), totalTokens: 12, outputTokens: 3 }];

    const { merged } = mergedPipeline(local);
    const day = byLabel(merged, "2026-04-24");
    assert.ok(day);
    assert.equal(day.totalTokens, 8888);
    assert.equal(day.outputTokens, 777);
  });

  it("shows the corrected own-machine column for --by-machine", () => {
    seedSnapshot(MACHINE, entry("2026-04-24", 236.0));
    seedSnapshot("devws", entry("2026-04-24", 308.12));
    const local = [entry("2026-04-24", 9.46), entry("2026-06-10", 5.0)];

    const { machineMap } = mergedPipeline(local);
    const own = machineMap.get(MACHINE);
    const devws = machineMap.get("devws");
    assert.ok(own && devws);
    assert.equal(byLabel(own, "2026-04-24")?.totalCost, 236.0); // corrected, not 9.46
    assert.equal(byLabel(own, "2026-06-10")?.totalCost, 5.0); // live window untouched
    assert.equal(byLabel(devws, "2026-04-24")?.totalCost, 308.12);
  });

  it("behaves as before for a fresh machine with no own snapshots", () => {
    seedSnapshot("devws", entry("2026-06-09", 2.0));
    const local = [entry("2026-06-10", 1.0)];

    const { merged, machineMap } = mergedPipeline(local);
    assert.equal(byLabel(merged, "2026-06-10")?.totalCost, 1.0);
    assert.equal(byLabel(merged, "2026-06-09")?.totalCost, 2.0);
    // effectiveLocal degenerates to local... plus the file writeMetrics just
    // wrote for today, which is the same entry — still exactly local.
    assert.deepEqual(machineMap.get(MACHINE), local);
  });

  it("merged totals never decrease relative to the pre-change pipeline", () => {
    // Pre-change: mergeEntries(local, remote-excluding-own). The new pipeline
    // adds max(local, own) — which is >= local per label — so every merged
    // label total is >= the old one.
    seedSnapshot(MACHINE, entry("2026-04-24", 236.0));
    seedSnapshot(MACHINE, entry("2026-05-03", 40.0));
    seedSnapshot("devws", entry("2026-04-24", 308.12));
    const local = [entry("2026-04-24", 9.46), entry("2026-05-03", 41.0), entry("2026-06-10", 5.0)];

    const oldRemote = readRemoteEntriesByMachine(TEST_DIR, USER, MACHINE, TOOL);
    const oldFlat: UsageEntry[] = [];
    for (const machineEntries of oldRemote.values()) oldFlat.push(...machineEntries);
    const oldMerged = mergeEntries(local, oldFlat);

    const { merged } = mergedPipeline(local);
    for (const oldEntry of oldMerged) {
      const newEntry = byLabel(merged, oldEntry.label);
      assert.ok(newEntry, `label ${oldEntry.label} missing from new merge`);
      assert.ok(
        newEntry.totalCost >= oldEntry.totalCost,
        `${oldEntry.label}: ${newEntry.totalCost} < ${oldEntry.totalCost}`,
      );
    }
  });
});
