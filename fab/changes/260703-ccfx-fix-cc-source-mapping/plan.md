# Plan: Fix tu cc source mapping for ccusage v20

**Change**: 260703-ccfx-fix-cc-source-mapping
**Intake**: `intake.md`

## Requirements

### Data Pipeline: cc source mapping and per-tool label key

#### R1: `cc` maps to the `claude` subcommand
The `TOOLS.cc` entry in `src/node/core/fetcher.ts` MUST use `prefixArgs: ["claude"]` (was `[]`), so `runTool` composes `ccusage claude daily --json` — the per-agent claude surface, not the v20 all-agents aggregate.

- **GIVEN** ccusage v20, where bare `ccusage daily` is an all-agents aggregate (JSON carries `agent: "all"`) and `--all` is a compat no-op
- **WHEN** `runTool("cc", "daily")` composes the argv `[...tool.prefixArgs, "daily", "--json"]`
- **THEN** the argv is `["claude", "daily", "--json"]`, invoking the claude-only per-agent subcommand
- **AND** on a machine with local codex/opencode/gemini transcripts, the `cc` column reports only Claude Code usage (no over-count, no double-count in the all-tools total)

#### R2: Per-tool `labelKey` field on `ToolConfig` replaces the `LABEL_KEY` const
`ToolConfig` (`src/node/core/types.ts`) MUST gain a `labelKey: string` field carrying the JSON key under which an entry's ISO date label appears. The period-keyed `LABEL_KEY` const in `fetcher.ts` MUST be removed and its two lookups replaced with the tool's `labelKey`. Registry values: `cc: "date"`, `codex: "period"`, `oc: "period"`.

- **GIVEN** the `claude` subcommand emits its date label under key `"date"` while codex/opencode emit it under `"period"`
- **WHEN** `fetchHistory` maps raw entries and `pickCurrentEntry` looks up the current entry
- **THEN** each raw-JSON label lookup uses the tool's `labelKey` (`cc → "date"`, `codex`/`oc → "period"`), not a period-keyed const
- **AND** the removed `LABEL_KEY` const no longer exists in `fetcher.ts`

#### R3: `pickCurrentEntry` takes the label key from its caller
`pickCurrentEntry` MUST derive the raw-JSON label key from a caller-supplied value rather than an internal period-keyed const. `fetchTotals` MUST pass `tool.labelKey`. The parameter SHOULD default to `"period"` so existing tests calling `pickCurrentEntry(entries, period, now)` remain valid.

- **GIVEN** the tool identity is not in scope inside `pickCurrentEntry` but its caller `fetchTotals` holds the `tool`
- **WHEN** `fetchTotals` calls `pickCurrentEntry` for the `cc` tool
- **THEN** the label key threaded in is `"date"`, and the matched entry's ISO label resolves correctly
- **AND** a call omitting the label-key argument falls back to `"period"`

#### R4: Both label-key spellings parse correctly
Raw entries keyed under `"date"` (ISO) and under `"period"` (ISO) MUST both parse to correct `UsageEntry.label` values through `toUsageEntry`, `pickCurrentEntry`, and the `fetchHistory` mapping. `normalizeLabel` passes ISO strings through unchanged, so no label-format work is required for either spelling.

- **GIVEN** a raw entry `{ date: "2026-06-01", ... }` and a raw entry `{ period: "2026-06-01", ... }`
- **WHEN** each is parsed with its matching `labelKey`
- **THEN** both yield `UsageEntry.label === "2026-06-01"`

#### R5: No consumer fallout
Adding `labelKey` to `ToolConfig` MUST NOT break existing consumers. `cli.ts`/`sync.ts` consume `Object.keys(TOOLS)` and `TOOLS[k].name` only; `ToolConfig` gains a field (no removals), so no other call sites change. The project MUST typecheck under strict mode.

- **GIVEN** `ToolConfig` gains a required `labelKey` field
- **WHEN** the project is type-checked (`npx tsc --noEmit`)
- **THEN** compilation succeeds with no errors

### Design Decisions

