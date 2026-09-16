# Plan: G1 rework 1 — tool registry to `fact`, history tables split

**Change**: 260916-m9of-g1-rework-1
**Intake**: `intake.md`

## Requirements

All paths are under `src/go/`. Nothing outside `src/go/`, `docs/memory/`, and `fab/changes/` changes. The external surface (CLI grammar, `--help`, table/JSON/CSV/Markdown bytes, exit codes, stderr) is frozen by the plan's Goal; every requirement below is internal restructuring proven by unchanged tests and an unchanged harness green set.

### Go port: tool registry in `fact`

#### R1: `fact` owns the tool registry
`package fact` SHALL define `Tool{Key, Name string}`, the ordered slice `Tools` (exactly `cc`/Claude Code, `codex`/Codex, `oc`/OpenCode, `gemini`/Gemini, `copilot`/Copilot, `kimi`/Kimi — in that order, never a map), and `Lookup(key string) (Tool, bool)` scanning the slice. Keys, names and order MUST be byte-identical to the pre-change `ccusage.Tools`. Source aliases (`co`, `gem`, `cop`, `ki`) MUST NOT be registry entries.

- **GIVEN** `fact.Tools` iterated
- **WHEN** keys are collected
- **THEN** they are `cc, codex, oc, gemini, copilot, kimi` in that order; `fact.Lookup("gemini")` returns `{gemini, Gemini}, true`; `fact.Lookup("gem")` returns `false`

#### R2: `source` owns the adapter-neutral fetch constants
`package source` SHALL define `const PeriodDaily = "daily"` and `const DefaultTimeout = 120 * time.Second` (new file `internal/source/fetch.go`), with doc comments stating that adapters impose no deadline of their own and that tu fetches daily records only. `package ccusage` MUST NOT export either.

- **GIVEN** `command.Run` composing a fetch
- **WHEN** it sets the deadline and the period
- **THEN** it reads `source.DefaultTimeout` and `source.PeriodDaily`, and no other package defines those names

### Go port: ccusage adapter keyed by fact key

#### R3: ccusage keeps only adapter-private invocation data
`package ccusage` SHALL replace its exported registry with an unexported `invocations map[string]invocation` keyed by `fact.Tool.Key`, where `invocation{prefixArgs []string; labelKey string}` carries exactly today's `PrefixArgs`/`LabelKey` values (`claude`, `codex`, `opencode`, `gemini`, `copilot`, `kimi`; every `labelKey` = `date`). `argv(tool fact.Tool, period, extraArgs)` MUST compose `invocations[tool.Key].prefixArgs…, period, "--json", extraArgs…`; `Parse(raw, tool fact.Tool)` MUST read `invocations[tool.Key].labelKey`; `run`/`Fetch` MUST fill `source.Error{Tool: tool.Key, Name: tool.Name}` from the `fact.Tool`. `Source.Fetch` SHALL take `tool fact.Tool`; `Source.FetchAll` SHALL iterate `fact.Tools`. The exported `ccusage.Tool`, `ccusage.Tools`, `ccusage.Lookup`, `ccusage.PeriodDaily`, `ccusage.DefaultTimeout` MUST be removed (no aliases). A registry test MUST assert the key set of `invocations` equals the key set of `fact.Tools`.

- **GIVEN** `fact.Lookup("cc")`
- **WHEN** `argv(tool, "daily", nil)` is composed
- **THEN** it is `["claude", "daily", "--json"]`, unchanged from today
- **GIVEN** `FetchAll` against the placeholder corpus
- **WHEN** the six tools are fetched concurrently
- **THEN** 18 records arrive in registry order and, for period `weekly`, six `KindExec` errors in registry order — the existing `source_test.go` assertions hold unchanged

### Go port: command edge fetches through `fact` and `source` only

#### R4: `command` imports no `source/*` adapter
`command.Fetcher.Fetch` SHALL take `tool fact.Tool`; `run.go` SHALL use `source.DefaultTimeout`, `source.PeriodDaily`, `fact.Tools`, `fact.Lookup`, and `[]fact.Tool{tool}`. No file under `internal/command/` (including `_test.go` files) MAY import `internal/source/ccusage`; the `var _ Fetcher = (*ccusage.Source)(nil)` assertion in `run_test.go` is deleted (the `Source: src` assignment in `cmd/tu/main.go` is the compile-time check). The `fakeFetcher` in `run_test.go` takes `fact.Tool` and iterates `fact.Tools`. The `Fetcher` interface stays in `command`.

