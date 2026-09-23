# Intake: Memory Rehydrate (plan row X3)

**Change**: 260923-k7y1-memory-rehydrate
**Created**: 2026-09-23

## Origin

> Context: fab/plans/sahil/26-09-15-go-port.md, row X3. Read the Decisions and Target architecture sections before writing the intake. Do not change any external surface listed under Goal. Scope: rewrite the docs/memory files from the Go code as it exists now in src/go/ (not translated from the TS descriptions) — the shipped implementation is Go as of v0.12.1. Domains likely reshape to the package list in the plan's Target architecture section (fact, source, query, view, render, command, sync, config, watch, toolkit). Run /docs-reorg-memory after the rewrite to fix the domain/index shape. docs/site/skill.md and docs/specs need only the toolchain lines changed (Go build/test commands replacing the Node ones), not a full rewrite.

One-shot `/fab-new` invocation from the operator queue (Phase 4, "X2, X3, X4 unattended after X1"). No prior discussion in this conversation. Sources read to ground the intake: the plan's Goal, Decisions D1–D13, Target architecture table, row X3 and its neighbours X1/X2/Z1; constitution v2.0.0 (§ Go Conventions, § Test Runner/Location); `fab/project/context.md` (post-X2, Go stack + package table); every `docs/memory/**` file's frontmatter, Overview and size profile; `fab docs-index docs/memory --check --json` (exit 0, 16 advisory warnings); `docs/site/skill.md` in full; `docs/specs/usage.md` header and § Data Model; `src/go/` file inventory (80 non-test source files, 31 k lines incl. tests) and `internal/fact/fact.go`; the FKF standard (`$(fab kit-path)/reference/fkf.md` §2, §3, §6, §7) and the `/docs-reorg-memory` shape bounds.