1. **Reuse the registry `prefixArgs` mechanism for `cc`**: map `cc → ["claude"]` exactly parallel to the existing `codex`/`oc` subcommand entries — *Why*: ccusage v20 exposes a first-class per-agent `claude` subcommand; reusing the existing mechanism is a one-line registry change (minimum pathways) — *Rejected*: client-side filtering of the bare aggregate via `metadata.agents` (adds a parsing path for data ccusage already serves cleanly per-agent).
2. **Per-tool `labelKey` field, not a period-keyed const**: the `claude` subcommand emits the date under `"date"` while codex/opencode emit `"period"`, so the label key is now a property of the tool, not the period — *Why*: a per-tool field is the only clean shape when key spelling varies by tool — *Rejected*: keeping a `LABEL_KEY[period]` const (cannot distinguish tools that share the same period).
3. **Thread the label key into `pickCurrentEntry` as a defaulted parameter**: `pickCurrentEntry(entries, period, now, labelKey = "period")`, with `fetchTotals` passing `tool.labelKey` — *Why*: tool identity is not in scope inside `pickCurrentEntry`; a defaulted param keeps existing `pickCurrentEntry(entries, period, now)` test call sites valid — *Rejected*: passing the whole `tool` object (leaks the registry type into a pure helper), or re-deriving from a const (the const is being removed).

## Tasks

### Phase 1: Type change

- [x] T001 Add `labelKey: string` field to the `ToolConfig` interface in `src/node/core/types.ts`, documented as the JSON key carrying the entry's ISO date label (`"date" | "period"`) <!-- R2 -->

### Phase 2: Core Implementation

- [x] T002 In `src/node/core/fetcher.ts`, change `TOOLS.cc.prefixArgs` from `[]` to `["claude"]` and add `labelKey` to all three registry entries (`cc: "date"`, `codex: "period"`, `oc: "period"`); update the `TOOLS` comment block so it reflects that all three tools now use subcommands <!-- R1 -->
- [x] T003 In `src/node/core/fetcher.ts`, remove the `LABEL_KEY` const and its stale comment; update `fetchHistory`'s mapping to use `tool.labelKey` in place of `LABEL_KEY.daily` <!-- R2 -->
- [x] T004 In `src/node/core/fetcher.ts`, add a defaulted `labelKey = "period"` parameter to `pickCurrentEntry` (replacing the internal `LABEL_KEY[period] || "period"` derivation) and have `fetchTotals` pass `tool.labelKey` at the call site <!-- R3 -->

### Phase 3: Tests

- [x] T005 In `src/node/core/__tests__/fetcher.test.ts`, update the `TOOLS` registry assertions: `TOOLS.cc.prefixArgs` is `["claude"]`, and `labelKey` values are `cc: "date"`, `codex: "period"`, `oc: "period"` (adjust the existing `subcommand prefixArgs` test and the `runTool argv construction` shape check that currently assert `cc.prefixArgs === []`) <!-- R1 R2 -->
- [x] T006 In `src/node/core/__tests__/fetcher.test.ts`, add label-key fixtures proving both spellings parse: a `"date"`-keyed ISO entry and a `"period"`-keyed ISO entry both resolve to the correct `UsageEntry.label` through `toUsageEntry`, `pickCurrentEntry` (with the `"date"` key threaded in), and the `fetchHistory` mapping shape <!-- R4 -->

### Phase 4: Verify

- [x] T007 Run the fetcher test suite (`npx tsx --test src/node/core/__tests__/fetcher.test.ts`) and `npx tsc --noEmit`; confirm both pass <!-- R5 -->

## Execution Order

- T001 precedes T002-T004 (the `labelKey` field must exist before the registry populates it and `fetchTotals` reads it)
- T002-T004 all edit `fetcher.ts` — apply sequentially
- T005-T006 depend on the implementation (T001-T004)
- T007 last

## Acceptance

### Functional Completeness

- [x] A-001 R1: `TOOLS.cc.prefixArgs` is `["claude"]`; `runTool` composes `ccusage claude daily --json` for `cc`
- [x] A-002 R2: `ToolConfig` has a `labelKey: string` field; the `LABEL_KEY` const is gone from `fetcher.ts`; registry values are `cc: "date"`, `codex: "period"`, `oc: "period"`
- [x] A-003 R3: `pickCurrentEntry` accepts a defaulted label-key parameter and `fetchTotals` passes `tool.labelKey`

