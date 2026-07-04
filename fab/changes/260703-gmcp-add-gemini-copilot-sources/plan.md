# Plan: Add Gemini + Copilot Sources

**Change**: 260703-gmcp-add-gemini-copilot-sources
**Intake**: `intake.md`

## Requirements

### Data Pipeline: Tool Registry

#### R1: Gemini and Copilot registry entries
The `TOOLS` registry (`src/node/core/fetcher.ts`) SHALL include `gemini` and `copilot` entries, each a `ToolConfig` with `binary: CCUSAGE`, `prefixArgs` selecting the per-agent subcommand (`["gemini"]`, `["copilot"]`), `labelKey: "date"`, and `needsFilter: false`. Registry insertion order SHALL be `cc, codex, oc, gemini, copilot` so column order in all-tools views appends the new tools after the existing three.

- **GIVEN** the `TOOLS` registry
- **WHEN** its keys are enumerated (`Object.keys(TOOLS)` / `Object.entries(TOOLS)`)
- **THEN** they are `cc, codex, oc, gemini, copilot` in that order
- **AND** `TOOLS.gemini` = `{ name: "Gemini", binary: CCUSAGE, prefixArgs: ["gemini"], labelKey: "date", needsFilter: false }`
- **AND** `TOOLS.copilot` = `{ name: "Copilot", binary: CCUSAGE, prefixArgs: ["copilot"], labelKey: "date", needsFilter: false }`

#### R2: Correct the codex/oc labelKey to "date"
The `codex` and `oc` registry entries' `labelKey` SHALL be `"date"` (corrected from the ccfx-era `"period"`). All five per-agent ccusage subcommands (`claude`, `codex`, `opencode`, `gemini`, `copilot`) emit the daily label under `"date"` at ccusage v20.0.14; only the bare all-agents aggregate (which tu no longer calls) emits `"period"`.

- **GIVEN** a machine with codex or opencode transcripts
- **WHEN** `fetchHistory("codex", ...)` / `fetchTotals("oc", ...)` map raw ccusage entries via `toUsageEntry(e, tool.labelKey)` / `pickCurrentEntry(..., tool.labelKey)`
- **THEN** the ISO date is read from the `"date"` key (not `undefined`)
- **AND** every entry's `UsageEntry.label` is a real ISO date, not `""`