- **GIVEN** the Go tree after the change
- **WHEN** `grep -rl 'internal/source/ccusage"' src/go --include='*.go'` runs
- **THEN** the only match outside `src/go/internal/source/ccusage/` is `src/go/cmd/tu/main.go`

#### R5: `cmd/tu` is the sole composition point for ccusage
`cmd/tu/main.go` SHALL keep `src := &ccusage.Source{Cache: store}` and `command.Deps{Source: src, …}` unchanged in behavior. Its e2e test bytes (`e2e_test.go`) MUST pass unchanged.

- **GIVEN** the staged e2e `$HOME`s and the placeholder corpus
- **WHEN** `go test ./cmd/tu/...` runs
- **THEN** every byte-exact stdout/stderr/exit assertion passes with no golden or expectation edit

### Go port: history tables split into named helpers

#### R6: `TotalHistory` is a short composition over concern-named helpers
`view.TotalHistory(series []Series, o HistoryOptions) Table` SHALL keep its signature, doc comment contract and output, and its body SHALL be under 50 lines, composing unexported helpers named for one concern each — at minimum: `pivotValues(series, m) []map[string]float64` (tool → label → metric value), `pivotData(series, valueMap, visible, labels) (rows []pivotRow, toolSums []float64, grandTotal float64)` with `pivotRow{label string; values []float64; rowValue float64}` as a package-level type, `pivotWidths(…) (toolWidths []int, costWidth int)`, and `pivotRows(…) []Row` (header, divider, month separators, data rows with bars). The Total row / footer / legend block MAY stay inline or become `pivotTotals`. No helper MAY exceed 50 lines. `significant`, `nonzero`, `LabelUnion`, `rowDelta`, `monthPrefixOf`, `footerText`, `rowBar`, `ComputeScale`, `Apportion` and every constant stay unchanged.

- **GIVEN** the nine existing `TestTotalHistory*` tests and the render/ansi goldens
- **WHEN** `go test ./internal/view/... ./internal/render/...` runs without `-update`
- **THEN** every test passes with no test or golden edit

#### R7: `History` follows the same split and shares the bar budget
`view.History(s Series, o HistoryOptions) Table` SHALL keep its signature and output, its body SHALL be under 50 lines, and it SHALL compose `historyValues(s, m) (values []float64, sum float64)`, `historyColumns(costWidth int, m Metric) []Column`, and `historyRows(…) ([]Row, fact.Totals)` (data rows plus the summed totals the Total row needs). Both tables SHALL compute their bar budget through one shared unexported helper `barBudget(width, bodyWidth, costWidth, reserve int) (barWidth int, show bool)` = `min(width − bodyWidth − gutterWidth − costWidth − barLeading − reserve, maxBarWidth)`, `show = barWidth ≥ minBarArea`; `History` passes `historyBodyWidth` and reserve 0, `TotalHistory` passes its computed table width and the `Prev != nil` indicator reserve.

- **GIVEN** the nine existing `TestHistory*` tests (including `TestHistoryBarBudget` and `TestTotalHistoryIndicatorReserve`)
- **WHEN** they run after the split
- **THEN** every test passes unchanged, and `TotalHistory` and `History` use the same `*Values` / `*Rows` verbs

### Go port: verification bar

#### R8: The harness green set is unchanged
After the change, `just go-lint` MUST be clean, `just go-test` MUST pass all packages with no golden updated, and `env -u TU_METRICS_REPO -u NO_COLOR just go-diff --placeholder` MUST report at least 134/368 green with every case that was green on `main` (7030a74) still green. The baseline report is at `/tmp/claude-1001/-home-sahil-code-sahil87-tu-worktrees-g1-rework-1/e6135319-6870-4f6c-9418-6dd5bc0bb8fa/scratchpad/baseline-harness.txt` when present; otherwise the run's per-case results are reported for the orchestrator to compare.

- **GIVEN** the baseline green set from `main`
- **WHEN** the post-change harness report is diffed against it per case
- **THEN** no case moved from green to red

### Non-Goals
- Verdict follow-ups 3–5 (`view` importing `render` for `FormatInt`/`FormatCost`; test volume; the Series-vs-Table encoder split) — backlog rows, not rework.
- `command/parse.go` `Parse` / `scanFlags` — flat flag tables, explicitly left alone by the verdict.
- Any behavior, output, flag, or exit-code change; any file under `src/node/`, `harness/`, `Formula/`, `.github/`.
- Moving the `Fetcher` interface out of `command`.

