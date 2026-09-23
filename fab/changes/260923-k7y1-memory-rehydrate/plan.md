# Plan: Memory Rehydrate (plan row X3)

**Change**: 260923-k7y1-memory-rehydrate
**Intake**: `intake.md`

> Docs-only change. Nothing under `src/`, `harness/`, `justfile`, `.github/`, `Formula/`, `scripts/`, `README.md` or `fab/project/` is touched. The deliverable is the `docs/memory/` tree itself plus two hunks in `docs/specs/usage.md`.

## Requirements

### Memory: target tree

#### R1: Package-aligned domains
`docs/memory/` MUST contain exactly these topic domains after apply: `fact`, `source`, `query`, `view`, `render`, `command`, `sync`, `config`, `watch`, `toolkit`, `build`, `harness`. The folders `cli/`, `configuration/`, `display/`, `watch-mode/` and `go-port/` MUST NOT exist (deleted whole: topic files, `index.md`, `log.md`, `log.seed.md`). `sync/multi-machine.md` MUST NOT exist. Every new domain folder MUST carry an `index.md` stub consisting of a frontmatter block with only a `description:` one-liner (≤ 500 chars, change-id-free) — no body; kept folders (`sync`, `build`, `harness`) keep their existing generated `index.md` untouched.

- **GIVEN** the apply has finished
- **WHEN** `ls docs/memory` is run
- **THEN** it lists `build`, `command`, `config`, `fact`, `harness`, `index.md`, `query`, `render`, `source`, `sync`, `toolkit`, `view`, `watch` and nothing else
- **AND** `ls docs/memory/fact` shows `index.md` plus the topic files, and `head -4 docs/memory/fact/index.md` shows only `---`, `description: …`, `---`

#### R2: File sizing and descriptions
Every topic file written by this change MUST be ≤ 400 lines and ≤ 15 KB — except the two kept Go-native files `build/go-release-pipeline.md` and `harness/differential-harness.md`, which receive only R17's framing pass, keep their content and size, and are split (never shrunk) by `/docs-reorg-memory` if it proposes so — and MUST lead with FKF frontmatter `type: memory` plus a single-line `description:` of at most 500 characters carrying no change id (neither a `YYMMDD-xxxx-slug` token nor a bare registered 4-char id). The intake § 1 file map is the starting partition; the apply MAY merge or split files **within** a domain when the source warrants, never across domains.

- **GIVEN** any file `docs/memory/*/*.md` that is not `index.md`, `log.md` or `log.seed.md`
- **WHEN** its line count, byte size and `description:` length are measured
- **THEN** all three are within the caps and `fab docs-index docs/memory --check --json` reports no `file-size`, `narration-density` or description-length warning naming it

### Memory: authoring contract

#### R3: Written from the Go source, present truth only
Every requirement statement in a new or rewritten body MUST be derived from `src/go/internal/**`, `src/go/cmd/tu/main.go`, their `_test.go` siblings and `testdata/` goldens as they exist on this branch, and MUST name at least one Go identifier or file per `### Requirement:` block so a reviewer can locate it. Numeric constants (TTLs, timeouts, fallback geometry, buffer sizes, the 3-month floor, the compact threshold) MUST match the code. Bodies MUST NOT contain: a `.ts` path, `src/node`, `__tests__`, a TS function name (`parseGlobalFlags`, `fetchToolMerged*`, `renderTotal*`, `writeMetrics`, `aggregateForPeriod`, `mergeEntries`, `maxMergeEntries`, …), or the phrases "the Go port", "unshipped", "until cutover", "the TS does", "pinned to the TS", "the TS path". Sanctioned exceptions: the closing retired-tree section of `build/toolchain.md`; `harness/differential-harness.md` (whose oracle is `node dist/tu.mjs`); and a Design Decision whose **Why** is byte or wire parity, which may say "parity with the frozen `src/node/` oracle (until plan row Z1)".

- **GIVEN** the finished tree
- **WHEN** `grep -rlE 'src/node|\.ts\b|__tests__|the Go port|unshipped|until cutover|the TS ' docs/memory --include='*.md'` runs
- **THEN** it lists at most `docs/memory/build/toolchain.md` and `docs/memory/harness/differential-harness.md`