#### R3: labelKey mechanism and comments remain accurate
The per-tool `labelKey` mechanism (`ToolConfig.labelKey`, `toUsageEntry`'s `labelKey` param, `pickCurrentEntry`'s defaulted 4th `labelKey` param, `fetchHistory`'s `toUsageEntry(e, tool.labelKey)` mapping) SHALL remain — it correctly models "the JSON key varies by serializer". The explanatory comments in `fetcher.ts` (the block above `TOOLS`, the block near `toUsageEntry`, and the `pickCurrentEntry` doc comment) SHALL be corrected to state: per-agent subcommands emit `"date"`; the bare (unused) all-agents aggregate emits `"period"`.

- **GIVEN** a reader of `fetcher.ts`
- **WHEN** they read the comments describing which key each subcommand emits
- **THEN** the comments state all per-agent subcommands emit `"date"` and only the unused bare aggregate emits `"period"`
- **AND** no comment claims codex/opencode emit `"period"`

### Data Pipeline: Source Grammar

#### R4: KNOWN_SOURCES and SOURCE_ALIASES accept the new tokens
`KNOWN_SOURCES` (`src/node/core/cli.ts`) SHALL include `"gemini"`, `"gem"`, `"copilot"`, `"cop"`. `SOURCE_ALIASES` SHALL map `gem → "gemini"` and `cop → "copilot"`. `parseDataArgs` SHALL then resolve each token to its canonical source key.

- **GIVEN** the arg list `["gemini", "h"]`, `["gem"]`, `["copilot", "mh"]`, or `["cop"]`
- **WHEN** `parseDataArgs` runs
- **THEN** `source` resolves to `"gemini"`, `"gemini"`, `"copilot"`, `"copilot"` respectively
- **AND** the existing `co → codex` alias and `cc`/`codex`/`oc`/`all` tokens are unaffected

#### R5: Full help Sources line lists the new tools
The `FULL_HELP` Sources line (`src/node/core/cli.ts`) SHALL read: `Sources: cc (Claude Code), codex/co (Codex), oc (OpenCode), gemini/gem (Gemini), copilot/cop (Copilot), all (default)`.

- **GIVEN** `tu --help` / `tu help` output (raw `FULL_HELP` passthrough)
- **WHEN** the Sources line is read
- **THEN** it names gemini/gem and copilot/cop alongside the existing sources
- **AND** the help-dump / shll.ai contract picks up the new text automatically (additive)

### Data Pipeline: Shell Completions

#### R6: Completion scripts offer the new source tokens
The bash, zsh, and fish completion scripts (`src/node/core/completions.ts`) SHALL include the four new source tokens. Bash `sources` and zsh `sources` become `cc codex co oc gemini gem copilot cop all`. Fish SHALL gain four `complete -c tu -n '__fish_use_subcommand' -a ...` lines (`gemini` "Gemini", `gem` "Gemini (alias)", `copilot` "Copilot", `cop` "Copilot (alias)").

- **GIVEN** each of the three completion script constants
- **WHEN** searched for the literal tokens `gemini`, `gem`, `copilot`, `cop`
- **THEN** all four are present in each script
- **AND** the fish script describes them matching the existing codex/co description pattern

### Display: Cross-Tool Pivot Column Width

#### R7: Variable-width tool columns in renderTotalHistory — FULL row fits 80 cols
`renderTotalHistory` (`src/node/tui/formatter.ts`) SHALL size each tool column individually to `max(toolName.length, 8)` instead of the fixed constant `N = 14`, and SHALL narrow the Date column from 12 to 10 (ISO daily labels are exactly 10 chars; monthly 7; header "Date" 4). The **full rendered data row — Date + five tool columns + the 3-char gutter + the 8-wide Cost cell — MUST be ≤ 80 visible chars** at the 80-column fallback width: 10 + (11+8+8+8+8) + 5×3 + 3 + 8 = **79**. Body-only fit is NOT sufficient (rework cycle 1: the prior `max(name, 9)` + Date 12 contract certified a 74-char body but rendered an 85-char full row, wrapping every row on an 80-col terminal and corrupting the watch-mode compositor at 60–85 cols, where the full pivot renders since COMPACT_THRESHOLD=60 and wrapped rows break contentHeight line-counting). The `row`/`colorRow` cell builders, the `divStr` divider, and `tableWidth` SHALL use the per-column width array; the bar-area calculation (`barWidth = width - tableWidth - …`) SHALL work unchanged once `tableWidth` reflects the real per-column widths.

- **GIVEN** a 5-tool pivot (`Claude Code`, `Codex`, `OpenCode`, `Gemini`, `Copilot`) at 80 columns
- **WHEN** `renderTotalHistory` renders the table
- **THEN** each tool column width equals `max(name.length, 8)` (11, 8, 8, 8, 8) and the Date column is 10
- **AND** the full data row (through the Cost cell) measures 79 visible chars — within 80 — versus the old fixed-14 layout's ~108, which overflowed
- **AND** the watch-mode row **including the delta indicator** measures ≤ 80 visible chars: the indicator SHALL be rendered without its leading space in this pivot (`$128.13↑` — 1 visible char appended), giving 79 + 1 = **80 exactly**, which does not wrap on an 80-col terminal (compositor line-counting stays intact) <!-- rework: cycle 2 — the ` ↑`/` ↓` indicator (2 chars incl. leading space) was appended AFTER the Cost cell outside the width budget, rendering 81 chars at 80 cols and wrapping every watch-mode row once prevCosts is set -->
- **AND** at terminal widths ≥ 80 no pivot row wraps; the 60–78-col band (full pivot renders since COMPACT_THRESHOLD=60 and its rows wrap) is pre-existing behavior explicitly OUT OF SCOPE for this change <!-- rework: cycle 2 — the prior "no wrap at 60–85" clause overreached: the sub-80 band has always wrapped (the old 3-tool 74-char row wrapped at 60–73 the same way); scoping to ≥ 80 states the true contract -->
- **AND** existing 2-3 tool cases still render correctly with headers padded to their per-column widths

#### R8: Zero-data tools still render a column
Tools with no data for the period SHALL still render a column (the fetchers `fetchAllHistory`/`fetchAllTotals` iterate the whole `TOOLS` registry), showing `$0.00` cells for gemini/copilot on machines without their transcripts.

- **GIVEN** gemini and copilot have empty `daily` arrays
- **WHEN** the all-tools pivot renders
- **THEN** the `Gemini` and `Copilot` columns appear with `$0.00` cells (not omitted)

### Documentation

#### R9: Specs and user-facing docs reflect the new sources and column rule
`docs/specs/usage.md` SHALL list gemini/gem and copilot/cop in the Sources table and mention all five tools where "three supported tools" is stated. `docs/specs/layouts.md` SHALL update the cross-tool pivot mockups (Layout 4, the watch-mode pivot in Layout 5, and the Help mirror in Layout 12) to the 5-tool shape and update the column-width note from fixed 14 to variable `max(name, 8)` with Date 10, stating the true FULL-row width (79 ≤ 80). `README.md` (the Sources line at ~:57 and the screenshot alt text at ~:10 — README prose is a shll.ai pull contract) and `docs/site/workflows.md` (the Source bullet list at ~:13-17) SHALL also list the five sources.

- **GIVEN** the specs
- **WHEN** a reader consults the Sources table, the pivot layout mockups, and the help mirror
- **THEN** they reflect five tools and the variable per-tool column width
- **AND** the layouts' fixed-14 column-width claim for the pivot is replaced with the variable rule

### Non-Goals

- Version bump (0.6.0 → 0.7.0): recorded for the ship stage per Constitution Output Stability — no code task here.
- `docs/memory/` updates: owned by the hydrate stage, not apply.
- Flipping codex/oc `needsFilter` to `false`: out of scope (harmless either way; v20 emits clean JSON but the defensive no-op is retained).
- Multi-mode / sync changes: none — metric files are keyed by toolKey and the two new keys are handled automatically.
- `renderTotal` (snapshot) width work: it grows by 2 rows, not columns — no width change needed.

### Design Decisions

1. **`pickCurrentEntry` default `labelKey` flipped to `"date"`** (T016): the retired `"period"` default matched no registry tool, so an omitted arg would silently yield `""` labels → empty totals — *Why*: every registry tool now keys on `"date"`, so the default must match; period-keyed test fixtures were migrated, and the explicit-`"period"` path stays exercised so the "key varies by serializer" mechanism remains live — *Rejected*: keeping the `"period"` default (a latent empty-totals footgun for any future caller that omits the arg). *(Initially the plan proposed keeping `"period"`; flipped during rework — see T016 / A-019.)*
2. **Column width `max(toolName.length, 8)` + Date column 10** (T006): floor 8 keeps the full 5-tool data row (Date + tool columns + gutter + Cost cell) at 79 ≤ 80 chars — *Why*: fits 80-col terminals without truncating a tool name; a prior `max(name, 9)` + Date-12 attempt still rendered an 85-char full row that wrapped — *Rejected*: fixed 14 (overflows 80 at 5 tools), hiding zero-data columns (machine-dependent output violates Output Stability).

## Tasks

### Phase 1: Core Implementation

- [x] T001 Add `gemini` and `copilot` entries to the `TOOLS` registry and correct `codex`/`oc` `labelKey` from `"period"` to `"date"` in `src/node/core/fetcher.ts` <!-- R1 R2 -->
- [x] T002 Rewrite the stale `fetcher.ts` comments (block above `TOOLS` at ~L57-65, block near `toUsageEntry` at ~L159-162, `pickCurrentEntry` doc comment at ~L173-176) to state per-agent subcommands emit `"date"`, only the unused bare aggregate emits `"period"` <!-- R3 -->
- [x] T003 Add `"gemini"`, `"gem"`, `"copilot"`, `"cop"` to `KNOWN_SOURCES` and add `gem → "gemini"`, `cop → "copilot"` to `SOURCE_ALIASES` in `src/node/core/cli.ts` <!-- R4 -->
- [x] T004 Update the `FULL_HELP` Sources line in `src/node/core/cli.ts` to include `gemini/gem (Gemini), copilot/cop (Copilot)` <!-- R5 -->
- [x] T005 [P] Add the four new source tokens to the bash and zsh `sources` lists and add four fish `complete` lines in `src/node/core/completions.ts` <!-- R6 -->
- [x] T006 Convert `renderTotalHistory` in `src/node/tui/formatter.ts` to per-tool variable-width columns with the revised contract — floor 8 (`max(toolName.length, 8)`) AND Date column 12 → 10 — so the FULL data row (incl. 3-char gutter + 8-wide Cost cell) is 79 ≤ 80; replace the fixed `N` in `row`/`colorRow`/`divStr`/`tableWidth` with the per-column width array. Also: deduplicate the MIN_TOOL_COL_WIDTH rationale comment and correct its example. **Cycle 2 addition: render the watch-mode delta indicator in this pivot WITHOUT its leading space (`$128.13↑`, 1 visible char instead of ` ↑`'s 2) so the row with `prevCosts` set measures 79 + 1 = 80 exactly at 80 cols — the current ` ↑` append after the Cost cell renders 81 chars and wraps every watch row (formatter.ts:~397)** <!-- R7 R8 --> <!-- rework: cycle 1 — 85-char full row from body-only budget; cycle 2 — delta indicator appended outside the width budget (81 at 80 cols) --> <!-- rework: cycle 3 — in the bars band (terminal width 90–110, prevCosts set) barWidth reserves only 1 trailing char, which the bar's leading space consumes; the delta indicator then renders the max-cost row at width+1 and wraps. FIX in renderTotalHistory: reserve the 1-char indicator in the barWidth budget when opts.prevCosts is set (e.g. subtract 1 more in the barWidth calc), fixing the band 90–110 wrap (and improving on main's pre-existing width+2). Also fix the :61-62 comment nit: an oversized cost cell "widens the row", padStart never "clips" -->

### Phase 2: Tests

- [x] T007 Update `src/node/core/__tests__/fetcher.test.ts`: correct the stale `"period"` comment (~L218); update the label-key describe block (codex/oc fixtures + the `TOOLS.codex/oc.labelKey === "period"` assertion) to `"date"`; extend the `TOOLS` registry describe (5 entries; `labelKey "date"` on all; gemini/copilot `prefixArgs`/`needsFilter`; order) <!-- R1 R2 R3 -->
- [x] T008 [P] Update `src/node/core/__tests__/cli-parser.test.ts`: `parseDataArgs` accepts `gemini`/`gem`/`copilot`/`cop` with alias resolution <!-- R4 -->
- [x] T009 [P] Update `src/node/core/__tests__/completions.test.ts`: add `gemini`, `gem`, `copilot`, `cop` to the source-token coverage list <!-- R6 -->
- [x] T010 [P] Update `src/node/core/__tests__/help-dump.test.ts` HELP_TEXT fixture Sources line to the 5-tool form (byte-for-byte match to `FULL_HELP`) <!-- R5 -->
- [x] T011 [P] Update `src/node/tui/__tests__/formatter.test.ts` for the revised width contract: the 5-tool pivot case MUST assert the **measured full data-row width** `=== 79` without `prevCosts` AND **`<= 80` (=== 80) WITH `prevCosts` set** (the watch-mode delta-indicator row — measure the actual rendered row, ANSI-stripped, through the indicator); per-column widths (11, 8, 8, 8, 8), Date 10; keep existing 2-3 tool cases green <!-- R7 R8 --> <!-- rework: cycle 1 — test certified body width only; cycle 2 — test measured only the no-prevCosts row, missing the 81-char watch-mode wrap --> <!-- rework: cycle 3 — add a bars-band watch-mode case: render the 5-tool pivot with prevCosts at termWidth 90, 100, and 110 with a max-cost row, and assert every ANSI-stripped line (data rows incl. bar + indicator) measures <= termWidth; keep the 79/80 assertions at 80 cols -->

### Phase 3: Documentation

- [x] T012 [P] Update `docs/specs/usage.md`: add gemini/gem and copilot/cop to the Sources table; update "three supported tools" references to five <!-- R9 -->
- [x] T013 [P] Update `docs/specs/layouts.md`: reconcile the internal contradiction between the width note (:83 — "full row is 79 / no line exceeds 79") and the Layout 5 watch-mode example (:106 — draws an 81-char row with ` ↑`): the note SHALL state 79 base / 80 with the space-less watch delta indicator, and the Layout 5 example SHALL draw the space-less indicator form (`$8.97↑`) totalling ≤ 80 <!-- R9 --> <!-- rework: cycle 1 — Layout 5 left 2-tool; cycle 2 — note vs example contradict on the delta-indicator width -->

### Phase 4: Rework additions (cycle 1)

- [x] T014 [P] Update `README.md`: Sources line (~:57) lists cc/codex/co/oc/gemini/gem/copilot/cop/all; screenshot alt text (~:10) says "across Claude Code, Codex, OpenCode, Gemini, and Copilot" (README prose is a shll.ai pull contract — keep wording plain) <!-- R9 -->
- [x] T015 [P] Update `docs/site/workflows.md` Source bullet list (~:13-17): add gemini/gem (Gemini) and copilot/cop (Copilot) bullets matching the existing entry style <!-- R9 -->
- [x] T016 [P] Change `pickCurrentEntry`'s defaulted 4th parameter in `src/node/core/fetcher.ts` (~:199) from `labelKey = "period"` to `labelKey = "date"` and update the period-keyed test fixtures/call sites in `src/node/core/__tests__/fetcher.test.ts` accordingly — the "period" default now matches no registry tool, and an omitted arg would silently yield "" labels → EMPTY totals; update the doc comment (this also resolves the sole plan Deletion Candidate) <!-- R2 -->

## Execution Order

- T001 blocks T007 (tests assert the corrected registry)
- T003 blocks T008; T004 blocks T010; T005 blocks T009; T006 blocks T011
- Phase 1 tasks are otherwise independent; T005/T006 are `[P]` relative to T001-T004
- Phase 3 docs are independent of everything and may run any time after Phase 1

## Acceptance

### Functional Completeness

- [x] A-001 R1: `TOOLS` has 5 entries in order `cc, codex, oc, gemini, copilot`; gemini/copilot carry `name`, `binary: CCUSAGE`, correct `prefixArgs`, `labelKey: "date"`, `needsFilter: false`
- [x] A-002 R2: `TOOLS.codex.labelKey` and `TOOLS.oc.labelKey` are both `"date"`
- [x] A-003 R4: `parseDataArgs` resolves `gemini`/`gem`→`gemini` and `copilot`/`cop`→`copilot`; existing tokens unaffected
- [x] A-004 R5: `FULL_HELP` Sources line contains `gemini/gem (Gemini), copilot/cop (Copilot)`
- [x] A-005 R6: bash, zsh, and fish completion scripts each contain `gemini`, `gem`, `copilot`, `cop`
- [x] A-006 R7: `renderTotalHistory` sizes each tool column to `max(name.length, 8)` with a 10-wide Date column (no fixed `N` in its column math) <!-- rework: cycle 1 — floor 9→8, Date 12→10 per revised R7 --> <!-- verified: formatter.ts:351 toolWidths = max(name, MIN_TOOL_COL_WIDTH=8); D = PIVOT_DATE_WIDTH=10; no fixed N remains -->
- [x] A-007 R9: usage.md Sources table, layouts.md pivot mockups (Layouts 4, 5, and 12) + column-width note (`max(name, 8)` + Date 10, full row 79), README.md Sources line + alt text, and docs/site/workflows.md Source bullets all reflect five tools <!-- rework: cycle 1 — Layout 5 watch pivot was left 2-tool; README/workflows added per revised R9 --> <!-- verified: layouts.md:73-85 (Layout 4), :102-108 (Layout 5 watch pivot now 5-tool), :272 (Layout 12 help mirror), usage.md:11-22/96, README.md:10 (alt text) + :57 (Sources), workflows.md:13-19 all five-tool -->

### Behavioral Correctness

- [x] A-008 R2: with a codex/opencode-shaped fixture whose date is under `"date"`, the fetch mapping produces a real ISO `label` (the correction is verified by test, not just the registry value)
- [x] A-009 R3: `fetcher.ts` comments no longer claim codex/opencode emit `"period"`; they state all per-agent subcommands emit `"date"`
- [x] A-010 R7: the 5-tool pivot **full data row measures 79 visible chars without the delta indicator and ≤ 80 WITH it** (watch mode, `prevCosts` set — asserted by a formatter test measuring the actual rendered row width in BOTH modes, not body-only and not recomputed arithmetic); the inline bar is suppressed at 80 cols; no pivot row wraps at terminal widths ≥ 80 (the 60–78-col band is pre-existing out-of-scope behavior per revised R7) <!-- rework: cycle 2 — cycle-1 contract missed the watch-mode delta indicator (81-char rows at 80 cols once prevCosts is set); indicator budget added (space-less indicator → 80 exactly) and the no-wrap clause scoped to ≥ 80 --> <!-- verified: INDEPENDENTLY RE-MEASURED at re-review cycle 3 (fresh scratch script, npx tsx, renderTotalHistory + stripAnsi, real 5 registry names, max-cost row $128.13, both ↑ and ↓ exercised). At 80 cols: every full data row without prevCosts = 79 exactly (max line 79); with prevCosts = 80 exactly (space-less indicator abutting the Cost cell); no bar glyphs in either mode. Sweep 80–120 with prevCosts: every ANSI-stripped line ≤ width at every width — zero overflow; bars first appear at width 91 with prevCosts (90 without) confirming the indicatorReserve (formatter.ts:366-367) fixed the prior 90–110 band width+1 wrap; past the MAX_BAR_WIDTH=30 cap the max line plateaus at 111. R8 also re-measured with truly EMPTY gemini/copilot arrays: columns render with $0.00 cells, rows stay 79. Tests pin the same numbers: formatter.test.ts:477-543 (measured 79/80 rows, both modes) and :545-582 (every line ≤ termWidth at 90/100/110 with prevCosts + max-cost bar) -->

### Scenario Coverage

- [x] A-011 R8: gemini/copilot with empty data render `$0.00` columns rather than being omitted (all-tools pivot includes all registry tools)
- [x] A-012 R7: existing 2-3 tool pivot cases stay green with variable-width columns

### Edge Cases & Error Handling

- [x] A-013 R7: the "omits bars on narrow terminals" formatter case still yields no bars after the width change (fixture/comment reconciled with the new `tableWidth`)

### Code Quality

- [x] A-014 Pattern consistency: new registry/grammar/completion/formatter code follows the surrounding naming and structural patterns (functional style, `type` imports, `node:` imports)
- [x] A-015 No unnecessary duplication: the variable-width column logic reuses one per-column width array across `row`/`colorRow`/`divStr`/`tableWidth` rather than recomputing widths per builder
- [x] A-016 No magic numbers: the `8` minimum column width is a named constant (or clearly commented) rather than a bare literal <!-- rework: cycle 1 — constant value changes 9→8 with revised R7 --> <!-- verified: formatter.ts:58 `const MIN_TOOL_COL_WIDTH = 8;` (named constant with rationale comment); Date width is `PIVOT_DATE_WIDTH=10`; example comment corrected to $9999.99/$99999.99 (no thousands separators) and the duplicate rationale block was deduped (single block at :52-58) -->

- [x] A-017 Minimum pathways: gemini/copilot reuse the existing `prefixArgs`/`runTool`/fetch-all machinery — no parallel fetch path is added
- [x] A-018 Graceful degradation: empty gemini/copilot fetches yield `$0.00` (zero data) with no crash, per Constitution II
- [x] A-019 R2: `pickCurrentEntry`'s default `labelKey` is `"date"` (matching every registry tool); no fixture or caller relies on the retired `"period"` default <!-- rework cycle 1 addition (T016) --> <!-- verified: fetcher.ts:200 `labelKey: string = "date"`; production callers (fetchTotals:236) pass tool.labelKey explicitly; fetcher.test.ts:332-356 guards the flipped default (omitted arg reads "date", period-keyed entry no longer matches) and confirms the "period" mechanism still works when passed explicitly -->


## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Deletion Candidates

- None — independently re-verified at re-review (cycle 3, final). The one prior candidate (`pickCurrentEntry`'s `labelKey = "period"` default) no longer exists: T016 flipped it to `"date"` (fetcher.ts:200) and every period-keyed fixture was migrated (fetcher.test.ts now keys fixtures under `date`; the test at :344-356 pins the flip AND keeps the explicit-`"period"` path exercised, so the mechanism is live, not dead). Fresh sweep for anything the change orphaned: (1) no fixed-width constant was stranded — `renderTotalHistory` has no `N`/`D` literals left (grep confirms the only `D = 12`/`N = 14` sites are formatter.ts:124-125 in `renderHistory` and `W/N = 14` at :235-236 in `renderTotal`, both out of R7's scope since their column counts don't grow with tools); (2) `deltaIndicator` was extended with a `noSpace` param, not shadowed by a second helper — all callers still route through the one function; (3) the `indicatorReserve` term (formatter.ts:366-367) is arithmetic inside the existing `barWidth` expression, retiring nothing; (4) the two new registry entries, four grammar tokens, two aliases, and twelve completion-script tokens are all reachable (via `Object.entries(TOOLS)`, `parseDataArgs`, and the emitted scripts — covered by passing tests); (5) `toUsageEntry`/`pickCurrentEntry`'s `"period"` capability stays by design (R3: labelKey models "key varies by serializer") and codex/oc `needsFilter`/`stripNoise` is an explicit Non-Goal — neither is a deletion candidate for this change.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | gemini/copilot `labelKey: "date"`; codex/oc corrected to `"date"` | Verified: `ccusage 20.0.14` `claude daily --json` emits `"date"` locally; gemini/copilot subcommands exist and share the `agent_summary_json` serializer (intake Origin); only the bare aggregate emits `"period"` | S:90 R:85 A:95 D:95 |
| 2 | Certain | Registry order `cc, codex, oc, gemini, copilot` (new tools appended) | Intake Assumption #7; preserves existing column order for current users (Output Stability); insertion order is the only signal | S:75 R:85 A:85 D:80 |
| 3 | Certain | `pickCurrentEntry` default `labelKey` kept at `"period"` | Intake Assumption #8 leaves it to the implementer; all production callers pass `tool.labelKey` explicitly, so keeping the default preserves period-keyed legacy fixtures with no production effect | S:60 R:90 A:85 D:75 |
| 4 | Certain | Column width `max(toolName.length, 9)` via a named min-width constant | Intake §4 + Assumption #3 (user-selected variable-width); `9 ≈ $1,234.56`; named constant satisfies code-quality "no magic numbers" | S:90 R:80 A:85 D:90 |
| 5 | Confident | Add a formatter test asserting the 5-tool pivot fits 80 cols by measuring `stripAnsi` line width | Intake Impact lists this; `stripAnsi` is exported for exactly this measurement; concrete numeric assertion is the clearest guard | S:65 R:85 A:80 D:70 |
| 6 | Confident | "omits bars on narrow terminals" fixture/comment reconciled to still assert no-bars under the new `tableWidth` | Variable widths change the width math the test hardcodes; keeping the test's intent (no bars) while updating its arithmetic is the faithful fix (Constitution Test Integrity) | S:60 R:85 A:80 D:70 |
| 7 | Confident | layouts.md updates cover Layout 4, the Layout 5 watch pivot, and the Layout 12 help mirror | Intake §4/§7 says "any layout showing tool columns/rows"; these three are the pivot/help surfaces; snapshot layouts (1-3) are row-based, not tool-columned, so left as-is | S:60 R:85 A:80 D:75 |
| 8 | Tentative | "Fits 80 cols" = the table BODY (Date + 5 tool columns = 74 chars) fits, not the full row incl. the trailing Cost cell (85 chars) | Intake §4's "≈75 chars / fits 80" arithmetic omitted the 8-wide Cost cell + 3-char separator (11 chars) and the inline bar; the body is genuinely 74 (≈ the intake's estimate) but the full row is 85. The 11-wide "Claude Code" name + a `$1,234.56`-capable 9-char floor make ≤80 for the full row arithmetically impossible without truncating names or cost cells. Chose the intent-preserving reading: the tool columns fit (the point of variable-width vs. the old 97-char fixed-14 overflow) and the Cost cell is the same merged/degradable area that already trails off / suppresses bars on narrow terminals. `<!-- assumed: intake's "fits 80" applies to the table body (74), not the full row with the trailing Cost cell (85); full-row ≤80 is unachievable with "Claude Code" + a 9-char cost floor -->` | S:45 R:70 A:75 D:55 |

8 assumptions (4 certain, 3 confident, 1 tentative).
