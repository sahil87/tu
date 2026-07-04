# Intake: Add Gemini + Copilot Sources

**Change**: 260703-gmcp-add-gemini-copilot-sources
**Created**: 2026-07-03

## Origin

> /fab-new gmcp — backlog item `[gmcp]` (fab/backlog.md, 2026-07-03):
>
> "Add gemini + copilot sources to tu. BLOCKED BY [ccfx]: until cc maps to the ccusage claude subcommand, bare-daily all-agents aggregation means adding these sources WORSENS double-counting; also reuses the per-tool labelKey mechanism [ccfx] introduces. ccusage v20 ships gemini and copilot subcommands with the standard daily --json surface. SCOPE: (1) TOOLS registry … labelKey "period" (VERIFY against real JSON output or an upstream ccusage fixture …) … (2) cli.ts KNOWN_SOURCES + SOURCE_ALIASES — suggest gem and cop … (3) completions.ts … (4) Display: all-tools table and watch layouts grow to 5 tool columns … (5) Multi-mode: … sync works unchanged … Constitution Output Stability: new table columns = breaking output change, needs a MINOR version bump."

Interactive intake. Key events during intake:

- **Blocker resolved**: `[ccfx]` (260703-ccfx-fix-cc-source-mapping) merged to main as PR #39 during this intake; the branch was rebased onto origin/main (which also brought in PR #40, the `[sntl]` since/until filters), and the interim cherry-pick commit was dropped as redundant.
- **The backlog's VERIFY item was executed and falsified the guess**: the vendored ccusage v20.0.14 was installed and inspected together with the upstream source at tag v20.0.14. ALL per-agent subcommands (`claude`, `codex`, `opencode`, `gemini`, `copilot`) emit the daily label under **`"date"`** — only the bare all-agents aggregate emits `"period"`. Evidence: `rust/crates/ccusage/src/adapter/opencode/report.rs` `period_key(Daily) = "date"` (shared serializer `agent_summary_json`, which `gemini/report.rs` and `copilot/report.rs` both delegate to), `codex/report.rs` has its own identical `Daily => "date"` mapping, the codex snapshot fixture shows `"date": "2026-01-02"`, and `adapter/all/report.rs` is the only serializer emitting `"period"`. Locally: `ccusage claude daily --json` entries carry `"date"`; gemini/copilot emit an empty `daily: []` on this machine (no transcripts), matching the backlog's caveat.
- **Two SRAD questions asked and answered**: (1) the latent codex/oc `labelKey: "period"` bug (introduced by ccfx, explicitly not live-verified there) is fixed **in this change**, not by amending ccfx or a third change; (2) the 5-tool pivot uses **variable-width tool columns** so it fits 80-col terminals.

## Why

1. **Pain point**: tu tracks Claude Code, Codex, and OpenCode only. Users running Gemini CLI or GitHub Copilot CLI have no cost visibility in tu, even though the vendored ccusage v20.0.14 binary already parses both (first-class `gemini` and `copilot` subcommands with the standard `daily --json` surface). The marginal cost of adding sources is small: the `TOOLS` registry + per-tool `labelKey` mechanism (from ccfx) is exactly the extension seam.
2. **Consequence if not done**: gemini/copilot usage remains invisible — and because bare `ccusage daily` aggregates all agents, a user who works around it manually gets numbers tu can't reconcile. Cost accuracy/coverage is the tool's entire purpose (Constitution I).
3. **Why now**: the `[ccfx]` blocker is merged — `cc` maps to the `claude` subcommand, so adding sources no longer worsens double-counting.
4. **Bundled correction (user-approved)**: intake verification found codex/oc `labelKey: "period"` is wrong at v20.0.14 — per-agent subcommands emit `"date"`. On any machine with codex/opencode transcripts, `toUsageEntry` reads `t["period"]` → `undefined` → every entry's label becomes `""`, silently breaking history rows, monthly aggregation, and snapshot pick. Latent on this fleet (no codex/oc transcripts anywhere). This change touches the exact registry lines, so the 2-line correction + test/memory updates land here rather than in a third PR.