#### R4: FKF v0.1 body shape
Each topic file MUST follow the conventional shape: `# {Name}`, `**Domain**: {domain}`, `## Overview` (1–2 sentences), `## Requirements` with `### Requirement: {Name}` blocks in RFC 2119 voice (`#### Scenario:` GIVEN/WHEN/THEN where the behavior has an observable edge), and `## Design Decisions` entries in the four-field shape (**Decision** / **Why** / **Rejected** / *Introduced by*). No `## Changelog`, no change id in any heading, no operational TODOs, no transition narration ("renamed", "now", "previously", "no longer", "was `old`"). Provenance is citation-only: a trailing `(id)` names the change that made the decision (TS-era ids such as `srmi`, `wkly`, `4xwg`, `gzrn`, `svlv` are valid when the decision predates the port; Go rows `v0as`, `3am6`, `4fs0`, `9ax5`, `xivf`, `pmsd`, `2gbb`, `lsml`, `4pze`, `vcur`, `0118`, `489t`, `jmh4`, `uerc`, `m9of` for port-specific choices).

- **GIVEN** any new topic file
- **WHEN** it is read
- **THEN** its H2 set is a subset of {Overview, Requirements, Design Decisions}, every DD entry has the four fields, and no heading line contains a 4-char change id or a date

#### R5: Design Decisions carried forward
Rationale that still holds MUST survive as Design Decisions, harvested from the old files where the Go code confirms the behavior: at minimum the never-shrink day-file guard, write-then-max-merge for the own machine, Sunday-anchored weeks, the p95 bar scale, the 3-month history cap, the 60 s cache TTL, cost carried unrounded to render time, the reserved `all` user, `--top` fold/collapse semantics, `NO_COLOR`, the `/Cellar/tu/` update gate, SIGTERM-graceful brew bounds, raw mode keeping `OPOST|ONLCR`, the embedded skill/completions/default-conf pattern, and the per-tool `ccusage` invocation table. Each carries its originating citation.

- **GIVEN** the finished tree
- **WHEN** `grep -rn 'never-shrink\|never shrink' docs/memory/sync` and `grep -rn 'OPOST' docs/memory/watch` run
- **THEN** each hits a `## Design Decisions` entry or a `### Requirement:` block with a citation

#### R6: Cross-links resolve
Memory-to-memory links MUST use the bundle-relative form `](/domain/file.md)` resolved from `docs/memory/`; links out of the bundle stay repo-relative. Every bundle-relative link in the tree MUST resolve to an existing file. Each domain SHOULD link its upstream and downstream neighbour in the pipeline at least once (e.g. `command` → `query`, `view`, `render`, `source`; `sync` → `source/metrics-reader`).

- **GIVEN** the finished tree
- **WHEN** every `](/…/….md` target is checked for existence under `docs/memory/`
- **THEN** zero targets are missing

### Memory: domain content (one requirement per domain; the parenthesised lists are the minimum coverage, taken from the intake § 1 map)

#### R7: `fact/`
Files `records.md` (`fact.Record`, `fact.Totals`, `Add`, `IsZero`, ISO label rule, the pinned JSON tags, Date/Tool/User/Machine semantics) and `tool-registry.md` (`fact.Tool`, `fact.Tools`, `Lookup`, the six keys `cc codex oc gemini copilot kimi`, aliases, display names, `ccusage` subcommand per tool, column order). Source: `src/go/internal/fact/*.go`.

- **GIVEN** `docs/memory/fact/tool-registry.md`
- **WHEN** compared with `fact.Tools` in `tool.go`
- **THEN** the six keys, display names and order match exactly

