# Tasks: Safer process spawning, CSV/Markdown export, and shell completions

**Change**: 260423-lx0g-exec-csv-completions
**Spec**: `spec.md`
**Intake**: `intake.md`

## Phase 1: Setup

- [x] T001 Update `ToolConfig` interface in `src/node/core/types.ts` — add `binary: string`, `prefixArgs: string[]`; remove `command: string`. Keep `name` and `needsFilter`.
- [x] T002 [P] Create `src/node/core/completions.ts` with three exported string constants: `BASH_COMPLETION`, `ZSH_COMPLETION`, `FISH_COMPLETION`. Each script covers the full grammar per spec Requirement "Completion Script Coverage" (non-data subcommands, sources, periods, display tokens, all long/short flags, `completions` args). Scripts MUST be statically generated.

## Phase 2: Core Implementation

- [x] T003 [P] Refactor `src/node/core/fetcher.ts` spawning layer: (a) rename/rewrite `execAsync(cmd, toolName)` to `execFileAsync(file, args, toolName)` using `node:child_process.execFile` instead of `exec`; (b) rebuild the `TOOLS` registry entries with the new shape (`binary`, `prefixArgs`) for both vendor and non-vendor paths; (c) update `runTool` to construct argv `[...tool.prefixArgs, period, "--json", ...extraArgs]` and pass to `execFileAsync`. Preserve `maxBuffer: 10 * 1024 * 1024` and the warn-on-error + empty-string-resolve behavior.
- [x] T004 [P] Refactor `src/node/sync/sync.ts` spawning layer: replace `execAsync(cmd: string)` with a wrapper accepting `(file: string, args: string[])` and invoking `node:child_process.execFile`. Update every call site in `syncMetrics` (add, status --porcelain, commit, pull --rebase, push, rebase --abort) to pass argv arrays. Preserve interrupted-rebase recovery and push retry semantics with their existing stderr warnings.
- [x] T005 [P] Add output-format parsing in `src/node/core/cli.ts`: extend `GlobalFlags` with `outputFormat: "table" | "json" | "csv" | "md"`; recognise `--csv` and `--md` in `parseGlobalFlags` (and add them to the filter-skip list); implement conflict detection that rejects (a) any two of `--json`/`--csv`/`--md` together, (b) any of `--json`/`--csv`/`--md` combined with `--watch`/`-w`. Error messages follow `Error: {flag-a} and {flag-b} are incompatible`. Retain backward-compatible `jsonFlag` boolean deriving from `outputFormat === "json"` if needed by downstream callers during the transition.
- [x] T006 [P] Implement `emitCsv(data, kind)` in `src/node/tui/formatter.ts`. Three `kind` values: `"snapshot"` (Map<string, UsageTotals>), `"history"` (UsageEntry[] with toolName), `"total-history"` (Map<string, UsageEntry[]>). RFC 4180 quoting, LF line endings, no BOM, raw numeric values (no thousands separators), cost two-decimal without `$`. Machine columns: append `machine_{name}_cost` columns sorted alphabetically when machine data present. Write to stdout via `process.stdout.write` or `console.log`.
- [x] T007 [P] Implement `emitMarkdown(data, kind)` in `src/node/tui/formatter.ts`. Same three `kind` values. Leading `## {title}` heading matching ANSI renderer titles (`Combined Usage (daily)`, `Claude Code (monthly)`, `Combined Cost History (daily)`). GFM tables: left-aligned string columns (`:---`), right-aligned numeric columns (`---:`). Numeric values use comma thousands separators; cost with `$` prefix. Total row bolded (`**Total**`, `**value**`) when `visibleCount > 1`. Machine columns use machine names directly (no letter codes or legend). Trailing blank line.
- [x] T008 Plumb `outputFormat` through dispatch in `src/node/core/cli.ts`. Update `dispatchAllHistory`, `dispatchAllSnapshot`, `dispatchSingleTool` to accept `outputFormat` and switch on it: `json` → `emitJson`, `csv` → `emitCsv(data, kind)`, `md` → `emitMarkdown(data, kind)`, `table` → existing `print*` functions. Fetch runs once per dispatch regardless of format. Update `main()` to pass `outputFormat` from parsed flags.
- [x] T009 Add `completions` non-data subcommand dispatch in `src/node/core/cli.ts` `main()` alongside `init-conf`, `init-metrics`, `sync`, `status`, `update`. Implement `runCompletions(shell?: string)`: no arg → print usage block with install examples for all three shells; `bash`/`zsh`/`fish` → `process.stdout.write(SCRIPT)` + exit 0; anything else → stderr `Unknown shell: {shell}. Supported: bash, zsh, fish` + exit 1. Import the three script constants from `src/node/core/completions.ts`.
- [x] T010 Update `FULL_HELP` in `src/node/core/cli.ts` to document the new `--csv` and `--md` flags alongside `--json`, and the new `completions` subcommand under the Setup section. Keep `SHORT_USAGE` unchanged.

## Phase 3: Integration & Edge Cases