## What Changes

### 1. `TOOLS` registry — add gemini + copilot, fix codex/oc labelKey (`src/node/core/fetcher.ts`)

Add two entries; correct two `labelKey` values:

```ts
export const TOOLS: Record<string, ToolConfig> = {
  cc:    { name: "Claude Code", binary: CCUSAGE, prefixArgs: ["claude"],   labelKey: "date", needsFilter: false },
  codex: { name: "Codex",       binary: CCUSAGE, prefixArgs: ["codex"],    labelKey: "date", needsFilter: true  },  // labelKey was "period" — wrong at v20.0.14
  oc:    { name: "OpenCode",    binary: CCUSAGE, prefixArgs: ["opencode"], labelKey: "date", needsFilter: true  },  // labelKey was "period" — wrong at v20.0.14
  gemini:  { name: "Gemini",  binary: CCUSAGE, prefixArgs: ["gemini"],  labelKey: "date", needsFilter: false },     // new
  copilot: { name: "Copilot", binary: CCUSAGE, prefixArgs: ["copilot"], labelKey: "date", needsFilter: false },     // new
};
```

- All five per-agent subcommands emit the daily label under `"date"` (verified — see Origin). Only the bare all-agents aggregate (which tu no longer calls) uses `"period"`.
- The `labelKey` **mechanism stays** (it is one release old and correctly models "key varies by serializer"), but the comment block above `TOOLS` and near `toUsageEntry`/`pickCurrentEntry` (fetcher.ts:59-65, 159-162, 173-176) currently claims codex/opencode emit `"period"` — rewrite to: per-agent subcommands emit `"date"`; bare aggregate (unused) emits `"period"`.
- `pickCurrentEntry`'s defaulted 4th parameter (`labelKey = "period"`) is the implementer's call: keeping it preserves period-keyed test call sites; switching the default to `"date"` better reflects the registry. Either way, all real callers pass `tool.labelKey` explicitly.
- `needsFilter: false` for the new tools (v20 emits clean JSON; `stripNoise` is a defensive no-op). codex/oc keep `needsFilter: true` — flipping them is out of scope (harmless either way).
- `ToolConfig` (types.ts) already has all needed fields — no type change.
- Registry order determines column order in all-tools views: cc, codex, oc, gemini, copilot.

### 2. Source grammar (`src/node/core/cli.ts`)

- `KNOWN_SOURCES` (cli.ts:751): add `"gemini"`, `"gem"`, `"copilot"`, `"cop"` → `new Set(["cc", "codex", "co", "oc", "gemini", "gem", "copilot", "cop", "all"])`.
- `SOURCE_ALIASES` (cli.ts:752): add `gem: "gemini"`, `cop: "copilot"` (`co` is taken by codex; `cop` avoids collision).
- `FULL_HELP` Sources line (cli.ts:76) becomes:
  `Sources: cc (Claude Code), codex/co (Codex), oc (OpenCode), gemini/gem (Gemini), copilot/cop (Copilot), all (default)`
- No dispatch changes: single-tool and all-tools paths iterate `TOOLS` / take the resolved source key, so `tu gemini`, `tu gem dh`, `tu copilot mh` etc. work once the registry and grammar know the keys.
- The help-dump / shll.ai contract picks the new help text up automatically (raw `FULL_HELP` passthrough; additive change, no drift concern).

### 3. Shell completions (`src/node/core/completions.ts`)

Add the four new source tokens to all three scripts:

- bash (line 20): `local sources="cc codex co oc gemini gem copilot cop all"`
- zsh (line 74): `sources=(cc codex co oc gemini gem copilot cop all)`
- fish (~line 150): four new `complete -c tu -n '__fish_use_subcommand' -a ...` lines — `gemini` "Gemini", `gem` "Gemini (alias)", `copilot` "Copilot", `cop` "Copilot (alias)" — matching the existing `codex`/`co` description pattern.

