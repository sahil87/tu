# Intake: G1 rework 1 — tool registry to `fact`, history tables split

**Change**: 260916-m9of-g1-rework-1
**Created**: 2026-09-16

## Origin

One-shot `/fab-new` invocation by the G1 operator loop (plan § G1 review protocol: "On `REWORK`: the operator spawns one fab change `g1-rework-<n>` via `fab-new` whose raw text is the verdict file's findings verbatim plus the standard context prefix"). Raw input:

> Context: fab/plans/sahil/26-09-15-go-port.md, row V3 (G1 cycle 1), verdict fab/plans/sahil/reviews/g1-1.md. Read the Decisions and Target architecture sections before writing the intake. Do not change any external surface listed under Goal. Scope: findings 1 and 2 above, exactly as written; harness must stay at >= 134/368 green with the same case set; no new behavior.
>
> Finding 1: internal/command is welded to the ccusage adapter (Fetcher.Fetch takes ccusage.Tool; run.go reads ccusage.Tools/Lookup/PeriodDaily/DefaultTimeout). Move Tool{Key,Name}/Tools (column order)/Lookup to internal/fact (or a tiny internal/tools); keep PrefixArgs/LabelKey in ccusage as an adapter-private map keyed by the fact key; Fetcher.Fetch takes fact.Tool (or the key string); move PeriodDaily/DefaultTimeout to source or command. After the fix, command imports source and fact for fetching and no source/* adapter; cmd/tu is the only place that names ccusage.
>
> Finding 2: view.TotalHistory (internal/view/pivot.go:46-178, 132 lines) interleaves five concerns (label union/value map, significance filter, width budget, bar scale/apportioning, row assembly). Extract at least pivotWidths(...), pivotRows(...), and the value-map build into named helpers. History (view/history.go, 83 lines) shares the same structure and should follow the same split so the two tables stay parallel. Leave command/parse.go's Parse/scanFlags alone (long but flat flag tables).

The verdict file `fab/plans/sahil/reviews/g1-1.md` (reviewed at `origin/main` 45b5f08 — V1 #84, V2 #85, B1 #86, B2 #87) is the authority for scope. Its follow-ups 3–5 (`view` importing `render` for number formatting; test volume; the JSON/CSV-from-`Series` vs table-from-`Table` split) are **out of scope** — they become backlog rows, not rework. No conversation preceded this invocation; every decision below comes from the verdict text, the plan's § Decisions / § Target architecture, and the current source.

## Why

**The problem.** `internal/command/run.go` names the ccusage adapter in four places: the `Fetcher` interface (`Fetch(ctx, tool ccusage.Tool, …)`), the deadline (`ccusage.DefaultTimeout`), the period vocabulary (`ccusage.PeriodDaily`), and the column-order registry (`ccusage.Tools`, `ccusage.Lookup`). The plan's § Target architecture puts `command` above `source` as a composer of *adapters*; a `Fetcher` whose parameter type is one adapter's struct is nominal, not a seam. Row B3 must add `source/metrics` implementing `Fetcher`, and today it cannot without importing `source/ccusage` for the parameter type — and `command` would grow a second concrete adapter import. The registry itself (key, display name, column order) is a property of the fact model — `fact.Record.Tool` already documents itself as "the registry key (cc, codex, oc, gemini, copilot, kimi)" — not of one exec adapter.

**The second problem.** `view.TotalHistory` is 132 lines and `view.History` 83, each interleaving value-map construction, the width budget, the bar scale, and row assembly in one body. `fab/project/code-quality.md` names ">50 lines without clear reason" a god-function anti-pattern. Rows B4 (machine columns, stacked bars) and B5 (`lbh` hooks — the function's own doc comment already reserves them) extend exactly this table. Left as is, both rows add lines to a body that has no named joints, and the single-tool table drifts from the pivot.

**If we don't fix it now.** The plan's gate G1 exists because "the layering is the one decision that is expensive to reverse once six more rows are stacked on it." B8, B3, B4, B5, B6, B7 queue behind this gate. Every one of them either implements `Fetcher` (B3), adds a column source to the pivot (B4), adds a row source to it (B5), or fills `HistoryOptions.Prev` (B7). Fixing the two seams after those rows land means touching six rows' worth of code instead of two files' worth.

**Why this approach.** The verdict prescribes the fix and the operator's request says "exactly as written". Both findings are pure refactors: the external surface (CLI grammar, output bytes, exit codes) is untouched by construction — the harness at `134/368` with the same green case set, the render golden files, and `go test ./...` are the proof. Alternatives the verdict already weighed: a new `internal/tools` package (allowed, but `fact` is the package every stage already imports and its doc already speaks of the registry key — one fewer package for ~30 lines); taking a key `string` in `Fetch` (allowed, but then every adapter re-looks-up and must handle a miss the command edge already handled).

## What Changes

All paths below are under `src/go/`. Nothing outside `src/go/`, `docs/memory/`, and `fab/changes/` changes. No file under `src/node/`, `harness/`, `Formula/`, or `.github/` is touched.

### 1. Tool registry moves from `source/ccusage` to `fact`

**New in `internal/fact`** (file `internal/fact/tool.go`, tests in `tool_test.go` — Go Transition article: `_test.go` siblings):

```go
// Tool is one entry of the tool registry: tu's key and the display name.
// Adapter-specific data (how ccusage is invoked for the tool) lives in the
// adapter, keyed by Key.
type Tool struct {
	Key  string // cc, codex, oc, gemini, copilot, kimi
	Name string // Claude Code, Codex, OpenCode, Gemini, Copilot, Kimi
}

// Tools is the registry in column order (Output Stability: insertion order is
// the all-tools column order; new tools append). It is a slice, never a map.
// Source aliases (co, gem, cop, ki) are command grammar, not registry entries.
var Tools = []Tool{
	{Key: "cc", Name: "Claude Code"},
	{Key: "codex", Name: "Codex"},
	{Key: "oc", Name: "OpenCode"},
	{Key: "gemini", Name: "Gemini"},
	{Key: "copilot", Name: "Copilot"},
	{Key: "kimi", Name: "Kimi"},
}

// Lookup returns the tool with the given registry key.
func Lookup(key string) (Tool, bool)
```

Order, keys, and names are byte-identical to today's `ccusage.Tools` (they drive the all-tools column order and every `Name` cell). `fact`'s package doc gains one sentence naming the registry; the existing "Tool holds the registry key" sentence stays.

**`internal/source/ccusage/registry.go` shrinks to the adapter-private map:**

```go
// invocation is how ccusage is driven for one registry tool: the per-agent
// subcommand and the JSON key carrying the ISO date label.
type invocation struct {
	prefixArgs []string
	labelKey   string
}

// invocations is keyed by fact.Tool.Key; every registry tool has an entry
// (asserted by registry_test.go against fact.Tools).
var invocations = map[string]invocation{
	"cc":      {prefixArgs: []string{"claude"}, labelKey: "date"},
	"codex":   {prefixArgs: []string{"codex"}, labelKey: "date"},
	"oc":      {prefixArgs: []string{"opencode"}, labelKey: "date"},
	"gemini":  {prefixArgs: []string{"gemini"}, labelKey: "date"},
	"copilot": {prefixArgs: []string{"copilot"}, labelKey: "date"},
	"kimi":    {prefixArgs: []string{"kimi"}, labelKey: "date"},
}
```

`argv(tool fact.Tool, period, extraArgs)` reads `invocations[tool.Key].prefixArgs`; `Parse(raw, tool fact.Tool)` reads `.labelKey`. The exported `ccusage.Tool`, `ccusage.Tools`, `ccusage.Lookup`, and `ccusage.PeriodDaily` are **removed** (not aliased — there is no external consumer of an unshipped tree, and a type alias would keep the weld). `Source.Fetch(ctx, tool fact.Tool, period, extraArgs, fresh)`; `FetchAll` iterates `fact.Tools`. A registry test asserts that `invocations` has exactly the keys of `fact.Tools` (no orphan, no missing entry), replacing today's `TestTools` DeepEqual on the six-struct slice. The argv assertions (`Tools[0].PrefixArgs[0] == "claude"`) move onto `invocations["cc"]`.

**`PeriodDaily` and `DefaultTimeout` move to `internal/source`** (file `internal/source/fetch.go`):

```go
// PeriodDaily is the one period every adapter is asked for: tu fetches daily
// records only and rolls up client-side (query.RollUp).
const PeriodDaily = "daily"

// DefaultTimeout is the per-invocation deadline the command edge puts on the
// context it hands to a Fetcher. Adapters impose no deadline of their own;
// they honor the given context only.
const DefaultTimeout = 120 * time.Second
```

`source` is already the adapter-neutral package (it owns `Error` and `WriteWarnings`), and both constants describe the fetch contract every adapter honors — B3's `source/metrics` and B6's `sync` fetch under the same deadline and the same period vocabulary. `command` is the sole consumer today; it reads `source.PeriodDaily` / `source.DefaultTimeout`.

**`internal/command/run.go` after the fix:**

```go
import (
	// … config, fact, query, render/*, source, view — no source/* adapter
)

// Fetcher is what command needs from a source. *ccusage.Source satisfies it
// (asserted where cmd/tu assigns it); B3's *metrics.Source will too.
type Fetcher interface {
	Fetch(ctx context.Context, tool fact.Tool, period string, extraArgs []string, fresh bool) ([]fact.Record, *source.Error)
	FetchAll(ctx context.Context, period string, extraArgs []string, fresh bool) ([]fact.Record, []*source.Error)
}
```

In `Run`: `context.WithTimeout(ctx, source.DefaultTimeout)`; `tool, _ := fact.Lookup(req.Source)`; `deps.Source.FetchAll(ctx, source.PeriodDaily, …)` / `Fetch(ctx, tool, source.PeriodDaily, …)`. In `runSnapshot` and `runHistory`: `tools := fact.Tools` and `[]fact.Tool{tool}`. The `Fetcher` interface **stays in `command`** (the verdict does not move it; the consumer defines the seam).

**`internal/command/run_test.go`:** the `fakeFetcher.Fetch` signature takes `fact.Tool`; its registry loops use `fact.Tools`; the compile-time assertion `var _ Fetcher = (*ccusage.Source)(nil)` is **deleted** together with the test's ccusage import. The assignment `Source: src` in `cmd/tu/main.go` (where `src` is `*ccusage.Source`) is the compile-time check that the adapter satisfies the interface — it already exists and fails the build on any drift. Rationale: the verdict's post-condition is "cmd/tu is the only place that names ccusage", and a test-only import of the adapter from `command` keeps a dependency edge the package graph should not have.

**`cmd/tu/main.go`:** unchanged in behavior; `ccusage.Source{Cache: store}` stays. Import graph after the fix, asserted by a grep in the plan's acceptance:

```
grep -rl 'internal/source/ccusage"' src/go --include='*.go'
→ src/go/cmd/tu/main.go   (and ccusage's own package files)
```

**Other consumers to sweep** (all under `src/go`): `internal/source/ccusage/{normalize,source}_test.go` (`Lookup`/`Tools` → `fact.Lookup`/`fact.Tools`; `Parse(raw, tool)` keeps taking the tool); `internal/harness` has its own unrelated `Corpus.Lookup` — untouched. `internal/source/cache` keys on `tool.Key` strings already — untouched.

### 2. `view.TotalHistory` and `view.History` split into parallel named helpers

No exported symbol in `view` changes. `TotalHistory(series, o) Table` and `History(s, o) Table` keep their signatures, their doc comments (the bullet lists are the behavioral contract memory records), and produce byte-identical `Table` values — the existing `pivot_test.go`, `history_test.go`, and every render golden pin this. The helpers are **unexported**; the split mirrors the five concerns the verdict names. Concern → helper, both tables:

| Concern | `pivot.go` (TotalHistory) | `history.go` (History) |
|---------|---------------------------|------------------------|
| Value map / values | `pivotValues(series, m) []map[string]float64` — tool → label → `metricValue` (today's lines 62–71) | `historyValues(s, m) (values []float64, sum float64)` (today's 117–123) |
| Row data (visible values, rowValue, toolSums, grandTotal) | `pivotData(series, valueMap, visible, labels) (rows []pivotRow, toolSums []float64, grandTotal float64)` with `pivotRow{label string; values []float64; rowValue float64}` promoted from the function-local type (today's 75–97) | *(not needed — the single-tool row IS the entry)* |
| Width budget | `pivotWidths(series, visible, rows, toolSums, rowValues, grandTotal, m) (toolWidths []int, costWidth int)` (today's 99–114) | `costWidth := metricColumnWidth(append(values, sum), m)` stays a one-liner; the fixed columns are the `historyColumns(costWidth, m) []Column` literal (today's 132–140) |
| Bar scale | shared `barBudget(width, bodyWidth, costWidth, reserve int) (barWidth int, show bool)` = `min(width − bodyWidth − gutterWidth − costWidth − barLeading − reserve, maxBarWidth)`, `show = barWidth ≥ minBarArea` — the two tables' formulas differ only in `bodyWidth` (97 fixed vs. `pivotDateWidth + Σ(toolWidth + gutter)`) and `reserve` (0 vs. the `Prev != nil` indicator) | same `barBudget(o.Width, historyBodyWidth, costWidth, 0)` |
| Row assembly (separators, data rows, bars) | `pivotRows(rows, visible, series, o, m, scale, showBars) []Row` — header, divider, month separators, data rows with `rowBar(rowValue, scale, values)` (today's 130–160) | `historyRows(s, values, o, m, scale, showBars) ([]Row, fact.Totals)` — returns the rows and the summed totals the Total row needs (today's 141–173) |
| Total / footer / legend | `pivotTotals(...)` or inline in `TotalHistory` — the `len(labels) > 1` block (162–177); the legend loop may stay inline (5 lines) | inline in `History` — the `len(s.Entries) > 1` block (175–186) |

Exact parameter lists are the apply stage's call — the contract is: each of `TotalHistory` and `History` becomes a short composition (build values → pick visible → size → budget bars → assemble rows → totals/footer/legend) that reads top to bottom in well under 50 lines, every helper is named for one concern, the two files use the same verbs (`*Values`, `*Widths`/`*Columns`, `*Rows`) so B4 adds a column source and B5 a row source by adding a helper, not lines. `significant`, `nonzero`, `LabelUnion`, `rowDelta`, `monthPrefixOf`, `footerText`, `rowBar`, `ComputeScale`, `Apportion` are untouched. `command/parse.go` (`Parse`, `scanFlags`) is **explicitly untouched** per the verdict.

Constants stay where they are (`negligible*`, `pivotDateWidth` in `pivot.go`; `history*Width`, `gutterWidth`, `barLeading` in `history.go`; `maxBarWidth`, `minBarArea` in `bar.go`).

### 3. Verification bar (no new behavior)

Run from the repo root, in this order; every step must be clean:

```sh
just go-lint                       # gofmt -l empty, go vet clean
just go-test                       # go test ./... -count=1 — all 19 packages ok, goldens unchanged (no -update run)
env -u TU_METRICS_REPO -u NO_COLOR just go-diff --placeholder   # ≥ 134/368 green, same case set
```

The harness comparison is the load-bearing check: the report's green set must be a superset of the pre-change green set (134 cases at 45b5f08 — capture `just go-diff --placeholder` on `main` first and diff the per-case results, not just the count). `TU_METRICS_REPO` / `NO_COLOR` must be unset in the shell that runs it (a leaked value flips single-mode cases to multi mode). The e2e test `cmd/tu/e2e_test.go` runs under `go test` and pins byte-exact stdout for the ported surfaces.

Grep-level acceptance for finding 1:

```sh
grep -rl 'internal/source/ccusage"' src/go --include='*.go' | grep -v '^src/go/internal/source/ccusage/'
# → exactly: src/go/cmd/tu/main.go
grep -n 'ccusage' src/go/internal/command/*.go
# → no import lines; comments may still say "ccusage" (e.g. the Fetcher doc)
```

Line-count acceptance for finding 2: `TotalHistory` and `History` bodies each < 50 lines; no helper introduced exceeds 50 lines.

## Affected Memory

- `go-port/fact-and-sources`: (modify) the "Ordered six-tool registry and argv composition" requirement — `fact.Tool{Key, Name}` / `fact.Tools` / `fact.Lookup` own the registry; `ccusage` keeps only the adapter-private `invocations` map keyed by fact key; `PeriodDaily` and `DefaultTimeout` now in `source`; the "Registry is an ordered slice" and "Caller-owned timeout with an exported default" design decisions re-homed accordingly; the `fact` requirement gains the registry; the FetchAll requirement iterates `fact.Tools`; the run_test / `Fetcher` sentence in the overview updated.
- `go-port/command-edge`: (modify) `Run` steps 1–4 — `source.DefaultTimeout`, `source.PeriodDaily`, `fact.Lookup`/`fact.Tools` in place of the ccusage names; the `Fetcher` seam sentence ("`*ccusage.Source` satisfies it" → asserted at the `cmd/tu` assignment); the import-graph statement that `command` imports no `source/*` adapter.
- `go-port/query-view-render`: (modify) the design decision "view receives display names, not registry keys" — its **Rejected** line ("Moving the registry into `fact` (churns the input layer for no gain)") is superseded by this change and must be rewritten (the decision's *Decision* and *Why* still hold: `command` resolves `fact.Lookup(key).Name`); the History / TotalHistory requirements are behavior and stay as written; a new design decision records the helper split (concern-per-helper, shared `barBudget`, parallel naming across the two tables, why: B4/B5 add a column/row source rather than lines).

No spec change: `docs/specs/usage.md` and `layouts.md` describe external surfaces, none of which move.

## Impact

- **Code**: `src/go/internal/fact/{fact.go,tool.go,tool_test.go}`, `src/go/internal/source/fetch.go` (new), `src/go/internal/source/ccusage/{registry.go,registry_test.go,exec.go,normalize.go,normalize_test.go,source.go,source_test.go}`, `src/go/internal/command/{run.go,run_test.go}`, `src/go/internal/view/{pivot.go,history.go}` (+ their tests only if a helper earns a direct unit test; the existing table tests already pin the outputs). `src/go/cmd/tu/main.go` unchanged.
- **Package graph**: `command → {config, fact, query, render/*, source, view}`; `source/ccusage → {fact, source, source/cache}`; `fact` and `source` import nothing internal. `view` still imports `render` (verdict follow-up 3 — out of scope).
- **Downstream rows unblocked**: B3 (`source/metrics` implements `command.Fetcher` against `fact.Tool`), B4/B5 (add helpers to `pivot.go`), B6 (`sync` depends on `fact` + `source/metrics`, never on the ccusage adapter — plan § Target architecture).
- **External surface**: none. Byte-identical output, same exit codes, same stderr. Constitution Principles I, II, V hold; the Go Transition "unshipped" clause holds (no formula, release, or `package.json` file touched).
- **Then**: the operator waits for this change's verified merge and re-runs the review as G1 cycle 2 (`fab/plans/sahil/reviews/g1-2.md`).

## Open Questions

None. Every decision point is graded Certain or Confident below; the verdict text fixed the scope and the reviewer's two "or" alternatives (`fact` vs. `internal/tools`; `fact.Tool` vs. key string; `source` vs. `command` for the constants) are resolved in the Assumptions table with rationale.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | Registry (`Tool{Key,Name}`, `Tools`, `Lookup`) lives in `internal/fact`, not a new `internal/tools` package | Verdict lists `fact` first; `fact` is the package every stage already imports and its doc already says `Tool` "holds the registry key"; a fourth package for ~30 lines adds an import to command, ccusage, and B3 for no isolation gain. Reversible with a mechanical move. | S:80 R:70 A:75 D:65 |
| 2 | Confident | `Fetcher.Fetch` takes `fact.Tool`, not the key `string` | The command edge already resolves `Lookup(req.Source)` (grammar validated the key), so passing the struct spares every adapter a re-lookup and a miss branch; `source.Error{Tool, Name}` needs the name anyway. Verdict allows either. | S:75 R:85 A:80 D:65 |
| 3 | Confident | `PeriodDaily` and `DefaultTimeout` move to `internal/source` (new `fetch.go`), not to `command` | Both describe the fetch contract every adapter honors (B3 metrics, B6 sync fetch under the same deadline and period vocabulary); `source` is already the adapter-neutral home (`Error`, `WriteWarnings`). `command` is the only consumer today, so either home compiles; one home for both beats splitting them. | S:70 R:90 A:65 D:55 |
| 4 | Certain | The `Fetcher` interface stays in `command` | Verdict does not ask to move it; the consumer defines the seam (Go idiom); moving it would be new scope. | S:85 R:90 A:90 D:90 |
| 5 | Certain | ccusage keeps `PrefixArgs`/`LabelKey` as an unexported `map[string]invocation` keyed by fact key; a test asserts its key set equals `fact.Tools`' keys; the old exported `ccusage.Tool/Tools/Lookup/PeriodDaily` are removed, not aliased | Verdict says "adapter-private map keyed by the fact key" verbatim; an alias would keep the weld; the key-set test replaces the lost DeepEqual on the slice. | S:90 R:90 A:90 D:85 |
| 6 | Confident | `command/run_test.go` drops its ccusage import and the `var _ Fetcher = (*ccusage.Source)(nil)` assertion; the `Source: src` assignment in `cmd/tu/main.go` is the compile-time check | Verdict's post-condition "cmd/tu is the only place that names ccusage" is about the dependency graph, and a test-only edge is still an edge; the main.go assignment already fails the build on drift. | S:60 R:95 A:80 D:70 |
| 7 | Certain | View helpers are unexported; `TotalHistory`/`History` signatures, doc contracts, and `Table` outputs are byte-identical (pinned by existing view tests and render goldens; no `-update` run) | "No new behavior" + Output Stability; the verdict asks for internal extraction only. | S:85 R:90 A:95 D:90 |
| 8 | Confident | Beyond the verdict's minimum (`pivotWidths`, `pivotRows`, value-map build), extract a shared `barBudget(width, bodyWidth, costWidth, reserve)` used by both tables, and give `History` the parallel `historyValues`/`historyColumns`/`historyRows` | The two bar formulas differ only in body width and reserve; the verdict wants the tables "parallel" and B4 (token-metric/stacked bars) touches exactly this budget. Small, reversible, in the spirit of the finding. | S:55 R:90 A:80 D:60 |
| 9 | Certain | Hydrate rewrites the "view receives display names" design decision's Rejected line in `go-port/query-view-render` and re-homes the registry / timeout decisions in `go-port/fact-and-sources`; `command-edge` follows the renamed symbols | Memory is present-truth; leaving "Rejected: moving the registry into fact" after doing exactly that is drift. | S:80 R:95 A:90 D:90 |
| 10 | Certain | Change type is `refactor` | No behavior, no surface, no new capability; the type inference keys on "refactor"/"redesign"-class text and this intake is a pure restructuring — verified after `fab status refresh`, overridden with `set-change-type` only if it lands elsewhere. | S:90 R:95 A:95 D:95 |
| 11 | Certain | Verification = `just go-lint`, `just go-test`, `env -u TU_METRICS_REPO -u NO_COLOR just go-diff --placeholder` with the green **set** (not just the count) compared against a fresh `main` run; follow-ups 3–5 of the verdict are not touched | Operator text: "harness must stay at ≥ 134/368 green with the same case set"; plan § G1 review protocol: follow-ups become backlog rows, not rework. | S:95 R:95 A:90 D:95 |

11 assumptions (6 certain, 5 confident, 0 tentative, 0 unresolved).