### Behavioral Correctness

- [x] A-004 R1: On a claude-only machine, `tu cc` output (and the composed argv `ccusage claude daily --json`) equals the pre-fix output — the aggregate equalled claude only because claude was the sole detected agent (verified live: bare and `claude` daily totals identical)
- [x] A-005 R2: `fetchHistory` maps with `tool.labelKey` and `pickCurrentEntry`'s label lookup uses the threaded key, not a period-keyed const

### Scenario Coverage

- [x] A-006 R4: Tests prove a `"date"`-keyed ISO entry and a `"period"`-keyed ISO entry both yield the correct `UsageEntry.label`
- [x] A-007 R1: A registry-shape test asserts `cc.prefixArgs === ["claude"]` and the three `labelKey` values

### Edge Cases & Error Handling

- [x] A-008 R3: Existing `pickCurrentEntry(entries, period, now)` call sites (tests) remain valid via the defaulted `"period"` label-key parameter

### Code Quality

- [x] A-009 Pattern consistency: New code follows the registry / `ToolConfig` conventions of surrounding code (functions and plain objects, `type` imports, `.js` extensions)
- [x] A-010 No unnecessary duplication: The per-tool `labelKey` replaces the `LABEL_KEY` const rather than layering a second lookup path; `toUsageEntry`'s existing `labelKey` parameter is reused unchanged
- [x] A-011 Strict typecheck: verified during inward review with a real `tsc` 5.x (scratchpad-installed; none in this worktree — note the original rationale here was unsound, since `tsx` type-strips and never evaluates `@ts-expect-error`). Differential result: `tsc --noEmit` reports the *same* 6 pre-existing strict-mode errors at merge-base a7e554f and in the working tree (3× TS2352 `ToolConfig`-cast in fetcher.test.ts:378-380 — pre-existing lines from 260703-bxuh, shifted; 2× TS2352 in cli-skip-brew-update-flag.test.ts:109-110; 1× TS2322 in watch.test.ts:97) — so this change introduces **zero** new type errors and no new `any`/casts, though R5's literal "compilation succeeds with no errors" was already false at base. (Constitution — strict mode; R5)

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Deletion Candidates

- None — the only code this change made redundant (the period-keyed `LABEL_KEY` const and its stale comment in `src/node/core/fetcher.ts`) was removed within the change itself; no other existing files, functions, branches, or config became redundant or unused.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | `cc.prefixArgs` becomes `["claude"]` (composed argv `ccusage claude daily --json`) | Explicit in intake; `claude` subcommand verified live this session to emit claude-only data under `"date"`, parallel to codex/oc | S:95 R:90 A:95 D:95 |
| 2 | Certain | `labelKey: string` on `ToolConfig` (`cc: "date"`, `codex`/`oc`: `"period"`) replaces the `LABEL_KEY` const | Explicit in intake with exact field name/values; two key spellings make a per-tool field the only clean shape | S:95 R:85 A:90 D:85 |
| 3 | Certain | `claude daily --json` emits the ISO label under `"date"`; bare/codex/oc under `"period"` | Verified live this session: claude entry keys include `date` (no `period`); bare entry keys include `period` + `agent: "all"` | S:90 R:95 A:100 D:100 |
| 4 | Confident | `pickCurrentEntry` gains a defaulted `labelKey = "period"` 4th parameter; `fetchTotals` passes `tool.labelKey` | Intake names pickCurrentEntry as a replacement site but leaves exact signature to the implementer; a defaulted trailing param keeps existing 3-arg test calls valid | S:80 R:85 A:80 D:70 |
| 5 | Confident | The existing `runTool argv construction` shape test (which asserts `cc.prefixArgs`) is updated alongside the registry assertions rather than left stale | Test currently builds `[...TOOLS.cc.prefixArgs, "daily", "--json"]` and asserts prefix entries are strings; changing `cc.prefixArgs` to `["claude"]` keeps it green but the explicit `deepEqual([])` assertion in the subcommand test would fail — both must move together | S:75 R:90 A:85 D:80 |

5 assumptions (3 certain, 2 confident, 0 tentative).