### 4. Display — variable-width pivot columns (`src/node/tui/formatter.ts`)

The cross-tool pivot `renderTotalHistory` uses a fixed per-tool column width `N = 14`. With 5 tools: `12 + 5×(14+3) = 97` chars before the cost column (~108 total) — overflows 80-col terminals (3 tools ≈ 74, fits). **User-selected fix: size each tool column individually** to `max(toolName.length, 9)` (9 ≈ widest typical cost cell, e.g. `$1,234.56`):

```
Date         | Claude Code |    Codex | OpenCode |   Gemini |  Copilot | Cost
─────────────|─────────────|──────────|──────────|──────────|──────────|─────
2026-07-01   |     $123.45 |    $0.12 |    $4.56 |    $0.00 |    $0.00 | $128.13
```

≈75 chars — all 5 tools fit at 80 cols. Implementation notes:

- `row`/`colorRow`/`divStr` builders and `tableWidth` in `renderTotalHistory` switch from the constant `N` to a per-column width array; the bar-area calculation (`barWidth = width - tableWidth - …`) works unchanged once `tableWidth` reflects real widths.
- The machine-cost variant of the width calculation (formatter.ts:120) must use the same per-column widths.
- Tools with zero data still get a column (fetchers iterate the whole `TOOLS` registry — `fetchAllHistory`/`fetchAllTotals`); `$0.00` columns for gemini/copilot are expected and intended.
- `renderTotal` (snapshot: Tool/Tokens/Input/Output/Cost rows) grows by 2 **rows**, not columns — no width work.
- Watch mode: the snapshot panel grows 2 rows; the history area uses compact mode (date + total only) below width 60 and `maxRows` truncation — verify rendering at 80 cols but no structural change expected. CSV/Markdown emitters take headers from tool names and are width-agnostic (columns grow naturally).
- `docs/specs/layouts.md` mockups for the cross-tool pivot (and any layout showing tool columns/rows) must be updated to the 5-tool shape.

### 5. Version + release

Constitution Output Stability: new table columns are a breaking output change → the release that ships this needs a **MINOR** bump (0.6.0 → 0.7.0). Recorded here for the ship stage; no code task.

### 6. Multi-mode / sync

No changes. Metric files are keyed by toolKey (`{user}/{year}/{machine}/{toolKey}-*.jsonl`); sync handles the two new keys automatically. Machines running older tu will not read the new tool keys — harmless, they just omit those columns.

### 7. Documentation + memory corrections

- `docs/memory/cli/data-pipeline.md`: registry claims gain gemini/copilot; the label-key bullets (requirement + design-decision sections) currently state codex/oc emit `"period"` — correct to: all per-agent subcommands emit `"date"`; only the bare (unused) aggregate emits `"period"`. The stale comment in `src/node/core/__tests__/fetcher.test.ts` (~line 218, "ccusage@20 emits the ISO label under `period`") gets the same correction.
- `docs/memory/display/formatting.md`: pivot column rule changes from fixed 14-char to variable per-tool width.
- `docs/specs/usage.md`: sources grammar line gains gemini/gem, copilot/cop.
- `docs/specs/layouts.md`: 5-tool mockups (per §4).

## Affected Memory

- `cli/data-pipeline`: (modify) TOOLS registry gains gemini/copilot entries; labelKey claims corrected — all per-agent subcommands emit `"date"`, codex/oc `"period"` was a ccfx-era error; only the bare aggregate uses `"period"`
- `display/formatting`: (modify) cross-tool pivot column width rule: fixed 14-char → variable per-tool `max(name, 9)`; 5-tool table fits 80 cols

## Impact

