import { describe, it, beforeEach, afterEach } from "node:test";
import assert from "node:assert/strict";
import { mkdirSync, writeFileSync, rmSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { readRemoteEntriesByMachine } from "../sync.js";

const TMP = join(tmpdir(), ".tmp-by-machine-test");

function writeEntry(dir: string, file: string, entry: object): void {
  mkdirSync(dir, { recursive: true });
  writeFileSync(join(dir, file), JSON.stringify(entry) + "\n");
}

describe("readRemoteEntriesByMachine", () => {
  beforeEach(() => {
    rmSync(TMP, { recursive: true, force: true });
  });

  afterEach(() => {
    rmSync(TMP, { recursive: true, force: true });
  });

  it("groups entries by machine directory", () => {
    const macbookDir = join(TMP, "sahil", "2026", "macbook");
    const studioDir = join(TMP, "sahil", "2026", "studio");
    writeEntry(macbookDir, "cc-2026-03-06.jsonl", {
      label: "2026-03-06", totalCost: 1.90, inputTokens: 100, outputTokens: 200,
      cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 300,
    });
    writeEntry(studioDir, "cc-2026-03-06.jsonl", {
      label: "2026-03-06", totalCost: 0.95, inputTokens: 50, outputTokens: 100,
      cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 150,
    });

    const result = readRemoteEntriesByMachine(TMP, "sahil", null, "cc");
    assert.equal(result.size, 2);
    assert.ok(result.has("macbook"));
    assert.ok(result.has("studio"));
    assert.equal(result.get("macbook")!.length, 1);
    assert.equal(result.get("macbook")![0].totalCost, 1.90);
    assert.equal(result.get("studio")![0].totalCost, 0.95);
  });

  it("excludes specified machine", () => {
    const macbookDir = join(TMP, "sahil", "2026", "macbook");
    const studioDir = join(TMP, "sahil", "2026", "studio");
    writeEntry(macbookDir, "cc-2026-03-06.jsonl", {
      label: "2026-03-06", totalCost: 1.90, inputTokens: 100, outputTokens: 200,
      cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 300,
    });
    writeEntry(studioDir, "cc-2026-03-06.jsonl", {
      label: "2026-03-06", totalCost: 0.95, inputTokens: 50, outputTokens: 100,
      cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 150,
    });

    const result = readRemoteEntriesByMachine(TMP, "sahil", "macbook", "cc");
    assert.equal(result.size, 1);
    assert.ok(result.has("studio"));
    assert.ok(!result.has("macbook"));
  });

  it("returns empty map when user path does not exist", () => {
    const result = readRemoteEntriesByMachine(TMP, "nonexistent", null, "cc");
    assert.equal(result.size, 0);
  });

  it("filters by tool key prefix", () => {
    const macbookDir = join(TMP, "sahil", "2026", "macbook");
    writeEntry(macbookDir, "cc-2026-03-06.jsonl", {
      label: "2026-03-06", totalCost: 1.90, inputTokens: 100, outputTokens: 200,
      cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 300,
    });
    writeEntry(macbookDir, "codex-2026-03-06.jsonl", {
      label: "2026-03-06", totalCost: 3.00, inputTokens: 200, outputTokens: 400,
      cacheCreationTokens: 0, cacheReadTokens: 0, totalTokens: 600,
    });

    const ccResult = readRemoteEntriesByMachine(TMP, "sahil", null, "cc");
    assert.equal(ccResult.get("macbook")!.length, 1);
    assert.equal(ccResult.get("macbook")![0].totalCost, 1.90);

    const codexResult = readRemoteEntriesByMachine(TMP, "sahil", null, "codex");
    assert.equal(codexResult.get("macbook")!.length, 1);
    assert.equal(codexResult.get("macbook")![0].totalCost, 3.00);
  });
});
