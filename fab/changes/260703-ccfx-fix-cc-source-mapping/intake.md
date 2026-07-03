# Intake: Fix tu cc source mapping for ccusage v20

**Change**: 260703-ccfx-fix-cc-source-mapping
**Created**: 2026-07-03

## Origin

Backlog item `[ccfx]` (2026-07-03), invoked via `/fab-new ccfx` (one-shot backlog intake, no prior conversation). Raw backlog entry:

> Fix tu cc source mapping for ccusage v20: bare `ccusage daily` is now an ALL-AGENTS aggregate (JSON has agent: "all" + metadata.agents listing detected agents; --all is a compat no-op), so TOOLS.cc with prefixArgs [] (src/node/core/fetcher.ts) mislabels all detected agents as Claude Code — on machines with local codex/opencode/gemini transcripts, tu cc over-counts and the all-tools total DOUBLE-COUNTS (codex usage lands under both the cc and codex columns). Missed in the v20 migration (fab change 260703-bxuh / PR #38); currently latent on dev-ws-sahil02 (only claude data detected, so numbers match today). FIX: change cc prefixArgs to ["claude"]; introduce per-tool JSON label key — ccusage claude daily emits the date under key "date" (ISO, e.g. 2026-07-01) while bare/codex/opencode emit "period" — add a labelKey field to ToolConfig (cc: "date", codex/oc: "period"), replacing the LABEL_KEY const lookups in fetchHistory/pickCurrentEntry (fetcher.ts). normalizeLabel already passes ISO through unchanged, so no format work. Tests: fetcher tests — registry shape + label-key fixtures for both key spellings. Hydrate: docs/memory/cli/data-pipeline.md documents cc -> [] and the single "period" label key — both claims need correcting. Verify: on a claude-only machine, npx ccusage claude daily --json totals must equal bare npx ccusage daily --json totals, and tu cc output must be unchanged; composed argv should read: ccusage claude daily --json.

During intake, the JSON-shape claims were **verified live** against ccusage v20 (via `npx -y ccusage@latest` on this machine, which has only Claude Code transcripts):

- `ccusage claude daily --json` → entries carry the ISO label under key `"date"` (e.g. `"date": "2026-06-01"`), claude-only data
- bare `ccusage daily --json` → entries carry the ISO label under key `"period"`, plus `"agent": "all"` and `"metadata": {"agents": ["claude"]}` — confirming the all-agents-aggregate behavior
- `ccusage codex daily --json` → empty on this machine (no codex transcripts); the `"period"` key for codex/opencode rests on the shipped v20 migration behavior (change 260703-bxuh), where the current code parses subcommand output with `"period"` successfully

## Why

**Problem.** ccusage v20 changed the semantics of the bare `ccusage daily` invocation: it is now an aggregate across *all detected agents* (claude, codex, opencode, gemini, …), with `--all` reduced to a compatibility no-op. `tu`'s `TOOLS.cc` entry still uses `prefixArgs: []` (bare invocation), so the "Claude Code" column actually reports the all-agents aggregate. Two concrete failures on any machine with non-claude transcripts:

1. **Over-count**: `tu cc` reports codex/opencode/gemini usage as Claude Code.
2. **Double-count**: the all-tools total counts codex usage twice — once inside the mislabeled `cc` column and once in the real `codex` column.

**Consequence if not fixed.** The bug is latent on dev-ws-sahil02 today (only claude transcripts are detected, so aggregate == claude and numbers happen to match), but it silently corrupts numbers the moment any other agent's transcripts appear locally — and cost accuracy is the tool's entire purpose (Constitution I). It also **blocks backlog item `[gmcp]`** (add gemini + copilot sources): adding more sources before this fix worsens the double-counting, and `[gmcp]` reuses the per-tool `labelKey` mechanism this change introduces.

**Why this approach.** ccusage v20 provides a first-class per-agent surface: the `claude` subcommand, exactly parallel to the existing `codex`/`opencode` subcommands already expressed via `prefixArgs`. Mapping `cc → ["claude"]` reuses the existing registry mechanism with a one-line change (minimum pathways). The alternative — client-side filtering of the bare aggregate via `metadata.agents` — was rejected in the backlog design: it adds a parsing path for data ccusage already serves cleanly per-agent. The only wrinkle is that the `claude` subcommand emits its date label under a different JSON key (`"date"`) than bare/codex/opencode (`"period"`), which forces the label-key lookup to become per-tool instead of per-period.

## What Changes

### 1. TOOLS registry: `cc` maps to the `claude` subcommand (`src/node/core/fetcher.ts`)

Change `cc.prefixArgs` from `[]` to `["claude"]`, and add the new `labelKey` field to all three entries (see §2):

```ts
export const TOOLS: Record<string, ToolConfig> = {
  cc: {
    name: "Claude Code",
    binary: CCUSAGE,
    prefixArgs: ["claude"],   // was: []
    labelKey: "date",         // new — claude subcommand emits ISO label under "date"
    needsFilter: false,
  },
  codex: {
    name: "Codex",
    binary: CCUSAGE,
    prefixArgs: ["codex"],
    labelKey: "period",       // new
    needsFilter: true,
  },
  oc: {
    name: "OpenCode",
    binary: CCUSAGE,
    prefixArgs: ["opencode"],
    labelKey: "period",       // new
    needsFilter: true,
  },
};
```

`runTool`'s argv composition (`[...tool.prefixArgs, period, "--json", ...extraArgs]`) is unchanged; for `cc` it now composes `ccusage claude daily --json`. The comment block above `TOOLS` (currently "Per-tool subcommands (codex/opencode) are expressed via prefixArgs") should be updated to reflect that all three tools now use subcommands.

### 2. Per-tool JSON label key: `labelKey` on `ToolConfig`, replacing the `LABEL_KEY` const

**`src/node/core/types.ts`** — add the field:

```ts
export interface ToolConfig {
  name: string;
  binary: string;
  prefixArgs: string[];
  labelKey: string;   // JSON key carrying the entry's ISO date label ("date" | "period")
  needsFilter: boolean;
}
```

**`src/node/core/fetcher.ts`** — remove the period-keyed const:

```ts
// removed:
const LABEL_KEY: Record<string, string> = { daily: "period", monthly: "period" };
```

and replace its two lookups with the tool's `labelKey`:

- `fetchHistory`: `entries.map((e) => toUsageEntry(e, LABEL_KEY.daily))` → `entries.map((e) => toUsageEntry(e, tool.labelKey))`
- `pickCurrentEntry`: currently derives `const labelKey = LABEL_KEY[period] || "period"` internally. The tool identity is not in scope there — its caller `fetchTotals` has the `tool`. Thread the label key in as a parameter (e.g. `pickCurrentEntry(entries, period, now, labelKey)` with `"period"` as the default to keep the signature ergonomic for tests), and have `fetchTotals` pass `tool.labelKey`. Exact parameter shape is the implementer's call — the requirement is that the raw-JSON label lookup uses the tool's `labelKey`, not a period-keyed const.

Only raw ccusage JSON is affected: all fetches run `period = "daily"` (monthly is aggregated client-side from `UsageEntry.label`, which is already normalized ISO), so `labelKey` applies exactly where raw entries are parsed — `fetchHistory` and the `fetchTotals → pickCurrentEntry` path. `normalizeLabel` passes ISO strings through unchanged, so no label-format work is needed for either key spelling. The stale comment above the removed `LABEL_KEY` const (describing v20's single `"period"` key) moves/updates to describe the per-tool split.

### 3. Tests (`src/node/core/__tests__/fetcher.test.ts`)

- **Registry shape**: assert `TOOLS.cc.prefixArgs` is `["claude"]` and `labelKey` values are `cc: "date"`, `codex: "period"`, `oc: "period"` (extends whatever registry assertions exist from 260703-bxuh).
- **Label-key fixtures for both spellings**: entries keyed `"date"` (ISO) and `"period"` (ISO) both parse to correct `UsageEntry.label` values through `toUsageEntry` / `pickCurrentEntry` / the `fetchHistory` mapping.
- Update existing `pickCurrentEntry` tests if its signature changes (they currently call `pickCurrentEntry(entries, period, now)` with `"period"`-keyed fixtures — a defaulted fourth parameter keeps them valid).

### 4. Verification criteria (from the backlog, machine-checkable)

On a claude-only machine (e.g. dev-ws-sahil02):

1. `npx ccusage claude daily --json` totals equal bare `npx ccusage daily --json` totals (aggregate == claude when only claude is detected)
2. `tu cc` output is unchanged before vs. after this fix
3. The composed argv for `cc` reads: `ccusage claude daily --json`

## Affected Memory

- `cli/data-pipeline`: (modify) two stale claims need correcting: (a) the `TOOLS` registry bullet documents `cc → []` prefixArgs — now `cc → ["claude"]`, all three tools use subcommands; (b) the label-key bullet documents a single `"period"` key via the `LABEL_KEY` const for both periods — now a per-tool `labelKey` field on `ToolConfig` (`cc: "date"`, `codex`/`oc`: `"period"`), with `fetchHistory`/`pickCurrentEntry` reading the tool's key. Both bullets carry `(260703-bxuh)` attributions that this change supersedes.

## Impact

- **`src/node/core/fetcher.ts`** — `TOOLS` registry (cc prefixArgs + new labelKey fields), `LABEL_KEY` const removed, `fetchHistory` mapping, `pickCurrentEntry` label lookup, `fetchTotals` call site. `toUsageEntry` already takes `labelKey` as a parameter — unchanged.
- **`src/node/core/types.ts`** — `ToolConfig` gains `labelKey: string`.
- **`src/node/core/__tests__/fetcher.test.ts`** — registry shape + label-key fixtures; possible signature update for `pickCurrentEntry` tests.
- **No consumer fallout**: `cli.ts`/`sync.ts` consume `Object.keys(TOOLS)` and `TOOLS[k].name` only; `ToolConfig` gains a field (no removals), so no other call sites change.
- **Cache**: `~/.tu/cache/cc-daily.json` may hold one stale (aggregate) fetch for up to the 60s TTL after upgrade — self-heals, no migration needed.
- **Output stability (Constitution)**: no layout/format change; numbers change only on multi-agent machines where they were wrong (correctness fix). `fix` change type, patch-level.
- **Unblocks**: backlog `[gmcp]` (gemini + copilot sources), which is explicitly blocked on this change and reuses `labelKey`.

## Open Questions

*None — the backlog entry fully specifies the fix, and the JSON-shape claims were verified live during intake.*

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | `cc.prefixArgs` changes from `[]` to `["claude"]` (composed argv: `ccusage claude daily --json`) | Explicit in backlog; `claude` subcommand verified live to exist and emit claude-only data; parallel to existing codex/oc subcommand mechanism | S:95 R:90 A:95 D:95 |
| 2 | Certain | Per-tool `labelKey: string` field on `ToolConfig` (`cc: "date"`, `codex`/`oc`: `"period"`) replaces the period-keyed `LABEL_KEY` const | Explicit in backlog with exact field name and values; the two key spellings make a per-tool field the only clean shape | S:95 R:85 A:90 D:85 |
| 3 | Certain | `ccusage claude daily --json` emits the ISO date label under key `"date"` | Verified live this session (`"date": "2026-06-01"`); bare daily verified to emit `"period"` + `agent: "all"` | S:90 R:95 A:100 D:100 |
| 4 | Certain | `codex`/`opencode` subcommands emit the label under `"period"` | Not verifiable live (no codex/oc transcripts on this machine) but this is the shipped, working v20 behavior — current code parses subcommand output with `"period"` (260703-bxuh, PR #38) | S:85 R:85 A:80 D:90 |
| 5 | Confident | `pickCurrentEntry` takes the label key from its caller (defaulted parameter, `fetchTotals` passes `tool.labelKey`) rather than deriving it internally | Backlog names pickCurrentEntry as a replacement site but not the exact signature; tool identity isn't in scope inside the function, so threading via parameter is the obvious shape; a defaulted param keeps existing tests valid — implementer may adjust | S:80 R:85 A:80 D:70 |
| 6 | Certain | Tests extend `src/node/core/__tests__/fetcher.test.ts`: registry-shape assertions + label-key fixtures for both `"date"` and `"period"` spellings | Explicit in backlog; test file exists at the co-located path mandated by the constitution | S:90 R:90 A:85 D:85 |
| 7 | Certain | Hydrate corrects two stale claims in `docs/memory/cli/data-pipeline.md` (cc `[]` prefixArgs; single `"period"` label key) | Explicit in backlog; both stale bullets located and confirmed in the memory file during intake | S:90 R:95 A:90 D:90 |

7 assumptions (6 certain, 1 confident, 0 tentative, 0 unresolved).