**Plan status the row inherits.** X1 (`260923-jmh4-cutover-formula`) landed: `release.yml` pushes the Go formula; the cutover release is 0.13.0 (`package.json` reads 0.12.1 today — the shipped Homebrew formula still points at the last Node build until Sahil cuts 0.13.0, but the *repository's* shipped implementation is `src/go/`). X2 (`260923-uerc-constitution-v2`, PR #99) landed: constitution 2.0.0 governs `src/go/` directly; `src/node/` is frozen, outside the constitution, retained as harness oracle and rollback build until Z1. This change is docs-only: **no file under `src/`, `harness/`, `justfile`, `.github/`, `Formula/` or `scripts/` is touched, and no external surface listed under the plan's Goal changes.**

## Why

**The memory tree describes two implementations, one of which no longer ships.** Today `docs/memory/` has 16 topic files in 8 domains (2 467 lines). Five of them — `cli/data-pipeline.md` (56 KB), `display/formatting.md` (47 KB), `configuration/config-system.md`, `sync/multi-machine.md`, `watch-mode/tui.md` — are the *primary* memory for argument parsing, fetching, formatting, config, sync and watch, and they describe `src/node/` symbol by symbol (`parseGlobalFlags`, `fetchToolMergedWithMachines`, `renderTotal`, `writeMetrics`, `src/node/tui/formatter.ts`, `__tests__/`). Eight more live in `go-port/` and do describe the Go code, but as a transition: every Overview opens "The Go port's …", `fact-and-sources.md` still says "unshipped until cutover", `multi-mode.md` frames each path as "the TS path" and pins decisions to "the TS", `command-edge.md` carries 21 narration markers. `build/toolchain.md` (37 KB, 24 narration markers) leads with the esbuild build. Only `build/go-release-pipeline.md` and `harness/differential-harness.md` are already Go-native, and even the harness Overview calls Node "the shipped TypeScript binary".

**What happens if it stays.** Every later fab change loads memory through the always-load layer and the Affected Memory walk. A reviewer grounded in `cli/data-pipeline.md` will judge Go code against TS function names; a hydrate that touches "the CLI domain" will edit a file about code that is frozen and slated for deletion at Z1; the Z2 backlog re-triage has no Go-shaped memory to re-point rows at. The plan's own contract says "the memory files describe what the TS actually does and are the reconciliation source" — after cutover that sentence has to become true of the Go code, or the specs lose their reconciliation partner. The stale surfaces also leak into `fab/project/code-review.md` ("New data sources MUST produce `UsageEntry[]`") and the README's CI paragraph ("the unshipped Go successor") — both noted below as follow-ups, not in this row.

**Why a rewrite from source rather than a translation or a rename.** D1 says the Go internals are *redesigned*, not translated: there is no `fetchToolMerged` to rename, there is `command.gather` over one `query.GroupBy`; there are no print/render twins, there is `command.Result`; there is no `types.ts`, there is `fact.Record`. A find-and-replace would preserve TS's structure with Go names. Renaming `go-port/` to something else would keep the transition framing and leave the five legacy files in place, giving two descriptions of every behavior (the exact duplicate-coverage condition `/docs-reorg-memory` flags). The constitution's package list is the natural domain shape because it is the dependency graph: a reader who needs "how is the week label computed" goes to `query/`, "how does the never-shrink guard decide" goes to `sync/`. The row also fixes the mechanical debt the index check already reports (5 files over the ~15 KB soft cap, 4 over the narration threshold, most `description:` values over the 500-char soft cap) as a by-product of writing to the FKF contract from the start.

## What Changes

### 1. Target tree

One domain per `src/go/internal/` pipeline package named in the plan's Target architecture and constitution § Go Conventions, plus the two existing Go-native domains kept. The **starting file map** below is the plan's input; the apply may merge or split *within* a domain when the source warrants it, keeping every file at or under ~400 lines / ~15 KB and every `description:` at or under 500 characters.

| Domain | Files (new unless noted) | Source read | Replaces |
|--------|--------------------------|-------------|----------|
| `fact/` | `records.md` (Record, Totals, `Add`/`IsZero`, ISO labels, pinned JSON tags), `tool-registry.md` (six tools: key, aliases, display name, `ccusage` subcommand, column order, `Lookup`) | `internal/fact/{fact,tool}.go` | `go-port/fact-and-sources.md` § fact; `cli/data-pipeline.md` tool registry |
| `source/` | `errors-and-warnings.md` (`source.Error`, `WriteWarnings`, `PeriodDaily`, `DefaultTimeout`, the edge-only stderr rule), `ccusage-adapter.md` (vendor-first binary resolution relative to `os.Executable()`, exec with context deadline, invocations map, normalize, `Fetch`/`FetchAll`, User/Machine stamping), `ccusage-json-shapes.md` (the ccusage v20 per-agent daily JSON shapes incl. the codex `costUSD`/`models{}` outlier and the empty-agent `daily: []` / `-0.0` form), `metrics-reader.md` (day-file layout `{user}/{year}/{machine}/{tool}-{date}.jsonl`, `DayFile`/`Name`/`Path` encoding shared with the writer, user enumeration), `cache.md` (on-disk JSON envelope, hash key over tool+period+args, 60 s TTL, `--fresh`) | `internal/source/**` | `go-port/fact-and-sources.md` § source; `cli/data-pipeline.md` fetch/cache/JSON-shape sections |
| `query/` | `periods-and-windows.md` (`Period`, labels, Sunday-anchored weeks, `ThreeMonthFloor`, `Relabel`, `since`/`until` window filter), `aggregation.md` (`RollUp`, the one `GroupBy`, `Collapse`, `MaxMerge` and the float-summation order) | `internal/query/*.go` | `go-port/query-view-render.md` § query; `cli/data-pipeline.md` aggregation |
| `view/` | `table-model.md` (`Table`/`CompactTable`/cells, metric cost-vs-tokens, data-sized columns, dimmed exact zeros), `snapshot.md`, `history.md` (single-tool history and the all-tools pivot, month separators, weekend dimming, negligible-column omission, stacked tool bars, `MaxRows` watch budget), `leaderboard.md` (ranking, share, Δ vs previous window, `--top` collapse/fold, `◂` pin), `bars-and-deltas.md` (eighths glyphs, p95 bar scale, cell-level and trailing deltas), `breakdown.md` (machine/user columns, legend, footer stats) | `internal/view/*.go` + `testdata/` | `display/formatting.md`; `go-port/query-view-render.md` § view |
| `render/` | `number-formatting.md` (`FormatInt`/`FormatCost`, `FixedHalfUp`/`JSRound`), `ansi.md` (`Colors`, `NO_COLOR`, palette, table and compact encoders), `json.md` (pinned snapshot/history/leaderboard shapes as Go structs), `csv-and-markdown.md` | `internal/render/**` + goldens | `display/formatting.md` emit sections; `go-port/query-view-render.md` § render |
| `command/` | `request-and-parse.md` (grammar, `Flags`, `UsageError`, `ShortUsage`/`FullHelp`, exit-code table), `guards.md` (`Normalize`: since/until on snapshot, `-u` in single mode, `--top` off-leaderboard, `--by-machine` on the pivot, reserved user `all`, the 3-month cap), `run-and-result.md` (`Run`, `Deps` with the `Fetcher`/`Repo`/`Writer` seams, `Result`, `Live` per-poll options), `multi-mode.md` (`gather`'s record paths, own-user write-then-`MaxMerge`, repo-only `-u` paths, `gatherAllUsers`, mode-keyed snapshot label rule), `entry-point.md` (`cmd/tu/main.go`: dispatch order, `MetricsDirGuard` placement, the only writer of `os.Stdout`/`os.Stderr`, `sync`/`--sync` and `-w` branches, exit codes) | `internal/command/*.go`, `cmd/tu/main.go` | `go-port/command-edge.md`, `go-port/multi-mode.md`; `cli/data-pipeline.md` parsing/dispatch |
| `sync/` (folder kept) | `day-file-writer.md` (`Write`, never-shrink guard and its `Number()` coercion table, `WriteDecision`), `git-flow.md` (`Exec` driver, `SyncMetrics` add/commit/pull/push, rebase-abort recovery, `CommitMessage`, `TouchLastSync`/`Stale`, auto-sync TTL), `dry-run-report.md` (`FullSync` live vs dry-run on one decision path, `Report.Format` byte rules), `repair.md` (`sync.Repair`, `cmd/turepair`, the ASCII ICU-root comparator in `localecmp.go`) | `internal/sync/*.go` | `sync/multi-machine.md` (removed), `go-port/metrics-sync.md` |
| `config/` | `cascade.md` (`ResolvePaths`, five-layer `Load` defaults ∪ org ∪ user ∪ env ∪ CLI, legacy `~/.tu.conf` warning, embedded drift-guarded `tu.default.conf`, sentinel expansion, `StateDir`/`Tildefy`), `setup-commands.md` (`InitConf`, `InitMetrics` and the `CloneStep` handoff), `status.md` (`Status`/`Lines`/`RelativeTime`/`LastSync`), `metrics-dir-guard.md` (`MetricsDirGuard`, `.clone-failed` marker, `Cloner`) | `internal/config/*.go` | `configuration/config-system.md`, `go-port/config-and-setup.md` |
| `watch/` | `loop-and-terminal.md` (`Run`'s single select over keys/SIGWINCH/SIGINT/poll/countdown/rain, re-entrancy guard, cleanup returning last lines; `Terminal` seam, 80×24 fallback, TTY-gated raw mode keeping `OPOST|ONLCR`, the darwin/linux termios files), `compositor.md` (`Lay`/`Frame`, skeleton, compact threshold, golden frames under a fake clock), `panel-and-rain.md` (`StatsGrid`/`BurnRate`, session stats, `RainState` on injected `rand/v2`, `--no-rain`) | `internal/watch/*.go` + goldens | `watch-mode/tui.md`, `go-port/watch-mode.md` |
| `toolkit/` | `version-and-help-dump.md` (`BareVersion`/`DisplayVersion`/`VersionLine`, `BuildHelpDoc`/`Encode` without HTML escaping, the shll.ai pull that consumes it), `update.md` (`Brew` seam, 600 s/60 s SIGTERM-graceful bounds, unbounded `HOMEBREW_NO_ASK=1` upgrade, `/Cellar/tu/` gate, `--skip-brew-update`), `shell-init-and-completions.md` (embedded `completions/tu.{bash,zsh,fish}`), `skill-bundle.md` (committed copy + `scripts/sync-skill.sh` + `go test` drift guard), `standards-audit.md` (the shll v0.1.32 audit record, findings S1/V1, toolkit-standards posture) | `internal/toolkit/**` | `go-port/toolkit-layer.md`; the help-dump/skill/standards sections of `build/toolchain.md` |
| `build/` (kept) | `toolchain.md` **(rewritten Go-first)**: module and `go 1.26.0`, `just go-build`/`go-build-all`/`go-lint`/`go-test`/`harness-*`, gofmt+vet gate, golden `-update` convention, CI lanes `go-build-and-test` + `tudiff` + `ci-gate` ruleset, `scripts/release.sh`; one closing section **Retired `src/node/` tree** (what it is, why it stays — harness oracle and D10 rollback — its `npm ci && npm run build && npm test` toolchain in three lines, removal at Z1). `go-release-pipeline.md` **(framing pass only)**: drop pre-cutover hedges, keep content | `justfile`, `.github/workflows/*.yml`, `scripts/` (read-only) | `build/toolchain.md` (in place) |
| `harness/` (kept) | `differential-harness.md` **(framing pass only)**: Overview and any sentence calling Node "shipped" — `node dist/tu.mjs` is the frozen oracle, `bin/tu` the shipped binary; content otherwise unchanged | `cmd/tudiff`, `internal/harness` (read-only) | in place |

Removed whole (topic files, `index.md`, `log.md`, `log.seed.md`): `docs/memory/cli/`, `docs/memory/configuration/`, `docs/memory/display/`, `docs/memory/watch-mode/`, `docs/memory/go-port/`. Removed file inside a kept folder: `docs/memory/sync/multi-machine.md`. Expected result: 12 domains, roughly 35 topic files, no file over the soft caps.

### 2. Authoring contract for every rewritten or new file

- **Source of truth is the Go tree as of `origin/main` at apply time**: `src/go/internal/**/*.go`, `src/go/cmd/tu/main.go`, the `_test.go` siblings and `testdata/*.golden`. `docs/specs/usage.md` and `layouts.md` supply the external-contract *wording* (exit codes, pinned JSON keys, layouts) and are cited, not copied. The old memory files may be read for two purposes only: to harvest **Design Decisions** whose rationale still holds (see next bullet) and to build a checklist of behaviors to *verify exist* in Go. **No sentence that names a TS symbol, a `.ts` path, `src/node/`, `__tests__/`, "the Go port", "unshipped", "until cutover", or "the TS does/pinned to the TS" is carried into a new body.** Sanctioned exceptions: `build/toolchain.md`'s retired-tree section; `harness/differential-harness.md` (the oracle is `node dist/tu.mjs` by definition); and a Design Decision whose **Why** is byte or wire parity, which may say "parity with the frozen `src/node/` oracle (until plan row Z1)" — the reason is the mixed fleet and the harness gate, and that reason is present truth.
- **Reuse rule for `go-port/*`**: those eight files are Go-derived and may be reused at sentence level *after* each reused claim is re-checked against the current source (rows B3–B7 and the G1 rework changed code the earlier rows documented) and the port framing is stripped. Anything not re-verified is rewritten.
- **FKF (v0.1) shape**: leading frontmatter `type: memory` + one-line, change-id-free `description:` ≤ 500 characters (a routing signal — the current 600–900-character descriptions are the anti-pattern); body `# {Name}`, `**Domain**: {domain}`, `## Overview` (1–2 sentences), `## Requirements` with `### Requirement:` blocks in RFC 2119 voice and `#### Scenario:` GIVEN/WHEN/THEN where the behavior has an observable edge, `## Design Decisions` in the four-field shape (**Decision** / **Why** / **Rejected** / *Introduced by*). No `## Changelog`, no headings carrying change ids, no operational TODOs, present tense only.
- **Every requirement is traceable to code**: each `### Requirement:` names at least one Go identifier or file (`command.Normalize`, `internal/sync/writer.go`) so the reviewer can spot-check claims against source. Numbers (TTL 60 s, 600 s/60 s brew bounds, 80×24 fallback, 10 MB buffer, the 3-month floor) are taken from constants in the code, not from the old prose.
- **Citations**: a trailing `(id)` names the change that *made the decision*, which may predate the port — e.g. the never-shrink guard `(srmi)`, weekly periods `(wkly)`, leaderboard semantics `(4xwg)`, config-home cascade `(gzrn)` — with the Go row cited where the port made its own choice (`(v0as)`, `(3am6)`, `(4fs0)`, `(9ax5)`, `(xivf)`, `(pmsd)`, `(2gbb)`, `(lsml)`, `(4pze)`, `(vcur)`, `(0118)`, `(489t)`, `(jmh4)`, `(uerc)`, `(m9of)` for the G1 rework). Citations are the only provenance a body carries.
- **Cross-links** are bundle-relative (`](/query/aggregation.md)`), resolved from `docs/memory/`. All 216 existing bundle-relative links point at files this change removes or rewrites; every link in the new tree must resolve (the index check reports broken links as advisories — target is zero).
- **Domain index stubs**: each new domain folder gets an `index.md` holding only the `description:` frontmatter one-liner (the domain's routing text) *before* any `fab docs-index docs/memory` run; kept folders keep theirs. The root `index.md` is generated; its manual block and `nav_note` are untouched.
- **Seeds and logs in kept folders**: `sync/log.seed.md` and `build/log.seed.md` are human seed inputs — rewrite their `](/sync/multi-machine.md)` links to the successor file (`/sync/day-file-writer.md`) so the regenerated `log.md` does not carry dangling links. Seeds of removed folders are deleted with the folder (their history remains in git, `fab/changes/**`, and the removal record the reorg writes).

### 3. Index regeneration and `/docs-reorg-memory` (the ordering that makes the deletions safe)

Deleting five domain folders leaves five rows in the committed root `index.md` (and the domain rows for `multi-machine.md`) whose link targets are absent — exactly the FKF §6.4 **tombstone** condition. `fab docs-index docs/memory --check` will exit **2** and every regeneration guard refuses to run until the rows are relocated. That relocation is `/docs-reorg-memory`'s sanctioned job (it authors `docs/memory/_shared/removed-domains.md`, the one body allowed to hold removal records). Therefore:

1. **Apply** writes the new tree, deletes the retired folders/files, creates the domain `index.md` stubs, rewrites seed links, and runs `fab docs-index docs/memory --check --json` expecting **exit 2 with only `tombstone` losses** (no `description` or `grouping` losses — the manual block is untouched). It does **not** force a regeneration and does not hand-edit generated rows. Its result reports the loss list verbatim.
2. **Review** verifies content (§ 5) against the source tree; it does not require a clean index.
3. **`/docs-reorg-memory` runs in the main session after review passes and before hydrate** — a deliberate manual stop in an otherwise unattended pipeline. Its approved plan: relocate the tombstone rows to `_shared/removed-domains.md` (one entry per removed domain naming its successor domains), apply any within-bounds merge it proposes (e.g. folding a one-file domain into its neighbour — `fact/` with two files is the likely candidate; the operator decides), rewrite links, and run `fab docs-index docs/memory` once (it also rebuilds each folder's `log.md`, merging the seeds).
4. **Hydrate** is then a verification pass: `fab docs-index docs/memory --check` exits 0 with zero `file-size`, `narration-density`, over-length-description and broken-link warnings on the files this change wrote (pre-existing warnings on files it did not touch are acceptable only if none remain — the list is expected to be empty since every file is rewritten or framing-passed). Hydrate writes no new memory of its own beyond that check — this change's *content* is the memory.

### 4. `docs/site/skill.md` and `docs/specs/`

- **`docs/site/skill.md` — no change.** It contains no toolchain line (the only shell-out it names is `brew`); it already describes the vendored `ccusage` and is runtime-neutral. Because it is unchanged, the committed copy `src/go/internal/toolkit/skill.md`, its `go test` drift guard and `scripts/sync-skill.sh` are not touched.
- **`docs/specs/layouts.md` — no change** (no toolchain or language reference).
- **`docs/specs/usage.md` — two edits, nothing else:**
  1. § Data Model's ` ```typescript ` fence (the `UsageTotals` / `UsageEntry` interfaces) becomes a ` ```go ` fence showing the pinned types verbatim from `src/go/internal/fact/fact.go`:
     ```go
     type Totals struct {
         TotalCost           float64 `json:"totalCost"`
         InputTokens         int64   `json:"inputTokens"`
         OutputTokens        int64   `json:"outputTokens"`
         CacheCreationTokens int64   `json:"cacheCreationTokens"`
         CacheReadTokens     int64   `json:"cacheReadTokens"`
         TotalTokens         int64   `json:"totalTokens"`
     }

     type Record struct {
         Date    string // ISO label: "YYYY-MM-DD" or "YYYY-MM"
         Tool    string // registry key: cc, codex, oc, gemini, copilot, kimi
         User    string
         Machine string
         Totals
     }
     ```
     The sentence after it ("`totalTokens` = input + output + cache creation + cache read …") stays; the JSON key names are unchanged, so no pinned output shape moves.
  2. The header blockquote's provenance sentence — "every observable behavior of the shipped binary (v0.11.5, reconciled 2026-09-16 against the 18 memory files and the installed binary …)" — is updated to say the shipped binary is the Go build from `src/go/` (v0.12.1 repository state; cutover release 0.13.0), that the 2026-09-16 reconciliation was performed against the v0.11.5 Node binary and the then-current memory tree, and that internal mechanisms live in `docs/memory/` (no count). The `(DC-NN)` ledger and all 59 marker references are untouched — they are G0's, not this row's.

  There are no Node build/test command lines in either spec, so "toolchain lines" reduces to these two edits.

### 5. Verification the reviewer runs

- `fab docs-index docs/memory --check --json` at review: exit 2, `losses[]` contains only `tombstone` entries, `malformed: []`, and no `file-size` / `narration-density` / description-length warning names a file under the 12 target domains.
- `grep -rlE 'src/node|\.ts\b|__tests__|the Go port|unshipped|until cutover' docs/memory` lists at most `build/toolchain.md` and `harness/differential-harness.md`.
- Every `description:` ≤ 500 characters, single line, no registered change id: `awk`/`fab docs-index` advisory count for descriptions is zero.
- Every bundle-relative link resolves: a script over `](/…/….md)` targets finds no missing file.
- Spot-check: for ten `### Requirement:` blocks chosen across five domains, the named Go identifier exists and behaves as stated (`go doc` / reading the function), including at least one numeric constant per domain.
- `git diff --stat origin/main -- src/ harness/ justfile .github/ Formula/ scripts/ README.md fab/project/` is empty; `go test ./internal/toolkit/...` stays green (skill.md untouched).
- `docs/specs/usage.md`: exactly the two hunks in § 4; `docs/specs/layouts.md` and `docs/site/skill.md` untouched.

## Affected Memory

Removed (retired TS-described files and the transition domain):

- `cli/data-pipeline`: (remove) content redistributed to `fact/tool-registry`, `source/*`, `query/*`, `command/*`
- `configuration/config-system`: (remove) → `config/*`
- `display/formatting`: (remove) → `view/*`, `render/*`
- `sync/multi-machine`: (remove) → `sync/day-file-writer`, `sync/git-flow`, `sync/dry-run-report`, `sync/repair`, `source/metrics-reader`
- `watch-mode/tui`: (remove) → `watch/*`
- `go-port/command-edge`: (remove) → `command/request-and-parse`, `command/guards`, `command/run-and-result`, `command/entry-point`
- `go-port/config-and-setup`: (remove) → `config/*`
- `go-port/fact-and-sources`: (remove) → `fact/*`, `source/*`
- `go-port/metrics-sync`: (remove) → `sync/*`
- `go-port/multi-mode`: (remove) → `command/multi-mode`
- `go-port/query-view-render`: (remove) → `query/*`, `view/*`, `render/*`
- `go-port/toolkit-layer`: (remove) → `toolkit/*`
- `go-port/watch-mode`: (remove) → `watch/*`

New (written from `src/go/`):

- `fact/records`: (new) `Record`/`Totals`, ISO labels, pinned JSON tags
- `fact/tool-registry`: (new) six-tool registry, keys/aliases/display names/order
- `source/errors-and-warnings`: (new) typed `Error`, `WriteWarnings`, edge-only stderr
- `source/ccusage-adapter`: (new) vendor-first exec, deadline, normalize, `Fetch`/`FetchAll`
- `source/ccusage-json-shapes`: (new) ccusage v20 per-agent JSON shapes incl. codex outlier and empty form
- `source/metrics-reader`: (new) day-file layout and the shared `DayFile` encoding, user enumeration
- `source/cache`: (new) hash-keyed JSON cache, 60 s TTL, `--fresh`
- `query/periods-and-windows`: (new) periods, labels, Sunday weeks, `ThreeMonthFloor`, window filter
- `query/aggregation`: (new) `RollUp`, `GroupBy`, `Collapse`, `MaxMerge`, summation order
- `view/table-model`: (new) table/compact models, metric unit, sizing, dimmed zeros
- `view/snapshot`: (new) snapshot table model
- `view/history`: (new) history and pivot models, separators, dimming, stacked bars, `MaxRows`
- `view/leaderboard`: (new) ranking, share, Δ, `--top`, pin marker
- `view/bars-and-deltas`: (new) eighths bars, p95 scale, delta placements
- `view/breakdown`: (new) machine/user columns, legend, footer
- `render/number-formatting`: (new) `FormatInt`/`FormatCost`, rounding twins
- `render/ansi`: (new) colors, `NO_COLOR`, table/compact encoders
- `render/json`: (new) pinned JSON shapes as Go structs
- `render/csv-and-markdown`: (new) CSV and Markdown encoders
- `command/request-and-parse`: (new) grammar, flags, usage errors, help text, exit codes
- `command/guards`: (new) `Normalize` guards and the 3-month cap
- `command/run-and-result`: (new) `Run`, `Deps` seams, `Result`, `Live`
- `command/multi-mode`: (new) `gather` paths, write-then-max-merge, repo-only paths, label rule
- `command/entry-point`: (new) `cmd/tu/main.go` dispatch, only-writer rule, exit codes
- `sync/day-file-writer`: (new) `Write`, never-shrink guard, `Number()` coercion, `WriteDecision`
- `sync/git-flow`: (new) git driver, sync round trip, rebase-abort recovery, `.last-sync`
- `sync/dry-run-report`: (new) `FullSync` live/dry-run, `Report.Format`
- `sync/repair`: (new) `sync.Repair`, `cmd/turepair`, locale comparator
- `config/cascade`: (new) paths, five-layer `Load`, legacy warning, embedded defaults
- `config/setup-commands`: (new) `InitConf`, `InitMetrics`, `CloneStep`
- `config/status`: (new) `Status`/`Lines`/`RelativeTime`/`LastSync`
- `config/metrics-dir-guard`: (new) auto-clone guard, `.clone-failed`, `Cloner`
- `watch/loop-and-terminal`: (new) select loop, keys, signals, raw mode, `Terminal` seam
- `watch/compositor`: (new) `Lay`/`Frame`, skeleton, compact threshold, golden frames
- `watch/panel-and-rain`: (new) stats grid, burn rate, rain state, `--no-rain`
- `toolkit/version-and-help-dump`: (new) version helpers, help-dump envelope, shll.ai pull
- `toolkit/update`: (new) `Brew` seam, bounds, `/Cellar/tu/` gate, `--skip-brew-update`
- `toolkit/shell-init-and-completions`: (new) embedded completion scripts
- `toolkit/skill-bundle`: (new) committed copy, sync script, drift guard
- `toolkit/standards-audit`: (new) shll standards audit record and posture

Modified in place:

- `build/toolchain`: (modify) rewritten Go-first; retired `src/node/` tree as one closing section; help-dump/skill/standards material moved to `toolkit/`
- `build/go-release-pipeline`: (modify) framing pass — post-cutover wording, content unchanged
- `harness/differential-harness`: (modify) framing pass — Node is the frozen oracle, Go the shipped binary

Generated (by `/docs-reorg-memory`'s `fab docs-index` run, not hand-edited): `docs/memory/index.md`, every domain `index.md` body, every `log.md`; plus `_shared/removed-domains.md` (new, reorg-authored tombstone ledger).

## Impact

- **Files**: ~35 new memory files, 13 removed, 3 modified, 5 folders removed, 10 new domain `index.md` stubs, 2 seed files link-edited; `docs/specs/usage.md` (2 hunks). Net memory size is expected to fall from ~350 KB to well under 250 KB because narration and duplicate coverage are dropped, while file count roughly doubles to respect the ~15 KB cap.
- **Code, tests, CI, release, formula, README**: untouched. No external surface under the plan's Goal changes. The differential harness and `go test` are unaffected; `go test ./internal/toolkit` remains the only test with a docs dependency (`skill.md`, unchanged).
- **fab pipeline behaviour after merge**: every later change's Affected Memory walk resolves against package-named domains; reviewers and hydrate read Go-shaped requirements; Z2's backlog re-triage has concrete Go targets. The always-load layer's `docs/memory/index.md` grows from 8 to ~12 domain rows.
- **Pipeline shape for this change**: one apply task per target domain (12) plus tasks for removals, seeds/stubs, the spec edits and the verification script — the `/fab-fff` lane fork will pick the full lane. The apply worker's dispatch is large (reads ~31 k lines of Go plus goldens); the plan should order domains by dependency (`fact` → `source` → `query` → `view` → `render` → `command` → `sync`/`config` → `watch` → `toolkit` → `build`/`harness`) so cross-links target files that already exist. **Manual stop**: `/docs-reorg-memory` between review and hydrate (§ 3).
- **Adjacent stale text, out of this row (follow-ups, not tasks)**: README "CI / branch protection" paragraph still calls Node "the shipped Node implementation" and Go "the unshipped Go successor" and lists the Node lane first; `fab/project/code-review.md` § Project-Specific Review Rules still says "New data sources MUST produce `UsageEntry[]`". Both are one-paragraph fixes for a docs follow-up (Policy B keeps README install prose pointing at hexokit.com; these are developer lines, not install prose).

## Open Questions

- None blocking. The one judgement left to the operator at the `/docs-reorg-memory` stop is whether to keep `fact/` as a two-file domain (matches the plan's package list and the constitution's dependency graph) or fold it into `source/` (respects the reorg's ~5-file soft floor). The intake's default is to keep it; the reorg proposal is the place to overrule.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Source of truth is `src/go/` code, tests and goldens at apply time; specs supply contract wording; old memory is read only to harvest Design Decisions and to build a verify-exists checklist | Stated verbatim in the row ("from the Go code as it exists now … not translated from the TS descriptions") and D1 | S:90 R:70 A:90 D:90 |
| 2 | Confident | Target domains are one per `internal/` pipeline package (`fact`, `source`, `query`, `view`, `render`, `command`, `sync`, `config`, `watch`, `toolkit`) plus `build` and `harness` kept; `cli`, `configuration`, `display`, `watch-mode`, `go-port` removed; the `sync` folder is reused | The row and the plan name the package list; constitution § Go Conventions fixes the same ten names; reorg can still merge thin folders afterwards | S:80 R:75 A:80 D:70 |
| 3 | Confident | Starting file map of ~35 files (§ 1), each ≤ ~400 lines / ~15 KB with a ≤ 500-char description; the apply may merge or split within a domain when the source warrants | Derived from the FKF soft caps and the source sizes; the exact partition is reversible at the reorg stop | S:60 R:80 A:75 D:60 |
| 4 | Confident | Retired folders are deleted whole (topic files, `index.md`, `log.md`, `log.seed.md`); tombstone rows are relocated to `_shared/removed-domains.md` by `/docs-reorg-memory`, not by the apply worker; seeds in kept folders get their links rewritten | FKF §3.3/§6.4 name the reorg skill as the sole author of the removal ledger; history survives in git and `fab/changes/`; the alternative (migrating seed entries into successor folders) invents provenance the generator would attribute to the wrong folder | S:50 R:75 A:65 D:50 |
| 5 | Confident | `/docs-reorg-memory` runs in the main session between review and hydrate as a manual stop; apply never forces index regeneration past an exit-2 check | The row says "run /docs-reorg-memory after the rewrite"; the exit-2 refuse-before-regen guard makes any other order fail | S:70 R:80 A:70 D:60 |
| 6 | Certain | FKF v0.1 authoring rules bind every new body: present truth, no TS symbols or transition phrasing, four-field Design Decisions, citation-only provenance, no Changelog | `$(fab kit-path)/reference/fkf.md` §3 is normative and `fab docs-index --check` enforces the blocking parts | S:85 R:85 A:95 D:90 |
| 7 | Confident | A citation names the change that made the decision even when it predates the port (`srmi`, `wkly`, `4xwg`, `gzrn` …); Go rows are cited for port-specific choices | FKF §3.3: a citation marks where a current fact came from; the never-shrink rule was decided in `srmi`, only implemented again in `lsml` | S:55 R:90 A:75 D:65 |
| 8 | Confident | Design Decisions whose Why is byte/wire parity may name "the frozen `src/node/` oracle (until plan row Z1)"; every other TS comparison is dropped | The mixed fleet (D11) and the `tudiff` gate are present-tense reasons; the comparison itself is transition narration | S:60 R:85 A:80 D:65 |
| 9 | Certain | `docs/site/skill.md` is not changed, so `src/go/internal/toolkit/skill.md`, its drift guard and `scripts/sync-skill.sh` are not touched | Read in full: it has no build/test line; the only shell-out it names is `brew` | S:95 R:95 A:95 D:95 |
| 10 | Confident | `docs/specs/usage.md` gets exactly two edits (Go fence for `Totals`/`Record`; header provenance sentence); `layouts.md` untouched | Neither spec contains a Node command line; the TypeScript fence is the one language leak in the language-neutral contract | S:60 R:90 A:85 D:70 |
| 11 | Confident | README's CI paragraph and `fab/project/code-review.md`'s `UsageEntry[]` rule are left as-is and reported as follow-ups | Outside the row's named files; the plan's Policy B and X1 kept README out of Phase 4 rows | S:55 R:95 A:70 D:60 |
| 12 | Certain | `change_type` is `docs`, pinned explicitly with `fab status set-change-type` | The description contains "fix the domain/index shape", which the keyword inference would read as `fix`; the change touches only `docs/` | S:90 R:95 A:95 D:95 |
| 13 | Confident | `build/toolchain.md` is rewritten Go-first with one retired-tree section; `go-release-pipeline.md` and `harness/differential-harness.md` get a framing pass only | X2 already made toolchain.md's *content* Go-aware but left its Node-first structure and 24 narration markers; the other two are Go-native | S:65 R:85 A:80 D:70 |
| 14 | Confident | `go-port/*` content may be reused sentence-level only after re-verification against current source; the apply worker owns the rewrite directly rather than invoking `/docs-hydrate-memory generate` inside the dispatch | Those files are Go-derived, so reuse does not violate "not translated from TS"; rows after their hydrate changed the code they describe | S:55 R:80 A:70 D:50 |

14 assumptions (4 certain, 10 confident, 0 tentative, 0 unresolved).