#### R8: `source/`
Files `errors-and-warnings.md` (`source.Error` fields and `Error()` text, `WriteWarnings`, `PeriodDaily`, `DefaultTimeout`, the rule that only `cmd/tu` opens stderr), `ccusage-adapter.md` (`ccusage.Source`, binary resolution vendor-first relative to `os.Executable()` then `PATH`, the invocations map, exec with deadline and output buffer, `normalize.go`'s JSON → `[]fact.Record` mapping, `Fetch`/`FetchAll`, User/Machine stamping, the `command.Fetcher` assertion), `ccusage-json-shapes.md` (the v20 per-agent daily JSON shapes normalize accepts: `daily[]` rows, `totals`, the codex `costUSD`/`models{}` outlier, the empty-agent `daily: []` / `-0.0` form — from `normalize.go` and its `testdata`), `metrics-reader.md` (`metrics.DayFile`, `Name`, `Path`, the `{user}/{year}/{machine}/{tool}-{date}.jsonl` layout, `Read*` functions, user/machine enumeration, absent-file-is-no-data), `cache.md` (`cache` package: envelope struct, key derivation, TTL constant, location under the state dir, `--fresh` bypass). Source: `src/go/internal/source/**`.

- **GIVEN** `docs/memory/source/cache.md`
- **WHEN** compared with `cache.go`
- **THEN** the TTL value, key inputs and on-disk location match the code

#### R9: `query/`
Files `periods-and-windows.md` (`query.Period` values, label formats, Sunday week anchoring, `ThreeMonthFloor`, `Relabel`, `Window`/since-until filtering, inclusive/exclusive bounds) and `aggregation.md` (`RollUp`, `GroupBy` and its dims, `Collapse`, `MaxMerge` semantics — whole-record max by `TotalCost`, ties to live — and the summation order that pins `--json` float bytes). Source: `src/go/internal/query/*.go`.

- **GIVEN** `docs/memory/query/aggregation.md`
- **WHEN** compared with `merge.go`
- **THEN** the max-merge tie rule and the field compared match the code

#### R10: `view/`
Files `table-model.md` (`view.Table`, `CompactTable`, cell/column types, `Metric` cost vs tokens, data-sized columns, dimmed exact zeros, compact threshold), `snapshot.md`, `history.md` (single-tool history and all-tools pivot, month separators, weekend dimming, negligible-column omission, stacked tool bars, `MaxRows`), `leaderboard.md` (ranking, share, Δ vs previous window, `new`, `--top` collapse line and `others` fold, `◂` pin, `user/machine` keys), `bars-and-deltas.md` (eighth-block glyphs, p95 scale, `BarAfter`/leader cells, delta placements), `breakdown.md` (machine/user columns, legend text, footer stats). Source: `src/go/internal/view/*.go` + `testdata/`.

- **GIVEN** `docs/memory/view/bars-and-deltas.md`
- **WHEN** compared with `bar.go`
- **THEN** the glyph set and the percentile used for the scale match

#### R11: `render/`
Files `number-formatting.md` (`render.FormatInt`, `FormatCost`, `FixedHalfUp`, `JSRound`, where each is used), `ansi.md` (`ansi.Colors`, `NO_COLOR` handling, palette, table encoder, compact encoder, golden files), `json.md` (the pinned snapshot/history/leaderboard JSON shapes as the Go structs in `render/json/*.go`, key order, raw float costs), `csv-and-markdown.md`. Source: `src/go/internal/render/**` + goldens.

- **GIVEN** `docs/memory/render/json.md`
- **WHEN** compared with `render/json/snapshot.go`
- **THEN** the struct field names and JSON tags listed match the code

#### R12: `command/`
Files `request-and-parse.md` (`Request`, `Flags`, `Display`/`Format`/`Metric` enums, `Parse`, `UsageError`, `ShortUsage`, `FullHelp`, exit codes 0/1/2), `guards.md` (`Normalize`: since/until on snapshot, `-u` in single mode, `--top` off-leaderboard, `--by-machine` on the pivot / `lbh`, reserved user `all`, leaderboard-requires-multi, the 3-month cap and `--full`), `run-and-result.md` (`Run`, `Deps` with `Fetcher`/`Repo`/`Writer`, `Result` fields, `Live` per-poll options, `ErrUnported`), `multi-mode.md` (`gather` record paths by mode and `-u`, own-user write-then-max-merge, repo-only paths, `gatherAllUsers`, mode-keyed snapshot label rule, the auto-clone guard's placement), `entry-point.md` (`cmd/tu/main.go`: argument routing order, setup commands, `MetricsDirGuard` between `config.Load` and the reserved-user check, the only writer of the process streams, `sync`/`--sync`/`--dry-run` and `-w` branches, exit-code mapping). Source: `src/go/internal/command/*.go`, `src/go/cmd/tu/main.go`.

- **GIVEN** `docs/memory/command/request-and-parse.md`
- **WHEN** compared with `parse.go` and `request.go`
- **THEN** every flag in `Flags` appears with its long/short spelling and the exit code for a usage error is 2

#### R13: `sync/`
Files `day-file-writer.md` (`sync.Write`, the never-shrink guard and its `Number()` coercion table, `WriteDecision`, the shared `metrics.DayFile` encoding), `git-flow.md` (`Exec` driver, `SyncMetrics` add/commit/pull/push order, rebase-abort recovery, `CommitMessage`, `TouchLastSync`/`Stale`, auto-sync TTL, error text), `dry-run-report.md` (`FullSync` live vs dry-run on one decision path, `Report.Format` byte rules), `repair.md` (`sync.Repair`, `cmd/turepair`, `localecmp.go`). Source: `src/go/internal/sync/*.go`. `sync/multi-machine.md` is removed and `sync/log.seed.md` links rewritten to `/sync/day-file-writer.md`.

- **GIVEN** `docs/memory/sync/day-file-writer.md`
- **WHEN** compared with `writer.go`
- **THEN** the guard's comparison rule and the coercion table match the code

#### R14: `config/`
Files `cascade.md` (`ResolvePaths`, the five-layer `Load` order and its warning lines, legacy `~/.tu.conf` deprecation, embedded `tu.default.conf` and its drift guard, sentinel expansion, `StateDir`/`Tildefy`/`ExpandHome`, INI parsing rules), `setup-commands.md` (`InitConf`, `InitMetrics`, `CloneStep`), `status.md` (`Status`, `Lines`, `RelativeTime`, `LastSync`), `metrics-dir-guard.md` (`MetricsDirGuard`, `.clone-failed` marker semantics, `Cloner`). Source: `src/go/internal/config/*.go`.

- **GIVEN** `docs/memory/config/cascade.md`
- **WHEN** compared with `config.go`
- **THEN** the layer order and the environment variable name match

#### R15: `watch/`
Files `loop-and-terminal.md` (`watch.Run`'s select loop, key bindings, SIGWINCH/SIGINT, poll and countdown, re-entrancy guard, cleanup, `Terminal` seam, 80×24 fallback, TTY-gated raw mode keeping `OPOST|ONLCR`, `termios_{darwin,linux}.go`), `compositor.md` (`Lay`/`Frame`, skeleton, compact threshold, golden frames under a fake clock), `panel-and-rain.md` (`StatsGrid`, `BurnRate`, session stats, `RainState` on injected `rand/v2`, density, `--no-rain`). Source: `src/go/internal/watch/*.go` + goldens.

- **GIVEN** `docs/memory/watch/loop-and-terminal.md`
- **WHEN** compared with `terminal.go`
- **THEN** the fallback geometry and the termios flags preserved match the code

#### R16: `toolkit/`
Files `version-and-help-dump.md` (`BareVersion`, `DisplayVersion`, `VersionLine`, `BuildHelpDoc`/`Encode` without HTML escaping, the flat `commands: []` envelope, the shll.ai pull that consumes it), `update.md` (`Brew` interface and `BrewExec`, 600 s/60 s bounds with SIGTERM, unbounded `HOMEBREW_NO_ASK=1` upgrade, `IsBrewInstall` `/Cellar/tu/` gate, `--skip-brew-update`, the off-Homebrew message), `shell-init-and-completions.md` (embedded `completions/tu.{bash,zsh,fish}`, `shell-init <sh>` output, unknown-shell exit 2), `skill-bundle.md` (committed copy of `docs/site/skill.md`, `scripts/sync-skill.sh`, the `go test` drift guard, `tu skill` output), `standards-audit.md` (the shll v0.1.32 audit record and findings S1/V1, the toolkit-standards posture — moved here from `build/toolchain.md`). Source: `src/go/internal/toolkit/**`.

- **GIVEN** `docs/memory/toolkit/update.md`
- **WHEN** compared with `update.go`
- **THEN** the two timeout constants and the gate substring match

#### R17: `build/` and `harness/`
`build/toolchain.md` MUST be rewritten Go-first: module and Go version, `just go-build`/`go-build-all`/`go-lint`/`go-test`/`go-package`/`go-formula`/`harness-*` recipes, gofmt+vet gate, golden `-update` convention, CI lanes (`go-build-and-test`, `tudiff`, `ci-gate` ruleset + `scripts/ci-gate-ruleset.sh`), `scripts/release.sh` and the tag-anchored release, with exactly one closing section titled `## Retired src/node tree` (what it is, why it stays until Z1 — harness oracle and D10 rollback — its `npm ci && npm run build && npm test` toolchain in at most five lines). The help-dump-producer, skill-bundle and standards-posture material moves to `toolkit/`. `build/go-release-pipeline.md` and `harness/differential-harness.md` get a **framing pass only**: sentences calling Node "shipped" or Go "successor/unshipped" are rewritten to present truth (`node dist/tu.mjs` is the frozen oracle, `bin/tu` the shipped binary); requirements and DDs are otherwise unchanged and their descriptions trimmed to ≤ 500 chars only if over.

- **GIVEN** `docs/memory/build/toolchain.md`
- **WHEN** read top to bottom
- **THEN** the first `### Requirement:` concerns the Go module or `just go-*`, and `npm` appears only inside `## Retired src/node tree`

### Removals, seeds, and the index guard

#### R18: Deletions and the expected exit-2 check
The apply MUST delete the folders and file named in R1, MUST NOT run `fab docs-index docs/memory` (regeneration) and MUST NOT hand-edit any generated `index.md` body or `log.md`. After all writes it MUST run `fab docs-index docs/memory --check --json` and record the result in its apply result: the expected state is exit 2 with `losses[]` containing only `tombstone` entries (no `description`, no `grouping`), `malformed: []`. The tombstone relocation is `/docs-reorg-memory`'s job in the main session after review.

- **GIVEN** the apply has finished
- **WHEN** `fab docs-index docs/memory --check --json; echo $?` runs
- **THEN** the exit code is 2, every `losses[].category` is `tombstone`, and no `warnings[]` entry names a file under the twelve target domains

#### R19: Seed links in kept folders
`docs/memory/sync/log.seed.md` links of the form `](/sync/multi-machine.md)` MUST be rewritten to `](/sync/day-file-writer.md)`; `docs/memory/build/log.seed.md` needs no change unless it links a removed file. Seeds of removed folders are deleted with the folder.

- **GIVEN** `docs/memory/sync/log.seed.md`
- **WHEN** grepped for `multi-machine.md`
- **THEN** there are no hits

### Specs

#### R20: `docs/specs/usage.md` — two hunks only
(1) The ` ```typescript ` fence in § Data Model becomes a ` ```go ` fence containing the `Totals` and `Record` struct declarations verbatim from `src/go/internal/fact/fact.go` (field comments included, method bodies excluded); the prose sentence after the fence is kept and "two core interfaces" becomes "two core types". (2) The header blockquote's provenance sentence is rewritten to: "every observable behavior of the shipped binary — the Go build from `src/go/` (repository at v0.12.1; cutover release 0.13.0). The contract was reconciled 2026-09-16 against the then-shipped v0.11.5 Node binary and the memory tree of that date (capture set in `fab/changes/260915-2y3l-spec-reconciliation/reconciliation.md`). Internal mechanisms stay in `docs/memory/`." followed by the existing `(DC-NN)` sentence unchanged. `docs/specs/layouts.md`, `docs/specs/index.md` and `docs/site/skill.md` MUST NOT change.

- **GIVEN** `git diff origin/main -- docs/specs docs/site`
- **WHEN** inspected
- **THEN** it touches only `docs/specs/usage.md`, in two hunks, and `grep -c typescript docs/specs/usage.md` prints 0

### Scope guard

#### R21: Nothing outside `docs/`
`git diff --stat origin/main -- src harness justfile .github Formula scripts README.md fab/project package.json` MUST be empty at the end of apply. `go test ./internal/toolkit/...` (from `src/go/`) MUST still pass (the skill drift guard).

- **GIVEN** the finished branch
- **WHEN** the diff stat above runs
- **THEN** it prints nothing

### Non-Goals
- Relocating tombstone rows, merging thin domains, or regenerating indexes — `/docs-reorg-memory` does this after review.
- Rewriting `docs/specs/*.md` beyond R20, or `docs/site/*.md` at all.
- Fixing README's CI paragraph or `fab/project/code-review.md`'s `UsageEntry[]` rule (follow-ups).
- Resolving `[DECIDE]` markers (gate G0's ledger).

### Design Decisions

#### One domain per pipeline package
**Decision**: The domain set equals the `internal/` package list in constitution § Go Conventions plus `build` and `harness`.
**Why**: The package graph is the dependency graph a reader navigates; the Affected Memory walk and reviewers then load exactly the stage a change touches.
**Rejected**: Renaming `go-port/` (keeps transition framing and the five TS files); stage-grouped super-domains (hide the boundary the constitution enforces).
*Introduced by*: 260923-k7y1-memory-rehydrate

#### Apply stops at the exit-2 check; reorg owns the regeneration
**Decision**: The apply worker deletes retired folders, runs `fab docs-index docs/memory --check --json`, records the tombstone list and does not regenerate.
**Why**: FKF §6.4's refuse-before-regen guard exists so navigation loss is remediated by the one skill that authors `_shared/removed-domains.md`; forcing a regen from the worker would drop the removal history silently.
**Rejected**: The worker writing `_shared/removed-domains.md` itself (out of its remit, and the operator asked for the reorg pass).
*Introduced by*: 260923-k7y1-memory-rehydrate

### Deprecated Requirements

#### TS-symbol memory (`cli/data-pipeline`, `display/formatting`, `configuration/config-system`, `sync/multi-machine`, `watch-mode/tui`) and the `go-port/*` transition set
**Reason**: They document `src/node/` (frozen, not shipped, removed at Z1) or frame the Go code as an unshipped port.
**Migration**: Replaced by the twelve package-aligned domains above; removal records are written by `/docs-reorg-memory` into `_shared/removed-domains.md`.

## Tasks

### Phase 1: Setup

- [x] T001 Read the always-load layer, `intake.md` § 1–§ 5, `$(fab kit-path)/reference/fkf.md` §3 and `$(fab kit-path)/templates/memory.md`; list every Go source file under `src/go/internal/` and `src/go/cmd/tu/`; create the ten new domain folders with `description:`-only `index.md` stubs (`fact`, `source`, `query`, `view`, `render`, `command`, `config`, `watch`, `toolkit` new; `sync`, `build`, `harness` keep their generated `index.md`) <!-- R1 -->

### Phase 2: Core Implementation (dependency order; each task reads its package's `.go`, `_test.go` and `testdata/` before writing)

- [x] T002 Write `docs/memory/fact/records.md` and `docs/memory/fact/tool-registry.md` from `src/go/internal/fact/` <!-- R7 -->
- [x] T003 Write `docs/memory/source/{errors-and-warnings,ccusage-adapter,ccusage-json-shapes,metrics-reader,cache}.md` from `src/go/internal/source/**` <!-- R8 -->
- [x] T004 Write `docs/memory/query/{periods-and-windows,aggregation}.md` from `src/go/internal/query/` <!-- R9 -->
- [x] T005 Write `docs/memory/view/{table-model,snapshot,history,leaderboard,bars-and-deltas,breakdown}.md` from `src/go/internal/view/` and its `testdata/` <!-- R10 -->
- [x] T006 Write `docs/memory/render/{number-formatting,ansi,json,csv-and-markdown}.md` from `src/go/internal/render/**` and goldens <!-- R11 -->
- [x] T007 Write `docs/memory/command/{request-and-parse,guards,run-and-result,multi-mode,entry-point}.md` from `src/go/internal/command/` and `src/go/cmd/tu/main.go` <!-- R12 -->
- [x] T008 [P] Write `docs/memory/sync/{day-file-writer,git-flow,dry-run-report,repair}.md` from `src/go/internal/sync/` and `src/go/cmd/turepair/`; delete `docs/memory/sync/multi-machine.md`; rewrite `docs/memory/sync/log.seed.md` links to `/sync/day-file-writer.md` <!-- R13 -->
- [x] T009 [P] Write `docs/memory/config/{cascade,setup-commands,status,metrics-dir-guard}.md` from `src/go/internal/config/` <!-- R14 -->
- [x] T010 Write `docs/memory/watch/{loop-and-terminal,compositor,panel-and-rain}.md` from `src/go/internal/watch/` and goldens <!-- R15 -->
- [x] T011 Write `docs/memory/toolkit/{version-and-help-dump,update,shell-init-and-completions,skill-bundle,standards-audit}.md` from `src/go/internal/toolkit/**` (standards-audit content lifted from the audit sections of `docs/memory/go-port/toolkit-layer.md` and `docs/memory/build/toolchain.md`) <!-- R16 -->
- [x] T012 Rewrite `docs/memory/build/toolchain.md` Go-first from `justfile`, `.github/workflows/ci.yml`, `.github/workflows/release.yml`, `scripts/release.sh`, `scripts/ci-gate-ruleset.sh` with the single closing `## Retired src/node tree` section; framing pass on `docs/memory/build/go-release-pipeline.md` and `docs/memory/harness/differential-harness.md` <!-- R17 -->

### Phase 3: Integration & Edge Cases

- [x] T013 Delete `docs/memory/cli/`, `docs/memory/configuration/`, `docs/memory/display/`, `docs/memory/watch-mode/`, `docs/memory/go-port/` whole; harvest any Design Decision from them not yet carried (R5 list) into the owning new file before deleting; verify every bundle-relative link in the new tree resolves (R6) <!-- R1 -->
- [x] T014 Edit `docs/specs/usage.md`: the two hunks of R20 only <!-- R20 -->
- [x] T015 Run the verification set — size/description caps (R2), the ban-list grep (R3), heading/DD shape scan (R4), link resolution (R6), `fab docs-index docs/memory --check --json` expecting exit 2 with tombstone-only losses (R18), the scope-guard diff stat and `go test ./internal/toolkit/...` (R21) — fix anything that fails, and record the exact check output in the apply result summary <!-- R18 -->

## Execution Order

- T001 first. T002 → T003 → T004 → T005 → T006 → T007 in that order (each domain's cross-links point at files already written). T008 and T009 may run in parallel after T007. T010 after T007 (it links `command/run-and-result`). T011 after T007. T012 after T011 (toolchain hands material to toolkit). T013 after every write task. T014 any time after T002. T015 last.

## Acceptance

### Functional Completeness

- [x] A-001 R1: `ls docs/memory` shows exactly `build command config fact harness index.md query render source sync toolkit view watch`; each of the nine new domain folders has a `description:`-only `index.md` stub
- [x] A-002 R7: both `fact/` files exist; `tool-registry.md` lists the six keys in `fact.Tools` order with matching display names (verified against `tool.go`)
- [x] A-003 R8: the five `source/` files exist; `cache.md` states the TTL and key inputs found in `cache.go`; `ccusage-adapter.md` states vendor-first resolution relative to `os.Executable()` (verified against `cache.go`, `exec.go`)
- [x] A-004 R9: both `query/` files exist; `aggregation.md` states `MaxMerge`'s whole-record rule and tie-break as coded in `merge.go` (verified)
- [x] A-005 R10: the six `view/` files exist; `bars-and-deltas.md` names the glyph set and percentile from `bar.go` (verified: `eighths`, `p95Percentile = 95.0`, `p95TriggerFactor = 1.5`)
- [x] A-006 R11: the four `render/` files exist; `json.md` field names and tags match `render/json/*.go` (verified against `snapshot.go`, `history.go`, `leaderboard.go`)
- [x] A-007 R12: the five `command/` files exist; `request-and-parse.md` lists every `Flags` field with its spellings and the 0/1/2 exit codes; `entry-point.md` states `cmd/tu` is the only writer of the process streams (verified against `request.go`, `parse.go`, `main.go`)
- [x] A-008 R13: the four `sync/` files exist; `multi-machine.md` is gone; `day-file-writer.md`'s guard rule matches `writer.go` (verified incl. the `jsNumber` coercion table)
- [x] A-009 R14: the four `config/` files exist; `cascade.md`'s layer order matches `config.Load` (verified incl. `TU_METRICS_REPO`, legacy warning, `CurrentConfigVersion = 2`)
- [x] A-010 R15: the three `watch/` files exist; `loop-and-terminal.md` states the 80×24 fallback and the `OPOST|ONLCR` rule as coded in `terminal.go` (verified)
- [x] A-011 R16: the five `toolkit/` files exist; `update.md` states the 600 s / 60 s bounds and the `/Cellar/tu/` gate from `update.go` (verified, incl. the 10 s `brewGraceDelay`)
- [x] A-012 R17: `build/toolchain.md` opens with Go requirements and confines `npm` to `## Retired src/node tree`; `go-release-pipeline.md` and `differential-harness.md` no longer call Node "shipped" or Go "successor" (verified)
- [x] A-013 R20: `docs/specs/usage.md` differs from `origin/main` in exactly two hunks (Go fence; header sentence); `layouts.md`, `specs/index.md`, `docs/site/skill.md` unchanged (verified; fence matches `fact.go` verbatim)

### Behavioral Correctness

- [x] A-014 R3: `grep -rlE 'src/node|\.ts\b|__tests__|the Go port|unshipped|until cutover|the TS ' docs/memory --include='*.md'` — every topic-file hit is the R3-sanctioned DD **Why** parity sentence ("parity with the frozen `src/node/` oracle (until plan row Z1)"); remaining hits are generated `index.md`/`log.md` tombstone content, kept `log.seed.md` history, and the kept D10 rollback requirement in `go-release-pipeline.md` (unchanged from `origin/main`, framing-pass-only file per R17)
- [x] A-015 R4: no topic file has a `## Changelog`; no heading contains a change id; every `## Design Decisions` entry has Decision/Why/Rejected/*Introduced by* (script-verified across all topic files)
- [x] A-016 R5: the never-shrink guard, write-then-max-merge, Sunday weeks, p95 scale, 3-month cap, 60 s TTL, reserved `all`, `--top` semantics, `NO_COLOR`, `/Cellar/tu/` gate, SIGTERM bounds, `OPOST|ONLCR`, embedded assets, and the invocation table each appear with a citation (grep-verified)
- [x] A-017 R2: every topic file ≤ 400 lines and ≤ 15 KB; every `description:` single-line, ≤ 500 chars, change-id-free (script-verified; only the two R2-exempt kept files exceed the size cap)

### Removal Verification

- [x] A-018 R1: `docs/memory/{cli,configuration,display,watch-mode,go-port}` do not exist; no generated `index.md` body or `log.md` was hand-edited (`git diff origin/main -- 'docs/memory/**/index.md' 'docs/memory/**/log.md'` shows deletions only — 155 deletions, 0 insertions)
- [x] A-019 R19: `grep -c multi-machine.md docs/memory/sync/log.seed.md` prints 0 (the one historical seed entry now links `/sync/day-file-writer.md`)

### Scenario Coverage

- [x] A-020 R18: `fab docs-index docs/memory --check --json` exits 2 with only `tombstone` losses (6), `malformed: []`; the only warnings are `file-size` advisories on the two R2-exempt kept files (`go-release-pipeline.md`, `differential-harness.md`)
- [x] A-021 R6: a link-resolution pass over every `](/…/….md` target finds zero missing files in authored content — the single broken target (`/sync/multi-machine.md`) exists only in the generated `sync/index.md`/`sync/log.md` rows, the expected tombstone state the reorg regenerates

### Edge Cases & Error Handling

- [x] A-022 R3: fifteen `### Requirement:` blocks sampled across ten domains (fact, source×3, query, view, command×3, sync, config, watch, toolkit, render) each name a Go identifier that exists, and every numeric constant checked (60 s TTL, 120 s `DefaultTimeout`, 600/60/10 s brew bounds, 80×24, 5–3600 interval, p95 = 95.0 / 1.5×, 3-month floor, exit codes 0/1/2) matches the code
- [x] A-023 R21: `git diff --stat origin/main -- src harness justfile .github Formula scripts README.md fab/project package.json` is empty and `go test ./internal/toolkit/...` passes

### Code Quality

- [x] A-024 Pattern consistency: new files follow the frontmatter and heading shape of `$(fab kit-path)/templates/memory.md` and the existing Go-native files (`build/go-release-pipeline.md`)
- [x] A-025 No unnecessary duplication: no behavior is documented in two domains (each topic has one owning file; others link to it — cross-domain mentions verified as one-line pointers)
- [x] A-026 Readability: Overviews are 1–2 sentences; requirement blocks are focused; no God-file over the caps

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | The header-sentence rewrite in `usage.md` uses the exact wording in R20 | Intake fixed the content but not the words; the wording keeps every fact and adds none | S:60 R:95 A:85 D:70 |
| 2 | Confident | `standards-audit.md` lives in `toolkit/` (not `build/`) | The audit is about the toolkit contracts the binary answers; `build/` keeps only build/CI/release | S:55 R:90 A:80 D:65 |
| 3 | Confident | `ccusage-json-shapes.md` is its own file rather than a section of `ccusage-adapter.md` | The shape catalogue is the single largest verified-fact block in the old `cli/data-pipeline.md`; splitting keeps the adapter file under the cap | S:50 R:90 A:75 D:60 |
| 4 | Certain | The apply does not create `_shared/` or write removal records | Intake § 3 and the FKF carve-out assign that to `/docs-reorg-memory` | S:85 R:90 A:95 D:90 |

4 assumptions (1 certain, 3 confident, 0 tentative).
