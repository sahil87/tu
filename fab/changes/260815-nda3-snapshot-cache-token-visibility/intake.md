# Intake: Snapshot Cache Token Visibility

**Change**: 260815-nda3-snapshot-cache-token-visibility
**Created**: 2026-08-15

## Origin

Conversational — a `/fab-discuss` session on improving tu's output DX. The agent reviewed live `tu` output and proposed five improvements; the user selected idea 2 for this change:

> Idea 2: The snapshot's numbers don't add up visually. `Tokens 487,683,047 | Input 3,734 | Output 1,121,329` — Tokens includes cache but the breakdown doesn't show it, so Input + Output ≠ Tokens by a factor of 400x. Anyone reading this for the first time thinks it's a bug. Either add a `Cache` column (write+read combined) or a compact `487.7M (99% cache)` treatment.

User instruction: "Then [the same for] 2."

## Why

1. **Pain point**: The cross-tool snapshot (`tu`, `tu m`, `tu cc`) shows `Tokens | Input | Output | Cost`. `Tokens` is `totalTokens` (input + output + cache write + cache read) but the visible breakdown omits both cache components. With real Claude Code data, cache dwarfs everything (487.7M total vs 1.1M input+output), so the table's own columns appear mutually inconsistent by orders of magnitude.
2. **Consequence of not fixing**: The default, most-used view of the tool reads as buggy to anyone who checks the arithmetic; the dominant cost driver (cache) is invisible in the snapshot even though the single-tool history already breaks it out.
3. **Why this approach**: Adding a combined `Cache` column makes the row arithmetic close (`Input + Output + Cache = Tokens`) using data already present on `UsageTotals` (`cacheCreationTokens + cacheReadTokens`). The single-tool history precedent (separate Cache Write / Cache Read columns) is too wide for the snapshot; one combined column is the right granularity for an at-a-glance view.

## What Changes

### 1. Add a combined `Cache` column to the snapshot

In `src/node/tui/formatter.ts`, `renderTotal()` gains a `Cache` column (`cacheCreationTokens + cacheReadTokens`) so the visible columns sum to `Tokens`:

```
Tool           |        Tokens |        Input |       Output |        Cache |         Cost
──────────────────────────────────────────────────────────────────────────────────────────
Claude Code    |   487,683,047 |        3,734 |    1,121,329 |  486,557,984 |      $465.67
```

Column order: `Tool | Tokens | Input | Output | Cache | Cost` — `Tokens` stays first among numerics (it is the headline figure), `Cache` sits between `Output` and `Cost` mirroring the single-tool history's ordering (input, output, cache, total, cost).

### 2. Width budget

Current table: `14 + 4×14 + 4×3 = 82` chars. Adding a fifth 14-wide numeric column gives `14 + 5×14 + 5×3 = 99` — too wide for 80-col terminals. Reclaim width by narrowing the numeric column width `N` for the snapshot from 14 to 12 (`14 + 5×12 + 5×3 = 89`) and the Tool column from 14 to 12 (`87`). A 12-wide cell holds `999,999,999,999` — comfortably above any real token count. If 80 cols cannot be reached without harming readability, prefer clean columns at ~87 and let sub-80 terminals wrap the (rare) snapshot rather than distorting the layout — the pivot already accepts >80 for 2+ tools. Exact final widths are a plan-level decision; the requirement is: **all five numeric columns visible, row arithmetic closes, and width ≤ 90**.

- The machine-column variant (multi mode, `machineCosts`) appends after `Cost` as today; its width math shifts accordingly.
- Total row sums the new column like the others.

### 3. Scope

- **ANSI snapshot** (`renderTotal`) — the change.
- **Markdown emitter** (`emitMarkdownSnapshot`) — add the same `Cache` column (human paste context, width-unconstrained).
- **CSV emitter** (`emitCsvSnapshot`) — add a `cache` column **appended after `output`**; CSV consumers index by header name per RFC 4180 conventions, and the snapshot CSV has no documented positional contract beyond the header row — but flag this in the PR body as a CSV shape change.
- **JSON output** — already exposes `cacheCreationTokens`/`cacheReadTokens`; no change.
- **Compact mode** (watch, <60 cols) — name + cost only; no change.
- **Single-tool history** — already shows cache columns; no change.

### 4. Version bump

Snapshot table shape changes (new column, adjusted widths) → minor version bump per the constitution's Output Stability rule (may share a bump with q6fx/oojd if released together).

## Affected Memory

- `display/formatting`: (modify) snapshot column set and width budget

## Impact

- `src/node/tui/formatter.ts` — `renderTotal`, `emitMarkdownSnapshot`, `emitCsvSnapshot`, snapshot width constants
- Formatter tests — snapshot expected-output updates; new cases: cache column arithmetic (Input+Output+Cache = Tokens), zero-cache tool renders `0`, machine-column variant width
- `docs/specs/layouts.md` — §1/§2 snapshot mockups and column notes
- Version: minor bump required

## Open Questions

*(none)*

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | Combined `Cache` column (write+read) rather than the compact `487.7M (99% cache)` treatment | Discussed — user offered both; the column closes the arithmetic explicitly and keeps the snapshot's tabular idiom; compact-format tokens would be a bigger departure | S:65 R:80 A:75 D:60 |
| 2 | Tentative | Narrow numeric columns 14 → 12 to keep width ≤ 90; accept >80 if needed | Width budget is a layout judgment call; requirement fixed (≤90, arithmetic closes), exact widths left to plan | S:50 R:85 A:70 D:55 |
| 3 | Confident | Keep the `Tokens` headline column (do not drop it as redundant) | It is the at-a-glance figure users read first; dropping it would be a bigger stability break than adding Cache | S:55 R:80 A:75 D:65 |
| 4 | Confident | CSV gains a `cache` column after `output`; JSON unchanged | JSON already carries both cache fields; CSV named-header convention makes an added column low-risk, flagged in PR | S:55 R:75 A:80 D:70 |
| 5 | Certain | Minor version bump ships with this change | Constitution "Output Stability" mandates it for output-shape changes | S:85 R:90 A:95 D:95 |

5 assumptions (1 certain, 3 confident, 1 tentative, 0 unresolved).
<!-- assumed: snapshot numeric column width 14 → 12 — exact width budget left to plan; hard requirement is all five columns ≤ 90 cols with closing arithmetic -->