### Design Decisions

#### Registry lives in `fact`, not a new package
**Decision**: `fact.Tool{Key, Name}`, `fact.Tools`, `fact.Lookup` own the registry; adapters hold their own per-key data.
**Why**: the registry (key, display name, column order) is a property of the fact model — `fact.Record.Tool` already carries the key — and `fact` is the package every stage imports; B3's `source/metrics` and `command` need it without touching an adapter.
**Rejected**: a tiny `internal/tools` package (a fourth import for ~30 lines, no isolation gain); a type alias in `ccusage` (keeps the weld the verdict flagged).
*Introduced by*: 260916-m9of-g1-rework-1

#### `Fetcher.Fetch` takes `fact.Tool`
**Decision**: the parameter is the struct, not the key string.
**Why**: the command edge already resolved `Lookup(req.Source)` after grammar validation; passing the struct spares every adapter a re-lookup and a miss branch, and `source.Error{Tool, Name}` needs the name anyway.
**Rejected**: `Fetch(ctx, key string, …)` — a smaller interface that pushes a lookup-and-miss path into each adapter.
*Introduced by*: 260916-m9of-g1-rework-1

#### Fetch contract constants live in `source`
**Decision**: `source.PeriodDaily` and `source.DefaultTimeout`.
**Why**: both describe the contract every adapter honors (the one period tu fetches; the edge-applied deadline); `source` is already the adapter-neutral package (`Error`, `WriteWarnings`), and B3/B6 fetch under the same rules.
**Rejected**: unexported constants in `command` (correct today, but B6's `sync` would re-declare the deadline).
*Introduced by*: 260916-m9of-g1-rework-1

#### One `barBudget` for both history tables
**Decision**: `barBudget(width, bodyWidth, costWidth, reserve)` is the single bar-width formula; the two tables differ only in `bodyWidth` and `reserve`.
**Why**: the verdict wants the tables parallel and B4's token-metric/stacked bars touch exactly this budget; one formula cannot drift into two.
**Rejected**: leaving the two `min(...)` expressions inline (the verdict's minimum) — two copies of one rule.
*Introduced by*: 260916-m9of-g1-rework-1

## Tasks

### Phase 1: Setup

- [x] T001 Create `src/go/internal/fact/tool.go` with `Tool{Key, Name}`, ordered `Tools` (cc/Claude Code, codex/Codex, oc/OpenCode, gemini/Gemini, copilot/Copilot, kimi/Kimi), and `Lookup`; add one sentence to the `fact` package doc naming the registry; create `src/go/internal/fact/tool_test.go` asserting order, names, `Lookup("gemini")` hit, and `Lookup` misses for `co`, `gem`, `cop`, `ki` <!-- R1 -->
- [x] T002 [P] Create `src/go/internal/source/fetch.go` with `PeriodDaily = "daily"` and `DefaultTimeout = 120 * time.Second` and their doc comments <!-- R2 -->

### Phase 2: Core Implementation

- [x] T003 Rewrite `src/go/internal/source/ccusage/registry.go`: delete exported `Tool`, `Tools`, `PeriodDaily`, `Lookup`; add unexported `invocation{prefixArgs, labelKey}` and `invocations` map keyed by fact key with today's values; `argv(tool fact.Tool, period, extraArgs)` reads the map; update the package doc comment (registry no longer owned here). Delete `DefaultTimeout` from `exec.go`; change `run(ctx, tool fact.Tool, …)` and `Parse(raw, tool fact.Tool)` (`normalize.go`) to take `fact.Tool` and read `labelKey` from the map <!-- R3 -->
- [x] T004 Update `src/go/internal/source/ccusage/source.go`: `Fetch(ctx, tool fact.Tool, period, extraArgs, fresh)`; `FetchAll` iterates `fact.Tools`; error construction uses `tool.Key`/`tool.Name` from `fact.Tool`; doc comments follow <!-- R3 -->
- [x] T005 Update ccusage tests: `registry_test.go` replaces the `Tools` DeepEqual with (a) key set of `invocations` == key set of `fact.Tools` and (b) `invocations["cc"].prefixArgs[0] == "claude"` plus the argv assertion via `fact.Lookup("cc")`; `normalize_test.go` and `source_test.go` use `fact.Tools` / `fact.Lookup` in place of the package's own <!-- R3 -->
- [x] T006 Update `src/go/internal/command/run.go`: `Fetcher.Fetch` takes `fact.Tool`; `context.WithTimeout(ctx, source.DefaultTimeout)`; `source.PeriodDaily`; `fact.Lookup` / `fact.Tools` / `[]fact.Tool{tool}` in `Run`, `runSnapshot`, `runHistory`; remove the ccusage import; update the `Fetcher` doc comment (asserted where `cmd/tu` assigns it) <!-- R4 -->
- [x] T007 Update `src/go/internal/command/run_test.go`: `fakeFetcher.Fetch` takes `fact.Tool`; loops use `fact.Tools`; delete `var _ Fetcher = (*ccusage.Source)(nil)` and the ccusage import <!-- R4 -->
- [x] T008 Split `src/go/internal/view/pivot.go`: promote `pivotRow` to a package-level type; extract `pivotValues`, `pivotData`, `pivotWidths`, `pivotRows` (and `pivotTotals` if the Total/footer/legend block keeps `TotalHistory` over 50 lines); `TotalHistory` becomes a top-to-bottom composition under 50 lines; every helper under 50 lines; doc comment bullets unchanged <!-- R6 -->
- [x] T009 Split `src/go/internal/view/history.go`: extract `historyValues`, `historyColumns`, `historyRows`; add the shared `barBudget(width, bodyWidth, costWidth, reserve) (int, bool)` next to `gutterWidth`/`barLeading` (or in `bar.go`) and call it from both `History` and `TotalHistory`; `History` under 50 lines <!-- R7 -->

### Phase 3: Integration & Edge Cases

- [x] T010 From `src/go/`: `gofmt -l .` empty, `go vet ./...` clean, `go build ./...` clean; verify the import graph — `grep -rl 'internal/source/ccusage"' src/go --include='*.go' | grep -v '^src/go/internal/source/ccusage/'` prints exactly `src/go/cmd/tu/main.go`; verify `grep -n 'ccusage' src/go/internal/command/*.go` shows no import lines; count the lines of the `TotalHistory` and `History` bodies and of every new helper (all < 50) <!-- R4 R5 R6 R7 -->
- [x] T011 From the repo root: `just go-test` (equivalently `cd src/go && go test ./... -count=1`) — all packages ok, **no `-update` flag anywhere**, `git status` shows no modified `.golden` file <!-- R5 R6 R7 R8 -->
- [x] T012 From the repo root: `env -u TU_METRICS_REPO -u NO_COLOR just go-diff --placeholder`; record the summary count (≥ 134/368) and diff the per-case results against the baseline report at the R8 path when it exists (no case green → red); when absent, save the full report under `.fab-dispatch/m9of/harness-after.txt` for the orchestrator to compare <!-- R8 -->

## Execution Order

- T001 and T002 are independent and block everything in Phase 2 (T003–T007 compile against `fact.Tool` and `source.PeriodDaily`)
- T003 → T004 → T005 (same package, sequential)
- T006 → T007 (compile together; T006 needs T003/T004 for `Fetch`'s new signature)
- T008 and T009 are independent of Phase 2's registry work but T009's `barBudget` must land before T008 calls it (or T008 introduces it and T009 reuses it — either order, one owner)
- T010 → T011 → T012 sequential

## Acceptance

### Functional Completeness

- [x] A-001 R1: `fact.Tool`, `fact.Tools` (six entries, registry order, exact names), and `fact.Lookup` exist with a sibling `tool_test.go`
- [x] A-002 R2: `source.PeriodDaily` and `source.DefaultTimeout` exist in `internal/source/fetch.go`; neither is exported from `ccusage`
- [x] A-003 R3: `ccusage` has the unexported `invocations` map keyed by fact key carrying today's `prefixArgs`/`labelKey`; `argv`, `Parse`, `run`, `Fetch`, `FetchAll` take or iterate `fact.Tool`/`fact.Tools`
- [x] A-004 R4: `command.Fetcher.Fetch` takes `fact.Tool`; `run.go` uses `source.DefaultTimeout`, `source.PeriodDaily`, `fact.Tools`, `fact.Lookup`
- [x] A-005 R5: `cmd/tu/main.go` composition unchanged in behavior; `e2e_test.go` passes unchanged
- [x] A-006 R6: `TotalHistory` composes `pivotValues`, `pivotData`, `pivotWidths`, `pivotRows` (± `pivotTotals`); `pivotRow` is a package-level type
- [x] A-007 R7: `History` composes `historyValues`, `historyColumns`, `historyRows`; both tables call the one `barBudget`
- [x] A-008 R8: `just go-lint` clean; `just go-test` all ok; harness ≥ 134/368 with the `main` green set preserved

### Behavioral Correctness

- [x] A-009 R4: `grep -rl 'internal/source/ccusage"' src/go --include='*.go'` matches only `src/go/cmd/tu/main.go` outside the ccusage package itself (test files included in the check)
- [x] A-010 R6 R7: `TotalHistory` and `History` bodies are each under 50 lines; no new helper exceeds 50 lines
- [x] A-011 R6 R7: no `view` export changed — `TotalHistory`/`History` signatures identical, no new exported identifier in `internal/view`

### Removal Verification

- [x] A-012 R3: `ccusage.Tool`, `ccusage.Tools`, `ccusage.Lookup`, `ccusage.PeriodDaily`, `ccusage.DefaultTimeout` no longer exist (`grep -n '^func Lookup\|^var Tools\|^type Tool\|PeriodDaily\|DefaultTimeout' src/go/internal/source/ccusage/*.go` returns nothing) and no dead code remains
- [x] A-013 R4: `run_test.go` contains no `ccusage` reference

### Scenario Coverage

- [x] A-014 R1: a test proves registry order `cc, codex, oc, gemini, copilot, kimi`, `Lookup("gemini")` hits, and the aliases miss
- [x] A-015 R3: a test proves the key set of `invocations` equals the key set of `fact.Tools`, and `argv(fact.Lookup("cc"), "daily", nil)` is `claude daily --json`
- [x] A-016 R3: the existing `FetchAll` registry-order assertions (18 records; 6 errors for `weekly`) pass unchanged
- [x] A-017 R6 R7: the eighteen existing `TestTotalHistory*` / `TestHistory*` tests and every render golden pass with no test or golden edit (`git status` shows no `.golden` or `_test.go` change under `internal/view` or `internal/render`)

### Edge Cases & Error Handling

- [x] A-018 R6: significance fallbacks (`TestTotalHistorySignificanceFallbacks`), the empty and single-row windows, the indicator reserve, and token mode behave identically after the split
- [x] A-019 R3: a fetch error still carries `source.Error{Tool: key, Name: display name}` built from the `fact.Tool` (the existing spawn-ENOENT / Command-failed detail tests pass)

### Code Quality

- [x] A-020 Pattern consistency: new helpers follow the surrounding unexported lower-camel naming and doc-comment style (`// name does X`); tests are `_test.go` siblings (Go Transition article)
- [x] A-021 No unnecessary duplication: one `barBudget`; `invocations` values are not duplicated in `fact`
- [x] A-022 No god functions: every function touched or added is under 50 lines
- [x] A-023 No magic strings/numbers: the registry keys/names live only in `fact.Tools` and `invocations`; width constants stay named
- [x] A-024 Readability over cleverness: `TotalHistory` and `History` read top-to-bottom as build values → pick visible → size → budget → rows → totals
- [x] A-025 gofmt / go vet clean; no `-update` golden run in the diff

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`
- Environment: `TU_METRICS_REPO` and `NO_COLOR` leak from the shell and flip harness cases to multi mode — always run the harness as `env -u TU_METRICS_REPO -u NO_COLOR just go-diff --placeholder`. `just go-diff` depends on `just build` (needs `node_modules`; `npm ci` once in a fresh worktree).

## Deletion Candidates

- None — the planned removals (`ccusage.Tool`/`Tools`/`Lookup`/`PeriodDaily`/`DefaultTimeout`, the run_test compile-time assertion, the function-local `rowData` type) were all applied during apply; review found no further code this change makes redundant or unused.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | `pivotRow` is promoted to a package-level type (the intake's function-local `rowData`) | Helpers must share it; a function-local type cannot cross function boundaries | S:80 R:95 A:90 D:85 |
| 2 | Confident | `barBudget` lives in `history.go` beside `gutterWidth`/`barLeading` (or `bar.go`) — apply picks one owner | Either file is a natural home; the intake left placement to apply | S:60 R:95 A:85 D:65 |
| 3 | Confident | The ccusage registry test's key-set assertion replaces the removed six-struct DeepEqual | The DeepEqual's content (keys/names/order) now lives in `fact.tool_test.go`; the adapter's invariant is "one invocation per registry tool" | S:75 R:90 A:90 D:80 |
| 4 | Certain | No `-update` golden run; a changed `.golden` file is a review failure | Byte-identical output is the acceptance bar; a golden update would hide a regression | S:95 R:90 A:95 D:95 |

4 assumptions (1 certain, 3 confident, 0 tentative).