- **Source**: `src/node/core/fetcher.ts` (registry + comments), `src/node/core/cli.ts` (KNOWN_SOURCES, SOURCE_ALIASES, FULL_HELP), `src/node/core/completions.ts` (3 scripts), `src/node/tui/formatter.ts` (variable-width pivot). No changes: `types.ts`, `sync.ts`, `watch.ts` (verify only).
- **Tests** (runner: `npx tsx --test`, co-located `__tests__/`):
  - `src/node/core/__tests__/fetcher.test.ts`: registry shape (5 entries; labelKey `"date"` on all; prefixArgs; needsFilter) — updates the ccfx assertions that pin codex/oc to `"period"`; date-keyed fixtures for the label-key paths (`toUsageEntry`/`pickCurrentEntry`/`fetchHistory` mapping); fix the stale `"period"` comment.
  - `src/node/core/__tests__/cli-parser.test.ts`: `parseDataArgs` accepts `gemini`/`gem`/`copilot`/`cop` (alias resolution included).
  - `src/node/core/__tests__/completions.test.ts`: new tokens present in all three scripts.
  - `src/node/core/__tests__/cli-help.test.ts` (if it pins the Sources line) + help-dump test: additive help text.
  - `src/node/tui/__tests__/formatter.test.ts` / `formatter-options.test.ts`: 5-tool pivot renders within 80 cols; per-column widths; existing 2–3-tool cases stay green.
- **Docs**: `docs/specs/usage.md`, `docs/specs/layouts.md`; memory per Affected Memory.
- **Release**: MINOR version bump when shipped.
- **Merge adjacency**: branch is rebased on main including `[sntl]` (PR #40) — the FULL_HELP/completions/tests adjacency the backlog warned about is already reconciled.

## Open Questions

*(none — both intake-time decision points were asked and resolved; see Assumptions #2 and #3)*

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | gemini/copilot `labelKey: "date"` (backlog guessed `"period"`) | Verified at v20.0.14: shared serializer `agent_summary_json` (`period_key(Daily)="date"`) used by opencode/gemini/copilot; codex has identical mapping; snapshot fixture shows `"date"`; only the bare aggregate emits `"period"` | S:90 R:85 A:95 D:95 |
| 2 | Certain | Fix latent codex/oc `labelKey: "period"` bug in this change (not by amending merged ccfx or a third PR) | Asked — user chose gmcp inclusion; ccfx merged mid-intake making PR-amendment moot; this change edits the same registry lines | S:90 R:75 A:90 D:90 |
| 3 | Certain | 5-tool pivot uses variable-width tool columns (`max(name, 9)`), keeping ≈75 chars at 80 cols | Asked — user chose variable-width over fixed-14 (overflows 80) and over hiding zero-data columns (machine-dependent output) | S:90 R:80 A:85 D:90 |
| 4 | Certain | Aliases `gem` → gemini, `cop` → copilot | Backlog explicitly suggests both; `co` is taken by codex; trivially reversible grammar addition | S:75 R:90 A:85 D:70 |
| 5 | Certain | `needsFilter: false` for gemini/copilot; codex/oc keep `true` | Backlog explicit (v20 emits clean JSON; stripNoise is a defensive no-op); flipping codex/oc is out of scope | S:85 R:95 A:90 D:85 |
| 6 | Certain | Display names `Gemini` / `Copilot` (not "Gemini CLI" / "GitHub Copilot") | Backlog names them; short names aid column fit; consistent with `Codex`/`OpenCode` brevity | S:75 R:85 A:85 D:75 |
| 7 | Confident | Registry/column order: cc, codex, oc, gemini, copilot (new tools appended) | Preserves existing column order for current users (Output Stability); insertion order is the only signal | S:60 R:85 A:80 D:75 |
| 8 | Confident | `pickCurrentEntry` default `labelKey` parameter left to implementer (keep `"period"` for legacy fixtures or flip to `"date"`) | All production callers pass `tool.labelKey` explicitly; the default only affects test-call-site ergonomics | S:55 R:90 A:80 D:60 |
| 9 | Certain | Zero-data tools still render `$0.00` columns (fetch-all iterates full registry) | Direct consequence of user-selected option in #3; existing fetcher behavior unchanged | S:85 R:80 A:90 D:90 |

9 assumptions (7 certain, 2 confident, 0 tentative, 0 unresolved).