- [x] T011 [P] Add tests in `src/node/core/__tests__/fetcher.test.ts` (extend existing file) covering: (a) `TOOLS` registry shape — each entry has `binary`, `prefixArgs`, `needsFilter`; vendor path uses `"node"` binary with `.../index.js` in prefixArgs; non-vendor path uses direct binary with empty prefixArgs; (b) argv construction in `runTool` — verify the array passed to `execFile` matches `[...prefixArgs, period, "--json", ...extraArgs]` (use a test double or spy for `execFile`).
- [x] T012 [P] Add tests in `src/node/sync/__tests__/sync.test.ts` (extend existing file) covering: (a) git commands are invoked via `execFile("git", [...])` not `exec("git ...")`; (b) metricsDir with spaces does not break sync (verify argv entry is literal); (c) interrupted-rebase recovery still emits `rebase --abort`; (d) push retry still attempts twice before warning+return false.
- [x] T013 [P] Add tests in `src/node/tui/__tests__/formatter.test.ts` (extend existing file) for `emitCsv`: (a) snapshot kind header/rows/total; (b) history kind header/rows; (c) total-history kind header/rows with per-tool columns; (d) machine columns appear with `--by-machine` data sorted alphabetically as `machine_{name}_cost`; (e) RFC 4180 quoting — fields with commas, quotes, or newlines; (f) LF line endings, no BOM; (g) cost formatting (two decimals, no `$`).
- [x] T014 [P] Add tests in `src/node/tui/__tests__/formatter.test.ts` for `emitMarkdown`: (a) leading `## {title}` matches ANSI renderer title for each kind; (b) GFM alignment separators (`:---` for strings, `---:` for numerics); (c) total row bolded when `visibleCount > 1`, omitted when 1; (d) numeric values have commas; (e) cost values have `$` prefix; (f) machine columns use machine names directly (no A/B/C letter codes, no legend line); (g) trailing blank line present.
- [x] T015 [P] Add tests in `src/node/core/__tests__/cli-parser.test.ts` (extend existing file — it already covers `parseGlobalFlags` behaviour) for flag-conflict rejection: `--json --csv`, `--csv --md`, `--json --md`, `--csv --watch`, `--md --watch`, `--csv -w`, `--md -w`. Each should produce the "incompatible" stderr message and exit 1. The existing `--json --watch` error (covered in `cli-watch-flag.test.ts`) MUST still fire with its current wording.
- [x] T016 [P] Add tests in a new `src/node/core/__tests__/completions.test.ts` for `runCompletions`: (a) `"bash"` writes a script containing `complete` builtin reference, exits 0; (b) `"zsh"` writes script containing `#compdef tu`, exits 0; (c) `"fish"` writes script containing `complete -c tu`, exits 0; (d) no-arg prints usage with install examples for all three shells, exits 0; (e) unknown shell writes `Unknown shell: powershell. Supported: bash, zsh, fish` to stderr, exits 1; (f) each of the three scripts contains literal occurrences of every long flag in the taxonomy (`--json`, `--csv`, `--md`, `--sync`, `--fresh`, `--watch`, `--interval`, `--user`, `--by-machine`, `--no-color`, `--no-rain`, `--version`, `--help`) and every non-data subcommand (`help`, `init-conf`, `init-metrics`, `sync`, `status`, `update`, `completions`).

## Phase 4: Polish

- [x] T017 Run full test suite via `just test` (which runs `npx tsx --test 'src/node/**/__tests__/*.test.ts'`) and confirm all tests pass. Run the esbuild bundle build via `just build` (which executes `scripts/build.sh`) and confirm `dist/tu.mjs` regenerates without errors.
- [x] T018 Manual smoke tests against the freshly-built bundle: (a) `./dist/tu.mjs --csv` emits valid CSV with header row; (b) `./dist/tu.mjs m --md` emits `## Combined Usage (monthly)` followed by a GFM table; (c) `./dist/tu.mjs completions bash | bash -n` validates the bash script parses cleanly; (d) `./dist/tu.mjs completions zsh` emits `#compdef tu` on the first or second line; (e) `./dist/tu.mjs completions fish` emits `complete -c tu` lines; (f) `./dist/tu.mjs --csv --json` exits with code 1 and the incompatible-flags error on stderr; (g) `./dist/tu.mjs completions` (no arg) prints usage and exits 0. Report any failure with the exact output.

---

## Execution Order

**Sequential phases**: Phase 1 → Phase 2 → Phase 3 → Phase 4.

**Within Phase 1**: T001 and T002 are both `[P]`, run in parallel.

**Phase 1 → Phase 2 dependencies**: T001 (Phase 1) is a prerequisite for T003 (TOOLS shape depends on `ToolConfig`). T002 (Phase 1) is a prerequisite for T009 (completions dispatch imports the script constants).

**Within Phase 2**: T005, T006, T007 are independent and can run in parallel. T008 depends on T005 (flag parsing), T006 (`emitCsv`), and T007 (`emitMarkdown`). T009 depends on T002. T010 is independent of other Phase 2 work.

**Practical ordering**: T003 + T004 + T006 + T007 can run in parallel (different files). T005 runs first among the cli.ts tasks; T008 and T009 follow (both touch cli.ts, serialise to avoid merge friction). T010 can fold into whichever cli.ts task closes last.

**Within Phase 3**: All test tasks are `[P]` and can run in parallel — they extend/create different test files.

**Phase 4 is sequential**: T017 (tests + build) before T018 (smoke tests against the bundle).
