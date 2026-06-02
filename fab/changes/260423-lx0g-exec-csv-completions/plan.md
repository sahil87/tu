# Plan: Safer process spawning, CSV/Markdown export, and shell completions

**Change**: 260423-lx0g-exec-csv-completions
**Status**: In Progress
**Intake**: `intake.md`
**Spec**: `spec.md`

## Requirements

<!-- migrated from spec.md on 2026-06-02 -->

### Non-Goals

- Streaming process output via `spawn` to replace `maxBuffer` — deferred to a follow-up change (originally perf #7 from `/fab-discuss`)
- Backwards-compatibility shim for the old `command: string` field on `ToolConfig` — `TOOLS` is internal; no external consumer
- Auto-detection of the user's shell for `tu completions` — explicit argument required (see Design Decisions)
- Dynamic-lookup completion scripts that shell out to `tu` on each tab-press — static generation only
- A `--no-heading` flag for Markdown output — always render `## {title}`; can be added in a follow-up if users request
- Shell support beyond bash/zsh/fish (no PowerShell, nushell, elvish, tcsh)

### CLI: Data Pipeline

#### Requirement: TOOLS Registry Shape

The `TOOLS` registry in `src/node/core/fetcher.ts` MUST expose each tool as a `ToolConfig` object with four fields: `name` (display name), `binary` (absolute or on-PATH executable to invoke), `prefixArgs` (array of arguments prepended before runtime arguments, e.g., `[".../index.js"]` for vendor-path invocations), and `needsFilter` (boolean — whether stdout needs `stripNoise` before JSON parsing).

The `ToolConfig` interface in `src/node/core/types.ts` MUST match this shape. No other exported identifiers from `types.ts` change.

##### Scenario: Vendor mode constructs binary + prefixArgs
- **GIVEN** `useVendor` is true (the bundled `vendor/` directory exists next to the running script)
- **WHEN** the `TOOLS` registry is constructed
- **THEN** each entry's `binary` MUST be `"node"`
- **AND** each entry's `prefixArgs` MUST be `[".../${toolName}/index.js"]` resolved under the vendor directory
- **AND** `needsFilter` MUST match the per-tool filter requirement (`false` for `cc`, `true` for `codex` and `oc`)

##### Scenario: Non-vendor mode uses on-PATH binary directly
- **GIVEN** `useVendor` is false (`vendor/` directory does not exist, dev/test mode uses `node_modules/.bin`)
- **WHEN** the `TOOLS` registry is constructed
- **THEN** each entry's `binary` MUST be the resolved path to the `ccusage`/`ccusage-codex`/`ccusage-opencode` launcher in `node_modules/.bin/`
- **AND** each entry's `prefixArgs` MUST be an empty array

#### Requirement: Child Process Spawning via execFile

All ccusage child process invocations in `src/node/core/fetcher.ts` MUST use `child_process.execFile` (or the equivalent `execFile` via a promisified wrapper). Direct use of `child_process.exec` is prohibited in this file.

`execFile` SHALL be invoked with the tool's `binary` as the first argument and a single argv array as the second argument, constructed as `[...tool.prefixArgs, period, "--json", ...extraArgs]`. The third argument MUST be the options object carrying `encoding: "utf-8"` and `maxBuffer: 10 * 1024 * 1024` (unchanged from the pre-change value).

Error handling MUST preserve existing behaviour: on process error, warn on stderr with the tool name and error message, resolve the wrapper Promise with an empty string (so downstream parsing produces `EMPTY` totals).

##### Scenario: execFile called with argv array
- **GIVEN** a call to `fetchHistory("cc", "daily", ["--since", "2026-04-01"])` with `useVendor: true`
- **WHEN** the internal spawning wrapper runs
- **THEN** `execFile` MUST be called with `"node"` as the first argument
- **AND** the argv array MUST be `["<vendor>/ccusage/index.js", "daily", "--json", "--since", "2026-04-01"]`
- **AND** no shell subprocess is invoked at any point

##### Scenario: Spawning error produces warning and empty string
- **GIVEN** the `ccusage` child process exits with a non-zero code or cannot be spawned
- **WHEN** `execFile`'s callback fires with an error
- **THEN** a warning MUST be written to stderr in the format `warning: {toolName} fetch failed ({error.message}), showing zero data`
- **AND** the wrapper Promise MUST resolve with `""` (empty string)
- **AND** the calling `fetchHistory`/`fetchTotals` MUST return `EMPTY` totals or `[]` accordingly

##### Scenario: Shell metacharacters in prefixArgs are passed literally
- **GIVEN** a `prefixArgs` entry containing spaces or quote characters (e.g., `"/path with spaces/index.js"`)
- **WHEN** `execFile` is invoked
- **THEN** the argument MUST be passed as a single literal argv entry
- **AND** no shell parsing or quote processing is applied

#### Requirement: Output Format Dispatch

The CLI MUST support four output formats: `table` (the existing ANSI-rendered default), `json`, `csv`, and `md`. The format is selected by mutually exclusive global flags: `--json`, `--csv`, `--md`. The default when none is set is `table`.

`parseGlobalFlags` in `src/node/core/cli.ts` MUST return an `outputFormat` field of type `"table" | "json" | "csv" | "md"` (or equivalent). The existing `jsonFlag` boolean MAY remain for internal compatibility during the transition, but all dispatch paths SHALL read from `outputFormat`.

Dispatch functions (`dispatchAllHistory`, `dispatchAllSnapshot`, `dispatchSingleTool`) MUST branch on `outputFormat` and call the appropriate emitter: `emitJson` for `json`, a new `emitCsv` for `csv`, a new `emitMarkdown` for `md`, and the existing `print*` functions for `table`.

##### Scenario: Default invocation uses table format
- **GIVEN** `tu` invoked with no output-format flag
- **WHEN** parsing completes
- **THEN** `outputFormat` MUST be `"table"`
- **AND** dispatch MUST call the existing ANSI-rendered `print*` functions

##### Scenario: --csv selects CSV format
- **GIVEN** `tu cc --csv` invoked
- **WHEN** dispatch runs
- **THEN** `outputFormat` MUST be `"csv"`
- **AND** `emitCsv` MUST be called instead of `printHistory`
- **AND** no ANSI escape codes appear in stdout

##### Scenario: --md selects Markdown format
- **GIVEN** `tu m --md` invoked
- **WHEN** dispatch runs
- **THEN** `outputFormat` MUST be `"md"`
- **AND** `emitMarkdown` MUST be called instead of `printTotal`
- **AND** stdout begins with a `## ` heading line

#### Requirement: Output Format Flag Conflicts

The CLI MUST reject invocations that combine incompatible output-format flags. The following combinations MUST produce an error on stderr and exit code 1:

- Any two of `--json`, `--csv`, `--md` together
- Any of `--json`, `--csv`, `--md` combined with `--watch` (or `-w`)

The existing `--watch` + `--json` error SHALL be retained as-is; new errors follow the same pattern (`Error: {flag-a} and {flag-b} are incompatible`).

##### Scenario: --csv + --json is rejected
- **GIVEN** `tu --csv --json` invoked
- **WHEN** `parseGlobalFlags` runs
- **THEN** stderr MUST contain `Error: --json and --csv are incompatible` (or equivalent; exact flag order in the message MAY reflect argv order)
- **AND** the process MUST exit with code 1

##### Scenario: --md + --watch is rejected
- **GIVEN** `tu --md --watch` invoked
- **WHEN** `parseGlobalFlags` runs
- **THEN** stderr MUST contain an error indicating the incompatibility
- **AND** the process MUST exit with code 1

##### Scenario: Existing --json + --watch rejection preserved
- **GIVEN** `tu --json --watch` invoked
- **WHEN** `parseGlobalFlags` runs
- **THEN** stderr MUST contain `Error: --watch and --json are incompatible`
- **AND** the process MUST exit with code 1

#### Requirement: `tu completions <shell>` Subcommand

The CLI MUST dispatch a new `completions` subcommand before grammar parsing, at the same dispatch site as existing non-data commands (`init-conf`, `init-metrics`, `sync`, `status`, `update`).

`runCompletions(shell?)` MUST behave as follows:
- `undefined` or no shell argument → print usage (see Install Examples below) to stdout, exit 0
- `"bash"`, `"zsh"`, or `"fish"` → write the corresponding static completion script to stdout, exit 0
- Any other string → print `Unknown shell: {shell}. Supported: bash, zsh, fish` to stderr, exit 1

Completion scripts MUST be statically generated (hardcoded strings in the bundle); they MUST NOT invoke `tu` at tab-press time to enumerate tokens.

##### Scenario: `tu completions bash` emits bash script
- **GIVEN** `tu completions bash` invoked
- **WHEN** dispatch runs
- **THEN** stdout MUST contain a bash completion script that uses the `complete` builtin
- **AND** the script MUST reference the `tu` command
- **AND** the process MUST exit 0

##### Scenario: `tu completions zsh` emits zsh script
- **GIVEN** `tu completions zsh` invoked
- **WHEN** dispatch runs
- **THEN** stdout MUST contain a zsh completion script using `#compdef tu` and `_arguments`/`_values`
- **AND** the process MUST exit 0

##### Scenario: `tu completions fish` emits fish script
- **GIVEN** `tu completions fish` invoked
- **WHEN** dispatch runs
- **THEN** stdout MUST contain a fish completion script using `complete -c tu -n ...` directives
- **AND** the process MUST exit 0

##### Scenario: `tu completions` with no argument prints usage
- **GIVEN** `tu completions` invoked with no further arguments
- **WHEN** dispatch runs
- **THEN** stdout MUST contain `Usage: tu completions <bash|zsh|fish>`
- **AND** stdout MUST contain install examples for all three shells
- **AND** the process MUST exit 0

##### Scenario: Unknown shell returns error
- **GIVEN** `tu completions powershell` invoked
- **WHEN** dispatch runs
- **THEN** stderr MUST contain `Unknown shell: powershell. Supported: bash, zsh, fish`
- **AND** the process MUST exit with code 1

#### Requirement: Completion Script Coverage

Each emitted completion script MUST cover the full grammar. Specifically:

- **Non-data subcommands**: `help`, `init-conf`, `init-metrics`, `sync`, `status`, `update`, `completions`
- **Sources**: `cc`, `codex`, `co`, `oc`, `all`
- **Periods**: `d`, `m`, `daily`, `monthly`
- **Display tokens**: `h`, `history`, `dh`, `mh`
- **Global flags (long)**: `--json`, `--csv`, `--md`, `--sync`, `--fresh`, `--watch`, `--interval`, `--user`, `--by-machine`, `--no-color`, `--no-rain`, `--version`, `--help`
- **Global flags (short)**: `-f`, `-w`, `-i`, `-u`, `-v`, `-V`, `-h`
- **`completions` args**: `bash`, `zsh`, `fish`

##### Scenario: Bash script completes `tu c<TAB>` with source + subcommand candidates
- **GIVEN** a shell with the bash completion script sourced
- **WHEN** the user types `tu c` and presses Tab
- **THEN** candidates MUST include at least `cc`, `codex`, `co`, and `completions`

##### Scenario: All three scripts reference the canonical flag list
- **GIVEN** each of the three generated scripts (bash, zsh, fish)
- **WHEN** the script content is inspected
- **THEN** each script MUST contain literal occurrences of every long flag in the taxonomy above

### Display: Formatting

#### Requirement: CSV Output Rendering

A new `emitCsv(data, kind)` function MUST render the three data kinds — `"snapshot"`, `"history"`, `"total-history"` — as RFC 4180-compliant CSV on stdout.

Common rules for all kinds:
- First line is a header row
- Field separator: comma
- Line terminator: LF (`\n`) — not CRLF
- No byte-order mark
- String fields containing `,`, `"`, or newlines MUST be quoted with `"` and any internal `"` doubled (`""`)
- Numeric fields MUST be rendered without thousands separators (raw integers or decimals; e.g., `1234567` not `1,234,567`)
- Cost fields MUST be formatted with two decimal places and no currency symbol (e.g., `12.34`)
- No ANSI escape codes appear in the output
- No inline bar characters or delta indicators

##### Scenario: Snapshot CSV has tool, tokens, input, output, cost columns
- **GIVEN** `tu --csv` invoked with multi-tool snapshot data
- **WHEN** `emitCsv` renders
- **THEN** the header line MUST be `tool,tokens,input,output,cost`
- **AND** each subsequent line MUST contain one tool's values in that column order
- **AND** when more than one tool has visible data, a final `Total,...` row MUST follow

##### Scenario: History CSV has date, token-breakdown, total, cost columns
- **GIVEN** `tu cc h --csv` invoked with single-tool history
- **WHEN** `emitCsv` renders
- **THEN** the header MUST be `date,input,output,cache_write,cache_read,total,cost`
- **AND** each subsequent row MUST have the date in ISO format (`YYYY-MM-DD` for daily, `YYYY-MM` for monthly)

##### Scenario: Total-history pivot CSV has date, per-tool, total columns
- **GIVEN** `tu h --csv` invoked with all-tools history
- **WHEN** `emitCsv` renders
- **THEN** the header MUST be `date,{tool1},{tool2},...,total` where `{toolN}` is the tool's display name
- **AND** rows are sorted by date ascending

##### Scenario: Machine columns append when --by-machine is active
- **GIVEN** `tu --csv --by-machine` invoked
- **WHEN** `emitCsv` renders snapshot data with machine breakdowns
- **THEN** additional columns MUST follow the base columns, named `machine_{name}_cost` for each machine
- **AND** machine columns MUST be sorted alphabetically by machine name

##### Scenario: String with comma is quoted
- **GIVEN** a tool name contains a comma (hypothetical)
- **WHEN** the tool row is rendered in CSV
- **THEN** the tool name field MUST be wrapped in `"` quotes

#### Requirement: Markdown Output Rendering

A new `emitMarkdown(data, kind)` function MUST render the same three data kinds as GitHub-flavoured Markdown tables on stdout.

Common rules for all kinds:
- Output begins with a leading heading line `## {title}` where `{title}` matches the title used by the corresponding ANSI table renderer (e.g., `Combined Usage (daily)`, `Claude Code (monthly)`, `Combined Cost History (daily)`)
- A blank line follows the heading
- Then a GFM table: header row, alignment separator row, data rows, optional bold `**Total**` row
- String columns are left-aligned (`:---`)
- Numeric columns (tokens, cost) are right-aligned (`---:`)
- Numeric values in Markdown use comma thousands separators (human-readable)
- Cost values use `$` prefix and two decimal places (e.g., `$12.34`)
- No ANSI escape codes
- No inline bar characters or delta indicators
- A trailing blank line at the end of the output

##### Scenario: Snapshot Markdown has heading + GFM table
- **GIVEN** `tu m --md` invoked
- **WHEN** `emitMarkdown` renders
- **THEN** the first line MUST be `## Combined Usage (monthly)`
- **AND** the third line MUST be a GFM header row: `| Tool | Tokens | Input | Output | Cost |` (or similar column set)
- **AND** the fourth line MUST be the alignment separator row with left/right alignments

##### Scenario: Total row rendered in bold when multiple tools visible
- **GIVEN** `tu --md` with at least two tools having non-zero totals
- **WHEN** `emitMarkdown` renders
- **THEN** the final table row MUST have `**Total**` as its first cell
- **AND** numeric values in the Total row MUST be bolded (`**value**`) to match the ANSI renderer's `boldWhite` convention

##### Scenario: Total row omitted when only one tool has visible data
- **GIVEN** `tu --md` with exactly one tool having non-zero totals
- **WHEN** `emitMarkdown` renders
- **THEN** no `**Total**` row MUST be rendered
- **AND** this matches the existing `renderTotal` behaviour (total row guarded by `visibleCount > 1`)

##### Scenario: History Markdown date column is left-aligned
- **GIVEN** `tu cc h --md` invoked
- **WHEN** `emitMarkdown` renders
- **THEN** the first column MUST be `Date` with left-alignment (`:---`)
- **AND** all numeric columns (input, output, cache write, cache read, total, cost) MUST be right-aligned (`---:`)

##### Scenario: Machine columns append when --by-machine is active
- **GIVEN** `tu --md --by-machine` invoked
- **WHEN** `emitMarkdown` renders
- **THEN** per-machine cost columns MUST follow the base columns, headed by the machine name (not the A/B/C letter codes used by the ANSI renderer)
- **AND** a `Machines: A = name, B = name` legend line MUST NOT be emitted (machines are named directly in the Markdown header)

#### Requirement: Shared Dispatch Layer

The three output formats (`json`, `csv`, `md`) and the default `table` path MUST be selectable via a single decision point per dispatch function. Duplication of fetch logic across format branches is prohibited — fetching happens once, rendering dispatches on `outputFormat`.

##### Scenario: Fetch runs once regardless of format
- **GIVEN** `tu h --csv` invoked
- **WHEN** `dispatchAllHistory` runs
- **THEN** `fetchAllHistory` or `fetchToolMerged` is called exactly once per tool
- **AND** the returned data is passed to `emitCsv` without re-fetching

### Sync: Multi-Machine

#### Requirement: Git Invocation via execFile

All git command invocations in `src/node/sync/sync.ts` MUST use `execFile("git", [...argv])` rather than `exec("git -C ... ...")`. The command string and its quoting are replaced by an argv array passed literally.

The `git` helper wrapper in `syncMetrics` MUST accept an argv array (e.g., `(args: string[]) => execFileAsync("git", args)`) rather than a string. Callsites MUST pass arrays:

- `git(["-C", metricsDir, "add", `${user}/`])`
- `git(["-C", metricsDir, "status", "--porcelain", `${user}/`])`
- `git(["-C", metricsDir, "commit", "-m", `# ${user}: update ${date}`])`
- `git(["-C", metricsDir, "pull", "--rebase", "origin", "main"])`
- `git(["-C", metricsDir, "push"])`
- `git(["-C", metricsDir, "rebase", "--abort"])`

The interrupted-rebase recovery path and the single-retry push path MUST preserve their existing semantics (same git commands, same ordering, same error messages on stderr).

##### Scenario: Git commit uses execFile
- **GIVEN** `syncMetrics` called with a dirty working tree
- **WHEN** the commit step runs
- **THEN** `execFile("git", ["-C", metricsDir, "commit", "-m", "# user: update 2026-04-23"])` MUST be invoked
- **AND** no shell is forked

##### Scenario: metricsDir with spaces works correctly
- **GIVEN** `config.metricsDir` is `/home/user/My Data/metrics`
- **WHEN** `syncMetrics` runs git commands
- **THEN** each `execFile` call MUST pass the path as a single literal argv entry
- **AND** all git commands MUST succeed (no path-parsing breakage)

##### Scenario: Interrupted rebase recovery still works
- **GIVEN** `metricsDir/.git/rebase-merge` or `metricsDir/.git/rebase-apply` exists at sync start
- **WHEN** `syncMetrics` detects the marker
- **THEN** `execFile("git", ["-C", metricsDir, "rebase", "--abort"])` MUST be invoked
- **AND** a warning MUST be written to stderr (`Warning: recovering from interrupted rebase`)
- **AND** sync MUST proceed with the normal flow

##### Scenario: Push retry preserved
- **GIVEN** the first `git push` fails
- **WHEN** `syncMetrics` handles the error
- **THEN** a second `execFile("git", ["-C", metricsDir, "push"])` MUST be attempted
- **AND** if the retry also fails, a stderr warning MUST be emitted with the error reason
- **AND** `syncMetrics` MUST return `false`

### Design Decisions

1. **`TOOLS` shape splits into `binary` + `prefixArgs`**
   - *Why*: `execFile` takes a binary and an argv array. Splitting the compound command cleanly maps to the `execFile` signature with no runtime string concatenation or shell-parsing fragility. The `prefixArgs` name (vs. just `args`) signals these arguments go *before* the per-call period/extraArgs.
   - *Rejected*: Keeping `command: string` and splitting at call time via `shlex`-like parsing — adds a dependency and re-introduces quoting complexity at the boundary.
   - *Rejected*: Single `argv: string[]` starting with the binary — less conventional, forces callers to destructure every time.

2. **Output format as a single enum value (`outputFormat`) plumbed through dispatch**
   - *Why*: Centralises the format decision at parse time and eliminates the `if (jsonFlag) ... else ...` branches that would multiply with each new format. Single well-exercised path per format.
   - *Rejected*: Three separate boolean flags (`csvFlag`, `mdFlag`, `jsonFlag`) — each dispatch function would need to repeat the same exclusion logic.

3. **`tu completions` prints usage when called with no shell argument**
   - *Why*: Completion installation is a one-off operation. Silent `$SHELL` auto-detection can mismatch the user's actual shell (e.g., a bash login shell running zsh interactively) and produces a confusing "completions appeared to install but don't work" state.
   - *Rejected*: `$SHELL` detection — too much magic for a user-initiated setup step.
   - *Rejected*: Default to bash — surprising for zsh/fish users.

4. **Static completion scripts (bundled as string constants)**
   - *Why*: The `tu` grammar is small and stable. Dynamic enumeration (shelling out to `tu` on each tab-press) adds 50-200ms latency to every completion and complicates the scripts without benefit. When the grammar changes, the release includes the updated script.
   - *Rejected*: Dynamic scripts calling `tu --list-sources` etc. — latency and complexity cost far exceeds the "auto-update" benefit.

5. **Markdown output always includes a `## {title}` heading**
   - *Why*: The dominant paste targets (GH PRs, GH issues, internal docs) benefit from the heading. Removing the heading is trivial post-hoc (`tail -n +2`); adding one is more work.
   - *Rejected*: No heading — optimises for the minority use case (users wanting pure tables).
   - *Deferred*: `--no-heading` suppression flag — listed as a non-goal. Easy follow-up if users request.

6. **Machine columns in Markdown use machine names directly (not A/B/C letter codes)**
   - *Why*: Markdown output is paste-ready; readers shouldn't need to cross-reference a legend. The A/B/C coding in the ANSI renderer exists to conserve width in narrow terminals — a constraint that doesn't apply to markdown (horizontal scroll is fine in GH/Slack).
   - *Rejected*: Letter codes with legend line — adds a row of unused text.

## Tasks

### Phase 1: Setup

- [x] T001 Update `ToolConfig` interface in `src/node/core/types.ts` — add `binary: string`, `prefixArgs: string[]`; remove `command: string`. Keep `name` and `needsFilter`.
- [x] T002 [P] Create `src/node/core/completions.ts` with three exported string constants: `BASH_COMPLETION`, `ZSH_COMPLETION`, `FISH_COMPLETION`. Each script covers the full grammar per spec Requirement "Completion Script Coverage" (non-data subcommands, sources, periods, display tokens, all long/short flags, `completions` args). Scripts MUST be statically generated.

### Phase 2: Core Implementation

- [x] T003 [P] Refactor `src/node/core/fetcher.ts` spawning layer: (a) rename/rewrite `execAsync(cmd, toolName)` to `execFileAsync(file, args, toolName)` using `node:child_process.execFile` instead of `exec`; (b) rebuild the `TOOLS` registry entries with the new shape (`binary`, `prefixArgs`) for both vendor and non-vendor paths; (c) update `runTool` to construct argv `[...tool.prefixArgs, period, "--json", ...extraArgs]` and pass to `execFileAsync`. Preserve `maxBuffer: 10 * 1024 * 1024` and the warn-on-error + empty-string-resolve behavior.
- [x] T004 [P] Refactor `src/node/sync/sync.ts` spawning layer: replace `execAsync(cmd: string)` with a wrapper accepting `(file: string, args: string[])` and invoking `node:child_process.execFile`. Update every call site in `syncMetrics` (add, status --porcelain, commit, pull --rebase, push, rebase --abort) to pass argv arrays. Preserve interrupted-rebase recovery and push retry semantics with their existing stderr warnings.
- [x] T005 [P] Add output-format parsing in `src/node/core/cli.ts`: extend `GlobalFlags` with `outputFormat: "table" | "json" | "csv" | "md"`; recognise `--csv` and `--md` in `parseGlobalFlags` (and add them to the filter-skip list); implement conflict detection that rejects (a) any two of `--json`/`--csv`/`--md` together, (b) any of `--json`/`--csv`/`--md` combined with `--watch`/`-w`. Error messages follow `Error: {flag-a} and {flag-b} are incompatible`. Retain backward-compatible `jsonFlag` boolean deriving from `outputFormat === "json"` if needed by downstream callers during the transition.
- [x] T006 [P] Implement `emitCsv(data, kind)` in `src/node/tui/formatter.ts`. Three `kind` values: `"snapshot"` (Map<string, UsageTotals>), `"history"` (UsageEntry[] with toolName), `"total-history"` (Map<string, UsageEntry[]>). RFC 4180 quoting, LF line endings, no BOM, raw numeric values (no thousands separators), cost two-decimal without `$`. Machine columns: append `machine_{name}_cost` columns sorted alphabetically when machine data present. Write to stdout via `process.stdout.write` or `console.log`.
- [x] T007 [P] Implement `emitMarkdown(data, kind)` in `src/node/tui/formatter.ts`. Same three `kind` values. Leading `## {title}` heading matching ANSI renderer titles (`Combined Usage (daily)`, `Claude Code (monthly)`, `Combined Cost History (daily)`). GFM tables: left-aligned string columns (`:---`), right-aligned numeric columns (`---:`). Numeric values use comma thousands separators; cost with `$` prefix. Total row bolded (`**Total**`, `**value**`) when `visibleCount > 1`. Machine columns use machine names directly (no letter codes or legend). Trailing blank line.
- [x] T008 Plumb `outputFormat` through dispatch in `src/node/core/cli.ts`. Update `dispatchAllHistory`, `dispatchAllSnapshot`, `dispatchSingleTool` to accept `outputFormat` and switch on it: `json` → `emitJson`, `csv` → `emitCsv(data, kind)`, `md` → `emitMarkdown(data, kind)`, `table` → existing `print*` functions. Fetch runs once per dispatch regardless of format. Update `main()` to pass `outputFormat` from parsed flags.
- [x] T009 Add `completions` non-data subcommand dispatch in `src/node/core/cli.ts` `main()` alongside `init-conf`, `init-metrics`, `sync`, `status`, `update`. Implement `runCompletions(shell?: string)`: no arg → print usage block with install examples for all three shells; `bash`/`zsh`/`fish` → `process.stdout.write(SCRIPT)` + exit 0; anything else → stderr `Unknown shell: {shell}. Supported: bash, zsh, fish` + exit 1. Import the three script constants from `src/node/core/completions.ts`.
- [x] T010 Update `FULL_HELP` in `src/node/core/cli.ts` to document the new `--csv` and `--md` flags alongside `--json`, and the new `completions` subcommand under the Setup section. Keep `SHORT_USAGE` unchanged.

### Phase 3: Integration & Edge Cases

- [x] T011 [P] Add tests in `src/node/core/__tests__/fetcher.test.ts` (extend existing file) covering: (a) `TOOLS` registry shape — each entry has `binary`, `prefixArgs`, `needsFilter`; vendor path uses `"node"` binary with `.../index.js` in prefixArgs; non-vendor path uses direct binary with empty prefixArgs; (b) argv construction in `runTool` — verify the array passed to `execFile` matches `[...prefixArgs, period, "--json", ...extraArgs]` (use a test double or spy for `execFile`).
- [x] T012 [P] Add tests in `src/node/sync/__tests__/sync.test.ts` (extend existing file) covering: (a) git commands are invoked via `execFile("git", [...])` not `exec("git ...")`; (b) metricsDir with spaces does not break sync (verify argv entry is literal); (c) interrupted-rebase recovery still emits `rebase --abort`; (d) push retry still attempts twice before warning+return false.
- [x] T013 [P] Add tests in `src/node/tui/__tests__/formatter.test.ts` (extend existing file) for `emitCsv`: (a) snapshot kind header/rows/total; (b) history kind header/rows; (c) total-history kind header/rows with per-tool columns; (d) machine columns appear with `--by-machine` data sorted alphabetically as `machine_{name}_cost`; (e) RFC 4180 quoting — fields with commas, quotes, or newlines; (f) LF line endings, no BOM; (g) cost formatting (two decimals, no `$`).
- [x] T014 [P] Add tests in `src/node/tui/__tests__/formatter.test.ts` for `emitMarkdown`: (a) leading `## {title}` matches ANSI renderer title for each kind; (b) GFM alignment separators (`:---` for strings, `---:` for numerics); (c) total row bolded when `visibleCount > 1`, omitted when 1; (d) numeric values have commas; (e) cost values have `$` prefix; (f) machine columns use machine names directly (no A/B/C letter codes, no legend line); (g) trailing blank line present.
- [x] T015 [P] Add tests in `src/node/core/__tests__/cli-parser.test.ts` (extend existing file — it already covers `parseGlobalFlags` behaviour) for flag-conflict rejection: `--json --csv`, `--csv --md`, `--json --md`, `--csv --watch`, `--md --watch`, `--csv -w`, `--md -w`. Each should produce the "incompatible" stderr message and exit 1. The existing `--json --watch` error (covered in `cli-watch-flag.test.ts`) MUST still fire with its current wording.
- [x] T016 [P] Add tests in a new `src/node/core/__tests__/completions.test.ts` for `runCompletions`: (a) `"bash"` writes a script containing `complete` builtin reference, exits 0; (b) `"zsh"` writes script containing `#compdef tu`, exits 0; (c) `"fish"` writes script containing `complete -c tu`, exits 0; (d) no-arg prints usage with install examples for all three shells, exits 0; (e) unknown shell writes `Unknown shell: powershell. Supported: bash, zsh, fish` to stderr, exits 1; (f) each of the three scripts contains literal occurrences of every long flag in the taxonomy (`--json`, `--csv`, `--md`, `--sync`, `--fresh`, `--watch`, `--interval`, `--user`, `--by-machine`, `--no-color`, `--no-rain`, `--version`, `--help`) and every non-data subcommand (`help`, `init-conf`, `init-metrics`, `sync`, `status`, `update`, `completions`).

### Phase 4: Polish

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

## Acceptance

### Functional Completeness

- [ ] CHK-001 TOOLS Registry Shape: `ToolConfig` interface in `types.ts` exposes `name`, `binary`, `prefixArgs`, `needsFilter` — no `command` field remains
- [ ] CHK-002 Child Process Spawning via execFile (fetcher): `src/node/core/fetcher.ts` uses `child_process.execFile` exclusively; no call to `child_process.exec` remains in the file
- [ ] CHK-003 Output Format Dispatch: `GlobalFlags.outputFormat` is one of `"table" | "json" | "csv" | "md"`; dispatch functions branch on this value for rendering
- [ ] CHK-004 Output Format Flag Conflicts: All seven combinations (`--json --csv`, `--csv --md`, `--json --md`, `--csv --watch`, `--md --watch`, `--csv -w`, `--md -w`) produce the "incompatible" stderr message + exit 1
- [ ] CHK-005 `tu completions <shell>` Subcommand: The `completions` subcommand is dispatched before grammar parsing alongside `status`/`sync`/etc.; supports `bash`, `zsh`, `fish`, no-arg, unknown-shell cases
- [ ] CHK-006 Completion Script Coverage: Each emitted script literally contains every non-data subcommand, every source, every period, every display token, and every long flag in the taxonomy
- [ ] CHK-007 CSV Output Rendering: `emitCsv(data, kind)` produces RFC 4180-compliant CSV for all three kinds (`snapshot`, `history`, `total-history`); LF line endings, no BOM, no ANSI
- [ ] CHK-008 Markdown Output Rendering: `emitMarkdown(data, kind)` produces GFM tables with leading `## {title}` heading for all three kinds; no ANSI, no bars, no delta arrows
- [ ] CHK-009 Shared Dispatch Layer: Each dispatch function fetches exactly once regardless of output format; no duplicated fetch logic across format branches
- [ ] CHK-010 Git Invocation via execFile (sync): `src/node/sync/sync.ts` uses `child_process.execFile` exclusively for all git commands; no call to `child_process.exec` remains

### Behavioral Correctness

- [ ] CHK-011 Existing `--watch + --json` rejection preserved: Error message still reads `Error: --watch and --json are incompatible` with its original wording
- [ ] CHK-012 Existing ccusage spawn-error semantics preserved: On process error, stderr warning in format `warning: {toolName} fetch failed ({error.message}), showing zero data`; wrapper resolves `""`; `fetchHistory`/`fetchTotals` return `EMPTY` / `[]`
- [ ] CHK-013 Existing interrupted-rebase recovery preserved: `.git/rebase-merge` or `.git/rebase-apply` presence triggers `git rebase --abort` with stderr warning `Warning: recovering from interrupted rebase`
- [ ] CHK-014 Existing push retry preserved: First push failure triggers a second attempt; second failure emits stderr warning with error reason and returns `false`
- [ ] CHK-015 Existing `maxBuffer: 10 * 1024 * 1024` preserved on the fetcher spawn wrapper
- [ ] CHK-016 Existing total-row visibility rule preserved: Total row rendered only when `visibleCount > 1` (applies to CSV and Markdown alike)
- [ ] CHK-017 Existing ANSI `print*` path unchanged: `tu` with no format flag produces byte-identical output to pre-change on the same input (aside from unrelated runtime nondeterminism)

### Removal Verification

- [ ] CHK-018 `ToolConfig.command` field removed: No reference to `tool.command` or `.command:` property access on `ToolConfig` remains anywhere in `src/node/**`
- [ ] CHK-019 String-based `exec(cmd)` call sites removed: grep for `child_process.exec\b` (not `execFile`) in `src/node/core/fetcher.ts` and `src/node/sync/sync.ts` returns zero matches

### Scenario Coverage

- [ ] CHK-020 execFile argv construction (vendor mode): Test verifies `execFile("node", [".../ccusage/index.js", "daily", "--json", ...])` for vendor path
- [ ] CHK-021 execFile argv construction (non-vendor mode): Test verifies `execFile("<bin>/ccusage", ["daily", "--json", ...])` with empty prefixArgs
- [ ] CHK-022 Shell metacharacters passed literally: Test verifies a prefixArgs entry containing spaces is passed as a single literal argv entry (no shell parsing)
- [ ] CHK-023 Default invocation uses table format: `tu` with no format flag produces ANSI table via `print*` functions
- [ ] CHK-024 `tu completions bash` emits bash script: stdout contains `complete` builtin reference; exit 0
- [ ] CHK-025 `tu completions zsh` emits zsh script: stdout contains `#compdef tu`; exit 0
- [ ] CHK-026 `tu completions fish` emits fish script: stdout contains `complete -c tu`; exit 0
- [ ] CHK-027 `tu completions` no-arg prints usage: stdout contains `Usage: tu completions <bash|zsh|fish>` and install examples; exit 0
- [ ] CHK-028 Unknown shell returns error: `tu completions powershell` writes `Unknown shell: powershell. Supported: bash, zsh, fish` to stderr; exit 1
- [ ] CHK-029 CSV snapshot has `tool,tokens,input,output,cost` header followed by data rows and Total row (if >1 tool)
- [ ] CHK-030 CSV history has `date,input,output,cache_write,cache_read,total,cost` header with ISO date labels
- [ ] CHK-031 CSV total-history has `date,{tool1},{tool2},...,total` header with rows sorted by date ascending
- [ ] CHK-032 Markdown heading matches ANSI renderer title for each kind
- [ ] CHK-033 Markdown GFM alignment: string columns `:---`, numeric columns `---:`
- [ ] CHK-034 Markdown Total row bold (`**Total**`, `**value**`) when >1 tool visible
- [ ] CHK-035 Machine columns CSV: `machine_{name}_cost` suffix columns sorted alphabetically when `--by-machine` data present
- [ ] CHK-036 Machine columns MD: machine names used directly as column headers (no A/B/C letter codes, no legend line)
- [ ] CHK-037 Git commit via execFile: Test verifies `execFile("git", ["-C", metricsDir, "commit", "-m", "..."])` invocation path
- [ ] CHK-038 `metricsDir` with spaces: Test with path containing a space verifies argv entry is literal and git commands succeed

### Edge Cases & Error Handling

- [ ] CHK-039 CSV string field with embedded comma is wrapped in `"` quotes per RFC 4180
- [ ] CHK-040 CSV string field with embedded `"` is quoted and the inner `"` is doubled (`""`)
- [ ] CHK-041 Empty data (no usage, `allZero`) in CSV and MD emits an empty-data indicator or a header-only table consistent with the existing `No usage` / `No data` ANSI fallback (document the chosen form in tests)
- [ ] CHK-042 Single-tool Markdown output with only one tool visible omits the Total row (mirrors `visibleCount > 1` guard)

### Code Quality

- [ ] CHK-043 Pattern consistency: New code follows naming and structural patterns of surrounding code (e.g., new functions co-located with similar existing renderers; same error-handling idioms as the ANSI path)
- [ ] CHK-044 No unnecessary duplication: `emitCsv` and `emitMarkdown` reuse existing utilities (`fmtCost`, `fmtNum`, `currentLabel`, `buildMachineColumns`) where applicable
- [ ] CHK-045 Readability over cleverness: No obscure one-liners or dense conditional chains where a short named helper would clarify intent (per `code-quality.md` Principles)
- [ ] CHK-046 Functions over classes: New code uses plain functions and objects — no classes introduced (per `code-quality.md` Principles)
- [ ] CHK-047 `type` imports used for type-only values: `import type { ... }` for `UsageTotals`, `UsageEntry`, `ToolConfig`, etc. where only the type is referenced
- [ ] CHK-048 `node:` prefixed imports: New code uses `node:child_process`, `node:process`, etc. (per `code-quality.md` Principles)
- [ ] CHK-049 Minimum pathways: Format dispatch funnels through a single switch/match on `outputFormat` — no parallel `if (csvFlag)` / `if (mdFlag)` branches duplicated across dispatch functions (per `code-quality.md` Principles)
- [ ] CHK-050 No god functions: `emitCsv` and `emitMarkdown` are broken into per-kind helpers if the combined body would exceed ~50 lines (per `code-quality.md` Anti-Patterns)
- [ ] CHK-051 No magic strings: CSV header column names and MD heading titles are named constants or derived from existing title strings (per `code-quality.md` Anti-Patterns)
- [ ] CHK-052 No silent error swallowing: `runCompletions` unknown-shell path emits stderr message (not silent `exit 1`) (per `code-quality.md` Anti-Patterns)
- [ ] CHK-053 No new dynamic `import()` calls introduced for core paths (per `code-quality.md` Anti-Patterns)

### Security

- [ ] CHK-054 No shell subprocess is forked by any new or modified code path in `fetcher.ts` or `sync.ts` (verified by grep for `child_process.exec\b` and by test assertion on the mock's invocation)
- [ ] CHK-055 Argv arrays pass user-controlled strings (config.metricsDir, config.user, toolKey, extraArgs) as literal entries — no string interpolation into a command that a shell would re-parse

### Notes

- Check items as you review: `- [x]`
- All items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] CHK-008 **N/A**: {reason}`
